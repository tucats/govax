package corevms

import (
	"errors"
	"sort"

	"github.com/tucats/govax/internal/vmsdef"
)

// Event flags (docs/PHASE-26.md). A process has 128 event flags in four
// 32-flag clusters: clusters 0 and 1 (flags 0-63) are local to the process
// (Process.LocalEventFlags), and clusters 2 and 3 (flags 64-127) are
// common event flag clusters, named and shared by the processes in a UIC
// group, that the process reaches only after associating one with the
// cluster number through $ASCEFC.

// Status codes the event-flag services return.
var (
	ssIllEfc  = vmsdef.Symbols["SS$_ILLEFC"]
	ssUnasEfc = vmsdef.Symbols["SS$_UNASEFC"]
)

// ErrWait is what a service returns when the process must wait: its event
// flag condition isn't met yet ($WAITFR, $SYNCH), or no wakeup has arrived
// ($HIBER). The console translates it to
// cpu.ErrServiceWait, which re-executes the service's XFC on the next
// instruction step, so the service checks again while interrupts (the
// interval timer) keep being delivered in between. It is never a VMS
// status: R0 is left alone until the wait is satisfied.
var ErrWait = errors.New("rtl: process waiting")

// maxClusterNameLength is the longest common event flag cluster name
// $ASCEFC accepts (SS$_IVLOGNAM beyond it).
const maxClusterNameLength = 15

// EventFlagCluster is one common event flag cluster (a VMS common event
// block).
type EventFlagCluster struct {
	// Name is the cluster name as given to $ASCEFC. Names are unique
	// within a UIC group, so Group is part of the cluster's identity.
	Name  string
	Group uint32

	// CreatorUIC is the UIC of the process that created the cluster.
	// When Protected, only a process with that UIC may associate.
	CreatorUIC uint32
	Protected  bool

	// Permanent clusters outlive their last association; temporary ones
	// are deleted when no process is associated any more. $DLCEFC marks a
	// permanent cluster DeletePending, after which it too is deleted once
	// its last association goes.
	Permanent     bool
	DeletePending bool

	// Flags holds the cluster's 32 event flags, all clear when created.
	Flags uint32

	refs int // associations (per process and cluster number)
}

// References returns how many associations the cluster has.
func (c *EventFlagCluster) References() int { return c.refs }

type clusterKey struct {
	group uint32
	name  string
}

// CommonEventFlags is the system-wide table of common event flag
// clusters. Clusters live in system memory on VMS, so the table belongs to
// the Environment (rebuilt by INIT/VMINIT/ZERO, which wipe memory) rather
// than to one image: a permanent cluster survives from one RUN to the
// next.
type CommonEventFlags struct {
	clusters map[clusterKey]*EventFlagCluster
}

// NewCommonEventFlags returns an empty cluster table.
func NewCommonEventFlags() *CommonEventFlags {
	return &CommonEventFlags{clusters: map[clusterKey]*EventFlagCluster{}}
}

// Lookup returns the cluster named name in UIC group group.
func (t *CommonEventFlags) Lookup(group uint32, name string) (*EventFlagCluster, bool) {
	c, ok := t.clusters[clusterKey{group, name}]

	return c, ok
}

// All returns every cluster, ordered by group and then name.
func (t *CommonEventFlags) All() []*EventFlagCluster {
	out := make([]*EventFlagCluster, 0, len(t.clusters))
	for _, c := range t.clusters {
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}

		return out[i].Name < out[j].Name
	})

	return out
}

// release drops one association from c, then deletes it if nothing
// keeps it (see deleteIfUnused).
func (t *CommonEventFlags) release(c *EventFlagCluster) {
	c.refs--
	t.deleteIfUnused(c)
}

// deleteIfUnused deletes c when it has no associations and is temporary
// or marked for deletion.
func (t *CommonEventFlags) deleteIfUnused(c *EventFlagCluster) {
	if c.refs <= 0 && (!c.Permanent || c.DeletePending) {
		delete(t.clusters, clusterKey{c.Group, c.Name})
	}
}

