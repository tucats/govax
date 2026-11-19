package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Program is a source file's program units after semantic analysis.
type Program struct {
	Units []*Unit

	// Modules are the modules the compilation compiled, in order, with their
	// texts, the module files to write (module.go).
	Modules []*ModuleText
	modules *modules

	// Ident is the string of the file's first CDEC$ IDENT directive, if
	// HasIdent says it has one (directive.go).
	Ident    string
	HasIdent bool
}

// Unit is one program unit: its symbol table, its storage, and its
// statements as a structured tree.
type Unit struct {
	// AST is the unit the parser built.
	AST *ast.Unit

	// Checks is what the unit's code checks: /CHECK, as its OPTIONS statements change it.
	Checks Checks

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

	// Params are the dummy arguments of the unit's header statement, in
	// order.
	Params []*Symbol

	// Dummies are every dummy argument of the unit: Params, then those only
	// ENTRY statements name, each once, in order of first appearance.
	Dummies []*Symbol

	// Entries are the entry points the unit's ENTRY statements make, in
	// order (entry.go).
	Entries []*Entry

	// Result is a function's own name, the variable whose last value is
	// the function's value; nil in a main program or subroutine. With RESULT
	// it is RESULT's variable, and the function's name is a UnitName whose
	// ResultVar it is.
	Result *Symbol

	// Recursive is set for a function or subroutine that may be active more
	// than once at a time: its header says RECURSIVE, or /RECURSIVE or an
	// OPTIONS statement's says so (the HP Fortran manual's 8.5.1.1;
	// docs/planning/PHASE-89 - reentrant FORTRAN.md). It may refer to itself and its
	// entry points. A main program or block data subprogram never is.
	Recursive bool

	// Body is the unit's executable statements, END last. IF blocks and
	// DO loops hold the statements inside them.
	Body []Stmt

	// Labels are the statement labels defined in the unit.
	Labels map[int]*Label

	// AltPlaces are the places in the header's dummy argument list of its
	// *s, alternate returns' places (section 6.1.1.5), which Params leaves
	// out: an actual argument list has no longword for one.
	AltPlaces []int

	// Assigned are the labels the unit's ASSIGN statements give, each
	// once, in the order they first appear: where an assigned GO TO
	// without a list may go.
	Assigned []int

	// LocalSize is the size of the unit's static storage, its
	// contribution to $LOCAL, in bytes.
	LocalSize int

	// FrameSize is the size of the frame area holding the unit's AUTOMATIC
	// variables, which each entry point reserves on the stack, in bytes.
	FrameSize int

	// Commons are the common blocks the unit declares, in order of first
	// appearance; a module's are its own section's too (module.go).
	// UsedBlocks are the unit's views of the program sections of the modules
	// it uses, each holding the unit's names of the module's variables
	// (use.go): no statement of the unit lays them out.
	Commons    []*CommonBlock
	UsedBlocks []*CommonBlock

	// usedStructs are the types USE statements gave the unit, each with its
	// module's name (use.go), and usedTypes the same by local name, in the
	// order the USEs gave them; ambiguousTypes are the local names two USEs
	// gave two types, with the two modules, an error only when used.
	usedStructs    map[*Structure]string
	usedTypes      []usedType
	ambiguousTypes map[string][2]string

	// hidden are the unit's copies of a used module's entities that no name
	// of the unit reaches: a generic interface's specific, a namelist group's
	// item, known here only through what holds them (use.go).
	hidden []*Symbol

	// access is a module's accessibility: its default, and what its PUBLIC and
	// PRIVATE statements and attributes gave (access.go). nil in any other
	// unit.
	access *moduleAccess

	// Initials are the initial values DATA statements and type statements
	// give the unit's names, in order of place (data.go).
	Initials []*Initial

	// Formats are the unit's FORMAT statements, by label, and Namelists
	// its NAMELIST groups, in order (io.go).
	Formats   map[int]*Format
	Namelists []*Namelist
	namelists map[string]*Namelist

	// GFloating is set when the unit's REAL*8 and COMPLEX*16 are in
	// G_floating (/G_FLOATING, or the unit's OPTIONS statement), clear
	// when they are in D_floating.
	GFloating bool

	// NoF77 is set when the unit is compiled with /NOF77, which the manual gives four
	// meanings: a DO runs at least once (section 5.3.1), OPEN's BLANK= defaults to 'ZERO'
	// (9.1.3) and STATUS= to 'NEW' (9.1.25), and EXTERNAL takes appendix A.6's form.
	NoF77 bool

	// F90 is set when the unit is compiled with /F90 (the default for a free-form
	// source), or its OPTIONS statement says /F90: where Fortran 90 and VAX FORTRAN
	// read a text differently, it is read as Fortran 90 reads it
	// (docs/planning/PHASE-99 - Fortran 90.md, Decision 2).
	F90 bool

	// LocalAlign is the alignment of the $LOCAL section, as a power of two, that
	// the unit's aligned records need beyond a longword's (align.go), or 0.
	LocalAlign int

	// Structures are the unit's structure declarations, nested ones
	// included, in the order they begin (structure.go); structs are the
	// named ones by name, which RECORD statements look in.
	Structures []*Structure
	structs    map[string]*Structure

	// typesKnown is set once the unit's declarations are read, when every
	// derived type it may name is defined; pendingObjects are the names
	// IMPLICIT TYPE(t) gave a type before then, and implicitFailed the types
	// such a letter named that aren't there (derived.go). headerType is a
	// TYPE(t) FUNCTION statement's type-spec, whose type may be defined in
	// the function's own body, so it too waits for the declarations.
	typesKnown     bool
	pendingObjects []*Symbol
	implicitFailed map[string]bool
	headerType     *ast.TypeSpec

	// scope is the unit's own names (scope.go), the names in Symbols.
	scope *scope

	// LinkName is a module procedure's global name, M.P, or its hashed form
	// past 31 characters (module.go's LongName); "" for any other unit.
	LinkName string

	// Host is the unit an internal procedure lies in, or nil for an external
	// unit; Contains are a unit's internal procedures, in order (internal.go;
	// docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md). An
	// internal procedure is in Program.Units too, after its host.
	Host     *Unit
	Contains []*Unit

	// Interfaces are the bodies of the unit's interface blocks, each a Unit
	// with a header and declarations but no statements, and no unit of the
	// program: it is in no Program.Units (interface.go).
	Interfaces []*Unit

	// operators are the unit's interface blocks for operators and assignment,
	// by the operator as OPERATOR( ) writes it in its dotted spelling
	// (.CROSS., +, .EQ.) and "=" for ASSIGNMENT(=) (operators.go).
	operators map[string]*Generic

	// others and labelRefs are the mentions of the names that have no Symbol, and
	// of the statement labels (crossref.go).
	others    map[otherName][]Reference
	labelRefs map[int][]Reference

	// notRead are the mentions of names that neither read nor give a value:
	// an actual argument passed as itself, which the routine may do either
	// with, and PRESENT's argument (intent.go's reportIntentOut).
	notRead map[Reference]bool

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
	return u.scope.find(name)
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

	// onlyDeclared is set for a name a unit declares and never uses, which
	// layout makes a variable: so it is no use of another unit's name
	// (checkReferences): a unit may declare LOGICAL F for a function F
	// it never calls.
	onlyDeclared bool

	// untyped is set for a name entered under IMPLICIT NONE, whose
	// first letter gives it no type: unless a declaration gives it one,
	// it is NOTYPE.
	untyped bool

	// typesDummy is set for a name a statement function's dummy argument shares,
	// whose declaration gives that dummy its type, so is of some use even if nothing
	// else mentions the name (neverUsed).
	typesDummy bool

	// included is set for a name first seen in a file INCLUDE brought in, in a
	// declaration (neverUsed).
	included bool

	// Struct is a record's structure, which a RECORD statement gave it
	// (section 4.13), its Type then of the kind Record; nil for any other
	// name.
	Struct *Structure

	// refs are the unit's mentions of the name (crossref.go).
	refs []Reference

	// Dims are an array's dimensions; DimPos is where they were
	// declared.
	Dims   []Dim
	DimPos diag.Position

	// Dummy is the name's position among the unit's Dummies, 1 for the
	// first, or 0 if it isn't one. A dummy argument has no storage of
	// its own: in a unit without ENTRY statements, its address is the
	// argument list's longword Dummy, at 4*Dummy(AP).
	Dummy int

	// Result is set for a function's own name within the function, and for
	// its entry points' names: or, with RESULT, for the variable RESULT names.
	Result bool

	// ResultVar is, for a function's or a function's entry point's name whose
	// statement says RESULT, the variable RESULT names, which holds its value
	// and has its type (the HP Fortran manual's 8.5.2.1); nil otherwise.
	ResultVar *Symbol

	// laterEntry is the ENTRY statement that first names the name, an entry
	// point or a dummy argument, while the statement walk hasn't reached it.
	laterEntry *ast.EntryStmt

	// BoundDummies are the dummy arguments in an adjustable array's bounds:
	// an entry point can compute the bounds only if its list has them all.
	BoundDummies []*Symbol

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

	// library is set for a name first seen in a statement from a text
	// library's module. ownExternal is set for a name in an EXTERNAL statement
	// of the program's own, and passed for an external name given as an
	// argument (Symbol.Linked).
	library     bool
	ownExternal bool
	passed      bool

	// Automatic is set for a name in an AUTOMATIC statement, or tied to one
	// by EQUIVALENCE: its Offset is its place in the unit's frame area
	// (Unit.FrameSize), not $LOCAL (storage.go).
	Automatic bool

	// Volatile is set for a name a VOLATILE statement names, directly, by its
	// common block, or by EQUIVALENCE with such a name (storage.go). It says
	// that no reference may be optimized away, which Phase 84's optimizer will
	// ask.
	Volatile bool

	// Intent is a dummy argument's INTENT, IN, OUT, or INOUT, and Optional
	// and Target its OPTIONAL and TARGET attributes: recorded, and meaning
	// nothing yet (docs/planning/PHASE-100 - Fortran 90 declarations and
	// kinds.md, finding 11). attrs are the attributes statements have given the
	// name, for attrtable.go's checks.
	Intent   string
	Optional bool
	Target   bool
	attrs    attribute

	// Allocatable is Fortran 90's ALLOCATABLE: an array of deferred shape whose
	// storage is its descriptor, class NCA, until ALLOCATE gives it elements
	// (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, finding 4).
	Allocatable bool

	// FreedAtReturn is set for an allocatable array that every return of its
	// unit deallocates, and every entry point marks unallocated: a
	// subprogram's own that isn't saved (finding 5; freedAtReturn).
	FreedAtReturn bool

	// AutomaticObject is Fortran 90's automatic array (the standard's
	// 5.1.2.4.1): a subprogram's own array, not a dummy argument, whose bounds
	// aren't all constants. Its bounds are computed at each entry point whose
	// list defines them, as an adjustable array's are, and its elements are made
	// on the stack then and last until the routine returns. Its own storage is
	// the class A descriptor that says where they are (docs/planning/PHASE-104 -
	// Fortran 90 dynamic arrays.md, finding 7).
	AutomaticObject bool

	// LengthExpr is a CHARACTER length that isn't constant, CHARACTER(LEN=N)
	// with N a dummy argument, as written (the Fortran 90 standard's 5.1.1.5),
	// and LengthValue its value, an INTEGER expression computed at each entry
	// point whose list defines it (BoundDummies). Type.Size is then 0, as for a
	// passed length, CHARACTER*(*), which PassedLength tells apart. A dummy
	// argument's text is its actual argument's first LengthValue characters; any
	// other name's is an automatic object (AutomaticObject), made on the stack
	// (charlength.go).
	LengthExpr  ast.Expr
	LengthValue Expr

	// f90Saved is set when a Fortran 90 form gave the name SAVE (an
	// attribute, SAVE ::, or an initialization), for the dump.
	f90Saved bool

	// saved is set for a name in a SAVE statement's list. storageStmt is the
	// AUTOMATIC or STATIC statement naming it, a name's only one.
	saved       bool
	storageStmt *ast.AttrStmt

	// automaticPos is where the AUTOMATIC statement names the name, or the
	// name that made it AUTOMATIC.
	automaticPos diag.Position

	// Equivalenced is set for a name in an EQUIVALENCE list, whose storage
	// may overlap other names'. group is the names it is tied to, and
	// groupOffset its place among them; equivPos is its first appearance
	// in a list (equivalence.go).
	Equivalenced bool
	commonAlign  Alignment     // the setting where its COMMON statement is (align.go)
	commonPos    diag.Position // where that statement names it
	group        *equivGroup
	groupOffset  int
	equivPos     diag.Position

	// Pos is where the name first appears.
	Pos diag.Position

	// Value is a constant's value, with its type; nil for a constant
	// whose PARAMETER statement had an error. An array constant (Fortran 90's
	// array PARAMETER) has Dims, and its value is ArrayValue.
	Value      *Const
	ArrayValue *ArrayConst

	// recordValue is the initial value a Fortran 90 declaration gives an
	// object of a derived type, TYPE(POINT) :: P = POINT(1.0, 2.0)
	// (structvalue.go), every element's if it is an array; recordValues an
	// array's from an array value, an element's each, in array element order.
	recordValue  *Const
	recordValues []*Const

	// Internal is the internal procedure a host's name stands for: the name of
	// a FUNCTION or SUBROUTINE after the host's CONTAINS, whose interface is the
	// procedure's own declarations (internal.go). nil for any other name.
	Internal *Unit

	// Interface is the interface body that gives an external procedure or a
	// dummy procedure its explicit interface (interface.go). nil for any other
	// name.
	Interface *Unit

	// Generic is what a generic name stands for in this unit: its specific
	// procedures (generic.go). nil for any other name. A generic name may be a
	// specific procedure's own too.
	Generic *Generic

	// StmtFunc is a statement function's definition, once the statement
	// walk has reached it (stmtfunc.go).
	StmtFunc *StmtFuncDef

	// defPos is where a statement function or a constant is defined;
	// oldForm is set for a constant appendix A.4's PARAMETER defined,
	// whose type is its value's.
	defPos  diag.Position
	oldForm bool

	// Use is where a use-associated name came from: the module, and its name
	// there (use.go). nil for a name of the unit's own.
	Use *UseOrigin

	// from is, for an entity of a module read from its text, the module that
	// defines it and its name there, when the text says it came from another
	// module (a FROM line); nil for the text's module's own (modread.go).
	from *entityOrigin

	// moduleProc is set on a use-associated module procedure's copy (an
	// internal procedure's Symbol has its Internal instead).
	moduleProc bool

	// private is set on a module's entity that its users can't reach: given
	// PRIVATE, or by the module's default (access.go).
	private bool

	// ambiguous is set on a name two USEs gave for two entities: the second
	// module's name. Referencing it is an error (USEAMBIG, 11.3.2).
	ambiguous string

	// LinkName is the name the MACRO file calls a procedure by, when that
	// isn't its name here: a module procedure's M.P (or its hashed form,
	// LongName), in its module and wherever a USE gives it, and a procedure a
	// USE renamed. "" for any other name.
	LinkName string

	// broken is set when a declaration of the name had an error, which
	// has been reported: its uses get no messages of their own, which
	// would only repeat the mistake.
	broken bool
}

