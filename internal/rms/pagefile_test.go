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

	// Opened again with another size, it's used at its own.
	p, err = s.OpenPageFile(path, 200)
	if err != nil {
		t.Fatal(err)
	}

	defer p.Close()

	if p.Created || p.Blocks() != 64 {
		t.Errorf("reopened: created %v, %d blocks; want the existing 64", p.Created, p.Blocks())
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

	if ra.Format != ondisk.RecordFormatFixed || ra.RecordSize != ondisk.BlockSize || ra.EndOfFileBlock != blocks+1 {
		t.Errorf("record attributes %+v; want fixed 512-byte records, end of file after block %d", ra, blocks)
	}

	if want := ondisk.FchContigB | ondisk.FchContig; attr.Characteristics&want != want {
		t.Errorf("characteristics %#x, want contiguous-best-try and contiguous", attr.Characteristics)
	}

	// Opened again (by a name the default completes), the same file at
	// its own size, with what was written still there.
	p, err = s.OpenPageFile("PAGEFILE.SYS", 10)
	if err != nil {
		t.Fatal(err)
	}

	if p.Created || p.Blocks() != blocks {
		t.Errorf("reopened: created %v, %d blocks; want the existing %d", p.Created, p.Blocks(), blocks)
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
