package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// Tests for F and D on the floating core (docs/PHASE-35.md, subtask 5).
// Each would have failed when the CPU held F and D values as float64s.
// Where VMS's run of the Phase 35 probes (testdata/insn35) has the same
// case, the expected bits are VMS's.

// runFloat decodes and runs one instruction at base, returning the
// handler's error (a fault or trap) rather than delivering it, so a test
// can look at it.
func runFloat(t *testing.T, e *Engine, bytes ...byte) error {
	t.Helper()

	putBytes(t, e.cpu, e.mem, base, bytes...)
	e.cpu.SetGPR(vax.PC, base)

	d, err := decodeInstructionValue(e.cpu, e.mem, instructionTable)
	if err != nil {
		return err
	}

	e.cpu.SetGPR(vax.PC, d.NextPC)
	e.instructionPC = base

	return instructionTable.HandlerFor(d.Instruction)(e, &d)
}

// setD puts D_floating bits, given as the four words from the most
// significant, in r and r+1.
func setD(cpu *vax.CPU, r vax.Reg, w0, w1, w2, w3 uint16) {
	cpu.SetGPR(r, uint32(w1)<<16|uint32(w0))
	cpu.SetGPR(r+1, uint32(w3)<<16|uint32(w2))
}

// TestMOVDKeepsLowBits: MOVD of a value with all 55 fraction bits set
// (1 - 2^-56) stores it unchanged. As float64 the low 3 bits were lost.
func TestMOVDKeepsLowBits(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setD(cpu, vax.R1, 0x407F, 0xFFFF, 0xFFFF, 0xFFFF)

	if err := runFloat(t, e, 0x70, regMode(vax.R1), regMode(vax.R3)); err != nil { // MOVD R1,R3
		t.Fatal(err)
	}

	if cpu.GPR(vax.R3) != 0xFFFF407F || cpu.GPR(vax.R4) != 0xFFFFFFFF {
		t.Errorf("R3:R4 = %#x:%#x, want 0xffff407f:0xffffffff", cpu.GPR(vax.R3), cpu.GPR(vax.R4))
	}
}

// TestADDFTieRoundsAway: 1 + 2^-24 is half way between 1 and 1 + 2^-23;
// the VAX rounds away from zero (VMS stored ^X4080, ^X0001), where IEEE's
// ties-to-even gave 1.
func TestADDFTieRoundsAway(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x00003480) // 2^-24
	cpu.SetGPR(vax.R2, 0x00004080) // 1.0

	if err := runFloat(t, e, 0x40, regMode(vax.R1), regMode(vax.R2)); err != nil { // ADDF2 R1,R2
		t.Fatal(err)
	}

	if got := cpu.GPR(vax.R2); got != 0x00014080 {
		t.Errorf("R2 = %#x, want 0x14080 (1 + 2^-23)", got)
	}
}

// TestDIVDLowBits: 1/3 in D_floating rounds up in its last bit: ^X3FAA,
// ^XAAAA, ^XAAAA, ^XAAAB (VMS's DIVD3 1/3).
func TestDIVDLowBits(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setD(cpu, vax.R1, 0x4140, 0, 0, 0) // 3.0
	setD(cpu, vax.R3, 0x4080, 0, 0, 0) // 1.0

	if err := runFloat(t, e, 0x67, regMode(vax.R1), regMode(vax.R3), regMode(vax.R5)); err != nil { // DIVD3 R1,R3,R5
		t.Fatal(err)
	}

	if cpu.GPR(vax.R5) != 0xAAAA3FAA || cpu.GPR(vax.R6) != 0xAAABAAAA {
		t.Errorf("R5:R6 = %#x:%#x, want 0xaaaa3faa:0xaaabaaaa", cpu.GPR(vax.R5), cpu.GPR(vax.R6))
	}
}

