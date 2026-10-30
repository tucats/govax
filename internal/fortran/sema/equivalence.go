package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// EQUIVALENCE (section 4.6). Each list says that its items start at one
// place, so what the lists really describe are groups of names whose
// places are fixed relative to each other. Take
//
//	DIMENSION A(4), B(6)
//	EQUIVALENCE (A(2), B(1)), (B(3), C)
//
// The first list puts B 4 bytes after A: A(2) is 4 bytes into A, and B(1)
// is B's start. In a group's own coordinates, A is at 0 and B at 4. The
// second list has C at B's 8 bytes in, so it joins them at 12. The three
// are one group, and placing any of them places them all.
//
// Reading the lists is merging groups. Each name starts in a group of its
// own, at 0. An item is a name and how far into it the item starts (its
// offset). Two items of a list share a place, so in a merged group
//
//	group offset of X + X's item's offset = group offset of Y + Y's item's offset
//
// and merging moves all of one group by whatever makes that true. If the
// two are in one group already, nothing can move, and the sum either
// agrees or the program has put a name in two places (EQUIVCONFLICT): the
// manual's "two or more elements of the same array", and two substrings
// of one variable starting at different characters.
//
// Then the group is placed (section 4.6.3). A group with a name in a
// common block is in that block: the name's place there says where the
// group's 0 is, which may not be before the block's start (EQUIVBEFORE:
// B(1) before A(1) in the manual's invalid example), and the group may
// run past the block's end, which lengthens it. Two names of a group in
// different blocks, or in one block at places the group disagrees with,
// can't be (EQUIVBLOCKS, EQUIVCOMMON). Any other group goes in $LOCAL as a
// unit, when layout reaches its first name.

// equivGroup is names whose places are fixed relative to each other. A
// member's groupOffset is its place in the group; once merged and placed,
// the smallest is 0, unless the group is in a common block.
type equivGroup struct {
	members []*Symbol
	placed  bool
}

// equivItem is one item of an EQUIVALENCE list: a name, and how many bytes
// into it the item starts.
type equivItem struct {
	sym    *Symbol
	offset int
	pos    diag.Position
}

// equivalences reads the unit's EQUIVALENCE statements, once every
// declaration is known, and places the groups in common blocks. The names
// are variables from here on, so a use of one as a function is NOTARRAY.
func (a *analyzer) equivalences() {
	var groups []*equivGroup

	for _, s := range a.equivStmts {
		for _, set := range s.Sets {
			groups = a.equivalenceSet(set, groups)
		}
	}

	for _, g := range groups {
		if len(g.members) > 0 {
			a.placeInCommon(g)
		}
	}
}

// equivalenceSet merges the groups of one list's items.
func (a *analyzer) equivalenceSet(set []ast.Expr, groups []*equivGroup) []*equivGroup {
	if len(set) < 2 {
		a.report("EQUIVONE", set[0].Position())
	}

	var first *equivItem

	for _, e := range set {
		item, ok := a.equivalenceItem(e)
		if !ok {
			continue
		}

		if item.sym.group == nil {
			g := &equivGroup{members: []*Symbol{item.sym}}
			item.sym.group, item.sym.Equivalenced, item.sym.equivPos = g, true, item.pos
			groups = append(groups, g)
		}

		if first == nil {
			first = &item

			continue
		}

		a.mergeGroups(first, &item)
	}

	return groups
}

// mergeGroups makes the two items share a place.
func (a *analyzer) mergeGroups(x, y *equivItem) {
	delta := (x.sym.groupOffset + x.offset) - (y.sym.groupOffset + y.offset)

	g, from := x.sym.group, y.sym.group

	if g == from {
		if delta != 0 {
			a.report("EQUIVCONFLICT", y.pos, y.sym.Name)
		}

		return
	}

	for _, s := range from.members {
		s.group = g
		s.groupOffset += delta
	}

	g.members = append(g.members, from.members...)
	from.members = nil
}

// equivalenceItem is the name and offset of an item of a list, or false if
// it has an error, which has been reported. Section 4.6 allows variables,
// arrays, array elements, and substrings of them, every subscript and
// substring bound an integer constant expression; records and their fields
// aren't allowed, nor dummy arguments.
func (a *analyzer) equivalenceItem(e ast.Expr) (equivItem, bool) {
	switch e := e.(type) {
	case *ast.Name:
		s, ok := a.equivalenceName(e.Name, e.Pos)

		return equivItem{s, 0, e.Pos}, ok

	case *ast.Apply:
		return a.equivalenceElement(e)

	case *ast.Substring:
		return a.equivalenceSubstring(e)
	}

	a.report("EQUIVFORM", e.Position())

	return equivItem{}, false
}

// equivalenceName is the symbol of a name in a list, if it may be in one.
func (a *analyzer) equivalenceName(name string, pos diag.Position) (*Symbol, bool) {
	s := a.lookup(name, pos)

	switch {
	case s.broken:
		return s, false

	case s.Dummy != 0:
		a.report("EQUIVDUMMY", pos, name)

		s.broken = true

		return s, false

	case s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction:
		a.badUse(s, pos, "be in an EQUIVALENCE list")

		s.broken = true

		return s, false
	}

	if s.Class == Unclassified {
		s.Class = Variable
	}

	return s, true
}

