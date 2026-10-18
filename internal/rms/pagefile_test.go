package rms

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ods2/ondisk"
)

// newPageFileSession returns a session with DUA0 mounted writable and
// the default on it.
func newPageFileSession(t *testing.T) *Session {
	t.Helper()

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "PAGEVOL"), true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	return s
}

// block returns a block filled with b.
func block(b byte) []byte { return bytes.Repeat([]byte{b}, ondisk.BlockSize) }

// roundTrip writes a block to the first and last of p's blocks, and reads
// them back.
func roundTrip(t *testing.T, p *PageFileStore) {
	t.Helper()

	for i, vbn := range []uint32{1, p.Blocks()} {
		want := block(byte(0x41 + i))
		if err := p.WriteBlock(vbn, want); err != nil {
			t.Fatalf("WriteBlock(%d): %v", vbn, err)
		}

		got := make([]byte, ondisk.BlockSize)
		if err := p.ReadBlock(vbn, got); err != nil {
			t.Fatalf("ReadBlock(%d): %v", vbn, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("block %d read back differently", vbn)
		}
	}
}

// neverGrows checks that p refuses blocks 0 and past its end, and that
// the refused write didn't change its size.
func neverGrows(t *testing.T, p *PageFileStore) {
	t.Helper()

	size := p.Blocks()

	for _, vbn := range []uint32{0, size + 1} {
		if err := p.WriteBlock(vbn, block(1)); !errors.Is(err, ErrACPEndOfFile) {
			t.Errorf("WriteBlock(%d) = %v, want ErrACPEndOfFile", vbn, err)
		}

		if err := p.ReadBlock(vbn, make([]byte, ondisk.BlockSize)); !errors.Is(err, ErrACPEndOfFile) {
			t.Errorf("ReadBlock(%d) = %v, want ErrACPEndOfFile", vbn, err)
		}
	}

	if p.Blocks() != size {
		t.Errorf("the page file grew from %d blocks to %d", size, p.Blocks())
	}
}

func TestPageFileHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pagefile.sys")
	s := NewSession(NewMountTable())

	p, err := s.OpenPageFile(`"`+path+`"`, 64)
	if err != nil {
		t.Fatal(err)
	}

	if !p.Host || !p.Created || p.Blocks() != 64 || p.Name != path {
		t.Errorf("opened %+v; want a new 64-block host file %s", p, path)
	}

	roundTrip(t, p)
	neverGrows(t, p)

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Stat(path); err != nil || info.Size() != 64*ondisk.BlockSize {
		t.Errorf("host file %v, %v; want 64 blocks", info, err)
	}

	// Opened again at its size, it's used as it is; with another size,
	// it's made that size, larger or smaller.
	for _, tc := range []struct{ size, resized uint32 }{{64, 0}, {200, 64}, {16, 200}} {
		p, err = s.OpenPageFile(path, tc.size)
		if err != nil {
			t.Fatal(err)
		}

		if p.Created || p.Blocks() != tc.size || p.Resized != tc.resized || p.GrowFailed != nil {
			t.Errorf("reopened at %d: %+v; want %d blocks, resized from %d", tc.size, p, tc.size, tc.resized)
		}

		if err := p.Close(); err != nil {
			t.Fatal(err)
		}

		if info, err := os.Stat(path); err != nil || info.Size() != int64(tc.size)*ondisk.BlockSize {
			t.Errorf("host file %v, %v; want %d blocks", info, err, tc.size)
		}
	}
}

