package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
)

// The output tests' arguments: an output descriptor and its text, a
// time, the user-context cell (dtCell, dtcontext_test.go), a length, and
// the flags.
const (
	outDesc   = 0x3000
	outText   = 0x3100
	outTime   = 0x3200
	outLength = 0x3208
	outFlags  = 0x320C
)

// manualTime is the time of the manual's examples of the predefined
// formats, 13-Jan-1988 09:13:25.14 (a Wednesday).
const manualTime = "13-JAN-1988 09:13:25.14"

// manualDates and manualTimes are the manual's examples of the
// predefined date and time formats (Tables 3-3 and 3-4), for manualTime,
// in the order of lnm.DateFormats and lnm.TimeFormats.
var manualDates = []string{
	"13-JAN-1988",
	"13 JANUARY 1988",
	"13.JANUARY 1988",
	"13.JANUARY.1988",
	"13 JANUARY 88",
	"13 JAN 88",
	"13.JAN 88",
	"13.JAN.88",
	"13 JAN 1988",
	"13.JAN 1988",
	"13.JAN.1988",
	"JANUARY 13, 1988",
	"01/13/88",
	"01-13-88",
	"01.13.88",
	"01 13 88",
	"13/01/88",
	"13/01-88",
	"13-01-88",
	"13.01.88",
	"13 01 88",
	"88/01/13",
	"88-01-13",
	"88.01.13",
	"88 01 13",
	"880113",
	"/88.01.13",
	"01/13/1988",
	"01-13-1988",
	"01.13.1988",
	"01 13 1988",
	"13/01/1988",
	"13-01-1988",
	"13.01.1988",
	"13 01 1988",
	"1988/01/13",
	"1988-01-13",
	"1988.01.13",
	"1988 01 13",
	"19880113",
}

var manualTimes = []string{
	"09:13:25.14",
	"09:13:25",
	"09.13.25",
	"09 13 25",
	"09:13",
	"09.13",
	"09 13",
	"9:13",
	"9.13",
	"9 13",
	"09:13 AM",
	"9:13 AM",
	"0913",
	"09H13m",
	"kl 09.13",
	"09H13'",
	"09.13 h",
	"h 09.13",
	"9 h 13",
	"9 h 13 min 25 s",
}

// englishSettings are the settings of a context with nothing defined:
// English, the standard formats.
func englishSettings(t *testing.T, env *corevms.Environment) *dtSettings {
	t.Helper()

	s, status, err := contextSettings(env, 0)
	if err != nil || status != ssNormal {
		t.Fatalf("contextSettings: %#x, %v", status, err)
	}

	return s
}

// formatWith writes v by the one format text, in the given settings.
func formatWith(t *testing.T, s *dtSettings, text string, v uint64) string {
	t.Helper()

	items, ok := parseDateFormat(text)
	if !ok {
		t.Fatalf("%q doesn't parse", text)
	}

	return s.formatTime([]dtElement{{text: text, items: items, slot: slotOf(items)}}, v)
}

// TestPredefinedFormatsOutput: each of the 60 predefined formats writes
// the manual's example for the manual's time.
func TestPredefinedFormatsOutput(t *testing.T) {
	env := fixture(t)
	s := englishSettings(t, env)
	v := absolute(t, env, manualTime)

	for i, format := range lnm.DateFormats {
		if got := formatWith(t, s, format, v); got != manualDates[i] {
			t.Errorf("LIB$DATE_FORMAT_%03d %q: %q, want %q", i+1, format, got, manualDates[i])
		}
	}

	for i, format := range lnm.TimeFormats {
		if got := formatWith(t, s, format, v); got != manualTimes[i] {
			t.Errorf("LIB$TIME_FORMAT_%03d %q: %q, want %q", i+1, format, got, manualTimes[i])
		}
	}
}

