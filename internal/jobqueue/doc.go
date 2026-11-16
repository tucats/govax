// Package jobqueue is govax's queue database: the batch queues, the jobs
// waiting in them or running, and the rules that say what may happen to
// each (docs/planning/PHASE-98 - batch queues.md, Decisions 2, 3, and 11).
//
// # What VMS keeps
//
// On VMS the queue manager, a detached process, keeps its database in
// files of its own (QMAN$MASTER.DAT and the queue and journal files that
// START/QUEUE/MANAGER names). A queue is a name, a kind (batch, printer,
// server, terminal, generic), its attributes (how many jobs it runs at
// once, the base priority its jobs get, CPU and working-set limits, which
// finished jobs it keeps), its protection (an owner, a protection code,
// and an access control list), and its state: stopped, started (idle,
// available, or busy, by how many jobs run), paused, or stopping after
// the jobs it is running. A job is an entry number, unique across the
// manager's queues, a name, the user it runs as, its files (command
// procedures, held by file ID), the parameters P1 to P8 every file gets,
// its log file, its options, and its state: pending, holding, holding
// until a time, executing, or retained after it ended. The database
// outlives a reboot.
//
// # What govax keeps
//
// govax keeps the same facts in one host file of its own format, JSON,
// as it keeps the authorization file (internal/uaf): by default
// ~/.govax/qman.json, or the file vax.queue.file names. A Database is the
// whole of it: the manager's state (Manager), the queues (Queue), the
// jobs (Job), and the counters that number entries and order
// submissions. Load reads a file whole and checks it, naming the line of
// anything wrong; Save writes it whole through a temporary file and a
// rename; Update is a read, a change, and a write under a lock
// (internal/hostlock), so that two govax sessions sharing the file never
// lose each other's changes. LockManager is the other lock, the one the
// session running the queue manager holds for as long as it runs
// (Decision 4): only that session starts jobs.
//
// Only batch queues are kept (Decision 1): no printer, server, terminal,
// or generic queues, forms, or characteristics.
//
// # The rules
//
// What an operation may do depends on the state of the queue or job it
// is about, and each refusal is a JBC condition, as VMS's job controller
// returns it. The rules are two tables, queueRules (queue.go) and
// jobRules (job.go), a row per operation and a column per state, so the
// whole of the behavior can be read in one place, and the tests walk
// every cell. A refusal is a *Refusal naming its JBC$_ condition.
//
// The rest of the rules are which job a queue runs next (NextJob: a
// pending job, highest priority first, then in the order submitted, as
// VMS 7.1 ran them, the phase doc's A.4), when a job held until a time
// becomes pending (ReleaseDue, NextTime), what happens to a job when it
// ends (Finish: requeued, retained, or gone, by the job's and the queue's
// /RETAIN), and what a new session does with what the last one left
// running (Recover: every queue stopped; a job submitted /RESTART
// requeued, any other ended with JBC$_QUERESET).
//
// # What it doesn't know
//
// The package knows nothing of processes, DCL, RMS, or the VAX: times are
// Go times, users are names and UICs, files are names and file IDs, and a
// job's status is a condition value. corevms's queue manager (Decision 5)
// asks it what to do and does it: creating the batch process, stopping
// it, and telling Finish how it ended. Nor does it check access: whether
// the asker may do what it asks is the queue's protection, which corevms
// checks with internal/security's rule before calling an operation here.
package jobqueue
