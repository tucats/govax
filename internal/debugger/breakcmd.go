package debugger

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET BREAK, SHOW BREAK, and CANCEL BREAK as the VMS debugger has them
// (docs/PHASE-42.md, subtask 6; the formats are from
// testdata/dbgcmd/vax/break.dlg and brkcls.dlg). govax adds one
// qualifier, /FAULT, for its own fault breakpoints (faultbreak.go).
//
// The command's parameter is the rest of the line, which this file takes
// apart itself, since what it holds is more than one expression:
//
//	SET BREAK address [, address...] [WHEN (condition)] [DO (commands)]

// classQualifiers are the qualifiers that choose a *kind* of breakpoint
// instead of an address, in the order the debugger checks them.
var classQualifiers = []string{"CALL", "BRANCH", "LINE", "INSTRUCTION", "RETURN", "EXCEPTION", "FAULT"}

// bindBreak binds the SET BREAK, SHOW BREAK, and CANCEL BREAK commands.
func (d *Dispatcher) bindBreak() {
	d.Grammar.Bind("SET_BREAK", func(id int64, r *dcl.Result) error { return d.Debugger.setBreak(r) })
	d.Grammar.Bind("SHOW_BREAK", func(id int64, r *dcl.Result) error { return d.Debugger.ShowBreakpoints() })
	d.Grammar.Bind("CANCEL_BREAK", func(id int64, r *dcl.Result) error { return d.Debugger.cancelBreak(r) })
}

// pointList returns the list a command works on: the tracepoints when trace
// is true, else the breakpoints. Tracepoints are Breakpoint values too
// (they are reached the same way), kept in their own list; what differs is
// what happens when one is reached (tracepoint.go).
func (d *Debugger) pointList(trace bool) *[]*Breakpoint {
	if trace {
		return &d.Tracepoints
	}

	return &d.Breakpoints
}

// splitClauses separates a SET BREAK parameter into what comes before its
// WHEN and DO clauses (the addresses or the routine), and those clauses'
// text, parentheses and all. The clauses may come in either order. A
// clause that isn't in parentheses is a syntax error.
func splitClauses(text string) (target, when, do string, err error) {
	upper := strings.ToUpper(text)

	whenAt := findWord(upper, "WHEN", 0)
	doAt := findWord(upper, "DO", 0)

	// The target runs up to the first clause.
	end := len(text)

	for _, at := range []int{whenAt, doAt} {
		if at >= 0 && at < end {
			end = at
		}
	}

	target = strings.TrimSpace(text[:end])

	clause := func(at int, word string) (string, error) {
		if at < 0 {
			return "", nil
		}

		stop := len(text)

		for _, other := range []int{whenAt, doAt} {
			if other > at && other < stop {
				stop = other
			}
		}

		body := strings.TrimSpace(text[at+len(word) : stop])
		if !strings.HasPrefix(body, "(") || matchingParen(body, 0) != len(body)-1 {
			return "", vmserrors.New(vmserrors.DBG_SYNTAX, firstWordOf(body))
		}

		return body, nil
	}

	if when, err = clause(whenAt, "WHEN"); err != nil {
		return "", "", "", err
	}

	if do, err = clause(doAt, "DO"); err != nil {
		return "", "", "", err
	}

	return target, when, do, nil
}

// firstWordOf is the first blank-delimited word of text, for a syntax
// error's "at or near".
func firstWordOf(text string) string {
	if f := strings.Fields(text); len(f) > 0 {
		return strings.ToUpper(f[0])
	}

	return ""
}

// class returns which class qualifier the command named, if any, and an
// error if it named two (they are alternatives). extra names further
// qualifiers that exclude the class ones (CANCEL BREAK's /ALL).
//
// A qualifier the grammar gives a default value (so that /INSTRUCTION
// alone is allowed) is present in every result, so a defaulted one, which
// the command line didn't type, doesn't count. The grammar's own DISALLOW
// can't tell the difference, so the check is made here.
func class(r *dcl.Result, extra ...string) (string, error) {
	var found []string

	for _, q := range append(slices.Clone(classQualifiers), extra...) {
		if r.Present(q) && !r.Defaulted(q) {
			found = append(found, q)
		}
	}

	switch len(found) {
	case 0:
		return "", nil

	case 1:
		return found[0], nil
	}

	return "", vmserrors.New(vmserrors.CLI_BADQUALIFIERCOMBO, found[0], found[1])
}

