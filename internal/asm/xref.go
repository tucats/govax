package asm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file makes a MACRO listing's cross reference (MACRO/CROSS_REFERENCE,
// docs/PHASE-29.md, subtask 9): for each symbol, macro, instruction,
// directive, or register the program used, the lines that defined it and
// the lines that referred to it. Each kind is a section of its own, on a
// page of its own, after the psect synopsis:
//
//	SYMBOL          VALUE        DEFINITION      REFERENCES...
//	------          -----        ----------      -------------
//	COUNT          =00000003     10     (1)      11     (1)    #-34     (1)
//
// Every rule here comes from real MACRO's listings: the probe's xref.lis
// (/CROSS_REFERENCE), xrefall.lis (/CROSS_REFERENCE=ALL), and symxref.lis
// (the symbol table probe with /CROSS_REFERENCE). docs/PHASE-29.md lists
// the ones no real listing shows yet, which are govax's own choices.
//
// A line number is the program's: a reference in a macro expansion (or a
// repeat block, or an .INCLUDE file) is listed at the program line that
// called it, as the error summary counts it. Each line is listed once
// for an entry, however many times the line names it.

// The kinds of cross reference, as MACRO's /CROSS_REFERENCE names them.
const (
	xrefSymbols = iota
	xrefMacros
	xrefOpcodes
	xrefDirectives
	xrefRegisters
	xrefKinds
)

// xrefKeywords are /CROSS_REFERENCE's keywords: each kind, ALL, and NONE.
var xrefKeywords = map[string]uint{
	"SYMBOLS":    1 << xrefSymbols,
	"MACROS":     1 << xrefMacros,
	"OPCODES":    1 << xrefOpcodes,
	"DIRECTIVES": 1 << xrefDirectives,
	"REGISTERS":  1 << xrefRegisters,
	"ALL":        1<<xrefKinds - 1,
	"NONE":       0,
}

// xrefDefault is what /CROSS_REFERENCE with no keywords lists: symbols and
// macros (xref.lis).
const xrefDefault = 1<<xrefSymbols | 1<<xrefMacros

// xrefFlagOperand is how the cross reference marks a reference that was a
// register operand of an instruction, or the value of a literal or
// immediate operand (#COUNT): "#-" in the two columns before the line
// number (xref.lis, xrefall.lis). A register named in a register mask
// (^M<R2>) and a symbol used as an address (MOVAL TABLE, R2) get no mark.
const xrefFlagOperand = "#-"

// crossRef is what the assembly has recorded for the cross reference.
type crossRef struct {
	// kinds are the kinds the listing shows (bits 1<<xrefSymbols...).
	kinds uint
	// entries are each kind's entries, by name.
	entries [xrefKinds]map[string]*xrefEntry
	// off says .NOCROSS (with no symbols) turned cross-referencing off,
	// until a .CROSS with none; excluded are the symbols a .NOCROSS
	// named, until a .CROSS names them.
	off      bool
	excluded map[string]bool
	// line is the program line being assembled, and quiet says the
	// statement's names aren't references (.END's transfer address).
	line  int
	quiet bool
	// flag is the mark the symbols read now get (xrefFlagOperand while
	// a literal's value is read).
	flag string
}

// xrefEntry is one name's cross-reference entry.
type xrefEntry struct {
	// def is the line that defined the name, or 0.
	def int
	// refs are the lines that referred to it, each once.
	refs []xrefRef
	// value is an opcode's value, and size a macro's (see macroSize).
	value uint32
	size  int
}

// xrefRef is a reference: its line, and its mark ("" or xrefFlagOperand).
type xrefRef struct {
	line int
	flag string
}

// SetCrossReference asks for a cross reference in the listing (MACRO's
// /CROSS_REFERENCE), of the kinds names lists (SYMBOLS, MACROS, OPCODES,
// DIRECTIVES, REGISTERS, ALL, or NONE), or of symbols and macros if it
// lists none. on false (/NOCROSS_REFERENCE) asks for none. A cross
// reference is made only along with a listing (SetListing).
func (a *Assembler) SetCrossReference(on bool, names []string) error {
	a.xrefKinds = 0

	if !on {
		return nil
	}

	// A blank name is no name: DCL gives /CROSS_REFERENCE with no value
	// as one empty one.
	var named []string

	for _, name := range names {
		if name = strings.ToUpper(strings.TrimSpace(name)); name != "" {
			named = append(named, name)
		}
	}

	if len(named) == 0 {
		a.xrefKinds = xrefDefault

		return nil
	}

	for _, name := range named {
		k, ok := xrefKeywords[name]
		if !ok {
			return vmserrors.New(vmserrors.VAX_BADKEYWORD, "/CROSS_REFERENCE", name)
		}

		// NONE undoes the kinds named before it.
		if k == 0 {
			a.xrefKinds = 0
		}

		a.xrefKinds |= k
	}

	return nil
}

