package corevms

import (
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// Frame claims (docs/PHASE-62 - VMS paging.md, subtask 5).
//
// Every physical page (frame) the system gives a page comes from
// vm.Memory.AllocateFrame, with a vm.FrameClaim saying whose page it is.
// With paging off the claim changes nothing: frames are handed out
// lowest first, as they always were. With paging on (VMINIT/PAGING) the
// claim is recorded in the PFN database (internal/paging): the frame's
// owner and type, and where its contents come from (its backing store),
// so that the pager can take the frame away from its page and know how
// to get the contents back (workingset.go, Phase 63). This file makes
// sure every frame is recorded as the right page's.
//
// A frame's owner names its virtual page, and two frames of one type
// never have the same owner (paging.Database.Check):
//
//   - A process page (P0 or P1) is owned by its process's PID and the
//     page's virtual address.
//   - A global section's page is owned by the section's serial number
//     (GlobalSection.serial, given when it's made) and the page's byte
//     offset in the section: one frame however many processes map it.
//   - An S0 page, or a page whose process can't be found, is a system
//     page, tied to nothing (the claim's zero value).

// processClaim is the claim for the page at addr of env's address
// space, whose contents come from back. A nil env, or an S0 address,
// makes an untied system claim.
func processClaim(env *Environment, addr uint32, back paging.Backing) vm.FrameClaim {
	if env == nil || env.Process == nil || addr >= 0x80000000 {
		return vm.FrameClaim{}
	}

	return vm.FrameClaim{
		Tied:    true,
		Type:    paging.ProcessPage,
		Owner:   paging.Owner{Handle: env.Process.PID, VA: addr &^ pageMask},
		Backing: back,
	}
}

// faultClaim is the claim for the page fault f touched, whose contents
// come from back: a page of the process whose address space f names.
func (sys *System) faultClaim(f vm.PageFault, back paging.Backing) vm.FrameClaim {
	if f.Addr >= 0x80000000 {
		return vm.FrameClaim{}
	}

	return processClaim(sys.processForSpace(f.Space), f.Addr, back)
}

// globalClaim is the claim for page i of global section s. A file
// section's page is backed by its global page table entry; a page-file
// section's by the page file, a block to be allocated when it's first
// written (Phase 64).
func globalClaim(s *GlobalSection, i int) vm.FrameClaim {
	back := paging.Backing{Kind: paging.PageFile}
	if s.File != nil {
		back = paging.Backing{Kind: paging.GlobalSection, Index: s.gpt + uint32(i)}
	}

	return vm.FrameClaim{
		Tied:    true,
		Type:    paging.GlobalPage,
		Owner:   paging.Owner{Handle: s.serial, VA: uint32(i) * pageSize},
		Backing: back,
	}
}

// sectionBacking is the backing store of block vbn of process section
// si: the section file's block.
func sectionBacking(si, vbn uint32) paging.Backing {
	return paging.Backing{Kind: paging.SectionFile, Index: si, Block: vbn}
}

// copyOnRefBacking is the backing store of a process's private copy of a
// copy-on-reference page, of a global section or a private one: the page
// file, from its first fault (the book's 15.2.1.3), a block to be
// allocated when it's first written.
var copyOnRefBacking = paging.Backing{Kind: paging.PageFile}

// demandZeroBacking is the backing store of a demand-zero page from its
// first fault: the page file too, a block to be allocated when it's first
// written, as VMS's pager gives it (the book's 15.2.2; docs/PHASE-64 -
// page file.md, finding 3). Its contents exist only in its frame until
// then, so the frame is effectively modified (effectivelyModified).
var demandZeroBacking = paging.Backing{Kind: paging.PageFile}

// pageFileBacking is the backing store of a page whose contents are in
// block vbn of the page file.
func pageFileBacking(vbn uint32) paging.Backing {
	return paging.Backing{Kind: paging.PageFile, Block: vbn}
}