// setBreak implements SET BREAK.
func (d *Debugger) setBreak(r *dcl.Result) error { return d.setPoint(r, false) }

// setTrace implements SET TRACE: the same command as SET BREAK (except for
// /EXCEPTION and /FAULT, which the grammar leaves out), making tracepoints.
func (d *Debugger) setTrace(r *dcl.Result) error { return d.setPoint(r, true) }

// setPoint is SET BREAK, or SET TRACE when trace is true.
func (d *Debugger) setPoint(r *dcl.Result, trace bool) error {
	list := d.pointList(trace)

	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	target, when, do, err := splitClauses(r.String("TARGET"))
	if err != nil {
		return err
	}

	proto := Breakpoint{
		Temporary: r.Present("TEMPORARY"),
		When:      when,
		Do:        do,
	}

	if r.Present("AFTER") {
		if proto.After = int(r.Int("AFTER")); proto.After < 1 {
			return vmserrors.New(vmserrors.DBG_SYNTAX, fmt.Sprint(r.Int("AFTER")))
		}
	}

	kind, err := class(r)
	if err != nil {
		return err
	}

	// Only an address breakpoint and /RETURN take a parameter.
	if target != "" && kind != "" && kind != "RETURN" {
		return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, target)
	}

	switch kind {
	case "":
		return d.setAddressBreaks(list, proto, target)

	case "RETURN":
		return d.setReturnBreak(list, proto, target)

	case "FAULT":
		return d.AddFaultBreakpoint(r.String("FAULT"))

	case "INSTRUCTION":
		return d.setInstructionBreak(list, proto, r.List("INSTRUCTION"))
	}

	proto.Kind = map[string]BreakKind{
		"CALL": BreakCall, "BRANCH": BreakBranch, "LINE": BreakLine, "EXCEPTION": BreakException,
	}[kind]

	d.replaceClassBreak(list, &proto)

	return nil
}

// setAddressBreaks makes an address breakpoint for each address in the
// list. An address that is a routine's entry gets its breakpoint just past
// the entry mask, shown as "at routine NAME".
func (d *Debugger) setAddressBreaks(list *[]*Breakpoint, proto Breakpoint, target string) error {
	if target == "" {
		return vmserrors.New(vmserrors.CLI_NEEDBREAKADDR)
	}

	// Evaluate every address before making any breakpoint, so a bad one
	// in the list makes none.
	var made []*Breakpoint

	for _, text := range splitTop(target, ',') {
		addr, err := d.evalWhole(text)
		if err != nil {
			return err
		}

		bp := proto
		bp.Kind = BreakAddress
		bp.Addr = addr

		if d.Console.RoutineEntry(addr) {
			bp.Routine = true
			bp.Name = d.Console.LocationText(addr)
			bp.Addr = addr + 2
		}

		made = append(made, &bp)
	}

	// A list typed in one command is listed in reverse (SET BREAK LOOP, ODD
	// shows ODD first: break.dlg), while the commands themselves follow one
	// another.
	slices.Reverse(made)

	for _, bp := range made {
		// A place has one eventpoint of the user's: a breakpoint, or a
		// tracepoint, replaces the other of the same address, as the VMS
		// debugger's SET BREAK replaced an earlier SET TRACE (the probe's
		// trace.dlg).
		for _, other := range []*[]*Breakpoint{&d.Breakpoints, &d.Tracepoints} {
			if other != list {
				*other = slices.DeleteFunc(*other, func(old *Breakpoint) bool {
					return old.Kind == BreakAddress && old.Addr == bp.Addr && !old.Step && !old.Quiet
				})
			}
		}

		// A breakpoint already at this address is replaced, so the new
		// options (/AFTER, WHEN) take its place. The debugger's own step
		// breakpoints are left alone: they coexist with a user's.
		*list = slices.DeleteFunc(*list, func(old *Breakpoint) bool {
			return old.Kind == BreakAddress && old.Addr == bp.Addr && !old.Step && !old.Quiet
		})

		*list = append(*list, bp)
	}

	return nil
}

