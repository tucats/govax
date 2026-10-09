package dcl

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// Result holds everything DCLdump/DCLresults would have handed back after a
// successful Parse: which verb/syntax ended up active, its /entry= (if any),
// and the value of every parameter/qualifier that was actually matched.
type Result struct {
	Verb       string // the top-level verb name, before any /syntax= or keyword redirect
	Active     string // the final active entry name (verb or a redirected-to syntax)
	ActiveID   int64 // the final active entry's ID, for the handler to disambiguate what part of the grammar invoked it
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

	// items holds, for a list parameter with positional qualifiers, each
	// element's own qualifiers, by the parameter's name.
	items map[string][]*Result
}

type matchedValue struct {
	id        int64
	present   bool
	negated   bool
	isString  bool
	isKeyword bool
	str       string
	i         int64
	list      []string // every element of a /list value; nil otherwise
	// defaulted marks a qualifier the command line left out, which is
	// present only because its grammar gives a default.
	defaulted bool
}

func newResult() *Result {
	return &Result{values: map[string]*matchedValue{}, paramValues: map[string]map[string]*matchedValue{}, items: map[string][]*Result{}}
}

// Items returns the positional qualifiers of each element of the list
// parameter name, in the list's order: element i's are read with
// Items(name)[i].Present, String, List, and so on. It's nil for a
// parameter with no positional qualifiers.
func (r *Result) Items(name string) []*Result {
	return r.items[upcase(name)]
}

