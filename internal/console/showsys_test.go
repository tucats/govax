package console_test

import (
	"encoding/binary"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
)

// Phase 44's subtask 10: SHOW SYSTEM and SHOW PROCESS.

// sayConsole runs a console command line through the console's grammar and
// returns what it printed.
func sayConsole(t *testing.T, c *console.Console, d *console.Dispatcher, line string) (string, error) {
	t.Helper()

	out := c.Out.(interface {
		String() string
		Reset()
	})
	out.Reset()

	err := d.Dispatch(line)

	return out.String(), err
}

// TestShowSystem: SHOW SYSTEM lists both processes, with the states the
// scheduler has them in and the CPU time each has used.
func TestShowSystem(t *testing.T) {
	code, _ := assembleAt(t, hiberCount)

	c, _ := scheduledConsole(t, longQuantum, counter())
	two := handBuiltProcess(t, c, code)
	two.Process.Name = "SLEEPER"
	c.RTL.Process.Name = "SYSTEM"

	step(t, c, 500) // process 2 hibernates; process 1 counts on

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	text, err := sayConsole(t, c, d, "SHOW SYSTEM")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("SHOW SYSTEM:\n%s", text)
	}

	// "GOVAX <version>  on node <host>": both vary, so they're matched
	// by pattern, and the rest exactly.
	title := regexp.MustCompile(`^GOVAX( \S+)?  on node \S+ +\d?\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d  Uptime  0 \d\d:\d\d:\d\d$`)
	if !title.MatchString(lines[0]) {
		t.Errorf("title %q", lines[0])
	}

	one := regexp.MustCompile(`^00000301 SYSTEM          CUR      4        0   0 00:00:\d\d\.\d\d +\d+ +\d+$`)
	sleeper := regexp.MustCompile(`^00000302 SLEEPER         HIB      4        0   0 00:00:00\.\d\d +\d+ +\d+$`)

	if !one.MatchString(lines[2]) || !sleeper.MatchString(lines[3]) {
		t.Errorf("process lines:\n%s\n%s", lines[2], lines[3])
	}

	// The page faults column is each process's JPI$_PAGEFLTS.
	for i, env := range []*corevms.Environment{c.ProcessOne(), two} {
		fields := strings.Fields(lines[2+i])
		if want := strconv.Itoa(int(env.Process.PageFaults)); fields[len(fields)-2] != want {
			t.Errorf("%s's page faults %s, want %s", env.Process.Name, fields[len(fields)-2], want)
		}
	}

	if strings.HasSuffix(lines[2], " 0") {
		t.Errorf("process 1 maps no pages: %q", lines[2])
	}

	// With paging off, $GETJPI's PPGCNT is the valid pages, the Pages
	// column with no global section mapped, and WSPEAK at least that.
	long := func(name string) int {
		data, ok := c.ProcessOne().JPIItem(name)
		if !ok || len(data) != 4 {
			t.Fatalf("%s: %q, %v", name, data, ok)
		}

		return int(binary.LittleEndian.Uint32([]byte(data)))
	}

	fields := strings.Fields(lines[2])
	if pages := fields[len(fields)-1]; strconv.Itoa(long("JPI$_PPGCNT")) != pages || long("JPI$_GPGCNT") != 0 ||
		long("JPI$_WSPEAK") < long("JPI$_PPGCNT") {
		t.Errorf("PPGCNT %d, GPGCNT %d, WSPEAK %d; want %s, 0, and at least PPGCNT",
			long("JPI$_PPGCNT"), long("JPI$_GPGCNT"), long("JPI$_WSPEAK"), pages)
	}
}

// TestShowProcess: SHOW PROCESS shows the console's process, or another
// by name or /IDENTIFICATION; a process that doesn't exist is an error.
func TestShowProcess(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, counter())
	two := handBuiltProcess(t, c, counter())
	two.Process.Name = "OTHER"
	c.RTL.Process.Name = "SYSTEM"

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	for _, tt := range []struct {
		line, want string
	}{
		{"SHOW PROCESS", `Process ID:   00000301`},
		{"SHOW PROCESS OTHER", `Process name: "OTHER"`},
		{"SHOW PROCESS/IDENTIFICATION=302", `Process ID:   00000302`},
	} {
		text, err := sayConsole(t, c, d, tt.line)
		if err != nil {
			t.Fatalf("%s: %v", tt.line, err)
		}

		if !strings.Contains(text, tt.want) || !strings.Contains(text, "Base priority:      4") {
			t.Errorf("%s:\n%s", tt.line, text)
		}
	}

	if _, err := sayConsole(t, c, d, "SHOW PROCESS/IDENTIFICATION=399"); err == nil || !strings.Contains(err.Error(), "NONEXPR") {
		t.Errorf("a process that doesn't exist: %v", err)
	}
}
