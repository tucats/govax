package rms

import (
	"path/filepath"
	"testing"
)

// TestInitializeContainer_buildsAMountableVolume confirms the happy path
// end to end: InitializeContainer builds a container file that a following
// MountTable.Mount can actually open, with the label it was given intact —
// the same "format, then mount separately" two-step real VMS's own
// INITIALIZE/MOUNT pair expects (InitializeContainer's own doc comment
// explains why the two are kept apart).
func TestInitializeContainer_buildsAMountableVolume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := InitializeContainer(path, 400, "TESTVOL", 0); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	mt := NewMountTable()
	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount of a freshly initialized container: %v", err)
	}

	if label, ok := mt.VolumeLabel("DUA0"); !ok || label != "TESTVOL" {
		t.Errorf("VolumeLabel(DUA0) = %q, %v, want TESTVOL, true", label, ok)
	}
}

// TestInitializeContainer_defaultLabel confirms an empty label defaults to
// "NONAME", matching ods2's own cmdInitialize (initialize.go's doc comment)
// rather than leaving the volume's on-disk VOLNAM field blank.
func TestInitializeContainer_defaultLabel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := InitializeContainer(path, 400, "", 0); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	mt := NewMountTable()
	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	if label, ok := mt.VolumeLabel("DUA0"); !ok || label != "NONAME" {
		t.Errorf("VolumeLabel(DUA0) with no label given = %q, %v, want NONAME, true", label, ok)
	}
}

// TestInitializeContainer_clusterSize confirms a non-zero clusterSize
// argument actually reaches volume.Initialize's own InitializeOptions.
// ClusterSize field, by reading it back off the mounted volume's home
// block afterward -- InitializeContainer itself has no clusterSize getter
// of its own, so this is the only way to confirm the value was threaded
// through rather than silently dropped.
func TestInitializeContainer_clusterSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := InitializeContainer(path, 400, "TESTVOL", 4); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	mt := NewMountTable()
	if err := mt.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, ok := mt.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after Mount = not found")
	}

	if got, want := vol.Devices[0].Home.ClusterSize, uint16(4); got != want {
		t.Errorf("ClusterSize = %d, want %d", got, want)
	}
}

// TestInitializeContainer_notMounted confirms InitializeContainer never
// mounts the volume it just built -- matching real VMS's own INITIALIZE,
// which formats a device without mounting it (InitializeContainer's own
// doc comment). A fresh MountTable that InitializeContainer was never
// handed can't possibly have anything mounted on it, so this is really a
// documentation-and-API-shape check (InitializeContainer takes no
// MountTable/device argument at all) more than a behavioral one -- but it
// guards against a future change accidentally threading mount state in.
func TestInitializeContainer_notMounted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := InitializeContainer(path, 400, "TESTVOL", 0); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	mt := NewMountTable()
	if _, ok := mt.Lookup("DUA0"); ok {
		t.Error("Lookup(DUA0) on a fresh MountTable after InitializeContainer = found, want not found")
	}
}

// TestInitializeContainer_zeroBlocks confirms a blocks argument of 0 is
// rejected -- diskimage.Create's own documented minimum -- rather than
// silently building an unusable, empty-length container file.
func TestInitializeContainer_zeroBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := InitializeContainer(path, 0, "TESTVOL", 0); err == nil {
		t.Error("InitializeContainer with blocks=0 = nil error, want an error")
	}
}

// TestInitializeContainer_tooSmallForVolumeLayout confirms a container
// that diskimage.Create is happy to build (a nonzero block count) but that
// volume.Initialize can't fit even the minimal reserved-file layout into
// still fails clearly, rather than leaving behind a truncated,
// inconsistent volume -- matching the sibling ods2 module's own
// TestInitializeRejectsUndersizedVolume (volume/initialize_test.go), which
// establishes 5 blocks as reliably too small.
func TestInitializeContainer_tooSmallForVolumeLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tiny.dsk")

	if err := InitializeContainer(path, 5, "TESTVOL", 0); err == nil {
		t.Error("InitializeContainer with a 5-block volume = nil error, want an error")
	}
}

// TestInitializeContainer_badPath confirms a path whose parent directory
// doesn't exist fails clearly (the diskimage.Create/os.OpenFile error
// surfaces as this function's own error), the InitializeContainer
// counterpart to TestConsoleMount_badContainer's "bad path" coverage for
// Mount.
func TestInitializeContainer_badPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-directory", "new.dsk")

	if err := InitializeContainer(path, 400, "TESTVOL", 0); err == nil {
		t.Error("InitializeContainer with a nonexistent parent directory = nil error, want an error")
	}
}
