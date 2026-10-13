// Package librtl emulates LIBRTL.EXE, VMS's general run-time library: the
// LIB$ and STR$ routines a program calls through the shareable image's
// transfer vector (docs/PHASE-34.md, subtask 10).
//
// # How a program reaches a routine here
//
// A program linked against LIBRTL.EXE calls, say, LIB$GET_VM through a G^
// reference that the image activator fixes up to LIBRTL's transfer vector
// plus the routine's offset. govax doesn't load a real LIBRTL.EXE; the
// console instead lays down a small stub for each routine (entry mask;
// MOVL #code, R0; XFC; RET) and points SHIM$LIBRTL_<offset> at it, so the
// fixup lands on the stub. The XFC runs the Go function registered under
// code in the process's corevms.ShimTable, with the call's argument list.
// Routines lists every routine with its code (its offset is VMS 7.3's,
// from vmsdef.ImageSymbols); Register installs them.
//
// A routine whose whole job is a transfer of control, such as LIB$CALLG,
// is VAX code in the microkernel (kernel.asm) instead: its row has code 0,
// and its SHIM$ symbol is the routine itself (docs/PHASE-53, Decision 1).
//
// # Failures that are signals
//
// A routine that can't read or write one of its arguments signals
// SS$_ACCVIO, as VMS's routine's own instruction would fault, rather
// than returning it (fault.go). Every LIBRTL.EXE entry govax doesn't
// implement has a stub too, which signals SS$_UNSUPPORTED when called
// (missing.go), so an image naming one can still run (docs/PHASE-53,
// Decisions 5 and 6).
//
// # The instruction routines
//
// Most of Phase 53's routines make a VAX instruction callable: the bit
// fields (bitfield.go), extended integers (math.go), EMOD and POLY
// (float.go), the interlocked queues and bits (queue.go), the character
// string instructions (chars.go), and CRC (crc.go). Each does its work
// with the CPU's own code, exported from internal/cpu as plain functions
// of addresses (cpu.MemoryField, cpu.ExtendedDivide, cpu.InsertInterlocked,
// cpu.LocateCharacter, cpu.MoveCharacters, cpu.CRC, ...), so a routine
// and its instruction follow one set of rules. Where the routine differs
// from the instruction -- a status in place of a trap, a position in
// place of a count, LIB$POLYx taking any degree -- it follows the LIB$
// manual, and where the manual is silent, VMS's own run of
// testdata/lib53's probe (TestLib53Oracle).
//
// # Strings, descriptors, and conversion
//
// Phase 54's routines work on strings by descriptor (descriptor.go reads
// every input class and writes S, VS, and D output). Dynamic strings
// (dynstr.go: LIB$SGET1_DD and the SFREE routines) own their heap
// blocks, and every routine that writes a class D string shares their
// storage rule. Beside them: copying and LIB$ANALYZE_SDESC (copy.go),
// LEN, CHAR, ICHAR, and INDEX (strfuncs.go), ASCII and EBCDIC
// (translate.go, over tables.go's LIB$AB_ tables), numeric text
// (cvttb.go), LIB$LOOKUP_KEY (lookupkey.go), the locale routines
// (locale.go), and LIBRTL2.EXE's LIB$CVT_DX_DX (cvtdxdx.go).
//
// Two kinds of transfer-vector entry aren't CALL routines. A JSB entry
// (LIB$SCOPY_DXDX6, LIB$ANALYZE_SDESC_R2, ...) takes its arguments in
// R0, R1, ...: its stub pushes its code for XFC$JSBSHIM, and its Go
// function (JSBRoutines, a corevms.JSBShimFunc) works on the registers.
// A data entry (LIB$AB_UPCASE and the other Tables) is a table the stubs'
// pages hold, which a program addresses directly. Both images' entries
// govax lacks, CALL and JSB, have missing-routine stubs.
//
// # Service jackets, items, resources, and timers
//
// Phase 55's routines sit on system services. The jackets
// (LIB$SYS_FAO and the like, jackets.go) call a service with a scratch
// buffer in their own frame and copy the result by the output's class;
// the item routines (items.go: LIB$GETJPI, LIB$GETDVI, LIB$GETSYI) ask
// one item of the services' Go code and write it as corevms's item text
// forms in LIB$'s style; the event flag and LUN pools (resources.go) and
// the timers' static block (timers.go) are per-image data
// (corevms.Environment.ImageData); misc.go has LIB$AST_IN_PROG,
// LIB$ASN_WTH_MBX, LIB$WAIT, and LIB$GET_USERS_LANGUAGE, and
// protection.go the 7.x protection-text routines and LIB$GET_ACCNAM's
// FILE table. SHOW_TIMER's action routine is called as the shim's tail
// call (corevms.CallRequest's Tail). VMS 7.1's run of testdata/lib55's
// probe (TestLib55Oracle) settled what the manuals left open.
//
// # Date and time
//
// Phase 56's routines work on VMS's quadword times (100 ns ticks since
// 17-Nov-1858; a delta stored negated). times.go is the arithmetic
// (LIB$ADD_TIMES, SUB_TIMES, MULT[F]_DELTA_TIME), cvttime.go the
// conversions by LIB$K_ operation code and LIB$CVT_VECTIM, days.go
// LIB$DATE_TIME, DAY, and DAY_OF_WEEK. The formatting routines share a
// table of the format mnemonics and its parser (dtformat.go) and a
// context (dtcontext.go: LIB$INIT/FREE_DATE_TIME_CONTEXT, a tagged heap
// block with its state in Go, which takes what INIT didn't give it from
// SYS$LANGUAGE, LIB$DT_FORMAT, and LIB$DT_INPUT_FORMAT); dtoutput.go
// writes by it (LIB$FORMAT_DATE_TIME, GET_MAXIMUM_DATE_LENGTH,
// GET_DATE_FORMAT) and dtinput.go reads by it (LIB$CONVERT_DATE_STRING,
// a format-driven matcher). The predefined formats and the system's
// default names are internal/lnm's (dtformat.go). VMS 7.1's runs of
// testdata/lib56 (TestLib56Oracle) settled what the manuals left open,
// but for CONVERT_DATE_STRING's strings with letters, which failed on
// that VAX (docs/DEVIATIONS.md).
//
// # What's here and what's in corevms
//
// This package holds each routine's documented interface: reading its
// arguments, checking them, and returning its status. The process it runs
// in -- the CPU and memory, call frames and the condition dispatcher, the
// heap that DECC$MALLOC shares with LIB$GET_VM, the RMS session -- is
// internal/corevms's, reached through the methods it exports for this
// (internal/corevms/export.go). Other shareable images get packages of
// their own as they're emulated.
//
// # Clean room
//
// New routines are written from DIGITAL's documentation (the Run-Time
// Library Routines manuals) and checked against VMS itself where a probe
// can; VMS's source isn't read. The routines moved here from rtl in
// Phase 34 began as ports of eVAX's librtl_*.c and keep its behavior.
package librtl
