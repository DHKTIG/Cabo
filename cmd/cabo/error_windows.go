//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procMessageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

func reportError(title, msg string) {
	t, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	m, err := windows.UTF16PtrFromString(msg)
	if err != nil {
		return
	}
	const mbIconError = 0x00000010
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), uintptr(mbIconError))
}
