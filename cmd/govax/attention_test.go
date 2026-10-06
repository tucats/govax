package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chzyer/readline"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

func testAttentionEngine() *cpu.Engine {
	return cpu.NewEngine(vax.New(), vm.NewMemory(4096))
}

// lockedBuffer is a bytes.Buffer that the pump's goroutine and the test's
// can share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// newTestStdin is newAttentionStdin on r, with the echoes going to the
// returned buffer and forceExit counting its calls in *exits.
func newTestStdin(r io.Reader, vmsMode bool, e *cpu.Engine) (s *attentionStdin, echoes *lockedBuffer, exits *int) {
	echoes = &lockedBuffer{}
	exits = new(int)

	s = newAttentionStdin(r, echoes, vmsMode, func() *cpu.Engine { return e }, func() { *exits++ })

	return s, echoes, exits
}

// readAll reads s until its input ends, and returns what it read. A read
// a CTRL/C in the input interrupted is read again.
func readAll(t *testing.T, s *attentionStdin) string {
	t.Helper()

	var got []byte

	buf := make([]byte, 1)

	for {
		_, err := s.Read(buf)
		if errors.Is(err, io.EOF) {
			return string(got)
		}

		if errors.Is(err, errReadInterrupted) {
			continue
		}

		if err != nil {
			t.Fatalf("Read: %v", err)
		}

		got = append(got, buf[0])
	}
}

// chunkReader returns its chunks one per Read, as a terminal in its
// ordinary mode returns lines: an empty chunk is a read that returned
// nothing (CTRL/Z on an empty line).
type chunkReader struct{ chunks []string }

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}

	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]

	if chunk == "" {
		return 0, io.EOF
	}

	return copy(p, chunk), nil
}

func TestAttentionStdin_forwardsOrdinaryBytes(t *testing.T) {
	e := testAttentionEngine()
	s, _, _ := newTestStdin(strings.NewReader("AB"), false, e)

	if got := readAll(t, s); got != "AB" {
		t.Errorf("read %q, want %q", got, "AB")
	}

	if e.AttentionRequested() {
		t.Error("AttentionRequested() = true, want false (no Ctrl-C in the stream)")
	}
}

// TestAttentionStdin_filtersCtrlCAndCallsAttention: a CTRL/C byte outside
// a prompt (input that isn't a terminal in VMS mode) interrupts the
// program and isn't delivered as data.
func TestAttentionStdin_filtersCtrlCAndCallsAttention(t *testing.T) {
	e := testAttentionEngine()
	s, echoes, _ := newTestStdin(strings.NewReader("A\x03B"), false, e)

	if got := readAll(t, s); got != "AB" {
		t.Errorf("read %q, want %q (the CTRL/C isn't data)", got, "AB")
	}

	if e.PendingAttention() != cpu.AttentionCtrlC {
		t.Errorf("PendingAttention() = %#x, want CTRL/C", e.PendingAttention())
	}

	if echoes.String() != echoInterrupt+"\r\n" {
		t.Errorf("echoed %q, want %q", echoes.String(), echoInterrupt+"\r\n")
	}
}

// TestAttentionStdin_ctrlZ: in VMS mode, a read that returns nothing is
// CTRL/Z on an empty line (echoing *Exit*), and a line that ends without a
// newline was ended by CTRL/Z: its text, a CTRL/Z ending it, and a CTRL/Z
// for the next read.
func TestAttentionStdin_ctrlZ(t *testing.T) {
	e := testAttentionEngine()
	r := &chunkReader{chunks: []string{"one\n", "", "two", "three\n"}}
	s, echoes, _ := newTestStdin(r, true, e)

	if got, want := readAll(t, s), "one\n\x1Atwo\x1A\x1Athree\n"; got != want {
		t.Errorf("read %q, want %q", got, want)
	}

	if got, want := echoes.String(), echoExit+"\r\n\r\n"; got != want {
		t.Errorf("echoed %q, want %q", got, want)
	}
}

