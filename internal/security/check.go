package security

import (
	"fmt"
	"strings"
)

// Profile is an object's security profile: what an access check looks at.
type Profile struct {
	Owner      UIC
	Protection Protection
	ACL        []ACE

	// OnVolume says the object is a file or directory on a disk volume
	// owned by VolumeOwner, whose owner is in the SYSTEM category for
	// the volume's files (the Guide to System Security, 4.5.1). The
	// volume's own protection is out of this phase's scope.
	OnVolume    bool
	VolumeOwner UIC

	// Name names the object for SET DEBUG PROTECTION's lines (a file's
	// "NAME.TYP;1"); it decides nothing.
	Name string
}

// Source says what gave a process some of its access.
type Source uint

// The sources, as Decision.Sources bits.
const (
	FromACE       Source = 1 << iota // an identifier ACE
	FromSystem                       // the protection code's SYSTEM field
	FromOwner                        // its OWNER field
	FromGroup                        // its GROUP field
	FromWorld                        // its WORLD field
	FromUnowned                      // an object owned by [0,0] with no identifier ACEs
	FromBYPASS                       // the BYPASS privilege
	FromREADALL                      // the READALL privilege (Check's only)
	FromOwnership                    // control: being the owner or a system user
)

// Decision is the outcome of an access check.
type Decision struct {
	// Granted is whether the process has every access it asked for.
	Granted bool

	// Access is every access the process has to the object, whatever it
	// asked for, and Sources what gave it.
	Access  Access
	Sources Source

	// ACE is the index in the ACL of the identifier ACE that matched, or
	// -1 if none did. A matching ACE that doesn't give the access asked
	// for is a denial, which leaves only the SYSTEM and OWNER fields and
	// privileges to give it.
	ACE int
}

// Check decides whether the process with credentials c may have every
// access in want to an object with profile p, by the rule in the
// package's comment.
//
// READALL answers a request as a whole: it gives a request for read
// alone, and adds nothing to what the ACL and the code give. VMS 7.1
// refused READALL processes FW.DAT, (S:W,O:W,G:W,W:W), for read and
// write, though the code gave them write and READALL read (A.4's matrix,
// TestProbeMatrix).
func Check(c Credentials, p Profile, want Access) Decision {
	d := Evaluate(c, p)
	d.Granted = d.Access&want == want

	if !d.Granted && c.Has(PrivREADALL) && want&^Read == 0 {
		d.Granted = true
		d.Sources |= FromREADALL
	}

	if c.Trace != nil {
		c.Trace(traceLine(c, p, want, d))
	}

	return d
}

// traceLine is SET DEBUG PROTECTION's line for a check: who, what it
// asked for, the object (its name, owner, code, and how many ACEs), and
// the outcome with what gave the access:
//
//	[200,2] wants RW of FR.DAT;1 owner [200,1] (S:RWED,O:RWED,G:R,W:R) ACL 0: denied, has R (group,world)
func traceLine(c Credentials, p Profile, want Access, d Decision) string {
	name := p.Name
	if name == "" {
		name = "object"
	}

	outcome := "denied"
	if d.Granted {
		outcome = "granted"
	}

	return fmt.Sprintf("%v wants %s of %s owner %v %s ACL %d: %s, has %s (%s)",
		c.UIC, accessText(want), name, p.Owner, codeText(p.Protection), len(p.ACL), outcome, accessText(d.Access), sourceText(d))
}

// accessText is a's letters, or "none".
func accessText(a Access) string {
	if s := a.String(); s != "" {
		return s
	}

	return "none"
}

// codeText is p as a protection code, "(S:RWED,O:RWED,G:RE,W)".
func codeText(p Protection) string {
	parts := make([]string, len(categoryNames))

	for c, n := range categoryNames {
		parts[c] = n[:1]
		if letters := p.Allows(Category(c)).String(); letters != "" {
			parts[c] += ":" + letters
		}
	}

	return "(" + strings.Join(parts, ",") + ")"
}

// sourceText names what gave d's access, the matching ACE by its index.
func sourceText(d Decision) string {
	var names []string

	for _, s := range []struct {
		bit  Source
		name string
	}{
		{FromACE, "ACE"}, {FromSystem, "system"}, {FromOwner, "owner"}, {FromGroup, "group"},
		{FromWorld, "world"}, {FromUnowned, "[0,0]"}, {FromBYPASS, "BYPASS"},
		{FromREADALL, "READALL"}, {FromOwnership, "ownership"},
	} {
		if d.Sources&s.bit != 0 {
			name := s.name
			if s.bit == FromACE {
				name = fmt.Sprintf("ACE %d", d.ACE+1)
			}

			names = append(names, name)
		}
	}

	if d.ACE >= 0 && d.Sources&FromACE == 0 {
		names = append(names, fmt.Sprintf("ACE %d denies", d.ACE+1))
	}

	if len(names) == 0 {
		return "nothing"
	}

	return strings.Join(names, ",")
}

