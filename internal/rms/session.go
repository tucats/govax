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

	// DeviceExists, when set, reports whether the system has a device of
	// a physical name ("TTA0"): F$PARSE of a device that holds no volume,
	// a terminal, is a name all the same (Parse). The console sets it to
	// its device table's.
	DeviceExists func(device string) bool
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
// Any component text doesn't specify is inherited from the previous Default
// rather than cleared: SET DEFAULT DUA0: alone, for instance, changes only
// the device, leaving whatever directory was previously in effect
// untouched — exactly how real VMS's own SET DEFAULT behaves for a
// device-only argument.
//
// The device goes into SYS$DISK (process table, supervisor mode) and the
// rest into Default, as VMS keeps them. Logical names in text are
// translated first, keeping a concealed name rather than the device it
// hides (User's Manual §11.3.4). A search list is the exception: its name
// goes into SYS$DISK untranslated and the directory is left as it was
// (§11.7.2), so later commands search each element in turn.
func (s *Session) SetDefault(text string) error {
	if _, err := translateSpec(s.Logicals, text); err != nil {
		return err
	}

	typed, sts := scanName(text)
	if sts != 0 {
		return &NameError{Spec: text, Status: sts}
	}

	// A logical name alone is a device, its colon left off.
	if typed.Dev == "" && typed.Dir == "" && typed.Root == "" && typed.Type == "" && typed.Ver == "" && s.Logicals != nil {
		if _, err := s.Logicals.Translate(lnm.FileDevName, typed.Name, lnm.User, 0); err == nil {
			typed = fileName{Dev: typed.Name + ":"}
		}
	}

	device, translated, err := s.defaultDevice(typed, text)
	if err != nil {
		return err
	}

	if device != "" {
		eqv := []lnm.Equivalence{{Value: device + ":"}}
		if _, err := s.Logicals.Define(lnm.ProcessTableName, sysDiskName, lnm.Supervisor, 0, eqv); err != nil {
			return fmt.Errorf("rms: SET DEFAULT %q: %w", text, err)
		}
	}

	// From here on SYS$DISK has changed, whatever becomes of the directory
	// (VMS 7.1: testdata/lnm91).
	switch {
	case translated != nil:
		s.Default = filespec.Spec{Dirs: translated.names()}

		return nil
	case typed.Root != "":
		return &NameError{Spec: text, Status: rmsDirError}
	case device != "" && typed.Dir == "":
		return nil
	}

	names, err := expandText(s.Logicals, s.Default, nameInputs{Primary: typed.Dir + typed.Name + typed.Type + typed.Ver, ThroughPermanent: true})
	if err != nil {
		return fmt.Errorf("rms: SET DEFAULT %q: %w", text, err)
	}

	// The default directory is relative to the root, if SYS$DISK has one.
	spec := names[0].spec()
	spec.Device = ""
	spec.Dirs = names[0].DirSpec.names()
	s.Default = spec

	return nil
}

