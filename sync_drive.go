package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	driveKeyringService = "CipherSync"
	driveKeyringAccount = "drive"
	driveScope          = "https://www.googleapis.com/auth/drive.file"
	driveFolderName     = "CipherSync"
)

type driveTokens struct {
	AccessToken  string `json:"access"`
	RefreshToken string `json:"refresh"`
	Expiry       int64  `json:"expiry"`
	Email        string `json:"email"`
}

func driveOAuthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		RedirectURL: "http://127.0.0.1:0/callback", // replaced with ephemeral port
		Scopes:      []string{driveScope},
	}
}

func driveLoadTokens() (*driveTokens, error) {
	raw, err := keyring.Get(driveKeyringService, driveKeyringAccount)
	if err != nil {
		return nil, err
	}
	var t driveTokens
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil, err
	}
	if t.RefreshToken == "" && t.AccessToken == "" {
		return nil, errors.New("no tokens")
	}
	return &t, nil
}

func driveSaveTokens(t *driveTokens) error {
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return keyring.Set(driveKeyringService, driveKeyringAccount, string(raw))
}

func driveDeleteTokens() {
	_ = keyring.Delete(driveKeyringService, driveKeyringAccount)
}

// driveAuthedClient returns an HTTP client with a fresh access token,
// refreshing via the stored refresh token when needed.
func (a *App) driveAuthedClient() (*http.Client, *driveTokens, error) {
	v := a.currentVault()
	if v == nil {
		return nil, nil, ErrVaultLocked
	}
	clientID, _ := v.getSetting("drive_client_id")
	clientSecret, _ := v.getSetting("drive_client_secret")
	if clientID == "" {
		return nil, nil, errors.New("Google client ID não configurado (veja Configurações)")
	}
	tok, err := driveLoadTokens()
	if err != nil {
		return nil, nil, errors.New("Google Drive não conectado")
	}
	cfg := driveOAuthConfig(clientID, clientSecret)
	oauthTok := &oauth2.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       time.UnixMilli(tok.Expiry),
	}
	// refresh with margin
	if tok.RefreshToken != "" && time.Until(time.UnixMilli(tok.Expiry)) < 5*time.Minute {
		src := cfg.TokenSource(a.ctx, oauthTok)
		refreshed, err := src.Token()
		if err != nil {
			return nil, nil, errors.New("sessão do Google expirada — reconecte")
		}
		tok.AccessToken = refreshed.AccessToken
		if refreshed.RefreshToken != "" {
			tok.RefreshToken = refreshed.RefreshToken
		}
		tok.Expiry = refreshed.Expiry.UnixMilli()
		_ = driveSaveTokens(tok)
		oauthTok = refreshed
	}
	return cfg.Client(a.ctx, oauthTok), tok, nil
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// DriveConnect runs the OAuth loopback flow (browser + PKCE), stores tokens
// in the OS keyring and returns the connected account email.
func (a *App) DriveConnect(clientID, clientSecret string) (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", errors.New("informe o client ID do Google Cloud")
	}
	if err := v.setSetting("drive_client_id", clientID); err != nil {
		return "", err
	}
	if err := v.setSetting("drive_client_secret", strings.TrimSpace(clientSecret)); err != nil {
		return "", err
	}

	cfg := driveOAuthConfig(clientID, strings.TrimSpace(clientSecret))
	verifier := oauth2.GenerateVerifier()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	state := randomState()
	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.S256ChallengeOption(verifier),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer srv.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- errors.New("state inválido")
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			errCh <- errors.New("autorização negada: " + e)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- errors.New("código ausente")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body style="font-family:sans-serif;text-align:center;padding-top:40px"><h2>CipherSync conectado!</h2><p>Você já pode fechar esta aba e voltar ao app.</p></body></html>`))
		codeCh <- code
	})
	srv.Handler = mux

	runtime.BrowserOpenURL(a.ctx, authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return "", err
	case <-time.After(5 * time.Minute):
		return "", errors.New("tempo esgotado aguardando autorização")
	}

	tok, err := cfg.Exchange(a.ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", errors.New("falha ao trocar o código: " + err.Error())
	}
	stored := &driveTokens{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry.UnixMilli(),
	}
	// fetch account email
	client := cfg.Client(a.ctx, tok)
	email := driveAccountEmail(client)
	stored.Email = email
	if err := driveSaveTokens(stored); err != nil {
		return "", err
	}
	return email, nil
}

func driveAccountEmail(client *http.Client) string {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/drive/v3/about?fields=user", nil)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		User struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"user"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return ""
	}
	return out.User.EmailAddress
}

// DriveDisconnect revokes tokens and clears Drive sync settings.
func (a *App) DriveDisconnect() error {
	if tok, err := driveLoadTokens(); err == nil && tok.AccessToken != "" {
		form := url.Values{"token": {tok.AccessToken}}
		_, _ = http.PostForm("https://oauth2.googleapis.com/revoke", form)
	}
	driveDeleteTokens()
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	_ = v.setSetting("sync_provider", "")
	_ = v.setSetting("sync_remote", "")
	return nil
}

// DriveSetupFolder finds or creates the CipherSync folder and stores its ID
// as this vault's sync remote (also enables the drive provider).
func (a *App) DriveSetupFolder() (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	client, _, err := a.driveAuthedClient()
	if err != nil {
		return "", err
	}
	p := &driveProvider{client: client}
	folderID, err := p.ensureFolder()
	if err != nil {
		return "", err
	}
	if err := v.setSetting("sync_provider", "drive"); err != nil {
		return "", err
	}
	if err := v.setSetting("sync_remote", folderID); err != nil {
		return "", err
	}
	return folderID, nil
}

// ---------- Drive provider (REST) ----------

type driveProvider struct {
	client *http.Client
}

func (p *driveProvider) Name() string { return "drive" }

type driveFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ModifiedTime string `json:"modifiedTime"`
	Md5Checksum  string `json:"md5Checksum"`
	Size         string `json:"size"`
}

func driveEscape(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func (p *driveProvider) findInFolder(folderID, name string) (*driveFile, error) {
	q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", driveEscape(name), folderID)
	u := "https://www.googleapis.com/drive/v3/files?q=" + url.QueryEscape(q) +
		"&fields=files(id,name,modifiedTime,md5Checksum,size)&spaces=drive&pageSize=10"
	req, _ := http.NewRequest("GET", u, nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("drive: list %d", resp.StatusCode)
	}
	var out struct {
		Files []driveFile `json:"files"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Files) == 0 {
		return nil, nil
	}
	return &out.Files[0], nil
}

