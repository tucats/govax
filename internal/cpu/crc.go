package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// CRC has no working C reference to port: emul_crc.c's `emul_crc` is a
// complete no-op stub (`return VAX_OK;`, no computation at all), and its
// instruction_table.h row has an all-zero operand count/scale/access to
// match. Per user direction (docs/PHASE-06.md's design notes,
// docs/DEVIATIONS.md), this is implemented from vax_instr_set.pdf's CRC
// entry instead -- the table row is patched to the manual's actual operand
// shape by internal/cpu/gen's knownTableFixes (see its comment) rather than
// hand-edited into the generated instructions_table.go.
//
// The instruction's own algorithm description processes the stream one bit
// at a time (XOR the byte into the CRC's low 8 bits, then 8 rounds of
// "shift right one bit, XOR in the polynomial if the bit shifted out was
// 1"); Note 6 states this is equivalent to processing 4 bits at a time using
// all 16 entries of the table operand (table index = the CRC's low nibble),
// which is what emulCrc does -- verified against the manual's own CRC-16
// polynomial (Note 5, poly 0xA001 once converted from octal) and the
// standard CRC-16/ARC test vector (CRC of "123456789" is 0xBB3D) via a
// standalone reference implementation of both the bit-at-a-time and
// nibble-table forms, not just derived from the prose.
func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x0B}), emulCrc)
}

// CRC is the CRC instruction's calculation: crc run over the length
// bytes at stream, a nibble at a time through the 16-longword table at
// table, as Note 6 describes. It returns the CRC. LIBRTL's LIB$CRC shares
// it.
func CRC(cpu *registers.CPU, mem *vm.Memory, table, crc uint32, length uint16, stream uint32) (uint32, error) {
	var t [16]uint32

	for i := range t {
		v, err := mem.LoadLongword(cpu, table+uint32(i*4))
		if err != nil {
			return 0, err
		}

		t[i] = v
	}

	for i := range length {
		b, err := mem.LoadByte(cpu, stream+uint32(i))
		if err != nil {
			return 0, err
		}

		crc ^= uint32(b)
		crc = (crc >> 4) ^ t[crc&0xF]
		crc = (crc >> 4) ^ t[crc&0xF]
	}

	return crc, nil
}

// CRCTable is the 16-longword table for the polynomial mask poly
// (CRC-16's is octal 120001, ^XA001; the manual's Note 5): entry i is i shifted right four times, the polynomial
// XORed in after each shift that shifts out a 1, the manual's table
// construction. LIBRTL's LIB$CRC_TABLE is this.
func CRCTable(poly uint32) [16]uint32 {
	var t [16]uint32

	for i := range t {
		v := uint32(i)

		for range 4 {
			if v&1 != 0 {
				v = (v >> 1) ^ poly
			} else {
				v >>= 1
			}
		}

		t[i] = v
	}

	return t
}

// emulCrc is CRC tbl.ab, inicrc.rl, strlen.rw, stream.ab. R0 gets the
// CRC, R1 and R2 0, and R3 the address just past the stream. N <- R0 LSS
// 0, Z <- R0 EQL 0, V <- 0, C <- 0, per the manual. A zero-length stream
// leaves R0 as the initial CRC unchanged (Note 7).
func emulCrc(e *Engine, d *Decoded) error {
	inicrc, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	lenV, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length, stream := uint16(lenV), d.Operands[3].Addr

	crc, err := CRC(e.cpu, e.mem, d.Operands[0].Addr, uint32(inicrc), length, stream)
	if err != nil {
		return err
	}

	e.cpu.SetGPR(registers.R0, crc)
	e.cpu.SetGPR(registers.R1, 0)
	e.cpu.SetGPR(registers.R2, 0)
	e.cpu.SetGPR(registers.R3, stream+uint32(length))

	psl := e.cpu.PSL()
	psl.SetN(int32(crc) < 0)
	psl.SetZ(crc == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}
