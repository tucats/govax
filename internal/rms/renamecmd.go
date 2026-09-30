package rms

import (
	"errors"
	"strconv"
	"strings"

	"github.com/tucats/ods2/filespec"
)

// This file is the console RENAME command's half in this package:
// Session.Rename, which does what VMS's DCL RENAME does. DCL's RENAME
// (VMS V7.3 CLIUTL RENAME.B32) hands each input file specification to
// LIB$RENAME_FILE (LIBRTL LIBRENAME.BLI), which searches for the files
// it matches and, for each one, works out its new name and calls $RENAME.
// Session.Rename follows LIB$RENAME_FILE, and calls the same code
// SYS$RENAME uses (renameText, rename.go) for each file, so the console
// and a VAX program get the same checks and the same RMS statuses.
//
// # How each file's new name is worked out
//
// The output specification is parsed with the old file's full name as
// its "related file": any field the output leaves out -- device,
// directory, name, type -- is the old file's. So "RENAME [A]X.TXT [B]"
// gives [B]X.TXT, and "RENAME X.TXT .DAT" gives X.DAT. A field written as
// "*" is the old file's too, which is how one output name serves several
// files: "RENAME *.TXT *.OLD". Any other wildcard in the output is
// RMS$_WLD.
//
// The version follows LIB$RENAME_FILE's rules, which copy DCL's:
//
//   - an output version is used as given, and ";*" means the old file's;
//   - with no output version, the old file keeps its version if the input
//     named one (";3", ";-1") or all of them (";*"), or if /NONEW_VERSION
//     was given;
//   - otherwise (the input named no version, which selects each name's
//     highest one) the file gets the next version of its new name.
//
// # Several input files
//
// RENAME takes a list of inputs and one output. Each input's device and
// directory carry over to the next one as its defaults, the "temporary
// defaults" of the User's Manual §4.3.3: "RENAME [A]X.TXT,Y.TXT [B]"
// renames [A]Y.TXT too. A failure with one file is reported and the rest
// are still renamed.

// RenameOptions are the RENAME qualifiers that change what's renamed.
type RenameOptions struct {
	// NewVersion is /NEW_VERSION, on unless /NONEW_VERSION: with it off,
	// a file keeps its version even when the input named none.
	NewVersion bool
}

// RenameStage says how far one file's rename got: LIB$RENAME_FILE's
// "error source", which decides which message DCL's RENAME shows.
type RenameStage int

const (
	// RenameSearching: looking for the files an input names.
	RenameSearching RenameStage = iota

	// RenameParsing: working out a file's new name.
	RenameParsing

	// RenameRenaming: the $RENAME itself.
	RenameRenaming
)

// RenamedFile is one file RENAME renamed, or failed to.
type RenamedFile struct {
	// Old and New are the file's full names, "DUA0:[DIR]NAME.TYP;VER",
	// before and after. For a failed search, Old is the input as
	// expanded and New is empty; for a failed parse, New is the output
	// as far as it could be expanded (as typed, if not at all).
	Old, New string

	// Status is the RMS status (RMS$_NORMAL on success) and STV its
	// secondary value, as $RENAME leaves them in the FAB.
	Status, STV uint32

	// Stage is where a failure happened (see RenameStage).
	Stage RenameStage
}

// OK reports whether the file was renamed.
func (r RenamedFile) OK() bool { return r.Status == rmsNormal }

// renameSource is one file an input matched.
type renameSource struct {
	spec    filespec.Spec // its full name, with its version
	display string        // the device to show: a concealed name, if one was used

	// wildVersion and explicitVersion say what the input's version was:
	// ";*", or a number (see this file's opening comment).
	wildVersion, explicitVersion bool
}

// Rename renames every file inputs match to output (see this file's
// opening comment), returning one result per file, or per input that
// matched nothing. It never fails as a whole; each result says how its
// file fared.
func (s *Session) Rename(inputs []string, output string, opts RenameOptions) []RenamedFile {
	ctx := &Context{Mounts: s.Mounts, Logicals: s.Logicals, Session: s}

	var (
		results []RenamedFile
		sticky  *filespec.Spec
	)

	for _, input := range inputs {
		sources, failed, next := s.findRenameInputs(input, sticky)
		if next != nil {
			sticky = next
		}

		if failed != nil {
			results = append(results, *failed)

			continue
		}

		for _, src := range sources {
			results = append(results, s.renameOne(ctx, src, output, opts))
		}
	}

	return results
}

// findRenameInputs resolves one input and searches for the files it
// names: every match, in every element of a search list, when it has
// wildcards, and otherwise the first element's file that exists, as
// $SEARCH does. sticky, when set, is the previous input's device and
// directory, the defaults for an input that names no device. It returns
// the files, or the search failure to report, and this input's device and
// directory for the next.
func (s *Session) findRenameInputs(input string, sticky *filespec.Spec) ([]renameSource, *RenamedFile, *filespec.Spec) {
	fail := func(expanded string, sts uint32) *RenamedFile {
		return &RenamedFile{Old: expanded, Status: sts, STV: sts, Stage: RenameSearching}
	}

	specs, err := s.expandRenameInput(input, sticky)
	if err != nil {
		var lne *LogicalNameError
		if errors.As(err, &lne) {
			return nil, fail(input, rmsLogicalNameError), nil
		}

		return nil, fail(input, rmsSyntaxError), nil
	}

	first := specs[0].Spec
	next := &filespec.Spec{Device: first.Device, Dirs: first.Dirs}

	var sources []renameSource

	sts := rmsFileNotFound
	expanded := resultName(specs[0].Display, first)

	for _, r := range specs {
		wild := hasWildcard(r.Spec)

		found, elementSts := searchRenameElement(s, r)
		if len(found) == 0 {
			sts, expanded = elementSts, resultName(r.Display, r.Spec)

			continue
		}

		for _, m := range found {
			sources = append(sources, renameSource{
				spec: filespec.Spec{
					Device:  r.Spec.Device,
					Dirs:    m.Dirs,
					Name:    m.Name,
					Type:    m.Type,
					Version: strconv.Itoa(int(m.Version)),
				},
				display:         r.Display,
				wildVersion:     r.Spec.Version == "*",
				explicitVersion: r.Spec.Version != "" && r.Spec.Version != "*" && r.Spec.Version != "0",
			})
		}

		if !wild {
			break
		}
	}

	if len(sources) == 0 {
		return nil, fail(expanded, sts), next
	}

	return sources, nil, next
}

