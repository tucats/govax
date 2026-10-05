package dbgsym

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/symtab"
)

// Symbolize names addr as the VMS debugger names the address an
// instruction's operand refers to, and the location an instruction is at
// (docs/PHASE-41.md, subtask 1's results). From the most specific name to
// the least:
//
//  1. A routine, label, or data symbol at addr, in the module whose
//     psects hold it: its path name, the module always given, even for
//     the module the instruction is in (L^DBGDIS\COUNT, BSBW
//     DBGDIS\LOCALR\JSBRTN). A routine named as its module is just its
//     name (FORTH). A routine's entry outranks a label, and a label
//     outranks data. Psect names and constants never name an address,
//     and neither does data described by a string descriptor (.ASCII,
//     .ASCID): the debugger passes over them to the globals (TEXT+10 is
//     L^GLIMIT+24D, MSG is L^GLIMIT+22F).
//  2. An element of an array (a .LONG or .BYTE list with a descriptor):
//     DBGDIS\TABLE[2] for TABLE+8; the first element when addr is the
//     array's own (OPSTK_END[0]).
//  3. A line's code: DBGDIS\START\%LINE 42 at its first byte,
//     FORTH\%LINE 332+6 past it. The scope is the routine holding the
//     line, as for a label.
//  4. A routine's code with no line table (a traceback link):
//     DBGDIS\START+2, TRACE+10.
//  5. The global symbol table: the global with the greatest value at or
//     below addr, constants included, as NAME+offset (GLIMIT+24D,
//     SUB2+0F0), or the name alone (SYS$OPEN).
//
// Offsets are in radix (16, the default for MACRO, or 10): hexadecimal
// with a leading 0 when the first digit is a letter (+0C, +24D), as the
// debugger writes them. Line numbers are always decimal.
//
// ok is false when nothing names addr, and the debugger shows the number
// (L^00003408, @#00000000).
//
// Unconfirmed (no probe line shows them): an operand's address in the
// middle of a line is named as a location is (%LINE n+offset), an array
// element whose address isn't a whole element from the array's start
// falls to the next rule, and a subscript is written in radix.
func (p *Program) Symbolize(addr uint32, radix int) (string, bool) {
	if p == nil {
		return "", false
	}

	if m, ok := p.ModuleAt(addr); ok {
		if name, ok := m.symbolize(addr, radix); ok {
			return name, true
		}
	}

	if p.Globals != nil {
		if s, off, ok := p.Globals.Nearest(addr, nil); ok {
			return s.Name + offset(off, radix), true
		}
	}

	return "", false
}

// symbolize names addr from the module's own symbols and lines: rules 1
// to 4 of Program.Symbolize.
func (m *Module) symbolize(addr uint32, radix int) (string, bool) {
	if s, ok := m.symbolAt(addr); ok {
		name := m.path(s)

		if d := m.datumNamed(s.Name); d != nil && d.isArray() {
			name += "[" + subscript(d.Descriptor.Bounds[0].Lower, radix) + "]"
		}

		return name, true
	}

	if name, ok := m.element(addr, radix); ok {
		return name, true
	}

	if name, ok := m.lineName(addr, radix); ok {
		return name, true
	}

	if r := m.RoutineAt(addr); r != nil {
		return m.routinePath(r) + offset(addr-r.Address, radix), true
	}

	return "", false
}

// LineName names addr by the line whose code holds it, as rule 3 of
// Symbolize does (DBGDIS\START\%LINE 85), even where a label or routine
// would name it first: the debugger names the first location of an
// EXAMINE range typed as %LINE n so (EXAMINE/INSTRUCTION %LINE 85 shows
// DBGDIS\START\%LINE 85:, where a range from START shows the same
// instruction at DBGDIS\START\LOOP:).
func (p *Program) LineName(addr uint32, radix int) (string, bool) {
	if p == nil {
		return "", false
	}

	if m, ok := p.ModuleAt(addr); ok {
		return m.lineName(addr, radix)
	}

	return "", false
}

// lineName is LineName within the module.
func (m *Module) lineName(addr uint32, radix int) (string, bool) {
	l, ok := m.LineAt(addr)
	if !ok {
		return "", false
	}

	return m.scope(m.RoutineAt(addr)) + `\%LINE ` + strconv.Itoa(l.Line) + offset(addr-l.Address, radix), true
}

// symbolAt returns the module's symbol that names addr exactly: a CALL
// routine's entry, else a label (or a JSB routine), else data that isn't
// described by a string descriptor.
func (m *Module) symbolAt(addr uint32) (*symtab.Symbol, bool) {
	if s, ok := m.Symbols.At(addr, func(s *symtab.Symbol) bool { return s.Has(symtab.Entry) }); ok {
		return s, true
	}

	if s, ok := m.Symbols.At(addr, func(s *symtab.Symbol) bool { return s.Has(symtab.Label) }); ok {
		return s, true
	}

	return m.Symbols.At(addr, func(s *symtab.Symbol) bool {
		if !s.Has(symtab.Data) {
			return false
		}

		d := m.datumNamed(s.Name)

		return d != nil && d.Kind == Address && (d.Descriptor == nil || d.isArray())
	})
}

