package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// TestDiskQIO_assembledProgram runs testdata/asm/disk_qio.asm
// (docs/PHASE-26.md subtask 41): a file on a mounted ODS-2 volume looked
// up, accessed, read, written, and read again with the disk's $QIO
// functions.
func TestDiskQIO_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	mountFreshContainer(t, c, "DUA0")

	host := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(host, []byte("Original text, long enough to see."), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Copy(host, true, "DUA0:[000000]DATA.TXT", false, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	runFixture(t, c, "disk_qio.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call or IOSB failed)", got)
	}

	text := func(sym string, n int) string {
		a, _ := c.Symbols.Get(sym)
		b := make([]byte, n)

		if err := c.Mem.Load(c.CPU, a, b); err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	if got := text("BUF1", 13); got != "Original text" {
		t.Errorf("BUF1 = %q, want the file's first block", got)
	}

	if got := text("BUF2", 14); got != "Hello, disk!\x00\x00" {
		t.Errorf("BUF2 = %q, want the new block, zero-padded", got)
	}

	if got := text("RESULT", 10); got != "DATA.TXT;1" {
		t.Errorf("RESULT = %q", got)
	}

	// The change reached the volume: copy the file back out.
	back := filepath.Join(t.TempDir(), "back.txt")
	if err := c.Copy("DUA0:[000000]DATA.TXT", false, back, true, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy back: %v", err)
	}

	data, _ := os.ReadFile(back)
	if !strings.HasPrefix(string(data), "Hello, disk!") {
		t.Errorf("the file on the volume starts %q", data[:min(len(data), 16)])
	}
}
