package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// Dynamic strings (docs/PHASE-54, "Dynamic strings"): LIB$SGET1_DD,
// LIB$SFREE1_DD, LIB$SFREEN_DD, and their JSB entries.
//
// A dynamic string is a class D descriptor whose text lives in the heap:
//
//	+0  DSC$W_LENGTH   word: the string's length
//	+2  DSC$B_DTYPE    byte: the data type (14, text)
//	+3  DSC$B_CLASS    byte: 2, dynamic
//	+4  DSC$A_POINTER  longword: the text's address, or 0 for none
//
// The RTL owns the storage: a routine that writes a dynamic string reuses
// the block when it's big enough and otherwise frees it and allocates
// another (dynamicText). The blocks come from the process heap
// (Environment.AllocateVM), until Phase 59 gives the RTL its zones.

var libFatErrLib = vmsdef.LibrarySymbols["LIB$_FATERRLIB"]

// dynamicText returns where n bytes of a dynamic string's text go. text is
// the descriptor's pointer, and length its length. The block at text is
// kept when it holds n bytes: a heap block by its real size, or, when text
// isn't a heap block's address (storage the program set up itself), by
// the descriptor's length. Otherwise the block is freed (if it's the
// heap's) and a new one allocated; with none needed (n is 0 and alloc0
// false) the result is 0. errInsVirMem means the heap is full.
func dynamicText(env *corevms.Environment, text, length, n uint32, alloc0 bool) (uint32, error) {
	if text != 0 {
		size, block := env.BlockSize(text)
		if !block {
			size = length
		}

		if n <= size {
			return text, nil
		}

		if block {
			env.FreeVM(text)
		}
	}

	if n == 0 && !alloc0 {
		return 0, nil
	}

	addr, err := env.AllocateVM(n, 0)
	if err != nil {
		return 0, err
	}

	if addr == 0 || addr == 0xFFFFFFFF {
		return 0, errInsVirMem
	}

	return addr, nil
}

// getDynamic is LIB$SGET1_DD's work: give the dynamic string described at
// desc room for length bytes, setting its length, class (D), and pointer.
// The class isn't checked first, nor the type changed. A length of 0
// still allocates (the manual: "a small amount of space"), so the
// pointer is never 0 after.
func getDynamic(env *corevms.Environment, desc, length uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	text, err := mem.LoadLongword(cpu, desc+4)
	if err != nil {
		return memoryFault(env, err)
	}

	old, err := mem.LoadWord(cpu, desc)
	if err != nil {
		return memoryFault(env, err)
	}

	if text, err = dynamicText(env, text, uint32(old), length, true); err != nil {
		return stringFailure(env, err)
	}

	if err := mem.StoreWord(cpu, desc, uint16(length)); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreByte(cpu, desc+3, dscClassD); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, desc+4, text); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// freeDynamic is LIB$SFREE1_DD's work: free the dynamic string described
// at desc and set its pointer and length to 0. The class isn't checked. A
// pointer of 0 has nothing to free; one that isn't a heap block's is
// LIB$_FATERRLIB (an internal consistency check failed), the descriptor
// left as it was.
func freeDynamic(env *corevms.Environment, desc uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	text, err := mem.LoadLongword(cpu, desc+4)
	if err != nil {
		return memoryFault(env, err)
	}

	if text != 0 && !env.FreeVM(text) {
		return libFatErrLib, nil
	}

	if err := mem.StoreWord(cpu, desc, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, desc+4, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// freeDynamics is LIB$SFREEN_DD's work: freeDynamic of count adjacent
// descriptors from first, stopping at the first that fails.
func freeDynamics(env *corevms.Environment, count, first uint32) (uint32, error) {
	for i := range count {
		if status, err := freeDynamic(env, first+8*i); err != nil || status != ssNormal {
			return status, err
		}
	}

	return ssNormal, nil
}

// libSget1DD is LIB$SGET1_DD (LIB-349): word-integer-length.rwu.r,
// descriptor-part.wq.r. Returns SS$_NORMAL or LIB$_INSVIRMEM.
func libSget1DD(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, err := env.Memory().LoadWord(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	return getDynamic(env, arg(argv, 1), uint32(length))
}

// libSfree1DD is LIB$SFREE1_DD (LIB-346): descriptor-address.mq.r.
func libSfree1DD(env *corevms.Environment, argv []uint32) (uint32, error) {
	return freeDynamic(env, arg(argv, 0))
}

// libSfreenDD is LIB$SFREEN_DD (LIB-348): number-of-descriptors.rlu.r,
// first-descriptor-array.mq.ra.
func libSfreenDD(env *corevms.Environment, argv []uint32) (uint32, error) {
	count, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	return freeDynamics(env, count, arg(argv, 1))
}

// jsbStatus runs a JSB routine's work and leaves its status in R0: the
// one register the dynamic-string and copy routines' JSB entries return
// (the survey's table).
func jsbStatus(regs *corevms.JSBRegisters, status uint32, err error) error {
	regs[registers.R0] = status

	return err
}

// libSget1DDR6 is LIB$SGET1_DD_R6: R0<15:0> the length, R1 the
// descriptor's address.
func libSget1DDR6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := getDynamic(env, regs[registers.R1], regs[registers.R0]&0xFFFF)

	return jsbStatus(regs, status, err)
}

// libSfree1DD6 is LIB$SFREE1_DD6: R0 the descriptor's address.
func libSfree1DD6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := freeDynamic(env, regs[registers.R0])

	return jsbStatus(regs, status, err)
}

// libSfreenDD6 is LIB$SFREEN_DD6: R0 the count (by value), R1 the first
// descriptor's address.
func libSfreenDD6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := freeDynamics(env, regs[registers.R0], regs[registers.R1])

	return jsbStatus(regs, status, err)
}
