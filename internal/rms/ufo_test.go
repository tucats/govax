package rms

import (
	"bytes"
	"errors"
	"testing"
)

// ufoChannels is a stand-in for internal/corevms's channel assignment:
// it records the files given to it and numbers them 8, 16, ..., or
// refuses with status if it's set.
type ufoChannels struct {
	files  []*ACPFile
	device []string
	status uint32
}

func (u *ufoChannels) assign(device string, f *ACPFile) (uint16, uint32) {
	if u.status != 0 {
		return 0, u.status
	}

	u.files = append(u.files, f)
	u.device = append(u.device, device)

	return uint16(8 * len(u.files)), 1
}

// TestUFO_create: $CREATE with FAB$V_UFO leaves the new file accessed on
// a channel, writable, its number in FAB$L_STV and FAB$W_IFI 0; the
// file's blocks can be written and read as pages, and $CLOSE has nothing
// to close.
func TestUFO_create(t *testing.T) {
	f := newCreateFixture(t, true)
	u := &ufoChannels{}
	f.ctx.AssignFileChannel = u.assign

	newFAB(t, f.ctx, "DUA0:MAP.DAT")
	putLongwordAt(t, f.ctx, testFabAddr+fabFOP, fopUFO)
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("ALQ"), 3)

	if st, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil || st != rmsNormal {
		t.Fatalf("SysCreate = %#x, %v", st, err)
	}

	if ifi := readWord(t, f.ctx, testFabAddr+fabIFI); ifi != 0 {
		t.Errorf("FAB$W_IFI = %d, want 0", ifi)
	}

	if stv := readLongword(t, f.ctx, testFabAddr+fabSTV); stv != 8 {
		t.Errorf("FAB$L_STV = %d, want the channel, 8", stv)
	}

	if len(u.files) != 1 || u.device[0] != "DUA0" {
		t.Fatalf("channels assigned: %v", u.device)
	}

	a := u.files[0]
	if !a.Writable() || a.AllocatedBlocks() < 3 {
		t.Errorf("the channel's file: writable %v, %d blocks; want writable, 3", a.Writable(), a.AllocatedBlocks())
	}

	page := bytes.Repeat([]byte{0x5A}, 512)
	if err := a.WritePage(3, page); err != nil {
		t.Fatalf("WritePage: %v", err)
	}

	got := make([]byte, 512)
	if err := a.ReadPage(3, got); err != nil || !bytes.Equal(got, page) {
		t.Errorf("ReadPage: %v, %x...", err, got[:4])
	}

	if err := a.ReadPage(a.AllocatedBlocks()+1, got); !errors.Is(err, ErrACPEndOfFile) {
		t.Errorf("ReadPage past the allocation: %v, want ErrACPEndOfFile", err)
	}

	// A new UFO file ends at the end of block ALQ (the RMS manual), and
	// a page write doesn't move that.
	if used := a.EndOfFileBlock(); used != 3 {
		t.Errorf("%d blocks used after a page write, want 3", used)
	}

	if st, _ := SysClose(f.ctx, []uint32{testFabAddr}); st != rmsInvalidIFI {
		t.Errorf("SysClose of the UFO FAB = %#x, want RMS$_IFI", st)
	}

	if err := a.Deaccess(); err != nil {
		t.Errorf("Deaccess: %v", err)
	}
}

