package authorize

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
)

// memoryStore is an authorization file in memory, as corevms's System
// keeps one when no host file is named.
type memoryStore struct{ f *uaf.File }

func (m *memoryStore) Authorization() (*uaf.File, error) { return m.f.Clone(), nil }

func (m *memoryStore) UpdateAuthorization(change func(*uaf.File) error) error {
	f := m.f.Clone()
	if err := change(f); err != nil {
		return err
	}

	if err := f.Check(); err != nil {
		return err
	}

	m.f = f

	return nil
}

// probeCase is one of testdata/uaf95's AUTHORIZE cases: the command
// "MCR AUTHORIZE" ran, what it printed, and the $STATUS it left.
type probeCase struct {
	number  int
	command string
	output  []string
	status  uint32
}

// readProbeCases reads the numbered one-command cases from VMS 7.1's log.
func readProbeCases(t *testing.T) []probeCase {
	t.Helper()

	file, err := os.Open("../../testdata/uaf95/vax/uaf95.log")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	numberLine := regexp.MustCompile(`^== (\d+)$`)
	statusLine := regexp.MustCompile(`^  \$STATUS == "%X([0-9A-F]{8})"$`)

	var (
		cases []probeCase
		cur   *probeCase
		inOut bool
	)

	scan := bufio.NewScanner(file)
	for scan.Scan() {
		line := scan.Text()

		if m := numberLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			cases = append(cases, probeCase{number: n})
			cur, inOut = &cases[len(cases)-1], false

			continue
		}

		if cur == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "$ MCR AUTHORIZE "):
			cur.command, inOut = strings.TrimPrefix(line, "$ MCR AUTHORIZE "), true
		case line == "$ SHOW SYMBOL $STATUS":
			inOut = false
		case statusLine.MatchString(line):
			v, _ := strconv.ParseUint(statusLine.FindStringSubmatch(line)[1], 16, 32)
			cur.status = uint32(v)
			cur = nil
		case inOut:
			cur.output = append(cur.output, line)
		}
	}

	return cases
}

// laterSubtask says whether a case's command is SHOW, LIST, or an
// identifier command, which are subtask 6's.
func laterSubtask(command string) bool {
	verb := strings.ToUpper(strings.Fields(command)[0])
	for _, v := range []string{"SHOW", "LIST", "GRANT", "REVOKE", "CREATE"} {
		if strings.HasPrefix(verb, v) {
			return true
		}
	}

	return strings.Contains(strings.ToUpper(command), "/IDENTIFIER")
}

// expectedProbeDifferences are the cases govax answers otherwise, and why.
var expectedProbeDifferences = map[int]string{
	123: "govax keeps no access times, password lifetimes, or expirations: /ACCESS is IVQUAL (A.3)",
	125: "the same: /NOACCESS is IVQUAL, and UAFO was never added",
}

// TestUAF95Oracle runs testdata/uaf95's one-command AUTHORIZE cases in
// order on a file of govax's own (uaf.Default, as VMS 7.1's AUTHORIZE
// made the scratch file: SYSTEM and DEFAULT), and holds each case's
// messages and $STATUS to VMS 7.1's.
func TestUAF95Oracle(t *testing.T) {
	cases := readProbeCases(t)
	if len(cases) < 130 {
		t.Fatalf("read %d cases from the log, want 134", len(cases))
	}

	store := &memoryStore{f: uaf.Default()}
	ran := 0

	for _, c := range cases {
		if c.command == "" || laterSubtask(c.command) {
			continue
		}

		var out strings.Builder

		u := &Utility{Store: store, Credentials: security.SystemCredentials(), Out: &out}
		status := u.Run(c.command)
		got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")

		if out.Len() == 0 {
			got = nil
		}

		if why, ok := expectedProbeDifferences[c.number]; ok {
			t.Logf("case %d differs as expected (%s)", c.number, why)

			continue
		}

		ran++

		if fmt.Sprint(got) != fmt.Sprint(c.output) || status != c.status {
			t.Errorf("case %d, %s:\n got %%X%08X %q\nwant %%X%08X %q", c.number, c.command, status, got, c.status, c.output)
		}
	}

	t.Logf("%d cases compared", ran)
}

