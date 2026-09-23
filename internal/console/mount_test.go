package console

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// newTestContainer builds a small, freshly initialized ODS-2 container
// file at a temp path and returns that path — mirroring internal/rms's own
// mount_test.go helper (newTestVolumeFile) and docs/PHASE-22.md's "Test
// fixtures" design decision: every automated test in this project builds
// its own throwaway container with the sibling ods2 module's own
// diskimage.Create/volume.Initialize calls rather than depending on a
// binary blob checked into testdata/, so go test ./... stays fully
// portable (works on a fresh clone, in CI, anywhere) with nothing to
// commit.
func newTestContainer(t *testing.T, label string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.dsk")

	c, err := diskimage.Create(path, 400)
	if err != nil {
		t.Fatalf("diskimage.Create: %v", err)
	}

	if err := volume.Initialize(c, volume.InitializeOptions{Label: label}); err != nil {
		_ = c.Close()
		t.Fatalf("volume.Initialize: %v", err)
	}

	// Mount always reopens the container fresh by path (internal/rms's own
	// MountTable.Mount, see its doc comment), so this handle has to be
	// closed first -- two separate open file handles onto the same path at
	// once isn't a scenario these tests want to exercise.
	if err := c.Close(); err != nil {
		t.Fatalf("closing freshly initialized container: %v", err)
	}

	return path
}

// TestConsoleMount_autoCreatesDevice confirms Mount both attaches the
// container (visible afterward via c.Mounts.Lookup) and, since "DUA0"
// was never DEFINE/DEVICE'd, auto-creates a disk-class device record for
// it -- docs/PHASE-22.md's "Device model" design decision, matching how
// an operator would expect a bare MOUNT command to just work without a
// separate DEFINE/DEVICE ceremony first.
func TestConsoleMount_autoCreatesDevice(t *testing.T) {
	c, _ := newTestConsole(t)
	path := newTestContainer(t, "TESTVOL")

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	if _, ok := c.Mounts.Lookup("DUA0"); !ok {
		t.Error("Mounts.Lookup(DUA0) after Mount = not found, want found")
	}

	dev, ok := c.Devices.Find("DUA0")
	if !ok {
		t.Fatal("Devices.Find(DUA0) after Mount = not found, want an auto-created device")
	}

	if dev.DevClass != iodev.DeviceClassDisk {
		t.Errorf("auto-created device DevClass = %d, want DeviceClassDisk (%d)", dev.DevClass, iodev.DeviceClassDisk)
	}
}

