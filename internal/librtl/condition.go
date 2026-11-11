package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// Condition values these routines return or compare.
var (
	ssResignal = vmsdef.Symbols["SS$_RESIGNAL"]
	ssContinue = vmsdef.Symbols["SS$_CONTINUE"]
	ssUnwind   = vmsdef.Symbols["SS$_UNWIND"]
)

// The condition-handling routines (moved from internal/corevms, then named
// rtl, docs/PHASE-26.md subtask 33). The dispatch itself -- building the signal and mechanism
// arrays, searching the call frames, unwinding -- is corevms's; see
// internal/corevms/signal.go and unwind.go.

// libSignal is LIB$SIGNAL:
//
//	LIB$SIGNAL condition-value [,number-of-arguments] [,FAO-argument...]
//
// It signals the condition: its argument list becomes the signal array,
// and the condition goes through the same search a hardware exception does.
func libSignal(env *corevms.Environment, argv []uint32) (uint32, error) {
	return env.Signal(argv, false)
}

// libStop is LIB$STOP: LIB$SIGNAL with the severity forced to SEVERE,
// which can't be continued.
func libStop(env *corevms.Environment, argv []uint32) (uint32, error) {
	return env.Signal(argv, true)
}

// libEstablish is LIB$ESTABLISH:
//
//	LIB$ESTABLISH new-handler
//
// It makes new-handler (a procedure's entry mask address, passed by value)
// the condition handler of the calling procedure's frame, and returns the
// one it replaces (0 if none). With no argument it changes nothing, and
// returns the handler there (testdata/for83's part C, case 25).
func libEstablish(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 {
		return env.CallerHandler("LIB$ESTABLISH")
	}

	return env.SetCallerHandler("LIB$ESTABLISH", argv[0])
}

// libRevert is LIB$REVERT: it removes the calling procedure's condition
// handler, returning the one it removes (0 if none).
func libRevert(env *corevms.Environment, _ []uint32) (uint32, error) {
	return env.SetCallerHandler("LIB$REVERT", 0)
}

// libSigToRet is LIB$SIG_TO_RET:
//
//	LIB$SIG_TO_RET signal-args ,mechanism-args
//
// A condition handler that turns any condition into a return status: the
// procedure that established it returns the condition value to its caller.
// It returns SS$_NORMAL, SS$_NOSIGNAL outside a handler, or $UNWIND's
// error status. It doesn't check its arguments: with too few it is an access
// violation, in what it finds past the argument list on VMS.
func libSigToRet(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return accessViolation(env, 0, accvioWrite)
	}

	return env.ConditionToReturn(argv[0], argv[1])
}

// libSigToStop is LIB$SIG_TO_STOP:
//
//	LIB$SIG_TO_STOP signal-args ,mechanism-args
//
// It turns a signaled condition into a stop: the signal array's severity
// becomes severe, and the condition can no longer be continued. It returns
// SS$_NORMAL, which as a handler's own value means continue: so as a handler
// it ends the image with the condition's message, and no outer handler is
// called. A handler that calls it and resignals shows outer handlers the
// severe condition. SS$_UNWIND is left alone, with LIB$_INVARG. Like
// LIB$SIG_TO_RET it doesn't check its arguments.
func libSigToStop(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return accessViolation(env, 0, accvioWrite)
	}

	mem, cpu := env.Memory(), env.CPU()

	cond, err := mem.LoadLongword(cpu, argv[0]+4)
	if err != nil {
		return memoryFault(env, err)
	}

	if cond&condIDMask == ssUnwind&condIDMask {
		return libInvArg, nil
	}

	if err := mem.StoreLongword(cpu, argv[0]+4, cond&^7|severitySevere); err != nil {
		return memoryFault(env, writing(err))
	}

	env.StopCondition(argv[0])

	return ssNormal, nil
}

