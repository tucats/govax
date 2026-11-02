package sema

import "github.com/tucats/govax/internal/fortran/ast"

// The system subroutines and functions of the 1988 VAX FORTRAN Language
// Reference Manual's D.4 and D.5.3, which FORTRAN's run-time library provides.
// A program calls them as it would its own, but they aren't quite ordinary
// calls: a routine gets only addresses and nothing of what's at them, so the
// compiler chooses among FTNRTL's routines by the arguments it can see, as FORRTL
// has its own for each (FOR$DATE and FOR$DATE_T_DS, FOR$IDATE and FOR$JDATE, ...;
// docs/PHASE-83 - FORTRAN and VMS.md, Part A). A symbol of its own, in no unit's
// table, stands for the chosen routine. As for any routine, a unit's own of the
// name wins, and so does EXTERNAL DATE, a program's way of saying it has its own
// in another file: the callers only come here for a name the unit hasn't
// classified.
//
// What the manual doesn't say is unconfirmed, there being no VAX FORTRAN to ask:
// here an argument the routine would fault on (too few, or null, and none of them
// can be null but ERRSNS's) is an error at compile time, and the types are as the
// manual gives them. A MACRO program calling the routines gets what FORRTL's do.

// systemSubroutines are the subroutines, each analyzing its own CALL.
var systemSubroutines = map[string]func(*analyzer, *ast.CallStmt, []ast.Expr) Stmt{
	"DATE":   (*analyzer).systemBuffer,
	"TIME":   (*analyzer).systemBuffer,
	"IDATE":  (*analyzer).systemIDate,
	"ERRSNS": (*analyzer).errsns,
	"EXIT":   (*analyzer).systemExit,
	"MVBITS": (*analyzer).systemMVBits,
}

// systemFunction is a function that takes one argument and is REAL*4 whatever
// IMPLICIT says of its name: what it calls, and the argument it takes.
type systemFunction struct {
	routine string
	arg     Type
	place   bool // the argument must be a variable or element, which it changes
	what    string
}

// systemFunctions are the functions. RAN is MTH$RANDOM, the same generator
// (D.4.7), and needs nothing of FTNRTL's.
var systemFunctions = map[string]systemFunction{
	"RAN":    {routine: "MTH$RANDOM", arg: IntegerType, place: true, what: "an INTEGER*4 variable or array element"},
	"SECNDS": {routine: "FTN$SECNDS", arg: RealType, what: "a REAL*4 expression"},
}

// systemFunction analyzes y = RAN(i) or y = SECNDS(x).
func (a *analyzer) systemFunction(e *ast.Apply, f systemFunction) Expr {
	a.noteOther(whatSystem, e.Name, e.Pos, RefUsed)

	if len(e.Args) != 1 {
		a.report("SYSCOUNT", e.Pos, e.Name, "1 argument", len(e.Args))

		return a.bad(e.Pos)
	}

	arg := a.arg(e.Args[0])

	if arg.Expr == nil || arg.Expr.Type() != f.arg || arg.Mode != ByDefault || (f.place && !isPlace(arg)) {
		if arg.Expr == nil || arg.Expr.Type().Kind != Invalid {
			a.report("SYSARG", e.Pos, e.Name, f.what)
		}

		return a.bad(e.Pos)
	}

	if f.place {
		a.noteAssigned(arg.Expr)
	}

	sym := &Symbol{Name: f.routine, Class: Function, Type: RealType}

	return &FuncRef{exprBase{RealType, e.Pos}, sym, []Arg{arg}}
}

// isPlace reports whether an argument goes as a variable or an array element
// itself, so that a routine can store there.
func isPlace(arg Arg) bool {
	if arg.Copy {
		return false
	}

	switch e := arg.Expr.(type) {
	case *Var, *Elem:
		return true
	case *FieldRef:
		return !e.WholeArray()
	}

	return false
}

// integerPlace is the type of an argument a routine stores an integer at, a
// variable or element of INTEGER*2 or INTEGER*4 by its default mode; ok is false
// if it's anything else.
func integerPlace(arg Arg) (t Type, ok bool) {
	t = arg.Expr.Type()

	return t, isPlace(arg) && arg.Mode == ByDefault && isSystemInteger(t)
}

// isSystemInteger reports whether t is an integer of a length the system
// subroutines have routines for.
func isSystemInteger(t Type) bool {
	return t.Kind == Integer && (t.Size == 2 || t.Size == 4)
}

// systemCall is a call of FTNRTL's routine, through a symbol of its own.
func systemCall(s *ast.CallStmt, routine string, args []Arg) Stmt {
	sub := &Symbol{Name: routine, Class: Subroutine}

	return &Call{stmtBase: stmtBase{s}, Sub: sub, Args: args}
}

