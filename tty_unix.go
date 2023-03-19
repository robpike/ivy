//go:build aix || darwin || dragonfly || freebsd || hurd || linux || netbsd || openbsd

package main

import (
	"syscall"
	"unsafe"
)

func init() {
	isTTY = isatty
}

func isatty(fd uintptr) bool {
	// size of winsize struct, we only need the syscall error code.
	// https://pkg.go.dev/golang.org/x/sys@v0.6.0/unix#Winsize
	// ioctl_tty(2)
	p := [4]uint16{}
	_, _, e1 := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&p[0])))
	return e1 == 0
}
