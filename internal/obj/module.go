package obj

import (
	"encoding/binary"
	"fmt"
)

// Module is one object module: its records, in order. Encode(Decode(r))
// reproduces r byte for byte, so a module read from a real .OBJ file can
// be checked, dumped, or written back unchanged.
type Module struct {
	Records []Record
}

// Record is one object record: *MainHeader, *TextHeader, *GSD, *TIR,
// *EOM, *LNK, or *Unknown.
type Record interface {
	RecordType() RecordType
}

// MainHeader is the main module header (HDR/MHD), which must be the first
// record.
type MainHeader struct {
	StructureLevel byte
	// MaxRecordSize is the size of the module's longest record, or
	// usually just OBJ$C_MAXRECSIZ.
	MaxRecordSize uint16
	Name          string
	Version       string
	// Created is the creation time, in the fixed 17-character form
	// "dd-mmm-yyyy hh:mm". Patched is the last patch time in the same
	// form, which the linker ignores; it may be blanks or zero bytes.
	Created string
	Patched string
}

// TextHeader is a header record other than MHD: the language processor
// name (LNM), or text the linker ignores (SRC, TTL, CPR, MTC, GTX, or a
// header type from the reserved or ignored ranges).
type TextHeader struct {
	Type HeaderType
	Text string
}

// GSD is a global symbol directory record: one or more subrecords, each a
// *Psect, *Symbol, *IdentCheck, or *Environment.
type GSD struct {
	Subrecords []Subrecord
}

// Subrecord is one GSD subrecord.
type Subrecord interface {
	GSDType() GSDType
}

// Psect is a program section definition (GSD$C_PSC), or a program section
// definition in a shareable image (GSD$C_SPSC) when Shared is set.
type Psect struct {
	Shared bool
	// Align is the psect's alignment as a power of two: 0 is byte, 2 is
	// longword, 9 is page.
	Align byte
	Flags uint16
	// Alloc is the size of this module's contribution to the psect.
	Alloc uint32
	// Base is the psect's base address in its shareable image (GSD$C_SPSC
	// only).
	Base uint32
	Name string
}

// GSDType reports the subrecord's type.
func (p *Psect) GSDType() GSDType {
	if p.Shared {
		return GSDSharedPsect
	}

	return GSDPsect
}

// Symbol is a symbol subrecord: a global symbol definition or reference
// (SYM), an entry point (EPM), or a procedure (PRO), in any of their
// word-psect (SYMW, EPMW, PROW), vectored (SYMV, EPMV, PROV), version-mask
// (SYMM, EPMM, PROM), or module-local (LSY, LEPM, LPRO) forms. Which fields
// are present depends on Type and, for the plain symbol forms, on whether
// Flags has SymDEF: a reference has only a data type, flags, and a name.
type Symbol struct {
	Type     GSDType
	DataType byte
	Flags    uint16
	Env      uint16 // the environment index (LSY, LEPM, LPRO)
	Psect    uint16 // the index of the psect that defines the symbol
	Value    uint32 // the symbol's value or entry address
	// Extra is the vector value (SYMV, EPMV, PROV) or the version mask
	// (SYMM, EPMM, PROM).
	Extra   uint32
	Mask    uint16   // the entry mask (entry points and procedures)
	Formals *Formals // procedures only
	Name    string
}

// GSDType reports the subrecord's type.
func (s *Symbol) GSDType() GSDType { return s.Type }

// Defined reports whether the subrecord defines its symbol rather than
// refers to it.
func (s *Symbol) Defined() bool {
	return s.Flags&SymDEF != 0 || symbolLayouts[s.Type].alwaysDefines
}

// Formals are a procedure definition's argument counts and formal argument
// descriptors (section 7.3.4). There is one descriptor for each of the
// maximum number of arguments.
type Formals struct {
	Min, Max byte
	// Args holds each descriptor: its validation control byte (the
	// argument passing mechanism, in bits 0 and 1), then its detailed
	// description, which the linker ignores.
	Args []FormalArg
}

// FormalArg is one formal argument descriptor.
type FormalArg struct {
	ValCtl byte
	Detail []byte
}

