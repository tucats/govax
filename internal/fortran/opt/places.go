package opt

import (
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

	if a.Region != b.Region || a.Common != b.Common || a.Param != b.Param || a.Cell != b.Cell {
		return false
	}

	return a.Lo < b.Hi && b.Lo < a.Hi
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

	return p
}

// throughPointer is where a place reached through a computed address is, which the
// lowerer makes in a few forms: the only ones this needs to know.
func (s *Storage) throughPointer(m *ir.Mem, size int, span func(Place, int) Place) Place {
	whole := Place{Lo: wholeLo, Hi: wholeHi}

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
		// descriptor, or a record dummy's record.
		if isCharacter(ptr.Param) {
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

// isCharacter reports whether dummy p is a CHARACTER dummy, whose argument points
// at a descriptor.
func isCharacter(p *ir.Param) bool {
	return p.Sym != nil && p.Sym.Type.Kind == sema.Character
}

// isDescriptorPointer reports whether m is the address field of the descriptor
// a CHARACTER dummy's argument points at.
func isDescriptorPointer(m *ir.Mem) bool {
	ptr, ok := m.Ptr.(*ir.ArgPointer)

	return ok && isCharacter(ptr.Param) && m.Offset == ir.PointerOffset
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
