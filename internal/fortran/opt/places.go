package opt

import (
	"fmt"
	"math"

	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Places: which parts of memory an operand can be, so that a pass can ask whether
// a store changes what a load reads (Decision 7 of docs/planning/PHASE-84 - FORTRAN
// optimizer.md).
//
// Names won't answer that. With
//
//	      INTEGER*2 HALF(2)
//	      EQUIVALENCE (IW, HALF)
//	      IW = 65537
//	      HALF(2) = 5
//	      K = IW
//
// IW and HALF are two cells at one offset, and a store to HALF(2) changes half of
// IW. So a Place is a region of storage and a range of bytes in it, and two places
// conflict if their ranges overlap. The same rule gives a union's fields, a
// substring, and a complex number's parts, none of which needs saying separately.
//
// A subscript that isn't constant makes the range the whole array, since which
// element it is isn't known. It is taken to stay inside the array, the FORTRAN
// standard's rule, which the manual doesn't state but /CHECK=BOUNDS enforces. The
// same goes for a substring.
//
// # Dummy arguments
//
// A dummy argument is its own region, conflicting only with itself: neither with
// another dummy nor with a variable, though the caller may have passed that very
// variable. The language forbids changing such a thing through one name and
// reading it through another, so a program that did so is wrong, and the phase's
// review accepted that it may print something different under /OPTIMIZE. That is
// worth a lot: in
//
//	      SUBROUTINE MUL(A, B, C, N)
//	      ...
//	      C(I,J) = C(I,J) + A(I,K)*B(K,J)
//
// a store to C leaves A and B alone, and nothing but this rule can say so.
//
// A character dummy has two places, its text and the descriptor that says where
// the text is. The caller made the descriptor and nothing here writes it.
//
// # Pointers
//
// A Fortran 90 pointer may point only at a variable with the TARGET attribute, a
// part of one, or another pointer's target (the standard's 7.5.2), of its own type
// (docs/planning/PHASE-107 - Fortran 90 pointers.md, finding 11). So a place
// reached through a pointer, which the lowerer marks with the type it points at
// (ir.Mem.Pointee), is a PointerRegion of that type's class, and it conflicts with
//
//   - another PointerRegion of a class that may meet it (sema.PointersMayMeet: the
//     same class, or a record holding a component of the other's);
//   - a place overlapping a TARGET of such a class: a local, frame, host, or module
//     variable, an allocatable array, or a dummy argument, and whatever shares its
//     storage by EQUIVALENCE, since its place overlaps the TARGET's (Place.Targets);
//   - anything in a common block, which another unit may declare a TARGET (the
//     author's answer 3); a module's variables are judged by their own attributes,
//     which every unit using the module sees;
//   - an UnknownRegion.
//
// And with nothing else: not a local without TARGET, not a dummy without it (a
// dummy may be a pointer's target in the caller, but the standard's 12.5.2.9
// forbids changing it through the pointer while it is the dummy), not $PDATA, not a
// register. That is what TARGET is for. In
//
//	      INTEGER, POINTER :: P
//	      DO I = 1, N
//	         P = P + K
//	      END DO
//
// K, without TARGET, stays in a register across every store through P.

// Region is the storage a place is in.
type Region int

// The regions.
const (
	// LocalRegion is the routine's $LOCAL section; the range is its offsets.
	LocalRegion Region = iota

	// FrameRegion is the frame area, holding AUTOMATIC variables.
	FrameRegion

	// CommonRegion is one common block, Place.Common.
	CommonRegion

	// DataRegion is the read-only $PDATA section, which nothing changes.
	DataRegion

	// DummyRegion is what one dummy argument, Place.Param, points at.
	DummyRegion

	// DescriptorRegion is the descriptor a CHARACTER dummy argument, Place.Param,
	// points at, which is read-only here.
	DescriptorRegion

	// DynamicRegion is the text of a dynamic string, the one the descriptor cell
	// Place.Cell describes: a concatenation's value, the compiler's own.
	DynamicRegion

	// UnknownRegion is a place reached through an address the analysis can't follow
	// (the pieces of a move of a record longer than 65,535 bytes). It may be
	// anything that can be written.
	UnknownRegion

	// RegisterRegion is a register cell, Place.Cell, which is no memory at all.
	RegisterRegion

	// AllocatedRegion is the elements of the allocatable or automatic array
	// whose descriptor is Place.Cell (docs/planning/PHASE-104 - Fortran 90
	// dynamic arrays.md): memory FTNRTL took for it, or the stack below the
	// frame, which nothing reaches but through that descriptor or an address a
	// call was given. Any call may change it, as
	// it may a dummy's (ChangedByCall).
	AllocatedRegion

	// HostRegion is, in an internal procedure, its host's $LOCAL section, where
	// the host's variables it uses are (ir/lowerinternal.go): any call may read
	// or change them, since a call of the host's other internal procedures may,
	// as a call may change COMMON. The range is the host's offsets.
	HostRegion

	// PointerRegion is what a Fortran 90 pointer to values of type Place.Class
	// points at: some TARGET, part of one, or memory ALLOCATE made, of a type
	// that may meet the class (see "Pointers" above). Any call may change it.
	PointerRegion
)

// The range of a place whose extent isn't known: everything in its region.
const (
	wholeLo = math.MinInt
	wholeHi = math.MaxInt
)

// Place is a range of bytes in one region of storage.
type Place struct {
	Region Region

	// Common, Param, and Cell say which block, dummy, or string, for the regions
	// that have several.
	Common *sema.CommonBlock
	Param  *ir.Param
	Cell   *ir.Cell

	// Lo and Hi are the range: the bytes from Lo up to but not including Hi.
	Lo, Hi int

	// Volatile says the Mem it came from is marked. Storage.Volatile also looks at
	// what the place overlaps.
	Volatile bool

	// Class is a PointerRegion's: the type of what its pointer points at.
	Class sema.Type

	// Targets are the types of the TARGETs the place overlaps, which a store
	// through a pointer of a class meeting one of them may change; nil if it
	// overlaps none. The Storage that made the place keeps one list for each
	// set of TARGETs, so that two places are equal, as a map's keys, when they
	// are the same place.
	Targets *[]sema.Type
}

// Conflict reports whether a store to one place can change what the other holds.
func Conflict(a, b Place) bool {
	// Nothing stores to read-only storage.
	if a.Region == DataRegion || b.Region == DataRegion || a.Region == DescriptorRegion || b.Region == DescriptorRegion {
		return false
	}

	if a.Region == UnknownRegion || b.Region == UnknownRegion {
		return true
	}

	if a.Region == PointerRegion {
		return pointerConflict(a, b)
	}

	if b.Region == PointerRegion {
		return pointerConflict(b, a)
	}

	if a.Region != b.Region || a.Common != b.Common || a.Param != b.Param || a.Cell != b.Cell {
		return false
	}

	return a.Lo < b.Hi && b.Lo < a.Hi
}

// pointerConflict reports whether a store through the pointer place p can change
// what q holds, or the other way round (see "Pointers" above).
func pointerConflict(p, q Place) bool {
	switch q.Region {
	case PointerRegion:
		return sema.PointersMayMeet(p.Class, q.Class)
	case CommonRegion:
		if q.Common.Module == "" {
			return true
		}
	case RegisterRegion, DynamicRegion:
		return false
	}

	if q.Targets == nil {
		return false
	}

	for _, t := range *q.Targets {
		if sema.PointersMayMeet(p.Class, t) {
			return true
		}
	}

	return false
}

// wholeSize, given as a size, says the place's extent isn't a known number of bytes.
const wholeSize = -1

// Place is where m is.
func (s *Storage) Place(m *ir.Mem) Place {
	return s.place(m, m.T.Size())
}

// TextPlace is where a CHARACTER value is, for as long as it is: the bytes of a
// constant length, or the whole of what it's in for one known only at run time.
func (s *Storage) TextPlace(t ir.Text) Place {
	if n, ok := t.Len.(*ir.Const); ok {
		return s.place(t.Mem, int(n.Int()))
	}

	return s.place(t.Mem, wholeSize)
}

// Whole is everything that can be reached from m, by a subscript or by a routine
// given its address: all of the variable or array m is in, since a routine given
// an element may use the elements after it (section 6.1.1: an array element as the
// actual argument of an array dummy).
func (s *Storage) Whole(m *ir.Mem) Place {
	return s.place(m, wholeSize)
}

// place is where size bytes at m are.
func (s *Storage) place(m *ir.Mem, size int) Place {
	exact := m.Index == nil && size != wholeSize

	span := func(p Place, base int) Place {
		if exact {
			p.Lo, p.Hi = base+m.Offset, base+m.Offset+size
		}

		return p
	}

	p := Place{Lo: wholeLo, Hi: wholeHi}

	switch {
	case m.Cell != nil && m.Cell.Register != 0:
		p.Region, p.Cell = RegisterRegion, m.Cell

	case m.Cell != nil:
		c := m.Cell
		p.Lo, p.Hi = c.Offset, c.Offset+c.Size

		switch {
		case c.Common != nil:
			p.Region, p.Common = CommonRegion, c.Common
		case s.r.Host != nil && !s.own(c):
			p.Region = HostRegion
		case c.Frame:
			p.Region = FrameRegion
		default:
			p.Region = LocalRegion
		}

		p = span(p, c.Offset)

	case m.Datum != nil:
		p.Region = DataRegion

	case m.Param != nil:
		p.Region, p.Param = DummyRegion, m.Param
		p = span(p, 0)

	default:
		p = s.throughPointer(m, size, span)
	}

	p.Volatile = p.Volatile || m.Volatile
	p.Targets = s.targetsOver(p)

	return p
}

// target is a TARGET's place and type.
type target struct {
	place Place
	t     sema.Type
}

// findTargets lists every TARGET the routine can reach but through a pointer:
// its own variables and allocatable arrays, its host's, the module variables of
// every module whose block it uses (all of them, not only those it names, since
// one it doesn't name may share another's storage by EQUIVALENCE), and its dummy
// arguments. A common block's needn't be listed: a pointer place conflicts with
// all of it.
func (s *Storage) findTargets() {
	blocks := map[*sema.CommonBlock]bool{}

	cells := func(list []*ir.Cell) {
		for _, c := range list {
			if c.Common != nil && c.Common.Module != "" {
				blocks[c.Common] = true
			}

			if c.Sym == nil || !c.Sym.Target || c.Register != 0 || c.Located() {
				continue
			}

			var p Place

			if c.Kind == ir.DescriptorCell && (c.Sym.Allocatable || c.Sym.AutomaticObject) {
				p = Place{Region: AllocatedRegion, Cell: c, Lo: wholeLo, Hi: wholeHi}
			} else {
				p = s.Whole(&ir.Mem{Cell: c, T: c.T})
			}

			s.targets = append(s.targets, target{p, c.Sym.Type})
		}
	}

	cells(s.r.Cells)

	if s.r.Host != nil {
		cells(s.r.Host.Cells)
	}

	for b := range blocks {
		for _, m := range b.Members {
			if m.Target {
				p := Place{Region: CommonRegion, Common: b, Lo: m.Offset, Hi: m.Offset + max(m.Size(), 1)}
				s.targets = append(s.targets, target{p, m.Type})
			}
		}
	}

	for _, p := range s.r.Params {
		if p.Sym != nil && p.Sym.Target {
			s.targets = append(s.targets, target{Place{Region: DummyRegion, Param: p, Lo: wholeLo, Hi: wholeHi}, p.Sym.Type})
		}
	}
}

// targetsOver are the types of the TARGETs place p overlaps, the list s keeps
// for that set of them; nil for none.
func (s *Storage) targetsOver(p Place) *[]sema.Type {
	switch p.Region {
	case LocalRegion, FrameRegion, CommonRegion, HostRegion, DummyRegion, AllocatedRegion:
	default:
		return nil
	}

	var (
		list []sema.Type
		key  []byte
	)

	for i, t := range s.targets {
		if Conflict(p, t.place) {
			list = append(list, t.t)
			key = fmt.Appendf(key, "%d,", i)
		}
	}

	if list == nil {
		return nil
	}

	if s.targetSets == nil {
		s.targetSets = map[string]*[]sema.Type{}
	}

	if kept, ok := s.targetSets[string(key)]; ok {
		return kept
	}

	s.targetSets[string(key)] = &list

	return &list
}

// throughPointer is where a place reached through a computed address is, which the
// lowerer makes in a few forms: the only ones this needs to know.
func (s *Storage) throughPointer(m *ir.Mem, size int, span func(Place, int) Place) Place {
	whole := Place{Lo: wholeLo, Hi: wholeHi}

	// A pointer's target (see "Pointers" above), unless the address has become a
	// place's own, which says more.
	if _, ok := m.Ptr.(*ir.Addr); m.Pointee != nil && !ok {
		whole.Region, whole.Class = PointerRegion, *m.Pointee

		return whole
	}

	if c := allocatedBase(m.Ptr); c != nil {
		whole.Region, whole.Cell = AllocatedRegion, c

		return whole
	}

	// An assumed-shape dummy's element, through the descriptor its entry copied
	// (ir/lowershaped.go): what its argument reaches, the caller's.
	if c := shapedBase(m.Ptr); c != nil {
		if p := s.paramNamed(c.Dummy); p != nil {
			whole.Region, whole.Param = DummyRegion, p

			return whole
		}
	}

	switch ptr := m.Ptr.(type) {
	case *ir.Addr:

		// A field of a packed record, at an address computed on the spot.
		if m.Index != nil || size == wholeSize || ptr.Mem.Index != nil {
			return s.place(ptr.Mem, wholeSize)
		}

		inner := s.place(ptr.Mem, size)
		inner.Lo += m.Offset
		inner.Hi += m.Offset

		return inner

	case *ir.ArgPointer:
		// What the argument list's longword points at: a CHARACTER dummy's
		// descriptor, or a record dummy's record. A CHARACTER pointer dummy's
		// descriptor is the caller's pointer, which the routine may change:
		// the dummy's own storage, as any other pointer dummy's is.
		if isCharacter(ptr.Param) && !ptr.Param.Sym.Pointer {
			whole.Region, whole.Param = DescriptorRegion, ptr.Param

			return whole
		}

		whole.Region, whole.Param = DummyRegion, ptr.Param

		return span(whole, 0)

	case *ir.Load:
		inner := ptr.Mem

		switch {
		// A CHARACTER dummy's text, at the address in its descriptor.
		case isDescriptorPointer(inner):
			whole.Region, whole.Param = DummyRegion, inner.Ptr.(*ir.ArgPointer).Param

			return span(whole, 0)

		case inner.Cell == nil:

		// An address the register allocator keeps: where it points says what it is.
		case inner.Cell.Points != nil:
			return s.Whole(inner.Cell.Points)

		// An adjustable array's element, from A0 in its descriptor. A0 is where an
		// element of all zero subscripts would be, so its offsets say nothing about
		// offsets elsewhere.
		case inner.Cell.Dummy != "" && inner.Offset == ir.A0Offset:
			if p := s.paramNamed(inner.Cell.Dummy); p != nil {
				whole.Region, whole.Param = DummyRegion, p

				return whole
			}

		// A dynamic string's text, at the address in its descriptor.
		case inner.Cell.Kind == ir.DescriptorCell && inner.Offset == ir.PointerOffset &&
			inner.Cell.Desc != nil && inner.Cell.Desc.Class == ir.ClassD:
			whole.Region, whole.Cell = DynamicRegion, inner.Cell

			return whole
		}
	}

	whole.Region = UnknownRegion

	return whole
}

// allocatedBase is the allocatable or automatic array's descriptor cell an element's
// address e is computed from: its A0, or its POINTER, plus whatever else
// (ir/lowerallocate.go, ir/lowerautomatic.go); nil if e isn't such an address.
// An array pointer's descriptor is laid out as an allocatable array's, but what
// it describes may be any TARGET's storage, so it isn't one: its elements are a
// PointerRegion (throughPointer, by their Mem.Pointee).
func allocatedBase(e ir.Expr) *ir.Cell {
	switch e := e.(type) {
	case *ir.Load:
		m := e.Mem
		if c := m.Cell; c != nil && m.Index == nil && c.Kind == ir.DescriptorCell && c.Sym != nil && !c.Sym.Pointer &&
			c.Desc != nil && (c.Desc.Class == ir.ClassNCA || c.Sym.AutomaticObject) &&
			(m.Offset == ir.A0Offset || m.Offset == ir.PointerOffset) {
			return c
		}

	case *ir.Op:
		if e.Op != ir.Add {
			return nil
		}

		if c := allocatedBase(e.X); c != nil {
			return c
		}

		return allocatedBase(e.Y)

	case *ir.Guard:
		// /CHECK=POINTERS's test of the descriptor's POINTER: the address is X.
		return allocatedBase(e.X)
	}

	return nil
}

// shapedBase is the assumed-shape dummy's descriptor cell an element's address
// starts from, A0 or POINTER plus strides times subscripts; nil if there is none.
func shapedBase(e ir.Expr) *ir.Cell {
	switch e := e.(type) {
	case *ir.Load:
		m := e.Mem
		if c := m.Cell; c != nil && m.Index == nil && c.Kind == ir.DescriptorCell && c.Dummy != "" &&
			c.Desc != nil && c.Desc.Class == ir.ClassNCA && (m.Offset == ir.A0Offset || m.Offset == ir.PointerOffset) {
			return c
		}

	case *ir.Op:
		if e.Op != ir.Add {
			return nil
		}

		if c := shapedBase(e.X); c != nil {
			return c
		}

		return shapedBase(e.Y)

	case *ir.Guard:
		return shapedBase(e.X)
	}

	return nil
}

// isCharacter reports whether dummy p is a CHARACTER dummy, whose argument points
// at a descriptor.
func isCharacter(p *ir.Param) bool {
	return p.Sym != nil && p.Sym.Type.Kind == sema.Character
}

// isDescriptorPointer reports whether m is the address field of the descriptor
// a CHARACTER dummy's argument points at, which points at the dummy's text. A
// CHARACTER pointer dummy's descriptor is the caller's pointer, whose target is
// anybody's (docs/planning/PHASE-107 - Fortran 90 pointers.md, finding 8).
func isDescriptorPointer(m *ir.Mem) bool {
	ptr, ok := m.Ptr.(*ir.ArgPointer)

	return ok && isCharacter(ptr.Param) && !ptr.Param.Sym.Pointer && m.Offset == ir.PointerOffset
}

// paramNamed is the routine's dummy argument of that name.
func (s *Storage) paramNamed(name string) *ir.Param {
	for _, p := range s.r.Params {
		if p.Name == name {
			return p
		}
	}

	return nil
}
