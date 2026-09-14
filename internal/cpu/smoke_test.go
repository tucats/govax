package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestSmokeSumLoop is Phase 04's integration smoke test: a small hand-
// encoded VAX program (no assembler yet -- Phase 11) exercising decode and
// execute end-to-end across several of this phase's instruction families
// -- data movement (CLRL, MOVL with a short-literal immediate), integer
// arithmetic (ADDL2), loop control (SOBGTR), and control (HALT) -- run
// through Engine.Run rather than individual Step calls or direct handler
// invocations the way the rest of this phase's tests do.
//
// Equivalent to:
//
//	        CLRL   R0          ; sum = 0
//	        MOVL   #5,R1       ; counter = 5
//	LOOP:   ADDL2  R1,R0       ; sum += counter
//	        SOBGTR R1,LOOP     ; counter--; branch if counter > 0
//	        HALT
//
// which sums 5+4+3+2+1 = 15 into R0, leaving R1 at 0.
func TestSmokeSumLoop(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	putBytes(t, cpu, mem, base,
		0xD4, 0x50, // CLRL R0
		0xD0, 0x05, 0x51, // MOVL S^#5,R1
		0xC0, 0x51, 0x50, // LOOP: ADDL2 R1,R0
		0xF5, 0x51, 0xFA, // SOBGTR R1,LOOP (-6)
		0x00, // HALT
	)
	cpu.SetGPR(vax.PC, base)

	err := e.Run()
	if !errors.Is(err, ErrHalted) {
		t.Fatalf("Run() = %v, want ErrHalted", err)
	}
	if !e.Halted() {
		t.Error("Halted() = false, want true")
	}
	if got := cpu.GPR(vax.R0); got != 15 {
		t.Errorf("R0 = %d, want 15 (5+4+3+2+1)", got)
	}
	if got := cpu.GPR(vax.R1); got != 0 {
		t.Errorf("R1 = %d, want 0", got)
	}
	if got := cpu.GPR(vax.PC); got != base+12 {
		t.Errorf("PC = %#x, want %#x (address after HALT)", got, base+12)
	}
}
