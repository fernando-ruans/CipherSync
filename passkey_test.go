package main

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
)

func validPasskey(rp, cred string) *PasskeyData {
	return &PasskeyData{RpID: rp, CredentialID: cred}
}

func TestValidatePasskey(t *testing.T) {
	items := []Item{
		{ID: "a", Passkey: validPasskey("github.com", "cred-1")},
	}

	cases := []struct {
		name    string
		selfID  string
		p       *PasskeyData
		wantErr bool
	}{
		{"nil data", "", nil, true},
		{"empty rp", "", validPasskey("", "cred"), true},
		{"empty cred", "", validPasskey("github.com", ""), true},
		{"valid", "", validPasskey("github.com", "cred-2"), false},
		{"subdomain rp ok", "", validPasskey("gist.github.com", "cred-2"), false},
		{"rp with scheme", "", validPasskey("https://github.com", "cred-2"), true},
		{"rp with path", "", validPasskey("github.com/login", "cred-2"), true},
		{"rp uppercase", "", validPasskey("GitHub.com", "cred-2"), false},
		{"cred not base64url", "", validPasskey("github.com", "not base64!!"), true},
		{"duplicate pair", "b", validPasskey("github.com", "cred-1"), true},
		{"same pair same item", "a", validPasskey("github.com", "cred-1"), false},
		{"duplicate on trashed item", "", validPasskey("github.com", "cred-1"), false}, // handled below with Deleted
		{"invalid user handle", "", &PasskeyData{RpID: "x.com", CredentialID: "c1", UserHandle: "@@"}, true},
		{"valid user handle", "", &PasskeyData{RpID: "x.com", CredentialID: "c1", UserHandle: "abc"}, false},
	}
	for _, tc := range cases {
		testItems := items
		if tc.name == "duplicate on trashed item" {
			testItems = []Item{{ID: "a", Deleted: true, Passkey: validPasskey("github.com", "cred-1")}}
		}
		err := validatePasskey(tc.selfID, tc.p, testItems)
		if tc.wantErr && err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
	}
}

func TestImportValidatesPasskey(t *testing.T) {
	dir := t.TempDir()
	v, err := createVault(filepath.Join(dir, "v.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	defer v.close()

	// pre-existing passkey credential
	if _, err := v.create(Item{Title: "GH", Passkey: validPasskey("github.com", "cred-1")}); err != nil {
		t.Fatal(err)
	}

	res := v.importItems([]Item{
		{Title: "Dup", Passkey: validPasskey("github.com", "cred-1")}, // duplicate pair
		{Title: "Bad", Passkey: validPasskey("https://bad.example", "cred-x")},
		{Title: "Ok", Passkey: validPasskey("gitlab.com", "cred-2")},
	})
	if res.Created != 1 || len(res.Errors) != 2 {
		t.Fatalf("expected 1 created + 2 errors, got %+v", res)
	}
	if len(v.list()) != 2 {
		t.Fatalf("expected 2 items total, got %d", len(v.list()))
	}
}

func TestRestoreVersionValidatesPasskey(t *testing.T) {
	dir := t.TempDir()
	v, err := createVault(filepath.Join(dir, "v.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	defer v.close()

	it, err := v.create(Item{Title: "GH", Passkey: validPasskey("github.com", "cred-old")})
	if err != nil {
		t.Fatal(err)
	}
	// change passkey to a new credential (creates a version with the old one)
	if err := v.update(Item{ID: it.ID, Type: TypePasskey, Title: "GH", Passkey: validPasskey("github.com", "cred-new")}); err != nil {
		t.Fatal(err)
	}
	// second item claims the old credential
	if _, err := v.create(Item{Title: "Other", Passkey: validPasskey("github.com", "cred-old")}); err != nil {
		t.Fatal(err)
	}

	versions, err := v.getVersions(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least one version")
	}
	// restoring the version with cred-old must be rejected (duplicate)
	if _, err := v.restoreVersion(versions[len(versions)-1].ID); err == nil {
		t.Fatal("expected duplicate passkey restore to be rejected")
	}
}

func TestExportJSONStripsPrivateKey(t *testing.T) {
	items := []Item{{
		Title: "GH",
		Passkey: &PasskeyData{
			RpID:       "github.com",
			CredentialID: "cred-1",
			PrivateKey: "SUPER-SECRET-KEY-MATERIAL",
		},
	}}
	out, err := exportJSON(items)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SUPER-SECRET-KEY-MATERIAL") {
		t.Fatal("exportJSON leaked passkey private key")
	}
	if !strings.Contains(out, "github.com") {
		t.Fatal("rpId should still be exported")
	}
	// the input must not be mutated
	if items[0].Passkey.PrivateKey != "SUPER-SECRET-KEY-MATERIAL" {
		t.Fatal("exportJSON mutated the input items")
	}
}

func TestOpenTransferRejectsHugePLen(t *testing.T) {
	// header: magic + version 1 + pLen = 0xFFFFFFFF
	buf := []byte{}
	buf = append(buf, []byte(transferMagic)...)
	buf = append(buf, 1)
	buf = append(buf, 0xFF, 0xFF, 0xFF, 0xFF)
	data := base64.StdEncoding.EncodeToString(buf)
	if _, err := openTransfer(data, "pw"); err == nil {
		t.Fatal("expected rejection of malicious pLen")
	}
}
