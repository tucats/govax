package machine

import (
	"fmt"
	"slices"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/mthrtl"
	"github.com/tucats/govax/internal/vmserrors"
)

// ShimTable is every routine a program may link to that govax provides:
// LIBRTL.EXE's (internal/librtl), the C library's and govax's own
// (corevms.CRTLLinks), MTHRTL.EXE's (internal/mthrtl), whose stubs
// ensureShims writes, and the microkernel's (kernelLinks), which need none.
var ShimTable = slices.Concat(librtl.Links(), corevms.CRTLLinks, mthrtl.Links(), kernelLinks)

// kernelLinks are the code-0 rows (corevms.ShimLink): routines
// kernel.asm implements as native VAX code, transcribed from its second
// `.shim` table, whose SHIM$<library>_<offset> symbols are the routines'
// own addresses. They get no stub, which matters: decc$main, for one, is
// a plain label reached by JSB ("the MAIN entry initialization is via JSB
// not CALL"), not a CALLS-style .ENTRY, and so has no entry mask. A
// stub's leading 2-byte entry mask, reached by JSB, would run as an
// instruction: opcode 0x00 is HALT, a privileged-instruction fault
// outside kernel mode. Offsets are hex, as kernel.asm writes them.
var kernelLinks = []corevms.ShimLink{
	{Name: "DECC$MAIN", Library: "DECC$SHR", Offset: 0x0000},
	{Name: "DECC$EXIT", Library: "DECC$SHR", Offset: 0x0528},
	{Name: "DECC$STRLEN", Library: "DECC$SHR", Offset: 0x06E8},
	{Name: "DECC$STRCPY", Library: "DECC$SHR", Offset: 0x06D0},
	{Name: "DECC$STRCAT", Library: "DECC$SHR", Offset: 0x06B0},
	{Name: "CMA$TIS_ERRNO_GET_ADDR", Library: "CMA$TIS_SHR", Offset: 0x0038},
	{Name: "DECC$CALLOC", Library: "DECC$SHR", Offset: 0x0520},
	{Name: "DECC$$GL___CTYPEA", Library: "DECC$SHR", Offset: 0x3098},
	{Name: "DECC$$GA___CTYPET", Library: "DECC$SHR", Offset: 0x309C},
}

// shimPageBytes is the space VMInit reserves for the stubs (vminit.go): one
// page, room for 42. The SCB follows it directly, so ensureShims never
// writes past it: the stubs that don't fit go to pages of their own from
// the S0 pool (shimOverflow), as many as they need. VMInit's page isn't
// made bigger, since that would move the SCB and everything VMInit places
// after it, which listings and tests show.
const shimPageBytes = 512

