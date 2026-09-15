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
	got := cpu.PSL()
	if got.N() != true || got.Z() != true || got.V() != true || got.C() != true {
		t.Errorf("PSL = %+v, want unaffected (all true, as pre-set)", got)
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
