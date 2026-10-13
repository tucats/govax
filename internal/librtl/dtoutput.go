package librtl

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The date and time output routines (docs/PHASE-56, subtask 6; the
// survey's "Output"): LIB$FORMAT_DATE_TIME, which writes a time by the
// context's output format, LIB$GET_MAXIMUM_DATE_LENGTH, the widest text
// that format can write, and LIB$GET_DATE_FORMAT, the input format in a
// form a person can read. All three find their context and its settings
// through dtcontext.go, and walk the items dtformat.go's parser makes.
//
// An output format has a date part and a time part (each an element, in
// the order they're written); the flags argument chooses which are
// written, and a single blank separates the two when both write
// something.

// The output routines' flags (LIB$M_DATE_FIELDS, LIB$M_TIME_FIELDS).
var (
	flagDateFields = vmsdef.LibrarySymbols["LIB$M_DATE_FIELDS"]
	flagTimeFields = vmsdef.LibrarySymbols["LIB$M_TIME_FIELDS"]
)

// The legible mnemonics' order in LIB$K_FORMAT_MNEMONICS's nine elements
// (dtComponents): year, numeric month, day, hours, minutes, seconds,
// fraction, meridiem, alphabetic month.
const (
	legibleYear = iota
	legibleMonthNumber
	legibleDay
	legibleHour
	legibleMinute
	legibleSecond
	legibleFraction
	legibleMeridiem
	legibleMonthName
)

// dtFields is a time broken into what the mnemonics show: $NUMTIM's
// seven fields, but with the fraction in whole ticks of 100 ns (so !C7
// can show them all), and the day of the week, 1 (Monday) to 7.
type dtFields struct {
	year, month, day     int
	hour, minute, second int
	ticks                int
	weekDay              int
}

// fieldsOf breaks the absolute time v into its fields.
func fieldsOf(v uint64) dtFields {
	t := vmsdef.GoTime(v)

	weekDay := int(t.Weekday()) // Go counts from Sunday, 0
	if weekDay == 0 {
		weekDay = 7
	}

	return dtFields{
		year: t.Year(), month: int(t.Month()), day: t.Day(),
		hour: t.Hour(), minute: t.Minute(), second: t.Second(),
		ticks:   int(v % vmsdef.TicksPerSecond),
		weekDay: weekDay,
	}
}

// tenTo is 10 to the n.
func tenTo(n int) int {
	p := 1
	for range n {
		p *= 10
	}

	return p
}

// filled writes a one- or two-digit number by a mnemonic's fill: "09",
// "9", or " 9".
func filled(n int, fill dtFill) string {
	switch fill {
	case fillNone:
		return fmt.Sprint(n)
	case fillBlank:
		return fmt.Sprintf("%2d", n)
	}

	return fmt.Sprintf("%02d", n)
}

// spellingsFor is the spellings a spelled mnemonic chooses from, in its
// case: the month or weekday names (abbreviated or not), or the
// meridiem indicators.
func (s *dtSettings) spellingsFor(m *dtMnemonic) []string {
	code := componentMeridiem

	switch m.field {
	case fieldMonthName:
		code = componentMonthName
		if m.abbreviated {
			code = componentMonthAbb
		}
	case fieldWeekday:
		code = componentWeekday
		if m.abbreviated {
			code = componentWeekdayAbb
		}
	}

	return s.spellings[code][m.letterCase]
}

// formatItem writes one item of a format for the time f: punctuation as
// it is, a mnemonic as its field. These are govax's readings of the
// manual, unconfirmed: a year shows its last n digits, zero-filled (Y2 of
// 2005 is "05"); a fraction is truncated, as $ASCTIM's is (C2 of .149 is
// "14"); the 12-hour clock shows 00:xx and 12:xx as 12; and the meridiem
// is the first indicator (AM) before noon, the second from noon on.
func (s *dtSettings) formatItem(it dtItem, f dtFields) string {
	m := it.m
	if m == nil {
		return it.literal
	}

	switch m.field {
	case fieldDay:
		return filled(f.day, m.fill)
	case fieldMonthNumber:
		return filled(f.month, m.fill)
	case fieldMonthName:
		return s.spellingsFor(m)[f.month-1]
	case fieldWeekday:
		return s.spellingsFor(m)[f.weekDay-1]
	case fieldYear:
		return fmt.Sprintf("%0*d", m.digits, f.year%tenTo(m.digits))
	case fieldHour24:
		return filled(f.hour, m.fill)
	case fieldHour12:
		h := f.hour % 12
		if h == 0 {
			h = 12
		}

		return filled(h, m.fill)
	case fieldMinute:
		return filled(f.minute, m.fill)
	case fieldSecond:
		return filled(f.second, m.fill)
	case fieldFraction:
		return fmt.Sprintf("%0*d", m.digits, f.ticks/tenTo(7-m.digits))
	case fieldMeridiem:
		if f.hour < 12 {
			return s.spellingsFor(m)[0]
		}

		return s.spellingsFor(m)[1]
	}

	return ""
}

// itemWidth is the most bytes an item can write: punctuation's own
// length, a two-digit number's 2 (whatever its fill), a year's or a
// fraction's digits, and a spelled field's longest spelling in its case.
func (s *dtSettings) itemWidth(it dtItem) int {
	m := it.m
	if m == nil {
		return len(it.literal)
	}

	switch m.field {
	case fieldYear, fieldFraction:
		return m.digits
	case fieldMonthName, fieldWeekday, fieldMeridiem:
		widest := 0
		for _, sp := range s.spellingsFor(m) {
			widest = max(widest, len(sp))
		}

		return widest
	}

	return 2
}

// outputElements is the elements a call writes, in order, by VMS
// V7.1's rules (testdata/lib56). An empty second element stands for the
// standard format's element of its part (an output format of "|||"
// writes the standard time); an empty first element writes nothing.
// Without flags (given false) they are the output format's own. With
// flags, they are its elements whose part the flags choose
// (LIB$M_DATE_FIELDS, LIB$M_TIME_FIELDS; other bits ignored), and after
// them the standard element of a chosen part the format has no element
// for (LIB$DT_FORMAT naming only a time format, with both flags, writes
// "09:13 13-JAN-1988"). Flags choosing neither part write nothing.
func (s *dtSettings) outputElements(given bool, flags uint32) []dtElement {
	elements := append([]dtElement{}, s.output...)
	if len(elements) == 2 && len(elements[1].items) == 0 {
		elements[1] = standardElement(elements[1].slot)
	}

	if !given {
		return elements
	}

	chosen := func(p dtPart) bool {
		return (p == partDate && flags&flagDateFields != 0) || (p == partTime && flags&flagTimeFields != 0)
	}

	var out []dtElement

	for _, e := range elements {
		if chosen(e.slot) {
			out = append(out, e)
		}
	}

	for _, part := range []dtPart{partDate, partTime} {
		if chosen(part) && !hasSlot(elements, part) {
			out = append(out, standardElement(part))
		}
	}

	return out
}

// hasSlot reports whether one of elements stands for part.
func hasSlot(elements []dtElement, part dtPart) bool {
	for _, e := range elements {
		if e.slot == part {
			return true
		}
	}

	return false
}

// formatTime writes the time v by elements, a blank between each two
// that write something.
func (s *dtSettings) formatTime(elements []dtElement, v uint64) string {
	f := fieldsOf(v)

	var parts []string

	for _, e := range elements {
		var b strings.Builder
		for _, it := range e.items {
			b.WriteString(s.formatItem(it, f))
		}

		if b.Len() > 0 {
			parts = append(parts, b.String())
		}
	}

	return strings.Join(parts, " ")
}

// maximumLength is the most bytes formatTime can write by elements.
func (s *dtSettings) maximumLength(elements []dtElement) int {
	n := 0

	for _, e := range elements {
		width := 0
		for _, it := range e.items {
			width += s.itemWidth(it)
		}

		if width > 0 && n > 0 {
			n++ // the blank between the parts
		}

		n += width
	}

	return n
}

// legibleFormat is an input format as LIB$GET_DATE_FORMAT shows it: each
// field as the language's legible mnemonic (a year's and a fraction's
// with their digits after: YYYY4, CC2), and the punctuation with each
// run of blanks and tabs made one blank, and none at the ends
// (unconfirmed: the manual says "most whitespace compressed").
func (s *dtSettings) legibleFormat(items []dtItem) string {
	legible := s.spellings[componentMnemonics][caseUpper]

	var b strings.Builder

	for _, it := range items {
		m := it.m
		if m == nil {
			b.WriteString(compressBlanks(it.literal))

			continue
		}

		switch m.field {
		case fieldYear:
			fmt.Fprintf(&b, "%s%d", legible[legibleYear], m.digits)
		case fieldMonthNumber:
			b.WriteString(legible[legibleMonthNumber])
		case fieldMonthName:
			b.WriteString(legible[legibleMonthName])
		case fieldDay:
			b.WriteString(legible[legibleDay])
		case fieldHour24, fieldHour12:
			b.WriteString(legible[legibleHour])
		case fieldMinute:
			b.WriteString(legible[legibleMinute])
		case fieldSecond:
			b.WriteString(legible[legibleSecond])
		case fieldFraction:
			fmt.Fprintf(&b, "%s%d", legible[legibleFraction], m.digits)
		case fieldMeridiem:
			b.WriteString(legible[legibleMeridiem])
		}
	}

	return strings.TrimSpace(b.String())
}

// compressBlanks makes each run of blanks and tabs in s one blank.
func compressBlanks(s string) string {
	var b strings.Builder

	blank := false

	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			blank = true

			continue
		}

		if blank {
			b.WriteByte(' ')
			blank = false
		}

		b.WriteByte(s[i])
	}

	if blank {
		b.WriteByte(' ')
	}

	return b.String()
}

