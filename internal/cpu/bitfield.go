package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// This is the Go port of emul_bitfield.c's field instructions: EXTV/EXTZV,
// CMPV/CMPZV, INSV, FFS/FFC. The bit-branch instructions that live in the
// same C file (BBS/BBC/BBSS/BBCS/BBSC/BBCC/BBSSI/BBCCI) are in bitbranch.go.
//
// All six instructions share a position/size/base operand triplet: base is
// either a register (the field lives in one or two adjacent registers) or a
// VAX address (the field lives in memory, byte-addressed with position as a
// possibly-negative bit displacement from that address) -- decided by the
// base operand's decoded Kind, mirroring the C source's is_register[n]
// check. A base that decoded as an immediate (a short literal or PC-relative
// immediate operand -- the C source's OP_TEMPORARY) is a reserved-operand
// fault: there's no addressable location to read or write a field from.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0xEA, emulFf)   // FFS
	reg(0xEB, emulFf)   // FFC
	reg(0xEC, emulCmpv) // CMPV
	reg(0xED, emulCmpv) // CMPZV
	reg(0xEE, emulExtv) // EXTV
	reg(0xEF, emulExtv) // EXTZV
	reg(0xF0, emulInsv) // INSV
}

// bitFieldMask returns the low-order size-bit mask (0 for size <= 0, all 32
// bits for size >= 32), avoiding the undefined-in-Go 1<<32 shift emul_bit-
// field.c's own (1 << size) - 1 formula would need for a full-width field.
func bitFieldMask(size int) uint32 {
	if size <= 0 {
		return 0
	}

	if size >= 32 {
		return 0xFFFFFFFF
	}

	return uint32(1)<<uint(size) - 1
}

// signExtendBitField sign-extends a size-bit field value to a full 32 bits.
// Port of emul_bitfield.c's bit_sext, avoided for size 0 (where the C
// source's 1 << (size-1) shifts by -1, undefined behavior it happens to get
// away with because the value is 0 regardless of the shift's outcome).
func signExtendBitField(value uint32, size int) uint32 {
	if size <= 0 || size >= 32 {
		return value
	}

	if value&(1<<uint(size-1)) != 0 {
		return value | ^bitFieldMask(size)
	}

	return value & bitFieldMask(size)
}

// fieldOperands reads the shared position/size operand pair common to all
// six field instructions: position as a sign-extended longword (memory-base
// fields may legitimately use a negative bit displacement), size as a
// sign-extended byte (the manual bounds it to 0-32; anything outside that is
// a reserved-operand fault, checked by getRegisterField/getMemoryField/
// setRegisterField/setMemoryField rather than here).
func fieldOperands(cpu *vax.CPU, mem *vm.Memory, posOp, sizeOp Operand) (position int32, size int, err error) {
	p, err := posOp.Load(cpu, mem)
	if err != nil {
		return 0, 0, err
	}

	s, err := sizeOp.Load(cpu, mem)
	if err != nil {
		return 0, 0, err
	}

	return int32(signExtend(p, posOp.Size)), int(signExtend(s, sizeOp.Size)), nil
}

// loadField extracts a bit field from base (a register or memory operand)
// given its position and size, faulting reserved-operand for an
// out-of-range size/position or an unaddressable (immediate) base.
func loadField(e *Engine, base Operand, position int32, size int) (uint32, error) {
	switch base.Kind {
	case OperandRegister:
		return getRegisterField(e.cpu, position, size, base.Reg)

	case OperandMemory:
		return getMemoryField(e.cpu, e.mem, position, size, base.Addr)

	default:
		return 0, &Fault{Code: ExcReservedOp}
	}
}

// storeField is loadField's write-side counterpart, used by INSV.
func storeField(e *Engine, base Operand, position int32, size int, data uint32) error {
	switch base.Kind {
	case OperandRegister:
		return setRegisterField(e.cpu, position, size, base.Reg, data)

	case OperandMemory:
		return setMemoryField(e.cpu, e.mem, position, size, base.Addr, data)

	default:
		return &Fault{Code: ExcReservedOp}
	}
}

