package dcl

import (
	"fmt"
	"strings"
)

// Result holds everything DCLdump/DCLresults would have handed back after a
// successful Parse: which verb/syntax ended up active, its /entry= (if any),
// and the value of every parameter/qualifier that was actually matched.
type Result struct {
	Verb       string // the top-level verb name, before any /syntax= or keyword redirect
	Active     string // the final active entry name (verb or a redirected-to syntax)
	ActiveID   int64
	EntryPoint string

	values map[string]*matchedValue
}

type matchedValue struct {
	id        int64
	present   bool
	negated   bool
	isString  bool
	isKeyword bool
	str       string
	i         int64
}

func newResult() *Result { return &Result{values: map[string]*matchedValue{}} }

// Present reports whether the named parameter or qualifier was supplied
// (matching DCLpresent).
func (r *Result) Present(name string) bool {
	v, ok := r.values[upcase(name)]
	return ok && v.present
}

// Negated reports whether the named qualifier was supplied in its "NO"
// form.
func (r *Result) Negated(name string) bool {
	v, ok := r.values[upcase(name)]
	return ok && v.negated
}

// String returns the string value of the named parameter or qualifier
// (matching DCLgetstring); "" if absent or the value is a keyword/integer.
func (r *Result) String(name string) string {
	v, ok := r.values[upcase(name)]
	if !ok || !v.isString || v.isKeyword {
		return ""
	}
	return v.str
}

// Int returns the integer value of the named parameter or qualifier
// (matching DCLgetinteger): the literal value for a $integer, or the
// matched Keyword's ID for a keyword-typed value; 0 if absent or the value
// is a plain string.
func (r *Result) Int(name string) int64 {
	v, ok := r.values[upcase(name)]
	if !ok || (v.isString && !v.isKeyword) {
		return 0
	}
	return v.i
}

// Keyword returns the name of the matched keyword for a keyword-typed
// parameter/qualifier (e.g. "SHOW MEMORY" -> Keyword("SHOW_TYPE") ==
// "MEMORY"); "" if absent or not a keyword value.
func (r *Result) Keyword(name string) string {
	v, ok := r.values[upcase(name)]
	if !ok || !v.isKeyword {
		return ""
	}
	return v.str
}

func (r *Result) set(name string, id int64, negated bool, val Value) {
	r.values[upcase(name)] = &matchedValue{
		id: id, present: true, negated: negated,
		isString: val.IsString, isKeyword: val.IsKeyword, str: val.Str, i: val.Int,
	}
}

func (r *Result) markPresent(name string, id int64, negated bool) {
	r.values[upcase(name)] = &matchedValue{id: id, present: true, negated: negated}
}

// Parse parses one command line against the grammar — the Go equivalent of
// DCLparse, minus its interactive re-prompting for a missing required
// parameter/qualifier (see doc.go): a required item still missing once the
// line is exhausted is reported as an error rather than triggering a
// terminal prompt, leaving any such prompting to the console layer that
// calls Parse.
func (g *Grammar) Parse(line string) (*Result, error) {
	line = upcaseOutsideQuotes(line)
	pos := strings.TrimSpace(line)
	if pos == "" {
		return nil, fmt.Errorf("dcl: empty command")
	}

	verbTok, pos := readBareToken(pos)
	verb, err := g.matchVerb(verbTok)
	if err != nil {
		return nil, err
	}

	r := newResult()
	r.Verb = verb.Name

	active := verb
	nextParam := 0

	for {
		pos = strings.TrimLeft(pos, " \t")
		if pos == "" {
			break
		}

		// A '/' always introduces a qualifier, even once a REST_OF_LINE
		// parameter is due — qualifiers may precede it on the line. Only
		// once we're about to read a plain positional token does a due
		// REST_OF_LINE parameter greedily consume everything remaining
		// (qualifier slashes included) and end the parse, matching
		// DCLparse's fsm_allow_rest gate.
		if pos[0] == '/' {
			active, nextParam, pos, err = g.parseQualifier(r, active, nextParam, pos[1:])
			if err != nil {
				return nil, err
			}
			continue
		}

		if nextParam < len(active.Parameters) && active.Parameters[nextParam].Type == TypeRestOfLine {
			p := active.Parameters[nextParam]
			val := strings.TrimSpace(pos)
			r.set(p.Name, p.ID, false, Value{IsString: true, Str: val})
			nextParam++
			pos = ""
			break
		}

		if nextParam >= len(active.Parameters) {
			return nil, fmt.Errorf("dcl: unexpected extra parameter near %q", pos)
		}
		p := active.Parameters[nextParam]
		token, rem, err := readValueToken(pos)
		if err != nil {
			return nil, err
		}
		pos = rem

		val, redirect, negated, err := g.resolveValue(p.Type, p.TypeName, token)
		if err != nil {
			return nil, fmt.Errorf("parameter %s: %w", p.Name, err)
		}
		r.set(p.Name, p.ID, negated, val)
		nextParam++
		if redirect != "" {
			active = g.entries[redirect]
			nextParam = 0
		}
	}

	if err := g.checkRequirements(active, r); err != nil {
		return nil, err
	}

	r.Active = active.Name
	r.ActiveID = active.ID
	r.EntryPoint = active.EntryPoint
	return r, nil
}

