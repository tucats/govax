package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// trapVector is where these tests' arithmetic exception vector points.
const trapVector = 0x300

// newTrapEngine returns an engine with a kernel stack and an arithmetic
// exception vector, and PSL<IV> set as iv says.
func newTrapEngine(t *testing.T, iv bool) *Engine {
	t.Helper()

	e := newEngine()
	e.cpu.SetGPR(registers.SP, 0x7000)
	e.cpu.SetPR(registers.KSP, 0x7000)
	putVector(t, e, ExcArithmetic, trapVector, 0)

	psl := e.cpu.PSL()
	psl.SetIV(iv)
	e.cpu.SetPSL(psl)

	return e
}

// trapTaken reports whether the instruction just stepped took an
// arithmetic exception, and if it did, the exception's type code and the
// PC it saved (the exception frame: the type code, then the PC, then the
// PSL).
func trapTaken(t *testing.T, e *Engine) (taken bool, code, pc uint32) {
	t.Helper()

	if e.cpu.GPR(registers.PC) != trapVector {
		return false, 0, 0
	}

	sp := e.cpu.GPR(registers.SP)

	return true, mustLongword(t, e.cpu, e.mem, sp), mustLongword(t, e.cpu, e.mem, sp+4)
}

// TestIntegerOverflowTrap checks the integer overflow trap: with IV set,
// an instruction whose result overflowed stores it, sets the condition
// codes, and traps with type 1 and the next instruction's PC; with IV
// clear, or without overflow, there's no trap.
func TestIntegerOverflowTrap(t *testing.T) {
	addl2 := []byte{0xC0, 0x01, regMode(registers.R0)} // ADDL2 #1,R0

	cases := []struct {
		name string
		iv   bool
		r0   uint32
		trap bool
	}{
		{"overflow, IV set", true, 0x7FFFFFFF, true},
		{"overflow, IV clear", false, 0x7FFFFFFF, false},
		{"no overflow, IV set", true, 5, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTrapEngine(t, tc.iv)
			e.cpu.SetGPR(registers.R0, tc.r0)
			stepInstruction(t, e, addl2...)

			if got, want := e.cpu.GPR(registers.R0), tc.r0+1; got != want {
				t.Errorf("R0 = %#x, want %#x (stored either way)", got, want)
			}

			taken, code, pc := trapTaken(t, e)
			if taken != tc.trap {
				t.Fatalf("trap taken = %v, want %v", taken, tc.trap)
			}

			if !taken {
				return
			}

			if code != trapIntOvf {
				t.Errorf("type code = %d, want %d (integer overflow)", code, trapIntOvf)
			}

			if want := uint32(base + len(addl2)); pc != want {
				t.Errorf("saved PC = %#x, want %#x (the next instruction's)", pc, want)
			}
		})
	}
}

// TestIntegerOverflowTrapAfterBranch checks that a loop instruction's
// trap comes after its branch: SOBGTR of the largest negative longword
// overflows to the largest positive one, so it branches, and the PC saved
// is the branch's target.
func TestIntegerOverflowTrapAfterBranch(t *testing.T) {
	e := newTrapEngine(t, true)
	e.cpu.SetGPR(registers.R0, 0x80000000)
	stepInstruction(t, e, 0xF5, regMode(registers.R0), 0x10) // SOBGTR R0,.+3+16

	if got := e.cpu.GPR(registers.R0); got != 0x7FFFFFFF {
		t.Errorf("R0 = %#x, want 0x7FFFFFFF", got)
	}

	taken, code, pc := trapTaken(t, e)
	if !taken || code != trapIntOvf {
		t.Fatalf("trap = %v, type %d; want an integer overflow trap", taken, code)
	}

	if want := uint32(base + 3 + 0x10); pc != want {
		t.Errorf("saved PC = %#x, want %#x (the branch's target)", pc, want)
	}
}

// TestIntegerOverflowNotForOtherV checks that an instruction whose V
// isn't an integer overflow doesn't trap: REMQUE of an empty queue sets V
// to say so.
func TestIntegerOverflowNotForOtherV(t *testing.T) {
	e := newTrapEngine(t, true)

	// An empty queue: its header at 0x4000 points to itself both ways.
	putLongword(t, e.cpu, e.mem, 0x4000, 0x4000)
	putLongword(t, e.cpu, e.mem, 0x4004, 0x4000)

	stepInstruction(t, e, 0x0F, 0x9F, 0x00, 0x40, 0x00, 0x00, regMode(registers.R1)) // REMQUE @#^X4000,R1

	if !e.cpu.PSL().V() {
		t.Fatal("V = false, want true (the queue was empty)")
	}

	if taken, _, _ := trapTaken(t, e); taken {
		t.Error("REMQUE's V took an integer overflow trap")
	}
}

