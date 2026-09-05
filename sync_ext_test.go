package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestNativeFraming(t *testing.T) {
	var buf bytes.Buffer
	msg := map[string]interface{}{"action": "ping", "requestId": "r1"}
	if err := writeNativeMessage(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got, err := readNativeMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got["action"] != "ping" || got["requestId"] != "r1" {
		t.Fatalf("round trip failed: %+v", got)
	}
	if _, err := readNativeMessage(&buf); err == nil {
		t.Fatal("expected EOF on empty buffer")
	}
}

func TestNativePing(t *testing.T) {
	associated := false
	res := dispatchNative(map[string]interface{}{"action": "ping", "requestId": "x"}, &associated)
	if res["success"] != true || res["requestId"] != "x" {
		t.Fatalf("bad ping: %+v", res)
	}
	if associated {
		t.Fatal("ping must not associate")
	}
}

func TestNativeAssociateFlow(t *testing.T) {
	dir := t.TempDir()
	pairingsPathOverride = filepath.Join(dir, "pairings.json")
	defer func() { pairingsPathOverride = "" }()

	code, err := GeneratePairingCode()
	if err != nil || code == "" {
		t.Fatalf("code: %v %q", code, err)
	}
	associated := false
	res := dispatchNative(map[string]interface{}{"action": "associate", "code": code}, &associated)
	if res["success"] != true {
		t.Fatalf("associate failed: %+v", res)
	}
	if !associated {
		t.Fatal("should be associated")
	}
	id, _ := res["id"].(string)
	if id == "" {
		t.Fatal("missing assoc id")
	}
	// code is one-time
	associated2 := false
	res2 := dispatchNative(map[string]interface{}{"action": "associate", "code": code}, &associated2)
	if res2["success"] != false {
		t.Fatal("code reuse should fail")
	}
	// test-associate works
	associated3 := false
	res3 := dispatchNative(map[string]interface{}{"action": "test-associate", "id": id}, &associated3)
	if res3["success"] != true || !associated3 {
		t.Fatalf("test-associate failed: %+v", res3)
	}
}

func TestNativeRequiresAssoc(t *testing.T) {
	associated := false
	res := dispatchNative(map[string]interface{}{"action": "get-logins", "url": "https://x.com"}, &associated)
	if res["success"] != false || res["error"] != "not-associated" {
		t.Fatalf("expected not-associated: %+v", res)
	}
}

func TestSplitDriveRemote(t *testing.T) {
	f, n, err := splitDriveRemote("folderABC/a.passapp")
	if err != nil || f != "folderABC" || n != "a.passapp" {
		t.Fatalf("bad split: %q %q %v", f, n, err)
	}
	if _, _, err := splitDriveRemote("nope"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := splitDriveRemote("/x"); err == nil {
		t.Fatal("expected error")
	}
}