// startCrossReference starts an assembly's cross reference, if one was
// asked for.
func (a *Assembler) startCrossReference() {
	a.xref = nil

	if a.xrefKinds == 0 {
		return
	}

	a.xref = &crossRef{kinds: a.xrefKinds, excluded: map[string]bool{}}

	for k := range a.xref.entries {
		a.xref.entries[k] = map[string]*xrefEntry{}
	}
}

// xrefProgramLine notes the program line now being assembled, which the
// references that follow, in it and in what it expands, are listed at.
func (a *Assembler) xrefProgramLine(line int) {
	if a.xref != nil {
		a.xref.line = line
	}
}

// xrefEntryFor returns kind's entry for name, making it, or nil when
// nothing is being recorded: no cross reference was asked for, or
// .NOCROSS turned it off, or this is a symbol a .NOCROSS named, or a
// macro library's definition is being read.
func (a *Assembler) xrefEntryFor(kind int, name string) *xrefEntry {
	x := a.xref
	if x == nil || !a.listing || x.off || x.line == 0 || name == "" {
		return nil
	}

	// A macro library's definition isn't the program's: its .MACRO and
	// .ENDM aren't listed.
	if a.inLibrary() {
		return nil
	}

	if kind == xrefSymbols && (x.excluded[name] || isLocalLabel(name)) {
		return nil
	}

	e, ok := x.entries[kind][name]
	if !ok {
		e = &xrefEntry{}
		x.entries[kind][name] = e
	}

	return e
}

// xrefDefine records that the current line defined kind's name.
func (a *Assembler) xrefDefine(kind int, name string) *xrefEntry {
	e := a.xrefEntryFor(kind, name)
	if e != nil {
		e.def = a.xref.line
	}

	return e
}

// xrefRefer records that the current line referred to kind's name, with
// flag as its mark. A line already listed for the name keeps its first
// mark.
func (a *Assembler) xrefRefer(kind int, name, flag string) *xrefEntry {
	if a.xref != nil && a.xref.quiet {
		return nil
	}

	e := a.xrefEntryFor(kind, name)
	if e == nil {
		return nil
	}

	for _, r := range e.refs {
		if r.line == a.xref.line {
			return e
		}
	}

	e.refs = append(e.refs, xrefRef{line: a.xref.line, flag: flag})

	return e
}

// xrefSymbol records a reference to the symbol name, marked as the
// operand being read says (see crossRef.flag).
func (a *Assembler) xrefSymbol(name string) {
	if a.xref != nil {
		a.xrefRefer(xrefSymbols, name, a.xref.flag)
	}
}

// xrefOperandMark marks the symbols read from now on as a literal
// operand's (xrefFlagOperand), and returns the function that ends it:
//
//	defer a.xrefOperandMark()()
func (a *Assembler) xrefOperandMark() func() {
	if a.xref == nil {
		return func() {}
	}

	save := a.xref.flag
	a.xref.flag = xrefFlagOperand

	return func() { a.xref.flag = save }
}

// xrefOpcode records a use of the instruction written name, whose opcode
// is value: a one-byte opcode, or a two-byte one's bytes as a word (the
// FD prefix low).
func (a *Assembler) xrefOpcode(name string, value uint32) {
	if e := a.xrefRefer(xrefOpcodes, name, ""); e != nil {
		e.value = value
	}
}

// xrefRegister records a reference to the register written name: marked
// as an operand's in an instruction (flag xrefFlagOperand), unmarked in a
// register mask.
func (a *Assembler) xrefRegister(name, flag string) {
	a.xrefRefer(xrefRegisters, name, flag)
}

// register parses a register at c, as parseRegister does, and records it
// for the cross reference as an instruction's operand.
func (a *Assembler) register(c *cursor, first byte) (reg vax.Reg, err error) {
	start := c.pos
	if first != 0 {
		start--
	}

	reg, err = parseRegister(c, first)
	if err == nil && a.xref != nil {
		a.xrefRegister(strings.TrimLeft(c.s[start:c.pos], " \t"), xrefFlagOperand)
	}

	return reg, err
}

