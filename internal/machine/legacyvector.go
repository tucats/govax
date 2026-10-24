package machine

import (
	"github.com/tucats/govax/internal/vmsdef"
)

// This file is VMS V1's system service vector (docs/PHASE-72 - legacy
// images.md, subtask 3). Before VMS V4 an image called a system service
// through an entry in S0, at legacyVectorBase plus the service's offset;
// V4 moved the vector to P1, at p1VectorBase, keeping every offset (SYS$IMGSTA
// is ^X168 in both), and kept the old one too, so an image linked on V1
// still runs (testdata/legacy72, answer 8). govax's old vector is a
// forwarding stub per P1 entry, at the same offset: the P1 entry's own
// empty entry mask, so a CALL saves what it would there, then a jump to
// the P1 entry's XFC, which dispatches the service by its P1 address.
//
// S0 maps physical memory one to one (S0 page n is frame n), so the
// vector is VMINIT's first legacyVectorPages pages, from physical 0, and
// the system page table and the rest of VMINIT's layout follow it.

const (
	legacyVectorBase = 0x80000000
	p1VectorBase     = 0x7FFEDE00
)

// legacyImgsta is SYS$IMGSTA's entry in the old vector.
var legacyImgsta = legacyVectorBase + imgstaAddr - p1VectorBase

// legacyVectorBytes is the old vector's size: up to the end of the
// highest P1 entry's stub (8 bytes), and legacyVectorPages its pages.
// VMS 7.1's old vector has entries at least to ^X918 (testdata/legacy72);
// govax's has one for every P1 entry, a superset an old image can't tell
// from VMS's.
var (
	legacyVectorBytes = func() uint32 {
		end := uint32(0)
		for _, e := range vmsdef.P1VectorTable {
			end = max(end, e.Addr-p1VectorBase+legacyEntryBytes)
		}

		return end
	}()

	legacyVectorPages = (legacyVectorBytes + 511) / 512
)

// legacyEntryBytes is one stub's size: a procedure's entry mask and a
// JMP @#address.
const legacyEntryBytes = 8

// legacyStub is the old vector's stub for P1 entry e, and the offset it
// goes at; ok is false for a data cell (SYS$GL_ASTRET, SYS$GL_COMMON),
// which has no stub. A procedure's stub is an empty entry mask (the P1
// entry's own, .P1VECTOR's) and JMP @# its XFC, two bytes in; the one
// JMP-reached entry (SYS$SRCHANDLER) has no mask in either vector, so
// its stub is the jump alone, to the P1 entry itself.
func legacyStub(e vmsdef.P1VectorEntry) (offset uint32, code []byte, ok bool) {
	if e.DataCell() {
		return 0, nil, false
	}

	offset = e.Addr - p1VectorBase
	target := e.Addr + 2

	if e.Jmp {
		target = e.Addr
	} else {
		code = []byte{0x00, 0x00}
	}

	const jmpAbsolute = 0x17 // JMP, then mode 9F: @#address

	code = append(code, jmpAbsolute, 0x9F, byte(target), byte(target>>8), byte(target>>16), byte(target>>24))

	return offset, code, true
}

// writeLegacyVector writes the old vector's stubs into its pages, by
// their physical addresses (VMINIT runs it before turning MAPEN on).
func (m *Machine) writeLegacyVector() error {
	for _, e := range vmsdef.P1VectorTable {
		offset, code, ok := legacyStub(e)
		if !ok {
			continue
		}

		for i, b := range code {
			if err := m.Mem.StoreByte(m.CPU, offset+uint32(i), b); err != nil {
				return err
			}
		}
	}

	return nil
}
