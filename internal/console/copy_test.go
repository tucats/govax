package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestConsoleCopy_volumeToVolume confirms the Console-level wrapper's
// plain happy path (neither /HOST set): a mounted volume's file is copied
// onto a new file on the same volume, and a single confirmation line is
// printed.
func TestConsoleCopy_volumeToVolume(t *testing.T) {
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

	if err := c.Copy("FOO.TXT", false, "BAR.TXT", false); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !strings.Contains(buf.String(), "%COPY-S-COPIED, FOO.TXT;1 copied to DUA0:[000000]BAR.TXT;1") {
		t.Errorf("Copy output = %q, want a %%COPY-S-COPIED confirmation line", buf.String())
	}

	buf.Reset()

	if err := c.Type("BAR.TXT"); err != nil {
		t.Fatalf("Type(BAR.TXT): %v", err)
	}

	if !strings.Contains(buf.String(), "hello, world") {
		t.Errorf("Type(BAR.TXT) output = %q, want it to contain the copied content", buf.String())
	}
}

// TestConsoleCopy_fromHost confirms the Console-level wrapper's /HOST-on-
// SOURCE direction: a plain host file's content lands on the mounted
// volume under the given destination name.
func TestConsoleCopy_fromHost(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	hostPath := filepath.Join(t.TempDir(), "host.txt")
	if err := os.WriteFile(hostPath, []byte("from the host\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := c.Copy(hostPath, true, "BAZ.TXT", false); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !strings.Contains(buf.String(), "%COPY-S-COPIED, "+hostPath+" copied to DUA0:[000000]BAZ.TXT;1") {
		t.Errorf("Copy output = %q, want a %%COPY-S-COPIED confirmation line", buf.String())
	}
}

// TestConsoleCopy_toHost confirms the Console-level wrapper's /HOST-on-
// DESTINATION direction: a volume file's content lands on the host
// filesystem at the given path.
func TestConsoleCopy_toHost(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", "hello, world")

	outPath := filepath.Join(t.TempDir(), "out.txt")

	if err := c.Copy("FOO.TXT", false, outPath, true); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(got), "hello, world") {
		t.Errorf("copied host file content = %q, want it to contain %q", got, "hello, world")
	}
}

// TestConsoleCopy_notMounted confirms a file spec naming an unmounted
// device is reported as SS_DEVNOTMOUNT, matching Console.Type/Console.
// Delete's own translation for the same underlying condition.
func TestConsoleCopy_notMounted(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Copy("DUB0:FOO.TXT", false, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy from an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Copy error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestConsoleCopy_notFound confirms a source spec matching no file at all
// is reported as SS_NOSUCHFILE, matching Console.Type/Console.Delete's own
// translation for the same underlying condition.
func TestConsoleCopy_notFound(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	err := c.Copy("NOSUCH.TXT", false, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy of a nonexistent file = nil error, want SS_NOSUCHFILE")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Copy error = %v, want SS_NOSUCHFILE", err)
	}
}

// TestConsoleCopy_ambiguous confirms a wildcarded source spec matching
// more than one file is reported as CLI_AMBIGUOUS, the same status
// Console.Type already uses for its own single-match restriction.
func TestConsoleCopy_ambiguous(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFile(t, vol, "DUP.TXT")
	createConsoleTestFile(t, vol, "DUP.TXT")

	err := c.Copy("DUP.TXT;*", false, "OUT.TXT", false)
	if err == nil {
		t.Fatal("Copy of a wildcarded multi-version spec = nil error, want CLI_AMBIGUOUS")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_AMBIGUOUS)) {
		t.Errorf("Copy error = %v, want CLI_AMBIGUOUS", err)
	}
}

// TestConsoleCopy_hostToHostRejected confirms /HOST on both SOURCE and
// DESTINATION at once is reported as CLI_BADQUALIFIERCOMBO.
func TestConsoleCopy_hostToHostRejected(t *testing.T) {
	c, _ := newTestConsole(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")

	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := c.Copy(src, true, filepath.Join(dir, "b.txt"), true)
	if err == nil {
		t.Fatal("Copy with /HOST on both sides = nil error, want CLI_BADQUALIFIERCOMBO")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADQUALIFIERCOMBO)) {
		t.Errorf("Copy error = %v, want CLI_BADQUALIFIERCOMBO", err)
	}
}

// TestConsoleCopy_badFileSpec confirms a malformed file specification is
// reported as CLI_BADFILESPEC, matching Console.Type/Console.Delete/
// Console.Directory's own catch-all status for the same underlying
// condition.
func TestConsoleCopy_badFileSpec(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	err := c.Copy("DUA0:[UNTERMINATED", false, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy with an unterminated directory bracket = nil error, want CLI_BADFILESPEC")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADFILESPEC)) {
		t.Errorf("Copy error = %v, want CLI_BADFILESPEC", err)
	}
}