// IdentCheck is an entity ident consistency check subrecord (GSD$C_IDC).
type IdentCheck struct {
	Flags  uint16
	Name   string // the entity
	Ident  []byte // an ASCII ident, or a binary longword
	Object string
}

// GSDType reports the subrecord's type.
func (*IdentCheck) GSDType() GSDType { return GSDIdentCheck }

// Environment is an environment definition or reference (GSD$C_ENV).
type Environment struct {
	Flags  uint16
	Parent uint16
	Name   string
}

// GSDType reports the subrecord's type.
func (*Environment) GSDType() GSDType { return GSDEnvironment }

// TIR is a text information and relocation record (OBJ$C_TIR), or, with
// Type RecDBG or RecTBT, a debugger or traceback information record: all
// three hold TIR commands.
type TIR struct {
	Type     RecordType
	Commands []Command
}

// EOM is the end of module record (OBJ$C_EOM), or the end of module with
// word psect record (OBJ$C_EOMW) when Word is set.
type EOM struct {
	Word     bool
	Severity byte
	// HasTransfer says whether the module names a transfer address: the
	// program's starting point, which MACRO's ".END label" sets.
	HasTransfer bool
	Psect       uint16
	Transfer    uint32
	// HasFlags says whether the optional transfer flags byte is present.
	HasFlags bool
	Flags    byte
}

// LNK is a link option specification record (OBJ$C_LNK).
type LNK struct {
	Type  byte
	Flags uint16
	Name  string
	// Rest holds what follows the file name: an object library with
	// inclusion list's module names (LNK$C_OLI).
	Rest []byte
}

// Unknown is a record of a type this package doesn't know, kept whole.
type Unknown struct {
	Type RecordType
	Data []byte // everything after the type byte
}

// RecordType reports the record's type.
func (*MainHeader) RecordType() RecordType { return RecHDR }

// RecordType reports the record's type.
func (*TextHeader) RecordType() RecordType { return RecHDR }

// RecordType reports the record's type.
func (*GSD) RecordType() RecordType { return RecGSD }

// RecordType reports the record's type.
func (t *TIR) RecordType() RecordType { return t.Type }

// RecordType reports the record's type.
func (e *EOM) RecordType() RecordType {
	if e.Word {
		return RecEOMW
	}

	return RecEOM
}

// RecordType reports the record's type.
func (*LNK) RecordType() RecordType { return RecLNK }

// RecordType reports the record's type.
func (u *Unknown) RecordType() RecordType { return u.Type }

// symbolLayout says which fields a symbol subrecord type has.
type symbolLayout struct {
	local         bool // an environment index follows the flags (LSY, LEPM, LPRO)
	wordPsect     bool // the psect index is a word
	extra         bool // a vector or version mask longword follows the value
	entry         bool // an entry mask follows
	procedure     bool // formal arguments follow the name
	alwaysDefines bool // there is no reference form
}

var symbolLayouts = map[GSDType]symbolLayout{}

func init() {
	symbolLayouts[GSDSymbol] = symbolLayout{}
	symbolLayouts[GSDEntry] = symbolLayout{entry: true, alwaysDefines: true}
	symbolLayouts[GSDProcedure] = symbolLayout{entry: true, procedure: true, alwaysDefines: true}
	symbolLayouts[GSDSymbolW] = symbolLayout{wordPsect: true, alwaysDefines: true}
	symbolLayouts[GSDEntryW] = symbolLayout{wordPsect: true, entry: true, alwaysDefines: true}
	symbolLayouts[GSDProcedureW] = symbolLayout{wordPsect: true, entry: true, procedure: true, alwaysDefines: true}
	symbolLayouts[GSDSymbolV] = symbolLayout{extra: true}
	symbolLayouts[GSDEntryV] = symbolLayout{extra: true, entry: true, alwaysDefines: true}
	symbolLayouts[GSDProcedureV] = symbolLayout{extra: true, entry: true, procedure: true, alwaysDefines: true}
	symbolLayouts[GSDSymbolM] = symbolLayout{extra: true}
	symbolLayouts[GSDEntryM] = symbolLayout{extra: true, entry: true, alwaysDefines: true}
	symbolLayouts[GSDProcedureM] = symbolLayout{extra: true, entry: true, procedure: true, alwaysDefines: true}
	symbolLayouts[GSDLocalSymbol] = symbolLayout{local: true, wordPsect: true}
	symbolLayouts[GSDLocalEntry] = symbolLayout{local: true, wordPsect: true, entry: true, alwaysDefines: true}
	symbolLayouts[GSDLocalProc] = symbolLayout{local: true, wordPsect: true, entry: true, procedure: true, alwaysDefines: true}
}

