package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/tucats/govax/internal/console"
)

// Phase 46's subtask 8: testdata/mp/mbxpingpong.mar, a MACRO program
// that creates a subprocess (testdata/mp/mbxpong.mar) and exchanges
// messages with it both ways through two temporary mailboxes, which the
// child finds by their logical names, with a handshake on a common event
// flag cluster. Assembled and linked by govax, and run as process 1's
// image under the scheduler.

// TestMbxPingPong runs MBXPINGPONG.EXE, giving it MBXPONG.EXE on its
// command line, and checks everything both processes print.
func TestMbxPingPong(t *testing.T) {
	for _, tc := range []struct {
		name    string
		quantum string
	}{
		// A long quantum: each process runs until it waits.
		{"long quantum", longQuantum},
		// A 5-instruction quantum: the CPU changes process in the middle
		// of nearly everything either does, which mustn't change the
		// output.
		{"short quantum", "5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, out := scheduledConsole(t, tc.quantum, brbSelf)

			read := func(name string) string {
				t.Helper()

				b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", name))
				if err != nil {
					t.Fatal(err)
				}

				return string(b)
			}

			child := buildImage(t, c, "mbxpong", read("mbxpong.mar"))
			parent := buildImage(t, c, "mbxpingpong", read("mbxpingpong.mar"))

			out.Reset()

			if err := c.Run(parent, RunOptions{CommandLine: child}); err != nil {
				t.Fatalf("RUN: %v\n%s", err, out.String())
			}

			want := strings.Join([]string{
				"Child: running",
				"Parent: child is ready",
				"Parent: sending PING 1",
				"Child: got PING 1",
				"Parent: got PONG 1",
				"Parent: sending PING 2",
				"Child: got PING 2",
				"Parent: got PONG 2",
				"Parent: sending PING 3",
				"Child: got PING 3",
				"Parent: got PONG 3",
				"Child: end of file",
				"Parent: child saw end of file",
				"Child: done",
				"Parent: child ended with status 00000001",
			}, "\n")

			// The vax.init debug settings also print DEBUG lines, which
			// aren't the program's.
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
		})
	}
}
