package ir

import (
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The intrinsic functions written in line (section 6.3 of the language
// manual; sema's intrinsics.go says which). Most are an instruction or two:
// NINT is CVTRFL (rounding, where CVTFL truncates), ZEXT is MOVZWL, the bit
// functions are EXTZV, ASHL, BIS, and BIC, ICHAR is MOVZBL, INDEX is MATCHC.
//
// ABS, MAX, MIN, DIM, and SIGN choose between values, which takes a branch
// (Decision 6 of docs/PHASE-78 - FORTRAN expressions and types.md): they
// are lowered as a function's call is, their blocks before the statement and
// their value in a temporary the statement loads. MAX(A, B, C) is
//
//	B1: assign $T1 = A
//	    branch gt.F(B, $T1) B2 B3
//	B2: assign $T1 = B
//	B3: branch gt.F(C, $T1) B4 B5
//	B4: assign $T1 = C
//	B5: (the statement, which loads $T1)
//
// A statement's blocks can end in the middle of the statement this way, so
// the statements to be done at its end (after: the freeing of a dynamic
// string) wait for its end, past these blocks (divert).

// intrinsic lowers a reference to an intrinsic function whose value is an
// INTEGER, a LOGICAL, or a REAL.
func (l *lowerer) intrinsic(e *sema.IntrinsicRef) Expr {
	t := l.typeOf(e.T)

	switch e.Op {
	case sema.AbsOp:
		return l.absolute(t, l.expr(e.Args[0]))

	case sema.MaxOp, sema.MinOp:
		return l.extreme(e, t)

	case sema.DimOp:
		return l.difference(t, l.expr(e.Args[0]), l.expr(e.Args[1]))

	case sema.SignOp:
		return l.transferSign(t, l.expr(e.Args[0]), l.expr(e.Args[1]))

	case sema.ModOp:
		// X - (X/Y)*Y: DIVL truncates toward zero, so the remainder has X's
		// sign, as MOD's does.
		x, y := l.reusable(l.expr(e.Args[0])), l.reusable(l.expr(e.Args[1]))

		return arith(Sub, t, x, arith(Mul, t, arith(Div, t, x, y), y))

	case sema.NintOp:
		// CVTRxL rounds to a longword; ININT's word is that, converted.
		return convertTo(t, arith(Round, Long, l.expr(e.Args[0]), nil))

	case sema.ZextOp:
		x := l.expr(e.Args[0])
		if x.Type() == t {
			return x
		}

		return arith(Zext, t, x, nil)

	case sema.ImagOp:
		_, im := l.complexParts(e.Args[0])

		return im

	case sema.ShiftOp:
		return l.shift(t, e)

	case sema.BitsOp, sema.BitSetOp, sema.BitClearOp, sema.BitTestOp:
		return l.bitFunction(t, e)

	case sema.CallOp:
		return &Load{l.callIntrinsic(e)}

	case sema.LenOp:
		return l.charLen(e.Args[0])

	case sema.IndexOp:
		return l.index(e)

	case sema.IcharOp:
		// The first character's code, an unsigned byte.
		x := l.chars(e.Args[0])

		return arith(Zext, Long, &Load{x.Mem}, nil)
	}

	panic("ir: an intrinsic function semantic analysis left that lowering can't lower: " + e.Name)
}

// callIntrinsic calls the routine of an intrinsic function written as a call
// (MTHRTL's, or FTNRTL's FTN$IISHFTC and FTN$JISHFTC), returning the
// temporary its value went to. The arguments are by reference, as for any
// FORTRAN function. An INTEGER, a LOGICAL, a REAL*4, a REAL*8, or a COMPLEX*8
// value comes back in R0, or in R0 and R1; a REAL*16 or a COMPLEX*16 value is
// too wide for the registers MTHRTL uses, and goes through a hidden first
// argument, the address of the place for it (Phase 77's survey).
func (l *lowerer) callIntrinsic(e *sema.IntrinsicRef) *Mem {
	t := l.typeOf(e.T)
	result := l.temp(t)

	args := make([]sema.Arg, len(e.Args))
	for i, x := range e.Args {
		args[i] = sema.Arg{Expr: x}
	}

	call := &Call{origin: l.origin(), Routine: e.Routine, Args: l.args(args)}

	if t == HFloat || t == Octa {
		call.Args = append([]Arg{{Ref: result}}, call.Args...)
	} else {
		call.Result = result
	}

	l.emit(call)

	return result
}

// divert ends the current block with a branch on cond, in the middle of a
// statement: unlike branch, it leaves the statement's end's statements
// (after) for its end.
func (l *lowerer) divert(cond Expr, t, f *Block) {
	if l.cur == nil {
		l.begin(l.newBlock())
	}

	l.cur.End = &Branch{l.origin(), cond, t, f}
	l.cur = nil
}

// zero is the constant 0 of the integer or floating type t.
func zero(t Type) *Const {
	if t.Floating() {
		return RealConst(t, vaxfloat.Value{})
	}

	return IntConst(t, 0)
}

// choose sets v to x when cond holds: a block of its own that the branch
// on cond goes to, which goes on to the next one.
func (l *lowerer) choose(cond Expr, v *Mem, x Expr) {
	set, next := l.newBlock(), l.newBlock()
	l.divert(cond, set, next)
	l.begin(set)
	l.emit(&Assign{l.origin(), v, x})
	l.begin(next)
}

// absolute is |x|: x in a temporary, negated if it is less than zero.
func (l *lowerer) absolute(t Type, x Expr) Expr {
	v := l.temp(t)
	l.emit(&Assign{l.origin(), v, x})
	l.choose(&Cmp{Lt, &Load{v}, zero(t)}, v, arith(Neg, t, &Load{v}, nil))

	return &Load{v}
}

// extreme is MAX or MIN of the arguments: the first in a temporary, which
// each after it replaces if it is greater (or less).
func (l *lowerer) extreme(e *sema.IntrinsicRef, t Type) Expr {
	op := Gt
	if e.Op == sema.MinOp {
		op = Lt
	}

	v := l.temp(t)
	l.emit(&Assign{l.origin(), v, l.expr(e.Args[0])})

	for _, a := range e.Args[1:] {
		x := l.reusable(l.expr(a))
		l.choose(&Cmp{op, x, &Load{v}}, v, x)
	}

	return &Load{v}
}

// difference is DIM(x, y): x - y if x is greater, else 0. It compares rather
// than subtracting and testing the difference, which for INTEGERs could wrap
// around.
func (l *lowerer) difference(t Type, x, y Expr) Expr {
	x, y = l.reusable(x), l.reusable(y)

	v := l.temp(t)
	l.emit(&Assign{l.origin(), v, zero(t)})
	l.choose(&Cmp{Gt, x, y}, v, arith(Sub, t, x, y))

	return &Load{v}
}

// transferSign is SIGN(x, y): |x| if y is zero or more, -|x| if y is less.
// x is negated when its sign isn't y's, which takes one negation:
//
//	B1: assign $T1 = X
//	    branch lt(Y, 0) B3 B2
//	B2: branch lt($T1, 0) B4 B5        (y not negative: x mustn't be)
//	B3: branch gt($T1, 0) B4 B5        (y negative: x must be)
//	B4: assign $T1 = neg($T1)
//	B5:
func (l *lowerer) transferSign(t Type, x, y Expr) Expr {
	// A constant y decides it now: |x|, or -|x|.
	if c, ok := y.(*Const); ok {
		v := l.absolute(t, x)
		if t.Integer() && c.Int() < 0 || t.Floating() && c.Real().Sign() < 0 {
			return arith(Neg, t, v, nil)
		}

		return v
	}

	v := l.temp(t)
	l.emit(&Assign{l.origin(), v, x})

	positive, negative, flip, join := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()

	l.divert(&Cmp{Lt, y, zero(t)}, negative, positive)
	l.begin(positive)
	l.divert(&Cmp{Lt, &Load{v}, zero(t)}, flip, join)
	l.begin(negative)
	l.divert(&Cmp{Gt, &Load{v}, zero(t)}, flip, join)
	l.begin(flip)
	l.emit(&Assign{l.origin(), v, arith(Neg, t, &Load{v}, nil)})
	l.begin(join)

	return &Load{v}
}

// longword is an INTEGER value as a longword whose high bits past it are
// clear: a word's MOVZWL, which EXTZV can take a field of from a register
// without reading past the word in memory.
func longword(x Expr) Expr {
	if x.Type() == Long {
		return x
	}

	return arith(Zext, Long, x, nil)
}

// shift is ISHFT(I, N), N a constant (section D.5.2): a logical shift, the
// places vacated filled with zeros, by ASHL to the left and by EXTZV (the
// bits that stay, zero-extended) to the right. A shift of the whole value's
// width or more leaves zero. An INTEGER*2's is done in a longword, of which
// it is the low word.
func (l *lowerer) shift(t Type, e *sema.IntrinsicRef) Expr {
	x := l.expr(e.Args[0])
	n := l.expr(e.Args[1]).(*Const).Int()
	width := int32(8 * t.Size())

	var v Expr

	switch {
	case n == 0:
		return x
	case n >= width || n <= -width:
		return IntConst(t, 0)
	case n > 0:
		v = arith(Ash, Long, longword(x), LongConst(n))
	default:
		v = field(longword(x), LongConst(-n), LongConst(width+n))
	}

	return convertLow(t, v)
}

// convertLow is the integer x as the shorter integer type t: its low bits.
func convertLow(t Type, x Expr) Expr {
	if x.Type() == t {
		return x
	}

	return arith(Low, t, x, nil)
}

// bitFunction is IBITS, IBSET, IBCLR, or BTEST of an INTEGER*2 or INTEGER*4,
// whose type a is: the field or the bit from bit POS, by EXTZV, or a mask of
// it, 1 shifted left POS places by ASHL, set by BIS or cleared by BIC.
// IIBITS and IIBCLR compute in a word and return an INTEGER*4 (Table D-3, and
// the author's answer 3 in Phase 78's survey): the word's value,
// sign-extended. BITEST's LOGICAL*2 is the low word of BJTEST's -1 or 0.
func (l *lowerer) bitFunction(t Type, e *sema.IntrinsicRef) Expr {
	x := l.expr(e.Args[0])
	a := x.Type()
	pos := convertTo(Long, l.expr(e.Args[1]))

	switch e.Op {
	case sema.BitsOp:
		size := convertTo(Long, l.expr(e.Args[2]))

		return convertTo(t, convertLow(a, field(longword(x), pos, size)))

	case sema.BitTestOp:
		return convertLow(t, arith(Neg, Long, field(longword(x), pos, LongConst(1)), nil))
	}

	mask := convertLow(a, arith(Ash, Long, LongConst(1), pos))

	if e.Op == sema.BitSetOp {
		return convertTo(t, arith(Or, a, x, mask))
	}

	return convertTo(t, arith(And, a, x, arith(Not, a, mask, nil)))
}

// charLen is a CHARACTER value's length: a concatenation's, its parts' sum,
// with no temporary built.
func (l *lowerer) charLen(e sema.Expr) Expr {
	if c, ok := e.(*sema.Concat); ok {
		var n Expr
		for _, p := range c.Parts {
			n = arith(Add, Long, n, l.charLen(p))
		}

		return n
	}

	return l.chars(e).Len
}

// index is INDEX(C1, C2): MATCHC's search for C2 in C1 (CharIndex), into a
// temporary, since MATCHC changes R0 to R3. Its value is C1's length plus 1,
// less C2's, less the bytes of C1 after the match, which MATCHC leaves in R2;
// the first part, computed first, is a constant or a temporary's load.
func (l *lowerer) index(e *sema.IntrinsicRef) Expr {
	x, y := l.chars(e.Args[0]), l.chars(e.Args[1])

	base := arith(Sub, Long, arith(Add, Long, x.Len, LongConst(1)), y.Len)
	if _, ok := base.(*Const); !ok {
		base = &Load{l.value(base)}
	}

	v := l.temp(Long)
	l.emit(&Assign{l.origin(), v, &CharIndex{x, y, base}})

	return &Load{v}
}

// char is CHAR(I) as a CHARACTER value: I's low byte, in a one-byte
// temporary (MOVB, which takes a longer value's low byte; Table D-3's
// argument is from 0 to 255).
func (l *lowerer) char(e *sema.IntrinsicRef) Text {
	v := l.temp(Byte)
	l.emit(&Assign{l.origin(), v, convertLow(Byte, l.expr(e.Args[0]))})

	return Text{v, LongConst(1)}
}
