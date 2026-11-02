package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// The STR$ routines that take part of a string (docs/PHASE-83, subtask 9).
// Positions count from 1. A position off the string isn't a failure but an
// "alternate success": the routine uses the nearest one that makes sense and
// says so with STR$_ILLSTRPOS, and STR$_ILLSTRSPE when the two arguments don't
// fit each other. Each rule below is from VMS's run of testdata/str83, which the
// manual's words agree with as far as they go.
//
// Each routine is a function deciding the part, so the CALL entry (positions by
// reference) and the JSB entry (positions by value, in registers) share it. A
// part is a pair of offsets, s[from:to].

// part is the piece of the source a position routine chose, with its status.
type part struct {
	from, to int
	status   uint32
}

// posExtract is STR$POS_EXTR's rule: from start to end, both included.
func posExtract(n int, start, end int32) part {
	if start > end {
		return part{status: strIllStrSpe}
	}

	status := ssNormal

	if start < 1 {
		start, status = 1, strIllStrPos
	}

	if int(end) > n {
		end, status = int32(n), strIllStrPos
	}

	if start > end {
		return part{status: status}
	}

	return part{int(start) - 1, int(end), status}
}

// lenExtract is STR$LEN_EXTR's rule: length characters from start. That running
// past the end is STR$_ILLSTRSPE, over a start before the first's STR$_ILLSTRPOS
// (having both, unconfirmed).
func lenExtract(n int, start, length int32) part {
	if length < 0 {
		return part{status: strNegStrLen}
	}

	status := ssNormal

	if start < 1 {
		start, status = 1, strIllStrPos
	}

	if length == 0 {
		return part{status: status}
	}

	end := int64(start) + int64(length) - 1
	if end > int64(n) {
		end, status = int64(n), strIllStrSpe
	}

	if int64(start) > end {
		return part{status: status}
	}

	return part{int(start) - 1, int(end), status}
}

// leftPart is STR$LEFT's rule: through end. An end of 0 is an empty string with
// no complaint, and below that one with STR$_ILLSTRPOS.
func leftPart(n int, end int32) part {
	switch {
	case end < 0:
		return part{status: strIllStrPos}
	case int(end) > n:
		return part{0, n, strIllStrPos}
	}

	return part{0, int(end), ssNormal}
}

// rightPart is STR$RIGHT's rule: from start. The place just past the end is an
// empty string with no complaint, and further on one with STR$_ILLSTRPOS.
func rightPart(n int, start int32) part {
	switch {
	case start < 1:
		return part{0, n, strIllStrPos}
	case int(start) == n+1:
		return part{status: ssNormal}
	case int(start) > n+1:
		return part{status: strIllStrPos}
	}

	return part{int(start) - 1, n, ssNormal}
}

// strExtract stores what choose picks of the source at src in the destination.
func strExtract(env *corevms.Environment, dst, src uint32, regs *corevms.JSBRegisters, choose func(n int) part) (uint32, error) {
	s, text, err := strSource(env, src)
	if err != nil {
		return strFailure(env, err)
	}

	p := choose(len(s))

	return strWithStatus(env, dst, s[p.from:p.to], text+uint32(p.from), p.status, regs)
}

// signedAt reads a signed longword argument by reference.
func signedAt(env *corevms.Environment, addr uint32) (int32, error) {
	v, err := env.Memory().LoadLongword(env.CPU(), addr)

	return int32(v), err
}

// strPosExtr is STR$POS_EXTR: destination-string, source-string, start-position,
// end-position.
func strPosExtr(env *corevms.Environment, argv []uint32) (uint32, error) {
	start, err := signedAt(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	end, err := signedAt(env, strArg(env, argv, 3))
	if err != nil {
		return strFailure(env, err)
	}

	return strExtract(env, strArg(env, argv, 0), strArg(env, argv, 1), nil, func(n int) part { return posExtract(n, start, end) })
}

// strPosExtrR8 is STR$POS_EXTR_R8: R0 and R1 the strings, R2 and R3 the positions.
func strPosExtrR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	start, end := int32(regs[registers.R2]), int32(regs[registers.R3])
	status, err := strExtract(env, regs[registers.R0], regs[registers.R1], regs, func(n int) part { return posExtract(n, start, end) })

	return jsbStatus(regs, status, err)
}

