package rtl

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

func TestEventFlagStatuses(t *testing.T) {
	env, _ := fixture()

	// $SETEF and $CLREF report the flag's previous state; only the low
	// byte of efn counts (0x120 is flag 32, in local cluster 1).
	wantR0(t, callLNM(t, env, serviceSysSetef, 0x120), ssWasClr)
	wantR0(t, callLNM(t, env, serviceSysSetef, 32), ssWasSet)

	if env.Process.LocalEventFlags[1] != 1 {
		t.Errorf("LocalEventFlags[1] = %#x, want flag 32 (bit 0) set", env.Process.LocalEventFlags[1])
	}

	wantR0(t, callLNM(t, env, serviceSysClref, 32), ssWasSet)
	wantR0(t, callLNM(t, env, serviceSysClref, 32), ssWasClr)

	// Past flag 127 is illegal; clusters 2 and 3 need an association.
	for _, fn := range []ServiceFunc{serviceSysSetef, serviceSysClref, serviceSysReadef} {
		wantR0(t, callLNM(t, env, fn, 128), ssIllEfc)
		wantR0(t, callLNM(t, env, fn, 64), ssUnasEfc)
		wantR0(t, callLNM(t, env, fn, 127), ssUnasEfc)
	}
}

func TestServiceSysAscefc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// Associating creates the cluster; flags 64-95 then reach it.
	wantR0(t, callLNM(t, env, serviceSysAscefc, 70, a.desc("SHARED")), ssNormal)

	c, found := env.EventFlagClusters.Lookup(1, "SHARED")
	if !found {
		t.Fatal("cluster SHARED not created in group 1")
	}

	if c.CreatorUIC != NominalUIC || c.Permanent || c.Protected || c.References() != 1 {
		t.Errorf("cluster = %+v, want a temporary, unprotected cluster made by [1,4] with one reference", c)
	}

	wantR0(t, callLNM(t, env, serviceSysSetef, 65), ssWasClr)

	if c.Flags != 2 {
		t.Errorf("cluster flags = %#x, want flag 65 (bit 1) set", c.Flags)
	}

	state := a.long(0)
	wantR0(t, callLNM(t, env, serviceSysReadef, 95, state), ssWasClr)

	if got := a.readLong(state); got != 2 {
		t.Errorf("READEF state = %#x, want the cluster's flags 0x2", got)
	}

	// Cluster 3 still isn't associated.
	wantR0(t, callLNM(t, env, serviceSysSetef, 96), ssUnasEfc)

	// The same cluster can also be associated with cluster 3, sharing its
	// flags; associating the same number again is a no-op.
	wantR0(t, callLNM(t, env, serviceSysAscefc, 100, a.desc("SHARED")), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("SHARED")), ssNormal)

	if c.References() != 2 {
		t.Errorf("references = %d, want 2 (clusters 2 and 3)", c.References())
	}

	wantR0(t, callLNM(t, env, serviceSysReadef, 97), ssWasSet)
}

func TestServiceSysAscefcReassociate(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	table := env.EventFlagClusters

	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("TEMP")), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("KEEP"), 0, 1), ssNormal)

	// Reassigning cluster 2 disassociated TEMP, its only user, deleting it.
	if _, found := table.Lookup(1, "TEMP"); found {
		t.Error("temporary cluster TEMP still exists after its last disassociation")
	}

	keep, _ := table.Lookup(1, "KEEP")
	wantR0(t, callLNM(t, env, serviceSysSetef, 64), ssWasClr)

	// Image rundown disassociates; the permanent cluster and its flags
	// stay for the next image.
	env.ImageRundown()

	if env.Process.CommonClusters != [2]*EventFlagCluster{} {
		t.Errorf("associations after rundown = %v, want none", env.Process.CommonClusters)
	}

	wantR0(t, callLNM(t, env, serviceSysSetef, 64), ssUnasEfc)

	if got, found := table.Lookup(1, "KEEP"); !found || got != keep || keep.References() != 0 {
		t.Fatalf("permanent cluster KEEP after rundown: found=%v refs=%d, want kept with no references", found, keep.References())
	}

	wantR0(t, callLNM(t, env, serviceSysAscefc, 127, a.desc("KEEP")), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysReadef, 96), ssWasSet)
}

