package ir

import "github.com/tucats/govax/internal/fortran/sema"

// EXPONENT and FRACTION, the numeric model's two functions written in line
// (sema's model.go has all seven). Each VAX format keeps its exponent field,
// the model's exponent plus a bias, in the first word of the value, below the
// sign bit:
//
//	F, D   bits 7 to 14   bias 128
//	G      bits 4 to 14   bias 1024
//	H      bits 0 to 14   bias 16384
//
// and an exponent field of 0 is the value 0 (or, with the sign set, a
// reserved operand). X goes into a temporary of its own, whose first longword
// and first word the code then reads and changes as integers: EXPONENT(X) is
//
//	MOVF    X,$T1
//	EXTZV   #7,#8,$T1,R0      the field
//	BEQL    1$                0 for 0
//	SUBL2   #128,R0
//
// and FRACTION(X) X with the field the bias's where it isn't 0, BICW2 of the
// field's bits and BISW2 of the bias's, which in every format is ^X4000.

// modelField is where a REAL type's exponent field is in its first word, and
// its bias.
var modelField = map[Type]struct{ pos, size, bias int32 }{
	Float: {7, 8, 128}, Double: {7, 8, 128}, GFloat: {4, 11, 1024}, HFloat: {0, 15, 16384},
}

// modelPlace is x in a temporary of its own, as its own type, and the
// temporary's first longword and first word.
func (l *lowerer) modelPlace(x Expr) (value, long, word *Mem) {
	t := l.textTemp(x.Type().Size())

	v, lw, w := *t, *t, *t
	v.T, lw.T, w.T = x.Type(), Long, Word

	l.emit(&Assign{origin: l.origin(), Dst: &v, Src: x})

	return &v, &lw, &w
}

// exponent is EXPONENT(X), of the INTEGER type t.
func (l *lowerer) exponent(e *sema.IntrinsicRef, t Type) Expr {
	x := l.expr(e.Args[0])
	f := modelField[x.Type()]
	_, long, _ := l.modelPlace(x)

	v := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: v, Src: field(&Load{long}, LongConst(f.pos), LongConst(f.size))})
	l.choose(&Cmp{Ne, &Load{v}, LongConst(0)}, v, arith(Sub, Long, &Load{v}, LongConst(f.bias)))

	return convertTo(t, &Load{v})
}

// fraction is FRACTION(X).
func (l *lowerer) fraction(e *sema.IntrinsicRef) Expr {
	x := l.expr(e.Args[0])
	f := modelField[x.Type()]
	value, long, word := l.modelPlace(x)

	mask := (int32(1)<<f.size - 1) << f.pos
	cleared := arith(And, Word, &Load{word}, IntConst(Word, int32(int16(^mask))))
	l.choose(&Cmp{Ne, field(&Load{long}, LongConst(f.pos), LongConst(f.size)), LongConst(0)}, word,
		arith(Or, Word, cleared, IntConst(Word, f.bias<<f.pos)))

	return &Load{value}
}
