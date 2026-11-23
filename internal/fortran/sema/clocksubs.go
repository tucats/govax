package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Fortran 90's intrinsic subroutines about the clock and random numbers
// (the standard's 13.13.27, 13.13.83, 13.13.84, and 13.13.105;
// docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md, findings 9 to
// 11): DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER, and RANDOM_SEED. Each is a
// call of one of FTNRTL's routines (ftn_system.mar), chosen by its arguments'
// kinds as the system subroutines' are (system.go), with an argument left out
// passed as the longword 0, which the routine takes for absent:
//
//	CALL DATE_AND_TIME(DATE, TIME, ZONE, VALUES)
//	    FTN$DATE_AND_TIME(date, time, zone, values), the CHARACTER ones by
//	    descriptor and VALUES by address; FTN$DATE_AND_TIME_W for an
//	    INTEGER*2 VALUES
//	CALL SYSTEM_CLOCK(COUNT, COUNT_RATE, COUNT_MAX)
//	    FTN$SYSTEM_CLOCK(count, rate, max), or FTN$SYSTEM_CLOCK_W
//	CALL RANDOM_NUMBER(HARVEST)
//	    FTN$RANDOM_F, _D, _G, or _H(harvest), by HARVEST's format; for an
//	    array a call for each element, in array element order (as MVBITS's)
//	CALL RANDOM_SEED(SIZE, PUT, GET)
//	    FTN$RANDOM_SEED(size, put, get), or FTN$RANDOM_SEED_W
//	CALL CPU_TIME(TIME)
//	    FTN$CPU_TIME_F, _D, _G, or _H(time), by TIME's format (Fortran 95's
//	    13.14.33; docs/planning/PHASE-112 - Fortran 95.md, the author's answer 3)
//
// The standard says each integer argument is of the default kind; govax
// takes INTEGER*2 or INTEGER*4 under either /I4 setting, all of one length
// (SYSLEN), the routine chosen by it. The CHARACTER arguments take what fits,
// as an assignment would. An array argument is passed as its elements one
// after another (sequence.go): a section that isn't is copied, and for VALUES
// and GET copied back.

// randomRoutines are RANDOM_NUMBER's routines, by HARVEST's format.
var randomRoutines = map[vaxfloat.Format]string{
	vaxfloat.F: "FTN$RANDOM_F",
	vaxfloat.D: "FTN$RANDOM_D",
	vaxfloat.G: "FTN$RANDOM_G",
	vaxfloat.H: "FTN$RANDOM_H",
}

// cpuTimeRoutines are CPU_TIME's routines, by TIME's format.
var cpuTimeRoutines = map[vaxfloat.Format]string{
	vaxfloat.F: "FTN$CPU_TIME_F",
	vaxfloat.D: "FTN$CPU_TIME_D",
	vaxfloat.G: "FTN$CPU_TIME_G",
	vaxfloat.H: "FTN$CPU_TIME_H",
}

// dateValues is how many values DATE_AND_TIME's VALUES takes at least.
const dateValues = 8

// integerSlot analyzes an integer argument a routine stores in or reads, of
// INTEGER*2 or INTEGER*4 of the length size holds (or sets, if 0): a scalar
// variable or element if out, else any scalar expression. ok is false after a
// message; a nil e is an absent argument, a nil Arg.Expr.
func (a *analyzer) integerSlot(s *ast.CallStmt, e ast.Expr, out bool, size *int) (Arg, bool) {
	if e == nil {
		return Arg{}, true
	}

	arg := a.arg(e)
	if arg.Expr == nil {
		return arg, false
	}

	t := arg.Expr.Type()

	what := "an INTEGER*2 or INTEGER*4 expression"
	if out {
		what = "an INTEGER*2 or INTEGER*4 variable or array element"
	}

	switch {
	case t.Kind == Invalid:
		return arg, false

	case !isSystemInteger(t) || arg.Mode != ByDefault || rankOf(arg.Expr) != 0 || (out && !isPlace(arg)):
		a.report("SYSARG", arg.Expr.Position(), s.Name, what)

		return arg, false

	case *size != 0 && t.Size != *size:
		a.report("SYSLEN", arg.Expr.Position(), s.Name)

		return arg, false
	}

	*size = t.Size

	if out {
		a.noteAssigned(arg.Expr)
	}

	a.notePassed(arg)

	return arg, true
}

