package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulMoveFloat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opcode byte
		size   int
	}{
		{"MOVF", 0x50, 4},
		{"MOVD", 0x70, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			setFloatReg(t, cpu, tc.size, vax.R1, -2.5)
			dst := vax.R2
			if tc.size == 8 {
				dst = vax.R3
			}
			setC(cpu, true) // confirm C is left unaffected, not cleared

			stepInstruction(t, e, tc.opcode, regMode(vax.R1), regMode(dst))

			if got := getFloatReg(t, cpu, tc.size, dst); got != -2.5 {
				t.Errorf("result = %v, want -2.5", got)
			}
			psl := cpu.PSL()
			if !psl.N() || psl.Z() || psl.V() {
				t.Errorf("N=%v Z=%v V=%v, want N=true Z=false V=false", psl.N(), psl.Z(), psl.V())
			}
			if !psl.C() {
				t.Error("C = false, want true (MOV leaves C unaffected)")
			}
		})
	}
}

func TestEmulNegateFloat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opcode byte
		size   int
		src    float64
		want   float64
	}{
		{"MNEGF positive", 0x52, 4, 3.0, -3.0},
		{"MNEGF negative", 0x52, 4, -3.0, 3.0},
		{"MNEGD zero", 0x72, 8, 0.0, 0.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			setFloatReg(t, cpu, tc.size, vax.R1, tc.src)
			dst := vax.R2
			if tc.size == 8 {
				dst = vax.R3
			}

			stepInstruction(t, e, tc.opcode, regMode(vax.R1), regMode(dst))

			if got := getFloatReg(t, cpu, tc.size, dst); got != tc.want {
				t.Errorf("result = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEmulNegateFloatZeroSetsZ(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 4, vax.R1, 0.0)

	stepInstruction(t, e, 0x52, regMode(vax.R1), regMode(vax.R2)) // MNEGF

	psl := cpu.PSL()
	if !psl.Z() || psl.N() {
		t.Errorf("N=%v Z=%v, want N=false Z=true", psl.N(), psl.Z())
	}
}
