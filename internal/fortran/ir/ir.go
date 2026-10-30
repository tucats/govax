package ir

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Kind is what kind of routine a program unit becomes.
type Kind int

// The kinds of routine.
const (
	Main Kind = iota
	Subroutine
	Function
	BlockData
)

// String names the kind, for the dump.
func (k Kind) String() string {
	return [...]string{"main", "subroutine", "function", "block data"}[k]
}

// Type is a value's type as the machine sees it: what size it is and
// which instructions work on it. FORTRAN's INTEGER*4 and LOGICAL*4 are
// both a longword, so both are Long here; the difference that matters
// (how a LOGICAL is tested, what a descriptor says it is) has been
// settled by the time a value is lowered.
type Type int

// The types. None is a subroutine's (absent) result.
const (
	None  Type = iota
	Long       // a longword: INTEGER*4, LOGICAL*4; the L of ADDL3
	Float      // F_floating: REAL*4; the F of ADDF3
)

// String is the type's letter, the suffix of the VAX instructions that
// work on it: L, F.
func (t Type) String() string {
	return [...]string{"", "L", "F"}[t]
}

// Size is the type's size in bytes.
func (t Type) Size() int {
	return [...]int{0, 4, 4}[t]
}

// Routine is one program unit, lowered: its storage, its read-only data,
// and its code as a list of basic blocks.
type Routine struct {
	// Name is the routine's entry point: SQUARES, ISQ, SQUARES$MAIN.
	Name string

	Kind Kind

	// Result is a function's type, whose value Return leaves in R0.
	Result Type

	// Params are the dummy arguments, in order.
	Params []*Param

	// Cells are the routine's static storage, its contribution to the
	// $LOCAL program section, in order: the program's variables and
	// arrays where semantic analysis put them, then the compiler's own
	// cells (DO loop counts and steps, temporaries, descriptors filled
	// at run time).
	Cells []*Cell

	// LocalSize is the size of the routine's $LOCAL storage, in bytes.
	LocalSize int

	// Data are the routine's read-only data, its contribution to the
	// $PDATA program section: constants passed by reference, texts,
	// descriptors, and I/O control blocks.
	Data []*Datum

	// Blocks are the routine's code, in the order the code generator
	// lays them out; the first is the entry. A block data subprogram has
	// none: it has only storage and initial values.
	Blocks []*Block

	// Initials are the initial values of the routine's variables, in
	// order of place: bytes in the image, which no code stores.
	Initials []*sema.Initial

	// Externals are the names an EXTERNAL statement says are routines
	// outside the unit (a dummy argument's aside), which the object module
	// refers to whether or not any code calls them.
	Externals []string

	// Unit is the program unit the routine came from.
	Unit *sema.Unit
}

// Param is a dummy argument. Its value isn't the routine's: the
// argument list's longword Index (at 4*Index(AP)) holds the address of
// the caller's variable, array, or copy.
type Param struct {
	Name  string
	Index int
	T     Type
	Sym   *sema.Symbol
}

// CellKind is what a cell holds.
type CellKind int

// The kinds of cell.
const (
	VariableCell   CellKind = iota // a scalar variable
	ArrayCell                      // an array
	ResultCell                     // a function's value: the function's own name
	CountCell                      // a DO loop's iteration count
	StepCell                       // a DO loop's step, when it isn't a constant
	TempCell                       // a temporary, for one statement's use
	DescriptorCell                 // a descriptor whose addresses are set at run time
)

