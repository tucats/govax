package lex

// Fortran 90's statements (docs/planning/PHASE-99 - Fortran 90.md, subtask 5):
// the tests classify needs beyond VAX FORTRAN's. Most of Fortran 90's
// statements start with a keyword no VAX FORTRAN statement starts with (SELECT
// CASE, ALLOCATE, USE, ...), and kinds.go's table finds them as it finds the
// old ones. The rest are here:
//
//   - a construct name before a colon (constructName);
//   - WHERE, told apart like IF (whereStatement);
//   - the two TYPE statements beside VAX FORTRAN's output statement
//     (typeStatement);
//   - END followed by the kind of unit it ends, and perhaps its name
//     (endStatement);
//   - and in free form, the blanks a keyword may have (keywordBlanks).
//
// classify itself looks for :: and => at level 0 (findPair): :: makes a
// statement a declaration even with an = in it, and => with no comma before
// it is a pointer assignment.

// constructName returns where the colon after a construct name is, if the
// squeezed text from i to j starts with one (OUTER: DO), or -1. A colon at
// level 0 straight after a name is nothing else in either language: a
// substring's colon is inside parentheses, and a :: is a declaration's.
func (l *lexer) constructName(i, j int) int {
	text, lit := l.sq.text, l.sq.lit

	if i >= j || !isLetter(text[i]) || lit[i] {
		return -1
	}

	k := i
	for k < j && isNameChar(text[k]) && !lit[k] {
		k++
	}

	if k+1 >= j || text[k] != ':' || lit[k] || text[k+1] == ':' {
		return -1
	}

	l.nameBlanks(i, k)

	return k
}

// whereStatement finishes classifying a WHERE statement whose mask's
// parentheses are at open and close (close is -1 if the mask is never
// closed): with nothing after the mask, it starts a WHERE construct
// (WhereBlock); otherwise what follows is the assignment it masks, lexed as
// a statement of its own and hung on it as Then, as a logical IF's is.
func (l *lexer) whereStatement(st *Statement, open, close, j int) *Statement {
	st.Kind = WhereBlock

	if close < 0 {
		st.Tokens = l.tokenize(open, j, modeNone)

		return st
	}

	st.Tokens = l.tokenize(open, close+1, modeNone)

	if close+1 < j {
		st.Kind = Where
		st.Then = l.classify(close+1, j, false)
	}

	return st
}

// typeStatement classifies a statement starting with TYPE, from i to j;
// declaration says a :: is at level 0. VAX FORTRAN's TYPE is an output
// statement (TYPE *, X; TYPE 10, X; TYPE F); Fortran 90's TYPE starts a
// derived type's definition (TYPE POINT, TYPE, PUBLIC :: POINT) or declares
// objects of one (TYPE (POINT) P, TYPE(POINT), POINTER :: P). The phase doc's
// table settles which is which:
//
//   - TYPE followed by a comma or :: is a definition, and TYPE (name)
//     followed by a name or with a :: anywhere is a declaration: neither is
//     ever VAX FORTRAN's output statement, which has a format first;
//   - TYPE and a name alone is a definition under /F90 and output (the
//     format in the variable named) without it.
//
// Everything else is the output statement.
func (l *lexer) typeStatement(st *Statement, i, j int, declaration bool) *Statement {
	text, lit := l.sq.text, l.sq.lit
	p := i + len("TYPE")

	st.Kind = Type

	switch {
	case p < j && !lit[p] && (text[p] == ',' || declaration && text[p] == ':'):
		st.Kind = TypeDef

	case p < j && !lit[p] && text[p] == '(':
		if close := l.matchParen(p, j); close >= 0 &&
			(declaration || close+1 < j && isLetter(text[close+1]) && !lit[close+1]) {
			st.Kind = TypeDecl
		}

	case l.src.F90 && l.isName(p, j):
		st.Kind = TypeDef
	}

	l.keywordBlanks(i, "TYPE")
	st.Body = l.offset(p)
	st.Tokens = l.tokenize(p, j, modeNone)

	return st
}

// endUnits are the words that may follow END to say what kind of unit it
// ends (the standard's R1103, R1218, R1222, R1105, R1111), longest first so
// that BLOCKDATA isn't taken for something shorter.
var endUnits = []string{"SUBROUTINE", "BLOCKDATA", "FUNCTION", "PROGRAM", "MODULE"}

// caseDefault reports whether the text from p to j, after CASE, starts with
// the keyword DEFAULT: DEFAULT alone, or followed by a construct name, which
// in free form must have a blank before it (CASE DEFAULTS is no DEFAULT), or
// by anything else, which the parser reports.
func (l *lexer) caseDefault(p, j int) bool {
	end := p + len("DEFAULT")

	switch {
	case !l.hasPrefix(p, j, "DEFAULT"):
		return false
	case end == j || !isNameChar(l.sq.text[end]):
		return true
	}

	return l.src == nil || !l.src.Free || l.sq.gap[end]
}

