package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// Tests for the packed decimal core and MOVP, CMPP3, CMPP4, CVTLP, and
// CVTPL (docs/PHASE-35.md, subtask 10). Expected results are the
// manual's, and where marked VMS's run of the Phase 35 probe
// (testdata/insn35), which settles what the manual leaves UNPREDICTABLE.

const (
	decSrc  = 0x3000
	decSrc2 = 0x3040
	decDst  = 0x3080
)

// putMem stores bytes at addr.
func putMem(t *testing.T, e *Engine, addr uint32, bytes ...byte) {
	t.Helper()

	for i, b := range bytes {
		if err := e.mem.StoreByte(e.cpu, addr+uint32(i), b); err != nil {
			t.Fatal(err)
		}
	}
}

// getMem returns n bytes at addr.
func getMem(t *testing.T, e *Engine, addr uint32, n int) []byte {
	t.Helper()

	out := make([]byte, n)

	for i := range out {
		b, err := e.mem.LoadByte(e.cpu, addr+uint32(i))
		if err != nil {
			t.Fatal(err)
		}

		out[i] = b
	}

	return out
}

// absolute returns the operand specifier bytes for @#addr.
func absolute(addr uint32) []byte {
	return []byte{0x9F, byte(addr), byte(addr >> 8), byte(addr >> 16), byte(addr >> 24)}
}

func decimalEngine() *Engine {
	cpu, mem := fixture()

	return NewEngine(cpu, mem)
}

func TestMOVP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length byte
		src    []byte
		want   []byte
		n, z   bool
	}{
		{"-123", 3, []byte{0x12, 0x3D}, []byte{0x12, 0x3D}, true, false},
		{"-0 becomes +0", 3, []byte{0x00, 0x0D}, []byte{0x00, 0x0C}, false, true},
		{"sign F becomes C", 3, []byte{0x12, 0x3F}, []byte{0x12, 0x3C}, false, false},
		{"sign B becomes D", 3, []byte{0x12, 0x3B}, []byte{0x12, 0x3D}, true, false},
		{"even length's high nibble cleared", 2, []byte{0x51, 0x2C}, []byte{0x01, 0x2C}, false, false},
		// VMS copies an invalid digit nibble unchanged.
		{"invalid digit copied", 3, []byte{0x1A, 0x3C}, []byte{0x1A, 0x3C}, false, false},
		{"length 0", 0, []byte{0x0D}, []byte{0x0C}, false, true},
	} {
		e := decimalEngine()
		putMem(t, e, decSrc, tc.src...)
		setC(e.cpu, true)

		insn := append([]byte{0x34, tc.length}, absolute(decSrc)...)
		insn = append(insn, absolute(decDst)...)

		if err := runFloat(t, e, insn...); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		if got := getMem(t, e, decDst, len(tc.want)); string(got) != string(tc.want) {
			t.Errorf("%s: dst = % x, want % x", tc.name, got, tc.want)
		}

		p := e.cpu.PSL()
		if p.N() != tc.n || p.Z() != tc.z || p.V() || !p.C() {
			t.Errorf("%s: PSL %+v, want N %v Z %v, V clear, C unaffected", tc.name, p, tc.n, tc.z)
		}

		if e.cpu.GPR(vax.R0) != 0 || e.cpu.GPR(vax.R1) != decSrc || e.cpu.GPR(vax.R2) != 0 || e.cpu.GPR(vax.R3) != decDst {
			t.Errorf("%s: R0-R3 = %#x %#x %#x %#x", tc.name, e.cpu.GPR(vax.R0), e.cpu.GPR(vax.R1), e.cpu.GPR(vax.R2), e.cpu.GPR(vax.R3))
		}
	}
}

// TestDecimalLengthOver31 checks a length over 31 is a reserved operand,
// with the registers unchanged.
func TestDecimalLengthOver31(t *testing.T) {
	e := decimalEngine()
	e.cpu.SetGPR(vax.R1, 0x1111)

	insn := append([]byte{0x34, 32}, absolute(decSrc)...)
	insn = append(insn, absolute(decDst)...)

	err := runFloat(t, e, insn...)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcReservedOp {
		t.Errorf("MOVP #32: %v, want a reserved operand fault", err)
	}

	if e.cpu.GPR(vax.R1) != 0x1111 {
		t.Errorf("R1 changed")
	}
}

