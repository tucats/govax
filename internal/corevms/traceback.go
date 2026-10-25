package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/vmsdef"
)

// The traceback handler (docs/PHASE-70 - traceback.md, Decision 1).
//
// An image linked with traceback, LINK's default, has SYS$IMGSTA's
// address as its first transfer address. On VMS that "image startup"
// procedure runs first: it puts a condition handler in its own call
// frame, then calls the image's main routine (Internals book, 21.1.3.2
// and 21.1.3.3). So the handler covers the whole image, and the
// condition search finds it after every frame of the program's own, but
// before the catch-all in the CLI's frame below it.
//
// The handler takes the conditions the program's handlers passed on
// whose severity is warning, error, or severe (4.5.1): it writes the
// condition's message, then the traceback facility's symbolic stack
// dump, and continues the program for a warning or an error, or ends
// the image for a severe condition, with the condition as its status,
// marked STS$M_INHIB_MSG so the message isn't shown again. Success and
// informational conditions, and SS$_DEBUG (the CLI's DEBUG after a
// Ctrl/Y, which would map the debugger), it resignals, so the catch-all
// takes them.
//
// govax's image startup is RUN's IMAGE$INIT driver (internal/machine's
// buildDriver): for an image with SYS$IMGSTA's transfer address, it
// stores this handler's stub in its own frame before calling the main
// routine. The stub is a shim's (EXE$TRACEBACK, tracebackShimCode).

// tracebackShimCode is EXE$TRACEBACK's XFC$SHIM code. It's govax's own,
// so it takes a code outside every range LIBRTL's routines use (theirs
// count up from 1, the missing routines' from 1000 and 2000).
const tracebackShimCode = 3000

// ssDebug is SS$_DEBUG, the condition the CLI signals to start the
// debugger in an interrupted image, and ssContinue SS$_CONTINUE, a
// handler's answer that the condition has been dealt with.
var (
	ssDebug    = vmsdef.Symbols["SS$_DEBUG"]
	ssContinue = vmsdef.Symbols["SS$_CONTINUE"]
)

// Severities the traceback handler takes (a condition value's bits 0-2).
const (
	severityWarning = 0
	severityError   = 2
)

// shimTraceback is EXE$TRACEBACK, the traceback handler: a condition
// handler, called by the dispatcher with the signal and mechanism
// arrays (this file's opening comment).
//
// A condition raised while the image's exit handlers run is resignaled:
// on VMS image rundown has removed the image startup's frame by then,
// and the catch-all takes it (testdata/tb70's part X). So is a call that
// isn't the dispatcher's.
func shimTraceback(env *Environment, argv []uint32) (uint32, error) {
	d := env.dispatchCalling(argv)
	if d == nil || env.Process.exiting {
		return ssResignal, nil
	}

	// The condition as the signal array has it now: a handler's change
	// of it counts (testdata/tb70's S10).
	words, err := env.sigWords(d)
	if err != nil || len(words) < 3 {
		return ssResignal, nil
	}

	cond := words[0]

	switch sev := cond & 7; {
	case cond == ssDebug:
		return ssResignal, nil

	case sev != severityWarning && sev != severityError && sev != severitySevere:
		return ssResignal, nil
	}

	// The handler's frame is the image startup's: the mechanism array
	// names it.
	startup, err := env.mem.LoadLongword(env.cpu, d.mech+4)
	if err != nil {
		return ssResignal, nil
	}

	dump := func() []string { return env.tracebackDump(d, cond, startup) }

	if env.reportCondition(d, words, dump) {
		return 0, ErrWait
	}

	// A severe condition goes on too when the debugger stopped for it
	// (catchAll says why).
	if cond&7 != severitySevere || d.debuggerHeld && d.kind != kindStop {
		return ssContinue, nil
	}

	// The image ends here, by $EXIT, so its exit handlers run; the
	// dispatch, which its handler doesn't return to, is over.
	env.removeDispatch(d)

	return env.ExitFromShim(cond | stsInhibitMsg)
}

// TraceName is how the traceback dump names a PC: its module and routine
// columns, the PC relative to what names it (the PC itself when nothing
// does), and its listing line, 0 for none (internal/dbgsym's TraceName
// has the rules).
type TraceName struct {
	Module  string
	Routine string
	Rel     uint32
	Line    int
}

// The traceback dump's heading lines (testdata/tb70's VMS log).
const (
	tracebackMessage = "-TRACEBACK, symbolic stack dump follows"
	tracebackHeading = "module name     routine name                     line       rel PC    abs PC"
)

// maxTracebackLines bounds a dump. VMS's has no depth limit (60 and 100
// frames were all shown, testdata/tb70's S17 and D); this only stops a
// damaged chain from going on for ever.
const maxTracebackLines = 1 << 20

// faultConditions are the conditions of the exceptions that are faults:
// their PC is the faulting instruction's own, so the traceback dump's
// line for it is the PC's, where any other PC's (a trap's, which is the
// next instruction's; a return address) is the byte before it. VMS
// decides by the condition, however it was raised: TB70D's SS$_ACCVIO
// by LIB$SIGNAL (part C) has the line of the instruction after the call.
// Only ACCVIO, OPCDEC, and ROPRAND are confirmed (testdata/tb70).
var faultConditions = func() map[uint32]bool {
	m := map[uint32]bool{ssAccVioCond & condIDMask: true}

	for _, cond := range simpleExceptions {
		m[cond&condIDMask] = true
	}

	// The floating faults, arithmetic exception types 8 to 10.
	for code := uint32(8); code <= 10; code++ {
		m[arithmeticConditions[code]&condIDMask] = true
	}

	return m
}()