// strLenExtr is STR$LEN_EXTR: destination-string, source-string, start-position,
// longword-integer-length.
func strLenExtr(env *corevms.Environment, argv []uint32) (uint32, error) {
	start, err := signedAt(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	length, err := signedAt(env, strArg(env, argv, 3))
	if err != nil {
		return strFailure(env, err)
	}

	return strExtract(env, strArg(env, argv, 0), strArg(env, argv, 1), nil, func(n int) part { return lenExtract(n, start, length) })
}

// strLenExtrR8 is STR$LEN_EXTR_R8: R0 and R1 the strings, R2 the start, R3 the length.
func strLenExtrR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	start, length := int32(regs[registers.R2]), int32(regs[registers.R3])
	status, err := strExtract(env, regs[registers.R0], regs[registers.R1], regs, func(n int) part { return lenExtract(n, start, length) })

	return jsbStatus(regs, status, err)
}

// strLeft is STR$LEFT: destination-string, source-string, end-position.
func strLeft(env *corevms.Environment, argv []uint32) (uint32, error) {
	end, err := signedAt(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	return strExtract(env, strArg(env, argv, 0), strArg(env, argv, 1), nil, func(n int) part { return leftPart(n, end) })
}

// strLeftR8 is STR$LEFT_R8: R0 and R1 the strings, R2 the end.
func strLeftR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	end := int32(regs[registers.R2])
	status, err := strExtract(env, regs[registers.R0], regs[registers.R1], regs, func(n int) part { return leftPart(n, end) })

	return jsbStatus(regs, status, err)
}

// strRight is STR$RIGHT: destination-string, source-string, start-position.
func strRight(env *corevms.Environment, argv []uint32) (uint32, error) {
	start, err := signedAt(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	return strExtract(env, strArg(env, argv, 0), strArg(env, argv, 1), nil, func(n int) part { return rightPart(n, start) })
}

// strRightR8 is STR$RIGHT_R8: R0 and R1 the strings, R2 the start.
func strRightR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	start := int32(regs[registers.R2])
	status, err := strExtract(env, regs[registers.R0], regs[registers.R1], regs, func(n int) part { return rightPart(n, start) })

	return jsbStatus(regs, status, err)
}

// replaced is STR$REPLACE's work: the source before start, the replacement, and
// the source after end. start past end is STR$_ILLSTRSPE, and the two pieces are
// still taken by those positions, so a character may appear in both (the probe's
// 3 and 2 of ABCDE make AB...CDE).
func replaced(env *corevms.Environment, dst, src uint32, start, end int32, with uint32) (uint32, error) {
	s, _, err := strSource(env, src)
	if err != nil {
		return strFailure(env, err)
	}

	replacement, _, err := strSource(env, with)
	if err != nil {
		return strFailure(env, err)
	}

	status := ssNormal
	if start > end {
		status = strIllStrSpe
	}

	if start < 1 {
		start = 1

		if status == ssNormal {
			status = strIllStrPos
		}
	}

	if int(end) > len(s) {
		end = int32(len(s))

		if status == ssNormal {
			status = strIllStrPos
		}
	}

	head := min(int(start)-1, len(s))
	tail := max(int(end), 0)

	return strWithStatus(env, dst, s[:head]+replacement+s[tail:], 0, status, nil)
}

// strReplace is STR$REPLACE: destination-string, source-string, start-position,
// end-position, replacement-string.
func strReplace(env *corevms.Environment, argv []uint32) (uint32, error) {
	start, err := signedAt(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	end, err := signedAt(env, strArg(env, argv, 3))
	if err != nil {
		return strFailure(env, err)
	}

	return replaced(env, strArg(env, argv, 0), strArg(env, argv, 1), start, end, strArg(env, argv, 4))
}

// strReplaceR8 is STR$REPLACE_R8: R0 and R1 the strings, R2 and R3 the positions,
// R4 the replacement. It changes only R0: what VMS's leaves in the others is its
// heap code's (expectedStr83Differences).
func strReplaceR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := replaced(env, regs[registers.R0], regs[registers.R1], int32(regs[registers.R2]), int32(regs[registers.R3]), regs[registers.R4])

	return jsbStatus(regs, status, err)
}
