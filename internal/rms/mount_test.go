package rms

import (
	"path/filepath"
	"testing"

	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// newTestVolumeFile creates a small, freshly initialized ODS-2 container
// at a temp path and returns that path, ready for MountTable.Mount to
// open by name. This mirrors the sibling ods2 module's own test
// convention (see volume.TestInitializeProducesMountableVolume) of
// building throwaway fixtures on the fly with ods2's own
// diskimage.Create/volume.Initialize calls, rather than depending on a
// binary blob under testdata/ — see docs/PHASE-22.md's "Test fixtures"
// design decision for why: it keeps `go test ./...` fully portable, with
// nothing to check into git.
func newTestVolumeFile(t *testing.T, label string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.dsk")

	// 400 blocks (200KB) is comfortably enough for an empty, freshly
	// initialized volume — these tests never write a real file onto it,
	// they only need something MountTable.Mount can successfully open.
	c, err := diskimage.Create(path, 400)
	if err != nil {
		t.Fatalf("diskimage.Create: %v", err)
	}

	if err := volume.Initialize(c, volume.InitializeOptions{Label: label}); err != nil {
		_ = c.Close()

		t.Fatalf("volume.Initialize: %v", err)
	}

	// Mount (below) always opens the container fresh by path, so the
	// handle used to create/initialize it must be closed first — two
	// separate open file handles onto the same path at once isn't the
	// scenario these tests want to exercise.
	if err := c.Close(); err != nil {
		t.Fatalf("closing freshly initialized container: %v", err)
	}

	return path
}

// TestMountTable_mountAndLookup covers the ordinary happy path: mounting
// a container succeeds, and Lookup finds it back — including normalizing
// away case and the trailing ':' VMS device names conventionally have
// (see normalizeDeviceName), so "DUA0:" (given to Mount) and "dua0"
// (given to Lookup) refer to the same entry.
func TestMountTable_mountAndLookup(t *testing.T) {
	path := newTestVolumeFile(t, "TESTVOL")

	mt := NewMountTable()
	if err := mt.Mount("DUA0:", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, ok := mt.Lookup("dua0")
	if !ok {
		t.Fatal("Lookup(dua0) = not found, want found")
	}

	if got, want := vol.Devices[0].Home.VolumeName, "TESTVOL"; got != want {
		t.Errorf("mounted volume's label = %q, want %q", got, want)
	}

	if !mt.Writable("DUA0") {
		t.Error("Writable(DUA0) = false, want true (mounted with writable=true)")
	}
}

// TestMountTable_lookupMiss covers an empty table: nothing should be
// found, and Writable — which callers may check without first confirming
// a mount exists at all (see its own doc comment) — should report false
// rather than panicking on a map key that was never set.
func TestMountTable_lookupMiss(t *testing.T) {
	mt := NewMountTable()

	if _, ok := mt.Lookup("DUA0"); ok {
		t.Error("Lookup on an empty table = found, want not found")
	}

	if mt.Writable("DUA0") {
		t.Error("Writable on an unmounted device = true, want false")
	}
}

// TestMountTable_readOnlyMount confirms Writable correctly reflects a
// mount's own writable=false, not just "is anything mounted here at
// all".
func TestMountTable_readOnlyMount(t *testing.T) {
	path := newTestVolumeFile(t, "TESTVOL")

	mt := NewMountTable()
	if err := mt.Mount("DUA0", path, false); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	if _, ok := mt.Lookup("DUA0"); !ok {
		t.Fatal("Lookup after a read-only Mount = not found, want found")
	}

	if mt.Writable("DUA0") {
		t.Error("Writable = true for a mount opened with writable=false")
	}
}

// TestMountTable_mountAlreadyMounted matches real VMS's own MOUNT: mounting
// a device name that already has something mounted on it is an error, not
// an implicit swap — the caller must Dismount first.
func TestMountTable_mountAlreadyMounted(t *testing.T) {
	path := newTestVolumeFile(t, "TESTVOL")

	mt := NewMountTable()
	if err := mt.Mount("DUA0", path, false); err != nil {
		t.Fatalf("first Mount: %v", err)
	}

	if err := mt.Mount("DUA0:", path, false); err == nil {
		t.Error("second Mount of an already-mounted device = nil error, want an error")
	}
}

// TestMountTable_mountMissingFile confirms Mount surfaces diskimage.Open's
// own error rather than silently succeeding, and leaves no partial entry
// behind for Lookup to find.
func TestMountTable_mountMissingFile(t *testing.T) {
	mt := NewMountTable()

	missing := filepath.Join(t.TempDir(), "does-not-exist.dsk")
	if err := mt.Mount("DUA0", missing, false); err == nil {
		t.Error("Mount of a nonexistent container path = nil error, want an error")
	}

	if _, ok := mt.Lookup("DUA0"); ok {
		t.Error("a failed Mount left a Lookup-able entry behind")
	}
}

// TestMountTable_dismount confirms Dismount both succeeds and actually
// forgets the entry (Lookup afterward finds nothing), and that the freed
// device name can be Mounted again — real VMS's own DISMOUNT-then-MOUNT
// cycle.
func TestMountTable_dismount(t *testing.T) {
	path := newTestVolumeFile(t, "TESTVOL")

	mt := NewMountTable()
	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	if err := mt.Dismount("dua0:"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if _, ok := mt.Lookup("DUA0"); ok {
		t.Error("Lookup after Dismount = found, want not found")
	}

	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("re-Mount after Dismount: %v", err)
	}
}

// TestMountTable_dismountNotMounted confirms Dismount on a device with
// nothing mounted is a reported error, not a silent no-op.
func TestMountTable_dismountNotMounted(t *testing.T) {
	mt := NewMountTable()

	if err := mt.Dismount("DUA0"); err == nil {
		t.Error("Dismount of an unmounted device = nil error, want an error")
	}
}

// TestMountTable_volumeLabel confirms VolumeLabel returns the mounted
// volume's real on-disk label (the same one TestMountTable_mountAndLookup
// already reads directly off vol.Devices[0].Home.VolumeName), reports
// "not mounted" for a device with nothing attached, and stops reporting a
// label at all once Dismount has run — internal/console/device.go's
// ShowDevices (SHOW DEVICE/FULL) relies on that last part to stop showing
// a mounted-volume line for a device that was just dismounted.
func TestMountTable_volumeLabel(t *testing.T) {
	mt := NewMountTable()

	if _, ok := mt.VolumeLabel("DUA0"); ok {
		t.Error("VolumeLabel on an empty table = found, want not found")
	}

	path := newTestVolumeFile(t, "TESTVOL")
	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	label, ok := mt.VolumeLabel("dua0:")
	if !ok {
		t.Fatal("VolumeLabel after Mount = not found, want found")
	}

	if label != "TESTVOL" {
		t.Errorf("VolumeLabel = %q, want %q", label, "TESTVOL")
	}

	if err := mt.Dismount("DUA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if _, ok := mt.VolumeLabel("DUA0"); ok {
		t.Error("VolumeLabel after Dismount = found, want not found")
	}
}

// TestMountTable_volumeStats confirms VolumeStats reports "not mounted"
// for a device with nothing attached, and real, live volume.Stats data
// (nonzero MaxFiles/TotalBlocks, zero FileCount on a freshly initialized
// volume) once one is mounted — internal/console/device.go's ShowDevices
// (SHOW DEVICE/FULL) relies on this for its "Free blocks"/"Number of
// files"/"Maximum files allowed" fields.
func TestMountTable_volumeStats(t *testing.T) {
	mt := NewMountTable()

	if _, mounted, err := mt.VolumeStats("DUA0"); mounted || err != nil {
		t.Errorf("VolumeStats on an empty table = mounted=%v, err=%v, want mounted=false, err=nil", mounted, err)
	}

	path := newTestVolumeFile(t, "TESTVOL")
	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	stats, mounted, err := mt.VolumeStats("dua0:")
	if !mounted || err != nil {
		t.Fatalf("VolumeStats after Mount = mounted=%v, err=%v, want mounted=true, err=nil", mounted, err)
	}

	if stats.FileCount != 0 {
		t.Errorf("VolumeStats.FileCount on a freshly initialized volume = %d, want 0", stats.FileCount)
	}
	
	if stats.MaxFiles == 0 || stats.TotalBlocks == 0 {
		t.Errorf("VolumeStats = %+v, want nonzero MaxFiles/TotalBlocks", stats)
	}
}
