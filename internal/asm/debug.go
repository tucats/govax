package asm

import (
	"sort"
	"strings"

	"github.com/tucats/govax/internal/obj"
)

// This file decides what the object's debugger (DBG) records say
// (docs/PHASE-29.md, subtask 12): which symbols get a record and what
// each says of its data, and the line-number table's rows. Object
// (object.go) writes them; obj builds and packs the records (dbg.go,
// dbglines.go).
//
// Debugger records are on from the first point .ENABLE DEBUG (or MACRO's
// /DEBUG) is in force: real MACRO keeps writing them after a .DISABLE
// DEBUG (testdata/mar/dst's DSTDIS), and writes none for what came
// before the .ENABLE (DSTDBG). Each symbol defined from then on gets a
// record, and the listing's symbol table flags it D; each line of code
// assembled from then on gets a row.

// debugging reports whether debugger records are on: whether DEBUG has
// been enabled at any point so far.
func (a *Assembler) debugging() bool {
	if a.enabled&enableDebug != 0 {
		a.debugSeen = true
	}

	return a.debugSeen
}

// debugKind is what a label's symbol record holds of its data. (A label
// with no data type has no labelData, and a DSTLabel record.)
type debugKind int

const (
	// debugList: a list of items (or a .BLKx's), one or many.
	debugList debugKind = iota
	// debugString: a string descriptor follows (.ASCII, .PACKED).
	debugString
	// debugSimple: one item, the data's address (.ASCIC, .ASCIZ).
	debugSimple
	// debugDescriptor: the address of a descriptor (.ASCID).
	debugDescriptor
)

// debugData is what the data directive after a label says of its data:
// its type, the kind of record, and the size of an item.
type debugData struct {
	dtype obj.DSTType
	kind  debugKind
	size  uint16
}

// dataDirectiveTypes are the data directives that give a label a data
// type, from real MACRO's records for testdata/mar/dst/dstsym.mar.
var dataDirectiveTypes = map[string]debugData{
	"BYTE": {obj.DSTDataB, debugList, 1}, "SIGNED_BYTE": {obj.DSTDataB, debugList, 1}, "BLKB": {obj.DSTDataB, debugList, 1},
	"WORD": {obj.DSTDataW, debugList, 2}, "SIGNED_WORD": {obj.DSTDataW, debugList, 2}, "BLKW": {obj.DSTDataW, debugList, 2},
	"LONG": {obj.DSTDataL, debugList, 4}, "BLKL": {obj.DSTDataL, debugList, 4},
	"ADDRESS": {obj.DSTDataLU, debugList, 4}, "BLKA": {obj.DSTDataLU, debugList, 4},
	"QUAD": {obj.DSTDataQ, debugList, 8}, "BLKQ": {obj.DSTDataQ, debugList, 8},
	"OCTA": {obj.DSTDataO, debugList, 16}, "BLKO": {obj.DSTDataO, debugList, 16},
	"F_FLOATING": {obj.DSTDataF, debugList, 4}, "FLOAT": {obj.DSTDataF, debugList, 4}, "BLKF": {obj.DSTDataF, debugList, 4},
	"D_FLOATING": {obj.DSTDataD, debugList, 8}, "DOUBLE": {obj.DSTDataD, debugList, 8}, "BLKD": {obj.DSTDataD, debugList, 8},
	"G_FLOATING": {obj.DSTDataG, debugList, 8}, "BLKG": {obj.DSTDataG, debugList, 8},
	"H_FLOATING": {obj.DSTDataH, debugList, 16}, "BLKH": {obj.DSTDataH, debugList, 16},
	"ASCII":  {obj.DSTDataT, debugString, 1},
	"PACKED": {obj.DSTDataP, debugString, 1},
	"ASCIC":  {obj.DSTDataASCIC, debugSimple, 1},
	"ASCIZ":  {obj.DSTDataASCIZ, debugSimple, 1},
	"ASCID":  {obj.DSTDataT, debugDescriptor, 1},
}

// untypingDirectives are the directives that leave a label before them
// with no data type: real MACRO's label before .ALIGN, and one at a
// psect's end, are DSTLabel symbols (dstsym.mar's LALIGN and LEND). The
// others here (a .PSECT, .ENTRY, .MASK, or .END after a label) aren't
// confirmed.
var untypingDirectives = map[string]bool{
	"ALIGN": true, "PSECT": true, "SAVE_PSECT": true, "SAVE": true, "RESTORE_PSECT": true,
	"RESTORE": true, "END": true, "ENTRY": true, "MASK": true,
}