// getRegisterField reads a size-bit field starting at bit position of the
// register pair base/base+1. Port of emul_bitfield.c's get_register_field,
// with its cross-register split arithmetic corrected -- see
// docs/DEVIATIONS.md ("get_register_field/set_register_field split a field
// one bit short of the base register").
func getRegisterField(cpu *vax.CPU, position int32, size int, base vax.Reg) (uint32, error) {
	if size < 0 || size > 32 || position < 0 || position > 31 || (base == vax.PC && int(position)+size > 31) {
		return 0, &Fault{Code: ExcReservedOp}
	}

	lo := cpu.GPR(base) >> uint(position)
	loBits := 32 - int(position)

	if size <= loBits {
		return lo & bitFieldMask(size), nil
	}

	hiBits := size - loBits
	hi := cpu.GPR(base+1) & bitFieldMask(hiBits)

	return (lo & bitFieldMask(loBits)) | hi<<uint(loBits), nil
}

// setRegisterField is getRegisterField's write-side counterpart, used by
// INSV. Port of emul_bitfield.c's set_register_field, with the same
// cross-register split fix.
func setRegisterField(cpu *vax.CPU, position int32, size int, base vax.Reg, data uint32) error {
	if size < 0 || size > 32 || position < 0 || position > 31 || (base == vax.PC && int(position)+size > 31) {
		return &Fault{Code: ExcReservedOp}
	}

	loBits := 32 - int(position)

	if size <= loBits {
		mask := bitFieldMask(size) << uint(position)
		cpu.SetGPR(base, (cpu.GPR(base)&^mask)|((data<<uint(position))&mask))

		return nil
	}

	hiBits := size - loBits
	loMask := bitFieldMask(loBits) << uint(position)
	hiMask := bitFieldMask(hiBits)

	cpu.SetGPR(base, (cpu.GPR(base)&^loMask)|((data<<uint(position))&loMask))
	cpu.SetGPR(base+1, (cpu.GPR(base+1)&^hiMask)|((data>>uint(loBits))&hiMask))

	return nil
}

// bitFieldByteSpan returns the byte address of the first byte touched by a
// size-bit field at bit displacement position from base, and the field's bit
// offset within that first byte -- the position>>3 / position&7 split
// emul_bitfield.c's get_memory_field/set_memory_field both use, computed
// once and shared by their Go ports below.
func bitFieldByteSpan(base uint32, position int32) (addr uint32, bitOff uint) {
	return base + uint32(position>>3), uint(uint32(position) & 7)
}

// getMemoryField reads a size-bit field at bit displacement position from
// base, byte-addressed and little-endian (bit 0 of the field is the
// low-order bit of the byte at addr). Port of emul_bitfield.c's
// get_memory_field, replacing its bit-by-bit loop with an equivalent
// load-then-shift-then-mask (at most 5 bytes for a 32-bit field at any
// sub-byte offset) -- same result, no ISA behavior to preserve in the loop
// shape itself.
func getMemoryField(cpu *vax.CPU, mem *vm.Memory, position int32, size int, base uint32) (uint32, error) {
	if size < 0 || size > 32 {
		return 0, &Fault{Code: ExcReservedOp}
	}

	if size == 0 {
		return 0, nil
	}

	addr, bitOff := bitFieldByteSpan(base, position)
	nBytes := (bitOff + uint(size) + 7) / 8

	var v uint64

	for i := uint32(0); i < uint32(nBytes); i++ {
		b, err := mem.LoadByte(cpu, addr+i)
		if err != nil {
			return 0, err
		}

		v |= uint64(b) << (8 * i)
	}

	return uint32(v>>bitOff) & bitFieldMask(size), nil
}

// setMemoryField is getMemoryField's write-side counterpart, used by INSV.
// Port of emul_bitfield.c's set_memory_field, reading the whole affected
// byte span, merging data into it, and writing it back -- equivalent to
// (and simpler than) the C source's write-as-you-go per-byte loop, since
// nothing else can observe memory mid-instruction.
func setMemoryField(cpu *vax.CPU, mem *vm.Memory, position int32, size int, base uint32, data uint32) error {
	if size < 0 || size > 32 {
		return &Fault{Code: ExcReservedOp}
	}

	if size == 0 {
		return nil
	}

	addr, bitOff := bitFieldByteSpan(base, position)
	nBytes := (bitOff + uint(size) + 7) / 8

	var orig uint64

	for i := uint32(0); i < uint32(nBytes); i++ {
		b, err := mem.LoadByte(cpu, addr+i)
		if err != nil {
			return err
		}

		orig |= uint64(b) << (8 * i)
	}

	mask := (uint64(1)<<(bitOff+uint(size)) - 1) &^ (uint64(1)<<bitOff - 1)
	v := (orig &^ mask) | ((uint64(data) << bitOff) & mask)

	for i := uint32(0); i < uint32(nBytes); i++ {
		if err := mem.StoreByte(cpu, addr+i, byte(v>>(8*i))); err != nil {
			return err
		}
	}

	return nil
}

