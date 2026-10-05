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
	}

	for _, c := range cases {
		if got := analyzeCommand(c.files, c.flags); got != c.want {
			t.Errorf("analyzeCommand(%v, %+v) = %q, want %q", c.files, c.flags, got, c.want)
		}
	}
}

// TestRun_analyzeOneShot analyzes a real VMS object with a one-shot
// command, as govax analyze would.
func TestRun_analyzeOneShot(t *testing.T) {
	out := filepath.Join(t.TempDir(), "Hello.Anl")
	command := analyzeCommand([]string{"../../testdata/mar/vax/hello.obj"}, analyzeFlags{output: true, outputFile: out})

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
