package console

import (
	"sort"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// The lexical functions on strings and data types (docs/PHASE-50 - DCL
// command procedures.md, subtask 14; the User's Manual, 15.6 and 15.7):
// F$EXTRACT, F$LOCATE, F$ELEMENT, F$EDIT, F$FAO, F$CVSI, F$CVUI,
// F$INTEGER, F$LENGTH, F$STRING, and F$TYPE. Offsets count from 0
// (15.6.1).

// lexInteger is F$INTEGER(expression): the expression's value as an
// integer, converted by the rules for strings (12.9.1).
func lexInteger(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	return dclInteger(args[0].int()), nil
}

// lexLength is F$LENGTH(string): the string's length.
func lexLength(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	return dclInteger(int32(len(args[0].str()))), nil
}

// lexString is F$STRING(expression): the expression's value as a string.
func lexString(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	return dclString(args[0].str()), nil
}

// lexExtract is F$EXTRACT(start, length, string): length characters of
// string from offset start, fewer if the string ends first, and "" if
// start is past its end. A negative start or length is CLI_INVRANGE
// (unconfirmed against VMS).
func lexExtract(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	start, length, s := args[0].int(), args[1].int(), args[2].str()
	if start < 0 || length < 0 {
		return dclValue{}, vmserrors.New(vmserrors.CLI_INVRANGE)
	}

	if int(start) >= len(s) {
		return dclString(""), nil
	}

	end := min(int64(start)+int64(length), int64(len(s)))

	return dclString(s[start:end]), nil
}

// lexLocate is F$LOCATE(substring, string): the offset of substring's
// first occurrence in string, or string's length if it has none (15.6.1).
// An empty substring is at offset 0 (unconfirmed against VMS).
func lexLocate(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	sub, s := args[0].str(), args[1].str()

	i := strings.Index(s, sub)
	if i < 0 {
		i = len(s)
	}

	return dclInteger(int32(i)), nil
}

// lexElement is F$ELEMENT(number, delimiter, string): the element of
// string, counting from 0, between the delimiters around it (15.6.2). A
// string with no delimiters is one element. Past the last element, the
// result is the delimiter itself. The delimiter is one character: a
// longer or empty one is CLI_IVVALU, and a negative number CLI_INVRANGE
// (both unconfirmed against VMS).
func lexElement(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	n, delim, s := args[0].int(), args[1].str(), args[2].str()

	if len(delim) != 1 {
		return dclValue{}, vmserrors.New(vmserrors.CLI_IVVALU)
	}

	if n < 0 {
		return dclValue{}, vmserrors.New(vmserrors.CLI_INVRANGE)
	}

	elements := strings.Split(s, delim)
	if int(n) >= len(elements) {
		return dclString(delim), nil
	}

	return dclString(elements[n]), nil
}

// dclEdit is one of F$EDIT's edits: where it comes in the order the edits
// are made, and what it does to a string's text outside quotes.
type dclEdit struct {
	order int
	edit  func(s string) string
}

// dclEdits are F$EDIT's edits, by keyword. However they're listed, they
// are made in this order: UNCOMMENT, COLLAPSE, COMPRESS, TRIM, LOWERCASE,
// UPCASE (an order govax chose; unconfirmed against VMS, where it would
// only matter for UPCASE with LOWERCASE). Text inside quotes is never
// edited, quotes and all.
var dclEdits = map[string]dclEdit{
	"UNCOMMENT": {0, uncomment},
	"COLLAPSE":  {1, func(s string) string { return editOutsideQuotes(s, removeBlanks) }},
	"COMPRESS":  {2, func(s string) string { return editOutsideQuotes(s, compressBlanks) }},
	"TRIM":      {3, trimOutsideQuotes},
	"LOWERCASE": {4, func(s string) string { return editOutsideQuotes(s, strings.ToLower) }},
	"UPCASE":    {5, func(s string) string { return editOutsideQuotes(s, strings.ToUpper) }},
}

// lexEdit is F$EDIT(string, edit-list): string with the edits the list
// names (dclEdits), separated by commas. An edit DCL doesn't have is
// CLI_IVKEYW.
func lexEdit(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	var edits []dclEdit

	for _, key := range lexicalKeywords(args[1]) {
		edit, _, err := lexicalKeyword(dclEdits, lexicalArg{value: dclString(key)})
		if err != nil {
			return dclValue{}, err
		}

		edits = append(edits, edit)
	}

	sort.SliceStable(edits, func(i, j int) bool { return edits[i].order < edits[j].order })

	s := args[0].str()
	for _, e := range edits {
		s = e.edit(s)
	}

	return dclString(s), nil
}

// textRun is a piece of a string: inside quotes (quoted, quotes and all)
// or outside them.
type textRun struct {
	text   string
	quoted bool
}

// quoteRuns splits s into runs outside and inside quotes. A quoted run
// starts at a quote and ends at the next one not doubled; one with no end
// runs to the end of s.
func quoteRuns(s string) []textRun {
	var runs []textRun

	for s != "" {
		q := strings.IndexByte(s, '"')
		if q < 0 {
			runs = append(runs, textRun{text: s})

			break
		}

		if q > 0 {
			runs = append(runs, textRun{text: s[:q]})
		}

		_, n, _ := quotedString(s[q:])
		runs = append(runs, textRun{text: s[q : q+n], quoted: true})
		s = s[q+n:]
	}

	return runs
}

// editOutsideQuotes applies edit to each run of s outside quotes.
func editOutsideQuotes(s string, edit func(string) string) string {
	var b strings.Builder

	for _, r := range quoteRuns(s) {
		if r.quoted {
			b.WriteString(r.text)
		} else {
			b.WriteString(edit(r.text))
		}
	}

	return b.String()
}

// isBlank reports whether ch is a blank or a tab, the characters F$EDIT
// treats as space.
func isBlank(ch rune) bool { return ch == ' ' || ch == '\t' }

// removeBlanks is COLLAPSE's edit: every blank and tab removed.
func removeBlanks(s string) string {
	return strings.Map(func(ch rune) rune {
		if isBlank(ch) {
			return -1
		}

		return ch
	}, s)
}

// compressBlanks is COMPRESS's edit: each run of blanks and tabs made one
// blank.
func compressBlanks(s string) string {
	var b strings.Builder

	inBlanks := false

	for _, ch := range s {
		if isBlank(ch) {
			if !inBlanks {
				b.WriteByte(' ')
			}

			inBlanks = true

			continue
		}

		inBlanks = false

		b.WriteRune(ch)
	}

	return b.String()
}

// trimOutsideQuotes is TRIM's edit: blanks and tabs removed from the
// start and end of s, unless a quoted run is there.
func trimOutsideQuotes(s string) string {
	runs := quoteRuns(s)
	if len(runs) == 0 {
		return s
	}

	if first := &runs[0]; !first.quoted {
		first.text = strings.TrimLeftFunc(first.text, isBlank)
	}

	if last := &runs[len(runs)-1]; !last.quoted {
		last.text = strings.TrimRightFunc(last.text, isBlank)
	}

	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.text)
	}

	return b.String()
}

