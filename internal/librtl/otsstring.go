package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// OTS$'s moves and dynamic strings (Phase 72, subtask 6), from the VMS
// 5.0 OTS$ manual: OTS$MOVE3 and OTS$MOVE5 (OTS-49 to OTS-52), MOVC3 and
// MOVC5 with longword lengths; and OTS$SCOPY_DXDX, OTS$SCOPY_R_DX,
// OTS$SGET1_DD, OTS$SFREE1_DD, and OTS$SFREEN_DD (OTS-89 to OTS-98),
// LIB$'s dynamic-string routines (dynstr.go, copy.go) with their scalars
// by value and their errors signaled, not returned: OTS$_INVSTRDES,
// OTS$_INSVIRMEM, OTS$_FATINTERR. Each has a JSB entry.
//
// The copies' CALL entries return in R0 what MOVC5 leaves there, the
// number of source bytes not copied (truncation is no error), and their
// JSB entries leave R0 to R5 as MOVC5 does (copyMoved): R0 that number,
// R1 one byte beyond the source bytes copied, R2 0, R3 one byte beyond
// the destination, R4 and R5 0. The JSB entries take their arguments in
// R0, R1, and R2, in the CALL entry's order, a length in R0<15:0>.
//
// Unconfirmed (the manual doesn't say; testdata/ots72 asks VMS): the
// MOVE routines' JSB entries' input registers (MOVE3_R5 as MOVC3 holds
// them, R0 the length, R1 the source, R3 the destination; MOVE5_R5 in
// MOVC5's operand order, R0 to R4); a negative length (0); what the
// routines that return nothing leave in R0 (MOVE3 0, as MOVC3 does;
// MOVE5 MOVC5's R0; SGET1, SFREE1, and SFREEN SS$_NORMAL); that the
// copies' JSB entries leave R6 alone; and the condition SFREE1_DD
// signals for a pointer that isn't a dynamic string's (OTS$_FATINTERR,
// the one the manual lists; LIB$SFREE1_DD returns STR$_ERRFREDYN).

var (
	otsInsVirMem   = vmsdef.LibrarySymbols["OTS$_INSVIRMEM"]
	otsFatIntErr   = vmsdef.LibrarySymbols["OTS$_FATINTERR"]
	otsStatusOfLIB = map[uint32]uint32{libInvStrDes: otsInvStrDes, libInsVirMem: otsInsVirMem}
)

// moveLong is the MOVE routines' move: srcLen bytes from src, then fill,
// to make dstLen bytes at dst, as MOVC5 moves them (the source read
// before the destination is written, so overlap doesn't matter). It
// returns the bytes moved from the source.
func moveLong(env *corevms.Environment, srcLen, src uint32, fill byte, dstLen, dst uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	moved := min(srcLen, dstLen)
	buf := make([]byte, dstLen)

	for i := range moved {
		b, err := mem.LoadByte(cpu, src+i)
		if err != nil {
			return 0, err
		}

		buf[i] = b
	}

	for i := moved; i < dstLen; i++ {
		buf[i] = fill
	}

	for i, b := range buf {
		if err := mem.StoreByte(cpu, dst+uint32(i), b); err != nil {
			return 0, writing(err)
		}
	}

	return moved, nil
}

// length is a MOVE routine's length: a signed longword, 0 when negative.
func length(v uint32) uint32 { return uint32(max(int32(v), 0)) }

func otsMove3(env *corevms.Environment, argv []uint32) (uint32, error) {
	n := length(arg(argv, 0))

	if _, err := moveLong(env, n, arg(argv, 1), 0, n, arg(argv, 2)); err != nil {
		return memoryFault(env, err)
	}

	return 0, nil
}

func otsMove5(env *corevms.Environment, argv []uint32) (uint32, error) {
	srcLen, dstLen := length(arg(argv, 0)), length(arg(argv, 3))

	moved, err := moveLong(env, srcLen, arg(argv, 1), byte(arg(argv, 2)), dstLen, arg(argv, 4))
	if err != nil {
		return memoryFault(env, err)
	}

	return srcLen - moved, nil
}

// otsMove3R5 is OTS$MOVE3_R5: R0 the length, R1 the source, R3 the
// destination (unconfirmed), leaving the registers as MOVC3 does.
func otsMove3R5(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	n, src, dst := length(regs[registers.R0]), regs[registers.R1], regs[registers.R3]

	if _, err := moveLong(env, n, src, 0, n, dst); err != nil {
		_, err = memoryFault(env, err)

		return err
	}

	*regs = corevms.JSBRegisters{0, src + n, 0, dst + n, 0, 0, regs[registers.R6]}

	return nil
}

// otsMove5R5 is OTS$MOVE5_R5: R0 to R4 the source length, source, fill,
// destination length, and destination (unconfirmed), leaving the
// registers as MOVC5 does.
func otsMove5R5(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	srcLen, src, dstLen, dst := length(regs[registers.R0]), regs[registers.R1], length(regs[registers.R3]), regs[registers.R4]

	moved, err := moveLong(env, srcLen, src, byte(regs[registers.R2]), dstLen, dst)
	if err != nil {
		_, err = memoryFault(env, err)

		return err
	}

	*regs = corevms.JSBRegisters{srcLen - moved, src + moved, 0, dst + dstLen, 0, 0, regs[registers.R6]}

	return nil
}

