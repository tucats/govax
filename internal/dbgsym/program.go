package dbgsym

import (
	"sort"
	"strings"

	"github.com/tucats/govax/internal/symtab"
)

// Program is an image's debug symbols: each module the DST describes, in
// the DST's order.
type Program struct {
	Modules []*Module

	// Globals are the global symbol table's symbols (an image linked
	// /DEBUG has one; see gst.go): every global the link defined, by
	// value, without types or scopes. The debugger falls back on them
	// where no module's symbols name an address. Empty without a GST.
	Globals *symtab.Table

	// Skipped counts the records of each type the reader didn't
	// interpret: none, in the images MACRO and LINK write.
	Skipped map[byte]int
}

// Module is one module's debug symbols.
type Module struct {
	Name     string
	Language uint32 // DST$L_MODBEG_LANGUAGE (MACRO is 0)

	// DSTOffset and DSTSize are where the module's records are in the
	// DST: the offset of its module begin record, and the size of its
	// records through its module end. The debug module table gives the
	// same two numbers for each module (dmt.go).
	DSTOffset, DSTSize uint32

	// Psects are the module's contributions to its psects (DST$K_PSECT
	// records), in the DST's order.
	Psects []Psect

	// Ranges are the module's psect contributions as the debug module
	// table gives them (dmt.go): the same address ranges as Psects, in
	// the same order in every fixture, but without names. Empty for an
	// image with no DMT (a traceback link).
	Ranges []Range

	// Routines are the module's routines, sorted by address, each with
	// its extent worked out (see Read).
	Routines []*Routine

	// Data are the module's data symbols and constants, sorted by name.
	Data []*Datum

	// Symbols holds every name the module defines (routines, labels,
	// data, constants, psects, and secondary entry points) for lookups
	// by name and by address. Each symbol's Scope is its path less its
	// name: "DBGDIS" or "DBGDIS\START".
	Symbols *symtab.Table

	// Lines is the module's line-number table, sorted by address: which
	// listing line each stretch of code comes from (empty for a
	// traceback link).
	Lines []Line

	// Files are the source files the module's source correlation
	// records declare, and sources maps listing lines to their records.
	Files   []SourceFile
	sources []sourceRange

	// lineData is the module's line-number program, its line-number
	// records joined (section 13.1), and sourceData its source
	// correlation commands, gathered until the module ends.
	lineData   []byte
	sourceData []byte
}

// Psect is a module's contribution to a program section.
type Psect struct {
	Name    string
	Address uint32
	Size    uint32
}

// End is the address just past the psect.
func (p Psect) End() uint32 { return p.Address + p.Size }

// Range is a stretch of addresses: a psect contribution as the debug
// module table gives it.
type Range struct {
	Address uint32
	Size    uint32
}

// Contains reports whether addr is in the range.
func (r Range) Contains(addr uint32) bool {
	return addr >= r.Address && addr-r.Address < r.Size
}

// Routine is a routine: a CALLS/CALLG entry point, or, with NoCall, a
// JSB one.
type Routine struct {
	Name    string
	Address uint32
	// Size is the routine's length in bytes. MACRO writes no routine end
	// records (docs/DEBUG-RECORDS.md 5.3), so a MACRO routine runs to the
	// next routine in its psect, or the psect's end, as the VMS debugger
	// shows (SHOW SYMBOL/ADDRESS gives START the size 0x132, the distance
	// to LOCALR).
	Size   uint32
	NoCall bool
}

// Contains reports whether addr is in the routine.
func (r *Routine) Contains(addr uint32) bool {
	return addr >= r.Address && addr-r.Address < r.Size
}

// Datum is a data symbol or a constant.
type Datum struct {
	Name string
	// Type is the DSC$K_DTYPE code of the data (8 for a longword, 14
	// for text): the record's type.
	Type byte
	// Kind is how Value is to be read: a constant (Literal), the data's
	// address (Address), or the address of a descriptor of the data
	// (DescriptorAddress, as .ASCID's label is).
	Kind ValueKind
	// Value is the constant, or the address Kind says.
	Value uint32
	// Descriptor is the descriptor the record holds, for a datum MACRO
	// describes by one (.ASCII's string, a list's array); Value is then
	// the data's address, from the descriptor's pointer.
	Descriptor *Descriptor
}

// ValueKind says what a Datum's Value is.
type ValueKind int

const (
	Literal ValueKind = iota
	Address
	DescriptorAddress
)

