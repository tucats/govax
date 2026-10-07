package corevms

import "github.com/tucats/govax/internal/vmsdef"

// Process quotas, as $CREPRC sets them (docs/PHASE-45.md, subtask 4).
//
// A quota limits how much of some system resource a process may use: how
// many ASTs it may have outstanding (ASTLM), how many files open (FILLM),
// how many subprocesses (PRCLM), ... $CREPRC's quota argument is a list
// of the ones the creator wants to set, each named by a PQL$_ code. Each
// quota is one of three kinds (VMS 5.0 System Services Reference Manual,
// $CREPRC):
//
//   - Nondeductible: each process has its own, independent of its
//     creator's (ASTLM, BIOLM, DIOLM, and the working-set sizes).
//   - Deductible: a subprocess's comes out of its creator's (CPULM, the
//     CPU time limit; the manual names JTQUOTA too, see below).
//   - Pooled: one value for the whole job, which every process in it
//     draws on together (BYTLM, FILLM, PGFLQUOTA, PRCLM, TQELM, ENQLM).
//     It lives in the job (job.go), so only a detached process, which
//     starts a job, sets it; a subprocess's list values are ignored.
//
// The values a new process gets are worked out in three steps:
//
//  1. Every quota starts at its default, a SYSGEN parameter (PQL_Dxxxxx).
//  2. Each entry of the list replaces its quota's value; with several
//     entries for one quota, the last wins.
//  3. A value below the quota's minimum (PQL_Mxxxxx) is raised to it.
//     Then, unless a detached process is being created by a creator
//     with the DETACH privilege, each value is lowered to the creator's
//     own, and a deductible quota's value is taken out of the creator's.
//
// govax has no SYSGEN: the defaults and minimums below are nominal
// values in the range VMS systems use, unconfirmed (a VMS 7.3 system's
// SYSGEN SHOW/PQL would give real ones). Like the quotas themselves
// (process.go, job.go), they're recorded and reported, not enforced.
//
// The manual's list of statuses also says that a subprocess asking for
// more than its creator has is SS$_EXQUOTA, where step 3 says the value
// is lowered instead; govax follows step 3. JTQUOTA, the job table's
// size, is "deductible" in the manual, but it's ignored for a subprocess
// and a detached process starts a new job; govax treats it as the job's,
// like a pooled quota, and deducts nothing.

// pqlListEnd is PQL$_LISTEND, the code that ends a quota list.
var pqlListEnd = vmsdef.Symbols["PQL$_LISTEND"]

// quotaKind is how a quota is shared (see above).
type quotaKind int

const (
	quotaNondeductible quotaKind = iota
	quotaDeductible
	quotaPooled
)

// quotaRule is one PQL$_ quota: its default and minimum, its kind, and
// where a process keeps its value (in the Process, or in its Job for a
// pooled quota).
type quotaRule struct {
	name     string // the code's name, after PQL$_
	def, min uint32
	kind     quotaKind
	field    func(p *Process) *uint32
}

// quotaRules are the quotas $PQLDEF defines, with the defaults and minimums
// (SYSGEN's PQL_Dxxx and PQL_Mxxx) that the VMS 7.1 simh system had in use
// (SYSGEN SHOW/PQL, testdata/mp/probe2/vax): its working set and
// paging file values are the larger ones its installation set, and ENQLM's
// are not the stock 30 and 4.
var quotaRules = []quotaRule{
	{"ASTLM", 24, 4, quotaNondeductible, func(p *Process) *uint32 { return &p.ASTLimit }},
	{"BIOLM", 18, 4, quotaNondeductible, func(p *Process) *uint32 { return &p.BufferedIOLimit }},
	{"BYTLM", 8192, 1024, quotaPooled, func(p *Process) *uint32 { return &p.Job.Pooled.BYTLM }},
	{"CPULM", 0, 0, quotaDeductible, func(p *Process) *uint32 { return &p.CPULimit }},
	{"DIOLM", 18, 4, quotaNondeductible, func(p *Process) *uint32 { return &p.DirectIOLimit }},
	{"FILLM", 16, 2, quotaPooled, func(p *Process) *uint32 { return &p.Job.Pooled.FILLM }},
	{"PGFLQUOTA", 16400, 16400, quotaPooled, func(p *Process) *uint32 { return &p.Job.Pooled.PGFLQUOTA }},
	{"PRCLM", 8, 0, quotaPooled, func(p *Process) *uint32 { return &p.Job.SubprocessLimit }},
	{"TQELM", 8, 0, quotaPooled, func(p *Process) *uint32 { return &p.Job.Pooled.TQELM }},
	{"WSQUOTA", 588, 1024, quotaNondeductible, func(p *Process) *uint32 { return &p.WSQuota }},
	{"WSDEFAULT", 294, 512, quotaNondeductible, func(p *Process) *uint32 { return &p.WSDefault }},
	{"ENQLM", 128, 30, quotaPooled, func(p *Process) *uint32 { return &p.Job.Pooled.ENQLM }},
	{"WSEXTENT", 16400, 16400, quotaNondeductible, func(p *Process) *uint32 { return &p.WSExtent }},
	{"JTQUOTA", 1024, 0, quotaPooled, func(p *Process) *uint32 { return &p.Job.Pooled.JTQUOTA }},
}