// endStatement finishes an END statement starting at i: plain END, or
// Fortran 90's END PROGRAM [name] and the rest. The word after END is a
// Keyword token (BLOCKDATA for BLOCK DATA) and the name, if any, a Name
// token, for the parser to check against the unit. Anything else after END
// is left as tokens, which the parser reports (EXTRATEXT), as it always has.
func (l *lexer) endStatement(st *Statement, i, j int) *Statement {
	p := i + len("END")

	l.keywordBlanks(i, "END")

	st.Kind = End
	st.Body = l.offset(p)

	for _, word := range endUnits {
		if l.hasPrefix(p, j, word) && (p+len(word) == j || isLetter(l.sq.text[p+len(word)])) {
			l.keywordBlanks(p, word)
			st.Tokens = append(st.Tokens, Token{Kind: Keyword, Text: word, Offset: l.offset(p)})
			st.Tokens = append(st.Tokens, l.tokenize(p+len(word), j, modeNone)...)

			return st
		}
	}

	st.Tokens = l.tokenize(p, j, modeNone)

	return st
}

// keywordSplits are the places a keyword may have a blank in free form (the
// standard's 3.3.1.1: BLOCK DATA, END DO, GO TO, SELECT CASE, and the rest),
// each a count of the keyword's letters before it. DEC's own compound
// keywords (END STRUCTURE, DEFINE FILE) and ELSE WHERE, which compilers
// accept, are here too. MODULE PROCEDURE's blank is required (requiredSplits).
var keywordSplits = map[string]int{
	"BLOCKDATA": 5, "DOUBLEPRECISION": 6, "DOUBLECOMPLEX": 6, "ELSEIF": 4, "ELSEWHERE": 4,
	"ENDDO": 3, "ENDFILE": 3, "ENDIF": 3, "ENDINTERFACE": 3, "ENDSELECT": 3, "ENDTYPE": 3,
	"ENDWHERE": 3, "ENDMAP": 3, "ENDSTRUCTURE": 3, "ENDUNION": 3, "GOTO": 2, "SELECTCASE": 6,
	"DEFINEFILE": 6, "MODULEPROCEDURE": 6,
}

// requiredSplits are the keywords of two words whose blank free form requires
// (the standard's 3.3.1.1, "blank mandatory"), the place as in keywordSplits.
// CASE DEFAULT, DO WHILE, IMPLICIT NONE, and the rest are a keyword and a
// token, whose blank checkBlanks requires.
var requiredSplits = map[string]int{"MODULEPROCEDURE": 6}

// keywordBlanks applies free form's rules for blanks to the keyword kw, which
// starts at i in the squeezed text: a blank inside it is BLANKINNAME, but where
// keywordSplits allows one, and one requiredSplits asks for is NEEDBLANK if
// missing. GO TO and GOTO are both GO TO; G O T O is an error. In fixed form
// blanks mean nothing, and this does nothing.
func (l *lexer) keywordBlanks(i int, kw string) {
	if l.src == nil || !l.src.Free {
		return
	}

	split, allowed := keywordSplits[kw]

	for m := i + 1; m < i+len(kw); m++ {
		if l.sq.gap[m] && !(allowed && m == i+split) {
			l.messages.Report("BLANKINNAME", l.src.Position(l.offset(m)), "a keyword")

			return
		}
	}

	if r, ok := requiredSplits[kw]; ok && !l.sq.gap[i+r] {
		l.messages.Report("NEEDBLANK", l.src.Position(l.offset(i+r)), kw[:r], kw[r:])
	}
}

// nameBlanks reports a blank inside the name from i to k in free form, as
// checkBlanks does for a name token: for a construct name, which is no token.
func (l *lexer) nameBlanks(i, k int) {
	if l.src == nil || !l.src.Free {
		return
	}

	for m := i + 1; m < k; m++ {
		if l.sq.gap[m] {
			l.messages.Report("BLANKINNAME", l.src.Position(l.offset(m)), "a name")

			return
		}
	}
}

// findPair returns the index of the first a followed at once by b from i to
// j, both outside every parenthesis and constant, or -1: Fortran 90's :: and
// =>.
func (l *lexer) findPair(i, j int, a, b byte) int {
	for k := l.findLevel0(i, j, a); k >= 0; k = l.findLevel0(k+1, j, a) {
		if k+1 < j && l.sq.text[k+1] == b && !l.sq.lit[k+1] {
			return k
		}
	}

	return -1
}

// isName reports whether the squeezed text from i to j is one name: a letter,
// then letters, digits, $, and _, none in a constant.
func (l *lexer) isName(i, j int) bool {
	if i >= j || !isLetter(l.sq.text[i]) {
		return false
	}

	for k := i; k < j; k++ {
		if !isNameChar(l.sq.text[k]) || l.sq.lit[k] {
			return false
		}
	}

	return true
}
