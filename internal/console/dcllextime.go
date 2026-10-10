package console

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The lexical functions on time (docs/PHASE-50 - DCL command
// procedures.md, subtask 14; the User's Manual, 15.3 and 15.6): F$TIME
// and F$CVTIME. The time is the system's (the console engine's clock,
// as $GETTIM reads it).
//
// DCL writes times as the User's Manual's chapter 1 describes them:
//
//	absolute     [dd-mmm-yyyy][:hh:mm:ss.cc]   (or a blank before the time)
//	             TODAY, TOMORROW, YESTERDAY, each with an optional :time
//	delta        [dddd-][hh:mm:ss.cc]
//	combination  absolute+delta or absolute-delta
//
// Fields may be left out: a date's fields are today's, and a time's are
// 0, so "14:00" is two o'clock today and "9-OCT-2026" is its midnight
// (unconfirmed against VMS: whether an absolute time's missing fields
// are 0 or the current time's, as $BINTIM's are).

// lexTime is F$TIME(): the current date and time, as $ASCTIM writes it,
// " 9-OCT-2026 14:23:45.12".
func lexTime(e *dclExpression, _ []lexicalArg) (dclValue, error) {
	return dclString(timeText(e.console.systemTime())), nil
}

// systemTime is the system time now, in VMS's format: the process's
// clock, or the engine's before there is a process, or the host's before
// there is a machine.
func (c *Console) systemTime() uint64 {
	switch {
	case c.RTL != nil && c.RTL.Clock != nil:
		return c.RTL.Clock()
	case c.Engine != nil:
		return c.Engine.SystemTime()
	default:
		return vmsdef.Time(time.Now())
	}
}

// Time-unit sizes, in VMS's 100-nanosecond ticks.
const (
	ticksPerHundredth = vmsdef.TicksPerSecond / 100
	ticksPerDay       = 86400 * vmsdef.TicksPerSecond
)

// cvtimeFormats are F$CVTIME's output formats.
type cvtimeFormat int

const (
	cvtComparison cvtimeFormat = iota
	cvtAbsolute
	cvtDelta
)

var cvtimeFormats = map[string]cvtimeFormat{
	"COMPARISON": cvtComparison,
	"ABSOLUTE":   cvtAbsolute,
	"DELTA":      cvtDelta,
}

// cvtimeField is one of F$CVTIME's output fields, written in each format:
// comparison (fixed width, zero-filled, so that strings compare as the
// times do), absolute, and delta (nil for a field a delta time doesn't
// have: CLI_IVKEYW).
type cvtimeField struct {
	comparison func(t time.Time) string
	absolute   func(t time.Time) string
	delta      func(d deltaTime) string
}

// deltaTime is a delta time taken apart.
type deltaTime struct {
	days, hour, minute, second, hundredth int
}

// clock writes hours, minutes, seconds, and hundredths, "14:23:45.12".
func clock(hour, minute, second, hundredth int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%02d", hour, minute, second, hundredth)
}

// timeClock is t's time of day, as clock writes it.
func timeClock(t time.Time) string {
	return clock(t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/10_000_000)
}

// yearHours, yearMinutes, and yearSeconds count from the start of t's
// year.
func yearHours(t time.Time) int   { return (t.YearDay()-1)*24 + t.Hour() }
func yearMinutes(t time.Time) int { return yearHours(t)*60 + t.Minute() }
func yearSeconds(t time.Time) int { return yearMinutes(t)*60 + t.Second() }

// zeros writes n zero-filled to width digits; plain writes it as it is.
func zeros(n, width int) string { return fmt.Sprintf("%0*d", width, n) }
func plain(n int) string        { return strconv.Itoa(n) }

// cvtimeFields are F$CVTIME's output fields (unconfirmed against VMS: the
// widths of DAYOFYEAR and the other OFYEAR fields in comparison format,
// the absolute DAY's lack of a leading blank, and a delta's DATETIME).
var cvtimeFields = map[string]cvtimeField{
	"DATETIME": {
		comparison: func(t time.Time) string { return t.Format("2006-01-02") + " " + timeClock(t) },
		absolute:   func(t time.Time) string { return absoluteDate(t) + " " + timeClock(t) },
		delta: func(d deltaTime) string {
			return plain(d.days) + " " + clock(d.hour, d.minute, d.second, d.hundredth)
		},
	},
	"DATE": {
		comparison: func(t time.Time) string { return t.Format("2006-01-02") },
		absolute:   absoluteDate,
	},
	"TIME": {
		comparison: timeClock,
		absolute:   timeClock,
		delta:      func(d deltaTime) string { return clock(d.hour, d.minute, d.second, d.hundredth) },
	},
	"YEAR": {
		comparison: func(t time.Time) string { return zeros(t.Year(), 4) },
		absolute:   func(t time.Time) string { return plain(t.Year()) },
	},
	"MONTH": {
		comparison: func(t time.Time) string { return zeros(int(t.Month()), 2) },
		absolute:   func(t time.Time) string { return monthAbbreviation(t) },
	},
	"DAY": {
		comparison: func(t time.Time) string { return zeros(t.Day(), 2) },
		absolute:   func(t time.Time) string { return plain(t.Day()) },
		delta:      func(d deltaTime) string { return plain(d.days) },
	},
	"HOUR": {
		comparison: func(t time.Time) string { return zeros(t.Hour(), 2) },
		absolute:   func(t time.Time) string { return zeros(t.Hour(), 2) },
		delta:      func(d deltaTime) string { return zeros(d.hour, 2) },
	},
	"MINUTE": {
		comparison: func(t time.Time) string { return zeros(t.Minute(), 2) },
		absolute:   func(t time.Time) string { return zeros(t.Minute(), 2) },
		delta:      func(d deltaTime) string { return zeros(d.minute, 2) },
	},
	"SECOND": {
		comparison: func(t time.Time) string { return zeros(t.Second(), 2) },
		absolute:   func(t time.Time) string { return zeros(t.Second(), 2) },
		delta:      func(d deltaTime) string { return zeros(d.second, 2) },
	},
	"HUNDREDTH": {
		comparison: func(t time.Time) string { return zeros(t.Nanosecond()/10_000_000, 2) },
		absolute:   func(t time.Time) string { return zeros(t.Nanosecond()/10_000_000, 2) },
		delta:      func(d deltaTime) string { return zeros(d.hundredth, 2) },
	},
	"WEEKDAY": {
		comparison: func(t time.Time) string { return t.Weekday().String() },
		absolute:   func(t time.Time) string { return t.Weekday().String() },
	},
	"DAYOFYEAR": {
		comparison: func(t time.Time) string { return zeros(t.YearDay(), 3) },
		absolute:   func(t time.Time) string { return plain(t.YearDay()) },
	},
	"HOUROFYEAR": {
		comparison: func(t time.Time) string { return zeros(yearHours(t), 4) },
		absolute:   func(t time.Time) string { return plain(yearHours(t)) },
	},
	"MINUTEOFYEAR": {
		comparison: func(t time.Time) string { return zeros(yearMinutes(t), 6) },
		absolute:   func(t time.Time) string { return plain(yearMinutes(t)) },
	},
	"SECONDOFYEAR": {
		comparison: func(t time.Time) string { return zeros(yearSeconds(t), 8) },
		absolute:   func(t time.Time) string { return plain(yearSeconds(t)) },
	},
}

// monthNames are the months as VMS writes them in a time.
var monthNames = [12]string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}

