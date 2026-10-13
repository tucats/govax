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
	status, _, err := copyMoved(env, s, 0, dst, nil)

	return status, err
}

// copyMoved is copyString, also setting regs (when it isn't nil) as a
// MOVC5 of the source text at src into the destination would leave them:
// R1 one byte beyond the source bytes moved, R2 0, R3 one byte beyond the
// destination, R4 and R5 0. That's what VMS's copy routines' JSB entries
// leave (the probe, testdata/lib54; the OTS$ manual says so of its own
// copy routines), with the status in R6 as well as R0. moved is how many
// bytes were.
func copyMoved(env *corevms.Environment, s string, src, dst uint32, regs *corevms.JSBRegisters) (uint32, int, error) {
	moved, truncated, end, err := writeOut(env, dst, s, ' ', true)
	if err != nil {
		status, err := stringFailure(env, err)

		return status, 0, err
	}

	status := ssNormal
	if truncated {
		status = libStrTru
	}

	if regs != nil {
		regs[registers.R1] = src + uint32(moved)
		regs[registers.R2], regs[registers.R3] = 0, end
		regs[registers.R4], regs[registers.R5] = 0, 0
		regs[registers.R6] = status
	}

	return status, moved, nil
}

// copyDescriptor is LIB$SCOPY_DXDX's work: the source string, of any
// input class, into the destination.
func copyDescriptor(env *corevms.Environment, src, dst uint32, regs *corevms.JSBRegisters) (uint32, error) {
	d, err := readDescriptor(env, src)
	if err != nil {
		return stringFailure(env, err)
	}

	s, err := readText(env, d.text, d.length)
	if err != nil {
		return memoryFault(env, err)
	}

	status, _, err := copyMoved(env, s, d.text, dst, regs)

	return status, err
}

// copyReference is LIB$SCOPY_R_DX's work: length bytes at src into the
// destination.
func copyReference(env *corevms.Environment, length, src, dst uint32, regs *corevms.JSBRegisters) (uint32, error) {
	s, err := readText(env, src, length)
	if err != nil {
		return memoryFault(env, err)
	}

	status, _, err := copyMoved(env, s, src, dst, regs)

	return status, err
}

// libScopyDxdx is LIB$SCOPY_DXDX (LIB-335): source-string.rt.dx,
// destination-string.wt.dx.
func libScopyDxdx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return copyDescriptor(env, arg(argv, 0), arg(argv, 1), nil)
}

// libScopyRDx is LIB$SCOPY_R_DX (LIB-337): word-integer-source-length.rwu.r,
// source-string-address.rt.r, destination-string.wt.dx.
func libScopyRDx(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, err := env.Memory().LoadWord(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	return copyReference(env, uint32(length), arg(argv, 1), arg(argv, 2), nil)
}

// libScopyDxdx6 is LIB$SCOPY_DXDX6: R0 the source descriptor's address,
// R1 the destination's. It returns the status in R0 and R6, and R1 to R5
// as MOVC5 leaves them (copyMoved).
func libScopyDxdx6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := copyDescriptor(env, regs[registers.R0], regs[registers.R1], regs)

	return jsbStatus(regs, status, err)
}

// libScopyRDx6 is LIB$SCOPY_R_DX6: R0<15:0> the length (by value), R1
// the source's address, R2 the destination descriptor's. Its registers
// are LIB$SCOPY_DXDX6's.
func libScopyRDx6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := copyReference(env, regs[registers.R0]&0xFFFF, regs[registers.R1], regs[registers.R2], regs)

	return jsbStatus(regs, status, err)
}

// analyze is LIB$ANALYZE_SDESC's work: the length and the text's address
// of the string the descriptor at addr describes, by its class (an
// array's ARSIZE, a varying string's CURLEN and the text after it), or
// LIB$_INVSTRDES as status for a class that isn't a string's or an
// array of 65,536 bytes or more. With LIB$_INVSTRDES, text is still the
// descriptor's DSC$A_POINTER and length 0, which VMS's routine returns
// too (the probe, testdata/lib54).
func analyze(env *corevms.Environment, addr uint32) (status, length, text uint32, err error) {
	d, err := readDescriptor(env, addr)
	if err != nil {
		status, err = stringFailure(env, err)

		return status, 0, d.text, err
	}

	return ssNormal, d.length, d.text, nil
}

// libAnalyzeSdesc is LIB$ANALYZE_SDESC (LIB-9): input-descriptor.rt.dx,
// data-length.wwu.r, data-address.wa.r. For an invalid descriptor it
// returns LIB$_INVSTRDES, writing a length of 0 and the descriptor's
// pointer, as VMS does.
func libAnalyzeSdesc(env *corevms.Environment, argv []uint32) (uint32, error) {
	status, length, text, err := analyze(env, arg(argv, 0))
	if err != nil {
		return status, err
	}

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreWord(cpu, arg(argv, 1), uint16(length)); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, arg(argv, 2), text); err != nil {
		return memoryFault(env, writing(err))
	}

	return status, nil
}

// libAnalyzeSdescR2 is LIB$ANALYZE_SDESC_R2: R0 the descriptor's
// address; it returns the status in R0, the length in R1, and the text's
// address in R2 (the survey's reading, which VMS's run confirmed). For
// an invalid descriptor it leaves R1 alone and returns the descriptor's
// pointer in R2, as VMS does.
func libAnalyzeSdescR2(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, length, text, err := analyze(env, regs[registers.R0])
	if err == nil {
		if status == ssNormal {
			regs[registers.R1] = length
		}

		regs[registers.R2] = text
	}

	return jsbStatus(regs, status, err)
}
