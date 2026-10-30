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

// The routines from a real program: every entry of the square root,
// exponential, logarithm, trigonometric, and inverse trigonometric families,
// CALL and JSB, for answers, each condition, and a handler that continues from
// it. MTH$SINCOS and MTH$SINCOSD return two results, and have a program of
// their own (TestPairsFromAProgram).

// entryCase is one call in the generated program, by what's looked at: the
// argument's text as a MACRO floating literal (or empty for a reserved
// operand), whether the caller has FU set, and what should come back.
type entryCase struct {
	entry     string
	format    vaxfloat.Format
	arg       string
	binary    bool // the routine takes two arguments, the second arg2 (empty: a reserved operand)
	arg2      string
	fu        bool
	want      vaxfloat.Value
	reserved  bool   // the result is the reserved operand, not want
	condition uint32 // what the handler should see, or 0
}

// entryFamily is a family's entries in format order, F, D, G, H. A format's
// JSB names may be several (a second entry), and LOG2's are none.
type entryFamily struct {
	call [4]string
	jsb  [4][]string
}

var entryFamilies = map[string]entryFamily{
	"SQRT": {
		[4]string{"MTH$SQRT", "MTH$DSQRT", "MTH$GSQRT", "MTH$HSQRT"},
		[4][]string{{"MTH$SQRT_R3"}, {"MTH$DSQRT_R5"}, {"MTH$GSQRT_R5"}, {"MTH$HSQRT_R8"}},
	},
	"EXP": {
		[4]string{"MTH$EXP", "MTH$DEXP", "MTH$GEXP", "MTH$HEXP"},
		[4][]string{{"MTH$EXP_R4"}, {"MTH$DEXP_R6", "MTH$DEXP_R7"}, {"MTH$GEXP_R6", "MTH$GEXP_R7"}, {"MTH$HEXP_R6"}},
	},
	"LOG": {
		[4]string{"MTH$ALOG", "MTH$DLOG", "MTH$GLOG", "MTH$HLOG"},
		[4][]string{{"MTH$ALOG_R5"}, {"MTH$DLOG_R8"}, {"MTH$GLOG_R8"}, {"MTH$HLOG_R8"}},
	},
	"LOG2": {
		[4]string{"MTH$ALOG2", "MTH$DLOG2", "MTH$GLOG2", "MTH$HLOG2"},
		[4][]string{},
	},
	"LOG10": {
		[4]string{"MTH$ALOG10", "MTH$DLOG10", "MTH$GLOG10", "MTH$HLOG10"},
		[4][]string{{"MTH$ALOG10_R5"}, {"MTH$DLOG10_R8"}, {"MTH$GLOG10_R8"}, {"MTH$HLOG10_R8"}},
	},
	"SIN":   trigFamily("SIN", "4", "7", "5"),
	"COS":   trigFamily("COS", "4", "7", "5"),
	"TAN":   trigFamily("TAN", "4 5", "7", "5"),
	"SIND":  trigFamily("SIND", "4", "7", "5"),
	"COSD":  trigFamily("COSD", "4", "7", "5"),
	"TAND":  trigFamily("TAND", "4 5", "7", "5"),
	"ASIN":  trigFamily("ASIN", "4 5", "7 9", "8"),
	"ACOS":  trigFamily("ACOS", "4 5", "7 9", "8"),
	"ATAN":  trigFamily("ATAN", "4", "7", "8"),
	"ASIND": trigFamily("ASIND", "4", "7", "8"),
	"ACOSD": trigFamily("ACOSD", "4", "7", "8"),
	"ATAND": trigFamily("ATAND", "4", "7", "8"),

	// These have JSB entries too, which no manual gives (Decision 7).
	"ATAN2":  trigFamily("ATAN2", "", "", ""),
	"ATAND2": trigFamily("ATAND2", "", "", ""),
	"SINH":   trigFamily("SINH", "", "", ""),
	"COSH":   trigFamily("COSH", "", "", ""),
	"TANH":   trigFamily("TANH", "", "", ""),
	"ATANH":  trigFamily("ATANH", "", "", ""),
}

