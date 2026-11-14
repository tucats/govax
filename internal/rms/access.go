package rms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is where RMS and the ACP decide whether the process making a
// call may do what it asks to a file or directory (docs/planning/PHASE-93
// - file protection.md, Decisions 5 and 6). The rule itself, how a
// process's credentials meet an object's owner, protection code, and
// access control list, is internal/security's; this file says which
// access each operation needs, of which object, and reads the object's
// profile from its file header.
//
// # A note for a reader new to VMS
//
// A directory on a VMS disk is a file like any other, with an owner, a
// protection code, and possibly an ACL, and its access types mean:
//
//   - R (read): list it, as a wildcard search does;
//   - E (execute): look a name up in it, but not list it;
//   - W (write): change its entries, as making, deleting, and renaming
//     files do (VMS 7.1 wants R as well; the probe's A.4).
//
// To reach DUA1:[A.B]X.DAT, VMS looks [A] up in the master file
// directory, [B] in [A], and X.DAT in [B]: each of the three needs R or E
// on the directory looked in. Then the file's own protection decides the
// open: R to read it, R and W to write it (write alone isn't enough), D to
// delete it. A file opened by its file ID passes over the directories.
//
// An access is checked when it's made, and never again for the same open
// (VMS checks at access too): a protection changed later doesn't affect
// a file already open. Nothing is kept between checks: each one reads the
// header it's about and asks the process's credentials again.
//
// A process holding BYPASS may do anything, so its checks are skipped
// (security.Check would grant them all): that is every process govax has
// today, SYSTEM, whose work therefore costs nothing more than before.
// SET DEBUG PROTECTION makes them all the same, so each shows.


// unchecked reports whether checks for creds can be skipped: BYPASS gives
// every access, so no check could refuse, unless SET DEBUG PROTECTION is
// tracing them (security.Credentials.Trace), when each is made so it's
// shown.
func unchecked(creds security.Credentials) bool {
	return creds.Has(security.PrivBYPASS) && creds.Trace == nil
}

// headerProfile is the security profile the header h of a file on vol
// gives: its owner, its protection code, and its ACL.
func headerProfile(vol *volume.Volume, h *ondisk.FileHeader) security.Profile {
	p := security.Profile{
		Owner:      uicOf(h.Owner),
		Protection: security.Protection(h.FileProtection),
		OnVolume:   true,
	}

	if id, err := h.Ident(); err == nil {
		p.Name = strings.TrimRight(id.Filename+id.FilenameExtension, " ")
	}

	if raw, err := h.ACL(); err == nil && len(raw) > 0 {
		p.ACL, _ = security.DecodeACL(raw)
	}

	if vol != nil && len(vol.Devices) > 0 && vol.Devices[0] != nil {
		p.VolumeOwner = uicOf(vol.Devices[0].Home.VolumeOwner)
	}

	return p
}

// may reports whether creds give every access in want to the object
// whose header is h.
func may(creds security.Credentials, vol *volume.Volume, h *ondisk.FileHeader, want security.Access) bool {
	if unchecked(creds) {
		return true
	}

	return security.Check(creds, headerProfile(vol, h), want).Granted
}

// mayEither reports whether creds give a or b to the object whose header
// is h: R or E to look a name up, to run an image, or to start a command
// procedure; R or C to read a profile.
func mayEither(creds security.Credentials, vol *volume.Volume, h *ondisk.FileHeader, a, b security.Access) bool {
	return may(creds, vol, h, a) || may(creds, vol, h, b)
}

// dirNeed is what an operation does in a directory, which decides the
// access it needs to it.
type dirNeed int

const (
	// needLookup is looking a known name up: R or E.
	needLookup dirNeed = iota

	// needList is listing the entries, as a wildcard search does: R.
	needList

	// needChange is changing the entries (a create, an erase, a rename):
	// R and W. VMS 7.1 refused a create, erase, and rename in a directory
	// giving W and E, and allowed them with R and W (the probe's A.4,
	// "Directories").
	needChange
)

