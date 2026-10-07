package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/tucats/govax/internal/console"
)

// Phase 45's probe 1 (testdata/mp/probe1): the program that VMS 7.3 is
// also asked to run, here under govax, to check it works and to see the
// answers govax gives.

// TestProbe1 runs PROBE1.EXE with CHILD.EXE and INFO.EXE and checks the
// shape of its report. (The VMS run's report, once made, is compared
// by hand: this test is for the program.)
func TestProbe1(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	read := func(path string) string {
		t.Helper()

		b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", path))
		if err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	child := buildImage(t, c, "child", read("child.mar"))
	info := buildImage(t, c, "info", read("probe1/info.mar"))
	probe := buildImage(t, c, "probe1", read("probe1/probe1.mar"))

	out.Reset()

	if err := c.Run(probe, RunOptions{CommandLine: child + " " + info}); err != nil {
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
		"$GETJPI of the child, hibernating:",
		"= 00000007  (status 00000001)", // the child hibernates: SCH$C_HIB
		"$GETJPI of this process:",
		"The child's termination message:",
		"+50  ", // the last longword of the 84-byte message
		"The second child (no input, output, or error):",
		`SYS$OUTPUT = "`,
	} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q", want)
		}
	}
}