// Decode parses a module's records, each given without any file framing:
// the record type byte, then the record's contents.
func Decode(records [][]byte) (*Module, error) {
	m := &Module{Records: make([]Record, 0, len(records))}

	for i, b := range records {
		rec, err := decodeRecord(b)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", i+1, err)
		}

		m.Records = append(m.Records, rec)
	}

	return m, nil
}

// Encode turns a module's records back into bytes, one slice per record.
func Encode(m *Module) ([][]byte, error) {
	out := make([][]byte, 0, len(m.Records))

	for i, rec := range m.Records {
		b, err := EncodeRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("record %d (%s): %w", i+1, rec.RecordType(), err)
		}

		out = append(out, b)
	}

	return out, nil
}

func decodeRecord(b []byte) (Record, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty record")
	}

	r := reader{b: b, pos: 1}

	switch t := RecordType(b[0]); t {
	case RecHDR:
		return decodeHeader(&r)

	case RecGSD:
		g := &GSD{}

		for !r.done() {
			e, err := decodeGSDEntry(&r)
			if err != nil {
				return nil, err
			}

			g.Subrecords = append(g.Subrecords, e)
		}

		if len(g.Subrecords) == 0 {
			return nil, fmt.Errorf("GSD record has no subrecords")
		}

		return g, nil

	case RecTIR, RecDBG, RecTBT:
		rec := &TIR{Type: t}

		for !r.done() {
			c, n, err := decodeCommand(r.b[r.pos:])
			if err != nil {
				return nil, fmt.Errorf("%s at offset %d: %w", t, r.pos, err)
			}

			rec.Commands = append(rec.Commands, c)
			r.pos += n
		}

		return rec, nil

	case RecEOM, RecEOMW:
		e := &EOM{Word: t == RecEOMW, Severity: r.byte()}

		if !r.done() {
			e.HasTransfer = true
			if e.Word {
				e.Psect = r.word()
			} else {
				e.Psect = uint16(r.byte())
			}

			e.Transfer = r.long()
		}

		if !r.done() {
			e.HasFlags, e.Flags = true, r.byte()
		}

		if r.err == nil && !r.done() {
			return nil, fmt.Errorf("%s record has %d extra bytes", t, len(r.b)-r.pos)
		}

		return e, r.err

	case RecLNK:
		l := &LNK{Type: r.byte(), Flags: r.word()}
		l.Name = string(r.bytes(int(r.word())))
		l.Rest = r.bytes(len(r.b) - r.pos)

		return l, r.err
	}

	return &Unknown{Type: RecordType(b[0]), Data: append([]byte(nil), b[1:]...)}, nil
}

func decodeHeader(r *reader) (Record, error) {
	t := HeaderType(r.byte())
	if t != HdrMHD {
		return &TextHeader{Type: t, Text: string(r.bytes(len(r.b) - r.pos))}, r.err
	}

	h := &MainHeader{StructureLevel: r.byte(), MaxRecordSize: r.word()}
	h.Name = r.counted()
	h.Version = r.counted()
	h.Created = string(r.bytes(17))
	h.Patched = string(r.bytes(len(r.b) - r.pos))

	return h, r.err
}

