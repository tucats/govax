package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
)

// The LIB$ routines' failures that are signals rather than statuses
// (docs/PHASE-53, subtask 4): an argument a routine can't reach is an
// access violation, as on VMS, and a routine govax lacks signals
// SS$_UNSUPPORTED when called, so an image that names one still runs.

// runMacroProgram assembles, links, and runs source as one module,
// returning R0 and what it wrote (after the kernel's own boot message).
func runMacroProgram(t *testing.T, source string) (uint32, string) {
	t.Helper()

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()
	path := filepath.Join(dir, "prog")

	writeHostFile(t, path+".mar", source)

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar"}); err != nil {
		t.Fatalf("MACRO: %v\n%s", err, out.String())
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	r0 := runImage(t, c, path+".exe")
	_, after, _ := strings.Cut(out.String(), "Kernel initialized\n\r")

	return r0, after
}

// TestLibrtlMissingRoutine: an image calling LIB$GETQUI, which govax
// doesn't have (no queue manager), links and runs; the call signals SS$_UNSUPPORTED with
// the routine's name, which the catch-all shows, and, the condition being
// an error rather than a severe one, returns SS$_UNSUPPORTED.
func TestLibrtlMissingRoutine(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n\tCALLS\t#0,G^LIB$GETQUI\n\tRET\n\t.END\tSTART\n")

	if r0 != 0xE4A { // SS$_UNSUPPORTED
		t.Errorf("R0 = %#x, want SS$_UNSUPPORTED (0xE4A)", r0)
	}

	want := "%SYSTEM-E-UNSUPPORTED, unsupported operation or function\n" +
		"-GOVAX-E-NOTIMPL, routine LIB$GETQUI is not implemented in govax\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestLibrtlAccessViolation: LIB$ADAWI given an address it can't read, or
// one it can't write, signals SS$_ACCVIO with the hardware's reason mask
// and address; no handler takes it, and the image ends with it.
func TestLibrtlAccessViolation(t *testing.T) {
	cases := []struct {
		name, add, sign, mask, va string
	}{
		// Beyond P0's length: a length violation (reason bit 0).
		{"read", "PUSHL\t#^X20000000", "PUSHAL\tSIGN", "01", "20000000"},
		// The code psect is read-only: a write (reason bit 2).
		{"write", "PUSHAL\tADD", "PUSHAB\tSTART", "04", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
				"ADD:\t.LONG\t1\nSUM:\t.LONG\t2\nSIGN:\t.LONG\t0\n"+
				"\t.PSECT\tCODE,NOWRT,EXE\n"+
				"\t.ENTRY\tSTART,^M<>\n\t"+tc.sign+"\n\tPUSHAL\tSUM\n\t"+tc.add+"\n"+
				"\tCALLS\t#3,G^LIB$ADAWI\n\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

			if r0 != 0x1000000C { // SS$_ACCVIO, message shown
				t.Errorf("R0 = %#x, want SS$_ACCVIO with the message inhibited (0x1000000C)", r0)
			}

			prefix := "%SYSTEM-F-ACCVIO, access violation, reason mask=" + tc.mask + ", virtual address=" + tc.va
			if !strings.HasPrefix(out, prefix) {
				t.Errorf("output %q, want it to start %q", out, prefix)
			}
		})
	}
}

// TestLibrtlAccessViolationContinued: a handler that continues the
// access violation makes the routine return SS$_ACCVIO.
func TestLibrtlAccessViolationContinued(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tHANDLER,^M<>\n\tMOVL\t#1,R0\n\tRET\n"+ // SS$_CONTINUE
		"\t.ENTRY\tSTART,^M<>\n\tMOVAB\tHANDLER,(FP)\n"+
		"\tPUSHL\t#0\n\tPUSHL\t#0\n\tPUSHL\t#^X20000000\n"+
		"\tCALLS\t#3,G^LIB$ADAWI\n\tRET\n\t.END\tSTART\n")

	if r0 != 0xC {
		t.Errorf("R0 = %#x, want SS$_ACCVIO (0xC)", r0)
	}

	if out != "" {
		t.Errorf("output %q, want none: the handler took the condition", out)
	}
}
