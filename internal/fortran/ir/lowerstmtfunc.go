package ir

import (
	"github.com/tucats/govax/internal/fortran/sema"
)

// Statement functions (section 6.2.1), expanded in line at each reference
// (Decision 8 of docs/PHASE-79). The definition's expression is lowered again
// each time, with its dummy arguments standing for places: symbolMem finds them
// in bound while the reference is being expanded. So with
//
//	AREA(R) = 3.14*R*R
//	Y = AREA(X) + AREA(Z(I))
//
// the first reference's R is X itself, and the second's a temporary holding
// Z(I), and the statement is what it would be with the multiplications written
// out. A temporary stays busy to the statement's end (release), as long as the
// tree the expansion returns reads it.
//
// There is no recursion to worry about: a definition can use only the statement
// functions before it, and a reference's actual arguments are lowered before its
// dummy arguments are bound, so F(F(X)) has F's X bound once at a time.

// bindDummies gives the dummy arguments of the statement function e refers to
// their places, and returns what takes them away again.
func (l *lowerer) bindDummies(e *sema.StmtFuncRef) (unbind func()) {
	places := make([]*Mem, len(e.Args))

	for i, actual := range e.Args {
		places[i] = l.actualPlace(e.Def.Dummies[i], actual)
	}

	for i, d := range e.Def.Dummies {
		l.bound[d] = places[i]
	}

	return func() {
		for _, d := range e.Def.Dummies {
			delete(l.bound, d)
		}
	}
}

// actualPlace is where the dummy argument d finds its actual argument's value
// (the author's answer 5 in docs/PHASE-79): where it is, if it is a constant or
// a variable of d's type and length, and otherwise in a temporary it is assigned
// to, as an assignment would, so a CHARACTER value is cut or padded to d's
// length.
func (l *lowerer) actualPlace(d *sema.Symbol, actual sema.Expr) *Mem {
	char := d.Type.Kind == sema.Character

	switch x := actual.(type) {
	case *sema.Const:
		switch {
		case !char:
			return l.constant(l.constOf(x))
		case len(x.Text) == d.Type.Size:
			return l.text(x.Text)
		}

	case *sema.Var:
		if x.T == d.Type {
			return l.mem(x)
		}
	}

	if char {
		place := Text{l.textTemp(d.Type.Size), LongConst(int32(d.Type.Size))}
		l.emit(&CharMove{l.origin(), place, l.chars(actual)})

		return place.Mem
	}

	place := l.temp(l.typeOf(d.Type))
	l.store(place, actual)

	return place
}

// stmtFuncText is a CHARACTER statement function's value: its expression's, or
// when that has another length, the expression's assigned to a temporary of the
// function's length.
func (l *lowerer) stmtFuncText(e *sema.StmtFuncRef) Text {
	defer l.bindDummies(e)()

	value := l.chars(e.Def.Value)
	if e.Def.Value.Type().Size == e.T.Size {
		return value
	}

	place := Text{l.textTemp(e.T.Size), LongConst(int32(e.T.Size))}
	l.emit(&CharMove{l.origin(), place, value})

	return place
}