// integerArray analyzes an INTEGER*2 or INTEGER*4 array argument of rank 1
// and at least least elements, if that is known when compiling, of the length
// size holds (or sets). An out one (VALUES, GET) is a whole array or a section
// the routine stores in, copied back if it was copied; an in one (PUT) is any
// array value.
func (a *analyzer) integerArray(s *ast.CallStmt, e ast.Expr, out bool, least int, size *int) (Arg, bool) {
	if e == nil {
		return Arg{}, true
	}

	in := &Symbol{Intent: "IN"}
	if out {
		in = nil
	}

	arg := a.sequenced(a.argValue(e), in)
	if arg.Expr == nil {
		return arg, false
	}

	t := arg.Expr.Type()
	what := "an INTEGER*2 or INTEGER*4 array of rank 1"
	shape := argumentShape(arg.Expr)

	switch {
	case t.Kind == Invalid:
		return arg, false

	case !isSystemInteger(t) || len(shape) != 1 || arg.Mode != ByDefault || (out && !Described(arg)):
		a.report("SYSARG", arg.Expr.Position(), s.Name, what)

		return arg, false

	case *size != 0 && t.Size != *size:
		a.report("SYSLEN", arg.Expr.Position(), s.Name)

		return arg, false
	}

	if x := shape[0]; x.Known && x.Size < least {
		a.report("SYSARG", arg.Expr.Position(), s.Name,
			"an INTEGER array of at least "+count(least, "element"))

		return arg, false
	}

	*size = t.Size

	if out {
		a.noteAssigned(arg.Expr)
	}

	a.notePassed(arg)

	return arg, true
}

// argumentShape is an array argument's shape. A whole array passed as an
// actual argument is an ArrayRef with no shape of its own (FORTRAN-77's way of
// naming it, arg's), so its symbol's bounds give it; an assumed-size array's
// extents are only counted, none known.
func argumentShape(e Expr) Shape {
	r, ok := e.(*ArrayRef)
	if !ok || r.S != nil {
		return e.Shape()
	}

	if r.Sym.AssumedSize() {
		return make(Shape, len(r.Sym.Dims))
	}

	return arrayShape(r)
}

// wordForm is a routine's name for the length size: name_W for INTEGER*2,
// name for INTEGER*4 or no integer argument at all.
func wordForm(name string, size int) string {
	if size == 2 {
		return name + "_W"
	}

	return name
}

// dateAndTime analyzes CALL DATE_AND_TIME(DATE, TIME, ZONE, VALUES)
// (13.13.27): each a CHARACTER variable, element, or substring, VALUES an
// integer array of at least eight elements, all optional.
func (a *analyzer) dateAndTime(s *ast.CallStmt, slots []ast.Expr) Stmt {
	args := make([]Arg, 4)
	ok := true

	for i, e := range slots[:3] {
		if e == nil {
			continue
		}

		arg := a.arg(e)
		if arg.Expr == nil {
			ok = false

			continue
		}

		t := arg.Expr.Type()

		_, sub := arg.Expr.(*Substring)

		switch {
		case t.Kind == Invalid:
			ok = false

			continue

		case t.Kind != Character || rankOf(arg.Expr) != 0 || arg.Mode != ByDefault || !(isPlace(arg) || sub && !arg.Copy):
			a.report("SYSARG", arg.Expr.Position(), s.Name,
				"a CHARACTER variable, array element, or substring where it says "+dateAndTimeNames[i])

			ok = false

			continue
		}

		a.noteAssigned(arg.Expr)
		a.notePassed(arg)

		args[i] = arg
	}

	size := 0

	values, good := a.integerArray(s, slots[3], true, dateValues, &size)
	if !good || !ok {
		return nil
	}

	args[3] = values

	return systemCall(s, wordForm("FTN$DATE_AND_TIME", size), args)
}

// dateAndTimeNames are DATE_AND_TIME's character dummies' names.
var dateAndTimeNames = []string{"DATE", "TIME", "ZONE"}

