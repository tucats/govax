package asm

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements MACRO-32 macros (docs/PHASE-28.md): defining them
// with .MACRO/.ENDM, calling them, and the directives that end or delete
// them (.MEXIT, .MDELETE). macroargs.go parses a call's arguments and
// substitutes them into the macro's lines.
//
// # What a macro is, for a reader new to VAX MACRO
//
// A macro is a named piece of source text with parameters. Defining one:
//
//	        .MACRO  STORE   VALUE, WHERE=R0     ; name, then formal arguments
//	        MOVL    VALUE, WHERE                ; the body
//	        .ENDM   STORE
//
// only records the text; nothing is assembled. Using the name where an
// instruction would go (a "call"):
//
//	        STORE   #5, R3
//
// replaces that line with the body, each formal argument's name replaced
// by the text given for it (the "actual" argument):
//
//	        MOVL    #5, R3
//
// and that text (the "expansion") is then assembled as if it had been
// written there. Everything is text substitution: the assembler doesn't
// know or care what VALUE means until the expanded line is assembled.
// VMS's system macros ($EXIT_S, $FAB, $QIOW_S, ...) are nothing more than
// macros like this, kept in the macro library SYS$LIBRARY:STARLET.MLB.
//
// # How govax does it
//
//   - pseudoMacro (the .MACRO directive) reads the macro's name and
//     formal arguments and starts a definition. From then on, the source
//     loop (runSource) hands each raw source line to collectDefinition
//     instead of assembling it, until the matching .ENDM. The raw line is
//     kept, comment and case and all, since arguments are substituted
//     everywhere, even inside strings and comments.
//   - assembleMacroCall recognizes a call: a statement whose first word
//     names a macro. It parses the actual arguments (parseActuals), binds
//     them to the formal ones (bind), substitutes them into each line
//     (substitute), and assembles the result as a new source on the
//     source stack (runSource with a sourceMacro frame).
//
// Because an expansion is assembled like any other source, a macro's body
// can do anything source can: call other macros (or itself), use
// conditional assembly, and even define macros of its own, as some of
// VMS's system macros do.

// macroDef is one macro definition.
type macroDef struct {
	// name is the macro's name, in uppercase: macro names, like symbols,
	// don't depend on case.
	name string
	// formals are the formal arguments, in the order the .MACRO
	// directive lists them.
	formals []formal
	// body is the macro's lines as written in the source, from the line
	// after .MACRO to the line before .ENDM.
	body []string
	// end is the text an expansion's listing shows for the .ENDM (or a
	// repeat block's .ENDR) that ended the definition: the line with the
	// directive taken out, which leaves its leading blanks (see
	// endLineText). hasEnd is false when the directive had a label: the
	// label is the body's last line, and is listed as the expansion's
	// last line.
	end    string
	hasEnd bool
}

// endLineText returns the text a listing shows for raw, the .ENDM or
// .ENDR line that ended a definition with no label: its leading blanks.
func endLineText(raw string) string {
	return raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
}

// formal is one formal argument.
type formal struct {
	// name is the argument's name, in uppercase.
	name string
	// def is the default value, used when a call gives no value; "" if
	// the definition gives none.
	def string
	// created says the argument was written "?NAME": when a call leaves
	// it blank, the assembler makes up a local label for it
	// (docs/PHASE-28.md, subtask 2).
	created bool
}

// formalIndex returns the index of the formal argument named name, or -1.
func (m *macroDef) formalIndex(name string) int {
	for i, f := range m.formals {
		if strings.EqualFold(f.name, name) {
			return i
		}
	}

	return -1
}

// definition is a macro definition, or a repeat block, whose lines are
// being collected.
type definition struct {
	// def is the macro, or for a repeat block, the block's range and its
	// formal argument (see repeat.go).
	def *macroDef
	// repeat is the repeat block, or nil for a macro definition.
	repeat *repeatBlock
	// depth counts the blocks inside the definition not yet ended: a
	// macro's body can define a macro, or hold a repeat block, and the
	// inner block's end mustn't end the definition.
	depth int
}

// expansion is what a macro call's expansion knows about the call, for
// the directives that ask about it (.NARG, subtask 2).
type expansion struct {
	def *macroDef
	// positional is how many positional arguments the call gave,
	// counting null ones (.NARG's value).
	positional int
}

