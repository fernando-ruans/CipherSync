//go:build windows

package main

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey   = modUser32.NewProc("RegisterHotKey")
	procUnregisterHotKey = modUser32.NewProc("UnregisterHotKey")
	procGetMessageW      = modUser32.NewProc("GetMessageW")
	procTranslateMessage = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW = modUser32.NewProc("DispatchMessageW")
	// PostThreadMessageW targets the pump thread for clean shutdown.
	procPostThreadMessageW = modUser32.NewProc("PostThreadMessageW")

	modKernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId = modKernel32.NewProc("GetCurrentThreadId")
)

const (
	hotkeyModControl  = 0x0002
	hotkeyModShift    = 0x0004
	hotkeyModNoRepeat = 0x4000
	hotkeyVKSpace     = 0x20
	hotkeyWMHotkey    = 0x0312
	// WM_APP+1: posted to the pump thread to stop it cleanly.
	hotkeyWMStop     = 0x8001
	quickAccessHotID = 0xC1FE
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

func currentThreadID() uint32 {
	r, _, _ := procGetCurrentThreadId.Call()
	return uint32(r)
}

// startQuickAccessHotkey spawns a dedicated OS thread that registers
// Ctrl+Shift+Space (hWnd=NULL hotkeys bind to the calling thread) and runs
// the message pump until stopQuickAccessHotkey targets the returned thread
// id. It blocks until registration succeeds or fails.
func startQuickAccessHotkey(onHotkey func()) (uint32, error) {
	type result struct {
		tid uint32
		err error
	}
	ready := make(chan result, 1)
	go func() {
		// Pin the goroutine: registration AND the message pump must live on
		// the same OS thread for RegisterHotKey/GetMessageW to see WM_HOTKEY.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		tid := currentThreadID()
		r, _, callErr := procRegisterHotKey.Call(
			0,
			quickAccessHotID,
			hotkeyModControl|hotkeyModShift|hotkeyModNoRepeat,
			hotkeyVKSpace,
		)
		if r == 0 {
			if errno, ok := callErr.(windows.Errno); ok && errno != 0 {
				ready <- result{0, errors.New("atalho global já está em uso por outro aplicativo")}
			} else {
				ready <- result{0, errors.New("não foi possível registrar o atalho global")}
			}
			return
		}
		ready <- result{tid, nil}
		quickAccessLoop(onHotkey)
	}()
	res := <-ready
	return res.tid, res.err
}

// stopQuickAccessHotkey posts the stop message to the pump thread, which
// then unregisters the hotkey on the thread that owns it and exits.
func stopQuickAccessHotkey(threadID uint32) {
	if threadID == 0 {
		return
	}
	_, _, _ = procPostThreadMessageW.Call(uintptr(threadID), hotkeyWMStop, 0, 0)
}

// quickAccessLoop runs a message pump on the calling thread and invokes
// onHotkey for every registered hotkey press. hotkeyWMStop unregisters the
// hotkey (same thread that registered it) and ends the loop.
func quickAccessLoop(onHotkey func()) {
	var msg winMsg
	for {
		r, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if int32(r) <= 0 {
			// loop ended (WM_QUIT/error): release the hotkey if still owned
			unregisterQuickAccessHotkey()
			return
		}
		switch msg.Message {
		case hotkeyWMHotkey:
			if msg.WParam == quickAccessHotID {
				onHotkey()
			}
		case hotkeyWMStop:
			unregisterQuickAccessHotkey()
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func unregisterQuickAccessHotkey() {
	_, _, _ = procUnregisterHotKey.Call(0, quickAccessHotID)
}
