package librtl

import (
	"sort"
	"strings"
)

// The date and time format mnemonics (docs/PHASE-56, "Formatting:
// mnemonics and predefined formats"; the 5.0 LIB$ manual's section
// 3.3.2.1). A format is text in which each field of a date or time is
// written as a mnemonic after an exclamation mark ("!DB-!MAAU-!Y4"), and
// everything else is punctuation: copied as written on output, matched
// on input. The formatting routines share this table and parser: the
// context (dtcontext.go) checks a format with it, and the output and
// input engines walk the items it makes.

// dtField is the field of a date or time a mnemonic stands for.
type dtField int

const (
	fieldDay         dtField = iota // day of the month, 1-31
	fieldWeekday                    // day of the week, spelled
	fieldMonthName                  // month, spelled
	fieldMonthNumber                // month, 1-12
	fieldYear                       // the year's last n digits
	fieldHour24                     // hour, 0-23
	fieldHour12                     // hour, 1-12
	fieldMinute                     // 0-59
	fieldSecond                     // 0-59
	fieldFraction                   // of a second, n digits
	fieldMeridiem                   // AM or PM, spelled
)

// isDate reports whether f is one of the date's fields rather than the
// time's: a format's elements are one or the other, never both.
func (f dtField) isDate() bool {
	return f <= fieldYear
}

// dtFill is how a number is filled to its width: with zeros ("09"), not
// at all ("9"), or with blanks (" 9"). The mnemonics spell it with a 0, a
// doubled letter, or a B.
type dtFill int

const (
	fillZero dtFill = iota
	fillNone
	fillBlank
)

// dtCase is how a spelled field is written: upper case (JANUARY),
// capitalized (January), or lower case (january); the mnemonics' U, C,
// and L. The order is that of the spellings kept for each (dtcontext.go).
type dtCase int

const (
	caseUpper dtCase = iota
	caseLower
	caseCapital
	caseCount
)

// dtMnemonic is one mnemonic: its text after the "!", the field, and how
// it is written. digits is a year's or a fraction's number of digits.
type dtMnemonic struct {
	code        string
	field       dtField
	fill        dtFill
	letterCase  dtCase
	abbreviated bool
	digits      int
}

// dtMnemonics is every mnemonic the manual lists, in its order. VMS
// V7.1 also reads them in lower case ("!d0", testdata/lib56); only what
// follows a "!" is a mnemonic, so "!H04H!M0m"'s "m" is punctuation.
var dtMnemonics = []dtMnemonic{
	{code: "D0", field: fieldDay, fill: fillZero},
	{code: "DD", field: fieldDay, fill: fillNone},
	{code: "DB", field: fieldDay, fill: fillBlank},

	{code: "WU", field: fieldWeekday, letterCase: caseUpper},
	{code: "WAU", field: fieldWeekday, letterCase: caseUpper, abbreviated: true},
	{code: "WC", field: fieldWeekday, letterCase: caseCapital},
	{code: "WAC", field: fieldWeekday, letterCase: caseCapital, abbreviated: true},
	{code: "WL", field: fieldWeekday, letterCase: caseLower},
	{code: "WAL", field: fieldWeekday, letterCase: caseLower, abbreviated: true},

	{code: "MAU", field: fieldMonthName, letterCase: caseUpper},
	{code: "MAAU", field: fieldMonthName, letterCase: caseUpper, abbreviated: true},
	{code: "MAC", field: fieldMonthName, letterCase: caseCapital},
	{code: "MAAC", field: fieldMonthName, letterCase: caseCapital, abbreviated: true},
	{code: "MAL", field: fieldMonthName, letterCase: caseLower},
	{code: "MAAL", field: fieldMonthName, letterCase: caseLower, abbreviated: true},

	{code: "MN0", field: fieldMonthNumber, fill: fillZero},
	{code: "MNM", field: fieldMonthNumber, fill: fillNone},
	{code: "MNB", field: fieldMonthNumber, fill: fillBlank},

	{code: "Y4", field: fieldYear, digits: 4},
	{code: "Y3", field: fieldYear, digits: 3},
	{code: "Y2", field: fieldYear, digits: 2},
	{code: "Y1", field: fieldYear, digits: 1},

	{code: "H04", field: fieldHour24, fill: fillZero},
	{code: "HH4", field: fieldHour24, fill: fillNone},
	{code: "HB4", field: fieldHour24, fill: fillBlank},
	{code: "H02", field: fieldHour12, fill: fillZero},
	{code: "HH2", field: fieldHour12, fill: fillNone},
	{code: "HB2", field: fieldHour12, fill: fillBlank},

	{code: "M0", field: fieldMinute, fill: fillZero},
	{code: "MM", field: fieldMinute, fill: fillNone},
	{code: "MB", field: fieldMinute, fill: fillBlank},

	{code: "S0", field: fieldSecond, fill: fillZero},
	{code: "SS", field: fieldSecond, fill: fillNone},
	{code: "SB", field: fieldSecond, fill: fillBlank},

	{code: "C7", field: fieldFraction, digits: 7},
	{code: "C6", field: fieldFraction, digits: 6},
	{code: "C5", field: fieldFraction, digits: 5},
	{code: "C4", field: fieldFraction, digits: 4},
	{code: "C3", field: fieldFraction, digits: 3},
	{code: "C2", field: fieldFraction, digits: 2},
	{code: "C1", field: fieldFraction, digits: 1},

	{code: "MIU", field: fieldMeridiem, letterCase: caseUpper},
	{code: "MIC", field: fieldMeridiem, letterCase: caseCapital},
	{code: "MIL", field: fieldMeridiem, letterCase: caseLower},
}

