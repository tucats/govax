package asm

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmserrors"
)

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
	return a.pseudoDataSigned(c, scale, false)
}

// pseudoDataSigned is pseudoData, or with signed MACRO-32's .SIGNED_BYTE
// and .SIGNED_WORD: each value must fit the field as a signed number,
// and one the linker stores is stored signed (STO_SB, STO_SW).
func (a *Assembler) pseudoDataSigned(c *cursor, scale int, signed bool) error {
	first := true

	fixup := addrFixup(scale)
	if signed {
		fixup = signedFixup(scale)
	}

	// In MACRO-32, one with no value stores a zero: $FAB's ".WORD" (a
	// spare word) is two zero bytes in real MACRO's object
	// (testdata/mar/macros/vax/fabalign.obj).
	if c.skipBlanks(); c.atEnd() && a.dialect == DialectMACRO {
		return a.emitScaled(0, scale)
	}

	for {
		c.skipBlanks()

		if err := a.listSeparator(c, first); err != nil || c.atEnd() {
			return err
		}

		first = false

		loc := a.pc()

		v, wasForward, err := a.exprValue(c, loc, fixup)
		if err != nil {
			return err
		}

		// A value may be written signed (.BYTE -1) or as its unsigned bit
		// pattern (.BYTE 0FF); the reference tool rejected the former. A
		// forward reference's fixup checks its own value.
		if wasForward {
			v = 0
		}

		// A value too big for its field is an error; real MACRO stores
		// it truncated (DATATRUNC) and goes on.
		switch {
		case signed && scale == 1 && (int32(v) < -128 || int32(v) > 127):
			err = a.recoverable(vmserrors.New(vmserrors.VAX_DATARANGE, ".SIGNED_BYTE", int32(v)))
		case signed && scale == 2 && (int32(v) < -32768 || int32(v) > 32767):
			err = a.recoverable(vmserrors.New(vmserrors.VAX_DATARANGE, ".SIGNED_WORD", int32(v)))
		case scale == 1 && (int32(v) < -128 || int32(v) > 0xFF):
			err = a.recoverable(vmserrors.New(vmserrors.VAX_DATARANGE, ".BYTE", int32(v)))
		case scale == 2 && (int32(v) < -32768 || int32(v) > 0xFFFF):
			err = a.recoverable(vmserrors.New(vmserrors.VAX_DATARANGE, ".WORD", int32(v)))
		}

		if err != nil {
			return err
		}

		if err := a.emitScaled(v, scale); err != nil {
			return err
		}
	}
}

// listSeparator is called before each item of a data directive's list
// (.BYTE, .QUAD, .FLOAT, .CASE, ...), with blanks already skipped: items
// after the first must follow a comma, as in MACRO-32. The reference tool
// made the comma optional, so ".BYTE 0FF" in decimal would silently be
// two items, 0 and the symbol FF.
func (a *Assembler) listSeparator(c *cursor, first bool) error {
	if first || c.atEnd() {
		return nil
	}

	if c.peek() != ',' {
		return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
	}

	c.next()
	c.skipBlanks()

	return nil
}

// pseudoQuad assembles .QUAD: a comma-separated list of 64-bit values,
// each read by wideItem. The reference tool had no .QUAD at all.
func (a *Assembler) pseudoQuad(c *cursor) error {
	first := true

	for {
		c.skipBlanks()

		// VAX MACRO takes one value: a second is "Directive syntax
		// error", after the first is stored (testdata/mar/dst's
		// DSTSYM, .QUAD 1, 2; .OCTA's is assumed the same).
		if !first && !c.atEnd() && a.dialect == DialectMACRO {
			return vmserrors.New(vmserrors.VAX_DIRSYNX)
		}

		if err := a.listSeparator(c, first); err != nil || c.atEnd() {
			return err
		}

		first = false

		v, err := a.wideItem(c)
		if err != nil {
			return err
		}

		if err := a.emitLongword(uint32(v.lo)); err != nil {
			return err
		}

		if err := a.emitLongword(uint32(v.lo >> 32)); err != nil {
			return err
		}
	}
}

