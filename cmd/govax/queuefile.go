package main

import (
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/uaf"
)

// queueFile, when set, is the queue database's path in place of
// vax.queue.file's and the default: the tests' (TestMain gives each run a
// temporary one), never the author's own.
var queueFile string

// ensureQueueFile returns the queue database's host path (queueFile, else
// vax.queue.file, else ~/.govax/qman.json, "~/" the home directory),
// checking the file if it is there and making its directory if not
// (docs/planning/PHASE-98 - batch queues.md, Decision 3). A missing file
// is an empty database, written the first time a queue is made; one that
// exists but can't be read is an error naming the file and the line, so
// govax doesn't start and lose the queues and jobs in it.
func ensureQueueFile() (string, error) {
	path := queueFile
	if path == "" {
		path = strings.TrimSpace(settings.Get("vax.queue.file"))
	}

	if path == "" {
		path = jobqueue.DefaultPath
	}

	path, err := uaf.ExpandHome(path)
	if err != nil {
		return "", err
	}

	if err := jobqueue.Ensure(path); err != nil {
		return "", err
	}

	return path, nil
}
