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
	// Header is the unit's first statement: a *ProgramStmt, a
	// *FunctionStmt, a *SubroutineStmt, a *NotImplStmt for BLOCK DATA,
	// or nil for a main program without a PROGRAM statement.
	Header Stmt

	// Body is the statements after the header, in order. A unit the
	// parser finished properly ends with an *EndStmt.
	Body []Stmt
}

// Kind is what kind of unit this is, from its header.
func (u *Unit) Kind() UnitKind {
	switch h := u.Header.(type) {
	case *FunctionStmt:
		return FunctionUnit
	case *SubroutineStmt:
		return SubroutineUnit
	case *NotImplStmt:
		return kindOf(h.Kind)
	case *BadStmt:
		// A header with an error still says what kind of unit it starts.
		return kindOf(h.Kind)
	}

	return MainProgram
}

// kindOf is the kind of unit a header statement of kind k starts.
func kindOf(k lex.Kind) UnitKind {
	switch k {
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
// source file, section 4.12).
func (u *Unit) Name() string {
	switch h := u.Header.(type) {
	case *ProgramStmt:
		return h.Name
	case *FunctionStmt:
		return h.Name
	case *SubroutineStmt:
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

	// Base is the statement's label and position.
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

	// Source is the statement the source pass read, which says which of
	// the source's lines it came from (its First and Last lines) and
	// where its label is (LabelPosition).
	Source *source.Statement
}

// Position is where the statement starts.
func (b *StmtBase) Position() diag.Position { return b.Pos }

// Base is the statement's StmtBase itself.
func (b *StmtBase) Base() *StmtBase { return b }

// BaseType is one of FORTRAN's data types as a declaration names it.
type BaseType int

// The data types the parser accepts so far. The others (BYTE, DOUBLE
// PRECISION, COMPLEX, DOUBLE COMPLEX, CHARACTER) come with later phases;
// until then their statements are NOTIMPL.
const (
	Integer BaseType = iota + 1
	Real
	Logical
)

// String is the type's keyword.
func (t BaseType) String() string {
	return [...]string{"(none)", "INTEGER", "REAL", "LOGICAL"}[t]
}

// TypeSpec is a data type as a statement writes it: INTEGER, REAL*4.
type TypeSpec struct {
	Type BaseType

	// Length is the *n written after the type keyword, in bytes, or 0
	// if none was (section 4.4.1). Whether the length is one the type
	// allows is semantic analysis's question.
	Length int

	Pos diag.Position
}

// Declarator is one entity a type or DIMENSION statement declares: a
// name, the length written after it, and its dimensions if it is an
// array (section 2.2.3.1): IVEC*4(10) is Name IVEC, Length 4, one
// dimension with upper bound 10.
type Declarator struct {
	Name   string
	Length int // the *n after the name, or 0
	Dims   []Bound
	Pos    diag.Position
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
// holding the format, or a variable a format's label was ASSIGNed to).
// Exactly one of its fields is set.
type FormatSpec struct {
	Star  bool
	Label *LabelRef
	Expr  Expr
	Pos   diag.Position
}

// UnitSpec is an I/O statement's unit: * (the default unit), or an
// expression giving the unit's number.
type UnitSpec struct {
	Star bool
	Expr Expr
	Pos  diag.Position
}

// The statements.
type (
	// ProgramStmt is PROGRAM name (section 4.12).
	ProgramStmt struct {
		StmtBase
		Name string
	}

	// FunctionStmt is [type] FUNCTION name [([p[,p]...])] (section
	// 6.2.2). Type is nil when the statement names no type, and the
	// function's type comes from the declarations or the name's first
	// letter.
	FunctionStmt struct {
		StmtBase
		Type   *TypeSpec
		Name   string
		Params []*Name
	}

	// SubroutineStmt is SUBROUTINE name [([p[,p]...])] (section 6.2.3).
	SubroutineStmt struct {
		StmtBase
		Name   string
		Params []*Name
	}

	// TypeStmt is a type declaration: INTEGER I, A(10) (section 4.4.1).
	TypeStmt struct {
		StmtBase
		Type     TypeSpec
		Entities []*Declarator
	}

	// DimensionStmt is DIMENSION a(d)[,a(d)]... (section 4.5).
	DimensionStmt struct {
		StmtBase
		Entities []*Declarator
	}

	// AssignStmt is an assignment, Target = Value (chapter 3). Target is
	// a *Name or an *Apply: an array element, or, if the name isn't an
	// array, a statement function's definition.
	AssignStmt struct {
		StmtBase
		Target Expr
		Value  Expr
	}

	// CallStmt is CALL name [([a][,[a]]...)] (section 5.1). A nil
	// argument is a null one: CALL S(A,,B).
	CallStmt struct {
		StmtBase
		Name    string
		NamePos diag.Position
		Args    []Expr
	}

	// GotoStmt is the unconditional GO TO (section 5.6.1).
	GotoStmt struct {
		StmtBase
		Target LabelRef
	}

	// IfStmt is the logical IF, IF (Cond) Then (section 5.7.2). Then is
	// the statement executed when Cond is true; it has no label of its
	// own.
	IfStmt struct {
		StmtBase
		Cond Expr
		Then Stmt
	}

	// IfThenStmt is IF (Cond) THEN, which starts an IF block (section
	// 5.7.3).
	IfThenStmt struct {
		StmtBase
		Cond Expr
	}

	// ElseIfStmt is ELSE IF (Cond) THEN.
	ElseIfStmt struct {
		StmtBase
		Cond Expr
	}

	// ElseStmt is ELSE.
	ElseStmt struct {
		StmtBase
	}

	// EndIfStmt is END IF.
	EndIfStmt struct {
		StmtBase
	}

	// DoStmt is DO s [,] v = e1, e2 [, e3] (section 5.3.1). Step is nil
	// when e3 is left out (it is 1).
	DoStmt struct {
		StmtBase
		Target LabelRef
		Var    *Name
		Start  Expr
		Limit  Expr
		Step   Expr
	}

	// ContinueStmt is CONTINUE (section 5.2).
	ContinueStmt struct {
		StmtBase
	}

	// StopStmt is STOP [disp] (section 5.10). Display is nil, a
	// *CharConst, or an *IntConst of 1 to 5 digits.
	StopStmt struct {
		StmtBase
		Display Expr
	}

	// ReturnStmt is RETURN [i] (section 5.9). Alt is the alternate
	// return's number, or nil.
	ReturnStmt struct {
		StmtBase
		Alt Expr
	}

	// EndStmt is END (section 5.5), every unit's last statement.
	EndStmt struct {
		StmtBase
	}

	// PrintStmt is PRINT f [, iolist] (section 7.6).
	PrintStmt struct {
		StmtBase
		Format FormatSpec
		Items  []Expr
	}

	// WriteStmt is WRITE (control list) [iolist] (section 7.6). Format
	// is nil for an unformatted WRITE.
	WriteStmt struct {
		StmtBase
		Unit   UnitSpec
		Format *FormatSpec
		Items  []Expr
	}

	// NotImplStmt is a statement the compiler recognizes but can't
	// compile yet, which the parser has reported (NOTIMPL). What is the
	// message's subject ("the READ statement"), and Kind the lexer's
	// kind of the statement.
	NotImplStmt struct {
		StmtBase
		What string
		Kind lex.Kind
	}

	// BadStmt is a statement with an error the parser or the lexer
	// reported. Kind is the lexer's kind of the statement.
	BadStmt struct {
		StmtBase
		Kind lex.Kind
	}
)

// Executable reports whether s is an executable statement (Figure 1-1),
// whose label a jump may go to. A statement with an error counts as
// executable when the lexer couldn't say what it was, so that no further
// message is given about jumps to it.
func Executable(s Stmt) bool {
	switch s := s.(type) {
	case *ProgramStmt, *FunctionStmt, *SubroutineStmt, *TypeStmt, *DimensionStmt:
		return false
	case *NotImplStmt:
		return s.Kind.Executable()
	case *BadStmt:
		return s.Kind == lex.Unknown || s.Kind.Executable()
	}

	return true
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
	// 2.2.1.1). A sign before it is a Unary.
	IntConst struct {
		ExprBase
		Text string
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
)
