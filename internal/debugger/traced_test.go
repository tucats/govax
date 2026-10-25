package debugger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
)

// The debugger beside a program that traces itself (docs/PHASE-73 -
// trace trap.md, subtask 3; traced.go).

// tracedSource is a program that traces itself: TRACER takes each
// SS$_TBIT, counting it in COUNT, and resignals anything else. The BISPSW
// sets T, so the first trace comes after the first NOP (line 8), before
// the second (line 9); the BICPSW (line 11) is traced too, and the last
// trace comes before the NOP after it (line 12): four in all.
const tracedSource = `	.TITLE	TRACED
	.PSECT	DATA,WRT,NOEXE
COUNT:	.LONG	0
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	START,^M<>
	MOVAB	TRACER,(FP)
	BISPSW	#^X10
	NOP
	NOP
	NOP
	BICPSW	#^X10
	NOP
	MOVL	#1,R0
	RET
	.ENTRY	TRACER,^M<>
	MOVL	4(AP),R0
	CMPL	4(R0),#^X464
	BNEQ	10$
	INCL	COUNT
	MOVL	#1,R0
	RET
10$:	MOVL	#^X918,R0
	RET
	.END	START
`

// untracedSource is tracedSource without the BISPSW, for a DEPOSIT to
// the PSL to start tracing.
var untracedSource = strings.Replace(tracedSource, "\tBISPSW\t#^X10\n", "", 1)

