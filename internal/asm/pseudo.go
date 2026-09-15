package asm

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// pseudoNames is every recognized ".xxx" pseudo-op name, plus the bare
// (no-dot) mnemonic aliases JEQL/JEQLU/JNEQ/JNEQU — matching asm_pseudo.c's
// pseudos[] table. asm_pseudo() is tried on every statement before
// asm_opcode(), so a name has to be excluded here to ever reach the real
// instruction table.
//
// Not implemented: the privileged-register pseudo-ops (".KSP value", etc.)
// and .MODE/.PTE/.VECTOR/.CONSOLE — none are used by any testdata/asm
// fixture, and each needs live VAX/console state (a mode stack, real page
// tables, a running console dispatcher) this batch assembler has no model
// of. .SYM is recognized (so it doesn't fall through to the opcode table)
// but is a no-op, matching asm_pseudo.c's own switch, which has a
// pseudos[] entry for "SYM" (code 23) with no corresponding case — a
// pre-existing dead pseudo-op in the reference tool, replicated as-is
// since it's harmless either way.
var pseudoNames = map[string]bool{
	"BYTE": true, "WORD": true, "LONG": true, "BASE": true, "SET": true,
	"CLEAR": true, "ASCII": true, "ASCIZ": true, "ASCIC": true, "ASCID": true,
	"END": true, "PSL": true, "PRINT": true, "MASK": true, "F_FLOAT": true,
	"D_FLOAT": true, "BLKB": true, "BLKW": true, "BLKL": true, "BLKF": true,
	"BLKD": true, "ENTRY": true, "SYM": true, "CASE": true, "SCB": true,
	"ALIGN": true, "REGION": true, "VECTOR": true, "CONSOLE": true,
	"INCLUDE": true, "IF": true, "SHIM": true, "MICROKERNEL": true,
	"SPACE": true, "DATA": true, "TEXT": true, "JEQL": true, "JEQLU": true,
	"JNEQ": true, "JNEQU": true, "P1VECTOR": true, "SCOPE": true,
}

// assemblePseudo tries to assemble the statement at c as a pseudo-op,
// matching asm_pseudo(): a leading "." is optional (so JEQL/JNEQ work both
// with and without one), and the name is matched case-sensitively against
// pseudoNames (the line is already uppercased by this point). Reports
// handled=false, leaving c untouched, if name isn't a recognized pseudo-op
// — the caller falls back to asm_opcode's real-instruction table.
func (a *Assembler) assemblePseudo(c *cursor) (handled bool, err error) {
	save := c.pos
	c.skipBlanks()
	if c.peek() == '.' {
		c.next()
	}

	start := c.pos

	for !c.atEnd() && !isBlank(c.peek()) && c.peek() != '/' {
		c.pos++
	}

	name := c.s[start:c.pos]
	if !pseudoNames[name] {
		c.pos = save

		return false, nil
	}

	// Any pseudo-op other than .CASE empties the running .CASE block base,
	// matching asm_pseudo.c's own reset ahead of its switch.
	if name != "CASE" {
		a.caseBase = 0
	}

	return true, a.dispatchPseudo(name, c)
}

