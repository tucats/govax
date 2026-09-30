package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	tokenByte      = "byte"
	tokenCharacter = "character"
	tokenConstant  = "constant"
	tokenDimension = "dimension"
	tokenFill      = "fill"
	tokenLength    = "length"
	tokenLongword  = "longword"
	tokenPrefix    = "prefix"
	tokenQuadword  = "quadword"
	tokenSigned    = "signed"
	tokenStructure = "structure"
	tokenTag       = "tag"
	tokenUnion     = "union"
	tokenUnsigned  = "unsigned"
	tokenWord      = "word"
)

// parseSDL extracts symbolic constants from a VMS SDL (Structure Definition
// Language) source module, such as reference/vms/lnmdef.sdl. Only the
// subset of SDL that the modules govax reads actually use is understood;
// anything else is reported as an error rather than skipped, so a future
// module using wider SDL syntax can't silently produce a partial or wrong
// table:
//
//   - "module $NAME;" / "end_module ...;" — ignored. So are "ifsymbol
//     NAME;" / "end_ifsymbol;": their contents are always taken, as the
//     MACRO and BLISS versions of a module see them ($OBJRECDEF wraps its
//     zero-length name fields in "ifsymbol not_h_files", which only the C
//     header generator leaves out).
//   - "aggregate NAME {structure|union} prefix P$ [origin FIELD];" ...
//     "end NAME;" — a record layout, whose members are described by the
//     next four items. "origin FIELD" makes every byte offset in the
//     aggregate relative to FIELD's instead of the aggregate's start.
//   - A bitfield member, "FIELD bitfield [length N] [mask] [fill];": N
//     bits (default 1) at the current bit position, which counts from the
//     byte where the current run of bitfields started. A non-fill field
//     defines P$V_FIELD (its bit position), P$S_FIELD when N > 1 (its
//     width), and, with "mask", P$M_FIELD (its already-shifted mask) —
//     SDL's own naming rules.
//   - A byte-aligned member, "FIELD {byte|word|longword|quadword|character}
//     [length N] [dimension N] [signed|unsigned] [fill] [prefix P$]
//     [tag T];": a field of that many bytes (a character field is N bytes,
//     default 1) at the current byte offset, which it then advances. A
//     non-fill field defines P$T_FIELD, its byte offset, where T is B, W,
//     L, Q or T for the five types unless "tag" names another; a character
//     field with N > 0 also defines P$S_FIELD, its length.
//   - A nested aggregate, "MEMBER {structure|union} [TYPE]
//     [signed|unsigned] [fill];" ... "end MEMBER;". A structure's members
//     follow each other; a union's overlay each other, so each starts where
//     the union starts, and bitfields in each union member number from the
//     union's own starting bit position again ($DEVDEF's DEVCHAR and
//     DEVCHAR2 longwords, or a flags word and its bits in $GPSDEF). With a
//     TYPE ("PSC_FLAG union word unsigned") the aggregate occupies exactly
//     that many bytes. A non-fill nested aggregate defines P$T_MEMBER (T is
//     the TYPE's letter, or R when it has none). A nested structure holding
//     only bitfields continues its parent's bit run ($JPIDEF's
//     "JPICTLFLGS structure longword unsigned fill").
//   - "constant NAME equals . ...;" inside an aggregate: "." is the
//     current byte offset.
//   - "constant NAME equals V prefix P$ tag T;" and the list form
//     "constant (A, B, ...) equals V increment I prefix P$ tag T;" — each
//     name becomes P$T_NAME (P$_NAME when T is empty), successive list
//     entries counting up from V by I. $JPIDEF writes the "$" in the tag
//     instead ("prefix JPI tag $C"), which composes the same names. V is a
//     decimal or %x hexadecimal literal, or "OTHER@N": an earlier constant
//     shifted left N bits, as $JPIDEF numbers its item-code lists. Inside
//     an aggregate, the prefix and tag default to the aggregate's prefix
//     and K.
//   - "#NAME = V;" — an SDL local symbol: a named number, used only
//     inside the module and never emitted. $IODEF defines
//     "#fcode_size = 6;" and then sizes bitfields with it
//     ("length #fcode_size", "length 16-#fcode_size"). A bitfield length
//     may be a decimal literal, a local symbol, or two of those joined by
//     "+" or "-".
//
// Comments run from "{" or "/*" to the end of the line. SDL quotes a name
// (e.g. "STRING") only when it collides with an SDL keyword; the quotes are
// not part of the symbol.
func parseSDL(src string) (map[string]uint32, error) {
	splits := strings.Split(src, "\n")
	lines := make([]string, 0, len(splits))

	for _, line := range splits {
		for _, marker := range []string{"{", "/*"} {
			if i := strings.Index(line, marker); i >= 0 {
				line = line[:i]
			}
		}

		lines = append(lines, line)
	}

	out := map[string]uint32{}

	define := func(name string, v uint32) error {
		if prev, ok := out[name]; ok && prev != v {
			return fmt.Errorf("%s redefined with a different value (%d, then %d)", name, prev, v)
		}

		out[name] = v

		return nil
	}

	var (
		// locals holds the module's "#NAME = V" symbols, by name
		// including the "#".
		locals = map[string]int64{}

		// agg is the aggregate being read, or nil outside one.
		agg *sdlAggregate
	)

	for _, stmt := range strings.Split(strings.Join(lines, " "), ";") {
		toks := sdlTokens(stmt)
		if len(toks) == 0 {
			continue
		}

		switch kw := strings.ToLower(toks[0]); {
		case kw == "module" || kw == "end_module" || kw == "ifsymbol" || kw == "end_ifsymbol":
			continue

		case strings.HasPrefix(kw, "#"):
			// #NAME = V
			if len(toks) != 3 || toks[1] != "=" {
				return nil, fmt.Errorf("unsupported local symbol statement %q", stmt)
			}

			v, err := sdlValue(toks[2], nil)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

			locals[kw] = v

		case kw == "aggregate":
			if agg != nil {
				return nil, fmt.Errorf("nested aggregate statement %q", stmt)
			}

			a, err := newSDLAggregate(toks)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

			agg = a

		case kw == "end":
			if agg == nil {
				return nil, fmt.Errorf("%q outside an aggregate", stmt)
			}

			done, err := agg.end()
			if err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

			if done {
				if err := agg.flush(define); err != nil {
					return nil, err
				}

				agg = nil
			}

		case kw == tokenConstant:
			// Outside an aggregate, a constant must spell out its prefix
			// and tag. Inside one ($IODEF's "constant LOOPTEST equals
			// 57344;"), SDL's defaults apply: the aggregate's prefix and
			// the tag K, so IO$K_LOOPTEST.
			defaultPrefix, defaultTag := "", ""
			if agg != nil {
				defaultPrefix, defaultTag = agg.prefix, "K"
			}

			args := toks[1:]

			// "equals ." is the current byte offset, which (like every
			// offset in an aggregate with an origin) is only final once
			// the aggregate ends.
			isOffset := false

			for i := 0; i+1 < len(args); i++ {
				if strings.EqualFold(args[i], "equals") && args[i+1] == "." {
					if agg == nil {
						return nil, fmt.Errorf("%q: \".\" outside an aggregate", stmt)
					}

					args = append(append([]string{}, args[:i+1]...), append([]string{strconv.FormatInt(agg.offset(), 10)}, args[i+2:]...)...)
					isOffset = true
				}
			}

			consts, err := sdlConstant(args, defaultPrefix, defaultTag, func(name string) (uint32, bool) {
				v, ok := out[name]

				return v, ok
			})
			if err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

			for _, c := range consts {
				if isOffset {
					agg.offsets = append(agg.offsets, sdlOffset{name: c.name, offset: int64(c.value)})

					continue
				}

				if err := define(c.name, c.value); err != nil {
					return nil, err
				}
			}

		case agg != nil:
			if err := agg.member(toks, locals, define); err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

		default:
			return nil, fmt.Errorf("unsupported SDL statement %q", stmt)
		}
	}

	if agg != nil {
		return nil, fmt.Errorf("unterminated aggregate (prefix %s)", agg.prefix)
	}

	return out, nil
}

