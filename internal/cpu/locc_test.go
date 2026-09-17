package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulLocc(t *testing.T) {
	t.Run("finds the character", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'a', 'b', 'c', 'd')

		bytes := []byte{0x3A, 0x8F, 'c', 4}
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if cpu.PSL().Z() {
			t.Error("Z = true, want false (character found)")
		}

		if cpu.GPR(vax.R0) != 2 {
			t.Errorf("R0 = %d, want 2 (remaining including the match)", cpu.GPR(vax.R0))
		}

		if cpu.GPR(vax.R1) != 0x2002 {
			t.Errorf("R1 = %#x, want 0x2002", cpu.GPR(vax.R1))
		}
	})

	t.Run("character not found", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'a', 'b', 'c')

		bytes := []byte{0x3A, 0x8F, 'z', 3}
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (character not found)")
		}

		if cpu.GPR(vax.R0) != 0 {
			t.Errorf("R0 = %d, want 0", cpu.GPR(vax.R0))
		}
		
		if cpu.GPR(vax.R1) != 0x2003 {
			t.Errorf("R1 = %#x, want 0x2003 (one past the string)", cpu.GPR(vax.R1))
		}
	})

	t.Run("zero-length string: Z set", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)

		bytes := []byte{0x3A, 0x8F, 'a', 0}
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (zero-length string)")
		}
	})
}

func TestEmulSkpc(t *testing.T) {
	t.Run("finds an unequal byte", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'a', 'a', 'b', 'a')

		bytes := []byte{0x3B, 0x8F, 'a', 4}
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if cpu.PSL().Z() {
			t.Error("Z = true, want false (an unequal byte was found)")
		}

		if cpu.GPR(vax.R0) != 2 {
			t.Errorf("R0 = %d, want 2 (remaining including the unequal byte)", cpu.GPR(vax.R0))
		}

		if cpu.GPR(vax.R1) != 0x2002 {
			t.Errorf("R1 = %#x, want 0x2002", cpu.GPR(vax.R1))
		}
	})

	t.Run("every byte equal", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'a', 'a', 'a')

		bytes := []byte{0x3B, 0x8F, 'a', 3}
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (every byte equal)")
		}

		if cpu.GPR(vax.R0) != 0 {
			t.Errorf("R0 = %d, want 0", cpu.GPR(vax.R0))
		}
	})

	t.Run("zero-length string: Z set", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)

		bytes := []byte{0x3B, 0x8F, 'a', 0}
		bytes = append(bytes, absoluteMode(0x2000)...)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (zero-length string)")
		}
	})
}
