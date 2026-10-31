package format

import (
	"fmt"
	"strconv"
	"strings"
)

// Code is an edit descriptor's kind.
type Code int

// The edit descriptors. The data descriptors come first, I to A, so
// Code.Data can tell them by their place.
const (
	I Code = iota // Iw[.m]: an integer in decimal
	O             // Ow[.m]: in octal
	Z             // Zw[.m]: in hexadecimal
	F             // Fw.d: a real, fixed point
	E             // Ew.d[Ee]: a real with an exponent
	D             // Dw.d: the same, D for E
	G             // Gw.d[Ee]: F or E, by the value's size
	L             // Lw: a logical
	A             // Aw: characters

	Text   // 'text' or nH text: Item.Text
	X      // nX: skip n columns
	T      // Tn: go to column n
	TL     // TLn: go left n columns
	TR     // TRn: go right n columns
	P      // kP: the scale factor k
	Q      // Q: the characters left in an input record
	S      // S: the plus sign as the processor chooses
	SP     // SP: plus signs written
	SS     // SS: plus signs not written
	BN     // BN: blanks in numeric input ignored
	BZ     // BZ: blanks in numeric input zeros
	Slash  // /: the record ends
	Colon  // :: the format ends if no items are left
	Dollar // $: no carriage return after the record
	Group  // r(...): Item.Items, Repeat times
)

// names are the codes as a format writes them.
var names = [...]string{
	I: "I", O: "O", Z: "Z", F: "F", E: "E", D: "D", G: "G", L: "L", A: "A",
	Text: "'", X: "X", T: "T", TL: "TL", TR: "TR", P: "P", Q: "Q", S: "S", SP: "SP", SS: "SS",
	BN: "BN", BZ: "BZ", Slash: "/", Colon: ":", Dollar: "$", Group: "(",
}

func (c Code) String() string { return names[c] }

// Data reports whether c takes an item from the I/O list.
func (c Code) Data() bool { return c <= A }

// Value is one of a descriptor's numbers: N, or with VFE set the
// variable format expression Format.VFEs[VFE-1], whose value the program
// computes. Set is clear when the number was left out.
type Value struct {
	N   int
	VFE int
	Set bool
}

func (v Value) String() string {
	switch {
	case v.VFE > 0:
		return "<" + strconv.Itoa(v.VFE) + ">"
	case v.Set:
		return strconv.Itoa(v.N)
	}

	return ""
}

// Item is one edit descriptor, or a group of them.
type Item struct {
	Code Code

	// Repeat is a data descriptor's or a group's repeat count, 1 when it
	// is left out.
	Repeat Value

	// W, M, D, and E are a data descriptor's width, minimum digits,
	// digits after the point, and exponent digits, as it has them: Iw.m,
	// Fw.d, Ew.dEe. W left out is Table 8-4's default width.
	W, M, D, E Value

	// N is the number of X, T, TL, TR, and P (the scale factor, which may
	// be negative).
	N Value

	// Text is a character constant's text, its doubled apostrophes made
	// one, or an H's.
	Text string

	// Items are a group's.
	Items []*Item

	// Offset is where the item starts in the format's text.
	Offset int
}

// VFE is a variable format expression, <expr>: Text is what is between
// the angle brackets, starting at Offset in the format's text.
type VFE struct {
	Text   string
	Offset int
}

// Format is a format specification read by Parse.
type Format struct {
	Items []*Item
	VFEs  []VFE
}

// Options say what kind of format is read.
type Options struct {
	// RunTime is set for a run-time format, which may have no variable
	// format expression and may have anything after its last parenthesis
	// (VMS ignores it); a FORMAT statement may have only blanks there.
	RunTime bool
}

// Error is what's wrong with a format: Text, about the character at
// Offset in the format's text.
type Error struct {
	Offset int
	Text   string
}

func (e *Error) Error() string { return fmt.Sprintf("at %d: %s", e.Offset, e.Text) }

// The limits on a format's numbers (section 8.2, and what VMS accepted).
const (
	maxWidth  = 32767 // a repeat count, a width, and X's, T's, TL's, TR's, and H's counts
	maxDigits = 255   // m, d, and e
	minScale  = -128  // kP
	maxScale  = 127

	// maxVFEs is how many variable format expressions a FORMAT may have:
	// FTNRTL's parsed format keeps an expression's number where the value
	// goes, and m, d, and e are bytes.
	maxVFEs = 255
)

