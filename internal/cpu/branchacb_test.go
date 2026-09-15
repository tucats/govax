package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulAcbPositiveAddendBoundary(t *testing.T) {
	// The C source's strict `<` misses the boundary case where a positive-
	// addend loop's index lands exactly on the limit; the manual specifies
	// <=. This is the counting-up FOR-loop idiom: index starts below limit
	// and should branch (continue looping) on the iteration that reaches it
	// exactly.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 10) // limit
	cpu.SetGPR(vax.R2, 1)  // addend
	cpu.SetGPR(vax.R3, 9)  // index -> becomes 10, equal to limit
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x9D, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00) // ACBB limit,addend,index,+16

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := byte(cpu.GPR(vax.R3)); got != 10 {
		t.Errorf("index = %d, want 10", got)
	}
	want := uint32(base + 6 + 16) // taken
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (branch taken: index reached limit exactly)", got, want)
	}
}

func TestEmulAcbNegativeAddendBoundary(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0)    // limit
	cpu.SetGPR(vax.R2, 0xFF) // addend: -1
	cpu.SetGPR(vax.R3, 1)    // index -> becomes 0, equal to limit
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x9D, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	want := uint32(base + 6 + 16)
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (branch taken: index still >= limit)", got, want)
	}
}

func TestEmulAcbNotTaken(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 10) // limit
	cpu.SetGPR(vax.R2, 1)  // addend
	cpu.SetGPR(vax.R3, 10) // index -> becomes 11, past limit
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x9D, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	want := uint32(base + 6) // not taken
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (loop exits)", got, want)
	}
}

func TestEmulAcbCarryUnaffected(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 10)
	cpu.SetGPR(vax.R2, 1)
	cpu.SetGPR(vax.R3, 5)
	setC(cpu, true)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x9D, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3), 0x10, 0x00)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if !cpu.PSL().C() {
		t.Error("C = false, want unaffected (true)")
	}
}

// CASEB layout: opcode, 3 register operands (selector, base, limit), then
// (limit+1) word displacements starting immediately at the next byte.

func TestEmulCaseInRangeBranches(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 5) // selector
	cpu.SetGPR(vax.R2, 3) // base -> idx = 2
	cpu.SetGPR(vax.R3, 3) // limit (4 entries: 0..3)
	cpu.SetGPR(vax.PC, base)

	instrEnd := base + 4 // opcode + 3 register-mode bytes
	tableBase := uint32(instrEnd)

	putBytes(t, cpu, mem, base, 0x8F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3)) // CASEB

	// Table entries for idx 0..3; idx 2 (selected) branches +100.
	for i, disp := range []int16{10, 20, 100, 30} {
		putBytes(t, cpu, mem, tableBase+uint32(i*2), byte(disp), byte(disp>>8))
	}

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := cpu.GPR(vax.PC); got != tableBase+100 {
		t.Errorf("PC = %#x, want %#x", got, tableBase+100)
	}

	psl := cpu.PSL()
	if !psl.N() || psl.Z() || psl.V() {
		t.Errorf("N=%v Z=%v V=%v, want N=true Z=false V=false (idx=2 < limit=3)", psl.N(), psl.Z(), psl.V())
	}

	if !psl.C() {
		t.Error("C = false, want true (idx < limit)")
	}
}

func TestEmulCaseAtLimit(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.R1, 6) // selector
	cpu.SetGPR(vax.R2, 3) // base -> idx = 3 == limit
	cpu.SetGPR(vax.R3, 3) // limit
	cpu.SetGPR(vax.PC, base)

	instrEnd := base + 4
	tableBase := uint32(instrEnd)

	putBytes(t, cpu, mem, base, 0x8F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3))

	for i, disp := range []int16{10, 20, 30, 40} {
		putBytes(t, cpu, mem, tableBase+uint32(i*2), byte(disp), byte(disp>>8))
	}

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := cpu.GPR(vax.PC); got != tableBase+40 {
		t.Errorf("PC = %#x, want %#x", got, tableBase+40)
	}

	psl := cpu.PSL()
	if !psl.Z() {
		t.Error("Z = false, want true (idx == limit)")
	}

	if psl.C() {
		t.Error("C = true, want false (idx not < limit)")
	}
}

func TestEmulCaseOutOfRangeSkipsTable(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetGPR(vax.R1, 10) // selector
	cpu.SetGPR(vax.R2, 3)  // base -> idx = 7, > limit
	cpu.SetGPR(vax.R3, 3)  // limit (4 entries)
	cpu.SetGPR(vax.PC, base)

	tableBase := uint32(base + 4)

	putBytes(t, cpu, mem, base, 0x8F, regMode(vax.R1), regMode(vax.R2), regMode(vax.R3))

	for i, disp := range []int16{10, 20, 30, 40} {
		putBytes(t, cpu, mem, tableBase+uint32(i*2), byte(disp), byte(disp>>8))
	}

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	// PC should land right after the 4-entry (8-byte) table.
	want := tableBase + 2 + 3*2
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x (past the displacement table)", got, want)
	}

	psl := cpu.PSL()
	if psl.Z() || psl.C() {
		t.Errorf("Z=%v C=%v, want both false", psl.Z(), psl.C())
	}
}
