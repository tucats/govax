package cpu

// DataType is the kind of data one instruction operand holds, as the VAX
// Architecture Reference Manual gives it in each instruction's format line:
// the second letter of an operand such as "sum.wl" (l, a longword). Each
// Instruction table entry carries one DataType per operand slot, next to
// its AccessKind (the first letter) and its Scale (the size in bytes, which
// the data type decides).
//
// The size alone isn't enough to know what an operand means. D_floating
// and G_floating are both 8 bytes, and H_floating and an octaword are both
// 16, but each lays its bits out differently, so code that loads or stores
// a floating value needs the format, not just the size. A conversion
// mixes types in one instruction: CVTFG reads an F_floating value and
// writes a G_floating one.
//
// For an address operand ("addr.ab") the data type is that of the data at
// the address, and for a branch displacement ("displ.bb") it is the
// displacement's own size.
type DataType int

// The VAX data types an operand specifier can name. In Go, a const block
// with iota numbers the constants 0, 1, 2, ... in order, so DataNone (the
// zero value of a DataType) is what every unused operand slot holds.
const (
	DataNone      DataType = iota // operand slot unused
	DataByte                      // b: 8-bit integer
	DataWord                      // w: 16-bit integer
	DataLongword                  // l: 32-bit integer
	DataQuadword                  // q: 64-bit integer
	DataOctaword                  // o: 128-bit integer
	DataFFloating                 // f: 32-bit floating, 8-bit exponent, 24-bit fraction
	DataDFloating                 // d: 64-bit floating, 8-bit exponent, 56-bit fraction
	DataGFloating                 // g: 64-bit floating, 11-bit exponent, 53-bit fraction
	DataHFloating                 // h: 128-bit floating, 15-bit exponent, 113-bit fraction
)

// dataTypeInfo describes each DataType: the manual's letter for it, and its
// size in bytes. It is an array indexed by DataType, so dataTypeInfo[t]
// looks t up directly.
var dataTypeInfo = [...]struct {
	letter string
	size   int
}{
	DataNone:      {"", 0},
	DataByte:      {"b", 1},
	DataWord:      {"w", 2},
	DataLongword:  {"l", 4},
	DataQuadword:  {"q", 8},
	DataOctaword:  {"o", 16},
	DataFFloating: {"f", 4},
	DataDFloating: {"d", 8},
	DataGFloating: {"g", 8},
	DataHFloating: {"h", 16},
}

// Letter returns the manual's one-letter name for t ("l" for a longword,
// "g" for G_floating), or "" for DataNone.
func (t DataType) Letter() string { return dataTypeInfo[t].letter }

// Size returns the size of a t in bytes (0 for DataNone).
func (t DataType) Size() int { return dataTypeInfo[t].size }

// IsFloat reports whether t is one of the four floating formats.
func (t DataType) IsFloat() bool {
	return t >= DataFFloating && t <= DataHFloating
}

// String returns the data type's name, so fmt's %v prints it readably (Go's
// fmt package calls a String method automatically when a value has one).
func (t DataType) String() string {
	switch t {
	case DataByte:
		return "byte"
	case DataWord:
		return "word"
	case DataLongword:
		return "longword"
	case DataQuadword:
		return "quadword"
	case DataOctaword:
		return "octaword"
	case DataFFloating:
		return "F_floating"
	case DataDFloating:
		return "D_floating"
	case DataGFloating:
		return "G_floating"
	case DataHFloating:
		return "H_floating"
	default:
		return "none"
	}
}