// setReturnBreak makes a SET BREAK/RETURN breakpoint: it stops at each RET
// that ends a call of the named routine.
func (d *Debugger) setReturnBreak(list *[]*Breakpoint, proto Breakpoint, target string) error {
	if target == "" {
		return vmserrors.New(vmserrors.CLI_NEEDBREAKADDR)
	}

	addr, err := d.evalWhole(target)
	if err != nil {
		return err
	}

	start, size, ok := d.Console.RoutineExtent(addr)
	if !ok {
		return vmserrors.New(vmserrors.DBG_SYNTAX, strings.ToUpper(target))
	}

	proto.Kind = BreakReturn
	proto.Start, proto.Size = start, size
	proto.Name = d.Console.LocationText(start)

	d.replaceClassBreak(list, &proto)

	return nil
}

// setInstructionBreak makes a SET BREAK/INSTRUCTION breakpoint: with
// opcodes, it stops before each of those instructions; with none, before
// every instruction.
func (d *Debugger) setInstructionBreak(list *[]*Breakpoint, proto Breakpoint, names []string) error {
	names = slices.DeleteFunc(slices.Clone(names), func(s string) bool { return strings.TrimSpace(s) == "" })

	proto.Kind = BreakAnyInstruction

	if len(names) > 0 {
		proto.Kind = BreakInstruction

		for _, name := range names {
			name = strings.ToUpper(strings.TrimSpace(name))

			inst := lookupInstruction(name)
			if inst == nil {
				return vmserrors.New(vmserrors.CLI_BADOPCODE, name)
			}

			proto.Ops = append(proto.Ops, inst)
			proto.OpNames = append(proto.OpNames, name)
		}
	}

	d.replaceClassBreak(list, &proto)

	return nil
}

// replaceClassBreak adds a breakpoint of a class kind (calls, branches,
// lines, instructions, a routine's returns, exceptions), replacing the one
// of the same class already set: a class has one breakpoint, as in the
// VMS debugger.
func (d *Debugger) replaceClassBreak(list *[]*Breakpoint, bp *Breakpoint) {
	*list = slices.DeleteFunc(*list, func(old *Breakpoint) bool {
		if bp.Kind == BreakReturn {
			return old.Kind == BreakReturn && old.Start == bp.Start
		}

		// /INSTRUCTION with opcodes and without are the same class.
		instruction := func(k BreakKind) bool { return k == BreakInstruction || k == BreakAnyInstruction }

		return old.Kind == bp.Kind || instruction(old.Kind) && instruction(bp.Kind)
	})

	*list = append(*list, bp)
}

// ShowBreakpoints implements SHOW BREAK: every breakpoint, in the order
// they were set (the VMS debugger's own order isn't one the probe could
// explain, so govax's is unconfirmed), then govax's fault breakpoints.
func (d *Debugger) ShowBreakpoints() error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	c := d.Console
	faults := c.Engine.FaultBreakpoints()
	shown := 0

	for _, bp := range d.Breakpoints {
		// The debugger's own breakpoints (a STEP's, the start of an
		// image) aren't the user's.
		if bp.Step || bp.Quiet {
			continue
		}

		shown++

		d.showPoint(bp, "breakpoint")
	}

	for _, code := range faults {
		shown++

		c.Printf("breakpoint on fault %02X %s\n", uint8(code), c.ExceptionName(code))
	}

	if shown == 0 {
		c.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOBREAKS))
	}

	return nil
}

