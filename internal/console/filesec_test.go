package console_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// Phase 49's subtask 9: a global section of a file shared by two
// processes. testdata/sec49/secparent.mar makes the section and starts
// secchild.mar, which maps it by name; each sees the other's writes, and
// the pages reach the file when the section goes.

// sectionMachine boots a scheduled console at quantum with a fresh volume
// on DUA0 and testdata/sec49's programs names built onto it.
func sectionMachine(t *testing.T, quantum string, names ...string) *console.Console {
	t.Helper()

	c, out := scheduledConsole(t, quantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 2000, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	for _, name := range names {
		src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sec49", name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		mar, obj := filepath.Join(dir, name+".mar"), filepath.Join(dir, name+".obj")
		if err := os.WriteFile(mar, src, 0o644); err != nil {
			t.Fatal(err)
		}

		for _, cmd := range []string{
			`MACRO "` + mar + `"/OBJECT="` + obj + `"`,
			`LINK "` + obj + `"/EXECUTABLE=DUA0:[000000]` + strings.ToUpper(name) + `.EXE`,
		} {
			if err := d.Dispatch(cmd); err != nil {
				t.Fatalf("%s: %v\n%s", cmd, err, out.String())
			}
		}
	}

	return c
}

// sectionFileBlocks reads the first n blocks of a volume file, whatever
// its end of file says (a section's pages don't move it).
func sectionFileBlocks(t *testing.T, c *console.Console, name string, n uint32) []byte {
	t.Helper()

	fid, _, err := c.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	f, err := c.Mounts.ACPAccess("DUA0", fid, false)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Deaccess() }()

	out := make([]byte, n*512)
	for i := range n {
		if err := f.ReadPage(i+1, out[i*512:]); err != nil {
			t.Fatalf("%s block %d: %v", name, i+1, err)
		}
	}

	return out
}

// TestFileSectionPair runs the pair under a short quantum and the
// default one, and checks the file.
func TestFileSectionPair(t *testing.T) {
	for _, quantum := range []string{"7", "20000"} {
		t.Run(quantum, func(t *testing.T) {
			c := sectionMachine(t, quantum, "secchild", "secparent")

			if err := c.Run("DUA0:[000000]SECPARENT.EXE", console.RunOptions{}); err != nil {
				t.Fatalf("RUN: %v", err)
			}

			if got := c.CPU.GPR(0); got != 999 {
				t.Fatalf("R0 = %d, want 999 (another value is the step that failed)", got)
			}

			if n := len(c.RTL.Processes()); n != 1 {
				t.Errorf("%d processes left, want only process 1", n)
			}

			if s := c.RTL.Sections.Sections(); len(s) != 0 {
				t.Errorf("sections left: %+v", s)
			}

			b := sectionFileBlocks(t, c, "SHARED.SEC", 2)
			long := func(off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }

			if long(0) != 0x11111111 || long(512) != 0x22222222 || long(516) != 1 {
				t.Errorf("the file has %08X, %08X, %08X; want 11111111, 22222222, 1", long(0), long(512), long(516))
			}
		})
	}
}

// TestFileSectionRundown runs testdata/sec49/rundown.mar, which leaves a
// private section and a copy-on-reference one mapped when it exits:
// image rundown writes the first's modified page to the file, and not
// the copy's, and lets the file go.
func TestFileSectionRundown(t *testing.T) {
	c := sectionMachine(t, longQuantum, "rundown")

	if err := c.Run("DUA0:[000000]RUNDOWN.EXE", console.RunOptions{}); err != nil {
		t.Fatal(err)
	}

	if got := c.CPU.GPR(vax.R0); got != 999 {
		t.Fatalf("R0 = %d, want 999 (another value is the step that failed)", got)
	}

	fid, _, err := c.Mounts.ACPLookup("DUA0", rms.FileID{Num: 4, Seq: 4}, "RUNDOWN.SEC")
	if err != nil {
		t.Fatal(err)
	}

	// Exclusive access: no channel or section still has the file.
	f, err := c.Mounts.ACPAccessWith("DUA0", fid, rms.ACPAccessMode{Write: true, NoRead: true, NoWrite: true})
	if err != nil {
		t.Fatalf("exclusive access after rundown: %v", err)
	}

	defer func() { _ = f.Deaccess() }()

	buf := make([]byte, 512)
	if err := f.ReadPage(1, buf); err != nil {
		t.Fatal(err)
	}

	if got := binary.LittleEndian.Uint32(buf); got != 0x55555555 {
		t.Errorf("block 1 starts %08X, want 55555555 (the private section's page, not the copy's)", got)
	}
}
