package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

const nativeHostName = "com.ciphersync.host"

// isNativeHostInvocation detects a browser-spawned launch: either the
// explicit flag, or piped stdin plus a non-file argument. Browsers always
// pipe stdin and pass non-file args (extension origin, browser flags);
// GUI launches with args almost always pass an existing file path
// (e.g. "Open with" on a vault), which keeps the GUI alive.
func isNativeHostInvocation() bool {
	for _, a := range os.Args[1:] {
		if a == "--native-host" {
			return true
		}
	}
	if isatty.IsTerminal(os.Stdin.Fd()) {
		return false
	}
	for _, a := range os.Args[1:] {
		if a == "" {
			continue
		}
		if _, err := os.Stat(a); err == nil {
			continue // existing file: GUI-style launch argument
		}
		return true
	}
	return false
}

// runNativeHost is the browser extension entrypoint:
// CipherSync.exe --native-host
// Speaks Chrome/Firefox native messaging on stdin/stdout and forwards to
// the GUI's loopback API (only while a vault is unlocked).
func runNativeHost() {
	// a single persistent connection requires associate/test-associate first
	associated := false
	for {
		req, err := readNativeMessage(os.Stdin)
		if err != nil {
			return // EOF or corrupt: browser closed the port
		}
		resp := dispatchNative(req, &associated)
		if err := writeNativeMessage(os.Stdout, resp); err != nil {
			return
		}
	}
}

func readNativeMessage(r io.Reader) (map[string]interface{}, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	if n == 0 || n > 64<<20 {
		return nil, fmt.Errorf("bad length %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(buf, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func writeNativeMessage(w io.Writer, v interface{}) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		raw, _ = json.Marshal(map[string]interface{}{"success": false, "error": "too-large"})
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(raw)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}

func strField(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func dispatchNative(req map[string]interface{}, associated *bool) map[string]interface{} {
	requestID, _ := req["requestId"].(string)
	action, _ := req["action"].(string)
	reply := func(payload map[string]interface{}) map[string]interface{} {
		if requestID != "" {
			payload["requestId"] = requestID
		}
		return payload
	}

	switch action {
	case "ping":
		return reply(map[string]interface{}{"success": true, "version": "1"})

	case "associate":
		code := strings.TrimSpace(strField(req, "code"))
		id, err := pairingsConsume(code)
		if err != nil {
			return reply(map[string]interface{}{"success": false, "error": "bad-code"})
		}
		*associated = true
		return reply(map[string]interface{}{"success": true, "id": id})

	case "test-associate":
		id := strField(req, "id")
		if pairingsCheck(id) {
			*associated = true
			return reply(map[string]interface{}{"success": true})
		}
		return reply(map[string]interface{}{"success": false, "error": "not-associated"})
	}

	if !*associated {
		return reply(map[string]interface{}{"success": false, "error": "not-associated"})
	}

	// forward everything else to the GUI loopback API
	res, err := callLocalAPI(action, req)
	if err != nil {
		if err == errLocalLocked {
			return reply(map[string]interface{}{"success": false, "error": "database-locked", "message": "Unlock CipherSync"})
		}
		if err == errLocalDown {
			return reply(map[string]interface{}{"success": false, "error": "not-running", "message": "CipherSync is not running"})
		}
		return reply(map[string]interface{}{"success": false, "error": "error"})
	}
	res["requestId"] = requestID
	// never leak more than one screen of logins
	if logs, ok := res["logins"].([]interface{}); ok && len(logs) > 50 {
		res["logins"] = logs[:50]
	}
	return res
}

var errLocalLocked = fmt.Errorf("database-locked")
var errLocalDown = fmt.Errorf("not-running")

func callLocalAPI(action string, req map[string]interface{}) (map[string]interface{}, error) {
	infoRaw, err := os.ReadFile(localAPIInfoPathGlobal())
	if err != nil {
		return nil, errLocalDown
	}
	var info struct {
		Port  int    `json:"port"`
		Token string `json:"token"`
	}
	// port must be a sane TCP port; anything else means a corrupt file
	if err := json.Unmarshal(infoRaw, &info); err != nil || info.Port < 1 || info.Port > 65535 || info.Token == "" {
		return nil, errLocalDown
	}
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/", info.Port), bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Token", info.Token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errLocalDown
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errLocalDown
	}
	var out map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if ok, _ := out["success"].(bool); !ok {
		if out["error"] == "database-locked" {
			return nil, errLocalLocked
		}
	}
	return out, nil
}

func localAPIInfoPathGlobal() string {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "ciphersync", localAPITokenFile)
	}
	return filepath.Join(cfg, "LockSync", localAPITokenFile)
}

