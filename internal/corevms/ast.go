package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// AST delivery: the RTL's half (docs/PHASE-26.md subtask 15).
//
// # ASTs in brief
//
// An AST (asynchronous system trap) is a procedure call VMS makes on a
// program's behalf, *interrupting* it, when something it asked about
// happens: a timer expires ($SETIMR's astadr), a request completes
// ($GETJPI's), or the program simply asks for one ($DCLAST). The
// program continues afterwards as if nothing had happened. It's how VMS
// programs do asynchronous work without threads: start an operation,
// get on with something else (or $HIBER), and let an AST routine handle
// the completion.
//
// An AST belongs to an *access mode* (kernel 0, executive 1, supervisor
// 2, user 3): it runs in that mode. Each mode has its own queue of
// pending ASTs, its own "ASTs enabled" switch ($SETAST), and at most one
// AST running at a time — an AST routine is never interrupted by
// another AST of its own mode, so it needn't be reentrant.
//
// # When an AST is delivered
//
// Between any two instructions, the engine asks NextAST whether an AST
// can run now. An AST for mode m can run only if all of these hold, as
// on VMS:
//
//   - The CPU is running in mode m or a less privileged one. A user-mode
//     AST never interrupts kernel code, but a kernel-mode AST does
//     interrupt user code: the CPU is switched into kernel mode to run it
//     (see "Changing mode" below).
//   - IPL is below 2 (IPL$_ASTDEL) and the CPU isn't on the interrupt
//     stack. So an interrupt handler, or kernel code that raised IPL to
//     protect itself, is never interrupted by an AST.
//   - ASTs are enabled for mode m and every more privileged one.
//   - No AST of mode m is already running.
//
// Of the ASTs that could run, the most privileged mode's goes first, and
// within a mode the oldest.
//
// Because waiting services ($WAITFR, $HIBER) wait by re-executing their
// XFC instruction every step (see ErrWait), each retry is an instruction
// boundary where an AST can be delivered. The AST's saved PC is the XFC,
// so when the routine returns the wait resumes — VMS's "the wait is
// interrupted by the AST, then re-executed" — and if the routine called
// $WAKE or $SETEF, the re-executed wait is now satisfied.
//
// # The AST's stack frame
//
// NextAST pushes six longwords on the current stack, which together are
// both the routine's argument list and the saved state to restore:
//
//	SP+0   5          argument count
//	SP+4   astprm     the request's parameter
//	SP+8   R0         the interrupted program's registers ...
//	SP+12  R1
//	SP+16  PC         ... where it was ...
//	SP+20  PSL        ... and its processor status.
//
// (R0 and R1 are saved because the calling standard lets a procedure
// change them without saving them; every other register the routine
// changes is saved and restored by the call frame, from its entry mask.)
// The engine then calls the routine as CALLG (SP), routine would, with
// the return address set to the AST exit below.
//
// # How an AST returns
//
// The return address is the P1-vector entry SYS$CLRAST, plus 2: past
// the entry's (unused) procedure entry mask, at its XFC instruction. On
// VMS, SYS$CLRAST is the (undocumented) system service AST delivery
// returns through; govax uses its vector entry the same way. So the
// routine's RET lands on that XFC, which calls serviceSysClrast, which
// pops the six longwords and puts back R0, R1, PC, and PSL. Execution
// continues at the interrupted instruction.
//
// This needs the P1 vector's stubs in memory (assembled by .P1VECTOR, as
// any program calling system services has). NextAST checks the XFC is
// there before delivering.
//
// # Changing mode
//
// Each access mode has its own stack. When the AST's mode is more
// privileged than the CPU's (docs/PHASE-26.md subtask 22), NextAST first
// switches the CPU into it, as VMS does: the current stack pointer is
// saved in the current mode's stack-pointer register (USP, SSP, ESP),
// PSL<CUR_MOD> becomes the AST's mode, PSL<PRV_MOD> the interrupted one,
// and SP the AST mode's saved stack pointer (KSP, ...). The frame then goes
// on that stack, holding the interrupted, less privileged PSL. The AST
// exit sees that PSL's mode and switches back the same way, the
// equivalent of REI. An AST exit never switches to a *more* privileged
// mode than the AST's: a routine that forges its frame can't gain
// privilege.

