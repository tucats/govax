package console

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/hostlock"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sysvol"
)

// The system volume's tests (docs/PHASE-87 - system volume.md, subtask
// 3). They make their volumes RD51s, big enough for every file and
// quicker to make than the default RD54.
const testSystemType = "RD51"

// mountTestSystemVolume mounts a system volume at path on a new console,
// dismounted when the test ends, and returns the console, its output, and
// what MountSystemVolume did.
func mountTestSystemVolume(t *testing.T, v dclcmd.SystemVolume) (*Console, *bytes.Buffer, dclcmd.SystemVolumeResult) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Verbose = true

	t.Cleanup(func() { _ = c.Mounts.DismountAll() })

	if v.Type == "" {
		v.Type = testSystemType
	}

	return c, buf, c.cmd().MountSystemVolume(v)
}

// TestSystemVolumeCreated checks a first start: the container made, with
// its label, mounted writable on DUA0, recorded as the system volume, and
// every file of internal/sysvol's table on it with the contents govax
// carries, and the manifest.
func TestSystemVolumeCreated(t *testing.T) {
	start := time.Now()
	path := filepath.Join(t.TempDir(), "profile", "govax-system.dsk")

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	t.Logf("made and filled in %v", time.Since(start))

	if !r.Mounted || !r.Writable || !r.Created || r.Shared || r.Device != "DUA0" || r.Path != path {
		t.Fatalf("result = %+v; output:\n%s", r, buf)
	}

	out := buf.String()
	for _, want := range []string{
		"%MOUNT-I-CREATED, container " + path + " created as an RD51 volume labeled GOVAXSYS\n",
		"%MOUNT-I-MOUNTED, GOVAXSYS mounted on _DUA0:\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "-W-") || strings.Contains(out, "-E-") {
		t.Errorf("a warning or error:\n%s", out)
	}

	if c.Mounts.System() != "DUA0" || !c.Mounts.IsSystem("_dua0:") {
		t.Errorf("system device = %q", c.Mounts.System())
	}

	for _, f := range sysvol.Files() {
		want, err := f.Contents()
		if err != nil {
			t.Fatal(err)
		}

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:" + f.Spec()}, rms.TextRecords)
		if err != nil {
			t.Errorf("%s: %v", f.Spec(), err)

			continue
		}

		if got := sysvol.Join(records, f.Format); !bytes.Equal(got, want) {
			t.Errorf("%s reads back as %d bytes, not its %d", f.Spec(), len(got), len(want))
		}
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:" + sysvol.ManifestSpec}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	m, err := sysvol.ParseManifest(records)
	if err != nil {
		t.Fatal(err)
	}

	// Every supplied file has a digest, and the seeded one is marked.
	for _, f := range sysvol.Files() {
		_, summed := m.Sum(f.Spec())
		if summed != (f.Kind == sysvol.Supplied) || m.Seeded(f.Spec()) != (f.Kind == sysvol.Seeded) {
			t.Errorf("%s: in the manifest with a digest %v, seeded %v", f.Spec(), summed, m.Seeded(f.Spec()))
		}
	}

	// A binary file has LINK's attributes, and a text file CREATE's.
	for spec, want := range map[string]string{
		sysvol.SysLib + "FTNRTL.EXE": "Fixed length 512 byte records",
		sysvol.SysMgr + "STARTUP.COM": "Variable length",
	} {
		buf.Reset()

		if err := c.cmd().Directory("DUA0:"+spec, rms.DirectoryOptions{Full: true}); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(buf.String(), want) {
			t.Errorf("DIRECTORY/FULL %s lacks %q:\n%s", spec, want, buf)
		}
	}
}

// TestSystemVolumeReused checks a later start: the container found and
// mounted as it is, refreshed, and nothing written again.
func TestSystemVolumeReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	first, _, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Created {
		t.Fatalf("first start = %+v", r)
	}

	if err := first.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted || !r.Writable || r.Created {
		t.Fatalf("second start = %+v; output:\n%s", r, buf)
	}

	// The refresh reads every file at every start (Decision 3).
	t.Logf("mounted and refreshed in %v", time.Since(start))

	if strings.Contains(buf.String(), "CREATED") || strings.Contains(buf.String(), "SYSUPDATED") {
		t.Errorf("made again:\n%s", buf)
	}

	// One version of each file: nothing was written twice.
	names, err := c.ContainerSession.Search("DUA0:" + sysvol.SysMgr + "STARTUP.COM;*")
	if err != nil || len(names) != 1 {
		t.Errorf("STARTUP.COM's versions: %v, %v", names, err)
	}
}

