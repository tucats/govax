package dbgsym

import (
	"sort"
	"strings"

	"github.com/tucats/govax/internal/symtab"
)

// This file is what the debugger's data commands (EXAMINE of a data
// location, EVALUATE, SYMBOLIZE; docs/PHASE-42.md, subtask 10) ask of the
// debug symbol table: which datum is at an address, how big its type is,
// and the names the debugger gives an address.
//
// A "datum" here is a data symbol the program defined with a data
// directive after a label (WATCHL: .LONG 0). MACRO records the directive's
// data type in the debug symbol table (a longword, a byte, text), and, for
// an array or a string, a descriptor: the VAX calling standard's small
// record that says how long the data is.

// The calling standard's data type codes (DSC$K_DTYPE_*) a datum's Type
// holds.
const (
	dtypeBU = 2  // unsigned byte
	dtypeWU = 3  // unsigned word
	dtypeLU = 4  // unsigned longword
	dtypeQU = 5  // unsigned quadword
	dtypeB  = 6  // signed byte
	dtypeW  = 7  // signed word
	dtypeL  = 8  // signed longword
	dtypeQ  = 9  // signed quadword
	dtypeF  = 10 // F_floating
	dtypeD  = 11 // D_floating
	dtypeT  = 14 // text
	dtypeOU = 25 // unsigned octaword
	dtypeO  = 26 // signed octaword
	dtypeG  = 27 // G_floating
	dtypeH  = 28 // H_floating
)

// TypeSize is the size in bytes of one item of the data type code t, or 0
// where the type has no fixed size (text, whose length is the descriptor's)
// or isn't one the debugger shows as a number.
func TypeSize(t byte) uint32 {
	switch t {
	case dtypeBU, dtypeB:
		return 1
	case dtypeWU, dtypeW:
		return 2
	case dtypeLU, dtypeL, dtypeF:
		return 4
	case dtypeQU, dtypeQ, dtypeD, dtypeG:
		return 8
	case dtypeOU, dtypeO, dtypeH:
		return 16
	}

	return 0
}

// IsText reports whether the datum's data is text: a string described by a
// string descriptor (.ASCII, or .ASCID's, which the label names by the
// address of a descriptor).
func (d *Datum) IsText() bool {
	return d.Type == dtypeT || (d.Descriptor != nil && d.Descriptor.IsString())
}

// IsArray reports whether the datum is an array (a data directive with a
// list or a block, which MACRO describes with an array descriptor).
func (d *Datum) IsArray() bool { return d.isArray() }

// ElementSize is the size in bytes of the datum's type, or of one element
// of an array. 0 for text.
func (d *Datum) ElementSize() uint32 {
	if d.isArray() && d.Descriptor.Length > 0 {
		return uint32(d.Descriptor.Length)
	}

	return TypeSize(d.Type)
}

