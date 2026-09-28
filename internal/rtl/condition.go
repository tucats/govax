package rtl

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Condition handling (docs/PHASE-26.md subtasks 31-34): how VMS reports
// an exception or a software error to the program's condition handlers.
//
// # Conditions and handlers
//
// When something goes wrong in a VMS program — the hardware detects an
// access violation, a divide by zero, a reserved operand; or software
// decides to report an error by calling LIB$SIGNAL — VMS *signals a
// condition*. A condition is described by a condition value (SS$_ACCVIO,
// SS$_INTDIV, or a program's own code: the same kind of value a service
// returns in R0) plus a few longwords of detail.
//
// A program can supply *condition handlers*: procedures VMS calls when a
// condition is signaled, which decide what to do about it. Most handlers
// are attached to a call frame. The first longword of every VAX call
// frame (at 0(FP)) is reserved for the address of a handler; CALLS and
// CALLG set it to 0, and a procedure that wants a handler stores its
// address there (MOVAB handler, (FP), or LIB$ESTABLISH). A handler then
// covers everything that happens while its procedure is active,
// including inside the procedures it calls.
//
// # The search
//
// VMS looks for a handler by walking the chain of call frames, starting
// with the frame of the procedure that was running when the condition
// happened and moving to its caller, its caller's caller, and so on
// (each frame's saved FP, at 12(FP), is its caller's frame). Each handler
// found is called with two arguments:
//
//	signal array:     n, condition value, detail..., PC, PSL
//	mechanism array:  4, frame, depth, R0, R1
//
// The signal array describes the condition: n counts the longwords after
// it, PC is where execution would continue, and PSL is the processor
// status at the time. The mechanism array describes the search: frame is
// the frame that established the handler being called, depth how many
// frames the search has moved up from where the condition happened (0
// for that frame itself), and R0/R1 the registers when it happened.
//
// The handler returns one of two answers in R0:
//
//   - SS$_RESIGNAL (or any value with the low bit clear): "not mine" —
//     the search goes on to the next frame.
//   - SS$_CONTINUE (or any value with the low bit set): "dealt with" —
//     the program continues where the condition happened, with R0 and R1
//     taken from the mechanism array, which the handler may have changed.
//
// A third answer is to unwind the stack with $UNWIND (unwind.go).
//
// If every handler resignals, VMS's *catch-all* handler — established by
// the system in the program's outermost frame — prints the condition's
// message (%SYSTEM-F-ACCVIO, access violation, ...) and, if the
// condition's severity is SEVERE (F), ends the image through $EXIT;
// otherwise the program continues.
//
// # How govax does it
//
// On VMS, the kernel's exception code copies the arrays onto the stack of
// the mode the condition happened in and "returns" to SYS$SRCHANDLER, a
// system service vector entry reached by a jump (it has no argument list
// of its own), which does the search in that mode. govax does the same:
//
//  1. Something starts a dispatch. For a hardware exception it's the
//     engine: kernel.asm's SCB points the exceptions programs can handle
//     (ACCVIO, reserved operand, arithmetic, ...) at console$handler, and
//     the engine hands those to DispatchException (below). For software
//     it's LIB$SIGNAL or LIB$STOP (signal.go). Either way the RTL pushes
//     the signal array, the mechanism array, and a two-argument list
//     pointing at them onto the current stack, records a
//     conditionDispatch, and sets PC to SYS$SRCHANDLER.
//  2. SYS$SRCHANDLER's XFC runs serviceSysSrchandler, which finds the next
//     handler and returns a *CallRequest for it. The engine calls the
//     handler with the XFC as its return address — the same mechanism
//     $EXIT uses for exit handlers.
//  3. The handler's RET lands on the XFC again, so the service runs again
//     and looks at the handler's answer: resignal means back to step 2
//     for the next handler; continue means restoring the registers and
//     resuming the program; no handlers left means the catch-all.
//
// A handler may itself cause a condition (or signal one); that starts a
// dispatch of its own, stacked on top of the first, which is why the
// dispatches in progress are a stack (Process.conditions), each
// identified by the stack pointer its handlers return with.
//
// Phase 20's console-side search (internal/console/chf.go) remains as a
// fallback for an exception the RTL can't dispatch: one it has no
// condition value for, one taken on the interrupt stack, or one whose
// stack can't be written.

