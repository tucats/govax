package debugger

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET TRACE, SHOW TRACE, and CANCEL TRACE as the VMS debugger has them
// (docs/PHASE-42.md, subtask 13; the formats are from
// testdata/dbgcmd/vax/trace.dlg).
//
// A *tracepoint* is a breakpoint that doesn't stop the program: when the
// program reaches it, the debugger says so and the program goes on. SET
// TRACE takes the same places as SET BREAK (an address or routine, or a
// class of instruction with /CALL, /BRANCH, /LINE, /INSTRUCTION, or
// /RETURN), and the same /AFTER, /TEMPORARY, WHEN, and DO. They are kept
// in Debugger.Tracepoints as Breakpoint values, reached by the same rules
// (eventpoint.go's triggers), so only what happens next differs. The
// commands are breakcmd.go's, run on the tracepoint list.

// bindTrace binds the SET TRACE, SHOW TRACE, and CANCEL TRACE commands.
func (d *Dispatcher) bindTrace() {
	d.Grammar.Bind("SET_TRACE", func(id int64, r *dcl.Result) error { return d.Debugger.setTrace(r) })
	d.Grammar.Bind("SHOW_TRACE", func(id int64, r *dcl.Result) error { return d.Debugger.showTrace() })
	d.Grammar.Bind("CANCEL_TRACE", func(id int64, r *dcl.Result) error { return d.Debugger.cancelTrace(r) })
}

// traceHit reports each tracepoint the instruction about to run at pc
// reaches, in the VMS debugger's words:
//
//	trace at routine DBGCMD\FACT
//	trace on lines at DBGCMD\START\%LINE 39
//
// each followed by the source line (shown once if several tracepoints are
// reached at one pc; see showSource). The program is not stopped.
//
// When more than one tracepoint is reached at a pc, they are reported in
// the order of Tracepoints, and each one reported then moves to the end of
// the list. The VMS debugger's order was found to follow that rule: with
// /LINE then /BRANCH set, the first pc that is both reports the line, then
// the branch; but after a pc that is only a line start has reported the
// line, the next pc that is both reports the branch first, then the line
// (the probe's trace.dlg).
func (d *Debugger) traceHit(pc uint32) {
	if len(d.Tracepoints) == 0 {
		return
	}

	peek := d.instructionPeeker()

	var fired []*Breakpoint

	// Work from a copy: a temporary tracepoint removes itself as it fires,
	// and a DO command may change the list.
	for _, tp := range slices.Clone(d.Tracepoints) {
		if d.triggers(tp, pc, peek) {
			fired = append(fired, tp)
		}
	}

	if len(fired) == 0 {
		return
	}

	// Move what fired to the end of the list, in the order it fired, and
	// drop the temporary ones.
	d.Tracepoints = slices.DeleteFunc(d.Tracepoints, func(tp *Breakpoint) bool { return slices.Contains(fired, tp) })

	for _, tp := range fired {
		if !tp.Temporary {
			d.Tracepoints = append(d.Tracepoints, tp)
		}
	}

	for _, tp := range fired {
		d.Console.Printf("%s\n", d.traceMessage(tp, pc))
		d.showSource(pc)

		d.runCommands(tp.Do)
	}
}

// traceMessage is what the debugger says when tp is reached at pc: the
// words of the breakpoint's stop message with "trace" for "break".
func (d *Debugger) traceMessage(tp *Breakpoint, pc uint32) string {
	return "trace" + strings.TrimPrefix(d.stopMessage(tp, pc), "break")
}

// showTrace implements SHOW TRACE: each tracepoint in the layout of SHOW
// BREAK, or the message that none are set.
func (d *Debugger) showTrace() error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	if len(d.Tracepoints) == 0 {
		d.Console.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOTRACES))

		return nil
	}

	for _, tp := range d.Tracepoints {
		d.showPoint(tp, "tracepoint")
	}

	return nil
}

// cancelTrace implements CANCEL TRACE: of the tracepoints at the given
// addresses, of a class (/CALL, /BRANCH, /LINE, /INSTRUCTION, /RETURN), or
// all of them (/ALL). Cancelling nothing that was set is %DEBUG-I-NOTRACES
// (unconfirmed: the probe cancelled only what was set).
func (d *Debugger) cancelTrace(r *dcl.Result) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	before := len(d.Tracepoints)

	kind, err := class(r, "ALL")
	if err != nil {
		return err
	}

	switch kind {
	case "ALL":
		d.Tracepoints = nil

	case "":
		err = d.cancelAddressBreaks(&d.Tracepoints, r.String("TARGET"))

	default:
		err = d.cancelClassBreak(&d.Tracepoints, kind, r)
	}

	if err != nil {
		return err
	}

	if len(d.Tracepoints) == before {
		d.Console.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOTRACES))
	}

	return nil
}
