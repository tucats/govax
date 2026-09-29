package rtl

import (
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// IO$_DELETE's function codes.
var (
	fnDelete     = vmsdef.IOConstants["IO$_DELETE"]
	fnDeleteFile = fnDelete | vmsdef.IOConstants["IO$M_DELETE"]
)

// fileGone reports whether the file with ID fid no longer exists.
func fileGone(env *Environment, fid rms.FileID) bool {
	_, err := env.Mounts.ACPReadAttributes("DUA0", fid)

	return err != nil
}

// TestDiskDelete_entryThenFile removes DATA.BIN's entry (the FIB gets its
// file ID, p4 the entry's name), then deletes the file by that ID.
func TestDiskDelete_entryThenFile(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fid := dataBinFID(t, env)
	fibd, fib := newFIB(a, 64, 0)
	resd, resbuf := a.outDesc(40)
	reslen := a.alloc(2)

	if r0, st, _ := diskQIO(t, env, a, ch, fnDelete, fibd, a.desc("DATA.BIN"), reslen, resd); r0 != ssNormal || st != ssNormal {
		t.Fatalf("IO$_DELETE = %#x, IOSB %#x", r0, st)
	}

	if got := a.readString(resbuf, uint16(a.readLong(reslen))); got != "DATA.BIN;1" {
		t.Errorf("result name %q", got)
	}

	if fibFileID(a, fib) != fid {
		t.Errorf("the FIB's file ID %v, want %v", fibFileID(a, fib), fid)
	}

	if _, _, err := env.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, "DATA.BIN"); err == nil {
		t.Error("the entry is still there")
	}

	if fileGone(env, fid) {
		t.Fatal("removing the entry deleted the file")
	}

	// Now the file itself, by the ID in the FIB (no directory).
	putWord(t, env, fib+fibDID, 0)

	if _, st, _ := diskQIO(t, env, a, ch, fnDeleteFile, fibd); st != ssNormal {
		t.Fatalf("IO$_DELETE!IO$M_DELETE by file ID: %#x", st)
	}

	if !fileGone(env, fid) {
		t.Error("the file is still there")
	}
}

// TestDiskDelete_whileAccessed deletes DATA.BIN from a second channel
// while the first has it accessed: it's still readable there, can't be
// accessed on the second, and goes when the first is deassigned.
func TestDiskDelete_whileAccessed(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fid := dataBinFID(t, env)
	fibd, _ := newFIB(a, 64, 0)

	if _, st, _ := diskQIO(t, env, a, ch, fnAccessOp, fibd, a.desc("DATA.BIN")); st != ssNormal {
		t.Fatalf("IO$_ACCESS: %#x", st)
	}

	ch2 := assignCall(t, env, a, "DUA0", uint32(vax.User))
	fib2d, fib2 := newFIB(a, 64, 0)

	if _, st, _ := diskQIO(t, env, a, ch2, fnDeleteFile, fib2d, a.desc("DATA.BIN")); st != ssNormal {
		t.Fatalf("IO$_DELETE!IO$M_DELETE: %#x", st)
	}

	buf := a.alloc(512)
	if _, st, n := diskQIO(t, env, a, ch, fnReadVBlk, buf, 512, 1); st != ssNormal || n != 512 {
		t.Errorf("reading the marked file: %#x, %d bytes", st, n)
	}

	putWord(t, env, fib2+fibDID, 0)

	if _, st, _ := diskQIO(t, env, a, ch2, fnAccessOp, fib2d); uint32(st) != vmsdef.SSConstants["SS$_NOSUCHFILE"] {
		t.Errorf("accessing the marked file: %#x", st)
	}

	if fileGone(env, fid) {
		t.Fatal("the file went while accessed")
	}

	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	if !fileGone(env, fid) {
		t.Error("the file is still there after $DASSGN")
	}
}

// TestDiskCreate_temporary: IO$_CREATE!IO$M_CREATE!IO$M_ACCESS!IO$M_DELETE
// makes a file that goes, with its entry, at IO$_DEACCESS.
func TestDiskCreate_temporary(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, fib := newFIB(a, 64, fibMWrite)

	if _, st, _ := diskQIO(t, env, a, ch, fnCreateAccess|vmsdef.IOConstants["IO$M_DELETE"], fibd, a.desc("SCRATCH.TMP")); st != ssNormal {
		t.Fatalf("IO$_CREATE (temporary): %#x", st)
	}

	fid := fibFileID(a, fib)
	if fileGone(env, fid) {
		t.Fatal("the temporary file went while accessed")
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnDeaccess); st != ssNormal {
		t.Fatalf("IO$_DEACCESS: %#x", st)
	}

	if !fileGone(env, fid) {
		t.Error("the temporary file is still there")
	}

	if _, _, err := env.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, "SCRATCH.TMP"); err == nil {
		t.Error("its entry is still there")
	}
}

func TestDiskDelete_errors(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, _ := newFIB(a, 64, 0)
	noDir, noDirFIB := newFIB(a, 64, 0)
	putWord(t, env, noDirFIB+fibDID, 0)

	mfdFIBd, mfdFIB := newFIB(a, 64, 0)
	putWord(t, env, mfdFIB+fibDID, 0)
	putWord(t, env, mfdFIB+fibFID, 4)
	putWord(t, env, mfdFIB+fibFID+2, 4)

	checks := []struct {
		name string
		fn   uint32
		p    []uint32
		r0   uint32
		st   uint32
	}{
		{"no such file", fnDelete, []uint32{fibd, a.desc("NOSUCH.DAT")}, ssNormal, vmsdef.SSConstants["SS$_NOSUCHFILE"]},
		{"no entry, no IO$M_DELETE", fnDelete, []uint32{fibd}, ssNormal, ssBadParam},
		{"no entry, no file ID", fnDeleteFile, []uint32{noDir}, ssNormal, ssBadParam},
		{"the MFD", fnDeleteFile, []uint32{mfdFIBd}, ssNormal, ssNoPriv},
		{"the MFD's entry", fnDelete, []uint32{fibd, a.desc("000000.DIR")}, ssNormal, ssNoPriv},
		{"no FIB", fnDelete, []uint32{0}, ssAccVio, 0},
	}

	for _, ck := range checks {
		r0, st, _ := diskQIO(t, env, a, ch, ck.fn, ck.p...)
		if r0 != ck.r0 || (ck.st != 0 && uint32(st) != ck.st&0xFFFF) {
			t.Errorf("%s: R0 %#x IOSB %#x; want %#x, %#x", ck.name, r0, st, ck.r0, ck.st)
		}
	}

	if err := env.Mounts.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnDelete, fibd, a.desc("DATA.BIN")); uint32(st) != ssDevNotMnt {
		t.Errorf("not mounted: %#x", st)
	}
}
