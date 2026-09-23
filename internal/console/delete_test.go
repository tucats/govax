package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// createConsoleTestFile creates name (e.g. "FOO.TXT") in vol's master file
// directory with a small Stream_LF record -- the same volume.CreateFile
// call internal/rms/directory_test.go's own createTestFile makes, reused
// here at the Console-test level (which can't call that unexported helper
// directly, since it lives in a different package) so this file's DELETE
// tests have a real, ordinary file to remove rather than only exercising
// DELETE against the volume's own reserved files.
func createConsoleTestFile(t *testing.T, vol *volume.Volume, name string) {
	t.Helper()

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	bm, err := dir.Device.Bitmap()
	if err != nil {
		t.Fatalf("Bitmap: %v", err)
	}

	ib, err := dir.Device.IndexBitmap()
	if err != nil {
		t.Fatalf("IndexBitmap: %v", err)
	}

	f, err := vol.CreateFile(dir, name, ondisk.RecAttr{
		Format:        ondisk.RecordFormatStreamLF,
		MaxRecordSize: 512,
	}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile(%s): %v", name, err)
	}

	if err := f.CloseWithFinalByte(0); err != nil {
		t.Fatalf("Close(%s): %v", name, err)
	}
}

// TestConsoleDelete_notMounted confirms a file spec naming an unmounted
// device is reported as SS_DEVNOTMOUNT, matching Console.Directory's own
// status for the same underlying condition.
func TestConsoleDelete_notMounted(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Delete("DUB0:FOO.TXT;1")
	if err == nil {
		t.Fatal("Delete against an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Delete error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestConsoleDelete_versionRequired confirms a file spec with no version at
// all is reported as CLI_NEEDVERSION -- DELETE's own deliberate refusal to
// default to "the newest version" the way DIRECTORY would.
func TestConsoleDelete_versionRequired(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Delete("DUA0:INDEXF.SYS")
	if err == nil {
		t.Fatal("Delete with no version = nil error, want CLI_NEEDVERSION")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_NEEDVERSION)) {
		t.Errorf("Delete error = %v, want CLI_NEEDVERSION", err)
	}
}

// TestConsoleDelete_notFound confirms a file spec matching nothing on the
// volume is reported as SS_NOSUCHFILE.
func TestConsoleDelete_notFound(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Delete("DUA0:NOSUCHFILE.TXT;1")
	if err == nil {
		t.Fatal("Delete of a nonexistent file = nil error, want SS_NOSUCHFILE")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Delete error = %v, want SS_NOSUCHFILE", err)
	}
}

// TestConsoleDelete_badFileSpec confirms a malformed file specification is
// reported as CLI_BADFILESPEC, matching Console.SetDefault/Console.
// Directory's own status for the same underlying condition.
func TestConsoleDelete_badFileSpec(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Delete("DUA0:[UNTERMINATED")
	if err == nil {
		t.Fatal("Delete with an unterminated directory bracket = nil error, want CLI_BADFILESPEC")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADFILESPEC)) {
		t.Errorf("Delete error = %v, want CLI_BADFILESPEC", err)
	}
}

// TestConsoleDelete_removesFileAndPrintsConfirmation confirms the
// Console-level wrapper's happy path: deleting an existing, ordinary file
// removes it from the volume and prints the same "%DELETE-S-DELETED, ..."
// confirmation line real VMS (and ods2's own cmdDelete, this command's
// behavioral reference) would.
func TestConsoleDelete_removesFileAndPrintsConfirmation(t *testing.T) {
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

	if err := c.Delete("FOO.TXT;1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if !strings.Contains(buf.String(), "%DELETE-S-DELETED, FOO.TXT;1 deleted") {
		t.Errorf("Delete output = %q, want a DELETE-S-DELETED confirmation", buf.String())
	}

	if err := c.Delete("FOO.TXT;1"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Delete of the now-removed file = %v, want SS_NOSUCHFILE", err)
	}
}

// TestDispatch_deleteViaDCL exercises this subtask's own dispatch.go work
// directly: parsing and dispatching a real "DELETE ..." command line
// through the DCL grammar (internal/bootdata/files/evax.dcl's delete verb)
// into the g.Bind("DELETE", ...) closure this subtask added.
func TestDispatch_deleteViaDCL(t *testing.T) {
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

	if err := d.Dispatch("DELETE FOO.TXT;1"); err != nil {
		t.Fatalf("Dispatch DELETE FOO.TXT;1: %v", err)
	}

	if !strings.Contains(buf.String(), "%DELETE-S-DELETED, FOO.TXT;1 deleted") {
		t.Errorf("Dispatch DELETE output = %q, want a DELETE-S-DELETED confirmation", buf.String())
	}
}

// TestDispatch_deleteRequiresSpec confirms a bare DELETE with nothing typed
// after it fails as a missing required argument, matching the grammar's own
// /prompt="File specification" on SPEC -- unlike DIRECTORY's SPEC, DELETE
// has no sensible "delete everything" default to fall back to.
func TestDispatch_deleteRequiresSpec(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("DELETE"); err == nil {
		t.Fatal("Dispatch bare DELETE = nil error, want a missing-parameter error")
	}
}

// TestDispatch_deleteAbbreviated confirms "DEL" (an unambiguous 3-letter
// abbreviation -- the nearest other verb starting with those letters is
// DEFINE, which diverges at the third character) resolves to the same
// DELETE verb as the fully spelled-out form.
func TestDispatch_deleteAbbreviated(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	err := d.Dispatch("DEL DUA0:BITMAP.SYS;7")
	if err == nil {
		t.Fatal("Dispatch DEL of a nonexistent version = nil error, want SS_NOSUCHFILE")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Dispatch DEL error = %v, want SS_NOSUCHFILE", err)
	}
}
