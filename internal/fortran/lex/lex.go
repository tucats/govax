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

	// Recursive is set for a FUNCTION or SUBROUTINE statement that says
	// RECURSIVE, before the type or FUNCTION or SUBROUTINE, or between the type
	// and FUNCTION (the HP Fortran manual's 8.5.2 and 8.5.3; docs/planning/PHASE-89 -
	// reentrant FORTRAN.md). The keyword is no token.
	Recursive bool

	// Construct is the construct name a DO, DO WHILE, block IF, or SELECT
	// CASE statement's text starts with (OUTER: DO I = 1, N; the standard's
	// 8.1), or "" for none, and ConstructOffset is where it starts. The name
	// a construct's other statements may end with (END DO OUTER) is among
	// their tokens. Semantic analysis checks them (sema/constructs.go).
	Construct       string
	ConstructOffset int

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

		case Contains, Interface, ModuleProcedure:
			// Fortran 90's procedures inside others: after CONTAINS, and
			// in an interface block, a FUNCTION or SUBROUTINE statement
			// starts a procedure as a unit's header does.
			unitStart, l.specPart = true, true
		case Options, Directive, Empty:
		default:
			unitStart = false
			l.specPart = l.specPart && !st.Kind.Executable()
		}

		switch st.Kind {
		case Interface:
			l.interfaces++
		case EndInterface:
			l.interfaces = max(l.interfaces-1, 0)
		}
	}

	return out
}

// Expression lexes the text of src from offset from to offset to as an
// expression's tokens, with offsets into the statement's text as a token's
// always are. A FORMAT statement has no tokens, but a variable format
// expression inside it (section 8.3.2) is an expression like any other.
func Expression(src *source.Statement, from, to int, messages *diag.List) []Token {
	sq, problems := squeeze(src.Text[from:to], src.F90)
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

	// interfaces counts the interface blocks open (INTERFACE, not yet
	// END INTERFACE): only inside one is MODULE PROCEDURE a statement,
	// and not a MODULE statement naming a module PROCEDUREx.
	interfaces int
}

