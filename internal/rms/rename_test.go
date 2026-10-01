package rms

import (
	"testing"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// Where SYS$RENAME's tests lay out their second FAB and its name, clear
// of create_test.go's testFabAddr/testFnaAddr (the old FAB here) and
// connect_test.go's testRabAddr.
const (
	testNewFabAddr = 0x3000
	testNewFnaAddr = 0x3100
)

// renameFixture is a createFixture whose DUA0: volume, the container at
// path, has two empty subdirectories, [A] and [B].
type renameFixture struct {
	*createFixture
	path string
	vol  *volume.Volume
	mfd  *volume.Directory
	a, b *volume.Directory
	bm   *volume.Bitmap
	ib   *volume.IndexBitmap
}

func newRenameFixture(t *testing.T) renameFixture {
	t.Helper()

	f := newCreateFixture(t, true)
	path := newTestVolumeFile(t, "RENAME")

	// newCreateFixture's own DUA0: doesn't say where its container is,
	// which remountReadOnly needs; replace it.
	if err := f.ctx.Mounts.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	if err := f.ctx.Mounts.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	vol, _ := f.ctx.Mounts.Lookup("DUA0")

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatal(err)
	}

	bm, ib, err := deviceBitmaps(mfd.Device)
	if err != nil {
		t.Fatal(err)
	}

	fx := renameFixture{createFixture: f, path: path, vol: vol, mfd: mfd, bm: bm, ib: ib}

	if fx.a, err = vol.CreateDirectory(mfd, "A.DIR", volume.DirectoryOptions{}, bm, ib); err != nil {
		t.Fatal(err)
	}

	if fx.b, err = vol.CreateDirectory(mfd, "B.DIR", volume.DirectoryOptions{}, bm, ib); err != nil {
		t.Fatal(err)
	}

	return fx
}

// remountReadOnly dismounts DUA0: and mounts the same container again
// read-only.
func (fx *renameFixture) remountReadOnly(t *testing.T) {
	t.Helper()

	if err := fx.ctx.Mounts.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	if err := fx.ctx.Mounts.Mount("DUA0", fx.path, false); err != nil {
		t.Fatal(err)
	}

	fx.vol, _ = fx.ctx.Mounts.Lookup("DUA0")
}

// create makes name;version (0: the next) in dir and returns its FID.
func (fx renameFixture) create(t *testing.T, dir *volume.Directory, name string, version uint16) ondisk.Fid {
	t.Helper()

	f, err := fx.vol.CreateFileVersion(dir, name, version, ondisk.RecAttr{}, fx.bm, fx.ib)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	return f.Header.Fid
}

// lookup returns the FID dirPath's name;version (0: the highest) names,
// and whether it exists, reading the volume afresh.
func (fx renameFixture) lookup(t *testing.T, dirs []string, name string, version uint16) (ondisk.Fid, bool) {
	t.Helper()

	vol, _ := fx.ctx.Mounts.Lookup("DUA0")

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range dirs {
		e, err := dir.Lookup(d+".DIR", 1)
		if err != nil {
			return ondisk.Fid{}, false
		}

		if dir, err = vol.OpenDirectory(e.Fid); err != nil {
			t.Fatal(err)
		}
	}

	e, err := dir.Lookup(name, version)
	if err != nil {
		return ondisk.Fid{}, false
	}

	return e.Fid, true
}

// putFAB writes a minimal, valid FAB at fabAddr whose file name, spec,
// is stored at fnaAddr.
func putFAB(t *testing.T, ctx *Context, fabAddr, fnaAddr uint32, spec string) {
	t.Helper()

	for i := 0; i < len(spec); i++ {
		putByte(t, ctx, fnaAddr+uint32(i), spec[i])
	}

	putByte(t, ctx, fabAddr+fabBID, fabBIDValue)
	putByte(t, ctx, fabAddr+fabBLN, fabBLNValue)
	putLongwordAt(t, ctx, fabAddr+fabFNA, fnaAddr)
	putByte(t, ctx, fabAddr+fabFNS, byte(len(spec)))
}

