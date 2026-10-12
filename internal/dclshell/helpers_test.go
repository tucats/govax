package dclshell

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/machine"
)

// Test helpers for DCL's own tests: an interpreter on a small machine,
// with the console's grammar and only DCL's verbs bound. (Tests that
// need the console's other commands, or its routing to the debugger, are
// internal/console's.)

// testDispatcher stands in for the console's Dispatcher: Dispatch runs a
// line as DCL does (DispatchConsole), with no debugger or assembler to
// route to.
type testDispatcher struct {
	Console *Interpreter
}

func (d *testDispatcher) Dispatch(line string) error { return d.Console.DispatchConsole(line) }

// newTestInterpreter is DCL on a 64 KB machine that has been INITed,
// writing to the buffer returned.
func newTestInterpreter(t *testing.T) (*Interpreter, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	m := machine.NewMachine(&buf)
	if err := m.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return NewInterpreter(m, &buf), &buf
}

// newTestDispatcher is newTestInterpreter's DCL with the console's
// grammar, its DCL verbs bound, and a testDispatcher to run lines.
func newTestDispatcher(t *testing.T) (*testDispatcher, *Interpreter) {
	t.Helper()

	c, _ := newTestInterpreter(t)
	g := loadEvaxGrammar(t)
	c.Grammar = g
	c.BindDCL(g)

	d := &testDispatcher{Console: c}
	c.SetRoute(d.Dispatch)

	return d, c
}

// loadEvaxGrammar reads the console's grammar, console.dcl.
func loadEvaxGrammar(t *testing.T) *dcl.Grammar {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	g, err := dcl.LoadGrammarFile(filepath.Join(filepath.Dir(file), "..", "bootdata", "files", "console.dcl"))
	if err != nil {
		t.Fatalf("LoadGrammarFile: %v", err)
	}

	return g
}
