package rms

import (
	"fmt"
	"strings"

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
// built into the file system itself). DELETE always has to say which
// version(s) it means: a specific number ("FOO.TXT;3"), every version at
// once ("FOO.TXT;*"), or the newest, written as a version of 0
// ("FOO.TXT;0") or as a semicolon with no number after it ("FOO.TXT;").
// Unlike DIRECTORY -- where "no version typed" sensibly means "show me the
// newest one" -- DELETE never guesses when the version is left out
// altogether, because guessing wrong here would silently destroy data: see
// VersionRequiredError below.

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

	// Full is the file's full name, "DUA0:[WORK]X.DAT;1", its device as
	// the user named it; Blocks the blocks it had allocated.
	Full   string
	Blocks uint32
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
// specText's version field is mandatory: a bare "FOO.TXT" fails with a
// *VersionRequiredError before anything on the volume is touched. A
// specific number ("FOO.TXT;3") deletes just that version; "*"
// ("FOO.TXT;*") deletes every version of that name; 0 ("FOO.TXT;0") or a
// null version ("FOO.TXT;", see nullVersion) deletes the newest; the
// name/type portion may itself be wildcarded ("*.TXT;3") to hit several
// files in one call.
//
// Matching real VMS (and ods2's own DeleteFile/cmdDelete), this only
// supports a single-device volume -- not expected to matter in practice,
// since this project's MountTable never mounts a multi-device volume set.
//
// A search list deletes the matching files in each of its elements
// (User's Manual §11.7.1); it's an error only if none of them has any.
func (s *Session) Delete(specText string) (deleted []DeletedFile, err error) {
	specText = nullVersion(specText)

	err = s.eachSpec(specText, func(vol *volume.Volume, r resolvedSpec) error {
		files, err := deleteOnVolume(vol, r, specText)
		deleted = append(deleted, files...)

		return err
	})
	if err != nil {
		return deleted, fmt.Errorf("delete: %w", err)
	}

	return deleted, nil
}

// nullVersion returns specText with a null version -- a semicolon with no
// number after it, "FOO.TXT;" -- written as version 0, "FOO.TXT;0". DELETE
// reads both as the file's latest version (DCL Dictionary, DELETE), but
// filespec.Parse can't tell "FOO.TXT;" from a spec with no version at all
// (see filespec.Spec.Version's own doc comment), so the 0 has to be added
// to the text before it is parsed.
func nullVersion(specText string) string {
	trimmed := strings.TrimRight(specText, " \t")
	if strings.HasSuffix(trimmed, ";") {
		return trimmed + "0"
	}

	return specText
}

// DeleteResult is what DeleteList did with one element of its list.
type DeleteResult struct {
	// Spec is the element as Delete was given it: the text typed, with the
	// defaults the elements before it supplied filled in.
	Spec string

	// Deleted is the files removed (even when Err is set, those removed
	// before the failure), and Err why the element failed, as Delete
	// returns them.
	Deleted []DeletedFile
	Err     error
}

// DeleteList deletes the files each element of specs names, in order, as
// DCL's DELETE takes its list of file specifications. As VMS's DELETE,
// each element after the first takes its defaults from the one before it
// (DCL Dictionary, DELETE: "Subsequent file specifications need contain
// only a version number; the defaults will come from the preceding
// specification"): an element that names no device gets the device,
// directory, name, and type it leaves out from the previous element,
// though never its version, which each element must still give. So
// "A.TXT;1,B;2,;3" deletes A.TXT;1, B.TXT;2, and B.TXT;3.
//
// A failing element doesn't stop the list; its result says why it failed,
// and the next element is tried.
func (s *Session) DeleteList(specs []string) []DeleteResult {
	var (
		results []DeleteResult
		sticky  *filespec.Spec
	)

	for _, text := range specs {
		text = s.listDefaults(nullVersion(text), sticky)

		if next := s.listSticky(text); next != nil {
			sticky = next
		}

		deleted, err := s.Delete(text)
		results = append(results, DeleteResult{Spec: text, Deleted: deleted, Err: err})
	}

	return results
}

// listDefaults returns text with sticky's device, directory, name, and
// type filled in where text leaves them out (see DeleteList), as a full
// specification. Text that names a device of its own (directly, or as a
// logical name that translates to one) is returned as it is, to be
// resolved against the process defaults like any other, as is text that
// doesn't parse: Delete reports its error.
func (s *Session) listDefaults(text string, sticky *filespec.Spec) string {
	if sticky == nil {
		return text
	}

	fs, err := translateSpec(s.Logicals, text)
	if err != nil {
		return text
	}

	probe, err := parseTranslated(fs[0], filespec.Spec{})
	if err != nil || probe.Device != "" {
		return text
	}

	spec, err := parseTranslated(fs[0], *sticky)
	if err != nil {
		return text
	}

	// filespec.Parse inherits a default's directory but not its "..."
	// flag, so carry that across when the text has no directory.
	if !strings.ContainsAny(text, "[<") {
		spec.Recursive = sticky.Recursive
	}

	return spec.String()
}

// listSticky returns the defaults text supplies to the next element of a
// DeleteList: its device, directory, name, and type, as its first search
// list element resolves them, or nil if text doesn't resolve.
func (s *Session) listSticky(text string) *filespec.Spec {
	specs, err := expandSpec(s.Logicals, text, s.Default)
	if err != nil || len(specs) == 0 {
		return nil
	}

	r := specs[0].Spec

	return &filespec.Spec{Device: r.Device, Dirs: r.Dirs, Recursive: r.Recursive, Name: r.Name, Type: r.Type}
}

// deleteOnVolume is Delete for one resolved spec on one volume, returning
// the files it removed (even when it fails partway).
func deleteOnVolume(vol *volume.Volume, r resolvedSpec, specText string) (deleted []DeletedFile, err error) {
	spec := r.Spec

	if spec.Version == "" {
		return nil, &VersionRequiredError{Spec: specText}
	}

	if len(vol.Devices) != 1 {
		return nil, fmt.Errorf("%s is a %d-device volume set; DELETE only supports a single-device volume", spec.Device, len(vol.Devices))
	}

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, &NotFoundError{Spec: specText}
	}

	dev := vol.Devices[0]

	bm, err := dev.Bitmap()
	if err != nil {
		return nil, err
	}

	ib, err := dev.IndexBitmap()
	if err != nil {
		return nil, err
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
			err = flushErr
		}

		if flushErr := ib.Flush(); flushErr != nil && err == nil {
			err = flushErr
		}
	}()

	for _, group := range groupMatchesByDir(matches) {
		dir, dirErr := filespec.ResolveDirectory(vol, group.dirs)
		if dirErr != nil {
			return deleted, dirErr
		}

		for _, m := range group.matches {
			fullName := m.Name + "." + m.Type

			var blocks uint32
			if f, openErr := vol.OpenFID(m.Fid); openErr == nil {
				blocks = f.Header.RecordAttributes.HighestBlock
			}

			if delErr := volume.DeleteFile(dir, fullName, m.Version, bm, ib); delErr != nil {
				return deleted, delErr
			}

			deleted = append(deleted, DeletedFile{
				Name: m.Name, Type: m.Type, Version: m.Version,
				Full:   r.Display + ":" + dirSpec{Elems: m.Dirs}.String() + m.ShortName(';'),
				Blocks: blocks,
			})
		}
	}

	return deleted, nil
}
