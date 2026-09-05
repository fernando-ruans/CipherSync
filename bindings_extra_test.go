package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseKeePassDBRejectsBadInput(t *testing.T) {
	// garbage input must fail cleanly
	if _, err := parseKeePassDB([]byte("not a database"), "pw"); err == nil {
		t.Fatal("garbage input must fail")
	}
	if _, err := parseKeePassDB(nil, "pw"); err == nil {
		t.Fatal("empty input must fail")
	}
}

func TestCallLocalAPI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)

	// fake loopback API
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["action"] == "ping" {
			fmt.Fprint(w, `{"success":true,"version":"1"}`)
			return
		}
		fmt.Fprint(w, `{"success":false,"error":"database-locked"}`)
	}))
	defer srv.Close()

	// write info file with the server port
	tcpAddr := srv.Listener.Addr().String()
	idx := strings.LastIndex(tcpAddr, ":")
	portNum := 0
	fmt.Sscanf(tcpAddr[idx+1:], "%d", &portNum)
	infoPath := localAPIInfoPathGlobal()
	if err := os.MkdirAll(filepath.Dir(infoPath), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]interface{}{"port": portNum, "token": "good-token"})
	if err := os.WriteFile(infoPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(infoPath)

	res, err := callLocalAPI("ping", map[string]interface{}{"action": "ping"})
	if err != nil || res["success"] != true {
		t.Fatalf("callLocalAPI: %v %+v", err, res)
	}

	if _, err := callLocalAPI("get-logins", map[string]interface{}{"action": "get-logins"}); err != errLocalLocked {
		t.Fatalf("expected errLocalLocked, got %v", err)
	}

	// stale info file -> errLocalDown
	os.Remove(infoPath)
	if _, err := callLocalAPI("ping", nil); err != errLocalDown {
		t.Fatalf("expected errLocalDown, got %v", err)
	}

	// corrupt port -> errLocalDown
	raw, _ = json.Marshal(map[string]interface{}{"port": 99999, "token": "good-token"})
	os.WriteFile(infoPath, raw, 0o600)
	if _, err := callLocalAPI("ping", nil); err != errLocalDown {
		t.Fatalf("expected errLocalDown for bad port, got %v", err)
	}
	os.Remove(infoPath)
}

func TestBackupPrune(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	a := &App{}
	vDir := a.VaultDir()
	v, err := createVault(filepath.Join(vDir, "prune.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	defer v.close()

	// seed 12 stale backups
	backupDir := filepath.Join(vDir, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -30)
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("prune-202601%02d-000000.passapp", i+1)
		p := filepath.Join(backupDir, name)
		if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.backupVaultFile(v, "prune.passapp"); err != nil {
		t.Fatalf("backupVaultFile: %v", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > maxBackupsPerVault {
		t.Fatalf("prune left %d backups, want <= %d", len(entries), maxBackupsPerVault)
	}
}

func TestNativeHostManifestContent(t *testing.T) {
	exe := `C:\path\to\CipherSync.exe`
	raw, err := nativeHostManifest(exe, []string{"chrome-id-here"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := string(raw)
	if !strings.Contains(m, "com.ciphersync.host") {
		t.Fatalf("manifest missing host name: %s", m)
	}
	if !strings.Contains(m, "chrome-id-here") {
		t.Fatal("manifest missing chrome extension id")
	}
	// backslashes are JSON-escaped, so match a backslash-free fragment
	if !strings.Contains(m, "CipherSync.exe") {
		t.Fatal("manifest missing exe path")
	}
}

func TestParseIntDefault(t *testing.T) {
	if n, err := parseIntDefault("30", 5); err != nil || n != 30 {
		t.Fatalf("valid: %v %d", err, n)
	}
	if n, err := parseIntDefault("", 5); err != nil || n != 5 {
		t.Fatalf("empty must fall back to default: %v %d", err, n)
	}
	if n, err := parseIntDefault("-3", 5); err == nil || n != 5 {
		t.Fatalf("negative: %v %d", err, n)
	}
	if n, err := parseIntDefault("0", 5); err != nil || n != 0 {
		t.Fatalf("zero is valid: %v %d", err, n)
	}
}

func TestRandomTokenShape(t *testing.T) {
	tok := randomToken()
	if len(tok) != 43 {
		t.Fatalf("32 bytes base64url = 43 chars, got %d", len(tok))
	}
	if tok == randomToken() {
		t.Fatal("tokens must not repeat")
	}
}

func TestTOTPSecretDisplay(t *testing.T) {
	if got := totpSecretDisplay("JBSWY3DPEHPK3PXP"); !strings.Contains(got, " ") {
		t.Fatalf("expected grouped display, got %q", got)
	}
}