// sdlFrame is one open aggregate in an SDL record layout: the aggregate
// statement itself, or a nested structure or union member of it.
type sdlFrame struct {
	union bool

	// start is the byte offset where the aggregate starts, and startBits
	// the bit position (within the bitfield run open at start) where it
	// starts.
	start     int64
	startBits uint32

	// A structure's next free byte offset, and the bits already used in
	// the bitfield run starting at cur.
	cur  int64
	bits uint32

	// A union's furthest member end, as a byte offset, and whether every
	// member so far held only bitfields; if so, endBits is the most bits
	// any member used.
	end      int64
	endBits  uint32
	bitsOnly bool

	// size is the byte size a typed aggregate ("union word unsigned")
	// declares, or 0.
	size int64
}

// sdlOffset is a byte-offset symbol waiting for its aggregate to end, so an
// "origin" can be applied to it.
type sdlOffset struct {
	name   string
	offset int64
}

// sdlAggregate is the state of one top-level aggregate while its members
// are read.
type sdlAggregate struct {
	prefix string

	// origin is the field every offset is relative to (see parseSDL), and
	// originOffset its offset once that field is seen.
	origin       string
	originOffset int64
	originSeen   bool

	stack   []*sdlFrame
	offsets []sdlOffset
}

// newSDLAggregate starts an aggregate from its statement's tokens:
// "aggregate NAME {structure|union} prefix P$ [origin FIELD]".
func newSDLAggregate(toks []string) (*sdlAggregate, error) {
	if len(toks) != 5 && len(toks) != 7 {
		return nil, fmt.Errorf("unsupported aggregate statement")
	}

	kind := strings.ToLower(toks[2])
	if (kind != tokenStructure && kind != tokenUnion) || !strings.EqualFold(toks[3], tokenPrefix) {
		return nil, fmt.Errorf("unsupported aggregate statement")
	}

	a := &sdlAggregate{prefix: toks[4]}

	if len(toks) == 7 {
		if !strings.EqualFold(toks[5], "origin") {
			return nil, fmt.Errorf("unsupported aggregate keyword %q", toks[5])
		}

		a.origin = toks[6]
	}

	a.stack = []*sdlFrame{{union: kind == tokenUnion, bitsOnly: true}}

	return a, nil
}