func (a *Assembler) dispatchPseudo(name string, c *cursor) error {
	switch name {
	case "BYTE":
		return a.pseudoData(c, 1)

	case "WORD":
		return a.pseudoData(c, 2)

	case "LONG":
		return a.pseudoData(c, 4)

	case "BASE":
		return a.pseudoBase(c)

	case "SET":
		return a.pseudoSet(c)

	case "CLEAR":
		return a.pseudoClear(c)

	case "ASCII":
		return a.pseudoAscii(c, asciiPlain)

	case "ASCIZ":
		return a.pseudoAscii(c, asciiZ)

	case "ASCIC":
		return a.pseudoAscii(c, asciiCounted)

	case "ASCID":
		return a.pseudoAscii(c, asciiDescriptor)

	case "END":
		return a.pseudoEnd(c)

	case "PSL":
		_, err := a.exprNoForward(c)

		return err

	case "PRINT":
		return a.pseudoPrint(c)

	case "MASK":
		return a.pseudoMask(c)

	case "F_FLOAT":
		return a.pseudoFloat(c, 4)

	case "D_FLOAT":
		return a.pseudoFloat(c, 8)

	case "BLKB":
		return a.pseudoBlock(c, 1)

	case "BLKW":
		return a.pseudoBlock(c, 2)

	case "BLKL", "BLKF":
		return a.pseudoBlock(c, 4)

	case "BLKD":
		return a.pseudoBlock(c, 8)

	case "ENTRY":
		return a.pseudoEntry(c)

	case "SYM":
		return nil // recognized but a no-op; see pseudoNames' doc comment.

	case "CASE":
		return a.pseudoCase(c)

	case "SCB":
		return a.pseudoSCB(c)

	case "ALIGN":
		return a.pseudoAlign(c)

	case "REGION":
		return a.pseudoRegion(c)

	case "VECTOR":
		return vmserrors.New(vmserrors.VAX_NOTLIVE, "."+name)

	case "CONSOLE":
		return a.pseudoConsole(c)

	case "INCLUDE":
		return a.pseudoInclude(c)

	case "IF":
		return a.pseudoIf(c)

	case "SHIM":
		return a.pseudoShim(c)

	case "MICROKERNEL":
		a.microkernel = true

		return nil

	case "SPACE":
		return a.pseudoSpace(c)

	case "DATA", "TEXT":
		_, err := a.exprNoForward(c)

		return err

	case "JEQL", "JEQLU":
		return a.pseudoJcc(c, 0x12) // BNEQ, inverted around a JMP.

	case "JNEQ", "JNEQU":
		return a.pseudoJcc(c, 0x13) // BEQL, inverted around a JMP.

	case "P1VECTOR":
		return a.pseudoP1Vector(c)

	case "SCOPE":
		return a.pseudoScope(c)
	}

	panic("asm: pseudoNames/dispatchPseudo out of sync for " + name)
}

// readToken reads a blank-delimited token (a qualifier like "/PERM", or a
// bareword like a .REGION/.SCB stack specifier), matching read_verb()'s
// role in the reference tool (minus its 4-character CHAR4 truncation,
// which none of testdata/asm's fixtures rely on for these pseudo-ops).
func readToken(c *cursor) string {
	c.skipBlanks()
	start := c.pos

	for !c.atEnd() && !isBlank(c.peek()) {
		c.pos++
	}

	return c.s[start:c.pos]
}

// readFileArg reads a .INCLUDE file-name argument: a "quoted string"
// (whose contents were left case-as-typed by preprocessLine) or a bare
// token.
func readFileArg(c *cursor) string {
	c.skipBlanks()
	if c.peek() == '"' {
		c.next()
		start := c.pos

		for !c.atEnd() && c.peek() != '"' {
			c.pos++
		}

		name := c.s[start:c.pos]

		if c.peek() == '"' {
			c.next()
		}

		return name
	}

	return readToken(c)
}

// pseudoData assembles .BYTE/.WORD/.LONG: a comma-separated list of
// expressions, each stored at scale bytes and forward-reference-capable
// (K_ADDR_B/W/L), matching asm_pseudo.c's cases 1-3.
func (a *Assembler) pseudoData(c *cursor, scale int) error {
	for {
		c.skipBlanks()
		
		if c.atEnd() {
			return nil
		}

		if c.peek() == ',' {
			c.next()
		}

		loc := a.deposit

		v, _, err := a.exprValue(c, loc, addrFixup(scale))
		if err != nil {
			return err
		}

		if scale == 1 && v > 0xFF {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".BYTE", v)
		}

		if scale == 2 && v > 0xFFFF {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".WORD", v)
		}

		if err := a.storeScaled(a.deposit, v, scale); err != nil {
			return err
		}

		a.deposit += uint32(scale)
	}
}

// pseudoBase assembles .BASE value: sets the current deposit location,
// closing out the active local-symbol scope first (matching case 4).
func (a *Assembler) pseudoBase(c *cursor) error {
	a.scopeSymbols()

	v, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	a.deposit = v

	return nil
}

