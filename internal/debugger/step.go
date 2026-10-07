package debugger

import (
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// STEP as the VMS debugger has it (docs/PHASE-42.md, subtask 7). A step
// has three independent choices, each with a default that SET STEP changes
// and that a STEP qualifier overrides for one command:
//
//   - the *unit*: one source line (the default) or one machine instruction
//     (/LINE, /INSTRUCTION). Where the program has no line-number table (the
//     kernel, code assembled at the console) a step is by instruction
//     whatever the unit.
//   - how to treat *calls*: step over a routine call so the whole routine
//     runs as one step (/OVER, the default), or step into it (/INTO). A
//     third mode, /RETURN, runs until the routine currently executing is
//     about to return.
//   - the *report*: /SILENT shows nothing; /SOURCE (the default) also shows
//     the source line, which subtask 8 adds.
//
// /BRANCH and /CALL are different again: they run to the next instruction
// of that class, wherever it is.
//
// A VAX *call frame* is the block of stack a CALLS or CALLG instruction
// builds; FP, the frame pointer, points at the current one, and RET tears
// it down.

// StepMode selects what a STEP does about routine calls, matching vax.h's
// STEP_INSTRUCTION/STEP_OVER/STEP_RETURN constants — see docs/PHASE-18.md.
type StepMode int

const (
	// StepInto follows a call into the called routine — STEP/INTO (also
	// /IN).
	StepInto StepMode = iota
	// StepOver lets a "call-like" instruction (see stepOverInstructions)
	// run the called routine to completion, silently, and stops once
	// control returns to the instruction after it. The VMS default.
	StepOver
	// StepReturn runs until the RET that ends the call frame current when
	// the command was given is about to execute.
	StepReturn
)

// String matches show_step's own ps assignment (console_show.c:765):
// "INTO" for anything other than OVER/RETURN.
func (m StepMode) String() string {
	switch m {
	case StepOver:
		return "OVER"
	case StepReturn:
		return "RETURN"
	default:
		return "INTO"
	}
}

// stepOverInstructions matches break_over_instruction's break_list
// (vax.c): the procedure-call/subroutine-call/change-mode instructions
// STEP/OVER treats as candidates to run to completion rather than step
// into, classified by Instruction.Name rather than raw opcode value.
var stepOverInstructions = map[string]bool{
	"BSBB": true, "JSB": true, "BSBW": true,
	"CHMK": true, "CHME": true, "CHMS": true, "CHMU": true,
	"CALLG": true, "CALLS": true,
}

// stepSettings are the defaults SET STEP changes and SHOW STEP shows:
// the unit, whether the report is silent, and whether it shows source. The
// call-handling default is Debugger.StepMode, which predates this struct.
type stepSettings struct {
	byInstruction bool
	silent        bool
	noSource      bool
}

// stepRequest is one STEP command once the defaults and the command's own
// qualifiers have been combined.
type stepRequest struct {
	mode          StepMode
	byInstruction bool
	class         string // "", "BRANCH", or "CALL"
	silent        bool
	noSource      bool
	count         int
}

// pendingReturn is a STEP/RETURN waiting for its RET. It belongs to a call
// frame, not to the code: frame is FP when the command was given, and the
// step is done when the RET that ends that frame is about to run, however
// many other commands, breaks, and deeper calls come first. from is where
// the command was given, for the report.
type pendingReturn struct {
	frame    uint32
	from     string
	silent   bool
	noSource bool
}

// parseStepWord applies one SET STEP keyword to the settings: LINE or
// INSTRUCTION (the unit), INTO/IN, OVER, or RETURN (what a step does about
// calls), SILENT or NOSILENT, SOURCE or NOSOURCE. A leading '/' is
// ignored, and any unambiguous prefix of three letters or more is
// accepted (IN and S are too short to be unique, so IN is a special case
// and "S" is refused).
func (d *Debugger) parseStepWord(word string) error {
	w := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(word), "/"))

	is := func(full string) bool { return len(w) >= 3 && strings.HasPrefix(full, w) }

	switch {
	case w == "IN" || is("INTO"):
		d.StepMode = StepInto
	case is("OVER"):
		d.StepMode = StepOver
	case is("RETURN"):
		d.StepMode = StepReturn
	case is("INSTRUCTION"):
		d.stepDefaults.byInstruction = true
	case is("LINE"):
		d.stepDefaults.byInstruction = false
	case is("SILENT"):
		d.stepDefaults.silent = true
	case is("NOSILENT"):
		d.stepDefaults.silent = false
	case is("SOURCE"):
		d.stepDefaults.noSource = false
	case is("NOSOURCE"):
		d.stepDefaults.noSource = true
	default:
		return vmserrors.New(vmserrors.DBG_SYNTAX, word)
	}

	return nil
}