// rename calls SYS$RENAME to rename oldSpec to newSpec, with garbage in
// both FABs' STS/STV beforehand, and returns R0 and the old FAB's STS and
// STV -- checking that R0 and STS agree and that the new FAB's were
// cleared.
func (fx renameFixture) rename(t *testing.T, oldSpec, newSpec string) (sts, stv uint32) {
	t.Helper()

	ctx := fx.ctx

	putFAB(t, ctx, testFabAddr, testFnaAddr, oldSpec)
	putFAB(t, ctx, testNewFabAddr, testNewFnaAddr, newSpec)

	for _, a := range []uint32{testFabAddr + fabSTS, testFabAddr + fabSTV, testNewFabAddr + fabSTS, testNewFabAddr + fabSTV} {
		putLongwordAt(t, ctx, a, 0xDEADBEEF)
	}

	r0, err := SysRename(ctx, []uint32{testFabAddr, 0, 0, testNewFabAddr})
	if err != nil {
		t.Fatalf("SysRename(%s, %s): %v", oldSpec, newSpec, err)
	}

	sts = readLongword(t, ctx, testFabAddr+fabSTS)
	stv = readLongword(t, ctx, testFabAddr+fabSTV)

	if r0 != sts {
		t.Errorf("R0 = %#x but the old FAB's STS = %#x", r0, sts)
	}

	if got := readLongword(t, ctx, testNewFabAddr+fabSTS); got != 0 {
		t.Errorf("the new FAB's STS = %#x, want it cleared", got)
	}

	return sts, stv
}

// wantStatus fails unless sts (and, when stv isn't nil, the STV) are as
// wanted.
func wantStatus(t *testing.T, what string, sts, stv, wantSTS uint32, wantSTV ...uint32) {
	t.Helper()

	if sts != wantSTS {
		t.Errorf("%s: STS = %#x, want %#x", what, sts, wantSTS)
	}

	if len(wantSTV) > 0 && stv != wantSTV[0] {
		t.Errorf("%s: STV = %#x, want %#x", what, stv, wantSTV[0])
	}
}

// TestSysRename_renamesAndMoves renames a file within its directory, then
// moves it to another one, then gives it an explicit version.
func TestSysRename_renamesAndMoves(t *testing.T) {
	fx := newRenameFixture(t)
	fid := fx.create(t, fx.a, "OLD.TXT", 3)

	sts, stv := fx.rename(t, "DUA0:[A]OLD.TXT", "DUA0:[A]NEW.DAT")
	wantStatus(t, "rename", sts, stv, rmsNormal, 0)

	if _, ok := fx.lookup(t, []string{"A"}, "OLD.TXT", 0); ok {
		t.Error("[A]OLD.TXT still exists")
	}

	// No new version: the next one, ;1 for a new name.
	if got, ok := fx.lookup(t, []string{"A"}, "NEW.DAT", 1); !ok || got != fid {
		t.Errorf("[A]NEW.DAT;1 = %v, %v; want %v", got, ok, fid)
	}

	sts, stv = fx.rename(t, "DUA0:[A]NEW.DAT;1", "DUA0:[B]MOVED.DAT;42")
	wantStatus(t, "move", sts, stv, rmsNormal, 0)

	if got, ok := fx.lookup(t, []string{"B"}, "MOVED.DAT", 42); !ok || got != fid {
		t.Errorf("[B]MOVED.DAT;42 = %v, %v; want %v", got, ok, fid)
	}

	if _, ok := fx.lookup(t, []string{"A"}, "NEW.DAT", 0); ok {
		t.Error("[A]NEW.DAT still exists")
	}

	// The header follows: its back link is [B].
	f, err := fx.vol.OpenFID(fid)
	if err != nil {
		t.Fatal(err)
	}

	if f.Header.Backlink != fx.b.Header.Fid {
		t.Errorf("back link = %v, want [B] %v", f.Header.Backlink, fx.b.Header.Fid)
	}
}

// TestSysRename_nextVersion: with no new version, a name that exists gets
// its next one.
func TestSysRename_nextVersion(t *testing.T) {
	fx := newRenameFixture(t)
	fid := fx.create(t, fx.a, "X.TXT", 0)
	fx.create(t, fx.b, "X.TXT", 6)

	sts, stv := fx.rename(t, "DUA0:[A]X.TXT", "DUA0:[B]X.TXT")
	wantStatus(t, "rename", sts, stv, rmsNormal)

	if got, ok := fx.lookup(t, []string{"B"}, "X.TXT", 7); !ok || got != fid {
		t.Errorf("[B]X.TXT;7 = %v, %v; want %v", got, ok, fid)
	}
}

