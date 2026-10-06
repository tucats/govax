package debugger_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the session oracle (docs/PHASE-42.md, subtask 16): it
// replays each debugger command file of the two probes (testdata/dbg and
// testdata/dbgcmd) through govax's debugger, over the same image, and
// compares what govax printed for every command with what the VMS 7.3
// debugger logged for it (the .dlg files), line by line.
//
// The earlier subtasks each tested the commands they built against the
// part of these logs they were about. This test is the whole-session check
// that nothing between those parts drifted: every command of every script,
// in order, including the ones no other test looks at (an error message, a
// SHOW whose output no one asserted).

// sessionCase is one probe session: a command file, the VMS log of running
// it on one image, and the image to run under govax's debugger.
type sessionCase struct {
	dir    string // the probe: "dbg" or "dbgcmd"
	script string // the command file, in testdata/<dir>
	log    string // the VMS session log, in testdata/<dir>/vax
	image  string // the image path, from the repository root
	source string // the directory holding the program's source files
}

// sessionCases lists every session the probes logged.
func sessionCases() []sessionCase {
	cases := []sessionCase{
		{"dbg", "dbgdis.dbg", "dbgdis.dlg", "testdata/dbg/vax/dbgdis.exe", "testdata/dbg"},
		{"dbg", "dbgdis.dbg", "gvdbgdis.dlg", "testdata/dbg/vax/gvdbgdis.exe", "testdata/dbg"},
		{"dbg", "dbgdis.dbg", "dbgtrc.dlg", "testdata/dbg/vax/dbgtrc.exe", "testdata/dbg"},
		{"dbg", "trace.dbg", "trdbglnk.dlg", "testdata/dbg/vax/trdbglnk.exe", "testdata/mar/list"},
		{"dbg", "trace.dbg", "trlnkdbg.dlg", "testdata/dbg/vax/trlnkdbg.exe", "testdata/mar/list"},
		{"dbg", "trace.dbg", "trdbgtrc.dlg", "testdata/dbg/vax/trdbgtrc.exe", "testdata/mar/list"},
		{"dbg", "trace.dbg", "gvtrace.dlg", "testdata/dbg/vax/gvtrace.exe", "testdata/mar/list"},
		{"dbg", "fail.dbg", "faillnk.dlg", "testdata/dbg/vax/faillnk.exe", "testdata/mar/list"},
		{"dbg", "forth.dbg", "forth.dlg", "testdata/dbg/vax/forth.exe", "testdata/mar"},
	}

	for _, name := range []string{"break", "brkcls", "call", "errors", "exam", "except", "step", "trace", "watch"} {
		cases = append(cases, sessionCase{"dbgcmd", name + ".dbg", name + ".dlg", "testdata/dbgcmd/vax/dbgcmd.exe", "testdata/dbgcmd"})
	}

	return cases
}

// sessionDiff is one command whose output differs from the log's.
type sessionDiff struct {
	n       int // the command's position among the script's commands
	command string
	got     string
	want    string
}

