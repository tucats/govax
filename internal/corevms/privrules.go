package corevms

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// The privilege gate and its rules (docs/planning/PHASE-94 -
// privileges.md, Decisions 1 and 2).
//
// # One gate
//
// Every operation VMS guards with a privilege is described here by a
// PrivilegeRule: what the operation is, which privileges it needs (all
// of them, or any one), which caller modes need none, and the status a
// refusal returns. A service that guards an operation names its rule and
// asks the gate:
//
//	if st := env.requirePrivilege(ruleDelmbx); st != 0 {
//		return st, nil
//	}
//
// requirePrivilege answers 0 or the rule's status; holdsPrivilege answers
// true or false, for the services whose answer to a missing privilege
// isn't a failure (a priority quietly lowered, a privilege mask cut to
// the caller's). Both read the process's current privileges (CURPRIV),
// at the moment of the call, so a privilege turned off counts at once,
// and both trace the check under SET DEBUG PRIVILEGES.
//
// Nothing else in govax reads CURPRIV to decide whether an operation is
// allowed: TestNoUncheckedPrivileges fails on a file that does, but for
// a named list of the places that manage the masks (and $SETPRV, image
// rundown, $CREPRC's copying, SHOW PROCESS/PRIVILEGES, a protection
// check's credentials) rather than check them.
//
// # The table
//
// The rules are package variables, each made by newRule, which also adds
// it to privilegeRules. serviceRules says, for every system service
// govax registers, which rules it can ask the gate about, or that it
// asks about none; TestEveryServiceHasPrivilegeRules fails on a service
// that isn't there, so a new service must decide, and
// TestEveryPrivilegeRuleUsed fails on a rule no check names. The phase
// doc's table of rules (A.5) is PrivilegeRuleTable's output.

// exemption says which caller modes a rule lets through without the
// privilege: VMS's rule for a few services, that a caller already in an
// inner mode needs no privilege for what it could do anyway.
type exemption int

const (
	// The zero value: every mode needs the privilege.
	_ exemption = iota

	// exemptKernel: a kernel-mode caller needs none.
	exemptKernel

	// exemptInner: a kernel- or executive-mode caller needs none.
	exemptInner
)

// exempts reports whether a caller in mode needs no privilege.
func (e exemption) exempts(mode registers.AccessMode) bool {
	switch e {
	case exemptKernel:
		return mode == registers.Kernel
	case exemptInner:
		return mode <= registers.Executive
	}

	return false
}

// String names the exemption for PrivilegeRuleTable.
func (e exemption) String() string {
	switch e {
	case exemptKernel:
		return "kernel"
	case exemptInner:
		return "kernel, executive"
	}

	return "none"
}

// PrivilegeRule is one guarded operation.
type PrivilegeRule struct {
	// Name says what the operation is: a service and what it does
	// ("$CREMBX permanent"), or an operation several services share
	// ("logical name: write the system table").
	Name string

	// Privileges are the privileges' names, as $PRVDEF has them
	// (PRV$V_name). Mask is their bits.
	Privileges []string
	Mask       uint64

	// Any is true when one of the privileges is enough; otherwise
	// every one is needed.
	Any bool

	// Exempt is the caller modes that need none.
	Exempt exemption

	// Status is what requirePrivilege returns for a refusal:
	// SS$_NOPRIV unless VMS says otherwise.
	Status uint32

	// Source is where the rule comes from: the manual, the probe's
	// cases (testdata/priv94), or both.
	Source string
}

// privilegeRules is every rule, in the order they're made.
var privilegeRules []*PrivilegeRule

// ruleOption sets one of a rule's optional fields.
type ruleOption func(*PrivilegeRule)

// anyOf makes a rule satisfied by any one of its privileges.
func anyOf(r *PrivilegeRule) { r.Any = true }

// exempt sets the caller modes that need no privilege.
func exempt(e exemption) ruleOption { return func(r *PrivilegeRule) { r.Exempt = e } }

// refusedWith sets the status of a refusal, by its SS$_ name.
func refusedWith(name string) ruleOption {
	return func(r *PrivilegeRule) { r.Status = vmsdef.Symbols[name] }
}

// newRule makes the rule called name, needing privileges (a list of
// $PRVDEF names, separated by blanks), and adds it to privilegeRules.
// A name $PRVDEF doesn't have panics, so a misspelling fails at once.
func newRule(name, privileges, source string, options ...ruleOption) *PrivilegeRule {
	r := &PrivilegeRule{Name: name, Privileges: strings.Fields(privileges), Status: ssNoPriv, Source: source}

	for _, p := range r.Privileges {
		r.Mask |= privilegeBit(p)
	}

	for _, o := range options {
		o(r)
	}

	privilegeRules = append(privilegeRules, r)

	return r
}

