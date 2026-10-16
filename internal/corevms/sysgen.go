package corevms

// The system parameters (docs/PHASE-62 - VMS paging.md, Decision 11).
//
// On VMS, the system manager tunes the system with SYSGEN, the System
// Generation utility: a few hundred named parameters (MINWSCNT,
// PQL_DWSQUOTA, ...) that the executive reads. govax has no SYSGEN
// command. Its parameters come from the configuration, as the
// vax.sysgen.* keys, each named for the VMS parameter in lowercase, so
// that someone who knows SYSGEN recognizes them: vax.sysgen.pql_dwsquota
// is PQL_DWSQUOTA. A few settings VMS has no parameter for (whether
// paging is on at all) are govax's own, under vax.sysgen.paging.
//
// The console reads the keys (internal/machine's sysgen.go) into a Sysgen
// and gives it to each new System (SetSysgen). Values that start a
// process off, its working set sizes and its job's paging file quota, are
// copied into the process when it's created, so a later change for one
// process (SET WORKING_SET, a quota list) changes only that process.

// DefaultMinWSCount is MINWSCNT's default: the smallest working set a
// process may have, and the least $ADJWSL can shrink one to.
const DefaultMinWSCount = 20

// DefaultGrowLimit is GROWLIM's default (the phase doc's Decision 11):
// past its quota, a working set takes a new page into an available entry
// only while the free page list holds more than this many pages.
const DefaultGrowLimit = 63

// DefaultTBSkipWSL is TBSKIPWSL's default, the book's value: how many
// pages still valid in the translation buffer a working set's
// replacement scan passes over before it takes the next page whatever
// its state.
const DefaultTBSkipWSL = 8

// DefaultMPWHiLimit and DefaultMPWLoLimit are MPW_HILIMIT's and
// MPW_LOLIMIT's defaults (the phase doc's Decision 11): the modified
// page writer starts when the modified page list holds more than the
// first, and writes until it holds no more than the second (mpw.go).
const (
	DefaultMPWHiLimit = 500
	DefaultMPWLoLimit = 120
)

// DefaultFreeLimit and DefaultFreeGoal are FREELIM's and FREEGOAL's
// defaults (the phase doc's Decision 11): when an allocation leaves the
// free page list with fewer than the first, the reclaimer works to bring
// it back up to the second (reclaim.go).
const (
	DefaultFreeLimit = 32
	DefaultFreeGoal  = 200
)

// Sysgen holds the system parameters a System runs with.
type Sysgen struct {
	// Paging is whether VMS paging is on (vax.sysgen.paging.enable, or
	// VMINIT's /[NO]PAGING): working sets, the free and modified page
	// lists, and a page file. It's chosen when VMINIT builds the page
	// tables, and lasts until the next VMINIT.
	Paging bool

	// PQL holds the PQL_Dxxx parameters the configuration set, by quota
	// name ("WSDEFAULT", "WSQUOTA", "WSEXTENT", "PGFLQUOTA"): the values
	// $CREPRC gives a quota its list doesn't name, in place of
	// quotaRules' own defaults. A quota that isn't here keeps its
	// default. They're also laid over a logging-in process's account
	// values (LoginQuotas).
	PQL map[string]uint32

	// MinWSCount is MINWSCNT.
	MinWSCount uint32

	// GrowLimit is GROWLIM, and TBSkipWSL is TBSKIPWSL (0 turns the TB
	// check off): two of the rules working set replacement follows
	// (workingset.go).
	GrowLimit uint32
	TBSkipWSL uint32

	// MPWHiLimit is MPW_HILIMIT and MPWLoLimit MPW_LOLIMIT: the modified
	// page writer's thresholds (mpw.go).
	MPWHiLimit uint32
	MPWLoLimit uint32

	// FreeLimit is FREELIM and FreeGoal FREEGOAL: the reclaimer's
	// thresholds (reclaim.go).
	FreeLimit uint32
	FreeGoal  uint32

	// PageFileName is vax.sysgen.paging.file, the page file's name (a
	// volume file's specification, or a host path, in quotes when it
	// could be taken for either; "" for no page file), and PageFileSize
	// vax.sysgen.paging.size, its size in blocks when it's created
	// (pagefile.go).
	PageFileName string
	PageFileSize uint32

	// MPWWriteCluster is MPW_WRTCLUSTER: the most pages the modified page
	// writer writes to the page file at once.
	MPWWriteCluster uint32
}

