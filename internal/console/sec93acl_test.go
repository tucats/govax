package console

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmserrors"
)

// Round 3 of Phase 93's probe on the ACL services (docs/planning/PHASE-93
// - file protection.md, subtask 10): testdata/prot93's SEC93.MAR, built by
// govax's MACRO and LINK, runs its parts P ($PARSE_ACL), F ($FORMAT_ACL),
// and C ($CHANGE_ACL), and part X's cases of $CHANGE_ACL, on a volume made
// as SEC93B.COM made the VAX's, and each case's report is held to VMS
// 7.1's (vax/round3b/sec93b.log, and part X's vax/round3/sec93x_n.log).

const sec93Dir = "../../testdata/prot93"

// sec93Identifiers are the identifiers SEC93B.COM's AUTHORIZE made, which
// the cases name and $FORMAT_ACL writes (sec93b.log's AUTHORIZE section).
var sec93Identifiers = []struct {
	name  string
	value uint32
}{
	{"PROT93A", 0x00800001}, {"PROT93", 0x0080FFFF}, {"PROT93B", 0x00800002},
	{"PROT93C", 0x00C00001}, {"PROT93_TEAM", 0x80010001},
}

// sec93Users are the accounts SEC93.COM's AUTHORIZE made (the README's
// "The identities"), for $CHECK_ACCESS: PROT93A with no privileges,
// PROT93B and PROT93C with READALL, GRPPRV, SYSPRV, and BYPASS authorized,
// PROT93C holding PROT93_TEAM.
var sec93Users = []corevms.UserAccount{
	{Name: "PROT93A", UIC: 0x00800001},
	{Name: "PROT93B", UIC: 0x00800002, Privileges: sec93Authorized},
	{Name: "PROT93C", UIC: 0x00C00001, Privileges: sec93Authorized, Rights: []security.Identifier{{Value: 0x80010001}}},
}

// sec93Authorized are PROT93B's and PROT93C's authorized privileges.
var sec93Authorized = security.PrivREADALL | security.PrivGRPPRV | security.PrivSYSPRV | security.PrivBYPASS

// sec93XCases are part X's cases of $CHANGE_ACL, each run alone as
// SEC93X.COM ran them: an unknown item, no items, a short ACLLENGTH buffer,
// ADDACLENT buffers that aren't whole ACEs, an unknown type, HIDDEN, and a
// reserved longword.
var sec93XCases = []int{573, 574, 575, 580, 581, 582, 583, 586, 587}

// sec93Masks are what may differ between VMS's reports and govax's: the
// bits 16 to 23 of $CHANGE_ACL's context, which VMS 7.1 used for
// something the probe didn't settle (securertl's change.go), and the
// channel number OPENCHAN's open reports.
var sec93Masks = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`contxt=(..)..(....)`), "contxt=$1..$2"},
	{regexp.MustCompile(`stv=[0-9A-F]{8}`), "stv=........"},
}

func TestSec93ACLOracle(t *testing.T) {
	t.Parallel()
	c, run := sec93Console(t)

	image := sec93Build(t, c)

	got := map[int][]string{}

	run("DEFINE SEC93_PART PFC")

	for n, lines := range sec93Cases(strings.Split(run(`RUN "`+image+`"`), "\n")) {
		got[n] = lines
	}

	run("DEASSIGN SEC93_PART")

	for _, n := range sec93XCases {
		run(fmt.Sprintf("DEFINE SEC93_ONLY %d", n))

		for k, lines := range sec93Cases(strings.Split(run(`RUN "`+image+`"`), "\n")) {
			got[k] = lines
		}
	}

	want := sec93Cases(sec93Log(t, "round3b/sec93b.log"))

	for _, n := range sec93XCases {
		for k, lines := range sec93Cases(sec93Log(t, fmt.Sprintf("round3/sec93x_%d.log", n))) {
			want[k] = lines
		}
	}

	compared := 0

	for n := 1; n <= 587; n++ {
		w, ok := want[n]
		if !ok || !sec93Compared(w) {
			continue
		}

		compared++

		g := got[n]
		if strings.Join(sec93Masked(g), "\n") != strings.Join(sec93Masked(w), "\n") {
			t.Errorf("case %d:\n got:\n%s\nwant:\n%s", n, strings.Join(g, "\n"), strings.Join(w, "\n"))
		}
	}

	t.Logf("compared %d cases", compared)

	if compared < 240 {
		t.Errorf("compared %d cases, want parts P, F, and C and part X's", compared)
	}
}

