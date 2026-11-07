package bootdata

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
)

// TestHelpLibrariesMatchSources: each committed help library is what its
// help files build, so the two can't drift apart. If this fails, run
// "go generate ./internal/bootdata".
func TestHelpLibrariesMatchSources(t *testing.T) {
	for name, dir := range HelpLibraries {
		want, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatal(err)
		}

		got, err := BuildHelp(FS, dir)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("%s doesn't match its sources in %s: run go generate ./internal/bootdata", name, dir)
		}
	}
}

// TestHelpModulesWellFormed is Decision 10's last check on govax's help
// files: every module parses, each key line is at most one level below
// the one before it (LIBRARY's ILLKEYLVL), no two subtopics of a topic have
// the same key, no line of text would be read as a key line or a
// qualifier line by accident (a digit or a "/" in column 1), and each
// library reads back as a help library holding every module.
func TestHelpModulesWellFormed(t *testing.T) {
	for name, dir := range HelpLibraries {
		entries, err := HelpModules(FS, dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}

		if len(entries) == 0 {
			t.Fatalf("%s: no modules", dir)
		}

		for _, e := range entries {
			last := 0

			for i, rec := range e.Records {
				level, key, ok := lbr.HelpKeyLine(rec)
				if !ok {
					if len(rec) > 0 && (rec[0] == '/' || rec[0] >= '0' && rec[0] <= '9') {
						t.Errorf("%s module %s, record %d: %q starts like a key or qualifier line", dir, e.Name, i+1, rec)
					}

					continue
				}

				if level > last+1 {
					t.Errorf("%s module %s: key %s at level %d after level %d", dir, e.Name, key, level, last)
				}

				last = level
			}

			top := help.ParseModule(e.Records)
			if top == nil {
				t.Errorf("%s module %s doesn't parse", dir, e.Name)

				continue
			}

			checkSiblings(t, dir+" "+e.Name, top)
		}

		data, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatal(err)
		}

		l, err := lbr.Open(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if l.Type != lbr.TypeHelp {
			t.Fatalf("%s is a %s library", name, l.Type)
		}

		if got := len(l.Indexes[0].Keys); got != len(entries) {
			t.Errorf("%s has %d modules, its sources %d", name, got, len(entries))
		}
	}
}

// checkSiblings reports subtopics of t, and of each topic below it, that
// share a key.
func checkSiblings(t *testing.T, where string, top *help.Topic) {
	t.Helper()

	seen := map[string]bool{}

	for _, s := range top.Subtopics {
		k := strings.ToUpper(s.Key)
		if seen[k] {
			t.Errorf("%s %s: two subtopics %s", where, top.Key, s.Key)
		}

		seen[k] = true

		checkSiblings(t, where, s)
	}
}

// TestDebugHelpLibraryShape: the debugger's library is VMS's shape, one
// DEBUG module (Decision 2).
func TestDebugHelpLibraryShape(t *testing.T) {
	entries, err := HelpModules(FS, DebugHelpSourceDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name != "DEBUG" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name)
		}

		t.Fatalf("modules %v, want DEBUG alone", names)
	}

	if got := HelpLibraryName(DebugHelpLibrary); got != "DBG$HELP.HLB" {
		t.Errorf("name %s", got)
	}
}
