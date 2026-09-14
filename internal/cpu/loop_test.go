package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulAobleq(t *testing.T) {
	cases := []struct {
		name       string
		limit, idx uint32
		wantTaken  bool
	}{
		{"boundary taken (index reaches limit)", 10, 9, true},
		{"not taken (index exceeds limit)", 10, 10, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.limit)
			cpu.SetGPR(vax.R2, tc.idx)
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, 0xF3, regMode(vax.R1), regMode(vax.R2), 0x10) // AOBLEQ +16

			if err := e.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}
			if got := cpu.GPR(vax.R2); got != tc.idx+1 {
				t.Errorf("index = %d, want %d", got, tc.idx+1)
			}
			want := uint32(base + 4)
			if tc.wantTaken {
				want += 16
			}
			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x (taken=%v)", got, want, tc.wantTaken)
			}
		})
	}
}

func TestEmulAoblss(t *testing.T) {
	cases := []struct {
		name       string
		limit, idx uint32
		wantTaken  bool
	}{
		{"strictly below limit", 10, 8, true},
		{"reaches limit exactly, not taken", 10, 9, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.limit)
			cpu.SetGPR(vax.R2, tc.idx)
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, 0xF2, regMode(vax.R1), regMode(vax.R2), 0x10) // AOBLSS +16

			if err := e.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}
			want := uint32(base + 4)
			if tc.wantTaken {
				want += 16
			}
			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x (taken=%v)", got, want, tc.wantTaken)
			}
		})
	}
}

func TestEmulSobgtr(t *testing.T) {
	cases := []struct {
		name      string
		idx       uint32
		wantTaken bool
	}{
		{"decrements to positive", 2, true},
		{"decrements to zero, not taken", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.idx)
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, 0xF5, regMode(vax.R1), 0x10) // SOBGTR +16

			if err := e.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}
			want := uint32(base + 3)
			if tc.wantTaken {
				want += 16
			}
			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x (taken=%v)", got, want, tc.wantTaken)
			}
		})
	}
}

func TestEmulSobgeq(t *testing.T) {
	cases := []struct {
		name      string
		idx       uint32
		wantTaken bool
	}{
		{"decrements to zero, taken", 1, true},
		{"decrements to negative, not taken", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.idx)
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, 0xF4, regMode(vax.R1), 0x10) // SOBGEQ +16

			if err := e.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}
			want := uint32(base + 3)
			if tc.wantTaken {
				want += 16
			}
			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x (taken=%v)", got, want, tc.wantTaken)
			}
		})
	}
}

func TestEmulLoopOverflowAndCarryUnaffected(t *testing.T) {
	// AOBLEQ/AOBLSS/SOBGTR/SOBGEQ are exactly INCL/DECL arithmetically, so
	// they share INC/DEC's confirmed longword-overflow fix (sub-phase 5);
	// unlike INC/DEC, C is unaffected here rather than a real carry/borrow.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x7FFFFFFF) // limit, irrelevant to overflow here
	cpu.SetGPR(vax.R2, 0x7FFFFFFF) // index: INT32_MAX
	setC(cpu, true)
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0xF3, regMode(vax.R1), regMode(vax.R2), 0x10) // AOBLEQ

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if !cpu.PSL().V() {
		t.Error("V = false, want true (INT32_MAX + 1 overflows)")
	}
	if !cpu.PSL().C() {
		t.Error("C = false, want unaffected (true)")
	}
}