// TestSystemVolumeShared checks a second session: a container another
// session holds is mounted read-only, and said to be.
func TestSystemVolumeShared(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	_, _, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Writable {
		t.Fatalf("first session = %+v", r)
	}

	// The first session's console still has it mounted, and so its lock.
	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted || r.Writable || !r.Shared {
		t.Fatalf("second session = %+v; output:\n%s", r, buf)
	}

	if want := "%MOUNT-I-SYSREADONLY, system volume " + path +
		" is in use by another govax session; mounted read-only\n"; !strings.Contains(buf.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, buf)
	}

	if c.Mounts.Writable("DUA0") || c.Mounts.System() != "DUA0" {
		t.Errorf("writable %v, system %q", c.Mounts.Writable("DUA0"), c.Mounts.System())
	}

	// So is --system-initialize's, which can't make the volume again under
	// the first session.
	_, buf, r = mountTestSystemVolume(t, dclcmd.SystemVolume{File: path, Initialize: true})
	if !r.Shared || r.Created {
		t.Errorf("initialize while shared = %+v; output:\n%s", r, buf)
	}

	if _, err := os.Stat(path + ".old"); err == nil {
		t.Error("the container in use was renamed")
	}
}

// TestSystemVolumeLockReleased checks that a dismounted system volume's
// lock goes, so the next session can write it.
func TestSystemVolumeLockReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	c, _, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	if _, err := hostlock.TryLock(path); err == nil {
		t.Fatal("a mounted system volume isn't locked")
	}

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	lock, err := hostlock.TryLock(path)
	if err != nil {
		t.Fatalf("after DISMOUNT: %v", err)
	}

	_ = lock.Close()

	if c.Mounts.System() != "" {
		t.Errorf("system device %q after DISMOUNT", c.Mounts.System())
	}
}

// TestSystemVolumeInitialize checks --system-initialize: the old
// container kept as FILE.old, a new one made, and said so.
func TestSystemVolumeInitialize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	first, _, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	// Something of the user's, which the new volume won't have.
	if _, err := first.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:[SYSTEM]MINE.TXT"},
		rms.TextRecords, [][]byte{[]byte("mine")}); err != nil {
		t.Fatal(err)
	}

	if err := first.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path, Initialize: true})
	if !r.Mounted || !r.Created || !r.Writable {
		t.Fatalf("result = %+v; output:\n%s", r, buf)
	}

	if want := "%MOUNT-I-SYSINIT, system volume " + path + " initialized again; the old one is " +
		path + ".old\n"; !strings.Contains(buf.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, buf)
	}

	if names, _ := c.ContainerSession.Search("DUA0:[SYSTEM]MINE.TXT"); len(names) != 0 {
		t.Error("the new volume has the old one's file")
	}

	if _, err := c.ContainerSession.Search("DUA0:" + sysvol.SysMgr + "STARTUP.COM"); err != nil {
		t.Errorf("the new volume lacks STARTUP.COM: %v", err)
	}

	// The old one is a whole volume still, with the user's file.
	old, _ := newTestConsole(t)
	if err := old.cmd().Mount("DUA5", path+".old", false); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = old.Mounts.DismountAll() })

	if names, err := old.ContainerSession.Search("DUA5:[SYSTEM]MINE.TXT"); err != nil || len(names) != 1 {
		t.Errorf("the old volume's file: %v, %v", names, err)
	}
}

// TestSystemVolumeNotUsed checks the cases where there is no system
// volume, and govax goes on without one: none asked for, a type govax
// doesn't know, a device already mounted, a container that isn't a
// system volume.
func TestSystemVolumeNotUsed(t *testing.T) {
	dir := t.TempDir()

	_, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: "NONE"})
	if r.Mounted || buf.Len() != 0 {
		t.Errorf("none: %+v, output %q", r, buf)
	}

	_, buf, r = mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(dir, "a.dsk"), Type: "RZ99"})
	if r.Mounted || !strings.Contains(buf.String(), "%MOUNT-W-NOSYSVOL, system volume "+filepath.Join(dir, "a.dsk")+
		" not mounted: unknown device type RZ99") {
		t.Errorf("unknown type: %+v, output %q", r, buf)
	}

	work := filepath.Join(dir, "work.dsk")
	setup, _ := newTestConsole(t)

	if err := setup.cmd().InitializeContainer(work, 0, "WORK", 0, "RX50"); err != nil {
		t.Fatal(err)
	}

	_, buf, r = mountTestSystemVolume(t, dclcmd.SystemVolume{File: work})
	if r.Mounted || !strings.Contains(buf.String(), "volume label is WORK, not GOVAXSYS") {
		t.Errorf("a work volume: %+v, output %q", r, buf)
	}

	c, _ := newTestConsole(t)
	t.Cleanup(func() { _ = c.Mounts.DismountAll() })

	if err := c.cmd().Mount("DUA0", work, false); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	c.Out = buf
	c.Verbose = true

	if r := c.cmd().MountSystemVolume(dclcmd.SystemVolume{File: filepath.Join(dir, "b.dsk"), Type: testSystemType}); r.Mounted ||
		!strings.Contains(buf.String(), "DUA0: already mounted") {
		t.Errorf("DUA0 taken: %+v, output %q", r, buf)
	}
}

