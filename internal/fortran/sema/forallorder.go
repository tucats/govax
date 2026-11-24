package sema

// How a FORALL runs (Fortran 95's 7.5.4.4; forall.go): finishForall judges
// the outermost FORALL, statement or construct, once it is whole.
//
// Every statement that isn't itself a FORALL is a step, done for every tuple
// of the indexes of the FORALLs around it (its levels, outermost first) whose
// masks hold, wholly before the next step begins. A step is done tuple by
// tuple in turn, a nest of loops with each level's mask tested after its
// loops, when no tuple's statement can change what another tuple's reads.
// Otherwise its values are computed first, and stored after: for a scalar
// assignment over a rectangle of tuples, into temporaries of the index set's
// shape (Forall.Temporaries), as for the statement; for anything else (an
// array or a WHERE's assignment, a CHARACTER target of run-time length, a
// pointer or defined assignment, a target whose subscripts read its own
// storage or whose storage isn't known, an index set that isn't a rectangle)
// by recording each store, its target's address with its value, for every
// tuple, and then doing them (Forall.Recorded; Phase 113).
//
// What a FORALL's header computes is computed once, as the FORALL is reached.
// Lowering keeps the outermost bounds then, and a nested FORALL's when it is
// reached. A nested FORALL's bounds are computed again for each step and each
// outer tuple, which gives the same values unless its own steps change what
// they read. A mask is tested again by each step, which is right unless one
// step changes what it reads before another tests it: then (Forall.MaskTemp)
// it is computed for the whole index set into a LOGICAL temporary at the
// FORALL, and its steps test that, if the index set is a rectangle. Where
// neither will do (a nested bound its steps change; a mask they change, in an
// index set that isn't a rectangle or inside a listed one), the FORALL's
// tuples are listed as it is reached, its mask tested then (Forall.List), and
// its steps walk the list.

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
// MaskTemp or List; then f's nested FORALLs.
func (a *analyzer) forallHeaderRules(f *Forall, outer []*Forall, steps []forallStep) bool {
	levels := append(outer[:len(outer):len(outer)], f)

	var mine []forallStep

	for _, st := range steps {
		if st.under(f) {
			mine = append(mine, st)
		}
	}

	listed := false
	for _, o := range outer {
		listed = listed || o.List
	}

	if len(outer) > 0 {
		for _, x := range f.Indexes {
			for _, e := range []Expr{x.Lower, x.Upper, x.Stride} {
				if e != nil && stepsWrite(mine, e) {
					f.List = true
				}
			}
		}
	}

	if f.Mask != nil && !f.List && len(mine) > 1 && stepsWrite(mine, f.Mask) {
		if listed || forallVaries(levels) {
			f.List = true
		} else {
			f.MaskTemp = true
		}
	}

	ok := true

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
	var outer []*Symbol

	for _, l := range levels {
		for _, x := range l.Indexes {
			for _, e := range []Expr{x.Lower, x.Upper, x.Stride} {
				if e == nil {
					continue
				}

				// Anywhere in the expression: J = 1:I-1 uses I.
				for _, s := range outer {
					if readsAny(e, s) {
						return true
					}
				}
			}
		}

		for _, x := range l.Indexes {
			outer = append(outer, x.Sym)
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
// through temporaries (Forall.Temporaries), or by recording its stores
// (Forall.Recorded). False after NOTIMPL, for the one form none takes.
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

	inTurn, plain := true, true

	for _, t := range targets {
		// A target whose storage isn't known (through a pointer), or whose
		// subscripts read its own (V(V(I)) = 0): every target's subscripts are
		// computed before any element is stored (7.5.4.4), so its address is.
		if t.sym == nil || forallSubscriptsRead(t.expr, t.sym) {
			inTurn, plain = false, false

			continue
		}

		same := referenceKey(t.expr)

		for _, e := range reads {
			if readsElsewhere(e, t.sym, same) {
				inTurn = false
			}
		}
	}

	// A pointer assignment whose target is a pointer reads that pointer's
	// storage, which for an array pointer component is its record's though the
	// reference names the component's own symbol: V(I+1)%P => V(I)%P.
	if p, ok := st.stmt().(*PointerAssign); ok && IsPointer(p.Target) {
		if home := pointerOwner(p.Target); home != nil {
			for _, t := range targets {
				if t.sym != nil && mayShare(t.sym, home) && referenceKey(t.expr) != referenceKey(p.Target) {
					inTurn = false
				}
			}
		}
	}

	switch {
	case inTurn:
		return true

	case plain && forallTemporaries(st):
		f.Temporaries[st.at] = true

		return true
	}

	if w, ok := st.stmt().(*Where); ok && !whereRecordable(w, masks) {
		a.notImplForm(w.Mask.Position(), "a FORALL's WHERE of more than one assignment, whose assignments change what its masks read and overlap what it assigns,")

		return false
	}

	f.Recorded[st.at] = true

	return true
}

// forallTemporaries reports whether the step st, done through temporaries, can
// have them in the index set's shape: a scalar assignment, not of a CHARACTER
// target of run-time length, in FORALLs whose index set is a rectangle and
// whose tuples aren't listed.
func forallTemporaries(st forallStep) bool {
	s, assign := st.stmt().(*Assign)
	if !assign || rankOf(s.Target) > 0 || forallVaries(st.levels) {
		return false
	}

	if t := s.Target.Type(); t.Kind == Character && t.Size == 0 {
		return false
	}

	for _, l := range st.levels {
		if l.List {
			return false
		}
	}

	return true
}

// whereRecordable reports whether the WHERE w, a FORALL's step whose stores are
// recorded, can be: one recorded pass for each of its assignments in turn, each
// testing the masks again (WhereAssignments), which needs one assignment, or
// masks (its own, and the FORALL's masks the step tests) that none of its
// assignments may change.
func whereRecordable(w *Where, masks []Expr) bool {
	if len(WhereAssignments(w)) <= 1 {
		return true
	}

	masks = append(masks, whereMasks(w)...)

	for _, t := range stepTargets(w) {
		if t.sym == nil {
			return false
		}

		for _, m := range masks {
			if arrayReads(m, t.sym) {
				return false
			}
		}
	}

	return true
}

// whereMasks are every mask of the WHERE w and of the WHEREs nested in it.
func whereMasks(w *Where) []Expr {
	masks := []Expr{w.Mask}

	for _, m := range w.Masked {
		masks = append(masks, m.Mask)
	}

	for _, arm := range w.arms() {
		for _, st := range arm.Body {
			if inner, ok := st.(*Where); ok {
				masks = append(masks, whereMasks(inner)...)
			}
		}
	}

	return masks
}

// WhereAssignments are the WHERE w's assignments (an *Assign, or an elemental
// defined assignment's *Call), those of the WHEREs nested in it among them, in
// the order written: the order they are done in.
func WhereAssignments(w *Where) []Stmt {
	var list []Stmt

	for _, arm := range w.arms() {
		for _, st := range arm.Body {
			if inner, ok := st.(*Where); ok {
				list = append(list, WhereAssignments(inner)...)

				continue
			}

			list = append(list, st)
		}
	}

	return list
}

// WhereOnly is a copy of the WHERE w with only its assignment keep, one of
// WhereAssignments(w): every mask the same, every other assignment gone.
func WhereOnly(w *Where, keep Stmt) *Where {
	only := func(arm *Arm) *Arm {
		if arm == nil {
			return nil
		}

		copied := *arm
		copied.Body = nil

		for _, st := range arm.Body {
			if inner, ok := st.(*Where); ok {
				copied.Body = append(copied.Body, WhereOnly(inner, keep))
			} else if st == keep {
				copied.Body = append(copied.Body, st)
			}
		}

		return &copied
	}

	copied := *w
	copied.Then = only(w.Then)
	copied.Else = only(w.Else)
	copied.Masked = nil

	for _, m := range w.Masked {
		copied.Masked = append(copied.Masked, &MaskedArm{Arm: only(m.Arm), Mask: m.Mask, MaskTemp: m.MaskTemp})
	}

	return &copied
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
