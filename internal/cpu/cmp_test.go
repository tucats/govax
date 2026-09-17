package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
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
			cpu.SetGPR(vax.R1, tc.src1)
			cpu.SetGPR(vax.R2, tc.src2)

			stepInstruction(t, e, 0x91, regMode(vax.R1), regMode(vax.R2)) // CMPB

			if cpu.GPR(vax.R1) != tc.src1 || cpu.GPR(vax.R2) != tc.src2 {
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

func TestEmulCmplUsesGenericHandler(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 5)
	cpu.SetGPR(vax.R2, 5)

	stepInstruction(t, e, 0xD1, regMode(vax.R1), regMode(vax.R2)) // CMPL

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}

func TestEmulBitLeavesCarryUnaffected(t *testing.T) {
	// emul_cmp.c force-clears C for BIT; the manual specifies unaffected.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x0F)
	cpu.SetGPR(vax.R2, 0xF0)
	setC(cpu, true)

	stepInstruction(t, e, 0x93, regMode(vax.R1), regMode(vax.R2)) // BITB: 0x0F & 0xF0 = 0

	if cpu.GPR(vax.R1) != 0x0F || cpu.GPR(vax.R2) != 0xF0 {
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
	cpu.SetGPR(vax.R1, 0xFF)
	cpu.SetGPR(vax.R2, 0x80)

	stepInstruction(t, e, 0x93, regMode(vax.R1), regMode(vax.R2)) // BITB: 0xFF & 0x80 = 0x80

	psl := cpu.PSL()
	if !psl.N() || psl.Z() {
		t.Errorf("N=%v Z=%v, want N=true Z=false", psl.N(), psl.Z())
	}
}

func TestEmulTst(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x80) // byte: negative
	setC(cpu, true)

	stepInstruction(t, e, 0x95, regMode(vax.R1)) // TSTB

	if cpu.GPR(vax.R1) != 0x80 {
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
	cpu.SetGPR(vax.R1, 0)

	stepInstruction(t, e, 0x95, regMode(vax.R1)) // TSTB

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}
