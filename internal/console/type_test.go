package console

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	if err := c.cmd().SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", "hello, world")

	if err := c.cmd().Type("FOO.TXT", false, false); err != nil {
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

	err := c.cmd().Type("DUB0:FOO.TXT", false, false)
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

	err := c.cmd().Type("DUA0:NOSUCHFILE.TXT", false, false)
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

	if err := c.cmd().SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "FOO.TXT")
	createConsoleTestFile(t, vol, "BAR.TXT")

	err := c.cmd().Type("*.TXT", false, false)
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

	err := c.cmd().Type("DUA0:[UNTERMINATED", false, false)
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

	if err := c.cmd().SetDefault("DUA0:"); err != nil {
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

	if err := c.cmd().SetDefault("DUA0:"); err != nil {
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

// writeHostTextFile writes content to a new host file named name in a
// test's temporary directory and returns its path.
func writeHostTextFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}

	return path
}

// TestConsoleType_hostPath: a name with a host path is a host file, typed
// line by line, a CR before each LF dropped and a last line with no LF
// still a line.
func TestConsoleType_hostPath(t *testing.T) {
	c, buf := newTestConsole(t)
	path := writeHostTextFile(t, "notes.txt", "first\r\nsecond\nlast")

	if err := c.cmd().Type(path, false, false); err != nil {
		t.Fatalf("Type(%s): %v", path, err)
	}

	if want := "first\nsecond\nlast\n"; buf.String() != want {
		t.Errorf("Type output = %q, want %q", buf.String(), want)
	}
}

// TestConsoleType_hostBareName: with the default directory not on a
// mounted volume, a bare name is a host file.
func TestConsoleType_hostBareName(t *testing.T) {
	c, buf := newTestConsole(t)
	path := writeHostTextFile(t, "BARE.TXT", "bare host file\n")
	t.Chdir(filepath.Dir(path))

	if err := c.cmd().Type("BARE.TXT", false, false); err != nil {
		t.Fatalf("Type(BARE.TXT): %v", err)
	}

	if buf.String() != "bare host file\n" {
		t.Errorf("Type output = %q, want the host file's content", buf.String())
	}
}

// TestConsoleType_hostForced: with SET DEFAULT on a volume, a bare name is
// a volume file, and /HOST makes it a host file instead.
func TestConsoleType_hostForced(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.cmd().SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", "volume file")

	path := writeHostTextFile(t, "FOO.TXT", "host file\n")
	t.Chdir(filepath.Dir(path))

	if err := c.cmd().Type("FOO.TXT", false, false); err != nil {
		t.Fatalf("Type(FOO.TXT): %v", err)
	}

	if !strings.Contains(buf.String(), "volume file") {
		t.Errorf("Type without /HOST = %q, want the volume file", buf.String())
	}

	buf.Reset()

	if err := c.cmd().Type("FOO.TXT", true, false); err != nil {
		t.Fatalf("Type(FOO.TXT, host): %v", err)
	}

	if buf.String() != "host file\n" {
		t.Errorf("Type with /HOST = %q, want the host file", buf.String())
	}
}

// TestConsoleType_hostNotFound: a host file that doesn't exist is
// SS_NOSUCHFILE, as a volume file is.
func TestConsoleType_hostNotFound(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.cmd().Type(filepath.Join(t.TempDir(), "missing.txt"), true, false)
	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Type of a missing host file = %v, want SS_NOSUCHFILE", err)
	}
}

// TestConsoleType_hostPaged: TYPE/PAGE pages a host file as it does a
// volume file.
func TestConsoleType_hostPaged(t *testing.T) {
	c, buf := pagedConsole(t, 5, 0, "q\n")
	path := writeHostTextFile(t, "long.txt", numberedLines(10))

	if err := c.cmd().Type(path, true, true); err != nil {
		t.Fatalf("Type(%s, paged): %v", path, err)
	}

	out := buf.String()
	if !strings.Contains(out, "line 4\n"+pagePrompt) || strings.Contains(out, "line 5") {
		t.Errorf("Type/PAGE output = %q, want it to stop after the first screen", out)
	}
}

// TestDispatch_typeHost: /HOST may follow the verb or the file name.
func TestDispatch_typeHost(t *testing.T) {
	for _, form := range []string{"TYPE/HOST %q", "TYPE %q/HOST", "TYPE/PAGE %q/HOST"} {
		c, buf := newTestConsole(t)
		d := NewDispatcher(c, loadEvaxGrammar(t), nil)
		c.ScreenSize = func() (int, int) { return 24, 0 }
		path := writeHostTextFile(t, "notes.txt", "host notes\n")

		line := fmt.Sprintf(form, path)
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("Dispatch %s: %v", line, err)
		}

		if buf.String() != "host notes\n" {
			t.Errorf("Dispatch %s output = %q, want the host file's content", line, buf.String())
		}
	}
}
