package console

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// GOSUB and RETURN, CALL, SUBROUTINE, and ENDSUBROUTINE (docs/PHASE-50 -
// DCL command procedures.md, subtask 13), as the OpenVMS User's Manual
// describes them (14.16.6 and 14.17).
//
// GOSUB label is a local subroutine call: the procedure goes on at the
// label, as for GOTO (dcllabel.go), and RETURN comes back to the command
// after the GOSUB. It makes no new command level, so the subroutine sees
// the level's labels and local symbols. GOSUBs nest at most 16 deep at a
// level. RETURN [status] gives $STATUS the status, if there is one, or
// leaves it as the subroutine left it.
//
// CALL label [p1 ... p8] runs a subroutine of the same file at a new
// command level, as @ runs a file (procedure.go): with its own local
// symbols, P1 to P8 among them, its own labels, and /OUTPUT. The
// subroutine is the lines between the label's SUBROUTINE and its
// ENDSUBROUTINE:
//
//	$ SHOW_IT: SUBROUTINE
//	$     WRITE SYS$OUTPUT P1
//	$ ENDSUBROUTINE
//	$ CALL SHOW_IT "Hello"
//
// Its level ends at the ENDSUBROUTINE (its end of file, procedureSource's
// end) or at an EXIT, and CALL's status is the subroutine's, as @'s is
// the procedure's. When DCL comes to a SUBROUTINE line by running the
// procedure's lines one by one, it passes over the subroutine to its
// ENDSUBROUTINE: a subroutine runs only when it's called (14.17.1.2). A
// CALL's target is found as GOTO's is, so a subroutine inside another,
// or inside an IF block, can be called only from within it.

// maxGosubDepth is how deeply GOSUBs nest at one command level (the
// User's Manual's 14.16.6).
const maxGosubDepth = 16

// gosubFrame is where RETURN goes back to: the record after the GOSUB's
// line, and the IF blocks that were open there.
type gosubFrame struct {
	pos    int
	blocks []ifBlock
}

// gosubCommand is GOSUB label (Console.Gosub).
func (d *Dispatcher) gosubCommand(id int64, r *dcl.Result) error {
	label, err := labelParameter(r.String("LABEL"))
	if err != nil {
		return err
	}

	return d.Console.Gosub(label)
}

// Gosub is GOSUB label: the procedure goes on at the label, as for GOTO,
// and RETURN comes back. A label that isn't there leaves the procedure
// at its end of file, with CLI$_USGOSUB ("the procedure cannot continue
// executing and is forced to exit", 14.16.6). At the terminal GOSUB is
// CLI$_INVGOSUB. GOSUB leaves $STATUS alone (unconfirmed).
func (c *Console) Gosub(label string) error {
	f := c.flow()
	if f.source == nil {
		return vmserrors.New(vmserrors.CLI_INVGOSUB)
	}

	if len(f.gosubs) >= maxGosubDepth {
		return vmserrors.New(vmserrors.CLI_GOSUBMAX)
	}

	target, err := f.findLabel(strings.ToUpper(label), vmserrors.New(vmserrors.CLI_USGOSUB))
	if err != nil {
		f.source.Seek(f.source.end)

		return err
	}

	f.gosubs = append(f.gosubs, gosubFrame{pos: f.source.Position(), blocks: slices.Clone(f.blocks)})
	f.jump(target)
	c.keepStatus()

	return nil
}

// returnCommand is RETURN [status]: the procedure goes back to the
// command after the latest GOSUB, with the IF blocks that were open
// there. With a status (a DCL expression), $STATUS is that status, and
// the level's ON action looks at it as at any command's (unconfirmed);
// without one, $STATUS is as the subroutine left it. A RETURN with no
// GOSUB to return from is CLI$_BADRET.
func (d *Dispatcher) returnCommand(id int64, r *dcl.Result) error {
	f := d.Console.flow()
	if len(f.gosubs) == 0 {
		return vmserrors.New(vmserrors.CLI_BADRET)
	}

	var status *uint32

	if text := strings.TrimSpace(r.String("STATUS")); text != "" {
		v, err := evaluateDCLExpression(text, &d.Console.dclSymbols, d.Console)
		if err != nil {
			return err
		}

		s := uint32(v.Int())
		status = &s
	}

	frame := f.gosubs[len(f.gosubs)-1]
	f.gosubs = f.gosubs[:len(f.gosubs)-1]

	f.source.Seek(frame.pos)
	f.blocks, f.skip = frame.blocks, blockSkip{}

	if status != nil {
		d.Console.setCommandStatus(*status, nil)
	} else {
		d.Console.keepStatus()
	}

	return nil
}

