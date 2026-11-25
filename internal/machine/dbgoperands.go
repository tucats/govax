package machine

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/registers"
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

	// OperandsFull is /OPERANDS=FULL: as brief, but a constant gets a
	// line, and an operand whose address is computed says how, as VMS
	// 7.1's did (testdata/dbg113's DBG113I): "R2 contains ...  (00000200)+
	// evaluates to ..., which contains ...".
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
// address is the register's present value, and so on. A branch
// destination or an instruction's inline data refers to neither, and has
// no line; a constant (a short literal, an immediate) has one only under
// OperandsFull (VMS 7.1's DBG113I).
func (m *Machine) printOperands(dec disasm.Decoded, format disasm.Options, names consoleSymbolizer, mode OperandsMode) {
	if dec.IsMask {
		return
	}

	for _, op := range dec.Symbolized(format).Operands {
		desc, ok := "", false
		if mode == OperandsFull {
			desc, ok = m.explainOperand(op, names)
		}

		if !ok {
			desc, ok = m.describeOperand(op, names)
		}

		if !ok {
			continue
		}

		text := op.DebuggerTextIn(format.Decimal)

		if len(text) <= operandTextWidth {
			m.Printf("%*s%-*s %s\n", operandIndent, "", operandTextWidth, text, desc)

			continue
		}

		m.Printf("%*s%s\n", operandIndent, "", text)
		m.Printf("%*s%s\n", operandIndent+operandTextWidth+1, "", desc)
	}
}

// describeOperand says what an operand refers to and what is there:
// "R0 contains 00000003", or "DBGDIS\COUNT (address 00000200) contains
// 00000000" (just the address when nothing names it). ok is false for an
// operand that refers to neither a register nor a memory location.
func (m *Machine) describeOperand(op disasm.Operand, names consoleSymbolizer) (string, bool) {
	if op.Mode == disasm.ModeRegister && !op.Indexed() {
		name := cpu.RegisterName(op.Register)
		value := uint64(m.CPU.GPR(registers.Reg(op.Register)))

		// A quadword (or D or G floating) operand is a pair of registers.
		if op.Size == 8 && op.Register < int(registers.R15) {
			value |= uint64(m.CPU.GPR(registers.Reg(op.Register+1))) << 32
		}

		return fmt.Sprintf("%s contains %s", name, hexValue(value, op.Size)), true
	}

	addr, ok := m.operandAddress(op)
	if !ok {
		return "", false
	}

	return where(addr, names) + " contains " + m.memoryValue(addr, op.Size), true
}

// where names an address as the operand lines do: "DBGDIS\COUNT (address
// 00000200)", or just the address when nothing names it.
func where(addr uint32, names consoleSymbolizer) string {
	if name, named := names.Symbolize(addr); named {
		return fmt.Sprintf("%s (address %08X)", name, addr)
	}

	return fmt.Sprintf("%08X", addr)
}

