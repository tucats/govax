package security

// A third party's access check: the services $CHKPRO and $CHECK_ACCESS
// (docs/planning/PHASE-93 - file protection.md, A.4's "Round 3"), which
// decide for someone else, not the calling process, and also say which
// privileges the access needed.
//
// They follow the same sequence as Check (the ACL's first matching
// identifier ACE, then the protection code's categories, then the
// privileges), in one of two ways of reading the access bits:
//
//   - Interpreted, $CHECK_ACCESS's: the bits mean what they mean for a
//     file, as Check reads them. Control comes only from being the owner
//     or a system user, never from the protection code.
//   - Compared, $CHKPRO's: the bits mean nothing to the service. A
//     category's field of the protection code denies the access bits set
//     in it, so it can deny only the low four bits (read, write, execute,
//     delete), and every other bit asked for is granted with the category.
//     An identifier ACE grants the bits its access holds. VMS 7.1 granted
//     the group CONTROL with a protection code of one word (the probe's
//     case 252), and with a second word denying it too (254): the VSI
//     manual says the second word's fields hold the next four bits, but
//     VMS 7.1 didn't look at them, so govax takes only the first word.

// Rules says how a third party's check reads the access bits.
type Rules int

const (
	// Interpreted reads them as a file's (Check's rules).
	Interpreted Rules = iota

	// Compared compares them with the protection code's fields bit by
	// bit, $CHKPRO's way.
	Compared
)

// Used is the privileges a check needed, as $CHKPRO's and $CHECK_ACCESS's
// CHP$_PRIVUSED report them.
type Used uint

// The privileges a check can need.
const (
	// UsedSYSPRV: the SYSTEM category of the protection code gave the
	// access, which the process is in by holding SYSPRV or by its UIC
	// being in a system group. VMS 7.1 reported SYSPRV for both (the
	// probe's case 503: SYSTEM, [1,4], with no privileges, granted the
	// world's file through the SYSTEM field).
	UsedSYSPRV Used = 1 << iota

	// UsedGRPPRV: the SYSTEM category gave the access through GRPPRV,
	// the process being in the owner's group.
	UsedGRPPRV

	// UsedBYPASS: BYPASS gave it.
	UsedBYPASS

	// UsedREADALL: READALL gave it.
	UsedREADALL
)

// Outcome is a third party's check's answer.
type Outcome struct {
	Granted bool

	// ACE is the index in the ACL of the identifier ACE that matched,
	// granting or denying, or -1.
	ACE int

	// Used is the privileges the access needed: none if it was granted
	// without them, and none if it was refused.
	Used Used
}

// ThirdParty decides whether the accessor with credentials c may have
// every access in want to an object with profile p, by rules. With
// useReadAll, READALL may grant a request for read alone; Interpreted
// rules always allow it, as Check does, and Compared rules only when the
// caller says so ($CHKPRO's CHP$M_USEREADALL: VMS 7.1 refused READALL a
// world read without it, the probe's case 246).
//
// The privileges are tried in steps, and Used names the step that
// granted the access: first what the accessor has without privileges
// (the ACL, its own categories, and, for a [0,0] owner with no
// identifier ACEs, everything), then the SYSTEM category, then BYPASS,
// then READALL.
func ThirdParty(c Credentials, p Profile, want Access, rules Rules, useReadAll bool) Outcome {
	o := Outcome{ACE: -1}

	// The ACL: the first identifier ACE the accessor matches.
	identifierACEs := false

	for i, ace := range p.ACL {
		if !ace.controlsAccess() {
			continue
		}

		identifierACEs = true

		if o.ACE < 0 && c.holdsAll(ace.Identifiers) {
			o.ACE = i
		}
	}

	var have Access

	if o.ACE >= 0 {
		have = p.ACL[o.ACE].Access
		if rules == Interpreted {
			have &= All
		}
	}

	// field is what a category of the protection code gives.
	field := func(cat Category) Access {
		if rules == Interpreted {
			return p.Protection.Allows(cat)
		}

		return ^Access(0) &^ Access(uint32(p.Protection)>>(4*uint(cat))&0xF)
	}

	owned := p.Owner != 0
	isOwner := owned && c.UIC == p.Owner

	// Step 1, without privileges.
	switch {
	case !owned:
		if !identifierACEs {
			if rules == Interpreted {
				have |= codeAccess
			} else {
				have = ^Access(0)
			}
		}
	default:
		if isOwner {
			have |= field(Owner)

			if rules == Interpreted {
				have |= Control
			}
		}

		if o.ACE < 0 {
			if c.UIC.Group() == p.Owner.Group() {
				have |= field(Group)
			}

			have |= field(World)
		}
	}

	granted := func() bool { return have&want == want }

	if granted() {
		o.Granted = true

		return o
	}

	// Step 2, the SYSTEM category.
	if owned {
		var by Used

		switch {
		case c.InSystemGroup(), c.Has(PrivSYSPRV), p.OnVolume && c.UIC == p.VolumeOwner:
			by = UsedSYSPRV
		case c.Has(PrivGRPPRV) && c.UIC.Group() == p.Owner.Group():
			by = UsedGRPPRV
		}

		if by != 0 {
			have |= field(System)

			if rules == Interpreted {
				have |= Control
			}

			if granted() {
				o.Granted, o.Used = true, by

				return o
			}
		}
	}

	// Step 3, BYPASS; step 4, READALL for a read.
	switch {
	case c.Has(PrivBYPASS):
		o.Granted, o.Used = true, UsedBYPASS
	case c.Has(PrivREADALL) && want&^Read == 0 && (rules == Interpreted || useReadAll):
		o.Granted, o.Used = true, UsedREADALL
	}

	return o
}
