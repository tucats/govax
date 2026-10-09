package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The condition values the tests expect: CLI$_NORMAL and CLI$_IVVERB, as
// VMS 7.3 gave them (testdata/dcl50), and govax's SET BOGUS failure,
// CLI_BADPARAMETER, which VMS has no CLI$_ name for.
const (
	statusNormal = 0x00030001
	statusIvverb = 0x00038090
)

var statusBadParameter = vmsCondition(vmserrors.CLI_BADPARAMETER)

// TestConditionValue: a govax status is VMS's condition value for the same
// message, with govax's severity; one VMS has no name for keeps govax's
// code; any other error is SS$_ABORT; a message already shown sets
// STS$M_INHIB_MSG.
func TestConditionValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want uint32
	}{
		{"IVVERB", vmserrors.NewSegment(vmserrors.CLI_IVVERB, "FOOBAR"), statusIvverb},
		{"UNDSYM", vmserrors.New(vmserrors.CLI_UNDSYM), 0x00038140},
		{"a system service status", vmserrors.New(vmserrors.SS_NOSUCHFILE, "X"), vmsdef.Symbols["SS$_NOSUCHFILE"]},
		{"no VMS name", vmserrors.New(vmserrors.CLI_BADPARAMETER, "X", "Y"), vmserrors.CLI_BADPARAMETER},
		{"a Go error", errors.New("boom"), vmsdef.Symbols["SS$_ABORT"]},
		{"shown", vmserrors.InhibitMessage(vmserrors.New(vmserrors.CLI_IVVERB)), statusIvverb | stsInhibitMsg},
		{"a statusError", statusError{status: 0x2C, text: "SYSTEM-F-ABORT, abort"}, 0x2C},
	} {
		if got := conditionValue(tc.err); got != tc.want {
			t.Errorf("%s: %08X, want %08X", tc.name, got, tc.want)
		}
	}
}

// TestStatusSymbols: $STATUS and $SEVERITY are global string symbols set
// by each command; SHOW SYMBOL, CONTINUE, and a false IF leave them as
// they were (the User's Manual, 13.15).
func TestStatusSymbols(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)

	show := func(name string) string {
		buf.Reset()

		if err := d.Dispatch("SHOW SYMBOL " + name); err != nil {
			t.Fatalf("SHOW SYMBOL %s: %v", name, err)
		}

		return strings.TrimSpace(buf.String())
	}

	if err := d.Dispatch(`X = "a"`); err != nil {
		t.Fatal(err)
	}

	if got, want := show("$STATUS"), `$STATUS == "%X00030001"`; got != want {
		t.Errorf("after an assignment: %s, want %s", got, want)
	}

	_ = d.Dispatch("FOOBAR")

	// SHOW SYMBOL $STATUS doesn't change it, so $SEVERITY is FOOBAR's.
	if got, want := show("$STATUS"), `$STATUS == "%X00038090"`; got != want {
		t.Errorf("after FOOBAR: %s, want %s", got, want)
	}

	if got, want := show("$SEVERITY"), `$SEVERITY == "0"`; got != want {
		t.Errorf("after FOOBAR: %s, want %s", got, want)
	}

	for _, line := range []string{"CONTINUE", "IF 0 THEN PRINT 1"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if c.Status() != statusIvverb {
			t.Errorf("after %s: $STATUS %08X, want FOOBAR's still", line, c.Status())
		}
	}

	if err := d.Dispatch("IF 1 THEN PRINT 1"); err != nil || c.Status() != statusNormal {
		t.Errorf("a true IF: %v, $STATUS %08X; want its command's status", err, c.Status())
	}

	// $STATUS is a string holding a number, usable in an expression.
	if err := d.Dispatch("N = $STATUS .AND. 1"); err != nil {
		t.Fatal(err)
	}

	if got, want := show("N"), "N = 1   Hex = 00000001  Octal = 00000000001"; got != want {
		t.Errorf("$STATUS .AND. 1: %s, want %s", got, want)
	}
}

