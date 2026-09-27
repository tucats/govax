package rtl

import (
	"strings"
	"testing"
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
