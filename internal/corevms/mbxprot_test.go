package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 3: a mailbox's protection and lifetime between
// processes, and $GETDVI of it from either side.

// uic makes a UIC from its group and member.
func uic(group, member uint32) uint32 { return group<<16 | member }

func TestUICAccess(t *testing.T) {
	owner := uic(0o100, 1)

	// System: nothing denied. Owner: physical I/O denied. Group: write
	// and physical I/O denied. World: everything denied. (A process is
	// in every category it qualifies for, so the owner is in GROUP and
	// WORLD too.)
	const mask = 0xFA80

	cases := []struct {
		name  string
		uic   uint32
		privs uint64
		want  uint32
		ok    bool
	}{
		{"owner reads", owner, 0, accessRead, true},
		{"owner writes", owner, 0, accessWrite, true},
		{"owner physical I/O", owner, 0, accessPhysical, false},
		{"group reads", uic(0o100, 2), 0, accessRead, true},
		{"group writes", uic(0o100, 2), 0, accessWrite, false},
		{"group reads and writes", uic(0o100, 2), 0, accessRead | accessWrite, false},
		{"world reads", uic(0o200, 1), 0, accessRead, false},
		{"system group", uic(1, 4), 0, accessRead | accessWrite | accessPhysical, true},
		{"world with SYSPRV", uic(0o200, 1), privSYSPRV, accessWrite, true},
		{"world with BYPASS", uic(0o200, 1), privBYPASS, accessPhysical, true},
		{"world with READALL reads", uic(0o200, 1), privREADALL, accessRead, true},
		{"world with READALL writes", uic(0o200, 1), privREADALL, accessWrite, false},
		{"group with GRPPRV", uic(0o100, 2), privGRPPRV, accessWrite, true},
		{"world with GRPPRV", uic(0o200, 1), privGRPPRV, accessRead, false},
	}

	for _, c := range cases {
		p := &Process{UIC: c.uic, CurrentPrivileges: c.privs}
		if got := p.uicAccess(owner, mask, c.want); got != c.ok {
			t.Errorf("%s: %v, want %v", c.name, got, c.ok)
		}
	}

	if !(&Process{UIC: uic(0o200, 1)}).uicAccess(owner, 0, accessRead|accessWrite|accessLogical|accessPhysical) {
		t.Error("a mask of 0 denied something")
	}
}

// protFixture is a mailbox PIPE made with protection mask promsk by an
// unprivileged process (but for TMPMBX) in group 100, and its channel.
func protFixture(t *testing.T, promsk uint32) (*Environment, *arena, uint32) {
	t.Helper()

	env, _ := fixture()
	a := newArena(t, env)

	env.Process.UIC = uic(0o100, 1)
	env.Process.CurrentPrivileges = privTMPMBX

	chanAdr := a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysCrembx, 0, chanAdr, 0, 0, promsk, 0, a.desc("PIPE")), ssNormal)

	return env, a, uint32(a.readLong(chanAdr) & 0xFFFF)
}

// otherUser is another process with UIC u and only privileges privs.
func otherUser(t *testing.T, env *Environment, u uint32, privs uint64) *Environment {
	t.Helper()

	other := newProcess(t, env)
	other.Process.UIC, other.Process.CurrentPrivileges = u, privs

	return other
}

// iosbStatus is the status word of the IOSB at iosb.
func iosbStatus(a *arena, iosb uint32) uint32 { return a.readLong(iosb) & 0xFFFF }

// TestMailboxProtection_assign: $ASSIGN, and $CREMBX of an existing
// name, need read or write access by the mailbox's protection.
func TestMailboxProtection_assign(t *testing.T) {
	env, a, _ := protFixture(t, 0xF200) // group: no write; world: nothing

	stranger := otherUser(t, env, uic(0o200, 1), privTMPMBX)
	chanAdr := a.alloc(2)

	wantR0(t, callLNM(t, stranger, serviceSysAssign, a.desc("PIPE"), chanAdr), ssNoPriv)
	wantR0(t, callLNM(t, stranger, serviceSysAssign, a.desc("MBA1:"), chanAdr), ssNoPriv)
	wantR0(t, callLNM(t, stranger, serviceSysCrembx, 0, chanAdr, 0, 0, 0, 0, a.desc("PIPE")), ssNoPriv)

	if n := len(env.Mailboxes.All()); n != 1 {
		t.Errorf("%d mailboxes, want 1: a refused $CREMBX made one", n)
	}

	if d := mailboxOn(t, env, 8).Device; d.RefCnt != 1 {
		t.Errorf("refcnt %d after refused assignments, want 1", d.RefCnt)
	}

	// A privilege that reaches a category with access lets it in.
	for _, priv := range []uint64{privBYPASS, privSYSPRV} {
		stranger.Process.CurrentPrivileges = priv
		assignCall(t, stranger, a, "PIPE", 0)
	}

	// So does a UIC in the owner's group, or a system group.
	assignCall(t, otherUser(t, env, uic(0o100, 2), 0), a, "PIPE", 0)
	assignCall(t, otherUser(t, env, uic(1, 4), 0), a, "PIPE", 0)
}

