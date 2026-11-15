package corevms

// The privilege checks of DCL's commands (docs/planning/PHASE-94 -
// privileges.md, Decisions 5 and 10): the gate asked from supervisor mode,
// DCL's, for govax's own volume commands, and for STOP of another process.
// The logical name commands' are dcllogicals.go's.

// commandRules are the rules of operations that only govax's DCL
// commands do, with no service of their own: each command asks the gate
// through CommandPrivilege with its name here.
var commandRules = map[string]*PrivilegeRule{
	"MOUNT":                ruleMountVolume,
	"DISMOUNT":             ruleDismountVolume,
	"INITIALIZE/CONTAINER": ruleInitContainer,
}

// CommandPrivilege is the gate for one of govax's own DCL commands (a
// commandRules name), asked from supervisor mode, DCL's: 0, or the rule's
// status. A name with no rule panics, so a misspelling fails at once.
func (env *Environment) CommandPrivilege(command string) uint32 {
	r, ok := commandRules[command]
	if !ok {
		panic("corevms: no privilege rule for the command " + command)
	}

	return env.requirePrivilegeAt(r, dclMode)
}

// MayAffect is mayAffect for DCL (STOP, and SET PROCESS of another
// process): whether the process may change target, asked from supervisor
// mode. It returns 0, or SS$_NOPRIV.
func (env *Environment) MayAffect(target *Environment) uint32 {
	return env.mayAffectAt(target, dclMode)
}
