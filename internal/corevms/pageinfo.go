package corevms

import (
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// What a page's mapping is, for the debugger's SHOW IMAGE/FULL: a
// read-only look at a page's PTE, the frame behind it, and the private
// section it belongs to. Nothing here faults a page in or changes any
// state.

// PageForm is the form a page's PTE is in.
type PageForm uint8

const (
	// PageValid: the page is in memory, in the process's working set (or,
	// with paging off, simply in memory).
	PageValid PageForm = iota

	// PageTransition: the page left the working set but its frame still
	// holds it, on the free or modified list; a fault takes it back
	// without reading anything (a soft fault).
	PageTransition

	// PageInSection: the page has never been read in (or was dropped
	// clean): its PTE holds a process section table index, and a fault
	// reads it from the section's file.
	PageInSection

	// PageInPageFile: the page was written to the page file and its frame
	// reused; a fault reads it back from there.
	PageInPageFile

	// PageDemandZero: the page has never been touched; a fault gives it a
	// frame of zeros.
	PageDemandZero

	// PageGlobal: the PTE holds a global page table index.
	PageGlobal
)

func (f PageForm) String() string {
	switch f {
	case PageValid:
		return "valid"
	case PageTransition:
		return "in transition"
	case PageInSection:
		return "not yet read"
	case PageInPageFile:
		return "in the page file"
	case PageDemandZero:
		return "demand zero"
	case PageGlobal:
		return "global"
	}

	return "unknown"
}

// PageInfo describes one page's mapping.
type PageInfo struct {
	// Form is the PTE's form, and PTE the PTE itself.
	Form PageForm
	PTE  vm.PTE

	// Frame is the page's frame, for a valid page or one in transition.
	Frame uint32

	// List is the list a page in transition's frame is on (paging.Free or
	// paging.Modified); Active for a valid page.
	List paging.State

	// Backing is where the frame's contents go back to when it's reused,
	// for a valid page or one in transition, with paging on. HasBacking
	// is false with paging off, when a frame has no backing store: it
	// never leaves memory.
	Backing    paging.Backing
	HasBacking bool
}

// PageInfoAt describes the mapping of the process's page at addr,
// through its own page tables (it needn't be the current process).
func (env *Environment) PageInfoAt(addr uint32) (PageInfo, error) {
	pte, err := env.mem.LookupPTEIn(env.cpu, env.pageSpace(), addr)
	if err != nil {
		return PageInfo{}, err
	}

	info := PageInfo{PTE: pte, List: paging.Active}

	if pte.Valid() {
		info.Form, info.Frame = PageValid, pte.PFN()
	} else if pfn, ok := pte.Transition(); ok {
		info.Form, info.Frame = PageTransition, pfn
	} else {
		switch _, isSection := pte.SectionIndex(); {
		case isSection:
			info.Form = PageInSection
		case isPageFilePTE(pte):
			info.Form = PageInPageFile
		case isGlobalPTE(pte):
			info.Form = PageGlobal
		default:
			info.Form = PageDemandZero
		}

		return info, nil
	}

	if db := env.mem.Paging(); db != nil {
		if e, err := db.Entry(paging.PFN(info.Frame)); err == nil {
			info.List, info.Backing, info.HasBacking = e.State, e.Backing, true
		}
	}

	return info, nil
}

func isPageFilePTE(pte vm.PTE) bool {
	_, ok := pte.PageFileBlock()

	return ok
}

func isGlobalPTE(pte vm.PTE) bool {
	_, ok := pte.GlobalIndex()

	return ok
}

// SectionInfo describes one of the process's private sections.
type SectionInfo struct {
	// Index is its process section table index, the one its pages' PTEs
	// hold.
	Index uint32

	// First is the address of its first page, Pages how many pages it
	// spans, and Mapped how many of them are still mapped to it (a
	// page deleted or mapped over is no longer).
	First  uint32
	Pages  int
	Mapped int

	// VBN is the block of its file that holds its first page.
	VBN uint32

	// Image is set for an image section the image activator mapped;
	// CopyOnRef for a section whose pages become the process's own,
	// backed by the page file, at their first fault; WriteBack for one
	// whose modified pages go back to its file.
	Image, CopyOnRef, WriteBack bool

	// PFC is its read cluster size, 0 for PFCDEFAULT.
	PFC uint32
}

// SectionAt returns the private section whose pages span addr, if any.
func (env *Environment) SectionAt(addr uint32) (SectionInfo, bool) {
	for i, ps := range env.procSections {
		if ps == nil || addr < ps.first || addr >= ps.first+uint32(len(ps.present))*pageSize {
			continue
		}

		mapped := 0

		for _, p := range ps.present {
			if p {
				mapped++
			}
		}

		return SectionInfo{
			Index: uint32(i), First: ps.first, Pages: len(ps.present), Mapped: mapped,
			VBN: ps.vbn, Image: ps.image, CopyOnRef: ps.copyOnRef, WriteBack: ps.writeBack,
			PFC: ps.pfc,
		}, true
	}

	return SectionInfo{}, false
}