// monthAbbreviation is t's month, "OCT".
func monthAbbreviation(t time.Time) string { return monthNames[t.Month()-1] }

// absoluteDate is t's date in absolute format, "9-OCT-2026".
func absoluteDate(t time.Time) string {
	return fmt.Sprintf("%d-%s-%d", t.Day(), monthAbbreviation(t), t.Year())
}

// lexCVTime is F$CVTIME([time] [,format] [,field]): the time (the current
// one if it's left out or ""), in the format (COMPARISON if not given)
// and the field (DATETIME if not given). With the DELTA format, the time
// must be a delta time; otherwise it is an absolute or a combination
// time. A time that can't be read is CLI_IVATIME or CLI_IVDTIME.
func lexCVTime(e *dclExpression, args []lexicalArg) (dclValue, error) {
	format := cvtComparison

	if args[1].present {
		f, _, err := lexicalKeyword(cvtimeFormats, args[1])
		if err != nil {
			return dclValue{}, err
		}

		format = f
	}

	field := cvtimeFields["DATETIME"]

	if args[2].present {
		f, key, err := lexicalKeyword(cvtimeFields, args[2])
		if err != nil {
			return dclValue{}, err
		}

		if format == cvtDelta && f.delta == nil {
			return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
		}

		field = f
	}

	text := strings.ToUpper(strings.TrimSpace(args[0].str()))

	if format == cvtDelta {
		ticks, ok := parseDeltaTime(text)
		if !ok {
			return dclValue{}, vmserrors.New(vmserrors.CLI_IVDTIME)
		}

		return dclString(field.delta(splitDelta(ticks))), nil
	}

	now := vmsdef.GoTime(e.console.systemTime())

	t, ok := parseCombinationTime(text, now)
	if !ok {
		return dclValue{}, vmserrors.New(vmserrors.CLI_IVATIME)
	}

	if format == cvtAbsolute {
		return dclString(field.absolute(t)), nil
	}

	return dclString(field.comparison(t)), nil
}

