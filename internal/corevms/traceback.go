package corevms

import "github.com/tucats/govax/internal/vmsdef"

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

	if env.reportCondition(d, words) {
		return 0, ErrWait
	}

	// The symbolic stack dump follows the message here (subtask 6).

	if cond&7 != severitySevere {
		return ssContinue, nil
	}

	// The image ends here, by $EXIT, so its exit handlers run; the
	// dispatch, which its handler doesn't return to, is over.
	env.removeDispatch(d)

	return env.ExitFromShim(cond | stsInhibitMsg)
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
