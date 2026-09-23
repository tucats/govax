package rms

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// disksDir returns the absolute path of testdata/disks/ (docs/PHASE-22.md's
// "Container format fidelity / simh interoperability" section), regardless
// of the directory `go test` happens to be run from. This mirrors
// internal/console/asm_test.go's own asmFixturePath helper: runtime.Caller
// gives us this source file's own path on disk, which is a fixed location
// relative to the repo root even if a test runner's current directory
// isn't.
func disksDir(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "disks")
}

// skipUnlessDiskPresent returns the full path to name under testdata/disks/
// if it exists, or calls t.Skip and never returns otherwise. testdata/disks/
// is gitignored (see that directory's own README.md) and only ever
// populated by hand on a real development machine, so every test in this
// file must be able to skip cleanly -- not fail -- on a fresh clone or in
// CI, where these large, licensed VAX/VMS disk images are never present.
func skipUnlessDiskPresent(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(disksDir(t), name)

	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not present (see testdata/disks/README.md) -- skipping simh interop test", path)
	}

	return path
}

// copyFile copies src to dst byte for byte, failing the test on any error.
// Used to make a scratch copy of testdata/disks/empty.dsk before mounting
// it read/write -- the fixture under testdata/disks/ is a real file a
// developer placed by hand, and this package's own tests must never modify
// it in place.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

// TestSimhInterop_readRealVMSDisk is docs/PHASE-22.md's primary ODS-2
// read-fidelity check: mount testdata/disks/rq0-ra92.dsk (a full, real
// VAX/VMS system disk this project had no part in creating, per that
// file's own doc comment "a full, real VAX/VMS system disk") read-only,
// list its master file directory (MFD) and at least one subdirectory
// under it, and read back at least one real file's records -- proving
// this port's ODS-2 support (via the sibling `ods2` module) can make
// sense of a volume built by real VMS software, not just one it wrote
// itself. Reading a govax-written file back only proves this port is
// internally consistent; reading a real one proves actual ODS-2 format
// compatibility, which is this phase's whole point (see PHASE-22.md's
// "Container format fidelity / simh interoperability" section).
//
// This test goes straight through ods2's own volume/ondisk API rather
// than through internal/rms's SYS$OPEN/SYS$GET handlers, matching
// PHASE-22.md subtask 15's own wording ("...or the underlying ods2 calls
// directly in an internal/rms test") -- there is no FAB/RAB or file name
// to hand-encode here at all, since the whole point is discovering real
// file names from the directory listing itself, not assuming one ahead of
// time the way every other test in this package does against its own
// freshly created fixtures.
func TestSimhInterop_readRealVMSDisk(t *testing.T) {
	path := skipUnlessDiskPresent(t, "rq0-ra92.dsk")

	// diskimage.Open (not OpenWritable) matches this half's "mounted
	// read-only" requirement -- and is also the only way to mount a
	// container this test must never be allowed to write to.
	container, err := diskimage.Open(path)
	if err != nil {
		t.Fatalf("diskimage.Open(%s): %v", path, err)
	}

	vol, err := volume.Mount(container)
	if err != nil {
		// volume.Mount failed after the container was already opened, so
		// this test -- like internal/rms.MountTable.Mount -- is
		// responsible for closing it again itself, or the open file
		// handle leaks.
		_ = container.Close()
		t.Fatalf("volume.Mount(%s): %v", path, err)
	}

	defer func() {
		if err := vol.Dismount(); err != nil {
			t.Errorf("Dismount: %v", err)
		}
	}()

	// ondisk.MasterFileDirectoryFid is the well-known, fixed file ID
	// every ODS-2 volume's master file directory (MFD) lives at -- real
	// VMS software and this port agree on it because it's part of the
	// on-disk format itself, not something either side gets to choose.
	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	mfdEntries, err := mfd.List()
	if err != nil {
		t.Fatalf("listing the MFD: %v", err)
	}

	if len(mfdEntries) == 0 {
		t.Fatal("a real VAX/VMS system disk's MFD listed zero entries -- wrong volume, or a genuine read-fidelity failure")
	}

	t.Logf("MFD: %d entries", len(mfdEntries))

	// Find a non-empty subdirectory to descend into and list -- a real
	// system disk's MFD entries ending ".DIR" (ondisk.DirEntry.Name
	// combines the file's name and type into one string, e.g.
	// "SYS0.DIR") are themselves directory files, opened the exact same
	// way as the MFD itself, just by a different Fid.
	var (
		subName    string
		subEntries []ondisk.DirEntry
	)

	for _, entry := range mfdEntries {
		if !strings.HasSuffix(entry.Name, ".DIR") {
			continue
		}

		sub, err := vol.OpenDirectory(entry.Fid)
		if err != nil {
			continue
		}

		list, err := sub.List()
		if err != nil || len(list) == 0 {
			continue
		}

		subName, subEntries = entry.Name, list

		break
	}

	if subEntries == nil {
		t.Fatal("found no non-empty subdirectory under the MFD to list")
	}

	t.Logf("subdirectory %s: %d entries", subName, len(subEntries))

	// Find a real, non-directory file with actual content in that
	// subdirectory, and read at least one record back through
	// odsrms.Reader -- the same reader type internal/rms's own SYS$GET
	// handler (connect.go's armForFAC) uses, so this is exercising real
	// production read logic against a volume this project never wrote a
	// single byte of.
	var readOK bool

	for _, entry := range subEntries {
		if strings.HasSuffix(entry.Name, ".DIR") {
			continue
		}

		f, err := vol.OpenFID(entry.Fid)
		if err != nil {
			continue
		}

		// HighestBlock == 0 means an empty file -- skip it in favor of
		// one that actually has records to read.
		if f.Header.RecordAttributes.HighestBlock == 0 {
			continue
		}

		reader, err := odsrms.NewReader(f)
		if err != nil {
			continue
		}

		record, err := reader.Next()
		if err != nil && !errors.Is(err, io.EOF) {
			continue
		}

		t.Logf("read %s/%s: first record is %d bytes", subName, entry.Name, len(record))

		readOK = true

		break
	}

	if !readOK {
		t.Fatalf("found no readable, non-empty file under %s to read back", subName)
	}
}

