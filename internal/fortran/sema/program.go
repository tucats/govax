package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Program is a source file's program units after semantic analysis.
type Program struct {
	Units []*Unit
}

// Unit is one program unit: its symbol table, its storage, and its
// statements as a structured tree.
type Unit struct {
	// AST is the unit the parser built.
	AST *ast.Unit

	// Kind is a main program, a FUNCTION, or a SUBROUTINE.
	Kind ast.UnitKind

	// Name is the unit's name: its header's, or, for a main program
	// without a PROGRAM statement, the source file's name cut to 26
	// characters with $MAIN after it (section 4.12).
	Name string

	// Symbols are every name the unit uses, in order of first
	// appearance, which is the order their storage is laid out in.
	Symbols []*Symbol

	// Params are the dummy arguments, in order.
	Params []*Symbol

	// Result is a function's own name, the variable whose last value is
	// the function's value; nil in a main program or subroutine.
	Result *Symbol

	// Body is the unit's executable statements, END last. IF blocks and
	// DO loops hold the statements inside them.
	Body []Stmt

	// Labels are the statement labels defined in the unit.
	Labels map[int]*Label

	// LocalSize is the size of the unit's static storage, its
	// contribution to $LOCAL, in bytes.
	LocalSize int

	byName map[string]*Symbol

	// calls are the unit's references to routines, which Analyze checks
	// against the file's other units once every unit is analyzed.
	calls []routineRef

	// incomplete is set when the unit has a declaration this phase can't
	// read (incompleteDeclarations).
	incomplete bool
}

// Lookup returns the symbol named name, or nil if the unit never uses
// that name.
func (u *Unit) Lookup(name string) *Symbol {
	return u.byName[name]
}

// Class is what a name names, decided by how the unit declares and uses
// it.
type Class int

// The classes of name.
const (
	// Unclassified is a name only declared (given a type) or a dummy
	// argument, not used yet. At the end of the unit such a name becomes
	// a variable.
	Unclassified Class = iota

	// Variable is a scalar variable (a dummy argument, or a function's
	// result, among them).
	Variable

	// Array is an array: a name with dimensions.
	Array

	// Function is an external function the unit references.
	Function

	// Subroutine is an external subroutine the unit calls.
	Subroutine

	// UnitName is the main program's name, or a subroutine's own name
	// within it: names that may not stand for anything else.
	UnitName
)

// String is the class as messages say it: "a variable".
func (c Class) String() string {
	return [...]string{"a name", "a variable", "an array", "a function", "a subroutine", "the name of this program unit"}[c]
}

// Symbol is one name in a unit.
type Symbol struct {
	Name  string
	Class Class
	Type  Type

	// Typed is set when a type statement, or the FUNCTION statement,
	// gave the name its type; TypePos is where.
	Typed   bool
	TypePos diag.Position

	// Dims are an array's dimensions; DimPos is where they were
	// declared.
	Dims   []Dim
	DimPos diag.Position

	// Dummy is the name's position among the dummy arguments, 1 for the
	// first, or 0 if it isn't one. A dummy argument has no storage of
	// its own: its address is the argument list's longword Dummy, at
	// 4*Dummy(AP).
	Dummy int

	// Result is set for a function's own name within the function.
	Result bool

	// Offset is the name's place in the unit's $LOCAL storage, in
	// bytes, or -1 if it has none (a dummy argument, a routine).
	Offset int

	// Pos is where the name first appears.
	Pos diag.Position

	// broken is set when a declaration of the name had an error, which
	// has been reported: its uses get no messages of their own, which
	// would only repeat the mistake.
	broken bool
}

// Dim is one dimension of an array: its bounds. Assumed is an
// assumed-size dummy array's last dimension, written *, whose upper bound
// is unknown.
type Dim struct {
	Lower, Upper int
	Assumed      bool
}

// Elements is how many elements the array has (1 for a scalar), or 0 for
// an assumed-size array, whose size isn't known.
func (s *Symbol) Elements() int {
	n := 1

	for _, d := range s.Dims {
		if d.Assumed {
			return 0
		}

		n *= d.Upper - d.Lower + 1
	}

	return n
}

// Size is the storage the name takes, in bytes.
func (s *Symbol) Size() int {
	return s.Type.Size * s.Elements()
}

// Label is a statement label defined in a unit.
type Label struct {
	Number int

	// Stmt is the statement the label is on.
	Stmt ast.Stmt

	// path is the IF blocks and DO loops the statement is in, which
	// decides which jumps may go to it.
	path []frame

	// loops are the DO loops that end at this statement, innermost
	// first.
	loops []int
}

// Stmt is an executable statement in the structured tree.
type Stmt interface {
	// Source is the statement the parser built, which has its label,
	// its position, and the source lines it came from.
	Source() ast.Stmt
}

// stmtBase is what every statement has: the syntax it came from.
type stmtBase struct {
	AST ast.Stmt
}

// Source is the statement's syntax tree.
func (b *stmtBase) Source() ast.Stmt { return b.AST }

