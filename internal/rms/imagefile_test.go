package rms

import (
	"bytes"
	"errors"
	"testing"
)

// TestAccessImage: an image file the activator holds (Phase 66, subtask
// 2) reads its blocks as the page source, lets RMS readers in, keeps RMS
// and IO$_ACCESS writers out, holds its volume mounted, and lets go at
// Deaccess.
func TestAccessImage(t *testing.T) {
	p, mounts := newSharers(t, 1)
	a := p[0]

	recs := numbered("image", 40, 60) // a few blocks of records
	writeFile(t, mounts, "PROG.EXE", recs...)

	s := NewSession(mounts)

	want, _, err := s.ReadRawFile(FileLocation{Name: "DUA0:[000000]PROG.EXE"})
	if err != nil {
		t.Fatal(err)
	}

	img, data, found, err := s.AccessImage(FileLocation{Name: "DUA0:[000000]PROG.EXE"})
	if err != nil {
		t.Fatalf("AccessImage: %v", err)
	}

	if !bytes.Equal(data, want) {
		t.Errorf("AccessImage's bytes differ from ReadRawFile's (%d, %d)", len(data), len(want))
	}

	if found.Name == "" {
		t.Error("no location returned")
	}

	// Its blocks, as a section reads them.
	buf := make([]byte, 512)
	for vbn := uint32(1); vbn <= img.EndOfFileBlock(); vbn++ {
		if err := img.ReadPage(vbn, buf); err != nil {
			t.Fatalf("ReadPage(%d): %v", vbn, err)
		}

		end := min(int(vbn)*512, len(want))
		if !bytes.Equal(buf[:end-int(vbn-1)*512], want[(vbn-1)*512:end]) {
			t.Errorf("block %d differs", vbn)
		}
	}

	if img.Writable() {
		t.Error("the image file is accessed for writing")
	}

	// Readers may share it; writers may not.
	a.mustOpen("PROG.EXE", facGet, shrGet, false, 0)
	a.close()

	if r0 := a.open("PROG.EXE", facPut, shrGet|shrPut, false, ropEOF); r0 != rmsFileLocked {
		t.Errorf("RMS writer while the image is held: %#x, want RMS$_FLK", r0)
	}

	fid := img.FileID()
	if _, err := mounts.ACPAccess("DUA0", fid, true); !errors.Is(err, ErrACPAccessConflict) {
		t.Errorf("IO$_ACCESS for writing while the image is held: %v", err)
	}

	if err := mounts.Dismount("DUA0"); !errors.Is(err, ErrFilesOpen) {
		t.Errorf("Dismount while the image is held: %v", err)
	}

	if err := img.Deaccess(); err != nil {
		t.Fatal(err)
	}

	a.mustOpen("PROG.EXE", facPut, shrGet|shrPut, false, ropEOF)
	a.close()

	if err := mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after the image let go: %v", err)
	}
}

// TestAccessImage_notFound: a missing image is a *NotFoundError, as
// ReadRawFile's is, and a host location is refused.
func TestAccessImage_notFound(t *testing.T) {
	_, mounts := newSharers(t, 1)
	s := NewSession(mounts)

	_, _, _, err := s.AccessImage(FileLocation{Name: "DUA0:[000000]NONE.EXE"})

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("missing image: %v, want a NotFoundError", err)
	}

	if _, _, _, err := s.AccessImage(FileLocation{Name: "prog.exe", Host: true}); err == nil {
		t.Error("a host location was accessed")
	}
}
