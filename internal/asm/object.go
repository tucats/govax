package asm

import (
	"fmt"
	"sort"
	"time"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmserrors"
)

// ObjectOptions are the parts of an object module that don't come from
// the source.
type ObjectOptions struct {
	// Created is the module's creation time; the zero time means now.
	Created time.Time
	// Language is the LNM header's text, naming the language processor;
	// "" means "govax MACRO".
	Language string
	// Source, when not "", is the SRC header's text. Real MACRO puts its
	// command line there.
	Source string
}

// defaultLanguage names the language processor when ObjectOptions doesn't.
const defaultLanguage = "govax MACRO"

// Object returns the object module for a finished MACRO-dialect assembly
// (see Assemble). It writes the records real MACRO writes, in the same
// order (docs/PHASE-27.md, subtask 3's log): the headers, then a GSD of
// every global symbol but the entry points, then each psect's definition
// and contents in source order, with each entry point's EPM where its
// mask is stored, then the end of module record. With traceback (the
// default, .ENABLE TRACEBACK), traceback records go after the headers and
// before the end of module record (see traceback). Debugger records
// aren't written yet.
func (a *Assembler) Object(opts ObjectOptions) (*obj.Module, error) {
	if a.dialect != DialectMACRO {
		return nil, vmserrors.New(vmserrors.VAX_INTERNAL, "Object needs the MACRO dialect")
	}

	pass := StartPhase()
	defer func() { a.phases[phasePass2] = a.phases[phasePass2].add(pass.Elapsed()) }()

	name, title := a.Title()

	b := &obj.Builder{
		Name:     name,
		Version:  a.Ident(),
		Language: opts.Language,
		Created:  opts.Created,
		Source:   opts.Source,
		Title:    title,
	}

	if b.Language == "" {
		b.Language = defaultLanguage
	}

	if b.Created.IsZero() {
		b.Created = time.Now()
	}

	if len(a.warnings) > 0 {
		b.Severity = obj.SeverityWarning
	} else {
		b.Severity = obj.SeveritySuccess
	}

	traceback := a.enabled&enableTraceback != 0
	if traceback {
		if err := b.Traceback(obj.DSTModuleBeginRecord(name)); err != nil {
			return nil, err
		}
	}

	a.globalSymbols(b)
	b.Break()

	e := emitter{a: a, b: b, relocs: map[relocKey]relocation{}, defined: map[*section]bool{}}
	for _, r := range a.relocs {
		e.relocs[relocKey{r.sect, r.offset, r.sect.storedBy(r.offset, r.stmt)}] = r
	}

	for i, ev := range a.events {
		e.index = i

		if err := e.event(ev); err != nil {
			return nil, err
		}
	}

	e.flushAbsStart(false)

	if len(e.relocs) > 0 {
		return nil, vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("%d relocations outside any stored data", len(e.relocs)))
	}

	if a.entrySeen {
		psect := uint16(0)
		if a.entrySect != nil {
			psect = uint16(a.entrySect.index)
		}

		b.SetTransfer(psect, a.entryAddr, false)
	}

	if traceback {
		if err := a.traceback(b); err != nil {
			return nil, err
		}
	}

	m, err := b.Build()
	if m != nil {
		a.objectRecords = len(m.Records)
	}

	return m, err
}