// callCommand is CALL[/OUTPUT=file] label [p1 ... p8]
// (Console.callSubroutine). The label and the parameters are read as @
// reads a file name and its parameters, so /OUTPUT may also follow the
// label directly.
func (d *Dispatcher) callCommand(id int64, r *dcl.Result) error {
	text := strings.TrimSpace(r.String("TEXT"))
	if text == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	cmd, err := parseProcedureCommand(text)
	if err != nil {
		return err
	}

	if r.Present("OUTPUT") {
		cmd.output, cmd.hasOutput = r.String("OUTPUT"), true
	}

	return d.Console.callSubroutine(cmd, d.Dispatch)
}

// callSubroutine runs the subroutine cmd names (cmd.file is its label)
// at a new command level, sending its lines to dispatch, and returns
// CALL's status as @'s (finishProcedure). The label must be found as
// GOTO's is, on a SUBROUTINE line; otherwise, and at the terminal, CALL
// is CLI$_USCALL, and the procedure goes on after it (unconfirmed).
func (c *Console) callSubroutine(cmd procedureCommand, dispatch func(string) error) error {
	label := strings.ToUpper(cmd.file)
	notFound := vmserrors.New(vmserrors.CLI_USCALL, label)

	f := c.flow()
	if f.source == nil {
		return notFound
	}

	if len(c.levels) >= maxCommandLevels {
		return vmserrors.New(vmserrors.CLI_STKOVF)
	}

	target, err := f.findLabel(label, notFound)
	if err != nil {
		return err
	}

	if verb, _ := flowVerb(target.rest); verb != "SUBROUTINE" {
		return notFound
	}

	_, _, _, body, _ := f.source.lineAt(target.pos)

	end, ok := f.source.subroutineEnd(body)
	if !ok {
		return vmserrors.New(vmserrors.CLI_MSNGENDS)
	}

	source := &procedureSource{name: f.source.name, records: f.source.records, next: body, end: end}

	return c.finishProcedure(c.runLevel(source, cmd, dispatch))
}

// subroutineEnd finds the ENDSUBROUTINE that ends the subroutine whose
// lines start at record pos, counting the subroutines inside it: the
// index of its line's first record, and whether there is one.
func (p *procedureSource) subroutineEnd(pos int) (int, bool) {
	depth := 0

	for {
		line, data, start, next, ok := p.lineAt(pos)
		if !ok {
			return p.end, false
		}

		pos = next

		if data {
			continue
		}

		_, afterLabel, _ := splitLabel(line)

		switch verb, _ := flowVerb(afterLabel); verb {
		case "SUBROUTINE":
			depth++
		case "ENDSUBROUTINE":
			if depth == 0 {
				return start, true
			}

			depth--
		}
	}
}

// subroutineCommand is SUBROUTINE, reached by running a procedure's
// lines one by one: DCL passes over the subroutine to the line after its
// ENDSUBROUTINE. With no ENDSUBROUTINE, the procedure is left at its end
// of file, with CLI$_MSNGENDS. At the terminal it's CLI$_INVCALL
// (unconfirmed). SUBROUTINE leaves $STATUS alone (unconfirmed).
func (d *Dispatcher) subroutineCommand(id int64, r *dcl.Result) error {
	f := d.Console.flow()
	if f.source == nil {
		return vmserrors.New(vmserrors.CLI_INVCALL)
	}

	end, ok := f.source.subroutineEnd(f.source.Position())
	if !ok {
		f.source.Seek(f.source.end)

		return vmserrors.New(vmserrors.CLI_MSNGENDS)
	}

	_, _, _, next, _ := f.source.lineAt(end)
	f.source.Seek(next)
	d.Console.keepStatus()

	return nil
}

// endsubroutineCommand is ENDSUBROUTINE reached where no subroutine
// started: a CALL's level ends before its ENDSUBROUTINE, and passing
// over a subroutine passes over its ENDSUBROUTINE, so one reached as a
// command is out of place, CLI$_INVCALL (unconfirmed).
func (d *Dispatcher) endsubroutineCommand(id int64, r *dcl.Result) error {
	return vmserrors.New(vmserrors.CLI_INVCALL)
}
