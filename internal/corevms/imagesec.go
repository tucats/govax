package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// Image sections (docs/PHASE-66 - mapped image activation.md, subtask 3).
//
// With paging on, the image activator maps an image rather than copying
// it, as VMS's does (the book's 21.1.1.3): each process-private image
// section descriptor (ISD) becomes a private section of the image file,
// its pages invalid PTEs holding the section's index, read in from the
// file when the program (or the activator's fixups) first touches them;
// each demand-zero ISD becomes demand-zero pages. Nothing is read at
// activation but the image's header.
//
// An image section differs from a $CRMPSC section in three ways:
//
//   - it never writes back to the image file. A writable section is
//     copy-on-reference (every writable section the linker makes is, and
//     one that isn't is treated as if it were): each page's first fault
//     makes it the process's own copy, backed by the page file from then
//     on, and charged against the job's paging file quota when it's
//     mapped. A read-only section's pages stay backed by the file; one
//     modified anyway (the debugger's DEPOSIT) takes the page file as
//     its backing store when it leaves the working set (evictPage's
//     leaveReadOnlySection), so the change isn't lost;
//   - a block past the file's allocation reads as zeros, as the old copy
//     filled it, rather than failing the fault (zeroPastEnd);
//   - its pages are mapped read/write for user mode until the fixups are
//     done; the activator then gives each section its own protection.
//
// Image reset needs nothing of its own: DeleteP0 releases every page,
// each section goes with its last page, and the image file with its
// last section.

// MapImageSection maps pages pages of the image file src, from block
// vbn, at addr: a process-private image section. Its pages are owned by
// user mode and read/write for it (the activator sets their protection
// after the fixups). A writable section is copy-on-reference, each page
// charged against the paging file quota as it's mapped (SS$_EXQUOTA when
// the quota runs out, the pages before it mapped); pfc is the ISD's page
// fault cluster, 0 for PFCDEFAULT. The section holds src while any of
// its pages is mapped; the activator lets its own hold go with
// ReleaseSource.
func (env *Environment) MapImageSection(src PageSource, vbn, pages, addr uint32, pfc byte, writable bool) error {
	ps := &processSection{
		file: src, vbn: vbn, first: addr &^ pageMask,
		present: make([]bool, pages), mapped: 1,
		copyOnRef: writable, pfc: uint32(pfc), image: true,
	}

	idx, ok := env.addProcessSection(ps)
	if !ok {
		return vmserrors.New(ssVasFull)
	}

	env.holdFile(src)

	var err error

	for i := range ps.present {
		page := ps.first + uint32(i)*pageSize

		pte := vm.SectionPTE(idx, vm.ProtUW, uint8(registers.User))
		if st := env.mapImagePage(page, pte, writable); st != ssNormal {
			err = vmserrors.New(st)

			break
		}

		ps.present[i] = true
		ps.mapped++
	}

	// The hold the mapping had goes; with no page mapped, so does the
	// section.
	ps.mapped--
	env.maybeDropProcessSection(idx)

	return err
}

// CreateImagePages creates pages demand-zero pages at addr: a
// demand-zero image section, read/write for user mode and charged
// against the paging file quota as $CRETVA's are (SS$_EXQUOTA when the
// quota runs out).
func (env *Environment) CreateImagePages(addr, pages uint32) error {
	pte := ProcessPTE(false, 1) // a demand-zero page, UW, owned by user mode

	for i := range pages {
		if st := env.mapImagePage(addr&^pageMask+i*pageSize, pte, true); st != ssNormal {
			return vmserrors.New(st)
		}
	}

	return nil
}

// mapImagePage gives the P0 page at addr the PTE pte, through the
// process's own page tables (it needn't be the current process), the
// page that was there released; charged, if charge is set. It returns
// the status: SS$_EXQUOTA with no quota left, SS$_VASFULL past P0's page
// table.
func (env *Environment) mapImagePage(addr uint32, pte vm.PTE, charge bool) uint32 {
	if addr >= p1Base {
		return ssVasFull
	}

	if charge && !env.canChargePage(addr) {
		return ssExQuota
	}

	// In created space (regions.go) P0 grows to cover the page first.
	if !env.coverPage(addr) {
		return ssVasFull
	}

	space := env.pageSpace()

	old, err := env.mem.LookupPTEIn(env.cpu, space, addr)
	if err != nil || env.mem.StorePTEIn(env.cpu, space, addr, pte) != nil {
		return ssVasFull
	}

	env.releasePTE(addr, old)

	if charge {
		env.chargePage(addr, true)
	}

	env.raiseP0Mark(addr)

	return ssNormal
}

// source is the page source ps's pages are read from: its file, or, for
// an image section, its file reading zeros past its allocation.
func (ps *processSection) source() PageSource {
	if ps.image {
		return zeroPastEnd{ps.file}
	}

	return ps.file
}

// zeroPastEnd is an image file as an image section reads it: a block
// past the file's allocation reads as zeros (an image whose last section
// runs past the file's end), as the image activator's copy filled it.
type zeroPastEnd struct {
	PageSource
}

// ReadPage reads block vbn into buf, zeros for a block the file doesn't
// have.
func (z zeroPastEnd) ReadPage(vbn uint32, buf []byte) error {
	err := z.PageSource.ReadPage(vbn, buf)
	if errors.Is(err, errPastSource) || errors.Is(err, rms.ErrACPEndOfFile) {
		clear(buf[:pageSize])

		return nil
	}

	return err
}

// leaveReadOnlySection gives frame pfn, whose backing store was back,
// the page file as its backing store, if it's a modified page of a
// private section that doesn't write back to its file (an image's
// read-only section, which the debugger's DEPOSIT wrote): its contents
// no longer match the file, so the modified page writer writes it to the
// page file, and once its frame is reused its PTE takes the page file
// form, its tie to the file broken as a copy-on-reference page's is.
func (env *Environment) leaveReadOnlySection(pfn paging.PFN, back paging.Backing) {
	if back.Kind != paging.SectionFile || back.Index >= uint32(len(env.procSections)) {
		return
	}

	if ps := env.procSections[back.Index]; ps == nil || ps.writeBack {
		return
	}

	_ = env.mem.Paging().SetBacking(pfn, copyOnRefBacking)
}
