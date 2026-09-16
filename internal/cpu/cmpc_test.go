package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestEmulCmpc3(t *testing.T) {
	t.Run("equal strings", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'A', 'B', 'C')
		putBytes(t, cpu, mem, 0x3000, 'A', 'B', 'C')

		bytes := []byte{0x29, 3}
		bytes = append(bytes, absoluteMode(0x2000)...)
		bytes = append(bytes, absoluteMode(0x3000)...)
		stepInstruction(t, e, bytes...)

		got := cpu.PSL()
		if got.N() || !got.Z() || got.V() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=1 V=0 C=0", got)
		}
		
		if cpu.GPR(vax.R0) != 0 {
			t.Errorf("R0 = %d, want 0 (strings equal)", cpu.GPR(vax.R0))
		}

		if cpu.GPR(vax.R1) != 0x2003 || cpu.GPR(vax.R3) != 0x3003 {
			t.Errorf("R1/R3 = %#x/%#x, want one past each string (0x2003/0x3003)", cpu.GPR(vax.R1), cpu.GPR(vax.R3))
		}

		if cpu.GPR(vax.R2) != cpu.GPR(vax.R0) {
			t.Errorf("R2 = %d, want equal to R0 (%d)", cpu.GPR(vax.R2), cpu.GPR(vax.R0))
		}
	})

	t.Run("stops at first inequality", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'A', 'X', 'C')
		putBytes(t, cpu, mem, 0x3000, 'A', 'B', 'C')

		bytes := []byte{0x29, 3}
		bytes = append(bytes, absoluteMode(0x2000)...)
		bytes = append(bytes, absoluteMode(0x3000)...)
		stepInstruction(t, e, bytes...)

		if cpu.GPR(vax.R0) != 2 {
			t.Errorf("R0 = %d, want 2 (bytes remaining including the mismatch)", cpu.GPR(vax.R0))
		}

		if cpu.GPR(vax.R1) != 0x2001 || cpu.GPR(vax.R3) != 0x3001 {
			t.Errorf("R1/R3 = %#x/%#x, want the mismatching bytes' addresses (0x2001/0x3001)", cpu.GPR(vax.R1), cpu.GPR(vax.R3))
		}

		got := cpu.PSL()
		// 'X' (0x58) LSS 'B' (0x42) is false; LSSU also false.
		if got.N() || got.Z() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=0 C=0 ('X' > 'B')", got)
		}
	})

	t.Run("zero length compares equal", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)

		bytes := []byte{0x29, 0}
		bytes = append(bytes, absoluteMode(0x2000)...)
		bytes = append(bytes, absoluteMode(0x3000)...)
		stepInstruction(t, e, bytes...)

		got := cpu.PSL()
		if got.N() || !got.Z() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=1 C=0", got)
		}
	})
}

func TestEmulCmpc5(t *testing.T) {
	t.Run("equal after fill-extending the shorter string", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'A', 'B')
		putBytes(t, cpu, mem, 0x3000, 'A', 'B', '.', '.')

		bytes := []byte{0x2D, 2}
		bytes = append(bytes, absoluteMode(0x2000)...) // src1len=2, src1addr
		bytes = append(bytes, '.')                     // fill
		bytes = append(bytes, 4)
		bytes = append(bytes, absoluteMode(0x3000)...) // src2len=4, src2addr
		stepInstruction(t, e, bytes...)

		got := cpu.PSL()
		if got.N() || !got.Z() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=1 C=0 (string1 padded with fill equals string2)", got)
		}

		if cpu.GPR(vax.R0) != 0 || cpu.GPR(vax.R2) != 0 {
			t.Errorf("R0/R2 = %d/%d, want both 0 (fully consumed)", cpu.GPR(vax.R0), cpu.GPR(vax.R2))
		}
	})

	t.Run("one-sided zero length is still fill-padded", func(t *testing.T) {
		// Regression for the outer-gate fix: emul_cmpc5.c's literal
		// `if (tmp1 > 0 && tmp3 > 0)` would skip fill-padding entirely here
		// (src1len == 0), contradicting the manual's "shorter string is
		// conceptually extended" description.
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x3000, '.', '.', '.')

		bytes := []byte{0x2D, 0}
		bytes = append(bytes, absoluteMode(0x2000)...) // src1len=0
		bytes = append(bytes, '.')
		bytes = append(bytes, 3)
		bytes = append(bytes, absoluteMode(0x3000)...) // src2len=3, all fill
		stepInstruction(t, e, bytes...)

		if !cpu.PSL().Z() {
			t.Error("Z = false, want true (empty string1 padded with fill equals string2)")
		}

		if cpu.GPR(vax.R2) != 0 {
			t.Errorf("R2 = %d, want 0 (string2 fully consumed against fill)", cpu.GPR(vax.R2))
		}
	})

	t.Run("inequality found with bytes remaining on both sides is not overwritten by fill padding", func(t *testing.T) {
		// Regression for docs/DEVIATIONS.md's CMPC5 finding, confirmed
		// against the manual's own CMPC entry ("comparison proceeds until
		// inequality is detected or all the bytes of the strings have
		// been examined[;] condition codes are affected by the result of
		// the last byte comparison") and fixed in Phase 12: once the main
		// loop finds a mismatch with bytes still remaining on both sides,
		// the two fill-padding loops must not also run and clobber the
		// condition codes/R0-R3 with a comparison against the fill byte.
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)
		putBytes(t, cpu, mem, 0x2000, 'A', 'Z')
		putBytes(t, cpu, mem, 0x3000, 'A', 'B')

		bytes := []byte{0x2D, 2}
		bytes = append(bytes, absoluteMode(0x2000)...) // src1len=2, src1addr
		bytes = append(bytes, '.')                     // fill
		bytes = append(bytes, 2)
		bytes = append(bytes, absoluteMode(0x3000)...) // src2len=2, src2addr
		stepInstruction(t, e, bytes...)

		// 'Z' (0x5A) vs 'B' (0x42): positive difference, no borrow.
		got := cpu.PSL()
		if got.N() || got.Z() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=0 C=0 ('Z' vs 'B', not 'Z' vs fill)", got)
		}

		if cpu.GPR(vax.R0) != 1 || cpu.GPR(vax.R2) != 1 {
			t.Errorf("R0/R2 = %d/%d, want both 1 (the mismatching byte still counted as remaining)", cpu.GPR(vax.R0), cpu.GPR(vax.R2))
		}

		if cpu.GPR(vax.R1) != 0x2001 || cpu.GPR(vax.R3) != 0x3001 {
			t.Errorf("R1/R3 = %#x/%#x, want 0x2001/0x3001 (pointing at the mismatching bytes)", cpu.GPR(vax.R1), cpu.GPR(vax.R3))
		}
	})

	t.Run("both zero length compare equal", func(t *testing.T) {
		cpu, mem := fixture()
		e := NewEngine(cpu, mem)

		bytes := []byte{0x2D, 0}
		bytes = append(bytes, absoluteMode(0x2000)...)
		bytes = append(bytes, '.')
		bytes = append(bytes, 0)
		bytes = append(bytes, absoluteMode(0x3000)...)
		stepInstruction(t, e, bytes...)

		got := cpu.PSL()
		if got.N() || !got.Z() || got.V() || got.C() {
			t.Errorf("PSL = %+v, want N=0 Z=1 V=0 C=0", got)
		}
	})
}
