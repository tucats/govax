package debugger_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/respath"
)

// sleepSource hibernates for good.
const sleepSource = `	.title	sleep
	.entry	start,^m<>
	$hiber_s
	ret
	.end	start
`

// TestSetProcess: the debugger's SET PROCESS moves the CPU to the process
// named (by name or PID), so the machine's state the debugger shows is
// that process's; SHOW PROCESS marks it; SET PROCESS alone comes back to
// process 1; a process that isn't there is NONEXPR.
func TestSetProcess(t *testing.T) {
	s := bootedSession(t)

	// OTHER runs an image that hibernates: it stays, waiting (HIB).
	dir := t.TempDir()
	src, obj, exe := filepath.Join(dir, "sleep.mar"), filepath.Join(dir, "sleep.obj"), filepath.Join(dir, "sleep.exe")

	if err := os.WriteFile(src, []byte(sleepSource), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		`MACRO "` + src + `"/OBJECT="` + obj + `"`,
		`LINK "` + obj + `"/EXECUTABLE="` + exe + `"`,
		`SPAWN/NOWAIT/PROCESS=OTHER RUN "` + exe + `"`,
	} {
		if out, err := s.do(t, line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, out)
		}
	}

	var other *corevms.Environment

	for _, env := range s.c.RTL.Processes() {
		if env.Process.Name == "OTHER" {
			other = env
		}
	}

	if other == nil {
		t.Fatal("SPAWN made no process OTHER")
	}

	if _, err := s.do(t, "DEBUG"); err != nil {
		t.Fatal(err)
	}

	marked := func() string {
		t.Helper()

		out, err := s.do(t, "SHOW PROCESS")
		if err != nil {
			t.Fatalf("SHOW PROCESS: %v", err)
		}

		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 3 || !strings.HasPrefix(lines[0], "  Pid    Process Name") {
			t.Fatalf("SHOW PROCESS:\n%s", out)
		}

		for _, line := range lines[1:] {
			if strings.HasPrefix(line, "*") {
				return strings.Fields(line[9:])[0]
			}
		}

		t.Fatalf("SHOW PROCESS marks none:\n%s", out)

		return ""
	}

	one := s.c.RTL.Process.Name

	if got := marked(); got != one {
		t.Errorf("before SET PROCESS, %s is marked, want %s", got, one)
	}

	for _, spec := range []string{"OTHER", strings.ToLower(fmt.Sprintf("%X", other.Process.PID))} {
		if _, err := s.do(t, "SET PROCESS "+spec); err != nil {
			t.Fatalf("SET PROCESS %s: %v", spec, err)
		}

		if s.c.RTL.Current() != other || marked() != "OTHER" {
			t.Errorf("SET PROCESS %s: the CPU holds %08X", spec, s.c.RTL.Current().Process.PID)
		}
	}

	if _, err := s.do(t, "SET PROCESS"); err != nil {
		t.Fatal(err)
	}

	if s.c.RTL.Current() != s.c.RTL || marked() != one {
		t.Errorf("SET PROCESS alone: the CPU holds %08X, want process 1", s.c.RTL.Current().Process.PID)
	}

	if _, err := s.do(t, "SET PROCESS NOSUCH"); err == nil || !strings.Contains(err.Error(), "NONEXPR") {
		t.Errorf("SET PROCESS NOSUCH: %v, want NONEXPR", err)
	}
}

// bootedSession is a session on a machine booted by vax.init, as
// cmd/govax boots one: the microkernel, the devices, and the scheduler.
func bootedSession(t *testing.T) *session {
	t.Helper()

	var out bytes.Buffer

	c := console.New(&out)
	c.Paths = respath.New(nil, bootdata.FS)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), consoletest.ParseHelp(t, "console.help"))
	c.Dispatcher = d

	db := debugger.Install(c, consoletest.DebugGrammar(t), consoletest.ParseHelp(t, "debug.help"))

	if err := c.Include("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v\n%s", err, out.String())
	}

	return &session{c: c, d: d, db: db, out: &out}
}
