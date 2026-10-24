package debugger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/debugger"
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

// TestTracedExaminePSL: EXAMINE PSL shows T once the BISPSW has run, and
// TP too once the instruction after it has (the trace is then pending).
func TestTracedExaminePSL(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "STEP")
	say(t, c, "STEP")

	//                                  CMP TP FPD IS CURMOD PRVMOD IPL DV FU IV T N Z V C
	expect(t, "EXAMINE PSL after BISPSW", pslLine(t, c), "0 0 0 0 KERNEL USER 0 0 0 0 1 0 0 0 0")

	say(t, c, "STEP")
	expect(t, "EXAMINE PSL after a traced NOP", pslLine(t, c), "0 1 0 0 KERNEL USER 0 0 0 0 1 0 0 0 0")
}

// TestTracedStep: a STEP whose instruction has a trace pending lets the
// program's handler take it, silently, then runs the instruction: each
// step moves one instruction, as it does without tracing, and the handler
// sees every trace, as under GO.
func TestTracedStep(t *testing.T) {
	c := tracedSession(t, tracedSource)

	for _, s := range []struct{ step, count string }{
		{"stepped to TRACED\\START\\%LINE 7: BISPSW   S^#10\n", "00000000"},
		{"stepped to TRACED\\START\\%LINE 8: NOP     \n", "00000000"},
		{"stepped to TRACED\\START\\%LINE 9: NOP     \n", "00000000"},
		{"stepped to TRACED\\START\\%LINE 10: NOP     \n", "00000001"},
		{"stepped to TRACED\\START\\%LINE 11: BICPSW   S^#10\n", "00000002"},
		{"stepped to TRACED\\START\\%LINE 12: NOP     \n", "00000003"},
		{"stepped to TRACED\\START\\%LINE 13: MOVL     S^#01,R0\n", "00000004"},
	} {
		out := say(t, c, "STEP")
		if first, _, _ := strings.Cut(out, "\n"); first+"\n" != s.step {
			t.Errorf("STEP:\n got %q\nwant %q", out, s.step)
		}

		expectCount(t, c, s.count)
	}
}

// TestTracedGoFromBreak: a breakpoint stops before a trace that is
// pending there; GO gives the trace to the program and runs on, without
// stopping at the same breakpoint again when the handler returns.
func TestTracedGoFromBreak(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK %LINE 10")
	say(t, c, "SET BREAK %LINE 12")

	expect(t, "GO", say(t, c, "GO"), "break at TRACED\\START\\%LINE 10\n    10:         NOP\n")
	expect(t, "EXAMINE PSL", pslLine(t, c), "0 1 0 0 KERNEL USER 0 0 0 0 1 0 0 0 0")
	expectCount(t, c, "00000001")

	expect(t, "GO", say(t, c, "GO"), "break at TRACED\\START\\%LINE 12\n    12:         NOP\n")
	expectCount(t, c, "00000003")

	// STEP from a breakpoint is the same: one instruction, the trace
	// before it taken.
	if out := say(t, c, "STEP"); !strings.HasPrefix(out, "stepped to TRACED\\START\\%LINE 13: ") {
		t.Errorf("STEP from the breakpoint:\n%s", out)
	}

	expectCount(t, c, "00000004")
}

// TestTracedStepBreakInHandler: a breakpoint in the handler stops a STEP
// that is giving the program its trace; the handler can be stepped, and GO
// goes back to the program.
func TestTracedStepBreakInHandler(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "STEP 3") // to line 9, the trace pending
	say(t, c, "SET BREAK TRACER")

	expect(t, "STEP", say(t, c, "STEP"), "break at routine TRACED\\TRACER\n    16:         MOVL    4(AP),R0\n")

	if out := say(t, c, "STEP"); !strings.HasPrefix(out, "stepped to TRACED\\TRACER\\%LINE 17: ") {
		t.Errorf("STEP in the handler:\n%s", out)
	}

	say(t, c, "CANCEL BREAK/ALL")
	say(t, c, "SET BREAK %LINE 10")

	expect(t, "GO", say(t, c, "GO"), "break at TRACED\\START\\%LINE 10\n    10:         NOP\n")
	expectCount(t, c, "00000001")
}