// TestSimhInterop_directoryListsRealVMSDisk is docs/PHASE-23.md subtask 5's
// own opt-in interop check: run a real DIRECTORY (Session.Directory, not
// ods2's lower-level Directory.List that TestSimhInterop_readRealVMSDisk
// already exercises directly) against testdata/disks/rq0-ra92.dsk's master
// file directory, mounted read-only exactly the way an operator would MOUNT
// a real VAX/VMS system disk. This is the one DIRECTORY test in this phase
// that lists a volume this project never wrote a single byte of, proving
// the glob/formatting path handles a real system disk's MFD (likely much
// larger and differently laid out than this package's own small, hermetic
// test fixtures) rather than only ever exercising its own creations.
func TestSimhInterop_directoryListsRealVMSDisk(t *testing.T) {
	path := skipUnlessDiskPresent(t, "rq0-ra92.dsk")

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", path, false); err != nil {
		t.Fatalf("Mount (read-only): %v", err)
	}

	defer func() {
		if err := mounts.Dismount("DUA0"); err != nil {
			t.Errorf("Dismount: %v", err)
		}
	}()

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, err := s.Directory("", DirectoryOptions{Full: true})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "Directory DUA0:[]") {
		t.Errorf("Directory output = %q, want it to contain a DUA0:[] header", out)
	}

	if !strings.Contains(out, "Total of ") {
		t.Fatalf("Directory output = %q, want a file-count summary", out)
	}

	t.Logf("DIRECTORY/FULL of a real VAX/VMS system disk's MFD:\n%s", out)
}

