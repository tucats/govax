package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
)

// The CRC routines (docs/PHASE-53, subtask 11): LIB$CRC_TABLE builds the
// 16-longword table the CRC instruction works from, and LIB$CRC runs the
// instruction's calculation over a string. Both are the CPU's own
// (cpu.CRCTable, cpu.CRC).

// libCrcTable is LIB$CRC_TABLE (LIB-33): the table for the polynomial
// mask polynomial-coefficient, stored at crc-table. The manual gives it no
// value; govax returns SS$_NORMAL (unconfirmed).
//
//	LIB$CRC_TABLE polynomial-coefficient ,crc-table
func libCrcTable(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	poly, err := mem.LoadLongword(c, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	for i, v := range cpu.CRCTable(poly) {
		if err := mem.StoreLongword(c, arg(argv, 1)+uint32(4*i), v); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return ssNormal, nil
}

// libCrc is LIB$CRC (LIB-31): the CRC of stream, from initial-crc, through
// the table at crc-table. A CRC-16 polynomial's result is its low word.
// The stream is a string of any class; an invalid descriptor, there being
// no status to return, is signaled as LIB$_INVSTRDES (unconfirmed).
//
//	LIB$CRC crc-table ,initial-crc ,stream
func libCrc(env *corevms.Environment, argv []uint32) (uint32, error) {
	initial, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	stream, err := readDescriptor(env, arg(argv, 2))
	if err != nil {
		return searchFailure(env, err)
	}

	crc, err := cpu.CRC(env.CPU(), env.Memory(), arg(argv, 0), initial, uint16(stream.length), stream.text)
	if err != nil {
		return memoryFault(env, err)
	}

	return crc, nil
}
