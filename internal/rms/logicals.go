package rms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/ods2/filespec"
)

// This file is RMS's use of the logical-name database (docs/PHASE-25.md
// subtask 7): translating a file specification's logical names, applying
// the default device (SYS$DISK) and directory, and fanning a search list
// out into the specs it stands for.
//
// How the fields of a name are filled in is name processing's own affair
// (name.go); this file is its face toward the DCL utilities, which take Go
// errors and a resolvedSpec where the services take a status and a
// parsedName.
//
// A search list in the spec, or in SYS$DISK when the spec has no device,
// gives one resolved spec per element, in order.

// sysDiskName is the logical name whose translation is the default
// device, as SET DEFAULT sets it.
const sysDiskName = "SYS$DISK"

// LogicalNameError reports that a file specification's logical names
// couldn't be translated: a circular definition, or one more than
// lnm.MaxDepth levels deep. Err is the lnm status (SS$_TOOMANYLNAM); a
// system service reports it as RMS$_LNE.
type LogicalNameError struct {
	Spec string
	Err  error
}

func (e *LogicalNameError) Error() string {
	return fmt.Sprintf("rms: %s: %v", e.Spec, e.Err)
}

func (e *LogicalNameError) Unwrap() error { return e.Err }

// resolvedSpec is one file specification after translation and defaults.
type resolvedSpec struct {
	// Spec is the complete specification. Its Device has any leading
	// "_" (the physical-device marker) removed.
	Spec filespec.Spec

	// Display is the device name to show the user: the concealed
	// logical name when the translation went through one, and otherwise
	// the device as it was written, "_" and all.
	Display string

	// Explicit reports that the specification, once its logical names
	// were translated, named a directory itself, rather than taking the
	// default's: "[A]" or "DISK:" where DISK translates to "DUA1:[A]",
	// but not "DUA1:".
	Explicit bool

	// name is the name processing this came from.
	name parsedName
}

// shownDir is the device and directory to show for a file found in dirs, a
// path from the volume's MFD. Every command that names a file it found
// shows it through here or shownFile, since a directory under a root isn't
// shown as the path the volume has: "BASE:[SUB]", "_DUA1:[ROOT.][SUB]", and
// the root itself "[000000]" (VMS 7.1: testdata/lnm91).
func (r resolvedSpec) shownDir(dirs []string) string {
	return r.Display + ":" + r.name.shownDir(r.name.relative(dirs))
}

// shownFile is m's whole name as a user sees it: "BASE:[SUB]A.DAT;1".
func (r resolvedSpec) shownFile(m filespec.Match) string {
	return r.shownDir(m.Dirs) + m.ShortName(';')
}

// translateSpec returns text's logical-name translations in search order
// (lnm.Database.TranslateFileSpec at user mode, which sees every name).
// A nil db translates nothing.
func translateSpec(db *lnm.Database, text string) ([]lnm.FileSpec, error) {
	if db == nil {
		return []lnm.FileSpec{{Spec: text, Display: text, Remainder: text}}, nil
	}

	fs, err := db.TranslateFileSpec(text, lnm.User)
	if err != nil {
		return nil, &LogicalNameError{Spec: text, Err: err}
	}

	return fs, nil
}

// NameError reports a file specification name processing refused. Status
// is the RMS status it failed with: RMS$_DIR, RMS$_SYN, and the like.
type NameError struct {
	Spec   string
	Status uint32
}

func (e *NameError) Error() string {
	if m, ok := vmsdef.LookupMessage(e.Status); ok {
		return fmt.Sprintf("rms: %q: %s", e.Spec, m.Text)
	}

	return fmt.Sprintf("rms: %q: status %%X%08X", e.Spec, e.Status)
}

// expandText is expandNames with a failure as a Go error, for callers
// outside the system services: a *LogicalNameError or a *NameError.
func expandText(db *lnm.Database, base filespec.Spec, in nameInputs) ([]parsedName, error) {
	names, sts := expandNames(db, base, in)
	if sts == 0 {
		return names, nil
	}

	if sts == rmsLogicalNameError {
		// Name processing keeps only the status; translating again finds
		// which name it was, and why.
		for _, text := range []string{in.Primary, in.Default, sysDiskName + ":"} {
			if _, err := translateSpec(db, strings.ToUpper(text)); err != nil {
				return nil, err
			}
		}
	}

	return nil, &NameError{Spec: in.Primary, Status: sts}
}

