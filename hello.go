package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func helloBlobName(vaultFile string) string {
	return "hello-" + strings.TrimSuffix(vaultFile, ".passapp") + ".blob"
}

func (a *App) helloBlobPath(vaultFile string) string {
	return filepath.Join(a.VaultDir(), helloBlobName(vaultFile))
}

// IsHelloAvailable reports whether Windows Hello unlock is supported.
func (a *App) IsHelloAvailable() bool {
	return helloAvailable()
}

// IsHelloEnabled reports whether Hello unlock is configured for the current vault.
func (a *App) IsHelloEnabled() bool {
	if a.vaultFile == "" {
		return false
	}
	return fileExists(a.helloBlobPath(a.vaultFile))
}

// EnableHello protects the current vault key with the Windows session (DPAPI).
func (a *App) EnableHello() error {
	if !a.IsUnlocked() {
		return ErrVaultLocked
	}
	blob, err := protectForHello(a.vault.vaultKey, a.vaultFile)
	if err != nil {
		return err
	}
	return os.WriteFile(a.helloBlobPath(a.vaultFile), blob, 0o600)
}

// DisableHello removes the stored Hello blob.
func (a *App) DisableHello() error {
	path := a.helloBlobPath(a.vaultFile)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HelloUnlock unlocks the given vault using the stored Windows Hello blob.
func (a *App) HelloUnlock(file string) (bool, error) {
	if a.IsUnlocked() {
		return false, errors.New("a vault is already unlocked")
	}
	blobPath := filepath.Join(a.VaultDir(), helloBlobName(file))
	blob, err := os.ReadFile(blobPath)
	if err != nil {
		return false, nil
	}
	key, err := unprotectWithHello(blob)
	if err != nil {
		return false, err
	}
	defer wipe(key)
	v, err := openVaultWithKey(filepath.Join(a.VaultDir(), file), key)
	if err != nil {
		return false, err
	}
	a.vault = v
	a.vaultFile = file
	path := filepath.Join(a.VaultDir(), file)
	if name, err := readVaultName(path); err == nil && name != "" {
		a.vaultName = name
	} else {
		a.vaultName = strings.TrimSuffix(file, ".passapp")
	}
	return true, nil
}
