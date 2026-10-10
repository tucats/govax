package console

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// Symbol substitution (docs/PHASE-50 - DCL command procedures.md,
// subtask 8), as the OpenVMS User's Manual (7.3) describes it in sections
// 12.12 and 12.13, with the details VMS 7.3 showed in testdata/dcl50's
// probe. DCL processes a command line in three phases, and the console
// follows them:
//
//  1. Scanning (substituteApostrophes): each symbol after an apostrophe
//     is replaced by its value, left to right, before anything else reads
//     the line. Outside quotes, 'NAME' is replaced, and the value is
//     scanned again, so a value that holds 'OTHER' is replaced in turn
//     (iterative substitution). Blanks may follow the apostrophe, and the
//     closing apostrophe may be left out: 'NAME at the end of a word is
//     replaced too. Inside quotes, it takes two apostrophes before the
//     name and one after, ''NAME', and the value isn't scanned again. A
//     lexical function call may stand in for the name: 'F$LENGTH(X)'.
//     An undefined symbol is replaced by nothing.
//  2. Parsing: the first word is looked up as a symbol (an alias or a
//     foreign command, dclsym.go), once: an alias's value isn't looked up
//     again. Then each &NAME, after a blank or a special character and
//     outside quotes, is replaced by its value (substituteAmpersands),
//     once, left to right. In a command the console's grammar reads, the
//     value is one token, its case and blanks kept (VMS 7.3's DEFINE),
//     so it goes in quoted; in an expression it goes in as it is. A ":="
//     assignment, an "@" command's parameters, and a foreign command's
//     text get no ampersand substitution.
//  3. Evaluation: the symbols in an expression ("=", and later IF and
//     WRITE) are replaced by their values as it's evaluated (dclexpr.go).
//
// Data lines of a procedure, which go to the debugger or the assembler,
// are never scanned (12.13.3); neither is a debugger command.
//
// Unconfirmed against VMS: that ''NAME inside quotes needs its closing
// apostrophe; that a comment isn't scanned; that a foreign command's
// text gets no ampersand substitution; and that an &NAME's value becomes
// a quoted token (it might be read again for qualifiers).

// maxSubstitutions is how many apostrophe substitutions one command line
// may have: far more than a real line needs, short of a symbol whose
// value names itself ('A' with A = "'A'"), which would never end. VMS
// reports that case as CLI_EXPSYN.
const maxSubstitutions = 256

// substituteApostrophes is phase 1 for the console's command line: the
// line with every apostrophe substitution made.
func (c *Console) substituteApostrophes(line string) (string, error) {
	return substituteApostrophes(line, &c.dclSymbols, c)
}

// substituteApostrophes returns line with every apostrophe substitution
// made, the symbols coming from symbols, and c the console a lexical
// function asks (nil for a subprocess's CLI).
func substituteApostrophes(line string, symbols *dclSymbolTable, c *Console) (string, error) {
	quoted := false
	count := 0

	for i := 0; i < len(line); {
		switch ch := line[i]; {
		case ch == '"':
			// A doubled quote inside quotes turns the state off and on
			// again, so it needs nothing special.
			quoted = !quoted
			i++

		case ch == '!' && !quoted:
			// A comment isn't scanned, but for F$VERIFY between
			// apostrophes (dclverify.go).
			if c != nil {
				c.verifyInComment(line[i:], symbols)
			}

			return line, nil

		case ch == '\'' && quoted:
			if !strings.HasPrefix(line[i:], "''") {
				i++

				continue
			}

			term, err := substitutionTerm(line, i+2, true, symbols, c)
			if err != nil {
				return "", err
			}

			if !term.ok {
				i++

				continue
			}

			// Inside quotes the value isn't scanned again.
			line = line[:i] + term.value + line[term.end:]
			i += len(term.value)

		case ch == '\'':
			term, err := substitutionTerm(line, i+1, false, symbols, c)
			if err != nil {
				return "", err
			}

			if !term.ok {
				i++

				continue
			}

			if count++; count > maxSubstitutions {
				return "", vmserrors.NewSegment(vmserrors.CLI_EXPSYN, "'"+term.name)
			}

			// Outside quotes the value is scanned again, from where it
			// starts: iterative substitution.
			line = line[:i] + term.value + line[term.end:]

		default:
			i++
		}
	}

	return line, nil
}

