package cpu

import (
	"errors"
	"math"
	"testing"
)

// The F/D-floating raw bit patterns below were produced by a standalone C
// test harness linked against the real reference/eVAX/eVAX/Source/CPU/fpu.c
// (`clang -DLINUX86 -I eVAX/Headers probe.c fpu.c`), calling fpu_store/
// fpu_load directly -- not hand-derived -- per docs/PHASE-05.md's design
// notes on why this file's byte-shuffle loops aren't trustworthy to trace by
// eye. Some of these (1.0, -3.5, 3.14159265358979, 123456.789, -0.001) are
// exactly reference/eVAX/AUDIT.md's own C3 verification values.
func TestFpuStoreFloatingKnownValues(t *testing.T) {
	cases := []struct {
		value float64
		raw   uint64 // low 32 bits only checked for size 4
	}{
		{1.0, 0x00004080},
		{-1.0, 0x0000C080},
		{2.0, 0x00004100},
		{0.5, 0x00004000},
		{-3.5, 0x0000C160},
		{3.14159265358979, 0x0FDB4149},
		{123456.789, 0x206548F1},
		{-0.001, 0x126FBB83},
		{0.0, 0x00000000},
		{1.0 / 3.0, 0xAAAB3FAA},
		{1e-10, 0xE6FF2FDB},
	}
	cpu, _ := fixture()
	for _, c := range cases {
		got, err := fpuStore(cpu, 4, c.value)
		if err != nil {
			t.Errorf("fpuStore(4, %v) error: %v", c.value, err)
			continue
		}
		if got != c.raw {
			t.Errorf("fpuStore(4, %v) = %#010x, want %#010x", c.value, got, c.raw)
		}
	}
}

func TestFpuStoreDoubleFloatingKnownValues(t *testing.T) {
	cases := []struct {
		value float64
		raw   uint64
	}{
		{1.0, 0x0000000000004080},
		{-3.5, 0x000000000000C160},
		{3.14159265358979, 0x6888A2210FDA4149},
		{123456.789, 0xB648FDF3206448F1},
		{-0.001, 0x4FE0978D126EBB83},
		{1.0 / 3.0, 0xAAA8AAAAAAAA3FAA},
		{1e-10, 0xEDD8CEBDE6FE2FDB},
	}
	cpu, _ := fixture()
	for _, c := range cases {
		got, err := fpuStore(cpu, 8, c.value)
		if err != nil {
			t.Errorf("fpuStore(8, %v) error: %v", c.value, err)
			continue
		}
		if got != c.raw {
			t.Errorf("fpuStore(8, %v) = %#018x, want %#018x", c.value, got, c.raw)
		}
	}
}

func TestFpuLoadRoundTrip(t *testing.T) {
	cpu, _ := fixture()
	vals := []float64{1.0, -1.0, 2.0, 0.5, -3.5, 100.0, 1000.0, -100000.0, 65536.0, 1e10}
	for _, v := range vals {
		raw, err := fpuStore(cpu, 4, v)
		if err != nil {
			t.Fatalf("fpuStore(4, %v): %v", v, err)
		}
		back, err := fpuLoad(raw, 4)
		if err != nil {
			t.Fatalf("fpuLoad(4, %v raw): %v", v, err)
		}
		if back != v {
			t.Errorf("F round-trip %v -> raw %#x -> %v, want exact", v, raw, back)
		}

		rawD, err := fpuStore(cpu, 8, v)
		if err != nil {
			t.Fatalf("fpuStore(8, %v): %v", v, err)
		}
		backD, err := fpuLoad(rawD, 8)
		if err != nil {
			t.Fatalf("fpuLoad(8, %v raw): %v", v, err)
		}
		if backD != v {
			t.Errorf("D round-trip %v -> raw %#x -> %v, want exact", v, rawD, backD)
		}
	}
}

func TestFpuLoadDoubleFullPrecision(t *testing.T) {
	// F_floating only keeps 23 of the IEEE mantissa's 52 bits, so it's lossy
	// for values needing more precision; D_floating keeps all 52 and must
	// round-trip exactly, unlike F's already-confirmed-lossy 3.1415927...
	cpu, _ := fixture()
	v := 3.14159265358979
	rawD, err := fpuStore(cpu, 8, v)
	if err != nil {
		t.Fatalf("fpuStore(8, %v): %v", v, err)
	}
	back, err := fpuLoad(rawD, 8)
	if err != nil {
		t.Fatalf("fpuLoad: %v", err)
	}
	if back != v {
		t.Errorf("D_floating round-trip = %.17g, want exact %.17g", back, v)
	}

	rawF, err := fpuStore(cpu, 4, v)
	if err != nil {
		t.Fatalf("fpuStore(4, %v): %v", v, err)
	}
	backF, err := fpuLoad(rawF, 4)
	if err != nil {
		t.Fatalf("fpuLoad: %v", err)
	}
	if backF == v {
		t.Error("F_floating round-trip came back exact; expected lossy (only 23 mantissa bits)")
	}
}

