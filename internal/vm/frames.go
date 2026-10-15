package vm

import "github.com/tucats/govax/internal/paging"

// Frame allocation (docs/PHASE-62 - VMS paging.md, subtask 4).
//
// # Who hands out physical pages
//
// A page frame (a "frame") is one 512-byte page of physical memory,
// named by its page frame number (PFN). Every time a virtual page needs
// a physical page behind it, something must choose a frame nobody is
// using, and every time a page goes away its frame must be given back.
// That something is a FrameAllocator, and a Memory has one of two:
//
//   - With VMS paging off (the default, and govax's behavior before
//     paging existed), bitmapFrames: one bool per frame, the lowest free
//     frame handed out first. A frame given back is cleared then and
//     there, so every frame handed out reads as zeros.
//   - With VMS paging on (VMINIT/PAGING, SetPaging), pagingFrames: the
//     PFN database of internal/paging, which records who each frame
//     belongs to and keeps the free and modified page lists VMS keeps.
//     A frame given back keeps its contents (on VMS a page on the free
//     list can be faulted back without reading it again), so a frame is
//     cleared when it's handed out instead, unless the caller is about to
//     fill all of it anyway (FrameClaim.Fill).
//
// Either way, frame 0 is never handed out: VMINIT always maps it as the
// first page of S0.

// FrameClaim describes the page a frame is wanted for. The zero value is
// an untied frame: a system page that is never paged, which is all a
// claim can say with paging off (bitmapFrames ignores the rest).
type FrameClaim struct {
	// Tied says the frame holds a virtual page that may be paged: Owner
	// names it, Type is its kind, and Backing is where its contents come
	// from and go back to (internal/paging's Allocate). An untied claim
	// is a system page (its Type is ignored).
	Tied    bool
	Type    paging.Type
	Owner   paging.Owner
	Backing paging.Backing

	// Fill says the caller writes all 512 bytes of the frame itself (it
	// reads the page from a file, or copies another page), so it needn't
	// be cleared first. With paging off frames are always handed out
	// clear, and Fill changes nothing.
	Fill bool
}

// FrameAllocator hands out and takes back physical page frames. Memory
// uses one of the two in this file; nothing else implements it.
type FrameAllocator interface {
	// Reserve claims frame pfn itself, for a page VMINIT lays out. A
	// frame already reserved stays reserved.
	Reserve(pfn uint32)

	// Allocate claims a free frame for the page c describes, or reports
	// false if there is none.
	Allocate(c FrameClaim) (uint32, bool)

	// Free gives frame pfn back: its page is gone.
	Free(pfn uint32)

	// Claimed is how many frames, frame 0 apart, are claimed.
	Claimed() int
}

// bitmapFrames is the allocator with paging off: claimed or not, per
// frame, and the lowest free frame first.
type bitmapFrames struct {
	claimed []bool
}

func newBitmapFrames(frames uint32) *bitmapFrames {
	return &bitmapFrames{claimed: make([]bool, frames)}
}

func (b *bitmapFrames) Reserve(pfn uint32) {
	if int(pfn) < len(b.claimed) {
		b.claimed[pfn] = true
	}
}

// Allocate's search starts at frame 1: frame 0 is always VMINIT's.
func (b *bitmapFrames) Allocate(FrameClaim) (uint32, bool) {
	for n := 1; n < len(b.claimed); n++ {
		if !b.claimed[n] {
			b.claimed[n] = true

			return uint32(n), true
		}
	}

	return 0, false
}

func (b *bitmapFrames) Free(pfn uint32) {
	if pfn != 0 && int(pfn) < len(b.claimed) {
		b.claimed[pfn] = false
	}
}

func (b *bitmapFrames) Claimed() int {
	count := 0

	for n := 1; n < len(b.claimed); n++ {
		if b.claimed[n] {
			count++
		}
	}

	return count
}

// pagingFrames is the allocator with paging on: internal/paging's PFN
// database. Frame 0 is reserved when it's made, so the free list never
// offers it.
type pagingFrames struct {
	db *paging.Database
}

func newPagingFrames(frames uint32) *pagingFrames {
	p := &pagingFrames{db: paging.New(int(frames))}
	if frames > 0 {
		_ = p.db.Reserve(0)
	}

	return p
}

// Reserve leaves a frame that's already a system page alone, as the
// bitmap does; any other frame not free can't be reserved, and stays as
// it is.
func (p *pagingFrames) Reserve(pfn uint32) {
	e, err := p.db.Entry(paging.PFN(pfn))
	if err != nil || (e.State == paging.Active && e.Type == paging.SystemPage && !e.Tied) {
		return
	}

	_ = p.db.Reserve(paging.PFN(pfn))
}

