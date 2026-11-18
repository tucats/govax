package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

func TestEmulCmp(t *testing.T) {
	cases := []struct {
		name                       string
		src1, src2                 uint32
		wantN, wantZ, wantV, wantC bool
	}{
		{"equal", 5, 5, false, true, false, false},
		{"src1 less (signed)", 3, 5, true, false, false, true},
		{"src1 greater (signed)", 5, 3, false, false, false, false},
		// -1 (0xFF) is signed-less than 1, but unsigned-greater: N and C
		// should disagree, and V must stay 0 regardless.
		{"signed/unsigned disagree", 0xFF, 1, true, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(registers.R1, tc.src1)
			cpu.SetGPR(registers.R2, tc.src2)

			stepInstruction(t, e, 0x91, regMode(registers.R1), regMode(registers.R2)) // CMPB

			if cpu.GPR(registers.R1) != tc.src1 || cpu.GPR(registers.R2) != tc.src2 {
				t.Error("CMP modified an operand")
			}

			psl := cpu.PSL()
			if psl.N() != tc.wantN || psl.Z() != tc.wantZ || psl.V() != tc.wantV || psl.C() != tc.wantC {
				t.Errorf("N=%v Z=%v V=%v C=%v, want N=%v Z=%v V=%v C=%v",
					psl.N(), psl.Z(), psl.V(), psl.C(), tc.wantN, tc.wantZ, tc.wantV, tc.wantC)
			}
		})
	}
}

// TestEmulCmpOverflowingDifference holds N to the signed comparison where
// src1 - src2 overflows: 3 is greater than -2147483648 (MAXVAL's starting value
// in a FORTRAN reduction), though their difference's sign bit is set, and ^X7F
// greater than ^X80 as bytes; C, unsigned, says the opposite in both.
func TestEmulCmpOverflowingDifference(t *testing.T) {
	cases := []struct {
		name         string
		opcode       byte
		src1, src2   uint32
		wantN, wantC bool
	}{
		{"CMPL 3, -2147483648", 0xD1, 3, 0x80000000, false, true},
		{"CMPL -2147483648, 3", 0xD1, 0x80000000, 3, true, false},
		{"CMPL 2147483647, -1", 0xD1, 0x7FFFFFFF, 0xFFFFFFFF, false, true},
		{"CMPW 32767, -32768", 0xB1, 0x7FFF, 0x8000, false, true},
		{"CMPB 127, -128", 0x91, 0x7F, 0x80, false, true},
		{"CMPB -128, 127", 0x91, 0x80, 0x7F, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(registers.R1, tc.src1)
			cpu.SetGPR(registers.R2, tc.src2)

			stepInstruction(t, e, tc.opcode, regMode(registers.R1), regMode(registers.R2))

			psl := cpu.PSL()
			if psl.N() != tc.wantN || psl.Z() || psl.V() || psl.C() != tc.wantC {
				t.Errorf("N=%v Z=%v V=%v C=%v, want N=%v Z=false V=false C=%v",
					psl.N(), psl.Z(), psl.V(), psl.C(), tc.wantN, tc.wantC)
			}
		})
	}
}

func TestEmulCmplUsesGenericHandler(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(registers.R1, 5)
	cpu.SetGPR(registers.R2, 5)

	stepInstruction(t, e, 0xD1, regMode(registers.R1), regMode(registers.R2)) // CMPL

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}

func TestEmulBitLeavesCarryUnaffected(t *testing.T) {
	// emul_cmp.c force-clears C for BIT; the manual specifies unaffected.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(registers.R1, 0x0F)
	cpu.SetGPR(registers.R2, 0xF0)
	setC(cpu, true)

	stepInstruction(t, e, 0x93, regMode(registers.R1), regMode(registers.R2)) // BITB: 0x0F & 0xF0 = 0

	if cpu.GPR(registers.R1) != 0x0F || cpu.GPR(registers.R2) != 0xF0 {
		t.Error("BIT modified an operand")
	}

	psl := cpu.PSL()
	if !psl.Z() || psl.N() || psl.V() {
		t.Errorf("N=%v Z=%v V=%v, want N=false Z=true V=false", psl.N(), psl.Z(), psl.V())
	}
	
	if !psl.C() {
		t.Error("C = false, want unaffected (true)")
	}
}

func TestEmulBitNonzeroResult(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(registers.R1, 0xFF)
	cpu.SetGPR(registers.R2, 0x80)

	stepInstruction(t, e, 0x93, regMode(registers.R1), regMode(registers.R2)) // BITB: 0xFF & 0x80 = 0x80

	psl := cpu.PSL()
	if !psl.N() || psl.Z() {
		t.Errorf("N=%v Z=%v, want N=true Z=false", psl.N(), psl.Z())
	}
}

func TestEmulTst(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(registers.R1, 0x80) // byte: negative
	setC(cpu, true)

	stepInstruction(t, e, 0x95, regMode(registers.R1)) // TSTB

	if cpu.GPR(registers.R1) != 0x80 {
		t.Error("TST modified its operand")
	}
	
	psl := cpu.PSL()
	if !psl.N() || psl.Z() || psl.V() || psl.C() {
		t.Errorf("N=%v Z=%v V=%v C=%v, want N=true Z=false V=false C=false", psl.N(), psl.Z(), psl.V(), psl.C())
	}
}

func TestEmulTstZero(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(registers.R1, 0)

	stepInstruction(t, e, 0x95, regMode(registers.R1)) // TSTB

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}
