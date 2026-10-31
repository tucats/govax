package lex

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/source"
)

// Statement is one statement, lexed: its kind and its tokens.
type Statement struct {
	// Kind is what statement it is.
	Kind Kind

	// Label is its statement label, or 0.
	Label int

	// Tokens are the statement's words after its keyword (all of them for
	// an assignment, which has none). For the IF statements they are the
	// parenthesized condition, then THEN as a Keyword token for a block
	// IF or ELSE IF, or the three labels of an arithmetic IF.
	Tokens []Token

	// Then is the statement a logical IF executes when its condition is
	// true (IF (X) GO TO 10: a GO TO statement), lexed as a statement of
	// its own; nil for every other kind.
	Then *Statement

	// FunctionType is the data type keyword of a typed FUNCTION
	// statement (Integer for INTEGER FUNCTION F(X)), and Unknown for an
	// untyped one or any other statement. The type's length, if any, is
	// among the tokens before the FUNCTION keyword token.
	FunctionType Kind

	// Offset is where the statement (its keyword, or its first token)
	// starts in the source statement's text, for messages about the
	// statement as a whole.
	Offset int

	// Body is where the text after a keyword statement's keyword starts
	// in the source statement's text. A FORMAT statement has no tokens:
	// its specification is read from here (Phase 80).
	Body int

	// Source is the statement the source pass read, whose Position maps
	// a token's Offset back to its file, line, and column.
	Source *source.Statement
}

// Position is where offset, an Offset of the statement or one of its
// tokens, is in the source.
func (s *Statement) Position(offset int) diag.Position {
	return s.Source.Position(offset)
}

// Lex lexes the statements the source pass read, in order, reporting
// what it finds wrong to messages. Every statement gets a Statement, even
// one with a problem (an Unknown, or one whose bad characters were
// skipped), so that later passes still see its label.
func Lex(statements []*source.Statement, messages *diag.List) []*Statement {
	l := &lexer{messages: messages}
	out := make([]*Statement, 0, len(statements))

	// unitStart is true where a program unit may begin: at the start of
	// the file, and after an END. Only there can INTEGER FUNCTIONF(X) be a
	// FUNCTION statement (classify's typed FUNCTION test).
	unitStart := true
	l.specPart = true

	for _, src := range statements {
		st := l.statement(src, unitStart)
		out = append(out, st)

		switch st.Kind {
		case End:
			unitStart, l.specPart = true, true
		case Options, Directive, Empty:
		default:
			unitStart = false
			l.specPart = l.specPart && !st.Kind.Executable()
		}
	}

	return out
}

// Expression lexes the text of src from offset from to offset to as an
// expression's tokens, with offsets into the statement's text as a token's
// always are. A FORMAT statement has no tokens, but a variable format
// expression inside it (section 8.3.2) is an expression like any other.
func Expression(src *source.Statement, from, to int, messages *diag.List) []Token {
	sq, problems := squeeze(src.Text[from:to])
	for _, p := range problems {
		messages.Report(p.ident, src.Position(from+p.offset), p.args...)
	}

	for i := range sq.at {
		sq.at[i] += from
	}

	l := &lexer{messages: messages, src: src, sq: sq}

	return l.tokenize(0, len(sq.text), modeNone)
}

// lexer is Lex's state for one statement.
type lexer struct {
	messages *diag.List

	src *source.Statement
	sq  *squeezed

	// specPart is true until the unit's first executable statement:
	// only there is PARAMETER P=1 appendix A.4's PARAMETER statement
	// rather than an assignment to PARAMETERP (oldParameter).
	specPart bool
}