// wideItem reads one .QUAD or .OCTA item. An item that is a single
// numeric literal (digits in the current radix, or after a ^X, ^D, or 0X
// prefix) is read at full width; the expression evaluator works in 32
// bits, so any other expression is a longword, widened. A forward
// reference patches only the low longword, and the rest stays zero.
//
// The two dialects widen differently. VAX MACRO (the MACRO dialect)
// zero-extends an expression, and refuses a negative number with
// "Directive syntax error": its object for testdata/insn35/asm/asm35.mar
// shows both (.QUAD NEG, NEG = -3, is ^XFFFFFFFD then zeros). The console
// dialect keeps what eVAX's fixtures need: a negative number at full
// width (.QUAD -^D100000 for a delta time) and an expression
// sign-extended.
func (a *Assembler) wideItem(c *cursor) (octa, error) {
	if a.dialect == DialectMACRO {
		save := c.pos
		c.skipBlanks()
		negative := c.peek() == '-'
		c.pos = save

		if negative {
			return octa{}, vmserrors.New(vmserrors.VAX_DIRSYNX)
		}
	}

	if v, ok := a.wideLiteral(c); ok {
		return v, nil
	}

	lo, wasForward, err := a.exprValue(c, a.pc(), fixAddrL)

	switch {
	case err != nil:
		return octa{}, err
	case wasForward, a.dialect == DialectMACRO:
		return octa{lo: uint64(lo)}, nil
	}

	return signExtendOcta(lo), nil
}

// pseudoBase assembles .BASE value: sets the current deposit location,
// closing out the active local-symbol scope first (matching case 4).
func (a *Assembler) pseudoBase(c *cursor) error {
	a.scopeSymbols()

	v, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	a.setPC(v)

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

// pseudoSet assembles .VERSION "string" value, which stores the symbol
// value or "SYS$MK_VERSION".
func (a *Assembler) pseudoVersion(c *cursor) error {
	tok := readToken(c)

	v, err := strconv.Atoi(tok)
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_BADINTEGER, err, tok)
	}

	return a.setSymbol("SYS$MK_VERSION", uint32(v), SymPermanent, true)
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

// pseudoAscii assembles .ASCII/.ASCIZ/.ASCIC/.ASCID: a list of delimited
// strings and <expression> bytes (see asciiItem), concatenated into one
// run of character data framed per
// asciiKind — a trailing NUL (.ASCIZ), a leading one-byte count (.ASCIC), or
// a leading VMS string descriptor whose address field points at the string
// data immediately following it (.ASCID). Matches asm_pseudo.c's cases
// 7-10, except that .ASCIC's count is a byte, as MACRO-32 defines it (and
// as VMS's counted-string users, such as $FAO's !AC, read it), where
// eVAX stored a 16-bit word; a string longer than 255 characters is
// VAX_DATARANGE (docs/PHASE-26.md, docs/DEVIATIONS.md). The reference
// tool also took an undelimited word as a string (.ASCII TEXT), which
// MACRO-32 doesn't; that is now VAX_BADSTRING.
func (a *Assembler) pseudoAscii(c *cursor, kind asciiKind) error {
	if err := a.output(); err != nil {
		return err
	}

	count := 0

	countPC := a.pc()

	switch kind {
	case asciiCounted:
		if err := a.cur.img.storeByte(countPC, 0); err != nil {
			return err
		}

		// Real MACRO stores the count through the linker's stack, still
		// zero, as a signed byte, and stores the count once it's counted
		// the string (see the evPatch below).
		a.logEvent(outEvent{kind: evConst, sect: a.cur, offset: a.cur.loc, size: 1, signed: true})
		a.listConst(a.cur.loc, 1, 0)
		a.move(1)

	case asciiDescriptor:
		if err := a.cur.img.storeWord(countPC, 0); err != nil { // length (patched below)
			return err
		}

		if err := a.cur.img.storeWord(countPC+2, 0x010E); err != nil { // class S, dtype T
			return err
		}

		// Real MACRO stores this longword through the linker's stack, the
		// length still zero, and stores the length once it's counted the
		// string (see the evPatch below).
		a.logEvent(outEvent{kind: evConst, sect: a.cur, offset: a.cur.loc, size: 4, value: 0x010E << 16})
		a.listConst(a.cur.loc, 4, 0x010E<<16)
		a.move(4)

		// The address of the string, just past this longword: ".+4".
		if here := a.dot(); !here.known() {
			a.queueFixup(a.pc(), fixAddress, &rexpr{op: rBinary, bin: '+', l: here.x, r: constNode(4)})
		}

		if err := a.emitLongword(a.pc() + 4); err != nil {
			return err
		}
	}

	for {
		c.skipBlanks()

		if c.atEnd() {
			break
		}

		n, err := a.asciiItem(c)
		if err != nil {
			return err
		}

		count += n
	}

	switch kind {
	case asciiZ:
		if err := a.emitByte(0); err != nil {
			return err
		}

	case asciiCounted:
		if count > 0xFF {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".ASCIC", count)
		}

		if err := a.cur.img.storeByte(countPC, byte(count)); err != nil {
			return err
		}

		a.logEvent(outEvent{kind: evPatch, sect: a.cur, offset: countPC - a.cur.base, size: 1, value: uint32(count), immediate: true})
		a.listPatch(countPC-a.cur.base, 1, uint32(count))

	case asciiDescriptor:
		if count > 0xFFFF {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".ASCID", count)
		}

		if err := a.cur.img.storeWord(countPC, uint16(count)); err != nil {
			return err
		}

		a.logEvent(outEvent{kind: evPatch, sect: a.cur, offset: countPC - a.cur.base, size: 2, value: uint32(count)})
	}

	return nil
}

