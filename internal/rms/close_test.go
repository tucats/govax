package rms

import (
	"bytes"
	"testing"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
)

// TestSysClose_console confirms SYS$CLOSE against a FAB open on the TTA0:
// console pseudo-device succeeds and frees the IFI, without needing (or
// attempting) any real file-close work.
func TestSysClose_console(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)

	r0, err := SysClose(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysClose: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if sts := readLongword(t, f.ctx, testFabAddr+fabSTS); sts != rmsNormal {
		t.Errorf("FAB$L_STS = %d, want rmsNormal (%d)", sts, rmsNormal)
	}

	if _, ok := f.ctx.Files.Lookup(ifi); ok {
		t.Errorf("Files.Lookup(%d) still found an entry after SysClose, want it released", ifi)
	}
}

// TestSysClose_diskWriter confirms SYS$CLOSE against a file that was
// written to (SYS$CREATE, SYS$CONNECT, one or more SYS$PUTs) actually
// finalizes the write — the record(s) are only guaranteed visible to a
// fresh reader once the file has been closed (see ods2's rms.Writer.Close
// doc comment) — and frees the IFI. This is put_test.go's own
// TestSysPut_diskFile scenario, but going through the real SysClose
// handler instead of reaching into the sibling ods2 module's Writer
// directly, since that manual step is exactly what this subtask replaces.
func TestSysClose_diskWriter(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	ifi := connectRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	record := make([]byte, 80)
	copy(record, "closed via SysClose")
	putRecord(t, f.ctx, testRabAddr, record)

	if r0, err := SysPut(f.ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("SysPut: %v", err)
	} else if r0 != rmsNormal {
		t.Fatalf("SysPut r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	handle, ok := f.ctx.Files.Lookup(ifi)
	if !ok {
		t.Fatalf("Files.Lookup(%d) = not found before SysClose", ifi)
	}

	fid := handle.File.Header.Fid

	r0, err := SysClose(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysClose: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if _, ok := f.ctx.Files.Lookup(ifi); ok {
		t.Errorf("Files.Lookup(%d) still found an entry after SysClose, want it released", ifi)
	}

	vol, _ := f.ctx.Mounts.Lookup("DUA0")

	reopened, err := vol.OpenFID(fid)
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

// TestSysClose_diskReader confirms SYS$CLOSE against a file that was only
// ever opened for reading (Writer nil, Reader set — see
// TestSysConnect_readArming for how this fixture shape is built elsewhere
// in this package) succeeds too: closeVolumeFile falls back to a plain
// *volume.File.Close, which ods2 documents as a harmless no-op for a file
// never armed for writing.
func TestSysClose_diskReader(t *testing.T) {
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

	connectRAB(t, f.ctx, testFabAddr)

	handle, ok := f.ctx.Files.Lookup(ifi)
	if !ok || handle.Reader == nil {
		t.Fatal("fixture setup did not arm a Reader as expected")
	}

	r0, err := SysClose(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysClose: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if _, ok := f.ctx.Files.Lookup(ifi); ok {
		t.Errorf("Files.Lookup(%d) still found an entry after SysClose, want it released", ifi)
	}
}

// TestSysClose_invalidIFI confirms SYS$CLOSE against a FAB whose
// FAB$W_IFI doesn't name a currently open file fails with RMS$_IFI rather
// than panicking on a nil FileHandle.
func TestSysClose_invalidIFI(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")
	// Deliberately never call SysCreate — fabIFI is whatever newFAB left
	// it at (zero, one of ifi.go's reserved-and-never-allocated slots),
	// so no handle is registered under it.

	r0, err := SysClose(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysClose: %v", err)
	}

	if r0 != rmsInvalidIFI {
		t.Errorf("r0 = %d, want rmsInvalidIFI (%d)", r0, rmsInvalidIFI)
	}
}

// TestSysClose_doubleClose confirms a second SYS$CLOSE against the same
// FAB (its FAB$W_IFI now naming an already-released slot) fails cleanly
// with RMS$_IFI rather than double-releasing or panicking.
func TestSysClose_doubleClose(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0, err := SysClose(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("first SysClose: %v", err)
	} else if r0 != rmsNormal {
		t.Fatalf("first SysClose r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	r0, err := SysClose(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("second SysClose: %v", err)
	}

	if r0 != rmsInvalidIFI {
		t.Errorf("second SysClose r0 = %d, want rmsInvalidIFI (%d)", r0, rmsInvalidIFI)
	}
}
