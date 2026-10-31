package ir

import (
	"fmt"
	"sort"
)

// Check verifies what the code generator relies on in a routine: every
// block ends with a terminator, every jump and branch goes to one of the
// routine's blocks, the blocks' names are unique, every cell and datum a
// statement uses is the routine's, and the cells lie in $LOCAL one after
// another, overlapping only where EQUIVALENCE made them. It returns the
// first problem found, or nil. Lowering's tests run it on every routine.
func Check(r *Routine) error {
	blocks := map[*Block]bool{}
	names := map[string]bool{}

	for _, b := range r.Blocks {
		if names[b.Name] {
			return fmt.Errorf("%s: two blocks named %s", r.Name, b.Name)
		}

		blocks[b], names[b.Name] = true, true
	}

	cells := map[*Cell]bool{}

	var local []*Cell

	for _, c := range r.Cells {
		cells[c] = true

		if c.Common == nil {
			local = append(local, c)
		}
	}

	sort.SliceStable(local, func(i, j int) bool { return local[i].Offset < local[j].Offset })

	end := 0

	for _, c := range local {
		if c.Offset < end && (c.Sym == nil || !c.Sym.Equivalenced) {
			return fmt.Errorf("%s: cell %s at %d overlaps the cell before it", r.Name, c.Name, c.Offset)
		}

		end = max(end, c.Offset+c.Size)
	}

	if end > r.LocalSize {
		return fmt.Errorf("%s: the cells end at %d, past $LOCAL's %d bytes", r.Name, end, r.LocalSize)
	}

	data := map[*Datum]bool{}
	for _, d := range r.Data {
		data[d] = true
	}

	c := checker{r: r, blocks: blocks, cells: cells, data: data}

	for _, b := range r.Blocks {
		if err := c.block(b); err != nil {
			return fmt.Errorf("%s: %s: %w", r.Name, b.Name, err)
		}
	}

	return nil
}

// checker is Check's state: what belongs to the routine.
type checker struct {
	r      *Routine
	blocks map[*Block]bool
	cells  map[*Cell]bool
	data   map[*Datum]bool
}

// block checks one block.
func (c *checker) block(b *Block) error {
	for _, s := range b.Stmts {
		var err error

		switch s := s.(type) {
		case *Assign:
			if err = c.mem(s.Dst); err == nil {
				// A character comparison or index may be the whole value
				// assigned.
				switch src := s.Src.(type) {
				case *CharCmp:
					err = c.charCompare(src)
				case *CharIndex:
					err = c.charIndex(src)
				default:
					err = c.expr(s.Src)
				}
			}

			if err == nil && s.Dst.T != s.Src.Type() {
				err = fmt.Errorf("%s assigns a %s value to a %s place", StmtText(s), s.Src.Type(), s.Dst.T)
			}

		case *CharMove:
			if err = c.text(s.Dst); err == nil {
				err = c.text(s.Src)
			}

		case *Call:
			for _, a := range s.Args {
				if a.Routine != "" {
					if a.Ref != nil || a.Value != nil {
						err = fmt.Errorf("%s passes the routine %s and a value too", StmtText(s), a.Routine)
					}
				} else if a.Ref != nil {
					err = c.mem(a.Ref)
				} else if err = c.expr(a.Value); err == nil && a.Value.Type().Size() < 4 {
					// A value is pushed as one or more longwords.
					err = fmt.Errorf("%s passes a %s value, shorter than a longword", StmtText(s), a.Value.Type())
				}

				if err != nil {
					break
				}
			}

			if err == nil && s.Result != nil {
				err = c.mem(s.Result)
			}
		}

		if err != nil {
			return err
		}
	}

	switch t := b.End.(type) {
	case nil:
		return fmt.Errorf("no terminator")
	case *Jump:
		return c.target(t.To)
	case *Branch:
		if err := c.target(t.True); err != nil {
			return err
		}

		if err := c.target(t.False); err != nil {
			return err
		}

		return c.condition(t.Cond)
	case *Return:
		if t.Value != nil {
			return c.expr(t.Value)
		}
	}

	return nil
}

// target checks that a jump's or branch's target is the routine's.
func (c *checker) target(b *Block) error {
	if !c.blocks[b] {
		return fmt.Errorf("a jump to a block not laid out")
	}

	return nil
}

