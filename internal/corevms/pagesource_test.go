package corevms

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vm"
)

// countingSource is a page source that counts its deaccesses; its block
// n is filled with byte n.
type countingSource struct {
	deaccesses int
}

func (c *countingSource) ReadPage(vbn uint32, buf []byte) error {
	for i := range buf[:pageSize] {
		buf[i] = byte(vbn)
	}

	return nil
}

func (c *countingSource) Deaccess() error {
	c.deaccesses++

	return nil
}

// closeCounter counts Close calls.
type closeCounter struct{ closes int }

func (c *closeCounter) Close() error {
	c.closes++

	return nil
}

// TestHostPageSource: a host file's blocks, the last filled out with
// zeros, nothing past it, and Deaccess closing it once.
func TestHostPageSource(t *testing.T) {
	data := make([]byte, 2*pageSize+100)
	for i := range data {
		data[i] = byte(i/pageSize + 1)
	}

	closer := &closeCounter{}
	h := NewHostPageSource(bytes.NewReader(data), int64(len(data)), closer)

	if h.Blocks() != 3 {
		t.Fatalf("Blocks() = %d, want 3", h.Blocks())
	}

	buf := make([]byte, pageSize)

	for vbn := uint32(1); vbn <= 3; vbn++ {
		for i := range buf {
			buf[i] = 0xEE // what a read must replace
		}

		if err := h.ReadPage(vbn, buf); err != nil {
			t.Fatalf("ReadPage(%d): %v", vbn, err)
		}

		used := pageSize
		if vbn == 3 {
			used = 100
		}

		for i, b := range buf {
			want := byte(vbn)
			if i >= used {
				want = 0
			}

			if b != want {
				t.Fatalf("block %d byte %d = %#x, want %#x", vbn, i, b, want)
			}
		}
	}

	for _, vbn := range []uint32{0, 4} {
		if err := h.ReadPage(vbn, buf); !errors.Is(err, errPastSource) {
			t.Errorf("ReadPage(%d) = %v, want errPastSource", vbn, err)
		}
	}

	_ = h.Deaccess()
	_ = h.Deaccess()

	if closer.closes != 1 {
		t.Errorf("closed %d times, want 1", closer.closes)
	}
}

// TestReleaseSource: the activator's release of an image file no section
// uses lets it go at once; one sections use waits for the last of them.
func TestReleaseSource(t *testing.T) {
	env, _ := fixture()

	unused := &countingSource{}
	if err := env.ReleaseSource(unused); err != nil || unused.deaccesses != 1 {
		t.Errorf("unused source: %v, %d deaccesses, want 1", err, unused.deaccesses)
	}

	held := &countingSource{}
	env.holdFile(held)
	env.holdFile(held)

	if err := env.ReleaseSource(held); err != nil || held.deaccesses != 0 {
		t.Fatalf("held source: %v, %d deaccesses, want 0", err, held.deaccesses)
	}

	env.dropFile(held)

	if held.deaccesses != 0 {
		t.Fatalf("deaccessed with a section still using it")
	}

	env.dropFile(held)

	if held.deaccesses != 1 {
		t.Errorf("%d deaccesses after the last section, want 1", held.deaccesses)
	}

	if len(env.sectionFiles) != 0 {
		t.Errorf("sectionFiles still has %d files", len(env.sectionFiles))
	}
}

// TestSourcePages: readPage reads a page source's block into a new frame,
// and writePage refuses a source that can't be written.
func TestSourcePages(t *testing.T) {
	env, _ := fixture()
	src := &countingSource{}

	pfn, ok := env.readPage(src, 7, vm.FrameClaim{})
	if !ok {
		t.Fatal("readPage failed")
	}

	buf := make([]byte, pageSize)
	if err := env.mem.LoadPhysical(pfn*pageSize, buf); err != nil {
		t.Fatal(err)
	}

	if buf[0] != 7 || buf[pageSize-1] != 7 {
		t.Errorf("frame holds %#x...%#x, want block 7's", buf[0], buf[pageSize-1])
	}

	if err := env.writePage(src, 7, pfn); !errors.Is(err, errReadOnlySource) {
		t.Errorf("writePage to a read-only source: %v", err)
	}

	env.mem.FreePage(pfn)
}
