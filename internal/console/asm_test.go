package console

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func asmFixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "asm", name)
}

// TestAssemble_xorDepositsAndIsCallable assembles testdata/asm/xor.asm (a
// small, dependency-free fixture: MOVL/MOVL/XORL3/RET, no RTL/kernel calls)
// and CALLs its "test" entry point, checking the computed XOR lands in R4 --
// confirming ASM's deposit-into-live-memory and symbol-table-merge (so
// "test" resolves by name) both work end-to-end.
func TestAssemble_xorDepositsAndIsCallable(t *testing.T) {
	c := newRunnableConsole(t)

	_, hasEntry, err := c.Assemble(asmFixturePath(t, "xor.asm"))
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if hasEntry {
		t.Fatal("xor.asm's bare .end should not report an entry address")
	}

	addr, ok := c.Symbols.Get("TEST")
	if !ok {
		t.Fatal("expected xor.asm's \"test\" label to be merged into Console.Symbols")
	}

	if err := c.Call(addr, false); err != nil {
		t.Fatalf("Call: %v", err)
	}

	const want = 0xC8600 ^ 0x10
	if got := c.CPU.GPR(vax.R4); got != want {
		t.Errorf("R4 = %#x, want %#x", got, want)
	}
}

// TestAssemble_persistentSessionSharesSymbolsAcrossFiles assembles two
// fixtures back to back through the same Console (and therefore the same
// persistent asmSession -- see machine.go's doc comment) and checks the
// second file's own label is independently callable, and that neither
// file's ".end" state leaks into the other's TakeEntry() result -- movq.asm
// has ".end main" (an entry) and is assembled first; xor.asm's plain ".end"
// (no entry) assembled second must report hasEntry=false, not a stale
// leftover from movq.asm.
func TestAssemble_persistentSessionSharesSymbolsAcrossFiles(t *testing.T) {
	c := newRunnableConsole(t)

	_, hasEntry1, err := c.Assemble(asmFixturePath(t, "movq.asm"))
	if err != nil {
		t.Fatalf("Assemble(movq.asm): %v", err)
	}
	if !hasEntry1 {
		t.Fatal("expected movq.asm's \".end main\" to report an entry address")
	}

	_, hasEntry2, err := c.Assemble(asmFixturePath(t, "xor.asm"))
	if err != nil {
		t.Fatalf("Assemble(xor.asm): %v", err)
	}
	if hasEntry2 {
		t.Fatal("expected xor.asm's bare \".end\" to report no entry, even after movq.asm's did")
	}

	if _, ok := c.Symbols.Get("MAIN"); !ok {
		t.Error("expected movq.asm's \"main\" label to still be defined")
	}
	if _, ok := c.Symbols.Get("TEST"); !ok {
		t.Error("expected xor.asm's \"test\" label to be defined")
	}
}

// TestAssemble_kernelThenHelloRunsBounded assembles kernel.asm (the
// microkernel image, defining LIB$PUT_OUTPUT and wiring the .SCB CHMK
// vector) and then hello.asm on top, in the same session -- matching
// vax.init's own boot sequence (ASM kernel.asm) followed by an interactive
// "ASM hello.asm" -- and runs hello.asm's auto-triggered entry (.end main)
// with a bounded step count so a real gap anywhere in the CHMK/RTL dispatch
// chain reports as a clear, logged outcome rather than hanging the suite.
//
// This currently hits the step cap, not a clean completion: kernel.asm's
// own EXE$$PUT_CONSOLE (CHMK 0) writes each byte by clearing a memory flag
// (exe$tx_ready), doing MTPR to TXDB, then spin-waiting on that same flag
// -- expecting the EXC$CONWRITE interrupt kernel.asm's own ISR (exe$tx)
// handles to set it back to 1. setPrivReg's TXCS/TXDB cases are a plain
// register store with no device/interrupt modeling (see its own doc
// comment: deferred to Phase 09, which never actually implemented this),
// so exe$tx_ready is only ever cleared, never reset -- the byte after the
// first spins forever. This is exactly the mechanism the user asked about
// while this finding was still being chased (2026-09-15): not a countdown
// timer, but a missing interrupt-delivery path. Tracked as a real, well-
// understood Phase 09 gap rather than a hang/panic bug; revisit there or
// in a dedicated follow-up. This test's own bar is only "reaches the step
// cap in a controlled, non-panicking way" -- if it ever completes cleanly
// instead, that's progress, not a regression, and this test should be
// updated to expect it.
func TestAssemble_kernelThenHelloRunsBounded(t *testing.T) {
	c := newRunnableConsole(t)
	c.asmSession = nil // start from a clean session explicitly, for clarity

	if _, hasEntry, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	} else if hasEntry {
		t.Fatal("kernel.asm has no .end entry name; should not auto-CALL")
	}

	entryAddr, hasEntry, err := c.Assemble(asmFixturePath(t, "hello.asm"))
	if err != nil {
		t.Fatalf("Assemble(hello.asm): %v", err)
	}
	if !hasEntry {
		t.Fatal("expected hello.asm's \".end main\" to report an entry address")
	}

	err, hitCap := callBounded(t, c, entryAddr, 2_000_000)
	if err != nil {
		t.Errorf("hello.asm: unexpected error (want either a clean finish or the step cap): %v", err)
	}
	if !hitCap {
		t.Log("hello.asm completed cleanly -- the TXCS/TXDB interrupt-delivery gap this test documents may be fixed; update this test's expectations")
	}
}
