// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package run

import (
	"syscall"
	"time"
)

func init() {
	cpuTime = rusageTime
}

func rusageTime() (user, sys time.Duration) {
	var rusage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage)
	if err != nil {
		return 0, 0
	}
	return time.Duration(rusage.Utime.Nano()), time.Duration(rusage.Stime.Nano())
}
