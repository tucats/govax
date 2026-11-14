package rms

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// The DCL utilities' half of the checked layer (docs/planning/PHASE-93 -
// file protection.md, Decision 6, subtask 9). RMS's services check each
// access in the service (access.go); the utilities (TYPE, COPY,
// DIRECTORY, DELETE, ...) find their files with ods2's own search and open
// them by file ID, so they check here, against the Session's process
// (Session.Identity): globChecked in place of filespec.Glob, openChecked
// in place of Volume.OpenFID, and the directory checks of access.go for
// what makes, removes, or renames an entry. A refusal is a *DeniedError,
// which each utility turns into its own message, as VMS's do (the probe's
// A.4, "Statuses and messages").

// DeniedError is an access a DCL utility's process wasn't given: a
// directory it may not search (Search), or a file it may not open as it
// asks. Name is what the utility's message names, where the layer knows
// it: the file's whole name ("DUA1:[A]B.DAT;1"). It is ErrNoPrivilege
// (errors.Is).
//
// Output says it was a file the utility was making: its directory, or the
// version it would have superseded or passed the version limit with,
// refused it. Source is then the input it was making the file from, for a
// utility that names it too (COPY's NOTCOPIED).
type DeniedError struct {
	Name   string
	Search bool
	Output bool
	Source string
}

func (e *DeniedError) Error() string {
	what := "opening"

	switch {
	case e.Search:
		what = "searching for"
	case e.Output:
		what = "creating"
	}

	return "rms: " + what + " " + e.Name + ": " + ErrNoPrivilege.Error()
}

// Unwrap makes a DeniedError ErrNoPrivilege.
func (e *DeniedError) Unwrap() error { return ErrNoPrivilege }

// IsDenied reports whether err is a refusal by the process's protection,
// and returns the DeniedError if there is one (nil for a bare
// ErrNoPrivilege).
func IsDenied(err error) (*DeniedError, bool) {
	var denied *DeniedError
	if errors.As(err, &denied) {
		return denied, true
	}

	return nil, errors.Is(err, ErrNoPrivilege)
}

// globChecked is filespec.Glob for a DCL utility of the session's
// process: the directories above the one searched must let it look
// names up (R or E), and that one must let it list its entries (R) when
// the name or type has a wildcard, or look the name up otherwise. A
// version wildcard alone is a lookup: VMS 7.1's DIRECTORY [D]A.DAT, a
// search of A.DAT;*, listed it in a directory giving E alone, and
// DIRECTORY [D]*.DAT was refused there (the probe's Part B). A refusal is
// a *DeniedError with Search set.
//
// A specification whose directory has a wildcard or an ellipsis finds
// only what is in the directories the process may reach and search; the
// rest are passed over without a message (unconfirmed: the probe didn't
// search a wildcard directory with one it couldn't read).
func (s *Session) globChecked(vol *volume.Volume, spec filespec.Spec) ([]filespec.Match, error) {
	creds := s.Identity()
	if unchecked(creds) {
		return filespec.Glob(vol, spec)
	}

	need := globAccess(spec)

	if !dirsHaveWildcard(spec.Dirs) && !spec.Recursive {
		if _, err := directoryFor(creds, vol, spec.Dirs, need); errors.Is(err, ErrNoPrivilege) {
			return nil, &DeniedError{Search: true}
		}

		// A directory that isn't there is Glob's to report, as before.
		return filespec.Glob(vol, spec)
	}

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return nil, err
	}

	allowed := map[string]bool{}
	kept := matches[:0]

	for _, m := range matches {
		key := strings.Join(m.Dirs, ".")

		ok, seen := allowed[key]
		if !seen {
			_, err := directoryFor(creds, vol, m.Dirs, need)
			ok = err == nil
			allowed[key] = ok
		}

		if ok {
			kept = append(kept, m)
		}
	}

	return kept, nil
}

// globAccess is what a search for spec needs of the directory it looks
// in: a listing when its name or type has a wildcard (or is left out,
// which is "*"), a lookup otherwise.
func globAccess(spec filespec.Spec) dirNeed {
	if spec.Name == "" || spec.Type == "" || strings.ContainsAny(spec.Name+spec.Type, "*%") {
		return needList
	}

	return needLookup
}

// openChecked opens the file m found on vol for a DCL utility that wants
// access want of it (security.Read to read it, Read|Write to write it,
// Delete to delete it, ...): a *DeniedError named name if the session's
// process doesn't have it. The directories were checked when m was found
// (globChecked).
func (s *Session) openChecked(vol *volume.Volume, m filespec.Match, want security.Access, name string) (*volume.File, error) {
	f, err := vol.OpenFID(m.Fid)
	if err != nil {
		return nil, err
	}

	if creds := s.Identity(); !unchecked(creds) && !may(creds, vol, &f.Header, want) {
		return nil, &DeniedError{Name: name}
	}

	return f, nil
}

// DeniedFileError is a refusal err as a *DCLFileError (RMS$_PRV) for a
// utility that reports its failures so, or nil if err isn't one. The name
// is the file's, or with bare, or for a directory it may not search,
// specText expanded ("DUA1:[A]B.COM;"), as DCL's @ names it.
func (s *Session) DeniedFileError(err error, specText string, bare bool) *DCLFileError {
	denied, ok := IsDenied(err)
	if !ok {
		return nil
	}

	name := ""
	if denied != nil && !bare {
		name = denied.Name
	}

	if name == "" {
		var expandErr error
		if name, expandErr = s.ExpandName(specText); expandErr != nil {
			name = strings.ToUpper(specText)
		}
	}

	return newDCLFileError(name, rmsPrivilegeViolation)
}

// createAllowed reports whether who may make a file called name
// (NAME.TYP) at version (0 for the next) in dir on vol: the directory
// must give R and W, and the next version W on the highest version there
// and D on each the version limit would take (newVersionAllowed). A
// refusal is a *DeniedError with Output set, for the caller to name.
func createAllowed(who creator, vol *volume.Volume, dir *volume.Directory, name string, version uint16) error {
	creds := who.creds
	if unchecked(creds) {
		return nil
	}

	if !needChange.allows(creds, vol, dir) || version == 0 && !newVersionAllowed(creds, vol, dir, name) {
		return &DeniedError{Output: true}
	}

	return nil
}

// pathAllowed reports whether creds let a utility look its way down dirs
// on vol (every level but the last needs R or E): a *DeniedError with
// Output set if not. A level that isn't there is the caller's own lookup
// to report.
func pathAllowed(creds security.Credentials, vol *volume.Volume, dirs []string) error {
	if unchecked(creds) || dirsHaveWildcard(dirs) {
		return nil
	}

	if _, err := resolveChecked(creds, vol, dirs); errors.Is(err, ErrNoPrivilege) {
		return &DeniedError{Output: true}
	}

	return nil
}

// outputDenied gives a *DeniedError for an output file in err its name,
// dest with the version asked for ("DUA1:[A]B.DAT;" for none, as VMS's
// messages name it), and the input it was made from.
func outputDenied(err error, dest filespec.Spec, source string) error {
	var denied *DeniedError
	if errors.As(err, &denied) && denied.Output && denied.Name == "" {
		denied.Name = dest.String()
		if dest.Version == "" {
			denied.Name += ";"
		}

		denied.Source = source
	}

	return err
}
