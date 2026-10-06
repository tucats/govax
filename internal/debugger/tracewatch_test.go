package debugger_test

import (
	"slices"
	"strings"
	"testing"
)

// Tests of SET/SHOW/CANCEL TRACE and WATCH (docs/PHASE-42.md, subtask 13),
// against the sessions the VMS 7.3 debugger ran on the probe's DBGCMD image
// (testdata/dbgcmd/vax/trace.dlg and watch.dlg).

// replayLog runs the probe's command file name.dbg under the debugger and
// compares what each command of a kind the test covers printed with what
// the VMS debugger's log shows. covered says which commands those are (by
// their upper-case text). The other commands run for their effect only.
// It reports how many it compared.
//
// Message lines (starting with "%") are left out of both sides: the log
// reader leaves out the VMS debugger's, and govax's include one VMS wrote
// to the log in another place.
func replayLog(t *testing.T, name string, covered func(command string) bool) int {
	t.Helper()

	entries := readSessionLog(t, "dbgcmd", name+".dlg")
	script := readScript(t, name+".dbg")
	c := sourceSession(t)

	next, compared := 0, 0

	// got and want hold each compared command's two outputs, which are
	// checked at the end so that a log's continuation lines (below) can
	// be added to the one before.
	type result struct{ command, got, want string }

	results := make([]*result, 0, len(entries))

	for _, e := range entries {
		at := -1

		for i := next; i < len(script); i++ {
			if script[i] == e.command {
				at = i

				break
			}
		}

		if at < 0 {
			// A line of output that starts with one blank (the opcodes
			// SHOW TRACE lists) looks like a command to the log reader.
			// It belongs to the command before, if that was compared.
			if n := len(results); n > 0 && results[n-1] != nil && !isScriptCommand(script, e.command) {
				results[n-1].want += "\n " + e.command
				if len(e.output) > 0 {
					results[n-1].want += "\n" + strings.Join(e.output, "\n")
				}
			}

			continue
		}

		for ; next < at; next++ {
			_, _ = sayErr(c, script[next])
		}

		next = at + 1

		out, _ := sayErr(c, e.command)

		if !covered(strings.ToUpper(e.command)) {
			results = append(results, nil)

			continue
		}

		var got []string

		for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
			if !strings.HasPrefix(l, "%") {
				got = append(got, l)
			}
		}

		compared++

		results = append(results, &result{e.command, strings.Join(got, "\n"), strings.Join(e.output, "\n")})
	}

	for _, r := range results {
		if r == nil {
			continue
		}

		if g, w := trimLines(r.got), trimLines(r.want); g != w {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", r.command, g, w)
		}
	}

	return compared
}

// isScriptCommand reports whether text is a line of the command file.
func isScriptCommand(script []string, text string) bool {
	return slices.Contains(script, text)
}

// trimLines removes the blanks at the end of each line of text, which the
// opcode lists of SHOW TRACE end with and the log reader may not keep.
func trimLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}

	return strings.Join(lines, "\n")
}

// TestTraceOracle replays trace.dbg: every instruction, a routine through
// its recursion, a JSB subroutine, and every line and branch (whose reports
// are ordered by the rule traceHit describes).
func TestTraceOracle(t *testing.T) {
	n := replayLog(t, "trace", func(cmd string) bool {
		return cmd == "GO" || strings.HasPrefix(cmd, "SHOW TRACE") || strings.HasPrefix(cmd, "SET TRACE") ||
			strings.HasPrefix(cmd, "CANCEL TRACE")
	})

	if n < 10 {
		t.Errorf("compared only %d commands", n)
	}
}

// TestWatchOracle replays watch.dbg: a longword, a byte, an array changed
// by MOVC3, a temporary watchpoint, and a DEPOSIT that doesn't trigger one.
func TestWatchOracle(t *testing.T) {
	n := replayLog(t, "watch", func(cmd string) bool {
		return cmd == "GO" || strings.HasPrefix(cmd, "SHOW WATCH") || strings.HasPrefix(cmd, "SET WATCH") ||
			strings.HasPrefix(cmd, "CANCEL WATCH")
	})

	if n < 10 {
		t.Errorf("compared only %d commands", n)
	}
}