// eventFlagWord finds the 32-flag longword holding event flag efn (only
// its low byte counts) and efn's bit in it: SS$_ILLEFC past flag 127,
// SS$_UNASEFC for a common cluster the process hasn't associated. It is
// every event-flag service's way in, so it first lets any expired
// $SETIMR timers set their flags (timers.go).
func (env *Environment) eventFlagWord(efn uint32) (*uint32, uint32, uint32) {
	env.expireTimers()

	return env.flagWord(efn)
}

// flagWord is eventFlagWord without expiring timers first.
func (env *Environment) flagWord(efn uint32) (*uint32, uint32, uint32) {
	efn &= 0xFF
	if efn > 127 {
		return nil, 0, ssIllEfc
	}

	cluster, bit := efn/32, efn%32
	p := env.Process

	if cluster < 2 {
		return &p.LocalEventFlags[cluster], bit, 0
	}

	c := p.CommonClusters[cluster-2]
	if c == nil {
		return nil, 0, ssUnasEfc
	}

	return &c.Flags, bit, 0
}

// flagStatus is SS$_WASSET or SS$_WASCLR for bit of word.
func flagStatus(word, bit uint32) uint32 {
	if word&(1<<bit) != 0 {
		return ssWasSet
	}

	return ssWasClr
}

// serviceSysClref is SYS$CLREF: clears one event flag, returning its
// previous state as SS$_WASSET/SS$_WASCLR.
func serviceSysClref(env *Environment, argv []uint32) (uint32, error) {
	word, bit, st := env.eventFlagWord(argv[0])
	if st != 0 {
		return st, nil
	}

	st = flagStatus(*word, bit)
	*word &^= 1 << bit

	return st, nil
}

// serviceSysSetef is SYS$SETEF: sets one event flag, returning its
// previous state as SS$_WASSET/SS$_WASCLR.
func serviceSysSetef(env *Environment, argv []uint32) (uint32, error) {
	word, bit, st := env.eventFlagWord(argv[0])
	if st != 0 {
		return st, nil
	}

	st = flagStatus(*word, bit)
	*word |= 1 << bit

	return st, nil
}

// serviceSysReadef is SYS$READEF: returns the whole 32-flag cluster
// containing efn through state, and whether efn itself is set as
// SS$_WASSET/SS$_WASCLR. state is required by the manual, but a call that
// omits it (as eVAX allowed) just gets the status.
func serviceSysReadef(env *Environment, argv []uint32) (uint32, error) {
	word, bit, st := env.eventFlagWord(argv[0])
	if st != 0 {
		return st, nil
	}

	if state := optArg(argv, 1); state != 0 {
		if err := env.mem.StoreLongword(env.cpu, state, *word); err != nil {
			return ssAccVio, nil
		}
	}

	return flagStatus(*word, bit), nil
}

// serviceSysAscefc is SYS$ASCEFC: associates the common event flag
// cluster named name (in the process's UIC group) with cluster number 2 or
// 3 — whichever holds event flag efn — creating the cluster, with all
// flags clear, if it doesn't exist. prot makes a new cluster usable only
// by the creator's UIC; perm makes it permanent. A cluster number already
// associated with a different cluster is disassociated from it first.
//
// Creating a permanent cluster needs the PRMCEB privilege (SS$_NOPRIV
// otherwise; privilege.go). There are no quotas, shared memory, or
// multiport clusters (docs/DEVIATIONS.md).
func serviceSysAscefc(env *Environment, argv []uint32) (uint32, error) {
	efn, nameDesc := optArg(argv, 0)&0xFF, optArg(argv, 1)
	prot, perm := optArg(argv, 2)&1 != 0, optArg(argv, 3)&1 != 0

	if efn < 64 || efn > 127 {
		return ssIllEfc, nil
	}

	name, st := clusterName(env, nameDesc)
	if st != 0 {
		return st, nil
	}

	p, table := env.Process, env.EventFlagClusters
	slot := efn/32 - 2

	c, found := table.Lookup(p.UICGroup(), name)
	if found && c.Protected && c.CreatorUIC != p.UIC {
		return ssNoPriv, nil
	}

	if found && p.CommonClusters[slot] == c {
		return ssNormal, nil
	}

	if !found {
		if perm && !p.hasPrivilege(privPRMCEB) {
			return ssNoPriv, nil
		}

		c = &EventFlagCluster{
			Name:       name,
			Group:      p.UICGroup(),
			CreatorUIC: p.UIC,
			Protected:  prot,
			Permanent:  perm,
		}
		table.clusters[clusterKey{c.Group, c.Name}] = c
	}

	env.disassociateCluster(slot)

	c.refs++
	p.CommonClusters[slot] = c

	return ssNormal, nil
}

