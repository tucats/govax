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
	text, err := a.messageText(c)
	if err != nil {
		return err
	}

	return vmserrors.New(vmserrors.VAX_GENERR, text)
}

// pseudoWarn assembles .WARN.
func (a *Assembler) pseudoWarn(c *cursor) error {
	text, err := a.messageText(c)
	if err != nil {
		return err
	}

	a.warn(vmserrors.New(vmserrors.VAX_GENWRN, text))

	return nil
}

// pseudoPrintMACRO assembles MACRO-32's .PRINT.
func (a *Assembler) pseudoPrintMACRO(c *cursor) error {
	text, err := a.messageText(c)
	if err != nil {
		return err
	}

	a.messages = append(a.messages, text)
	a.listMessage(text)

	return nil
}

// messageText reads a message directive's optional expression and returns
// the message: the expression's value in decimal, if it isn't zero, then
// the comment, as written but for trailing blanks.
func (a *Assembler) messageText(c *cursor) (string, error) {
	var parts []string

	c.skipBlanks()

	if !c.atEnd() {
		v, err := a.exprNoForward(c)
		if err != nil {
			return "", err
		}

		c.skipBlanks()

		if !c.atEnd() {
			return "", vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
		}

		if v != 0 {
			parts = append(parts, strconv.FormatInt(int64(int32(v)), 10))
		}
	}

	comment := strings.TrimRight(a.comment, " \t")

	// After a value, one blank separates it from the comment, as in the
	// manual's examples ("25 Need larger WORK_AREA").
	if len(parts) > 0 {
		comment = strings.TrimLeft(comment, " \t")
	}

	if comment != "" {
		parts = append(parts, comment)
	}

	return strings.Join(parts, " "), nil
}

// Messages returns the MACRO dialect's .PRINT messages, in the order
// assembly displayed them.
func (a *Assembler) Messages() []string { return a.messages }