// defaultDevice is what SET DEFAULT makes SYS$DISK ("" to leave it), from
// the name's device, typed, and the default directory the device's logical
// name gives, if it gives one. It follows VMS 7.1's answers
// (testdata/lnm91, A.4 of Phase 91's doc), which are the name's translation
// taken a level at a time:
//
//   - A device that isn't a logical name is itself.
//   - A concealed name is SYS$DISK as it stands, whatever it translates to,
//     and a search list too. So is a plain name that gives only a device
//     (LDEV = _DUA1: stays LDEV:).
//   - A plain name that gives a directory gives the default directory as
//     well, and SYS$DISK is its device, followed in turn: A1 = BASE:[SUB] is
//     BASE: and [SUB]. A physical device loses its "_" there (P =
//     _DUA1:[LNM91] is DUA1:). A directory typed after it is CLI$_DIRECT,
//     which is why a root that isn't concealed can't be a default.
//   - A name that gives a file name is RMS$_DIR.
//
// A root given as the directory, A2 = BASE:[SUB.], is taken as that
// directory (unconfirmed: on VMS the case can't be told from leaving the
// default directory as it was).
func (s *Session) defaultDevice(typed fileName, text string) (string, *dirSpec, error) {
	if typed.Dev == "" {
		return "", nil, nil
	}

	name := strings.TrimSuffix(typed.Dev, ":")
	hasDir := typed.Dir != "" || typed.Root != ""

	var translated *dirSpec

	for range lnm.MaxDepth {
		if s.Logicals == nil || strings.HasPrefix(name, "_") {
			break
		}

		e, err := s.Logicals.Translate(lnm.FileDevName, name, lnm.User, 0)
		if err != nil || len(e.Equivalences) != 1 || e.Equivalences[0].Attrs&lnm.AttrConcealed != 0 {
			break
		}

		eqv, sts := scanName(e.Equivalences[0].Value)
		if sts != 0 {
			break
		}

		if eqv.Name != "" || eqv.Type != "" || eqv.Ver != "" {
			return "", nil, &NameError{Spec: text, Status: rmsDirError}
		}

		if eqv.Dir == "" && eqv.Root == "" || eqv.Dev == "" {
			break
		}

		if hasDir {
			return "", nil, &DirectorySyntaxError{Spec: text}
		}

		hasDir = true

		// With no directory after it, a root is the one directory there is.
		body := dirBody(eqv.Dir)
		if eqv.Dir == "" {
			body = strings.TrimSuffix(dirBody(eqv.Root), ".")
		}

		d, _ := scanDir(body)
		translated = &d

		name = strings.TrimPrefix(strings.TrimSuffix(eqv.Dev, ":"), "_")

		if e.Equivalences[0].Attrs&lnm.AttrTerminal != 0 {
			break
		}
	}

	return name, translated, nil
}

// DirectorySyntaxError reports a SET DEFAULT with a directory after a
// logical name that gives one itself: CLI$_DIRECT.
type DirectorySyntaxError struct {
	Spec string
}

func (e *DirectorySyntaxError) Error() string {
	return fmt.Sprintf("rms: %q: invalid directory syntax", e.Spec)
}

// DefaultString renders the current default back into VMS file-
// specification text (e.g. "DUA0:[MYDIR]") for SHOW DEFAULT to display —
// the Go equivalent of running SHOW DEFAULT. The device is SYS$DISK's
// value as SET DEFAULT stored it. When that is a search list, one
// "=   DEVICE:[DIR]" line follows for each place it stands for, as VMS
// shows it (User's Manual §11.7.2), each named as RMS names it.
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

	names, err := expandText(s.Logicals, s.Default, nameInputs{ThroughPermanent: true})
	if err != nil || len(names) < 2 {
		return out
	}

	for _, n := range names {
		out += "\n  =   " + n.Dev + n.Dir
	}

	return out
}

// DefaultMissing reports whether the current default directory can't be
// used, which SHOW DEFAULT then says: its name isn't one RMS takes, or its
// directory isn't on its volume (VMS 7.1's INVDEF: testdata/lnm91).
func (s *Session) DefaultMissing() bool {
	_, found, err := s.Parse("", "", "", false, false)

	return err != nil || !found
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

	return specs[0].name.String(), nil
}

// PhysicalName is specText as ExpandName expands it but with its physical
// device and any root in place of a concealed name, as VMS's image
// activator names a file ("_DUA1:[USERS.][SUB]NOSUCH.EXE;").
func (s *Session) PhysicalName(specText string) (string, error) {
	specs, err := expandSpec(s.Logicals, specText, s.Default)
	if err != nil {
		return "", err
	}

	return specs[0].name.physical(), nil
}

// NotFound is how DCL's messages report a volume file name that found no
// file: the name as ExpandName expands it ("DUA0:[WORK]NOSUCH.COM;"),
// with RMS$_FNF, or RMS$_DNF (and SS$_NOSUCHFILE under it, as with
// OPEN) when the name's directory isn't on the volume. A name that can't
// be expanded is shown as it was given.
func (s *Session) NotFound(specText string) *DCLFileError {
	name, err := s.ExpandName(specText)
	if err != nil {
		name = strings.ToUpper(specText)
	}

	if _, found, err := s.Parse(specText, "", "", false, false); err == nil && !found {
		return newDCLFileError(name, rmsDirNotFound)
	}

	return newDCLFileError(name, rmsFileNotFound)
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