// The rules. Sources: "SSRM" is the VMS 5.0 System Services Reference
// Manual's entry for the service; "probe n" is testdata/priv94's case n
// (round 1's priv94.mar, or "round 2" priv94b.mar), whose answers the
// phase doc's A.4 gives. The rules follow the probe since subtasks 4 and
// 5 (TestPriv94Oracle holds them to it). Where an object's protection
// can grant an operation too (a logical name table, a mailbox), the
// protection is asked first, and the rule is only the privilege that
// overrides it: SYSPRV's part is the protection's (lnmTableAccess).
var (
	// Logical names ($CRELNM, $DELLNM, $CRELNT, $CRELOG, $DELLOG,
	// LIB$SET_LOGICAL, LIB$DELETE_LOGICAL).
	ruleLnmSystemTable = newRule("logical name: write the system table past its protection", "SYSNAM",
		"SSRM $CRELNM; probe 4 to 7")
	ruleLnmGroupTable = newRule("logical name: write the group table past its protection", "GRPNAM",
		"SSRM $CRELNM; probe 8 to 11")
	ruleLnmInnerMode = newRule("logical name: a mode inner to the caller's", "SYSNAM",
		"SSRM $CRELNM (the mode is maximized); probe 22 to 24")

	// Mailboxes.
	ruleCrembxTemporary = newRule("$CREMBX temporary", "TMPMBX", "SSRM $CREMBX; probe 80, 81")
	ruleCrembxPermanent = newRule("$CREMBX permanent", "PRMMBX", "SSRM $CREMBX; probe 83, 84")
	ruleDelmbx         = newRule("$DELMBX", "PRMMBX", "SSRM $DELMBX; probe 88 to 91", exempt(exemptKernel))
	ruleMailboxSetProt = newRule("mailbox IO$_SETPROTECTION, not the owner", "BYPASS SYSPRV",
		"probe 92 to 97", anyOf)

	// Common event flag clusters.
	ruleAscefcPermanent = newRule("$ASCEFC permanent", "PRMCEB", "SSRM $ASCEFC; probe 99, 100")
	ruleDlcefcOthers    = newRule("$DLCEFC another creator's cluster", "PRMCEB", "SSRM $DLCEFC; probe 103 to 105")

	// Global sections.
	ruleCrmpscSystem    = newRule("$CRMPSC system section", "SYSGBL", "SSRM $CRMPSC; probe 106 to 113")
	ruleCrmpscPermanent = newRule("$CRMPSC permanent section", "PRMGBL", "SSRM $CRMPSC; probe 106 to 113")
	ruleDgblscSystem    = newRule("$DGBLSC system section", "SYSGBL", "SSRM $DGBLSC; probe 117 to 122")
	ruleDgblscPermanent = newRule("$DGBLSC permanent section", "PRMGBL", "SSRM $DGBLSC; probe 117 to 122")

	// Locks.
	ruleEnqSystem = newRule("$ENQ system-wide resource", "SYSLCK", "SSRM $ENQ; probe 124 to 127",
		exempt(exemptInner), refusedWith("SS$_NOSYSLCK"))
	ruleGetlkiSystem = newRule("$GETLKI system-wide lock", "SYSLCK", "SSRM $GETLKI",
		exempt(exemptInner), refusedWith("SS$_NOSYSLCK"))
	ruleGetlkiOtherGroup = newRule("$GETLKI another group's lock", "WORLD", "SSRM $GETLKI",
		refusedWith("SS$_NOWORLD"))

	// Other processes ($WAKE, $SCHDWK, $CANWAK, $SUSPND, $RESUME,
	// $SETPRI, $SETPRN... any service that reaches another process).
	ruleProcessInGroup = newRule("another process in the group", "GROUP WORLD",
		"SSRM $DELPRC and the rest; probe 128 to 202", anyOf)
	ruleProcessOtherGroup = newRule("a process in another group", "WORLD",
		"SSRM $DELPRC and the rest; probe 128 to 202")
	ruleSetpriAbove = newRule("$SETPRI above the authorized priority", "ALTPRI",
		"SSRM $SETPRI; probe 149, 150, 179, 186, 187")

	// Creating processes.
	ruleCreprcOtherUIC = newRule("$CREPRC detached with another UIC", "DETACH CMKRNL",
		"SSRM $CREPRC; probe 212 to 219, 226, 227", anyOf)
	ruleCreprcBatchNetwork = newRule("$CREPRC batch or network process", "DETACH CMKRNL",
		"SSRM $CREPRC", anyOf)
	ruleCreprcNetwork     = newRule("$CREPRC network process", "NETMBX", "SSRM $CREPRC")
	ruleCreprcPSWAPM      = newRule("$CREPRC PRC$M_PSWAPM", "PSWAPM", "SSRM $CREPRC; probe 220, 221")
	ruleCreprcNOACNT      = newRule("$CREPRC PRC$M_NOACNT", "ACNT", "SSRM $CREPRC; probe 222, 223")
	ruleCreprcIndependent = newRule("$CREPRC detached quotas of its own", "DETACH", "SSRM $CREPRC")
	ruleCreprcPrivileges  = newRule("$CREPRC privileges beyond the creator's", "SETPRV",
		"SSRM $CREPRC; probe 207 to 211")
	ruleCreprcHigherPriority = newRule("$CREPRC base priority above the creator's", "ALTPRI",
		"SSRM $CREPRC; probe 224, 225")

	// Privileges.
	ruleSetprvUnauthorized = newRule("$SETPRV a privilege not authorized", "SETPRV",
		"SSRM $SETPRV; probe 229 to 241", exempt(exemptInner), refusedWith("SS$_NOTALLPRIV"))

	// Change mode.
	ruleCmkrnl = newRule("$CMKRNL", "CMKRNL", "SSRM $CMKRNL; probe 284 to 289", exempt(exemptInner))
	ruleCmexec = newRule("$CMEXEC", "CMEXEC CMKRNL", "SSRM $CMEXEC; probe 284 to 289", anyOf, exempt(exemptInner))

	// Pages.
	ruleLckpag = newRule("$LCKPAG", "PSWAPM", "SSRM $LCKPAG; probe 262 to 264")
	ruleSetswm = newRule("$SETSWM", "PSWAPM", "SSRM $SETSWM; probe 268 to 270")

	// Operators and terminals.
	ruleSndoprOperator = newRule("$SNDOPR an operator's function", "OPER", "SSRM $SNDOPR; probe 247 to 252")
	ruleBrkthruUser    = newRule("$BRKTHRU to another user", "OPER", "probe 257 to 260",
		refusedWith("SS$_NOOPER"))
	ruleBrkthruAll     = newRule("$BRKTHRU to every terminal or user", "OPER", "SSRM $BRKTHRU; probe 253 to 256, 261",
		refusedWith("SS$_NOOPER"))

	// Disk I/O.
	ruleDiskLogical  = newRule("disk logical block I/O", "LOG_IO PHY_IO", "I/O User's Reference; probe 271 to 273, 277", anyOf)
	ruleDiskPhysical = newRule("disk physical block I/O", "PHY_IO", "I/O User's Reference; probe 274 to 276")
)

