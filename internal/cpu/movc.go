package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// MOVC3 and MOVC5 (Move Character), and MOVTC's copy (movtc.go).
//
// A register-mode source or destination address operand already faults
// at decode time (AccessAddress; see operand.go and docs/DEVIATIONS.md's
// "Register mode used where OP_AD/OP_VA access is required" finding).
//
// The lengths are unsigned words, 0 to 65,535: the manual's character
// string data type ("The length L of a string is in the range 0 to
// 65,535"; docs/DEVIATIONS.md).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x28, emulMovc3)
	reg(0x2C, emulMovc5)
}

// MoveCharacters is MOVC3's, MOVC5's, and MOVTC's copy: the srcLen bytes
// at src replace the dstLen bytes at dst, the destination's excess
// filled with fill, the source's excess not moved. With a table (MOVTC),
// each source byte is translated through the 256-byte table at
// *table first. The source is read before the destination is written,
// so the strings may overlap either way ("the result is as if the
// source were moved to a temporary place first"). It returns the source
// bytes not moved. LIBRTL's LIB$MOVC3 and LIB$MOVC5 share it.
func MoveCharacters(cpu *registers.CPU, mem *vm.Memory, srcLen uint16, src uint32, fill byte, dstLen uint16, dst uint32, table *uint32) (uint16, error) {
	moved := min(srcLen, dstLen)
	buf := make([]byte, dstLen)

	for i := range moved {
		b, err := mem.LoadByte(cpu, src+uint32(i))
		if err != nil {
			return 0, err
		}

		if table != nil {
			if b, err = mem.LoadByte(cpu, *table+uint32(b)); err != nil {
				return 0, err
			}
		}

		buf[i] = b
	}

	for i := moved; i < dstLen; i++ {
		buf[i] = fill
	}

	for i, b := range buf {
		if err := mem.StoreByte(cpu, dst+uint32(i), b); err != nil {
			return 0, err
		}
	}

	return srcLen - moved, nil
}

// moveOperands says where a move instruction's operands are, by index; a
// negative index is an operand the instruction doesn't have.
type moveOperands struct {
	srcLen, src, fill, table, dstLen, dst int
}

// emulMovc3 is MOVC3 len.rw, srcaddr.ab, dstaddr.ab: len bytes are copied
// from src to dst. R0, R2, R4, and R5 get 0, R1 the address just past the
// source and R3 just past the destination. N, V, and C are cleared and Z
// set.
func emulMovc3(e *Engine, d *Decoded) error {
	return moveCharacters(e, d, moveOperands{srcLen: 0, src: 1, fill: -1, table: -1, dstLen: -1, dst: 2})
}

// emulMovc5 is MOVC5 srclen.rw, srcaddr.ab, fill.rb, dstlen.rw,
// dstaddr.ab: the source replaces the destination, padded with fill or
// truncated. R0 gets the source bytes not moved, R1 the address of the
// first of them, R2, R4, and R5 0, and R3 the address just past the
// destination. N, Z, and C compare the two lengths as words (signed for
// N, unsigned for C); V is cleared.
func emulMovc5(e *Engine, d *Decoded) error {
	return moveCharacters(e, d, moveOperands{srcLen: 0, src: 1, fill: 2, table: -1, dstLen: 3, dst: 4})
}

// moveCharacters is MOVC3, MOVC5, and MOVTC, their operands at ops.
func moveCharacters(e *Engine, d *Decoded, ops moveOperands) error {
	load := func(i int) (uint32, error) {
		v, err := d.Operands[i].Load(e.cpu, e.mem)

		return uint32(v), err
	}

	srcLenV, err := load(ops.srcLen)
	if err != nil {
		return err
	}

	srcLen, dstLen := uint16(srcLenV), uint16(srcLenV)
	src, dst := d.Operands[ops.src].Addr, d.Operands[ops.dst].Addr

	var fill byte

	if ops.fill >= 0 {
		v, err := load(ops.fill)
		if err != nil {
			return err
		}

		fill = byte(v)
	}

	if ops.dstLen >= 0 {
		v, err := load(ops.dstLen)
		if err != nil {
			return err
		}

		dstLen = uint16(v)
	}

	var table *uint32
	if ops.table >= 0 {
		table = &d.Operands[ops.table].Addr
	}

	left, err := MoveCharacters(e.cpu, e.mem, srcLen, src, fill, dstLen, dst, table)
	if err != nil {
		return err
	}

	e.cpu.SetGPR(registers.R0, uint32(left))
	e.cpu.SetGPR(registers.R1, src+uint32(srcLen-left))
	e.cpu.SetGPR(registers.R2, 0)
	e.cpu.SetGPR(registers.R3, dst+uint32(dstLen))
	e.cpu.SetGPR(registers.R4, 0)
	e.cpu.SetGPR(registers.R5, 0)

	if table != nil { // MOVTC: R3 the table, R5 the destination's end
		e.cpu.SetGPR(registers.R3, *table)
		e.cpu.SetGPR(registers.R5, dst+uint32(dstLen))
	}

	result, _, c := subResult(uint64(srcLen), uint64(dstLen), 2)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 2))
	psl.SetZ(isZero(result, 2))
	psl.SetV(false)
	psl.SetC(c)
	e.cpu.SetPSL(psl)

	return nil
}
