package rtl

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Disk function codes and FIB values for the tests.
var (
	fnAccess   = vmsdef.IOConstants["IO$_ACCESS"]
	fnAccessOp = vmsdef.IOConstants["IO$_ACCESS"] | vmsdef.IOConstants["IO$M_ACCESS"]
	fnDeaccess = vmsdef.IOConstants["IO$_DEACCESS"]
	fnModify   = vmsdef.IOConstants["IO$_MODIFY"]
	fnWriteV   = vmsdef.IOConstants["IO$_WRITEVBLK"]
)

// diskContent is DATA.BIN's contents: three blocks, each filled with its
// own letter, and a little more.
var diskContent = append(append(append(bytes.Repeat([]byte("A"), 512), bytes.Repeat([]byte("B"), 512)...),
	bytes.Repeat([]byte("C"), 512)...), []byte("tail")...)

// diskFixture returns an Environment with DUA0 mounted (writable) on a
// fresh volume holding DATA.BIN, and a channel assigned to it.
func diskFixture(t *testing.T, writable bool) (*Environment, *arena, uint32) {
	t.Helper()

	env, _ := fixture()
	dir := t.TempDir()
	image := filepath.Join(dir, "disk.dsk")

	if err := rms.InitializeContainer(image, 2000, "TESTVOL", 1, "RA81"); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	if err := env.Mounts.Mount("DUA0", image, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	host := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(host, diskContent, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := rms.NewSession(env.Mounts).Copy(host, true, "DUA0:[000000]DATA.BIN", false, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !writable {
		if err := env.Mounts.Dismount("DUA0"); err != nil {
			t.Fatal(err)
		}

		if err := env.Mounts.Mount("DUA0", image, false); err != nil {
			t.Fatal(err)
		}
	}

	defineTestDevice(env, "DUA0", iodev.DeviceClassDisk)
	a := newArena(t, env)

	return env, a, assignCall(t, env, a, "DUA0", uint32(vax.User))
}

// newFIB returns a FIB (and its descriptor) of size bytes, with acctl
// and the directory ID set to the master file directory's (4,4,0).
func newFIB(a *arena, size uint16, acctl uint32) (desc, addr uint32) {
	addr = a.alloc(uint32(size))
	putLongword(a.t, a.env, addr, acctl)

	if size >= 16 {
		putWord(a.t, a.env, addr+10, 4)
		putWord(a.t, a.env, addr+12, 4)
	}

	desc = a.alloc(8)
	putWord(a.t, a.env, desc, size)
	putLongword(a.t, a.env, desc+4, addr)

	return desc, addr
}

// diskQIO issues a disk $QIO and returns R0 and the IOSB's status and
// count.
func diskQIO(t *testing.T, env *Environment, a *arena, ch, fn uint32, p ...uint32) (uint32, uint16, uint16) {
	t.Helper()

	iosb := a.alloc(8)
	q := qioArgs{channel: ch, function: fn, iosb: iosb}
	copy(q.p[:], p)

	r0 := callQIO(t, env, q)
	st, n, _ := readIOSB(a, iosb)

	return r0, st, n
}

func TestDiskAccessAndRead(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, 0)
	resd, resbuf := a.outDesc(40)
	reslen := a.alloc(2)

	// Look the name up, and access the file.
	r0, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd, a.desc("data.bin"), reslen, resd)
	if r0 != ssNormal || st != ssNormal {
		t.Fatalf("IO$_ACCESS = %#x, IOSB %#x", r0, st)
	}

	if n := uint16(a.readLong(reslen)); a.readString(resbuf, n) != "DATA.BIN;1" {
		t.Errorf("result name %q", a.readString(resbuf, n))
	}

	if num := uint16(a.readLong(fib + 4)); num == 0 {
		t.Error("the FIB's file ID wasn't filled in")
	}

	// Read two blocks from VBN 2.
	buf := a.alloc(1024)

	r0, st, n := diskQIO(t, env, a, ch, fnReadVBlk, buf, 1024, 2)
	if r0 != ssNormal || st != ssNormal || n != 1024 {
		t.Fatalf("READVBLK = %#x, IOSB %#x, %d bytes", r0, st, n)
	}

	got := readBytes(t, env, buf, 1024)
	if !bytes.Equal(got, diskContent[512:1536]) {
		t.Errorf("blocks 2-3 = %q...%q", got[:4], got[1020:])
	}

	// Across the end of file: what's there, then SS$_ENDOFFILE.
	_, st, n = diskQIO(t, env, a, ch, fnReadVBlk, buf, 1024, 4)
	if st != uint16(ssEndOfFile) || n != 512 || !bytes.HasPrefix(readBytes(t, env, buf, 4), []byte("tail")) {
		t.Errorf("across the end: IOSB %#x, %d bytes", st, n)
	}

	// Accessing again on the same channel: SS$_FILALRACC. A write to a
	// file accessed for reading: SS$_NOPRIV.
	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd); st != uint16(ssFilAlrAcc) {
		t.Errorf("second access: %#x", st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnWriteV, buf, 512, 1); st != uint16(ssNoPriv) {
		t.Errorf("write without write access: %#x", st)
	}

	// Deaccess; then there's nothing to read.
	if _, st, _ := diskQIO(t, env, a, ch, fnDeaccess); st != ssNormal {
		t.Errorf("IO$_DEACCESS: %#x", st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnReadVBlk, buf, 512, 1); st != uint16(ssFilNotAcc) {
		t.Errorf("read after deaccess: %#x", st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnDeaccess); st != uint16(ssFilNotAcc) {
		t.Errorf("deaccess twice: %#x", st)
	}
}

func TestDiskLookupOnly(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 22, 0)

	// Without IO$M_ACCESS: the file ID comes back, nothing is opened.
	if _, st, _ := diskQIO(t, env, a, ch, fnAccess, fibd, a.desc("DATA.BIN")); st != ssNormal {
		t.Fatalf("lookup: %#x", st)
	}

	c, _ := env.findChannel(ch)
	if c.acp != nil || uint16(a.readLong(fib+4)) == 0 {
		t.Fatal("a lookup should fill in the FID and open nothing")
	}

	// Access by that file ID alone (no name).
	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd); st != ssNormal || c.acp == nil {
		t.Errorf("access by FID: %#x", st)
	}

	// $DASSGN deaccesses.
	a2 := c.acp
	
	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	if c.acp != nil || a2 == nil {
		t.Error("$DASSGN should deaccess the file")
	}
}

func TestDiskWriteAndExtend(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, vmsdef.FIBConstants["FIB$M_WRITE"])

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd, a.desc("DATA.BIN")); st != ssNormal {
		t.Fatalf("access for write: %#x", st)
	}

	c, _ := env.findChannel(ch)
	allocated := c.acp.AllocatedBlocks()

	// Overwrite block 1.
	if _, st, n := diskQIO(t, env, a, ch, fnWriteV, a.str("HELLO"), 5, 1); st != ssNormal || n != 5 {
		t.Fatalf("write: %#x, %d", st, n)
	}

	// Past the allocation: SS$_ENDOFFILE. IO$_MODIFY extends.
	if _, st, _ := diskQIO(t, env, a, ch, fnWriteV, a.str("X"), 1, allocated+1); st != uint16(ssEndOfFile) {
		t.Errorf("write past the allocation: %#x", st)
	}

	putWord(t, env, fib+0x16, uint16(vmsdef.FIBConstants["FIB$M_EXTEND"]))
	putLongword(t, env, fib+0x18, 3)

	if _, st, _ := diskQIO(t, env, a, ch, fnModify, fibd); st != ssNormal {
		t.Fatalf("IO$_MODIFY: %#x", st)
	}

	if got := a.readLong(fib + 0x1C); got != allocated+1 {
		t.Errorf("FIB$L_EXVBN %d, want %d", got, allocated+1)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnWriteV, a.str("NEW"), 3, allocated+1); st != ssNormal {
		t.Errorf("writing the new block: %#x", st)
	}

	diskQIO(t, env, a, ch, fnDeaccess)

	// Read back.
	fibr, _ := newFIB(a, 64, 0)
	diskQIO(t, env, a, ch, fnAccessOp, fibr, a.desc("DATA.BIN"))

	buf := a.alloc(512)
	diskQIO(t, env, a, ch, fnReadVBlk, buf, 512, 1)

	if got := readBytes(t, env, buf, 6); !bytes.Equal(got, []byte("HELLO\x00")) {
		t.Errorf("block 1 = %q", got)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnReadVBlk, buf, 512, allocated+1); st != ssNormal || !bytes.HasPrefix(readBytes(t, env, buf, 3), []byte("NEW")) {
		t.Errorf("new block: %#x", st)
	}
}

