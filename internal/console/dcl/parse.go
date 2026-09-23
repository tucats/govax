package dcl

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
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

	// paramValues holds the results of parameter-scoped qualifiers (Phase
	// 23) — a qualifier that was matched against one specific Parameter's
	// own Qualifiers list rather than the enclosing entry's. It's a nested
	// map (outer key: the parameter's name; inner key: the qualifier's
	// name) kept entirely separate from values above, on purpose: COPY
	// declares a qualifier named HOST under both its SOURCE and DESTINATION
	// parameters, and those two /HOST occurrences must be told apart
	// (ParamPresent("SOURCE", "HOST") vs. ParamPresent("DESTINATION",
	// "HOST")) rather than one overwriting the other in a single flat map,
	// and also kept apart from any qualifier that happens to share the same
	// name at the plain entry level.
	paramValues map[string]map[string]*matchedValue
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

func newResult() *Result {
	return &Result{values: map[string]*matchedValue{}, paramValues: map[string]map[string]*matchedValue{}}
}

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

// paramValue looks up the recorded match (if any) for a parameter-scoped
// qualifier, keyed first by the owning parameter's name and then by the
// qualifier's own name. Both names are upcased before lookup, matching every
// other name lookup in this package (grammar names are always compared
// upcased — see upcase in match.go).
func (r *Result) paramValue(paramName, qualName string) (*matchedValue, bool) {
	inner, ok := r.paramValues[upcase(paramName)]
	if !ok {
		return nil, false
	}

	v, ok := inner[upcase(qualName)]

	return v, ok
}

// setParam and markParamPresent are the parameter-scoped counterparts of set
// and markPresent above: they record a matched qualifier's value/presence
// under (paramName, qualName) in paramValues instead of under a single flat
// name in values, so that two identically-named qualifiers scoped to
// different parameters (COPY's /HOST under SOURCE and again under
// DESTINATION being the motivating case) don't collide.
func (r *Result) setParam(paramName, qualName string, id int64, negated bool, val Value) {
	paramName = upcase(paramName)

	inner, ok := r.paramValues[paramName]
	if !ok {
		inner = map[string]*matchedValue{}
		r.paramValues[paramName] = inner
	}

	inner[upcase(qualName)] = &matchedValue{
		id: id, present: true, negated: negated,
		isString: val.IsString, isKeyword: val.IsKeyword, str: val.Str, i: val.Int,
	}
}

func (r *Result) markParamPresent(paramName, qualName string, id int64, negated bool) {
	paramName = upcase(paramName)

	inner, ok := r.paramValues[paramName]
	if !ok {
		inner = map[string]*matchedValue{}
		r.paramValues[paramName] = inner
	}

	inner[upcase(qualName)] = &matchedValue{id: id, present: true, negated: negated}
}

// ParamPresent reports whether qualName was supplied on the command line
// scoped to paramName — the parameter-scoped mirror of Present above. For
// example, on "COPY FOO.TXT BAR.TXT/HOST", ParamPresent("DESTINATION",
// "HOST") is true and ParamPresent("SOURCE", "HOST") is false, even though
// both parameters' grammar declares a qualifier named HOST.
func (r *Result) ParamPresent(paramName, qualName string) bool {
	v, ok := r.paramValue(paramName, qualName)

	return ok && v.present
}

// ParamNegated reports whether qualName, scoped to paramName, was supplied
// in its "NO" form — the parameter-scoped mirror of Negated above.
func (r *Result) ParamNegated(paramName, qualName string) bool {
	v, ok := r.paramValue(paramName, qualName)

	return ok && v.negated
}

// ParamString returns the string value of qualName scoped to paramName —
// the parameter-scoped mirror of String above; "" if absent or the value is
// a keyword/integer.
func (r *Result) ParamString(paramName, qualName string) string {
	v, ok := r.paramValue(paramName, qualName)
	if !ok || !v.isString || v.isKeyword {
		return ""
	}

	return v.str
}

// ParamInt returns the integer value of qualName scoped to paramName — the
// parameter-scoped mirror of Int above; 0 if absent or the value is a plain
// string.
func (r *Result) ParamInt(paramName, qualName string) int64 {
	v, ok := r.paramValue(paramName, qualName)
	if !ok || (v.isString && !v.isKeyword) {
		return 0
	}

	return v.i
}