// Dim is one dimension of an array: its bounds. Assumed is an
// assumed-size dummy array's last dimension, written *, whose upper bound
// is unknown. LowerExpr and UpperExpr are an adjustable array's bounds
// that aren't constant, which are computed when the subprogram is entered
// (section 6.1.1.1); nil for a constant bound, which is
// Lower or Upper.
type Dim struct {
	Lower, Upper         int
	Assumed              bool
	LowerExpr, UpperExpr ast.Expr

	// Deferred is a dimension of Fortran 90's deferred shape, written (:): an
	// allocatable array's, whose bounds ALLOCATE gives it when the program
	// runs (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md). Neither
	// bound is known, and Lower and Upper mean nothing.
	Deferred bool

	// AssumedShape is a dimension of a dummy argument's assumed shape (the
	// Fortran 90 standard's 5.1.2.4.2), written (:) or (lb:): its extent is the
	// actual argument's, and its lower bound Lower (or LowerExpr), 1 when it is
	// left out, never the actual's. Its upper bound is known only when the
	// routine is entered (docs/planning/PHASE-104 - Fortran 90 dynamic
	// arrays.md, finding 9).
	AssumedShape bool

	// lowerGiven says a deferred shape's colon had a lower bound before it,
	// (lb:), which only an assumed shape may have (checkShapes).
	lowerGiven bool

	// LowerValue and UpperValue are those bounds analyzed and converted to
	// INTEGER, which lowering computes when the subprogram is entered.
	LowerValue, UpperValue Expr

	// Runtime is a dimension of one of lowering's array temporaries whose
	// extent is known only when the program runs (NewRuntimeArray): its lower
	// bound is 1, Lower, and its upper bound is in its descriptor.
	Runtime bool
}

