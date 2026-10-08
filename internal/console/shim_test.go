package console

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

func TestEnsureShims_definesSymbolsAndIsIdempotent(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	// ensureShims's code-0 entries (e.g. decc$main, below) resolve by
	// looking up kernel.asm's own already-assembled routine by name -- see
	// shim.go's own doc comment -- so it must be assembled first here,
	// matching vax.init's own boot sequence (ASM kernel.asm before any RUN).
	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	addr, ok := c.Symbols.Get("SHIM$LIBRTL_00000A70") // lib$adawi, code 1
	if !ok {
		t.Fatal("expected SHIM$LIBRTL_00000A70 to be defined")
	}

	if addr != c.shimBase {
		t.Errorf("first stub address = %#x, want shimBase %#x", addr, c.shimBase)
	}

	addr2, ok := c.Symbols.Get("SHIM$DECC$SHR_00000000") // decc$main, code 0
	if !ok {
		t.Fatal("expected SHIM$DECC$SHR_00000000 to be defined")
	}

	if addr2 == addr {
		t.Error("distinct table entries should get distinct stub addresses")
	}

	// A code-0 entry resolves by symbol lookup against kernel.asm's own
	// real DECC$MAIN routine (see shim.go's own doc comment), not a
	// synthesized stub in the shim page -- so it must fall well outside
	// the shim page's own address range.
	deccMain, ok := c.Symbols.Get("DECC$MAIN")
	if !ok {
		t.Fatal("expected kernel.asm to define DECC$MAIN")
	}

	if addr2 != deccMain {
		t.Errorf("SHIM$DECC$SHR_00000000 = %#x, want kernel.asm's own DECC$MAIN (%#x)", addr2, deccMain)
	}

	if addr2 >= c.shimBase && addr2 < c.shimBase+uint32(len(shimTable))*shimStubSize {
		t.Errorf("SHIM$DECC$SHR_00000000 = %#x, want an address outside the synthesized shim page", addr2)
	}

	// Idempotent: a second call must not move any stub (a fixup performed
	// after a later RUN must still resolve to the same address).
	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims (again): %v", err)
	}

	addrAgain, _ := c.Symbols.Get("SHIM$LIBRTL_00000A70")
	if addrAgain != addr {
		t.Errorf("second ensureShims moved the stub: %#x -> %#x", addr, addrAgain)
	}
}

// TestEnsureShims_stubDispatchesThroughXFCShim confirms a synthesized
// "live" (nonzero code) stub is a real, callable procedure: CALLS into it
// dispatches DECC$ISASCII (code 28) via XFC$SHIM and returns its result in
// R0, then RET unwinds cleanly.
func TestEnsureShims_stubDispatchesThroughXFCShim(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	addr, ok := c.Symbols.Get("SHIM$DECC$SHR_00000028") // decc$isascii, code 28
	if !ok {
		t.Fatal("expected SHIM$DECC$SHR_00000028 to be defined")
	}

	if err := c.Engine.CallEntry(addr); err != nil {
		t.Fatalf("CallEntry: %v", err)
	}

	for {
		err := c.Engine.Step()
		if err == nil {
			continue
		}

		if errors.Is(err, cpu.ErrConsoleCallReturned) {
			break
		}
		
		t.Fatalf("Step: %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 after DECC$ISASCII stub = %d, want 1", got)
	}
}

// TestShimTable_codesDistinctAndRegistered: every stub's code belongs to
// one routine only, across rtl's shims and internal/librtl's, and each is
// registered in the RTL environment the console builds.
func TestShimTable_codesDistinctAndRegistered(t *testing.T) {
	c := newRunnableConsole(t)
	seen := map[uint32]string{}

	for _, e := range shimTable {
		if e.code == 0 {
			continue
		}

		if other, dup := seen[e.code]; dup {
			t.Errorf("%s and %s share shim code %d", e.name, other, e.code)
		}

		seen[e.code] = e.name

		if !c.RTL.HasShim(e.code) {
			t.Errorf("%s: shim code %d isn't registered", e.name, e.code)
		}
	}
}

// TestEnsureShims_overflowPage: the stubs that don't fit in the page
// VMInit reserves go to an overflow page from the S0 pool, so nothing is
// written over the SCB that follows the reserved page, and each stub's
// symbol names it where it is; more than the overflow page holds is an
// error.
func TestEnsureShims_overflowPage(t *testing.T) {
	saved := shimTable

	t.Cleanup(func() { shimTable = saved })

	perPage := shimPageBytes / shimStubSize

	// Fill both pages exactly.
	fillers := 2 * perPage

	for _, e := range saved {
		if e.code != 0 {
			fillers--
		}
	}

	shimTable = append([]shimEntry{}, saved...)
	for i := 0; i < fillers; i++ {
		shimTable = append(shimTable, shimEntry{name: fmt.Sprintf("TEST$FILLER%d", i), library: "TEST", offset: uint32(i), code: uint32(1000 + i)})
	}

	c := newRunnableConsole(t)
	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	scb := c.CPU.PR(vax.SCBB)

	before := make([]byte, 512)
	if err := c.Mem.LoadPhysical(scb, before); err != nil {
		t.Fatal(err)
	}

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	after := make([]byte, 512)
	if err := c.Mem.LoadPhysical(scb, after); err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Error("ensureShims wrote over the SCB")
	}

	last := fmt.Sprintf("TEST$FILLER%d", fillers-1)

	addr, ok := c.Symbols.Get(last)
	if !ok || (addr >= c.shimBase && addr < c.shimBase+shimPageBytes) {
		t.Errorf("%s at %08X (%v); want it in an overflow page, not VMInit's at %08X", last, addr, ok, c.shimBase)
	}

	// One more doesn't fit.
	for i := fillers; i < fillers+1; i++ {
		shimTable = append(shimTable, shimEntry{name: fmt.Sprintf("TEST$FILLER%d", i), library: "TEST", offset: uint32(i), code: uint32(1000 + i)})
	}

	c = newRunnableConsole(t)
	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.ensureShims(); err == nil {
		t.Error("ensureShims with one stub too many for two pages: no error")
	}
}