// ParamKeyword returns the name of the matched keyword for a keyword-typed
// qualifier scoped to paramName — the parameter-scoped mirror of Keyword
// above; "" if absent or not a keyword value.
func (r *Result) ParamKeyword(paramName, qualName string) string {
	v, ok := r.paramValue(paramName, qualName)
	if !ok || !v.isKeyword {
		return ""
	}

	return v.str
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
		return nil, vmserrors.New(vmserrors.CLI_EMPTYCOMMAND)
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

	// lastParam remembers whichever Parameter was most recently filled in
	// on this command line (nil until the first one is), so that a "/name"
	// qualifier token encountered right after it can be checked against
	// that parameter's own private Qualifiers list first — see
	// parseQualifier and the Parameter.Qualifiers doc comment in
	// grammar.go. It's reset to nil whenever active itself changes (a
	// qualifier's or a keyword parameter's /syntax= redirect switches to a
	// different verb/syntax entry entirely), since a Parameter belonging to
	// the entry we just left has no business being consulted against the
	// new entry's command line.
	var lastParam *Parameter

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
			active, nextParam, lastParam, pos, err = g.parseQualifier(r, active, nextParam, lastParam, pos[1:])
			if err != nil {
				return nil, err
			}

			continue
		}

		if nextParam < len(active.Parameters) && active.Parameters[nextParam].Type == TypeRestOfLine {
			p := active.Parameters[nextParam]
			val := strings.TrimSpace(pos)

			r.set(p.Name, p.ID, false, Value{IsString: true, Str: val})
			lastParam = p

			nextParam++ //nolint:ineffassign
			pos = ""

			break
		}

		if nextParam >= len(active.Parameters) {
			return nil, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, pos)
		}

		p := active.Parameters[nextParam]

		token, rem, err := readValueToken(pos)
		if err != nil {
			return nil, err
		}

		pos = rem

		val, redirect, negated, err := g.resolveValue(p.Type, p.TypeName, token)
		if err != nil {
			return nil, vmserrors.Wrap(vmserrors.CLI_BADPARAMETER, err, p.Name)
		}

		r.set(p.Name, p.ID, negated, val)
		lastParam = p

		nextParam++

		if redirect != "" {
			active = g.entries[redirect]
			nextParam = 0
			lastParam = nil
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
// parameter index, the last-filled parameter to carry forward (unchanged
// unless a redirect happened, in which case it becomes nil — see Parse's own
// doc comment on lastParam), and the remaining unparsed text.
//
// lastParam is whichever positional parameter Parse most recently filled in,
// or nil if none has been filled yet on this command line. When it's non-nil,
// this function tries matching the "/name" token against that parameter's
// own private Qualifiers list first (Phase 23's parameter-scoped
// qualifiers); only when that lookup doesn't find anything does it fall back
// to matching against the active entry's own Qualifiers list, exactly as
// this function always worked before that addition. This ordering is what
// keeps the change purely additive: a grammar that declares no
// parameter-scoped qualifiers at all (every verb/syntax before Phase 23)
// always misses the first lookup and falls straight through to the second,
// unchanged.
func (g *Grammar) parseQualifier(r *Result, active *Entry, nextParam int, lastParam *Parameter, rest string) (*Entry, int, *Parameter, string, error) {
	name, rest := readBareToken(rest)
	if name == "" {
		return nil, 0, nil, "", vmserrors.New(vmserrors.CLI_NEEDQUALIFIERNAME)
	}

	// scopeParam records which Parameter (if any) the matched qualifier
	// came from, so its matched value can be filed into Result under the
	// right (paramName, qualName) pair instead of the flat entry-level map.
	var (
		q          *Qualifier
		negated    bool
		err        error
		scopeParam *Parameter
	)

	if lastParam != nil {
		if q, negated, err = lastParam.qualifier(name); err == nil {
			scopeParam = lastParam
		}
	}

	if q == nil {
		if q, negated, err = matchQualifier(active.Qualifiers, name); err != nil {
			return nil, 0, nil, "", err
		}
	}

	if negated && q.NoNegate {
		return nil, 0, nil, "", vmserrors.New(vmserrors.CLI_NONEGATE, q.Name)
	}

	if q.aliasRef != nil {
		q = q.aliasRef
	}

	var token string

	haveVal := false

	if strings.HasPrefix(rest, "=") {
		token, rest, err = readValueToken(rest[1:])
		if err != nil {
			return nil, 0, nil, "", err
		}

		haveVal = true
	}

	if !q.hasValue() {
		if haveVal {
			return nil, 0, nil, "", vmserrors.New(vmserrors.CLI_NOQUALIFIERVALUE, q.Name)
		}

		if scopeParam != nil {
			r.markParamPresent(scopeParam.Name, q.Name, q.ID, negated)
		} else {
			r.markPresent(q.Name, q.ID, negated)
		}

		if q.Syntax != "" {
			target, ok := g.entries[q.Syntax]
			if !ok {
				return nil, 0, nil, "", vmserrors.New(vmserrors.CLI_SYNTAXNOTFOUND, q.Name, q.Syntax)
			}

			return target, 0, nil, rest, nil
		}

		return active, nextParam, lastParam, rest, nil
	}

	if !haveVal {
		if q.Default != nil {
			if scopeParam != nil {
				r.setParam(scopeParam.Name, q.Name, q.ID, negated, *q.Default)
			} else {
				r.set(q.Name, q.ID, negated, *q.Default)
			}

			return active, nextParam, lastParam, rest, nil
		}

		return nil, 0, nil, "", vmserrors.New(vmserrors.CLI_NEEDQUALIFIERVALUE, q.Name)
	}

	val, redirect, kwNegated, err := g.resolveValue(q.Type, q.TypeName, token)
	if err != nil {
		return nil, 0, nil, "", vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, q.Name)
	}

	if negated {
		kwNegated = negated
	}

	if scopeParam != nil {
		r.setParam(scopeParam.Name, q.Name, q.ID, kwNegated, val)
	} else {
		r.set(q.Name, q.ID, kwNegated, val)
	}

	if redirect != "" {
		target, ok := g.entries[redirect]
		if !ok {
			return nil, 0, nil, "", vmserrors.New(vmserrors.CLI_SYNTAXNOTFOUND, q.Name, redirect)
		}

		return target, 0, nil, rest, nil
	}

	return active, nextParam, lastParam, rest, nil
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
			return Value{}, "", false, vmserrors.New(vmserrors.CLI_UNKNOWNTYPE, typeName)
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
			return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, p.Name)
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

	// Each parameter's own private qualifiers (Phase 23) get the same
	// missing-but-defaulted treatment as the entry-level ones just above,
	// just filed into the parameter-scoped side of Result instead.
	for _, p := range active.Parameters {
		for _, q := range p.Qualifiers {
			if q.aliasRef != nil {
				continue
			}

			if !r.ParamPresent(p.Name, q.Name) && q.Default != nil {
				r.setParam(p.Name, q.Name, q.ID, false, *q.Default)
			}
		}
	}

	for _, d := range active.Disallows {
		s1 := r.Present(d.Qual1) && r.Negated(d.Qual1) == d.Negated1
		s2 := r.Present(d.Qual2) && r.Negated(d.Qual2) == d.Negated2

		if s1 && s2 {
			return vmserrors.New(vmserrors.CLI_BADQUALIFIERCOMBO, d.Qual1, d.Qual2)
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
			return 0, vmserrors.New(vmserrors.CLI_BADINTEGER, token)
		}
	}

	return v * mult, nil
}

// upcaseOutsideQuotes upcases every character not inside a double-quoted
// substring — a direct port of DCLupcase (reference/eVAX/eVAX/Source/
// Console/dclrtl.c). A prior version of this function also truncated the
// line at an unquoted ';', but the real DCLupcase does no such thing (it
// only tracks quote state and upcases outside it) -- that truncation was a
// plain porting bug, found while implementing docs/PHASE-23.md's DELETE
// (subtask 6): unquoted ';' is exactly how an ODS-2 file version is
// written (e.g. "FOO.TXT;1"), so it silently ate every file spec's version
// field before Parse ever saw it. Fixed to match the real C source exactly,
// per CLAUDE.md's bug-fixing policy (a clear, obvious logic error
// contradicting this function's own "direct port" doc comment, not an ISA/
// hardware fidelity question).
func upcaseOutsideQuotes(s string) string {
	var b strings.Builder

	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
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
			return "", "", vmserrors.New(vmserrors.CLI_UNTERMSTR)
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