// apostropheTerm is what follows a substitution's apostrophe: the symbol
// (or lexical function) named, the value to put in place of the
// apostrophes and the name (a string, or an integer's digits; "" for an
// undefined symbol), and where the text after them starts.
type apostropheTerm struct {
	ok    bool
	name  string
	value string
	end   int
}

// substitutionTerm reads what follows a substitution's apostrophe (or
// two) at start in line: blanks, a symbol name or a lexical function
// call, and the closing apostrophe, which only inside quotes (closing)
// must be there.
func substitutionTerm(line string, start int, closing bool, symbols *dclSymbolTable, c *Console) (apostropheTerm, error) {
	i := start
	if !closing {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
	}

	nameStart := i
	if i >= len(line) || !isDCLSymbolStart(line[i]) {
		return apostropheTerm{}, nil
	}

	for i < len(line) && isDCLNameChar(line[i]) {
		i++
	}

	term := apostropheTerm{name: line[nameStart:i]}
	call := false

	if len(term.name) > 2 && strings.EqualFold(term.name[:2], "F$") {
		j := i
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}

		if j < len(line) && line[j] == '(' {
			if paren, found := matchingParen(line, j); found {
				v, err := evaluateDCLExpression(line[nameStart:paren+1], symbols, c)
				if err != nil {
					return apostropheTerm{}, err
				}

				term.value, i, call = v.String(), paren+1, true
			}
		}
	}

	if !call {
		if sym, found := symbols.lookup(term.name); found {
			term.value = sym.value
		}
	}

	switch {
	case i < len(line) && line[i] == '\'':
		i++
	case closing:
		return apostropheTerm{}, nil
	}

	term.ok, term.end = true, i

	return term, nil
}

// matchingParen returns the index of the ")" that closes the "(" at open
// in line, passing over quoted strings and nested parentheses, and
// whether there is one.
func matchingParen(line string, open int) (int, bool) {
	depth, quoted := 0, false

	for i := open; i < len(line); i++ {
		switch ch := line[i]; {
		case ch == '"':
			quoted = !quoted
		case quoted:
		case ch == '(':
			depth++
		case ch == ')':
			if depth--; depth == 0 {
				return i, true
			}
		}
	}

	return 0, false
}

// substituteAmpersands is phase 2's substitution: line with each &NAME
// outside quotes, after a blank or a special character, replaced by the
// symbol's value ("" for an undefined one), left to right, and not
// scanned again. A comment ends the scan. With quote, the value goes in
// as a quoted string (a doubled quote for each quote in it), so that it
// is one token whose case the grammar keeps; otherwise as it is, for an
// expression.
func (t *dclSymbolTable) substituteAmpersands(line string, quote bool) string {
	var b strings.Builder

	quoted := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		switch {
		case ch == '"':
			quoted = !quoted
		case quoted:
		case ch == '!':
			return b.String() + line[i:]
		case ch == '&':
			value, end, ok := t.ampersandAt(line, i)
			if !ok {
				break
			}

			if quote {
				value = `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
			}

			b.WriteString(value)

			i = end - 1

			continue
		}

		b.WriteByte(ch)
	}

	return b.String()
}

// ampersandAt reports whether text has an &NAME substitution at i, which
// the caller has found outside quotes: an "&" at the start of text or
// after a blank or special character, and then a symbol name. It returns
// the symbol's value ("" for an undefined one) and where the name ends.
func (t *dclSymbolTable) ampersandAt(text string, i int) (value string, end int, ok bool) {
	if text[i] != '&' || (i > 0 && isDCLNameChar(text[i-1])) ||
		i+1 >= len(text) || !isDCLSymbolStart(text[i+1]) {
		return "", 0, false
	}

	end = i + 1
	for end < len(text) && isDCLNameChar(text[end]) {
		end++
	}

	if sym, found := t.lookup(text[i+1 : end]); found {
		value = sym.value
	}

	return value, end, true
}

// upcaseOutsideQuotes returns s with every letter outside quotes made
// uppercase, as DCL uppercases a command line before its second phase.
func upcaseOutsideQuotes(s string) string {
	var b strings.Builder

	quoted := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		switch {
		case ch == '"':
			quoted = !quoted
		case !quoted && ch >= 'a' && ch <= 'z':
			ch -= 'a' - 'A'
		}

		b.WriteByte(ch)
	}

	return b.String()
}
