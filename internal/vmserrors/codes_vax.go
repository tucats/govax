package vmserrors

// VAX facility message IDs -- govax's own internal messages (there is no
// real VMS facility for "the emulator itself"). Private: only the
// composite VAX_* codes below are part of this package's public API.
//
// Grouped by originating subsystem for ease of locating a particular
// message: internal/cpu's engine-control signals first, then
// internal/asm's assembler diagnostics.
const (
	vaxHalted uint32 = iota + 1
	vaxInstLimit
	vaxTimeLimit
	vaxAttention
	vaxNoHandler
	vaxUnhandled
	vaxImmOperand
	vaxCallReturned

	// internal/asm diagnostics, in roughly the order the assembler's own
	// files (register/opcode/functions/disasm/assembler/pseudo/operand/
	// symbol/value) encounter them.
	vaxBadReg
	vaxBadOpcode
	vaxBadOperands
	vaxOperandErr
	vaxDefArg
	vaxDefQuote
	vaxBadOpcodeAt
	vaxInternal
	vaxIndexNest
	vaxIncludeDepth
	vaxNotLive
	vaxDataRange
	vaxUndefSym
	vaxNeedMicro
	vaxSCBCode
	vaxSCBAlign
	vaxSCBStack
	vaxAlignZero
	vaxRegionSpec
	vaxBadScale
	vaxNeedHash
	vaxBadShortLit
	vaxShortRange
	vaxBadShortFloat
	vaxBadMode
	vaxBadImmLit
	vaxFloatRange
	vaxDupSym
	vaxFwdWord
	vaxFwdByte
	vaxBadFloat
	vaxFwdOperator
	vaxDivZero
	vaxIncompleteNum
	vaxFloatHere
	vaxBadDecimal
	vaxBadHex
	vaxBadCharLit
	vaxUntermChar
	vaxCharTooLong
	vaxBadMask
	vaxBadMaskEntry

	// main.go's top-level startup diagnostics.
	vaxGrammar
	vaxAllocVAX
	vaxReadline

	// Clean exit from emulation.
	vaxQuit

	// More internal/asm diagnostics, added after the codes above so
	// their values stay put.
	vaxNoClose
	vaxBadString
	vaxExtraText
	vaxBadDigit
	vaxBadCond
	vaxNoCond
	vaxNoEndc
	vaxCondDepth
	vaxNotMACRO
	vaxRelExpr
	vaxMACROOnly
	vaxPsectAttr
	vaxPsectConflict
	vaxAbsData
	vaxAlignPsect
	vaxPsectStack
	vaxTooManyPsects
	vaxBadKeyword
	vaxEntryMask
	vaxNotEntry
	vaxIgnored
	vaxNotGlobal

	// MACRO-32 macros (docs/PHASE-28.md).
	vaxNoEndm
	vaxEndmName
	vaxNotInDef
	vaxNotInMacro
	vaxMacroName
	vaxMacroDepth
	vaxTooManyArgs
	vaxBadFormal
	vaxBadOperator
	vaxNoEndr
	vaxNotInRepeat
	vaxGenErr
	vaxGenWrn
	vaxUndefMacro
	vaxLibRead
	vaxNoLibResolver
	vaxLibrary
	vaxModeAccess
	vaxIllExpr
	vaxBadPacked
)

