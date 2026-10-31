package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
)

// What each of LIB$TPARSE's symbol types matches (the manual's Table LIB-21),
// a table with a row per type code. The rules the manuals leave open are in
// docs/PHASE-80's subtask 10, to be settled by testdata/tpa80's run on VMS.

// tpaMatch is what a transition matched.
type tpaMatch struct {
	// n is how many characters the token is.
	n uint32

	// char says the type is a single character's: value8 goes to TPA$B_CHAR
	// for an action routine, and to msk-adr as a byte.
	char   bool
	value8 byte

	// number says value goes to TPA$L_NUMBER for an action routine; longword
	// that it also goes to msk-adr as a longword, where another type stores
	// its token's descriptor.
	number, longword bool
	value            uint32
}

// tpaInput reads the string left to parse. A memory error ends the string, and
// is kept in the parser, for whoever called the matcher to find.
type tpaInput struct {
	p   *tparser
	ptr uint32
	n   uint32
}

// at returns the character at offset i, if the string goes that far.
func (in tpaInput) at(i uint32) (byte, bool) {
	if i >= in.n {
		return 0, false
	}

	c, err := in.p.mem.LoadByte(in.p.cpu, in.ptr+i)
	if err != nil {
		if in.p.err == nil {
			in.p.err = err
		}

		return 0, false
	}

	return c, true
}

// run returns how many characters from offset i on satisfy class.
func (in tpaInput) run(i uint32, class func(byte) bool) uint32 {
	n := uint32(0)

	for {
		c, ok := in.at(i + n)
		if !ok || !class(c) {
			return n
		}

		n++
	}
}

// text returns up to limit characters of the string.
func (in tpaInput) text(limit uint32) string {
	var b strings.Builder

	for i := range min(in.n, limit) {
		c, ok := in.at(i)
		if !ok {
			break
		}

		b.WriteByte(c)
	}

	return b.String()
}

func isBlankChar(c byte) bool { return c == ' ' || c == '\t' }

func isDigitChar(c byte) bool { return c >= '0' && c <= '9' }

func isOctalChar(c byte) bool { return c >= '0' && c <= '7' }

func isHexChar(c byte) bool {
	return isDigitChar(c) || c >= 'A' && c <= 'F'
}

func isLetterChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// isMultinational says whether c is a letter of the DEC multinational set.
func isMultinational(c byte) bool {
	return c >= 0xC0 && c <= 0xFD
}

func isAlphaChar(c byte) bool { return isLetterChar(c) || isMultinational(c) }

func isStringChar(c byte) bool { return isLetterChar(c) || isDigitChar(c) }

// isKeywordChar is the symbol constituent set a keyword is written in.
func isKeywordChar(c byte) bool { return isStringChar(c) || c == '$' || c == '_' }

// isSymbolChar is TPA$_SYMBOL's set: the keywords', and multinational letters.
func isSymbolChar(c byte) bool { return isKeywordChar(c) || isMultinational(c) }

// tpaSymbolTypes is each symbol type's matcher, by its type code.
var tpaSymbolTypes = map[byte]func(*tparser, tpaInput) (tpaMatch, bool){
	231: func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		parts := corevms.ScanFileSpec(in.text(tpaSpecLimit))

		return tpaMatch{n: uint32(parts.Access.End)}, parts.Primary.Len() > 0
	},
	232: func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		parts := corevms.ScanFileSpec(in.text(tpaSpecLimit))

		return tpaMatch{n: uint32(parts.Primary.End)}, parts.Primary.Len() > 0
	},
	233: func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		parts := corevms.ScanFileSpec(in.text(tpaSpecLimit))

		return tpaMatch{n: uint32(parts.Node.End)}, parts.Node.Len() > 0
	},
	234: func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		parts := corevms.ScanFileSpec(in.text(tpaSpecLimit))

		return tpaMatch{n: uint32(parts.End)}, parts.End > 0
	},
	235: matchUIC,
	236: matchIdent,
	237: oneChar(func(byte) bool { return true }),
	238: oneChar(isAlphaChar),
	239: oneChar(isDigitChar),
	240: charRun(isStringChar),
	241: charRun(isSymbolChar),
	242: func(p *tparser, in tpaInput) (tpaMatch, bool) {
		options, err := p.field(tpaOptions)
		if err != nil {
			p.err = err

			return tpaMatch{}, false
		}

		n := in.run(0, isBlankChar)

		return tpaMatch{n: n}, options&tpaBlanks != 0 && n > 0
	},
	243: numberOf(10, isDigitChar),
	244: numberOf(8, isOctalChar),
	245: numberOf(16, isHexChar),
	246: func(*tparser, tpaInput) (tpaMatch, bool) { return tpaMatch{}, true },
	247: func(_ *tparser, in tpaInput) (tpaMatch, bool) { return tpaMatch{}, in.n == 0 },
}

