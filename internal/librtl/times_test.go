package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The time tests' arguments: two input quadwords, a result, and a
// multiplier.
const (
	timeA    = 0x1000
	timeB    = 0x1008
	timeR    = 0x1010
	timeMult = 0x1018
)

// Units of time in ticks, and deltas of them (stored negated).
const (
	second = int64(vmsdef.TicksPerSecond)
	hour   = 3600 * second
	day    = 24 * hour
)

func delta(ticks int64) uint64 { return uint64(-ticks) }

// unchanged is what the tests put in the result first, so they can tell
// a routine left it alone.
const unchanged = 0xEEEEEEEE_EEEEEEEE

// quad reads the quadword at addr.
func quad(t *testing.T, env *corevms.Environment, addr uint32) uint64 {
	t.Helper()

	return uint64(longword(t, env, addr)) | uint64(longword(t, env, addr+4))<<32
}

// absolute is the absolute time text stands for, as $BINTIM reads it.
func absolute(t *testing.T, env *corevms.Environment, text string) uint64 {
	t.Helper()

	v, ok := env.ParseTime(text)
	if !ok || isDelta(v) {
		t.Fatalf("%q is not an absolute time", text)
	}

	return v
}

// timeCall lays out two times, calls name (LIB$ADD_TIMES or
// LIB$SUB_TIMES), and returns its status and result.
func timeCall(t *testing.T, env *corevms.Environment, name string, t1, t2 uint64) (uint32, uint64) {
	t.Helper()

	putQuad(t, env, timeA, t1)
	putQuad(t, env, timeB, t2)
	putQuad(t, env, timeR, unchanged)

	status := call(t, env, name, timeA, timeB, timeR)

	return status, quad(t, env, timeR)
}

func TestLibAddTimes(t *testing.T) {
	env := fixture(t)
	jan13 := absolute(t, env, "13-JAN-1988 09:13:25.14")
	jan14 := absolute(t, env, "14-JAN-1988 10:13:25.14")

	for _, c := range []struct {
		name   string
		t1, t2 uint64
		status uint32
		want   uint64
	}{
		{"two deltas", delta(day), delta(hour), libNormal, delta(day + hour)},
		{"absolute + delta", jan13, delta(day + hour), libNormal, jan14},
		{"delta + absolute", delta(day + hour), jan13, libNormal, jan14},
		{"two absolutes", jan13, jan14, libOneDelTim, unchanged},
		// The base date (zero) is an absolute time.
		{"base date", 0, delta(day), libNormal, uint64(day)},
		{"base dates", 0, 0, libOneDelTim, unchanged},
		// 10,000 days: too long, given or computed.
		{"sum at the limit", delta(9999 * day), delta(day), libIvTime, unchanged},
		{"sum under the limit", delta(9999 * day), delta(day - 1), libNormal, delta(10_000*day - 1)},
		{"delta at the limit", jan13, delta(10_000 * day), libIvTime, unchanged},
		{"absolute overflow", 1<<63 - 1, delta(1), libIvTime, unchanged},
	} {
		status, got := timeCall(t, env, "LIB$ADD_TIMES", c.t1, c.t2)
		if status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}
}

func TestLibSubTimes(t *testing.T) {
	env := fixture(t)
	jan13 := absolute(t, env, "13-JAN-1988 09:13:25.14")
	jan14 := absolute(t, env, "14-JAN-1988 10:13:25.14")

	for _, c := range []struct {
		name   string
		t1, t2 uint64
		status uint32
		want   uint64
	}{
		{"two deltas", delta(day + hour), delta(hour), libNormal, delta(day)},
		{"two absolutes", jan14, jan13, libNormal, delta(day + hour)},
		{"absolute - delta", jan14, delta(day + hour), libNormal, jan13},
		{"delta - absolute", delta(day), jan13, libInvArgOrd, unchanged},
		// time1 the shorter interval or the earlier time.
		{"negative delta", delta(hour), delta(day), libNegTim, unchanged},
		{"negative absolutes", jan13, jan14, libNegTim, unchanged},
		{"before the base date", uint64(hour), delta(day), libNegTim, unchanged},
		// Equal times: the smallest delta, one tick.
		{"equal deltas", delta(day), delta(day), libNormal, delta(1)},
		{"equal absolutes", jan13, jan13, libNormal, delta(1)},
		// An interval of 10,000 days or more isn't a delta.
		{"long interval", jan13, 0, libIvTime, unchanged},
		{"delta at the limit", jan13, delta(10_000 * day), libIvTime, unchanged},
	} {
		status, got := timeCall(t, env, "LIB$SUB_TIMES", c.t1, c.t2)
		if status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}
}

