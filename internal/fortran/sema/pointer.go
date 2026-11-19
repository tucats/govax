package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Pointers (the Fortran 90 standard's 5.1.2.7, 5.1.2.8, 6.3.2, 7.5.2, and
// 13.13.13; docs/planning/PHASE-107 - Fortran 90 pointers.md).
//
//	INTEGER, TARGET :: N, M
//	INTEGER, POINTER :: P
//	P => N        ! P is now another name for N
//	P = 5         ! so N is 5
//	P => M        ! and now for M
//	NULLIFY (P)   ! and now for nothing: ASSOCIATED (P) is false
//
// A pointer is a variable whose storage holds not its value but where its value
// is: its target's address, 0 while it points at nothing (disassociated). A
// CHARACTER pointer's storage is a class S descriptor, the target's length and
// address, as a CHARACTER dummy argument's is (Symbol.Size, PointerSize).
//
// So most of a pointer is in what a reference to it means. Wherever a value or a
// variable is wanted (an expression, the left side of =, an I/O list, an actual
// argument whose dummy isn't a pointer) a pointer stands for its target (7.1.4.1):
// semantic analysis keeps the reference a Var of the pointer's symbol, and
// lowering, seeing Symbol.Pointer, reaches the value through the address. Only
// pointer assignment, NULLIFY, ASSOCIATED, and later ALLOCATE and a pointer dummy
// see the pointer itself, and those are this file's: each takes its pointer as a
// pointerObject and makes a node of its own.
//
// A pointer may point only at a variable with the TARGET attribute, a part of
// one, or another pointer's target (7.5.2). That rule is what lets the optimizer
// assume a store through a pointer leaves every other variable alone (5.1.2.8's
// purpose).
//
// Scalar pointers are compiled (subtask 2); array pointers, pointer dummies and
// results, pointer components, and ALLOCATE of a pointer are NOTIMPL until their
// subtasks.

// PointerAssign is pointer assignment, Pointer => Target (7.5.2): Pointer a Var
// of a pointer, Target a variable with TARGET, a part of one, or a pointer (a
// Var of one), whose association Pointer takes.
type PointerAssign struct {
	stmtBase
	Pointer Expr
	Target  Expr
}

// Nullify is NULLIFY: each of Pointers, each a Var of a pointer, disassociated.
type Nullify struct {
	stmtBase
	Pointers []Expr
}

// Associated is ASSOCIATED(POINTER [, TARGET]) (13.13.13), whether Pointer, a
// Var of a pointer, is associated: with anything, if Target is nil; with
// Target's storage if Target is a variable; or with the same target as Target if
// that is a pointer, and false if either is disassociated. Its type is the
// default LOGICAL.
type Associated struct {
	exprBase
	Pointer Expr
	Target  Expr
}

// IsPointer reports whether e is a reference to a pointer itself: a Var of a
// pointer's symbol. Lowering reaches such a reference's value through the
// pointer, but in pointer assignment and ASSOCIATED, where it is the pointer.
func IsPointer(e Expr) bool {
	v, ok := e.(*Var)

	return ok && v.Sym.Pointer
}

// checkPointers reports what a pointer may not be, once the declarations have
// said what each name is: in EQUIVALENCE (POINTERATTR; 5.5.1); and what this
// compiler doesn't compile yet (NOTIMPL): an array pointer, a pointer dummy
// argument or function result, a CHARACTER pointer whose length isn't constant.
// DATA's, NAMELIST's, and an initial value's are reported where those are read.
func (a *analyzer) checkPointers() {
	for _, s := range a.unit.Symbols {
		if !s.Pointer || s.broken {
			continue
		}

		switch {
		case s.Equivalenced:
			a.report("POINTERATTR", s.equivPos, s.Name, "be in an EQUIVALENCE list")
		case s.Dims != nil:
			a.notImplForm(s.DimPos, "an array pointer")
		case s.Dummy != 0:
			a.notImplForm(s.Pos, "a pointer dummy argument")
		case s.Result:
			a.notImplForm(s.Pos, "a function whose result is a pointer")
		case s.Type.Kind == Character && s.Type.Size == 0:
			a.notImplForm(s.Pos, "a CHARACTER pointer whose length isn't constant")
		default:
			continue
		}

		s.broken = true
	}
}

// pointerObject analyzes x where a pointer itself must stand: in pointer
// assignment, NULLIFY, and ASSOCIATED. It is a Var of a pointer, or nil after a
// message: NOTPOINTER for anything else. assigned says the statement gives the
// pointer an association, for the cross reference.
func (a *analyzer) pointerObject(x ast.Expr, assigned bool) Expr {
	switch x := x.(type) {
	case *ast.Name:
		s := a.lookup(x.Name, x.Pos)
		if s.broken {
			return nil
		}

		if !s.Pointer {
			a.report("NOTPOINTER", x.Pos, x.Name)

			return nil
		}

		if assigned {
			a.note(&s.refs, x.Pos, RefAssigned)
		}

		return &Var{exprBase{T: s.Type, Pos: x.Pos}, s}

	case *ast.FieldRef:
		a.notImplForm(x.Pos, "a pointer component")

		return nil
	}

	a.report("NOTPOINTER", x.Position(), ast.DumpExpr(x))

	return nil
}

// pointerTarget analyzes x as a target for the pointer ptr (7.5.2's
// constraints): a variable with TARGET or a part of one (an element, a
// substring, a component), a pointer, or a part of a pointer's target; of the
// pointer's type, kind, rank, and, when both are constants, length. nil after a
// message: POINTERFUNC for a function's reference, NOTTARGET for anything that
// isn't such a variable, POINTERTYPE for one of another type.
func (a *analyzer) pointerTarget(ptr Expr, x ast.Expr) Expr {
	if a.functionReference(x) {
		a.report("POINTERFUNC", x.Position(), ast.DumpExpr(x))

		return nil
	}

	t := a.place(x, true)

	switch {
	case t == nil:
		a.report("NOTTARGET", x.Position(), targetText(x))

		return nil
	case t.Type().Kind == Invalid:
		return nil
	}

	base := targetBase(t)
	if base == nil || !base.Pointer && !base.Target {
		a.report("NOTTARGET", x.Position(), ast.DumpExpr(x))

		return nil
	}

	if !a.pointerMatches(ptr, t, x.Position()) {
		return nil
	}

	return t
}

// targetText names x for NOTTARGET: a reference or a constant as written,
// anything else as "the expression", which the message's pointer shows.
func targetText(x ast.Expr) string {
	switch x.(type) {
	case *ast.Name, *ast.Apply, *ast.Substring, *ast.FieldRef, *ast.IntConst, *ast.RealConst, *ast.CharConst, *ast.LogicalConst:
		return ast.DumpExpr(x)
	}

	return "the expression"
}

// functionReference reports whether x is a reference to a function, F or
// F(X), rather than a variable: a name the unit knows as a function, an
// external, or an intrinsic, or not as a variable or array at all, applied to
// arguments.
func (a *analyzer) functionReference(x ast.Expr) bool {
	var name string

	switch x := x.(type) {
	case *ast.Name:
		name = x.Name
	case *ast.Apply:
		name = x.Name
	default:
		return false
	}

	s := a.scope.find(name)

	switch {
	case s == nil:
		_, isApply := x.(*ast.Apply)

		return isApply
	case s.Class == Function, s.Class == Intrinsic, s.External, s.Class == StmtFunction:
		return true
	case s.Class == Unclassified:
		_, isApply := x.(*ast.Apply)

		return isApply && s.Dims == nil
	}

	return false
}

// targetBase is the variable whose storage the place t is part of: the name of
// a Var, an element, a substring, or a component reference.
func targetBase(t Expr) *Symbol {
	switch t := t.(type) {
	case *Var:
		return t.Sym
	case *Elem:
		return t.Sym
	case *Substring:
		return targetBase(t.X)
	}

	return baseSymbol(t)
}

// pointerMatches reports, after POINTERTYPE if not, whether the place t may be
// the pointer ptr's target by type, kind, rank, and length (7.5.2: the same
// type, type parameters, and rank). A length is compared only when both are
// constants; a substring's whose bounds aren't is the program's to get right.
func (a *analyzer) pointerMatches(ptr, t Expr, pos diag.Position) bool {
	pt, tt := ptr.Type(), t.Type()

	same := sameTypes(pt, tt)

	if pt.Kind == Character && tt.Kind == Character {
		same = pt.Size == tt.Size || pt.Size == 0 || tt.Size == 0
	}

	if same && targetRank(t) == targetRank(ptr) {
		return true
	}

	a.report("POINTERTYPE", pos, ptr.(*Var).Sym.Name, pointedText(pt, targetRank(ptr)), pointedText(tt, targetRank(t)))

	return false
}

// targetRank is the place t's rank: a whole array's (an ArrayRef has no shape
// of its own until an array's value is wanted), or a section's.
func targetRank(t Expr) int {
	if r, ok := t.(*ArrayRef); ok {
		return len(r.Sym.Dims)
	}

	return rankOf(t)
}

// pointedText describes a target's type and rank for POINTERTYPE: "INTEGER*4",
// "an array of REAL*4".
func pointedText(t Type, rank int) string {
	text := t.String()
	if t.Kind == Record && t.Struct != nil {
		text = t.Struct.Display()
	}

	if rank > 0 {
		return "an array of " + text
	}

	return text
}

// pointerAssignment analyzes pointer => target (7.5.2).
func (a *analyzer) pointerAssignment(s *ast.PointerAssignStmt) Stmt {
	ptr := a.pointerObject(s.Pointer, true)
	if ptr == nil {
		return nil
	}

	t := a.pointerTarget(ptr, s.Target)
	if t == nil {
		return nil
	}

	return &PointerAssign{stmtBase: stmtBase{s}, Pointer: ptr, Target: t}
}

// nullify analyzes NULLIFY (6.3.2). A pointer may be named twice: the second
// does nothing more.
func (a *analyzer) nullify(s *ast.NullifyStmt) Stmt {
	st := &Nullify{stmtBase: stmtBase{s}}
	ok := true

	for _, x := range s.Objects {
		p := a.pointerObject(x, true)
		if p == nil {
			ok = false

			continue
		}

		st.Pointers = append(st.Pointers, p)
	}

	if !ok {
		return nil
	}

	return st
}

// associatedRef analyzes ASSOCIATED(POINTER [, TARGET]) (13.13.13): POINTER a
// pointer, TARGET a pointer or a variable that may be POINTER's target
// (pointerTarget's rules), each positional or by its keyword.
func (a *analyzer) associatedRef(e *ast.Apply) Expr {
	var args [2]ast.Expr

	for i, arg := range e.Args {
		switch {
		case arg == nil:
			a.report("NULLINTRINARG", e.Pos, "ASSOCIATED")

			return a.bad(e.Pos)

		case i > 1:
			a.report("INTRINARGS", e.Pos, "ASSOCIATED", "1 or 2 arguments", len(e.Args))

			return a.bad(e.Pos)
		}

		k, isKeyword := arg.(*ast.KeywordArg)

		switch {
		case !isKeyword && args[i] == nil:
			args[i] = arg
		case isKeyword && k.Name == "POINTER" && args[0] == nil:
			args[0] = k.X
		case isKeyword && k.Name == "TARGET" && args[1] == nil:
			args[1] = k.X
		default:
			name := ""
			if isKeyword {
				name = k.Name
			}

			a.report("KEYWORDNAME", arg.Position(), "the intrinsic function ASSOCIATED", name, "POINTER or TARGET")

			return a.bad(e.Pos)
		}
	}

	if args[0] == nil {
		a.report("INTRINARGS", e.Pos, "ASSOCIATED", "1 or 2 arguments", len(e.Args))

		return a.bad(e.Pos)
	}

	ptr := a.pointerObject(args[0], false)
	if ptr == nil {
		return a.bad(e.Pos)
	}

	r := &Associated{exprBase: exprBase{T: LogicalType, Pos: e.Pos}, Pointer: ptr}

	if args[1] != nil {
		if r.Target = a.pointerTarget(ptr, args[1]); r.Target == nil {
			return a.bad(e.Pos)
		}
	}

	return r
}
