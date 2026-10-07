package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// Process deletion (docs/PHASE-45.md, subtask 6).
//
// On VMS, a process created to run an image (one with no command
// interpreter) is deleted when the image exits, and $DELPRC deletes any
// process. Almost all of the work happens in the context of the process
// being deleted, in a kernel-mode AST, so that its address space and
// process header are at hand (VAX/VMS Internals and Data Structures,
// chapter 22, section 22.2.1). In order, that AST
//
//   - runs the image down, and RMS's rundown closes every open file
//     (steps 2 and 3; ImageRundown's CloseFiles);
//   - runs the process down: every channel, in every access mode, is
//     deassigned, every device it allocated deallocated (steps 5 and 8);
//   - gives back to the owner, if the process is a subprocess, what the
//     owner gave it that the process didn't use, which is only CPU time:
//     the other quotas are pooled or nondeductible (step 10);
//   - sends the termination message, if the creator asked for one (step
//     11; termmsg.go);
//   - takes the process out of the scheduler (SVPCTX), makes the null
//     process current, frees its slot in the PCB vector, and then frees
//     the pages it kept to the end, its kernel stack among them (steps
//     14 to 16);
//   - takes it out of its job's counts, deallocating the job itself when
//     the process was the job's master (step 19), and its owner's
//     subprocess count (step 20), and frees the PCB (step 21).
//
// govax does the same in Go, in DeleteProcess, run in the process's
// context when the process deletes itself (its image ended). The pages
// the CPU is still using, those of the process's page tables, stacks, and
// PCB, are the last thing to go: they're freed as soon as the CPU holds
// another process (switchTo), which the scheduler chooses next because
// the deleted process has left it. A deleted process that isn't the
// current one gives everything back at once.
//
// A process that owns subprocesses can't be deleted before them: they
// hold quotas it lent them, and they belong to its job. So the deletion
// first marks each of them for deletion and, while any is left, waits
// (step 4, and section 22.2.2's example): each subprocess, deleted in
// its own context, leaves its owner's count, and when the last has gone,
// the owner's deletion goes on. A tree of subprocesses is deleted from
// its leaves up. (VMS runs the owner's RMS rundown before step 4; govax
// waits first and runs the whole rundown once, which no program can tell
// apart.)
//
// $DELPRC of another process (section 22.1.1) only marks it for
// deletion (markForDeletion). The deletion runs, as above, the next time
// the scheduler gives the process the CPU (switchTo): VMS's special
// kernel-mode AST, queued to the target, which makes it computable with
// a boost of 3, and which is the first thing it runs.

// DeleteProcess deletes env's process (see above), unless it already has
// been. If env owns subprocesses, they are marked for deletion and env
// waits for them to go (awaitSubprocesses): its own deletion goes on
// when the scheduler next runs it. Process 1, the console's, is never
// deleted: callers end its image instead.
func (sys *System) DeleteProcess(env *Environment) {
	if env.Deleted || sys.awaitSubprocesses(env) {
		return
	}

	env.deletePending = false

	// Charge the CPU time used so far, while env is (as a rule) still the
	// process the CPU holds: the unused CPU time it gives back, and its
	// termination message, depend on it.
	sys.accountTime()

	env.ImageRundown()
	env.processRundown()
	sys.returnQuotas(env)
	sys.sendTerminationMessage(env)

	if env.waiting != nil {
		env.waiting = nil
		sys.waiters--
	}

	env.pendingWait = nil
	env.Startup = nil
	env.Deleted = true

	sys.endJob(env)
	sys.removeFromTable(env)

	// Every service now finds the process gone. Its memory goes now, or,
	// if the CPU is still running in it, once the CPU leaves it.
	if sys.Current() != env {
		sys.releaseMemory(env)
	}

	if sys.ProcessDeleted != nil {
		sys.ProcessDeleted(env)
	}

	sys.requestReschedule()

	if sys.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): %08X deleted, status %08X\n",
			env.Process.PID, env.Process.ExitStatus)
	}
}

// markForDeletion is $DELPRC's part in its caller's context, for a
// process other than the caller (VAX/VMS Internals and Data Structures,
// section 22.1.1): env is marked for deletion, unless it already is, and
// made computable if it's waiting, so that the scheduler soon gives it
// the CPU and its deletion runs (switchTo). Its final status, the one its
// termination message reports, is SS$_ABORT, unless its image has
// already called $EXIT with a status of its own: VMS's for a process
// deleted with its image unfinished is unconfirmed.
//
// Without the scheduler, nothing would ever give env the CPU: it's
// deleted at once.
//
// (VMS also resumes a suspended process here, or its AST could never be
// delivered; $SUSPND of another process is subtask 9's.)
func (sys *System) markForDeletion(env *Environment) {
	if env.Deleted || env.deletePending {
		return
	}

	env.deletePending = true

	if env.Process.ExitStatus == 0 {
		env.Process.ExitStatus = ssAbort
	}

	if sys.engine == nil {
		sys.DeleteProcess(env)

		return
	}

	if env.waiting != nil {
		env.endWait(sched.ClassResourceAvailable)
	}

	sys.requestReschedule()

	if sys.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): %08X marked for deletion\n", env.Process.PID)
	}
}

// subprocessesOf returns the processes env owns: those whose owner is
// env's PID. As on VMS, nothing in the owner points to them, so the
// whole table is searched (the book's section 22.2.2).
func (sys *System) subprocessesOf(env *Environment) []*Environment {
	var subs []*Environment

	for _, p := range sys.procs.slots {
		if p != nil && p != env && p.Process.Owner == env.Process.PID {
			subs = append(subs, p)
		}
	}

	return subs
}

