package machine

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
)

// Stubs for run-time library routines govax lacks (docs/PHASE-72 -
// legacy images.md, subtask 4).
//
// govax stands for VMS's run-time libraries (runtimeLibraries) unless a
// logical name of the library's name leads to its file. A program's G^
// cell into one of them, at an offset no shim has, used to stop
// activation as LIB$_UNRESOLVED. But an image may name far more than it
// calls: VMSRTL.EXE, the jacket a V1 image maps, has a cell for every
// routine of BASRTL, COBRTL, and CMA$TIS_SHR. So such a cell resolves to
// a stub made for it here, a CALL stub (corevms.ShimStub) whose shim
// signals SS$_UNSUPPORTED and GOVAX-E-NOTIMPL naming the library and
// the transfer-vector offset called:
//
//	%SYSTEM-E-UNSUPPORTED, unsupported operation or function
//	-GOVAX-E-NOTIMPL, routine at BASRTL+^X88 is not implemented in govax
//
// which is what a shim for it would need. The program runs up to that
// call, as with LIBRTL's missing routines (librtl's missing.go). A JSB
// entry reached this way runs the stub's entry mask as an instruction
// (HALT, a reserved instruction outside kernel mode): there's no table
// saying which entries are JSB ones.

// unresolvedCodeBase is the XFC$SHIM code of the first such stub; the
// rest follow in the order fixups reach them, past every code of
// ShimTable's and librtl's missing routines.
const unresolvedCodeBase = 10000

// unresolvedStubs are the stubs made so far for the current System,
// by SHIM$library_offset name, and the room left on their S0 page.
type unresolvedStubs struct {
	addrs     map[string]uint32
	next, end uint32
}

// unresolvedStub is the stub for the routine at offset in library, made
// on first use: its code registered with the System's shims, the stub
// written to S0 so every process reaches it at the same address.
func (m *Machine) unresolvedStub(library string, offset uint32) (uint32, error) {
	u := &m.unresolved
	name := fmt.Sprintf("SHIM$%s_%08X", library, offset)

	if addr, ok := u.addrs[name]; ok {
		return addr, nil
	}

	if u.addrs == nil {
		u.addrs = map[string]uint32{}
	}

	if u.next+corevms.ShimStubSize > u.end {
		page, err := m.processOne.AllocateS0(1, 0, "unresolved routine stubs")
		if err != nil {
			return 0, err
		}

		u.next, u.end = page, page+vaxPageSize
	}

	code := unresolvedCodeBase + uint32(len(u.addrs))
	routine := fmt.Sprintf("at %s+^X%X", library, offset)
	m.processOne.Shims().Register(code, name, librtl.NotImplemented(routine))

	addr := u.next
	if err := m.storeBytes(addr, corevms.ShimStub(code)); err != nil {
		return 0, err
	}

	u.addrs[name] = addr
	u.next += corevms.ShimStubSize

	return addr, nil
}
