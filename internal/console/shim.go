package console

import (
	"fmt"
	"slices"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/vmserrors"
)

// shimTable is every routine a program may link to that govax provides:
// LIBRTL.EXE's (internal/librtl), the C library's and govax's own
// (corevms.CRTLLinks), whose stubs ensureShims writes, and the
// microkernel's (kernelLinks), which need none.
var shimTable = slices.Concat(librtl.Links(), corevms.CRTLLinks, kernelLinks)

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
// writes past it: the stubs that don't fit go to a page of their own from
// the S0 pool (shimOverflow). VMInit's page isn't made bigger, since
// that would move the SCB and everything VMInit places after it, which
// listings and tests show.
const shimPageBytes = 512

// ensureShims writes a stub (corevms.ShimStub) for every shimTable entry
// that has a shim code, and resolves every code-0 entry against its
// already-assembled kernel.asm routine by name instead (kernelLinks) --
// either way defining that entry's SHIM$<library>_
// <offset> symbol, matching what kernel.asm's boot-time assembly of its own
// `.shim` pseudo-ops actually produces. Must run after kernel.asm has been
// assembled (Console.Run's own call site does, via vax.init's boot
// sequence), since the code-0 branch depends on every routine name it
// resolves already being a defined symbol. Idempotent: a second call is a
// no-op, so fixups performed across multiple RUNs keep resolving to the
// same addresses.
func (c *Console) ensureShims() error {
	if c.shimsReady {
		return nil
	}

	addr, end := c.shimBase, c.shimBase+shimPageBytes
	overflowed := false

	for _, e := range shimTable {
		shimName := fmt.Sprintf("SHIM$%s_%08X", e.Library, e.Offset)

		if e.Code == 0 {
			target, ok := c.Symbols.Get(e.Name)
			if !ok {
				return vmserrors.New(vmserrors.LIB_UNRESOLVED, e.Name)
			}

			c.Symbols.Set(shimName, target, SymbolSystem)

			continue
		}

		// VMInit's page is full: the rest go to an overflow page (once).
		if addr+corevms.ShimStubSize > end {
			if overflowed {
				return fmt.Errorf("console: too many shim stubs for VMINIT's page and one overflow page (internal/console/shim.go)")
			}

			page, err := c.shimOverflow()
			if err != nil {
				return err
			}

			addr, end, overflowed = page, page+512, true
		}

		if err := c.storeBytes(addr, corevms.ShimStub(e.Code)); err != nil {
			return err
		}

		c.Symbols.Set(shimName, addr, SymbolSystem)
		c.Symbols.SetEntry(e.Name, addr, SymbolSystem)

		addr += corevms.ShimStubSize
	}

	c.shimsReady = true

	return nil
}

// shimOverflow returns a page for the shim stubs that don't fit in the
// page VMInit reserved: a page of the S0 pool, the system's own (PID 0),
// for the life of the machine. S0, so every process's image reaches its
// stubs at the same addresses, as it does the first page's; kernel mode
// alone may write it, and every mode may read (and so run) it.
func (c *Console) shimOverflow() (uint32, error) {
	if c.RTL == nil {
		return 0, fmt.Errorf("console: no room for the shim stubs (VMINIT has not been run)")
	}

	return c.RTL.AllocateS0(1, 0, "shim stubs")
}
