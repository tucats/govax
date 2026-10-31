package ir

import (
	"cmp"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
)

// The FTNRTL routines lowered statements call (Decision 12 of
// docs/PHASE-74 - FORTRAN plan.md).
const (
	ftnStop      = "FTN$STOP"
	ftnPause     = "FTN$PAUSE"
	ftnPrintList = "FTN$PRINT_LIST"
	ftnWriteList = "FTN$WRITE_LIST"
	ftnIOItem    = "FTN$IO_ITEM"
	ftnIOEnd     = "FTN$IO_END"
)

// stmts lowers a list of statements.
func (l *lowerer) stmts(list []sema.Stmt) {
	for _, s := range list {
		l.stmt(s)
	}
}

// stmt lowers one statement. A labeled statement starts a block, since
// a jump may go to it; the temporaries the statement used are given
// back after it.
func (l *lowerer) stmt(s sema.Stmt) {
	l.from = s.Source()

	if _, ok := s.(*sema.End); ok && l.cur == nil && l.from.Base().Label == 0 {
		// END after a RETURN, GO TO, or STOP can't be reached: the
		// usual end of a subprogram (RETURN, END) gets one return.
		return
	}

	if n := l.from.Base().Label; n != 0 {
		l.startLabel(n)
	}

	switch s := s.(type) {
	case *sema.Assign:
		l.assign(s)

	case *sema.Call:
		l.call(s)

	case *sema.Goto:
		l.terminate(&Jump{l.origin(), l.labelBlock(s.Label)})

	case *sema.ComputedGoto:
		l.computedGoto(s)

	case *sema.AssignLabel:
		l.emit(&Assign{l.origin(), l.symbolMem(s.Var), &LabelAddr{l.labelBlock(s.Label)}})

	case *sema.AssignedGoto:
		labels := s.Labels
		if labels == nil {
			labels = l.unit.Assigned
		}

		l.terminate(&IndirectJump{l.origin(), l.symbolMem(s.Var), l.labelBlocks(labels)})

	case *sema.ArithIf:
		l.arithIf(s)

	case *sema.If:
		if s.Block {
			l.blockIf(s)
		} else {
			l.logicalIf(s)
		}

	case *sema.Do:
		l.do(s)

	case *sema.DoWhile:
		l.doWhile(s)

	case *sema.Continue:
		// Nothing to do: CONTINUE is there for its label.

	case *sema.Stop:
		l.stop(s)

	case *sema.Return:
		if s.Alt != nil {
			l.terminate(&Return{origin: l.origin(), Value: l.expr(s.Alt)})
		} else {
			l.returnFromRoutine()
		}

	case *sema.End:
		l.returnFromRoutine()

	case *sema.EntryPoint:
		l.entryPoint(s)

	case *sema.Print:
		l.print(s)
	}

	l.flush()
	l.release()
}

// labelBlocks are the blocks that start at labels.
func (l *lowerer) labelBlocks(labels []int) []*Block {
	blocks := make([]*Block, len(labels))
	for i, n := range labels {
		blocks[i] = l.labelBlock(n)
	}

	return blocks
}

// computedGoto lowers GO TO (s1, ..., sn) e to a Switch on e, which goes
// on to the next statement when e is out of range. A constant e is a
// jump, or nothing at all.
//
//	  switch <e> [L10 L20 L30] B2
//	B2: (the next statement)
func (l *lowerer) computedGoto(s *sema.ComputedGoto) {
	index := l.expr(s.Index)

	if c, ok := index.(*Const); ok {
		if n := int(c.Int()); n >= 1 && n <= len(s.Labels) {
			l.terminate(&Jump{l.origin(), l.labelBlock(s.Labels[n-1])})
		}

		return
	}

	next := l.newBlock()
	l.terminate(&Switch{l.origin(), index, l.labelBlocks(s.Labels), next})
	l.begin(next)
}