// DatumPath finds a data symbol by its path name (NAME, MODULE\NAME) and
// returns its record. ok is false for a name that isn't a data symbol with
// an address (a routine, a label, a constant).
func (p *Program) DatumPath(path string) (*Datum, bool) {
	if p == nil {
		return nil, false
	}

	s, ok := p.Lookup(path)
	if !ok {
		return nil, false
	}

	module, _, _ := strings.Cut(s.Scope, `\`)

	m, ok := p.ModuleNamed(module)
	if !ok {
		return nil, false
	}

	d := m.datumNamed(s.Name)
	if d == nil || d.Kind == Literal || !s.Has(symtab.Data) {
		return nil, false
	}

	return d, true
}

// DatumAt returns the datum whose label is exactly addr, with the module
// that defines it. A datum known by the address of a descriptor (.ASCID)
// counts at that address.
func (p *Program) DatumAt(addr uint32) (*Datum, *Module, bool) {
	if p == nil {
		return nil, nil, false
	}

	if m, ok := p.ModuleAt(addr); ok {
		if d := m.datumAt(addr); d != nil {
			return d, m, true
		}
	}

	return nil, nil, false
}

// datumAt is DatumAt within the module.
func (m *Module) datumAt(addr uint32) *Datum {
	for _, d := range m.Data {
		if d.Kind != Literal && d.Value == addr {
			return d
		}
	}

	return nil
}

// ElementDatumAt returns the array datum one of whose elements is at addr
// (a whole element in, not necessarily the first), so that the debugger
// types the location by the array's element type: BUFFER[2] is a byte,
// not a longword.
func (p *Program) ElementDatumAt(addr uint32) (*Datum, bool) {
	if p == nil {
		return nil, false
	}

	m, ok := p.ModuleAt(addr)
	if !ok {
		return nil, false
	}

	for _, d := range m.Data {
		if !d.isArray() || len(d.Descriptor.Bounds) != 1 || d.Descriptor.Length == 0 || addr < d.Value {
			continue
		}

		size := uint32(d.Descriptor.Length)
		b := d.Descriptor.Bounds[0]
		off := addr - d.Value

		if off%size == 0 && b.Upper >= b.Lower && off/size <= uint32(b.Upper-b.Lower) {
			return d, true
		}
	}

	return nil, false
}

// DataName names a data location as the debugger's EXAMINE labels it, in
// radix for offsets and subscripts: the data symbol's path name
// (DBGCMD\WATCHL), a subscripted element of an array (DBGCMD\BUFFER[4]),
// or, for an address inside a datum or past one, the nearest data symbol
// before it in the same program section and the distance
// (DBGCMD\WATCHL+3). ok is false for an address in no module's data.
//
// An array's own address is named by its first subscript, as an
// instruction's operand is (BUFFER[0]), except by a caller that wants the
// array as a whole.
func (p *Program) DataName(addr uint32, radix int) (string, bool) {
	if p == nil {
		return "", false
	}

	m, ok := p.ModuleAt(addr)
	if !ok {
		return "", false
	}

	if d := m.datumAt(addr); d != nil {
		if s, found := m.Symbols.Get(d.Name); found {
			name := m.path(s)
			if d.isArray() && len(d.Descriptor.Bounds) == 1 {
				name += "[" + subscript(d.Descriptor.Bounds[0].Lower, radix) + "]"
			}

			return name, true
		}
	}

	if name, ok := m.element(addr, radix); ok {
		return name, true
	}

	// The nearest datum before addr. (Not bounded by the program section:
	// SYMBOLIZE names the NOLABEL section's first bytes by the datum
	// before it in another section, DBGDIS\TEXT+1A, in the probe's log.)
	var best *Datum

	for _, d := range m.Data {
		if d.Kind == Literal || d.Value > addr {
			continue
		}

		if best == nil || d.Value > best.Value {
			best = d
		}
	}

	if best == nil {
		return "", false
	}

	s, found := m.Symbols.Get(best.Name)
	if !found {
		return "", false
	}

	return m.path(s) + offset(addr-best.Value, radix), true
}

// SymbolizeNames returns every name the debugger's SYMBOLIZE gives addr,
// from the modules' symbols, and the name the global symbol table gives
// it. The module names are, in order: the symbol or the nearest symbol
// before addr (a routine's code is named by the routine and an offset, a
// data location by the nearest data symbol and an offset), and, where the
// address is in a line's code, the line (DBGDIS\START\%LINE 42, or
// DBGDIS\%LINE 41 for the code a CALLS routine's entry mask makes line
// 41 to). global is "" for an image with no global symbol table
// (linked without /DEBUG) or no global at or before addr.
//
// SYMBOLIZE names data with a plain offset (TABLE+8), where EXAMINE
// names an element (TABLE[2]); see DataName.
func (p *Program) SymbolizeNames(addr uint32, radix int) (names []string, global string) {
	if p == nil {
		return nil, ""
	}

	if m, ok := p.ModuleAt(addr); ok {
		if name, ok := m.symbolizeName(addr, radix); ok {
			names = append(names, name)
		}

		if name, ok := m.symbolizeLine(addr, radix); ok {
			names = append(names, name)
		}
	}

	if p.Globals != nil {
		if s, off, ok := p.Globals.Nearest(addr, nil); ok {
			global = s.Name + offset(off, radix)
		}
	}

	return names, global
}

// symbolizeName is SymbolizeNames's first module name for addr.
func (m *Module) symbolizeName(addr uint32, radix int) (string, bool) {
	if s, ok := m.symbolAt(addr); ok {
		return m.path(s), true
	}

	if r := m.RoutineAt(addr); r != nil {
		return m.routinePath(r) + offset(addr-r.Address, radix), true
	}

	// Data: the nearest data symbol at or before addr, string data
	// included (TEXT+10 is in .ASCII's TEXT), whatever program section it
	// is in.
	datums := make([]*Datum, 0, len(m.Data))

	for _, d := range m.Data {
		if d.Kind != Literal && d.Value <= addr {
			datums = append(datums, d)
		}
	}

	if len(datums) == 0 {
		return "", false
	}

	sort.Slice(datums, func(i, j int) bool { return datums[i].Value > datums[j].Value })

	s, found := m.Symbols.Get(datums[0].Name)
	if !found {
		return "", false
	}

	return m.path(s) + offset(addr-datums[0].Value, radix), true
}

// symbolizeLine is the line name SYMBOLIZE gives addr. The first two
// bytes of a CALLS routine are its entry mask, data that the line table
// still gives a line to; the debugger puts that line in the module's scope
// (DBGDIS\%LINE 41), not the routine's.
func (m *Module) symbolizeLine(addr uint32, radix int) (string, bool) {
	l, ok := m.LineAt(addr)
	if !ok {
		return "", false
	}

	scope := m.scope(m.RoutineAt(addr))

	if r := m.RoutineAt(addr); r != nil && !r.NoCall && addr < r.Address+2 {
		scope = m.Name
	}

	return scope + `\%LINE ` + number(int64(l.Line), 10) + offset(addr-l.Address, radix), true
}