// serviceRules is, for every system service govax registers, the rules
// it can ask the gate about; nil for none. $QIO's are its drivers'.
var serviceRules = func() map[string][]*PrivilegeRule {
	lnmWrite := []*PrivilegeRule{ruleLnmSystemTable, ruleLnmGroupTable, ruleLnmInnerMode}
	affects := []*PrivilegeRule{ruleProcessInGroup, ruleProcessOtherGroup}
	qio := []*PrivilegeRule{ruleDiskLogical, ruleDiskPhysical, ruleMailboxSetProt}

	m := map[string][]*PrivilegeRule{
		"SYS$CRELNM": lnmWrite,
		"SYS$DELLNM": lnmWrite,
		"SYS$CRELNT": {ruleLnmInnerMode},
		"SYS$CRELOG": lnmWrite,
		"SYS$DELLOG": lnmWrite,

		"SYS$CREMBX": {ruleCrembxTemporary, ruleCrembxPermanent, ruleLnmSystemTable},
		"SYS$DELMBX": {ruleDelmbx},

		"SYS$ASCEFC": {ruleAscefcPermanent},
		"SYS$DLCEFC": {ruleDlcefcOthers},

		"SYS$CRMPSC": {ruleCrmpscSystem, ruleCrmpscPermanent},
		"SYS$DGBLSC": {ruleDgblscSystem, ruleDgblscPermanent},

		"SYS$ENQ":     {ruleEnqSystem},
		"SYS$ENQW":    {ruleEnqSystem},
		"SYS$GETLKI":  {ruleGetlkiSystem, ruleGetlkiOtherGroup},
		"SYS$GETLKIW": {ruleGetlkiSystem, ruleGetlkiOtherGroup},

		"SYS$SETPRI": append([]*PrivilegeRule{ruleSetpriAbove}, affects...),
		"SYS$CREPRC": {
			ruleCreprcOtherUIC, ruleCreprcBatchNetwork, ruleCreprcNetwork, ruleCreprcPSWAPM,
			ruleCreprcNOACNT, ruleCreprcIndependent, ruleCreprcPrivileges, ruleCreprcHigherPriority,
		},

		"SYS$SETPRV": {ruleSetprvUnauthorized},

		"SYS$CMKRNL": {ruleCmkrnl},
		"SYS$CMEXEC": {ruleCmexec},
		"SYS$LCKPAG": {ruleLckpag},
		"SYS$SETSWM": {ruleSetswm},

		"SYS$SNDOPR":   {ruleSndoprOperator},
		"SYS$BRKTHRU":  {ruleBrkthruUser, ruleBrkthruAll},
		"SYS$BRKTHRUW": {ruleBrkthruUser, ruleBrkthruAll},

		"SYS$QIO":  qio,
		"SYS$QIOW": qio,
	}

	for _, name := range affectingServices {
		m[name] = affects
	}

	for _, name := range unprivilegedServices {
		m[name] = nil
	}

	return m
}()

