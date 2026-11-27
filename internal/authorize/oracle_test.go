package authorize

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"slices"
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
// "MCR AUTHORIZE" ran, what it printed, the $STATUS it left, and the
// listing file TYPE showed after it, if any.
type probeCase struct {
	number  int
	label   string
	command string
	output  []string
	status  uint32

	listingName string
	listing     []string
}

// readProbeCases reads the numbered one-command cases from VMS 7.1's log
// of round 2 (uaf95.log, cases "== 1" on) or round 3 (round3/uaf95b.log,
// "== L1" on).
func readProbeCases(t *testing.T, log string) []probeCase {
	t.Helper()

	file, err := os.Open("../../testdata/uaf95/vax/" + log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	numberLine := regexp.MustCompile(`^== (L?)(\d+)$`)
	statusLine := regexp.MustCompile(`^  \$STATUS == "%X([0-9A-F]{8})"$`)
	typeLine := regexp.MustCompile(`^\$ TYPE UAF95\$DISK:\[UAF95SCR\](\S+)$`)

	var (
		cases               []probeCase
		cur                 *probeCase
		inOut, inStatus     bool
		inListing, finished bool
	)

	scan := bufio.NewScanner(file)
	for scan.Scan() {
		line := scan.Text()

		if m := numberLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			cases = append(cases, probeCase{number: n, label: m[1] + m[2]})
			cur, inOut, inStatus, inListing, finished = &cases[len(cases)-1], false, false, false, false

			continue
		}

		if cur == nil {
			continue
		}

		if inListing && !strings.HasPrefix(line, "$ ") {
			cur.listing = append(cur.listing, line)

			continue
		}

		inListing = false

		switch {
		case finished && typeLine.MatchString(line):
			cur.listingName, inListing = typeLine.FindStringSubmatch(line)[1], true
		case finished:
		case strings.HasPrefix(line, "$ MCR AUTHORIZE "):
			cur.command, inOut = strings.TrimPrefix(line, "$ MCR AUTHORIZE "), true
		case line == "$ SHOW SYMBOL $STATUS":
			inOut, inStatus = false, true
		case inStatus && statusLine.MatchString(line):
			v, _ := strconv.ParseUint(statusLine.FindStringSubmatch(line)[1], 16, 32)
			cur.status, finished = uint32(v), true
		case inOut:
			cur.output = append(cur.output, line)
		}
	}

	return cases
}

// expectedProbeDifferences are the cases govax answers otherwise, and why.
var expectedProbeDifferences = map[string]string{
	"2":   "SYSTEM's record is process 1's (Decision 3), not 7.1's new file's, and its UIC has its identifier",
	"4":   "the same, SHOW SYSTEM/FULL",
	"123": "govax keeps no access times, password lifetimes, or expirations: /ACCESS is IVQUAL (A.3)",
	"124": "UAFO was never added (case 123)",
	"125": "the same: /NOACCESS is IVQUAL, and UAFO was never added",
	"126": "UAFO was never added (case 123)",
}

// systemIdentifier is the line SHOW/IDENTIFIER and LIST/IDENTIFIER write
// for SYSTEM's UIC identifier, which govax's new file has and VMS 7.1's
// CREATE/RIGHTS didn't make: it is taken out of govax's reports before
// they are compared (cases 6 and 7, round 3's L8 and L10).
const systemIdentifier = "  SYSTEM                           [000001,000004] "

// TestUAF95Oracle runs testdata/uaf95's one-command AUTHORIZE cases (round
// 2's 134) in order on a file of govax's own (uaf.Default, as VMS 7.1's
// AUTHORIZE made the scratch file: SYSTEM and DEFAULT), and holds each
// case's messages, $STATUS, and listing file to VMS 7.1's.
func TestUAF95Oracle(t *testing.T) {
	cases := readProbeCases(t, "uaf95.log")
	if len(cases) < 134 {
		t.Fatalf("read %d cases from the log, want 134", len(cases))
	}

	runProbeCases(t, cases)
}

// TestUAF95Round3Oracle is round 3's LIST cases (L1 to L13): identifiers
// with holders, and LIST/IDENTIFIER and LIST/RIGHTS by name.
func TestUAF95Round3Oracle(t *testing.T) {
	cases := readProbeCases(t, "round3/uaf95b.log")
	if len(cases) < 13 {
		t.Fatalf("read %d cases from the log, want 13", len(cases))
	}

	runProbeCases(t, cases)
}

