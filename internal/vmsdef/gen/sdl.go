package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseSDL extracts symbolic constants from a VMS SDL (Structure Definition
// Language) source module, such as reference/vms/lnmdef.sdl. Only the small
// subset of SDL that $LNMDEF actually uses is understood; anything else is
// reported as an error rather than skipped, so a future module using wider
// SDL syntax can't silently produce a partial or wrong table:
//
//   - "module $NAME;" / "end_module ...;" — ignored.
//   - "aggregate NAME structure prefix P$;" ... "end NAME;" — a block of
//     consecutive bitfields, numbered from bit 0. Each
//     "FIELD bitfield [length N] [mask] [fill];" advances the bit position
//     by N (default 1). A non-fill field defines P$V_FIELD (its bit
//     position), P$S_FIELD when N > 1 (its width), and, with "mask",
//     P$M_FIELD (its already-shifted mask) — SDL's own naming rules.
//   - "aggregate NAME union prefix P$;" whose members are nested
//     "MEMBER structure [fill];" ... "end MEMBER;" blocks of bitfields, as
//     $DEVDEF uses for its DEVCHAR and DEVCHAR2 longwords. Union members
//     overlay each other, so each nested structure numbers its bitfields
//     from bit 0 again, all under the union's prefix. A structure
//     aggregate may nest one the same way ($JPIDEF's
//     "JPICTLFLGS structure longword unsigned fill;"), continuing from the
//     current bit position, since structure members follow each other.
//   - "constant NAME equals V prefix P$ tag T;" and the list form
//     "constant (A, B, ...) equals V increment I prefix P$ tag T;" — each
//     name becomes P$T_NAME (P$_NAME when T is empty), successive list
//     entries counting up from V by I. $JPIDEF writes the "$" in the tag
//     instead ("prefix JPI tag $C"), which composes the same names. V is a
//     decimal or %x hexadecimal literal, or "OTHER@N": an earlier constant
//     shifted left N bits, as $JPIDEF numbers its item-code lists.
//
// Comments run from "{" or "/*" to the end of the line. SDL quotes a name
// (e.g. "STRING") only when it collides with an SDL keyword; the quotes are
// not part of the symbol.
func parseSDL(src string) (map[string]uint32, error) {
	var lines []string

	for _, line := range strings.Split(src, "\n") {
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
		inAggregate bool
		isUnion     bool // the aggregate is a union of nested structures
		inMember    bool // inside one of a union's nested structures
		aggPrefix   string
		bitPos      uint32
	)

	for _, stmt := range strings.Split(strings.Join(lines, " "), ";") {
		toks := sdlTokens(stmt)
		if len(toks) == 0 {
			continue
		}

		switch kw := strings.ToLower(toks[0]); {
		case kw == "module" || kw == "end_module":
			continue

		case kw == "aggregate":
			// aggregate NAME {structure|union} prefix P$
			if inAggregate {
				return nil, fmt.Errorf("nested aggregate %q is not supported", stmt)
			}

			kind := ""
			if len(toks) == 5 {
				kind = strings.ToLower(toks[2])
			}

			if (kind != "structure" && kind != "union") || !strings.EqualFold(toks[3], "prefix") {
				return nil, fmt.Errorf("unsupported aggregate statement %q", stmt)
			}

			inAggregate, isUnion, inMember, aggPrefix, bitPos = true, kind == "union", false, toks[4], 0

		case kw == "end":
			switch {
			case inMember:
				inMember = false
			case inAggregate:
				inAggregate = false
			default:
				return nil, fmt.Errorf("%q outside an aggregate", stmt)
			}

		case inAggregate && !inMember && (isUnion || len(toks) >= 2 && strings.EqualFold(toks[1], "structure")):
			// MEMBER structure [longword] [unsigned] [fill]
			if len(toks) < 2 || !strings.EqualFold(toks[1], "structure") {
				return nil, fmt.Errorf("unsupported union member %q", stmt)
			}

			for _, t := range toks[2:] {
				switch strings.ToLower(t) {
				case "fill", "longword", "unsigned":
				default:
					return nil, fmt.Errorf("unsupported nested structure keyword %q in %q", t, stmt)
				}
			}

			inMember = true
			if isUnion {
				bitPos = 0
			}

		case kw == "constant":
			if inAggregate {
				return nil, fmt.Errorf("constant inside an aggregate is not supported: %q", stmt)
			}

			consts, err := sdlConstant(toks[1:], func(name string) (uint32, bool) {
				v, ok := out[name]

				return v, ok
			})
			if err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

			for _, c := range consts {
				if err := define(c.name, c.value); err != nil {
					return nil, err
				}
			}

		case inAggregate:
			width, err := sdlBitfield(toks, aggPrefix, bitPos, define)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", stmt, err)
			}

			bitPos += width
			if bitPos > 32 {
				return nil, fmt.Errorf("aggregate bitfields exceed 32 bits at %q", stmt)
			}

		default:
			return nil, fmt.Errorf("unsupported SDL statement %q", stmt)
		}
	}

	if inAggregate {
		return nil, fmt.Errorf("unterminated aggregate (prefix %s)", aggPrefix)
	}

	return out, nil
}

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
// defining its symbols at bit position pos and returning its width.
func sdlBitfield(toks []string, prefix string, pos uint32, define func(string, uint32) error) (uint32, error) {
	if len(toks) < 2 || !strings.EqualFold(toks[1], "bitfield") {
		return 0, fmt.Errorf("unsupported aggregate member")
	}

	name := toks[0]
	width := uint32(1)

	var mask, fill bool

	for i := 2; i < len(toks); i++ {
		switch strings.ToLower(toks[i]) {
		case "length":
			if i+1 >= len(toks) {
				return 0, fmt.Errorf("length without a value")
			}

			n, err := strconv.ParseUint(toks[i+1], 10, 6)
			if err != nil || n == 0 || n > 32 {
				return 0, fmt.Errorf("bad bitfield length %q", toks[i+1])
			}

			width = uint32(n)
			i++

		case "mask":
			mask = true

		case "fill":
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
// the required "prefix P$" and "tag T".
func sdlConstant(toks []string, lookup func(string) (uint32, bool)) ([]sdlConst, error) {
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

		case "prefix":
			prefix, havePrefix = arg, true

		case "tag":
			tag, haveTag = arg, true

		default:
			return nil, fmt.Errorf("unsupported constant keyword %q", toks[i-1])
		}
	}

	// SDL has default tags, but $LNMDEF always spells them out; requiring
	// them keeps this parser from having to guess.
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

// sdlValue evaluates an SDL constant value: a decimal literal, a %x
// hexadecimal literal, or NAME@N (the earlier constant NAME shifted left N
// bits).
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

	return strconv.ParseInt(arg, 10, 64)
}
