package sema

import (
	"math"
	"strconv"

	"github.com/tucats/govax/internal/vaxfloat"
)

// Kind is a data type's kind: what sort of value it holds.
type Kind int

// The kinds of type. Invalid is the type of an expression semantic
// analysis has already reported an error in: any operation on it is
// Invalid too, without another message, so one mistake is reported once.
const (
	Invalid Kind = iota
	Integer
	Real
	Logical
	Character
	Complex
)

// Type is a data type: its kind and its size in bytes (section 2.1.1,
// Table 2-1). Every length VAX FORTRAN has is a Type: INTEGER*2 is
// {Integer, 2}, DOUBLE PRECISION {Real, 8}, BYTE {Logical, 1} (BYTE is
// LOGICAL*1), DOUBLE COMPLEX {Complex, 16}. A CHARACTER type's size is
// its length in characters, 1 to 65535, or 0 for a passed length,
// CHARACTER*(*), which a dummy argument takes from its actual argument.
//
// Declarations may give a name any of these types (Phase 76), and
// expressions of every one are compiled (Phase 78). A CHARACTER expression
// whose length isn't known until the program runs, a substring with a
// bound that isn't constant, has the size 0 too.
type Type struct {
	Kind Kind
	Size int
}

// INTEGER*4, REAL*4, LOGICAL*4, and the Invalid type. A name declared
// INTEGER or LOGICAL with no length has its unit's default (analyzer's
// defaultInt, defaultLogical), these unless /NOI4.
var (
	IntegerType = Type{Integer, 4}
	RealType    = Type{Real, 4}
	LogicalType = Type{Logical, 4}
	invalid     = Type{}
)

// String is the type as FORTRAN writes it, for messages and dumps: a
// type of its kind's default length without one (INTEGER, REAL,
// LOGICAL, COMPLEX), any other with it (INTEGER*2, REAL*8, LOGICAL*1,
// COMPLEX*16, CHARACTER*5, CHARACTER*(*)).
func (t Type) String() string {
	name, def := "", 4

	switch t.Kind {
	case Integer:
		name = "INTEGER"
	case Real:
		name = "REAL"
	case Logical:
		name = "LOGICAL"
	case Complex:
		name, def = "COMPLEX", 8
	case Character:
		if t.Size == 0 {
			return "CHARACTER*(*)"
		}

		return "CHARACTER*" + strconv.Itoa(t.Size)
	default:
		return "(invalid)"
	}

	if t.Size == def {
		return name
	}

	return name + "*" + strconv.Itoa(t.Size)
}

// HiddenResult reports whether a function of the type returns its value
// through a hidden first argument, not in registers. Up to 8 bytes come
// back in R0, or R0 and R1 (a COMPLEX*8's real part in R0, its imaginary
// part in R1). A REAL*16 or COMPLEX*16 value is too wide, so the caller
// passes the address of a place for it; and a CHARACTER value goes to the
// place a class S descriptor describes, which the caller passes, so the
// descriptor also tells a CHARACTER*(*) function its length (Decision 3 of
// docs/PHASE-79 - FORTRAN control flow and subprograms.md).
func (t Type) HiddenResult() bool {
	return t.Kind == Character || t.Size > 8
}

// Align is the boundary a variable of the type is placed on in $LOCAL:
// its natural alignment (a longword's on a multiple of 4, a word's of 2;
// a COMPLEX's that of its parts), but never more than a longword, the
// boundary the $LOCAL program section itself is aligned on, so no
// stricter one could be kept. The VAX reaches unaligned data too, but
// more slowly.
func (t Type) Align() int {
	size := t.Size

	switch t.Kind {
	case Character:
		return 1
	case Complex:
		size /= 2
	}

	return min(max(size, 1), 4)
}

// letter is the type's one-letter tag in the dump: (+:R X Y) is a REAL
// addition.
func (t Type) letter() string {
	switch t.Kind {
	case Integer:
		return "I"
	case Real:
		return "R"
	case Logical:
		return "L"
	case Character:
		return "C"
	case Complex:
		return "X"
	}

	return "?"
}