func TestDisk_errors(t *testing.T) {
	env, a, ch := diskFixture(t, false)
	fibd, fib := newFIB(a, 64, vmsdef.FIBConstants["FIB$M_WRITE"])

	checks := []struct {
		name string
		fn   uint32
		p    []uint32
		r0   uint32
		st   uint32
	}{
		{"no such file", fnAccessOp, []uint32{fibd, a.desc("NOSUCH.DAT")}, ssNormal, vmsdef.SSConstants["SS$_NOSUCHFILE"]},
		{"bad name", fnAccessOp, []uint32{fibd, a.desc("[X]Y.Z")}, ssNormal, vmsdef.SSConstants["SS$_BADFILENAME"]},
		{"write access to a read-only volume", fnAccessOp, []uint32{fibd, a.desc("DATA.BIN")}, ssNormal, vmsdef.SSConstants["SS$_WRITLCK"]},
		{"no FIB", fnAccessOp, []uint32{0}, ssAccVio, 0},
		{"IO$M_DELETE on IO$_ACCESS", fnAccessOp | vmsdef.IOConstants["IO$M_DELETE"], []uint32{fibd}, ssIllIoFunc, 0},
	}

	for _, ck := range checks {
		r0, st, _ := diskQIO(t, env, a, ch, ck.fn, ck.p...)
		if r0 != ck.r0 || (ck.st != 0 && uint32(st) != ck.st&0xFFFF) {
			t.Errorf("%s: R0 %#x IOSB %#x; want %#x, %#x", ck.name, r0, st, ck.r0, ck.st)
		}
	}

	// A FIB too short to hold a file ID; a zero file ID with no name.
	short, _ := newFIB(a, 8, 0)
	if r0, _, _ := diskQIO(t, env, a, ch, fnAccessOp, short); r0 != ssAccVio {
		t.Errorf("short FIB: %#x", r0)
	}

	for off := uint32(0); off < 16; off += 2 { // no access flags, file ID, or directory ID
		putWord(t, env, fib+off, 0)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd); st != ssBadParam {
		t.Errorf("no file ID: %#x", st)
	}

	// Not mounted.
	if err := env.Mounts.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	fib2, _ := newFIB(a, 64, 0)
	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fib2, a.desc("DATA.BIN")); uint32(st) != vmsdef.SSConstants["SS$_DEVNOTMOUNT"] {
		t.Errorf("not mounted: %#x", st)
	}
}
