package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Fortran 90's character functions (sema/charfuncs.go says which way each
// reference goes; docs/planning/PHASE-108 - Fortran 90 intrinsic
// procedures.md, finding 3).
//
// LEN_TRIM, and SCAN and VERIFY of one constant character, are a loop in line
// over the string's bytes, its address and length in temporaries. LEN_TRIM(S)
// from the back is
//
//	B1: assign $T1 = addr(S)
//	    assign $T2 = len(S)
//	B2: branch eq($T2, 0) B4 B3            (none left: 0)
//	B3: branch ne(-1(@$T1)[$T2], ' ') B4 B5
//	B5: assign $T2 = sub($T2, 1)
//	    jump B2
//	B4: (the statement, which loads $T2)
//
// and a search from the front counts a position up from 1 to the length, 0 if
// it passes the end. TRIM(S) is S's own text, as long as LEN_TRIM says: no
// storage of its own. ADJUSTL, ADJUSTR, and REPEAT make new text, in a
// temporary of the statement's as long as the value can be, or in a dynamic
// string when that isn't known, which FTNRTL's routine fills: called with the
// result's descriptor first, then the arguments, a string by descriptor and
// REPEAT's count by reference.

// blank is the character LEN_TRIM and TRIM pass over.
const blank = ' '

// search is where in the text t the byte c is (equal) or another than c is
// (not equal): the first such position counted from 1, or with back the last;
// 0 if none.
func (l *lowerer) search(t Text, c byte, equal, back bool) Expr {
	at, n := l.temp(Long), l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: at, Src: &Addr{t.Mem}})
	l.emit(&Assign{origin: l.origin(), Dst: n, Src: t.Len})

	op := Ne
	if equal {
		op = Eq
	}

	// The byte at position p, counted from 1: -1(at)[p].
	byteAt := func(p *Mem) Expr {
		return &Load{&Mem{Ptr: &Load{at}, Offset: -1, Index: &Load{p}, T: Byte}}
	}

	loop, test, step, done := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()

	if back {
		// From the end, n down to 1: n is the answer where the loop stops.
		l.begin(loop)
		l.divert(&Cmp{Eq, &Load{n}, LongConst(0)}, done, test)
		l.begin(test)
		l.divert(&Cmp{op, byteAt(n), IntConst(Byte, int32(c))}, done, step)
		l.begin(step)
		l.emit(&Assign{origin: l.origin(), Dst: n, Src: arith(Sub, Long, &Load{n}, LongConst(1))})
		l.cur.End = &Jump{l.origin(), loop}
		l.cur = nil
		l.begin(done)

		return &Load{n}
	}

	// From the start, p up from 1 to n; past n, 0.
	p, none := l.temp(Long), l.newBlock()
	l.emit(&Assign{origin: l.origin(), Dst: p, Src: LongConst(1)})
	l.begin(loop)
	l.divert(&Cmp{Gt, &Load{p}, &Load{n}}, none, test)
	l.begin(test)
	l.divert(&Cmp{op, byteAt(p), IntConst(Byte, int32(c))}, done, step)
	l.begin(step)
	l.emit(&Assign{origin: l.origin(), Dst: p, Src: arith(Add, Long, &Load{p}, LongConst(1))})
	l.cur.End = &Jump{l.origin(), loop}
	l.cur = nil
	l.begin(none)
	l.emit(&Assign{origin: l.origin(), Dst: p, Src: LongConst(0)})
	l.begin(done)

	return &Load{p}
}

// searchFunction is LEN_TRIM, or SCAN or VERIFY of a one-character constant
// SET and a constant BACK, in line.
func (l *lowerer) searchFunction(e *sema.IntrinsicRef) Expr {
	t := l.chars(e.Args[0])

	if e.Op == sema.LenTrimOp {
		return l.search(t, blank, false, true)
	}

	c := e.Args[1].(*sema.Const).Text[0]
	back := e.Args[2].(*sema.Const).Truth()

	return l.search(t, c, e.Op == sema.ScanOp, back)
}

// trim is TRIM(S): S's text, as long as LEN_TRIM(S). Its address is kept in a
// temporary, as the loop has it.
func (l *lowerer) trim(e *sema.IntrinsicRef) Text {
	t := l.chars(e.Args[0])

	at := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: at, Src: &Addr{t.Mem}})

	n := l.search(Text{&Mem{Ptr: &Load{at}, T: Byte}, t.Len}, blank, false, true)

	return Text{&Mem{Ptr: &Load{at}, T: Byte}, n}
}

// madeByRoutine is ADJUSTL, ADJUSTR, or REPEAT: FTNRTL's routine called with a
// descriptor of the value's place first. ADJUSTL's and ADJUSTR's value is as
// long as the string, in a temporary when that is known (a dynamic string, of
// that length, when not). REPEAT's is in a temporary when its length is known;
// otherwise FTN$REPEAT gets an empty dynamic string's descriptor and makes the
// string itself, once it has checked the count.
func (l *lowerer) madeByRoutine(e *sema.IntrinsicRef) Text {
	s := l.chars(e.Args[0])

	var (
		dst  *Mem // the value's descriptor
		text Text // and the value
	)

	switch {
	case e.T.Size > 0:
		text = Text{l.textTemp(e.T.Size), LongConst(int32(e.T.Size))}
		dst = l.describeText(&Descriptor{DType: DTypeT, Class: ClassS}, text)

	case e.Op == sema.RepeatOp:
		c := l.descriptorCell(&Descriptor{DType: DTypeT, Class: ClassD})
		dst = &Mem{Cell: c, T: Long}
		l.after = append(l.after, &Call{origin: l.origin(), Routine: libFreeString, Args: []Arg{{Ref: dst}}})
		text = Text{
			&Mem{Ptr: &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, T: Byte},
			&Op{Zext, Long, &Load{&Mem{Cell: c, T: Word}}, nil},
		}

	default:
		n := l.reusable(s.Len)
		s.Len = n
		text = Text{l.dynamicText(n), n}
		dst = l.describeText(&Descriptor{DType: DTypeT, Class: ClassS}, text)
	}

	args := []Arg{{Ref: dst}, {Ref: l.describeText(&Descriptor{DType: DTypeT, Class: ClassS}, s)}}
	if e.Op == sema.RepeatOp {
		args = append(args, Arg{Ref: l.value(l.expr(e.Args[1]))})
	}

	l.emit(&Call{origin: l.origin(), Routine: e.Routine, Args: args})

	return text
}
