package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulClr(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		reg    vax.Reg
	}{
		{"CLRB", 0x94, vax.R1},
		{"CLRW", 0xB4, vax.R1},
		{"CLRL", 0xD4, vax.R1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(tc.reg, 0xFFFFFFFF)
			psl := cpu.PSL()
			psl.SetNZVC(true, false, true, true)
			cpu.SetPSL(psl)

			stepInstruction(t, e, tc.opcode, regMode(tc.reg))

			got := cpu.PSL()
			if got.N() || !got.Z() || got.V() {
				t.Errorf("N=%v Z=%v V=%v, want N=false Z=true V=false", got.N(), got.Z(), got.V())
			}

			if !got.C() {
				t.Error("C = false, want unaffected (true)")
			}
		})
	}
}

func TestEmulClrbPreservesUpperBytes(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xAABBCCDD)

	stepInstruction(t, e, 0x94, regMode(vax.R1)) // CLRB

	if got := cpu.GPR(vax.R1); got != 0xAABBCC00 {
		t.Errorf("R1 = %#x, want 0xAABBCC00 (only low byte cleared)", got)
	}
}

func TestEmulClrq(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFFFFFFFF)
	cpu.SetGPR(vax.R2, 0xFFFFFFFF)

	stepInstruction(t, e, 0x7C, regMode(vax.R1)) // CLRQ

	if cpu.GPR(vax.R1) != 0 || cpu.GPR(vax.R2) != 0 {
		t.Errorf("register pair = %#x:%#x, want 0:0", cpu.GPR(vax.R1), cpu.GPR(vax.R2))
	}
	
	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}