// labelData is what a label's symbol record says of its data.
type labelData struct {
	debugData
	// count is how many items the directive stored (a list's, or a
	// .BLKx's), and length a string's length (a packed number's in
	// digits).
	count  uint32
	length uint16
}

// debugLabelDefined notes that the label name was just defined: the next
// statement that stores data, or does something else, decides its data
// type. A label before it that's still waiting has none: real MACRO's
// first of two labels at one location is a DSTLabel symbol (LTWO1).
func (a *Assembler) debugLabelDefined(name string) {
	if a.dialect != DialectMACRO || isLocalLabel(name) {
		return
	}

	a.typeLabels(nil)

	resolved, _ := a.resolvedName(name)
	if sym, ok := a.symbols.find(resolved); ok {
		a.untyped = append(a.untyped, sym)
	}
}

// typeLabels gives the labels waiting for a type what d says of their
// data (nil for none).
func (a *Assembler) typeLabels(d *labelData) {
	for _, sym := range a.untyped {
		sym.data = d
	}

	a.untyped = nil
}

// debugDirective types the waiting labels for the directive name, which
// stored size bytes in the section it began in. digits is a .PACKED's
// digit count.
func (a *Assembler) debugDirective(name string, size uint32, digits int) {
	if len(a.untyped) == 0 {
		return
	}

	d, ok := dataDirectiveTypes[name]
	if !ok {
		if untypingDirectives[name] {
			a.typeLabels(nil)
		}

		return
	}

	ld := &labelData{debugData: d, count: size / uint32(d.size), length: uint16(size)}
	if name == "PACKED" {
		ld.length = uint16(digits)
	}

	a.typeLabels(ld)
}

// debugSymbols returns the symbol records for the module: one for each
// symbol defined while debugger records were on, but external ones,
// local labels, and .ENTRY routines (whose traceback routine begin
// record describes them), in ASCII name order.
func (a *Assembler) debugSymbols() []obj.DSTRecord {
	var syms []*symbol

	for _, s := range a.symbols.byName {
		if s.debug && s.defined() && s.flags&(SymBuiltin|SymLocalLabel|SymEntry) == 0 && !isLocalLabel(s.name) {
			syms = append(syms, s)
		}
	}

	sort.Slice(syms, func(i, j int) bool { return syms[i].name < syms[j].name })

	recs := make([]obj.DSTRecord, 0, len(syms))
	for _, s := range syms {
		recs = append(recs, symbolRecord(s))
	}

	return recs
}

// symbolRecord returns s's symbol record.
func symbolRecord(s *symbol) obj.DSTRecord {
	switch {
	case s.sect == nil:
		// A constant, or a label in an absolute psect (dstsym.mar's
		// ABSLAB): its value, of type L.
		return obj.DSTValueRecord(obj.DSTDataL, s.name, s.value)

	case s.flags&SymLabel == 0 || s.data == nil:
		// A relocatable assignment, or a label of no data.
		return obj.DSTAddressRecord(obj.DSTLabel, s.name, obj.SymbolValue, uint16(s.sect.index), s.value)
	}

	d, psect := s.data, uint16(s.sect.index)

	switch d.kind {
	case debugString:
		return obj.DSTStringRecord(d.dtype, s.name, d.length, psect, s.value)
	case debugDescriptor:
		return obj.DSTAddressRecord(d.dtype, s.name, obj.SymbolDescriptorAddress, psect, s.value)
	case debugList:
		if d.count != 1 {
			return obj.DSTArrayRecord(d.dtype, s.name, d.size, d.count, psect, s.value)
		}
	}

	return obj.DSTAddressRecord(d.dtype, s.name, obj.SymbolAddress, psect, s.value)
}

// debugLine returns the line a recorded line of the source on top of the
// source stack, line of its own, is for in the line-number table: the
// program's line it came from. A line of a macro's expansion is the
// macro call's; one of a repeat block is the block's own line in the
// source (the line after the directive, plus its place in the block),
// and repeat says so: real MACRO starts each such row with LineSetLine
// (testdata/mar/dst/dstln3.mar).
func (a *Assembler) debugLine(line int) (int, bool) {
	repeat := false

	for i := len(a.sources) - 1; i > 0; i-- {
		f := a.sources[i]

		switch f.kind {
		case sourceRepeat:
			line = f.callLine + line
			repeat = true
		default:
			line = f.callLine
		}
	}

	return line, repeat
}

