package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const for83CondDir = "../../testdata/for83/"

// for83Case is one case of a run's output: its title line and what followed.
type for83Case struct {
	title string
	lines []string
}

// for83Cases splits a program's output at each "#n title" line.
func for83Cases(text string) []for83Case {
	var cases []for83Case

	for line := range strings.Lines(strings.ReplaceAll(text, "\r", "")) {
		line = strings.TrimRight(line, "\n")

		switch {
		case strings.HasPrefix(line, "#"):
			cases = append(cases, for83Case{title: line})
		case len(cases) > 0:
			cases[len(cases)-1].lines = append(cases[len(cases)-1].lines, line)
		}
	}

	return cases
}

// for83RunPart builds one of the probe's programs on a new volume and runs its
// cases as FOR83.COM does, returning what FOR83R.COM wrote.
func for83RunPart(t *testing.T, dispatch func(string) string, program string, count int, link string, files ...string) string {
	t.Helper()

	dir := t.TempDir()
	dsk := filepath.Join(dir, "for83.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR83`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range append([]string{program + ".MAR", "FOR83R.COM"}, files...) {
		host, err := filepath.Abs(for83CondDir + strings.ToLower(name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + host + `"/HOST ` + name)
	}

	if out := dispatch("MACRO/NOLIST " + program); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP " + program + link); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	dispatch(fmt.Sprintf("@FOR83R/OUTPUT=OUT.TXT %s %d", program, count))

	out := filepath.Join(dir, "out.txt")
	dispatch(`COPY OUT.TXT "` + out + `"/HOST`)

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	return string(got)
}

// for83Register is a register's value on a handler's line.
var for83Register = regexp.MustCompile(` (r[01])=([0-9A-F]{8})`)

// for83Sentinel is a value a case set up, which govax must show as VMS does; any
// other R0 or R1 on a mechanism array's line is what the program's last call left,
// LIB$PUT_OUTPUT's and $FAOL's work, which isn't either's business.
var for83Sentinel = regexp.MustCompile(`^(11111111|22222222|33333333|44444444)$`)

// for83MaskMechanism masks the registers of a "=m" line unless VMS's has a
// sentinel there.
func for83MaskMechanism(line, vms string) string {
	if !strings.HasPrefix(line, " =m ") {
		return line
	}

	values := for83Register.FindAllStringSubmatch(vms, -1)
	i := 0

	return for83Register.ReplaceAllStringFunc(line, func(m string) string {
		defer func() { i++ }()

		if i < len(values) && for83Sentinel.MatchString(values[i][2]) {
			return m
		}

		return " " + m[1:3] + "=<masked>"
	})
}

// for83PSLCodes masks a PSL's condition codes. The cases that end up in HND2 ran
// MOVAB G^LIB$FIXUP_FLT just before the fault, whose N is set by where LIBRTL is,
// in P0 on VMS and S0 on govax.
var for83PSLCodes = regexp.MustCompile(`psl=(\w{7})\w`)

// for83ConditionMasked are part C's cases where an instruction before the fault
// sets the codes in LIBRTL's place: the "as handler" case of each FIXUP_FLT
// failure.
var for83ConditionMasked = map[int]bool{58: true, 82: true, 85: true}

// for83FaultInRoutine are part C's cases whose access violation VMS takes in the
// routine's own code, from what it found past a short argument list or at a null
// address. On VMS it's a fault at a PC in LIBRTL, which continuing runs again until
// the program stops itself. govax's routines are shims, whose memory faults are
// conditions in the caller's frame that return SS$_ACCVIO if continued (librtl's
// fault.go; docs/PHASE-53, Decision 5), so all that's compared is the condition.
var for83FaultInRoutine = map[int]bool{32: true, 33: true, 39: true, 97: true, 103: true, 109: true}

// TestFor83ConditionOracle runs part C, LIBRTL's condition routines, as
// FOR83.COM does and holds each case to VMS's run.
func TestFor83ConditionOracle(t *testing.T) {
	_, dispatch := wsDispatcher(t)

	got := for83RunPart(t, dispatch, "FOR83C", 109, "")

	data, err := os.ReadFile(for83CondDir + "vax/for83c-vax.txt")
	if err != nil {
		t.Fatal(err)
	}

	want, have := for83Cases(string(data)), for83Cases(got)
	if len(want) != 109 || len(have) != len(want) {
		t.Fatalf("%d cases of VMS's, %d of govax's", len(want), len(have))
	}

	for i, w := range want {
		n, g := i+1, have[i]

		if for83FaultInRoutine[n] {
			if len(g.lines) == 0 || !strings.Contains(g.lines[0], "cond=0000000C") {
				t.Errorf("%s: no access violation: %q", w.title, g.lines)
			}

			continue
		}

		if g.title != w.title || len(g.lines) != len(w.lines) {
			t.Errorf("%s: govax's output differs:\n%s\nVMS's:\n%s", w.title, strings.Join(g.lines, "\n"), strings.Join(w.lines, "\n"))

			continue
		}

		for j, line := range w.lines {
			a, b := for83MaskMechanism(g.lines[j], line), for83MaskMechanism(line, line)

			if for83ConditionMasked[n] {
				a, b = for83PSLCodes.ReplaceAllString(a, "psl=$1"), for83PSLCodes.ReplaceAllString(b, "psl=$1")
			}

			if a != b {
				t.Errorf("%s, line %d:\n govax %s\n   VMS %s", w.title, j+1, a, b)
			}
		}
	}
}

// for83PC masks a PC in FORRTL: where the library is.
var for83PC = regexp.MustCompile(`pc=\+\w+|user PC \w+`)

// TestFor83ContinueOnFORRTL runs round 2 of part B (testdata/for83/gen.go's
// continueCases) on VMS's FORRTL.EXE over govax: which errors FORRTL goes on
// from. Whether it does is FORRTL's own code, so the result is FTNRTL's
// requirement. vax/for83e-forrtl.txt is that run; with FOR83E_WRITE set the test
// writes it instead.
func TestFor83ContinueOnFORRTL(t *testing.T) {
	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)

	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	got := for83RunPart(t, dispatch, "FOR83E", 18, ",FOR83E/OPTIONS", "FOR83E.OPT", "FOR83F.DAT", "FOR83G.DAT", "FOR83H.DAT")
	got = for83PC.ReplaceAllString(strings.ReplaceAll(got, "\r", ""), "pc")
	path := for83CondDir + "vax/for83e-forrtl.txt"

	if os.Getenv("FOR83E_WRITE") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("FORRTL's run differs from %s:\n%s", path, got)
	}
}

