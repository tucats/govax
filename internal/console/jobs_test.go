package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/vmserrors"
)

// The job commands (docs/planning/PHASE-98 - batch queues.md, subtask
// 7), held to VMS 7.1's run of the probe: BAT98.LOG's section JOBS ON A
// STOPPED QUEUE (testdata/bat98/vax/bat98.log), cases 69 to 155, replayed
// line by line as the procedure ran them, every command's output compared
// with VMS's.

// bat98Log is the probe's SYSTEM log.
var bat98Log = filepath.Join("..", "..", "testdata", "bat98", "vax", "bat98.log")

// sectionLines are the lines of the probe log's section that begins with
// "######## from" and ends before "######## to".
func sectionLines(t *testing.T, log, from, to string) []string {
	t.Helper()

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	text := strings.ReplaceAll(string(data), "\r", "")

	start := strings.Index(text, "\n######## "+from+"\n")
	end := strings.Index(text, "\n######## "+to+"\n")

	if start < 0 || end < start {
		t.Fatalf("no section %s in %s", from, log)
	}

	return strings.Split(text[start+len("\n######## "+from+"\n"):end+1], "\n")
}

// logStep is one command of a section and what VMS wrote after it.
type logStep struct {
	command string
	want    string
}

// logSteps splits a section's lines into its commands ("$ " lines, as
// SET VERIFY echoed them) and each one's output.
func logSteps(lines []string) []logStep {
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	var steps []logStep

	for _, line := range lines {
		if cmd, ok := strings.CutPrefix(line, "$ "); ok {
			steps = append(steps, logStep{command: cmd})

			continue
		}

		if len(steps) > 0 {
			steps[len(steps)-1].want += line + "\n"
		}
	}

	return steps
}