// Cell is a place in the routine's static storage.
type Cell struct {
	// Name is the cell's name in the routine: a variable's FORTRAN
	// name, or the compiler's own ($C1, $T2, $D3). The code generator
	// makes it a MACRO symbol by putting the routine's name and a
	// period before it.
	Name string

	Kind CellKind

	// T is the type of the cell's value, or of each element of an
	// array; Long for a descriptor.
	T Type

	// Size is the cell's size in bytes, and Offset its place in $LOCAL,
	// or in its common block.
	Size, Offset int

	// Common is the common block the cell is in; nil for a cell in $LOCAL.
	Common *sema.CommonBlock

	// Sym is the variable, array, or function result the cell holds;
	// nil for the compiler's cells.
	Sym *sema.Symbol

	// Loop is a count's or step's DO loop: the label of its terminal
	// statement.
	Loop int

	// Desc is a descriptor cell's contents: what it describes, with
	// Target nil, since the address it holds is stored there when the
	// descriptor is used (see Descriptor).
	Desc *Descriptor
}

// DatumKind is what a datum is.
type DatumKind int

// The kinds of datum.
const (
	ConstantDatum   DatumKind = iota // a numeric constant, passed by reference
	TextDatum                        // a character constant's text
	DescriptorDatum                  // a descriptor
	ControlDatum                     // an I/O statement's control block
)

// Datum is one item of the routine's read-only data. Two uses of the
// same constant, text, descriptor, or control block share one datum.
type Datum struct {
	// Name is the datum's name in the routine ($K1, $D2, $IO1), made a
	// MACRO symbol as a cell's name is.
	Name string

	Kind DatumKind

	// Value is a constant's.
	Value *Const

	// Text is a text's characters.
	Text string

	// Desc is a descriptor's contents.
	Desc *Descriptor

	// Control is a control block's contents.
	Control *Control
}

// DType is a descriptor's data type code (the OpenVMS Calling Standard's
// atomic data types, DSC$K_DTYPE_x): it tells a routine given the
// descriptor, such as FTNRTL's FTN$IO_ITEM, what kind of value it
// describes.
type DType int

// The data types the subset uses.
const (
	DTypeLU DType = 4  // a longword logical (unsigned): LOGICAL*4
	DTypeL  DType = 8  // a longword integer: INTEGER*4
	DTypeF  DType = 10 // F_floating: REAL*4
	DTypeT  DType = 14 // text: a character string
)

// String is the code's name after DSC$K_DTYPE_: LU, L, F, T.
func (d DType) String() string {
	switch d {
	case DTypeLU:
		return "LU"
	case DTypeL:
		return "L"
	case DTypeF:
		return "F"
	case DTypeT:
		return "T"
	}

	return "?"
}

// Class is a descriptor's class code (DSC$K_CLASS_x): how the rest of
// the descriptor is laid out.
type Class int

// The classes the subset uses.
const (
	ClassS Class = 1 // a scalar or a fixed-length string
	ClassA Class = 4 // an array
)

// String is the class's name after DSC$K_CLASS_: S, A.
func (c Class) String() string {
	switch c {
	case ClassS:
		return "S"
	case ClassA:
		return "A"
	}

	return "?"
}

// Descriptor is a VMS descriptor (the OpenVMS Calling Standard, chapter
// 7): how a routine is told where a value is and what it is. Every
// descriptor starts with the same two longwords, a word of length, a
// byte of data type, a byte of class, then the value's address
// (DSC$A_POINTER, at PointerOffset). An array's (class A) goes on with
// its scale, digits, flags, dimension count, and size in bytes, then
// A0 (at A0Offset), the address its element with every subscript zero
// would have, then a multiplier and a pair of bounds per dimension.
//
// The code generator lays the descriptor out. Lowering needs only the
// two address fields' places, to store addresses into a descriptor in
// a cell when they aren't known until the routine runs (a dummy
// argument's, an array element's with a subscript that isn't constant).
type Descriptor struct {
	DType DType
	Class Class

	// Length is the value's size in bytes, or an array element's.
	Length int

	// Target is what the descriptor describes, when that has an address
	// the linker knows; nil in a descriptor cell.
	Target *Mem

	// Dims are an array's bounds.
	Dims []Bound
}

// The places, in a descriptor, of the addresses lowering stores.
const (
	PointerOffset = 4  // DSC$A_POINTER
	A0Offset      = 16 // DSC$A_A0, in a class A descriptor
)

