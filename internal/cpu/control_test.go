package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulHaltInKernelMode(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x00) // HALT

	err := e.Step()
	if !errors.Is(err, ErrHalted) {
		t.Fatalf("Step() = %v, want ErrHalted", err)
	}
	if !e.Halted() {
		t.Error("Halted() = false, want true")
	}
}

// TestEmulHaltFaultsOutsideKernelMode calls emulHalt directly rather than
// through Engine.Step/HandleFault: a full end-to-end fault delivery from User
// mode would also exercise setModeStack's unconditional MAPEN=1 write on a
// real mode switch (see docs/DEVIATIONS.md), which needs page tables this
// test has no reason to set up. What matters here is only that HALT itself
// reports the privileged-instruction fault outside kernel mode.
func TestEmulHaltFaultsOutsideKernelMode(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)

	err := emulHalt(e, &Decoded{})

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcPrivileged {
		t.Fatalf("emulHalt() = %v, want *Fault{Code: ExcPrivileged}", err)
	}
	if e.Halted() {
		t.Error("Halted() = true, want false (privileged fault, not a halt)")
	}
}

func TestEmulNop(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x01) // NOP

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if e.Halted() {
		t.Error("Halted() = true, want false")
	}
	if cpu.GPR(vax.PC) != base+1 {
		t.Errorf("PC = %#x, want %#x", cpu.GPR(vax.PC), base+1)
	}
}
