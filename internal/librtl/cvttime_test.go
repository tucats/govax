package librtl

import (
	"math/big"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The conversion tests' arguments: the operation code, a longword or
// floating result or input, and a quadword time (timeA, from
// times_test.go). cvtVector is LIB$CVT_VECTIM's seven words.
const (
	cvtOperation = 0x1100
	cvtLong      = 0x1104
	cvtVector    = 0x1110
)

const minute = 60 * second

// cvtFrom calls LIB$CVT_FROM_INTERNAL_TIME with operation op on the time
// v, returning its status and result.
func cvtFrom(t *testing.T, env *corevms.Environment, op uint32, v uint64) (uint32, uint32) {
	t.Helper()

	putLongword(t, env, cvtOperation, op)
	putLongword(t, env, cvtLong, 0xEEEEEEEE)
	putQuad(t, env, timeA, v)

	status := call(t, env, "LIB$CVT_FROM_INTERNAL_TIME", cvtOperation, cvtLong, timeA)

	return status, longword(t, env, cvtLong)
}

// TestCvtFromInternalTimeAbsolute: every absolute operation on one time,
// and the ends of the year-long ranges on the last moment of a leap year.
func TestCvtFromInternalTimeAbsolute(t *testing.T) {
	env := fixture(t)

	// A Wednesday; 09:13:25 is second 33,205 of the day.
	jan13 := absolute(t, env, "13-JAN-1988 09:13:25.14")
	dec31 := absolute(t, env, "31-DEC-1988 23:59:59.99")

	for _, c := range []struct {
		name string
		v    uint64
		want uint32
	}{
		{"MONTH_OF_YEAR", jan13, 1},
		{"DAY_OF_YEAR", jan13, 13},
		{"HOUR_OF_YEAR", jan13, 12*24 + 9 + 1},
		{"MINUTE_OF_YEAR", jan13, (12*24+9)*60 + 13 + 1},
		{"SECOND_OF_YEAR", jan13, 12*86400 + 33205 + 1},
		{"DAY_OF_MONTH", jan13, 13},
		{"HOUR_OF_MONTH", jan13, 12*24 + 9 + 1},
		{"MINUTE_OF_MONTH", jan13, (12*24+9)*60 + 13 + 1},
		{"SECOND_OF_MONTH", jan13, 12*86400 + 33205 + 1},
		{"DAY_OF_WEEK", jan13, 3},
		{"HOUR_OF_WEEK", jan13, 2*24 + 9 + 1},
		{"MINUTE_OF_WEEK", jan13, (2*24+9)*60 + 13 + 1},
		{"SECOND_OF_WEEK", jan13, 2*86400 + 33205 + 1},
		{"HOUR_OF_DAY", jan13, 9},
		{"MINUTE_OF_DAY", jan13, 9*60 + 13},
		{"SECOND_OF_DAY", jan13, 33205},
		{"MINUTE_OF_HOUR", jan13, 13},
		{"SECOND_OF_HOUR", jan13, 13*60 + 25},
		{"SECOND_OF_MINUTE", jan13, 25},
		// The manual's LIB$DAY example: 4-Nov-1986 is day 46,738.
		{"JULIAN_DATE", jan13, 46738 + 365 + 30 + 31 + 9},
		// The 5.0 manual's upper limits.
		{"DAY_OF_YEAR", dec31, 366},
		{"HOUR_OF_YEAR", dec31, 8784},
		{"MINUTE_OF_YEAR", dec31, 527_040},
		{"SECOND_OF_YEAR", dec31, 31_622_400},
		{"DAY_OF_WEEK", dec31, 6},
		// The base date was a Wednesday, day 0.
		{"DAY_OF_WEEK", 0, 3},
		{"JULIAN_DATE", 0, 0},
		{"DAY_OF_YEAR", 0, 321},
	} {
		op := vmsdef.LibrarySymbols["LIB$K_"+c.name]
		if status, got := cvtFrom(t, env, op, c.v); status != libNormal || got != c.want {
			t.Errorf("%s of %#x: %#x, %d; want LIB$_NORMAL, %d", c.name, c.v, status, got, c.want)
		}
	}
}

func TestCvtFromInternalTimeDelta(t *testing.T) {
	env := fixture(t)
	d := delta(10*day + 5*hour + 7*minute + 3*second + 1)

	for _, c := range []struct {
		name string
		want uint32
	}{
		{"DELTA_WEEKS", 1},
		{"DELTA_DAYS", 10},
		{"DELTA_HOURS", 245},
		{"DELTA_MINUTES", 245*60 + 7},
		{"DELTA_SECONDS", (245*60+7)*60 + 3},
	} {
		op := vmsdef.LibrarySymbols["LIB$K_"+c.name]
		if status, got := cvtFrom(t, env, op, d); status != libNormal || got != c.want {
			t.Errorf("%s: %#x, %d; want LIB$_NORMAL, %d", c.name, status, got, c.want)
		}
	}
}

func TestCvtFromInternalTimeErrors(t *testing.T) {
	env := fixture(t)
	jan13 := absolute(t, env, "13-JAN-1988 09:13:25.14")

	for _, c := range []struct {
		name   string
		op     uint32
		v      uint64
		status uint32
	}{
		{"absolute operation, delta", 1, delta(day), libAbsTimReq},
		{"delta operation, absolute", 22, jan13, libDelTimReq},
		{"delta operation, base date", 22, 0, libDelTimReq},
		{"delta at the limit", 22, delta(10_000 * day), libIvTime},
		{"operation 0", 0, jan13, libInvOper},
		{"a floating operation", 27, delta(day), libInvOper},
		{"operation 31", 31, jan13, libInvOper},
	} {
		if status, got := cvtFrom(t, env, c.op, c.v); status != c.status || got != 0xEEEEEEEE {
			t.Errorf("%s: %#x, %#x; want %#x, unchanged", c.name, status, got, c.status)
		}
	}
}

// TestCvtFromInternalTimeCurrent: with no input-time, or 0 by value, the
// conversion is of the current time, so a delta operation is DELTIMREQ.
func TestCvtFromInternalTimeCurrent(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, "13-JAN-1988 09:13:25.14")
	env.Clock = func() uint64 { return now }

	putLongword(t, env, cvtOperation, vmsdef.LibrarySymbols["LIB$K_DAY_OF_MONTH"])

	for _, argv := range [][]uint32{{cvtOperation, cvtLong}, {cvtOperation, cvtLong, 0}} {
		putLongword(t, env, cvtLong, 0)

		if status := call(t, env, "LIB$CVT_FROM_INTERNAL_TIME", argv...); status != libNormal || longword(t, env, cvtLong) != 13 {
			t.Errorf("%d arguments: %#x, %d; want LIB$_NORMAL, 13", len(argv), status, longword(t, env, cvtLong))
		}
	}

	putLongword(t, env, cvtOperation, vmsdef.LibrarySymbols["LIB$K_DELTA_DAYS"])

	if status := call(t, env, "LIB$CVT_FROM_INTERNAL_TIME", cvtOperation, cvtLong); status != libDelTimReq {
		t.Errorf("DELTA_DAYS of now: %#x, want LIB$_DELTIMREQ", status)
	}
}

