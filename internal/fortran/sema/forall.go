package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The FORALL statement (Fortran 95's 7.5.4; the HP Fortran manual's 4.2.5;
// docs/planning/PHASE-112 - Fortran 95.md, subtask 8):
//
//	FORALL (I = 1:N, J = 1:N, A(I,J) /= 0.0) B(I,J) = 1.0 / A(I,J)
//
// assigns B(I,J) for every pair of I and J in the index set whose mask is true.
// It is an array assignment whose elements are named by indexes of its own
// rather than by sections, and it keeps an array assignment's rule (7.5.4.4):
// every index's bounds and stride are computed first, then the mask for the
// whole index set, then every value and every target's subscripts, and only
// then is anything stored. So
//
//	FORALL (I = 2:8) V(I) = V(I-1)
//
// shifts V by one place, where a DO loop running up would copy V(1) into
// every element.
//
// The indexes are the statement's own names (7.5.4.1: "statement entities"),
// as a statement function's dummy arguments are its own: a FORALL is a scope
// inside the unit's (scope.go) where each index is a scalar INTEGER symbol of
// its own, Forall.Indexes, in none of the unit's tables, taking only its type's
// being INTEGER from the entity the unit calls by its name, which it doesn't
// change. Lowering gives each a place (ir/lowerforall.go).
//
// Running the assignment for each tuple of indexes in turn, as a nest of DO
// loops, is right when no assignment can change what another reads: when
// nothing the value, the mask, or the target's subscripts read may share the
// target's storage, or what does is the very element the target names (the
// same text: B(I,J) = B(I,J) * 2), which no other tuple writes, since a FORALL
// may assign an element only once (7.5.4.5). Otherwise (Forall.Temporaries) the
// mask and the values are computed first for every tuple into temporaries of
// the index set's shape, and a second nest stores them. Only procedures
// declared PURE may be referenced in a FORALL (7.5.4.3), so computing the
// value has no effect but its result.

// The FORALL construct (7.5.4.2; subtask 9):
//
//	FORALL (I = 1:N)
//	   FORALL (J = 1:N, J > I) A(I,J) = 10.0*I + J
//	   WHERE (A(I,:) == 0.0) A(I,:) = -1.0
//	END FORALL
//
// is its statements one after another, each done for every tuple before the
// next begins: an assignment, a pointer assignment, a WHERE, or a nested
// FORALL, whose indexes join the construct's (a nested FORALL's bounds may use
// them, its mask too). Every statement that isn't itself a FORALL is a step
// (forallStep), judged as a FORALL statement is, with the indexes and masks of
// every FORALL around it, by finishForall once the outermost FORALL is whole.
// What the header computes is computed once (7.5.4.4): the outermost bounds
// are kept as the construct starts (ir/lowerforall.go), a nested FORALL's
// when it is reached, and a mask that the construct's statements may change is
// computed into a temporary first (Forall.MaskTemp); anything subtler is
// NOTIMPL (finishForall's rules).

// Construct reports whether f is the FORALL construct, not the statement.
func (f *Forall) Construct() bool {
	_, ok := f.AST.(*ast.ForallBlockStmt)

	return ok
}

// forallStatement analyzes the FORALL statement s, or nil after an error.
func (a *analyzer) forallStatement(s *ast.ForallStmt) Stmt {
	f := &Forall{stmtBase: stmtBase{s}}

	// The statement's scope, inside the unit's (or an enclosing FORALL's):
	// its indexes are its own names, and every other name in it is the
	// unit's.
	outer := a.scope
	a.scope = newInnerScope(outer)

	defer func() { a.scope = outer }()

	first := len(a.unit.calls)
	ok := a.forallHeader(s.Header, f)

	var st Stmt

	switch then := s.Then.(type) {
	case *ast.AssignStmt:
		st = a.assignment(then)
	case *ast.PointerAssignStmt:
		st = a.pointerAssignment(then)
	}

	if st == nil || !ok {
		return nil
	}

	f.Body, f.Temporaries = []Stmt{st}, []bool{false}

	if a.inForall() {
		// The outermost FORALL judges it, with the rest of its statements.
		return f
	}

	if !a.forallReferences(first) || !a.finishForall(f) {
		return nil
	}

	return f
}

// forallBlock analyzes FORALL (header) alone, which opens a FORALL construct:
// its scope, holding its indexes, lasts until its END FORALL (pop). The
// construct is kept after an error in the header, with no Forall, so that its
// statements are matched and checked.
func (a *analyzer) forallBlock(s *ast.ForallBlockStmt) {
	f := &Forall{stmtBase: stmtBase{s}}
	c := &construct{forallStart: s, arm: &Arm{AST: s}, outerScope: a.scope, firstCall: len(a.unit.calls)}
	a.scope = newInnerScope(a.scope)

	if a.forallHeader(s.Header, f) {
		c.forall = f
	}

	a.push(c)
}

// endForall analyzes END FORALL, which closes its construct. The construct is
// added to the block around it, judged with its statements if it is the
// outermost FORALL.
func (a *analyzer) endForall(s *ast.EndForallStmt) {
	c := a.match(s, (*construct).isForall, "NOFORALL")
	if c == nil {
		return
	}

	a.checkName(c, s, true)
	a.pop()

	f := c.forall
	if f == nil {
		return
	}

	f.EndForall = s
	f.Body = c.arm.Body
	f.Temporaries = make([]bool, len(f.Body))

	if !a.inForall() && (!a.forallReferences(c.firstCall) || !a.finishForall(f)) {
		return
	}

	a.add(f)
}

// brokenForall keeps a FORALL construct's part in the block structure for a
// statement the parser rejected.
func (a *analyzer) brokenForall(kind lex.Kind, s ast.Stmt) {
	switch kind {
	case lex.ForallBlock:
		a.push(&construct{forallStart: s, arm: &Arm{AST: s}, reported: true})

	case lex.EndForall:
		for c := a.top(); c != nil && !c.isForall(); c = a.top() {
			a.pop()
		}

		if a.top() != nil {
			a.pop()
		}
	}
}

// forallBody reports a statement in a FORALL construct that may not be there,
// FORALLBODY: anything but an assignment, a pointer assignment, a WHERE, a
// FORALL, and END FORALL (R757). It keeps the statement's part in the block
// structure, and reports whether it did.
func (a *analyzer) forallBody(s ast.Stmt) bool {
	c := a.top()
	if c == nil || !c.isForall() {
		return false
	}

	switch s.(type) {
	case *ast.AssignStmt, *ast.PointerAssignStmt, *ast.WhereStmt, *ast.WhereBlockStmt, *ast.ForallStmt,
		*ast.ForallBlockStmt, *ast.EndForallStmt, *ast.BadStmt:
		return false

	case *ast.EndStmt, *ast.ElseWhereStmt, *ast.EndWhereStmt:
		// NOENDFORALL from closeAll; NOWHERE from their own analysis.
		return false
	}

	a.report("FORALLBODY", s.Position(), aStatement(s))
	a.brokenStatement(s.Base().Kind, s)

	return true
}

// inForall reports whether a FORALL construct is open.
func (a *analyzer) inForall() bool {
	for _, c := range a.open {
		if c.isForall() {
			return true
		}
	}

	return false
}

// forallHeader analyzes a FORALL's header into f, in the FORALL's own scope,
// a.scope: its indexes, their bounds and strides, and its mask. False after an
// error.
func (a *analyzer) forallHeader(h ast.ForallHeader, f *Forall) bool {
	ok := true

	for _, x := range h.Indexes {
		index, good := a.forallIndex(x, f)
		ok = ok && good

		f.Indexes = append(f.Indexes, index)
		a.scope.define(index.Sym)
	}

	// The bounds and strides, which may not use an index of this FORALL
	// (7.5.4.1's constraint): read in the statement's scope, so such a use
	// finds the index and is reported, rather than reading the unit's
	// variable of its name. An enclosing FORALL's indexes they may use.
	for i, x := range h.Indexes {
		index := f.Indexes[i]
		index.Lower = a.forallBound(x.Lower, f)
		index.Upper = a.forallBound(x.Upper, f)

		if x.Stride != nil {
			index.Stride = a.forallBound(x.Stride, f)

			if n, isConst := constantInt(index.Stride); isConst && n == 0 {
				a.report("FORALLSTRIDE", x.Stride.Position())

				ok = false
			}
		}

		ok = ok && index.Lower != nil && index.Upper != nil && (x.Stride == nil || index.Stride != nil)
	}

	if h.Mask != nil {
		f.Mask = a.forallMask(h.Mask)
		ok = ok && f.Mask != nil
	}

	return ok
}

// forallIndex is the index x of the FORALL f: a symbol of its own, of type
// INTEGER. The name must be one the unit could give a scalar INTEGER variable
// (7.5.4.1: "a named scalar variable of type integer"): FORALLINDEX if the
// unit's entity of that name is anything else, NOTYPE if it has no type.
// FORALLTWICE for a name given twice. False after an error.
func (a *analyzer) forallIndex(x *ast.ForallIndex, f *Forall) (*ForallIndex, bool) {
	sym := NewSymbol(x.Name, IntegerType)
	sym.Pos = x.Pos
	index := &ForallIndex{Sym: sym}

	// The FORALL gives the index its values, as a DO does its variable.
	a.note(&sym.refs, x.Pos, RefAssigned)
	a.unit.forallIndexes = append(a.unit.forallIndexes, sym)

	for _, other := range f.Indexes {
		if other.Sym.Name == x.Name {
			a.report("FORALLTWICE", x.Pos, x.Name)

			return index, false
		}
	}

	var (
		t       Type
		untyped bool
		what    string
	)

	if s := a.scope.find(x.Name); s != nil {
		if a.enclosingIndex(s) {
			a.report("FORALLOUTER", x.Pos, x.Name)

			return index, false
		}

		s.typesEntity = true

		if s.broken {
			return index, false
		}

		t, untyped = s.Type, s.untyped && !s.Typed

		switch {
		case s.Class == Constant:
			what = "a named constant"
		case len(s.Dims) > 0:
			what = "an array"
		case s.Class != Variable && s.Class != Unclassified:
			what = "not a variable"
		}
	} else {
		t, untyped = a.scope.rules().typeOf(x.Name)

		if a.warnings.Declarations && !untyped {
			a.report("UNDECLARED", x.Pos, x.Name, t.String())
		}
	}

	switch {
	case untyped:
		a.report("NOTYPE", x.Pos, x.Name)

		return index, false

	case what == "" && t.Kind != Integer:
		what = article(t.String())
	}

	if what != "" {
		a.report("FORALLINDEX", x.Pos, x.Name, what)

		return index, false
	}

	return index, true
}

// forallBound is a FORALL index's bound or stride, a scalar INTEGER
// (FORALLINT) converted to the default INTEGER, that uses no index of the
// FORALL f (FORALLBOUND); nil after an error.
func (a *analyzer) forallBound(e ast.Expr, f *Forall) Expr {
	v := a.numericConst(a.value(e))

	switch t := v.Type(); {
	case t.Kind == Invalid:
		return nil

	case t.Kind != Integer:
		a.report("FORALLINT", e.Position(), article(t.String()))

		return nil

	case t != IntegerType:
		v = a.convert(v, IntegerType)
	}

	for _, s := range readSymbols(v) {
		for _, index := range f.Indexes {
			if s == index.Sym {
				a.report("FORALLBOUND", e.Position(), s.Name)

				return nil
			}
		}
	}

	return v
}

// forallMask is a FORALL's mask, a scalar LOGICAL (an INTEGER's bit 0, as an
// IF's condition is, VAX FORTRAN's rule), or nil after an error.
func (a *analyzer) forallMask(e ast.Expr) Expr {
	m := a.numericConst(a.value(e))

	switch t := m.Type(); t.Kind {
	case Invalid:
		return nil
	case Logical:
		return m
	case Integer:
		return a.convert(m, Type{Kind: Logical, Size: t.Size})
	default:
		a.report("FORALLMASK", e.Position(), article(t.String()))
	}

	return nil
}

// forallReferences reports each procedure a FORALL references that isn't
// declared PURE (7.5.4.3; FORALLPURE): those the statement's analysis added to
// the unit's calls after the first. False if there is one.
func (a *analyzer) forallReferences(first int) bool {
	ok := true

	for _, r := range a.unit.calls[first:] {
		if r.sym.broken {
			continue
		}

		if u := a.interfaceOf(r.sym); u == nil || !u.Pure {
			a.report("FORALLPURE", r.pos, r.sym.Name)

			ok = false
		}
	}

	return ok
}

// forallTargetSymbol is the symbol whose storage the FORALL target t writes.
func forallTargetSymbol(t Expr) *Symbol {
	if s := baseSymbol(t); s != nil {
		return s
	}

	return definedSymbol(t)
}

// forallSubscriptsRead reports whether the target's subscripts, or a
// substring's bounds, may read storage ts may share.
func forallSubscriptsRead(target Expr, ts *Symbol) bool {
	switch t := target.(type) {
	case *Elem:
		for _, s := range t.Subs {
			if readsAny(s, ts) {
				return true
			}
		}

	case *Section:
		for _, s := range t.Subs {
			for _, x := range []Expr{s.Index, s.Vector, s.Lower, s.Upper, s.Stride} {
				if x != nil && arrayReads(x, ts) {
					return true
				}
			}
		}

	case *Substring:
		return forallSubscriptsRead(t.X, ts) || readsAny(t.Lower, ts) || t.Upper != nil && readsAny(t.Upper, ts)

	case *FieldRef:
		for _, x := range t.Subs {
			if readsAny(x, ts) {
				return true
			}
		}

		for _, step := range t.Path {
			for _, x := range step.Subs {
				if readsAny(x, ts) {
					return true
				}
			}
		}
	}

	return false
}

// readsElsewhere reports whether e may read storage ts may share anywhere but
// at the reference whose text is same, the target's: arrayReads, passing over
// each part of e that is that very reference.
func readsElsewhere(e Expr, ts *Symbol, same string) bool {
	if e == nil {
		return false
	}

	if DumpExpr(e) == same {
		return false
	}

	switch e := e.(type) {
	case *Unary:
		return readsElsewhere(e.X, ts, same)
	case *Convert:
		return readsElsewhere(e.X, ts, same)
	case *Binary:
		return readsElsewhere(e.X, ts, same) || readsElsewhere(e.Y, ts, same)
	case *Compare:
		return readsElsewhere(e.X, ts, same) || readsElsewhere(e.Y, ts, same)
	case *Substring:
		return readsElsewhere(e.X, ts, same) || readsElsewhere(e.Lower, ts, same) || readsElsewhere(e.Upper, ts, same)
	case *Concat:
		for _, p := range e.Parts {
			if readsElsewhere(p, ts, same) {
				return true
			}
		}

		return false
	case *IntrinsicRef:
		for _, x := range e.Args {
			if readsElsewhere(x, ts, same) {
				return true
			}
		}

		return false
	case *FuncRef:
		// What a function can reach besides its arguments (readsAny's rule).
		if e.Pointer || e.Elemental || ts.Common != nil || ts.Dummy != 0 || ts.group != nil {
			return arrayReads(e, ts)
		}

		for _, x := range e.Args {
			if x.Expr != nil && readsElsewhere(x.Expr, ts, same) {
				return true
			}
		}

		return false
	}

	return arrayReads(e, ts)
}
