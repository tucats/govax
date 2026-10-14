package librtl

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
)

// The statistics routines, LIB$STAT_VM and LIB$SHOW_VM (5.0 LIB$ manual,
// LIB-356 and LIB-392): counts of the heap routines' and the page
// routines' work since the image started. The heap's (codes 1 to 3) are
// corevms's (VMStatistics), counted as VMS 7.1 counts them
// (testdata/lib59): LIB$GET_VM's successful calls and LIB$FREE_VM's
// calls (a user-defined zone's too, whose own routine's calls count as
// well), the RTL's allocations of control blocks and dynamic strings
// among them, and the bytes their blocks hold as rounded (tags
// included). The pages' (5 to 7) are the image's page pool's own counts
// (corevms.Environment.PageStatistics): every page request the pool
// satisfies, a zone's areas among them.

// The statistic codes LIB$STAT_VM and LIB$SHOW_VM take. LIB$SHOW_VM's 0
// shows the first three on one line, and 4 the last three.
const (
	vmGets       = 1 // successful LIB$GET_VM calls
	vmFrees      = 2 // LIB$FREE_VM calls
	vmBytes      = 3 // bytes LIB$GET_VM allocated and LIB$FREE_VM hasn't freed
	vmPageLine   = 4 // LIB$SHOW_VM's line of codes 5 to 7
	vmPageGets   = 5 // LIB$GET_VM_PAGE calls
	vmPageFrees  = 6 // LIB$FREE_VM_PAGE calls
	vmPagesInUse = 7 // pages LIB$GET_VM_PAGE allocated and not freed
)

// vmStatistic returns the statistic code names (1 to 3, 5 to 7), and
// false for any other code.
func vmStatistic(env *corevms.Environment, code int32) (uint32, bool) {
	s := env.VMStatistics()
	pageGets, pageFrees, pages := env.PageStatistics()

	switch code {
	case vmGets:
		return s.Gets, true
	case vmFrees:
		return s.Frees, true
	case vmBytes:
		return s.Bytes, true
	case vmPageGets:
		return pageGets, true
	case vmPageFrees:
		return pageFrees, true
	case vmPagesInUse:
		return pages, true
	}

	return 0, false
}

// libStatVM is LIB$STAT_VM:
//
//	LIB$STAT_VM code.rl.r ,value-argument.wz.r
//
// It writes the statistic code names (1 to 3, 5 to 7; see the constants
// above) to the longword at value-argument. "It is invalid to omit code
// or to give a code of 0 or 4": those, and any other code, are
// LIB$_INVARG, with nothing written.
func libStatVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	if arg(argv, 0) == 0 {
		return libInvArg, nil
	}

	code, err := readArg(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	value, ok := vmStatistic(env, code)
	if !ok {
		return libInvArg, nil
	}

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 1), value); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// vmLine is LIB$SHOW_VM's line for code (0 to 7), and false for any
// other code. Code 0's is the manual's
//
//	 mmm calls to LIB$GET_VM, nnn calls to LIB$FREE_VM, ppp bytes still allocated
//
// with a blank first ("The first character is a blank"); code 4's has
// the same form for the page routines, and each other code's is its
// part of those lines, the numbers not padded, as VMS 7.1 writes them
// (testdata/lib59).
func vmLine(env *corevms.Environment, code int32) (string, bool) {
	fields := [vmPagesInUse + 1]string{
		vmGets:       "%d calls to LIB$GET_VM",
		vmFrees:      "%d calls to LIB$FREE_VM",
		vmBytes:      "%d bytes still allocated",
		vmPageGets:   "%d calls to LIB$GET_VM_PAGE",
		vmPageFrees:  "%d calls to LIB$FREE_VM_PAGE",
		vmPagesInUse: "%d pages still allocated",
	}

	field := func(c int32) string {
		v, _ := vmStatistic(env, c)

		return fmt.Sprintf(fields[c], int32(v)) // signed, as STAT_VM's longword is
	}

	switch {
	case code == 0 || code == vmPageLine:
		return fmt.Sprintf(" %s, %s, %s", field(code+1), field(code+2), field(code+3)), true
	case code > 0 && code <= vmPagesInUse:
		return " " + field(code), true
	}

	return "", false
}

// libShowVM is LIB$SHOW_VM:
//
//	LIB$SHOW_VM [code.rl.r] [,user-action-procedure.flc.rp]
//	            [,user-specified-argument.rz.v]
//
// It writes the line for code (vmLine; 0 when omitted), another code
// being LIB$_INVARG, to SYS$OUTPUT as LIB$PUT_OUTPUT writes it,
// returning the write's status; or, with an action routine, calls the
// routine with a class S descriptor of the line and
// user-specified-argument (0 when omitted), as the shim's tail call (as
// LIB$SHOW_TIMER does, callAction), and returns its status.
func libShowVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	code, err := optionalLongword(env, arg(argv, 0), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	line, ok := vmLine(env, int32(code))
	if !ok {
		return libInvArg, nil
	}

	action := arg(argv, 1)
	if action == 0 {
		return env.PutOutput(line)
	}

	return callAction(env, action, line, arg(argv, 2))
}
