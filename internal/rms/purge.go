package rms

import (
	"fmt"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-23.md subtask 7: PURGE, the operator-
// console command that trims every name a file specification matches down
// to its N most recent surviving versions. Its behavioral reference is the
// sibling ods2 module's own cmd/ods2/internal/session/purge.go (read-only,
// per this phase's own "behavioral spec, not code to link against" framing
// -- see docs/PHASE-23.md's "Why this phase looks different from most
// others"); Session.Purge below follows that file's shape closely (the
// same "*.* means everything", the same forced ";*" version override, the
// same group-by-directory/PurgeVersions loop, the same deferred bitmap
// flush) without being a literal port, since ods2's own session.Session
// type lives in an internal/ package this module isn't allowed to import.
//
// # A note for a reader new to VMS
//
// PURGE is DELETE's "housekeeping" cousin: instead of naming one exact
// version to remove, it looks at every name a (possibly wildcarded) file
// specification matches and keeps only the N newest versions of each,
// deleting the rest -- the everyday way a VMS user cleans up the pile of
// numbered backup versions a text editor or compiler leaves behind, without
// having to know or type each individual version number. Because of this,
// PURGE deliberately ignores any version a typed spec happens to include
// (see Purge's own doc comment below) -- unlike DELETE, where the version
// is the whole point.

// Purge trims every distinct name specText matches down to its keep most
// recent surviving versions (volume.PurgeVersions does the actual survivor
// selection and deletion), and returns the distinct "NAME.TYPE" names it
// touched -- enough for the console layer to print one "%PURGE-S-PURGED,
// NAME.TYPE purged (keeping N version(s))" confirmation line per name,
// matching ods2's own cmdPurge output.
//
// An empty specText defaults to "*.*" (every name in the current default
// directory), the same convenience default DIRECTORY itself uses. Whatever
// version selector specText's own text happens to carry is overridden
// outright (not rejected as an error the way DELETE rejects a missing
// version) -- PURGE always considers every surviving version of a matched
// name, regardless of what, if anything, was typed after a ";".
//
// keep must be at least 1; a caller wanting to remove every version of a
// name should use Delete's own ";*" version selector instead (matching
// real VMS's convention that DELETE, not PURGE, is the tool for removing
// everything).
func (s *Session) Purge(specText string, keep uint16) (purged []string, err error) {
	if keep == 0 {
		return nil, fmt.Errorf("purge: %w", &InvalidLimitError{Limit: keep})
	}

	if specText == "" {
		specText = "*.*"
	}

	vol, spec, err := s.resolveVolume(specText)
	if err != nil {
		return nil, fmt.Errorf("purge: %w", err)
	}

	spec.Version = "*"

	if len(vol.Devices) != 1 {
		return nil, fmt.Errorf("purge: %s is a %d-device volume set; PURGE only supports a single-device volume", spec.Device, len(vol.Devices))
	}

	dev := vol.Devices[0]

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return nil, fmt.Errorf("purge: %w", err)
	}

	bm, err := dev.Bitmap()
	if err != nil {
		return nil, fmt.Errorf("purge: %w", err)
	}

	ib, err := dev.IndexBitmap()
	if err != nil {
		return nil, fmt.Errorf("purge: %w", err)
	}

	// Flushed once, however this function returns -- see Delete's own
	// identical pattern (delete.go) and doc comment for why a deferred
	// flush (rather than one only on the success path) is correct here
	// too: a PURGE that trims some names before failing on a later one
	// still keeps whatever storage it already freed.
	defer func() {
		if flushErr := bm.Flush(); flushErr != nil && err == nil {
			err = fmt.Errorf("purge: %w", flushErr)
		}

		if flushErr := ib.Flush(); flushErr != nil && err == nil {
			err = fmt.Errorf("purge: %w", flushErr)
		}
	}()

	for _, group := range groupMatchesByDir(matches) {
		dir, dirErr := filespec.ResolveDirectory(vol, group.dirs)
		if dirErr != nil {
			return purged, fmt.Errorf("purge: %w", dirErr)
		}

		for _, name := range distinctNames(group.matches) {
			if purgeErr := volume.PurgeVersions(dir, name, keep, bm, ib); purgeErr != nil {
				return purged, fmt.Errorf("purge: %w", purgeErr)
			}

			purged = append(purged, name)
		}
	}

	return purged, nil
}

// InvalidLimitError reports a /LIMIT value PURGE can't use -- currently
// just zero (volume.PurgeVersions' own doc comment explains why: "DELETE
// NAME;*" is the correct way to remove every version of a name, not
// "PURGE/LIMIT=0"). Given its own type (matching NotMountedError/
// VersionRequiredError's own reasoning elsewhere in this package), the
// console layer can report this as a distinct VMS status (CLI_BADLIMIT)
// rather than lumping it in with an ordinary malformed file specification.
type InvalidLimitError struct {
	Limit uint16
}

func (e *InvalidLimitError) Error() string {
	return fmt.Sprintf("rms: /LIMIT=%d is invalid; must be at least 1 (DELETE NAME;* removes every version)", e.Limit)
}

// distinctNames returns matches' combined "NAME.TYPE" names with
// duplicates removed, in first-seen order -- Glob's flat match list has one
// entry per surviving version, so a name with several surviving versions
// appears several times, but volume.PurgeVersions only needs to be called
// once per distinct name (it resolves every version itself via
// Directory.List internally), matching ods2's own identically named,
// identically shaped helper in its purge.go.
func distinctNames(matches []filespec.Match) []string {
	seen := make(map[string]bool, len(matches))
	names := make([]string, 0, len(matches))

	for _, m := range matches {
		full := m.Name + "." + m.Type
		if !seen[full] {
			seen[full] = true

			names = append(names, full)
		}
	}

	return names
}
