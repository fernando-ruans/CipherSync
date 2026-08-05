//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	crypt32                   = syscall.NewLazyDLL("crypt32.dll")
	cryptProtectData          = crypt32.NewProc("CryptProtectData")
	cryptUnprotectData        = crypt32.NewProc("CryptUnprotectData")
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	localFree                 = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

type cryptProtectPromptStruct struct {
	cbSize     uint32
	dwPromptFlags uint32
	hwndApp    uintptr
	szPrompt   *uint16
}

const (
	cryptProtectUIForbidden = 0x1
	cryptProtectLocalMachine = 0x4
)

// helloAvailable reports whether DPAPI protection is usable on this machine.
func helloAvailable() bool {
	return true
}

// protectForHello encrypts data with the current Windows user's credentials
// (DPAPI). The data can only be decrypted by the same Windows user session.
func protectForHello(data []byte, identifier string) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("nothing to protect")
	}
	in := &dataBlob{cbData: uint32(len(data))}
	if len(data) > 0 {
		in.pbData = &data[0]
	}
	var out dataBlob

	r, _, err := cryptProtectData.Call(
		uintptr(unsafe.Pointer(in)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(identifier))),
		0,
		0,
		0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	result := make([]byte, out.cbData)
	for i := uint32(0); i < out.cbData; i++ {
		result[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(out.pbData)) + uintptr(i)))
	}
	return result, nil
}

// unprotectWithHello decrypts data protected by protectForHello.
func unprotectWithHello(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("nothing to unprotect")
	}
	in := &dataBlob{cbData: uint32(len(data))}
	if len(data) > 0 {
		in.pbData = &data[0]
	}
	var out dataBlob

	r, _, err := cryptUnprotectData.Call(
		uintptr(unsafe.Pointer(in)),
		0,
		0,
		0,
		0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, errors.New("falha ao desbloquear com Windows Hello: " + err.Error())
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	result := make([]byte, out.cbData)
	for i := uint32(0); i < out.cbData; i++ {
		result[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(out.pbData)) + uintptr(i)))
	}
	return result, nil
}
