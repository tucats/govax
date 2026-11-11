package console_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// rtl89Dir holds Phase 89's programs of FTNRTL under recursion and ASTs
// (docs/planning/PHASE-89 - reentrant FORTRAN.md, subtask 8).
const rtl89Dir = "../../testdata/rtl89/"

// rtl89Run copies a program's files to a new volume, compiles the FORTRAN ones
// /RECURSIVE or not as given, assembles the MACRO ones, links them with the first
// as the image's name, and returns what RUN prints.
func rtl89Run(t *testing.T, qualifier string, names ...string) string {
	t.Helper()

	_, dispatch := wsDispatcher(t)

	dsk := filepath.Join(t.TempDir(), "rtl89.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 RTL89`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	var modules []string

	for _, name := range names {
		host, err := filepath.Abs(rtl89Dir + strings.ToLower(name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + host + `"/HOST ` + name)

		module, kind, _ := strings.Cut(name, ".")
		modules = append(modules, module)

		command := "MACRO " + module
		if kind == "FOR" {
			command = "FORTRAN" + qualifier + " " + module + "\nMACRO " + module
		}

		for _, line := range strings.Split(command, "\n") {
			if out := dispatch(line); out != "" {
				t.Fatalf("%s: %s", line, out)
			}
		}
	}

	if out := dispatch("LINK " + strings.Join(modules, ",")); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	return dispatch("RUN " + modules[0])
}

// TestRTL89FormatCache runs fmtrace.for, whose FORMAT is used again by an AST at
// each instruction of its first statement in turn, the parsed format's cell emptied
// before each: some ASTs come while the main line is parsing it, and then both
// statements parse it, and the copy that doesn't go in the cell must be freed.
func TestRTL89FormatCache(t *testing.T) {
	for _, qualifier := range []string{"/NOOPTIMIZE", "/OPTIMIZE"} {
		t.Run(qualifier, func(t *testing.T) {
			got := rtl89Run(t, qualifier, "FMTRACE.FOR", "RTL89U.MAR")
			want := "RUNS T|RACES T|LOST           0|BAD           0"

			if got != want {
				t.Errorf("RUN FMTRACE printed:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestRTL89SameFrames runs gone89.for: after an unwind out of a statement, the
// recursion goes down again through the same frames, and a statement on the gone
// statement's unit from deeper than it is RECIO_OPE, FTNRTL's one known difference
// from FORRTL (docs/DEVIATIONS.md). If this changes, the difference may be gone:
// see for83GoneSameFrame too.
func TestRTL89SameFrames(t *testing.T) {
	got := rtl89Run(t, "", "GONE89.FOR")
	want := "DEEPER          40|AFTER           0"

	if got != want {
		t.Errorf("RUN GONE89 printed:\n%s\nwant:\n%s", got, want)
	}
}