// parseStepModeWord classifies a word that names a call-handling mode —
// OVER, INTO/IN, or RETURN, an optional leading '/' ignored — by an
// unambiguous prefix. It is how the console's STEP/INTO, /OVER, and
// /RETURN reach the debugger (Activation.StepMode).
func parseStepModeWord(word string) (StepMode, bool) {
	w := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(word), "/"))

	switch {
	case w == "IN" || strings.HasPrefix(w, "INT"):
		return StepInto, true
	case strings.HasPrefix(w, "OVE"):
		return StepOver, true
	case strings.HasPrefix(w, "RET"):
		return StepReturn, true
	default:
		return 0, false
	}
}

// SetStepMode implements SET STEP: one or more keywords, separated by
// commas or blanks (SET STEP INSTRUCTION, SET STEP NOSOURCE, SET STEP
// INTO,LINE). A keyword it doesn't know is %DEBUG-E-SYNTAX, naming it (the
// probe's errors.dlg), and the keywords before it have taken effect.
func (d *Debugger) SetStepMode(words string) error {
	fields := strings.FieldsFunc(words, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return vmserrors.New(vmserrors.DBG_SYNTAX, words)
	}

	for _, word := range fields {
		if err := d.parseStepWord(word); err != nil {
			return err
		}
	}

	return nil
}

// ShowStepMode implements SHOW STEP, in the VMS debugger's two-line form:
//
//	step type: source, nosilent, by line,
//	           over routine calls
//
// A default of RETURN (govax's SET STEP RETURN) is shown as "to return".
func (d *Debugger) ShowStepMode() error {
	s := d.stepDefaults

	source, silent, unit := "source", "nosilent", "line"

	if s.noSource {
		source = "nosource"
	}

	if s.silent {
		silent = "silent"
	}

	if s.byInstruction {
		unit = "instruction"
	}

	calls := "over routine calls"

	switch d.StepMode {
	case StepInto:
		calls = "into routine calls"
	case StepReturn:
		calls = "to return"
	}

	d.Console.Printf("step type: %s, %s, by %s,\n           %s\n", source, silent, unit, calls)

	return nil
}

// buildStepRequest combines the defaults with what the STEP command
// asked for. Everything in the activation is optional; an unset field
// leaves the default.
func (d *Debugger) buildStepRequest(a console.Activation) (stepRequest, error) {
	req := stepRequest{
		mode:          d.StepMode,
		byInstruction: d.stepDefaults.byInstruction,
		silent:        d.stepDefaults.silent,
		noSource:      d.stepDefaults.noSource,
		count:         max(a.Count, 1),
	}

	if a.StepMode != "" {
		mode, ok := parseStepModeWord(a.StepMode)
		if !ok {
			return req, vmserrors.New(vmserrors.CLI_BADQUALIFIER, a.StepMode)
		}

		req.mode = mode
	}

	switch a.StepUnit {
	case "INSTRUCTION":
		req.byInstruction = true
	case "LINE":
		req.byInstruction = false
	}

	req.class = a.StepClass

	if a.StepSilent != nil {
		req.silent = *a.StepSilent
	}

	if a.StepSource != nil {
		req.noSource = !*a.StepSource
	}

	return req, nil
}

