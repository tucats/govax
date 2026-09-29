package rms

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/ods2/ondisk"
)

// mfd is the master file directory's file ID, (4,4,0).
var mfd = fileIDFrom(ondisk.MasterFileDirectoryFid)

// newACPFixture returns a MountTable with DUA0 mounted writable, holding
// DATA.TXT (500 bytes of text: one block) in its master file
// directory.
func newACPFixture(t *testing.T) *MountTable {
	t.Helper()

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "ACPVOL"), true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, _ := mounts.Lookup("DUA0")
	createTestFile(t, vol, "DATA.TXT", strings.Repeat("0123456789", 50))

	return mounts
}

func TestSplitACPName(t *testing.T) {
	cases := []struct {
		in, base string
		ver      uint16
		err      error
	}{
		{"data.txt", "DATA.TXT", 0, nil},
		{"DATA.TXT;3", "DATA.TXT", 3, nil},
		{"DATA.TXT.3", "DATA.TXT", 3, nil},
		{"NOTYPE", "NOTYPE.", 0, nil},
		{"", "", 0, ErrACPBadName},
		{"[DIR]X.Y", "", 0, ErrACPBadName},
		{"X.Y;abc", "", 0, ErrACPBadVersion},
		{"X.Y;40000", "", 0, ErrACPBadVersion},
	}

	for _, tc := range cases {
		base, ver, err := splitACPName(tc.in)
		if base != tc.base || ver != tc.ver || !errors.Is(err, tc.err) && err != tc.err {
			t.Errorf("splitACPName(%q) = %q, %d, %v; want %q, %d, %v", tc.in, base, ver, err, tc.base, tc.ver, tc.err)
		}
	}
}

func TestACPLookup(t *testing.T) {
	m := newACPFixture(t)

	fid, name, err := m.ACPLookup("DUA0:", mfd, "data.txt")
	if err != nil || fid.Num == 0 || name != "DATA.TXT;1" {
		t.Fatalf("ACPLookup = %v, %q, %v", fid, name, err)
	}

	checks := []struct {
		device string
		did    FileID
		name   string
		want   error
	}{
		{"DUA0", mfd, "NOSUCH.TXT", ErrACPNoSuchFile},
		{"DUA0", mfd, "DATA.TXT;9", ErrACPNoSuchFile},
		{"DUA1", mfd, "DATA.TXT", ErrACPNotMounted},
		{"DUA0", fid, "DATA.TXT", ErrACPBadDirectory}, // a file, not a directory
		{"DUA0", FileID{Num: 4, Seq: 99}, "DATA.TXT", ErrACPBadDirectory},
		{"DUA0", mfd, "*.TXT", ErrACPBadName},
	}

	for _, ck := range checks {
		if _, _, err := m.ACPLookup(ck.device, ck.did, ck.name); !errors.Is(err, ck.want) {
			t.Errorf("ACPLookup(%s, %v, %q) = %v, want %v", ck.device, ck.did, ck.name, err, ck.want)
		}
	}
}

func TestACPReadVirtual(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	a, err := m.ACPAccess("DUA0", fid, false)
	if err != nil {
		t.Fatal(err)
	}

	if a.Writable() || a.FileID() != fid || a.EndOfFileBlock() < 1 {
		t.Fatalf("accessed file: writable %v, fid %v, EOF block %d", a.Writable(), a.FileID(), a.EndOfFileBlock())
	}

	data, err := a.ReadVirtual(1, 20)
	if err != nil || !bytes.HasPrefix(data, []byte("0123456789")) || len(data) != 20 {
		t.Errorf("ReadVirtual(1, 20) = %q, %v", data, err)
	}

	// Past the end: whatever blocks there were, then end of file.
	eof := a.EndOfFileBlock()

	data, err = a.ReadVirtual(eof, 3*512)
	if !errors.Is(err, ErrACPEndOfFile) || len(data) != 512 {
		t.Errorf("reading across the end: %d bytes, %v; want one block, end of file", len(data), err)
	}

	if _, err := a.ReadVirtual(eof+1, 512); !errors.Is(err, ErrACPEndOfFile) {
		t.Errorf("reading past the end: %v", err)
	}

	if _, err := a.ReadVirtual(0, 512); !errors.Is(err, ErrACPBadBlock) {
		t.Errorf("VBN 0: %v", err)
	}

	if err := a.WriteVirtual(1, []byte("x")); !errors.Is(err, ErrACPReadOnly) {
		t.Errorf("writing a read-only access: %v", err)
	}

	if _, err := a.Extend(1); !errors.Is(err, ErrACPReadOnly) {
		t.Errorf("extending a read-only access: %v", err)
	}

	if err := a.Deaccess(); err != nil {
		t.Errorf("Deaccess: %v", err)
	}
}

func TestACPWriteVirtual(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	a, err := m.ACPAccess("DUA0", fid, true)
	if err != nil {
		t.Fatal(err)
	}

	allocated := a.AllocatedBlocks()

	// Overwrite block 1 with a short buffer: the rest of the block is 0.
	if err := a.WriteVirtual(1, []byte("HELLO")); err != nil {
		t.Fatal(err)
	}

	// Past the allocation: nothing written, end of file. Extending
	// makes room.
	if err := a.WriteVirtual(allocated+1, []byte("more")); !errors.Is(err, ErrACPEndOfFile) {
		t.Fatalf("writing past the allocation: %v", err)
	}

	first, err := a.Extend(2)
	if err != nil || first != allocated+1 || a.AllocatedBlocks() < allocated+2 {
		t.Fatalf("Extend(2) = %d, %v; allocated %d", first, err, a.AllocatedBlocks())
	}

	if err := a.WriteVirtual(first, []byte("more")); err != nil {
		t.Fatalf("writing the new block: %v", err)
	}

	if err := a.Deaccess(); err != nil {
		t.Fatal(err)
	}

	// Read it back through a new access: the end of file moved to the
	// highest block written.
	r, err := m.ACPAccess("DUA0", fid, false)
	if err != nil {
		t.Fatal(err)
	}

	block1, _ := r.ReadVirtual(1, 512)
	if !bytes.HasPrefix(block1, []byte("HELLO\x00\x00")) {
		t.Errorf("block 1 = %q...", block1[:8])
	}

	if r.EndOfFileBlock() != first {
		t.Errorf("end of file block %d, want %d", r.EndOfFileBlock(), first)
	}

	last, err := r.ReadVirtual(first, 512)
	if err != nil || !bytes.HasPrefix(last, []byte("more")) {
		t.Errorf("new block = %q, %v", last[:4], err)
	}
}

func TestACPAccess_errors(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	if _, err := m.ACPAccess("DUA1", fid, false); !errors.Is(err, ErrACPNotMounted) {
		t.Errorf("unmounted: %v", err)
	}

	if _, err := m.ACPAccess("DUA0", FileID{Num: fid.Num, Seq: fid.Seq + 1}, false); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("stale FID: %v", err)
	}

	ro := NewMountTable()
	if err := ro.Mount("DUA0", newTestVolumeFile(t, "ROVOL"), false); err != nil {
		t.Fatal(err)
	}

	if _, err := ro.ACPAccess("DUA0", mfd, true); !errors.Is(err, ErrACPWriteLocked) {
		t.Errorf("write access on a read-only mount: %v", err)
	}
}