// logLines reads a session log's lines, each without its leading "!" (the
// log is the debugger's own echo of the session, every line of which is
// written as a comment so that the log can be read back as a command file).
func logLines(t *testing.T, dir, name string) []string {
	t.Helper()

	data, err := os.ReadFile(consoletest.RepoPath(t, "testdata", dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	var lines []string

	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		if l, ok := strings.CutPrefix(l, "!"); ok {
			lines = append(lines, strings.TrimRight(l, " "))
		}
	}

	return lines
}

// nearestEcho returns the index of the first log line at or after from that
// is the echo of command (a blank and the command's text), or -1.
func nearestEcho(lines []string, from int, command string) int {
	want := " " + strings.TrimRight(command, " ")

	for i := from; i < len(lines); i++ {
		if lines[i] == want {
			return i
		}
	}

	return -1
}

// alignScript finds, for each script command, the log line that echoes it.
// The log leaves out the echo of a command that fails, so a command whose
// echo can't be found, or whose nearest echo comes after a later command's,
// has none (-1), and its output is part of the command before it.
func alignScript(lines, script []string) []int {
	echoes := make([]int, len(script))
	pos := 0

	for i, command := range script {
		at := nearestEcho(lines, pos, command)

		// Is a command of the next few the one that this echo is for? Then
		// this command's echo is missing from the log.
		for k := 1; k <= 3 && at >= 0 && i+k < len(script); k++ {
			if script[i+k] == command {
				continue
			}

			if later := nearestEcho(lines, pos, script[i+k]); later >= 0 && later < at {
				at = -1
			}
		}

		echoes[i] = at

		if at >= 0 {
			pos = at + 1
		}
	}

	return echoes
}

// expectedOutput returns the log's output for each group of script commands:
// a command with an echo, and the commands after it that have none. The
// lines are those between the echo and the next group's, less the log's
// echo of the script's comments and blank lines (" !..."). groups holds the
// index of each group's first command.
func expectedOutput(lines []string, echoes []int) (groups []int, output [][]string) {
	for i, e := range echoes {
		if e >= 0 {
			groups = append(groups, i)
		}
	}

	for g, first := range groups {
		start := echoes[first] + 1
		end := len(lines)

		if g+1 < len(groups) {
			end = echoes[groups[g+1]]
		}

		var out []string

		for _, l := range lines[start:end] {
			// The echo of a comment line or a blank one, and the debugger's
			// note that it has finished reading the command file.
			if l == " " || l == "" || strings.HasPrefix(l, " !") || strings.HasPrefix(l, "%DEBUG-I-VERIFYICF") {
				continue
			}

			out = append(out, l)
		}

		output = append(output, out)
	}

	return groups, output
}

// govaxOutput runs one command and returns what govax printed, its error
// message (as the front end shows it, with a "%") included.
func govaxOutput(c *console.Console, command string) []string {
	out, err := sayErr(c, command)
	if err != nil && !vmserrors.MessageInhibited(err) {
		out += "%" + err.Error() + "\n"
	}

	var lines []string

	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if l = strings.TrimRight(l, " "); l != "" && !programOutput[l] {
			lines = append(lines, l)
		}
	}

	return lines
}

// programOutput is what the probe's programs print themselves. The VMS log
// holds the debugger's own output, and the program's went to the terminal,
// so these lines are left out of govax's.
var programOutput = map[string]bool{
	"DBGDIS: done":              true,
	"DBGCMD: done":              true,
	"TRACE: three routines ran": true,
}

// replaySession runs a session and returns the commands whose output
// differed from the log's.
func replaySession(t *testing.T, sc sessionCase) []sessionDiff {
	t.Helper()

	script := readScriptIn(t, sc.dir, sc.script)
	lines := logLines(t, sc.dir, sc.log)
	echoes := alignScript(lines, script)
	groups, want := expectedOutput(lines, echoes)

	c, _ := runImage(t, consoletest.RepoPath(t, filepath.FromSlash(sc.image)), console.RunOptions{Debug: console.DebugOn})
	noUserStep(c)
	say(t, c, `SET SOURCE "`+consoletest.RepoPath(t, filepath.FromSlash(sc.source))+`"`)

	var diffs []sessionDiff

	for g, first := range groups {
		last := len(script)
		if g+1 < len(groups) {
			last = groups[g+1]
		}

		// Commands before the first echoed one (the probe's set-up) run
		// for their effect.
		if g == 0 {
			for i := 0; i < first; i++ {
				govaxOutput(c, script[i])
			}
		}

		var got []string

		for i := first; i < last; i++ {
			got = append(got, govaxOutput(c, script[i])...)
		}

		if !sameLines(normalize(script[first], got), normalize(script[first], want[g])) {
			diffs = append(diffs, sessionDiff{first, script[first], strings.Join(got, "\n"), strings.Join(want[g], "\n")})
		}
	}

	return diffs
}

// vmsOnlyImages are the images the VMS debugger lists in SHOW IMAGE besides
// the program's: its own, and the run-time library the program links to.
// govax has none of them (its library routines are its own shims).
var vmsOnlyImages = map[string]bool{"DEBUG": true, "DBGSSISHR": true, "DBGTBKMSG": true, "LIBRTL": true}

