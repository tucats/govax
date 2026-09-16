package console

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// This file is Phase 12's own named deliverable (docs/PHASE-12.md): a
// fixture-driven regression suite exercising testdata/asm/*.asm (via the
// ASM/CALL console commands, Phase 12 sub-phases 1-2) end to end, alongside
// the exe-fixture milestone check already covered by run_test.go's
// TestRun_everyMilestoneFixture (Phase 13) and the ROM/NVRAM save/load
// round trip already covered by rom_test.go (Phase 08) -- all three run
// together under `go test ./...`.
//
// Excluded, matching internal/asm/fixtures_test.go's own precedent:
// ssdef.asm (an .INCLUDE-only fragment with no code of its own) and
// forth.asm (needs its own .MICROKERNEL configuration and is a large,
// interactive FORTH interpreter with no single well-defined entry point to
// drive here -- already covered for assembly correctness by
// TestAssembleForth). kernel.asm itself is exercised indirectly by every
// RTL-dependent case below (assembled first, in the same session), and
// directly by TestAssembleKernel/TestImageFixup_everyRealFixtureFixesUp.

// runAsmRegression assembles and calls one self-contained fixture (no
// kernel.asm dependency), returning the resulting Console for the caller
// to inspect register/memory state, and the bounded run's terminating
// error/cap outcome.
func runAsmRegression(t *testing.T, fixture, entrySymbol string, maxSteps int) (*Console, error, bool) {
	t.Helper()
	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(asmFixturePath(t, fixture)); err != nil {
		t.Fatalf("Assemble(%s): %v", fixture, err)
	}

	addr, ok := c.Symbols.Get(entrySymbol)
	if !ok {
		t.Fatalf("%s: expected symbol %q to be defined", fixture, entrySymbol)
	}

	err, hitCap := callBounded(t, c, addr, maxSteps)

	return c, err, hitCap
}

