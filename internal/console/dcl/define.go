package dcl

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// ParseGrammar parses grammar-definition text (the dialect
// testdata/dcl/evax.dcl uses: "grammar"/"verb"/"syntax"/"type"/"keyword"/
// "qualifier"/"parameter"/"disallow"/"end" statements, "!" line comments,
// and "-" line-continuation) into a validated Grammar — the Go equivalent
// of DCLread/DCLdefine/DCLvalidate, minus the FSM/self-hosting machinery
// (see doc.go).
func ParseGrammar(text string) (*Grammar, error) {
	var (
		g       *Grammar
		cur     *Entry
		curType *Type
	)

	lines := joinContinuations(text)
	for lineNo, stmt := range lines {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "!") {
			continue
		}

		directive, name, switches, err := tokenizeStatement(stmt)
		if err != nil {
			return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
		}

		switch directive {
		case "GRAMMAR":
			g = newGrammar(name)
			cur, curType = nil, nil

		case "END":
			// No further statements expected; nothing to reset.

		case "TYPE":
			if g == nil {
				return nil, vmserrors.New(vmserrors.CLI_OUTSIDE, lineNo+1, "TYPE", "a grammar")
			}

			curType = &Type{Name: name}
			g.types[upcase(name)] = curType
			cur = nil

		case "KEYWORD":
			if curType == nil {
				return nil, vmserrors.New(vmserrors.CLI_OUTSIDE, lineNo+1, "KEYWORD", "a TYPE")
			}

			kw := &Keyword{Name: upcase(name)}

			for k, v := range switches {
				switch k {
				case "ID":
					kw.ID, err = parseID(v)

				case "SYNTAX":
					kw.Syntax = upcase(v)

				default:
					err = vmserrors.New(vmserrors.CLI_BADSWITCH, "keyword", k)
				}

				if err != nil {
					return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
				}
			}

			curType.Keywords = append(curType.Keywords, kw)

		case "VERB", "SYNTAX":
			if g == nil {
				return nil, vmserrors.New(vmserrors.CLI_OUTSIDE, lineNo+1, directive, "a grammar")
			}

			e := &Entry{Name: upcase(name), IsVerb: directive == "VERB"}

			for k, v := range switches {
				switch k {
				case "ID":
					e.ID, err = parseID(v)

				case "ENTRY":
					e.EntryPoint = upcase(v)

				case "ALIAS":
					e.Alias = upcase(v)

				default:
					err = vmserrors.New(vmserrors.CLI_BADSWITCH, directive, k)
				}

				if err != nil {
					return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
				}
			}

			if _, exists := g.entries[e.Name]; exists {
				return nil, vmserrors.New(vmserrors.CLI_REDEFINED, lineNo+1, directive, e.Name)
			}

			g.entries[e.Name] = e
			if e.IsVerb {
				g.verbOrder = append(g.verbOrder, e)
			}

			cur, curType = e, nil

		case "PARAMETER":
			if cur == nil {
				return nil, vmserrors.New(vmserrors.CLI_OUTSIDE, lineNo+1, "PARAMETER", "a VERB/SYNTAX")
			}

			p := &Parameter{Name: upcase(name)}
			if err := applyValueSwitches(switches, &p.Type, &p.TypeName, &p.Default); err != nil {
				return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
			}

			if id, ok := switches["ID"]; ok {
				if p.ID, err = parseID(id); err != nil {
					return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
				}
			}

			if prompt, ok := switches["PROMPT"]; ok {
				p.Prompt = prompt
			}

			cur.Parameters = append(cur.Parameters, p)

		case "QUALIFIER":
			if cur == nil {
				return nil, vmserrors.New(vmserrors.CLI_OUTSIDE, lineNo+1, "QUALIFIER", "a VERB/SYNTAX")
			}

			q := &Qualifier{Name: upcase(name)}
			if err := applyValueSwitches(switches, &q.Type, &q.TypeName, &q.Default); err != nil {
				return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
			}

			if id, ok := switches["ID"]; ok {
				if q.ID, err = parseID(id); err != nil {
					return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
				}
			}

			if syn, ok := switches["SYNTAX"]; ok {
				q.Syntax = upcase(syn)
			}

			if alias, ok := switches["ALIAS"]; ok {
				q.Alias = upcase(alias)
			}

			if _, ok := switches["NONEGATABLE"]; ok {
				q.NoNegate = true
			}

			cur.Qualifiers = append(cur.Qualifiers, q)

		case "DISALLOW":
			if cur == nil {
				return nil, vmserrors.New(vmserrors.CLI_OUTSIDE, lineNo+1, "DISALLOW", "a VERB/SYNTAX")
			}

			d, err := parseDisallow(name)
			if err != nil {
				return nil, vmserrors.Wrap(vmserrors.CLI_LINEERR, err, lineNo+1)
			}

			cur.Disallows = append(cur.Disallows, d)

		default:
			return nil, vmserrors.New(vmserrors.CLI_BADDIRECTIVE, lineNo+1, directive)
		}
	}

	if g == nil {
		return nil, vmserrors.New(vmserrors.CLI_NOGRAMMAR)
	}

	if err := g.validate(); err != nil {
		return nil, err
	}

	return g, nil
}