// runStatusProcedure runs a procedure of lines from the terminal, and
// returns @'s error, the output, and $STATUS after it.
func runStatusProcedure(t *testing.T, lines ...string) (error, string, uint32) {
	t.Helper()

	d, c, buf := newCommandDispatcher(t)
	proc := writeProcedure(t, t.TempDir(), "status.com", lines...)

	err := d.Dispatch("@" + proc)

	if c.CommandLevel() != 0 {
		t.Errorf("level %d after the procedure, want 0", c.CommandLevel())
	}

	return err, buf.String(), c.Status()
}

// TestProcedureStatus: how a procedure's ending sets $STATUS (the User's
// Manual, 13.8 and 13.14, and VMS 7.3's run of testdata/dcl50): the end
// of the file passes $STATUS on; EXIT without a status passes it marked
// as shown; EXIT with one passes that, showing its message if it's a
// failure not marked as shown; the default error action passes the
// error's status, marked as shown.
func TestProcedureStatus(t *testing.T) {
	err, out, status := runStatusProcedure(t, "$ FOOBAR")
	if status != statusIvverb || !vmserrors.MessageInhibited(err) || strings.Count(out, "IVVERB") != 1 {
		t.Errorf("end of file after a warning: %v, $STATUS %08X, output %q; want IVVERB's status, shown once", err, status, out)
	}

	_, _, status = runStatusProcedure(t, "$ FOOBAR", "$ EXIT")
	if status != statusIvverb|stsInhibitMsg {
		t.Errorf("EXIT after a warning: $STATUS %08X, want %08X", status, statusIvverb|stsInhibitMsg)
	}

	err, out, status = runStatusProcedure(t, "$ EXIT 3")
	if err != nil || out != "" || status != 3 {
		t.Errorf("EXIT 3: %v, %q, $STATUS %08X; want success, quietly", err, out, status)
	}

	err, _, status = runStatusProcedure(t, "$ EXIT 44")
	if err == nil || vmserrors.MessageInhibited(err) || err.Error() != "SYSTEM-F-ABORT, abort" || status != 44 {
		t.Errorf("EXIT 44: %v, $STATUS %08X; want SS$_ABORT's message to show", err, status)
	}

	err, _, status = runStatusProcedure(t, "$ EXIT %X10000002")
	if !vmserrors.MessageInhibited(err) || status != 0x10000002 {
		t.Errorf("EXIT %%X10000002: %v, $STATUS %08X; want it, not shown", err, status)
	}

	err, out, status = runStatusProcedure(t, "$ SET BOGUS", `$ PRINT "after"`)
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADPARAMETER)) || !vmserrors.MessageInhibited(err) ||
		status != statusBadParameter|stsInhibitMsg || strings.Contains(out, "after") {
		t.Errorf("an error: %v, $STATUS %08X, output %q; want it to end the procedure, marked as shown", err, status, out)
	}
}