// asciiItem stores one item of a string directive and returns how many
// bytes it stored. An item is a string between two of the same delimiter
// character (any that isStringDelimiter accepts), or "<expression>" for a
// single byte, as in MACRO-32's .ASCII /text/<13><10>. Items may be
// separated by blanks. As in MACRO-32, a backslash is an ordinary
// character, and a comma is a delimiter like any other. The reference
// tool separated strings with commas, had C-style escapes (\n, \r, \t)
// for control characters, and stopped at anything else without an error,
// silently dropping the rest of the line.
func (a *Assembler) asciiItem(c *cursor) (int, error) {
	if c.peek() == '<' {
		// Just the one <...> term: "<CR><LF>" is two bytes.
		v, _, err := a.exprValueOf(c, a.pc(), fixAddrB, a.exprAtom)
		if err != nil {
			return 0, err
		}

		if int32(v) < -128 || int32(v) > 0xFF {
			return 0, vmserrors.New(vmserrors.VAX_DATARANGE, "String byte", int32(v))
		}

		if err := a.emitByte(byte(v)); err != nil {
			return 0, err
		}

		return 1, nil
	}

	q := c.next()
	if !isStringDelimiter(q) {
		return 0, vmserrors.New(vmserrors.VAX_BADSTRING, string(q))
	}

	count := 0

	for c.peek() != q {
		if c.atEnd() {
			return 0, vmserrors.New(vmserrors.VAX_NOCLOSE, string(q))
		}

		ch := c.next()

		if err := a.emitByte(ch); err != nil {
			return 0, err
		}
		
		count++
	}

	c.next() // closing delimiter

	return count, nil
}

