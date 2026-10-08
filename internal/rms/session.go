package rms

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// NotMountedError reports that a parsed file specification named a device
// with nothing currently mounted on it — resolveVolume's own "device not
// mounted" failure, given a proper type (rather than a plain fmt.Errorf
// string) specifically so a console-layer wrapper — e.g. internal/console/
// directory.go's Console.Directory — can both recognize this particular
// failure via errors.As and recover the exact device name for its own
// diagnostic, without having to re-parse specText itself just to find it
// again. Console.Directory reports this case as SS_DEVNOTMOUNT (matching
// MOUNT/DISMOUNT's own convention for the same underlying condition) and
// every other resolveVolume failure (a malformed file specification) as
// CLI_BADFILESPEC (matching SET DEFAULT's own convention) — see its doc
// comment for the full reasoning.
type NotMountedError struct {
	Device string
}

func (e *NotMountedError) Error() string {
	return fmt.Sprintf("rms: %s: not mounted", e.Device)
}

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

	// Logicals is the process's logical-name database: file specs are
	// translated through it, and SYS$DISK in it is the default device
	// (docs/PHASE-25.md). NewSession gives a Session a private database;
	// the console replaces it with the one it shares with RMS and the
	// RTL.
	Logicals *lnm.Database

	// Default is the operator's current default directory: the
	// filespec.Spec a partial file spec typed at the console is parsed
	// against (filespec.Parse's own "def" argument). Its zero value (no
	// device, no name, empty Dirs) is a legitimate starting state — an
	// empty Dirs means "the master file directory", not "unset" (see
	// filespec.Spec.Dirs's own doc comment in the sibling ods2 module) —
	// matching a freshly booted VMS session that has never run SET
	// DEFAULT: any command needing a device still has to name one
	// explicitly until SET DEFAULT establishes one. Its Device is always
	// empty: the default device is SYS$DISK, as on VMS.
	Default filespec.Spec

	// HostFallback, when set, reads a host input file named with no
	// directory that isn't in the current directory (hostfile.go). The
	// console sets it to its search-path resolver's ReadFile.
	HostFallback func(name string) ([]byte, error)
}

// NewSession returns a Session sharing mounts (a Console's existing
// MountTable), with its own logical-name database and no default
// device/directory set yet.
func NewSession(mounts *MountTable) *Session {
	return &Session{Mounts: mounts, Logicals: lnm.NewDatabase(0)}
}

// ForProcess returns a copy of s for a new process whose logical names
// are logicals: the same mounts and host fallback, and the same default
// directory, which the new process may then change without changing s's
// (docs/PHASE-45.md: a process $CREPRC creates starts in its creator's
// default directory).
func (s *Session) ForProcess(logicals *lnm.Database) *Session {
	c := *s
	c.Logicals = logicals
	c.Default.Dirs = slices.Clone(s.Default.Dirs)

	return &c
}

// SetDefault parses text (an operator-typed file specification, e.g.
// "DUA0:[MYDIR]") against the current Default and, on success, makes it
// the new default — the Go equivalent of running SET DEFAULT.
//
// Per filespec.Parse's own documented rules, any component text doesn't
// specify is inherited from the previous Default rather than cleared: SET
// DEFAULT DUA0: alone, for instance, changes only the device, leaving
// whatever directory was previously in effect untouched — exactly how real
// VMS's own SET DEFAULT behaves for a device-only argument.
//
// The device goes into SYS$DISK (process table, supervisor mode) and the
// rest into Default, as VMS keeps them. Logical names in text are
// translated first, keeping a concealed name rather than the device it
// hides (User's Manual §11.3.4). A search list is the exception: its name
// goes into SYS$DISK untranslated and the directory is left as it was
// (§11.7.2), so later commands search each element in turn.
func (s *Session) SetDefault(text string) error {
	fs, err := translateSpec(s.Logicals, text)
	if err != nil {
		return err
	}

	base := s.Default
	base.Device = ""

	var (
		device string
		spec   filespec.Spec
	)

	if len(fs) > 1 {
		device = strings.TrimSuffix(text[:len(text)-len(fs[0].Remainder)], ":")

		spec, err = filespec.Parse(fs[0].Remainder, base)
	} else {
		t := fs[0].Spec
		if fs[0].Concealed != "" {
			t = fs[0].Display
		}

		spec, err = filespec.Parse(t, base)
		device = spec.Device
	}

	if err != nil {
		return fmt.Errorf("rms: SET DEFAULT %q: %w", text, err)
	}

	if device != "" {
		eqv := []lnm.Equivalence{{Value: device + ":"}}
		if _, err := s.Logicals.Define(lnm.ProcessTableName, sysDiskName, lnm.Supervisor, 0, eqv); err != nil {
			return fmt.Errorf("rms: SET DEFAULT %q: %w", text, err)
		}
	}

	spec.Device = ""
	s.Default = spec

	return nil
}

