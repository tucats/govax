package rtl

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// IO$_CREATE's function codes and the FIB fields the tests set.
var (
	fnCreate       = vmsdef.IOConstants["IO$_CREATE"]
	fnCreateNew    = fnCreate | vmsdef.IOConstants["IO$M_CREATE"]
	fnCreateAccess = fnCreateNew | vmsdef.IOConstants["IO$M_ACCESS"]
	fnAccessCreate = fnAccessOp | vmsdef.IOConstants["IO$M_CREATE"]
	ssCreatedTest  = vmsdef.SSConstants["SS$_CREATED"]
	ssDupFileName  = vmsdef.SSConstants["SS$_DUPFILENAME"]
)

// fibFileID reads the file ID stored in the FIB at fib.
func fibFileID(a *arena, fib uint32) rms.FileID {
	num := readWordEnv(a.t, a.env, fib+fibFID)
	seq := readWordEnv(a.t, a.env, fib+fibFID+2)

	return rms.FileID{Num: num, Seq: seq}
}

// TestDiskCreate_newFile creates NEW.DAT with two blocks and a record
// format, accessed for writing; writes both blocks; and checks what the
// FIB, the result name, and the file's header say.
func TestDiskCreate_newFile(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, fibMWrite)
	putWord(t, env, fib+fibEXCTL, uint16(fibMExtend))
	putLongword(t, env, fib+fibEXSZ, 2)

	// ATR$C_RECATTR: fixed-length (FAT$B_RTYPE 1), 512-byte records
	// (FAT$W_RSIZE, at 2).
	fat := a.alloc(32)
	putBytes(t, env, fat, []byte{1, 0, 0, 2})
	list := a.attrList(attrEntry{"RECATTR", 32, fat})

	resd, resbuf := a.outDesc(40)
	reslen := a.alloc(2)

	r0, st, _ := diskQIO(t, env, a, ch, fnCreateAccess, fibd, a.desc("new.dat"), reslen, resd, list)
	if r0 != ssNormal || st != ssNormal {
		t.Fatalf("IO$_CREATE = %#x, IOSB %#x", r0, st)
	}

	if n := uint16(a.readLong(reslen)); a.readString(resbuf, n) != "NEW.DAT;1" {
		t.Errorf("result name %q", a.readString(resbuf, n))
	}

	fid := fibFileID(a, fib)
	if found, _, _ := env.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, "NEW.DAT"); found != fid || fid.Num == 0 {
		t.Errorf("the FIB's file ID %v, the directory's %v", fid, found)
	}

	if a.readLong(fib+fibEXSZ) < 2 || a.readLong(fib+fibEXVBN) != 1 {
		t.Errorf("FIB$L_EXSZ %d, FIB$L_EXVBN %d", a.readLong(fib+fibEXSZ), a.readLong(fib+fibEXVBN))
	}

	// Accessed for writing: fill both blocks, and deaccess.
	buf := a.str(string(bytes.Repeat([]byte("Z"), 1024)))
	if _, st, _ := diskQIO(t, env, a, ch, fnWriteV, buf, 1024, 1); st != ssNormal {
		t.Fatalf("IO$_WRITEVBLK: %#x", st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnDeaccess); st != ssNormal {
		t.Fatalf("IO$_DEACCESS: %#x", st)
	}

	attrs, err := env.Mounts.ACPReadAttributes("DUA0", fid)
	if err != nil {
		t.Fatal(err)
	}

	if attrs.RecordAttributes[0] != 1 || attrs.RecordAttributes[3] != 2 {
		t.Errorf("record attributes % x: the list wasn't written", attrs.RecordAttributes[:4])
	}

	if blk, _ := fatEOF(attrs.RecordAttributes[:]); blk != 3 {
		t.Errorf("end of file block %d, want 3", blk)
	}

	if attrs.Owner != env.Process.UIC {
		t.Errorf("owner %#x, want the process's %#x", attrs.Owner, env.Process.UIC)
	}
}

// TestDiskCreate_versions: an explicit version already there, with and
// without FIB$M_NEWVER and FIB$M_SUPERSEDE.
func TestDiskCreate_versions(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, 0)
	name := a.desc("DATA.BIN;1")

	if _, st, _ := diskQIO(t, env, a, ch, fnCreateNew, fibd, name); uint32(st) != ssDupFileName {
		t.Errorf("duplicate: %#x", st)
	}

	resd, resbuf := a.outDesc(40)
	reslen := a.alloc(2)

	putWord(t, env, fib+fibNMCTL, uint16(fibMNewVer))
	if _, st, _ := diskQIO(t, env, a, ch, fnCreateNew, fibd, name, reslen, resd); st != ssNormal {
		t.Errorf("FIB$M_NEWVER: %#x", st)
	}

	if got := a.readString(resbuf, uint16(a.readLong(reslen))); got != "DATA.BIN;2" {
		t.Errorf("FIB$M_NEWVER made %q", got)
	}

	old := dataBinFIDVersion(t, env, "DATA.BIN;1")

	putWord(t, env, fib+fibNMCTL, uint16(fibMSupersede))
	if _, st, _ := diskQIO(t, env, a, ch, fnCreateNew, fibd, name); uint32(st) != ssSupersede {
		t.Errorf("FIB$M_SUPERSEDE: %#x", st)
	}

	if now := dataBinFIDVersion(t, env, "DATA.BIN;1"); now == old || now != fibFileID(a, fib) {
		t.Errorf("DATA.BIN;1 is %v (was %v), the FIB says %v", now, old, fibFileID(a, fib))
	}
}

