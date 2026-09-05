package main

import (
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
)

var rpIDRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func isBase64URL(s string) bool {
	if s == "" {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil
}

// validatePasskey checks a passkey payload. Empty privateKey is allowed and
// means "reference only" (e.g. imported without private material) — the UI
// must badge it as requiring re-registration.
func validatePasskey(selfID string, p *PasskeyData, items []Item) error {
	if p == nil {
		return errors.New("dados da passkey ausentes")
	}
	rp := strings.ToLower(strings.TrimSpace(p.RpID))
	if rp == "" {
		return errors.New("RP ID é obrigatório (ex: github.com)")
	}
	if !rpIDRe.MatchString(rp) {
		return errors.New("RP ID inválido (use apenas o domínio, ex: github.com)")
	}
	if !isBase64URL(strings.TrimSpace(p.CredentialID)) {
		return errors.New("credential ID inválido (esperado base64url)")
	}
	if uh := strings.TrimSpace(p.UserHandle); uh != "" && !isBase64URL(uh) {
		return errors.New("user handle inválido (esperado base64url)")
	}
	cred := strings.TrimSpace(p.CredentialID)
	for _, it := range items {
		if it.ID == selfID || it.Deleted || it.Passkey == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(it.Passkey.RpID), rp) &&
			strings.TrimSpace(it.Passkey.CredentialID) == cred {
			return errors.New("esta passkey já existe no cofre")
		}
	}
	return nil
}
