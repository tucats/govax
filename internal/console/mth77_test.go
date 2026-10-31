package console

import (
	"bytes"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The Phase 77 oracle (docs/PHASE-77 - MTHRTL.md, subtask 10): testdata/mth77's
// mth77.mar calls MTHRTL's routines case by case, and prints what each returned and
// signaled. vax/mth77-vax.txt is its output on VMS. govax's must match line for line
// but for expectedMTH77Differences, except that a floating result may differ in its
// last bits (the phase's Decision 4: exact last bits aren't a goal), which the test
// counts.

const mth77Dir = "../../testdata/mth77"

// mth77UlpLimit is the most a result may differ from VMS's, in last bits.
const mth77UlpLimit = 3

// mth77ComplexUlpLimit is the same for the functions of complex numbers whose VMS code
// rounds at each step, and so is off by more than that where a part partly cancels:
// complex logarithms, powers, and quotients. govax's parts are exact, then rounded once
// (the tests in internal/mthrtl hold them to a reference).
const mth77ComplexUlpLimit = 200

// expectedMTH77Differences are the cases whose lines differ from VMS's, by title, and
// why: what VMS does that nothing should depend on, or that a Go routine can't do.
var expectedMTH77Differences = map[string]string{
	"MTH$RANDOM()":                         "no argument: VMS uses what follows the argument list as the seed's address",
	"MTH$DSQRT(-1.0)":                      "VMS's user PC is inside the library for this one routine",
	"MTH$HEXP(RES,5.0E4931) CORR":          "VMS stores the handler's corrected R0 and R1 after it continues, where a Go routine has ended; the manual says a handler can't correct this",
	"MTH$HEXP(RES,-11356.53) FU":           "just past the format's limit, VMS leaves a dirty zero with no condition",
	"MTH$DTAND(3.0E-39) FU":                "VMS's D form leaves a dirty zero with no condition, where its F and G forms signal",
	"MTH$CVT_GA_DA(ARRAY,RES,3)":           "VMS signals in the middle of the loop, and returns 0 from it; a Go routine signals at the end",
	"OTS$POWHLU_R3(3.0,5)":                 "VMS's routine leaves R2 and R3 unchanged, so the last half of its result is lost; govax's is whole",
	"MTH$IMAX0()":                          "no arguments: VMS reads what follows the argument list",
	"MTH$JMAX0()":                          "no arguments: VMS reads what follows the argument list",
	"MTH$UMAX()":                           "no arguments: VMS reads what follows the argument list",
	"OTS$POWCC((-1.0,0.0),(2.0,0.0))":      "the imaginary part should be 0: each has a tiny error in the angle, VMS's larger",
	"OTS$POWCDCD_R3((-1.0,0.0),(2.0,0.0))": "the imaginary part should be 0: VMS's has a tiny error in the angle, and govax's rounds to 0",
	"OTS$POWCGCG_R3((-1.0,0.0),(2.0,0.0))": "the imaginary part should be 0: each has a tiny error in the angle, VMS's larger",
}

// runMTH77 assembles, links, and runs the probe on a fresh console, returning its
// status and its output.
func runMTH77(t *testing.T) (uint32, string) {
	t.Helper()

	c := newBootableConsole(t)
	c.CPU.SetDebug(c.CPU.Debug() &^ registers.DebugUserHalt)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join(mth77Dir, "mth77.mar"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "mth77")
	writeHostFile(t, path+".mar", string(src))

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar"}); err != nil {
		t.Fatalf("MACRO: %v\n%s", err, out.String())
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}}); err != nil {
		t.Fatalf("LINK: %v\n%s", err, out.String())
	}

	r0 := runImageBounded(t, c, path+".exe", 100_000_000)
	_, after, _ := strings.Cut(out.String(), "Kernel initialized\n\r")

	return r0, strings.ReplaceAll(after, "\r", "")
}

// mth77Line is a line taken apart: its title; the longwords after R= and M=, in order;
// and the rest, what the handler saw, with what depends on the system masked.
type mth77Line struct {
	title string
	regs  []string
	mem   []string
	rest  string
}