// astDeliveryIPL is IPL$_ASTDEL: ASTs are delivered only below it.
const astDeliveryIPL = 2

// astArgCount is the number of arguments an AST routine receives
// (astprm, R0, R1, PC, PSL); astFrameSize the bytes NextAST pushes
// (those plus the count).
const (
	astArgCount  = 5
	astFrameSize = (astArgCount + 1) * 4
)

// astExitAddr is where an AST routine returns to: the XFC instruction in
// the SYS$CLRAST P1-vector entry (the entry address plus its 2-byte
// procedure entry mask). xfcP1VectorWord is that XFC as a little-endian
// word: opcode 0xFC, then selector 0x7A (XFC$P1VECTOR).
var astExitAddr = func() uint32 {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$CLRAST" {
			return e.Addr + 2
		}
	}

	panic("rtl: SYS$CLRAST missing from the P1 vector table")
}()

const xfcP1VectorWord = 0x7AFC

// astRequest is one queued AST (a VMS AST control block, ACB).
type astRequest struct {
	routine uint32 // the AST routine's entry mask address (astadr)
	param   uint32 // astprm, passed as its first argument
	mode    uint32 // the access mode it runs in
}

// astState is the process's AST bookkeeping, part of Process.
type astState struct {
	// queue holds the pending ASTs of every mode, oldest first (VMS
	// keeps one queue ordered by mode, PCB$L_ASTQFL; within a mode the
	// order is the same).
	queue []astRequest

	// enabled is each mode's $SETAST switch (PCB$B_ASTEN). All start
	// enabled.
	enabled [4]bool

	// active marks a mode whose AST is running (PCB$B_ASTACT), and
	// frame is where its six-longword frame (see above) starts: the AST
	// exit checks SP is back there before restoring from it.
	active [4]bool
	frame  [4]uint32
}

// newASTState returns a process's initial AST state: nothing queued or
// running, and ASTs enabled in every mode.
func newASTState() astState {
	return astState{enabled: [4]bool{true, true, true, true}}
}

// queueAST adds an AST for routine, with parameter param, to run in mode.
// It runs at the first instruction boundary where NextAST's conditions
// hold. The services that take an optional astadr call this when their
// event happens, if astadr isn't 0 ("no AST").
func (env *Environment) queueAST(routine, param, mode uint32) {
	p := env.Process
	p.ast.queue = append(p.ast.queue, astRequest{routine: routine, param: param, mode: mode & 3})
}

// PendingASTs reports how many ASTs are queued, in every mode.
func (env *Environment) PendingASTs() int { return len(env.Process.ast.queue) }

// NextAST is the engine's per-instruction question: can an AST run now?
// (cpu.ASTSource, reached through the console.) See this file's opening
// comment for the conditions. When one can, NextAST pushes its frame on
// the current stack, marks the mode's AST active, removes it from the
// queue, and returns what the engine needs to call it: the routine, its
// argument list (the new SP), and the address it returns to.
//
// Due timers are expired first, whatever the IPL, so a timer's event
// flag or wakeup (and its AST, subtask 16) happens on the tick it's due,
// not only when the program next calls a service.
//
// An error means the AST can't be delivered at all: the P1-vector stub
// it would return through isn't in memory, or its frame can't be
// pushed. It stops the engine; VMS would delete the process.
func (env *Environment) NextAST() (routine, argList, returnPC uint32, ok bool, err error) {
	env.expireTimers()

	p := env.Process
	if len(p.ast.queue) == 0 {
		return 0, 0, 0, false, nil
	}

	mode, i := p.deliverableAST(env.cpu.PSL())
	if i < 0 {
		return 0, 0, 0, false, nil
	}

	if w, err := env.mem.LoadWord(env.cpu, astExitAddr); err != nil || w != xfcP1VectorWord {
		return 0, 0, 0, false, fmt.Errorf("rtl: can't deliver an AST: no SYS$CLRAST stub at %08X to return through (the program needs .P1VECTOR)", astExitAddr)
	}

	a := p.ast.queue[i]

	sp, err := env.enterASTMode(vax.AccessMode(mode), a.param)
	if err != nil {
		return 0, 0, 0, false, err
	}

	p.ast.queue = append(p.ast.queue[:i], p.ast.queue[i+1:]...)
	p.ast.active[mode] = true
	p.ast.frame[mode] = sp

	return a.routine, sp, astExitAddr, true, nil
}

