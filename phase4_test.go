package main

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPGeneration(t *testing.T) {
	secret, uri, err := generateTOTPKey("CipherSync", "ana@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) < 16 {
		t.Fatalf("secret too short: %q", secret)
	}
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("bad otpauth URI: %s", uri)
	}
	code, remaining, err := totpCode(secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", code)
	}
	if remaining <= 0 || remaining > 30 {
		t.Fatalf("expected seconds remaining in 1..30, got %d", remaining)
	}
	if err := validateTOTPSecret(secret); err != nil {
		t.Fatal(err)
	}
	if err := validateTOTPSecret("not-a-secret!!"); err == nil {
		t.Fatal("expected invalid secret to fail")
	}
}

func TestTOTPURIparse(t *testing.T) {
	secret, uri, err := generateTOTPKey("GitHub", "ana")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseTOTPSecretFromURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != secret {
		t.Fatalf("parsed %q want %q", parsed, secret)
	}
	if _, err := parseTOTPSecretFromURI("https://github.com"); err == nil {
		t.Fatal("expected non-otpauth URI to fail")
	}
	if _, err := parseTOTPSecretFromURI("otpauth://totp/X?secret="); err == nil {
		t.Fatal("expected empty secret to fail")
	}
}

func TestTOTPQRGeneration(t *testing.T) {
	_, uri, _ := generateTOTPKey("Site", "user")
	qr, err := generateTOTPQR(uri)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(qr, "data:image/png;base64,") {
		t.Fatalf("bad QR data URI: %s", qr[:30])
	}
}

func TestGoPasswordScore(t *testing.T) {
	if goPasswordScore("") != 0 {
		t.Fatal("empty should score 0")
	}
	if goPasswordScore("password") != 0 {
		t.Fatal("common password should score 0")
	}
	if goPasswordScore("123456") != 0 {
		t.Fatal("common numeric password should score 0")
	}
	strong := "K7#mPq2!vXz9"
	if goPasswordScore(strong) < 3 {
		t.Fatalf("strong password scored too low: %d", goPasswordScore(strong))
	}
}

func TestAnalyzeVault(t *testing.T) {
	v := setupVault(t)
	defer v.close()

	// weak password
	_, _ = v.create(Item{Type: TypeLogin, Title: "Weak", Password: "123456", URL: "https://a.com"})
	// duplicate passwords (two items share it)
	_, _ = v.create(Item{Type: TypeLogin, Title: "DupA", Password: "SharedPass1!", URL: "https://b.com"})
	_, _ = v.create(Item{Type: TypeLogin, Title: "DupB", Password: "SharedPass1!", URL: "https://c.com"})
	// old password (updated long ago)
	old, _ := v.create(Item{Type: TypeLogin, Title: "Old", Password: "OldPass123!", URL: "https://d.com"})
	old.UpdatedAt = time.Now().Add(-2 * 365 * 24 * time.Hour).UnixMilli()
	if err := v.persistItem(old); err != nil {
		t.Fatal(err)
	}
	for i := range v.items {
		if v.items[i].ID == old.ID {
			v.items[i] = old
			v.itemsBy[old.ID] = &v.items[i]
			break
		}
	}
	// has 2FA
	_, _ = v.create(Item{Type: TypeLogin, Title: "Secure", Password: "S3cur3#Pass!", URL: "https://e.com", TotpSecret: "JBSWY3DPEHPK3PXP"})

	oldChecker := breachChecker
	breachChecker = func(pw []string, w int) map[string]int { return map[string]int{} }
	defer func() { breachChecker = oldChecker }()

	report := analyzeVault(v.list())
	if report.WeakCount != 1 {
		t.Fatalf("expected 1 weak, got %d", report.WeakCount)
	}
	if report.DuplicateCount != 1 {
		t.Fatalf("expected 1 duplicate, got %d", report.DuplicateCount)
	}
	if report.OldCount != 1 {
		t.Fatalf("expected 1 old, got %d", report.OldCount)
	}
	if report.Missing2FA != 4 { // Weak, DupA, DupB, Old have URL but no TOTP
		t.Fatalf("expected missing 2FA count 4, got %d", report.Missing2FA)
	}
	if report.TotalScore >= 100 {
		t.Fatal("score should be penalized")
	}
}

func TestBreachCacheAndLookup(t *testing.T) {
	// Verify our suffix matching works on a known breached password.
	// "password" is the most common breached password in HIBP.
	breached, count, err := checkBreach("password")
	if err != nil {
		// no network in tests; skip silently
		t.Skip("no network:", err)
	}
	if !breached || count == 0 {
		t.Fatalf("expected 'password' to be breached, got %v %d", breached, count)
	}
}