func parseMTH77(line string) mth77Line {
	end := len(line)

	for _, mark := range []string{" R=", " M=", " c="} {
		if i := strings.Index(line, mark); i >= 0 && i < end {
			end = i
		}
	}

	l := mth77Line{title: line[:end]}

	var into *[]string

	library := strings.Contains(line, " c=00168") // facility MTH

	var rest []string

	skip := false

	for _, field := range strings.Fields(line[end:]) {
		if skip {
			skip = false

			continue
		}

		switch {
		case strings.HasPrefix(field, "R="):
			into, field = &l.regs, field[2:]
		case strings.HasPrefix(field, "M="):
			into, field = &l.mem, field[2:]
		case strings.Contains(field, "="):
			into = nil
		}

		switch {
		case into != nil:
			*into = append(*into, field)
		case strings.HasPrefix(field, "pc="):
			// Where the signal came from: inside VMS's library, at the return
			// address in govax's (the phase's Decision 8).
		case strings.HasPrefix(field, "mr="):
			// R0 as the handler found it matters for the library's conditions; R1
			// shows in R= when it's part of the result, and is otherwise whatever
			// the routine had there. For another condition, neither means anything.
			if library {
				rest = append(rest, field)
			}

			skip = true
		case library && strings.HasPrefix(field, "a=") && field != "a=ret":
			rest = append(rest, "a=oth")
		default:
			rest = append(rest, field)
		}
	}

	l.rest = strings.Join(rest, " ")

	return l
}

// mth77Format is the floating format of an entry's result, by its name, and false for
// an entry with an integer result or a line the name doesn't settle, compared exactly.
func mth77Format(title string) (vaxfloat.Format, bool) {
	if len(title) > 2 && title[0] == '~' {
		return map[byte]vaxfloat.Format{'F': vaxfloat.F, 'D': vaxfloat.D, 'G': vaxfloat.G, 'H': vaxfloat.H}[title[1]], true
	}

	name, _, _ := strings.Cut(title, "(")
	_, name, _ = strings.Cut(name, "$")

	if strings.HasPrefix(name, "POWII") || strings.HasPrefix(name, "POWJJ") || strings.HasPrefix(name, "POWLU") {
		return 0, false
	}

	switch {
	case strings.Contains(name, "CVT_D_G"):
		return vaxfloat.G, true
	case strings.Contains(name, "CVT_G_D"):
		return vaxfloat.D, true
	case name == "DIM", strings.HasPrefix(name, "DIV") && !strings.HasPrefix(name, "DIVCD") && !strings.HasPrefix(name, "DIVCG"):
		return vaxfloat.F, true
	}

	name = strings.TrimPrefix(name, "POW")
	name = strings.TrimPrefix(name, "DIVC")
	name = strings.TrimPrefix(name, "MULC")

	if strings.HasPrefix(name, "CD") || strings.HasPrefix(name, "CG") || name == "CMPLX" {
		name = name[1:]
	}

	switch {
	case name == "":
		return vaxfloat.F, true
	case strings.HasPrefix(name, "J"), strings.HasPrefix(name, "I"), strings.HasPrefix(name, "U"), strings.HasPrefix(name, "SGN"):
		return 0, false
	case name[0] == 'H':
		return vaxfloat.H, true
	case name[0] == 'D':
		return vaxfloat.D, true
	case name[0] == 'G':
		return vaxfloat.G, true
	}

	return vaxfloat.F, true
}

// mth77Bits is words as format f's bits.
func mth77Bits(words []string) (vaxfloat.Bits, bool) {
	var b vaxfloat.Bits

	for i, w := range words {
		x, err := strconv.ParseUint(w, 16, 32)
		if err != nil {
			return b, false
		}

		switch i {
		case 0:
			b.Lo |= x
		case 1:
			b.Lo |= x << 32
		case 2:
			b.Hi |= x
		case 3:
			b.Hi |= x << 32
		}
	}

	return b, true
}

// mth77Ulps is how far apart two values of format f are, spelled as longwords, in
// last bits of the one VMS gave. A value with sign and exponent both 0 is 0.0 whatever
// its fraction: VMS's H routines leave such dirty zeros, which Unpack reads as 0.0. ok
// is false for what can't be compared so, a reserved operand.
func mth77Ulps(f vaxfloat.Format, got, want []string) (units float64, ok bool) {
	gb, ok1 := mth77Bits(got)
	wb, ok2 := mth77Bits(want)

	if !ok1 || !ok2 {
		return 0, false
	}

	g, err1 := vaxfloat.Unpack(f, gb)
	w, err2 := vaxfloat.Unpack(f, wb)

	if err1 != nil || err2 != nil {
		return 0, false
	}

	difference := new(big.Float).Sub(g.Big(), w.Big())
	if difference.Sign() == 0 {
		return 0, true
	}

	scale := w.Big()
	if scale.Sign() == 0 {
		scale = g.Big()
	}

	ulp := new(big.Float).SetMantExp(big.NewFloat(1), scale.MantExp(nil)-int(f.Precision()))
	units, _ = new(big.Float).Quo(difference.Abs(difference), ulp).Float64()

	return units, true
}

