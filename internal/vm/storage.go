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
// Everything in this file works at the level of *virtual* addresses (as
// opposed to memory.go's phys/readPhysLongword/writePhysLongword, which
// work directly in physical addresses): every method here first calls
// through to translate.go's Translate to turn the virtual address into a
// physical one — and, in doing so, checks the page's protection bits and
// updates its Modify bit — before touching any actual byte. This is the
// layer that the rest of the emulator (instruction decode, the console's
// EXAMINE/DEPOSIT commands, and so on) should call through instead of
// reaching into Memory's physical-address helpers directly, since those
// skip translation and protection checking entirely.
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
//
// The `(byte, error)` return signature — a result value paired with an
// error that's non-nil exactly when something went wrong — is Go's normal
// substitute for exceptions: rather than throwing, a function that can fail
// returns an extra value the caller is expected to check with `if err !=
// nil { ... }` immediately after the call, as every method in this file
// does. When err is non-nil, the other return value (here, the byte) should
// be treated as meaningless — by convention it's usually the type's zero
// value (0 for byte), as it is throughout this file.
func (m *Memory) LoadByte(cpu *vax.CPU, addr uint32) (byte, error) {
	paddr, err := m.Translate(cpu, addr, AccessRead)
	if err != nil {
		return 0, err
	}

	b, err := m.phys(paddr, 1)
	if err != nil {
		return 0, err
	}

	m.readCount++

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

	m.writeCount++
	b[0] = v

	return nil
}

// Load reads len(dest) bytes starting at a virtual address into dest, in VAX
// (little-endian) byte order.
//
// dest is a slice the caller already owns (typically a fixed-size array
// like the `var buf [2]byte` below, sliced with buf[:] — see "Why [N]byte
// and buf[:]" further down); Load fills it in place rather than allocating
// and returning a new one, which is why its own signature is just `error`
// with no accompanying data value. `for i := range dest` iterates over
// every valid index of dest (0 to len(dest)-1) without needing to know its
// length up front.
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
//
// `for i, b := range src` is Go's usual two-value form of range over a
// slice: i is the index, b is a copy of src[i] — equivalent to writing
// `for i := range src { b := src[i]; ... }`, just more idiomatic.
func (m *Memory) Store(cpu *vax.CPU, addr uint32, src []byte) error {
	for i, b := range src {
		if err := m.StoreByte(cpu, addr+uint32(i), b); err != nil {
			return err
		}
	}

	return nil
}

// Why [N]byte and buf[:], seen throughout the rest of this file:
//
// `var buf [2]byte` declares buf as a Go *array* (fixed size, known at
// compile time — two bytes, always) rather than a slice (dynamically
// sized). Arrays are convenient here because their size documents exactly
// how many bytes a word/longword/quadword takes, and because declaring one
// as a local variable allocates it directly on the stack with no separate
// heap allocation — but the Load/Store/binary.LittleEndian methods above
// and below all expect a slice ([]byte), not an array. `buf[:]` is a slice
// expression with no bounds given, which is shorthand for "a slice over
// the *entire* array" (equivalent to `buf[0:len(buf)]`); it doesn't copy
// buf's bytes, just hands out a slice that refers to the same underlying
// storage, so writes through that slice are writes to buf itself.

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
//
// This models a real quirk of the VAX instruction set: many instructions
// can operate on a byte, a word, or a longword, and when an operand is a
// register rather than a memory location, only the requested number of
// low-order bytes of that register are supposed to change — e.g. loading a
// byte into R3 must not disturb the upper three bytes already sitting in
// R3. R0-R15 are the 16 architected general-purpose registers (see
// vax.R15); some of the decode engine's own scratch registers reuse this
// same LoadRegister helper for temporary values that have no such
// "preserve the rest" requirement and should simply start from zero
// instead, which is what the `reg > vax.R15` check distinguishes.
//
// buf[:count] slices the local 4-byte array down to just the first count
// bytes (1, 2, or 4) — the same no-copy slicing idea used throughout this
// file, just with an explicit upper bound instead of the implicit
// "whole array" of buf[:].
func (m *Memory) LoadRegister(cpu *vax.CPU, reg vax.Reg, addr uint32, count int) error {
	var buf [4]byte

	if err := m.Load(cpu, addr, buf[:count]); err != nil {
		return err
	}

	v := cpu.GPR(reg)
	if reg > vax.R15 {
		v = 0
	}

	// Merge the newly-loaded bytes into v one at a time, from the
	// least-significant byte up, without disturbing any byte of v beyond
	// the count being loaded. For each byte i: `shift` is how far to move
	// it to reach its proper position (byte 0 stays put, byte 1 moves up
	// 8 bits, and so on); `v &^ (0xFF << shift)` clears just that one
	// byte's-worth of bits in v (see pte.go's "How the bit packing works"
	// for more on the &^ operator); and `| uint32(buf[i])<<shift` drops
	// the loaded byte into the now-empty slot. This is the same
	// clear-then-set pattern PTE's Set* methods use, just applied
	// byte-by-byte in a loop instead of once for a fixed-width field.
	for i := 0; i < count; i++ {
		shift := uint(i * 8)
		v = (v &^ (0xFF << shift)) | uint32(buf[i])<<shift
	}

	cpu.SetGPR(reg, v)

	return nil
}
