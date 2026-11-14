package rms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// The identities of the probe's accounts (Phase 93, A.4): PROT93A owns the
// directories, PROT93B is in its group, PROT93C in another.
var (
	uicA = security.NewUIC(0o200, 1)
	uicB = security.NewUIC(0o200, 2)
	uicC = security.NewUIC(0o300, 1)
)

// account is the credentials of an account without privileges, as the
// probe's accounts were, with privs added.
func account(u security.UIC, privs uint64) security.Credentials {
	return security.Credentials{UIC: u, Privileges: privs, DefaultProtection: security.DefaultFileProtection}
}

// profileSession is a session on a fresh volume with the directories
// names made by SYSTEM, each then given owner and protection (%X0000,
// every access to everyone, unless set later).
func profileSession(t *testing.T, owner security.UIC, names ...string) (*Session, *volume.Volume) {
	t.Helper()

	s, vol := newCopyTestSession(t)

	for _, n := range names {
		if _, err := s.CreateDirectory("DUA0:["+n+"]", CreateDirectoryOptions{}); err != nil {
			t.Fatalf("CreateDirectory [%s]: %v", n, err)
		}

		changeHeader(t, vol, []string{n}, "", func(h *ondisk.FileHeader) {
			h.Owner = diskUIC(owner)
			h.FileProtection = 0
		})
	}

	return s, vol
}

// openEntry opens name (NAME.TYP, the highest version; "" for the
// directory dirs itself) in dirs.
func openEntry(t *testing.T, vol *volume.Volume, dirs []string, name string) *volume.File {
	t.Helper()

	dir, err := filespec.ResolveDirectory(vol, dirs)
	if err != nil {
		t.Fatalf("ResolveDirectory %v: %v", dirs, err)
	}

	if name == "" {
		return dir.File
	}

	e, err := dir.Lookup(name, 0)
	if err != nil {
		t.Fatalf("Lookup %s: %v", name, err)
	}

	f, err := vol.OpenFID(e.Fid)
	if err != nil {
		t.Fatal(err)
	}

	return f
}

func changeHeader(t *testing.T, vol *volume.Volume, dirs []string, name string, change func(*ondisk.FileHeader)) {
	t.Helper()

	err := volume.UpdateHeader(openEntry(t, vol, dirs, name), func(h *ondisk.FileHeader, _ *ondisk.Ident) { change(h) })
	if err != nil {
		t.Fatal(err)
	}
}

func setACL(t *testing.T, vol *volume.Volume, dirs []string, name string, aces ...ondisk.ACE) {
	t.Helper()

	if err := volume.SetACL(openEntry(t, vol, dirs, name), ondisk.EncodeACL(aces)); err != nil {
		t.Fatal(err)
	}
}

// profileOf is a file's owner, protection, and ACL.
func profileOf(t *testing.T, vol *volume.Volume, dirs []string, name string) (security.UIC, uint16, []ondisk.ACE) {
	t.Helper()

	f := openEntry(t, vol, dirs, name)

	aces, err := f.Header.ACEs()
	if err != nil {
		t.Fatal(err)
	}

	return uicOf(f.Header.Owner), f.Header.FileProtection, aces
}

func makeFile(t *testing.T, s *Session, creds security.Credentials, spec string) {
	t.Helper()

	s.Credentials = func() security.Credentials { return creds }

	if _, err := s.CreateRecordFile(FileLocation{Name: spec}, TextRecords, [][]byte{[]byte("x")}); err != nil {
		t.Fatalf("creating %s: %v", spec, err)
	}
}

