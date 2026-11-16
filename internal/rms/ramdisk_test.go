package rms

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// ramLimit is a limit on the RAM disks' total that the tests below don't
// reach unless they mean to.
const ramLimit = 512000

// defineRAM makes device a RAM disk of blocks blocks, or fails the test.
func defineRAM(t *testing.T, mounts *MountTable, device string, blocks uint32) {
	t.Helper()

	if err := mounts.DefineMemory(device, blocks, ramLimit); err != nil {
		t.Fatalf("DefineMemory(%s, %d): %v", device, blocks, err)
	}
}

// mountRAM mounts device's RAM disk for writing, or fails the test.
func mountRAM(t *testing.T, mounts *MountTable, device string, opts MemoryVolume) {
	t.Helper()

	if err := mounts.MountMemory(device, true, opts); err != nil {
		t.Fatalf("MountMemory(%s): %v", device, err)
	}
}

// writeRAMFile writes a text file of lines through RMS at spec.
func writeRAMFile(t *testing.T, mounts *MountTable, spec string, lines []string) {
	t.Helper()

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := NewSession(mounts).CreateRecordFile(FileLocation{Name: spec}, TextRecords, records); err != nil {
		t.Fatalf("CreateRecordFile(%s): %v", spec, err)
	}
}

// readRAMFile reads a text file's lines through RMS from spec.
func readRAMFile(t *testing.T, mounts *MountTable, spec string) []string {
	t.Helper()

	records, _, err := NewSession(mounts).ReadRecordFile(FileLocation{Name: spec}, TextRecords)
	if err != nil {
		t.Fatalf("ReadRecordFile(%s): %v", spec, err)
	}

	lines := make([]string, len(records))
	for i, record := range records {
		lines[i] = string(record)
	}

	return lines
}

// TestRAMDiskOutlivesDismount is a RAM disk's life through RMS: defined,
// given a volume by its first mount with the label, cluster size, owner,
// and protection asked for, written, dismounted, and mounted again (with
// options the second mount ignores), the file still there and the volume
// clean.
func TestRAMDiskOutlivesDismount(t *testing.T) {
	mounts := NewMountTable()
	defer func() { _ = mounts.ReleaseMemory() }()

	defineRAM(t, mounts, "_mda0:", 20000)

	if !mounts.IsMemory("MDA0") || mounts.IsMemory("MDA1") {
		t.Fatalf("IsMemory(MDA0, MDA1) = %v, %v; want true, false", mounts.IsMemory("MDA0"), mounts.IsMemory("MDA1"))
	}

	if size, ok := mounts.MemorySize("MDA0:"); size != 20000 || !ok {
		t.Errorf("MemorySize = %d, %v; want 20000, true", size, ok)
	}

	if _, mounted := mounts.Lookup("MDA0"); mounted {
		t.Fatal("a RAM disk is mounted before MountMemory")
	}

	owner := security.UIC(0o200<<16 | 0o5)
	mountRAM(t, mounts, "MDA0", MemoryVolume{Label: "SCRATCH", ClusterSize: 4, Owner: owner, Protection: 0x1234})

	vol, _ := mounts.Lookup("MDA0")
	home := vol.Devices[0].Home

	if home.VolumeName != "SCRATCH" {
		t.Errorf("label = %q, want SCRATCH", home.VolumeName)
	}

	if home.ClusterSize != 4 {
		t.Errorf("cluster = %d, want 4", home.ClusterSize)
	}

	if home.VolumeOwner.Group != 0o200 || home.VolumeOwner.Member != 0o5 {
		t.Errorf("owner = %+v, want [200,5]", home.VolumeOwner)
	}

	if home.Protection != 0x1234 {
		t.Errorf("protection = %#x, want 0x1234", home.Protection)
	}

	if _, err := NewSession(mounts).CreateDirectory("MDA0:[WORK]", CreateDirectoryOptions{}); err != nil {
		t.Fatalf("CreateDirectory: %v", err)
	}

	want := []string{"first line", "second line"}
	writeRAMFile(t, mounts, "MDA0:[WORK]NOTES.TXT", want)

	if err := mounts.Dismount("MDA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if mounts.MemoryInUse("MDA0") == 0 {
		t.Fatal("the RAM disk holds nothing after its dismount")
	}

	mountRAM(t, mounts, "MDA0", MemoryVolume{Label: "OTHER", ClusterSize: 1})

	if label, _ := mounts.VolumeLabel("MDA0"); label != "SCRATCH" {
		t.Errorf("label after remounting = %q, want SCRATCH", label)
	}

	if got := readRAMFile(t, mounts, "MDA0:[WORK]NOTES.TXT"); !reflect.DeepEqual(got, want) {
		t.Errorf("after remounting the file holds %q, want %q", got, want)
	}

	problems, err := mounts.VerifyVolume("MDA0")
	if err != nil || len(problems) > 0 {
		t.Errorf("VerifyVolume = %q, %v", problems, err)
	}

	// The image is sparse: a 20,000-block disk holding a directory and a
	// small file uses far less than its 10 MB.
	if inUse := mounts.MemoryInUse("MDA0"); inUse <= 0 || inUse >= 20000*512/4 {
		t.Errorf("MemoryInUse = %d bytes, want some but under a quarter of the disk", inUse)
	}
}

