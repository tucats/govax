package corevms

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETSYI and $GETSYIW (docs/PHASE-26.md subtask 23): information about
// the system — its VMS version, node name, CPU, boot time, and so on.
//
// It is $GETJPI's pattern for the system instead of a process: an item
// list names what to return, and the request completes with an event
// flag, an IOSB, and an optional AST. A VMS system may belong to a
// VAXcluster, and $GETSYI can ask about any node in it; govax's system is
// a single node outside any cluster, so a request either names this node
// or no node at all.

// Status codes $GETSYI returns.
var (
	ssNoMoreNode  = vmsdef.Symbols["SS$_NOMORENODE"]
	ssNoSuchNode  = vmsdef.Symbols["SS$_NOSUCHNODE"]
	VersionString = "V7.3" // Default value, main injects govax build version
)

// The system's identity. The node name is govax's own. The version is
// that of the VMS sources govax's definitions come from (the $SYIDEF,
// $IODEF, $JPIDEF, ... item codes and values are VMS 7.3's), so a program
// that tests the version for a feature finds the release those
// definitions describe.
const (
	nominalNodeName = "GOVAX"
	vmsSoftwareType = "VMS"

	// localCSID is the node's cluster system ID. A node outside a
	// cluster has none, so it is 0 — which also means "this node" in
	// csidadr.
	localCSID = 0
)

// Wildcard $GETSYI contexts, the longword at csidadr, as for $GETJPI's
// pidadr: -1 starts a scan of the cluster's nodes, which returns this
// node, and syiWildcardDone marks the scan finished (SS$_NOMORENODE).
const (
	syiWildcard     = 0xFFFFFFFF
	syiWildcardDone = 0xFFFFFFFE
)

// maxNodeNameLength is the longest node name $GETSYI's nodename accepts
// (SS$_IVLOGNAM beyond it).
const maxNodeNameLength = 15

