// Package dbgtable is what a FORTRAN program compiled with /DEBUG tells the debugger
// about its variables (docs/planning/PHASE-85 - FORTRAN compiler usability.md, subtask 10).
//
// The compiler writes MACRO, and MACRO's debugger records know a variable only as a label
// over some directive: not that it's REAL, or that M is 2 by 3, or that a dummy argument
// is reached through AP. VAX FORTRAN's own debugger records would say so, but this compiler
// doesn't write any (Phase 74's Decision 3). So it says so in the MACRO file, as comments
// of a form MACRO ignores and the debugger, which reads that file anyway to show source
// lines, recognizes:
//
//	;DBG$ E SUB85 SUB85
//	;DBG$ V SUB85 A RF 4 A:1 1:? SUB85.$B1
//	;DBG$ V SUB85 I I 4 S:SUB85.I - -
//
// An E line says an entry point is in a unit. A V line has the unit, the name, the type
// (below), the element's size in bytes (a CHARACTER variable's length, 0 if passed), where
// it is (Place), its bounds or -, and the symbol of the descriptor holding the bounds
// not known until the program runs, or -.
//
// A record's type is REC:name, the name of a structure the unit's S line describes, and
// each of its components has a C line (docs/planning/PHASE-110 - Fortran 90 compiler
// usability.md, subtask 7a):
//
//	;DBG$ S GEO POINT 8 D
//	;DBG$ C GEO POINT X 0 RF 4 -
//	;DBG$ C GEO POINT Y 4 RF 4 -
//	;DBG$ V GEO PT REC:POINT 8 S:GEO.PT - -
//
// An S line has the unit, the structure's name, its size, and D for a Fortran 90 derived
// type (whose components are named with %) or R for a DEC record structure (named with
// a period). A C line has the unit, the structure, the component's name, its offset in
// the record, its type (a nested structure's REC:name), its element's size, and its
// bounds or -. A structure with no name of its own, DEC's nested one, is given one
// beginning with $, which no FORTRAN name does.
//
// A Fortran 90 pointer's type, a variable's or a component's, is its target's with ^
// before it, ^RF or ^REC:NODE (subtask 7b), and its place is where the pointer's own
// storage is: a longword holding the target's address, 0 while it isn't associated; for
// a CHARACTER pointer a class S descriptor; and for an array pointer, whose bounds are
// all ?:?, a class NCA descriptor. A pointer dummy's storage is the caller's, at the
// address its argument gives, as any dummy's value is.
//
// A B line says where a break at an entry point stops, after the code that starts the
// entry point (copying descriptors, setting up its arguments): at the label of a line of
// the unit, UNIT.$LINEn.
//
//	;DBG$ B SUB SUB 30
//
// A leaf package: the code generator writes the lines and the debugger reads them, and
// neither should need the other.
package dbgtable

import (
	"fmt"
	"strconv"
	"strings"
)

// Prefix starts each of the table's lines.
const Prefix = ";DBG$ "

// The types, as a V line has them. A real's and a complex's second letter is its format
// (an H complex's is CH16, since CH is CHARACTER's).
const (
	Integer = "I"
	Logical = "L"
	Char    = "CH"
	Record  = "REC" // a record: REC:name in a line, the structure's name in Var.Struct
	RealF   = "RF"
	RealD   = "RD"
	RealG   = "RG"
	RealH   = "RH"
	ComplxF = "CF"
	ComplxD = "CD"
	ComplxG = "CG"
	ComplxH = "CH16" // COMPLEX*32, Fortran 90's COMPLEX(16)
)

// PlaceKind says how to find a variable.
type PlaceKind byte

// The kinds of place.
const (
	Static     PlaceKind = 'S' // at the address of Symbol
	Argument   PlaceKind = 'A' // at the address in argument N of the routine's frame
	Through    PlaceKind = 'T' // at the address in the cell Symbol, a routine with ENTRY statements
	Frame      PlaceKind = 'F' // N bytes from the routine's FP: AUTOMATIC, or a RECURSIVE unit's local
	InRegister PlaceKind = 'R' // in register N

	// ThroughFrame is Through in a RECURSIVE unit, whose ENTRY cells are in the frame: at
	// the address in the longword N bytes from the routine's FP.
	ThroughFrame PlaceKind = 'P'

	// Described is a variable whose own storage is a descriptor at Symbol, which says
	// where its elements are (Fortran 90's dynamic arrays): an allocatable array's
	// class NCA descriptor, an automatic array's class A one, an automatic CHARACTER
	// scalar's class S one, or an assumed-shape dummy's class NCA copy. Its POINTER is
	// the first element's address, 0 for an allocatable array not allocated, and a
	// class NCA one's strides say where the others are. DescribedFrame is the same
	// descriptor N bytes from the routine's FP, in a RECURSIVE unit.
	Described      PlaceKind = 'D'
	DescribedFrame PlaceKind = 'G'
)