func TestFpuStoreOverflowFaults(t *testing.T) {
	cpu, _ := fixture()
	_, err := fpuStore(cpu, 4, 1e60)
	var f *Fault
	if !errors.As(err, &f) {
		t.Fatalf("fpuStore(1e60) err = %v, want *Fault", err)
	}
	if f.Code != ExcArithmetic || len(f.Args) != 1 || f.Args[0] != faultFltOvf {
		t.Errorf("fault = %+v, want {ExcArithmetic, [faultFltOvf]}", f)
	}
}

func TestFpuStoreUnderflowFlushesToZeroWhenFUClear(t *testing.T) {
	// fpu_store's own C implementation is dead-code buggy here (the zeroed
	// local variable is never actually read from) and produces a nonzero
	// raw pattern instead of a true flush-to-zero; this is a confirmed,
	// clear-cut bug fixed in Go -- see docs/PHASE-05.md's design notes and
	// docs/DEVIATIONS.md.
	cpu, _ := fixture()
	psl := cpu.PSL()
	psl.SetFU(false)
	cpu.SetPSL(psl)

	got, err := fpuStore(cpu, 4, 1e-100)
	if err != nil {
		t.Fatalf("fpuStore(1e-100) with FU clear: unexpected error %v", err)
	}
	if got != 0 {
		t.Errorf("fpuStore(1e-100) with FU clear = %#x, want 0 (flush to zero)", got)
	}
}

func TestFpuStoreUnderflowFaultsWhenFUSet(t *testing.T) {
	cpu, _ := fixture()
	psl := cpu.PSL()
	psl.SetFU(true)
	cpu.SetPSL(psl)

	_, err := fpuStore(cpu, 4, 1e-100)
	var f *Fault
	if !errors.As(err, &f) {
		t.Fatalf("fpuStore(1e-100) with FU set err = %v, want *Fault", err)
	}
	if f.Code != ExcArithmetic || len(f.Args) != 1 || f.Args[0] != faultFltUnd {
		t.Errorf("fault = %+v, want {ExcArithmetic, [faultFltUnd]}", f)
	}
}

func TestFpuLoadReservedOperandFault(t *testing.T) {
	// sign=1, exponent=0, fraction nonzero -- the VAX reserved-operand
	// encoding. Constructed and confirmed against the C reference harness
	// (raw 0x34568012, natural form 0x80123456 before word-swapping).
	_, err := fpuLoad(0x34568012, 4)
	var f *Fault
	if !errors.As(err, &f) {
		t.Fatalf("fpuLoad(reserved) err = %v, want *Fault", err)
	}
	if f.Code != ExcReservedOp || len(f.Args) != 0 {
		t.Errorf("fault = %+v, want {ExcReservedOp, []}", f)
	}
}

func TestFpuLoadNegativeZeroIsZeroNotReserved(t *testing.T) {
	// sign=1, exponent=0, fraction=0 -- confirmed against the harness as a
	// valid (non-faulting) zero, not a reserved-operand encoding: the
	// reserved check masks off the sign bit before comparing to zero.
	got, err := fpuLoad(0x00008000, 4)
	if err != nil {
		t.Fatalf("fpuLoad(negative zero): unexpected error %v", err)
	}
	if got != 0 {
		t.Errorf("fpuLoad(negative zero) = %v, want 0", got)
	}
}

func TestFpuLoadZero(t *testing.T) {
	got, err := fpuLoad(0x00000000, 4)
	if err != nil {
		t.Fatalf("fpuLoad(0): unexpected error %v", err)
	}
	if got != 0 {
		t.Errorf("fpuLoad(0) = %v, want 0", got)
	}
}

func TestLoadFloatShortLiteralIsPreDecoded(t *testing.T) {
	// decodeOperand stores a short-literal float operand's value as
	// math.Float64bits directly (already an IEEE double, not VAX F/D
	// bits) -- loadFloat must read it straight through, not via fpuLoad.
	cpu, mem := fixture()
	op := Operand{Kind: OperandImmediate, Value: math.Float64bits(0.5), Size: 4}
	got, err := loadFloat(cpu, mem, op)
	if err != nil {
		t.Fatalf("loadFloat: %v", err)
	}
	if got != 0.5 {
		t.Errorf("loadFloat(short literal 0.5) = %v, want 0.5", got)
	}
}

func TestStoreFloatRegisterRoundTrip(t *testing.T) {
	cpu, mem := fixture()
	op := Operand{Kind: OperandRegister, Reg: 1, Size: 4, Access: AccessWrite}
	if err := storeFloat(cpu, mem, op, 2.0); err != nil {
		t.Fatalf("storeFloat: %v", err)
	}
	got, err := loadFloat(cpu, mem, Operand{Kind: OperandRegister, Reg: 1, Size: 4, Access: AccessRead})
	if err != nil {
		t.Fatalf("loadFloat: %v", err)
	}
	if got != 2.0 {
		t.Errorf("round trip via storeFloat/loadFloat = %v, want 2.0", got)
	}
}
