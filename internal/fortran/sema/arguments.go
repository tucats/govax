package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Actual arguments (sections 5.1 and 6.1): what each one in a CALL or a
// function reference is, and how it goes to the routine. By default a number
// goes by reference and a CHARACTER value by descriptor; %VAL, %REF, and %DESCR
// choose otherwise, for routines not written in FORTRAN, as far as Table 6-1
// lets them. A procedure's name is the procedure, and a Hollerith constant its
// bytes.

// args analyzes a list of actual arguments.
func (a *analyzer) args(list []ast.Expr) []Arg {
	args := make([]Arg, len(list))

	for i, e := range list {
		args[i] = a.arg(e)
	}

	return args
}

// arg analyzes an actual argument: a variable, an array element, or a whole
// array is passed as itself; a parenthesized one, or any other expression, as a
// copy.
func (a *analyzer) arg(e ast.Expr) Arg {
	switch x := e.(type) {
	case nil:
		// A null argument: CALL S(A,,B).
		return Arg{}

	case *ast.Name:
		return Arg{Expr: a.nameArg(x)}

	case *ast.Paren:
		v := a.value(x)

		switch v.(type) {
		case *Var, *Elem, *Substring, *StmtFuncRef:
			return Arg{Expr: v, Copy: true}
		}

		return Arg{Expr: v}

	case *ast.HollerithConst:
		c := &Const{exprBase: exprBase{Type{Character, len(x.Value)}, x.Pos}, Text: x.Value}

		return Arg{Expr: c, Mode: ByRef, Hollerith: true}

	case *ast.BuiltinArg:
		if x.Func != "LOC" {
			return a.passedBy(x)
		}
	}

	v := a.value(e)

	// A statement function may be only its dummy argument's name, the actual
	// variable itself; what is passed is its value.
	_, stmtFunc := v.(*StmtFuncRef)

	return Arg{Expr: v, Copy: stmtFunc}
}

// nameArg analyzes a name standing alone as an actual argument: a procedure
// (section 6.1.1, an EXTERNAL or INTRINSIC name, or a dummy procedure passed
// on), or what name makes it.
func (a *analyzer) nameArg(n *ast.Name) Expr {
	s := a.unit.byName[n.Name]

	switch {
	case s == nil || s.broken:

	case s.Class == Intrinsic:
		return a.procedure(n)

	case s.DummyProcedure():
		return &Procedure{exprBase: exprBase{s.Type, n.Pos}, Name: s.Name, Dummy: s}

	case s.External:
		return &Procedure{exprBase: exprBase{s.Type, n.Pos}, Name: s.Name, Routine: s.Name}
	}

	return a.name(n, true)
}

// DummyProcedure reports whether the name is a dummy argument that is a
// procedure, by an EXTERNAL statement or by being called.
func (s *Symbol) DummyProcedure() bool {
	return s.Dummy != 0 && (s.External || s.Class == Function || s.Class == Subroutine)
}

// builtinModes are the argument list built-in functions.
var builtinModes = map[string]ArgMode{"VAL": ByValue, "REF": ByRef, "DESCR": ByDescriptor}

// passedBy analyzes %VAL(X), %REF(X), or %DESCR(X) as an actual argument.
func (a *analyzer) passedBy(b *ast.BuiltinArg) Arg {
	mode := builtinModes[b.Func]

	x, ok := a.builtinOperand(b)
	if !ok {
		return Arg{Expr: a.bad(b.Pos)}
	}

	if _, bad := x.Expr.(*bad); bad {
		return x
	}

	if what := notAllowed(mode, x); what != "" {
		a.report("BUILTINARG", b.Pos, b.Func, what)

		return Arg{Expr: a.bad(b.Pos)}
	}

	x.Mode = mode

	return x
}

// builtinOperand analyzes what a built-in function is given, an actual argument
// itself. ok is false if that is %VAL, %REF, or %DESCR in its turn, already
// reported, which say how an argument goes and have no value of their own.
func (a *analyzer) builtinOperand(b *ast.BuiltinArg) (x Arg, ok bool) {
	if inner, nested := b.X.(*ast.BuiltinArg); nested && inner.Func != "LOC" {
		a.report("BUILTINPLACE", inner.Pos, inner.Func)

		return Arg{}, false
	}

	return a.arg(b.X), true
}

// notAllowed is Table 6-1's answer for passing x by mode: "" if the table says
// Yes, and otherwise what x is, for the message. Every mode takes a logical or
// integer value, a REAL*4, or any other value by reference or by descriptor, but
// %VAL takes nothing wider than a longword, nothing CHARACTER, and no array or
// procedure, and nothing takes a Hollerith constant.
func notAllowed(mode ArgMode, x Arg) string {
	t := x.Expr.Type()

	switch {
	case x.Hollerith:
		return "a Hollerith constant"
	case mode != ByValue:
		return ""
	}

	switch x.Expr.(type) {
	case *ArrayRef:
		return "an array"
	case *Procedure:
		return "a procedure"
	}

	switch t.Kind {
	case Logical, Integer:
	case Real:
		if t.Size != 4 {
			return article(t.String()) + " value"
		}
	default:
		return article(t.String()) + " value"
	}

	return ""
}

// builtinValue analyzes a built-in function used for a value, where only %LOC
// can be.
func (a *analyzer) builtinValue(b *ast.BuiltinArg) Expr {
	if b.Func != "LOC" {
		a.report("BUILTINPLACE", b.Pos, b.Func)

		return a.bad(b.Pos)
	}

	x, ok := a.builtinOperand(b)
	if !ok {
		return a.bad(b.Pos)
	}

	switch x.Expr.(type) {
	case *Var, *Elem, *ArrayRef, *Substring, *Procedure:
		if !x.Copy {
			return &Loc{exprBase{IntegerType, b.Pos}, x.Expr}
		}
	case *bad:
		return x.Expr
	}

	a.report("LOCARG", b.Pos)

	return a.bad(b.Pos)
}

// constantArguments checks a reference's character and Hollerith constants
// against the dummy arguments of the file's routine it calls (section 6.1.1.4). A
// Hollerith constant may go only to a number. A character constant may go to
// either: to a number it goes as a Hollerith constant would. VAX FORTRAN's linker
// fixes that up wherever the routine is, which govax's can't, so without the
// routine in the file (or when it is a dummy procedure, where section 6.1.1.4
// allows only a CHARACTER dummy) it goes by descriptor.
func constantArguments(r routineRef, fu *fileUnit, messages *diag.List) {
	for i := range r.args {
		arg := &r.args[i]

		c, ok := arg.Expr.(*Const)
		p := fu.params()[i]

		if !ok || c.T.Kind != Character || p == nil {
			continue
		}

		switch {
		case arg.Hollerith && p.Type.Kind == Character:
			messages.Report("HOLLCHAR", c.Pos, r.sym.Name, p.Name, fu.pos.Line)
		case arg.Mode == ByDefault && p.Type.Kind != Character:
			arg.Mode = ByRef
		}
	}
}