// Place is where a variable is. For a CHARACTER variable that's a dummy argument, the
// address found is a descriptor's, which has the length and the text's address.
type Place struct {
	Kind   PlaceKind
	Symbol string
	N      int
}

func (p Place) String() string {
	if p.Kind == Static || p.Kind == Through || p.Kind == Described {
		return string(p.Kind) + ":" + p.Symbol
	}

	return string(p.Kind) + ":" + strconv.Itoa(p.N)
}

// Bound is one dimension. Known says its numbers are good; otherwise the variable's
// descriptor has them. Assumed is an upper bound of *, which nothing has.
type Bound struct {
	Lower, Upper int
	Known        bool
	Assumed      bool
}

func (b Bound) String() string {
	switch {
	case b.Assumed && b.Known:
		return strconv.Itoa(b.Lower) + ":*"
	case b.Assumed:
		return "?:*"
	case b.Known:
		return strconv.Itoa(b.Lower) + ":" + strconv.Itoa(b.Upper)
	}

	return "?:?"
}

// Var is one variable, array, dummy argument, or function value.
type Var struct {
	Unit, Name string
	Type       string
	Size       int
	Place      Place
	Bounds     []Bound

	// Descriptor is the symbol of the class A descriptor with the bounds that
	// aren't known until the program runs, "" for none. In a RECURSIVE unit the
	// descriptor is in the frame: DescriptorInFrame is set, and DescriptorOffset is
	// its displacement from the routine's FP (written "F:n" in the line).
	Descriptor        string
	DescriptorInFrame bool
	DescriptorOffset  int

	// Struct is a record's structure's name (Type is Record), "" for any other, or
	// for a record whose layout the table doesn't give.
	Struct string

	// Pointer says the variable is a pointer: Type, Size, and Struct are its
	// target's, and Place is where the pointer's own storage is.
	Pointer bool
}

// typeText is a type as a line has it: a record's with its structure's name, and a
// pointer's with ^ before its target's.
func typeText(code, structure string, pointer bool) string {
	text := code
	if code == Record && structure != "" {
		text = Record + ":" + structure
	}

	if pointer {
		text = "^" + text
	}

	return text
}

// parseType splits a line's type into its code, a record's structure, and whether it
// is a pointer's.
func parseType(text string) (code, structure string, pointer bool) {
	text, pointer = strings.CutPrefix(text, "^")

	if name, ok := strings.CutPrefix(text, Record+":"); ok {
		return Record, name, pointer
	}

	return text, "", pointer
}

// Structure is a record's layout: a derived type's or a DEC structure's.
type Structure struct {
	Name    string
	Size    int
	Derived bool // a Fortran 90 derived type, its components named with %

	// Components are in the order the lines give them, the order declared.
	Components []*Component
}

// Separator is what comes between a record's name and its component's when the
// debugger shows them: % for a derived type, a period for a DEC record.
func (s *Structure) Separator() string {
	if s.Derived {
		return "%"
	}

	return "."
}

// Component finds a component by name, or nil.
func (s *Structure) Component(name string) *Component {
	for _, c := range s.Components {
		if c.Name == name {
			return c
		}
	}

	return nil
}

// Component is one of a structure's components, at Offset bytes from the record's
// start.
type Component struct {
	Name   string
	Offset int
	Type   string // a code, Record for a nested structure
	Struct string // a nested structure's name
	Size   int    // an element's size
	Bounds []Bound

	// Pointer says the component is a pointer, Type, Struct, and Size its
	// target's: its storage in the record is a longword address, a class S
	// descriptor, or for an array pointer (Bounds all ?:?) a class NCA one.
	Pointer bool
}

// StructureLine is a structure's S line.
func StructureLine(unit string, s *Structure) string {
	kind := "R"
	if s.Derived {
		kind = "D"
	}

	return fmt.Sprintf("%sS %s %s %d %s", Prefix, unit, s.Name, s.Size, kind)
}

