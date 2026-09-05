package main

import (
	"strings"
	"testing"
)

func TestTOTPAppBindings(t *testing.T) {
	a, v := newTestAppWithVault(t)
	defer v.close()

	// GetTOTPCodeForSecret is vault-independent and must work without a vault
	locked := &App{}
	if _, err := locked.GetTOTPCodeForSecret("JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatalf("GetTOTPCodeForSecret must work without a vault: %v", err)
	}

	// GenerateTOTPSetup requires an existing item
	it, err := v.create(Item{Type: TypeLogin, Title: "GH"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := a.GenerateTOTPSetup(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Secret == "" || info.OtpauthURL == "" || info.QR == "" {
		t.Fatalf("incomplete setup info: %+v", info)
	}
	if !strings.Contains(info.OtpauthURL, "otpauth://totp") {
		t.Fatalf("unexpected otpauth url: %s", info.OtpauthURL)
	}

	// code from the generated secret matches the raw computation
	code, err := a.GetTOTPCodeForSecret(info.Secret)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := totpCode(info.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != raw || code.SecondsRemaining <= 0 {
		t.Fatalf("code mismatch: %+v vs %s", code, raw)
	}

	// invalid secret rejected
	if _, err := a.GetTOTPCodeForSecret("not-base32!!!"); err == nil {
		t.Fatal("expected error for invalid secret")
	}
	if err := a.ValidateTOTPSecret(info.Secret); err != nil {
		t.Fatalf("valid secret rejected: %v", err)
	}
	if err := a.ValidateTOTPSecret(""); err == nil {
		t.Fatal("empty secret must be rejected")
	}

	// IngestTOTPURI extracts the secret
	uri := "otpauth://totp/GitHub:me?secret=" + info.Secret + "&issuer=GitHub"
	secret, err := a.IngestTOTPURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(secret, info.Secret) {
		t.Fatalf("ingested secret mismatch: %s vs %s", secret, info.Secret)
	}
	if _, err := a.IngestTOTPURI("https://not-otpauth.example"); err == nil {
		t.Fatal("non-otpauth URI must be rejected")
	}

	// attach + GetTOTPCode for the item
	item2 := it
	item2.TotpSecret = info.Secret
	if err := v.update(item2); err != nil {
		t.Fatal(err)
	}
	code2, err := a.GetTOTPCode(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if code2.Code != raw {
		t.Fatalf("item code mismatch: %s vs %s", code2.Code, raw)
	}

	// item without 2FA
	it3, err := v.create(Item{Type: TypeLogin, Title: "No2FA"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetTOTPCode(it3.ID); err == nil {
		t.Fatal("expected error for item without 2FA")
	}
}
