package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmsdef"
)

// priv94Identity is one of the probe's three identities (testdata/priv94's
// README): its name, UIC, and the privileges it's authorized for and holds
// at login.
type priv94Identity struct {
	name       string
	uic        security.UIC
	authorized uint64
	holds      uint64
}

// priv94Mask is the mask of the privileges named, by $PRVDEF name.
func priv94Mask(names ...string) uint64 {
	var m uint64

	for _, n := range names {
		v, ok := vmsdef.Symbols["PRV$V_"+n]
		if !ok {
			panic("no privilege " + n)
		}

		m |= 1 << v
	}

	return m
}

// The identities. A mask is all 64 bits on VMS (the phase doc's A.4).
var (
	priv94System  = priv94Identity{"SYSTEM", security.NewUIC(1, 4), ^uint64(0), ^uint64(0)}
	priv94Account = priv94Identity{"PRIV94", security.NewUIC(0o200, 3), ^uint64(0), priv94Mask("TMPMBX", "NETMBX")}
	priv94Plain   = priv94Identity{"PRIV94N", security.NewUIC(0o200, 4),
		priv94Mask("TMPMBX", "NETMBX"), priv94Mask("TMPMBX", "NETMBX")}
)

// become makes process 1 the identity, as a login would.
func (id priv94Identity) become(c *console.Console) {
	p := c.RTL.Process
	p.Username = id.name
	p.UIC = uint32(id.uic)
	p.AuthorizedPrivileges = id.authorized
	p.ProcessPrivileges, p.CurrentPrivileges = id.holds, id.holds
	c.RTL.Logicals.SetUIC(p.UIC)
}

// priv94Run is one run of the program: who runs it, and its parts. With
// keep, the run starts with the masks the last one left.
type priv94Run struct {
	who   priv94Identity
	parts string
	keep  bool
}

// runPriv94As is runPriv94 with each run made by an identity.
func runPriv94As(t *testing.T, programs []string, runs []priv94Run) []string {
	t.Helper()

	var outs []string

	parts := make([]string, len(runs))
	for i, r := range runs {
		parts[i] = r.parts
	}

	runPriv94Each(t, programs, parts, func(i int, c *console.Console) {
		if !runs[i].keep {
			runs[i].who.become(c)
		}
	},
		func(out string) { outs = append(outs, out) })

	return outs
}

// priv94VMSRun is the output of the program's run in VMS's log file whose
// "parts" line is parts, from its first line ("user ...") to "end of cases"
// and the two lines after.
func priv94VMSRun(t *testing.T, file, parts string) []string {
	t.Helper()

	return priv94VMSRunAt(t, file, parts, 1)
}

// priv94VMSRunAt is priv94VMSRun for the nth such run.
func priv94VMSRunAt(t *testing.T, file, parts string, nth int) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "priv94", "vax", file))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")

	for i := 3; i < len(lines); i++ {
		if lines[i] != "parts "+parts {
			continue
		}

		if nth--; nth > 0 {
			continue
		}

		for j := i; j < len(lines); j++ {
			if lines[j] == "end of cases" {
				return lines[i-3 : j+3]
			}
		}
	}

	t.Fatalf("%s: no run of parts %s", file, parts)

	return nil
}

// priv94Case is one case's lines, its "==" line first.
type priv94Case struct {
	number int
	lines  []string
}

// priv94Cases splits a run's lines into its cases.
func priv94Cases(lines []string) map[int]priv94Case {
	cases := map[int]priv94Case{}

	var cur *priv94Case

	for _, line := range lines {
		var n int
		if _, err := fmt.Sscanf(line, "== %d ", &n); err == nil {
			cases[n] = priv94Case{number: n}
			c := cases[n]
			cur = &c
		}

		if line == "end of cases" {
			cur = nil
		}

		if cur != nil {
			cur.lines = append(cur.lines, line)
			cases[cur.number] = *cur
		}
	}

	return cases
}