// Defaulted reports whether the named qualifier is present only because
// its grammar gives it a default, the command line having left it out
// (DCL's CLI$_DEFAULTED, where CLI$PRESENT returns CLI$_PRESENT for one
// given explicitly).
func (r *Result) Defaulted(name string) bool {
	v, ok := r.values[upcase(name)]

	return ok && v.defaulted
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

// List returns the elements of a /list parameter or qualifier, in the
// order given: the string values, or for a keyword type the matched
// keywords' names. For an item that isn't a list, or a list item filled
// from its /default=, it returns its one string value as a one-element
// list; nil if absent or not a string.
func (r *Result) List(name string) []string {
	v, ok := r.values[upcase(name)]
	if !ok || !v.present {
		return nil
	}

	if v.list != nil {
		return append([]string(nil), v.list...)
	}

	if !v.isString {
		return nil
	}

	return []string{v.str}
}

// setList records a /list value: its elements, plus the first element as
// the item's plain String/Keyword value.
func (r *Result) setList(name string, id int64, vals []Value) {
	list := make([]string, len(vals))
	for i, v := range vals {
		list[i] = v.Str
	}

	first := vals[0]
	r.values[upcase(name)] = &matchedValue{
		id: id, present: true,
		isString: first.IsString, isKeyword: first.IsKeyword, str: first.Str, i: first.Int,
		list: list,
	}
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

	// A leading '@' is a verb by itself, as DCL reads "@FILE".
	var verbTok string
	if strings.HasPrefix(pos, "@") {
		verbTok, pos = "@", pos[1:]
	} else {
		verbTok, pos = readBareToken(pos)
	}

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
			next, np, lp, rest, err := g.parseQualifier(r, active, nextParam, lastParam, pos[1:])

			// A qualifier the verb doesn't know may belong to a syntax
			// one of the verb's qualifiers switches to later on the line
			// (ANALYZE/GSD/OBJECT): switch now, and try it again.
			if err != nil && active == verb && errors.Is(err, vmserrors.New(vmserrors.CLI_UNRECOGNIZED)) {
				if target, without, ok := g.syntaxLater(r, verb, pos); ok {
					active, nextParam, lastParam, pos = target, 0, nil, without

					continue
				}
			}

			if err != nil {
				return nil, err
			}

			active, nextParam, lastParam, pos = next, np, lp, rest

			continue
		}

		if nextParam < len(active.Parameters) && active.Parameters[nextParam].Type == TypeRestOfLine {
			p := active.Parameters[nextParam]
			val := strings.TrimSpace(pos)

			r.set(p.Name, p.ID, false, Value{IsString: true, Str: val})
			lastParam = p //nolint:ineffassign
			nextParam++   //nolint:ineffassign
			pos = ""

			break
		}

		// An entry with /assignment= continues in that syntax when its
		// first positional token is "name=": SET NAME=value.
		if nextParam == 0 && lastParam == nil && active.Assignment != "" && isAssignment(pos) && !g.valueKeyword(active, pos) {
			active = g.entries[active.Assignment]

			continue
		}

		if nextParam >= len(active.Parameters) {
			return nil, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, pos)
		}

		p := active.Parameters[nextParam]

		if p.List && len(p.positional()) > 0 {
			var (
				vals  []Value
				items []*Result
			)

			vals, items, pos, err = g.readItems(r, active, nextParam, p, pos)
			if err != nil {
				return nil, err
			}

			r.setList(p.Name, p.ID, vals)
			r.items[p.Name] = items
			lastParam = p
			nextParam++

			continue
		}

		if p.List {
			tokens, rem, err := readList(pos, false, elementReader(p.Type))
			if err != nil {
				return nil, vmserrors.Wrap(vmserrors.CLI_BADPARAMETER, err, p.Name)
			}

			vals, err := g.resolveList(p.Type, p.TypeName, tokens)
			if err != nil {
				return nil, vmserrors.Wrap(vmserrors.CLI_BADPARAMETER, err, p.Name)
			}

			pos = skipSeparator(rem, p.Separator)

			r.setList(p.Name, p.ID, vals)
			lastParam = p
			nextParam++

			continue
		}

		// A keyword that takes a value (KEYWORD=value) ends at its "=".
		sep := p.Separator
		if t := g.types[p.TypeName]; p.Type == TypeKeyword && sep == 0 && t != nil && t.hasValueKeywords() {
			sep = '='
		}

		token, rem, err := readTypedValue(pos, p.Type, sep)
		if err != nil {
			return nil, err
		}

		pos = skipSeparator(rem, p.Separator)

		val, redirect, negated, err := g.resolveValue(p.Type, p.TypeName, token)
		if err != nil {
			return nil, vmserrors.Wrap(vmserrors.CLI_BADPARAMETER, err, p.Name)
		}

		// Its value follows the "=", for the redirected syntax.
		if p.Type == TypeKeyword && g.takesValue(p.TypeName, val.Str) {
			pos = skipSeparator(pos, '=')
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

	// DCL writes a qualifier's value after "=" or ":" (/AFTER=3, /AFTER:3);
	// the VMS debugger's commands favor the colon.
	if strings.HasPrefix(rest, ":") {
		rest = "=" + rest[1:]
	}

	var token string

	haveVal := false

	if strings.HasPrefix(rest, "=") && q.List && q.hasValue() {
		tokens, rem, err := readList(rest[1:], true, elementReader(q.Type))
		if err != nil {
			return nil, 0, nil, "", vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, q.Name)
		}

		vals, err := g.resolveList(q.Type, q.TypeName, tokens)
		if err != nil {
			return nil, 0, nil, "", vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, q.Name)
		}

		if scopeParam == nil {
			r.setList(q.Name, q.ID, vals)
			r.values[q.Name].negated = negated
		} else {
			r.setParam(scopeParam.Name, q.Name, q.ID, negated, vals[0])
		}

		return active, nextParam, lastParam, rem, nil
	}

	if strings.HasPrefix(rest, "=") {
		token, rest, err = readTypedValue(rest[1:], q.Type, 0)
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

		if neg && kw.NoNegate {
			return Value{}, "", false, vmserrors.New(vmserrors.CLI_NONEGATE, kw.Name)
		}

		return Value{IsString: true, IsKeyword: true, Str: kw.Name, Int: kw.ID}, kw.Syntax, neg, nil

	default: // TypeAny, TypeName, TypeString, TypeRestOfLine, TypeExpression, TypeSwitch
		return Value{IsString: true, Str: token}, "", false, nil
	}
}

