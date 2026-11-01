package librtl

import (
	"encoding/binary"
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/vaxfloat"
)

// LIB$FIXUP_FLT:
//
//	LIB$FIXUP_FLT signal-args ,mechanism-args [,new-operand]
//
// A reserved floating operand (a value with its sign set and its exponent
// zero, which no floating instruction accepts) is a fault: the instruction
// has done nothing, and continuing would run it again and fault again. This
// routine, a condition handler or called from one, looks at the instruction,
// finds its first reserved operand, replaces it with new-operand (an F_floating
// value, converted to the operand's format) or with zero, and returns
// SS$_CONTINUE, so the instruction runs again on the new value. An operand of
// an instruction with two reserved operands faults once for each.
//
// It returns SS$_RESIGNAL if the condition isn't SS$_ROPRAND from an
// instruction, if there aren't two arguments, or if the instruction has no
// reserved floating operand of its own to change: a POLY's table is memory
// the instruction reads as it goes, and a packed decimal string isn't
// floating. A reserved new-operand is SS$_ROPRAND, and an operand that can't be
// written SS$_ACCVIO: an immediate is in the instruction stream, which an
// image's code doesn't let it write (testdata/for83's part C, cases 40 to 91).
// A fault restores the registers an instruction's operands changed, so the
// specifiers are worked through here as the instruction will work through
// them when it runs again, each autoincrement counted for the next operand.

func libFixupFlt(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return ssResignal, nil
	}

	mem, c := env.Memory(), env.CPU()

	count, err1 := mem.LoadLongword(c, argv[0])
	cond, err2 := mem.LoadLongword(c, argv[0]+4)

	if err := errors.Join(err1, err2); err != nil {
		return memoryFault(env, err)
	}

	if cond&condIDMask != ssRoprand&condIDMask || count < 3 {
		return ssResignal, nil
	}

	regs, ok := env.ConditionRegisters(argv[0])
	if !ok {
		return ssResignal, nil
	}

	replacement := vaxfloat.FromFloat64(0)

	if len(argv) > 2 && argv[2] != 0 {
		raw, err := mem.LoadLongword(c, argv[2])
		if err != nil {
			return memoryFault(env, err)
		}

		if replacement, err = vaxfloat.Unpack(vaxfloat.F, vaxfloat.Bits{Lo: uint64(raw)}); err != nil {
			return ssRoprand, nil
		}
	}

	pc, err := mem.LoadLongword(c, argv[0]+4*(count-1))
	if err != nil {
		return memoryFault(env, err)
	}

	inst, err := disasm.Disassemble(streamReader{env}, pc)
	if err != nil {
		return ssResignal, nil
	}

	f := fixup{env: env, regs: regs, value: replacement}

	for _, op := range inst.Operands {
		found, status := f.operand(op)
		if found {
			return status, nil
		}
	}

	return ssResignal, nil
}

// streamReader reads the instruction stream for the disassembler.
type streamReader struct{ env *corevms.Environment }

func (r streamReader) ByteAt(addr uint32) byte {
	b, err := r.env.Memory().LoadByte(r.env.CPU(), addr)
	if err != nil {
		return 0
	}

	return b
}

// fixup is one instruction's specifiers being worked through. changed holds
// the registers its autoincrements and autodecrements have changed so far.
type fixup struct {
	env     *corevms.Environment
	regs    *corevms.ConditionRegisters
	value   vaxfloat.Value
	changed map[int]uint32
}

// reg returns a register as the specifiers so far leave it.
func (f *fixup) reg(n int) (uint32, error) {
	if v, ok := f.changed[n]; ok {
		return v, nil
	}

	return f.regs.Get(n)
}

func (f *fixup) setReg(n int, v uint32) {
	if f.changed == nil {
		f.changed = map[int]uint32{}
	}

	f.changed[n] = v
}