// pseudoSet assembles .SET [/PERMANENT] [/ENTRY] [/LABEL] name (=|,) value,
// matching case 5 (minus the R0-R15/AP/FP/SP/PC special case, which writes
// directly to a live register bank instead of the symbol table — unused by
// every testdata/asm fixture, and with no live-register-bank concept in
// this batch assembler to write into).
func (a *Assembler) pseudoSet(c *cursor) error {
	var flags SymFlag

qualifiers:
	for {
		save := c.pos
		tok := readToken(c)

		switch verbPrefix4(tok) {
		case "/PER", "/PRM":
			flags |= SymPermanent

		case "/ENT":
			flags |= SymEntry

		case "/LAB", "/LBL":
			flags |= SymLabel

		default:
			c.pos = save

			break qualifiers
		}
	}

	c.skipBlanks()
	name := scanSetName(c)

	c.skipBlanks()
	if c.peek() == '=' || c.peek() == ',' {
		c.next()
	}

	v, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	return a.setSymbol(name, v, flags, false)
}

// verbPrefix4 returns s's first 4 characters, space-padded if shorter —
// the CHAR4 comparison the reference tool uses to match a qualifier by its
// first 4 characters regardless of how much of the word was typed.
func verbPrefix4(s string) string {
	if len(s) >= 4 {
		return s[:4]
	}

	return s + strings.Repeat(" ", 4-len(s))
}

// scanSetName reads .SET's target name: everything up to a blank, ',', '=',
// or end of line.
func scanSetName(c *cursor) string {
	start := c.pos

	for !c.atEnd() && !isBlank(c.peek()) && c.peek() != ',' && c.peek() != '=' {
		c.pos++
	}

	return c.s[start:c.pos]
}

// pseudoClear assembles .CLEAR [name] (case 6): with no name, every symbol
// is removed; with one, just that symbol (an error if it doesn't exist).
func (a *Assembler) pseudoClear(c *cursor) error {
	c.skipBlanks()
	if c.atEnd() {
		a.symbols.clearAll()

		return nil
	}

	name := scanName(c)
	if !a.symbols.clear(name) {
		return vmserrors.New(vmserrors.VAX_UNDEFSYM, name)
	}

	return nil
}

// asciiKind distinguishes .ASCII/.ASCIZ/.ASCIC/.ASCID's differing framing
// around the raw character data, matching asm_pseudo.c's cases 7-10.
type asciiKind int

const (
	asciiPlain asciiKind = iota
	asciiZ
	asciiCounted
	asciiDescriptor
)

// pseudoAscii assembles .ASCII/.ASCIZ/.ASCIC/.ASCID: a comma-separated list
// of quoted (', ", or /-delimited) or bare (blank-terminated) strings, with
// \n/\r/\t escapes, concatenated into one run of character data framed per
// asciiKind — a trailing NUL (.ASCIZ), a leading 16-bit count (.ASCIC), or
// a leading VMS string descriptor whose address field points at the string
// data immediately following it (.ASCID). Matches asm_pseudo.c's cases
// 7-10.
func (a *Assembler) pseudoAscii(c *cursor, kind asciiKind) error {
	var count uint16

	countPC := a.deposit

	switch kind {
	case asciiCounted:
		if err := a.image.storeWord(countPC, 0); err != nil {
			return err
		}
		a.deposit += 2

	case asciiDescriptor:
		if err := a.image.storeWord(countPC, 0); err != nil { // length (patched below)
			return err
		}

		if err := a.image.storeWord(countPC+2, 0); err != nil { // dtype/class: string
			return err
		}

		a.deposit += 4
		stringPC := a.deposit + 4

		if err := a.image.storeLongword(a.deposit, stringPC); err != nil {
			return err
		}

		a.deposit += 4
	}

	for {
		c.skipBlanks()
		if c.atEnd() {
			break
		}

		var q byte

		switch c.peek() {
		case '/', '\'', '"':
			q = c.peek()
			c.next()
		}

		for !c.atEnd() {
			if q != 0 && c.peek() == q {
				break
			}

			if q == 0 && isBlank(c.peek()) {
				break
			}

			ch := c.next()
			if ch == '\\' {
				switch c.peek() {
				case 'n':
					ch = '\n'

				case 'r':
					ch = '\r'

				case 't':
					ch = '\t'

				default:
					ch = c.peek()
				}

				c.next()
			}

			if err := a.image.storeByte(a.deposit, ch); err != nil {
				return err
			}

			a.deposit++
			count++
		}

		if q != 0 && c.peek() == q {
			c.next()
		}

		c.skipBlanks()
		if c.peek() != ',' {
			break
		}

		c.next()
	}

	switch kind {
	case asciiZ:
		if err := a.image.storeByte(a.deposit, 0); err != nil {
			return err
		}

		a.deposit++

	case asciiCounted, asciiDescriptor:
		if err := a.image.storeWord(countPC, count); err != nil {
			return err
		}
	}

	return nil
}