// ---------- pairing store (VaultDir/pairings.json) ----------

// pairingsPathOverride allows tests to redirect the pairing store.
var pairingsPathOverride = ""

// pairingsMu serializes load-modify-save cycles across host/GUI processes
// that could otherwise lose codes or ids.
var pairingsMu sync.Mutex

// pairingPendingTTL is how long a generated code can be redeemed.
const pairingPendingTTL = 10 * time.Minute

func pairingsPath() string {
	if pairingsPathOverride != "" {
		return pairingsPathOverride
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "ciphersync", "pairings.json")
	}
	return filepath.Join(cfg, "LockSync", "pairings.json")
}

func pairingsLoad() map[string]string {
	m := map[string]string{}
	raw, err := os.ReadFile(pairingsPath())
	if err != nil {
		return m
	}
	_ = json.Unmarshal(raw, &m)
	return m
}

func pairingsSave(m map[string]string) {
	raw, _ := json.Marshal(m)
	_ = os.WriteFile(pairingsPath(), raw, 0o600)
}

// gcPairings drops expired pending codes so the store does not grow forever.
func gcPairings(m map[string]string) {
	now := time.Now().UnixMilli()
	for k, v := range m {
		if !strings.HasPrefix(k, "pending:") {
			continue
		}
		if exp, ok := parsePendingValue(v); ok && now > exp {
			delete(m, k)
		}
	}
}

// parsePendingValue reads the expiry encoded in a pending entry
// ("pending:<unix-millis>"). Legacy entries ("pending") never expire.
func parsePendingValue(v string) (int64, bool) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 || parts[0] != "pending" {
		return 0, false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return exp, true
}

// GeneratePairingCode creates a one-time code shown in Settings for the user
// to paste into the browser extension. The code expires after 10 minutes.
func GeneratePairingCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := strings.ToUpper(base64.RawURLEncoding.EncodeToString(b))[:8]
	pairingsMu.Lock()
	defer pairingsMu.Unlock()
	m := pairingsLoad()
	gcPairings(m)
	m["pending:"+code] = fmt.Sprintf("pending:%d", time.Now().Add(pairingPendingTTL).UnixMilli())
	pairingsSave(m)
	return code, nil
}

// pairingsConsume validates a one-time code and binds an association id.
func pairingsConsume(code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("bad code")
	}
	pairingsMu.Lock()
	defer pairingsMu.Unlock()
	m := pairingsLoad()
	v := m["pending:"+code]
	if v == "" {
		return "", fmt.Errorf("bad code")
	}
	if exp, ok := parsePendingValue(v); ok && time.Now().UnixMilli() > exp {
		delete(m, "pending:"+code)
		pairingsSave(m)
		return "", fmt.Errorf("bad code")
	}
	delete(m, "pending:"+code)
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	id := "assoc-" + base64.RawURLEncoding.EncodeToString(b)
	m[id] = "paired"
	pairingsSave(m)
	return id, nil
}

func pairingsCheck(id string) bool {
	if id == "" {
		return false
	}
	pairingsMu.Lock()
	defer pairingsMu.Unlock()
	m := pairingsLoad()
	return m[id] == "paired"
}
