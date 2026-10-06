package debugger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/vmserrors"
)

// session is a console with both grammars loaded and a debugger
// installed, as cmd/govax builds them.
type session struct {
	c  *console.Console
	d  *console.Dispatcher
	db *debugger.Debugger
	// out is the console's output.
	out *bytes.Buffer
}

func newSession(t *testing.T) *session {
	t.Helper()

	c, out := consoletest.New(t)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), consoletest.ParseHelp(t, "console.help"))
	c.Dispatcher = d

	db := debugger.Install(c, consoletest.DebugGrammar(t), consoletest.ParseHelp(t, "debug.help"))

	return &session{c: c, d: d, db: db, out: out}
}

// do dispatches one line the way the front end does, through the console
// dispatcher's routing, and returns what it printed.
func (s *session) do(t *testing.T, line string) (string, error) {
	t.Helper()

	s.out.Reset()

	err := s.d.Dispatch(line)

	return s.out.String(), err
}

// TestDebugEntersAndExitLeaves: the console's DEBUG starts a session, and
// the debugger's EXIT ends it.
func TestDebugEntersAndExitLeaves(t *testing.T) {
	s := newSession(t)

	if s.c.InDebugger() {
		t.Fatal("a session is active before DEBUG")
	}

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatalf("DEBUG: %v", err)
	}

	if !s.c.InDebugger() || !s.db.Active() {
		t.Fatal("DEBUG didn't start a session")
	}

	if _, err := s.do(t, "EXIT"); err != nil {
		t.Fatalf("EXIT: %v", err)
	}

	if s.c.InDebugger() {
		t.Fatal("EXIT didn't end the session")
	}

	// EXIT at DBG> returned to the console; it didn't quit govax.
	if !s.c.Running() {
		t.Fatal("the debugger's EXIT quit the console")
	}
}

// TestQuitEndsSession: QUIT ends the session too, without leaving govax.
func TestQuitEndsSession(t *testing.T) {
	s := newSession(t)

	for _, line := range []string{"DEBUG", "QUIT"} {
		if _, err := s.do(t, line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if s.c.InDebugger() || !s.c.Running() {
		t.Fatal("QUIT should end the session and leave the console running")
	}
}

// TestConsoleExitStillQuits: at the console prompt, EXIT is still the
// console's, and leaves govax.
func TestConsoleExitStillQuits(t *testing.T) {
	s := newSession(t)

	if _, err := s.do(t, "EXIT"); err != nil {
		t.Fatalf("EXIT: %v", err)
	}

	if s.c.Running() {
		t.Fatal("the console's EXIT should stop the console")
	}
}

// TestDebugWithoutDebugger: a console with no debugger installed says so.
func TestDebugWithoutDebugger(t *testing.T) {
	c, _ := consoletest.New(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	err := d.Dispatch("DEBUG")
	if err == nil || !strings.Contains(err.Error(), "NOTAVAILABLE") {
		t.Fatalf("DEBUG with no debugger: got %v, want NOTAVAILABLE", err)
	}
}

// TestPromptFollowsMode: the front end picks the prompt from the mode,
// which is what InDebugger reports at each step.
func TestPromptFollowsMode(t *testing.T) {
	s := newSession(t)

	modes := []struct {
		line   string
		prompt string
	}{
		{"DEBUG", debugger.Prompt},
		{"EXIT", "VAX> "},
	}

	for _, m := range modes {
		if _, err := s.do(t, m.line); err != nil {
			t.Fatalf("%s: %v", m.line, err)
		}

		got := "VAX> "
		if s.c.InDebugger() {
			got = debugger.Prompt
		}

		if got != m.prompt {
			t.Errorf("after %s: prompt %q, want %q", m.line, got, m.prompt)
		}
	}
}

// TestConsoleVerbRefused: a console command at DBG> gets the VMS
// debugger's syntax error, then govax's hint, and doesn't run.
func TestConsoleVerbRefused(t *testing.T) {
	s := newSession(t)

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatal(err)
	}

	out, err := s.do(t, "DIRECTORY")

	want := "%DEBUG-E-SYNTAX, command syntax error at or near 'DIRECTORY'\n" +
		"%DEBUG-I-CONSOLECOMMAND, 'DIRECTORY' is a console command; EXIT returns to the console\n"
	if out != want {
		t.Errorf("DIRECTORY at DBG>:\n got %q\nwant %q", out, want)
	}

	// The error is returned (a one-shot caller sees the failure) but
	// marked as shown, so the front end doesn't print it twice.
	if err == nil || !vmserrors.MessageInhibited(err) {
		t.Errorf("error = %v, want an inhibited failure", err)
	}

	if !s.c.InDebugger() {
		t.Error("a refused command ended the session")
	}
}

// TestUnknownWordIsSyntaxError: a word that is no command at all gets the
// syntax error alone, with no hint, as VMS's errors.dlg shows; so does an
// unknown qualifier on a verb the debugger has.
func TestUnknownWordIsSyntaxError(t *testing.T) {
	s := newSession(t)

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatal(err)
	}

	tests := []struct{ line, word string }{
		{"FOO", "FOO"},
		{"exit/nosuchqual", "NOSUCHQUAL"},
	}

	for _, tt := range tests {
		out, err := s.do(t, tt.line)

		if out != "" {
			t.Errorf("%s: printed %q, want nothing (the front end prints the error)", tt.line, out)
		}

		want := "DEBUG-E-SYNTAX, command syntax error at or near '" + tt.word + "'"
		if err == nil || err.Error() != want {
			t.Errorf("%s: error %v, want %q", tt.line, err, want)
		}
	}

	if !s.c.InDebugger() {
		t.Error("a syntax error ended the session")
	}
}

// TestDebuggerHelp: HELP at DBG> reads debug.help, not vax.help.
func TestDebuggerHelp(t *testing.T) {
	s := newSession(t)

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatal(err)
	}

	out, err := s.do(t, "HELP")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "govax debugger") {
		t.Errorf("HELP at DBG>: %q isn't debug.help's", out)
	}

	out, err = s.do(t, "HELP EXIT")
	if err != nil || !strings.Contains(out, "Ends the debugger session") {
		t.Errorf("HELP EXIT at DBG>: %q, %v", out, err)
	}

	// The same word at VAX> is vax.help's.
	if _, err := s.do(t, "EXIT"); err != nil {
		t.Fatal(err)
	}

	out, _ = s.do(t, "HELP")
	if strings.Contains(out, "govax debugger") {
		t.Errorf("HELP at VAX> showed debug.help: %q", out)
	}
}

