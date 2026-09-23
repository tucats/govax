package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// createConsoleTestFileWithContent creates name as a Stream_LF file holding
// content -- unlike delete_test.go's own createConsoleTestFile (which
// creates an empty file, all that DELETE/PURGE's own tests need), TYPE's
// tests need a file with real content to print.
func createConsoleTestFileWithContent(t *testing.T, vol *volume.Volume, name, content string) {
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

	block := make([]byte, ondisk.BlockSize)
	copy(block, content)

	if err := f.WriteBlock(1, block); err != nil {
		t.Fatalf("WriteBlock(%s): %v", name, err)
	}

	if err := f.CloseWithFinalByte(uint16(len(content))); err != nil {
		t.Fatalf("Close(%s): %v", name, err)
	}
}

// TestConsoleType_printsFileContent confirms the Console-level wrapper's
// happy path: typing an existing file prints its content to the console's
// output stream.
func TestConsoleType_printsFileContent(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", "hello, world")

	if err := c.Type("FOO.TXT"); err != nil {
		t.Fatalf("Type: %v", err)
	}

	if !strings.Contains(buf.String(), "hello, world") {
		t.Errorf("Type output = %q, want it to contain the file's content", buf.String())
	}
}

// TestConsoleType_notMounted confirms a file spec naming an unmounted
// device is reported as SS_DEVNOTMOUNT, matching Console.Delete's own
// status for the same underlying condition.
func TestConsoleType_notMounted(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Type("DUB0:FOO.TXT")
	if err == nil {
		t.Fatal("Type against an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Type error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestConsoleType_notFound confirms a file spec matching nothing on the
// volume is reported as SS_NOSUCHFILE, matching Console.Delete's own
// status for the same underlying condition.
func TestConsoleType_notFound(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Type("DUA0:NOSUCHFILE.TXT")
	if err == nil {
		t.Fatal("Type of a nonexistent file = nil error, want SS_NOSUCHFILE")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Type error = %v, want SS_NOSUCHFILE", err)
	}
}

// TestConsoleType_ambiguous confirms a file spec matching more than one
// file is reported as CLI_AMBIGUOUS.
func TestConsoleType_ambiguous(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "FOO.TXT")
	createConsoleTestFile(t, vol, "BAR.TXT")

	err := c.Type("*.TXT")
	if err == nil {
		t.Fatal("Type with a wildcard matching several files = nil error, want CLI_AMBIGUOUS")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_AMBIGUOUS)) {
		t.Errorf("Type error = %v, want CLI_AMBIGUOUS", err)
	}
}

// TestConsoleType_badFileSpec confirms a malformed file specification is
// reported as CLI_BADFILESPEC, matching Console.Delete/Console.Directory's
// own catch-all status for the same underlying condition.
func TestConsoleType_badFileSpec(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	err := c.Type("DUA0:[UNTERMINATED")
	if err == nil {
		t.Fatal("Type with an unterminated directory bracket = nil error, want CLI_BADFILESPEC")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADFILESPEC)) {
		t.Errorf("Type error = %v, want CLI_BADFILESPEC", err)
	}
}

// TestDispatch_typeViaDCL exercises this subtask's own dispatch.go work
// directly: parsing and dispatching a real "TYPE ..." command line through
// the DCL grammar (internal/bootdata/files/evax.dcl's type verb) into the
// g.Bind("TYPE", ...) closure this subtask added.
func TestDispatch_typeViaDCL(t *testing.T) {
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

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", "hello, world")

	if err := d.Dispatch("TYPE FOO.TXT"); err != nil {
		t.Fatalf("Dispatch TYPE FOO.TXT: %v", err)
	}

	if !strings.Contains(buf.String(), "hello, world") {
		t.Errorf("Dispatch TYPE output = %q, want it to contain the file's content", buf.String())
	}
}

// TestDispatch_typeRequiresSpec confirms a bare TYPE with nothing typed
// after it fails as a missing required argument, matching the grammar's own
// /prompt="File specification" on SPEC.
func TestDispatch_typeRequiresSpec(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("TYPE"); err == nil {
		t.Fatal("Dispatch bare TYPE = nil error, want a missing-parameter error")
	}
}

// TestDispatch_typeAbbreviated confirms "TYP" (an unambiguous 3-letter
// abbreviation -- no other verb in this grammar starts with those letters)
// resolves to the same TYPE verb as the fully spelled-out form.
func TestDispatch_typeAbbreviated(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "FOO.TXT")

	if err := d.Dispatch("TYP FOO.TXT"); err != nil {
		t.Fatalf("Dispatch TYP FOO.TXT: %v", err)
	}
}