// DefaultSysgen returns the parameters with no key set.
func DefaultSysgen() Sysgen {
	return Sysgen{
		MinWSCount: DefaultMinWSCount, GrowLimit: DefaultGrowLimit, TBSkipWSL: DefaultTBSkipWSL,
		MPWHiLimit: DefaultMPWHiLimit, MPWLoLimit: DefaultMPWLoLimit,
		FreeLimit: DefaultFreeLimit, FreeGoal: DefaultFreeGoal,
		PageFileSize: DefaultPageFileSize, MPWWriteCluster: DefaultMPWWriteCluster,
	}
}

// SetSysgen gives the System its parameters. The console calls it for
// each new System, before process 1 is made.
func (sys *System) SetSysgen(sg Sysgen) {
	sys.Sysgen = sg
	sys.installReclaimer()
}

// pqlDefault returns the default $CREPRC gives rule's quota: the
// configured PQL_Dxxx parameter if there is one, else the rule's own. A
// nil Sysgen has none configured.
func (sg *Sysgen) pqlDefault(rule *quotaRule) uint32 {
	if sg != nil {
		if v, ok := sg.PQL[rule.name]; ok {
			return v
		}
	}

	return rule.def
}

// Account holds the quotas an authorization record gives a user's
// process when it logs in: on VMS, the user's entry in SYSUAF.DAT, the
// authorization file. govax has no authorization file yet, so its one
// account is SYSTEM, the console's (SystemAccount). A later SYSUAF would
// supply an Account for each user and each detached job it starts, and
// LoginQuotas would be given that one instead.
type Account struct {
	WSDefault, WSQuota, WSExtent uint32
	PGFLQUOTA                    uint32
}

// SystemAccount is the SYSTEM account's quotas: process 1's, as $GETJPI
// reported them for SYSTEM on a VMS 7.1 run (testdata/mp/probe1/vax), and
// the nominal paging file quota of a job (job.go).
func SystemAccount() Account {
	return Account{
		WSDefault: nominalWSDefault,
		WSQuota:   nominalWSQuota,
		WSExtent:  nominalWSExtent,
		PGFLQUOTA: nominalPGFLQUOTA,
	}
}

// LoginQuotas gives p, a process logging in as the user acct describes
// and the master process of its own job, its working set sizes and its
// job's paging file quota. A PQL_Dxxx parameter the configuration set
// replaces the account's value: with no authorization file, the
// configuration is how the author sets every process's starting values
// (docs/PHASE-62, Decision 11). The working-set default can't exceed the
// quota, the limit starts at the default, the quota and extent are the
// ones the process is authorized for (WSAuth, WSAuthExt), and the
// smallest working set is MINWSCNT.
func (sg *Sysgen) LoginQuotas(p *Process, acct Account) {
	pick := func(name string, v uint32) uint32 {
		if c, ok := sg.PQL[name]; ok {
			return c
		}

		return v
	}

	p.WSQuota = pick("WSQUOTA", acct.WSQuota)
	p.WSDefault = min(pick("WSDEFAULT", acct.WSDefault), p.WSQuota)
	p.WSExtent = pick("WSEXTENT", acct.WSExtent)
	p.WSLimit = p.WSDefault
	p.WSAuth, p.WSAuthExt = p.WSQuota, p.WSExtent
	p.MinWSCount = sg.MinWSCount

	if p.Job != nil {
		p.Job.Pooled.PGFLQUOTA = pick("PGFLQUOTA", acct.PGFLQUOTA)
	}
}