// showPoint prints one SHOW BREAK or SHOW TRACE entry: its description (word
// is "breakpoint" or "tracepoint"), the opcodes of an instruction class, and
// the /AFTER, WHEN, and DO clauses.
func (d *Debugger) showPoint(bp *Breakpoint, word string) {
	c := d.Console

	c.Printf("%s\n", d.breakDescription(bp, word))

	if bp.Kind == BreakCall || bp.Kind == BreakBranch || bp.Kind == BreakInstruction {
		for _, line := range opcodeLines(d.breakOpNames(bp)) {
			c.Printf("%s\n", line)
		}
	}

	if bp.After > 0 {
		c.Printf("   /after: %d\n", bp.After)
	}

	if bp.When != "" {
		c.Printf("   when %s\n", bp.When)
	}

	if bp.Do != "" {
		c.Printf("   do %s\n", bp.Do)
	}
}

// breakDescription is the first line SHOW BREAK shows for bp, or SHOW TRACE,
// whose word for it ("breakpoint" or "tracepoint") it takes.
func (d *Debugger) breakDescription(bp *Breakpoint, word string) string {
	var text string

	switch bp.Kind {
	case BreakCall:
		text = word + " on calls:"

	case BreakBranch:
		text = word + " on branches:"

	case BreakLine:
		text = word + " on lines"

	case BreakInstruction:
		// VMS ends this line with a blank before the opcodes' lines.
		text = word + " on instruction(s): "

	case BreakAnyInstruction:
		text = word + " on instructions"

	case BreakReturn:
		text = word + " on return from routine " + bp.Name

	case BreakException:
		text = word + " on exception"

	case BreakAddress:
		if bp.Routine {
			text = word + " at routine " + bp.Name
		} else {
			text = word + " at " + d.Console.LocationText(bp.Addr)
		}
	}

	if bp.Temporary {
		text += " [temporary]"
	}

	return text
}

// breakOpNames are the mnemonics a class breakpoint's SHOW BREAK lists.
func (d *Debugger) breakOpNames(bp *Breakpoint) []string {
	switch bp.Kind {
	case BreakCall:
		return callNames

	case BreakBranch:
		return branchNames
	}

	return bp.OpNames
}

// opcodeLines lays mnemonics out as SHOW BREAK does: eight to a line, each
// padded to eight columns, the lines starting with a blank.
func opcodeLines(names []string) []string {
	var lines []string

	for i := 0; i < len(names); i += 8 {
		var fields []string

		for _, name := range names[i:min(i+8, len(names))] {
			fields = append(fields, fmt.Sprintf("%-8s", name))
		}

		lines = append(lines, " "+strings.Join(fields, " "))
	}

	return lines
}

// cancelBreak implements CANCEL BREAK: of the breakpoints at the given
// addresses, of a class (/CALL, /BRANCH, /LINE, /INSTRUCTION, /RETURN,
// /EXCEPTION, /FAULT), or all of them (/ALL). Cancelling nothing that was
// set is %DEBUG-I-NOBREAKS.
func (d *Debugger) cancelBreak(r *dcl.Result) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	before := len(d.Breakpoints)
	faultsBefore := len(d.Console.Engine.FaultBreakpoints())
	opcodesBefore := len(d.InstructionBreakpoints)

	kind, err := class(r, "ALL")
	if err != nil {
		return err
	}

	switch {
	case kind == "ALL":
		// The debugger's own breakpoints stay: a STEP in progress needs
		// its.
		d.Breakpoints = slices.DeleteFunc(d.Breakpoints, func(bp *Breakpoint) bool { return !bp.Step && !bp.Quiet })
		d.Console.Engine.ClearFaultBreakpoints()
		d.InstructionBreakpoints = nil

	case kind == "FAULT":
		if text := r.String("FAULT"); text != "" {
			err = d.RemoveFaultBreakpoint(text)
		} else {
			err = d.ClearAllFaultBreakpoints()
		}

	case kind == "":
		err = d.cancelAddressBreaks(&d.Breakpoints, r.String("TARGET"))

	default:
		err = d.cancelClassBreak(&d.Breakpoints, kind, r)
	}

	if err != nil {
		return err
	}

	if len(d.Breakpoints) == before &&
		len(d.Console.Engine.FaultBreakpoints()) == faultsBefore &&
		len(d.InstructionBreakpoints) == opcodesBefore {
		d.Console.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOBREAKS))
	}

	return nil
}

