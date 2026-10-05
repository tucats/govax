package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/vax"
)

// stepImage loads image (testdata/dbg/vax) as RUN/STEP does, but stepping
// the image's own instructions: the test console runs in kernel mode, so
// STEP's default of stepping only user-mode code (USERSTEP) is turned
// off. It returns a dispatcher and RUN/STEP's output; the buffer is
// cleared before each command by dispatchOutput.
func stepImage(t *testing.T, image string) (*Dispatcher, *bytes.Buffer, string) {
	t.Helper()

	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	if err := c.Run(dbgImagePath(t, image), RunOptions{Step: true}); err != nil {
		t.Fatalf("RUN/STEP %s: %v", image, err)
	}

	return NewDispatcher(c, loadEvaxGrammar(t), nil), buf, buf.String()
}

// dispatchOutput runs command and returns what it printed.
func dispatchOutput(t *testing.T, d *Dispatcher, buf *bytes.Buffer, command string) string {
	t.Helper()

	buf.Reset()

	if err := d.Dispatch(command); err != nil {
		t.Fatalf("%s: %v", command, err)
	}

	return buf.String()
}

// withoutStack strips the trace's "[KSP 800049C0] " prefix from each
// line, whose stack address isn't what a test is about.
func withoutStack(s string) string {
	lines := strings.Split(s, "\n")

	for i, l := range lines {
		if strings.HasPrefix(l, "[") {
			if _, rest, ok := strings.Cut(l, "] "); ok {
				lines[i] = rest
			}
		}
	}

	return strings.Join(lines, "\n")
}