// tracedSession assembles and links source with /DEBUG and runs it under
// the debugger, stopped at START's first instruction, stepping by
// instruction. The test console runs programs in kernel mode, so STEP's
// default of stepping only user-mode code is turned off, as stepImage
// does.
func tracedSession(t *testing.T, source string) *console.Console {
	t.Helper()

	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	c.CPU.SetDebug(c.CPU.Debug() &^ registers.DebugUserStep)

	path := filepath.Join(t.TempDir(), "traced")
	if err := os.WriteFile(path+".mar", []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Macro(dclcmd.MacroOptions{Source: path + ".mar", Debug: true}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Commands().Link(dclcmd.LinkOptions{Objects: []string{path}, Debug: true}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	c.Out.(*bytes.Buffer).Reset()

	if err := c.Run(path+".exe", machine.RunOptions{Debug: machine.DebugOn}); err != nil {
		t.Fatalf("RUN/DEBUG: %v", err)
	}

	say(t, c, "SET STEP INSTRUCTION")

	return c
}

// expectCount checks how many traces the program's handler has taken.
func expectCount(t *testing.T, c *console.Console, want string) {
	t.Helper()

	expect(t, "EXAMINE COUNT", say(t, c, "EXAMINE COUNT"), "TRACED\\COUNT:   "+want+"\n")
}

// pslLine is the second line of EXAMINE PSL's table, the fields' values,
// with its runs of blanks made one.
func pslLine(t *testing.T, c *console.Console) string {
	t.Helper()

	lines := strings.Split(say(t, c, "EXAMINE PSL"), "\n")
	if len(lines) < 3 {
		t.Fatalf("EXAMINE PSL: %q", lines)
	}

	return strings.Join(strings.Fields(lines[2]), " ")
}

// TestTracedBreakTakesTrace: a breakpoint takes the trace pending where
// it stops (the handler never sees it; T shows set and TP clear), and a
// STEP from it leaves T clear, so the program's tracing ends, as VMS
// 7.1's debugger does (testdata/trace73's TRSTEP).
func TestTracedBreakTakesTrace(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK %LINE 9")
	expect(t, "GO", say(t, c, "GO"), "break at TRACED\\START\\%LINE 9: NOP     \n     9:         NOP\n")

	//                                  CMP TP FPD IS CURMOD PRVMOD IPL DV FU IV T N Z V C
	expect(t, "EXAMINE PSL at the break", pslLine(t, c), "0 0 0 0 KERNEL USER 0 0 0 0 1 0 0 0 0")
	expectCount(t, c, "00000000")

	if out := say(t, c, "STEP"); !strings.HasPrefix(out, "stepped to TRACED\\START\\%LINE 10: ") {
		t.Errorf("STEP:\n%s", out)
	}

	expect(t, "EXAMINE PSL after STEP", pslLine(t, c), "0 0 0 0 KERNEL USER 0 0 0 0 0 0 0 0 0")
	expectCount(t, c, "00000000")

	say(t, c, "GO")
	expectCount(t, c, "00000000")
}

// TestTracedGoFromBreak: going on from a breakpoint in traced code, the
// first instruction is the debugger's and the program's T is cleared
// after it, so its handler gets no more traces, though later breakpoints
// are still reached (TRGO).
func TestTracedGoFromBreak(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK %LINE 9")
	say(t, c, "SET BREAK %LINE 12")

	expect(t, "GO", say(t, c, "GO"), "break at TRACED\\START\\%LINE 9: NOP     \n     9:         NOP\n")
	expect(t, "GO", say(t, c, "GO"), "break at TRACED\\START\\%LINE 12: NOP     \n    12:         NOP\n")
	expectCount(t, c, "00000000")

	say(t, c, "GO")
	expectCount(t, c, "00000000")
}

// TestTracedBreakInHandler: a breakpoint in the trace handler doesn't
// disturb the tracing: it is reached at each trace, and the handler takes
// every one (TRHAND).
func TestTracedBreakInHandler(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK TRACER")

	for range 2 {
		expect(t, "GO", say(t, c, "GO"), "break at routine TRACED\\TRACER\n    16:         MOVL    4(AP),R0\n")
	}

	say(t, c, "CANCEL BREAK/ALL")
	say(t, c, "GO")
	expectCount(t, c, "00000004")
}

// TestTracedBreakException: SET BREAK/EXCEPTION stops at each SS$_TBIT
// before the program's handler sees it, with the condition's message
// formatted from its signal array, the PC there the instruction the trace
// comes before; GO passes the condition on to the handler.
func TestTracedBreakException(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK/EXCEPTION")

	out := say(t, c, "GO")
	want := "%SYSTEM-F-TBIT, T-bit pending trap at PC=0000040C, PSL=00C00010\n" +
		"break on exception at TRACED\\START\\%LINE 9: NOP     \n"

	if !strings.HasPrefix(out, want) {
		t.Errorf("GO:\n got %q\nwant %q...", out, want)
	}

	expect(t, "EVALUATE %EXC_NAME", say(t, c, "EVALUATE %EXC_NAME"), "\"TBIT\"\n")
	expectCount(t, c, "00000000")

	if out := say(t, c, "GO"); !strings.Contains(out, "PC=0000040D") {
		t.Errorf("the second GO:\n%s", out)
	}

	expectCount(t, c, "00000001")

	say(t, c, "CANCEL BREAK/ALL")

	if out := say(t, c, "GO"); !strings.Contains(out, "%DEBUG-I-EXITSTATUS") {
		t.Errorf("GO to the end:\n%s", out)
	}

	expectCount(t, c, "00000004")
}

// TestTracedFaultBreak: govax's SET BREAK/FAULT=28, the trace fault's
// vector, stops before each trace is delivered, with the trace still
// pending (TP set) and the PC the instruction it comes before; GO delivers
// it to the program and goes on to the next.
func TestTracedFaultBreak(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK/FAULT=28")

	expect(t, "GO", say(t, c, "GO"), "Break on fault 28 TP at PC = 0000040C\n")
	expect(t, "EXAMINE PSL", pslLine(t, c), "0 1 0 0 KERNEL USER 0 0 0 0 1 0 0 0 0")
	expectCount(t, c, "00000000")

	expect(t, "GO", say(t, c, "GO"), "Break on fault 28 TP at PC = 0000040D\n")
	expectCount(t, c, "00000001")

	// A STEP from the stop delivers the trace, then ends the tracing, as
	// any step does.
	if out := say(t, c, "STEP"); !strings.HasPrefix(out, "stepped to TRACED\\START\\%LINE 11: ") {
		t.Errorf("STEP from the fault break:\n%s", out)
	}

	expectCount(t, c, "00000002")

	say(t, c, "CANCEL BREAK/ALL")
	say(t, c, "GO")
	expectCount(t, c, "00000002")
}

// TestTracedDeposit: DEPOSIT to the PSL changes only its low word
// (TRPRIV): a longword or word sets the PSW, a byte its low byte. A T it
// sets is the debugger's to clear when the program goes on from the stop
// (TRDEP, TRDEPGO). Other sizes are refused.
func TestTracedDeposit(t *testing.T) {
	c := tracedSession(t, untracedSource)

	say(t, c, "STEP") // past the MOVAB, at line 7's NOP

	before := uint32(c.CPU.PSL())

	say(t, c, "DEPOSIT/WORD PSL = 10")

	if got := uint32(c.CPU.PSL()); got != before&^0xFFFF|0x10 {
		t.Errorf("PSL after DEPOSIT/WORD = %08X, from %08X", got, before)
	}

	say(t, c, "STEP")
	expect(t, "EXAMINE PSL", pslLine(t, c), "0 0 0 0 KERNEL USER 0 0 0 0 0 0 0 0 0")

	say(t, c, "DEPOSIT/WORD PSL = 10")
	say(t, c, "GO")
	expectCount(t, c, "00000000")

	if _, err := sayErr(c, "DEPOSIT/QUADWORD PSL = 0"); err == nil || !strings.Contains(err.Error(), "OPTNOTALLOW") {
		t.Errorf("DEPOSIT/QUADWORD PSL: %v", err)
	}

	high := uint32(c.CPU.PSL()) &^ 0xFFFF

	say(t, c, "DEPOSIT PSL = 12345678")

	if got := uint32(c.CPU.PSL()); got != high|0x5678 {
		t.Errorf("PSL after DEPOSIT = %08X, want %08X", got, high|0x5678)
	}

	say(t, c, "DEPOSIT/BYTE PSL = 0F")

	if got := uint32(c.CPU.PSL()); got != high|0x560F {
		t.Errorf("PSL after DEPOSIT/BYTE = %08X, want %08X", got, high|0x560F)
	}
}

// TestTracedUnhandledContinues: GO from a break on an unhandled SS$_TBIT
// (a severe condition) warns, %DEBUG-I-CONFROMEXC, and continues the
// program, which takes its next trace, as VMS 7.1's debugger does
// (TRUNH).
func TestTracedUnhandledContinues(t *testing.T) {
	c := tracedSession(t, "\t.TITLE\tTRACED\n\t.PSECT\tCODE,NOWRT,EXE\n\t.ENTRY\tSTART,^M<>\n"+
		"\tNOP\n\tBISPSW\t#^X10\n\tNOP\n\tNOP\n\tBICPSW\t#^X10\n\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	expect(t, "GO", say(t, c, "GO"), "%SYSTEM-F-TBIT, T-bit pending trap at PC=00000206, PSL=00C00010\n"+
		"break on unhandled exception at TRACED\\START\\%LINE 7: NOP     \n     7:         NOP\n")

	expect(t, "GO", say(t, c, "GO"), "%DEBUG-I-CONFROMEXC, WARNING: continuing after severe error; integrity not guaranteed\n"+
		"%SYSTEM-F-TBIT, T-bit pending trap at PC=00000207, PSL=00C00010\n"+
		"break on unhandled exception at TRACED\\START\\%LINE 8: BICPSW   S^#10\n     8:         BICPSW  #^X10\n")
}
