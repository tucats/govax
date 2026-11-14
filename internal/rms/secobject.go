package rms

import (
	"errors"
	"strings"

	"github.com/tucats/ods2/filespec"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/ods2/volume"
)

// A file's security profile as the security services reach it
// (docs/planning/PHASE-93 - file protection.md, subtasks 10 and 13):
// $CHANGE_ACL now, $GET_SECURITY and $SET_SECURITY later, find a file by
// name or by a channel it is accessed on, read its owner, protection, and
// ACL, and change them. SecurityObject is that file, in the checked
// layer: it says what its process may do (MayRead, MayChange) and changes
// only what the service has already been told it may.

// SecurityObject is one file whose profile a security service reads or
// changes, for the process with credentials creds.
type SecurityObject struct {
	vol   *volume.Volume
	file  *volume.File
	creds security.Credentials

	// writable is whether the profile may be written: the volume is
	// mounted writable and, for a file reached through a channel, the
	// channel's access is for writing (VMS 7.1 refused $CHANGE_ACL a
	// change through a channel accessed for reading, A.4).
	writable bool

	// Name is the file's name, for tracing and messages.
	Name string

	// parent is the directory the file was found in, when it was found
	// by name (SecurityObjects): a directory's default protection is its
	// parent's, less delete.
	parent *volume.File
}

// ssInvFilForOp is SS$_INVFILFOROP, a name with a wildcard given a
// service that takes one file; ssUnsupported SS$_UNSUPPORTED, a host
// file, which has no profile (Decision 12).
var (
	ssInvFilForOp = vmsdef.Symbols["SS$_INVFILFOROP"]
	ssUnsupported = vmsdef.Symbols["SS$_UNSUPPORTED"]
)

// FindSecurityObject finds the file spec names for a security service
// of the session's process, as VMS 7.1's $CHANGE_ACL found it (A.4): by
// RMS's name rules (logical names, the default device and directory, no
// default type: SEC93$CHG:CA6 is CA6. and RMS$_FNF), the directories
// looked in with the process's lookup access. A failure is a
// *DCLFileError with the status VMS gave: RMS$_FNF or RMS$_DNF for a file
// or directory that isn't there, SS$_INVFILFOROP for a wildcard, RMS$_DEV
// for a device that isn't mounted, SS$_UNSUPPORTED for a host file; or a
// *DeniedError for a directory the process may not look in (unconfirmed:
// the probe gave the services no such name).
func (s *Session) FindSecurityObject(spec string) (*SecurityObject, error) {
	loc, err := s.Locate(spec, false)
	if err != nil {
		return nil, newDCLFileError(strings.ToUpper(spec), rmsFileNotFound)
	}

	if loc.Host {
		return nil, newDCLFileError(loc.Name, ssUnsupported)
	}

	if strings.ContainsAny(spec, "*%") || strings.Contains(spec, "...") {
		return nil, newDCLFileError(strings.ToUpper(spec), ssInvFilForOp)
	}

	var found *SecurityObject

	err = s.firstSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := s.globChecked(vol, r.Spec)
		if _, denied := IsDenied(err); denied {
			return err
		}

		if err != nil {
			return &NotFoundError{Spec: spec}
		}

		for _, m := range matches {
			if (r.Spec.Name == "" && m.Name != "") || (r.Spec.Type == "" && m.Type != "") {
				continue
			}

			f, err := vol.OpenFID(m.Fid)
			if err != nil {
				return err
			}

			found = &SecurityObject{
				vol: vol, file: f, creds: s.Identity(),
				writable: s.Mounts.Writable(r.Spec.Device),
				Name:     r.shownFile(m),
			}

			return nil
		}

		return &NotFoundError{Spec: spec}
	})

	var (
		notFound   *NotFoundError
		notMounted *NotMountedError
	)

	switch {
	case found != nil:
		return found, nil
	case errors.As(err, &notMounted):
		return nil, newDCLFileError(strings.ToUpper(spec), rmsDeviceError)
	case err == nil || errors.As(err, &notFound):
		return nil, s.NotFound(spec)
	}

	return nil, err
}

