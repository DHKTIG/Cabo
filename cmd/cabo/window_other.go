//go:build !windows

package main

import (
	"errors"
	"log"
)

// runWindow 在非 Windows 平台上不可用，调用方会退回到浏览器模式。
func runWindow(url string, debug bool) error {
	return errors.New("当前平台不支持原生窗口，请用 -browser 参数")
}

func reportError(title, msg string) {
	log.Printf("%s: %s", title, msg)
}