// tpaSpecLimit is the most of the string a file specification is looked for in.
const tpaSpecLimit = 4096

func oneChar(class func(byte) bool) func(*tparser, tpaInput) (tpaMatch, bool) {
	return func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		c, ok := in.at(0)
		if !ok || !class(c) {
			return tpaMatch{}, false
		}

		return tpaMatch{n: 1, char: true, value8: c}, true
	}
}

func charRun(class func(byte) bool) func(*tparser, tpaInput) (tpaMatch, bool) {
	return func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		n := in.run(0, class)

		return tpaMatch{n: n}, n > 0
	}
}

func numberOf(radix uint64, class func(byte) bool) func(*tparser, tpaInput) (tpaMatch, bool) {
	return func(_ *tparser, in tpaInput) (tpaMatch, bool) {
		n := in.run(0, class)
		if n == 0 {
			return tpaMatch{}, false
		}

		value, ok := in.value(0, n, radix)

		return tpaMatch{n: n, number: true, longword: true, value: value}, ok
	}
}

// value converts n digits at offset i, and says whether it's less than 2**32.
func (in tpaInput) value(i, n uint32, radix uint64) (uint32, bool) {
	total := uint64(0)

	for k := range n {
		c, ok := in.at(i + k)
		if !ok {
			return 0, false
		}

		digit := uint64(c - '0')
		if c >= 'A' {
			digit = uint64(c-'A') + 10
		}

		total = total*radix + digit
		if total > 0xFFFFFFFF {
			return 0, false
		}
	}

	return uint32(total), true
}

// uicField reads one field of a UIC at offset i: octal digits up to a word, or
// * for all of its bits. It returns the value and the field's length.
func (in tpaInput) uicField(i uint32) (uint32, uint32, bool) {
	if c, ok := in.at(i); ok && c == '*' {
		return 0xFFFF, 1, true
	}

	n := in.run(i, isOctalChar)
	if n == 0 {
		return 0, 0, false
	}

	value, ok := in.value(i, n, 8)

	return value, n, ok && value <= 0xFFFF
}

// closingBracket returns the bracket that ends one opening at c.
func closingBracket(c byte) (byte, bool) {
	switch c {
	case '[':
		return ']', true
	case '<':
		return '>', true
	}

	return 0, false
}

// matchUIC matches [group,member] or <group,member>. Its value is an action
// routine's TPA$L_NUMBER; msk-adr gets its descriptor (Table LIB-21 calls only
// the three number types numbers).
func matchUIC(_ *tparser, in tpaInput) (tpaMatch, bool) {
	open, ok := in.at(0)
	if !ok {
		return tpaMatch{}, false
	}

	closing, ok := closingBracket(open)
	if !ok {
		return tpaMatch{}, false
	}

	group, n, ok := in.uicField(1)
	if !ok {
		return tpaMatch{}, false
	}

	i := 1 + n
	if c, ok := in.at(i); !ok || c != ',' {
		return tpaMatch{}, false
	}

	member, n, ok := in.uicField(i + 1)
	if !ok {
		return tpaMatch{}, false
	}

	i += 1 + n
	if c, ok := in.at(i); !ok || c != closing {
		return tpaMatch{}, false
	}

	return tpaMatch{n: i + 1, number: true, value: group<<16 | member}, true
}

