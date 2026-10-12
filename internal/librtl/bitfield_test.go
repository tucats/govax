package librtl

import (
	"strings"
	"testing"
)

// The bit field tests lay their arguments out in the fixture's memory:
// the position longword, the size byte, the source longword (INSV), the
// find-position longword (FFx), and the field, a quadword at fieldAddr
// with room on both sides for negative positions.
const (
	posAddr   = 0x1000
	sizeAddr  = 0x1004
	srcAddr   = 0x1008
	findAddr  = 0x100C
	fieldAddr = 0x1100
)

// fieldCall runs a bit field routine with the position and size given,
// the field's memory set to lo (fieldAddr) and hi (fieldAddr+4).
func fieldCall(t *testing.T, name string, position int32, size byte, lo, hi uint32, argv ...uint32) (uint32, func(uint32) uint32) {
	t.Helper()

	env := fixture(t)
	putLongword(t, env, posAddr, uint32(position))

	if err := env.Memory().StoreByte(env.CPU(), sizeAddr, size); err != nil {
		t.Fatal(err)
	}

	putLongword(t, env, fieldAddr-4, 0)
	putLongword(t, env, fieldAddr, lo)
	putLongword(t, env, fieldAddr+4, hi)

	return call(t, env, name, argv...), func(addr uint32) uint32 { return longword(t, env, addr) }
}

func TestLibExtv(t *testing.T) {
	for _, c := range []struct {
		name     string
		position int32
		size     byte
		lo, hi   uint32
		want     uint32
	}{
		{"LIB$EXTV", 0, 4, 0x0000000F, 0, 0xFFFFFFFF},  // 1111 is -1
		{"LIB$EXTZV", 0, 4, 0x0000000F, 0, 0x0000000F}, // and 15
		{"LIB$EXTV", 4, 4, 0x00000070, 0, 7},           // 0111 is 7
		{"LIB$EXTZV", 28, 8, 0xA0000000, 0x0000000B, 0xBA},
		{"LIB$EXTV", 28, 8, 0xA0000000, 0x0000000B, 0xFFFFFFBA},
		{"LIB$EXTZV", 0, 32, 0x89ABCDEF, 0, 0x89ABCDEF},
		{"LIB$EXTZV", 4, 32, 0x89ABCDEF, 0x01234567, 0x789ABCDE},
		{"LIB$EXTV", 0, 0, 0xFFFFFFFF, 0, 0}, // no bits
		// Negative positions reach the bytes below the base.
		{"LIB$EXTZV", -8, 16, 0x000000AB, 0, 0xAB00},
		{"LIB$EXTZV", -3, 3, 0, 0, 0},
	} {
		got, _ := fieldCall(t, c.name, c.position, c.size, c.lo, c.hi, posAddr, sizeAddr, fieldAddr)
		if got != c.want {
			t.Errorf("%s(%d, %d) of %08X %08X = %#x, want %#x", c.name, c.position, c.size, c.hi, c.lo, got, c.want)
		}
	}
}

func TestLibInsv(t *testing.T) {
	for _, c := range []struct {
		position int32
		size     byte
		src      uint32
		lo, hi   uint32 // the field's memory after the insert
	}{
		// The manual's example (LIB-254): bits 0 to 2 set to 4.
		{0, 3, 4, 0xFFFFFFFC, 0xFFFFFFFF},
		{4, 8, 0x5A, 0xFFFFF5AF, 0xFFFFFFFF},
		{28, 8, 0x00, 0x0FFFFFFF, 0xFFFFFFF0},
		{0, 32, 0x12345678, 0x12345678, 0xFFFFFFFF},
		{0, 0, 0, 0xFFFFFFFF, 0xFFFFFFFF}, // nothing stored
		{3, 4, 0xFFFFFFF0, 0xFFFFFF87, 0xFFFFFFFF},
	} {
		env := fixture(t)
		putLongword(t, env, posAddr, uint32(c.position))

		if err := env.Memory().StoreByte(env.CPU(), sizeAddr, c.size); err != nil {
			t.Fatal(err)
		}

		putLongword(t, env, srcAddr, c.src)
		putLongword(t, env, fieldAddr, 0xFFFFFFFF)
		putLongword(t, env, fieldAddr+4, 0xFFFFFFFF)

		if r0 := call(t, env, "LIB$INSV", srcAddr, posAddr, sizeAddr, fieldAddr); r0 != ssNormal {
			t.Errorf("LIB$INSV = %#x, want SS$_NORMAL", r0)
		}

		if lo, hi := longword(t, env, fieldAddr), longword(t, env, fieldAddr+4); lo != c.lo || hi != c.hi {
			t.Errorf("INSV %#x at (%d, %d): field %08X %08X, want %08X %08X", c.src, c.position, c.size, hi, lo, c.hi, c.lo)
		}
	}
}

func TestLibFfx(t *testing.T) {
	for _, c := range []struct {
		name     string
		position int32
		size     byte
		lo, hi   uint32
		status   uint32
		find     uint32
	}{
		{"LIB$FFS", 0, 32, 0x00000100, 0, ssNormal, 8},
		{"LIB$FFC", 0, 32, 0xFFFFFEFF, 0, ssNormal, 8},
		// The position is relative to the base, not to the field.
		{"LIB$FFS", 4, 8, 0x00000101, 0, ssNormal, 8},
		{"LIB$FFS", 30, 4, 0, 0x00000002, ssNormal, 33},
		// None found: position + size.
		{"LIB$FFS", 4, 4, 0x0000000F, 0, libNotFou, 8},
		{"LIB$FFC", 0, 16, 0x0000FFFF, 0, libNotFou, 16},
		// Size 0: not found, at the position.
		{"LIB$FFS", 5, 0, 0xFFFFFFFF, 0, libNotFou, 5},
		// A negative position finds a bit below the base.
		{"LIB$FFC", -8, 8, 0, 0, ssNormal, 0xFFFFFFF8},
	} {
		got, mem := fieldCall(t, c.name, c.position, c.size, c.lo, c.hi, posAddr, sizeAddr, fieldAddr, findAddr)
		if got != c.status {
			t.Errorf("%s(%d, %d) of %08X %08X = %#x, want %#x", c.name, c.position, c.size, c.hi, c.lo, got, c.status)
		}

		if find := mem(findAddr); find != c.find {
			t.Errorf("%s(%d, %d) of %08X %08X: find-position %#x, want %#x", c.name, c.position, c.size, c.hi, c.lo, find, c.find)
		}
	}
}

// TestBitFieldReservedOperand: a size above 32 signals SS$_ROPRAND for
// every routine. (This fixture has no SYS$SRCHANDLER stub to signal
// through, so the signal fails with an error; the console's
// TestLibrtlBitFieldReservedOperand checks the signal itself.)
func TestBitFieldReservedOperand(t *testing.T) {
	for _, name := range []string{"LIB$EXTV", "LIB$EXTZV", "LIB$INSV", "LIB$FFC", "LIB$FFS"} {
		env := fixture(t)
		putLongword(t, env, posAddr, 0)

		if err := env.Memory().StoreByte(env.CPU(), sizeAddr, 33); err != nil {
			t.Fatal(err)
		}

		argv := []uint32{posAddr, sizeAddr, fieldAddr, findAddr}
		if name == "LIB$INSV" {
			argv = []uint32{srcAddr, posAddr, sizeAddr, fieldAddr}
		}

		fn := shim(t, env, name)
		if _, err := fn(env, argv); err == nil || !strings.Contains(err.Error(), "signal") {
			t.Errorf("%s: err = %v, want a signal", name, err)
		}
	}
}
