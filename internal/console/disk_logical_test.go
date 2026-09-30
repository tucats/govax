package console

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestDiskLogical_assembledProgram runs testdata/asm/disk_logical.asm
// (docs/PHASE-26.md subtask 45): the home block read logically, the boot
// block written logically and read back physically, the privilege check,
// and a block past the end of the volume.
func TestDiskLogical_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	image := filepath.Join(t.TempDir(), "logical.dsk")
	if err := c.InitializeContainer(image, 400, "LBNVOL", 0, "RD54"); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	if err := c.Mount("DUA0", image, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	runFixture(t, c, "disk_logical.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call or IOSB failed)", got)
	}

	home := symbolBytes(t, c, "HOME", 512)
	if got := string(home[496:508]); got != "DECFILE11B  " {
		t.Errorf("HOME's format (HM2$T_FORMAT) %q", got)
	}

	if got := strings.TrimRight(string(home[0x1D8:0x1E4]), " "); got != "LBNVOL" {
		t.Errorf("HOME's volume name (HM2$T_VOLNAME) %q", got)
	}

	if got := string(symbolBytes(t, c, "BACK", 17)); got != "govax boot block\x00" {
		t.Errorf("BACK = %q", got)
	}

	le := func(sym string) uint32 { return binary.LittleEndian.Uint32(symbolBytes(t, c, sym, 4)) }

	if got := le("NOPRV"); got != vmsdef.Symbols["SS$_NOPRIV"] {
		t.Errorf("NOPRV = %#x, want SS$_NOPRIV", got)
	}

	if got := le("ILLBLK"); got != vmsdef.Symbols["SS$_ILLBLKNUM"] {
		t.Errorf("ILLBLK = %#x, want SS$_ILLBLKNUM", got)
	}

	// The write reached the image: dismount, mount again, and look.
	if err := c.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", image, false); err != nil {
		t.Fatal(err)
	}

	block, err := c.Mounts.ReadLogical("DUA0", 0, 16)
	if err != nil || string(block) != "govax boot block" {
		t.Errorf("LBN 0 on the image: %q, %v", block, err)
	}
}