func (a *sdlAggregate) top() *sdlFrame { return a.stack[len(a.stack)-1] }

// position returns where the next member of the innermost aggregate
// starts: its byte offset and the bit position within the bitfield run
// open there.
func (a *sdlAggregate) position() (int64, uint32) {
	f := a.top()
	if f.union {
		return f.start, f.startBits
	}

	return f.cur, f.bits
}

// offset returns the byte offset of the next byte-aligned member: past
// any bitfields in the run open at the current position.
func (a *sdlAggregate) offset() int64 {
	off, bits := a.position()

	return off + int64(bits+7)/8
}

// advance records that a member of the innermost aggregate ended at byte
// offset end, followed by bits more bits.
func (a *sdlAggregate) advance(end int64, bits uint32, bitsOnly bool) {
	f := a.top()
	if !f.union {
		f.cur, f.bits = end, bits

		return
	}

	if !bitsOnly {
		f.bitsOnly = false
	}

	if full := end + int64(bits+7)/8; full > f.end {
		f.end = full
	}

	if bits > f.endBits {
		f.endBits = bits
	}
}

// member reads one member statement of the aggregate.
func (a *sdlAggregate) member(toks []string, locals map[string]int64, define func(string, uint32) error) error {
	if len(toks) < 2 {
		return fmt.Errorf("unsupported aggregate member")
	}

	name, kind := toks[0], strings.ToLower(toks[1])

	switch kind {
	case "bitfield":
		off, pos := a.position()

		width, err := sdlBitfield(toks, a.prefix, pos, locals, define)
		if err != nil {
			return err
		}

		if pos+width > 32 {
			return fmt.Errorf("bitfields exceed 32 bits")
		}

		a.advance(off, pos+width, true)

		return nil

	case tokenStructure, tokenUnion:
		return a.push(name, kind == tokenUnion, toks[2:])

	case tokenByte, tokenWord, tokenLongword, tokenQuadword, tokenCharacter:
	default:
		return fmt.Errorf("unsupported aggregate member type %q", toks[1])
	}

	size := sdlTypeSizes[kind]
	count := int64(1)
	prefix, tag := a.prefix, sdlTypeTags[kind]

	var fill bool

	for i := 2; i < len(toks); i++ {
		switch kw := strings.ToLower(toks[i]); kw {
		case tokenSigned, "unsigned":
		case tokenFill:
			fill = true
		case tokenLength, tokenDimension, tokenPrefix, tokenTag:
			if i+1 >= len(toks) {
				return fmt.Errorf("%q without a value", toks[i])
			}

			arg := toks[i+1]
			i++

			switch kw {
			case tokenLength, tokenDimension:
				n, err := strconv.ParseInt(arg, 10, 64)
				if err != nil || n < 0 || kw == tokenLength && kind != tokenCharacter {
					return fmt.Errorf("bad %s %q", kw, arg)
				}

				if kw == tokenLength {
					size = n
				} else {
					count = n
				}
			case tokenPrefix:
				prefix = arg
			default:
				tag = arg
			}
		default:
			return fmt.Errorf("unsupported field keyword %q", toks[i])
		}
	}

	off := a.offset()

	if strings.EqualFold(name, a.origin) {
		a.originOffset, a.originSeen = off, true
	}

	if !fill {
		a.offsets = append(a.offsets, sdlOffset{name: prefix + tag + "_" + name, offset: off})

		if kind == tokenCharacter && size > 0 {
			if err := define(prefix+"S_"+name, uint32(size*count)); err != nil {
				return err
			}
		}
	}

	a.advance(off+size*count, 0, false)

	return nil
}