// deliverableAST finds the AST the process would take next if the CPU
// were at psl (see this file's opening comment for the conditions): its
// mode, and its index in the queue, or -1 if none can run. It changes
// nothing, so it can be asked about a process that isn't running.
func (p *Process) deliverableAST(psl vax.PSL) (mode uint32, index int) {
	if psl.IPL() >= astDeliveryIPL || psl.IS() {
		return 0, -1
	}

	// Look for the most privileged mode, from kernel out to psl's
	// current mode, with an AST that can run.
	for m := uint32(0); m <= uint32(psl.CurMod()); m++ {
		// Disabling ASTs in a mode also holds back every less privileged
		// mode's ASTs.
		if !p.ast.enabled[m] {
			break
		}

		if p.ast.active[m] {
			continue
		}

		for n, a := range p.ast.queue {
			if a.mode == m {
				return m, n
			}
		}
	}

	return 0, -1
}

// enterASTMode prepares the CPU to run an AST of mode (which is the
// CPU's current mode or a more privileged one), with parameter param: it
// switches into mode if it isn't already there (see "Changing mode"
// above), then pushes the six-longword AST frame (count, astprm, R0, R1,
// PC, PSL; see this file's opening comment) holding the interrupted
// state, and returns the frame's address, the new SP.
//
// If the frame can't be pushed, everything is put back as it was and the
// error is returned.
func (env *Environment) enterASTMode(mode vax.AccessMode, param uint32) (uint32, error) {
	c := env.cpu
	old, oldSP := c.PSL(), c.GPR(vax.SP)
	frame := [6]uint32{astArgCount, param, c.GPR(vax.R0), c.GPR(vax.R1), c.GPR(vax.PC), uint32(old)}

	if mode != old.CurMod() {
		env.switchMode(old.CurMod(), mode)

		psl := c.PSL()
		psl.SetPrvMod(old.CurMod())
		c.SetPSL(psl)
	}

	sp := c.GPR(vax.SP) - astFrameSize

	for i, v := range frame {
		if err := env.mem.StoreLongword(c, sp+uint32(i)*4, v); err != nil {
			stackSP := c.GPR(vax.SP)

			if mode != old.CurMod() {
				env.switchMode(mode, old.CurMod())
			}

			c.SetPSL(old)
			c.SetGPR(vax.SP, oldSP)

			return 0, fmt.Errorf("rtl: can't deliver an AST: its frame can't be pushed at SP=%08X: %w", stackSP, err)
		}
	}

	c.SetGPR(vax.SP, sp)

	return sp, nil
}

// switchMode moves the CPU from access mode from to access mode to, the
// way the VAX does when it changes mode: SP is saved in from's
// stack-pointer register and loaded from to's (KSP, ESP, SSP, and USP are
// processor registers 0-3, numbered like the modes), and PSL<CUR_MOD>
// changes. The memory system's cached access checks are discarded, since
// they were for the old mode. Nothing else in the PSL changes.
func (env *Environment) switchMode(from, to vax.AccessMode) {
	c := env.cpu

	c.SetPR(vax.PrivReg(from), c.GPR(vax.SP))
	c.SetGPR(vax.SP, c.PR(vax.PrivReg(to)))

	psl := c.PSL()
	psl.SetCurMod(to)
	c.SetPSL(psl)

	env.mem.InvalidateProtection()
}

