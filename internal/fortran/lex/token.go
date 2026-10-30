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
	// IntegerConst with Radix 8: it is an INTEGER, not typeless.)
	TypelessConst

	// LogicalConst is a logical constant: Text is .TRUE. or .FALSE.
	LogicalConst

	// Operator is everything else: ** * / // + - ( ) , = : % & ' and the
	// dotted operators, whose Text keeps the periods (.EQ., .AND.). A
	// period that starts no operator or number is the operator "." (as in
	// the record field references of Phase 82).
	Operator
)

// String names the kind, for dumps and tests.
func (k TokenKind) String() string {
	return [...]string{"name", "keyword", "integer", "real", "character", "hollerith",
		"typeless", "logical", "operator"}[k]
}

// Token is one word of a statement.
type Token struct {
	Kind TokenKind

	// Text is the token as written, in capitals outside constants (I,
	// .EQ., 1.5E3), or a constant's value (see the kinds).
	Text string

	// Radix is a TypelessConst token's base, 8 or 16, or 8 for an octal
	// IntegerConst ("17); 0 for every other token.
	Radix int

	// Offset is where the token starts in its statement's text
	// (source.Statement.Text), which Statement.Position turns into a file,
	// line, and column.
	Offset int
}

// String shows the token for dumps and tests: its kind and text, the
// text of a constant quoted.
func (t Token) String() string {
	switch t.Kind {
	case CharConst, HollerithConst:
		return fmt.Sprintf("%s %q", t.Kind, t.Text)
	case TypelessConst:
		return fmt.Sprintf("%s %q radix %d", t.Kind, t.Text, t.Radix)
	}

	return t.Kind.String() + " " + t.Text
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