// cancelAddressBreaks removes the user's address breakpoints at each
// address in the list. A routine's name cancels its breakpoint "at
// routine", which is past its entry mask.
func (d *Debugger) cancelAddressBreaks(list *[]*Breakpoint, target string) error {
	if strings.TrimSpace(target) == "" {
		return vmserrors.New(vmserrors.CLI_NEEDBREAKADDR)
	}

	for _, text := range splitTop(target, ',') {
		addr, err := d.evalWhole(text)
		if err != nil {
			return err
		}

		if d.Console.RoutineEntry(addr) {
			addr += 2
		}

		*list = slices.DeleteFunc(*list, func(bp *Breakpoint) bool {
			return bp.Kind == BreakAddress && bp.Addr == addr && !bp.Step && !bp.Quiet
		})
	}

	return nil
}

// cancelClassBreak removes the breakpoints of a class qualifier's kind.
// /INSTRUCTION=(opcodes) removes just those opcodes from an opcode
// breakpoint, and /RETURN routine just that routine's.
func (d *Debugger) cancelClassBreak(list *[]*Breakpoint, kind string, r *dcl.Result) error {
	switch kind {
	case "CALL":
		d.removeKind(list, BreakCall)

	case "BRANCH":
		d.removeKind(list, BreakBranch)

	case "LINE":
		d.removeKind(list, BreakLine)

	case "EXCEPTION":
		d.removeKind(list, BreakException)

	case "RETURN":
		return d.cancelReturnBreak(list, strings.TrimSpace(r.String("TARGET")))

	case "INSTRUCTION":
		return d.cancelInstructionBreak(list, r.List("INSTRUCTION"))
	}

	return nil
}

// removeKind removes every breakpoint of a kind.
func (d *Debugger) removeKind(list *[]*Breakpoint, kind BreakKind) {
	*list = slices.DeleteFunc(*list, func(bp *Breakpoint) bool { return bp.Kind == kind })
}

// cancelReturnBreak removes the /RETURN breakpoint of the named routine,
// or of every routine when none is named.
func (d *Debugger) cancelReturnBreak(list *[]*Breakpoint, target string) error {
	if target == "" {
		d.removeKind(list, BreakReturn)

		return nil
	}

	addr, err := d.evalWhole(target)
	if err != nil {
		return err
	}

	start, _, _ := d.Console.RoutineExtent(addr)

	*list = slices.DeleteFunc(*list, func(bp *Breakpoint) bool {
		return bp.Kind == BreakReturn && bp.Start == start
	})

	return nil
}

// cancelInstructionBreak removes opcodes from the instruction breakpoint,
// or the whole breakpoint when no opcodes are given (or it breaks on every
// instruction). A breakpoint left with no opcodes is removed.
func (d *Debugger) cancelInstructionBreak(list *[]*Breakpoint, names []string) error {
	var gone []*cpu.Instruction

	for _, name := range names {
		if name = strings.ToUpper(strings.TrimSpace(name)); name == "" {
			continue
		}

		inst := lookupInstruction(name)
		if inst == nil {
			return vmserrors.New(vmserrors.CLI_BADOPCODE, name)
		}

		gone = append(gone, inst)
	}

	if len(gone) == 0 {
		d.removeKind(list, BreakInstruction)
		d.removeKind(list, BreakAnyInstruction)

		return nil
	}

	for _, bp := range *list {
		if bp.Kind != BreakInstruction {
			continue
		}

		for i := len(bp.Ops) - 1; i >= 0; i-- {
			if slices.Contains(gone, bp.Ops[i]) {
				bp.Ops = slices.Delete(bp.Ops, i, i+1)
				bp.OpNames = slices.Delete(bp.OpNames, i, i+1)
			}
		}
	}

	*list = slices.DeleteFunc(*list, func(bp *Breakpoint) bool {
		return bp.Kind == BreakInstruction && len(bp.Ops) == 0
	})

	return nil
}