func TestServiceSysAscefcProtection(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process

	// [1,4] creates a protected cluster and stays associated with it.
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("PRIVATE"), 1), ssNormal)

	// Another UIC in the same group finds it but may not associate.
	p.UIC = 0x00010005
	wantR0(t, callLNM(t, env, serviceSysAscefc, 96, a.desc("PRIVATE")), ssNoPriv)

	if p.CommonClusters[1] != nil {
		t.Error("cluster 3 associated despite SS$_NOPRIV")
	}

	// Names are per group: in group 2, PRIVATE is a different cluster.
	p.UIC = 0x00020004
	wantR0(t, callLNM(t, env, serviceSysAscefc, 96, a.desc("PRIVATE")), ssNormal)

	g1, _ := env.EventFlagClusters.Lookup(1, "PRIVATE")
	g2, _ := env.EventFlagClusters.Lookup(2, "PRIVATE")

	if g1 == nil || g2 == nil || g1 == g2 {
		t.Fatalf("group 1 cluster %p, group 2 cluster %p, want two distinct clusters", g1, g2)
	}

	if all := env.EventFlagClusters.All(); len(all) != 2 || all[0] != g1 || all[1] != g2 {
		t.Errorf("All() = %v, want the group 1 cluster then the group 2 one", all)
	}
}

func TestServiceSysAscefcErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	wantR0(t, callLNM(t, env, serviceSysAscefc, 63, a.desc("X")), ssIllEfc)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 128, a.desc("X")), ssIllEfc)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("")), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc(strings.Repeat("N", 16))), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc(strings.Repeat("N", 15))), ssNormal)
}

func TestServiceSysDacefc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	table := env.EventFlagClusters

	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("TEMP")), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 96, a.desc("PERM"), 0, 1), ssNormal)

	// Disassociating a temporary cluster's last user deletes it.
	wantR0(t, callLNM(t, env, serviceSysDacefc, 90), ssNormal)

	if _, found := table.Lookup(1, "TEMP"); found {
		t.Error("TEMP survived its last $DACEFC")
	}

	wantR0(t, callLNM(t, env, serviceSysSetef, 64), ssUnasEfc)

	// A permanent one stays; only the low byte of efn counts.
	wantR0(t, callLNM(t, env, serviceSysDacefc, 0x160), ssNormal)

	if c, found := table.Lookup(1, "PERM"); !found || c.References() != 0 {
		t.Error("permanent cluster PERM not kept, unreferenced, after $DACEFC")
	}

	// An unassociated cluster number succeeds; outside 64-127 doesn't.
	wantR0(t, callLNM(t, env, serviceSysDacefc, 100), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDacefc, 63), ssIllEfc)
	wantR0(t, callLNM(t, env, serviceSysDacefc, 128), ssIllEfc)
}

func TestServiceSysDlcefc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	table := env.EventFlagClusters

	// Unused permanent cluster: deleted at once.
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("IDLE"), 0, 1), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDacefc, 64), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDlcefc, a.desc("IDLE")), ssNormal)

	if _, found := table.Lookup(1, "IDLE"); found {
		t.Error("unassociated permanent cluster IDLE survived $DLCEFC")
	}

	// In use: marked, still associated and usable, deleted when the last
	// association goes (here, at image rundown).
	wantR0(t, callLNM(t, env, serviceSysAscefc, 64, a.desc("BUSY"), 0, 1), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDlcefc, a.desc("BUSY")), ssNormal)

	c, found := table.Lookup(1, "BUSY")
	if !found || !c.DeletePending {
		t.Fatal("associated cluster BUSY not kept and marked by $DLCEFC")
	}

	wantR0(t, callLNM(t, env, serviceSysSetef, 64), ssWasClr)
	env.ImageRundown()

	if _, found := table.Lookup(1, "BUSY"); found {
		t.Error("marked cluster BUSY survived its last disassociation")
	}

	// A nonexistent cluster succeeds; a bad name doesn't.
	wantR0(t, callLNM(t, env, serviceSysDlcefc, a.desc("NOSUCH")), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDlcefc, a.desc("")), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysDlcefc, a.desc(strings.Repeat("N", 16))), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysDlcefc, 0), ssAccVio)
}

// wantWait checks a wait service call reports ErrWait (not done yet).
func wantWait(t *testing.T, env *Environment, fn ServiceFunc, argv ...uint32) {
	t.Helper()

	if _, err := fn(env, argv); !errors.Is(err, ErrWait) {
		t.Errorf("err = %v, want ErrWait", err)
	}
}

