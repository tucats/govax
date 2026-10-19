package corevms

import (
	"bytes"
	"errors"
	"testing"
)

// TestZeroPastEnd: an image section reads a block past its file's last
// as zeros (docs/PHASE-66, question 4), where a $CRMPSC section's read
// fails; the file's own blocks read as they are, and other errors still
// fail.
func TestZeroPastEnd(t *testing.T) {
	data := bytes.Repeat([]byte{0x5A}, 600) // a block and part of another
	src := NewHostPageSource(bytes.NewReader(data), int64(len(data)), nil)

	plain := &processSection{file: src}
	image := &processSection{file: src, image: true}

	buf := bytes.Repeat([]byte{0xFF}, pageSize)

	if err := plain.source().ReadPage(3, buf); !errors.Is(err, errPastSource) {
		t.Errorf("a $CRMPSC section's block 3: %v, want errPastSource", err)
	}

	if err := image.source().ReadPage(3, buf); err != nil {
		t.Fatalf("an image section's block 3: %v", err)
	}

	if !bytes.Equal(buf, make([]byte, pageSize)) {
		t.Error("an image section's block past the file isn't zeros")
	}

	if err := image.source().ReadPage(2, buf); err != nil || buf[87] != 0x5A || buf[88] != 0 {
		t.Errorf("an image section's last block: %v, bytes %02X %02X, want 5A then zeros", err, buf[87], buf[88])
	}

	failing := &processSection{file: failingSource{}, image: true}
	if err := failing.source().ReadPage(1, buf); err == nil {
		t.Error("an image file's read error was taken for zeros")
	}
}

// failingSource is a page source whose every read fails.
type failingSource struct{}

func (failingSource) ReadPage(uint32, []byte) error { return errors.New("read failed") }

func (failingSource) Deaccess() error { return nil }