// affectingServices are the services whose only privileges are those to
// reach another process (mayAffect).
var affectingServices = []string{
	"SYS$WAKE", "SYS$SCHDWK", "SYS$CANWAK", "SYS$SUSPND", "SYS$RESUME", "SYS$FORCEX",
	"SYS$DELPRC", "SYS$GETJPI", "SYS$GETJPIW",
}

// unprivilegedServices are the services that ask the gate about no rule.
// For most the manual names no privilege. For the others the
// privileges count through the protection rule, whose credentials carry
// the ones that override protection (BYPASS, SYSPRV, GRPPRV, READALL):
// RMS's services, $ASSIGN of a mailbox, $MGBLSC, $CHKPRO, and the
// security services. $ALLOC has no spooled devices to need ALLSPOOL.
var unprivilegedServices = []string{
	"SYS$ADJSTK", "SYS$ADJWSL", "SYS$ALLOC", "SYS$ASCTIM", "SYS$ASCTOID", "SYS$ASSIGN",
	"SYS$BINTIM", "SYS$CANCEL", "SYS$CANEXH", "SYS$CANTIM", "SYS$CHKPRO", "SYS$CLI",
	"SYS$CLOSE", "SYS$CLRAST", "SYS$CLREF", "SYS$CNTREG", "SYS$CONNECT", "SYS$CREATE",
	"SYS$CRETVA", "SYS$DACEFC", "SYS$DALLOC", "SYS$DASSGN", "SYS$DCLAST", "SYS$DCLCMH",
	"SYS$DCLEXH", "SYS$DELETE", "SYS$DELTVA", "SYS$DEQ", "SYS$DISCONNECT", "SYS$DISPLAY",
	"SYS$ERASE", "SYS$EXIT", "SYS$EXPREG", "SYS$EXTEND", "SYS$FAO", "SYS$FAOL",
	"SYS$FILESCAN", "SYS$FIND", "SYS$FINISH_RDB", "SYS$FLUSH", "SYS$FREE", "SYS$GET",
	"SYS$GETDVI", "SYS$GETDVIW", "SYS$GETMSG", "SYS$GETSYI", "SYS$GETSYIW", "SYS$GETTIM",
	"SYS$GET_SECURITY", "SYS$HIBER", "SYS$IDTOASC", "SYS$LKWSET", "SYS$MGBLSC", "SYS$NUMTIM",
	"SYS$OPEN", "SYS$PARSE", "SYS$PURGWS", "SYS$PUT", "SYS$PUTMSG", "SYS$READ",
	"SYS$READEF", "SYS$RELEASE", "SYS$RENAME", "SYS$REWIND", "SYS$SEARCH", "SYS$SETAST",
	"SYS$SETDFPROT", "SYS$SETEF", "SYS$SETEXV", "SYS$SETIMR", "SYS$SETPRN", "SYS$SETPRT",
	"SYS$SETRWM", "SYS$SETSFM", "SYS$SET_SECURITY", "SYS$SRCHANDLER", "SYS$SYNCH",
	"SYS$TRNLNM", "SYS$TRNLOG", "SYS$TRUNCATE", "SYS$ULKPAG", "SYS$ULWSET", "SYS$UNWIND",
	"SYS$UPDATE", "SYS$UPDSEC", "SYS$UPDSECW", "SYS$WAIT", "SYS$WAITFR", "SYS$WFLAND",
	"SYS$WFLOR", "SYS$WRITE",
}

