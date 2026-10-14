package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

var (
	ssNoSuchFileTest  = vmsdef.Symbols["SS$_NOSUCHFILE"]
	ssDevNotMountTest = vmsdef.Symbols["SS$_DEVNOTMOUNT"]
)

// fidFixture is findFixture's volume (A.TXT, B.TXT, C.DAT in [000000],
// D.TXT in [SUB]) with [SUB.DEEP]E.TXT added, DUA0 defined as a disk,
// DUA1 a disk with nothing mounted, and NLA0 the null device.
func fidFixture(t *testing.T) *corevms.Environment {
	t.Helper()

	env := findFixture(t)

	for _, d := range []struct {
		name  string
		class iodev.DeviceClass
	}{{"DUA0", iodev.DeviceClassDisk}, {"DUA1", iodev.DeviceClassDisk}, {"NLA0", iodev.DeviceClassNone}} {
		env.Devices.Define(d.name, iodev.DeviceOptions{DevClass: d.class})
	}

	if st := call(t, env, "LIB$CREATE_DIR", newArena(t, env).desc("DUA0:[SUB.DEEP]")); st&1 == 0 {
		t.Fatalf("LIB$CREATE_DIR = %#x", st)
	}

	if _, err := rms.NewSession(env.Mounts).CreateRecordFile(rms.FileLocation{Name: "DUA0:[SUB.DEEP]E.TXT"}, rms.TextRecords, [][]byte{[]byte("x")}); err != nil {
		t.Fatal(err)
	}

	return env
}

// fileOf returns the directory entry of name in the directory dirs names
// on DUA0.
func fileOf(t *testing.T, env *corevms.Environment, name string, dirs ...string) ondisk.DirEntry {
	t.Helper()

	vol, _ := env.Mounts.Lookup("DUA0")

	d, err := filespec.ResolveDirectory(vol, dirs)
	if err != nil {
		t.Fatalf("ResolveDirectory(%v): %v", dirs, err)
	}

	e, err := d.Lookup(name, 0)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", name, err)
	}

	return e
}

// fidArg writes fid as $FIDDEF's three words and returns their address.
func (a *arena) fidArg(fid ondisk.Fid) uint32 {
	addr := a.next
	a.next += 8

	for i, w := range []uint16{fid.Num, fid.Seq, uint16(fid.Rvn) | uint16(fid.Nmx)<<8} {
		if err := a.env.Memory().StoreWord(a.env.CPU(), addr+uint32(2*i), w); err != nil {
			a.t.Fatal(err)
		}
	}

	return addr
}

// fidToName calls LIB$FID_TO_NAME for device and fid with a filespec of
// size characters, returning the status, the name (trailing blanks
// removed), filespec-length, and acp-status (each 0xEEEE... when the
// routine didn't write it).
func fidToName(t *testing.T, env *corevms.Environment, device string, fid ondisk.Fid, size int) (uint32, string, uint32, uint32) {
	t.Helper()

	a := newArena(t, env)
	a.next = 0x30000
	out := a.desc(strings.Repeat("?", size))
	length, acp := a.long(0xEEEEEEEE), a.long(0xEEEEEEEE)

	st := call(t, env, "LIB$FID_TO_NAME", a.desc(device), a.fidArg(fid), out, length, 0, acp)

	return st, strings.TrimRight(descText(t, env, out), " "), longword(t, env, length), longword(t, env, acp)
}

