//go:build !windows

package main

// startQuickAccessHotkey is only supported on Windows for now.
func startQuickAccessHotkey(onHotkey func()) (uint32, <-chan struct{}, error) {
	return 0, nil, errQuickAccessUnsupported
}

// stopQuickAccessHotkey is a no-op off Windows.
func stopQuickAccessHotkey(threadID uint32, finished <-chan struct{}) {}