// requirePrivilege is the gate (Decision 1): 0 if the process may do
// what rule r guards, from the caller's current access mode, or r's
// status if not.
func (env *Environment) requirePrivilege(r *PrivilegeRule) uint32 {
	return env.requirePrivilegeAt(r, env.callerMode())
}

// requirePrivilegeAt is requirePrivilege for a caller in mode, for the
// services that take the mode to judge by from an argument.
func (env *Environment) requirePrivilegeAt(r *PrivilegeRule, mode registers.AccessMode) uint32 {
	if env.privilegeGranted(r, mode) {
		return 0
	}

	return r.Status
}

// holdsPrivilege is the gate for the operations a missing privilege
// changes rather than refuses: whether the process may do what r
// guards, from the caller's current access mode.
func (env *Environment) holdsPrivilege(r *PrivilegeRule) bool {
	return env.privilegeGranted(r, env.callerMode())
}

// callerMode is the access mode a service's caller is in: the CPU's
// current mode, or user mode for a process built in Go with no CPU.
func (env *Environment) callerMode() registers.AccessMode {
	if env.cpu == nil {
		return registers.User
	}

	return env.cpu.PSL().CurMod()
}

// privilegeGranted decides rule r for a caller in mode, by the process's
// current privileges, and traces the decision under SET DEBUG
// PRIVILEGES.
func (env *Environment) privilegeGranted(r *PrivilegeRule, mode registers.AccessMode) bool {
	held := env.Process.CurrentPrivileges & r.Mask

	var granted bool

	var why string

	switch {
	case r.Exempt.exempts(mode):
		granted, why = true, "exempt in its mode"
	case r.Any && held != 0, !r.Any && held == r.Mask:
		granted, why = true, "holds "+privilegeNames(held)
	default:
		why = "refused, " + statusIdent(r.Status)
	}

	if env.System != nil && env.cpu != nil && env.cpu.DebugEnabled(registers.DebugPrivileges) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG(PRIVILEGES): %08X %s mode: %s (needs %s): %s\n",
			env.Process.PID, modeNames[mode&3], r.Name, r.needs(), why)
	}

	return granted
}

// needs is r's privileges as the trace and the table show them:
// "SYSNAM or SYSPRV", "SYSGBL and PRMGBL".
func (r *PrivilegeRule) needs() string {
	join := " and "
	if r.Any {
		join = " or "
	}

	return strings.Join(r.Privileges, join)
}

// privilegeNames is the names of the privileges in mask, joined by
// commas, in $PRVDEF's order.
func privilegeNames(mask uint64) string {
	var names []string

	for bit := range 64 {
		if mask&(1<<bit) == 0 {
			continue
		}

		if name, ok := privilegeName(bit); ok {
			names = append(names, name)
		} else {
			names = append(names, fmt.Sprintf("bit %d", bit))
		}
	}

	return strings.Join(names, ",")
}

// privilegeName is the name of privilege bit, as SHOW PROCESS/PRIVILEGES
// and AUTHORIZE name it (the part after PRV$V_). Three bits have a
// second name in $PRVDEF, IMPERSONATE, NOACNT, and SETPRI, which aren't
// used.
func privilegeName(bit int) (string, bool) {
	name, ok := privilegeNamesByBit[bit]

	return name, ok
}

// privilegeNamesByBit is privilegeName's table, from $PRVDEF.
var privilegeNamesByBit = func() map[int]string {
	aliases := map[string]bool{"IMPERSONATE": true, "NOACNT": true, "SETPRI": true}
	names := map[int]string{}

	for symbol, v := range vmsdef.Symbols {
		if name, ok := strings.CutPrefix(symbol, "PRV$V_"); ok && !aliases[name] {
			names[int(v)] = name
		}
	}

	return names
}()

// statusIdent is a status's SS$_ name, or its value in hex.
func statusIdent(status uint32) string {
	if msg, ok := vmsdef.LookupMessage(status); ok {
		return "SS$_" + msg.Ident
	}

	return fmt.Sprintf("%08X", status)
}

// modeNames are the access modes' names, by number.
var modeNames = [4]string{"KERNEL", "EXECUTIVE", "SUPERVISOR", "USER"}

// PrivilegeRuleTable is the rules as a Markdown table, a row per rule:
// the phase doc's A.5 (go test -run TestPrivilegeRuleTable -v prints it).
func PrivilegeRuleTable() string {
	var b strings.Builder

	b.WriteString("| Operation | Needs | Exempt modes | Refusal | Source |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")

	for _, r := range privilegeRules {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.Name, r.needs(), r.Exempt, statusIdent(r.Status), r.Source)
	}

	return b.String()
}
