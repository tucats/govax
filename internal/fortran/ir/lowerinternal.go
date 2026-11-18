package ir

import (
	"fmt"
	"strconv"

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
// All of that is static storage, which is right while only one activation of the
// host can exist. A host in its stack frame (RECURSIVE, or with AUTOMATIC
// variables) needs the host's FP as a hidden argument, which semantic analysis
// reports isn't done yet (subtask 4).
//
// So a host's places are made before its internal procedures are lowered
// (lowerHost), each procedure's lowering finds the host's places through
// cellOf, paramOf, and adjustableOf, which note what it reached, and the host's
// own statements are lowered last, once the cells for the dummies reached are
// known (passHostArgs). A host cell an internal procedure reaches is Reached: a
// call in the host may read or change it, which the optimizer must know.

// lowerHost lowers the unit u and its internal procedures, and returns their
// routines, the unit's first.
func lowerHost(u *sema.Unit) []*Routine {
	l := newLowerer(u, nil)
	l.prepare()

	if len(u.Contains) == 0 {
		return []*Routine{l.lowerBody()}
	}

	l.reachedParams = map[*Param]bool{}
	inner := make([]*Routine, 0, len(u.Contains))

	for _, c := range u.Contains {
		il := newLowerer(c, l)
		il.prepare()
		inner = append(inner, il.lowerBody())
	}

	l.passHostArgs()

	return append([]*Routine{l.lowerBody()}, inner...)
}

// passHostArgs gives each of the host's dummy arguments that an internal
// procedure reaches a cell holding its address, which the host's entry fills
// in. A host with ENTRY statements has such a cell for every dummy already.
func (l *lowerer) passHostArgs() {
	if len(l.unit.Entries) > 0 {
		return
	}

	var args []EntryArg

	for i, p := range l.r.Params {
		if !l.reachedParams[p] {
			continue
		}

		p.Through = l.newCell("$A"+strconv.Itoa(i+1), ArgCell, Long, Long.Size())
		p.Through.Dummy = p.Name
		args = append(args, EntryArg{p.Index, p})
	}

	if len(args) > 0 {
		l.r.Entries = []*Entry{{Name: l.unit.Name, Number: 1, Args: args}}
	}
}

// cellOf is the cell of the variable or array s: the routine's own, or in an
// internal procedure its host's.
func (l *lowerer) cellOf(s *sema.Symbol) *Cell {
	c := l.cells[s]
	if c != nil || l.host == nil {
		return c
	}

	return l.host.reach(l.host.cellOf(s))
}

// adjustableOf is the descriptor cell of the adjustable array s, the
// routine's own or its host's, or nil if s isn't one.
func (l *lowerer) adjustableOf(s *sema.Symbol) *Cell {
	c := l.adjustable[s]
	if c != nil || l.host == nil {
		return c
	}

	return l.host.reach(l.host.adjustableOf(s))
}

// reach notes that an internal procedure reaches the host's cell c, which may
// be nil.
func (l *lowerer) reach(c *Cell) *Cell {
	if c == nil {
		return nil
	}

	// Semantic analysis refuses such a host (sema's hostLimits).
	if c.Frame {
		panic(fmt.Sprintf("ir: %s: an internal procedure reaches %s in its host's frame", l.r.Name, c.Name))
	}

	c.Reached = true

	return c
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
		r := l.r
		if r.Host != nil {
			r = r.Host
		}

		return r.Name + "." + s.Name, true

	case l.r.Host != nil && (s.Class == sema.UnitName || s.Result) && s.Name == l.unit.Name:
		return l.r.Name, true
	}

	return "", false
}
