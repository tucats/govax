package opt

import "github.com/tucats/govax/internal/fortran/ir"

// OneCharacter: a string of one character is a byte, and the VAX has instructions for
// bytes.
//
//	      CHARACTER*80 LINE, OUT
//	      ...
//	      IF (LINE(I:I) .NE. ' ') THEN
//	         OUT(J:J) = LINE(I:I)
//
// Lowering treats these as it does any CHARACTER value, and the code generator writes
// the string instructions, which change registers they don't need to (R0 to R3 for a
// CMPC, R0 to R5 for a MOVC) and which the routine's entry mask then has to save:
//
//	      MOVL    S.I,R0
//	      CMPC3   #1,S.LINE-1[R0],S.$K1
//
// This pass makes them what they are:
//
//	      MOVL    S.I,R0
//	      CMPB    S.LINE-1[R0],#32
//
// # What it changes, and why it is the same
//
// A move whose destination is one character long and whose source is at least that is
// a byte moved: the rest of the source would be cut. A move whose source is shorter
// than its destination has blanks to fill in, and stays. A length that isn't a constant
// may turn out to be anything, so it stays too.
//
// A comparison of two values one character long is a comparison of two bytes. CMPB
// sets the condition codes as CMPC3 does for its first different byte, but only the
// unsigned branches read them correctly: section 2.3.3 of the language manual has
// characters in the ASCII order, and CHAR(200) comes after 'A' though it is a negative
// byte. So the comparison becomes ir's LtU and so on, not Lt. A value of different
// lengths is compared as if the shorter were filled with blanks, and stays a CMPC5.
//
// A character constant is a read of $PDATA, which is just as well an immediate operand,
// one memory reference fewer, and then its text may have no use left. The same goes for
// a move: C = 'X' is MOVB #88,C.
//
// # This pass runs first
//
// What it leaves is an ordinary Assign of a Byte, or an ordinary Cmp, so all the passes
// after it know what to do with one: Forward knows C holds 'X' after C = 'X', a loop's
// LINE(1:1) can be moved out of it, and Check no longer needs it to be a statement's
// whole value. That also means a length Forward works out later, C(1:L) with L known to
// be 1, stays a string instruction, which is correct and no worse than it was.
func OneCharacter(r *ir.Routine) {
	replaced := map[*ir.Datum]bool{}

	for _, b := range r.Blocks {
		for i, st := range b.Stmts {
			switch st := st.(type) {
			case *ir.CharMove:
				if isOneByte(st.Dst) && isByteAtLeast(st.Src) {
					b.Stmts[i] = ir.NewAssign(st.Source(), st.Dst.Mem, byteValue(st.Src, replaced))
				}

			case *ir.Assign:
				if c, ok := st.Src.(*ir.CharCmp); ok {
					st.Src = oneCharacterCondition(c, replaced)
				}
			}
		}

		if t, ok := b.End.(*ir.Branch); ok {
			t.Cond = oneCharacterCondition(t.Cond, replaced)
		}
	}

	dropUnusedText(r, replaced)
}

// oneCharacterCondition is e, a Branch's condition or an Assign's value, with each
// character comparison of one byte against one byte made a byte comparison. Those are
// the only places lowering puts a CharCmp (ir.CharCmp), under And, Or, and Not at most.
// e is left as it is otherwise, and a new Op made rather than one changed, as an
// expression may be used twice.
func oneCharacterCondition(e ir.Expr, replaced map[*ir.Datum]bool) ir.Expr {
	switch e := e.(type) {
	case *ir.CharCmp:
		if isOneByte(e.X) && isOneByte(e.Y) {
			return &ir.Cmp{Op: unsignedComparisons[e.Op], X: byteValue(e.X, replaced), Y: byteValue(e.Y, replaced)}
		}

	case *ir.Op:
		switch e.Op {
		case ir.Not:
			if x := oneCharacterCondition(e.X, replaced); x != e.X {
				return &ir.Op{Op: e.Op, T: e.T, X: x}
			}

		case ir.And, ir.Or:
			x, y := oneCharacterCondition(e.X, replaced), oneCharacterCondition(e.Y, replaced)
			if x != e.X || y != e.Y {
				return &ir.Op{Op: e.Op, T: e.T, X: x, Y: y}
			}
		}
	}

	return e
}

// unsignedComparisons are a character comparison's operators on bytes.
var unsignedComparisons = map[ir.Operator]ir.Operator{
	ir.Eq: ir.Eq, ir.Ne: ir.Ne, ir.Lt: ir.LtU, ir.Le: ir.LeU, ir.Gt: ir.GtU, ir.Ge: ir.GeU,
}

// isOneByte reports whether t is a place of bytes, known to be one long.
func isOneByte(t ir.Text) bool {
	n, ok := t.Len.(*ir.Const)

	return ok && n.Int() == 1 && t.Mem.T == ir.Byte
}

// isByteAtLeast reports whether t is a place of bytes known to be at least one long.
func isByteAtLeast(t ir.Text) bool {
	n, ok := t.Len.(*ir.Const)

	return ok && n.Int() >= 1 && t.Mem.T == ir.Byte
}

// byteValue is t's first character: the constant itself if it is one, and the text it
// came from is noted, or a load.
func byteValue(t ir.Text, replaced map[*ir.Datum]bool) ir.Expr {
	m := t.Mem

	if d := m.Datum; d != nil && d.Kind == ir.TextDatum && m.Index == nil && m.Ptr == nil && m.Offset >= 0 && m.Offset < len(d.Text) {
		replaced[d] = true

		return ir.IntConst(ir.Byte, int32(d.Text[m.Offset]))
	}

	return &ir.Load{Mem: m}
}

// dropUnusedText takes out of the routine's data the texts this pass read for the
// last time. A text may be shared with anything that said the same words: a descriptor
// (which walkMems looks at), a FORMAT, or a NAMELIST's names.
func dropUnusedText(r *ir.Routine, replaced map[*ir.Datum]bool) {
	if len(replaced) == 0 {
		return
	}

	walkMems(r, func(m *ir.Mem) {
		if m.Datum != nil {
			delete(replaced, m.Datum)
		}
	})

	for _, d := range r.Data {
		switch {
		case d.Format != nil:
			delete(replaced, d.Format.Text)

		case d.Namelist != nil:
			delete(replaced, d.Namelist.Name)

			for _, n := range d.Namelist.Entities {
				delete(replaced, n.Name)
			}
		}
	}

	kept := r.Data[:0]

	for _, d := range r.Data {
		if !replaced[d] {
			kept = append(kept, d)
		}
	}

	r.Data = kept
}
