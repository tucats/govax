package asm

import (
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements MACRO-32's repeat blocks (docs/PHASE-28.md,
// subtask 3): .REPEAT (or .REPT), .IRP, and .IRPC, each ended by .ENDR.
//
// # Repeat blocks, for a reader new to VAX MACRO
//
// A repeat block is a piece of source assembled several times over, in
// place. .REPEAT gives the count:
//
//	        .REPEAT 3
//	        .BYTE   0
//	        .ENDR                   ; assembles .BYTE 0 three times
//
// .IRP ("indefinite repeat") is like a macro with one formal argument,
// called once for each actual argument in a list:
//
//	        .IRP    REG,<R2,R3,R4>
//	        CLRL    REG
//	        .ENDR                   ; CLRL R2, then CLRL R3, then CLRL R4
//
// and .IRPC is the same, once for each character of a string:
//
//	        .IRPC   CH,<ABC>
//	        .ASCII  /CH/
//	        .ENDR                   ; .ASCII /A/, /B/, then /C/
//
// The block's lines (its "range") are collected up to its .ENDR the same
// way a macro's body is collected up to its .ENDM (collectDefinition),
// and then each repetition is assembled as a new source on the source
// stack (a sourceRepeat frame), with the formal argument, if any,
// substituted as a macro's arguments are. .MEXIT in a repetition ends it
// and the rest of the block.

// repeatBlock is a repeat block whose range is being collected.
type repeatBlock struct {
	// directive is ".REPEAT", ".IRP", or ".IRPC", for error messages.
	directive string
	// line is the directive's line, which errors in the block name.
	line int
	// values holds, for each repetition, its formal argument's value (for
	// .IRP and .IRPC) or nothing (for .REPEAT).
	values [][]string
}

// pseudoRepeat assembles .REPEAT expression (or .REPT): a repeat block
// whose range is assembled expression times, and not at all when that's
// zero or less. The expression must be absolute and already defined.
func (a *Assembler) pseudoRepeat(c *cursor) error {
	n, err := a.exprNoForward(c)
	if err != nil {
		return err
	}

	c.skipBlanks()

	if !c.atEnd() {
		return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
	}

	count := int(int32(n))
	values := make([][]string, max(count, 0))

	a.startRepeat(".REPEAT", nil, values)

	return nil
}

// pseudoIrp assembles .IRP symbol,<argument list>: a repeat block
// assembled once for each argument in the list, with symbol replaced by
// that argument. The arguments are read as a macro call's are (commas
// make null arguments, <...> and ^x...x delimit, \symbol passes a value),
// and an empty list assembles nothing.
func (a *Assembler) pseudoIrp(c *cursor) error {
	name, list, err := a.repeatHead(c)
	if err != nil {
		return err
	}

	actuals, err := a.parseActuals(newCursor(list), &macroDef{})
	if err != nil {
		return err
	}

	values := make([][]string, len(actuals))
	for i, act := range actuals {
		values[i] = []string{act.text}
	}

	a.startRepeat(".IRP", []formal{{name: name}}, values)

	return nil
}

// pseudoIrpc assembles .IRPC symbol,<string>: a repeat block assembled
// once for each character of the string, with symbol replaced by that
// character.
func (a *Assembler) pseudoIrpc(c *cursor) error {
	name, text, err := a.repeatHead(c)
	if err != nil {
		return err
	}

	values := make([][]string, len(text))
	for i := range len(text) {
		values[i] = []string{text[i : i+1]}
	}

	a.startRepeat(".IRPC", []formal{{name: name}}, values)

	return nil
}

// repeatHead reads what follows .IRP or .IRPC: the formal argument's name,
// a comma or blanks, and one argument, read as a macro call's actual
// argument is (so its delimiters are removed).
func (a *Assembler) repeatHead(c *cursor) (name string, arg string, err error) {
	c.skipBlanks()

	name = scanName(c)
	if name == "" {
		return "", "", vmserrors.New(vmserrors.VAX_BADFORMAL, c.rest())
	}

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()
		c.skipBlanks()
	}

	if !c.atEnd() {
		if arg, err = a.scanActual(c); err != nil {
			return "", "", err
		}
	}

	c.skipBlanks()

	if !c.atEnd() {
		return "", "", vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
	}

	return name, arg, nil
}

// startRepeat starts collecting a repeat block's range, as .MACRO starts
// collecting a macro's body.
func (a *Assembler) startRepeat(directive string, formals []formal, values [][]string) {
	a.defining = &definition{
		def:    &macroDef{name: directive, formals: formals},
		repeat: &repeatBlock{directive: directive, line: a.line, values: values},
	}
}

// assembleRepeat assembles a repeat block whose range has been collected:
// each repetition, with its value of the formal argument substituted, as
// a new source. .MEXIT in a repetition ends the block.
//
// The repetitions are named, in an error's location, by the block's first
// line (its directive), so a.line is left at that line.
func (a *Assembler) assembleRepeat(d *definition) error {
	if a.expansions() >= maxExpansionDepth {
		return vmserrors.New(vmserrors.VAX_MACRODEPTH, maxExpansionDepth)
	}

	a.line = d.repeat.line

	for i, values := range d.repeat.values {
		lines := make([]string, len(d.def.body))
		for k, line := range d.def.body {
			lines[k] = substitute(line, d.def, values)
		}

		f := &sourceFrame{kind: sourceRepeat, name: d.repeat.directive, repetition: i + 1}

		if err := a.runSource(f, lines); err != nil {
			return err
		}

		if f.exit || a.stop {
			break
		}
	}

	return nil
}

// pseudoEndr assembles an .ENDR that no repeat block started
// (collectDefinition takes every .ENDR that ends one).
func (a *Assembler) pseudoEndr(*cursor) error {
	return vmserrors.New(vmserrors.VAX_NOTINREPEAT, ".ENDR")
}