// systemClock analyzes CALL SYSTEM_CLOCK(COUNT, COUNT_RATE, COUNT_MAX)
// (13.13.105): three integer variables or elements, all optional.
func (a *analyzer) systemClock(s *ast.CallStmt, slots []ast.Expr) Stmt {
	args := make([]Arg, 3)
	size := 0

	for i, e := range slots {
		arg, ok := a.integerSlot(s, e, true, &size)
		if !ok {
			return nil
		}

		args[i] = arg
	}

	return systemCall(s, wordForm("FTN$SYSTEM_CLOCK", size), args)
}

// randomNumber analyzes CALL RANDOM_NUMBER(HARVEST) (13.13.83): HARVEST a
// REAL variable, element, array, or section of any kind, each element given
// a number from the uniform distribution over 0 <= x < 1. An array is a call
// for each element in array element order (Call.Shape; ir's elementalCall),
// so one seed gives one sequence however the elements are asked for.
func (a *analyzer) randomNumber(s *ast.CallStmt, slots []ast.Expr) Stmt {
	const what = "a REAL variable, array element, array, or array section where it says HARVEST"

	switch a.substringForm(slots[0]).(type) {
	case *ast.Name, *ast.Apply, *ast.FieldRef:
	default:
		a.report("SYSARG", slots[0].Position(), s.Name, what)

		return nil
	}

	to := a.target(slots[0])
	if to == nil {
		return nil
	}

	t := to.Type()

	switch {
	case t.Kind == Invalid:
		return nil

	case t.Kind != Real:
		a.report("SYSARG", to.Position(), s.Name, what)

		return nil
	}

	a.noteAssigned(to)

	shape, ok := a.conforms(s.NamePos, to)
	if !ok {
		return nil
	}

	c := systemCall(s, randomRoutines[a.format(t)], []Arg{{Expr: to}}).(*Call)
	c.Shape = shape

	return c
}

// randomSeed analyzes CALL RANDOM_SEED(SIZE, PUT, GET) (13.13.84): at most
// one of them, SIZE an integer variable or element, PUT and GET integer arrays
// of at least the seed's size (one longword, or two words for INTEGER*2).
// With none the seed is FTNRTL's first.
func (a *analyzer) randomSeed(s *ast.CallStmt, slots []ast.Expr) Stmt {
	given := 0

	for _, e := range slots {
		if e != nil {
			given++
		}
	}

	if given > 1 {
		a.report("SEEDARGS", s.NamePos)

		return nil
	}

	args := make([]Arg, 3)
	size := 0

	var ok bool

	if args[0], ok = a.integerSlot(s, slots[0], true, &size); !ok {
		return nil
	}

	// The seed's size depends on the arrays' length, known only once the
	// argument is analyzed: an INTEGER*4 array's is 1, an INTEGER*2's 2.
	for i, out := range []bool{false, true} {
		e := slots[i+1]
		if e == nil {
			continue
		}

		arg, good := a.integerArray(s, e, out, 1, &size)
		if !good {
			return nil
		}

		if x := argumentShape(arg.Expr)[0]; size == 2 && x.Known && x.Size < 2 {
			a.report("SYSARG", arg.Expr.Position(), s.Name, "an INTEGER array of at least 2 elements")

			return nil
		}

		args[i+1] = arg
	}

	return systemCall(s, wordForm("FTN$RANDOM_SEED", size), args)
}

// cpuTime analyzes CALL CPU_TIME(TIME) (Fortran 95's 13.14.33): TIME, a REAL
// scalar variable, array element, or component of any kind, is given the
// process's CPU time in seconds (FTNRTL's routine for its format).
func (a *analyzer) cpuTime(s *ast.CallStmt, slots []ast.Expr) Stmt {
	const what = "a REAL scalar variable or array element where it says TIME"

	switch a.substringForm(slots[0]).(type) {
	case *ast.Name, *ast.Apply, *ast.FieldRef:
	default:
		a.report("SYSARG", slots[0].Position(), s.Name, what)

		return nil
	}

	to := a.target(slots[0])
	if to == nil {
		return nil
	}

	t := to.Type()

	switch {
	case t.Kind == Invalid:
		return nil

	case t.Kind != Real || rankOf(to) > 0:
		a.report("SYSARG", to.Position(), s.Name, what)

		return nil
	}

	a.noteAssigned(to)

	return systemCall(s, cpuTimeRoutines[a.format(t)], []Arg{{Expr: to}})
}