// call lowers CALL. With alternate return specifiers, the routine's value says
// where to go on (section 5.9), and anything but 1 to the number of them is the
// next statement, as CASEL has it:
//
//	  call CHECK(...)
//	  switch R0 [L10 L20] B2
//	B2: (the next statement)
//
// What the statement must do last (freeing a concatenation's dynamic string)
// comes before the Switch and uses R0, so then the value waits in a temporary.
func (l *lowerer) call(s *sema.Call) {
	call := &Call{origin: l.origin(), Args: l.args(s.Args)}
	call.Routine, call.Through = l.callee(s.Sub)

	if s.AltReturns == nil {
		l.emit(call)

		return
	}

	var index Expr = &CallValue{}

	if len(l.after) > 0 {
		call.Result = l.temp(Long)
		index = &Load{call.Result}
	}

	l.emit(call)

	next := l.newBlock()
	l.terminate(&Switch{l.origin(), index, l.labelBlocks(s.AltReturns), next})
	l.begin(next)
}

// arithIf lowers IF (e) s1, s2, s3 to a Sign of e. A constant e is a
// jump to its label.
func (l *lowerer) arithIf(s *sema.ArithIf) {
	value := l.expr(s.Cond)
	neg, zero, pos := l.labelBlock(s.Labels[0]), l.labelBlock(s.Labels[1]), l.labelBlock(s.Labels[2])

	if c, ok := value.(*Const); ok {
		sign := c.Real().Sign()
		if c.T.Integer() {
			sign = cmp.Compare(c.Int(), 0)
		}

		to := [3]*Block{neg, zero, pos}[sign+1]

		l.terminate(&Jump{l.origin(), to})

		return
	}

	l.terminate(&Sign{l.origin(), value, neg, zero, pos})
}

// returnValue is what the routine returns in R0: a function's value, the
// last one assigned to its name, unless assigning it put it where the
// caller wanted it (a hidden argument); SS$_NORMAL (1) from a main program,
// for the image's exit status; from a subroutine with alternate returns 0, the
// caller's sign to go on to its next statement; nothing from another subroutine.
func (l *lowerer) returnValue() Expr {
	switch l.r.Kind {
	case Function:
		if l.r.Result == None {
			return nil
		}

		return &Load{l.symbolMem(l.unit.Result)}
	case Main:
		return LongConst(1)
	case Subroutine:
		if l.unit.HasAltReturns() {
			return LongConst(0)
		}
	}

	return nil
}

// assign lowers Target = Value. J = F(X), a function's value stored as
// it is, has the call put the value straight into J, but for a value that
// goes through a hidden argument: the function writes that place as it runs,
// and in H = F(H) would change its own argument. A CHARACTER value is
// moved (CharMove), and a character comparison's value stored as it is
// computed.
func (l *lowerer) assign(s *sema.Assign) {
	if s.Target.Type().Kind == sema.Character {
		l.emit(&CharMove{l.origin(), l.chars(s.Target), l.chars(s.Value)})

		return
	}

	l.store(l.mem(s.Target), s.Value)
}

// store lowers dst = value, for a value that isn't CHARACTER, of dst's type.
func (l *lowerer) store(dst *Mem, value sema.Expr) {
	if c, ok := value.(*sema.Compare); ok && c.X.Type().Kind == sema.Character && dst.T == Long {
		l.emit(&Assign{l.origin(), dst, l.charCompare(c)})

		return
	}

	if f, ok := value.(*sema.FuncRef); ok && !f.T.HiddenResult() {
		l.callFunction(f, dst)

		return
	}

	if value.Type().Kind == sema.Complex {
		l.assignComplex(dst, value)

		return
	}

	l.emit(&Assign{l.origin(), dst, l.expr(value)})
}

