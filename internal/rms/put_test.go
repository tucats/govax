package rms

import (
	"bytes"
	"testing"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
)

// testRecordAddr is where these tests lay out a SYS$PUT record's raw bytes
// in the fixture's memory — chosen clear of every other address this
// package's test fixtures already use (testFabAddr, testFnaAddr,
// testRabAddr) so a wrong offset constant would land in obviously
// unrelated (zeroed) memory rather than silently overlapping something
// else.
const testRecordAddr = 0x5000

// putRecord writes data's bytes verbatim at testRecordAddr and points a
// RAB (already laid out at rabAddr by putRAB) at it via RAB$L_RBF/
// RAB$W_RSZ — the two fields a calling VAX program sets before SYS$PUT to
// say "the record to write starts here, and is this many bytes long".
func putRecord(t *testing.T, ctx *Context, rabAddr uint32, data []byte) {
	t.Helper()

	for i, b := range data {
		putByte(t, ctx, testRecordAddr+uint32(i), b)
	}

	putLongwordAt(t, ctx, rabAddr+rabRBF, testRecordAddr)
	putWord(t, ctx, rabAddr+rabRSZ, uint16(len(data)))
}

// connectRAB is a small test-only convenience that performs the
// SYS$CONNECT step (putRAB + SysConnect) a SYS$PUT test always needs
// first — a RAB has to be bound to an open file, and (for a real ODS-2
// file) armed for writing, before SYS$PUT can do anything with it. Returns
// the IFI the underlying FAB was assigned, so a test can look its
// FileHandle back up directly if it wants to.
func connectRAB(t *testing.T, ctx *Context, fabAddr uint32) uint16 {
	t.Helper()

	ifi := readWord(t, ctx, fabAddr+fabIFI)

	putRAB(t, ctx, fabAddr)

	if _, err := SysConnect(ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	return ifi
}

// TestSysPut_console confirms SYS$PUT against a RAB connected to the
// TTA0: console pseudo-device writes the record's bytes straight to the
// console, followed by a newline — matching the deleted Phase 10 RMS
// stopgap's own console-output convention for SYS$PUT (see put.go's own
// doc comment on why the newline is added).
func TestSysPut_console(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, f.ctx, testFabAddr)

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putRecord(t, f.ctx, testRabAddr, []byte("hello, console"))

	r0, err := SysPut(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if sts := readLongword(t, f.ctx, testRabAddr+rabSTS); sts != rmsNormal {
		t.Errorf("RAB$L_STS = %d, want rmsNormal (%d)", sts, rmsNormal)
	}

	if got, want := f.console.String(), "hello, console\n"; got != want {
		t.Errorf("console output = %q, want %q", got, want)
	}
}

// TestSysPut_diskFile confirms SYS$PUT against a RAB connected to a real
// mounted ODS-2 volume actually writes the record through to disk: after
// the write, this test closes the Writer directly (SYS$CLOSE is a later
// subtask — docs/PHASE-22.md's subtask 8 — so this test reaches into the
// sibling ods2 module's own API the same way TestSysConnect_readArming
// already does for its own read-side fixture) and reopens the file fresh
// to read the record back with an independent odsrms.Reader, confirming
// the bytes match exactly what SysPut was asked to write.
func TestSysPut_diskFile(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT") // facPut, orgSeq, RecordFormatFixed, MRS 80 — see newFAB

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	ifi := connectRAB(t, f.ctx, testFabAddr)

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	// newFAB's own comment says MRS (maximum record size) 80, and its
	// record format is Fixed — a Fixed-format record must be exactly
	// that many bytes long (see ods2's rms.Writer.putFixed), so the test
	// record is padded out to 80 bytes.
	record := make([]byte, 80)
	copy(record, "hello, ods2 disk file")
	putRecord(t, f.ctx, testRabAddr, record)

	r0, err := SysPut(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	handle, ok := f.ctx.Files.Lookup(ifi)
	if !ok {
		t.Fatalf("Files.Lookup(%d) = not found", ifi)
	}

	// Closing the Writer finalizes the file's on-disk end-of-file
	// position (rms.Writer.Close's own doc comment) — without this, the
	// record just written wouldn't be visible to a fresh Reader at all,
	// the same way an unflushed write in many I/O libraries isn't
	// visible until the file is closed or explicitly flushed.
	if err := handle.Writer.Close(); err != nil {
		t.Fatalf("closing the Writer SysConnect armed: %v", err)
	}

	vol, ok := f.ctx.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("DUA0 not mounted")
	}

	reopened, err := vol.OpenFID(handle.File.Header.Fid)
	if err != nil {
		t.Fatalf("OpenFID: %v", err)
	}

	reader, err := odsrms.NewReader(reopened)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	got, err := reader.Next()
	if err != nil {
		t.Fatalf("Reader.Next: %v", err)
	}

	if !bytes.Equal(got, record) {
		t.Errorf("record read back = %q, want %q", got, record)
	}
}

// TestSysPut_multipleRecords confirms successive SYS$PUT calls through the
// same RAB append records in order, rather than each one overwriting the
// last — the ordinary way a VAX program writes a multi-record file (one
// SYS$PUT call per record).
func TestSysPut_multipleRecords(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	ifi := connectRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	records := [][]byte{
		bytes.Repeat([]byte{'A'}, 80),
		bytes.Repeat([]byte{'B'}, 80),
		bytes.Repeat([]byte{'C'}, 80),
	}

	for _, record := range records {
		putRecord(t, f.ctx, testRabAddr, record)

		r0, err := SysPut(f.ctx, []uint32{testRabAddr})
		if err != nil {
			t.Fatalf("SysPut: %v", err)
		}

		if r0 != rmsNormal {
			t.Fatalf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
		}
	}

	handle, _ := f.ctx.Files.Lookup(ifi)
	if err := handle.Writer.Close(); err != nil {
		t.Fatalf("closing the Writer: %v", err)
	}

	vol, _ := f.ctx.Mounts.Lookup("DUA0")

	reopened, err := vol.OpenFID(handle.File.Header.Fid)
	if err != nil {
		t.Fatalf("OpenFID: %v", err)
	}

	reader, err := odsrms.NewReader(reopened)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	for i, want := range records {
		got, err := reader.Next()
		if err != nil {
			t.Fatalf("Reader.Next (record %d): %v", i, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("record %d = %q, want %q", i, got, want)
		}
	}
}

// TestSysPut_invalidIFI confirms SYS$PUT against a RAB that was never
// SYS$CONNECTed (RAB$W_ISI never set to a real IFI) fails with RMS$_IFI
// rather than panicking on a nil FileHandle.
func TestSysPut_invalidIFI(t *testing.T) {
	f := newCreateFixture(t, true)

	// Deliberately never call SysCreate/SysConnect — just a bare RAB with
	// its RAB$W_ISI left at zero (ifi.go's reserved, never-allocated
	// sentinel slot).
	putRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putRecord(t, f.ctx, testRabAddr, []byte("no file behind this RAB"))

	r0, err := SysPut(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if r0 != rmsInvalidIFI {
		t.Errorf("r0 = %d, want rmsInvalidIFI (%d)", r0, rmsInvalidIFI)
	}
}

// TestSysPut_invalidRAC confirms a RAB whose RAB$B_RAC names anything
// other than sequential access (rab.go's racSeq) fails with RMS$_RAC —
// this phase implements sequential organization only.
func TestSysPut_invalidRAC(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, f.ctx, testFabAddr)

	putByte(t, f.ctx, testRabAddr+rabRAC, racKey) // not sequential
	putRecord(t, f.ctx, testRabAddr, make([]byte, 80))

	r0, err := SysPut(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if r0 != rmsInvalidRAC {
		t.Errorf("r0 = %d, want rmsInvalidRAC (%d)", r0, rmsInvalidRAC)
	}
}

// TestSysPut_noWriteAccess confirms SYS$PUT through a RAB that was
// SYS$CONNECTed for reading only (FAB$B_FAC asking for FAB$V_GET, not
// FAB$V_PUT — the same setup TestSysConnect_readArming in connect_test.go
// builds) fails with RMS$_PRV rather than dereferencing a nil Writer.
func TestSysPut_noWriteAccess(t *testing.T) {
	f := newCreateFixture(t, true)

	vol, ok := f.ctx.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("DUA0 not mounted")
	}

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

	written, err := vol.CreateFile(dir, "READONLY.DAT", ondisk.RecAttr{
		Format:        ondisk.RecordFormatFixed,
		MaxRecordSize: 80,
	}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	if err := written.Close(); err != nil {
		t.Fatalf("closing the freshly written fixture file: %v", err)
	}

	reopened, err := vol.OpenFID(written.Header.Fid)
	if err != nil {
		t.Fatalf("OpenFID: %v", err)
	}

	ifi := f.ctx.Files.Alloc(&FileHandle{File: reopened})

	newFAB(t, f.ctx, "DUA0:READONLY.DAT")
	putWord(t, f.ctx, testFabAddr+fabIFI, ifi)
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	connectRAB(t, f.ctx, testFabAddr)

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putRecord(t, f.ctx, testRabAddr, make([]byte, 80))

	r0, err := SysPut(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if r0 != rmsPrivilegeViolation {
		t.Errorf("r0 = %d, want rmsPrivilegeViolation (%d)", r0, rmsPrivilegeViolation)
	}
}

// TestSysPut_wrongRecordSize confirms a record whose length doesn't match
// a Fixed-format file's own declared maximum record size (newFAB's MRS
// 80) fails with RMS$_RSZ rather than silently truncating/padding the
// record or corrupting the file.
func TestSysPut_wrongRecordSize(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, f.ctx, testFabAddr)

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putRecord(t, f.ctx, testRabAddr, []byte("too short")) // not 80 bytes

	r0, err := SysPut(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysPut: %v", err)
	}

	if r0 != rmsRecordTooBig {
		t.Errorf("r0 = %d, want rmsRecordTooBig (%d)", r0, rmsRecordTooBig)
	}
}