// priv94Varying are the parts of a line that differ from system to system:
// PIDs, job table names, mailbox units, a process's current priority
// (which VMS's scheduler boosts and govax's needn't), and the lines
// beginning "  ~".
var priv94Varying = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`LNM\$JOB_[0-9A-F]{8}`), "LNM$JOB_x"},
	{regexp.MustCompile(`pid [0-9A-F]{8}`), "pid x"},
	{regexp.MustCompile(`MBA[0-9]+:`), "MBAn:"},
	{regexp.MustCompile(`^  pri [0-9]+ prib`), "  pri n prib"},
	{regexp.MustCompile(`^(  self 1 G [01] W [01]) nopriv [0-9]+`), "$1 nopriv n"},
}

// priv94Shown reports whether a line of govax's run is one the terminal
// showed beside the program's output: an OPCOM message (govax's process 1
// is the operator's console) or a broadcast, which on the VAX went to the
// terminal and not the log.
func priv94Shown(line string) bool {
	return line == "" || strings.HasPrefix(line, "%%%%%%%%%%% OPCOM") ||
		strings.HasPrefix(line, "Message from user ") || strings.HasPrefix(line, "PRIV94 PROBE ")
}

// priv94Normalize masks what varies in a case's lines.
func priv94Normalize(lines []string) []string {
	var out []string

	for _, line := range lines {
		if priv94Shown(line) {
			continue
		}

		if strings.HasPrefix(line, "  ~") {
			if name, _, ok := strings.Cut(strings.TrimPrefix(line, "  ~"), " "); ok {
				line = "  ~" + name
			}
		}

		for _, v := range priv94Varying {
			line = v.re.ReplaceAllString(line, v.with)
		}

		out = append(out, line)
	}

	return out
}

// priv94Difference is a case of VMS's run that govax's doesn't match, and
// why.
type priv94Difference struct {
	who   string
	cases []int
	why   string
}

// priv94Differences are the cases the oracle doesn't hold govax to.
var priv94Differences = []priv94Difference{
	{"SYSTEM PRIV94", []int{106, 108}, "VMS 7.1's temporary page-file section outlived " +
		"its only mapping, so the undo's $DGBLSC found it; govax's goes with the mapping (unconfirmed)"},
	{"SYSTEM PRIV94", []int{254}, "the IOSB's count of terminals that refused the broadcast " +
		"is the VAX's (32); govax's one terminal takes it"},
}

// priv94LineSpeed is a terminal case's IOSB (part T): its second word is
// the terminal's transmit speed on VMS (6, TT$C_BAUD_300, the VAX's
// OPA0), which govax's console doesn't have and reports as 0
// (ttdriver.go's ttSenseMode), so the oracle doesn't compare it.
var priv94LineSpeed = regexp.MustCompile(`^(  iosb [0-9A-F]{4}) [0-9A-F]{4} `)

// priv94Excused reports whether case n of who's run is one the oracle
// doesn't compare.
func priv94Excused(who string, n int, part string) bool {
	for _, d := range priv94Differences {
		if slices.Contains(strings.Fields(d.who), who) && slices.Contains(d.cases, n) {
			return true
		}
	}

	return false
}

// priv94Walk is a wildcard $GETJPI case's line of govax's run: the
// processes it may not see (SS$_NOPRIV) are the holders it didn't count,
// where VMS's count has its own system processes too.
var priv94Walk = regexp.MustCompile(`^  self 1 G ([01]) W ([01]) nopriv ([0-9]+) `)

