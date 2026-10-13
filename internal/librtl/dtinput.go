package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The date and time input routine (docs/PHASE-56, subtask 7; the survey's
// "Input"): LIB$CONVERT_DATE_STRING, which reads a date and time written
// by the context's input format and makes it a VMS internal time.
//
// The input format is an outline the string is matched against (the 5.0
// manual's section 3.3.2.1, "Punctuation"): each mnemonic is a field to
// read, a run of blanks in the format is synchronizing whitespace (any
// run of blanks and tabs matches it), and any other punctuation must
// appear exactly (case aside). A field may be omitted, its punctuation
// written without its value ("-MAR" leaves out the day and the year),
// and then takes a default: the defaults vector's, else today's date for
// the date fields and 0 for the time fields.
//
// The manual's Table 3-2 shows how far this goes: with the standard
// format, "23:" is 23:00 today, "::23" is 00:00:23, and "-1988" is today's
// day and month in 1988. govax reads it this way (unconfirmed): the
// string matches one contiguous run of the format's items, from any item
// to any item, every field in the run present or omitted, and everything
// before and after the run omitted. Within the run, a field may also be
// left out with the punctuation before it ("4:39 pm" for
// "!H02:!M0:!S0.!C2 !MIU"). The reading that leaves out the least
// punctuation wins, then the earliest run: "23" alone is a day, and "23:"
// (whose colon only the time has) an hour.

var (
	libAmbDatTim = vmsdef.LibrarySymbols["LIB$_AMBDATTIM"]
	libIncDatTim = vmsdef.LibrarySymbols["LIB$_INCDATTIM"]
)

// The fields of a date and time in $NUMTIM's order, which is the order of
// the defaults vector and of the bits of the flags and defaulted-fields
// masks (LIB$M_YEAR is bit 0, LIB$M_HUNDREDTHS bit 6).
const (
	numYear = iota
	numMonth
	numDay
	numHour
	numMinute
	numSecond
	numFraction
	numFields
)

// defaultOmittable is the flags' default, 0x78: the time's fields may be
// omitted, the date's may not.
const defaultOmittable = 0x78

// numFieldOf is the $NUMTIM field a mnemonic's field fills, or -1 for the
// meridiem, which only adjusts the hour (and a weekday, which an input
// format can't have).
func numFieldOf(f dtField) int {
	switch f {
	case fieldYear:
		return numYear
	case fieldMonthName, fieldMonthNumber:
		return numMonth
	case fieldDay:
		return numDay
	case fieldHour24, fieldHour12:
		return numHour
	case fieldMinute:
		return numMinute
	case fieldSecond:
		return numSecond
	case fieldFraction:
		return numFraction
	}

	return -1
}

// dtReading is what a match has read so far: each field's value and
// whether it was given, and the meridiem (-1 when none was read, else 0
// for the first indicator, 1 for the second). A fraction's value is in
// ticks of 100 ns, already scaled by its digits.
type dtReading struct {
	value    [numFields]int
	given    [numFields]bool
	meridiem int
}

// dtMatcher matches one input string against an input format's items.
// ambiguous is set when a word matched more than one spelling (an
// abbreviation such as "JU"), which, if nothing matches, is the reason.
type dtMatcher struct {
	s         *dtSettings
	items     []dtItem
	input     string
	ambiguous bool
}

// isBlank reports whether c is whitespace: a blank or a tab.
func isBlank(c byte) bool { return c == ' ' || c == '\t' }

// isSynchronizing reports whether a punctuation item is whitespace only,
// which matches any run of blanks and tabs rather than itself.
func isSynchronizing(literal string) bool {
	return strings.Trim(literal, " \t") == ""
}

// isNameByte reports whether c can be part of a spelled word: an ASCII
// letter, or a byte of the DEC Multinational set's upper half, whose
// letters other languages' names use.
func isNameByte(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || c >= 0xC0
}

// match tries to match the input from pos against the items from i on,
// with r what has been read before, leaving out at most budget
// punctuation items. The input may end anywhere: what's left of the
// format is omitted. It returns the completed reading.
//
// A punctuation item must be in the input, unless the field after it is
// omitted too: then both may be left out, which spends one of the
// budget. So "4:39 pm" can be read by "!H02:!M0:!S0.!C2 !MIU", the
// seconds and the fraction left out with their colon and period.
func (m *dtMatcher) match(i, pos int, r dtReading, budget int) (dtReading, bool) {
	if pos == len(m.input) {
		return r, true
	}

	if i == len(m.items) {
		return r, false
	}

	it := m.items[i]

	if it.m == nil {
		if n := m.punctuation(it.literal, pos); n > 0 {
			if out, ok := m.match(i+1, pos+n, r, budget); ok {
				return out, true
			}
		}

		if budget > 0 && i+1 < len(m.items) && m.items[i+1].m != nil {
			return m.match(i+2, pos, r, budget-1)
		}

		return r, false
	}

	// Each way the field can be read here, longest first, and last the
	// field omitted.
	for _, c := range m.candidates(it.m, pos) {
		next := r
		if f := numFieldOf(it.m.field); f >= 0 {
			next.value[f], next.given[f] = c.value, true
		} else {
			next.meridiem = c.value
		}

		if out, ok := m.match(i+1, pos+c.length, next, budget); ok {
			return out, true
		}
	}

	return m.match(i+1, pos, r, budget)
}

// punctuation is how many bytes of the input at pos a punctuation item
// matches, 0 when it doesn't: one or more blanks and tabs for
// synchronizing whitespace, else the item's own text, case aside.
func (m *dtMatcher) punctuation(literal string, pos int) int {
	if isSynchronizing(literal) {
		n := 0
		for pos+n < len(m.input) && isBlank(m.input[pos+n]) {
			n++
		}

		return n
	}

	if len(m.input)-pos >= len(literal) && strings.EqualFold(m.input[pos:pos+len(literal)], literal) {
		return len(literal)
	}

	return 0
}

// dtCandidate is one way to read a field: its value and the bytes it
// takes.
type dtCandidate struct {
	value, length int
}

// candidates is the ways the field m can be read from the input at pos,
// longest first.
func (m *dtMatcher) candidates(mn *dtMnemonic, pos int) []dtCandidate {
	switch mn.field {
	case fieldMonthName:
		full := m.s.spellings[componentMonthName][caseUpper]
		abbreviated := m.s.spellings[componentMonthAbb][caseUpper]

		return m.word(pos, full, abbreviated, 1)
	case fieldMeridiem:
		return m.word(pos, m.s.spellings[componentMeridiem][caseUpper], nil, 0)
	}

	return m.number(mn, pos)
}

// number is the ways a numeric field can be read at pos: one digit up to
// the field's most (two, or a year's or fraction's own digits), longest
// first, so that fields with no punctuation between them ("!Y2!MN0!D0")
// share the digits. A blank-filled field may begin with blanks. A
// fraction's digits are a decimal fraction of a second (".1" is a tenth),
// as $BINTIM reads them, made ticks.
func (m *dtMatcher) number(mn *dtMnemonic, pos int) []dtCandidate {
	start := pos
	if mn.fill == fillBlank {
		for start < len(m.input) && isBlank(m.input[start]) {
			start++
		}
	}

	most := 2
	if mn.field == fieldYear || mn.field == fieldFraction {
		most = mn.digits
	}

	digits := 0
	for digits < most && start+digits < len(m.input) && '0' <= m.input[start+digits] && m.input[start+digits] <= '9' {
		digits++
	}

	var out []dtCandidate

	for n := digits; n >= 1; n-- {
		v := 0
		for _, c := range m.input[start : start+n] {
			v = v*10 + int(c-'0')
		}

		if mn.field == fieldFraction {
			v *= tenTo(7 - n)
		}

		out = append(out, dtCandidate{value: v, length: start - pos + n})
	}

	return out
}

// word is the ways a spelled field can be read at pos, matched against
// spellings (and abbreviations, the month's abbreviated names) without
// regard to case. Its values are the index into spellings plus base (the
// month number counts from 1). A spelling written out in full matches,
// the longest first; otherwise the word at pos (its letters) may be an
// abbreviation of one spelling. An abbreviation of more than one
// ("JU", JUNE and JULY) matches none, and marks the matcher ambiguous.
func (m *dtMatcher) word(pos int, spellings, abbreviations []string, base int) []dtCandidate {
	rest := m.input[pos:]

	best := -1
	length := 0

	for _, list := range [][]string{spellings, abbreviations} {
		for i, sp := range list {
			if sp != "" && len(sp) > length && len(rest) >= len(sp) && strings.EqualFold(rest[:len(sp)], sp) {
				best, length = i, len(sp)
			}
		}
	}

	if best >= 0 {
		return []dtCandidate{{value: best + base, length: length}}
	}

	n := 0
	for n < len(rest) && isNameByte(rest[n]) {
		n++
	}

	if n == 0 {
		return nil
	}

	found := -1

	for i, sp := range spellings {
		if len(sp) >= n && strings.EqualFold(sp[:n], rest[:n]) {
			if found >= 0 && found != i {
				m.ambiguous = true

				return nil
			}

			found = i
		}
	}

	if found < 0 {
		return nil
	}

	return []dtCandidate{{value: found + base, length: n}}
}

// relativeDay is the day a relative day name gives (the language's
// YESTERDAY, TODAY, or TOMORROW, as its spellings have them), case aside,
// written in full or abbreviated: -1, 0, or 1. ok is false when text is
// no such name; ambiguous when it abbreviates more than one ("T").
func (s *dtSettings) relativeDay(text string) (offset int, ok, ambiguous bool) {
	found := -1

	for i, sp := range s.spellings[componentRelativeDay][caseUpper] {
		if text == "" || len(sp) < len(text) || !strings.EqualFold(sp[:len(text)], text) {
			continue
		}

		if len(sp) == len(text) {
			return i - 1, true, false
		}

		if found >= 0 {
			ambiguous = true
		}

		found = i
	}

	if found < 0 || ambiguous {
		return 0, false, ambiguous
	}

	return found - 1, true, false
}

// dtConversion is one call's inputs besides the string: which fields may
// be omitted (the flags), the defaults (the defaults vector, or today's
// date and a midnight time), and the current time, for the relative days.
type dtConversion struct {
	omittable uint32
	defaults  [numFields]int // the fraction in hundredths, as $NUMTIM's
	now       uint64
}

// convertDateString reads text by the settings' input format. It returns
// the time, the mask of the fields that were defaulted, and the status:
// SS$_NORMAL; LIB$_AMBDATTIM for an ambiguous abbreviation;
// LIB$_INCDATTIM for a field omitted that the flags don't allow; and
// LIB$_IVTIME for a string that matches nothing, or whose date and time
// don't exist.
//
// The string's leading and trailing whitespace is ignored. A relative
// day name alone gives that day, with the time fields omitted
// (unconfirmed: whether a time may follow it).
func (s *dtSettings) convertDateString(text string, c dtConversion) (v uint64, defaulted uint32, status uint32) {
	text = strings.Trim(text, " \t")

	r := dtReading{meridiem: -1}

	if offset, ok, ambiguous := s.relativeDay(text); ok || ambiguous {
		if ambiguous {
			return 0, 0, libAmbDatTim
		}

		day := vmsdef.GoTime(c.now).AddDate(0, 0, offset)
		r.value[numYear], r.value[numMonth], r.value[numDay] = day.Year(), int(day.Month()), day.Day()
		r.given[numYear], r.given[numMonth], r.given[numDay] = true, true, true
	} else {
		m := &dtMatcher{s: s, items: s.input, input: text}

		matched := false

		for budget := 0; budget <= len(m.items) && !matched; budget++ {
			for start := range len(m.items) + 1 {
				if r, matched = m.match(start, 0, dtReading{meridiem: -1}, budget); matched {
					break
				}
			}
		}

		switch {
		case !matched && m.ambiguous:
			return 0, 0, libAmbDatTim
		case !matched:
			return 0, 0, libIvTime
		}
	}

	return s.complete(r, c)
}

// complete fills a reading's omitted fields with their defaults and makes
// the time. An omitted field whose bit the flags don't have is
// LIB$_INCDATTIM. A year written with fewer digits than four (!Y2's "88")
// takes the rest from the current year, not the defaults vector's: 88 in
// 2026 is 2088, and so is 8 by !Y2 with a default year of 1988 (VMS V7.1,
// testdata/lib56).
// A 12-hour hour must be 1 to 12; with the second meridiem indicator (PM)
// it is 12 hours later, and 12 with the first (AM) is 0. Without a
// meridiem, a 12-hour hour is taken as before noon, and a meridiem
// without an hour changes nothing: the default hour stays (both
// unconfirmed).
func (s *dtSettings) complete(r dtReading, c dtConversion) (uint64, uint32, uint32) {
	var (
		fields    [numFields]int
		defaulted uint32
	)

	for f := range numFields {
		if r.given[f] {
			fields[f] = r.value[f]

			continue
		}

		if c.omittable&(1<<f) == 0 {
			return 0, 0, libIncDatTim
		}

		defaulted |= 1 << f
		fields[f] = c.defaults[f]

		if f == numFraction {
			fields[f] *= ticksPerHundredth
		}
	}

	for _, it := range s.input {
		if it.m == nil {
			continue
		}

		if f := numFieldOf(it.m.field); f < 0 || !r.given[f] {
			continue
		}

		switch it.m.field {
		case fieldYear:
			if it.m.digits < 4 {
				unit := tenTo(it.m.digits)
				year := vmsdef.GoTime(c.now).Year()
				fields[numYear] = year - year%unit + r.value[numYear]
			}
		case fieldHour12:
			h := fields[numHour]
			if h < 1 || h > 12 {
				return 0, 0, libIvTime
			}

			fields[numHour] = h % 12
			if r.meridiem == 1 {
				fields[numHour] += 12
			}
		}
	}

	v, ok := vectorFields(fields)
	if !ok {
		return 0, 0, libIvTime
	}

	return v, defaulted, ssNormal
}

// vectorFields makes the absolute time of seven fields whose fraction is
// in ticks, by corevms.VectorTime's limits. Year 0 or month 0, which
// VectorTime reads as a delta, is out of range here.
func vectorFields(fields [numFields]int) (uint64, bool) {
	if fields[numYear] < 1 || fields[numMonth] < 1 || fields[numFraction] >= int(vmsdef.TicksPerSecond) {
		return 0, false
	}

	var vector [7]uint16

	for i := range numFraction {
		if fields[i] < 0 || fields[i] > 0xFFFF {
			return 0, false
		}

		vector[i] = uint16(fields[i])
	}

	v, ok := corevms.VectorTime(vector)
	if !ok {
		return 0, false
	}

	return v + uint64(fields[numFraction]), true
}

// conversionDefaults is the defaults an omitted field takes: the seven
// words of a defaults vector at addr, or, with none, the current date
// and a time of 00:00:00.00 (the manual's rule).
func conversionDefaults(env *corevms.Environment, addr uint32, now uint64) ([numFields]int, error) {
	var out [numFields]int

	if addr == 0 {
		t := vmsdef.GoTime(now)
		out[numYear], out[numMonth], out[numDay] = t.Year(), int(t.Month()), t.Day()

		return out, nil
	}

	mem, cpu := env.Memory(), env.CPU()
	for i := range out {
		w, err := mem.LoadWord(cpu, addr+uint32(2*i))
		if err != nil {
			return out, err
		}

		out[i] = int(w)
	}

	return out, nil
}

// libConvertDateString is LIB$CONVERT_DATE_STRING (LIB-27):
//
//	LIB$CONVERT_DATE_STRING date-string, date-time [,user-context]
//	                        [,flags] [,defaults] [,defaulted-fields]
//
// It reads date-string by the context's input format (convertDateString)
// and writes the time to date-time, and the mask of the fields it
// defaulted to defaulted-fields. flags says which fields may be omitted
// (0x78, the time's, when it's omitted; bits past the seventh are
// ignored, unconfirmed), and defaults gives their values. Its statuses:
//
//   - LIB$_WRONUMARG for fewer than two arguments or more than six, and
//     LIB$_INVARG when date-string or date-time is omitted;
//   - LIB$_ILLFORMAT for an input format it can't read by
//     (checkInputFormat), and the context's statuses;
//   - LIB$_INVSTRDES for a bad descriptor;
//   - convertDateString's LIB$_AMBDATTIM, LIB$_INCDATTIM, and
//     LIB$_IVTIME, with nothing written;
//   - else the settings' LIB$_ENGLUSED or LIB$_DEFFORUSE, or SS$_NORMAL.
func libConvertDateString(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 || len(argv) > 6 {
		return libWroNumArg, nil
	}

	if arg(argv, 0) == 0 || arg(argv, 1) == 0 {
		return libInvArg, nil
	}

	s, status, err := contextSettings(env, arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	}

	if status != ssNormal {
		return status, nil
	}

	if s.inputError != 0 {
		return s.inputError, nil
	}

	if !checkInputFormat(s.input) {
		return libIllFormat, nil
	}

	text, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	c := dtConversion{omittable: defaultOmittable, now: env.Clock()}

	flags, given, err := optionalFlags(env, arg(argv, 3))
	if err != nil {
		return memoryFault(env, err)
	}

	if given {
		c.omittable = flags & (1<<numFields - 1)
	}

	if c.defaults, err = conversionDefaults(env, arg(argv, 4), c.now); err != nil {
		return memoryFault(env, err)
	}

	v, defaulted, status := s.convertDateString(text, c)
	if status != ssNormal {
		return status, nil
	}

	if err := env.Memory().StoreQuadword(env.CPU(), arg(argv, 1), v); err != nil {
		return memoryFault(env, writing(err))
	}

	if addr := arg(argv, 5); addr != 0 {
		if err := writeResult(env, addr, int32(defaulted)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return s.inputWarning(), nil
}
