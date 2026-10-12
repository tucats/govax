package librtl

import (
	"strings"
	"testing"
)

// The integer tests lay their arguments out in the fixture's memory: up
// to three arrays of four longwords and an array-length longword.
const (
	arrayA   = 0x1000
	arrayB   = 0x1010
	arrayR   = 0x1020
	lenAddr  = 0x1030
	quadAddr = 0x1040
)

func TestLibAddxSubx(t *testing.T) {
	for _, c := range []struct {
		name   string
		a, b   []uint32 // least significant first
		length int32    // 0: omitted (the default, 2)
		want   []uint32
		status uint32
	}{
		// The manual's examples (LIB-8, LIB-400), written most
		// significant first there.
		{"LIB$ADDX", []uint32{0x00000001, 0x7FFF407F}, []uint32{0xFFFFFFFF, 0x8000BF80}, 0, []uint32{0, 0}, ssNormal},
		{"LIB$ADDX", []uint32{0x00000001, 0x10000000}, []uint32{0xFFFFFFFF, 0x00000000}, 0, []uint32{0, 0x10000001}, ssNormal},
		{"LIB$SUBX", []uint32{0x00000001, 0x10000000}, []uint32{0xFFFFFFFF, 0x00000000}, 0, []uint32{2, 0x0FFFFFFF}, ssNormal},
		// Overflow: the result is right but for its sign.
		{"LIB$ADDX", []uint32{0xFFFFFFFF, 0x7FFFFFFF}, []uint32{1, 0}, 0, []uint32{0, 0x80000000}, ssIntOvf},
		{"LIB$SUBX", []uint32{0, 0x80000000}, []uint32{1, 0}, 0, []uint32{0xFFFFFFFF, 0x7FFFFFFF}, ssIntOvf},
		// Three longwords, the carry running through both.
		{"LIB$ADDX", []uint32{0xFFFFFFFF, 0xFFFFFFFF, 0}, []uint32{1, 0, 0}, 3, []uint32{0, 0, 1}, ssNormal},
		{"LIB$SUBX", []uint32{0, 0, 1}, []uint32{1, 0, 0}, 3, []uint32{0xFFFFFFFF, 0xFFFFFFFF, 0}, ssNormal},
		// One longword.
		{"LIB$ADDX", []uint32{0x7FFFFFFF}, []uint32{1}, 1, []uint32{0x80000000}, ssIntOvf},
	} {
		env := fixture(t)

		for i := range 4 {
			putLongword(t, env, arrayR+4*uint32(i), 0xDEADBEEF)
		}

		for i := range c.a {
			putLongword(t, env, arrayA+4*uint32(i), c.a[i])
			putLongword(t, env, arrayB+4*uint32(i), c.b[i])
		}

		argv := []uint32{arrayA, arrayB, arrayR}
		if c.length != 0 {
			putLongword(t, env, lenAddr, uint32(c.length))
			argv = append(argv, lenAddr)
		}

		if got := call(t, env, c.name, argv...); got != c.status {
			t.Errorf("%s(%08X, %08X) = %#x, want %#x", c.name, c.a, c.b, got, c.status)
		}

		for i, want := range c.want {
			if got := longword(t, env, arrayR+4*uint32(i)); got != want {
				t.Errorf("%s(%08X, %08X): result[%d] = %08X, want %08X", c.name, c.a, c.b, i, got, want)
			}
		}

		// Nothing is written past the array.
		if got := longword(t, env, arrayR+4*uint32(len(c.want))); got != 0xDEADBEEF {
			t.Errorf("%s: longword past the result = %08X, want it unchanged", c.name, got)
		}
	}
}

// TestLibAddxSameArray: the result may be one of the inputs.
func TestLibAddxSameArray(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, arrayA, 0x80000000)
	putLongword(t, env, arrayA+4, 1)

	if got := call(t, env, "LIB$ADDX", arrayA, arrayA, arrayA); got != ssNormal {
		t.Errorf("LIB$ADDX = %#x, want SS$_NORMAL", got)
	}

	if lo, hi := longword(t, env, arrayA), longword(t, env, arrayA+4); lo != 0 || hi != 3 {
		t.Errorf("doubled: %08X %08X, want 00000003 00000000", hi, lo)
	}
}

// TestLibAddxSubxLength: a negative length is LIB$_INVARG with nothing
// written; a length of 0 writes nothing and succeeds.
func TestLibAddxSubxLength(t *testing.T) {
	for _, c := range []struct {
		length int32
		status uint32
	}{{-1, libInvArg}, {0, ssNormal}} {
		for _, name := range []string{"LIB$ADDX", "LIB$SUBX"} {
			env := fixture(t)
			putLongword(t, env, arrayA, 1)
			putLongword(t, env, arrayB, 1)
			putLongword(t, env, arrayR, 0xDEADBEEF)
			putLongword(t, env, lenAddr, uint32(c.length))

			if got := call(t, env, name, arrayA, arrayB, arrayR, lenAddr); got != c.status {
				t.Errorf("%s length %d = %#x, want %#x", name, c.length, got, c.status)
			}

			if got := longword(t, env, arrayR); got != 0xDEADBEEF {
				t.Errorf("%s length %d: result %08X, want it unchanged", name, c.length, got)
			}
		}
	}
}

