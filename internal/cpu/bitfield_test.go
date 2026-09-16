package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// absoluteMode returns the addressing-mode byte for Absolute mode (@#addr),
// followed by the 4-byte address to append to an instruction's bytes.
func absoluteMode(addr uint32) []byte {
	return []byte{0x9F, byte(addr), byte(addr >> 8), byte(addr >> 16), byte(addr >> 24)}
}

func TestGetRegisterField(t *testing.T) {
	cpu, _ := fixture()
	cpu.SetGPR(vax.R2, 0xABCD1234)
	cpu.SetGPR(vax.R3, 0x000000F0)

	t.Run("within one register", func(t *testing.T) {
		got, err := getRegisterField(cpu, 4, 8, vax.R2)
		if err != nil {
			t.Fatalf("getRegisterField: %v", err)
		}

		if want := uint32(0x23); got != want {
			t.Errorf("field = %#x, want %#x", got, want)
		}
	})

	t.Run("exactly fills the base register (position=0, size=32)", func(t *testing.T) {
		// The C source's `position + size < 32` boundary check would wrongly
		// take the two-register path here; getRegisterField must not.
		got, err := getRegisterField(cpu, 0, 32, vax.R2)
		if err != nil {
			t.Fatalf("getRegisterField: %v", err)
		}

		if got != 0xABCD1234 {
			t.Errorf("field = %#x, want 0xABCD1234 (R3 must not be consulted)", got)
		}
	})

	t.Run("spans base and base+1", func(t *testing.T) {
		// position=28, size=8: 4 bits from the top of R2 (0xA), 4 bits from
		// the bottom of R3 (0x0), packed low-bits-first.
		got, err := getRegisterField(cpu, 28, 8, vax.R2)
		if err != nil {
			t.Fatalf("getRegisterField: %v", err)
		}

		if want := uint32(0x0A); got != want {
			t.Errorf("field = %#x, want %#x", got, want)
		}
	})

	t.Run("size too large faults", func(t *testing.T) {
		if _, err := getRegisterField(cpu, 0, 33, vax.R2); !isReservedOp(err) {
			t.Errorf("err = %v, want reserved-operand fault", err)
		}
	})

	t.Run("negative position faults", func(t *testing.T) {
		if _, err := getRegisterField(cpu, -1, 8, vax.R2); !isReservedOp(err) {
			t.Errorf("err = %v, want reserved-operand fault", err)
		}
	})

	t.Run("position beyond 31 faults", func(t *testing.T) {
		if _, err := getRegisterField(cpu, 32, 1, vax.R2); !isReservedOp(err) {
			t.Errorf("err = %v, want reserved-operand fault", err)
		}
	})

	t.Run("PC base cannot span into a nonexistent R16", func(t *testing.T) {
		if _, err := getRegisterField(cpu, 30, 4, vax.PC); !isReservedOp(err) {
			t.Errorf("err = %v, want reserved-operand fault", err)
		}
	})
}

func TestSetRegisterField(t *testing.T) {
	t.Run("within one register preserves surrounding bits", func(t *testing.T) {
		cpu, _ := fixture()
		cpu.SetGPR(vax.R2, 0xFFFFFFFF)

		if err := setRegisterField(cpu, 4, 8, vax.R2, 0x00); err != nil {
			t.Fatalf("setRegisterField: %v", err)
		}

		if want := uint32(0xFFFFF00F); cpu.GPR(vax.R2) != want {
			t.Errorf("R2 = %#x, want %#x", cpu.GPR(vax.R2), want)
		}
	})

	t.Run("spans base and base+1", func(t *testing.T) {
		cpu, _ := fixture()
		cpu.SetGPR(vax.R2, 0x00000000)
		cpu.SetGPR(vax.R3, 0x00000000)

		if err := setRegisterField(cpu, 28, 8, vax.R2, 0xFF); err != nil {
			t.Fatalf("setRegisterField: %v", err)
		}

		if cpu.GPR(vax.R2) != 0xF0000000 {
			t.Errorf("R2 = %#x, want 0xF0000000", cpu.GPR(vax.R2))
		}

		if cpu.GPR(vax.R3) != 0x0000000F {
			t.Errorf("R3 = %#x, want 0x0000000F", cpu.GPR(vax.R3))
		}
	})

	t.Run("round-trips through getRegisterField", func(t *testing.T) {
		cpu, _ := fixture()
		cpu.SetGPR(vax.R2, 0)
		cpu.SetGPR(vax.R3, 0)

		if err := setRegisterField(cpu, 0, 32, vax.R2, 0xDEADBEEF); err != nil {
			t.Fatalf("setRegisterField: %v", err)
		}

		got, err := getRegisterField(cpu, 0, 32, vax.R2)
		if err != nil {
			t.Fatalf("getRegisterField: %v", err)
		}

		if got != 0xDEADBEEF {
			t.Errorf("round-trip = %#x, want 0xDEADBEEF", got)
		}

		if cpu.GPR(vax.R3) != 0 {
			t.Errorf("R3 = %#x, want untouched 0", cpu.GPR(vax.R3))
		}
	})
}