// TestOnCommand: ON's actions (the User's Manual, 13.9): the severity it
// acts on and worse, its command run once and the default back after it,
// and only at its own command level.
func TestOnCommand(t *testing.T) {
	// ON WARNING THEN EXIT ends on a warning, with its status.
	_, out, status := runStatusProcedure(t, "$ ON WARNING THEN EXIT", "$ FOOBAR", `$ PRINT "after"`)
	if strings.Contains(out, "after") || status != statusIvverb|stsInhibitMsg {
		t.Errorf("ON WARNING THEN EXIT: output %q, $STATUS %08X; want it to end on FOOBAR", out, status)
	}

	// ON ERROR THEN CONTINUE goes on after one error; the default action
	// ends the procedure on the next.
	_, out, _ = runStatusProcedure(t,
		"$ ON ERROR THEN CONTINUE", "$ SET BOGUS", `$ PRINT "one"`, "$ SET BOGUS", `$ PRINT "two"`)
	if !strings.Contains(out, "one") || strings.Contains(out, "two") || strings.Count(out, "BADPARAMETER") != 2 {
		t.Errorf("ON ERROR THEN CONTINUE:\n%s\nwant one, then the end at the second error", out)
	}

	// ON SEVERE_ERROR lets an error go on; the action runs a command.
	_, out, _ = runStatusProcedure(t, `$ ON SEVERE_ERROR THEN PRINT "severe"`, "$ SET BOGUS", `$ PRINT "after"`)
	if !strings.Contains(out, "after") || strings.Contains(out, "severe") {
		t.Errorf("ON SEVERE_ERROR, an error:\n%s\nwant the procedure to go on", out)
	}

	_, out, _ = runStatusProcedure(t, `$ ON WARNING THEN PRINT "acted"`, "$ FOOBAR", `$ PRINT "after"`)
	if !strings.Contains(out, "acted\nafter") {
		t.Errorf("ON WARNING THEN PRINT:\n%s\nwant acted, then after", out)
	}

	// A nested procedure has the default action, not its caller's ON.
	dir := t.TempDir()
	inner := writeProcedure(t, dir, "inner.com", "$ SET BOGUS", `$ PRINT "inner after"`)
	_, out, _ = runStatusProcedure(t, "$ ON ERROR THEN CONTINUE", "$ @"+inner, `$ PRINT "outer after"`)

	if strings.Contains(out, "inner after") || !strings.Contains(out, "outer after") {
		t.Errorf("nested:\n%s\nwant the inner procedure to end, and the outer to go on", out)
	}
}

// TestSetNoOn: SET NOON makes a procedure go on after errors, still
// setting $STATUS; SET ON turns the checking back on (the User's Manual,
// 13.10).
func TestSetNoOn(t *testing.T) {
	err, out, status := runStatusProcedure(t, "$ SET NOON", "$ SET BOGUS", `$ PRINT "one"`, "$ SET BOGUS",
		"$ X = $STATUS", "$ SET ON", "$ SET BOGUS", `$ PRINT "two"`)

	if !strings.Contains(out, "one\n") || strings.Contains(out, "two") || strings.Count(out, "BADPARAMETER") != 3 {
		t.Errorf("output:\n%s\nwant one, then the end at the error after SET ON", out)
	}

	if !vmserrors.MessageInhibited(err) || status != statusBadParameter|stsInhibitMsg {
		t.Errorf("@: %v, $STATUS %08X", err, status)
	}

	// SET NOON at the terminal does nothing.
	d, _, _ := newCommandDispatcher(t)
	if err := d.Dispatch("SET NOON"); err != nil {
		t.Errorf("SET NOON at the terminal: %v", err)
	}
}

// TestOnSyntax: ON needs THEN and a command after it; ON CONTROL_Y keeps
// its command, separately from the error action.
func TestOnSyntax(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	if err := d.Dispatch("ON ERROR EXIT"); !hasStatus(err, vmserrors.CLI_NOTHEN) {
		t.Errorf("ON ERROR EXIT: %v, want CLI_NOTHEN", err)
	}

	if err := d.Dispatch("ON ERROR THEN"); !hasStatus(err, vmserrors.CLI_INSFPRM) {
		t.Errorf("ON ERROR THEN: %v, want CLI_INSFPRM", err)
	}

	c.levels = append(c.levels, &commandLevel{})

	for _, line := range []string{"ON CONTROL_Y THEN $ SHOW TIME", "ON ERROR THEN CONTINUE"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if got := c.ControlYAction(); got != "SHOW TIME" {
		t.Errorf("ON CONTROL_Y's command: %q, want SHOW TIME", got)
	}

	if on := c.currentLevel().on; on.command != "CONTINUE" || on.rank != rankError {
		t.Errorf("the error action: %+v, want CONTINUE at ERROR", on)
	}
}
