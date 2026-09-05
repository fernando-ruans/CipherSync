package main

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

func TestImportCommitViaApp(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer v.close()

	res, err := a.ImportCommit([]Item{
		{Title: "GH", Username: "u", Password: "p"},
		{Title: "GL", Username: "u", Password: "p"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 2 {
		t.Fatalf("expected 2 created, got %+v", res)
	}

	// duplicate (title+username) is skipped
	res, err = a.ImportCommit([]Item{{Title: "gh", Username: "U", Password: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 0 || res.Skipped != 1 {
		t.Fatalf("expected dedupe skip, got %+v", res)
	}

	// import with duplicate passkey must record an error, not create
	res, err = a.ImportCommit([]Item{
		{Title: "PK", Passkey: validPasskey("github.com", "cred-1")},
		{Title: "PK2", Passkey: validPasskey("github.com", "cred-1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || len(res.Errors) != 1 {
		t.Fatalf("expected 1 created + 1 error, got %+v", res)
	}
}

func TestExportJSONViaAppStripsPrivateKey(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer v.close()

	if _, err := v.create(Item{
		Type:  TypePasskey,
		Title: "GH",
		Passkey: &PasskeyData{
			RpID:         "github.com",
			CredentialID: "cred-1",
			PrivateKey:   "TOP-SECRET-MATERIAL",
		},
	}); err != nil {
		t.Fatal(err)
	}
	out, err := a.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "TOP-SECRET-MATERIAL") {
		t.Fatal("App.ExportJSON leaked passkey private key")
	}
}

func TestExportEncryptedJSONRoundTripKeepsPasskey(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer v.close()

	if _, err := v.create(Item{
		Type:  TypePasskey,
		Title: "GH",
		Passkey: &PasskeyData{
			RpID:         "github.com",
			CredentialID: "cred-1",
			PrivateKey:   "ENC-SECRET",
		},
	}); err != nil {
		t.Fatal(err)
	}
	sealed, err := a.ExportEncryptedJSON("transfer-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	items, err := openTransfer(sealed, "transfer-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Passkey == nil || items[0].Passkey.PrivateKey != "ENC-SECRET" {
		t.Fatalf("transfer lost passkey: %+v", items)
	}
	// wrong password must fail
	if _, err := openTransfer(sealed, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestOpenTransferCorruptInputs(t *testing.T) {
	// not base64
	if _, err := openTransfer("not base64!!", "pw"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
	// wrong magic
	buf := []byte("XXXXX")
	buf = append(buf, 1, 0, 0, 0, 0)
	if _, err := openTransfer(base64.StdEncoding.EncodeToString(buf), "pw"); err == nil {
		t.Fatal("expected error for wrong magic")
	}
	// legacy magic still accepted structurally (fails later on bad pLen=0)
	buf = []byte("LKSYNC")
	buf = append(buf, 1, 0, 0, 0, 1)
	buf = append(buf, '{', '}') // params json (not valid kdf)
	if _, err := openTransfer(base64.StdEncoding.EncodeToString(buf), "pw"); err == nil {
		t.Fatal("expected error for truncated payload")
	}
	// huge pLen must be rejected without allocating
	buf = []byte(transferMagic)
	buf = append(buf, 1)
	var pLen uint32 = 1 << 30
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], pLen)
	buf = append(buf, l[:]...)
	if _, err := openTransfer(base64.StdEncoding.EncodeToString(buf), "pw"); err == nil {
		t.Fatal("expected error for oversized pLen")
	}
}
