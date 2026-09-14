package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulMatchc(t *testing.T) {
	t.Run("finds the substring, backtracking through a false start", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'b', 'c') // object
		putBytes(t, cpu, mem, 0x2000, 'a', 'b', 'c', 'a', 'b', 'c')

		bytes := []byte{0x39, 2}
		bytes = append(bytes, absoluteMode(0x3000)...) // objlen, objaddr
		bytes = append(bytes, 6)
		bytes = append(bytes, absoluteMode(0x2000)...) // srclen, srcaddr
		stepInstruction(t, e, bytes...)

		got := cpu.PSL()
		if !got.Z() {
			t.Error("Z = false, want true (match found)")
		}
		if cpu.GPR(vax.R0) != 0 {
			t.Errorf("R0 = %d, want 0 (match found)", cpu.GPR(vax.R0))
		}
		if cpu.GPR(vax.R1) != 0x3002 {
			t.Errorf("R1 = %#x, want 0x3002 (one past the object string)", cpu.GPR(vax.R1))
		}
		if cpu.GPR(vax.R2) != 3 {
			t.Errorf("R2 = %d, want 3 (bytes remaining in source after the match)", cpu.GPR(vax.R2))
		}
		if cpu.GPR(vax.R3) != 0x2003 {
			t.Errorf("R3 = %#x, want 0x2003 (one past the last matched byte)", cpu.GPR(vax.R3))
		}
	})

	t.Run("no match", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'x', 'y')
		putBytes(t, cpu, mem, 0x2000, 'a', 'b', 'c')

		bytes := []byte{0x39, 2}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, 3)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if cpu.PSL().Z() {
			t.Error("Z = true, want false (no match)")
		}
		if cpu.GPR(vax.R0) != 2 {
			t.Errorf("R0 = %d, want 2 (object length)", cpu.GPR(vax.R0))
		}
		if cpu.GPR(vax.R1) != 0x3000 {
			t.Errorf("R1 = %#x, want 0x3000 (object address)", cpu.GPR(vax.R1))
		}
		if cpu.GPR(vax.R2) != 0 {
			t.Errorf("R2 = %d, want 0", cpu.GPR(vax.R2))
		}
		if cpu.GPR(vax.R3) != 0x2003 {
			t.Errorf("R3 = %#x, want 0x2003 (one past the source string)", cpu.GPR(vax.R3))
		}
	})

	t.Run("zero-length object: treated as found", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'a', 'b', 'c')

		bytes := []byte{0x39, 0}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, 3)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (zero-length object)")
		}
		if cpu.GPR(vax.R1) != 0x3000 || cpu.GPR(vax.R2) != 3 || cpu.GPR(vax.R3) != 0x2000 {
			t.Errorf("R1/R2/R3 = %#x/%d/%#x, want 0x3000/3/0x2000",
				cpu.GPR(vax.R1), cpu.GPR(vax.R2), cpu.GPR(vax.R3))
		}
	})

	t.Run("zero-length source, nonzero object: treated as not found", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, 'a', 'b', 'c')

		bytes := []byte{0x39, 3}
		bytes = append(bytes, absoluteMode(0x3000)...)
		bytes = append(bytes, 0)
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if cpu.PSL().Z() {
			t.Error("Z = true, want false (zero-length source)")
		}
		if cpu.GPR(vax.R0) != 3 || cpu.GPR(vax.R1) != 0x3000 || cpu.GPR(vax.R2) != 0 || cpu.GPR(vax.R3) != 0x2000 {
			t.Errorf("R0/R1/R2/R3 = %d/%#x/%d/%#x, want 3/0x3000/0/0x2000",
				cpu.GPR(vax.R0), cpu.GPR(vax.R1), cpu.GPR(vax.R2), cpu.GPR(vax.R3))
		}
	})
}
