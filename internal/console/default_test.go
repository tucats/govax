package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestConsoleSetShowDefault_roundTrip confirms the plain SET DEFAULT/SHOW
// DEFAULT round trip works end to end through the Console-level wrappers
// (not just internal/rms.Session directly, which session_test.go already
// covers): SetDefault stores the parsed spec, and ShowDefault prints it
// back out.
func TestConsoleSetShowDefault_roundTrip(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := c.ShowDefault(); err != nil {
		t.Fatalf("ShowDefault: %v", err)
	}

	if !strings.Contains(buf.String(), "DUA0:[MYDIR]") {
		t.Errorf("ShowDefault output = %q, want it to contain DUA0:[MYDIR]", buf.String())
	}
}

// TestConsoleSetDefault_badFileSpec confirms a malformed file specification
// is reported as the real CLI_BADFILESPEC status (not a bare, unrecognizable
// Go error), matching every other console command that reports a bad
// argument via a real VMS-style status code.
func TestConsoleSetDefault_badFileSpec(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.SetDefault("DUA0:[UNTERMINATED")
	if err == nil {
		t.Fatal("SetDefault with an unterminated directory bracket = nil error, want CLI_BADFILESPEC")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADFILESPEC)) {
		t.Errorf("SetDefault error = %v, want CLI_BADFILESPEC", err)
	}
}

// TestConsoleShowDefault_initialState confirms SHOW DEFAULT on a freshly
// constructed Console (no SET DEFAULT run yet) prints something
// displayable rather than erroring or printing nothing at all.
func TestConsoleShowDefault_initialState(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.ShowDefault(); err != nil {
		t.Fatalf("ShowDefault: %v", err)
	}

	if !strings.Contains(buf.String(), "[000000]") {
		t.Errorf("ShowDefault output on a fresh Console = %q, want it to contain [000000]", buf.String())
	}
}

// TestDispatch_setDefaultViaFixedTable exercises SET DEFAULT through the
// real Dispatcher (SET is a fixedCommands entry -- dispatch.go's cmdSet --
// not a DCL grammar verb), confirming the command line actually reaches
// Console.SetDefault with its argument threaded through correctly.
func TestDispatch_setDefaultViaFixedTable(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch("SET DEFAULT DUA0:[MYDIR]"); err != nil {
		t.Fatalf("Dispatch SET DEFAULT: %v", err)
	}

	if got, want := c.ContainerSession.DefaultString(), "DUA0:[MYDIR]"; got != want {
		t.Errorf("DefaultString() after SET DEFAULT = %q, want %q", got, want)
	}
}

// TestDispatch_setDefaultNeedsArgument confirms a bare "SET DEFAULT" with
// no file specification at all is reported as CLI_NEEDSETARG (matching
// every other SET sub-form's own "you didn't give me anything" check, e.g.
// SET RADIX/SET BASE) rather than silently doing nothing or panicking on
// an empty string.
func TestDispatch_setDefaultNeedsArgument(t *testing.T) {
	d, _ := newTestDispatcher(t)

	err := d.Dispatch("SET DEFAULT")
	if err == nil {
		t.Fatal("Dispatch \"SET DEFAULT\" with no argument = nil error, want CLI_NEEDSETARG")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_NEEDSETARG)) {
		t.Errorf("Dispatch SET DEFAULT error = %v, want CLI_NEEDSETARG", err)
	}
}

// TestDispatch_showDefaultViaDCL exercises SHOW DEFAULT through the real
// DCL grammar (internal/bootdata/files/evax.dcl's "type show_types"
// DEFAULT keyword redirecting to "syntax show_default"), confirming the
// g.Bind("SHOW_DEFAULT", ...) closure in dispatch.go actually reaches
// Console.ShowDefault. Built manually (rather than via newTestDispatcher,
// which discards its Console's output buffer) so this test can inspect
// what actually got printed.
func TestDispatch_showDefaultViaDCL(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	if err := c.SetDefault("DUB0:[OTHERDIR]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := d.Dispatch("SHOW DEFAULT"); err != nil {
		t.Fatalf("Dispatch SHOW DEFAULT: %v", err)
	}

	if !strings.Contains(buf.String(), "DUB0:[OTHERDIR]") {
		t.Errorf("Dispatch SHOW DEFAULT output = %q, want it to contain DUB0:[OTHERDIR]", buf.String())
	}
}

// TestDispatch_showDefaultAbbreviated confirms "SHOW DEF" (an unambiguous
// abbreviation of DEFAULT among show_types' own keyword list) resolves to
// the same show_default syntax as the fully spelled-out form, matching how
// every other SHOW sub-form's keyword already abbreviates.
func TestDispatch_showDefaultAbbreviated(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("SHOW DEF"); err != nil {
		t.Fatalf("Dispatch SHOW DEF: %v", err)
	}
}
