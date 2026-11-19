package sema

import (
	"strconv"

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
// Scalar pointers (subtask 2), array pointers (subtask 3), ALLOCATE and
// DEALLOCATE of pointers (subtask 4), and scalar pointer components (subtask
// 5a: a FieldRef whose last field is one stands for the pointer here), array
// pointer components (5b), pointer dummies (6a: pointerArg), and pointer
// results (6b: pointerresult.go) are compiled.
//
// An array pointer, REAL, POINTER :: P(:, :), has a deferred shape: its shape,
// bounds, and elements are its target's. P => A(1:9:2, :) makes P describe
// those elements as a section does (the section's bounds start at 1), and P =>
// A the whole array with A's own bounds (5.1.2.4.3).

// PointerAssign is pointer assignment, Pointer => Target (7.5.2): Pointer a
// pointer (PointerSymbol), Target a variable with TARGET, a part of one, or a
// pointer, whose association Pointer takes.
type PointerAssign struct {
	stmtBase
	Pointer Expr
	Target  Expr
}

// Nullify is NULLIFY: each of Pointers, each a pointer (PointerSymbol),
// disassociated.
type Nullify struct {
	stmtBase
	Pointers []Expr
}

// Associated is ASSOCIATED(POINTER [, TARGET]) (13.13.13), whether Pointer, a
// pointer (PointerSymbol), is associated: with anything, if Target is nil; with
// Target's storage if Target is a variable; or with the same target as Target if
// that is a pointer, and false if either is disassociated. Its type is the
// default LOGICAL.
type Associated struct {
	exprBase
	Pointer Expr
	Target  Expr
}

// IsPointer reports whether e is a reference to a pointer itself: a Var of a
// scalar pointer's symbol, an ArrayRef of an array pointer's, or a FieldRef
// whose last field is a pointer component (finding 7). Lowering reaches such
// a reference's value through the pointer, but in pointer assignment and
// ASSOCIATED, where it is the pointer.
func IsPointer(e Expr) bool {
	if f, ok := e.(*FieldRef); ok {
		return f.IsPointer()
	}

	return PointerSymbol(e) != nil
}

// pointerName names the pointer e (IsPointer) for a message: P, or HEAD%NEXT.
func pointerName(e Expr) string {
	switch f := e.(type) {
	case *FieldRef:
		return f.Text()
	case *FuncRef:
		return f.Fn.Name
	}

	return PointerSymbol(e).Name
}

// PointerSymbol is the pointer e refers to as a whole, if it is a named
// variable's (a pointer component's FieldRef has none), or nil.
func PointerSymbol(e Expr) *Symbol {
	switch e := e.(type) {
	case *Var:
		if e.Sym.Pointer {
			return e.Sym
		}
	case *ArrayRef:
		if e.Sym.Pointer {
			return e.Sym
		}
	}

	return nil
}

// checkPointers reports what a pointer may not be, once the declarations have
// said what each name is: in EQUIVALENCE (POINTERATTR; 5.5.1); and what this
// compiler doesn't compile yet (NOTIMPL): a CHARACTER pointer whose length
// isn't constant but a dummy's CHARACTER(*).
// DATA's, NAMELIST's, and an initial value's are reported where those are read.
func (a *analyzer) checkPointers() {
	for _, s := range a.unit.Symbols {
		if !s.Pointer && s.Struct != nil && s.Struct.HasPointers() && s.Equivalenced && !s.broken {
			// 5.5.1: nor an object with a pointer in it.
			a.report("POINTERPART", s.equivPos, s.Name, s.Struct.Name, "be in an EQUIVALENCE list")

			s.broken = true
		}

		if !s.Pointer || s.broken {
			continue
		}

		switch {
		case s.Equivalenced:
			a.report("POINTERATTR", s.equivPos, s.Name, "be in an EQUIVALENCE list")
		case s.Type.Kind == Character && s.Type.Size == 0 && !(s.Dummy != 0 && s.PassedLength()):
			// A pointer dummy's CHARACTER(*) is its actual's length, which the
			// actual's descriptor holds (finding 8); any other length known only
			// when the program runs isn't compiled.
			a.notImplForm(s.Pos, "a CHARACTER pointer whose length isn't constant")
		default:
			continue
		}

		s.broken = true
	}
}

// pointerObject analyzes x where a pointer itself must stand: in pointer
// assignment, NULLIFY, and ASSOCIATED. It is a Var of a scalar pointer or an
// ArrayRef of an array pointer (PointerSymbol), or nil after a message:
// NOTPOINTER for anything else. assigned says the statement gives the
// pointer an association, for the cross reference.
func (a *analyzer) pointerObject(x ast.Expr, assigned bool) Expr {
	return a.pointerObjectOr(x, assigned, func(pos diag.Position, text string) {
		a.report("NOTPOINTER", pos, text)
	})
}

// pointerObjectOr is pointerObject with refuse saying what isn't a pointer,
// at pos, named text, in place of NOTPOINTER.
func (a *analyzer) pointerObjectOr(x ast.Expr, assigned bool, refuse func(pos diag.Position, text string)) Expr {
	switch x := x.(type) {
	case *ast.Name:
		s := a.lookup(x.Name, x.Pos)
		if s.broken {
			return nil
		}

		if !s.Pointer {
			refuse(x.Pos, x.Name)

			return nil
		}

		if assigned {
			a.note(&s.refs, x.Pos, RefAssigned)
		}

		if s.Dims != nil {
			return NewArrayRef(s, x.Pos)
		}

		return &Var{exprBase{T: s.Type, Pos: x.Pos}, s}

	case *ast.FieldRef:
		t := a.field(x, false)

		// An array pointer component, whose symbol stands for it.
		if r, isArray := t.(*ArrayRef); isArray && r.Sym.Component != nil {
			if assigned {
				a.noteAssigned(r.Sym.Component)
				a.checkIntentIn(pointerHome(r), x.Pos)
			}

			return r
		}

		f, ok := t.(*FieldRef)

		switch {
		case !ok && t.Type().Kind == Invalid:
			// An error, reported.
			return nil
		case !ok:
			// An element or a section of an array pointer component.
			refuse(x.Pos, ast.DumpExpr(x))

			return nil
		case !f.IsPointer():
			refuse(x.Pos, f.Text())

			return nil
		}

		if assigned {
			a.noteAssigned(f)
			a.checkIntentIn(pointerHome(f), x.Pos)
		}

		return f
	}

	refuse(x.Position(), ast.DumpExpr(x))

	return nil
}

// pointerArg analyzes x, argument n (from 0) of a reference to the procedure
// proc, as the actual argument of the pointer dummy p (12.4.1.1): a pointer of
// p's type, kind, rank, and (where both are constants) length, passed as itself
// (Arg.Pointer), so that what the procedure does to p it does to the actual
// (the author's answer 5). Anything else is POINTERARG. An OPTIONAL dummy's
// argument left out is passed as the longword 0, as any other's.
func (a *analyzer) pointerArg(x ast.Expr, n int, proc, p *Symbol) Arg {
	if x == nil {
		return Arg{}
	}

	refuse := func(pos diag.Position, _ string) {
		a.report("POINTERARG", pos, n+1, proc.Name, p.Name, pointedText(p.Type, len(p.Dims)))
	}

	// nil after a message: POINTERARG, or the reference's own error.
	ptr := a.pointerObjectOr(x, true, refuse)
	if ptr == nil {
		return Arg{Expr: a.bad(x.Position())}
	}

	pt, rank := ptr.Type(), targetRank(ptr)

	same := sameTypes(p.Type, pt)
	if p.Type.Kind == Character && pt.Kind == Character {
		same = p.Type.Size == pt.Size || p.Type.Size == 0 || pt.Size == 0
	}

	if !same || rank != len(p.Dims) {
		refuse(x.Position(), "")

		return Arg{Expr: a.bad(x.Position())}
	}

	return Arg{Expr: ptr, Pointer: true}
}

// pointerTarget analyzes x as a target for the pointer ptr (7.5.2's
// constraints): a variable with TARGET or a part of one (an element, a
// section, a substring, a component), a pointer, or a part of a pointer's
// target; of the pointer's type, kind, rank, and, when both are constants,
// length; or a reference to a function whose value is a pointer
// (pointerresult.go). nil after a message: POINTERFUNC for another function's
// reference,
// NOTTARGET for anything that isn't such a variable, VECTORTARGET for a section
// with a vector subscript, POINTERTYPE for one of another type.
func (a *analyzer) pointerTarget(ptr Expr, x ast.Expr) Expr {
	return a.targetFor(pointerName(ptr), ptr.Type(), targetRank(ptr), x)
}

// targetFor analyzes x as a target for a pointer named name (for messages) to
// values of type t and rank rank: pointer assignment's, ASSOCIATED's, or a
// structure constructor's pointer component's (pointerTarget's rules).
func (a *analyzer) targetFor(name string, pt Type, rank int, x ast.Expr) Expr {
	if a.functionReference(x) {
		if f, ok := a.pointerFunctionTarget(name, pt, rank, x); ok {
			return f
		}

		a.report("POINTERFUNC", x.Position(), ast.DumpExpr(x))

		return nil
	}

	t := a.targetPlace(x)

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

	if vectorSubscripted(t) {
		a.report("VECTORTARGET", x.Position(), ast.DumpExpr(x))

		return nil
	}

	if !a.pointerMatches(name, pt, rank, t, x.Position()) {
		return nil
	}

	return t
}

// targetPlace analyzes x as a variable that may be a target: a name (an
// array's as the whole array, its bounds its own), an element or a section, a
// substring, or a component (a whole array component as an array of its own
// bounds); nil for what is no variable, a constant or a procedure.
func (a *analyzer) targetPlace(x ast.Expr) Expr {
	switch e := a.substringForm(x).(type) {
	case *ast.Name:
		if s := a.scope.find(e.Name); s != nil && (s.Class == Constant || s.External || s.Class == Intrinsic ||
			s.Class == Function || s.Class == Subroutine || s.Class == StmtFunction || s.Class == UnitName) {
			return nil
		}

		switch t := a.name(e, true).(type) {
		case *ArrayRef:
			return a.wholeArray(t.Sym, e)
		case *Var:
			return t
		case *Const:
			return nil
		default:
			// An error, reported.
			return t
		}

	case *ast.Apply:
		s := a.scope.find(e.Name)
		if s == nil || s.Class != Array {
			return nil
		}

		return a.element(s, e)

	case *ast.Substring:
		return a.substring(e)

	case *ast.FieldRef:
		if a.constantComponent(e) {
			return a.bad(e.Pos)
		}

		t := a.field(e, true)
		if f, ok := t.(*FieldRef); ok && f.WholeArray() {
			return f.wholeRanked()
		}

		return t
	}

	return nil
}

// vectorSubscripted reports whether the place t is a section with a vector
// subscript, or a substring or component of one.
func vectorSubscripted(t Expr) bool {
	var subs []SectionSub

	switch t := t.(type) {
	case *Section:
		subs = t.Subs
	case *FieldRef:
		subs = t.Sections
	case *Substring:
		return vectorSubscripted(t.X)
	}

	for _, sub := range subs {
		if sub.Vector != nil {
			return true
		}
	}

	return false
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
	case s.Class == UnitName:
		// A recursive function's reference to itself, its value a RESULT
		// variable.
		_, isApply := x.(*ast.Apply)

		return isApply
	case s.Class == Unclassified:
		_, isApply := x.(*ast.Apply)

		return isApply && s.Dims == nil
	}

	return false
}

