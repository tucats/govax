package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulMovtc(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x3000, 'h', 'i') // source
	for n := 0; n < 256; n++ {
		v := byte(n)
		if v >= 'a' && v <= 'z' {
			v -= 'a' - 'A'
		}
		putBytes(t, cpu, mem, 0x4000+uint32(n), v)
	}
	putBytes(t, cpu, mem, 0x2000, 0, 0, 0, 0, 0)

	bytes := []byte{0x2E, 2}
	bytes = append(bytes, absoluteMode(0x3000)...) // srcaddr
	bytes = append(bytes, '.')                     // fill
	bytes = append(bytes, absoluteMode(0x4000)...) // tbladdr
	bytes = append(bytes, 5)
	bytes = append(bytes, absoluteMode(0x2000)...) // dstaddr
	stepInstruction(t, e, bytes...)

	want := []byte("HI...")
	for i, w := range want {
		got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
		if err != nil {
			t.Fatalf("LoadByte: %v", err)
		}
		if got != w {
			t.Errorf("byte %d = %q, want %q (result %q)", i, got, w, want)
		}
	}
	if cpu.GPR(vax.R3) != 0x4000 {
		t.Errorf("R3 = %#x, want 0x4000 (table address)", cpu.GPR(vax.R3))
	}
}

func TestEmulMovtcOverlappingBackwardCopyTranslatesCorrectly(t *testing.T) {
	// Regression for the tbladdr+tmp2 (address) vs tbladdr+ch (byte value)
	// bug: src < dst, forcing the backward-copy branch.
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 'a', 'b', 'c')
	for n := 0; n < 256; n++ {
		v := byte(n)
		if v >= 'a' && v <= 'z' {
			v -= 'a' - 'A'
		}
		putBytes(t, cpu, mem, 0x4000+uint32(n), v)
	}

	bytes := []byte{0x2E, 3}
	bytes = append(bytes, absoluteMode(0x2000)...) // srcaddr
	bytes = append(bytes, '.')
	bytes = append(bytes, absoluteMode(0x4000)...) // tbladdr
	bytes = append(bytes, 3)
	bytes = append(bytes, absoluteMode(0x2001)...) // dstaddr overlaps, src < dst
	stepInstruction(t, e, bytes...)

	// Byte 0 (0x2000) is never written -- the destination range is
	// 0x2001-0x2003 -- so it stays the untranslated original 'a'.
	want := []byte("aABC")
	for i, w := range want {
		got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
		if err != nil {
			t.Fatalf("LoadByte: %v", err)
		}
		if got != w {
			t.Errorf("byte %d = %q, want %q (result %q)", i, got, w, want)
		}
	}
}

func TestEmulMovtuc(t *testing.T) {
	t.Run("translates until escape", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'h', 'i', '!', 'x')
		for n := 0; n < 256; n++ {
			v := byte(n)
			if v >= 'a' && v <= 'z' {
				v -= 'a' - 'A'
			}
			putBytes(t, cpu, mem, 0x4000+uint32(n), v)
		}
		putBytes(t, cpu, mem, 0x2000, 0, 0, 0, 0)

		bytes := []byte{0x2F, 4}
		bytes = append(bytes, absoluteMode(0x3000)...) // srcaddr
		bytes = append(bytes, '!')                     // esc
		bytes = append(bytes, absoluteMode(0x4000)...) // tbladdr
		bytes = append(bytes, 4)
		bytes = append(bytes, absoluteMode(0x2000)...) // dstaddr
		stepInstruction(t, e, bytes...)

		want := []byte("HI")
		for i, w := range want {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}
			if got != w {
				t.Errorf("byte %d = %q, want %q", i, got, w)
			}
		}
		if !cpu.PSL().V() {
			t.Error("V = false, want true (terminated by escape)")
		}
		if cpu.GPR(vax.R2) != 0 {
			t.Errorf("R2 = %d, want 0", cpu.GPR(vax.R2))
		}
		if cpu.GPR(vax.R0) != 2 {
			t.Errorf("R0 = %d, want 2 (including the escaping byte)", cpu.GPR(vax.R0))
		}
	})

	t.Run("no escape: full translation, V clear", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'h', 'i')
		for n := 0; n < 256; n++ {
			v := byte(n)
			if v >= 'a' && v <= 'z' {
				v -= 'a' - 'A'
			}
			putBytes(t, cpu, mem, 0x4000+uint32(n), v)
		}
		putBytes(t, cpu, mem, 0x2000, 0, 0)

		bytes := []byte{0x2F, 2}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, '!')
		bytes = append(bytes, absoluteMode(0x4000)...)
		bytes = append(bytes, 2)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		want := []byte("HI")
		for i, w := range want {
			got, err := mem.LoadByte(cpu, 0x2000+uint32(i))
			if err != nil {
				t.Fatalf("LoadByte: %v", err)
			}
			if got != w {
				t.Errorf("byte %d = %q, want %q", i, got, w)
			}
		}
		if cpu.PSL().V() {
			t.Error("V = true, want false (not terminated by escape)")
		}
		if cpu.GPR(vax.R0) != 0 {
			t.Errorf("R0 = %d, want 0", cpu.GPR(vax.R0))
		}
	})

	t.Run("remaining lengths sharing no set bit still both nonzero", func(t *testing.T) {
		// Regression for the `tmp1 & tmp3` (bitwise) vs `tmp1 != 0 && tmp3
		// != 0` bug: lengths 2 and 1 share no set bit, so the buggy
		// condition would never enter the loop at all.
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'h', 'i')
		for n := 0; n < 256; n++ {
			putBytes(t, cpu, mem, 0x4000+uint32(n), byte(n))
		}
		putBytes(t, cpu, mem, 0x2000, 0)

		bytes := []byte{0x2F, 2}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, '!')
		bytes = append(bytes, absoluteMode(0x4000)...)
		bytes = append(bytes, 1)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		got, err := mem.LoadByte(cpu, 0x2000)
		if err != nil {
			t.Fatalf("LoadByte: %v", err)
		}
		if got != 'h' {
			t.Errorf("byte 0 = %q, want 'h' (loop must run despite 2 & 1 == 0)", got)
		}
	})
}
