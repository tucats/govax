package ir

import (
	"github.com/tucats/govax/internal/fortran/sema"
)

// Actual arguments: the longwords of a call's argument list (sections 5.1 and
// 6.1.2.1, Table 6-1). A number goes by reference and a CHARACTER value by
// descriptor unless %VAL, %REF, or %DESCR says otherwise. A procedure's name is
// the address of its entry mask, what CALLS starts at (the calling standard, 1.4),
// or for a CHARACTER function a descriptor holding that address and the length
// the passing unit declares, from which a CHARACTER*(*) dummy procedure takes its
// length.

// args lowers a call's actual arguments.
func (l *lowerer) args(list []sema.Arg) []Arg {
	args := make([]Arg, len(list))

	for i, a := range list {
		args[i] = l.arg(a)
	}

	return args
}

// arg lowers an actual argument. A variable, an array element, or a whole array
// is passed itself, so the routine can change it; a constant is passed in a
// datum, read-only, so a routine that assigns to its dummy gets an access
// violation, not a changed constant; any other expression, and a variable or
// element in parentheses, is computed into a temporary and that is passed. A null
// argument is the longword 0.
func (l *lowerer) arg(a sema.Arg) Arg {
	if a.Expr == nil {
		return Arg{Value: LongConst(0)}
	}

	if p, ok := a.Expr.(*sema.Procedure); ok {
		return l.procedureArg(p, a.Mode)
	}

	char := a.Expr.Type().Kind == sema.Character

	switch {
	case a.Mode == sema.ByValue:
		return Arg{Value: l.byValue(a.Expr)}

	case a.Mode == sema.ByDescriptor && !char:
		return l.numberDescriptor(a)

	case a.Mode == sema.ByRef && char:
		return l.textAddress(a)

	case char:
		return l.charArg(a)
	}

	switch e := a.Expr.(type) {
	case *sema.Var, *sema.Elem:
		if a.Copy {
			return Arg{Ref: l.value(l.expr(e))}
		}

		return Arg{Ref: l.mem(e)}

	case *sema.ArrayRef:
		return Arg{Ref: l.symbolMem(e.Sym)}

	case *sema.Const:
		return Arg{Ref: l.constant(l.constOf(e))}
	}

	return Arg{Ref: l.value(l.expr(a.Expr))}
}

// byValue is %VAL(e): e's value as a longword, a shorter integer or logical
// value sign-extended (Table 6-1's footnote says to use ZEXT for zeros).
func (l *lowerer) byValue(e sema.Expr) Expr {
	x := l.expr(e)

	switch x.Type() {
	case Byte, Word:
		return convertTo(Long, x)
	}

	return x
}

// numberDescriptor is %DESCR of a number or an array of numbers: what an output
// list's item is. An assumed-size array, whose size isn't known, is described by
// its first element, as a CHARACTER one is (charArg).
func (l *lowerer) numberDescriptor(a sema.Arg) Arg {
	t := a.Expr.Type()
	d := &Descriptor{DType: l.dtypeOf(t), Class: ClassS, Length: t.Size}

	if a.Copy {
		return Arg{Ref: l.describe(d, l.value(l.expr(a.Expr)))}
	}

	if r, ok := a.Expr.(*sema.ArrayRef); ok && r.Sym.AssumedSize() {
		return Arg{Ref: l.describe(d, l.symbolMem(r.Sym))}
	}

	return Arg{Ref: l.item(a.Expr)}
}

// textAddress is a CHARACTER value by reference: %REF(S), or a character or
// Hollerith constant given to a numeric dummy argument (section 6.1.1.4). The
// routine gets the address of the text and no length.
func (l *lowerer) textAddress(a sema.Arg) Arg {
	if r, ok := a.Expr.(*sema.ArrayRef); ok {
		return Arg{Ref: l.symbolMem(r.Sym)}
	}

	t := l.chars(a.Expr)
	if a.Copy {
		t = l.copyText(a.Expr, t)
	}

	return Arg{Ref: t.Mem}
}

// procedureArg is a procedure's name as an actual argument. A dummy procedure's
// longword is passed on as it came, unless the argument's form changes: a
// CHARACTER one's %REF is the address its descriptor holds, and any other's
// %DESCR is a new descriptor of the address it holds.
func (l *lowerer) procedureArg(p *sema.Procedure, mode sema.ArgMode) Arg {
	char := p.T.Kind == sema.Character
	descriptor := mode == sema.ByDescriptor || (char && mode != sema.ByRef)

	d := &Descriptor{DType: DTypeZEM, Class: ClassS}
	if char {
		d.Length = p.T.Size
	}

	if p.Dummy == nil {
		if !descriptor {
			return Arg{Routine: p.Routine}
		}

		d.Routine = p.Routine

		return Arg{Ref: l.descriptor(d)}
	}

	param := l.params[p.Dummy]

	switch {
	case char == descriptor:
		return Arg{Value: &ArgPointer{param}}

	case char:
		return Arg{Value: &Load{descriptorField(param, PointerOffset, Long)}}
	}

	c := l.descriptorCell(d)
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: PointerOffset, T: Long}, &ArgPointer{param}})

	return Arg{Ref: &Mem{Cell: c, T: Long}}
}

// loc is %LOC(X), the address of what X names (section 6.1.2.2). A dummy
// argument's is its caller's, or 0 for a null argument.
func (l *lowerer) loc(e *sema.Loc) Expr {
	switch x := e.X.(type) {
	case *sema.Procedure:
		arg := l.procedureArg(x, sema.ByRef)
		if arg.Routine != "" {
			return &RoutineAddr{arg.Routine}
		}

		return arg.Value

	case *sema.ArrayRef:
		return &Addr{l.symbolMem(x.Sym)}
	}

	if e.X.Type().Kind == sema.Character {
		return &Addr{l.chars(e.X).Mem}
	}

	return &Addr{l.mem(e.X)}
}

// callee is what a call of the routine s goes to: its name, or for a dummy
// procedure the place it is. The argument list's longword points at the entry
// mask, so the routine is @F(AP), and a CHARACTER one's descriptor holds that
// address, so it is what a CHARACTER dummy's text would be.
func (l *lowerer) callee(s *sema.Symbol) (string, *Mem) {
	p := l.params[s]
	if p == nil {
		return s.Name, nil
	}

	if s.Type.Kind == sema.Character {
		return "", dummyText(p)
	}

	return "", &Mem{Param: p, T: Byte}
}

// callFunction calls the function f refers to, its value going to result:
// from R0 after the call, or, for a value that goes through a hidden first
// argument, where the argument says (the address of result, or for a
// CHARACTER value a descriptor of it).
func (l *lowerer) callFunction(f *sema.FuncRef, result *Mem) {
	args := l.args(f.Args)

	switch {
	case f.T.Kind == sema.Character:
		d := &Descriptor{DType: DTypeT, Class: ClassS}
		place := Text{result, l.charLength(f.Fn)}
		args = append([]Arg{{Ref: l.describeText(d, place)}}, args...)
		result = nil

	case f.T.HiddenResult():
		args = append([]Arg{{Ref: result}}, args...)
		result = nil
	}

	call := &Call{origin: l.origin(), Args: args, Result: result}
	call.Routine, call.Through = l.callee(f.Fn)
	l.emit(call)
}
