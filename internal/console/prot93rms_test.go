package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 93's subtask 8 (docs/planning/PHASE-93 - file protection.md):
// RMS's services and the ACP checked against the calling process's
// identity, held to the probe's program runs on VMS 7.1.
//
// The probe's program, PROT93.EXE (testdata/prot93/prot93.mar), reads one
// case per line and reports what RMS or the ACP did. Each account ran it
// with privileges turned on and off, and each run's output is in the logs
// (testdata/prot93/vax). This test runs VMS's own image on govax, on a
// copy of the exchange volume as VMS left it after round 2 (local only:
// testdata/disks/prot93-exchange.dsk, whose headers VMS wrote), as each
// identity in turn, and compares each case's report with VMS's.
//
// The volume is as it was at the end of the run, not at each pass: a
// file a pass made, erased, or renamed is already there, or gone. So a
// case that changes the volume can't give VMS's answer when VMS's answer
// was a success; prot93Changed lists the operations that do, and their
// cases, and those of the files each pass made (named with its tag), are
// compared only when VMS refused them (when the refusal is what is being
// checked, and the volume holds the same files). An allowed search is
// compared to its first $SEARCH's status: what it lists is the
// directory as it was.

// prot93Run is one RUN of the program in a log: the identity, the tag,
// the case file, and VMS's output.
type prot93Run struct {
	log, tag, input string
	uic             security.UIC
	privileges      uint64
	rights          []security.Identifier
	output          []string
}

// prot93Accounts are the probe's identities by username (the README's
// table); PROT93_TEAM is PROT93C's general identifier, at round 1's
// value, which the tree's ACEs name.
var prot93Accounts = map[string]struct {
	uic    security.UIC
	rights []security.Identifier
}{
	"PROT93A": {security.NewUIC(0o200, 1), nil},
	"PROT93B": {security.NewUIC(0o200, 2), nil},
	"PROT93C": {security.NewUIC(0o300, 1), []security.Identifier{{Value: 0x80010001}}},
	"SYSTEM":  {security.SystemUIC, nil},
}

// prot93Runs reads the program's runs from the log name, run by user.
func prot93Runs(t *testing.T, name, user string) []prot93Run {
	t.Helper()

	account := prot93Accounts[user]
	privileges := uint64(prot93Privilege("NETMBX") | prot93Privilege("TMPMBX"))

	if user == "SYSTEM" {
		privileges = security.AllPrivileges
	}

	var (
		runs       []prot93Run
		tag, input string
	)

	log := prot93Log(t, name)

	for i, line := range log {
		switch {
		case strings.HasPrefix(line, "$ SET PROCESS/PRIVILEGES="):
			list := strings.Trim(strings.TrimPrefix(line, "$ SET PROCESS/PRIVILEGES="), "()")
			for _, p := range strings.Split(list, ",") {
				if bare, off := strings.CutPrefix(p, "NO"); off {
					privileges &^= prot93Privilege(bare)
				} else {
					privileges |= prot93Privilege(p)
				}
			}

		case strings.HasPrefix(line, "$ DEFINE PROT93_TAG "):
			tag = strings.TrimPrefix(line, "$ DEFINE PROT93_TAG ")

		case strings.HasPrefix(line, "$ DEFINE/USER_MODE SYS$INPUT "):
			input = strings.TrimPrefix(line, "$ DEFINE/USER_MODE SYS$INPUT ")

		case line == "$ RUN PROT93$DISK:[000000]PROT93":
			r := prot93Run{log: name, tag: tag, input: input, uic: account.uic, privileges: privileges, rights: account.rights}

			for j := i + 1; j < len(log) && !strings.HasPrefix(log[j], "$ "); j++ {
				r.output = append(r.output, log[j])
			}

			runs = append(runs, r)
		}
	}

	return runs
}

// prot93Privilege is a privilege's bit by name.
func prot93Privilege(name string) uint64 {
	v, ok := vmsdef.Symbols["PRV$V_"+name]
	if !ok {
		panic("no privilege " + name)
	}

	return 1 << v
}

