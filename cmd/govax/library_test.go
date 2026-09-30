package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryCommand(t *testing.T) {
	cases := []struct {
		lib    string
		inputs []string
		flags  libraryFlags
		want   string
	}{
		{"x.olb", nil, libraryFlags{list: true}, `LIBRARY "x.olb"/LIST`},
		{"m", []string{"a.mar", "b"}, libraryFlags{create: true, macro: true, log: true}, `LIBRARY "m" "a.mar","b"/CREATE/MACRO/LOG`},
		{"x", []string{"a"}, libraryFlags{insert: true, selective: true, noSqueeze: true}, `LIBRARY "x" "a"/INSERT/NOSQUEEZE/SELECTIVE_SEARCH`},
		{"x", nil, libraryFlags{delete: []string{"a", "B*"}}, `LIBRARY "x"/DELETE=("a","B*")`},
		{"x", nil, libraryFlags{extract: []string{"m"}, output: "o.mar"}, `LIBRARY "x"/EXTRACT=("m")/OUTPUT="o.mar"`},
		{"x", nil, libraryFlags{list: true, listFile: "x.lis", full: true, names: true}, `LIBRARY "x"/LIST="x.lis"/FULL/NAMES`},
	}

	for _, c := range cases {
		if got := libraryCommand(c.lib, c.inputs, c.flags); got != c.want {
			t.Errorf("libraryCommand(%q, %v, %+v) = %q, want %q", c.lib, c.inputs, c.flags, got, c.want)
		}
	}
}

// TestRun_libraryOneShot creates and lists a macro library with one-shot
// commands, as govax library would.
func TestRun_libraryOneShot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Macros.Mar")

	if err := os.WriteFile(src, []byte("\t.MACRO\tclear x\n\tCLRL\tx\n\t.ENDM\tclear\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lib := filepath.Join(dir, "Mine")

	var buf bytes.Buffer

	for _, command := range []string{
		libraryCommand(lib, []string{src}, libraryFlags{create: true, macro: true}),
		libraryCommand(lib+".MLB", nil, libraryFlags{list: true}),
	} {
		if err := run(nil, 0, 0, &buf, emptyStdin(), []string{command}); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, buf.String())
		}
	}

	// The library's type follows the case of the name given it.
	if _, err := os.Stat(filepath.Join(dir, "Mine.MLB")); err != nil {
		t.Errorf("no Mine.MLB: %v", err)
	}

	if !strings.Contains(buf.String(), "Directory of MACRO library") || !strings.Contains(buf.String(), "\nCLEAR\n") {
		t.Errorf("listing:\n%s", buf.String())
	}
}