// sec93Compared reports whether a case is one of this subtask's: parts P,
// F, and C, and part X's $CHANGE_ACL cases.
func sec93Compared(lines []string) bool {
	if len(lines) == 0 {
		return false
	}

	f := strings.Fields(lines[0])

	return len(f) > 2 && (f[2] == "P" || f[2] == "F" || f[2] == "C" || f[2] == "X" && strings.Contains(lines[0], "$CHANGE_ACL"))
}

// sec93Masked is lines with sec93Masks applied.
func sec93Masked(lines []string) []string {
	out := make([]string, len(lines))

	for i, l := range lines {
		for _, m := range sec93Masks {
			l = m.re.ReplaceAllString(l, m.with)
		}

		out[i] = l
	}

	return out
}

// sec93Cases are a run's case reports by number: each "== n ..." line and
// those after it, to the next case or "end of cases".
func sec93Cases(lines []string) map[int][]string {
	cases := map[int][]string{}
	current := 0

	for _, l := range lines {
		l = strings.TrimRight(l, "\r")

		var n int

		switch {
		case strings.HasPrefix(l, "== "):
			if _, err := fmt.Sscanf(l, "== %d ", &n); err == nil {
				current = n
			}
		case l == "end of cases":
			current = 0

			continue
		}

		if current != 0 {
			cases[current] = append(cases[current], l)
		}
	}

	return cases
}

// sec93Log is one of round 3's logs, by its path under vax/.
func sec93Log(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(sec93Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")
}

// sec93Console is a console with a new volume on DUA1 holding SEC93B.COM's
// [SEC93.CHG] files, each with its owner, protection, and ACL, and the
// probe's identifiers in the rights database.
func sec93Console(t *testing.T) (*Console, func(string) string) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	if err := c.RunHostProcedure("startup.com", d.Dispatch); err != nil {
		t.Fatalf("STARTUP.COM: %v", err)
	}

	run := func(line string) string {
		t.Helper()

		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString(dclMessage(err) + "\n")
		}

		return buf.String()
	}

	dir := t.TempDir()
	disk := filepath.Join(dir, "sec93.dsk")
	data := filepath.Join(dir, "t.dat")
	writeHostFile(t, data, "SEC93 FILE\n")

	for _, line := range []string{
		`INITIALIZE/CONTAINER "` + disk + `" /SIZE=4000 SEC93`,
		`MOUNT/WRITE DUA1 "` + disk + `"`,
		"DEFINE PROT93$DISK DUA1:",
		"DEFINE SEC93$CHG PROT93$DISK:[SEC93.CHG]",
		"DEFINE SEC93$GET PROT93$DISK:[SEC93.GET]",
	} {
		if out := run(line); strings.Contains(out, "%") && !strings.Contains(out, "-I-") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	for _, id := range sec93Identifiers {
		if status := c.RTL.AddIdentifier(id.name, id.value, 0); status != 1 {
			t.Fatalf("AddIdentifier %s: %#x", id.name, status)
		}
	}

	for _, u := range sec93Users {
		if status := c.RTL.AddUser(u); status != 1 {
			t.Fatalf("AddUser %s: %#x", u.Name, status)
		}
	}

	sec93Tree(t, c, run, data, "sec93b.com", "[SEC93.CHG")
	sec93Tree(t, c, run, data, "sec93.com", "[SEC93.CHK")
	sec93Tree(t, c, run, data, "sec93.com", "[SEC93.NP")
	sec93Tree(t, c, run, data, "sec93.com", "[SEC93.GET")
	sec93Tree(t, c, run, data, "sec93.com", "[SEC93.SET")

	return c, run
}