// Patterns for the numbers that differ for a reason that isn't about the
// debugger: where VMS put its stack, and where its heap put a descriptor.
var (
	stackAddress             = regexp.MustCompile(`\b(7FE[0-9A-F]|800[0-9A-F])[0-9A-F]{4}\b`)
	sessionDescriptorAddress = regexp.MustCompile(`descriptor address: [0-9A-F]{8}`)
	moduleSize               = regexp.MustCompile(`(\s)\d+\.?\s*$`)
	bytesAllocated           = regexp.MustCompile(`bytes allocated: \d+`)
)

// normalize makes one command's output comparable. It leaves out what can't
// be the same (the numbers named above), the lines govax adds (its own
// access mode in SHOW MODE) or lacks (the VMS-only images in SHOW IMAGE),
// and the sizes SHOW MODULE and SHOW IMAGE print, which are the sizes of
// VMS's own tables.
func normalize(command string, lines []string) []string {
	out := make([]string, 0, len(lines))
	upper := strings.ToUpper(command)

	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "access mode:"):
			continue

		case strings.HasPrefix(upper, "SHOW IMAGE"):
			if f := strings.Fields(strings.TrimPrefix(l, "*")); len(f) > 0 && (vmsOnlyImages[f[0]] || f[0] == "total") {
				continue
			}

		case strings.HasPrefix(upper, "SHOW MODULE"):
			l = bytesAllocated.ReplaceAllString(l, "bytes allocated")
			if !strings.HasPrefix(l, "module name") && !strings.HasPrefix(l, "total") {
				l = moduleSize.ReplaceAllString(l, "$1")
			}
		}

		l = stackAddress.ReplaceAllString(l, "<stack>")
		l = sessionDescriptorAddress.ReplaceAllString(l, "descriptor address: <heap>")

		out = append(out, strings.TrimRight(l, " "))
	}

	return out
}

// sameLines reports whether two outputs have the same lines.
func sameLines(a, b []string) bool {
	return strings.Join(a, "\n") == strings.Join(b, "\n")
}

// readScriptIn is readScript for either probe's directory.
func readScriptIn(t *testing.T, dir, name string) []string {
	t.Helper()

	data, err := os.ReadFile(consoletest.RepoPath(t, "testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}

	var cmds []string

	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "!") {
			cmds = append(cmds, l)
		}
	}

	return cmds
}

// Why each difference from the VMS log is expected. A command that differs
// and isn't listed under one of these fails the test, and so does a listed
// one that no longer differs: the list is what is left to do, or to decide.
const (
	reasonWhen = "a WHEN condition's .LABEL is the contents at the address COUNT holds in VMS, and the " +
		"contents at COUNT in govax (condition.go says why), so the break at BUMP or CATCH differs, and " +
		"what the later GOs reach"

	reasonDispatcher = "at a break on an unhandled exception govax's program is paused inside the condition " +
		"dispatcher (an XFC), not at the faulting instruction, so .PC, SHOW CALLS, and the frame's register " +
		"names are the dispatcher's (docs/PHASE-42.md, subtask 5)"

	reasonHandlerCalls = "SHOW CALLS in a condition handler lacks the \"above condition handler called with " +
		"exception\" lines between its frame and the one that signaled"

	reasonCall = "CALL's %VAL argument and its \"value returned is\" message (Future features)"

	reasonStepException = "STEP, and what follows it, from a break on an exception: govax runs the paused " +
		"dispatcher (docs/PHASE-42.md: stepping into a handler is a Future feature)"

	reasonGlobalName = "VMS names a data address past the last label by the global constant GLIMIT " +
		"(GLIMIT+25D); govax by the label before it"

	reasonSymbolListing = "VMS lists only the routine named as the module for the TRACE images (docs/PHASE-42.md, " +
		"subtask 12)"

	reasonUnconfirmedMSG = "EVALUATE/ADDRESS of a string label: VMS shows the string's own address, govax the " +
		"descriptor's (unconfirmed)"

	reasonUserMode = "VMS ran the program in user mode, and govax's test console in kernel mode (PSL)"
)

