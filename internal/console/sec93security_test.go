package console

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// Round 3 of Phase 93's probe on $GET_SECURITY and $SET_SECURITY
// (docs/planning/PHASE-93 - file protection.md, subtask 13b): SEC93.MAR's
// parts G and S run as SYSTEM with every privilege, part X's cases of the
// two services each alone, and parts M (SYSTEM without READALL, GRPPRV,
// SYSPRV, and BYPASS) and U (PROT93C) whole, each case held to VMS 7.1's
// report (vax/round3's sec93.log, sec93u.log, and sec93x_n.log).

// sec93SecurityXCases are part X's cases of the two services.
var sec93SecurityXCases = []int{567, 568, 576, 577, 578, 579, 584, 585}

// sec93SecurityMasks are what may differ from VMS's reports beyond
// sec93Masks: ACCESS_NAMES's descriptors' addresses, which point into
// VMS's system space and into the buffer in govax's.
var sec93SecurityMasks = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`^(    0[4-7] 00 00 00) .. .. .. .. (0[4-7] 00 00 00) .. .. .. ..$`), "$1 .. .. .. .. $2 .. .. .. .."},
	{regexp.MustCompile(`^(    0[4-7] 00 00 00) .. .. .. .. (00 00 00 00 00 00 00 00)$`), "$1 .. .. .. .. $2"},
}

// sec93SecurityDifferences are the cases whose reports differ from VMS
// 7.1's, and why.
var sec93SecurityDifferences = map[int]string{
	415: "the system disk: VMS's DUA0 is OPENVMS071, govax's the system volume",
	416: "the system disk's volume: VMS's OPENVMS071 owned by [1,1]",
}

func TestSec93SecurityOracle(t *testing.T) {
	c, run := sec93Console(t)
	c.RTL.NodeName = "SIMVAX"
	image := sec93Build(t, c)

	// As SEC93.COM ran it, the default directory on the volume, so a
	// relative name is looked for there (case 409).
	run("SET DEFAULT PROT93$DISK:[000000]")

	privileged, unprivileged := sec93Passes(t)
	compared := 0

	compare := func(pass string, got, want map[int][]string, keep func([]string) bool) {
		t.Helper()

		for n, w := range want {
			if !keep(w) {
				continue
			}

			compared++

			g := got[n]
			if strings.Join(sec93SecurityMasked(g), "\n") == strings.Join(sec93SecurityMasked(w), "\n") {
				if why, ok := sec93SecurityDifferences[n]; ok {
					t.Logf("%s case %d now matches VMS (listed as %q)", pass, n, why)
				}

				continue
			}

			if _, ok := sec93SecurityDifferences[n]; ok {
				continue
			}

			t.Errorf("%s case %d:\n got:\n%s\nwant:\n%s", pass, n, strings.Join(g, "\n"), strings.Join(w, "\n"))
		}
	}

	part := func(p string) func([]string) bool {
		return func(lines []string) bool {
			f := strings.Fields(lines[0])

			return len(f) > 2 && f[2] == p
		}
	}

	runParts := func(parts string) map[int][]string {
		run("DEFINE SEC93_PART " + parts)
		defer run("DEASSIGN SEC93_PART")

		return sec93Cases(strings.Split(run(`RUN "`+image+`"`), "\n"))
	}

	got := runParts("GS")
	compare("G", got, privileged, part("G"))
	compare("S", got, privileged, part("S"))

	for _, n := range sec93SecurityXCases {
		run(fmt.Sprintf("DEFINE SEC93_ONLY %d", n))
		got := sec93Cases(strings.Split(run(`RUN "`+image+`"`), "\n"))
		compare("X", got, sec93Cases(sec93Log(t, fmt.Sprintf("round3/sec93x_%d.log", n))), part("X"))
	}

	run("DEASSIGN SEC93_ONLY")

	run("SET PROCESS/PRIVILEGES=(NOREADALL,NOGRPPRV,NOSYSPRV,NOBYPASS)")
	compare("M", runParts("M"), unprivileged, part("M"))

	p := c.RTL.Process
	p.UIC, p.Username = uint32(security.NewUIC(0o300, 1)), "PROT93C"
	privileges := prot93Privilege("NETMBX") | prot93Privilege("TMPMBX")
	p.AuthorizedPrivileges = sec93Authorized | privileges
	p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges
	p.Rights = append(p.Rights, security.Identifier{Value: 0x80010001})

	// A new login's default protection, which part M's $SETDFPROT case
	// changed in SYSTEM's process.
	p.DefaultProtection = 0xFA00

	compare("U", runParts("U"), sec93Cases(sec93Log(t, "round3/sec93u.log")), part("U"))

	t.Logf("compared %d cases", compared)

	if compared < 200 {
		t.Errorf("compared %d cases, want parts G, S, M, and U, and part X's", compared)
	}
}

// sec93SecurityMasked is lines with sec93Masks and sec93SecurityMasks
// applied.
func sec93SecurityMasked(lines []string) []string {
	out := sec93Masked(lines)

	for i, l := range out {
		for _, m := range sec93SecurityMasks {
			l = m.re.ReplaceAllString(l, m.with)
		}

		out[i] = l
	}

	return out
}
