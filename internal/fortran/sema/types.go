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
)

// Type is a data type: its kind and its size in bytes (section 2.1.1,
// Table 2-1). In this phase every variable is a longword (INTEGER*4,
// REAL*4, LOGICAL*4), and CHARACTER is only the type of a character
// constant, whose size is its length.
type Type struct {
	Kind Kind
	Size int
}

// The types of the subset's variables, and the Invalid type.
var (
	IntegerType = Type{Integer, 4}
	RealType    = Type{Real, 4}
	LogicalType = Type{Logical, 4}
	invalid     = Type{}
)

// String is the type as FORTRAN writes it, for messages and dumps:
// INTEGER, REAL, LOGICAL, CHARACTER*5.
func (t Type) String() string {
	switch t.Kind {
	case Integer:
		return "INTEGER"
	case Real:
		return "REAL"
	case Logical:
		return "LOGICAL"
	case Character:
		return "CHARACTER*" + strconv.Itoa(t.Size)
	}

	return "(invalid)"
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
	}

	return "?"
}

// rank is the type's rank in a mixed operation (section 2.3.1.2): the
// operand of lower rank is converted to the other's type before the
// operation, and the result has the higher rank's type. LOGICAL is
// lowest; INTEGER*2 (2) comes with Phase 78.
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

// implicitType is the type a name has when no declaration gives it one
// (section 2.2.2.2): INTEGER if it starts with I to N, REAL otherwise.
func implicitType(name string) Type {
	if name != "" && name[0] >= 'I' && name[0] <= 'N' {
		return IntegerType
	}

	return RealType
}