// DefaultString renders the current default back into VMS file-
// specification text (e.g. "DUA0:[MYDIR]") for SHOW DEFAULT to display —
// the Go equivalent of running SHOW DEFAULT. The device is SYS$DISK's
// value as SET DEFAULT stored it. When that is a search list, one
// "=   DEVICE:[DIR]" line follows for each place it stands for, as VMS
// shows it (User's Manual §11.7.2).
//
// It never fails: filespec.Spec.String always produces something
// displayable, even for the all-zero-value "nothing set yet" starting
// state (which renders as "[000000]", the master file directory of no
// particular device).
func (s *Session) DefaultString() string {
	spec := s.Default

	if e, err := s.Logicals.Translate(lnm.FileDevName, sysDiskName, lnm.User, 0); err == nil {
		spec.Device = strings.TrimSuffix(e.Equivalences[0].Value, ":")
	}

	out := spec.String()

	defaults, err := defaultSpecs(s.Logicals, s.Default)
	if err != nil || len(defaults) < 2 {
		return out
	}

	for _, d := range defaults {
		out += "\n=   " + d.String()
	}

	return out
}

// ExpandName is specText as RMS expands it, with its logical names
// translated and the default device and directory applied (a search
// list's first element): "DUA0:[000000]NOSUCH.EXE;", with the device as
// the user should see it and ";" alone when no version is given, as
// DCL's messages show a file that wasn't found.
func (s *Session) ExpandName(specText string) (string, error) {
	specs, err := expandSpec(s.Logicals, specText, s.Default)
	if err != nil {
		return "", err
	}

	r := specs[0]
	spec := r.Spec
	spec.Device = ""

	text := spec.String()
	if spec.Version == "" {
		text += ";"
	}

	device := r.Display
	if device == "" {
		device = r.Spec.Device
	}

	return device + ":" + text, nil
}

// resolveVolume resolves specText (translating its logical names and
// applying the default device and directory) and looks up the resulting
// device in Mounts. A search list resolves to its first element, the
// rule $CREATE follows and COPY uses for both its source and its
// destination.
//
// It returns both the resolved *volume.Volume to operate on and the fully
// resolved filespec.Spec (device, directory, name, type, and version all
// filled in) — a caller needs the complete spec too, not just the volume,
// since it still has to match file names against the (possibly wildcarded)
// name/type/version fields.
func (s *Session) resolveVolume(specText string) (*volume.Volume, filespec.Spec, error) {
	specs, err := expandSpec(s.Logicals, specText, s.Default)
	if err != nil {
		return nil, filespec.Spec{}, err
	}

	return s.mounted(specs[0])
}

// mounted returns r's mounted volume, or a *NotMountedError.
func (s *Session) mounted(r resolvedSpec) (*volume.Volume, filespec.Spec, error) {
	vol, ok := s.Mounts.Lookup(r.Spec.Device)
	if !ok {
		return nil, filespec.Spec{}, &NotMountedError{Device: r.Spec.Device}
	}

	return vol, r.Spec, nil
}

// eachSpec calls fn for every spec specText resolves to, in search order
// — how a command that accepts wildcards (DIRECTORY, DELETE, PURGE) uses
// a search list (User's Manual §11.7.1). An element whose device isn't
// mounted, or for which fn returns a *NotFoundError, is passed over; if
// every element was, the last such error is returned. Any other error
// stops the walk.
func (s *Session) eachSpec(specText string, fn func(vol *volume.Volume, r resolvedSpec) error) error {
	specs, err := expandSpec(s.Logicals, specText, s.Default)
	if err != nil {
		return err
	}

	var lastErr error

	found := false

	for _, r := range specs {
		vol, _, err := s.mounted(r)
		if err == nil {
			err = fn(vol, r)
		}

		switch {
		case err == nil:
			found = true

		case isNotFound(err):
			lastErr = err

		default:
			return err
		}
	}

	if !found {
		return lastErr
	}

	return nil
}

// firstSpec calls fn for each spec specText resolves to, in search
// order, until one call succeeds — how a command that works on one file
// (TYPE, $OPEN) uses a search list: the first file found wins, and if
// none is, the error for the last element tried is reported (User's
// Manual §11.7). Errors other than "not found"/"not mounted" stop the
// search at once.
func (s *Session) firstSpec(specText string, fn func(vol *volume.Volume, r resolvedSpec) error) error {
	specs, err := expandSpec(s.Logicals, specText, s.Default)
	if err != nil {
		return err
	}

	var lastErr error

	for _, r := range specs {
		vol, _, err := s.mounted(r)
		if err == nil {
			err = fn(vol, r)
		}

		if err == nil || !isNotFound(err) {
			return err
		}

		lastErr = err
	}

	return lastErr
}