// runProbeCases runs cases on a new file and compares each with VMS's.
func runProbeCases(t *testing.T, cases []probeCase) {
	t.Helper()

	store := &memoryStore{f: uaf.Default()}
	ran, listed := 0, 0

	for _, c := range cases {
		if c.command == "" {
			continue
		}

		var out strings.Builder

		listings := listingFiles{}
		u := &Utility{Store: store, Credentials: security.SystemCredentials(), Out: &out, WriteListing: listings.write}
		status := u.Run(c.command)
		got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")

		if out.Len() == 0 {
			got = nil
		}

		got = slices.DeleteFunc(got, func(line string) bool { return line == systemIdentifier })

		for name, lines := range listings {
			listings[name] = slices.DeleteFunc(lines, func(line string) bool { return line == systemIdentifier })
		}

		if why, ok := expectedProbeDifferences[c.label]; ok {
			t.Logf("case %s differs as expected (%s)", c.label, why)

			continue
		}

		ran++

		want := comparableLines(c.output)
		if fmt.Sprint(comparableLines(got)) != fmt.Sprint(want) || status != c.status {
			t.Errorf("case %s, %s: got %%X%08X, want %%X%08X\n%s", c.label, c.command, status, c.status, lineDiff(comparableLines(got), want))
		}

		// A listing VMS's TYPE showed is held to the file this case wrote;
		// TYPE after a LIST that wrote none showed an older file.
		if wrote, ok := listings[c.listingName]; ok {
			listed++

			if fmt.Sprint(comparableLines(wrote)) != fmt.Sprint(comparableLines(c.listing)) {
				t.Errorf("case %s, %s's %s:\n%s", c.label, c.command, c.listingName, lineDiff(comparableLines(wrote), comparableLines(c.listing)))
			}
		} else if len(listings) > 0 {
			t.Errorf("case %s wrote %v, VMS's TYPE showed %q", c.label, listings, c.listingName)
		}
	}

	t.Logf("%d cases compared, %d with listing files", ran, listed)
}

// listingFiles are LIST's files, by name, as a test keeps them.
type listingFiles map[string][]string

func (l listingFiles) write(name string, lines []string) error {
	l[name] = lines

	return nil
}

// unkeptLines are report lines about what govax doesn't keep (the phase
// doc's A.3), made the same on both sides: VMS 7.1's password minimum
// and lifetime, which govax has none of, and the password change, which
// VMS's ADD left pre-expired and govax gives a time.
var unkeptLines = []struct {
	match *regexp.Regexp
	with  string
}{
	{regexp.MustCompile(`Pwdminimum: +\d+`), "Pwdminimum:  0"},
	{regexp.MustCompile(`^Pwdlifetime: .*$`), "Pwdlifetime: (not kept)"},
}

// comparableLines is lines with unkeptLines' changes, and without UAFO's full
// report, which VMS's case 134 has (govax never added UAFO, case 123).
func comparableLines(lines []string) []string {
	var out []string

	for i := 0; i < len(lines); i++ {
		if lines[i] == "" && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "Username: UAFO ") {
			for i++; i+1 < len(lines) && !(lines[i+1] == "" && i+2 < len(lines) && strings.HasPrefix(lines[i+2], "Username: ")); i++ {
			}

			continue
		}

		line := lines[i]
		for _, u := range unkeptLines {
			line = u.match.ReplaceAllString(line, u.with)
		}

		out = append(out, line)
	}

	return out
}

// lineDiff shows where got and want first differ, and their lengths.
func lineDiff(got, want []string) string {
	for i := 0; i < len(got) || i < len(want); i++ {
		g, w := "<none>", "<none>"
		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g != w {
			return fmt.Sprintf("  line %d (of %d, want %d):\n   got %q\n  want %q", i+1, len(got), len(want), g, w)
		}
	}

	return "  same lines"
}

// TestUAF95FinalState runs the same cases and holds the file they leave
// to what VMS 7.1 showed of its own afterward: LIST/BRIEF's accounts and
// UICs (case 119), some of the fields SHOW UAF* gave (case 134), and the
// identifiers SHOW/IDENTIFIER found (cases 115 and 61). SHOW itself is
// subtask 6's, so the test reads the file.
func TestUAF95FinalState(t *testing.T) {
	store := &memoryStore{f: uaf.Default()}

	for _, c := range readProbeCases(t, "uaf95.log") {
		if c.command == "" {
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
