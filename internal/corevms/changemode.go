package corevms

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// Change mode handlers and system service failure exceptions: $DCLCMH and
// $SETSFM, the two services that change how VMS reports something to a
// program rather than doing something for it.
//
// # Change mode handlers ($DCLCMH)
//
// The VAX has four change-mode instructions, CHMK, CHME, CHMS, and CHMU,
// each a trap into its mode (or the current one, if that's more
// privileged) through its own vector in the system control block. VMS
// uses CHMK and CHME for its system services; CHMS and CHMU are left to
// programs. A program, or a command interpreter in supervisor mode (DCL
// uses one to get from an image's user mode back to its own), can declare
// a *change mode handler* for the mode it calls $DCLCMH from:
//
//	SYS$DCLCMH addres ,[prvhnd] ,[type]
//
// A CHMS then goes to the supervisor-mode handler, and a CHMU to the
// user-mode one, with the trap's stack exactly as the hardware left it:
// the change mode code on top, then the PC of the instruction after the
// CHMx, then the PSL. The handler removes the code and returns with REI
// (*VAX/VMS Internals and Data Structures*, 4.2.1.4 item 4 and 9.2.2; the
// VSI System Services manual's $DCLCMH). With no handler, the trap is a
// condition, SS$_CMODSUPR or SS$_CMODUSER, the code its one detail
// longword (the book's Table 4-2), reported to the mode that executed the
// instruction like any exception (condition.go).
//
// govax's microkernel sends both vectors to console$handler, and the
// engine asks the running process for a handler (ChangeModeHandler)
// before dispatching the condition, and if there is one delivers the trap
// to it as the hardware delivers the trap, at the handler's exact address
// (cpu.ChangeModeDispatcher). The handlers
// are kept in Go, not in P1 (CTL$GL_CMSUPR and CTL$GL_CMUSER on VMS).
// Image rundown forgets the user-mode one; the supervisor-mode one lasts
// as long as the process (the book's 21.3, step 16).
//
// A type of 1 declares a *compatibility mode* handler, for PDP-11 code.
// govax has no compatibility mode, so it's kept, reported to the next
// $DCLCMH, and never called.
//
// # System service failure exceptions ($SETSFM)
//
// A program can ask that a system service that fails, returning an error
// or severe status, *signal* SS$_SSFAIL instead of just returning the
// status, so that it needn't check every call:
//
//	SYS$SETSFM [enbflg]
//
// VMS keeps a bit per access mode (PCB$V_SSFEXC and its neighbors, the
// book's Table 12-2); $SETSFM sets or clears the caller's mode's bit and
// returns SS$_WASSET or SS$_WASCLR for what it was. $CREPRC with
// PRC$M_SSFEXCU starts a process with the user-mode bit set, and image
// rundown clears it.
//
// VMS raises the exception in the common exit path of the services that
// change mode, SRVEXIT, which RMS's services reach too (through $SSVEXC)
// when they fail: an error or severe status (low bit clear, severity not
// a warning) in a mode whose bit is set (the book's 9.3.5 and 9.3.6.2).
// The services that run in the caller's mode, never passing through it
// ($FAO, $GETMSG, $PUTMSG, $UNWIND, and the rest of the book's Table
// 9-1's last group), don't. The signal array is SS$_SSFAIL, the status,
// and the PC and PSL of the change-mode trap: the PC is the instruction
// after the CHMx in the vector, its RET. A handler that continues the
// condition resumes there, so the service returns with R0 from the
// mechanism array, the status unless the handler changed it.
//
// govax raises it in SystemService, after a service returns its final
// status (failureException), the PC the RET after the vector entry's XFC.

// Status values these services use.
var (
	ssWasClrStatus = vmsdef.Symbols["SS$_WASCLR"]
	ssWasSetStatus = vmsdef.Symbols["SS$_WASSET"]
	ssSSFail       = vmsdef.Symbols["SS$_SSFAIL"]
	prcSSFEXCU     = vmsdef.Symbols["PRC$M_SSFEXCU"]
)

// callerModeServices are the services VMS runs in the caller's mode,
// without a change mode instruction, so whose failure never reaches the
// exit path that raises SS$_SSFAIL (the book's Table 9-1), and the
// vector entries that aren't calls at all (the condition search, the AST
// exit, and $EXIT, which doesn't return).
var callerModeServices = map[string]bool{
	"SYS$ASCTIM":     true,
	"SYS$BINTIM":     true,
	"SYS$EXCMSG":     true,
	"SYS$FAO":        true,
	"SYS$FAOL":       true,
	"SYS$GETMSG":     true,
	"SYS$IMGFIX":     true,
	"SYS$IMGSTA":     true,
	"SYS$PUTMSG":     true,
	"SYS$UNWIND":     true,
	"SYS$SRCHANDLER": true,
	"SYS$CLRAST":     true,
	"SYS$EXIT":       true,
}

