//go:build windows

package main

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32               = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey      = modUser32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = modUser32.NewProc("UnregisterHotKey")
	procGetMessageW         = modUser32.NewProc("GetMessageW")
	procTranslateMessage    = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW    = modUser32.NewProc("DispatchMessageW")
)

const (
	hotkeyModControl  = 0x0002
	hotkeyModShift    = 0x0004
	hotkeyModNoRepeat = 0x4000
	hotkeyVKSpace     = 0x20
	hotkeyWMHotkey    = 0x0312
	quickAccessHotID  = 0xC1FE
)

// winMsg mirrors the Win32 MSG struct (64-bit layout).
type winMsg struct {
	Hwnd    uintptr
	Message uint32
	_       uint32 // padding
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

// registerQuickAccessHotkey registers Ctrl+Shift+Space system-wide.
// Fails if another application already owns the combination.
func registerQuickAccessHotkey() error {
	r, _, err := procRegisterHotKey.Call(
		0,
		quickAccessHotID,
		hotkeyModControl|hotkeyModShift|hotkeyModNoRepeat,
		hotkeyVKSpace,
	)
	if r == 0 {
		if errno, ok := err.(windows.Errno); ok && errno != 0 {
			return errors.New("atalho global já está em uso por outro aplicativo")
		}
		return errors.New("não foi possível registrar o atalho global")
	}
	return nil
}

func unregisterQuickAccessHotkey() {
	_, _, _ = procUnregisterHotKey.Call(0, quickAccessHotID)
}

// quickAccessLoop runs a message pump on its own thread and invokes
// onHotkey every time the registered combination is pressed.
// With hWnd=NULL the WM_HOTKEY goes to this thread's queue, which is why
// it must NOT run on Wails' main thread.
func quickAccessLoop(onHotkey func()) {
	var msg winMsg
	for {
		r, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if int32(r) <= 0 {
			return
		}
		if msg.Message == hotkeyWMHotkey && msg.WParam == quickAccessHotID {
			onHotkey()
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
