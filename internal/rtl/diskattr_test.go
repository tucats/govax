package rtl

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// attrEntry is one attribute list entry, for attrList.
type attrEntry struct {
	code string // the ATR$C_ name
	size uint16
	addr uint32
}

// attrList builds an attribute list in the arena, ended by a zero
// longword, returning its address.
func (a *arena) attrList(entries ...attrEntry) uint32 {
	list := a.alloc(uint32(8*len(entries) + 4))

	for i, e := range entries {
		at := list + uint32(8*i)
		putWord(a.t, a.env, at, e.size)
		putWord(a.t, a.env, at+2, uint16(atrCode("ATR$C_"+e.code)))
		putLongword(a.t, a.env, at+4, e.addr)
	}

	return list
}

// fatEOF decodes the end of file from a record attribute area ($FATDEF):
// FAT$L_EFBLK at offset 8, a "swapped" longword (high word first), and
// FAT$W_FFBYTE at 12.
func fatEOF(fat []byte) (block uint32, ffb uint16) {
	block = uint32(binary.LittleEndian.Uint16(fat[8:]))<<16 | uint32(binary.LittleEndian.Uint16(fat[10:]))

	return block, binary.LittleEndian.Uint16(fat[12:])
}

// dataBinFID looks DATA.BIN up, returning its file ID.
func dataBinFID(t *testing.T, env *Environment) rms.FileID {
	t.Helper()

	fid, _, err := env.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, "DATA.BIN")
	if err != nil {
		t.Fatal(err)
	}

	return fid
}

func TestDiskAttributes_readOnAccess(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, _ := newFIB(a, 64, 0)

	fat, uchar, name, hdr, back := a.alloc(32), a.alloc(4), a.alloc(20), a.alloc(512), a.alloc(6)
	list := a.attrList(
		attrEntry{"RECATTR", 32, fat},
		attrEntry{"UCHAR", 4, uchar},
		attrEntry{"ASCNAME", 20, name},
		attrEntry{"HEADER", 512, hdr},
		attrEntry{"BACKLINK", 6, back},
	)

	r0, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd, a.desc("DATA.BIN"), 0, 0, list)
	if r0 != ssNormal || st != ssNormal {
		t.Fatalf("IO$_ACCESS = %#x, IOSB %#x", r0, st)
	}

	// DATA.BIN is 1540 bytes: its data ends 4 bytes into block 4.
	if blk, ffb := fatEOF(readBytes(t, env, fat, 32)); blk != 4 || ffb != 4 {
		t.Errorf("end of file %d/%d, want 4/4", blk, ffb)
	}

	if got := a.readString(name, 20); got != "DATA.BIN;1          " {
		t.Errorf("ASCNAME %q", got)
	}

	if got := readBytes(t, env, back, 6); !bytes.Equal(got, []byte{4, 0, 4, 0, 0, 0}) {
		t.Errorf("BACKLINK % x, want the MFD's (4,4,0)", got)
	}

	fid := dataBinFID(t, env)
	if num := binary.LittleEndian.Uint16(readBytes(t, env, hdr+8, 2)); num != fid.Num {
		t.Errorf("the header block's FH2$W_FID_NUM is %d, want %d", num, fid.Num)
	}

	if a.readLong(uchar)&0x2000 != 0 {
		t.Error("UCHAR says DATA.BIN is a directory")
	}
}

// TestDiskAttributes_readByFID reads attributes without accessing the
// file (IO$_ACCESS alone, the FIB holding the file ID), with a short
// size and a long one.
func TestDiskAttributes_readByFID(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fid := dataBinFID(t, env)

	fibd, fib := newFIB(a, 64, 0)
	putWord(t, env, fib+4, fid.Num)
	putWord(t, env, fib+6, fid.Seq)

	short, long := a.alloc(2), a.alloc(40)
	putBytes(t, env, short, []byte{0xFF, 0xFF})
	putBytes(t, env, long, bytes.Repeat([]byte{0xFF}, 40))

	list := a.attrList(attrEntry{"RECATTR", 2, short}, attrEntry{"ASCNAME", 40, long})

	if r0, st, _ := diskQIO(t, env, a, ch, fnAccess, fibd, 0, 0, 0, list); r0 != ssNormal || st != ssNormal {
		t.Fatalf("IO$_ACCESS = %#x, IOSB %#x", r0, st)
	}

	// A short read takes the first bytes; a long one pads the name with
	// blanks. (DATA.BIN, copied in binary, has no record format: its
	// first bytes are 0, so they're poisoned first to see the store.)
	want, _ := env.Mounts.ACPReadAttributes("DUA0", fid)
	if got := readBytes(t, env, short, 2); !bytes.Equal(got, want.RecordAttributes[:2]) {
		t.Errorf("RECATTR's first bytes % x, want % x", got, want.RecordAttributes[:2])
	}

	if got := a.readString(long, 40); got != "DATA.BIN;1"+strings.Repeat(" ", 30) {
		t.Errorf("ASCNAME %q", got)
	}

	if c, _ := env.findChannel(ch); c.acp != nil {
		t.Error("reading attributes accessed the file")
	}
}