// tracebackDump returns the traceback facility's symbolic stack dump for
// d's condition, cond (as the signal array has it now), whose handler is
// established in the image startup's frame, startup (Part A, "The
// frames" and "The layout"):
//
//	%TRACE-E-TRACEBACK, symbolic stack dump follows
//	module name     routine name                     line       rel PC    abs PC
//
//	FAILSUB         SUB2                                       00000005  0000021C
//	...
//
// The lines are, innermost first, the condition's PC, then each call
// frame's saved PC from the frame the condition happened in, up to the
// frame the image startup called, whose saved PC (in the startup) isn't
// shown. A condition handler's frame, whose saved PC is SYS$SRCHANDLER's
// (the dispatcher called it), is shown differently: the condition it was
// called for is shown, as its value and message between two "-----"
// lines, then that condition's PC and its frames; the dispatcher's own
// aren't. An AST routine's frame is like any other: its saved PC is the
// AST exit's, and its saved FP the frame the AST interrupted.
func (env *Environment) tracebackDump(d *conditionDispatch, cond, startup uint32) []string {
	lines := []string{
		"%TRACE-" + severityLetters[cond&7] + tracebackMessage,
		tracebackHeading,
		"",
	}

	name := func(pc uint32, exact bool) TraceName {
		if env.TraceName != nil {
			return env.TraceName(env, pc, exact)
		}

		return TraceName{Rel: pc}
	}

	show := func(pc uint32, exact bool) {
		n := name(pc, exact)

		line := ""
		if n.Line > 0 {
			line = fmt.Sprint(n.Line)
		}

		lines = append(lines, fmt.Sprintf("%-15.15s %-31s%6s      %08X  %08X", n.Module, n.Routine, line, n.Rel, pc))
	}

	load := func(addr uint32) (uint32, bool) {
		v, err := env.mem.LoadLongword(env.cpu, addr)

		return v, err == nil
	}

	// showCondition shows a condition's PC and returns the frame its
	// walk starts from.
	showCondition := func(d *conditionDispatch) (uint32, bool) {
		words, err := env.sigWords(d)
		if err != nil || len(words) < 3 {
			return 0, false
		}

		show(words[len(words)-2], faultConditions[words[0]&condIDMask])

		return d.startFP, true
	}

	frame, ok := showCondition(d)

	for ok && len(lines) < maxTracebackLines {
		if frame == 0 || frame == consoleCallSentinel {
			break
		}

		caller, ok1 := load(frame + frameSavedFP)
		savedPC, ok2 := load(frame + frameSavedPC)

		if !ok1 || !ok2 || caller == startup {
			break
		}

		switch savedPC {
		case srchandlerAddr:
			earlier := env.dispatchForHandler(frame)
			if earlier == nil {
				return lines
			}

			lines = append(lines, env.earlierCondition(earlier)...)

			if frame, ok = showCondition(earlier); !ok {
				return lines
			}

			continue

		default:
			show(savedPC, false)
		}

		// Callers' frames are higher on the stack; a chain that doesn't
		// climb is damaged.
		if caller <= frame {
			break
		}

		frame = caller
	}

	return lines
}

// earlierCondition returns the lines that show, in a traceback dump, the
// condition a handler was called for:
//
//	----- above condition handler called with exception 00000870:
//	%SYSTEM-W-ENDOFFILE, end of file
//	----- end of exception message
func (env *Environment) earlierCondition(d *conditionDispatch) []string {
	words, err := env.sigWords(d)
	if err != nil || len(words) < 3 {
		return nil
	}

	lines := []string{fmt.Sprintf("----- above condition handler called with exception %08X:", words[0])}
	lines = append(lines, env.signalMessage(words)...)

	return append(lines, "----- end of exception message")
}

// dispatchForHandler returns the dispatch that called the condition
// handler whose call frame is frame: the one whose argument list the
// handler was called with. Nil if there's none.
func (env *Environment) dispatchForHandler(frame uint32) *conditionDispatch {
	argList, ok := env.handlerArgList(frame)
	if !ok {
		return nil
	}

	for _, d := range env.Process.conditions {
		if d.argList == argList {
			return d
		}
	}

	return nil
}

// dispatchCalling returns the dispatch that is calling a condition
// handler with argv as its arguments (the signal array's and the
// mechanism array's addresses), or nil.
func (env *Environment) dispatchCalling(argv []uint32) *conditionDispatch {
	if len(argv) < 2 {
		return nil
	}

	for i := len(env.Process.conditions) - 1; i >= 0; i-- {
		d := env.Process.conditions[i]
		if d.calling && d.sig == argv[0] && d.mech == argv[1] {
			return d
		}
	}

	return nil
}

// removeDispatch removes d from the dispatches in progress.
func (env *Environment) removeDispatch(d *conditionDispatch) {
	p := env.Process

	for i, c := range p.conditions {
		if c == d {
			p.conditions = append(p.conditions[:i], p.conditions[i+1:]...)

			return
		}
	}
}

func registerTracebackShims(t *ShimTable) {
	t.Register(tracebackShimCode, "EXE$TRACEBACK", shimTraceback)
}
