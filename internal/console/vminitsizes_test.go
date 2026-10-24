package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/respath"
)

// Phase 67's subtask 8 (docs/PHASE-67 - address space under paging.md):
// with paging on, vax.init gives VMINIT 0 for each region the
// configuration doesn't size, so VMINIT's paging defaults apply.

// runVaxInit runs the built-in vax.init on a new console with the
// settings given, and returns the console.
func runVaxInit(t *testing.T, config map[string]string) *Console {
	t.Helper()

	withSettings(t, config)

	c, _ := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	return c
}

// TestVaxInitPagingSizes: vax.init with paging on uses VMINIT's paging
// defaults (P0 19456, P1 5120, S0 16384), leaving nearly every frame
// free, as SHOW MEMORY shows; a vax.memory.* key still sizes its region.
func TestVaxInitPagingSizes(t *testing.T) {
	c := runVaxInit(t, map[string]string{machine.PagingSetting: "true"})

	if p0, p1 := c.VMInitSizes(); p0 != 19456 || p1 != 5120 {
		t.Errorf("/P0 %d, /P1 %d, want 19456 and 5120", p0, p1)
	}

	if got := c.CPU.PR(registers.SLR); got != 16384 {
		t.Errorf("S0 %d pages, want 16384", got)
	}

	if got := c.ProcessOne().Sysgen.VirtualPageCount; got != 24576 {
		t.Errorf("VIRTUALPAGECNT %d, want 24576", got)
	}

	// The frames in use: VMINIT's layout (its 128-page system page table
	// among it, but for process 1's unused table pages), the microkernel,
	// the shim stubs, and process 1's few pages.
	counts := c.Mem.Paging().Counts()
	if counts.Free < 16000 {
		t.Errorf("%d free frames of 16384, want at least 16000", counts.Free)
	}

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	if err := c.ShowMemory(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Size = 00004000 (16384 decimal) pages\n            PFN database = 128 pages  S0BR = 00000C00    S0LR = 00004000",
		"Virtual addresses  = 80000000-807FFFFF",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("SHOW MEMORY lacks %q:\n%s", want, buf.String())
		}
	}

	// A configured region keeps its size with paging on.
	c = runVaxInit(t, map[string]string{machine.PagingSetting: "yes", "vax.memory.p0": "8192"})

	if p0, p1 := c.VMInitSizes(); p0 != 8192 || p1 != 5120 {
		t.Errorf("vax.memory.p0 8192: /P0 %d, /P1 %d, want 8192 and 5120", p0, p1)
	}
}

// TestVaxInitNoPagingSizes: with paging off vax.init's sizes are as
// before, P0 16384, P1 8192, S0 8192.
func TestVaxInitNoPagingSizes(t *testing.T) {
	c := runVaxInit(t, map[string]string{machine.PagingSetting: "false"})

	if p0, p1 := c.VMInitSizes(); p0 != 16384 || p1 != 8192 {
		t.Errorf("/P0 %d, /P1 %d, want 16384 and 8192", p0, p1)
	}

	if got := c.CPU.PR(registers.SLR); got != 8192 {
		t.Errorf("S0 %d pages, want 8192", got)
	}
}