// floatAt reads the F_floating value at addr.
func floatAt(t *testing.T, env *corevms.Environment, addr uint32) vaxfloat.Value {
	t.Helper()

	v, err := loadFloat(env, vaxfloat.F, addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func TestCvtfFromInternalTime(t *testing.T) {
	env := fixture(t)

	seventh, _ := vaxfloat.Round(vaxfloat.F, vaxfloat.FromRat(big.NewRat(1, 7)))

	for _, c := range []struct {
		name   string
		op     string
		v      uint64
		status uint32
		want   vaxfloat.Value
	}{
		{"a day and a half", "DELTA_DAYS_F", delta(36 * hour), libNormal, vaxfloat.FromFloat64(1.5)},
		{"a day in weeks", "DELTA_WEEKS_F", delta(day), libNormal, seventh},
		{"90 minutes in hours", "DELTA_HOURS_F", delta(90 * minute), libNormal, vaxfloat.FromFloat64(1.5)},
		{"30 seconds in minutes", "DELTA_MINUTES_F", delta(30 * second), libNormal, vaxfloat.FromFloat64(0.5)},
		{"a quarter second", "DELTA_SECONDS_F", delta(second / 4), libNormal, vaxfloat.FromFloat64(0.25)},
		{"an absolute time", "DELTA_DAYS_F", uint64(day), libDelTimReq, vaxfloat.Value{}},
		{"at the limit", "DELTA_DAYS_F", delta(10_000 * day), libIvTime, vaxfloat.Value{}},
		{"a whole-unit operation", "DELTA_DAYS", delta(day), libInvOper, vaxfloat.Value{}},
	} {
		putLongword(t, env, cvtOperation, vmsdef.LibrarySymbols["LIB$K_"+c.op])
		putLongword(t, env, cvtLong, 0)
		putQuad(t, env, timeA, c.v)

		status := call(t, env, "LIB$CVTF_FROM_INTERNAL_TIME", cvtOperation, cvtLong, timeA)
		if got := floatAt(t, env, cvtLong); status != c.status || got.Cmp(c.want) != 0 {
			t.Errorf("%s: %#x, %v; want %#x, %v", c.name, status, got, c.status, c.want)
		}
	}
}

func TestCvtToInternalTime(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name   string
		op     string
		n      int32
		status uint32
		want   uint64
	}{
		{"5 hours", "DELTA_HOURS", 5, libNormal, delta(5 * hour)},
		{"2 weeks", "DELTA_WEEKS", 2, libNormal, delta(14 * day)},
		{"90 minutes", "DELTA_MINUTES", 90, libNormal, delta(90 * minute)},
		{"a second", "DELTA_SECONDS", 1, libNormal, delta(second)},
		{"zero", "DELTA_DAYS", 0, libNormal, 0},
		{"negative", "DELTA_DAYS", -1, libIvTime, unchanged},
		{"9999 days", "DELTA_DAYS", 9999, libNormal, delta(9999 * day)},
		{"10,000 days", "DELTA_DAYS", 10_000, libIvTime, unchanged},
		{"1428 weeks", "DELTA_WEEKS", 1428, libNormal, delta(1428 * 7 * day)},
		{"1429 weeks", "DELTA_WEEKS", 1429, libIvTime, unchanged},
		{"the most seconds", "DELTA_SECONDS", 1<<31 - 1, libIvTime, unchanged},
		{"a floating operation", "DELTA_DAYS_F", 1, libInvOper, unchanged},
		{"an absolute operation", "DAY_OF_YEAR", 1, libInvOper, unchanged},
	} {
		putLongword(t, env, cvtOperation, vmsdef.LibrarySymbols["LIB$K_"+c.op])
		putLongword(t, env, cvtLong, uint32(c.n))
		putQuad(t, env, timeR, unchanged)

		status := call(t, env, "LIB$CVT_TO_INTERNAL_TIME", cvtOperation, cvtLong, timeR)
		if got := quad(t, env, timeR); status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}
}

func TestCvtfToInternalTime(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name   string
		op     string
		x      float64
		status uint32
		want   uint64
	}{
		{"a day and a half", "DELTA_DAYS_F", 1.5, libNormal, delta(36 * hour)},
		{"a quarter second", "DELTA_SECONDS_F", 0.25, libNormal, delta(second / 4)},
		{"half a week", "DELTA_WEEKS_F", 0.5, libNormal, delta(7 * day / 2)},
		{"zero", "DELTA_HOURS_F", 0, libNormal, 0},
		// 2^-24 seconds is 0.596 ticks, which rounds to 1.
		{"under a tick", "DELTA_SECONDS_F", 1.0 / (1 << 24), libNormal, delta(1)},
		{"negative", "DELTA_DAYS_F", -1, libIvTime, unchanged},
		{"10,000 days", "DELTA_DAYS_F", 10_000, libIvTime, unchanged},
		{"huge", "DELTA_SECONDS_F", 1e38, libIvTime, unchanged},
		{"a whole-unit operation", "DELTA_DAYS", 1, libInvOper, unchanged},
	} {
		putLongword(t, env, cvtOperation, vmsdef.LibrarySymbols["LIB$K_"+c.op])
		putFloat(t, env, vaxfloat.F, cvtLong, c.x)
		putQuad(t, env, timeR, unchanged)

		status := call(t, env, "LIB$CVTF_TO_INTERNAL_TIME", cvtOperation, cvtLong, timeR)
		if got := quad(t, env, timeR); status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}

	// A reserved operand is signaled (this fixture can't deliver it).
	putLongword(t, env, cvtOperation, vmsdef.LibrarySymbols["LIB$K_DELTA_DAYS_F"])
	putLongword(t, env, cvtLong, 0x00008000)

	if _, err := shim(t, env, "LIB$CVTF_TO_INTERNAL_TIME")(env, []uint32{cvtOperation, cvtLong, timeR}); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("reserved: err = %v, want a signal", err)
	}
}

