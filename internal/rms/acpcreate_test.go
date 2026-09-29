package rms

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/ods2/ondisk"
)

func TestACPCreate_inDirectory(t *testing.T) {
	m := newACPFixture(t)

	got, err := m.ACPCreate("DUA0", ACPCreateRequest{
		Directory: mfd,
		Name:      "new.dat",
		Blocks:    3,
		Owner:     0o300<<16 | 0o7,
		Attributes: func(a *ACPAttributes) {
			copy(a.RecordAttributes[:], ondisk.EncodeRecAttr(ondisk.RecAttr{Format: ondisk.RecordFormatFixed, RecordSize: 512}))
		},
		Access: true,
		Write:  true,
	})
	if err != nil {
		t.Fatalf("ACPCreate: %v", err)
	}

	if got.Name != "NEW.DAT;1" || got.Blocks < 3 || got.File == nil || got.Superseded {
		t.Fatalf("ACPCreate = %+v", got)
	}

	if fid, _, err := m.ACPLookup("DUA0", mfd, "NEW.DAT"); err != nil || fid != got.FID {
		t.Errorf("lookup: %v, %v; want %v", fid, err, got.FID)
	}

	// The new file is accessed for writing: fill its three blocks.
	for vbn := uint32(1); vbn <= 3; vbn++ {
		if err := got.File.WriteVirtual(vbn, bytes.Repeat([]byte{byte('0' + vbn)}, 512)); err != nil {
			t.Fatalf("WriteVirtual(%d): %v", vbn, err)
		}
	}

	if err := got.File.Deaccess(); err != nil {
		t.Fatal(err)
	}

	a := mustAttributes(t, m, got.FID)
	if ra := recAttrOf(t, a); ra.Format != ondisk.RecordFormatFixed || ra.RecordSize != 512 || ra.EndOfFileBlock != 4 {
		t.Errorf("record attributes %+v", ra)
	}

	if a.Owner != 0o300<<16|0o7 || a.Backlink != mfd {
		t.Errorf("owner %#o, back link %v", a.Owner, a.Backlink)
	}

	r, _ := m.ACPAccess("DUA0", got.FID, false)
	if b, err := r.ReadVirtual(3, 512); err != nil || b[0] != '3' {
		t.Errorf("block 3: %q..., %v", b[:1], err)
	}
}

// TestACPCreate_versions: the next version, an explicit one, and the
// three ways an explicit version already there can go.
func TestACPCreate_versions(t *testing.T) {
	m := newACPFixture(t)
	create := func(name string, newVer, sup bool) (ACPCreated, error) {
		return m.ACPCreate("DUA0", ACPCreateRequest{Directory: mfd, Name: name, NewVersion: newVer, Supersede: sup})
	}

	if got, err := create("DATA.TXT", false, false); err != nil || got.Name != "DATA.TXT;2" {
		t.Errorf("next version: %+v, %v", got, err)
	}

	if got, err := create("DATA.TXT;7", false, false); err != nil || got.Name != "DATA.TXT;7" {
		t.Errorf("explicit version: %+v, %v", got, err)
	}

	if _, err := create("DATA.TXT;7", false, false); !errors.Is(err, ErrACPDuplicate) {
		t.Errorf("duplicate: %v", err)
	}

	if got, err := create("DATA.TXT;7", true, false); err != nil || got.Name != "DATA.TXT;8" {
		t.Errorf("FIB$M_NEWVER: %+v, %v", got, err)
	}

	old, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT;1")

	got, err := create("DATA.TXT;1", false, true)
	if err != nil || got.Name != "DATA.TXT;1" || !got.Superseded || got.FID == old {
		t.Fatalf("FIB$M_SUPERSEDE: %+v, %v", got, err)
	}

	if _, err := m.ACPReadAttributes("DUA0", old); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("the superseded file is still there: %v", err)
	}

	if _, err := create("LAST.DAT;32767", false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := create("LAST.DAT", false, false); !errors.Is(err, ErrACPBadVersion) {
		t.Errorf("past 32767: %v", err)
	}
}

// TestACPCreate_noDirectory makes a header with no directory entry: it's
// there by its file ID, and in no directory.
func TestACPCreate_noDirectory(t *testing.T) {
	m := newACPFixture(t)

	got, err := m.ACPCreate("DUA0", ACPCreateRequest{Name: "TEMP.TMP;4"})
	if err != nil || got.Name != "" || got.FID == (FileID{}) || got.File != nil {
		t.Fatalf("ACPCreate = %+v, %v", got, err)
	}

	a := mustAttributes(t, m, got.FID)
	if a.Name != "TEMP.TMP" {
		t.Errorf("header name %q", a.Name)
	}

	if _, _, err := m.ACPLookup("DUA0", mfd, "TEMP.TMP"); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("lookup: %v", err)
	}
}

func TestACPCreate_errors(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	cases := []struct {
		device string
		req    ACPCreateRequest
		want   error
	}{
		{"DUA1", ACPCreateRequest{Directory: mfd, Name: "A.B"}, ErrACPNotMounted},
		{"DUA0", ACPCreateRequest{Directory: fid, Name: "A.B"}, ErrACPBadDirectory},
		{"DUA0", ACPCreateRequest{Directory: mfd, Name: "*.B"}, ErrACPBadName},
		{"DUA0", ACPCreateRequest{Directory: mfd, Name: "A.B", Blocks: 1 << 30}, ErrACPDeviceFull},
	}

	for _, tc := range cases {
		if _, err := m.ACPCreate(tc.device, tc.req); !errors.Is(err, tc.want) {
			t.Errorf("ACPCreate(%s, %+v) = %v, want %v", tc.device, tc.req, err, tc.want)
		}
	}

	ro := NewMountTable()
	if err := ro.Mount("DUA0", newTestVolumeFile(t, "ROVOL"), false); err != nil {
		t.Fatal(err)
	}

	if _, err := ro.ACPCreate("DUA0", ACPCreateRequest{Directory: mfd, Name: "A.B"}); !errors.Is(err, ErrACPWriteLocked) {
		t.Errorf("read-only mount: %v", err)
	}
}

func TestACPEnter(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	name, err := m.ACPEnter("DUA0", mfd, "alias.txt", fid, false)
	if err != nil || name != "ALIAS.TXT;1" {
		t.Fatalf("ACPEnter = %q, %v", name, err)
	}

	if got, _, _ := m.ACPLookup("DUA0", mfd, "ALIAS.TXT"); got != fid {
		t.Errorf("ALIAS.TXT is %v, want DATA.TXT's %v", got, fid)
	}

	if _, err := m.ACPEnter("DUA0", mfd, "ALIAS.TXT;1", fid, false); !errors.Is(err, ErrACPDuplicate) {
		t.Errorf("duplicate: %v", err)
	}

	if name, err := m.ACPEnter("DUA0", mfd, "ALIAS.TXT;1", fid, true); err != nil || name != "ALIAS.TXT;2" {
		t.Errorf("new version: %q, %v", name, err)
	}

	if _, err := m.ACPEnter("DUA0", mfd, "GHOST.TXT", FileID{Num: fid.Num, Seq: fid.Seq + 1}, false); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("stale file ID: %v", err)
	}

	if _, err := m.ACPEnter("DUA0", fid, "X.Y", fid, false); !errors.Is(err, ErrACPBadDirectory) {
		t.Errorf("not a directory: %v", err)
	}
}
