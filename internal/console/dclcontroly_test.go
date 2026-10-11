package console

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// Tests of STOP and Ctrl/Y in command procedures (dclcontroly.go), from
// the User's Manual's rules (13.7, 13.8.1, 13.11 to 13.13) and the DCL
// Dictionary's (STOP, CONTINUE, SET CONTROL).

// keyReader is the terminal for a test: its text, and a Ctrl/Y typed
// (onFirstRead) as the first read of it starts, which is an INQUIRE in
// the procedure, so the key is pending when that command has returned.
// As cmd/govax does, it types the key only while Ctrl/Y is enabled.
type keyReader struct {
	r           *strings.Reader
	onFirstRead func()
}

func (k *keyReader) Read(p []byte) (int, error) {
	if k.onFirstRead != nil {
		k.onFirstRead()
		k.onFirstRead = nil
	}

	return k.r.Read(p)
}

// runControlY runs the procedure lines (as main.com, beside inner.com's
// innerLines when there are any, which INNER.COM in lines names), with typed as the terminal's input and
// a Ctrl/Y typed when the terminal is first read. It returns the
// output, the console, and @'s error.
func runControlY(t *testing.T, typed string, lines []string, innerLines ...string) (string, *Console, error) {
	t.Helper()

	d, c, buf := newCommandDispatcher(t)
	c.In = &keyReader{r: strings.NewReader(typed), onFirstRead: func() {
		if c.ControlYEnabled() {
			c.Engine.AttentionKey(cpu.AttentionCtrlY)
		}
	}}

	dir := t.TempDir()
	if len(innerLines) > 0 {
		inner := writeProcedure(t, dir, "inner.com", innerLines...)
		for i, line := range lines {
			lines[i] = strings.ReplaceAll(line, "INNER.COM", inner)
		}
	}

	proc := writeProcedure(t, dir, "main.com", lines...)
	err := d.Dispatch("@" + proc)

	if c.CommandLevel() != 0 {
		t.Errorf("level %d after the procedure, want 0", c.CommandLevel())
	}

	if k := c.Engine.PendingAttention(); k != 0 {
		t.Errorf("attention key %02X still pending", k)
	}

	return buf.String(), c, err
}

// TestStopProcedures: STOP in a nested procedure ends every level, the
// outer one without going on and without its ON action; the procedures
// end with SS$_ABORT, not shown. ON ERROR THEN STOP does the same
// (the DCL Dictionary's STOP example 4).
func TestStopProcedures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inner []string
	}{
		{"STOP", []string{`$ PRINT "inner"`, "$ STOP", `$ PRINT "inner after"`}},
		{"ON ERROR", []string{`$ PRINT "inner"`, "$ ON ERROR THEN STOP", "$ OPEN X NOSUCH.FILE", `$ PRINT "inner after"`}},
	} {
		d, c, buf := newCommandDispatcher(t)
		dir := t.TempDir()
		inner := writeProcedure(t, dir, "inner.com", tc.inner...)
		outer := writeProcedure(t, dir, "outer.com",
			"$ ON WARNING THEN PRINT \"outer action\"",
			"$ @"+inner,
			`$ PRINT "outer after"`)

		err := d.Dispatch("@" + outer)
		if !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: @ gave %v, want a failure already shown", tc.name, err)
		}

		out := buf.String()
		if !strings.HasPrefix(out, "inner\n") || strings.Contains(out, "after") || strings.Contains(out, "action") {
			t.Errorf("%s: output %q, want only the inner procedure's first line (and a message)", tc.name, out)
		}

		if c.CommandLevel() != 0 || c.Status() != ssAbort|stsInhibitMsg {
			t.Errorf("%s: level %d, $STATUS %08X; want 0, %08X", tc.name, c.CommandLevel(), c.Status(), ssAbort|stsInhibitMsg)
		}

		// The next procedure runs as usual.
		buf.Reset()

		if err := d.Dispatch("@" + writeProcedure(t, dir, "again.com", `$ PRINT "again"`)); err != nil || buf.String() != "again\n" {
			t.Errorf("%s: a procedure after STOP: %v, %q", tc.name, err, buf.String())
		}
	}

	// At the terminal, STOP does nothing.
	d, _, _ := newCommandDispatcher(t)
	if err := d.Dispatch("STOP"); err != nil {
		t.Errorf("STOP at the terminal: %v", err)
	}
}

// TestControlY_onControlY: ON CONTROL_Y's command runs when Ctrl/Y is
// typed, after a command of DCL's own has completed (13.12.1): a GOTO
// to a cleanup (Figure 13-2's FILES.COM), or a command after which the
// procedure goes on, staying in effect for the next Ctrl/Y (PRIV.COM).
func TestControlY_onControlY(t *testing.T) {
	out, _, err := runControlY(t, "abc\n", []string{
		"$ ON CONTROL_Y THEN GOTO CLEAN_UP",
		"$ INQUIRE X",
		`$ PRINT "after"`,
		"$ EXIT",
		`$ CLEAN_UP: PRINT "clean ''X'"`,
	})
	if err != nil || !strings.Contains(out, "clean ABC\n") || strings.Contains(out, "after") {
		t.Errorf("GOTO: %v, output %q; want the cleanup only", err, out)
	}

	out, _, err = runControlY(t, "abc\n", []string{
		`$ ON CONTROL_Y THEN PRINT "Interruption not allowed...continuing"`,
		"$ INQUIRE X",
		`$ PRINT "after ''X' ''F$ENVIRONMENT("ON_CONTROL_Y")'"`,
	})
	if err != nil || !strings.HasSuffix(out, "Interruption not allowed...continuing\nafter ABC TRUE\n") {
		t.Errorf("continuing: %v, output %q", err, out)
	}
}