// statement lexes one statement.
func (l *lexer) statement(src *source.Statement, unitStart bool) *Statement {
	sq, problems := squeeze(src.Text, src.F90)
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
		if initial, _ := squeeze(src.Initial(), src.F90); string(initial.text) == "END" {
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

	// A construct name and its colon (OUTER: DO): no statement of VAX
	// FORTRAN has a colon at level 0, so a name and one there can only be
	// Fortran 90's construct name. What follows is classified on its own.
	if colon := l.constructName(i, j); colon >= 0 {
		inner := l.classify(colon+1, j, unitStart)
		inner.Construct, inner.ConstructOffset = string(l.sq.text[i:colon]), st.Offset

		switch inner.Kind {
		case Do, DoWhile, IfThen, SelectCase, Unknown:
		default:
			l.messages.Report("NOTCONSTRUCT", l.src.Position(st.Offset), inner.Kind.String())
			inner.Construct = ""
		}

		return inner
	}

	// The IF statements, and ELSE IF: what follows the condition's
	// closing parenthesis says which. If it is =, the "IF" is an array
	// named IF, and the statement is an assignment to one of its
	// elements. Fortran 90's WHERE is told apart the same way.
	for _, kw := range []string{"IF(", "ELSEIF(", "WHERE("} {
		if !l.hasPrefix(i, j, kw) {
			continue
		}

		open := i + len(kw) - 1
		close := l.matchParen(open, j)

		if close >= 0 && close+1 < j && l.sq.text[close+1] == '=' && !l.sq.lit[close+1] {
			break // an assignment, found below
		}

		l.keywordBlanks(i, kw[:len(kw)-1])

		if kw == "WHERE(" {
			return l.whereStatement(st, open, close, j)
		}

		return l.ifStatement(st, kw == "ELSEIF(", open, close, j)
	}

	// A :: at level 0 is a Fortran 90 declaration's (INTEGER, PARAMETER ::
	// N = 10), whatever else is in it: its = is an initial value's, and its
	// keyword decides the kind, below.
	declaration := l.findPair(i, j, ':', ':') >= 0

	// A => at level 0 with no comma before it is a pointer assignment (P =>
	// T, X%NEXT => Y); USE's renames (USE M, A => B) have a comma first.
	if arrow := l.findPair(i, j, '=', '>'); arrow >= 0 && !declaration &&
		l.findAssignment(i, j) < 0 && l.findLevel0(i, arrow, ',') < 0 {
		st.Kind = PointerAssignment
		st.Tokens = l.tokenize(i, j, modeNone)

		return st
	}

	// An = outside parentheses: an assignment, or a DO statement if a
	// comma follows the = outside parentheses too. This is FORTRAN's famous
	// trap: DO 10 I = 1,5 is a DO loop, but DO 10 I = 1.5, with a period
	// for the comma, squeezes to DO10I=1.5, an assignment to a variable
	// named DO10I. Only the comma tells them apart. (OPTIONS
	// /CHECK=ALL has one, but no assignment's target is followed by a
	// slash.)
	if eq := l.findAssignment(i, j); eq >= 0 && !declaration && !l.hasPrefix(i, j, "OPTIONS/") {
		if l.hasPrefix(i, j, "DO") && l.findLevel0(eq+1, j, ',') >= 0 {
			l.keywordBlanks(i, "DO")
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

	// RECURSIVE FUNCTION F(N), RECURSIVE INTEGER*2 FUNCTION F(N), RECURSIVE
	// SUBROUTINE S: a unit's header, which only a unit's first statement can be.
	// No statement's keyword starts with RECURSIVE, and the = test above has
	// already taken RECURSIVE = 1.
	if unitStart && l.hasPrefix(i, j, recursive) {
		if fn := l.classify(i+len(recursive), j, true); (fn.Kind == Function || fn.Kind == Subroutine) && !fn.Recursive {
			fn.Recursive = true
			fn.Offset = st.Offset

			return fn
		}
	}

	// A keyword statement: the longest keyword the text starts with. The
	// text has no = at level 0 here (the test above took those), so it
	// can't be an assignment to a variable whose name starts like a
	// keyword; whatever it starts with is the keyword.
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

		switch k {
		case End:
			return l.endStatement(st, i, j)
		case Type:
			return l.typeStatement(st, i, j, declaration)
		case Module:
			// In an interface block, MODULE PROCEDURE names module
			// procedures; anywhere else MODULE starts a module, whose name
			// may start with PROCEDURE.
			if l.interfaces > 0 && l.hasPrefix(i, j, kinds[ModuleProcedure].keyword) {
				k, kw = ModuleProcedure, kinds[ModuleProcedure].keyword
			}
		}

		l.keywordBlanks(i, kw)

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
		// Skip the optional label and comma (DO 10, WHILE (...)) to see
		// whether WHILE( follows.
		p := i + 2
		for p < j && isDigit(l.sq.text[p]) {
			p++
		}

		if p < j && l.sq.text[p] == ',' {
			p++
		}

		l.keywordBlanks(i, "DO")

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

	case elseIf && l.hasPrefix(rest, j, "THEN") && l.isName(rest+len("THEN"), j):
		// Fortran 90's ELSE IF (e) THEN name, the name its IF construct's.
		st.Tokens = append(st.Tokens, Token{Kind: Keyword, Text: "THEN", Offset: l.offset(rest)})
		st.Tokens = append(st.Tokens, l.tokenize(rest+len("THEN"), j, modeNone)...)

	case elseIf, rest == j:
		// ELSE IF without THEN, or an IF with nothing after its
		// condition: the parser reports what is missing.
		st.Tokens = append(st.Tokens, l.tokenize(rest, j, modeNone)...)

	case isDigit(l.sq.text[rest]):
		st.Kind = ArithmeticIf
		st.Tokens = append(st.Tokens, l.tokenize(rest, j, modeNone)...)

	default:
		// A logical IF: IF (e) st, where st is any other executable
		// statement. Classify st on its own, a recursive call, and hang it
		// on the IF as Then.
		//
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
// here is ours (docs/planning/PHASE-76 - FORTRAN front end.md, Part A, unconfirmed
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

// recursive is the keyword that makes a subprogram recursive (Statement.Recursive).
const recursive = "RECURSIVE"

// typedFunction recognizes a typed FUNCTION statement (INTEGER FUNCTION
// F(X), REAL*8 FUNCTION G, CHARACTER*(*) FUNCTION H(S), INTEGER RECURSIVE
// FUNCTION F(N)), whose type keyword k ends at i, or returns nil. Squeezed, INTEGER FUNCTION F(10)
// and INTEGER FUNCTIONF(10), which declares an array named FUNCTIONF,
// are the same text; a FUNCTION statement can only be a program unit's
// first statement, so only there is the text taken for one.
func (l *lexer) typedFunction(st *Statement, k Kind, i, j int) *Statement {
	p := i

	// Fortran 90's kind or length selector, (...): INTEGER(4) FUNCTION F,
	// which the parser reads (typeSelector).
	if p < j && l.sq.text[p] == '(' && !l.sq.lit[p] {
		if p = l.matchParen(p, j); p < 0 {
			return nil
		}

		p++
	}

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

	length := p

	if l.hasPrefix(p, j, recursive+fn) {
		p += len(recursive)
		st.Recursive = true
	}

	if !l.hasPrefix(p, j, fn) || p+len(fn) >= j || !isLetter(l.sq.text[p+len(fn)]) {
		st.Recursive = false

		return nil
	}

	st.Kind = Function
	st.FunctionType = k
	st.Tokens = l.tokenize(i, length, modeLengths)
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
	// depth counts the parentheses open; the one at open makes it 1, and
	// the parenthesis that brings it back to 0 is the match.
	depth := 0

	for k := open; k < j; k++ {
		if l.sq.lit[k] {
			continue
		}

		switch l.sq.text[k] {
		case '(':
			depth++
		case ')':
			// An if statement may start with a simple statement, run before
			// the condition: here it decrements depth, then tests it.
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

// findAssignment returns the index of the first = from i to j that is
// outside every parenthesis and constant and isn't part of one of Fortran
// 90's relational operators, == /= <= >= (PRINT *, A == B is no assignment),
// or of a pointer assignment's =>, or -1. VAX FORTRAN has none of them, so old programs find the = they
// always did.
func (l *lexer) findAssignment(i, j int) int {
	text, lit := l.sq.text, l.sq.lit

	for k := l.findLevel0(i, j, '='); k >= 0; k = l.findLevel0(k+1, j, '=') {
		before := k > i && !lit[k-1] && strings.IndexByte("=/<>", text[k-1]) >= 0
		after := k+1 < j && !lit[k+1] && (text[k+1] == '=' || text[k+1] == '>')

		if !before && !after {
			return k
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

// Dump shows a statement as one line, for tests: its label, its construct
// name, its kind, its tokens in short form, and after a / the statement a
// logical IF or WHERE executes.
//
//	10 logical IF: ( X .GT. 0 ) / GO TO: 20
func Dump(s *Statement) string {
	var b strings.Builder

	if s.Label != 0 {
		b.WriteString(strconv.Itoa(s.Label) + " ")
	}

	if s.Construct != "" {
		b.WriteString(s.Construct + ": ")
	}

	b.WriteString(s.Kind.String())

	if s.FunctionType != Unknown {
		b.WriteString(" (" + s.FunctionType.String() + ")")
	}

	if s.Recursive {
		b.WriteString(" (recursive)")
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
// constant as nH"text"; a typeless one as 'digits'X or 'digits'O, or B'digits',
// O'digits', or Z'digits' as it was written; a kind parameter after its
// constant, or before a character constant (K_'text').
func (t Token) short() string {
	switch t.Kind {
	case CharConst:
		text := "'" + strings.ReplaceAll(t.Text, "'", "''") + "'"
		if t.KindParam != "" {
			return t.KindParam + "_" + text
		}

		return text
	case HollerithConst:
		return strconv.Itoa(len(t.Text)) + `H"` + t.Text + `"`
	case Radix50Const:
		return strconv.Itoa(len(t.Text)) + `R"` + t.Text + `"`
	case IntegerConst:
		if t.Radix == 8 {
			return `"` + t.Text
		}
	case TypelessConst:
		if t.Prefixed {
			return map[int]string{2: "B", 8: "O", 16: "Z"}[t.Radix] + "'" + t.Text + "'"
		}

		if t.Radix == 8 {
			return "'" + t.Text + "'O"
		}

		return "'" + t.Text + "'X"
	}

	if t.KindParam != "" {
		return t.Text + "_" + t.KindParam
	}

	return t.Text
}
