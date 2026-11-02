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

	case *ir.InRange:
		w.expr(e.X, false, maybe)
		w.expr(e.Low, false, maybe)
		w.expr(e.High, false, maybe)

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

// memRole is what a statement does with a place.
type memRole int

// The roles.
const (
	roleRead    memRole = iota // its value is read as an operand
	roleIndex                  // its value is read as an index
	rolePointer                // its value is read as the address of another place
	roleWrite                  // its value is written
	roleAddress                // its address is wanted, which a register hasn't
	roleText                   // it is a CHARACTER value, reached by address
)

// memWalker goes through every place a statement or terminator names, as an operand,
// a destination, or on the way to another: where walker asks what is computed, this
// asks what is touched, a Call's Result and Through among them.
type memWalker struct {
	// visit is told of each place: what's done with it, and whether it's done only some
	// of the time.
	visit func(m *ir.Mem, role memRole, maybe bool)

	maybe bool
}

// stmt goes through st.
func (w *memWalker) stmt(st ir.Stmt) {
	switch st := st.(type) {
	case *ir.Assign:
		w.use(st.Dst, roleWrite)
		w.expr(st.Src)

	case *ir.CharMove:
		w.text(st.Dst)
		w.text(st.Src)

	case *ir.Call:
		if st.Through != nil {
			w.use(st.Through, roleAddress)
		}

		for _, a := range st.Args {
			switch {
			case a.Ref != nil:
				w.use(a.Ref, roleAddress)
			case a.Value != nil:
				w.expr(a.Value)
			}
		}

		if st.Result != nil {
			w.use(st.Result, roleWrite)
		}
	}
}

// terminator goes through t.
func (w *memWalker) terminator(t ir.Terminator) {
	switch t := t.(type) {
	case *ir.Branch:
		w.condition(t.Cond)
	case *ir.Switch:
		w.expr(t.Index)
	case *ir.Sign:
		w.expr(t.Value)
	case *ir.IndirectJump:
		w.use(t.Through, roleRead)
	case *ir.Return:
		if t.Value != nil {
			w.expr(t.Value)
		}
	}
}

// condition goes through a branch's condition, whose .AND. and .OR. go on to their
// second operand only some of the time.
func (w *memWalker) condition(e ir.Expr) {
	if op, ok := e.(*ir.Op); ok {
		switch op.Op {
		case ir.Not:
			w.condition(op.X)

			return
		case ir.And, ir.Or:
			w.condition(op.X)

			was := w.maybe
			w.maybe = true
			w.condition(op.Y)
			w.maybe = was

			return
		}
	}

	w.expr(e)
}

// expr goes through e.
func (w *memWalker) expr(e ir.Expr) {
	switch e := e.(type) {
	case *ir.Load:
		w.use(e.Mem, roleRead)

	case *ir.Addr:
		w.use(e.Mem, roleAddress)

	case *ir.Op:
		w.expr(e.X)

		if e.Y != nil {
			w.expr(e.Y)
		}

	case *ir.Cmp:
		w.expr(e.X)
		w.expr(e.Y)

	case *ir.Field:
		w.expr(e.X)
		w.expr(e.Pos)
		w.expr(e.Size)

	case *ir.InRange:
		w.expr(e.X)
		w.expr(e.Low)
		w.expr(e.High)

	case *ir.CharCmp:
		w.text(e.X)
		w.text(e.Y)

	case *ir.CharIndex:
		w.text(e.X)
		w.text(e.Y)
		w.expr(e.Base)
	}
}

// use goes through the place m, and what finding it reads.
func (w *memWalker) use(m *ir.Mem, role memRole) {
	w.visit(m, role, w.maybe)

	if l, ok := m.Index.(*ir.Load); ok {
		w.use(l.Mem, roleIndex)
	} else if m.Index != nil {
		w.expr(m.Index)
	}

	switch p := m.Ptr.(type) {
	case nil:
	case *ir.Load:
		w.use(p.Mem, rolePointer)
	default:
		w.expr(p)
	}
}

// text goes through a CHARACTER value.
func (w *memWalker) text(t ir.Text) {
	w.use(t.Mem, roleText)
	w.expr(t.Len)
}
