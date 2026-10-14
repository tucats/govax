package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// Routines that call back into the program (docs/PHASE-58, subtask 2;
// the survey's "The callbacks").
//
// # What a callback is
//
// Some LIB$ routines call procedures the program gives them:
// LIB$FILE_SCAN calls its success routine for each file it finds,
// LIB$DELETE_FILE asks a confirm routine about each file before deleting
// it, and so on. The program passes the procedure's address (its entry
// mask's), and the LIB$ routine calls it as CALLG would, then looks at
// the R0 it returns and goes on, perhaps calling it again many times
// before it returns to the program itself.
//
// govax's LIB$ routines are Go functions (shims), reached through a stub
// in the program's address space:
//
//	stub:  .WORD  0             ; entry mask
//	       MOVL   #code, R0     ; which routine
//	       XFC    #XFC$SHIM     ; run it, in Go
//	       RET                  ; back to the program, R0 the status
//
// Go can't call VAX code directly: only the engine runs instructions, and
// it's stopped in the XFC while the routine runs. So the routine asks
// the engine to make the call (a *CallRequest), and the engine builds the
// procedure's call frame with the XFC as its return address. When the
// procedure returns, its RET lands on the XFC, which calls the shim
// dispatch (Environment.Shim) again, with the procedure's R0.
//
// # Finding the routine again
//
// A service run again this way is found by its XFC's address, but a shim
// by R0, and R0 is now the procedure's result. So Shim first looks for a
// routine waiting for a callback (a shimCall on Process.shimCalls) that
// this XFC belongs to: the frame pointer is the stub's frame again (the
// procedure's RET restored it), the stack pointer is where it was when
// the procedure was called, and PC is just past the same XFC. Nothing
// else can execute that XFC with that FP: a callback that calls the same
// LIB$ routine again does so through a new stub frame, with its own FP.
// This is how $CMKRNL recognizes its routine returning (cmode.go).
//
// # Writing the routine as a loop
//
// A routine like LIB$DELETE_FILE calls out from several places in its
// loop (confirm, success, error). Rather than write each as a state
// machine re-entered after every callback, a routine registered with
// ShimTable.RegisterCallback runs on a goroutine of its own, and a
// callback is a Go call, Environment.Callback, which returns the
// procedure's R0. Behind it, the goroutine and the engine take turns
// (a strict handoff over two channels, as the subprocess CLI does,
// internal/dclshell/subcli.go): Callback hands the dispatch the request
// and waits; the dispatch returns the CallRequest to the engine; when
// the XFC runs again, the dispatch hands the goroutine the R0 and waits
// for its next request or its result. Only one of the two runs at any
// time, so the machine stays deterministic and nothing needs a lock.
//
// The callback's argument list goes on the stack below SP, in the stub's
// frame, and SP moves below it before the engine builds the frame
// there. Data the routine passes that must last across callbacks (a
// descriptor of the file name, say) it puts below SP itself; the stub's
// RET gives all of it back when the routine returns.
//
// # A callback that doesn't come back
//
// A procedure may never return to the routine: a condition handler may
// unwind past the stub's frame ($UNWIND, LIB$SIG_TO_RET), or the image
// may exit. The routine is then abandoned: its pending Callback panics
// with a sentinel the goroutine recovers, so the routine's own code needs
// no check (its deferred functions run, and mustn't change the CPU's
// registers), and the goroutine ends. $UNWIND abandons the calls whose
// stub frames it removes (finishUnwind), image rundown all of them
// (cancelShimCalls), and as a backstop, the dispatch abandons any whose
// stack has been popped past (a call left behind by some other way of
// leaving a frame).
//
// A routine registered this way can't wait (ErrWait) once it has made a
// callback, since the XFC run again would start it over: it fails
// instead. None of this phase's routines waits.

// shimCall is one call of a routine that calls back, from its start
// until it returns.
type shimCall struct {
	// name is the routine's, for errors.
	name string

	// fp, sp, and pc are the stub's frame pointer, the stack pointer
	// when the callback was called, and PC (just past the XFC) then,
	// mode the access mode: all four the same again when the callback
	// has returned.
	fp, sp, pc uint32
	mode       registers.AccessMode

	// resume carries the callback's R0 (or the word to give up) to the
	// routine's goroutine, and yield what the goroutine wants next.
	resume chan shimResume
	yield  chan shimYield

	// calledBack is true once the routine has made a callback, and
	// pending while it is on Process.shimCalls waiting for one.
	// abandoned is true once it has been told to give up.
	calledBack, pending, abandoned bool
}

