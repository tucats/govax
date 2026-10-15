package machine

import (
	"fmt"
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