// systemBuffer analyzes CALL DATE(buf) and CALL TIME(buf) (D.4.1, D.4.6). A
// CHARACTER buffer goes by descriptor, to the routine that takes its length; any
// other goes by address, and nothing checks it has the room, 9 bytes or 8: a
// variable of any type, an array, an element, a substring, or a field.
func (a *analyzer) systemBuffer(s *ast.CallStmt, list []ast.Expr) Stmt {
	if len(list) != 1 {
		a.report("SYSCOUNT", s.NamePos, s.Name, "1 argument", len(list))

		return nil
	}

	arg := a.arg(list[0])

	if arg.Expr == nil {
		a.report("SYSARG", s.NamePos, s.Name, "a variable, array, array element, or substring")

		return nil
	}

	a.noteAssigned(arg.Expr)

	place := false

	switch arg.Expr.(type) {
	case *Var, *Elem, *ArrayRef, *Substring, *FieldRef:
		place = !arg.Copy && arg.Mode == ByDefault
	}

	if !place {
		if arg.Expr.Type().Kind != Invalid {
			a.report("SYSARG", arg.Expr.Position(), s.Name, "a variable, array, array element, or substring")
		}

		return nil
	}

	routine := "FTN$" + s.Name
	if arg.Expr.Type().Kind == Character {
		routine += "_T_DS"
	}

	return systemCall(s, routine, []Arg{arg})
}

// systemIDate analyzes CALL IDATE(i,j,k) (D.4.2): three places, all INTEGER*2 or
// all INTEGER*4.
func (a *analyzer) systemIDate(s *ast.CallStmt, list []ast.Expr) Stmt {
	if len(list) != 3 {
		a.report("SYSCOUNT", s.NamePos, s.Name, "3 arguments", len(list))

		return nil
	}

	args := a.args(list)
	size := 0

	for _, arg := range args {
		a.noteAssigned(arg.Expr)

		if !a.systemPlace(s, arg, &size) {
			return nil
		}
	}

	if size == 2 {
		return systemCall(s, "FTN$IDATE", args)
	}

	return systemCall(s, "FTN$JDATE", args)
}

// systemPlace checks one of a subroutine's integer places, which must have the
// length size holds, or set it if it's 0, and reports what's wrong if it doesn't.
func (a *analyzer) systemPlace(s *ast.CallStmt, arg Arg, size *int) bool {
	const what = "an INTEGER*2 or INTEGER*4 variable or array element"

	if arg.Expr == nil {
		a.report("SYSARG", s.NamePos, s.Name, what)

		return false
	}

	t, ok := integerPlace(arg)

	switch {
	case t.Kind == Invalid:
		return false

	case !ok:
		a.report("SYSARG", arg.Expr.Position(), s.Name, what)

		return false

	case *size != 0 && t.Size != *size:
		a.report("SYSLEN", arg.Expr.Position(), s.Name)

		return false
	}

	*size = t.Size

	return true
}

// systemExit analyzes CALL EXIT[(exit-status)] (D.4.4): an integer of either
// length, any expression of one, to the routine for its length.
func (a *analyzer) systemExit(s *ast.CallStmt, list []ast.Expr) Stmt {
	if len(list) > 1 {
		a.report("SYSCOUNT", s.NamePos, s.Name, "at most 1 argument", len(list))

		return nil
	}

	if len(list) == 0 {
		return systemCall(s, "FTN$EXIT", nil)
	}

	arg := a.arg(list[0])

	if arg.Expr == nil {
		a.report("SYSARG", s.NamePos, s.Name, "an INTEGER*2 or INTEGER*4 expression")

		return nil
	}

	t := arg.Expr.Type()

	if !isSystemInteger(t) || arg.Mode != ByDefault {
		if t.Kind != Invalid {
			a.report("SYSARG", arg.Expr.Position(), s.Name, "an INTEGER*2 or INTEGER*4 expression")
		}

		return nil
	}

	if t.Size == 2 {
		return systemCall(s, "FTN$EXIT_W", []Arg{arg})
	}

	return systemCall(s, "FTN$EXIT", []Arg{arg})
}

// systemMVBits analyzes CALL MVBITS(m,i,len,n,j) (D.5.3). m and n are places
// whose length chooses the routine, which reads every argument at that length,
// so i, len, and j, which are expressions, are converted to it (the manual's rule
// that all are of one length, with its allowance for values in range, and a
// constant has whatever length the unit's default gives it).
func (a *analyzer) systemMVBits(s *ast.CallStmt, list []ast.Expr) Stmt {
	if len(list) != 5 {
		a.report("SYSCOUNT", s.NamePos, s.Name, "5 arguments", len(list))

		return nil
	}

	args := a.args(list)
	size := 0

	a.noteAssigned(args[3].Expr)

	if !a.systemPlace(s, args[0], &size) || !a.systemPlace(s, args[3], &size) {
		return nil
	}

	for _, i := range []int{1, 2, 4} {
		e := args[i].Expr

		if e == nil || !isSystemInteger(e.Type()) || args[i].Mode != ByDefault {
			if e == nil {
				a.report("SYSARG", s.NamePos, s.Name, "an integer expression where it says a position or a length")
			} else if e.Type().Kind != Invalid {
				a.report("SYSARG", e.Position(), s.Name, "an integer expression where it says a position or a length")
			}

			return nil
		}

		args[i].Expr = a.convert(e, Type{Integer, size})
	}

	if size == 2 {
		return systemCall(s, "FTN$IMVBITS", args)
	}

	return systemCall(s, "FTN$JMVBITS", args)
}