// TestFormatFields: the mnemonics the predefined formats don't show,
// and the 12-hour clock around midnight and noon.
func TestFormatFields(t *testing.T) {
	env := fixture(t)
	s := englishSettings(t, env)

	for _, c := range []struct{ time, format, want string }{
		{manualTime, "!WU !WAU !WC !WAC !WL !WAL", "WEDNESDAY WED Wednesday Wed wednesday wed"},
		{manualTime, "!MAC !MAAC !MAL !MAAL", "January Jan january jan"},
		{manualTime, "[!MNM] [!MNB] [!MN0]", "[1] [ 1] [01]"},
		{"5-JAN-1988 00:00:00.00", "[!DD] [!DB] [!D0]", "[5] [ 5] [05]"},
		{manualTime, "!Y4 !Y3 !Y2 !Y1", "1988 988 88 8"},
		{"1-JAN-2005 00:00:00.00", "!Y3 !Y2 !Y1", "005 05 5"},
		{"1-JAN-1988 09:03:05.00", "[!HH4] [!HB4] [!MM] [!MB] [!SS] [!SB]", "[9] [ 9] [3] [ 3] [5] [ 5]"},
		{"1-JAN-1988 00:05:00.00", "!H02:!M0 !MIU !MIC !MIL", "12:05 AM Am am"},
		{"1-JAN-1988 11:59:59.99", "!HB2 !MIU", "11 AM"},
		{"1-JAN-1988 12:00:00.00", "!H02 !MIU", "12 PM"},
		{"1-JAN-1988 23:59:00.00", "!HH2 !MIU", "11 PM"},
		{"1-JAN-1988 13:00:00.00", "[!HB2]", "[ 1]"},
	} {
		if got := formatWith(t, s, c.format, absolute(t, env, c.time)); got != c.want {
			t.Errorf("%s by %q: %q, want %q", c.time, c.format, got, c.want)
		}
	}

	// The fraction is truncated to its digits: 0.1234567 seconds.
	v := absolute(t, env, manualTime) - 1_400_000 + 1_234_567
	if got := formatWith(t, s, "!C7 !C6 !C5 !C4 !C3 !C2 !C1", v); got != "1234567 123456 12345 1234 123 12 1" {
		t.Errorf("fractions: %q", got)
	}
}

// TestMaximumLength: each field at its widest in English, and a blank
// between the parts.
func TestMaximumLength(t *testing.T) {
	env := fixture(t)
	s := englishSettings(t, env)

	for _, c := range []struct {
		formats []string
		want    int
	}{
		{[]string{lnm.DateFormats[0], lnm.TimeFormats[0]}, 23}, // $ASCTIM's
		// The manual's example says 28 for "SEPTEMBER 21, 1988 11:24 PM".
		{[]string{lnm.DateFormats[11], lnm.TimeFormats[11]}, 27},
		{[]string{"!WU !WAL"}, 13},
		{[]string{"!C7"}, 7},
		{[]string{"!MIL"}, 2},
	} {
		var elements []dtElement

		for _, text := range c.formats {
			items, _ := parseDateFormat(text)
			elements = append(elements, dtElement{text: text, items: items, slot: slotOf(items)})
		}

		if got := s.maximumLength(elements); got != c.want {
			t.Errorf("%q: %d, want %d", c.formats, got, c.want)
		}
	}
}

// TestCheckInputFormat: an input format needs each of the seven fields
// once, a meridiem exactly with the 12-hour clock, and the date's and
// time's fields in two groups.
func TestCheckInputFormat(t *testing.T) {
	for _, c := range []struct {
		format string
		ok     bool
	}{
		{standardInputFormat, true},
		{"!MAU !DD, !Y4 !H04:!M0:!S0:!C2", true},
		{"!MN0/!D0/!Y2 !H02:!M0:!S0.!C2 !MIU", true},
		{"!H04:!M0:!S0.!C2 !D0-!MN0-!Y4", true},  // time first
		{"!D0-!MN0-!Y4 !H04:!M0:!S0", false},     // no fraction
		{"!D0-!MN0-!Y4 !H02:!M0:!S0.!C2", false}, // 12-hour, no meridiem
		{"!D0-!MN0-!Y4 !H04:!M0:!S0.!C2 !MIU", false},
		{"!D0-!MN0 !H04:!M0:!S0.!C2 !Y4", false}, // three groups
		{"!WU !D0-!MN0-!Y4 !H04:!M0:!S0.!C2", false},
		{"!D0-!MN0-!MAU-!Y4 !H04:!M0:!S0.!C2", false}, // two months
		{"!D0-!MN0-!Y4 !H04:!M0:!S0.!C2.!C2", false},  // a field twice
	} {
		items, ok := parseDateFormat(c.format)
		if !ok {
			t.Fatalf("%q doesn't parse", c.format)
		}

		if got := checkInputFormat(items); got != c.ok {
			t.Errorf("%q: %v, want %v", c.format, got, c.ok)
		}
	}
}

