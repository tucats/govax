package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulIncByte(t *testing.T) {
	cases := []struct {
		name                       string
		src                        uint32
		wantResult                 byte
		wantN, wantZ, wantV, wantC bool
	}{
		{"ordinary", 0x05, 0x06, false, false, false, false},
		{"to zero", 0xFF /* -1 */, 0x00, false, true, false, true},
		// The C source's byte-range V check (data>255||data<-256) misses
		// this: incrementing the largest positive byte (0x7F=127) overflows
		// per the manual's own INC note, but 128 is neither >255 nor
		// <-256. See docs/DEVIATIONS.md.
		{"overflow (largest positive)", 0x7F, 0x80, true, false, true, false},
		// The C source's data&0x100 carry check also misses this: 0xFF is
		// -1 signed, so data=-1+1=0, and 0&0x100==0 reports no carry -- but
		// the true unsigned carry (255+1=256) should set C.
		{"carry (0xFF unsigned)", 0xFF, 0x00, false, true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.src)

			stepInstruction(t, e, 0x96, regMode(vax.R1)) // INCB

			if got := byte(cpu.GPR(vax.R1)); got != tc.wantResult {
				t.Errorf("result = %#x, want %#x", got, tc.wantResult)
			}
			psl := cpu.PSL()
			if psl.N() != tc.wantN || psl.Z() != tc.wantZ || psl.V() != tc.wantV || psl.C() != tc.wantC {
				t.Errorf("N=%v Z=%v V=%v C=%v, want N=%v Z=%v V=%v C=%v",
					psl.N(), psl.Z(), psl.V(), psl.C(), tc.wantN, tc.wantZ, tc.wantV, tc.wantC)
			}
		})
	}
}

func TestEmulDecByte(t *testing.T) {
	cases := []struct {
		name                       string
		src                        uint32
		wantResult                 byte
		wantN, wantZ, wantV, wantC bool
	}{
		{"ordinary", 0x05, 0x04, false, false, false, false},
		{"borrow (zero)", 0x00, 0xFF, true, false, false, true},
		{"no borrow (nonzero)", 0x01, 0x00, false, true, false, false},
		{"overflow (largest negative)", 0x80, 0x7F, false, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.src)

			stepInstruction(t, e, 0x97, regMode(vax.R1)) // DECB

			if got := byte(cpu.GPR(vax.R1)); got != tc.wantResult {
				t.Errorf("result = %#x, want %#x", got, tc.wantResult)
			}
			psl := cpu.PSL()
			if psl.N() != tc.wantN || psl.Z() != tc.wantZ || psl.V() != tc.wantV || psl.C() != tc.wantC {
				t.Errorf("N=%v Z=%v V=%v C=%v, want N=%v Z=%v V=%v C=%v",
					psl.N(), psl.Z(), psl.V(), psl.C(), tc.wantN, tc.wantZ, tc.wantV, tc.wantC)
			}
		})
	}
}

func TestEmulIncDecLongword(t *testing.T) {
	// The longword `data == udata` truncation-comparison trick in the C
	// source can never detect overflow once LONGWORD is genuinely 32-bit
	// (see docs/DEVIATIONS.md); confirm the Go port actually does.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x7FFFFFFF) // INT32_MAX

	stepInstruction(t, e, 0xD6, regMode(vax.R1)) // INCL

	if got := cpu.GPR(vax.R1); got != 0x80000000 {
		t.Errorf("result = %#x, want 0x80000000", got)
	}
	
	if !cpu.PSL().V() {
		t.Error("V = false, want true (INT32_MAX + 1 overflows)")
	}

	cpu.SetGPR(vax.R1, 0x80000000) // INT32_MIN
	stepInstruction(t, e, 0xD7, regMode(vax.R1))

	if got := cpu.GPR(vax.R1); got != 0x7FFFFFFF {
		t.Errorf("result = %#x, want 0x7fffffff", got)
	}

	if !cpu.PSL().V() {
		t.Error("V = false, want true (INT32_MIN - 1 overflows)")
	}
}

func TestEmulIncWordPreservesUpperBytes(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xAAAA0005)

	stepInstruction(t, e, 0xB6, regMode(vax.R1)) // INCW

	if got := cpu.GPR(vax.R1); got != 0xAAAA0006 {
		t.Errorf("R1 = %#x, want 0xAAAA0006", got)
	}
}