// serviceSysDclcmh is SYS$DCLCMH:
//
//	SYS$DCLCMH addres ,[prvhnd] ,[type]
//
// It makes addres the change mode handler of the caller's mode (type 0,
// the default) or the compatibility mode handler (type 1), 0 clearing it,
// first writing the handler it replaces to prvhnd if prvhnd isn't 0
// (SS$_ACCVIO if it can't be written, nothing changed). Only supervisor
// and user modes have change mode handlers; a call from kernel or
// executive mode is SS$_IVSSRQ (govax's choice: the manual says only
// that the service may be called from those two modes).
func serviceSysDclcmh(env *Environment, argv []uint32) (uint32, error) {
	addres, prvhnd, typ := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2)
	mode := env.cpu.PSL().CurMod()

	if mode < registers.Supervisor {
		return ssIvSsRq, nil
	}

	slot := &env.Process.changeModeHandlers[mode]
	if typ&1 != 0 {
		slot = &env.Process.compatibilityHandler
	}

	if prvhnd != 0 {
		if err := env.mem.StoreLongword(env.cpu, prvhnd, *slot); err != nil {
			return ssAccVio, nil
		}
	}

	*slot = addres

	return ssNormal, nil
}

// ChangeModeHandler returns the handler $DCLCMH declared for CHMS (mode
// supervisor) or CHMU (mode user), or 0 if there's none.
func (env *Environment) ChangeModeHandler(mode registers.AccessMode) uint32 {
	if mode < registers.Supervisor || mode > registers.User {
		return 0
	}

	return env.Process.changeModeHandlers[mode]
}

// serviceSysSetsfm is SYS$SETSFM:
//
//	SYS$SETSFM [enbflg]
//
// It enables (enbflg 1) or disables (0, the default) system service
// failure exceptions for the caller's access mode, returning SS$_WASSET
// if they were enabled before the call and SS$_WASCLR if not.
func serviceSysSetsfm(env *Environment, argv []uint32) (uint32, error) {
	bit := &env.Process.failureExceptions[env.cpu.PSL().CurMod()]

	status := ssWasClrStatus
	if *bit {
		status = ssWasSetStatus
	}

	*bit = optArg(argv, 0)&1 != 0

	return status, nil
}

// failureException raises SS$_SSFAIL for the service name, whose vector
// entry is at entry, if it has just failed with status and its caller's
// mode has failure exceptions enabled (see this file's opening comment).
// It reports whether it did: the condition dispatch is then under way,
// PC at SYS$SRCHANDLER, and the service's XFC is to leave R0 the status.
// A stack the arrays can't be written on raises nothing, and the service
// returns its status, as without the exception.
func (env *Environment) failureException(name string, entry, status uint32) bool {
	c := env.cpu

	if status&1 != 0 || status&7 == 0 || callerModeServices[name] ||
		!env.Process.failureExceptions[c.PSL().CurMod()] {
		return false
	}

	sp := c.GPR(registers.SP)
	d := &conditionDispatch{
		kind:        kindException,
		resumeSP:    sp,
		resumeFP:    c.GPR(registers.FP),
		resumePSL:   c.PSL(),
		conditionSP: sp,
	}

	// The vector entry is an entry mask, the XFC, and RET: the trap's PC
	// is the RET's, after the XFC.
	sig := []uint32{ssSSFail, status, entry + 4, uint32(c.PSL())}

	return env.startDispatch(d, sig, status, c.GPR(registers.R1))
}

// FailureExceptions reports whether mode has system service failure
// exceptions enabled ($SETSFM, or $CREPRC's PRC$M_SSFEXCU for user mode).
func (env *Environment) FailureExceptions(mode registers.AccessMode) bool {
	return env.Process.failureExceptions[mode]
}

// cancelUserChangeMode is image rundown's step for these services: the
// user-mode change mode and compatibility mode handlers are forgotten,
// and user-mode failure exceptions are disabled.
func (env *Environment) cancelUserChangeMode() {
	p := env.Process
	p.changeModeHandlers[registers.User] = 0
	p.compatibilityHandler = 0
	p.failureExceptions[registers.User] = false
}

func registerChangeModeHandlerServices(t *ServiceTable) {
	t.Register("SYS$DCLCMH", serviceSysDclcmh)
	t.Register("SYS$SETSFM", serviceSysSetsfm)
}