// SecurityObjects finds the files spec names for DCL's SET SECURITY and
// its obsolete forms (docs/planning/PHASE-93 - file protection.md,
// subtask 11): wildcards allowed, every element of a search list, a name
// with no version its highest version, the directories searched with the
// process's access (globChecked). The objects are in the order DIRECTORY
// lists the files. No file at all is a *DCLFileError, RMS$_FNF or
// RMS$_DNF (with SS$_NOSUCHFILE) for the expanded name, as VMS 7.1's SET
// SECURITY reported it; a directory the process may not search is a
// *DeniedError; a host file SS$_UNSUPPORTED (Decision 12: a host file has
// no profile).
func (s *Session) SecurityObjects(spec string) ([]*SecurityObject, error) {
	loc, err := s.Locate(spec, false)
	if err != nil {
		return nil, newDCLFileError(strings.ToUpper(spec), rmsFileNotFound)
	}

	if loc.Host {
		return nil, newDCLFileError(loc.Name, ssUnsupported)
	}

	var found []*SecurityObject

	err = s.eachSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := s.globChecked(vol, r.Spec)
		if _, denied := IsDenied(err); denied {
			return err
		}

		if err != nil {
			return &NotFoundError{Spec: spec}
		}

		if r.Spec.Version == "" {
			matches = highestVersions(matches)
		}

		before := len(found)
		writable := s.Mounts.Writable(r.Spec.Device)
		parents := map[string]*volume.File{}

		for _, m := range matches {
			f, err := vol.OpenFID(m.Fid)
			if err != nil {
				return err
			}

			key := strings.Join(m.Dirs, ".")

			parent, ok := parents[key]
			if !ok {
				if dir, err := filespec.ResolveDirectory(vol, m.Dirs); err == nil {
					parent = dir.File
				}

				parents[key] = parent
			}

			found = append(found, &SecurityObject{
				vol: vol, file: f, creds: s.Identity(), writable: writable,
				Name: r.shownFile(m), parent: parent,
			})
		}

		if len(found) == before {
			return &NotFoundError{Spec: spec}
		}

		return nil
	})

	var notMounted *NotMountedError

	switch {
	case len(found) > 0 && err == nil:
		return found, nil
	case errors.As(err, &notMounted):
		return nil, newDCLFileError(strings.ToUpper(spec), rmsDeviceError)
	case err == nil || isNotFound(err):
		return nil, s.NotFound(spec)
	}

	return nil, err
}

// highestVersions is matches with only the highest version of each name,
// what a specification with no version names.
func highestVersions(matches []filespec.Match) []filespec.Match {
	highest := map[string]int{}

	var out []filespec.Match

	for _, m := range matches {
		key := strings.Join(m.Dirs, ".") + "]" + m.Name + "." + m.Type

		i, ok := highest[key]
		switch {
		case !ok:
			highest[key] = len(out)
			out = append(out, m)
		case m.Version > out[i].Version:
			out[i] = m
		}
	}

	return out
}

// IsDirectory reports whether the file is a directory.
func (o *SecurityObject) IsDirectory() bool { return o.file.Header.IsDirectory() }

// DirectoryDefault is the protection SET SECURITY/DEFAULT gives a
// directory: the one a new directory in its parent would get, the
// parent's less delete (A.4, round 2: SD1.DIR under SEC.DIR's
// (RWE,RWE,RWE,RWE)). ok is false when the parent isn't known.
func (o *SecurityObject) DirectoryDefault() (p security.Protection, ok bool) {
	if o.parent == nil {
		return 0, false
	}

	return headerProfile(o.vol, &o.parent.Header).Protection.Without(security.Delete), true
}

// SecurityObject is the file accessed through a, for the process with
// credentials creds.
func (a *ACPFile) SecurityObject(creds security.Credentials) *SecurityObject {
	o := &SecurityObject{file: a.file, creds: creds, writable: a.writable}

	if a.mount != nil {
		o.vol = a.mount.Volume
		o.writable = o.writable && a.mount.Writable
	}

	if id, err := a.file.Header.Ident(); err == nil {
		o.Name = strings.TrimRight(id.Filename+id.FilenameExtension, " ")
	}

	return o
}

// Profile is the file's owner, protection code, and ACL.
func (o *SecurityObject) Profile() security.Profile {
	p := headerProfile(o.vol, &o.file.Header)
	if p.Name == "" {
		p.Name = o.Name
	}

	return p
}

// ACL is the file's access control list as its header holds it, its
// entries one after another.
func (o *SecurityObject) ACL() []byte {
	acl, err := o.file.Header.ACL()
	if err != nil {
		return nil
	}

	return acl
}

// MayRead reports whether the process may read the profile: read or
// control access (A.4: VMS 7.1's $CHANGE_ACL read an ACL with either).
func (o *SecurityObject) MayRead() bool {
	return unchecked(o.creds) || mayEither(o.creds, o.vol, &o.file.Header, security.Read, security.Control)
}

// MayChange reports whether the process may change the profile: control
// access (Decision 5), and the profile writable.
func (o *SecurityObject) MayChange() bool {
	if !o.writable {
		return false
	}

	return unchecked(o.creds) || may(o.creds, o.vol, &o.file.Header, security.Control)
}

// SetACL makes acl the file's access control list. The caller has asked
// MayChange.
func (o *SecurityObject) SetACL(acl []byte) error {
	if !o.writable {
		return ErrNoPrivilege
	}

	return volume.SetACL(o.file, acl)
}

// SetOwnerProtection makes owner and protection the file's: control
// access, and an owner the process may give (security.CanChangeOwner),
// else ErrNoPrivilege.
func (o *SecurityObject) SetOwnerProtection(owner security.UIC, protection security.Protection) error {
	if !o.MayChange() {
		return ErrNoPrivilege
	}

	profile := o.Profile()
	if owner != profile.Owner && !unchecked(o.creds) && !security.CanChangeOwner(o.creds, profile, owner) {
		return ErrNoPrivilege
	}

	return updateAttributes(o.file, func(a *ACPAttributes) {
		a.Owner, a.Protection = uint32(owner), uint16(protection)
	})
}