// parseCombinationTime reads text, uppercase, as an absolute time, or a
// combination of one and a delta time added or taken away; "" is now. A
// "-" can be the date's own or a delta's sign, so each place one could
// split the two is tried, from the left, until both halves read.
func parseCombinationTime(text string, now time.Time) (time.Time, bool) {
	if text == "" {
		return now, true
	}

	if t, ok := parseAbsoluteTime(text, now); ok {
		return t, true
	}

	for i := 1; i < len(text); i++ {
		if text[i] != '+' && text[i] != '-' {
			continue
		}

		t, ok := parseAbsoluteTime(text[:i], now)
		if !ok {
			continue
		}

		ticks, ok := parseDeltaTime(text[i+1:])
		if !ok {
			continue
		}

		d := time.Duration(ticks) * 100 // ticks are 100 nanoseconds
		if text[i] == '-' {
			d = -d
		}

		return t.Add(d), true
	}

	return time.Time{}, false
}

// relativeDays are the absolute times DCL names with a word: midnight of
// today, tomorrow, and yesterday.
var relativeDays = map[string]int{"TODAY": 0, "TOMORROW": 1, "YESTERDAY": -1}

// parseAbsoluteTime reads text, uppercase, as an absolute time: a date
// (dd-mmm-yyyy, or one of relativeDays) and a time (hh:mm:ss.cc), either
// one optional, with ":" or a blank between them. A date's missing fields
// are now's, a time's are 0.
func parseAbsoluteTime(text string, now time.Time) (time.Time, bool) {
	date, clockText := "", text

	if strings.Contains(text, "-") || startsWithDayWord(text) {
		date, clockText = text, ""

		if i := strings.IndexAny(text, ": "); i >= 0 {
			date, clockText = text[:i], text[i+1:]
		}
	}

	year, month, day := now.Date()

	switch offset, word := relativeDays[date]; {
	case word:
		year, month, day = now.AddDate(0, 0, offset).Date()
	case date != "":
		var ok bool
		if year, month, day, ok = parseDate(date, year, month, day); !ok {
			return time.Time{}, false
		}
	}

	ticks, ok := parseClock(clockText)
	if !ok {
		return time.Time{}, false
	}

	midnight := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if midnight.Day() != day { // a day the month doesn't have
		return time.Time{}, false
	}

	return midnight.Add(time.Duration(ticks) * 100), true
}

