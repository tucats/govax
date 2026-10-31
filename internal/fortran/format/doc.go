// Package format reads a FORTRAN format specification, the text of a
// FORMAT statement or of a run-time format: (I5, 2X, F8.2), say. It is
// the compiler's check of a FORMAT statement's syntax (docs/PHASE-80 -
// FORTRAN formatted IO.md, Decision 3): the same grammar is written again
// in MACRO in FTNRTL, which interprets formats when the program runs, and
// both are held to what VMS's FORTRAN run-time library accepted and
// refused in the probe (testdata/for80, its "Syntax" section).
//
// # What a format is
//
// A format is a parenthesized list of edit descriptors (chapter 8 of the
// 1988 VAX FORTRAN Language Reference Manual). Some take a value from the
// I/O list and convert it, the data descriptors:
//
//	Iw[.m] Ow[.m] Zw[.m]	integers in decimal, octal, hexadecimal
//	Fw.d Ew.d[Ee] Dw.d Gw.d[Ee]	reals
//	Lw	logicals
//	Aw	characters
//
// each with an optional repeat count before it (3I5 is I5,I5,I5) and, as
// a VAX extension, with its numbers optional (I alone takes the item's
// default width, Table 8-4). The rest place things in the record or
// change how the data descriptors work: 'text' and nH text, nX, Tn,
// TLn, TRn, kP, Q, S, SP, SS, BN, BZ, /, :, and $. A parenthesized list
// may itself be an item, a group, with a repeat count of its own:
// 2(I3,F6.1).
//
// Blanks mean nothing in a format, outside a character constant and an
// H's text: ( 1 X , I 3 ) is (1X,I3). Lowercase letters are capitals.
//
// # Separators
//
// The manual says items are "generally" separated by commas. What VMS
// accepts is more exact, and this package follows it: a comma is
// optional before and after /, :, $, a character constant, and an H
// constant, and after kP (2PF8.2, 2P3I3); between any other two items
// it is required, so (1X I3) is an error, as (1XI3I3) is, there being no
// way to tell where one item ends. Extra commas are harmless: (1X,,I3)
// and (,) are accepted.
//
// # Numbers
//
// A repeat count and a width are from 1 to 32767 (but I0 and its kind
// are accepted, and fail when used: VMS's choice); m, d, and e are up to
// 255, e at least 1; kP is from -128 to 127, the only number that may
// have a sign; nX, Tn, TLn, and TRn take 0 to 32767, and X alone is 1X;
// nH needs n from 1. In a FORMAT statement each number may instead be a
// variable format expression, <expr> (section 8.3.2), but for H's count:
// the expression is kept as text, with its offset, for the compiler to
// read (Format.VFEs). A run-time format may not have one.
//
// # What comes out
//
// Parse returns the items as a tree (Item, Group's Items), each with the
// offset in the text where it starts, or an *Error with the offset of the
// character that makes the format wrong, so that the compiler's message
// can point at it.
package format