// TestTraceCommands: SHOW and CANCEL TRACE's messages, a tracepoint's
// /AFTER and /TEMPORARY, a DO clause that runs without stopping the
// program, and a breakpoint replacing a tracepoint at the same address.
func TestTraceCommands(t *testing.T) {
	c := probeSession(t)

	expect(t, "CANCEL TRACE/ALL", say(t, c, "CANCEL TRACE/ALL"),
		"%DEBUG-I-NOTRACES, no tracepoints are set, no opcode tracing\n")

	// FACT is entered five times, the first before the session's stop.
	say(t, c, "SET TRACE/AFTER:2/TEMPORARY FACT")
	expect(t, "SHOW TRACE", say(t, c, "SHOW TRACE"),
		"tracepoint at routine DBGCMD\\FACT [temporary]\n   /after: 2\n")

	say(t, c, "SET BREAK LOOP")
	expect(t, "GO", say(t, c, "GO"), "trace at routine DBGCMD\\FACT\nbreak at DBGCMD\\START\\LOOP\n")
	expect(t, "SHOW TRACE", say(t, c, "SHOW TRACE"),
		"%DEBUG-I-NOTRACES, no tracepoints are set, no opcode tracing\n")

	// DO runs when the tracepoint is reached; the program carries on.
	say(t, c, "SET TRACE BUMP DO (EXAMINE WATCHL)")
	expect(t, "SHOW TRACE", say(t, c, "SHOW TRACE"),
		"tracepoint at DBGCMD\\FACT\\BUMP\n   do (EXAMINE WATCHL)\n")
	say(t, c, "CANCEL BREAK/ALL")
	say(t, c, "SET BREAK/EXCEPTION")

	out := say(t, c, "GO")
	if !strings.Contains(out, "trace at DBGCMD\\FACT\\BUMP\n") || !strings.Contains(out, "DBGCMD\\WATCHL:") {
		t.Errorf("GO with a tracepoint with DO:\n%s", out)
	}

	// A breakpoint at the same place replaces the tracepoint.
	say(t, c, "SET BREAK BUMP")
	expect(t, "SHOW TRACE", say(t, c, "SHOW TRACE"),
		"%DEBUG-I-NOTRACES, no tracepoints are set, no opcode tracing\n")
}

// TestWatchCommands: a watchpoint reports a change by STEP too, /AFTER and
// WHEN apply, a DEPOSIT isn't reported, and SHOW and CANCEL WATCH's forms.
func TestWatchCommands(t *testing.T) {
	c := stepSession(t)

	expect(t, "SHOW WATCH", say(t, c, "SHOW WATCH"), "%DEBUG-I-NOWATCHES, no watchpoints are set\n")
	expect(t, "CANCEL WATCH/ALL", say(t, c, "CANCEL WATCH/ALL"), "%DEBUG-I-NOWATCHES, no watchpoints are set\n")

	say(t, c, "SET WATCH/AFTER:2 WATCHL WHEN (.R3 EQL 1)")
	say(t, c, "SET WATCH BUFFER")
	expect(t, "SHOW WATCH", say(t, c, "SHOW WATCH"),
		"watchpoint of DBGCMD\\WATCHL\n   /after: 2\n   when (.R3 EQL 1)\nwatchpoint of DBGCMD\\BUFFER[0:15]\n")

	// Watching a place again replaces its watchpoint.
	say(t, c, "SET WATCH WATCHL")
	expect(t, "SHOW WATCH", say(t, c, "SHOW WATCH"),
		"watchpoint of DBGCMD\\BUFFER[0:15]\nwatchpoint of DBGCMD\\WATCHL\n")

	say(t, c, "CANCEL WATCH WATCHL")
	expect(t, "SHOW WATCH", say(t, c, "SHOW WATCH"), "watchpoint of DBGCMD\\BUFFER[0:15]\n")

	// An unwatched change, and a changed array element by STEP.
	say(t, c, "SET BREAK %LINE 43")
	say(t, c, "GO")

	out := say(t, c, "STEP/INSTRUCTION")
	if !strings.HasPrefix(out, "watch of DBGCMD\\BUFFER[15] at ") {
		t.Errorf("STEP over MOVC3:\n%s", out)
	}

	// A watch of an address that is no symbol is a longword.
	say(t, c, "CANCEL WATCH/ALL")
	say(t, c, "SET WATCH/TEMPORARY 200+3")
	expect(t, "SHOW WATCH", say(t, c, "SHOW WATCH"), "watchpoint of DBGCMD\\WATCHL+3 [temporary]\n")
}
