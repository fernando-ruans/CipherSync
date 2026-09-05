//go:build !windows

package main

import "errors"

// installNativeHostWindows is Windows-only.
func installNativeHostWindows(manifest []byte) error {
	return errors.New("disponível apenas no Windows")
}

func uninstallNativeHostWindows() error {
	return errors.New("disponível apenas no Windows")
}
