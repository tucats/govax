package sema

import (
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Constants with no type of their own. An octal or hexadecimal constant
// ('17'O, '1F'X; section 2.2.1.4) and a Hollerith constant (5HHELLO; section
// 2.2.1.7) are only bytes until they are used, and take a type from where they
// are:
//
//   - beside a binary operator, the assignment operator included, the other
//     operand's type: in X + 'FF'X, X's;
//   - where a type is required, as a subscript's is, that type;
//   - as an actual argument, none (arguments.go): four bytes for an octal or
//     hexadecimal one, and a Hollerith one's own length;
//   - anywhere else, INTEGER*4, or INTEGER*2 under /NOI4.
//
// A character constant is a Hollerith constant in a numeric context (section
// 2.2.1.6): in X = 'AB', but not in C = 'AB' or C .EQ. 'AB'.
//
// The two kinds differ in what they do when the bytes and the type's length
// don't match. An octal or hexadecimal constant is a number, so its high end is
// what is added to or cut, and on the VAX that end is the higher addresses; a
// Hollerith constant is text, so what is added to or cut is on the right, which
// is the higher addresses too. Their bytes then are the type's bytes as memory
// holds them, whatever the type: '00004080'X as a REAL*4 is 1.0, and 4HABCD as
// an INTEGER*4 is ^X44434241.
//
// That needs care for a floating type, since not every bit pattern is a number:
// a sign of 1 and an exponent of 0 is a "reserved operand", which faults in any
// floating instruction. Assigned (ev.moved) such bits are moved as an integer of
// the same width, as they should be, and in any other expression they are an
// error (BADTYPELESS), rather than a fault when the program runs (ours).

// untyped is the constant e is, if it has no type until used, parentheses
// aside; and nil if it is anything else.
func untyped(e ast.Expr) ast.Expr {
	switch x := e.(type) {
	case *ast.Paren:
		return untyped(x.X)
	case *ast.TypelessConst, *ast.HollerithConst:
		return x
	}

	return nil
}

// untypedWhat names an untyped constant for a message.
func untypedWhat(e ast.Expr) string {
	if _, ok := e.(*ast.HollerithConst); ok {
		return "a Hollerith constant"
	}

	return "an octal or hexadecimal constant"
}

// typelessValue is an octal or hexadecimal constant's number. It reports
// digits its base doesn't have, and more than the 128 bits it may have. A
// quiet evaluator's report isn't made, so it says only that there is no value.
func (ev *evaluator) typelessValue(e *ast.TypelessConst) (*big.Int, *constErr) {
	v, ok := new(big.Int).SetString(e.Digits, e.Radix)
	if !ok || v.Sign() < 0 {
		what := "octal"
		if e.Radix == 16 {
			what = "hexadecimal"
		}

		return nil, ev.report("BADDIGITS", e.Pos, what)
	}

	if v.BitLen() > 128 {
		return nil, ev.report("TYPELESSLONG", e.Pos)
	}

	return v, nil
}

// untypedAs is the untyped constant e given type t.
func (ev *evaluator) untypedAs(e ast.Expr, t Type) (*Const, *constErr) {
	switch x := e.(type) {
	case *ast.TypelessConst:
		return ev.typelessAs(x, t)
	case *ast.HollerithConst:
		return ev.textAs(x.Value, t, x.Pos)
	}

	return nil, failed
}

// typelessAs is an octal or hexadecimal constant given type t: extended on the
// left with zeros, or cut on the left, "an error ... if any nonzero digits are
// truncated".
func (ev *evaluator) typelessAs(e *ast.TypelessConst, t Type) (*Const, *constErr) {
	v, err := ev.typelessValue(e)
	if err != nil {
		return nil, err
	}

	switch {
	case t.Kind == Character:
		return nil, ev.report("NOCHARVALUE", e.Pos, untypedWhat(e))
	case v.BitLen() > t.Size*8:
		return nil, ev.report("TYPELESSCUT", e.Pos, article(t.String()), t.Size)
	}

	return ev.bytesConst(littleEndian(v, t.Size), t, e.Pos)
}

// textAs is a Hollerith constant's text, or a character constant's in a numeric
// context, given type t: extended on the right with blanks, or cut on the right,
// "an error ... if any characters other than space characters are truncated".
func (ev *evaluator) textAs(text string, t Type, pos diag.Position) (*Const, *constErr) {
	switch {
	case t.Kind == Character:
		return nil, ev.report("NOCHARVALUE", pos, "a Hollerith constant")
	case len(text) > t.Size && strings.Trim(text[t.Size:], " ") != "":
		return nil, ev.report("HOLLCUT", pos, article(t.String()), t.Size)
	}

	b := []byte(strings.Repeat(" ", t.Size))
	copy(b, text)

	return ev.bytesConst(b, t, pos)
}

// numeric is x as an operand with nothing to take its type from: a character
// constant is a Hollerith constant of the default integer type, and anything
// else stays as it is.
func (ev *evaluator) numeric(x *Const) (*Const, *constErr) {
	if x.T.Kind == Character {
		return ev.textAs(x.Text, ev.a.defaultInt, x.Pos)
	}

	return x, nil
}

// bytesConst is the constant of type t that memory holding b holds, lowest
// address first. b is as long as t.
func (ev *evaluator) bytesConst(b []byte, t Type, pos diag.Position) (*Const, *constErr) {
	c := &Const{exprBase: exprBase{t, pos}}

	var err error

	switch t.Kind {
	case Integer, Logical:
		var v uint32

		for i := 0; i < len(b) && i < 4; i++ {
			v |= uint32(b[i]) << (8 * i)
		}

		c.Int = truncate(int32(v), t.Size)

	case Real:
		c.Real, err = vaxfloat.Unpack(ev.a.format(t), bitsOf(b))

	case Complex:
		var err2 error

		f, half := ev.a.format(t), t.Size/2
		c.Real, err = vaxfloat.Unpack(f, bitsOf(b[:half]))
		c.Imag, err2 = vaxfloat.Unpack(f, bitsOf(b[half:]))

		if err == nil {
			err = err2
		}

	default:
		return nil, failed
	}

	if err != nil && !ev.moved {
		return nil, ev.report("BADTYPELESS", pos, t.String())
	}

	if ev.moved && (t.Kind == Real || t.Kind == Complex) {
		c.Raw = b
	}

	return c, nil
}

// bitsOf is bytes as a floating value's vaxfloat.Bits: the first eight in Lo,
// the rest in Hi, each lowest byte first.
func bitsOf(b []byte) vaxfloat.Bits {
	var bits vaxfloat.Bits

	for i, x := range b {
		if i < 8 {
			bits.Lo |= uint64(x) << (8 * i)
		} else {
			bits.Hi |= uint64(x) << (8 * (i - 8))
		}
	}

	return bits
}

// pair is the values of a binary operator's operands, as constants. Each
// untyped one takes the other's type, or the default integer if the other has
// none, and so does a character constant beside a number.
func (ev *evaluator) pair(ex, ey ast.Expr) (*Const, *Const, *constErr) {
	ux, uy := untyped(ex), untyped(ey)

	if (ux != nil) != (uy != nil) {
		u, other := ux, ey
		if uy != nil {
			u, other = uy, ex
		}

		o, err := ev.eval(other)
		if err != nil {
			return nil, nil, err
		}

		if o, err = ev.numeric(o); err != nil {
			return nil, nil, err
		}

		c, err := ev.untypedAs(u, o.T)
		if err != nil {
			return nil, nil, err
		}

		if uy != nil {
			return o, c, nil
		}

		return c, o, nil
	}

	x, err := ev.eval(ex)
	y, err2 := ev.eval(ey)

	if err = worse(err, err2); err != nil {
		return nil, nil, err
	}

	switch cx, cy := x.T.Kind == Character, y.T.Kind == Character; {
	case cx && !cy:
		x, err = ev.textAs(x.Text, y.T, x.Pos)
	case cy && !cx:
		y, err = ev.textAs(y.Text, x.T, y.Pos)
	}

	return x, y, err
}

// Typed expressions.

// untypedValue is the untyped constant e given type t, as an operand. After an
// error it is a bad expression.
func (a *analyzer) untypedValue(e ast.Expr, t Type) Expr {
	c, err := (&evaluator{a: a}).untypedAs(e, t)
	if err != nil {
		return a.bad(e.Position())
	}

	return c
}

// textValue is a character constant in a numeric context, of type t.
func (a *analyzer) textValue(c *Const, t Type) Expr {
	v, err := (&evaluator{a: a}).textAs(c.Text, t, c.Pos)
	if err != nil {
		return a.bad(c.Pos)
	}

	return v
}

// numericConst is e as an operand with nothing to take its type from: a
// character constant is a Hollerith constant of the default integer type.
func (a *analyzer) numericConst(e Expr) Expr {
	if c, ok := e.(*Const); ok && c.T.Kind == Character {
		return a.textValue(c, a.defaultInt)
	}

	return e
}

// operands analyzes a binary operator's two operands. An untyped constant takes
// the other's type, or the default integer's beside another that has none; a
// character constant beside a number is a Hollerith constant of the number's
// type; a Hollerith, octal, or hexadecimal constant can't be a CHARACTER value.
func (a *analyzer) operands(ex, ey ast.Expr) (x, y Expr) {
	ux, uy := untyped(ex), untyped(ey)

	if (ux != nil) != (uy != nil) {
		if uy != nil {
			x = a.value(ex)
			x, y = a.typedBeside(uy, x)

			return x, y
		}

		y = a.value(ey)
		y, x = a.typedBeside(ux, y)

		return x, y
	}

	x, y = a.value(ex), a.value(ey)

	kx, okx := x.(*Const)
	ky, oky := y.(*Const)

	switch cx, cy := x.Type().Kind == Character, y.Type().Kind == Character; {
	case okx && cx && !cy && y.Type().Kind != Invalid:
		x = a.textValue(kx, y.Type())
	case oky && cy && !cx && x.Type().Kind != Invalid:
		y = a.textValue(ky, x.Type())
	}

	return x, y
}

// typedBeside gives the untyped constant u the type of other, the operator's
// other operand, and returns both.
func (a *analyzer) typedBeside(u ast.Expr, other Expr) (Expr, Expr) {
	other = a.numericConst(other)

	switch t := other.Type(); t.Kind {
	case Invalid:
		return other, a.bad(u.Position())

	case Character:
		a.report("NOCHARVALUE", u.Position(), untypedWhat(u))

		return a.bad(other.Position()), a.bad(u.Position())

	default:
		return other, a.untypedValue(u, t)
	}
}

// assignedExpr analyzes e, the value assigned to a place of type t: an untyped
// constant takes t, keeping its bits whatever they are, since it is only moved.
func (a *analyzer) assignedExpr(e ast.Expr, t Type) Expr {
	u := untyped(e)

	switch {
	case u == nil:
		return a.value(e)

	case t.Kind == Character:
		a.report("NOCHARVALUE", u.Position(), untypedWhat(u))

		return a.bad(u.Position())
	}

	c, err := (&evaluator{a: a, moved: true}).untypedAs(u, t)
	if err != nil {
		return a.bad(u.Position())
	}

	return c
}

// movedText is a character constant assigned to a place of type t, a Hollerith
// constant by section 2.2.1.6.
func (a *analyzer) movedText(c *Const, t Type) Expr {
	v, err := (&evaluator{a: a, moved: true}).textAs(c.Text, t, c.Pos)
	if err != nil {
		return nil
	}

	return v
}

// notMoved is e for a place in an expression, not only a move: a statement
// function's value, expanded wherever it is referenced. Bits kept for a move are
// a number again, or BADTYPELESS if they are none.
func (a *analyzer) notMoved(e Expr) Expr {
	c, ok := e.(*Const)
	if !ok || c.Raw == nil {
		return e
	}

	v, err := (&evaluator{a: a}).bytesConst(c.Raw, c.T, c.Pos)
	if err != nil {
		return a.bad(c.Pos)
	}

	return v
}

// RADIX-50.

// radix50Codes are the RADIX-50 characters, each at its code (Table B-2, whose
// values are octal). Code 29 is unassigned, marked by a character no source
// could give, since a constant's text is letters in capitals.
const radix50Codes = " ABCDEFGHIJKLMNOPQRSTUVWXYZ$.\x000123456789"

// radix50Bytes is a RADIX-50 constant's bytes: every three characters a word,
// (i*40+j)*40+k (the manual writes ((i*50+j)*50+k) in octal), lowest byte first
// as a VAX word is. A count that isn't a multiple of 3 has blanks, code 0, to
// make the last word up (ours: the manual's 4RABCD implies it).
func (a *analyzer) radix50Bytes(e *ast.Radix50Const) ([]byte, bool) {
	if n := len(e.Value); n > 12 {
		a.report("RAD50LONG", e.Pos, n)

		return nil, false
	}

	var b []byte

	for i := 0; i < len(e.Value); i += 3 {
		word := 0

		for j := i; j < i+3; j++ {
			code := 0

			if j < len(e.Value) {
				code = strings.IndexByte(radix50Codes, e.Value[j])
				if code < 0 || e.Value[j] == 0 {
					a.report("BADRAD50", e.Pos, string(e.Value[j]))

					return nil, false
				}
			}

			word = word*40 + code
		}

		b = append(b, byte(word), byte(word>>8))
	}

	return b, true
}
