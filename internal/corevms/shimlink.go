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
type ShimLink struct {
	Name    string
	Library string
	Offset  uint32
	Code    uint32
}

// CRTLLinks are the C library's routines (DECC$SHR) and govax's own
// (EVAX) that this package's shims stand in for, codes 3 to 32
// (registerShims; the name a routine is registered under there is its
// shim's, which may differ from the one a program links to, as
// DECC$OPEN's shim is EXE$OPEN). LIBRTL.EXE's are internal/librtl's
// (librtl.Links). Offsets are each routine's transfer-vector entry.
var CRTLLinks = []ShimLink{
	{"EXE$INPUT", "EVAX", 0x0004, 3},
	{"DECC$OPEN", "DECC$SHR", 0x04E8, 4},
	{"DECC$CLOSE", "DECC$SHR", 0x0490, 5},
	{"DECC$READ", "DECC$SHR", 0x04F0, 6},
	{"DECC$WRITE", "DECC$SHR", 0x0508, 7},
	{"DECC$PRINTF", "DECC$SHR", 0x0380, 8},
	{"DECC$SPRINTF", "DECC$SHR", 0x03E0, 9},
	{"DECC$STRCMP", "DECC$SHR", 0x06C0, 10},
	{"DECC$STRNCMP", "DECC$SHR", 0x06F8, 11},
	{"DECC$STRNCPY", "DECC$SHR", 0x0700, 12},
	{"DECC$ATOI", "DECC$SHR", 0x0568, 13},
	{"DECC$GETS", "DECC$SHR", 0x0370, 14},
	{"DECC$MALLOC", "DECC$SHR", 0x0548, 15},
	{"DECC$FREE", "DECC$SHR", 0x0538, 16},
	{"DECC$ISALNUM", "DECC$SHR", 0x0018, 17},
	{"DECC$ISALPHA", "DECC$SHR", 0x0020, 18},
	{"DECC$ISCNTRL", "DECC$SHR", 0x0030, 19},
	{"DECC$ISDIGIT", "DECC$SHR", 0x0038, 20},
	{"DECC$ISGRAPH", "DECC$SHR", 0x0040, 21},
	{"DECC$ISLOWER", "DECC$SHR", 0x0048, 22},
	{"DECC$ISPRINT", "DECC$SHR", 0x0050, 23},
	{"DECC$ISPUNCT", "DECC$SHR", 0x0058, 24},
	{"DECC$ISSPACE", "DECC$SHR", 0x0060, 25},
	{"DECC$ISUPPER", "DECC$SHR", 0x0068, 26},
	{"DECC$ISXDIGIT", "DECC$SHR", 0x0070, 27},
	{"DECC$ISASCII", "DECC$SHR", 0x0028, 28},
	{"DECC$TIME", "DECC$SHR", 0x0768, 32},
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
