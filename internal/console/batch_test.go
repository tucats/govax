package console_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmserrors"
)

// Batch jobs (docs/planning/PHASE-98 - batch queues.md, subtask 8), held
// to VMS 7.1's run of the probe (testdata/bat98/vax/bat98.log): the jobs
// run, their logs, how they end, SYNCHRONIZE.

// bat98VAXLog is the probe's SYSTEM log.
var bat98VAXLog = filepath.Join("..", "..", "testdata", "bat98", "vax", "bat98.log")

// batchConsole is a booted console with the scheduler on, a queue
// database of its own with its manager started, the probe's volume as
// DUA1 (BAT98$DISK, its procedures in [000000], [BAT98LOG] and
// [BAT98SCR], SYSTEM's home [SYSMGR] as SYS$LOGIN), and the probe's
// accounts; and a function running a line and returning what it wrote.
func batchConsole(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	return batchSession(t, filepath.Join(t.TempDir(), "bat98.dsk"), filepath.Join(t.TempDir(), "qman.json"), true)
}

// batchSession is batchConsole on the volume container dsk and the queue
// database path: with fresh, both made new and the manager started, as
// batchConsole's; otherwise a later session of the same govax, the volume
// mounted as it was left and the manager started as a session starts it
// (ResumeQueueManager), with whatever the database says.
func batchSession(t *testing.T, dsk, path string, fresh bool) (*console.Console, func(string) string) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	dispatch := func(line string) string {
		t.Helper()
		out.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			out.WriteString("%" + err.Error() + "\n")
		}

		return strings.ReplaceAll(out.String(), "\r", "")
	}

	c.QueueFile = path
	c.RTL.System.SetQueueFile(path)

	if status := c.RTL.AddIdentifier("BAT98_ID", 0x80010098, 0); status != 1 {
		t.Fatalf("AddIdentifier: %#x", status)
	}

	setup := []string{
		`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 BAT98`,
		`MOUNT/WRITE DUA1 "` + dsk + `"`,
		"CREATE/DIRECTORY DUA1:[BAT98LOG]",
		"CREATE/DIRECTORY DUA1:[BAT98SCR]",
		"SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) DUA1:[000000]BAT98LOG.DIR",
		"SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) DUA1:[000000]BAT98SCR.DIR",
		"DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK DUA1:",
		"CREATE/DIRECTORY DUA1:[SYSMGR]",
		"DEFINE/JOB/EXECUTIVE_MODE SYS$LOGIN DUA1:[SYSMGR]",
		"SET DEFAULT DUA1:[SYSMGR]",
		"START/QUEUE/MANAGER",
	}

	if !fresh {
		setup = []string{
			`MOUNT/WRITE DUA1 "` + dsk + `"`,
			"DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK DUA1:",
			"DEFINE/JOB/EXECUTIVE_MODE SYS$LOGIN DUA1:[SYSMGR]",
			"SET DEFAULT DUA1:[SYSMGR]",
		}
	}

	// A second session's START/QUEUE/MANAGER, while another runs the
	// manager, is refused (TestBatchSecondSession checks it).
	for _, line := range setup {
		if got := dispatch(line); strings.Contains(got, "%") && !strings.Contains(got, "-I-") && !strings.Contains(got, "QMANNOTSTARTED") {
			t.Fatalf("%s: %s", line, got)
		}
	}

	if !fresh {
		c.ResumeQueueManager()
	}

	procedures := []string{
		"bat98ok.com", "bat98wrn.com", "bat98err.com", "bat98fat.com", "bat98bad.com",
		"bat98w.com", "bat98c.com", "bat98o.com", "bat98r.com", "bat98j.com",
	}

	if !fresh {
		procedures = nil // on the volume already
	}

	for _, name := range procedures {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "bat98", name))
		if err != nil {
			t.Fatal(err)
		}

		// BAT98W's job waits a second a time, which its log doesn't show:
		// here it waits a twentieth, so the tests take less of the host's
		// time.
		if name == "bat98w.com" {
			text, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}

			src = filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(src, []byte(strings.ReplaceAll(string(text), "WAIT 0:0:1", "WAIT 0:0:0.05")), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		if got := dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(name) + "/QUIET"); got != "" {
			t.Fatalf("COPY %s: %s", name, got)
		}

		dispatch("SET SECURITY/PROTECTION=(W:RE) DUA1:[000000]" + strings.ToUpper(name) + ";*")
	}

	// SYSTEM's account, whose jobs these are, has its home on the volume,
	// as the VAX's SYS$SYSROOT:[SYSMGR].
	if err := c.RTL.System.UpdateAuthorization(func(f *uaf.File) error {
		r, ok := f.Find("SYSTEM")
		if !ok {
			t.Fatal("no SYSTEM account")
		}

		r.Device, r.Directory = "DUA1:", "[SYSMGR]"

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The probe's accounts, with homes of their own on the volume, owned
	// by each and (S:RWE,O:RWE,G:RE,W:RE), as BAT98ACCT.COM made them.
	for _, u := range []corevms.UserAccount{
		{Name: "BAT98", UIC: 0o200<<16 | 0o10},
		{Name: "BAT98W", UIC: 0o300<<16 | 0o10},
	} {
		if st := c.RTL.System.AddUser(u); st != 1 {
			t.Fatalf("AddUser %s: %#x", u.Name, st)
		}

		dispatch("CREATE/DIRECTORY DUA1:[" + u.Name + "]")
		dispatch(fmt.Sprintf("SET SECURITY/OWNER=[%o,%o]/PROTECTION=(S:RWE,O:RWE,G:RE,W:RE) DUA1:[000000]%s.DIR",
			u.UIC>>16, u.UIC&0xFFFF, u.Name))
	}

	if err := c.RTL.System.UpdateAuthorization(func(f *uaf.File) error {
		for _, name := range []string{"BAT98", "BAT98W"} {
			r, _ := f.Find(name)
			r.Device, r.Directory = "DUA1:", "["+name+"]"
			r.DefaultPrivileges = r.Privileges
			r.Account = "GOVAX98"
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	return c, dispatch
}

// probeLogSection are the lines of the probe log's section that begins
// with "######## from" and ends before "######## to".
func probeLogSection(t *testing.T, from, to string) []string {
	t.Helper()

	data, err := os.ReadFile(bat98VAXLog)
	if err != nil {
		t.Fatal(err)
	}

	text := strings.ReplaceAll(string(data), "\r", "")

	start := strings.Index(text, "\n######## "+from+"\n")
	end := strings.Index(text, "\n######## "+to+"\n")

	if start < 0 || end < start {
		t.Fatalf("no section %s", from)
	}

	return strings.Split(text[start+len("\n######## "+from+"\n"):end+1], "\n")
}

// probeStep is one command of a section and what VMS wrote after it.
type probeStep struct{ command, want string }

// probeCommand reports whether line, after a TYPE of a job's log, is the
// probe's own next command rather than a line of the log, which a job's
// verification fills with "$ " lines too.
func probeCommand(line string) bool {
	for _, prefix := range []string{
		`$ WRITE SYS$OUTPUT "==`, `$ WRITE SYS$OUTPUT "####`, `$ DELETE BAT98$DISK:`,
		`$ COPY BAT98$DISK:`, `$ DIRECTORY`, `$ TYPE `, `$ SHOW `, `$ IF F$TYPE($ENTRY)`,
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	return line == "$ !"
}

// probeSteps splits a section's lines into its commands ("$ " lines) and
// each one's output.
func probeSteps(lines []string) []probeStep {
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	var steps []probeStep

	awaiting := false

	for _, line := range lines {
		// CALL AWAIT's subroutine, a loop waiting for a job's marker file,
		// is one step, which the test does its own way (await): what it
		// wrote is its line saying the job started.
		if awaiting {
			if line == "$ ENDSUBROUTINE" {
				awaiting = false
			} else if !strings.HasPrefix(line, "$ ") {
				steps[len(steps)-1].want += line + "\n"
			}

			continue
		}

		if strings.HasPrefix(line, "$ CALL AWAIT ") {
			steps = append(steps, probeStep{command: strings.TrimPrefix(line, "$ ")})
			awaiting = true

			continue
		}

		typing := len(steps) > 0 && strings.HasPrefix(steps[len(steps)-1].command, "TYPE ")

		if cmd, ok := strings.CutPrefix(line, "$ "); ok && (!typing || probeCommand(line)) {
			steps = append(steps, probeStep{command: cmd})

			continue
		}

		if len(steps) > 0 {
			steps[len(steps)-1].want += line + "\n"
		}
	}

	return steps
}

// batchMasks are what a job's output may differ in from VMS's: the
// times, the accounting report's counts (govax's processes aren't VMS's),
// and the node.
var batchMasks = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d(:\d\d\.\d\d)?`), "<TIME>"},
	{regexp.MustCompile(`(?m)^(  (?:Buffered I/O count|Direct I/O count|Page faults):) +\d+ +(Peak working set size|Peak page file size|Mounted volumes): +\d+$`), "$1 <N> $2 <N>"},
	{regexp.MustCompile(`(?m)^  Charged CPU time: .*$`), "  Charged CPU time: <CPU>"},

	// BAT98L3's run, shortened by shorter.
	{regexp.MustCompile(`/PARAM=\("BAT98L3","1[25]"\)`), `/PARAM=("BAT98L3","15")`},
}

// maskBatch applies batchMasks to s.
func maskBatch(s string) string {
	s = strings.ReplaceAll(s, "SIMVAX", "GOVAX")

	for _, m := range batchMasks {
		s = m.re.ReplaceAllString(s, m.with)
	}

	return s
}

// replayQueues runs the probe's QUEUES and JOBS ON A STOPPED QUEUE
// sections, unchecked (TestQueueCommandsOracle and TestJobCommandsOracle
// check them), for the queues they leave, and numbers the next entry n,
// as the VAX's was.
func replayQueues(t *testing.T, c *console.Console, dispatch func(string) string, n int) {
	t.Helper()

	for _, s := range probeSteps(probeLogSection(t, "QUEUES", "JOBS ON A STOPPED QUEUE")) {
		dispatch(s.command)
	}

	for _, s := range probeSteps(probeLogSection(t, "JOBS ON A STOPPED QUEUE", "RUNNING JOBS")) {
		dispatch(s.command)
	}

	if err := jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
		d.NextEntry = n

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// runSteps runs a section's steps, each compared with VMS's output (after
// batchMasks), and returns the /NOTIFY notices they brought. CALL AWAIT
// name entry waits for the job's marker file, as the probe's subroutine
// does; a WAIT of the probe's own is shortened, as BAT98W's are.
func runSteps(t *testing.T, dispatch func(string) string, steps []probeStep, differences map[string]string) []string {
	t.Helper()

	var (
		notices  []string
		previous string
	)

	for _, s := range steps {
		var got string

		if rest, ok := strings.CutPrefix(s.command, "CALL AWAIT "); ok {
			got = await(dispatch, strings.Fields(rest)[0])
		} else {
			var notice []string

			got, notice = splitNotices(dispatch(shorter(s.command)))
			notices = append(notices, notice...)
		}

		want := s.want
		if d, ok := differences[s.command]; ok {
			want = d
		} else if d, ok := differences[previous+"|"+s.command]; ok {
			want = d
		}

		previous = s.command

		if maskBatch(got) != maskBatch(want) {
			t.Errorf("%s:\n got %q\nwant %q", s.command, got, want)
		}
	}

	return notices
}

// shorter is a probe command with its waits shortened: WAIT 0:0:5, and a
// job held /AFTER="+0:00:15" or "+0:00:20", wait a second at most here (the times
// shown are masked).
//
// And THE JOB LIMIT's BAT98L3 runs 12 seconds, not 15. On the VAX it had
// ended and gone by the time SYNCHRONIZE asked for it, after L1's and L2's
// (NOSUCHENT, case 216): it ended while an earlier SYNCHRONIZE waited, which
// in govax forgets it (corevms's jobEnd). Started a moment after L1 and L2
// and running as long, it ends within milliseconds of them, before or after
// that SYNCHRONIZE completes as the host's load decides (the console's tests
// run in parallel). Three seconds sooner, it always ends while SYNCHRONIZE
// waits for L1.
func shorter(command string) string {
	command = strings.ReplaceAll(command, "WAIT 0:0:5", "WAIT 0:0:0.25")

	command = strings.ReplaceAll(command, "/PARAMETERS=(BAT98L3,15)", "/PARAMETERS=(BAT98L3,12)")

	command = strings.ReplaceAll(command, `/AFTER="+0:00:15"`, `/AFTER="+0:00:01"`)

	return strings.ReplaceAll(command, `/AFTER="+0:00:20"`, `/AFTER="+0:00:01"`)
}

// await is CALL AWAIT name: it waits, a while at most, for the job name's
// marker file, BAT98$DISK:[BAT98SCR]name.RUN, and says whether it came.
func await(dispatch func(string) string, name string) string {
	for range 400 {
		if strings.TrimSpace(dispatch(`WRITE SYS$OUTPUT F$SEARCH("BAT98$DISK:[BAT98SCR]`+name+`.RUN")`)) != "" {
			return "  " + name + " STARTED\n"
		}

		dispatch("WAIT 0:0:0.02")
	}

	return "  " + name + " NEVER STARTED\n"
}

// TestBatchRunningJobsOracle holds BAT98.LOG's RUNNING JOBS section,
// cases 156 to 173, to VMS's output: jobs of one or several files ending
// with each severity, parameters, /NOTIFY, and /CPUTIME, each
// SYNCHRONIZEd, its log listed and typed.
func TestBatchRunningJobsOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	c, dispatch := batchConsole(t)
	replayQueues(t, c, dispatch, 46)

	steps := probeSteps(probeLogSection(t, "RUNNING JOBS", "ENDINGS"))
	steps = steps[:len(steps)-1] // the next section's heading

	notices := runSteps(t, dispatch, steps, nil)

	// /NOTIFY's message came to the VAX's terminal (terminal.log).
	if want := []string{"Job BAT98R7 (queue BAT98_A, entry 52) completed"}; strings.Join(notices, "|") != strings.Join(want, "|") {
		t.Errorf("notices %q, want %q", notices, want)
	}
}

// noticeLine is the first line of /NOTIFY's message at the terminal.
var noticeLine = regexp.MustCompile(`^Job \S+ \(queue \S+, entry \d+\) (completed|terminated with error status)$`)

// splitNotices takes /NOTIFY's messages out of out, which come to the
// terminal, not a procedure's log: a blank line, the notice, and for an
// error its condition's line. It returns the rest and each notice.
func splitNotices(out string) (string, []string) {
	lines := strings.Split(out, "\n")

	var (
		kept    []string
		notices []string
	)

	for i := 0; i < len(lines); i++ {
		if lines[i] == "" && i+1 < len(lines) && noticeLine.MatchString(lines[i+1]) {
			notices = append(notices, lines[i+1])
			i++

			if strings.HasSuffix(lines[i], "error status") && i+1 < len(lines) {
				notices[len(notices)-1] += " " + lines[i+1]
				i++
			}

			continue
		}

		kept = append(kept, lines[i])
	}

	return strings.Join(kept, "\n"), notices
}

// endingsDifferences are ENDINGS' steps where govax's output isn't VMS's,
// and govax's.
var endingsDifferences = map[string]string{
	// A job ended by STOP/QUEUE/RESET: VMS's log had nothing in it yet,
	// its first line still in RMS's buffer when the process went; govax
	// writes each record through.
	"DIRECTORY/SIZE/OWNER/PROTECTION BAT98$DISK:[BAT98LOG]BAT98R15.LOG;*": "\nDirectory DUA1:[BAT98LOG]\n\n" +
		"BAT98R15.LOG;1             1  [SYSTEM]                         (RWED,RWED,RE,)\n\nTotal of 1 file, 1 block.\n",
	"TYPE BAT98$DISK:[BAT98LOG]BAT98R15.LOG;*": " \nDUA1:[BAT98LOG]BAT98R15.LOG;1\n \nBAT98W BAT98R15 STARTED\n",
}

// TestBatchEndingsOracle holds BAT98.LOG's ENDINGS section, cases 174 to
// 208, to VMS's output: jobs ended by STOP/QUEUE/ENTRY, DELETE/ENTRY, and
// STOP of the process; a queue paused (its job suspended), stopped with
// /NEXT, and reset; SET ENTRY of an executing job.
func TestBatchEndingsOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	c, dispatch := batchConsole(t)
	replayQueues(t, c, dispatch, 46)

	for _, s := range probeSteps(probeLogSection(t, "RUNNING JOBS", "ENDINGS")) {
		dispatch(s.command)
	}

	steps := probeSteps(probeLogSection(t, "ENDINGS", "THE JOB LIMIT"))
	steps = steps[:len(steps)-1] // the next section's heading

	if notices := runSteps(t, dispatch, steps, endingsDifferences); len(notices) != 0 {
		t.Errorf("notices %q, want none", notices)
	}
}

// laterDifferences are the later sections' steps where govax's output
// isn't VMS's (by the step's command, or the command before it and the
// step's, joined by "|"), and govax's output.
var laterDifferences = map[string]string{
	// DIRECTORY that finds nothing: VMS 7.1 left RMS$_FNF with its
	// message bit, govax DIRECT-W-NOFILES' own status (an older
	// difference).
	"DIRECTORY BAT98$DISK:[BAT98SCR]DELME.COM|SHOW SYMBOL $STATUS": "  $STATUS == \"%X00921330\"\n",

	// SET ENTRY/RELEASE of ORDER6, then SYNCHRONIZE at once: on the VAX
	// the job ran and was gone before SYNCHRONIZE asked (a race); govax
	// keeps a job's end until a SYNCHRONIZE completes (corevms's jobEnd),
	// so it has the job's status.
	"IF O6 .NES. \"\" THEN SYNCHRONIZE/ENTRY=89":                     "",
	"IF O6 .NES. \"\" THEN SYNCHRONIZE/ENTRY=89|SHOW SYMBOL $STATUS": "  $STATUS == \"%X10010001\"\n",
}

// TestBatchLaterSectionsOracle holds BAT98.LOG's sections THE JOB LIMIT,
// RETAINED JOBS, SYNCHRONIZE, DELETE, AFTER ON A RUNNING QUEUE, and ORDER
// to VMS's output, after replaying the sections before them (LOG FILES,
// whose subroutine searches directories in a loop, is
// TestBatchLogFilesOracle's).
func TestBatchLaterSectionsOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	c, dispatch := batchConsole(t)
	replayQueues(t, c, dispatch, 46)

	for _, s := range probeSteps(probeLogSection(t, "RUNNING JOBS", "ENDINGS")) {
		dispatch(s.command)
	}

	for _, s := range probeSteps(probeLogSection(t, "ENDINGS", "THE JOB LIMIT")) {
		if rest, ok := strings.CutPrefix(s.command, "CALL AWAIT "); ok {
			await(dispatch, strings.Fields(rest)[0])
		} else {
			dispatch(shorter(s.command))
		}
	}

	sections := []string{"THE JOB LIMIT", "RETAINED JOBS", "SYNCHRONIZE", "DELETE", "AFTER ON A RUNNING QUEUE", "LOG FILES"}

	for i := 0; i+1 < len(sections); i++ {
		steps := probeSteps(probeLogSection(t, sections[i], sections[i+1]))
		runSteps(t, dispatch, steps[:len(steps)-1], laterDifferences)
	}

	// ORDER's entries start at 84, the VAX's after LOG FILES' jobs.
	if err := jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
		d.NextEntry = 84

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	steps := probeSteps(probeLogSection(t, "ORDER", "RESTART"))
	runSteps(t, dispatch, steps[:len(steps)-1], laterDifferences)

	// RESTART's entries start at 91 (subtask 10).
	if err := jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
		d.NextEntry = 91

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	steps = probeSteps(probeLogSection(t, "RESTART", "THE JOB'S VIEW"))
	runSteps(t, dispatch, steps[:len(steps)-1], restartDifferences(steps))
}

// restartDifferences are RESTART's steps (cases 306 to 319) where govax's
// output isn't VMS's, made from VMS's own: laterDifferences, and these.
//
// On the VAX a requeued job's process took a moment to go, so the SHOW
// straight after STOP/QUEUE/REQUEUE found it "Aborting" (cases 309, 313,
// 317), and case 318's /RELEASE came while it still ran (EXECUTING), which
// left it holding and stalled the probe until a release by hand (the
// README's "Round 1"). govax ends the process between process 1's
// commands, so the job has been requeued by then: X1 and X2 have run
// again and gone (their queue keeps only failures), X3 is holding,
// checkpointed, and the /RELEASE takes it. The queue is idle, and BAT98G9,
// a LOG FILES job this test doesn't replay, isn't there.
func restartDifferences(steps []probeStep) map[string]string {
	diffs := maps.Clone(laterDifferences)

	previous := ""
	for _, s := range steps {
		key := previous + "|" + s.command
		previous = s.command

		switch key {
		case `WRITE SYS$OUTPUT "== 309"|SHOW QUEUE/FULL/ALL BAT98_A`,
			`WRITE SYS$OUTPUT "== 313"|SHOW QUEUE/FULL/ALL BAT98_A`:
			want := dropEntries(s.want, "91", "92", "78")
			diffs[key] = strings.Replace(want, "BAT98_A, available,", "BAT98_A, idle,", 1)
		}
	}

	diffs["SHOW ENTRY/FULL 93"] = strings.Join([]string{
		"  Entry  Jobname         Username     Blocks  Status",
		"  -----  -------         --------     ------  ------",
		"     93  BAT98X3         SYSTEM               Holding",
		"         On idle batch queue BAT98_A",
		`         Submitted 16-NOV-2026 11:27:28.15 /KEEP /LOG=DUA1:[BAT98LOG]BAT98X3.LOG; /PARAM=("BAT98X3") /NOPRINT /PRIORITY=100 `,
		`         /RESTART=BAT98_A /RESTART_VALUE="STEP2"`,
		"         File: _GOVAX$DUA1:[000000]BAT98R.COM;1 (checkpointed)",
		"",
	}, "\n")
	diffs["SET ENTRY 93/RELEASE"] = ""
	diffs["SET ENTRY 93/RELEASE|SHOW SYMBOL $STATUS"] = "  $STATUS == \"%X10040001\"\n"
	diffs[`IF X3 .NES. "" THEN SYNCHRONIZE/ENTRY=93`] = ""

	return diffs
}

// dropEntries is SHOW QUEUE's output without the jobs entries: each a
// row with its number and the lines after it, to the blank line that ends
// its paragraph (and that blank line).
func dropEntries(out string, entries ...string) string {
	var kept []string

	dropping := false

	for _, line := range strings.SplitAfter(out, "\n") {
		row := strings.TrimLeft(line, " ")
		if slices.ContainsFunc(entries, func(e string) bool { return strings.HasPrefix(row, e+"  ") }) {
			dropping = true

			continue
		}

		if dropping {
			dropping = line != "\n"

			continue
		}

		kept = append(kept, line)
	}

	return strings.TrimRight(strings.Join(kept, ""), "\n") + "\n"
}

// TestBatchLogFilesOracle holds where each job's log went to BAT98.LOG's
// LOG FILES section (cases 243 to 275): VMS's FINDLOG subroutine, which
// searched the probe's directories and SYS$LOGIN for the job's name,
// printed "LOGS FOUND: ..." for each; here each job is SYNCHRONIZEd and
// the file looked for where VMS found it, and nowhere else.
func TestBatchLogFilesOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	_, dispatch := batchConsole(t)

	dispatch("INITIALIZE/QUEUE/BATCH/START/JOB_LIMIT=3/RETAIN=ERROR BAT98_A")

	const submit = "SUBMIT/QUEUE=BAT98_A/NAME="

	cases := []struct {
		qualifiers string
		submitted  string // SUBMIT's output, the job's name for "started on"
		logs       []string
	}{
		{"BAT98G1/NOPRINTER/KEEP", "", []string{"DUA1:[SYSMGR]BAT98G1.LOG;1"}},
		{"BAT98G2/NOPRINTER/KEEP/USER=BAT98", "", []string{"DUA1:[BAT98]BAT98G2.LOG;1"}},
		{"BAT98G3/NOPRINTER/KEEP/LOG_FILE=BAT98$DISK:[BAT98LOG]", "", []string{"DUA1:[BAT98LOG]BAT98G3.LOG;1"}},
		{"BAT98G4/NOPRINTER/KEEP/LOG_FILE=BAT98G4X", "", []string{"DUA1:[SYSMGR]BAT98G4X.LOG;1"}},
		{"BAT98G5/NOPRINTER/KEEP/LOG_FILE=BAT98$DISK:[BAT98LOG]BAT98G5.TXT", "", []string{"DUA1:[BAT98LOG]BAT98G5.TXT;1"}},
		{"BAT98G6/NOPRINTER/KEEP/LOG_FILE", "", []string{"DUA1:[SYSMGR]BAT98G6.LOG;1"}},
		{"BAT98G7/NOPRINTER/KEEP/NOLOG_FILE", "", nil},
		{"BAT98G8/NOPRINTER/KEEP/LOG_FILE=BAT98$DISK:[NOSUCHDIR98]BAT98G8.LOG",
			"%SUBMIT-F-INVLOGFIL, invalid log file specification\n-RMS-E-DNF, directory not found\n-SYSTEM-W-NOSUCHFILE, no such file\n", nil},
		{"BAT98G9/NOPRINTER/KEEP/USER=BAT98W/LOG_FILE=BAT98$DISK:[BAT98]BAT98G9.LOG", "", nil},
		{"BAT98G10/NOPRINTER/NOKEEP", "", nil},
		{"BAT98G11/NOPRINTER", "", []string{"DUA1:[SYSMGR]BAT98G11.LOG;1"}},
		{"BAT98G12/LOG_FILE=BAT98$DISK:[BAT98SCR]BAT98G12.LOG/NOPRINTER/NOKEEP", "", nil},
		{"BAT98G13", "", []string{"DUA1:[SYSMGR]BAT98G13.LOG;1"}},
		{"BAT98G14/KEEP", "", []string{"DUA1:[SYSMGR]BAT98G14.LOG;1"}},
		{"BAT98G15/PRINTER/NOKEEP", "%SUBMIT-F-CREJOB, error creating job\n-JBC-E-NODSTQUE, no such destination queue\n", nil},
	}

	for _, tc := range cases {
		name, _, _ := strings.Cut(tc.qualifiers, "/")

		dispatch("IF F$TYPE($ENTRY) .NES. \"\" THEN DELETE/SYMBOL/LOCAL $ENTRY")

		got := dispatch(submit + tc.qualifiers + " BAT98$DISK:[000000]BAT98OK")

		want := tc.submitted
		if want == "" {
			want = "Job " + name + " (queue BAT98_A, entry "
		}

		if !strings.HasPrefix(got, want) {
			t.Errorf("%s: SUBMIT said %q, want %q", name, got, want)

			continue
		}

		if tc.submitted == "" {
			dispatch("SYNCHRONIZE/ENTRY='$ENTRY'")
		}

		var found []string

		for _, dir := range []string{"[SYSMGR]", "[BAT98]", "[BAT98W]", "[BAT98LOG]", "[BAT98SCR]"} {
			for _, f := range strings.Fields(dispatch("DIRECTORY/NOHEADING/NOTRAILING DUA1:" + dir + name + "*.*;*")) {
				if strings.Contains(f, "]") && strings.Contains(f, ";") {
					found = append(found, f)
				}
			}
		}

		if strings.Join(found, ",") != strings.Join(tc.logs, ",") {
			t.Errorf("%s: logs %q, want %q", name, found, tc.logs)
		}
	}

	// BAT98G9's log couldn't be made in BAT98's home by BAT98W: the job
	// was retained with RMS$_PRV (case 262).
	if got := dispatch("SHOW QUEUE/ALL BAT98_A"); !strings.Contains(got, "BAT98G9         BAT98W               Retained on error\n       %RMS-E-PRV,") {
		t.Errorf("BAT98G9 not retained with RMS-E-PRV:\n%s", got)
	}
}

// jobViewItems are the F$GETJPI items of BAT98J's that say what kind of
// process a batch job is, compared with VMS's; the rest are the process's
// counts, addresses, and quotas, which govax's processes don't share.
var jobViewItems = []string{
	"AUTHPRI", "CPULIM", "CREPRC_FLAGS", "GRP", "JOBPRCCNT", "JOBTYPE", "MEM", "MODE",
	"OWNER", "PRIB", "TERMINAL", "UIC",
}

// TestBatchJobViewOracle runs the probe's BAT98J.COM as a job, as SYSTEM
// and as BAT98 (whose LOGIN.COM says it ran), and holds what the job sees
// of itself to VMS 7.1's logs (testdata/bat98/vax/bat98v_*.log): its lines
// up to F$GETJPI (F$MODE, verification, $RESTART, P1 to P8, F$ENVIRONMENT
// but for DEFAULT, OUTPUT_RATE, and PROCEDURE, which name the VAX's
// disks), jobViewItems, its name and master, and the rights identifier
// BATCH.
func TestBatchJobViewOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	_, dispatch := batchConsole(t)

	src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "bat98", "login_bat98.com"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST DUA1:[BAT98]LOGIN.COM/QUIET`)
	dispatch("SET SECURITY/OWNER=[200,10]/PROTECTION=(W:RE) DUA1:[BAT98]LOGIN.COM;*")
	dispatch("INITIALIZE/QUEUE/BATCH/START/JOB_LIMIT=3/BASE_PRIORITY=3/CPUDEFAULT=0:05:00 BAT98_A")

	for _, user := range []string{"SYSTEM", "BAT98"} {
		t.Run(user, func(t *testing.T) {
			name := "BAT98V_" + user

			qualifiers := ""
			if user != "SYSTEM" {
				qualifiers = "/USER=" + user
			}

			got := dispatch("SUBMIT/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=" + name + qualifiers +
				"/LOG_FILE=BAT98$DISK:[BAT98LOG]" + name + `.LOG/PARAMETERS=(ALPHA,"two words",3,"",E,F,G,"H h") BAT98$DISK:[000000]BAT98J`)
			if !strings.Contains(got, "started on BAT98_A") {
				t.Fatalf("SUBMIT: %q", got)
			}

			dispatch("SYNCHRONIZE/ENTRY='$ENTRY'")

			if got := dispatch("SHOW SYMBOL $STATUS"); got != "  $STATUS == \"%X10010001\"\n" {
				t.Errorf("the job's status: %q", got)
			}

			log := strings.Split(dispatch("TYPE BAT98$DISK:[BAT98LOG]"+name+".LOG"), "\n")

			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bat98", "vax", "bat98v_"+strings.ToLower(user)+".log"))
			if err != nil {
				t.Fatal(err)
			}

			vax := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")

			head := func(lines []string) []string {
				var kept []string

				for _, line := range lines {
					if line == "######## F$GETJPI" {
						break
					}

					for _, skip := range []string{"  DEFAULT = ", "  OUTPUT_RATE = ", "  PROCEDURE = "} {
						if strings.HasPrefix(line, skip) {
							line = skip
						}
					}

					kept = append(kept, line)
				}

				return kept
			}

			if g, w := strings.Join(head(log), "\n"), strings.Join(head(vax), "\n"); g != w {
				t.Errorf("the job's first lines:\n got %s\nwant %s", g, w)
			}

			item := func(lines []string, name string) string {
				for _, line := range lines {
					if v, ok := strings.CutPrefix(line, "  "+name+" = "); ok {
						return strings.TrimSpace(v)
					}
				}

				return "(none)"
			}

			// The VAX's rights database named group 200 PROT93 (Phase
			// 93's accounts), which govax's hasn't.
			for _, name := range jobViewItems {
				if g, w := item(log, name), strings.Replace(item(vax, name), "[PROT93,", "[", 1); g != w {
					t.Errorf("F$GETJPI %s: got %q, want %q", name, g, w)
				}
			}

			if !strings.HasPrefix(item(log, "PRCNAM"), "BATCH_") || item(log, "MASTER_PID") != item(log, "PID") {
				t.Errorf("PRCNAM %q, MASTER_PID %q, PID %q: want BATCH_n, its own master", item(log, "PRCNAM"), item(log, "MASTER_PID"), item(log, "PID"))
			}

			if !slices.Contains(log, " BATCH                             ") {
				t.Errorf("no rights identifier BATCH in the job's SHOW PROCESS/RIGHTS")
			}

			// F$GETQUI of the job's own job and file (subtask 9), line
			// for line: its two DISPLAY_QUEUE lines with no object-id are
			// DCL's ARGREQ. The times, the entry number, and the PID are
			// the run's; the node is govax's.
			getqui := func(lines []string) []string {
				var kept []string

				in := false

				for _, line := range lines {
					switch {
					case line == "######## F$GETQUI":
						in = true
					case strings.HasPrefix(line, "######## "):
						in = false
					}

					if !in {
						continue
					}

					for _, skip := range []string{"  AFTER_TIME = ", "  SUBMISSION_TIME = ", "  ENTRY_NUMBER = ", "  JOB_PID = "} {
						if strings.HasPrefix(line, skip) {
							line = skip
						}
					}

					line = strings.Replace(line, "_SIMVAX$", "_GOVAX$", 1)
					line = strings.Replace(line, "[PROT93,", "[", 1)
					kept = append(kept, line)
				}

				return kept
			}

			if g, w := strings.Join(getqui(log), "\n"), strings.Join(getqui(vax), "\n"); g != w {
				t.Errorf("the job's F$GETQUI:\n got %s\nwant %s", g, w)
			}

			if pid := item(log, "JOB_PID"); pid != item(log, "PID") {
				t.Errorf("F$GETQUI's JOB_PID %q, the job's PID %q", pid, item(log, "PID"))
			}
		})
	}
}