// operand evaluates one specifier. found is true if it was the reserved
// operand, and then status is the routine's result.
func (f *fixup) operand(op disasm.Operand) (found bool, status uint32) {
	floating := op.Type.IsFloat() && (op.Access == cpu.AccessRead || op.Access == cpu.AccessModify)

	switch op.Mode {
	case disasm.ModeLiteral, disasm.ModeBranch, disasm.ModeInline:
		return false, 0

	case disasm.ModeRegister:
		if !floating {
			return false, 0
		}

		return f.inRegisters(op)

	case disasm.ModeImmediate:
		if floating && len(op.Bytes) >= 2 && isReserved(op.Type, binary.LittleEndian.Uint16(op.Bytes)) {
			return true, ssAccVio
		}

		return false, 0
	}

	addr, err := f.address(op)
	if err != nil {
		return true, ssAccVio
	}

	if !floating {
		return false, 0
	}

	mem, c := f.env.Memory(), f.env.CPU()

	word, err := mem.LoadWord(c, addr)
	if err != nil {
		return true, ssAccVio
	}

	if !isReserved(op.Type, word) {
		return false, 0
	}

	data, status := f.newBits(op)
	if status != 0 {
		return true, status
	}

	for i, b := range data {
		if err := mem.StoreByte(c, addr+uint32(i), b); err != nil {
			return true, ssAccVio
		}
	}

	return true, ssContinue
}

// inRegisters is a floating operand in registers: Rn, and Rn+1 up for a
// longer format.
func (f *fixup) inRegisters(op disasm.Operand) (found bool, status uint32) {
	first, err := f.regs.Get(op.Register)
	if err != nil || !isReserved(op.Type, uint16(first)) {
		return false, 0
	}

	data, status := f.newBits(op)
	if status != 0 {
		return true, status
	}

	for i := 0; i < len(data); i += 4 {
		n := op.Register + i/4
		if n > 11 {
			return true, ssAccVio
		}

		if err := f.regs.Set(n, binary.LittleEndian.Uint32(data[i:])); err != nil {
			return true, ssAccVio
		}
	}

	return true, ssContinue
}

// newBits is the new value in op's format, as memory holds it. status is
// other than 0 if it doesn't fit: F_floating's range is inside the others',
// so that can't happen.
func (f *fixup) newBits(op disasm.Operand) (data []byte, status uint32) {
	format := op.Type.FloatFormat()

	bits, err := vaxfloat.Pack(format, f.value)
	if err != nil {
		return nil, ssRoprand
	}

	data = binary.LittleEndian.AppendUint64(nil, bits.Lo)
	data = binary.LittleEndian.AppendUint64(data, bits.Hi)

	return data[:format.Size()], 0
}

// address is the operand's address, with its specifier's change to its
// register, and for an indexed one the index added.
func (f *fixup) address(op disasm.Operand) (uint32, error) {
	addr, err := f.base(op)
	if err != nil {
		return 0, err
	}

	if op.Indexed() {
		index, err := f.reg(op.Index)
		if err != nil {
			return 0, err
		}

		addr += index * uint32(op.Size)
	}

	return addr, nil
}

func (f *fixup) base(op disasm.Operand) (uint32, error) {
	mem, c := f.env.Memory(), f.env.CPU()

	deferred := func(addr uint32) (uint32, error) {
		if !op.Deferred {
			return addr, nil
		}

		return mem.LoadLongword(c, addr)
	}

	switch op.Mode {
	case disasm.ModeAbsolute:
		return op.Target, nil

	case disasm.ModeRelative:
		return deferred(op.Target)
	}

	v, err := f.reg(op.Register)
	if err != nil {
		return 0, err
	}

	switch op.Mode {
	case disasm.ModeRegisterDeferred:
		return v, nil

	case disasm.ModeAutoincrement:
		f.setReg(op.Register, v+uint32(op.Size))

		return v, nil

	case disasm.ModeAutodecrement:
		v -= uint32(op.Size)
		f.setReg(op.Register, v)

		return v, nil

	case disasm.ModeAutoincrementDeferred:
		f.setReg(op.Register, v+4)

		return mem.LoadLongword(c, v)
	}

	return deferred(v + uint32(op.Displacement))
}

// isReserved reports whether a floating value's first word, which holds its
// sign and exponent, makes it a reserved operand: the sign set and the
// exponent zero.
func isReserved(t cpu.DataType, word uint16) bool {
	switch t.FloatFormat() {
	case vaxfloat.G:
		return word&0xFFF0 == 0x8000
	case vaxfloat.H:
		return word == 0x8000
	}

	return word&0xFF80 == 0x8000
}