// TestTimesArgumentCount: each arithmetic routine takes exactly its
// arguments, else LIB$_WRONUMARG.
func TestTimesArgumentCount(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name string
		argv []uint32
	}{
		{"LIB$ADD_TIMES", []uint32{timeA, timeB}},
		{"LIB$ADD_TIMES", []uint32{timeA, timeB, timeR, timeR}},
		{"LIB$SUB_TIMES", []uint32{timeA, timeB}},
		{"LIB$MULT_DELTA_TIME", []uint32{timeMult}},
		{"LIB$MULT_DELTA_TIME", []uint32{timeMult, timeA, timeB}},
		{"LIB$MULTF_DELTA_TIME", []uint32{timeMult}},
	} {
		if got := call(t, env, c.name, c.argv...); got != libWroNumArg {
			t.Errorf("%s with %d arguments: %#x, want LIB$_WRONUMARG", c.name, len(c.argv), got)
		}
	}
}

func TestLibMultDeltaTime(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name       string
		multiplier int32
		in         uint64
		status     uint32
		want       uint64
	}{
		{"times 3", 3, delta(hour + 1), libNormal, delta(3*hour + 3)},
		{"negative multiplier", -3, delta(hour), libNormal, delta(3 * hour)},
		{"most negative multiplier", -1 << 31, delta(1), libNormal, delta(1 << 31)},
		{"times 0", 0, delta(hour), libNormal, 0},
		{"up to the limit", 10_000, delta(day - 1), libNormal, delta(10_000 * (day - 1))},
		{"to the limit", 10_000, delta(day), libIvTime, delta(day)},
		{"far past the limit", 1<<31 - 1, delta(9999 * day), libIvTime, delta(9999 * day)},
		{"an absolute time", 2, uint64(hour), libIvTime, uint64(hour)},
		{"the base date", 2, 0, libIvTime, 0},
		{"a delta at the limit", 1, delta(10_000 * day), libIvTime, delta(10_000 * day)},
	} {
		putLongword(t, env, timeMult, uint32(c.multiplier))
		putQuad(t, env, timeA, c.in)

		status := call(t, env, "LIB$MULT_DELTA_TIME", timeMult, timeA)
		if got := quad(t, env, timeA); status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}
}

func TestLibMultfDeltaTime(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name       string
		multiplier float64
		in         uint64
		status     uint32
		want       uint64
	}{
		{"times 1.5", 1.5, delta(hour), libNormal, delta(hour * 3 / 2)},
		{"negative multiplier", -0.5, delta(day), libNormal, delta(day / 2)},
		// Half a tick rounds away from zero; less than half doesn't.
		{"half a tick", 0.5, delta(3), libNormal, delta(2)},
		{"a quarter tick", 0.25, delta(5), libNormal, delta(1)},
		{"times 0", 0, delta(hour), libNormal, 0},
		{"to the limit", 10_000, delta(day), libIvTime, delta(day)},
		{"huge", 1e38, delta(day), libIvTime, delta(day)},
		{"an absolute time", 2, uint64(hour), libIvTime, uint64(hour)},
	} {
		putFloat(t, env, vaxfloat.F, timeMult, c.multiplier)
		putQuad(t, env, timeA, c.in)

		status := call(t, env, "LIB$MULTF_DELTA_TIME", timeMult, timeA)
		if got := quad(t, env, timeA); status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}

	// A reserved multiplier is SS$_ROPRAND, signaled; nothing stored.
	// (This fixture has nothing to signal through, so the signal fails
	// with an error.)
	putLongword(t, env, timeMult, 0x00008000)
	putQuad(t, env, timeA, delta(hour))

	if _, err := shim(t, env, "LIB$MULTF_DELTA_TIME")(env, []uint32{timeMult, timeA}); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("reserved: err = %v, want a signal", err)
	}

	if got := quad(t, env, timeA); got != delta(hour) {
		t.Errorf("reserved: delta-time = %#x, want it unchanged", got)
	}
}

// TestTimesAccessViolation: an input that can't be read is a signaled
// access violation, as for the other routines.
func TestTimesAccessViolation(t *testing.T) {
	env := fixture(t)

	if _, err := shim(t, env, "LIB$ADD_TIMES")(env, []uint32{0x7FFF0000, timeB, timeR}); err == nil {
		t.Error("LIB$ADD_TIMES of an unreadable time: no error, want an access violation")
	}
}