// shimResume is what a shimCall's goroutine is given when it runs again.
type shimResume struct {
	r0      uint32
	abandon bool
}

// shimYield is what the goroutine hands back: a callback to make (call
// non-nil), or its result.
type shimYield struct {
	call *CallRequest
	r0   uint32
	err  error
}

// errShimAbandoned is the panic that ends an abandoned routine's pending
// Callback (see this file's opening comment).
var errShimAbandoned = errors.New("corevms: routine abandoned during a callback")

// Callback calls the procedure whose entry mask is at routine, with args
// as its argument list (CALLG's: a count, then the longwords), and
// returns the procedure's R0. Only a routine registered with
// ShimTable.RegisterCallback may call it, from the goroutine the routine
// runs on; anywhere else it panics, a mistake in the caller. An
// argument list it can't write, or an entry mask it can't read, is the
// memory error, and nothing is called.
func (env *Environment) Callback(routine uint32, args ...uint32) (uint32, error) {
	call := env.Process.runningShim
	if call == nil {
		panic("corevms: Callback outside a routine registered with RegisterCallback")
	}

	if call.abandoned {
		panic(errShimAbandoned)
	}

	c, mem := env.cpu, env.mem

	if _, err := mem.LoadWord(c, routine); err != nil {
		return 0, err
	}

	sp := c.GPR(registers.SP)
	argList := (sp - 4*uint32(len(args)+1)) &^ 3

	for i, v := range append([]uint32{uint32(len(args))}, args...) {
		if err := mem.StoreLongword(c, argList+4*uint32(i), v); err != nil {
			return 0, err
		}
	}

	c.SetGPR(registers.SP, argList)

	call.yield <- shimYield{call: &CallRequest{Routine: routine, ArgList: argList}}

	r := <-call.resume
	if r.abandon {
		call.abandoned = true

		panic(errShimAbandoned)
	}

	// The procedure's RET left SP at the argument list; the list goes.
	c.SetGPR(registers.SP, sp)

	return r.r0, nil
}

// CallingBack reports whether the Go code running now is a routine on a
// goroutine of its own, so that it may call Callback: one registered
// with RegisterCallback, or one RunCallingBack started.
func (env *Environment) CallingBack() bool {
	return env.Process.runningShim != nil
}

// RunCallingBack runs fn, a routine registered with Register (not
// RegisterCallback), on a goroutine of its own, as if it had been
// registered with RegisterCallback, and returns what the routine's shim
// returns. It's for a routine that only sometimes calls back, and finds
// out it must after it has started: LIB$GET_VM calls the program's
// get-page routine only for a zone made with one (docs/PHASE-59, the
// phase doc's Decision 4). Such a routine, having changed nothing yet,
// returns RunCallingBack(name, itself, argv) unless CallingBack, and its
// second run, on the goroutine, can call Callback. Every other call of
// the routine runs as a plain shim and pays nothing.
func (env *Environment) RunCallingBack(name string, fn ShimFunc, argv []uint32) (uint32, error) {
	return env.startShimCall(name, fn, argv)
}

// startShimCall runs fn, a routine registered with RegisterCallback, on
// a goroutine of its own, and returns what Shim returns: its result, or
// the first callback it asks for. The goroutine waits for its first turn
// like any other (handOff), so that it only ever runs while the
// dispatch waits for it.
func (env *Environment) startShimCall(name string, fn ShimFunc, argv []uint32) (uint32, error) {
	call := &shimCall{name: name, resume: make(chan shimResume), yield: make(chan shimYield)}

	go func() {
		var y shimYield

		defer func() {
			if p := recover(); p != nil {
				if p == errShimAbandoned { //nolint:errorlint // the panic value itself, not a wrapped error
					y = shimYield{err: errShimAbandoned}
				} else {
					y = shimYield{err: vmserrors.New(vmserrors.LIB_PANIC, fmt.Sprintf("%v", p))}
				}
			}

			call.yield <- y
		}()

		<-call.resume

		r0, err := fn(env, argv)
		y = shimYield{r0: r0, err: err}
	}()

	return env.endTurn(call, env.handOff(call, shimResume{}))
}

