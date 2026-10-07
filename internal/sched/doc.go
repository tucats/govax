// Package sched holds VMS's process-scheduling rules as plain Go, with no
// CPU or VMS dependency, so they can be tested on their own (Phase 44;
// docs/PHASE-44.md, and docs/PHASE-43.md's Part A for the program it
// belongs to).
//
// A VMS system has many processes but, on a uniprocessor VAX, only one
// runs at a time: the *current* process. The others are *computable*
// (ready, waiting only for the CPU) or *waiting* for something: an event
// flag, a wakeup, a resource. The scheduler decides which computable
// process runs next and when the current one must give the CPU up.
//
// The rules here are VMS's, as DIGITAL's book *VAX/VMS Internals and Data
// Structures* (Kenah and Bate, version 3.3 edition; chapter 10,
// "Scheduling") describes them:
//
//   - Every process has a base priority and a current priority, 0 to 31,
//     higher meaning more important. 0-15 are "normal" priorities, 16-31
//     "real-time" (section 10.1.2).
//   - Computable processes wait in one first-in, first-out queue per
//     priority. The next process to run is the one at the head of the
//     highest non-empty queue (section 10.3.3).
//   - When a waiting normal process's event happens, its current priority
//     is boosted by an amount that depends on the kind of event (Table
//     10-3), but never above 15, and never below what it already was.
//     Real-time processes are never boosted (section 10.2.4).
//   - Each time a normal process above its base priority is chosen to
//     run, its current priority first drops by one, so a process that
//     keeps the CPU drifts back down to its base (section 10.3.3, step 3).
//   - A normal process runs until it waits, is preempted by a computable
//     process of higher *or equal* priority, or uses up its quantum, and
//     then goes to the back of its priority's queue: so processes of
//     equal priority take turns (round robin). A real-time process has no
//     quantum end: it runs until it waits or a higher-or-equal priority
//     process preempts it (sections 10.1.2.1, 10.1.2.2, 10.1.2.4).
//
// The Scheduler doesn't run anything itself. Its caller (the CPU's
// scheduling hook and the system services, in later subtasks) tells it
// what happened — an instruction executed, a process started waiting, an
// event ended a wait — and asks it which process should run. Processes
// are named by opaque Handles that the caller chooses (govax uses process
// IDs); the Scheduler keeps only what scheduling needs to know about each.
//
// What the Scheduler leaves out, because govax has no use for it: memory
// residence (VMS's outswapped states COMO, LEFO, HIBO, SUSPO and the
// swapper), the page-fault waits (govax has no paging), and IOTA, the
// extra quantum VMS charges a process each time it waits. The codes for
// the outswapped and paging states are defined anyway, since $GETJPI and
// SHOW SYSTEM report states by these numbers.
package sched
