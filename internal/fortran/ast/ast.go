package ast

import (
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/source"
)

// Node is anything in the tree: it knows where in the source it is.
type Node interface {
	Position() diag.Position
}

// File is one source file's program units, in order.
type File struct {
	Units []*Unit

	// Trailing are OPTIONS statements and directives after the file's
	// last unit, which begin no unit (see Unit.Prologue).
	Trailing []Stmt
}

// UnitKind is what kind of program unit a unit is (section 1.1.1).
type UnitKind int

// The kinds of program unit.
const (
	MainProgram UnitKind = iota
	FunctionUnit
	SubroutineUnit
	BlockDataUnit
)

// String names the kind, for messages and dumps.
func (k UnitKind) String() string {
	return [...]string{"main program", "FUNCTION", "SUBROUTINE", "BLOCK DATA"}[k]
}

// Unit is one program unit: its header statement and the statements
// after it.
type Unit struct {
	// Prologue is what comes before the unit's first statement and
	// belongs to it: an OPTIONS statement, which must precede the header
	// (section 1.4.3), and CDEC$ directives between the previous unit's
	// END and this unit.
	Prologue []Stmt

	// Header is the unit's first statement: a *ProgramStmt, a
	// *FunctionStmt, a *SubroutineStmt, a *BlockDataStmt, a *BadStmt for
	// one of those with an error, or nil for a main program without a
	// PROGRAM statement.
	Header Stmt

	// Body is the statements after the header, in order. A unit the
	// parser finished properly ends with an *EndStmt.
	Body []Stmt
}

// Kind is what kind of unit this is, from its header.
func (u *Unit) Kind() UnitKind {
	if u.Header == nil {
		return MainProgram
	}

	// A header with an error still says what kind of unit it starts.
	switch u.Header.Base().Kind {
	case lex.Function:
		return FunctionUnit
	case lex.Subroutine:
		return SubroutineUnit
	case lex.BlockData:
		return BlockDataUnit
	}

	return MainProgram
}

// Name is the name its header gives the unit, or "" for a main program
// without a PROGRAM statement (which semantic analysis names after the
// source file, section 4.12) or a BLOCK DATA statement without a name.
func (u *Unit) Name() string {
	switch h := u.Header.(type) {
	case *ProgramStmt:
		return h.Name
	case *FunctionStmt:
		return h.Name
	case *SubroutineStmt:
		return h.Name
	case *BlockDataStmt:
		return h.Name
	}

	return ""
}

// Position is where the unit starts: its header, or its first statement.
func (u *Unit) Position() diag.Position {
	switch {
	case u.Header != nil:
		return u.Header.Position()
	case len(u.Body) > 0:
		return u.Body[0].Position()
	}

	return diag.Position{}
}

// Stmt is a statement. Every statement type embeds StmtBase.
type Stmt interface {
	Node

	// Base is the statement's label, position, and kind.
	Base() *StmtBase
}

// StmtBase is what every statement has.
type StmtBase struct {
	// Pos is where the statement starts: its keyword, or its first
	// token. Messages about the statement as a whole point here.
	Pos diag.Position

	// Label is the statement's label, or 0 if it has none (as a logical
	// IF's statement never has).
	Label int

	// Kind is the lexer's kind of the statement. Several kinds share a
	// node (PRINT, TYPE, and ACCEPT are all PrintStmts; SAVE, EXTERNAL,
	// and VOLATILE AttrStmts), and Kind says which statement it was.
	Kind lex.Kind

	// Source is the statement the source pass read, which says which of
	// the source's lines it came from (its First and Last lines) and
	// where its label is (LabelPosition).
	Source *source.Statement
}

// Position is where the statement starts.
func (b *StmtBase) Position() diag.Position { return b.Pos }

// Base is the statement's StmtBase itself.
func (b *StmtBase) Base() *StmtBase { return b }

// BaseType is one of FORTRAN's data types as a declaration names it
// (section 2.1).
type BaseType int

// The data types. DOUBLE PRECISION and DOUBLE COMPLEX are their own
// keywords, though they mean REAL*8 and COMPLEX*16; BYTE is LOGICAL*1.
const (
	Integer BaseType = iota + 1
	Real
	Logical
	Byte
	DoublePrecision
	Complex
	DoubleComplex
	Character
)

// String is the type's keyword.
func (t BaseType) String() string {
	return [...]string{"(none)", "INTEGER", "REAL", "LOGICAL", "BYTE", "DOUBLE PRECISION",
		"COMPLEX", "DOUBLE COMPLEX", "CHARACTER"}[t]
}

// TypeKeywords are the data types by the lexer's kind of their keyword.
var TypeKeywords = map[lex.Kind]BaseType{
	lex.Integer:         Integer,
	lex.Real:            Real,
	lex.Logical:         Logical,
	lex.Byte:            Byte,
	lex.DoublePrecision: DoublePrecision,
	lex.Complex:         Complex,
	lex.DoubleComplex:   DoubleComplex,
	lex.Character:       Character,
}