func TestLibEmul(t *testing.T) {
	for _, c := range []struct {
		mulr, muld, add uint32
		want            uint64
	}{
		// The manual's example (LIB-137): 2**28 * 2**12.
		{1 << 28, 1 << 12, 0, 0x00000100_00000000},
		{0xFFFFFFFF, 5, 0, 0xFFFFFFFF_FFFFFFFB},          // -1 * 5
		{3, 4, 0xFFFFFFFF, 11},                           // the addend sign-extended
		{0x80000000, 0x80000000, 0, 0x40000000_00000000}, // the largest product
	} {
		env := fixture(t)
		putLongword(t, env, arrayA, c.mulr)
		putLongword(t, env, arrayA+4, c.muld)
		putLongword(t, env, arrayA+8, c.add)

		if got := call(t, env, "LIB$EMUL", arrayA, arrayA+4, arrayA+8, quadAddr); got != ssNormal {
			t.Errorf("LIB$EMUL = %#x, want SS$_NORMAL", got)
		}

		got := uint64(longword(t, env, quadAddr+4))<<32 | uint64(longword(t, env, quadAddr))
		if got != c.want {
			t.Errorf("%#x * %#x + %#x = %016X, want %016X", c.mulr, c.muld, c.add, got, c.want)
		}
	}
}

func TestLibEdiv(t *testing.T) {
	for _, c := range []struct {
		divisor  uint32
		dividend uint64
		quo, rem uint32
		status   uint32
	}{
		// The manual's example (LIB-127): 4,600,387,192 / 4096.
		{4096, 0x00000001_12345678, 1123141, 1656, ssNormal},
		{3, 0xFFFFFFFF_FFFFFFF9, 0xFFFFFFFE, 0xFFFFFFFF, ssNormal}, // -7/3: -2 rem -1
		// The quotient doesn't fit: the dividend's low longword, 0.
		{1, 0x00000001_12345678, 0x12345678, 0, ssIntOvf},
		{0xFFFFFFFF, 0x80000000_00000000, 0, 0, ssIntOvf},
		// Division by zero, likewise.
		{0, 0x00000001_12345678, 0x12345678, 0, ssIntDiv},
	} {
		env := fixture(t)
		putLongword(t, env, arrayA, c.divisor)
		putLongword(t, env, quadAddr, uint32(c.dividend))
		putLongword(t, env, quadAddr+4, uint32(c.dividend>>32))

		if got := call(t, env, "LIB$EDIV", arrayA, quadAddr, arrayR, arrayR+4); got != c.status {
			t.Errorf("%016X / %#x = status %#x, want %#x", c.dividend, c.divisor, got, c.status)
		}

		if quo, rem := longword(t, env, arrayR), longword(t, env, arrayR+4); quo != c.quo || rem != c.rem {
			t.Errorf("%016X / %#x = %#x rem %#x, want %#x rem %#x", c.dividend, c.divisor, quo, rem, c.quo, c.rem)
		}
	}
}

func TestLibAdawi(t *testing.T) {
	for _, c := range []struct {
		add, sum     uint16
		result, sign uint16
		status       uint32
	}{
		{5, 10, 15, 1, ssNormal},
		{0, 0, 0, 0, ssNormal},
		{0xFFFF, 0, 0xFFFF, 0xFFFF, ssNormal},  // -1
		{1, 0x7FFF, 0x8000, 0xFFFF, libIntOvf}, // overflow: truncated
		{0xFFFF, 1, 0, 0, ssNormal},            // a carry isn't an overflow
	} {
		env := fixture(t)
		mem, cpu := env.Memory(), env.CPU()

		for addr, v := range map[uint32]uint16{0x1000: c.add, 0x1002: c.sum, 0x1004: 0x5555, 0x1006: 0x5555} {
			if err := mem.StoreWord(cpu, addr, v); err != nil {
				t.Fatal(err)
			}
		}

		if got := call(t, env, "LIB$ADAWI", 0x1000, 0x1002, 0x1004); got != c.status {
			t.Errorf("%#x + %#x: status %#x, want %#x", c.add, c.sum, got, c.status)
		}

		sum, _ := mem.LoadWord(cpu, 0x1002)
		sign, _ := mem.LoadWord(cpu, 0x1004)
		after, _ := mem.LoadWord(cpu, 0x1006)

		if sum != c.result || sign != c.sign {
			t.Errorf("%#x + %#x: sum %#x, sign %#x; want %#x, %#x", c.add, c.sum, sum, sign, c.result, c.sign)
		}

		if after != 0x5555 {
			t.Errorf("%#x + %#x: the word after the sign is %#x; want it unchanged (a word is written)", c.add, c.sum, after)
		}
	}
}

// TestLibAdawiOddSum: an odd sum address is the instruction's reserved
// operand fault, signaled. (As TestBitFieldReservedOperand: the fixture
// can't signal, so the signal fails with an error.)
func TestLibAdawiOddSum(t *testing.T) {
	env := fixture(t)

	if _, err := shim(t, env, "LIB$ADAWI")(env, []uint32{0x1000, 0x1003, 0x1006}); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("err = %v, want a signal", err)
	}
}
