package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestRelativeDisplacementSizes checks the MACRO manual's rule (§5.2.1,
// §5.2.2): a target already defined in the same psect gets the smallest
// displacement, finished here; anything else gets the default
// displacement, finished by the linker, even a label defined later in
// the same psect.
func TestRelativeDisplacementSizes(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
BACK:	CLRL BACK
	CLRL @BACK
	CLRL AHEAD
AHEAD:	CLRL W^BACK
	CLRL B^LAST
LAST:	NOP`)

	requireBytes(t, psectBytes(t, a, "CODE"),
		0xD4, 0xAF, 0xFD, // CLRL BACK: a byte, from 3 back to 0
		0xD4, 0xBF, 0xFA, // CLRL @BACK: a byte, deferred
		0xD4, 0xEF, 0, 0, 0, 0, // CLRL AHEAD: the default, a longword
		0xD4, 0xCF, 0xF0, 0xFF, // CLRL W^BACK: as written, finished here
		0xD4, 0xAF, 0, // CLRL B^LAST: as written, for the linker
		0x01)

	requireRelocations(t, a, "CODE+8 LD CODE:C", "CODE+12 BD CODE:13")
}

// TestDefaultDisplacementApplies checks that .DEFAULT DISPLACEMENT
// changes the size of relative operands whose target isn't known, and
// only those.
func TestDefaultDisplacementApplies(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
HERE:	.DEFAULT DISPLACEMENT, BYTE
	CLRL EXT
	CLRL HERE
	.DEFAULT DISPLACEMENT, WORD
	CLRL @EXT
	CLRL 4(R1)`)

	requireBytes(t, psectBytes(t, a, "CODE"),
		0xD4, 0xAF, 0,
		0xD4, 0xAF, 0xFA,
		0xD4, 0xDF, 0, 0,
		0xD4, 0xA1, 4)
	requireRelocations(t, a, "CODE+2 BD EXT", "CODE+8 WD EXT")
}

// TestDisplacementModeUnknown checks displacement mode from a register
// (§5.1.6): an unknown displacement (relocatable, external, or defined
// later) gets a word, which the linker stores signed (STO_SW), and a
// known one the smallest size.
func TestDisplacementModeUnknown(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
ITEM:	.LONG 0
	.PSECT CODE
	TSTB ITEM(R3)
	TSTB EXT(R3)
	TSTB @LATER(R3)
	TSTB 300(R3)
LATER = 8`)

	requireBytes(t, psectBytes(t, a, "CODE"),
		0x95, 0xC3, 0, 0,
		0x95, 0xC3, 0, 0,
		0x95, 0xD3, 8, 0,
		0x95, 0xC3, 0x2C, 0x01)
	requireRelocations(t, a, "CODE+2 SW DATA:0", "CODE+6 SW EXT")
}

// TestRelativeToAbsoluteAddress checks that a relative operand or a branch
// from a relocatable psect to an absolute address is left to the linker:
// the distance depends on where the psect goes.
func TestRelativeToAbsoluteAddress(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
	CLRL ^X200
	BRW ^X200`)

	requireBytes(t, psectBytes(t, a, "CODE"), 0xD4, 0xEF, 0, 0, 0, 0, 0x31, 0, 0)
	requireRelocations(t, a, "CODE+2 LD 512", "CODE+7 WD 512")
}

// TestGeneralMode checks G^: five bytes the linker writes (STO_PICR),
// whatever the address, even one known to be absolute, as real MACRO
// leaves it.
func TestGeneralMode(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
	CALLS #0, G^SUB
	CLRL G^ABSOLUTE
	CLRL G^LATER
	CLRL G^HERE[R2]
HERE:	NOP
ABSOLUTE = ^X1000
LATER = ^X2000`)

	requireBytes(t, psectBytes(t, a, "CODE"),
		0xFB, 0x00, 0, 0, 0, 0, 0,
		0xD4, 0, 0, 0, 0, 0,
		0xD4, 0, 0, 0, 0, 0,
		0xD4, 0x42, 0, 0, 0, 0, 0,
		0x01)
	requireRelocations(t, a, "CODE+2 PICR SUB", "CODE+8 PICR 4096", "CODE+E PICR 8192", "CODE+15 PICR CODE:1A")

	requireMACROError(t, ".PSECT CODE\nCLRL @G^SUB", vmserrors.VAX_BADMODE)
}

// TestEnableAbsolute checks .ENABLE ABSOLUTE: relative operands are
// assembled as absolute mode, and relative deferred ones are unchanged.
func TestEnableAbsolute(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
HERE:	.ENABLE ABSOLUTE
	CLRL HERE
	CLRL ^X300
	CLRL @HERE
	.DISABLE ABSOLUTE
	CLRL HERE`)

	requireBytes(t, psectBytes(t, a, "CODE"),
		0xD4, 0x9F, 0, 0, 0, 0,
		0xD4, 0x9F, 0x00, 0x03, 0, 0,
		0xD4, 0xBF, 0xF1,
		0xD4, 0xAF, 0xEE)
	requireRelocations(t, a, "CODE+2 L CODE:0")
}

// TestConsoleOperandEncoding checks that the console dialect keeps its own
// sizes: a known absolute address gets the smallest displacement, a
// forward reference a longword, and G^ is absolute mode.
func TestConsoleOperandEncoding(t *testing.T) {
	requireBytes(t, assembleBytes(t, `HERE:	CLRL HERE
	CLRL AHEAD
	CLRL G^HERE
	CLRL AHEAD(R1)
AHEAD:	NOP`),
		0xD4, 0xAF, 0xFD,
		0xD4, 0xEF, 0x0C, 0, 0, 0,
		0xD4, 0x9F, 0x00, 0x02, 0, 0,
		0xD4, 0xE1, 0x15, 0x02, 0, 0,
		0x01)
}