// element names addr as an element of one of the module's arrays:
// NAME[i] when addr is a whole element past the array's start and within
// its bounds. Only one-dimensional arrays are named so (MACRO writes no
// others).
func (m *Module) element(addr uint32, radix int) (string, bool) {
	for _, d := range m.Data {
		if !d.isArray() || len(d.Descriptor.Bounds) != 1 || d.Descriptor.Length == 0 || addr < d.Value {
			continue
		}

		size := uint32(d.Descriptor.Length)
		b := d.Descriptor.Bounds[0]
		off := addr - d.Value

		if off%size != 0 || b.Upper < b.Lower || off/size > uint32(b.Upper-b.Lower) {
			continue
		}

		if s, ok := m.Symbols.Get(d.Name); ok {
			return m.path(s) + "[" + subscript(b.Lower+int32(off/size), radix) + "]", true
		}
	}

	return "", false
}

// datumNamed returns the module's data symbol or constant named name, or
// nil.
func (m *Module) datumNamed(name string) *Datum {
	i := sort.Search(len(m.Data), func(i int) bool { return m.Data[i].Name >= name })
	if i < len(m.Data) && strings.EqualFold(m.Data[i].Name, name) {
		return m.Data[i]
	}

	// Data is sorted by name as the DST spells it; look the slow way in
	// case the caller spelled it otherwise.
	for _, d := range m.Data {
		if strings.EqualFold(d.Name, name) {
			return d
		}
	}

	return nil
}

// isArray reports whether d is described by an array descriptor.
func (d *Datum) isArray() bool {
	return d.Kind == Address && d.Descriptor != nil && d.Descriptor.IsArray() && len(d.Descriptor.Bounds) > 0
}

// path is a symbol's path name, with the debugger's one exception to
// Path: a routine named as its module is just its name (FORTH, FAILMAIN,
// TRACE), not FORTH\FORTH.
func (m *Module) path(s *symtab.Symbol) string {
	if strings.EqualFold(s.Name, m.Name) && strings.EqualFold(s.Scope, m.Name) {
		return s.Name
	}

	return Path(s)
}

// routinePath is routine r's path name: MODULE\ROUTINE, or the routine's
// name alone when it's named as its module.
func (m *Module) routinePath(r *Routine) string {
	if strings.EqualFold(r.Name, m.Name) {
		return r.Name
	}

	return m.Name + `\` + r.Name
}

// scope is the path a line in routine r (nil for none) is written under:
// the module, and the routine unless it's named as its module.
func (m *Module) scope(r *Routine) string {
	if r == nil {
		return m.Name
	}

	return DisplayScope(m.Name + `\` + r.Name)
}

// offset is "+n" for a nonzero offset in radix, as the debugger writes
// it (see Symbolize), and "" for zero.
func offset(n uint32, radix int) string {
	if n == 0 {
		return ""
	}

	return "+" + number(int64(n), radix)
}

// subscript is an array subscript in radix.
func subscript(n int32, radix int) string {
	return number(int64(n), radix)
}

// number writes n in radix 10 or 16 (any other means 16); hexadecimal
// gets a leading 0 when its first digit is a letter, so it reads as a
// number, not a name.
func number(n int64, radix int) string {
	if radix == 10 {
		return strconv.FormatInt(n, 10)
	}

	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}

	text := strings.ToUpper(strconv.FormatInt(n, 16))
	if text[0] >= 'A' {
		text = "0" + text
	}

	return sign + text
}

// Constant names value as a constant of the module holding pc, when
// exactly one of its constants (LIMIT = 10) has that value: the option
// Decision 5 of docs/PHASE-41.md adds, beyond what the debugger shows. A
// value two constants share names neither, since either could be meant.
func (p *Program) Constant(pc, value uint32) (string, bool) {
	if p == nil {
		return "", false
	}

	m, ok := p.ModuleAt(pc)
	if !ok {
		return "", false
	}

	var found *Datum

	for _, d := range m.Data {
		if d.Kind != Literal || d.Value != value {
			continue
		}

		if found != nil {
			return "", false
		}

		found = d
	}

	if found == nil {
		return "", false
	}

	return m.Name + `\` + found.Name, true
}

// Names adapts a Program to internal/disasm's Symbolizer: it names the
// addresses an instruction's operands refer to, in Radix (0 means 16).
// It's a disasm.ConstantNamer too, for a caller that asks for constants'
// names.
type Names struct {
	Program *Program
	Radix   int
}

// Symbolize is Program.Symbolize in n's radix.
func (n Names) Symbolize(addr uint32) (string, bool) {
	radix := n.Radix
	if radix == 0 {
		radix = 16
	}

	return n.Program.Symbolize(addr, radix)
}

// Constant is Program.Constant.
func (n Names) Constant(pc, value uint32) (string, bool) {
	return n.Program.Constant(pc, value)
}