func TestGetMemoryField(t *testing.T) {
	cpu, mem := fixture()
	// Bytes at 0x2000: 0x00, 0xF0, 0x0F, 0x00 -- a field spanning bytes 1-2
	// at bit offset 4 reads the middle nibble of each: 0x0F.
	putBytes(t, cpu, mem, 0x2000, 0x00, 0xF0, 0x0F, 0x00)

	t.Run("within one byte", func(t *testing.T) {
		got, err := getMemoryField(cpu, mem, 12, 4, 0x2000) // byte 1, bits 4-7
		if err != nil {
			t.Fatalf("getMemoryField: %v", err)
		}

		if want := uint32(0xF); got != want {
			t.Errorf("field = %#x, want %#x", got, want)
		}
	})

	t.Run("spans a byte boundary", func(t *testing.T) {
		got, err := getMemoryField(cpu, mem, 12, 8, 0x2000) // bits 12-19
		if err != nil {
			t.Fatalf("getMemoryField: %v", err)
		}

		if want := uint32(0xFF); got != want {
			t.Errorf("field = %#x, want %#x", got, want)
		}
	})

	t.Run("negative position reaches before base", func(t *testing.T) {
		got, err := getMemoryField(cpu, mem, -4, 4, 0x2001) // byte 0's top nibble
		if err != nil {
			t.Fatalf("getMemoryField: %v", err)
		}

		if want := uint32(0x0); got != want {
			t.Errorf("field = %#x, want %#x", got, want)
		}
	})

	t.Run("size zero never touches memory", func(t *testing.T) {
		got, err := getMemoryField(cpu, mem, 0, 0, 0xFFFFFFFF) // address would fault if read
		if err != nil {
			t.Fatalf("getMemoryField: %v", err)
		}

		if got != 0 {
			t.Errorf("field = %#x, want 0", got)
		}
	})

	t.Run("size too large faults", func(t *testing.T) {
		if _, err := getMemoryField(cpu, mem, 0, 33, 0x2000); !isReservedOp(err) {
			t.Errorf("err = %v, want reserved-operand fault", err)
		}
	})
}

func TestSetMemoryField(t *testing.T) {
	cpu, mem := fixture()
	putBytes(t, cpu, mem, 0x2000, 0xFF, 0xFF, 0xFF, 0xFF)

	if err := setMemoryField(cpu, mem, 4, 16, 0x2000, 0x0000); err != nil {
		t.Fatalf("setMemoryField: %v", err)
	}

	got, err := getMemoryField(cpu, mem, 0, 32, 0x2000)
	if err != nil {
		t.Fatalf("getMemoryField: %v", err)
	}

	// Bits 4-19 cleared, bits 0-3 and 20-31 left as 1.
	if want := uint32(0xFFF0000F); got != want {
		t.Errorf("field = %#x, want %#x", got, want)
	}
}

// isReservedOp reports whether err is the reserved-operand fault
// getRegisterField/getMemoryField/setRegisterField/setMemoryField raise for
// an out-of-range position or size.
func isReservedOp(err error) bool {
	f, ok := err.(*Fault)

	return ok && f.Code == ExcReservedOp
}

func TestEmulExtv(t *testing.T) {
	cases := []struct {
		name       string
		opcode     byte
		regValue   uint32
		wantResult uint32
		wantN      bool
	}{
		// Bits 4-11 of 0x00000AB0 are 0xAB (its own top bit set); EXTV
		// sign-extends the 8-bit field, EXTZV zero-extends it.
		{"EXTV sign-extends", 0xEE, 0x00000AB0, 0xFFFFFFAB, true},
		{"EXTZV zero-extends", 0xEF, 0x00000AB0, 0x000000AB, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.regValue)
			psl := cpu.PSL()
			psl.SetNZVC(false, false, true, true)
			cpu.SetPSL(psl)

			// EXTx pos=4, size=8, R1, R2
			stepInstruction(t, e, tc.opcode, 4, 8, regMode(vax.R1), regMode(vax.R2))

			if got := cpu.GPR(vax.R2); got != tc.wantResult {
				t.Errorf("R2 = %#x, want %#x", got, tc.wantResult)
			}
			got := cpu.PSL()
			if got.N() != tc.wantN {
				t.Errorf("N = %v, want %v", got.N(), tc.wantN)
			}

			if got.Z() {
				t.Error("Z = true, want false (nonzero result)")
			}

			if got.V() || got.C() {
				t.Errorf("V=%v C=%v, want both false", got.V(), got.C())
			}
		})
	}
}

func TestEmulExtvZeroResultSetsZ(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0)

	stepInstruction(t, e, 0xEF, 0, 8, regMode(vax.R1), regMode(vax.R2)) // EXTZV

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}

func TestEmulExtvMemoryBase(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 0x00, 0xF0, 0x0F, 0x00)

	bytes := []byte{0xEF, 12, 8} // EXTZV pos=12, size=8
	bytes = append(bytes, absoluteMode(0x2000)...)
	bytes = append(bytes, regMode(vax.R2))
	stepInstruction(t, e, bytes...)

	if got := cpu.GPR(vax.R2); got != 0xFF {
		t.Errorf("R2 = %#x, want 0xFF", got)
	}
}