// syiItemsByName is the item-code registry, keyed by $SYIDEF name: what
// each supported item returns. Items not here are SS$_BADPARAM.
//
//   - SYI$_VERSION is 8 characters, blank-padded; SYI$_NODE_SWVERS and
//     SYI$_NODE_SWTYPE are the 4-character forms of the version and the
//     software type ("VMS ").
//   - SYI$_SID is the CPU's system identification register, and SYI$_CPU
//     the processor type in its high byte (1 for a VAX-11/780, ...).
//   - SYI$_BOOTTIME is when this Environment was created: the INIT,
//     VMINIT, or ZERO that set the machine up, govax's "boot".
//   - The cluster items describe a node outside any cluster: not a
//     member, cluster system ID 0.
//   - SYI$_MINWSCNT is the SYSGEN parameter that bounds $ADJWSL.
//   - The paging parameters (Phase 63) are the System's Sysgen
//     (sysgen.go), the vax.sysgen.* keys or their defaults, and the PQL_D
//     parameters the defaults $CREPRC gives (sysgenPQL). Three that
//     govax has no use for are items all the same, since VMS 7.1 answers
//     them (testdata/pg63, round 2): WSMAX is the largest extent govax
//     gives a process (16400, which that VAX had too), and BORROWLIM is
//     SYSGEN's default (300). PFCDEFAULT, the read cluster's default
//     size (Phase 65, subtask 6), is the System's; PAGTBLPFC, the page
//     table pages' cluster, is SYSGEN's default (2, as that VAX had),
//     since govax's page tables aren't paged.
//   - The page file's (Phase 64, subtask 7): MPW_WRTCLUSTER and
//     PQL_MPGFLQUOTA (quotaRules' minimum) are parameters;
//     PAGEFILE_PAGE and PAGEFILE_FREE the page file's size and free
//     blocks, in pages (pageFileSizes; 0 with none); SWAPFILE_PAGE and
//     SWAPFILE_FREE are 0, since govax has no swap file (unconfirmed:
//     VMS may count a page file in their place).
//   - The global section limits (Phase 65, subtask 4): GBLSECTIONS,
//     GBLPAGES, and GBLPAGFIL are the parameters (the System's
//     GlobalSections, from the vax.sysgen.* keys); FREE_GBLSECTS and
//     FREE_GBLPAGES what's left of the first two; CONTIG_GBLPAGES, the
//     largest run of free global page table entries on VMS, is
//     FREE_GBLPAGES, since govax's global page table has no fixed size to
//     fragment (unconfirmed).
var syiItemsByName = map[string]func(env *Environment) itemValue{
	"SYI$_VERSION":        func(env *Environment) itemValue { return itemPadded(VersionString, 8) },
	"SYI$_NODE_SWVERS":    func(env *Environment) itemValue { return itemPadded(VersionString, 4) },
	"SYI$_NODE_SWTYPE":    func(env *Environment) itemValue { return itemPadded(vmsSoftwareType, 4) },
	"SYI$_NODENAME":       func(env *Environment) itemValue { return itemString(env.NodeName) },
	"SYI$_SID":            func(env *Environment) itemValue { return itemLong(env.cpu.PR(registers.SID)) },
	"SYI$_CPU":            func(env *Environment) itemValue { return itemLong(env.cpu.PR(registers.SID) >> 24) },
	"SYI$_BOOTTIME":       func(env *Environment) itemValue { return itemQuad(env.BootTime) },
	"SYI$_CLUSTER_MEMBER": func(env *Environment) itemValue { return itemByte(0) },
	"SYI$_NODE_CSID":      func(env *Environment) itemValue { return itemLong(localCSID) },
	"SYI$_MINWSCNT":       func(env *Environment) itemValue { return itemLong(env.Process.MinWSCount) },
	"SYI$_GROWLIM":        func(env *Environment) itemValue { return itemLong(env.Sysgen.GrowLimit) },
	"SYI$_FREELIM":        func(env *Environment) itemValue { return itemLong(env.Sysgen.FreeLimit) },
	"SYI$_FREEGOAL":       func(env *Environment) itemValue { return itemLong(env.Sysgen.FreeGoal) },
	"SYI$_TBSKIPWSL":      func(env *Environment) itemValue { return itemLong(env.Sysgen.TBSkipWSL) },
	"SYI$_MPW_HILIMIT":    func(env *Environment) itemValue { return itemLong(env.Sysgen.MPWHiLimit) },
	"SYI$_MPW_LOLIMIT":    func(env *Environment) itemValue { return itemLong(env.Sysgen.MPWLoLimit) },
	"SYI$_WSMAX":          func(env *Environment) itemValue { return itemLong(nominalWSExtent) },
	"SYI$_BORROWLIM":      func(env *Environment) itemValue { return itemLong(defaultBorrowLimit) },
	"SYI$_PFCDEFAULT":     func(env *Environment) itemValue { return itemLong(env.Sysgen.PFCDefault) },
	"SYI$_PAGTBLPFC":      func(env *Environment) itemValue { return itemLong(defaultPageTablePFC) },
	"SYI$_PQL_DWSDEFAULT": func(env *Environment) itemValue { return itemLong(env.sysgenPQL("WSDEFAULT")) },
	"SYI$_PQL_DWSQUOTA":   func(env *Environment) itemValue { return itemLong(env.sysgenPQL("WSQUOTA")) },
	"SYI$_PQL_DWSEXTENT":  func(env *Environment) itemValue { return itemLong(env.sysgenPQL("WSEXTENT")) },
	"SYI$_PQL_DPGFLQUOTA": func(env *Environment) itemValue { return itemLong(env.sysgenPQL("PGFLQUOTA")) },
	"SYI$_PQL_MPGFLQUOTA": func(env *Environment) itemValue { return itemLong(pqlMinimum("PGFLQUOTA")) },
	"SYI$_MPW_WRTCLUSTER": func(env *Environment) itemValue { return itemLong(env.Sysgen.MPWWriteCluster) },
	"SYI$_PAGEFILE_PAGE": func(env *Environment) itemValue {
		size, _ := env.pageFileSizes()

		return itemLong(size)
	},
	"SYI$_PAGEFILE_FREE": func(env *Environment) itemValue {
		_, free := env.pageFileSizes()

		return itemLong(free)
	},
	"SYI$_GBLSECTIONS":     func(env *Environment) itemValue { return itemLong(uint32(env.Sections.SectionLimit)) },
	"SYI$_GBLPAGES":        func(env *Environment) itemValue { return itemLong(uint32(env.Sections.PageLimit)) },
	"SYI$_GBLPAGFIL":       func(env *Environment) itemValue { return itemLong(uint32(env.Sections.PageFileLimit)) },
	"SYI$_FREE_GBLSECTS":   func(env *Environment) itemValue { return itemLong(uint32(env.Sections.FreeSections())) },
	"SYI$_FREE_GBLPAGES":   func(env *Environment) itemValue { return itemLong(uint32(env.Sections.FreePages())) },
	"SYI$_CONTIG_GBLPAGES": func(env *Environment) itemValue { return itemLong(uint32(env.Sections.FreePages())) },
	"SYI$_SWAPFILE_PAGE":   func(env *Environment) itemValue { return itemLong(0) },
	"SYI$_SWAPFILE_FREE":   func(env *Environment) itemValue { return itemLong(0) },
}

