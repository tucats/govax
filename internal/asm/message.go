package asm

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// The message directives: .ERROR, .WARN, and .PRINT [expression] ;comment
// (the MACRO manual, chapter 6). Each displays the expression's value,
// unless it's zero, and the comment: .ERROR as an assembly error, .WARN as
// a warning, and .PRINT as an informational message, which the MACRO
// command displays and which doesn't change the assembly's severity.
// .PRINT's message is displayed bare, with no prefix of MACRO's own: the
// RMS block macros' alignment message carries its own
// ("%MACRO-I-GENINFO, Generated INFO: RMS BLOCK NOT LONGWORD ALIGNED"). The
// comment is the statement's own (see statement), with any argument
// already substituted into it, since a macro's arguments are substituted
// everywhere. It's displayed as written, from just after the ";": real
// MACRO keeps a leading blank, and the second ";" a comment in a macro
// library ends with so that the librarian won't strip it
// (testdata/mar/macros/vax/macros.log: " USERMAC: ..." and "... NOT
// LONGWORD ALIGNED;").
//
// The console dialect has .ERROR and .WARN too; its .PRINT is eVAX's (see
// pseudoPrint).

// pseudoError assembles .ERROR.
func (a *Assembler) pseudoError(c *cursor) error {
	text, _, err := a.messageText(c)
	if err != nil {
		return err
	}

	return vmserrors.New(vmserrors.VAX_GENERR, text)
}

// pseudoWarn assembles .WARN.
func (a *Assembler) pseudoWarn(c *cursor) error {
	text, _, err := a.messageText(c)
	if err != nil {
		return err
	}

	a.warn(vmserrors.New(vmserrors.VAX_GENWRN, text))

	return nil
}

// pseudoPrintMACRO assembles MACRO-32's .PRINT. Its line lists the
// expression's value, as an assignment's does (VMS 7.1's MAC113B, in
// testdata/err113).
func (a *Assembler) pseudoPrintMACRO(c *cursor) error {
	text, value, err := a.messageText(c)
	if err != nil {
		return err
	}

	if value != nil {
		a.listValue(nil, *value)
	}

	a.messages = append(a.messages, text)
	a.messageLines = append(a.messageLines, a.outerLine())
	a.listMessage(text)

	return nil
}

// messageText reads a message directive's optional expression and returns
// the message: the expression's value in decimal, if it isn't zero, then
// the comment, as written but for trailing blanks; and the expression's
// value, nil if there was none.
func (a *Assembler) messageText(c *cursor) (string, *uint32, error) {
	var (
		parts []string
		value *uint32
	)

	c.skipBlanks()

	if !c.atEnd() {
		v, err := a.exprNoForward(c)
		if err != nil {
			return "", nil, err
		}

		c.skipBlanks()

		if !c.atEnd() {
			return "", nil, vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
		}

		value = &v

		if v != 0 {
			parts = append(parts, strconv.FormatInt(int64(int32(v)), 10))
		}
	}

	// After a value, one blank separates it from the comment, which keeps
	// its own leading blank: real MACRO's ".ERROR 27 ; .ERROR with a
	// value" is "Generated ERROR: 27  .ERROR with a value" (errors.lis).
	comment := strings.TrimRight(a.comment, " \t")

	if comment != "" {
		parts = append(parts, comment)
	}

	return strings.Join(parts, " "), value, nil
}

// Messages returns the MACRO dialect's .PRINT messages, in the order
// assembly displayed them.
func (a *Assembler) Messages() []string { return a.messages }

// MessageLines returns the source line of each of Messages, the line an
// Error at the same place would name: the outermost source's, for a .PRINT
// in a macro's expansion.
func (a *Assembler) MessageLines() []int { return a.messageLines }

// outerLine is the line of the outermost source being assembled: a.line, or
// the line that called the macro or block being expanded (see located).
func (a *Assembler) outerLine() int {
	line := a.line

	for k := len(a.sources) - 1; k > 0; k-- {
		line = a.sources[k].callLine
	}

	return line
}
