package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

func TestEmulMovc3(t *testing.T) {
	t.Run("non-overlapping forward copy (src > dst)", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'H', 'e', 'l', 'l', 'o')
		psl := cpu.PSL()
		psl.SetC(true)
		cpu.SetPSL(psl)

		bytes := []byte{0x28, 5}
		bytes = append(bytes, absoluteMode(0x3000)...) // src
		bytes = append(bytes, absoluteMode(0x2000)...) // dst
		stepInstruction(t, e, bytes...)

		for i, want := range []byte("Hello") {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}

			if got != want {
				t.Errorf("byte %d = %q, want %q", i, got, want)
			}
		}

		if cpu.GPR(registers.R1) != 0x3005 {
			t.Errorf("R1 = %#x, want 0x3005 (one past source)", cpu.GPR(registers.R1))
		}

		if cpu.GPR(registers.R3) != 0x2005 {
			t.Errorf("R3 = %#x, want 0x2005 (one past destination)", cpu.GPR(registers.R3))
		}

		if cpu.GPR(registers.R0) != 0 || cpu.GPR(registers.R2) != 0 || cpu.GPR(registers.R4) != 0 || cpu.GPR(registers.R5) != 0 {
			t.Error("R0/R2/R4/R5 = nonzero, want all 0")
		}

		got := cpu.PSL()
		if got.N() || !got.Z() || got.V() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=1 V=0 C=0", got)
		}
	})

	t.Run("overlapping copy shifted right copies backward", func(t *testing.T) {
		// src < dst and the ranges overlap: a forward copy would corrupt
		// not-yet-read source bytes, so this must copy from the end.
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'A', 'B', 'C', 'D', 'E')

		bytes := []byte{0x28, 4}
		bytes = append(bytes, absoluteMode(0x2000)...) // src
		bytes = append(bytes, absoluteMode(0x2001)...) // dst (overlaps, src < dst)
		stepInstruction(t, e, bytes...)

		want := []byte("AABCD")
		for i, w := range want {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}

			if got != w {
				t.Errorf("byte %d = %q, want %q (result %q)", i, got, w, want)
			}
		}
	})
}

func TestEmulMovc5(t *testing.T) {
	t.Run("destination longer: excess filled", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'H', 'i')
		putBytes(t, cpu, mem, 0x2000, 0, 0, 0, 0, 0)

		bytes := []byte{0x2C, 2}
		bytes = append(bytes, absoluteMode(0x3000)...) // srcaddr
		bytes = append(bytes, '.')                     // fill
		bytes = append(bytes, 5)
		bytes = append(bytes, absoluteMode(0x2000)...) // dstaddr
		stepInstruction(t, e, bytes...)

		want := []byte("Hi...")
		for i, w := range want {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}

			if got != w {
				t.Errorf("byte %d = %q, want %q (result %q)", i, got, w, want)
			}
		}

		if cpu.GPR(registers.R0) != 0 {
			t.Errorf("R0 = %#x, want 0 (source not longer than destination)", cpu.GPR(registers.R0))
		}
		
		got := cpu.PSL()
		if !got.N() || got.Z() || got.V() || !got.C() {
			// srclen(2) LSS dstlen(5) is true (N); LSSU true too (C).
			t.Errorf("PSL = %+v, want N=1 Z=0 V=0 C=1 (srclen 2 < dstlen 5)", got)
		}
	})

	t.Run("source longer: excess left unmoved, reported in R0", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'H', 'e', 'l', 'l', 'o')
		putBytes(t, cpu, mem, 0x2000, 0, 0)

		bytes := []byte{0x2C, 5}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, '.')
		bytes = append(bytes, 2)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		want := []byte("He")
		for i, w := range want {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}

			if got != w {
				t.Errorf("byte %d = %q, want %q", i, got, w)
			}
		}

		if cpu.GPR(registers.R0) != 3 {
			t.Errorf("R0 = %d, want 3 (unmoved source bytes)", cpu.GPR(registers.R0))
		}

		if cpu.GPR(registers.R1) != 0x3002 {
			t.Errorf("R1 = %#x, want 0x3002 (one past last byte moved)", cpu.GPR(registers.R1))
		}
	})

	t.Run("equal lengths: no fill, Z set", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'H', 'i')
		putBytes(t, cpu, mem, 0x2000, 0, 0)

		bytes := []byte{0x2C, 2}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, '.')
		bytes = append(bytes, 2)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		for i, w := range []byte("Hi") {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}

			if got != w {
				t.Errorf("byte %d = %q, want %q", i, got, w)
			}
		}

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (equal lengths)")
		}
	})
}

// TestStringLengthsUnsigned: a length of 32,768 or more is a count, not a
// negative number (the manual's strings are 0 to 65,535 bytes long), for
// MOVC3, LOCC, SCANC, and CMPC3.
func TestStringLengthsUnsigned(t *testing.T) {
	const (
		src    = 0x10000
		dst    = 0x30000
		length = 0x9000
	)

	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, src+length-1, 'X')

	run := func(name string, bytes []byte) {
		t.Helper()
		cpu.SetGPR(registers.R0, 0xDEAD)
		stepInstruction(t, e, bytes...)
	}

	word := immediate(byte(length&0xFF), byte(length>>8))

	run("MOVC3", append(append(append([]byte{0x28}, word...), absoluteMode(src)...), absoluteMode(dst)...))

	if got, _ := mem.LoadByte(cpu, dst+length-1); got != 'X' {
		t.Errorf("MOVC3: last byte %q, want 'X'", got)
	}

	if got := cpu.GPR(registers.R3); got != dst+length {
		t.Errorf("MOVC3: R3 = %#x, want %#x", got, dst+length)
	}

	run("LOCC", append(append(append([]byte{0x3A}, immediate('X')...), word...), absoluteMode(src)...))

	if got := cpu.GPR(registers.R0); got != 1 {
		t.Errorf("LOCC: R0 = %#x, want 1 (the last byte)", got)
	}

	run("CMPC3", append(append(append([]byte{0x29}, word...), absoluteMode(src)...), absoluteMode(dst)...))

	if got, z := cpu.GPR(registers.R0), cpu.PSL().Z(); got != 0 || !z {
		t.Errorf("CMPC3: R0 = %#x, Z %v; want 0, equal", got, z)
	}

	putBytes(t, cpu, mem, 0x50000+'X', 1) // the table: only X has bit 0
	run("SCANC", append(append(append(append([]byte{0x2A}, word...), absoluteMode(src)...), absoluteMode(0x50000)...), immediate(1)...))

	if got := cpu.GPR(registers.R0); got != 1 {
		t.Errorf("SCANC: R0 = %#x, want 1 (the last byte)", got)
	}
}