// explainOperand is OperandsFull's line for the operands it says more of
// than describeOperand: a constant, a displacement, an autoincrement or
// autodecrement, and an absolute or relative base with an index, in VMS
// 7.1's words (testdata/dbg113's DBG113I). ok is false for the rest, which
// read as in the brief form. A non-deferred displacement and an immediate
// are written by the same pattern as the probe's (unconfirmed).
func (m *Machine) explainOperand(op disasm.Operand, names consoleSymbolizer) (string, bool) {
	switch {
	case op.Mode == disasm.ModeLiteral && !op.Indexed():
		return fmt.Sprintf("%02X (constant)", op.Value), true

	case op.Mode == disasm.ModeImmediate && !op.Indexed():
		return hexValue(uint64(op.Value), min(op.Size, 4)) + " (constant)", true

	case op.Indexed() && (op.Mode == disasm.ModeAbsolute || op.Mode == disasm.ModeRelative) && !op.Deferred:
		base := op.Target
		index := m.CPU.GPR(registers.Reg(op.Index))
		size := uint32(max(op.Size, 1))
		addr := base + index*size

		return fmt.Sprintf("%s (constant).  %s contains %08X.  %08X[%08X * %d] evaluates to %s, which contains %s",
			where(base, names), cpu.RegisterName(op.Index), index, base, index, size,
			where(addr, names), m.memoryValue(addr, op.Size)), true

	case op.Indexed() || op.Register < 0:
		return "", false

	case op.Mode == disasm.ModeAutoincrement, op.Mode == disasm.ModeAutodecrement:
		reg := m.CPU.GPR(registers.Reg(op.Register))
		addr, form := reg, fmt.Sprintf("(%08X)+", reg)

		if op.Mode == disasm.ModeAutodecrement {
			addr, form = reg-uint32(op.Size), fmt.Sprintf("-(%08X)", reg)
		}

		return fmt.Sprintf("%s contains %s.  %s  evaluates to %s, which contains %s",
			cpu.RegisterName(op.Register), where(reg, names), form, where(addr, names), m.memoryValue(addr, op.Size)), true

	case op.Mode == disasm.ModeDisplacement:
		reg := m.CPU.GPR(registers.Reg(op.Register))
		name := cpu.RegisterName(op.Register)
		text := strings.Replace(op.DebuggerText(), "("+name+")", fmt.Sprintf("(%08X)", reg), 1)
		at := reg + uint32(op.Displacement)

		if !op.Deferred {
			return fmt.Sprintf("%s contains %s, %s evaluates to %s, which contains %s",
				name, where(reg, names), text, where(at, names), m.memoryValue(at, op.Size)), true
		}

		pointer, ok := m.loadAddress(at)
		if !ok {
			return fmt.Sprintf("%s contains %s, %s evaluates to @%08X, which contains (no read access)",
				name, where(reg, names), text, at), true
		}

		return fmt.Sprintf("%s contains %s, %s evaluates to @%08X, which contains %s, which contains %s",
			name, where(reg, names), text, at, where(pointer, names), m.memoryValue(pointer, op.Size)), true
	}

	return "", false
}

// operandAddress computes the memory address an operand refers to from the
// registers as they stand, without changing them (an autoincrement operand
// refers to where the register points *now*; the instruction will advance
// it afterward). ok is false for the modes that have no address.
func (m *Machine) operandAddress(op disasm.Operand) (uint32, bool) {
	var (
		addr uint32
		ok   = true
		reg  uint32
	)

	if op.Register >= 0 {
		reg = m.CPU.GPR(registers.Reg(op.Register))
	}

	switch op.Mode {
	case disasm.ModeRegisterDeferred, disasm.ModeAutoincrement:
		addr = reg

	case disasm.ModeAutodecrement:
		// The register is decremented first, then holds the address.
		addr = reg - uint32(op.Size)

	case disasm.ModeAutoincrementDeferred:
		addr, ok = m.loadAddress(reg)

	case disasm.ModeDisplacement:
		addr = reg + uint32(op.Displacement)
		if op.Deferred {
			addr, ok = m.loadAddress(addr)
		}

	case disasm.ModeAbsolute:
		addr = op.Target

	case disasm.ModeRelative:
		addr = op.Target
		if op.Deferred {
			addr, ok = m.loadAddress(addr)
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
		addr += m.CPU.GPR(registers.Reg(op.Index)) * uint32(max(op.Size, 1))
	}

	return addr, true
}

// loadAddress reads the longword at addr, for a deferred operand's pointer.
func (m *Machine) loadAddress(addr uint32) (uint32, bool) {
	v, err := m.loadSized(addr, SizeLongword)

	return v, err == nil
}

// memoryValue reads size bytes at addr, as the hexadecimal digits the
// debugger shows. A location that can't be read (unmapped, or protected)
// shows as "(no read access)", VMS 7.1's words (testdata/dbg113's DBG113I).
func (m *Machine) memoryValue(addr uint32, size int) string {
	var value uint64

	for i := 0; i < size && i < 8; i++ {
		b, err := m.loadSized(addr+uint32(i), SizeByte)
		if err != nil {
			return "(no read access)"
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