// quotasByCode finds a quota's rule by its PQL$_ code.
var quotasByCode = func() map[uint32]*quotaRule {
	m := map[uint32]*quotaRule{}

	for i := range quotaRules {
		code, ok := vmsdef.Symbols["PQL$_"+quotaRules[i].name]
		if !ok {
			panic("corevms: no $PQLDEF quota " + quotaRules[i].name)
		}

		m[code] = &quotaRules[i]
	}

	return m
}()

// pqlCPULM is the CPU time limit's code, which has rules of its own
// (cpuLimit).
var pqlCPULM = vmsdef.Symbols["PQL$_CPULM"]

// resolveQuotas works out a new process's quotas from creator's and the
// quota list items, by the three steps above: a value for each PQL$_
// code, and how much CPU time to take out of the creator's limit.
// detached says whether the new process is detached, and independent
// whether its creator has DETACH too, so its values aren't held to the
// creator's. SS$_EXQUOTA if the CPU time taken would leave the creator
// none.
func resolveQuotas(creator *Process, items []QuotaItem, detached, independent bool) (map[uint32]uint32, uint32, uint32) {
	values := map[uint32]uint32{}
	given := map[uint32]bool{}

	for code, rule := range quotasByCode {
		values[code] = rule.def
	}

	for _, item := range items {
		values[item.Code] = item.Value
		given[item.Code] = true
	}

	var deducted uint32

	for code, rule := range quotasByCode {
		if code == pqlCPULM {
			limit, take, st := cpuLimit(creator, values[code], given[code], detached, independent)
			if st != 0 {
				return nil, 0, st
			}

			values[code], deducted = limit, take

			continue
		}

		v := max(values[code], rule.min)
		if !independent {
			v = min(v, *rule.field(creator))
		}

		values[code] = v
	}

	return values, deducted, 0
}

// cpuLimit is the CPU time limit's step 3 (in 10-millisecond units; 0 is
// no limit), which the manual spells out on its own: a detached process
// that doesn't ask for a limit has none; a subprocess that doesn't gets
// half its creator's. A creator that has a limit can't give "none": a 0
// asked for becomes half the creator's too. A subprocess's limit comes
// out of its creator's (it's returned when the subprocess is deleted),
// and must leave the creator some (SS$_EXQUOTA). It returns the limit
// and how much to take from the creator.
func cpuLimit(creator *Process, v uint32, given, detached, independent bool) (uint32, uint32, uint32) {
	have := creator.CPULimit

	if !given {
		if detached {
			v = 0
		} else {
			v = have / 2
		}
	}

	if independent || have == 0 {
		return v, 0, 0
	}

	// The creator has a limit, and the new process's comes out of it.
	if v == 0 {
		v = have / 2
	}

	v = min(max(v, 1), have)
	if have-v < 1 {
		return 0, 0, ssExQuota
	}

	return v, v, 0
}

// applyQuotas gives p, a new process, the quotas resolveQuotas worked
// out. The pooled ones are its job's: set only for a detached process,
// which has a new job. The working-set default can't be above the
// working-set quota (the manual's PQL$_WSDEFAULT), and the working set
// starts at its default.
func applyQuotas(p *Process, values map[uint32]uint32, detached bool) {
	for code, rule := range quotasByCode {
		if rule.kind == quotaPooled && !detached {
			continue
		}

		*rule.field(p) = values[code]
	}

	p.WSDefault = min(p.WSDefault, p.WSQuota)
	p.WSLimit = p.WSDefault
}
