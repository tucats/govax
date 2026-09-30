package rms

import (
	"errors"
	"testing"

	"github.com/tucats/ods2/ondisk"
)

// gone reports whether the file with ID fid no longer exists.
func gone(m *MountTable, fid FileID) bool {
	_, err := m.ACPReadAttributes("DUA0", fid)

	return errors.Is(err, ErrACPNoSuchFile)
}

func TestACPDelete_entryAndFile(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	// The entry only: the file stays.
	got, err := m.ACPDelete("DUA0", ACPDeleteRequest{Directory: mfd, Name: "data.txt"})
	if err != nil || got.FID != fid || got.Name != file11FileName || got.Deferred {
		t.Fatalf("ACPDelete (entry) = %+v, %v", got, err)
	}

	if _, _, err := m.ACPLookup("DUA0", mfd, "DATA.TXT"); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("the entry is still there: %v", err)
	}

	if gone(m, fid) {
		t.Fatal("removing the entry deleted the file")
	}

	// Then the file, by its ID.
	if _, err := m.ACPDelete("DUA0", ACPDeleteRequest{FID: fid, DeleteFile: true}); err != nil {
		t.Fatalf("ACPDelete (by FID): %v", err)
	}

	if !gone(m, fid) {
		t.Error("the file is still there")
	}

	if _, err := m.ACPDelete("DUA0", ACPDeleteRequest{FID: fid, DeleteFile: true}); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("deleting it again: %v", err)
	}
}

func TestACPDelete_byNameWithFile(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	if _, err := m.ACPDelete("DUA0", ACPDeleteRequest{Directory: mfd, Name: file11FileName, DeleteFile: true}); err != nil {
		t.Fatal(err)
	}

	if !gone(m, fid) {
		t.Error("the file is still there")
	}

	// Its slot's next file gets a new file ID.
	created, err := m.ACPCreate("DUA0", ACPCreateRequest{Directory: mfd, Name: "NEXT.DAT"})
	if err != nil {
		t.Fatal(err)
	}

	if created.FID == fid {
		t.Errorf("the next file reused the deleted file's ID %v", fid)
	}
}

// TestACPDelete_whileAccessed: deleting an accessed file marks it; it's
// still readable on its channels, can't be accessed again, and goes at
// the last deaccess.
func TestACPDelete_whileAccessed(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	a1, _ := m.ACPAccess("DUA0", fid, false)
	a2, _ := m.ACPAccess("DUA0", fid, true)

	got, err := m.ACPDelete("DUA0", ACPDeleteRequest{Directory: mfd, Name: "DATA.TXT", DeleteFile: true})
	if err != nil || !got.Deferred {
		t.Fatalf("ACPDelete = %+v, %v; want deferred", got, err)
	}

	if _, _, err := m.ACPLookup("DUA0", mfd, "DATA.TXT"); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("the entry is still there: %v", err)
	}

	if _, err := m.ACPAccess("DUA0", fid, false); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("accessing a marked file: %v", err)
	}

	if b, err := a1.ReadVirtual(1, 10); err != nil || string(b) != "0123456789" {
		t.Errorf("reading the marked file: %q, %v", b, err)
	}

	if err := a1.Deaccess(); err != nil || gone(m, fid) {
		t.Fatalf("after the first deaccess: %v, gone %v", err, gone(m, fid))
	}

	if err := a2.Deaccess(); err != nil || !gone(m, fid) {
		t.Errorf("after the last deaccess: %v, gone %v", err, gone(m, fid))
	}

	if err := a2.Deaccess(); err != nil {
		t.Errorf("deaccessing twice: %v", err)
	}
}

// TestACPCreate_temporary: a temporary file goes, entry and all, when
// deaccessed; one never accessed goes at once.
func TestACPCreate_temporary(t *testing.T) {
	m := newACPFixture(t)

	got, err := m.ACPCreate("DUA0", ACPCreateRequest{Directory: mfd, Name: "WORK.TMP", Access: true, Write: true, Temporary: true})
	if err != nil {
		t.Fatal(err)
	}

	if gone(m, got.FID) {
		t.Fatal("the temporary file went while accessed")
	}

	if err := got.File.Deaccess(); err != nil {
		t.Fatal(err)
	}

	if !gone(m, got.FID) {
		t.Error("the temporary file is still there")
	}

	if _, _, err := m.ACPLookup("DUA0", mfd, "WORK.TMP"); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("its entry is still there: %v", err)
	}

	quick, err := m.ACPCreate("DUA0", ACPCreateRequest{Name: "GONE.TMP", Temporary: true})
	if err != nil || !gone(m, quick.FID) {
		t.Errorf("a temporary file never accessed: %v, gone %v", err, gone(m, quick.FID))
	}
}

func TestACPDelete_errors(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	// A subdirectory with an entry in it.
	vol, _ := m.Lookup("DUA0")
	root, _ := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	bm, ib, _ := deviceBitmaps(root.Device)

	sub, err := vol.CreateDirectory(root, "SUB.DIR", 0, bm, ib)
	if err != nil {
		t.Fatal(err)
	}

	if err := sub.Insert("INSIDE.DAT", 1, fid.toOds2(), bm, ib); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		device string
		req    ACPDeleteRequest
		want   error
	}{
		{"DUA1", ACPDeleteRequest{FID: fid, DeleteFile: true}, ErrACPNotMounted},
		{"DUA0", ACPDeleteRequest{Directory: mfd, Name: "NOSUCH.DAT"}, ErrACPNoSuchFile},
		{"DUA0", ACPDeleteRequest{Directory: mfd, Name: "*.DAT"}, ErrACPBadName},
		{"DUA0", ACPDeleteRequest{Directory: fid, Name: "X.DAT"}, ErrACPBadDirectory},
		{"DUA0", ACPDeleteRequest{FID: mfd, DeleteFile: true}, ErrACPProtected},
		{"DUA0", ACPDeleteRequest{Directory: mfd, Name: "000000.DIR"}, ErrACPProtected},
		{"DUA0", ACPDeleteRequest{Directory: mfd, Name: "SUB.DIR", DeleteFile: true}, ErrACPDirNotEmpty},
	}

	for _, tc := range cases {
		if _, err := m.ACPDelete(tc.device, tc.req); !errors.Is(err, tc.want) {
			t.Errorf("ACPDelete(%s, %+v) = %v, want %v", tc.device, tc.req, err, tc.want)
		}
	}

	if _, _, err := m.ACPLookup("DUA0", mfd, "SUB.DIR"); err != nil {
		t.Errorf("the refused delete removed SUB.DIR's entry: %v", err)
	}

	ro := NewMountTable()
	if err := ro.Mount("DUA0", newTestVolumeFile(t, "ROVOL"), false); err != nil {
		t.Fatal(err)
	}

	if _, err := ro.ACPDelete("DUA0", ACPDeleteRequest{FID: fid, DeleteFile: true}); !errors.Is(err, ErrACPWriteLocked) {
		t.Errorf("read-only mount: %v", err)
	}
}