// TargetBase is the variable whose storage the place t is part of (targetBase):
// for lowering, which must know whether a pointer's target reads the pointer.
func TargetBase(t Expr) *Symbol {
	return targetBase(t)
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
// the target of the pointer name, which points at values of type pt and rank
// rank, by type, kind, rank, and length (7.5.2: the same type, type
// parameters, and rank). A length is compared only when both are constants; a
// substring's whose bounds aren't is the program's to get right.
func (a *analyzer) pointerMatches(name string, pt Type, rank int, t Expr, pos diag.Position) bool {
	tt := t.Type()

	same := sameTypes(pt, tt)

	if pt.Kind == Character && tt.Kind == Character {
		same = pt.Size == tt.Size || pt.Size == 0 || tt.Size == 0
	}

	if same && targetRank(t) == rank {
		return true
	}

	a.report("POINTERTYPE", pos, name, pointedText(pt, rank), pointedText(tt, targetRank(t)))

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
// "an array of REAL*4", "an array of rank 2 of REAL*4".
func pointedText(t Type, rank int) string {
	text := t.String()
	if t.Kind == Record && t.Struct != nil {
		text = t.Struct.Display()
	}

	switch {
	case rank == 1:
		return "an array of " + text
	case rank > 1:
		return "an array of rank " + strconv.Itoa(rank) + " of " + text
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

	var ptr Expr
	if a.functionReference(args[0]) {
		ptr = a.pointerFunctionValue(args[0])
	} else {
		ptr = a.pointerObject(args[0], false)
	}

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