func TestFidToName(t *testing.T) {
	env := fidFixture(t)

	d, a, e := fileOf(t, env, "D.TXT", "SUB"), fileOf(t, env, "A.TXT"), fileOf(t, env, "E.TXT", "SUB", "DEEP")
	sub := fileOf(t, env, "SUB.DIR")

	for _, c := range []struct {
		device string
		fid    ondisk.Fid
		want   string
	}{
		{"DUA0:", d.Fid, "DISK$LIBCRD:[SUB]D.TXT;1"},
		{"_DUA0:", a.Fid, "DISK$LIBCRD:[000000]A.TXT;1"},
		{"dua0", e.Fid, "DISK$LIBCRD:[SUB.DEEP]E.TXT;1"},
		{"_GOVAX$DUA0", d.Fid, "DISK$LIBCRD:[SUB]D.TXT;1"},
		{"DUA0:", sub.Fid, "DISK$LIBCRD:[000000]SUB.DIR;1"},
		{"DUA0:", ondisk.MasterFileDirectoryFid, "DISK$LIBCRD:[000000]000000.DIR;1"},
	} {
		st, name, length, acp := fidToName(t, env, c.device, c.fid, 80)
		if st != ssNormal || name != c.want || length != 0xEEEE0000|uint32(len(c.want)) || acp != ssNormal {
			t.Errorf("%s %v: status %#x, %q (length %#x), acp-status %#x; want SS$_NORMAL, %q",
				c.device, c.fid, st, name, length, acp, c.want)
		}
	}

	// A short filespec: LIB$_STRTRU, the length the characters written.
	st, name, length, _ := fidToName(t, env, "DUA0:", d.Fid, 10)
	if st != libStrTru || name != "DISK$LIBCR" || length&0xFFFF != 10 {
		t.Errorf("short filespec: status %#x, %q, length %d", st, name, length&0xFFFF)
	}
}

func TestFidToNameFailures(t *testing.T) {
	env := fidFixture(t)
	d := fileOf(t, env, "D.TXT", "SUB")

	stale := d.Fid
	stale.Seq++

	// The ACP can't name the file: the device alone, with the ACP's
	// status as acp-status and the routine's.
	for _, c := range []struct {
		what string
		fid  ondisk.Fid
	}{
		{"a stale file ID", stale},
		{"a file ID out of range", ondisk.Fid{Num: 4000, Seq: 1}},
	} {
		st, name, length, acp := fidToName(t, env, "DUA0:", c.fid, 40)
		if st != ssNoSuchFileTest || name != "DISK$LIBCRD:" || length&0xFFFF != 12 || acp != ssNoSuchFileTest {
			t.Errorf("%s: status %#x, %q, length %#x, acp-status %#x; want SS$_NOSUCHFILE and the device",
				c.what, st, name, length, acp)
		}
	}

	// Not a disk: filespec-length 0 and acp-status SS$_NORMAL, filespec
	// unwritten.
	st, name, length, acp := fidToName(t, env, "NLA0:", d.Fid, 40)
	if st != libInvFilSpe || name != strings.Repeat("?", 40) || length != 0xEEEE0000 || acp != ssNormal {
		t.Errorf("not a disk: status %#x, %q, length %#x, acp-status %#x", st, name, length, acp)
	}

	for _, c := range []struct {
		what   string
		device string
		fid    ondisk.Fid
		want   uint32
	}{
		{"no such device", "XYZ7:", d.Fid, ssNoSuchDev},
		{"not mounted", "DUA1:", d.Fid, ssDevNotMountTest},
		{"a 65-character device name", strings.Repeat("D", 65), d.Fid, libInvArg},
	} {
		st, name, length, acp := fidToName(t, env, c.device, c.fid, 40)
		if st != c.want || name != strings.Repeat("?", 40) || length != 0xEEEEEEEE || acp != 0xEEEEEEEE {
			t.Errorf("%s: status %#x, %q, length %#x, acp-status %#x; want %#x and nothing written",
				c.what, st, name, length, acp, c.want)
		}
	}

	a := newArena(t, env)
	if st := call(t, env, "LIB$FID_TO_NAME", a.desc("DUA0:"), a.fidArg(d.Fid)); st != libInvArg {
		t.Errorf("two arguments: status %#x, want LIB$_INVARG", st)
	}

	// A file-id of 0 isn't refused as an argument: it is read (on the VAX
	// an access violation; here address 0 holds zeros, file number 0).
	if st := call(t, env, "LIB$FID_TO_NAME", a.desc("DUA0:"), 0, a.desc("    ")); st != ssNoSuchFileTest {
		t.Errorf("file-id 0: status %#x, want SS$_NOSUCHFILE", st)
	}
}