// mem checks that a place's base is the routine's.
func (c *checker) mem(m *Mem) error {
	switch {
	case m.Cell != nil && !c.cells[m.Cell]:
		return fmt.Errorf("cell %s isn't the routine's", m.Cell.Name)
	case m.Datum != nil && !c.data[m.Datum]:
		return fmt.Errorf("datum %s isn't the routine's", m.Datum.Name)
	case m.Cell == nil && m.Datum == nil && m.Param == nil && m.Ptr == nil:
		return fmt.Errorf("a place with no base")
	}

	if m.Ptr != nil {
		if m.Ptr.Type() != Long {
			return fmt.Errorf("an address of type %s", m.Ptr.Type())
		}

		if err := c.expr(m.Ptr); err != nil {
			return err
		}
	}

	if m.Index != nil {
		if m.Index.Type() != Long {
			return fmt.Errorf("an index of type %s", m.Index.Type())
		}

		return c.expr(m.Index)
	}

	return nil
}

// condition checks a Branch's condition: an expression, in which a
// character comparison may be an operand of And, Or, and Not, which the
// code generator branches on as it goes.
func (c *checker) condition(e Expr) error {
	switch e := e.(type) {
	case *CharCmp:
		return c.charCompare(e)

	case *Op:
		switch e.Op {
		case Not:
			return c.condition(e.X)
		case And, Or:
			if err := c.condition(e.X); err != nil {
				return err
			}

			return c.condition(e.Y)
		}
	}

	return c.expr(e)
}

// charCompare checks a character comparison's two places.
func (c *checker) charCompare(e *CharCmp) error {
	if err := c.text(e.X); err != nil {
		return err
	}

	return c.text(e.Y)
}

// charIndex checks a character index's two places, and that its base holds
// no register across MATCHC: a constant, or a load of a temporary.
func (c *checker) charIndex(e *CharIndex) error {
	if err := c.charCompare(&CharCmp{X: e.X, Y: e.Y}); err != nil {
		return err
	}

	switch b := e.Base.(type) {
	case *Const:
	case *Load:
		if b.Mem.Cell == nil || b.Mem.Cell.Kind != TempCell || b.Mem.Index != nil {
			return fmt.Errorf("%s has a base that isn't a temporary", ExprText(e))
		}
	default:
		return fmt.Errorf("%s has a base that must be computed", ExprText(e))
	}

	return c.expr(e.Base)
}

// text checks a CHARACTER value's place: a Byte place and a Long length.
func (c *checker) text(t Text) error {
	if t.Mem.T != Byte {
		return fmt.Errorf("a character value in a %s place", t.Mem.T)
	}

	if t.Len.Type() != Long {
		return fmt.Errorf("a character value's length of type %s", t.Len.Type())
	}

	if err := c.mem(t.Mem); err != nil {
		return err
	}

	return c.expr(t.Len)
}

// expr checks an expression's places and operand types.
func (c *checker) expr(e Expr) error {
	switch e := e.(type) {
	case *Load:
		return c.mem(e.Mem)

	case *Addr:
		return c.mem(e.Mem)

	case *Op:
		if err := c.expr(e.X); err != nil {
			return err
		}

		if e.Op != Cvt && e.Op != Zext && e.Op != Round && e.Op != Low && e.X.Type() != e.T {
			return fmt.Errorf("%s has a %s operand", ExprText(e), e.X.Type())
		}

		if e.Y != nil {
			if err := c.expr(e.Y); err != nil {
				return err
			}

			if e.Y.Type() != e.T {
				return fmt.Errorf("%s has a %s operand", ExprText(e), e.Y.Type())
			}
		}

	case *Cmp:
		if err := c.expr(e.X); err != nil {
			return err
		}

		if err := c.expr(e.Y); err != nil {
			return err
		}

		if e.X.Type() != e.Y.Type() {
			return fmt.Errorf("%s compares a %s with a %s", ExprText(e), e.X.Type(), e.Y.Type())
		}

	case *Field:
		for _, x := range []Expr{e.X, e.Pos, e.Size} {
			if err := c.expr(x); err != nil {
				return err
			}

			if x.Type() != Long {
				return fmt.Errorf("%s has a %s operand", ExprText(e), x.Type())
			}
		}

	case *CharCmp:
		return fmt.Errorf("%s is inside an expression, where it would hold registers across CMPC", ExprText(e))

	case *CharIndex:
		return fmt.Errorf("%s is inside an expression, where it would hold registers across MATCHC", ExprText(e))
	}

	return nil
}
