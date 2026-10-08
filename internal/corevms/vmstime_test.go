package corevms

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

func TestServiceSysGettim(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)
	*now = 0x00A1B2C3D4E5F607

	timadr := a.quad(0)
	wantR0(t, callLNM(t, env, serviceSysGettim, timadr), ssNormal)

	if got, _ := env.loadQuad(timadr); got != *now {
		t.Errorf("$GETTIM stored %#x, want the clock's %#x", got, *now)
	}

	// The clock moves on; a second call sees it.
	*now += 5 * ms

	wantR0(t, callLNM(t, env, serviceSysGettim, timadr), ssNormal)

	if got, _ := env.loadQuad(timadr); got != *now {
		t.Errorf("second $GETTIM stored %#x, want %#x", got, *now)
	}
}

func TestServiceSysGettimAccvio(t *testing.T) {
	env, _ := fixture()

	// timadr 0, omitted, or outside memory.
	wantR0(t, callLNM(t, env, serviceSysGettim, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysGettim), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysGettim, 0x7FFFFFF0), ssAccVio)
}

// manualNow is the current time the manual's $BINTIM examples assume:
// 30-DEC-1988 04:15:28.00.
var manualNow = vmsdef.Time(time.Date(1988, time.December, 30, 4, 15, 28, 0, time.UTC))