// handOff gives call's goroutine its turn, with r, and waits until it
// hands the CPU back: with a callback to make, or its result. While it
// runs it is the process's running routine (Callback's).
func (env *Environment) handOff(call *shimCall, r shimResume) shimYield {
	p := env.Process

	outer := p.runningShim
	p.runningShim = call
	call.resume <- r
	y := <-call.yield
	p.runningShim = outer

	return y
}

// endTurn returns what Shim returns at the end of call's turn y: a
// callback, which it records and returns as a CallRequest, or the
// routine's result.
func (env *Environment) endTurn(call *shimCall, y shimYield) (uint32, error) {
	if y.call != nil {
		c := env.cpu
		call.fp, call.sp, call.pc = c.GPR(registers.FP), c.GPR(registers.SP), c.GPR(registers.PC)
		call.mode = c.PSL().CurMod()
		call.calledBack = true

		if !call.pending {
			call.pending = true
			env.Process.shimCalls = append(env.Process.shimCalls, call)
		}

		return 0, y.call
	}

	env.dropShimCall(call)

	if errors.Is(y.err, ErrWait) && call.calledBack {
		return 0, fmt.Errorf("corevms: %s can't wait after calling back", call.name)
	}

	return y.r0, y.err
}

// dropShimCall takes call off the process's list.
func (env *Environment) dropShimCall(call *shimCall) {
	if !call.pending {
		return
	}

	p := env.Process
	call.pending = false

	for i, c := range p.shimCalls {
		if c == call {
			p.shimCalls = append(p.shimCalls[:i], p.shimCalls[i+1:]...)

			break
		}
	}
}

// returningShimCall returns the routine the XFC now executing belongs to,
// when it is a callback returning to it (see this file's opening
// comment), first abandoning any call whose stack has been popped past.
func (env *Environment) returningShimCall() *shimCall {
	p := env.Process
	if len(p.shimCalls) == 0 {
		return nil
	}

	c := env.cpu
	fp, sp, pc, mode := c.GPR(registers.FP), c.GPR(registers.SP), c.GPR(registers.PC), c.PSL().CurMod()

	// A call in this mode whose callback's stack pointer is below SP has
	// had its frame removed: nothing will return to it.
	for n := len(p.shimCalls); n > 0; n-- {
		call := p.shimCalls[n-1]
		if call.mode != mode || call.sp >= sp {
			break
		}

		env.abandonShimCall(call)
	}

	if n := len(p.shimCalls); n > 0 {
		call := p.shimCalls[n-1]
		if call.fp == fp && call.sp == sp && call.pc == pc && call.mode == mode {
			return call
		}
	}

	return nil
}

// resumeShimCall hands call's goroutine the callback's R0, and returns
// what Shim returns: the routine's next callback or its result.
func (env *Environment) resumeShimCall(call *shimCall, r0 uint32) (uint32, error) {
	return env.endTurn(call, env.handOff(call, shimResume{r0: r0}))
}

// abandonShimCall tells call's routine to give up, and waits until its
// goroutine has ended.
func (env *Environment) abandonShimCall(call *shimCall) {
	env.dropShimCall(call)
	env.handOff(call, shimResume{abandon: true})
}

// abandonShimCallsBelow abandons the calls whose stub frames are below
// fp, in the current mode: $UNWIND has removed them (finishUnwind, whose
// surviving frame is fp).
func (env *Environment) abandonShimCallsBelow(fp uint32) {
	p := env.Process
	mode := env.cpu.PSL().CurMod()

	for n := len(p.shimCalls); n > 0; n-- {
		call := p.shimCalls[n-1]
		if call.mode != mode || call.fp >= fp {
			break
		}

		env.abandonShimCall(call)
	}
}

// cancelShimCalls is image rundown's step for routines that call back:
// any still waiting for a callback is abandoned.
func (env *Environment) cancelShimCalls() {
	p := env.Process

	for n := len(p.shimCalls); n > 0; n = len(p.shimCalls) {
		env.abandonShimCall(p.shimCalls[n-1])
	}
}
