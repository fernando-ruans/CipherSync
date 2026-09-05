//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

var nativeHostRegKeys = []string{
	`SOFTWARE\Google\Chrome\NativeMessagingHosts\` + nativeHostName,
	`SOFTWARE\Chromium\NativeMessagingHosts\` + nativeHostName,
	`SOFTWARE\BraveSoftware\Brave-Browser\NativeMessagingHosts\` + nativeHostName,
	`SOFTWARE\Microsoft\Edge\NativeMessagingHosts\` + nativeHostName,
	`Software\Mozilla\NativeMessagingHosts\` + nativeHostName,
}

// installNativeHostWindows writes the manifest next to the exe and points
// HKCU registry keys at it (no admin required).
func installNativeHostWindows(manifest []byte) error {
	exe, err := currentExePath()
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(filepath.Dir(exe), nativeHostName+".json")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		return err
	}
	for _, sub := range nativeHostRegKeys {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, sub, registry.SET_VALUE)
		if err != nil {
			continue
		}
		_ = k.SetStringValue("", manifestPath)
		k.Close()
	}
	return nil
}

func uninstallNativeHostWindows() error {
	exe, err := currentExePath()
	if err == nil {
		_ = os.Remove(filepath.Join(filepath.Dir(exe), nativeHostName+".json"))
	}
	for _, sub := range nativeHostRegKeys {
		_ = registry.DeleteKey(registry.CURRENT_USER, sub)
	}
	return nil
}