// parseQualifier consumes one "/name" or "/name=value" qualifier (rest
// points just past the leading '/'), applying any /syntax= redirect, and
// returns the (possibly redirected) active entry, its next unfilled
// parameter index, and the remaining unparsed text.
func (g *Grammar) parseQualifier(r *Result, active *Entry, nextParam int, rest string) (*Entry, int, string, error) {
	name, rest := readBareToken(rest)
	if name == "" {
		return nil, 0, "", fmt.Errorf("dcl: expected a qualifier name after '/'")
	}

	q, negated, err := matchQualifier(active.Qualifiers, name)
	if err != nil {
		return nil, 0, "", err
	}
	if negated && q.NoNegate {
		return nil, 0, "", fmt.Errorf("dcl: cannot negate qualifier %q", q.Name)
	}
	if q.aliasRef != nil {
		q = q.aliasRef
	}

	var token string
	haveVal := false
	if strings.HasPrefix(rest, "=") {
		token, rest, err = readValueToken(rest[1:])
		if err != nil {
			return nil, 0, "", err
		}
		haveVal = true
	}

	if !q.hasValue() {
		if haveVal {
			return nil, 0, "", fmt.Errorf("dcl: qualifier %s does not take a value", q.Name)
		}
		r.markPresent(q.Name, q.ID, negated)
		if q.Syntax != "" {
			target, ok := g.entries[q.Syntax]
			if !ok {
				return nil, 0, "", fmt.Errorf("dcl: qualifier %s: syntax %q not found", q.Name, q.Syntax)
			}
			return target, 0, rest, nil
		}
		return active, nextParam, rest, nil
	}

	if !haveVal {
		if q.Default != nil {
			r.set(q.Name, q.ID, negated, *q.Default)
			return active, nextParam, rest, nil
		}
		return nil, 0, "", fmt.Errorf("dcl: required value for qualifier %s not found", q.Name)
	}

	val, redirect, kwNegated, err := g.resolveValue(q.Type, q.TypeName, token)
	if err != nil {
		return nil, 0, "", fmt.Errorf("qualifier %s: %w", q.Name, err)
	}
	if negated {
		kwNegated = negated
	}
	r.set(q.Name, q.ID, kwNegated, val)
	if redirect != "" {
		target, ok := g.entries[redirect]
		if !ok {
			return nil, 0, "", fmt.Errorf("dcl: qualifier %s: syntax %q not found", q.Name, redirect)
		}
		return target, 0, rest, nil
	}
	return active, nextParam, rest, nil
}

// resolveValue type-checks token against typ (and, for TypeKeyword,
// typeName's keyword list), returning the resulting Value and, for a
// matched keyword that itself carries /syntax=, the Entry name to redirect
// into.
func (g *Grammar) resolveValue(typ ValueType, typeName string, token string) (val Value, redirect string, negated bool, err error) {
	switch typ {
	case TypeInteger:
		n, err := parseDCLInteger(token)
		if err != nil {
			return Value{}, "", false, err
		}
		return Value{Int: n}, "", false, nil

	case TypeKeyword:
		t, ok := g.types[typeName]
		if !ok {
			return Value{}, "", false, fmt.Errorf("dcl: unknown type %q", typeName)
		}
		kw, neg, err := t.lookup(token)
		if err != nil {
			return Value{}, "", false, err
		}
		return Value{IsString: true, IsKeyword: true, Str: kw.Name, Int: kw.ID}, kw.Syntax, neg, nil

	default: // TypeAny, TypeName, TypeString, TypeRestOfLine, TypeSwitch
		return Value{IsString: true, Str: token}, "", false, nil
	}
}