// prot93Changed are the operations that change the volume: a case of
// one is compared only when VMS refused it.
var prot93Changed = map[string]bool{"C": true, "E": true, "R": true, "QM": true, "QD": true, "QC": true}

// prot93Case is one case of a run's output: its "==" line and what
// follows.
type prot93Case struct {
	line  string
	lines []string
}

// prot93Cases splits a run's output into its cases.
func prot93Cases(output []string) []prot93Case {
	var cases []prot93Case

	for _, l := range output {
		l = strings.TrimRight(l, " ")

		switch {
		case strings.HasPrefix(l, "== "):
			cases = append(cases, prot93Case{line: l})
		case len(cases) > 0 && strings.HasPrefix(l, "  "):
			cases[len(cases)-1].lines = append(cases[len(cases)-1].lines, l)
		}
	}

	return cases
}

// refused reports whether VMS's report of a case is a refusal: its first
// status RMS$_PRV, RMS$_ENT, or RMS$_RMV, or its IOSB SS$_NOPRIV.
func (c prot93Case) refused() bool {
	for _, l := range c.lines {
		if strings.Contains(l, " sts=0001829A ") || strings.HasSuffix(l, " iosb=0024") ||
			strings.Contains(l, " sts=0001C01A ") || strings.Contains(l, " sts=0001C0FC ") {
			return true
		}
	}

	return false
}

// operation is the case's operation: the line's letter, or Q and its flag.
func (c prot93Case) operation() string {
	op := strings.Fields(strings.TrimPrefix(c.line, "== "))[0]
	if op[0] == 'Q' {
		return op
	}

	return op[:1]
}

// prot93Normalize takes out of a report what differs between VMS and
// govax for reasons this phase isn't about: the STV of a successful open
// or create, which VMS sets to an internal value.
func prot93Normalize(lines []string) []string {
	out := make([]string, len(lines))

	for i, l := range lines {
		if strings.Contains(l, " sts=0001") && strings.HasSuffix(l, " stv=000000A0") {
			l = strings.TrimSuffix(l, "000000A0") + "00000000"
		}

		out[i] = l
	}

	return out
}

// prot93Differences are the cases whose report differs from VMS's for a
// reason the volume's state or VMS's directory cache explains, by the
// log, the pass's tag, and the case's line (the pass's tag in it as "@"),
// with the reason. Each must still differ.
var prot93Differences = map[string]string{}

// The reasons, and the cases each explains.
func init() {
	add := func(why string, keys ...string) {
		for _, k := range keys {
			prot93Differences[k] = why
		}
	}

	// VMS refused a new version of L_@.DAT, limit 2, whose ;1 gave no D;
	// round 2 then left only ;2 on the volume, so a new one is within the
	// limit. rms's TestNewVersionPastLimit holds the refusal.
	for _, r := range []string{"prot93.log M0", "round2/prot93v_prot93a.log A0", "round2/prot93v_prot93b.log B0", "round2/prot93v_prot93c.log C0"} {
		add("the volume holds only L_@.DAT;2 now", r+" == C- PROT93$DISK:[PROT93.NV]L_@.DAT")
	}

	// FACLCTL's protection and ACL changed between round 1's runs and
	// the end (security's TestProbeMatrix takes it as a case of its own).
	const faclctl = "PROT93$DISK:[PROT93.F]FACLCTL.DAT"
	for _, r := range []string{"prot93u_prot93b.log B1", "prot93u_prot93b.log B4", "prot93u_prot93c.log C1", "prot93u_prot93c.log C4"} {
		add("FACLCTL changed since", r+" == A- "+faclctl, r+" == QA "+faclctl)
	}

	add("FACLCTL changed since", "prot93u_prot93b.log B3 == A- "+faclctl, "prot93u_prot93b.log B3 == QA "+faclctl,
		"prot93u_prot93b.log B3 == QR "+faclctl, "prot93u_prot93b.log B3 == O- "+faclctl+"|G",
		"prot93u_prot93b.log B3 == O- "+faclctl+"|B", "prot93u_prot93c.log C3 == QA "+faclctl)

	// B3 and C3 (SYSPRV) ran after their READALL passes in the same
	// process, and VMS's RMS kept PX1's and PX2's upper levels in its
	// directory cache, so it didn't look them up again; govax keeps no
	// cache, and refuses them as VMS refused every first look (the phase
	// doc's A.4, answer 1).
	for _, r := range []string{"prot93u_prot93b.log B3", "prot93u_prot93c.log C3"} {
		for _, px := range []string{"PX1", "PX2"} {
			d := "PROT93$DISK:[PROT93." + px + ".PY.PZ]"
			add("VMS's directory cache", r+" == O- "+d+"X.DAT|G", r+" == QA "+d+"X.DAT", r+" == S- "+d+"*.*;*")
		}
	}
}

