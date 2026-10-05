package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeCommand(t *testing.T) {
	cases := []struct {
		files []string
		flags analyzeFlags
		want  string
	}{
		{[]string{"a.obj"}, analyzeFlags{}, `ANALYZE/OBJECT "a.obj"`},
		{[]string{"a", "b"}, analyzeFlags{output: true, records: []string{"gsd", "eom"}}, `ANALYZE/OBJECT/GSD/EOM/OUTPUT "a","b"`},
		{[]string{"x.olb"}, analyzeFlags{output: true, outputFile: "x.anl", include: []string{"M*"}}, `ANALYZE/OBJECT/OUTPUT="x.anl"/INCLUDE=("M*") "x.olb"`},
		{[]string{"p.exe"}, analyzeFlags{image: true}, `ANALYZE/IMAGE "p.exe"`},
		{[]string{"p"}, analyzeFlags{image: true, header: true, output: true}, `ANALYZE/IMAGE/HEADER/OUTPUT "p"`},
		{[]string{"p"}, analyzeFlags{image: true, fixups: true}, `ANALYZE/IMAGE/FIXUP_SECTION "p"`},
	}

	for _, c := range cases {
		got, err := analyzeCommand(c.files, c.flags)
		if err != nil || got != c.want {
			t.Errorf("analyzeCommand(%v, %+v) = %q, %v, want %q", c.files, c.flags, got, err, c.want)
		}
	}

	// Options of the other kind of analysis.
	for _, f := range []analyzeFlags{
		{image: true, records: []string{"gsd"}},
		{image: true, include: []string{"M"}},
		{header: true},
		{fixups: true},
	} {
		if got, err := analyzeCommand([]string{"a"}, f); err == nil {
			t.Errorf("analyzeCommand(%+v) = %q, want an error", f, got)
		}
	}
}

// TestRun_analyzeOneShot analyzes a real VMS object with a one-shot
// command, as govax analyze would.
func TestRun_analyzeOneShot(t *testing.T) {
	out := filepath.Join(t.TempDir(), "Hello.Anl")
	command, err := analyzeCommand([]string{"../../testdata/mar/vax/hello.obj"}, analyzeFlags{output: true, outputFile: out})
	if err != nil {
		t.Fatal(err)
	}

	if err := run(nil, 0, 0, os.Stdout, nil, []string{command}); err != nil {
		t.Fatalf("%s: %v", command, err)
	}

	text, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(text), "The analysis uncovered NO errors.") {
		t.Errorf("the report:\n%s", text)
	}
}

// TestRun_analyzeImageOneShot analyzes a real VMS image with a one-shot
// command, as govax analyze --image would.
func TestRun_analyzeImageOneShot(t *testing.T) {
	out := filepath.Join(t.TempDir(), "Prog.Ani")

	command, err := analyzeCommand([]string{"../../testdata/link/vax/prog.exe"}, analyzeFlags{image: true, output: true, outputFile: out})
	if err != nil {
		t.Fatal(err)
	}

	if err := run(nil, 0, 0, os.Stdout, nil, []string{command}); err != nil {
		t.Fatalf("%s: %v", command, err)
	}

	text, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"\f\nAnalyze Image ", "IMAGE ACTIVATOR FIXUP SECTION", "The analysis uncovered NO errors."} {
		if !strings.Contains(string(text), want) {
			t.Errorf("no %q in the report:\n%s", want, text)
		}
	}
}
