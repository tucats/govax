package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// identityTable builds a 256-byte identity translation table (entry n == n)
// at addr, for tests that want the table's AND behavior driven purely by
// the string bytes and mask.
func identityTable(t *testing.T, cpu *vax.CPU, mem *vm.Memory, addr uint32) {
	t.Helper()
	for n := 0; n < 256; n++ {
		putBytes(t, cpu, mem, addr+uint32(n), byte(n))
	}
}

func TestEmulScanc(t *testing.T) {
	t.Run("finds a nonzero AND result", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 0, 0, 5, 0)
		identityTable(t, cpu, mem, 0x4000)

		bytes := []byte{0x2A, 4}
		bytes = append(bytes, absoluteMode(0x2000)...) // addr
		bytes = append(bytes, absoluteMode(0x4000)...) // tbladdr
		bytes = append(bytes, 0x8F, 0xFF)              // mask (immediate: 0xFF isn't a short literal)
		stepInstruction(t, e, bytes...)

		if cpu.PSL().Z() {
			t.Error("Z = true, want false (a match was found)")
		}
		if cpu.GPR(vax.R0) != 2 {
			t.Errorf("R0 = %d, want 2 (bytes remaining including the match)", cpu.GPR(vax.R0))
		}
		if cpu.GPR(vax.R1) != 0x2002 {
			t.Errorf("R1 = %#x, want 0x2002 (address of the matching byte)", cpu.GPR(vax.R1))
		}
		if cpu.GPR(vax.R3) != 0x4000 {
			t.Errorf("R3 = %#x, want 0x4000 (table address)", cpu.GPR(vax.R3))
		}
	})

	t.Run("no match: Z set, R0 zero", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 0, 0, 0)
		identityTable(t, cpu, mem, 0x4000)

		bytes := []byte{0x2A, 3}
		bytes = append(bytes, absoluteMode(0x2000)...)
		bytes = append(bytes, absoluteMode(0x4000)...)
		bytes = append(bytes, 0x8F, 0xFF)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (no match)")
		}
		if cpu.GPR(vax.R0) != 0 {
			t.Errorf("R0 = %d, want 0", cpu.GPR(vax.R0))
		}
		if cpu.GPR(vax.R1) != 0x2003 {
			t.Errorf("R1 = %#x, want 0x2003 (one past the string)", cpu.GPR(vax.R1))
		}
	})

	t.Run("zero-length string is treated as exhausted, Z set", func(t *testing.T) {
		// Regression for the Z-polarity bug: the C source's un-fixed
		// default leaves Z clear for a zero-length string, contradicting
		// the manual's explicit note.
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		identityTable(t, cpu, mem, 0x4000)

		bytes := []byte{0x2A, 0}
		bytes = append(bytes, absoluteMode(0x2000)...)
		bytes = append(bytes, absoluteMode(0x4000)...)
		bytes = append(bytes, 0x8F, 0xFF)
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (zero-length string)")
		}
	})
}

func TestEmulSpanc(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	putBytes(t, cpu, mem, 0x2000, 5, 5, 0, 7)
	identityTable(t, cpu, mem, 0x4000)

	bytes := []byte{0x2B, 4}
	bytes = append(bytes, absoluteMode(0x2000)...)
	bytes = append(bytes, absoluteMode(0x4000)...)
	bytes = append(bytes, 0x8F, 0xFF)
	stepInstruction(t, e, bytes...)

	if cpu.PSL().Z() {
		t.Error("Z = true, want false (a zero AND result was found)")
	}
	if cpu.GPR(vax.R0) != 2 {
		t.Errorf("R0 = %d, want 2", cpu.GPR(vax.R0))
	}
	if cpu.GPR(vax.R1) != 0x2002 {
		t.Errorf("R1 = %#x, want 0x2002", cpu.GPR(vax.R1))
	}
}
