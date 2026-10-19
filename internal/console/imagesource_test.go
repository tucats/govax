package console

import (
	"bytes"
	"os"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
)

// checkImageSource fails the test unless src's blocks are want's, the
// last filled out with zeros.
func checkImageSource(t *testing.T, src corevms.PageSource, want []byte) {
	t.Helper()

	buf := make([]byte, 512)

	for vbn := uint32(1); int(vbn-1)*512 < len(want); vbn++ {
		if err := src.ReadPage(vbn, buf); err != nil {
			t.Fatalf("ReadPage(%d): %v", vbn, err)
		}

		block := want[(vbn-1)*512 : min(int(vbn)*512, len(want))]
		if !bytes.Equal(buf[:len(block)], block) {
			t.Fatalf("block %d differs from the file's", vbn)
		}
	}
}

// TestOpenImage_volume: the main image on a mounted volume is opened as a
// held page source (Phase 66, subtask 2): its bytes are the file's, its
// blocks read through the source, and while it's held the volume can't
// be dismounted.
func TestOpenImage_volume(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	host := exeFixturePath(t, "put.exe")

	want, err := os.ReadFile(host)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Copy(host, true, "DUA0:[000000]PUT.EXE", false, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	data, src, err := c.OpenImage("DUA0:[000000]PUT.EXE", true)
	if err != nil {
		t.Fatalf("OpenImage: %v", err)
	}

	if !bytes.Equal(data, want) {
		t.Errorf("OpenImage's bytes differ from the file's (%d, %d)", len(data), len(want))
	}

	if _, ok := src.(*rms.ACPFile); !ok {
		t.Errorf("a volume image's source is a %T, want an *rms.ACPFile", src)
	}

	checkImageSource(t, src, want)

	if err := c.Mounts.Dismount("DUA0"); err == nil {
		t.Error("the image's volume was dismounted while the image was held")
	}

	if err := c.ProcessOne().ReleaseSource(src); err != nil {
		t.Fatal(err)
	}

	if err := c.Mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after the image was let go: %v", err)
	}
}

// TestOpenImage_host: a host image is an open host file.
func TestOpenImage_host(t *testing.T) {
	c, _ := newTestConsole(t)

	host := exeFixturePath(t, "put.exe")

	want, err := os.ReadFile(host)
	if err != nil {
		t.Fatal(err)
	}

	data, src, err := c.OpenImage(host, false)
	if err != nil {
		t.Fatalf("OpenImage: %v", err)
	}

	if !bytes.Equal(data, want) {
		t.Errorf("OpenImage's bytes differ from the file's")
	}

	if _, ok := src.(*corevms.HostPageSource); !ok {
		t.Errorf("a host image's source is a %T, want a *corevms.HostPageSource", src)
	}

	checkImageSource(t, src, want)

	if err := src.Deaccess(); err != nil {
		t.Error(err)
	}

	if _, _, err := c.OpenImage("no-such-image", false); err == nil {
		t.Error("a missing image was opened")
	}
}