// contextSettings finds the settings of the context a user-context
// argument names (dateContextFor). status is SS$_NORMAL or the reason
// there are none; err is a memory error, which the caller signals.
func contextSettings(env *corevms.Environment, cell uint32) (*dtSettings, uint32, error) {
	c, status, err := dateContextFor(env, cell)
	if err != nil || status != ssNormal {
		return nil, status, err
	}

	return c.settings(env), ssNormal, nil
}

// optionalFlags reads a flags argument by reference; given is false when
// it is omitted.
func optionalFlags(env *corevms.Environment, addr uint32) (flags uint32, given bool, err error) {
	if addr == 0 {
		return 0, false, nil
	}

	flags, err = env.Memory().LoadLongword(env.CPU(), addr)

	return flags, true, err
}

// libFormatDateTime is LIB$FORMAT_DATE_TIME (LIB-169):
//
//	LIB$FORMAT_DATE_TIME date-string [,date] [,user-context]
//	                     [,date-length] [,flags]
//
// It writes the time date (the current time when date is omitted or 0 by
// value) by the context's output format (outputElements chooses the
// parts) to date-string, by the output's class, and the bytes written to
// date-length. A delta time is LIB$_ABSTIMREQ, and an output format
// with an unknown mnemonic LIB$_UNRFORCOD (LIB$_ELETOOLON for one INIT
// was given badly), with nothing written. Its statuses after writing:
// LIB$_STRTRU when the text was cut, else the settings' LIB$_ENGLUSED or
// LIB$_DEFFORUSE, else SS$_NORMAL.
func libFormatDateTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	v, err := optionalTime(env, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	if isDelta(v) {
		return libAbsTimReq, nil
	}

	flags, given, err := optionalFlags(env, arg(argv, 4))
	if err != nil {
		return memoryFault(env, err)
	}

	s, status, err := contextSettings(env, arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	}

	if status != ssNormal {
		return status, nil
	}

	if s.outputError != 0 {
		return s.outputError, nil
	}

	stored, truncated, err := writeString(env, arg(argv, 0), s.formatTime(s.outputElements(given, flags), v))
	if err != nil {
		return stringFailure(env, err)
	}

	if addr := arg(argv, 3); addr != 0 {
		if err := writeResult(env, addr, int32(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return s.outputWarning(), nil
}

// libGetMaximumDateLength is LIB$GET_MAXIMUM_DATE_LENGTH (LIB-216):
//
//	LIB$GET_MAXIMUM_DATE_LENGTH date-length [,user-context] [,flags]
//
// It writes to date-length the most bytes LIB$FORMAT_DATE_TIME can write
// with the same context and flags (maximumLength): each field at its
// widest in the language, and the blank between the parts. The manual's
// example says 28 for a format whose widest text, "SEPTEMBER 21, 1988
// 11:24 PM", is 27 bytes; VMS V7.1 says 27 (testdata/lib56). A format
// that can't be used writes 0 and returns its status. Its statuses are
// LIB$FORMAT_DATE_TIME's warnings.
func libGetMaximumDateLength(env *corevms.Environment, argv []uint32) (uint32, error) {
	flags, given, err := optionalFlags(env, arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	}

	s, status, err := contextSettings(env, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	if status != ssNormal {
		return status, nil
	}

	// A format that can't be used writes 0, as VMS V7.1 does.
	n := 0
	if s.outputError == 0 {
		n = s.maximumLength(s.outputElements(given, flags))
	}

	if err := writeResult(env, arg(argv, 0), int32(n)); err != nil {
		return memoryFault(env, writing(err))
	}

	if s.outputError != 0 {
		return s.outputError, nil
	}

	return s.outputWarning(), nil
}

// libGetDateFormat is LIB$GET_DATE_FORMAT (LIB-204):
//
//	LIB$GET_DATE_FORMAT format-string [,user-context]
//
// It writes the context's input format in legible form (legibleFormat)
// to format-string, by the output's class. An input format that
// LIB$CONVERT_DATE_STRING can't read by (checkInputFormat) is
// LIB$_ILLFORMAT, with nothing written. Its statuses after writing:
// LIB$_STRTRU when the text was cut, else the settings' LIB$_ENGLUSED or
// LIB$_DEFFORUSE (the standard input format's text when no format was
// given; unconfirmed), else SS$_NORMAL. One or two arguments, else
// LIB$_WRONUMARG (unconfirmed for more than two).
func libGetDateFormat(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 1 && len(argv) != 2 {
		return libWroNumArg, nil
	}

	if arg(argv, 0) == 0 {
		return libInvArg, nil
	}

	s, status, err := contextSettings(env, arg(argv, 1))
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

	_, truncated, err := writeString(env, arg(argv, 0), s.legibleFormat(s.input))
	if err != nil {
		return stringFailure(env, err)
	}

	if truncated {
		return libStrTru, nil
	}

	return s.inputWarning(), nil
}
