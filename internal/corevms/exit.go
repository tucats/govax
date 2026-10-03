package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Image exit and exit handlers (docs/PHASE-26.md subtask 19): $EXIT,
// $DCLEXH, and $CANEXH.
//
// # Exit handlers
//
// A program can ask VMS to call a procedure of its own when its image
// exits, to tidy up — flush a buffer, close a file, restore a terminal
// setting — however the exit happens. It describes the procedure in an
// *exit control block* in its own memory and passes that block's
// address to $DCLEXH:
//
//	desblk+0    forward link (VMS's; the program leaves it alone)
//	desblk+4    the exit handler's address (its entry mask)
//	desblk+8    the argument count, in the low byte (the rest must be 0)
//	desblk+12   the first argument: the address of a longword that VMS
//	            fills with the exit status before calling the handler
//	desblk+16   further arguments, if the count says so
//
// From desblk+8 on, the block is an ordinary VAX argument list, so VMS
// calls the handler as CALLG desblk+8, handler.
//
// VMS keeps one list of blocks per access mode, chained through their
// forward links, newest first; govax keeps the same lists as Go slices
// (Process.exitHandlers) and also maintains the links in memory, as the
// program may look at them. $CANEXH takes a block off its list.
//
// # $EXIT
//
// $EXIT never returns. It calls the handlers the caller's mode declared,
// newest first, each once (it's removed from the list before the call),
// then ends the image. The engine does the calling: $EXIT returns a
// *CallRequest for one handler, the engine calls it with $EXIT's own XFC
// instruction as the return address, and when the handler returns the
// XFC runs $EXIT again, for the next handler. When none are left, $EXIT
// returns ErrExit, and the engine ends the image by returning from the
// console's call frame, with R0 holding the exit status (see
// internal/cpu/exit.go). Since each call removes the handler it calls, a
// handler that itself calls $EXIT simply moves on to the next one.
//
// VMS images normally end by returning from their main routine, not by
// calling $EXIT; VMS's image activator then calls $EXIT with the value
// main returned. RUN's image driver (internal/console) does the same.

// CallRequest is the error a service returns to have the engine call a
// guest procedure before the service continues: Routine is the entry
// mask's address and ArgList the argument list, as for CALLG. The
// procedure returns to the service's XFC, calling the service again. The
// console turns it into a cpu.ServiceCall. $EXIT uses it to call exit
// handlers.
type CallRequest struct {
	Routine uint32
	ArgList uint32
}

func (r *CallRequest) Error() string {
	return fmt.Sprintf("rtl: service calls %08X", r.Routine)
}

// ErrExit is what $EXIT returns once the image has finished exiting,
// with the exit status as its R0. The console turns it into
// cpu.ErrImageExit, which ends the image.
var ErrExit = errors.New("rtl: image exit")

// Status codes the exit services return.
var (
	ssIvSsRq    = vmsdef.Symbols["SS$_IVSSRQ"]
	ssNoHandler = vmsdef.Symbols["SS$_NOHANDLER"]
)

// Offsets in an exit control block (see this file's opening comment).
const (
	exhLink      = 0
	exhHandler   = 4
	exhArgList   = 8
	exhStatusAdr = 12
)

// serviceSysDclexh is SYS$DCLEXH:
//
//	SYS$DCLEXH desblk
//
// It adds the exit control block at desblk to the front of the caller's
// access mode's list, writing the previous front block's address (0 if
// none) to its forward link. Kernel mode has no exit handlers
// (SS$_IVSSRQ). desblk of 0 is SS$_NOHANDLER, and an unwritable forward
// link SS$_ACCVIO.
func serviceSysDclexh(env *Environment, argv []uint32) (uint32, error) {
	desblk := optArg(argv, 0)
	mode := env.cpu.PSL().CurMod()

	if mode == vax.Kernel {
		return ssIvSsRq, nil
	}

	if desblk == 0 {
		return ssNoHandler, nil
	}

	list := &env.Process.exitHandlers[mode]

	next := uint32(0)
	if n := len(*list); n > 0 {
		next = (*list)[n-1]
	}

	if err := env.mem.StoreLongword(env.cpu, desblk+exhLink, next); err != nil {
		return ssAccVio, nil
	}

	*list = append(*list, desblk)

	return ssNormal, nil
}

