package fdl

import (
	"slices"
	"strconv"
	"strings"
)

// Options are what Parse needs from its caller.
type Options struct {
	// ParseTime reads a DATE section's time as DCL reads an absolute
	// time, returning it as a VMS time; false is INVDATIM. Unset, every
	// date is refused.
	ParseTime func(text string) (uint64, bool)
}

// statement is one statement of the file: its number and text.
type statement struct {
	number int
	text   string
}

// parser is one Parse's state.
type parser struct {
	opts Options
	def  *Definition
	msgs []Message

	// cur is the section secondaries go into, and curPrim its primary;
	// both nil before the first primary, after an unknown one, and after
	// a refused one.
	cur     *Section
	curPrim *primary

	// refused says the last primary recognized was refused (VALPRI,
	// OUTORDER): the next one adds NULLPRI.
	refused bool

	// next is each numbered primary's next number.
	next map[string]int
}

// Parse reads text, an FDL file, into a Definition, and returns the
// messages VMS's parser would give in the order it would give them
// (doc.go has the rules). The Definition holds what was taken; Failed
// says whether an error means it mustn't be used.
func Parse(text string, opts Options) (*Definition, []Message) {
	p := &parser{opts: opts, def: &Definition{}, next: map[string]int{}}

	for _, s := range statements(text) {
		p.statement(s)
	}

	return p.def, p.msgs
}

// statements splits text into its statements: each line's parts between
// semicolons, up to an exclamation point, neither counting inside quotation
// marks. A string with no closing quotation mark runs to the line's end.
// Blank statements aren't numbered.
func statements(text string) []statement {
	var (
		list []statement
		n    int
	)

	add := func(s string) {
		if s = strings.Trim(s, " \t"); s != "" {
			n++
			list = append(list, statement{number: n, text: s})
		}
	}

	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSuffix(line, "\r")

		var quote byte

		start := 0

	scan:
		for i := 0; i < len(line); i++ {
			c := line[i]

			switch {
			case quote != 0:
				if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == ';':
				add(line[start:i])
				start = i + 1
			case c == '!':
				line = line[:i]

				break scan
			}
		}

		add(line[start:])
	}

	return list
}

// keyword splits a statement into its keyword, in capitals, and what
// follows it as written.
func keyword(text string) (word, rest string) {
	i := 0
	for i < len(text) && isWordChar(text[i]) {
		i++
	}

	return strings.ToUpper(text[:i]), text[i:]
}

