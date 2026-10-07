package corevms

import (
	"fmt"

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
//     11; subtask 7);
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
// Subprocesses are left alone for now: deleting them with their owner,
// and $DELPRC of another process, are subtask 8's.

// DeleteProcess deletes env's process (see above), unless it already has
// been. Process 1, the console's, is never deleted: callers end its image
// instead.
func (sys *System) DeleteProcess(env *Environment) {
	if env.Deleted {
		return
	}

	// Charge the CPU time used so far, while env is (as a rule) still the
	// process the CPU holds: the unused CPU time it gives back, and its
	// termination message, depend on it.
	sys.accountTime()

	env.ImageRundown()
	env.processRundown()
	sys.returnQuotas(env)

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