// statement lexes one statement.
func (l *lexer) statement(src *source.Statement, unitStart bool) *Statement {
	sq, problems := squeeze(src.Text)
	for _, p := range problems {
		l.messages.Report(p.ident, src.Position(p.offset), p.args...)
	}

	l.src, l.sq = src, sq

	if src.Directive {
		// A CDEC$ directive's text (what follows the tag) is its name,
		// IDENT, PSECT, TITLE, or SUBTITLE, and the name's operands: all
		// of it tokens, the name first.
		return &Statement{Kind: Directive, Offset: l.offset(0), Tokens: l.tokenize(0, len(sq.text), modeNone), Source: src}
	}

	// Section 5.5: an initial line holding only END is an END statement,
	// whatever continuation lines follow it.
	if len(src.Initial()) < len(src.Text) {
		if initial, _ := squeeze(src.Initial()); string(initial.text) == "END" {
			return &Statement{Kind: End, Label: src.Label, Offset: initial.at[0], Source: src}
		}
	}

	st := l.classify(0, len(sq.text), unitStart)
	st.Label = src.Label

	for s := st; s != nil; s = s.Then {
		if s.Kind == Unknown {
			l.messages.Report("UNRECSTMT", src.Position(s.Offset))
		}
	}

	return st
}

// classify decides what statement the squeezed text from i to j is, and
// tokenizes it. This is the heart of the lexer: see doc.go for why
// FORTRAN must decide a statement's kind before it can split it into
// tokens, and the order of the tests below.
func (l *lexer) classify(i, j int, unitStart bool) *Statement {
	st := &Statement{Source: l.src, Offset: l.offset(i)}

	if i == j {
		st.Kind = Empty

		return st
	}

	// The IF statements, and ELSE IF: what follows the condition's
	// closing parenthesis says which. If it is =, the "IF" is an array
	// named IF, and the statement is an assignment to one of its
	// elements.
	for _, kw := range []string{"IF(", "ELSEIF("} {
		if !l.hasPrefix(i, j, kw) {
			continue
		}

		open := i + len(kw) - 1
		close := l.matchParen(open, j)

		if close >= 0 && close+1 < j && l.sq.text[close+1] == '=' && !l.sq.lit[close+1] {
			break // an assignment, found below
		}

		return l.ifStatement(st, kw == "ELSEIF(", open, close, j)
	}

	// An = outside parentheses: an assignment, or a DO statement if a
	// comma follows the = outside parentheses too. (OPTIONS
	// /CHECK=ALL has one, but no assignment's target is followed by a
	// slash.)
	if eq := l.findLevel0(i, j, '='); eq >= 0 && !l.hasPrefix(i, j, "OPTIONS/") {
		if l.hasPrefix(i, j, "DO") && l.findLevel0(eq+1, j, ',') >= 0 {
			st.Kind = Do
			st.Tokens = l.tokenize(i+2, j, modeLabel)

			return st
		}

		if l.oldParameter(i, eq) {
			st.Kind = Parameter
			st.Body = l.offset(i + len("PARAMETER"))
			st.Tokens = l.tokenize(i+len("PARAMETER"), j, modeNone)

			return st
		}

		st.Kind = Assignment
		st.Tokens = l.tokenize(i, j, modeNone)

		return st
	}

	// A keyword statement: the longest keyword the text starts with.
	for _, k := range byKeyword {
		kw := kinds[k].keyword
		if !l.hasPrefix(i, j, kw) {
			continue
		}

		if k.TypeKeyword() && unitStart {
			if fn := l.typedFunction(st, k, i+len(kw), j); fn != nil {
				return fn
			}
		}

		st.Kind = k
		st.Body = l.offset(i + len(kw))

		// A FORMAT statement's specification isn't made of FORTRAN's
		// tokens (F8.2 is one edit descriptor, not a name and a number),
		// so it isn't tokenized: its parser reads the text from Body.
		if k != Format {
			st.Tokens = l.tokenize(i+len(kw), j, modeFor(k))
		}

		return st
	}

	// DO WHILE, and a DO statement too broken for the = test (DO 10 I
	// 1, 5), which the parser reports.
	if l.hasPrefix(i, j, "DO") {
		p := i + 2
		for p < j && isDigit(l.sq.text[p]) {
			p++
		}

		if p < j && l.sq.text[p] == ',' {
			p++
		}

		if l.hasPrefix(p, j, "WHILE(") {
			st.Kind = DoWhile
			st.Tokens = l.tokenize(i+2, p, modeLabel)
			st.Tokens = append(st.Tokens, Token{Kind: Keyword, Text: "WHILE", Offset: l.offset(p)})
			st.Tokens = append(st.Tokens, l.tokenize(p+len("WHILE"), j, modeNone)...)

			return st
		}

		st.Kind = Do
		st.Tokens = l.tokenize(i+2, j, modeLabel)

		return st
	}

	st.Kind = Unknown

	return st
}

