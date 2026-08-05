package main

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

const totpPeriod = 30

// generateTOTPKey creates a fresh TOTP secret and its otpauth:// URL.
func generateTOTPKey(issuer, account string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Period:      totpPeriod,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// generateTOTPQR returns a QR code (PNG base64 data URI) for an otpauth URL.
func generateTOTPQR(otpauthURL string) (string, error) {
	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 320)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// totpCode computes the current code and seconds until it rotates.
func totpCode(secret string) (code string, secondsRemaining int, err error) {
	code, err = totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", 0, err
	}
	secondsRemaining = totpPeriod - int(time.Now().Unix()%totpPeriod)
	return code, secondsRemaining, nil
}

// validateTOTPSecret checks a user-supplied secret key.
func validateTOTPSecret(secret string) error {
	secret = strings.TrimSpace(strings.ToUpper(secret))
	if secret == "" {
		return errors.New("informe a chave secreta")
	}
	key, err := totp.GenerateCode(secret, time.Now())
	if err != nil || key == "" {
		return errors.New("chave secreta inválida")
	}
	return nil
}

// parseTOTPSecretFromURI extracts the secret from an otpauth:// URL
// (e.g. scanned from a QR code) and validates it.
func parseTOTPSecretFromURI(uri string) (secret string, err error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", errors.New("nada detectado na imagem")
	}
	u, err := url.Parse(uri)
	if err != nil {
		return "", errors.New("código QR inválido")
	}
	if u.Scheme != "otpauth" {
		return "", errors.New("o QR code não é do tipo otpauth (TOTP)")
	}
	secret = u.Query().Get("secret")
	if secret == "" {
		return "", errors.New("QR code sem campo secret")
	}
	if err := validateTOTPSecret(secret); err != nil {
		return "", err
	}
	return secret, nil
}

// totpSecretDisplay returns a masked representation of a secret for display.
func totpSecretDisplay(secret string) string {
	if len(secret) <= 8 {
		return strings.Repeat("•", len(secret))
	}
	return secret[:4] + " " + strings.Repeat("•", len(secret)-8) + " " + secret[len(secret)-4:]
}

// otpIssuerForItem builds the issuer name used in generated TOTP QR codes.
func otpIssuerForItem(title, username string) (issuer, account string) {
	issuer = title
	if issuer == "" {
		issuer = "CipherSync"
	}
	account = username
	if account == "" {
		account = issuer
	}
	return issuer, account
}