const (
	// condIDMask is STS$M_COND_ID, bits 3-27 of a condition value: the facility
	// and message number, without the severity (bits 0-2) or the control bits
	// (28-31), which a handler along the way may have changed.
	condIDMask = 0x0FFFFFF8

	// systemIDMask is the part of a condition's ID the LIB$ manual has
	// LIB$MATCH_COND compare for a systemwide value, leaving out the facility
	// code, bits 17-27.
	systemIDMask = 0x0001FFF8

	// facilitySpecific is STS$V_FAC_SP, bit 15: clear in a systemwide value.
	facilitySpecific = 1 << 15

	// severitySevere is a condition value's severity field for SEVERE (F).
	severitySevere = 4
)

// libMatchCond is LIB$MATCH_COND:
//
//	LIB$MATCH_COND condition-value ,condition-value-1 [,condition-value-2...]
//
// Every argument is a condition value passed by reference. It compares the
// first with each of the others, by their STS$V_COND_ID fields only, and
// returns the (1-based) position of the first that matches, or 0 if none
// does. If the first is systemwide (bit 15 clear), the facility code isn't
// compared. An argument that can't be read doesn't match.
func libMatchCond(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return 0, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	want, err := mem.LoadLongword(cpu, argv[0])
	if err != nil {
		return 0, nil
	}

	mask := uint32(condIDMask)
	if want&facilitySpecific == 0 {
		mask = systemIDMask
	}

	for i, adr := range argv[1:] {
		if v, err := mem.LoadLongword(cpu, adr); err == nil && v&mask == want&mask {
			return uint32(i + 1), nil
		}
	}

	return 0, nil
}

// The trap enables a procedure sets for itself: PSL<IV>, PSL<DV>, and PSL<FU>.
const (
	pswIV = 1 << 5
	pswFU = 1 << 6
	pswDV = 1 << 7
)

// libIntOver is LIB$INT_OVER new-setting: integer overflow traps in the
// calling procedure if bit 0 of the longword at new-setting is set. It
// returns the old setting. Each procedure has its own: a procedure's call
// starts it with the entry mask's, and the setting goes back with its RET.
func libIntOver(env *corevms.Environment, argv []uint32) (uint32, error) {
	return setTrapEnable(env, argv, pswIV)
}

// libDecOver is LIB$DEC_OVER new-setting, for decimal overflow.
func libDecOver(env *corevms.Environment, argv []uint32) (uint32, error) {
	return setTrapEnable(env, argv, pswDV)
}

// libFltUnder is LIB$FLT_UNDER new-setting, for floating underflow. It is a
// fault, so a handler that continues it gets it again.
func libFltUnder(env *corevms.Environment, argv []uint32) (uint32, error) {
	return setTrapEnable(env, argv, pswFU)
}

// setTrapEnable is the three routines' work. The running stub's frame holds
// the PSW its caller had at the call, which its RET puts back, so a bit
// changed there takes effect as the call returns, and only in the caller.
// With no argument it changes nothing (testdata/for83's cases 96, 102, and
// 108 show 0 with the bit clear; with it set is unconfirmed).
func setTrapEnable(env *corevms.Environment, argv []uint32, bit uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	at := cpu.GPR(registers.FP) + frameMaskPSW

	word, err := mem.LoadLongword(cpu, at)
	if err != nil {
		return 0, err
	}

	old := uint32(0)
	if word&bit != 0 {
		old = 1
	}

	if len(argv) == 0 {
		return old, nil
	}

	setting, err := mem.LoadLongword(cpu, argv[0])
	if err != nil {
		return memoryFault(env, err)
	}

	word &^= bit
	if setting&1 != 0 {
		word |= bit
	}

	if err := mem.StoreLongword(cpu, at, word); err != nil {
		return 0, err
	}

	return old, nil
}

// frameMaskPSW is where a call frame holds the PSW its call saved, in the low
// word: the second longword.
const frameMaskPSW = 4