// clusterName reads a common event flag cluster name argument: 1-15
// characters (SS$_IVLOGNAM otherwise), and SS$_ACCVIO for a missing or
// unreadable descriptor.
func clusterName(env *Environment, desc uint32) (string, uint32) {
	if desc == 0 { // page 0 is never accessible on VMS
		return "", ssAccVio
	}

	name, ok, err := strGet(env, desc, maxClusterNameLength)
	if err != nil {
		return "", ssAccVio
	}

	if !ok || name == "" {
		return "", ssIvLogNam
	}

	return name, 0
}

// serviceSysDacefc is SYS$DACEFC: drops the process's association with
// the common cluster holding event flag efn (low byte, 64-127, else
// SS$_ILLEFC). A cluster number with no association succeeds anyway, as
// the manual says. A temporary or marked-for-deletion cluster left with
// no associations is deleted.
func serviceSysDacefc(env *Environment, argv []uint32) (uint32, error) {
	efn := optArg(argv, 0) & 0xFF
	if efn < 64 || efn > 127 {
		return ssIllEfc, nil
	}

	env.disassociateCluster(efn/32 - 2)

	return ssNormal, nil
}

// serviceSysDlcefc is SYS$DLCEFC: marks the common cluster named name (in
// the process's UIC group) for deletion; it is deleted now if nothing is
// associated with it, otherwise when the last association goes. It
// doesn't disassociate anyone. A cluster that doesn't exist succeeds
// anyway. Deleting needs PRMCEB or the creator's UIC; the emulated
// process always has PRMCEB, so SS$_NOPRIV can't happen.
func serviceSysDlcefc(env *Environment, argv []uint32) (uint32, error) {
	name, st := clusterName(env, optArg(argv, 0))
	if st != 0 {
		return st, nil
	}

	table := env.EventFlagClusters
	if c, found := table.Lookup(env.Process.UICGroup(), name); found {
		// Deleting takes PRMCEB, or being the creator.
		if c.CreatorUIC != env.Process.UIC && !env.Process.hasPrivilege(privPRMCEB) {
			return ssNoPriv, nil
		}

		c.DeletePending = true
		table.deleteIfUnused(c)
	}

	return ssNormal, nil
}

// disassociateCluster drops the process's association for common cluster
// number slot+2, if it has one.
func (env *Environment) disassociateCluster(slot uint32) {
	if old := env.Process.CommonClusters[slot]; old != nil {
		env.Process.CommonClusters[slot] = nil
		env.EventFlagClusters.release(old)
	}
}

// disassociateClusters is image rundown's event-flag step: associations
// last only for the current image.
func (env *Environment) disassociateClusters() {
	for slot := range env.Process.CommonClusters {
		env.disassociateCluster(uint32(slot))
	}
}

// waitFor is the shared body of the wait services: it finds efn's cluster
// (SS$_ILLEFC/SS$_UNASEFC as for $SETEF) and completes with SS$_NORMAL
// once done(cluster flags) holds, or reports ErrWait to be called again.
func (env *Environment) waitFor(efn uint32, done func(flags uint32) bool) (uint32, error) {
	word, _, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	if !done(*word) {
		return 0, ErrWait
	}

	return ssNormal, nil
}