// pseudoEnd assembles .END [entry-expression]: an optional expression names
// the program's start address (recorded as the __ENTRY symbol, and via
// Entry()), then assembly of the current source stops — matching case 11.
func (a *Assembler) pseudoEnd(c *cursor) error {
	if err := a.closeLocalBlock(); err != nil {
		return err
	}

	a.scopeSymbols()

	c.skipBlanks()

	switch {
	case c.atEnd():

	case a.dialect == DialectMACRO:
		// The transfer address, for the end of module record. Real
		// MACRO's cross reference doesn't list it as a reference
		// (xref.lis's XREF).
		if a.xref != nil {
			a.xref.quiet = true
		}

		x, err := a.exprKnown(c)

		if a.xref != nil {
			a.xref.quiet = false
		}

		if err != nil {
			return err
		}

		addr := x.v

		if !x.known() {
			sect, offset, ok := x.x.simpleRelocatable()
			if !ok {
				return vmserrors.New(vmserrors.VAX_RELEXPR)
			}

			a.entrySect, addr = sect, offset
		}

		a.entrySeen = true
		a.entryAddr = addr

	default:
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
// and/or expressions (printed in decimal, the assembler's radix; the
// reference tool printed them in the console's radix, hexadecimal by
// default), matching console_print() — except output is captured (see Prints())
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

		fmt.Fprintf(&sb, "%d", int32(v))
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

	if err := a.emitWord(uint16(m)); err != nil {
		return err
	}

	return nil
}

// pseudoFloat assembles .F_FLOATING, .D_FLOATING, .G_FLOATING, and
// .H_FLOATING (and the console's .F_FLOAT/.D_FLOAT): a comma-separated
// list of floating constants, each stored in format f, rounded once from
// its decimal value as VAX MACRO rounds it.
func (a *Assembler) pseudoFloat(c *cursor, f vaxfloat.Format) error {
	first := true

	for {
		c.skipBlanks()

		if err := a.listSeparator(c, first); err != nil || c.atEnd() {
			return err
		}

		first = false

		if err := a.output(); err != nil {
			return err
		}

		v, err := a.parseFloat(c)
		if err != nil {
			return err
		}

		if err := a.storeImmediateFloat(f, v); err != nil {
			return err
		}
	}
}

// pseudoPacked assembles .PACKED decimal-string[,symbol]: a decimal
// number of 0 to 31 digits with an optional sign, stored as a packed
// decimal string, two digits a byte with the sign in the last byte's low
// nibble (^XC for plus, the default, ^XD for minus) and, for an even
// number of digits, a zero first nibble. The symbol, if given, is set to
// the number of digits (the sign doesn't count): the length operand the
// decimal string instructions take. From the MACRO manual's .PACKED.
func (a *Assembler) pseudoPacked(c *cursor) error {
	c.skipBlanks()

	neg := false
	if c.peek() == '+' || c.peek() == '-' {
		neg = c.next() == '-'
	}

	start := c.pos
	for isDigit(c.peek()) {
		c.next()
	}

	digits := c.s[start:c.pos]
	if len(digits) == 0 || len(digits) > 31 {
		return vmserrors.New(vmserrors.VAX_BADPACKED)
	}

	a.packedDigits = len(digits)

	sign := byte(0xC)
	if neg {
		sign = 0xD
	}

	nibbles := make([]byte, 0, len(digits)+2)
	if len(digits)%2 == 0 {
		nibbles = append(nibbles, 0)
	}

	for _, ch := range digits {
		nibbles = append(nibbles, byte(ch-'0'))
	}

	nibbles = append(nibbles, sign)

	for i := 0; i < len(nibbles); i += 2 {
		if err := a.emitByte(nibbles[i]<<4 | nibbles[i+1]); err != nil {
			return err
		}
	}

	c.skipBlanks()

	if c.peek() != ',' {
		return nil
	}

	c.next()
	c.skipBlanks()

	name := scanName(c)
	if name == "" {
		return vmserrors.New(vmserrors.VAX_BADPACKED)
	}

	return a.setSymbol(name, uint32(len(digits)), SymNone, false)
}

// pseudoBlock assembles .BLKx [count] (.BLKB, .BLKW, .BLKL, .BLKQ,
// .BLKO, and the address and floating forms .BLKA, .BLKF, .BLKD, .BLKG,
// .BLKH): reserves count*size zero bytes. The reference tool (case 18-21) zero-fills by
// storing a literal zero byte count*size times; this just advances the
// deposit counter without writing anything, since an address this package
// never wrote to already reads back as zero (see image's doc comment) —
// same observable result, without allocating a map entry per byte for
// what can be a very large count.
func (a *Assembler) pseudoBlock(c *cursor, size int) error {
	c.skipBlanks()

	// The count defaults to 1 (the MACRO manual, .BLKx).
	n := uint32(1)

	if !c.atEnd() {
		var err error

		if n, err = a.exprNoForward(c); err != nil {
			return err
		}
	}

	// Storage before any .PSECT goes in . BLANK .; in an absolute psect,
	// .BLKx only moves the location, defining offsets.
	if a.dialect == DialectMACRO {
		a.useBlankPsect()
	}

	// Real MACRO writes even an empty block's CTL_AUGRB, of 0
	// (testdata/mar/dst's DSTSYM, .BLKB 0).
	if n == 0 && a.dialect == DialectMACRO {
		a.logEvent(outEvent{kind: evGap, sect: a.cur, offset: a.cur.loc})
	}

	a.advance(n * uint32(size))

	return nil
}

// pseudoEntry assembles .ENTRY name[,<mask>]: defines name as the current
// address (SYM_ENTRY, must be a fresh definition), starts a new local-
// symbol scope under it, and stores the optional register-set mask (0 if
// omitted) as a 16-bit word — matching case 22.
//
// In the MACRO dialect the name is global, and the mask is any absolute
// expression (usually ^M<...>) that doesn't use R0, R1, AP, or FP.
func (a *Assembler) pseudoEntry(c *cursor) error {
	if err := a.endLocalBlock(); err != nil {
		return err
	}

	a.scopeSymbols()

	c.skipBlanks()
	name := scanName(c)

	flags := SymEntry
	if a.dialect == DialectMACRO {
		flags |= SymGlobal
	}

	if err := a.defineHere(name, flags, true); err != nil {
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
		var err error

		if a.dialect == DialectMACRO {
			mask, err = a.entryMaskMACRO(c)
		} else {
			mask, err = a.maskLiteral(c)
		}

		if err != nil {
			return err
		}
	}

	sym, found := a.symbols.find(name)
	if found {
		sym.mask = uint16(mask)
	}

	if a.dialect != DialectMACRO || !found {
		return a.emitWord(uint16(mask))
	}

	// The object defines the entry point where its mask is stored.
	if err := a.output(); err != nil {
		return err
	}

	if err := a.cur.img.storeWord(a.pc(), uint16(mask)); err != nil {
		return err
	}

	a.logEvent(outEvent{kind: evEntry, sect: a.cur, offset: a.cur.loc, size: 2, value: mask, sym: sym})
	a.listData(a.cur.loc, 2)
	a.move(2)

	return nil
}

// pseudoScope assembles .SCOPE name: like .ENTRY, but only starts a new
// local-symbol scope (SYM_LABEL, must be fresh) — no mask, no code of its
// own. Matches case 41.
func (a *Assembler) pseudoScope(c *cursor) error {
	if err := a.closeLocalBlock(); err != nil {
		return err
	}

	a.scopeSymbols()

	c.skipBlanks()

	name := scanName(c)
	if err := a.setSymbol(name, a.pc(), SymLabel, true); err != nil {
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
		a.caseBase = a.pc()
	}

	first := true

	for {
		c.skipBlanks()

		if err := a.listSeparator(c, first); err != nil || c.atEnd() {
			return err
		}

		first = false

		loc := a.pc()

		v, wasForward, err := a.exprValue(c, loc, fixCaseW)
		if err != nil {
			return err
		}

		// A forward reference's fixup stores its offset later. The
		// reference tool stored a label already defined as its address
		// rather than its offset from the table.
		d := int32(0)
		if !wasForward {
			d = int32(v - a.caseBase)
		}

		if d < math.MinInt16 || d > math.MaxInt16 {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".CASE", d)
		}

		if err := a.emitWord(uint16(int16(d))); err != nil {
			return err
		}
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

	saved := a.pc()
	a.setPC(0x80000000 + a.scbb + code)

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()
	}

	loc := a.pc()

	value, _, err := a.exprValue(c, loc, addrFixup(4))
	if err != nil {
		a.setPC(saved)

		return err
	}

	if value != 0xFFFFFFFF && value&0x3 != 0 {
		a.setPC(saved)

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
			a.setPC(saved)

			return vmserrors.New(vmserrors.VAX_SCBSTACK)
		}
	}

	if err := a.image.storeLongword(a.pc(), value); err != nil {
		a.setPC(saved)

		return err
	}

	a.setPC(saved)

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

	n := (a.pc() / size) * size
	if n < a.pc() {
		a.setPC(n + size)
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

	if err := a.closeLocalBlock(); err != nil {
		return err
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

	if toS0 {
		a.cur = a.s0
	} else {
		a.cur = a.p0
	}

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

	if err := a.closeLocalBlock(); err != nil {
		return err
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
		if err := a.setSymbol(name, a.pc(), SymEntry, true); err != nil {
			return err
		}

		shimAddr = a.pc()

		if err := a.emitWord(0); err != nil { // empty entry mask word
			return err
		}

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
	if err := a.emitBytes(0xD0, 0x8F); err != nil { // MOVL I^#
		return err
	}

	if err := a.emitLongword(code); err != nil {
		return err
	}

	return a.emitBytes(0x50, 0xFC, 0x7D, 0x04) // R0 ; XFC #XFC$SHIM ; RET
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
		a.advance(n)

		return nil
	}

	for i := uint32(0); i < n; i++ {
		if err := a.emitByte(fill); err != nil {
			return err
		}
	}

	return nil
}

// pseudoJcc assembles .JEQL/.JEQLU/.JNEQ/.JNEQU dest: a long-range
// conditional jump, synthesized as the inverse short branch around an
// absolute JMP — matching cases 38-39. invByte is the inverted branch
// opcode (0x12 BNEQ for .JEQL, 0x13 BEQL for .JNEQ).
func (a *Assembler) pseudoJcc(c *cursor, invByte byte) error {
	if err := a.emitBytes(
		invByte, // BNEQ/BEQL
		0x06,    // branch-around displacement (past the 4-byte JMP operand)
		0x17,    // JMP
		0x9F,    // @# addressing mode
	); err != nil {
		return err
	}

	v, _, err := a.exprValue(c, a.pc(), fixAddrL)
	if err != nil {
		return err
	}

	return a.emitLongword(v)
}

// pseudoConsole assembles .CONSOLE <command>: the reference tool dispatches
// the rest of the line to the interactive console's own DCL-style command
// dispatcher (Phase 08's internal/console), which this package can't call
// into without an inverted, assembler-depends-on-console package
// dependency, so the command is accepted as a no-op, like a console
// command whose only observable effect would be on a live interactive
// session this batch assembler doesn't have. In the reference tool,
// ".CONSOLE SET RADIX" also changed the assembler's own radix, which the
// console shared; here the assembler's radix is always MACRO-32's decimal
// (see Assembler.radix).
func (a *Assembler) pseudoConsole(c *cursor) error {
	c.pos = len(c.s)

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

// pseudoP1Vector assembles .P1VECTOR, matching asm_pseudo.c's case 40
// (`return p1_init();`) — a direct call into p1_vector.c's own p1_init(),
// not deferred to anything downstream of assembly. For every entry in
// internal/vmsdef's fixed VMS P1VectorTable it defines the SYS$xxx symbol
// (permanent, SymEntry for an ordinary CALL target or SymLabel for the one
// JMP-reached entry, SYS$SRCHANDLER — see vmsdef.P1VectorEntry.Jmp) and deposits
// a CALLS-compatible trampoline at its address: a 2-byte zero procedure-
// entry mask (skipped for the JMP entry, which has no CALL frame to build),
// then "XFC #XFC$P1VECTOR" (0xFC 0x7A), then RET (0x04) — byte-for-byte
// what p1_init() itself writes via store_memory. Also defines
// EXE$P1_VECTOR_BASE/END from the real min/max addresses seen, which
// internal/console/asm.go's depositAsmImage uses to know which range of
// the assembled image to copy into live memory (this pseudo-op only
// touches the assembler's own sparse image buffer, like every other
// pseudo-op in this file — nothing here talks to live VM memory directly,
// unlike the reference tool's own store_memory).
//
// p1_init()'s own declare_services() call (wiring each service's native
// function pointer) has no Go counterpart to invoke here: this port's
// internal/rtl already registers every implemented SYS$ handler statically
// at package init (ServiceTable.Register), entirely independent of
// assembly. Its final setpte_multiple(...PROT=PTE$K_UR) call is similarly
// not replicated — this port has no PTE-protection pseudo-op or enforcement
// this fine-grained (see this file's own scope-cut list), and every
// existing caller through this trampoline already works without it (kernel
// mode's own access already covers it; no fixture here runs the calling
// program in user mode against page-protection checks that would need it).
// Requires .MICROKERNEL, matching .SCB/.SHIM/.REGION.
func (a *Assembler) pseudoP1Vector(c *cursor) error {
	_ = c

	if !a.microkernel {
		return vmserrors.New(vmserrors.VAX_NEEDMICRO, ".P1VECTOR")
	}

	a.scopeSymbols()

	minValue := uint32(0x7FFFFFFF)
	maxValue := uint32(0)

	for _, e := range vmsdef.P1VectorTable {
		flags := SymEntry
		if e.Jmp {
			flags = SymLabel
		}

		// unique=false: matching set_symbol_direct's own call in p1_init()
		// (asm_symbols.c's set_symbol only rejects a redefinition when the
		// caller has separately set ASM_UNIQUE ahead of the call — p1_init()
		// never does), not .SHIM's own uniqueness-checked pseudoShim call
		// this was originally modeled on. Running .P1VECTOR twice (e.g. a
		// live console session that already booted kernel.asm's own
		// ".p1vector" ASMing a second file — such as this project's own
		// testdata/asm/rms_roundtrip.asm fixture, whose own ".p1vector"
		// line exists only because its tests run outside full console
		// boot/VMINIT and so never get kernel.asm's copy for free) just
		// redefines every symbol to the same value again rather than
		// failing with a duplicate-symbol error.
		if err := a.setSymbol(e.Name, e.Addr, flags|SymPermanent, false); err != nil {
			return err
		}

		if e.Addr > maxValue {
			maxValue = e.Addr
		}

		if e.Addr < minValue {
			minValue = e.Addr
		}

		// The reference tool's p1_init() wrote a trampoline for every
		// entry, so SYS$GL_ASTRET/SYS$GL_COMMON's overwrote SYS$CLRAST_2's
		// RET and SYS$GL_COMMON's RET was in turn overwritten by
		// SYS$SRCHANDLER's XFC. See vmsdef.P1VectorEntry.DataCell.
		if e.DataCell() {
			continue
		}

		addr := e.Addr

		if !e.Jmp {
			if err := a.image.storeWord(addr, 0); err != nil { // empty entry mask word
				return err
			}
		} else {
			addr -= 2
		}

		if err := a.image.storeByte(addr+2, 0xFC); err != nil { // XFC opcode
			return err
		}

		if err := a.image.storeByte(addr+3, 0x7A); err != nil { // XFC$P1VECTOR selector
			return err
		}

		if err := a.image.storeByte(addr+4, 0x04); err != nil { // RET
			return err
		}
	}

	if err := a.setSymbol("EXE$P1_VECTOR_BASE", minValue, SymNone, false); err != nil {
		return err
	}

	a.p1VectorBase, a.p1VectorEnd, a.p1VectorSet = minValue, maxValue+5, true

	return a.setSymbol("EXE$P1_VECTOR_END", maxValue, SymNone, false)
}

// pseudoRMSDEF assembles .RMSDEF — this port's combined equivalent of real
// MACRO-32's $FABDEF/$RABDEF/$RMSDEF library macros (docs/PHASE-24.md):
// defines every real FAB$/RAB$/RMS$ symbol as a permanent assembler
// symbol — each field's offset symbol (e.g. "FAB$B_FAC"), every bit
// number, bitmask flag, and named code value, and every RMS$_
// completion-status code (internal/vmsdef.Symbols' FAB$, RAB$, and RMS$
// names; the offsets agree with FABFields/RABFields, TestFields_matchSymbols)
// — so a program can address a FAB/RAB field the real-MACRO-32 way
// (<label>+FAB$L_STS) and use symbolic names (FAB$C_SEQ, RMS$_NORMAL, ...)
// anywhere an expression is expected, including as a .FAB/.RAB keyword's
// own value.
//
// Deliberately not gated on .MICROKERNEL: unlike .P1VECTOR/.SHIM/.SCB/
// .REGION, .RMSDEF deposits no bytes into the image at all — purely
// symbol-table definition, exactly like a real program's own $FABDEF
// .INCLUDE, which any ordinary user-mode assembly can use regardless of
// microkernel context.
//
// unique=false on every setSymbol call, from the start (not a fix bolted
// on after the fact the way .P1VECTOR's was — see docs/PHASE-11.md's own
// progress log on that bug): running .RMSDEF twice in the same session
// (e.g. a persistent console session's own kernel.asm boot having already
// run it, then a second file's own ".RMSDEF" line) must not fail with a
// duplicate-symbol error, matching real $FABDEF/$RABDEF/$RMSDEF's own
// tolerance for being .INCLUDEd more than once.
func (a *Assembler) pseudoRMSDEF(c *cursor) error {
	_ = c

	a.scopeSymbols()

	for _, name := range vmsdef.SymbolNames("FAB$", "RAB$", "RMS$") {
		if err := a.setSymbol(name, vmsdef.Symbols[name], SymPermanent, false); err != nil {
			return err
		}
	}

	return nil
}

// pseudoFAB assembles .FAB [KEYWORD=value[, KEYWORD=value]...] — this
// port's equivalent of real MACRO-32's $FAB macro (docs/PHASE-24.md):
// builds an 80-byte FAB instance at the current deposit location. See
// buildControlBlock's own doc comment for the shared implementation.
func (a *Assembler) pseudoFAB(c *cursor) error {
	return a.buildControlBlock(c, ".FAB", vmsdef.FABFields, 80, "FAB$C_BID", "FAB$K_BLN")
}

// pseudoRAB assembles .RAB [KEYWORD=value[, KEYWORD=value]...] — this
// port's equivalent of real MACRO-32's $RAB macro: builds a 68-byte RAB
// instance. See buildControlBlock's own doc comment.
func (a *Assembler) pseudoRAB(c *cursor) error {
	return a.buildControlBlock(c, ".RAB", vmsdef.RABFields, 68, "RAB$C_BID", "RAB$K_BLN")
}

// buildControlBlock is .FAB/.RAB's shared implementation. Matches real
// $FAB/$RAB's own expansion: writes the block's BID/BLN identification
// bytes unconditionally first (from vmsdef.Symbols directly, not through
// the assembler's own symbol table — so .FAB/.RAB need no preceding
// .RMSDEF to produce a correctly self-identifying block; see .RMSDEF's own
// doc comment on why *it* only matters once a value is written
// symbolically), then parses zero or more comma-separated "KEYWORD=value"
// (or "KEYWORD:value" or "KEYWORD,value" — matching .SET's own
// scanSetName/separator convention) parameters against fields, placing
// each value at that field's own real offset with the width (byte/word/
// longword) its own Size gives — a value expression may itself reference
// a symbolic constant (e.g. "ORG=FAB$C_SEQ"), including one .RMSDEF
// defined, or a forward-referenced label (e.g. "FNA=fspec" for a fspec
// defined later in the file, matching .LONG's own forward-reference
// support via the same exprValue/addrFixup machinery).
//
// Every field not named in the parameter list is left at zero, matching
// .BLKB's own "never written reads back as zero" convention (internal/asm/
// image.go) — no explicit zero-fill pass is needed. A keyword naming a
// field whose own Size isn't 1, 2, or 4 (RAB$W_RFA is the one such case;
// see vmsdef.Field.Size's own doc comment) fails through storeScaled's
// existing VAX_BADSCALE error rather than a bespoke one — that field isn't
// independently settable via a single keyword value in real $RAB either.
func (a *Assembler) buildControlBlock(c *cursor, name string, fields []vmsdef.Field, blockSize uint32, bidConst, blnConst string) error {
	a.scopeSymbols()

	base := a.pc()

	bid, ok := lookupField(fields, "BID")
	if !ok {
		panic(name + ": no BID field in its own field table")
	}

	bln, ok := lookupField(fields, "BLN")
	if !ok {
		panic(name + ": no BLN field in its own field table")
	}

	if err := a.storeScaled(base+bid.Offset, vmsdef.Symbols[bidConst], int(bid.Size)); err != nil {
		return err
	}

	if err := a.storeScaled(base+bln.Offset, vmsdef.Symbols[blnConst], int(bln.Size)); err != nil {
		return err
	}

	c.skipBlanks()

	for !c.atEnd() {
		if c.peek() == ',' {
			c.next()
			c.skipBlanks()
		}

		if c.atEnd() {
			break
		}

		keyword := scanSetName(c)

		f, ok := lookupField(fields, keyword)
		if !ok {
			return vmserrors.New(vmserrors.VAX_UNDEFSYM, keyword)
		}

		c.skipBlanks()

		if c.peek() == '=' || c.peek() == ':' || c.peek() == ',' {
			c.next()
		}

		loc := base + f.Offset

		v, _, err := a.exprValue(c, loc, addrFixup(int(f.Size)))
		if err != nil {
			return err
		}

		if err := a.storeScaled(loc, v, int(f.Size)); err != nil {
			return err
		}

		c.skipBlanks()
	}

	a.setPC(base + blockSize)

	return nil
}

// lookupField finds keyword in fields (vmsdef.FABFields/RABFields), the
// shared helper buildControlBlock's own keyword parsing uses.
func lookupField(fields []vmsdef.Field, keyword string) (vmsdef.Field, bool) {
	for _, f := range fields {
		if f.Keyword == keyword {
			return f, true
		}
	}

	return vmsdef.Field{}, false
}