// resolveList type-checks each element of a list value, as resolveValue
// does for a single one. A keyword element can't be negated, and can't
// redirect parsing (a /syntax= keyword in a list would be meaningless).
func (g *Grammar) resolveList(typ ValueType, typeName string, tokens []string) ([]Value, error) {
	vals := make([]Value, 0, len(tokens))

	for _, tok := range tokens {
		val, _, negated, err := g.resolveValue(typ, typeName, tok)
		if err != nil {
			return nil, err
		}

		if negated {
			return nil, vmserrors.New(vmserrors.CLI_NONEGATE, tok)
		}

		vals = append(vals, val)
	}

	return vals, nil
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
			r.values[q.Name].defaulted = true
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
// substring. A prior version of this function also truncated the
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

// syntaxLater looks along line for a qualifier of verb that switches to
// another syntax (a value-less /syntax= qualifier, such as ANALYZE's
// /OBJECT). Real DCL applies such a qualifier to the whole command,
// wherever it is on the line; the parser, which reads left to right,
// switches only when it reaches it. When a qualifier before it is one only
// the new syntax knows, Parse calls this to switch early: it marks the
// switching qualifier present and returns its syntax and the line without
// it. Quoted text is skipped.
func (g *Grammar) syntaxLater(r *Result, verb *Entry, line string) (*Entry, string, bool) {
	inQuote := false

	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '"':
			inQuote = !inQuote

			continue
		case inQuote || line[i] != '/':
			continue
		}

		name, rest := readBareToken(line[i+1:])
		if name == "" || strings.HasPrefix(rest, "=") {
			continue
		}

		q, negated, err := matchQualifier(verb.Qualifiers, name)
		if err != nil || negated || q.Syntax == "" || q.hasValue() {
			continue
		}

		target, ok := g.entries[q.Syntax]
		if !ok {
			return nil, "", false
		}

		r.markPresent(q.Name, q.ID, false)

		return target, line[:i] + rest, true
	}

	return nil, "", false
}

// valueKeyword reports whether s, the rest of a command line, starts
// with "KEYWORD=" for a /value keyword of active's first parameter's
// type, spelled out in full: SET PROMPT="text" is the PROMPT keyword, not
// an assignment to a symbol named PROMPT (while SET P=1 still is one).
func (g *Grammar) valueKeyword(active *Entry, s string) bool {
	if len(active.Parameters) == 0 || active.Parameters[0].Type != TypeKeyword {
		return false
	}

	name, _ := readBareToken(strings.TrimLeft(s, " \t"))

	t := g.types[active.Parameters[0].TypeName]
	if t == nil {
		return false
	}

	for _, k := range t.Keywords {
		if k.Value && k.Name == name {
			return true
		}
	}

	return false
}

// takesValue reports whether keyword name of type typeName is a /value
// keyword.
func (g *Grammar) takesValue(typeName, name string) bool {
	t := g.types[typeName]
	if t == nil {
		return false
	}

	for _, k := range t.Keywords {
		if k.Name == name {
			return k.Value
		}
	}

	return false
}