// signalKind is how a condition was signaled, which decides how the
// program continues after a handler says so.
type signalKind int

const (
	// kindException is a hardware exception: continuing resumes at the
	// signal array's PC with its PSL's condition codes, as REI would.
	kindException signalKind = iota

	// kindSignal is LIB$SIGNAL: continuing returns from LIB$SIGNAL.
	kindSignal

	// kindStop is LIB$STOP, which can't be continued: a handler that
	// tries gets "attempt to continue from stop", and the image exits.
	kindStop
)

// searchStage is where a dispatch's search has got to.
type searchStage int

const (
	// stagePrimary and stageSecondary are the access mode's primary and
	// secondary exception vectors ($SETEXV), searched before the frames.
	stagePrimary searchStage = iota
	stageSecondary

	// stageFrames is the call-frame walk.
	stageFrames

	// stageLastChance is the last-chance exception vector, searched when
	// the frames have nothing more.
	stageLastChance

	// stageCatchAll is the end: no handler continued, so the catch-all
	// handler's action applies.
	stageCatchAll
)

// Mechanism-array depths VMS reports for the three exception vectors
// (LIB$ manual, section 4.1.3.2).
const (
	depthPrimary    = -2
	depthSecondary  = -1
	depthLastChance = -3
)

// conditionDispatch is one condition being dispatched: the arrays on
// the stack, how far the search has got, and how to resume.
type conditionDispatch struct {
	kind signalKind

	// mode is the access mode the condition happened in, whose vectors
	// are searched.
	mode vax.AccessMode

	// sp is the stack pointer while the search runs: just below the
	// argument list, which is below the mechanism array, which is below
	// the signal array. A handler returns with SP here again, which is how
	// serviceSysSrchandler knows which dispatch it's continuing.
	sp uint32

	// sig, mech, and argList are the addresses of the signal array, the
	// mechanism array, and the handlers' two-argument list.
	sig, mech, argList uint32

	// sigCount is the signal array's first longword as the dispatcher
	// built it: the PC and PSL are its last two entries.
	sigCount uint32

	// stage and fp are the search's progress: fp is the next call frame
	// to look at, and depth its depth.
	stage searchStage
	fp    uint32
	depth int32

	// startFP is the frame the condition happened in, reported as the
	// mechanism array's frame for a vectored handler.
	startFP uint32

	// calling is true while a handler this dispatch called is running.
	calling bool

	// unwind is the $UNWIND a handler asked for, carried out when the
	// handler returns (unwind.go); nil if none.
	unwind *unwindRequest

	// resumeSP, resumeFP, and resumePC are where execution resumes when
	// a handler continues: for an exception, the stack as it was before
	// the arrays were pushed (the PC comes from the signal array); for
	// LIB$SIGNAL, the RET that ends LIB$SIGNAL's stub. resumePSL is the
	// PSL at the time, whose mode and IPL a continue keeps.
	resumeSP, resumeFP, resumePC uint32
	resumePSL                    vax.PSL
}

// Status values the condition dispatcher uses.
var (
	ssAccVioCond = vmsdef.SSConstants["SS$_ACCVIO"]
	ssResignal   = vmsdef.SSConstants["SS$_RESIGNAL"]
	ssNoSignal   = vmsdef.SSConstants["SS$_NOSIGNAL"]
)

// stsInhibitMsg is STS$M_INHIB_MSG, bit 28 of a condition value: "the
// message has already been shown". The catch-all sets it in the status
// it exits with, so whatever reports the image's exit status doesn't
// show the message a second time.
const stsInhibitMsg = 0x10000000