// equivalenceElement is an array element's item: its offset is its place
// in storage order, by its subscripts, or (in EQUIVALENCE alone, section
// 4.6.1) by the one subscript of a multidimensional array, which is the
// element's number counting from 1: TABLE(4) of TABLE(2,2) is TABLE(2,2).
func (a *analyzer) equivalenceElement(e *ast.Apply) (equivItem, bool) {
	item := equivItem{pos: e.Pos}

	s, ok := a.equivalenceName(e.Name, e.Pos)
	if !ok {
		return item, false
	}

	item.sym = s

	if s.Class != Array {
		a.report("NOTARRAY", e.Pos, e.Name)

		return item, false
	}

	subs := make([]int, len(e.Args))

	for i, arg := range e.Args {
		if subs[i], ok = a.equivalenceConstant(arg); !ok {
			return item, false
		}
	}

	var number int

	switch {
	case len(subs) == 1 && len(s.Dims) > 1:
		number = subs[0] - 1
		ok = subs[0] >= 1 && subs[0] <= s.Elements()

	case len(subs) != len(s.Dims):
		a.report("SUBSCRIPTS", e.Pos, e.Name, count(len(s.Dims), "subscript"), len(subs))

		return item, false

	default:
		number, ok = s.ElementNumber(subs)
	}

	if !ok {
		a.report("EQUIVRANGE", e.Pos, e.Name)

		return item, false
	}

	item.offset = number * s.Type.Size

	return item, true
}

// equivalenceSubstring is a substring's item: its offset is its first
// character's place, in the variable or the array element it is part of.
func (a *analyzer) equivalenceSubstring(e *ast.Substring) (equivItem, bool) {
	var (
		item equivItem
		ok   bool
	)

	switch x := e.X.(type) {
	case *ast.Name:
		item.sym, ok = a.equivalenceName(x.Name, x.Pos)
		if ok && item.sym.Class == Array {
			a.report("NEEDSUBSCRIPT", x.Pos, x.Name)

			return item, false
		}

	case *ast.Apply:
		item, ok = a.equivalenceElement(x)

	default:
		a.report("EQUIVFORM", e.Pos)
	}

	if !ok {
		return item, false
	}

	item.pos = e.Pos

	if t := item.sym.Type; t.Kind != Character {
		a.report("EQUIVSUBSTR", e.Pos, item.sym.Name, t.String())

		return item, false
	}

	first, last := 1, item.sym.Type.Size

	if e.Lower != nil {
		if first, ok = a.equivalenceConstant(e.Lower); !ok {
			return item, false
		}
	}

	if e.Upper != nil {
		if last, ok = a.equivalenceConstant(e.Upper); !ok {
			return item, false
		}
	}

	if first < 1 || first > last || last > item.sym.Type.Size {
		a.report("EQUIVSUBRANGE", e.Pos, first, last, item.sym.Name, item.sym.Type.Size)

		return item, false
	}

	item.offset += first - 1

	return item, true
}

// equivalenceConstant is the value of a subscript or a substring bound.
func (a *analyzer) equivalenceConstant(e ast.Expr) (int, bool) {
	v, kind := a.constInt(e)

	switch kind {
	case constOK:
		return int(v), true
	case constFailed:
		return 0, false
	}

	a.report("EQUIVCONST", e.Position())

	return 0, false
}

// placeInCommon places a group with a member in a common block, and moves
// the block's end if the group runs past it. A group with none has only
// its group offsets made to start from 0, for layout to place.
func (a *analyzer) placeInCommon(g *equivGroup) {
	var anchor *Symbol

	for _, s := range g.members {
		switch {
		case s.Common == nil:
		case anchor == nil:
			anchor = s
		case s.Common != anchor.Common:
			a.report("EQUIVBLOCKS", s.equivPos, s.Name, s.Common.String(), anchor.Name, anchor.Common.String())

			return
		case s.Offset-s.groupOffset != anchor.Offset-anchor.groupOffset:
			a.report("EQUIVCOMMON", s.equivPos, s.Name, anchor.Name, s.Common.String())

			return
		}
	}

	if anchor == nil {
		lowest := g.members[0].groupOffset

		for _, s := range g.members {
			lowest = min(lowest, s.groupOffset)
		}

		for _, s := range g.members {
			s.groupOffset -= lowest
		}

		return
	}

	block, base := anchor.Common, anchor.Offset-anchor.groupOffset

	for _, s := range g.members {
		if base+s.groupOffset < 0 {
			a.report("EQUIVBEFORE", s.equivPos, s.Name, block.String())

			return
		}
	}

	for _, s := range g.members {
		s.Common, s.Offset = block, base+s.groupOffset
		block.Size = max(block.Size, s.Offset+s.Size())
	}
}

// place gives the group, none of it in a common block, its places in
// $LOCAL at or after offset, and returns where it ends. Its strictest
// member gets its natural boundary, so that member is as fast as any other
// variable; the others go where the program's lists put them, which the
// VAX doesn't mind (ours, unconfirmed).
func (g *equivGroup) place(offset int) int {
	strict := g.members[0]

	for _, s := range g.members {
		if s.Type.Align() > strict.Type.Align() {
			strict = s
		}
	}

	align := strict.Type.Align()
	base := (offset+strict.groupOffset+align-1)/align*align - strict.groupOffset
	end := offset

	for _, s := range g.members {
		s.Offset = base + s.groupOffset
		end = max(end, s.Offset+s.Size())
	}

	g.placed = true

	return end
}
