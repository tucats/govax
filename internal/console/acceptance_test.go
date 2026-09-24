package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestPhase23Acceptance_fullOperatorSession is docs/PHASE-23.md subtask
// 11's end-to-end acceptance pass: a single scripted operator session that
// dispatches real command lines -- exactly the strings a person would type
// at the govax console -- through the same Dispatcher/DCL-grammar path
// cmd/govax itself uses, exercising every command this phase added, in
// sequence, against one container:
//
//	INITIALIZE/CONTAINER -> MOUNT -> SET DEFAULT -> file creation
//	(COPY/HOST from host fixtures, the design doc's own explicitly offered
//	alternative to a full SYS$CREATE/SYS$PUT VAX-program setup) ->
//	DIRECTORY -> TYPE -> COPY (container -> host) -> DELETE -> PURGE ->
//	DISMOUNT.
//
// Every individual command already has its own focused unit tests
// (subtasks 4-10's own *_test.go files); this test's job is different --
// proving the commands compose correctly in the order a real operator
// would actually use them, sharing one Console/Dispatcher/mounted-volume
// state throughout, the way none of the per-command tests (which each
// build their own fresh fixture) do.
func TestPhase23Acceptance_fullOperatorSession(t *testing.T) {
	c, buf := newTestConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	// ---- INITIALIZE/CONTAINER: format a new, empty container file ----

	containerPath := filepath.Join(t.TempDir(), "accept.dsk")

	if err := d.Dispatch(`INITIALIZE/CONTAINER "` + containerPath + `" /SIZE=800 ACCEPT01`); err != nil {
		t.Fatalf("Dispatch INITIALIZE/CONTAINER: %v", err)
	}

	// INITIALIZE never mounts its own result (matching real VMS and this
	// project's own MOUNT/INITIALIZE split -- see the design doc's own
	// "INITIALIZE/CONTAINER" section), so nothing is usable yet until an
	// explicit MOUNT below.
	if _, ok := c.Mounts.Lookup("DUA0"); ok {
		t.Fatal("a freshly INITIALIZE/CONTAINER'd volume is already mounted, want not mounted")
	}

	// ---- MOUNT ----

	if err := d.Dispatch(`MOUNT DUA0 "` + containerPath + `"`); err != nil {
		t.Fatalf("Dispatch MOUNT: %v", err)
	}

	if label, ok := c.Mounts.VolumeLabel("DUA0"); !ok || label != "ACCEPT01" {
		t.Fatalf("VolumeLabel(DUA0) after MOUNT = %q, %v, want ACCEPT01, true", label, ok)
	}

	// ---- SET DEFAULT ----

	if err := d.Dispatch("SET DEFAULT DUA0:"); err != nil {
		t.Fatalf("Dispatch SET DEFAULT: %v", err)
	}

	// ---- file creation, via COPY/HOST from host fixtures ----

	hostDir := t.TempDir()

	fooHostPath := filepath.Join(hostDir, "foo_source.txt")
	if err := os.WriteFile(fooHostPath, []byte("acceptance pass content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(foo_source.txt): %v", err)
	}

	if err := d.Dispatch(`COPY "` + fooHostPath + `" /HOST FOO.TXT`); err != nil {
		t.Fatalf("Dispatch COPY (host -> container, FOO.TXT): %v", err)
	}

	// BAR.TXT is copied in from the host twice, on purpose: PURGE below
	// needs a name with more than one version to have anything meaningful
	// to trim, and CreateFile/Directory.Insert (this project's own
	// ODS-2-backed auto-versioning, confirmed in subtask 10's own progress
	// log) assigns BAR.TXT;1 then BAR.TXT;2 automatically, exactly as a
	// real operator repeating a COPY onto the same name would see happen.
	barHostPath := filepath.Join(hostDir, "bar_source.txt")

	if err := os.WriteFile(barHostPath, []byte("bar version one\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(bar_source.txt v1): %v", err)
	}

	if err := d.Dispatch(`COPY "` + barHostPath + `" /HOST BAR.TXT`); err != nil {
		t.Fatalf("Dispatch COPY (host -> container, BAR.TXT;1): %v", err)
	}

	if err := os.WriteFile(barHostPath, []byte("bar version two\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(bar_source.txt v2): %v", err)
	}

	if err := d.Dispatch(`COPY "` + barHostPath + `" /HOST BAR.TXT`); err != nil {
		t.Fatalf("Dispatch COPY (host -> container, BAR.TXT;2): %v", err)
	}

	// ---- DIRECTORY ----

	buf.Reset()

	if err := d.Dispatch("DIRECTORY"); err != nil {
		t.Fatalf("Dispatch DIRECTORY: %v", err)
	}

	listing := buf.String()
	for _, want := range []string{"FOO.TXT", "BAR.TXT"} {
		if !strings.Contains(listing, want) {
			t.Errorf("DIRECTORY output = %q, want it to contain %q", listing, want)
		}
	}

	// ---- TYPE ----

	buf.Reset()

	if err := d.Dispatch("TYPE FOO.TXT"); err != nil {
		t.Fatalf("Dispatch TYPE FOO.TXT: %v", err)
	}

	if !strings.Contains(buf.String(), "acceptance pass content") {
		t.Errorf("TYPE FOO.TXT output = %q, want it to contain the copied-in content", buf.String())
	}

	// ---- COPY (container -> host) ----

	fooOutPath := filepath.Join(t.TempDir(), "foo_out.txt")

	if err := d.Dispatch(`COPY FOO.TXT "` + fooOutPath + `" /HOST`); err != nil {
		t.Fatalf("Dispatch COPY (container -> host): %v", err)
	}

	gotOut, err := os.ReadFile(fooOutPath)
	if err != nil {
		t.Fatalf("ReadFile(foo_out.txt): %v", err)
	}

	if !strings.Contains(string(gotOut), "acceptance pass content") {
		t.Errorf("COPY'd-out host file content = %q, want it to contain the original content", gotOut)
	}

	// ---- DELETE ----

	buf.Reset()

	if err := d.Dispatch("DELETE FOO.TXT;1"); err != nil {
		t.Fatalf("Dispatch DELETE FOO.TXT;1: %v", err)
	}

	if !strings.Contains(buf.String(), "%DELETE-S-DELETED, FOO.TXT;1 deleted") {
		t.Errorf("DELETE output = %q, want a %%DELETE-S-DELETED confirmation", buf.String())
	}

	// FOO.TXT only ever had one version, so it's entirely gone now.
	if err := c.Type("FOO.TXT"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("TYPE of the now-deleted FOO.TXT = %v, want SS_NOSUCHFILE", err)
	}

	// ---- PURGE ----

	buf.Reset()

	if err := d.Dispatch("PURGE BAR.TXT"); err != nil {
		t.Fatalf("Dispatch PURGE BAR.TXT: %v", err)
	}

	if !strings.Contains(buf.String(), "%PURGE-S-PURGED, BAR.TXT purged") {
		t.Errorf("PURGE output = %q, want a %%PURGE-S-PURGED confirmation", buf.String())
	}

	// The default /LIMIT=1 keeps only the newest version (;2); the older
	// one (;1) should now be gone.
	if err := c.Type("BAR.TXT;1"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("TYPE of the purged BAR.TXT;1 = %v, want SS_NOSUCHFILE", err)
	}

	buf.Reset()

	if err := d.Dispatch("TYPE BAR.TXT"); err != nil {
		t.Fatalf("Dispatch TYPE BAR.TXT (surviving version): %v", err)
	}

	if !strings.Contains(buf.String(), "bar version two") {
		t.Errorf("TYPE BAR.TXT (surviving version) output = %q, want the newest version's content", buf.String())
	}

	// ---- DISMOUNT ----

	if err := d.Dispatch("DISMOUNT DUA0"); err != nil {
		t.Fatalf("Dispatch DISMOUNT: %v", err)
	}

	if _, ok := c.Mounts.Lookup("DUA0"); ok {
		t.Error("Mounts.Lookup(DUA0) after DISMOUNT = found, want not found")
	}
}
