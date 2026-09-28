package rtl

import (
	"github.com/tucats/govax/internal/vax"
)

// $CMKRNL and $CMEXEC (docs/PHASE-26.md subtask 26): call a routine in
// kernel or executive mode.
//
// # Why a program changes mode
//
// The VAX has four access modes, from most to least privileged: kernel
// (0), executive (1), supervisor (2), and user (3). Ordinary programs run
// in user mode, where the hardware keeps them from touching the operating
// system's memory or executing privileged instructions (MTPR, HALT, ...).
// A program that needs to do something privileged — read a processor
// register, change a system data structure — and whose user holds the
// CMKRNL (or CMEXEC) privilege can ask VMS to run one of its own routines
// in the more privileged mode:
//
//	SYS$CMKRNL routin ,[arglst]     call routin in kernel mode
//	SYS$CMEXEC routin ,[arglst]     call routin in executive mode
//
// VMS switches the process into that mode, calls the routine with CALLG
// arglst, routin, and, when the routine returns, switches back to the
// caller's mode and returns the routine's R0 as the service's status.
//
// # How govax does it
//
// This combines two earlier mechanisms. Changing mode is switchMode, as
// AST delivery uses it (ast.go): the stack pointer is saved in the
// current mode's stack-pointer register (USP, ...) and loaded from the
// new mode's (KSP, ...), and PSL<CUR_MOD> changes. Calling the routine is
// a *CallRequest, as $EXIT uses for exit handlers (exit.go): the engine
// calls it with the service's own XFC instruction as its return address.
//
// So a change-mode service runs twice:
//
//  1. Called by the program: switch into the target mode, remember the
//     call (a cmodeCall on Process.cmode), and return a CallRequest for
//     the routine. The engine builds the routine's call frame on the
//     target mode's stack.
//  2. The routine's RET returns to the XFC, which calls the service again.
//     The service recognizes its call in progress — the frame pointer is
//     the SYS$CMKRNL stub's again (RET restored it) and the stack pointer
//     is back where the call frame was built — switches back to the
//     caller's mode, and returns the routine's R0.
//
// The calls in progress are a stack, so a kernel routine may call
// $CMKRNL (or $CMEXEC) again.
//
// govax's process holds every privilege, so the services never fail
// with SS$_NOPRIV.

// cmodeCall is one change-mode call in progress.
type cmodeCall struct {
	// fp is the frame pointer of the service stub's frame, and sp the
	// stack pointer (in the target mode) when the routine was called:
	// both are the same again when the routine has returned.
	fp, sp uint32

	// caller is the mode to switch back to, and callerPrv the caller's
	// PSL<PRV_MOD>, restored with it. target is the mode the routine
	// runs in.
	caller, callerPrv, target vax.AccessMode
}

// serviceSysCmkrnl is SYS$CMKRNL:
//
//	SYS$CMKRNL routin ,[arglst]
//
// It calls routin in kernel mode with the argument list at arglst (AP is
// 0 if arglst is omitted), and returns the routine's R0. See this file's
// opening comment.
func serviceSysCmkrnl(env *Environment, argv []uint32) (uint32, error) {
	return env.changeMode(vax.Kernel, argv)
}

// serviceSysCmexec is SYS$CMEXEC:
//
//	SYS$CMEXEC routin ,[arglst]
//
// It is $CMKRNL for executive mode. As the manual says, called from
// kernel mode it runs the routine in kernel mode: a change-mode service
// never makes the process less privileged.
func serviceSysCmexec(env *Environment, argv []uint32) (uint32, error) {
	return env.changeMode(vax.Executive, argv)
}

// changeMode is the body of both services: mode is the one the service
// asks for.
func (env *Environment) changeMode(mode vax.AccessMode, argv []uint32) (uint32, error) {
	c := env.cpu
	p := env.Process
	psl := c.PSL()

	// The routine returning (step 2 of this file's opening comment)?
	if n := len(p.cmode); n > 0 {
		call := p.cmode[n-1]

		if call.fp == c.GPR(vax.FP) && call.sp == c.GPR(vax.SP) && call.target == psl.CurMod() {
			p.cmode = p.cmode[:n-1]

			if call.caller != call.target {
				env.switchMode(call.target, call.caller)
			}

			psl = c.PSL()
			psl.SetPrvMod(call.callerPrv)
			c.SetPSL(psl)

			return c.GPR(vax.R0), nil
		}
	}

	// A new call (step 1): the target is the more privileged (smaller)
	// of the requested mode and the caller's.
	routin, arglst := optArg(argv, 0), optArg(argv, 1)
	call := &cmodeCall{
		fp:        c.GPR(vax.FP),
		caller:    psl.CurMod(),
		callerPrv: psl.PrvMod(),
		target:    min(mode, psl.CurMod()),
	}

	if call.target != call.caller {
		env.switchMode(call.caller, call.target)

		psl = c.PSL()
		psl.SetPrvMod(call.caller)
		c.SetPSL(psl)
	}

	call.sp = c.GPR(vax.SP)
	p.cmode = append(p.cmode, call)

	return 0, &CallRequest{Routine: routin, ArgList: arglst}
}

// cancelChangeModeCalls is image rundown's change-mode step: a routine
// that never returned (it called $EXIT, say) leaves its call behind,
// which is forgotten.
func (env *Environment) cancelChangeModeCalls() {
	env.Process.cmode = nil
}

func registerChangeModeServices(t *ServiceTable) {
	t.Register("SYS$CMKRNL", serviceSysCmkrnl)
	t.Register("SYS$CMEXEC", serviceSysCmexec)
}
