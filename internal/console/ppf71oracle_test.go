package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Phase 71 oracle (docs/PHASE-71 - process-permanent files.md,
// subtask 9): testdata/ppf71's probe, run on govax, held to VMS 7.1's
// logs.

// ppf71Dir is the probe's directory, from this package's.
var ppf71Dir = filepath.Join("..", "..", "testdata", "ppf71")

// ppf71Masks are what VMS's logs and govax's may differ in beyond
// ppfMasks: the node's and the terminal's names (SIMVAX and OPA0 on the
// VAX; their bytes as F$CVUI shows them, and the device text after the
// first four bytes of a string that starts with the node's name), RFAs (a
// log's records sit where its earlier lines put them, and a procedure's
// records' RFAs aren't modeled), and times.
var ppf71Masks = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`SIMVAX`), "GOVAX"},
	{regexp.MustCompile(`OPA0`), "TTA0"},
	{regexp.MustCompile(`bytes 4F 50 3041`), "bytes 54 54 3041"},
	{regexp.MustCompile(`bytes 79 80 12353`), "bytes 84 84 12353"},
	{regexp.MustCompile(`bytes 95 \d+ \d+ after 4 "[^"]*\$`), `bytes 95 ? ? after 4 "?$`},
	{regexp.MustCompile(`bytes 5F [0-9A-F]{2} [0-9A-F]{4}, string "_GOVAX\$([^"]*)", after 4 "[^"]*"`), `bytes ?, string "_GOVAX$$$1"`},
	{regexp.MustCompile(`rfa [0-9A-F]{8},[0-9A-F]{4}`), "rfa ?"},
	{regexp.MustCompile(`\d\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d`), "<time>"},
}

// ppf71VAXOnly are VMS's lines that come from the VAX's own setup, not
// from the probe, rewritten as govax's setup has them ("" drops the
// line):
//
//   - its login gave SYS$DISK an executive-mode name too, and an earlier
//     probe left TB70SHR defined;
//   - SET DEFAULT's SYS$DISK has the CRELOG attribute on VMS (%X0004 in
//     $TRNLNM's attributes), which govax's doesn't record;
//   - EXCHANGE copied the procedures in with carriage-return carriage
//     control (RAT 02), govax's COPY/HOST copies them in with none.
var ppf71VAXOnly = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`^  "SYS\$DISK" \[exec\] = .*$`), ""},
	{regexp.MustCompile(`^  "TB70SHR" = .*$`), ""},
	{regexp.MustCompile(`^  "SYS\$DISK" \[super\] = `), `  "SYS$$DISK" = `},
	{regexp.MustCompile(`^(SYS\$DISK at mode \d: .*attributes 0000040)4`), "${1}0"},
	{regexp.MustCompile(`^(  fac (02|82|01) shr 00 org 00 rfm 05) rat 02`), "$1 rat 00"},
}

// ppf71Lines returns a log's lines from its first verified line on (the
// VAX's session verified the lines before SET VERIFY, govax's doesn't),
// with the version line's answer replaced and every mask applied; vax
// applies ppf71VAXOnly too.
func ppf71Lines(text string, vax bool) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")

	for i, line := range raw {
		if strings.HasPrefix(line, `$ WRITE SYS$OUTPUT F$GETSYI("VERSION")`) && i+1 < len(raw) {
			raw = raw[i:]
			raw[1] = "<version>"

			break
		}
	}

	var lines []string

	for _, line := range raw {
		line = maskPPF(line)

		for _, m := range ppf71Masks {
			line = m.re.ReplaceAllString(line, m.with)
		}

		if vax {
			dropped := false

			for _, m := range ppf71VAXOnly {
				if m.re.MatchString(line) && m.with == "" {
					dropped = true

					break
				}

				line = m.re.ReplaceAllString(line, m.with)
			}

			if dropped {
				continue
			}
		}

		lines = append(lines, line)
	}

	return lines
}

