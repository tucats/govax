package dbgsym

import (
	"errors"
	"fmt"
	"sort"

	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vmsimage"
)

// ErrNoDST is Read's error for an image with no debug symbol table: one
// linked /NOTRACEBACK.
var ErrNoDST = errors.New("the image has no debug symbol table")

// Read reads the debug symbol table of the image img decodes, whose file
// is data, adding base to every address: the address the image was
// loaded at, which is 0 for a main image (whose link-time addresses are
// its run-time ones).
func Read(img *vmsimage.Image, data []byte, base uint32) (*Program, error) {
	if img.DSTVBN == 0 {
		return nil, ErrNoDST
	}

	dst := vmsimage.Blocks(data, img.DSTVBN, img.DSTBlockCount())
	if dst == nil {
		return nil, fmt.Errorf("the debug symbol table (VBN %d, %d blocks) isn't in the file", img.DSTVBN, img.DSTBlockCount())
	}

	p, err := ReadDST(dst, base)
	if err != nil {
		return nil, err
	}

	// An image linked /DEBUG has a debug module table and a global symbol
	// table beside the DST; a traceback link has neither.
	if img.DMTVBN != 0 {
		dmt := vmsimage.Blocks(data, img.DMTVBN, (img.DMTBytes+vmsimage.BlockSize-1)/vmsimage.BlockSize)
		if dmt == nil {
			return nil, fmt.Errorf("the debug module table (VBN %d, %d bytes) isn't in the file", img.DMTVBN, img.DMTBytes)
		}

		if err := p.applyDMT(dmt[:img.DMTBytes], base); err != nil {
			return nil, err
		}
	}

	if img.GSTVBN != 0 {
		if err := p.readGST(img, data, base); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// ReadDST reads a debug symbol table's records, adding base to every
// address.
func ReadDST(dst []byte, base uint32) (*Program, error) {
	recs, err := readRecords(dst)
	if err != nil {
		return nil, err
	}

	b := &builder{base: base, prog: &Program{Skipped: map[byte]int{}}}

	for _, r := range recs {
		if err := b.record(r); err != nil {
			return nil, fmt.Errorf("DST record at %#x (type %d): %w", r.offset, r.typ, err)
		}
	}

	if b.mod != nil {
		if err := b.finishModule(); err != nil {
			return nil, err
		}
	}

	return b.prog, nil
}

// builder gathers one module's records until its module end.
type builder struct {
	base uint32
	prog *Program
	mod  *Module

	// labels and entries are held until the module ends, when the
	// routines' extents, and so the routines the labels are in, are
	// known.
	labels  []symtab.Symbol
	entries []symtab.Symbol
}

func (b *builder) record(r record) error {
	if b.mod == nil && r.typ != typeModuleBegin {
		// Records outside a module: fixups after the last module end
		// (section 18), which MACRO's images don't have.
		b.prog.Skipped[r.typ]++

		return nil
	}

	switch {
	case r.typ == typeModuleBegin:
		return b.moduleBegin(r)

	case r.typ == typeModuleEnd:
		b.mod.DSTSize = uint32(r.end) - b.mod.DSTOffset

		return b.finishModule()

	case r.typ == typeRoutineBegin:
		return b.routineBegin(r)

	case r.typ == typeRoutineEnd:
		return b.routineEnd(r)

	case r.typ == typePsect:
		return b.psect(r)

	case r.typ == typeLabel:
		_, addr, name, _, err := addrNameRecord(r)
		if err != nil {
			return err
		}

		b.labels = append(b.labels, symtab.Symbol{Name: name, Value: addr + b.base, Flags: symtab.Label})

	case r.typ == typeEntry:
		_, addr, name, _, err := addrNameRecord(r)
		if err != nil {
			return err
		}

		b.entries = append(b.entries, symtab.Symbol{Name: name, Value: addr + b.base, Flags: symtab.Entry})

	case r.typ == typeLabelOrLit:
		return b.labelOrLiteral(r)

	case r.typ == typeLineNumbers:
		b.mod.lineData = append(b.mod.lineData, r.data...)

	case r.typ == typeSource:
		b.mod.sourceData = append(b.mod.sourceData, r.data...)

	case r.typ >= 1 && r.typ <= maxDataType:
		return b.datum(r)

	default:
		b.prog.Skipped[r.typ]++
	}

	return nil
}

// moduleBegin starts a module: a flags byte, the language (a longword),
// and the name.
func (b *builder) moduleBegin(r record) error {
	if b.mod != nil {
		// A module with no module end runs to this one's begin.
		b.mod.DSTSize = uint32(r.offset) - b.mod.DSTOffset

		if err := b.finishModule(); err != nil {
			return err
		}
	}

	lang, err := long(r.data, 1)
	if err != nil {
		return err
	}

	name, _, err := counted(r.data, 5)
	if err != nil {
		return err
	}

	b.mod = &Module{Name: name, Language: lang, Symbols: symtab.New(), DSTOffset: uint32(r.offset)}

	return nil
}

// routineBegin adds a routine. Its flags' top bit (DST$V_RTNBEG_NO_CALL)
// marks one entered by JSB.
func (b *builder) routineBegin(r record) error {
	flags, addr, name, _, err := addrNameRecord(r)
	if err != nil {
		return err
	}

	b.mod.Routines = append(b.mod.Routines, &Routine{Name: name, Address: addr + b.base, NoCall: flags&0x80 != 0})

	return nil
}

// routineEnd gives the last routine's size, for a language that writes
// routine ends (MACRO doesn't).
func (b *builder) routineEnd(r record) error {
	size, err := long(r.data, 1)
	if err != nil {
		return err
	}

	if n := len(b.mod.Routines); n > 0 {
		b.mod.Routines[n-1].Size = size
	}

	return nil
}

// psect adds a psect: a byte, the base address, the name, and the size.
func (b *builder) psect(r record) error {
	_, addr, name, next, err := addrNameRecord(r)
	if err != nil {
		return err
	}

	size, err := long(r.data, next)
	if err != nil {
		return err
	}

	b.mod.Psects = append(b.mod.Psects, Psect{Name: name, Address: addr + b.base, Size: size})

	return nil
}

// labelOrLiteral adds a MACRO symbol that is a label or a constant, as
// its value kind says.
func (b *builder) labelOrLiteral(r record) error {
	flags, value, name, _, err := addrNameRecord(r)
	if err != nil {
		return err
	}

	if flags&valKindMask == valKindAddress {
		b.labels = append(b.labels, symtab.Symbol{Name: name, Value: value + b.base, Flags: symtab.Label})
	} else {
		b.mod.Data = append(b.mod.Data, &Datum{Name: name, Type: dtypeLongword, Kind: Literal, Value: value})
	}

	return nil
}

// dtypeLongword is DSC$K_DTYPE_L.
const dtypeLongword = 8

// datum adds a data symbol: a value-flags byte, a value, the name, and,
// in the descriptor form, a descriptor (sections 7.2 to 7.6).
func (b *builder) datum(r record) error {
	vflags, value, name, _, err := addrNameRecord(r)
	if err != nil {
		return err
	}

	d := &Datum{Name: name, Type: r.typ}

	switch {
	case vflags == vflagsDescriptor:
		// The descriptor is value bytes past the name's count byte,
		// which is r.data[5].
		desc, err := descriptor(r.data, 5+int(value))
		if err != nil {
			return err
		}

		desc.Pointer += b.base
		d.Kind, d.Value, d.Descriptor = Address, desc.Pointer, desc

	case vflags == vflagsNoValue || vflags > 0x7F:
		// A type, not an object, or a form MACRO doesn't write.
		b.prog.Skipped[r.typ]++

		return nil

	case vflags&(vflagIndirect|vflagDisp) != 0:
		// An address computed from a register or through a pointer
		// (a routine's argument, in a high-level language): not a
		// static address.
		b.prog.Skipped[r.typ]++

		return nil

	case vflags&valKindMask == valKindLiteral:
		d.Kind, d.Value = Literal, value

	case vflags&valKindMask == valKindAddress:
		d.Kind, d.Value = Address, value+b.base

	case vflags&valKindMask == valKindDesc:
		d.Kind, d.Value = DescriptorAddress, value+b.base

	default: // valKindReg: in a register
		b.prog.Skipped[r.typ]++

		return nil
	}

	b.mod.Data = append(b.mod.Data, d)

	return nil
}

// descriptor decodes the calling-standard descriptor at b[at:]: a word
// length, a byte data type, a byte class, and a longword pointer; an
// array's (class A) goes on with a scale, digits, flags, and dimension
// count (a byte each), the array's size and its A0 (longwords), then, as
// its flags say, a multiplier per dimension and a lower and upper bound
// per dimension.
func descriptor(b []byte, at int) (*Descriptor, error) {
	if at+8 > len(b) {
		return nil, errShort
	}

	d := &Descriptor{
		Length:  le.Uint16(b[at:]),
		Type:    b[at+2],
		Class:   b[at+3],
		Pointer: le.Uint32(b[at+4:]),
	}

	if d.Class != descClassA {
		return d, nil
	}

	const (
		flagCoefficients = 0x20 // DSC$V_FL_COEFF: the multipliers follow
		flagBounds       = 0x40 // DSC$V_FL_BOUNDS: the bounds follow
	)

	if at+20 > len(b) {
		return nil, errShort
	}

	flags, dims := b[at+10], int(b[at+11])
	next := at + 20

	if flags&flagCoefficients != 0 {
		next += 4 * dims
	}

	if flags&flagBounds != 0 {
		if next+8*dims > len(b) {
			return nil, errShort
		}

		for i := 0; i < dims; i++ {
			d.Bounds = append(d.Bounds, Bound{
				Lower: int32(le.Uint32(b[next+8*i:])),
				Upper: int32(le.Uint32(b[next+8*i+4:])),
			})
		}
	}

	return d, nil
}

// finishModule works out the routines' extents, scopes the labels, puts
// every name in the module's symbol table, and runs the line-number and
// source correlation programs.
func (b *builder) finishModule() error {
	m := b.mod

	sort.SliceStable(m.Routines, func(i, j int) bool { return m.Routines[i].Address < m.Routines[j].Address })

	// The relative SET_PC commands count from the lowest routine
	// address (docs/DEBUG-RECORDS.md 13.3).
	var startPC uint32
	if len(m.Routines) > 0 {
		startPC = m.Routines[0].Address
	}

	var err error

	if m.Lines, err = lineTable(m.lineData, startPC, b.base); err != nil {
		return fmt.Errorf("module %s: %w", m.Name, err)
	}

	if m.Files, m.sources, err = sourceTable(m.sourceData); err != nil {
		return fmt.Errorf("module %s: %w", m.Name, err)
	}

	m.lineData, m.sourceData = nil, nil

	for i, r := range m.Routines {
		if r.Size != 0 {
			continue // from a routine end
		}

		end := uint32(0)

		for _, ps := range m.Psects {
			if r.Address >= ps.Address && r.Address < ps.End() {
				end = ps.End()
			}
		}

		if i+1 < len(m.Routines) && (end == 0 || m.Routines[i+1].Address < end) {
			end = m.Routines[i+1].Address
		}

		if end > r.Address {
			r.Size = end - r.Address
		}
	}

	sort.Slice(m.Data, func(i, j int) bool { return m.Data[i].Name < m.Data[j].Name })

	for _, ps := range m.Psects {
		m.Symbols.Set(symtab.Symbol{Name: ps.Name, Value: ps.Address, Flags: symtab.Psect, Scope: m.Name, Size: ps.Size})
	}

	for _, d := range m.Data {
		flags := symtab.Data
		if d.Kind == Literal {
			flags = symtab.Literal
		}

		m.Symbols.Set(symtab.Symbol{Name: d.Name, Value: d.Value, Flags: flags, Scope: m.Name})
	}

	for _, l := range append(b.labels, b.entries...) {
		l.Scope = m.Name
		if r := m.RoutineAt(l.Value); r != nil {
			l.Scope += `\` + r.Name
		}

		m.Symbols.Set(l)
	}

	for _, r := range m.Routines {
		flags := symtab.Entry
		if r.NoCall {
			flags = symtab.Label
		}

		m.Symbols.Set(symtab.Symbol{Name: r.Name, Value: r.Address, Flags: flags, Scope: m.Name, Size: r.Size})
	}

	b.prog.Modules = append(b.prog.Modules, m)
	b.mod, b.labels, b.entries = nil, nil, nil

	return nil
}
