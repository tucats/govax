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
// both a longword, so both are Long here, and INTEGER*2 and LOGICAL*2 both
// a Word; the difference that matters (how a LOGICAL is tested, what a
// descriptor says it is) has been settled by the time a value is lowered.
type Type int

// The types. None is a subroutine's (absent) result.
const (
	None   Type = iota
	Long        // a longword: INTEGER*4, LOGICAL*4; the L of ADDL3
	Float       // F_floating: REAL*4; the F of ADDF3
	Byte        // a byte: BYTE, LOGICAL*1; the B of BISB3
	Word        // a word: INTEGER*2, LOGICAL*2; the W of ADDW3
	Double      // D_floating: REAL*8; the D of ADDD3
	GFloat      // G_floating: REAL*8 under /G_FLOATING; the G of ADDG3
	HFloat      // H_floating: REAL*16; the H of ADDH3
	Quad        // a quadword moved whole: COMPLEX*8; the Q of MOVQ
	Octa        // an octaword moved whole: COMPLEX*16; the O of MOVO
)

// String is the type's letter, the suffix of the VAX instructions that
// work on it: L, F, B, W, D, G, H, Q, O.
func (t Type) String() string {
	return [...]string{"", "L", "F", "B", "W", "D", "G", "H", "Q", "O"}[t]
}

// Size is the type's size in bytes.
func (t Type) Size() int {
	return [...]int{0, 4, 4, 1, 2, 8, 8, 16, 8, 16}[t]
}

// Integer reports whether t is an integer type: Byte, Word, or Long.
func (t Type) Integer() bool {
	return t == Byte || t == Word || t == Long
}

// Floating reports whether t is a floating type: Float, Double, GFloat, or
// HFloat.
func (t Type) Floating() bool {
	return t == Float || t == Double || t == GFloat || t == HFloat
}

// Format is a floating type's VAX format.
func (t Type) Format() vaxfloat.Format {
	switch t {
	case Double:
		return vaxfloat.D
	case GFloat:
		return vaxfloat.G
	case HFloat:
		return vaxfloat.H
	}

	return vaxfloat.F
}

// Width is how many registers a value of type t takes: one for a longword
// or less, two for a D, G, or Quad value (Rn and Rn+1, the low-addressed
// half in Rn), four for an H or Octa value.
func (t Type) Width() int {
	return max(t.Size()/4, 1)
}

