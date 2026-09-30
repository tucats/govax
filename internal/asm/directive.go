package asm

import (
	"github.com/tucats/govax/internal/vmserrors"
)

// Dialect is the source language an Assembler accepts. The two share one
// core (one expression evaluator, instruction encoder, symbol table, and
// directive table), and differ in which directives they allow and, as
// docs/PHASE-27.md's later subtasks land, in how they place and relocate
// what they assemble.
type Dialect int

const (
	// DialectConsole is the console's ASM language, the default: MACRO-32
	// plus eVAX's directives for assembling straight into emulated memory
	// (.REGION, .SCB, .SHIM, .P1VECTOR, .FAB, and so on).
	DialectConsole Dialect = iota
	// DialectMACRO is VAX MACRO-32 itself, as the MACRO command assembles
	// it into an object module. It rejects the eVAX-only directives, and
	// its directives must be written with their leading ".".
	DialectMACRO
)

// SetDialect selects the source language. Only meaningful before Assemble
// is called. The MACRO dialect assembles into MACRO-32's default psects
// (see macroSections) instead of the console's P0 and S0.
func (a *Assembler) SetDialect(d Dialect) {
	a.dialect = d

	if d == DialectMACRO {
		a.macroSections()
		a.enabled = enableGlobal | enableTraceback
		a.defaultDisp = 4
	}
}

// Dialect reports the source language being assembled.
func (a *Assembler) Dialect() Dialect { return a.dialect }

// dialects is a set of Dialect values, one bit each.
type dialects uint8

const (
	console dialects = 1 << DialectConsole
	macro   dialects = 1 << DialectMACRO
	both             = console | macro
)

func (d dialects) has(dialect Dialect) bool { return d&(1<<dialect) != 0 }

// directive is one entry in the directive table: which dialects allow it,
// and the function that assembles it, given the cursor just past its name.
type directive struct {
	dialects dialects
	assemble func(a *Assembler, c *cursor) error
}

// directives is every directive name (without its leading "."), plus the
// bare mnemonic aliases JEQL/JEQLU/JNEQ/JNEQU, matching asm_pseudo.c's
// pseudos[] table. assemblePseudo tries it on every statement before the
// instruction table, so a name has to be excluded here to ever reach a
// real instruction.
//
// A directive allowed in the MACRO dialect has MACRO-32's own syntax and
// meaning. The rest are eVAX's: some can't be expressed in an object
// module at all (.REGION, .SCB, .SHIM, .P1VECTOR, .CONSOLE). .ALIGN and
// .MASK mean something else in MACRO-32 (.ALIGN's operand is a power of
// two, and .MASK reserves a transfer vector's mask word), so each dialect
// has its own form (see byDialect). .PRINT stays console-only until the
// MACRO dialect has its own.
//
// Not implemented: the privileged-register pseudo-ops (".KSP value", etc.)
// and .MODE/.PTE — none are used by any testdata/asm fixture, and each
// needs live VAX/console state (a mode stack, real page tables) this batch
// assembler has no model of. .SYM is recognized (so it doesn't fall
// through to the opcode table) but is a no-op, matching asm_pseudo.c's own
// switch, which has a pseudos[] entry for "SYM" (code 23) with no
// corresponding case — a pre-existing dead pseudo-op in the reference
// tool, replicated as-is since it's harmless either way.
//
// It's filled in by init, since some directives (.IIF, .INCLUDE) assemble
// statements themselves, which refers back to this table.
var directives map[string]directive