// LoadGrammarFile reads and parses a grammar-definition file, matching
// DCLread.
func LoadGrammarFile(path string) (*Grammar, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseGrammar(string(b))
}

// joinContinuations splits text into lines and joins a line ending in "-"
// with the following line (with a single space between them, and the
// trailing "-" removed) — DCLread's own continuation handling, run once
// over the whole file up front rather than the C source's read-one-record-
// at-a-time loop.
func joinContinuations(text string) []string {
	var (
		out     []string
		pending strings.Builder
	)

	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)

	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")

		trimmed := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmed, "-") {
			pending.WriteString(" ")
			pending.WriteString(strings.TrimSuffix(trimmed, "-"))

			continue
		}

		pending.WriteString(line)
		out = append(out, pending.String())
		pending.Reset()
	}

	if pending.Len() > 0 {
		out = append(out, pending.String())
	}

	return out
}

// tokenizeStatement splits one grammar statement into its leading directive
// keyword, the name (if any) following it on the same word, and its
// "/switch" or "/switch=value" suffixes — quote-aware, since a switch value
// (e.g. /prompt="Name") may itself contain '/' or whitespace.
func tokenizeStatement(stmt string) (directive, name string, switches map[string]string, err error) {
	segments, err := splitUnquoted(stmt, '/')
	if err != nil {
		return "", "", nil, err
	}

	if len(segments) == 0 {
		return "", "", nil, vmserrors.New(vmserrors.CLI_EMPTYSTATEMENT)
	}

	head := strings.Join(strings.Fields(segments[0]), " ")
	fields := strings.SplitN(head, " ", 2)

	directive = upcase(fields[0])
	if directive == "DISALLOW" {
		if len(fields) < 2 {
			return "", "", nil, vmserrors.New(vmserrors.CLI_DISALLOWEXPR)
		}

		return directive, strings.TrimSpace(fields[1]), nil, nil
	}

	if len(fields) == 2 {
		name = upcase(strings.TrimSpace(fields[1]))
	}

	switches = map[string]string{}

	for _, seg := range segments[1:] {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		eq := strings.IndexByte(seg, '=')
		if eq < 0 {
			switches[upcase(seg)] = ""

			continue
		}

		key := upcase(strings.TrimSpace(seg[:eq]))
		val := strings.TrimSpace(seg[eq+1:])
		val = strings.Trim(val, `"`)
		switches[key] = val
	}

	return directive, name, switches, nil
}

// splitUnquoted splits s on sep, ignoring any sep byte that falls inside a
// double-quoted substring.
func splitUnquoted(s string, sep byte) ([]string, error) {
	var (
		out []string
		cur strings.Builder
	)

	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
		}

		if ch == sep && !inQuote {
			out = append(out, cur.String())
			cur.Reset()

			continue
		}

		cur.WriteByte(ch)
	}

	if inQuote {
		return nil, vmserrors.New(vmserrors.CLI_UNTERMQUOTE, s)
	}

	out = append(out, cur.String())

	return out, nil
}

func parseID(v string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
}

// applyValueSwitches interprets the /type=/default= switches shared by
// PARAMETER and QUALIFIER statements, matching DCLdefine_element's handling
// of DCL_QUALIFIER_TYPE/DCL_QUALIFIER_DEFAULT for both kinds of items.
func applyValueSwitches(switches map[string]string, typ *ValueType, typeName *string, def **Value) error {
	if t, ok := switches["TYPE"]; ok {
		switch upcase(t) {
		case "$ANY":
			*typ = TypeAny

		case "$NAME":
			*typ = TypeName

		case "$STRING":
			*typ = TypeString

		case "$INTEGER":
			*typ = TypeInteger

		case "$REST_OF_LINE":
			*typ = TypeRestOfLine

		default:
			*typ = TypeKeyword
			*typeName = upcase(t)
		}
	}

	if d, ok := switches["DEFAULT"]; ok {
		if n, err := strconv.ParseInt(d, 10, 64); err == nil {
			*def = &Value{Int: n}
		} else {
			*def = &Value{IsString: true, Str: d}
		}
	}

	return nil
}

func parseDisallow(expr string) (*Disallow, error) {
	fields := strings.Fields(expr)
	if len(fields) != 3 || upcase(fields[1]) != "AND" {
		return nil, vmserrors.New(vmserrors.CLI_BADDISALLOW, expr)
	}

	q1, neg1 := splitNegated(upcase(fields[0]))
	q2, neg2 := splitNegated(upcase(fields[2]))

	return &Disallow{Qual1: q1, Negated1: neg1, Qual2: q2, Negated2: neg2}, nil
}

func splitNegated(name string) (string, bool) {
	if strings.HasPrefix(name, "NO") && len(name) > 2 {
		return name[2:], true
	}

	return name, false
}
