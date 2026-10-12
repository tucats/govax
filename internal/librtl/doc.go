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
