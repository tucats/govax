package rtl

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// The VMS time services (docs/PHASE-26.md subtasks 12, 13, and 20):
// $GETTIM reads the system clock, $ASCTIM/$BINTIM convert between binary
// times and the text forms VMS prints and accepts, and $NUMTIM breaks a
// binary time into numbers.
//
// A VMS time is a signed 64-bit count of 100-nanosecond "ticks":
//
//   - A positive value is an *absolute* time: ticks since the VMS base
//     date, 17-Nov-1858 00:00, in local time (see vmsdef.Time).
//   - A negative value is a *delta* time: an interval, such as "wait
//     10 seconds", stored negated so services like $SETIMR can tell the
//     two kinds apart by the sign bit alone.
//
// The VAX has no 64-bit integer registers, so these values always travel
// by reference: a service argument is the address of a "quadword" (two
// longwords, low-order one first) holding the time.

// loadQuad reads the quadword at addr, low-order longword first.
// ok is false if either longword can't be read.
func (env *Environment) loadQuad(addr uint32) (v uint64, ok bool) {
	lo, err := env.mem.LoadLongword(env.cpu, addr)
	if err != nil {
		return 0, false
	}

	hi, err := env.mem.LoadLongword(env.cpu, addr+4)
	if err != nil {
		return 0, false
	}

	return uint64(hi)<<32 | uint64(lo), true
}

// storeQuad writes v to the quadword at addr, low-order longword first.
// ok is false if either longword can't be written.
func (env *Environment) storeQuad(addr uint32, v uint64) (ok bool) {
	if err := env.mem.StoreLongword(env.cpu, addr, uint32(v)); err != nil {
		return false
	}

	return env.mem.StoreLongword(env.cpu, addr+4, uint32(v>>32)) == nil
}

// serviceSysGettim is SYS$GETTIM:
//
//	SYS$GETTIM timadr
//
// It stores the current system time (Environment.Clock, the same clock
// $SETIMR's timers run on) in the quadword at timadr. timadr is
// required; 0 or an unwritable address is SS$_ACCVIO.
//
// VMS updates its clock every 10ms, so real VMS returns a multiple of
// 100,000 ticks. govax's clock advances 1ms per interval-clock tick and
// returns it unrounded: a program can't depend on the 10ms granularity,
// and a finer clock only makes elapsed-time measurements more accurate.
func serviceSysGettim(env *Environment, argv []uint32) (uint32, error) {
	timadr := optArg(argv, 0)
	if timadr == 0 { // page 0 is never accessible on VMS
		return ssAccVio, nil
	}

	if !env.storeQuad(timadr, env.Clock()) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

// ssIvTime is SS$_IVTIME: a time string that doesn't parse, a field out
// of range, or a delta time of 10,000 days or more.
var ssIvTime = vmsdef.Symbols["SS$_IVTIME"]

// Time-unit sizes in VMS ticks (100ns).
const (
	ticksPerHundredth = vmsdef.TicksPerSecond / 100
	ticksPerDay       = 86400 * vmsdef.TicksPerSecond

	// maxDeltaDays: a delta time must be under 10,000 days, so its day
	// count fits the four-character "dddd" field.
	maxDeltaDays = 10_000
)

// monthNames are the month abbreviations VMS time strings use. They're
// always upper case: $BINTIM rejects "Dec".
var monthNames = [12]string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}

