package corevms

// ShimLink is one routine of a shareable image that a shim stands in
// for: a program linked against the image calls Name through Library's
// transfer vector at Offset, and govax puts a stub there that runs shim
// Code (XFC$SHIM). A program's G^ fixup to that entry is resolved to the
// stub through the symbol SHIM$<Library>_<Offset>.
//
// Code 0 is not a shim: it names a routine the microkernel (kernel.asm)
// implements in VAX code, and SHIM$<Library>_<Offset> is defined as
// that routine's own address instead of a stub's.
//
// JSB marks a routine reached by JSB rather than CALL (its stub is
// JSBShimStub). Data, when it isn't nil, makes the entry a table rather
// than a routine (LIBRTL's LIB$AB_UPCASE): the symbol names a copy of
// Data in system space, and Code is 0.
type ShimLink struct {
	Name    string
	Library string
	Offset  uint32
	Code    uint32
	JSB     bool
	Data    []byte
}

// CRTLLinks are the C library's routines (DECC$SHR) and govax's own
// (EVAX) that this package's shims stand in for, codes 3 to 32 and the
// traceback handler's (registerShims; the name a routine is registered
// under there is its shim's, which may differ from the one a program
// links to, as DECC$OPEN's shim is EXE$OPEN). LIBRTL.EXE's are internal/librtl's
// (librtl.Links). Offsets are each routine's transfer-vector entry.
var CRTLLinks = []ShimLink{
	{Name: "EXE$INPUT", Library: "EVAX", Offset: 0x0004, Code: 3},
	{Name: "DECC$OPEN", Library: "DECC$SHR", Offset: 0x04E8, Code: 4},
	{Name: "DECC$CLOSE", Library: "DECC$SHR", Offset: 0x0490, Code: 5},
	{Name: "DECC$READ", Library: "DECC$SHR", Offset: 0x04F0, Code: 6},
	{Name: "DECC$WRITE", Library: "DECC$SHR", Offset: 0x0508, Code: 7},
	{Name: "DECC$PRINTF", Library: "DECC$SHR", Offset: 0x0380, Code: 8},
	{Name: "DECC$SPRINTF", Library: "DECC$SHR", Offset: 0x03E0, Code: 9},
	{Name: "DECC$STRCMP", Library: "DECC$SHR", Offset: 0x06C0, Code: 10},
	{Name: "DECC$STRNCMP", Library: "DECC$SHR", Offset: 0x06F8, Code: 11},
	{Name: "DECC$STRNCPY", Library: "DECC$SHR", Offset: 0x0700, Code: 12},
	{Name: "DECC$ATOI", Library: "DECC$SHR", Offset: 0x0568, Code: 13},
	{Name: "DECC$GETS", Library: "DECC$SHR", Offset: 0x0370, Code: 14},
	{Name: "DECC$MALLOC", Library: "DECC$SHR", Offset: 0x0548, Code: 15},
	{Name: "DECC$FREE", Library: "DECC$SHR", Offset: 0x0538, Code: 16},
	{Name: "DECC$ISALNUM", Library: "DECC$SHR", Offset: 0x0018, Code: 17},
	{Name: "DECC$ISALPHA", Library: "DECC$SHR", Offset: 0x0020, Code: 18},
	{Name: "DECC$ISCNTRL", Library: "DECC$SHR", Offset: 0x0030, Code: 19},
	{Name: "DECC$ISDIGIT", Library: "DECC$SHR", Offset: 0x0038, Code: 20},
	{Name: "DECC$ISGRAPH", Library: "DECC$SHR", Offset: 0x0040, Code: 21},
	{Name: "DECC$ISLOWER", Library: "DECC$SHR", Offset: 0x0048, Code: 22},
	{Name: "DECC$ISPRINT", Library: "DECC$SHR", Offset: 0x0050, Code: 23},
	{Name: "DECC$ISPUNCT", Library: "DECC$SHR", Offset: 0x0058, Code: 24},
	{Name: "DECC$ISSPACE", Library: "DECC$SHR", Offset: 0x0060, Code: 25},
	{Name: "DECC$ISUPPER", Library: "DECC$SHR", Offset: 0x0068, Code: 26},
	{Name: "DECC$ISXDIGIT", Library: "DECC$SHR", Offset: 0x0070, Code: 27},
	{Name: "DECC$ISASCII", Library: "DECC$SHR", Offset: 0x0028, Code: 28},
	{Name: "DECC$TIME", Library: "DECC$SHR", Offset: 0x0768, Code: 32},

	// The traceback handler, which RUN's driver puts in its frame
	// (traceback.go); no program links to it.
	{Name: "EXE$TRACEBACK", Library: "EVAX", Offset: 0x0008, Code: tracebackShimCode},
}

// ShimStubSize is the length in bytes of one stub (ShimStub).
const ShimStubSize = 12

// ShimStub returns the stub that calls shim code: a procedure a program
// reaches with CALLS or CALLG, whose 2-byte entry mask saves no
// registers, then MOVL I^#code,R0 (6 bytes), XFC #XFC$SHIM (2 bytes),
// which runs the shim with the caller's argument list and leaves its
// result in R0, and RET.
func ShimStub(code uint32) []byte {
	return []byte{
		0x00, 0x00, // entry mask: no registers saved
		0xD0, 0x8F, // MOVL I^#...
		byte(code), byte(code >> 8), byte(code >> 16), byte(code >> 24),
		0x50, // R0
		0xFC, // XFC
		0x7D, // #XFC$SHIM
		0x04, // RET
	}
}

// JSBShimStub returns the stub of a routine reached by JSB (shim code
// code, registered with RegisterJSB). Its arguments are in R0, R1, ...,
// so the code can't go in R0 as ShimStub's does: the stub pushes it,
// PUSHL I^#code (6 bytes), for XFC #XFC$JSBSHIM (2 bytes) to pop, then
// returns with RSB. It has no entry mask: JSB jumps straight to its
// first instruction. NOPs pad it to ShimStubSize, so every stub is the
// same size.
func JSBShimStub(code uint32) []byte {
	return []byte{
		0xDD, 0x8F, // PUSHL I^#...
		byte(code), byte(code >> 8), byte(code >> 16), byte(code >> 24),
		0xFC,             // XFC
		0x05,             // #XFC$JSBSHIM
		0x05,             // RSB
		0x01, 0x01, 0x01, // NOP, never reached
	}
}
