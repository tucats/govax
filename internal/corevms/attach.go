package corevms

import (
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// The terminal's holder and ATTACH (docs/PHASE-57, subtask 9).
//
// A job's processes share one terminal (its input stream), but only one
// of them has it at a time: the process the terminal is "attached" to,
// whose DCL reads the commands typed and which the control keys go to.
// The DCL Dictionary's SPAWN and ATTACH describe how it moves:
//
//   - A SPAWN that waits (LIB$SPAWN without CLI$M_NOWAIT) gives it to
//     the new subprocess, and the parent hibernates "until the
//     subprocess terminates or returns control to the parent by way of
//     the ATTACH command". A SPAWN/NOWAIT subprocess shares the terminal
//     without having it.
//   - ATTACH (DCL's command, or LIB$ATTACH) gives it to another process
//     of the job that is "hibernating as a result of the SPAWN/WAIT
//     command or another ATTACH command", and the caller hibernates in
//     turn, until some process attaches the terminal back to it.
//   - When the process that has it is deleted (LOGOUT), it goes back to
//     that process's parent.
//
// govax keeps the holder on the job (Job.terminal; nil for the job's
// master process). A process that has given the terminal away and waits
// for it to come back is awaitingTerminal: it is "detached", the only
// kind of process ATTACH may give the terminal to. Its wait (a SPAWN's,
// AwaitSpawn, or an ATTACH's, AwaitAttach) ends when the terminal is
// its again.
//
// Choices, each unconfirmed and a probe case:
//
//   - A SPAWN gives the terminal to its subprocess only when the spawner
//     has it: a SPAWN/NOWAIT subprocess's own SPAWN doesn't take it.
//   - ATTACH needs the caller to have the terminal (LIB$_ATTREQREF
//     otherwise), and so attaching to oneself is LIB$_ATTREQREF too.
//   - "Share the same input stream" isn't checked beyond the job: every
//     detached process of the job is taken to share it.
//   - A SPAWN's wait also ends when its subprocess is deleted while some
//     other process has the terminal; the spawner then goes on without
//     it, as a SPAWN/NOWAIT subprocess does.

// libAttReqRef is LIB$_ATTREQREF, "attach request refused".
var libAttReqRef = vmsdef.LibrarySymbols["LIB$_ATTREQREF"]

// Attached is the process the terminal of env's job is attached to (see
// this file's opening comment): the job's master process unless a SPAWN
// or ATTACH has given it to another. On VMS the terminal's control keys
// (Ctrl/Y, Ctrl/C) go to that process (the DCL Dictionary, SPAWN and
// ATTACH).
func (env *Environment) Attached() *Environment {
	job := env.Process.Job
	if job == nil {
		return env
	}

	if holder := job.terminal; holder != nil && !holder.Deleted {
		return holder
	}

	if master, ok := env.FindProcess(job.MasterPID); ok {
		return master
	}

	return env
}

// holdsTerminal reports whether env's job's terminal is attached to env.
func (env *Environment) holdsTerminal() bool { return env.Attached() == env }

// giveTerminal attaches env's job's terminal to target, which is told,
// should it be waiting for it.
func (env *Environment) giveTerminal(target *Environment) {
	env.Process.Job.terminal = target
	target.reportEvent(hibernateBoost)
}

// detached reports whether env is a process ATTACH may give the
// terminal to: one that gave it away (a waiting SPAWN, or an ATTACH) and
// waits for it to come back.
func (env *Environment) detached() bool {
	return env.awaitingTerminal && !env.Deleted && !env.holdsTerminal()
}

// AttachTerminal is ATTACH's first half, LIB$ATTACH's and DCL's: it gives
// the terminal of env's job to the process whose PID is pid, and returns
// SS$_NORMAL; env then waits for it to come back (AwaitAttach). It
// returns SS$_NONEXPR if there is no such process, and LIB$_ATTREQREF if
// the process isn't detached in env's job, or env doesn't have the
// terminal to give (see this file's opening comment).
func (env *Environment) AttachTerminal(pid uint32) uint32 {
	target, ok := env.FindProcess(pid)
	if !ok {
		return ssNonExpr
	}

	if target == env || target.Process.Job != env.Process.Job || !target.detached() || !env.holdsTerminal() {
		return libAttReqRef
	}

	env.awaitingTerminal, env.attachWait = true, true
	env.giveTerminal(target)

	return ssNormal
}

// AwaitAttach is ATTACH's wait, once AttachTerminal has given the
// terminal away: waiting is false when there is nothing to wait for (a
// LIB$ATTACH called for the first time). While another process has the
// terminal, err is ErrWait, env hibernating until it comes back; then err
// is nil and the wait is over.
func (env *Environment) AwaitAttach() (waiting bool, err error) {
	if !env.attachWait {
		return false, nil
	}

	if env.holdsTerminal() {
		env.attachWait, env.awaitingTerminal = false, false

		return true, nil
	}

	return true, env.waitOn(sched.StateHIB, sched.ResourceNone, hibernateBoost, env.holdsTerminal)
}

// returnTerminal is the terminal's part of env's deletion: if env has it,
// it goes back to env's parent (the nearest owner still there), or to the
// job's master process when env is the master or has no owner left.
func (sys *System) returnTerminal(env *Environment) {
	job := env.Process.Job
	if job == nil || job.terminal != env {
		return
	}

	job.terminal = nil

	for pid := env.Process.Owner; pid != 0; {
		owner, ok := sys.FindProcess(pid)
		if !ok {
			break
		}

		if !owner.Deleted {
			env.giveTerminal(owner)

			return
		}

		pid = owner.Process.Owner
	}

	if master, ok := sys.FindProcess(job.MasterPID); ok && master != env {
		master.reportEvent(hibernateBoost)
	}
}