// sec93Tree makes the files in a directory of round 3's tree, those of
// procedure proc's lines that name dir ("[SEC93.CHG"): its CREATE/DIRECTORY
// and COPY lines through DCL, and its SET SECURITY lines in Go (SET
// SECURITY is subtask 11's), the ACEs read by $PARSE_ACL's grammar.
func sec93Tree(t *testing.T, c *Console, run func(string) string, data, proc, dir string) {
	t.Helper()

	text, err := os.ReadFile(filepath.Join(sec93Dir, proc))
	if err != nil {
		t.Fatal(err)
	}

	owner := regexp.MustCompile(`/OWNER=(\[[^\]]*\])`)
	protection := regexp.MustCompile(`/PROTECTION=(\([^)]*\))`)
	acl := regexp.MustCompile(`/ACL=(\(.*\)) \S+$`)
	names := consoleNames{c}

	for _, line := range strings.Split(string(text), "\n") {
		if !strings.Contains(line, dir) {
			continue
		}

		f := strings.Fields(line)
		spec := f[len(f)-1]

		switch {
		case strings.HasPrefix(line, "$ CREATE/DIRECTORY "):
			run("CREATE/DIRECTORY " + spec)

		case strings.HasPrefix(line, "$ COPY "):
			if out := run(`COPY "` + data + `" ` + spec); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
				t.Fatalf("%s: %s", line, out)
			}

		case strings.HasPrefix(line, "$ SET SECURITY"):
			o, err := c.RTL.Session.FindSecurityObject(spec)
			if err != nil {
				t.Fatalf("%s: %v", line, err)
			}

			if m := owner.FindStringSubmatch(line); m != nil {
				p := protection.FindStringSubmatch(line)

				uic, err := security.ParseUIC(m[1], names.IdentifierValue)
				if err != nil {
					t.Fatalf("%s: %v", line, err)
				}

				code, err := security.ParseProtection(p[1], security.NoAccess)
				if err != nil {
					t.Fatalf("%s: %v", line, err)
				}

				if err := o.SetOwnerProtection(uic, code); err != nil {
					t.Fatalf("%s: %v", line, err)
				}
			}

			if m := acl.FindStringSubmatch(line); m != nil {
				var raw []byte

				for _, text := range sec93ACEs(m[1]) {
					ace, err := security.ParseACE(text, names, nil)
					if err != nil {
						t.Fatalf("%s: %q: %v", line, text, err)
					}

					raw = append(raw, ace...)
				}

				if err := o.SetACL(raw); err != nil {
					t.Fatalf("%s: %v", line, err)
				}
			}
		}
	}
}

// sec93ACEs are the ACEs of SET SECURITY/ACL's value: "(ace)" or
// "((ace),(ace),...)", each "(ace)".
func sec93ACEs(value string) []string {
	if !strings.HasPrefix(value, "((") {
		return []string{value}
	}

	var (
		aces  []string
		depth int
		start int
	)

	inner := value[1 : len(value)-1]

	for i, ch := range inner {
		switch ch {
		case '(':
			if depth == 0 {
				start = i
			}

			depth++
		case ')':
			depth--
			if depth == 0 {
				aces = append(aces, inner[start:i+1])
			}
		}
	}

	return aces
}

// consoleNames is process 1's rights database as security.Names.
type consoleNames struct{ c *Console }

func (n consoleNames) IdentifierValue(name string) (uint32, bool) {
	v, status := n.c.RTL.IdentifierValue(name)

	return v, status == 0
}

func (n consoleNames) IdentifierName(id uint32) (string, bool) { return n.c.RTL.IdentifierName(id) }

// sec93Build assembles and links SEC93.MAR on the host, returning the
// image's path.
func sec93Build(t *testing.T, c *Console) string {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(sec93Dir, "sec93.mar"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "sec93")
	writeHostFile(t, path+".mar", string(src))

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar", Object: path + ".obj"}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}, Executable: path + ".exe"}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	return path + ".exe"
}

// TestSec93VMSImage runs VMS 7.1's own SEC93.EXE (round 3's, linked
// against SECURESHR and SECURESHRP by VMS's LINK) on govax: it activates,
// and its calls reach securertl's routines through the transfer vector
// offsets its fixup vector holds. Cases 1 ($PARSE_ACL), 88 ($FORMAT_ACL),
// and 176 ($CHANGE_ACL) are held to round 3b's reports, and 259 ($CHKPRO,
// a system service) and 304 ($CHECK_ACCESS) to round 3's.
func TestSec93VMSImage(t *testing.T) {
	t.Parallel()
	_, run := sec93Console(t)

	image, err := filepath.Abs(filepath.Join(sec93Dir, "vax", "round3", "sec93.exe"))
	if err != nil {
		t.Fatal(err)
	}

	want := sec93Cases(sec93Log(t, "round3b/sec93b.log"))
	privileged, _ := sec93Passes(t)

	for _, n := range []int{259, 304} {
		want[n] = privileged[n]
	}

	for _, n := range []int{1, 88, 176, 259, 304} {
		run(fmt.Sprintf("DEFINE SEC93_ONLY %d", n))
		out := run(`RUN "` + image + `"`)

		got := sec93Cases(strings.Split(out, "\n"))[n]
		if strings.Join(got, "\n") != strings.Join(want[n], "\n") {
			t.Errorf("case %d:\n got:\n%s\nwant:\n%s\n(output %q)", n, strings.Join(got, "\n"), strings.Join(want[n], "\n"), out)
		}
	}
}