// vmsTimes are the times a log shows, masked before comparing: the run's
// own times are the VAX's clock's, govax's its own.
var vmsTimes = regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d(:\d\d\.\d\d)?`)

// jobConsole is queueConsole with the probe's volume (DUA1, BAT98$DISK,
// its command procedures and [BAT98LOG]) and accounts (BAT98 [200,10],
// BAT98W [300,10]); dispatch runs a line and returns what it wrote.
func jobConsole(t *testing.T) (*Console, func(string) string) {
	t.Helper()

	c, _ := queueConsole(t)
	buf := c.Out.(interface {
		Reset()
		String() string
		WriteString(string) (int, error)
	})
	d := NewDispatcher(c, loadEvaxGrammar(t))

	dispatch := func(line string) string {
		t.Helper()

		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			_, _ = buf.WriteString(dclMessage(err) + "\n")
		}

		return strings.ReplaceAll(buf.String(), "\r", "")
	}

	dsk := filepath.Join(t.TempDir(), "bat98.dsk")

	for _, line := range []string{
		`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 BAT98`,
		`MOUNT/WRITE DUA1 "` + dsk + `"`,
		"CREATE/DIRECTORY DUA1:[BAT98LOG]",
		"DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK DUA1:",
		"CREATE/DIRECTORY DUA1:[SYSMGR]",
		"DEFINE/JOB/EXECUTIVE_MODE SYS$LOGIN DUA1:[SYSMGR]",
	} {
		dispatch(line)
	}

	for _, name := range []string{"bat98ok.com", "bat98wrn.com"} {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "bat98", name))
		if err != nil {
			t.Fatal(err)
		}

		if out := dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(name) + "/QUIET"); out != "" {
			t.Fatalf("COPY %s: %s", name, out)
		}

		dispatch("SET SECURITY/PROTECTION=(W:RE) DUA1:[000000]" + strings.ToUpper(name) + ";*")
	}

	for _, u := range []corevms.UserAccount{
		{Name: "BAT98", UIC: 0o200<<16 | 0o10},
		{Name: "BAT98W", UIC: 0o300<<16 | 0o10},
	} {
		if st := c.RTL.System.AddUser(u); st != 1 {
			t.Fatalf("AddUser %s: %#x", u.Name, st)
		}
	}

	return c, dispatch
}

// jobDifferences are the steps of the section where govax's output isn't
// VMS's, by the step's command, and govax's output.
var jobDifferences = map[string]string{}

// TestJobCommandsOracle replays BAT98.LOG's QUEUES section (its output not
// compared: TestQueueCommandsOracle does that), then holds every step of
// JOBS ON A STOPPED QUEUE to VMS's output. The VAX's node, SIMVAX, is
// govax's GOVAX; SYSTEM's login directory, SIMVAX$DUA0:[SYS0.][SYSMGR], is
// govax's; the VAX's entry numbers started at 18, and govax's database is
// set to number from there; the run's times are masked, and a time the
// procedure computed (LATER) is govax's in the commands that use it.
func TestJobCommandsOracle(t *testing.T) {
	t.Parallel()
	c, dispatch := jobConsole(t)

	for _, s := range logSteps(sectionLines(t, bat98Log, "QUEUES", "JOBS ON A STOPPED QUEUE")) {
		dispatch(s.command)
	}

	err := jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
		d.NextEntry = 18

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	login := "SYS$LOGIN:"
	if p, _, err := c.Commands().ContainerSession.Parse("SYS$LOGIN:", "", "", true, false); err == nil {
		login = p.Device + p.Directory
	}

	steps := logSteps(sectionLines(t, bat98Log, "JOBS ON A STOPPED QUEUE", "RUNNING JOBS"))
	if len(steps) < 200 {
		t.Fatalf("%d steps", len(steps))
	}

	// The last step writes the next section's heading.
	steps = steps[:len(steps)-1]

	vmsLater, later := "", ""

	for _, s := range steps {
		command := s.command
		if vmsLater != "" {
			command = strings.ReplaceAll(command, vmsLater, later)
		}

		got := dispatch(command)

		if command == "SHOW SYMBOL LATER" {
			vmsLater = strings.TrimSuffix(strings.TrimPrefix(s.want, `  LATER = "`), "\"\n")
			later = strings.TrimSuffix(strings.TrimPrefix(got, `  LATER = "`), "\"\n")
		}

		want := strings.ReplaceAll(s.want, "SIMVAX", "GOVAX")
		want = strings.ReplaceAll(want, "GOVAX$DUA0:[SYS0.][SYSMGR]", login)

		if d, ok := jobDifferences[s.command]; ok {
			want = d
		}

		if vmsTimes.ReplaceAllString(got, "<TIME>") != vmsTimes.ReplaceAllString(want, "<TIME>") {
			t.Errorf("%s:\n got %q\nwant %q", command, got, want)
		}
	}
}

