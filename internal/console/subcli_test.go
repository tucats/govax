package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 48: the subprocess CLI (subcli.go). A $CREPRC of LOGINOUT.EXE
// makes a process that runs the CLI, reading its commands from its
// SYS$INPUT; LIB$SPAWN's tests (spawn_test.go) give it a command.

// echoSource is an image that writes its foreign command's text
// (LIB$GET_FOREIGN) on SYS$OUTPUT and returns SS$_NORMAL.
const echoSource = `	.title	echo
	.psect	data,noexe,wrt
buf:	.blkb	80
desc:	.word	80
	.byte	14,1
	.address buf
len:	.word	0
out:	.word	0
	.byte	14,1
	.address buf
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaw	len
	clrl	-(sp)
	pushaq	desc
	calls	#3,g^lib$get_foreign
	movw	len,out
	pushaq	out
	calls	#1,g^lib$put_output
	movl	#1,r0
	ret
	.end	start
`

// abortSource is an image that ends with SS$_ABORT.
const abortSource = `	.title	abort
	.psect	code,exe,nowrt
	.entry	start,^m<>
	movl	#^x2C,r0
	ret
	.end	start
`

// debugTrace is a DEBUG trace line, which the vax.init debug settings
// write.
var debugTrace = regexp.MustCompile(`DEBUG\([A-Z]+\): [^\n]*\n?`)