// ComponentLine is a component's C line.
func ComponentLine(unit, structure string, c *Component) string {
	return fmt.Sprintf("%sC %s %s %s %d %s %d %s", Prefix, unit, structure, c.Name, c.Offset,
		typeText(c.Type, c.Struct, c.Pointer), c.Size, boundsText(c.Bounds))
}

// BodyLine says that a break at the entry point entry of unit stops at the label of
// the unit's line line.
func BodyLine(unit, entry string, line int) string {
	return fmt.Sprintf("%sB %s %s %d", Prefix, unit, entry, line)
}

// boundsText is a line's bounds, or - for none.
func boundsText(bounds []Bound) string {
	if len(bounds) == 0 {
		return "-"
	}

	parts := make([]string, len(bounds))
	for i, b := range bounds {
		parts[i] = b.String()
	}

	return strings.Join(parts, ",")
}

// parseBounds reads a line's bounds.
func parseBounds(text string) ([]Bound, bool) {
	if text == "-" {
		return nil, true
	}

	var bounds []Bound

	for _, part := range strings.Split(text, ",") {
		b, ok := parseBound(part)
		if !ok {
			return nil, false
		}

		bounds = append(bounds, b)
	}

	return bounds, true
}

// Line is the variable's line in the MACRO file.
func (v Var) Line() string {
	bounds := boundsText(v.Bounds)

	descriptor := "-"

	switch {
	case v.DescriptorInFrame:
		descriptor = "F:" + strconv.Itoa(v.DescriptorOffset)
	case v.Descriptor != "":
		descriptor = v.Descriptor
	}

	return fmt.Sprintf("%sV %s %s %s %d %s %s %s", Prefix, v.Unit, v.Name, typeText(v.Type, v.Struct, v.Pointer), v.Size, v.Place, bounds, descriptor)
}

// EntryLine says the entry point entry is in unit.
func EntryLine(unit, entry string) string {
	return Prefix + "E " + unit + " " + entry
}

// LoopLine says that during a loop, /OPTIMIZE keeps the variable name in a register.
func LoopLine(unit, name string, register int) string {
	return fmt.Sprintf("%sH %s %s %d", Prefix, unit, name, register)
}

// Unit is one program unit's variables, by name.
type Unit struct {
	Name string
	Vars map[string]*Var

	// Loops are the registers that hold a variable during a loop, by the variable's name.
	Loops map[string]int

	// Structures are the layouts of the unit's records, by the structure's name.
	Structures map[string]*Structure

	// Bodies are, by entry point, the line of the unit a break at it stops at.
	Bodies map[string]int
}

// Table is the units a MACRO file describes.
type Table struct {
	Units map[string]*Unit

	// Entries says which unit each entry point is in.
	Entries map[string]*Unit
}

// Parse reads the table from a MACRO file's lines. It's nil if the file has none, which
// is every file but one compiled with /DEBUG. A line that can't be read is left out:
// these are comments, in a file someone may have edited.
func Parse(lines []string) *Table {
	var t *Table

	for _, line := range lines {
		if !strings.HasPrefix(line, Prefix) {
			continue
		}

		if t == nil {
			t = &Table{Units: map[string]*Unit{}, Entries: map[string]*Unit{}}
		}

		f := strings.Fields(line[len(Prefix):])

		switch {
		case len(f) == 3 && f[0] == "E":
			t.Entries[f[2]] = t.unit(f[1])
		case len(f) == 4 && f[0] == "H":
			if n, err := strconv.Atoi(f[3]); err == nil {
				t.unit(f[1]).Loops[f[2]] = n
			}
		case len(f) == 8 && f[0] == "V":
			if v, ok := parseVar(f); ok {
				t.unit(v.Unit).Vars[v.Name] = v
			}
		case len(f) == 5 && f[0] == "S":
			if size, err := strconv.Atoi(f[3]); err == nil {
				s := t.unit(f[1]).structure(f[2])
				s.Size, s.Derived = size, f[4] == "D"
			}
		case len(f) == 4 && f[0] == "B":
			if n, err := strconv.Atoi(f[3]); err == nil {
				t.unit(f[1]).Bodies[f[2]] = n
			}
		case len(f) == 8 && f[0] == "C":
			if c, ok := parseComponent(f); ok {
				s := t.unit(f[1]).structure(f[2])
				s.Components = append(s.Components, c)
			}
		}
	}

	return t
}

