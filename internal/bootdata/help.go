package bootdata

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmsdef"
)

//go:generate go run ./mkhelp

// govax's help libraries (docs/planning/PHASE-90 - VMS help.md, Decisions 1 and 2):
// HELPLIB.HLB, DCL's HELP, and DBG$HELP.HLB, the debugger's. Each is built
// from a directory of help files in VMS's format (.HLP files, a module for
// each level-1 key), one file per level-1 topic, as LIBRARY/CREATE/HELP
// builds a library from them. Like STARLET.MLB, the libraries are generated
// ("go generate ./internal/bootdata", mkhelp) and committed beside their
// sources, and a test rebuilds them and checks that they match.
//
// The debugger's library has one module, DEBUG, with each command a level-2
// topic below it, as VMS's DBG$HELP.HLB has.
const (
	HelpLibrary          = "helplib.hlb"
	HelpSourceDir        = "help"
	DebugHelpLibrary     = "dbghelp.hlb"
	DebugHelpSourceDir   = "dbghelp"
	helpSourceType       = ".hlp"
	debugHelpLibraryName = "DBG$HELP.HLB"
)

// HelpLibraryName is a help library's name on the system volume and in
// SYS$HELP: its embedded file name as VMS names it ("dbghelp.hlb" is
// DBG$HELP.HLB).
func HelpLibraryName(embedded string) string {
	if embedded == DebugHelpLibrary {
		return debugHelpLibraryName
	}

	return strings.ToUpper(embedded)
}

// HelpLibraries maps each help library's embedded name to the directory of
// its sources.
var HelpLibraries = map[string]string{
	HelpLibrary:      HelpSourceDir,
	DebugHelpLibrary: DebugHelpSourceDir,
}

// helpTime is the help libraries' creation, revision, and insertion time,
// fixed so that a library doesn't change unless its sources do.
var helpTime = time.Date(2026, time.November, 7, 0, 0, 0, 0, time.UTC)

// HelpTime is helpTime as a VMS time, for the system volume's refresh,
// which inserts govax's modules with it.
func HelpTime() uint64 { return vmsdef.Time(helpTime) }

// HelpSource is one help file: its name ("copy.hlp") and its lines.
type HelpSource struct {
	Name  string
	Lines [][]byte
}

// HelpSources returns the help files in the directory dir of fsys, sorted
// by name in upper case, the order a wildcard search of a VMS directory
// gives them in (BUILDHELP.COM's LIBRARY/CREATE/HELP reads them so).
func HelpSources(fsys fs.FS, dir string) ([]HelpSource, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}

	var out []HelpSource

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), helpSourceType) {
			continue
		}

		data, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}

		text := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

		var lines [][]byte
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, []byte(line))
		}

		out = append(out, HelpSource{Name: e.Name(), Lines: lines})
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToUpper(out[i].Name) < strings.ToUpper(out[j].Name)
	})

	return out, nil
}

// HelpModules returns the modules of the help files in dir, in the order
// the files give them. A bad key line or a module two files define is an
// error, since govax's own files should have neither.
func HelpModules(fsys fs.FS, dir string) ([]*lbr.Entry, error) {
	sources, err := HelpSources(fsys, dir)
	if err != nil {
		return nil, err
	}

	b, err := lbr.Create(lbr.TypeHelp)
	if err != nil {
		return nil, err
	}

	var out []*lbr.Entry

	seen := map[string]string{}

	for _, src := range sources {
		entries, bad := b.HelpModules(src.Lines)
		if bad != nil {
			return nil, fmt.Errorf("%s: %w", src.Name, bad)
		}

		for _, e := range entries {
			key := strings.ToUpper(e.Name)
			if prev, dup := seen[key]; dup {
				return nil, fmt.Errorf("%s: module %s is also in %s", src.Name, e.Name, prev)
			}

			seen[key] = src.Name
			out = append(out, e)
		}
	}

	return out, nil
}

// BuildHelp builds the help library whose sources are the directory dir of
// fsys, as LIBRARY/CREATE/HELP would at govax's fixed time.
func BuildHelp(fsys fs.FS, dir string) ([]byte, error) {
	return BuildHelpAt(fsys, dir, HelpTime())
}

// BuildHelpAt is BuildHelp with when, a VMS time, as the library's
// creation, revision, and insertion time: what LIBRARY/CREATE/HELP makes of
// the same files at that time (BUILDHELP.COM's libraries are held to it).
func BuildHelpAt(fsys fs.FS, dir string, when uint64) ([]byte, error) {
	entries, err := HelpModules(fsys, dir)
	if err != nil {
		return nil, err
	}

	b, err := lbr.Create(lbr.TypeHelp)
	if err != nil {
		return nil, err
	}

	b.Created, b.Updated = when, when

	for _, e := range entries {
		e.Inserted = when

		if err := b.Insert(e); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
	}

	return b.Bytes(), nil
}