// TestFidToNameDirectoryID: a directory-id is where the file is looked
// for, in place of its back link; a directory without an entry for it is
// SS$_NOSUCHFILE, the device alone written.
func TestFidToNameDirectoryID(t *testing.T) {
	env := fidFixture(t)
	d, sub := fileOf(t, env, "D.TXT", "SUB"), fileOf(t, env, "SUB.DIR")

	for _, c := range []struct {
		what string
		did  ondisk.Fid
		st   uint32
		want string
	}{
		{"its own directory", sub.Fid, ssNormal, "DISK$LIBCRD:[SUB]D.TXT;1"},
		{"another directory", ondisk.MasterFileDirectoryFid, ssNoSuchFileTest, "DISK$LIBCRD:"},
		{"a directory ID of 0", ondisk.Fid{}, ssNoSuchFileTest, "DISK$LIBCRD:"},
	} {
		a := newArena(t, env)
		a.next = 0x30000
		out := a.desc(strings.Repeat(" ", 40))

		st := call(t, env, "LIB$FID_TO_NAME", a.desc("DUA0:"), a.fidArg(d.Fid), out, 0, a.fidArg(c.did))
		if name := strings.TrimRight(descText(t, env, out), " "); st != c.st || name != c.want {
			t.Errorf("%s: status %#x, %q; want %#x, %q", c.what, st, name, c.st, c.want)
		}
	}
}

// TestFidToNameBrokenBacklink names files whose back links don't reach
// the master file directory: the file's name alone, acp-status
// SS$_NOSUCHFILE, and the routine still succeeds. A file its directory
// has no entry for is "[]" and its name.
func TestFidToNameBrokenBacklink(t *testing.T) {
	env := fidFixture(t)
	vol, _ := env.Mounts.Lookup("DUA0")

	relink := func(fid, to ondisk.Fid) {
		f, err := vol.OpenFID(fid)
		if err != nil {
			t.Fatal(err)
		}

		if err := volume.UpdateHeader(f, func(h *ondisk.FileHeader, _ *ondisk.Ident) { h.Backlink = to }); err != nil {
			t.Fatal(err)
		}
	}

	// No back link: the name its header records.
	d := fileOf(t, env, "D.TXT", "SUB")
	relink(d.Fid, ondisk.Fid{})

	st, name, _, acp := fidToName(t, env, "DUA0:", d.Fid, 80)
	if st != ssNormal || !strings.HasPrefix(name, "DISK$LIBCRD:D.TXT") || acp != ssNoSuchFileTest {
		t.Errorf("no back link: status %#x, %q, acp-status %#x", st, name, acp)
	}

	// [SUB.DEEP]'s back link to a file that isn't a directory: E.TXT,
	// in it, is named by its entry but has no directory.
	deep, e := fileOf(t, env, "DEEP.DIR", "SUB"), fileOf(t, env, "E.TXT", "SUB", "DEEP")
	relink(deep.Fid, fileOf(t, env, "A.TXT").Fid)

	st, name, _, acp = fidToName(t, env, "DUA0:", e.Fid, 80)
	if st != ssNormal || name != "DISK$LIBCRD:E.TXT;1" || acp != ssNoSuchFileTest {
		t.Errorf("a broken chain: status %#x, %q, acp-status %#x", st, name, acp)
	}

	// A back link to a directory with no entry for the file.
	b := fileOf(t, env, "B.TXT")
	relink(b.Fid, fileOf(t, env, "SUB.DIR").Fid)

	st, name, _, acp = fidToName(t, env, "DUA0:", b.Fid, 80)
	if st != ssNormal || !strings.HasPrefix(name, "DISK$LIBCRD:[]B.TXT") || acp != ssNormal {
		t.Errorf("no entry: status %#x, %q, acp-status %#x", st, name, acp)
	}
}