// serviceSysCanexh is SYS$CANEXH:
//
//	SYS$CANEXH [desblk]
//
// It removes the exit control block at desblk from the caller's access
// mode's list, so its handler won't be called. The block declared just
// after it, whose forward link points to it, is relinked past it
// (SS$_ACCVIO if the links can't be read or written). A block that isn't
// on the list is SS$_NOHANDLER. With desblk omitted or 0, every block
// for the mode is removed. Kernel mode is SS$_IVSSRQ, as for $DCLEXH.
func serviceSysCanexh(env *Environment, argv []uint32) (uint32, error) {
	desblk := optArg(argv, 0)
	mode := env.cpu.PSL().CurMod()

	if mode == vax.Kernel {
		return ssIvSsRq, nil
	}

	list := &env.Process.exitHandlers[mode]

	if desblk == 0 {
		*list = nil

		return ssNormal, nil
	}

	i := -1

	for n, b := range *list {
		if b == desblk {
			i = n
		}
	}

	if i < 0 {
		return ssNoHandler, nil
	}

	// The list is oldest first, so the block declared after this one —
	// the one whose forward link points here — is the next entry.
	if i+1 < len(*list) {
		link, err := env.mem.LoadLongword(env.cpu, desblk+exhLink)
		if err != nil {
			return ssAccVio, nil
		}

		if err := env.mem.StoreLongword(env.cpu, (*list)[i+1]+exhLink, link); err != nil {
			return ssAccVio, nil
		}
	}

	*list = append((*list)[:i], (*list)[i+1:]...)

	return ssNormal, nil
}

// serviceSysExit is SYS$EXIT:
//
//	SYS$EXIT [code]
//
// It ends the image with completion status code, which is saved in
// Process.ExitStatus. A call with no arguments at all passes SS$_NORMAL,
// as the MACRO-32 $EXIT_S macro does when code is omitted.
//
// Each call either asks the engine to call the next exit handler of the
// caller's mode (a *CallRequest; see this file's opening comment), or,
// when none are left, reports ErrExit with code as R0. Before a handler is
// called, the status is stored at the address in its block's first
// argument (if it has one and that address can be written). A block
// whose handler address can't be read is skipped.
//
// Only the caller's mode's handlers are called: VMS would then run the
// supervisor- and executive-mode handlers in their own modes, but govax
// images have none (docs/DEVIATIONS.md). $EXIT from kernel mode has no
// handlers to call and ends the image at once.
func serviceSysExit(env *Environment, argv []uint32) (uint32, error) {
	p := env.Process

	status := uint32(ssNormal)
	if len(argv) > 0 {
		status = argv[0]
	}

	p.ExitStatus = status

	list := &p.exitHandlers[env.cpu.PSL().CurMod()]

	for len(*list) > 0 {
		desblk := (*list)[len(*list)-1]
		*list = (*list)[:len(*list)-1]

		handler, err := env.mem.LoadLongword(env.cpu, desblk+exhHandler)
		if err != nil {
			continue
		}

		if count, err := env.mem.LoadByte(env.cpu, desblk+exhArgList); err == nil && count >= 1 {
			if addr, err := env.mem.LoadLongword(env.cpu, desblk+exhStatusAdr); err == nil {
				_ = env.mem.StoreLongword(env.cpu, addr, status)
			}
		}

		return 0, &CallRequest{Routine: handler, ArgList: desblk + exhArgList}
	}

	return status, ErrExit
}

// ExitHandlers reports how many exit handlers mode has declared.
func (env *Environment) ExitHandlers(mode vax.AccessMode) int {
	return len(env.Process.exitHandlers[mode])
}

// cancelUserExitHandlers is image rundown's exit-handler step: user-mode
// exit control blocks live in the image's memory, so any the image left
// declared (it ended without $EXIT, or they were declared after it
// began) are forgotten.
func (env *Environment) cancelUserExitHandlers() {
	env.Process.exitHandlers[vax.User] = nil
}

func registerExitServices(t *ServiceTable) {
	t.Register("SYS$DCLEXH", serviceSysDclexh)
	t.Register("SYS$CANEXH", serviceSysCanexh)
	t.Register("SYS$EXIT", serviceSysExit)
}