// severitySevere is a condition value's severity field (bits 0-2) for
// SEVERE (F) conditions, the ones the catch-all exits for.
const severitySevere = 4

// libAttConSto is LIB$_ATTCONSTO, "attempt to continue from stop"
// (%LIB-F-ATTCONSTO), from the VMS 7.3 message file's LIB facility.
const libAttConSto = 0x0015827C

// consoleCallSentinel is the saved FP and PC of the call frame the
// console builds to start a program (cpu.SentinelReturn): the bottom of
// the program's call stack, where the frame search stops.
const consoleCallSentinel = 0xFFFFDEAF

// SCB offsets of the exceptions DispatchException turns into condition
// values (cpu.Exc*: the offset of each exception's vector in the system
// control block, which is how the engine identifies an exception).
const (
	scbPrivileged    = 0x10 // privileged or reserved instruction
	scbCustomer      = 0x14 // customer-reserved instruction
	scbReservedOp    = 0x18 // reserved operand
	scbReservedAddr  = 0x1C // reserved addressing mode
	scbAccessViol    = 0x20 // access control violation
	scbTranslationNV = 0x24 // translation not valid
	scbArithmetic    = 0x34 // arithmetic trap or fault
)

// simpleExceptions are the exceptions whose condition value depends on
// nothing else, and which have no detail longwords.
var simpleExceptions = map[uint32]uint32{
	scbPrivileged:   vmsdef.SSConstants["SS$_OPCDEC"],
	scbCustomer:     vmsdef.SSConstants["SS$_OPCCUS"],
	scbReservedOp:   vmsdef.SSConstants["SS$_ROPRAND"],
	scbReservedAddr: vmsdef.SSConstants["SS$_RADRMOD"],
}

// arithmeticConditions maps an arithmetic exception's type code (its one
// parameter, VAX Architecture Reference Manual table 5-2) to its
// condition value. Types 1-7 are traps, 8-10 the faults of the same
// names.
var arithmeticConditions = map[uint32]uint32{
	1:  vmsdef.SSConstants["SS$_INTOVF"],
	2:  vmsdef.SSConstants["SS$_INTDIV"],
	3:  vmsdef.SSConstants["SS$_FLTOVF"],
	4:  vmsdef.SSConstants["SS$_FLTDIV"],
	5:  vmsdef.SSConstants["SS$_FLTUND"],
	6:  vmsdef.SSConstants["SS$_DECOVF"],
	7:  vmsdef.SSConstants["SS$_SUBRNG"],
	8:  vmsdef.SSConstants["SS$_FLTOVF_F"],
	9:  vmsdef.SSConstants["SS$_FLTDIV_F"],
	10: vmsdef.SSConstants["SS$_FLTUND_F"],
}

// ssArtRes is SS$_ARTRES, "reserved arithmetic trap": an arithmetic
// exception with a type code the table doesn't have.
var ssArtRes = vmsdef.SSConstants["SS$_ARTRES"]

// exceptionCondition returns the condition value and signal-array detail
// longwords for the hardware exception with SCB offset code and
// parameters params, as the engine pushes them (the first parameter
// deepest on the stack). ok is false for an exception that isn't
// signaled to handlers.
//
// An access violation's parameters are the virtual address and the
// reason mask; in the signal array (as in the exception frame, read from
// the top of the stack) the mask comes first. VMS turns a page fault on
// a page that can't be made valid into an access violation too, so a
// translation-not-valid exception becomes SS$_ACCVIO with its parameters.
func exceptionCondition(code uint32, params []uint32) (cond uint32, detail []uint32, ok bool) {
	if cond, ok := simpleExceptions[code]; ok {
		return cond, nil, true
	}

	switch code {
	case scbAccessViol, scbTranslationNV:
		va, mask := uint32(0), uint32(0)

		if len(params) > 0 {
			va = params[0]
		}

		if len(params) > 1 {
			mask = params[1]
		}

		return ssAccVioCond, []uint32{mask, va}, true

	case scbArithmetic:
		if len(params) > 0 {
			if cond, ok := arithmeticConditions[params[0]]; ok {
				return cond, nil, true
			}
		}

		return ssArtRes, nil, true
	}

	return 0, nil, false
}

