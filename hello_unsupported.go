//go:build !windows

package main

import "errors"

// helloAvailable reports whether Windows Hello is usable (Windows only).
func helloAvailable() bool {
	return false
}

// protectForHello is only supported on Windows.
func protectForHello(data []byte, identifier string) ([]byte, error) {
	return nil, errors.New("Windows Hello está disponível apenas no Windows")
}

// unprotectWithHello is only supported on Windows.
func unprotectWithHello(data []byte) ([]byte, error) {
	return nil, errors.New("Windows Hello está disponível apenas no Windows")
}
