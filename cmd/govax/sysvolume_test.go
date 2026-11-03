package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/rms"
)

// TestMain starts every test in this package with no system volume
// (docs/PHASE-87 - system volume.md): run would otherwise mount, and the
// first time make, the user's own ~/.govax/govax-system.dsk, or one a
// profile's vax.default.system.file names. A test of the system volume
// sets systemOptions itself (withSystemVolume).
func TestMain(m *testing.M) {
	systemOptions = systemFlags{file: "none"}

	os.Exit(m.Run())
}

// withSystemVolume gives one test the system volume options flags, put
// back when it ends.
func withSystemVolume(t *testing.T, flags systemFlags) {
	t.Helper()

	saved := systemOptions
	systemOptions = flags

	t.Cleanup(func() { systemOptions = saved })
}

// TestRun_systemVolume runs govax with a system volume of its own, by
// the options, three times: the first start makes it (an RD54, the
// default, timed), a one-shot command reads it, and --system-initialize
// makes it again, keeping the old one.
func TestRun_systemVolume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sys", "govax-system.dsk")
	withSystemVolume(t, systemFlags{file: path})

	start := time.Now()

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	t.Logf("first start, making an RD54 system volume: %v", time.Since(start))

	for _, want := range []string{
		"%MOUNT-I-CREATED, container " + path + " created as an RD54 volume labeled GOVAXSYS\n",
		"%MOUNT-I-MOUNTED, GOVAXSYS mounted on _DUA0:\n",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("first start's output lacks %q:\n%s", want, buf.String())
		}
	}

	// A one-shot command sees its files; the volume is mounted quietly.
	buf.Reset()

	if err := run(nil, 0, 0, &buf, emptyStdin(), []string{"DIRECTORY DUA0:[SYSTEM.SYSLIB]"}); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	out := buf.String()
	for _, want := range []string{"FTNRTL.EXE;1", "FORSYSDEF.TLB;1"} {
		if !strings.Contains(out, want) {
			t.Errorf("DIRECTORY's output lacks %s:\n%s", want, out)
		}
	}

	if strings.Contains(out, "%MOUNT-") {
		t.Errorf("a one-shot command shows mount messages:\n%s", out)
	}

	// --system-initialize, and the device moved by --system-device.
	withSystemVolume(t, systemFlags{file: path, device: "DUB", initialize: true})
	buf.Reset()

	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	if want := "%MOUNT-I-SYSINIT, system volume " + path + " initialized again; the old one is " + path + ".old\n"; !strings.Contains(buf.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, buf.String())
	}

	if !strings.Contains(buf.String(), "GOVAXSYS mounted on _DUB0:") {
		t.Errorf("not mounted on DUB0:\n%s", buf.String())
	}

	if _, err := os.Stat(path + ".old"); err != nil {
		t.Errorf("the old volume: %v", err)
	}
}

// TestRun_mountNextUnit checks --mount with a controller alone: the next
// unit after the system volume's.
func TestRun_mountNextUnit(t *testing.T) {
	dir := t.TempDir()
	withSystemVolume(t, systemFlags{file: filepath.Join(dir, "govax-system.dsk")})

	work := filepath.Join(dir, "work.dsk")

	if err := rms.InitializeContainer(work, 0, "WORK", 0, "RX50"); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	saved := mountRequests
	mountRequests = []mountRequest{{device: "DUA", path: work}}

	t.Cleanup(func() { mountRequests = saved })

	if err := run(nil, 0, 0, &buf, emptyStdin(), []string{"SHOW DEVICES DUA1"}); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	if !strings.Contains(buf.String(), "WORK") {
		t.Errorf("DUA1 isn't the work volume:\n%s", buf.String())
	}
}
