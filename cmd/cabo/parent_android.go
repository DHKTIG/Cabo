//go:build android

package main

import (
	"log"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// bindToParent asks Android to terminate this server if its launcher dies.
func bindToParent() {
	parentPID := os.Getppid()
	if parentPID == 1 {
		os.Exit(1)
	}
	if err := unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(syscall.SIGKILL), 0, 0, 0); err != nil {
		log.Printf("设置父进程死亡信号失败: %v", err)
		return
	}

	// Close the race where the parent died just before prctl succeeded.
	if os.Getppid() != parentPID {
		log.Printf("父进程已退出，停止本地服务")
		os.Exit(1)
	}
}