// pseudoCross assembles .CROSS and .NOCROSS (on says which). With no
// symbols, they turn cross-referencing on and off for everything; with
// symbols, for those symbols. A .CROSS with no symbols leaves the symbols
// a .NOCROSS named excluded, and a .CROSS with symbols does nothing while
// cross-referencing is off (the manual's .CROSS notes 1 and 2: excluded
// is kept apart from off, so both follow).
func (a *Assembler) pseudoCross(c *cursor, on bool) error {
	first := true
	named := false

	for {
		c.skipBlanks()

		if err := a.listSeparator(c, first); err != nil {
			return err
		}

		if c.atEnd() {
			break
		}

		first = false

		name := scanName(c)
		if name == "" {
			return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
		}

		named = true

		if a.xref != nil {
			if on {
				delete(a.xref.excluded, name)
			} else {
				a.xref.excluded[name] = true
			}
		}
	}

	if !named && a.xref != nil {
		a.xref.off = !on
	}

	return nil
}

// macroSize returns the size the macro cross reference shows for a macro
// whose body is lines: the 512-byte pages its text fills, at least 1.
// Both real listings' macros, of one and four short lines, show 1; no
// real listing shows a bigger one yet.
func macroSize(lines []string) int {
	n := 0
	for _, line := range lines {
		n += len(line) + 1
	}

	return max(1, (n+511)/512)
}

// The second heading line's name for the cross-reference pages.
const labelCrossReference = "Cross reference"

// xrefLineWidth is the width the cross reference fills: a listing line's.
const xrefLineWidth = 132

// crossReference appends the cross-reference sections the assembly asked
// for, each on a page of its own: symbols, macros, opcodes, directives,
// and registers. A section with no entries is left out.
func (a *Assembler) crossReference(p *listPager) {
	x := a.xref
	if x == nil {
		return
	}

	type section struct {
		kind   int
		title  string
		indent int
		lines  func(names []string) []string
	}

	sections := []section{
		{xrefSymbols, "Symbol Cross Reference", 45, a.symbolXrefLines},
		{xrefMacros, "Macros Cross Reference", 45, x.macroXrefLines},
		{xrefOpcodes, "Opcode Cross Reference", 45, x.opcodeXrefLines},
		{xrefDirectives, "Directives Cross Reference", 43, x.directiveXrefLines},
		{xrefRegisters, "Register Cross Reference", 44, x.registerXrefLines},
	}

	for _, s := range sections {
		if x.kinds&(1<<s.kind) == 0 {
			continue
		}

		names := x.names(s.kind)
		if s.kind == xrefSymbols {
			names = a.xrefSymbolNames(names)
		}

		if len(names) == 0 {
			continue
		}

		p.label = labelCrossReference
		p.breakPage()
		addSection(p, s.title, s.indent)

		for _, line := range s.lines(names) {
			p.add(line)
		}
	}
}

// names returns kind's names, in ASCII order (so "$" and "." sort before
// letters, and R10 before R2).
func (x *crossRef) names(kind int) []string {
	out := make([]string, 0, len(x.entries[kind]))

	for name := range x.entries[kind] {
		out = append(out, name)
	}

	sort.Strings(out)

	return out
}

// xrefSymbolNames returns the names that are symbols the symbol table
// knows, leaving out the assembler's own.
func (a *Assembler) xrefSymbolNames(names []string) []string {
	out := names[:0]

	for _, name := range names {
		if s, ok := a.symbols.find(name); ok && s.flags&(SymBuiltin|SymLocalLabel) == 0 {
			out = append(out, name)
		}
	}

	return out
}

// xrefRefText returns a reference (or a definition) as the cross
// reference lists it: its mark in flagWidth columns (none if 0), the line
// number in 7, the file number, and 4 blanks.
func xrefRefText(r xrefRef, flagWidth int) string {
	return fmt.Sprintf("%-*s%-7d(1)    ", flagWidth, r.flag, r.line)
}

// xrefRows returns an entry's lines: head, then the references, as many to
// a line as fit in xrefLineWidth, each further line starting with blanks
// as wide as head. The references are in the order of their line
// numbers' text: real MACRO lists line 38 before line 8 (xref.lis's
// LIB$PUT_OUTPUT).
func xrefRows(head string, refs []xrefRef, flagWidth int) []string {
	sorted := append([]xrefRef(nil), refs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return strconv.Itoa(sorted[i].line) < strconv.Itoa(sorted[j].line)
	})

	size := flagWidth + 14
	perLine := max(1, (xrefLineWidth-len(head))/size)

	var (
		out []string
		b   strings.Builder
	)

	b.WriteString(head)

	for i, r := range sorted {
		if i > 0 && i%perLine == 0 {
			out = append(out, b.String())
			b.Reset()
			b.WriteString(strings.Repeat(" ", len(head)))
		}

		b.WriteString(xrefRefText(r, flagWidth))
	}

	return append(out, b.String())
}