func driveParseTime(s string) int64 {
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixMilli()
	}
	return 0
}

func driveParseSize(s string) int64 {
	var n int64
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// remote identifier format: "<folderID>/<filename>"
func splitDriveRemote(remote string) (folderID, name string, err error) {
	i := strings.LastIndex(remote, "/")
	if i <= 0 || i == len(remote)-1 {
		return "", "", errInvalidRemote
	}
	return remote[:i], remote[i+1:], nil
}

func (p *driveProvider) Stat(remote string) (SyncMeta, error) {
	folderID, name, err := splitDriveRemote(remote)
	if err != nil {
		return SyncMeta{}, err
	}
	f, err := p.findInFolder(folderID, name)
	if err != nil {
		return SyncMeta{}, err
	}
	if f == nil {
		return SyncMeta{Exists: false}, nil
	}
	return SyncMeta{
		Exists:  true,
		ModTime: driveParseTime(f.ModifiedTime),
		Size:    driveParseSize(f.Size),
		Rev:     f.ID + ":" + f.Md5Checksum,
	}, nil
}

func (p *driveProvider) Download(remote, dest string) error {
	folderID, name, err := splitDriveRemote(remote)
	if err != nil {
		return err
	}
	f, err := p.findInFolder(folderID, name)
	if err != nil {
		return err
	}
	if f == nil {
		return errInvalidRemote
	}
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/drive/v3/files/"+f.ID+"?alt=media", nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("drive: download %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	// cap at 64MB as a sanity bound (vaults are far smaller)
	_, err = io.Copy(out, io.LimitReader(resp.Body, 64<<20))
	return err
}

func (p *driveProvider) Upload(src, remote string) (string, error) {
	folderID, name, err := splitDriveRemote(remote)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	existing, err := p.findInFolder(folderID, name)
	if err != nil {
		return "", err
	}
	var fileID string
	if existing == nil {
		// multipart create: metadata + media
		meta, _ := json.Marshal(map[string]interface{}{"name": name, "parents": []string{folderID}})
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", "application/json; charset=UTF-8")
		part, err := mw.CreatePart(h)
		if err != nil {
			return "", err
		}
		if _, err := part.Write(meta); err != nil {
			return "", err
		}
		h2 := textproto.MIMEHeader{}
		h2.Set("Content-Type", "application/octet-stream")
		part2, err := mw.CreatePart(h2)
		if err != nil {
			return "", err
		}
		if _, err := part2.Write(data); err != nil {
			return "", err
		}
		if err := mw.Close(); err != nil {
			return "", err
		}
		req, _ := http.NewRequest("POST", "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&fields=id,md5Checksum", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := p.client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("drive: create %d", resp.StatusCode)
		}
		var out struct {
			ID   string `json:"id"`
			Md5  string `json:"md5Checksum"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
		fileID = out.ID
	} else {
		req, _ := http.NewRequest("PATCH", "https://www.googleapis.com/upload/drive/v3/files/"+existing.ID+"?uploadType=media&fields=id,md5Checksum", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.ContentLength = int64(len(data))
		resp, err := p.client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("drive: update %d", resp.StatusCode)
		}
		fileID = existing.ID
	}
	// re-stat for the fresh rev
	meta, err := p.Stat(remote)
	if err != nil {
		return fileID, nil
	}
	_ = fileID
	return meta.Rev, nil
}

func (p *driveProvider) ensureFolder() (string, error) {
	q := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and trashed = false", driveFolderName)
	u := "https://www.googleapis.com/drive/v3/files?q=" + url.QueryEscape(q) + "&fields=files(id)&spaces=drive&pageSize=5"
	req, _ := http.NewRequest("GET", u, nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	if len(out.Files) > 0 {
		return out.Files[0].ID, nil
	}
	body, _ := json.Marshal(map[string]string{"name": driveFolderName, "mimeType": "application/vnd.google-apps.folder"})
	req2, _ := http.NewRequest("POST", "https://www.googleapis.com/drive/v3/files?fields=id", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := p.client.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return "", fmt.Errorf("drive: mkdir %d", resp2.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp2.Body, 1<<20)).Decode(&created); err != nil || created.ID == "" {
		return "", errors.New("drive: não foi possível criar a pasta")
	}
	return created.ID, nil
}
