package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// nativeHostManifest builds the host manifest JSON. allowedOrigins lists
// the extension IDs (chrome-extension://...) or Firefox add-on IDs.
func nativeHostManifest(exePath string, chromeOrigins, firefoxIDs []string) ([]byte, error) {
	m := map[string]interface{}{
		"name":        nativeHostName,
		"description": "CipherSync native messaging host",
		"path":        exePath,
		"type":        "stdio",
	}
	if len(chromeOrigins) > 0 {
		m["allowed_origins"] = chromeOrigins
	} else {
		m["allowed_origins"] = []string{}
	}
	if len(firefoxIDs) > 0 {
		m["allowed_extensions"] = firefoxIDs
	}
	return json.MarshalIndent(m, "", "  ")
}

func currentExePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Clean(p), nil
}

// linuxManifestDirs returns per-user native-messaging dirs for the
// supported browsers.
func linuxManifestDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	rel := []string{
		".config/google-chrome/NativeMessagingHosts",
		".config/chromium/NativeMessagingHosts",
		".config/BraveSoftware/Brave-Browser/NativeMessagingHosts",
		".mozilla/native-messaging-hosts",
	}
	var out []string
	for _, r := range rel {
		out = append(out, filepath.Join(home, r))
	}
	return out
}

// InstallNativeHost writes manifests (+ registry on Windows) so browsers
// can spawn CipherSync.exe as a native host. extID is the published
// extension ID (chrome-extension://...) or add-on ID (Firefox).
func (a *App) InstallNativeHost(chromeExtID, firefoxAddonID string) error {
	exe, err := currentExePath()
	if err != nil {
		return err
	}
	var chromeOrigins, firefoxIDs []string
	if chromeExtID != "" {
		chromeOrigins = []string{"chrome-extension://" + chromeExtID + "/"}
	}
	if firefoxAddonID != "" {
		firefoxIDs = []string{firefoxAddonID}
	}
	manifest, err := nativeHostManifest(exe, chromeOrigins, firefoxIDs)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return installNativeHostWindows(manifest)
	}
	return installNativeHostLinux(manifest)
}

// installNativeHostLinux writes the manifest JSON into each browser dir.
func installNativeHostLinux(manifest []byte) error {
	dirs := linuxManifestDirs()
	if len(dirs) == 0 {
		return errors.New("home não encontrado")
	}
	var firstErr error
	ok := 0
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(d, nativeHostName+".json"), manifest, 0o644); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ok++
	}
	if ok == 0 && firstErr != nil {
		return firstErr
	}
	return nil
}

// UninstallNativeHost removes manifests and registry entries.
func (a *App) UninstallNativeHost() error {
	if runtime.GOOS == "windows" {
		return uninstallNativeHostWindows()
	}
	for _, d := range linuxManifestDirs() {
		_ = os.Remove(filepath.Join(d, nativeHostName+".json"))
	}
	return nil
}

// GeneratePairingCode creates a one-time pairing code for the extension.
func (a *App) GeneratePairingCode() (string, error) {
	if !a.IsUnlocked() {
		return "", ErrVaultLocked
	}
	return GeneratePairingCode()
}
