//go:build !windows

package main

// startQuickAccessHotkey is only supported on Windows for now.
func startQuickAccessHotkey(onHotkey func()) (uint32, error) {
	return 0, errQuickAccessUnsupported
}

// stopQuickAccessHotkey is a no-op off Windows.
func stopQuickAccessHotkey(threadID uint32) {}
