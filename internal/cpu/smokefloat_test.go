package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// shortFloatLit returns the short-literal addressing-mode byte for value,
// the inverse of decodeOperand's shortDouble[optype] lookup -- lets this
// test's program read in terms of the actual float values used (1.5, 2.5,
// 4.0) rather than pre-computed magic mode bytes.
func shortFloatLit(t *testing.T, value float64) byte {
	t.Helper()

	for i, v := range shortDouble {
		if v == value {
			return byte(i)
		}
	}

	t.Fatalf("shortFloatLit(%v): not exactly representable as a VAX float short literal", value)

	return 0
}

// TestSmokeFloatArithmeticConversionCompare is Phase 05's integration smoke
// test: a small hand-encoded VAX program exercising several of this phase's
// families together through Engine.Run -- data movement (MOVF/MOVD, both
// via short-literal operands), arithmetic (ADDF2), float->int and
// int->float conversion (CVTFL, CVTLD), compare (CMPD), a conditional
// branch on the result, and HALT.
//
// Equivalent to:
//
//	        MOVF   S^#1.5,R1        ; R1 = 1.5
//	        ADDF2  S^#2.5,R1        ; R1 = 4.0
//	        CVTFL  R1,R2            ; R2 = 4 (long)
//	        CVTLD  R2,R4            ; R4:R5 = 4.0 (double)
//	        MOVD   S^#4.0,R6        ; R6:R7 = 4.0 (double)
//	        CMPD   R4,R6            ; compare -- should be equal
//	        BEQL   SUCCESS
//	        MOVL   #0,R8            ; (not taken)
//	        BRB    DONE
//	SUCCESS:MOVL   #1,R8
//	DONE:   HALT
//
// which round-trips 1.5+2.5 through a float->int->double conversion and
// confirms it against a freshly-loaded double 4.0, leaving R8 = 1 only if
// every step along the way agreed.
func TestSmokeFloatArithmeticConversionCompare(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	lit15 := shortFloatLit(t, 1.5)
	lit25 := shortFloatLit(t, 2.5)
	lit4 := shortFloatLit(t, 4.0)

	var prog []byte

	emit := func(b ...byte) { prog = append(prog, b...) }

	emit(0x50, lit15, regMode(vax.R1))           // MOVF S^#1.5,R1
	emit(0x40, lit25, regMode(vax.R1))           // ADDF2 S^#2.5,R1
	emit(0x4A, regMode(vax.R1), regMode(vax.R2)) // CVTFL R1,R2
	emit(0x6E, regMode(vax.R2), regMode(vax.R4)) // CVTLD R2,R4
	emit(0x70, lit4, regMode(vax.R6))            // MOVD S^#4.0,R6
	emit(0x71, regMode(vax.R4), regMode(vax.R6)) // CMPD R4,R6

	beqlAt := len(prog)

	emit(0x13, 0x00) // BEQL SUCCESS (patched below)
	emit(0xD0, 0x00, regMode(vax.R8)) // MOVL #0,R8 (not taken)

	brbAt := len(prog)

	emit(0x11, 0x00) // BRB DONE (patched below)

	successAt := len(prog)

	emit(0xD0, 0x01, regMode(vax.R8)) // SUCCESS: MOVL #1,R8

	doneAt := len(prog)
	
	emit(0x00) // DONE: HALT

	prog[beqlAt+1] = byte(int8(successAt - (beqlAt + 2)))
	prog[brbAt+1] = byte(int8(doneAt - (brbAt + 2)))

	putBytes(t, cpu, mem, base, prog...)
	cpu.SetGPR(vax.PC, base)

	err := e.Run()
	if !errors.Is(err, ErrHalted) {
		t.Fatalf("Run() = %v, want ErrHalted", err)
	}

	if got := getFloatReg(t, cpu, 4, vax.R1); got != 4.0 {
		t.Errorf("R1 = %v, want 4.0 (1.5 + 2.5)", got)
	}

	if got := int32(cpu.GPR(vax.R2)); got != 4 {
		t.Errorf("R2 = %d, want 4", got)
	}

	if got := getFloatReg(t, cpu, 8, vax.R4); got != 4.0 {
		t.Errorf("R4:R5 = %v, want 4.0", got)
	}

	if got := getFloatReg(t, cpu, 8, vax.R6); got != 4.0 {
		t.Errorf("R6:R7 = %v, want 4.0", got)
	}
	// Z isn't checked here: it reflects CMPD's result only until the next
	// instruction that writes it, and the SUCCESS path's own MOVL #1,R8
	// (which sets Z from the moved value, 1, i.e. false) runs after it --
	// R8 == 1 below already confirms BEQL read CMPD's Z correctly at the
	// time it mattered.
	if got := cpu.GPR(vax.R8); got != 1 {
		t.Errorf("R8 = %d, want 1 (BEQL taken -- every conversion/compare step agreed)", got)
	}
	
	if got := cpu.GPR(vax.PC); got != base+uint32(doneAt)+1 {
		t.Errorf("PC = %#x, want %#x (address after HALT)", got, base+uint32(doneAt)+1)
	}
}
