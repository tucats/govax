package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

func TestEmulBpt(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcBreakpoint, 0x300, 0)

	stepInstruction(t, e, 0x03) // BPT

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (breakpoint fault vector)", got)
	}
}

func TestEmulBugl(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcPrivileged, 0x300, 0)

	// BUGL #0x1234 -- extended opcode 0xFF 0xFD, 4-byte implicit immediate.
	stepInstruction(t, e, 0xFF, 0xFD, 0x34, 0x12, 0x00, 0x00)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (privileged-instruction fault vector)", got)
	}
}

func TestEmulIndexInRange(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.R0, 3) // subscript
	cpu.SetGPR(vax.R1, 0) // low
	cpu.SetGPR(vax.R2, 9) // high
	cpu.SetGPR(vax.R3, 4) // size
	cpu.SetGPR(vax.R4, 0) // in

	stepInstruction(t, e, 0x0A,
		regMode(vax.R0), regMode(vax.R1), regMode(vax.R2),
		regMode(vax.R3), regMode(vax.R4), regMode(vax.R5))

	if got := cpu.GPR(vax.R5); got != 12 { // (0+3)*4
		t.Errorf("out = %d, want 12", got)
	}

	if cpu.PSL().N() || cpu.PSL().Z() {
		t.Error("N/Z set, want both clear for a positive nonzero result")
	}
}

func TestEmulIndexOutOfRangeFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcArithmetic, 0x300, 0)

	cpu.SetGPR(vax.R0, 20) // subscript, out of [0,9]
	cpu.SetGPR(vax.R1, 0)
	cpu.SetGPR(vax.R2, 9)
	cpu.SetGPR(vax.R3, 4)
	cpu.SetGPR(vax.R4, 0)
	cpu.SetGPR(vax.R5, 0xDEADBEEF)

	stepInstruction(t, e, 0x0A,
		regMode(vax.R0), regMode(vax.R1), regMode(vax.R2),
		regMode(vax.R3), regMode(vax.R4), regMode(vax.R5))

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (arithmetic fault vector)", got)
	}

	if got := cpu.GPR(vax.R5); got != 0xDEADBEEF {
		t.Errorf("R5 = %#x, want unchanged 0xDEADBEEF (not stored on fault)", got)
	}
}

func TestEmulBispswBicpsw(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	// #0x40 (pslFU) exceeds the short-literal range (0-63), so use I^#0x40
	// (a word-sized immediate) instead.
	stepInstruction(t, e, 0xB8, 0x8F, 0x40, 0x00) // BISPSW #0x40 -> sets FU

	if !cpu.PSL().FU() {
		t.Fatal("FU = false after BISPSW #0x40, want true")
	}

	stepInstruction(t, e, 0xB9, 0x8F, 0x40, 0x00) // BICPSW #0x40 -> clears FU
	
	if cpu.PSL().FU() {
		t.Error("FU = true after BICPSW #0x40, want false")
	}
}

func TestEmulBitpswReservedBitsFault(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	// I^#0x0100: bit 8 set, which must be zero.
	stepInstruction(t, e, 0xB8, 0x8F, 0x00, 0x01, 0x00, 0x00)

	if got := cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulPushrPoprRoundTrip(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.SP, 0x9000)
	cpu.SetGPR(vax.R0, 0x11111111)
	cpu.SetGPR(vax.R2, 0x22222222)
	cpu.SetGPR(vax.R5, 0x55555555)

	// PUSHR^M #^M<R0,R2,R5>: mask bits 0, 2, 5.
	const mask = 1<<0 | 1<<2 | 1<<5

	stepInstruction(t, e, 0xBB, shortLiteral(mask))

	if got := cpu.GPR(vax.SP); got != 0x9000-12 {
		t.Fatalf("SP after PUSHR = %#x, want %#x", got, uint32(0x9000-12))
	}

	cpu.SetGPR(vax.R0, 0)
	cpu.SetGPR(vax.R2, 0)
	cpu.SetGPR(vax.R5, 0)

	stepInstruction(t, e, 0xBA, shortLiteral(mask))

	if got := cpu.GPR(vax.SP); got != 0x9000 {
		t.Errorf("SP after POPR = %#x, want 0x9000", got)
	}

	if got := cpu.GPR(vax.R0); got != 0x11111111 {
		t.Errorf("R0 = %#x, want 0x11111111", got)
	}

	if got := cpu.GPR(vax.R2); got != 0x22222222 {
		t.Errorf("R2 = %#x, want 0x22222222", got)
	}

	if got := cpu.GPR(vax.R5); got != 0x55555555 {
		t.Errorf("R5 = %#x, want 0x55555555", got)
	}
}

func TestEmulProberAccessibleWithVMDisabled(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.SP, 0)
	psl := cpu.PSL()
	psl.SetC(true) // PROBEx must leave C unaffected
	cpu.SetPSL(psl)

	bytes := []byte{0x0C, shortLiteral(0), shortLiteral(4)}
	bytes = append(bytes, absoluteMode(0x2000)...)
	stepInstruction(t, e, bytes...)

	if cpu.PSL().Z() {
		t.Error("Z = true, want false (accessible with MAPEN disabled)")
	}

	if cpu.PSL().N() {
		t.Error("N = true, want false")
	}

	if !cpu.PSL().C() {
		t.Error("C = false, want true (PROBEx doesn't touch C)")
	}
}

// TestEmulProberNotAccessibleSetsZWithoutFaulting runs the PROBER
// instruction itself out of mapped S0 memory (page 0, backed by a valid
// PTE) while probing a *different* S0 page (page 1, in range per SLR but
// with no valid PTE) -- so decode succeeds normally and only the probed
// address is inaccessible, exercising Translate's real failure path rather
// than relying on MAPEN being off.
func TestEmulProberNotAccessibleSetsZWithoutFaulting(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const sbr = 0x3000   // physical: the (two-entry) S0 page table

	const codePFN = 0x10 // page 0 -> physical 0x2000

	const codeVA = 0x80000000

	const codePhys = 0x2000

	const probeTargetVA = 0x80000200 // S0 page 1: in range, no valid PTE

	var pte vm.PTE
	
	pte.SetValid(true)
	pte.SetProtection(vm.ProtKW)
	pte.SetPFN(codePFN)
	putLongword(t, cpu, mem, sbr, uint32(pte)) // page 0's PTE
	putLongword(t, cpu, mem, sbr+4, 0)         // page 1's PTE: invalid

	bytes := []byte{0x0C, shortLiteral(0), shortLiteral(4)}
	bytes = append(bytes, absoluteMode(probeTargetVA)...)
	putBytes(t, cpu, mem, codePhys, bytes...)

	cpu.SetPR(vax.SBR, sbr)
	cpu.SetPR(vax.SLR, 1) // pages 0 and 1 in range
	cpu.SetGPR(vax.PC, codeVA)
	cpu.SetPR(vax.MAPEN, 1)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true (not accessible)")
	}

	if got := cpu.GPR(vax.PC); got != codeVA+uint32(len(bytes)) {
		t.Errorf("PC = %#x, want the next instruction (PROBEx never signals a real fault)", got)
	}
}

func TestEmulMovpsl(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	psl := cpu.PSL()
	psl.SetN(true)
	psl.SetIPL(5)
	cpu.SetPSL(psl)

	stepInstruction(t, e, 0xDC, regMode(vax.R1))

	if got := cpu.GPR(vax.R1); got != uint32(psl) {
		t.Errorf("R1 = %#x, want %#x (the whole PSL)", got, uint32(psl))
	}
}