// expectedDifferences are the commands, by session log and position among
// the script's commands, that differ from VMS's log, and why.
var expectedDifferences = map[string]map[int]string{
	"dbgdis.dlg":   {29: reasonGlobalName},
	"gvdbgdis.dlg": {29: reasonGlobalName},

	"trdbglnk.dlg": {5: reasonSymbolListing, 13: reasonUnconfirmedMSG},
	"trlnkdbg.dlg": {5: reasonSymbolListing},
	"trdbgtrc.dlg": {5: reasonSymbolListing},
	"gvtrace.dlg":  {5: reasonSymbolListing, 13: reasonUnconfirmedMSG},

	"faillnk.dlg": {
		2: "the ACCVIO message: VMS 7.3 prints \"reason mask=00\" and \"PSL=\" where govax's " +
			"(vmsdef's generated text) prints \"mask=02\" and \"PS=\", and the PSL is of another mode",
		3: reasonDispatcher, 4: reasonDispatcher, 7: reasonDispatcher, 8: reasonDispatcher,
	},

	"forth.dlg": {
		2: "FORTH's dictionary headers (F_ABORT_H and so on, labels in a data psect) are listed by govax " +
			"and not by VMS, which no probe explains (docs/PHASE-42.md, subtask 12)",
		4: "%LINE of a line with no code: VMS says %DEBUG-E-LINEINFO and the lines on either side, govax " +
			"says the name is undefined",
	},

	"break.dlg": {36: reasonWhen, 38: reasonWhen, 39: reasonWhen, 43: reasonWhen},

	"call.dlg": {
		2: reasonCall, 3: reasonCall, 6: reasonCall, 8: reasonCall, 9: reasonCall, 10: reasonCall,
		11: reasonCall, 12: reasonCall, 13: reasonCall, 15: reasonCall, 16: reasonCall,
	},

	"errors.dlg": {
		15: "EXAMINE/BYTE/WORD: VMS takes the last type qualifier; govax refuses two",
		18: "7FFFFFFF is mapped in govax's address space (the top of P1) and not in VMS's",
		20: reasonDispatcher,
	},

	"exam.dlg": {
		4:  "SHOW STACK's last frames: VMS's are the debugger's own (SHARE$DEBUG), govax's the console's",
		12: reasonUserMode, 13: reasonUserMode,
		77: "SET MODE NOLINE: a location is named by routine plus offset (FACT+1A); govax records the mode " +
			"and doesn't act on it (modes.go)",
	},

	"except.dlg": {
		3: reasonDispatcher, 4: reasonDispatcher, 6: reasonHandlerCalls, 8: reasonDispatcher, 10: reasonDispatcher,
	},

	"step.dlg": {
		26: reasonStepException, 29: reasonStepException, 30: reasonStepException, 33: reasonStepException,
		37: reasonStepException, 38: reasonStepException, 39: reasonStepException,
	},
}

// TestDebuggerSessionOracle replays every probe session and checks that the
// commands whose output differs from the VMS log's are exactly those listed
// in expectedDifferences. A listed difference is a gap, or a choice, that
// the reason names; one that disappeared is reported so it can be taken off
// the list.
func TestDebuggerSessionOracle(t *testing.T) {
	for _, sc := range sessionCases() {
		t.Run(sc.log, func(t *testing.T) {
			expected := expectedDifferences[sc.log]
			seen := map[int]bool{}

			for _, d := range replaySession(t, sc) {
				seen[d.n] = true

				if _, ok := expected[d.n]; !ok {
					t.Errorf("#%d %s differs from the VMS log:\n got:\n%s\nwant:\n%s", d.n, d.command, d.got, d.want)
				}
			}

			for n, reason := range expected {
				if !seen[n] {
					t.Errorf("#%d no longer differs from the VMS log; take it off the list (%s)", n, reason)
				}
			}
		})
	}
}
