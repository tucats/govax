//go:build !(darwin || linux)

package main

// vmsTerminalMode leaves the terminal alone where govax doesn't know how
// to set its control characters: CTRL/C still interrupts a program, but
// CTRL/Y and CTRL/Z keep the host's meanings while one runs.
func vmsTerminalMode(int) (restore func(), ok bool) { return func() {}, false }
