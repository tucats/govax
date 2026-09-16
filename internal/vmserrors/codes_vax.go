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

	// cmd/govax's top-level startup diagnostics.
	vaxGrammar
	vaxAllocVAX
	vaxReadline
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

	// cmd/govax's top-level startup diagnostics -- StatusSevere, since
	// each one aborts startup entirely.
	VAX_GRAMMAR  = VAXFacility<<FacilityPosition | vaxGrammar<<MessagePosition | StatusSevere
	VAX_ALLOCVAX = VAXFacility<<FacilityPosition | vaxAllocVAX<<MessagePosition | StatusSevere
	VAX_READLINE = VAXFacility<<FacilityPosition | vaxReadline<<MessagePosition | StatusSevere
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

	DefineMessage(VAX_GRAMMAR, VAXFacility, "GRAMMAR", "Loading command grammar")
	DefineMessage(VAX_ALLOCVAX, VAXFacility, "ALLOCVAX", "Allocating initial VAX")
	DefineMessage(VAX_READLINE, VAXFacility, "READLINE", "Initializing readline")
}