// TestPriv94Oracle runs testdata/priv94's program as each identity, as the
// VAX's run did (the phase doc's Part D), and holds each case to VMS's
// (testdata/priv94/vax), but for priv94Differences and the parts later
// subtasks hold; then image rundown's part R and a run after it, whole.
func TestPriv94Oracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	systemDown := priv94System
	systemDown.holds = priv94Mask("TMPMBX", "NETMBX")

	outs := runPriv94As(t, []string{"PRIV94", "PRIV94H", "PRIV94I"}, []priv94Run{
		{priv94System, "FH", false},
		{priv94System, "", false},
		{priv94Account, "", false},
		{priv94Plain, "SPX", false},
		{systemDown, "R", false},
		{systemDown, "-", true},
		{priv94System, "HZ", false},
	})

	compare := []struct {
		name, file, parts string
		out               string
	}{
		{"SYSTEM", "priv94.log", "LMEGQPCSOWDTX", outs[1]},
		{"PRIV94", "priv94u_priv94.log", "LMEGQPCSOWDTX", outs[2]},
		{"PRIV94N", "priv94u_priv94n.log", "SPX", outs[3]},
	}

	for _, cmp := range compare {
		want := priv94Cases(priv94VMSRun(t, cmp.file, cmp.parts))
		got := priv94Cases(strings.Split(cmp.out, "\n"))

		for n := 1; n <= 289; n++ {
			w, ok := want[n]
			if !ok {
				continue
			}

			g := got[n]

			if len(g.lines) == 0 {
				t.Errorf("%s case %d didn't run", cmp.name, n)

				continue
			}

			if priv94Excused(cmp.name, n, strings.Fields(w.lines[0])[2]) {
				continue
			}

			gl, wl := priv94Normalize(g.lines), priv94Normalize(w.lines)

			if strings.Fields(w.lines[0])[2] == "T" {
				for _, lines := range [][]string{gl, wl} {
					for i, line := range lines {
						lines[i] = priv94LineSpeed.ReplaceAllString(line, "$1 speed ")
					}
				}
			}

			if strings.Join(gl, "\n") != strings.Join(wl, "\n") {
				t.Errorf("%s case %d:\n got %s\nwant %s", cmp.name, n,
					strings.Join(gl, "\n     "), strings.Join(wl, "\n     "))
			}
		}
	}

	// The rundown: part R leaves OPER enabled permanently and ALTPRI for
	// the image; the next run starts with the permanent mask alone.
	for i, parts := range []string{"R", "-"} {
		want := priv94Normalize(priv94VMSRun(t, "priv94.log", parts))
		got := priv94Normalize(strings.Split(strings.TrimSuffix(outs[4+i], "\n"), "\n"))

		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("part %s:\n got %s\nwant %s", parts, strings.Join(got, "\n     "), strings.Join(want, "\n     "))
		}
	}

	// What the wildcard $GETJPI couldn't see, in govax's runs.
	for _, out := range outs[1:4] {
		for _, line := range strings.Split(out, "\n") {
			m := priv94Walk.FindStringSubmatch(line)
			if m == nil {
				continue
			}

			if seen := int(m[1][0]-'0') + int(m[2][0]-'0'); m[3] != strconv.Itoa(2-seen) {
				t.Errorf("a wildcard $GETJPI: %q, want nopriv %d (the holders it didn't see)", line, 2-seen)
			}
		}
	}
}

// TestPriv94BOracle is TestPriv94Oracle for round 2's program, PRIV94B:
// its permanent mailboxes (part M), $ULKPAG (part U), $CRELNT under
// shareable tables (N), and the modes $DELLNM deletes (D), as SYSTEM and
// then as PRIV94, who finds the tables SYSTEM's undo left (as VMS's
// PRIV94 did: SS$_SUPERSEDE).
func TestPriv94BOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	outs := runPriv94As(t, []string{"PRIV94B"}, []priv94Run{
		{priv94System, "F", false},
		{priv94System, "MUND", false},
		{priv94Account, "MUND", false},
		{priv94System, "Z", false},
	})

	for i, file := range []string{"round2/priv94r.log", "round2/priv94v.log"} {
		want := priv94Cases(priv94VMSRun(t, file, "MUND"))
		got := priv94Cases(strings.Split(outs[1+i], "\n"))

		for n := 1; n <= 40; n++ {
			w, ok := want[n]
			if !ok {
				continue
			}

			gl, wl := priv94Normalize(got[n].lines), priv94Normalize(w.lines)

			if strings.Join(gl, "\n") != strings.Join(wl, "\n") {
				t.Errorf("%s case %d:\n got %s\nwant %s", file, n,
					strings.Join(gl, "\n     "), strings.Join(wl, "\n     "))
			}
		}
	}
}