// TestMailboxProtection_readWrite: a process that may only read a
// mailbox has its writes refused with SS$_NOPRIV, and one that may only
// write its reads. (A group member is in WORLD too, so WORLD must deny
// what GROUP does.)
func TestMailboxProtection_readWrite(t *testing.T) {
	// Group and world may only read.
	env, a, ch := protFixture(t, 0x2200)
	buf, iosb := a.alloc(16), a.alloc(8)

	reader := otherUser(t, env, uic(0o100, 2), 0)
	rch := assignCall(t, reader, a, "PIPE", 0)

	wantR0(t, mbxQIO(t, reader, 0, rch, fnWriteNow, iosb, 0, a.str("hi"), 2), ssNoPriv)
	wantR0(t, mbxQIO(t, reader, 0, rch, fnWriteEOF, iosb, 0, 0, 0), ssNoPriv)

	if n := mailboxOn(t, env, ch).Messages(); n != 0 {
		t.Fatalf("%d messages after refused writes, want 0", n)
	}

	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, iosb, 0, a.str("hi"), 2), ssNormal)
	wantR0(t, mbxQIO(t, reader, 0, rch, fnReadNow, iosb, 0, buf, 16), ssNormal)

	if st, got := iosbStatus(a, iosb), a.readString(buf, 2); st != ssNormal || got != "hi" {
		t.Errorf("the reader's read: %04X %q, want SS$_NORMAL \"hi\"", st, got)
	}

	// Group and world may only write.
	env, a, ch = protFixture(t, 0x1100)
	buf, iosb = a.alloc(16), a.alloc(8)

	writer := otherUser(t, env, uic(0o200, 1), 0)
	wch := assignCall(t, writer, a, "PIPE", 0)

	wantR0(t, mbxQIO(t, writer, 0, wch, fnWriteNow, iosb, 0, a.str("hi"), 2), ssNormal)
	wantR0(t, mbxQIO(t, writer, 0, wch, fnReadNow, iosb, 0, buf, 16), ssNoPriv)

	if n := mailboxOn(t, env, ch).Messages(); n != 1 {
		t.Errorf("%d messages after a refused read, want 1", n)
	}
}

// TestMailboxProtection_setprot: IO$_SETMODE!IO$M_SETPROT changes the
// mask, for the owner (or a privileged process) only.
func TestMailboxProtection_setprot(t *testing.T) {
	env, a, ch := protFixture(t, 0)

	fnSetProt := vmsdef.Symbols["IO$_SETMODE"] | vmsdef.Symbols["IO$M_SETPROT"]

	groupmate := otherUser(t, env, uic(0o100, 2), 0)
	gch := assignCall(t, groupmate, a, "PIPE", 0)

	wantR0(t, mbxQIO(t, groupmate, 0, gch, fnSetProt, 0, 0, 0, 0xFF00), ssNoPriv)

	if m := mailboxOn(t, env, ch); m.Protection != 0 {
		t.Fatalf("protection %04X after a refused IO$M_SETPROT", m.Protection)
	}

	wantR0(t, mbxQIO(t, env, 0, ch, fnSetProt, 0, 0, 0, 0xFF00), ssNormal)

	if m := mailboxOn(t, env, ch); m.Protection != 0xFF00 {
		t.Errorf("protection %04X, want FF00", m.Protection)
	}

	// Group and world are shut out now; the channel already assigned
	// keeps working only as far as the new mask allows.
	wantR0(t, callLNM(t, groupmate, serviceSysAssign, a.desc("PIPE"), a.alloc(2)), ssNoPriv)
	wantR0(t, mbxQIO(t, groupmate, 0, gch, fnWriteNow, 0, 0, a.str("x"), 1), ssNoPriv)

	groupmate.Process.CurrentPrivileges = privSYSPRV
	wantR0(t, mbxQIO(t, groupmate, 0, gch, fnSetProt, 0, 0, 0, 0), ssNormal)
}

// detached is a process in a job of its own, with its own job table.
func detached(t *testing.T, env *Environment) *Environment {
	t.Helper()

	other := newProcess(t, env)
	other.Logicals = env.Logicals.NewProcessView(other.Process.UIC, env.Logicals.NewJobTable())

	return other
}

// translates reports whether name translates in env's LNM$JOB.
func translates(env *Environment, name string) bool {
	_, err := env.Logicals.Translate("LNM$JOB", name, lnm.User, 0)

	return err == nil
}

