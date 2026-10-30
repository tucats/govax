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
// Declarations may give a name any of these types (Phase 76); the
// expressions this phase compiles are on the longword types, INTEGER*4,
// REAL*4, and LOGICAL*4 (Compiled), and a name of any other type is
// NOTIMPL where an expression uses it, until Phase 78.
type Type struct {
	Kind Kind
	Size int
}

// The default types, and the Invalid type.
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

// Compiled reports whether expressions of the type are compiled yet:
// INTEGER*4, REAL*4, and LOGICAL*4 are (Phase 75); the other types and
// lengths are Phase 78's.
func (t Type) Compiled() bool {
	switch t {
	case IntegerType, RealType, LogicalType:
		return true
	}

	return false
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
// type's parts (Decision 16 of docs/PHASE-74 - FORTRAN plan.md): REAL*4 is
// F_floating, REAL*8 D_floating (G_floating, with /G_FLOATING, is Phase
// 78's), REAL*16 H_floating.
func (t Type) format() vaxfloat.Format {
	switch t.part().Size {
	case 8:
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

// realRange is a REAL type's range, as REALRANGE says it: F_floating and
// D_floating have one exponent range, H_floating a far wider one.
func (t Type) realRange() string {
	if t.format() == vaxfloat.H {
		return "0.84E-4932 to 0.59E4932"
	}

	return "0.29E-38 to 1.7E38"
}
