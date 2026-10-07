package debugger_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// stripAccessMode removes SHOW MODE's last line, govax's own "access mode:"
// (the VMS debugger prints nothing like it), so the rest can be compared
// with VMS's.
func stripAccessMode(out string) string {
	var kept []string

	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, "access mode:") {
			kept = append(kept, l)
		}
	}

	return strings.Join(kept, "\n")
}

// maskStackAddresses makes a line of SHOW STACK's output comparable with
// the one VMS printed: VMS's stack is at 7FED...., and govax's is
// elsewhere, so a line with such an address is compared up to where it
// starts.
func maskStackAddresses(want, got string) (string, string) {
	if at := strings.Index(want, "7FED"); at >= 0 && len(got) >= at {
		return want[:at], got[:at]
	}

	return want, got
}

// TestMachineStateOracle replays the probe's exam.dbg under the debugger
// and compares the commands subtask 11 moved: SHOW MODE and SHOW RADIX
// after each SET and CANCEL of a mode or radix (every line but govax's
// access mode), SHOW CALLS four frames deep, and SHOW STACK's frames.
func TestMachineStateOracle(t *testing.T) {
	entries := readSessionLog(t, "dbgcmd", "exam.dlg")
	script := readScript(t, "exam.dbg")
	c := stepSession(t)

	next, compared := 0, 0

	for n, e := range entries {
		at := -1

		for i := next; i < len(script); i++ {
			if script[i] == e.command {
				at = i

				break
			}
		}

		if at < 0 {
			continue
		}

		for ; next < at; next++ {
			_, _ = sayErr(c, script[next])
		}

		next = at + 1

		out, err := sayErr(c, e.command)

		cmd := strings.ToUpper(e.command)
		if !strings.HasPrefix(cmd, "SHOW MODE") && !strings.HasPrefix(cmd, "SHOW RADIX") &&
			!strings.HasPrefix(cmd, "SHOW CALLS") && !strings.HasPrefix(cmd, "SHOW STACK") {
			continue
		}

		if err != nil {
			t.Errorf("%s: %v", e.command, err)

			continue
		}

		compared++

		want := e.output

		// The log's heading of SHOW CALLS' table starts like a command's
		// echo, so the reader takes it for one, and the rows are the
		// output of that "command".
		if strings.HasPrefix(cmd, "SHOW CALLS") && n+1 < len(entries) {
			want = append([]string{" " + entries[n+1].command}, entries[n+1].output...)
		}

		got := strings.Split(strings.TrimSuffix(stripAccessMode(out), "\n"), "\n")

		// The log's blank lines are the ones in the middle of SHOW
		// STACK, which the log reader drops ("!" with nothing after it).
		got = nonBlank(got)

		// The log's last frame is VMS's own debugger calling the image,
		// which govax has no counterpart of, so only the frames before
		// it are compared.
		if strings.HasPrefix(cmd, "SHOW STACK") {
			for i, l := range want {
				if strings.HasPrefix(l, "stack frame 2") {
					want = want[:i]

					break
				}
			}

			if len(got) > len(want) {
				got = got[:len(want)]
			}
		}

		if len(got) != len(want) {
			t.Errorf("%s: got %d lines, want %d:\n%s", e.command, len(got), len(want), strings.Join(got, "\n"))

			continue
		}

		for i := range want {
			w, g := maskStackAddresses(want[i], got[i])
			if w != g {
				t.Errorf("%s line %d:\n got %q\nwant %q", e.command, i+1, got[i], want[i])
			}
		}
	}

	if compared < 10 {
		t.Errorf("only %d commands compared; is the log being read?", compared)
	}
}

// nonBlank drops the empty lines, which the session log reader leaves out
// (a line of blanks it keeps).
func nonBlank(lines []string) []string {
	var kept []string

	for _, l := range lines {
		if l != "" {
			kept = append(kept, l)
		}
	}

	return kept
}

// TestShowStackLayout: SHOW STACK's blank lines and the layout of a
// frame's lines (the log reader drops the blanks, so the oracle can't
// check them): two blank lines before the first frame, one before each
// later frame, one after a frame's header, and a line of 18 blanks ending
// the argument list.
func TestShowStackLayout(t *testing.T) {
	c := stepSession(t)

	say(t, c, "SET BREAK/AFTER:3 BACK")
	say(t, c, "GO")
	say(t, c, "CANCEL BREAK/ALL")

	out := say(t, c, "SHOW STACK 2")
	lines := strings.Split(out, "\n")

	if lines[0] != "" || lines[1] != "" || !strings.HasPrefix(lines[2], "stack frame 0 (") || lines[3] != "" {
		t.Fatalf("frame 0's header:\n%q", lines[:5])
	}

	if !strings.HasPrefix(lines[4], "    condition handler: 00000000") {
		t.Errorf("first field %q", lines[4])
	}

	// Frame 1 follows the 18 blanks and one more blank line.
	for i, l := range lines {
		if strings.HasPrefix(l, "stack frame 1 (") {
			if lines[i-1] != "" || lines[i-2] != strings.Repeat(" ", 18) {
				t.Errorf("before frame 1: %q, %q", lines[i-2], lines[i-1])
			}

			return
		}
	}

	t.Errorf("no frame 1 in:\n%s", out)
}