// defineMacro adds m to the macro table, replacing any macro of the same
// name: redefining a macro is allowed, and is how a macro can change what
// its own later calls do.
func (a *Assembler) defineMacro(m *macroDef) {
	if a.macros == nil {
		a.macros = map[string]*macroDef{}
	}

	a.macros[m.name] = m

	if a.xref != nil {
		if e := a.xrefEntryFor(xrefMacros, m.name); e != nil {
			e.size = macroSize(m.body)
		}
	}
}

// pseudoMacro assembles .MACRO name [formal-argument-list], starting a
// definition: the lines that follow are the macro's body, up to the
// matching .ENDM (see collectDefinition). The name may be followed by a
// comma ("$SSDEF,$GBL"), as the formal arguments may be separated by
// commas or blanks.
func (a *Assembler) pseudoMacro(c *cursor) error {
	c.skipBlanks()

	name := scanName(c)
	if name == "" {
		return vmserrors.New(vmserrors.VAX_MACRONAME, c.rest())
	}

	formals, err := parseFormals(c)
	if err != nil {
		return err
	}

	a.defining = &definition{def: &macroDef{name: name, formals: formals}}
	a.xrefDefine(xrefMacros, name)

	return nil
}

// Kinds of line that matter while collecting a block's lines (see
// blockDirective).
const (
	blockNone  = iota
	blockStart // .MACRO, or a repeat block's .IRP, .IRPC, .REPEAT, .REPT
	blockEnd   // .ENDM or .ENDR
)

// blockStarts and blockEnds name the directives that start and end a
// block of lines: a macro definition or a repeat block. Either ending
// directive ends either kind of block (MACRO-11 used .ENDM for both, and
// VMS's STARLET.MLB still ends some .IRP blocks with .ENDM), except that
// only .ENDM ends a macro definition itself.
var (
	blockStarts = map[string]bool{"MACRO": true, "IRP": true, "IRPC": true, "REPEAT": true, "REPT": true}
	blockEnds   = map[string]bool{"ENDM": true, "ENDR": true}
)

// blockDirective reports whether raw (an unassembled source line) starts
// or ends a block, and returns the directive's name (without its "."),
// any label before it, and a cursor after the name. It's how a block's
// end is found without assembling anything.
func (a *Assembler) blockDirective(raw string) (kind int, name string, labelText string, rest *cursor) {
	c := newCursor(preprocessLine(raw))
	c.skipBlanks()

	// A label: a name and one or two colons before the first blank.
	if i := strings.IndexByte(c.rest(), ':'); i > 0 && !strings.ContainsAny(c.rest()[:i], " \t") {
		labelText = c.rest()[:i+1]
		c.skip(i + 1)

		if c.peek() == ':' {
			labelText += ":"
			
			c.next()
		}

		c.skipBlanks()
	}

	dotted := c.peek() == '.'
	if dotted {
		c.next()
	}

	// As for every directive, the "." is required in the MACRO dialect
	// and optional in the console's (see assemblePseudo).
	if !dotted && a.dialect == DialectMACRO {
		return blockNone, "", labelText, c
	}

	name = scanName(c)

	switch {
	case blockStarts[name]:
		return blockStart, name, labelText, c
	case blockEnds[name]:
		return blockEnd, name, labelText, c
	}

	return blockNone, name, labelText, c
}

// collectDefinition takes one raw source line while a macro definition or
// a repeat block is being collected: it's added to the macro's body (or
// the block's range), unless it's the .ENDM (or .ENDR) that ends it. A
// .MACRO inside the body starts a nested definition, which is collected as
// part of the body (it's defined only when the outer macro is called),
// and its .ENDM is part of the body too; likewise a repeat block and its
// .ENDR.
//
// A repeat block ends with .ENDR, or with .ENDM as some STARLET.MLB
// macros' blocks do, and is then assembled (assembleRepeat).
func (a *Assembler) collectDefinition(raw string) error {
	d := a.defining

	kind, word, label, c := a.blockDirective(raw)

	switch {
	case kind == blockStart:
		d.depth++

	case kind == blockEnd && d.depth > 0:
		d.depth--

	case kind == blockEnd && d.repeat != nil:
		a.xrefRefer(xrefDirectives, "."+word, "")

		// The end of a repeat block. A label on the .ENDR line is
		// part of the range, as .ENDM's is part of a macro's body.
		if label != "" {
			d.def.body = append(d.def.body, label)
		} else {
			d.def.end, d.def.hasEnd = endLineText(raw), true
		}

		a.defining = nil

		return a.assembleRepeat(d)

	case word == "ENDM":
		a.xrefRefer(xrefDirectives, ".ENDM", "")

		// The end of the definition. A label on the .ENDM line is
		// part of the body: the manual's POSITIVE macro ends with
		// "L1: .ENDM", so that L1 labels the line after the
		// expansion.
		if label != "" {
			d.def.body = append(d.def.body, label)
		} else {
			d.def.end, d.def.hasEnd = endLineText(raw), true
		}

		a.defining = nil
		a.defineMacro(d.def)

		// .ENDM may name the macro it ends, and then must name it
		// correctly.
		c.skipBlanks()

		if name := scanName(c); name != "" && name != d.def.name {
			return vmserrors.New(vmserrors.VAX_ENDMNAME, name, d.def.name)
		}

		return nil
	}

	d.def.body = append(d.def.body, raw)

	return nil
}

