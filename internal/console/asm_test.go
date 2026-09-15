package console

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func asmFixturePath(t testing.TB, name string) string {
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
// Through Phase 12, this test hit its step cap every time: kernel.asm's own
// EXE$$PUT_CONSOLE (CHMK 0) writes each byte by clearing a memory flag
// (exe$tx_ready), doing MTPR to TXDB, then spin-waiting on that same flag --
// expecting the EXC$CONWRITE interrupt kernel.asm's own ISR (exe$tx) handles
// to set it back to 1, which setPrivReg's then-plain-register-store TXCS/
// TXDB cases never delivered. Phase 14 closes that gap (interrupt.go's
// Engine.Interrupt/quantum-boundary delivery, wired into TXCS/TXDB in
// procreg.go) -- this test now enables TXCS<IE> directly (the same effect
// as kernel.asm's own EXE$INITIALIZE, without also running that routine's
// separate, unrelated boot-message-printing and page-protection machinery,
// which this test has no need to exercise) and expects hello.asm's whole
// "Hello world" print plus its own 1,000,000-iteration ADDF2/SOBGTR delay
// loop to run to completion, not hit the cap.
func TestAssemble_kernelThenHelloRunsBounded(t *testing.T) {
	c := newRunnableConsole(t)
	c.asmSession = nil // start from a clean session explicitly, for clarity
	out := &bytes.Buffer{}
	c.Out = out

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

	// Enable TXCS<IE> -- the same one-time setup EXE$INITIALIZE's own
	// `mtpr #40,#VAX$PR_TXCS` performs -- so EXE$$PUT_CONSOLE's ready-flag
	// wait loop actually gets woken up by the EXC$CONWRITE interrupt its own
	// ISR (exe$tx) delivers, instead of spinning forever after the first
	// byte. See this test's own doc comment on why EXE$INITIALIZE itself
	// isn't run here.
	c.CPU.SetPR(vax.TXCS, 0x40)

	// hello.asm's own delay loop ("movl #1000000,r5") costs far more than
	// its literal decimal reading suggests: this assembler's default radix
	// is hex (no "^d"/"^o" prefix on the literal), so r5 actually starts at
	// 0x1000000 (16,777,216) -- a ~33.5M-step ADDF2/SOBGTR loop, not 2M. The
	// print and kernel-dispatch overhead around it (a few hundred steps per
	// character) is negligible by comparison.
	err, hitCap := callBounded(t, c, entryAddr, 40_000_000)
	if err != nil {
		t.Fatalf("hello.asm: %v", err)
	}
	if hitCap {
		t.Fatalf("hello.asm hit the step cap instead of completing -- output so far=%q", out.String())
	}
	if !strings.Contains(out.String(), "Hello world") {
		t.Errorf("console output = %q, want it to contain \"Hello world\"", out.String())
	}
}