// TypeNames are the data types by their keywords as a name token spells
// them, without blanks: IMPLICIT DOUBLE PRECISION (D) has the name
// DOUBLEPRECISION.
var TypeNames = map[string]BaseType{
	"INTEGER": Integer, "REAL": Real, "LOGICAL": Logical, "BYTE": Byte,
	"DOUBLEPRECISION": DoublePrecision, "COMPLEX": Complex,
	"DOUBLECOMPLEX": DoubleComplex, "CHARACTER": Character,
}

// Len is a length specifier written after a * (section 4.4): a number of
// bytes, *8; for CHARACTER also a parenthesized integer constant
// expression, *(4+N), or *(*), a passed length. All zero when no length
// is written.
type Len struct {
	// Length is *n's n, or 0.
	Length int

	// LenExpr is the expression of *(e), or nil.
	LenExpr Expr

	// LenStar is *(*).
	LenStar bool
}

// HasLength reports whether a length was written.
func (l Len) HasLength() bool {
	return l.Length != 0 || l.LenExpr != nil || l.LenStar
}

// TypeSpec is a data type as a statement writes it: INTEGER, REAL*4,
// CHARACTER*(*).
type TypeSpec struct {
	Type BaseType

	// Len is the length written after the type keyword (section 4.4.1),
	// in bytes or (for CHARACTER) characters. Whether the length is one
	// the type allows is semantic analysis's question.
	Len

	Pos diag.Position
}

// Declarator is one entity a type, DIMENSION, COMMON, or RECORD
// statement declares: a name, the length written after it, its
// dimensions if it is an array (section 2.2.3.1), and the initial values
// a type statement may give it in slashes (section 4.4). IVEC*4(10) is
// Name IVEC, Length 4, one dimension with upper bound 10; in a CHARACTER
// statement the dimensions come first, NAMES(100)*32. In a structure's
// field declarations the name may be %FILL, an unnamed field.
type Declarator struct {
	Name string
	Len
	Dims []Bound
	Pos  diag.Position

	// Init are the initial values in slashes after the entity, as in a
	// DATA statement (sections 4.4.1, 4.3); nil if none. InitPos is where
	// the opening slash is.
	Init    []*DataValue
	InitPos diag.Position
}

// Bound is one dimension's bounds in an array declarator, [dl:]du.
type Bound struct {
	// Lower is the lower bound, or nil when it is left out (it is 1).
	Lower Expr

	// Upper is the upper bound; nil when Star is set.
	Upper Expr

	// Star is an upper bound of *: an assumed-size dummy array's last
	// dimension.
	Star bool
}

// LabelRef is a statement label a statement refers to: GO TO 10, DO 10,
// PRINT 100.
type LabelRef struct {
	Label int
	Pos   diag.Position
}

// FormatSpec is an I/O statement's format: * (list-directed), the label
// of a FORMAT statement, or an expression (a character expression
// holding the format, an array holding one, a variable a format's label
// was ASSIGNed to, or, for a name, perhaps a namelist group, which only
// the declarations can tell). Exactly one of its fields is set.
type FormatSpec struct {
	Star  bool
	Label *LabelRef
	Expr  Expr
	Pos   diag.Position
}

// UnitSpec is an I/O statement's unit: * (the default unit), or an
// expression giving the unit's number, or naming an internal file.
type UnitSpec struct {
	Star bool
	Expr Expr
	Pos  diag.Position
}

// Executable reports whether s is an executable statement (Figure 1-1),
// whose label a jump may go to: its lexer's kind says. A statement with
// an error counts as executable when the lexer couldn't say what it was,
// so that no further message is given about jumps to it.
func Executable(s Stmt) bool {
	k := s.Base().Kind

	return k == lex.Unknown || k.Executable()
}

// Op is an operator in an expression (section 2.3).
type Op int

// The operators. Add and Sub are also the unary signs, and Not is only
// unary.
const (
	Add Op = iota
	Sub
	Mul
	Div
	Pow
	Concat
	EQ
	NE
	LT
	LE
	GT
	GE
	Not
	And
	Or
	Eqv
	Neqv
	Xor
)

// String is the operator as FORTRAN writes it: +, **, .EQ., .AND.
func (o Op) String() string {
	return [...]string{"+", "-", "*", "/", "**", "//",
		".EQ.", ".NE.", ".LT.", ".LE.", ".GT.", ".GE.",
		".NOT.", ".AND.", ".OR.", ".EQV.", ".NEQV.", ".XOR."}[o]
}

// Relational reports whether o compares two values (section 2.3.3).
func (o Op) Relational() bool {
	return o >= EQ && o <= GE
}

// Expr is an expression. Every expression type embeds ExprBase.
type Expr interface {
	Node
	exprNode()
}

