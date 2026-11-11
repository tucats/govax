package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// COMMON (section 4.2). A common block is storage program units share by
// name: each unit that declares /BLK/ reaches the same bytes, whatever
// names and types it gives them. The block is a program section (Decision
// 10 of docs/planning/PHASE-74 - FORTRAN plan.md), overlaid, so that every unit's
// contribution starts at its base, and LINK makes it as long as the
// longest. A unit's names are laid out in the order its COMMON statements
// list them, with nothing between them: the VAX reads unaligned data, and
// aligning would move the names of one unit away from those of another.
// /ALIGN asks for just that (align.go), name by name, by the setting where each
// COMMON statement is.
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

	// Attrs are the attributes of its program section, as this unit's CDEC$
	// PSECT directives leave them (directive.go). psectPos is where the first
	// of them names the block, and psectBad is set if one had an error.
	Attrs    PsectAttrs
	psectPos diag.Position
	psectBad bool

	// alignGiven is set if a directive gave ALIGN=.
	alignGiven bool

	// LayoutAlign is the alignment of the program section, as a power of two,
	// that this unit's layout needs for its offsets to be addresses' (align.go);
	// 0 if it needs none.
	LayoutAlign int
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
	a.noteOther(whatCommon, name, block.Pos, RefDefined)

	for _, b := range a.unit.Commons {
		if b.Name == name {
			return b
		}
	}

	b := &CommonBlock{Name: name, Psect: name, NamePos: block.Pos, Attrs: PsectAttrs{Align: DefaultAlign}}
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
	sym := a.declare(d.Name, d.Pos)

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
	sym.commonAlign, sym.commonPos = a.align, d.Pos
	b.Members = append(b.Members, sym)

	if sym.Class == Unclassified {
		sym.Class = Variable
	}

	if d.Dims != nil {
		a.dimension(sym, d)
	}
}

// layoutCommon gives each block's names their places, one after another,
// each on its boundary if the setting at its COMMON statement says so, and
// each block its size. A block with an aligned name needs a program section
// aligned at least as strictly, or an offset's alignment wouldn't be an
// address's.
func (a *analyzer) layoutCommon() {
	for _, b := range a.unit.Commons {
		needs := 1

		for _, s := range b.Members {
			natural := s.boundary()

			boundary := 1

			switch s.commonAlign.Commons {
			case Natural:
				boundary = natural
			case Standard:
				boundary = min(natural, 4)
			}

			needs = max(needs, boundary)
			s.Offset = alignUp(b.Size, boundary)
			b.Size = s.Offset + s.Size()

			if s.commonAlign.Warn && s.Offset%natural != 0 {
				a.report("ALIGNCOMMON", s.commonPos, s.Name, s.Offset, b.String(), natural)
			}
		}

		b.LayoutAlign = alignPower(needs)

		if b.alignGiven && !b.psectBad && b.Attrs.Align < b.LayoutAlign {
			a.report("PSECTSMALL", b.psectPos, b.String(), b.LayoutAlign, b.Attrs.Align)
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
