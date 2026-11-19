package ir

import (
	"fmt"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Internal procedures (docs/planning/PHASE-103 - Fortran 90 procedures and
// interfaces.md, findings 4 and 5). Each is a Routine of its own, written after
// its host in the same module, named HOST.INNER, and entered at that local label
// rather than a global .ENTRY: no other module can call it, and two hosts in one
// file may each have an INNER.
//
// An internal procedure uses its host's names (sema/internal.go), and each of
// those is a place the host already has:
//
//   - a host variable or array in $LOCAL, or in a common block, is the host's
//     cell, which the procedure names as the host does (HOST.X);
//   - a host's dummy argument is reached through a cell holding its address,
//     the mechanism of ENTRY statements (lowerentry.go): the host copies the
//     address there as it starts (MOVL 4(AP),HOST.$A1), and both it and its
//     procedures reach the dummy through the cell (@HOST.$A1), since only the
//     host's own code has the host's AP;
//   - a host's adjustable array is reached through its descriptor cell ($B1),
//     which the host fills as it starts.
//
// A host whose storage is in its stack frame (a RECURSIVE one, or one with
// AUTOMATIC variables) can be active several times at once, and its internal
// procedure must reach the activation that called it. So every call of one of
// its internal procedures passes a hidden last argument: the host's FP, or in an
// internal procedure calling a sibling or itself, the FP it was given
// (Routine.HostFP, Decision 11). The procedure's code reaches a host's frame cell
// from that FP by the host's own displacement for it (HOST.X(R10), the code
// generator's hostBaseRegister): a displacement the host's code fixes only once
// its optimizer has closed its frame up, so the IR names the cell, not its offset.
// The dummies' cells of such a host are in its frame too (Routine.NewCell), each
// activation having its own.
//
// So a host's places are made before its internal procedures are lowered
// (lowerHost), each procedure's lowering finds the host's places through
// cellOf, paramOf, and adjustableOf, which note what it reached, and the host's
// own statements are lowered last, once the cells for the dummies reached are
// known (passHostArgs). A host cell an internal procedure reaches is Reached: a
// call in the host may read or change it, which the optimizer must know.

// lowerHost lowers the unit u and its internal procedures, and returns their
// routines, the unit's first; host is u's host's lowerer, or nil. A module's
// procedures are lowered as its internal procedures are, each with internal
// procedures of its own (Phase 106): a module has no frame, so they take no
// hidden argument, and each is entered at its global .ENTRY, M.P.
func lowerHost(u *sema.Unit, host *lowerer) []*Routine {
	l := newLowerer(u, host)
	l.prepare()

	if len(u.Contains) == 0 {
		return []*Routine{l.lowerBody()}
	}

	l.reachedParams = map[*Param]bool{}
	inner := make([]*Routine, 0, len(u.Contains))
	inFrame := l.r.Reentrant || l.r.FrameSize > 0

	for _, c := range u.Contains {
		if u.Kind == ast.ModuleUnit {
			inner = append(inner, lowerHost(c, l)...)

			continue
		}

		il := newLowerer(c, l)
		il.prepare()

		if inFrame {
			il.r.HostFP = &Param{Name: "$HOST", Index: len(il.r.Params) + 1, T: Long}
		}

		inner = append(inner, il.lowerBody())
	}

	l.passHostArgs()

	return append([]*Routine{l.lowerBody()}, inner...)
}

// passHostArgs gives each of the host's dummy arguments that an internal
// procedure reaches a cell holding its address, which the host's entry fills
// in. A host with ENTRY statements has such a cell for every dummy already.
func (l *lowerer) passHostArgs() {
	for i, p := range l.r.Params {
		if !l.reachedParams[p] {
			continue
		}

		// A host with ENTRY statements has a cell for every dummy, and an
		// OPTIONAL dummy has one already (prepareOptionals).
		if p.Through == nil {
			l.throughCell(i, p)
		}

		p.Through.Reached = true
	}
}

// throughCell gives the routine's dummy argument p, its ith, a cell its entry
// copies its address to ($A1, $A2, ...), for a routine without ENTRY
// statements, whose one entry point (Entries[0]) is then made if it hasn't one.
func (l *lowerer) throughCell(i int, p *Param) {
	p.Through = l.newCell("$A"+strconv.Itoa(i+1), ArgCell, Long, Long.Size())
	p.Through.Dummy = p.Name

	if len(l.r.Entries) == 0 {
		l.r.Entries = []*Entry{{Name: l.unit.Name, Number: 1}}
	}

	l.r.Entries[0].Args = append(l.r.Entries[0].Args, EntryArg{p.Index, p})
}

// cellOf is the cell of the variable or array s: the routine's own, or in an
// internal procedure its host's.
func (l *lowerer) cellOf(s *sema.Symbol) *Cell {
	c := l.cells[s]
	if c != nil || l.host == nil {
		return c
	}

	return l.reach(l.host.cellOf(s))
}

// adjustableOf is the descriptor cell of the adjustable array s, the
// routine's own or its host's, or nil if s isn't one.
func (l *lowerer) adjustableOf(s *sema.Symbol) *Cell {
	if s.Component != nil {
		return l.componentCell(s)
	}

	if c := l.dummyPointerCell(s); c != nil {
		return c
	}

	c := l.adjustable[s]
	if c != nil || l.host == nil {
		return c
	}

	return l.reach(l.host.adjustableOf(s))
}

// reach notes that this internal procedure reaches its host's cell c, which may
// be nil.
func (l *lowerer) reach(c *Cell) *Cell {
	if c == nil {
		return nil
	}

	if c.Frame {
		if l.r.HostFP == nil {
			panic(fmt.Sprintf("ir: %s: reaches %s in its host's frame with no FP of its host's", l.r.Name, c.Name))
		}

		l.r.HostFrame = true
	}

	c.Reached = true

	return c
}

// hostFrameArg is the hidden argument a call of an internal procedure passes, if
// its host's storage is in its frame: the host's FP, from the host itself or
// passed on from an internal procedure's own hidden argument. ok is false for a
// call that passes none.
func (l *lowerer) hostFrameArg(s *sema.Symbol) (arg Arg, ok bool) {
	// The callee is this routine's own internal procedure, or else one of its
	// host's (a sibling, or itself).
	host := l
	if own := s.Internal != nil && s.Internal.Host == l.unit; !own && l.host != nil {
		host = l.host
	}

	if !host.r.Reentrant && host.r.FrameSize == 0 {
		return Arg{}, false
	}

	if host == l {
		return Arg{Value: &FramePointer{}}, true
	}

	return Arg{Value: &ArgPointer{l.r.HostFP}}, true
}

// paramOf is the dummy argument s, the routine's own or in an internal
// procedure its host's, or nil if s isn't one.
func (l *lowerer) paramOf(s *sema.Symbol) *Param {
	p := l.params[s]
	if p != nil || l.host == nil {
		return p
	}

	p = l.host.paramOf(s)
	if p != nil {
		l.host.reachedParams[p] = true

		// A reentrant host's cells for its dummies are in its frame.
		if l.host.r.Reentrant {
			l.r.HostFrame = true
		}
	}

	return p
}

// internalCallee is the routine called by the name s if it is an internal
// procedure, its name HOST.INNER, and whether it is: a host's name for one of
// its internal procedures, or an internal procedure's own name in a call of
// itself.
func (l *lowerer) internalCallee(s *sema.Symbol) (string, bool) {
	switch {
	case s.Internal != nil:
		// This routine's own internal procedure, or a sibling's name of its
		// host's.
		r := l.r
		if s.Internal.Host != l.unit && r.Host != nil {
			r = r.Host
		}

		return r.Name + "." + s.Name, true

	case l.r.Host != nil && (s.Class == sema.UnitName || s.Result) && s.Name == l.unit.Name:
		return l.r.Name, true
	}

	return "", false
}
