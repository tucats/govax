package console_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/testproc"
)

// A session's end and the next session's start (docs/planning/PHASE-98 -
// batch queues.md, subtask 10; Decision 11), held to the probe's reboot
// (testdata/bat98/vax/terminal.log, bootr.log): BOOTR, submitted
// /RESTART, executing as the VAX stopped, came back pending, checkpointed
// with its restart value, on a stopped queue, and ran from STEP2 when the
// queue was started; BOOTN, a plain job, came back retained with
// JBC$_QUERESET.

// probeQueueA makes BAT98_A as the probe had made it by the reboot, and
// starts it.
func probeQueueA(t *testing.T, dispatch func(string) string) {
	t.Helper()

	for _, line := range []string{
		`INITIALIZE/QUEUE/BATCH/JOB_LIMIT=3/BASE_PRIORITY=3/DESCRIPTION="GOVAX PROBE A" BAT98_A`,
		"INITIALIZE/QUEUE/BATCH/WSDEFAULT=200/WSQUOTA=400/WSEXTENT=800 BAT98_A",
		"INITIALIZE/QUEUE/BATCH/CPUDEFAULT=0:05:00/CPUMAXIMUM=0:10:00/RETAIN=ERROR BAT98_A",
		"START/QUEUE BAT98_A",
	} {
		if got := dispatch(line); got != "" {
			t.Fatalf("%s: %s", line, got)
		}
	}
}

// terminalSection is terminal.log's text from the line after the first
// one that is from (after a "$ " prompt), past the line after, to the next
// such prompt.
func terminalSection(t *testing.T, after, from string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bat98", "vax", "terminal.log"))
	if err != nil {
		t.Fatal(err)
	}

	text := strings.ReplaceAll(string(data), "\r", "")

	if i := strings.Index(text, after); i >= 0 {
		text = text[i:]
	}

	i := strings.Index(text, "$ $ "+from+"\n")
	if i < 0 {
		t.Fatalf("terminal.log has no %s", from)
	}

	text = text[i+len("$ $ "+from+"\n"):]
	if end := strings.Index(text, "\n$ "); end >= 0 {
		text = text[:end+1]
	}

	return text
}

