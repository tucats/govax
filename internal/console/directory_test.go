package console

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// mountFreshContainer is a small test helper (this file's own DIRECTORY
// tests are the first in this package to need a mounted volume with an
// operator-set default in one step) that formats a brand-new container via
// the already-tested c.InitializeContainer and mounts it on device --
// exactly the two-step INITIALIZE/MOUNT sequence a real operator would run,
// reused here so each test below doesn't repeat it.
func mountFreshContainer(t *testing.T, c *Console, device string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "dir.dsk")

	if err := c.InitializeContainer(path, 400, "DIRVOL", 0); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	if err := c.Mount(device, path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}
}

// TestConsoleDirectory_printsListing confirms the Console-level wrapper's
// happy path: a mounted, empty volume with SET DEFAULT already pointed at
// it produces a listing (its reserved files, at minimum -- INDEXF.SYS and
// friends, from volume.Initialize) printed to the console's own output
// stream, matching Console.ShowDefault's own "rms computes, console
// prints" split.
func TestConsoleDirectory_printsListing(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := c.Directory("", rms.DirectoryOptions{}); err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(buf.String(), "INDEXF.SYS;1") {
		t.Errorf("Directory output = %q, want it to contain the volume's own INDEXF.SYS;1", buf.String())
	}

	if !strings.Contains(buf.String(), "Total of") {
		t.Errorf("Directory output = %q, want it to contain a file-count summary", buf.String())
	}
}

// TestConsoleDirectory_notMounted confirms a file spec naming an unmounted
// device is reported as SS_DEVNOTMOUNT, matching Console.Dismount's own
// status for the same underlying condition -- see internal/console/
// directory.go's own doc comment for why this case is told apart from a
// merely malformed file specification.
func TestConsoleDirectory_notMounted(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Directory("DUB0:*.*;*", rms.DirectoryOptions{})
	if err == nil {
		t.Fatal("Directory against an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Directory error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestConsoleDirectory_badFileSpec confirms a malformed file specification
// is reported as CLI_BADFILESPEC, matching Console.SetDefault's own status
// for the same underlying condition.
func TestConsoleDirectory_badFileSpec(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Directory("DUA0:[UNTERMINATED", rms.DirectoryOptions{})
	if err == nil {
		t.Fatal("Directory with an unterminated directory bracket = nil error, want CLI_BADFILESPEC")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADFILESPEC)) {
		t.Errorf("Directory error = %v, want CLI_BADFILESPEC", err)
	}
}

// TestDispatch_directoryViaDCL exercises this subtask's own dispatch.go
// work directly: parsing and dispatching a real "DIRECTORY ..." command
// line through the DCL grammar (internal/bootdata/files/evax.dcl's
// directory verb) into the g.Bind("DIRECTORY", ...) closure this subtask
// added, confirming a bare DIRECTORY (SPEC omitted entirely) reaches
// Console.Directory and prints the current default directory's listing.
func TestDispatch_directoryViaDCL(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := d.Dispatch("DIRECTORY"); err != nil {
		t.Fatalf("Dispatch DIRECTORY: %v", err)
	}

	if !strings.Contains(buf.String(), "INDEXF.SYS;1") {
		t.Errorf("Dispatch DIRECTORY output = %q, want it to contain INDEXF.SYS;1", buf.String())
	}
}

// TestDispatch_directoryAbbreviated confirms "DIR" (an unambiguous 3-letter
// abbreviation -- no other verb in this grammar starts with those letters)
// resolves to the same DIRECTORY verb as the fully spelled-out form, the
// same DCL prefix-matching mechanism INIT/INITIALIZE already relies on.
func TestDispatch_directoryAbbreviated(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if err := d.Dispatch("DIR"); err != nil {
		t.Fatalf("Dispatch DIR: %v", err)
	}
}

// TestDispatch_directoryWithSpecAndQualifiers confirms an explicit file
// spec plus a qualifier (here /SIZE) both thread through the grammar
// correctly: r.String("SPEC") reaches Console.Directory as the narrowing
// pattern, and r.Present("SIZE") turns on DirectoryOptions.Size.
func TestDispatch_directoryWithSpecAndQualifiers(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	mountFreshContainer(t, c, "DUA0")

	if err := d.Dispatch("DIRECTORY DUA0:INDEXF.SYS/SIZE"); err != nil {
		t.Fatalf("Dispatch DIRECTORY DUA0:INDEXF.SYS/SIZE: %v", err)
	}

	if !strings.Contains(buf.String(), "block(s)") {
		t.Errorf("Dispatch DIRECTORY .../SIZE output = %q, want it to contain a block count", buf.String())
	}
}
