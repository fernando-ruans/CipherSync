package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPairingCodeExpiry(t *testing.T) {
	dir := t.TempDir()
	pairingsPathOverride = filepath.Join(dir, "pairings.json")
	defer func() { pairingsPathOverride = "" }()

	// fresh code works
	code, err := GeneratePairingCode()
	if err != nil || code == "" {
		t.Fatalf("generate: %v %q", err, code)
	}
	if _, err := pairingsConsume(code); err != nil {
		t.Fatalf("consume fresh code: %v", err)
	}

	// expired code is rejected
	expired := "EXPIRED1"
	m := map[string]string{}
	m["pending:"+expired] = "pending:" + itoaMillis(time.Now().Add(-time.Minute))
	pairingsSave(m)
	if _, err := pairingsConsume(expired); err == nil {
		t.Fatal("expired code must be rejected")
	}

	// legacy pending value (no expiry) still redeemable — backwards compat
	legacy := "LEGACY01"
	m2 := map[string]string{}
	m2["pending:"+legacy] = "pending"
	pairingsSave(m2)
	if _, err := pairingsConsume(legacy); err != nil {
		t.Fatalf("legacy code must still work: %v", err)
	}
}

func TestGcPairingsRemovesExpired(t *testing.T) {
	m := map[string]string{
		"pending:OLD":   "pending:" + itoaMillis(time.Now().Add(-time.Hour)),
		"pending:NEW":   "pending:" + itoaMillis(time.Now().Add(time.Hour)),
		"pending:LEG":   "pending",
		"assoc-abc":     "paired",
	}
	gcPairings(m)
	if _, ok := m["pending:OLD"]; ok {
		t.Fatal("expired pending must be collected")
	}
	if _, ok := m["pending:NEW"]; !ok {
		t.Fatal("valid pending must be kept")
	}
	if _, ok := m["pending:LEG"]; !ok {
		t.Fatal("legacy pending must be kept")
	}
	if m["assoc-abc"] != "paired" {
		t.Fatal("paired ids must be kept")
	}
}

func TestPairingGenerateTriggersGc(t *testing.T) {
	dir := t.TempDir()
	pairingsPathOverride = filepath.Join(dir, "pairings.json")
	defer func() { pairingsPathOverride = "" }()

	// seed an expired pending
	m := map[string]string{"pending:OLD": "pending:" + itoaMillis(time.Now().Add(-time.Hour))}
	pairingsSave(m)

	if _, err := GeneratePairingCode(); err != nil {
		t.Fatal(err)
	}
	loaded := pairingsLoad()
	if _, ok := loaded["pending:OLD"]; ok {
		t.Fatal("GeneratePairingCode should garbage-collect expired codes")
	}
}

func itoaMillis(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func TestWriteNativeMessageOversize(t *testing.T) {
	var buf bytes.Buffer
	big := strings.Repeat("x", 2<<20) // 2 MB payload > 1 MB limit
	if err := writeNativeMessage(&buf, map[string]interface{}{"data": big}); err != nil {
		t.Fatal(err)
	}
	// must produce a framed error response, not a truncated/oversized frame
	got, err := readNativeMessage(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got["success"] != false || got["error"] != "too-large" {
		t.Fatalf("expected too-large error response: %+v", got)
	}
	if buf.Len() != 0 {
		t.Fatal("no trailing bytes expected")
	}
}

func TestLocalAPIStaleTokenCleanup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	a := &App{}
	// simulate a stale token file from a previous crashed session
	if err := os.MkdirAll(a.VaultDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(a.VaultDir(), localAPITokenFile)
	if err := os.WriteFile(stale, []byte(`{"port":1,"token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// stopLocalAPI removes the file even when the server is not running
	a.stopLocalAPI()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale token file must be removed")
	}
}