// srchandlerAddr is the SYS$SRCHANDLER P1-vector entry, where a dispatch
// begins: an XFC at the entry address itself (it's reached by a jump, so
// there's no entry mask before it).
var srchandlerAddr = p1VectorAddr("SYS$SRCHANDLER")

// p1VectorAddr returns the address of the P1-vector entry name, which
// must exist.
func p1VectorAddr(name string) uint32 {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == name {
			return e.Addr
		}
	}

	panic("rtl: " + name + " missing from the P1 vector table")
}

// DispatchException starts dispatching a hardware exception to the
// program's condition handlers (step 1 of this file's opening comment).
// code is the exception's SCB offset and params its parameters, as the
// engine has them; pc is the PC to report and resume at, and psl the PSL
// when it happened.
//
// It reports false, having changed nothing, for an exception it can't
// dispatch: one with no condition value, one on the interrupt stack (VMS
// treats those as fatal system errors, not conditions), a program without
// the SYS$SRCHANDLER stub (no .P1VECTOR), or a stack it can't write. The
// engine then falls back to the console's own report.
func (env *Environment) DispatchException(code uint32, params []uint32, pc uint32, psl vax.PSL) (bool, error) {
	cond, detail, ok := exceptionCondition(code, params)
	if !ok || psl.IS() {
		return false, nil
	}

	c := env.cpu
	d := &conditionDispatch{
		kind:      kindException,
		resumeSP:  c.GPR(vax.SP),
		resumeFP:  c.GPR(vax.FP),
		resumePSL: psl,
	}

	sig := append(append([]uint32{cond}, detail...), pc, uint32(psl))

	if !env.startDispatch(d, sig, c.GPR(vax.R0), c.GPR(vax.R1)) {
		return false, nil
	}

	if c.DebugEnabled(vax.DebugExceptions) {
		fmt.Fprintf(c.DebugWriter(), "DEBUG(EXCEPTION): DISPATCH, CONDITION=%08X  PC=%08X  SIGNAL=%08X\n", cond, pc, d.sig)
	}

	return true, nil
}

// startDispatch lays out a dispatch's arrays on the current stack and
// sends execution to SYS$SRCHANDLER. sig is the signal array's contents
// after its count (the condition value, detail, PC, and PSL), and r0/r1
// the registers for the mechanism array. d's kind and resume fields are
// already set; the rest are filled in here.
//
// It reports false, having changed nothing, if the SYS$SRCHANDLER stub
// isn't there or the stack can't be written.
func (env *Environment) startDispatch(d *conditionDispatch, sig []uint32, r0, r1 uint32) bool {
	c := env.cpu

	if w, err := env.mem.LoadWord(c, srchandlerAddr); err != nil || w != xfcP1VectorWord {
		return false
	}

	// The layout, from the top of the stack (lowest address) up:
	//
	//	argList:  2, sig, mech
	//	mech:     4, frame, depth, R0, R1
	//	sig:      n, condition, detail..., PC, PSL
	sigBytes := uint32(4 * (len(sig) + 1))
	sp := c.GPR(vax.SP) &^ 3
	d.sig = sp - sigBytes
	d.mech = d.sig - 20
	d.argList = d.mech - 12
	d.sp = d.argList
	d.sigCount = uint32(len(sig))

	words := []uint32{2, d.sig, d.mech, 4, 0, 0, r0, r1, d.sigCount}
	words = append(words, sig...)

	for i, w := range words {
		if err := env.mem.StoreLongword(c, d.argList+uint32(4*i), w); err != nil {
			return false
		}
	}

	d.mode = c.PSL().CurMod()
	d.startFP = c.GPR(vax.FP)
	d.fp = d.startFP
	d.stage = stagePrimary

	env.Process.conditions = append(env.Process.conditions, d)

	c.SetGPR(vax.SP, d.sp)
	c.SetGPR(vax.PC, srchandlerAddr)

	return true
}