func init() {
	directives = map[string]directive{
		// Data storage.
		"BYTE":  {both, func(a *Assembler, c *cursor) error { return a.pseudoData(c, 1) }},
		"WORD":  {both, func(a *Assembler, c *cursor) error { return a.pseudoData(c, 2) }},
		"LONG":  {both, func(a *Assembler, c *cursor) error { return a.pseudoData(c, 4) }},
		"QUAD":  {both, (*Assembler).pseudoQuad},
		"ASCII": {both, func(a *Assembler, c *cursor) error { return a.pseudoAscii(c, asciiPlain) }},
		"ASCIZ": {both, func(a *Assembler, c *cursor) error { return a.pseudoAscii(c, asciiZ) }},
		"ASCIC": {both, func(a *Assembler, c *cursor) error { return a.pseudoAscii(c, asciiCounted) }},
		"ASCID": {both, func(a *Assembler, c *cursor) error { return a.pseudoAscii(c, asciiDescriptor) }},

		// Location control.
		"BLKB": {both, func(a *Assembler, c *cursor) error { return a.pseudoBlock(c, 1) }},
		"BLKW": {both, func(a *Assembler, c *cursor) error { return a.pseudoBlock(c, 2) }},
		"BLKL": {both, func(a *Assembler, c *cursor) error { return a.pseudoBlock(c, 4) }},
		"BLKF": {both, func(a *Assembler, c *cursor) error { return a.pseudoBlock(c, 4) }},
		"BLKD": {both, func(a *Assembler, c *cursor) error { return a.pseudoBlock(c, 8) }},
		"END":  {both, (*Assembler).pseudoEnd},

		// MACRO-32 forms of names the console dialect uses for eVAX's own
		// directives.
		"ALIGN": {both, byDialect((*Assembler).pseudoAlign, (*Assembler).pseudoAlignMACRO)},
		"MASK":  {both, byDialect((*Assembler).pseudoMask, (*Assembler).pseudoMaskMACRO)},

		// More data storage.
		"ADDRESS":    {both, (*Assembler).pseudoAddress},
		"F_FLOATING": {both, func(a *Assembler, c *cursor) error { return a.pseudoFloat(c, 4) }},
		"FLOAT":      {both, func(a *Assembler, c *cursor) error { return a.pseudoFloat(c, 4) }},
		"D_FLOATING": {both, func(a *Assembler, c *cursor) error { return a.pseudoFloat(c, 8) }},
		"DOUBLE":     {both, func(a *Assembler, c *cursor) error { return a.pseudoFloat(c, 8) }},

		// Module identification. Listings will use .SUBTITLE's text.
		"TITLE":    {both, (*Assembler).pseudoTitle},
		"IDENT":    {both, (*Assembler).pseudoIdent},
		"SUBTITLE": {both, ignoreRest},
		"SBTTL":    {both, ignoreRest},

		// Program sections.
		"PSECT":         {macro, (*Assembler).pseudoPsect},
		"SAVE_PSECT":    {macro, (*Assembler).pseudoSavePsect},
		"SAVE":          {macro, (*Assembler).pseudoSavePsect},
		"RESTORE_PSECT": {macro, (*Assembler).pseudoRestorePsect},
		"RESTORE":       {macro, (*Assembler).pseudoRestorePsect},

		// Global and external symbols.
		"GLOBAL":   {macro, func(a *Assembler, c *cursor) error { return a.declareSymbols(c, SymGlobal) }},
		"GLOBL":    {macro, func(a *Assembler, c *cursor) error { return a.declareSymbols(c, SymGlobal) }},
		"EXTERNAL": {macro, func(a *Assembler, c *cursor) error { return a.declareSymbols(c, SymGlobal) }},
		"EXTRN":    {macro, func(a *Assembler, c *cursor) error { return a.declareSymbols(c, SymGlobal) }},
		"WEAK":     {macro, func(a *Assembler, c *cursor) error { return a.declareSymbols(c, SymGlobal|SymWeak) }},

		// Assembler functions.
		"ENABLE":  {macro, func(a *Assembler, c *cursor) error { return a.pseudoEnable(c, true) }},
		"ENABL":   {macro, func(a *Assembler, c *cursor) error { return a.pseudoEnable(c, true) }},
		"DISABLE": {macro, func(a *Assembler, c *cursor) error { return a.pseudoEnable(c, false) }},
		"DSABL":   {macro, func(a *Assembler, c *cursor) error { return a.pseudoEnable(c, false) }},
		"DEFAULT": {macro, (*Assembler).pseudoDefault},

		// Routine entry points.
		"ENTRY": {both, (*Assembler).pseudoEntry},

		// Conditional assembly.
		"IF":            {both, (*Assembler).pseudoIf},
		"IF_FALSE":      {both, subconditional("IF_FALSE")},
		"IFF":           {both, subconditional("IFF")},
		"IF_TRUE":       {both, subconditional("IF_TRUE")},
		"IFT":           {both, subconditional("IFT")},
		"IF_TRUE_FALSE": {both, subconditional("IF_TRUE_FALSE")},
		"IFTF":          {both, subconditional("IFTF")},
		"ENDC":          {both, subconditional("ENDC")},
		"IIF":           {both, (*Assembler).pseudoIif},

		// Macros (macros.go). Both dialects: the console's ASM gets
		// macros too, since the two share the assembler core.
		"MACRO":   {both, (*Assembler).pseudoMacro},
		"ENDM":    {both, (*Assembler).pseudoEndm},
		"MEXIT":   {both, (*Assembler).pseudoMexit},
		"MDELETE": {both, (*Assembler).pseudoMdelete},

		// Not a MACRO-32 directive (it has .LIBRARY and .MCALL instead),
		// but the MACRO command resolves .INCLUDE across host and ODS-2
		// files (docs/PHASE-27.md, subtask 10).
		"INCLUDE": {both, (*Assembler).pseudoInclude},

		// eVAX console directives, and eVAX forms of MACRO-32 names.
		"PRINT":   {console, (*Assembler).pseudoPrint},
		"F_FLOAT": {console, func(a *Assembler, c *cursor) error { return a.pseudoFloat(c, 4) }},
		"D_FLOAT": {console, func(a *Assembler, c *cursor) error { return a.pseudoFloat(c, 8) }},
		"SPACE":   {console, (*Assembler).pseudoSpace},
		"BASE":    {console, (*Assembler).pseudoBase},
		"SET":     {console, (*Assembler).pseudoSet},
		"CLEAR":   {console, (*Assembler).pseudoClear},
		"CASE":    {console, (*Assembler).pseudoCase},
		"SCOPE":   {console, (*Assembler).pseudoScope},
		"SYM":     {console, func(*Assembler, *cursor) error { return nil }}, // a no-op; see above
		"PSL":     {console, ignoreExpression},
		"DATA":    {console, ignoreExpression},
		"TEXT":    {console, ignoreExpression},
		"VERSION": {console, (*Assembler).pseudoVersion},
		"RMSDEF":  {console, (*Assembler).pseudoRMSDEF},
		"FAB":     {console, (*Assembler).pseudoFAB},
		"RAB":     {console, (*Assembler).pseudoRAB},
		"CONSOLE": {console, (*Assembler).pseudoConsole},

		// eVAX microkernel directives.
		"MICROKERNEL": {console, func(a *Assembler, _ *cursor) error { a.microkernel = true; return nil }},
		"SCB":         {console, (*Assembler).pseudoSCB},
		"VECTOR":      {console, func(*Assembler, *cursor) error { return vmserrors.New(vmserrors.VAX_NOTLIVE, ".VECTOR") }},
		"REGION":      {console, (*Assembler).pseudoRegion},
		"SHIM":        {console, (*Assembler).pseudoShim},
		"P1VECTOR":    {console, (*Assembler).pseudoP1Vector},

		// The Posix/UNIX VAX instruction set's long conditional jumps,
		// each an inverted short branch around a JMP.
		"JEQL":  {console, func(a *Assembler, c *cursor) error { return a.pseudoJcc(c, 0x12) }}, // BNEQ
		"JEQLU": {console, func(a *Assembler, c *cursor) error { return a.pseudoJcc(c, 0x12) }},
		"JNEQ":  {console, func(a *Assembler, c *cursor) error { return a.pseudoJcc(c, 0x13) }}, // BEQL
		"JNEQU": {console, func(a *Assembler, c *cursor) error { return a.pseudoJcc(c, 0x13) }},
	}
}

