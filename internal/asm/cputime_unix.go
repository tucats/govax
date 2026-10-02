//go:build unix

package asm

import (
	"syscall"
	"time"
)

// processCPU returns the CPU time the govax process has used so far, user
// and system time together, for a listing's performance indicators. It's
// 0 if the system won't say.
func processCPU() time.Duration {
	var u syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &u); err != nil {
		return 0
	}

	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}