// setStepBreakpoint installs a one-shot address breakpoint used internally
// by STEP/OVER to resume the debugger once execution returns to addr — the
// Go equivalent of vax.c's own set_break(pc, 0,
// BREAK_ADDRESS|BREAK_STEP|BREAK_TEMPORARY) calls. It returns the
// breakpoint, for the STEP to remove once its run stops for any reason
// (endStep), and to ask whether it was the one that stopped the run
// (Breakpoint.fired).
func (d *Debugger) setStepBreakpoint(addr uint32) *Breakpoint {
	return d.addStepBreakpoint(&Breakpoint{Kind: BreakAddress, Addr: addr})
}

// addStepBreakpoint adds bp to the list as one of a STEP's own: it is
// removed when it fires, says nothing when it does (the STEP reports), and
// is marked so.
func (d *Debugger) addStepBreakpoint(bp *Breakpoint) *Breakpoint {
	bp.Temporary, bp.Step, bp.Quiet = true, true, true
	d.Breakpoints = append(d.Breakpoints, bp)

	return bp
}

// endStep removes a STEP's one-shot breakpoint when the STEP's run stops.
// When the run reached it, runLoop has already removed it and this does
// nothing; when the run stopped first somewhere else (a user breakpoint,
// a fault breakpoint, Ctrl-C, a HALT, an unhandled fault), the STEP is
// over, and its breakpoint would otherwise stay set and stop a later GO
// (docs/PHASE-42.md, bugs 2 and 3). A STEP/RETURN is not like this: it is
// bound to its frame and stays pending (pendingReturn).
func (d *Debugger) endStep(bp *Breakpoint) {
	d.removeBreakpointPtr(bp)
}

// stepRun implements STEP: starting at the current PC (or startAddr, if
// non-nil), it takes req.count steps, each as req describes, stopping early
// if a step ends or is interrupted by anything but its own completion
// (a break, an exception, the program ending).
func (d *Debugger) stepRun(startAddr *uint32, req stepRequest) (runOutcome, error) {
	c := d.Console

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	} else if err := d.requireProgram(); err != nil {
		return runEnded, err
	}

	c.Engine.BeginRun()

	var (
		outcome runOutcome
		err     error
	)

	// STEP n reports only where it ends up, as VMS's does: the steps
	// before the last are silent.
	last := req

	for i := 0; i < req.count; i++ {
		var completed bool

		this := last
		this.silent = last.silent || i < req.count-1

		outcome, completed, err = d.stepOnce(this)
		if err != nil || outcome != runStopped || !completed {
			return outcome, err
		}
	}

	return outcome, err
}

// stepOnce takes one step. completed is true when the step ran to its
// natural end (and reported it), false when something else stopped the
// run, which has said so itself and ends the whole command.
func (d *Debugger) stepOnce(req stepRequest) (outcome runOutcome, completed bool, err error) {
	switch {
	case req.mode == StepReturn && req.class == "":
		return d.stepReturn(req)

	case req.class != "":
		return d.stepToClass(req)
	}

	return d.stepUnit(req)
}

// stepUnit steps one instruction or one source line, treating calls as
// req.mode says.
//
// By instruction it executes one instruction and reports where it
// landed. By line it keeps executing until the PC is at the first
// instruction of a source line; an instruction that returns from a routine
// (RET or RSB) also ends the step, since it lands in the middle of the
// caller's line (unconfirmed: the probe never stepped a line off a
// RET). Where the program has no line-number table the step is by
// instruction.
//
// The default is USER-mode stepping, so we just keep running the CPU loop
// if we get out of USER mode (for example, handling an interrupt). Use the
// SET DEBUG command to turn on USER mode stepping if you need to step into
// KERNEL mode, etc.
func (d *Debugger) stepUnit(req stepRequest) (runOutcome, bool, error) {
	c := d.Console

	// Stepping by line starts from code that has a line-number table; if
	// the step leaves such code (into a routine of the run-time library,
	// or off the end of the program into the image driver) it carries on
	// until it is back in some, or the program ends.
	hadLines := c.HasLineInfo(c.CPU.GPR(vax.PC))

	for {
		if outcome, done, err := d.stepOne(); done {
			return outcome, false, err
		}

		dec := c.Engine.LastDecoded()
		name := dec.Instruction.Name

		// Step over a call: let the routine run, silently, until control
		// is back at the instruction after the call.
		if req.mode == StepOver && stepOverInstructions[name] {
			bp := d.setStepBreakpoint(dec.NextPC)

			outcome, err := d.runLoop(false, func(uint32) func() { return func() {} })

			d.endStep(bp)

			if err != nil || outcome != runStopped || !bp.fired {
				return outcome, false, err
			}
		}

		pc := c.CPU.GPR(vax.PC)
		hasLines := c.HasLineInfo(pc)

		if !req.byInstruction && hadLines && !hasLines {
			continue
		}

		if req.byInstruction || !hasLines || c.LineStart(pc) || name == "RET" || name == "RSB" {
			d.reportStep(req, pc)

			return runStopped, true, nil
		}
	}
}

