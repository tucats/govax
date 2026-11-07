package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sysvol"
)

// The system volume's refresh on a real volume (docs/PHASE-87 - system
// volume.md, Decision 3 and subtask 4); internal/sysvol's tests take each
// case on its own.

// volumeText reads the highest version of spec on DUA0 in its canonical
// form for f.
func volumeText(t *testing.T, c *Console, spec string, f sysvol.Format) []byte {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:" + spec}, rms.TextRecords)
	if err != nil {
		t.Fatalf("%s: %v", spec, err)
	}

	return sysvol.Join(records, f)
}

// writeVolumeFile writes contents, canonical for f, as a new version of
// spec on DUA0, as RMS would for a user's COPY or EDIT.
func writeVolumeFile(t *testing.T, c *Console, spec string, contents []byte, f sysvol.Format) {
	t.Helper()

	kind := rms.TextRecords
	if f == sysvol.Binary {
		kind = rms.ImageBlocks
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:" + spec}, kind, sysvol.Records(contents, f)); err != nil {
		t.Fatalf("%s: %v", spec, err)
	}
}

// versionCount returns how many versions of spec DUA0 has.
func versionCount(t *testing.T, c *Console, spec string) int {
	t.Helper()

	names, err := c.ContainerSession.Search("DUA0:" + spec + ";*")
	if err != nil {
		return 0
	}

	return len(names)
}

// TestSystemVolumeRefresh makes a volume look as an older govax left it,
// and a user after it: two files of govax's with older contents the
// manifest records (one text, one binary), a file the user changed whose
// govax copy has changed too, one the user changed that govax hasn't, and
// a file deleted. The next start updates the first two as new versions,
// writes the deleted one, keeps the user's, and says so once; the start
// after it does nothing.
func TestSystemVolumeRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	c, _, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Created || !r.Writable {
		t.Fatalf("first start = %+v", r)
	}

	help := sysvol.HelpSources + "COPY.HLP"
	image := sysvol.SysLib + "FTNRTL.EXE"
	startup := sysvol.SysMgr + "STARTUP.COM"
	grammar := sysvol.SysMgr + "DEBUG.DCL"
	debugHelp := sysvol.DebugHelpSources + "DEBUG.HLP"

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:" + sysvol.ManifestSpec}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	m, err := sysvol.ParseManifest(records)
	if err != nil {
		t.Fatal(err)
	}

	// An older govax's copies, recorded in the manifest as its own.
	oldHelp := sysvol.Canonical([]byte("1 HELP\n  An older govax's help.\n"), sysvol.Text)
	oldImage := sysvol.Canonical([]byte("an older FTNRTL"), sysvol.Binary)

	writeVolumeFile(t, c, help, oldHelp, sysvol.Text)
	writeVolumeFile(t, c, image, oldImage, sysvol.Binary)
	m.Set(help, sysvol.SumOf(oldHelp))
	m.Set(image, sysvol.SumOf(oldImage))

	// The user's STARTUP.COM, over an older govax's (whose digest the
	// manifest has), and the user's DEBUG.DCL over this govax's.
	mine := sysvol.Canonical([]byte("$ ! my startup\n"), sysvol.Text)
	writeVolumeFile(t, c, startup, mine, sysvol.Text)
	m.Set(startup, sysvol.SumOf([]byte("an older govax's STARTUP.COM\n")))

	myGrammar := sysvol.Canonical([]byte("! my grammar\n"), sysvol.Text)
	writeVolumeFile(t, c, grammar, myGrammar, sysvol.Text)

	if _, err := c.ContainerSession.Delete("DUA0:" + debugHelp + ";*"); err != nil {
		t.Fatal(err)
	}

	writeVolumeFile(t, c, sysvol.ManifestSpec, bytes.Join(append(m.Records(), nil), []byte{'\n'}), sysvol.Text)

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	// The next start.
	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted || !r.Writable || r.Created {
		t.Fatalf("second start = %+v; output:\n%s", r, buf)
	}

	out := buf.String()
	for _, want := range []string{
		"%MOUNT-I-SYSKEPT, DUA0:" + startup + " was changed, govax's new version not installed\n",
		"%MOUNT-I-SYSUPDATED, 3 files on system volume DUA0: updated by this govax\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	if strings.Count(out, "SYSKEPT") != 1 || strings.Contains(out, "-W-") {
		t.Errorf("output:\n%s", out)
	}

	for _, check := range []struct {
		spec     string
		format   sysvol.Format
		versions int
		want     []byte // nil: govax's copy
	}{
		{help, sysvol.Text, 3, nil},
		{image, sysvol.Binary, 3, nil},
		{debugHelp, sysvol.Text, 1, nil},
		{startup, sysvol.Text, 2, mine},
		{grammar, sysvol.Text, 2, myGrammar},
	} {
		want := check.want
		if want == nil {
			f, _ := sysvol.Lookup(check.spec)
			want, _ = f.Contents()
		}

		if got := volumeText(t, c, check.spec, check.format); !bytes.Equal(got, want) {
			t.Errorf("%s: the highest version isn't what it should be", check.spec)
		}

		if got := versionCount(t, c, check.spec); got != check.versions {
			t.Errorf("%s: %d versions, want %d", check.spec, got, check.versions)
		}
	}

	manifests := versionCount(t, c, sysvol.ManifestSpec)

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	// And the start after it: nothing to say, nothing written.
	c, buf, _ = mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	if out := buf.String(); strings.Contains(out, "SYSKEPT") || strings.Contains(out, "SYSUPDATED") {
		t.Errorf("a third start reported changes:\n%s", out)
	}

	if got := versionCount(t, c, sysvol.ManifestSpec); got != manifests {
		t.Errorf("the manifest has %d versions, was %d", got, manifests)
	}
}

