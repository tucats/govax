package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// The input tests' arguments: the string's descriptor and text, the
// result, the flags, the defaults vector, and the defaulted fields.
const (
	inDesc      = 0x3400
	inText      = 0x3500
	inResult    = 0x3600
	inFlags     = 0x3608
	inDefaults  = 0x3610
	inDefaulted = 0x3620
)

// manualToday is the current time of the manual's Table 3-2, whose
// examples assume a current date of 25-FEB-1988.
const manualToday = "25-FEB-1988 10:11:12.13"

// inputSettings are English settings that read by the input format
// format (the standard one when it's "").
func inputSettings(t *testing.T, env *corevms.Environment, format string) *dtSettings {
	t.Helper()

	s := *englishSettings(t, env)

	if format != "" {
		items, ok := parseDateFormat(format)
		if !ok || !checkInputFormat(items) {
			t.Fatalf("%q isn't an input format", format)
		}

		s.input, s.inputText = items, format
	}

	return &s
}

// converted is a conversion's result as text ($ASCTIM's), or "" when
// there is none.
func converted(t *testing.T, v uint64, status uint32) string {
	t.Helper()

	if status != ssNormal {
		return ""
	}

	text, ok := corevms.FormatTime(v, false)
	if !ok {
		t.Fatalf("%#x isn't a time", v)
	}

	return text
}

// todayDefaults are the defaults with no defaults vector, for now: its
// date and midnight.
func todayDefaults(t *testing.T, env *corevms.Environment, now uint64) [numFields]int {
	t.Helper()

	d, err := conversionDefaults(env, 0, now)
	if err != nil {
		t.Fatal(err)
	}

	return d
}

// TestConvertManualTable: the manual's Table 3-2, with every field
// allowed to be omitted: the string's punctuation places its values, and
// the rest are today's date and midnight.
func TestConvertManualTable(t *testing.T) {
	env := fixture(t)
	s := inputSettings(t, env, "")
	now := absolute(t, env, manualToday)
	c := dtConversion{omittable: 0x7F, defaults: todayDefaults(t, env, now), now: now}

	for _, tc := range []struct {
		input     string
		want      string
		status    uint32
		defaulted uint32
	}{
		// The manual's first example is 31 February, which doesn't exist.
		{"31", "", libIvTime, 0},
		{"21", "21-FEB-1988 00:00:00.00", ssNormal, 0x7B},
		{"-MAR", "25-MAR-1988 00:00:00.00", ssNormal, 0x7D},
		{"-SEPTEMBER", "25-SEP-1988 00:00:00.00", ssNormal, 0x7D},
		{"-1988", "25-FEB-1988 00:00:00.00", ssNormal, 0x7E},
		{"23:", "25-FEB-1988 23:00:00.00", ssNormal, 0x77},
		{":45:", "25-FEB-1988 00:45:00.00", ssNormal, 0x6F},
		{"::23", "25-FEB-1988 00:00:23.00", ssNormal, 0x5F},
		{".01", "25-FEB-1988 00:00:00.01", ssNormal, 0x3F},

		// Whole strings, and pieces of them.
		{"13-JAN-1988 09:13:25.14", "13-JAN-1988 09:13:25.14", ssNormal, 0},
		{"  9-oct-1990\t 1:02:03.4 ", " 9-OCT-1990 01:02:03.40", ssNormal, 0},
		{"13-JAN-1988", "13-JAN-1988 00:00:00.00", ssNormal, 0x78},
		{"13-JAN-1988 09:13", "13-JAN-1988 09:13:00.00", ssNormal, 0x60},
		{"13--1988", "13-FEB-1988 00:00:00.00", ssNormal, 0x7A},
		{"13-January-1988", "13-JAN-1988 00:00:00.00", ssNormal, 0x78},
		{"13-Ja-1988", "13-JAN-1988 00:00:00.00", ssNormal, 0x78},
		{"", "25-FEB-1988 00:00:00.00", ssNormal, 0x7F},

		// Values out of range, and strings the format doesn't fit.
		{"29-FEB-1989", "", libIvTime, 0},
		{"13-JAN-1988 24:00", "", libIvTime, 0},
		{"13-JAN-1988 09:60", "", libIvTime, 0},
		{"13/JAN/1988", "", libIvTime, 0},
		{"hello", "", libIvTime, 0},
		{"13-JU-1988", "", libAmbDatTim, 0},
		{"13-MA-1988", "", libAmbDatTim, 0},
	} {
		v, defaulted, status := s.convertDateString(tc.input, c)
		if got := converted(t, v, status); status != tc.status || got != tc.want || defaulted != tc.defaulted {
			t.Errorf("%q: %#x, %q, defaulted %#x; want %#x, %q, %#x",
				tc.input, status, got, defaulted, tc.status, tc.want, tc.defaulted)
		}
	}
}