// programLines are the lines of output that aren't DEBUG traces, with
// carriage returns dropped.
func programLines(out string) []string {
	var lines []string

	// A trace line can follow a prompt on the same line.
	out = debugTrace.ReplaceAllString(strings.ReplaceAll(out, "\r", ""), "")

	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

// splitLogoutReport splits a LOGINOUT job's output lines (programLines,
// so with no blank lines) at LOGOUT's report.
func splitLogoutReport(lines []string) (before, report []string) {
	for i, line := range lines {
		if strings.HasPrefix(line, "  SYSTEM       job terminated at ") {
			return lines[:i], lines[i:]
		}
	}

	return lines, nil
}

// checkLogoutReport checks LOGOUT's report for a job that isn't
// interactive (corevms's LogoutReport) as programLines leaves it: VMS's
// lines, less the blank one, with govax's counts.
func checkLogoutReport(t *testing.T, report []string) {
	t.Helper()

	want := []string{
		"  SYSTEM       job terminated at ",
		"  Accounting information:",
		"  Buffered I/O count:               0         Peak working set size:       0",
		"  Direct I/O count:                 0         Peak page file size:         0",
		"  Page faults:                      0         Mounted volumes:             0",
		"  Charged CPU time:           ",
	}

	if len(report) != len(want) {
		t.Errorf("LOGOUT's report:\n%s", strings.Join(report, "\n"))

		return
	}

	for i, w := range want {
		if !strings.HasPrefix(report[i], w) {
			t.Errorf("LOGOUT's report, line %d: %q, want %q...", i+1, report[i], w)
		}
	}
}

// loginout creates a process running LOGINOUT (the CLI) with SYS$INPUT
// input, runs the machine until it has been deleted, and returns it.
func loginout(t *testing.T, c *console.Console, input string) *corevms.Environment {
	t.Helper()

	one := c.RTL

	child, st := one.CreateProcess(corevms.CreateRequest{
		Image: "SYS$SYSTEM:LOGINOUT.EXE", Input: input, BasePriority: one.Process.BasePriority,
	})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()
	runUntil(t, c, 2_000_000, func() bool { return child.Deleted })

	return child
}

// writeCommands writes lines as a host command file and returns its path.
func writeCommands(t *testing.T, lines ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "commands.com")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestCLI_commandFile: LOGINOUT's CLI reads a command file as its
// SYS$INPUT and runs each command in turn, in the one process: RUN, a
// comment, an unknown verb (DCL's message, and the CLI goes on), a
// symbol assignment and the foreign command it defines (its text, as
// DCL treats it, reaching LIB$GET_FOREIGN), a failing image (its status's
// message), an alias, and LOGOUT, after which nothing more runs. Its
// input being a file, it echoes each line it reads (DCL's verify), and
// LOGOUT writes the job's report. The process ends with the last
// command's status.
func TestCLI_commandFile(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	child := buildChildImage(t, c)
	echo := buildImage(t, c, "echo", echoSource)
	abort := buildImage(t, c, "abort", abortSource)

	commands := writeCommands(t,
		"$ ! a comment",
		`$ RUN "`+child+`"`,
		"$ FROBNICATE",
		`$ ECHO :== "$`+echo+`"`,
		"$ ECHO some   text",
		`$ RUN "`+abort+`"`,
		`$ SAY*IT :== ECHO said`,
		"$ SAY it again",
		"$ LOGOUT",
		`$ RUN "`+child+`"`,
	)

	out.Reset()

	cli := loginout(t, c, commands)

	want := []string{
		"$ ! a comment",
		`$ RUN "` + child + `"`,
		"Hello from the child",
		"$ FROBNICATE",
		"%DCL-W-IVVERB, unrecognized command verb - check validity and spelling",
		` \FROBNICATE\`,
		`$ ECHO :== "$` + echo + `"`,
		"$ ECHO some   text",
		"SOME TEXT",
		`$ RUN "` + abort + `"`,
		"%SYSTEM-F-ABORT, abort",
		`$ SAY*IT :== ECHO said`,
		"$ SAY it again",
		"SAID IT AGAIN",
		"$ LOGOUT",
	}

	got, report := splitLogoutReport(programLines(out.String()))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	checkLogoutReport(t, report)

	if cli.Process.ExitStatus != 1 {
		t.Errorf("final status %08X, want the last image's, 1", cli.Process.ExitStatus)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes left, want only process 1", n)
	}
}

// TestCLI_terminal: with no SYS$INPUT of its own, LOGINOUT's CLI reads
// the terminal it shares with process 1, prompting with "$ ", until the
// end of the input; EXIT's status becomes the final status.
func TestCLI_terminal(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	child := buildChildImage(t, c)

	c.In = strings.NewReader(`RUN "` + child + "\"\nEXIT 7\n")

	out.Reset()

	cli := loginout(t, c, "")

	// An interactive job's LOGOUT writes one line (unconfirmed).
	const want = "$ Hello from the child\n$   SYSTEM       logged out at "
	if got := strings.Join(programLines(out.String()), "\n"); !strings.HasPrefix(got, want) || strings.Count(got, "\n") != 1 {
		t.Errorf("output %q, want the prompt, the child's line, the prompt for EXIT, and LOGOUT's line", got)
	}

	if cli.Process.ExitStatus != 7 {
		t.Errorf("final status %08X, want EXIT's 7", cli.Process.ExitStatus)
	}
}

// TestCLI_missingInput: a SYS$INPUT that isn't there leaves the CLI
// nothing to run: the process logs out at once with RMS$_FNF.
func TestCLI_missingInput(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)

	cli := loginout(t, c, filepath.Join(t.TempDir(), "nosuch.com"))

	if want := vmsdef.Symbols["RMS$_FNF"]; cli.Process.ExitStatus != want {
		t.Errorf("final status %08X, want RMS$_FNF (%08X)", cli.Process.ExitStatus, want)
	}
}

// TestCLI_runMissingImage: RUN of an image that isn't there shows DCL's
// activation message and IMAGEFNF's, with the image's name as RMS expands
// it (here a host file, with no volume mounted), and the CLI goes on; the
// status is CLI$_IMAGEFNF with STS$M_INHIB_MSG, as on VMS
// (testdata/mp/probe4, step 4).
func TestCLI_runMissingImage(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	child := buildChildImage(t, c)
	commands := writeCommands(t, "RUN NOSUCH", `RUN "`+child+`"`)

	out.Reset()

	cli := loginout(t, c, commands)

	got, report := splitLogoutReport(programLines(out.String()))
	want := []string{
		"RUN NOSUCH",
		"%DCL-W-ACTIMAGE, error activating image NOSUCH",
		"-CLI-E-IMAGEFNF, image file not found NOSUCH.EXE;",
		`RUN "` + child + `"`,
		"Hello from the child",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	checkLogoutReport(t, report)

	if cli.Process.ExitStatus != 3 {
		t.Errorf("final status %08X, want the child's 3", cli.Process.ExitStatus)
	}
}
