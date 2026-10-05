package vmserrors

// DBG facility message IDs -- the debugger (docs/PHASE-42.md). The
// facility prints as DEBUG, as the VMS debugger's own messages do
// (%DEBUG-E-SYNTAX, ...). Private: only the composite DBG_* codes below
// are part of this package's public API.
const (
	dbgSyntax uint32 = iota + 1
	dbgNotAvailable
	dbgConsoleCommand
	dbgInitial
	dbgExitStatus
	dbgBadStartPC
	dbgNoBreaks
	dbgNoAccessR
	dbgNoSourceDir
)

// DBG facility status codes.
const (
	// DBG_SYNTAX reports a debugger command the debugger's grammar can't
	// parse. Its argument is the first word it couldn't take: an unknown
	// verb, keyword, or qualifier. The text is the VMS debugger's own,
	// from the probe in testdata/dbgcmd (errors.dlg).
	DBG_SYNTAX = DBGFacility<<FacilityPosition | dbgSyntax<<MessagePosition | StatusError
	// DBG_NOTAVAILABLE reports a console command that starts the debugger
	// (DEBUG) when this console has no debugger installed, as a console
	// built without cmd/govax's wiring (a unit test) hasn't.
	DBG_NOTAVAILABLE = DBGFacility<<FacilityPosition | dbgNotAvailable<<MessagePosition | StatusError
	// DBG_CONSOLECOMMAND is govax's hint, shown after DBG_SYNTAX when the
	// word that failed is a console (VMS command line) verb: the debugger
	// has no DCL, but EXIT returns to the console where it works. VMS has
	// no such message; this one is govax's.
	DBG_CONSOLECOMMAND = DBGFacility<<FacilityPosition | dbgConsoleCommand<<MessagePosition | StatusInfo
	// DBG_INITIAL is the debugger's start-up message for an image: the
	// language and module of the program's main routine. Its arguments
	// are the language and the module name (VMS's own text).
	DBG_INITIAL = DBGFacility<<FacilityPosition | dbgInitial<<MessagePosition | StatusInfo
	// DBG_EXITSTATUS is shown when the image under the debugger exits.
	// Its argument is the exit status's own message line, minus the
	// leading percent sign (VMS's own text).
	DBG_EXITSTATUS = DBGFacility<<FacilityPosition | dbgExitStatus<<MessagePosition | StatusInfo
	// DBG_BADSTARTPC is what GO and STEP say when there is no program to
	// run, as after the image has exited. Its argument is the PC.
	DBG_BADSTARTPC = DBGFacility<<FacilityPosition | dbgBadStartPC<<MessagePosition | StatusError
	// DBG_NOBREAKS is SHOW BREAK's answer when none is set, and CANCEL
	// BREAK's when there is nothing to cancel.
	DBG_NOBREAKS = DBGFacility<<FacilityPosition | dbgNoBreaks<<MessagePosition | StatusInfo
	// DBG_NOACCESSR reports a read of an address the program can't read,
	// as a breakpoint's WHEN condition can do. Its argument is the address.
	DBG_NOACCESSR = DBGFacility<<FacilityPosition | dbgNoAccessR<<MessagePosition | StatusError
	// DBG_NOSOURCEDIR is SHOW SOURCE's answer when SET SOURCE has given no
	// directory list. The wording is govax's choice: no probe showed it.
	DBG_NOSOURCEDIR = DBGFacility<<FacilityPosition | dbgNoSourceDir<<MessagePosition | StatusInfo
)

func init() {
	DefineMessage(DBG_SYNTAX, DBGFacility, "SYNTAX", "command syntax error at or near '!S'")
	DefineMessage(DBG_NOTAVAILABLE, DBGFacility, "NOTAVAILABLE", "the debugger is not available")
	DefineMessage(DBG_CONSOLECOMMAND, DBGFacility, "CONSOLECOMMAND", "'!S' is a console command; EXIT returns to the console")
	DefineMessage(DBG_INITIAL, DBGFacility, "INITIAL", "Language: !S, Module: !S")
	DefineMessage(DBG_EXITSTATUS, DBGFacility, "EXITSTATUS", "is '!S'")
	DefineMessage(DBG_BADSTARTPC, DBGFacility, "BADSTARTPC", "cannot start from PC !XL")
	DefineMessage(DBG_NOBREAKS, DBGFacility, "NOBREAKS", "no breakpoints are set")
	DefineMessage(DBG_NOACCESSR, DBGFacility, "NOACCESSR", "no read access to address !XL")
	DefineMessage(DBG_NOSOURCEDIR, DBGFacility, "NOSOURCEDIR", "no source directory search list is in effect")
}
