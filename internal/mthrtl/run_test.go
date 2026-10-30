package mthrtl_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/mthrtl"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The helpers from a real program: what a routine does to its caller's
// registers, how a condition reaches the program's handler, and what comes
// back. The routines aren't written yet (subtask 3 on), so these tests put
// routines of their own, built on the helpers, at some entries.

// mthProgram calls each entry the routines below stand at, showing after
// each call the registers it returned and what its handler saw: the signal
// array's size and condition, and the user PC and the PC, each less the
// address of the instruction after the call (so both are 0 when right).
const mthProgram = `	.TITLE	MTHRUN
	.PSECT	DATA,WRT,NOEXE,LONG
X:	.FLOAT	1.0
BADF:	.LONG	^X8000
HRES:	.BLKL	4
EXPECT:	.BLKL	1
SCOUNT:	.BLKL	1
SCOND:	.BLKL	1
SUSER:	.BLKL	1
SPC:	.BLKL	1
CTL:	.ASCID	/!XL !XL !XL !XL !XL !XL !XL !XL n=!UL c=!XL u=!XL p=!XL/
BUF:	.BLKB	132
OUT:	.LONG	132
	.ADDRESS BUF
LEN:	.BLKL	1
LINE:	.BLKL	2
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	MOVL	(R0),R1
	MOVL	R1,SCOUNT
	MOVL	4(R0),SCOND
	MOVL	-4(R0)[R1],SPC
	SUBL2	EXPECT,SPC
	CLRL	SUSER
	CMPL	R1,#5
	BNEQ	10$
	SUBL3	EXPECT,12(R0),SUSER
10$:	MOVL	#1,R0
	RET
	; SHOW prints R0 to R7 and what the handler saw, for the line just done.
SHOW:	PUSHR	#^M<R0,R1,R2,R3,R4,R5,R6,R7>
	PUSHL	SPC
	PUSHL	SUSER
	PUSHL	SCOND
	PUSHL	SCOUNT
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHL	<7*4>+16(SP)
	PUSHAL	OUT
	PUSHAL	LEN
	PUSHAQ	CTL
	CALLS	#15,G^SYS$FAO
	MOVL	LEN,LINE
	MOVAB	BUF,LINE+4
	PUSHAQ	LINE
	CALLS	#1,G^LIB$PUT_OUTPUT
	CLRL	SCOUNT
	CLRL	SCOND
	CLRL	SUSER
	CLRL	SPC
	POPR	#^M<R0,R1,R2,R3,R4,R5,R6,R7>
	RSB
	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7>
	MOVAL	HANDLER,(FP)
	; 1. A CALL entry's condition, continued.
	MOVAB	A1,EXPECT
	MOVL	#^X12345678,R1
	PUSHAF	X
	CALLS	#1,G^MTH$SIN
A1:	BSBW	SHOW
	; 2. A reserved operand as argument.
	MOVAB	A2,EXPECT
	PUSHAL	BADF
	CALLS	#1,G^MTH$COS
A2:	BSBW	SHOW
	; 3. An argument it can't read.
	MOVAB	A3,EXPECT
	PUSHL	#^X3FFF0000
	CALLS	#1,G^MTH$TAN
A3:	BSBW	SHOW
	; 4. R0:R1 from a CALL entry.
	MOVAB	A4,EXPECT
	CALLS	#0,G^MTH$DSQRT
A4:	BSBW	SHOW
	; 5. R0:R3 from a CALL entry.
	MOVAB	A5,EXPECT
	CLRQ	R2
	CALLS	#0,G^OTS$POWHH_R3
A5:	BSBW	SHOW
	; 6. A result through the first argument.
	MOVAB	A6,EXPECT
	PUSHAL	HRES
	CALLS	#1,G^MTH$HSQRT
	MOVQ	HRES,R0
	MOVQ	HRES+8,R2
A6:	BSBW	SHOW
	; 7. A JSB entry's condition, continued.
	MOVAB	A7,EXPECT
	MOVF	X,R0
	MOVL	#^X12345678,R1
	JSB	G^MTH$SIN_R4
A7:	BSBW	SHOW
	; 8. A JSB entry's two H results, in R0:R7.
	MOVAB	A8,EXPECT
	JSB	G^MTH$HSINCOS_R7
A8:	BSBW	SHOW
	; 9. The caller's FU, clear and set, CALL then JSB.
	MOVAB	A9,EXPECT
	BICPSW	#^X40
	CALLS	#0,G^MTH$EXP
	MOVL	R0,R1
	JSB	G^MTH$EXP_R4
	MOVL	R0,R2
	BISPSW	#^X40
	CALLS	#0,G^MTH$EXP
	MOVL	R0,R3
	JSB	G^MTH$EXP_R4
	MOVL	R0,R4
	BICPSW	#^X40
	MOVL	R1,R0
	MOVL	R2,R1
	MOVL	R3,R2
	MOVL	R4,R3
A9:	BSBW	SHOW
	MOVL	#1,R0
	RET
	.END	START
`

