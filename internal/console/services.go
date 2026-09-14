package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rtl"
)

// This file makes Console implement cpu.SystemServices (internal/cpu/
// services.go), the hook Engine's XFC opcode handler delegates to. SYS$/
// LIB$ dispatch is delegated straight to RTL (Phase 10); console I/O and
// command dispatch are handled directly since Console already owns them.
//
// See services.go's own doc comment on Console.Dispatcher for why
// XFC$CONSOLE_CMD can report "unavailable" even on a fully initialized
// Console, and DCLGetString/DCLPresent/DCLGetKeyword/DCLGetInteger's own
// comments for why the DCL-parse-callback subfunctions (XFC$DCL) are
// deliberately minimal stubs.

var _ cpu.SystemServices = (*Console)(nil)

// ConsoleWriteByte is XFC$CONSOLE_WRITE.
func (c *Console) ConsoleWriteByte(b byte) {
	if c.Out != nil {
		_, _ = c.Out.Write([]byte{b})
	}
}

// ConsoleReadByte is XFC$CONSOLE_READ. Reads one byte from In, or reports 0
// if no input source is configured or it's exhausted — matching a real
// getchar() at EOF returning a sentinel the C source doesn't itself check
// for either.
func (c *Console) ConsoleReadByte() byte {
	if c.In == nil {
		return 0
	}
	var buf [1]byte
	if _, err := c.In.Read(buf[:]); err != nil {
		return 0
	}
	return buf[0]
}

// ConsoleCommand is XFC$CONSOLE_CMD: dispatches cmd as a console command
// line. Returns 0 on success, 1 on any error (a Dispatch failure or no
// Dispatcher configured yet) — console_dispatch's own real VAX_xxx status
// codes have no exact Go equivalent here since Dispatch reports failure as
// a Go error, not a status code; callers needing more than "did it work"
// have Console's own richer error already available through normal Go
// channels wherever Dispatch is called directly.
func (c *Console) ConsoleCommand(cmdLine string) uint32 {
	if c.Dispatcher == nil {
		return 1
	}
	if err := c.Dispatcher.Dispatch(cmdLine); err != nil {
		return 1
	}
	return 0
}

// DCLPresent/DCLGetKeyword/DCLGetString/DCLGetInteger implement XFC$DCL's
// four subfunctions — emul_xfc.c's own callback into dclrtl.c's
// DCLpresent/DCLgetkeyword/DCLgetstring/DCLgetinteger, used by compiled
// VAX code (chiefly the microkernel's own kernel.asm bootstrap, assembled
// at vax_init time — see docs/PHASE-13.md) to ask the console's DCL parser
// whether a qualifier was present on some other, already-parsed command
// line and what value it carried.
//
// Phase 08's DCL engine (internal/console/dcl) parses one complete command
// line against the grammar as a single pass — it has no API for "re-open
// an already-parsed line and ask about one specific qualifier by name from
// outside," which is what these four callbacks need. Building one with no
// real caller yet to validate it against (nothing reaches XFC$DCL until
// Phase 13's kernel.asm bootstrap actually runs) would be guessing at an
// interface this port can't exercise; deliberately stubbed as "not
// present"/zero until a concrete need shows up.
func (c *Console) DCLPresent(r1, r2 uint32) uint32        { return 0 }
func (c *Console) DCLGetKeyword(r1, r2, r3 uint32) uint32 { return 0 }
func (c *Console) DCLGetInteger(r1, r2 uint32) uint32     { return 0 }

// DCLGetString reports ok=false unconditionally, for the same reason as its
// three siblings above — plus, even with real qualifier-lookup logic, this
// port has nowhere to write the result: the real callback stores into a
// fixed scratch buffer named by the "EXE$DCLSTRING" symbol, which only
// exists once Phase 13's kernel.asm bootstrap defines it.
func (c *Console) DCLGetString(r1, r2 uint32) (uint32, bool) { return 0, false }

// SystemService delegates to RTL (Phase 10's SYS$ dispatch).
func (c *Console) SystemService(pc uint32) (uint32, bool, error) {
	r0, handled, err := c.RTL.SystemService(pc)
	return r0, handled, translateHalt(err)
}

// Shim delegates to RTL (Phase 10's LIB$/CRTL shim dispatch).
func (c *Console) Shim(code uint32) (uint32, bool, error) {
	r0, handled, err := c.RTL.Shim(code)
	return r0, handled, translateHalt(err)
}

// translateHalt turns rtl.ErrHalt (a SYS$ service or shim requesting the
// machine halt, e.g. an unrecognized SYS$CLI request) into cpu.ErrHalted,
// the sentinel Engine.Step actually recognizes — kept as a translation at
// the boundary rather than internal/rtl importing internal/cpu, so that
// package has no dependency on this one.
func translateHalt(err error) error {
	if errors.Is(err, rtl.ErrHalt) {
		return cpu.ErrHalted
	}
	return err
}
