package vmserrors

// LIB facility message IDs -- internal/rtl's LIBRTL shim/system-service
// simulation. Private: only the composite LIB_* codes below are part of
// this package's public API.
const (
	libHalt uint32 = iota + 1
	libPanic
	libUnresolved
)

// LIB facility status codes -- LIB_ prefix, matching real VMS's LIB$_
// status codes.
const (
	// LIB_HALT reports a ServiceFunc/ShimFunc requesting the machine
	// halt (rtl.ErrHalt), matching cli.c's own "vax.halted = 1" on an
	// unrecognized CLI request -- not a failure, hence StatusSuccess.
	LIB_HALT = LIBFacility<<FacilityPosition | libHalt<<MessagePosition | StatusSuccess
	// LIB_PANIC reports a recovered panic from inside a service/shim
	// handler (see callHandler).
	LIB_PANIC = LIBFacility<<FacilityPosition | libPanic<<MessagePosition | StatusSevere
	// LIB_UNRESOLVED reports a G^ fixup target with no loaded image or
	// SHIM$ stub to resolve against (internal/console's image.go).
	LIB_UNRESOLVED = LIBFacility<<FacilityPosition | libUnresolved<<MessagePosition | StatusError
)

func init() {
	DefineMessage(LIB_HALT, LIBFacility, "HALT", "Halt requested")
	DefineMessage(LIB_PANIC, LIBFacility, "PANIC", "Handler panic: !S")
	DefineMessage(LIB_UNRESOLVED, LIBFacility, "UNRESOLVED", "Unresolved shim symbol !S")
}