func TestCMPP(t *testing.T) {
	for _, tc := range []struct {
		name       string
		insn       []byte
		src1, src2 []byte
		n, z       bool
	}{
		{"CMPP3 less", []byte{0x35, 3}, []byte{0x12, 0x2C}, []byte{0x12, 0x3C}, true, false},
		{"CMPP3 -0 equals +0", []byte{0x35, 3}, []byte{0x00, 0x0D}, []byte{0x00, 0x0C}, false, true},
		{"CMPP3 negatives", []byte{0x35, 3}, []byte{0x12, 0x4D}, []byte{0x12, 0x3D}, true, false},
	} {
		e := decimalEngine()
		putMem(t, e, decSrc, tc.src1...)
		putMem(t, e, decSrc2, tc.src2...)

		insn := append(append([]byte{}, tc.insn...), absolute(decSrc)...)
		insn = append(insn, absolute(decSrc2)...)

		if err := runFloat(t, e, insn...); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		if p := e.cpu.PSL(); p.N() != tc.n || p.Z() != tc.z {
			t.Errorf("%s: PSL %+v, want N %v Z %v", tc.name, p, tc.n, tc.z)
		}
	}

	// CMPP4 with different lengths: 7 equals 0000007.
	e := decimalEngine()
	putMem(t, e, decSrc, 0x7C)
	putMem(t, e, decSrc2, 0x00, 0x00, 0x7C)

	insn := append([]byte{0x37, 1}, absolute(decSrc)...)
	insn = append(insn, 5)
	insn = append(insn, absolute(decSrc2)...)

	if err := runFloat(t, e, insn...); err != nil || !e.cpu.PSL().Z() {
		t.Errorf("CMPP4 7 vs 00007: %v, Z %v; want equal", err, e.cpu.PSL().Z())
	}
}

func TestCVTLP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    int32
		length   byte
		want     []byte
		n, z, v  bool
		dv, trap bool
	}{
		{"-123", -123, 4, []byte{0x00, 0x12, 0x3D}, true, false, false, false, false},
		{"-2^31", -2147483648, 10, []byte{0x02, 0x14, 0x74, 0x83, 0x64, 0x8D}, true, false, false, false, false},
		// Overflow keeps the low-order digits and the true sign (VMS).
		{"overflow", -12345, 3, []byte{0x34, 0x5D}, true, false, true, false, false},
		{"overflow, DV", -12345, 3, []byte{0x34, 0x5D}, true, false, true, true, true},
		{"length 0", 1, 0, []byte{0x0C}, false, true, true, false, false},
	} {
		e := decimalEngine()
		e.cpu.SetGPR(vax.R6, uint32(tc.value))

		psl := e.cpu.PSL()
		psl.SetDV(tc.dv)
		e.cpu.SetPSL(psl)

		insn := append([]byte{0xF9, regMode(vax.R6), tc.length}, absolute(decDst)...)
		err := runFloat(t, e, insn...)

		var f *Fault
		if gotTrap := errors.As(err, &f) && f.Code == ExcArithmetic && f.Args[0] == trapDecOvf; gotTrap != tc.trap || (err != nil && !gotTrap) {
			t.Errorf("%s: err %v, want trap %v", tc.name, err, tc.trap)
		}

		if got := getMem(t, e, decDst, len(tc.want)); string(got) != string(tc.want) {
			t.Errorf("%s: dst = % x, want % x", tc.name, got, tc.want)
		}

		if p := e.cpu.PSL(); p.N() != tc.n || p.Z() != tc.z || p.V() != tc.v {
			t.Errorf("%s: PSL %+v, want N %v Z %v V %v", tc.name, p, tc.n, tc.z, tc.v)
		}

		if e.cpu.GPR(vax.R3) != decDst {
			t.Errorf("%s: R3 = %#x, want the destination's address", tc.name, e.cpu.GPR(vax.R3))
		}
	}
}

func TestCVTPL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length byte
		src    []byte
		want   uint32
		v      bool
	}{
		{"-123", 3, []byte{0x12, 0x3D}, 0xFFFFFF85, false},
		{"-0", 3, []byte{0x00, 0x0D}, 0, false},
		{"2^31 overflows", 10, []byte{0x02, 0x14, 0x74, 0x83, 0x64, 0x8C}, 0x80000000, true},
		// VMS uses an invalid nibble's value: 4, 12, 6 is 526.
		{"invalid digit", 3, []byte{0x4C, 0x6C}, 526, false},
	} {
		e := decimalEngine()
		putMem(t, e, decSrc, tc.src...)

		insn := append([]byte{0x36, tc.length}, absolute(decSrc)...)
		insn = append(insn, regMode(vax.R1)) // the destination may be R0-R3

		if err := runFloat(t, e, insn...); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		if got := e.cpu.GPR(vax.R1); got != tc.want {
			t.Errorf("%s: R1 = %#x, want %#x", tc.name, got, tc.want)
		}

		if e.cpu.PSL().V() != tc.v {
			t.Errorf("%s: V %v, want %v", tc.name, e.cpu.PSL().V(), tc.v)
		}
	}
}
