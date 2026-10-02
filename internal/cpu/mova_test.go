package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// deferredMode returns the addressing-mode byte for Register deferred mode
// on r: (Rn), a memory operand whose address is simply r's value.
func deferredMode(r vax.Reg) byte { return 0x60 | byte(r) }

func TestEmulMova(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R3, 0x2000)
	psl := cpu.PSL()
	psl.SetNZVC(true, true, true, true)
	cpu.SetPSL(psl)

	stepInstruction(t, e, 0xDE, deferredMode(vax.R3), regMode(vax.R5)) // MOVAL (R3),R5

	if got := cpu.GPR(vax.R5); got != 0x2000 {
		t.Errorf("R5 = %#x, want 0x2000 (the address held in R3, not its contents)", got)
	}

	// N and Z from the address, V cleared, C unaffected.
	got := cpu.PSL()
	if got.N() || got.Z() || got.V() || !got.C() {
		t.Errorf("PSL = %+v, want N=0 Z=0 V=0 C=1 (C left as pre-set)", got)
	}
}

// TestEmulMovaPushaConditionCodes checks MOVA's and PUSHA's N and Z for
// a negative and a zero address (the manual: N <- dst LSS 0, Z <- dst EQL
// 0, V <- 0, C <- C).
func TestEmulMovaPushaConditionCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		addr   uint32
		n, z   bool
		pushal bool
	}{
		{"MOVAL negative", 0x80001000, true, false, false},
		{"MOVAL zero", 0, false, true, false},
		{"PUSHAL negative", 0x80001000, true, false, true},
		{"PUSHAL zero", 0, false, true, true},
	} {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		cpu.SetGPR(vax.R3, tc.addr)
		cpu.SetGPR(vax.SP, 0x8000)
		psl := cpu.PSL()
		psl.SetNZVC(!tc.n, !tc.z, true, false)
		cpu.SetPSL(psl)

		if tc.pushal {
			stepInstruction(t, e, 0xDF, deferredMode(vax.R3)) // PUSHAL (R3)
		} else {
			stepInstruction(t, e, 0xDE, deferredMode(vax.R3), regMode(vax.R5)) // MOVAL (R3),R5
		}

		got := cpu.PSL()
		if got.N() != tc.n || got.Z() != tc.z || got.V() || got.C() {
			t.Errorf("%s: PSL = %+v, want N=%v Z=%v V=0 C=0 (C left as pre-set)", tc.name, got, tc.n, tc.z)
		}
	}
}

func TestEmulMovaRegisterModeFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, 0xDE, regMode(vax.R1), regMode(vax.R2)) // MOVAL R1,R2
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x300, 0)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v (fault should be handled, not propagated)", err)
	}

	if cpu.GPR(vax.PC) != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (fault vector)", cpu.GPR(vax.PC))
	}
}

func TestEmulPushaRegisterModeFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, 0xDF, regMode(vax.R1)) // PUSHAL R1
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x300, 0)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v (fault should be handled, not propagated)", err)
	}

	if cpu.GPR(vax.PC) != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (fault vector)", cpu.GPR(vax.PC))
	}
}

func TestEmulPushal(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R3, 0x2000)
	cpu.SetGPR(vax.SP, 0x8000)

	stepInstruction(t, e, 0xDF, deferredMode(vax.R3)) // PUSHAL (R3)

	if cpu.GPR(vax.SP) != 0x7FFC {
		t.Fatalf("SP = %#x, want 0x7FFC", cpu.GPR(vax.SP))
	}
	
	v, err := mem.LoadLongword(cpu, cpu.GPR(vax.SP))
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	if v != 0x2000 {
		t.Errorf("pushed value = %#x, want 0x2000 (the address held in R3)", v)
	}
}

func TestEmulPushl(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x12345678)
	cpu.SetGPR(vax.SP, 0x8000)

	stepInstruction(t, e, 0xDD, regMode(vax.R1)) // PUSHL R1

	if cpu.GPR(vax.SP) != 0x7FFC {
		t.Fatalf("SP = %#x, want 0x7FFC", cpu.GPR(vax.SP))
	}

	v, err := mem.LoadLongword(cpu, cpu.GPR(vax.SP))
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	if v != 0x12345678 {
		t.Errorf("pushed value = %#x, want 0x12345678 (R1's value)", v)
	}
}
