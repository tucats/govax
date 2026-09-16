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

func TestClearSymbolTemporary(t *testing.T) {
	c, _ := newTestConsole(t)
	c.Symbols.SetQualified("PERM", 1, true, false, false)
	c.Symbols.SetQualified("TEMP", 2, false, false, false)
	c.Symbols.Set("SYS$FOO", 3, SymbolSystem)

	if err := c.ClearSymbolTemporary(); err != nil {
		t.Fatalf("ClearSymbolTemporary: %v", err)
	}

	if _, ok := c.Symbols.Get("PERM"); !ok {
		t.Error("expected PERM to survive CLEAR SYMBOL/TEMPORARY")
	}
	if _, ok := c.Symbols.Get("TEMP"); ok {
		t.Error("expected TEMP to be cleared")
	}
	if _, ok := c.Symbols.Get("SYS$FOO"); !ok {
		t.Error("expected the system symbol to survive")
	}
}

func TestClearString(t *testing.T) {
	c, _ := newTestConsole(t)

	if err := c.ClearString(); err == nil {
		t.Error("expected an error before the string-pool symbols exist")
	}

	base := uint32(0x2000)
	c.Symbols.Set("CONSOLE$STRINGPOOL_BASE", base, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL_SIZE", 16, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL", base+8, SymbolSystem)

	if err := c.Mem.StoreLongword(c.CPU, base, 0xDEADBEEF); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := c.ClearString(); err != nil {
		t.Fatalf("ClearString: %v", err)
	}

	if got, _ := c.Symbols.Get("CONSOLE$STRINGPOOL"); got != base {
		t.Errorf("CONSOLE$STRINGPOOL = %#x, want reset to base %#x", got, base)
	}

	got, err := c.Mem.LoadLongword(c.CPU, base)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}
	if got != 0 {
		t.Errorf("pool storage = %#x, want zeroed", got)
	}
}

func TestClearMemory_reinitializes(t *testing.T) {
	c, _ := newTestConsole(t)
	c.Symbols.Set("FOO", 1, SymbolUser)
	if err := c.Mem.StoreLongword(c.CPU, 0x100, 0xCAFEBABE); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := c.ClearMemory(); err != nil {
		t.Fatalf("ClearMemory: %v", err)
	}

	if _, ok := c.Symbols.Get("FOO"); ok {
		t.Error("expected CLEAR MEMORY (== ZERO) to clear symbols too")
	}
	got, err := c.Mem.LoadLongword(c.CPU, 0x100)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}
	if got != 0 {
		t.Errorf("memory at 0x100 = %#x, want zeroed", got)
	}
}

func TestClearInterruptAndClearAllInterrupts(t *testing.T) {
	c, _ := newTestConsole(t)

	psl := c.CPU.PSL()
	psl.SetIPL(20)
	c.CPU.SetPSL(psl)
	c.Engine.SetQuantum(4)
	c.Engine.Interrupt(0x24, 20, 0) // masked: queued

	if err := c.ClearInterrupt(0x24); err != nil {
		t.Fatalf("ClearInterrupt: %v", err)
	}
	if _, queued := c.Engine.PendingInterrupts(); len(queued) != 0 {
		t.Errorf("queued = %+v, want empty after ClearInterrupt", queued)
	}

	c.Engine.Interrupt(0x24, 20, 0)
	if err := c.ClearAllInterrupts(); err != nil {
		t.Fatalf("ClearAllInterrupts: %v", err)
	}
	if pending, queued := c.Engine.PendingInterrupts(); pending != nil || len(queued) != 0 {
		t.Errorf("pending=%v queued=%v, want both empty after ClearAllInterrupts", pending, queued)
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