// mth77RoundsSteps says a line is one of mth77ComplexUlpLimit's.
func mth77RoundsSteps(line string) bool {
	return strings.Contains(line, "POWC") || strings.Contains(line, "DIVC") || strings.Contains(line, "CGLOG") ||
		strings.Contains(line, "CDLOG") || strings.Contains(line, "CLOG")
}

// firstCondition is what the handler saw first: the condition, up to the arguments.
func firstCondition(rest string) string {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

// jsbResultWords is how many of the 12 registers shown for a JSB entry no manual gives
// hold its result. The rest are the routine's scratch on VMS, and are left alone by a
// Go routine.
func jsbResultWords(title string) int {
	name, _, _ := strings.Cut(title, "(")
	_, name, _ = strings.Cut(name, "$")

	f, isFloat := mth77Format(title)
	if !isFloat {
		return 1
	}

	size := f.Size() / 4

	if strings.HasPrefix(name, "C") && !strings.Contains(name, "ABS") && !strings.HasPrefix(name, "COS") {
		return 2 * size
	}

	return size
}

// compareMTH77 compares two lines, and returns the largest difference in last bits
// between their results, and false if they differ otherwise.
func compareMTH77(got, want string) (float64, bool) {
	g, w := parseMTH77(got), parseMTH77(want)

	if g.title != w.title {
		return 0, false
	}

	// A second signal: VMS's routine went on after a handler continued from the first,
	// and faulted on the reserved operand it was given (a phase doc's difference, since
	// a Go routine has ended by then). The first condition is what can be compared.
	if strings.Contains(want, " h=00000002") {
		return 0, firstCondition(g.rest) == firstCondition(w.rest)
	}

	if g.rest != w.rest || len(g.regs) != len(w.regs) || len(g.mem) != len(w.mem) {
		return 0, false
	}

	if result := jsbResultWords(w.title); len(w.regs) == 12 && result > 0 {
		g.regs, w.regs = g.regs[:result], w.regs[:result]
	}

	f, isFloat := mth77Format(w.title)
	size := 1

	if isFloat {
		size = f.Size() / 4
	}

	worst := 0.0

	for _, pair := range [][2][]string{{g.regs, w.regs}, {g.mem, w.mem}} {
		for i := 0; i < len(pair[1]); i += size {
			end := min(i+size, len(pair[1]))
			if strings.Join(pair[0][i:end], "") == strings.Join(pair[1][i:end], "") {
				continue
			}

			if !isFloat {
				return 0, false
			}

			units, ok := mth77Ulps(f, pair[0][i:end], pair[1][i:end])
			if !ok {
				return 0, false
			}

			worst = max(worst, units)
		}
	}

	return worst, true
}

// TestMTH77Runs: the probe runs to its end on govax, one line per case, whatever VMS
// has to say. That's what's needed before a VAX is asked to run it.
func TestMTH77Runs(t *testing.T) {
	r0, out := runMTH77(t)
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	// MTH77_OUT names a file for govax's output, for study.
	if path := os.Getenv("MTH77_OUT"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	src, err := os.ReadFile(filepath.Join(mth77Dir, "mth77.mar"))
	if err != nil {
		t.Fatal(err)
	}

	titles := strings.Count(string(src), ".ASCID\t")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	if len(lines) != titles {
		t.Errorf("%d lines, want %d", len(lines), titles)
	}
}

func TestMTH77Oracle(t *testing.T) {
	want, err := os.ReadFile(filepath.Join(mth77Dir, "vax", "mth77-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("no VMS run yet (vax/mth77-vax.txt)")
	}

	if err != nil {
		t.Fatal(err)
	}

	_, out := runMTH77(t)
	got := strings.Split(strings.TrimRight(out, "\n"), "\n")
	vax := strings.Split(strings.TrimRight(strings.ReplaceAll(string(want), "\r", ""), "\n"), "\n")
	seen := map[string]bool{}

	var lines, exact, within int

	for i := range max(len(got), len(vax)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(vax) {
			w = vax[i]
		}

		lines++

		units, ok := compareMTH77(g, w)

		switch {
		case ok && units == 0:
			exact++

			continue
		case ok && units <= mth77UlpLimit:
			within++

			continue
		case ok && units <= mth77ComplexUlpLimit && mth77RoundsSteps(w):
			within++

			continue
		}

		title := parseMTH77(w).title
		if why, listed := expectedMTH77Differences[title]; listed {
			seen[title] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d (%g last bits):\n\tgot  %q\n\twant %q", i+1, units, g, w)
	}

	t.Logf("%d lines: %d the same, %d within %d last bits", lines, exact, within, mth77UlpLimit)

	for title := range expectedMTH77Differences {
		if !seen[title] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedMTH77Differences", title)
		}
	}
}
