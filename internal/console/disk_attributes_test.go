package console

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// symbolBytes reads n bytes of the program's memory at the symbol sym.
func symbolBytes(t *testing.T, c *Console, sym string, n int) []byte {
	t.Helper()

	a, ok := c.Symbols.Get(sym)
	if !ok {
		t.Fatalf("no symbol %s", sym)
	}

	b := make([]byte, n)
	if err := c.Mem.Load(c.CPU, a, b); err != nil {
		t.Fatal(err)
	}

	return b
}

// fatEndOfFile decodes a $FATDEF record attribute area's end of file:
// FAT$L_EFBLK (a swapped longword, high word first) and FAT$W_FFBYTE.
func fatEndOfFile(fat []byte) (uint32, uint16) {
	return uint32(binary.LittleEndian.Uint16(fat[8:]))<<16 | uint32(binary.LittleEndian.Uint16(fat[10:])),
		binary.LittleEndian.Uint16(fat[12:])
}

// TestDiskAttributes_assembledProgram runs testdata/asm/disk_attributes.asm
// (docs/PHASE-26.md subtask 42): a file's record attributes read on
// access, its first block overwritten with shorter text, and its end of
// file set to the byte by an attribute list on deaccess, as RMS does.
func TestDiskAttributes_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	mountFreshContainer(t, c, "DUA0")

	const original = "Original text, long enough to see."

	host := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(host, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Copy(host, true, "DUA0:[000000]DATA.TXT", false, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	runFixture(t, c, "disk_attributes.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call or IOSB failed)", got)
	}

	// FAT1 was read before the change (its end of file then 1/34, and
	// the program has since set it to 1/10); FAT2 after.
	if blk, ffb := fatEndOfFile(symbolBytes(t, c, "FAT2", 32)); blk != 1 || ffb != 10 {
		t.Errorf("FAT2's end of file %d/%d, want 1/10", blk, ffb)
	}

	if got := string(symbolBytes(t, c, "NAME2", 20)); got != "DATA.TXT;1          " {
		t.Errorf("NAME2 = %q", got)
	}

	// The file on the volume is now exactly the 10 bytes written.
	back := filepath.Join(t.TempDir(), "back.txt")
	if err := c.Copy("DUA0:[000000]DATA.TXT", false, back, true, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy back: %v", err)
	}

	if data, _ := os.ReadFile(back); string(data) != "Short now." {
		t.Errorf("the file on the volume is %q, want \"Short now.\"", data)
	}
}