// expandRenameInput resolves one input: against the process defaults, or,
// when sticky is set and the input names no device of its own, against
// sticky's device and directory.
func (s *Session) expandRenameInput(input string, sticky *filespec.Spec) ([]resolvedSpec, error) {
	if sticky == nil {
		return expandSpec(s.Logicals, input, s.Default)
	}

	fs, err := translateSpec(s.Logicals, input)
	if err != nil {
		return nil, err
	}

	probe, err := parseTranslated(fs[0], filespec.Spec{})
	if err != nil {
		return nil, err
	}

	if probe.Device != "" {
		return expandSpec(s.Logicals, input, s.Default)
	}

	var out []resolvedSpec

	for _, f := range fs {
		spec, err := parseTranslated(f, *sticky)
		if err != nil {
			return nil, err
		}

		out = append(out, resolvedSpec{Spec: spec, Display: spec.Device})
	}

	return out, nil
}

// searchRenameElement finds the files one resolved input names, or the
// status saying why there are none: RMS$_DNR (not mounted), RMS$_DNF (no
// such directory), or RMS$_FNF.
func searchRenameElement(s *Session, r resolvedSpec) ([]filespec.Match, uint32) {
	vol, ok := s.Mounts.Lookup(r.Spec.Device)
	if !ok {
		return nil, rmsDeviceNotReady
	}

	if !r.Spec.Recursive && !strings.ContainsAny(strings.Join(r.Spec.Dirs, "."), "*%") {
		if _, err := filespec.ResolveDirectory(vol, r.Spec.Dirs); err != nil {
			return nil, rmsDirNotFound
		}
	}

	matches, err := filespec.Glob(vol, r.Spec)
	if err != nil {
		return nil, rmsDirNotFound
	}

	if len(matches) == 0 {
		return nil, rmsFileNotFound
	}

	return matches, 0
}

// renameOne works out src's new name from output and renames it.
func (s *Session) renameOne(ctx *Context, src renameSource, output string, opts RenameOptions) RenamedFile {
	result := RenamedFile{Old: resultName(src.display, src.spec)}

	target, sts := s.renameTarget(src, output, opts)
	if sts != 0 {
		result.New, result.Status, result.STV, result.Stage = output, sts, sts, RenameParsing
		if target.Device != "" {
			result.New = resultName(target.Device, target)
		}

		return result
	}

	result.New = resultName(target.Device, target)

	r, sts, stv := renameText(ctx, src.spec.String(), target.String())
	result.Status, result.STV, result.Stage = sts, stv, RenameRenaming

	if sts == rmsNormal {
		target.Version = strconv.Itoa(int(r.Version))
		result.New = resultName(target.Device, target)
	}

	return result
}

// renameTarget parses output with src as its related file, and applies
// the output wildcards and the version rules (see this file's opening
// comment). A search list in output uses its first element. On RMS$_WLD
// the parsed target is still returned, for the message to show.
func (s *Session) renameTarget(src renameSource, output string, opts RenameOptions) (filespec.Spec, uint32) {
	fs, err := translateSpec(s.Logicals, output)
	if err != nil {
		return filespec.Spec{}, rmsLogicalNameError
	}

	old := src.spec
	related := filespec.Spec{Device: old.Device, Dirs: old.Dirs, Name: old.Name, Type: old.Type}

	target, err := parseTranslated(fs[0], related)
	if err != nil {
		return filespec.Spec{}, rmsSyntaxError
	}

	if target.Name == "*" {
		target.Name = old.Name
	}

	if target.Type == "*" {
		target.Type = old.Type
	}

	switch {
	case target.Version == "*":
		target.Version = old.Version
	case target.Version == "" && (src.wildVersion || src.explicitVersion || !opts.NewVersion):
		target.Version = old.Version
	}

	if hasWildcard(target) {
		return target, rmsWildcardError
	}

	return target, 0
}

// resultName is spec as RMS shows a resultant name, on device:
// "DUA0:[DIR]NAME.TYP;VER" -- the "." and ";" are always there, even with
// no type or version.
func resultName(device string, spec filespec.Spec) string {
	var b strings.Builder

	if device != "" {
		b.WriteString(device)
		b.WriteByte(':')
	}

	b.WriteByte('[')

	if len(spec.Dirs) == 0 {
		b.WriteString("000000")
	} else {
		b.WriteString(strings.Join(spec.Dirs, "."))
	}

	if spec.Recursive {
		b.WriteString("...")
	}

	b.WriteByte(']')
	b.WriteString(spec.Name)
	b.WriteByte('.')
	b.WriteString(spec.Type)
	b.WriteByte(';')
	b.WriteString(spec.Version)

	return b.String()
}
