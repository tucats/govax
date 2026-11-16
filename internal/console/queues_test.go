package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// The queue commands (docs/planning/PHASE-98 - batch queues.md, subtask
// 6), held to VMS 7.1's run of the probe: BAT98.LOG's QUEUES section,
// cases 1 to 68 (testdata/bat98/vax/bat98.log), each command's output and
// $STATUS.

// queueConsole is a console with a queue database of its own, the queue
// manager started in it, and the identifier BAT98_ID; and a function
// running a command and returning what it printed with $STATUS after it.
func queueConsole(t *testing.T) (*Console, func(string) string) {
	t.Helper()

	c, buf := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t))

	path := filepath.Join(t.TempDir(), "qman.json")
	c.QueueFile = path
	c.RTL.System.SetQueueFile(path)

	if status := c.RTL.AddIdentifier("BAT98_ID", 0x80010098, 0); status != 1 {
		t.Fatalf("AddIdentifier: %#x", status)
	}

	run := func(line string) string {
		t.Helper()

		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString(dclMessage(err) + "\n")
		}

		out := buf.String()

		buf.Reset()

		if err := d.Dispatch("SHOW SYMBOL $STATUS"); err != nil {
			t.Fatal(err)
		}

		return out + buf.String()
	}

	if got := run("START/QUEUE/MANAGER"); got != "  $STATUS == \"%X10040001\"\n" {
		t.Fatalf("START/QUEUE/MANAGER: %q", got)
	}

	return c, run
}

// probeCase is one of a probe log's cases: its number, the command, and
// what VMS printed for it, $STATUS's line last.
type probeCase struct {
	number  string
	command string
	want    string
}

// probeCases reads the cases of the section of a probe log that begins
// with the line "######## from" and ends before "######## to". A case is
// "== n", its command, its output, and SHOW SYMBOL $STATUS's line.
func probeCases(t *testing.T, log, from, to string) []probeCase {
	t.Helper()

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	text := strings.ReplaceAll(string(data), "\r", "")

	start := strings.Index(text, "\n######## "+from+"\n")
	end := strings.Index(text, "\n######## "+to+"\n")

	if start < 0 || end < start {
		t.Fatalf("no section %s in %s", from, log)
	}

	lines := strings.Split(text[start:end], "\n")
	header := regexp.MustCompile(`^== (\d+)$`)

	var cases []probeCase

	for i := 0; i < len(lines); i++ {
		m := header.FindStringSubmatch(lines[i])
		if m == nil || i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "$ ") {
			continue
		}

		pc := probeCase{number: m[1], command: strings.TrimPrefix(lines[i+1], "$ ")}

		var out strings.Builder

		for i += 2; i < len(lines) && lines[i] != "$ SHOW SYMBOL $STATUS"; i++ {
			out.WriteString(lines[i] + "\n")
		}

		if i+1 < len(lines) {
			out.WriteString(lines[i+1] + "\n")
		}

		pc.want = out.String()
		cases = append(cases, pc)
	}

	return cases
}

// queueDifferences are the QUEUES section's cases where govax's answer
// isn't VMS's, and why: by case number, govax's output.
var queueDifferences = map[string]string{
	// INITIALIZE/QUEUE without /BATCH makes a printer queue on VMS;
	// govax has only batch queues (Decision 1), so it refuses, and the
	// queue isn't there to show or delete.
	"46": "%JBC-F-NOTSUPPORTED, item code or function code is not supported\n  $STATUS == \"%X100481F4\"\n",
	"47": "%JBC-E-NOSUCHQUE, no such queue\n  $STATUS == \"%X1004803A\"\n",
	"48": "%DELETE-W-SEARCHFAIL, error searching for BAT98_E\n-JBC-E-NOSUCHQUE, no such queue\n  $STATUS == \"%X10931238\"\n",
}

// TestQueueCommandsOracle replays the probe's QUEUES section. The VAX's
// node, SIMVAX, is govax's GOVAX; the VAX named [200,10] by its rights
// database ([PROT93,BAT98]), which govax, with no such identifiers, shows
// as the UIC; and SHOW QUEUE/MANAGER/FULL's master file is govax's host
// file (Decision 3).
func TestQueueCommandsOracle(t *testing.T) {
	c, run := queueConsole(t)

	cases := probeCases(t, filepath.Join("..", "..", "testdata", "bat98", "vax", "bat98.log"),
		"QUEUES", "JOBS ON A STOPPED QUEUE")
	if len(cases) != 68 {
		t.Fatalf("%d cases, want 68", len(cases))
	}

	for _, pc := range cases {
		want := strings.ReplaceAll(pc.want, "SIMVAX", "GOVAX")
		want = strings.ReplaceAll(want, "[PROT93,BAT98]", "[200,10]")
		want = strings.ReplaceAll(want, "SYS$SYSROOT:[SYSEXE]QMAN$MASTER.DAT;", c.QueueFile)

		if d, ok := queueDifferences[pc.number]; ok {
			want = d
		}

		if got := run(pc.command); got != want {
			t.Errorf("case %s, %s:\n got %q\nwant %q", pc.number, pc.command, got, want)
		}
	}
}