// known reports whether the dimension's bounds are both known now.
func (d Dim) known() bool {
	return !d.Assumed && d.LowerExpr == nil && d.UpperExpr == nil && !d.Deferred && !d.Runtime && !d.AssumedShape
}

// LowerKnown reports whether the dimension's lower bound is a constant, Lower.
func (d Dim) LowerKnown() bool {
	return d.LowerExpr == nil && !d.Deferred
}

// UpperKnown reports whether the dimension's upper bound is a constant, Upper:
// not adjustable, not deferred, nor an assumed-size array's *.
func (d Dim) UpperKnown() bool {
	return d.UpperExpr == nil && !d.Deferred && !d.Assumed && !d.Runtime && !d.AssumedShape
}

// Extent is how many elements the dimension has, du - dl + 1 (section
// 2.2.3.1), if both its bounds are known.
func (d Dim) Extent() int {
	return d.Upper - d.Lower + 1
}

// Adjustable reports whether s is an adjustable array: a dummy array with a
// bound that isn't constant (section 6.1.1.1). An assumed-shape array isn't
// one, whatever its lower bounds.
func (s *Symbol) Adjustable() bool {
	if s.AssumedShape() {
		return false
	}

	for _, d := range s.Dims {
		if d.LowerExpr != nil || d.UpperExpr != nil {
			return true
		}
	}

	return false
}

