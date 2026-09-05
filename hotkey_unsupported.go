//go:build !windows

package main

// registerQuickAccessHotkey is only supported on Windows for now.
func registerQuickAccessHotkey() error {
	return errQuickAccessUnsupported
}

// unregisterQuickAccessHotkey is a no-op off Windows.
func unregisterQuickAccessHotkey() {}

// quickAccessLoop never fires off Windows.
func quickAccessLoop(onHotkey func()) {}