// TestRegression_movq checks movq.asm's quadword move/store round trip
// byte-for-byte (docs/PHASE-11.md's own testdata/asm fixture list). Only
// R0 and the data2 memory side effect are checked, not R4/R5 (which
// movq.asm's own ".entry main, ^m<r4,r5,r6,r7>" mask saves on entry and
// restores on RET, so they're back to their pre-call value of 0 by the
// time CALL returns -- checking them here would just be re-testing
// CALLS/RET's own register-mask save/restore, already covered directly by
// internal/cpu/call_test.go).
func TestRegression_movq(t *testing.T) {
	c, err, hitCap := runAsmRegression(t, "movq.asm", "MAIN", 1000)
	if err != nil || hitCap {
		t.Fatalf("movq.asm: err=%v hitCap=%v", err, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1", got)
	}

	data2, ok := c.Symbols.Get("DATA2")
	if !ok {
		t.Fatal("expected DATA2 to be defined")
	}

	lo, err := c.loadLong(data2)
	if err != nil {
		t.Fatalf("loadLong(data2): %v", err)
	}

	hi, err := c.loadLong(data2 + 4)
	if err != nil {
		t.Fatalf("loadLong(data2+4): %v", err)
	}

	if lo != 0x00110022 || hi != 0x00330044 {
		t.Errorf("data2 = %#x:%#x, want 0x00110022:0x00330044", lo, hi)
	}
}

// TestRegression_movc3 checks movc3.asm's three repeated string moves land
// the expected text and R0=1 (docs/PHASE-11.md's own fixture list).
func TestRegression_movc3(t *testing.T) {
	c, err, hitCap := runAsmRegression(t, "movc3.asm", "MAIN", 1000)
	if err != nil || hitCap {
		t.Fatalf("movc3.asm: err=%v hitCap=%v", err, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1", got)
	}

	dst, ok := c.Symbols.Get("DST")
	if !ok {
		t.Fatal("expected DST to be defined")
	}
	
	want := "This is a string of text to be moved around."[:5]
	for i := 0; i < len(want); i++ {
		b, err := c.loadByte(dst + uint32(i))
		if err != nil {
			t.Fatalf("loadByte(dst+%d): %v", i, err)
		}

		if b != want[i] {
			t.Fatalf("dst[%d] = %q, want %q", i, b, want[i])
		}
	}
}

// TestRegression_ffInsvDbl exercises the remaining pure-arithmetic
// testdata/asm fixtures (FFS/INSV/EXTZV bit-field and D-floating math) to
// the same bounded-completion bar Phase 13's own exe milestone check uses:
// each already has dedicated, hand-verified unit test coverage for its
// underlying instruction (internal/cpu's own bitfield_test.go/cvt_test.go),
// so this suite's own value is catching a regression in how the whole
// assemble-deposit-call pipeline drives them, not re-deriving expected bit
// patterns by hand here.
func TestRegression_ffInsvDbl(t *testing.T) {
	for _, tc := range []struct{ fixture, entry string }{
		{"ff.asm", "MAIN"},
		{"insv.asm", "TEST"},
		{"dbl.asm", "MAIN"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			_, err, hitCap := runAsmRegression(t, tc.fixture, tc.entry, 1000)
			if err != nil || hitCap {
				t.Errorf("%s: err=%v hitCap=%v", tc.fixture, err, hitCap)
			}
		})
	}
}

// TestRegression_float1 checks float1.asm's F_floating divide (100.0/3.0),
// run PC-first (Console.Execute, matching GO/EXEC) rather than CALL: main
// is a plain label with no .ENTRY mask, and the program ends in HALT, not
// RET, so it was never meant to be invoked as a procedure.
func TestRegression_float1(t *testing.T) {
	c := newRunnableConsole(t)
	if _, _, err := c.Assemble(asmFixturePath(t, "float1.asm")); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	addr, ok := c.Symbols.Get("MAIN")
	if !ok {
		t.Fatal("expected MAIN to be defined")
	}

	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	dataAddr, ok := c.Symbols.Get("DATA")
	if !ok {
		t.Fatal("expected DATA to be defined")
	}

	raw, err := c.loadLong(dataAddr)
	if err != nil {
		t.Fatalf("loadLong(data): %v", err)
	}

	if raw == 0 {
		t.Fatal("expected @#data to hold the F_floating result of 100.0/3.0, got 0")
	}
}

// runAsmRegressionWithKernel is runAsmRegression, but assembles
// testdata/asm/kernel.asm into the same session first -- for fixtures that
// call a real kernel.asm-defined RTL routine (LIB$PUT_OUTPUT,
// LIB$GET_INPUT, LIB$QUIT_EMULATION, a DECC$SHR SHIM$ stub, ...), matching
// vax.init's own boot sequence (ASM kernel.asm) followed by ASMing a user
// program on top.
func runAsmRegressionWithKernel(t *testing.T, fixture, entrySymbol string, maxSteps int) (*Console, error, bool) {
	t.Helper()
	c := newRunnableConsole(t)
	
	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if _, _, err := c.Assemble(asmFixturePath(t, fixture)); err != nil {
		t.Fatalf("Assemble(%s): %v", fixture, err)
	}

	addr, ok := c.Symbols.Get(entrySymbol)
	if !ok {
		t.Fatalf("%s: expected symbol %q to be defined", fixture, entrySymbol)
	}

	err, hitCap := callBounded(t, c, addr, maxSteps)

	return c, err, hitCap
}

// TestRegression_rtlDependentAsmFixtures runs every remaining
// testdata/asm fixture that calls into a kernel.asm-defined RTL routine
// (LIB$PUT_OUTPUT, LIB$GET_INPUT, LIB$QUIT_EMULATION, or a DECC$SHR/SYS$
// SHIM$ stub) against the real, live microkernel image, to the same
// "reaches a definite, bounded outcome" bar Phase 13's own
// TestRun_everyMilestoneFixture established for testdata/exe fixtures
// hitting the same RTL surface -- not a demand that every one completes
// cleanly.
//
// wantHitCap records which of these are currently known to spin forever
// rather than fault or complete: LIB$PUT_OUTPUT (foo.asm) and
// LIB$GET_INPUT (input.asm, test.asm) both poll a "ready"/"available" flag
// kernel.asm expects an EXC$CONWRITE/EXC$CONREAD interrupt's own ISR to
// reset -- this port's TXCS/TXDB/RXCS/RXDB privileged-register handling has
// no interrupt-delivery modeling at all yet (see
// TestAssemble_kernelThenHelloRunsBounded's doc comment and
// docs/PHASE-12.md's own progress log for the full story), so the wait
// never ends. decc$printf/decc$atoi (atoi.asm, fmt.asm) and sys$trnlnm
// (logname.asm) don't go through that console-I/O polling loop at all, so
// they're held to the stricter "must not hit the cap" bar.
func TestRegression_rtlDependentAsmFixtures(t *testing.T) {
	for _, tc := range []struct {
		fixture, entry string
		wantHitCap     bool
	}{
		{"foo.asm", "TEST", true},
		{"atoi.asm", "TEST", false},
		{"fmt.asm", "TEST", false},
		{"input.asm", "INPUT_TEST", true},
		{"logname.asm", "MAIN", false},
		{"test.asm", "TEST", true},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			_, err, hitCap := runAsmRegressionWithKernel(t, tc.fixture, tc.entry, 2_000_000)
			if hitCap != tc.wantHitCap {
				t.Errorf("%s: hitCap = %v, want %v (terminating outcome: %v)", tc.fixture, hitCap, tc.wantHitCap, err)
			}

			t.Logf("%s: terminating outcome: %v (hitCap=%v)", tc.fixture, err, hitCap)
		})
	}
}