// formatVMSTime is $ASCTIM's conversion: the text form of the VMS time v.
//
//	absolute: "dd-mmm-yyyy hh:mm:ss.cc"  (23 characters, " 9-OCT-1988 ...")
//	delta:    "dddd hh:mm:ss.cc"         (16 characters, "   5 03:18:32.07")
//
// The day of the month is padded to two characters and a delta's day
// count to four, with blanks, so each form has a fixed length. With
// timeOnly (cvtflg 1) only "hh:mm:ss.cc" is returned. The hundredths are
// truncated, not rounded. ok is false for a delta of 10,000 days or more,
// or an absolute time past the year 9999, which don't fit the fields.
func formatVMSTime(v uint64, timeOnly bool) (s string, ok bool) {
	// The time of day is the same arithmetic for both kinds: ticks into
	// the current day, split into hours, minutes, seconds, hundredths.
	clock := func(ticks uint64) string {
		ticks %= ticksPerDay
		secs := ticks / vmsdef.TicksPerSecond

		return fmt.Sprintf("%02d:%02d:%02d.%02d",
			secs/3600, secs/60%60, secs%60, ticks%vmsdef.TicksPerSecond/ticksPerHundredth)
	}

	if int64(v) < 0 { // a delta time: stored negated
		ticks := uint64(-int64(v))

		days := ticks / ticksPerDay
		if days >= maxDeltaDays {
			return "", false
		}

		if timeOnly {
			return clock(ticks), true
		}

		return fmt.Sprintf("%4d %s", days, clock(ticks)), true
	}

	if timeOnly {
		return clock(v), true
	}

	t := vmsdef.GoTime(v)
	if t.Year() > 9999 {
		return "", false
	}

	return fmt.Sprintf("%2d-%s-%04d %s", t.Day(), monthNames[t.Month()-1], t.Year(), clock(v)), true
}

// parseVMSTime is $BINTIM's conversion: the VMS time a text string
// stands for, or ok false for SS$_IVTIME. now supplies the omitted fields
// of an absolute time.
//
// The two forms are the ones formatVMSTime produces, but almost any field
// can be left out (the manual's rules, with its examples, as they would
// run at 30-DEC-1988 04:15:28.00):
//
//   - Leading blanks, and any number of blanks between the date (or day
//     count) and the time, are allowed. There can be no blanks *within*
//     a field.
//   - A string whose first field contains a hyphen is absolute. An
//     omitted date or time field takes its value from now, so
//     "-- :50" is 30-DEC-1988 04:50:28.00. Punctuation before a field that
//     is given must still be there; trailing fields can be dropped.
//   - Anything else is a delta. With two fields, the first is the day
//     count; with one, it's the time ("05" is five hours). Omitted time
//     fields are 0.
//   - The fraction after the seconds is a true fraction (".1" is ten
//     hundredths); a third digit rounds the hundredths, and later
//     digits are ignored.
//
// The result for a delta is negated, so its sign marks it as a delta.
func parseVMSTime(s string, now uint64) (v uint64, ok bool) {
	fields := strings.Fields(s)
	if len(fields) == 0 || len(fields) > 2 {
		return 0, false
	}

	if strings.Contains(fields[0], "-") {
		return parseAbsoluteTime(fields, now)
	}

	days, timeField := uint64(0), fields[0]

	if len(fields) == 2 {
		n, ok := parseTimeNumber(fields[0], 4, maxDeltaDays-1)
		if !ok {
			return 0, false
		}

		days, timeField = n, fields[1]
	}

	ticks, ok := parseTimeOfDay(timeField, [4]uint64{})
	if !ok {
		return 0, false
	}

	ticks += days * ticksPerDay
	if ticks/ticksPerDay >= maxDeltaDays { // rounding the fraction can carry this far
		return 0, false
	}

	return uint64(-int64(ticks)), true
}

// parseAbsoluteTime is parseVMSTime for an absolute time: fields[0] is
// the date, "dd-mmm-yyyy", and fields[1], if there, the time of day.
func parseAbsoluteTime(fields []string, now uint64) (uint64, bool) {
	cur := vmsdef.GoTime(now)
	day, month, year := cur.Day(), int(cur.Month()), cur.Year()

	parts := strings.Split(fields[0], "-")
	if len(parts) > 3 {
		return 0, false
	}

	if parts[0] != "" {
		n, ok := parseTimeNumber(parts[0], 2, 31)
		if !ok || n == 0 {
			return 0, false
		}

		day = int(n)
	}

	if len(parts) > 1 && parts[1] != "" {
		month = 0

		for i, name := range monthNames {
			if parts[1] == name {
				month = i + 1
			}
		}

		if month == 0 {
			return 0, false
		}
	}

	if len(parts) > 2 && parts[2] != "" {
		n, ok := parseTimeNumber(parts[2], 4, 9999)
		if !ok || n < 1858 {
			return 0, false
		}

		year = int(n)
	}

	// Go's time.Date would quietly turn 31-FEB into 3-MAR; VMS rejects it.
	// Day 0 of the next month is the last day of this one.
	if day > time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return 0, false
	}

	// Today's time of day fills in any time field left out.
	secs := now % ticksPerDay / vmsdef.TicksPerSecond
	current := [4]uint64{secs / 3600, secs / 60 % 60, secs % 60, now % vmsdef.TicksPerSecond / ticksPerHundredth}

	timeField := ""
	if len(fields) == 2 {
		timeField = fields[1]
	}

	ticks, ok := parseTimeOfDay(timeField, current)
	if !ok {
		return 0, false
	}

	midnight := vmsdef.Time(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC))
	if year == 1858 && midnight > 1<<62 { // before 17-NOV-1858: wrapped negative
		return 0, false
	}

	return midnight + ticks, true
}

