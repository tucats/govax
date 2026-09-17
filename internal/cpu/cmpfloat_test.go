package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulCmpFloat(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		size   int
		a, b   float64
		wantN  bool
		wantZ  bool
	}{
		{"CMPF less", 0x51, 4, 1.0, 2.0, true, false},
		{"CMPF greater", 0x51, 4, 2.0, 1.0, false, false},
		{"CMPF equal", 0x51, 4, 3.0, 3.0, false, true},
		{"CMPD less", 0x71, 8, -5.0, -1.0, true, false},
		{"CMPD equal", 0x71, 8, 0.0, 0.0, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			setFloatReg(t, cpu, c.size, vax.R1, c.a)
			r2 := vax.R2

			if c.size == 8 {
				r2 = vax.R3
			}

			setFloatReg(t, cpu, c.size, r2, c.b)
			stepInstruction(t, e, c.opcode, regMode(vax.R1), regMode(r2))

			psl := cpu.PSL()
			if psl.N() != c.wantN || psl.Z() != c.wantZ {
				t.Errorf("N=%v Z=%v, want N=%v Z=%v", psl.N(), psl.Z(), c.wantN, c.wantZ)
			}

			if psl.V() || psl.C() {
				t.Errorf("V=%v C=%v, want both false", psl.V(), psl.C())
			}

			// operands unmodified
			if got := getFloatReg(t, cpu, c.size, vax.R1); got != c.a {
				t.Errorf("src1 modified: = %v, want %v", got, c.a)
			}

			if got := getFloatReg(t, cpu, c.size, r2); got != c.b {
				t.Errorf("src2 modified: = %v, want %v", got, c.b)
			}
		})
	}
}

func TestEmulCmpFloatCUnaffectedByUnsignedNotion(t *testing.T) {
	// Unlike integer CMP, C is always 0 for a float compare -- there's no
	// unsigned interpretation of a floating value.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setC(cpu, true)
	setFloatReg(t, cpu, 4, vax.R1, -1.0)
	setFloatReg(t, cpu, 4, vax.R2, 1.0)

	stepInstruction(t, e, 0x51, regMode(vax.R1), regMode(vax.R2)) // CMPF

	if cpu.PSL().C() {
		t.Error("C = true, want false (CMPF always clears C)")
	}
}

func TestEmulTstFloat(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		size   int
		value  float64
		wantN  bool
		wantZ  bool
	}{
		{"TSTF negative", 0x53, 4, -1.5, true, false},
		{"TSTF zero", 0x53, 4, 0.0, false, true},
		{"TSTF positive", 0x53, 4, 1.5, false, false},
		{"TSTD negative", 0x73, 8, -1.5, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			setFloatReg(t, cpu, c.size, vax.R1, c.value)
			setC(cpu, true) // confirm TST forces C to 0, not "unaffected"

			stepInstruction(t, e, c.opcode, regMode(vax.R1))

			psl := cpu.PSL()
			if psl.N() != c.wantN || psl.Z() != c.wantZ {
				t.Errorf("N=%v Z=%v, want N=%v Z=%v", psl.N(), psl.Z(), c.wantN, c.wantZ)
			}

			if psl.V() || psl.C() {
				t.Errorf("V=%v C=%v, want both false", psl.V(), psl.C())
			}
			
			if got := getFloatReg(t, cpu, c.size, vax.R1); got != c.value {
				t.Errorf("source modified: = %v, want %v", got, c.value)
			}
		})
	}
}
