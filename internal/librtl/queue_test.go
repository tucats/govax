package librtl

import (
	"strings"
	"testing"
)

// The queue tests' layout: a header, three entries, the remque-address
// longword, and a retry-count longword.
const (
	qHeader  = 0x2000
	qEntry1  = 0x2100
	qEntry2  = 0x2200
	qEntry3  = 0x2300
	qRemoved = 0x2400
	qRetry   = 0x2408
)

func TestLibInsqRemq(t *testing.T) {
	env := fixture(t)

	for _, step := range []struct {
		name   string
		argv   []uint32
		status uint32
		gone   uint32 // the entry a removal reports
	}{
		{"LIB$REMQHI", []uint32{qHeader, qRemoved}, libQueWasEmp, qHeader},
		{"LIB$REMQTI", []uint32{qHeader, qRemoved}, libQueWasEmp, qHeader},
		{"LIB$INSQHI", []uint32{qEntry1, qHeader}, libOneEntQue, 0},
		{"LIB$INSQHI", []uint32{qEntry2, qHeader}, ssNormal, 0},
		{"LIB$INSQTI", []uint32{qEntry3, qHeader}, ssNormal, 0},
		// The queue: entry 2, entry 1, entry 3.
		{"LIB$REMQTI", []uint32{qHeader, qRemoved}, ssNormal, qEntry3},
		{"LIB$REMQHI", []uint32{qHeader, qRemoved}, ssNormal, qEntry2},
		{"LIB$REMQHI", []uint32{qHeader, qRemoved}, libOneEntQue, qEntry1},
		{"LIB$INSQTI", []uint32{qEntry3, qHeader}, libOneEntQue, 0},
		{"LIB$REMQTI", []uint32{qHeader, qRemoved}, libOneEntQue, qEntry3},
	} {
		if got := call(t, env, step.name, step.argv...); got != step.status {
			t.Errorf("%s = %#x, want %#x", step.name, got, step.status)
		}

		if step.gone != 0 {
			if got := longword(t, env, qRemoved); got != step.gone {
				t.Errorf("%s removed %#x, want %#x", step.name, got, step.gone)
			}
		}
	}

	if f, b := longword(t, env, qHeader), longword(t, env, qHeader+4); f != 0 || b != 0 {
		t.Errorf("header %08X %08X, want zeros (empty)", f, b)
	}
}

// TestLibQueueInterlock: a header with its secondary interlock set fails
// every attempt, LIB$_SECINTFAI, the queue and remque-address unchanged.
func TestLibQueueInterlock(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, qHeader, 1)
	putLongword(t, env, qRemoved, 0xDEADBEEF)
	putLongword(t, env, qRetry, 3)

	for _, c := range []struct {
		name string
		argv []uint32
	}{
		{"LIB$INSQHI", []uint32{qEntry1, qHeader}},
		{"LIB$INSQTI", []uint32{qEntry1, qHeader, qRetry}},
		{"LIB$REMQHI", []uint32{qHeader, qRemoved}},
		{"LIB$REMQTI", []uint32{qHeader, qRemoved, qRetry}},
	} {
		if got := call(t, env, c.name, c.argv...); got != libSecIntFai {
			t.Errorf("%s = %#x, want LIB$_SECINTFAI", c.name, got)
		}
	}

	if got := longword(t, env, qHeader); got != 1 {
		t.Errorf("header %#x, want 1 (unchanged)", got)
	}

	if got := longword(t, env, qRemoved); got != 0xDEADBEEF {
		t.Errorf("remque-address %#x, want it unwritten", got)
	}
}

// TestLibQueueReservedOperand: a misaligned entry or header, an entry
// that is its header, or a remque-address that is the header, signals
// SS$_ROPRAND. (This fixture has nothing to signal through, so the signal
// fails with an error; the console's TestLibrtlQueueReservedOperand
// checks the signal itself.)
func TestLibQueueReservedOperand(t *testing.T) {
	for _, c := range []struct {
		name string
		argv []uint32
	}{
		{"LIB$INSQHI", []uint32{qEntry1 + 4, qHeader}},
		{"LIB$INSQTI", []uint32{qHeader, qHeader}},
		{"LIB$REMQHI", []uint32{qHeader + 2, qRemoved}},
		{"LIB$REMQTI", []uint32{qHeader, qHeader}},
	} {
		env := fixture(t)

		if _, err := shim(t, env, c.name)(env, c.argv); err == nil || !strings.Contains(err.Error(), "signal") {
			t.Errorf("%s: err = %v, want a signal", c.name, err)
		}

		if f, b := longword(t, env, qHeader), longword(t, env, qHeader+4); f != 0 || b != 0 {
			t.Errorf("%s: header %08X %08X, want it unchanged", c.name, f, b)
		}
	}
}

func TestLibBbssiBbcci(t *testing.T) {
	env := fixture(t)
	const bits = 0x3000 // a 128-bit array, as the manual's examples use

	for i := range uint32(4) {
		putLongword(t, env, bits+4*i, 0)
	}

	for _, c := range []struct {
		name     string
		position int32
		want     uint32
	}{
		{"LIB$BBSSI", 42, 0}, // the manual's example: bit 42 was clear
		{"LIB$BBSSI", 42, 1},
		{"LIB$BBCCI", 42, 1},
		{"LIB$BBCCI", 42, 0},
		{"LIB$BBSSI", 104, 0},
		{"LIB$BBSSI", -1, 0}, // bit 7 of the byte below
	} {
		putLongword(t, env, 0x1000, uint32(c.position))

		if got := call(t, env, c.name, 0x1000, bits); got != c.want {
			t.Errorf("%s(%d) = %d, want %d", c.name, c.position, got, c.want)
		}
	}

	if got := longword(t, env, bits+12); got != 1<<8 {
		t.Errorf("longword 3 = %08X, want bit 104 set (00000100)", got)
	}

	if got := longword(t, env, bits+4); got != 0 {
		t.Errorf("longword 1 = %08X, want bit 42 clear again", got)
	}

	if got := longword(t, env, bits-4); got != 0x80000000 {
		t.Errorf("longword below = %08X, want bit -1 set (80000000)", got)
	}
}
