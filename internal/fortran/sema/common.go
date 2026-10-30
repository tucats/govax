package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// COMMON (section 4.2). A common block is storage program units share by
// name: each unit that declares /BLK/ reaches the same bytes, whatever
// names and types it gives them. The block is a program section (Decision
// 10 of docs/PHASE-74 - FORTRAN plan.md), overlaid, so that every unit's
// contribution starts at its base, and LINK makes it as long as the
// longest. A unit's names are laid out in the order its COMMON statements
// list them, with nothing between them: the VAX reads unaligned data, and
// aligning would move the names of one unit away from those of another.
//
//	COMMON HEAT,X /BLK1/KILO,Q         HEAT $BLANK+0, X $BLANK+4,
//	                                   KILO BLK1+0, Q BLK1+4
//
// A block named again continues: COMMON /A/ X followed by COMMON /A/ Y
// puts Y after X.

// blankPsect is the program section of blank common, the block with no
// name (Decision 10; the manual doesn't name it).
const blankPsect = "$BLANK"

// CommonBlock is one unit's view of a common block.
type CommonBlock struct {
	// Name is the block's name, "" for blank common; Psect is the name of
	// its program section.
	Name  string
	Psect string

	// Members are the names the unit's COMMON statements put in the
	// block, in storage order. A name EQUIVALENCE brought in isn't one,
	// though its Common says it is in the block.
	Members []*Symbol

	// Size is the block's size in this unit, in bytes, with what
	// EQUIVALENCE added at its end (section 4.6.3).
	Size int

	// NamePos is where the unit first names the block.
	NamePos diag.Position
}

// String is the block as messages and the storage map say it: /BLK1/, or
// blank common.
func (b *CommonBlock) String() string {
	if b.Name == "" {
		return "blank common"
	}

	return "/" + b.Name + "/"
}

// commonBlock returns the unit's block named name, making it the first
// time.
func (a *analyzer) commonBlock(name string, block *ast.CommonBlock) *CommonBlock {
	for _, b := range a.unit.Commons {
		if b.Name == name {
			return b
		}
	}

	b := &CommonBlock{Name: name, Psect: name, NamePos: block.Pos}
	if name == "" {
		b.Psect = blankPsect
	}

	a.unit.Commons = append(a.unit.Commons, b)

	return b
}

// commonStatement puts each name a COMMON statement lists in its block.
// Their places wait until every declaration is read (layoutCommon), since
// a name's type and dimensions may come after it.
func (a *analyzer) commonStatement(s *ast.CommonStmt) {
	for _, block := range s.Blocks {
		b := a.commonBlock(block.Name, block)

		for _, d := range block.Entities {
			a.commonEntity(b, d)
		}
	}
}

// commonEntity puts the name d declares in the block b, with its
// dimensions if it has an array declarator. A dummy argument's storage is
// its caller's, so it can't be in a block (the standard's rule, which the
// manual doesn't state), and a name can be in only one block, once.
func (a *analyzer) commonEntity(b *CommonBlock, d *ast.Declarator) {
	sym := a.lookup(d.Name, d.Pos)

	switch {
	case sym.Dummy != 0:
		a.report("COMMONDUMMY", d.Pos, d.Name)

		sym.broken = true

		return

	case sym.Class == UnitName || sym.Result || sym.Class == Constant || sym.Class == StmtFunction ||
		sym.External || sym.Class == Intrinsic:
		a.badUse(sym, d.Pos, "be in a common block")

		sym.broken = true

		return

	case sym.Common != nil:
		a.report("DUPCOMMON", d.Pos, d.Name, sym.Common.String())

		sym.broken = true

		return
	}

	sym.Common = b
	b.Members = append(b.Members, sym)

	if sym.Class == Unclassified {
		sym.Class = Variable
	}

	if d.Dims != nil {
		a.dimension(sym, d)
	}
}

// layoutCommon gives each block's names their places, one after another,
// and each block its size.
func (a *analyzer) layoutCommon() {
	for _, b := range a.unit.Commons {
		for _, s := range b.Members {
			s.Offset = b.Size
			b.Size += s.Size()
		}
	}
}

// checkCommonSizes reports a block too large for the VAX to address, once
// EQUIVALENCE has had its say.
func (a *analyzer) checkCommonSizes() {
	for _, b := range a.unit.Commons {
		if b.Size > maxArrayBytes {
			a.report("COMMONSIZE", b.NamePos, b.String())
		}
	}
}
