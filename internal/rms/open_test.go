package rms

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/ondisk"
)

// createAndCloseTestFile builds a real, closed Fixed-format (80-byte
// record), sequential file named name on the fixture's mounted "DUA0"
// device, holding exactly one record (record, which must be 80 bytes —
// see newFAB's own comment on the fixed record size every fixture FAB
// declares). It does this entirely through this package's own already-
// tested services — SysCreate, SysConnect, SysPut, SysClose — the exact
// CREATE-then-PUT-then-CLOSE sequence a real VAX program performs, rather
// than reaching into the sibling ods2 module's API directly the way some
// of this package's earlier tests had to before SysOpen (this file) and
// SysClose (close.go) existed yet.
//
// It reuses the fixture's shared testFabAddr/testRabAddr scratch memory
// (create_test.go/connect_test.go's own constants); that's safe here
// because SysClose fully finishes with the FAB before this function
// returns, and every SysOpen test below immediately overwrites that same
// memory with its own fresh newFAB call before using it again.
func createAndCloseTestFile(t *testing.T, ctx *Context, name string, record []byte) {
	t.Helper()

	newFAB(t, ctx, "DUA0:"+name)

	if _, err := SysCreate(ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, ctx, testFabAddr)
	putByte(t, ctx, testRabAddr+rabRAC, racSeq)
	putRecord(t, ctx, testRabAddr, record)

	if _, err := SysPut(ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if _, err := SysClose(ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysClose: %v", err)
	}
}

// newReadOnlyFixtureWithFile builds a createFixture (matching create_test.
// go's own newCreateFixture shape) whose "DUA0" device already has one
// real, closed file named name (holding record) on it before this
// fixture's own mount is established with the given writable flag.
//
// This exists because MountTable itself has no "create a file, then mount
// read-only" convenience — Mount always opens whatever is already on disk
// at a path, so populating the volume has to happen through a first,
// separate writable mount that's dismounted again before the fixture's own
// mount (writable or not, per the writable argument) takes over. The
// underlying container file on disk is unaffected by which mount is
// currently open on it — this is exactly what a real operator does when
// preparing read-only test media: write the content while writable, then
// remount it read-only for whoever actually consumes it.
func newReadOnlyFixtureWithFile(t *testing.T, writable bool, name string, record []byte) *createFixture {
	t.Helper()

	path := newTestVolumeFile(t, "TESTVOL")

	seedMounts := NewMountTable()
	if err := seedMounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount (writable, to seed the fixture's file): %v", err)
	}

	vol, _ := seedMounts.Lookup("DUA0")

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	bm, err := dir.Device.Bitmap()
	if err != nil {
		t.Fatalf("Bitmap: %v", err)
	}

	ib, err := dir.Device.IndexBitmap()
	if err != nil {
		t.Fatalf("IndexBitmap: %v", err)
	}

	written, err := vol.CreateFile(dir, name, ondisk.RecAttr{
		Format:        ondisk.RecordFormatFixed,
		MaxRecordSize: uint16(len(record)),
	}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	if err := written.WriteBlock(1, padToBlock(record)); err != nil {
		t.Fatalf("WriteBlock: %v", err)
	}

	if err := written.CloseWithFinalByte(uint16(len(record))); err != nil {
		t.Fatalf("CloseWithFinalByte: %v", err)
	}

	if err := seedMounts.Dismount("DUA0"); err != nil {
		t.Fatalf("Dismount (seeding mount): %v", err)
	}

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", path, writable); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	logicals := newTestLogicals(t)

	console := &bytes.Buffer{}

	return &createFixture{
		ctx: &Context{
			Mem:      vm.NewMemory(1 << 20),
			CPU:      vax.New(),
			Mounts:   mounts,
			Files:    NewFileTable(nil),
			Logicals: logicals,
			Console:  console,
		},
		console: console,
	}
}

// padToBlock zero-pads data out to a full ondisk.BlockSize block, the
// shape volume.File.WriteBlock requires.
func padToBlock(data []byte) []byte {
	block := make([]byte, ondisk.BlockSize)
	copy(block, data)

	return block
}

// TestSysOpen_console confirms SYS$OPEN against "TTA0:" binds a fresh IFI
// to the console writer, exactly like SysCreate's own console case
// (create.go) — real VMS RMS genuinely supports opening a terminal device
// this way (see create.go's consoleDeviceName doc comment).
func TestSysOpen_console(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok || !h.IsConsole() {
		t.Fatal("SysOpen against TTA0: did not allocate a console handle")
	}
}

// TestSysOpen_diskFileForRead confirms SYS$OPEN against a real, existing
// disk file resolves it (rather than creating a new, empty one — the
// difference from SYS$CREATE) and returns an IFI whose FileHandle wraps
// the same underlying file, still holding its one written record — proven
// by then doing the same SYS$CONNECT (for GET access) a real VAX program
// would do next, and reading the record back.
func TestSysOpen_diskFileForRead(t *testing.T) {
	f := newCreateFixture(t, true)

	record := bytes.Repeat([]byte{'R'}, 80)
	createAndCloseTestFile(t, f.ctx, "READ.DAT", record)

	newFAB(t, f.ctx, "DUA0:READ.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)
	if ifi < firstUserIFI {
		t.Fatalf("IFI = %d, want >= %d", ifi, firstUserIFI)
	}

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok {
		t.Fatalf("Files.Lookup(%d) = not found", ifi)
	}

	if h.IsConsole() || h.File == nil {
		t.Fatal("allocated handle is not a real ODS-2 file")
	}

	connectRAB(t, f.ctx, testFabAddr)

	h, _ = f.ctx.Files.Lookup(ifi)
	if h.Reader == nil {
		t.Fatal("handle.Reader = nil after SYS$CONNECT for GET access")
	}

	got, err := h.Reader.Next()
	if err != nil {
		t.Fatalf("Reader.Next: %v", err)
	}

	if !bytes.Equal(got, record) {
		t.Errorf("record read back = %q, want %q", got, record)
	}
}

// TestSysOpen_diskFileForWrite confirms SYS$OPEN honoring FAB$B_FAC's
// FAB$V_PUT bit arms the found file for writing (volume.File.OpenForWrite)
// so that a following SYS$CONNECT can actually construct a usable Writer —
// docs/PHASE-22.md subtask 9's own description of what's new here versus
// SysCreate. Without that arming, SYS$CONNECT's own odsrms.NewWriter would
// still succeed (it doesn't itself check for an armed File), but the first
// SYS$PUT that tried to extend the file past its current length would fail
// deep inside ods2 — see writefile.go's own OpenForWrite doc comment for
// why. The stream is connected with RAB$V_EOF: without it, it would start
// at the file's first record, where a $PUT can't go (RMS$_NEF;
// TestSharing_appendToExisting).
func TestSysOpen_diskFileForWrite(t *testing.T) {
	f := newCreateFixture(t, true)

	createAndCloseTestFile(t, f.ctx, "WRITE.DAT", bytes.Repeat([]byte{'W'}, 80))

	newFAB(t, f.ctx, "DUA0:WRITE.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facPut)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)
	putRAB(t, f.ctx, testFabAddr)
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, ropEOF)

	if _, err := SysConnect(f.ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	h, _ := f.ctx.Files.Lookup(ifi)
	if h.Writer == nil {
		t.Fatal("handle.Writer = nil, want a Writer armed by SysConnect")
	}

	if err := h.Writer.Put(bytes.Repeat([]byte{'X'}, 80)); err != nil {
		t.Errorf("Put through the SysConnect-armed Writer: %v", err)
	}
}

// TestSysOpen_explicitVersion confirms a file spec naming an explicit
// version number (";1") resolves the same file an unversioned spec would
// (the fixture only ever creates one version of any given name), rather
// than being rejected as an unsupported version syntax.
func TestSysOpen_explicitVersion(t *testing.T) {
	f := newCreateFixture(t, true)

	createAndCloseTestFile(t, f.ctx, "VER.DAT", bytes.Repeat([]byte{'V'}, 80))

	newFAB(t, f.ctx, "DUA0:VER.DAT;1")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}
}

// TestSysOpen_invalidVersion confirms a file spec with a wildcard version
// (";*") fails with RMS$_WLD, as any wildcard does on $OPEN, rather than
// being silently misinterpreted or panicking inside strconv.Atoi.
func TestSysOpen_invalidVersion(t *testing.T) {
	f := newCreateFixture(t, true)

	createAndCloseTestFile(t, f.ctx, "VER.DAT", bytes.Repeat([]byte{'V'}, 80))

	newFAB(t, f.ctx, "DUA0:VER.DAT;*")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsWildcardError {
		t.Errorf("r0 = %d, want rmsWildcardError (%d)", r0, rmsWildcardError)
	}
}

// TestSysOpen_deviceNotMounted confirms a file spec naming a device with
// nothing mounted on it at all fails with RMS$_DNR, matching SysCreate's
// own equivalent check (create.go's TestSysCreate_deviceNotMounted).
func TestSysOpen_deviceNotMounted(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUB0:TEST.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsDeviceNotReady {
		t.Errorf("r0 = %d, want rmsDeviceNotReady (%d)", r0, rmsDeviceNotReady)
	}
}

// TestSysOpen_readOnlyMountWantsWrite confirms asking to open an existing
// file for writing (FAB$V_PUT) on a device mounted read-only fails with
// RMS$_PRV and leaves FAB$W_IFI untouched, mirroring SysCreate's own
// TestSysCreate_readOnlyMount.
func TestSysOpen_readOnlyMountWantsWrite(t *testing.T) {
	record := bytes.Repeat([]byte{'Z'}, 80)
	f := newReadOnlyFixtureWithFile(t, false, "RO.DAT", record)

	newFAB(t, f.ctx, "DUA0:RO.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facPut)
	putWord(t, f.ctx, testFabAddr+fabIFI, 0xFFFF) // sentinel: must stay untouched

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsPrivilegeViolation {
		t.Errorf("r0 = %d, want rmsPrivilegeViolation (%d)", r0, rmsPrivilegeViolation)
	}

	if ifi := readWord(t, f.ctx, testFabAddr+fabIFI); ifi != 0xFFFF {
		t.Errorf("FAB$W_IFI = %#x, want the untouched sentinel 0xFFFF", ifi)
	}
}

// TestSysOpen_readOnlyMountReadOnlyOK confirms real RMS behavior: opening
// a file for read access only (FAB$V_GET) succeeds even on a device
// mounted read-only — only actually asking to write is a problem, unlike
// SYS$CREATE, which always implies writing.
func TestSysOpen_readOnlyMountReadOnlyOK(t *testing.T) {
	record := bytes.Repeat([]byte{'Y'}, 80)
	f := newReadOnlyFixtureWithFile(t, false, "RO2.DAT", record)

	newFAB(t, f.ctx, "DUA0:RO2.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	ifi := connectRAB(t, f.ctx, testFabAddr)

	h, _ := f.ctx.Files.Lookup(ifi)

	got, err := h.Reader.Next()
	if err != nil {
		t.Fatalf("Reader.Next: %v", err)
	}

	if !bytes.Equal(got, record) {
		t.Errorf("record read back = %q, want %q", got, record)
	}
}

// TestSysOpen_facNone: a FAB that asks for no access (FAB$B_FAC 0, a $FAB
// with no FAC=) is opened for GET, as the RMS Reference says, and $CONNECT
// arms it for reading. The FAB's FAC stays 0.
func TestSysOpen_facNone(t *testing.T) {
	record := bytes.Repeat([]byte{'N'}, 80)
	f := newReadOnlyFixtureWithFile(t, false, "NONE.DAT", record)

	newFAB(t, f.ctx, "DUA0:NONE.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, 0)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Fatalf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if fac := readByte(t, f.ctx, testFabAddr+fabFAC); fac != 0 {
		t.Errorf("FAB$B_FAC = %#x after $OPEN, want it left 0", fac)
	}

	ifi := connectRAB(t, f.ctx, testFabAddr)
	h, _ := f.ctx.Files.Lookup(ifi)

	if h.Reader == nil || h.Writer != nil {
		t.Fatalf("$CONNECT armed Reader %v, Writer %v; want reading only", h.Reader != nil, h.Writer != nil)
	}

	if got, err := h.Reader.Next(); err != nil || !bytes.Equal(got, record) {
		t.Errorf("record read back = %q, %v; want %q", got, err, record)
	}
}

// TestSysOpen_facDeleteOnly: a FAB whose only access is DEL opens (DEL
// is write access, and since Phase 49 every kind of access is
// implemented: $DELETE of a sequential file's record is RMS$_IOP).
func TestSysOpen_facDeleteOnly(t *testing.T) {
	f := newCreateFixture(t, true)
	writeFile(t, f.ctx.Mounts, "TEST.DAT", "ONE")

	newFAB(t, f.ctx, "DUA0:TEST.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, byte(vmsConst("FAB$M_DEL")))

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %#x, want RMS$_NORMAL", r0)
	}
}

// TestSysOpen_terminalFacNone: $OPEN of SYS$INPUT (the terminal) from a
// $FAB with no FAC=, then $GET, reads a line typed: the common way a VMS
// program reads its terminal.
func TestSysOpen_terminalFacNone(t *testing.T) {
	f := newCreateFixture(t, true)
	f.ctx.Console = &bytes.Buffer{}
	f.ctx.ConsoleIn = bufio.NewReader(strings.NewReader("typed\n"))
	f.ctx.Logicals = newTestLogicals(t)

	newFAB(t, f.ctx, "SYS$INPUT:")
	putByte(t, f.ctx, testFabAddr+fabFAC, 0)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Fatalf("$OPEN of SYS$INPUT: = %#x, want RMS$_NORMAL", r0)
	}

	connectRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testUserBufAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, 80)

	if r0, got := consoleGet(t, f); r0 != rmsNormal || got != "typed" {
		t.Errorf("$GET = %#x, %q; want RMS$_NORMAL, %q", r0, got, "typed")
	}
}

// TestSysOpen_noNameGiven confirms a file spec naming only a device, with
// no file name at all, is rejected rather than reaching
// volume.Directory.Lookup with an empty name — mirroring SysCreate's own
// TestSysCreate_noNameGiven.
func TestSysOpen_noNameGiven(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsFileNotFound {
		t.Errorf("r0 = %d, want rmsFileNotFound (%d)", r0, rmsFileNotFound)
	}
}

// TestSysOpen_fileNotFound confirms a well-formed spec naming a file that
// genuinely doesn't exist on the volume fails with RMS$_FNF.
func TestSysOpen_fileNotFound(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:NOSUCHFILE.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsFileNotFound {
		t.Errorf("r0 = %d, want rmsFileNotFound (%d)", r0, rmsFileNotFound)
	}
}

// TestSysOpen_directoryNotFound confirms a file spec naming a subdirectory
// that doesn't exist on the volume fails cleanly (RMS$_DNF) rather than
// panicking inside filespec.ResolveDirectory, mirroring SysCreate's own
// TestSysCreate_directoryNotFound.
func TestSysOpen_directoryNotFound(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:[NOSUCHDIR]TEST.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsDirNotFound {
		t.Errorf("r0 = %d, want rmsDirNotFound (%d)", r0, rmsDirNotFound)
	}
}

// TestSysOpen_logicalNameTranslation confirms a file spec that is itself a
// defined logical name is translated before being parsed — here, a
// made-up logical pointing at "TTA0:" should reach the same console path
// TestSysOpen_console exercises directly, mirroring SysCreate's own
// TestSysCreate_logicalNameTranslation.
func TestSysOpen_logicalNameTranslation(t *testing.T) {
	f := newCreateFixture(t, true)
	defineTestLogical(t, f.ctx.Logicals, "MYIN", "TTA0:")
	newFAB(t, f.ctx, "MYIN")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysOpen: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok || !h.IsConsole() {
		t.Fatal("logical-name-translated spec did not resolve to the console handle")
	}
}

// TestSysOpen_translationAndSearchLists covers docs/PHASE-25.md subtask
// 7's file-spec translation in SYS$OPEN: the leftmost component is
// translated, SYS$DISK supplies a missing device, a search list opens the
// first element that has the file (or reports the last element's
// status), and an untranslatable name is RMS$_LNE.
func TestSysOpen_translationAndSearchLists(t *testing.T) {
	f := newCreateFixture(t, true)
	createAndCloseTestFile(t, f.ctx, "HERE.DAT", []byte("hello"))

	db := f.ctx.Logicals
	defineTestLogical(t, db, "D", "DUA0:")
	defineTestLogical(t, db, "LIST", "DUA0:[NOSUCHDIR]", "DUA0:")
	defineTestLogical(t, db, "GAP", "DUA9:", "D:")
	defineTestLogical(t, db, "NONE", "DUA0:[NOSUCHDIR]", "DUA9:")
	defineTestLogical(t, db, "LOOP1", "LOOP2:")
	defineTestLogical(t, db, "LOOP2", "LOOP1:")

	tests := []struct {
		spec string
		want uint32
	}{
		{"D:HERE.DAT", rmsNormal},
		{"LIST:HERE.DAT", rmsNormal},
		{"GAP:HERE.DAT", rmsNormal},
		{"LIST:NOSUCH.DAT", rmsFileNotFound},
		{"NONE:HERE.DAT", rmsDeviceNotReady},
		{"LOOP1:HERE.DAT", rmsLogicalNameError},
		{"HERE.DAT", rmsDeviceNotReady}, // no SYS$DISK yet
	}

	check := func(spec string, want uint32) {
		t.Helper()

		newFAB(t, f.ctx, spec)
		putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

		r0, err := SysOpen(f.ctx, []uint32{testFabAddr})
		if err != nil {
			t.Fatalf("SysOpen(%s): %v", spec, err)
		}

		if r0 != want {
			t.Errorf("SysOpen(%s) = %#x, want %#x", spec, r0, want)
		}

		if r0 == rmsNormal {
			if _, err := SysClose(f.ctx, []uint32{testFabAddr}); err != nil {
				t.Fatalf("SysClose: %v", err)
			}
		}
	}

	for _, tt := range tests {
		check(tt.spec, tt.want)
	}

	defineTestLogical(t, db, "SYS$DISK", "DUA0:")
	check("HERE.DAT", rmsNormal)
}

// TestSysCreate_searchListUsesFirstElement: $CREATE puts the file in a
// search list's first element, even when that fails.
func TestSysCreate_searchListUsesFirstElement(t *testing.T) {
	f := newCreateFixture(t, true)
	defineTestLogical(t, f.ctx.Logicals, "GOOD", "DUA0:", "DUA9:")
	defineTestLogical(t, f.ctx.Logicals, "BAD", "DUA9:", "DUA0:")

	for _, tt := range []struct {
		spec string
		want uint32
	}{
		{"GOOD:NEW.DAT", rmsNormal},
		{"BAD:NEW.DAT", rmsDeviceNotReady},
	} {
		newFAB(t, f.ctx, tt.spec)

		r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
		if err != nil {
			t.Fatalf("SysCreate(%s): %v", tt.spec, err)
		}

		if r0 != tt.want {
			t.Errorf("SysCreate(%s) = %#x, want %#x", tt.spec, r0, tt.want)
		}

		if r0 == rmsCreated {
			if _, err := SysClose(f.ctx, []uint32{testFabAddr}); err != nil {
				t.Fatalf("SysClose: %v", err)
			}
		}
	}
}
