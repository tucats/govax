package asm

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// enableFlags is a set of the assembler functions .ENABLE and .DISABLE
// control (the MACRO manual, Table 6-3).
type enableFlags uint16

const (
	// enableAbsolute assembles relative operands as absolute ones.
	enableAbsolute enableFlags = 1 << iota
	// enableDebug puts local symbols in the object for the debugger.
	enableDebug
	// enableGlobal makes every undefined symbol external (on by default).
	enableGlobal
	// enableSuppression leaves unreferenced symbols out of the listing.
	enableSuppression
	// enableTraceback puts traceback records in the object (on by
	// default).
	enableTraceback
	// enableTruncation truncates F_floating constants instead of
	// rounding them.
	enableTruncation
	// enableVector accepts vector instructions.
	enableVector
	// enableLocalBlock isn't a lasting setting: .ENABLE LOCAL_BLOCK
	// starts a local label block that labels and .PSECTs don't end.
	enableLocalBlock
)

// enableArgs maps each .ENABLE/.DISABLE argument, in its long and short
// forms, to its function.
var enableArgs = map[string]enableFlags{
	"ABSOLUTE": enableAbsolute, "AMA": enableAbsolute,
	"DEBUG": enableDebug, "DBG": enableDebug,
	"GLOBAL": enableGlobal, "GBL": enableGlobal,
	"LOCAL_BLOCK": enableLocalBlock, "LSB": enableLocalBlock,
	"SUPPRESSION": enableSuppression, "SUP": enableSuppression,
	"TRACEBACK": enableTraceback, "TBK": enableTraceback,
	"TRUNCATION": enableTruncation, "FPT": enableTruncation,
	"VECTOR": enableVector,
}

// unsupportedEnables are the functions govax doesn't implement. Enabling
// one is a warning, and assembly goes on without it.
var unsupportedEnables = map[enableFlags]string{
	enableTruncation: "TRUNCATION",
	enableVector:     "VECTOR",
}

// pseudoEnable assembles .ENABLE (on true) or .DISABLE (on false) and a
// list of arguments, separated by commas or blanks. ABSOLUTE, DEBUG,
// GLOBAL, SUPPRESSION, and TRACEBACK are recorded for the parts of
// assembly that use them. LOCAL_BLOCK ends the current local label block;
// enabled, it also starts one that only another .ENABLE LOCAL_BLOCK, or a
// label or .PSECT after .DISABLE LOCAL_BLOCK, ends (the manual, §3.4).
func (a *Assembler) pseudoEnable(c *cursor, on bool) error {
	directive := ".ENABLE"
	if !on {
		directive = ".DISABLE"
	}

	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.atEnd() {
			return nil
		}

		word := scanName(c)

		f, ok := enableArgs[word]
		if !ok {
			if word == "" {
				word = c.rest()
			}

			return vmserrors.New(vmserrors.VAX_BADKEYWORD, directive, word)
		}

		switch {
		case f == enableLocalBlock && on:
			if err := a.closeLocalBlock(); err != nil {
				return err
			}

			a.localBlockHeld = true

		case f == enableLocalBlock:
			a.localBlockHeld = false

		case on:
			if name, unsupported := unsupportedEnables[f]; unsupported {
				a.warn(vmserrors.New(vmserrors.VAX_IGNORED, ".ENABLE "+name))
			}

			a.enabled |= f

		default:
			a.enabled &^= f
		}
	}
}

// SetFunctions turns the .ENABLE functions enable names on, and those
// disable names off, for the start of the assembly: MACRO's /ENABLE= and
// /DISABLE=, and /DEBUG= mapped onto DEBUG and TRACEBACK (see the console's
// MACRO command). The source's .ENABLE and .DISABLE can still change
// them. Names are .ENABLE's arguments, long or short (TRACEBACK or TBK);
// LOCAL_BLOCK, which starts a block rather than setting a function, isn't
// one. Call it after SetDialect, which sets the defaults.
func (a *Assembler) SetFunctions(enable, disable []string) error {
	for _, list := range []struct {
		names []string
		on    bool
	}{{enable, true}, {disable, false}} {
		for _, name := range list.names {
			name = strings.ToUpper(strings.TrimSpace(name))
			if name == "" {
				continue
			}

			f, ok := enableArgs[name]
			if !ok || f == enableLocalBlock {
				qualifier := "/ENABLE"
				if !list.on {
					qualifier = "/DISABLE"
				}

				return vmserrors.New(vmserrors.VAX_BADKEYWORD, qualifier, name)
			}

			if list.on {
				a.enabled |= f
			} else {
				a.enabled &^= f
			}
		}
	}

	return nil
}