// checkRequirements matches DCLcheck_requirements: every required (has a
// /prompt=) parameter must have been seen, every present qualifier that
// takes a value but wasn't given one is filled from its default (already
// handled inline in parseQualifier; nothing left to do here beyond the
// value-required check itself, which is also enforced inline), and no
// DISALLOW combination may hold.
func (g *Grammar) checkRequirements(active *Entry, r *Result) error {
	for _, p := range active.Parameters {
		if p.required() && !r.Present(p.Name) {
			return fmt.Errorf("dcl: required parameter %s not found", p.Name)
		}
	}

	for _, q := range active.Qualifiers {
		if q.aliasRef != nil {
			continue
		}
		if !r.Present(q.Name) && q.Default != nil {
			r.set(q.Name, q.ID, false, *q.Default)
		}
	}

	for _, d := range active.Disallows {
		s1 := r.Present(d.Qual1) && r.Negated(d.Qual1) == d.Negated1
		s2 := r.Present(d.Qual2) && r.Negated(d.Qual2) == d.Negated2
		if s1 && s2 {
			return fmt.Errorf("dcl: invalid combination of qualifiers %s and %s", d.Qual1, d.Qual2)
		}
	}
	return nil
}

// parseDCLInteger parses a $integer token, matching DCLatoi: decimal
// digits, an optional trailing K/M/G multiplier suffix, and any '-'
// character (not just a leading one) flips the sign of that multiplier —
// replicated as-is since this is the DCL engine's own tool-input parsing,
// not VAX ISA behavior (see doc.go).
func parseDCLInteger(token string) (int64, error) {
	if token == "" {
		return 0, nil
	}
	mult := int64(1)
	digits := token
	switch token[len(token)-1] {
	case 'K':
		mult = 1024
		digits = token[:len(token)-1]
	case 'M':
		mult = 1024000
		digits = token[:len(token)-1]
	case 'G':
		mult = 1024000000
		digits = token[:len(token)-1]
	}

	var v int64
	for i := 0; i < len(digits); i++ {
		ch := digits[i]
		switch {
		case ch == '-':
			mult = -mult
		case ch >= '0' && ch <= '9':
			v = v*10 + int64(ch-'0')
		default:
			return 0, fmt.Errorf("dcl: invalid integer %q", token)
		}
	}
	return v * mult, nil
}

// upcaseOutsideQuotes upcases every character not inside a double-quoted
// substring, and truncates the line at an unquoted ';' — a direct port of
// DCLupcase.
func upcaseOutsideQuotes(s string) string {
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
		}
		if !inQuote && ch == ';' {
			break
		}
		if inQuote {
			b.WriteByte(ch)
		} else if ch >= 'a' && ch <= 'z' {
			b.WriteByte(ch - 32)
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// readBareToken reads a run of characters up to the next '=', '/',
// whitespace, or end of string — used for verb and qualifier names, which
// are never quoted.
func readBareToken(s string) (token, rest string) {
	i := 0
	for i < len(s) {
		switch s[i] {
		case '=', '/', ' ', '\t':
			return s[:i], s[i:]
		}
		i++
	}
	return s, ""
}

// readValueToken reads one parameter/qualifier value: a double-quoted
// string (returned with quotes stripped, case preserved by
// upcaseOutsideQuotes having skipped it) or a bare token running up to the
// next '/', whitespace, or end of string.
func readValueToken(s string) (token, rest string, err error) {
	s = strings.TrimLeft(s, " \t")
	if strings.HasPrefix(s, `"`) {
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return "", "", fmt.Errorf("dcl: unterminated quoted string")
		}
		return s[1 : end+1], s[end+2:], nil
	}
	i := 0
	for i < len(s) {
		switch s[i] {
		case '/', ' ', '\t':
			return s[:i], s[i:], nil
		}
		i++
	}
	return s, "", nil
}