func decodeGSDEntry(r *reader) (Subrecord, error) {
	t := GSDType(r.byte())

	switch t {
	case GSDPsect, GSDSharedPsect:
		p := &Psect{Shared: t == GSDSharedPsect, Align: r.byte(), Flags: r.word(), Alloc: r.long()}
		if p.Shared {
			p.Base = r.long()
		}

		p.Name = r.counted()

		return p, r.err

	case GSDIdentCheck:
		c := &IdentCheck{Flags: r.word()}
		c.Name = r.counted()
		c.Ident = []byte(r.counted())
		c.Object = r.counted()

		return c, r.err

	case GSDEnvironment:
		e := &Environment{Flags: r.word(), Parent: r.word()}
		e.Name = r.counted()

		return e, r.err
	}

	layout, ok := symbolLayouts[t]
	if !ok {
		return nil, fmt.Errorf("unknown GSD subrecord type %d", byte(t))
	}

	s := &Symbol{Type: t, DataType: r.byte(), Flags: r.word()}
	if layout.local {
		s.Env = r.word()
	}

	if s.Defined() {
		if layout.wordPsect {
			s.Psect = r.word()
		} else {
			s.Psect = uint16(r.byte())
		}

		s.Value = r.long()

		if layout.extra {
			s.Extra = r.long()
		}

		if layout.entry {
			s.Mask = r.word()
		}
	}

	s.Name = r.counted()

	if layout.procedure && s.Defined() {
		f := &Formals{Min: r.byte(), Max: r.byte()}

		for range int(f.Max) {
			a := FormalArg{ValCtl: r.byte()}
			a.Detail = r.bytes(int(r.byte()))
			f.Args = append(f.Args, a)
		}

		s.Formals = f
	}

	if r.err != nil {
		return nil, fmt.Errorf("%s subrecord: %w", t, r.err)
	}

	return s, nil
}

// EncodeRecord turns one record into bytes.
func EncodeRecord(rec Record) ([]byte, error) {
	b := []byte{byte(rec.RecordType())}

	var err error

	switch rec := rec.(type) {
	case *MainHeader:
		if len(rec.Created) != 17 {
			return nil, fmt.Errorf("creation time %q is not 17 characters", rec.Created)
		}

		b = binary.LittleEndian.AppendUint16(append(b, byte(HdrMHD), rec.StructureLevel), rec.MaxRecordSize)
		if b, err = appendCounted(b, rec.Name); err != nil {
			return nil, err
		}

		if b, err = appendCounted(b, rec.Version); err != nil {
			return nil, err
		}

		b = append(append(b, rec.Created...), rec.Patched...)

	case *TextHeader:
		if rec.Type == HdrMHD {
			return nil, fmt.Errorf("a text header can't have type MHD")
		}

		b = append(append(b, byte(rec.Type)), rec.Text...)

	case *GSD:
		if len(rec.Subrecords) == 0 {
			return nil, fmt.Errorf("GSD record has no subrecords")
		}

		for _, e := range rec.Subrecords {
			if b, err = encodeGSDEntry(b, e); err != nil {
				return nil, err
			}
		}

	case *TIR:
		if rec.Type != RecTIR && rec.Type != RecDBG && rec.Type != RecTBT {
			return nil, fmt.Errorf("a TIR record can't have type %s", rec.Type)
		}

		for _, c := range rec.Commands {
			if b, err = c.encode(b); err != nil {
				return nil, err
			}
		}

	case *EOM:
		b = append(b, rec.Severity)

		if rec.HasTransfer {
			if rec.Word {
				b = binary.LittleEndian.AppendUint16(b, rec.Psect)
			} else if rec.Psect > 0xFF {
				return nil, fmt.Errorf("psect index %d needs an EOMW record", rec.Psect)
			} else {
				b = append(b, byte(rec.Psect))
			}

			b = binary.LittleEndian.AppendUint32(b, rec.Transfer)
		} else if rec.HasFlags {
			return nil, fmt.Errorf("transfer flags without a transfer address")
		}

		if rec.HasFlags {
			b = append(b, rec.Flags)
		}

	case *LNK:
		if len(rec.Name) > 0xFFFF {
			return nil, fmt.Errorf("LNK file name is too long")
		}

		b = binary.LittleEndian.AppendUint16(append(b, rec.Type), rec.Flags)
		b = binary.LittleEndian.AppendUint16(b, uint16(len(rec.Name)))
		b = append(append(b, rec.Name...), rec.Rest...)

	case *Unknown:
		b = append(b, rec.Data...)

	default:
		return nil, fmt.Errorf("unknown record %T", rec)
	}

	return b, nil
}