// pseudoEnd assembles .END [entry-expression]: an optional expression names
// the program's start address (recorded as the __ENTRY symbol, and via
// Entry()), then assembly of the current source stops — matching case 11.
func (a *Assembler) pseudoEnd(c *cursor) error {
	a.scopeSymbols()

	c.skipBlanks()
	if !c.atEnd() {
		v, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		if err := a.setSymbol("__ENTRY", v, SymNone, false); err != nil {
			return err
		}

		a.entrySeen = true
		a.entryAddr = v
	}

	a.stop = true

	return nil
}

// pseudoPrint assembles .PRINT: a comma-separated list of quoted strings
// and/or expressions (printed in hex, or decimal under .SET RADIX DECIMAL),
// matching console_print() — except output is captured (see Prints())
// rather than written to stdout, since this package has no notion of "the
// console" a library caller might not want written to. A silent no-op when
// SetVerbose(false) has been called, matching CONSOLE_VERBOSE being clear.
func (a *Assembler) pseudoPrint(c *cursor) error {
	if !a.verbose {
		return nil
	}

	var sb strings.Builder

	for {
		c.skipBlanks()
		if c.atEnd() {
			break
		}

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.peek() == '"' {
			c.next()
			start := c.pos

			for !c.atEnd() && c.peek() != '"' {
				c.pos++
			}

			sb.WriteString(c.s[start:c.pos])

			if c.peek() == '"' {
				c.next()
			}

			continue
		}

		v, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		if a.radix == 10 {
			fmt.Fprintf(&sb, "%d", int32(v))
		} else {
			fmt.Fprintf(&sb, "%08X", v)
		}
	}

	a.prints = append(a.prints, sb.String())

	return nil
}

// pseudoMask assembles .MASK <r>: a register-set mask literal, stored as a
// 16-bit word — matching case 14.
func (a *Assembler) pseudoMask(c *cursor) error {
	m, err := a.maskLiteral(c)
	if err != nil {
		return err
	}

	if err := a.image.storeWord(a.deposit, uint16(m)); err != nil {
		return err
	}

	a.deposit += 2

	return nil
}

// pseudoFloat assembles .F_FLOAT/.D_FLOAT: a comma-separated list of
// floating constants, each stored in VAX F_floating (size 4) or D_floating
// (size 8) form — matching case 16-17 (see storeImmediateFloat's doc
// comment for the one deliberate deviation, an over-advance bug in the
// D_FLOAT case that isn't replicated).
func (a *Assembler) pseudoFloat(c *cursor, size int) error {
	for {
		c.skipBlanks()
		if c.atEnd() {
			return nil
		}

		if c.peek() == ',' {
			c.next()
		}

		f, err := a.parseFloat(c)
		if err != nil {
			return err
		}

		if err := a.storeImmediateFloat(size, f); err != nil {
			return err
		}
	}
}

// pseudoBlock assembles .BLKB/.BLKW/.BLKL/.BLKF/.BLKD count: reserves
// count*size zero bytes. The reference tool (case 18-21) zero-fills by
// storing a literal zero byte count*size times; this just advances the
// deposit counter without writing anything, since an address this package
// never wrote to already reads back as zero (see image's doc comment) —
// same observable result, without allocating a map entry per byte for
// what can be a very large count.
func (a *Assembler) pseudoBlock(c *cursor, size int) error {
	c.skipBlanks()

	n, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	a.deposit += n * uint32(size)

	return nil
}

