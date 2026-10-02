//go:build !unix

package asm

import "time"

// processCPU returns the CPU time the govax process has used so far. On
// a system without getrusage it isn't measured, and a listing's
// performance indicators show no CPU time.
func processCPU() time.Duration { return 0 }
