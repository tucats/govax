package librtl

import (
	"testing"
)

// The simple date tests' arguments: an output descriptor and its text, a
// quadword time, and two longword results.
const (
	daysOut     = 0x1000
	daysOutText = 0x1100
	daysTime    = 0x1200
	daysNumber  = 0x1208
	daysHundred = 0x120C
)

// TestLibDateTime: the current time in $ASCTIM's text, by the output's
// class: fixed (padded, or cut with LIB$_STRTRU), dynamic, and varying.
func TestLibDateTime(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, "9-OCT-1988 10:15:43.21")
	env.Clock = func() uint64 { return now }

	const want = " 9-OCT-1988 10:15:43.21"

	putDesc(t, env, daysOut, 25, dscClassS, daysOutText)

	if got := call(t, env, "LIB$DATE_TIME", daysOut); got != ssNormal || descText(t, env, daysOut) != want+"  " {
		t.Errorf("fixed 25: %#x, %q", got, descText(t, env, daysOut))
	}

	putDesc(t, env, daysOut, 11, dscClassS, daysOutText)

	if got := call(t, env, "LIB$DATE_TIME", daysOut); got != libStrTru || descText(t, env, daysOut) != want[:11] {
		t.Errorf("fixed 11: %#x, %q; want LIB$_STRTRU, the date", got, descText(t, env, daysOut))
	}

	putDesc(t, env, daysOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$DATE_TIME", daysOut); got != ssNormal || descText(t, env, daysOut) != want {
		t.Errorf("dynamic: %#x, %q", got, descText(t, env, daysOut))
	}

	// A varying string: MAXSTRLEN in the descriptor, CURLEN the word its
	// pointer addresses, the text after it.
	putDesc(t, env, daysOut, 40, dscClassVS, daysOutText)

	if got := call(t, env, "LIB$DATE_TIME", daysOut); got != ssNormal || word(t, env, daysOutText) != 23 || bytesAt(t, env, daysOutText+2, 23) != want {
		t.Errorf("varying: %#x, length %d", got, word(t, env, daysOutText))
	}
}

func TestLibDay(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name       string
		v          uint64
		days, time int32
	}{
		// The manual's example: 4-Nov-1986 is day 46,738.
		{"4-Nov-1986", absolute(t, env, "4-NOV-1986 00:00:00.00"), 46738, 0},
		{"13-Jan-1988", absolute(t, env, "13-JAN-1988 09:13:25.14"), 46738 + 365 + 30 + 31 + 9, 3320514},
		// The address of a zero quadword is the base date.
		{"zero", 0, 0, 0},
		// A delta's count is truncated toward zero, its day-time
		// positive (VMS V7.1).
		{"delta", delta(2*day + hour + 5*second/2), -2, 360000 + 250},
		// The last time before the overflow, 2^31 x 10^9 - 1 ticks.
		{"limit", uint64(1<<31)*1_000_000_000 - 1, 2485513, 4159999},
	} {
		putQuad(t, env, daysTime, c.v)
		putLongword(t, env, daysNumber, 0xEEEEEEEE)
		putLongword(t, env, daysHundred, 0xEEEEEEEE)

		got := call(t, env, "LIB$DAY", daysNumber, daysTime, daysHundred)
		days, hundredths := int32(longword(t, env, daysNumber)), int32(longword(t, env, daysHundred))
		if got != ssNormal || days != c.days || hundredths != c.time {
			t.Errorf("%s: %#x, %d, %d; want SS$_NORMAL, %d, %d", c.name, got, days, hundredths, c.days, c.time)
		}
	}

	// From 25-Dec-8663 on, and for a delta as long, SS$_INTOVF, with
	// nothing stored.
	for _, v := range []uint64{uint64(1<<31) * 1_000_000_000, delta(int64(1<<31)*1_000_000_000 + 1_000_000_000)} {
		putQuad(t, env, daysTime, v)
		putLongword(t, env, daysNumber, 0xEEEEEEEE)

		if got := call(t, env, "LIB$DAY", daysNumber, daysTime); got != ssIntOvf || longword(t, env, daysNumber) != 0xEEEEEEEE {
			t.Errorf("%#x: %#x, %#x; want SS$_INTOVF, unchanged", v, got, longword(t, env, daysNumber))
		}
	}
}

// TestLibDayCurrent: with no time, or 0 by value, LIB$DAY counts the
// current time, and day-time is optional.
func TestLibDayCurrent(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, "4-NOV-1986 12:00:00.00")
	env.Clock = func() uint64 { return now }

	for _, argv := range [][]uint32{{daysNumber}, {daysNumber, 0}, {daysNumber, 0, daysHundred}} {
		putLongword(t, env, daysNumber, 0)
		putLongword(t, env, daysHundred, 0xEEEEEEEE)

		wantTime := uint32(0xEEEEEEEE)
		if len(argv) == 3 {
			wantTime = 4320000
		}

		if got := call(t, env, "LIB$DAY", argv...); got != ssNormal || longword(t, env, daysNumber) != 46738 || longword(t, env, daysHundred) != wantTime {
			t.Errorf("%d arguments: %#x, %d, %#x", len(argv), got, longword(t, env, daysNumber), longword(t, env, daysHundred))
		}
	}
}

func TestLibDayOfWeek(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, "13-JAN-1988 09:13:25.14") // a Wednesday
	env.Clock = func() uint64 { return now }

	for _, c := range []struct {
		name string
		v    uint64
		want uint32
	}{
		{"Wednesday", now, 3},
		{"Sunday", absolute(t, env, "17-JAN-1988 23:59:59.99"), 7},
		{"Monday", absolute(t, env, "18-JAN-1988 00:00:00.00"), 1},
		// 17-Nov-1858 was a Wednesday.
		{"zero", 0, 3},
		// A delta of 3 days counts back from that Wednesday to Sunday
		// (unconfirmed); one of under a day is day 0, Wednesday.
		{"delta 3 days", delta(3 * day), 7},
		{"delta 1 hour", delta(hour), 3},
	} {
		putQuad(t, env, daysTime, c.v)
		putLongword(t, env, daysNumber, 0)

		if got := call(t, env, "LIB$DAY_OF_WEEK", daysTime, daysNumber); got != ssNormal || longword(t, env, daysNumber) != c.want {
			t.Errorf("%s: %#x, %d; want SS$_NORMAL, %d", c.name, got, longword(t, env, daysNumber), c.want)
		}
	}

	// The current time, with the time omitted (0 by value).
	putLongword(t, env, daysNumber, 0)

	if got := call(t, env, "LIB$DAY_OF_WEEK", 0, daysNumber); got != ssNormal || longword(t, env, daysNumber) != 3 {
		t.Errorf("now: %#x, %d; want SS$_NORMAL, 3", got, longword(t, env, daysNumber))
	}
}