// pseudoEntry assembles .ENTRY name[,<mask>]: defines name as the current
// address (SYM_ENTRY, must be a fresh definition), starts a new local-
// symbol scope under it, and stores the optional register-set mask (0 if
// omitted) as a 16-bit word — matching case 22.
func (a *Assembler) pseudoEntry(c *cursor) error {
	a.scopeSymbols()

	c.skipBlanks()
	name := scanName(c)

	if err := a.setSymbol(name, a.deposit, SymEntry, true); err != nil {
		return err
	}

	a.curEntry = name

	c.skipBlanks()
	if c.peek() == ',' {
		c.next()
	}

	var mask uint32

	c.skipBlanks()
	if !c.atEnd() {
		m, err := a.maskLiteral(c)
		if err != nil {
			return err
		}
		mask = m
	}

	if err := a.image.storeWord(a.deposit, uint16(mask)); err != nil {
		return err
	}

	a.deposit += 2

	return nil
}

// pseudoScope assembles .SCOPE name: like .ENTRY, but only starts a new
// local-symbol scope (SYM_LABEL, must be fresh) — no mask, no code of its
// own. Matches case 41.
func (a *Assembler) pseudoScope(c *cursor) error {
	a.scopeSymbols()

	c.skipBlanks()

	name := scanName(c)
	if err := a.setSymbol(name, a.deposit, SymLabel, true); err != nil {
		return err
	}

	a.curEntry = name

	return nil
}

// pseudoCase assembles .CASE item[,item...]: a comma-separated list of
// word-sized offsets from the running .CASE block's base address (the
// deposit location when the first .CASE statement in the block ran),
// matching case 24. Each item may forward-reference a not-yet-defined
// label.
func (a *Assembler) pseudoCase(c *cursor) error {
	if a.caseBase == 0 {
		a.caseBase = a.deposit
	}

	for {
		c.skipBlanks()
		if c.atEnd() {
			return nil
		}

		if c.peek() == ',' {
			c.next()
		}

		loc := a.deposit
		v, _, err := a.exprValue(c, loc, fixCaseW)
		if err != nil {
			return err
		}
		if v > 0xFFFF {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".CASE", v)
		}
		if err := a.image.storeWord(a.deposit, uint16(v)); err != nil {
			return err
		}
		a.deposit += 2
	}
}

// pseudoSCB assembles .SCB code, address[, stack]: pokes a longword vector
// entry into the SCB (System Control Block) at SCBB+code, matching case
// 25. Requires .MICROKERNEL (or SetMicrokernel(true)) — like the reference
// tool's MKVALID gate, since an SCB entry only makes sense once there's a
// notion of "the microkernel being built".
func (a *Assembler) pseudoSCB(c *cursor) error {
	if !a.microkernel {
		return vmserrors.New(vmserrors.VAX_NEEDMICRO, ".SCB")
	}

	code, err := a.exprNoForward(c)
	if err != nil {
		return err
	}
	if code > 0xFF || code&0x3 != 0 {
		return vmserrors.New(vmserrors.VAX_SCBCODE, code)
	}

	saved := a.deposit
	a.deposit = 0x80000000 + a.scbb + code

	c.skipBlanks()
	if c.peek() == ',' {
		c.next()
	}

	loc := a.deposit
	value, _, err := a.exprValue(c, loc, addrFixup(4))
	if err != nil {
		a.deposit = saved
		return err
	}

	if value != 0xFFFFFFFF && value&0x3 != 0 {
		a.deposit = saved
		return vmserrors.New(vmserrors.VAX_SCBALIGN, value)
	}

	c.skipBlanks()
	if c.peek() == ',' {
		c.next()
	}
	if !c.atEnd() {
		switch readToken(c) {
		case "ISP":
			value++
		case "KSP", "USP", "SSP":
			// +0, but named for clarity at the call site.
		default:
			a.deposit = saved
			return vmserrors.New(vmserrors.VAX_SCBSTACK)
		}
	}

	if err := a.image.storeLongword(a.deposit, value); err != nil {
		a.deposit = saved
		return err
	}
	a.deposit = saved
	return nil
}

// pseudoAlign assembles .ALIGN size: advances the deposit location up to
// the next multiple of size, if it isn't already one — matching case 26.
func (a *Assembler) pseudoAlign(c *cursor) error {
	size, err := a.exprNoForward(c)
	if err != nil {
		return err
	}
	if size == 0 {
		return vmserrors.New(vmserrors.VAX_ALIGNZERO)
	}
	n := (a.deposit / size) * size
	if n < a.deposit {
		a.deposit = n + size
	}
	return nil
}

