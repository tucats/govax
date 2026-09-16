package vmserrors

// CLI facility message IDs -- internal/console's command-line/monitor
// diagnostics, plus internal/console/dcl's grammar-parsing diagnostics.
// Private: only the composite CLI_* codes below are part of this package's
// public API.
const (
	cliUnkverb uint32 = iota + 1

	// internal/console (the interactive monitor), roughly in the order
	// its files (exam/disasm/run/misc/show/dispatch/vminit/asm/machine/
	// expr/set) encounter them.
	cliBadReg
	cliDisasm
	cliActivate
	cliFixup
	cliNoScratch
	cliUntermStr
	cliNotImpl
	cliNoStringPool
	cliNoPoolSize
	cliNoPool
	cliBadCount
	cliNoFrames
	cliUndefSym
	cliNoProfile
	cliNoModes
	cliBadOpcode
	cliNeedRTL
	cliBadHexVal
	cliNeedDep
	cliNeedPages
	cliNoFile
	cliNeedEntry
	cliIncompleteArgs
	cliNeedComma
	cliBadRange
	cliNeedDeposit
	cliNeedSetArg
	cliNeedRadix
	cliBadRadixVal
	cliNeedBreakAddr
	cliBadSetSyntax
	cliBadDebugFlag
	cliNeedRomNvram
	cliNeedFilename
	cliVMTooLarge
	cliS0TooSmall
	cliAssembling
	cliDepositing
	cliNoVAX
	cliNotKernel
	cliDivZero
	cliNeedExpr
	cliNeedParen
	cliDefArg
	cliDefQuote
	cliBadRadixPrefix
	cliBadNumber
	cliBadRadix
	cliSpoolOvf
	cliNeedStepMode
	cliNeedBreakOpcode
	cliInvSetPSL
	cliNeedMode
	cliBadMode
	cliBadPTEField
	cliBadQualPrefix

	// internal/console/dcl (the DCL command-grammar interpreter).
	cliUnrecognized
	cliAmbiguous
	cliLineErr
	cliOutside
	cliBadSwitch
	cliRedefined
	cliBadDirective
	cliNoGrammar
	cliEmptyStatement
	cliDisallowExpr
	cliUntermQuote
	cliBadDisallow
	cliNoHandler
	cliAliasNotFound
	cliQualAliasNotFound
	cliTypeNotFound
	cliSyntaxNotFound
	cliDisallowNotFound
	cliKeywordSyntaxNotFound
	cliEmptyCommand
	cliExtraParameter
	cliBadParameter
	cliNeedQualifierName
	cliNoNegate
	cliNoQualifierValue
	cliNeedQualifierValue
	cliBadQualifier
	cliUnknownType
	cliMissingParameter
	cliBadQualifierCombo
	cliBadInteger
)