func TestEventFlagWaits(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	// $WAITFR: waits while the flag is clear, completes once it's set;
	// only the low byte of efn counts.
	wantWait(t, env, serviceSysWaitfr, 0x103)
	p.LocalEventFlags[0] |= 1 << 3
	wantR0(t, callLNM(t, env, serviceSysWaitfr, 0x103), ssNormal)

	// $WFLAND: all of the mask; an empty mask is satisfied at once.
	wantWait(t, env, serviceSysWfland, 35, 0x6)
	p.LocalEventFlags[1] |= 0x2
	wantWait(t, env, serviceSysWfland, 35, 0x6)
	p.LocalEventFlags[1] |= 0x4
	wantR0(t, callLNM(t, env, serviceSysWfland, 35, 0x6), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysWfland, 0, 0), ssNormal)

	// $WFLOR: any of the mask; an empty mask never is.
	wantWait(t, env, serviceSysWflor, 0, 0x30)
	p.LocalEventFlags[0] |= 0x20
	wantR0(t, callLNM(t, env, serviceSysWflor, 0, 0x30), ssNormal)
	wantWait(t, env, serviceSysWflor, 0, 0)

	// Common clusters and bad flag numbers, as for $SETEF.
	for _, fn := range []ServiceFunc{serviceSysWaitfr, serviceSysWfland, serviceSysWflor} {
		wantR0(t, callLNM(t, env, fn, 128, 1), ssIllEfc)
		wantR0(t, callLNM(t, env, fn, 64, 1), ssUnasEfc)
	}

	a := newArena(t, env)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 96, a.desc("C")), ssNormal)
	wantWait(t, env, serviceSysWaitfr, 97)
	wantR0(t, callLNM(t, env, serviceSysSetef, 97), ssWasClr)
	wantR0(t, callLNM(t, env, serviceSysWaitfr, 97), ssNormal)
}

// TestEventFlagWaitTrace: a waiting service is called again every step;
// DEBUG(SERVICES) traces its first attempt ("waits") and its completion,
// not every retry.
func TestEventFlagWaitTrace(t *testing.T) {
	env, _ := fixture()

	var buf bytes.Buffer

	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(vax.DebugServices)

	waitfr := uint32(0)
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$WAITFR" {
			waitfr = e.Addr
		}
	}

	putArgs(t, env, 0x2000, []uint32{3})

	for i := 0; i < 3; i++ {
		if _, handled, err := env.SystemService(waitfr); !handled || !errors.Is(err, ErrWait) {
			t.Fatalf("attempt %d: handled=%v err=%v, want ErrWait", i, handled, err)
		}
	}

	env.Process.LocalEventFlags[0] |= 1 << 3

	if r0, _, err := env.SystemService(waitfr); err != nil || r0 != ssNormal {
		t.Fatalf("after setting the flag: r0=%#x err=%v", r0, err)
	}

	out := buf.String()
	if n := strings.Count(out, "SYS$WAITFR( 00000003 ), waits"); n != 1 {
		t.Errorf("%d \"waits\" traces, want 1:\n%s", n, out)
	}

	if !strings.Contains(out, "SYS$WAITFR( 00000003 ), returns 00000001") {
		t.Errorf("no completion trace:\n%s", out)
	}
}

func TestServiceSysSynch(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	iosb := a.alloc(8)
	p := env.Process

	// Step 1: the flag isn't set, so it waits.
	p.LocalEventFlags[0] = 0
	putLongword(t, env, iosb, 0)
	wantWait(t, env, serviceSysSynch, 5, iosb)

	// Step 3: the flag is set but the IOSB is still 0 — another event set
	// the flag. $SYNCH clears it and keeps waiting.
	p.LocalEventFlags[0] = 1 << 5
	wantWait(t, env, serviceSysSynch, 5, iosb)

	if flagSet(env, 5) {
		t.Error("a false alarm left the flag set")
	}

	// Step 2: the request completed (a nonzero status word). $SYNCH
	// returns, leaving the flag set. Only the status word counts.
	p.LocalEventFlags[0] = 1 << 5
	putLongword(t, env, iosb, 0xFFFF0000)
	wantWait(t, env, serviceSysSynch, 5, iosb)

	p.LocalEventFlags[0] = 1 << 5
	putLongword(t, env, iosb, ssNormal)
	wantR0(t, callLNM(t, env, serviceSysSynch, 5, iosb), ssNormal)

	if !flagSet(env, 5) {
		t.Error("$SYNCH cleared the flag of a completed request")
	}

	// Without an IOSB, only the flag counts; efn defaults to 0.
	p.LocalEventFlags[0] = 1
	wantR0(t, callLNM(t, env, serviceSysSynch), ssNormal)

	p.LocalEventFlags[0] = 0
	wantWait(t, env, serviceSysSynch)

	// Errors.
	p.LocalEventFlags[0] = 1 << 5
	wantR0(t, callLNM(t, env, serviceSysSynch, 5, badAddr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysSynch, 200, iosb), ssIllEfc)
	wantR0(t, callLNM(t, env, serviceSysSynch, 64, iosb), ssUnasEfc)
}

// TestServiceSysSynch_afterQIO checks the pattern $SYNCH exists for: a
// $QIO with an event flag and IOSB, some other work, then $SYNCH. The
// request completed during $QIO, so $SYNCH returns at once.
func TestServiceSysSynch_afterQIO(t *testing.T) {
	env, _, a, ch := qioFixture(t, "")
	iosb := a.alloc(8)

	wantR0(t, callQIO(t, env, qioArgs{efn: 7, channel: ch, function: fnWriteVBlk, iosb: iosb, p: [6]uint32{a.str("x"), 1}}), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysSynch, 7, iosb), ssNormal)
}