// TestCVTFDAndCVTDF: CVTFD is exact; CVTDF rounds half away from zero
// (1 + 2^-24 in D becomes 1 + 2^-23 in F), and overflows when D's largest
// value rounds up past F's.
func TestCVTFDAndCVTDF(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xAAAB3EAA) // 1/12 in F: ^X3EAA, ^XAAAB

	if err := runFloat(t, e, 0x56, regMode(vax.R1), regMode(vax.R2)); err != nil { // CVTFD R1,R2
		t.Fatal(err)
	}

	if cpu.GPR(vax.R2) != 0xAAAB3EAA || cpu.GPR(vax.R3) != 0 {
		t.Errorf("CVTFD: R2:R3 = %#x:%#x, want 0xaaab3eaa:0", cpu.GPR(vax.R2), cpu.GPR(vax.R3))
	}

	setD(cpu, vax.R4, 0x4080, 0x0000, 0x8000, 0x0000) // 1 + 2^-24

	if err := runFloat(t, e, 0x76, regMode(vax.R4), regMode(vax.R6)); err != nil { // CVTDF R4,R6
		t.Fatal(err)
	}

	if got := cpu.GPR(vax.R6); got != 0x00014080 {
		t.Errorf("CVTDF tie: R6 = %#x, want 0x14080", got)
	}

	setD(cpu, vax.R4, 0x7FFF, 0xFFFF, 0xFFFF, 0xFFFF) // D's largest value

	err := runFloat(t, e, 0x76, regMode(vax.R4), regMode(vax.R6))

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcArithmetic || len(f.Args) != 1 || f.Args[0] != faultFltOvf {
		t.Errorf("CVTDF of D's max: %v, want a floating overflow fault", err)
	}
}

// TestDIVFByZeroFault: the floating divide-by-zero fault (type 9,
// SS$_FLTDIV_F, as VMS signalled), not overflow.
func TestDIVFByZeroFault(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0)
	cpu.SetGPR(vax.R2, 0x00004080)

	err := runFloat(t, e, 0x46, regMode(vax.R1), regMode(vax.R2)) // DIVF2 R1,R2

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcArithmetic || len(f.Args) != 1 || f.Args[0] != faultFltDiv {
		t.Errorf("DIVF2 by zero: %v, want a floating divide-by-zero fault", err)
	}

	if cpu.GPR(vax.R2) != 0x00004080 {
		t.Errorf("R2 = %#x, want it unchanged", cpu.GPR(vax.R2))
	}
}

// TestMNEGFClearsC: the manual's MNEG clears C (MOV leaves it alone).
func TestMNEGFClearsC(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x00004080)

	setC(cpu, true)

	if err := runFloat(t, e, 0x52, regMode(vax.R1), regMode(vax.R2)); err != nil { // MNEGF R1,R2
		t.Fatal(err)
	}

	if cpu.GPR(vax.R2) != 0x0000C080 || cpu.PSL().C() || !cpu.PSL().N() {
		t.Errorf("MNEGF: R2 = %#x, PSL %+v; want 0xc080 with N set and C clear", cpu.GPR(vax.R2), cpu.PSL())
	}

	setC(cpu, true)

	if err := runFloat(t, e, 0x50, regMode(vax.R1), regMode(vax.R2)); err != nil { // MOVF R1,R2
		t.Fatal(err)
	}

	if !cpu.PSL().C() {
		t.Error("MOVF cleared C, want it unaffected")
	}
}

// TestFloatUnderflow: with PSL<FU> clear an underflowing result is zero
// (Z set); with it set, it's a floating underflow fault and the
// destination is unchanged.
func TestFloatUnderflow(t *testing.T) {
	for _, fu := range []bool{false, true} {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		cpu.SetGPR(vax.R1, 0x00000080) // F's smallest value, 2^-128
		cpu.SetGPR(vax.R2, 0x00004000) // 0.5

		psl := cpu.PSL()
		psl.SetFU(fu)
		cpu.SetPSL(psl)

		err := runFloat(t, e, 0x45, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // MULF3 R1,R2,R3

		var f *Fault

		switch {
		case !fu && (err != nil || cpu.GPR(vax.R3) != 0 || !cpu.PSL().Z()):
			t.Errorf("FU clear: err %v, R3 %#x, Z %v; want zero stored, Z set", err, cpu.GPR(vax.R3), cpu.PSL().Z())
		case fu && (!errors.As(err, &f) || f.Args[0] != faultFltUnd):
			t.Errorf("FU set: err %v, want a floating underflow fault", err)
		}
	}
}

// TestEMODFExtensionIsAnInteger: EMODF's second operand, the extension
// byte, is an integer even though the instruction's others are floating,
// so a short literal there is the integer 5, not the floating literal 5
// (whose F bits are ^X4120). Before Phase 35 a short literal took its type
// from the instruction as a whole.
func TestEMODFExtensionIsAnInteger(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, base, 0x54, 0x08, 0x05, 0x08, regMode(vax.R1), regMode(vax.R2)) // EMODF S^#1.0,S^#5,S^#1.0,R1,R2
	cpu.SetGPR(vax.PC, base)

	d, err := decodeInstructionValue(cpu, mem, instructionTable)
	if err != nil {
		t.Fatal(err)
	}

	if d.Operands[1].Value != 5 {
		t.Errorf("EMODF's extension literal = %#x, want 5", d.Operands[1].Value)
	}

	if d.Operands[0].Value != 0x4080 {
		t.Errorf("EMODF's multiplier literal = %#x, want 0x4080 (1.0 in F)", d.Operands[0].Value)
	}
}