// TestStepSymbolic: inside an image with a debug symbol table, STEP's
// trace shows each instruction as DISASSEMBLE/SYMBOLIC does, and
// "Stepped to" and "Break at" name the location as the debugger's
// "stepped to" and "break at" do (dbgdis.dlg); outside one (the IMAGE$INIT
// driver's CALLS), the trace is the console's own.
func TestStepSymbolic(t *testing.T) {
	d, buf, run := stepImage(t, "dbgdis.exe")

	want := `8000660A: CALLS I^#^X00000000,@#START
Stepped to DBGDIS\START\%LINE 42
`
	if got := withoutStack(run); !strings.HasPrefix(got, "8000660A:") || !strings.HasSuffix(got, "Stepped to DBGDIS\\START\\%LINE 42\n") {
		t.Errorf("RUN/STEP:\ngot:\n%s\nwant (registers aside):\n%s", got, want)
	}

	want = `DBGDIS\START\%LINE 42:  MOVL     S^#0A,R2
                     R2:  0000000A  10
Stepped to DBGDIS\START\%LINE 43
`
	if got := withoutStack(dispatchOutput(t, d, buf, "STEP")); got != want {
		t.Errorf("STEP:\ngot:\n%s\nwant:\n%s", got, want)
	}

	dispatchOutput(t, d, buf, `SET BREAK DBGSUB\SUB2+2`)

	if got := dispatchOutput(t, d, buf, "GO"); got != "Break at DBGSUB\\SUB2\\%LINE 20\n" {
		t.Errorf("GO: got %q", got)
	}

	// STEP into a JSB subroutine lands on its label.
	want = `DBGSUB\SUB2\%LINE 20:   JSB      L^DBGSUB\SUB2\SUBJSB
Stepped to DBGSUB\SUB2\SUBJSB
`
	if got := withoutStack(dispatchOutput(t, d, buf, "STEP")); got != want {
		t.Errorf("STEP into SUBJSB:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestStepTraceback: an image with only traceback records has no lines,
// so a location is its routine plus an offset (dbgtrc.dlg's "stepped to
// DBGDIS\START+5").
func TestStepTraceback(t *testing.T) {
	d, buf, _ := stepImage(t, "dbgtrc.exe")

	want := `DBGDIS\START+2: MOVL     S^#0A,R2
                     R2:  0000000A  10
Stepped to DBGDIS\START+5
`
	if got := withoutStack(dispatchOutput(t, d, buf, "STEP")); got != want {
		t.Errorf("STEP:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestStepNoSymbolic: with the vax.disassemble.symbolic setting false,
// STEP is the console's own display, in an image with debug data too.
func TestStepNoSymbolic(t *testing.T) {
	old, had := settings.Get(symbolicSetting), settings.Exists(symbolicSetting)

	t.Cleanup(func() {
		if had {
			settings.Set(symbolicSetting, old)
		} else {
			_ = settings.Delete(symbolicSetting)
		}
	})

	settings.Set(symbolicSetting, "false")

	d, buf, run := stepImage(t, "dbgdis.exe")

	if !strings.HasSuffix(run, "Stepped to 00000402\n") {
		t.Errorf("RUN/STEP:\n%s", run)
	}

	want := `00000402: MOVL S^#10,R2
                     R2:  0000000A  10
Stepped to 00000405
`
	if got := withoutStack(dispatchOutput(t, d, buf, "STEP")); got != want {
		t.Errorf("STEP:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestShowCallsFault: FAILLNK's access violation, three calls deep.
// Stepped to the faulting instruction, SHOW CALLS is the debugger's
// table, as FAIL.DBG's session showed it at the fault (faillnk.dlg), and
// SHOW CALLS 1 its first row; /NOSYMBOLIC is the console's own dump.
// Stepping the fault puts the PC in the condition dispatcher, in no image,
// where SHOW CALLS is the console's dump too.
func TestShowCallsFault(t *testing.T) {
	d, buf, _ := stepImage(t, "faillnk.exe")

	for range 4 {
		dispatchOutput(t, d, buf, "STEP")
	}

	if got := d.Console.CPU.GPR(vax.PC); got != 0x21C {
		t.Fatalf("PC is %08X, not the MOVL @#0 at 0000021C", got)
	}

	// faillnk.dlg's lines 21 to 24 and 44 to 45.
	rows := []string{
		debugCallsHeading,
		"*FAILSUB         SUB2                12               00000005         0000021C",
		"*FAILSUB         SUB1                 7               0000000A         00000216",
		"*FAILMAIN        FAILMAIN            14               00000009         00000209",
	}

	for command, want := range map[string]string{
		"SHOW CALLS":   strings.Join(rows, "\n") + "\n",
		"SHOW CALLS 1": strings.Join(rows[:2], "\n") + "\n",
	} {
		if got := dispatchOutput(t, d, buf, command); got != want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", command, got, want)
		}
	}

	if got := dispatchOutput(t, d, buf, "SHOW CALLS/NOSYMBOLIC"); !strings.HasPrefix(got, "    FRAME: ") {
		t.Errorf("SHOW CALLS/NOSYMBOLIC:\n%s", got)
	}

	// The fault sends the PC to the condition dispatcher, in no image.
	want := "FAILSUB\\SUB2\\%LINE 12:  MOVL     @#00000000,R0\nStepped to 7FFEE118\n"
	if got := withoutStack(dispatchOutput(t, d, buf, "STEP")); got != want {
		t.Errorf("STEP into the fault:\ngot:\n%s\nwant:\n%s", got, want)
	}

	if got := dispatchOutput(t, d, buf, "SHOW CALLS"); !strings.HasPrefix(got, "    FRAME: ") {
		t.Errorf("SHOW CALLS in the condition dispatcher:\n%s", got)
	}
}

// TestShowCallsFrames: DBGDIS.DBG's breakpoints, as dbgdis.dlg shows
// them: in a JSB subroutine (no frame of its own: the row is the routine
// that holds it), and in SUB2, four calls deep, from START's two calls of
// SUB1. In DBGTRC, a traceback link, the line column is blank.
func TestShowCallsFrames(t *testing.T) {
	const heading = debugCallsHeading + "\n"

	d, buf, _ := stepImage(t, "dbgdis.exe")

	dispatchOutput(t, d, buf, `SET BREAK DBGDIS\LOCALR\JSBRTN`)
	dispatchOutput(t, d, buf, "GO")

	want := heading + "*DBGDIS          LOCALR             110               00000009         0000053B\n"
	if got := dispatchOutput(t, d, buf, "SHOW CALLS"); got != want {
		t.Errorf("at JSBRTN:\ngot:\n%s\nwant:\n%s", got, want)
	}

	dispatchOutput(t, d, buf, "CLEAR BREAKPOINT/ALL")
	dispatchOutput(t, d, buf, `SET BREAK DBGSUB\SUB2+2`)

	for _, line := range []string{"90               000000FD         000004FD", "91               00000108         00000508"} {
		dispatchOutput(t, d, buf, "GO")

		want := heading +
			"*DBGSUB          SUB2                20               00000002         0000055A\n" +
			"*DBGSUB          SUB1                15               00000010         00000554\n" +
			"*DBGDIS          START               " + line + "\n"
		if got := dispatchOutput(t, d, buf, "SHOW CALLS"); got != want {
			t.Errorf("at SUB2:\ngot:\n%s\nwant:\n%s", got, want)
		}
	}

	d, buf, _ = stepImage(t, "dbgtrc.exe")

	dispatchOutput(t, d, buf, `SET BREAK DBGSUB\SUB2+2`)
	dispatchOutput(t, d, buf, "GO")

	want = heading +
		"*DBGSUB          SUB2                                 00000002         0000055A\n" +
		"*DBGSUB          SUB1                                 00000010         00000554\n" +
		"*DBGDIS          START                                000000FD         000004FD\n"
	if got := dispatchOutput(t, d, buf, "SHOW CALLS"); got != want {
		t.Errorf("DBGTRC at SUB2:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