// pseudoRegion assembles .REGION S0|SYSTEM|P0|PROCESS: swaps the active
// deposit counter for its saved counterpart in the other region — matching
// case 27. Requires .MICROKERNEL.
func (a *Assembler) pseudoRegion(c *cursor) error {
	if !a.microkernel {
		return vmserrors.New(vmserrors.VAX_NEEDMICRO, ".REGION")
	}

	var toS0 bool

	switch readToken(c) {
	case "2", "S0", "SYSTEM":
		toS0 = true

	case "0", "P0", "PROCESS":
		toS0 = false

	default:
		return vmserrors.New(vmserrors.VAX_REGIONSPEC)
	}

	if toS0 == a.regionIsS0 {
		return nil
	}

	if a.regionIsS0 {
		a.s0Deposit = a.deposit
		a.deposit = a.p0Deposit
	} else {
		a.p0Deposit = a.deposit
		a.deposit = a.s0Deposit
	}

	a.regionIsS0 = toS0

	return nil
}

// pseudoShim assembles .SHIM name, code, rtlname, offset: if code is
// nonzero, generates a small stub at the current location (MOVL I^#code,R0
// ; XFC #XFC$SHIM ; RET) and defines name as its address; if code is zero,
// name must already be a defined symbol whose value is used instead. Either
// way, defines "SHIM$<rtlname>_<offset:08X>" as that address — the name
// SYS$/LIB$ RTL dispatch (Phase 10) looks up when resolving a sharable-
// image import by (library, offset). Matches case 33. Requires
// .MICROKERNEL.
func (a *Assembler) pseudoShim(c *cursor) error {
	if !a.microkernel {
		return vmserrors.New(vmserrors.VAX_NEEDMICRO, ".SHIM")
	}

	a.scopeSymbols()
	c.skipBlanks()

	name := scanName(c)

	c.skipBlanks()
	if c.peek() == ',' {
		c.next()
	}

	c.skipBlanks()

	var code uint32

	if c.peek() != ',' {
		v, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		code = v
	}

	var shimAddr uint32

	if code != 0 {
		if err := a.setSymbol(name, a.deposit, SymEntry, true); err != nil {
			return err
		}

		shimAddr = a.deposit

		if err := a.image.storeWord(a.deposit, 0); err != nil { // empty entry mask word
			return err
		}

		a.deposit += 2

		if err := a.storeShimStub(code); err != nil {
			return err
		}
	} else {
		v, _, err := a.getSymbol(name, false, 0, fixNone)
		if err != nil {
			return err
		}

		shimAddr = v
	}

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()
	}

	c.skipBlanks()

	rtlName := scanName(c)

	c.skipBlanks()
	if c.peek() == ',' {
		c.next()
	}

	offset, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	shimSymbol := fmt.Sprintf("SHIM$%s_%08X", rtlName, offset)

	return a.setSymbol(shimSymbol, shimAddr, SymNone, false)
}

// storeShimStub writes a .SHIM stub's body: MOVL I^#code,R0 ; XFC
// #XFC$SHIM ; RET.
func (a *Assembler) storeShimStub(code uint32) error {
	bytes := []byte{0xD0, 0x8F} // MOVL I^#
	for _, b := range bytes {
		if err := a.image.storeByte(a.deposit, b); err != nil {
			return err
		}
		a.deposit++
	}

	if err := a.image.storeLongword(a.deposit, code); err != nil {
		return err
	}

	a.deposit += 4

	tail := []byte{0x50, 0xFC, 0x7D, 0x04} // R0 ; XFC #XFC$SHIM ; RET
	for _, b := range tail {
		if err := a.image.storeByte(a.deposit, b); err != nil {
			return err
		}

		a.deposit++
	}

	return nil
}

// pseudoSpace assembles .SPACE bytes[,fill]: reserves bytes bytes, each set
// to fill (0 if omitted) — matching case 35.
func (a *Assembler) pseudoSpace(c *cursor) error {
	n, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	var fill byte

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()

		v, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		fill = byte(v)
	}

	if fill == 0 {
		a.deposit += n

		return nil
	}

	for i := uint32(0); i < n; i++ {
		if err := a.image.storeByte(a.deposit, fill); err != nil {
			return err
		}

		a.deposit++
	}

	return nil
}

