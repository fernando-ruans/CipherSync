package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattn/go-isatty"
)

func newTestAppWithVault(t *testing.T) (*App, *Vault) {
	t.Helper()
	dir := t.TempDir()
	v, err := createVault(filepath.Join(dir, "v.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{vault: v, vaultFile: "v.passapp", vaultName: "v"}
	return a, v
}

func postLocal(a *App, action string, payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["action"] = action
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(raw))
	req.Header.Set("X-Token", "tok")
	w := httptest.NewRecorder()
	// inject a server with a known token
	a.localAPIMu.Lock()
	a.localAPI = &localAPIServer{token: "tok", port: 1}
	a.localAPIMu.Unlock()
	a.handleLocalAPI(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func cleanupLocal(a *App) {
	a.localAPIMu.Lock()
	a.localAPI = nil
	a.localAPIMu.Unlock()
}

func TestLocalGetLoginsDomainScoping(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer func() { v.close(); cleanupLocal(a) }()

	if _, err := v.create(Item{Type: TypeLogin, Title: "GH", Username: "u", Password: "p", URL: "https://github.com/login"}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.create(Item{Type: TypeLogin, Title: "GL", Username: "u", Password: "p", URL: "https://gitlab.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.create(Item{Type: TypeLogin, Title: "NoURL", Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	}

	// missing url must be rejected, never return the whole vault
	res := postLocal(a, "get-logins", nil)
	if res["success"] != false || res["error"] != "missing-url" {
		t.Fatalf("empty url must be rejected: %+v", res)
	}

	// same eTLD+1 matches (subdomain included)
	res = postLocal(a, "get-logins", map[string]interface{}{"url": "https://gist.github.com/x"})
	logins, _ := res["logins"].([]interface{})
	if res["success"] != true || len(logins) != 1 {
		t.Fatalf("expected 1 github login, got %+v", res)
	}

	// different site matches nothing
	res = postLocal(a, "get-logins", map[string]interface{}{"url": "https://evil.example"})
	logins, _ = res["logins"].([]interface{})
	if len(logins) != 0 {
		t.Fatalf("expected 0 logins for unrelated site, got %d", len(logins))
	}
}

func TestLocalGetLoginsCountAndTotp(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer func() { v.close(); cleanupLocal(a) }()

	it, err := v.create(Item{Type: TypeLogin, Title: "GH", Username: "u", Password: "p", URL: "https://github.com", TotpSecret: "JBSWY3DPEHPK3PXP"})
	if err != nil {
		t.Fatal(err)
	}
	_ = it

	res := postLocal(a, "get-logins", map[string]interface{}{"url": "https://github.com"})
	logins, _ := res["logins"].([]interface{})
	if len(logins) != 1 {
		t.Fatalf("expected 1 login, got %+v", res)
	}
	entry, _ := logins[0].(map[string]interface{})
	if entry["totp"] == "" {
		t.Fatal("expected totp code in entry")
	}

	res = postLocal(a, "get-logins-count", map[string]interface{}{"url": "https://github.com"})
	if res["count"].(float64) != 1 {
		t.Fatalf("expected count 1, got %+v", res)
	}
}

func TestLocalSetLoginUpsert(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer func() { v.close(); cleanupLocal(a) }()

	// missing fields rejected
	res := postLocal(a, "set-login", map[string]interface{}{"url": "https://github.com"})
	if res["error"] != "missing-fields" {
		t.Fatalf("expected missing-fields: %+v", res)
	}

	payload := map[string]interface{}{
		"url": "https://github.com", "title": "GitHub", "username": "u", "password": "p1",
	}
	res = postLocal(a, "set-login", payload)
	if res["success"] != true || res["updated"] == true {
		t.Fatalf("expected create: %+v", res)
	}
	// same URL+username updates
	payload["password"] = "p2"
	res = postLocal(a, "set-login", payload)
	if res["success"] != true || res["updated"] != true {
		t.Fatalf("expected update: %+v", res)
	}
	items := v.list()
	if len(items) != 1 || items[0].Password != "p2" {
		t.Fatalf("upsert failed: %+v", items)
	}
}

func TestLocalGetTotp(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer func() { v.close(); cleanupLocal(a) }()

	// unknown id -> not-found
	res := postLocal(a, "get-totp", map[string]interface{}{"id": "nope"})
	if res["error"] != "not-found" {
		t.Fatalf("expected not-found: %+v", res)
	}

	it, err := v.create(Item{Type: TypeLogin, Title: "GH", TotpSecret: "JBSWY3DPEHPK3PXP"})
	if err != nil {
		t.Fatal(err)
	}
	res = postLocal(a, "get-totp", map[string]interface{}{"id": it.ID})
	if res["success"] != true || res["totp"] == "" {
		t.Fatalf("expected totp: %+v", res)
	}

	// item without 2FA -> explicit error
	it2, err := v.create(Item{Type: TypeLogin, Title: "No2FA"})
	if err != nil {
		t.Fatal(err)
	}
	res = postLocal(a, "get-totp", map[string]interface{}{"id": it2.ID})
	if res["success"] != false {
		t.Fatalf("expected error for item without 2FA: %+v", res)
	}
}

func TestLocalAPIMethodAndAuth(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer func() { v.close(); cleanupLocal(a) }()

	a.localAPIMu.Lock()
	a.localAPI = &localAPIServer{token: "tok", port: 1}
	a.localAPIMu.Unlock()

	// GET rejected
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a.handleLocalAPI(w, req)
	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}

	// wrong token rejected
	req = httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`{"action":"ping"}`)))
	req.Header.Set("X-Token", "wrong")
	w = httptest.NewRecorder()
	a.handleLocalAPI(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDomainRootAndFold(t *testing.T) {
	if domainRoot("gist.github.com") != "github.com" {
		t.Fatal("domainRoot failed")
	}
	if domainRoot("github.com") != "github.com" {
		t.Fatal("domainRoot failed")
	}
	if domainRoot("GITHUB.COM") != "github.com" {
		t.Fatal("domainRoot must lowercase")
	}
	if !stringsEqualFold("ABC", "abc") {
		t.Fatal("stringsEqualFold failed")
	}
	if stringsEqualFold("abc", "abcd") {
		t.Fatal("stringsEqualFold must distinguish lengths")
	}
}

func TestIsNativeHostInvocation(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// explicit flag always wins
	os.Args = []string{"CipherSync.exe", "--native-host"}
	if !isNativeHostInvocation() {
		t.Fatal("explicit flag must win")
	}

	// no args at all: GUI launch (regardless of stdin)
	os.Args = []string{"CipherSync.exe"}
	if isNativeHostInvocation() {
		t.Fatal("plain launch must stay GUI")
	}

	// piped-stdin cases depend on the test process's stdin; skip when
	// running interactively (stdin is a terminal)
	if isatty.IsTerminal(os.Stdin.Fd()) {
		t.Skip("stdin is a terminal; skipping pipe-detection cases")
	}

	// browser-style non-file arg with piped stdin
	os.Args = []string{"CipherSync.exe", "chrome-extension://abcdef/host.html"}
	if !isNativeHostInvocation() {
		t.Fatal("browser origin arg should trigger host mode")
	}

	// existing file path arg = GUI launch (e.g. "Open with")
	f := filepath.Join(t.TempDir(), "vault.passapp")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"CipherSync.exe", f}
	if isNativeHostInvocation() {
		t.Fatal("existing-file arg should not trigger host mode")
	}
}
