//go:build !windows

package main

import "log"

func reportError(title, msg string) {
	log.Printf("%s: %s", title, msg)
}
