package console

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

func TestEnsureShims_fitsReservedPage(t *testing.T) {
	if got := len(shimTable) * shimStubSize; got > 512 {
		t.Fatalf("shim table needs %d bytes, only 512 reserved", got)
	}
}

func TestEnsureShims_definesSymbolsAndIsIdempotent(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	// ensureShims's code-0 entries (e.g. decc$main, below) resolve by
	// looking up kernel.asm's own already-assembled routine by name -- see
	// shim.go's own doc comment -- so it must be assembled first here,
	// matching vax.init's own boot sequence (ASM kernel.asm before any RUN).
	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	addr, ok := c.Symbols.Get("SHIM$LIBRTL_00000778") // lib$adawi, code 1
	if !ok {
		t.Fatal("expected SHIM$LIBRTL_00000778 to be defined")
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

	addrAgain, _ := c.Symbols.Get("SHIM$LIBRTL_00000778")
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

	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
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
