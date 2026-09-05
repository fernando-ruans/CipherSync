package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Local API: loopback JSON server that the --native-host shim talks to.
// Only runs while a vault is unlocked. Token file has 0600 permissions.
const localAPITokenFile = ".localapi.json"

type localAPIServer struct {
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	token    string
	port     int
}

func (a *App) localAPIInfoPath() string {
	return filepath.Join(a.VaultDir(), localAPITokenFile)
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (a *App) startLocalAPI() error {
	a.localAPIMu.Lock()
	defer a.localAPIMu.Unlock()
	if a.localAPI != nil {
		return nil
	}
	token := randomToken()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleLocalAPI)
	srv := &http.Server{Handler: mux}
	a.localAPI = &localAPIServer{listener: ln, server: srv, token: token, port: port}
	info, _ := json.Marshal(map[string]interface{}{"port": port, "token": token})
	_ = os.WriteFile(a.localAPIInfoPath(), info, 0o600)
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

func (a *App) stopLocalAPI() {
	a.localAPIMu.Lock()
	defer a.localAPIMu.Unlock()
	if a.localAPI != nil {
		_ = a.localAPI.server.Close()
		_ = a.localAPI.listener.Close()
		a.localAPI = nil
	}
	_ = os.Remove(a.localAPIInfoPath())
}

func (a *App) handleLocalAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.localAPIMu.Lock()
	token := ""
	if a.localAPI != nil {
		token = a.localAPI.token
	}
	a.localAPIMu.Unlock()
	if token == "" || r.Header.Get("X-Token") != token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !a.IsUnlocked() {
		writeLocalJSON(w, map[string]interface{}{"success": false, "error": "database-locked"})
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeLocalJSON(w, map[string]interface{}{"success": false, "error": "bad-request"})
		return
	}
	action, _ := req["action"].(string)
	switch action {
	case "ping":
		writeLocalJSON(w, map[string]interface{}{"success": true, "version": "1"})
	case "get-logins":
		urlStr, _ := req["url"].(string)
		writeLocalJSON(w, a.localGetLogins(urlStr))
	case "get-logins-count":
		urlStr, _ := req["url"].(string)
		res := a.localGetLogins(urlStr)
		if ok, _ := res["success"].(bool); !ok {
			writeLocalJSON(w, res)
			return
		}
		logins, _ := res["logins"].([]interface{})
		writeLocalJSON(w, map[string]interface{}{"success": true, "count": len(logins)})
	case "get-totp":
		id, _ := req["id"].(string)
		code, _, err := a.totpCodeForItem(id)
		if err != nil {
			writeLocalJSON(w, map[string]interface{}{"success": false, "error": "not-found"})
			return
		}
		writeLocalJSON(w, map[string]interface{}{"success": true, "totp": code})
	case "generate-password":
		pw, err := generatePassword(PasswordOptions{Length: 20, UseUpper: true, UseLower: true, UseDigits: true, UseSymbols: true})
		if err != nil {
			writeLocalJSON(w, map[string]interface{}{"success": false, "error": "error"})
			return
		}
		writeLocalJSON(w, map[string]interface{}{"success": true, "password": pw})
	case "set-login":
		writeLocalJSON(w, a.localSetLogin(req))
	case "lock-database":
		a.Lock()
		writeLocalJSON(w, map[string]interface{}{"success": true})
	default:
		writeLocalJSON(w, map[string]interface{}{"success": false, "error": "unknown-action"})
	}
}

func writeLocalJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// localGetLogins returns URL-matched logins (eTLD+1-ish matching).
// An empty/unparseable pageURL is rejected: without it we could not scope
// results to the requesting site and would hand out the whole vault.
func (a *App) localGetLogins(pageURL string) map[string]interface{} {
	v := a.currentVault()
	if v == nil {
		return map[string]interface{}{"success": false, "error": "database-locked"}
	}
	host := extractDomain(pageURL)
	if host == "" {
		return map[string]interface{}{"success": false, "error": "missing-url"}
	}
	root := domainRoot(host)
	out := []interface{}{}
	for _, it := range v.list() {
		if it.Deleted || it.Type != TypeLogin || it.Password == "" {
			continue
		}
		if it.URL == "" {
			continue
		}
		itemHost := extractDomain(it.URL)
		if itemHost == "" || !stringsEqualFold(domainRoot(itemHost), root) {
			continue
		}
		entry := map[string]interface{}{
			"id":       it.ID,
			"title":    it.Title,
			"username": it.Username,
			"password": it.Password,
		}
		if it.TotpSecret != "" {
			if code, _, err := totpCode(it.TotpSecret); err == nil {
				entry["totp"] = code
			}
		}
		out = append(out, entry)
		if len(out) >= 50 {
			break
		}
	}
	return map[string]interface{}{"success": true, "logins": out}
}

// localSetLogin creates or updates a login from the extension save prompt.
func (a *App) localSetLogin(req map[string]interface{}) map[string]interface{} {
	v := a.currentVault()
	if v == nil {
		return map[string]interface{}{"success": false, "error": "database-locked"}
	}
	urlStr, _ := req["url"].(string)
	title, _ := req["title"].(string)
	username, _ := req["username"].(string)
	password, _ := req["password"].(string)
	if password == "" || username == "" {
		return map[string]interface{}{"success": false, "error": "missing-fields"}
	}
	if title == "" {
		title = extractDomain(urlStr)
		if title == "" {
			title = "Sem título"
		}
	}
	// update if same URL+username exists, else create
	for _, it := range v.list() {
		if it.Type == TypeLogin && stringsEqualFold(extractDomain(it.URL), extractDomain(urlStr)) && it.Username == username {
			it.Password = password
			if title != "" {
				it.Title = title
			}
			if err := v.update(it); err != nil {
				return map[string]interface{}{"success": false, "error": "error"}
			}
			return map[string]interface{}{"success": true, "id": it.ID, "updated": true}
		}
	}
	created, err := v.create(Item{Type: TypeLogin, Title: title, Username: username, Password: password, URL: urlStr})
	if err != nil {
		return map[string]interface{}{"success": false, "error": "error"}
	}
	return map[string]interface{}{"success": true, "id": created.ID}
}

func (a *App) totpCodeForItem(id string) (string, int, error) {
	v := a.currentVault()
	if v == nil {
		return "", 0, ErrVaultLocked
	}
	item, err := v.getItem(id)
	if err != nil {
		return "", 0, err
	}
	if item.TotpSecret == "" {
		return "", 0, fmt.Errorf("no totp")
	}
	return totpCode(item.TotpSecret)
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		// still compare case-insensitively without strings pkg import cycle issues
		if len(a) == 0 || len(b) == 0 {
			return a == b
		}
	}
	la, lb := toLowerASCII(a), toLowerASCII(b)
	return la == lb
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// domainRoot returns the last two labels (good enough for matching).
func domainRoot(host string) string {
	host = toLowerASCII(host)
	parts := splitDots(host)
	if len(parts) <= 2 {
		return host
	}
	return parts[len(parts)-2] + "." + parts[len(parts)-1]
}

func splitDots(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
