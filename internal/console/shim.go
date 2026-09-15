package console

import "fmt"

// shimEntry is one row of kernel.asm's `.shim <name>, <code>, <library>,
// <offset>` pseudo-op table (see docs/PHASE-13.md's "Key finding" section):
// code selects the numeric XFC$SHIM dispatch (internal/rtl.ShimTable,
// already fully implemented by Phase 10) a sharable image's real routine at
// (library, offset) is replaced by; code 0 means kernel.asm's own table has
// no working numeric dispatch for that entry either (a "dead" stub -- see
// asm_pseudo.c's own `.SHIM` handling, case 33: a zero code looks up an
// already-defined symbol instead of synthesizing a dispatch stub, and none
// of these particular entries are ever defined elsewhere in kernel.asm).
//
// Transcribed directly from kernel.asm's own two `.shim` tables (lines
// ~1146-1177 and ~1546-1555); offsets are hex (the assembler's default
// radix, `vax.console.radix == 16`, applies to every bare numeral here).
type shimEntry struct {
	library string
	offset  uint32
	code    uint32
}

var shimTable = []shimEntry{
	{"LIBRTL", 0x0778, 1},      // lib$adawi
	{"LIBRTL", 0x0A70, 2},      // str$upcase
	{"EVAX", 0x0004, 3},        // exe$input
	{"DECC$SHR", 0x04E8, 4},    // decc$open
	{"DECC$SHR", 0x0490, 5},    // decc$close
	{"DECC$SHR", 0x04F0, 6},    // decc$read
	{"DECC$SHR", 0x0508, 7},    // decc$write
	{"DECC$SHR", 0x0380, 8},    // decc$printf
	{"DECC$SHR", 0x03E0, 9},    // decc$sprintf
	{"DECC$SHR", 0x06C0, 10},   // decc$strcmp
	{"DECC$SHR", 0x06F8, 11},   // decc$strncmp
	{"DECC$SHR", 0x0700, 12},   // decc$strncpy
	{"DECC$SHR", 0x0568, 13},   // decc$atoi
	{"DECC$SHR", 0x0370, 14},   // decc$gets
	{"DECC$SHR", 0x0548, 15},   // decc$malloc
	{"DECC$SHR", 0x0538, 16},   // decc$free
	{"DECC$SHR", 0x0018, 17},   // decc$isalnum
	{"DECC$SHR", 0x0020, 18},   // decc$isalpha
	{"DECC$SHR", 0x0030, 19},   // decc$iscntrl
	{"DECC$SHR", 0x0038, 20},   // decc$isdigit
	{"DECC$SHR", 0x0040, 21},   // decc$isgraph
	{"DECC$SHR", 0x0048, 22},   // decc$islower
	{"DECC$SHR", 0x0050, 23},   // decc$isprint
	{"DECC$SHR", 0x0058, 24},   // decc$ispunct
	{"DECC$SHR", 0x0060, 25},   // decc$isspace
	{"DECC$SHR", 0x0068, 26},   // decc$isupper
	{"DECC$SHR", 0x0070, 27},   // decc$isxdigit
	{"DECC$SHR", 0x0028, 28},   // decc$isascii
	{"LIBRTL", 0x0550, 29},     // lib$get_vm
	{"LIBRTL", 0x0548, 30},     // lib$free_vm
	{"LIBRTL", 0x0A48, 31},     // lib$delete_vm_zone
	{"DECC$SHR", 0x0768, 32},   // decc$time
	{"LIBRTL", 0x0478, 0},      // lib$put_output (dead: no numeric dispatch)
	{"DECC$SHR", 0x0000, 0},    // decc$main
	{"DECC$SHR", 0x0528, 0},    // decc$exit
	{"DECC$SHR", 0x06E8, 0},    // decc$strlen
	{"DECC$SHR", 0x06D0, 0},    // decc$strcpy
	{"DECC$SHR", 0x06B0, 0},    // decc$strcat
	{"CMA$TIS_SHR", 0x0038, 0}, // cma$tis_errno_get_addr
	{"DECC$SHR", 0x0520, 0},    // decc$calloc
	{"DECC$SHR", 0x3098, 0},    // decc$$gl___ctypea
	{"DECC$SHR", 0x309C, 0},    // decc$$ga___ctypet
}

// shimStubSize is the length in bytes of one synthesized stub: a 2-byte
// (empty) entry mask, MOVL #code,R0 (6 bytes: opcode, immediate-mode byte,
// 4-byte code), XFC #0x7D (2 bytes: opcode, immediate operand byte), RET
// (1 byte) -- matching asm_pseudo.c's own `.SHIM` code-generation case 33
// byte-for-byte.
const shimStubSize = 12

// ensureShims synthesizes every shimTable entry's stub into c.shimBase (a
// dedicated S0 page reserved by VMInit) and defines its SHIM$<library>_
// <offset> symbol, matching what kernel.asm's boot-time assembly of its
// own `.shim` pseudo-ops would produce -- done directly as bytes rather
// than through a real assembler, per docs/PHASE-13.md's "Key finding".
// Idempotent: a second call is a no-op, so fixups performed across
// multiple RUNs keep resolving to the same stub addresses.
func (c *Console) ensureShims() error {
	if c.shimsReady {
		return nil
	}

	addr := c.shimBase

	for _, e := range shimTable {
		stub := []byte{
			0x00, 0x00, // entry mask: no registers saved
			0xD0, 0x8F, // MOVL I^#...
			byte(e.code), byte(e.code >> 8), byte(e.code >> 16), byte(e.code >> 24),
			0x50, // R0
			0xFC, // XFC
			0x7D, // #XFC$SHIM
			0x04, // RET
		}

		if err := c.storeBytes(addr, stub); err != nil {
			return err
		}

		name := fmt.Sprintf("SHIM$%s_%08X", e.library, e.offset)
		c.Symbols.Set(name, addr, SymbolSystem)

		addr += shimStubSize
	}

	c.shimsReady = true
	
	return nil
}