// EnsureShims writes a stub (corevms.ShimStub, or JSBShimStub for a
// routine reached by JSB) for every shimTable entry that has a shim code,
// copies every data entry (a table, as LIB$AB_UPCASE) into system space,
// and resolves every code-0 entry against its already-assembled
// kernel.asm routine by name instead (kernelLinks, and LIBRTL's routines
// written in VAX code) -- either way defining that entry's
// SHIM$<library>_<offset> symbol, matching what kernel.asm's boot-time
// assembly of its own `.shim` pseudo-ops actually produces. The stubs
// fill VMInit's page first, so their addresses don't move as the table
// grows, then contiguous S0 pages allocated together, which hold the
// tables too, after the stubs. Must run after kernel.asm has been
// assembled (Console.Run's own call site does, via vax.init's boot
// sequence), since the code-0 branch depends on every routine name it
// resolves already being a defined symbol. Idempotent: a second call is a
// no-op, so fixups performed across multiple RUNs keep resolving to the
// same addresses.
func (m *Machine) EnsureShims() error {
	if m.shimsReady {
		return nil
	}

	stubs, dataBytes := 0, uint32(0)

	for _, e := range ShimTable {
		switch {
		case e.Data != nil:
			dataBytes += shimDataSize(e.Data)
		case e.Code != 0:
			stubs++
		}
	}

	stubs++ // EXE$ASTRET, after the table's (corevms.ASTExitStub)

	addr, end := m.shimBase, m.shimBase+shimPageBytes

	// The stubs that don't fit in VMInit's page, and the tables, go to an
	// overflow area big enough for all of them, allocated once.
	var overflow uint32

	rest := max(0, stubs-shimPageBytes/corevms.ShimStubSize)
	if size := uint32(rest*corevms.ShimStubSize) + dataBytes; size > 0 {
		var err error

		if overflow, err = m.shimOverflow((size + vaxPageSize - 1) / vaxPageSize); err != nil {
			return err
		}
	}

	// The tables follow the overflow area's stubs.
	data := overflow + uint32(rest*corevms.ShimStubSize)

	for _, e := range ShimTable {
		shimName := fmt.Sprintf("SHIM$%s_%08X", e.Library, e.Offset)

		if e.Data != nil {
			if err := m.storeBytes(data, e.Data); err != nil {
				return err
			}

			m.Symbols.Set(shimName, data, SymbolSystem)
			m.Symbols.Set(e.Name, data, SymbolSystem)

			data += shimDataSize(e.Data)

			continue
		}

		if e.Code == 0 {
			target, ok := m.Symbols.Get(e.Name)
			if !ok {
				return vmserrors.New(vmserrors.LIB_UNRESOLVED, e.Name)
			}

			m.Symbols.Set(shimName, target, SymbolSystem)

			continue
		}

		// VMInit's page is full: the rest go to the overflow area.
		if addr+corevms.ShimStubSize > end {
			addr, end = overflow, ^uint32(0)
		}

		stub := corevms.ShimStub(e.Code)
		if e.JSB {
			stub = corevms.JSBShimStub(e.Code)
		}

		if err := m.storeBytes(addr, stub); err != nil {
			return err
		}

		m.Symbols.Set(shimName, addr, SymbolSystem)

		// A CALL stub is a procedure (an entry mask comes first); a JSB
		// stub's first byte is an instruction.
		if e.JSB {
			m.Symbols.Set(e.Name, addr, SymbolSystem)
		} else {
			m.Symbols.SetEntry(e.Name, addr, SymbolSystem)
		}

		addr += corevms.ShimStubSize
	}

	// EXE$ASTRET, the AST exit in system space (corevms.ASTExitStub),
	// takes the slot after the table's last stub, so no stub moves.
	if addr+corevms.ASTExitStubSize > end {
		addr = overflow
	}

	if err := m.storeBytes(addr, corevms.ASTExitStub()); err != nil {
		return err
	}

	m.Symbols.Set(astExitName, addr, SymbolSystem)

	m.shimsReady = true

	return nil
}

// astExitName is the AST exit's symbol, which ASTExit looks up.
const astExitName = "EXE$ASTRET"

// ASTExit is the System's ASTExit (corevms.System): EXE$ASTRET's
// address once EnsureShims has placed it, else 0.
func (m *Machine) ASTExit() uint32 {
	if !m.shimsReady {
		return 0
	}

	addr, _ := m.Symbols.Get(astExitName)

	return addr
}

// shimDataSize is the room a data entry takes in the overflow area: its
// length, rounded up to a quadword so the next one is aligned.
func shimDataSize(data []byte) uint32 {
	return (uint32(len(data)) + 7) &^ 7
}

// shimOverflow returns pages contiguous pages for the shim stubs that don't
// fit in the page VMInit reserved: pages of the S0 pool, the system's own (PID 0),
// for the life of the machine. S0, so every process's image reaches its
// stubs at the same addresses, as it does the first page's; kernel mode
// alone may write it, and every mode may read (and so run) it.
func (m *Machine) shimOverflow(pages uint32) (uint32, error) {
	if m.processOne == nil {
		return 0, fmt.Errorf("console: no room for the shim stubs (VMINIT has not been run)")
	}

	return m.processOne.AllocateS0(pages, 0, "shim stubs")
}
