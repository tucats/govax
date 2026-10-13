package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// Copying strings and analyzing descriptors (docs/PHASE-54, "Copying and
// descriptors"): LIB$SCOPY_DXDX, LIB$SCOPY_R_DX, LIB$ANALYZE_SDESC, and
// their JSB entries. The copies write the destination by its class, as
// every LIB$ output string is written (writeString, whose rules are
// LIB$SCOPY_DXDX's own table), and ANALYZE_SDESC is readDescriptor's
// class rules as a routine.

// copyString is the copy routines' work: s into the destination string
// described at dst. It returns SS$_NORMAL, LIB$_STRTRU when a fixed or
// varying destination couldn't hold it all, LIB$_INVSTRDES, or
// LIB$_INSVIRMEM.
func copyString(env *corevms.Environment, s string, dst uint32) (uint32, error) {
	_, truncated, err := writeString(env, dst, s)
	if err != nil {
		return stringFailure(env, err)
	}

	if truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}

// copyDescriptor is LIB$SCOPY_DXDX's work: the source string, of any
// input class, into the destination.
func copyDescriptor(env *corevms.Environment, src, dst uint32) (uint32, error) {
	s, err := readString(env, src)
	if err != nil {
		return stringFailure(env, err)
	}

	return copyString(env, s, dst)
}

// copyReference is LIB$SCOPY_R_DX's work: length bytes at src into the
// destination.
func copyReference(env *corevms.Environment, length, src, dst uint32) (uint32, error) {
	s, err := readText(env, src, length)
	if err != nil {
		return memoryFault(env, err)
	}

	return copyString(env, s, dst)
}

// libScopyDxdx is LIB$SCOPY_DXDX (LIB-335): source-string.rt.dx,
// destination-string.wt.dx.
func libScopyDxdx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return copyDescriptor(env, arg(argv, 0), arg(argv, 1))
}

// libScopyRDx is LIB$SCOPY_R_DX (LIB-337): word-integer-source-length.rwu.r,
// source-string-address.rt.r, destination-string.wt.dx.
func libScopyRDx(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, err := env.Memory().LoadWord(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	return copyReference(env, uint32(length), arg(argv, 1), arg(argv, 2))
}

// libScopyDxdx6 is LIB$SCOPY_DXDX6: R0 the source descriptor's address,
// R1 the destination's.
func libScopyDxdx6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := copyDescriptor(env, regs[registers.R0], regs[registers.R1])

	return jsbStatus(regs, status, err)
}

// libScopyRDx6 is LIB$SCOPY_R_DX6: R0<15:0> the length (by value), R1
// the source's address, R2 the destination descriptor's.
func libScopyRDx6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := copyReference(env, regs[registers.R0]&0xFFFF, regs[registers.R1], regs[registers.R2])

	return jsbStatus(regs, status, err)
}

// analyze is LIB$ANALYZE_SDESC's work: the length and the text's address
// of the string the descriptor at addr describes, by its class (an
// array's ARSIZE, a varying string's CURLEN and the text after it), or
// LIB$_INVSTRDES as status for a class that isn't a string's or an
// array of 65,536 bytes or more.
func analyze(env *corevms.Environment, addr uint32) (status, length, text uint32, err error) {
	d, err := readDescriptor(env, addr)
	if err != nil {
		status, err = stringFailure(env, err)

		return status, 0, 0, err
	}

	return ssNormal, d.length, d.text, nil
}

// libAnalyzeSdesc is LIB$ANALYZE_SDESC (LIB-9): input-descriptor.rt.dx,
// data-length.wwu.r, data-address.wa.r. Nothing is written for an
// invalid descriptor.
func libAnalyzeSdesc(env *corevms.Environment, argv []uint32) (uint32, error) {
	status, length, text, err := analyze(env, arg(argv, 0))
	if err != nil || status != ssNormal {
		return status, err
	}

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreWord(cpu, arg(argv, 1), uint16(length)); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, arg(argv, 2), text); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libAnalyzeSdescR2 is LIB$ANALYZE_SDESC_R2: R0 the descriptor's
// address; it returns the status in R0, the length in R1, and the text's
// address in R2 (the survey's reading, unconfirmed). R1 and R2 are left
// alone for an invalid descriptor.
func libAnalyzeSdescR2(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, length, text, err := analyze(env, regs[registers.R0])
	if err == nil && status == ssNormal {
		regs[registers.R1], regs[registers.R2] = length, text
	}

	return jsbStatus(regs, status, err)
}
