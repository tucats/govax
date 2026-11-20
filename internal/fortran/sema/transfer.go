package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// TRANSFER(SOURCE, MOLD, SIZE) (the Fortran 90 standard's 13.13.108; docs/
// planning/PHASE-108 - Fortran 90 intrinsic procedures.md, finding 8): SOURCE's
// bytes, as memory holds them, taken as a value of MOLD's type. Only MOLD's
// type and rank matter, never its value. The result is
//
//   - a scalar, if MOLD is a scalar and SIZE is left out;
//   - otherwise an array of rank one, of SIZE elements if SIZE is given, else
//     of as few as hold all of SOURCE's bytes.
//
// A result longer than SOURCE has an undefined tail, which is zero here (the
// author's answer 5); a shorter one is SOURCE's leading part. So, on the VAX,
//
//	TRANSFER(1.0, 0)            16512 (^X4080, F_floating's 1.0)
//	TRANSFER('ABCD', 0)         1145258561 (^X44434241)
//	TRANSFER(1.0D0, (/0/))      (/ 16512, 0 /)
//	TRANSFER((/1, 2, 3/), 0D0, 2)   two REAL*8s, the second's last half 0
//
// Of constants it is folded (transferValue), so a PARAMETER or an initial value
// may use it (7.1.6.1 (5)). Otherwise a scalar is an IntrinsicRef of
// TransferOp, its one argument SOURCE, and an array an ArrayFunc named
// TRANSFER, Array SOURCE and Other SIZE, whose value lowering computes into a
// temporary before an array statement's loops (Transformed; ir/lowertransfer.go).
// Either way the bytes are moved as bytes, never as MOLD's type, so bits that
// are a reserved operand in MOLD's floating format don't fault in TRANSFER.

// transfer analyzes a reference to TRANSFER, its arguments in its dummies'
// order (SOURCE, MOLD, SIZE), nil for one left out.
func (a *analyzer) transfer(e *ast.Apply, args []ast.Expr) Expr {
	source, mold := a.operand(args[0]), a.operand(args[1])
	if source.Type().Kind == Invalid || mold.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	t := mold.Type()

	// A MOLD whose length is known only when the program runs (a passed-length
	// dummy) would make the result's length that, which only a few of the
	// expressions that take CHARACTER values could have; and one of length
	// zero makes a value of none, or (with no SIZE) an array of no size.
	if t.Kind == Character && t.Size == 0 {
		a.notImplForm(mold.Position(), "TRANSFER with a MOLD whose length is zero or known only when the program runs")

		return a.bad(e.Pos)
	}

	var size Expr

	if len(args) > 2 && args[2] != nil {
		size = a.value(args[2])
		if size.Type().Kind == Invalid {
			return a.bad(e.Pos)
		}

		if size.Type().Kind != Integer {
			a.report("BADARG", size.Position(), e.Name, "an INTEGER SIZE", size.Type().String())

			return a.bad(e.Pos)
		}

		size = a.convert(size, IntegerType)
	}

	array := rankOf(mold) > 0 || size != nil

	// A CHARACTER array SOURCE of a length known only when the program runs:
	// lowering finds the length only of a name's (as transform.go's).
	if source.Type().Kind == Character && source.Type().Size == 0 && rankOf(source) > 0 {
		switch source.(type) {
		case *ArrayRef, *Section:
		default:
			a.notImplForm(e.Pos, "TRANSFER of a CHARACTER array expression whose length is known only when the program runs")

			return a.bad(e.Pos)
		}
	}

	// Of constants, folded.
	if src, ok := constantBytes(a, source); ok {
		n := -1

		if size != nil {
			if n, ok = constantInt(size); ok {
				n = max(n, 0)
			}
		}

		if ok && foldable(t) {
			c, arr := a.transferValue(src, t, array, n, e.Pos)
			if arr != nil {
				return arr
			}

			return c
		}
	}

	if !array {
		return &IntrinsicRef{exprBase: exprBase{T: t, Pos: e.Pos}, Name: e.Name, Op: TransferOp, Args: []Expr{source}}
	}

	f := &ArrayFunc{exprBase: exprBase{T: t, Pos: e.Pos}, Name: e.Name, Array: source, Other: size}

	// The extent: SIZE's, if it is a constant; else, with no SIZE, as many
	// elements as hold SOURCE's bytes, if their number is known.
	extent := Extent{Of: f, Dim: 0}

	if n, ok := constantInt(size); ok {
		extent = Extent{Size: max(n, 0), Known: true}
	} else if bytes, ok := knownBytes(source); ok && size == nil {
		extent = Extent{Size: (bytes + t.Size - 1) / t.Size, Known: true}
	}

	f.S = Shape{extent}

	return f
}

