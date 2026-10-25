package console_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Phase 73's probe of the trace trap (testdata/trace73, docs/PHASE-73 -
// trace trap.md, subtask 4): VMS 7.1's own images run part by part on
// govax, each part's report and $STATUS held to VMS's log. The debugger
// sessions are internal/debugger's session oracle's.

// trace73Dir is the probe's directory, from this package's.
var trace73Dir = filepath.Join("..", "..", "testdata", "trace73")

// trace73Parts are round 1's parts, in TRACE73.COM's order.
var trace73Parts = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "N"}

// trace73Differences are the runs whose records differ from VMS's where a
// PC is outside the program, and why: they are compared without those
// records and the counts, which the records outside make differ.
var trace73Differences = map[string]string{
	// A service's vector entry: VMS's CHMK is four bytes (SYS$GETTIM+6),
	// govax's XFC two (+4); LIBRTL's routines are VMS's code, about 15
	// traces each, and govax's stubs, three (Decision 6).
	"TRACE73 E": "the service vector's and LIBRTL's code",
	// SYS$DCLAST's vector entry, as SYS$GETTIM's.
	"TRACE73 F": "the service vector's code",
	// VMS's LIB$SIGNAL and condition search, traced; govax's LIB$SIGNAL
	// is a shim (tracedSignal).
	"TRACE73B G": "LIB$SIGNAL's code",
	"TRACE73B M": "LIB$SIGNAL's code",
}

// trace73Outside matches a record whose PC is outside the program, or
// the counts line.
var trace73Outside = regexp.MustCompile(`^  ([TO] [0-9A-F]{8} (?:[A-Z$_]+\+[0-9A-F]{4}|P[01] [0-9A-F]{8}|S0 [0-9A-F]{8}) |traces: )`)

// trace73Console is a console with VMS's images on a volume, DUA1, the
// default directory.
func trace73Console(t *testing.T) func(string) string {
	t.Helper()

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "trace73.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 TRACE73`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, image := range []string{"TRACE73", "TRACE73NT", "TRACE73D", "TRACE73B"} {
		src, err := filepath.Abs(filepath.Join(trace73Dir, "vax", strings.ToLower(image)+".exe"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST/BINARY DUA1:[000000]` + image + ".EXE")
	}

	return dispatch
}

// TestTrace73Oracle runs every part of VMS's TRACE73 and round 2's
// TRACE73B on govax and matches VMS's output and $STATUS, but for the
// records outside the program in trace73Differences' runs; then part J
// without traceback (TRACE73NT, its dump by excmsgMatch's rules) and with
// /DEBUG's lines (TRACE73D run /NODEBUG).
func TestTrace73Oracle(t *testing.T) {
	dispatch := trace73Console(t)

	type run struct {
		image, define, part, command string
		want                         []string
	}

	var runs []run

	round1 := logRuns(t, trace73Dir, "trace73.log", "$   DEFINE TRACE73_PART ", "$   RUN TRACE73")
	for _, part := range trace73Parts {
		runs = append(runs, run{"TRACE73", "TRACE73_PART", part, "RUN TRACE73", round1[part]})
	}

	round2 := logRuns(t, trace73Dir, "trace73b.log", "$   DEFINE TRACE73B_PART ", "$   RUN TRACE73B")
	for _, part := range []string{"S", "G", "L", "M"} {
		runs = append(runs, run{"TRACE73B", "TRACE73B_PART", part, "RUN TRACE73B", round2[part]})
	}

	runs = append(runs,
		run{"TRACE73NT", "TRACE73_PART", "J", "RUN TRACE73NT",
			logRuns(t, trace73Dir, "trace73.log", "$ DEFINE TRACE73_PART ", "$ RUN TRACE73NT")["J"]},
		run{"TRACE73D", "TRACE73_PART", "J", "RUN/NODEBUG TRACE73D",
			logRuns(t, trace73Dir, "trace73.log", "$ DEFINE TRACE73_PART ", "$ RUN/NODEBUG TRACE73D")["J"]})

	for _, r := range runs {
		if len(r.want) == 0 {
			t.Fatalf("%s part %s: not in VMS's log", r.image, r.part)
		}

		dispatch("DEFINE/NOLOG " + r.define + " " + r.part)

		got := strings.Split(dispatch(r.command), "|")
		got = append(got, strings.TrimSpace(dispatch("SHOW SYMBOL $STATUS")))
		want := r.want

		if r.image == "TRACE73NT" {
			// R1 is what LIB$PUT_OUTPUT left: VMS's varies from run to
			// run (0, an S0 address; testdata/tb70's dumps), govax's shim
			// leaves a data address.
			if problems := excmsgMatch(got, want, false, []string{"R1"}); len(problems) > 0 {
				t.Errorf("%s part %s: %s\ngot:\n%s\nwant:\n%s", r.image, r.part,
					strings.Join(problems, "; "), strings.Join(got, "\n"), strings.Join(want, "\n"))
			}

			continue
		}

		if _, ok := trace73Differences[r.image+" "+r.part]; ok {
			got, want = trace73Inside(got), trace73Inside(want)
		}

		if g, w := strings.Join(got, "\n"), strings.Join(want, "\n"); g != w {
			t.Errorf("%s part %s:\ngot:\n%s\nwant:\n%s", r.image, r.part, g, w)
		}
	}
}

// trace73Inside is a run's lines without the records outside the program
// and the counts.
func trace73Inside(lines []string) []string {
	var out []string

	for _, l := range lines {
		if !trace73Outside.MatchString(l) {
			out = append(out, l)
		}
	}

	return out
}