// Routine is one program unit, lowered: its storage, its read-only data,
// and its code as a list of basic blocks.
type Routine struct {
	// Name is the routine's entry point: SQUARES, ISQ, SQUARES$MAIN.
	Name string

	Kind Kind

	// Result is a function's type, whose value Return leaves in R0 (and the
	// registers after it, for a value wider than a longword).
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
	TextCell                       // a CHARACTER temporary, for one statement's use: a concatenation's value
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

// The data types in use.
const (
	DTypeBU DType = 2  // a byte logical (unsigned): BYTE, LOGICAL*1
	DTypeWU DType = 3  // a word logical (unsigned): LOGICAL*2
	DTypeLU DType = 4  // a longword logical (unsigned): LOGICAL*4
	DTypeW  DType = 7  // a word integer: INTEGER*2
	DTypeL  DType = 8  // a longword integer: INTEGER*4
	DTypeF  DType = 10 // F_floating: REAL*4
	DTypeD  DType = 11 // D_floating: REAL*8
	DTypeFC DType = 12 // F_floating complex: COMPLEX*8
	DTypeDC DType = 13 // D_floating complex: COMPLEX*16
	DTypeT  DType = 14 // text: a character string
	DTypeG  DType = 27 // G_floating: REAL*8 under /G_FLOATING
	DTypeH  DType = 28 // H_floating: REAL*16
	DTypeGC DType = 29 // G_floating complex: COMPLEX*16 under /G_FLOATING
)

// String is the code's name after DSC$K_DTYPE_: BU, WU, LU, W, L, F, D,
// FC, DC, G, H, GC, T.
func (d DType) String() string {
	switch d {
	case DTypeBU:
		return "BU"
	case DTypeWU:
		return "WU"
	case DTypeLU:
		return "LU"
	case DTypeW:
		return "W"
	case DTypeL:
		return "L"
	case DTypeF:
		return "F"
	case DTypeD:
		return "D"
	case DTypeG:
		return "G"
	case DTypeH:
		return "H"
	case DTypeFC:
		return "FC"
	case DTypeDC:
		return "DC"
	case DTypeGC:
		return "GC"
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
	ClassD Class = 2 // a dynamic string, whose text LIB$SGET1_DD allocates
	ClassA Class = 4 // an array
)

// String is the class's name after DSC$K_CLASS_: S, D, A.
func (c Class) String() string {
	switch c {
	case ClassS:
		return "S"
	case ClassD:
		return "D"
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

// The places, in a descriptor, of the fields lowering stores: the length
// (a word, DSC$W_LENGTH, at the descriptor's start), the address, and in an
// array's, its size and A0.
const (
	PointerOffset = 4  // DSC$A_POINTER
	ArsizeOffset  = 12 // DSC$L_ARSIZE, in a class A descriptor
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

// Stmt is a statement in a block: an Assign, a CharMove, or a Call.
type Stmt interface {
	// Source is the FORTRAN statement the IR statement came from, which
	// says which source lines the code generator writes before its
	// instructions.
	Source() ast.Stmt
	stmtNode()
}

// Terminator ends a block: a Jump, a Branch, a Switch, a Sign, an
// IndirectJump, a Return, or a NoReturn.
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

	// CharMove is Dst = Src for CHARACTER values (section 3.3 of the
	// language manual): Src's characters, cut to Dst's length, or filled
	// after with blanks to it. It is MOVC3 or MOVC5, which leave R0 to R5
	// changed (Decision 2 of docs/PHASE-78 - FORTRAN expressions and
	// types.md): every operand is computed before the instruction runs, so
	// none is held across it. A Dst that overlaps Src is right, as the
	// instructions move characters as though Src were read first.
	CharMove struct {
		origin
		Dst, Src Text
	}

	// Call is a call of an external routine, by CALLS: the arguments
	// pushed last to first, then their count in longwords. A function's
	// value goes to Result, from R0 up by its type's width (R0:R3 for an
	// Octa, an OTS$..._R3 routine's); nil for a subroutine.
	Call struct {
		origin
		Routine string
		Args    []Arg
		Result  *Mem
	}
)

func (*Assign) stmtNode()   {}
func (*CharMove) stmtNode() {}
func (*Call) stmtNode()     {}

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

	// Switch goes to Targets[Index-1] when Index, a Long, is from 1 to
	// the number of targets, and to Else otherwise: the computed GO TO
	// (section 5.6.2 of the language manual), which is CASEL.
	Switch struct {
		origin
		Index   Expr
		Targets []*Block
		Else    *Block
	}

	// Sign goes to Neg, Zero, or Pos as Value, an integer or floating
	// value, is negative, zero, or positive: the arithmetic IF (section
	// 5.7.1), a TST and its branches. Two or three of the targets may be
	// the same block.
	Sign struct {
		origin
		Value          Expr
		Neg, Zero, Pos *Block
	}

	// IndirectJump goes to the address the Long at Through holds, which
	// an ASSIGN put there (a LabelAddr): the assigned GO TO (section
	// 5.6.3), JMP @Through. Targets are the blocks it may go to, which
	// the jump itself doesn't need, but which say where control goes.
	IndirectJump struct {
		origin
		Through *Mem
		Targets []*Block
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

func (*Jump) terminator()         {}
func (*Branch) terminator()       {}
func (*Switch) terminator()       {}
func (*Sign) terminator()         {}
func (*IndirectJump) terminator() {}
func (*Return) terminator()       {}
func (*NoReturn) terminator()     {}

// Arg is an actual argument: by reference (Ref, whose address is
// passed) or by value (Value, as a null argument's 0 is, or each part of
// a complex value an OTS$ routine is given): a value of any type takes as
// many longwords of the argument list as its width, in the order memory
// holds them. A descriptor is passed by reference to the descriptor. A
// routine passed (an intrinsic function's name given as an argument) is
// Routine, whose address is passed: the address of its entry mask, where a
// CALLS of it begins.
type Arg struct {
	Ref     *Mem
	Value   Expr
	Routine string
}

// Mem is a place in memory: a base, which is a cell, a datum, the
// variable a dummy argument's address points at, or the address Ptr's value
// is; a constant byte offset from it; and an optional index, a Long
// expression multiplied by the type's size. That is the VAX's indexed
// addressing mode, base[Rx]: TABLE(I), with TABLE(1:10), is TABLE-4[I], the
// element at TABLE's address less 4 plus 4 times I.
//
// Ptr is a place reached through an address computed at run time: a
// CHARACTER dummy argument's text, whose address is in the descriptor its
// argument points at (the descriptor's DSC$A_POINTER), or a dynamic
// string's.
type Mem struct {
	Cell  *Cell
	Datum *Datum
	Param *Param
	Ptr   Expr

	Offset int
	Index  Expr

	// T is the type of the value there.
	T Type
}

// Text is a CHARACTER value's place: Mem, a Byte place whose offset and
// index count bytes (the VAX's indexed mode multiplies an index by the
// operand's size, a byte's 1, so an element of a CHARACTER*8 array has its
// subscript times 8 as its index), and Len, its length in characters, a
// Long expression, constant where the compiler knows it.
type Text struct {
	Mem *Mem
	Len Expr
}

// Expr is an expression: a tree whose leaves are constants, loads,
// and addresses.
type Expr interface {
	Type() Type
	exprNode()
}

// Operator is an operation in an Op or a Cmp.
type Operator int

// The operators. Neg, Not, Cvt, Zext, Round, and Low have one operand; the
// comparisons are Cmp's.
const (
	Add Operator = iota
	Sub
	Mul
	Div
	Neg
	And // bit by bit, as .AND. is on LOGICALs stored as -1 and 0
	Or
	Xor
	Not
	Cvt // conversion to the Op's type: CVTLF, CVTFD, CVTDF (which rounds), or CVTFL (which truncates)
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
	Zext  // conversion of an unsigned integer to a longer one: MOVZWL
	Round // a floating value rounded to the nearest Long, halves away from zero: CVTRFL
	Ash   // X shifted left Y places (right -Y), Longs, the sign copied in from the left: ASHL
	Low   // an integer's low bits, as a shorter type, with no check that it fits: MOVW of a Long
)

// String is the operator's name in the dump.
func (o Operator) String() string {
	return [...]string{"add", "sub", "mul", "div", "neg", "and", "or", "xor", "not", "cvt",
		"eq", "ne", "lt", "le", "gt", "ge", "zext", "round", "ash", "low"}[o]
}

// The expressions.
type (
	// Const is a constant: Bits is its value as memory holds it, read
	// as a little-endian number (a Long's two's complement, an F_floating
	// longword's bits, 1.0 being ^X00004080, a D_floating quadword's), and
	// High the bytes of an H_floating value past its first eight. A
	// complex constant is a Quad or an Octa, its real part first, and Part
	// is its parts' type (Float, Double, or GFloat).
	Const struct {
		T          Type
		Bits, High uint64
		Part       Type
	}

	// Load is the value at Mem.
	Load struct {
		Mem *Mem
	}

	// Addr is Mem's address, a Long.
	Addr struct {
		Mem *Mem
	}

	// LabelAddr is the address of Block's first instruction, a Long: what
	// ASSIGN stores for an assigned GO TO to jump to (section 3.5).
	LabelAddr struct {
		Block *Block
	}

	// ArgPointer is the address the argument list holds for Param, a Long:
	// the longword at 4*Index(AP) itself, not what it points at. A
	// CHARACTER dummy's is its descriptor's address.
	ArgPointer struct {
		Param *Param
	}

	// Op is an arithmetic, logical, or conversion operation, computed in
	// type T; Y is nil for Neg, Not, Cvt, Zext, Round, and Low. A Cvt's,
	// Zext's, Round's, or Low's X has the type being converted from. An Ash's
	// Y is its count, a Long.
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

	// Field is the bit field of X, a Long, Size bits from bit Pos (bit 0 the
	// lowest), as an unsigned Long: EXTZV. Pos and Size are Longs, Pos from 0
	// to 31 and Size from 0 to 32 with Pos+Size at most 32.
	Field struct {
		X, Pos, Size Expr
	}

	// CharIndex is where the CHARACTER value Y first is in X, counted from
	// 1, or 0 if it isn't there (the INDEX intrinsic function): MATCHC,
	// which leaves R0 to R3 changed and says where it stopped as the bytes of
	// X after the match, R2. So it is only ever the whole of an Assign's Src,
	// as a CharCmp may be, and its value is Base less R2: Base is X's length
	// plus 1 less Y's, a constant or a Load of a temporary computed before
	// it, which needs no register.
	CharIndex struct {
		X, Y Text
		Base Expr
	}

	// CharCmp is a comparison of two CHARACTER values (section 2.3.3), in
	// the ASCII order, the shorter as if filled with blanks: CMPC3 or
	// CMPC5, which leave R0 to R3 changed. So that no register is held
	// across it, it is only ever a Branch's condition (or an operand of
	// that condition's And, Or, and Not, which branch as they go), or the
	// whole of an Assign's Src, its value a LOGICAL like Cmp's.
	CharCmp struct {
		Op   Operator
		X, Y Text
	}
)

// Type is the constant's type.
func (c *Const) Type() Type { return c.T }

// Type is the type of the value loaded.
func (l *Load) Type() Type { return l.Mem.T }

// Type is Long: an address is a longword.
func (*Addr) Type() Type { return Long }

// Type is Long: an address is a longword.
func (*ArgPointer) Type() Type { return Long }

// Type is Long: an address.
func (*LabelAddr) Type() Type { return Long }

// Type is the operation's type.
func (o *Op) Type() Type { return o.T }

// Type is Long: a comparison's value is a LOGICAL.
func (*Cmp) Type() Type { return Long }

// Type is Long: a comparison's value is a LOGICAL.
func (*CharCmp) Type() Type { return Long }

// Type is Long: the field's value, zero-extended.
func (*Field) Type() Type { return Long }

// Type is Long: a position.
func (*CharIndex) Type() Type { return Long }

func (*Const) exprNode()      {}
func (*Load) exprNode()       {}
func (*Addr) exprNode()       {}
func (*LabelAddr) exprNode()  {}
func (*ArgPointer) exprNode() {}
func (*Op) exprNode()         {}
func (*Cmp) exprNode()        {}
func (*CharCmp) exprNode()    {}
func (*Field) exprNode()      {}
func (*CharIndex) exprNode()  {}

// LongConst is the Long constant n.
func LongConst(n int32) *Const {
	return &Const{T: Long, Bits: uint64(uint32(n))}
}

// IntConst is the constant n of the integer type t: its low bits, as memory
// holds a t.
func IntConst(t Type, n int32) *Const {
	bits := uint64(uint32(n))

	switch t {
	case Byte:
		bits &= 0xFF
	case Word:
		bits &= 0xFFFF
	}

	return &Const{T: t, Bits: bits}
}

// FloatConst is the F_floating constant v, which must already be one
// (rounded to F_floating's precision and range).
func FloatConst(v vaxfloat.Value) *Const {
	return RealConst(Float, v)
}

// RealConst is the constant v of the floating type t, which v must already
// be (rounded to t's precision and range).
func RealConst(t Type, v vaxfloat.Value) *Const {
	b, _ := vaxfloat.Pack(t.Format(), v)

	return &Const{T: t, Bits: b.Lo, High: b.Hi}
}

// ComplexConst is the complex constant whose parts are the constants re and
// im, both of one floating type: a Quad for two F_floating parts, an Octa
// for two D or G ones.
func ComplexConst(re, im *Const) *Const {
	if re.T == Float {
		return &Const{T: Quad, Bits: re.Bits&0xFFFFFFFF | im.Bits<<32, Part: Float}
	}

	return &Const{T: Octa, Bits: re.Bits, High: im.Bits, Part: re.T}
}

// Parts are a complex constant's real and imaginary parts.
func (c *Const) Parts() (re, im *Const) {
	if c.T == Quad {
		return &Const{T: c.Part, Bits: c.Bits & 0xFFFFFFFF}, &Const{T: c.Part, Bits: c.Bits >> 32}
	}

	return &Const{T: c.Part, Bits: c.Bits}, &Const{T: c.Part, Bits: c.High}
}

// Int is an integer constant's value, a Byte's or Word's sign-extended.
func (c *Const) Int() int32 {
	switch c.T {
	case Byte:
		return int32(int8(c.Bits))
	case Word:
		return int32(int16(c.Bits))
	}

	return int32(uint32(c.Bits))
}

// Real is a floating constant's value.
func (c *Const) Real() vaxfloat.Value {
	v, _ := vaxfloat.Unpack(c.T.Format(), vaxfloat.Bits{Lo: c.Bits, Hi: c.High})

	return v
}

// Longwords are a floating or complex constant's bits as the longwords
// memory holds, the lowest-addressed first: one for F, two for D, G, or a
// Quad, four for H or an Octa.
func (c *Const) Longwords() []uint32 {
	n := c.T.Width()
	words := make([]uint32, n)

	for i := range words {
		b := c.Bits
		if i >= 2 {
			b = c.High
		}

		words[i] = uint32(b >> (32 * (i % 2)))
	}

	return words
}

// IsZero reports whether the constant is zero (0, 0.0, or (0.0, 0.0)).
func (c *Const) IsZero() bool {
	if c.T == Quad || c.T == Octa {
		return c.Bits == 0 && c.High == 0
	}

	if c.T.Floating() {
		return c.Real().IsZero()
	}

	return c.Int() == 0
}