// TestParseVMSTimeManualExamples runs the $BINTIM example table from the
// VMS 5.0 System Services Reference Manual through parseVMSTime and then
// formatVMSTime, as the manual does with $BINTIM and $ASCTIM.
func TestParseVMSTimeManualExamples(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"-- :50", "30-DEC-1988 04:50:28.00"},
		// The manual prints 29-DEC-1989 for this one, which contradicts
		// its own rule that an omitted day is today's: taken as a typo.
		{"--1989 0:0:0.0", "30-DEC-1989 00:00:00.00"},
		{"30-DEC-1988 12:32:1.1161", "30-DEC-1988 12:32:01.12"},
		{"29-DEC-1989 16:35:0.0", "29-DEC-1989 16:35:00.00"},
		{"0 ::.1", "   0 00:00:00.10"},
		{"0 ::.06", "   0 00:00:00.06"},
		{"5 3:18:32.068", "   5 03:18:32.07"},
		{"20 12:", "  20 12:00:00.00"},
		{"05", "   0 05:00:00.00"},
	} {
		v, ok := parseVMSTime(tc.in, manualNow)
		if !ok {
			t.Errorf("parseVMSTime(%q) failed", tc.in)

			continue
		}

		if got, _ := formatVMSTime(v, false); got != tc.want {
			t.Errorf("parseVMSTime(%q) formats as %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseVMSTimeForms(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"  30-DEC-1988    12:00  ", "30-DEC-1988 12:00:28.00"}, // blanks around fields
		{"1-JAN", " 1-JAN-1988 04:15:28.00"},                    // trailing fields dropped
		{"29-FEB-1988 00:00", "29-FEB-1988 00:00:28.00"},        // a leap day
		{"17-NOV-1858 00:00:00.00", "17-NOV-1858 00:00:00.00"},  // time zero
		{"31-DEC-9999 23:59:59.99", "31-DEC-9999 23:59:59.99"},  // the last representable
		{"0 ::59.999", "   0 00:01:00.00"},                      // rounding carries
		{"9999 23:59:59.99", "9999 23:59:59.99"},                // the largest delta
		{"0 1:2:3", "   0 01:02:03.00"},
	} {
		v, ok := parseVMSTime(tc.in, manualNow)
		if !ok {
			t.Errorf("parseVMSTime(%q) failed", tc.in)

			continue
		}

		if got, _ := formatVMSTime(v, false); got != tc.want {
			t.Errorf("parseVMSTime(%q) formats as %q, want %q", tc.in, got, tc.want)
		}
	}

	// A delta is negative, an absolute time positive; a zero delta is 0.
	if v, _ := parseVMSTime("0 0:0:10", manualNow); int64(v) != -10*vmsdef.TicksPerSecond {
		t.Errorf("10-second delta = %d, want %d", int64(v), -10*vmsdef.TicksPerSecond)
	}

	if v, _ := parseVMSTime("0 ::", manualNow); v != 0 {
		t.Errorf("zero delta = %d, want 0", v)
	}
}

func TestParseVMSTimeErrors(t *testing.T) {
	for _, in := range []string{
		"", "   ", "1 2 3", // no fields, too many
		"30-Dec-1988", "30-XYZ-1988", // month case and name
		"-FEB-", "31-APR-1988", "29-FEB-1989", "0-JAN-1988", "32-JAN-1988", // days
		"1-JAN-1857", "16-NOV-1858", "1-JAN-10000", "1-JAN-19a8", // years
		"30-DEC-1988-1", "1-JAN-1988 1:2:3:4", // too many parts
		"0 24:00", "0 :60", "0 ::60", "0 123", // time ranges and widths
		"10000 0:", "9999 23:59:59.999", // 10,000 days, directly and by rounding
		"0 12.5", "0 ::1.5x", "a", "0 1 :2", "-1 0:", // punctuation and junk
	} {
		if v, ok := parseVMSTime(in, manualNow); ok {
			t.Errorf("parseVMSTime(%q) = %#x, want SS$_IVTIME", in, v)
		}
	}
}

// delta is the VMS delta time for an interval of ticks: its negation.
func delta(ticks int64) uint64 { return uint64(-ticks) }

func TestFormatVMSTime(t *testing.T) {
	v := vmsdef.Time(time.Date(1988, time.October, 9, 7, 5, 3, 456_789_000, time.UTC))

	for _, tc := range []struct {
		v        uint64
		timeOnly bool
		want     string
	}{
		{v, false, " 9-OCT-1988 07:05:03.45"}, // day padded with a blank; hundredths truncated
		{v, true, "07:05:03.45"},
		{delta(3*ticksPerDay + 90*vmsdef.TicksPerSecond), false, "   3 00:01:30.00"},
		{delta(3*ticksPerDay + 90*vmsdef.TicksPerSecond), true, "00:01:30.00"},
	} {
		if got, ok := formatVMSTime(tc.v, tc.timeOnly); !ok || got != tc.want {
			t.Errorf("formatVMSTime(%#x, %v) = %q, %v; want %q", tc.v, tc.timeOnly, got, ok, tc.want)
		}
	}

	if _, ok := formatVMSTime(delta(maxDeltaDays*ticksPerDay), false); ok {
		t.Error("a 10,000-day delta formatted, want failure")
	}
}

func TestServiceSysAsctim(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)
	*now = manualNow

	// timadr omitted: the current time, and its length.
	timlen := a.alloc(2)
	desc, buf := a.outDesc(23)
	wantR0(t, callLNM(t, env, serviceSysAsctim, timlen, desc), ssNormal)

	if got := a.readString(buf, 23); got != "30-DEC-1988 04:15:28.00" {
		t.Errorf("$ASCTIM(now) = %q", got)
	}

	if n := a.readLong(timlen) & 0xFFFF; n != 23 {
		t.Errorf("timlen = %d, want 23", n)
	}

	// A given time, time only; timlen omitted.
	desc, buf = a.outDesc(11)
	wantR0(t, callLNM(t, env, serviceSysAsctim, 0, desc, a.quad(-int64(90*vmsdef.TicksPerSecond)), 1), ssNormal)

	if got := a.readString(buf, 11); got != "00:01:30.00" {
		t.Errorf("$ASCTIM(90s delta, time only) = %q", got)
	}

	// A 12-byte buffer gets the date: truncation is SS$_BUFFEROVF.
	desc, buf = a.outDesc(12)
	wantR0(t, callLNM(t, env, serviceSysAsctim, timlen, desc), ssBufferOvf)

	if got := a.readString(buf, 12); got != "30-DEC-1988 " {
		t.Errorf("$ASCTIM into 12 bytes = %q", got)
	}

	if n := a.readLong(timlen) & 0xFFFF; n != 12 {
		t.Errorf("timlen = %d, want 12", n)
	}
}

func TestServiceSysAsctimErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	desc, _ := a.outDesc(23)

	wantR0(t, callLNM(t, env, serviceSysAsctim, 0, desc, a.quad(-int64(maxDeltaDays*ticksPerDay))), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysAsctim, 0, 0), ssAccVio)                // no timbuf
	wantR0(t, callLNM(t, env, serviceSysAsctim, 0, desc, 0x7FFFFFF0), ssAccVio) // unreadable timadr
	wantR0(t, callLNM(t, env, serviceSysAsctim, 0x7FFFFFF0, desc), ssAccVio)    // unwritable timlen
}

