package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// testEngine builds an Engine around a private Table (not the shared
// package-level instructionTable), so dispatch tests that register
// Handlers don't leak state into other tests.
func testEngine(instructions []*Instruction) *Engine {
	cpu, mem := fixture()
	cpu.SetPR(vax.SCBB, scbb)
	return &Engine{cpu: cpu, mem: mem, table: newTable(instructions)}
}

func TestEngineStepDispatchesHandler(t *testing.T) {
	inst := &Instruction{Name: "TESTNOP", Opcode: Opcode{Function: 0x01}}
	e := testEngine([]*Instruction{inst})
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0x01)

	var gotPC uint32
	var gotInst *Instruction
	e.table.SetHandler(inst, func(eng *Engine, d *Decoded) error {
		gotPC = eng.cpu.GPR(vax.PC)
		gotInst = d.Instruction
		return nil
	})

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if gotInst != inst {
		t.Errorf("handler saw Instruction %+v, want %+v", gotInst, inst)
	}
	if gotPC != base+1 {
		t.Errorf("PC at handler call = %#x, want %#x (advanced past the instruction)", gotPC, base+1)
	}
}

func TestEngineStepUnimplementedFaultsAndContinues(t *testing.T) {
	inst := &Instruction{Name: "TESTUNIMP", Opcode: Opcode{Function: 0x01}}
	e := testEngine([]*Instruction{inst}) // no handler registered
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0x01)
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcPrivileged, 0x200, 0)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v (fault should be handled, not propagated)", err)
	}
	if e.cpu.GPR(vax.PC) != 0x200 {
		t.Errorf("PC = %#x, want 0x200 (fault vector)", e.cpu.GPR(vax.PC))
	}
}

func TestEngineStepHandlerReturnsErrHalted(t *testing.T) {
	inst := &Instruction{Name: "TESTHALT", Opcode: Opcode{Function: 0x00}}
	e := testEngine([]*Instruction{inst})
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0x00)
	e.table.SetHandler(inst, func(eng *Engine, d *Decoded) error { return ErrHalted })

	err := e.Step()
	if !errors.Is(err, ErrHalted) {
		t.Fatalf("err = %v, want ErrHalted", err)
	}
	if !e.Halted() {
		t.Error("Halted() = false, want true")
	}
}

func TestEngineRunStopsOnHalt(t *testing.T) {
	inst := &Instruction{Name: "TESTHALT", Opcode: Opcode{Function: 0x00}}
	e := testEngine([]*Instruction{inst})
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0x00)
	e.table.SetHandler(inst, func(eng *Engine, d *Decoded) error { return ErrHalted })

	err := e.Run()
	if !errors.Is(err, ErrHalted) {
		t.Fatalf("Run() = %v, want ErrHalted", err)
	}
}

func TestEngineRunPropagatesOtherHandlerError(t *testing.T) {
	inst := &Instruction{Name: "TESTBAD", Opcode: Opcode{Function: 0x00}}
	e := testEngine([]*Instruction{inst})
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0x00)
	wantErr := errors.New("boom")
	e.table.SetHandler(inst, func(eng *Engine, d *Decoded) error { return wantErr })

	err := e.Run()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() = %v, want %v", err, wantErr)
	}
}

func TestEngineStepDecodeFaultHandled(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetPR(vax.SCBB, scbb)
	e := NewEngine(cpu, mem) // uses the real, shared instruction table
	cpu.SetGPR(vax.PC, base)
	// MOVL S^#5,S^#5 : illegal write to a short literal -> reserved
	// addressing mode fault at decode time.
	putBytes(t, cpu, mem, base, 0xD0, 0x05, 0x05)
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedAddr, 0x300, 0)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if cpu.GPR(vax.PC) != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (fault vector)", cpu.GPR(vax.PC))
	}
}