// readBareToken reads a run of characters up to the next '=', ':', '/',
// whitespace, or end of string — used for verb and qualifier names, which
// are never quoted.
func readBareToken(s string) (token, rest string) {
	i := 0
	for i < len(s) {
		switch s[i] {
		case '=', ':', '/', ',', ' ', '\t':
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

// readList reads a comma-separated list value from s: elements separated
// by commas, with optional whitespace on either side of each comma. Each
// element is a double-quoted string (quotes stripped, so it may contain
// commas, slashes, or spaces) or a bare token running up to the next
// ',', '/', whitespace, or end of string.
//
// When paren is set (a qualifier value), s may instead start with "(",
// in which case the list runs to the matching ")", ')' also ends a bare
// element, and whitespace may follow "(" or precede ")". Without "(" a
// qualifier's value is a single element, as in DCL, where "/X=A,B" means
// /X=A followed by a second parameter-list element.
func readList(s string, paren bool, element elementFunc) (tokens []string, rest string, err error) {
	orig := s
	s = strings.TrimLeft(s, " \t")

	if paren {
		if !strings.HasPrefix(s, "(") {
			tok, rem, err := element(s, false)
			if err != nil {
				return nil, "", err
			}

			if tok == "" {
				return nil, "", vmserrors.New(vmserrors.CLI_EMPTYELEMENT, orig)
			}

			return []string{tok}, rem, nil
		}

		s = s[1:]
	}

	for {
		s = strings.TrimLeft(s, " \t")

		tok, rem, err := element(s, paren)
		if err != nil {
			return nil, "", err
		}

		if tok == "" && !strings.HasPrefix(s, `""`) {
			return nil, "", vmserrors.New(vmserrors.CLI_EMPTYELEMENT, orig)
		}

		tokens = append(tokens, tok)

		after := strings.TrimLeft(rem, " \t")
		if strings.HasPrefix(after, ",") {
			s = after[1:]

			continue
		}

		if paren {
			if !strings.HasPrefix(after, ")") {
				return nil, "", vmserrors.New(vmserrors.CLI_NEEDPAREN)
			}

			return tokens, after[1:], nil
		}

		return tokens, rem, nil
	}
}

// readListElement reads one element of a list for readList.
func readListElement(s string, paren bool) (token, rest string, err error) {
	if strings.HasPrefix(s, `"`) {
		return readValueToken(s)
	}

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ',', '/', ' ', '\t':
			return s[:i], s[i:], nil

		case ')':
			if paren {
				return s[:i], s[i:], nil
			}
		}
	}

	return s, "", nil
}

// readItems reads a list parameter whose elements can each carry
// positional qualifiers ("MAIN,MYLIB/LIBRARY,OPTS/OPTIONS"). Each
// element's positional qualifiers go into its own Result. Any other
// qualifier between elements is parsed as parseQualifier would parse it
// after the parameter, so "A/MAP,B" works as DCL allows; one that switches
// to another syntax there is an error.
func (g *Grammar) readItems(r *Result, active *Entry, nextParam int, p *Parameter, s string) ([]Value, []*Result, string, error) {
	var (
		tokens []string
		items  []*Result
	)

	orig := s
	positional := p.positional()

	for {
		s = strings.TrimLeft(s, " \t")

		tok, rem, err := readListElement(s, false)
		if err != nil {
			return nil, nil, "", err
		}

		if tok == "" && !strings.HasPrefix(s, `""`) {
			return nil, nil, "", vmserrors.New(vmserrors.CLI_EMPTYELEMENT, orig)
		}

		item := newResult()

		for {
			after := strings.TrimLeft(rem, " \t")
			if !strings.HasPrefix(after, "/") {
				break
			}

			name, _ := readBareToken(after[1:])

			if q, negated, err := matchQualifier(positional, name); err == nil {
				if rem, err = g.itemQualifier(item, q, negated, after[1+len(name):]); err != nil {
					return nil, nil, "", err
				}

				continue
			}

			next, _, _, rest, err := g.parseQualifier(r, active, nextParam, p, after[1:])
			if err != nil {
				return nil, nil, "", err
			}

			if next != active {
				return nil, nil, "", vmserrors.New(vmserrors.CLI_BADQUALIFIER, name)
			}

			rem = rest
		}

		tokens = append(tokens, tok)
		items = append(items, item)

		after := strings.TrimLeft(rem, " \t")
		if !strings.HasPrefix(after, ",") {
			vals, err := g.resolveList(p.Type, p.TypeName, tokens)
			if err != nil {
				return nil, nil, "", vmserrors.Wrap(vmserrors.CLI_BADPARAMETER, err, p.Name)
			}

			return vals, items, rem, nil
		}

		s = after[1:]
	}
}

// itemQualifier records a positional qualifier (rest is what follows its
// name) in its element's Result, and returns what follows it.
func (g *Grammar) itemQualifier(item *Result, q *Qualifier, negated bool, rest string) (string, error) {
	if negated && q.NoNegate {
		return "", vmserrors.New(vmserrors.CLI_NONEGATE, q.Name)
	}

	if !strings.HasPrefix(rest, "=") {
		switch {
		case !q.hasValue():
			item.markPresent(q.Name, q.ID, negated)
		case q.Default != nil:
			item.set(q.Name, q.ID, negated, *q.Default)
		default:
			return "", vmserrors.New(vmserrors.CLI_NEEDQUALIFIERVALUE, q.Name)
		}

		return rest, nil
	}

	if !q.hasValue() {
		return "", vmserrors.New(vmserrors.CLI_NOQUALIFIERVALUE, q.Name)
	}

	if q.List {
		tokens, rem, err := readList(rest[1:], true, elementReader(q.Type))
		if err != nil {
			return "", vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, q.Name)
		}

		vals, err := g.resolveList(q.Type, q.TypeName, tokens)
		if err != nil {
			return "", vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, q.Name)
		}

		item.setList(q.Name, q.ID, vals)
		item.values[q.Name].negated = negated

		return rem, nil
	}

	token, rem, err := readTypedValue(rest[1:], q.Type, 0)
	if err != nil {
		return "", err
	}

	val, _, _, err := g.resolveValue(q.Type, q.TypeName, token)
	if err != nil {
		return "", vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, q.Name)
	}

	item.set(q.Name, q.ID, negated, val)

	return rem, nil
}