// Bound is one dimension of an array: its lower and upper bounds.
type Bound struct {
	Lower, Upper int
}

// Elements is how many elements an array descriptor's array has.
func (d *Descriptor) Elements() int {
	n := 1

	for _, b := range d.Dims {
		n *= b.Upper - b.Lower + 1
	}

	return n
}

// Size is the descriptor's own size in bytes: two longwords for class S;
// for class A five longwords, then a multiplier and two bounds for each
// dimension.
func (d *Descriptor) Size() int {
	if d.Class == ClassA {
		return 20 + 12*len(d.Dims)
	}

	return 8
}

// Control is an I/O statement's control block (Decision 12 of
// docs/PHASE-74 - FORTRAN plan.md), which the statement's first FTNRTL
// call is given. Its layout is FTNRTL's (internal/ftnrtl's
// mar/ftn_listout.mar describes it); here it says what the statement is
// and which unit it uses.
type Control struct {
	// Write is set for WRITE (*,*), clear for PRINT *.
	Write bool

	// Unit is the unit number, or DefaultUnit for *.
	Unit int
}

// DefaultUnit is the unit * names: the default output (or input) unit.
const DefaultUnit = -1

// Block is a basic block: statements run one after another, then a
// terminator that says where control goes next. Nothing jumps into the
// middle of a block, and nothing leaves it but its terminator, which is
// what lets a later pass reason about a block as a unit.
type Block struct {
	// Name is the block's name in the dump: B1, B2, ..., in layout
	// order.
	Name string

	// Label is the FORTRAN statement label at the block's start, or 0.
	Label int

	// Note says what the block is, when that isn't plain: "DO 10's
	// body".
	Note string

	Stmts []Stmt
	End   Terminator
}

// Stmt is a statement in a block: an Assign or a Call.
type Stmt interface {
	// Source is the FORTRAN statement the IR statement came from, which
	// says which source lines the code generator writes before its
	// instructions.
	Source() ast.Stmt
	stmtNode()
}

// Terminator ends a block: a Jump, a Branch, a Return, or a NoReturn.
type Terminator interface {
	Source() ast.Stmt
	terminator()
}

// origin is the FORTRAN statement an IR statement came from.
type origin struct {
	from ast.Stmt
}

// Source is the FORTRAN statement the IR statement came from.
func (o origin) Source() ast.Stmt { return o.from }

// The statements.
type (
	// Assign is Dst = Src. Src has Dst's type.
	Assign struct {
		origin
		Dst *Mem
		Src Expr
	}

	// Call is a call of an external routine, by CALLS: the arguments
	// pushed last to first, then their count. A function's value (in
	// R0) goes to Result; nil for a subroutine.
	Call struct {
		origin
		Routine string
		Args    []Arg
		Result  *Mem
	}
)

func (*Assign) stmtNode() {}
func (*Call) stmtNode()   {}

// The terminators.
type (
	// Jump goes to To.
	Jump struct {
		origin
		To *Block
	}

	// Branch goes to True if Cond's bit 0 is set, and to False if it is
	// clear: a comparison's value, or a LOGICAL's (appendix C.4 of the
	// language manual: only bit 0 of a LOGICAL counts).
	Branch struct {
		origin
		Cond        Expr
		True, False *Block
	}

	// Return returns from the routine, with Value in R0 (nil for a
	// subroutine): SS$_NORMAL from a main program, a function's value
	// from a function.
	Return struct {
		origin
		Value Expr
	}

	// NoReturn ends a block that never goes on, as after a call of
	// FTN$STOP, which ends the image.
	NoReturn struct {
		origin
	}
)

func (*Jump) terminator()     {}
func (*Branch) terminator()   {}
func (*Return) terminator()   {}
func (*NoReturn) terminator() {}

// Arg is an actual argument: by reference (Ref, whose address is
// passed) or by value (Value, a longword, as a null argument's 0 is).
// A descriptor is passed by reference to the descriptor.
type Arg struct {
	Ref   *Mem
	Value Expr
}