// VAX facility status codes -- VAX_ prefix.
const (
	// VAX_HALTED reports a normal CPU halt (cpu.ErrHalted) -- not a
	// failure, hence StatusSuccess.
	VAX_HALTED = VAXFacility<<FacilityPosition | vaxHalted<<MessagePosition | StatusSuccess
	// VAX_INSTLIM/VAX_TIMELIM report Engine.Step stopping because a
	// configured instruction-count/wall-clock budget ran out
	// (cpu.ErrInstructionLimitExceeded/ErrTimeLimitExceeded) -- the
	// machine itself is fine, so these are warnings, not errors.
	VAX_INSTLIM = VAXFacility<<FacilityPosition | vaxInstLimit<<MessagePosition | StatusWarning
	VAX_TIMELIM = VAXFacility<<FacilityPosition | vaxTimeLimit<<MessagePosition | StatusWarning
	// VAX_ATTENTION reports Engine.Step stopping because the user pressed
	// Ctrl-C (cpu.ErrInterrupted) -- matching console.c's own attention()/
	// vax.halted = VAX_ATTENTION (SEV_INFO in the C source, hence
	// StatusInfo here, not StatusWarning like the two budget-exceeded
	// codes above): the machine is fine, execution was just asked to
	// pause.
	VAX_ATTENTION = VAXFacility<<FacilityPosition | vaxAttention<<MessagePosition | StatusInfo
	// VAX_NOHANDLER/VAX_UNHANDLED report Engine.HandleFault finding no
	// usable SCB vector for a fault (cpu.ErrNoExceptionHandler/
	// ErrUnhandledVector) -- genuine failures, hence StatusSevere.
	VAX_NOHANDLER = VAXFacility<<FacilityPosition | vaxNoHandler<<MessagePosition | StatusSevere
	VAX_UNHANDLED = VAXFacility<<FacilityPosition | vaxUnhandled<<MessagePosition | StatusSevere
	// VAX_IMMOPND reports Operand.Store refusing to write to an
	// immediate operand (cpu.ErrImmutableOperand) -- a defensive
	// backstop that should never trigger from real VAX code.
	VAX_IMMOPND = VAXFacility<<FacilityPosition | vaxImmOperand<<MessagePosition | StatusError
	// VAX_CALLRET reports a console-initiated CallEntry's RET returning
	// cleanly (cpu.ErrConsoleCallReturned) -- normal completion, not a
	// failure.
	VAX_CALLRET = VAXFacility<<FacilityPosition | vaxCallReturned<<MessagePosition | StatusSuccess

	// internal/asm diagnostics -- all StatusError (assembly-time
	// diagnostics against user-supplied source), except VAX_INTERNAL
	// (an assertion failure the assembler should never actually reach).
	VAX_BADREG        = VAXFacility<<FacilityPosition | vaxBadReg<<MessagePosition | StatusError
	VAX_BADOPCODE     = VAXFacility<<FacilityPosition | vaxBadOpcode<<MessagePosition | StatusError
	VAX_BADOPERANDS   = VAXFacility<<FacilityPosition | vaxBadOperands<<MessagePosition | StatusError
	VAX_OPERANDERR    = VAXFacility<<FacilityPosition | vaxOperandErr<<MessagePosition | StatusError
	VAX_DEFARG        = VAXFacility<<FacilityPosition | vaxDefArg<<MessagePosition | StatusError
	VAX_DEFQUOTE      = VAXFacility<<FacilityPosition | vaxDefQuote<<MessagePosition | StatusError
	VAX_BADOPCODEAT   = VAXFacility<<FacilityPosition | vaxBadOpcodeAt<<MessagePosition | StatusError
	VAX_INTERNAL      = VAXFacility<<FacilityPosition | vaxInternal<<MessagePosition | StatusSevere
	VAX_INDEXNEST     = VAXFacility<<FacilityPosition | vaxIndexNest<<MessagePosition | StatusError
	VAX_INCLUDEDEPTH  = VAXFacility<<FacilityPosition | vaxIncludeDepth<<MessagePosition | StatusError
	VAX_NOTLIVE       = VAXFacility<<FacilityPosition | vaxNotLive<<MessagePosition | StatusError
	VAX_DATARANGE     = VAXFacility<<FacilityPosition | vaxDataRange<<MessagePosition | StatusError
	VAX_UNDEFSYM      = VAXFacility<<FacilityPosition | vaxUndefSym<<MessagePosition | StatusError
	VAX_NEEDMICRO     = VAXFacility<<FacilityPosition | vaxNeedMicro<<MessagePosition | StatusError
	VAX_SCBCODE       = VAXFacility<<FacilityPosition | vaxSCBCode<<MessagePosition | StatusError
	VAX_SCBALIGN      = VAXFacility<<FacilityPosition | vaxSCBAlign<<MessagePosition | StatusError
	VAX_SCBSTACK      = VAXFacility<<FacilityPosition | vaxSCBStack<<MessagePosition | StatusError
	VAX_ALIGNZERO     = VAXFacility<<FacilityPosition | vaxAlignZero<<MessagePosition | StatusError
	VAX_REGIONSPEC    = VAXFacility<<FacilityPosition | vaxRegionSpec<<MessagePosition | StatusError
	VAX_BADSCALE      = VAXFacility<<FacilityPosition | vaxBadScale<<MessagePosition | StatusError
	VAX_NEEDHASH      = VAXFacility<<FacilityPosition | vaxNeedHash<<MessagePosition | StatusError
	VAX_BADSHORTLIT   = VAXFacility<<FacilityPosition | vaxBadShortLit<<MessagePosition | StatusError
	VAX_SHORTRANGE    = VAXFacility<<FacilityPosition | vaxShortRange<<MessagePosition | StatusError
	VAX_BADSHORTFLOAT = VAXFacility<<FacilityPosition | vaxBadShortFloat<<MessagePosition | StatusError
	VAX_BADMODE       = VAXFacility<<FacilityPosition | vaxBadMode<<MessagePosition | StatusError
	VAX_BADIMMLIT     = VAXFacility<<FacilityPosition | vaxBadImmLit<<MessagePosition | StatusError
	VAX_FLOATRANGE    = VAXFacility<<FacilityPosition | vaxFloatRange<<MessagePosition | StatusError
	VAX_DUPSYM        = VAXFacility<<FacilityPosition | vaxDupSym<<MessagePosition | StatusError
	VAX_FWDWORD       = VAXFacility<<FacilityPosition | vaxFwdWord<<MessagePosition | StatusError
	VAX_FWDBYTE       = VAXFacility<<FacilityPosition | vaxFwdByte<<MessagePosition | StatusError
	VAX_BADFLOAT      = VAXFacility<<FacilityPosition | vaxBadFloat<<MessagePosition | StatusError
	VAX_FWDOPERATOR   = VAXFacility<<FacilityPosition | vaxFwdOperator<<MessagePosition | StatusError
	VAX_DIVZERO       = VAXFacility<<FacilityPosition | vaxDivZero<<MessagePosition | StatusError
	VAX_INCOMPLETENUM = VAXFacility<<FacilityPosition | vaxIncompleteNum<<MessagePosition | StatusError
	VAX_FLOATHERE     = VAXFacility<<FacilityPosition | vaxFloatHere<<MessagePosition | StatusError
	VAX_BADDECIMAL    = VAXFacility<<FacilityPosition | vaxBadDecimal<<MessagePosition | StatusError
	VAX_BADHEX        = VAXFacility<<FacilityPosition | vaxBadHex<<MessagePosition | StatusError
	VAX_BADCHARLIT    = VAXFacility<<FacilityPosition | vaxBadCharLit<<MessagePosition | StatusError
	VAX_UNTERMCHAR    = VAXFacility<<FacilityPosition | vaxUntermChar<<MessagePosition | StatusError
	VAX_CHARTOOLONG   = VAXFacility<<FacilityPosition | vaxCharTooLong<<MessagePosition | StatusError
	VAX_BADMASK       = VAXFacility<<FacilityPosition | vaxBadMask<<MessagePosition | StatusError
	VAX_BADMASKENTRY  = VAXFacility<<FacilityPosition | vaxBadMaskEntry<<MessagePosition | StatusError

	// main.go's top-level startup diagnostics -- StatusSevere, since
	// each one aborts startup entirely.
	VAX_GRAMMAR  = VAXFacility<<FacilityPosition | vaxGrammar<<MessagePosition | StatusSevere
	VAX_ALLOCVAX = VAXFacility<<FacilityPosition | vaxAllocVAX<<MessagePosition | StatusSevere
	VAX_READLINE = VAXFacility<<FacilityPosition | vaxReadline<<MessagePosition | StatusSevere

	// Return code to exit the emulation engine entirely.
	VAX_QUIT = VAXFacility<<FacilityPosition | vaxQuit<<MessagePosition | StatusSuccess

	VAX_NOCLOSE   = VAXFacility<<FacilityPosition | vaxNoClose<<MessagePosition | StatusError
	VAX_BADSTRING = VAXFacility<<FacilityPosition | vaxBadString<<MessagePosition | StatusError
	VAX_EXTRATEXT = VAXFacility<<FacilityPosition | vaxExtraText<<MessagePosition | StatusError
	VAX_BADDIGIT  = VAXFacility<<FacilityPosition | vaxBadDigit<<MessagePosition | StatusError
	VAX_BADCOND   = VAXFacility<<FacilityPosition | vaxBadCond<<MessagePosition | StatusError
	VAX_NOCOND    = VAXFacility<<FacilityPosition | vaxNoCond<<MessagePosition | StatusError
	VAX_NOENDC    = VAXFacility<<FacilityPosition | vaxNoEndc<<MessagePosition | StatusError
	VAX_CONDDEPTH = VAXFacility<<FacilityPosition | vaxCondDepth<<MessagePosition | StatusError
	VAX_NOTMACRO  = VAXFacility<<FacilityPosition | vaxNotMACRO<<MessagePosition | StatusError
	VAX_RELEXPR   = VAXFacility<<FacilityPosition | vaxRelExpr<<MessagePosition | StatusError
	VAX_MACROONLY = VAXFacility<<FacilityPosition | vaxMACROOnly<<MessagePosition | StatusError

	// MACRO-32 program sections and directives (docs/PHASE-27.md, subtask 6).
	VAX_PSECTATTR     = VAXFacility<<FacilityPosition | vaxPsectAttr<<MessagePosition | StatusError
	VAX_PSECTCONFLICT = VAXFacility<<FacilityPosition | vaxPsectConflict<<MessagePosition | StatusError
	VAX_ABSDATA       = VAXFacility<<FacilityPosition | vaxAbsData<<MessagePosition | StatusError
	VAX_ALIGNPSECT    = VAXFacility<<FacilityPosition | vaxAlignPsect<<MessagePosition | StatusError
	VAX_PSECTSTACK    = VAXFacility<<FacilityPosition | vaxPsectStack<<MessagePosition | StatusError
	VAX_TOOMANYPSECTS = VAXFacility<<FacilityPosition | vaxTooManyPsects<<MessagePosition | StatusError
	VAX_BADKEYWORD    = VAXFacility<<FacilityPosition | vaxBadKeyword<<MessagePosition | StatusError
	VAX_ENTRYMASK     = VAXFacility<<FacilityPosition | vaxEntryMask<<MessagePosition | StatusError
	VAX_NOTENTRY      = VAXFacility<<FacilityPosition | vaxNotEntry<<MessagePosition | StatusError
	VAX_NOTGLOBAL     = VAXFacility<<FacilityPosition | vaxNotGlobal<<MessagePosition | StatusError
	// VAX_IGNORED is a warning: assembly goes on without the feature.
	VAX_IGNORED = VAXFacility<<FacilityPosition | vaxIgnored<<MessagePosition | StatusWarning

	// MACRO-32 macros (docs/PHASE-28.md).
	VAX_NOENDM      = VAXFacility<<FacilityPosition | vaxNoEndm<<MessagePosition | StatusError
	VAX_ENDMNAME    = VAXFacility<<FacilityPosition | vaxEndmName<<MessagePosition | StatusError
	VAX_NOTINDEF    = VAXFacility<<FacilityPosition | vaxNotInDef<<MessagePosition | StatusError
	VAX_NOTINMACRO  = VAXFacility<<FacilityPosition | vaxNotInMacro<<MessagePosition | StatusError
	VAX_MACRONAME   = VAXFacility<<FacilityPosition | vaxMacroName<<MessagePosition | StatusError
	VAX_MACRODEPTH  = VAXFacility<<FacilityPosition | vaxMacroDepth<<MessagePosition | StatusError
	VAX_TOOMNYARGS  = VAXFacility<<FacilityPosition | vaxTooManyArgs<<MessagePosition | StatusError
	VAX_BADFORMAL   = VAXFacility<<FacilityPosition | vaxBadFormal<<MessagePosition | StatusError
	VAX_BADOPERATOR = VAXFacility<<FacilityPosition | vaxBadOperator<<MessagePosition | StatusError
	VAX_NOENDR      = VAXFacility<<FacilityPosition | vaxNoEndr<<MessagePosition | StatusError
	VAX_NOTINREPEAT = VAXFacility<<FacilityPosition | vaxNotInRepeat<<MessagePosition | StatusError

	// The message directives .ERROR and .WARN (MACRO's GENERR and
	// GENWRN).
	VAX_GENERR = VAXFacility<<FacilityPosition | vaxGenErr<<MessagePosition | StatusError
	VAX_GENWRN = VAXFacility<<FacilityPosition | vaxGenWrn<<MessagePosition | StatusWarning

	// Macro libraries: .MCALL of a macro no library has, a library that
	// can't be read, and .LIBRARY with no way to open, or failing to
	// open, its file.
	VAX_UNDEFMACRO    = VAXFacility<<FacilityPosition | vaxUndefMacro<<MessagePosition | StatusError
	VAX_LIBREAD       = VAXFacility<<FacilityPosition | vaxLibRead<<MessagePosition | StatusError
	VAX_NOLIBRESOLVER = VAXFacility<<FacilityPosition | vaxNoLibResolver<<MessagePosition | StatusError
	VAX_LIBRARY       = VAXFacility<<FacilityPosition | vaxLibrary<<MessagePosition | StatusError

	// An addressing mode that faults (or is UNPREDICTABLE) for the
	// operand's access type, or as an indexed operand's base (the
	// architecture manual's tables 8-5 and 8-6).
	VAX_MODEACCESS   = VAXFacility<<FacilityPosition | vaxModeAccess<<MessagePosition | StatusError
	// VAX_ILLEXPR is MACRO-32's "Illegal expression": for example, a
	// parenthesis in an expression, which MACRO doesn't group with.
	VAX_ILLEXPR = VAXFacility<<FacilityPosition | vaxIllExpr<<MessagePosition | StatusError
	// VAX_BADPACKED is a .PACKED operand that isn't a decimal string of 0
	// to 31 digits.
	VAX_BADPACKED = VAXFacility<<FacilityPosition | vaxBadPacked<<MessagePosition | StatusError
)

