package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
)

// TestProbe4 runs Phase 48's probe 4 (testdata/mp/probe4) under govax, as
// PROBE4.COM runs it on VMS: its images and P4CMDS.COM on a volume that
// is the default directory, and the DCL symbol P4SYM defined. The report
// must be VMS's (testdata/mp/probe4/vax/probe4.log) line for line, and
// each spawned process's log VMS's but for what maskProbe4 masks.
func TestProbe4(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 2000, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	for _, name := range []string{"p4child", "p4nocli", "probe4"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "probe4", name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		mar, obj := filepath.Join(dir, name+".mar"), filepath.Join(dir, name+".obj")
		if err := os.WriteFile(mar, src, 0o644); err != nil {
			t.Fatal(err)
		}

		for _, cmd := range []string{
			`MACRO "` + mar + `"/OBJECT="` + obj + `"`,
			`LINK "` + obj + `"/EXECUTABLE=DUA0:[000000]` + strings.ToUpper(name) + `.EXE`,
		} {
			if err := d.Dispatch(cmd); err != nil {
				t.Fatalf("%s: %v\n%s", cmd, err, out.String())
			}
		}
	}

	cmds, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "probe4", "p4cmds.com"))
	if err != nil {
		t.Fatal(err)
	}

	var records [][]byte
	for _, line := range strings.Split(strings.TrimRight(string(cmds), "\n"), "\n") {
		records = append(records, []byte(line))
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]P4CMDS.COM"}, rms.TextRecords, records); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{"SET DEFAULT DUA0:[000000]", `P4SYM == "a global symbol"`} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatal(err)
		}
	}

	out.Reset()

	if err := c.Run("DUA0:[000000]PROBE4.EXE", console.RunOptions{}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	report := programLines(out.String())
	t.Logf("\n%s", strings.Join(report, "\n"))

	vmsReport, vmsLogs := readProbe4Log(t)

	if got, want := strings.Join(report, "\n"), strings.Join(vmsReport, "\n"); got != want {
		t.Errorf("the report differs from VMS's:\n%s\nwant\n%s", got, want)
	}

	// The spawned processes' output files, as PROBE4.COM types them,
	// each beside VMS's.
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "9", "10"} {
		name := "P4_" + n

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]" + name + ".LOG"}, rms.TextRecords)
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		t.Logf("%s.LOG:\n%s", name, strings.Join(lines, "\n"))

		want, ok := vmsLogs[name]
		if !ok {
			t.Errorf("VMS's log has no %s.LOG", name)

			continue
		}

		if got, want := maskProbe4(lines), maskProbe4(want); got != want {
			t.Errorf("%s.LOG differs from VMS's (masked):\n%s\nwant\n%s", name, got, want)
		}
	}
}

// readProbe4Log reads VMS's run of probe 4 (testdata/mp/probe4/vax/
// probe4.log): the program's report, up to its "end", and each spawned
// process's log as TYPE showed it: a blank line (a space), the file's
// name, a blank line, and its records. The last file's records run into
// the lines of DCL's own SPAWN that followed, which start with
// "%DCL-S-".
func readProbe4Log(t *testing.T) ([]string, map[string][]string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "probe4", "vax", "probe4.log"))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")

	var report []string

	i := 0
	for ; i < len(lines); i++ {
		if lines[i] != "" {
			report = append(report, lines[i]) // programLines has no blank lines
		}

		if lines[i] == "end" {
			break
		}
	}

	header := regexp.MustCompile(`^DUA1:\[000000\](P4_[0-9A-Z]+)\.LOG;1$`)
	logs := map[string][]string{}

	for i++; i < len(lines); i++ {
		m := header.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}

		var records []string

		for i += 2; i < len(lines) && lines[i] != " " && !strings.HasPrefix(lines[i], "%DCL-S-"); i++ {
			records = append(records, lines[i])
		}

		logs[m[1]] = records
		i--
	}

	return report, logs
}

// probe4Time is a VMS absolute time, as LOGOUT's report shows one.
var probe4Time = regexp.MustCompile(`[ 0-9]?[0-9]-[A-Z]{3}-[0-9]{4} [0-9:.]+`)

// probe4Number is a number in LOGOUT's accounting lines, with the blanks
// that right-align it.
var probe4Number = regexp.MustCompile(`[ 0-9:.]*[0-9][ 0-9:.]*`)

// maskProbe4 is a log's lines with what differs between VMS's run and
// govax's made the same: VMS's volume was DUA1, govax's test's is DUA0;
// the time a job logged out; and LOGOUT's counts, which govax doesn't
// keep (corevms's LogoutReport), and its CPU and elapsed times.
func maskProbe4(lines []string) string {
	masked := make([]string, len(lines))

	for i, line := range lines {
		line = strings.ReplaceAll(line, "DUA1:", "DUA0:")
		line = probe4Time.ReplaceAllString(line, "<time>")

		if strings.Contains(line, "count:") || strings.Contains(line, "Page faults:") || strings.Contains(line, "CPU time:") {
			line = probe4Number.ReplaceAllString(line, "#")
		}

		masked[i] = line
	}

	return strings.Join(masked, "\n")
}
