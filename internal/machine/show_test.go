package machine

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vm"
)

// TestShowMemory_frames: in paging mode SHOW MEMORY adds the PFN
// database's counts of frames by state; with paging off it doesn't.
func TestShowMemory_frames(t *testing.T) {
	c, buf := newTestMachine(t)

	if err := c.VMInitPaging(false, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	buf.Reset()

	if err := c.ShowMemory(); err != nil {
		t.Fatalf("ShowMemory: %v", err)
	}

	if strings.Contains(buf.String(), "Page frames") {
		t.Errorf("paging off, output = %q, want no frame counts", buf.String())
	}

	if err := c.VMInitPaging(true, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	buf.Reset()

	if err := c.ShowMemory(); err != nil {
		t.Fatalf("ShowMemory: %v", err)
	}

	counts := c.Mem.Paging().Counts()
	out := buf.String()

	for _, line := range []string{
		"    Page frames (paging on)\n",
		fmt.Sprintf("        Free list          %8d\n", counts.Free),
		fmt.Sprintf("        Modified list      %8d\n", counts.Modified),
		fmt.Sprintf("        Active             %8d\n", counts.Active()),
		fmt.Sprintf("            System pages   %8d\n", counts.System),
		"    Page faults        ",
		"        Demand zero        ",
		"    Reclaimer runs     ",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("output = %q, want it to contain %q", out, line)
		}
	}

	if counts.Free == 0 || counts.System == 0 {
		t.Errorf("counts = %+v, want free frames and VMINIT's system frames", counts)
	}

	if counts.Free+counts.Modified+counts.Active() != c.Mem.Paging().Frames() {
		t.Errorf("counts = %+v don't add up to %d frames", counts, c.Mem.Paging().Frames())
	}
}

// TestShowPage_forms: SHOW PAGE names each form a PTE takes, valid and
// invalid, and the number the form holds.
func TestShowPage_forms(t *testing.T) {
	c, buf := newTestMachine(t)

	if err := c.VMInitPaging(true, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	const addr = 0x200 // P0's second page

	cases := []struct {
		pte  vm.PTE
		want string
	}{
		{vm.ValidPTE(0x123, vm.ProtUW, 3), "valid, PFN 00000123"},
		{vm.SectionPTE(5, vm.ProtUW, 3), "process section, PSTX 5"},
		{vm.GlobalPTE(0x41, vm.ProtUW, 3), "global page, GPTX 41"},
		{vm.PageFilePTE(0x1A, vm.ProtUW, 3), "page file, block 1A"},
		{vm.PageFilePTE(0, vm.ProtUW, 3), "page file, block not yet allocated"},
		{vm.TransitionPTE(0x77, vm.ProtUW, 3), "transition, PFN 00000077"},
		{vm.PTE(0), "demand zero"},
	}

	for _, tc := range cases {
		if err := c.Mem.StorePTE(c.CPU, addr, tc.pte); err != nil {
			t.Fatalf("StorePTE: %v", err)
		}

		buf.Reset()

		if err := c.ShowPage(fmt.Sprintf("^X%X", addr), false); err != nil {
			t.Fatalf("ShowPage: %v", err)
		}

		if want := "    PTE Form:          " + tc.want + "\n"; !strings.Contains(buf.String(), want) {
			t.Errorf("PTE %08X: output = %q, want %q", uint32(tc.pte), buf.String(), want)
		}
	}
}

// TestShowMemory_pageFile: in paging mode SHOW MEMORY has a page file
// section: none configured, or one configured but not yet needed (SHOW
// MEMORY doesn't open it), with the pages committed to it; and the
// writer's write I/Os under its pages written.
func TestShowMemory_pageFile(t *testing.T) {
	c, buf := newTestMachine(t)

	if err := c.VMInitPaging(true, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	show := func() string {
		t.Helper()
		buf.Reset()

		if err := c.ShowMemory(); err != nil {
			t.Fatalf("ShowMemory: %v", err)
		}

		return buf.String()
	}

	out := show()
	for _, line := range []string{
		"        Write I/Os                0\n",
		"    Page file: none\n",
		"    Page file committed",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("output = %q, want it to contain %q", out, line)
		}
	}

	c.processOne.Sysgen.PageFileName = "DUA0:[SYSEXE]PAGEFILE.SYS"

	if out := show(); !strings.Contains(out, "    Page file: DUA0:[SYSEXE]PAGEFILE.SYS (not opened yet)\n") {
		t.Errorf("output = %q, want the page file not opened yet", out)
	}

	if c.processOne.PageFile() != nil {
		t.Error("SHOW MEMORY opened the page file")
	}
}

// TestShowMemoryFiles: SHOW MEMORY/FILES and /FILES/FULL in VMS 7.1's
// VAX layout (testdata/pg64), with no page file and with a host one.
func TestShowMemoryFiles(t *testing.T) {
	c, buf := newTestMachine(t)

	if err := c.VMInitPaging(true, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	show := func(full bool) []string {
		t.Helper()
		buf.Reset()

		if err := c.ShowMemoryFiles(full); err != nil {
			t.Fatalf("ShowMemoryFiles: %v", err)
		}

		return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	}

	const headings = "Paging File Usage (pages):                      Free  Reservable       Total"

	// No page file: the title and the headings alone; /FULL the title.
	lines := show(false)
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "              System Memory Resources on ") ||
		lines[1] != "" || lines[2] != headings {
		t.Errorf("no page file: %q", lines)
	}

	if lines := show(true); len(lines) != 2 {
		t.Errorf("no page file, /FULL: %q", lines)
	}

	path := filepath.Join(t.TempDir(), "pagefile.sys")
	c.processOne.Sysgen.PageFileName = `"` + path + `"`
	c.processOne.Sysgen.PageFileSize = 600

	if c.processOne.OpenPageFile() == nil {
		t.Fatal("no page file opened")
	}

	committed := int(c.processOne.PageFileReport().Charged)

	lines = show(false)
	want := []string{
		headings,
		fmt.Sprintf("  %-78s", path),
		fmt.Sprintf("%52d%12d%12d", 600, 600-committed, 600),
	}

	if len(lines) != 5 || !slices.Equal(lines[2:], want) {
		t.Errorf("SHOW MEMORY/FILES:\n%s\nwant\n%s", strings.Join(lines[2:], "\n"), strings.Join(want, "\n"))
	}

	lines = show(true)
	want = []string{
		fmt.Sprintf("%-80s", path),
		fmt.Sprintf("    Free Blocks                    600    Reservable Blocks %16d", 600-committed),
		"    Total Size (blocks)            600    Paging File Number               3",
		fmt.Sprintf("    Swap Usage (processes)           0    Paging Usage (processes)%10d", len(c.processOne.Processes())),
		"  This file can be used for either paging or swapping.",
	}

	if len(lines) != 7 || !slices.Equal(lines[2:], want) {
		t.Errorf("SHOW MEMORY/FILES/FULL:\n%s\nwant\n%s", strings.Join(lines[2:], "\n"), strings.Join(want, "\n"))
	}
}
