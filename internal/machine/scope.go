package machine

import (
	"math/bits"
	"strings"

	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/registers"
)

// The call levels the debugger's SET SCOPE numbers (Phase 89, subtask 10),
// and the registers as each level saw them.
//
// Level 0 is the routine the program is in; level 1 the routine that
// called it, and so on out, the rows SHOW CALLS lists. Each level's
// routine built a call frame when it was called (CALLS or CALLG), and
// the frames are chained through their saved FPs:
//
//	FP+0   the condition handler's address
//	FP+4   the saved PSW in the low word; the entry mask's registers in
//	       bits 16 to 27 (R0 to R11); S, "called by CALLS", in bit 29;
//	       and in bits 30 and 31 how many bytes CALL dropped SP by to
//	       align it on a longword (SPA)
//	FP+8   the caller's AP
//	FP+12  the caller's FP
//	FP+16  the caller's PC, where it goes on after the RET
//	FP+20  the registers the entry mask named, lowest first
//
// So the registers of level k are in the frames of the levels inside it.
// Its PC, FP, and AP are what level k-1's frame saved. A register R2 to
// R11 is in the frame of the nearest level inside k whose entry mask
// saved it; if none did, no routine called since has changed it, and it
// is the live register. R0 and R1 hold results by the calling standard,
// and nobody saves them, so level k's are gone (testdata/dbg89:
// %DEBUG-W-SYMNOTACT for each). SP is what the RET of level k-1's frame
// would leave in it.

// CallLevel is one level of the call stack: the module and routine its
// PC is in, the PC, and the FP of the frame its routine built.
type CallLevel struct {
	Module, Routine string
	HasRoutine      bool
	PC, FP          uint32
}

// Path is the level's name as SHOW SCOPE writes it, without the
// invocation number: MODULE\ROUTINE, or the module alone when the
// routine has its module's name (a FORTRAN main program, DBG89B).
func (l CallLevel) Path() string {
	if l.HasRoutine && !strings.EqualFold(l.Routine, l.Module) {
		return l.Module + `\` + l.Routine
	}

	return l.Module
}

// CallLevels lists the call levels, innermost first. ok is false when the
// PC isn't in a module the debug symbols know (no image, or one without
// them).
func (m *Machine) CallLevels() (levels []CallLevel, ok bool, err error) {
	rows, ok, err := m.debugFrames(0)

	for _, row := range rows {
		levels = append(levels, CallLevel{
			Module: row.module, Routine: row.routine, HasRoutine: row.hasRoutine, PC: row.pc, FP: row.fp,
		})
	}

	return levels, ok, err
}

// LevelRegisters are the general registers and PSL as one call level saw
// them. Active says which of them the level still has; Slot is where in
// memory a register a frame saved is kept, 0 for a live one (which is
// where DEPOSIT puts a new value, so that the RET brings it back).
type LevelRegisters struct {
	Value  [16]uint32
	Active [16]bool
	Slot   [16]uint32
	PSL    uint32
}

// RegistersAt returns the registers of level k of levels (CallLevels'
// list). Level 0's are the live ones.
func (m *Machine) RegistersAt(levels []CallLevel, k int) (LevelRegisters, error) {
	var r LevelRegisters

	for n := range r.Value {
		r.Value[n] = m.CPU.GPR(registers.Reg(n))
		r.Active[n] = true
	}

	r.PSL = uint32(m.CPU.PSL())

	if k == 0 {
		return r, nil
	}

	r.Active[registers.R0], r.Active[registers.R1] = false, false
	r.Value[registers.R0], r.Value[registers.R1] = 0, 0

	// The frame level k-1 built holds level k's AP, FP, and PC.
	callee := levels[k-1].FP

	for _, saved := range []struct {
		reg    registers.Reg
		offset uint32
	}{{registers.AP, 8}, {registers.FP, 12}, {registers.PC, 16}} {
		v, err := m.Mem.LoadLongword(m.CPU, callee+saved.offset)
		if err != nil {
			return r, err
		}

		r.Value[saved.reg], r.Slot[saved.reg] = v, callee+saved.offset
	}

	maskWord, err := m.Mem.LoadLongword(m.CPU, callee+4)
	if err != nil {
		return r, err
	}

	// The PSW the CALL saved has its condition codes cleared, as the RET
	// leaves them (whether the debugger shows these or the live PSW is
	// unconfirmed: the probe's PSWs were all zero).
	r.PSL = r.PSL&^0xFFFF | maskWord&0xFFE0

	// SP after the RET: past the frame and its saved registers, the
	// alignment bytes, and, for a CALLS, the argument list.
	mask := maskWord >> 16 & 0xFFF
	sp := callee + 20 + 4*uint32(bits.OnesCount32(mask)) + maskWord>>30

	if maskWord&(1<<29) != 0 {
		count, err := m.Mem.LoadLongword(m.CPU, sp)
		if err != nil {
			return r, err
		}

		sp += 4 + 4*(count&0xFF)
	}

	r.Value[registers.SP], r.Slot[registers.SP] = sp, 0

	// R2 to R11: the nearest frame inside level k that saved each.
	for reg := registers.R2; reg <= registers.R11; reg++ {
		for j := k - 1; j >= 0; j-- {
			frame := levels[j].FP

			word := maskWord
			if j != k-1 {
				if word, err = m.Mem.LoadLongword(m.CPU, frame+4); err != nil {
					return r, err
				}
			}

			saves := word >> 16 & 0xFFF
			if saves&(1<<uint(reg)) == 0 {
				continue
			}

			below := uint32(bits.OnesCount32(saves & (1<<uint(reg) - 1)))
			slot := frame + 20 + 4*below

			v, err := m.Mem.LoadLongword(m.CPU, slot)
			if err != nil {
				return r, err
			}

			r.Value[reg], r.Slot[reg] = v, slot

			break
		}
	}

	return r, nil
}

// byteSlice is a disasm.ByteReader over bytes held in Go, address 0 the
// first; past the end it reads zeros.
type byteSlice []byte

func (b byteSlice) ByteAt(addr uint32) byte {
	if addr < uint32(len(b)) {
		return b[addr]
	}

	return 0
}

// InstructionInBytes is the instruction data holds, as the debugger's
// EXAMINE/INSTRUCTION writes it: what VMS's shows for a register, whose
// contents it decodes as an instruction (testdata/dbg89: EXAMINE/INSTRUCTION
// REC 1\%PC, the bytes of 0000046D, is "CVTWD S^#04,S^#0.5000000"). An
// operand that names an address is relative to the bytes, which have none.
func (m *Machine) InstructionInBytes(data []byte, opts DisassembleOptions) (string, error) {
	radix := opts.Radix
	if radix == 0 {
		radix = m.symbolRadix()
	}

	names := consoleSymbolizer{m: m, radix: radix, numeric: !opts.Symbolic}

	dec, err := disasm.Disassemble(byteSlice(data), 0)
	if err != nil {
		return "", err
	}

	return dec.Format(m.formatOptions(opts, names)), nil
}