// uncomment is UNCOMMENT's edit: everything from the first "!" outside
// quotes removed.
func uncomment(s string) string {
	pos := 0

	for _, r := range quoteRuns(s) {
		if i := strings.IndexByte(r.text, '!'); !r.quoted && i >= 0 {
			return s[:pos+i]
		}

		pos += len(r.text)
	}

	return s
}

// lexCVSI is F$CVSI(start-bit, bits, string): the bit field of string
// that starts at start-bit and is bits long, as a signed integer (15.7).
func lexCVSI(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	return bitField(args, true)
}

// lexCVUI is F$CVUI(start-bit, bits, string): the bit field as an
// unsigned integer.
func lexCVUI(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	return bitField(args, false)
}

// bitField is F$CVSI's and F$CVUI's work. The string's bits are numbered
// as the VAX numbers them: bit 0 is the low bit of its first character,
// bit 8 the low bit of the second, and so on. A field longer than 32
// bits, or one that doesn't fit in the string, is CLI_INVRANGE.
func bitField(args []lexicalArg, signed bool) (dclValue, error) {
	start, bits, s := args[0].int(), args[1].int(), args[2].str()

	if start < 0 || bits < 0 || bits > 32 || int64(start)+int64(bits) > 8*int64(len(s)) {
		return dclValue{}, vmserrors.New(vmserrors.CLI_INVRANGE)
	}

	var v uint64

	for i := bits - 1; i >= 0; i-- {
		bit := start + i
		v = v<<1 | uint64(s[bit/8]>>(bit%8)&1)
	}

	if signed && bits > 0 && v&(1<<(bits-1)) != 0 {
		v |= ^uint64(0) << bits
	}

	return dclInteger(int32(uint32(v))), nil
}

// lexType is F$TYPE(symbol-name): "INTEGER" for a symbol whose value is
// an integer, or a string that is one (as "12" is), "STRING" for any
// other string, and "" for a name that isn't a symbol (15.7.3).
func lexType(e *dclExpression, args []lexicalArg) (dclValue, error) {
	sym, ok := e.symbols.lookup(args[0].name)

	switch {
	case !ok:
		return dclString(""), nil
	case sym.integer || isIntegerString(sym.value):
		return dclString("INTEGER"), nil
	default:
		return dclString("STRING"), nil
	}
}

// isIntegerString reports whether s is an integer as DCL writes one: a
// sign, if any, then a number (decimal, or with a radix prefix), with
// blanks around it allowed (unconfirmed against VMS: the blanks and the
// radix prefix).
func isIntegerString(s string) bool {
	s = strings.TrimSpace(s)
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = strings.TrimSpace(s[1:])
	}

	_, ok := parseDCLNumber(s)

	return ok
}

// lexFAO is F$FAO(control-string, argument...): the control string with
// its $FAO directives replaced by the arguments, formatted (15.6.3), by
// the formatter $FAO uses (corevms.Environment.FormatFAOValues). A
// string directive takes a string argument, and a numeric one an
// integer (a string argument is converted to one). A directive $FAO
// doesn't know, or too few arguments, is $FAO's status, as an error.
func lexFAO(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	var params []corevms.FAOValue

	for _, a := range args[1:] {
		if !a.present {
			break
		}

		params = append(params, corevms.FAOValue{IsString: !a.value.integer, Num: uint32(a.int()), Str: a.str()})
	}

	s, status := env.FormatFAOValues(args[0].str(), params)
	if status != 0 {
		return dclValue{}, e.console.statusFailure(status)
	}

	return dclString(s), nil
}

// lexicalEnvironment is the process the lexical functions ask about: the
// console's own (process 1's). Before INIT there is none (CLI_NOVAX).
func (c *Console) lexicalEnvironment() (*corevms.Environment, error) {
	if c.RTL == nil {
		return nil, vmserrors.New(vmserrors.CLI_NOVAX)
	}

	return c.RTL, nil
}
