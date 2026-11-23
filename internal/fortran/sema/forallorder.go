package sema

// How a FORALL runs (Fortran 95's 7.5.4.4; forall.go): finishForall judges
// the outermost FORALL, statement or construct, once it is whole.
//
// Every statement that isn't itself a FORALL is a step, done for every tuple
// of the indexes of the FORALLs around it (its levels, outermost first) whose
// masks hold, wholly before the next step begins. A step is done tuple by
// tuple in turn, a nest of loops with each level's mask tested after its
// loops, when no tuple's statement can change what another tuple's reads;
// otherwise, for an assignment, its masks and values are computed first into
// temporaries over the index set (Forall.Temporaries), as for the statement.
//
// What a FORALL's header computes is computed once, as the FORALL is reached.
// Lowering keeps the outermost bounds then, and a nested FORALL's when it is
// reached. A nested FORALL's bounds are computed again for each step and each
// outer tuple, which gives the same values unless its own steps change what
// they read: NOTIMPL. A mask is tested again by each step, which is right
// unless one step changes what it reads before another tests it: then
// (Forall.MaskTemp) it is computed for the whole index set into a LOGICAL
// temporary at the FORALL, and its steps test that. Both temporaries need an
// index set whose bounds don't use an outer index (a rectangle, varying);
// anything else is NOTIMPL.

// forallStep is one statement of a FORALL that isn't itself a FORALL: the
// FORALLs around it, outermost first, and its place in the innermost's Body.
type forallStep struct {
	levels []*Forall
	at     int
}

// stmt is the step's statement.
func (s forallStep) stmt() Stmt { return s.levels[len(s.levels)-1].Body[s.at] }

// under reports whether the step is inside the FORALL f.
func (s forallStep) under(f *Forall) bool {
	for _, l := range s.levels {
		if l == f {
			return true
		}
	}

	return false
}

// forallSteps is every step of the FORALL f, in order, with outer the FORALLs
// around f.
func forallSteps(outer []*Forall, f *Forall) []forallStep {
	levels := append(outer[:len(outer):len(outer)], f)

	var steps []forallStep

	for i, st := range f.Body {
		if inner, ok := st.(*Forall); ok {
			steps = append(steps, forallSteps(levels, inner)...)

			continue
		}

		steps = append(steps, forallStep{levels: levels, at: i})
	}

	return steps
}

// finishForall decides how the outermost FORALL f runs (see the top of this
// file). False after an error or NOTIMPL.
func (a *analyzer) finishForall(f *Forall) bool {
	steps := forallSteps(nil, f)
	ok := true

	for _, st := range steps {
		ok = a.forallSetsIndex(st) && ok
	}

	if !ok {
		return false
	}

	ok = a.forallHeaderRules(f, nil, steps)

	for _, st := range steps {
		ok = ok && a.forallStepOrder(st)
	}

	return ok
}

// forallSetsIndex reports a step that gives a value to an index of a FORALL
// around it, which only the FORALL may (FORALLSET). False if it does.
func (a *analyzer) forallSetsIndex(st forallStep) bool {
	for _, t := range stepTargets(st.stmt()) {
		target := definedSymbol(t.expr)

		for _, l := range st.levels {
			for _, index := range l.Indexes {
				if target == index.Sym {
					a.report("FORALLSET", t.expr.Position(), target.Name)

					return false
				}
			}
		}
	}

	return true
}

// forallHeaderRules checks what the FORALL f's header computes against its
// steps (the top of this file), f inside the FORALLs outer, and sets
// MaskTemp; then f's nested FORALLs. False after NOTIMPL.
func (a *analyzer) forallHeaderRules(f *Forall, outer []*Forall, steps []forallStep) bool {
	levels := append(outer[:len(outer):len(outer)], f)

	var mine []forallStep

	for _, st := range steps {
		if st.under(f) {
			mine = append(mine, st)
		}
	}

	ok := true

	if len(outer) > 0 {
		for _, x := range f.Indexes {
			for _, e := range []Expr{x.Lower, x.Upper, x.Stride} {
				if e != nil && stepsWrite(mine, e) {
					a.notImplForm(e.Position(), "a nested FORALL's bound that its own statements may change")

					return false
				}
			}
		}
	}

	if f.Mask != nil && len(mine) > 1 && stepsWrite(mine, f.Mask) {
		if forallVaries(levels) {
			a.notImplForm(f.Mask.Position(), "a FORALL mask its statements may change, inside a FORALL whose bounds use an outer index")

			return false
		}

		f.MaskTemp = true
	}

	for _, st := range f.Body {
		if inner, nested := st.(*Forall); nested {
			ok = a.forallHeaderRules(inner, levels, steps) && ok
		}
	}

	return ok
}

// stepsWrite reports whether a step of steps may write storage e reads.
func stepsWrite(steps []forallStep, e Expr) bool {
	for _, st := range steps {
		for _, t := range stepTargets(st.stmt()) {
			if t.sym == nil || arrayReads(e, t.sym) {
				return true
			}
		}
	}

	return false
}

// forallVaries reports whether the index set of the FORALLs levels, outermost
// first, isn't a rectangle: whether a bound or stride of one uses an index of
// another around it.
func forallVaries(levels []*Forall) bool {
	outer := map[*Symbol]bool{}

	for _, l := range levels {
		for _, x := range l.Indexes {
			for _, e := range []Expr{x.Lower, x.Upper, x.Stride} {
				if e == nil {
					continue
				}

				for _, s := range readSymbols(e) {
					if outer[s] {
						return true
					}
				}
			}
		}

		for _, x := range l.Indexes {
			outer[x.Sym] = true
		}
	}

	return false
}