// TestEmulDivByZeroTraps checks the integer divide-by-zero trap: whatever
// IV is, a zero divisor stores the dividend, sets V, and traps with type 2
// and the next instruction's PC.
func TestEmulDivByZeroTraps(t *testing.T) {
	for _, iv := range []bool{false, true} {
		e := newTrapEngine(t, iv)
		e.cpu.SetGPR(registers.R1, 0)  // divisor: zero
		e.cpu.SetGPR(registers.R2, 10) // dividend

		stepInstruction(t, e, 0xC6, regMode(registers.R1), regMode(registers.R2)) // DIVL2 R1,R2

		if got := e.cpu.GPR(registers.R2); got != 10 {
			t.Errorf("IV %v: result = %d, want 10 (the dividend)", iv, got)
		}

		if !e.cpu.PSL().V() {
			t.Errorf("IV %v: V = false, want true (divide by zero)", iv)
		}

		taken, code, pc := trapTaken(t, e)
		if !taken || code != trapIntDiv {
			t.Errorf("IV %v: trap = %v, type %d; want an integer divide-by-zero trap", iv, taken, code)
		}

		if pc != base+3 {
			t.Errorf("IV %v: saved PC = %#x, want %#x (the next instruction's)", iv, pc, base+3)
		}
	}
}

// TestEmulDivMinIntOverflowTraps checks DIV's other V, the largest
// negative number divided by -1: an integer overflow, trapped only under
// IV.
func TestEmulDivMinIntOverflowTraps(t *testing.T) {
	for _, iv := range []bool{false, true} {
		e := newTrapEngine(t, iv)
		e.cpu.SetGPR(registers.R1, 0xFF) // divisor: -1
		e.cpu.SetGPR(registers.R2, 0x80) // dividend: -128

		stepInstruction(t, e, 0x86, regMode(registers.R1), regMode(registers.R2)) // DIVB2 R1,R2

		taken, code, _ := trapTaken(t, e)
		if taken != iv || (taken && code != trapIntOvf) {
			t.Errorf("IV %v: trap = %v, type %d; want an integer overflow trap only under IV", iv, taken, code)
		}
	}
}

// TestEmulEdivByZeroTraps checks EDIV's divide by zero: Note 3's results
// (the dividend's low longword, a zero remainder, V), then the trap.
func TestEmulEdivByZeroTraps(t *testing.T) {
	e := newTrapEngine(t, false)
	e.cpu.SetGPR(registers.R1, 0) // divisor
	e.cpu.SetGPR(registers.R4, 0x23456789)
	e.cpu.SetGPR(registers.R5, 0x00000001) // dividend R4/R5

	stepInstruction(t, e, 0x7B, regMode(registers.R1), regMode(registers.R4),
		regMode(registers.R2), regMode(registers.R3)) // EDIV R1,R4,R2,R3

	if got := e.cpu.GPR(registers.R2); got != 0x23456789 {
		t.Errorf("quotient = %#x, want 0x23456789 (bits 31:0 of the dividend)", got)
	}

	if got := e.cpu.GPR(registers.R3); got != 0 {
		t.Errorf("remainder = %#x, want 0", got)
	}

	taken, code, pc := trapTaken(t, e)
	if !taken || code != trapIntDiv {
		t.Fatalf("trap = %v, type %d; want an integer divide-by-zero trap", taken, code)
	}

	if pc != base+5 {
		t.Errorf("saved PC = %#x, want %#x (the next instruction's)", pc, base+5)
	}
}

// TestEmulIndexOutOfRangeTraps checks INDEX's subscript range trap (the
// architecture manual's E.1.7): the index is stored and the condition
// codes set as if the subscript were in range, then the trap, type 7,
// saves the next instruction's PC.
func TestEmulIndexOutOfRangeTraps(t *testing.T) {
	e := newTrapEngine(t, false)
	e.cpu.SetGPR(registers.R0, 20) // subscript, out of [0,9]
	e.cpu.SetGPR(registers.R1, 0)
	e.cpu.SetGPR(registers.R2, 9)
	e.cpu.SetGPR(registers.R3, 4)
	e.cpu.SetGPR(registers.R4, 0)
	e.cpu.SetGPR(registers.R5, 0xDEADBEEF)

	stepInstruction(t, e, 0x0A,
		regMode(registers.R0), regMode(registers.R1), regMode(registers.R2),
		regMode(registers.R3), regMode(registers.R4), regMode(registers.R5))

	if got := e.cpu.GPR(registers.R5); got != 80 {
		t.Errorf("R5 = %#x, want 80 ((0+20)*4, stored before the trap)", got)
	}

	taken, code, pc := trapTaken(t, e)
	if !taken || code != trapSubRng {
		t.Fatalf("trap = %v, type %d; want a subscript range trap", taken, code)
	}

	if pc != base+7 {
		t.Errorf("saved PC = %#x, want %#x (the next instruction's)", pc, base+7)
	}
}
