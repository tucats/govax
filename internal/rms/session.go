package rms

import (
	"fmt"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-23.md subtask 3: Session, the piece of
// operator-console state that DIRECTORY/DELETE/PURGE/COPY/TYPE (and
// INITIALIZE/CONTAINER's sibling console commands) all need but that
// Phase 22's MountTable never had to track on its own — a "current default
// device and directory" a partial, abbreviated file spec typed at the
// console gets filled in from.
//
// # Why this matters, for a reader new to VMS
//
// Real VMS never makes you type a fully-qualified file specification every
// time. After you SET DEFAULT DUA0:[MYDIR], a later command like
// "DIRECTORY *.TXT" or "TYPE FOO.TXT" is understood as if you'd typed
// "DUA0:[MYDIR]*.TXT" / "DUA0:[MYDIR]FOO.TXT" — the device and directory
// you didn't type are filled in from whatever SET DEFAULT last established.
// Session.Default is exactly that "whatever was last established" state,
// and Session.SetDefault/DefaultString are the Go equivalents of running
// SET DEFAULT and SHOW DEFAULT.

// Session holds one operator console's current-default-directory state
// (Default) alongside the same MountTable Phase 22's MOUNT/DISMOUNT
// commands already populate (Mounts) — bundled together here because
// resolving a partial file spec into a usable *volume.Volume genuinely
// needs both: Default supplies whatever the spec's own text left
// unwritten, and Mounts is what turns the resulting device name into an
// actual open volume to operate on (see resolveVolume below).
//
// A Session has no concurrency protection of its own, matching every other
// piece of shared console state in this project (MountTable's own doc
// comment explains why: govax has no concept of multiple VAX processes
// sharing one Console at once, so nothing here needs to guard against two
// goroutines racing on it).
type Session struct {
	// Mounts is the same *MountTable a Console already owns (Phase 22) —
	// Session doesn't create or own a separate one of its own, it just
	// holds a reference to the Console's existing table, exactly the way
	// docs/PHASE-23.md's design section describes it ("reuses Console's
	// existing MountTable, not a copy").
	Mounts *MountTable

	// Default is the operator's current default device/directory: the
	// filespec.Spec a partial file spec typed at the console is parsed
	// against (filespec.Parse's own "def" argument). Its zero value (no
	// device, no name, empty Dirs) is a legitimate starting state — an
	// empty Dirs means "the master file directory", not "unset" (see
	// filespec.Spec.Dirs's own doc comment in the sibling ods2 module) —
	// matching a freshly booted VMS session that has never run SET
	// DEFAULT: any command needing a device still has to name one
	// explicitly until SET DEFAULT establishes one.
	Default filespec.Spec
}

// NewSession returns a Session sharing mounts (a Console's existing
// MountTable) and with no default device/directory set yet.
func NewSession(mounts *MountTable) *Session {
	return &Session{Mounts: mounts}
}

// SetDefault parses text (an operator-typed file specification, e.g.
// "DUA0:[MYDIR]") against the current Default and, on success, replaces
// Default with the result — the Go equivalent of running SET DEFAULT.
//
// Per filespec.Parse's own documented rules, any component text doesn't
// specify is inherited from the previous Default rather than cleared: SET
// DEFAULT DUA0: alone, for instance, changes only the device, leaving
// whatever directory was previously in effect untouched — exactly how real
// VMS's own SET DEFAULT behaves for a device-only argument.
func (s *Session) SetDefault(text string) error {
	spec, err := filespec.Parse(text, s.Default)
	if err != nil {
		return fmt.Errorf("rms: SET DEFAULT %q: %w", text, err)
	}

	s.Default = spec

	return nil
}

// DefaultString renders the current Default back into VMS file-
// specification text (e.g. "DUA0:[MYDIR]") for SHOW DEFAULT to display —
// the Go equivalent of running SHOW DEFAULT. It never fails: filespec.Spec.
// String always produces something displayable, even for the all-zero-
// value "nothing set yet" starting state (which renders as "[000000]", the
// master file directory of no particular device).
func (s *Session) DefaultString() string {
	return s.Default.String()
}

// resolveVolume parses specText against the session's current Default and
// looks up the resulting device name in Mounts — the one two-step lookup
// (parse the spec, then find its device's mounted volume) that every one of
// this phase's file-operating commands (DIRECTORY, DELETE, PURGE, COPY,
// TYPE) needs to perform before it can do anything else, factored out here
// so each of those command implementations (added in later docs/PHASE-23.md
// subtasks) doesn't have to repeat it.
//
// It returns both the resolved *volume.Volume to operate on and the fully
// resolved filespec.Spec (device, directory, name, type, and version all
// filled in) — a caller needs the complete spec too, not just the volume,
// since it still has to match file names against the (possibly wildcarded)
// name/type/version fields.
func (s *Session) resolveVolume(specText string) (*volume.Volume, filespec.Spec, error) {
	spec, err := filespec.Parse(specText, s.Default)
	if err != nil {
		return nil, filespec.Spec{}, fmt.Errorf("rms: %q: %w", specText, err)
	}

	vol, ok := s.Mounts.Lookup(spec.Device)
	if !ok {
		return nil, filespec.Spec{}, fmt.Errorf("rms: %s: not mounted", spec.Device)
	}

	return vol, spec, nil
}
