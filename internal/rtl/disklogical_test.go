package rtl

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// The block I/O function codes.
var (
	fnReadLBlk  = vmsdef.Symbols["IO$_READLBLK"]
	fnWriteLBlk = vmsdef.Symbols["IO$_WRITELBLK"]
	fnReadPBlk  = vmsdef.Symbols["IO$_READPBLK"]
	fnWritePBlk = vmsdef.Symbols["IO$_WRITEPBLK"]
	ssIllBlkNum = vmsdef.Symbols["SS$_ILLBLKNUM"]
)

// TestDiskLogical_readAndWrite reads the home block (LBN 1) logically and
// physically, and writes and reads back the boot block (LBN 0).
func TestDiskLogical_readAndWrite(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	buf := a.alloc(1024)

	for _, fn := range []uint32{fnReadLBlk, fnReadPBlk} {
		r0, st, n := diskQIO(t, env, a, ch, fn, buf, 512, 1)
		if r0 != ssNormal || st != ssNormal || n != 512 {
			t.Fatalf("read %#x of LBN 1 = %#x, IOSB %#x, %d bytes", fn, r0, st, n)
		}

		// HM2$T_FORMAT, at 496.
		if got := a.readString(buf+496, 12); got != "DECFILE11B  " {
			t.Errorf("read %#x: home block format %q", fn, got)
		}
	}

	text := a.str("Written logically")

	if _, st, n := diskQIO(t, env, a, ch, fnWriteLBlk, text, 17, 0); st != ssNormal || n != 17 {
		t.Fatalf("IO$_WRITELBLK: IOSB %#x, %d bytes", st, n)
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnReadLBlk, buf, 512, 0); st != ssNormal {
		t.Fatalf("reading it back: %#x", st)
	}

	got := readBytes(t, env, buf, 512)
	if !bytes.HasPrefix(got, []byte("Written logically\x00")) || got[511] != 0 {
		t.Errorf("LBN 0 = %q...", got[:20])
	}

	if _, st, _ := diskQIO(t, env, a, ch, fnWritePBlk, text, 17, 0); st != ssNormal {
		t.Errorf("IO$_WRITEPBLK: %#x", st)
	}
}

// TestDiskLogical_privileges: logical I/O takes LOG_IO or PHY_IO,
// physical I/O PHY_IO, checked in $QIO's R0.
func TestDiskLogical_privileges(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	buf := a.alloc(512)
	all := env.Process.CurrentPrivileges

	cases := []struct {
		name  string
		privs uint64
		fn    uint32
		want  uint32
	}{
		{"logical with PHY_IO only", all &^ privLOGIO, fnReadLBlk, ssNormal},
		{"logical with LOG_IO only", all &^ privPHYIO, fnReadLBlk, ssNormal},
		{"logical with neither", all &^ (privLOGIO | privPHYIO), fnReadLBlk, ssNoPriv},
		{"logical write with neither", all &^ (privLOGIO | privPHYIO), fnWriteLBlk, ssNoPriv},
		{"physical with LOG_IO only", all &^ privPHYIO, fnReadPBlk, ssNoPriv},
		{"physical with PHY_IO", all &^ privLOGIO, fnWritePBlk, ssNormal},
	}

	for _, tc := range cases {
		env.Process.CurrentPrivileges = tc.privs

		if r0, _, _ := diskQIO(t, env, a, ch, tc.fn, buf, 512, 0); r0 != tc.want {
			t.Errorf("%s: R0 %#x, want %#x", tc.name, r0, tc.want)
		}
	}
}

func TestDiskLogical_errors(t *testing.T) {
	env, a, ch := diskFixture(t, false)
	buf := a.alloc(1024)

	vol, _ := env.Mounts.Lookup("DUA0")
	end := vol.Devices[0].Container.Blocks()

	checks := []struct {
		name string
		fn   uint32
		p    []uint32
		r0   uint32
		st   uint32
	}{
		{"past the end", fnReadLBlk, []uint32{buf, 1024, end - 1}, ssNormal, ssIllBlkNum},
		{"a read-only volume", fnWriteLBlk, []uint32{buf, 512, 0}, ssNormal, vmsdef.Symbols["SS$_WRITLCK"]},
		{"an unwritable buffer", fnReadLBlk, []uint32{badAddr, 512, 0}, ssAccVio, 0},
		{"an unreadable buffer", fnWriteLBlk, []uint32{badAddr, 512, 0}, ssAccVio, 0},
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

	if _, st, _ := diskQIO(t, env, a, ch, fnReadLBlk, buf, 512, 1); uint32(st) != ssDevNotMnt {
		t.Errorf("not mounted: %#x", st)
	}
}
