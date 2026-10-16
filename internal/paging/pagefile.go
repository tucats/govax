package paging

import (
	"fmt"
	"math/bits"
)

// The page file's allocation map (docs/PHASE-64 - page file.md, the
// survey's finding 7).
//
// VMS's page file holds the contents of pages that have no other home:
// demand-zero pages a program has written, and its private copies of
// copy-on-reference section pages. When the modified page writer needs
// to write such a page out to free its frame, it takes blocks of the
// page file for it; when the page is deleted (or faulted back and
// written again, so the copy in the file is stale) the blocks go back.
// One block holds one page: both are 512 bytes.
//
// *VAX/VMS Internals and Data Structures*, 14.5 and 15.5.2.4, describes
// how VMS keeps track: a bitmap with one bit per block, and an
// *allocation size*. The writer asks for a run of contiguous blocks, up
// to MPW_WRTCLUSTER of them, so that one write can take a whole cluster
// of pages. The map looks for a free run of the current allocation size,
// scanning from the file's start; when there's none, the size drops by
// 16 and the scan starts again. Freeing blocks that leave a run large
// enough lets the size grow back by 8, up to MPW_WRTCLUSTER. At 16 or
// less, the map takes the first free run it finds, however short. When
// the file first passes 65% and 90% full, VMS tells the operator
// (SYSTEM-W-PAGEFRAG and PAGECRIT); the map reports that to its caller.
//
// Like the PFN database, the map is kept in Go, not in the file, and
// starts empty each session: nothing in a page file outlives the system
// that wrote it (the phase plan's Decision 9). Blocks are numbered from
// 1, as a file's virtual blocks are, so block n is the file's VBN n, and
// a page file PTE whose block number is 0 can still mean "no block".

// minAllocationSize is the allocation size at which the map stops
// looking for a run of the size and takes the first free run it finds:
// the book's special case.
const minAllocationSize = 16

// allocationDrop and allocationRegrow are how far the allocation size
// falls when no run of it is free, and how far it climbs back when
// freeing leaves one.
const (
	allocationDrop   = 16
	allocationRegrow = 8
)

// Warning is a page file use threshold the map has just passed for the
// first time.
type Warning uint8

const (
	// NoWarning: no new threshold passed.
	NoWarning Warning = iota

	// PageFrag: more than 65% of the file is in use (SYSTEM-W-PAGEFRAG).
	PageFrag

	// PageCrit: more than 90% of the file is in use (SYSTEM-W-PAGECRIT).
	PageCrit
)

func (w Warning) String() string {
	switch w {
	case NoWarning:
		return "none"
	case PageFrag:
		return "PAGEFRAG"
	case PageCrit:
		return "PAGECRIT"
	}

	return fmt.Sprintf("Warning(%d)", uint8(w))
}

// Run is a run of contiguous page file blocks: Count blocks from First.
type Run struct {
	First, Count uint32
}

// Last is the run's last block.
func (r Run) Last() uint32 { return r.First + r.Count - 1 }

// PageFileMap is the allocation map of one page file of Size blocks.
type PageFileMap struct {
	size uint32

	// inUse has one bit per block, block n at bit n-1, set while the
	// block is allocated. (VMS's bitmap sets a bit for a free block; here
	// the zero value is an empty file.)
	inUse []uint64

	// free counts the blocks not in use.
	free uint32

	// cluster is the allocation size's ceiling, MPW_WRTCLUSTER, and
	// allocSize the current allocation size (PFL$L_ALLOCSIZ).
	cluster, allocSize uint32

	// warned is the highest threshold reported so far: each is reported
	// once.
	warned Warning
}

// NewPageFileMap returns the map of an empty page file of size blocks,
// whose writer asks for runs of at most cluster blocks (MPW_WRTCLUSTER).
// A cluster under 16 is taken as 16.
func NewPageFileMap(size, cluster uint32) *PageFileMap {
	cluster = max(cluster, minAllocationSize)

	return &PageFileMap{
		size:      size,
		inUse:     make([]uint64, (uint64(size)+63)/64),
		free:      size,
		cluster:   cluster,
		allocSize: cluster,
	}
}

// Size is the file's size in blocks.
func (m *PageFileMap) Size() uint32 { return m.size }

// FreeBlocks is how many blocks are free, and UsedBlocks how many are
// allocated.
func (m *PageFileMap) FreeBlocks() uint32 { return m.free }

// UsedBlocks is how many blocks are allocated.
func (m *PageFileMap) UsedBlocks() uint32 { return m.size - m.free }

// AllocationSize is the size of run the map looks for now.
func (m *PageFileMap) AllocationSize() uint32 { return m.allocSize }

// Allocated reports whether block vbn is allocated. A block outside the
// file isn't.
func (m *PageFileMap) Allocated(vbn uint32) bool {
	if vbn == 0 || vbn > m.size {
		return false
	}

	return m.test(vbn - 1)
}

// test, set, and clear work on the bit of the block whose index (its
// number less 1) is i.
func (m *PageFileMap) test(i uint32) bool { return m.inUse[i/64]&(1<<(i%64)) != 0 }
func (m *PageFileMap) set(i uint32)       { m.inUse[i/64] |= 1 << (i % 64) }
func (m *PageFileMap) clear(i uint32)     { m.inUse[i/64] &^= 1 << (i % 64) }