func TestPageFileVolume(t *testing.T) {
	s := newPageFileSession(t)

	p, err := s.OpenPageFile("DUA0:[000000]PAGEFILE.SYS", 50)
	if err != nil {
		t.Fatal(err)
	}

	if p.Host || !p.Created || !p.Contiguous || p.Blocks() < 50 || p.Name != "DUA0:[000000]PAGEFILE.SYS;1" {
		t.Errorf("opened %+v; want a new contiguous volume file of at least 50 blocks", p)
	}

	roundTrip(t, p)
	neverGrows(t, p)

	// No one else may open it while it's the page file.
	fid, _, err := s.Mounts.ACPLookup("DUA0", mfd, "PAGEFILE.SYS")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Mounts.ACPAccess("DUA0", fid, false); !errors.Is(err, ErrACPAccessConflict) {
		t.Errorf("another access = %v, want ErrACPAccessConflict", err)
	}

	// Its volume can't be dismounted while it's open.
	// Its volume can't be dismounted while it's open, and DISMOUNT is told
	// it's a page file, not a user's.
	err = s.Mounts.Dismount("DUA0")

	var open *FilesOpenError
	if !errors.As(err, &open) || !errors.Is(err, ErrFilesOpen) {
		t.Errorf("Dismount = %v, want a FilesOpenError", err)
	} else if open.Device != "DUA0" || open.PageFiles != 1 || open.UserFiles != 0 {
		t.Errorf("Dismount's error %+v, want one page file and no user files on DUA0", open)
	}

	blocks := p.Blocks()

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	attr := mustAttributes(t, s.Mounts, fid)
	ra, _ := ondisk.DecodeRecAttr(attr.RecordAttributes[:])

	// VMS 7.1's page file's attributes (testdata/pg64).
	if ra.Format != ondisk.RecordFormatUndefined || ra.RecordSize != 0 || ra.EndOfFileBlock != blocks+1 {
		t.Errorf("record attributes %+v; want undefined records, end of file after block %d", ra, blocks)
	}

	if want := ondisk.FchNoBackup | fchNoMove; attr.Characteristics&want != want ||
		attr.Characteristics&(ondisk.FchContigB|ondisk.FchContig) != 0 {
		t.Errorf("characteristics %#x, want backups and moving disabled, not contiguous", attr.Characteristics)
	}

	if attr.Owner != 1<<16|4 || attr.Protection != 0xFF00 {
		t.Errorf("owner %#x protection %#x, want [1,4] and (S:RWED,O:RWED,G,W)", attr.Owner, attr.Protection)
	}

	// Opened again (by a name the default completes) at its size, the
	// same file, with what was written still there.
	p, err = s.OpenPageFile("PAGEFILE.SYS", 50)
	if err != nil {
		t.Fatal(err)
	}

	if p.Created || p.Blocks() != blocks || p.Resized != 0 {
		t.Errorf("reopened: %+v; want the existing %d blocks", p, blocks)
	}

	got := make([]byte, ondisk.BlockSize)
	if err := p.ReadBlock(1, got); err != nil || !bytes.Equal(got, block(0x41)) {
		t.Errorf("block 1 after reopening: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	// Closed, the volume can go.
	if err := s.Mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after Close = %v", err)
	}
}

// TestPageFileVolumeResize: a volume page file opened with a larger size
// is extended in place, keeping its version; with a smaller one it's
// deleted and made again at that size, with the same version; with a
// size the volume hasn't room to grow to, it's used at its own size, and
// the reason is given.
func TestPageFileVolumeResize(t *testing.T) {
	s := newPageFileSession(t)

	open := func(size uint32) *PageFileStore {
		t.Helper()

		p, err := s.OpenPageFile("DUA0:[000000]PAGEFILE.SYS", size)
		if err != nil {
			t.Fatal(err)
		}

		return p
	}

	closeIt := func(p *PageFileStore) {
		t.Helper()

		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}

	p := open(50)
	first := p.Blocks()
	closeIt(p)

	p = open(200)
	if p.Created || p.Resized != first || p.Blocks() < 200 || p.Name != "DUA0:[000000]PAGEFILE.SYS;1" {
		t.Errorf("grown: %+v; want ;1 extended from %d to at least 200 blocks", p, first)
	}

	roundTrip(t, p)

	grown := p.Blocks()
	closeIt(p)

	attr := mustAttributes(t, s.Mounts, mustLookup(t, s.Mounts, "PAGEFILE.SYS"))
	if ra, _ := ondisk.DecodeRecAttr(attr.RecordAttributes[:]); ra.EndOfFileBlock != grown+1 {
		t.Errorf("grown: end of file block %d, want %d", ra.EndOfFileBlock, grown+1)
	}

	p = open(20)
	if !p.Created || p.Resized != grown || p.Blocks() < 20 || p.Blocks() >= grown || p.Name != "DUA0:[000000]PAGEFILE.SYS;1" {
		t.Errorf("shrunk: %+v; want ;1 made again with about 20 blocks", p)
	}

	small := p.Blocks()
	closeIt(p)

	if _, _, err := s.Mounts.ACPLookup("DUA0", mfd, "PAGEFILE.SYS;2"); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("a second version after shrinking: %v", err)
	}

	p = open(100000)
	if p.Created || p.Resized != 0 || p.Blocks() != small || !errors.Is(p.GrowFailed, ErrACPDeviceFull) {
		t.Errorf("too big to grow: %+v; want the existing %d blocks and ErrACPDeviceFull", p, small)
	}

	closeIt(p)
}

// mustLookup returns the file ID of name in the volume's MFD.
func mustLookup(t *testing.T, mounts *MountTable, name string) FileID {
	t.Helper()

	fid, _, err := mounts.ACPLookup("DUA0", mfd, name)
	if err != nil {
		t.Fatal(err)
	}

	return fid
}

func TestPageFileVolumeErrors(t *testing.T) {
	s := newPageFileSession(t)

	for _, spec := range []string{"", "DUA0:[000000]*.SYS", "DUA0:[NOSUCH]PAGEFILE.SYS", "DUB0:[000000]PAGEFILE.SYS"} {
		if p, err := s.OpenPageFile(spec, 10); err == nil {
			_ = p.Close()

			t.Errorf("OpenPageFile(%q) succeeded", spec)
		}
	}

}

func TestPageFileTooBigLeavesNoFile(t *testing.T) {
	s := newPageFileSession(t)

	if p, err := s.OpenPageFile("DUA0:[000000]HUGE.SYS", 100000); err == nil {
		_ = p.Close()

		t.Fatal("a page file bigger than the volume was created")
	}

	if _, _, err := s.Mounts.ACPLookup("DUA0", mfd, "HUGE.SYS"); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("lookup of the refused file = %v, want ErrACPNoSuchFile", err)
	}
}
