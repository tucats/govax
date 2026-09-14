package vm

import (
	"encoding/binary"
	"fmt"
)

// Memory is a VAX machine's physical memory: a flat byte array addressed
// 0..Size()-1. It owns the raw storage itself (the Phase 02 open question on
// vm/vax.CPU coupling is resolved this way: internal/vax.CPU is registers/PSL
// only, and Memory is independent of it, taking a *vax.CPU as a parameter
// wherever address translation needs to consult MAPEN/base-and-length
// registers/cur_mod — see translate.go and storage.go).
//
// Only main RAM is modeled here. The C source's get_address also resolves
// console ROM, NVRAM, and a "microvax" special range, and falls through to
// memory-mapped I/O (load_io/store_io) for anything else — those belong to
// the console (Phase 08) and I/O (Phase 09) subsystems that own that state,
// so a physical address outside RAM is simply reported as unbacked for now.
type Memory struct {
	ram []byte
}

// NewMemory returns a Memory with size bytes of zeroed RAM.
func NewMemory(size uint32) *Memory {
	return &Memory{ram: make([]byte, size)}
}

// Size returns the number of bytes of RAM.
func (m *Memory) Size() uint32 { return uint32(len(m.ram)) }

// PhysicalAddressError reports an access to a physical address with no
// backing storage in this Memory.
type PhysicalAddressError struct {
	Addr uint32
}

func (e *PhysicalAddressError) Error() string {
	return fmt.Sprintf("vm: no physical storage at %#08x", e.Addr)
}

// phys returns the size-byte window of RAM starting at addr, or a
// PhysicalAddressError if any of it falls outside RAM.
func (m *Memory) phys(addr uint32, size uint32) ([]byte, error) {
	if uint64(addr)+uint64(size) > uint64(len(m.ram)) {
		return nil, &PhysicalAddressError{Addr: addr}
	}
	return m.ram[addr : addr+size], nil
}

// readPhysLongword and writePhysLongword give translate.go direct,
// untranslated access to a page table entry's storage (PTEs live at
// physical addresses computed from P0BR/P1BR/SBR, not virtual ones).

func (m *Memory) readPhysLongword(addr uint32) (uint32, error) {
	b, err := m.phys(addr, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (m *Memory) writePhysLongword(addr uint32, v uint32) error {
	b, err := m.phys(addr, 4)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(b, v)
	return nil
}
