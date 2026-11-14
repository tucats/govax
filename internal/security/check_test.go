package security

import (
	"testing"
)

// Cases from the Guide to System Security (A.1a) and the probe's answers
// (A.4) that TestProbeMatrix's [PROT93.F] matrix doesn't reach.

func TestProtectionFields(t *testing.T) {
	// %XFA00, RMS_FILEPROT's default: (S:RWED,O:RWED,G:RE,W).
	p := Protection(0xFA00)

	for _, c := range []struct {
		cat  Category
		want string
	}{{System, "RWED"}, {Owner, "RWED"}, {Group, "RE"}, {World, ""}} {
		if got := p.Allows(c.cat).String(); got != c.want {
			t.Errorf("category %d: %q, want %q", c.cat, got, c.want)
		}
	}

	if got := NoAccess.With(System, Read|Write|Execute|Delete).With(Owner, All).With(Group, Read|Execute); got != 0xFA00 {
		t.Errorf("With: %#04x, want 0xfa00", uint16(got))
	}

	// The probe's $DISPLAY of FROT2.DAT, (S:W,O:E,G:D,W:R): %XE7BD.
	if got := NoAccess.With(System, Write).With(Owner, Execute).With(Group, Delete).With(World, Read); got != 0xE7BD {
		t.Errorf("FROT2: %#04x, want 0xe7bd", uint16(got))
	}
}

func TestUICs(t *testing.T) {
	if got := NewUIC(0o200, 1); uint32(got) != 0x00800001 || got.String() != "[200,1]" {
		t.Errorf("NewUIC: %#08x %s", uint32(got), got)
	}

	c := Credentials{UIC: uicB}
	for _, h := range []struct {
		id   uint32
		want bool
	}{
		{0x00800002, true},  // [200,2]
		{0x00800001, false}, // [200,1]
		{0x0080FFFF, true},  // [200,*]
		{0x00C0FFFF, false}, // [300,*]
		{0x3FFFFFFF, true},  // [*,*]
		{idTeam, false},
	} {
		if got := c.Holds(h.id); got != h.want {
			t.Errorf("[200,2] holds %#08x: %v", h.id, got)
		}
	}
}

// can reports whether c may have want to p.
func can(c Credentials, p Profile, want Access) bool { return Check(c, p, want).Granted }