// trigFamily spells a family's entries, whose names follow one rule: MTH$xNAME
// with x empty, D, G, or H, and each JSB entry _Rn for the n's given, for F, for
// D and G alike, and for H.
func trigFamily(stem, f, dg, h string) entryFamily {
	var family entryFamily

	for i, prefix := range []string{"", "D", "G", "H"} {
		family.call[i] = "MTH$" + prefix + stem

		numbers := []string{f, dg, dg, h}[i]
		for _, n := range strings.Fields(numbers) {
			family.jsb[i] = append(family.jsb[i], "MTH$"+prefix+stem+"_R"+n)
		}
	}

	return family
}

// tinyArgument is a value of format f as a MACRO floating literal, about the
// format's smallest, so that its sine in degrees is below the format's range.
func tinyArgument(f vaxfloat.Format) string {
	switch f {
	case vaxfloat.G:
		return "6.0E-309"
	case vaxfloat.H:
		return "9.0E-4933"
	default:
		return "3.0E-39"
	}
}

// entryCases makes the program's calls: each row below for each family it
// names, in each format, by each of the family's entries.
func entryCases(t *testing.T) []entryCase {
	t.Helper()

	symbol := func(name string) uint32 { return vmsdef.LibrarySymbols[name] }
	number := func(f vaxfloat.Format, s string) vaxfloat.Value {
		x, err := vaxfloat.Parse(s)
		if err != nil {
			t.Fatal(err)
		}

		v, err := vaxfloat.Round(f, x)
		if err != nil {
			t.Fatal(err)
		}

		return v
	}

	rows := []struct {
		family    string
		arg       string
		binary    bool
		arg2      string
		tiny      bool // the argument is tinyArgument's
		fu        bool
		want      string // empty: the reserved operand, or 0 when zero is set
		zero      bool
		condition uint32
	}{
		{family: "SQRT", arg: "2.25", want: "1.5"},
		{family: "SQRT", arg: "0.0", want: "0"},
		{family: "SQRT", arg: "-1.0", condition: symbol("MTH$_SQUROONEG")},
		{family: "SQRT", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "EXP", arg: "0.0", want: "1"},
		{family: "EXP", arg: "1.0", want: "2.718281828459045235360287471352662497757247"},
		{family: "EXP", arg: "20000.0", condition: symbol("MTH$_FLOOVEMAT")},
		{family: "EXP", arg: "-20000.0", zero: true},
		{family: "EXP", arg: "-20000.0", fu: true, zero: true, condition: symbol("MTH$_FLOUNDMAT")},
		{family: "EXP", arg: "1.0", fu: true, want: "2.718281828459045235360287471352662497757247"},
		{family: "EXP", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "LOG", arg: "1.0", want: "0"},
		{family: "LOG", arg: "10.0", want: "2.302585092994045684017991454684364207601101"},
		{family: "LOG", arg: "0.0", condition: symbol("MTH$_LOGZERNEG")},
		{family: "LOG", arg: "-2.0", condition: symbol("MTH$_LOGZERNEG")},
		{family: "LOG", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "LOG2", arg: "8.0", want: "3"},
		{family: "LOG2", arg: "0.25", want: "-2"},
		{family: "LOG2", arg: "0.0", condition: symbol("MTH$_LOGZERNEG")},
		{family: "LOG2", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "LOG10", arg: "1000.0", want: "3"},
		{family: "LOG10", arg: "0.0", condition: symbol("MTH$_LOGZERNEG")},
		{family: "LOG10", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "SIN", arg: "0.0", want: "0"},
		{family: "SIN", arg: "1.0", want: "0.841470984807896506652502321630298999622"},
		{family: "COS", arg: "0.0", want: "1"},
		{family: "COS", arg: "1.0", want: "0.540302305868139717400936607442976603732"},
		{family: "TAN", arg: "0.0", want: "0"},
		{family: "TAN", arg: "1.0", want: "1.557407724654902230506974807458360173087"},
		{family: "SIND", arg: "30.0", want: "0.5"},
		{family: "SIND", arg: "90.0", want: "1"},
		{family: "SIND", arg: "180.0", want: "0"},
		{family: "SIND", arg: "-90.0", want: "-1"},
		{family: "COSD", arg: "60.0", want: "0.5"},
		{family: "COSD", arg: "90.0", want: "0"},
		{family: "COSD", arg: "180.0", want: "-1"},
		{family: "TAND", arg: "45.0", want: "1"},
		{family: "TAND", arg: "-45.0", want: "-1"},
		{family: "TAND", arg: "180.0", want: "0"},
		{family: "ASIN", arg: "0.0", want: "0"},
		{family: "ASIN", arg: "1.0", want: "1.570796326794896619231321691639751442098"},
		{family: "ASIN", arg: "-1.0", want: "-1.570796326794896619231321691639751442098"},
		{family: "ACOS", arg: "1.0", want: "0"},
		{family: "ACOS", arg: "-1.0", want: "3.141592653589793238462643383279502884197"},
		{family: "ACOS", arg: "0.0", want: "1.570796326794896619231321691639751442098"},
		{family: "ATAN", arg: "0.0", want: "0"},
		{family: "ATAN", arg: "1.0", want: "0.785398163397448309615660845819875721049"},
		{family: "ATAN", arg: "-1.0", want: "-0.785398163397448309615660845819875721049"},
		{family: "ASIND", arg: "1.0", want: "90"},
		{family: "ASIND", arg: "0.5", want: "30"},
		{family: "ASIND", arg: "-1.0", want: "-90"},
		{family: "ACOSD", arg: "0.5", want: "60"},
		{family: "ACOSD", arg: "-1.0", want: "180"},
		{family: "ACOSD", arg: "0.0", want: "90"},
		{family: "ATAND", arg: "1.0", want: "45"},
		{family: "ATAND", arg: "-1.0", want: "-45"},
		{family: "ATAND", arg: "0.0", want: "0"},

		// Too large a result, only where a cosine is exactly 0.
		{family: "TAND", arg: "90.0", condition: symbol("MTH$_FLOOVEMAT")},
		{family: "TAND", arg: "-270.0", condition: symbol("MTH$_FLOOVEMAT")},

		// A result too small for the format: 0.0 quietly, unless the caller
		// set FU.
		{family: "SIND", tiny: true, zero: true},
		{family: "SIND", tiny: true, fu: true, zero: true, condition: symbol("MTH$_FLOUNDMAT")},
		{family: "TAND", tiny: true, zero: true},
		{family: "TAND", tiny: true, fu: true, zero: true, condition: symbol("MTH$_FLOUNDMAT")},
		{family: "COSD", tiny: true, want: "1"},
		{family: "COSD", tiny: true, fu: true, want: "1"},

		// No sine or cosine is beyond 1.
		{family: "ASIN", arg: "2.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ACOS", arg: "-2.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ASIND", arg: "-2.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ACOSD", arg: "2.0", condition: symbol("MTH$_INVARGMAT")},

		{family: "SIN", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "COS", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "TAN", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "SIND", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "COSD", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "TAND", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ASIN", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ACOS", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ATAN", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ASIND", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ACOSD", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ATAND", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "SINH", arg: "0.0", want: "0"},
		{family: "SINH", arg: "1.0", want: "1.175201193643801456882381850595600815155"},
		{family: "SINH", arg: "-1.0", want: "-1.175201193643801456882381850595600815155"},
		{family: "SINH", arg: "20000.0", condition: symbol("MTH$_FLOOVEMAT")},
		{family: "SINH", arg: "-20000.0", condition: symbol("MTH$_FLOOVEMAT")},
		{family: "SINH", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "COSH", arg: "0.0", want: "1"},
		{family: "COSH", arg: "1.0", want: "1.543080634815243778477905620757061682601"},
		{family: "COSH", arg: "-1.0", want: "1.543080634815243778477905620757061682601"},
		{family: "COSH", arg: "20000.0", condition: symbol("MTH$_FLOOVEMAT")},
		{family: "COSH", arg: "-20000.0", condition: symbol("MTH$_FLOOVEMAT")},
		{family: "COSH", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "TANH", arg: "0.0", want: "0"},
		{family: "TANH", arg: "1.0", want: "0.761594155955764888119458282604793590412"},
		{family: "TANH", arg: "20000.0", want: "1"},
		{family: "TANH", arg: "-20000.0", want: "-1"},
		{family: "TANH", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "ATANH", arg: "0.0", want: "0"},
		{family: "ATANH", arg: "0.5", want: "0.549306144334054845697622618461262852323"},
		{family: "ATANH", arg: "-0.5", want: "-0.549306144334054845697622618461262852323"},
		{family: "ATANH", arg: "1.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ATANH", arg: "-1.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ATANH", arg: "2.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ATANH", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "ATAN2", binary: true, arg: "1.0", arg2: "1.0", want: "0.785398163397448309615660845819875721049"},
		{family: "ATAN2", binary: true, arg: "-1.0", arg2: "-1.0", want: "-2.356194490192344928846982537459627163147"},
		{family: "ATAN2", binary: true, arg: "1.0", arg2: "0.0", want: "1.570796326794896619231321691639751442098"},
		{family: "ATAN2", binary: true, arg: "-1.0", arg2: "0.0", want: "-1.570796326794896619231321691639751442098"},
		{family: "ATAN2", binary: true, arg: "0.0", arg2: "-1.0", want: "3.141592653589793238462643383279502884197"},
		{family: "ATAN2", binary: true, arg: "0.0", arg2: "1.0", want: "0"},
		{family: "ATAN2", binary: true, arg: "0.0", arg2: "0.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ATAN2", binary: true, arg: "1.0", condition: vmsdef.Symbols["SS$_ROPRAND"]},
		{family: "ATAN2", binary: true, arg2: "1.0", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		{family: "ATAND2", binary: true, arg: "1.0", arg2: "1.0", want: "45"},
		{family: "ATAND2", binary: true, arg: "1.0", arg2: "-1.0", want: "135"},
		{family: "ATAND2", binary: true, arg: "-1.0", arg2: "-1.0", want: "-135"},
		{family: "ATAND2", binary: true, arg: "1.0", arg2: "0.0", want: "90"},
		{family: "ATAND2", binary: true, arg: "0.0", arg2: "-1.0", want: "180"},
		{family: "ATAND2", binary: true, arg: "0.0", arg2: "0.0", condition: symbol("MTH$_INVARGMAT")},
		{family: "ATAND2", binary: true, arg: "1.0", condition: vmsdef.Symbols["SS$_ROPRAND"]},

		// A quotient too small for the format: 0.0 quietly, whatever the
		// caller's FU, the manual giving these routines no underflow.
		{family: "ATAN2", binary: true, tiny: true, arg2: "1.0E30", zero: true},
		{family: "ATAN2", binary: true, tiny: true, arg2: "1.0E30", fu: true, zero: true},
		{family: "ATAND2", binary: true, tiny: true, arg2: "1.0E30", fu: true, zero: true},
	}

	var cases []entryCase

	formats := []vaxfloat.Format{vaxfloat.F, vaxfloat.D, vaxfloat.G, vaxfloat.H}

	for _, row := range rows {
		family := entryFamilies[row.family]

		for i, f := range formats {
			names := []string{family.call[i]}
			names = append(names, family.jsb[i]...)

			for _, name := range names {
				c := entryCase{
					entry: name, format: f, arg: row.arg, binary: row.binary, arg2: row.arg2,
					fu: row.fu, condition: row.condition,
				}
				if row.tiny {
					c.arg = tinyArgument(f)
				}

				switch {
				case row.want != "":
					c.want = number(f, row.want)
				case !row.zero:
					c.reserved = true
				}

				// SS$_ROPRAND leaves an H result in memory alone, so the
				// program finds there what it put there, which is the
				// reserved operand too.
				cases = append(cases, c)
			}
		}
	}

	return cases
}

// entryProgram is the MACRO for the cases. Each call is followed by SHOW, which
// prints R0 to R3 and the condition the handler saw.
func entryProgram(cases []entryCase) string {
	directive := map[vaxfloat.Format]string{
		vaxfloat.F: ".F_FLOATING", vaxfloat.D: ".D_FLOATING", vaxfloat.G: ".G_FLOATING", vaxfloat.H: ".H_FLOATING",
	}

	var data, code strings.Builder

	for i, c := range cases {
		label := fmt.Sprintf("A%d", i)

		fmt.Fprintf(&code, "\t; %d. %s\n\tCLRL\tSCOND\n", i, c.entry)

		if c.fu {
			code.WriteString("\tBISPSW\t#^X40\n")
		}

		define := func(label, arg string) {
			if arg == "" {
				// A reserved operand: the sign set and the exponent zero, in any format.
				fmt.Fprintf(&data, "%s:\t.LONG\t^X8000,0,0,0\n", label)
			} else {
				fmt.Fprintf(&data, "%s:\t%s\t%s\n", label, directive[c.format], arg)
			}
		}

		define(label, c.arg)

		// A CALLS's arguments are pushed last first.
		count := 1

		if c.binary {
			second := fmt.Sprintf("B%d", i)
			define(second, c.arg2)
			fmt.Fprintf(&code, "\tPUSHAL\t%s\n", second)

			count = 2
		}

		isJSB := strings.Contains(c.entry, "_R")

		switch {
		case isJSB && c.format == vaxfloat.F:
			fmt.Fprintf(&code, "\tMOVL\t%s,R0\n", label)
		case isJSB && c.format == vaxfloat.H:
			fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n\tMOVQ\t%s+8,R2\n", label, label)
		case isJSB:
			fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n", label)
		case c.format == vaxfloat.H:
			// What a reserved argument leaves in the result: this.
			fmt.Fprintf(&code, "\tMOVL\t#^X8000,HRES\n\tCLRL\tHRES+4\n\tCLRQ\tHRES+8\n")
			fmt.Fprintf(&code, "\tPUSHAL\t%s\n\tPUSHAL\tHRES\n", label)
		default:
			fmt.Fprintf(&code, "\tPUSHAL\t%s\n", label)
		}

		switch {
		case isJSB:
			fmt.Fprintf(&code, "\tJSB\tG^%s\n", c.entry)
		case c.format == vaxfloat.H:
			fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n\tMOVQ\tHRES,R0\n\tMOVQ\tHRES+8,R2\n", count+1, c.entry)
		default:
			fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n", count, c.entry)
		}

		if c.fu {
			code.WriteString("\tBICPSW\t#^X40\n")
		}

		code.WriteString("\tJSB\tSHOW\n")
	}

	return `	.TITLE	MTHFAM
	.PSECT	DATA,WRT,NOEXE,LONG
HRES:	.BLKL	4
SCOND:	.BLKL	1
CTL:	.ASCID	/!XL !XL !XL !XL c=!XL/
BUF:	.BLKB	132
OUT:	.LONG	132
	.ADDRESS BUF
LEN:	.BLKL	1
LINE:	.BLKL	2
` + data.String() + `	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	MOVL	4(R0),SCOND
	MOVL	#1,R0
	RET
SHOW:	PUSHL	SCOND
	PUSHL	R3
	PUSHL	R2
	PUSHL	R1
	PUSHL	R0
	PUSHAL	OUT
	PUSHAL	LEN
	PUSHAQ	CTL
	CALLS	#8,G^SYS$FAO
	MOVL	LEN,LINE
	MOVAB	BUF,LINE+4
	PUSHAQ	LINE
	CALLS	#1,G^LIB$PUT_OUTPUT
	RSB
	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7,R8,R9>
	MOVAL	HANDLER,(FP)
` + code.String() + `	MOVL	#1,R0
	RET
	.END	START
`
}

func TestEntriesFromAProgram(t *testing.T) {
	_, d, out := boot(t)
	cases := entryCases(t)

	build(t, d, out, "MTHFAM", entryProgram(cases))

	if err := d.Dispatch("RUN MTHFAM"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(cases), out)
	}

	for i, c := range cases {
		// The result's longwords, in the registers its format takes.
		want := make([]string, c.format.Size()/4)

		bits, err := vaxfloat.Pack(c.format, c.want)
		if err != nil {
			t.Fatal(err)
		}

		if c.reserved {
			bits = vaxfloat.Reserved(c.format)
		}

		for j, w := range []uint32{uint32(bits.Lo), uint32(bits.Lo >> 32), uint32(bits.Hi), uint32(bits.Hi >> 32)}[:len(want)] {
			want[j] = fmt.Sprintf("%08X", w)
		}

		fields := strings.Fields(lines[i])
		if got := strings.Join(fields[:len(want)], " "); got != strings.Join(want, " ") {
			t.Errorf("%s(%q, %q): registers %s, want %s", c.entry, c.arg, c.arg2, got, strings.Join(want, " "))
		}

		if got, want := fields[4], fmt.Sprintf("c=%08X", c.condition); got != want {
			t.Errorf("%s(%q, %q): %s, want %s", c.entry, c.arg, c.arg2, got, want)
		}
	}
}

