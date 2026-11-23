package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Generic interfaces (the Fortran 90 standard's 12.3.2.1, 14.1.2.3, and
// 14.1.2.4; docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md,
// finding 11). An interface block with a name makes that name generic: it
// stands for every specific procedure the block's bodies describe, and a
// reference by it is to the one whose interface the reference fits.
//
//	      INTERFACE SWAP
//	        SUBROUTINE SWAPR(A, B)
//	          REAL A, B
//	        END SUBROUTINE
//	        SUBROUTINE SWAPI(I, J)
//	          INTEGER I, J
//	        END SUBROUTINE
//	      END INTERFACE
//	      CALL SWAP(X, Y)       ! SWAPR
//	      CALL SWAP(M, N)       ! SWAPI
//
// The choice is made when compiling, never when the program runs: each body is
// an external procedure's interface (interface.go), and the reference becomes a
// reference to that procedure by its specific name, checked against its
// interface like any other.
//
// "Fits" is 14.1.2.4's "consistent with": the arguments go to the specific's
// dummies by position and keyword as for any explicit interface (argumentOrder),
// every dummy not OPTIONAL gets one, and each argument has its dummy's type, kind,
// and rank exactly. A scalar doesn't fit an array dummy here, as sequence
// association would let it in a plain call: a generic reference is told apart by
// rank (12.4.1.1).
//
// So that the choice is never between two, 14.1.2.3 asks of every pair of
// specific procedures of one generic name in a scoping unit that both be
// functions or both subroutines (GENERICMIX), and that one of them have a dummy
// argument, not OPTIONAL, that the other can't take in its place both by position
// and by keyword: at its position the other has no dummy, or one of another type,
// kind, or rank, and the other's dummy of its name is missing or different the
// same way (GENERICAMBIG). The standard's own example: SWAPR's A and SWAPI's I
// differ in type at position 1, and SWAPI has no A, so the pair is told apart.
// With I REAL they would differ by keyword (SWAPI has no A) but not by position,
// and CALL SWAP(0.0, 1.0) could be either.
//
// A generic name may be an intrinsic function's, which it then extends: a
// reference no specific fits is to the intrinsic (14.1.2.4.1's (4)). The rules of
// 14.1.2.3 then hold "as if the intrinsic consisted of a collection of specific
// procedures", so a specific that takes what the intrinsic takes is
// GENERICINTRINSIC. A generic name in a host is generic in its internal
// procedures, and a block in an internal procedure adds to it there: the
// procedure's own specifics are tried first, then its host's (14.1.2.4.1's (3)).

// Generic is a generic name's specific procedures in one scoping unit.
type Generic struct {
	Name      string
	Specifics []*Symbol

	// Host is the host's symbol for the name, if the name is generic there too,
	// whose specifics are tried after these; nil if not.
	Host *Symbol
}

// compiledBlock reports whether the interface block is one this phase
// compiles: every one, since Phase 106 gave MODULE PROCEDURE its meaning.
func compiledBlock(*ast.InterfaceBlock) bool {
	return true
}

// declareGeneric makes the block's generic name stand for the specific
// procedures specifics, the symbols its bodies declared, adding them to what a
// block before it in this unit gave the name.
func (a *analyzer) declareGeneric(spec ast.GenericSpec, specifics []*Symbol) {
	old := a.scope.byName[spec.Name]

	switch {
	case old == nil:
		s := &Symbol{Name: spec.Name, Offset: -1, Pos: spec.Pos, Typed: true, Generic: &Generic{Name: spec.Name}}
		s.Generic.Host = a.hostGeneric(spec.Name)
		a.scope.enter(s)
		old = s

	case old.broken:
		return

	case old.Generic != nil:

	case slices.Contains(specifics, old):
		// The generic name is one of its specific procedures' own.
		old.Generic = &Generic{Name: spec.Name, Host: a.hostGeneric(spec.Name)}

	default:
		a.report("GENERICDECL", spec.Pos, spec.Name, a.describe(old), old.Pos.Line)
		old.broken = true

		return
	}

	a.note(&old.refs, spec.Pos, RefDefined)
	old.Generic.Specifics = append(old.Generic.Specifics, specifics...)

	if old.Interface == nil && old.Class == Unclassified && len(old.Generic.Specifics) > 0 {
		old.Class = old.Generic.Specifics[0].Class
	}
}

// hostGeneric is the symbol of a host of this unit for the name, if the name is
// generic there; nil otherwise.
func (a *analyzer) hostGeneric(name string) *Symbol {
	if a.host == nil {
		return nil
	}

	if s := a.host.scope.find(name); s != nil && s.Generic != nil {
		return s
	}

	return nil
}

// checkGenerics checks 14.1.2.3's rules for the generic names this unit's
// blocks made: each pair of their specific procedures, and each specific of a
// generic name that is an intrinsic function's against the intrinsic.
func (a *analyzer) checkGenerics() {
	for _, s := range a.unit.Symbols {
		if s.Generic == nil || s.broken {
			continue
		}

		specifics := s.Generic.Specifics

		for i, p := range specifics {
			pu := explicitInterface(p)
			if pu == nil || pu.incomplete {
				continue
			}

			for _, q := range specifics[i+1:] {
				qu := explicitInterface(q)
				if qu == nil || qu.incomplete || p == q {
					continue
				}

				switch {
				case (p.Class == Function) != (q.Class == Function):
					a.report("GENERICMIX", specificPos(q), p.Name, q.Name, s.Name)
				case !distinguishable(dummyShapes(pu), dummyShapes(qu)):
					a.report("GENERICAMBIG", specificPos(q), p.Name, p.Pos.Line, q.Name, s.Name)
				}
			}

			if p.Class == Function && a.intrinsicNamed(s.Name) && takenByIntrinsic(dummyShapes(pu), s.Name) {
				a.report("GENERICINTRINSIC", specificPos(p), p.Name, s.Name)
			}
		}
	}
}

// specificPos is where a specific procedure of a generic name or an operator is
// reported: its interface's header, or for one a USE gave the unit, the USE,
// since its interface is in the module's text (finding 2).
func specificPos(s *Symbol) diag.Position {
	if s.Use != nil {
		return s.Pos
	}

	return explicitInterface(s).AST.Position()
}

// dummyShape is what of a dummy argument tells specific procedures apart: its
// name, type and kind, rank, whether it is OPTIONAL, and whether it is a
// procedure. A type of Invalid is any type, a rank of -1 any rank, and -2 an
// array of any rank, as an intrinsic function's table gives them.
type dummyShape struct {
	name     string
	t        Type
	rank     int
	optional bool
	proc     bool
}

// dummyShapes are the interface u's dummies' shapes.
func dummyShapes(u *Unit) []dummyShape {
	shapes := make([]dummyShape, len(u.Params))
	for i, p := range u.Params {
		shapes[i] = dummyShape{p.Name, p.Type, len(p.Dims), p.Optional, p.DummyProcedure()}
	}

	return shapes
}

// differ reports whether one dummy can't stand in for the other: one is a
// procedure and the other not, or their types, kinds, or ranks differ. A
// CHARACTER length doesn't count, nor do two procedures differ.
func differ(x, y dummyShape) bool {
	switch {
	case x.proc || y.proc:
		return x.proc != y.proc
	case x.rank >= 0 && y.rank >= 0 && x.rank != y.rank:
		return true
	case x.rank == -2 && y.rank == 0, x.rank == 0 && y.rank == -2:
		return true
	case x.t.Kind == Invalid || y.t.Kind == Invalid:
		return false
	case x.t.Kind == Record && y.t.Kind == Record && x.t.Struct != nil && y.t.Struct != nil:
		// Two derived types differ unless they are one type (structvalue.go).
		return !argumentStructures(x.t.Struct, y.t.Struct)
	}

	return x.t.Kind != y.t.Kind || (x.t.Kind != Character && x.t.Size != y.t.Size)
}

// distinguishable is 14.1.2.3's rule for two specific procedures of one generic
// name: one of them has a dummy argument, not OPTIONAL, that the other has nothing
// in the place of, both by position and by keyword.
func distinguishable(p, q []dummyShape) bool {
	return tellsApart(p, q) || tellsApart(q, p)
}

// tellsApart reports whether p has a dummy argument, not OPTIONAL, at whose
// position q has none or a different one, and whose name in q is missing or a
// different one.
func tellsApart(p, q []dummyShape) bool {
	for i, d := range p {
		if d.optional {
			continue
		}

		byPosition := i >= len(q) || differ(d, q[i])

		k := slices.IndexFunc(q, func(e dummyShape) bool { return e.name != "" && e.name == d.name })
		byKeyword := k < 0 || differ(d, q[k])

		if byPosition && byKeyword {
			return true
		}
	}

	return false
}

// takenByIntrinsic reports whether the intrinsic function name takes what a
// specific procedure with the dummies p takes, so that the two can't be told
// apart: some row of the intrinsic, with some number of arguments it allows, is
// a specific that 14.1.2.3 can't tell from p. An elemental function takes an
// array of any rank; another, what its table's dummies say (intrinsics.go):
// SUM's ARRAY an array of any rank, its DIM a scalar.
func takenByIntrinsic(p []dummyShape, name string) bool {
	for _, f := range formsNamed(name) {
		hi := f.max
		if hi < 0 || hi > len(p)+1 {
			hi = len(p) + 1
		}

		for n := f.min; n <= hi; n++ {
			row := make([]dummyShape, n)
			for i := range row {
				row[i] = dummyShape{t: f.arg}

				switch {
				case f.class == 'E':
					row[i].rank = -1
				case i < len(f.dummies):
					row[i].rank = f.dummies[i].rank
				}

				if i < len(f.dummies) {
					row[i].name = f.dummies[i].name
				}
			}

			if !distinguishable(p, row) {
				return true
			}
		}
	}

	return false
}

// genericChoice is how a reference by a generic name came out.
type genericChoice int

const (
	genericFailed    genericChoice = iota // nothing fits, reported
	genericSpecific                       // a specific procedure fits
	genericIntrinsic                      // the intrinsic function of the name is the one
)

// resolveGeneric chooses the specific procedure the reference by the generic name
// s fits (14.1.2.4.1), a function if function is set, else a subroutine: list is
// the reference's arguments as written, the alternate return specifiers taken out
// (which were in the places places). It returns the specific and its arguments in
// its dummies' order, a zero Arg for each left out; or genericIntrinsic, for the
// caller to analyze the reference as the intrinsic function's; or genericFailed,
// after GENERICNONE.
func (a *analyzer) resolveGeneric(s *Symbol, list []ast.Expr, places []int, function bool, pos diag.Position) (*Symbol, []Arg, genericChoice) {
	analyzed := make([]Arg, len(list))
	open := false

	for i, e := range list {
		// NULL(), a pointer of MOLD's kind, or of any if it has none, which
		// then mustn't leave two specifics to choose from (null.go).
		if app, ok := a.nullReference(unkeyed(e)); ok {
			n := a.genericNull(app)
			if n == nil {
				return nil, nil, genericFailed
			}

			analyzed[i], open = Arg{Expr: n, Pointer: true}, open || n.open

			continue
		}

		// An array section or expression is kept until the specific is
		// chosen: an assumed-shape dummy takes it (shaped.go).
		analyzed[i] = a.argValue(unkeyed(e))
		a.notePassed(analyzed[i])

		if _, bad := analyzed[i].Expr.(*bad); bad {
			return nil, nil, genericFailed
		}
	}

	var chosen *Symbol

	var chosenSlots []int

	// A specific that fits exactly first, then, only if none does, an
	// elemental one that fits element by element (14.1.2.4.1; elemental.go).
	for _, elementwise := range []bool{false, true} {
		if chosen != nil {
			break
		}

		chosen, chosenSlots = nil, nil

		for g := s; g != nil; g = g.Generic.Host {
			for _, spec := range g.Generic.Specifics {
				target := explicitInterface(spec)
				if target == nil || target.incomplete || spec.broken || (spec.Class == Function) != function {
					continue
				}

				slots, fault := argumentOrder(spec.Name, target, list, places, pos)
				if fault != nil || !fits(slots, analyzed, target, elementwise) {
					continue
				}

				if chosen != nil {
					// A second fits only by an open NULL().
					a.report("NULLMOLD", pos, s.Name, chosen.Name, spec.Name)

					return nil, nil, genericFailed
				}

				chosen, chosenSlots = spec, slots

				if !open {
					break
				}
			}

			if chosen != nil && !open {
				break
			}
		}
	}

	if chosen != nil {
		target := explicitInterface(chosen)
		args := make([]Arg, len(chosenSlots))

		for j, i := range chosenSlots {
			p := target.Params[j]

			switch {
			case i < 0:
			case p.Pointer:
				// The pointer itself (pointerArg), analyzed again as one.
				args[j] = a.pointerArg(unkeyed(list[i]), j, chosen, p)
			case p.AssumedShape():
				// An array to an assumed-shape dummy goes by descriptor
				// (shaped.go).
				args[j] = a.shaped(analyzed[i])
			case target.Elemental && len(p.Dims) == 0:
				args[j] = a.elementalArg(analyzed[i])
			default:
				args[j] = a.sequenced(analyzed[i], p)
			}
		}

		a.noteThrough(chosen, pos)

		return chosen, args, genericSpecific
	}

	if function && a.intrinsicNamed(s.Name) {
		return nil, nil, genericIntrinsic
	}

	a.report("GENERICNONE", pos, s.Name, describeActuals(list, analyzed))

	return nil, nil, genericFailed
}

// fits reports whether the arguments analyzed, put in place by slots, fit the
// interface target exactly: each given to a dummy has its type, kind, and rank (a
// procedure for a dummy procedure), and a null one goes only to an OPTIONAL dummy.
// With elementwise set, the reference fits as an elemental procedure's:
// target is elemental, and an array of a scalar dummy's type and kind
// fits it too, each element in turn (elemental.go).
func fits(slots []int, analyzed []Arg, target *Unit, elementwise bool) bool {
	if elementwise && !target.Elemental {
		return false
	}

	for j, i := range slots {
		p := target.Params[j]

		if i < 0 {
			continue
		}

		arg := analyzed[i]
		_, proc := arg.Expr.(*Procedure)
		null, isNull := arg.Expr.(*Null)

		switch {
		case isNull:
			// NULL() fits only a pointer dummy, of MOLD's kind if it has one.
			if !p.Pointer || !null.open && differ(dummyShape{t: null.T, rank: null.Rank}, dummyShape{t: p.Type, rank: len(p.Dims)}) {
				return false
			}
		case arg.Expr == nil:
			if !p.Optional {
				return false
			}
		case arg.Hollerith || arg.Typeless:
			return false
		case proc || p.DummyProcedure():
			if proc != p.DummyProcedure() {
				return false
			}
		case elementwise && len(p.Dims) == 0:
			if differ(dummyShape{t: arg.Expr.Type()}, dummyShape{t: p.Type}) {
				return false
			}
		case differ(actualShape(arg), dummyShape{t: p.Type, rank: len(p.Dims)}):
			return false
		}
	}

	return true
}

// actualShape is an actual argument's type and rank, as differ compares them.
func actualShape(arg Arg) dummyShape {
	return dummyShape{t: arg.Expr.Type(), rank: argumentRank(arg.Expr)}
}

// describeActuals says what a reference's arguments are, for GENERICNONE: "a
// REAL and an INTEGER array of rank 1", a keyword's with its name.
func describeActuals(list []ast.Expr, analyzed []Arg) string {
	if len(list) == 0 {
		return "no arguments"
	}

	parts := make([]string, len(list))

	for i, arg := range analyzed {
		what := "a null argument"

		switch e := arg.Expr.(type) {
		case nil:
		case *Procedure:
			what = "a procedure"
		case *Null:
			what = "NULL()"
			if !e.open {
				what = "a pointer to " + pointedText(e.T, e.Rank)
			}
		default:
			what = article(e.Type().String())
			if rank := argumentRank(e); rank > 0 {
				what += " array of rank " + strconv.Itoa(rank)
			}
		}

		if k, ok := list[i].(*ast.KeywordArg); ok {
			what = k.Name + "=" + what
		}

		parts[i] = what
	}

	return nameList(parts)
}