// currentDispatch returns the innermost dispatch in progress if SP is
// where its handlers return to (so this is its SYS$SRCHANDLER step), or
// nil.
func (env *Environment) currentDispatch() *conditionDispatch {
	n := len(env.Process.conditions)
	if n == 0 {
		return nil
	}

	d := env.Process.conditions[n-1]
	if d.sp != env.cpu.GPR(vax.SP) {
		return nil
	}

	return d
}

// endDispatch removes the innermost dispatch, which is finishing.
func (env *Environment) endDispatch() {
	p := env.Process
	p.conditions = p.conditions[:len(p.conditions)-1]
}

// serviceSysSrchandler is SYS$SRCHANDLER, the condition dispatcher's
// search (steps 2 and 3 of this file's opening comment). It isn't called
// with an argument list: execution jumps to it, with SP at the dispatch's
// arrays, and each handler returns to it.
//
// If a handler has just returned, its answer is acted on first; then the
// next handler is called, or, when there are none left, the catch-all
// acts.
func serviceSysSrchandler(env *Environment, _ []uint32) (uint32, error) {
	d := env.currentDispatch()
	if d == nil {
		return 0, fmt.Errorf("rtl: SYS$SRCHANDLER reached at SP %08X with no condition being dispatched there", env.cpu.GPR(vax.SP))
	}

	if d.calling {
		d.calling = false

		// A handler that called $UNWIND has its answer ignored: the
		// unwind happens instead (unwind.go).
		if d.unwind != nil {
			return env.continueUnwind(d)
		}

		if env.cpu.GPR(vax.R0)&1 != 0 {
			return env.continueCondition(d)
		}
	}

	return env.nextHandler(d)
}

// nextHandler moves d's search on to the next handler and calls it, or,
// when there are none, applies the catch-all.
func (env *Environment) nextHandler(d *conditionDispatch) (uint32, error) {
	for {
		switch d.stage {
		case stagePrimary:
			d.stage = stageSecondary

			if h := env.Process.exceptionVectors[d.mode][vectorPrimary]; h != 0 {
				return env.callConditionHandler(d, h, d.startFP, depthPrimary)
			}

		case stageSecondary:
			d.stage = stageFrames

			if h := env.Process.exceptionVectors[d.mode][vectorSecondary]; h != 0 {
				return env.callConditionHandler(d, h, d.startFP, depthSecondary)
			}

		case stageFrames:
			frame, depth := d.fp, d.depth
			if frame == 0 || frame == consoleCallSentinel {
				d.stage = stageLastChance

				continue
			}

			// A frame that can't be read ends the walk: VMS calls the
			// last-chance handler when "the stack is invalid".
			handler, err1 := env.mem.LoadLongword(env.cpu, frame)
			caller, err2 := env.mem.LoadLongword(env.cpu, frame+12)

			if err1 != nil || err2 != nil {
				d.stage = stageLastChance

				continue
			}

			d.fp, d.depth = caller, depth+1

			if handler != 0 {
				return env.callConditionHandler(d, handler, frame, depth)
			}

		case stageLastChance:
			d.stage = stageCatchAll

			if h := env.Process.exceptionVectors[d.mode][vectorLastChance]; h != 0 {
				return env.callConditionHandler(d, h, d.startFP, depthLastChance)
			}

		default:
			return env.catchAll(d)
		}
	}
}

