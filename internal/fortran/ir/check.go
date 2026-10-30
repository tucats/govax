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
				err = c.expr(s.Src)
			}

			if err == nil && s.Dst.T != s.Src.Type() {
				err = fmt.Errorf("%s assigns a %s value to a %s place", StmtText(s), s.Src.Type(), s.Dst.T)
			}

		case *Call:
			for _, a := range s.Args {
				if a.Ref != nil {
					err = c.mem(a.Ref)
				} else {
					err = c.expr(a.Value)
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

		return c.expr(t.Cond)
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
	case m.Cell == nil && m.Datum == nil && m.Param == nil:
		return fmt.Errorf("a place with no base")
	}

	if m.Index != nil {
		if m.Index.Type() != Long {
			return fmt.Errorf("an index of type %s", m.Index.Type())
		}

		return c.expr(m.Index)
	}

	return nil
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

		if e.Op != Cvt && e.X.Type() != e.T {
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
	}

	return nil
}
