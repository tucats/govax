package corevms

import (
	"io"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Jobs (docs/PHASE-45.md, subtask 2).
//
// On VMS a "job" is a process tree: a detached process (one with no
// owner, such as the process a user logs in to) and every subprocess
// created under it, directly or indirectly. The processes of a job share
// one job information block (JIB), which holds what the job has once:
// its master process's PID, the "pooled" quotas every process in the job
// draws on together (the subprocess limit among them), and, later, the
// job's logical-name table. Creating a detached process allocates a new
// JIB; creating a subprocess points the new process at its creator's.
//
// Each process also records, in its own PCB, the PID of the process that
// created it (its owner; 0 for a detached process) and how many
// subprocesses it has created itself. The JIB's count is of every
// subprocess in the job. (VAX/VMS Internals and Data Structures, section
// 20.1.1, steps 3, 8, 17, and 19, and figure 20-2; section 20.1.2 and
// table 20-3 for which quotas are pooled.)

// Job is one job's shared state: VMS's JIB.
type Job struct {
	// MasterPID is the PID of the job's detached process, the root of
	// its process tree (JIB$L_MPID). $GETJPI reports it as
	// JPI$_MASTER_PID.
	MasterPID uint32

	// SubprocessLimit is the most subprocesses the job may have at once
	// (the PRCLM quota, JIB$W_PRCLIM), and SubprocessCount how many it
	// has now (JIB$W_PRCCNT). Creating a subprocess that would take the
	// count past the limit fails (reserveSubprocess).
	SubprocessLimit uint32
	SubprocessCount uint32

	// Pooled are the job's other pooled quotas (table 20-3): limits
	// every process in the job shares. They're recorded, for $GETJPI
	// and $CREPRC, but not enforced.
	Pooled PooledQuotas
}

// PooledQuotas are the limits a job's processes share, as $CREPRC's
// quota list names them (the PQL$_ codes). Each is a count of the
// resource the quota limits: bytes of buffered I/O (BYTLM), open files
// (FILLM), paging-file pages (PGFLQUOTA), timer queue entries (TQELM),
// lock requests (ENQLM), and bytes of the job logical-name table
// (JTQUOTA).
type PooledQuotas struct {
	BYTLM, FILLM, PGFLQUOTA, TQELM, ENQLM, JTQUOTA uint32
}

// Nominal job quotas: those of process 1, and of every detached process
// govax creates, standing in for the SYSTEM account's authorization-file
// (UAF) entry. They're in the range VMS's SYSTEM account uses, not taken
// from any particular system's UAF.
const (
	nominalPRCLM = 10

	nominalBYTLM     = 32768
	nominalFILLM     = 100
	nominalPGFLQUOTA = 50000
	nominalTQELM     = 20
	nominalENQLM     = 300
	nominalJTQUOTA   = 4096
)

// ssExQuota is SS$_EXQUOTA: what $CREPRC returns when the job already
// has as many subprocesses as its PRCLM quota allows (the VMS 5.0 System
// Services Reference Manual's $CREPRC entry). The same entry gives
// SS$_EXPRCLM a different meaning: a user's limit on detached processes
// (the UAF's MAXDETACH), which govax doesn't have.
var ssExQuota = vmsdef.Symbols["SS$_EXQUOTA"]

// newJob returns a new job whose master process has PID master, with the
// nominal quotas and no subprocesses.
func newJob(master uint32) *Job {
	return &Job{
		MasterPID:       master,
		SubprocessLimit: nominalPRCLM,
		Pooled: PooledQuotas{
			BYTLM:     nominalBYTLM,
			FILLM:     nominalFILLM,
			PGFLQUOTA: nominalPGFLQUOTA,
			TQELM:     nominalTQELM,
			ENQLM:     nominalENQLM,
			JTQUOTA:   nominalJTQUOTA,
		},
	}
}

// reserveSubprocess counts one more subprocess in the job, if its PRCLM
// quota allows one more, and reports whether it did. A creation that
// then fails gives the reservation back with releaseSubprocess.
func (j *Job) reserveSubprocess() bool {
	if j.SubprocessCount >= j.SubprocessLimit {
		return false
	}

	j.SubprocessCount++

	return true
}

// releaseSubprocess counts one subprocess fewer in the job.
func (j *Job) releaseSubprocess() {
	if j.SubprocessCount > 0 {
		j.SubprocessCount--
	}
}

// NewSubprocess returns a new process created by owner, a subprocess in
// owner's job, added to the process table (which gives it its PID) as
// NewEnvironment adds a process. It takes owner's user name, account, and
// UIC, as a subprocess does on VMS; everything else starts as a new
// process's does. Its logical names are a new view of owner's database
// (lnm.Database.NewProcessView): an empty process table of its own, and
// the job, group, and system tables shared with owner. consoleIn and
// consoleOut are its terminal input and output, as for NewEnvironment.
//
// The job counts it against its PRCLM quota: SS$_EXQUOTA if the job
// already has as many subprocesses as the quota allows. SS$_NOSLOT if the
// process table is full. Either way nothing is created.
func NewSubprocess(owner *Environment, consoleIn io.Reader, consoleOut io.Writer) (*Environment, error) {
	job := owner.Process.Job
	if !job.reserveSubprocess() {
		return nil, vmserrors.New(ssExQuota)
	}

	logicals := owner.Logicals.NewProcessView(owner.Process.UIC, owner.Logicals.JobTableName)
	env := newEnvironment(owner.System, logicals, consoleIn, consoleOut)

	p := env.Process
	p.Username, p.Account, p.UIC = owner.Process.Username, owner.Process.Account, owner.Process.UIC
	p.Job, p.Owner = job, owner.Process.PID

	if err := owner.addProcess(env); err != nil {
		job.releaseSubprocess()

		return nil, err
	}

	owner.Process.SubprocessCount++

	return env, nil
}

// leaveJob takes env's process out of its job's counts as it leaves the
// process table: a subprocess is one subprocess fewer for its owner and
// for the job. (A detached process's job ends with it; on VMS its
// subprocesses are deleted before it, docs/PHASE-45.md, subtask 8.)
func (sys *System) leaveJob(env *Environment) {
	p := env.Process
	if p.Owner == 0 || p.Job == nil {
		return
	}

	p.Job.releaseSubprocess()

	if owner, ok := sys.FindProcess(p.Owner); ok && owner.Process.SubprocessCount > 0 {
		owner.Process.SubprocessCount--
	}
}