func TestCheckCases(t *testing.T) {
	m0 := probeIdentity("M0")
	a0 := probeIdentity("A0")
	b0 := probeIdentity("B0")
	c0 := probeIdentity("C0")
	files := Profile{Owner: uicA, Protection: 0xFFFF}

	cases := []struct {
		name string
		c    Credentials
		p    Profile
		want Access
		ok   bool
	}{
		// FACLCTL: an ACE giving PROT93C control alone, the world field
		// later opened to RE. The ACE decides: PROT93C may not read.
		{"FACLCTL read", c0, Profile{Owner: uicA, Protection: NoAccess.With(World, Read|Execute), ACL: grant(uint32(uicC), Control)}, Read, false},
		{"FACLCTL control", c0, Profile{Owner: uicA, Protection: NoAccess, ACL: grant(uint32(uicC), Control)}, Control, true},
		{"FACLCTL others read", b0, Profile{Owner: uicA, Protection: NoAccess.With(World, Read|Execute), ACL: grant(uint32(uicC), Control)}, Read, true},

		// Control (A.4): the system category's and the owner's, never
		// the group's from the code.
		{"system controls another's file", m0, files, Control, true},
		{"owner controls its file", a0, files, Control, true},
		{"group RWED gives no control", b0, Profile{Owner: uicA, Protection: NoAccess.With(Group, All)}, Control, false},
		{"group RWED gives delete", b0, Profile{Owner: uicA, Protection: NoAccess.With(Group, All)}, Delete, true},
		{"world gives no control", c0, Profile{Owner: uicA}, Control, false},
		{"GRPPRV controls its group's files", withPriv(b0, PrivGRPPRV), files, Control, true},
		{"GRPPRV controls nothing elsewhere", withPriv(c0, PrivGRPPRV), files, Control, false},
		{"SYSPRV controls", withPriv(c0, PrivSYSPRV), files, Control, true},
		{"READALL gives no control", withPriv(c0, PrivREADALL), files, Control, false},
		{"BYPASS controls", withPriv(c0, PrivBYPASS), files, Control, true},

		// [0,0]: everything but control with no ACL; with one, only it.
		{"UIC 0 write", c0, Profile{Protection: NoAccess}, Read | Write | Execute | Delete, true},
		{"UIC 0 control", m0, Profile{Protection: NoAccess}, Control, false},
		{"UIC 0 ACL reader", b0, Profile{Protection: NoAccess, ACL: grant(uint32(uicB), Read)}, Read, true},
		{"UIC 0 ACL others", m0, Profile{Protection: NoAccess, ACL: grant(uint32(uicB), Read)}, Read, false},
		{"UIC 0 ACL no control", m0, Profile{Protection: NoAccess, ACL: grant(uint32(uicB), Read)}, Control, false},
		{"UIC 0 ACL BYPASS", withPriv(m0, PrivBYPASS), Profile{Protection: NoAccess, ACL: grant(uint32(uicB), Read)}, All, true},

		// A DEFAULT ACE is ignored in the check: the code decides, and a
		// [0,0] object with only DEFAULT ACEs has none that count.
		{"DEFAULT ACE ignored", b0, Profile{Owner: uicA, Protection: NoAccess.With(World, Read), ACL: []ACE{NewIdentifierACE([]uint32{uint32(uicB)}, 0, OptionDefault)}}, Read, true},
		{"DEFAULT ACE on UIC 0", b0, Profile{Protection: NoAccess, ACL: []ACE{NewIdentifierACE([]uint32{uint32(uicB)}, 0, OptionDefault)}}, Write, true},

		// Several identifiers in one ACE: all must be held.
		{"ACE of two, both held", c0, Profile{Owner: uicA, Protection: NoAccess, ACL: []ACE{NewIdentifierACE([]uint32{uint32(uicC), idTeam}, Read, 0)}}, Read, true},
		{"ACE of two, one held", b0, Profile{Owner: uicA, Protection: NoAccess, ACL: []ACE{NewIdentifierACE([]uint32{uint32(uicB), idTeam}, Read, 0)}}, Read, false},

		// Only the first matching ACE counts.
		{"later ACE not reached", c0, Profile{Owner: uicA, Protection: NoAccess, ACL: append(grant(idTeam, Read), grant(uint32(uicC), Write)...)}, Write, false},

		// After a denial the SYSTEM and OWNER fields still give access,
		// the GROUP and WORLD fields don't.
		{"denied, owner field", a0, Profile{Owner: uicA, Protection: NoAccess.With(Owner, Read), ACL: grant(uint32(uicA), 0)}, Read, true},
		{"denied, world field", b0, Profile{Owner: uicA, Protection: 0, ACL: grant(uint32(uicB), 0)}, Read, false},
		{"denied, SYSPRV", withPriv(b0, PrivSYSPRV), Profile{Owner: uicA, Protection: 0, ACL: grant(uint32(uicB), 0)}, Write, true},
		{"denied, READALL reads", withPriv(b0, PrivREADALL), Profile{Owner: uicA, Protection: 0, ACL: grant(uint32(uicB), 0)}, Read, true},
		{"denied, READALL writes", withPriv(b0, PrivREADALL), Profile{Owner: uicA, Protection: 0, ACL: grant(uint32(uicB), 0)}, Write, false},

		// The volume's owner is in the SYSTEM category for its files.
		{"volume owner", c0, Profile{Owner: uicA, Protection: NoAccess.With(System, Read), OnVolume: true, VolumeOwner: uicC}, Read | Control, true},
		{"not the volume owner", b0, Profile{Owner: uicA, Protection: NoAccess.With(System, Read), OnVolume: true, VolumeOwner: uicC}, Read, false},

		// MAXSYSGROUP.
		{"group 10 is a system group", Credentials{UIC: NewUIC(0o10, 1)}, Profile{Owner: uicA, Protection: NoAccess.With(System, Read)}, Read, true},
		{"group 11 isn't", Credentials{UIC: NewUIC(0o11, 1)}, Profile{Owner: uicA, Protection: NoAccess.With(System, Read)}, Read, false},
		{"MAXSYSGROUP 20", Credentials{UIC: NewUIC(0o11, 1), MaxSysGroup: 0o20}, Profile{Owner: uicA, Protection: NoAccess.With(System, Read)}, Read, true},
	}

	for _, c := range cases {
		if got := can(c.c, c.p, c.want); got != c.ok {
			t.Errorf("%s: %v, want %v", c.name, got, c.ok)
		}
	}
}