// TestUAF95FinalState runs the same cases and holds the file they leave
// to what VMS 7.1 showed of its own afterward: LIST/BRIEF's accounts and
// UICs (case 119), some of the fields SHOW UAF* gave (case 134), and the
// identifiers SHOW/IDENTIFIER found (cases 115 and 61). SHOW itself is
// subtask 6's, so the test reads the file.
func TestUAF95FinalState(t *testing.T) {
	store := &memoryStore{f: uaf.Default()}

	for _, c := range readProbeCases(t) {
		if c.command == "" || laterSubtask(c.command) {
			continue
		}

		u := &Utility{Store: store, Credentials: security.SystemCredentials(), Out: &strings.Builder{}}
		u.Run(c.command)
	}

	f := store.f
	want := map[string]string{
		"DEFAULT": "[200,200]", "SYSTEM": "[1,4]", "UAFA": "[300,1]", "UAFB": "[200,200]",
		"UAFC": "[300,1]", "UAFE": "[300,5]", "UAFG": "[300,1]", "UAFL": "[300,10]",
		"UAFM": "[300,1]", "UAFN": "[300,13]",
	}

	if len(f.Accounts) != len(want) {
		t.Errorf("%d accounts, want %d", len(f.Accounts), len(want))
	}

	for name, uic := range want {
		r, ok := f.Find(name)
		if !ok {
			t.Errorf("no account %s", name)

			continue
		}

		if r.UIC.String() != uic {
			t.Errorf("%s's UIC is %s, want %s", name, r.UIC, uic)
		}
	}

	g, _ := f.Find("UAFG")
	if g.Owner != "NEW OWNER" || g.Account != "NEWACCT" || g.Priority != 5 || g.Device != "DUA2:" ||
		g.Directory != "[NEWDIR]" || g.Quotas.WSQUOTA != 700 || g.Quotas.CPUTIME != 60000 || g.QueuePriority != 6 {
		t.Errorf("UAFG is %+v", *g)
	}

	if names := strings.Join(g.Flags.Names(), " "); names != "DISCTLY DEFCLI DISWELCOME DISNEWMAIL" {
		t.Errorf("UAFG's flags are %s", names)
	}

	if a, _ := f.Find("UAFA"); a.Quotas.ASTLM != 60 || a.Quotas.BIOLM != 61 {
		t.Errorf("UAFA's ASTLM and BIOLM are %d and %d, want 60 and 61", a.Quotas.ASTLM, a.Quotas.BIOLM)
	}

	if d, _ := f.Find("DEFAULT"); d.Quotas.ASTLM != 24 || d.Quotas.BIOLM != 18 || d.Device != "SYS$SYSDEVICE:" || d.Flags != uaf.FlagDISUSER {
		t.Errorf("DEFAULT is %+v", *d)
	}

	if e, _ := f.Find("UAFE"); e.CLI != "NOSUCHCLI" {
		t.Errorf("UAFE's CLI is %q", e.CLI)
	}

	for name, value := range map[string]string{
		"UAFM": "%X00C00001", "UAFB": "%X00800080", "CASED": "%X00C0FFFF", "UAFN": "%X00C0000B",
	} {
		if id, ok := f.IdentifierNamed(name); !ok || id.Value.String() != value {
			t.Errorf("identifier %s: %v %v, want %s", name, id, ok, value)
		}
	}

	for _, gone := range []string{"UAFA", "UAFI", "UAFF", "UAFC", "UAFG"} {
		if _, ok := f.IdentifierNamed(gone); ok {
			t.Errorf("identifier %s is in the file", gone)
		}
	}
}
