package disasm

import "github.com/tucats/govax/internal/cpu"

// Mode is an operand's addressing mode, as Disassemble decoded it. The
// VAX has more modes than the four bits of an operand specifier byte
// suggest, because register PC (R15) turns the general modes into the
// PC-relative ones (immediate, absolute, relative); Mode names what the
// operand means rather than the raw nibble.
type Mode int

const (
	// ModeLiteral is a short literal, S^#n: the specifier byte itself
	// holds a value from 0 to 63 (or, for a floating operand, one of 64
	// floating values).
	ModeLiteral Mode = iota
	// ModeRegister is Rn: the operand is the register.
	ModeRegister
	// ModeRegisterDeferred is (Rn): the register holds the operand's
	// address.
	ModeRegisterDeferred
	// ModeAutodecrement is -(Rn): the register is decremented by the
	// operand's size, then holds its address.
	ModeAutodecrement
	// ModeAutoincrement is (Rn)+: the register holds the address, and is
	// then incremented by the operand's size.
	ModeAutoincrement
	// ModeAutoincrementDeferred is @(Rn)+: the register holds the address
	// of the operand's address, and is then incremented by 4.
	ModeAutoincrementDeferred
	// ModeDisplacement is B^d(Rn), W^d(Rn), or L^d(Rn), and with Deferred,
	// @B^d(Rn) and so on: the register plus a displacement.
	ModeDisplacement
	// ModeImmediate is I^#value: autoincrement on PC, so the value follows
	// the specifier in the instruction stream.
	ModeImmediate
	// ModeAbsolute is @#address: autoincrement deferred on PC, so the
	// operand's address follows the specifier.
	ModeAbsolute
	// ModeRelative is B^address, W^address, or L^address, and with
	// Deferred, @B^address and so on: displacement mode on PC, so the
	// address is the displacement plus the address of the next byte.
	ModeRelative
	// ModeBranch is a branch instruction's displacement (BRB, BEQL,
	// SOBGTR, ...): no specifier byte, just a byte or word displacement
	// from the end of the instruction.
	ModeBranch
	// ModeInline is an operand with no specifier byte that isn't a
	// branch: the data that follows XFC, BUGL, and BUGW.
	ModeInline
)

// Operand is one decoded operand: its addressing mode and the parts of
// its specifier, kept apart so a caller can name an address, show a value
// in another radix, or lay the operand out its own way. Its String method
// gives the text internal/asm can reassemble.
type Operand struct {
	Mode Mode

	// Deferred marks the deferred form of ModeDisplacement and
	// ModeRelative (the @ prefix): the computed address holds the
	// operand's address.
	Deferred bool

	// Register is the general register the mode uses (0 to 15), or -1
	// for a mode with none (literal, immediate, absolute, relative,
	// branch, inline).
	Register int

	// Index is the index register of an indexed operand (base[Rx]), or
	// -1. The other fields describe the base operand.
	Index int

	// Width is the size in bytes of a displacement (ModeDisplacement,
	// ModeRelative, ModeBranch: 1, 2, or 4), or of the value
	// (ModeImmediate, ModeInline: the operand's size).
	Width int

	// Displacement is a displacement mode's displacement, sign-extended.
	Displacement int32

	// Value is a short literal (0 to 63) or the low longword of an
	// immediate or inline value.
	Value uint32

	// Bytes holds an immediate or inline value's bytes, low byte first,
	// all Width of them: for a quadword, an octaword, or a floating value,
	// which Value can't hold whole.
	Bytes []byte

	// Target is the address the operand refers to, when that is known
	// without running the program (HasTarget): a branch's destination, a
	// relative operand's address (for a deferred one, the address of the
	// pointer), and an absolute operand's address.
	Target    uint32
	HasTarget bool

	// Symbol, when set, is shown in place of Target's number: a name for
	// the address, which a caller fills in from its symbol table. On a
	// short literal or an integer immediate it's a constant's name, shown
	// in place of the value (Options.Constants).
	Symbol string

	// Cell, when set on a deferred relative operand, names what its
	// pointer leads to: the operand's Target is a G^ reference's fixup
	// cell, which the image activator filled with a shareable image's
	// address, and Cell is that routine's name (Options.Cells). The
	// operand is then shown as the source wrote it, G^LIB$PUT_OUTPUT, in
	// place of @L^cell.
	Cell string

	// Access, Type, and Size are what the instruction table says of this
	// operand: how the instruction uses it, its data type, and its size
	// in bytes.
	Access cpu.AccessKind
	Type   cpu.DataType
	Size   int
}

// Indexed reports whether the operand is indexed (base[Rx]).
func (op Operand) Indexed() bool {
	return op.Index >= 0
}
