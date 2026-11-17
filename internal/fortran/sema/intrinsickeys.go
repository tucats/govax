package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
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
// The table's last column (intrinsics.go) is each function's dummies; a
// function without them takes its arguments by position only, as VAX FORTRAN
// always did.
//
// A function's KIND (13.13: INT, REAL, CMPLX, NINT, AINT, ANINT, LOGICAL) is
// the kind of its result. It is taken out of the arguments before the row is
// chosen, since it never takes part in the choice (its arguments' types do),
// and must be a constant (KINDNOTCONST): a result's type is known when
// compiling.

// intrinsicArgs is a reference's arguments in its function's dummies' order,
// and its KIND, if it has one (nil if not). A function without dummies' names
// has its arguments as written. An optional dummy left out before one given
// is nil (SELECTED_REAL_KIND(R=40)'s P); those at the end are left off.
type intrinsicArgs struct {
	args []ast.Expr
	kind ast.Expr
}

// dummiesOf is the dummies a reference to name by forms may name: the rows'
// own, if they have them and the reference uses the name they were given
// for. A specific name (IINT of INT's row) takes its arguments by position.
func dummiesOf(name string, forms []form) []dummyArg {
	f := forms[0]
	if f.dummies == nil || f.generic != "" && f.generic != name {
		return nil
	}

	return f.dummies
}

// intrinsicSlots puts e's arguments in its function's dummies' order
// (intrinsicArgs), reporting what's wrong: a keyword the function hasn't
// (KEYWORDNAME), a dummy given twice (KEYWORDTWICE), an argument without a
// keyword after one with (KEYWORDORDER), more arguments than dummies
// (INTRINARGS), a null argument (NULLINTRINARG), and a required one left out
// (MISSINGARG). quiet leaves them unreported, for the constant evaluator's
// first try (expr.go's folded).
func (a *analyzer) intrinsicSlots(e *ast.Apply, forms []form, quiet bool) (intrinsicArgs, bool) {
	report := func(ident string, args ...any) (intrinsicArgs, bool) {
		if !quiet {
			a.report(ident, e.Pos, args...)
		}

		return intrinsicArgs{}, false
	}

	dummies := dummiesOf(e.Name, forms)
	if dummies == nil {
		for _, x := range e.Args {
			if k, ok := x.(*ast.KeywordArg); ok {
				if !quiet {
					a.notImplForm(k.Pos, "a keyword argument to the intrinsic function "+e.Name)
				}

				return intrinsicArgs{}, false
			}
		}

		return intrinsicArgs{args: e.Args}, true
	}

	slots := make([]ast.Expr, len(dummies))
	keywords := false

	for i, x := range e.Args {
		if k, ok := x.(*ast.KeywordArg); ok {
			keywords = true

			j := dummyIndex(dummies, k.Name)

			switch {
			case j < 0:
				return report("KEYWORDNAME", e.Name, k.Name, dummyNames(dummies))
			case slots[j] != nil:
				return report("KEYWORDTWICE", k.Name, e.Name)
			}

			slots[j] = k.X

			continue
		}

		switch {
		case keywords:
			return report("KEYWORDORDER", e.Name)
		case i >= len(dummies):
			return report("INTRINARGS", e.Name, takesAll(forms), len(e.Args))
		case x == nil:
			return report("NULLINTRINARG", e.Name)
		}

		slots[i] = x
	}

	for j, d := range dummies {
		if slots[j] == nil && !d.optional {
			return report("MISSINGARG", e.Name, d.name)
		}
	}

	var out intrinsicArgs

	for j, d := range dummies {
		if d.name == "KIND" {
			out.kind = slots[j]

			continue
		}

		out.args = append(out.args, slots[j])
	}

	for len(out.args) > 0 && out.args[len(out.args)-1] == nil {
		out.args = out.args[:len(out.args)-1]
	}

	return out, true
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