// serviceSysWaitfr is SYS$WAITFR: waits until event flag efn is set.
func serviceSysWaitfr(env *Environment, argv []uint32) (uint32, error) {
	_, bit, _ := env.eventFlagWord(argv[0])

	return env.waitFor(argv[0], func(flags uint32) bool { return flags&(1<<bit) != 0 })
}

// serviceSysWfland is SYS$WFLAND: waits until every flag mask selects, in
// the cluster holding efn, is set. An empty mask is satisfied at once.
func serviceSysWfland(env *Environment, argv []uint32) (uint32, error) {
	mask := argv[1]

	return env.waitFor(argv[0], func(flags uint32) bool { return flags&mask == mask })
}

// serviceSysWflor is SYS$WFLOR: waits until any flag mask selects, in the
// cluster holding efn, is set. An empty mask never is, so the process
// waits indefinitely, as it would on VMS.
func serviceSysWflor(env *Environment, argv []uint32) (uint32, error) {
	mask := argv[1]

	return env.waitFor(argv[0], func(flags uint32) bool { return flags&mask != 0 })
}

// serviceSysSynch is SYS$SYNCH:
//
//	SYS$SYNCH [efn] ,[iosb]
//
// It waits for a request made with event flag efn and I/O status block
// iosb — a $QIO, $GETJPI, $GETSYI, ... — to complete. An event flag alone
// can't say that: flags are shared, and another request, a timer, or a
// $SETEF may set the same one. The IOSB can: the request writes a nonzero
// condition value to its first word when it completes, and nothing else
// writes it. So, as the manual describes:
//
//  1. Wait (as $WAITFR does) until the event flag is set.
//  2. If the IOSB's condition value is nonzero, the request is complete:
//     return SS$_NORMAL, with the flag still set.
//  3. Otherwise something else set the flag. Clear it and go back to 1.
//
// With iosb omitted, only step 1 is done. The flag is left set on return,
// so a second $SYNCH for the other request sharing the flag also sees it
// ("the flag set by $GETSYI is not lost"). A request whose ...W form
// completes during the call (every govax one) is already complete, so
// $SYNCH returns at once.
//
// SS$_ILLEFC and SS$_UNASEFC are as for $WAITFR. An unreadable iosb is
// SS$_ACCVIO; the manual lists only SS$_NORMAL, but VMS would stop the
// program with an access violation there.
func serviceSysSynch(env *Environment, argv []uint32) (uint32, error) {
	efn, iosb := optArg(argv, 0), optArg(argv, 1)

	word, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	if *word&(1<<bit) == 0 {
		return 0, ErrWait // step 1: not set yet
	}

	if iosb == 0 {
		return ssNormal, nil
	}

	status, err := env.mem.LoadWord(env.cpu, iosb)
	if err != nil {
		return ssAccVio, nil
	}

	if status != 0 {
		return ssNormal, nil // step 2: the request completed
	}

	*word &^= 1 << bit // step 3: a false alarm

	return 0, ErrWait
}

func registerEventFlagServices(t *ServiceTable) {
	t.Register("SYS$CLREF", serviceSysClref)
	t.Register("SYS$SETEF", serviceSysSetef)
	t.Register("SYS$READEF", serviceSysReadef)
	t.Register("SYS$ASCEFC", serviceSysAscefc)
	t.Register("SYS$DACEFC", serviceSysDacefc)
	t.Register("SYS$DLCEFC", serviceSysDlcefc)
	t.Register("SYS$WAITFR", serviceSysWaitfr)
	t.Register("SYS$WFLAND", serviceSysWfland)
	t.Register("SYS$WFLOR", serviceSysWflor)
	t.Register("SYS$SYNCH", serviceSysSynch)
}
