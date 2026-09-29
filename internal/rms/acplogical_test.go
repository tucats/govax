package rms

import (
	"bytes"
	"errors"
	"testing"
)

func TestReadLogical(t *testing.T) {
	m := newACPFixture(t)

	// LBN 1 is the home block: its format string, HM2$T_FORMAT, is at
	// offset 496.
	home, err := m.ReadLogical("DUA0:", 1, 512)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(home[496:508]); got != "DECFILE11B  " {
		t.Errorf("home block format %q", got)
	}

	// A partial block, across two blocks.
	two, err := m.ReadLogical("DUA0", 1, 700)
	if err != nil || len(two) != 700 || !bytes.Equal(two[:512], home) {
		t.Errorf("700 bytes from LBN 1: %d bytes, %v", len(two), err)
	}

	vol, _ := m.Lookup("DUA0")
	last := vol.Devices[0].Container.Blocks() - 1

	if _, err := m.ReadLogical("DUA0", last, 512); err != nil {
		t.Errorf("the last block: %v", err)
	}

	if _, err := m.ReadLogical("DUA0", last, 513); !errors.Is(err, ErrACPIllegalBlock) {
		t.Errorf("past the end: %v", err)
	}

	if _, err := m.ReadLogical("DUA1", 0, 512); !errors.Is(err, ErrACPNotMounted) {
		t.Errorf("unmounted: %v", err)
	}
}

func TestWriteLogical(t *testing.T) {
	m := newACPFixture(t)

	// LBN 0, the boot block, which the file system doesn't use: a short
	// write, zero-padded.
	if err := m.WriteLogical("DUA0", 0, []byte("boot me")); err != nil {
		t.Fatal(err)
	}

	got, _ := m.ReadLogical("DUA0", 0, 512)
	if !bytes.HasPrefix(got, []byte("boot me\x00\x00")) || got[511] != 0 {
		t.Errorf("LBN 0 = %q...", got[:10])
	}

	vol, _ := m.Lookup("DUA0")
	end := vol.Devices[0].Container.Blocks()

	if err := m.WriteLogical("DUA0", end-1, make([]byte, 1024)); !errors.Is(err, ErrACPIllegalBlock) {
		t.Errorf("past the end: %v", err)
	}

	ro := NewMountTable()
	if err := ro.Mount("DUA0", newTestVolumeFile(t, "ROVOL"), false); err != nil {
		t.Fatal(err)
	}

	if err := ro.WriteLogical("DUA0", 0, []byte("x")); !errors.Is(err, ErrACPWriteLocked) {
		t.Errorf("read-only mount: %v", err)
	}
}
