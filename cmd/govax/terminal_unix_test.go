//go:build darwin || linux

package main

import (
	"os"
	"testing"
	"time"
)

// terminalInputOn is a terminalInput on a pipe: select(2) and read(2) treat one as
// they do a terminal in the ways that matter here. The pipe's writer plays the
// keyboard.
func terminalInputOn(t *testing.T) (*terminalInput, *os.File, *os.File) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	return newTerminalInput(int(r.Fd())), r, w
}

// TestTerminalInputPause is why terminalInput exists: while a host program has the
// terminal, a key goes to it and not to the reader that is waiting all along.
func TestTerminalInputPause(t *testing.T) {
	input, program, keyboard := terminalInputOn(t)

	got := make(chan string)

	go func() {
		buf := make([]byte, 16)

		for {
			n, err := input.Read(buf)
			if err != nil {
				close(got)

				return
			}

			got <- string(buf[:n])
		}
	}()

	if _, err := keyboard.WriteString("a"); err != nil {
		t.Fatal(err)
	}

	if text := <-got; text != "a" {
		t.Fatalf("the reader got %q", text)
	}

	// The reader is now waiting for the next key, as the pump nearly always is.
	time.Sleep(10 * time.Millisecond)

	input.pause()

	if _, err := keyboard.WriteString("b"); err != nil {
		t.Fatal(err)
	}

	// The reader wakes to the key, and has to leave it.
	select {
	case text := <-got:
		t.Fatalf("the reader got %q while paused", text)
	case <-time.After(50 * time.Millisecond):
	}

	buf := make([]byte, 16)

	n, err := program.Read(buf)
	if err != nil || string(buf[:n]) != "b" {
		t.Fatalf("the program read %q, %v", buf[:n], err)
	}

	// With the key gone, the reader goes back to waiting, not to a read that
	// would stop pause returning.
	input.resume()
	time.Sleep(10 * time.Millisecond)

	paused := make(chan struct{})

	go func() {
		input.pause()
		input.resume()
		close(paused)
	}()

	select {
	case <-paused:
	case <-time.After(time.Second):
		t.Fatal("pause waited for a key")
	}

	if _, err := keyboard.WriteString("c"); err != nil {
		t.Fatal(err)
	}

	if text := <-got; text != "c" {
		t.Fatalf("after resuming, the reader got %q", text)
	}
}

// TestTerminalInputEnd is the end of the input, which a read reports as a file's
// read does: attentionStdin's pump looks for it.
func TestTerminalInputEnd(t *testing.T) {
	input, _, keyboard := terminalInputOn(t)

	keyboard.Close()

	buf := make([]byte, 16)
	if n, err := input.Read(buf); n != 0 || err == nil || err.Error() != "EOF" {
		t.Errorf("%d, %v", n, err)
	}
}