// prot93SearchStart is a search's report up to its first $SEARCH's
// status line.
func prot93SearchStart(lines []string) []string {
	for i, l := range lines {
		if strings.HasPrefix(l, "  $SEARCH ") {
			return lines[:i+1]
		}
	}

	return lines
}

// TestProt93ProgramOracle runs the probe's program as each identity of
// both rounds' runs (SYSTEM's without its four privileges, with READALL,
// and with all four; the accounts' passes without privileges and with
// each privilege alone) and holds each case's report to VMS's.
func TestProt93ProgramOracle(t *testing.T) {
	c, run := prot93Console(t, "prot93-exchange.dsk", false)

	var runs []prot93Run
	for _, l := range []struct{ log, user string }{
		{"prot93.log", "SYSTEM"},
		{"prot93u_prot93a.log", "PROT93A"},
		{"prot93u_prot93b.log", "PROT93B"},
		{"prot93u_prot93c.log", "PROT93C"},
		{"round2/prot93r.log", "SYSTEM"},
		{"round2/prot93v_prot93a.log", "PROT93A"},
		{"round2/prot93v_prot93b.log", "PROT93B"},
		{"round2/prot93v_prot93c.log", "PROT93C"},
	} {
		runs = append(runs, prot93Runs(t, l.log, l.user)...)
	}

	p := c.RTL.Process
	compared, skipped := 0, 0
	differed := map[string]bool{}

	for _, r := range runs {
		want := prot93Cases(r.output)
		if len(want) == 0 {
			continue // the image couldn't be run (round 1's), or nothing ran
		}

		p.UIC, p.CurrentPrivileges, p.Rights = uint32(r.uic), r.privileges, append(prot93InteractiveRights(), r.rights...)

		run("DEFINE PROT93_TAG " + r.tag)
		run("DEFINE/USER_MODE SYS$INPUT " + r.input)
		out := run("RUN PROT93$DISK:[000000]PROT93")
		got := prot93Cases(strings.Split(out, "\n"))

		if len(got) == 0 {
			t.Fatalf("%s %s: RUN printed %q", r.log, r.tag, out)
		}

		p.UIC, p.CurrentPrivileges, p.Rights = uint32(security.SystemUIC), security.AllPrivileges, prot93InteractiveRights()

		byLine := map[string][]prot93Case{}
		for _, g := range got {
			byLine[g.line] = append(byLine[g.line], g)
		}

		for _, w := range want {
			// The pass's own files (their names carry its tag) and the
			// cases that change the volume depend on the volume's state.
			if (prot93Changed[w.operation()] || strings.Contains(w.line, "_"+r.tag)) && !w.refused() {
				skipped++

				continue
			}

			list := byLine[w.line]
			if len(list) == 0 {
				t.Errorf("%s %s: %s: not run", r.log, r.tag, w.line)

				continue
			}

			g := list[0]
			byLine[w.line] = list[1:]
			compared++

			// An allowed search lists what the directory held then: its
			// $PARSE and first $SEARCH's status say what was allowed.
			gLines, wLines := prot93Normalize(g.lines), prot93Normalize(w.lines)
			if w.operation() == "S" && !w.refused() {
				gLines, wLines = prot93SearchStart(gLines), prot93SearchStart(wLines)
			}

			gl, wl := strings.Join(gLines, "\n"), strings.Join(wLines, "\n")
			key := r.log + " " + r.tag + " " + strings.ReplaceAll(w.line, "_"+r.tag, "_@")

			if why, ok := prot93Differences[key]; ok {
				if gl != wl {
					differed[key] = true

					t.Logf("%s %s: %s differs as expected (%s)", r.log, r.tag, w.line, why)
				}

				continue
			}

			if gl != wl {
				t.Errorf("%s %s: %s\n got:\n%s\nwant:\n%s", r.log, r.tag, w.line, gl, wl)
			}
		}
	}

	t.Logf("%d cases compared, %d changing cases VMS allowed left out", compared, skipped)

	for key := range prot93Differences {
		if !differed[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from prot93Differences", key)
		}
	}

	if compared < 1000 {
		t.Errorf("only %d cases compared", compared)
	}
}