// TestRAMDiskDefineRules covers DefineMemory's refusals and the total:
// too small, mounted, over the limit (counting every disk's defined size,
// and a redefined disk at its new size only), and a redefinition
// discarding the volume.
func TestRAMDiskDefineRules(t *testing.T) {
	mounts := NewMountTable()
	defer func() { _ = mounts.ReleaseMemory() }()

	if err := mounts.DefineMemory("MDA0", MinimumMemoryBlocks-1, ramLimit); !errors.Is(err, ErrMemoryTooSmall) {
		t.Errorf("DefineMemory(99 blocks) = %v, want ErrMemoryTooSmall", err)
	}

	if mounts.IsMemory("MDA0") {
		t.Fatal("a refused definition made a RAM disk")
	}

	const limit = 50000

	if err := mounts.DefineMemory("MDA0", 30000, limit); err != nil {
		t.Fatalf("DefineMemory(MDA0, 30000): %v", err)
	}

	if err := mounts.DefineMemory("MDA1", 20000, limit); err != nil {
		t.Fatalf("DefineMemory(MDA1, 20000), reaching the limit: %v", err)
	}

	var over *MemoryLimitError

	err := mounts.DefineMemory("MDA2", 100, limit)
	if !errors.As(err, &over) || over.Total != 50100 || over.Limit != limit {
		t.Fatalf("DefineMemory past the limit = %v, want a MemoryLimitError of 50100 over 50000", err)
	}

	if mounts.IsMemory("MDA2") {
		t.Error("a definition over the limit made a RAM disk")
	}

	// MDA1 again, smaller: it counts at its new size, not both.
	if err := mounts.DefineMemory("MDA1", 15000, limit); err != nil {
		t.Fatalf("redefining MDA1 at 15000: %v", err)
	}

	if defined, _ := mounts.MemoryTotal(); defined != 45000 {
		t.Errorf("MemoryTotal's defined = %d, want 45000", defined)
	}

	// Mounted, MDA0 can't be defined again.
	mountRAM(t, mounts, "MDA0", MemoryVolume{Label: "FIRST"})
	writeRAMFile(t, mounts, "MDA0:[000000]KEEP.TXT", []string{"kept"})

	if err := mounts.DefineMemory("MDA0", 1000, limit); !errors.Is(err, ErrMounted) {
		t.Errorf("DefineMemory of a mounted disk = %v, want ErrMounted", err)
	}

	if err := mounts.Dismount("MDA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	// Dismounted, it can, and its volume goes: the next mount makes a
	// new one, with the new label.
	if err := mounts.DefineMemory("MDA0", 1000, limit); err != nil {
		t.Fatalf("redefining MDA0: %v", err)
	}

	if size, _ := mounts.MemorySize("MDA0"); size != 1000 {
		t.Errorf("MemorySize after redefining = %d, want 1000", size)
	}

	if inUse := mounts.MemoryInUse("MDA0"); inUse != 0 {
		t.Errorf("MemoryInUse after redefining = %d, want 0", inUse)
	}

	mountRAM(t, mounts, "MDA0", MemoryVolume{Label: "SECOND"})

	if label, _ := mounts.VolumeLabel("MDA0"); label != "SECOND" {
		t.Errorf("label after redefining = %q, want SECOND", label)
	}

	if _, found, _ := NewSession(mounts).ReadRecordFile(FileLocation{Name: "MDA0:[000000]KEEP.TXT"}, TextRecords); found.Name != "" {
		t.Errorf("a file from before the redefinition is still there: %s", found.Name)
	}
}

// TestRAMDiskMountRules covers MountMemory's refusals, a read-only later
// mount, and a mount of a device that isn't a RAM disk.
func TestRAMDiskMountRules(t *testing.T) {
	mounts := NewMountTable()
	defer func() { _ = mounts.ReleaseMemory() }()

	if err := mounts.MountMemory("MDA0", true, MemoryVolume{}); !errors.Is(err, ErrNotMemory) {
		t.Errorf("MountMemory of an undefined device = %v, want ErrNotMemory", err)
	}

	defineRAM(t, mounts, "MDA0", 2000)

	if err := mounts.MountMemory("MDA0", false, MemoryVolume{}); !errors.Is(err, ErrMemoryNoWrite) {
		t.Errorf("a read-only first mount = %v, want ErrMemoryNoWrite", err)
	}

	mountRAM(t, mounts, "MDA0", MemoryVolume{})

	if label, _ := mounts.VolumeLabel("MDA0"); label != "NONAME" {
		t.Errorf("a first mount with no label gave %q, want INITIALIZE's NONAME", label)
	}

	if err := mounts.MountMemory("MDA0", true, MemoryVolume{}); err == nil {
		t.Error("MountMemory of a mounted RAM disk succeeded")
	}

	writeRAMFile(t, mounts, "MDA0:[000000]A.TXT", []string{"a"})

	if err := mounts.Dismount("MDA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	// A later mount may be read-only: the file reads, and RMS sees the
	// volume as not writable.
	if err := mounts.MountMemory("MDA0", false, MemoryVolume{}); err != nil {
		t.Fatalf("a read-only later mount: %v", err)
	}

	if mounts.Writable("MDA0") {
		t.Error("a read-only mount is writable")
	}

	if got := readRAMFile(t, mounts, "MDA0:[000000]A.TXT"); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("read-only mount reads %q", got)
	}
}

// TestRAMDiskSessionEnd covers the session's end: DismountAll dismounts a
// RAM disk and keeps its image, and ReleaseMemory frees every image,
// dismounting one still mounted.
func TestRAMDiskSessionEnd(t *testing.T) {
	mounts := NewMountTable()

	defineRAM(t, mounts, "MDA0", 2000)
	defineRAM(t, mounts, "MDA1", 3000)
	mountRAM(t, mounts, "MDA0", MemoryVolume{Label: "ZERO"})
	mountRAM(t, mounts, "MDA1", MemoryVolume{Label: "ONE"})
	writeRAMFile(t, mounts, "MDA0:[000000]A.TXT", []string{"a"})

	if err := mounts.DismountAll(); err != nil {
		t.Fatalf("DismountAll: %v", err)
	}

	if !mounts.IsMemory("MDA0") || mounts.MemoryInUse("MDA0") == 0 {
		t.Fatal("DismountAll discarded a RAM disk")
	}

	mountRAM(t, mounts, "MDA0", MemoryVolume{})

	if got := readRAMFile(t, mounts, "MDA0:[000000]A.TXT"); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("after DismountAll the file holds %q", got)
	}

	if err := mounts.ReleaseMemory(); err != nil {
		t.Fatalf("ReleaseMemory: %v", err)
	}

	if _, mounted := mounts.Lookup("MDA0"); mounted {
		t.Error("ReleaseMemory left a RAM disk mounted")
	}

	if mounts.IsMemory("MDA0") || mounts.IsMemory("MDA1") {
		t.Error("ReleaseMemory left a RAM disk")
	}

	if defined, resident := mounts.MemoryTotal(); defined != 0 || resident != 0 {
		t.Errorf("MemoryTotal after ReleaseMemory = %d, %d; want 0, 0", defined, resident)
	}
}