// passCases reads one pass of a protection matrix log
// (testdata/bat98/vax/bat98p_*.log): the cases after "######## PASS pass
// AS", each "== pass n" and its lines to $STATUS's, by number.
func passCases(t *testing.T, log, pass string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	text := strings.ReplaceAll(string(data), "\r", "")

	start := strings.Index(text, "######## PASS "+pass+" AS")
	if start < 0 {
		t.Fatalf("no pass %s in %s", pass, log)
	}

	text = text[start+1:]
	if end := strings.Index(text, "######## PASS "); end >= 0 {
		text = text[:end]
	}

	cases := map[string]string{}
	header := regexp.MustCompile(`^== ` + pass + ` (\d+)$`)
	lines := strings.Split(text, "\n")

	for i := 0; i < len(lines); i++ {
		m := header.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}

		var out strings.Builder

		for i++; i < len(lines) && !strings.HasPrefix(lines[i], "== "); i++ {
			out.WriteString(lines[i] + "\n")

			if strings.HasPrefix(lines[i], "  $STATUS == ") {
				break
			}
		}

		cases[m[1]] = out.String()
	}

	return cases
}

// TestQueueCommandsMatrix holds the queue commands' refusals to the
// probe's protection matrix in state A (BAT98_P and BAT98_Dn owned by
// [SYSTEM], (S:M,O:D,G:R,W:S)): BAT98 and BAT98W with no privilege, who
// may manage neither queue and create none, and BAT98O with OPER, who may
// do everything. Each pass's queue commands are its cases 5 and 6
// (F$GETQUI of the queue's protection and owner, which BAT98 and BAT98W
// may not see), 17 to 22, 25 to 28, and 31 (testdata/bat98/bat98p.com).
func TestQueueCommandsMatrix(t *testing.T) {
	commands := map[string]string{
		"5":  `WRITE SYS$OUTPUT "  GETQUI PROTECTION ", F$GETQUI("DISPLAY_QUEUE","PROTECTION","BAT98_P")`,
		"6":  `WRITE SYS$OUTPUT "  GETQUI OWNER ", F$GETQUI("DISPLAY_QUEUE","OWNER_UIC","BAT98_P")`,
		"17": "STOP/QUEUE/NEXT BAT98_P",
		"18": "START/QUEUE BAT98_P",
		"19": "SET QUEUE/JOB_LIMIT=2 BAT98_P",
		"20": "SET QUEUE/JOB_LIMIT=1 BAT98_P",
		"21": "INITIALIZE/QUEUE/BATCH BAT98_X",
		"22": "DELETE/QUEUE BAT98_X",
		"25": "INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2 D",
		"26": "START/QUEUE D",
		"27": "STOP/QUEUE/NEXT D",
		"28": "DELETE/QUEUE D",
		"31": "SHOW QUEUE/MANAGER",
	}
	order := []string{"5", "6", "17", "18", "19", "20", "21", "22", "25", "26", "27", "28", "31"}

	passes := []struct {
		log, pass string
		user      string
		group     uint32
		member    uint32
		privilege []string
		queue     string
	}{
		{"bat98p_a_bat98w.log", "NONE", "BAT98W", 0o300, 0o10, nil, "BAT98_D1"},
		{"bat98p_a_bat98.log", "NONE", "BAT98", 0o200, 0o10, nil, "BAT98_D1"},
		{"bat98p_a_bat98o.log", "OPER", "BAT98O", 0o200, 0o11, []string{"OPER"}, "BAT98_D2"},
	}

	for _, p := range passes {
		t.Run(p.log+" "+p.pass, func(t *testing.T) {
			c, run := queueConsole(t)

			for _, setup := range []string{
				"INITIALIZE/QUEUE/BATCH/JOB_LIMIT=1/START BAT98_P",
				"INITIALIZE/QUEUE/BATCH " + p.queue,
			} {
				if got := run(setup); got != "  $STATUS == \"%X10040001\"\n" {
					t.Fatalf("%s: %q", setup, got)
				}
			}

			proc := c.RTL.Process
			proc.Username, proc.UIC = p.user, p.group<<16|p.member
			proc.ProcessPrivileges = privilegeMask("NETMBX", "TMPMBX")

			for _, name := range p.privilege {
				proc.ProcessPrivileges |= privilegeMask(name)
			}

			proc.CurrentPrivileges = proc.ProcessPrivileges

			want := passCases(t, filepath.Join("..", "..", "testdata", "bat98", "vax", p.log), p.pass)

			for _, n := range order {
				command := strings.ReplaceAll(commands[n], " D", " "+p.queue)
				w := strings.ReplaceAll(want[n], "SIMVAX", "GOVAX")

				if got := run(command); got != w {
					t.Errorf("case %s, %s:\n got %q\nwant %q", n, command, got, w)
				}
			}
		})
	}
}