// awaitSubprocesses is the deletion's step 4: every subprocess env owns
// is marked for deletion, and if any is left, env waits until none is
// (state MWAIT, resource RWAST: VMS's owner waits for the special
// kernel-mode AST each subprocess sends it as it goes), marked for
// deletion itself, so that the scheduler finishes its deletion when it
// next runs it (switchTo). It reports whether env must wait. While env
// waits, it takes no ASTs: its image is over.
//
// Without the scheduler, the subprocesses have already been deleted (by
// markForDeletion), and there is nothing to wait for.
func (sys *System) awaitSubprocesses(env *Environment) bool {
	subs := sys.subprocessesOf(env)
	if len(subs) == 0 {
		return false
	}

	for _, sub := range subs {
		sys.markForDeletion(sub)
	}

	if len(sys.subprocessesOf(env)) == 0 {
		return false
	}

	w := &waitCondition{
		state:      sched.StateMWAIT,
		resource:   sched.ResourceAST,
		class:      sched.ClassResourceAvailable,
		over:       func() bool { return len(sys.subprocessesOf(env)) == 0 },
		ignoreASTs: true,
	}

	if err := sys.sched.Wait(handle(env), w.state, w.resource); err != nil {
		return false // not in the scheduler: nothing to wait in
	}

	if env.waiting == nil {
		sys.waiters++
	}

	env.waiting = w
	env.deletePending = true

	sys.requestReschedule()

	if sys.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): %08X waits for %d subprocess(es) to be deleted\n",
			env.Process.PID, len(subs))
	}

	return true
}

// processRundown is what process deletion releases beyond image rundown
// (ImageRundown, which has already closed the process's files, and
// leaves a process's privileged-mode state for its next image): every
// channel, in any access mode, so a temporary mailbox the process held
// the last channel to is deleted; every device it allocated; and every
// AST, exit handler, and I/O request still queued, in any mode.
func (env *Environment) processRundown() {
	for _, c := range append([]*channel(nil), env.channels...) {
		env.releaseChannel(c)
	}

	env.deallocateAll(uint32(vax.Kernel))

	p := env.Process
	p.ast.queue = nil
	p.exitHandlers = [4][]uint32{}

	env.pendingIO = nil
	env.qiowWaits = nil
}

// cpuQuotaUnit is the CPU time limit's unit, 10 milliseconds, in VMS
// time units (100 nanoseconds).
const cpuQuotaUnit = 100_000

// returnQuotas gives env's owner back what it lent env, if env is a
// subprocess whose owner still exists: the part of the CPU time limit it
// took from its owner (Process.cpuDeducted) that env didn't use. The
// book's step 10 says that's the only quota to return; a detached
// process returns nothing, even when its creator's limit paid for its
// own.
func (sys *System) returnQuotas(env *Environment) {
	p := env.Process
	lent := p.cpuDeducted
	p.cpuDeducted = 0

	if lent == 0 || p.Owner == 0 {
		return
	}

	owner, ok := sys.FindProcess(p.Owner)
	if !ok {
		return
	}

	if used := env.cpuTime / cpuQuotaUnit; used < uint64(lent) {
		owner.Process.CPULimit += lent - uint32(used)
	}
}

// endJob ends env's job if env is its master, a detached process: the
// job's logical-name table, made when $CREPRC created the process, is
// deleted with every name in it (the book's step 19). A job whose table
// govax didn't make for it keeps it: process 1's, and those of the
// processes tests make, which share process 1's names.
func (sys *System) endJob(env *Environment) {
	p := env.Process
	if p.Owner != 0 || p.Job == nil || p.Job.MasterPID != p.PID || p.Job.LogicalTable == "" {
		return
	}

	env.Logicals.DeleteJobTable(p.Job.LogicalTable)
	p.Job.LogicalTable = ""
}

// releaseMemory gives back the memory env's process was given: the
// physical pages of its address space and its page tables
// (TeardownAddressSpace), and every S0 pool page charged to its PID (its
// stacks, its PCB, its image's driver, and its page tables, if their
// teardown failed). The CPU must not be running in the process. A second
// call finds nothing to free.
func (sys *System) releaseMemory(env *Environment) {
	// A failure here (the space is the CPU's current one) leaves the
	// frames allocated, a leak rather than a corruption; FreeProcess below
	// still gives back the tables' pages.
	_ = sys.TeardownAddressSpace(env.Space)

	if sys.s0 != nil {
		sys.s0.FreeProcess(env.Process.PID)
	}
}

// CloseFiles closes every file the process has open, as RMS's rundown
// does at image exit and process deletion (VAX/VMS Internals and Data
// Structures, section 22.2.1, step 3): its RMS files, so that what it
// wrote and never closed is on the volume (rms.FileTable.Rundown), and
// the host files it opened through the C library's descriptors, which on
// VMS are RMS files too. VMS keeps a process-permanent file, one opened
// in executive mode (by DCL), open across images; govax doesn't record
// the mode a file was opened in, and every file its programs open is the
// image's, so it closes them all. Its terminal stays open. A process
// with no files open is left as it was.
func (env *Environment) CloseFiles() {
	if n, err := env.files.Rundown(); n > 0 && env.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG(PROCESS): %08X's rundown closed %d file(s), error %v\n",
			env.Process.PID, n, err)
	}

	for fd, f := range env.openFiles {
		_ = f.Close()

		delete(env.openFiles, fd)
	}
}