// allows reports whether creds give the access n needs to the directory
// dir on vol.
func (n dirNeed) allows(creds security.Credentials, vol *volume.Volume, dir *volume.Directory) bool {
	switch n {
	case needList:
		return may(creds, vol, &dir.Header, security.Read)
	case needChange:
		return may(creds, vol, &dir.Header, security.Read|security.Write)
	default:
		return mayEither(creds, vol, &dir.Header, security.Read, security.Execute)
	}
}

// fileAccess is what an open with access fac (FAB$B_FAC, already
// defaulted) needs of the file: R, and W too if it asks to write. Block
// I/O for reading (BIO, BRO) is reading.
func fileAccess(fac byte) security.Access {
	if fac&facWrite != 0 {
		return security.Read | security.Write
	}

	return security.Read
}

// errDirectoryNotFound is a directory path that doesn't lead anywhere: a
// level that isn't there, or isn't a directory.
var errDirectoryNotFound = errors.New("rms: directory not found")

// resolveChecked is ods2's filespec.ResolveDirectory, with each lookup
// on the way checked: every directory a level is looked up in (the
// master file directory, then each level but the last) needs R or E
// (needLookup). The last level, the directory itself, is the caller's to
// check, by what it does there (directoryFor). A level refused is
// ErrNoPrivilege; one that isn't there an error that isn't.
//
// VMS 7.1 refused $OPEN and $PARSE a path an upper level of which gave
// no access, and allowed $PARSE when only the last level did (the probe's
// PX1 to PX3). VMS's RMS keeps a process's directories in a cache, so a
// level it has already looked up may not be looked up again; govax keeps
// none, so every level is checked every time, as the Guide to System
// Security's rule says.
func resolveChecked(creds security.Credentials, vol *volume.Volume, dirs []string) (*volume.Directory, error) {
	if unchecked(creds) || dirsHaveWildcard(dirs) {
		return filespec.ResolveDirectory(vol, dirs)
	}

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		return nil, err
	}

	for _, level := range dirs {
		if !needLookup.allows(creds, vol, dir) {
			return nil, ErrNoPrivilege
		}

		entry, err := dir.Lookup(strings.ToUpper(level)+".DIR", 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errDirectoryNotFound, level)
		}

		// The MFD's entry for itself, 000000.DIR, isn't a level below it.
		if entry.Fid.Equal(dir.Header.Fid) {
			return nil, fmt.Errorf("%w: %s", errDirectoryNotFound, level)
		}

		if dir, err = vol.OpenDirectory(entry.Fid); err != nil {
			return nil, fmt.Errorf("%w: %s", errDirectoryNotFound, level)
		}
	}

	return dir, nil
}

// directoryFor resolves the directory dirs names on vol (resolveChecked)
// and checks that creds give what need asks of it: ErrNoPrivilege if not.
func directoryFor(creds security.Credentials, vol *volume.Volume, dirs []string, need dirNeed) (*volume.Directory, error) {
	dir, err := resolveChecked(creds, vol, dirs)
	if err != nil {
		return nil, err
	}

	if !need.allows(creds, vol, dir) {
		return nil, ErrNoPrivilege
	}

	return dir, nil
}

// dirsHaveWildcard reports whether any of dirs has a wildcard character, which
// the checked walk doesn't follow (a search's walk does its own).
func dirsHaveWildcard(dirs []string) bool {
	for _, d := range dirs {
		if strings.ContainsAny(d, "*%") || d == ellipsis {
			return true
		}
	}

	return false
}

// directoryStatus is the RMS status for a failed directoryFor or
// resolveChecked: RMS$_PRV for a refusal (the caller's STV SS$_NOPRIV,
// through Context.denied), RMS$_DNF for a directory that isn't there.
func (ctx *Context) directoryStatus(err error) uint32 {
	if errors.Is(err, ErrNoPrivilege) {
		return ctx.denied()
	}

	return rmsDirNotFound
}

// denied returns RMS$_PRV for an access the process's credentials don't
// give, noting that its STV is SS$_NOPRIV (failureSTV). RMS$_PRV for a
// write to a volume mounted read-only has none.
func (ctx *Context) denied() uint32 {
	ctx.noPrivilege = true

	return rmsPrivilegeViolation
}

