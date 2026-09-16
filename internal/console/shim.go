package console

import (
	"fmt"

	"github.com/tucats/govax/internal/vmserrors"
)

// shimEntry is one row of kernel.asm's `.shim <name>, <code>, <library>,
// <offset>` pseudo-op table. code selects the numeric XFC$SHIM dispatch
// (internal/rtl.ShimTable, already fully implemented by Phase 10) a
// sharable image's real routine at (library, offset) is replaced by --
// *except* when code is 0. asm_pseudo.c's own `.SHIM` handling (case 33)
// branches on exactly this: a nonzero code synthesizes a small dispatch
// stub (MOVL #code,R0 / XFC #0x7D / RET) and points SHIM$<library>_<offset>
// at it; code 0 means there's no numeric dispatch at all for this routine
// -- instead the pseudo-op does a plain `get_symbol(name)` and points
// SHIM$<library>_<offset> directly at *that* symbol's existing address.
//
// This matters because kernel.asm's own second `.shim` table (lines
// ~1546-1555) exclusively uses code 0 for routines it implements as real,
// already-written native VAX code living elsewhere in the same file (e.g.
// `decc$main:`, a plain label reached via JSB per that file's own comment
// "the MAIN entry initialization is via JSB not CALL" -- notably *not* a
// CALLS-style `.ENTRY`, so it carries no register-save entry mask word).
// An earlier version of this port treated every code-0 entry as simply
// "dead" and synthesized the same MOVL/XFC/RET stub for it as a nonzero-code
// entry would get -- including that stub's own leading 2-byte entry-mask
// placeholder. A G^ fixup resolving to one of those synthesized stubs and
// then reached via JSB (matching kernel.asm's own calling convention for
// these routines, not a CALLS/CALLG that would skip the mask word) executed
// that placeholder directly as an instruction: opcode 0x00 is HALT, and
// HALT outside kernel mode is a privileged-instruction fault -- exactly the
// "JSB to a HALT instruction" symptom this fix addresses. name is the
// routine's own label as kernel.asm spells it (case-insensitive; symbol
// lookup upcases), needed for both branches (code!=0 also defines this bare
// name as a second symbol pointing at its own synthesized stub, matching
// asm_pseudo.c's own `set_symbol(&bp, vax.console.deposit, SYM_ENTRY)` --
// SHOW SYMBOL's existing "(system, entry)" outputs for e.g. DECC$EXIT are
// this project's kernel.asm doing exactly that already for the code==0
// routines; ensureShims now does the equivalent for the code!=0 ones).
//
// Transcribed directly from kernel.asm's own two `.shim` tables (lines
// ~1146-1177 and ~1546-1555); offsets are hex (the assembler's default
// radix, `vax.console.radix == 16`, applies to every bare numeral here).
type shimEntry struct {
	name    string
	library string
	offset  uint32
	code    uint32
}

var shimTable = []shimEntry{
	{"LIB$ADAWI", "LIBRTL", 0x0778, 1},
	{"STR$UPCASE", "LIBRTL", 0x0A70, 2},
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
	{"LIB$GET_VM", "LIBRTL", 0x0550, 29},
	{"LIB$FREE_VM", "LIBRTL", 0x0548, 30},
	{"LIB$DELETE_VM_ZONE", "LIBRTL", 0x0A48, 31},
	{"DECC$TIME", "DECC$SHR", 0x0768, 32},

	// code 0: resolved by symbol lookup against kernel.asm's own
	// already-assembled native routines, not stub synthesis -- see this
	// file's own top comment.
	{"LIB$PUT_OUTPUT", "LIBRTL", 0x0478, 0},
	{"DECC$MAIN", "DECC$SHR", 0x0000, 0},
	{"DECC$EXIT", "DECC$SHR", 0x0528, 0},
	{"DECC$STRLEN", "DECC$SHR", 0x06E8, 0},
	{"DECC$STRCPY", "DECC$SHR", 0x06D0, 0},
	{"DECC$STRCAT", "DECC$SHR", 0x06B0, 0},
	{"CMA$TIS_ERRNO_GET_ADDR", "CMA$TIS_SHR", 0x0038, 0},
	{"DECC$CALLOC", "DECC$SHR", 0x0520, 0},
	{"DECC$$GL___CTYPEA", "DECC$SHR", 0x3098, 0},
	{"DECC$$GA___CTYPET", "DECC$SHR", 0x309C, 0},
}

// shimStubSize is the length in bytes of one synthesized stub: a 2-byte
// (empty) entry mask, MOVL #code,R0 (6 bytes: opcode, immediate-mode byte,
// 4-byte code), XFC #0x7D (2 bytes: opcode, immediate operand byte), RET
// (1 byte) -- matching asm_pseudo.c's own `.SHIM` code-generation case 33
// byte-for-byte.
const shimStubSize = 12

// ensureShims synthesizes a dispatch stub for every shimTable entry that
// has a real numeric XFC$SHIM code, and resolves every code-0 entry against
// its already-assembled kernel.asm routine by name instead (see shimEntry's
// own doc comment) -- either way defining that entry's SHIM$<library>_
// <offset> symbol, matching what kernel.asm's boot-time assembly of its own
// `.shim` pseudo-ops actually produces. Must run after kernel.asm has been
// assembled (Console.Run's own call site does, via vax.init's boot
// sequence), since the code-0 branch depends on every routine name it
// resolves already being a defined symbol. Idempotent: a second call is a
// no-op, so fixups performed across multiple RUNs keep resolving to the
// same addresses.
func (c *Console) ensureShims() error {
	if c.shimsReady {
		return nil
	}

	addr := c.shimBase

	for _, e := range shimTable {
		shimName := fmt.Sprintf("SHIM$%s_%08X", e.library, e.offset)

		if e.code == 0 {
			target, ok := c.Symbols.Get(e.name)
			if !ok {
				return vmserrors.New(vmserrors.LIB_UNRESOLVED, e.name)
			}

			c.Symbols.Set(shimName, target, SymbolSystem)

			continue
		}

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

		c.Symbols.Set(shimName, addr, SymbolSystem)
		c.Symbols.SetEntry(e.name, addr, SymbolSystem)

		addr += shimStubSize
	}

	c.shimsReady = true

	return nil
}