// parseTimeOfDay reads "hh:mm:ss.cc" as ticks since midnight. Each field
// may be empty (or missing from the end), and then its value comes from
// defaults: hours, minutes, seconds, hundredths.
func parseTimeOfDay(s string, defaults [4]uint64) (uint64, bool) {
	hms, frac, hasFrac := strings.Cut(s, ".")

	parts := strings.Split(hms, ":")
	if len(parts) > 3 || (hasFrac && len(parts) != 3) {
		return 0, false // a fraction belongs to the seconds field
	}

	limits := [3]uint64{23, 59, 59}
	fields := defaults

	for i, p := range parts {
		if p == "" {
			continue
		}

		n, ok := parseTimeNumber(p, 2, limits[i])
		if !ok {
			return 0, false
		}

		fields[i] = n
	}

	if hasFrac && frac != "" {
		// Milliseconds from the first three digits (".1" is 100ms), then
		// rounded to hundredths: the third digit only rounds.
		digits := (frac + "00")[:3]

		ms, ok := parseTimeNumber(digits, 3, 999)
		if !ok || strings.Trim(frac, "0123456789") != "" {
			return 0, false
		}

		fields[3] = (ms + 5) / 10 // 100 carries into the seconds below
	}

	secs := fields[0]*3600 + fields[1]*60 + fields[2]

	return secs*vmsdef.TicksPerSecond + fields[3]*ticksPerHundredth, true
}

// parseTimeNumber reads a field of 1 to width decimal digits whose value
// is at most limit.
func parseTimeNumber(s string, width int, limit uint64) (uint64, bool) {
	if s == "" || len(s) > width || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}

	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n > limit {
		return 0, false
	}

	return n, true
}

// serviceSysAsctim is SYS$ASCTIM:
//
//	SYS$ASCTIM [timlen] ,timbuf ,[timadr] ,[cvtflg]
//
// It writes the text form of the quadword time at timadr (the current
// time when timadr is 0) into the buffer the timbuf descriptor describes,
// and its length to the word at timlen. cvtflg bit 0 asks for the time of
// day only. See formatVMSTime for the forms.
//
// A buffer shorter than the text gets as much as fits and the status is
// SS$_BUFFEROVF, a success. That's deliberate VMS usage: a 12-byte buffer
// gets just the date ("30-DEC-1988 "). A delta of 10,000 days or more
// (or an absolute time past 9999) is SS$_IVTIME. The manual says bad
// addresses raise an access violation; govax returns SS$_ACCVIO, as its
// other services do.
func serviceSysAsctim(env *Environment, argv []uint32) (uint32, error) {
	timlen, timbuf, timadr := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2)
	timeOnly := optArg(argv, 3)&1 != 0

	v := env.Clock()

	if timadr != 0 {
		q, ok := env.loadQuad(timadr)
		if !ok {
			return ssAccVio, nil
		}

		v = q
	}

	text, ok := formatVMSTime(v, timeOnly)
	if !ok {
		return ssIvTime, nil
	}

	if timbuf == 0 {
		return ssAccVio, nil
	}

	n, truncated, err := storeDescriptor(env, timbuf, text)
	if err != nil {
		return ssAccVio, nil
	}

	if timlen != 0 {
		if err := env.mem.StoreWord(env.cpu, timlen, n); err != nil {
			return ssAccVio, nil
		}
	}

	if truncated {
		return ssBufferOvf, nil
	}

	return ssNormal, nil
}

