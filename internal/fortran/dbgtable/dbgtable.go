// Package dbgtable is what a FORTRAN program compiled with /DEBUG tells the debugger
// about its variables (docs/PHASE-85 - FORTRAN compiler usability.md, subtask 10).
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

// The types, as a V line has them. A real's and a complex's second letter is its format.
const (
	Integer = "I"
	Logical = "L"
	Char    = "CH"
	Record  = "REC"
	RealF   = "RF"
	RealD   = "RD"
	RealG   = "RG"
	RealH   = "RH"
	ComplxF = "CF"
	ComplxD = "CD"
	ComplxG = "CG"
)

// PlaceKind says how to find a variable.
type PlaceKind byte

// The kinds of place.
const (
	Static    PlaceKind = 'S' // at the address of Symbol
	Argument  PlaceKind = 'A' // at the address in argument N of the routine's frame
	Through   PlaceKind = 'T' // at the address in the cell Symbol, a routine with ENTRY statements
	Frame     PlaceKind = 'F' // N bytes from the routine's FP: AUTOMATIC, or a RECURSIVE unit's local
	InRegister PlaceKind = 'R' // in register N

	// ThroughFrame is Through in a RECURSIVE unit, whose ENTRY cells are in the frame: at
	// the address in the longword N bytes from the routine's FP.
	ThroughFrame PlaceKind = 'P'
)

// Place is where a variable is. For a CHARACTER variable that's a dummy argument, the
// address found is a descriptor's, which has the length and the text's address.
type Place struct {
	Kind   PlaceKind
	Symbol string
	N      int
}

func (p Place) String() string {
	if p.Kind == Static || p.Kind == Through {
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
}

// Line is the variable's line in the MACRO file.
func (v Var) Line() string {
	bounds := "-"

	if len(v.Bounds) > 0 {
		parts := make([]string, len(v.Bounds))
		for i, b := range v.Bounds {
			parts[i] = b.String()
		}

		bounds = strings.Join(parts, ",")
	}

	descriptor := "-"

	switch {
	case v.DescriptorInFrame:
		descriptor = "F:" + strconv.Itoa(v.DescriptorOffset)
	case v.Descriptor != "":
		descriptor = v.Descriptor
	}

	return fmt.Sprintf("%sV %s %s %s %d %s %s %s", Prefix, v.Unit, v.Name, v.Type, v.Size, v.Place, bounds, descriptor)
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
		}
	}

	return t
}

func (t *Table) unit(name string) *Unit {
	u := t.Units[name]
	if u == nil {
		u = &Unit{Name: name, Vars: map[string]*Var{}, Loops: map[string]int{}}
		t.Units[name] = u
	}

	return u
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

	v := &Var{Unit: f[1], Name: f[2], Type: f[3], Size: size, Place: place}

	if f[6] != "-" {
		for _, text := range strings.Split(f[6], ",") {
			b, ok := parseBound(text)
			if !ok {
				return nil, false
			}

			v.Bounds = append(v.Bounds, b)
		}
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
	case Static, Through:
		p.Symbol = value
	case Argument, Frame, InRegister, ThroughFrame:
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

// Sized reports whether the element size is the type's own: all but CHARACTER, whose
// size may be 0, and a record, whose size is its structure's.
func (v *Var) Sized() bool {
	return v.Type != Char && v.Type != Record
}

// DescriptorPassed reports whether what the place leads to is a descriptor of the
// variable, not the variable: a CHARACTER dummy argument (Phase 78).
func (v *Var) DescriptorPassed() bool {
	return v.Type == Char && (v.Place.Kind == Argument || v.Place.Kind == Through || v.Place.Kind == ThroughFrame)
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