// callConditionHandler calls handler for d with the mechanism array's
// frame and depth set to frame and depth.
func (env *Environment) callConditionHandler(d *conditionDispatch, handler, frame uint32, depth int32) (uint32, error) {
	c := env.cpu

	if env.mem.StoreLongword(c, d.mech+4, frame) != nil ||
		env.mem.StoreLongword(c, d.mech+8, uint32(depth)) != nil {
		return 0, fmt.Errorf("rtl: can't update the mechanism array at %08X", d.mech)
	}

	d.calling = true

	if c.DebugEnabled(vax.DebugExceptions) {
		fmt.Fprintf(c.DebugWriter(), "DEBUG(EXCEPTION): condition handler %08X, frame %08X, depth %d\n", handler, frame, depth)
	}

	return 0, &CallRequest{Routine: handler, ArgList: d.argList}
}

// sigWords reads d's signal array: its count and the longwords after it.
func (env *Environment) sigWords(d *conditionDispatch) ([]uint32, error) {
	n, err := env.mem.LoadLongword(env.cpu, d.sig)
	if err != nil {
		return nil, err
	}

	words := make([]uint32, n)
	for i := range words {
		if words[i], err = env.mem.LoadLongword(env.cpu, d.sig+uint32(4*(i+1))); err != nil {
			return nil, err
		}
	}

	return words, nil
}

// continueCondition ends d with execution continuing where the condition
// happened (a handler returned SS$_CONTINUE, or the catch-all decided the
// condition wasn't severe). R0 and R1 come from the mechanism array; the
// returned value is R0, which the XFC handler stores.
//
// For an exception, PC is the signal array's PC — a handler may have
// changed it, to skip the faulting instruction say — and the PSL's
// condition codes and trap enables (its low byte) come from the signal
// array's PSL; its mode, IPL, and the rest stay as they were, as REI
// would insist. For LIB$SIGNAL, execution returns from LIB$SIGNAL. A
// LIB$STOP can't be continued: the image exits instead.
func (env *Environment) continueCondition(d *conditionDispatch) (uint32, error) {
	c := env.cpu

	if d.kind == kindStop {
		return env.attemptToContinueFromStop(d)
	}

	r0, err1 := env.mem.LoadLongword(c, d.mech+12)
	r1, err2 := env.mem.LoadLongword(c, d.mech+16)
	pcAdr := d.sig + 4*(d.sigCount-1)
	pc, err3 := env.mem.LoadLongword(c, pcAdr)
	psl, err4 := env.mem.LoadLongword(c, pcAdr+4)

	if err := errors.Join(err1, err2, err3, err4); err != nil {
		return 0, fmt.Errorf("rtl: can't read the condition's arrays at %08X: %w", d.sig, err)
	}

	env.endDispatch()
	c.SetGPR(vax.R1, r1)
	c.SetGPR(vax.SP, d.resumeSP)
	c.SetGPR(vax.FP, d.resumeFP)

	switch d.kind {
	case kindException:
		c.SetGPR(vax.PC, pc)
		c.SetPSL(d.resumePSL&^0xFF | vax.PSL(psl&0xFF))

	default:
		c.SetGPR(vax.PC, d.resumePC)
	}

	return r0, nil
}

// catchAll is what happens when no handler continued (the search's
// stageCatchAll): VMS's catch-all handler. It writes the condition's
// message to the terminal, then either continues the program (a
// condition that isn't SEVERE, signaled by LIB$SIGNAL or the hardware)
// or ends the image through $EXIT with the condition value as the status
// (a SEVERE one, or any LIB$STOP), marked STS$M_INHIB_MSG because the
// message has been shown.
//
// The message is formatted from the signal array the handlers leave, as
// VMS's catch-all does: the condition and its detail, with the PC and
// PSL left off the vector but available as the last message's trailing
// $FAO parameters (see formatMessageVector), which is where the system
// exception messages take theirs from.
func (env *Environment) catchAll(d *conditionDispatch) (uint32, error) {
	words, err := env.sigWords(d)
	if err != nil || len(words) < 3 {
		return 0, fmt.Errorf("rtl: can't read the condition's signal array at %08X", d.sig)
	}

	cond := words[0]
	body, tail := words[:len(words)-2], words[len(words)-2:]

	for _, line := range env.formatMessageVector(body, tail, defaultMessageFlags, "") {
		env.writeConsole(line + "\n")
	}

	if d.kind == kindStop || cond&7 == severitySevere {
		return env.exitForCondition(d, cond)
	}

	return env.continueCondition(d)
}