// blockIf lowers an IF THEN ... END IF construct: each arm's condition
// branches to its block or to the next arm's test; each block ends by
// jumping past the construct, to the block where END IF is (a jump to
// END IF's label goes there too).
//
//	B1: branch cond1 B2 B3
//	B2: (the IF block)       jump B5
//	B3: branch cond2 B4 B5
//	B4: (the ELSE IF block)  jump B5
//	B5: (after END IF)
func (l *lowerer) blockIf(s *sema.If) {
	var join *Block

	if s.EndIf != nil && s.EndIf.Base().Label != 0 {
		join = l.labelBlock(s.EndIf.Base().Label)
	} else {
		join = l.newBlock()
	}

	for i, arm := range s.Arms {
		l.from = arm.AST

		then := l.newBlock()

		next := join
		if i < len(s.Arms)-1 || s.Else != nil {
			next = l.newBlock()
		}

		l.branch(l.cond(arm.Cond), then, next)
		l.release()

		l.begin(then)
		l.stmts(arm.Body)

		// The jump past the construct is written with the statement
		// that ends the block: the next ELSE IF, the ELSE, or END IF.
		switch {
		case i < len(s.Arms)-1:
			l.from = s.Arms[i+1].AST
		case s.Else != nil:
			l.from = s.Else.AST
		default:
			l.from = s.EndIf
		}

		if next != join {
			l.jump(join)
			l.begin(next)
		}
	}

	if s.Else != nil {
		l.stmts(s.Else.Body)
	}

	l.from = s.EndIf
	l.begin(join)
}

// logicalIf lowers IF (e) st: a branch around st, or for IF (e) GO TO n
// a branch to n.
func (l *lowerer) logicalIf(s *sema.If) {
	arm := s.Arms[0]
	cond := l.cond(arm.Cond)
	next := l.newBlock()

	if len(arm.Body) == 1 {
		if g, ok := arm.Body[0].(*sema.Goto); ok {
			l.branch(cond, l.labelBlock(g.Label), next)
			l.begin(next)

			return
		}
	}

	then := l.newBlock()
	l.branch(cond, then, next)
	l.begin(then)
	l.stmts(arm.Body)
	l.from = s.AST
	l.begin(next)
}

// do lowers a DO loop (section 5.3.1.1). The parameters are evaluated
// first, then the iteration count, (e2 - e1 + e3)/e3 truncated to an
// integer, then the variable is set to e1: in that order, so that a
// parameter that uses the variable sees its value before the loop. A
// count of zero or less skips the body. After each pass through the
// body the variable is stepped and the count decreased, and the loop
// goes round again while the count is above zero. The count and the
// step are computed once, so nothing the body does changes how many
// times it runs.
//
//	  assign $C1 = <count>
//	  assign I = <e1>
//	  branch le.L($C1, 0) B3 B2      (not when the count is a constant)
//	B2: (the body, ending with the terminal statement)
//	  assign I = add.L(I, <e3>)
//	  assign $C1 = sub.L($C1, 1)
//	  branch gt.L($C1, 0) B2 B3
//	B3: (after the loop)
func (l *lowerer) do(d *sema.Do) {
	l.loops++
	n := l.loops

	t := l.typeOf(d.Var.Type)
	v := l.symbolMem(d.Var)

	start := l.expr(d.Start)
	limit := l.expr(d.Limit)
	step := l.expr(d.Step)

	name := loopName(d.Label, d.AST)

	if _, ok := step.(*Const); !ok {
		s := l.loopCell(StepCell, n, t, name)
		l.emit(&Assign{l.origin(), s, step})
		step = &Load{s}
	}

	count := l.loopCell(CountCell, n, Long, name)
	trips := tripCount(t, start, limit, step)

	l.emit(&Assign{l.origin(), count, trips})
	l.emit(&Assign{l.origin(), v, start})
	l.release()

	body := l.newBlock()
	body.Note = name + "'s body"
	exit := l.newBlock()

	switch c, ok := trips.(*Const); {
	case !ok:
		l.branch(&Cmp{Le, &Load{count}, LongConst(0)}, exit, body)
	case c.Int() <= 0:
		l.jump(exit)
	}

	l.begin(body)
	l.stmts(d.Body)
	l.loopEnd(d.Label, d.EndDo)

	l.emit(&Assign{l.origin(), v, arith(Add, t, &Load{v}, step)})
	l.emit(&Assign{l.origin(), count, arith(Sub, Long, &Load{count}, LongConst(1))})
	l.branch(&Cmp{Gt, &Load{count}, LongConst(0)}, body, exit)
	l.begin(exit)
}