func TestCvtVectim(t *testing.T) {
	env := fixture(t)
	jan13 := absolute(t, env, "13-JAN-1988 09:13:25.14")

	for _, c := range []struct {
		name   string
		fields [7]uint16
		status uint32
		want   uint64
	}{
		{"absolute", [7]uint16{1988, 1, 13, 9, 13, 25, 14}, libNormal, jan13},
		{"delta", [7]uint16{0, 0, 5, 3, 18, 32, 7}, libNormal, delta(5*day + 3*hour + 18*minute + 32*second + 7*second/100)},
		{"the base date", [7]uint16{1858, 11, 17, 0, 0, 0, 0}, libNormal, 0},
		{"29 February 1988", [7]uint16{1988, 2, 29, 0, 0, 0, 0}, libNormal, absolute(t, env, "29-FEB-1988 00:00:00.00")},
		{"29 February 1987", [7]uint16{1987, 2, 29, 0, 0, 0, 0}, libIvTime, unchanged},
		{"month 13", [7]uint16{1988, 13, 1, 0, 0, 0, 0}, libIvTime, unchanged},
		{"hour 24", [7]uint16{1988, 1, 1, 24, 0, 0, 0}, libIvTime, unchanged},
		{"year 0, month 1", [7]uint16{0, 1, 1, 0, 0, 0, 0}, libIvTime, unchanged},
		{"before the base date", [7]uint16{1858, 11, 16, 0, 0, 0, 0}, libIvTime, unchanged},
		{"10,000 days", [7]uint16{0, 0, 10_000, 0, 0, 0, 0}, libIvTime, unchanged},
	} {
		for i, f := range c.fields {
			putWord(t, env, cvtVector+uint32(2*i), f)
		}

		putQuad(t, env, timeR, unchanged)

		status := call(t, env, "LIB$CVT_VECTIM", cvtVector, timeR)
		if got := quad(t, env, timeR); status != c.status || got != c.want {
			t.Errorf("%s: %#x, %#x; want %#x, %#x", c.name, status, got, c.status, c.want)
		}
	}
}

// TestCvtTimeArgumentCount: each conversion takes its arguments, else
// LIB$_WRONUMARG.
func TestCvtTimeArgumentCount(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name string
		argv []uint32
	}{
		{"LIB$CVT_FROM_INTERNAL_TIME", []uint32{cvtOperation}},
		{"LIB$CVT_FROM_INTERNAL_TIME", []uint32{cvtOperation, cvtLong, timeA, timeA}},
		{"LIB$CVTF_FROM_INTERNAL_TIME", []uint32{cvtOperation, cvtLong}},
		{"LIB$CVT_TO_INTERNAL_TIME", []uint32{cvtOperation, cvtLong}},
		{"LIB$CVTF_TO_INTERNAL_TIME", []uint32{cvtOperation, cvtLong, timeR, timeR}},
		{"LIB$CVT_VECTIM", []uint32{cvtVector}},
	} {
		if got := call(t, env, c.name, c.argv...); got != libWroNumArg {
			t.Errorf("%s with %d arguments: %#x, want LIB$_WRONUMARG", c.name, len(c.argv), got)
		}
	}
}