// Mem is a place in memory: a base, which is a cell, a datum, or the
// variable a dummy argument's address points at; a constant byte offset
// from it; and an optional index, a Long expression multiplied by the
// type's size. That is the VAX's indexed addressing mode, base[Rx]:
// TABLE(I), with TABLE(1:10), is TABLE-4[I], the element at TABLE's
// address less 4 plus 4 times I.
type Mem struct {
	Cell  *Cell
	Datum *Datum
	Param *Param

	Offset int
	Index  Expr

	// T is the type of the value there.
	T Type
}

// Expr is an expression: a tree whose leaves are constants, loads,
// and addresses.
type Expr interface {
	Type() Type
	exprNode()
}

// Operator is an operation in an Op or a Cmp.
type Operator int

// The operators. Neg, Not, and Cvt have one operand; the comparisons
// are Cmp's.
const (
	Add Operator = iota
	Sub
	Mul
	Div
	Neg
	And // bit by bit, as .AND. is on LOGICALs stored as -1 and 0
	Or
	Not
	Cvt // conversion to the Op's type: CVTLF, or CVTFL (which truncates)
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
)

// String is the operator's name in the dump.
func (o Operator) String() string {
	return [...]string{"add", "sub", "mul", "div", "neg", "and", "or", "not", "cvt",
		"eq", "ne", "lt", "le", "gt", "ge"}[o]
}

// The expressions.
type (
	// Const is a constant: Bits is its value as memory holds it, read
	// as a little-endian number (a Long's two's complement, an F_floating
	// longword's bits, 1.0 being ^X00004080).
	Const struct {
		T    Type
		Bits uint64
	}

	// Load is the value at Mem.
	Load struct {
		Mem *Mem
	}

	// Addr is Mem's address, a Long.
	Addr struct {
		Mem *Mem
	}

	// Op is an arithmetic, logical, or conversion operation, computed in
	// type T; Y is nil for Neg, Not, and Cvt. A Cvt's X has the type
	// being converted from.
	Op struct {
		Op   Operator
		T    Type
		X, Y Expr
	}

	// Cmp is a comparison of X and Y, which have one type: as a value,
	// a LOGICAL, -1 if it holds and 0 if not; as a Branch's condition,
	// a test.
	Cmp struct {
		Op   Operator
		X, Y Expr
	}
)

// Type is the constant's type.
func (c *Const) Type() Type { return c.T }

// Type is the type of the value loaded.
func (l *Load) Type() Type { return l.Mem.T }

// Type is Long: an address is a longword.
func (*Addr) Type() Type { return Long }

// Type is the operation's type.
func (o *Op) Type() Type { return o.T }

// Type is Long: a comparison's value is a LOGICAL.
func (*Cmp) Type() Type { return Long }

func (*Const) exprNode() {}
func (*Load) exprNode()  {}
func (*Addr) exprNode()  {}
func (*Op) exprNode()    {}
func (*Cmp) exprNode()   {}

// LongConst is the Long constant n.
func LongConst(n int32) *Const {
	return &Const{T: Long, Bits: uint64(uint32(n))}
}

// FloatConst is the F_floating constant v, which must already be one
// (rounded to F_floating's precision and range).
func FloatConst(v vaxfloat.Value) *Const {
	b, _ := vaxfloat.Pack(vaxfloat.F, v)

	return &Const{T: Float, Bits: b.Lo}
}

// Int is a Long constant's value.
func (c *Const) Int() int32 {
	return int32(uint32(c.Bits))
}

// Real is a Float constant's value.
func (c *Const) Real() vaxfloat.Value {
	v, _ := vaxfloat.Unpack(vaxfloat.F, vaxfloat.Bits{Lo: c.Bits})

	return v
}

// IsZero reports whether the constant is zero (0, or 0.0).
func (c *Const) IsZero() bool {
	if c.T == Float {
		return c.Real().IsZero()
	}

	return c.Int() == 0
}
