package corevms

// DCL's commands that change another process (docs/planning/PHASE-94 -
// privileges.md, subtask 8): SET PROCESS/PRIORITY, /SUSPEND, and /RESUME,
// each the service's work ($SETPRI, $SUSPND, $RESUME) from supervisor
// mode, DCL's, after the same check, MayAffect. What another process may
// see of one is the same rule (MaySee): SHOW PROCESS/IDENTIFICATION,
// F$GETJPI, and F$PID follow $GETJPI's. SHOW SYSTEM asks nothing: VMS's
// SHOW is installed with WORLD, so VMS 7.1 listed every process to every
// user (the probe's SEE sections).

// MaySee is whether the process may see target's details, as $GETJPI may,
// asked from supervisor mode: 0, or SS$_NOPRIV. A process may always see
// itself and any process with its UIC; GROUP lets it see its group's, and
// WORLD every process.
func (env *Environment) MaySee(target *Environment) uint32 {
	return env.mayAffectAt(target, dclMode)
}

// MayRaisePriority is SET PROCESS/PRIORITY's own check, made before
// $SETPRI's: a priority above the process's authorized one needs ALTPRI.
// VMS 7.1's DCL refused such a request ("need ALTPRI privilege to elevate
// above base priority", round 2 of the probe) where $SETPRI would have
// lowered it quietly.
func (env *Environment) MayRaisePriority(priority uint32) bool {
	if priority <= env.Process.AuthorizedPriority {
		return true
	}

	return env.privilegeGranted(ruleSetpriAbove, dclMode)
}

// SetProcessPriority is SET PROCESS/PRIORITY of target: $SETPRI's work
// from supervisor mode, its check first (0 or SS$_NOPRIV), a priority above
// the caller's authorized one lowered to it without ALTPRI.
func (env *Environment) SetProcessPriority(target *Environment, priority uint32) uint32 {
	if st := env.MayAffect(target); st != 0 {
		return st
	}

	priority &= maxPriority
	if priority > env.Process.AuthorizedPriority && !env.privilegeGranted(ruleSetpriAbove, dclMode) {
		priority = env.Process.AuthorizedPriority
	}

	p := target.Process
	p.BasePriority, p.Priority = priority, priority

	if err := env.sched.SetBasePriority(handle(target), int(priority)); err == nil {
		env.requestReschedule()
	}

	return ssNormal
}

// SuspendProcess is SET PROCESS/SUSPEND of target: $SUSPND's work from
// supervisor mode, after its check (0 or SS$_NOPRIV). DCL's own process
// isn't suspended: nothing would be left to resume it (unconfirmed: VMS
// suspends it, and the terminal waits).
func (env *Environment) SuspendProcess(target *Environment) uint32 {
	if st := env.MayAffect(target); st != 0 {
		return st
	}

	if target != env {
		target.suspend()
	}

	return ssNormal
}

// ResumeProcess is SET PROCESS/RESUME of target: $RESUME's work from
// supervisor mode, after its check (0 or SS$_NOPRIV).
func (env *Environment) ResumeProcess(target *Environment) uint32 {
	if st := env.MayAffect(target); st != 0 {
		return st
	}

	target.resume()

	return ssNormal
}
