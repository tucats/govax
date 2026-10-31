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

	// Kind is a main program, a FUNCTION, a SUBROUTINE, or BLOCK DATA.
	Kind ast.UnitKind

	// Name is the unit's name: its header's, or, for a main program
	// without a PROGRAM statement, the source file's name cut to 26
	// characters with $MAIN after it (section 4.12), or for a BLOCK DATA
	// statement without a name, with $DATA after it.
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

	// Assigned are the labels the unit's ASSIGN statements give, each
	// once, in the order they first appear: where an assigned GO TO
	// without a list may go.
	Assigned []int

	// LocalSize is the size of the unit's static storage, its
	// contribution to $LOCAL, in bytes.
	LocalSize int

	// Commons are the common blocks the unit declares, in order of first
	// appearance.
	Commons []*CommonBlock

	// Initials are the initial values DATA statements and type statements
	// give the unit's names, in order of place (data.go).
	Initials []*Initial

	// GFloating is set when the unit's REAL*8 and COMPLEX*16 are in
	// G_floating (/G_FLOATING, or the unit's OPTIONS statement), clear
	// when they are in D_floating.
	GFloating bool

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

	// StmtFunction is a statement function the unit defines (section
	// 6.2.1).
	StmtFunction

	// Constant is a named constant, a PARAMETER statement's (section
	// 4.11, appendix A.4): writing its name is writing its value.
	Constant

	// External is a name an EXTERNAL statement says is a routine outside
	// the unit, not yet used as a function or called (section 4.7). A
	// reference makes it a Function or a Subroutine; its External field
	// stays set.
	External

	// Intrinsic is a name an INTRINSIC statement says is FORTRAN's own
	// function (section 4.9).
	Intrinsic
)

// String is the class as messages say it: "a variable".
func (c Class) String() string {
	return [...]string{"a name", "a variable", "an array", "a function", "a subroutine",
		"the name of this program unit", "a statement function", "a constant",
		"an external procedure", "an intrinsic function"}[c]
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

	// untyped is set for a name entered under IMPLICIT NONE, whose
	// first letter gives it no type: unless a declaration gives it one,
	// it is NOTYPE.
	untyped bool

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

	// Offset is the name's place, in bytes, in its common block if it is
	// in one, or else in the unit's $LOCAL storage; or -1 if it has none
	// (a dummy argument, a routine).
	Offset int

	// Common is the common block the name is in, by a COMMON statement or
	// by EQUIVALENCE with a name in one; nil if none (common.go).
	Common *CommonBlock

	// External is set for a name in an EXTERNAL statement, a dummy
	// argument's among them.
	External bool

	// Equivalenced is set for a name in an EQUIVALENCE list, whose storage
	// may overlap other names'. group is the names it is tied to, and
	// groupOffset its place among them; equivPos is its first appearance
	// in a list (equivalence.go).
	Equivalenced bool
	group        *equivGroup
	groupOffset  int
	equivPos     diag.Position

	// Pos is where the name first appears.
	Pos diag.Position

	// Value is a constant's value, with its type; nil for a constant
	// whose PARAMETER statement had an error.
	Value *Const

	// defPos is where a statement function or a constant is defined;
	// oldForm is set for a constant appendix A.4's PARAMETER defined,
	// whose type is its value's.
	defPos  diag.Position
	oldForm bool

	// broken is set when a declaration of the name had an error, which
	// has been reported: its uses get no messages of their own, which
	// would only repeat the mistake.
	broken bool
}

// Dim is one dimension of an array: its bounds. Assumed is an
// assumed-size dummy array's last dimension, written *, whose upper bound
// is unknown. LowerExpr and UpperExpr are an adjustable array's bounds
// that aren't constant, which are computed when the subprogram is entered
// (section 6.1.1.1, Phase 79's code); nil for a constant bound, which is
// Lower or Upper.
type Dim struct {
	Lower, Upper         int
	Assumed              bool
	LowerExpr, UpperExpr ast.Expr
}

// known reports whether the dimension's bounds are both known now.
func (d Dim) known() bool {
	return !d.Assumed && d.LowerExpr == nil && d.UpperExpr == nil
}

// Extent is how many elements the dimension has, du - dl + 1 (section
// 2.2.3.1), if both its bounds are known.
func (d Dim) Extent() int {
	return d.Upper - d.Lower + 1
}