// matchIdent matches a rights identifier in any of the manual's forms: a UIC,
// [name1,name2], [name], and their angle-bracketed forms, name, %Xhex, or *.
func matchIdent(p *tparser, in tpaInput) (tpaMatch, bool) {
	if m, ok := matchUIC(p, in); ok {
		return m, true
	}

	first, ok := in.at(0)
	if !ok {
		return tpaMatch{}, false
	}

	if first == '%' {
		if c, ok := in.at(1); !ok || c != 'X' && c != 'x' {
			return tpaMatch{}, false
		}

		n := in.run(2, isHexChar)
		if n == 0 {
			return tpaMatch{}, false
		}

		value, ok := in.value(2, n, 16)

		return tpaMatch{n: 2 + n, number: true, value: value}, ok
	}

	if first == '*' {
		return tpaMatch{n: 1, number: true, value: 0xFFFFFFFF}, true
	}

	lookup := func(start, n uint32) (uint32, bool) {
		var name strings.Builder

		for k := range n {
			c, _ := in.at(start + k)
			name.WriteByte(c)
		}

		value, status := p.env.IdentifierValue(name.String())

		return value, status == 0
	}

	closing, bracketed := closingBracket(first)
	if !bracketed {
		n := in.run(0, isKeywordChar)
		if n == 0 {
			return tpaMatch{}, false
		}

		value, ok := lookup(0, n)

		return tpaMatch{n: n, number: true, value: value}, ok
	}

	n := in.run(1, isKeywordChar)
	if n == 0 {
		return tpaMatch{}, false
	}

	start, i := uint32(1), 1+n

	if c, ok := in.at(i); ok && c == ',' {
		start = i + 1

		n = in.run(start, isKeywordChar)
		if n == 0 {
			return tpaMatch{}, false
		}

		i = start + n
	}

	if c, ok := in.at(i); !ok || c != closing {
		return tpaMatch{}, false
	}

	value, ok := lookup(start, n)

	return tpaMatch{n: i + 1, number: true, value: value}, ok
}

// match says whether t's type matches the front of the string.
func (p *tparser) match(t *tpaTransition, in tpaInput, ambiguous *bool) (tpaMatch, bool, error) {
	var (
		m  tpaMatch
		ok bool
	)

	switch {
	case !t.has(tpaFlagSymbol):
		var c byte

		if c, ok = in.at(0); ok && c == t.kind {
			m = tpaMatch{n: 1, char: true, value8: c}
		} else {
			ok = false
		}

	case t.kind < tpaKeywords:
		var err error

		m, ok, err = p.matchKeyword(t.kind, in, ambiguous)
		if err != nil {
			return m, false, err
		}

	default:
		matcher, known := tpaSymbolTypes[t.kind]
		if !known {
			return m, false, errTPAInvalidType
		}

		m, ok = matcher(p, in)
	}

	if p.err != nil {
		return m, false, p.err
	}

	return m, ok, nil
}

// matchKeyword matches the keyword whose index is index: the token is what's in
// front of the string in the symbol constituent set, and whether a shorter one
// will do depends on the options.
//
// With TPA$V_ABBREV the keywords of a state are in order, so an abbreviation
// that also abbreviates the next keyword is ambiguous. The strings area has each
// keyword end in -1 and an extra -1 end a state's, so the next keyword is right
// after this one's end. An ambiguous token sets TPA$V_AMBIG, and no keyword
// transition is recognized in the rest of the state.
func (p *tparser) matchKeyword(index byte, in tpaInput, ambiguous *bool) (tpaMatch, bool, error) {
	if *ambiguous {
		return tpaMatch{}, false, nil
	}

	token := in.run(0, isKeywordChar)
	if token == 0 {
		return tpaMatch{}, false, nil
	}

	offset, err := p.bytes(p.keys+2*uint32(index), 2)
	if err != nil {
		return tpaMatch{}, false, err
	}

	at := p.keys + uint32(int32(int16(uint16(offset[0])|uint16(offset[1])<<8)))

	keyword, next, err := p.keywordAt(at)
	if err != nil {
		return tpaMatch{}, false, err
	}

	text := in.text(token)
	if text == keyword {
		return tpaMatch{n: token}, true, nil
	}

	if !strings.HasPrefix(keyword, text) {
		return tpaMatch{}, false, nil
	}

	options, err := p.field(tpaOptions)
	if err != nil {
		return tpaMatch{}, false, err
	}

	switch {
	case options&tpaAbbrfm != 0:
		return tpaMatch{n: token}, true, nil

	case options&tpaAbbrev != 0:
		following, _, err := p.keywordAt(next)
		if err != nil {
			return tpaMatch{}, false, err
		}

		if following != "" && strings.HasPrefix(following, text) {
			*ambiguous = true

			return tpaMatch{}, false, p.setField(tpaOptions, options|tpaAmbig)
		}

		return tpaMatch{n: token}, true, nil

	case options>>24 != 0 && token >= options>>24:
		return tpaMatch{n: token}, true, nil
	}

	return tpaMatch{}, false, nil
}

// keywordAt reads the keyword string at addr, and returns it and the address
// just past its -1.
func (p *tparser) keywordAt(addr uint32) (string, uint32, error) {
	var b strings.Builder

	for {
		c, err := p.mem.LoadByte(p.cpu, addr)
		if err != nil {
			return "", 0, err
		}

		addr++

		if c == 0xFF {
			return b.String(), addr, nil
		}

		b.WriteByte(c)
	}
}