// traceback adds the traceback records real MACRO writes at the end of a
// module (docs/PHASE-29.md, subtask 10): a TBT record of routine begin
// records, one for each .ENTRY in name order, then one of psect records,
// one for each relocatable psect in psect number order (none for an
// absolute psect, named or not: psects.obj's OFFSETS), and the module
// end record. (The module begin record is written after the
// headers.)
func (a *Assembler) traceback(b *obj.Builder) error {
	var entries []*symbol

	for _, s := range a.symbols.byName {
		if s.flags&SymEntry != 0 && s.defined() && s.sect != nil {
			entries = append(entries, s)
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	routines := make([]obj.DSTRecord, 0, len(entries))
	for _, s := range entries {
		routines = append(routines, obj.DSTRoutineBeginRecord(s.name, uint16(s.sect.index), s.value))
	}

	if len(routines) > 0 {
		if err := b.Traceback(routines...); err != nil {
			return err
		}
	}

	var psects []obj.DSTRecord

	for _, s := range a.sections {
		if s.relocatable {
			psects = append(psects, obj.DSTPsectRecord(s.name, uint16(s.index), s.hi))
		}
	}

	return b.Traceback(append(psects, obj.DSTModuleEndRecord())...)
}

// globalSymbols adds a GSD of the module's global symbols, sorted by
// name, as real MACRO writes it before anything else: each external
// reference, and each global definition but an entry point's, whose EPM
// goes where its mask is stored. Real MACRO flags every reference REL.
func (a *Assembler) globalSymbols(b *obj.Builder) {
	names := make([]string, 0, len(a.symbols.byName))
	for name := range a.symbols.byName {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		s := a.symbols.byName[name]

		var flags uint16
		if s.flags&SymWeak != 0 {
			flags |= obj.SymWEAK
		}

		switch {
		case s.flags&SymExternal != 0:
			b.AddSymbol(obj.Symbol{Type: obj.GSDSymbol, Flags: flags | obj.SymREL, Name: name})

		case s.flags&SymGlobal != 0 && s.flags&SymEntry == 0 && s.defined():
			sym := obj.Symbol{Type: obj.GSDSymbol, Flags: flags | obj.SymDEF, Value: s.value, Name: name}

			switch {
			case s.sect != nil:
				sym.Flags |= obj.SymREL
				sym.Psect = uint16(s.sect.index)
			case s.absSect != nil:
				sym.Psect = uint16(s.absSect.index)
			}

			b.AddSymbol(sym)
		}
	}
}

// relocKey locates a relocation: its psect, its field's offset, and the
// output event that stored the field (see overwrite.go).
type relocKey struct {
	sect   *section
	offset uint32
	event  int
}

// emitter replays a MACRO-dialect assembly's output events into an
// object module builder.
type emitter struct {
	a      *Assembler
	b      *obj.Builder
	relocs map[relocKey]relocation
	// index is the event being written, an index in Assembler.events.
	index int
	// defined holds the psects the object has defined so far.
	defined map[*section]bool
	// sect and loc are the linker's location as the TIR commands so far
	// leave it.
	sect *section
	loc  uint32
	// absStart is set while the location in .  ABS  ., where assembly
	// starts, is still to be set (see flushAbsStart).
	absStart bool
}

// flushAbsStart sets the location in .  ABS  . for the start of assembly,
// or a return there, if that's still to do, unless skip. Real MACRO sets it except when code
// or data before any .PSECT moves assembly into . BLANK . first
// (docs/PHASE-27.md, subtask 11's log).
func (e *emitter) flushAbsStart(skip bool) {
	if !e.absStart {
		return
	}

	e.absStart = false

	if !skip {
		e.b.Emit(stackPsect(0, 0, false), obj.Command{Op: opSetRelocBase})
	}
}

// Real MACRO writes no TIR commands for an absolute psect, other than
// setting the location in .  ABS  . at the start, and on a return there:
// its offsets are only symbol values (docs/PHASE-27.md, subtask 11's
// log).
func (e *emitter) event(ev outEvent) error {
	if ev.kind != evSwitch || e.sect != nil {
		e.flushAbsStart(ev.kind == evSwitch && ev.implicit)
	}

	switch ev.kind {
	case evSwitch:
		if !e.defined[ev.sect] {
			if err := e.definePsect(ev.sect); err != nil {
				return err
			}
		}

		switch {
		case ev.sect.index == 0:
			// The start of assembly, or a return to .  ABS . (a
			// .RESTORE at the end of $RMSDEF, say): real MACRO
			// sets the location there as at the start
			// (testdata/mar/macros/vax/rmscopy.obj, after $RMSDEF).
			// See flushAbsStart.
			e.absStart = true
			e.sect, e.loc = ev.sect, ev.offset

		case ev.sect.relocatable:
			e.setLocation(ev.sect, ev.offset)

		default:
			e.sect, e.loc = ev.sect, ev.offset
		}

	case evSet:
		// ". =" moves the location by the difference, as real MACRO
		// writes it (CTL_AUGRB).
		if ev.sect.relocatable {
			e.b.Emit(obj.Command{Op: opAugmentRelocBase, Value: ev.offset - e.loc})
		}

		e.loc = ev.offset

	case evGap:
		if ev.sect.relocatable {
			e.b.Emit(obj.Command{Op: opAugmentRelocBase, Value: ev.size})
		}

		e.loc += ev.size

	case evData:
		return e.data(ev.sect, ev.offset, ev.size)

	case evConst:
		store := storeCommand(ev.size)
		if ev.signed {
			store = obj.Command{Op: tirOp("STO_SB")}
		}

		e.b.Emit(stackConstant(ev.value), store)
		e.loc += ev.size

	case evEntry:
		e.b.Emit(stackConstant(ev.value))
		e.entryPoint(ev)
		e.b.Emit(storeCommand(2))
		e.b.Break()

		e.loc += ev.size

	case evPatch:
		e.b.Emit(obj.Command{Op: opAugmentRelocBase, Value: ev.offset - e.loc})

		if ev.immediate {
			e.b.Store(littleEndian(ev.value, ev.size))
		} else {
			e.b.Emit(stackConstant(ev.value), storeCommand(ev.size))
		}

		e.b.Emit(obj.Command{Op: opAugmentRelocBase, Value: e.loc - ev.offset - ev.size})
	}

	return nil
}

// definePsect adds s's PSC subrecord. Its index must be the one the
// assembler numbered it with, which it is when psects are defined in the
// order they were created. An absolute psect allocates nothing (the MACRO
// manual, .PSECT).
func (e *emitter) definePsect(s *section) error {
	alloc := s.hi
	if !s.relocatable {
		alloc = 0
	}

	// Each psect definition is a GSD record of its own, as real MACRO
	// writes them (psects.obj, in docs/PHASE-27.md subtask 11's log).
	e.b.Break()
	index := e.b.AddPsect(obj.Psect{Align: byte(s.align), Flags: uint16(s.flags), Alloc: alloc, Name: s.name})
	e.b.Break()

	if int(index) != s.index {
		return vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("psect %s is index %d in the object, %d in the assembly", s.name, index, s.index))
	}

	e.defined[s] = true

	return nil
}

// setLocation points the linker at an offset in a psect.
func (e *emitter) setLocation(s *section, offset uint32) {
	e.b.Emit(stackPsect(s.index, offset, false), obj.Command{Op: opSetRelocBase})
	e.sect, e.loc = s, offset
}

// entryPoint adds the EPM subrecord for an .ENTRY.
func (e *emitter) entryPoint(ev outEvent) {
	s := ev.sym

	flags := obj.SymDEF
	if s.flags&SymWeak != 0 {
		flags |= obj.SymWEAK
	}

	sym := obj.Symbol{Type: obj.GSDEntry, Flags: flags, Value: ev.offset, Mask: s.mask, Name: s.name}
	if ev.sect.relocatable {
		sym.Flags |= obj.SymREL
		sym.Psect = uint16(ev.sect.index)
	}

	e.b.AddSymbol(sym)
}

// data writes the size bytes at offset in s: STORE IMMEDIATE runs for the
// bytes the assembler finished, and each relocation's stack program and
// store command in place of its field. Each relocation is written by the
// event that stored its field, even one a later event stores over (see
// overwrite.go).
func (e *emitter) data(s *section, offset, size uint32) error {
	for p := offset; p < offset+size; {
		// An operand's index and mode bytes, then the field they
		// introduce: real MACRO stacks the value first, then stores
		// those bytes, then the value.
		next, prefix := e.prefixedAt(s, p, offset+size)

		r, ok := e.relocAt(s, p)

		switch {
		case prefix > 0:
			r = next

			if err := e.stackProgram(r.expr); err != nil {
				return err
			}

			for range prefix {
				e.b.Store([]byte{e.byteAt(s, p)})

				p++
			}

		case !ok:
			e.b.Store([]byte{e.byteAt(s, p)})

			p++

			continue

		default:
			if err := e.stackProgram(r.expr); err != nil {
				return err
			}
		}

		delete(e.relocs, relocKey{s, p, e.index})

		store, ok := relocStores[r.kind]
		if !ok {
			return vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("no store command for a %s relocation", fixupKindNames[r.kind]))
		}

		e.b.Emit(obj.Command{Op: store})
		
		p += uint32(fixupSize(r.kind))
	}

	e.loc = offset + size

	return nil
}

