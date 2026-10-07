package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/tucats/govax/internal/console"
)

// Phase 45's subtask 13: testdata/mp/crechild.mar, a MACRO program that
// creates a subprocess with a termination mailbox, looks at it with
// $GETJPI, wakes it, and reports the status its termination message
// carries. Assembled and linked by govax, and run as process 1's image.

// TestCreChild runs CRECHILD.EXE, giving it CHILD.EXE on its command
// line, and checks everything it prints.
func TestCreChild(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	read := func(name string) string {
		t.Helper()

		b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", name))
		if err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	child := buildImage(t, c, "child", read("child.mar"))
	parent := buildImage(t, c, "crechild", read("crechild.mar"))

	out.Reset()

	if err := c.Run(parent, RunOptions{CommandLine: child}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	want := strings.Join([]string{
		"Child: running",
		"Parent: child is in state 7, named CHILD_1",
		"Parent: child's owner and master are this process",
		"Child: awake",
		"Parent: child ended with status 00000007",
	}, "\n")

	// The vax.init debug settings also print DEBUG lines, which aren't
	// the program's.
	var lines []string

	for _, line := range strings.Split(strings.ReplaceAll(out.String(), "\r", ""), "\n") {
		if line != "" && !strings.HasPrefix(line, "DEBUG") {
			lines = append(lines, line)
		}
	}

	if got := strings.Join(lines, "\n"); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes left, want only process 1", n)
	}
}
