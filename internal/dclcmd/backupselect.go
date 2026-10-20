package dclcmd

import (
	"strings"

	"github.com/tucats/govax/internal/lnm"
)

// BACKUP's /SELECT and /EXCLUDE on an input save set (docs/PHASE-69 -
// backup save sets.md, subtask 9): file specifications matched against
// the names the save set holds ("[B69.SUB1]A.TXT;1"), not against a
// directory on a volume, so they are matched here as text.
//
// The manual's rules (System Management Utilities Reference Manual, 7.3.3,
// and /SELECT): any DCL wildcard may be used; an omitted name, type, or
// version is "*"; and ";" (the latest version) and ";-n" match every
// version. What VMS V7.1 did with them (testdata/backup69's log): a
// specification with no directory matches files in any directory
// (/SELECT=*.TXT restored [B69]ACL.TXT and [B69.SUB1]A.TXT, and
// /EXCLUDE=*.DAT left out [B69.SUB1]MEDIUM.DAT); a device is ignored.
//
// A restore to [dir...] carries part of each file's directory below dir
// (7.3.3): with /SELECT, "only the ellipsis portion of the selected file
// specification". So /SELECT=[B69.SUB1...] restored [B69.SUB1.SUB2]B.TXT
// to [R3.SUB2], and /SELECT=*.TXT, taken as [*...]*.TXT, restored
// [B69.SUB1]A.TXT to [R5.SUB1]: the "*" matched B69, which isn't carried.

// ellipsisToken stands for "..." among a pattern's directory components.
const ellipsisToken = "\x00"

// backupPattern is one /SELECT or /EXCLUDE specification, upper case.
type backupPattern struct {
	// dirs are the directory's components, each possibly wild, with
	// ellipsisToken for "..."; anyDir is a specification with no
	// directory, which is [*...].
	dirs   []string
	anyDir bool

	name, typ, version string
}

// parseBackupPattern parses one /SELECT or /EXCLUDE value.
func parseBackupPattern(text string) backupPattern {
	text = strings.ToUpper(strings.TrimSpace(text))

	// A device names nothing in a save set.
	if i := strings.Index(text, ":"); i >= 0 && !strings.ContainsAny(text[:i], "[<") {
		text = text[i+1:]
	}

	var p backupPattern

	if dir, rest, ok := splitBackupDirectory(text); ok {
		p.dirs = directoryComponents(dir)
		text = rest
	} else {
		p.anyDir = true
		p.dirs = []string{"*", ellipsisToken}
	}

	name, version, _ := strings.Cut(text, ";")
	name, typ, _ := strings.Cut(name, ".")

	p.name, p.typ, p.version = orStar(name), orStar(typ), orStar(version)

	// The latest version, -n, or 0: every version (the manual's 7.3.3).
	if strings.HasPrefix(p.version, "-") || p.version == "0" {
		p.version = "*"
	}

	return p
}

// splitBackupDirectory splits text, "[B69.SUB1]A.TXT", into the text
// between its brackets and the rest. ok is false when text has no
// directory.
func splitBackupDirectory(text string) (dir, rest string, ok bool) {
	if text == "" || (text[0] != '[' && text[0] != '<') {
		return "", text, false
	}

	end := strings.IndexAny(text, "]>")
	if end < 0 {
		return text[1:], "", true
	}

	return text[1:end], text[end+1:], true
}

// directoryComponents is a directory's text, "B69.SUB1...", as its
// components, "..." as ellipsisToken. "000000" and nothing at all are the
// master file directory, no components.
func directoryComponents(dir string) []string {
	dir = strings.ReplaceAll(dir, "...", "."+ellipsisToken+".")

	var out []string

	for _, c := range strings.Split(dir, ".") {
		if c == "" || c == "000000" {
			continue
		}

		out = append(out, c)
	}

	return out
}

// orStar is s, or "*" when it's empty.
func orStar(s string) string {
	if s == "" {
		return "*"
	}

	return s
}

// savedName is a name a save set holds, split: "[B69.SUB1]A.TXT;1".
type savedName struct {
	dirs               []string
	name, typ, version string
}

// parseSavedName splits a saved file's name.
func parseSavedName(text string) savedName {
	var s savedName

	dir, rest, _ := splitBackupDirectory(strings.ToUpper(text))
	s.dirs = directoryComponents(dir)

	name, version, _ := strings.Cut(rest, ";")
	s.name, s.typ, _ = strings.Cut(name, ".")
	s.version = version

	return s
}

// match reports whether p matches the saved name s, and the components of
// s's directory its ellipses matched: what a restore to [dir...] carries.
func (p backupPattern) match(s savedName) ([]string, bool) {
	if !lnm.Match(p.name, s.name) || !lnm.Match(p.typ, s.typ) || !lnm.Match(p.version, s.version) {
		return nil, false
	}

	// A save set's [000000] files are in no directory [*...] names.
	if p.anyDir && len(s.dirs) == 0 {
		return nil, true
	}

	return matchDirectories(p.dirs, s.dirs)
}

// matchDirectories matches directory components against a pattern's,
// where ellipsisToken matches any number of components (none included),
// and returns the components the ellipses matched.
func matchDirectories(pattern, dirs []string) ([]string, bool) {
	if len(pattern) == 0 {
		return nil, len(dirs) == 0
	}

	if pattern[0] == ellipsisToken {
		// The longest run first, so a pattern ending in "..." carries
		// everything below its fixed part.
		for n := len(dirs); n >= 0; n-- {
			if rest, ok := matchDirectories(pattern[1:], dirs[n:]); ok {
				return append(append([]string(nil), dirs[:n]...), rest...), true
			}
		}

		return nil, false
	}

	if len(dirs) == 0 || !lnm.Match(pattern[0], dirs[0]) {
		return nil, false
	}

	return matchDirectories(pattern[1:], dirs[1:])
}

// backupSelection is a restore's /SELECT and /EXCLUDE.
type backupSelection struct {
	selects, excludes []backupPattern
}

// newBackupSelection parses /SELECT's and /EXCLUDE's values.
func newBackupSelection(selects, excludes []string) backupSelection {
	var b backupSelection

	for _, s := range selects {
		b.selects = append(b.selects, parseBackupPattern(s))
	}

	for _, e := range excludes {
		b.excludes = append(b.excludes, parseBackupPattern(e))
	}

	return b
}

// selected reports whether the saved file name is restored, and the part
// of its directory a restore to [dir...] carries below dir: all of it
// without /SELECT ("[* . . . ] ... carr[ies] over the entire directory
// name from the first level on"), and with it the part the first
// matching specification's ellipses matched.
func (b backupSelection) selected(name string) ([]string, bool) {
	s := parseSavedName(name)

	for _, e := range b.excludes {
		if _, ok := e.match(s); ok {
			return nil, false
		}
	}

	if len(b.selects) == 0 {
		return s.dirs, true
	}

	for _, p := range b.selects {
		if carried, ok := p.match(s); ok {
			return carried, true
		}
	}

	return nil, false
}
