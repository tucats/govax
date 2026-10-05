package console

import (
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// SHOW CALLS as the VMS debugger shows it (docs/PHASE-41.md, subtask 12,
// Decision 6): one row per call frame, naming the module, routine, and
// line each frame's PC is in, from the loaded images' debug symbol
// tables. The probe's sessions (testdata/dbg/vax/*.dlg) show the layout:
//
//	 module name     routine name      line                rel PC           abs PC
//	*FAILSUB         SUB2                12               00000005         0000021C
//	*FAILSUB         SUB1                 7               0000000A         00000216
//	*FAILMAIN        FAILMAIN            14               00000009         00000209
//
// The "*" says the module's symbols are loaded (the debugger's SET
// MODULE); govax loads every module's, so it's always there.

// debugCallsHeading is the debugger's SHOW CALLS heading.
const debugCallsHeading = " module name     routine name      line                rel PC           abs PC"

// maxDebugFrames bounds the frames a SHOW CALLS with no count follows,
// should a corrupt stack link its frames into a loop (govax's choice).
const maxDebugFrames = 1000

// callRow is one frame's row of the debugger's SHOW CALLS.
type callRow struct {
	module, routine string
	line            int // 0 when the module has no line table
	rel             uint32
	hasRoutine      bool
	pc              uint32
}

// String lays the row out as the debugger does: the module in 16
// columns after the "*", the routine in 17, the line right-justified in
// 5 (blank without one), then the PC relative to the routine and the
// absolute PC. A name too long for its column pushes the rest along
// (unconfirmed: no probe name was that long), and with no routine the
// routine and relative PC are blank (unconfirmed too).
func (r callRow) String() string {
	line := ""
	if r.line != 0 {
		line = fmt.Sprintf("%d", r.line)
	}

	rel := "        "
	if r.hasRoutine {
		rel = fmt.Sprintf("%08X", r.rel)
	}

	return fmt.Sprintf("*%-16s%-17s%5s%15s%s%9s%08X", r.module, r.routine, line, "", rel, "", r.pc)
}

// debugCallRow is the row for a frame executing at pc, or false when no
// module of a loaded image's debug symbol table holds pc. A caller's pc
// is its frame's return address, the instruction after the CALLS or
// CALLG: its line is the call's, found from pc-1, while the relative PC
// is the return address's (FAILSUB SUB1's line 7 at 00000216, which is
// line 8's first byte).
func (c *Console) debugCallRow(pc uint32, caller bool) (callRow, bool) {
	p := c.debugImageAt(pc)
	if p == nil {
		return callRow{}, false
	}

	m, ok := p.ModuleAt(pc)
	if !ok {
		return callRow{}, false
	}

	row := callRow{module: m.Name, pc: pc}

	if r := m.RoutineAt(pc); r != nil {
		row.routine, row.rel, row.hasRoutine = r.Name, pc-r.Address, true
	}

	at := pc
	if caller {
		at--
	}

	if l, ok := m.LineAt(at); ok {
		row.line = l.Line
	}

	return row, true
}

// showDebugCalls prints SHOW CALLS as the debugger does, when the PC is
// in a module of a loaded image's debug symbol table, and reports false
// (printing nothing) when it isn't, for the console's own frame dump.
//
// The first row is the PC's; each further row is the saved PC of the
// frame before it, following the saved FPs from the current FP, as long
// as that PC is in such a module too. So, as in the debugger's sessions,
// the frames of the code that called the image (VMS's image activator,
// govax's IMAGE$INIT driver and console) aren't shown, and a JSB
// subroutine, which builds no frame, is shown by its frame's routine
// (LOCALR at DBGDIS's JSBRTN, though START called it). count bounds the
// rows; 0 is every frame.
func (c *Console) showDebugCalls(count uint32) (bool, error) {
	pc := c.CPU.GPR(vax.PC)

	row, ok := c.debugCallRow(pc, false)
	if !ok {
		return false, nil
	}

	if count == 0 {
		count = maxDebugFrames
	}

	c.Printf("%s\n", debugCallsHeading)
	c.Printf("%s\n", row)

	fp := c.CPU.GPR(vax.FP)

	for n := uint32(1); n < count && fp != 0 && fp != cpu.SentinelReturn; n++ {
		savedFP, err := c.Mem.LoadLongword(c.CPU, fp+12)
		if err != nil {
			return true, err
		}

		savedPC, err := c.Mem.LoadLongword(c.CPU, fp+16)
		if err != nil {
			return true, err
		}

		row, ok := c.debugCallRow(savedPC, true)
		if !ok {
			break
		}

		c.Printf("%s\n", row)

		// The stack grows down, so a caller's frame is above its
		// callee's: anything else is a broken chain.
		if savedFP <= fp {
			break
		}

		fp = savedFP
	}

	return true, nil
}
