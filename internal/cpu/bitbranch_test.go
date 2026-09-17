package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// instructionLen4NextPC computes the NextPC (and, for a taken branch, the
// target) of a 4-byte bit-branch instruction (opcode, position, base,
// single-byte displacement) encoded starting at base, matching every test
// in this file's instruction shape.
func bitBranchTargets(disp byte) (nextPC, taken uint32) {
	nextPC = uint32(base) + 4

	return nextPC, nextPC + uint32(int32(int8(disp)))
}

func TestEmulBb(t *testing.T) {
	cases := []struct {
		name       string
		opcode     byte
		regValue   uint32
		position   byte
		wantBranch bool
	}{
		{"BBS taken (bit set)", 0xE0, 0x10, 4, true},
		{"BBS not taken (bit clear)", 0xE0, 0x10, 0, false},
		{"BBC taken (bit clear)", 0xE1, 0x10, 0, true},
		{"BBC not taken (bit set)", 0xE1, 0x10, 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.regValue)

			stepInstruction(t, e, tc.opcode, tc.position, regMode(vax.R1), 0x10)

			nextPC, taken := bitBranchTargets(0x10)
			want := nextPC

			if tc.wantBranch {
				want = taken
			}

			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x", got, want)
			}

			// BBS/BBC never modify the tested operand.
			if got := cpu.GPR(vax.R1); got != tc.regValue {
				t.Errorf("R1 = %#x, want unchanged %#x", got, tc.regValue)
			}
		})
	}
}

func TestEmulBbMemoryBase(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0x10) // bit 4 set

	bytes := []byte{0xE0, 4} // BBS pos=4
	bytes = append(bytes, absoluteMode(0x2000)...)
	bytes = append(bytes, 0x10) // branch displacement
	stepInstruction(t, e, bytes...)

	nextPC := uint32(base) + uint32(len(bytes))

	want := nextPC + 0x10
	if got := cpu.GPR(vax.PC); got != want {
		t.Errorf("PC = %#x, want %#x", got, want)
	}
}

func TestEmulBbImmediateBaseFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, 0xE0, 4, 0x00, 0x10) // base = short literal 0
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v (fault should be handled, not propagated)", err)
	}

	if cpu.GPR(vax.PC) != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (fault vector)", cpu.GPR(vax.PC))
	}
}

func TestEmulBbState(t *testing.T) {
	cases := []struct {
		name         string
		opcode       byte
		initialBit   uint32
		wantBranch   bool
		wantBitAfter uint32
	}{
		{"BBSS bit was set: branches, stays set", 0xE2, 1, true, 1},
		{"BBSS bit was clear: no branch, gets set", 0xE2, 0, false, 1},
		{"BBCS bit was clear: branches, gets set", 0xE3, 0, true, 1},
		{"BBCS bit was set: no branch, gets set", 0xE3, 1, false, 1},
		{"BBSC bit was set: branches, gets cleared", 0xE4, 1, true, 0},
		{"BBSC bit was clear: no branch, stays cleared", 0xE4, 0, false, 0},
		{"BBCC bit was clear: branches, stays cleared", 0xE5, 0, true, 0},
		{"BBCC bit was set: no branch, gets cleared", 0xE5, 1, false, 0},
		{"BBSSI behaves like BBSS", 0xE6, 1, true, 1},
		{"BBCCI behaves like BBCC", 0xE7, 0, true, 0},
	}
	
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.initialBit) // field is bit 0 of R1

			stepInstruction(t, e, tc.opcode, 0, regMode(vax.R1), 0x10)

			nextPC, taken := bitBranchTargets(0x10)
			
			want := nextPC
			if tc.wantBranch {
				want = taken
			}

			if got := cpu.GPR(vax.PC); got != want {
				t.Errorf("PC = %#x, want %#x", got, want)
			}

			gotBit, err := getRegisterField(cpu, 0, 1, vax.R1)
			if err != nil {
				t.Fatalf("getRegisterField: %v", err)
			}

			if gotBit != tc.wantBitAfter {
				t.Errorf("bit after = %d, want %d", gotBit, tc.wantBitAfter)
			}
		})
	}
}

func TestEmulBbStateMemoryBase(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0x00)

	bytes := []byte{0xE2, 4} // BBSS pos=4 (bit clear -> no branch, gets set)
	bytes = append(bytes, absoluteMode(0x2000)...)
	bytes = append(bytes, 0x10)
	stepInstruction(t, e, bytes...)

	nextPC := uint32(base) + uint32(len(bytes))
	if got := cpu.GPR(vax.PC); got != nextPC {
		t.Errorf("PC = %#x, want %#x (not taken)", got, nextPC)
	}
	
	got, err := getMemoryField(cpu, mem, 0, 8, 0x2000)
	if err != nil {
		t.Fatalf("getMemoryField: %v", err)
	}

	if want := uint32(0x10); got != want {
		t.Errorf("byte at 0x2000 = %#x, want %#x", got, want)
	}
}
