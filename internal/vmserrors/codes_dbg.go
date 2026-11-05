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
	dbgNoCalls
	dbgNoSymbol
	dbgNoSuchModule
	dbgNoTraces
	dbgNoWatches
	dbgOptNotAllow
	dbgIValOutBnds
	dbgNoType
	dbgInvTime
	dbgNeedQuote
	dbgAsmError
	dbgNoCurVal
	dbgNoExc
	dbgNotNumeric
	dbgConFromExc
	dbgVerifyICF
	dbgMisOpeMis
	dbgNotRecord
	dbgSymNotFnd
	dbgNoSuchImage
	dbgTooManSub
	dbgTooFewSub
	dbgSubOutBnd
	dbgNotActive
	dbgSymNotAct
	dbgNoDefScpe
	dbgBadScope
	dbgNotInScope
	dbgNotNumScope
	dbgNoScopeList
	dbgMisInvNum
	dbgNeedMore
	dbgDynModSet
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
	// DBG_NOCALLS is what SHOW CALLS and SHOW STACK say when there is no
	// call frame to show, as after the image has exited (the probe's
	// errors.dlg).
	DBG_NOCALLS = DBGFacility<<FacilityPosition | dbgNoCalls<<MessagePosition | StatusError
	// DBG_NOSYMBOL says a name isn't in the debugger's symbol table; its
	// argument is the name. The text is the VMS debugger's own (the
	// probe's sessions), as SHOW SYMBOL answers a pattern that matches
	// nothing.
	DBG_NOSYMBOL = DBGFacility<<FacilityPosition | dbgNoSymbol<<MessagePosition | StatusError
	// DBG_MISOPEMIS is what VMS's debugger says of an operand it can't go on
	// from (testdata/dbg85's probe: A .B); its argument is what it found.
	DBG_MISOPEMIS = DBGFacility<<FacilityPosition | dbgMisOpeMis<<MessagePosition | StatusError
	// DBG_NOTRECORD is language FORTRAN's answer to X.Y when X is data but
	// no record (testdata/dbg85's probe); its argument is the component.
	DBG_NOTRECORD = DBGFacility<<FacilityPosition | dbgNotRecord<<MessagePosition | StatusWarning
	// DBG_SYMNOTFND is SHOW SYMBOL's answer when nothing matches (testdata/dbg85), and
	// DBG_NOSUCHIMAGE SET IMAGE's to an image not loaded, whose wording is govax's.
	DBG_SYMNOTFND   = DBGFacility<<FacilityPosition | dbgSymNotFnd<<MessagePosition | StatusInfo
	DBG_NOSUCHIMAGE = DBGFacility<<FacilityPosition | dbgNoSuchImage<<MessagePosition | StatusError
	// DBG_TOOMANSUB is too many subscripts for an array (testdata/dbg85's probe); its
	// argument is how many dimensions it has, with the word. DBG_TOOFEWSUB is the other
	// way, DBG_SUBOUTBND a subscript past its dimension, and DBG_NOTACTIVE a dummy
	// argument or AUTOMATIC variable of a routine with no active frame: their
	// wording is govax's, no probe having shown them.
	DBG_TOOMANSUB = DBGFacility<<FacilityPosition | dbgTooManSub<<MessagePosition | StatusWarning
	DBG_TOOFEWSUB = DBGFacility<<FacilityPosition | dbgTooFewSub<<MessagePosition | StatusWarning
	DBG_SUBOUTBND = DBGFacility<<FacilityPosition | dbgSubOutBnd<<MessagePosition | StatusWarning
	DBG_NOTACTIVE = DBGFacility<<FacilityPosition | dbgNotActive<<MessagePosition | StatusWarning
	// The scope messages (Phase 89, VMS 7.1's texts from testdata/dbg89).
	// DBG_SYMNOTACT names a register of a call level that isn't there, or
	// that the level doesn't keep (R0 and R1 of a caller): its argument is
	// the path, '9\%R2' or 'DBG89\REC\%R0'.
	DBG_SYMNOTACT = DBGFacility<<FacilityPosition | dbgSymNotAct<<MessagePosition | StatusWarning
	// DBG_NODEFSCPE is SET SCOPE/CURRENT, or a scope built-in symbol,
	// while a SET SCOPE list is in effect; its argument is what was asked.
	DBG_NODEFSCPE = DBGFacility<<FacilityPosition | dbgNoDefScpe<<MessagePosition | StatusError
	// DBG_BADSCOPE is a SET SCOPE location no module or routine has.
	DBG_BADSCOPE = DBGFacility<<FacilityPosition | dbgBadScope<<MessagePosition | StatusError
	// DBG_NOTINSCOPE is SET SCOPE/CURRENT n past the last call level.
	DBG_NOTINSCOPE = DBGFacility<<FacilityPosition | dbgNotInScope<<MessagePosition | StatusError
	// DBG_NOTNUMSCOPE is SET SCOPE/CURRENT of a path name.
	DBG_NOTNUMSCOPE = DBGFacility<<FacilityPosition | dbgNotNumScope<<MessagePosition | StatusError
	// DBG_NOSCOPELIST is SET SCOPE/CURRENT of a list.
	DBG_NOSCOPELIST = DBGFacility<<FacilityPosition | dbgNoScopeList<<MessagePosition | StatusError
	// DBG_MISINVNUM is an invocation number before a name a routine's
	// activation doesn't hold (REC 1\Y, Y being the module's).
	DBG_MISINVNUM = DBGFacility<<FacilityPosition | dbgMisInvNum<<MessagePosition | StatusWarning
	// DBG_NEEDMORE is a command that ended before its parameter (SET SCOPE
	// alone).
	DBG_NEEDMORE = DBGFacility<<FacilityPosition | dbgNeedMore<<MessagePosition | StatusWarning
	// DBG_DYNMODSET is the debugger setting a module's symbols itself, when
	// the program stops in it.
	DBG_DYNMODSET = DBGFacility<<FacilityPosition | dbgDynModSet<<MessagePosition | StatusInfo
	// DBG_NOSUCHMODULE is SET MODULE's and SHOW SYMBOL's answer to a
	// module name no loaded image has. The wording is govax's choice: no
	// probe showed it.
	DBG_NOSUCHMODULE = DBGFacility<<FacilityPosition | dbgNoSuchModule<<MessagePosition | StatusError
	// DBG_NOTRACES is SHOW TRACE's answer when no tracepoint is set (the
	// probe's trace.dlg). CANCEL TRACE of nothing says it too: unconfirmed.
	DBG_NOTRACES = DBGFacility<<FacilityPosition | dbgNoTraces<<MessagePosition | StatusInfo
	// DBG_NOWATCHES is SHOW WATCH's answer when no watchpoint is set (the
	// probe's watch.dlg). CANCEL WATCH of nothing says it too: unconfirmed.
	DBG_NOWATCHES = DBGFacility<<FacilityPosition | dbgNoWatches<<MessagePosition | StatusInfo
	// DBG_OPTNOTALLOW reports a command that can't act on the data type
	// it was given (EXAMINE/ASCIC of a register); its argument is the
	// command's verb. The text is the VMS 5.5 Debugger Manual's.
	DBG_OPTNOTALLOW = DBGFacility<<FacilityPosition | dbgOptNotAllow<<MessagePosition | StatusError
	// DBG_IVALOUTBNDS reports a value DEPOSIT can't store in the type it
	// was given (a number with more digits than PACKED:n holds). The
	// text is the manual's; VMS stores the value anyway, as an
	// informational says, but govax stores nothing (unconfirmed), so it
	// is an error here.
	DBG_IVALOUTBNDS = DBGFacility<<FacilityPosition | dbgIValOutBnds<<MessagePosition | StatusError
	// DBG_NOTYPE reports a TYPE=(name) whose name isn't a datum the
	// program defines (govax's own text; the manual shows none).
	DBG_NOTYPE = DBGFacility<<FacilityPosition | dbgNoType<<MessagePosition | StatusError
	// DBG_INVTIME reports DEPOSIT/DATE_TIME text that isn't a VMS time
	// (govax's own text).
	DBG_INVTIME = DBGFacility<<FacilityPosition | dbgInvTime<<MessagePosition | StatusError
	// DBG_NEEDQUOTE reports a DEPOSIT/INSTRUCTION whose instruction isn't
	// in quotes, as the manual says it must be (govax's own text).
	DBG_NEEDQUOTE = DBGFacility<<FacilityPosition | dbgNeedQuote<<MessagePosition | StatusError
	// DBG_ASMERROR reports an instruction DEPOSIT/INSTRUCTION couldn't
	// assemble; the assembler's own message follows it (govax's text).
	DBG_ASMERROR = DBGFacility<<FacilityPosition | dbgAsmError<<MessagePosition | StatusError
	// DBG_NOCURVAL reports %CURVAL (or "\") before anything has been
	// shown or stored (govax's own text).
	DBG_NOCURVAL = DBGFacility<<FacilityPosition | dbgNoCurVal<<MessagePosition | StatusError
	// DBG_NOEXC reports an %EXC_ symbol when the program hasn't stopped
	// for an exception (govax's own text).
	DBG_NOEXC = DBGFacility<<FacilityPosition | dbgNoExc<<MessagePosition | StatusError
	// DBG_NOTNUMERIC reports a string built-in symbol (%EXC_NAME) used
	// where a number is needed (govax's own text).
	DBG_NOTNUMERIC = DBGFacility<<FacilityPosition | dbgNotNumeric<<MessagePosition | StatusError
	// DBG_CONFROMEXC is GO's (or STEP's) warning when the program goes on
	// from a break on an unhandled severe condition, which it then
	// continues (VMS 7.1's text, testdata/trace73's TRUNH).
	DBG_CONFROMEXC = DBGFacility<<FacilityPosition | dbgConFromExc<<MessagePosition | StatusInfo
	// DBG_VERIFYICF is shown, with SET OUTPUT VERIFY, as a debugger
	// command procedure (@) starts and ends: "entering" or "exiting", and
	// the file as given (VMS 7.1's text, testdata/trace73's sessions).
	DBG_VERIFYICF = DBGFacility<<FacilityPosition | dbgVerifyICF<<MessagePosition | StatusInfo
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
	DefineMessage(DBG_NOCALLS, DBGFacility, "NOCALLS", "no active call frames")
	DefineMessage(DBG_NOSYMBOL, DBGFacility, "NOSYMBOL", "symbol '!S' is not in the symbol table")
	DefineMessage(DBG_MISOPEMIS, DBGFacility, "MISOPEMIS", "misplaced operator or missing operand at '!S'")
	DefineMessage(DBG_NOTRECORD, DBGFacility, "NOTRECORD", "variable is not record; cannot select component !S")
	DefineMessage(DBG_SYMNOTFND, DBGFacility, "SYMNOTFND", "no symbols matching !S are defined")
	DefineMessage(DBG_NOSUCHIMAGE, DBGFacility, "NOSUCHIMAGE", "image '!S' is not loaded")
	DefineMessage(DBG_TOOMANSUB, DBGFacility, "TOOMANSUB", "too many subscripts, array has !S")
	DefineMessage(DBG_TOOFEWSUB, DBGFacility, "TOOFEWSUB", "too few subscripts, array has !S")
	DefineMessage(DBG_SUBOUTBND, DBGFacility, "SUBOUTBND", "subscript !S is out of bounds, value is !S, bounds are !S")
	DefineMessage(DBG_NOTACTIVE, DBGFacility, "NOTACTIVE", "!S has no active call frame")
	DefineMessage(DBG_SYMNOTACT, DBGFacility, "SYMNOTACT", "non-static variable '!S' is not active")
	DefineMessage(DBG_NODEFSCPE, DBGFacility, "NODEFSCPE", "No default scope list: error performing !S")
	DefineMessage(DBG_BADSCOPE, DBGFacility, "BADSCOPE", "invalid pathname !S, SCOPE not changed")
	DefineMessage(DBG_NOTINSCOPE, DBGFacility, "NOTINSCOPE", "specified scope cannot be found in the default scope list")
	DefineMessage(DBG_NOTNUMSCOPE, DBGFacility, "NOTNUMSCOPE", "specified scope is not a numbered scope.")
	DefineMessage(DBG_NOSCOPELIST, DBGFacility, "NOSCOPELIST", "a list of scopes is not allowed with this command.")
	DefineMessage(DBG_MISINVNUM, DBGFacility, "MISINVNUM", "misplaced invocation number in !S")
	DefineMessage(DBG_NEEDMORE, DBGFacility, "NEEDMORE", "unexpected end of command line")
	DefineMessage(DBG_DYNMODSET, DBGFacility, "DYNMODSET", "setting module !S")
	DefineMessage(DBG_NOSUCHMODULE, DBGFacility, "NOSUCHMODULE", "module '!S' is not in the module table")
	DefineMessage(DBG_NOTRACES, DBGFacility, "NOTRACES", "no tracepoints are set, no opcode tracing")
	DefineMessage(DBG_NOWATCHES, DBGFacility, "NOWATCHES", "no watchpoints are set")
	DefineMessage(DBG_OPTNOTALLOW, DBGFacility, "OPTNOTALLOW", "operator \"!S\" not allowed on given data type")
	DefineMessage(DBG_IVALOUTBNDS, DBGFacility, "IVALOUTBNDS", "value assigned is out of bounds at or near '!S'")
	DefineMessage(DBG_NOTYPE, DBGFacility, "NOTYPE", "'!S' does not name a data type")
	DefineMessage(DBG_INVTIME, DBGFacility, "INVTIME", "invalid date or time at or near '!S'")
	DefineMessage(DBG_NEEDQUOTE, DBGFacility, "NEEDQUOTE", "an instruction must be in quotes at or near '!S'")
	DefineMessage(DBG_VERIFYICF, DBGFacility, "VERIFYICF", "!S command procedure !S")
	DefineMessage(DBG_CONFROMEXC, DBGFacility, "CONFROMEXC", "WARNING: continuing after severe error; integrity not guaranteed")
	DefineMessage(DBG_ASMERROR, DBGFacility, "ASMERROR", "cannot assemble '!S'")
	DefineMessage(DBG_NOCURVAL, DBGFacility, "NOCURVAL", "no value has been examined, evaluated, or deposited")
	DefineMessage(DBG_NOEXC, DBGFacility, "NOEXC", "there is no current exception")
	DefineMessage(DBG_NOTNUMERIC, DBGFacility, "NOTNUMERIC", "'!S' is a string, not a number")
	DefineMessage(DBG_NOSOURCEDIR, DBGFacility, "NOSOURCEDIR", "no source directory search list is in effect")
}