// pseudoEndm assembles an .ENDM that no .MACRO started (collectDefinition
// takes every .ENDM that ends a definition).
func (a *Assembler) pseudoEndm(*cursor) error {
	return vmserrors.New(vmserrors.VAX_NOTINDEF, ".ENDM")
}

// pseudoMdelete assembles .MDELETE macro-name-list: the named macros are
// deleted. Deleting a macro that isn't defined does nothing. A listing
// shows how many macros were deleted, as a direct assignment shows its
// value.
func (a *Assembler) pseudoMdelete(c *cursor) error {
	deleted := uint32(0)

	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.atEnd() {
			a.listValue(nil, deleted)

			return nil
		}

		name := scanName(c)
		if name == "" {
			return vmserrors.New(vmserrors.VAX_MACRONAME, c.rest())
		}

		if _, ok := a.macros[name]; ok {
			delete(a.macros, name)

			deleted++
		}
	}
}

// pseudoMexit assembles .MEXIT: the rest of the innermost macro expansion
// is skipped, as if its .ENDM had been reached. When the innermost
// expansion is a repeat block's, the rest of the repetition and the
// repetitions still to come are skipped (the manual, .MEXIT's notes 1
// and 2). (runSource closes the conditional blocks the expansion opened.)
func (a *Assembler) pseudoMexit(*cursor) error {
	f := a.innermost(sourceMacro, sourceRepeat)
	if f == nil {
		return vmserrors.New(vmserrors.VAX_NOTINMACRO, ".MEXIT")
	}

	f.exit = true

	return nil
}

// assembleMacroCall assembles the statement at c if it's a macro call: its
// first word names a defined macro. It reports handled=false, leaving c
// untouched, if it isn't one, and the statement is then an instruction.
// (Macros are tried before instructions, so a macro can take an
// instruction's name and replace it.)
func (a *Assembler) assembleMacroCall(c *cursor) (handled bool, err error) {
	save := c.pos
	c.skipBlanks()

	m, ok := a.macros[scanName(c)]
	if !ok || (!c.atEnd() && !isBlank(c.peek()) && c.peek() != ',') {
		c.pos = save

		return false, nil
	}

	return true, a.expandMacro(m, c)
}

// expandMacro expands a call of m whose arguments are at c, and assembles
// the expansion.
func (a *Assembler) expandMacro(m *macroDef, c *cursor) error {
	a.listOp(m.name)
	a.listCall()
	// A library macro's size is set here, at its call: its definition
	// was read from the library, which records nothing.
	if e := a.xrefRefer(xrefMacros, m.name, ""); e != nil {
		e.size = macroSize(m.body)
	}

	if a.expansions() >= maxExpansionDepth {
		return vmserrors.New(vmserrors.VAX_MACRODEPTH, maxExpansionDepth)
	}

	// The arguments are passed as written, case and all.
	actuals, err := a.parseActuals(a.caseCursor(c), m)
	if err != nil {
		return err
	}

	values, positional, err := a.bind(m, actuals)
	if err != nil {
		return err
	}

	lines := make([]string, len(m.body))
	for i, line := range m.body {
		lines[i] = substitute(line, m, values)
	}

	f := &sourceFrame{
		kind:      sourceMacro,
		name:      m.name,
		expansion: &expansion{def: m, positional: positional},
		end:       m.end,
		hasEnd:    m.hasEnd,
	}

	return a.runSource(f, lines)
}