// prot93InteractiveRights are the environmental identifiers every
// account in the probe held, INTERACTIVE and LOCAL.
func prot93InteractiveRights() []security.Identifier {
	return []security.Identifier{
		{Value: uint32(vmsdef.Symbols["KGB$K_INTERACTIVE_ID"])},
		{Value: uint32(vmsdef.Symbols["KGB$K_LOCAL_ID"])},
	}
}

// prot93Console is a console on which STARTUP.COM has run, with a copy of
// the local volume disk (testdata/disks) mounted on DUA1 as PROT93$DISK,
// and a function running a command and returning what it printed. It
// skips without the volume.
func prot93Console(t *testing.T, disk string, paging bool) (*Console, func(string) string) {
	t.Helper()

	if paging {
		withSettings(t, map[string]string{machine.PagingSetting: "true"})
	}

	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "disks", disk))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Skipf("no local copy of the probe's volume %s (testdata/prot93/README.md)", disk)
	}

	copied := filepath.Join(t.TempDir(), "prot93.dsk")
	copyFile(t, path, copied)

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

	for _, line := range []string{`MOUNT/WRITE DUA1 "` + copied + `"`, "DEFINE PROT93$DISK DUA1:"} {
		if out := run(line); strings.Contains(out, "%") && !strings.Contains(out, "-I-") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	return c, run
}

// TestProt93ActivationOracle runs PROT93.EXE as round 1's accounts did
// without privileges, on round 1's volume, where the image had SYSTEM's
// default protection, (RWED,RWED,RE,), so the world category can't run
// it: RUN's messages and status are VMS's (prot93u_prot93b.log's B0), in
// both paging modes (each loader reads the image its own way).
func TestProt93ActivationOracle(t *testing.T) {
	for _, paging := range []bool{false, true} {
		t.Run(fmt.Sprintf("paging=%v", paging), func(t *testing.T) { prot93Activation(t, paging) })
	}
}

// prot93Activation is TestProt93ActivationOracle in one paging mode.
func prot93Activation(t *testing.T, paging bool) {
	c, run := prot93Console(t, "prot93-round1.dsk", paging)

	log := prot93Log(t, "prot93u_prot93b.log")
	want := logSection(t, log, "$ RUN PROT93$DISK:[000000]PROT93", "$ DEFINE")

	p := c.RTL.Process
	p.UIC, p.CurrentPrivileges = uint32(security.NewUIC(0o200, 2)), prot93Privilege("NETMBX")|prot93Privilege("TMPMBX")

	run("DEFINE/USER_MODE SYS$INPUT PROT93$DISK:[000000]PROT93C.DAT")

	got := "$ RUN PROT93$DISK:[000000]PROT93\n" + run("RUN PROT93$DISK:[000000]PROT93") +
		"$ SHOW SYMBOL $STATUS\n" + run("SHOW SYMBOL $STATUS")

	if strings.TrimRight(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}

	// With READALL it runs (B1 ran it): the first case's line is there.
	p.CurrentPrivileges |= prot93Privilege("READALL")
	run("DEFINE PROT93_TAG B1")
	run("DEFINE/USER_MODE SYS$INPUT PROT93$DISK:[000000]PROT93C.DAT")

	if out := run("RUN PROT93$DISK:[000000]PROT93"); !strings.Contains(out, "== O- PROT93$DISK:[PROT93.F]FNONE.DAT|G") {
		t.Errorf("with READALL: %q", out)
	}
}