// CLI facility status codes -- CLI_ prefix, matching real VMS's CLI$_
// status codes.
const (
	// CLI_UNKVERB reports an unrecognized command verb, matching real
	// VMS's CLI$_UNKVERB.
	CLI_UNKVERB = CLIFacility<<FacilityPosition | cliUnkverb<<MessagePosition | StatusError

	CLI_BADREG          = CLIFacility<<FacilityPosition | cliBadReg<<MessagePosition | StatusError
	CLI_DISASM          = CLIFacility<<FacilityPosition | cliDisasm<<MessagePosition | StatusError
	CLI_ACTIVATE        = CLIFacility<<FacilityPosition | cliActivate<<MessagePosition | StatusError
	CLI_FIXUP           = CLIFacility<<FacilityPosition | cliFixup<<MessagePosition | StatusError
	CLI_NOSCRATCH       = CLIFacility<<FacilityPosition | cliNoScratch<<MessagePosition | StatusError
	CLI_UNTERMSTR       = CLIFacility<<FacilityPosition | cliUntermStr<<MessagePosition | StatusError
	CLI_NOTIMPL         = CLIFacility<<FacilityPosition | cliNotImpl<<MessagePosition | StatusError
	CLI_NOSTRINGPOOL    = CLIFacility<<FacilityPosition | cliNoStringPool<<MessagePosition | StatusError
	CLI_NOPOOLSIZE      = CLIFacility<<FacilityPosition | cliNoPoolSize<<MessagePosition | StatusError
	CLI_NOPOOL          = CLIFacility<<FacilityPosition | cliNoPool<<MessagePosition | StatusError
	CLI_BADCOUNT        = CLIFacility<<FacilityPosition | cliBadCount<<MessagePosition | StatusError
	CLI_NOFRAMES        = CLIFacility<<FacilityPosition | cliNoFrames<<MessagePosition | StatusError
	CLI_UNDEFSYM        = CLIFacility<<FacilityPosition | cliUndefSym<<MessagePosition | StatusError
	CLI_NOPROFILE       = CLIFacility<<FacilityPosition | cliNoProfile<<MessagePosition | StatusError
	CLI_NOMODES         = CLIFacility<<FacilityPosition | cliNoModes<<MessagePosition | StatusError
	CLI_BADOPCODE       = CLIFacility<<FacilityPosition | cliBadOpcode<<MessagePosition | StatusError
	CLI_NEEDRTL         = CLIFacility<<FacilityPosition | cliNeedRTL<<MessagePosition | StatusError
	CLI_BADHEXVAL       = CLIFacility<<FacilityPosition | cliBadHexVal<<MessagePosition | StatusError
	CLI_NEEDDEP         = CLIFacility<<FacilityPosition | cliNeedDep<<MessagePosition | StatusError
	CLI_NEEDPAGES       = CLIFacility<<FacilityPosition | cliNeedPages<<MessagePosition | StatusError
	CLI_NOFILE          = CLIFacility<<FacilityPosition | cliNoFile<<MessagePosition | StatusError
	CLI_NEEDENTRY       = CLIFacility<<FacilityPosition | cliNeedEntry<<MessagePosition | StatusError
	CLI_INCOMPLETEARGS  = CLIFacility<<FacilityPosition | cliIncompleteArgs<<MessagePosition | StatusError
	CLI_NEEDCOMMA       = CLIFacility<<FacilityPosition | cliNeedComma<<MessagePosition | StatusError
	CLI_BADRANGE        = CLIFacility<<FacilityPosition | cliBadRange<<MessagePosition | StatusError
	CLI_NEEDDEPOSIT     = CLIFacility<<FacilityPosition | cliNeedDeposit<<MessagePosition | StatusError
	CLI_NEEDSETARG      = CLIFacility<<FacilityPosition | cliNeedSetArg<<MessagePosition | StatusError
	CLI_NEEDRADIX       = CLIFacility<<FacilityPosition | cliNeedRadix<<MessagePosition | StatusError
	CLI_BADRADIXVAL     = CLIFacility<<FacilityPosition | cliBadRadixVal<<MessagePosition | StatusError
	CLI_NEEDBREAKADDR   = CLIFacility<<FacilityPosition | cliNeedBreakAddr<<MessagePosition | StatusError
	CLI_BADSETSYNTAX    = CLIFacility<<FacilityPosition | cliBadSetSyntax<<MessagePosition | StatusError
	CLI_BADDEBUGFLAG    = CLIFacility<<FacilityPosition | cliBadDebugFlag<<MessagePosition | StatusError
	CLI_NEEDROMNVRAM    = CLIFacility<<FacilityPosition | cliNeedRomNvram<<MessagePosition | StatusError
	CLI_NEEDFILENAME    = CLIFacility<<FacilityPosition | cliNeedFilename<<MessagePosition | StatusError
	CLI_VMTOOLARGE      = CLIFacility<<FacilityPosition | cliVMTooLarge<<MessagePosition | StatusError
	CLI_S0TOOSMALL      = CLIFacility<<FacilityPosition | cliS0TooSmall<<MessagePosition | StatusError
	CLI_ASSEMBLING      = CLIFacility<<FacilityPosition | cliAssembling<<MessagePosition | StatusError
	CLI_DEPOSITING      = CLIFacility<<FacilityPosition | cliDepositing<<MessagePosition | StatusError
	CLI_NOVAX           = CLIFacility<<FacilityPosition | cliNoVAX<<MessagePosition | StatusError
	CLI_NOTKERNEL       = CLIFacility<<FacilityPosition | cliNotKernel<<MessagePosition | StatusError
	CLI_DIVZERO         = CLIFacility<<FacilityPosition | cliDivZero<<MessagePosition | StatusError
	CLI_NEEDEXPR        = CLIFacility<<FacilityPosition | cliNeedExpr<<MessagePosition | StatusError
	CLI_NEEDPAREN       = CLIFacility<<FacilityPosition | cliNeedParen<<MessagePosition | StatusError
	CLI_DEFARG          = CLIFacility<<FacilityPosition | cliDefArg<<MessagePosition | StatusError
	CLI_DEFQUOTE        = CLIFacility<<FacilityPosition | cliDefQuote<<MessagePosition | StatusError
	CLI_BADRADIXPREFIX  = CLIFacility<<FacilityPosition | cliBadRadixPrefix<<MessagePosition | StatusError
	CLI_BADNUMBER       = CLIFacility<<FacilityPosition | cliBadNumber<<MessagePosition | StatusError
	CLI_BADRADIX        = CLIFacility<<FacilityPosition | cliBadRadix<<MessagePosition | StatusError
	CLI_SPOOLOVF        = CLIFacility<<FacilityPosition | cliSpoolOvf<<MessagePosition | StatusError
	CLI_NEEDSTEPMODE    = CLIFacility<<FacilityPosition | cliNeedStepMode<<MessagePosition | StatusError
	CLI_NEEDBREAKOPCODE = CLIFacility<<FacilityPosition | cliNeedBreakOpcode<<MessagePosition | StatusError
	CLI_INVSETPSL       = CLIFacility<<FacilityPosition | cliInvSetPSL<<MessagePosition | StatusError
	CLI_NEEDMODE        = CLIFacility<<FacilityPosition | cliNeedMode<<MessagePosition | StatusError
	CLI_BADMODE         = CLIFacility<<FacilityPosition | cliBadMode<<MessagePosition | StatusError
	CLI_BADPTEFIELD     = CLIFacility<<FacilityPosition | cliBadPTEField<<MessagePosition | StatusError
	CLI_BADQUALPREFIX   = CLIFacility<<FacilityPosition | cliBadQualPrefix<<MessagePosition | StatusError

	// internal/console/dcl
	CLI_UNRECOGNIZED          = CLIFacility<<FacilityPosition | cliUnrecognized<<MessagePosition | StatusError
	CLI_AMBIGUOUS             = CLIFacility<<FacilityPosition | cliAmbiguous<<MessagePosition | StatusError
	CLI_LINEERR               = CLIFacility<<FacilityPosition | cliLineErr<<MessagePosition | StatusError
	CLI_OUTSIDE               = CLIFacility<<FacilityPosition | cliOutside<<MessagePosition | StatusError
	CLI_BADSWITCH             = CLIFacility<<FacilityPosition | cliBadSwitch<<MessagePosition | StatusError
	CLI_REDEFINED             = CLIFacility<<FacilityPosition | cliRedefined<<MessagePosition | StatusError
	CLI_BADDIRECTIVE          = CLIFacility<<FacilityPosition | cliBadDirective<<MessagePosition | StatusError
	CLI_NOGRAMMAR             = CLIFacility<<FacilityPosition | cliNoGrammar<<MessagePosition | StatusError
	CLI_EMPTYSTATEMENT        = CLIFacility<<FacilityPosition | cliEmptyStatement<<MessagePosition | StatusError
	CLI_DISALLOWEXPR          = CLIFacility<<FacilityPosition | cliDisallowExpr<<MessagePosition | StatusError
	CLI_UNTERMQUOTE           = CLIFacility<<FacilityPosition | cliUntermQuote<<MessagePosition | StatusError
	CLI_BADDISALLOW           = CLIFacility<<FacilityPosition | cliBadDisallow<<MessagePosition | StatusError
	CLI_NOHANDLER             = CLIFacility<<FacilityPosition | cliNoHandler<<MessagePosition | StatusError
	CLI_ALIASNOTFOUND         = CLIFacility<<FacilityPosition | cliAliasNotFound<<MessagePosition | StatusError
	CLI_QUALALIASNOTFOUND     = CLIFacility<<FacilityPosition | cliQualAliasNotFound<<MessagePosition | StatusError
	CLI_TYPENOTFOUND          = CLIFacility<<FacilityPosition | cliTypeNotFound<<MessagePosition | StatusError
	CLI_SYNTAXNOTFOUND        = CLIFacility<<FacilityPosition | cliSyntaxNotFound<<MessagePosition | StatusError
	CLI_DISALLOWNOTFOUND      = CLIFacility<<FacilityPosition | cliDisallowNotFound<<MessagePosition | StatusError
	CLI_KEYWORDSYNTAXNOTFOUND = CLIFacility<<FacilityPosition | cliKeywordSyntaxNotFound<<MessagePosition | StatusError
	CLI_EMPTYCOMMAND          = CLIFacility<<FacilityPosition | cliEmptyCommand<<MessagePosition | StatusError
	CLI_EXTRAPARAMETER        = CLIFacility<<FacilityPosition | cliExtraParameter<<MessagePosition | StatusError
	CLI_BADPARAMETER          = CLIFacility<<FacilityPosition | cliBadParameter<<MessagePosition | StatusError
	CLI_NEEDQUALIFIERNAME     = CLIFacility<<FacilityPosition | cliNeedQualifierName<<MessagePosition | StatusError
	CLI_NONEGATE              = CLIFacility<<FacilityPosition | cliNoNegate<<MessagePosition | StatusError
	CLI_NOQUALIFIERVALUE      = CLIFacility<<FacilityPosition | cliNoQualifierValue<<MessagePosition | StatusError
	CLI_NEEDQUALIFIERVALUE    = CLIFacility<<FacilityPosition | cliNeedQualifierValue<<MessagePosition | StatusError
	CLI_BADQUALIFIER          = CLIFacility<<FacilityPosition | cliBadQualifier<<MessagePosition | StatusError
	CLI_UNKNOWNTYPE           = CLIFacility<<FacilityPosition | cliUnknownType<<MessagePosition | StatusError
	CLI_MISSINGPARAMETER      = CLIFacility<<FacilityPosition | cliMissingParameter<<MessagePosition | StatusError
	CLI_BADQUALIFIERCOMBO     = CLIFacility<<FacilityPosition | cliBadQualifierCombo<<MessagePosition | StatusError
	CLI_BADINTEGER            = CLIFacility<<FacilityPosition | cliBadInteger<<MessagePosition | StatusError
)