// symbolXrefLines returns the symbol cross reference: each symbol's name,
// "=" if a direct assignment gave its value, the value (0 for an external
// symbol), "-R" if it's relocatable or "-XR" if external, the line that
// defined it, and the lines that referred to it, each with its mark.
//
// The name column is the symbol table's (listedSymbols): 31 wide if any
// symbol's name is longer than 15, whether the cross reference lists it
// or not. Real MACRO's cross reference of testdata/mar/forth.mar is 31
// wide though no name it lists is longer than 15: the long ones are
// $FABDEF's, defined under .NOCROSS.
func (a *Assembler) symbolXrefLines(names []string) []string {
	_, width := a.listedSymbols()

	out := []string{
		fmt.Sprintf("%-*s%-13s%-16s%s", width+1, "SYMBOL", "VALUE", "DEFINITION", "REFERENCES... "),
		fmt.Sprintf("%-*s%-13s%-16s%s", width+1, "------", "-----", "----------", "------------- "),
	}

	for _, name := range names {
		s, _ := a.symbols.find(name)
		e := a.xref.entries[xrefSymbols][name]

		defined := s.defined()

		assign := ' '
		if defined && s.flags&(SymLabel|SymEntry) == 0 {
			assign = '='
		}

		value := uint32(0)
		if defined {
			value = s.value
		}

		kind := "   "

		switch {
		case !defined:
			kind = "-XR"
		case s.sect != nil:
			kind = "-R "
		}

		def := strings.Repeat(" ", 16)
		if e.def != 0 {
			def = xrefRefText(xrefRef{line: e.def}, 2)
		}

		head := fmt.Sprintf("%-*s%c%08X%s%s", width, name, assign, value, kind, def)
		out = append(out, xrefRows(head, e.refs, 2)...)
	}

	return out
}

// macroXrefLines returns the macro cross reference: each macro's name, its
// size (see macroSize), the line that defined it (for a library's, the
// line that loaded it; see loadLibraryMacro), and the lines that called
// it.
func (x *crossRef) macroXrefLines(names []string) []string {
	out := []string{
		"MACRO             SIZE          DEFINITION       REFERENCES... ",
		"-----             ----          ----------       ------------- ",
	}

	for _, name := range names {
		e := x.entries[xrefMacros][name]

		def := strings.Repeat(" ", 17)
		if e.def != 0 {
			def = xrefRefText(xrefRef{line: e.def}, 3)
		}

		head := fmt.Sprintf("%-18s%-11d%s", name, max(e.size, 1), def)
		out = append(out, xrefRows(head, e.refs, 3)...)
	}

	return out
}

// opcodeXrefLines returns the opcode cross reference: each instruction's
// name as written, its opcode, and the lines that used it.
func (x *crossRef) opcodeXrefLines(names []string) []string {
	out := []string{
		"OPCODE         VALUE     REFERENCES... ",
		"------         -----     ------------- ",
	}

	for _, name := range names {
		e := x.entries[xrefOpcodes][name]
		head := fmt.Sprintf("%-15s%04X      ", name, e.value)
		out = append(out, xrefRows(head, e.refs, 0)...)
	}

	return out
}

// directiveXrefLines returns the directive cross reference: each
// directive's name as written, and the lines that used it.
func (x *crossRef) directiveXrefLines(names []string) []string {
	out := []string{
		"DIRECTIVE      REFERENCES... ",
		"---------      ------------- ",
	}

	for _, name := range names {
		out = append(out, xrefRows(fmt.Sprintf("%-15s", name), x.entries[xrefDirectives][name].refs, 0)...)
	}

	return out
}

// registerXrefLines returns the register cross reference: each register's
// name as written, how many lines referred to it, and those lines, each
// with its mark.
func (x *crossRef) registerXrefLines(names []string) []string {
	out := []string{
		"REGISTER  NO. REFERENCES     REFERENCES... ",
		"--------  --------------     ------------- ",
	}

	for _, name := range names {
		e := x.entries[xrefRegisters][name]
		head := fmt.Sprintf("%-10s%10d%9s", name, len(e.refs), "")
		out = append(out, xrefRows(head, e.refs, 2)...)
	}

	return out
}