// dtLongestFirst is dtMnemonics ordered longest code first, the order a
// format is matched in: after "!MAAU" the mnemonic is MAAU, not MAU or
// MA-something, and after "!MM" it is MM, not M0.
var dtLongestFirst = func() []*dtMnemonic {
	out := make([]*dtMnemonic, len(dtMnemonics))
	for i := range dtMnemonics {
		out[i] = &dtMnemonics[i]
	}

	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].code) > len(out[j].code)
	})

	return out
}()

// dtItem is one piece of a parsed format: a mnemonic, or, when m is nil,
// punctuation.
type dtItem struct {
	m       *dtMnemonic
	literal string
}

// parseDateFormat splits a format into its items: each "!" begins the
// longest mnemonic that follows it, and the text between mnemonics is
// punctuation. ok is false (LIB$_UNRFORCOD) when a "!" begins none.
func parseDateFormat(format string) (items []dtItem, ok bool) {
	var text strings.Builder

	flush := func() {
		if text.Len() > 0 {
			items = append(items, dtItem{literal: text.String()})
			text.Reset()
		}
	}

	for i := 0; i < len(format); {
		if format[i] != '!' {
			text.WriteByte(format[i])
			i++

			continue
		}

		m := matchMnemonic(format[i+1:])
		if m == nil {
			return nil, false
		}

		flush()

		items = append(items, dtItem{m: m})
		i += 1 + len(m.code)
	}

	flush()

	return items, true
}

// matchMnemonic is the longest mnemonic s begins with, in either case,
// or nil.
func matchMnemonic(s string) *dtMnemonic {
	for _, m := range dtLongestFirst {
		if len(s) >= len(m.code) && strings.EqualFold(s[:len(m.code)], m.code) {
			return m
		}
	}

	return nil
}

// dtPart is which part of a date and time an output format's element
// gives: the date, the time, or neither (no fields at all). An element
// whose fields are of both is mixed.
type dtPart int

const (
	partNone dtPart = iota
	partDate
	partTime
	partMixed
)

// partOf is the part a format's items give.
func partOf(items []dtItem) dtPart {
	part := partNone

	for _, it := range items {
		if it.m == nil {
			continue
		}

		p := partTime
		if it.m.field.isDate() {
			p = partDate
		}

		switch part {
		case partNone:
			part = p
		case p:
		default:
			return partMixed
		}
	}

	return part
}

// slotOf is the part an output format's element stands for, by its
// first field: the time if that is a time field, else the date (an
// element with no fields included). VMS V7.1 doesn't refuse an element
// that mixes the two ("!D0 !H04" is a date part, testdata/lib56).
func slotOf(items []dtItem) dtPart {
	for _, it := range items {
		if it.m != nil {
			if it.m.field.isDate() {
				return partDate
			}

			return partTime
		}
	}

	return partDate
}

// parseInputFormat parses an input format. Its status is LIB$_UNRFORCOD
// for an unknown mnemonic, a weekday's included: VMS V7.1 doesn't know
// the weekday mnemonics in an input format (testdata/lib56).
func parseInputFormat(format string) ([]dtItem, uint32) {
	items, ok := parseDateFormat(format)
	if !ok {
		return nil, libUnrForCod
	}

	for _, it := range items {
		if it.m != nil && it.m.field == fieldWeekday {
			return nil, libUnrForCod
		}
	}

	return items, ssNormal
}

// checkInputFormat reports whether an input format is one
// LIB$CONVERT_DATE_STRING can read by (the manual's section 3.3.3, and
// LIB$_ILLFORMAT when it isn't). It must give, once each, a month
// (spelled or numeric), the day, the year, the hour, the minute, the
// second, and the fraction; a meridiem with a 12-hour clock, and none
// with a 24-hour one. The date's fields and the time's must form two
// groups, one after the other. A field given twice isn't allowed
// (unconfirmed). (A weekday, which parseInputFormat refuses, isn't
// either.)
func checkInputFormat(items []dtItem) bool {
	seen := map[dtField]bool{}
	groups := 0
	last := partNone

	for _, it := range items {
		if it.m == nil {
			continue
		}

		f := it.m.field
		if f == fieldWeekday || seen[f] {
			return false
		}

		seen[f] = true

		part := partTime
		if f.isDate() {
			part = partDate
		}

		if part != last {
			groups++
			last = part
		}
	}

	months := 0
	if seen[fieldMonthName] {
		months++
	}

	if seen[fieldMonthNumber] {
		months++
	}

	hours := 0
	if seen[fieldHour24] {
		hours++
	}

	if seen[fieldHour12] {
		hours++
	}

	return groups == 2 && months == 1 && hours == 1 &&
		seen[fieldDay] && seen[fieldYear] && seen[fieldMinute] && seen[fieldSecond] && seen[fieldFraction] &&
		seen[fieldMeridiem] == seen[fieldHour12]
}