// rank is the type's rank in a mixed operation (section 2.3.1.2): the
// operand of lower rank is converted to the other's type before the
// operation, and the result has the higher rank's type. LOGICAL is
// lowest, then INTEGER*2, INTEGER*4, REAL*4, REAL*8, REAL*16, COMPLEX*8,
// and COMPLEX*16.
func (t Type) rank() int {
	switch t.Kind {
	case Logical:
		return 1
	case Integer:
		if t.Size == 2 {
			return 2
		}

		return 3
	case Real:
		switch t.Size {
		case 8:
			return 5
		case 16:
			return 6
		}

		return 4
	case Complex:
		if t.Size == 16 {
			return 8
		}

		return 7
	}

	return 0
}

// higher is the type of an operation on a t and a u: whichever ranks
// higher, but for COMPLEX*8 with REAL*8 or REAL*16, which is COMPLEX*16
// (section 2.3.1.2: the REAL*8 operand "is not rounded").
func higher(t, u Type) Type {
	if u.rank() > t.rank() {
		t, u = u, t
	}

	if t == (Type{Complex, 8}) && u.Kind == Real && u.Size > 4 {
		return Type{Complex, 16}
	}

	return t
}

// operationType is the type an arithmetic operation or a comparison on a t
// and a u is done in. A logical ranks lowest (section 2.3.1.2), so it takes
// any other operand's type. Two logicals are "treated as integers": of the
// longer one's length, and for two LOGICAL*1s INTEGER*2, there being no
// INTEGER*1 (our reading, unconfirmed).
func operationType(t, u Type) Type {
	h := higher(t, u)
	if h.Kind == Logical {
		return Type{Integer, max(t.Size, u.Size, 2)}
	}

	return h
}

// logicalType is the type of .AND., .OR., .EQV., .NEQV., or .XOR. on a t and
// a u (section 2.3.4): on logicals a logical, of the longer one's length; on
// integers, or an integer and a logical, an integer.
func logicalType(t, u Type) Type {
	if t.Kind == Logical && u.Kind == Logical {
		return Type{Logical, max(t.Size, u.Size)}
	}

	return operationType(t, u)
}

// integer is the INTEGER type a logical is treated as when it is alone in an
// arithmetic context: its own length, or INTEGER*2 for a LOGICAL*1. Any other
// type is itself.
func (t Type) integer() Type {
	if t.Kind == Logical {
		return Type{Integer, max(t.Size, 2)}
	}

	return t
}

// numeric reports whether the type is an arithmetic one: INTEGER, REAL,
// or COMPLEX.
func (t Type) numeric() bool {
	switch t.Kind {
	case Integer, Real, Complex:
		return true
	}

	return false
}

// part is a COMPLEX type's parts' type (COMPLEX*8's are REAL*4,
// COMPLEX*16's REAL*8), and any other type itself.
func (t Type) part() Type {
	if t.Kind == Complex {
		return Type{Real, t.Size / 2}
	}

	return t
}

// format is the VAX floating format of a REAL type, or of a COMPLEX
// type's parts, in the unit being analyzed (Decision 16 of docs/PHASE-74 -
// FORTRAN plan.md): REAL*4 is F_floating, REAL*8 D_floating, or G_floating
// with /G_FLOATING (section 1.4.3: the command's qualifier, or the unit's
// OPTIONS statement), REAL*16 H_floating. A unit has one format for REAL*8,
// so D and G never meet in an expression.
func (a *analyzer) format(t Type) vaxfloat.Format {
	switch t.part().Size {
	case 8:
		if a.gFloating {
			return vaxfloat.G
		}

		return vaxfloat.D
	case 16:
		return vaxfloat.H
	}

	return vaxfloat.F
}

// intRange is the range of an INTEGER type's values: INTEGER*2's or
// INTEGER*4's (section 2.2.1.1).
func (t Type) intRange() (lo, hi int64) {
	if t.Size == 2 {
		return math.MinInt16, math.MaxInt16
	}

	return minInt, maxInt
}

// realRange is a floating format's range, as REALRANGE says it (Table 2-1's
// note): F_floating and D_floating have one exponent range, G_floating a
// wider one, and H_floating a wider one still.
func realRange(f vaxfloat.Format) string {
	switch f {
	case vaxfloat.G:
		return "0.56E-308 to 0.9E308"
	case vaxfloat.H:
		return "0.84E-4932 to 0.59E4932"
	}

	return "0.29E-38 to 1.7E38"
}