// TestConvertOmittable: an omitted field the flags don't allow is
// INCDATTIM; the default flags allow only the time's.
func TestConvertOmittable(t *testing.T) {
	env := fixture(t)
	s := inputSettings(t, env, "")
	now := absolute(t, env, manualToday)
	defaults := todayDefaults(t, env, now)

	for _, tc := range []struct {
		input     string
		omittable uint32
		status    uint32
	}{
		{"-MAR", defaultOmittable, libIncDatTim},
		{"23:", defaultOmittable, libIncDatTim},
		{"25-FEB-1988", defaultOmittable, ssNormal},
		{"25-FEB-1988 10:11", defaultOmittable &^ 0x20, libIncDatTim},
		{"25-FEB-1988 10:11:12.13", 0, ssNormal},
		{"", defaultOmittable, libIncDatTim},
	} {
		c := dtConversion{omittable: tc.omittable, defaults: defaults, now: now}
		if _, _, status := s.convertDateString(tc.input, c); status != tc.status {
			t.Errorf("%q, flags %#x: %#x, want %#x", tc.input, tc.omittable, status, tc.status)
		}
	}
}

// TestConvertFormats: other input formats: the manual's 12-hour one, a
// two-digit year, fields with no punctuation between them, and a
// fraction to the tick.
func TestConvertFormats(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, manualToday)

	for _, tc := range []struct {
		format, input string
		want          string
		status        uint32
	}{
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "feb 25, 1988 04:39:02:55 am", "25-FEB-1988 04:39:02.55", ssNormal},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "FEBRUARY 25, 1988 04:39:02:55 PM", "25-FEB-1988 16:39:02.55", ssNormal},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25, 1988 12:00:00:00 AM", "25-FEB-1988 00:00:00.00", ssNormal},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25, 1988 12:30:00:00 p", "25-FEB-1988 12:30:00.00", ssNormal},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25, 1988 3:30", "25-FEB-1988 03:30:00.00", ssNormal},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25, 1988 13:00:00:00 PM", "", libIvTime},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25,1988", "", libIvTime},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25, 1988 4:39 pm", "25-FEB-1988 16:39:00.00", ssNormal},
		{"!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU", "Feb 25, 1988 pm", "25-FEB-1988 00:00:00.00", ssNormal},
		{"!MN0/!D0/!Y2 !H04:!M0:!S0.!C2", "01/13/88 09:13:25.14", "13-JAN-1988 09:13:25.14", ssNormal},
		{"!MN0/!D0/!Y2 !H04:!M0:!S0.!C2", "1/3/8", " 3-JAN-1908 00:00:00.00", ssNormal},
		{"!Y4!MN0!D0 !H04!M0!S0.!C2", "19880113 091325.14", "13-JAN-1988 09:13:25.14", ssNormal},
		{"!Y4!MN0!D0 !H04!M0!S0.!C2", "19880113", "13-JAN-1988 00:00:00.00", ssNormal},
		{"!H04:!M0:!S0.!C2 !D0-!MN0-!Y4", "09:13:25.14 13-01-1988", "13-JAN-1988 09:13:25.14", ssNormal},
		{"!H04:!M0:!S0.!C2 !D0-!MN0-!Y4", "13-01-1988", "13-JAN-1988 00:00:00.00", ssNormal},
	} {
		s := inputSettings(t, env, tc.format)
		c := dtConversion{omittable: defaultOmittable, defaults: todayDefaults(t, env, now), now: now}

		v, _, status := s.convertDateString(tc.input, c)
		if got := converted(t, v, status); status != tc.status || got != tc.want {
			t.Errorf("%q by %q: %#x, %q; want %#x, %q", tc.input, tc.format, status, got, tc.status, tc.want)
		}
	}

	// A seven-digit fraction is whole ticks.
	s := inputSettings(t, env, "!DB-!MAAU-!Y4 !H04:!M0:!S0.!C7")
	c := dtConversion{omittable: defaultOmittable, defaults: todayDefaults(t, env, now), now: now}

	v, _, status := s.convertDateString("13-JAN-1988 09:13:25.1234567", c)
	if status != ssNormal || v%vmsdef.TicksPerSecond != 1234567 {
		t.Errorf(".1234567: %#x, %d ticks", status, v%vmsdef.TicksPerSecond)
	}

	if _, _, status := s.convertDateString("13-JAN-1988 09:13:25.12345678", c); status != libIvTime {
		t.Errorf("eight digits: %#x, want IVTIME", status)
	}
}