// pseudoDefault assembles .DEFAULT DISPLACEMENT,BYTE|WORD|LONG: the
// displacement size of a relative operand whose target isn't already
// defined in the same psect.
func (a *Assembler) pseudoDefault(c *cursor) error {
	c.skipBlanks()

	if word := scanName(c); word != "DISPLACEMENT" {
		return vmserrors.New(vmserrors.VAX_BADKEYWORD, ".DEFAULT", word)
	}

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()
		c.skipBlanks()
	}

	word := scanName(c)

	switch word {
	case "BYTE":
		a.defaultDisp = 1
	case "WORD":
		a.defaultDisp = 2
	case "LONG":
		a.defaultDisp = 4
	default:
		return vmserrors.New(vmserrors.VAX_BADKEYWORD, ".DEFAULT DISPLACEMENT", word)
	}

	return nil
}

// Limits on module identification (the MACRO manual, .TITLE and .IDENT):
// the module name and ident are at most 31 characters, and the title
// comment is cut to 40.
const (
	maxModuleName = 31
	maxTitleText  = 40
)

// defaultModuleName names a module with no .TITLE.
const defaultModuleName = ".MAIN."

// pseudoTitle assembles .TITLE module-name comment: the module name is
// the first 31 or fewer nonblank characters (the MACRO manual, .TITLE),
// and the rest, as written (see preprocessLine), is the title comment,
// cut to 40 characters. The last .TITLE wins.
func (a *Assembler) pseudoTitle(c *cursor) error {
	name := readToken(c)
	if name == "" {
		return vmserrors.New(vmserrors.VAX_BADKEYWORD, ".TITLE", "")
	}

	if len(name) > maxModuleName {
		name = name[:maxModuleName]
	}

	c.skipBlanks()

	text := c.rest()
	if len(text) > maxTitleText {
		text = text[:maxTitleText]
	}

	c.pos = len(c.s)
	a.title, a.titleText = name, text

	return nil
}

// pseudoIdent assembles .IDENT /string/: the module's version, a
// delimited string of 1 to 31 characters, kept as written. The last
// .IDENT wins.
func (a *Assembler) pseudoIdent(c *cursor) error {
	ident, err := readDelimited(c)
	if err != nil {
		return err
	}

	if len(ident) == 0 || len(ident) > maxModuleName {
		return vmserrors.New(vmserrors.VAX_DATARANGE, ".IDENT length", len(ident))
	}

	a.ident = ident

	return nil
}

// readDelimited reads a delimited string (/text/, "text", ...), the
// operand of .IDENT and .LIBRARY, and returns the text between the
// delimiters.
func readDelimited(c *cursor) (string, error) {
	c.skipBlanks()

	q := c.next()
	if !isStringDelimiter(q) {
		return "", vmserrors.New(vmserrors.VAX_BADSTRING, string(q))
	}

	start := c.pos

	for c.peek() != q {
		if c.atEnd() {
			return "", vmserrors.New(vmserrors.VAX_NOCLOSE, string(q))
		}

		c.next()
	}

	text := c.s[start:c.pos]
	c.next()

	return text, nil
}

// Title returns the module's name (.TITLE's, or .MAIN. without one) and
// its title comment.
func (a *Assembler) Title() (name, text string) {
	if a.title == "" {
		return defaultModuleName, a.titleText
	}

	return a.title, a.titleText
}

// Ident returns the module's version, from .IDENT.
func (a *Assembler) Ident() string { return a.ident }

// declareSymbols assembles .GLOBAL, .EXTERNAL, or .WEAK and a
// comma-separated list of symbols, giving each flags. A symbol named
// before it's defined, or never defined, is external if the module
// doesn't define it, whatever .ENABLE GLOBAL says (see finish). The
// manual gives .GLOBAL and .EXTERNAL the same meaning for a symbol the
// module doesn't define, and .GLOBAL's is the only meaning left for one
// it does.
func (a *Assembler) declareSymbols(c *cursor, flags SymFlag) error {
	first := true

	for {
		c.skipBlanks()

		if err := a.listSeparator(c, first); err != nil || c.atEnd() {
			return err
		}

		first = false

		name := scanName(c)

		switch {
		case name == "":
			return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
		case isLocalLabel(name):
			return vmserrors.New(vmserrors.VAX_NOTGLOBAL, name)
		}

		a.declareSymbol(name, flags)
	}
}

// declareSymbol gives the symbol name flags, creating it, with no value
// yet, if the module hasn't defined it.
func (a *Assembler) declareSymbol(name string, flags SymFlag) *symbol {
	// .GLOBAL, .EXTERNAL, and .WEAK refer to the symbols they name
	// (xref.lis's LIB$PUT_OUTPUT, symxref.lis's WEAKDEF).
	a.xrefSymbol(name)

	sym, found := a.symbols.find(name)
	if !found {
		sym = a.symbols.create(name)
		sym.firstSect = a.cur
		flags |= SymUndefined
	} else if !sym.defined() {
		flags |= SymUndefined
	}

	sym.flags |= flags

	return sym
}