func init() {
	DefineMessage(CLI_UNKVERB, CLIFacility, "UNKVERB", "Unknown verb: !S")

	DefineMessage(CLI_BADREG, CLIFacility, "BADREG", "Unknown register !Q")
	DefineMessage(CLI_DISASM, CLIFacility, "DISASM", "Disassemble at !XL")
	DefineMessage(CLI_ACTIVATE, CLIFacility, "ACTIVATE", "Unable to activate !S")
	DefineMessage(CLI_FIXUP, CLIFacility, "FIXUP", "Relocation/fixup error for image !S")
	DefineMessage(CLI_NOSCRATCH, CLIFacility, "NOSCRATCH", "No CONSOLE$SCRATCH area (VMINIT required)")
	DefineMessage(CLI_UNTERMSTR, CLIFacility, "UNTERMSTR", "Unterminated quoted string")
	DefineMessage(CLI_NOTIMPL, CLIFacility, "NOTIMPL", "SHOW !S is not implemented")
	DefineMessage(CLI_NOSTRINGPOOL, CLIFacility, "NOSTRINGPOOL", "CONSOLE$STRINGPOOL_BASE is undefined (boot the microkernel first)")
	DefineMessage(CLI_NOPOOLSIZE, CLIFacility, "NOPOOLSIZE", "CONSOLE$STRINGPOOL_SIZE is undefined")
	DefineMessage(CLI_NOPOOL, CLIFacility, "NOPOOL", "CONSOLE$STRINGPOOL is undefined")
	DefineMessage(CLI_BADCOUNT, CLIFacility, "BADCOUNT", "Invalid count !Q")
	DefineMessage(CLI_NOFRAMES, CLIFacility, "NOFRAMES", "No call frames (FP/AP not established)")
	DefineMessage(CLI_UNDEFSYM, CLIFacility, "UNDEFSYM", "Undefined symbol !Q")
	DefineMessage(CLI_NOPROFILE, CLIFacility, "NOPROFILE", "SHOW INSTRUCTIONS/PROFILE is not implemented (no per-opcode execution counters in this port)")
	DefineMessage(CLI_NOMODES, CLIFacility, "NOMODES", "SHOW INSTRUCTIONS/MODES is not implemented (no addressing-mode legality table exposed by this port)")
	DefineMessage(CLI_BADOPCODE, CLIFacility, "BADOPCODE", "Invalid opcode !Q")
	DefineMessage(CLI_NEEDRTL, CLIFacility, "NEEDRTL", "!S requires the RTL microkernel (not yet implemented, see docs/PHASE-08.md)")
	DefineMessage(CLI_BADHEXVAL, CLIFacility, "BADHEXVAL", "Invalid hex value !Q")
	DefineMessage(CLI_NEEDDEP, CLIFacility, "NEEDDEP", "!S requires !S (not yet implemented, see docs/PHASE-08.md)")
	DefineMessage(CLI_NEEDPAGES, CLIFacility, "NEEDPAGES", "INIT requires a page count")
	DefineMessage(CLI_NOFILE, CLIFacility, "NOFILE", "Missing file name to run")
	DefineMessage(CLI_NEEDENTRY, CLIFacility, "NEEDENTRY", "CALL requires an entry-point address or symbol")
	DefineMessage(CLI_INCOMPLETEARGS, CLIFacility, "INCOMPLETEARGS", "CALL: incomplete argument list")
	DefineMessage(CLI_NEEDCOMMA, CLIFacility, "NEEDCOMMA", "CALL: expected ',' in argument list")
	DefineMessage(CLI_BADRANGE, CLIFacility, "BADRANGE", "End address before start address")
	DefineMessage(CLI_NEEDDEPOSIT, CLIFacility, "NEEDDEPOSIT", "DEPOSIT requires an address/register and a value")
	DefineMessage(CLI_NEEDSETARG, CLIFacility, "NEEDSETARG", "SET requires an argument")
	DefineMessage(CLI_NEEDRADIX, CLIFacility, "NEEDRADIX", "SET RADIX requires a value")
	DefineMessage(CLI_BADRADIXVAL, CLIFacility, "BADRADIXVAL", "Invalid radix !Q")
	DefineMessage(CLI_NEEDBREAKADDR, CLIFacility, "NEEDBREAKADDR", "SET BREAKPOINT requires an address")
	DefineMessage(CLI_BADSETSYNTAX, CLIFacility, "BADSETSYNTAX", "Unrecognized SET syntax !Q")
	DefineMessage(CLI_BADDEBUGFLAG, CLIFacility, "BADDEBUGFLAG", "Invalid SET DEBUG flag !Q")
	DefineMessage(CLI_NEEDROMNVRAM, CLIFacility, "NEEDROMNVRAM", "SAVE/LOAD requires /ROM or /NVRAM (the plain .VAX form isn't implemented)")
	DefineMessage(CLI_NEEDFILENAME, CLIFacility, "NEEDFILENAME", "!S requires a file name")
	DefineMessage(CLI_VMTOOLARGE, CLIFacility, "VMTOOLARGE", "Requested VM size exceeds physical memory")
	DefineMessage(CLI_S0TOOSMALL, CLIFacility, "S0TOOSMALL", "S0 region too small to hold its own page tables")
	DefineMessage(CLI_ASSEMBLING, CLIFacility, "ASSEMBLING", "Assembling !S")
	DefineMessage(CLI_DEPOSITING, CLIFacility, "DEPOSITING", "Depositing !S")
	DefineMessage(CLI_NOVAX, CLIFacility, "NOVAX", "No VAX processor allocated (use INIT first)")
	DefineMessage(CLI_NOTKERNEL, CLIFacility, "NOTKERNEL", "Not permitted outside kernel mode")
	DefineMessage(CLI_DIVZERO, CLIFacility, "DIVZERO", "Division by zero")
	DefineMessage(CLI_NEEDEXPR, CLIFacility, "NEEDEXPR", "Expected an expression")
	DefineMessage(CLI_NEEDPAREN, CLIFacility, "NEEDPAREN", "Expected ')'")
	DefineMessage(CLI_DEFARG, CLIFacility, "DEFARG", "DEFINED() requires an argument")
	DefineMessage(CLI_DEFQUOTE, CLIFacility, "DEFQUOTE", "DEFINED() requires a quoted symbol name")
	DefineMessage(CLI_BADRADIXPREFIX, CLIFacility, "BADRADIXPREFIX", `Unsupported radix prefix "^!C"`)
	DefineMessage(CLI_BADNUMBER, CLIFacility, "BADNUMBER", "Invalid number !Q")
	DefineMessage(CLI_BADRADIX, CLIFacility, "BADRADIX", "Invalid radix !D (must be 8, 10, or 16)")
	DefineMessage(CLI_SPOOLOVF, CLIFacility, "SPOVF", "String pool overflow; use CLEAR STRING to reset")
	DefineMessage(CLI_NEEDSTEPMODE, CLIFacility, "NEEDSTEPMODE", "SET STEP requires OVER, INTO, or RETURN")
	DefineMessage(CLI_NEEDBREAKOPCODE, CLIFacility, "NEEDBREAKOPCODE", "SET BREAK/INSTRUCTION requires an opcode mnemonic")
	DefineMessage(CLI_INVSETPSL, CLIFacility, "INVSETPSL", "Invalid SET PSL field or value !Q")
	DefineMessage(CLI_NEEDMODE, CLIFacility, "NEEDMODE", "SET MODE requires KERNEL, EXEC, SUPER, USER, or INTERRUPT")
	DefineMessage(CLI_BADMODE, CLIFacility, "BADMODE", "Unknown mode !Q")
	DefineMessage(CLI_BADPTEFIELD, CLIFacility, "BADPTEFIELD", "Unknown SET PTE field !Q")
	DefineMessage(CLI_BADQUALPREFIX, CLIFacility, "BADQUALPREFIX", "Unknown qualifier /!S")

	DefineMessage(CLI_UNRECOGNIZED, CLIFacility, "UNRECOGNIZED", "Unrecognized !S !Q")
	DefineMessage(CLI_AMBIGUOUS, CLIFacility, "AMBIGUOUS", "Ambiguous !S !Q")
	DefineMessage(CLI_LINEERR, CLIFacility, "LINEERR", "Line !D")
	DefineMessage(CLI_OUTSIDE, CLIFacility, "OUTSIDE", "Line !D: !S outside of !S")
	DefineMessage(CLI_BADSWITCH, CLIFacility, "BADSWITCH", "Unsupported !S switch /!S")
	DefineMessage(CLI_REDEFINED, CLIFacility, "REDEFINED", "Line !D: !S !Q redefined")
	DefineMessage(CLI_BADDIRECTIVE, CLIFacility, "BADDIRECTIVE", "Line !D: unknown directive !Q")
	DefineMessage(CLI_NOGRAMMAR, CLIFacility, "NOGRAMMAR", "No GRAMMAR statement found")
	DefineMessage(CLI_EMPTYSTATEMENT, CLIFacility, "EMPTYSTATEMENT", "Empty statement")
	DefineMessage(CLI_DISALLOWEXPR, CLIFacility, "DISALLOWEXPR", "DISALLOW requires an expression")
	DefineMessage(CLI_UNTERMQUOTE, CLIFacility, "UNTERMQUOTE", "Unterminated quoted string in !Q")
	DefineMessage(CLI_BADDISALLOW, CLIFacility, "BADDISALLOW", `Malformed DISALLOW expression !Q (want "Q1 and Q2")`)
	DefineMessage(CLI_NOHANDLER, CLIFacility, "NOHANDLER", "No handler bound for !S")
	DefineMessage(CLI_ALIASNOTFOUND, CLIFacility, "ALIASNOTFOUND", "Verb !Q: alias target !Q not found")
	DefineMessage(CLI_QUALALIASNOTFOUND, CLIFacility, "QUALALIASNOTFOUND", "Qualifier !Q: alias target !Q not found")
	DefineMessage(CLI_TYPENOTFOUND, CLIFacility, "TYPENOTFOUND", "!S !Q: type !Q not found")
	DefineMessage(CLI_SYNTAXNOTFOUND, CLIFacility, "SYNTAXNOTFOUND", "Qualifier !Q: syntax !Q not found")
	DefineMessage(CLI_DISALLOWNOTFOUND, CLIFacility, "DISALLOWNOTFOUND", "Entry !Q: disallow qualifier !Q not found")
	DefineMessage(CLI_KEYWORDSYNTAXNOTFOUND, CLIFacility, "KEYWORDSYNTAXNOTFOUND", "Type !Q: keyword !Q: syntax !Q not found")
	DefineMessage(CLI_EMPTYCOMMAND, CLIFacility, "EMPTYCOMMAND", "Empty command")
	DefineMessage(CLI_EXTRAPARAMETER, CLIFacility, "EXTRAPARAMETER", "Unexpected extra parameter near !Q")
	DefineMessage(CLI_BADPARAMETER, CLIFacility, "BADPARAMETER", "Parameter !S")
	DefineMessage(CLI_NEEDQUALIFIERNAME, CLIFacility, "NEEDQUALIFIERNAME", "Expected a qualifier name after '/'")
	DefineMessage(CLI_NONEGATE, CLIFacility, "NONEGATE", "Cannot negate qualifier !Q")
	DefineMessage(CLI_NOQUALIFIERVALUE, CLIFacility, "NOQUALIFIERVALUE", "Qualifier !S does not take a value")
	DefineMessage(CLI_NEEDQUALIFIERVALUE, CLIFacility, "NEEDQUALIFIERVALUE", "Required value for qualifier !S not found")
	DefineMessage(CLI_BADQUALIFIER, CLIFacility, "BADQUALIFIER", "Qualifier !S")
	DefineMessage(CLI_UNKNOWNTYPE, CLIFacility, "UNKNOWNTYPE", "Unknown type !Q")
	DefineMessage(CLI_MISSINGPARAMETER, CLIFacility, "MISSINGPARAMETER", "Required parameter !S not found")
	DefineMessage(CLI_BADQUALIFIERCOMBO, CLIFacility, "BADQUALIFIERCOMBO", "Invalid combination of qualifiers !S and !S")
	DefineMessage(CLI_BADINTEGER, CLIFacility, "BADINTEGER", "Invalid integer !Q")
}