// TestMailbox_lifetimeAcrossProcesses: a temporary mailbox lasts while
// any process has a channel to it, and its logical name is in its
// creator's job table: a subprocess finds it, a detached process in
// another job doesn't.
func TestMailbox_lifetimeAcrossProcesses(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	_, ch := crembx(t, env, a, 0, 0, 0, "PIPE")
	m := mailboxOn(t, env, ch)

	child := newSubprocess(t, env)
	cch := assignCall(t, child, a, "PIPE", 0)

	if mailboxOn(t, child, cch) != m || m.Device.RefCnt != 2 {
		t.Fatalf("the subprocess's PIPE isn't the mailbox, or refcnt %d isn't 2", m.Device.RefCnt)
	}

	// Another job doesn't see the name: its $CREMBX makes a mailbox of
	// its own, whose name goes in its own job table.
	other := detached(t, env)
	if translates(other, "PIPE") {
		t.Fatal("PIPE translates in another job")
	}

	_, och := crembx(t, other, a, 0, 0, 0, "PIPE")
	if om := mailboxOn(t, other, och); om == m || om.Device.Name != "MBA2" {
		t.Errorf("another job's PIPE is %s, want a new MBA2", om.Device.Name)
	}

	// The creator's channel goes; the subprocess's keeps the mailbox and
	// its name.
	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	if _, ok := env.Mailboxes.For(m.Device); !ok || !translates(env, "PIPE") {
		t.Fatal("the mailbox or its name went with the creator's channel")
	}

	// The last channel goes, in the subprocess: the mailbox and its name
	// go too; the other job's are untouched.
	wantR0(t, callLNM(t, child, serviceSysDassgn, cch), ssNormal)

	if _, ok := env.Mailboxes.For(m.Device); ok || translates(env, "PIPE") {
		t.Error("the mailbox or its name outlived its last channel")
	}

	if !translates(other, "PIPE") || len(env.Mailboxes.All()) != 1 {
		t.Error("the other job's PIPE went too")
	}
}

// TestMailbox_lastChannelInAnotherJob: a temporary mailbox whose last
// channel is another job's goes with its name, which is in its creator's
// job table, not the deleting process's.
func TestMailbox_lastChannelInAnotherJob(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	_, ch := crembx(t, env, a, 0, 0, 0, "PIPE")
	m := mailboxOn(t, env, ch)

	other := detached(t, env)
	och := assignCall(t, other, a, m.Device.Name+":", 0)

	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)
	wantR0(t, callLNM(t, other, serviceSysDassgn, och), ssNormal)

	if _, ok := env.Mailboxes.For(m.Device); ok {
		t.Error("the mailbox outlived its last channel")
	}

	if translates(env, "PIPE") {
		t.Error("PIPE is still in the creator's job table")
	}
}

// TestMailbox_deletedProcess: a process's deletion releases its
// channels, so a temporary mailbox it held the last channel to goes.
func TestMailbox_deletedProcess(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	_, ch := crembx(t, env, a, 0, 0, 0, "PIPE")
	m := mailboxOn(t, env, ch)

	child := newSubprocess(t, env)
	assignCall(t, child, a, "PIPE", 0)
	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	child.processRundown()

	if _, ok := env.Mailboxes.For(m.Device); ok || translates(env, "PIPE") {
		t.Error("the mailbox or its name outlived the process with its last channel")
	}
}

// TestGetdvi_mailboxBothSides: $GETDVI of a mailbox reports the same
// from its creator's channel and another process's: its message count
// (DEVDEPEND's low word), reference count, owner (the creator's PID and
// UIC, not the last process to assign it), and protection.
func TestGetdvi_mailboxBothSides(t *testing.T) {
	env, a, ch := protFixture(t, 0xF000)
	m := mailboxOn(t, env, ch)

	groupmate := otherUser(t, env, uic(0o100, 2), 0)
	gch := assignCall(t, groupmate, a, "PIPE", 0)

	wantR0(t, mbxQIO(t, groupmate, 0, gch, fnWriteNow, 0, 0, a.str("one"), 3), ssNormal)
	wantR0(t, mbxQIO(t, groupmate, 0, gch, fnWriteNow, 0, 0, a.str("two"), 3), ssNormal)

	items := []string{"DVI$_DEVDEPEND", "DVI$_REFCNT", "DVI$_PID", "DVI$_OWNUIC", "DVI$_VPROT", "DVI$_DEVNAM"}
	want := []string{long(2), long(2), long(0), long(uic(0o100, 1)), long(0xF000), "_" + m.Device.Name + ":"}

	for _, side := range []struct {
		env *Environment
		ch  uint32
	}{{env, ch}, {groupmate, gch}} {
		got := dviValues(t, side.env, a, side.ch, 0, items...)

		for i := range items {
			if got[i] != want[i] {
				t.Errorf("process %08X's %s = % x, want % x", side.env.Process.PID, items[i], got[i], want[i])
			}
		}
	}

	// By name, from a third process (the world may not use it, but may
	// look at it).
	stranger := otherUser(t, env, uic(0o200, 1), 0)
	if got := dviValues(t, stranger, a, 0, a.desc("PIPE"), "DVI$_DEVDEPEND"); got[0] != long(2) {
		t.Errorf("DEVDEPEND by name = % x, want 2 messages", got[0])
	}
}
