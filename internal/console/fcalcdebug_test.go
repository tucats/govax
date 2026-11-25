package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// fcalcDebugMasks are what a change to fcalc or the compiler moves in the
// debugger's session, each made a word: SHOW CALLS's line and PCs, the MACRO
// line and instruction under a break, and a break's FORTRAN line number (its
// statement's text stays). The names, the values, and the calls' order are
// compared as they are.
var fcalcDebugMasks = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`(?m)^(!?\*\S+ +\S+) +\d+ +[0-9A-F]{8} +[0-9A-F]{8} *$`), "$1  line  rel-PC  abs-PC"},
	{regexp.MustCompile(`(?m)^(!?) +\d+: +\S.*$`), "$1  line: instruction"},
	{regexp.MustCompile(`FORTRAN line \d+:`), "FORTRAN line n:"},
}

// TestFcalcDebugger runs fcalc under the debugger (docs/planning/PHASE-111 -
// Fortran 90 milestone.md, subtask 9): testdata/fcalc's FCALCD.COM builds it
// /DEBUG and runs it with DBG$INPUT naming FCALCD.DBG, which breaks at each
// level of a user's function's recursion, walks the expression trees through
// their pointers, shows the calls, and logs itself. The session's log,
// FCALCD.DLG, and the procedure's, FCALCD.LOG, are held to fcalcd.dlg and
// fcalcd.out, fcalcDebugMasks applied to both; FCALCD_WRITE=1 writes them
// instead.
func TestFcalcDebugger(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	dir := filepath.Join("..", "..", "testdata", "fcalc")
	fcalcSources := filepath.Join("..", "fcalc", "src")

	order, err := os.ReadFile(filepath.Join(fcalcSources, "compiles.txt"))
	if err != nil {
		t.Fatal(err)
	}

	// The procedure and the session (testdata/fcalc) and the sources
	// (internal/fcalc/src), gathered in one directory for runProbeProcedure.
	gathered := t.TempDir()
	sources := []string{"fcalcd.com", "fcalcd.dbg"}
	from := map[string]string{"fcalcd.com": dir, "fcalcd.dbg": dir}

	for name := range strings.FieldsSeq(string(order)) {
		sources = append(sources, strings.ToLower(name)+".f90")
		from[strings.ToLower(name)+".f90"] = fcalcSources
	}

	for _, name := range sources {
		data, err := os.ReadFile(filepath.Join(from[name], name))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(gathered, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := runProbeProcedure(t, gathered, sources, "FCALCD", []string{"fcalcd"})

	for _, file := range []struct{ made, held string }{
		{"fcalcd.dlg", "fcalcd.dlg"},
		{"fcalcd.log", "fcalcd.out"},
	} {
		data, err := os.ReadFile(filepath.Join(out, file.made))
		if err != nil {
			t.Fatal(err)
		}

		got := string(data)
		for _, mask := range fcalcDebugMasks {
			got = mask.pattern.ReplaceAllString(got, mask.with)
		}

		held := filepath.Join(dir, file.held)

		if os.Getenv("FCALCD_WRITE") != "" {
			if err := os.WriteFile(held, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}

			continue
		}

		want, err := os.ReadFile(held)
		if err != nil {
			t.Fatal(err)
		}

		if got != string(want) {
			t.Errorf("%s differs from %s:\n%s", file.made, file.held, firstDifference(string(want), got))
		}
	}
}