// maxTimeString bounds the timbuf string $BINTIM reads. The longest valid
// string is 23 characters plus blanks between the fields; this leaves
// generous room for those while refusing a runaway descriptor.
const maxTimeString = 256

// serviceSysBintim is SYS$BINTIM:
//
//	SYS$BINTIM timbuf ,timadr
//
// It converts the text time the timbuf descriptor describes to a VMS
// binary time, stored in the quadword at timadr. See parseVMSTime for
// what it accepts; anything else is SS$_IVTIME. Unreadable or unwritable
// arguments are SS$_ACCVIO.
func serviceSysBintim(env *Environment, argv []uint32) (uint32, error) {
	timbuf, timadr := optArg(argv, 0), optArg(argv, 1)
	if timbuf == 0 || timadr == 0 {
		return ssAccVio, nil
	}

	text, fits, err := strGet(env, timbuf, maxTimeString)
	if err != nil {
		return ssAccVio, nil
	}

	if !fits {
		return ssIvTime, nil
	}

	v, ok := parseVMSTime(text, env.Clock())
	if !ok {
		return ssIvTime, nil
	}

	if !env.storeQuad(timadr, v) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

// numericTime is $NUMTIM's conversion: the seven numbers a VMS time
// breaks down into, in the order $NUMTIM stores them — year, month, day
// of the month, hour, minute, second, hundredths.
//
// For a delta time, the year and month are 0 and the "day" is the number
// of whole days in the interval; ok is false for 10,000 days or more,
// which $NUMTIM rejects (SS$_IVTIME). The hundredths are truncated, as
// $ASCTIM's are.
func numericTime(v uint64) (fields [7]uint16, ok bool) {
	ticks := v

	if int64(v) < 0 { // a delta time: stored negated
		ticks = uint64(-int64(v))

		days := ticks / ticksPerDay
		if days >= maxDeltaDays {
			return fields, false
		}

		fields[2] = uint16(days)
	} else {
		t := vmsdef.GoTime(v)
		fields[0], fields[1], fields[2] = uint16(t.Year()), uint16(t.Month()), uint16(t.Day())
	}

	// The time of day: ticks into the current day.
	ticks %= ticksPerDay
	secs := ticks / vmsdef.TicksPerSecond

	fields[3] = uint16(secs / 3600)
	fields[4] = uint16(secs / 60 % 60)
	fields[5] = uint16(secs % 60)
	fields[6] = uint16(ticks % vmsdef.TicksPerSecond / ticksPerHundredth)

	return fields, true
}

// serviceSysNumtim is SYS$NUMTIM:
//
//	SYS$NUMTIM timbuf ,[timadr]
//
// It breaks the time in the quadword at timadr (the current time if
// timadr is omitted) into seven words, stored at timbuf: year, month,
// day, hour, minute, second, and hundredths (see numericTime). A time
// of 0 is the base date, 17-NOV-1858 00:00:00.00. A delta of 10,000 days
// or more is SS$_IVTIME; an unreadable time or an unwritable timbuf
// (including 0) is SS$_ACCVIO.
func serviceSysNumtim(env *Environment, argv []uint32) (uint32, error) {
	timbuf, timadr := optArg(argv, 0), optArg(argv, 1)

	v := env.Clock()

	if timadr != 0 {
		q, ok := env.loadQuad(timadr)
		if !ok {
			return ssAccVio, nil
		}

		v = q
	}

	fields, ok := numericTime(v)
	if !ok {
		return ssIvTime, nil
	}

	const size = len(fields) * 2

	if timbuf == 0 || !env.accessible(timbuf, uint32(size), vm.AccessWrite) {
		return ssAccVio, nil
	}

	var data [size]byte
	for i, f := range fields {
		data[2*i], data[2*i+1] = byte(f), byte(f>>8)
	}

	if err := env.mem.Store(env.cpu, timbuf, data[:]); err != nil {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

func registerTimeServices(t *ServiceTable) {
	t.Register("SYS$GETTIM", serviceSysGettim)
	t.Register("SYS$ASCTIM", serviceSysAsctim)
	t.Register("SYS$BINTIM", serviceSysBintim)
	t.Register("SYS$NUMTIM", serviceSysNumtim)
}