// pseudoAddress assembles .ADDRESS: a comma-separated list of addresses,
// each stored as a longword. In an object module each is position
// independent (the object language's STO_PIDR).
func (a *Assembler) pseudoAddress(c *cursor) error {
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

		v, deferred, err := a.exprValue(c, a.pc(), fixAddress)
		if err != nil {
			return err
		}

		// Real MACRO stores even an absolute address through the
		// linker's stack (STA_UB 0, STO_PIDR for .ADDRESS 0, as in every
		// $FAB), so the field holds zero until the linker stores it.
		if !deferred {
			a.relocs = append(a.relocs, relocation{sect: a.cur, offset: a.cur.loc, kind: fixAddress, expr: constNode(v), stmt: a.stmt})
		}

		if err := a.emitLongword(0); err != nil {
			return err
		}
	}
}

// pseudoMaskMACRO assembles MACRO-32's .MASK symbol[,expression]: it
// reserves a word for a transfer vector's register save mask, which the
// linker fills with the entry point symbol's mask, ORed with expression
// if there is one (the MACRO manual, .MASK). symbol must be an .ENTRY
// point, here or in another module.
func (a *Assembler) pseudoMaskMACRO(c *cursor) error {
	c.skipBlanks()

	name := scanName(c)
	if name == "" {
		return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
	}

	if sym, found := a.symbols.find(name); found && sym.defined() && sym.flags&SymEntry == 0 {
		return vmserrors.New(vmserrors.VAX_NOTENTRY, name)
	}

	t := &rexpr{op: rMask, key: name}

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()

		v, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		t = &rexpr{op: rBinary, bin: '!', l: t, r: constNode(v)}
	}

	// The mask is the entry point's, wherever it's defined, so the
	// linker always supplies it: an undefined symbol is external.
	a.declareSymbol(name, SymNone).referenced = true

	if err := a.output(); err != nil {
		return err
	}

	a.relocs = append(a.relocs, relocation{sect: a.cur, offset: a.cur.loc, kind: fixAddrW, expr: t, stmt: a.stmt})

	return a.emitWord(0)
}

// warn records a warning for the statement being assembled.
func (a *Assembler) warn(err error) {
	a.listWarning(err)
	a.warnings = append(a.warnings, a.located(err))
}

// recoverable reports err, an error real MACRO reports and then goes on
// from, assembling the rest of the statement as best it can (a value too
// big for its field is stored truncated, for one). In the MACRO dialect
// it records err for the statement being assembled and returns nil, so
// the caller goes on as MACRO does; the assembly still fails. In the
// console dialect it returns err, which ends the statement, as eVAX did.
// A nil err is returned as it is.
func (a *Assembler) recoverable(err error) error {
	if err == nil || a.dialect != DialectMACRO {
		return err
	}

	a.listError(err)
	a.errs = append(a.errs, a.located(err))

	return nil
}

// recoverableAt is recoverable for an error the listing shows before
// the statement's bytes from loc on, in the current section, rather than
// before those stored from the location counter on.
func (a *Assembler) recoverableAt(err error, loc uint32) error {
	if err == nil || a.dialect != DialectMACRO {
		return err
	}

	n := len(a.listCur.notesOrNil())

	if err := a.recoverable(err); err != nil {
		return err
	}

	if l := a.listCur; l != nil && len(l.notes) > n {
		l.notes[n].loc = loc
	}

	return nil
}

// Warnings returns the warnings assembly produced, each an *Error naming
// its line.
func (a *Assembler) Warnings() []error { return a.warnings }

// entryMaskReserved are the register save mask bits .ENTRY can't set:
// R0, R1, AP, and FP (the MACRO manual, .ENTRY).
const entryMaskReserved = 1<<0 | 1<<1 | 1<<12 | 1<<13

// entryMaskMACRO reads MACRO-32's .ENTRY mask, an absolute expression
// (usually ^M<...>) using no reserved register.
func (a *Assembler) entryMaskMACRO(c *cursor) (uint32, error) {
	m, err := a.exprNoForward(c)
	if err != nil {
		return 0, err
	}

	if m&entryMaskReserved != 0 || m > 0xFFFF {
		return 0, vmserrors.New(vmserrors.VAX_ENTRYMASK, m)
	}

	return m, nil
}

// titleDirectiveAt returns the length of ".TITLE", ".SUBTITLE", or
// ".SBTTL" (in any case, with its "."), as a whole word starting at b[i],
// or 0. preprocessLine keeps their text as written.
func titleDirectiveAt(b []byte, i int) int {
	if b[i] != '.' || (i > 0 && !isBlank(b[i-1]) && b[i-1] != ':') {
		return 0
	}

	for _, name := range []string{"TITLE", "SUBTITLE", "SBTTL"} {
		n := 1 + len(name)
		if i+n <= len(b) && strings.EqualFold(string(b[i+1:i+n]), name) && (i+n == len(b) || isBlank(b[i+n])) {
			return n
		}
	}

	return 0
}