// TestAttentionStdin_ctrlZNotInVMSMode: input that isn't a terminal in VMS
// mode ends at its end, and a last line without a newline is just text.
func TestAttentionStdin_ctrlZNotInVMSMode(t *testing.T) {
	s, _, _ := newTestStdin(&chunkReader{chunks: []string{"one\n", "two"}}, false, testAttentionEngine())

	if got, want := readAll(t, s), "one\ntwo"; got != want {
		t.Errorf("read %q, want %q", got, want)
	}
}

// TestAttentionStdin_ctrlYAtPrompt: CTRL/Y typed at a prompt asks govax to
// end, and reaches readline as the CTRL/C that ends its read.
func TestAttentionStdin_ctrlYAtPrompt(t *testing.T) {
	e := testAttentionEngine()
	s, _, _ := newTestStdin(strings.NewReader("\x19"), true, e)
	s.setPrompt(true)

	if got, err := io.ReadAll(s.promptReader()); err != nil || string(got) != "\x03" {
		t.Errorf("readline read %q, %v; want CTRL/C", got, err)
	}

	if !s.abortRequested() {
		t.Error("abortRequested() = false after CTRL/Y")
	}
}

// TestAttentionStdin_interruptsWaitingRead: CTRL/C ends a program's read
// that is waiting for a line, but not readline's at a prompt.
func TestAttentionStdin_interruptsWaitingRead(t *testing.T) {
	e := testAttentionEngine()
	pr, pw := io.Pipe()

	defer pw.Close()

	s, _, _ := newTestStdin(pr, true, e)

	result := make(chan error, 1)

	go func() {
		_, err := s.Read(make([]byte, 1))
		result <- err
	}()

	time.Sleep(20 * time.Millisecond) // let the read start waiting
	s.ctrlC()

	select {
	case err := <-result:
		if !errors.Is(err, errReadInterrupted) {
			t.Errorf("Read = %v, want errReadInterrupted", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CTRL/C didn't end the waiting read")
	}

	// At a prompt, readline's read waits on.
	s.setPrompt(true)

	go func() {
		_, err := s.promptReader().Read(make([]byte, 1))
		result <- err
	}()

	time.Sleep(20 * time.Millisecond)
	s.ctrlC()

	select {
	case err := <-result:
		t.Errorf("CTRL/C ended readline's read (%v)", err)
	case <-time.After(50 * time.Millisecond):
	}

	_, _ = pw.Write([]byte("x"))

	if err := <-result; err != nil {
		t.Errorf("Read at the prompt = %v", err)
	}
}

// TestAttentionStdin_ctrlYTwice: a second CTRL/Y while the first is still
// waiting to be dealt with exits at once.
func TestAttentionStdin_ctrlYTwice(t *testing.T) {
	e := testAttentionEngine()
	s, _, exits := newTestStdin(strings.NewReader(""), true, e)

	s.ctrlY(true)

	if *exits != 0 {
		t.Fatal("one CTRL/Y exited at once")
	}

	s.ctrlY(true)

	if *exits != 1 {
		t.Errorf("forceExit called %d times after a second CTRL/Y, want 1", *exits)
	}
}

// TestAttentionStdin_ctrlYTakenByAST: a CTRL/Y a program's AST took (the
// key no longer pending, and the run not stopped by it) doesn't end govax.
func TestAttentionStdin_ctrlYTakenByAST(t *testing.T) {
	e := testAttentionEngine()
	s, _, _ := newTestStdin(strings.NewReader(""), true, e)

	s.ctrlY(true)
	e.BeginRun() // keeps the CTRL/Y

	if !s.abortRequested() {
		t.Fatal("abortRequested() = false with the CTRL/Y still pending")
	}

	e.AttentionKey(0) // as the AST handler's taking it does

	if s.abortRequested() {
		t.Error("abortRequested() = true for a CTRL/Y the program took")
	}

	if s.aborting.Load() {
		t.Error("the CTRL/Y the program took wasn't forgotten")
	}
}

// TestPromptReader_onlyAtPrompt: readline's reads take bytes only while it
// reads a command line; a read waiting meanwhile leaves the input to a
// program's read, and Close ends it.
func TestPromptReader_onlyAtPrompt(t *testing.T) {
	pr, pw := io.Pipe()

	defer pw.Close()

	s, _, _ := newTestStdin(pr, false, testAttentionEngine())

	got := make(chan byte, 2)
	ended := make(chan error, 1)

	go func() {
		buf := make([]byte, 1)

		for {
			if _, err := s.promptReader().Read(buf); err != nil {
				ended <- err

				return
			}

			got <- buf[0]
		}
	}()

	// Not at a prompt: the program's read gets the byte.
	go func() { _, _ = pw.Write([]byte("p")) }()

	buf := make([]byte, 1)
	if _, err := s.Read(buf); err != nil || buf[0] != 'p' {
		t.Fatalf("program's Read = %q, %v; want 'p'", buf[0], err)
	}

	// At a prompt, readline's read does.
	s.setPrompt(true)

	go func() { _, _ = pw.Write([]byte("r")) }()

	select {
	case b := <-got:
		if b != 'r' {
			t.Errorf("readline read %q, want 'r'", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readline's read didn't get the byte at the prompt")
	}

	s.setPrompt(false)

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-ended:
		if !errors.Is(err, io.EOF) {
			t.Errorf("readline's read after Close = %v, want io.EOF", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close didn't end readline's waiting read")
	}
}

func TestAttentionStdin_eofOnUnderlyingEOF(t *testing.T) {
	s, _, _ := newTestStdin(strings.NewReader(""), false, testAttentionEngine())

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
	s, _, _ := newTestStdin(strings.NewReader(""), false, testAttentionEngine())

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := s.Read(buf); err != io.EOF {
		t.Errorf("Read after Close = %v, want io.EOF", err)
	}
}

// TestAttentionStdin_nilEngineIsSafe covers the defensive nil checks --
// getEngine returning nil (Console.Engine not yet set, e.g. a Ctrl-C
// arriving before Init has ever run) must not panic.
func TestAttentionStdin_nilEngineIsSafe(t *testing.T) {
	s, _, _ := newTestStdin(strings.NewReader("\x03B"), false, nil)

	if got := readAll(t, s); got != "B" {
		t.Errorf("read %q, want %q", got, "B")
	}

	s.ctrlY(true)

	if !s.abortRequested() {
		t.Error("abortRequested() = false after CTRL/Y with no engine")
	}
}

// TestPromptKeys: at a prompt, CTRL/Z on an empty line ends the read as end
// of file, echoing *Exit*; after some text it ends the line, and the next
// read is end of file; CTRL/C cancels the line, echoing *Interrupt*.
func TestPromptKeys(t *testing.T) {
	cfg := &readline.Config{}
	k := newPromptKeys(cfg)

	if r, ok := k.filter(charCtrlZ); r != readline.CharInterrupt || !ok {
		t.Errorf("CTRL/Z on an empty line = %q, %v; want CharInterrupt", r, ok)
	}

	if cfg.InterruptPrompt != echoExit || !k.endOfFile(readline.ErrInterrupt) {
		t.Errorf("CTRL/Z on an empty line: echo %q, endOfFile false", cfg.InterruptPrompt)
	}

	if k.endOfFile(readline.ErrInterrupt) {
		t.Error("endOfFile reported the same CTRL/Z twice")
	}

	k.lineLength.Store(4)

	if r, _ := k.filter(charCtrlZ); r != readline.CharEnter {
		t.Errorf("CTRL/Z after text = %q, want CharEnter", r)
	}

	if !k.eofPending() || k.eofPending() {
		t.Error("CTRL/Z after text: the next read (only) should be end of file")
	}

	if r, _ := k.filter(charCtrlC); r != charCtrlC || cfg.InterruptPrompt != echoInterrupt {
		t.Errorf("CTRL/C = %q, echo %q; want CTRL/C, %q", r, cfg.InterruptPrompt, echoInterrupt)
	}

	if k.endOfFile(readline.ErrInterrupt) {
		t.Error("CTRL/C taken as end of file")
	}
}