// A file with no previous version and no default protection ACE gets the
// process's default protection; its owner is its creator's, or with
// privilege the directory's (A.4: READALL is no such privilege, GRPPRV is
// in its own group).
func TestNewFileFromDirectory(t *testing.T) {
	s, vol := profileSession(t, uicA, "OWN", "OTHER")

	cases := []struct {
		name  string
		creds security.Credentials
		dir   string
		owner security.UIC
	}{
		{"SYSTEM", security.SystemCredentials(), "OWN", uicA},
		{"no privileges", account(uicB, 0), "OWN", uicB},
		{"READALL", account(uicB, security.PrivREADALL), "OWN", uicB},
		{"SYSPRV", account(uicC, security.PrivSYSPRV), "OWN", uicA},
		{"BYPASS", account(uicC, security.PrivBYPASS), "OWN", uicA},
		{"GRPPRV in its group", account(uicB, security.PrivGRPPRV), "OWN", uicA},
		{"GRPPRV outside it", account(uicC, security.PrivGRPPRV), "OWN", uicC},
		{"a system UIC", account(security.NewUIC(0o10, 7), 0), "OWN", uicA},
	}

	for i, tc := range cases {
		name := string(rune('A'+i)) + ".DAT"
		makeFile(t, s, tc.creds, "DUA0:["+tc.dir+"]"+name)

		owner, prot, aces := profileOf(t, vol, []string{tc.dir}, name)
		if owner != tc.owner || prot != uint16(security.DefaultFileProtection) || len(aces) != 0 {
			t.Errorf("%s: owner %v, protection %#x, %d ACEs; want %v, %#x, none",
				tc.name, owner, prot, len(aces), tc.owner, uint16(security.DefaultFileProtection))
		}
	}

	// The process's own default.
	c := account(uicC, 0)
	c.DefaultProtection = 0x1234
	makeFile(t, s, c, "DUA0:[OTHER]D.DAT")

	if _, prot, _ := profileOf(t, vol, []string{"OTHER"}, "D.DAT"); prot != 0x1234 {
		t.Errorf("protection %#x, want the process default %#x", prot, 0x1234)
	}
}

// A new version takes the previous version's protection and its ACL but
// the NOPROPAGATE entries; its owner is the previous version's only with
// privilege (A.4: P1.DAT;2 and ;3, NP_A0.DAT;2).
func TestNewVersionFromPrevious(t *testing.T) {
	s, vol := profileSession(t, uicA, "P")

	makeFile(t, s, security.SystemCredentials(), "DUA0:[P]P1.DAT")

	team := ondisk.Identifier(0x80010001)
	kept := ondisk.NewIdentifierACE([]ondisk.Identifier{team}, 1, 0)
	dropped := ondisk.NewIdentifierACE([]ondisk.Identifier{ondisk.UicIdentifier(diskUIC(uicB))}, 1, ondisk.ACEOptionNoPropagate)
	droppedToo := ondisk.NewIdentifierACE([]ondisk.Identifier{ondisk.UicIdentifier(diskUIC(uicC))}, 1, ondisk.ACEOptionNoPropagate|ondisk.ACEOptionProtected)

	changeHeader(t, vol, []string{"P"}, "P1.DAT", func(h *ondisk.FileHeader) {
		h.Owner = diskUIC(uicC)
		h.FileProtection = 0x1A0F
	})
	setACL(t, vol, []string{"P"}, "P1.DAT", dropped, kept, droppedToo)

	makeFile(t, s, security.SystemCredentials(), "DUA0:[P]P1.DAT")

	owner, prot, aces := profileOf(t, vol, []string{"P"}, "P1.DAT")
	if owner != uicC || prot != 0x1A0F || len(aces) != 1 || aces[0].Identifiers[0] != team {
		t.Errorf("SYSTEM's ;2: owner %v, protection %#x, ACL %+v; want %v, %#x, the team ACE", owner, prot, aces, uicC, 0x1A0F)
	}

	makeFile(t, s, account(uicB, 0), "DUA0:[P]P1.DAT")

	if owner, prot, _ := profileOf(t, vol, []string{"P"}, "P1.DAT"); owner != uicB || prot != 0x1A0F {
		t.Errorf("PROT93B's ;3: owner %v, protection %#x; want %v, %#x", owner, prot, uicB, 0x1A0F)
	}
}

// A directory's default protection ACE gives its files their protection,
// and its DEFAULT entries their ACL, DEFAULT taken off and NOPROPAGATE
// kept (A.4: [.DP]'s files are (RWED,RWED,R,E) with the PROT93B ACE as
// OPTIONS=NOPROPAGATE).
func TestNewFileFromDirectoryDefaults(t *testing.T) {
	s, vol := profileSession(t, uicA, "DP")

	// (S:RWED,O:RWED,G:R,W:E) denies G:WED and W:RWD.
	const dp = 0xBE00

	b := []ondisk.Identifier{ondisk.UicIdentifier(diskUIC(uicB))}
	setACL(t, vol, []string{"DP"}, "",
		ondisk.NewDefaultProtectionACE(dp, 0),
		ondisk.NewIdentifierACE(b, 3, ondisk.ACEOptionDefault|ondisk.ACEOptionNoPropagate),
		ondisk.NewIdentifierACE([]ondisk.Identifier{ondisk.UicIdentifier(diskUIC(uicC))}, 1, 0))

	makeFile(t, s, security.SystemCredentials(), "DUA0:[DP]F.DAT")

	_, prot, aces := profileOf(t, vol, []string{"DP"}, "F.DAT")
	if prot != dp || len(aces) != 1 || aces[0].Flags != ondisk.ACEOptionNoPropagate || aces[0].Identifiers[0] != b[0] || aces[0].Access != 3 {
		t.Errorf("protection %#x, ACL %+v; want %#x and PROT93B's ACE, NOPROPAGATE", prot, aces, dp)
	}
}

