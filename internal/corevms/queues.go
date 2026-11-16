package corevms

// The queue database (docs/planning/PHASE-98 - batch queues.md,
// Decisions 2 to 5): a host file of internal/jobqueue's, which the
// System's queue manager will keep (subtask 6). cmd/govax finds and
// checks the file before the machine starts, and hands its path down.

// SetQueueFile makes path, a host file, the System's queue database; ""
// is none (the tests', until a test gives one). The file need not exist:
// a missing file is an empty database.
func (s *System) SetQueueFile(path string) { s.queueFile = path }

// QueueFile is the queue database's host path, "" when there is none.
func (s *System) QueueFile() string { return s.queueFile }