func dataBinFIDVersion(t *testing.T, env *Environment, name string) rms.FileID {
	t.Helper()

	fid, _, err := env.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, name)
	if err != nil {
		t.Fatal(err)
	}

	return fid
}

// TestDiskCreate_noDirectoryAndEnter creates a file with no directory
// entry, then enters it under a name with IO$_CREATE alone.
func TestDiskCreate_noDirectoryAndEnter(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, 0)
	putWord(t, env, fib+fibDID, 0)
	putWord(t, env, fib+fibDID+2, 0)

	if _, st, _ := diskQIO(t, env, a, ch, fnCreateNew, fibd, a.desc("WORK.TMP")); st != ssNormal {
		t.Fatalf("IO$_CREATE with no directory: %#x", st)
	}

	fid := fibFileID(a, fib)
	if _, err := env.Mounts.ACPReadAttributes("DUA0", fid); err != nil || fid.Num == 0 {
		t.Fatalf("the new file %v: %v", fid, err)
	}

	if _, _, err := env.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, "WORK.TMP"); err == nil {
		t.Error("the file has a directory entry")
	}

	// Enter it in the MFD, and access it too.
	putWord(t, env, fib+fibDID, 4)
	putWord(t, env, fib+fibDID+2, 4)

	if _, st, _ := diskQIO(t, env, a, ch, fnCreate|vmsdef.IOConstants["IO$M_ACCESS"], fibd, a.desc("KEPT.DAT")); st != ssNormal {
		t.Fatalf("IO$_CREATE (enter): %#x", st)
	}

	if got := dataBinFIDVersion(t, env, "KEPT.DAT"); got != fid {
		t.Errorf("KEPT.DAT is %v, want %v", got, fid)
	}

	if c, _ := env.findChannel(ch); c.acp == nil || c.acp.FileID() != fid {
		t.Error("the entered file wasn't accessed")
	}
}

// TestDiskAccess_create: IO$_ACCESS!IO$M_CREATE creates a name it
// doesn't find (SS$_CREATED), and just accesses one it does.
func TestDiskAccess_create(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, fibMWrite)

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessCreate, fibd, a.desc("FRESH.DAT")); uint32(st) != ssCreatedTest {
		t.Fatalf("IO$_ACCESS!IO$M_CREATE, new: %#x", st)
	}

	if c, _ := env.findChannel(ch); c.acp == nil || !c.acp.Writable() || c.acp.FileID() != fibFileID(a, fib) {
		t.Error("the created file isn't accessed for writing")
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnDeaccess); st != ssNormal {
		t.Fatal(st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessCreate, fibd, a.desc("FRESH.DAT")); st != ssNormal {
		t.Errorf("IO$_ACCESS!IO$M_CREATE, existing: %#x", st)
	}
}

func TestDiskCreate_errors(t *testing.T) {
	env, a, ch := diskFixture(t, false)
	fibd, fib := newFIB(a, 64, 0)

	checks := []struct {
		name string
		fn   uint32
		p    []uint32
		r0   uint32
		st   uint32
	}{
		{"read-only volume", fnCreateNew, []uint32{fibd, a.desc("X.DAT")}, ssNormal, vmsdef.SSConstants["SS$_WRITLCK"]},
		{"directory, no name", fnCreateNew, []uint32{fibd}, ssNormal, ssBadParam},
		{"enter, no file ID", fnCreate, []uint32{fibd, a.desc("X.DAT")}, ssNormal, ssBadParam},
		{"entering, temporary", fnCreate | vmsdef.IOConstants["IO$M_DELETE"], []uint32{fibd}, ssIllIoFunc, 0},
		{"no FIB", fnCreateNew, []uint32{0}, ssAccVio, 0},
	}

	for _, ck := range checks {
		r0, st, _ := diskQIO(t, env, a, ch, ck.fn, ck.p...)
		if r0 != ck.r0 || (ck.st != 0 && uint32(st) != ck.st&0xFFFF) {
			t.Errorf("%s: R0 %#x IOSB %#x; want %#x, %#x", ck.name, r0, st, ck.r0, ck.st)
		}
	}

	// A file already accessed on the channel.
	putWord(t, env, fib+fibFID, dataBinFID(t, env).Num)
	putWord(t, env, fib+fibFID+2, dataBinFID(t, env).Seq)

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd); st != ssNormal {
		t.Fatal(st)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnCreateAccess, fibd, a.desc("Y.DAT")); st != uint16(ssFilAlrAcc) {
		t.Errorf("already accessed: %#x", st)
	}
}