// An explicit owner other than the inherited one needs the privilege to
// give it, or nothing is made (A.4, answer 8); an explicit protection is
// applied to the base.
func TestExplicitOwner(t *testing.T) {
	_, vol := profileSession(t, uicA, "X")

	dir, err := filespec.ResolveDirectory(vol, []string{"X"})
	if err != nil {
		t.Fatal(err)
	}

	other := security.NewUIC(0o310, 1)
	merge := func(base security.Protection) security.Protection { return base.With(security.World, security.Read) }

	cases := []struct {
		name  string
		creds security.Credentials
		owner security.UIC
		err   error
	}{
		{"no privileges, another's", account(uicC, 0), other, ErrNoPrivilege},
		{"READALL, another's", account(uicC, security.PrivREADALL), other, ErrNoPrivilege},
		{"GRPPRV, another group's", account(uicB, security.PrivGRPPRV), other, ErrNoPrivilege},
		{"no privileges, its own", account(uicC, 0), uicC, nil},
		{"SYSPRV", account(uicC, security.PrivSYSPRV), other, nil},
		{"SYSTEM", security.SystemCredentials(), other, nil},
	}

	for _, tc := range cases {
		who := creator{creds: tc.creds, explicit: ExplicitProfile{Owner: tc.owner, Protection: merge}}

		p, err := newFileProfile(who, vol, dir, "N.DAT")
		if !errors.Is(err, tc.err) {
			t.Errorf("%s: %v, want %v", tc.name, err, tc.err)

			continue
		}

		if err == nil && (uicOf(*p.Owner) != tc.owner || *p.Protection != 0xEA00) {
			t.Errorf("%s: owner %v, protection %#x; want %v, %#x", tc.name, uicOf(*p.Owner), *p.Protection, tc.owner, 0xEA00)
		}
	}
}

// CREATE/DIRECTORY: the parent's protection and its ACL less NOPROPAGATE
// entries; the owner the creator's, or with privilege the parent's;
// /OWNER_UIC=PARENT under another's directory refused without privilege,
// nothing made (A.4, answers 7 and 8).
func TestNewDirectoryProfile(t *testing.T) {
	s, vol := profileSession(t, uicA, "TOP")

	changeHeader(t, vol, []string{"TOP"}, "", func(h *ondisk.FileHeader) { h.FileProtection = 0x0A00 })
	setACL(t, vol, []string{"TOP"}, "",
		ondisk.NewIdentifierACE([]ondisk.Identifier{ondisk.UicIdentifier(diskUIC(uicB))}, 1, ondisk.ACEOptionNoPropagate),
		ondisk.NewIdentifierACE([]ondisk.Identifier{ondisk.UicIdentifier(diskUIC(uicC))}, 1, ondisk.ACEOptionDefault))

	mk := func(creds security.Credentials, spec string, opts CreateDirectoryOptions) error {
		s.Credentials = func() security.Credentials { return creds }
		_, err := s.CreateDirectory(spec, opts)

		return err
	}

	if err := mk(account(uicC, 0), "DUA0:[TOP.MINE]", CreateDirectoryOptions{}); err != nil {
		t.Fatal(err)
	}

	owner, prot, aces := profileOf(t, vol, []string{"TOP", "MINE"}, "")
	if owner != uicC || prot != 0x0A00|ondisk.ProtectionNoDeleteAll || len(aces) != 1 || aces[0].Flags != ondisk.ACEOptionDefault {
		t.Errorf("[TOP.MINE]: owner %v, protection %#x, ACL %+v", owner, prot, aces)
	}

	if err := mk(security.SystemCredentials(), "DUA0:[TOP.SYS.DEEP]", CreateDirectoryOptions{}); err != nil {
		t.Fatal(err)
	}

	if owner, _, _ := profileOf(t, vol, []string{"TOP", "SYS", "DEEP"}, ""); owner != uicA {
		t.Errorf("SYSTEM's [TOP.SYS.DEEP]: owner %v, want the parent's %v", owner, uicA)
	}

	err := mk(account(uicC, 0), "DUA0:[TOP.PAR.DEEP]", CreateDirectoryOptions{OwnerParent: true})
	if !errors.Is(err, ErrNoPrivilege) {
		t.Errorf("/OWNER_UIC=PARENT under another's: %v, want ErrNoPrivilege", err)
	}

	if _, err := filespec.ResolveDirectory(vol, []string{"TOP", "PAR"}); err == nil {
		t.Error("a refused CREATE/DIRECTORY made [TOP.PAR]")
	}

	mine := diskUIC(uicC)
	if err := mk(account(uicC, 0), "DUA0:[TOP.MINE.OWN]", CreateDirectoryOptions{Owner: &mine}); err != nil {
		t.Errorf("its own UIC: %v", err)
	}
}