// stepMasks are the masks a step tests itself: those of the FORALLs inside
// the innermost whose mask is in a temporary (whose temporary already holds
// the outer ones).
func stepMasks(levels []*Forall) []Expr {
	var masks []Expr

	for j := len(levels) - 1; j >= 0 && !levels[j].MaskTemp; j-- {
		if levels[j].Mask != nil {
			masks = append(masks, levels[j].Mask)
		}
	}

	return masks
}

// forallStepOrder decides whether the step st is done tuple by tuple in turn,
// or, for an assignment, through temporaries (Forall.Temporaries). False after
// NOTIMPL, for the forms neither way takes yet.
func (a *analyzer) forallStepOrder(st forallStep) bool {
	f := st.levels[len(st.levels)-1]
	masks := stepMasks(st.levels)

	// Each target, and what the step reads besides; in turn is right when
	// nothing the step reads, nor any other target, is a target's storage
	// anywhere but at that target's own reference.
	targets := stepTargets(st.stmt())
	reads := append(stepReads(st.stmt()), masks...)

	for _, t := range targets {
		reads = append(reads, t.expr)
	}

	inTurn := true

	for _, t := range targets {
		if t.sym == nil {
			a.notImplForm(t.expr.Position(), "a FORALL whose pointer's storage isn't known")

			return false
		}

		if forallSubscriptsRead(t.expr, t.sym) {
			a.notImplForm(t.expr.Position(), "a FORALL target whose subscripts read its own storage")

			return false
		}

		same := DumpExpr(t.expr)

		for _, e := range reads {
			if readsElsewhere(e, t.sym, same) {
				inTurn = false
			}
		}
	}

	if inTurn {
		return true
	}

	s, assign := st.stmt().(*Assign)

	switch {
	case !assign:
		a.notImplForm(st.stmt().Source().Position(), "a FORALL's "+forallStepWhat(st.stmt())+" whose values overlap what it assigns")

		return false

	case rankOf(s.Target) > 0:
		a.notImplForm(s.Target.Position(), "a FORALL whose array target its value overlaps")

		return false

	case s.Target.Type().Kind == Character && s.Target.Type().Size == 0:
		a.notImplForm(s.Target.Position(), "a FORALL whose target of run-time length its value overlaps")

		return false

	case forallVaries(st.levels):
		a.notImplForm(s.Target.Position(), "a FORALL whose target its value overlaps, inside a FORALL whose bounds use an outer index")

		return false
	}

	f.Temporaries[st.at] = true

	return true
}

// forallStepWhat names a step's statement for NOTIMPL.
func forallStepWhat(s Stmt) string {
	switch s.(type) {
	case *Call:
		return "defined assignment"
	case *PointerAssign, *Nullify:
		return "pointer assignment"
	case *Where:
		return "WHERE"
	}

	return "assignment"
}

// forallTarget is what a FORALL's step gives a value or an association to,
// and the symbol whose storage that changes: a pointer's own storage for a
// pointer assignment (pointerOwner), not its target's.
type forallTarget struct {
	expr Expr
	sym  *Symbol
}

// stepTargets are what a FORALL's step gives values or associations to: an
// assignment's target, a defined assignment's first argument, a pointer, or
// each of a WHERE's.
func stepTargets(s Stmt) []forallTarget {
	switch s := s.(type) {
	case *Assign:
		return []forallTarget{{s.Target, forallTargetSymbol(s.Target)}}

	case *Call:
		if len(s.Args) > 0 && s.Args[0].Expr != nil {
			return []forallTarget{{s.Args[0].Expr, forallTargetSymbol(s.Args[0].Expr)}}
		}

	case *PointerAssign:
		return []forallTarget{{s.Pointer, pointerOwner(s.Pointer)}}

	case *Nullify:
		var targets []forallTarget

		for _, p := range s.Pointers {
			targets = append(targets, forallTarget{p, pointerOwner(p)})
		}

		return targets

	case *Where:
		var targets []forallTarget

		for _, arm := range s.arms() {
			for _, st := range arm.Body {
				targets = append(targets, stepTargets(st)...)
			}
		}

		return targets
	}

	return nil
}

// stepReads are what a FORALL's step reads, its targets' subscripts aside: an
// assignment's value, a defined assignment's other arguments, a pointer's
// target, or a WHERE's mask and its assignments' values.
func stepReads(s Stmt) []Expr {
	switch s := s.(type) {
	case *Assign:
		return []Expr{s.Value}

	case *Call:
		var reads []Expr

		for _, arg := range s.Args[min(1, len(s.Args)):] {
			if arg.Expr != nil {
				reads = append(reads, arg.Expr)
			}
		}

		return reads

	case *PointerAssign:
		return []Expr{s.Target}

	case *Where:
		reads := []Expr{s.Mask}

		for _, m := range s.Masked {
			reads = append(reads, m.Mask)
		}

		for _, arm := range s.arms() {
			for _, st := range arm.Body {
				reads = append(reads, stepReads(st)...)
			}
		}

		return reads
	}

	return nil
}

// enclosingIndex reports whether s is an index of a FORALL construct open
// around the statement being analyzed.
func (a *analyzer) enclosingIndex(s *Symbol) bool {
	for _, c := range a.open {
		if c.forall == nil {
			continue
		}

		for _, x := range c.forall.Indexes {
			if x.Sym == s {
				return true
			}
		}
	}

	return false
}

// ForallVaries reports whether the index set of the FORALLs levels, outermost
// first, isn't a rectangle: whether a bound or stride of one uses an index of
// another around it. Lowering computes such a FORALL's bounds in its loops.
func ForallVaries(levels []*Forall) bool { return forallVaries(levels) }
