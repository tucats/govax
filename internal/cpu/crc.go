package cpu

import "github.com/tucats/govax/internal/vax"

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

// emulCrc is CRC. N <- R0 LSS 0, Z <- R0 EQL 0, V <- 0, C <- 0, per the
// manual. A zero-length stream leaves R0 as the initial CRC unchanged (Note
// 7), which falls out naturally here since the loop never runs.
func emulCrc(e *Engine, d *Decoded) error {
	tbl := d.Operands[0].Addr

	inicrcV, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	crc := uint32(inicrcV)

	lenV, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length := uint16(lenV)
	stream := d.Operands[3].Addr

	var table [16]uint32

	for i := range table {
		v, err := e.mem.LoadLongword(e.cpu, tbl+uint32(i*4))
		if err != nil {
			return err
		}

		table[i] = v
	}

	for ; length != 0; length-- {
		b, err := e.mem.LoadByte(e.cpu, stream)
		if err != nil {
			return err
		}

		crc ^= uint32(b)
		crc = (crc >> 4) ^ table[crc&0xF]
		crc = (crc >> 4) ^ table[crc&0xF]
		stream++
	}

	e.cpu.SetGPR(vax.R0, crc)
	e.cpu.SetGPR(vax.R1, 0)
	e.cpu.SetGPR(vax.R2, 0)
	e.cpu.SetGPR(vax.R3, stream)

	psl := e.cpu.PSL()
	psl.SetN(int32(crc) < 0)
	psl.SetZ(crc == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)
	
	return nil
}