// withPriv is c with privileges p.
func withPriv(c Credentials, p uint64) Credentials {
	c.Privileges = p

	return c
}

func TestDecisionSources(t *testing.T) {
	d := Check(probeIdentity("B0"), Profile{Owner: uicA, Protection: NoAccess.With(Group, Read).With(World, Execute)}, Read)
	if !d.Granted || d.Sources != FromGroup|FromWorld || d.ACE != -1 || d.Access != Read|Execute {
		t.Errorf("group and world: %+v", d)
	}

	d = Check(probeIdentity("C0"), Profile{Owner: uicA, Protection: NoAccess, ACL: append(grant(uint32(uicB), Read), grant(idTeam, Write)...)}, Write)
	if !d.Granted || d.Sources != FromACE || d.ACE != 1 {
		t.Errorf("the second ACE: %+v", d)
	}

	d = Check(probeIdentity("C1"), Profile{Owner: uicA, Protection: NoAccess}, Read)
	if !d.Granted || d.Sources != FromREADALL || d.Access != 0 {
		t.Errorf("READALL: %+v", d)
	}
}

// ownerCase is a case of TestCanChangeOwner.
type ownerCase struct {
	name  string
	c     Credentials
	p     Profile
	owner UIC
	ok    bool
}

func TestCanChangeOwner(t *testing.T) {
	m0 := probeIdentity("M0")
	a0 := probeIdentity("A0")
	b0 := probeIdentity("B0")
	c0 := probeIdentity("C0")
	aFile := Profile{Owner: uicA, Protection: NoAccess}
	ctlFile := Profile{Owner: uicA, Protection: NoAccess, ACL: grant(uint32(uicC), Control)}

	cases := []ownerCase{
		{"SYSTEM, any owner", m0, aFile, NewUIC(0o310, 1), true},
		{"owner, its own UIC again", a0, aFile, uicA, true},
		{"owner gives it away", a0, aFile, uicB, false},
		{"ACE control doesn't take a file", c0, ctlFile, uicC, false},
		{"SYSPRV", withPriv(c0, PrivSYSPRV), aFile, uicC, true},
		{"BYPASS", withPriv(c0, PrivBYPASS), aFile, uicC, true},
		{"READALL", withPriv(c0, PrivREADALL), aFile, uicC, false},
		{"GRPPRV in the group", withPriv(b0, PrivGRPPRV), aFile, uicB, true},
		{"GRPPRV to another group", withPriv(b0, PrivGRPPRV), aFile, NewUIC(0o310, 1), false},
		{"GRPPRV on another group's file", withPriv(c0, PrivGRPPRV), aFile, uicC, false},
		{"volume owner", c0, Profile{Owner: uicA, OnVolume: true, VolumeOwner: uicC}, uicB, true},
	}

	// A general identifier with the resource attribute: its holder may
	// change the owner between identifiers it holds as resources, or,
	// with control through the ACL, to anything.
	const idProject = 0x80010002

	resource := withPriv(c0, 0)
	resource.Rights = append(resource.Rights, Identifier{Value: idProject, Attributes: AttributeResource})
	projectFile := Profile{Owner: UIC(idProject), Protection: NoAccess}
	cases = append(cases,
		ownerCase{"resource to its UIC", resource, projectFile, uicC, true},
		ownerCase{"resource to another", resource, projectFile, uicB, false},
		ownerCase{"resource with ACL control", resource, Profile{Owner: UIC(idProject), ACL: grant(uint32(uicC), Control)}, uicB, true},
	)

	for _, c := range cases {
		if got := CanChangeOwner(c.c, c.p, c.owner); got != c.ok {
			t.Errorf("%s: %v, want %v", c.name, got, c.ok)
		}
	}
}