// doWhile lowers DO WHILE (e) (section 5.3.2): the test at the top, before
// every pass, the first included, and a jump back to it after the body.
//
//	B1: branch <e> B2 B3
//	B2: (the body, ending with the terminal statement)
//	  jump B1
//	B3: (after the loop)
func (l *lowerer) doWhile(d *sema.DoWhile) {
	name := loopName(d.Label, d.AST)

	test := l.newBlock()
	test.Note = name + "'s test"
	body := l.newBlock()
	body.Note = name + "'s body"
	exit := l.newBlock()

	l.begin(test)
	l.branch(l.cond(d.Cond), body, exit)
	l.release()

	l.begin(body)
	l.stmts(d.Body)
	l.loopEnd(d.Label, d.EndDo)
	l.jump(test)
	l.begin(exit)
}

// loopEnd starts the code that ends a pass through a loop, whose
// terminal statement has the label label or is the END DO statement
// endDo: that statement is where the code comes from, and a labeled END
// DO's label is where it starts, since a jump to END DO goes on to the
// next pass.
func (l *lowerer) loopEnd(label int, endDo ast.Stmt) {
	switch {
	case endDo != nil:
		l.from = endDo

		if n := endDo.Base().Label; n != 0 {
			l.startLabel(n)
		}

	case l.unit.Labels[label] != nil:
		// The terminal statement's own code is already in the body.
		l.from = l.unit.Labels[label].Stmt
	}
}

// loopName names a loop for the notes on its blocks and cells: "DO 10"
// for the loop ending at label 10, "DO (line 12)" for one END DO ends.
func loopName(label int, s ast.Stmt) string {
	if label != 0 {
		return "DO " + strconv.Itoa(label)
	}

	return "DO (line " + strconv.Itoa(s.Position().Line) + ")"
}

// tripCount is a DO loop's iteration count, a Long: (e2 - e1 + e3)/e3,
// computed in the loop variable's type t; a REAL count is converted by
// truncation, as CVTFL does, and a Word loop's is computed in longwords,
// which its parameters' differences can't overflow. With a step of 1 or -1
// the division goes, and constant parameters make a constant count.
func tripCount(t Type, e1, e2, e3 Expr) Expr {
	if t.Integer() && t != Long {
		e1, e2, e3 = arith(Cvt, Long, e1, nil), arith(Cvt, Long, e2, nil), arith(Cvt, Long, e3, nil)
		t = Long
	}

	if t == Long {
		if c, ok := e3.(*Const); ok {
			switch c.Int() {
			case 1:
				return arith(Add, Long, arith(Sub, Long, e2, e1), LongConst(1))
			case -1:
				return arith(Add, Long, arith(Sub, Long, e1, e2), LongConst(1))
			}
		}

		return arith(Div, Long, arith(Add, Long, arith(Sub, Long, e2, e1), e3), e3)
	}

	return arith(Cvt, Long, arith(Div, t, arith(Add, t, arith(Sub, t, e2, e1), e3), e3), nil)
}

// stop lowers STOP and PAUSE: a call of FTN$STOP or FTN$PAUSE, with the
// display's descriptor if there is one. Nothing returns from FTN$STOP;
// FTN$PAUSE returns when the program is to go on (DCL's CONTINUE), and
// the next statement follows.
func (l *lowerer) stop(s *sema.Stop) {
	var args []Arg

	if s.HasDisplay {
		args = []Arg{{Ref: l.textDescriptor(s.Display)}}
	}

	if s.Pause {
		l.emit(&Call{origin: l.origin(), Routine: ftnPause, Args: args})

		return
	}

	l.emit(&Call{origin: l.origin(), Routine: ftnStop, Args: args})
	l.terminate(&NoReturn{l.origin()})
}