// stepOne executes the one instruction at the current PC. It is the
// building block of a step by instruction or line. done is true when the
// step must end without reporting: the run ended in that instruction (a
// HALT, a fault, ...), an unhandled condition paused the program, or a
// STEP/RETURN that was waiting for this instruction fired. In those cases
// outcome and err are the run's result and the caller returns them as they
// are.
func (d *Debugger) stepOne() (outcome runOutcome, done bool, err error) {
	c := d.Console

	if d.returnDue() {
		return runStopped, true, nil
	}

	d.snapshotWatches()

	startPC := c.CPU.GPR(vax.PC)
	finish := c.TraceStep(startPC, false)
	userStep := c.Engine.CPU().DebugEnabled(vax.DebugUserStep)

	for {
		if stepErr := c.StepMachine(); stepErr != nil {
			outcome, err = d.reportStop(stepErr)

			return outcome, true, err
		}

		// If USER-mode only stepping is enabled and we are not in USER
		// mode, just keep running. This allows a STEP operation to trace
		// just USER mode code and ignores timer interrupts, etc.
		if userStep && c.Engine.CPU().PSL().CurMod() < 0b11 {
			continue
		}

		break
	}

	finish()

	// A watched location the instruction changed ends the step with the
	// watchpoint's report, and so does a condition nobody handled.
	if d.watchHit(startPC) || d.signalBreak() || d.unhandledBreak() {
		return runStopped, true, nil
	}

	return runStopped, false, nil
}

// stepToClass implements STEP/BRANCH and STEP/CALL: run until the next
// instruction of that class is about to execute (a breakpoint of the class,
// one the step owns), and report it. The instruction the step starts at
// doesn't count, so repeating the command moves on.
func (d *Debugger) stepToClass(req stepRequest) (runOutcome, bool, error) {
	kind := BreakBranch
	if req.class == "CALL" {
		kind = BreakCall
	}

	bp := d.addStepBreakpoint(&Breakpoint{Kind: kind})
	c := d.Console

	outcome, err := d.runLoop(true, func(pc uint32) func() { return c.TraceStep(pc, false) })

	d.endStep(bp)

	if err != nil || outcome != runStopped || !bp.fired {
		return outcome, false, err
	}

	// A class step always shows the instruction it stopped at, since
	// that is what the user asked about.
	req.byInstruction = true
	d.reportStep(req, c.CPU.GPR(vax.PC))

	return runStopped, true, nil
}