// for83PartBMask takes out of part B's output what depends on where things are:
// PCs in FORRTL or the program, addresses in a traceback, and R0 and R1 on a
// mechanism array's line, which FORRTL's calls of RMS and LIBRTL leave there.
var (
	for83Mechanism = regexp.MustCompile(`( =m n=\d+ frame=\+\w+ depth=\d+) r0=\w+ r1=\w+`)
	for83BackR0    = regexp.MustCompile(`^ =(back|continued|after|hook's statement) r0=\w+`)
	for83Traceback = regexp.MustCompile(`^\s+[0-9A-F]{8}\s+[0-9A-F]{8}$`)
	for83UserPC    = regexp.MustCompile(`user PC \w+`)
)

func for83PartBMask(text string) []string {
	var lines []string

	// VMS's TYPE ends no line after a record of a file with no carriage control,
	// case 33's unformatted file, and govax's does: the phase doc's log.
	text = strings.Replace(text, "partial#34", "partial\n#34", 1)

	for line := range strings.Lines(strings.ReplaceAll(text, "\r", "")) {
		line = strings.TrimRight(line, "\n")

		switch {
		case for83Traceback.MatchString(line), strings.HasPrefix(line, "$ "):
			continue
		case strings.HasPrefix(line, "FOR83B "):
			line = "FOR83B traceback line"
		}

		line = for83PC.ReplaceAllString(line, "pc")
		line = for83UserPC.ReplaceAllString(line, "user PC")
		line = for83Mechanism.ReplaceAllString(line, "$1")
		line = for83BackR0.ReplaceAllString(line, " =$1")
		lines = append(lines, strings.ReplaceAll(line, "DUA1:", "DUA2:"))
	}

	return lines
}

// TestFor83PartBOnFORRTL runs part B on VMS's FORRTL.EXE over govax's services,
// RMS, LIBRTL, and DCL, and holds it to VMS's whole run, so a difference is
// govax's.
func TestFor83PartBOnFORRTL(t *testing.T) {
	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)

	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	got := for83PartBMask(for83RunPart(t, dispatch, "FOR83B", 64, ",FOR83E/OPTIONS", "FOR83E.OPT", "FOR83D.DAT", "FOR83E.DAT"))

	data, err := os.ReadFile(for83CondDir + "vax/for83b-vax.txt")
	if err != nil {
		t.Fatal(err)
	}

	want := for83PartBMask(string(data))

	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			t.Fatalf("line %d differs:\n govax %q\n   VMS %q", i+1, got[i], want[i])
		}
	}

	if len(got) != len(want) {
		t.Errorf("%d lines of govax's, %d of VMS's", len(got), len(want))
	}
}
