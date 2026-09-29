package console

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// TestDiskDelete_assembledProgram runs testdata/asm/disk_delete.asm
// (docs/PHASE-26.md subtask 44): a temporary file used and gone at
// deaccess, KEEP.TXT renamed by entering a new name and removing the old,
// and OLD.TXT deleted.
func TestDiskDelete_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	mountFreshContainer(t, c, "DUA0")

	dir := t.TempDir()

	for name, text := range map[string]string{"KEEP.TXT": "Keep me.", "OLD.TXT": "Delete me."} {
		host := filepath.Join(dir, name)
		if err := os.WriteFile(host, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := c.Copy(host, true, "DUA0:[000000]"+name, false, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
			t.Fatalf("Copy %s: %v", name, err)
		}
	}

	runFixture(t, c, "disk_delete.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call or IOSB failed)", got)
	}

	if got := string(symbolBytes(t, c, "BUF", 12)); got != "Scratch data" {
		t.Errorf("BUF = %q: the temporary file didn't work while accessed", got)
	}

	result := func(sym, lenSym string) string {
		n := binary.LittleEndian.Uint16(symbolBytes(t, c, lenSym, 2))

		return string(symbolBytes(t, c, sym, int(n)))
	}

	if got := result("RESULT2", "RESLEN2"); got != "RENAMED.TXT;1" {
		t.Errorf("RESULT2 = %q", got)
	}

	if got := result("RESULT3", "RESLEN3"); got != "KEEP.TXT;1" {
		t.Errorf("RESULT3 = %q", got)
	}

	// What's left: RENAMED.TXT, holding KEEP.TXT's text, and none of
	// the other three names.
	back := filepath.Join(dir, "back.txt")
	if err := c.Copy("DUA0:[000000]RENAMED.TXT", false, back, true, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy RENAMED.TXT: %v", err)
	}

	if data, _ := os.ReadFile(back); string(data) != "Keep me." {
		t.Errorf("RENAMED.TXT holds %q", data)
	}

	mfd := rms.FileID{Num: 4, Seq: 4}
	for _, name := range []string{"KEEP.TXT", "OLD.TXT", "SCRATCH.TMP"} {
		if _, _, err := c.Mounts.ACPLookup("DUA0", mfd, name); err == nil {
			t.Errorf("%s is still in the directory", name)
		}
	}
}