// TestConvertDefaults: a defaults vector supplies the omitted fields,
// its century the two-digit year's.
func TestConvertDefaults(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, manualToday)
	defaults := [numFields]int{2026, 6, 15, 8, 30, 0, 50}

	for _, tc := range []struct {
		format, input, want string
	}{
		{"", "-MAR", "15-MAR-2026 08:30:00.50"},
		{"", ":45", "15-JUN-2026 08:45:00.50"},
		// A two-digit year's century is the current year's (1988's
		// here), not the defaults vector's.
		{"!MN0/!D0/!Y2 !H04:!M0:!S0.!C2", "01/13/88", "13-JAN-1988 08:30:00.50"},
	} {
		s := inputSettings(t, env, tc.format)
		c := dtConversion{omittable: 0x7F, defaults: defaults, now: now}

		v, _, status := s.convertDateString(tc.input, c)
		if got := converted(t, v, status); got != tc.want {
			t.Errorf("%q: %#x, %q; want %q", tc.input, status, got, tc.want)
		}
	}
}

// TestConvertRelativeDays: YESTERDAY, TODAY, and TOMORROW, in any case
// and abbreviated, are days counted from the current date, with the
// time from the defaults.
func TestConvertRelativeDays(t *testing.T) {
	env := fixture(t)
	s := inputSettings(t, env, "")
	now := absolute(t, env, "1-MAR-1988 10:11:12.13")
	c := dtConversion{omittable: defaultOmittable, defaults: todayDefaults(t, env, now), now: now}

	for _, tc := range []struct {
		input  string
		want   string
		status uint32
	}{
		{"YESTERDAY", "29-FEB-1988 00:00:00.00", ssNormal},
		{"today", " 1-MAR-1988 00:00:00.00", ssNormal},
		{" Tomorrow ", " 2-MAR-1988 00:00:00.00", ssNormal},
		{"tom", " 2-MAR-1988 00:00:00.00", ssNormal},
		{"y", "29-FEB-1988 00:00:00.00", ssNormal},
		{"T", "", libAmbDatTim},
		{"TO", "", libAmbDatTim},
	} {
		v, defaulted, status := s.convertDateString(tc.input, c)
		if got := converted(t, v, status); status != tc.status || got != tc.want {
			t.Errorf("%q: %#x, %q; want %#x, %q", tc.input, status, got, tc.status, tc.want)
		}

		if status == ssNormal && defaulted != 0x78 {
			t.Errorf("%q: defaulted %#x, want 0x78", tc.input, defaulted)
		}
	}

	c.omittable = 0

	if _, _, status := s.convertDateString("TODAY", c); status != libIncDatTim {
		t.Errorf("TODAY with no fields omittable: %#x, want INCDATTIM", status)
	}
}

// convertCall calls LIB$CONVERT_DATE_STRING on text with the cell (0 for
// the static context), the flags (none when flags < 0), and the defaults
// (none when nil), returning the status, the result, and the defaulted
// fields.
func convertCall(t *testing.T, env *corevms.Environment, text string, cell uint32, flags int, defaults []uint16) (uint32, uint64, uint32) {
	t.Helper()

	putDescriptor(t, env, inDesc, inText, text)
	putQuad(t, env, inResult, unchanged)
	putLongword(t, env, inDefaulted, 0xFFFFFFFF)

	flagsArg := uint32(0)
	if flags >= 0 {
		putLongword(t, env, inFlags, uint32(flags))
		flagsArg = inFlags
	}

	defaultsArg := uint32(0)
	if defaults != nil {
		for i, w := range defaults {
			putWord(t, env, inDefaults+uint32(2*i), w)
		}

		defaultsArg = inDefaults
	}

	status := call(t, env, "LIB$CONVERT_DATE_STRING", inDesc, inResult, cell, flagsArg, defaultsArg, inDefaulted)

	return status, quad(t, env, inResult), longword(t, env, inDefaulted)
}