// emulExtv is EXTV/EXTZV: a size-bit field at position from base is
// extracted (sign-extended for EXTV, zero-extended for EXTZV) and stored in
// the destination longword. N/Z from the result, V <- 0, C <- 0 -- per the
// manual and emul_bitfield.c's emul_extv.
func emulExtv(e *Engine, d *Decoded) error {
	position, size, err := fieldOperands(e.cpu, e.mem, d.Operands[0], d.Operands[1])
	if err != nil {
		return err
	}

	field, err := loadField(e, d.Operands[2], position, size)
	if err != nil {
		return err
	}

	if d.Opcode.Function == 0xEE { // EXTV
		field = signExtendBitField(field, size)
	}

	psl := e.cpu.PSL()
	psl.SetN(signBit(uint64(field), 4))
	psl.SetZ(isZero(uint64(field), 4))
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return d.Operands[3].Store(e.cpu, e.mem, uint64(field))
}

// emulCmpv is CMPV/CMPZV: a size-bit field at position from base is
// extracted (sign-extended for CMPV, zero-extended for CMPZV) and compared
// with the fourth (longword) operand; neither operand is modified. N/Z/C
// from the comparison, V <- 0 -- per the manual and emul_bitfield.c's
// emul_cmpv.
func emulCmpv(e *Engine, d *Decoded) error {
	position, size, err := fieldOperands(e.cpu, e.mem, d.Operands[0], d.Operands[1])
	if err != nil {
		return err
	}

	field, err := loadField(e, d.Operands[2], position, size)
	if err != nil {
		return err
	}

	if d.Opcode.Function == 0xEC { // CMPV
		field = signExtendBitField(field, size)
	}

	data, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result, _, c := subResult(uint64(field), data, 4)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 4))
	psl.SetZ(isZero(result, 4))
	psl.SetV(false)
	psl.SetC(c)
	e.cpu.SetPSL(psl)

	return nil
}

// emulInsv is INSV: the low size bits of the first (longword) operand are
// written into the size-bit field at position from base. Condition codes
// are unaffected -- emul_bitfield.c's emul_insv never touches vax.pslw,
// matching the manual (INSV isn't listed as affecting N/Z/V/C).
func emulInsv(e *Engine, d *Decoded) error {
	data, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	position, size, err := fieldOperands(e.cpu, e.mem, d.Operands[1], d.Operands[2])
	if err != nil {
		return err
	}

	return storeField(e, d.Operands[3], position, size, uint32(data))
}

// emulFf is FFS/FFC: the size-bit field at position from base is searched,
// low bit first, for the first bit set (FFS) or clear (FFC); the
// destination is written with the position of that bit (or position+size if
// none matched). Z <- {a match was found}, N <- 0, V <- 0, C <- 0 -- per the
// manual and emul_bitfield.c's emul_ff. A zero-size field is a special case
// per the C source: no search is performed, Z is unconditionally set, and
// the destination gets position back unchanged.
func emulFf(e *Engine, d *Decoded) error {
	position, size, err := fieldOperands(e.cpu, e.mem, d.Operands[0], d.Operands[1])
	if err != nil {
		return err
	}

	field, err := loadField(e, d.Operands[2], position, size)
	if err != nil {
		return err
	}

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetV(false)
	psl.SetC(false)

	if size == 0 {
		psl.SetZ(true)
		e.cpu.SetPSL(psl)

		return d.Operands[3].Store(e.cpu, e.mem, uint64(uint32(position)))
	}

	testBit := uint32(1)
	if d.Opcode.Function == 0xEB { // FFC
		testBit = 0
	}

	for n := 0; n < size; n++ {
		if field&1 == testBit {
			psl.SetZ(false)
			e.cpu.SetPSL(psl)

			return d.Operands[3].Store(e.cpu, e.mem, uint64(uint32(position)+uint32(n)))
		}
		
		field >>= 1
	}

	psl.SetZ(true)
	e.cpu.SetPSL(psl)

	return d.Operands[3].Store(e.cpu, e.mem, uint64(uint32(position)+uint32(size)))
}