// TestRoutineUnhandled: a routine's condition with no handler to take it is
// the condition's message, and the severe condition ends the image.
func TestRoutineUnhandled(t *testing.T) {
	_, d, out := boot(t)

	build(t, d, out, "MTHLOG", `	.TITLE	MTHLOG
	.PSECT	DATA,WRT,NOEXE,LONG
Z:	.FLOAT	0.0
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAL	Z
	CALLS	#1,G^MTH$ALOG
	CALLS	#0,G^MTH$SIN
	RET
	.END	START
`)

	_ = d.Dispatch("RUN MTHLOG") // the image's end is what's looked at

	text := out.String()

	if !strings.Contains(text, "%MTH-F-LOGZERNEG, ") {
		t.Errorf("no message; RUN said:\n%s", text)
	}

	if strings.Contains(text, "MTH$SIN") {
		t.Errorf("the image went on after a severe condition:\n%s", text)
	}
}

// pairCase is one call of MTH$SINCOS's or MTH$SINCOSD's kind: the argument (or
// empty for a reserved operand), whether the caller has FU set, the two results,
// and the condition the handler should see.
type pairCase struct {
	entry        string
	format       vaxfloat.Format
	arg          string
	fu           bool
	sine, cosine string
	condition    uint32
}

// pairCases are each row below by each entry of the kind the row names.
func pairCases() []pairCase {
	floundmat := vmsdef.LibrarySymbols["MTH$_FLOUNDMAT"]
	ropRand := vmsdef.Symbols["SS$_ROPRAND"]

	rows := []struct {
		stem         string
		arg          string
		tiny, fu     bool
		sine, cosine string
		condition    uint32
	}{
		{stem: "SINCOS", arg: "0.0", sine: "0", cosine: "1"},
		{
			stem: "SINCOS", arg: "1.0",
			sine:   "0.841470984807896506652502321630298999622",
			cosine: "0.540302305868139717400936607442976603732",
		},
		{stem: "SINCOS", condition: ropRand},

		{stem: "SINCOSD", arg: "30.0", sine: "0.5", cosine: "0.866025403784438646763723170752936183471"},
		{stem: "SINCOSD", arg: "90.0", sine: "1", cosine: "0"},
		{stem: "SINCOSD", arg: "-180.0", sine: "0", cosine: "-1"},
		{stem: "SINCOSD", tiny: true, sine: "0", cosine: "1"},
		{stem: "SINCOSD", tiny: true, fu: true, sine: "0", cosine: "1", condition: floundmat},
		{stem: "SINCOSD", condition: ropRand},
	}

	var cases []pairCase

	for _, row := range rows {
		for i, f := range []vaxfloat.Format{vaxfloat.F, vaxfloat.D, vaxfloat.G, vaxfloat.H} {
			prefix := []string{"", "D", "G", "H"}[i]
			register := []string{"5", "7", "7", "7"}[i]

			for _, name := range []string{"MTH$" + prefix + row.stem, "MTH$" + prefix + row.stem + "_R" + register} {
				c := pairCase{
					entry: name, format: f, arg: row.arg, fu: row.fu,
					sine: row.sine, cosine: row.cosine, condition: row.condition,
				}
				if row.tiny {
					c.arg = tinyArgument(f)
				}

				cases = append(cases, c)
			}
		}
	}

	return cases
}

