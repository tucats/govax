package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/registers"
)

// EXE$EXCMSG's dump (docs/PHASE-70 - traceback.md, subtask 7).
//
// When the catch-all ends an image for a condition no handler took, and
// the condition is one of the hardware exceptions' (an access violation,
// a reserved operand, an arithmetic trap, ...), VMS writes "a formatted
// dump of the general registers, the signal array, and the stack" after
// the message (Internals book, 30.2.3):
//
//	%SYSTEM-F-ACCVIO, access violation, reason mask=00, ...
//
//	  Improperly handled condition, image exit forced.
//
//		Signal arguments	      Stack contents
//
//		Number = 00000005		 00000000
//		Name   = 0000000C		 2FFC0000
//				 00000000		 7FED5398
//		...
//						 7FED5384
//
//		Register dump
//
//		R0 = 10101010  R1 = 11111111  R2 = 22222222  R3 = 33333333
//		...
//		PSL= 03C00008
//
// In an image linked with traceback the traceback handler takes such a
// condition first, so this is the dump of an image linked /NOTRACEBACK,
// and of a condition raised while the image's exit handlers run, which
// the traceback handler no longer covers (testdata/tb70's part X).
//
// What decides it is the condition value, however it was raised:
// SS$_ACCVIO and SS$_OPCDEC signaled by LIB$SIGNAL, and SS$_ACCVIO by
// LIB$STOP, get the dump; SS$_BADPARAM, with a signal array as long as
// an access violation's, and every other LIB$STOP don't (testdata/tb70's
// rounds 1 and 2). Only ACCVIO, OPCDEC, INTOVF, INTDIV, and ROPRAND are
// confirmed; govax gives it for every condition its exception
// dispatcher raises.

// exceptionConditions are the conditions DispatchException raises
// (exceptionCondition's), by their condition IDs (severity and control
// bits left out).
var exceptionConditions = func() map[uint32]bool {
	m := map[uint32]bool{
		ssAccVioCond & condIDMask: true,
		ssArtRes & condIDMask:     true,
	}

	for _, cond := range simpleExceptions {
		m[cond&condIDMask] = true
	}

	for _, cond := range arithmeticConditions {
		m[cond&condIDMask] = true
	}

	return m
}()

// excmsgStackLongwords is how many longwords of the stack the dump
// shows, from the program's SP.
const excmsgStackLongwords = 10

// improperlyHandledDump returns the lines of EXE$EXCMSG's dump for d,
// whose signal array words holds (sigWords: the condition, its detail,
// PC, and PSL), to follow the condition's message.
//
// The signal array (its count first) and ten longwords of the stack are
// shown side by side, as many lines as the longer has; then the
// registers. R0 and R1 are the mechanism array's, R2 to R11 the
// program's, and PC and PSL the signal array's. SP is the program's as
// the condition leaves it (conditionSP). FP and AP aren't the
// program's but those of VMS's own frames below the condition's arrays:
// in all seven of the probe's dumps FP is SP less 68 and four bytes per
// signal array longword, and AP is FP+40. govax shows the same values.
//
// A stack longword that can't be read ends that column (unconfirmed:
// the probe's stacks were all readable), as does a signal array
// longer than ten longwords (unconfirmed: none was).
func (env *Environment) improperlyHandledDump(d *conditionDispatch, words []uint32) []string {
	c := env.cpu
	n := uint32(len(words))
	sp := d.conditionSP

	var stack []uint32

	for i := range uint32(excmsgStackLongwords) {
		v, err := env.mem.LoadLongword(c, sp+4*i)
		if err != nil {
			break
		}

		stack = append(stack, v)
	}

	lines := []string{
		"",
		"  Improperly handled condition, image exit forced.",
		"",
		"\tSignal arguments\t      Stack contents",
		"",
	}

	// The signal array's column: its count, the condition ("Name"), and
	// the rest of it.
	sigColumn := []string{
		fmt.Sprintf("\tNumber = %08X", n),
		fmt.Sprintf("\tName   = %08X", words[0]),
	}

	for _, w := range words[1:] {
		sigColumn = append(sigColumn, fmt.Sprintf("\t\t %08X", w))
	}

	for i := range max(len(sigColumn), len(stack)) {
		left := "\t\t\t"
		if i < len(sigColumn) {
			left = sigColumn[i]
		}

		if i < len(stack) {
			lines = append(lines, fmt.Sprintf("%s\t\t %08X", left, stack[i]))
		} else {
			lines = append(lines, left)
		}
	}

	r0, _ := env.mem.LoadLongword(c, d.mech+12)
	r1, _ := env.mem.LoadLongword(c, d.mech+16)
	fp := sp - (0x68 + 4*n)

	regs := []uint32{r0, r1}
	for r := registers.R2; r <= registers.R11; r++ {
		regs = append(regs, c.GPR(r))
	}

	regs = append(regs, fp+0x40, fp, sp, words[n-2])

	names := []string{
		"R0 ", "R1 ", "R2 ", "R3 ", "R4 ", "R5 ", "R6 ", "R7 ",
		"R8 ", "R9 ", "R10", "R11", "AP ", "FP ", "SP ", "PC ",
	}

	lines = append(lines, "", "\tRegister dump", "")

	for row := 0; row < len(regs); row += 4 {
		line := "\t"

		for i := row; i < row+4; i++ {
			if i > row {
				line += "  "
			}

			line += fmt.Sprintf("%s= %08X", names[i], regs[i])
		}

		lines = append(lines, line)
	}

	return append(lines, fmt.Sprintf("\tPSL= %08X", words[n-1]), "")
}

// spAfterReturn is the SP a LIB$SIGNAL or LIB$STOP caller has once the
// call returns, as the "improperly handled condition" dump shows it
// (testdata/tb70's round 2, parts O and Q). fp is the stub's frame and
// maskPSW its mask and PSW longword; sp, for a routine reached by JSB
// (which has no frame of its own), is where the JSB's return address
// is, which its RSB pops.
//
// A RET leaves SP where it was before the call (handlerArgList), then a
// CALLS's (bit 29) pops its argument list: the count longword and the
// arguments it counts.
func (env *Environment) spAfterReturn(fp, maskPSW, sp uint32) uint32 {
	if env.jsbCall {
		return sp + 4
	}

	argList, ok := env.handlerArgList(fp)
	if !ok || maskPSW&(1<<29) == 0 {
		return argList
	}

	count, err := env.mem.LoadByte(env.cpu, argList)
	if err != nil {
		return argList
	}

	return argList + 4 + 4*uint32(count)
}