// print lowers PRINT * and WRITE (*,*) to Decision 12's calls: the
// statement's begin call with its control block, a call of FTN$IO_ITEM
// with each item's descriptor, and FTN$IO_END, which writes the record.
func (l *lowerer) print(s *sema.Print) {
	routine := ftnPrintList
	if s.Write {
		routine = ftnWriteList
	}

	icb := l.control(Control{Write: s.Write, Unit: DefaultUnit})
	l.emit(&Call{origin: l.origin(), Routine: routine, Args: []Arg{{Ref: icb}}})

	for _, item := range s.Items {
		desc := l.item(item)
		l.emit(&Call{origin: l.origin(), Routine: ftnIOItem, Args: []Arg{{Ref: desc}}})
	}

	l.emit(&Call{origin: l.origin(), Routine: ftnIOEnd})
}

// item is the descriptor of an output list item. A variable, an array
// element, or a whole array is described where it is; a character
// constant by its text; any other CHARACTER value where chars finds it; a
// numeric constant by a datum holding it; any other value is computed
// into a temporary, which is described.
func (l *lowerer) item(e sema.Expr) *Mem {
	t := e.Type()
	d := &Descriptor{DType: l.dtypeOf(t), Class: ClassS, Length: t.Size}

	switch e := e.(type) {
	case *sema.Const:
		if t.Kind == sema.Character {
			return l.textDescriptor(e.Text)
		}

		return l.describe(d, l.constant(l.constOf(e)))

	case *sema.ArrayRef:
		return l.arrayDescriptor(e.Sym)

	case *sema.Var, *sema.Elem:
		if t.Kind != sema.Character {
			return l.describe(d, l.mem(e))
		}
	}

	if t.Kind == sema.Character {
		return l.describeText(d, l.chars(e))
	}

	return l.describe(d, l.value(l.expr(e)))
}

// describeText is a descriptor d of the CHARACTER value at t: describe's,
// when t's length is a constant; otherwise a descriptor cell, into which the
// statement stores the length (a word, CVTLW) and the address.
func (l *lowerer) describeText(d *Descriptor, t Text) *Mem {
	if n, ok := t.Len.(*Const); ok {
		d.Length = int(n.Int())

		return l.describe(d, t.Mem)
	}

	d.Length = 0
	c := l.descriptorCell(d)
	l.emit(&Assign{l.origin(), &Mem{Cell: c, T: Word}, arith(Cvt, Word, t.Len, nil)})
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: PointerOffset, T: Long}, &Addr{t.Mem}})

	return &Mem{Cell: c, T: Long}
}

// textDescriptor is the descriptor of a character constant.
func (l *lowerer) textDescriptor(s string) *Mem {
	return l.describe(&Descriptor{DType: DTypeT, Class: ClassS, Length: len(s)}, l.text(s))
}

// describe is a descriptor d of the place m. If the linker knows m's
// address, the descriptor is a datum; otherwise it is a descriptor
// cell, into which the statement stores m's address (and, for an array,
// A0: the address of the element whose subscripts are all zero, which
// is m's less the offset of the first element from it).
func (l *lowerer) describe(d *Descriptor, m *Mem) *Mem {
	if m.Param == nil && m.Ptr == nil && m.Index == nil {
		d.Target = m

		return l.descriptor(d)
	}

	c := l.descriptorCell(d)
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: PointerOffset, T: Long}, &Addr{m}})

	if d.Class == ClassA {
		first, multiplier := 0, 1

		for _, b := range d.Dims {
			first += b.Lower * multiplier
			multiplier *= b.Upper - b.Lower + 1
		}

		a0 := *m
		a0.Offset -= first * d.Length
		l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: A0Offset, T: Long}, &Addr{&a0}})
	}

	return &Mem{Cell: c, T: Long}
}