// pairProgram is the MACRO for the cases. Each call is followed by what puts the
// two results in the registers a JSB entry returns them in, so that one check
// does for both kinds of entry, and SHOW, which prints R0 to R7 and the condition
// the handler saw.
func pairProgram(cases []pairCase) string {
	directive := map[vaxfloat.Format]string{
		vaxfloat.F: ".F_FLOATING", vaxfloat.D: ".D_FLOATING", vaxfloat.G: ".G_FLOATING", vaxfloat.H: ".H_FLOATING",
	}

	var data, code strings.Builder

	for i, c := range cases {
		label := fmt.Sprintf("A%d", i)

		if c.arg == "" {
			fmt.Fprintf(&data, "%s:\t.LONG\t^X8000,0,0,0\n", label)
		} else {
			fmt.Fprintf(&data, "%s:\t%s\t%s\n", label, directive[c.format], c.arg)
		}

		fmt.Fprintf(&code, "\t; %d. %s\n\tCLRL\tSCOND\n", i, c.entry)

		if c.fu {
			code.WriteString("\tBISPSW\t#^X40\n")
		}

		if strings.Contains(c.entry, "_R") {
			switch c.format {
			case vaxfloat.F:
				fmt.Fprintf(&code, "\tMOVL\t%s,R0\n", label)
			case vaxfloat.H:
				fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n\tMOVQ\t%s+8,R2\n", label, label)
			default:
				fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n", label)
			}

			fmt.Fprintf(&code, "\tJSB\tG^%s\n", c.entry)
		} else {
			fmt.Fprintf(&code, "\tPUSHAL\tRCOS\n\tPUSHAL\tRSIN\n\tPUSHAL\t%s\n\tCALLS\t#3,G^%s\n", label, c.entry)

			switch c.format {
			case vaxfloat.F:
				code.WriteString("\tMOVL\tRSIN,R0\n\tMOVL\tRCOS,R1\n")
			case vaxfloat.H:
				code.WriteString("\tMOVQ\tRSIN,R0\n\tMOVQ\tRSIN+8,R2\n\tMOVQ\tRCOS,R4\n\tMOVQ\tRCOS+8,R6\n")
			default:
				code.WriteString("\tMOVQ\tRSIN,R0\n\tMOVQ\tRCOS,R2\n")
			}
		}

		if c.fu {
			code.WriteString("\tBICPSW\t#^X40\n")
		}

		code.WriteString("\tJSB\tSHOW\n")
	}

	return `	.TITLE	MTHPAIR
	.PSECT	DATA,WRT,NOEXE,LONG
RSIN:	.BLKL	4
RCOS:	.BLKL	4
SCOND:	.BLKL	1
CTL:	.ASCID	/!XL !XL !XL !XL !XL !XL !XL !XL c=!XL/
BUF:	.BLKB	132
OUT:	.LONG	132
	.ADDRESS BUF
LEN:	.BLKL	1
LINE:	.BLKL	2
` + data.String() + `	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	MOVL	4(R0),SCOND
	MOVL	#1,R0
	RET
SHOW:	PUSHL	SCOND
	PUSHL	R7
	PUSHL	R6
	PUSHL	R5
	PUSHL	R4
	PUSHL	R3
	PUSHL	R2
	PUSHL	R1
	PUSHL	R0
	PUSHAL	OUT
	PUSHAL	LEN
	PUSHAQ	CTL
	CALLS	#12,G^SYS$FAO
	MOVL	LEN,LINE
	MOVAB	BUF,LINE+4
	PUSHAQ	LINE
	CALLS	#1,G^LIB$PUT_OUTPUT
	RSB
	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7,R8,R9>
	MOVAL	HANDLER,(FP)
` + code.String() + `	MOVL	#1,R0
	RET
	.END	START
`
}