// ifStatement finishes classifying an IF or ELSE IF statement whose
// condition's parentheses are at open and close (close is -1 if the
// condition is never closed).
func (l *lexer) ifStatement(st *Statement, elseIf bool, open, close, j int) *Statement {
	st.Kind = LogicalIf
	if elseIf {
		st.Kind = ElseIf
	}

	if close < 0 {
		// No closing parenthesis: the parser reports it.
		st.Tokens = l.tokenize(open, j, modeNone)

		return st
	}

	st.Tokens = l.tokenize(open, close+1, modeNone)
	rest := close + 1

	switch {
	case l.isWord(rest, j, "THEN"):
		if !elseIf {
			st.Kind = IfThen
		}

		st.Tokens = append(st.Tokens, Token{Kind: Keyword, Text: "THEN", Offset: l.offset(rest)})

	case elseIf, rest == j:
		// ELSE IF without THEN, or an IF with nothing after its
		// condition: the parser reports what is missing.
		st.Tokens = append(st.Tokens, l.tokenize(rest, j, modeNone)...)

	case isDigit(l.sq.text[rest]):
		st.Kind = ArithmeticIf
		st.Tokens = append(st.Tokens, l.tokenize(rest, j, modeNone)...)

	default:
		// The IF is the unit's first executable statement if nothing
		// before it was, so what it executes is past the specification
		// part.
		spec := l.specPart
		l.specPart = false
		st.Then = l.classify(rest, j, false)
		l.specPart = spec
	}

	return st
}

// oldParameter reports whether the squeezed text from i, whose first =
// at level 0 is at eq, is appendix A.4's PARAMETER statement, PARAMETER
// p=c [,p=c]..., which has no parentheses. Squeezed, PARAMETER PI=3.14
// is PARAMETERPI=3.14, the same text as an assignment to a variable
// named PARAMETERPI, so the text alone can't tell them apart. The rule
// here is ours (docs/PHASE-76 - FORTRAN front end.md, Part A, unconfirmed
// on VMS): in a unit's specification part, before its first executable
// statement, text that starts with PARAMETER and then a name running up
// to the = is the PARAMETER statement. An assignment there would be the
// unit's first executable statement, and a statement function's
// definition (PARAMETERF(X) = ...) has a parenthesis before its =.
func (l *lexer) oldParameter(i, eq int) bool {
	const kw = "PARAMETER"

	if !l.specPart || !l.hasPrefix(i, eq, kw) || i+len(kw) >= eq || !isLetter(l.sq.text[i+len(kw)]) {
		return false
	}

	for k := i + len(kw); k < eq; k++ {
		if !isNameChar(l.sq.text[k]) || l.sq.lit[k] {
			return false
		}
	}

	return true
}

// typedFunction recognizes a typed FUNCTION statement (INTEGER FUNCTION
// F(X), REAL*8 FUNCTION G, CHARACTER*(*) FUNCTION H(S)), whose type
// keyword k ends at i, or returns nil. Squeezed, INTEGER FUNCTION F(10)
// and INTEGER FUNCTIONF(10), which declares an array named FUNCTIONF,
// are the same text; a FUNCTION statement can only be a program unit's
// first statement, so only there is the text taken for one.
func (l *lexer) typedFunction(st *Statement, k Kind, i, j int) *Statement {
	p := i

	// The type's length: *n or *(...).
	if p < j && l.sq.text[p] == '*' {
		p++

		switch {
		case p < j && l.sq.text[p] == '(':
			if p = l.matchParen(p, j); p < 0 {
				return nil
			}

			p++

		default:
			for p < j && isDigit(l.sq.text[p]) {
				p++
			}
		}
	}

	const fn = "FUNCTION"

	if !l.hasPrefix(p, j, fn) || p+len(fn) >= j || !isLetter(l.sq.text[p+len(fn)]) {
		return nil
	}

	st.Kind = Function
	st.FunctionType = k
	st.Tokens = l.tokenize(i, p, modeLengths)
	st.Tokens = append(st.Tokens, Token{Kind: Keyword, Text: fn, Offset: l.offset(p)})
	st.Tokens = append(st.Tokens, l.tokenize(p+len(fn), j, modeLengths)...)

	return st
}

