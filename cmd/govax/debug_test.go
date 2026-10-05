package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRun_debugSession drives the console loop with a script that enters
// the debugger, uses its HELP, leaves it, and quits: the wiring cmd/govax
// adds (the second grammar and help file, and the installed debugger)
// reaches every part. The prompt itself is readline's and isn't captured
// here; internal/debugger's tests cover which one is chosen.
func TestRun_debugSession(t *testing.T) {
	script := strings.Join([]string{
		"DEBUG",
		"HELP EXIT",
		"DIRECTORY", // a console command at DBG>
		"EXIT",      // the debugger's: back to the console
		"EXIT",      // the console's: leaves govax
	}, "\n") + "\n"

	var buf bytes.Buffer

	if err := run(nil, 0, 0, &buf, readCloser{strings.NewReader(script)}, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := buf.String()

	for _, want := range []string{
		"Ends the debugger session",
		"%DEBUG-E-SYNTAX, command syntax error at or near 'DIRECTORY'",
		"is a console command; EXIT returns to the console",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q", want)
		}
	}

	// The refused DIRECTORY must not have run, and the failure must not
	// have been printed a second time by the loop.
	if n := strings.Count(got, "command syntax error"); n != 1 {
		t.Errorf("the syntax error was shown %d times, want 1", n)
	}
}

// TestRunCommandDebugQualifiers: the run subcommand's --debug and
// --no-debug become RUN's /DEBUG and /NODEBUG; with both, --debug wins.
func TestRunCommandDebugQualifiers(t *testing.T) {
	cases := []struct {
		flags runFlags
		want  string
	}{
		{runFlags{}, `run "x.exe"`},
		{runFlags{debug: true}, `run "x.exe"/DEBUG`},
		{runFlags{noDebug: true}, `run "x.exe"/NODEBUG`},
		{runFlags{debug: true, noDebug: true}, `run "x.exe"/DEBUG`},
	}

	for _, tc := range cases {
		if got := runCommand("x.exe", tc.flags); got != tc.want {
			t.Errorf("runCommand(%+v) = %q, want %q", tc.flags, got, tc.want)
		}
	}
}
