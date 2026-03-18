//go:build windows

package main

import "syscall"

var shell32 = syscall.NewLazyDLL("shell32.dll")