// AssumedShape reports whether s is an assumed-shape dummy array (the Fortran
// 90 standard's 5.1.2.4.2), A(:, 0:): every dimension's extent is its actual
// argument's.
func (s *Symbol) AssumedShape() bool {
	return len(s.Dims) > 0 && s.Dims[0].AssumedShape
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

// TypeText is the name's type as maps and dumps show it: its Type's, or
// for a record its structure's name, /DATE/.
func (s *Symbol) TypeText() string {
	if s.Struct != nil {
		return s.Struct.Display()
	}

	if s.LengthExpr != nil {
		return "CHARACTER*(" + lengthText(s.LengthExpr) + ")"
	}

	return s.Type.String()
}

// Size is the storage the name takes, in bytes.
func (s *Symbol) Size() int {
	// An automatic CHARACTER scalar's is a class S descriptor, its length
	// and address.
	if s.AutomaticObject && s.Dims == nil {
		return StringDescriptorSize
	}

	if s.Allocatable || s.AutomaticObject {
		return DescriptorSize(len(s.Dims))
	}

	return s.Type.Size * s.Elements()
}

// StringDescriptorSize is the size in bytes of a class S descriptor: a word of
// length, its type and class bytes, and the address.
const StringDescriptorSize = 8

// DescriptorSize is the size in bytes of an array descriptor of rank n, class A
// or NCA (the OpenVMS Calling Standard's 5.4 and 5.7): five longwords, then
// for each dimension a multiplier or stride and two bounds. An allocatable
// array's storage is one of these (docs/planning/PHASE-104 - Fortran 90
// dynamic arrays.md, finding 4).
func DescriptorSize(n int) int {
	return 20 + 12*n
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
	// has Target's type: semantic analysis converted it (Table 3-1). An
	// array assignment's target is a whole array or a section, and Order is
	// how its loops run (overlap.go); a scalar assignment's Order is nil.
	Assign struct {
		stmtBase
		Target Expr
		Value  Expr
		Order  *ArrayOrder

		// InPlace says a structure constructor's value may be built straight
		// in the target, a component at a time, since none of its values reads
		// the target (structvalue.go's inPlace); otherwise it is built in a
		// temporary and moved.
		InPlace bool
	}

	// Call is CALL Sub(Args).
	Call struct {
		stmtBase
		Sub  *Symbol
		Args []Arg

		// AltReturns are the labels of the alternate return specifiers, in
		// order, which the argument list doesn't hold (altreturn.go).
		AltReturns []int
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
	// end are a Do's. Cond is nil for Fortran 90's DO with no loop control,
	// which runs until something leaves it (EXIT, GO TO, RETURN, STOP).
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

	// LoopExit is Fortran 90's EXIT, or CYCLE when Cycle is set (the
	// standard's 8.1.4.4): EXIT leaves Loop, a *Do or *DoWhile, and every
	// loop inside it; CYCLE goes on to Loop's next pass, as a jump to its
	// END DO would.
	LoopExit struct {
		stmtBase
		Cycle bool
		Loop  Stmt
	}

	// Select is Fortran 90's CASE construct (the standard's 8.1.3):
	// SELECT CASE (Expr), an INTEGER, CHARACTER, or LOGICAL value computed
	// once, then Cases in the order written, each a CASE statement and its
	// block, DEFAULT among them wherever it was written (Default points at
	// it, nil for none). At most one case matches Expr's value, since no
	// two overlap; with none, Default's block runs, or nothing. EndSelect
	// is the END SELECT statement, which a jump from any block may go to.
	Select struct {
		stmtBase
		Expr      Expr
		Cases     []*Case
		Default   *Case
		EndSelect ast.Stmt
	}

	// Where is a WHERE construct, or the WHERE statement as a construct of
	// one assignment (where.go): Mask a LOGICAL array; Then the assignments
	// done where it is true, Else ELSEWHERE's, where it is false (nil for
	// none), each an *Assign to an array of the mask's shape; EndWhere the
	// END WHERE statement (nil for the WHERE statement). MaskTemp is the
	// mask's extents when it is computed once into a temporary before the
	// assignments, nil when each assignment's loops compute it.
	Where struct {
		stmtBase
		Mask     Expr
		Then     *Arm
		Else     *Arm
		EndWhere ast.Stmt
		MaskTemp []int // each extent, or -1 for one known only when the program runs (tempShape)
	}

	// Stop is STOP, or PAUSE when Pause is set, with the text it
	// displays if it has one.
	Stop struct {
		stmtBase
		Display    string
		HasDisplay bool
		Pause      bool
	}

	// Return is RETURN in a subprogram; Alt, an INTEGER*4, is RETURN i's
	// value, nil for a plain RETURN.
	Return struct {
		stmtBase
		Alt Expr
	}

	// End is END, the unit's last statement.
	End struct {
		stmtBase
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

// Case is a CASE statement and its block in a CASE construct: its Arm (AST
// the CASE statement, no Cond, Body the block, so that the analyzer's
// stack of blocks treats it as an IF's arm), and its Ranges, or Default.
type Case struct {
	Arm
	Default bool
	Ranges  []CaseRange
}

// CaseRange is one case value range, its ends constants of the selector's
// type (an INTEGER one of any kind): a single value (Low and High the same
// constant), low: (High nil), :high (Low nil), or low:high.
type CaseRange struct {
	Low, High *Const
}

// Single reports whether r is a single value, not a range.
func (r CaseRange) Single() bool { return r.Low != nil && r.Low == r.High }

// Expr is an expression with its type and shape.
type Expr interface {
	Type() Type
	Shape() Shape
	Position() diag.Position
	exprNode()
}

// exprBase is what every expression has: its type, its position, and its
// shape. An array-valued expression's type is its elements' (Fortran 90's
// shape is the expression's, not the type's: docs/planning/PHASE-102 -
// Fortran 90 array expressions.md, finding 3), and S is nil for a scalar.
type exprBase struct {
	T   Type
	Pos diag.Position
	S   Shape
}

// Type is the expression's type: an array's elements'.
func (b *exprBase) Type() Type { return b.T }

// Shape is the expression's shape, nil for a scalar (shape.go).
func (b *exprBase) Shape() Shape { return b.S }

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

		// Raw is a REAL or COMPLEX constant's bytes as memory holds them, for
		// an octal, hexadecimal, or Hollerith constant only moved to a place
		// of its type (typeless.go). Its bits needn't be a number, and needn't
		// be what Real and Imag would pack to.
		Raw []byte

		// Parts are a derived type's constant's component values, one per
		// field of its structure in order: a *Const for a scalar component (a
		// derived type's own a Const like this one), an *ArrayConst for an
		// array component. Raw is then the whole value's bytes, padding zero
		// (structvalue.go).
		Parts []Expr
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

		// InBounds says which of its subscripts needn't be checked by
		// /CHECK=BOUNDS (nil: none): in an element the scalarizer made of a
		// section, those whose first and last are checked once, before its
		// loops, but not a vector subscript's, each of whose values is
		// checked (ir/lowerarray.go).
		InBounds []bool
	}

	// ArrayRef is a whole array: an actual argument or an I/O list item
	// (section 2.2.3.5), or since Fortran 90 an array-valued operand or
	// assignment target. Its type is its elements', and its shape its
	// dimensions' extents.
	ArrayRef struct {
		exprBase
		Sym *Symbol
	}

	// ArrayConst is an array of constants (Fortran 90's 7.1.6.1): a
	// constructor of constants, an array PARAMETER, a section of one with
	// constant subscripts, RESHAPE of constants, or an operation on them. Its
	// type is its elements', its shape known, and Elems its elements in array
	// element order (the leftmost subscript varying fastest).
	ArrayConst struct {
		exprBase
		Elems []*Const
	}

	// ArrayFunc is a reference to one of Fortran 90's array functions that
	// the compiler computes in line (arrayfuncs.go): Name; Array, its first
	// argument (ARRAY, MASK of ALL, ANY, and COUNT, MATRIX_A, MATRIX); Other,
	// MATRIX_B; Mask, MASK= of the others; Dim, a constant DIM= (0 for none),
	// or DimExpr, one not constant of SIZE, LBOUND, or UBOUND. DOT_PRODUCT is
	// a SUM or an ANY of a product.
	ArrayFunc struct {
		exprBase
		Name    string
		Array   Expr
		Other   Expr
		Mask    Expr
		Dim     int
		DimExpr Expr

		// Pad is EOSHIFT's BOUNDARY or RESHAPE's PAD, and Order RESHAPE's
		// ORDER, from 0, each dimension's place in the order its subscripts
		// vary (transform.go).
		Pad   Expr
		Order []int
	}

	// ArrayFuncAt is F's element at the indexes Index, the index variables of
	// an array statement's loops, one per dimension of F: an element of a
	// reduction with DIM= or of MATMUL, which lowering computes by a loop of
	// its own.
	ArrayFuncAt struct {
		exprBase
		F     *ArrayFunc
		Index []*Symbol
	}

	// Constructor is an array constructor whose values aren't all constants
	// (Fortran 90's 4.5): (/ X, A(2:3), (I * Y, I = 1, N) /), of rank one,
	// its extent known when compiling only if every item's is. LengthOf is,
	// for one of CHARACTER values whose length is known only when the program
	// runs (T.Size 0), the value whose length its elements have, its first:
	// a variable, an array, or a concatenation of them (constructor.go).
	Constructor struct {
		exprBase
		Items    []CtorItem
		LengthOf Expr
	}

	// StructValue is a structure constructor whose values aren't all
	// constant (the standard's 4.4.4; structvalue.go): POINT(X, 2.0), a value
	// of the derived type T.Struct. Values are its components' values, one per
	// field of the structure in order, each converted to its component's type
	// as an assignment converts: a scalar's an expression of its type, a
	// derived type's component a record of that type, an array component's
	// constant (Bytes holds its bytes, nil for any other component). A
	// constructor whose values are all constant is a Const instead.
	StructValue struct {
		exprBase
		Values []Expr
		Bytes  [][]byte
	}

	// Section is Fortran 90's array section, Sym(Subs) with a subscript
	// triplet in at least one dimension (the standard's 6.2.2): a
	// subscript per dimension of Sym. Its rank is the number of triplets,
	// and its extent in each is its triplet's number of values (shape.go).
	Section struct {
		exprBase
		Sym  *Symbol
		Subs []SectionSub
	}

	// FuncRef is a reference to an external function. Made says the function
	// makes its value itself, an array of a shape or a CHARACTER value of a length
	// its explicit interface gives as specification expressions, which only it can
	// work out (arrayvalue.go).
	FuncRef struct {
		exprBase
		Fn   *Symbol
		Args []Arg
		Made bool
	}

	// StmtFuncRef is a reference to a statement function (section 6.2.1):
	// its definition, and an actual argument of each dummy argument's type
	// (of any length, for CHARACTER). Lowering expands it in line.
	StmtFuncRef struct {
		exprBase
		Def  *StmtFuncDef
		Args []Expr
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

	// Procedure is a procedure's name given as an actual argument (sections
	// 4.7, 4.9, 6.1, and 6.3.1.1): the routine passed, so that the routine
	// given it can call it. Name is the name the program wrote, and Routine
	// the routine (MTH$SIN, FTN$LEN, SUB; routines.go). A procedure that is a
	// dummy argument itself has no routine to name, and is Dummy, passed on.
	// Its type is the function's value's, or for a name that may be a
	// subroutine what its first letter says. A CHARACTER one goes by
	// descriptor, and any other by its address (Table 6-1).
	Procedure struct {
		exprBase
		Name    string
		Routine string
		Dummy   *Symbol
	}

	// Loc is %LOC(X), the address of a variable, an array, an array element,
	// a substring, or a procedure, an INTEGER*4 (section 6.1.2.2).
	Loc struct {
		exprBase
		X Expr
	}

	// Present is PRESENT(A), whether the OPTIONAL dummy argument Sym, the
	// unit's own or its host's, was given an actual argument (the Fortran 90
	// standard's 13.13.80; optional.go). Its type is the default LOGICAL.
	Present struct {
		exprBase
		Sym *Symbol
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
	AbsOp       IntrinsicOp = iota + 1 // |X|, of an INTEGER or REAL X
	MaxOp                              // the largest of the arguments
	MinOp                              // the smallest
	DimOp                              // X-Y if X is greater, else 0
	SignOp                             // |X| with Y's sign (+ for a Y of 0)
	ModOp                              // X - (X/Y)*Y, of INTEGERs
	NintOp                             // a REAL rounded to the nearest INTEGER, halves away from 0
	ZextOp                             // an INTEGER or LOGICAL made longer with zeros, not its sign
	ImagOp                             // a COMPLEX value's imaginary part
	CmplxOp                            // the COMPLEX value (X, Y)
	ConjgOp                            // a COMPLEX value with its imaginary part negated
	ShiftOp                            // ISHFT(I, N): I's bits shifted left N places, right -N; N constant
	BitsOp                             // IBITS(I, POS, LEN): LEN bits of I from bit POS
	BitSetOp                           // IBSET(I, POS): I with bit POS set
	BitClearOp                         // IBCLR(I, POS): I with bit POS clear
	BitTestOp                          // BTEST(I, POS): bit POS of I, a LOGICAL
	LenOp                              // a passed-length CHARACTER value's length
	ArraySizeOp                        // SIZEOF of an adjustable array
	IndexOp                            // INDEX(C1, C2): where C2 first is in C1, or 0
	IcharOp                            // a CHARACTER value's first character's code
	CharOp                             // the CHARACTER*1 whose code is X
	CallOp                             // a call of Routine, MTHRTL's or FTNRTL's, with the arguments by reference
	MergeOp                            // MERGE(T, F, MASK): T if MASK is true, else F
)

// String is the operation's name in the dump.
func (op IntrinsicOp) String() string {
	return [...]string{"?", "abs", "max", "min", "dim", "sign", "mod", "nint", "zext", "imag", "cmplx",
		"conjg", "shift", "bits", "bitset", "bitclear", "bittest", "len", "arraysize", "index",
		"ichar", "char", "call", "merge"}[op]
}

// Arg is an actual argument. Expr is nil for a null argument (CALL
// S(A,,B)). Copy is set for a parenthesized variable or array element,
// (X), which is passed as a copy of its value, so the subprogram can't
// change X itself (section 5.1). Any other expression that isn't a
// variable, an array element, or a whole array is passed as a copy too.
// Mode is how the argument goes, when that isn't its type's default (Table
// 6-1). Hollerith is set for a Hollerith constant, whose Expr is a CHARACTER
// constant holding its text and whose Mode is ByRef.
type Arg struct {
	Expr      Expr
	Copy      bool
	Mode      ArgMode
	Hollerith bool

	// Typeless is set for an octal or hexadecimal constant: it has no type, so
	// its Expr's INTEGER*4 is only its four bytes (section 2.2.1.4).
	Typeless bool

	// Shaped is set for the actual argument of an assumed-shape dummy, which
	// goes by a class NCA descriptor (shaped.go): a whole array or a section of
	// one, or an array expression, a vector-subscripted section, or an array in
	// parentheses (Copy), whose value goes in a temporary.
	Shaped bool

	// Sequence is set for an array section or array expression given to a
	// dummy that isn't assumed-shape, or a whole assumed-shape array: what is
	// passed is the address of its elements one after another (sequence
	// association), copied into a temporary unless they already are
	// (sequence.go). CopyBack says such a copy goes back to the actual after
	// the call: the actual is definable, and the dummy isn't INTENT(IN).
	Sequence, CopyBack bool

	// expression is set by argValue for an array section or array expression,
	// for its dummy to decide on (shaped, sequenced, arrayChecked).
	expression bool
}

// ArgMode is how an actual argument is passed.
type ArgMode int

// The modes. A CHARACTER value's default is by descriptor, and any other's
// by reference.
const (
	ByDefault    ArgMode = iota
	ByValue              // %VAL: the value itself, as a longword
	ByRef                // %REF: its address, even a CHARACTER value's
	ByDescriptor         // %DESCR: a descriptor's address, even a number's
)

// String is the mode's built-in function's name, for the dump.
func (m ArgMode) String() string {
	return [...]string{"", "%VAL", "%REF", "%DESCR"}[m]
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

// CtorItem is one item of an array constructor: a value, scalar or array, or
// an implied-DO list, Do.
type CtorItem struct {
	Value Expr
	Do    *CtorDo
}

// CtorDo is an implied-DO list in an array constructor, (Items, Var = Start,
// Limit, Step): Var an INTEGER variable of the list's own (the standard's
// 14.1.3: the implied DO's variable has its scope), Step 1 when left out.
type CtorDo struct {
	Var                *Symbol
	Start, Limit, Step Expr
	Items              []CtorItem
}

// at is the array constant at pos.
func (c *ArrayConst) at(pos diag.Position) *ArrayConst {
	d := *c
	d.Pos = pos

	return &d
}