// stepReturn implements STEP/RETURN. It doesn't run to a place but to an
// event: the RET that ends the call frame the command was given in. The
// frame is bound to FP, so a deeper call's RET passes, and from a JSB
// subroutine (which has no frame of its own) the step waits for the RET of
// the routine it runs in, passing the subroutine's RSB.
//
// The wait outlives the command, as VMS's does (testdata/dbgcmd/vax/
// step.dlg): a break, an exception, or another STEP in between doesn't
// cancel it, and it fires whenever the RET is next about to run (returnDue).
// A second STEP/RETURN replaces a pending one (unconfirmed). Until it fires
// the program runs as GO does, tracing per Console.Trace.
func (d *Debugger) stepReturn(req stepRequest) (runOutcome, bool, error) {
	c := d.Console

	if c.CPU.GPR(vax.FP) == 0 || c.CPU.GPR(vax.AP) == 0 {
		return runEnded, false, vmserrors.New(vmserrors.CLI_NOFRAMES)
	}

	// The frame must be readable: its saved PC is the word at FP+16.
	if _, err := c.Mem.LoadLongword(c.CPU, c.CPU.GPR(vax.FP)+16); err != nil {
		return runEnded, false, err
	}

	d.pendingReturn = &pendingReturn{
		frame:  c.CPU.GPR(vax.FP),
		from:   c.LocationText(c.CPU.GPR(vax.PC)),
		silent: req.silent,

		noSource: req.noSource,
	}

	outcome, err := d.runLoop(true, func(pc uint32) func() { return c.TraceStep(pc, false) })

	// The step is complete only if the pending return fired. It prints
	// its own report, so there is nothing more to say either way.
	return outcome, false, err
}

// returnDue checks, before an instruction runs, whether it is the RET a
// STEP/RETURN is waiting for: a RET while FP is the frame the command was
// given in. If so it reports the step and clears it. A wait whose frame has
// gone without its RET running (the stack unwound past it) is dropped.
func (d *Debugger) returnDue() bool {
	p := d.pendingReturn
	if p == nil {
		return false
	}

	c := d.Console
	fp := c.CPU.GPR(vax.FP)

	// The stack grows toward lower addresses, so a frame above the current
	// one is gone.
	if fp > p.frame {
		d.pendingReturn = nil

		return false
	}

	inst, _ := c.Engine.PeekInstruction()
	if fp != p.frame || inst == nil || inst.Name != "RET" {
		return false
	}

	d.pendingReturn = nil

	if !p.silent {
		pc := c.CPU.GPR(vax.PC)
		c.Printf("stepped on return from %s to %s: %s\n", p.from, c.LocationText(pc), c.InstructionText(pc))

		if !p.noSource {
			d.showSource(pc)
		}
	}

	return true
}

// reportStep says where a completed step landed, as the VMS debugger
// does:
//
//	stepped to DBGCMD\START\%LINE 34                   (by line)
//	stepped to DBGCMD\FACT\%LINE 55: CMPL     R2,S^#01 (by instruction)
//	stepped to routine DBGCMD\FACT                     (into a routine)
//
// /SILENT says nothing. Each report is followed by the source line (unless
// /NOSOURCE), as the VMS debugger shows it.
func (d *Debugger) reportStep(req stepRequest, pc uint32) {
	if req.silent {
		return
	}

	c := d.Console

	if name, ok := c.EnteredRoutine(pc); ok {
		c.Printf("stepped to routine %s\n", name)
		d.stepSource(req, pc)

		return
	}

	// The instruction is shown only for a step by instruction. Where an image
	// has no line numbers a step by line is one instruction too, but without
	// the instruction's text (dbgtrc.dlg).
	if req.byInstruction {
		c.Printf("stepped to %s: %s\n", c.LocationText(pc), c.InstructionText(pc))
		d.stepSource(req, pc)

		return
	}

	c.Printf("stepped to %s\n", c.LocationText(pc))
	d.stepSource(req, pc)
}

// stepSource shows the source line a step stopped at, unless the step is
// /NOSOURCE (by its own qualifier or SET STEP NOSOURCE).
func (d *Debugger) stepSource(req stepRequest, pc uint32) {
	if !req.noSource {
		d.showSource(pc)
	}
}

// bindSetStep binds SET STEP and SHOW STEP.
func (d *Dispatcher) bindSetStep() {
	d.Grammar.Bind("SET_STEP", func(id int64, r *dcl.Result) error {
		return d.Debugger.SetStepMode(r.String("WORDS"))
	})
	d.Grammar.Bind("SHOW_STEP", func(id int64, r *dcl.Result) error { return d.Debugger.ShowStepMode() })
}