// prefixedAt returns the relocation whose operand specifier starts at p,
// before end, when bytes of it (an index byte, a mode byte) come before
// its field, and how many.
func (e *emitter) prefixedAt(s *section, p, end uint32) (relocation, int) {
	for n := uint32(1); n <= 2 && p+n < end; n++ {
		if r, ok := e.relocAt(s, p+n); ok && r.prefix == int(n) {
			return r, int(n)
		}
	}

	return relocation{}, 0
}

// byteAt returns the byte at offset p in s as the event being written
// stored it (see overwrite.go).
func (e *emitter) byteAt(s *section, p uint32) byte {
	history := s.owners[p]

	for i := len(history) - 1; i >= 0; i-- {
		if history[i].event == e.index {
			if i == len(history)-1 {
				break
			}

			return history[i].value
		}
	}

	return s.img.loadByte(p)
}

// relocAt returns the relocation whose field starts at p in s, if the
// event being written is the one that stored it.
func (e *emitter) relocAt(s *section, p uint32) (relocation, bool) {
	r, ok := e.relocs[relocKey{s, p, e.index}]

	return r, ok
}

// stackProgram emits the TIR commands that leave t's value on the
// linker's stack: t's tree in postfix order.
func (e *emitter) stackProgram(t *rexpr) error {
	switch t.op {
	case rConst:
		e.b.Emit(stackConstant(t.v))

	case rBase:
		e.b.Emit(stackPsect(t.sect.index, t.v, t.dot))

	case rSym:
		e.b.Emit(obj.Command{Op: opStackGlobal, Name: t.key})

	case rMask:
		e.b.Emit(obj.Command{Op: opStackEntryMask, Name: t.key})

	case rNeg, rCom:
		if err := e.stackProgram(t.l); err != nil {
			return err
		}

		op := opNegate
		if t.op == rCom {
			op = opComplement
		}

		e.b.Emit(obj.Command{Op: op})

	case rBinary:
		if err := e.stackProgram(t.l); err != nil {
			return err
		}

		if err := e.stackProgram(t.r); err != nil {
			return err
		}

		op, ok := binaryOps[t.bin]
		if !ok {
			return vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("no TIR operator for %q", t.bin))
		}

		e.b.Emit(obj.Command{Op: op})
	}

	return nil
}

