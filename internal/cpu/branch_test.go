package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulBranchAlways(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x11, 0x10) // BRB +16

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	// Target = address after the displacement byte (base+2) + 16.
	if got := cpu.GPR(vax.PC); got != base+2+16 {
		t.Errorf("PC = %#x, want %#x", got, base+2+16)
	}
}

func TestEmulJmp(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.PC, base)
	cpu.SetGPR(vax.R1, 0x9000)
	putBytes(t, cpu, mem, base, 0x17, deferredMode(vax.R1)) // JMP (R1)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := cpu.GPR(vax.PC); got != 0x9000 {
		t.Errorf("PC = %#x, want 0x9000", got)
	}
}

func TestCondBranches(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		setPSL func(p *vax.PSL)
		taken  bool
	}{
		{"BNEQ taken", 0x12, func(p *vax.PSL) { p.SetZ(false) }, true},
		{"BNEQ not taken", 0x12, func(p *vax.PSL) { p.SetZ(true) }, false},
		{"BEQL taken", 0x13, func(p *vax.PSL) { p.SetZ(true) }, true},
		{"BGTR taken", 0x14, func(p *vax.PSL) { p.SetN(false); p.SetZ(false) }, true},
		{"BGTR not taken (Z)", 0x14, func(p *vax.PSL) { p.SetN(false); p.SetZ(true) }, false},
		{"BLEQ taken (Z)", 0x15, func(p *vax.PSL) { p.SetN(false); p.SetZ(true) }, true},
		{"BGEQ taken", 0x18, func(p *vax.PSL) { p.SetN(false) }, true},
		{"BLSS taken", 0x19, func(p *vax.PSL) { p.SetN(true) }, true},
		{"BGTRU taken", 0x1A, func(p *vax.PSL) { p.SetC(false); p.SetZ(false) }, true},
		{"BLEQU taken (C)", 0x1B, func(p *vax.PSL) { p.SetC(true); p.SetZ(false) }, true},
		{"BVC taken", 0x1C, func(p *vax.PSL) { p.SetV(false) }, true},
		{"BVS taken", 0x1D, func(p *vax.PSL) { p.SetV(true) }, true},
		{"BGEQU taken", 0x1E, func(p *vax.PSL) { p.SetC(false) }, true},
		{"BCS taken", 0x1F, func(p *vax.PSL) { p.SetC(true) }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			psl := cpu.PSL()
			tc.setPSL(&psl)
			cpu.SetPSL(psl)
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, tc.opcode, 0x10) // +16

			if err := e.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}

			want := uint32(base + 2)
			if tc.taken {
				want += 16
			}

			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x (taken=%v)", got, want, tc.taken)
			}
		})
	}
}

func TestEmulRsb(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.SP, 0x8000)
	putLongword(t, cpu, mem, 0x8000, 0x12345678)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x05) // RSB

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := cpu.GPR(vax.PC); got != 0x12345678 {
		t.Errorf("PC = %#x, want 0x12345678", got)
	}

	if got := cpu.GPR(vax.SP); got != 0x8004 {
		t.Errorf("SP = %#x, want 0x8004", got)
	}
}

func TestEmulBsb(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.SP, 0x8000)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x10, 0x10) // BSBB +16

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	wantReturn := uint32(base + 2)
	if got := cpu.GPR(vax.PC); got != wantReturn+16 {
		t.Errorf("PC = %#x, want %#x", got, wantReturn+16)
	}

	if got := cpu.GPR(vax.SP); got != 0x7FFC {
		t.Fatalf("SP = %#x, want 0x7ffc", got)
	}

	pushed, err := mem.LoadLongword(cpu, 0x7FFC)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	if pushed != wantReturn {
		t.Errorf("pushed return address = %#x, want %#x", pushed, wantReturn)
	}
}

func TestEmulJsb(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.SP, 0x8000)
	cpu.SetGPR(vax.PC, base)
	cpu.SetGPR(vax.R1, 0x9000)
	putBytes(t, cpu, mem, base, 0x16, deferredMode(vax.R1)) // JSB (R1)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got := cpu.GPR(vax.PC); got != 0x9000 {
		t.Errorf("PC = %#x, want 0x9000", got)
	}

	if cpu.GPR(vax.SP) != 0x7FFC {
		t.Errorf("SP = %#x, want 0x7ffc", cpu.GPR(vax.SP))
	}
}

func TestEmulBlbsBlbc(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		value  uint32
		taken  bool
	}{
		{"BLBS taken", 0xE8, 1, true},
		{"BLBS not taken", 0xE8, 0, false},
		{"BLBC taken", 0xE9, 0, true},
		{"BLBC not taken", 0xE9, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.value)
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, tc.opcode, regMode(vax.R1), 0x10) // +16

			if err := e.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}
			
			want := uint32(base + 3)
			if tc.taken {
				want += 16
			}

			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x (taken=%v)", got, want, tc.taken)
			}
		})
	}
}