// TestDispatch_copyViaDCL exercises this subtask's own dispatch.go work
// directly: a plain "COPY SOURCE DESTINATION" command line, with neither
// parameter carrying /HOST, parsed and dispatched through the DCL grammar
// (internal/bootdata/files/evax.dcl's copy verb) into the g.Bind("COPY",
// ...) closure this subtask added.
func TestDispatch_copyViaDCL(t *testing.T) {
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

	if err := d.Dispatch("COPY FOO.TXT BAR.TXT"); err != nil {
		t.Fatalf("Dispatch COPY FOO.TXT BAR.TXT: %v", err)
	}

	if !strings.Contains(buf.String(), "%COPY-S-COPIED") {
		t.Errorf("Dispatch COPY output = %q, want a %%COPY-S-COPIED confirmation line", buf.String())
	}
}

// TestDispatch_copyHostToContainerAttachedNoSpace exercises COPY's own
// /HOST-on-SOURCE direction through the real DCL grammar, with /HOST
// written directly against its parameter's value with no intervening
// space ("COPY foo.txt/HOST BAR.TXT") -- docs/PHASE-23.md's own first
// COPY example.
func TestDispatch_copyHostToContainerAttachedNoSpace(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	// The temp directory's own path must not itself contain a '/' for
	// this specific no-space-before-/HOST form to parse the way the
	// design doc's own example does (a literal '/' would otherwise be
	// read as introducing a qualifier partway through the path) -- so
	// this test uses a short, relative-looking file name inside the
	// current test's working directory rather than filepath.Join with
	// t.TempDir(), matching the design doc's own "foo.txt/HOST" example
	// exactly. See TestDispatch_copyHostToContainerQuoted below for the
	// quoted, embedded-'/'-safe form every real host path actually needs.
	hostPath := "govax_copy_subtask9_test_host_file.txt"

	if err := os.WriteFile(hostPath, []byte("attached\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(hostPath) })

	if err := d.Dispatch("COPY " + hostPath + "/HOST BAR.TXT"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if err := c.Type("BAR.TXT"); err != nil {
		t.Fatalf("Type(BAR.TXT): %v", err)
	}
}

// TestDispatch_copyHostToContainerQuoted regresses docs/PHASE-23.md's own
// "Quoting" design section end to end, through the real DCL grammar and
// dispatch path: a host path that needs quoting because it contains an
// embedded '/' (an ordinary absolute path, exactly the motivating example
// in the design doc), with /HOST trailing it space-separated rather than
// attached directly -- "COPY "/tmp/.../foo.txt" /HOST BAR.TXT".
func TestDispatch_copyHostToContainerQuoted(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	hostPath := filepath.Join(t.TempDir(), "foo.txt")
	if err := os.WriteFile(hostPath, []byte("quoted host path\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := d.Dispatch(`COPY "` + hostPath + `" /HOST BAR.TXT`); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if err := c.Type("BAR.TXT"); err != nil {
		t.Fatalf("Type(BAR.TXT): %v", err)
	}
}

// TestDispatch_copyContainerToHostTrailingQualifier exercises COPY's own
// /HOST-on-DESTINATION direction through the real DCL grammar, confirming
// the parser's "most recently filled parameter" tracking correctly
// attaches a trailing /HOST to DESTINATION rather than SOURCE. The
// destination here is an absolute host path (a t.TempDir() path, starting
// with '/'), so -- per docs/PHASE-23.md's own "Quoting" design section --
// it has to be quoted (an unquoted leading '/' would otherwise be
// misread as introducing a qualifier immediately), with /HOST trailing it
// space-separated, exactly like the design doc's own quoted SOURCE
// example but applied to DESTINATION instead.
func TestDispatch_copyContainerToHostTrailingQualifier(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", "container to host")

	outPath := filepath.Join(t.TempDir(), "out.txt")

	if err := d.Dispatch(`COPY FOO.TXT "` + outPath + `" /HOST`); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(got), "container to host") {
		t.Errorf("copied host file content = %q, want it to contain %q", got, "container to host")
	}
}

// TestDispatch_copyRequiresBothParameters confirms a bare COPY with
// nothing typed after it fails as a missing required argument, matching
// the grammar's own /prompt= on both SOURCE and DESTINATION.
func TestDispatch_copyRequiresBothParameters(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Dispatch("COPY"); err == nil {
		t.Fatal("Dispatch bare COPY = nil error, want a missing-parameter error")
	}
}
