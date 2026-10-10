package rms

import (
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// What DCL's F$PARSE and F$SEARCH lexical functions ask of RMS
// (docs/PHASE-50 - DCL command procedures.md, subtask 14): a file
// specification taken apart, with the defaults RMS's $PARSE applies, and
// the files a specification (wildcards and all) names, as $SEARCH finds
// them.

// ParsedName is a file specification taken apart, each field written as
// RMS writes it in an expanded string: "DUA0:", "[WORK]", "LOGIN",
// ".COM", ";" (no version given) or ";3". Node is always "": govax has no
// network.
type ParsedName struct {
	Node, Device, Directory, Name, Type, Version string
}

// String is the whole specification: "DUA0:[WORK]LOGIN.COM;".
func (p ParsedName) String() string {
	return p.Node + p.Device + p.Directory + p.Name + p.Type + p.Version
}

// Parse takes spec apart as F$PARSE does: its logical names translated,
// and each field it leaves out taken from def (a default specification,
// "" for none), then the name and type from related (a related file),
// then the device (SYS$DISK) and directory of the process's default.
// The device is shown as the user named it: a concealed logical name
// stays, unless noConceal. Unless syntaxOnly, the device must be mounted
// and the directory (one with no wildcards) must be on it; found is false
// when one isn't. A specification RMS can't parse is an error. With no
// device at all (no SYS$DISK, as when the default is on the host), there
// is nothing to check, and the fields are returned.
func (s *Session) Parse(spec, def, related string, syntaxOnly, noConceal bool) (p ParsedName, found bool, err error) {
	var defaults resolvedSpec

	if def != "" {
		specs, err := expandSpec(s.Logicals, def, s.Default)
		if err != nil {
			return ParsedName{}, false, err
		}

		defaults = specs[0]
	} else {
		specs, err := defaultSpecs(s.Logicals, s.Default)
		if err != nil {
			return ParsedName{}, false, err
		}

		defaults = resolvedSpec{Spec: specs[0], Display: specs[0].Device}
	}

	if related != "" {
		rel, err := filespec.Parse(related, filespec.Spec{})
		if err != nil {
			return ParsedName{}, false, err
		}

		if defaults.Spec.Name == "" {
			defaults.Spec.Name = rel.Name
		}

		if defaults.Spec.Type == "" {
			defaults.Spec.Type = rel.Type
		}
	}

	fs, err := translateSpec(s.Logicals, spec)
	if err != nil {
		return ParsedName{}, false, err
	}

	own, err := parseTranslated(fs[0], filespec.Spec{Dirs: s.Default.Dirs})
	if err != nil {
		return ParsedName{}, false, err
	}

	full, err := parseTranslated(fs[0], defaults.Spec)
	if err != nil {
		return ParsedName{}, false, err
	}

	display := full.Device

	switch {
	case noConceal:
	case fs[0].Concealed != "":
		display = fs[0].Concealed
	case own.Device == "" && defaults.Display != "":
		display = defaults.Display
	}

	p = ParsedName{
		Name:      full.Name,
		Type:      "." + full.Type,
		Version:   ";" + full.Version,
		Directory: filespec.Spec{Dirs: full.Dirs, Recursive: full.Recursive}.String(),
	}

	if display != "" {
		p.Device = display + ":"
	}

	if syntaxOnly || full.Device == "" {
		return p, true, nil
	}

	vol, ok := s.Mounts.Lookup(full.Device)
	if !ok {
		return p, false, nil
	}

	if !full.Recursive && !strings.ContainsAny(strings.Join(full.Dirs, "."), "*%") {
		if _, err := filespec.ResolveDirectory(vol, full.Dirs); err != nil {
			return p, false, nil
		}
	}

	return p, true, nil
}

// Search returns every file spec names, as successive calls of F$SEARCH
// (or $SEARCH) find them: each element of a search list in turn, and in
// each directory the files in directory order, the newest version only
// unless spec gives one. A field spec leaves out is not a wildcard: with
// no type, only files with an empty type match, as $SEARCH matches them
// with no default name. Each is written with its device as the user
// named it ("DUA0:[WORK]LOGIN.COM;3"). A device that isn't mounted, or a
// directory that isn't there (or that ods2 can't search), adds nothing;
// a specification that can't be parsed is an error.
func (s *Session) Search(spec string) ([]string, error) {
	var names []string

	err := s.eachSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		// A directory that isn't there holds no files.
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return nil
		}

		for _, m := range matches {
			if (r.Spec.Name == "" && m.Name != "") || (r.Spec.Type == "" && m.Type != "") {
				continue
			}

			names = append(names, r.Display+":"+m.String())
		}

		return nil
	})

	if err != nil && !isNotFound(err) {
		return nil, err
	}

	return names, nil
}

// DefaultDirectory is the default directory alone, as F$DIRECTORY returns
// it: "[WORK]", or "[000000]" for the master file directory.
func (s *Session) DefaultDirectory() string {
	return filespec.Spec{Dirs: s.Default.Dirs}.String()
}