// TestDiskAttributes_writeOnDeaccess is what RMS does on close: write a
// block past the old end of file, then set the end of file, to the byte,
// with IO$_DEACCESS's attribute list.
func TestDiskAttributes_writeOnDeaccess(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, _ := newFIB(a, 64, fibMWrite)

	fat := a.alloc(32)
	readList := a.attrList(attrEntry{"RECATTR", 32, fat})

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd, a.desc("DATA.BIN"), 0, 0, readList); st != ssNormal {
		t.Fatalf("IO$_ACCESS: %#x", st)
	}

	// Block 4 now holds 20 bytes of data: FAT$L_EFBLK 4 (swapped: the
	// high word first), FAT$W_FFBYTE 20. Only the first 14 bytes of the
	// area are written; the rest stays as it was.
	buf := a.str("tail and some more..")
	if _, st, _ := diskQIO(t, env, a, ch, fnWriteV, buf, 20, 4); st != ssNormal {
		t.Fatalf("IO$_WRITEVBLK: %#x", st)
	}

	putBytes(t, env, fat+8, []byte{0, 0, 4, 0, 20, 0})
	writeList := a.attrList(attrEntry{"RECATTR", 14, fat})

	if r0, st, _ := diskQIO(t, env, a, ch, fnDeaccess, 0, 0, 0, 0, writeList); r0 != ssNormal || st != ssNormal {
		t.Fatalf("IO$_DEACCESS = %#x, IOSB %#x", r0, st)
	}

	attrs, err := env.Mounts.ACPReadAttributes("DUA0", dataBinFID(t, env))
	if err != nil {
		t.Fatal(err)
	}

	if blk, ffb := fatEOF(attrs.RecordAttributes[:]); blk != 4 || ffb != 20 {
		t.Errorf("end of file %d/%d, want 4/20", blk, ffb)
	}

	if got := readBytes(t, env, fat+14, 18); !bytes.Equal(attrs.RecordAttributes[14:], got) {
		t.Errorf("the unwritten part of the area changed: % x, was % x", attrs.RecordAttributes[14:], got)
	}
}

// TestDiskAttributes_modify writes attributes with IO$_MODIFY: by file ID
// with no file accessed, and to the accessed file.
func TestDiskAttributes_modify(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fid := dataBinFID(t, env)

	fibd, fib := newFIB(a, 64, fibMWrite)
	putWord(t, env, fib+4, fid.Num)
	putWord(t, env, fib+6, fid.Seq)

	prot := a.alloc(2)
	putWord(t, env, prot, 0xEE00)

	if _, st, _ := diskQIO(t, env, a, ch, fnModify, fibd, 0, 0, 0, a.attrList(attrEntry{"FPRO", 2, prot})); st != ssNormal {
		t.Fatalf("IO$_MODIFY by FID: %#x", st)
	}

	if attrs, _ := env.Mounts.ACPReadAttributes("DUA0", fid); attrs.Protection != 0xEE00 {
		t.Errorf("protection %#x, want 0xEE00", attrs.Protection)
	}

	// Access it (by the file ID); modify with no FIB.
	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd); st != ssNormal {
		t.Fatalf("IO$_ACCESS: %#x", st)
	}

	owner := a.long(0o300<<16 | 0o5)
	if _, st, _ := diskQIO(t, env, a, ch, fnModify, 0, 0, 0, 0, a.attrList(attrEntry{"UIC", 4, owner})); st != ssNormal {
		t.Fatalf("IO$_MODIFY on the accessed file: %#x", st)
	}

	if attrs, _ := env.Mounts.ACPReadAttributes("DUA0", fid); attrs.Owner != 0o300<<16|0o5 {
		t.Errorf("owner %#o", attrs.Owner)
	}
}

func TestDiskAttributes_errors(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fid := dataBinFID(t, env)

	fibd, fib := newFIB(a, 64, 0)
	putWord(t, env, fib+4, fid.Num)
	putWord(t, env, fib+6, fid.Seq)

	buf := a.alloc(512)

	cases := []struct {
		what   string
		fn     uint32
		list   uint32
		r0, st uint32
	}{
		{"an unknown code", fnAccess, a.attrList(attrEntry{"STATBLK", 10, buf}), ssNormal, ssBadAttrib},
		{"size zero", fnAccess, a.attrList(attrEntry{"UCHAR", 0, buf}), ssNormal, ssBadAttrib},
		{"past the maximum", fnAccess, a.attrList(attrEntry{"UCHAR", 5, buf}), ssNormal, ssBadAttrib},
		{"writing the header", fnModify, a.attrList(attrEntry{"HEADER", 512, buf}), ssNormal, ssBadAttrib},
		{"an inaccessible buffer", fnAccess, a.attrList(attrEntry{"UCHAR", 4, badAddr}), ssAccVio, 0},
		{"an inaccessible list", fnAccess, badAddr, ssAccVio, 0},
	}

	for _, tc := range cases {
		r0, st, _ := diskQIO(t, env, a, ch, tc.fn, fibd, 0, 0, 0, tc.list)
		if r0 != tc.r0 || r0 == ssNormal && uint32(st) != tc.st {
			t.Errorf("%s: R0 %#x, IOSB %#x; want %#x, %#x", tc.what, r0, st, tc.r0, tc.st)
		}
	}

	// Writing attributes on deaccess needs write access; the file is
	// closed anyway.
	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd); st != ssNormal {
		t.Fatalf("IO$_ACCESS: %#x", st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnDeaccess, 0, 0, 0, 0, a.attrList(attrEntry{"UCHAR", 4, buf})); st != uint16(ssNoPriv) {
		t.Errorf("deaccess with attributes, read access: %#x", st)
	}

	if c, _ := env.findChannel(ch); c.acp != nil {
		t.Error("the file is still accessed")
	}

	// IO$_MODIFY with no file accessed and nothing to write.
	if _, st, _ := diskQIO(t, env, a, ch, fnModify, fibd); st != uint16(ssFilNotAcc) {
		t.Errorf("modify with nothing: %#x", st)
	}
}