// serviceSysClrast is SYS$CLRAST, the AST exit. An AST routine's RET
// returns here (see "How an AST returns" above) with SP at the AST
// frame NextAST pushed. It restores R1, PC, and PSL from the frame,
// removes the frame, ends the mode's active AST, and returns the saved R0
// — which the XFC handler then puts in R0, the last register restored.
//
// It is registered with RegisterNoArgs: it isn't reached by CALLS, so AP
// is still the interrupted program's, not an argument list.
//
// Reached any other way (a program calling SYS$CLRAST, or an AST routine
// that left its stack unbalanced), SP isn't at the active AST's frame;
// then nothing is restored and it returns SS$_NORMAL.
//
// When the saved PSL's mode is less privileged than the AST's — the AST
// interrupted an outer mode — the CPU switches back to it, stacks and all
// (see "Changing mode" above). A saved mode *more* privileged than the
// AST's is ignored, as is the saved interrupt-stack bit: a PSL from the
// stack must never be able to raise the program's privilege.
func serviceSysClrast(env *Environment, _ []uint32) (uint32, error) {
	c := env.cpu
	p := env.Process
	cur := c.PSL()
	mode := uint32(cur.CurMod())
	sp := c.GPR(vax.SP)

	if !p.ast.active[mode] || sp != p.ast.frame[mode] {
		return ssNormal, nil
	}

	var frame [6]uint32

	for i := range frame {
		v, err := env.mem.LoadLongword(c, sp+uint32(i)*4)
		if err != nil {
			return ssNormal, nil
		}

		frame[i] = v
	}

	if frame[0] != astArgCount {
		return ssNormal, nil
	}

	psl := vax.PSL(frame[5])
	back := max(psl.CurMod(), cur.CurMod()) // never more privileged than the AST

	c.SetGPR(vax.R1, frame[3])
	c.SetGPR(vax.PC, frame[4])
	c.SetGPR(vax.SP, sp+astFrameSize)

	if back != cur.CurMod() {
		env.switchMode(cur.CurMod(), back)
	}

	psl.SetCurMod(back)
	psl.SetIS(cur.IS())
	c.SetPSL(psl)

	p.ast.active[mode] = false

	return frame[2], nil
}

// serviceSysDclast is SYS$DCLAST:
//
//	SYS$DCLAST astadr ,[astprm] ,[acmode]
//
// It queues an AST for routine astadr with parameter astprm, in acmode
// maximized with the caller's mode — so a program can queue an AST for
// its own mode or a less privileged one. It always succeeds: as the
// manual says, astadr isn't checked, and a bad one faults when the AST
// is delivered. An AST for the caller's own mode is typically delivered
// right after the call returns, before the next instruction.
//
// Not implemented: the ASTLM quota (SS$_EXQUOTA); SS$_INSFMEM can't
// happen.
func serviceSysDclast(env *Environment, argv []uint32) (uint32, error) {
	astadr, astprm := optArg(argv, 0), optArg(argv, 1)
	mode := max(optArg(argv, 2)&3, uint32(env.cpu.PSL().CurMod()))

	env.queueAST(astadr, astprm, mode)

	return ssNormal, nil
}

// serviceSysSetast is SYS$SETAST:
//
//	SYS$SETAST enbflg
//
// It enables (enbflg's low byte nonzero) or disables AST delivery for the
// caller's access mode, returning SS$_WASSET if they were enabled before
// and SS$_WASCLR if not. Queued ASTs stay queued while disabled, and are
// delivered once ASTs are enabled again. Disabling ASTs is how a program
// protects a critical section — code that shares data with its AST
// routines — from being interrupted.
func serviceSysSetast(env *Environment, argv []uint32) (uint32, error) {
	mode := env.cpu.PSL().CurMod()
	p := env.Process

	was := p.ast.enabled[mode]
	p.ast.enabled[mode] = byte(optArg(argv, 0)) != 0

	if was {
		return ssWasSet, nil
	}

	return ssWasClr, nil
}

// flushUserASTs is image rundown's AST step: user-mode ASTs belong to
// the image that exits, so any still queued are discarded, and user
// mode starts the next image with ASTs enabled and none running.
func (env *Environment) flushUserASTs() {
	p := env.Process
	remaining := p.ast.queue[:0]

	for _, a := range p.ast.queue {
		if a.mode != uint32(vax.User) {
			remaining = append(remaining, a)
		}
	}

	p.ast.queue = remaining
	p.ast.enabled[vax.User] = true
	p.ast.active[vax.User] = false
}

func registerASTServices(t *ServiceTable) {
	t.Register("SYS$DCLAST", serviceSysDclast)
	t.Register("SYS$SETAST", serviceSysSetast)
	t.RegisterNoArgs("SYS$CLRAST", serviceSysClrast)
}