// MaxDepth is how deeply groups may nest inside the format's own
// parentheses. VMS accepted 8 and refused 16 (testdata/for80, cases 405
// and 406); where between them its limit is wasn't probed, so this is
// 8, the most it is known to take.
const MaxDepth = 8

// Parse reads a format specification, its text from its opening
// parenthesis.
func Parse(text string, opts Options) (*Format, error) {
	p := &parser{text: text, opts: opts}

	f, err := p.parse()
	if err != nil {
		return nil, err
	}

	return f, nil
}

// parser is one format being read: text, and the offset of the next
// character to look at.
type parser struct {
	text string
	pos  int
	opts Options
	vfes []VFE
}

// failure is how the parser leaves a format it finds wrong: parse
// recovers it.
type failure struct{ err *Error }

// fail stops the parse with an error about the character at offset.
func (p *parser) fail(offset int, format string, args ...any) {
	panic(failure{&Error{Offset: offset, Text: fmt.Sprintf(format, args...)}})
}

func (p *parser) parse() (f *Format, err error) {
	defer func() {
		if r := recover(); r != nil {
			fail, ok := r.(failure)
			if !ok {
				panic(r)
			}

			f, err = nil, fail.err
		}
	}()

	if p.next() != '(' {
		p.fail(p.pos, "a format must start with (")
	}

	p.pos++
	items := p.list(0)

	if !p.opts.RunTime {
		if p.next() != 0 {
			p.fail(p.pos, "the format ends at its last parenthesis, and %q follows it", p.text[p.pos:])
		}
	}

	return &Format{Items: items, VFEs: p.vfes}, nil
}

// next is the next character that isn't a blank, made a capital, or 0 at
// the text's end; pos is left at it.
func (p *parser) next() byte {
	for p.pos < len(p.text) && (p.text[p.pos] == ' ' || p.text[p.pos] == '\t') {
		p.pos++
	}

	if p.pos >= len(p.text) {
		return 0
	}

	c := p.text[p.pos]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}

	return c
}

// list reads the items of a group, depth groups deep, up to and past its
// closing parenthesis.
func (p *parser) list(depth int) []*Item {
	var items []*Item

	// apart is set when the item just read needs a comma (or one of the
	// items that need none) before the next.
	apart := false

	for {
		start := p.pos

		switch c := p.next(); c {
		case 0:
			p.fail(p.pos, "the format has no closing parenthesis")

		case ')':
			p.pos++

			return items

		case ',':
			p.pos++
			apart = false

		case '/', ':', '$':
			p.pos++
			apart = false
			items = append(items, &Item{Code: map[byte]Code{'/': Slash, ':': Colon, '$': Dollar}[c], Offset: p.pos - 1})

		case '\'':
			items = append(items, &Item{Code: Text, Text: p.quoted(), Offset: start})
			apart = false

		default:
			if apart && !p.atHollerith() {
				p.fail(p.pos, "a comma is needed between two edit descriptors")
			}

			item, free := p.item(depth)
			items = append(items, item)
			apart = !free
		}
	}
}

// atHollerith reports whether an H constant starts at pos: digits, then
// H, blanks allowed among them.
func (p *parser) atHollerith() bool {
	save := p.pos
	defer func() { p.pos = save }()

	n, _ := p.number()

	return n.Set && p.next() == 'H'
}

// quoted reads a character constant, from its opening apostrophe: its
// text, with each doubled apostrophe one.
func (p *parser) quoted() string {
	start := p.pos
	p.pos++

	var b strings.Builder

	for {
		if p.pos >= len(p.text) {
			p.fail(start, "the character constant has no closing apostrophe")
		}

		c := p.text[p.pos]
		p.pos++

		if c != '\'' {
			b.WriteByte(c)

			continue
		}

		if p.pos < len(p.text) && p.text[p.pos] == '\'' {
			b.WriteByte('\'')
			p.pos++

			continue
		}

		break
	}

	if b.Len() == 0 {
		p.fail(start, "a character constant in a format must hold at least one character")
	}

	return b.String()
}