// resolved is p as the DCL utilities use a name: the volume layer's spec,
// and the device to show them.
func (p parsedName) resolved() resolvedSpec {
	return resolvedSpec{
		Spec:     p.spec(),
		Display:  strings.TrimSuffix(p.Dev, ":"),
		Explicit: p.FNB&fnbExpDir != 0,
		name:     p,
	}
}

// resolvedAll is resolved for each of names.
func resolvedAll(names []parsedName) []resolvedSpec {
	out := make([]resolvedSpec, len(names))
	for i, p := range names {
		out[i] = p.resolved()
	}

	return out
}

// defaultSpecs returns the defaults a spec with no device is resolved
// against: one per element of SYS$DISK's translation, each with base's
// directory unless that element has its own. When SYS$DISK isn't
// defined it returns base's directory alone.
func defaultSpecs(db *lnm.Database, base filespec.Spec) ([]filespec.Spec, error) {
	names, err := expandText(db, base, nameInputs{ThroughPermanent: true})
	if err != nil {
		return nil, fmt.Errorf("rms: %s: %w", sysDiskName, err)
	}

	out := make([]filespec.Spec, len(names))
	for i, p := range names {
		out[i] = p.spec()
	}

	return out, nil
}

// expandSpec is name processing for the DCL utilities: text's logical
// names translated and the process defaults applied (base is the default
// directory, SYS$DISK the device), every resolved spec in search order.
func expandSpec(db *lnm.Database, text string, base filespec.Spec) ([]resolvedSpec, error) {
	names, err := expandText(db, base, nameInputs{Primary: text, ThroughPermanent: true})
	if err != nil {
		return nil, err
	}

	return resolvedAll(names), nil
}

// PhysicalDevice translates text, a device name as typed for MOUNT or
// DISMOUNT ("DUA0:", "DUA0", "_DUA0:", or a logical name such as
// "DISK:"), into the device it names, without its "_" or ":". A search
// list names its first element's device. Only the device field is read:
// the services that want a device reach it whatever RMS would make of the
// rest of the name (VMS 7.1: testdata/lnm91; testdata/lib58's 64-character
// device).
func PhysicalDevice(db *lnm.Database, text string) (string, error) {
	if !strings.Contains(text, ":") {
		text += ":"
	}

	fs, err := translateSpec(db, text)
	if err != nil {
		return "", err
	}

	spec := fs[0].Spec

	device, _, ok := strings.Cut(spec, ":")
	device = strings.TrimPrefix(device, "_")

	if !ok || device == "" || strings.ContainsAny(device, "[<") {
		return "", fmt.Errorf("rms: %q: no device name", text)
	}

	return device, nil
}

// isNotFound reports whether err is one a search list moves past to try
// its next element: the file isn't there, or the device isn't mounted.
func isNotFound(err error) bool {
	var (
		notFound   *NotFoundError
		notMounted *NotMountedError
	)

	return errors.As(err, &notFound) || errors.As(err, &notMounted)
}

// resolveFileSpec is expandSpec for a system service: fn resolved against
// the console session's default directory (the master file directory
// when there's no session). On failure it returns the RMS status to
// report instead: RMS$_LNE for a logical name that can't be translated,
// and RMS$_FNF for a spec that can't be parsed, which has no file to
// find.
func (ctx *Context) resolveFileSpec(fn string) ([]resolvedSpec, uint32) {
	var base filespec.Spec
	if ctx.Session != nil {
		base = ctx.Session.Default
	}

	specs, err := expandSpec(ctx.Logicals, fn, base)
	if err != nil {
		var lne *LogicalNameError
		if errors.As(err, &lne) {
			return nil, rmsLogicalNameError
		}

		return nil, rmsFileNotFound
	}

	return specs, 0
}