// TestSimhInterop_writeThenRereadEmptyDsk is docs/PHASE-22.md's write-path
// interop check: testdata/disks/empty.dsk is a real container `ods2`
// itself already initialized (a real home block, INDEXF.SYS, BITMAP.SYS,
// MFD, no user files -- see that file's own role described in
// PHASE-22.md's "Container format fidelity / simh interoperability"
// section), copied to a scratch file so this test can mount it read/write
// without ever touching the checked-in fixture. It then exercises this
// package's own real SYS$CREATE/SYS$CONNECT/SYS$PUT/SYS$CLOSE handlers
// (createAndCloseTestFile, shared with open_test.go's own fixtures) to
// write one record, dismounts, remounts the exact same scratch file
// read-only under a completely separate MountTable/Context (nothing
// shared with the write half except the on-disk bytes themselves -- the
// same thing a real operator does: write while mounted read/write, then
// remount read-only for a different consumer), and confirms
// SYS$OPEN/SYS$CONNECT/SYS$GET reads the record back correctly.
func TestSimhInterop_writeThenRereadEmptyDsk(t *testing.T) {
	srcPath := skipUnlessDiskPresent(t, "empty.dsk")

	scratchPath := filepath.Join(t.TempDir(), "empty-scratch.dsk")
	copyFile(t, srcPath, scratchPath)

	// ---- write half: mount read/write, CREATE+PUT+CLOSE one record ----

	writeMounts := NewMountTable()
	if err := writeMounts.Mount("DUA0", scratchPath, true); err != nil {
		t.Fatalf("Mount (read/write): %v", err)
	}

	writeLogicals := iodev.NewLogicalNameTable()
	writeLogicals.InitLogicals()

	writeCtx := &Context{
		Mem:      vm.NewMemory(1 << 20),
		CPU:      vax.New(),
		Mounts:   writeMounts,
		Files:    NewFileTable(nil),
		Logicals: writeLogicals,
		Console:  &bytes.Buffer{},
	}

	// newFAB (create_test.go) always asks for a fixed-format, 80-byte-
	// maximum-record file, so the record written here has to be exactly
	// that long, matching every other disk-writing test in this package.
	record := bytes.Repeat([]byte("interop"), 12)[:80]
	createAndCloseTestFile(t, writeCtx, "INTEROP.DAT", record)

	if err := writeMounts.Dismount("DUA0"); err != nil {
		t.Fatalf("Dismount (read/write): %v", err)
	}

	// ---- read half: a fresh mount/Context, entirely separate from the
	// one above except for the scratch file's own bytes on disk ----

	readMounts := NewMountTable()
	if err := readMounts.Mount("DUA0", scratchPath, false); err != nil {
		t.Fatalf("Mount (read-only remount): %v", err)
	}

	defer func() {
		if err := readMounts.Dismount("DUA0"); err != nil {
			t.Errorf("Dismount (read-only): %v", err)
		}
	}()

	readLogicals := iodev.NewLogicalNameTable()
	readLogicals.InitLogicals()

	readCtx := &Context{
		Mem:      vm.NewMemory(1 << 20),
		CPU:      vax.New(),
		Mounts:   readMounts,
		Files:    NewFileTable(nil),
		Logicals: readLogicals,
		Console:  &bytes.Buffer{},
	}

	// openAndConnectForRead (get_test.go) does SYS$OPEN (for GET access)
	// then SYS$CONNECT, exactly what a real VAX program runs before its
	// first SYS$GET against an already-existing file.
	openAndConnectForRead(t, readCtx, "INTEROP.DAT")

	putByte(t, readCtx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, readCtx, testRabAddr+rabRBF, testRecordAddr)

	r0, err := SysGet(readCtx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsNormal {
		t.Fatalf("SysGet r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	got := make([]byte, len(record))
	for i := range got {
		got[i] = readByte(t, readCtx, testRecordAddr+uint32(i))
	}

	if !bytes.Equal(got, record) {
		t.Errorf("record read back from the remounted scratch volume = %q, want %q", got, record)
	}
}