func isWordChar(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// shownKeyword is how an unknown primary's keyword is shown in its
// message: up to the end of its first digits, as VMS 7.1 showed
// SEG0_LENGTH as SEG0.
func shownKeyword(word string) string {
	i := 0
	for i < len(word) && (word[i] < '0' || word[i] > '9') {
		i++
	}

	for i < len(word) && word[i] >= '0' && word[i] <= '9' {
		i++
	}

	return word[:i]
}

// report adds a message about statement n.
func (p *parser) report(ident string, severity byte, n int) {
	p.msgs = append(p.msgs, Message{Ident: ident, Severity: severity, Statement: n})
}

// reportSegment adds a message about statement n and the text segment.
func (p *parser) reportSegment(ident string, severity byte, n int, segment string) {
	p.msgs = append(p.msgs, Message{Ident: ident, Severity: severity, Statement: n, Segment: segment, HasSegment: true})
}

// statement reads one statement: a secondary of the current primary, or
// else a primary. A keyword that is a primary's name in full is a primary,
// unless it's a secondary's name in full too, so CONNECT's KEY_ secondaries
// don't take KEY 0 for an abbreviation of theirs.
func (p *parser) statement(s statement) {
	word, rest := keyword(s.text)
	value := strings.Trim(rest, " \t")
	pnames := primaryNames()

	if p.curPrim != nil {
		names := p.curPrim.secondaryNames()
		i, ambiguous := match(word, names)

		if (i >= 0 && names[i] == word) || !slices.Contains(pnames, word) {
			switch {
			case i >= 0:
				p.secondary(s.number, p.curPrim.secondaries[i], value, rest)

				return
			case ambiguous:
				p.reportSegment("ABKW", 'W', s.number, word)

				return
			}
		}
	}

	i, ambiguous := match(word, pnames)

	switch {
	case ambiguous:
		p.reportSegment("ABKW", 'W', s.number, word)
		p.cur, p.curPrim = nil, nil

		return
	case i < 0:
		p.reportSegment("UNPRIKW", 'E', s.number, shownKeyword(word))
		p.cur, p.curPrim = nil, nil

		return
	}

	if p.refused {
		p.msgs = append(p.msgs, Message{Ident: "NULLPRI", Severity: 'W'})
	}

	p.refused = false
	p.primary(s.number, &primaries[i], value)
}

// refuse leaves no current section after a primary that was refused.
func (p *parser) refuse() {
	p.cur, p.curPrim, p.refused = nil, nil, true
}

// primary begins a primary's section.
func (p *parser) primary(n int, prim *primary, value string) {
	if prim.numbered {
		number, err := strconv.Atoi(firstWord(value))
		if value == "" || err != nil || number < 0 {
			p.report("VALPRI", 'W', n)
			p.refuse()

			return
		}

		if number != p.next[prim.name] {
			p.report("OUTORDER", 'E', n)
			p.refuse()

			return
		}

		p.next[prim.name]++
		p.begin(prim, &Section{Name: prim.name, ID: number})

		return
	}

	// A primary given again: NOQUAL, and its secondaries go into the
	// first (VMS 7.1's C47).
	if s := p.def.Section(prim.name); s != nil {
		p.report("NOQUAL", 'W', n)
		p.cur, p.curPrim = s, prim

		return
	}

	s := &Section{Name: prim.name}
	if prim.text {
		s.Text = readString(value)
	}

	p.begin(prim, s)
}

// begin makes s the current section, of prim.
func (p *parser) begin(prim *primary, s *Section) {
	p.def.Sections = append(p.def.Sections, s)
	p.cur, p.curPrim = s, prim
}

// secondary reads a secondary's value into the current section. rest is
// the statement after the keyword as written, a SYNTAX message's segment.
func (p *parser) secondary(n int, a attribute, value, rest string) {
	if value == "" {
		p.report("NOQUAL", 'W', n)

		return
	}

	v, ok := p.value(n, a, value, rest)
	if !ok {
		return
	}

	if p.cur.set(a.name, v) {
		p.report("MULSEC", 'W', n)
	}
}

// value reads a's value from value, reporting what's wrong with it.
func (p *parser) value(n int, a attribute, value, rest string) (Value, bool) {
	v := Value{Kind: a.kind}
	syntax := func() (Value, bool) {
		p.reportSegment("SYNTAX", 'E', n, strings.ToUpper(strings.TrimRight(rest, " \t")))

		return v, false
	}

	switch a.kind {
	case Switch:
		w := strings.ToUpper(firstWord(value))

		switch {
		case isPrefix(w, "YES") || isPrefix(w, "TRUE"):
			v.Switch = true
		case isPrefix(w, "NO") || isPrefix(w, "FALSE"):
			v.Switch = false
		default:
			p.reportSegment("UNQUAKW", 'W', n, w)

			return v, false
		}

	case Keyword:
		w := strings.ToUpper(firstWord(value))

		i, ambiguous := match(w, a.keywords)

		switch {
		case ambiguous:
			p.reportSegment("ABKW", 'W', n, w)

			return v, false
		case i < 0:
			p.reportSegment("UNQUAKW", 'W', n, w)

			return v, false
		}

		v.Text = a.keywords[i]

	case Number:
		number, err := strconv.ParseInt(firstWord(value), 10, 64)
		if err != nil {
			return syntax()
		}

		v.Number = number

	case String, FreeText:
		v.Text = readString(value)

	case Date:
		v.Text = readString(value)

		ok := false
		if p.opts.ParseTime != nil {
			v.Time, ok = p.opts.ParseTime(v.Text)
		}

		if !ok {
			p.report("INVDATIM", 'W', n)

			return v, false
		}

	case Protection:
		mask, ok := parseProtection(value)
		if !ok {
			return syntax()
		}

		v.Protection = mask

	case Owner:
		if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
			return syntax()
		}

		v.Text = strings.ToUpper(value)

	case Character:
		c, ok := readCharacter(value)
		if !ok {
			return syntax()
		}

		v.Char = c
	}

	return v, true
}

// isPrefix reports whether w is a leading part of word, at least one
// character.
func isPrefix(w, word string) bool {
	return w != "" && strings.HasPrefix(word, w)
}

// firstWord is value's text up to its first blank or tab.
func firstWord(value string) string {
	if i := strings.IndexAny(value, " \t"); i >= 0 {
		return value[:i]
	}

	return value
}

// readString reads a string value: the text between quotation marks
// (single or double), the rest of the statement when the closing one is
// missing, or the text as written without them.
func readString(value string) string {
	if value == "" || (value[0] != '"' && value[0] != '\'') {
		return value
	}

	if end := strings.IndexByte(value[1:], value[0]); end >= 0 {
		return value[1 : 1+end]
	}

	return value[1:]
}

// readCharacter reads NULL_VALUE's character: one in quotation marks, or
// its code in decimal.
func readCharacter(value string) (byte, bool) {
	if value[0] == '"' || value[0] == '\'' {
		s := readString(value)
		if len(s) != 1 {
			return 0, false
		}

		return s[0], true
	}

	n, err := strconv.ParseUint(firstWord(value), 10, 8)

	return byte(n), err == nil
}

// protectionCategories are a protection's categories, in the mask's order.
var protectionCategories = []string{"SYSTEM", "OWNER", "GROUP", "WORLD"}

// parseProtection reads "(system:RWED, owner:RWE, group:R, world:)" into
// a mask, each category's four bits denying read, write, execute, and
// delete. A category not named has no access (unconfirmed: govax's
// choice, as ANALYZE/RMS_FILE writes all four).
func parseProtection(value string) (uint16, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return 0, false
	}

	mask := uint16(0xFFFF)

	for item := range strings.SplitSeq(value[1:len(value)-1], ",") {
		item = strings.ToUpper(strings.TrimSpace(item))
		if item == "" {
			continue
		}

		name, access, _ := strings.Cut(item, ":")

		category, ambiguous := match(strings.TrimSpace(name), protectionCategories)
		if category < 0 || ambiguous {
			return 0, false
		}

		shift := 4 * category
		mask |= 0xF << shift

		for _, c := range strings.TrimSpace(access) {
			bit := strings.IndexRune("RWED", c)
			if bit < 0 {
				return 0, false
			}

			mask &^= 1 << (shift + bit)
		}
	}

	return mask, true
}