// TestShowModeAndCancel: the modes line follows SET MODE, abbreviations
// work, an ambiguous one is refused, and CANCEL MODE restores them.
func TestShowModeAndCancel(t *testing.T) {
	c := stepSession(t)

	first := stripAccessMode(say(t, c, "SHOW MODE"))
	want := "modes: symbolic, line, d_float, noscreen, scroll, nokeypad, dynamic, interrupt, no separate window\n" +
		"input radix : hexadecimal\noutput radix: hexadecimal\n"

	expect(t, "SHOW MODE", first, want)

	say(t, c, "SET MODE NOSYM,NOLINE,G_FLOAT,NOSCROLL,OPERANDS=FULL")

	got := stripAccessMode(say(t, c, "SHOW MODE"))
	if !strings.HasPrefix(got, "modes: nosymbolic, noline, g_float, full operands, noscreen, noscroll,") {
		t.Errorf("after SET MODE: %q", got)
	}

	say(t, c, "CANCEL MODE")
	expect(t, "SHOW MODE", stripAccessMode(say(t, c, "SHOW MODE")), want)

	// "S" abbreviates SYMBOLIC, SCROLL, SUPERVISOR, and more.
	if _, err := sayErr(c, "SET MODE S"); !errors.Is(err, vmserrors.New(vmserrors.CLI_AMBIGUOUS)) {
		t.Errorf("SET MODE S: %v, want an ambiguous keyword", err)
	}

	// Screen mode isn't available; a made-up word isn't a mode.
	for _, bad := range []string{"SET MODE SCREEN", "SET MODE NOSUCH", "SET MODE SYMBOLIC=1"} {
		if _, err := sayErr(c, bad); !errors.Is(err, vmserrors.New(vmserrors.DBG_SYNTAX)) {
			t.Errorf("%s: %v, want %%DEBUG-E-SYNTAX", bad, err)
		}
	}
}

// TestSetAccessMode: SET MODE takes govax's access modes beside the
// display modes, and SHOW MODE ends with the current one.
func TestSetAccessMode(t *testing.T) {
	c := stepSession(t)

	say(t, c, "SET MODE USER,NOSYMBOLIC")

	out := say(t, c, "SHOW MODE")
	if !strings.HasSuffix(out, "access mode: USER\n") || !strings.HasPrefix(out, "modes: nosymbolic,") {
		t.Errorf("SHOW MODE:\n%s", out)
	}

	say(t, c, "SET MODE KERNEL")

	if out := say(t, c, "SHOW MODE"); !strings.HasSuffix(out, "access mode: KERNEL\n") {
		t.Errorf("SHOW MODE:\n%s", out)
	}
}

// TestMachineShowAndSet: the commands that moved from the console keep
// their output, and read numbers in the debugger's radix.
func TestMachineShowAndSet(t *testing.T) {
	c, _ := newTestConsole(t)

	// A register, by name and in the table.
	say(t, c, "DEPOSIT R3 = 1234")

	if out := say(t, c, "SHOW R3"); out != "R3   = 00001234\n" {
		t.Errorf("SHOW R3: %q", out)
	}

	if out := say(t, c, "SHOW REGISTERS"); !strings.Contains(out, "00001234") {
		t.Errorf("SHOW REGISTERS:\n%s", out)
	}

	// SET BASE and SHOW BASE: the address is read in hexadecimal.
	say(t, c, "SET BASE 400")

	if out := say(t, c, "SHOW BASE"); !strings.Contains(out, "Next storage address is 00000400") {
		t.Errorf("SHOW BASE: %q", out)
	}

	// SET PSL, with a value in the input radix.
	say(t, c, "SET RADIX DECIMAL")
	say(t, c, "SET PSL IPL=20")

	if out := say(t, c, "SHOW PSL"); !strings.Contains(out, "14") {
		t.Errorf("SHOW PSL after IPL=20 (decimal):\n%s", out)
	}

	// The stack dumps and the others take nothing, or a count.
	for _, cmd := range []string{
		"SHOW SP", "SHOW KSP 2", "SHOW CPU_STATUS", "SHOW TB", "SHOW MEMORY", "SHOW SCB",
		"SHOW EXCEPTIONS", "SHOW REGIONS", "SHOW CLOCK",
	} {
		if _, err := sayErr(c, cmd); err != nil {
			t.Errorf("%s: %v", cmd, err)
		}
	}

	// CANCEL, and CLEAR as its synonym.
	for _, cmd := range []string{"CANCEL TB", "CLEAR TB", "CANCEL INTERRUPT/ALL"} {
		if _, err := sayErr(c, cmd); err != nil {
			t.Errorf("%s: %v", cmd, err)
		}
	}

	// SET VM and SET NOVM.
	for _, cmd := range []string{"SET NOVM", "SET HISTORY 4"} {
		if _, err := sayErr(c, cmd); err != nil {
			t.Errorf("%s: %v", cmd, err)
		}
	}
}

// TestShowCallsAfterExit: with no program, SHOW CALLS and SHOW STACK have
// no frames to show (errors.dlg).
func TestShowCallsAfterExit(t *testing.T) {
	c := stepSession(t)

	// The probe's program signals an unhandled condition on its way, so
	// the first GO stops there and the second runs it out.
	say(t, c, "GO")
	say(t, c, "GO")

	for _, cmd := range []string{"SHOW CALLS", "SHOW STACK"} {
		if _, err := sayErr(c, cmd); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOCALLS)) {
			t.Errorf("%s: %v, want %%DEBUG-E-NOCALLS", cmd, err)
		}
	}
}