// startsWithDayWord reports whether text is one of relativeDays, alone or
// before a time.
func startsWithDayWord(text string) bool {
	word, _, _ := strings.Cut(text, ":")

	_, ok := relativeDays[word]

	return ok
}

// parseDate reads dd-mmm-yyyy, each field optional (an empty one is the
// one given), the month as VMS abbreviates it.
func parseDate(text string, year int, month time.Month, day int) (int, time.Month, int, bool) {
	fields := strings.Split(text, "-")
	if len(fields) > 3 {
		return 0, 0, 0, false
	}

	if f := fields[0]; f != "" {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 || n > 31 {
			return 0, 0, 0, false
		}

		day = n
	}

	if len(fields) > 1 && fields[1] != "" {
		m := monthIndex(fields[1])
		if m == 0 {
			return 0, 0, 0, false
		}

		month = m
	}

	if len(fields) > 2 && fields[2] != "" {
		n, err := strconv.Atoi(fields[2])
		if err != nil || n < 1858 || n > 9999 {
			return 0, 0, 0, false
		}

		year = n
	}

	return year, month, day, true
}

// monthIndex is the month VMS's abbreviation name stands for, 0 for
// none.
func monthIndex(name string) time.Month {
	for i, m := range monthNames {
		if m == name {
			return time.Month(i + 1)
		}
	}

	return 0
}

// parseClock reads a time of day or a delta's time, hh:mm:ss.cc, as
// ticks: fields left out at the end are 0, and so is an empty one. The
// fraction after the seconds is a true fraction: ".5" is 50 hundredths.
func parseClock(text string) (int64, bool) {
	if text == "" {
		return 0, true
	}

	seconds, fraction, _ := strings.Cut(text, ".")

	fields := strings.Split(seconds, ":")
	if len(fields) > 3 {
		return 0, false
	}

	limits := []int64{23, 59, 59}
	units := []int64{3600, 60, 1}

	var ticks int64

	for i, f := range fields {
		if f == "" {
			continue
		}

		n, err := strconv.ParseInt(f, 10, 64)
		if err != nil || n < 0 || n > limits[i] {
			return 0, false
		}

		ticks += n * units[i] * vmsdef.TicksPerSecond
	}

	if fraction != "" {
		if len(fields) < 3 {
			return 0, false
		}

		digits := (fraction + "00")[:2]

		n, err := strconv.Atoi(digits)
		if err != nil {
			return 0, false
		}

		ticks += int64(n) * ticksPerHundredth
	}

	return ticks, true
}

// parseDeltaTime reads text, uppercase, as a delta time, [dddd-][time],
// as ticks. A day count is under 10,000.
func parseDeltaTime(text string) (int64, bool) {
	days, clockText := "", text
	if d, rest, ok := strings.Cut(text, "-"); ok {
		days, clockText = d, rest
	}

	var ticks int64

	if days != "" {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 || n > 9999 {
			return 0, false
		}

		ticks = int64(n) * ticksPerDay
	}

	t, ok := parseClock(clockText)
	if !ok {
		return 0, false
	}

	return ticks + t, true
}

// splitDelta takes a delta time, in ticks, apart.
func splitDelta(ticks int64) deltaTime {
	secs := ticks / vmsdef.TicksPerSecond

	return deltaTime{
		days:      int(ticks / ticksPerDay),
		hour:      int(secs / 3600 % 24),
		minute:    int(secs / 60 % 60),
		second:    int(secs % 60),
		hundredth: int(ticks % vmsdef.TicksPerSecond / ticksPerHundredth),
	}
}

// timeText writes a VMS time as $ASCTIM does (" 9-OCT-2026 14:23:45.12"),
// "" for one too far away.
func timeText(v uint64) string {
	s, _ := corevms.FormatTime(v, false)

	return s
}