// TestControlY_nested: Ctrl/Y in a nested procedure with no ON
// CONTROL_Y ends it, and the procedure that called it goes on (13.8.1's
// table, Figure 13-3's SUBSEARCH.COM), even when that one has an ON
// CONTROL_Y of its own; @'s status is SS$_CONTROLY.
func TestControlY_nested(t *testing.T) {
	out, _, err := runControlY(t, "abc\n", []string{
		"$ ON CONTROL_Y THEN GOTO CLEAN_UP",
		"$ @INNER.COM",
		`$ NEXT_STEP: PRINT "outer goes on ''$STATUS'"`,
		"$ EXIT",
		`$ CLEAN_UP: PRINT "clean up"`,
	}, "$ INQUIRE X", `$ PRINT "inner after"`)

	want := fmt.Sprintf("outer goes on %%X%08X\n", ssControlY)
	if err != nil || !strings.HasSuffix(out, want) || strings.Contains(out, "inner after") || strings.Contains(out, "clean") {
		t.Errorf("%v, output %q; want the outer procedure to go on", err, out)
	}
}

// TestControlY_level: at command level 1 with no ON CONTROL_Y, Ctrl/Y
// interrupts the procedure, and DCL reads commands from the terminal in
// the procedure's context (13.11.1's note): CONTINUE resumes it, EXIT
// and STOP end it, a command that runs an image ends it and then runs at
// level 0, and Ctrl/Z (the end of the input) is EXIT.
func TestControlY_level(t *testing.T) {
	lines := []string{"$ X = 1", "$ INQUIRE Y", `$ PRINT "after ''X'"`}

	for _, tc := range []struct {
		name, typed, want string
		status            uint32
	}{
		{"CONTINUE", "y\nX = 2\nWRITE SYS$OUTPUT \"at Ctrl/Y ''X' ''F$ENVIRONMENT(\"DEPTH\")'\"\nCONTINUE\n", "at Ctrl/Y 2 1\nafter 2\n", cliNormal},
		{"EXIT", "y\nEXIT 3\n", "", 3},
		{"STOP", "y\nSTOP\n", "", ssAbort | stsInhibitMsg},
		{"image", "y\nPRINT \"later ''F$ENVIRONMENT(\"DEPTH\")'\"\n", "later 0\n", cliNormal},
		{"end of input", "y\n", "", 1 | stsInhibitMsg},
	} {
		out, c, _ := runControlY(t, tc.typed, lines)

		got := out[strings.LastIndex(out, "Y: ")+len("Y: "):]
		got = strings.ReplaceAll(got, c.Prompt(), "")

		if got != tc.want || c.Status() != tc.status {
			t.Errorf("%s: output %q, $STATUS %08X; want %q, %08X", tc.name, got, c.Status(), tc.want, tc.status)
		}
	}
}

// TestSetControl: SET [NO]CONTROL[=(T,Y)], Y by default, as F$ENVIRONMENT
// reports it; SET NOCONTROL=Y cancels the level's ON CONTROL_Y, and with
// it in effect a Ctrl/Y does nothing in a procedure.
func TestSetControl(t *testing.T) {
	out, c, err := runControlY(t, "abc\n", []string{
		`$ PRINT "''F$ENVIRONMENT("CONTROL")'/''F$ENVIRONMENT("NOCONTROL")'"`,
		"$ ON CONTROL_Y THEN GOTO CLEAN_UP",
		"$ SET NOCONTROL",
		`$ PRINT "''F$ENVIRONMENT("CONTROL")'/''F$ENVIRONMENT("NOCONTROL")' ''F$ENVIRONMENT("ON_CONTROL_Y")'"`,
		"$ INQUIRE X",
		"$ SET CONTROL=(T,Y)",
		`$ PRINT "''F$ENVIRONMENT("CONTROL")'/''F$ENVIRONMENT("NOCONTROL")'"`,
		"$ SET NOCONTROL=T",
		"$ EXIT",
		`$ CLEAN_UP: PRINT "clean up"`,
	})

	if err != nil || strings.Contains(out, "clean") || !strings.Contains(out, "Y/T\n/T,Y FALSE\n") || !strings.HasSuffix(out, "T,Y/\n") {
		t.Errorf("%v, output %q", err, out)
	}

	if !c.ControlYEnabled() || c.control.t {
		t.Errorf("Ctrl/Y enabled %v, Ctrl/T %v; want true, false", c.ControlYEnabled(), c.control.t)
	}

	d, _, _ := newCommandDispatcher(t)
	if err := d.Dispatch("SET CONTROL=X"); !hasStatus(err, vmserrors.CLI_IVKEYW) {
		t.Errorf("SET CONTROL=X: %v, want IVKEYW", err)
	}
}