// defaultBorrowLimit is SYSGEN's default for BORROWLIM, which only
// $GETSYI reports: govax has no automatic working set adjustment to
// borrow with.
const defaultBorrowLimit = 300

// defaultPageTablePFC is SYSGEN's default for PAGTBLPFC, the read cluster
// of process page table pages, which VMS 7.1 reported (testdata/pg65's
// G1): govax's page tables aren't paged (Phase 62, Decision 13), so only
// $GETSYI reports it.
const defaultPageTablePFC = 2

// pqlMinimum is the PQL_M parameter for the quota called name: the
// smallest value $CREPRC gives it.
func pqlMinimum(name string) uint32 {
	for i := range quotaRules {
		if quotaRules[i].name == name {
			return quotaRules[i].min
		}
	}

	return 0
}

// sysgenPQL is the PQL_D parameter for the quota called name: the
// default $CREPRC gives it (Sysgen.pqlDefault).
func (sys *System) sysgenPQL(name string) uint32 {
	for i := range quotaRules {
		if quotaRules[i].name == name {
			return sys.Sysgen.pqlDefault(&quotaRules[i])
		}
	}

	return 0
}

// syiCodeOf returns the $SYIDEF item code called name. A few items
// (FREE_GBLPAGES, FREE_GBLSECTS, CONTIG_GBLPAGES) are among STARLET.OLB's
// definitions but not vmsdef.Symbols'.
func syiCodeOf(name string) (uint32, bool) {
	if code, ok := vmsdef.Symbols[name]; ok {
		return code, true
	}

	code, ok := vmsdef.LibrarySymbols[name]

	return code, ok
}

// syiItems is syiItemsByName keyed by item code.
var syiItems = func() map[uint16]func(*Environment) itemValue {
	out := map[uint16]func(*Environment) itemValue{}

	for name, fn := range syiItemsByName {
		code, ok := syiCodeOf(name)
		if !ok {
			panic("rtl: no $SYIDEF item code " + name)
		}

		out[uint16(code)] = fn
	}

	return out
}()