func (p *pagingFrames) Allocate(c FrameClaim) (uint32, bool) {
	var (
		pfn paging.PFN
		err error
	)

	if c.Tied {
		pfn, err = p.db.Allocate(c.Type, c.Owner, c.Backing)
	} else {
		pfn, err = p.db.AllocateSystem()
	}

	if err != nil {
		return 0, false
	}

	return uint32(pfn), true
}

// Free deletes the frame's page: the frame goes to the head of the free
// list, to be reused first. Frame 0 and a frame beyond memory are
// ignored, as the bitmap ignores them.
func (p *pagingFrames) Free(pfn uint32) {
	if pfn != 0 {
		_ = p.db.Delete(paging.PFN(pfn))
	}
}

func (p *pagingFrames) Claimed() int {
	return p.db.Counts().Active() - 1 // frame 0, always reserved
}

// SetPaging chooses the frame allocator: the PFN database with on, the
// bitmap without. It starts afresh, every frame but 0 free, so VMINIT
// calls it before it reserves S0's frames, on a Memory it has just made.
// With paging on it also starts keeping the set of frames translated
// since the last instruction boundary (BeginInstruction).
func (m *Memory) SetPaging(on bool) {
	frames := uint32(len(m.ram)) / pageSize

	m.pagingOn = on

	if on {
		m.frames = newPagingFrames(frames)
		m.touched = make([]uint32, frames)
		m.touchGen = 1
	} else {
		m.frames = newBitmapFrames(frames)
		m.touched = nil
	}
}

// Paging returns the PFN database when paging is on, or nil when it's
// off.
func (m *Memory) Paging() *paging.Database {
	if p, ok := m.frames.(*pagingFrames); ok {
		return p.db
	}

	return nil
}

// AllocateFrame claims a free frame for the page c describes, cleared
// unless c.Fill says the caller fills it, or reports false if physical
// memory is exhausted.
func (m *Memory) AllocateFrame(c FrameClaim) (uint32, bool) {
	pfn, ok := m.frames.Allocate(c)
	if !ok {
		return 0, false
	}

	// With paging off, a frame was cleared when it was freed. With it
	// on, a freed frame kept its contents (it may be faulted back), so
	// it's cleared now.
	if m.pagingOn && !c.Fill {
		m.clearFrame(pfn)
	}

	return pfn, true
}

// clearFrame writes zeros over frame pfn.
func (m *Memory) clearFrame(pfn uint32) {
	if b, err := m.phys(pfn<<9, pageSize); err == nil {
		clear(b)
	}
}

// The translated-frames set (the phase doc's Decision 6, "Instruction
// safety"). One instruction may touch several pages, and Go code may
// translate two addresses before copying from one to the other. When the
// pager takes a page away from a process to make room for another, it
// must not take one of those: the instruction would fault again on the
// page it just lost, perhaps forever, or the Go code would copy through
// a physical address whose frame now holds something else. So, with
// paging on, Memory remembers which frames have been translated since
// the instruction began: touched[pfn] holds the instruction generation
// in which frame pfn was last translated, and touchGen is the current
// generation, which BeginInstruction moves on. A full page table walk
// and a TB hit record the frame. A hit in the STC doesn't (that path is
// kept as cheap as possible), so the STC's frame counts as translated
// whatever its generation; it's always the most recent translation. The
// instruction-fetch window's frame does too.

// BeginInstruction marks an instruction boundary: no frame has been
// translated in the new instruction yet. The CPU calls it before each
// instruction. With paging off it does nothing.
//
// It's kept small enough for the Go compiler to inline into the CPU's
// loop, so with paging off it costs one comparison.
func (m *Memory) BeginInstruction() {
	if m.touched != nil {
		m.nextGeneration()
	}
}

// nextGeneration starts a new instruction generation.
func (m *Memory) nextGeneration() {
	m.touchGen++
	if m.touchGen == 0 {
		// The generation wrapped around after 2^32 instructions: start
		// over, so an old frame's generation can't match a new one.
		clear(m.touched)
		m.touchGen = 1
	}
}

// noteTranslated records that the frame holding physical address paddr
// was translated in the current instruction.
func (m *Memory) noteTranslated(paddr uint32) {
	if pfn := paddr >> 9; int(pfn) < len(m.touched) {
		m.touched[pfn] = m.touchGen
	}
}

// TranslatedThisInstruction reports whether frame pfn has been
// translated since the last instruction boundary (BeginInstruction), or
// is the frame behind the STC or the instruction-fetch window: a frame
// the pager must not take away now. With paging off it reports false.
func (m *Memory) TranslatedThisInstruction(pfn uint32) bool {
	if m.touched == nil || int(pfn) >= len(m.touched) {
		return false
	}

	if m.touched[pfn] == m.touchGen {
		return true
	}

	if m.tb.stcGrant != grantNone && m.tb.stcPPage>>9 == pfn {
		return true
	}

	return m.tb.fetch.limit != 0 && m.tb.fetch.pbase>>9 == pfn
}
