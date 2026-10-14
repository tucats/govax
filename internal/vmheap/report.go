package vmheap

// What a zone shows of itself, for the RTL's debugging routines
// (LIB$SHOW_VM_ZONE and LIB$VERIFY_VM_ZONE, the 5.0 LIB$ manual's
// chapter 6): its areas with their free spans and unallocated ends, and
// its lookaside lists with the blocks on them. The bookkeeping is all
// in Go, so the links VMS analyzes aren't there; what the RTL checks
// against memory is the bytes of the free space, which a free fill
// should still hold, and the lists' state (a list that links a block to
// itself, FreeAgain).

// Span is a range of bytes, Start up to but not including End.
type Span struct {
	Start, End uint32
}

// Size is how many bytes s covers.
func (s Span) Size() uint32 {
	return s.End - s.Start
}

// AreaReport is one area of a zone: its pages (Base to End), the free
// spans among its blocks (sorted by address), and where its unallocated
// end begins (Top: from there to End no block has been).
type AreaReport struct {
	Base, End, Top uint32
	Free           []Span
}

// Unallocated is how many bytes of the area no block has been in yet.
func (a AreaReport) Unallocated() uint32 {
	return a.End - a.Top
}

// ListReport is one lookaside list: the class of its blocks (0 for a
// FrequentSizes list that has no size yet), the blocks on it, each as
// the bytes it occupies (tags included) and its address, and Loop, a
// block the list links to itself (FreeAgain), 0 if none: a list whose
// blocks can't be counted.
type ListReport struct {
	Size   uint32
	Blocks []Span
	Addrs  []uint32
	Loop   uint32
}

// AreaReports returns the zone's areas in the order they're searched.
func (z *Zone) AreaReports() []AreaReport {
	list := make([]AreaReport, 0, len(z.areas))

	for _, a := range z.areas {
		free := make([]Span, len(a.free))
		for i, s := range a.free {
			free[i] = Span{s.start, s.end}
		}

		list = append(list, AreaReport{Base: a.base, End: a.end, Top: a.top, Free: free})
	}

	return list
}

// ListReports returns the zone's lookaside lists in order (none for
// FirstFit), each list's blocks in the order they'd be taken from it
// (last in, first out).
func (z *Zone) ListReports() []ListReport {
	list := make([]ListReport, 0, len(z.lists))

	for _, l := range z.lists {
		r := ListReport{Size: l.n, Loop: l.loop}

		for i := len(l.addrs) - 1; i >= 0; i-- {
			b := z.blocks[l.addrs[i]]
			r.Blocks = append(r.Blocks, Span{b.start, b.end})
			r.Addrs = append(r.Addrs, l.addrs[i])
		}

		list = append(list, r)
	}

	return list
}

// Tagged reports whether the zone's blocks have boundary tags.
func (z *Zone) Tagged() bool {
	return z.tagged
}

// Quantum is what every block's size (tags included) is a multiple of,
// and Alignment what every block's address is a multiple of: without
// boundary tags, the larger of the block size and alignment for both
// (5.0 LIB$ manual, section 5.3.1).
func (z *Zone) Quantum() uint32 {
	return z.round
}

// Alignment is what every block's address is a multiple of (Quantum).
func (z *Zone) Alignment() uint32 {
	return z.align
}

// OnRelease sets the function the zone calls when free space changes:
// with the bytes a free gave back (freed) and the free span they're now
// part of (merged); and, when an allocation leaves the rest of a free
// span free, with freed empty and merged that rest. The RTL writes a
// free span's control words at its start as VMS does (its memory holds
// the links VMS keeps there), and refills what was a span's start as
// the fill.
func (z *Zone) OnRelease(f func(freed, merged Span)) {
	z.onRelease = f
}

// OnList sets the function the zone calls with a block it puts on a
// lookaside list (the bytes it occupies), whose start VMS uses for the
// list's link.
func (z *Zone) OnList(f func(Span)) {
	z.onList = f
}
