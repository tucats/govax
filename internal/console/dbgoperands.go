package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/vax"
)

// OperandsMode is how much of an instruction's operands EXAMINE/INSTRUCTION
// explains: SET MODE OPERANDS and EXAMINE/OPERANDS in the VMS debugger.
type OperandsMode int

const (
	// OperandsOff shows the instruction alone.
	OperandsOff OperandsMode = iota

	// OperandsBrief adds a line or two for each operand that names a
	// register or a memory location: what it is, and what it holds now.
	OperandsBrief

	// OperandsFull is /OPERANDS=FULL. The probe's logs show the same
	// text for it as for the brief form, so govax makes them the same
	// (unconfirmed: no probe line has an operand the two would show
	// differently).
	OperandsFull
)

// operandIndent is how far the debugger indents an operand's line, and
// operandTextWidth the width it gives the operand's own text before the
// description. A longer operand takes a line to itself and its description
// starts on the next, under where the description would have started.
const (
	operandIndent    = 5
	operandTextWidth = 10
)

// printOperands writes the explanation of each of dec's operands that
// refers to a register or a memory location, in the layout the VMS
// debugger gives EXAMINE/OPERANDS (testdata/dbg/vax/dbgdis.dlg):
//
//	R0         R0 contains 00000003
//	L^DBGDIS\COUNT
//	           DBGDIS\COUNT (address 00000200) contains 00000000
//
// dec is the instruction as decoded at its own address, with the machine
// as it is *before* the instruction runs: an autoincrement operand's
// address is the register's present value, and so on. An operand that is
// a constant (a short literal, an immediate), a branch destination, or an
// instruction's inline data refers to neither, and has no line
// (unconfirmed: no probe line has one; govax's choice).
func (c *Console) printOperands(dec disasm.Decoded, format disasm.Options, names consoleSymbolizer) {
	if dec.IsMask {
		return
	}

	for _, op := range dec.Symbolized(format).Operands {
		desc, ok := c.describeOperand(op, names)
		if !ok {
			continue
		}

		text := op.DebuggerText()

		if len(text) <= operandTextWidth {
			c.Printf("%*s%-*s %s\n", operandIndent, "", operandTextWidth, text, desc)

			continue
		}

		c.Printf("%*s%s\n", operandIndent, "", text)
		c.Printf("%*s%s\n", operandIndent+operandTextWidth+1, "", desc)
	}
}

// describeOperand says what an operand refers to and what is there:
// "R0 contains 00000003", or "DBGDIS\COUNT (address 00000200) contains
// 00000000" (just the address when nothing names it). ok is false for an
// operand that refers to neither a register nor a memory location.
func (c *Console) describeOperand(op disasm.Operand, names consoleSymbolizer) (string, bool) {
	if op.Mode == disasm.ModeRegister && !op.Indexed() {
		name := cpu.RegisterName(op.Register)
		value := uint64(c.CPU.GPR(vax.Reg(op.Register)))

		// A quadword (or D or G floating) operand is a pair of registers.
		if op.Size == 8 && op.Register < int(vax.R15) {
			value |= uint64(c.CPU.GPR(vax.Reg(op.Register+1))) << 32
		}

		return fmt.Sprintf("%s contains %s", name, hexValue(value, op.Size)), true
	}

	addr, ok := c.operandAddress(op)
	if !ok {
		return "", false
	}

	where := fmt.Sprintf("%08X", addr)
	if name, named := names.Symbolize(addr); named {
		where = fmt.Sprintf("%s (address %08X)", name, addr)
	}

	return where + " contains " + c.memoryValue(addr, op.Size), true
}

// operandAddress computes the memory address an operand refers to from the
// registers as they stand, without changing them (an autoincrement operand
// refers to where the register points *now*; the instruction will advance
// it afterward). ok is false for the modes that have no address.
func (c *Console) operandAddress(op disasm.Operand) (uint32, bool) {
	var (
		addr uint32
		ok   = true
		reg  uint32
	)

	if op.Register >= 0 {
		reg = c.CPU.GPR(vax.Reg(op.Register))
	}

	switch op.Mode {
	case disasm.ModeRegisterDeferred, disasm.ModeAutoincrement:
		addr = reg

	case disasm.ModeAutodecrement:
		// The register is decremented first, then holds the address.
		addr = reg - uint32(op.Size)

	case disasm.ModeAutoincrementDeferred:
		addr, ok = c.loadAddress(reg)

	case disasm.ModeDisplacement:
		addr = reg + uint32(op.Displacement)
		if op.Deferred {
			addr, ok = c.loadAddress(addr)
		}

	case disasm.ModeAbsolute:
		addr = op.Target

	case disasm.ModeRelative:
		addr = op.Target
		if op.Deferred {
			addr, ok = c.loadAddress(addr)
		}

	default:
		// Registers (handled by the caller), literals, immediates, branch
		// displacements, and inline data are not memory operands.
		return 0, false
	}

	if !ok {
		return 0, false
	}

	// An index register scales by the operand's size and is added last.
	if op.Indexed() {
		addr += c.CPU.GPR(vax.Reg(op.Index)) * uint32(max(op.Size, 1))
	}

	return addr, true
}

// loadAddress reads the longword at addr, for a deferred operand's pointer.
func (c *Console) loadAddress(addr uint32) (uint32, bool) {
	v, err := c.loadSized(addr, SizeLongword)

	return v, err == nil
}

// memoryValue reads size bytes at addr, as the hexadecimal digits the
// debugger shows. A location that can't be read (unmapped, or protected)
// shows as "<inaccessible>" (govax's wording; unconfirmed).
func (c *Console) memoryValue(addr uint32, size int) string {
	var value uint64

	for i := 0; i < size && i < 8; i++ {
		b, err := c.loadSized(addr+uint32(i), SizeByte)
		if err != nil {
			return "<inaccessible>"
		}

		value |= uint64(b) << (8 * uint(i))
	}

	return hexValue(value, size)
}

// hexValue formats the low size bytes of v as two hexadecimal digits per
// byte. A size of 0 or over 8 (an octaword, a packed string) shows a
// longword: those values are too wide for the one-line form.
func hexValue(v uint64, size int) string {
	if size < 1 || size > 8 {
		size = 4
	}

	return strings.ToUpper(fmt.Sprintf("%0*x", 2*size, v&(^uint64(0)>>(64-8*uint(size)))))
}
