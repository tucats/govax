package corevms

// Argument counts of the system services (docs/PHASE-45.md, probe 1).
//
// A service on VMS checks how many arguments its caller passed (the
// argument count byte of the CALLS/CALLG argument list) and fails with
// SS$_INSFARG when there are too few. A program built with the $xxx_S
// macros always passes every argument, an omitted one as 0, so it never
// notices; a program that builds its own argument list can. The VMS 7.1
// probe showed how strict it is: $GETDVIW with 4 arguments (it takes
// 8, the last a placeholder), $CREMBX with 4 (it takes 7), and $ASSIGN
// with 2 (it takes 4) were all refused, even though the arguments left
// off were optional.
//
// SystemService enforces serviceMinArgs before calling a service. The
// counts are of two kinds:
//
//   - Full: every argument of the manual's syntax, where VMS 7.1 was
//     seen to demand them all, or to accept exactly that many: marked
//     "seen" below.
//   - Required: only up to the last argument the manual does not bracket
//     as optional. VMS may want more, so govax is no stricter than the
//     manual; a program that passes this many passes govax's check, and
//     one that passes fewer would fail on VMS too.
//
// A service not listed (the RMS services, whose argument counts vary)
// has no minimum. A call made from Go (the unit tests call a
// ServiceFunc directly) is not checked.
var serviceMinArgs = map[string]int{
	// Seen on VMS 7.1.
	"SYS$GETDVI":  8,
	"SYS$GETDVIW": 8,
	"SYS$CREMBX":  7,
	"SYS$ASSIGN":  4,
	"SYS$GETJPI":  7,
	"SYS$GETJPIW": 7,
	"SYS$CREPRC":  12,
	"SYS$QIO":     12,
	"SYS$QIOW":    12,
	"SYS$TRNLNM":  5,
	"SYS$WAKE":    2,

	// Seen on VMS 7.3 (testdata/mp/probe5, step 8).
	"SYS$ADJWSL": 2,
	"SYS$ALLOC":  4,
	"SYS$ASCEFC": 4,

	// Required arguments, from the System Services Reference Manual.
	"SYS$ADJSTK": 3,
	"SYS$ASCTIM": 2,
	"SYS$BINTIM": 2,
	"SYS$CANCEL": 1,
	"SYS$CLREF":  1,
	"SYS$CMEXEC": 2,
	"SYS$CMKRNL": 2,
	"SYS$CRELNM": 5,
	"SYS$DACEFC": 1,
	"SYS$DASSGN": 1,
	"SYS$DCLAST": 2,
	"SYS$DELLNM": 1,
	"SYS$DELMBX": 1,
	"SYS$DLCEFC": 1,
	"SYS$FAO":    3,
	"SYS$FAOL":   4,
	"SYS$GETMSG": 3,
	"SYS$GETSYI": 4,
	"SYS$GETTIM": 1,
	"SYS$PUTMSG": 1,
	"SYS$READEF": 2,
	"SYS$SCHDWK": 3,
	"SYS$SETAST": 1,
	"SYS$SETEF":  1,
	"SYS$SETIMR": 2,
	"SYS$SETPRI": 3,
	"SYS$SETPRV": 1,
	"SYS$SNDOPR": 2,
	"SYS$WAITFR": 1,
	"SYS$WFLAND": 2,
	"SYS$WFLOR":  2,
}
