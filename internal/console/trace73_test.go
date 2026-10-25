package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The trace fault end to end (docs/PHASE-73 - trace trap.md, subtask 2):
// a program that sets PSL<T> takes SS$_TBIT in its handler after each
// instruction.

// TestTrace73SelfTest runs testdata/ots72's ots72t.mar, the study of
// FORRTL's calls that traces into FORRTL, and checks its case 0, the
// tracer's self-test on the program's own JSB routine: the tracer
// catches the JSB's entry and its RSB (state 2), R2 is 12345678 at the
// RSB, and the routine has stored 87654321 in the longword below R1.
// The other cases need VMS's FORRTL.EXE, which can't be committed.
// Registers and frames that hold stack addresses differ from VMS's run
// (vax/ots72t-vax.txt); what the self-test computes doesn't.
func TestTrace73SelfTest(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(ots72Dir, "ots72t.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	vax, err := os.ReadFile(filepath.Join(ots72Dir, "vax", "ots72t-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	got := selfTestLines(strings.ReplaceAll(out, "\r", ""))
	want := selfTestLines(string(vax))

	if len(got) == 0 || len(got) != len(want) {
		t.Fatalf("the self-test printed %d lines, VMS %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}

	// Lines that hold no stack addresses match VMS's whole.
	for i, w := range want {
		switch {
		case i == 0, strings.HasPrefix(w, "  tracer state:"), strings.HasPrefix(w, "  value at R0:"):
			if got[i] != w {
				t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, got[i], w)
			}
		}
	}

	// The RSB's registers: R2 (the third value) is what the routine set.
	rsb := lineStarting(got, "  RSB R0-R11 AP FP SP:")
	if f := strings.Fields(rsb); len(f) < 8 || f[7] != "12345678" {
		t.Errorf("RSB registers %q; want R2 12345678", rsb)
	}

	// The longword below R1, the last of the frame's -16 line.
	frame := lineStarting(got, "  RSB frame -16:")
	if f := strings.Fields(frame); len(f) == 0 || f[len(f)-1] != "87654321" {
		t.Errorf("RSB frame %q; want 87654321 last", frame)
	}
}

// selfTestLines returns the self-test case's lines from the program's
// output: its own line and the indented lines after it.
func selfTestLines(out string) []string {
	var lines []string

	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "tracer self-test "):
			lines = append(lines, l)
		case len(lines) > 0 && strings.HasPrefix(l, " "):
			lines = append(lines, l)
		case len(lines) > 0:
			return lines
		}
	}

	return lines
}

// lineStarting returns the first of lines that starts with prefix, or "".
func lineStarting(lines []string, prefix string) string {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}

	return ""
}

// TestTrace73Unhandled: a program that sets PSL<T> with no handler for
// SS$_TBIT ends at the first trace, with the condition's message and the
// traceback handler's dump (SS$_TBIT is severe), and the image's status
// SS$_TBIT. Tracing begins with the second instruction after the
// BISPSW: the first NOP runs, and the trace comes before the second.
func TestTrace73Unhandled(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tBISPSW\t#^X10\n"+
		"\tNOP\n"+
		"\tNOP\n"+
		"\tRET\n"+
		"\t.END\tSTART\n")

	if r0&0x0FFFFFFF != 0x464 { // SS$_TBIT
		t.Errorf("R0 = %#x, want SS$_TBIT (0x464)", r0)
	}

	if !strings.HasPrefix(out, "%SYSTEM-F-TBIT, T-bit pending trap at PC=00000205, PSL=03C00010\n") {
		t.Errorf("output:\n%s", out)
	}

	if !strings.Contains(out, "%TRACE-F-TRACEBACK, symbolic stack dump follows") {
		t.Errorf("no traceback dump in:\n%s", out)
	}
}