// lineRow is one row of the line-number table: a line, and where its
// code starts. size is how many bytes the row covers: its code and any
// data stored right after it.
type lineRow struct {
	line   int
	repeat bool
	sect   *section
	loc    uint32
	size   uint32
	// first says the row starts a segment: a run of code with no gap,
	// and entry that it's an .ENTRY's.
	first bool
	entry bool
}

// noCallRoutines returns the psects whose code, as the line-number table
// has it, begins with no .ENTRY: real MACRO gives each a routine begin
// record with no name, flagged as entered by JSB, at its base (dstln2's
// FIRST and DATA, and dstdbg's CODE, whose .ENTRY came before .ENABLE
// DEBUG). That the rule is the first row's, and not whether the psect
// has an .ENTRY at all, fits both but isn't confirmed.
func (a *Assembler) noCallRoutines() []*section {
	if !a.debugSeen {
		return nil
	}

	var out []*section

	seen := map[*section]bool{}

	for _, r := range a.lineRows() {
		if !seen[r.sect] {
			seen[r.sect] = true

			if !r.entry {
				out = append(out, r.sect)
			}
		}
	}

	return out
}

// lineRows returns the line-number table's rows, from the recorded lines
// of code assembled while debugger records were on.
//
// Each instruction (or .ENTRY mask) makes a row, but one that continues
// the row before it at the same line (a macro's expansion is one row,
// its call's line) joins it. Data stored right after a row's code (in
// its psect, where its code ends) is part of the row, as real MACRO
// counts it (dstln1.mar's .BYTE and .LONG in code). Code anywhere but
// where the last row's segment ends starts a new segment: after a gap, or
// in another psect.
func (a *Assembler) lineRows() []lineRow {
	var (
		end uint32 // where the last row's segment ends
	)

	rows := make([]lineRow, 0)

	for _, l := range a.listLines {
		if !l.debug || l.stmt == 0 || l.collected || l.skipped || l.endSect != l.sect || l.endLoc <= l.loc {
			continue
		}

		n := len(rows)
		continues := n > 0 && rows[n-1].sect == l.sect && end == l.loc

		if !l.instruction && l.op != ".ENTRY" {
			// Data: part of the row before it, if it continues it. (A
			// macro call's line, or a repeat block's, spans its
			// expansion's lines, which are recorded too.)
			_, data := dataDirectiveTypes[strings.TrimPrefix(l.op, ".")]
			if data && continues && len(l.fields) > 0 {
				end = l.endLoc
				rows[n-1].size = end - rows[n-1].loc
			}

			continue
		}

		if continues && !l.debugRepeat && rows[n-1].line == l.debugLine {
			end = l.endLoc
			rows[n-1].size = end - rows[n-1].loc

			continue
		}

		rows = append(rows, lineRow{
			line: l.debugLine, repeat: l.debugRepeat, sect: l.sect, loc: l.loc,
			size: l.endLoc - l.loc, first: !continues, entry: l.op == ".ENTRY",
		})
		end = l.endLoc
	}

	return rows
}

// lineCommands returns the line-number table's commands that come where
// row i's code starts: the end of the segment before it, if it starts
// one, then what makes its row.
func lineCommands(rows []lineRow, i int) []obj.DSTItem {
	r := rows[i]

	var out []obj.DSTItem

	if i == 0 {
		out = append(out, obj.LineStart())
	}

	if r.first {
		if i > 0 {
			out = append(out, obj.LineEnd(rows[i-1].size))
		}

		out = append(out,
			obj.LineSetAddress(stackPsect(r.sect.index, r.loc, false)),
			obj.LineSetLine(r.line-1),
			obj.LineAdvance(0))

		return out
	}

	prev := rows[i-1]

	switch {
	case r.repeat || r.line <= prev.line:
		out = append(out, obj.LineSetLine(r.line-1))
	case r.line > prev.line+1:
		out = append(out, obj.LineSkip(r.line-prev.line-1))
	}

	return append(out, obj.LineAdvance(prev.size))
}
