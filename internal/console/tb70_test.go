package console_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// tb70Dir holds Phase 70's probe of traceback (README.md): its sources,
// and in vax/ VMS V7.1's log and images.
const tb70Dir = "../../testdata/tb70"

// tb70Parts are the parts TB70.COM's RUNALL runs, in its order.
var tb70Parts = strings.Split("S,A,O,I,Z,R,J,P,L,K,W,E,H,T,X,D,G,V,C,M,N", ",")

// tb70Console is a console with a fresh volume as DUA1 holding VMS's
// TB70 images and TB70SHR, the shareable image they use, with TB70SHR
// defined as TB70.COM defines it.
func tb70Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "tb70.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 TB70`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, image := range []string{"TB70", "TB70NT", "TB70D", "TB70SHR"} {
		src, err := filepath.Abs(filepath.Join(tb70Dir, "vax", strings.ToLower(image)+".exe"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST/BINARY DUA1:[000000]` + image + ".EXE")
	}

	dispatch("DEFINE TB70SHR DUA1:[000000]TB70SHR.EXE")

	return c, dispatch
}

// tb70VAXRuns returns VMS's output for each RUN/NODEBUG of image in
// TB70.COM's RUNALL, by part: the log's lines from the RUN to the SHOW
// SYMBOL $STATUS after it, without blank lines (which the console's
// dispatcher drops too), and the $STATUS line.
func tb70VAXRuns(t *testing.T, image string) map[string][]string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(tb70Dir, "vax", "tb70.log"))
	if err != nil {
		t.Fatal(err)
	}

	runs := map[string][]string{}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")
	part := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if p, ok := strings.CutPrefix(line, "$   DEFINE/NOLOG TB70_PART "); ok {
			part = p

			continue
		}

		if line != "$   RUN/NODEBUG "+image || part == "" {
			continue
		}

		var out []string

		for i++; i < len(lines) && !strings.HasPrefix(lines[i], "$ "); i++ {
			if lines[i] != "" {
				out = append(out, lines[i])
			}
		}

		if i+1 < len(lines) {
			out = append(out, strings.TrimSpace(lines[i+1]))
		}

		runs[part] = out
		part = ""
	}

	return runs
}

// tb70Masked masks the dump lines of a run (tb70VAXRuns's form) whose
// PC is outside the main image, which ends below 1000: PCs in LIBRTL, in
// TB70SHR, and in the system's AST exit, all of which govax places
// elsewhere than VMS did (LIBRTL's routines are shims in S0, TB70SHR
// follows the image, the AST exit is the SYS$CLRAST stub in P1).
func tb70Masked(lines []string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		out[i] = line

		if len(line) != 77 || line[67:69] != "  " {
			continue
		}

		if pc, err := strconv.ParseUint(line[69:], 16, 32); err == nil && (pc < 0x200 || pc >= 0x1000) {
			out[i] = "(a PC outside the image)"
		}
	}

	return out
}

// tb70DumpDifferences are the runs whose output differs from VMS's for a
// reason other than a PC's place, and why.
var tb70DumpDifferences = map[string]string{
	// LIB$MOVC3 is a shim: its access violation is reported at its
	// caller's PC, not in LIBRTL's code (VMS: PC=000130DE, its line
	// first).
	"L": "LIBRTL's routines are shims",
	// The access violation's message has TB70SHR's PC (VMS: 00024026).
	"M": "TB70SHR's place",
	// An exit handler's condition gets the "improperly handled" dump
	// (subtask 7).
	"X": "EXE$EXCMSG's dump",
}

// TestTB70Traceback runs each part of VMS's TB70 and TB70D (linked
// /DEBUG, run /NODEBUG, which gives the dump its line numbers) on govax,
// and matches VMS's output, dumps and all, but for the PCs outside the
// image (tb70Masked) and tb70DumpDifferences.
func TestTB70Traceback(t *testing.T) {
	_, dispatch := tb70Console(t)

	for _, image := range []string{"TB70", "TB70D"} {
		want := tb70VAXRuns(t, image)

		for _, part := range tb70Parts {
			if _, ok := tb70DumpDifferences[part]; ok {
				continue
			}

			dispatch("DEFINE/NOLOG TB70_PART " + part)

			got := strings.Split(dispatch("RUN/NODEBUG "+image), "|")
			got = append(got, strings.TrimSpace(dispatch("SHOW SYMBOL $STATUS")))

			g, w := strings.Join(tb70Masked(got), "\n"), strings.Join(tb70Masked(want[part]), "\n")
			if g != w {
				t.Errorf("%s part %s:\ngot:\n%s\nwant:\n%s", image, part, g, w)
			}
		}
	}
}