// TestConsoleMount_doesNotRedefineExistingDevice confirms Mount leaves an
// already-DEFINE/DEVICE'd device's own fields alone -- auto-creation only
// ever fills in a device record that doesn't exist yet (Console.Mount's
// own doc comment), it never overwrites one an operator already set up
// with its own DEVTYPE/qualifiers.
func TestConsoleMount_doesNotRedefineExistingDevice(t *testing.T) {
	c, _ := newTestConsole(t)
	path := newTestContainer(t, "TESTVOL")

	c.DefineDevice("DUA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk, DevType: 42})

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	dev, ok := c.Devices.Find("DUA0")
	if !ok {
		t.Fatal("Devices.Find(DUA0) after Mount = not found")
	}

	if dev.DevType != 42 {
		t.Errorf("DevType = %d, want 42 (Mount must not redefine an existing device)", dev.DevType)
	}
}

// TestConsoleMount_alreadyMounted confirms a second Mount against a
// device that already has a container attached fails with the real
// SS_DEVMOUNT status (matching real VMS's own refusal to implicitly swap
// an already-mounted device's volume), and that the original mount is
// left completely undisturbed.
func TestConsoleMount_alreadyMounted(t *testing.T) {
	c, _ := newTestConsole(t)
	path := newTestContainer(t, "TESTVOL")

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("first Mount: %v", err)
	}

	err := c.Mount("DUA0", path, true)
	if err == nil {
		t.Fatal("second Mount of an already-mounted device = nil error, want SS_DEVMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVMOUNT)) {
		t.Errorf("Mount error = %v, want SS_DEVMOUNT", err)
	}

	if label, ok := c.Mounts.VolumeLabel("DUA0"); !ok || label != "TESTVOL" {
		t.Errorf("VolumeLabel after a rejected re-Mount = %q, %v, want TESTVOL, true (unchanged)", label, ok)
	}
}

// TestConsoleMount_badContainer confirms a Mount naming a container path
// that doesn't exist fails with the real SS_NOMOUNT status (the generic
// "the MOUNT operation itself could not be completed" code), and leaves
// no device record behind -- Console.Mount only auto-creates a device
// after the underlying mount has actually succeeded, so a failed MOUNT
// never leaves a phantom disk device pointing at nothing.
func TestConsoleMount_badContainer(t *testing.T) {
	c, _ := newTestConsole(t)

	missing := filepath.Join(t.TempDir(), "does-not-exist.dsk")

	err := c.Mount("DUA0", missing, true)
	if err == nil {
		t.Fatal("Mount of a nonexistent container = nil error, want SS_NOMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOMOUNT)) {
		t.Errorf("Mount error = %v, want SS_NOMOUNT", err)
	}

	if _, ok := c.Devices.Find("DUA0"); ok {
		t.Error("a failed Mount left a device record behind")
	}
}

// TestConsoleDismount confirms Dismount detaches a mounted container
// (Mounts.Lookup no longer finds it afterward) while leaving the
// auto-created device record itself in place -- real VMS's own DISMOUNT
// makes a device unmounted, not undefined.
func TestConsoleDismount(t *testing.T) {
	c, _ := newTestConsole(t)
	path := newTestContainer(t, "TESTVOL")

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	if err := c.Dismount("DUA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if _, ok := c.Mounts.Lookup("DUA0"); ok {
		t.Error("Mounts.Lookup(DUA0) after Dismount = found, want not found")
	}

	if _, ok := c.Devices.Find("DUA0"); !ok {
		t.Error("Devices.Find(DUA0) after Dismount = not found, want the device record to survive")
	}
}

// TestConsoleDismount_notMounted confirms Dismount against a device with
// nothing mounted fails with the real SS_DEVNOTMOUNT status rather than a
// silent no-op or a generic error.
func TestConsoleDismount_notMounted(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Dismount("DUA0")
	if err == nil {
		t.Fatal("Dismount of an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Dismount error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestConsoleMountThenRemount confirms Dismount actually frees the device
// name for a later Mount, the same MOUNT/DISMOUNT/MOUNT cycle a real VMS
// operator swapping media would perform.
func TestConsoleMountThenRemount(t *testing.T) {
	c, _ := newTestConsole(t)
	path := newTestContainer(t, "TESTVOL")

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("first Mount: %v", err)
	}

	if err := c.Dismount("DUA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if err := c.Mount("DUA0", path, false); err != nil {
		t.Fatalf("re-Mount: %v", err)
	}

	if c.Mounts.Writable("DUA0") {
		t.Error("re-Mount with write=false reports Writable = true")
	}
}

// TestShowDevices_mountedVolumeLine confirms SHOW DEVICE/FULL's new
// mounted-volume line (device.go's showMountedVolume) reflects live
// c.Mounts state: absent no mount at all it reports "<not mounted>",
// after a writable Mount it reports the real on-disk label and
// "READ/WRITE", and after a read-only Mount it reports "READ ONLY"
// instead -- all three read straight from internal/rms.MountTable, not
// from any field on the internal/io.Device record itself.
func TestShowDevices_mountedVolumeLine(t *testing.T) {
	c, buf := newTestConsole(t)
	path := newTestContainer(t, "TESTVOL")

	c.DefineDevice("DUA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk})

	if err := c.ShowDevices("DUA0", true); err != nil {
		t.Fatalf("ShowDevices (unmounted): %v", err)
	}

	if !strings.Contains(buf.String(), "MOUNTED=<not mounted>") {
		t.Errorf("ShowDevices (unmounted) output = %q, want a <not mounted> MOUNTED line", buf.String())
	}

	buf.Reset()

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	if err := c.ShowDevices("DUA0", true); err != nil {
		t.Fatalf("ShowDevices (writable mount): %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MOUNTED=TESTVOL (READ/WRITE)") {
		t.Errorf("ShowDevices (writable mount) output = %q, want a TESTVOL/READ/WRITE MOUNTED line", out)
	}

	buf.Reset()

	if err := c.Dismount("DUA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if err := c.Mount("DUA0", path, false); err != nil {
		t.Fatalf("re-Mount read-only: %v", err)
	}

	if err := c.ShowDevices("DUA0", true); err != nil {
		t.Fatalf("ShowDevices (read-only mount): %v", err)
	}

	out = buf.String()
	if !strings.Contains(out, "MOUNTED=TESTVOL (READ ONLY)") {
		t.Errorf("ShowDevices (read-only mount) output = %q, want a TESTVOL/READ ONLY MOUNTED line", out)
	}
}

// TestShowDevices_mountedVolumeLineOnlyForDisks confirms showMountedVolume
// is never reached for a non-disk device -- SHOW DEVICE/FULL's MOUNTED
// line only makes sense for the device class MOUNT actually targets.
func TestShowDevices_mountedVolumeLineOnlyForDisks(t *testing.T) {
	c, buf := newTestConsole(t)

	c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	if err := c.ShowDevices("TTA0", true); err != nil {
		t.Fatalf("ShowDevices: %v", err)
	}

	if strings.Contains(buf.String(), "MOUNTED=") {
		t.Errorf("ShowDevices for a terminal device printed a MOUNTED line: %q", buf.String())
	}
}

// TestDispatch_mountAndDismountViaDCL exercises subtask 13's own new work
// directly: parsing and dispatching real "MOUNT ..."/"DISMOUNT ..." command
// lines through the DCL grammar (internal/bootdata/files/evax.dcl's "verb
// mount"/"verb dismount", subtask 2) into the g.Bind("MOUNT", ...)/
// g.Bind("DISMOUNT", ...) closures this subtask added (dispatch.go), which
// in turn call the already-tested Console.Mount/Dismount (subtask 12).
// Everything below subtask 13's own two closures was already covered by
// TestConsoleMount_autoCreatesDevice/TestConsoleDismount and friends, so
// this test's job is narrower: confirm a command line actually reaches
// them at all, with its DEVICE/FILE/WRITE fields threaded through
// correctly.
//
// The container path is double-quoted in the command line: DCL upcases an
// unquoted token (TestParse_mount's own doc comment, internal/console/dcl/
// parse_test.go), and t.TempDir() paths are mixed-case, so an unquoted
// path here would fail to open under its now-upcased spelling on any
// case-sensitive filesystem.
func TestDispatch_mountAndDismountViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)
	path := newTestContainer(t, "TESTVOL")

	if err := d.Dispatch(fmt.Sprintf(`MOUNT DUA0 "%s"`, path)); err != nil {
		t.Fatalf("Dispatch MOUNT: %v", err)
	}

	if label, ok := c.Mounts.VolumeLabel("DUA0"); !ok || label != "TESTVOL" {
		t.Errorf("VolumeLabel(DUA0) after MOUNT = %q, %v, want TESTVOL, true", label, ok)
	}

	if !c.Mounts.Writable("DUA0") {
		t.Error("Writable(DUA0) after a plain MOUNT (no /NOWRITE) = false, want true")
	}

	if _, ok := c.Devices.Find("DUA0"); !ok {
		t.Error("Devices.Find(DUA0) after MOUNT = not found, want an auto-created device")
	}

	if err := d.Dispatch("DISMOUNT DUA0"); err != nil {
		t.Fatalf("Dispatch DISMOUNT: %v", err)
	}

	if _, ok := c.Mounts.Lookup("DUA0"); ok {
		t.Error("Mounts.Lookup(DUA0) after DISMOUNT = found, want not found")
	}
}

// TestDispatch_mountNowriteViaDCL confirms MOUNT/NOWRITE's automatic "NO"-
// prefix negation (dispatch.go's MOUNT closure reads r.Negated("WRITE"),
// not r.Present) actually reaches Console.Mount as writable=false, the
// dispatch-level counterpart to TestParse_mountNowrite (which only checks
// grammar-level parsing, never calls Console.Mount at all).
func TestDispatch_mountNowriteViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)
	path := newTestContainer(t, "TESTVOL")

	if err := d.Dispatch(fmt.Sprintf(`MOUNT/NOWRITE DUA0 "%s"`, path)); err != nil {
		t.Fatalf("Dispatch MOUNT/NOWRITE: %v", err)
	}

	if c.Mounts.Writable("DUA0") {
		t.Error("Writable(DUA0) after MOUNT/NOWRITE = true, want false")
	}
}

// TestDispatch_dismountNotMountedViaDCL confirms a DISMOUNT of a device
// with nothing mounted, dispatched through the real DCL grammar, surfaces
// Console.Dismount's SS_DEVNOTMOUNT error rather than being swallowed
// somewhere in the grammar-dispatch plumbing.
func TestDispatch_dismountNotMountedViaDCL(t *testing.T) {
	d, _ := newTestDispatcher(t)

	err := d.Dispatch("DISMOUNT DUA0")
	if err == nil {
		t.Fatal("Dispatch DISMOUNT of an unmounted device = nil error, want SS_DEVNOTMOUNT")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Dispatch DISMOUNT error = %v, want SS_DEVNOTMOUNT", err)
	}
}