// subconditional returns the directive function for one of the
// subconditional directives (.IF_FALSE, .ENDC, ...), which share one
// implementation told apart by name.
func subconditional(name string) func(a *Assembler, c *cursor) error {
	return func(a *Assembler, _ *cursor) error { return a.subconditional(name) }
}

// byDialect returns a directive function that assembles the console
// dialect's form of a directive with consoleForm, and MACRO-32's with
// macroForm, for a name the two languages use differently.
func byDialect(consoleForm, macroForm func(*Assembler, *cursor) error) func(*Assembler, *cursor) error {
	return func(a *Assembler, c *cursor) error {
		if a.dialect == DialectMACRO {
			return macroForm(a, c)
		}

		return consoleForm(a, c)
	}
}

// ignoreRest assembles a directive whose operand only matters to a
// listing: .SUBTITLE and .SBTTL.
func ignoreRest(_ *Assembler, c *cursor) error {
	c.pos = len(c.s)

	return nil
}

// ignoreExpression assembles a directive that evaluates its operand and
// does nothing with it: .PSL, .DATA, and .TEXT, which mattered only to the
// reference tool's live console.
func ignoreExpression(a *Assembler, c *cursor) error {
	_, err := a.exprNoForward(c)

	return err
}

// assemblePseudo tries to assemble the statement at c as a directive,
// matching asm_pseudo(). In the console dialect a leading "." is optional
// (so JEQL/JNEQ work both with and without one); in the MACRO dialect it's
// required, as in MACRO-32. The name is matched case-sensitively against
// directives (the line is already uppercased by this point). Reports
// handled=false, leaving c untouched, if name isn't a directive, and the
// caller falls back to the instruction table.
func (a *Assembler) assemblePseudo(c *cursor) (handled bool, err error) {
	save := c.pos
	c.skipBlanks()

	dotted := c.peek() == '.'
	if dotted {
		c.next()
	}

	start := c.pos

	for !c.atEnd() && !isBlank(c.peek()) && c.peek() != '/' {
		c.pos++
	}

	name := c.s[start:c.pos]

	d, ok := directives[name]
	if !ok || (a.dialect == DialectMACRO && !dotted) {
		c.pos = save

		return false, nil
	}

	if !d.dialects.has(a.dialect) {
		if a.dialect == DialectMACRO {
			return true, vmserrors.New(vmserrors.VAX_NOTMACRO, "."+name)
		}

		return true, vmserrors.New(vmserrors.VAX_MACROONLY, "."+name)
	}

	// Any directive other than .CASE empties the running .CASE block base,
	// matching asm_pseudo.c's own reset ahead of its switch.
	if name != "CASE" {
		a.caseBase = 0
	}

	return true, d.assemble(a, c)
}