// modeFor is how the text after keyword statement k's keyword is
// tokenized.
func modeFor(k Kind) tokenMode {
	switch {
	case k.TypeKeyword(), k == Implicit, k == Function:
		return modeLengths
	case k == Do:
		return modeLabel
	}

	return modeNone
}

// hasPrefix reports whether the squeezed text from i to j starts with
// word, outside any constant.
func (l *lexer) hasPrefix(i, j int, word string) bool {
	if j-i < len(word) {
		return false
	}

	for k := 0; k < len(word); k++ {
		if l.sq.text[i+k] != word[k] || l.sq.lit[i+k] {
			return false
		}
	}

	return true
}

// isWord reports whether the squeezed text from i to j is exactly word.
func (l *lexer) isWord(i, j int, word string) bool {
	return j-i == len(word) && l.hasPrefix(i, j, word)
}

// matchParen returns the index of the parenthesis that closes the one at
// open, before j, or -1 if none does. Parentheses in constants don't
// count.
func (l *lexer) matchParen(open, j int) int {
	depth := 0

	for k := open; k < j; k++ {
		if l.sq.lit[k] {
			continue
		}

		switch l.sq.text[k] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return k
			}
		}
	}

	return -1
}

// findLevel0 returns the index of the first c from i to j that is
// outside every parenthesis and constant, or -1.
func (l *lexer) findLevel0(i, j int, c byte) int {
	depth := 0

	for k := i; k < j; k++ {
		if l.sq.lit[k] {
			continue
		}

		switch l.sq.text[k] {
		case '(':
			depth++
		case ')':
			depth--
		case c:
			if depth == 0 {
				return k
			}
		}
	}

	return -1
}

// offset is where the squeezed text's character i was in the statement's
// text; the end of the text for i past it.
func (l *lexer) offset(i int) int {
	if i < len(l.sq.at) {
		return l.sq.at[i]
	}

	return len(l.src.Text)
}

// Dump shows a statement as one line, for tests: its label, its kind, its
// tokens in short form, and after a / the statement a logical IF
// executes.
//
//	10 logical IF: ( X .GT. 0 ) / GO TO: 20
func Dump(s *Statement) string {
	var b strings.Builder

	if s.Label != 0 {
		b.WriteString(strconv.Itoa(s.Label) + " ")
	}

	b.WriteString(s.Kind.String())

	if s.FunctionType != Unknown {
		b.WriteString(" (" + s.FunctionType.String() + ")")
	}

	b.WriteString(":")

	for _, t := range s.Tokens {
		b.WriteString(" " + t.short())
	}

	if s.Then != nil {
		b.WriteString(" / " + Dump(s.Then))
	}

	return b.String()
}

// short is the token as Dump shows it: names, keywords, numbers, and
// operators as written; a character constant in apostrophes; a Hollerith
// constant as nH"text"; a typeless one as 'digits'X or 'digits'O.
func (t Token) short() string {
	switch t.Kind {
	case CharConst:
		return "'" + strings.ReplaceAll(t.Text, "'", "''") + "'"
	case HollerithConst:
		return strconv.Itoa(len(t.Text)) + `H"` + t.Text + `"`
	case IntegerConst:
		if t.Radix == 8 {
			return `"` + t.Text
		}
	case TypelessConst:
		if t.Radix == 8 {
			return "'" + t.Text + "'O"
		}

		return "'" + t.Text + "'X"
	}

	return t.Text
}