// zeroLength reports whether e is a CHARACTER entity whose length is the
// constant 0 (a LengthExpr of 0, charlength.go).
func zeroLength(e Expr) bool {
	var s *Symbol

	switch x := e.(type) {
	case *Var:
		s = x.Sym
	case *ArrayRef:
		s = x.Sym
	case *Elem:
		s = x.Sym
	}

	return s != nil && s.ZeroLength()
}

// knownBytes is how many bytes the value e is, if that is known when
// compiling: its elements' size times their number.
func knownBytes(e Expr) (int, bool) {
	t := e.Type()
	if t.Kind == Character && t.Size == 0 && !zeroLength(e) {
		return 0, false
	}

	n := t.Size

	for _, x := range e.Shape() {
		if !x.Known {
			return 0, false
		}

		n *= x.Size
	}

	return n, true
}

// foldable reports whether TRANSFER's value of type t is folded when SOURCE
// is a constant: a number, a LOGICAL, or a CHARACTER value. A derived type's
// value is computed when the program runs, its component values being what a
// constant of it needs (Const.Parts).
func foldable(t Type) bool {
	switch t.Kind {
	case Integer, Logical, Real, Complex, Character:
		return true
	}

	return false
}

// constantBytes is the constant e's bytes, all its elements' in array element
// order, if it is a constant (a scalar or an array) of a type whose bytes are
// known.
func constantBytes(a *analyzer, e Expr) ([]byte, bool) {
	switch x := e.(type) {
	case *Const:
		return a.valueBytes(x), true

	case *ArrayConst:
		var b []byte

		for _, c := range x.Elems {
			b = append(b, a.valueBytes(c)...)
		}

		return b, true
	}

	return nil, false
}

// valueBytes is the constant c's bytes as memory holds them: a REAL or COMPLEX
// value's as its format packs it, or its Raw bytes (a typeless constant's, or a
// derived type's whole value); a CHARACTER value's characters, as many as its
// length.
func (a *analyzer) valueBytes(c *Const) []byte {
	if c.Raw != nil {
		return c.Raw
	}

	b := a.constBytes(c)

	if c.T.Kind == Character && len(b) != c.T.Size {
		b = append([]byte(nil), b...)

		for len(b) < c.T.Size {
			b = append(b, ' ')
		}

		b = b[:c.T.Size]
	}

	return b
}

// transferValue is TRANSFER of constants: the bytes src taken as values of
// type t, a scalar unless array, else an array of n elements (n < 0: as few as
// hold src), the bytes past src zero.
func (a *analyzer) transferValue(src []byte, t Type, array bool, n int, pos diag.Position) (*Const, *ArrayConst) {
	m := t.Size

	switch {
	case !array:
		n = 1
	case n < 0:
		n = (len(src) + m - 1) / m
	}

	b := make([]byte, n*m)
	copy(b, src)

	if !array {
		return a.constFromBytes(t, b, pos), nil
	}

	elems := make([]*Const, n)
	for i := range elems {
		elems[i] = a.constFromBytes(t, b[i*m:(i+1)*m], pos)
	}

	return nil, newArrayConst(t, elems, pos, n)
}