// TestBatchRestartAcrossSessions replays the probe's reboot as two govax
// sessions on one volume and queue database: the first submits BOOTR and
// BOOTN and ends while both run; the second finds them as VMS 7.1 did after
// its boot, starts the queue, and BOOTR's restart writes VMS's log.
func TestBatchRestartAcrossSessions(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	dsk := filepath.Join(t.TempDir(), "bat98.dsk")
	path := filepath.Join(t.TempDir(), "qman.json")

	c, dispatch := batchSession(t, dsk, path, true)
	probeQueueA(t, dispatch)

	if err := jobqueue.Update(path, func(d *jobqueue.Database) error {
		d.NextEntry = 417

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, s := range []struct{ line, want string }{
		{"SUBMIT/QUEUE=BAT98_A/RESTART/NOPRINTER/KEEP/NAME=BOOTR/LOG_FILE=BAT98$DISK:[BAT98LOG]BOOTR.LOG/PARAMETERS=BOOTR BAT98$DISK:[000000]BAT98R",
			"Job BOOTR (queue BAT98_A, entry 417) started on BAT98_A\n"},
		{"SUBMIT/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=BOOTN/LOG_FILE=BAT98$DISK:[BAT98LOG]BOOTN.LOG/PARAMETERS=(BOOTN,600) BAT98$DISK:[000000]BAT98W",
			"Job BOOTN (queue BAT98_A, entry 418) started on BAT98_A\n"},
	} {
		if got := dispatch(s.line); got != s.want {
			t.Fatalf("%s:\n got %q\nwant %q", s.line, got, s.want)
		}
	}

	for _, name := range []string{"BOOTR", "BOOTN"} {
		if got := await(dispatch, name); !strings.Contains(got, "STARTED") {
			t.Fatalf("%s never started", name)
		}
	}

	if n := c.ExecutingJobs(); n != 2 {
		t.Fatalf("%d jobs executing, want 2", n)
	}

	// The session ends as govax ends one (cmd/govax: confirmEnd, then
	// EndSession), its volume dismounted.
	requeued, ended, err := c.EndBatchJobs()
	if err != nil || requeued != 1 || ended != 1 {
		t.Fatalf("EndBatchJobs: %d requeued, %d ended, %v", requeued, ended, err)
	}

	c.EndSession()

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	// The next session: its manager starts by itself.
	c2, dispatch2 := batchSession(t, dsk, path, false)
	t.Cleanup(func() { _ = c2.Mounts.DismountAll() })

	if !c2.RTL.System.QueueManager().Running() {
		t.Fatal("the manager doesn't run in the second session")
	}

	show := "SHOW QUEUE/FULL/ALL BAT98_A"
	want := dropEntries(terminalSection(t, "$ $ DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK DUA1:\n", show), "48", "49", "53", "54", "56", "59", "60", "78")

	if got := dispatch2(show); maskBatch(got) != maskBatch(want) {
		t.Errorf("after the restart, %s:\n got %q\nwant %q", show, got, want)
	}

	if got := dispatch2("START/QUEUE BAT98_A"); got != "" {
		t.Errorf("START/QUEUE: %q", got)
	}

	if got := dispatch2("SYNCHRONIZE/ENTRY=417"); got != "" {
		t.Errorf("SYNCHRONIZE: %q", got)
	}

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bat98", "vax", "bootr.log"))
	if err != nil {
		t.Fatal(err)
	}

	want = strings.ReplaceAll(string(data), "\r", "")
	if got := dispatch2("TYPE BAT98$DISK:[BAT98LOG]BOOTR.LOG;2"); maskBatch(got) != maskBatch(want) {
		t.Errorf("BOOTR's restart:\n got %q\nwant %q", got, want)
	}
}

// TestBatchRecoverAfterCrash: a session that ended without ending its jobs
// (govax killed) leaves them executing in the database; the next session
// to run the manager ends them as VMS's boot did, whatever the order of
// its queues, and leaves waiting jobs as they were.
func TestBatchRecoverAfterCrash(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	dsk := filepath.Join(t.TempDir(), "bat98.dsk")
	path := filepath.Join(t.TempDir(), "qman.json")

	c, dispatch := batchSession(t, dsk, path, true)
	probeQueueA(t, dispatch)

	for _, line := range []string{
		"SUBMIT/QUEUE=BAT98_A/HOLD/NOPRINTER/NAME=HELD BAT98$DISK:[000000]BAT98OK",
		"SUBMIT/QUEUE=BAT98_A/HOLD/NOPRINTER/NAME=GONE BAT98$DISK:[000000]BAT98OK",
		"SUBMIT/QUEUE=BAT98_A/HOLD/NOPRINTER/RESTART/NAME=AGAIN BAT98$DISK:[000000]BAT98OK",
	} {
		dispatch(line)
	}

	c.EndSession()

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	// What the killed session left: GONE and AGAIN executing, AGAIN with
	// a restart value, and the manager's lock let go.
	c.RTL.System.QueueManager().Close()

	if err := jobqueue.Update(path, func(d *jobqueue.Database) error {
		for _, j := range d.Jobs {
			if j.Name != "HELD" {
				j.Status, j.Started = jobqueue.JobExecuting, time.Now()
			}

			if j.Name == "AGAIN" {
				j.RestartValue = "STEP2"
			}
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	c2, dispatch2 := batchSession(t, dsk, path, false)
	t.Cleanup(func() { _ = c2.Mounts.DismountAll() })

	got := dispatch2("SHOW QUEUE/ALL BAT98_A")

	for _, want := range []string{
		"Batch queue BAT98_A, stopped, on GOVAX::",
		"AGAIN           SYSTEM               Pending",
		"HELD            SYSTEM               Holding",
		"GONE            SYSTEM               Retained on error",
		"%JBC-F-QUERESET, job aborted during execution because queue was reset",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SHOW QUEUE has no %q:\n%s", want, got)
		}
	}

	d, err := jobqueue.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, j := range d.Jobs {
		if j.Name == "AGAIN" && (!j.Restarted || !j.Checkpointed) {
			t.Errorf("AGAIN not a checkpointed restart: %+v", j)
		}
	}
}

// TestBatchSecondSession: while one session runs the queue manager,
// another sharing its database may show the queues but change nothing
// (Decision 4): JBC$_QMANNOTSTARTED, its manager not started there.
func TestBatchSecondSession(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	path := filepath.Join(t.TempDir(), "qman.json")

	c, dispatch := batchSession(t, filepath.Join(t.TempDir(), "a.dsk"), path, true)
	probeQueueA(t, dispatch)

	// The first session holds the manager's lock for as long as it lives:
	// the console must, for the test, or the lock file's finalizer lets it
	// go.
	t.Cleanup(func() { runtime.KeepAlive(c) })

	c2, dispatch2 := batchSession(t, filepath.Join(t.TempDir(), "b.dsk"), path, true)

	if c2.RTL.System.QueueManager().Running() {
		t.Fatal("the manager runs in both sessions")
	}

	if got := dispatch2("SHOW QUEUE BAT98_A"); !strings.HasPrefix(got, "Batch queue BAT98_A, idle, on GOVAX::") {
		t.Errorf("SHOW QUEUE: %q", got)
	}

	for _, line := range []string{
		"STOP/QUEUE BAT98_A",
		"SUBMIT/QUEUE=BAT98_A BAT98$DISK:[000000]BAT98OK",
		"START/QUEUE/MANAGER",
	} {
		if got := dispatch2(line); !strings.Contains(got, "QMANNOTSTARTED") {
			t.Errorf("%s: %q", line, got)
		}
	}
}

// TestSystartupBatchBlock runs the batch queue lines of the
// SYSTARTUP_VMS.COM govax supplies, with their "$! " taken off as its
// comment says: the manager started, SYS$BATCH made and started, so that
// SUBMIT without /QUEUE runs a job. Run again, as at the next start, they
// find SYS$BATCH and don't make it again.
func TestSystartupBatchBlock(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	text, err := fs.ReadFile(bootdata.FS, "systartup_vms.com")
	if err != nil {
		t.Fatal(err)
	}

	_, block, ok := strings.Cut(string(text), "$!  -- batch queue: begin --\n")
	block, _, found := strings.Cut(block, "$!  -- batch queue: end --\n")

	if !ok || !found {
		t.Fatal("SYSTARTUP_VMS.COM has no batch queue block")
	}

	lines := make([]string, 0)
	
	for _, line := range strings.SplitAfter(strings.TrimSuffix(block, "\n"), "\n") {
		rest, ok := strings.CutPrefix(line, "$! ")
		if !ok {
			t.Fatalf("a line not commented as the block's comment says: %q", line)
		}

		lines = append(lines, rest)
	}

	host := filepath.Join(t.TempDir(), "batchq.com")
	if err := os.WriteFile(host, []byte(strings.Join(lines, "")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, dispatch := batchConsole(t)

	if got := dispatch(`COPY "` + host + `"/HOST DUA1:[SYSMGR]BATCHQ.COM/QUIET`); got != "" {
		t.Fatalf("COPY: %s", got)
	}

	if got := dispatch("@DUA1:[SYSMGR]BATCHQ"); got != "" {
		t.Errorf("the block: %q", got)
	}

	if got := dispatch("SHOW QUEUE SYS$BATCH"); !strings.HasPrefix(got, "Batch queue SYS$BATCH, idle, on GOVAX::") {
		t.Errorf("SHOW QUEUE: %q", got)
	}

	if got := dispatch("SUBMIT/NOPRINTER BAT98$DISK:[000000]BAT98OK"); !strings.Contains(got, "started on SYS$BATCH") {
		t.Errorf("SUBMIT: %q", got)
	}

	// Again, as the next session's startup runs it: SYS$BATCH stopped as
	// a session's start leaves it, so START/QUEUE starts it again.
	dispatch("STOP/QUEUE/NEXT SYS$BATCH")

	if got := dispatch("@DUA1:[SYSMGR]BATCHQ"); got != "" {
		t.Errorf("the block again: %q", got)
	}
}