// TestJobCommandsMatrix holds the job commands of the protection matrix's
// state A (BAT98_P owned by [SYSTEM], (S:M,O:D,G:R,W:S)) to the logs of
// BAT98 and BAT98W, neither with a privilege: SYSTEM's held job is "no
// privilege" to them and theirs to delete; their own jobs they may see and
// change; /PRIORITY=200 is lowered with PRIOSMALL; /USER needs the
// authorization file, which they may not read. Each pass's job cases are
// 4, 7 to 14, 29, and 30 (testdata/bat98/bat98p.com); the entry numbers
// are the VAX's.
func TestJobCommandsMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct{ number, commands string }{
		{"4", "SHOW ENTRY/FULL 'H'"},
		{"7", "IF F$TYPE($ENTRY) .NES. \"\" THEN DELETE/SYMBOL/LOCAL $ENTRY|" +
			"SUBMIT/QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/NAME=OWN98 BAT98$DISK:[000000]BAT98OK|SHOW SYMBOL $STATUS|" +
			"OWN = \"\"|IF F$TYPE($ENTRY) .NES. \"\" THEN OWN = $ENTRY|WRITE SYS$OUTPUT \"  OWN = \", OWN"},
		{"8", "SHOW ENTRY 'OWN'"},
		{"9", "SET ENTRY 'OWN'/NOTE=\"OWN NOTE\""},
		{"10", "IF F$TYPE($ENTRY) .NES. \"\" THEN DELETE/SYMBOL/LOCAL $ENTRY|" +
			"SUBMIT/QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/PRIORITY=200 BAT98$DISK:[000000]BAT98OK|SHOW SYMBOL $STATUS|" +
			"HI = \"\"|IF F$TYPE($ENTRY) .NES. \"\" THEN HI = $ENTRY|WRITE SYS$OUTPUT \"  HI = \", HI"},
		{"11", "DELETE/ENTRY='HI' BAT98_P"},
		{"12", "IF F$TYPE($ENTRY) .NES. \"\" THEN DELETE/SYMBOL/LOCAL $ENTRY|" +
			"SUBMIT/QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/USER=BAT98W BAT98$DISK:[000000]BAT98OK|SHOW SYMBOL $STATUS|" +
			"OTH = \"\"|IF F$TYPE($ENTRY) .NES. \"\" THEN OTH = $ENTRY|WRITE SYS$OUTPUT \"  OTH = \", OTH"},
		{"13", "IF OTH .EQS. \"\" THEN WRITE SYS$OUTPUT \"  NO OTH\"|IF OTH .NES. \"\" THEN DELETE/ENTRY='OTH' BAT98_P"},
		{"14", "SET ENTRY 'H'/NOTE=\"CHANGED\""},
		{"29", "DELETE/ENTRY='OWN' BAT98_P"},
		{"30", "DELETE/ENTRY='H' BAT98_P"},
	}

	passes := []struct {
		log, user string
		uic       uint32
		held, own int
	}{
		{"bat98p_a_bat98.log", "BAT98", 0o200<<16 | 0o10, 99, 101},
		{"bat98p_a_bat98w.log", "BAT98W", 0o300<<16 | 0o10, 147, 149},
	}

	for _, p := range passes {
		t.Run(p.log, func(t *testing.T) {
			c, dispatch := jobConsole(t)

			setEntry := func(n int) {
				t.Helper()

				if err := jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
					d.NextEntry = n

					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}

			dispatch("INITIALIZE/QUEUE/BATCH/JOB_LIMIT=1/START BAT98_P")
			setEntry(p.held)
			dispatch("SUBMIT/QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/NAME=SYSHELD BAT98$DISK:[000000]BAT98OK")
			setEntry(p.own)
			dispatch("H = \"" + strconv.Itoa(p.held) + "\"")

			proc := c.RTL.Process
			proc.Username, proc.UIC = p.user, p.uic
			proc.AuthorizedPrivileges = privilegeMask("NETMBX", "TMPMBX")
			proc.ProcessPrivileges = proc.AuthorizedPrivileges
			proc.CurrentPrivileges = proc.ProcessPrivileges

			want := passCases(t, filepath.Join("..", "..", "testdata", "bat98", "vax", p.log), "NONE")

			for _, tc := range cases {
				var got strings.Builder

				// The case's output ends with $STATUS's line: what its
				// commands do after SHOW SYMBOL $STATUS (keep $ENTRY in a
				// symbol) isn't compared.
				before, after, _ := strings.Cut(tc.commands, "|SHOW SYMBOL $STATUS")
				for _, cmd := range strings.Split(before, "|") {
					got.WriteString(dispatch(cmd))
				}

				got.WriteString(dispatch("SHOW SYMBOL $STATUS"))

				for _, cmd := range strings.Split(strings.TrimPrefix(after, "|"), "|") {
					if cmd != "" {
						dispatch(cmd)
					}
				}

				w := strings.ReplaceAll(want[tc.number], "SIMVAX", "GOVAX")

				if got.String() != w {
					t.Errorf("case %s:\n got %q\nwant %q", tc.number, got.String(), w)
				}
			}
		})
	}
}