// Allocate takes a run of contiguous free blocks for want pages (want is
// at least 1): a run of want blocks, or of the allocation size if that's
// smaller, found by the book's rule (the comment above). The run may be
// shorter than asked for when space is fragmented; it's never longer.
// The Warning is a threshold the allocation passed for the first time.
// It reports false, allocating nothing, when the file is full.
func (m *PageFileMap) Allocate(want uint32) (Run, Warning, bool) {
	if want == 0 || m.free == 0 {
		return Run{}, NoWarning, false
	}

	for {
		target := min(want, m.allocSize)

		if target <= minAllocationSize {
			// The special case: the first free run, however short.
			first, length := m.findRun(1)

			return m.take(Run{First: first, Count: min(length, target)}), m.warning(), true
		}

		if first, _ := m.findRun(target); first != 0 {
			return m.take(Run{First: first, Count: target}), m.warning(), true
		}

		// No run that long: look for shorter ones from now on.
		m.allocSize = max(m.allocSize-min(m.allocSize, allocationDrop), minAllocationSize)
	}
}

// findRun returns the first run of at least n free blocks, scanning from
// the file's start: its first block and its whole length. It returns 0
// for the first block when there's none.
func (m *PageFileMap) findRun(n uint32) (uint32, uint32) {
	var start, length uint32

	for i := uint32(0); i < m.size; {
		// A word whose blocks are all in use ends any run and is passed
		// over whole.
		if i%64 == 0 && m.inUse[i/64] == ^uint64(0) {
			if length >= n {
				return start + 1, length
			}

			length = 0
			i += 64

			continue
		}

		if m.test(i) {
			if length >= n {
				return start + 1, length
			}

			length = 0
		} else {
			if length == 0 {
				start = i
			}

			length++
		}

		i++
	}

	if length >= n {
		return start + 1, length
	}

	return 0, 0
}

// take marks run's blocks in use.
func (m *PageFileMap) take(run Run) Run {
	for b := run.First; b <= run.Last(); b++ {
		m.set(b - 1)
	}

	m.free -= run.Count

	return run
}

// warning returns the highest threshold the file's use has passed that
// hasn't been reported yet, and notes it reported.
func (m *PageFileMap) warning() Warning {
	used := uint64(m.UsedBlocks()) * 100

	w := NoWarning

	switch {
	case used > uint64(m.size)*90:
		w = PageCrit
	case used > uint64(m.size)*65:
		w = PageFrag
	}

	if w <= m.warned {
		return NoWarning
	}

	m.warned = w

	return w
}

// Free gives run's blocks back. Freeing a block that isn't allocated, or
// isn't in the file, is an error, and frees nothing. If the free run the
// blocks are now part of is long enough, the allocation size grows back
// by 8, up to the cluster size.
func (m *PageFileMap) Free(run Run) error {
	if run.Count == 0 {
		return nil
	}

	if run.First == 0 || uint64(run.First)+uint64(run.Count)-1 > uint64(m.size) {
		return fmt.Errorf("paging: freeing page file blocks %d to %d: the file has %d", run.First, run.Last(), m.size)
	}

	for b := run.First; b <= run.Last(); b++ {
		if !m.test(b - 1) {
			return fmt.Errorf("paging: freeing page file block %d, which isn't allocated", b)
		}
	}

	for b := run.First; b <= run.Last(); b++ {
		m.clear(b - 1)
	}

	m.free += run.Count

	if m.allocSize < m.cluster {
		grown := min(m.allocSize+allocationRegrow, m.cluster)
		if m.runAround(run) >= grown {
			m.allocSize = grown
		}
	}

	return nil
}

// runAround is the length of the free run that holds the free blocks of
// run.
func (m *PageFileMap) runAround(run Run) uint32 {
	length := run.Count

	for i := run.First - 1; i > 0 && !m.test(i-1); i-- {
		length++
	}

	for i := run.Last(); i < m.size && !m.test(i); i++ {
		length++
	}

	return length
}

// Check verifies the map's rules: the free count matches the bits, no
// bit is set past the file's end, and the allocation size is between 16
// and the cluster size. It returns the first rule broken, or nil.
func (m *PageFileMap) Check() error {
	var used uint32

	for w, word := range m.inUse {
		if extra := uint64(w+1)*64 - uint64(m.size); extra > 0 && extra < 64 && word>>(64-extra) != 0 {
			return fmt.Errorf("paging: page file map has blocks in use past its %d blocks", m.size)
		}

		used += uint32(bits.OnesCount64(word))
	}

	if used+m.free != m.size {
		return fmt.Errorf("paging: page file map has %d blocks in use and %d free, of %d", used, m.free, m.size)
	}

	if m.allocSize < minAllocationSize || m.allocSize > m.cluster {
		return fmt.Errorf("paging: page file allocation size %d isn't between %d and %d",
			m.allocSize, minAllocationSize, m.cluster)
	}

	return nil
}