// Evaluate finds every access the ACL, the protection code, ownership,
// and BYPASS give the process with credentials c to an object with
// profile p. READALL isn't in it: it is Check's, by the request.
//
// One point is unconfirmed: when a matching ACE gives part of an access
// and the SYSTEM or OWNER field the rest (an ACE of READ, an owner field
// of W, a request for both), the parts add here, as the protection code's
// categories do among themselves.
func Evaluate(c Credentials, p Profile) Decision {
	d := Decision{ACE: -1}

	add := func(a Access, from Source) {
		if a != 0 {
			d.Access |= a
			d.Sources |= from
		}
	}

	// Step 1, the ACL: the first identifier ACE the process matches. The
	// whole list is looked at, for the [0,0] rule below.
	identifierACEs := false

	for i, ace := range p.ACL {
		if !ace.controlsAccess() {
			continue
		}

		identifierACEs = true

		if d.ACE < 0 && c.holdsAll(ace.Identifiers) {
			d.ACE = i
		}
	}

	if d.ACE >= 0 {
		add(p.ACL[d.ACE].Access&All, FromACE)
	}

	owned := p.Owner != 0
	isOwner := owned && c.UIC == p.Owner
	isSystem := c.InSystemGroup() || c.Has(PrivSYSPRV) ||
		(c.Has(PrivGRPPRV) && c.UIC.Group() == p.Owner.Group()) ||
		(p.OnVolume && c.UIC == p.VolumeOwner)

	// Step 2, the protection code. With an ACE matched, only the SYSTEM
	// and OWNER fields are looked at.
	switch {
	case !owned:
		if !identifierACEs {
			add(codeAccess, FromUnowned)
		}
	default:
		if isSystem {
			add(p.Protection.Allows(System), FromSystem)
		}

		if isOwner {
			add(p.Protection.Allows(Owner), FromOwner)
		}

		if d.ACE < 0 {
			if c.UIC.Group() == p.Owner.Group() {
				add(p.Protection.Allows(Group), FromGroup)
			}

			add(p.Protection.Allows(World), FromWorld)
		}
	}

	// Control, which no protection code holds: the owner's and a system
	// user's, but never for an object owned by [0,0] (A.4: VMS 7.1
	// refused SYSTEM the attributes of FUIC0ACL.DAT, which needs read or
	// control).
	if owned && (isOwner || isSystem) {
		add(Control, FromOwnership)
	}

	// Step 3, privileges.
	if c.Has(PrivBYPASS) {
		add(All&^d.Access, FromBYPASS)
	}

	return d
}

// CanChangeOwner reports whether the process with credentials c may make
// newOwner the owner of an object with profile p. For a file, control
// access isn't enough (the Guide to System Security, 4.8.4.2); the
// process must also:
//
//   - hold both the old and the new owner's identifiers (a UIC is held by
//     the process it is, a general identifier with the resource
//     attribute; so an owner may set its own UIC again, but not give a
//     file away);
//   - hold the owning identifier with the resource attribute, and have
//     control through the object's ACL;
//   - be a system user (a system group, SYSPRV), hold BYPASS, or be the
//     volume's owner; or
//   - hold GRPPRV in the object owner's group. VMS 7.1 refused GRPPRV a
//     new owner outside its group (A.4, round 2: PROT93B, [200,2], making
//     a file owned by [310,1]), so the new owner must be in it too.
//
// VMS 7.1 refused an account that had control through an ACE the taking
// of another's file (A.4, round 2), which the first rule gives.
func CanChangeOwner(c Credentials, p Profile, newOwner UIC) bool {
	switch {
	case c.Has(PrivBYPASS), c.Has(PrivSYSPRV), c.InSystemGroup():
		return true
	case p.OnVolume && c.UIC == p.VolumeOwner:
		return true
	case c.Has(PrivGRPPRV) && c.UIC.Group() == p.Owner.Group() && c.UIC.Group() == newOwner.Group():
		return true
	}

	if c.HoldsAsResource(uint32(p.Owner)) && c.HoldsAsResource(uint32(newOwner)) {
		return true
	}

	d := Evaluate(c, p)

	return d.ACE >= 0 && p.ACL[d.ACE].Access&Control != 0 && c.HoldsAsResource(uint32(p.Owner))
}

// InheritsOwner reports whether a file or directory the process creates
// takes owner, its previous version's or its directory's owner, rather
// than the process's own UIC. VMS gives a new file the creator's UIC
// unless the creator has privilege over the files it makes: a system
// user (a system group or SYSPRV), BYPASS, or GRPPRV when owner is in its
// own group. READALL is not such a privilege. These are VMS 7.1's answers
// (Phase 93's probe, A.4's "New files' profiles": P1.DAT;2 made by SYSTEM
// without privileges kept [300,1]; PROT93B with READALL owned its files;
// with GRPPRV, its files in group 200's directories were [200,1]'s).
func InheritsOwner(c Credentials, owner UIC) bool {
	switch {
	case c.Has(PrivBYPASS), c.Has(PrivSYSPRV), c.InSystemGroup():
		return true
	case c.Has(PrivGRPPRV):
		return c.UIC.Group() == owner.Group()
	}

	return false
}
