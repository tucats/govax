package main

import (
	"io"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

func testAttentionEngine() *cpu.Engine {
	return cpu.NewEngine(vax.New(), vm.NewMemory(4096))
}

func TestAttentionStdin_forwardsOrdinaryBytes(t *testing.T) {
	e := testAttentionEngine()
	s := newAttentionStdin(strings.NewReader("AB"), func() *cpu.Engine { return e })

	buf := make([]byte, 1)

	if _, err := s.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if buf[0] != 'A' {
		t.Errorf("Read = %q, want 'A'", buf[0])
	}

	if _, err := s.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if buf[0] != 'B' {
		t.Errorf("Read = %q, want 'B'", buf[0])
	}

	if e.AttentionRequested() {
		t.Error("AttentionRequested() = true, want false (no Ctrl-C in the stream)")
	}
}

func TestAttentionStdin_filtersCtrlCAndCallsAttention(t *testing.T) {
	e := testAttentionEngine()
	s := newAttentionStdin(strings.NewReader("A\x03B"), func() *cpu.Engine { return e })

	buf := make([]byte, 1)

	if _, err := s.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if buf[0] != 'A' {
		t.Fatalf("Read = %q, want 'A'", buf[0])
	}

	// The 0x03 byte must never come out the other end as data -- only 'B'
	// should follow 'A'.
	if _, err := s.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if buf[0] != 'B' {
		t.Errorf("Read = %q, want 'B' (0x03 should be filtered, not delivered)", buf[0])
	}

	if !e.AttentionRequested() {
		t.Error("AttentionRequested() = false, want true after a Ctrl-C byte")
	}
}

func TestAttentionStdin_eofOnUnderlyingEOF(t *testing.T) {
	e := testAttentionEngine()
	s := newAttentionStdin(strings.NewReader(""), func() *cpu.Engine { return e })

	buf := make([]byte, 1)
	if _, err := s.Read(buf); err != io.EOF {
		t.Errorf("Read = %v, want io.EOF", err)
	}
}

// TestAttentionStdin_closeThenUnderlyingEOF confirms Close doesn't disturb
// ordinary EOF handling -- the narrower, harder-to-test-deterministically
// guarantee (Close aborting a pump goroutine already blocked delivering a
// buffered byte, see attentionStdin.Close's own doc comment) isn't
// exercised here, since doing so without a flaky sleep-based race would
// require plumbing a configurable buffer size purely for this test.
func TestAttentionStdin_closeThenUnderlyingEOF(t *testing.T) {
	e := testAttentionEngine()
	s := newAttentionStdin(strings.NewReader(""), func() *cpu.Engine { return e })

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := s.Read(buf); err != io.EOF {
		t.Errorf("Read after Close = %v, want io.EOF", err)
	}
}

// TestAttentionStdin_nilEngineIsSafe covers the defensive nil check in
// pump's Ctrl-C branch -- getEngine returning nil (Console.Engine not yet
// set, e.g. a Ctrl-C arriving before Init has ever run) must not panic.
func TestAttentionStdin_nilEngineIsSafe(t *testing.T) {
	s := newAttentionStdin(strings.NewReader("\x03B"), func() *cpu.Engine { return nil })

	buf := make([]byte, 1)
	if _, err := s.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if buf[0] != 'B' {
		t.Errorf("Read = %q, want 'B'", buf[0])
	}
}
