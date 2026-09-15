package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrint_quotedAndExpression(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.Print(`"Hello, " 200`); err != nil {
		t.Fatalf("Print: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Hello, ") || !strings.Contains(got, "00000200") {
		t.Errorf("output = %q, want the literal text and hex value", got)
	}
}

func TestQuit_setsRunningFalse(t *testing.T) {
	c, _ := newTestConsole(t)
	if !c.Running() {
		t.Fatal("expected Running() true before Quit")
	}
	if err := c.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	if c.Running() {
		t.Error("expected Running() false after Quit")
	}
}

func TestTime_runsCommandAndReportsElapsed(t *testing.T) {
	c, buf := newTestConsole(t)
	called := false

	buf.Reset()

	err := c.Time("NOP", func(cmd string) error {
		called = true
		if cmd != "NOP" {
			t.Errorf("dispatch got %q, want NOP", cmd)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Time: %v", err)
	}

	if !called {
		t.Error("expected dispatch to be called")
	}

	if !strings.Contains(buf.String(), "Elapsed time") {
		t.Errorf("output = %q, want an elapsed-time message", buf.String())
	}
}

func TestInclude_dispatchesEachLine(t *testing.T) {
	c, _ := newTestConsole(t)
	path := filepath.Join(t.TempDir(), "script.com")
	writeFile(t, path, "; a comment\nSHOW REG\n\nSHOW PSL\n")

	var got []string

	err := c.Include(path, func(cmd string) error {
		got = append(got, cmd)

		return nil
	})

	if err != nil {
		t.Fatalf("Include: %v", err)
	}

	want := []string{"SHOW REG", "SHOW PSL"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("dispatched lines = %v, want %v", got, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestClearSymbol(t *testing.T) {
	c, _ := newTestConsole(t)
	c.Symbols.Set("FOO", 1, SymbolUser)
	c.Symbols.Set("BAR", 2, SymbolUser)

	if err := c.ClearSymbol("FOO", false); err != nil {
		t.Fatalf("ClearSymbol: %v", err)
	}

	if _, ok := c.Symbols.Get("FOO"); ok {
		t.Error("expected FOO cleared")
	}

	if _, ok := c.Symbols.Get("BAR"); !ok {
		t.Error("expected BAR to remain")
	}

	if err := c.ClearSymbol("", true); err != nil {
		t.Fatalf("ClearSymbol: %v", err)
	}

	if _, ok := c.Symbols.Get("BAR"); ok {
		t.Error("expected BAR cleared by /ALL")
	}
}

func TestClearBreakpoint(t *testing.T) {
	c, _ := newTestConsole(t)
	c.AddBreakpoint(0x100)
	c.AddBreakpoint(0x200)

	if err := c.ClearBreakpoint(0x100, false); err != nil {
		t.Fatalf("ClearBreakpoint: %v", err)
	}

	if len(c.Breakpoints) != 1 {
		t.Errorf("len(Breakpoints) = %d, want 1", len(c.Breakpoints))
	}

	if err := c.ClearBreakpoint(0, true); err != nil {
		t.Fatalf("ClearBreakpoint: %v", err)
	}

	if len(c.Breakpoints) != 0 {
		t.Errorf("len(Breakpoints) = %d, want 0", len(c.Breakpoints))
	}
}

func TestHelp_lookupAndMissingTopic(t *testing.T) {
	h := ParseHelp("$SHOW,LOGI\n$SH  ,LOGI\nShows logical names.\nMore text.\n$OTHER\nOther text.\n")
	c, buf := newTestConsole(t)

	buf.Reset()

	if err := c.Help(h, []string{"SHOW", "LOGI"}); err != nil {
		t.Fatalf("Help: %v", err)
	}

	if !strings.Contains(buf.String(), "Shows logical names.") {
		t.Errorf("output = %q, want the SHOW LOGICAL help text", buf.String())
	}

	buf.Reset()

	if err := c.Help(h, []string{"SH", "LOGI"}); err != nil {
		t.Fatalf("Help: %v", err)
	}

	if !strings.Contains(buf.String(), "Shows logical names.") {
		t.Errorf("abbreviated key output = %q, want the same help text", buf.String())
	}

	buf.Reset()

	if err := c.Help(h, []string{"NOSUCHTOPIC"}); err != nil {
		t.Fatalf("Help: %v", err)
	}
	
	if !strings.Contains(buf.String(), "No help available") {
		t.Errorf("output = %q, want a no-help message", buf.String())
	}
}

func TestHelp_nilHelp(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.Help(nil, nil); err != nil {
		t.Fatalf("Help: %v", err)
	}
	if !strings.Contains(buf.String(), "No help file") {
		t.Errorf("output = %q, want a no-help-file message", buf.String())
	}
}
