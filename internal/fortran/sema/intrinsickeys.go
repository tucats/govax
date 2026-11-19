package sema

import (
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Keyword arguments and KIND= for the intrinsic functions (docs/planning/
// PHASE-100 - Fortran 90 declarations and kinds.md, subtask 6; Decision 14 of
// docs/planning/PHASE-99 - Fortran 90.md).
//
// Fortran 90 lets a reference name an argument by its dummy's name:
// SELECTED_REAL_KIND(R=40) gives R and leaves P out, and INT(X, KIND=2) gives
// INT's KIND. The standard's 12.4.1 is the rule: the arguments without a
// keyword come first and fill the dummies in order, those with one fill the
// dummy they name, no dummy twice, and only an optional dummy may be left out.
// The table's last column (intrinsics.go) is each function's dummies, every
// function's since Phase 108 (Table D-3's by the standard's names).
//
// A function's KIND (13.13: INT, REAL, CMPLX, NINT, AINT, ANINT, LOGICAL) is
// the kind of its result. It is taken out of the arguments before the row is
// chosen, since it never takes part in the choice (its arguments' types do),
// and must be a constant (KINDNOTCONST): a result's type is known when
// compiling.

// intrinsicArgs is a reference's arguments in its function's dummies' order,
// and its KIND, if it has one (nil if not). An optional dummy left out before one given
// is nil (SELECTED_REAL_KIND(R=40)'s P); those at the end are left off.
type intrinsicArgs struct {
	args []ast.Expr
	kind ast.Expr
}

// dummiesOf is the dummies a reference to name by forms may name: the rows'
// own, which a specific name (IINT of INT's rows) shares with its generic,
// but for KIND: a specific name's result has one kind.
func dummiesOf(name string, forms []form) []dummyArg {
	f := forms[0]
	if f.generic == "" || f.generic == name {
		return f.dummies
	}

	if k := dummyIndex(f.dummies, "KIND"); k >= 0 {
		return slices.Delete(slices.Clone(f.dummies), k, k+1)
	}

	return f.dummies
}

// intrinsicSlots puts e's arguments in its function's dummies' order
// (intrinsicArgs), reporting what's wrong (keywordSlots). quiet leaves it
// unreported, for the constant evaluator's first try (expr.go's folded).
func (a *analyzer) intrinsicSlots(e *ast.Apply, forms []form, quiet bool) (intrinsicArgs, bool) {
	dummies := dummiesOf(e.Name, forms)

	slots, ok := a.keywordSlots(e.Pos, e.Name, false, e.Args, dummies, forms[0].variadic,
		func() string { return takesAll(forms) }, quiet)
	if !ok {
		return intrinsicArgs{}, false
	}

	var out intrinsicArgs

	for j, x := range slots {
		if j < len(dummies) && dummies[j].name == "KIND" {
			out.kind = x

			continue
		}

		out.args = append(out.args, x)
	}

	for len(out.args) > 0 && out.args[len(out.args)-1] == nil {
		out.args = out.args[:len(out.args)-1]
	}

	return out, true
}

// keywordSlots puts a reference's arguments, list, in the order of the
// procedure's dummies, an intrinsic function's or subroutine's: a nil for an
// optional dummy left out, and those past the dummies after them if variadic
// allows them. It reports what's wrong, unless quiet: a keyword the procedure
// hasn't (KEYWORDNAME), a dummy given twice (KEYWORDTWICE), an argument without
// a keyword after one with (KEYWORDORDER), more arguments than dummies or,
// without keywords, fewer than it needs (INTRINARGS, with takes the count it
// allows), a null argument
// (NULLINTRINARG), and a required one left out (MISSINGARG); for a subroutine
// (sub), INTRINSUBARGS and NULLINTRINSUB.
func (a *analyzer) keywordSlots(pos diag.Position, name string, sub bool, list []ast.Expr, dummies []dummyArg,
	variadic bool, takes func() string, quiet bool) ([]ast.Expr, bool) {
	what, tooMany, null := "the intrinsic function "+name, "INTRINARGS", "NULLINTRINARG"
	if sub {
		what, tooMany, null = "the intrinsic subroutine "+name, "INTRINSUBARGS", "NULLINTRINSUB"
	}

	report := func(ident string, args ...any) ([]ast.Expr, bool) {
		if !quiet {
			a.report(ident, pos, args...)
		}

		return nil, false
	}

	slots := make([]ast.Expr, len(dummies))
	keywords := false

	for i, x := range list {
		if k, ok := x.(*ast.KeywordArg); ok {
			keywords = true

			j := dummyIndex(dummies, k.Name)

			switch {
			case j < 0 && len(dummies) == 0:
				return report(tooMany, name, takes(), len(list))
			case j < 0:
				return report("KEYWORDNAME", what, k.Name, dummyNames(dummies))
			case slots[j] != nil:
				return report("KEYWORDTWICE", k.Name, what)
			}

			slots[j] = k.X

			continue
		}

		switch {
		case keywords:
			return report("KEYWORDORDER", name)
		case i >= len(dummies) && !variadic:
			return report(tooMany, name, takes(), len(list))
		case x == nil:
			return report(null, name)
		case i >= len(dummies):
			slots = append(slots, x)

			continue
		}

		slots[i] = x
	}

	for j, d := range dummies {
		switch {
		case slots[j] != nil || d.optional:
		case !keywords:
			// Too few by position, as VAX FORTRAN's references always were.
			return report(tooMany, name, takes(), len(list))
		default:
			return report("MISSINGARG", what, d.name)
		}
	}

	return slots, true
}

// dummyIndex is the place of the dummy named name, or -1.
func dummyIndex(dummies []dummyArg, name string) int {
	for j, d := range dummies {
		if d.name == name {
			return j
		}
	}

	return -1
}

// dummyNames is the dummies' names for KEYWORDNAME: A and KIND, or X, Y, and
// KIND.
func dummyNames(dummies []dummyArg) string {
	names := make([]string, len(dummies))
	for j, d := range dummies {
		names[j] = d.name
	}

	return nameList(names)
}

// nameList is names as a message lists them: "A", "A and B", "A, B, and C".
func nameList(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}

	return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
}

// takesAll is how many arguments a function takes with its KIND counted, as
// INTRINARGS says it when a reference gives more than it has dummies.
func takesAll(forms []form) string {
	if dummyIndex(forms[0].dummies, "KIND") < 0 {
		return takes(forms)
	}

	lo, hi := forms[0].min, forms[0].max+1
	for _, f := range forms {
		lo, hi = min(lo, f.min), max(hi, f.max+1)
	}

	if hi == lo+1 {
		return strconv.Itoa(lo) + " or " + count(hi, "argument")
	}

	return strconv.Itoa(lo) + " to " + count(hi, "argument")
}

// resultKind is the type a reference's KIND makes its result, whose type
// without it would be t: the kind's type of t's kind (INTEGER's kind 2 is
// INTEGER*2, COMPLEX's kind 8 COMPLEX*16). ok is false when KIND isn't a
// constant integer, 0 or more, or isn't one of the type's kinds, reported.
func (a *analyzer) resultKind(kind ast.Expr, t Type) (Type, bool) {
	k, ok := a.kindValue(kind)
	if !ok {
		return invalid, false
	}

	r, problem := kindType(baseTypes[t.Kind], k)
	if problem != nil {
		a.reportKind(problem, kind.Position())

		return invalid, false
	}

	return r, true
}