// TestPairsFromAProgram: MTH$SINCOS and MTH$SINCOSD, in every format, by both
// kinds of entry. The second result of a JSB entry starts at R1, R2, or R4 by
// format, and that's how the program shows a CALL entry's. An underflow signaled
// leaves the sine 0.0 and the cosine good, for a handler that continues, in R1
// too for an F JSB entry, whose cosine goes through the mechanism array.
func TestPairsFromAProgram(t *testing.T) {
	_, d, out := boot(t)
	cases := pairCases()

	build(t, d, out, "MTHPAIR", pairProgram(cases))

	if err := d.Dispatch("RUN MTHPAIR"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(cases), out)
	}

	words := func(f vaxfloat.Format, text string) []string {
		x, err := vaxfloat.Parse(text)
		if err != nil {
			t.Fatal(err)
		}

		bits, err := vaxfloat.Pack(f, x)
		if err != nil {
			t.Fatal(err)
		}

		list := make([]string, f.Size()/4)
		for j, w := range []uint32{uint32(bits.Lo), uint32(bits.Lo >> 32), uint32(bits.Hi), uint32(bits.Hi >> 32)}[:len(list)] {
			list[j] = fmt.Sprintf("%08X", w)
		}

		return list
	}

	for i, c := range cases {
		fields := strings.Fields(lines[i])

		if got, want := fields[8], fmt.Sprintf("c=%08X", c.condition); got != want {
			t.Errorf("%s(%q): %s, want %s", c.entry, c.arg, got, want)
		}

		// A reserved operand has no results to look at.
		if c.arg == "" {
			continue
		}

		want := append(words(c.format, c.sine), words(c.format, c.cosine)...)
		if got := strings.Join(fields[:len(want)], " "); got != strings.Join(want, " ") {
			t.Errorf("%s(%q): registers %s, want %s", c.entry, c.arg, got, strings.Join(want, " "))
		}
	}
}
