package main

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// Phase 51, subtask 4: the other processes' output while a command line
// is read (promptout.go), and the keys being the command line's.

// TestPromptOutput: outside a read, writes go straight to the terminal;
// during one, whole lines go through the line editor and a line's start
// is held until it ends, or until the read does.
func TestPromptOutput(t *testing.T) {
	var term, editor bytes.Buffer

	p := newPromptOutput(&term)

	write := func(text string) {
		t.Helper()

		if n, err := p.Write([]byte(text)); n != len(text) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", text, n, err)
		}
	}

	write("before\n")

	p.begin(&editor)
	write("one\ntw")
	write("o\nthr")
	write("ee")

	if got, want := editor.String(), "one\ntwo\n"; got != want {
		t.Errorf("through the editor: %q, want %q", got, want)
	}

	p.end()
	write("after\n")

	if got, want := term.String(), "before\nthreeafter\n"; got != want {
		t.Errorf("the terminal: %q, want %q", got, want)
	}

	if fd := p.Fd(); fd != ^uintptr(0) {
		t.Errorf("Fd() = %d for a buffer, want ^0", fd)
	}
}

// TestAttentionStdin_readyDuringLineRead: a key waiting is ready for a
// program's read, except while the command loop reads a command line,
// whose key it is.
func TestAttentionStdin_readyDuringLineRead(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()

	s, _, _ := newTestStdin(r, true, testAttentionEngine())
	defer s.Close()

	s.lineRead.Store(true)

	go func() { _, _ = w.Write([]byte("A")) }()

	for deadline := time.Now().Add(5 * time.Second); len(s.bytes) == 0; {
		if time.Now().After(deadline) {
			t.Fatal("the key never arrived")
		}

		time.Sleep(time.Millisecond)
	}

	if s.Ready() {
		t.Error("Ready() = true while a command line is read")
	}

	s.lineRead.Store(false)

	if !s.Ready() {
		t.Error("Ready() = false with a key waiting")
	}
}