// comparePPF71 reports where govax's lines differ from VMS's, the first
// few differences with their line numbers.
func comparePPF71(t *testing.T, name string, got, want []string) {
	t.Helper()

	shown := 0

	for i := 0; i < max(len(got), len(want)); i++ {
		g, w := "<none>", "<none>"
		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g == w {
			continue
		}

		if shown++; shown > 10 {
			t.Errorf("%s: more differences", name)

			return
		}

		t.Errorf("%s: line %d:\n got %s\nwant %s", name, i+1, g, w)
	}
}

// TestPPF71Oracle runs testdata/ppf71's probe on govax, as the README's
// VAX run ran it (@PPF71/OUTPUT=PPF71.LOG with the volume's [000000] the
// default directory; the program assembled and linked by the procedure),
// and holds the procedure's log, the nested procedure's, the
// subprocess's, and PPF71B.DAT to VMS 7.1's, line for line, under the
// masks above and ppfMasks (the entries' numbers, the strings' lengths,
// STVs, ALQ, channels, and the disk's DEVCHAR).
func TestPPF71Oracle(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dir := t.TempDir()
	dsk := filepath.Join(dir, "ppf71.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 PPF71`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)

	for _, f := range []string{"ppf71.mar", "ppf71.com", "ppf71s.com", "ppf71p.com"} {
		src, err := filepath.Abs(filepath.Join(ppf71Dir, f))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(f))
	}

	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("@PPF71/OUTPUT=PPF71.LOG")

	for _, f := range []string{"ppf71.log", "ppf71s.log", "ppf71p.log", "ppf71b.dat"} {
		host := filepath.Join(dir, f)
		dispatch(fmt.Sprintf(`COPY DUA1:[000000]%s "%s"/HOST`, strings.ToUpper(f), host))

		got, err := os.ReadFile(host)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}

		want, err := os.ReadFile(filepath.Join(ppf71Dir, "vax", f))
		if err != nil {
			t.Fatal(err)
		}

		if f == "ppf71b.dat" {
			if g, w := string(got), strings.ReplaceAll(string(want), "\r", ""); g != w {
				t.Errorf("PPF71B.DAT:\n%s\nwant:\n%s", g, w)
			}

			continue
		}

		comparePPF71(t, strings.ToUpper(f), ppf71Lines(string(got), false), ppf71Lines(string(want), true))
	}
}

// TestPPF71Terminal runs VMS 7.1's own PPF71.EXE at govax's terminal,
// parts T, D, and O, as the README's step 3 ran them at the VAX's console
// terminal, where no /OUTPUT stands between the program and the
// terminal's strings, and holds the program's lines to VMS's
// (vax/ppf71-term.log; its blank lines, the Returns that answered O1's
// $FIND and $GET of the terminal, are the test's input here), under the
// same masks. PPF71_ALIAS and PPF71_DCLCOPY are defined as the procedure,
// run first on the VAX, left them.
func TestPPF71Terminal(t *testing.T) {
	c, dispatch := wsDispatcher(t)
	dir := t.TempDir()
	dsk := filepath.Join(dir, "ppf71.dsk")

	image, err := filepath.Abs(filepath.Join(ppf71Dir, "vax", "ppf71.exe"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 PPF71`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch(`COPY "` + image + `"/HOST/BINARY DUA1:[000000]PPF71.EXE`)
	dispatch("DEFINE PPF71_ALIAS SYS$OUTPUT")
	dispatch(`DEFINE PPF71_DCLCOPY "_GOVAX$DUA1:"`)

	c.In = strings.NewReader("\n\n")

	var got []string

	for _, part := range []string{"T", "D", "O"} {
		dispatch("DEFINE/USER_MODE PPF71_PART " + part)
		got = append(got, strings.Split(dispatch("RUN PPF71"), "|")...)
	}

	data, err := os.ReadFile(filepath.Join(ppf71Dir, "vax", "ppf71-term.log"))
	if err != nil {
		t.Fatal(err)
	}

	var want []string

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		if line == "" || strings.HasPrefix(line, "$ ") {
			continue
		}

		want = append(want, line)
	}

	comparePPF71(t, "the terminal run", ppf71Lines(strings.Join(got, "\n"), false), ppf71Lines(strings.Join(want, "\n"), true))
}