// TestSysRename_existingVersionRefused: $RENAME never replaces a file.
func TestSysRename_existingVersionRefused(t *testing.T) {
	fx := newRenameFixture(t)
	a := fx.create(t, fx.a, "X.TXT", 0)
	b := fx.create(t, fx.b, "Y.TXT", 2)

	sts, stv := fx.rename(t, "DUA0:[A]X.TXT", "DUA0:[B]Y.TXT;2")
	wantStatus(t, "onto Y.TXT;2", sts, stv, rmsEnterFailed, ssDuplicateFileName)

	if got, ok := fx.lookup(t, []string{"A"}, "X.TXT", 1); !ok || got != a {
		t.Errorf("[A]X.TXT;1 = %v, %v; want it untouched (%v)", got, ok, a)
	}

	if got, ok := fx.lookup(t, []string{"B"}, "Y.TXT", 2); !ok || got != b {
		t.Errorf("[B]Y.TXT;2 = %v, %v; want it untouched (%v)", got, ok, b)
	}
}

// TestSysRename_directories: a directory moves with its contents, but not
// into itself.
func TestSysRename_directories(t *testing.T) {
	fx := newRenameFixture(t)
	inside := fx.create(t, fx.b, "IN.TXT", 0)

	sts, stv := fx.rename(t, "DUA0:[000000]A.DIR;1", "DUA0:[A]A.DIR;1")
	wantStatus(t, "[A] into itself", sts, stv, rmsInvalidDirRename)

	sts, stv = fx.rename(t, "DUA0:[000000]B.DIR;1", "DUA0:[A]B.DIR;1")
	wantStatus(t, "[B] into [A]", sts, stv, rmsNormal)

	if got, ok := fx.lookup(t, []string{"A", "B"}, "IN.TXT", 0); !ok || got != inside {
		t.Errorf("[A.B]IN.TXT = %v, %v; want %v", got, ok, inside)
	}

	sts, stv = fx.rename(t, "DUA0:[000000]A.DIR;1", "DUA0:[A.B]A.DIR;1")
	wantStatus(t, "[A] into [A.B]", sts, stv, rmsInvalidDirRename)
}

// TestSysRename_sameDeviceThroughLogicalName: "the same device" is judged
// after translation.
func TestSysRename_sameDeviceThroughLogicalName(t *testing.T) {
	fx := newRenameFixture(t)
	fid := fx.create(t, fx.a, "X.TXT", 0)
	defineTestLogical(t, fx.ctx.Logicals, "MYDISK", "DUA0:")

	sts, stv := fx.rename(t, "DUA0:[A]X.TXT", "MYDISK:[B]X.TXT")
	wantStatus(t, "via MYDISK:", sts, stv, rmsNormal)

	if got, ok := fx.lookup(t, []string{"B"}, "X.TXT", 1); !ok || got != fid {
		t.Errorf("[B]X.TXT;1 = %v, %v; want %v", got, ok, fid)
	}
}

// TestSysRename_otherDeviceRefused: a rename can't leave its volume,
// whether the other device is mounted or not.
func TestSysRename_otherDeviceRefused(t *testing.T) {
	fx := newRenameFixture(t)
	fid := fx.create(t, fx.a, "X.TXT", 0)

	if err := fx.ctx.Mounts.Mount("DUA1", newTestVolumeFile(t, "OTHER"), true); err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{"DUA1:[000000]X.TXT", "DUA2:[000000]X.TXT"} {
		sts, stv := fx.rename(t, "DUA0:[A]X.TXT", target)
		wantStatus(t, target, sts, stv, rmsDeviceError)
	}

	if got, ok := fx.lookup(t, []string{"A"}, "X.TXT", 1); !ok || got != fid {
		t.Errorf("[A]X.TXT;1 = %v, %v; want it untouched", got, ok)
	}
}

