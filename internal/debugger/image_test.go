package debugger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vax"
)

// runImage loads the image at path with RUN's options opts on a runnable
// console with the debugger installed, and returns the console and RUN's
// output.
func runImage(t *testing.T, path string, opts console.RunOptions) (*console.Console, string) {
	t.Helper()

	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	if err := c.Run(path, opts); err != nil {
		t.Fatalf("RUN: %v", err)
	}

	return c, buf.String()
}

// goOutput continues the program with the debugger's own GO and returns
// what it printed, and the error it returned.
func goOutput(t *testing.T, c *console.Console) (string, error) {
	t.Helper()

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	err := c.Debugger.Dispatch("GO")

	return buf.String(), err
}

// probeImage is the Phase 42 probe's image, built by VMS 7.3's MACRO and
// LINK with /DEBUG.
func probeImage(t *testing.T) string {
	t.Helper()

	return consoletest.RepoPath(t, "testdata", "dbgcmd", "vax", "dbgcmd.exe")
}

// TestRunDebugStopsAtMain: RUN/DEBUG of an image with debug symbols shows
// the debugger's start-up messages and stops, with the DBG> session open,
// at the main routine's first instruction after its entry mask, before
// any of the program has run (dbgdis.dlg's first lines: SHOW SCOPE says
// DBGDIS\START, and .PC is START's line 42).
func TestRunDebugStopsAtMain(t *testing.T) {
	c, out := runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOn})

	if !strings.Contains(out, "%DEBUG-I-INITIAL, Language: MACRO, Module: DBGDIS\n") {
		t.Errorf("RUN/DEBUG output:\n%s", out)
	}

	if strings.Contains(out, "Break at") || strings.Contains(out, "done") {
		t.Errorf("RUN/DEBUG ran or reported a break:\n%s", out)
	}

	if !c.InDebugger() {
		t.Error("no debugger session after RUN/DEBUG")
	}

	if pc := c.CPU.GPR(vax.PC); pc != 0x402 {
		t.Errorf("PC = %08X, want 00000402 (START's first instruction)", pc)
	}

	// The stop leaves no breakpoint of the debugger's behind.
	if n := len(dbgOf(c).Breakpoints); n != 0 {
		t.Errorf("%d breakpoints after the start-up stop, want none", n)
	}
}

// TestRunDefaultsFollowLinkFlag: with no /DEBUG qualifier the debugger
// starts for an image linked /DEBUG (dbgdis.exe) and RUN/NODEBUG runs it
// to the end, with no session. An image linked /NOTRACEBACK runs without
// the debugger even with RUN/DEBUG (docs/PHASE-42.md, subtask 1: VMS
// ran DBGNOTB silently).
func TestRunDefaultsFollowLinkFlag(t *testing.T) {
	c, out := runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{})
	if !c.InDebugger() || !strings.Contains(out, "INITIAL") {
		t.Errorf("RUN of a /DEBUG image: session %v, output:\n%s", c.InDebugger(), out)
	}

	c, out = runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOff})
	if c.InDebugger() || strings.Contains(out, "INITIAL") || !strings.Contains(out, "DBGDIS: done") {
		t.Errorf("RUN/NODEBUG: session %v, output:\n%s", c.InDebugger(), out)
	}

	c, out = runImage(t, dbgImagePath(t, "dbgnotb.exe"), console.RunOptions{Debug: console.DebugOn})
	if c.InDebugger() || strings.Contains(out, "INITIAL") || strings.Contains(out, "DEBUG") {
		t.Errorf("RUN/DEBUG of a /NOTRACEBACK image: session %v, output:\n%s", c.InDebugger(), out)
	}
}

