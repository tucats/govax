package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
)

// withQueueFile gives one test a queue database of its own, put back
// when it ends.
func withQueueFile(t *testing.T, path string) {
	t.Helper()

	saved := queueFile
	queueFile = path

	t.Cleanup(func() { queueFile = saved })
}

// TestRun_queueFile: a first start makes the database's directory but
// not the file (an empty database until a queue is made); a database
// govax can read starts; one it can't stops startup with the file's name
// and the line (docs/planning/PHASE-98 - batch queues.md, Decision 3).
func TestRun_queueFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "qman.json")
	withQueueFile(t, path)

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("no directory: %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the file was made: %v", err)
	}

	err := jobqueue.Update(path, func(d *jobqueue.Database) error {
		_, _, err := d.CreateQueue("SYS$BATCH", nil)

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run on a good file: %v\n%s", err, buf.String())
	}

	data, _ := os.ReadFile(path)
	bad := strings.Replace(string(data), `"job_limit": 1`, `"job_limit": 0`, 1)

	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	err = run(nil, 0, 0, &buf, emptyStdin(), nil)

	var pe *jobqueue.ParseError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), path) {
		t.Errorf("run on a bad file = %v, want a ParseError naming %s", err, path)
	}

	if strings.Contains(buf.String(), "STARTUP") {
		t.Errorf("startup went on:\n%s", buf.String())
	}
}

// TestRun_endWithJobs: a session that ends while its batch jobs run, with
// no terminal to ask at, says so and ends them as a VMS system's stop
// does: a /RESTART job requeued as a restart, any other ended with
// JBC$_QUERESET (docs/planning/PHASE-98 - batch queues.md, Decision 11).
// The next session starts the manager by itself and finds its queue
// stopped and the requeued job waiting.
func TestRun_endWithJobs(t *testing.T) {
	dir := t.TempDir()
	withAuthorizationFile(t, filepath.Join(dir, "sysuaf.json"))
	withSystemVolume(t, systemFlags{file: filepath.Join(dir, "govax-system.dsk")})

	path := filepath.Join(dir, "qman.json")
	withQueueFile(t, path)

	loop := filepath.Join(dir, "loop.com")
	if err := os.WriteFile(loop, []byte("$ LOOP:\n$ WAIT 0:0:1\n$ GOTO LOOP\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := session(t, strings.Join([]string{
		`COPY "` + loop + `"/HOST SYS$LOGIN:LOOP.COM`,
		"START/QUEUE/MANAGER",
		"INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2/START SYS$BATCH",
		"SUBMIT/NOPRINTER/RESTART/NAME=AGAIN LOOP",
		"SUBMIT/NOPRINTER/RETAIN=ERROR/NAME=ONCE LOOP",
		"WAIT 0:0:0.5",
		"EXIT",
	}, "\n")+"\n")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}

	for _, want := range []string{
		"%VAX-W-JOBSEXECUTING, 2 batch jobs executing; ending govax ends them",
		"%VAX-I-JOBSENDED, batch jobs requeued to restart: 1; ended (queue reset): 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}

	out, err = session(t, "SHOW QUEUE/ALL SYS$BATCH\nEXIT\n")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}

	for _, want := range []string{
		"Batch queue SYS$BATCH, stopped, on GOVAX::",
		"AGAIN           SYSTEM               Pending",
		"ONCE            SYSTEM               Retained on error",
		"%JBC-F-QUERESET",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the next session: no %q in:\n%s", want, out)
		}
	}

	if strings.Contains(out, "JOBSEXECUTING") {
		t.Errorf("the next session ended with jobs executing:\n%s", out)
	}
}
