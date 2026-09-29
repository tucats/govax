package console

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// TestDiskCreate_assembledProgram runs testdata/asm/disk_create.asm
// (docs/PHASE-26.md subtask 43): a file created, allocated, written, and
// given its end of file with the disk's $QIO functions, then entered in
// the directory under a second name.
func TestDiskCreate_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	mountFreshContainer(t, c, "DUA0")

	runFixture(t, c, "disk_create.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call or IOSB failed)", got)
	}

	result := func(sym, lenSym string) string {
		n := binary.LittleEndian.Uint16(symbolBytes(t, c, lenSym, 2))

		return string(symbolBytes(t, c, sym, int(n)))
	}

	if got := result("RESULT1", "RESLEN1"); got != "NOTES.TXT;1" {
		t.Errorf("RESULT1 = %q", got)
	}

	if got := result("RESULT2", "RESLEN2"); got != "ALIAS.TXT;1" {
		t.Errorf("RESULT2 = %q", got)
	}

	// FIB$L_EXSZ (at 24) came back as the blocks allocated.
	if n := binary.LittleEndian.Uint32(symbolBytes(t, c, "FIB", 28)[24:]); n < 1 {
		t.Errorf("FIB$L_EXSZ = %d", n)
	}

	// Both names are the file: exactly the text written.
	for _, name := range []string{"NOTES.TXT", "ALIAS.TXT"} {
		back := filepath.Join(t.TempDir(), "back.txt")
		if err := c.Copy("DUA0:[000000]"+name, false, back, true, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
			t.Fatalf("Copy %s: %v", name, err)
		}

		if data, _ := os.ReadFile(back); string(data) != "Created by $QIO.\n" {
			t.Errorf("%s holds %q", name, data)
		}
	}
}