func encodeGSDEntry(b []byte, e Subrecord) ([]byte, error) {
	var err error

	b = append(b, byte(e.GSDType()))

	switch e := e.(type) {
	case *Psect:
		b = binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint16(append(b, e.Align), e.Flags), e.Alloc)
		if e.Shared {
			b = binary.LittleEndian.AppendUint32(b, e.Base)
		}

		return appendCounted(b, e.Name)

	case *IdentCheck:
		b = binary.LittleEndian.AppendUint16(b, e.Flags)
		if b, err = appendCounted(b, e.Name); err != nil {
			return nil, err
		}

		if b, err = appendCounted(b, string(e.Ident)); err != nil {
			return nil, err
		}

		return appendCounted(b, e.Object)

	case *Environment:
		b = binary.LittleEndian.AppendUint16(binary.LittleEndian.AppendUint16(b, e.Flags), e.Parent)

		return appendCounted(b, e.Name)

	case *Symbol:
		layout, ok := symbolLayouts[e.Type]
		if !ok {
			return nil, fmt.Errorf("%s is not a symbol subrecord type", e.Type)
		}

		b = binary.LittleEndian.AppendUint16(append(b, e.DataType), e.Flags)
		if layout.local {
			b = binary.LittleEndian.AppendUint16(b, e.Env)
		}

		if e.Defined() {
			switch {
			case layout.wordPsect:
				b = binary.LittleEndian.AppendUint16(b, e.Psect)
			case e.Psect > 0xFF:
				return nil, fmt.Errorf("%s %s: psect index %d needs a word-psect subrecord", e.Type, e.Name, e.Psect)
			default:
				b = append(b, byte(e.Psect))
			}

			b = binary.LittleEndian.AppendUint32(b, e.Value)

			if layout.extra {
				b = binary.LittleEndian.AppendUint32(b, e.Extra)
			}

			if layout.entry {
				b = binary.LittleEndian.AppendUint16(b, e.Mask)
			}
		}

		if b, err = appendCounted(b, e.Name); err != nil {
			return nil, err
		}

		if !layout.procedure || !e.Defined() {
			return b, nil
		}

		f := e.Formals
		if f == nil {
			return nil, fmt.Errorf("%s %s has no formal arguments", e.Type, e.Name)
		}

		if len(f.Args) != int(f.Max) {
			return nil, fmt.Errorf("%s %s: %d formal argument descriptors for a maximum of %d arguments", e.Type, e.Name, len(f.Args), f.Max)
		}

		b = append(b, f.Min, f.Max)

		for _, a := range f.Args {
			if len(a.Detail) > 0xFF {
				return nil, fmt.Errorf("%s %s: formal argument detail is too long", e.Type, e.Name)
			}

			b = append(append(b, a.ValCtl, byte(len(a.Detail))), a.Detail...)
		}

		return b, nil
	}

	return nil, fmt.Errorf("unknown GSD subrecord %T", e)
}

// reader reads little-endian fields from a record, remembering the first
// error (reading past the end) so callers can check once at the end.
type reader struct {
	b   []byte
	pos int
	err error
}

func (r *reader) done() bool { return r.err != nil || r.pos >= len(r.b) }

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}

	if n < 0 || r.pos+n > len(r.b) {
		r.err = fmt.Errorf("record ends %d bytes too soon", r.pos+n-len(r.b))

		return nil
	}

	b := r.b[r.pos : r.pos+n]
	r.pos += n

	return b
}

func (r *reader) byte() byte {
	if b := r.take(1); b != nil {
		return b[0]
	}

	return 0
}

func (r *reader) word() uint16 {
	if b := r.take(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}

	return 0
}

func (r *reader) long() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}

	return 0
}

// sized reads a byte (width 0), word (1), or longword (2).
func (r *reader) sized(width operandFormat) uint32 {
	switch width {
	case 0:
		return uint32(r.byte())
	case 1:
		return uint32(r.word())
	}

	return r.long()
}

// bytes returns a copy of the next n bytes.
func (r *reader) bytes(n int) []byte {
	return append([]byte(nil), r.take(n)...)
}

// counted reads a counted string: a length byte, then the characters.
func (r *reader) counted() string {
	return string(r.take(int(r.byte())))
}
