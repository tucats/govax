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
// but for expectedMTH77Differences, and for the sweep's lines, whose results may
// differ in their last bits (the phase's Decision 4: exact last bits aren't a goal),
// which the test counts.

const mth77Dir = "../../testdata/mth77"

// mth77UlpLimit is the most a sweep result may differ from VMS's, in last bits.
const mth77UlpLimit = 2

// expectedMTH77Differences are the cases whose lines may differ from VMS's, by title,
// and why.
var expectedMTH77Differences = map[string]string{}

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

// mth77Title is a line's title: what's before the first " R=", " M=", or " c=".
func mth77Title(line string) string {
	end := len(line)

	for _, mark := range []string{" R=", " M=", " c="} {
		if i := strings.Index(line, mark); i >= 0 && i < end {
			end = i
		}
	}

	return line[:end]
}

// mth77Words are the hex longwords of a sweep line's results, R= and M= together.
func mth77Words(line string) []string {
	var words []string

	for _, field := range strings.Fields(line[len(mth77Title(line)):]) {
		_, field, ok := cutResultMark(field)
		if ok {
			words = append(words, field)
		} else if len(field) == 8 && len(words) > 0 {
			words = append(words, field)
		}
	}

	return words
}

func cutResultMark(field string) (string, string, bool) {
	for _, mark := range []string{"R=", "M="} {
		if rest, ok := strings.CutPrefix(field, mark); ok {
			return mark, rest, true
		}
	}

	return "", field, false
}

// mth77Ulps is how far apart two values of format f are, spelled as longwords, in
// units of the last bit of the larger.
func mth77Ulps(f vaxfloat.Format, got, want []string) (float64, bool) {
	value := func(words []string) (vaxfloat.Value, bool) {
		var b vaxfloat.Bits

		for i, w := range words {
			x, err := strconv.ParseUint(w, 16, 32)
			if err != nil {
				return vaxfloat.Value{}, false
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

		v, err := vaxfloat.Unpack(f, b)

		return v, err == nil
	}

	g, ok1 := value(got)
	w, ok2 := value(want)

	if !ok1 || !ok2 {
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

	exponent := scale.MantExp(nil)
	ulp := new(big.Float).SetMantExp(big.NewFloat(1), exponent-int(f.Precision()))
	units, _ := new(big.Float).Quo(difference.Abs(difference), ulp).Float64()

	return units, true
}

// compareMTH77Sweep compares one sweep line's results, value by value, and returns
// the largest difference in last bits, and false if they can't be compared so.
func compareMTH77Sweep(got, want string) (float64, bool) {
	if mth77Title(got) != mth77Title(want) {
		return 0, false
	}

	formats := map[byte]vaxfloat.Format{'F': vaxfloat.F, 'D': vaxfloat.D, 'G': vaxfloat.G, 'H': vaxfloat.H}
	f := formats[want[1]]
	size := f.Size() / 4
	g, w := mth77Words(got), mth77Words(want)

	if len(g) != len(w) || strings.Contains(got, " c=") != strings.Contains(want, " c=") {
		return 0, false
	}

	worst := 0.0

	for i := 0; i+size <= len(w); i += size {
		if strings.Join(g[i:i+size], "") == strings.Join(w[i:i+size], "") {
			continue
		}

		units, ok := mth77Ulps(f, g[i:i+size], w[i:i+size])
		if !ok {
			return 0, false
		}

		worst = max(worst, units)
	}

	return worst, true
}

// mth77Masked is a line with what depends on the system taken out. A signal argument
// of one of the library's conditions that isn't the return address can only be an
// address inside the library, so it's kept as "oth", and a change from "ret" still
// shows. For any other condition, the PC is where the instruction faulted, inside
// VMS's library and at the return address in govax's (the phase's Decision 8), and
// what the mechanism array holds is whatever the routine had in R0 and R1 then.
func mth77Masked(line string) string {
	fields := strings.Fields(line)
	library := false
	kept := fields[:0]

	for _, field := range fields {
		if strings.HasPrefix(field, "c=") {
			library = strings.HasPrefix(field, "c=00168") // facility MTH
		}

		switch {
		case library && strings.HasPrefix(field, "a=") && field != "a=ret":
			field = "a=oth"
		case strings.Contains(line, " c=") && !strings.Contains(line, " c=00168") && strings.HasPrefix(field, "pc="):
			continue
		case !library && strings.HasPrefix(field, "mr="):
			kept = append(kept, "mr=*")
			continue
		}

		kept = append(kept, field)
	}

	return maskMechanismR1(kept)
}

// maskMechanismR1 joins fields, leaving out the longword after "mr=*", the second half
// of what was masked.
func maskMechanismR1(fields []string) string {
	kept := fields[:0]

	for i, field := range fields {
		if i > 0 && fields[i-1] == "mr=*" {
			continue
		}

		kept = append(kept, field)
	}

	return strings.Join(kept, " ")
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
	var exact, within, sweep int

	for i := range max(len(got), len(vax)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(vax) {
			w = vax[i]
		}

		if strings.HasPrefix(w, "~") {
			sweep++

			units, ok := compareMTH77Sweep(g, w)

			switch {
			case !ok:
				t.Errorf("sweep line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
			case units == 0:
				exact++
			case units <= mth77UlpLimit:
				within++
			default:
				t.Errorf("sweep line %d is %g last bits off:\n\tgot  %q\n\twant %q", i+1, units, g, w)
			}

			continue
		}

		if mth77Masked(g) == mth77Masked(w) {
			continue
		}

		title := mth77Title(w)
		if why, ok := expectedMTH77Differences[title]; ok {
			seen[title] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	t.Logf("the sweep: %d lines, %d exact, %d within %d last bits", sweep, exact, within, mth77UlpLimit)

	for title := range expectedMTH77Differences {
		if !seen[title] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedMTH77Differences", title)
		}
	}
}
