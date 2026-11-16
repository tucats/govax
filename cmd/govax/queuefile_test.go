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