// TestCommandFileSwitchesGrammars: the lines of a command file go to the
// grammar current as each is read. The first is a console command; DEBUG
// starts the session, so the debugger's HELP EXIT reads the next; EXIT
// ends it, so the console's PRINT reads the last.
func TestCommandFileSwitchesGrammars(t *testing.T) {
	s := newSession(t)

	path := filepath.Join(t.TempDir(), "session.com")
	script := strings.Join([]string{
		`PRINT "one: console"`,
		`DEBUG`,
		`HELP EXIT`,
		`EXIT`,
		`PRINT "two: console again"`,
	}, "\n")

	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := s.do(t, `@"`+path+`"`)
	if err != nil {
		t.Fatalf("@file: %v", err)
	}

	for _, want := range []string{"one: console", "Ends the debugger session", "two: console again"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	if s.c.InDebugger() {
		t.Error("the file's EXIT left the session active")
	}
}

// TestDebuggerAtFile: @file at DBG> runs debugger commands, and a file
// that ends the session hands the rest of itself to the console.
func TestDebuggerAtFile(t *testing.T) {
	s := newSession(t)

	path := filepath.Join(t.TempDir(), "dbg.com")
	if err := os.WriteFile(path, []byte("HELP QUIT\nEXIT\nPRINT \"back\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatal(err)
	}

	out, err := s.do(t, `@"`+path+`"`)
	if err != nil {
		t.Fatalf("@file at DBG>: %v", err)
	}

	if !strings.Contains(out, "Ends the debugger session and returns to the console, as EXIT does") ||
		!strings.Contains(out, "back") {
		t.Errorf("unexpected output:\n%s", out)
	}

	if s.c.InDebugger() {
		t.Error("session still active after the file's EXIT")
	}
}

// TestConsoleCommandFromProgram: XFC$CONSOLE_CMD reaches the console's
// grammar even while a debugger session is active.
func TestConsoleCommandFromProgram(t *testing.T) {
	s := newSession(t)

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatal(err)
	}

	s.out.Reset()

	if status := s.c.ConsoleCommand(`PRINT "from a program"`); status != 0 {
		t.Fatalf("ConsoleCommand status = %d, want 0", status)
	}

	if got := s.out.String(); !strings.Contains(got, "from a program") {
		t.Errorf("the console grammar didn't run PRINT: %q", got)
	}

	// And it didn't end or disturb the session.
	if !s.c.InDebugger() {
		t.Error("ConsoleCommand ended the session")
	}
}
