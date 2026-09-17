package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// regMode returns the addressing-mode byte for Register direct mode on r.
func regMode(r vax.Reg) byte { return 0x50 | byte(r) }

// setC sets the C condition code directly, for asserting an instruction
// leaves it unaffected.
func setC(cpu *vax.CPU, v bool) {
	psl := cpu.PSL()
	psl.SetC(v)
	cpu.SetPSL(psl)
}

func stepInstruction(t *testing.T, e *Engine, bytes ...byte) {
	t.Helper()
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, bytes...)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
}

func TestEmulMove(t *testing.T) {
	cases := []struct {
		name         string
		opcode       byte
		size         int
		src, dst     vax.Reg
		srcVal       uint32
		wantDstLow   uint32 // expected low `size` bytes of dst (rest preserved)
		wantN, wantZ bool
	}{
		{"MOVB", 0x90, 1, vax.R1, vax.R2, 0x7F, 0x7F, false, false},
		{"MOVB negative", 0x90, 1, vax.R1, vax.R2, 0x80, 0x80, true, false},
		{"MOVB zero", 0x90, 1, vax.R1, vax.R2, 0x00, 0x00, false, true},
		{"MOVW", 0xB0, 2, vax.R1, vax.R2, 0x8000, 0x8000, true, false},
		{"MOVL", 0xD0, 4, vax.R1, vax.R2, 0xFFFFFFFF, 0xFFFFFFFF, true, false},
		{"MOVZBL", 0x9A, 4, vax.R1, vax.R2, 0xFF, 0x000000FF, false, false},
		{"MOVZBW", 0x9B, 2, vax.R1, vax.R2, 0xFF, 0x00FF, false, false},
		{"MOVZWL", 0x3C, 4, vax.R1, vax.R2, 0xFFFF, 0x0000FFFF, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(tc.src, tc.srcVal)
			cpu.SetGPR(tc.dst, 0xAAAAAAAA)
			setC(cpu, true)

			stepInstruction(t, e, tc.opcode, regMode(tc.src), regMode(tc.dst))

			mask := uint32(1)<<(uint(tc.size)*8) - 1
			if tc.size >= 4 {
				mask = 0xFFFFFFFF
			}

			gotLow := cpu.GPR(tc.dst) & mask
			if gotLow != tc.wantDstLow {
				t.Errorf("dst low bits = %#x, want %#x", gotLow, tc.wantDstLow)
			}

			if tc.size < 4 {
				wantHigh := uint32(0xAAAAAAAA) &^ mask
				if cpu.GPR(tc.dst)&^mask != wantHigh {
					t.Errorf("dst high bits disturbed: got %#x", cpu.GPR(tc.dst))
				}
			}

			psl := cpu.PSL()
			if psl.N() != tc.wantN || psl.Z() != tc.wantZ {
				t.Errorf("N=%v Z=%v, want N=%v Z=%v", psl.N(), psl.Z(), tc.wantN, tc.wantZ)
			}

			if psl.V() {
				t.Error("V = true, want false")
			}

			if !psl.C() {
				t.Error("C = false, want unaffected (true)")
			}
		})
	}
}

func TestEmulMovq(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R2, 0x11111111) // source pair R2:R3
	cpu.SetGPR(vax.R3, 0x22222222)
	setC(cpu, true)

	stepInstruction(t, e, 0x7D, regMode(vax.R2), regMode(vax.R4))

	if cpu.GPR(vax.R4) != 0x11111111 || cpu.GPR(vax.R5) != 0x22222222 {
		t.Errorf("dst pair = %#x:%#x, want 11111111:22222222", cpu.GPR(vax.R4), cpu.GPR(vax.R5))
	}

	psl := cpu.PSL()
	if psl.N() || psl.Z() || psl.V() {
		t.Errorf("N=%v Z=%v V=%v, want all false", psl.N(), psl.Z(), psl.V())
	}

	if !psl.C() {
		t.Error("C = false, want unaffected (true)")
	}
}

func TestEmulMovqZero(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R2, 0)
	cpu.SetGPR(vax.R3, 0)

	stepInstruction(t, e, 0x7D, regMode(vax.R2), regMode(vax.R4))

	psl := cpu.PSL()
	if !psl.Z() || psl.N() {
		t.Errorf("N=%v Z=%v, want N=false Z=true", psl.N(), psl.Z())
	}
}

func TestEmulMcom(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x0000000F)
	cpu.SetGPR(vax.R2, 0xAAAAAAAA)
	setC(cpu, true)

	stepInstruction(t, e, 0x92, regMode(vax.R1), regMode(vax.R2)) // MCOMB

	if got := cpu.GPR(vax.R2); got != 0xAAAAAAF0 {
		t.Errorf("R2 = %#x, want 0xAAAAAAF0 (low byte complemented, rest preserved)", got)
	}

	psl := cpu.PSL()
	if !psl.N() || psl.Z() || psl.V() {
		t.Errorf("N=%v Z=%v V=%v, want N=true Z=false V=false", psl.N(), psl.Z(), psl.V())
	}

	if !psl.C() {
		t.Error("C = false, want unaffected (true)")
	}
}

func TestEmulMneg(t *testing.T) {
	cases := []struct {
		name                       string
		src                        uint32
		wantResultByte             byte
		wantN, wantZ, wantV, wantC bool
	}{
		{"positive", 0x05, 0xFB, true, false, false, true},
		{"negative", 0xFB /* -5 */, 0x05, false, false, false, true},
		{"zero", 0x00, 0x00, false, true, false, false},
		{"overflow (largest negative)", 0x80, 0x80, true, false, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.src)
			cpu.SetGPR(vax.R2, 0xAAAAAA00)

			stepInstruction(t, e, 0x8E, regMode(vax.R1), regMode(vax.R2)) // MNEGB

			if got := byte(cpu.GPR(vax.R2)); got != tc.wantResultByte {
				t.Errorf("result = %#x, want %#x", got, tc.wantResultByte)
			}
			
			psl := cpu.PSL()
			if psl.N() != tc.wantN || psl.Z() != tc.wantZ || psl.V() != tc.wantV || psl.C() != tc.wantC {
				t.Errorf("N=%v Z=%v V=%v C=%v, want N=%v Z=%v V=%v C=%v",
					psl.N(), psl.Z(), psl.V(), psl.C(), tc.wantN, tc.wantZ, tc.wantV, tc.wantC)
			}
		})
	}
}

func TestEmulMnegWordAndLong(t *testing.T) {
	// Sanity check the shared emulMneg across sizes, not just byte.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x00008000)                                // word: -32768, the size's minSigned
	stepInstruction(t, e, 0xAE, regMode(vax.R1), regMode(vax.R2)) // MNEGW

	if got := uint16(cpu.GPR(vax.R2)); got != 0x8000 {
		t.Errorf("MNEGW overflow result = %#x, want 0x8000 (unchanged)", got)
	}

	if !cpu.PSL().V() {
		t.Error("MNEGW overflow: V = false, want true")
	}

	cpu.SetGPR(vax.R1, 0x80000000)                                // long: minSigned
	stepInstruction(t, e, 0xCE, regMode(vax.R1), regMode(vax.R2)) // MNEGL

	if got := cpu.GPR(vax.R2); got != 0x80000000 {
		t.Errorf("MNEGL overflow result = %#x, want 0x80000000 (unchanged)", got)
	}

	if !cpu.PSL().V() {
		t.Error("MNEGL overflow: V = false, want true")
	}
}
