package vmserrors

// DBG facility message IDs -- the debugger (docs/PHASE-42.md). The
// facility prints as DEBUG, as the VMS debugger's own messages do
// (%DEBUG-E-SYNTAX, ...). Private: only the composite DBG_* codes below
// are part of this package's public API.
const (
	dbgSyntax uint32 = iota + 1
	dbgNotAvailable
	dbgConsoleCommand
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
)

func init() {
	DefineMessage(DBG_SYNTAX, DBGFacility, "SYNTAX", "command syntax error at or near '!S'")
	DefineMessage(DBG_NOTAVAILABLE, DBGFacility, "NOTAVAILABLE", "the debugger is not available")
	DefineMessage(DBG_CONSOLECOMMAND, DBGFacility, "CONSOLECOMMAND", "'!S' is a console command; EXIT returns to the console")
}
