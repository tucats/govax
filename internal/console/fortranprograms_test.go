package console

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// The FORTRAN end-to-end harness (docs/PHASE-75 - FORTRAN first
// light.md, subtask 8): each testdata/fortran/name.for is compiled by
// FORTRAN, assembled by MACRO, linked by LINK against FTNRTL, and run,
// and what it prints is compared with name.out, the expected terminal
// output followed by SHOW SYMBOL's line for $STATUS. FORTRAN, MACRO, and
// LINK must say nothing. To add a program, add the two files; the
// expected output is written by hand from the language's rules, never
// copied from a run.

// fortranPrograms is the harness's directory.
var fortranPrograms = filepath.Join("..", "..", "testdata", "fortran")

// fortranPagingPrograms are the programs also run with paging on
// (the plan's Decision 20: both paging modes for a subset).
var fortranPagingPrograms = []string{"args"}

func TestFortranPrograms(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join(fortranPrograms, "*.for"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no programs in %s (%v)", fortranPrograms, err)
	}

	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		for _, source := range sources {
			name := strings.TrimSuffix(filepath.Base(source), ".for")
			if paging && !slices.Contains(fortranPagingPrograms, name) {
				continue
			}

			t.Run(name, func(t *testing.T) {
				text, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}

				want, err := os.ReadFile(strings.TrimSuffix(source, ".for") + ".out")
				if err != nil {
					t.Fatal(err)
				}

				upper := strings.ToUpper(name)
				if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + upper + ".FOR"},
					rms.TextRecords, splitSource(string(text))); err != nil {
					t.Fatal(err)
				}

				buf.Reset()

				for _, command := range []string{"FORTRAN ", "MACRO ", "LINK "} {
					shareFindRun(t, d, command+upper)

					if out := buf.String(); out != "" {
						t.Fatalf("%s%s said:\n%s", command, upper, out)
					}
				}

				if got := shareRun(t, d, buf, upper); got != string(want) {
					t.Errorf("RUN %s printed:\n%s\nwant:\n%s", upper, got, want)
				}
			})
		}
	})
}