// The statements.
type (
	// Assign is Target = Value. Target is a *Var or an *Elem, and Value
	// has Target's type: semantic analysis converted it (Table 3-1).
	Assign struct {
		stmtBase
		Target Expr
		Value  Expr
	}

	// Call is CALL Sub(Args).
	Call struct {
		stmtBase
		Sub  *Symbol
		Args []Arg
	}

	// Goto is GO TO Label.
	Goto struct {
		stmtBase
		Label int
	}

	// If is a block IF construct (Block set: IF THEN, its ELSE IFs, its
	// ELSE, END IF), or a logical IF, which is one arm holding one
	// statement. Each arm's condition is LOGICAL.
	If struct {
		stmtBase
		Block bool
		Arms  []*Arm

		// Else is the ELSE block, or nil if there is none.
		Else *Arm

		// EndIf is the END IF statement (whose label a jump from the
		// last block may go to); nil for a logical IF.
		EndIf ast.Stmt
	}

	// Do is a DO loop: DO Label Var = Start, Limit, Step, with Body the
	// statements up to and including the one labeled Label (or a
	// nested DO loop ending there too). Start, Limit, and Step have
	// Var's type; Step is the constant 1 when the statement has none.
	Do struct {
		stmtBase
		Label              int
		Var                *Symbol
		Start, Limit, Step Expr
		Body               []Stmt
	}

	// Continue is CONTINUE.
	Continue struct {
		stmtBase
	}

	// Stop is STOP, with the text it displays if it has one.
	Stop struct {
		stmtBase
		Display    string
		HasDisplay bool
	}

	// Return is RETURN in a subprogram.
	Return struct {
		stmtBase
	}

	// End is END, the unit's last statement.
	End struct {
		stmtBase
	}

	// Print is a list-directed output statement: PRINT *, or (with
	// Write set) WRITE (*,*). Items are the values to write; a whole
	// array is an *ArrayRef, and a character constant a CHARACTER
	// *Const.
	Print struct {
		stmtBase
		Write bool
		Items []Expr
	}
)

// Arm is one block of an IF construct: the statement that starts it (IF
// THEN, ELSE IF, ELSE, or the logical IF), its condition (nil for ELSE),
// and its statements.
type Arm struct {
	AST  ast.Stmt
	Cond Expr
	Body []Stmt
}

// Expr is an expression with its type.
type Expr interface {
	Type() Type
	Position() diag.Position
	exprNode()
}

// exprBase is what every expression has: its type and position.
type exprBase struct {
	T   Type
	Pos diag.Position
}

// Type is the expression's type.
func (b *exprBase) Type() Type { return b.T }

// Position is where the expression is in the source.
func (b *exprBase) Position() diag.Position { return b.Pos }

func (*exprBase) exprNode() {}

// The expressions.
type (
	// Const is a constant, or a constant expression semantic analysis
	// folded. Its value is in the field its type uses: Int for INTEGER
	// and LOGICAL (a LOGICAL's longword: .TRUE. is -1 and .FALSE. 0,
	// though only bit 0 counts, appendix C.4), Real for REAL (already
	// rounded to F_floating), Text for CHARACTER.
	Const struct {
		exprBase
		Int  int32
		Real vaxfloat.Value
		Text string
	}

	// Var is a scalar variable.
	Var struct {
		exprBase
		Sym *Symbol
	}

	// Elem is an array element: the array and a subscript per dimension,
	// each INTEGER.
	Elem struct {
		exprBase
		Sym  *Symbol
		Subs []Expr
	}

	// ArrayRef is a whole array, which may be only an actual argument or
	// an I/O list item (section 2.2.3.5). Its type is its elements'.
	ArrayRef struct {
		exprBase
		Sym *Symbol
	}

	// FuncRef is a reference to an external function.
	FuncRef struct {
		exprBase
		Fn   *Symbol
		Args []Arg
	}

	// Unary is -X (Op ast.Sub) or .NOT. X (Op ast.Not). A unary plus is
	// dropped.
	Unary struct {
		exprBase
		Op ast.Op
		X  Expr
	}

	// Binary is X Op Y for the arithmetic operators and .AND. and .OR.;
	// X and Y have the result's type.
	Binary struct {
		exprBase
		Op   ast.Op
		X, Y Expr
	}

	// Compare is X Op Y for a relational operator: LOGICAL, with X and Y
	// of one type, the operands' (which the dump shows).
	Compare struct {
		exprBase
		Op   ast.Op
		X, Y Expr
	}

	// Convert is X converted to the expression's type (Table 3-1): REAL
	// to INTEGER truncates, INTEGER to REAL rounds to F_floating's 24
	// bits, and INTEGER and LOGICAL are the same longword.
	Convert struct {
		exprBase
		X Expr
	}

	// bad is an expression with an error, already reported.
	bad struct {
		exprBase
	}
)

// Arg is an actual argument. Expr is nil for a null argument (CALL
// S(A,,B)). Copy is set for a parenthesized variable or array element,
// (X), which is passed as a copy of its value, so the subprogram can't
// change X itself (section 5.1). Any other expression that isn't a
// variable, an array element, or a whole array is passed as a copy too.
type Arg struct {
	Expr Expr
	Copy bool
}

// Truth is a LOGICAL constant's truth: its bit 0 (appendix C.4).
func (c *Const) Truth() bool {
	return c.Int&1 != 0
}

// Bits is a REAL constant's longword as the VAX holds it in memory, read
// as a little-endian longword: 1.0 is ^X00004080.
func (c *Const) Bits() uint32 {
	b, _ := vaxfloat.Pack(vaxfloat.F, c.Real)

	return uint32(b.Lo)
}