// AssumedSize reports whether s is an assumed-size array, whose last
// upper bound is * (section 6.1.1.2).
func (s *Symbol) AssumedSize() bool {
	return len(s.Dims) > 0 && s.Dims[len(s.Dims)-1].Assumed
}

// Elements is how many elements the array has (1 for a scalar), or 0 for
// an assumed-size or adjustable array, whose size isn't known until the
// program runs.
func (s *Symbol) Elements() int {
	n := 1

	for _, d := range s.Dims {
		if !d.known() {
			return 0
		}

		n *= d.Extent()
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

	// ComputedGoto is GO TO (Labels) Index (section 5.6.2): to the
	// Index-th label, Index an INTEGER*4; past the statement when Index
	// is below 1 or past the last.
	ComputedGoto struct {
		stmtBase
		Labels []int
		Index  Expr
	}

	// AssignLabel is ASSIGN Label TO Var (section 3.5): Var, an INTEGER*4
	// variable, gets the address of the statement labeled Label.
	AssignLabel struct {
		stmtBase
		Label int
		Var   *Symbol
	}

	// AssignedGoto is GO TO Var [(Labels)] (section 5.6.3): to the
	// statement whose address the last ASSIGN put in Var. Labels, the
	// list, says only where that may be (nil without one: any label in
	// the unit's Assigned).
	AssignedGoto struct {
		stmtBase
		Var    *Symbol
		Labels []int
	}

	// ArithIf is the arithmetic IF, IF (Cond) Labels (section 5.7.1): to
	// the first label if Cond is negative, the second if it is zero, the
	// third if it is positive. Cond is an INTEGER or REAL value.
	ArithIf struct {
		stmtBase
		Cond   Expr
		Labels [3]int
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
	// A loop without a label (Label 0) ends at EndDo; so may a labeled
	// one, an END DO with its label (section 5.4).
	Do struct {
		stmtBase
		Label              int
		Var                *Symbol
		Start, Limit, Step Expr
		Body               []Stmt

		// EndDo is the END DO statement that ends the loop, or nil.
		EndDo ast.Stmt
	}

	// DoWhile is DO Label WHILE (Cond) (section 5.3.2): Body runs while
	// the LOGICAL Cond, tested before each pass, is true. Its body and
	// end are a Do's.
	DoWhile struct {
		stmtBase
		Label int
		Cond  Expr
		Body  []Stmt
		EndDo ast.Stmt
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
	// folded. Its value is in the fields its type uses: Int for INTEGER
	// and LOGICAL (a LOGICAL's value: .TRUE. is -1 and .FALSE. 0, though
	// only bit 0 counts, appendix C.4), Real for REAL (already rounded to
	// the type's format: F_floating, D_floating, or H_floating), Real and
	// Imag for COMPLEX's two parts, Text for CHARACTER.
	Const struct {
		exprBase
		Int  int32
		Real vaxfloat.Value
		Imag vaxfloat.Value
		Text string

		// literal is set for a REAL or COMPLEX constant as written (or
		// negated), and exactRe and exactIm are its parts' values before
		// rounding. In an operation of higher precision it is rounded
		// again from them: section 2.3.1.2 says that "additional
		// precision will be retained for the constant", so 1.0D0 +
		// 0.3333333 is 1.0D0 + 0.3333333000000000D0, not 1.0D0 plus
		// 0.3333333's F_floating value.
		literal          bool
		exactRe, exactIm vaxfloat.Value
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

	// IntrinsicRef is a reference to an intrinsic function compiled in line
	// or called (section 6.3; intrinsics.go): Name is the specific function
	// selected (JMAX0, IIBITS), or the generic one where Table D-3 gives
	// none; Op is what it computes; and Args are its arguments, each
	// converted to the type the function takes (for CMPLX's two, to the
	// result's parts' type). Its type is the type it computes in, which a
	// Convert around it makes the result's where footnote 10 says they
	// differ (MAX1 is the INTEGER of a REAL maximum). Routine is the
	// routine a CallOp calls (MTH$SQRT, FTN$JISHFTC; routines.go).
	IntrinsicRef struct {
		exprBase
		Name    string
		Op      IntrinsicOp
		Args    []Expr
		Routine string
	}

	// Procedure is an intrinsic function's name given as an actual
	// argument (sections 4.9 and 6.3.1.1): the routine passed, whose
	// address the argument list holds, so that the routine given it can
	// call it. Name is the name the program wrote, and Routine the routine
	// (MTH$SIN, FTN$LEN; routines.go). Its type is the function's value's.
	Procedure struct {
		exprBase
		Name    string
		Routine string
	}

	// Substring is X(Lower:Upper), part of a CHARACTER variable or array
	// element (section 2.2.4): X is a *Var or an *Elem, and Lower and Upper
	// INTEGER positions in it, counted from 1; Upper is nil for X's last
	// character, when the substring gives none. Its type is CHARACTER of its
	// length, or of the size 0 when a bound isn't constant and the length is
	// known only when the program runs.
	Substring struct {
		exprBase
		X            Expr
		Lower, Upper Expr
	}

	// Concat is Parts[0] // Parts[1] // ... (section 2.3.2): CHARACTER
	// values joined left to right, a nested concatenation's parts among
	// them, since parentheses change nothing. No part is a constant next to
	// another constant: those are joined already. Its type is CHARACTER of
	// the parts' lengths' sum, or of the size 0 when a part's length is
	// known only when the program runs.
	Concat struct {
		exprBase
		Parts []Expr
	}

	// Unary is -X (Op ast.Sub) or .NOT. X (Op ast.Not). A unary plus is
	// dropped.
	Unary struct {
		exprBase
		Op ast.Op
		X  Expr
	}

	// Binary is X Op Y for the arithmetic operators and .AND. and .OR.;
	// X and Y have the result's type, but for **, whose operands have the
	// types its OTS$ routine takes (powerTypes).
	Binary struct {
		exprBase
		Op   ast.Op
		X, Y Expr
	}

	// Compare is X Op Y for a relational operator: LOGICAL, with X and Y
	// of one type, the operands' (which the dump shows); or two CHARACTER
	// values, of any lengths.
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

// IntrinsicOp is what an IntrinsicRef computes. Those the language's operators
// can say (IAND is .AND. on INTEGERs, DPROD a product, LLT a comparison)
// and the conversions are those expressions instead.
type IntrinsicOp int

// The operations. Each says what its arguments are, by position.
const (
	AbsOp      IntrinsicOp = iota + 1 // |X|, of an INTEGER or REAL X
	MaxOp                             // the largest of the arguments
	MinOp                             // the smallest
	DimOp                             // X-Y if X is greater, else 0
	SignOp                            // |X| with Y's sign (+ for a Y of 0)
	ModOp                             // X - (X/Y)*Y, of INTEGERs
	NintOp                            // a REAL rounded to the nearest INTEGER, halves away from 0
	ZextOp                            // an INTEGER or LOGICAL made longer with zeros, not its sign
	ImagOp                            // a COMPLEX value's imaginary part
	CmplxOp                           // the COMPLEX value (X, Y)
	ConjgOp                           // a COMPLEX value with its imaginary part negated
	ShiftOp                           // ISHFT(I, N): I's bits shifted left N places, right -N; N constant
	BitsOp                            // IBITS(I, POS, LEN): LEN bits of I from bit POS
	BitSetOp                          // IBSET(I, POS): I with bit POS set
	BitClearOp                        // IBCLR(I, POS): I with bit POS clear
	BitTestOp                         // BTEST(I, POS): bit POS of I, a LOGICAL
	LenOp                             // a passed-length CHARACTER value's length
	IndexOp                           // INDEX(C1, C2): where C2 first is in C1, or 0
	IcharOp                           // a CHARACTER value's first character's code
	CharOp                            // the CHARACTER*1 whose code is X
	CallOp                            // a call of Routine, MTHRTL's or FTNRTL's, with the arguments by reference
)

// String is the operation's name in the dump.
func (op IntrinsicOp) String() string {
	return [...]string{"?", "abs", "max", "min", "dim", "sign", "mod", "nint", "zext", "imag", "cmplx",
		"conjg", "shift", "bits", "bitset", "bitclear", "bittest", "len", "index",
		"ichar", "char", "call"}[op]
}

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

// Bits is a REAL*4 constant's longword as the VAX holds it in memory,
// read as a little-endian longword: 1.0 is ^X00004080.
func (c *Const) Bits() uint32 {
	b, _ := vaxfloat.Pack(vaxfloat.F, c.Real)

	return uint32(b.Lo)
}

// at is a copy of c at pos: a constant's value where its name is used.
func (c *Const) at(pos diag.Position) *Const {
	d := *c
	d.Pos = pos

	return &d
}