// constFromBytes is the constant of type t whose memory holds b, as many bytes
// as t has. A REAL or COMPLEX value whose bytes aren't what its value packs to
// (a reserved operand, or a zero with fraction bits) keeps them as Raw too, so
// that it is moved as they are (ir's RawConst); its value is what the VAX's
// arithmetic would take it as, 0 for both.
func (a *analyzer) constFromBytes(t Type, b []byte, pos diag.Position) *Const {
	c := &Const{exprBase: exprBase{T: t, Pos: pos}}

	switch t.Kind {
	case Integer, Logical:
		var v uint32
		for i := len(b) - 1; i >= 0; i-- {
			v = v<<8 | uint32(b[i])
		}

		switch len(b) {
		case 1:
			c.Int = int32(int8(v))
		case 2:
			c.Int = int32(int16(v))
		default:
			c.Int = int32(v)
		}

	case Real:
		var exact bool

		c.Real, exact = unpackBytes(a.format(t), b)
		if !exact {
			c.Raw = append([]byte(nil), b...)
		}

	case Complex:
		half := len(b) / 2

		re, exactRe := unpackBytes(a.format(t), b[:half])
		im, exactIm := unpackBytes(a.format(t), b[half:])

		c.Real, c.Imag = re, im
		if !exactRe || !exactIm {
			c.Raw = append([]byte(nil), b...)
		}

	case Character:
		c.Text = string(b)
	}

	return c
}

// unpackBytes is the floating value in format f whose memory holds b, and
// whether packing it again gives b: a reserved operand is 0, not exact.
func unpackBytes(f vaxfloat.Format, b []byte) (vaxfloat.Value, bool) {
	var bits vaxfloat.Bits

	for i, x := range b {
		if i < 8 {
			bits.Lo |= uint64(x) << (8 * i)
		} else {
			bits.Hi |= uint64(x) << (8 * (i - 8))
		}
	}

	v, err := vaxfloat.Unpack(f, bits)
	if err != nil {
		return vaxfloat.Value{}, false
	}

	again := floatBytes(f, v)

	return v, string(again) == string(b)
}

// transferIsArray reports whether the reference e to TRANSFER, as a constant
// expression, is an array: whether MOLD is one, or SIZE is given.
func (ev *evaluator) transferIsArray(e *ast.Apply) bool {
	slots, ok := ev.a.intrinsicSlots(e, formsNamed(e.Name), true)
	if !ok || len(slots.args) < 2 || slots.args[1] == nil {
		return false
	}

	return len(slots.args) > 2 && slots.args[2] != nil || ev.isArrayForm(slots.args[1])
}

// transferConst is TRANSFER of constants in a constant expression: a scalar,
// c, or an array, arr, as transferIsArray says.
func (ev *evaluator) transferConst(e *ast.Apply) (c *Const, arr *ArrayConst, err *constErr) {
	slots, ok := ev.a.intrinsicSlots(e, formsNamed(e.Name), ev.quiet)
	if !ok {
		return nil, nil, failed
	}

	args := slots.args
	if len(args) < 2 || len(args) > 3 {
		return nil, nil, ev.report("INTRINARGS", e.Pos, e.Name, "2 or 3 arguments", len(args))
	}

	for _, x := range args {
		if x == nil {
			return nil, nil, ev.report("NULLINTRINARG", e.Pos, e.Name)
		}
	}

	// SOURCE's bytes, and MOLD's type.
	sc, sarr, err := ev.evalAny(args[0])
	if err != nil {
		return nil, nil, err
	}

	mc, marr, err := ev.evalAny(args[1])
	if err != nil {
		return nil, nil, err
	}

	var src []byte

	if sarr != nil {
		src, _ = constantBytes(ev.a, sarr)
	} else {
		src = ev.a.valueBytes(sc)
	}

	var t Type
	if marr != nil {
		t = marr.T
	} else {
		t = mc.T
	}

	if !foldable(t) || t.Size == 0 {
		return nil, nil, notConst("TRANSFER to a value of type "+t.String()+" can't be computed when compiling", e.Pos)
	}

	n := -1

	if len(args) > 2 {
		size, err := ev.eval(args[2])
		if err != nil {
			return nil, nil, err
		}

		if size.T.Kind != Integer {
			return nil, nil, ev.badArg(e, "an INTEGER SIZE", size)
		}

		n = max(int(size.Int), 0)
	}

	c, arr = ev.a.transferValue(src, t, marr != nil || n >= 0, n, e.Pos)

	return c, arr, nil
}
