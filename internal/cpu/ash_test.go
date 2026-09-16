package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulRotl(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 1) // count: left 1
	cpu.SetGPR(vax.R2, 0x80000001)
	setC(cpu, true)

	stepInstruction(t, e, 0x9C, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // ROTL

	if got := cpu.GPR(vax.R3); got != 0x00000003 {
		t.Errorf("result = %#x, want 0x00000003", got)
	}

	psl := cpu.PSL()
	if psl.V() {
		t.Error("V = true, want false")
	}

	if !psl.C() {
		t.Error("C = false, want unaffected (true)")
	}
}

func TestEmulRotlNegativeCountRotatesRight(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFFFFFFFF) // count: right 1
	cpu.SetGPR(vax.R2, 0x00000003)

	stepInstruction(t, e, 0x9C, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3))

	if got := cpu.GPR(vax.R3); got != 0x80000001 {
		t.Errorf("result = %#x, want 0x80000001", got)
	}
}

func TestEmulAshlLeftOverflow(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 1)          // count: left 1
	cpu.SetGPR(vax.R2, 0x7FFFFFFF) // INT32_MAX
	setC(cpu, true)

	stepInstruction(t, e, 0x78, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // ASHL

	if got := cpu.GPR(vax.R3); got != 0xFFFFFFFE {
		t.Errorf("result = %#x, want 0xfffffffe", got)
	}

	psl := cpu.PSL()
	if !psl.V() {
		t.Error("V = false, want true (shifting INT32_MAX left overflows)")
	}

	if !psl.C() {
		t.Error("C = false, want unaffected (true)")
	}
}

func TestEmulAshlRightNeverOverflows(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFFFFFFFF) // count: right 1
	cpu.SetGPR(vax.R2, 0x80000000) // INT32_MIN

	stepInstruction(t, e, 0x78, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3))

	if got := cpu.GPR(vax.R3); got != 0xC0000000 {
		t.Errorf("result = %#x, want 0xc0000000 (arithmetic shift, sign-extended)", got)
	}

	if cpu.PSL().V() {
		t.Error("V = true, want false (right shift never overflows)")
	}
}

func TestEmulAshlZeroCount(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0)
	cpu.SetGPR(vax.R2, 0x12345678)

	stepInstruction(t, e, 0x78, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3))

	if got := cpu.GPR(vax.R3); got != 0x12345678 {
		t.Errorf("result = %#x, want unchanged 0x12345678", got)
	}

	if cpu.PSL().V() {
		t.Error("V = true, want false")
	}
}

func TestEmulAshq(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 4) // count: left 4
	cpu.SetGPR(vax.R2, 1) // source pair R2:R3 = 1
	cpu.SetGPR(vax.R3, 0)

	stepInstruction(t, e, 0x79, regMode(vax.R1), regMode(vax.R2), regMode(vax.R4)) // ASHQ

	if cpu.GPR(vax.R4) != 16 || cpu.GPR(vax.R5) != 0 {
		t.Errorf("result pair = %#x:%#x, want 16:0", cpu.GPR(vax.R4), cpu.GPR(vax.R5))
	}
	
	if cpu.PSL().V() {
		t.Error("V = true, want false")
	}
}

func TestEmulAshqOverflow(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 1)          // count: left 1
	cpu.SetGPR(vax.R2, 0xFFFFFFFF) // source pair R2:R3 = INT64_MAX
	cpu.SetGPR(vax.R3, 0x7FFFFFFF)

	stepInstruction(t, e, 0x79, regMode(vax.R1), regMode(vax.R2), regMode(vax.R4))

	if !cpu.PSL().V() {
		t.Error("V = false, want true (shifting INT64_MAX left overflows)")
	}
}