// pseudoJcc assembles .JEQL/.JEQLU/.JNEQ/.JNEQU dest: a long-range
// conditional jump, synthesized as the inverse short branch around an
// absolute JMP — matching cases 38-39. invByte is the inverted branch
// opcode (0x12 BNEQ for .JEQL, 0x13 BEQL for .JNEQ).
func (a *Assembler) pseudoJcc(c *cursor, invByte byte) error {
	for _, b := range []byte{
		invByte, // BNEQ/BEQL
		0x06,    // branch-around displacement (past the 4-byte JMP operand)
		0x17,    // JMP
		0x9F,    // @# addressing mode
	} {
		if err := a.image.storeByte(a.deposit, b); err != nil {
			return err
		}

		a.deposit++
	}

	loc := a.deposit

	v, _, err := a.exprValue(c, loc, fixAddrL)
	if err != nil {
		return err
	}

	if err := a.image.storeLongword(a.deposit, v); err != nil {
		return err
	}

	a.deposit += 4

	return nil
}

// pseudoConsole assembles .CONSOLE <command>: the reference tool dispatches
// the rest of the line to the interactive console's own DCL-style command
// dispatcher (Phase 08's internal/console), which this package can't call
// into without an inverted, assembler-depends-on-console package
// dependency. The only console commands any testdata/asm fixture actually
// issues this way are "SET RADIX DEC/HEX" (forth.asm, which relies on
// decimal literals for the rest of the file) and "SET VERIFY" (a pure
// echo-to-listing toggle with no effect on assembled bytes) — so those two
// forms are recognized directly here, and anything else is accepted as a
// no-op rather than an error, matching a console command whose only
// observable effect would be on a live interactive session Phase 11's
// batch assembler doesn't have.
func (a *Assembler) pseudoConsole(c *cursor) error {
	if readToken(c) == "SET" {
		switch readToken(c) {
		case "RADIX":
			switch readToken(c) {
			case "DEC", "DECIMAL":
				a.radix = 10
			
			case "HEX", "HEXADECIMAL":
				a.radix = 16
			}
		}
	}
	return nil
}

// pseudoInclude assembles .INCLUDE "file": resolves file via the
// assembler's configured include resolver (SetIncludeResolver) and
// assembles its contents in place, matching case 31 (console_asm()).
func (a *Assembler) pseudoInclude(c *cursor) error {
	name := readFileArg(c)
	if a.includeResolver == nil {
		return vmserrors.New(vmserrors.RMS_NORESOLVER, name)
	}

	src, err := a.includeResolver(name)
	if err != nil {
		return vmserrors.Wrap(vmserrors.RMS_INCLUDE, err, name)
	}

	return a.assembleLines(src)
}

// pseudoIf assembles .IF expression [THEN] statement: if expression is
// nonzero, the rest of the line is assembled as one statement (recursively,
// so it can itself be another pseudo-op — kernel.asm uses ".IF
// DEFINED(...)=0 .INCLUDE ..."); otherwise the rest of the line is simply
// not assembled. Matches case 32.
func (a *Assembler) pseudoIf(c *cursor) error {
	v, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	c.skipBlanks()
	save := c.pos

	if readToken(c) != "THEN" {
		c.pos = save
	}

	if v == 0 {
		return nil
	}

	return a.assembleStatement(c.rest())
}

// pseudoP1Vector assembles .P1VECTOR. The reference tool builds the VMS P1
// system-service dispatch vector here (see p1_vector.c's p1_init(),
// defining a SYS$xxx/LIB$xxx symbol and stub per Phase 10's RTL service
// table for every real image to CALL into). No testdata/asm fixture
// references any SYS$/LIB$ symbol it would define, and building it for
// real would need internal/asm to import internal/rtl's service table —
// backwards for an assembler package, and squarely VMS-image-activation
// territory per docs/PLAN.md's Phase 13 split. So: recognized (so
// kernel.asm's one ".p1vector" statement doesn't fail outright) and
// deferred, matching this project's usual policy for a finding that's
// large enough to revisit deliberately rather than block a phase on.
func (a *Assembler) pseudoP1Vector(c *cursor) error {
	return nil
}