// push opens a nested structure or union member: "MEMBER {structure|union}
// [TYPE] [signed|unsigned] [fill]", whose keywords after the kind are
// rest.
func (a *sdlAggregate) push(name string, union bool, rest []string) error {
	var (
		size int64
		tag  = "R"
		fill bool
	)

	for _, t := range rest {
		switch kw := strings.ToLower(t); kw {
		case tokenSigned, tokenUnsigned:
		case tokenFill:
			fill = true
		default:
			n, ok := sdlTypeSizes[kw]
			if !ok || kw == tokenCharacter {
				return fmt.Errorf("unsupported nested aggregate keyword %q", t)
			}

			size, tag = n, sdlTypeTags[kw]
		}
	}

	off, bits := a.position()

	if !fill {
		a.offsets = append(a.offsets, sdlOffset{name: a.prefix + tag + "_" + name, offset: off + int64(bits+7)/8})
	}

	a.stack = append(a.stack, &sdlFrame{
		union: union, start: off, startBits: bits, cur: off, bits: bits,
		end: off, bitsOnly: true, size: size,
	})

	return nil
}

// end closes the innermost open aggregate, reporting whether that was the
// top-level one.
func (a *sdlAggregate) end() (bool, error) {
	f := a.top()
	a.stack = a.stack[:len(a.stack)-1]

	if len(a.stack) == 0 {
		return true, nil
	}

	// Where the closed aggregate ends: a typed one is exactly its type's
	// size; a union at its furthest member; a structure at its location
	// counter. One holding only bitfields leaves its bit run open for the
	// parent to continue.
	var (
		end      int64
		bits     uint32
		bitsOnly bool
	)

	switch {
	case f.size > 0:
		end = f.start + int64(f.startBits+7)/8 + f.size
	case f.union && f.bitsOnly:
		end, bits, bitsOnly = f.start, f.endBits, true
	case f.union:
		end = f.end
	case f.cur == f.start:
		end, bits, bitsOnly = f.start, f.bits, true
	default:
		end = f.cur + int64(f.bits+7)/8
	}

	a.advance(end, bits, bitsOnly)

	return false, nil
}

// flush defines the aggregate's byte-offset symbols, relative to its
// origin field if it has one.
func (a *sdlAggregate) flush(define func(string, uint32) error) error {
	base := int64(0)

	if a.origin != "" {
		if !a.originSeen {
			return fmt.Errorf("origin field %s is not in aggregate (prefix %s)", a.origin, a.prefix)
		}

		base = a.originOffset
	}

	for _, o := range a.offsets {
		if err := define(o.name, uint32(o.offset-base)); err != nil {
			return err
		}
	}

	return nil
}