// failureSTV is the STV a service reports with the failure sts: stvFor's,
// SS$_NOPRIV for a refusal denied noted, nothing for RMS$_FLK from another
// RMS opener's sharing.
func (ctx *Context) failureSTV(sts uint32) uint32 {
	stv := stvFor(sts)

	if ctx.lockedByOpener {
		stv, ctx.lockedByOpener = 0, false
	}

	if ctx.noPrivilege {
		ctx.noPrivilege = false

		if sts == rmsPrivilegeViolation {
			stv = ssNoPriv
		}
	}

	return stv
}

// mayOpenFID reports whether the process may have want to the file whose
// ID is fid on vol. A file ID that names no file is allowed here, for the
// caller's own lookup to report.
func (ctx *Context) mayOpenFID(vol *volume.Volume, fid ondisk.Fid, want security.Access) bool {
	creds := ctx.Identity()
	if unchecked(creds) {
		return true
	}

	f, err := vol.OpenFID(fid)

	return err != nil || may(creds, vol, &f.Header, want)
}

// fopSUP is FAB$M_SUP: $CREATE may supersede a file of the name and
// version given.
var fopSUP = vmsConst("FAB$M_SUP")

// supersedeTarget is the directory entry a $CREATE with FOP fop of name;
// version in dir replaces, nil for none: with FOP SUP, an explicit version
// that's there already. The file superseded must give the process W and D
// (RMS$_PRV); VMS 7.1 refused a supersede over a file without D, and over
// one with R and D alone, and the file it made in place of one allowed
// was the creator's, with the old file's protection (the probe's A.4,
// answer 3).
func (ctx *Context) supersedeTarget(creds security.Credentials, vol *volume.Volume, dir *volume.Directory, name string, version uint16, fop uint32) (*ondisk.DirEntry, uint32) {
	if fop&fopSUP == 0 || version == 0 {
		return nil, 0
	}

	entry, err := dir.Lookup(name, version)
	if err != nil {
		return nil, 0
	}

	if !ctx.mayOpenFID(vol, entry.Fid, security.Write|security.Delete) {
		return nil, ctx.denied()
	}

	return &entry, 0
}

// newVersionAllowed reports whether creds let the process make a new
// version of name in dir (the Guide to System Security, 4.6, and the
// probe's A.4, answer 3): the highest version there must give it W, and
// when the name's version limit would be passed, each version that would
// go to keep it must give it D. A name with no version yet needs nothing
// more than the directory's write access.
//
// The version limit is the name's directory record's, which VMS reads
// (a VMS volume's headers have none: VMS 7.1 refused the probe's L files,
// limit 2, whose oldest version gave no D), else the one ods2 enforces,
// the highest version's header's.
func newVersionAllowed(creds security.Credentials, vol *volume.Volume, dir *volume.Directory, name string) bool {
	if unchecked(creds) {
		return true
	}

	entries, err := dir.List()
	if err != nil {
		return true
	}

	var versions []ondisk.DirEntry

	for _, e := range entries {
		if strings.EqualFold(e.Name, name) {
			versions = append(versions, e) // highest first, in directory order
		}
	}

	if len(versions) == 0 {
		return true
	}

	highest, err := vol.OpenFID(versions[0].Fid)
	if err != nil {
		return true
	}

	if !may(creds, vol, &highest.Header, security.Write) {
		return false
	}

	limit := int(highest.Header.RecordAttributes.VersionLimit)
	if l := versions[0].VersionLimit; l != 0 && l < ondisk.NoVersionLimit {
		limit = int(l)
	}

	if limit == 0 || len(versions)+1 <= limit {
		return true
	}

	// The oldest versions past the limit, counting the new one.
	for _, e := range versions[limit-1:] {
		if f, err := vol.OpenFID(e.Fid); err == nil && !may(creds, vol, &f.Header, security.Delete) {
			return false
		}
	}

	return true
}