// mthUnhandled calls an entry that signals, with no handler to take it.
const mthUnhandled = `	.TITLE	MTHEND
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	CALLS	#0,G^MTH$SIN
	CALLS	#0,G^MTH$COS
	RET
	.END	START
`

// boot is a console through vax.init, whose host directory, the current one,
// is a new temporary directory to hold the programs.
func boot(t *testing.T) (*console.Console, *console.Dispatcher, *bytes.Buffer) {
	t.Helper()
	t.Chdir(t.TempDir())

	c, buf := consoletest.New(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	consoletest.InstallDebugger(t, c, d)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	buf.Reset()

	return c, d, buf
}

// build assembles and links source as NAME.EXE in the current directory.
func build(t *testing.T, d *console.Dispatcher, out *bytes.Buffer, name, source string) {
	t.Helper()

	if err := os.WriteFile(name+".MAR", []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{"MACRO " + name, "LINK " + name} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, out)
		}
	}

	if out.Len() != 0 {
		t.Fatalf("MACRO and LINK said:\n%s", out)
	}
}

// oneAndAHalf is 1.5, rounded to f.
func oneAndAHalf(t *testing.T, f vaxfloat.Format) vaxfloat.Value {
	t.Helper()

	v, err := vaxfloat.Round(f, vaxfloat.FromFloat64(1.5))
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func TestHelpersFromAProgram(t *testing.T) {
	c, d, out := boot(t)
	shims := c.ProcessOne().Shims()
	condition := vmsdef.LibrarySymbols["MTH$_INVARGMAT"]

	// Each entry's routine, in the program's order.
	call := func(name string, fn corevms.ShimFunc) { shims.Register(mthrtl.CodeOf(name), name, fn) }
	jsb := func(name string, fn corevms.JSBShimFunc) { shims.RegisterJSB(mthrtl.CodeOf(name), name, fn) }

	call("MTH$SIN", func(env *corevms.Environment, _ []uint32) (uint32, error) {
		return mthrtl.SignalCondition(env, condition)
	})

	load := func(env *corevms.Environment, argv []uint32) (uint32, error) {
		if _, err := mthrtl.LoadArg(env, vaxfloat.F, argv[0]); err != nil {
			return mthrtl.Fail(env, err)
		}

		return 0x7777, nil
	}
	call("MTH$COS", load)
	call("MTH$TAN", load)

	call("MTH$DSQRT", func(env *corevms.Environment, _ []uint32) (uint32, error) {
		return mthrtl.ReturnFloat(env, vaxfloat.D, oneAndAHalf(t, vaxfloat.D)), nil
	})
	call("OTS$POWHH_R3", func(env *corevms.Environment, _ []uint32) (uint32, error) {
		return mthrtl.ReturnFloat(env, vaxfloat.H, oneAndAHalf(t, vaxfloat.H)), nil
	})
	call("MTH$HSQRT", func(env *corevms.Environment, argv []uint32) (uint32, error) {
		if err := mthrtl.StoreResult(env, vaxfloat.H, argv[0], oneAndAHalf(t, vaxfloat.H)); err != nil {
			return mthrtl.Fail(env, err)
		}

		return 0, nil
	})

	jsb("MTH$SIN_R4", func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		v, err := mthrtl.LoadRegisters(vaxfloat.F, regs[:], 0)
		if err != nil || v.Float64() != 1 {
			return fmt.Errorf("R0 is %v, %v", v, err)
		}

		_, err = mthrtl.SignalCondition(env, condition)

		return err
	})
	jsb("MTH$HSINCOS_R7", func(_ *corevms.Environment, regs *corevms.JSBRegisters) error {
		mthrtl.StoreRegisters(vaxfloat.H, regs, 0, oneAndAHalf(t, vaxfloat.H))
		mthrtl.StoreRegisters(vaxfloat.H, regs, 4, oneAndAHalf(t, vaxfloat.H).Neg())

		return nil
	})

	fu := func(env *corevms.Environment) uint32 {
		if set, err := env.CallerFU(); err != nil {
			t.Error(err)
		} else if set {
			return 1
		}

		return 0
	}
	call("MTH$EXP", func(env *corevms.Environment, _ []uint32) (uint32, error) { return fu(env), nil })
	jsb("MTH$EXP_R4", func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		regs[0] = fu(env)

		return nil
	})

	build(t, d, out, "MTHRUN", mthProgram)

	if err := d.Dispatch("RUN MTHRUN"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	// H_floating 1.5: the exponent word ^X4001, then the fraction's first
	// word ^X8000.
	const (
		h15      = "80004001 00000000 00000000 00000000"
		minusH15 = "8000C001 00000000 00000000 00000000"
		quiet    = "n=0 c=00000000 u=00000000 p=00000000"
	)

	signaled := fmt.Sprintf("n=5 c=%08X u=00000000 p=00000000", condition)

	// Each want is the start of the line's registers, and of what the
	// handler saw. The reserved operand comes back as R0 ^X8000, R1 0.
	wants := []struct{ registers, handler string }{
		{"00008000 00000000", signaled},
		{"00008000 00000000", "n=3 c=00000454 u=00000000 p=00000000"}, // SS$_ROPRAND: no FAO arguments
		{"0000000C", "n=5 c=0000000C"},                                // SS$_ACCVIO, continuing returns it
		{"000040C0 00000000", quiet},
		{h15, quiet},
		{h15, quiet},
		{"00008000 00000000", signaled},
		{h15 + " " + minusH15, quiet},
		{"00000000 00000000 00000001 00000001", quiet},
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(wants) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(wants), out)
	}

	for i, want := range wants {
		fields := strings.Fields(lines[i])
		registers, handler := strings.Join(fields[:8], " "), strings.Join(fields[8:], " ")

		if !strings.HasPrefix(registers, want.registers) || !strings.HasPrefix(handler, want.handler) {
			t.Errorf("step %d:\n  got %s\n want %s ... %s", i+1, lines[i], want.registers, want.handler)
		}
	}
}

// TestUnhandledCondition: with no handler, the message names the condition
// and gives the user PC, and the severe condition ends the image before
// its next call.
func TestUnhandledCondition(t *testing.T) {
	c, d, out := boot(t)
	condition := vmsdef.LibrarySymbols["MTH$_SQUROONEG"]

	var userPC uint32

	c.ProcessOne().Shims().Register(mthrtl.CodeOf("MTH$SIN"), "MTH$SIN", func(env *corevms.Environment, _ []uint32) (uint32, error) {
		userPC, _ = env.CallerPC()

		return mthrtl.SignalCondition(env, condition)
	})

	build(t, d, out, "MTHEND", mthUnhandled)

	_ = d.Dispatch("RUN MTHEND") // the image's end is what's looked at

	text := out.String()

	if !strings.Contains(text, "%MTH-F-SQUROONEG, ") || !strings.Contains(text, fmt.Sprintf("user PC %08X", userPC)) {
		t.Errorf("no message with user PC %08X; RUN said:\n%s", userPC, text)
	}

	if strings.Contains(text, "MTH$COS") {
		t.Errorf("the image went on after a severe condition:\n%s", text)
	}
}