// attemptToContinueFromStop is a handler continuing a LIB$STOP, which
// VMS refuses: it reports %LIB-F-ATTCONSTO and exits with the stop's
// condition.
func (env *Environment) attemptToContinueFromStop(d *conditionDispatch) (uint32, error) {
	cond, err := env.mem.LoadLongword(env.cpu, d.sig+4)
	if err != nil {
		return 0, fmt.Errorf("rtl: can't read the condition's signal array at %08X", d.sig)
	}

	for _, line := range env.formatMessageVector([]uint32{libAttConSto, 0}, nil, defaultMessageFlags, "") {
		env.writeConsole(line + "\n")
	}

	return env.exitForCondition(d, cond)
}

// exitForCondition ends d by ending the image: it calls the SYS$EXIT
// entry, as a program would, with cond (marked STS$M_INHIB_MSG) as the
// status, so exit handlers run as for any $EXIT. $EXIT's argument list
// goes on the stack below the dispatch's.
func (env *Environment) exitForCondition(d *conditionDispatch, cond uint32) (uint32, error) {
	c := env.cpu
	argList := d.sp - 8

	if env.mem.StoreLongword(c, argList, 1) != nil ||
		env.mem.StoreLongword(c, argList+4, cond|stsInhibitMsg) != nil {
		return 0, fmt.Errorf("rtl: can't push $EXIT's argument list at %08X", argList)
	}

	env.endDispatch()
	c.SetGPR(vax.SP, argList)

	return 0, &CallRequest{Routine: exitEntryAddr, ArgList: argList}
}

// serviceSysSetexv is SYS$SETEXV (docs/PHASE-26.md subtask 32):
//
//	SYS$SETEXV [vector] ,[addres] ,[acmode] ,[prvhnd]
//
// Each access mode has three *exception vectors*, condition handlers
// that don't belong to any call frame: the primary vector, searched
// first; the secondary, searched next; and the last-chance vector,
// searched after every call frame's handler has resignaled. Debuggers and
// performance monitors use them to see conditions before (or after) the
// program's own handlers do.
//
// $SETEXV sets vector (0 primary, 1 secondary, 2 last chance; 0 by
// default) of access mode acmode to the handler at addres, or clears it
// if addres is 0 or omitted, storing the handler it replaces at prvhnd
// if prvhnd isn't 0. acmode is maximized with the caller's mode: a
// program can't change a more privileged mode's vectors.
//
// It returns SS$_NORMAL, SS$_ACCVIO if prvhnd can't be written (nothing
// is changed), or SS$_BADPARAM for a vector number other than 0-2.
func serviceSysSetexv(env *Environment, argv []uint32) (uint32, error) {
	vector, addres, prvhnd := optArg(argv, 0), optArg(argv, 1), optArg(argv, 3)
	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

	if vector > vectorLastChance {
		return ssBadParam, nil
	}

	slot := &env.Process.exceptionVectors[mode][vector]

	if prvhnd != 0 {
		if err := env.mem.StoreLongword(env.cpu, prvhnd, *slot); err != nil {
			return ssAccVio, nil
		}
	}

	*slot = addres

	return ssNormal, nil
}

// cancelConditions is image rundown's condition step: dispatches the
// image never finished (it exited from a handler) are forgotten, and, as
// the manual says, the user-mode exception vectors are cleared.
func (env *Environment) cancelConditions() {
	env.Process.conditions = nil
	env.Process.exceptionVectors[vax.User] = [3]uint32{}
}

func registerConditionServices(t *ServiceTable) {
	t.RegisterNoArgs("SYS$SRCHANDLER", serviceSysSrchandler)
	t.Register("SYS$SETEXV", serviceSysSetexv)
	t.Register("SYS$UNWIND", serviceSysUnwind)
}