// TestImageExitStatus: GO from the start runs the image to its end, and the
// debugger reports its exit status and stays open (dbgdis.dlg's last
// lines). After that there is no program to run: GO and STEP say
// %DEBUG-E-BADSTARTPC (except.dlg).
func TestImageExitStatus(t *testing.T) {
	c, _ := runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOn})

	out, err := goOutput(t, c)
	if err != nil {
		t.Fatalf("GO: %v", err)
	}

	want := "%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL, normal successful completion'\n"
	if !strings.HasSuffix(out, want) {
		t.Errorf("GO to the end:\n%s\nwant it to end with %q", out, want)
	}

	if !c.InDebugger() {
		t.Error("the session closed when the image exited")
	}

	for _, command := range []string{"GO", "STEP"} {
		err := c.Debugger.Dispatch(command)
		if err == nil || !strings.Contains(err.Error(), "BADSTARTPC, cannot start from PC 00000000") {
			t.Errorf("%s after the image exited: %v", command, err)
		}
	}
}

// TestUnhandledExceptionBreak: an access violation no handler takes is
// shown as VMS's catch-all shows it, and then the debugger breaks at the
// faulting instruction ("at", faillnk.dlg). GO lets the catch-all end the
// image with the condition as its status.
func TestUnhandledExceptionBreak(t *testing.T) {
	c, _ := runImage(t, dbgImagePath(t, "faillnk.exe"), console.RunOptions{Debug: console.DebugOn})

	out, err := goOutput(t, c)
	if err != nil {
		t.Fatalf("GO: %v", err)
	}

	if !strings.Contains(out, "%SYSTEM-F-ACCVIO, access violation") {
		t.Errorf("no ACCVIO message:\n%s", out)
	}

	if !strings.HasSuffix(out, "break on unhandled exception at FAILSUB\\SUB2\\%LINE 12\n") {
		t.Errorf("GO:\n%s\nwant the break at FAILSUB\\SUB2\\%%LINE 12", out)
	}

	// The message is shown once, however the program is continued.
	out, err = goOutput(t, c)
	if err != nil {
		t.Fatalf("GO after the break: %v", err)
	}

	if strings.Contains(out, "reason mask=00") {
		t.Errorf("the message was shown again:\n%s", out)
	}

	if !strings.Contains(out, "%DEBUG-I-EXITSTATUS, is '%SYSTEM-F-ACCVIO, access violation") {
		t.Errorf("GO after the break:\n%s\nwant the image's exit with ACCVIO", out)
	}
}

// TestUnhandledSignalBreak: a condition signaled with LIB$SIGNAL that no
// handler takes breaks "preceding" the instruction after the call
// (except.dlg's last stops). The probe's earlier signal is handled by its
// handler, so it doesn't break; the warning is continued, as the
// catch-all continues a non-severe condition, and the image then exits.
func TestUnhandledSignalBreak(t *testing.T) {
	c, _ := runImage(t, probeImage(t), console.RunOptions{Debug: console.DebugOn})

	out, err := goOutput(t, c)
	if err != nil {
		t.Fatalf("GO: %v", err)
	}

	if !strings.Contains(out, "%SYSTEM-W-ENDOFFILE, end of file") ||
		!strings.HasSuffix(out, "break on unhandled exception preceding DBGCMD\\START\\%LINE 48\n") {
		t.Fatalf("GO:\n%s\nwant ENDOFFILE and the break preceding DBGCMD\\START\\%%LINE 48", out)
	}

	out, err = goOutput(t, c)
	if err != nil {
		t.Fatalf("second GO: %v", err)
	}

	if !strings.Contains(out, "%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL, normal successful completion'") {
		t.Errorf("second GO:\n%s", out)
	}
}

// TestUnhandledOutsideImageDebug: an image run without the debugger is
// not stopped by an unhandled condition: the catch-all ends it as always.
func TestUnhandledOutsideImageDebug(t *testing.T) {
	c, out := runImage(t, dbgImagePath(t, "faillnk.exe"), console.RunOptions{Debug: console.DebugOff})

	if c.InDebugger() || strings.Contains(out, "break on") {
		t.Errorf("RUN/NODEBUG stopped for the condition: session %v\n%s", c.InDebugger(), out)
	}

	if !strings.Contains(out, "%SYSTEM-F-ACCVIO") {
		t.Errorf("no ACCVIO message:\n%s", out)
	}
}