// formatCall calls LIB$FORMAT_DATE_TIME into a fixed string of size
// bytes, with the time at outTime, the context at dtCell (or the static
// one with cell 0), the length, and the flags (none when flags < 0). It
// returns the status, the text, and the length.
func formatCall(t *testing.T, env *corevms.Environment, size uint16, v uint64, cell uint32, flags int) (uint32, string, uint32) {
	t.Helper()

	putDesc(t, env, outDesc, size, dscClassS, outText)
	putQuad(t, env, outTime, v)
	putLongword(t, env, outLength, 0xFFFFFFFF)

	flagsArg := uint32(0)
	if flags >= 0 {
		putLongword(t, env, outFlags, uint32(flags))
		flagsArg = outFlags
	}

	status := call(t, env, "LIB$FORMAT_DATE_TIME", outDesc, outTime, cell, outLength, flagsArg)

	return status, descText(t, env, outDesc), longword(t, env, outLength)
}

// TestLibFormatDateTime: the routine through the shim table, by the
// logical names: the standard format with nothing defined (ENGLUSED and
// DEFFORUSE), LIB$DT_FORMAT's parts, the flags, a cut string, a delta,
// and the current time.
func TestLibFormatDateTime(t *testing.T) {
	env := fixture(t)
	v := absolute(t, env, manualTime)

	// Nothing defined: $ASCTIM's text, English used.
	if got, text, n := formatCall(t, env, 23, v, 0, -1); got != libEnglUsed || text != manualTime || n != 23 {
		t.Errorf("nothing defined: %#x, %q, %d", got, text, n)
	}

	// A short string: cut, and the length is what was written.
	if got, text, n := formatCall(t, env, 11, v, 0, -1); got != libStrTru || text != "13-JAN-1988" || n != 11 {
		t.Errorf("11 bytes: %#x, %q, %d", got, text, n)
	}

	if got, _, n := formatCall(t, env, 23, uint64(-ticksPerDay), 0, -1); got != libAbsTimReq || n != 0xFFFFFFFF {
		t.Errorf("a delta: %#x, length %#x", got, n)
	}

	// The names, read by a new context: time first, then date.
	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")
	defineList(t, env, lnm.ProcessTableName, "LIB$DT_FORMAT", lnm.Supervisor, "LIB$TIME_FORMAT_012", "LIB$DATE_FORMAT_012")

	putLongword(t, env, dtCell, 0)

	for _, c := range []struct {
		flags  int
		status uint32
		want   string
	}{
		{-1, ssNormal, "9:13 AM JANUARY 13, 1988"},
		{3, ssNormal, "9:13 AM JANUARY 13, 1988"},
		{4 | 1, ssNormal, "9:13 AM"}, // other bits ignored
		{2, ssNormal, "JANUARY 13, 1988"},
		{0, ssNormal, ""},
	} {
		if got, text, _ := formatCall(t, env, 40, v, dtCell, c.flags); got != c.status || text != padded(c.want, 40) {
			t.Errorf("flags %d: %#x, %q; want %#x, %q", c.flags, got, text, c.status, c.want)
		}
	}

	// A format with only a time part: the flags' date is the standard
	// one, first, with DEFFORUSE.
	defineList(t, env, lnm.ProcessTableName, "LIB$DT_FORMAT", lnm.Supervisor, "LIB$TIME_FORMAT_005")
	putLongword(t, env, dtCell, 0)

	if got, text, _ := formatCall(t, env, 40, v, dtCell, -1); got != ssNormal || text != padded("09:13", 40) {
		t.Errorf("time only: %#x, %q", got, text)
	}

	// The standard date follows, with no warning, as on VMS.
	if got, text, _ := formatCall(t, env, 40, v, dtCell, 3); got != ssNormal || text != padded("09:13 13-JAN-1988", 40) {
		t.Errorf("time only, both flags: %#x, %q", got, text)
	}

	// No date argument: the current time.
	env.Clock = func() uint64 { return v }

	if got := call(t, env, "LIB$FORMAT_DATE_TIME", outDesc); got != libEnglUsed && got != ssNormal {
		t.Errorf("the current time: %#x", got)
	}

	if text := descText(t, env, outDesc); text != padded("13-JAN-1988 09:13:25.14", 40) {
		t.Errorf("the current time: %q", text)
	}
}

// padded is s with blanks to n bytes, as a fixed string holds it.
func padded(s string, n int) string {
	for len(s) < n {
		s += " "
	}

	return s
}

