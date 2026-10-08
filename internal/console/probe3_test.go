package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/tucats/govax/internal/console"
)

// Phase 46's probe 3 (testdata/mp/probe3), run under govax: the program
// VMS is also asked to run. Its report is in the -v output, for the
// side-by-side with the VMS log.
func TestProbe3(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	read := func(path string) string {
		t.Helper()

		b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", path))
		if err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	child := buildImage(t, c, "probe3c", read("probe3/probe3c.mar"))
	probe := buildImage(t, c, "probe3", read("probe3/probe3.mar"))

	out.Reset()

	if err := c.Run(probe, RunOptions{CommandLine: child}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	var lines []string

	for _, line := range strings.Split(strings.ReplaceAll(out.String(), "\r", ""), "\n") {
		if line != "" && !strings.HasPrefix(line, "DEBUG") {
			lines = append(lines, line)
		}
	}

	report := strings.Join(lines, "\n")
	t.Logf("\n%s", report)

	for _, want := range []string{
		"Part 1:", "1.7 the child's final status", "Part 2:", "Part 3:", "Done",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q", want)
		}
	}

	if strings.Contains(report, "FAILED") || strings.Contains(report, "TIMED OUT") {
		t.Error("a setup step failed")
	}
}
