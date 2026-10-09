package console

import (
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
)

// TestS0PoolAfterVaxInit boots as cmd/govax does, running vax.init (VMINIT
// /P0=16384 /P1=8192 /S0=8192 /KSP=20, then ASM of the microkernel), and
// checks the S0 pool that's left for new processes: it starts past the
// microkernel, ends at S0's end, and holds the number of processes
// corevms/s0pool.go's arithmetic promises.
func TestS0PoolAfterVaxInit(t *testing.T) {
	c, _ := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	pool := c.RTL.S0Pool()
	if pool == nil {
		t.Fatal("no S0 pool after VMINIT")
	}

	base, limit := pool.Range()
	t.Logf("S0 pool %08X-%08X: pages %d-%d, %d free; VMINIT's layout ends at %08X",
		base, limit, (base-0x80000000)/512, (limit-0x80000000)/512-1, pool.FreePages(), c.s0Free)

	if base <= c.s0Free {
		t.Errorf("pool base %08X, want it past the microkernel at %08X", base, c.s0Free)
	}

	if want := uint32(0x80000000 + 8192*512); limit != want {
		t.Errorf("pool limit %08X, want S0's end %08X", limit, want)
	}

	// The pool starts at the first page past the microkernel's S0.
	if c.asmSession == nil {
		t.Fatal("no ASM session after vax.init")
	}

	if end := c.asmSession.S0End(); base != (end+511)&^511 {
		t.Errorf("pool base %08X, want the page after the microkernel's end, %08X", base, end)
	}

	const perProcess = 128 + 64 + 38 + 1 // page tables, stacks, PCB
	if n := pool.FreePages() / perProcess; n < 34 {
		t.Errorf("room for %d processes, want at least 34", n)
	}
}
