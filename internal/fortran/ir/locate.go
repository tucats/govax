package ir

// Located cells (lowerpointer.go's componentCell). An array pointer
// component's descriptor is in a record, wherever the record is, and every
// routine of lowering that reads or fills a class NCA descriptor reaches one
// as a cell: Mem{Cell: c, Offset: StrideOffset(1)}. So lowering gives such a
// descriptor a cell that is only a name for its place, At (a record in
// $LOCAL, in COMMON, in the frame) or the address in a temporary, Base (a
// record reached through a pointer, a dummy, or an element whose subscripts
// aren't constants); so too an array pointer dummy argument's descriptor, the
// caller's own, at the address its argument holds, Arg. When the routine is
// lowered, resolveLocated makes every place that names such a cell name that
// place instead: H+8 rather than H%V+4, 4(R0) with R0 the address, or
// @4(AP) for the dummy's first longword. Nothing after lowering, the optimizer
// or the code generator, ever sees a located cell.

// resolveLocated makes each place in r's code that names a located cell name
// where the cell is.
func resolveLocated(r *Routine) {
	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			locateStmt(st)
		}

		locateTerminator(b.End)
	}
}

// locateStmt resolves the places of st.
func locateStmt(st Stmt) {
	switch st := st.(type) {
	case *Assign:
		locateMem(st.Dst)
		locateExpr(st.Src)

	case *CharMove:
		locateText(st.Dst)
		locateText(st.Src)

	case *Call:
		locateMem(st.Through)
		locateMem(st.Result)

		for _, a := range st.Args {
			locateMem(a.Ref)
			locateExpr(a.Value)
		}

	case *StackAllocate:
		locateMem(st.Dst)
		locateExpr(st.Size)

	case *StackRelease:
		locateExpr(st.To)

	case *StackMark:
		locateMem(st.Dst)
	}
}

// locateTerminator resolves the places of t.
func locateTerminator(t Terminator) {
	switch t := t.(type) {
	case *Branch:
		locateExpr(t.Cond)
	case *Switch:
		locateExpr(t.Index)
	case *Sign:
		locateExpr(t.Value)
	case *IndirectJump:
		locateMem(t.Through)
	case *Return:
		locateExpr(t.Value)
	}
}

// locateExpr resolves the places of e, which may be nil.
func locateExpr(e Expr) {
	switch e := e.(type) {
	case *Load:
		locateMem(e.Mem)
	case *Addr:
		locateMem(e.Mem)
	case *Op:
		locateExpr(e.X)
		locateExpr(e.Y)
	case *Cmp:
		locateExpr(e.X)
		locateExpr(e.Y)
	case *Field:
		locateExpr(e.X)
		locateExpr(e.Pos)
		locateExpr(e.Size)
	case *InRange:
		locateExpr(e.X)
		locateExpr(e.Low)
		locateExpr(e.High)
	case *CharCmp:
		locateText(e.X)
		locateText(e.Y)
	case *CharIndex:
		locateText(e.X)
		locateText(e.Y)
		locateExpr(e.Base)
	}
}

// locateText resolves a CHARACTER value's places.
func locateText(t Text) {
	locateMem(t.Mem)
	locateExpr(t.Len)
}

// locateMem makes m, if it names a located cell, name where the cell is, and
// resolves the places its index and address read.
func locateMem(m *Mem) {
	if m == nil {
		return
	}

	locateExpr(m.Index)
	locateExpr(m.Ptr)

	for m.Cell != nil && m.Cell.Located() {
		c := m.Cell
		m.Volatile = m.Volatile || c.At != nil && c.At.Volatile

		if c.Base != nil {
			m.Cell, m.Ptr = nil, &Load{&Mem{Cell: c.Base, T: Long}}
			m.Pointee = c.Pointee

			continue
		}

		if c.Arg != nil {
			m.Cell, m.Ptr = nil, &ArgPointer{c.Arg}

			continue
		}

		m.Cell, m.Datum, m.Offset = c.At.Cell, c.At.Datum, m.Offset+c.At.Offset
	}
}