// TestUFO_open: $OPEN with FAB$V_UFO, read-only, gives a read-only
// access; the open keeps its sharing (a writer is refused while it
// lasts) until the access ends.
func TestUFO_open(t *testing.T) {
	f := newCreateFixture(t, true)
	createAndCloseTestFile(t, f.ctx, "DATA.DAT", bytes.Repeat([]byte("R"), 80))

	u := &ufoChannels{}
	f.ctx.AssignFileChannel = u.assign

	newFAB(t, f.ctx, "DUA0:DATA.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)
	putLongwordAt(t, f.ctx, testFabAddr+fabFOP, fopUFO)

	if st, err := SysOpen(f.ctx, []uint32{testFabAddr}); err != nil || st != rmsNormal {
		t.Fatalf("SysOpen = %#x, %v", st, err)
	}

	if len(u.files) != 1 || u.files[0].Writable() {
		t.Fatalf("want one read-only access, got %d", len(u.files))
	}

	got := make([]byte, 512)
	if err := u.files[0].ReadPage(1, got); err != nil || got[0] != 'R' {
		t.Errorf("ReadPage: %v, %q", err, got[:4])
	}

	// A writer, with the default sharing, conflicts with the reader.
	newFAB(t, f.ctx, "DUA0:DATA.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facPut)
	putLongwordAt(t, f.ctx, testFabAddr+fabFOP, 0)

	if st, _ := SysOpen(f.ctx, []uint32{testFabAddr}); st != rmsFileLocked {
		t.Errorf("a writer while the UFO access lasts: %#x, want RMS$_FLK", st)
	}

	if err := u.files[0].Deaccess(); err != nil {
		t.Fatal(err)
	}

	if st, _ := SysOpen(f.ctx, []uint32{testFabAddr}); st != rmsNormal {
		t.Errorf("the writer after the access ended: %#x, want RMS$_NORMAL", st)
	}
}

// TestUFO_device: UFO of a device that holds no files (the terminal
// here) gives a channel to the device, with no file on it, its number in
// FAB$L_STV (VMS 7.3's answer for NLA0:; testdata/probe49, step 7f).
func TestUFO_device(t *testing.T) {
	f := newCreateFixture(t, true)

	newFAB(t, f.ctx, "TTA0:")
	putLongwordAt(t, f.ctx, testFabAddr+fabFOP, fopUFO)

	u := &ufoChannels{}
	f.ctx.AssignFileChannel = u.assign

	if st, _ := SysOpen(f.ctx, []uint32{testFabAddr}); st != rmsNormal {
		t.Fatalf("UFO of the terminal: %#x, want RMS$_NORMAL", st)
	}

	if stv := readLongword(t, f.ctx, testFabAddr+fabSTV); stv != 8 || len(u.files) != 1 || u.files[0] != nil || u.device[0] != "TTA0" {
		t.Errorf("FAB$L_STV %d, channels %v %q; want channel 8 to TTA0: with no file", stv, u.files, u.device)
	}
}

// TestUFO_refused: UFO with no way to assign a channel is RMS$_SUPPORT;
// a channel that can't be assigned is RMS$_CHN with the service's status
// in FAB$L_STV. Each time the file is closed again (a later exclusive
// open succeeds).
func TestUFO_refused(t *testing.T) {
	f := newCreateFixture(t, true)

	newFAB(t, f.ctx, "DUA0:A.DAT")
	putLongwordAt(t, f.ctx, testFabAddr+fabFOP, fopUFO)

	if st, _ := SysCreate(f.ctx, []uint32{testFabAddr}); st != rmsSupport {
		t.Errorf("UFO with no channels: %#x, want RMS$_SUPPORT", st)
	}

	const ssNoIOChan = 0x2C4 // any failure status

	f.ctx.AssignFileChannel = (&ufoChannels{status: ssNoIOChan}).assign

	newFAB(t, f.ctx, "DUA0:B.DAT")
	putLongwordAt(t, f.ctx, testFabAddr+fabFOP, fopUFO)

	if st, _ := SysCreate(f.ctx, []uint32{testFabAddr}); st != rmsChannel {
		t.Errorf("UFO whose channel fails: %#x, want RMS$_CHN", st)
	}

	if stv := readLongword(t, f.ctx, testFabAddr+fabSTV); stv != ssNoIOChan {
		t.Errorf("FAB$L_STV = %#x, want the assignment's status", stv)
	}

	// Both files were closed: each opens exclusively.
	for _, name := range []string{"DUA0:A.DAT", "DUA0:B.DAT"} {
		newFAB(t, f.ctx, name)
		putByte(t, f.ctx, testFabAddr+fabFAC, facPut)
		putLongwordAt(t, f.ctx, testFabAddr+fabFOP, 0)

		if st, _ := SysOpen(f.ctx, []uint32{testFabAddr}); st != rmsNormal {
			t.Errorf("%s after a refused UFO: %#x, want RMS$_NORMAL", name, st)
		}

		if st, _ := SysClose(f.ctx, []uint32{testFabAddr}); st != rmsNormal {
			t.Errorf("closing %s: %#x", name, st)
		}
	}
}
