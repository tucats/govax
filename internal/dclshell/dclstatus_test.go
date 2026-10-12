package dclshell

import (
	"errors"
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
		{"a vmserrors.ConditionError", vmserrors.ConditionError{Status: 0x2C, Text: "SYSTEM-F-ABORT, abort"}, 0x2C},
	} {
		if got := ConditionValue(tc.err); got != tc.want {
			t.Errorf("%s: %08X, want %08X", tc.name, got, tc.want)
		}
	}
}

// TestOnSyntax: ON needs THEN and a command after it; ON CONTROL_Y keeps
// its command, separately from the error action.
func TestOnSyntax(t *testing.T) {
	d, c := newTestDispatcher(t)

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