// TestTracedBreakException: SET BREAK/EXCEPTION stops at each SS$_TBIT
// before the program's handler sees it, with the condition's message
// formatted from its signal array, the PC there the instruction the trace
// comes before; GO passes the condition on to the handler.
func TestTracedBreakException(t *testing.T) {
	c := tracedSession(t, tracedSource)

	say(t, c, "SET BREAK/EXCEPTION")

	out := say(t, c, "GO")
	want := "%SYSTEM-F-TBIT, T-bit pending trap at PC=0000040C, PS=00C00010\n" +
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

	// A STEP from the stop delivers the trace too.
	if out := say(t, c, "STEP"); !strings.HasPrefix(out, "stepped to TRACED\\START\\%LINE 11: ") {
		t.Errorf("STEP from the fault break:\n%s", out)
	}

	expectCount(t, c, "00000002")

	// Clearing TP at the stop drops the trace.
	expect(t, "GO", say(t, c, "GO"), "Break on fault 28 TP at PC = 0000040E\n")
	say(t, c, "DEPOSIT PSL = 10")
	say(t, c, "CANCEL BREAK/ALL")
	say(t, c, "GO")
	expectCount(t, c, "00000003")
}

// TestTracedDeposit: DEPOSIT to the PSL setting T starts tracing (the
// next instruction runs, and the trace comes before the one after it);
// /WORD and /BYTE change only the low word or byte; other sizes are
// refused.
func TestTracedDeposit(t *testing.T) {
	c := tracedSession(t, untracedSource)

	say(t, c, "STEP") // past the MOVAB, at line 7's NOP

	before := uint32(c.CPU.PSL())

	say(t, c, "DEPOSIT/WORD PSL = 10")

	if got := uint32(c.CPU.PSL()); got != before&^0xFFFF|0x10 {
		t.Errorf("PSL after DEPOSIT/WORD = %08X, from %08X", got, before)
	}

	say(t, c, "STEP")
	expect(t, "EXAMINE PSL", pslLine(t, c), "0 1 0 0 KERNEL USER 0 0 0 0 1 0 0 0 0")
	expectCount(t, c, "00000000")

	say(t, c, "STEP")
	expectCount(t, c, "00000001")

	say(t, c, "GO")
	expectCount(t, c, "00000004")

	if _, err := sayErr(c, "DEPOSIT/QUADWORD PSL = 0"); err == nil || !strings.Contains(err.Error(), "OPTNOTALLOW") {
		t.Errorf("DEPOSIT/QUADWORD PSL: %v", err)
	}

	say(t, c, "DEPOSIT PSL = 12345678")

	if got := uint32(c.CPU.PSL()); got != 0x12345678 {
		t.Errorf("PSL after DEPOSIT = %08X", got)
	}

	say(t, c, "DEPOSIT/BYTE PSL = 0")

	if got := uint32(c.CPU.PSL()); got != 0x12345600 {
		t.Errorf("PSL after DEPOSIT/BYTE = %08X", got)
	}
}

// TestTracedDebuggerTakes: with the other answer to Decision 5, the
// debugger takes the trace pending when a STEP begins, so the program's
// handler doesn't see the traces of the instructions stepped; under GO
// it sees the rest.
func TestTracedDebuggerTakes(t *testing.T) {
	c := tracedSession(t, tracedSource)
	debugger.SetTakeTraces(dbgOf(c), true)

	// To line 11: the traces before lines 9 and 10 are dropped, and the
	// one before line 11 is pending.
	for range 5 {
		say(t, c, "STEP")
	}

	expectCount(t, c, "00000000")

	say(t, c, "GO")
	expectCount(t, c, "00000002") // before the BICPSW, and after it
}