// otsSignal turns a LIB$ string helper's status into OTS$'s rules:
// success and truncation are ok; LIB$_INVSTRDES and LIB$_INSVIRMEM are
// signaled as OTS$_INVSTRDES and OTS$_INSVIRMEM, and STR$_ERRFREDYN as
// OTS$_FATINTERR; anything else was signaled already (an access
// violation, memoryFault's), and r0 is what that left.
func otsSignal(env *corevms.Environment, status uint32, err error) (r0 uint32, ok bool, _ error) {
	if err != nil {
		return 0, false, err
	}

	cond, bad := otsStatusOfLIB[status]
	if status == strErrFreDyn {
		cond, bad = otsFatIntErr, true
	}

	switch {
	case bad:
		r0, err := env.SignalFailure([]uint32{cond}, cond)

		return r0, false, err
	case status == ssNormal || status == libStrTru:
		return 0, true, nil
	}

	return status, false, nil
}

// otsCopyText is the copies' work: s, whose bytes are at src, into the
// destination string at dst, returning the bytes not copied. regs, when
// it isn't nil, gets MOVC5's registers (R6 left as it was).
func otsCopyText(env *corevms.Environment, s string, src, dst uint32, regs *corevms.JSBRegisters) (uint32, error) {
	var work corevms.JSBRegisters

	status, moved, err := copyMoved(env, s, src, dst, &work)
	if r0, ok, err := otsSignal(env, status, err); !ok {
		return r0, err
	}

	unmoved := uint32(len(s) - moved)

	if regs != nil {
		r6 := regs[registers.R6]
		*regs = work
		regs[registers.R0], regs[registers.R6] = unmoved, r6
	}

	return unmoved, nil
}

// otsCopyDescriptor is OTS$SCOPY_DXDX's work: the source string, of any
// input class, into the destination.
func otsCopyDescriptor(env *corevms.Environment, src, dst uint32, regs *corevms.JSBRegisters) (uint32, error) {
	d, err := readDescriptor(env, src)
	if err != nil {
		status, err := stringFailure(env, err)
		r0, _, err := otsSignal(env, status, err)

		return r0, err
	}

	s, err := readText(env, d.text, d.length)
	if err != nil {
		return memoryFault(env, err)
	}

	return otsCopyText(env, s, d.text, dst, regs)
}

// otsCopyReference is OTS$SCOPY_R_DX's work: length bytes at src into
// the destination.
func otsCopyReference(env *corevms.Environment, length, src, dst uint32, regs *corevms.JSBRegisters) (uint32, error) {
	s, err := readText(env, src, length)
	if err != nil {
		return memoryFault(env, err)
	}

	return otsCopyText(env, s, src, dst, regs)
}

// otsScopyDxdx is OTS$SCOPY_DXDX (OTS-89): source-string.rt.dx,
// destination-string.wt.dx.
func otsScopyDxdx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return otsCopyDescriptor(env, arg(argv, 0), arg(argv, 1), nil)
}

// otsScopyRDx is OTS$SCOPY_R_DX (OTS-91): word-int-source-length.rwu.v,
// source-string-address.rt.r, destination-string.wt.dx.
func otsScopyRDx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return otsCopyReference(env, arg(argv, 0)&0xFFFF, arg(argv, 1), arg(argv, 2), nil)
}

// otsScopyDxdx6 is OTS$SCOPY_DXDX6: R0 the source descriptor's address,
// R1 the destination's.
func otsScopyDxdx6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	_, err := otsCopyDescriptor(env, regs[registers.R0], regs[registers.R1], regs)

	return err
}

// otsScopyRDx6 is OTS$SCOPY_R_DX6: R0<15:0> the length, R1 the source's
// address, R2 the destination descriptor's.
func otsScopyRDx6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	_, err := otsCopyReference(env, regs[registers.R0]&0xFFFF, regs[registers.R1], regs[registers.R2], regs)

	return err
}

// otsDynamic finishes a dynamic-string routine: its helper's status by
// otsSignal's rules, SS$_NORMAL in R0 on success.
func otsDynamic(env *corevms.Environment, status uint32, err error) (uint32, error) {
	if r0, ok, err := otsSignal(env, status, err); !ok {
		return r0, err
	}

	return ssNormal, nil
}

// otsSget1DD is OTS$SGET1_DD (OTS-96): word-integer-length.rwu.v,
// dynamic-descriptor.mq.r.
func otsSget1DD(env *corevms.Environment, argv []uint32) (uint32, error) {
	status, err := getDynamic(env, arg(argv, 1), arg(argv, 0)&0xFFFF)

	return otsDynamic(env, status, err)
}

// otsSfree1DD is OTS$SFREE1_DD (OTS-94): dynamic-descriptor.mq.r.
func otsSfree1DD(env *corevms.Environment, argv []uint32) (uint32, error) {
	status, err := freeDynamic(env, arg(argv, 0))

	return otsDynamic(env, status, err)
}

// otsSfreenDD is OTS$SFREEN_DD (OTS-95): descriptor-count.rlu.v,
// first-descriptor.mq.r.
func otsSfreenDD(env *corevms.Environment, argv []uint32) (uint32, error) {
	status, err := freeDynamics(env, arg(argv, 0), arg(argv, 1))

	return otsDynamic(env, status, err)
}

// otsSget1DDR6 is OTS$SGET1_DD_R6: R0<15:0> the length, R1 the
// descriptor's address; the status in R0.
func otsSget1DDR6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := getDynamic(env, regs[registers.R1], regs[registers.R0]&0xFFFF)
	regs[registers.R0], err = otsDynamic(env, status, err)

	return err
}

// otsSfree1DD6 is OTS$SFREE1_DD6: R0 the descriptor's address.
func otsSfree1DD6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := freeDynamic(env, regs[registers.R0])
	regs[registers.R0], err = otsDynamic(env, status, err)

	return err
}

// otsSfreenDD6 is OTS$SFREEN_DD6: R0 the count, R1 the first
// descriptor's address.
func otsSfreenDD6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := freeDynamics(env, regs[registers.R0], regs[registers.R1])
	regs[registers.R0], err = otsDynamic(env, status, err)

	return err
}