// TestSystemVolumeDismountRefused checks that DISMOUNT leaves the system
// volume mounted, and says why.
func TestSystemVolumeDismountRefused(t *testing.T) {
	c, buf, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "sys.dsk")})
	buf.Reset()

	if err := c.cmd().Dismount("DUA0:"); err == nil {
		t.Fatal("DISMOUNT of the system volume succeeded")
	}

	want := "%DISM-W-CANNOTDMT, DUA0: cannot be dismounted\n%DISM-W-SYSDISK, device is the system volume\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf, want)
	}

	if _, mounted := c.Mounts.Lookup("DUA0"); !mounted {
		t.Error("the system volume was dismounted")
	}
}

// TestResolveUnit checks the next-free-unit rule: a controller alone is
// its lowest unit with nothing mounted, and anything else is as given.
func TestResolveUnit(t *testing.T) {
	c, buf, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "sys.dsk")})

	if err := c.cmd().DefineLogicalName("LNM$PROCESS_TABLE", "DSK", []string{"DUA2:"}, lnm.Supervisor, 0, false); err != nil {
		t.Fatalf("DEFINE: %v (%s)", err, buf)
	}

	for in, want := range map[string]string{
		"DUA":   "DUA1",
		"dua:":  "DUA1",
		"_DUA:": "DUA1",
		"DUB":   "DUB0",
		"DUA0":  "DUA0",
		"DUA7:": "DUA7",
		"DSK":   "DSK",
		"_DSK":  "DSK0",
		"DISK":  "DISK",
	} {
		if got := c.cmd().ResolveUnit(in); got != want {
			t.Errorf("ResolveUnit(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDefaultVolumeBesideSystem checks that the default volume's default
// device is the next unit after the system volume's, and that naming
// the system volume's device for it is refused.
func TestDefaultVolumeBesideSystem(t *testing.T) {
	dir := t.TempDir()

	c, buf, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(dir, "sys.dsk")})
	buf.Reset()

	c.cmd().MountDefaultVolume(dclcmd.DefaultVolume{File: filepath.Join(dir, "work.dsk"), Type: "RX50"})

	if !strings.Contains(buf.String(), "%MOUNT-I-MOUNTED, WORK mounted on _DUA1:\n") {
		t.Errorf("output:\n%s", buf)
	}

	if got := c.ContainerSession.DefaultString(); got != "DUA1:[WORK]" {
		t.Errorf("default = %q", got)
	}

	other, buf, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(dir, "sys2.dsk")})
	buf.Reset()

	other.cmd().MountDefaultVolume(dclcmd.DefaultVolume{File: filepath.Join(dir, "work2.dsk"), Device: "DUA0:", Type: "RX50"})

	if want := "DUA0: is the system volume"; !strings.Contains(buf.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, buf)
	}
}

// TestMountCommandNextUnit checks the MOUNT command's next-free-unit
// rule: MOUNT DUA "file" mounts on the lowest DUA unit with nothing on
// it, after the system volume's, and says which.
func TestMountCommandNextUnit(t *testing.T) {
	dir := t.TempDir()

	c, buf, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(dir, "sys.dsk")})
	d := NewDispatcher(c, loadEvaxGrammar(t))

	for i, label := range []string{"FIRST", "SECOND"} {
		path := filepath.Join(dir, label+".dsk")
		if err := c.cmd().InitializeContainer(path, 0, label, 0, "RX50"); err != nil {
			t.Fatal(err)
		}

		buf.Reset()

		if err := d.Dispatch(`MOUNT DUA "` + path + `"`); err != nil {
			t.Fatalf("MOUNT DUA: %v\n%s", err, buf)
		}

		unit := fmt.Sprintf("DUA%d", i+1)
		if want := "%MOUNT-I-MOUNTED, " + label + " mounted on _" + unit + ":\n"; buf.String() != want {
			t.Errorf("output = %q, want %q", buf, want)
		}

		if got, _ := c.Mounts.VolumeLabel(unit); strings.TrimSpace(got) != label {
			t.Errorf("%s holds %q, not %s", unit, got, label)
		}
	}
}