// TestSysRename_parseAndLookupFailures covers the checks made before
// anything changes, each with its own RMS status.
func TestSysRename_parseAndLookupFailures(t *testing.T) {
	fx := newRenameFixture(t)
	fx.create(t, fx.a, "X.TXT", 0)

	for _, tc := range []struct {
		what, oldSpec, newSpec string
		want                   uint32
	}{
		{"a wildcard in the old name", "DUA0:[A]*.TXT", "DUA0:[A]Y.TXT", rmsWildcardError},
		{"a wildcard in the new name", "DUA0:[A]X.TXT", "DUA0:[A]Y.%XT", rmsWildcardError},
		{"a wildcard version", "DUA0:[A]X.TXT;*", "DUA0:[A]Y.TXT", rmsWildcardError},
		{"a wildcard directory", "DUA0:[A...]X.TXT", "DUA0:[A]Y.TXT", rmsWildcardError},
		{"the terminal", "TTA0:", "DUA0:[A]Y.TXT", rmsInvalidOperation},
		{"a missing file", "DUA0:[A]NOPE.TXT", "DUA0:[A]Y.TXT", rmsFileNotFound},
		{"a missing version", "DUA0:[A]X.TXT;9", "DUA0:[A]Y.TXT", rmsFileNotFound},
		{"a missing old directory", "DUA0:[NOPE]X.TXT", "DUA0:[A]Y.TXT", rmsDirNotFound},
		{"a missing new directory", "DUA0:[A]X.TXT", "DUA0:[NOPE]Y.TXT", rmsDirNotFound},
		{"an unmounted old device", "DUA3:[A]X.TXT", "DUA0:[A]Y.TXT", rmsDeviceNotReady},
		{"no new name", "DUA0:[A]X.TXT", "DUA0:[B]", rmsFileNameError},
		{"a new version over 32767", "DUA0:[A]X.TXT", "DUA0:[A]Y.TXT;32768", rmsInvalidVersion},
	} {
		sts, stv := fx.rename(t, tc.oldSpec, tc.newSpec)
		wantStatus(t, tc.what, sts, stv, tc.want)
	}

	if _, ok := fx.lookup(t, []string{"A"}, "X.TXT", 1); !ok {
		t.Error("[A]X.TXT;1 is gone after only failed renames")
	}
}

// TestSysRename_readOnlyVolume: the removal fails, write-locked.
func TestSysRename_readOnlyVolume(t *testing.T) {
	fx := newRenameFixture(t)
	fx.create(t, fx.a, "X.TXT", 0)
	fx.remountReadOnly(t)

	sts, stv := fx.rename(t, "DUA0:[A]X.TXT", "DUA0:[A]Y.TXT")
	wantStatus(t, "read-only", sts, stv, rmsRemoveFailed, ssWriteLocked)

	if _, ok := fx.lookup(t, []string{"A"}, "X.TXT", 1); !ok {
		t.Error("[A]X.TXT;1 is gone")
	}
}

// TestSysRename_badFABs: a block that isn't a FAB, or is too short, is
// refused before anything else; a bad new FAB's status goes in the old
// FAB.
func TestSysRename_badFABs(t *testing.T) {
	fx := newRenameFixture(t)
	ctx := fx.ctx

	putFAB(t, ctx, testFabAddr, testFnaAddr, "DUA0:[A]X.TXT")
	putFAB(t, ctx, testNewFabAddr, testNewFnaAddr, "DUA0:[A]Y.TXT")
	putByte(t, ctx, testNewFabAddr+fabBID, 0)

	r0, err := SysRename(ctx, []uint32{testFabAddr, 0, 0, testNewFabAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != rmsInvalidFAB || readLongword(t, ctx, testFabAddr+fabSTS) != rmsInvalidFAB {
		t.Errorf("a new FAB with a bad BID: R0 = %#x, old STS = %#x; want RMS$_FAB",
			r0, readLongword(t, ctx, testFabAddr+fabSTS))
	}

	putFAB(t, ctx, testNewFabAddr, testNewFnaAddr, "DUA0:[A]Y.TXT")
	putByte(t, ctx, testFabAddr+fabBLN, fabBLNValue-1)

	if r0, _ := SysRename(ctx, []uint32{testFabAddr, 0, 0, testNewFabAddr}); r0 != rmsInvalidBLN {
		t.Errorf("an old FAB too short: R0 = %#x, want RMS$_BLN", r0)
	}

	if r0, _ := SysRename(ctx, []uint32{testFabAddr}); r0 != ssInsufficientArgs {
		t.Errorf("one argument: R0 = %#x, want SS$_INSFARG", r0)
	}
}
