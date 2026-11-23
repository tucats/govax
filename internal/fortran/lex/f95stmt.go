package lex

// Fortran 95's statements (docs/planning/PHASE-112 - Fortran 95.md, subtask
// 2): the tests classify needs beyond Fortran 90's.
//
//   - PURE and ELEMENTAL, prefixes of a FUNCTION or SUBROUTINE statement
//     beside RECURSIVE, in any order, before the type or between it and
//     FUNCTION (prefixed, prefixWords);
//   - FORALL, told apart as WHERE is (forallStatement);
//   - and a recovery: a unit's first statement with words the lexer doesn't
//     know before FUNCTION or SUBROUTINE is still read as the unit's header,
//     with a message, so the parser keeps the units' structure (lostHeader).
//     Without it one IMPURE SUBROUTINE in a module costs a dozen messages
//     about the statements after it.

// prefixes are the words that may stand before FUNCTION or SUBROUTINE (the
// HP Fortran manual's 8.5.1, R1219). None starts another.
var prefixes = []string{"RECURSIVE", "ELEMENTAL", "PURE"}

// prefixSet is the prefixes a statement gives, each with where it is in the
// squeezed text, in order.
type prefixSet struct {
	words []string
	at    []int
}

// prefixWords reads the prefixes the squeezed text from p to j starts with
// into given, returning where the text after them starts. Squeezed text has
// no blanks, so PUREFUNCTION is PURE and FUNCTION; this is asked only where a
// header may be (a unit's first statement), where a variable PUREFUNCTIONF
// can't be declared without a type keyword first.
func (l *lexer) prefixWords(p, j int, given *prefixSet) int {
	for {
		found := false

		for _, word := range prefixes {
			if l.hasPrefix(p, j, word) {
				l.keywordBlanks(p, word)
				given.words = append(given.words, word)
				given.at = append(given.at, p)
				p += len(word)
				found = true

				break
			}
		}

		if !found {
			return p
		}
	}
}

// apply sets the statement's flags for the prefixes given, reporting one it
// has already (DUPPREFIX): the same word twice in one place, or before the
// type and again after it.
func (given *prefixSet) apply(st *Statement, l *lexer) {
	for n, word := range given.words {
		var flag *bool

		switch word {
		case "RECURSIVE":
			flag = &st.Recursive
		case "PURE":
			flag = &st.Pure
		default:
			flag = &st.Elemental
		}

		if *flag {
			l.messages.Report("DUPPREFIX", l.src.Position(l.offset(given.at[n])), word)
		}

		*flag = true
	}
}

// headerPrefixes classifies the squeezed text from i to j, a unit's first
// statement, if it starts with prefixes and goes on to a FUNCTION or
// SUBROUTINE statement (PURE INTEGER FUNCTION F, RECURSIVE ELEMENTAL
// SUBROUTINE S); otherwise it returns nil, and classify goes on as if it
// hadn't looked.
func (l *lexer) headerPrefixes(st *Statement, i, j int) *Statement {
	var given prefixSet

	p := l.prefixWords(i, j, &given)
	if len(given.words) == 0 {
		return nil
	}

	fn := l.classify(p, j, true)
	if fn.Kind != Function && fn.Kind != Subroutine {
		return nil
	}

	given.apply(fn, l)
	fn.Offset = st.Offset

	return fn
}

// headerWords are the keywords lostHeader looks for, with the prefixes, so
// that IMPURE ELEMENTAL SUBROUTINE keeps its ELEMENTAL.
var headerWords = []string{"FUNCTION", "SUBROUTINE"}

// lostHeader recovers a unit's first statement that would otherwise be
// unrecognized, IMPURE ELEMENTAL SUBROUTINE S or NONRECURSIVE FUNCTION F:
// letters, digits, and underscores (in free form, words ending at a blank)
// before a prefix, FUNCTION, or SUBROUTINE, and from there a statement of
// that kind. The words are reported (UNRECPREFIX), and the statement from the
// known word on is the unit's header. Otherwise nil, an unrecognized
// statement as before.
func (l *lexer) lostHeader(st *Statement, i, j int) *Statement {
	text, lit := l.sq.text, l.sq.lit

	for k := i + 1; k < j && isNameChar(text[k-1]) && !lit[k-1]; k++ {
		if l.src.Free && !l.sq.gap[k] {
			continue
		}

		for _, word := range append(headerWords, prefixes...) {
			if !l.hasPrefix(k, j, word) {
				continue
			}

			if fn := l.classify(k, j, true); fn.Kind == Function || fn.Kind == Subroutine {
				l.messages.Report("UNRECPREFIX", l.src.Position(st.Offset), string(text[i:k]), fn.Kind.String())
				fn.Offset = st.Offset

				return fn
			}
		}
	}

	return nil
}

// forallStatement finishes classifying a FORALL statement whose header's
// parentheses are at open and close (close is -1 if the header is never
// closed): with nothing after the header, it starts a FORALL construct
// (ForallBlock); otherwise what follows is the assignment it controls, lexed
// as a statement of its own and hung on it as Then, as a WHERE's is.
func (l *lexer) forallStatement(st *Statement, open, close, j int) *Statement {
	st.Kind = ForallBlock

	if close < 0 {
		st.Tokens = l.tokenize(open, j, modeNone)

		return st
	}

	st.Tokens = l.tokenize(open, close+1, modeNone)

	if close+1 < j {
		st.Kind = Forall
		st.Then = l.classify(close+1, j, false)
	}

	return st
}