// ExprBase is what every expression has: where it is. For a Binary
// expression that is its operator, which is what a message about the
// operation (mixing types, say) should point at; for every other
// expression it is its first token.
type ExprBase struct {
	Pos diag.Position
}

// Position is where the expression is.
func (b *ExprBase) Position() diag.Position { return b.Pos }

func (*ExprBase) exprNode() {}

// The expressions.
type (
	// IntConst is an integer constant, its digits as written (section
	// 2.2.1.1). A sign before it is a Unary. Radix is 8 for appendix
	// A.5's octal form, "17, whose Text is the digits after the quote;
	// otherwise 0, decimal.
	IntConst struct {
		ExprBase
		Text  string
		Radix int
	}

	// RealConst is a real constant as written: 1.5, 2., 1E3, 2.5D0
	// (section 2.2.1.2).
	RealConst struct {
		ExprBase
		Text string
	}

	// LogicalConst is .TRUE. or .FALSE. (section 2.2.1.5).
	LogicalConst struct {
		ExprBase
		Value bool
	}

	// CharConst is a character constant; Value is its characters,
	// without the apostrophes and with each doubled apostrophe made one
	// (section 2.2.1.6).
	CharConst struct {
		ExprBase
		Value string
	}

	// HollerithConst is a Hollerith constant, nHtext (section 2.2.1.7):
	// Value is its n characters.
	HollerithConst struct {
		ExprBase
		Value string
	}

	// TypelessConst is an octal or hexadecimal constant, '17'O or '1F'X
	// (section 2.2.1.4): its Digits, and Radix 8 or 16.
	TypelessConst struct {
		ExprBase
		Digits string
		Radix  int
	}

	// ComplexConst is a complex constant, (Re, Im) (section 2.2.1.3).
	// Each part should be a constant (signed) or a constant's name
	// (section 4.11), which semantic analysis checks: the parser takes
	// any expressions, so that (X, Y) gets a message saying what is
	// wrong rather than a syntax error.
	ComplexConst struct {
		ExprBase
		Re, Im Expr
	}

	// Name is a symbolic name standing alone: a variable, a whole array,
	// a dummy argument, an external routine passed as an argument.
	Name struct {
		ExprBase
		Name string
	}

	// Apply is a name followed by a parenthesized list, A(I, J): an
	// array element or a function reference, which only the
	// declarations can tell apart.
	Apply struct {
		ExprBase
		Name string
		Args []Expr
	}

	// Unary is a sign or .NOT. before an operand.
	Unary struct {
		ExprBase
		Op Op
		X  Expr
	}

	// Binary is an operation on two operands.
	Binary struct {
		ExprBase
		Op   Op
		X, Y Expr
	}

	// Paren is a parenthesized expression.
	Paren struct {
		ExprBase
		X Expr
	}

	// Substring is a character substring, X(Lower:Upper) (section
	// 2.2.4): X is a *Name (a character variable), an *Apply (an element
	// of a character array, NAMES(1,3)(:7)), or a *FieldRef. A bound left
	// out is nil (1, or the length).
	Substring struct {
		ExprBase
		X            Expr
		Lower, Upper Expr
	}

	// FieldRef is a reference to a record's field, X.Field (section
	// 2.2.5.2): X is the record (a *Name, an *Apply for an element of an
	// array of records, or another FieldRef).
	FieldRef struct {
		ExprBase
		X     Expr
		Field string
		Args  []Expr // the field's subscripts, if it is an array; else nil
	}

	// BuiltinArg is one of the built-in functions of section 6.1.2:
	// %LOC(X), the address of X, and %VAL(X), %REF(X), %DESCR(X), which
	// say how an actual argument is passed. Func is LOC, VAL, REF, or
	// DESCR.
	BuiltinArg struct {
		ExprBase
		Func string
		X    Expr
	}

	// AltReturn is an alternate return specifier among a CALL's actual
	// arguments, *s or &s (section 6.1.1.5).
	AltReturn struct {
		ExprBase
		Target LabelRef
	}

	// ImpliedDo is an implied-DO list, (Items, Var = Start, Limit[,
	// Step]), in an I/O list (section 7.1.2.2) or a DATA statement
	// (section 4.3). Step is nil when it is left out.
	ImpliedDo struct {
		ExprBase
		Items              []Expr
		Var                *Name
		Start, Limit, Step Expr
	}

	// KeyList is an OPEN statement's KEY=(kspec[,kspec]...) (section
	// 9.1.15), each kspec e1:e2[:dt[:dr]].
	KeyList struct {
		ExprBase
		Keys []*KeyField
	}
)

// KeyField is one key of an OPEN statement's KEY=: its first and last
// byte positions, and its type (INTEGER or CHARACTER) and direction
// (ASCENDING or DESCENDING), "" when left out.
type KeyField struct {
	Start, End Expr
	Type, Dir  string
	Pos        diag.Position
}
