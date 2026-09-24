package rms

import (
	"testing"

	"github.com/tucats/ods2/ondisk"
)

// testRabAddr is where these tests lay out a RAB in the fixture's memory
// — chosen well clear of testFabAddr/testFnaAddr (create_test.go) so a
// wrong offset constant reading past one region lands in obviously
// unrelated (zeroed) memory instead of silently overlapping another.
const testRabAddr = 0x4000

// putRAB writes a RAB at testRabAddr whose RAB$L_FAB field points at
// fabAddr — the one field SYS$CONNECT actually needs set going in
// (rab.go's rabFAB); every other RAB field this package's later subtasks
// will use (RAB$B_RAC, RAB$W_RSZ, ...) is left zero, since SYS$CONNECT
// itself never reads them.
func putRAB(t *testing.T, ctx *Context, fabAddr uint32) {
	t.Helper()

	putLongwordAt(t, ctx, testRabAddr+rabFAB, fabAddr)
}

// TestSysConnect_console confirms SYS$CONNECT against a FAB already
// SYS$CREATEd on TTA0: simply copies its IFI into the RAB's RAB$W_ISI —
// the console case needs no further arming (see SysConnect's own doc
// comment on why).
func TestSysConnect_console(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	wantIFI := readWord(t, f.ctx, testFabAddr+fabIFI)

	putRAB(t, f.ctx, testFabAddr)

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if isi := readWord(t, f.ctx, testRabAddr+rabISI); isi != wantIFI {
		t.Errorf("RAB$W_ISI = %d, want the FAB's own IFI %d", isi, wantIFI)
	}

	if sts := readLongword(t, f.ctx, testRabAddr+rabSTS); sts != rmsNormal {
		t.Errorf("RAB$L_STS = %d, want rmsNormal (%d)", sts, rmsNormal)
	}
}

// TestSysConnect_diskWrite confirms SYS$CONNECT against a freshly
// SYS$CREATEd disk file arms it for writing: the resulting FileHandle
// gets a real *rms.Writer (from the sibling ods2 module), and that
// Writer is immediately usable to put a record — the very next thing a
// real VAX program would do (SYS$PUT, docs/PHASE-22.md's subtask 7).
func TestSysConnect_diskWrite(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT") // facPut, orgSeq, RecordFormatFixed, MRS 80 — see newFAB

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)
	putRAB(t, f.ctx, testFabAddr)

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok {
		t.Fatalf("Files.Lookup(%d) = not found", ifi)
	}

	if h.Writer == nil {
		t.Fatal("handle.Writer = nil, want a Writer armed by SysConnect")
	}

	if h.Reader != nil {
		t.Error("handle.Reader is also set — a stream should never be armed both ways at once (ifi.go's own FileHandle invariant)")
	}

	record := make([]byte, 80)
	copy(record, "hello, ods2")

	if err := h.Writer.Put(record); err != nil {
		t.Errorf("Put through the SysConnect-armed Writer: %v", err)
	}
}

// TestSysConnect_invalidIFI confirms a RAB whose FAB names an IFI that
// isn't (or is no longer) open fails with RMS$_IFI, and leaves the RAB's
// own RAB$W_ISI untouched rather than writing a stale/bogus value into
// it.
func TestSysConnect_invalidIFI(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")
	// Deliberately never call SysCreate — fabIFI is whatever newFAB left
	// it at (zero, one of the reserved slots FileTable.Alloc never hands
	// out), so no handle is registered under it.
	putRAB(t, f.ctx, testFabAddr)
	putWord(t, f.ctx, testRabAddr+rabISI, 0xFFFF) // sentinel: must stay untouched

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	if r0 != rmsInvalidIFI {
		t.Errorf("r0 = %d, want rmsInvalidIFI (%d)", r0, rmsInvalidIFI)
	}

	if isi := readWord(t, f.ctx, testRabAddr+rabISI); isi != 0xFFFF {
		t.Errorf("RAB$W_ISI = %#x, want the untouched sentinel 0xFFFF", isi)
	}
}

// TestSysConnect_noAccessRequested confirms a FAB with neither FAB$V_PUT
// nor FAB$V_GET set (FAB$B_FAC left at 0, after the file was already
// created some other way) is rejected rather than silently arming
// nothing and reporting success.
func TestSysConnect_noAccessRequested(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	putByte(t, f.ctx, testFabAddr+fabFAC, 0) // neither PUT nor GET
	putRAB(t, f.ctx, testFabAddr)

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	if r0 != rmsPrivilegeViolation {
		t.Errorf("r0 = %d, want rmsPrivilegeViolation (%d)", r0, rmsPrivilegeViolation)
	}
}

// TestSysConnect_reusesExistingWriter confirms a second SYS$CONNECT
// against a FAB that already has an armed Writer (real VMS allows more
// than one RAB to share a FAB — rab.go's own doc comment) reuses that
// same Writer instance rather than constructing a second, independent one
// over the same underlying file (which would let two write positions
// race over one linear file — see armForFAC's own doc comment).
func TestSysConnect_reusesExistingWriter(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)
	putRAB(t, f.ctx, testFabAddr)

	if _, err := SysConnect(f.ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("first SysConnect: %v", err)
	}

	h, _ := f.ctx.Files.Lookup(ifi)
	firstWriter := h.Writer

	// A second RAB, connecting to the same already-open file.
	const secondRabAddr = testRabAddr + 0x100
	
	putLongwordAt(t, f.ctx, secondRabAddr+rabFAB, testFabAddr)

	if _, err := SysConnect(f.ctx, []uint32{secondRabAddr}); err != nil {
		t.Fatalf("second SysConnect: %v", err)
	}

	if h.Writer != firstWriter {
		t.Error("second SysConnect replaced the already-armed Writer instead of reusing it")
	}
}

// TestSysConnect_readArming confirms SYS$CONNECT against a FAB whose
// FAB$B_FAC asks for GET access arms the FileHandle's Reader instead of
// its Writer. There's no SYS$OPEN yet to produce a read-only FileHandle
// through this package's own services (docs/PHASE-22.md's subtask 9), so
// this test builds one directly against the sibling ods2 module's own
// volume API — writing a small file, closing it, and reopening it fresh
// — exactly the kind of file SYS$OPEN will eventually hand SYS$CONNECT
// once it exists.
func TestSysConnect_readArming(t *testing.T) {
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

	written, err := vol.CreateFile(dir, "READTEST.DAT", ondisk.RecAttr{
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

	newFAB(t, f.ctx, "DUA0:READTEST.DAT")
	putWord(t, f.ctx, testFabAddr+fabIFI, ifi)
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)
	putRAB(t, f.ctx, testFabAddr)

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	h, _ := f.ctx.Files.Lookup(ifi)
	if h.Reader == nil {
		t.Fatal("handle.Reader = nil, want a Reader armed by SysConnect")
	}

	if h.Writer != nil {
		t.Error("handle.Writer is also set for a GET-only connection")
	}
}