// TestLibGetMaximumDateLength: the routine through the shim table, with
// an initialized context and the flags.
func TestLibGetMaximumDateLength(t *testing.T) {
	env := fixture(t)
	undefineDTNames(t, env)
	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")

	if got := initContext(t, env, componentOutputFormat, "|!MAU !DD, !Y4|!HH2:!M0 !MIU|"); got != ssNormal {
		t.Fatalf("OUTPUT_FORMAT: %#x", got)
	}

	for _, c := range []struct {
		flags int
		want  uint32
	}{{-1, 27}, {3, 27}, {2, 18}, {1, 8}, {0, 0}} {
		flagsArg := uint32(0)
		if c.flags >= 0 {
			putLongword(t, env, outFlags, uint32(c.flags))
			flagsArg = outFlags
		}

		if got := call(t, env, "LIB$GET_MAXIMUM_DATE_LENGTH", outLength, dtCell, flagsArg); got != ssNormal || longword(t, env, outLength) != c.want {
			t.Errorf("flags %d: %#x, %d; want %d", c.flags, got, longword(t, env, outLength), c.want)
		}
	}

	// The static context, no LIB$DT_FORMAT: $ASCTIM's 23, the standard
	// format used.
	if got := call(t, env, "LIB$GET_MAXIMUM_DATE_LENGTH", outLength); got != libDefForUse || longword(t, env, outLength) != 23 {
		t.Errorf("static: %#x, %d", got, longword(t, env, outLength))
	}
}

// TestLibGetDateFormat: the manual's three input formats in legible
// form, the standard one's with nothing defined, and formats that can't
// be read by.
func TestLibGetDateFormat(t *testing.T) {
	env := fixture(t)
	undefineDTNames(t, env)
	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")

	for _, c := range []struct{ format, want string }{
		{"!MAU !DD, !Y4 !H04:!M0:!S0:!C2", "MONTH DD, YYYY4 HH:MM:SS:CC2"},
		{"!MN0-!D0-!Y2 !H04:!M0:!S0.!C2", "MM-DD-YYYY2 HH:MM:SS.CC2"},
		{"!MAAU !D0, !Y2 !H02:!M0:!S0.!C4 !MIU", "MONTH DD, YYYY2 HH:MM:SS.CC4 AM/PM"},
		{"  !MN0/!D0/!Y2 \t !H02:!M0:!S0.!C2 !MIU ", "MM/DD/YYYY2 HH:MM:SS.CC2 AM/PM"},
	} {
		putLongword(t, env, dtCell, 0)

		if got := initContext(t, env, componentInputFormat, "|"+c.format+"|"); got != ssNormal {
			t.Fatalf("INPUT_FORMAT %q: %#x", c.format, got)
		}

		putDesc(t, env, outDesc, 0, dscClassD, 0)

		if got := call(t, env, "LIB$GET_DATE_FORMAT", outDesc, dtCell); got != ssNormal || descText(t, env, outDesc) != c.want {
			t.Errorf("%q: %#x, %q; want %q", c.format, got, descText(t, env, outDesc), c.want)
		}
	}

	putLongword(t, env, dtCell, 0)

	if got := initContext(t, env, componentInputFormat, "|!D0-!MN0-!Y4 !H04:!M0:!S0|"); got != ssNormal {
		t.Fatalf("INPUT_FORMAT: %#x", got)
	}

	if got := call(t, env, "LIB$GET_DATE_FORMAT", outDesc, dtCell); got != libIllFormat {
		t.Errorf("no fraction: %#x, want ILLFORMAT", got)
	}

	// The static context, no LIB$DT_INPUT_FORMAT: the standard format,
	// with DEFFORUSE.
	if got := call(t, env, "LIB$GET_DATE_FORMAT", outDesc); got != libDefForUse || descText(t, env, outDesc) != "DD-MONTH-YYYY4 HH:MM:SS.CC2" {
		t.Errorf("standard: %#x, %q", got, descText(t, env, outDesc))
	}

	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")
	putLongword(t, env, dtCell, 0)

	if got := call(t, env, "LIB$GET_DATE_FORMAT", outDesc, dtCell); got != libDefForUse {
		t.Errorf("no LIB$DT_INPUT_FORMAT: %#x, want DEFFORUSE", got)
	}

	putDesc(t, env, outDesc, 5, dscClassS, outText)

	if got := call(t, env, "LIB$GET_DATE_FORMAT", outDesc, dtCell); got != libStrTru || descText(t, env, outDesc) != "DD-MO" {
		t.Errorf("5 bytes: %#x, %q", got, descText(t, env, outDesc))
	}

	for _, argv := range [][]uint32{{}, {outDesc, dtCell, 0}} {
		if got := call(t, env, "LIB$GET_DATE_FORMAT", argv...); got != libWroNumArg {
			t.Errorf("%d arguments: %#x, want WRONUMARG", len(argv), got)
		}
	}
}