// $CREATE with a XABPRO: its protection is the file's whole, and an owner
// the process may not give fails the create with RMS$_PRV and SS$_NOPRIV,
// making nothing (A.4, answer 8).
func TestSysCreateXABPRO(t *testing.T) {
	const xabAddr = 0x3000

	create := func(t *testing.T, f *createFixture, creds security.Credentials, spec string, owner security.UIC) uint32 {
		t.Helper()

		f.ctx.Credentials = func() security.Credentials { return creds }

		newFAB(t, f.ctx, spec)
		putLongwordAt(t, f.ctx, testFabAddr+fabOffset("XAB"), xabAddr)
		putByte(t, f.ctx, xabAddr+xabCOD, xabPRO)
		putByte(t, f.ctx, xabAddr+xabBLN, byte(vmsConst("XAB$C_PROLEN")))
		putWord(t, f.ctx, xabAddr+xabPROw, 0x1234)
		putLongwordAt(t, f.ctx, xabAddr+xabUIC, uint32(owner))

		r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
		if err != nil {
			t.Fatal(err)
		}

		return r0
	}

	other := security.NewUIC(0o310, 1)

	f := newCreateFixture(t, true)
	vol, _ := f.ctx.Mounts.Lookup("DUA0")

	// The accounts may make files in the MFD (subtask 8's checks).
	changeHeader(t, vol, nil, "", func(h *ondisk.FileHeader) { h.FileProtection = 0 })

	if r0 := create(t, f, account(uicC, 0), "DUA0:REFUSED.DAT", other); r0 != rmsPrivilegeViolation {
		t.Errorf("another's owner without privilege: %#x, want RMS$_PRV", r0)
	}

	if stv := readLongword(t, f.ctx, testFabAddr+fabSTV); stv != ssNoPriv {
		t.Errorf("STV %#x, want SS$_NOPRIV", stv)
	}

	if dir, _ := filespec.ResolveDirectory(vol, nil); dir != nil {
		if _, err := dir.Lookup("REFUSED.DAT", 0); err == nil {
			t.Error("a refused $CREATE made REFUSED.DAT")
		}
	}

	for _, tc := range []struct {
		name, spec string
		creds      security.Credentials
		xabOwner   security.UIC
		owner      security.UIC
	}{
		{"SYSTEM, another's", "DUA0:GIVEN.DAT", security.SystemCredentials(), other, other},
		{"its own", "DUA0:OWN.DAT", account(uicC, 0), uicC, uicC},
		{"owner 0 is none given", "DUA0:ZERO.DAT", account(uicC, 0), 0, uicC},
	} {
		if r0 := create(t, f, tc.creds, tc.spec, tc.xabOwner); r0 != rmsNormal {
			t.Errorf("%s: %#x, want RMS$_NORMAL", tc.name, r0)

			continue
		}

		name := tc.spec[len("DUA0:"):]
		if owner, prot, _ := profileOf(t, vol, nil, name); owner != tc.owner || prot != 0x1234 {
			t.Errorf("%s: owner %v, protection %#x; want %v, %#x", tc.name, owner, prot, tc.owner, 0x1234)
		}
	}
}