func init() {
	DefineMessage(VAX_HALTED, VAXFacility, "HALTED", "CPU halted")
	DefineMessage(VAX_INSTLIM, VAXFacility, "INSTLIM", "Instruction limit exceeded")
	DefineMessage(VAX_TIMELIM, VAXFacility, "TIMELIM", "Time limit exceeded")
	DefineMessage(VAX_ATTENTION, VAXFacility, "ATTENTION", "User requested attention (Ctrl-C)")
	DefineMessage(VAX_NOHANDLER, VAXFacility, "NOHANDLER", "No exception handler installed for this SCB vector")
	DefineMessage(VAX_UNHANDLED, VAXFacility, "UNHANDLED", "Exception vector is zero")
	DefineMessage(VAX_IMMOPND, VAXFacility, "IMMOPND", "Cannot store to an immediate operand")
	DefineMessage(VAX_CALLRET, VAXFacility, "CALLRET", "Console call returned")

	DefineMessage(VAX_BADREG, VAXFacility, "BADREG", "Invalid register specification")
	DefineMessage(VAX_BADOPCODE, VAXFacility, "BADOPCODE", "Invalid opcode !Q")
	DefineMessage(VAX_BADOPERANDS, VAXFacility, "BADOPERANDS", "Instruction !S: insufficient operands")
	DefineMessage(VAX_OPERANDERR, VAXFacility, "OPERANDERR", "Instruction !S operand !D")
	DefineMessage(VAX_DEFARG, VAXFacility, "DEFARG", "DEFINED() requires an argument")
	DefineMessage(VAX_DEFQUOTE, VAXFacility, "DEFQUOTE", "DEFINED() requires a quoted symbol name")
	DefineMessage(VAX_BADOPCODEAT, VAXFacility, "BADOPCODEAT", "Invalid opcode at !XL")
	DefineMessage(VAX_INTERNAL, VAXFacility, "INTERNAL", "Internal error: !S")
	DefineMessage(VAX_INDEXNEST, VAXFacility, "INDEXNEST", "Indexed addressing mode may not nest")
	DefineMessage(VAX_INCLUDEDEPTH, VAXFacility, "INCLUDEDEPTH", "Include nesting too deep (possible cycle)")
	DefineMessage(VAX_NOTLIVE, VAXFacility, "NOTLIVE", "!S is not supported outside a live console/VM")
	DefineMessage(VAX_DATARANGE, VAXFacility, "DATARANGE", "!S value !D out of range")
	DefineMessage(VAX_UNDEFSYM, VAXFacility, "UNDEFSYM", "Undefined symbol !Q")
	DefineMessage(VAX_NEEDMICRO, VAXFacility, "NEEDMICRO", "!S requires .MICROKERNEL")
	DefineMessage(VAX_SCBCODE, VAXFacility, "SCBCODE", ".SCB vector code !XL out of range")
	DefineMessage(VAX_SCBALIGN, VAXFacility, "SCBALIGN", ".SCB target address !XL is not quadword aligned")
	DefineMessage(VAX_SCBSTACK, VAXFacility, "SCBSTACK", ".SCB: invalid stack specifier")
	DefineMessage(VAX_ALIGNZERO, VAXFacility, "ALIGNZERO", ".ALIGN size must be nonzero")
	DefineMessage(VAX_REGIONSPEC, VAXFacility, "REGIONSPEC", ".REGION: invalid region specifier")
	DefineMessage(VAX_BADSCALE, VAXFacility, "BADSCALE", "Unsupported operand scale !D")
	DefineMessage(VAX_NEEDHASH, VAXFacility, "NEEDHASH", "Implicit immediate operand requires '#'")
	DefineMessage(VAX_BADSHORTLIT, VAXFacility, "BADSHORTLIT", "Invalid short literal syntax")
	DefineMessage(VAX_SHORTRANGE, VAXFacility, "SHORTRANGE", "Short literal out of range")
	DefineMessage(VAX_BADSHORTFLOAT, VAXFacility, "BADSHORTFLOAT", "Value is not a valid short float literal")
	DefineMessage(VAX_BADMODE, VAXFacility, "BADMODE", "Invalid addressing mode")
	DefineMessage(VAX_BADIMMLIT, VAXFacility, "BADIMMLIT", "Invalid immediate literal syntax")
	DefineMessage(VAX_FLOATRANGE, VAXFacility, "FLOATRANGE", "Floating literal out of range")
	DefineMessage(VAX_DUPSYM, VAXFacility, "DUPSYM", "Duplicate symbol definition !Q")
	DefineMessage(VAX_FWDWORD, VAXFacility, "FWDWORD", "Forward reference displacement !D out of word range")
	DefineMessage(VAX_FWDBYTE, VAXFacility, "FWDBYTE", "Forward reference displacement !D out of byte range")
	DefineMessage(VAX_BADFLOAT, VAXFacility, "BADFLOAT", "Invalid floating point value !Q")
	DefineMessage(VAX_FWDOPERATOR, VAXFacility, "FWDOPERATOR", "A forward-referenced symbol cannot be combined with an operator")
	DefineMessage(VAX_DIVZERO, VAXFacility, "DIVZERO", "Division by zero")
	DefineMessage(VAX_INCOMPLETENUM, VAXFacility, "INCOMPLETENUM", "Incomplete numeric value")
	DefineMessage(VAX_FLOATHERE, VAXFacility, "FLOATHERE", "Floating point value not valid here")
	DefineMessage(VAX_BADDECIMAL, VAXFacility, "BADDECIMAL", "Invalid decimal constant")
	DefineMessage(VAX_BADHEX, VAXFacility, "BADHEX", "Invalid hexadecimal constant")
	DefineMessage(VAX_BADCHARLIT, VAXFacility, "BADCHARLIT", "Invalid character literal")
	DefineMessage(VAX_UNTERMCHAR, VAXFacility, "UNTERMCHAR", "Unterminated character literal")
	DefineMessage(VAX_CHARTOOLONG, VAXFacility, "CHARTOOLONG", "Character literal too long")
	DefineMessage(VAX_BADMASK, VAXFacility, "BADMASK", "Invalid register mask")
	DefineMessage(VAX_BADMASKENTRY, VAXFacility, "BADMASKENTRY", "Invalid register mask entry !Q")
	DefineMessage(VAX_NOCLOSE, VAXFacility, "NOCLOSE", "Missing closing delimiter !Q")
	DefineMessage(VAX_BADSTRING, VAXFacility, "BADSTRING", "Invalid string delimiter !Q")
	DefineMessage(VAX_BADDIGIT, VAXFacility, "BADDIGIT", "Invalid digit for radix !D")
	DefineMessage(VAX_BADCOND, VAXFacility, "BADCOND", "Invalid conditional assembly: !S")
	DefineMessage(VAX_NOCOND, VAXFacility, "NOCOND", "!S is not inside a conditional assembly block")
	DefineMessage(VAX_NOENDC, VAXFacility, "NOENDC", "Missing .ENDC: !D conditional assembly block(s) still open")
	DefineMessage(VAX_CONDDEPTH, VAXFacility, "CONDDEPTH", "Conditional assembly blocks nested more than !D deep")
	DefineMessage(VAX_NOTMACRO, VAXFacility, "NOTMACRO", "!S is not a MACRO-32 directive")
	DefineMessage(VAX_RELEXPR, VAXFacility, "RELEXPR", "Relocatable or external value not allowed here")
	DefineMessage(VAX_MACROONLY, VAXFacility, "MACROONLY", "!S is only valid in MACRO-32 source")
	DefineMessage(VAX_PSECTATTR, VAXFacility, "PSECTATTR", "Invalid program section attribute !Q")
	DefineMessage(VAX_PSECTCONFLICT, VAXFacility, "PSECTCONFLICT", "Attribute !S conflicts with program section !S's definition")
	DefineMessage(VAX_ABSDATA, VAXFacility, "ABSDATA", "Code or data can't be stored in absolute program section !S")
	DefineMessage(VAX_ALIGNPSECT, VAXFacility, "ALIGNPSECT", "Alignment 2^!D exceeds program section !S's alignment")
	DefineMessage(VAX_PSECTSTACK, VAXFacility, "PSECTSTACK", "Program section context stack is !S")
	DefineMessage(VAX_TOOMANYPSECTS, VAXFacility, "TOOMANYPSECTS", "More than !D program sections")
	DefineMessage(VAX_BADKEYWORD, VAXFacility, "BADKEYWORD", "!S: unknown keyword !Q")
	DefineMessage(VAX_ENTRYMASK, VAXFacility, "ENTRYMASK", "Entry mask !XW uses a reserved register (R0, R1, AP, or FP)")
	DefineMessage(VAX_NOTENTRY, VAXFacility, "NOTENTRY", "!S is not an entry point")
	DefineMessage(VAX_NOTGLOBAL, VAXFacility, "NOTGLOBAL", "A local label can't be global: !S")
	DefineMessage(VAX_IGNORED, VAXFacility, "IGNORED", "!S is not supported and was ignored")
	DefineMessage(VAX_EXTRATEXT, VAXFacility, "EXTRATEXT", "Unexpected text !Q at end of statement")
	DefineMessage(VAX_NOENDM, VAXFacility, "NOENDM", "Missing .ENDM for macro !S")
	DefineMessage(VAX_ENDMNAME, VAXFacility, "ENDMNAME", ".ENDM !S does not match macro !S")
	DefineMessage(VAX_NOTINDEF, VAXFacility, "NOTINDEF", "!S is not inside a macro definition")
	DefineMessage(VAX_NOTINMACRO, VAXFacility, "NOTINMACRO", "!S is not inside a macro expansion or repeat block")
	DefineMessage(VAX_MACRONAME, VAXFacility, "MACRONAME", "Missing or invalid macro name !Q")
	DefineMessage(VAX_MACRODEPTH, VAXFacility, "MACRODEPTH", "Macro expansions nested more than !D deep")
	DefineMessage(VAX_TOOMNYARGS, VAXFacility, "TOOMNYARGS", "Too many arguments in call of macro !S")
	DefineMessage(VAX_BADFORMAL, VAXFacility, "BADFORMAL", "Invalid formal argument !Q")
	DefineMessage(VAX_BADOPERATOR, VAXFacility, "BADOPERATOR", "Wrong number of arguments to string operator !S")
	DefineMessage(VAX_NOENDR, VAXFacility, "NOENDR", "Missing .ENDR for !S repeat block")
	DefineMessage(VAX_NOTINREPEAT, VAXFacility, "NOTINREPEAT", "!S is not inside a repeat block")
	DefineMessage(VAX_GENERR, VAXFacility, "GENERR", "Generated ERROR: !S")
	DefineMessage(VAX_GENWRN, VAXFacility, "GENWRN", "Generated WARNING: !S")
	DefineMessage(VAX_UNDEFMACRO, VAXFacility, "UNDEFMACRO", "Macro !S is not in any macro library")
	DefineMessage(VAX_LIBREAD, VAXFacility, "LIBREAD", "Reading macro !S from its library")
	DefineMessage(VAX_NOLIBRESOLVER, VAXFacility, "NOLIBRESOLVER", ".LIBRARY !Q: no library resolver configured")
	DefineMessage(VAX_LIBRARY, VAXFacility, "LIBRARY", ".LIBRARY !Q")
	DefineMessage(VAX_MODEACCESS, VAXFacility, "MODEACCESS", "!S mode isn't allowed for !S operand")
	DefineMessage(VAX_ILLEXPR, VAXFacility, "ILLEXPR", "Illegal expression")
	DefineMessage(VAX_BADPACKED, VAXFacility, "BADPACKED", "Invalid packed decimal string")

	DefineMessage(VAX_GRAMMAR, VAXFacility, "GRAMMAR", "Loading command grammar")
	DefineMessage(VAX_ALLOCVAX, VAXFacility, "ALLOCVAX", "Allocating initial VAX")
	DefineMessage(VAX_READLINE, VAXFacility, "READLINE", "Initializing readline")
}