// sdlTypeSizes and sdlTypeTags give each SDL field type's size in bytes
// (a character field's default length) and the letter SDL puts in its
// field names (GPS$B_ALIGN, GPS$W_FLAGS, GPS$L_ALLOC, GPS$T_NAME).
var (
	sdlTypeSizes = map[string]int64{tokenByte: 1, tokenWord: 2, tokenLongword: 4, tokenQuadword: 8, tokenCharacter: 1}
	sdlTypeTags  = map[string]string{tokenByte: "B", tokenWord: "W", tokenLongword: "L", tokenQuadword: "Q", tokenCharacter: "T"}
)

// sdlTokens splits one SDL statement into tokens, treating "(", ")" and ","
// sdlTokens splits one SDL statement into tokens, treating "(", ")" and ","
// as tokens of their own and stripping the quotes from a quoted name.
func sdlTokens(stmt string) []string {
	r := strings.NewReplacer("(", " ( ", ")", " ) ", ",", " , ")

	toks := strings.Fields(r.Replace(stmt))
	for i, t := range toks {
		if len(t) >= 2 && strings.HasPrefix(t, `"`) && strings.HasSuffix(t, `"`) {
			toks[i] = t[1 : len(t)-1]
		}
	}

	return toks
}

// sdlBitfield handles one "FIELD bitfield [length N] [mask] [fill]" member,
// defining its symbols at bit position pos and returning its width. N may
// use the module's local symbols (see sdlLength).
func sdlBitfield(toks []string, prefix string, pos uint32, locals map[string]int64, define func(string, uint32) error) (uint32, error) {
	if len(toks) < 2 || !strings.EqualFold(toks[1], "bitfield") {
		return 0, fmt.Errorf("unsupported aggregate member")
	}

	name := toks[0]
	width := uint32(1)

	var mask, fill bool

	for i := 2; i < len(toks); i++ {
		switch strings.ToLower(toks[i]) {
		case tokenLength:
			if i+1 >= len(toks) {
				return 0, fmt.Errorf("length without a value")
			}

			n, err := sdlLength(toks[i+1], locals)
			if err != nil || n <= 0 || n > 32 {
				return 0, fmt.Errorf("bad bitfield length %q", toks[i+1])
			}

			width = uint32(n)
			i++

		case "mask":
			mask = true

		case tokenFill:
			fill = true

		default:
			return 0, fmt.Errorf("unsupported bitfield keyword %q", toks[i])
		}
	}

	if fill {
		return width, nil
	}

	if err := define(prefix+"V_"+name, pos); err != nil {
		return 0, err
	}

	if width > 1 {
		if err := define(prefix+"S_"+name, width); err != nil {
			return 0, err
		}
	}

	if mask {
		m := uint32((uint64(1)<<width - 1) << pos)
		if err := define(prefix+"M_"+name, m); err != nil {
			return 0, err
		}
	}

	return width, nil
}

type sdlConst struct {
	name  string
	value uint32
}

