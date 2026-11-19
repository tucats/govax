package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Allocatable arrays (the Fortran 90 standard's 5.1.2.4.3, 5.2.6, and 6.3;
// docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, findings 3 and 4).
//
//	REAL, ALLOCATABLE :: A(:), B(:, :)
//	...
//	ALLOCATE (A(N), B(0:M, 3))
//	...
//	DEALLOCATE (A, B)
//
// An allocatable array has a rank and nothing else until ALLOCATE gives it
// bounds and storage, which DEALLOCATE gives back; ALLOCATED(A) says which it is
// now. Its declaration is a deferred shape, a colon for each dimension, which
// only an allocatable array may have (a dummy's colons are an assumed shape,
// shaped.go, and a pointer's are Phase 107's).
//
// What it is in storage is its descriptor, class NCA (the OpenVMS Calling
// Standard's 5.7), in the variable's own place: Symbol.Size is the descriptor's
// size, and lowering reaches every element through it. ALLOCATE fills it in and
// DEALLOCATE clears its POINTER, both by calls of FTNRTL.
//
// The constraints are the standard's (5.1, 5.2.6) and the HP manual's, which
// gather them: an allocatable array is never a dummy argument nor a function's
// value (Fortran 2003 allows both), and never in COMMON, EQUIVALENCE, DATA, or a
// NAMELIST group, nor given an initial value. Its elements may be of any type, a
// derived type's or a STRUCTURE's records among them.

// Allocate is ALLOCATE, or DEALLOCATE if Deallocate is set (the standard's 6.3.1
// and 6.3.3): each of Objects in turn. Stat is STAT='s INTEGER variable or array
// element, or nil: with it, a failure is a number stored there, not an error
// that ends the program, and the statement stops at the object that failed,
// leaving the rest as they were (ours, unconfirmed: the standard leaves the
// others' status undefined); success stores 0.
type Allocate struct {
	stmtBase
	Deallocate bool
	Objects    []*Allocation
	Stat       Expr
}

// Allocation is one object of ALLOCATE or DEALLOCATE: the allocatable array Sym,
// and for ALLOCATE the bounds it is to have, each dimension's Lower and Upper an
// INTEGER expression (Lower the constant 1 where the statement leaves it out).
type Allocation struct {
	Sym          *Symbol
	Lower, Upper []Expr
}

// Allocated is ALLOCATED(ARRAY), whether the allocatable array Sym is allocated
// now (the standard's 13.13.9). Its type is the default LOGICAL.
type Allocated struct {
	exprBase
	Sym *Symbol
}

// checkShapes settles each array with a deferred shape, once every declaration,
// ALLOCATABLE's among them, is read: an allocatable array must have one, all
// colons with no lower bounds (ALLOCSHAPE, DEFERREDMIX), and nothing else may
// (DEFERREDNOTALLOC, SHAPENOTDUMMY), but a dummy's, whose colons are an assumed
// shape.
func (a *analyzer) checkShapes() {
	for _, s := range a.unit.Symbols {
		if s.broken {
			continue
		}

		deferred, lowered := 0, false

		for _, d := range s.Dims {
			if d.Deferred {
				deferred++
				lowered = lowered || d.lowerGiven
			}
		}

		switch {
		case deferred == 0:
			if s.Allocatable {
				a.report("ALLOCSHAPE", s.Pos, s.Name)

				s.broken = true
			}

		case deferred != len(s.Dims):
			a.report("DEFERREDMIX", s.DimPos, s.Name)

			s.broken = true

		case s.Allocatable && lowered:
			a.report("ALLOCSHAPE", s.DimPos, s.Name)

			s.broken = true

		case s.Allocatable:

		case s.Dummy != 0:
			// An assumed shape (shaped.go).
			for i := range s.Dims {
				s.Dims[i].Deferred, s.Dims[i].AssumedShape = false, true
			}

		case lowered:
			a.report("SHAPENOTDUMMY", s.DimPos, s.Name)

			s.broken = true

		default:
			a.report("DEFERREDNOTALLOC", s.DimPos, s.Name)

			s.broken = true
		}
	}
}

// checkAllocatables reports an allocatable array that is a dummy argument or a
// function's value (ALLOCDUMMY), or in COMMON or EQUIVALENCE (ALLOCATTR), once
// the declarations have said so. DATA's and NAMELIST's are reported where those
// statements are read (dataName, checkNamelists), and an initial value in a
// type statement here too.
func (a *analyzer) checkAllocatables() {
	for _, s := range a.unit.Symbols {
		if !s.Allocatable || s.broken {
			continue
		}

		switch {
		case s.Dummy != 0:
			a.report("ALLOCDUMMY", s.Pos, s.Name, "a dummy argument")
		case s.Result:
			a.report("ALLOCDUMMY", s.Pos, s.Name, "a function's value")
		case s.Common != nil && s.Common.Module != "":
			// A module's variable: its section holds its descriptor.
			continue
		case s.Common != nil:
			a.report("ALLOCATTR", s.Pos, s.Name, "be in a common block")
		case s.Equivalenced:
			a.report("ALLOCATTR", s.equivPos, s.Name, "be in an EQUIVALENCE list")
		default:
			continue
		}

		s.broken = true
	}
}

// allocateStatement analyzes ALLOCATE or DEALLOCATE (6.3.1, 6.3.3). Each object
// must be an allocatable array (NOTALLOCATABLE), named once (ALLOCOBJECT); an
// ALLOCATE object has a bound pair for each of its dimensions, each an INTEGER
// value (ALLOCBOUNDS), which may not use any object of the same statement
// (ALLOCSELF: 6.3.1's constraint, read here as any mention, the value, bounds, or
// status all being forbidden). STAT= names an INTEGER variable or array element
// (BADSTAT) that uses no object of the statement either (STATSELF).
func (a *analyzer) allocateStatement(s *ast.AllocateStmt) Stmt {
	st := &Allocate{stmtBase: stmtBase{s}, Deallocate: s.Kind == lex.Deallocate}
	named := map[string]bool{}
	ok := true

	for _, d := range s.Objects {
		if named[d.Name] {
			a.report("ALLOCOBJECT", d.Pos, d.Name, s.Kind.String())

			ok = false
		}

		named[d.Name] = true
	}

	for _, d := range s.Objects {
		sym := a.lookup(d.Name, d.Pos)
		a.note(&sym.refs, d.Pos, RefAssigned)

		if !sym.Allocatable || sym.broken {
			if !sym.broken {
				a.report("NOTALLOCATABLE", d.Pos, d.Name)
			}

			ok = false

			continue
		}

		obj := &Allocation{Sym: sym}
		st.Objects = append(st.Objects, obj)

		if st.Deallocate {
			continue
		}

		if !a.allocationBounds(obj, d, named) {
			ok = false
		}
	}

	if s.Stat != nil {
		st.Stat = a.statVariable(s.Stat, named)
		ok = ok && st.Stat != nil
	}

	if !ok {
		return nil
	}

	return st
}

// statVariable analyzes STAT='s variable x, which may not mention a name of
// named: nil after a message if it isn't an INTEGER variable or array element.
func (a *analyzer) statVariable(x ast.Expr, named map[string]bool) Expr {
	if n := mentionOf(x, named); n != nil {
		a.report("STATSELF", n.Pos, n.Name)

		return nil
	}

	t := a.place(x, false)

	switch {
	case t == nil:
		a.report("BADSTAT", x.Position())

		return nil
	case t.Type().Kind == Invalid:
		return nil
	case t.Type().Kind != Integer:
		a.report("BADSTAT", x.Position())

		return nil
	}

	if v, isVar := t.(*Var); isVar {
		a.checkLoopVariable(v.Sym, v.Pos)
	}

	a.noteAssigned(t)

	return t
}

// freedAtReturn marks each allocatable array that its unit's every return
// deallocates (Symbol.FreedAtReturn): one of a subprogram's own that isn't
// saved, by SAVE (with its name, or with none), or by STATIC. A main program's
// last as long as the program does. A host's array that an internal procedure
// uses is the host's symbol, so the host frees it, not the internal procedure
// (the standard's 6.3.3.1).
func (a *analyzer) freedAtReturn() {
	if a.unit.Kind == ast.MainProgram {
		return
	}

	for _, s := range a.unit.Symbols {
		if !s.Allocatable || s.broken || s.saved || a.saveAll || s.Common != nil ||
			(s.storageStmt != nil && s.storageStmt.Kind == lex.Static) {
			continue
		}

		s.FreedAtReturn = true
	}
}

// allocationBounds analyzes the bounds ALLOCATE's object d gives obj.Sym, none
// of which may mention a name of named.
func (a *analyzer) allocationBounds(obj *Allocation, d *ast.Declarator, named map[string]bool) bool {
	sym := obj.Sym

	if len(d.Dims) != len(sym.Dims) {
		a.report("ALLOCBOUNDS", d.Pos, sym.Name, count(len(sym.Dims), "bound"))

		return false
	}

	value := func(x ast.Expr) Expr {
		if n := mentionOf(x, named); n != nil {
			a.report("ALLOCSELF", n.Pos, n.Name)

			return nil
		}

		v := a.convert(a.arithmetic(a.value(x)), IntegerType)
		if v.Type().Kind == Invalid {
			return nil
		}

		return v
	}

	ok := true

	for _, b := range d.Dims {
		if b.Star || b.Colon {
			a.report("ALLOCBOUNDS", d.Pos, sym.Name, count(len(sym.Dims), "bound"))

			return false
		}

		lower := Expr(&Const{exprBase: exprBase{T: IntegerType, Pos: d.Pos}, Int: 1})
		if b.Lower != nil {
			lower = value(b.Lower)
		}

		upper := value(b.Upper)
		if lower == nil || upper == nil {
			ok = false

			continue
		}

		obj.Lower = append(obj.Lower, lower)
		obj.Upper = append(obj.Upper, upper)
	}

	return ok
}

// mentionOf is the first name of named that the expression x mentions, or nil.
func mentionOf(x ast.Expr, named map[string]bool) *ast.Name {
	switch x := x.(type) {
	case *ast.Name:
		if named[x.Name] {
			return x
		}

	case *ast.Apply:
		if named[x.Name] {
			return &ast.Name{ExprBase: x.ExprBase, Name: x.Name}
		}

		for _, arg := range x.Args {
			if n := mentionOf(arg, named); n != nil {
				return n
			}
		}

	case *ast.Paren:
		return mentionOf(x.X, named)

	case *ast.Unary:
		return mentionOf(x.X, named)

	case *ast.Binary:
		if n := mentionOf(x.X, named); n != nil {
			return n
		}

		return mentionOf(x.Y, named)

	case *ast.KeywordArg:
		return mentionOf(x.X, named)

	case *ast.Substring:
		for _, y := range []ast.Expr{x.X, x.Lower, x.Upper} {
			if y != nil {
				if n := mentionOf(y, named); n != nil {
					return n
				}
			}
		}

	case *ast.Triplet:
		for _, y := range []ast.Expr{x.Lower, x.Upper, x.Stride} {
			if y != nil {
				if n := mentionOf(y, named); n != nil {
					return n
				}
			}
		}
	}

	return nil
}

// allocated analyzes ALLOCATED(ARRAY) (13.13.9): ARRAY, or ARRAY= with it, the
// name of an allocatable array (NOTALLOCATABLE otherwise).
func (a *analyzer) allocated(e *ast.Apply) Expr {
	switch {
	case len(e.Args) != 1:
		a.report("INTRINARGS", e.Pos, "ALLOCATED", "1 argument", len(e.Args))

		return a.bad(e.Pos)

	case e.Args[0] == nil:
		a.report("NULLINTRINARG", e.Pos, "ALLOCATED")

		return a.bad(e.Pos)
	}

	arg := e.Args[0]
	if k, ok := arg.(*ast.KeywordArg); ok {
		if k.Name != "ARRAY" {
			a.report("KEYWORDNAME", k.Pos, "the intrinsic function ALLOCATED", k.Name, "ARRAY")

			return a.bad(e.Pos)
		}

		arg = k.X
	}

	n, ok := arg.(*ast.Name)
	if !ok {
		a.report("NOTALLOCATABLE", arg.Position(), ast.DumpExpr(arg))

		return a.bad(e.Pos)
	}

	s := a.lookup(n.Name, n.Pos)
	if !s.Allocatable || s.broken {
		if !s.broken {
			a.report("NOTALLOCATABLE", n.Pos, n.Name)
		}

		return a.bad(e.Pos)
	}

	return &Allocated{exprBase{T: LogicalType, Pos: e.Pos}, s}
}