// item reads one edit descriptor or group, which doesn't start with a
// comma, slash, colon, dollar sign, or apostrophe. free reports whether
// the next item may follow it without a comma (after an H's text, or
// kP).
func (p *parser) item(depth int) (item *Item, free bool) {
	start := p.pos

	// A number first, perhaps: a repeat count, or the count of X, H, or
	// P, or P's scale factor, which alone may have a sign.
	sign := 0

	if c := p.next(); c == '+' || c == '-' {
		sign = 1
		if c == '-' {
			sign = -1
		}

		p.pos++
	}

	count, countAt := p.number()

	c := p.next()
	if c == 0 {
		p.fail(p.pos, "the format has no closing parenthesis")
	}

	if sign != 0 && c != 'P' {
		p.fail(start, "only a scale factor (kP) may have a sign")
	}

	letterAt := p.pos

	switch c {
	case 'P':
		p.pos++

		k := count
		if !k.Set && k.VFE == 0 {
			k = Value{N: 0, Set: true} // P alone is 0P
		}

		if sign < 0 {
			k.N = -k.N
		}

		if k.VFE == 0 && (k.N < minScale || k.N > maxScale) {
			p.fail(countAt, "a scale factor must be from %d to %d", minScale, maxScale)
		}

		return &Item{Code: P, N: k, Offset: start}, true

	case 'X':
		p.pos++

		n := count
		if !n.Set && n.VFE == 0 {
			n = Value{N: 1, Set: true} // X alone is 1X
		}

		return &Item{Code: X, N: n, Offset: start}, false

	case 'H':
		return p.hollerith(start, count, countAt), true

	case '(':
		if depth >= MaxDepth {
			p.fail(p.pos, "groups are nested more than %d deep", MaxDepth)
		}

		p.pos++
		repeat := p.repeat(count, countAt)

		return &Item{Code: Group, Repeat: repeat, Items: p.list(depth + 1), Offset: start}, false

	case '\'':
		p.fail(start, "a character constant can't have a repeat count")
	}

	code, ok := p.code()
	if !ok {
		p.fail(letterAt, "%q is not an edit descriptor", p.text[letterAt:letterAt+1])
	}

	item = &Item{Code: code, Offset: start}

	switch code {
	case T, TL, TR:
		if count.Set || count.VFE > 0 {
			p.fail(start, "%s can't have a repeat count", code)
		}

		n, at := p.number()
		if !n.Set && n.VFE == 0 {
			p.fail(p.pos, "%s needs a column count, as in %s5", code, code)
		}

		if n.VFE == 0 && n.N > maxWidth {
			p.fail(at, "a column count must be at most %d", maxWidth)
		}

		item.N = n

	case Q, S, SP, SS, BN, BZ:
		if count.Set || count.VFE > 0 {
			p.fail(start, "%s can't have a repeat count", code)
		}

	default:
		item.Repeat = p.repeat(count, countAt)
		p.fields(item)
	}

	return item, false
}

// hollerith reads nH and its n characters, H at pos.
func (p *parser) hollerith(start int, count Value, countAt int) *Item {
	switch {
	case count.VFE > 0:
		p.fail(countAt, "an H constant's count must be a number")
	case !count.Set:
		p.fail(p.pos, "H needs a count of the characters after it, as in 3HABC")
	case count.N == 0:
		p.fail(countAt, "an H constant must hold at least one character")
	}

	p.pos++

	if p.pos+count.N > len(p.text) {
		p.fail(start, "the H constant has %d characters, and the format ends before them", count.N)
	}

	text := p.text[p.pos : p.pos+count.N]
	p.pos += count.N

	return &Item{Code: Text, Text: text, Offset: start}
}

// repeat is a repeat count read before a data descriptor or group: 1 if
// none was.
func (p *parser) repeat(count Value, at int) Value {
	switch {
	case count.VFE > 0:
		return count
	case !count.Set:
		return Value{N: 1}
	case count.N == 0:
		p.fail(at, "a repeat count must be at least 1")
	case count.N > maxWidth:
		p.fail(at, "a repeat count must be at most %d", maxWidth)
	}

	return count
}

