package rms

import (
	"fmt"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-23.md subtask 6: DELETE, the operator-
// console command that reclaims a file's storage and removes its directory
// entry. Its behavioral reference is the sibling ods2 module's own
// cmd/ods2/internal/session/delete.go (read-only, per this phase's own
// "behavioral spec, not code to link against" framing -- see docs/
// PHASE-23.md's "Why this phase looks different from most others");
// Session.Delete below follows that file's shape closely (the same
// required-version rule, the same group-by-directory/DeleteFile loop, the
// same deferred bitmap flush) without being a literal port, since ods2's own
// session.Session type lives in an internal/ package this module isn't
// allowed to import.
//
// # A note for a reader new to VMS
//
// Every file on an ODS-2 volume can have several "versions" -- FOO.TXT;1,
// FOO.TXT;2, and so on, each a complete, independent copy of the file kept
// side by side (the same idea as a text editor's numbered backup files, but
// built into the file system itself). DELETE always has to say exactly
// which version(s) it means: a specific number ("FOO.TXT;3"), or every
// version at once ("FOO.TXT;*"). Unlike DIRECTORY -- where "no version
// typed" sensibly means "show me the newest one" -- DELETE never guesses,
// because guessing wrong here would silently destroy data: see
// ErrVersionRequired below.

// DeletedFile reports one file DELETE actually removed -- Session.Delete's
// own per-match record, giving the console layer (internal/console/
// delete.go) enough to print one "%DELETE-S-DELETED, NAME.TYPE;version
// deleted" confirmation line per file, matching ods2's own cmdDelete
// convention of printing one line per deleted file rather than a single
// summary.
type DeletedFile struct {
	Name    string
	Type    string
	Version uint16
}

// VersionRequiredError reports that specText named a file with no specific
// version -- DELETE's own deliberate refusal to default to "the newest
// version", matching ods2's own cmdDelete rule that DIRECTORY's usual "no
// version means the highest one" convenience would be exactly the wrong
// behavior here: it would let a mistyped "DELETE *.TXT" silently delete the
// newest version of every matching name. Given its own type (rather than a
// plain fmt.Errorf string), the console layer can recognize this specific
// failure via errors.As and report it as a distinct VMS status
// (CLI_NEEDVERSION) rather than lumping it in with an ordinary malformed
// file specification.
type VersionRequiredError struct {
	Spec string
}

func (e *VersionRequiredError) Error() string {
	return fmt.Sprintf("rms: %s: a specific version is required, e.g. %s;3 or %s;* (DELETE never defaults to a version)", e.Spec, e.Spec, e.Spec)
}

// NotFoundError reports that specText's name/type/version pattern matched
// no file at all on its resolved volume -- Session.Delete's "nothing to
// delete" failure. Given its own type (matching NotMountedError's own
// reasoning in session.go), the console layer can report this as the real
// SS_NOSUCHFILE status rather than the generic CLI_BADFILESPEC every other
// kind of Session.Delete failure falls back to.
type NotFoundError struct {
	Spec string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("rms: %s: not found", e.Spec)
}

// Delete removes every file matching specText (resolved against the
// session's current Default exactly like any other command in this phase)
// from its volume, reclaiming its storage, and returns one DeletedFile per
// file actually removed.
//
// specText's version field is mandatory: a bare "FOO.TXT" (or a trailing-
// semicolon "FOO.TXT;", which filespec.Parse can't tell apart from "no
// version typed at all" -- see filespec.Spec.Version's own doc comment)
// fails with a *VersionRequiredError before anything on the volume is
// touched. A specific number ("FOO.TXT;3") deletes just that version; "*"
// ("FOO.TXT;*") deletes every version of that name; the name/type portion
// may itself be wildcarded ("*.TXT;3") to hit several files in one call.
//
// Matching real VMS (and ods2's own DeleteFile/cmdDelete), this only
// supports a single-device volume -- not expected to matter in practice,
// since this project's MountTable never mounts a multi-device volume set.
func (s *Session) Delete(specText string) (deleted []DeletedFile, err error) {
	vol, spec, err := s.resolveVolume(specText)
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}

	if spec.Version == "" {
		return nil, fmt.Errorf("delete: %w", &VersionRequiredError{Spec: specText})
	}

	if len(vol.Devices) != 1 {
		return nil, fmt.Errorf("delete: %s is a %d-device volume set; DELETE only supports a single-device volume", spec.Device, len(vol.Devices))
	}

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("delete: %w", &NotFoundError{Spec: specText})
	}

	dev := vol.Devices[0]

	bm, err := dev.Bitmap()
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}

	ib, err := dev.IndexBitmap()
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}

	// Flushed once, however this function returns (success, or an error
	// partway through several matches) -- matching ods2's own cmdDelete,
	// whose own doc comment explains why a deferred flush (rather than one
	// only on the success path) is correct: a DELETE that fails partway
	// through a multi-match glob still keeps whatever storage it already
	// freed, since bm/ib's mutations are in-memory only until Flush runs.
	// A flush failure only replaces err if the loop below didn't already
	// fail for its own reason.
	defer func() {
		if flushErr := bm.Flush(); flushErr != nil && err == nil {
			err = fmt.Errorf("delete: %w", flushErr)
		}

		if flushErr := ib.Flush(); flushErr != nil && err == nil {
			err = fmt.Errorf("delete: %w", flushErr)
		}
	}()

	for _, group := range groupMatchesByDir(matches) {
		dir, dirErr := filespec.ResolveDirectory(vol, group.dirs)
		if dirErr != nil {
			return deleted, fmt.Errorf("delete: %w", dirErr)
		}

		for _, m := range group.matches {
			fullName := m.Name + "." + m.Type

			if delErr := volume.DeleteFile(dir, fullName, m.Version, bm, ib); delErr != nil {
				return deleted, fmt.Errorf("delete: %w", delErr)
			}

			deleted = append(deleted, DeletedFile{Name: m.Name, Type: m.Type, Version: m.Version})
		}
	}

	return deleted, nil
}