// TestSystemVolumeRefreshReadOnly checks that a volume another session
// holds isn't refreshed: the first session's older file stays.
func TestSystemVolumeRefreshReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	first, _, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	help := sysvol.HelpSources + "COPY.HLP"
	writeVolumeFile(t, first, help, sysvol.Canonical([]byte("1 HELP\n"), sysvol.Text), sysvol.Text)

	_, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Shared {
		t.Fatalf("second session = %+v", r)
	}

	if out := buf.String(); strings.Contains(out, "SYSKEPT") || strings.Contains(out, "SYSUPDATED") || strings.Contains(out, "SYSFILES") {
		t.Errorf("a read-only volume was refreshed:\n%s", out)
	}
}

// TestSystemVolumeRefreshDamaged checks a manifest that can't be read: a
// warning, the volume mounted, and nothing written.
func TestSystemVolumeRefreshDamaged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	c, _, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	writeVolumeFile(t, c, sysvol.ManifestSpec, []byte("[SYSTEM.SYSHLP.SOURCES]COPY.HLP 1234\n"), sysvol.Text)

	if _, err := c.ContainerSession.Delete("DUA0:" + sysvol.DebugHelpSources + "DEBUG.HLP;*"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted || !r.Writable {
		t.Fatalf("result = %+v; output:\n%s", r, buf)
	}

	if want := "%MOUNT-W-SYSFILES, system volume DUA0: could not be brought up to date: GOVAX$FILES.DAT, record 1: "; !strings.Contains(buf.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, buf)
	}

	if !strings.Contains(buf.String(), "record 1") {
		t.Errorf("the warning doesn't name the manifest's record:\n%s", buf)
	}

	if versionCount(t, c, sysvol.DebugHelpSources+"DEBUG.HLP") != 0 {
		t.Error("a file was written past a damaged manifest")
	}
}

// TestStartupFromSystemVolume runs STARTUP.COM from the system volume as
// cmd/govax does (RunStartup), and checks what it left: the system logical
// names, the microkernel placed and its initialization run by the debugger
// from the procedure's data lines after its VMINIT (the debugger exited
// again), the terminal defined, and SYSTARTUP_VMS.COM run last.
func TestStartupFromSystemVolume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted || !r.Writable {
		t.Fatalf("result = %+v", r)
	}

	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	// The site's startup, run last: it can see what STARTUP.COM made.
	writeVolumeFile(t, c, sysvol.SysMgr+"SYSTARTUP_VMS.COM", sysvol.Canonical([]byte(
		"$ write sys$output \"site startup: \", f$trnlnm(\"SYS$SHARE\")\n"), sysvol.Text), sysvol.Text)

	// The volume's STARTUP.COM, with a line of the user's, so that it shows
	// where it ran from.
	f, _ := sysvol.Lookup(sysvol.SysMgr + "STARTUP.COM")
	startup, _ := f.Contents()
	writeVolumeFile(t, c, f.Spec(), append(startup, "$ write sys$output \"the volume's own\"\n"...), sysvol.Text)

	buf.Reset()

	if err := c.RunStartup(d.Dispatch); err != nil {
		t.Fatalf("RunStartup: %v\n%s", err, buf)
	}

	for name, want := range map[string]string{
		dclcmd.SystemDeviceName: "_DUA0:",
		dclcmd.SystemRootName:   "_DUA0:[SYSTEM]",
		"SYS$LIBRARY":           "SYS$SYSDEVICE:[SYSTEM.SYSLIB]",
		"SYS$MANAGER":           "SYS$SYSDEVICE:[SYSTEM.SYSMGR]",
	} {
		e, err := c.Logicals.Translate(lnm.SystemTableName, name, lnm.Executive, 0)
		if err != nil || e.Equivalences[0].Value != want {
			t.Errorf("%s = %v, %v; want %s", name, e, err, want)
		}
	}

	if !c.KernelPlaced() || c.InDebugger() {
		t.Errorf("after STARTUP.COM: kernel placed %v, in the debugger %v\n%s", c.KernelPlaced(), c.InDebugger(), buf)
	}

	if _, found := c.Devices.Find("TTA0"); !found {
		t.Error("STARTUP.COM didn't define TTA0:")
	}

	if want := "site startup: SYS$SYSDEVICE:[SYSTEM.SYSLIB]\n"; !strings.Contains(buf.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, buf)
	}

	if !strings.Contains(buf.String(), "the volume's own\n") {
		t.Errorf("STARTUP.COM didn't run from the volume:\n%s", buf)
	}
}