// setH puts H_floating bits, given as the eight words from the most
// significant, in r through r+3.
func setH(cpu *vax.CPU, r vax.Reg, w [8]uint16) {
	for i := 0; i < 4; i++ {
		cpu.SetGPR(r+vax.Reg(i), uint32(w[2*i+1])<<16|uint32(w[2*i]))
	}
}

// TestHFloating checks H_floating through registers (an H value spans
// four): a tie in ADDH rounds away from zero, CVTGH widens exactly, and
// CVTHG of a value beyond G's range overflows.
func TestHFloating(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	// 1.0 is ^X4001, then zeros; 2^-113 (half a unit in the last place
	// at 1.0) is exponent 1 - 113 + 16384 = ^X3F90.
	setH(cpu, vax.R0, [8]uint16{0x4001})
	setH(cpu, vax.R4, [8]uint16{0x3F90})

	if err := runFloat(t, e, 0xFD, 0x60, regMode(vax.R4), regMode(vax.R0)); err != nil { // ADDH2 R4,R0
		t.Fatal(err)
	}

	want := [4]uint32{0x00004001, 0, 0, 0x00010000} // 1 + 2^-112
	for i, w := range want {
		if got := cpu.GPR(vax.R0 + vax.Reg(i)); got != w {
			t.Errorf("ADDH tie: R%d = %#x, want %#x", i, got, w)
		}
	}

	// CVTGH of G's 1/3 (^X3FF5, ^X5555, ^X5555, ^X5555: exponent ^X3FF,
	// fraction 0101...) gives H's ^X3FFF, ^X5555, ^X5555, ^X5555, ^X5000,
	// then zeros: G's 52 fraction bits, exactly, and H's other 60 zero.
	cpu.SetGPR(vax.R2, 0x55553FF5)
	cpu.SetGPR(vax.R3, 0x55555555)

	if err := runFloat(t, e, 0xFD, 0x56, regMode(vax.R2), regMode(vax.R6)); err != nil { // CVTGH R2,R6
		t.Fatal(err)
	}

	for i, w := range [4]uint32{0x55553FFF, 0x55555555, 0x00005000, 0} {
		if got := cpu.GPR(vax.R6 + vax.Reg(i)); got != w {
			t.Errorf("CVTGH: R%d = %#x, want %#x", 6+i, got, w)
		}
	}

	// 2^2000: exponent ^X47D1, beyond G's (at most 2^1023).
	setH(cpu, vax.R0, [8]uint16{0x47D1})

	err := runFloat(t, e, 0xFD, 0x76, regMode(vax.R0), regMode(vax.R4)) // CVTHG R0,R4

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcArithmetic || f.Args[0] != faultFltOvf {
		t.Errorf("CVTHG of 2^2000: %v, want a floating overflow fault", err)
	}
}

// TestPOLYFManualExample runs the manual's POLY example: P(x) = 1.0 +
// 0.5x + 0.25x^2 with the table C2, C1, C0 = 0.25, 0.5, 1.0; at x = 2 it
// is 3. R1 and R2 are zero and R3 points just past the table. A degree
// over 31 is a reserved operand.
func TestPOLYFManualExample(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const table = 0x3000
	for i, v := range []uint32{0x00003F80, 0x00004000, 0x00004080} { // 0.25, 0.5, 1.0
		if err := mem.StoreLongword(cpu, table+uint32(4*i), v); err != nil {
			t.Fatal(err)
		}
	}

	cpu.SetGPR(vax.R6, 0x00004100) // x = 2.0
	cpu.SetGPR(vax.R7, table)
	cpu.SetGPR(vax.R1, 0x11111111)
	cpu.SetGPR(vax.R2, 0x22222222)

	if err := runFloat(t, e, 0x55, regMode(vax.R6), 0x02, 0x67); err != nil { // POLYF R6,#2,(R7)
		t.Fatal(err)
	}

	for r, want := range map[vax.Reg]uint32{vax.R0: 0x00004140, vax.R1: 0, vax.R2: 0, vax.R3: table + 12} {
		if got := cpu.GPR(r); got != want {
			t.Errorf("R%d = %#x, want %#x", r, got, want)
		}
	}

	err := runFloat(t, e, 0x55, regMode(vax.R6), 0x8F, 0x20, 0x00, 0x67) // POLYF R6,#32,(R7)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcReservedOp {
		t.Errorf("degree 32: %v, want a reserved operand fault", err)
	}
}