func (t *Table) unit(name string) *Unit {
	u := t.Units[name]
	if u == nil {
		u = &Unit{Name: name, Vars: map[string]*Var{}, Loops: map[string]int{}, Structures: map[string]*Structure{}, Bodies: map[string]int{}}
		t.Units[name] = u
	}

	return u
}

// structure is the unit's structure of that name, made empty at its first mention: a
// C line may come before its S line.
func (u *Unit) structure(name string) *Structure {
	s := u.Structures[name]
	if s == nil {
		s = &Structure{Name: name}
		u.Structures[name] = s
	}

	return s
}

func parseComponent(f []string) (*Component, bool) {
	offset, err := strconv.Atoi(f[4])
	if err != nil {
		return nil, false
	}

	size, err := strconv.Atoi(f[6])
	if err != nil {
		return nil, false
	}

	bounds, ok := parseBounds(f[7])
	if !ok {
		return nil, false
	}

	code, structure, pointer := parseType(f[5])

	return &Component{Name: f[3], Offset: offset, Type: code, Struct: structure, Size: size, Bounds: bounds, Pointer: pointer}, true
}

func parseVar(f []string) (*Var, bool) {
	size, err := strconv.Atoi(f[4])
	if err != nil {
		return nil, false
	}

	place, ok := parsePlace(f[5])
	if !ok {
		return nil, false
	}

	code, structure, pointer := parseType(f[3])
	v := &Var{Unit: f[1], Name: f[2], Type: code, Struct: structure, Size: size, Place: place, Pointer: pointer}

	if v.Bounds, ok = parseBounds(f[6]); !ok {
		return nil, false
	}

	if offset, inFrame := strings.CutPrefix(f[7], "F:"); inFrame {
		n, err := strconv.Atoi(offset)
		if err != nil {
			return nil, false
		}

		v.DescriptorInFrame, v.DescriptorOffset = true, n
	} else if f[7] != "-" {
		v.Descriptor = f[7]
	}

	return v, true
}

func parsePlace(text string) (Place, bool) {
	kind, value, ok := strings.Cut(text, ":")
	if !ok || len(kind) != 1 {
		return Place{}, false
	}

	p := Place{Kind: PlaceKind(kind[0])}

	switch p.Kind {
	case Static, Through, Described:
		p.Symbol = value
	case Argument, Frame, InRegister, ThroughFrame, DescribedFrame:
		n, err := strconv.Atoi(value)
		if err != nil {
			return Place{}, false
		}

		p.N = n
	default:
		return Place{}, false
	}

	return p, true
}

func parseBound(text string) (Bound, bool) {
	low, high, ok := strings.Cut(text, ":")
	if !ok {
		return Bound{}, false
	}

	b := Bound{Assumed: high == "*"}

	if low == "?" {
		return b, high == "?" || b.Assumed
	}

	lower, err := strconv.Atoi(low)
	if err != nil {
		return Bound{}, false
	}

	b.Lower, b.Known = lower, true

	switch high {
	case "*":
		return b, true
	case "?":
		b.Known = false

		return b, true
	}

	upper, err := strconv.Atoi(high)
	if err != nil {
		return Bound{}, false
	}

	b.Upper = upper

	return b, true
}

// DescriptorPassed reports whether what the place leads to is a descriptor of the
// variable, not the variable: a CHARACTER dummy argument (Phase 78).
func (v *Var) DescriptorPassed() bool {
	return v.Type == Char && !v.Pointer && (v.Place.Kind == Argument || v.Place.Kind == Through || v.Place.Kind == ThroughFrame)
}

// FortranLinePrefix is how long the part before a source line's text is in a MACRO file.
const FortranLinePrefix = 8

// FortranLine reports whether a line of a MACRO file is a line of the FORTRAN source, as
// the code generator writes them (Writer.SourceLineAt): a semicolon, an include depth if
// any, and a number, then two blanks. It says which number.
func FortranLine(record string) (number int, ok bool) {
	if !strings.HasPrefix(record, ";") || len(record) < 8 {
		return 0, false
	}

	field := strings.TrimSpace(record[1:6])
	if record[6] != ' ' || record[7] != ' ' {
		return 0, false
	}

	// An included line's depth comes first, with blanks between.
	if i := strings.LastIndexByte(field, ' '); i >= 0 {
		field = field[i+1:]
	}

	n, err := strconv.Atoi(field)

	return n, err == nil
}
