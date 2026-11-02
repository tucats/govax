package opt

import "github.com/tucats/govax/internal/fortran/ir"

// walker goes through what a statement or terminator computes, each expression before
// those inside it. Two passes share it, and what they share is the knowledge of what
// the code generator computes where: a function's Result and a dummy procedure's place
// are found after the call or by the call itself, a CharIndex's Base must keep its form,
// and a branch's .AND. and .OR. don't always run their second operand.
type walker struct {
	// visit is told of each expression. moved says the value is wanted in a register or
	// moved to a destination, so that one in memory would take an instruction more;
	// maybe says it's computed only some of the time. It says whether to go on inside.
	visit func(e ir.Expr, moved, maybe bool) bool
}

// stmt looks at what st computes before it stores anything.
func (w *walker) stmt(st ir.Stmt) {
	switch st := st.(type) {
	case *ir.Assign:
		w.mem(st.Dst, false)

		if st.InPlace {
			w.expr(ir.InPlaceOperand(st), false, false)
		} else {
			w.expr(st.Src, true, false)
		}

	case *ir.CharMove:
		w.text(st.Dst, false)
		w.text(st.Src, false)

	case *ir.Call:
		for _, a := range st.Args {
			switch {
			case a.Ref != nil:
				w.mem(a.Ref, false)
			case a.Value != nil:
				w.expr(a.Value, false, false)
			}
		}
	}
}

// terminator looks at what t computes.
func (w *walker) terminator(t ir.Terminator) {
	switch t := t.(type) {
	case *ir.Branch:
		w.condition(t.Cond, false)
	case *ir.Switch:
		w.expr(t.Index, true, false)
	case *ir.Sign:
		w.expr(t.Value, false, false)
	case *ir.IndirectJump:
		w.mem(t.Through, false)
	case *ir.Return:
		if t.Value != nil {
			w.expr(t.Value, true, false)
		}
	}
}

// condition looks at a branch's condition, whose .AND. and .OR. go on to their second
// operand only some of the time (codegen's branchTo).
func (w *walker) condition(e ir.Expr, maybe bool) {
	if op, ok := e.(*ir.Op); ok {
		switch op.Op {
		case ir.Not:
			w.condition(op.X, maybe)

			return
		case ir.And, ir.Or:
			w.condition(op.X, maybe)
			w.condition(op.Y, true)

			return
		}
	}

	w.expr(e, false, maybe)
}

// expr looks at e, and inside it if visit says to.
func (w *walker) expr(e ir.Expr, moved, maybe bool) {
	if !w.visit(e, moved, maybe) {
		return
	}

	switch e := e.(type) {
	case *ir.Load:
		w.mem(e.Mem, maybe)

	case *ir.Addr:
		w.mem(e.Mem, maybe)

	case *ir.Op:
		w.expr(e.X, false, maybe)

		if e.Y != nil {
			w.expr(e.Y, false, maybe)
		}

	case *ir.Cmp:
		w.expr(e.X, false, maybe)
		w.expr(e.Y, false, maybe)

	case *ir.Field:
		w.expr(e.X, false, maybe)
		w.expr(e.Pos, false, maybe)
		w.expr(e.Size, false, maybe)

	case *ir.CharCmp:
		w.text(e.X, maybe)
		w.text(e.Y, maybe)

	case *ir.CharIndex:
		// Its Base is a constant or a temporary, as ir.Check requires.
		w.text(e.X, maybe)
		w.text(e.Y, maybe)
	}
}

// mem looks at what finding m computes. An index is wanted in a register. An address
// stays as it is, what's inside it aside.
func (w *walker) mem(m *ir.Mem, maybe bool) {
	if m.Index != nil {
		w.expr(m.Index, true, maybe)
	}

	switch p := m.Ptr.(type) {
	case *ir.Addr:
		w.mem(p.Mem, maybe)
	case *ir.Load:
		w.mem(p.Mem, maybe)
	}
}

// text looks at what finding a CHARACTER value computes.
func (w *walker) text(t ir.Text, maybe bool) {
	w.mem(t.Mem, maybe)
	w.expr(t.Len, false, maybe)
}
