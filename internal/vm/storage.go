package vm

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/vax"
)

// This file is the Go port of storage.c's load_memory/store_memory/
// load_register primitive layer (get_operand/put_operand stay out of scope
// here — they belong to Phase 03's operand-decode engine, per
// docs/PHASE-02.md).
//
// storage.c handles a multi-byte access that straddles a page boundary by
// falling back to a byte-at-a-time loop through load_byte/store_memory(...,1)
// (see its "spans_pages" branches), and only takes a single wide translation
// when the access fits in one page. Since Translate here has no page-boundary-
// crossing TB/STC cache to make that distinction worth optimizing for (see
// translate.go), every multi-byte access below just uses that byte-at-a-time
// path unconditionally — same result, simpler code, one fewer place for a
// page-boundary bug to hide.

// LoadByte reads one byte at a virtual address, translating and
// fault-checking through cpu.
func (m *Memory) LoadByte(cpu *vax.CPU, addr uint32) (byte, error) {
	paddr, err := m.Translate(cpu, addr, AccessRead)
	if err != nil {
		return 0, err
	}
	b, err := m.phys(paddr, 1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// StoreByte writes one byte at a virtual address, translating and
// fault-checking through cpu.
func (m *Memory) StoreByte(cpu *vax.CPU, addr uint32, v byte) error {
	paddr, err := m.Translate(cpu, addr, AccessWrite)
	if err != nil {
		return err
	}
	b, err := m.phys(paddr, 1)
	if err != nil {
		return err
	}
	b[0] = v
	return nil
}

// Load reads len(dest) bytes starting at a virtual address into dest, in VAX
// (little-endian) byte order.
func (m *Memory) Load(cpu *vax.CPU, addr uint32, dest []byte) error {
	for i := range dest {
		b, err := m.LoadByte(cpu, addr+uint32(i))
		if err != nil {
			return err
		}
		dest[i] = b
	}
	return nil
}

// Store writes src starting at a virtual address, in VAX (little-endian)
// byte order.
func (m *Memory) Store(cpu *vax.CPU, addr uint32, src []byte) error {
	for i, b := range src {
		if err := m.StoreByte(cpu, addr+uint32(i), b); err != nil {
			return err
		}
	}
	return nil
}

// LoadWord reads a 16-bit word at a virtual address.
func (m *Memory) LoadWord(cpu *vax.CPU, addr uint32) (uint16, error) {
	var buf [2]byte
	if err := m.Load(cpu, addr, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

// StoreWord writes a 16-bit word at a virtual address.
func (m *Memory) StoreWord(cpu *vax.CPU, addr uint32, v uint16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	return m.Store(cpu, addr, buf[:])
}

// LoadLongword reads a 32-bit longword at a virtual address.
func (m *Memory) LoadLongword(cpu *vax.CPU, addr uint32) (uint32, error) {
	var buf [4]byte
	if err := m.Load(cpu, addr, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

// StoreLongword writes a 32-bit longword at a virtual address.
func (m *Memory) StoreLongword(cpu *vax.CPU, addr uint32, v uint32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	return m.Store(cpu, addr, buf[:])
}

// LoadQuadword reads a 64-bit quadword at a virtual address.
//
// storage.c splits an 8-byte access into two 4-byte load_memory/
// store_memory calls to keep byte order correct on big-endian hosts; on the
// little-endian-only host this Go port targets that split has no observable
// effect (see docs/PLAN.md's LONGWORD/typedef discussion), so it isn't
// ported — a straight little-endian 8-byte decode is equivalent.
func (m *Memory) LoadQuadword(cpu *vax.CPU, addr uint32) (uint64, error) {
	var buf [8]byte
	if err := m.Load(cpu, addr, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

// StoreQuadword writes a 64-bit quadword at a virtual address.
func (m *Memory) StoreQuadword(cpu *vax.CPU, addr uint32, v uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	return m.Store(cpu, addr, buf[:])
}

// LoadRegister loads count bytes (1, 2 or 4) from a virtual address into the
// low-order bytes of reg, leaving the remaining bytes of an architected
// register (reg <= vax.R15) undisturbed, or zeroing a scratch/temporary
// register (reg > vax.R15) first — matching load_register's distinction
// between "you asked for a word, only the low word changes" for real
// registers and "scratch registers always start from zero" for decode-time
// temporaries (sign/zero-extension of the loaded value is the caller's
// decision, same as in the C source).
func (m *Memory) LoadRegister(cpu *vax.CPU, reg vax.Reg, addr uint32, count int) error {
	var buf [4]byte
	if err := m.Load(cpu, addr, buf[:count]); err != nil {
		return err
	}

	v := cpu.GPR(reg)
	if reg > vax.R15 {
		v = 0
	}
	for i := 0; i < count; i++ {
		shift := uint(i * 8)
		v = (v &^ (0xFF << shift)) | uint32(buf[i])<<shift
	}
	cpu.SetGPR(reg, v)
	return nil
}