// tirOp returns the TIR command named name (without its TIR$C_ prefix).
func tirOp(name string) obj.Op {
	op, ok := obj.OpByName(name)
	if !ok {
		panic("no TIR command " + name)
	}

	return op
}

var (
	opStackGlobal      = tirOp("STA_GBL")
	opStackEntryMask   = tirOp("STA_EPM")
	opSetRelocBase     = tirOp("CTL_SETRB")
	opAugmentRelocBase = tirOp("CTL_AUGRB")
	opNegate           = tirOp("OPR_NEG")
	opComplement       = tirOp("OPR_COM")

	// binaryOps are the TIR operators for MACRO-32's binary operators.
	binaryOps = map[byte]obj.Op{
		'+': tirOp("OPR_ADD"), '-': tirOp("OPR_SUB"), '*': tirOp("OPR_MUL"), '/': tirOp("OPR_DIV"),
		'&': tirOp("OPR_AND"), '!': tirOp("OPR_IOR"), '\\': tirOp("OPR_EOR"), '@': tirOp("OPR_ASH"),
	}

	// relocStores are the store commands for each kind of relocation.
	relocStores = map[fixupKind]obj.Op{
		fixAddrB: tirOp("STO_B"), fixAddrW: tirOp("STO_W"), fixAddrL: tirOp("STO_L"),
		fixDispB: tirOp("STO_BD"), fixDispW: tirOp("STO_WD"), fixDispL: tirOp("STO_LD"),
		fixBranchB: tirOp("STO_BD"), fixBranchW: tirOp("STO_WD"), fixBranchL: tirOp("STO_LD"),
		fixAddress: tirOp("STO_PIDR"), fixPICR: tirOp("STO_PICR"),
		fixSignedB: tirOp("STO_SB"), fixSignedW: tirOp("STO_SW"),
	}
)

// storeCommand returns the command that stores a size-byte value from the
// stack.
func storeCommand(size uint32) obj.Command {
	switch size {
	case 1:
		return obj.Command{Op: tirOp("STO_B")}
	case 2:
		return obj.Command{Op: tirOp("STO_W")}
	}

	return obj.Command{Op: tirOp("STO_L")}
}

// stackConstant returns the shortest command that stacks v, as real MACRO
// chooses it: STA_UB 4, STA_UB ^XFF, STA_LW ^X010E0000. The signed forms
// for a negative value aren't confirmed by a real object.
func stackConstant(v uint32) obj.Command {
	s := int32(v)

	switch {
	case v <= 0xFF:
		return obj.Command{Op: tirOp("STA_UB"), Value: v}
	case s >= -128 && s < 0:
		return obj.Command{Op: tirOp("STA_SB"), Value: v & 0xFF}
	case v <= 0xFFFF:
		return obj.Command{Op: tirOp("STA_UW"), Value: v}
	case s >= -32768 && s < 0:
		return obj.Command{Op: tirOp("STA_SW"), Value: v & 0xFFFF}
	}

	return obj.Command{Op: tirOp("STA_LW"), Value: v}
}

// stackPsect returns the command that stacks a psect's base plus offset:
// the shortest form, whose byte or word offset is signed, or always the
// longword form for ".", as real MACRO writes them.
func stackPsect(index int, offset uint32, long bool) obj.Command {
	s := int32(offset)

	switch {
	case !long && s >= -128 && s <= 127:
		return obj.Command{Op: tirOp("STA_PB"), Psect: uint16(index), Value: offset & 0xFF}
	case !long && s >= -32768 && s <= 32767:
		return obj.Command{Op: tirOp("STA_PW"), Psect: uint16(index), Value: offset & 0xFFFF}
	}

	return obj.Command{Op: tirOp("STA_PL"), Psect: uint16(index), Value: offset}
}
