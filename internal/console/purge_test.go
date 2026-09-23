package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestConsolePurge_notMounted confirms a file spec naming an unmounted
// device is reported as SS_DEVNOTMOUNT, matching Console.Delete's own
// status for the same underlying condition.
func TestConsolePurge_notMounted(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Purge("DUB0:*.*", 1)
	if err == nil {
		t.Fatal("Purge against an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Purge error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestConsolePurge_badLimit confirms a /LIMIT=0 is reported as CLI_BADLIMIT.
func TestConsolePurge_badLimit(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Purge("DUA0:INDEXF.SYS", 0)
	if err == nil {
		t.Fatal("Purge with /LIMIT=0 = nil error, want CLI_BADLIMIT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADLIMIT)) {
		t.Errorf("Purge error = %v, want CLI_BADLIMIT", err)
	}
}

// TestConsolePurge_badFileSpec confirms a malformed file specification is
// reported as CLI_BADFILESPEC, matching Console.Delete's own catch-all
// status for the same underlying condition.
func TestConsolePurge_badFileSpec(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Purge("DUA0:[UNTERMINATED", 1)
	if err == nil {
		t.Fatal("Purge with an unterminated directory bracket = nil error, want CLI_BADFILESPEC")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADFILESPEC)) {
		t.Errorf("Purge error = %v, want CLI_BADFILESPEC", err)
	}
}

// TestConsolePurge_trimsVersionsAndPrintsConfirmation confirms the
// Console-level wrapper's happy path: purging a name with several versions
// leaves only the newest and prints the same "%PURGE-S-PURGED, ..."
// confirmation line real VMS (and ods2's own cmdPurge, this command's
// behavioral reference) would.
func TestConsolePurge_trimsVersionsAndPrintsConfirmation(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "FOO.TXT")
	createConsoleTestFile(t, vol, "FOO.TXT")

	if err := c.Purge("FOO.TXT", 1); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	if !strings.Contains(buf.String(), "%PURGE-S-PURGED, FOO.TXT purged (keeping 1 version(s))") {
		t.Errorf("Purge output = %q, want a PURGE-S-PURGED confirmation", buf.String())
	}

	if err := c.Delete("FOO.TXT;1"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Delete of the purged-away version = %v, want SS_NOSUCHFILE", err)
	}

	if err := c.Delete("FOO.TXT;2"); err != nil {
		t.Errorf("Delete of the surviving version = %v, want success", err)
	}
}

// TestDispatch_purgeViaDCL exercises this subtask's own dispatch.go work
// directly: parsing and dispatching a real "PURGE ..." command line through
// the DCL grammar (internal/bootdata/files/evax.dcl's purge verb) into the
// g.Bind("PURGE", ...) closure this subtask added.
func TestDispatch_purgeViaDCL(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "FOO.TXT")
	createConsoleTestFile(t, vol, "FOO.TXT")

	if err := d.Dispatch("PURGE FOO.TXT"); err != nil {
		t.Fatalf("Dispatch PURGE FOO.TXT: %v", err)
	}

	if !strings.Contains(buf.String(), "%PURGE-S-PURGED, FOO.TXT purged (keeping 1 version(s))") {
		t.Errorf("Dispatch PURGE output = %q, want a PURGE-S-PURGED confirmation", buf.String())
	}
}

// TestDispatch_purgeExplicitLimit confirms an explicit /LIMIT qualifier
// threads through the grammar correctly, overriding the default of 1.
func TestDispatch_purgeExplicitLimit(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "FOO.TXT")
	createConsoleTestFile(t, vol, "FOO.TXT")
	createConsoleTestFile(t, vol, "FOO.TXT")

	if err := d.Dispatch("PURGE FOO.TXT/LIMIT=2"); err != nil {
		t.Fatalf("Dispatch PURGE FOO.TXT/LIMIT=2: %v", err)
	}

	if !strings.Contains(buf.String(), "%PURGE-S-PURGED, FOO.TXT purged (keeping 2 version(s))") {
		t.Errorf("Dispatch PURGE/LIMIT=2 output = %q, want a keeping-2 confirmation", buf.String())
	}

	if err := c.Delete("FOO.TXT;1"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Delete of the purged-away version = %v, want SS_NOSUCHFILE", err)
	}
}

// TestDispatch_purgeBareDefaultsToStar confirms a bare PURGE (SPEC omitted
// entirely) dispatches successfully -- unlike DELETE's SPEC, PURGE's has no
// /prompt=, so there's no sensible default to fall back to other than
// internal/rms.Session.Purge's own "*.*".
func TestDispatch_purgeBareDefaultsToStar(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := d.Dispatch("PURGE"); err != nil {
		t.Fatalf("Dispatch bare PURGE: %v", err)
	}
}

// TestDispatch_purgeAbbreviated confirms "PURG" (an unambiguous 4-letter
// abbreviation -- no other verb in this grammar starts with those letters)
// resolves to the same PURGE verb as the fully spelled-out form.
func TestDispatch_purgeAbbreviated(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := d.Dispatch("PURG"); err != nil {
		t.Fatalf("Dispatch PURG: %v", err)
	}
}
