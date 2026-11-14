package rms

import (
	"errors"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is where a new file's or directory's security profile is
// chosen: its owner UIC, its protection code, and its access control list
// (docs/planning/PHASE-93 - file protection.md, Decision 4). Every way
// govax makes a file on a volume comes here: RMS's $CREATE, the ACP's
// IO$_CREATE, DCL's OPEN/WRITE, COPY, CREATE, EDIT, the record files the
// utilities write (LIBRARY, MACRO, LINK, ...), BACKUP's save sets and
// restores, the page file, and CREATE/DIRECTORY.
//
// # A note for a reader new to VMS
//
// Every file on a VMS disk has an owner (a UIC, "[group,member]"), a
// protection code saying what each of four categories of user (SYSTEM,
// OWNER, GROUP, WORLD) may do to it, and possibly an access control list,
// a list of entries (ACEs) granting or denying access to the holders of
// particular identifiers. A program creating a file rarely says what these
// should be; VMS works them out, in this order:
//
//   - The protection is the previous version's (the highest version of the
//     same name already in the directory), else what the directory's
//     default protection ACE says, else the process's default protection
//     (SET PROTECTION/DEFAULT). A protection the creator gives (a XABPRO,
//     /PROTECTION) is laid over it.
//   - The ACL is the previous version's, less its NOPROPAGATE entries, else
//     the directory's entries marked DEFAULT, with DEFAULT taken off.
//   - The owner is the creator's UIC, unless the creator has privilege
//     over what it makes (security.InheritsOwner): then the previous
//     version's owner, else the directory's. An owner the creator names
//     must be one it may give (security.CanChangeOwner), or nothing is
//     made: ErrNoPrivilege.
//
// A new directory takes its parent's protection less delete access, its
// parent's ACL less NOPROPAGATE entries (DEFAULT ones keeping DEFAULT, so
// they reach the files made there), and its owner by the same rule. VMS
// 7.1 confirmed each rule (the phase's probe, A.4).

// ErrNoPrivilege is an access the process's credentials don't give: RMS's
// RMS$_PRV with an STV of SS$_NOPRIV, and the ACP's SS$_NOPRIV.
var ErrNoPrivilege = errors.New("rms: insufficient privilege or object protection violation")

// ExplicitProfile is what the creator of a file asks for of its profile,
// over what VMS's rules would give it.
type ExplicitProfile struct {
	// Owner is the owner the creator names (a XABPRO's XAB$L_UIC,
	// /OWNER_UIC); 0 for none, as VMS takes a XABPRO's owner of 0.
	Owner security.UIC

	// Protection, if not nil, makes the file's protection from the one
	// VMS's rules would give it, base: a XABPRO replaces it whole
	// (WholeProtection), and a DCL /PROTECTION changes only the
	// categories it names.
	Protection func(base security.Protection) security.Protection
}

// WholeProtection is an ExplicitProfile.Protection giving p whatever the
// base: a XABPRO's protection word, which names every category.
func WholeProtection(p uint16) func(security.Protection) security.Protection {
	return func(security.Protection) security.Protection { return security.Protection(p) }
}

// creator is who is making a file and what they asked for its profile.
type creator struct {
	creds    security.Credentials
	explicit ExplicitProfile
}

// creator returns the session's process as a file's creator, asking for
// explicit.
func (s *Session) creator(explicit ExplicitProfile) creator {
	return creator{creds: s.Identity(), explicit: explicit}
}

// creator returns the calling process as a file's creator, asking for
// explicit.
func (c *Context) creator(explicit ExplicitProfile) creator {
	return creator{creds: c.Identity(), explicit: explicit}
}

// newFileProfile is the profile of a file named name (NAME.TYP, no
// version) that who is creating in dir on vol, by the rules in this file's
// opening comment. dir may be nil, for a file with no directory entry (the
// ACP's IO$_CREATE without a directory): then nothing is inherited. A
// refused explicit owner is ErrNoPrivilege.
func newFileProfile(who creator, vol *volume.Volume, dir *volume.Directory, name string) (*volume.NewFileProfile, error) {
	creds := who.creds
	protection := creds.DefaultProtection
	owner := creds.UIC

	var acl []byte

	// The previous version, if there is one; else the directory.
	var previous *ondisk.FileHeader

	if dir != nil {
		if e, err := dir.Lookup(name, 0); err == nil {
			if f, err := vol.OpenFID(e.Fid); err == nil {
				previous = &f.Header
			}
		}
	}

	switch {
	case previous != nil:
		protection = security.Protection(previous.FileProtection)

		if raw, err := previous.ACL(); err == nil {
			acl, _ = ondisk.ACLForNewVersion(raw)
		}

		if security.InheritsOwner(creds, uicOf(previous.Owner)) {
			owner = uicOf(previous.Owner)
		}

	case dir != nil:
		if p, ok := directoryDefaultProtection(dir); ok {
			protection = p
		}

		if raw, err := dir.Header.ACL(); err == nil {
			acl, _ = ondisk.ACLFromDirectoryDefaults(raw)
		}

		if security.InheritsOwner(creds, uicOf(dir.Header.Owner)) {
			owner = uicOf(dir.Header.Owner)
		}
	}

	if who.explicit.Protection != nil {
		protection = who.explicit.Protection(protection)
	}

	owner, err := explicitOwner(who, vol, owner, protection, acl)
	if err != nil {
		return nil, err
	}

	mask := uint16(protection)
	disk := diskUIC(owner)

	return &volume.NewFileProfile{Owner: &disk, Protection: &mask, ACL: acl}, nil
}

// newDirectoryProfile is the options of a directory who is creating in
// parent: what it inherits (volume.InheritedDirectoryOptions: the
// parent's protection less delete, its ACL less NOPROPAGATE entries, its
// version limit), the owner by this file's rule, and who's explicit
// protection and owner over them. A refused explicit owner is
// ErrNoPrivilege.
func newDirectoryProfile(who creator, vol *volume.Volume, parent *volume.Directory) (volume.DirectoryOptions, error) {
	d := volume.InheritedDirectoryOptions(parent)

	owner := who.creds.UIC
	if security.InheritsOwner(who.creds, uicOf(parent.Header.Owner)) {
		owner = uicOf(parent.Header.Owner)
	}

	protection := security.Protection(*d.Protection)
	if who.explicit.Protection != nil {
		protection = who.explicit.Protection(protection)
	}

	owner, err := explicitOwner(who, vol, owner, protection, d.ACL)
	if err != nil {
		return d, err
	}

	mask := uint16(protection)
	disk := diskUIC(owner)
	d.Owner, d.Protection = &disk, &mask

	return d, nil
}

// explicitOwner is the owner a new object gets: who's explicit owner, if
// it names one who may give the object (security.CanChangeOwner, the
// object's profile being what it would have without it: inherited, with
// protection and acl), else inherited. A refused one is ErrNoPrivilege
// (VMS 7.1: RMS$_PRV and SS$_NOPRIV, nothing made; the probe's A.4,
// answer 8).
func explicitOwner(who creator, vol *volume.Volume, inherited security.UIC, protection security.Protection, acl []byte) (security.UIC, error) {
	want := who.explicit.Owner
	if want == 0 || want == inherited {
		return inherited, nil
	}

	aces, _ := security.DecodeACL(acl)

	profile := security.Profile{Owner: inherited, Protection: protection, ACL: aces, OnVolume: true}
	if vol != nil && len(vol.Devices) > 0 {
		profile.VolumeOwner = uicOf(vol.Devices[0].Home.VolumeOwner)
	}

	if !security.CanChangeOwner(who.creds, profile, want) {
		return 0, ErrNoPrivilege
	}

	return want, nil
}

// directoryDefaultProtection is the protection dir's default protection
// ACE gives the files made in it, if it has one (the first, if several).
func directoryDefaultProtection(dir *volume.Directory) (security.Protection, bool) {
	aces, err := dir.Header.ACEs()
	if err != nil {
		return 0, false
	}

	for _, a := range aces {
		if a.Type == ondisk.ACETypeDefaultProtection {
			return security.Protection(a.DefaultProtection), true
		}
	}

	return 0, false
}

// uicOf is an ods2 UIC as security's.
func uicOf(u ondisk.Uic) security.UIC {
	return security.NewUIC(uint32(u.Group), uint32(u.Member))
}

// diskUIC is security's UIC as ods2's.
func diskUIC(u security.UIC) ondisk.Uic {
	return ondisk.Uic{Group: uint16(u.Group()), Member: uint16(u.Member())}
}
