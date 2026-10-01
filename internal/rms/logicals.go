package rms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/ods2/filespec"
)

// This file is RMS's use of the logical-name database (docs/PHASE-25.md
// subtask 7): translating a file specification's logical names, applying
// the default device (SYS$DISK) and directory, and fanning a search list
// out into the specs it stands for.
//
// A file specification is resolved in three layers, each filling in only
// what the one above left out, as RMS does:
//
//  1. the fields written in the spec itself, after its logical name;
//  2. the fields of that logical name's translation (so PAY_FILE:*.DAT,
//     with PAY_FILE = DISK1:[SALES_STAFF]PAYROLL, is
//     DISK1:[SALES_STAFF]*.DAT);
//  3. the defaults: SYS$DISK's device (and directory, when SYS$DISK is a
//     search list whose elements have one) and the process default
//     directory.
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
	// logical name when the translation went through one, Spec.Device
	// otherwise.
	Display string
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

// parseTranslated parses one translation of a spec with the precedence
// described at the top of this file: the fields in f.Remainder (what the
// user wrote after the logical name), then those of the translation
// itself, then def.
func parseTranslated(f lnm.FileSpec, def filespec.Spec) (filespec.Spec, error) {
	prefix := f.Spec[:len(f.Spec)-len(f.Remainder)]
	if prefix == "" {
		return physical(filespec.Parse(f.Spec, def))
	}

	mid, err := filespec.Parse(prefix, def)
	if err != nil {
		return filespec.Spec{}, err
	}

	spec, err := filespec.Parse(f.Remainder, mid)
	if err != nil {
		return filespec.Spec{}, err
	}

	// filespec.Parse inherits a default's directory but not its "..."
	// flag, so carry that across when the remainder has no directory.
	if !strings.ContainsAny(f.Remainder, "[<") {
		spec.Recursive = mid.Recursive
	}

	return physical(spec, nil)
}

// physical removes the "_" that marks a physical device name from spec's
// device, passing err through.
func physical(spec filespec.Spec, err error) (filespec.Spec, error) {
	spec.Device = strings.TrimPrefix(spec.Device, "_")

	return spec, err
}

// defaultSpecs returns the defaults a spec with no device is resolved
// against: one per element of SYS$DISK's translation, each with base's
// directory unless that element has its own. When SYS$DISK isn't
// defined it returns base alone.
func defaultSpecs(db *lnm.Database, base filespec.Spec) ([]filespec.Spec, error) {
	if db == nil {
		return []filespec.Spec{base}, nil
	}

	if _, err := db.Translate(lnm.FileDevName, sysDiskName, lnm.User, 0); err != nil {
		return []filespec.Spec{base}, nil
	}

	fs, err := translateSpec(db, sysDiskName)
	if err != nil {
		return nil, err
	}

	out := make([]filespec.Spec, 0, len(fs))

	for _, f := range fs {
		d, err := parseTranslated(f, base)
		if err != nil {
			return nil, fmt.Errorf("rms: %s: %w", sysDiskName, err)
		}

		out = append(out, d)
	}

	return out, nil
}

// expandSpec translates text and applies the defaults, returning every
// resolved spec in search order. base is the process default (its
// directory); a translation that names no device is resolved once
// against each of SYS$DISK's defaults, one that does name a device
// against base alone.
func expandSpec(db *lnm.Database, text string, base filespec.Spec) ([]resolvedSpec, error) {
	fs, err := translateSpec(db, text)
	if err != nil {
		return nil, err
	}

	var (
		out      []resolvedSpec
		defaults []filespec.Spec
	)

	for _, f := range fs {
		// The probe only asks whether the translation names a device, but
		// it's parsed against the default directory too, or a relative
		// directory that goes up ("[-.X]") would fail as going above the
		// MFD.
		probe, err := parseTranslated(f, filespec.Spec{Dirs: base.Dirs})
		if err != nil {
			return nil, fmt.Errorf("rms: %q: %w", text, err)
		}

		ds := []filespec.Spec{{Dirs: base.Dirs}}

		if probe.Device == "" {
			if defaults == nil {
				if defaults, err = defaultSpecs(db, base); err != nil {
					return nil, err
				}
			}

			ds = defaults
		}

		for _, d := range ds {
			spec, err := parseTranslated(f, d)
			if err != nil {
				return nil, fmt.Errorf("rms: %q: %w", text, err)
			}

			display := spec.Device
			if f.Concealed != "" {
				display = f.Concealed
			}

			out = append(out, resolvedSpec{Spec: spec, Display: display})
		}
	}

	return out, nil
}

// PhysicalDevice translates text, a device name as typed for MOUNT or
// DISMOUNT ("DUA0:", "DUA0", "_DUA0:", or a logical name such as
// "DISK:"), into the device it names, without its "_" or ":". A search
// list names its first element's device.
func PhysicalDevice(db *lnm.Database, text string) (string, error) {
	if !strings.Contains(text, ":") {
		text += ":"
	}

	fs, err := translateSpec(db, text)
	if err != nil {
		return "", err
	}

	spec, err := parseTranslated(fs[0], filespec.Spec{})
	if err != nil {
		return "", fmt.Errorf("rms: %q: %w", text, err)
	}

	if spec.Device == "" {
		return "", fmt.Errorf("rms: %q: no device name", text)
	}

	return spec.Device, nil
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