func TestEmulExtvImmediateBaseFaults(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, e.mem, base, 0xEF, 4, 8, 0x00, regMode(vax.R2)) // base = short literal 0
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

func TestEmulCmpv(t *testing.T) {
	cases := []struct {
		name       string
		opcode     byte
		compareLit byte
		wantN      bool
		wantZ      bool
	}{
		// Field (bits 4-11 of 0x00000AB0) is 0xAB: CMPV sign-extends it to
		// -85, which is LSS the literal 5.
		{"CMPV sign-extended field is negative, less than 5", 0xEC, 5, true, false},
		// CMPZV zero-extends the same bits to 0xAB (171); compared against
		// 5, it's greater, so N is false.
		{"CMPZV zero-extended field is 171, greater than 5", 0xED, 5, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, 0x00000AB0)

			stepInstruction(t, e, tc.opcode, 4, 8, regMode(vax.R1), tc.compareLit)

			got := cpu.PSL()
			if got.N() != tc.wantN {
				t.Errorf("N = %v, want %v", got.N(), tc.wantN)
			}

			if got.Z() != tc.wantZ {
				t.Errorf("Z = %v, want %v", got.Z(), tc.wantZ)
			}
			
			if got.V() {
				t.Error("V = true, want false")
			}
		})
	}
}

func TestEmulCmpvEqualSetsZ(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0x2F) // field bits 0-7 == 0x2F

	stepInstruction(t, e, 0xED, 0, 8, regMode(vax.R1), 0x2F) // CMPZV vs literal 0x2F

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}

func TestEmulInsv(t *testing.T) {
	t.Run("register base preserves surrounding bits", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		cpu.SetGPR(vax.R1, 0x00)
		psl := cpu.PSL()
		psl.SetNZVC(true, true, true, true)
		cpu.SetPSL(psl)
		cpu.SetGPR(vax.R2, 0xFFFFFFFF)

		// INSV R1,#4,#8,R2 -- clear an 8-bit field at position 4 of R2.
		stepInstruction(t, e, 0xF0, regMode(vax.R1), 4, 8, regMode(vax.R2))

		if want := uint32(0xFFFFF00F); cpu.GPR(vax.R2) != want {
			t.Errorf("R2 = %#x, want %#x", cpu.GPR(vax.R2), want)
		}

		got := cpu.PSL()
		if got.N() != true || got.Z() != true || got.V() != true || got.C() != true {
			t.Errorf("PSL = %+v, want unaffected (all true, as pre-set)", got)
		}
	})

	t.Run("memory base", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 0x00, 0x00, 0x00, 0x00)
		cpu.SetGPR(vax.R1, 0xFF) // 0xFF isn't representable as a short literal

		bytes := []byte{0xF0, regMode(vax.R1), 12, 8} // INSV R1,pos=12,size=8,base
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		got, err := getMemoryField(cpu, mem, 0, 32, 0x2000)
		if err != nil {
			t.Fatalf("getMemoryField: %v", err)
		}

		if want := uint32(0x000FF000); got != want {
			t.Errorf("memory field = %#x, want %#x", got, want)
		}
	})
}

func TestEmulFf(t *testing.T) {
	cases := []struct {
		name       string
		opcode     byte
		regValue   uint32
		wantResult uint32
		wantZ      bool
	}{
		// FFS on 0b...0001_0000 (bit 4 set), searching from position 0: the
		// first set bit is at position 4.
		{"FFS finds a set bit", 0xEA, 0x10, 4, false},
		// FFC on the same value: the first clear bit is position 0.
		{"FFC finds a clear bit", 0xEB, 0x10, 0, false},
		// FFS with no set bits in the field: result is position+size.
		{"FFS finds nothing", 0xEA, 0x00, 8, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)
			cpu.SetGPR(vax.R1, tc.regValue)

			// FFx pos=0, size=8, R1, R2
			stepInstruction(t, e, tc.opcode, 0, 8, regMode(vax.R1), regMode(vax.R2))

			if got := cpu.GPR(vax.R2); got != tc.wantResult {
				t.Errorf("R2 = %#x, want %#x", got, tc.wantResult)
			}

			if got := cpu.PSL().Z(); got != tc.wantZ {
				t.Errorf("Z = %v, want %v", got, tc.wantZ)
			}
		})
	}
}

func TestEmulFfZeroSizeField(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	cpu.SetGPR(vax.R1, 0xFFFFFFFF)

	stepInstruction(t, e, 0xEA, 9, 0, regMode(vax.R1), regMode(vax.R2)) // FFS pos=9, size=0

	if got := cpu.GPR(vax.R2); got != 9 {
		t.Errorf("R2 = %#x, want 9 (position returned unchanged)", got)
	}

	if !cpu.PSL().Z() {
		t.Error("Z = false, want true")
	}
}