// sdlConstant handles the tokens following "constant": a single name or a
// parenthesised name list, then "equals V", optionally "increment I", and
// "prefix P$" and "tag T". The prefix and tag are required unless
// defaultPrefix is given (a constant inside an aggregate), when either
// may be omitted and defaultPrefix/defaultTag are used.
func sdlConstant(toks []string, defaultPrefix, defaultTag string, lookup func(string) (uint32, bool)) ([]sdlConst, error) {
	var names []string

	i := 0

	if i < len(toks) && toks[i] == "(" {
		for i++; i < len(toks) && toks[i] != ")"; i++ {
			if toks[i] != "," {
				names = append(names, toks[i])
			}
		}

		if i >= len(toks) {
			return nil, fmt.Errorf("unterminated constant list")
		}

		i++
	} else if i < len(toks) {
		names = append(names, toks[i])
		i++
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("constant without a name")
	}

	var (
		value, increment int64 = 0, 1
		prefix, tag      string
		haveValue        bool
		havePrefix       bool
		haveTag          bool
	)

	for ; i < len(toks); i++ {
		kw := strings.ToLower(toks[i])

		if i+1 >= len(toks) {
			return nil, fmt.Errorf("%q without a value", toks[i])
		}

		arg := toks[i+1]
		i++

		// A value may be parenthesized: "equals (%B0000)", as $PRTDEF
		// writes its protection codes.
		if arg == "(" && i+2 < len(toks) && toks[i+2] == ")" {
			arg = toks[i+1]
			i += 2
		}

		switch kw {
		case "equals", "increment":
			n, err := sdlValue(arg, lookup)
			if err != nil {
				return nil, fmt.Errorf("bad %s value %q: %w", kw, arg, err)
			}

			if kw == "equals" {
				value, haveValue = n, true
			} else {
				increment = n
			}

		case tokenPrefix:
			prefix, havePrefix = arg, true

		case tokenTag:
			tag, haveTag = arg, true

		default:
			return nil, fmt.Errorf("unsupported constant keyword %q", toks[i-1])
		}
	}

	if defaultPrefix != "" {
		if !havePrefix {
			prefix, havePrefix = defaultPrefix, true
		}

		if !haveTag {
			tag, haveTag = defaultTag, true
		}
	}

	// SDL has default tags for top-level constants too, but $LNMDEF always
	// spells them out; requiring them keeps this parser from having to
	// guess.
	if !haveValue || !havePrefix || !haveTag {
		return nil, fmt.Errorf("constant needs explicit equals, prefix, and tag")
	}

	out := make([]sdlConst, 0, len(names))

	for n, name := range names {
		sym := prefix + tag + "_" + name
		out = append(out, sdlConst{name: sym, value: uint32(value + int64(n)*increment)})
	}

	return out, nil
}

// sdlLength evaluates a bitfield length: a decimal literal or a "#NAME"
// local symbol, or two of those joined by "+" or "-" ("16-#fcode_size").
// SDL allows fuller expressions; this is as much as the modules govax
// reads use, and anything else is an error.
func sdlLength(arg string, locals map[string]int64) (int64, error) {
	operand := func(s string) (int64, error) {
		if strings.HasPrefix(s, "#") {
			v, ok := locals[strings.ToLower(s)]
			if !ok {
				return 0, fmt.Errorf("undefined local symbol %s", s)
			}

			return v, nil
		}

		return strconv.ParseInt(s, 10, 64)
	}

	// Split at the first "+" or "-" after the first character, so a
	// leading sign (never used by SDL lengths) isn't taken for an
	// operator.
	if i := strings.IndexAny(arg[min(1, len(arg)):], "+-"); i >= 0 {
		i++

		a, err := operand(arg[:i])
		if err != nil {
			return 0, err
		}

		b, err := operand(arg[i+1:])
		if err != nil {
			return 0, err
		}

		if arg[i] == '+' {
			return a + b, nil
		}

		return a - b, nil
	}

	return operand(arg)
}

// sdlValue evaluates an SDL constant value: a decimal literal, a %x
// hexadecimal or %b binary literal (the page protection codes in
// $PRTDEF are written in binary), NAME@N (the earlier constant NAME
// shifted left N bits), or the full name of an earlier constant ($BRKDEF's
// "MAXSENDTYPE Equals BRK$C_ALLTERMS").
func sdlValue(arg string, lookup func(string) (uint32, bool)) (int64, error) {
	if name, shift, ok := strings.Cut(arg, "@"); ok {
		v, found := lookup(name)
		if !found {
			return 0, fmt.Errorf("undefined constant %s", name)
		}

		n, err := strconv.ParseUint(shift, 10, 5)
		if err != nil {
			return 0, fmt.Errorf("bad shift %q", shift)
		}

		return int64(v) << n, nil
	}

	if hex, ok := strings.CutPrefix(strings.ToLower(arg), "%x"); ok {
		return strconv.ParseInt(hex, 16, 64)
	}

	if bin, ok := strings.CutPrefix(strings.ToLower(arg), "%b"); ok {
		return strconv.ParseInt(bin, 2, 64)
	}

	if lookup != nil && strings.ContainsRune(arg, '$') {
		if v, found := lookup(strings.ToUpper(arg)); found {
			return int64(v), nil
		}

		return 0, fmt.Errorf("undefined constant %s", arg)
	}

	return strconv.ParseInt(arg, 10, 64)
}
