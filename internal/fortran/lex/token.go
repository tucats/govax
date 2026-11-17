package lex

import "fmt"

// TokenKind is what kind of word a token is.
type TokenKind int

// The kinds of token.
const (
	// Name is a symbolic name: TOTAL, I, SYS$GETJPI (section 1.1.3).
	Name TokenKind = iota

	// Keyword is a keyword inside a statement, which the lexer split off
	// the text around it because the statement's kind says it is there:
	// THEN in a block IF, FUNCTION in a typed FUNCTION statement. (The
	// keyword that starts a statement isn't a token; it is the
	// statement's Kind.)
	Keyword

	// IntegerConst is an integer constant: digits, without a sign (a sign is
	// an operator, which the parser applies). Its Radix is 0 (decimal), or 8 for
	// appendix A.5's "17, whose Text is the digits after the quote.
	IntegerConst

	// RealConst is a real constant: 1.5, 2., .5, 1E3, 2.5D0, 1Q0.
	RealConst

	// CharConst is a character constant. Text is its value, without the
	// apostrophes and with each doubled apostrophe made one.
	CharConst

	// HollerithConst is a Hollerith constant, nHxxx: Text is the n characters
	// after the H.
	HollerithConst

	// TypelessConst is an octal or hexadecimal constant: '17'O or '1F'X.
	// Text is its digits, and Radix 8 or 16. (Appendix A.5's "17 is an
	// IntegerConst with Radix 8: it is an INTEGER, not typeless.) Fortran
	// 90's B'101', O'17', and Z'1F' (the standard's R407) are typeless
	// constants too, Prefixed, with Radix 2, 8, or 16.
	TypelessConst

	// Radix50Const is a RADIX-50 constant, nRxxx (appendix B.3): Text is the
	// n characters after the R, lowercase letters made capitals.
	Radix50Const

	// LogicalConst is a logical constant: Text is .TRUE. or .FALSE.
	LogicalConst

	// Operator is everything else: ** * / // + - ( ) , = : % & ' and the
	// dotted operators, whose Text keeps the periods (.EQ., .AND.). A
	// period that starts no operator or number is the operator "." (as in
	// the record field references of Phase 82).
	//
	// Fortran 90 adds :: => (/ /) == /= < <= > >= (the standard's 3.2.3 and
	// 3.2.5), each one token, and defined operators, a dotted word that is
	// none of the known ones (.CROSS.; DefinedOperator). The relational
	// spellings keep their text as written; the parser reads == as .EQ.,
	// and so on.
	Operator
)

// String names the kind, for dumps and tests.
func (k TokenKind) String() string {
	return [...]string{"name", "keyword", "integer", "real", "character", "hollerith",
		"typeless", "radix50", "logical", "operator"}[k]
}

// Token is one word of a statement.
type Token struct {
	Kind TokenKind

	// Text is the token as written, in capitals outside constants (I,
	// .EQ., 1.5E3), or a constant's value (see the kinds).
	Text string

	// Radix is a TypelessConst token's base, 2, 8, or 16, or 8 for an octal
	// IntegerConst ("17); 0 for every other token.
	Radix int

	// Prefixed says a TypelessConst was written with its base's letter
	// first, Fortran 90's form (Z'1F'), rather than DEC's ('1F'X).
	Prefixed bool

	// KindParam is a constant's kind parameter as written, digits or a
	// name: the 8 of 1.5_8, the K of 2_K and K_'text' (the standard's R404,
	// R413, R420, R421). Empty for a constant without one, and for every
	// other token.
	KindParam string

	// Offset is where the token starts in its statement's text
	// (source.Statement.Text), which Statement.Position turns into a file,
	// line, and column.
	Offset int
}

// String shows the token for dumps and tests: its kind and text, the
// text of a constant quoted, and a kind parameter.
func (t Token) String() string {
	kind := ""
	if t.KindParam != "" {
		kind = " kind " + t.KindParam
	}

	switch t.Kind {
	case CharConst, HollerithConst, Radix50Const:
		return fmt.Sprintf("%s %q%s", t.Kind, t.Text, kind)
	case TypelessConst:
		return fmt.Sprintf("%s %q radix %d", t.Kind, t.Text, t.Radix)
	}

	return t.Kind.String() + " " + t.Text + kind
}

// DefinedOperator reports whether the token is a defined operator: a dotted
// word, .CROSS., that isn't one of FORTRAN's own (the standard's R311).
func (t Token) DefinedOperator() bool {
	if t.Kind != Operator || len(t.Text) < 3 || t.Text[0] != '.' || t.Text[len(t.Text)-1] != '.' {
		return false
	}

	_, known := dotted[t.Text[1:len(t.Text)-1]]

	return !known
}

// Is reports whether the token is the operator or keyword text: an easy
// test for the parser (tok.Is("(")).
func (t Token) Is(text string) bool {
	return (t.Kind == Operator || t.Kind == Keyword) && t.Text == text
}

// dotted are the words that may stand between periods: the relational and
// logical operators, and the logical constants (sections 2.3.3, 2.3.4,
// 2.2.1.5).
var dotted = map[string]TokenKind{
	"EQ": Operator, "NE": Operator, "LT": Operator, "LE": Operator, "GT": Operator, "GE": Operator,
	"AND": Operator, "OR": Operator, "NOT": Operator, "EQV": Operator, "NEQV": Operator, "XOR": Operator,
	"TRUE": LogicalConst, "FALSE": LogicalConst,
}