// serviceSysGetsyi is SYS$GETSYI and SYS$GETSYIW:
//
//	SYS$GETSYI[W] [efn] ,[csidadr] ,[nodename] ,itmlst [,iosb] [,astadr] [,astprm]
//
// The node is picked by nodeTarget. As for $GETJPI, the request
// completes at once, so both forms behave the same: event flag efn
// (default 0) is cleared and then set, the IOSB gets the final status, and
// if astadr isn't 0 an AST is queued to call it with astprm in the
// caller's mode. A request rejected before it starts (too few arguments,
// a bad event flag, no such node) completes nothing. An item code not in
// the registry ends the item list with SS$_BADPARAM, but the request
// still completes, with that status.
//
// Not implemented: the ASTLM quota (SS$_EXASTLM); other cluster nodes.
func serviceSysGetsyi(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 4 {
		return ssInsfArg, nil
	}

	efn, csidadr, nodename, itmlst := argv[0], argv[1], argv[2], argv[3]
	iosb, astadr, astprm := optArg(argv, 4), optArg(argv, 5), optArg(argv, 6)

	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*flags &^= 1 << bit

	if iosb != 0 && !env.storeQuad(iosb, 0) {
		return ssAccVio, nil
	}

	if st := env.nodeTarget(csidadr, nodename); st != 0 {
		return st, nil
	}

	status := env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		item, ok := syiItems[e.ItemCode]
		if !ok {
			return ssBadParam
		}

		return env.storeItem(e, item(env))
	})
	if status == 0 {
		status = ssNormal
	}

	if iosb != 0 {
		if err := env.mem.StoreLongword(env.cpu, iosb, status); err != nil {
			return ssAccVio, nil
		}
	}

	env.postFlag(efn, sched.ClassIOCompletion)

	if astadr != 0 {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return status, nil
}

// nodeTarget checks that csidadr/nodename — the cluster system ID by
// reference and the node name by descriptor — name this node, returning
// 0 if so. With neither, this node is meant. As the manual says, if both
// are given they must name the same node.
//
//   - The longword at csidadr: 0 (localCSID) is this node, and is written
//     back unchanged. -1 starts a wildcard scan, which returns this node
//     and leaves syiWildcardDone at csidadr; the next call with that
//     value is SS$_NOMORENODE. Any other value is SS$_NOSUCHNODE.
//   - nodename must be this node's name exactly (1-15 characters, no
//     abbreviation or trailing blanks), or SS$_NOSUCHNODE; empty or too
//     long is SS$_IVLOGNAM.
//
// An unreadable or unwritable argument is SS$_ACCVIO.
func (env *Environment) nodeTarget(csidadr, nodename uint32) uint32 {
	if csidadr != 0 {
		csid, err := env.mem.LoadLongword(env.cpu, csidadr)
		if err != nil {
			return ssAccVio
		}

		switch csid {
		case syiWildcard:
			if err := env.mem.StoreLongword(env.cpu, csidadr, syiWildcardDone); err != nil {
				return ssAccVio
			}

		case syiWildcardDone:
			return ssNoMoreNode

		case localCSID:
			if err := env.mem.StoreLongword(env.cpu, csidadr, localCSID); err != nil {
				return ssAccVio
			}

		default:
			return ssNoSuchNode
		}
	}

	if nodename != 0 {
		name, ok, err := strGet(env, nodename, maxNodeNameLength)
		if err != nil {
			return ssAccVio
		}

		if !ok {
			return ssIvLogNam
		}

		return env.nodeTargetName(0, name, true)
	}

	return 0
}

// nodeTargetName is nodeTarget with the node name already read: name, if
// named.
func (env *Environment) nodeTargetName(csidadr uint32, name string, named bool) uint32 {
	if st := env.nodeTarget(csidadr, 0); st != 0 || !named {
		return st
	}

	switch {
	case name == "" || len(name) > maxNodeNameLength:
		return ssIvLogNam
	case name != env.NodeName:
		return ssNoSuchNode
	}

	return 0
}

func registerSYIServices(t *ServiceTable) {
	t.Register("SYS$GETSYI", serviceSysGetsyi)
	t.Register("SYS$GETSYIW", serviceSysGetsyi)
}