func TestServiceSysBintim(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)
	*now = manualNow

	timadr := a.quad(0)
	wantR0(t, callLNM(t, env, serviceSysBintim, a.desc("-- :50"), timadr), ssNormal)

	want, _ := parseVMSTime("30-DEC-1988 04:50:28.00", manualNow)
	if got, _ := env.loadQuad(timadr); got != want {
		t.Errorf("$BINTIM stored %#x, want %#x", got, want)
	}

	// A delta converted by $BINTIM works as a $SETIMR time.
	wantR0(t, callLNM(t, env, serviceSysBintim, a.desc("0 ::10"), timadr), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysSetimr, 3, timadr), ssNormal)

	*now += 10*vmsdef.TicksPerSecond - 1

	wantR0(t, callLNM(t, env, readefState, 3), ssWasClr)

	*now++

	wantR0(t, callLNM(t, env, readefState, 3), ssWasSet)
}

func TestServiceSysBintimErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	timadr := a.quad(0)

	wantR0(t, callLNM(t, env, serviceSysBintim, a.desc("31-FEB-1988"), timadr), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysBintim, a.desc(strings.Repeat(" ", maxTimeString+1)), timadr), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysBintim, a.desc("0 ::10"), 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysBintim, a.desc("0 ::10"), 0x7FFFFFF0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysBintim), ssAccVio)

	if got, _ := env.loadQuad(timadr); got != 0 {
		t.Errorf("failed $BINTIM calls stored %#x, want nothing", got)
	}
}

// numtimWords reads the seven words $NUMTIM stored at timbuf.
func numtimWords(t *testing.T, env *Environment, timbuf uint32) [7]uint16 {
	t.Helper()

	var out [7]uint16

	for i := range out {
		w, err := env.mem.LoadWord(env.cpu, timbuf+uint32(2*i))
		if err != nil {
			t.Fatal(err)
		}

		out[i] = w
	}

	return out
}

func TestServiceSysNumtim(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	timbuf := a.alloc(14)

	parse := func(s string) int64 {
		t.Helper()

		v, ok := parseVMSTime(s, 0)
		if !ok {
			t.Fatalf("parseVMSTime(%q) failed", s)
		}

		return int64(v)
	}

	cases := []struct {
		name string
		time int64
		want [7]uint16
	}{
		{"the base date", 0, [7]uint16{1858, 11, 17, 0, 0, 0, 0}},
		{"an absolute time", parse("29-FEB-2000 23:59:59.99"), [7]uint16{2000, 2, 29, 23, 59, 59, 99}},
		{"hundredths truncate", parse("1-JAN-1990 00:00:00.00") + 99_999, [7]uint16{1990, 1, 1, 0, 0, 0, 0}},
		{"a delta time", parse("5 03:18:32.07"), [7]uint16{0, 0, 5, 3, 18, 32, 7}},
		{"the longest delta", parse("9999 23:59:59.99"), [7]uint16{0, 0, 9999, 23, 59, 59, 99}},
	}

	for _, c := range cases {
		wantR0(t, callLNM(t, env, serviceSysNumtim, timbuf, a.quad(c.time)), ssNormal)

		if got := numtimWords(t, env, timbuf); got != c.want {
			t.Errorf("%s: $NUMTIM = %v, want %v", c.name, got, c.want)
		}
	}

	// With timadr omitted, the current time.
	*fakeClock(env) = uint64(parse("4-JUL-1976 12:30:00.50"))
	wantR0(t, callLNM(t, env, serviceSysNumtim, timbuf), ssNormal)

	if got, want := numtimWords(t, env, timbuf), [7]uint16{1976, 7, 4, 12, 30, 0, 50}; got != want {
		t.Errorf("$NUMTIM of the current time = %v, want %v", got, want)
	}

	// Errors.
	wantR0(t, callLNM(t, env, serviceSysNumtim, timbuf, a.quad(-10_000*ticksPerDay)), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysNumtim, timbuf, badAddr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysNumtim, 0, a.quad(0)), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysNumtim, badAddr, a.quad(0)), ssAccVio)
}