// TestLibConvertDateString: the routine through the shim table: the
// arguments, the statuses, the flags and defaults arguments, and a
// context's input format.
func TestLibConvertDateString(t *testing.T) {
	env := fixture(t)
	now := absolute(t, env, manualToday)
	env.Clock = func() uint64 { return now }

	// The static context, nothing defined: the standard format, English
	// used.
	status, v, defaulted := convertCall(t, env, "13-JAN-1988 09:13", 0, -1, nil)
	if status != libEnglUsed || v != absolute(t, env, "13-JAN-1988 09:13:00.00") || defaulted != 0x60 {
		t.Errorf("standard: %#x, %#x, %#x", status, v, defaulted)
	}

	// Flags and defaults.
	status, v, defaulted = convertCall(t, env, "-MAR", 0, 0x7F, []uint16{1990, 6, 15, 8, 30, 0, 50})
	if status != libEnglUsed || v != absolute(t, env, "15-MAR-1990 08:30:00.50") || defaulted != 0x7D {
		t.Errorf("-MAR: %#x, %#x, %#x", status, v, defaulted)
	}

	// Bits past the seventh are ignored.
	if status, _, _ := convertCall(t, env, "-MAR", 0, 0xFFFFFF80|0x7D, nil); status != libEnglUsed {
		t.Errorf("high flag bits: %#x", status)
	}

	// An error writes nothing.
	status, v, defaulted = convertCall(t, env, "-MAR", 0, -1, nil)
	if status != libIncDatTim || v != unchanged || defaulted != 0xFFFFFFFF {
		t.Errorf("-MAR, default flags: %#x, %#x, %#x", status, v, defaulted)
	}

	// A context's input format; one the routine can't read by. An
	// initialized context still takes its language from SYS$LANGUAGE,
	// which the static context, read before, didn't have (ENGLUSED).
	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")
	putLongword(t, env, dtCell, 0)

	if got := initContext(t, env, componentInputFormat, "|!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU|"); got != ssNormal {
		t.Fatalf("INPUT_FORMAT: %#x", got)
	}

	status, v, _ = convertCall(t, env, "feb 25, 1988 04:39:02:55 pm", dtCell, -1, nil)
	// The context was given an input format, so no default is used.
	if status != ssNormal || v != absolute(t, env, "25-FEB-1988 16:39:02.55") {
		t.Errorf("12-hour: %#x, %#x", status, v)
	}

	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")
	defineList(t, env, lnm.ProcessTableName, "LIB$DT_INPUT_FORMAT", lnm.Supervisor, "!D0.!MN0.!Y4 !H04:!M0:!S0.!C2")
	putLongword(t, env, dtCell, 0)

	if status, v, _ := convertCall(t, env, "13.01.1988 09:13:25.14", dtCell, -1, nil); status != ssNormal || v != absolute(t, env, "13-JAN-1988 09:13:25.14") {
		t.Errorf("LIB$DT_INPUT_FORMAT: %#x, %#x", status, v)
	}

	defineList(t, env, lnm.ProcessTableName, "LIB$DT_INPUT_FORMAT", lnm.Supervisor, "!D0.!MN0.!Y4 !H04:!M0:!S0")
	putLongword(t, env, dtCell, 0)

	if status, _, _ := convertCall(t, env, "13.01.1988 09:13:25", dtCell, -1, nil); status != libIllFormat {
		t.Errorf("no fraction: %#x, want ILLFORMAT", status)
	}

	// The arguments.
	for _, tc := range []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"one argument", []uint32{inDesc}, libWroNumArg},
		{"seven arguments", []uint32{inDesc, inResult, 0, 0, 0, 0, 0}, libWroNumArg},
		{"no string", []uint32{0, inResult}, libInvArg},
		{"no result", []uint32{inDesc, 0}, libInvArg},
	} {
		if got := call(t, env, "LIB$CONVERT_DATE_STRING", tc.argv...); got != tc.want {
			t.Errorf("%s: %#x, want %#x", tc.name, got, tc.want)
		}
	}
}
