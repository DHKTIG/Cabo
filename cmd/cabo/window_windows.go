//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var procMessageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

// reportError 在没有控制台的 GUI 程序里用系统弹窗告知错误。
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
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbIconError)
}

// webviewDataPath 把 WebView2 的缓存放到 exe 同目录，做成绿色便携版。
// 这里曾经试过放到 AppData，但在部分机器上创建环境会失败导致启动即退出，
// 所以固定用 exe 同目录；目录不可写时再退回用户缓存目录。
func webviewDataPath() string {
	if exePath, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exePath), "cabo_webview2_data")
		if os.MkdirAll(p, 0o755) == nil {
			return p
		}
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		p := filepath.Join(dir, "Cabo", "WebView2")
		if os.MkdirAll(p, 0o755) == nil {
			return p
		}
	}
	return ""
}

// runWindow 创建一个原生窗口，里面直接渲染游戏页面。
func runWindow(url string, debug bool) error {
	// Win32 的窗口创建和消息循环必须在同一个系统线程上完成
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     debug,
		AutoFocus: true,
		DataPath:  webviewDataPath(),
		WindowOptions: webview2.WindowOptions{
			Title:  "CABO 卡波",
			Width:  1280,
			Height: 860,
			IconId: 1, // 对应 cabo.rc 里的图标资源 ID
			Center: true,
		},
	})
	if w == nil {
		return errors.New("WebView2 初始化失败，系统可能缺少 WebView2 运行时")
	}
	defer w.Destroy()

	// 限制最小尺寸：再小的话底部的操作按钮会被挤出可视区
	w.SetSize(780, 560, webview2.HintMin)

	w.Navigate(url)
	w.Run()
	return nil
}
