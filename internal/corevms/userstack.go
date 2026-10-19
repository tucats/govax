package corevms

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// The user stack in created space (docs/PHASE-67 - address space under
// paging.md, subtask 3).
//
// With VMS paging on, the user stack is part of the image, as VMS's is
// (the book's 16.2.1.3 and 21.1.1.3):
//
//   - Image activation creates it (CreateUserStack): the pages just below
//     UserStackTop, as many as the image's user stack ISD says (LINK's
//     STACK= option, 20 pages by default), demand-zero, owned by user
//     mode, and charged against the paging file quota like any page
//     $CRETVA creates. P1 expansions ($EXPREG of region 1, SEC$M_EXPREG)
//     then grow directly below it.
//   - When a program needs more stack than that, it grows by itself
//     (expandStack, EXE$EXPANDSTK): a user-mode access below P1's low end
//     is a length violation, which vm hands to the System
//     (vm.Memory.SetStackExpander) before it becomes an access violation,
//     and the pages from P1's low end down to the address are created,
//     typically one at a time. As the book says of VMS, nothing limits
//     this but the page table's end (SS$_VASFULL) and the paging file
//     quota (SS$_EXQUOTA): a stray user-mode reference anywhere below the
//     stack creates pages rather than failing. When creation fails, the
//     access is the access violation it would have been. $ADJSTK of the
//     user stack expands it the same way, returning creation's status.
//   - Image rundown deletes it with the image's other P1 pages
//     (deleteImageP1), all but the part still in use (imageP1Top).
//
// A process with no image (process 1 at the console, before or between
// RUNs) has no user stack pages until it pushes something, and then
// expansion makes them.
//
// With paging off none of this applies: P1's page table is whole from the
// start, the stack is the process's, and P1 expansions begin
// userStackPages below its top.

// defaultUserStackPages is the user stack's size for an image without a
// user stack ISD: LINK's default STACK= (the book's 21.1.1.3).
const defaultUserStackPages = 20

// CreateUserStack creates the user stack for the image being activated in
// env's process, pages pages just below UserStackTop (0 for LINK's
// default), each page that doesn't exist created as $CRETVA creates one
// for user mode. A page that exists (the frames of a subprocess's CLI,
// kept through the last rundown) is left as it is. The pages are created
// from the top down, P1 lengthening as each is; a failure partway (the
// paging file quota, SS$_EXQUOTA) keeps those already created, and is
// returned. Outside created space it does nothing.
func (env *Environment) CreateUserStack(pages uint32) error {
	if !env.createdSpace() {
		return nil
	}

	if pages == 0 {
		pages = defaultUserStackPages
	}

	for n := range pages {
		addr := uint32(UserStackTop) - (n+1)*pageSize

		if env.inRegion(addr) && env.pageAt(addr) {
			continue
		}

		if st := env.createPage(addr, registers.User); st != ssNormal {
			return vmserrors.New(st)
		}
	}

	return nil
}

// expandStack is EXE$EXPANDSTK (the book's 16.2.1.3): in created space,
// it creates the pages from P1's low end down to the one holding addr, a
// page at a time, each as $CRETVA creates one for user mode, P1
// lengthening as each is. It returns SS$_NORMAL when the page at addr
// exists afterward (an address already inside P1, or outside P1
// altogether, needs nothing), or creation's status where it stopped (the
// pages created before it are kept). Outside created space, where P1's
// table is whole, there's nothing to create.
func (env *Environment) expandStack(addr uint32) uint32 {
	if addr < p1Base || addr >= s0Base || !env.createdSpace() {
		return ssNormal
	}

	target := addr &^ pageMask

	// The table area must hold the whole expansion, or nothing is created
	// (spaceHolds).
	if !env.spaceHolds(pageRange{first: target, last: env.p1Low() - pageSize}) {
		return ssVasFull
	}

	for low := env.p1Low(); low > target; low -= pageSize {
		if st := env.createPage(low-pageSize, registers.User); st != ssNormal {
			return st
		}
	}

	return ssNormal
}

// expandUserStack is the System's stack expander (vm.Memory's
// SetStackExpander): a user-mode access to addr below P1's low end in the
// address space space, the current process's. It reports whether the
// process's stack now reaches addr.
func (sys *System) expandUserStack(space vm.AddressSpace, addr uint32) bool {
	env := sys.processForSpace(space)
	if env == nil || !env.createdSpace() {
		return false
	}

	return env.expandStack(addr) == ssNormal
}

// imageP1Top is where image rundown's deletion of P1 stops in created
// space (deleteImageP1): UserStackTop, or, when the user stack pointer is
// below it, the page that pointer is in, which with those above it holds
// what called the image (a subprocess's CLI stub's frames). At the
// console, process 1's user stack pointer is back at UserStackTop when an
// image ends, so the whole stack goes, as on VMS.
func (env *Environment) imageP1Top() uint32 {
	usp := env.userStackPointer()
	if usp >= p1Base && usp < UserStackTop {
		return max(usp&^pageMask, env.p1Low())
	}

	return UserStackTop
}

// userStackPointer is the user stack pointer of env's process: SP when
// the process is current and running in user mode (not on the interrupt
// stack), its USP register when it's current in another mode, and its
// hardware PCB's copy when it isn't current.
func (env *Environment) userStackPointer() uint32 {
	c := env.cpu

	if env.Space == nil || vm.CurrentAddressSpace(c).Same(env.Space.AddressSpace) {
		if psl := c.PSL(); psl.CurMod() == registers.User && !psl.IS() {
			return c.GPR(registers.SP)
		}

		return c.PR(registers.USP)
	}

	if env.Stacks == nil || env.Stacks.PCBB == 0 {
		return UserStackTop
	}

	pcb, err := cpu.ReadPCB(env.mem, env.Stacks.PCBB)
	if err != nil {
		return UserStackTop
	}

	return pcb.SP[registers.User]
}
