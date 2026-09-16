package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulCvtFloatToIntTruncates(t *testing.T) {
	cases := []struct {
		name    string
		opcode  byte
		size    int // source float size
		dstSize int // destination integer size
		value   float64
		want    int64
		wantN   bool
		wantZ   bool
	}{
		{"CVTFB positive truncates toward zero", 0x48, 4, 1, 4.9, 4, false, false},
		{"CVTFB negative truncates toward zero", 0x48, 4, 1, -4.9, -4, true, false},
		{"CVTFW", 0x49, 4, 2, 1000.5, 1000, false, false},
		{"CVTFL zero", 0x4A, 4, 4, 0.0, 0, false, true},
		{"CVTDB", 0x68, 8, 1, 100.9, 100, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			setFloatReg(t, cpu, c.size, vax.R1, c.value)
			dstReg := vax.R2
			if c.size == 8 {
				dstReg = vax.R3 // avoid overlapping R1:R2
			}
			cpu.SetGPR(dstReg, 0xFFFFFFFF)

			stepInstruction(t, e, c.opcode, regMode(vax.R1), regMode(dstReg))

			if got := signExtend(uint64(cpu.GPR(dstReg)), c.dstSize); got != c.want {
				t.Errorf("result = %d, want %d", got, c.want)
			}
			psl := cpu.PSL()
			if psl.N() != c.wantN || psl.Z() != c.wantZ {
				t.Errorf("N=%v Z=%v, want N=%v Z=%v", psl.N(), psl.Z(), c.wantN, c.wantZ)
			}

			if psl.V() || psl.C() {
				t.Errorf("V=%v C=%v, want both false", psl.V(), psl.C())
			}
		})
	}
}

func TestEmulCvtFloatToByteExactValue(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setFloatReg(t, cpu, 4, vax.R1, -4.9)

	stepInstruction(t, e, 0x48, regMode(vax.R1), regMode(vax.R2)) // CVTFB

	if got := int8(byte(cpu.GPR(vax.R2))); got != -4 {
		t.Errorf("R2 = %d, want -4 (truncated toward zero)", got)
	}
}

func TestEmulCvtFloatToIntOverflowFaults(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.PC, base)
	setFloatReg(t, cpu, 4, vax.R1, 200.0)                               // exceeds byte range (-128..127)
	putBytes(t, cpu, mem, base, 0x48, regMode(vax.R1), regMode(vax.R2)) // CVTFB

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}
	err = emulCvtFloatToInt(e, &d)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcArithmetic || len(f.Args) != 1 || f.Args[0] != trapIntOvf {
		t.Fatalf("err = %v, want *Fault{ExcArithmetic, [trapIntOvf]}", err)
	}
}

func TestEmulCvtRoundVsTruncate(t *testing.T) {
	// CVTRFL rounds to nearest (ties away from zero); CVTFL truncates
	// toward zero -- the distinction the C reference never actually
	// implements (CVTRFL always faults EXC_PRIV there instead). See
	// docs/PHASE-05.md's design notes.
	cases := []struct {
		name       string
		opcode     byte
		value      float64
		wantResult int32
	}{
		{"CVTFL truncates positive", 0x4A, 4.6, 4},
		{"CVTFL truncates negative", 0x4A, -4.6, -4},
		{"CVTRFL rounds positive up", 0x4B, 4.6, 5},
		{"CVTRFL rounds negative away from zero", 0x4B, -4.6, -5},
		{"CVTRFL rounds down when closer", 0x4B, 4.4, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			setFloatReg(t, cpu, 4, vax.R1, c.value)

			stepInstruction(t, e, c.opcode, regMode(vax.R1), regMode(vax.R2))

			if got := int32(cpu.GPR(vax.R2)); got != c.wantResult {
				t.Errorf("result = %d, want %d", got, c.wantResult)
			}
		})
	}
}

func TestEmulCvtRoundFloatToIntOverflow(t *testing.T) {
	// 32767.6 alone would fit a word... but CVTRFW doesn't exist (the R
	// variants are long-destination only); use a long-boundary case instead
	// via CVTRDL: a value that only overflows *after* rounding.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.PC, base)
	setFloatReg(t, cpu, 8, vax.R1, 2147483647.6)                        // rounds to 2147483648, overflows Long
	putBytes(t, cpu, mem, base, 0x6B, regMode(vax.R1), regMode(vax.R3)) // CVTRDL

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}
	err = emulCvtRoundFloatToInt(e, &d)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcArithmetic {
		t.Fatalf("err = %v, want *Fault{Code: ExcArithmetic} (rounding pushed the value out of Long range)", err)
	}
}

func TestEmulCvtIntToFloat(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		srcVal uint32
		size   int // source int size
		dst    int // destination float size
		want   float64
	}{
		{"CVTBF positive", 0x4C, 100, 1, 4, 100.0},
		{"CVTBF negative", 0x4C, 0x80, 1, 4, -128.0}, // byte 0x80 sign-extends to -128
		{"CVTWF", 0x4D, 1000, 2, 4, 1000.0},
		{"CVTLF", 0x4E, 0xFFFFFFFF, 4, 4, -1.0}, // long -1
		{"CVTBD", 0x6C, 100, 1, 8, 100.0},
		{"CVTLD", 0x6E, 123456, 4, 8, 123456.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, c.srcVal)
			dstReg := vax.R2
			if c.dst == 8 {
				dstReg = vax.R3
			}

			stepInstruction(t, e, c.opcode, regMode(vax.R1), regMode(dstReg))

			if got := getFloatReg(t, cpu, c.dst, dstReg); got != c.want {
				t.Errorf("result = %v, want %v", got, c.want)
			}
			psl := cpu.PSL()
			if psl.V() || psl.C() {
				t.Errorf("V=%v C=%v, want both false (exact int->float conversion never overflows)", psl.V(), psl.C())
			}
		})
	}
}

func TestEmulCvtIntToFloatNegativeSetsN(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFF) // byte -1

	stepInstruction(t, e, 0x4C, regMode(vax.R1), regMode(vax.R2)) // CVTBF

	if !cpu.PSL().N() {
		t.Error("N = false, want true")
	}
	
	if got := getFloatReg(t, cpu, 4, vax.R2); got != -1.0 {
		t.Errorf("result = %v, want -1.0", got)
	}
}