// code reads a descriptor's letters, from pos, past the ones that are
// some other descriptor's prefix (T, S, B).
func (p *parser) code() (Code, bool) {
	c := p.next()
	p.pos++

	single := map[byte]Code{'I': I, 'O': O, 'Z': Z, 'F': F, 'E': E, 'D': D, 'G': G, 'L': L, 'A': A, 'Q': Q}
	if code, ok := single[c]; ok {
		return code, true
	}

	second := p.next()

	switch {
	case c == 'T' && second == 'L':
		p.pos++

		return TL, true
	case c == 'T' && second == 'R':
		p.pos++

		return TR, true
	case c == 'T':
		return T, true
	case c == 'S' && second == 'P':
		p.pos++

		return SP, true
	case c == 'S' && second == 'S':
		p.pos++

		return SS, true
	case c == 'S':
		return S, true
	case c == 'B' && second == 'N':
		p.pos++

		return BN, true
	case c == 'B' && second == 'Z':
		p.pos++

		return BZ, true
	}

	p.pos--

	return 0, false
}

// fields reads a data descriptor's numbers after its letter: w, then .m
// for I, O, and Z, .d for the reals, and Ee for E and G. Each may be left
// out, the whole field list (I alone, a default width) or, for E and G,
// the exponent; but F, E, D, and G with a width need .d too.
func (p *parser) fields(item *Item) {
	w, at := p.number()
	if !w.Set && w.VFE == 0 {
		return
	}

	if w.VFE == 0 && w.N > maxWidth {
		p.fail(at, "a field width must be at most %d", maxWidth)
	}

	item.W = w

	switch item.Code {
	case L, A:
		return

	case I, O, Z:
		if p.next() == '.' {
			p.pos++
			item.M = p.digits("m", "the fewest digits")
		}

		return
	}

	if p.next() != '.' {
		p.fail(p.pos, "%s%s needs the digits after the point too, as in %s%s.2", item.Code, w, item.Code, w)
	}

	p.pos++
	item.D = p.digits("d", "the digits after the point")

	if (item.Code == E || item.Code == G) && p.next() == 'E' {
		p.pos++

		// E with no digits after it is taken as no exponent field, as VMS
		// takes it.
		if e, at := p.number(); e.Set || e.VFE > 0 {
			if e.VFE == 0 && (e.N < 1 || e.N > maxDigits) {
				p.fail(at, "an exponent's digits must be from 1 to %d", maxDigits)
			}

			item.E = e
		}
	}
}

// digits reads m or d, a number from 0 to 255 that must be there.
func (p *parser) digits(name, what string) Value {
	v, at := p.number()

	switch {
	case !v.Set && v.VFE == 0:
		p.fail(p.pos, "%s, %s, is missing after the point", name, what)
	case v.VFE == 0 && v.N > maxDigits:
		p.fail(at, "%s must be at most %d", name, maxDigits)
	}

	return v
}

// number reads an unsigned number, its digits perhaps with blanks among
// them, or a variable format expression; at is where it starts. A number
// too large is held at maxWidth+1, which every limit refuses. A number
// that isn't there is left unset.
func (p *parser) number() (v Value, at int) {
	c := p.next()
	at = p.pos

	if c == '<' {
		return p.vfe(), at
	}

	if c < '0' || c > '9' {
		return Value{}, at
	}

	n := 0

	for c >= '0' && c <= '9' {
		if n <= maxWidth {
			n = n*10 + int(c-'0')
		}

		p.pos++
		c = p.next()
	}

	return Value{N: min(n, maxWidth+1), Set: true}, at
}

// vfe reads <expr>, from its <.
func (p *parser) vfe() Value {
	start := p.pos

	if p.opts.RunTime {
		p.fail(start, "a run-time format can't have a variable format expression")
	}

	end := strings.IndexByte(p.text[start:], '>')
	if end < 0 {
		p.fail(start, "the variable format expression has no closing >")
	}

	text := p.text[start+1 : start+end]
	if strings.TrimSpace(text) == "" {
		p.fail(start, "the variable format expression is empty")
	}

	if len(p.vfes) == maxVFEs {
		p.fail(start, "a format can have at most %d variable format expressions", maxVFEs)
	}

	p.vfes = append(p.vfes, VFE{Text: text, Offset: start + 1})
	p.pos = start + end + 1

	return Value{VFE: len(p.vfes)}
}