// Descriptor is a VAX calling-standard descriptor embedded in a data
// record: its class (1 for a string, 4 for an array), its element data
// type and length, and, for an array, the bounds of each dimension.
type Descriptor struct {
	Class   byte
	Type    byte
	Length  uint16
	Pointer uint32
	Bounds  []Bound
}

// Bound is one dimension's lower and upper bound.
type Bound struct {
	Lower, Upper int32
}

// IsString reports whether the descriptor is a string's (class S).
func (d *Descriptor) IsString() bool { return d.Class == descClassS }

// IsArray reports whether the descriptor is an array's (class A).
func (d *Descriptor) IsArray() bool { return d.Class == descClassA }

// Descriptor classes (the calling standard's DSC$K_CLASS_S and _A).
const (
	descClassS = 1
	descClassA = 4
)

// ModuleNamed returns the module named name (ignoring case).
func (p *Program) ModuleNamed(name string) (*Module, bool) {
	for _, m := range p.Modules {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}

	return nil, false
}

// ModuleAt returns the module one of whose psects holds addr: by the
// debug module table's ranges where the image has one, as the debugger
// finds a module without reading its DST records, or else by the DST's
// PSECT records.
func (p *Program) ModuleAt(addr uint32) (*Module, bool) {
	for _, m := range p.Modules {
		if len(m.Ranges) > 0 {
			for _, r := range m.Ranges {
				if r.Contains(addr) {
					return m, true
				}
			}

			continue
		}

		for _, ps := range m.Psects {
			if addr >= ps.Address && addr < ps.End() {
				return m, true
			}
		}
	}

	return nil, false
}

// RoutineAt returns the routine that holds addr, and its module.
func (p *Program) RoutineAt(addr uint32) (*Routine, *Module, bool) {
	for _, m := range p.Modules {
		if r := m.RoutineAt(addr); r != nil {
			return r, m, true
		}
	}

	return nil, nil, false
}

// RoutineAt returns the module's routine that holds addr, or nil.
func (m *Module) RoutineAt(addr uint32) *Routine {
	// The last routine starting at or below addr is the only candidate.
	i := sort.Search(len(m.Routines), func(i int) bool { return m.Routines[i].Address > addr })
	if i > 0 && m.Routines[i-1].Contains(addr) {
		return m.Routines[i-1]
	}

	return nil
}

// Lookup finds a symbol by the debugger's path name: NAME (searched for
// in each module, in order), MODULE\NAME, or MODULE\ROUTINE\NAME. A label
// is in the scope of the routine that holds it, so DBGSUB\SUB2\SUBEND
// names it and DBGSUB\SUBEND doesn't, as with the VMS debugger. A routine
// named as its module is written once (FORTH\F_ABS for a label in routine
// FORTH of module FORTH), as Path writes it; the full path works too.
func (p *Program) Lookup(path string) (*symtab.Symbol, bool) {
	parts := strings.Split(path, `\`)

	if len(parts) == 1 {
		for _, m := range p.Modules {
			if s, ok := m.Symbols.Get(parts[0]); ok {
				return s, true
			}
		}

		return nil, false
	}

	m, ok := p.ModuleNamed(parts[0])
	if !ok {
		return nil, false
	}

	s, ok := m.Symbols.Get(parts[len(parts)-1])
	if !ok {
		return nil, false
	}

	scope := strings.Join(parts[:len(parts)-1], `\`)
	if !strings.EqualFold(s.Scope, scope) && !strings.EqualFold(DisplayScope(s.Scope), scope) {
		return nil, false
	}

	return s, true
}

// Path is a symbol's path name as the debugger writes it: its scope (see
// DisplayScope) and its name.
func Path(s *symtab.Symbol) string {
	if s.Scope == "" {
		return s.Name
	}

	return DisplayScope(s.Scope) + `\` + s.Name
}

// DisplayScope is a scope as the debugger writes it: a routine named as
// its module isn't repeated, so FORTH\FORTH (routine FORTH in module
// FORTH) is FORTH, as in the VMS debugger's FORTH\F_ABS, FORTH\%LINE 332,
// and FAILMAIN+0A (docs/PHASE-41.md, subtask 1's results).
func DisplayScope(scope string) string {
	parts := strings.Split(scope, `\`)
	if len(parts) >= 2 && strings.EqualFold(parts[0], parts[1]) {
		parts = append(parts[:1], parts[2:]...)
	}

	return strings.Join(parts, `\`)
}
