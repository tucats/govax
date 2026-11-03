//go:build !(darwin || linux)

package main

import "os"

// vmsTerminalMode leaves the terminal alone where govax doesn't know how
// to set its control characters: CTRL/C still interrupts a program, but
// CTRL/Y and CTRL/Z keep the host's meanings while one runs.
func vmsTerminalMode(int) (restore func(), hostMode func() (back func()), ok bool) {
	return func() {}, func() (back func()) { return func() {} }, false
}

// terminalInput is never made where the terminal isn't in vmsTerminalMode
// (main.go uses os.Stdin), so a host program started by a command competes
// with the pump for the console: an editor there must have a window of its
// own.
type terminalInput struct{}

func newTerminalInput(int) *terminalInput { return &terminalInput{} }

func (*terminalInput) Read(p []byte) (int, error) { return os.Stdin.Read(p) }

func (*terminalInput) pause() {}

func (*terminalInput) resume() {}
