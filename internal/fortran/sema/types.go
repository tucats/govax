package sema

import "strconv"

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
// lowest; INTEGER*2 (2) and the longer types come with Phase 78.
func (t Type) rank() int {
	switch t.Kind {
	case Logical:
		return 1
	case Integer:
		return 3
	case Real:
		return 4
	}

	return 0
}

// higher is whichever of t and u ranks higher.
func higher(t, u Type) Type {
	if u.rank() > t.rank() {
		return u
	}

	return t
}
