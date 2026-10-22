package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Tests of the files DCL's OPEN opens as process-permanent files
// (docs/PHASE-71 - process-permanent files.md, subtask 7;
// internal/dclshell/dclfile.go).

// TestDCLOpenFiles_partF runs part F of testdata/ppf71 on govax, with VMS
// 7.1's own PPF71.EXE: an image's indirect opens of the names OPEN
// defined reach DCL's files, and DCL and the image read and write each
// file's one stream in turn, as VMS 7.1's log shows.
func TestDCLOpenFiles_partF(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dir := t.TempDir()
	dsk := filepath.Join(dir, "ppf.dsk")

	image, err := filepath.Abs(filepath.Join("..", "..", "testdata", "ppf71", "vax", "ppf71.exe"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PPF71`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch(`COPY "` + image + `"/HOST/BINARY DUA1:[000000]PPF71.EXE`)

	proc := writeProcedure(t, dir, "partf.com",
		`$ CREATE PPF71A.DAT`,
		`A1 first record`,
		`A2 second record`,
		`A3 third record`,
		`A4 fourth record`,
		`A5 fifth record`,
		`A6 sixth record`,
		`A7 seventh record`,
		`A8 eighth record`,
		`$ OPEN/READ PPF71_IN PPF71A.DAT`,
		`$ READ PPF71_IN REC`,
		`$ OPEN/WRITE PPF71_OUT PPF71B.DAT`,
		`$ WRITE PPF71_OUT "B1 from DCL"`,
		`$ DEFINE/USER_MODE PPF71_PART F`,
		`$ RUN PPF71`,
		`$ READ PPF71_IN REC`,
		`$ WRITE SYS$OUTPUT REC`,
		`$ WRITE PPF71_OUT "B5 from DCL, after the image"`,
		`$ CLOSE PPF71_IN`,
		`$ CLOSE PPF71_OUT`,
		`$ TYPE PPF71B.DAT`,
	)

	got := strings.Split(dispatch("@"+proc), "|")

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ppf71", "vax", "ppf71.log"))
	if err != nil {
		t.Fatal(err)
	}

	log := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")
	want := linesBetween(t, log, "F1 the names", "$ SHOW SYMBOL $STATUS")
	want = append(want, "A7 seventh record") // DCL's READ after the image
	want = append(want, linesBetween(t, log, "$ TYPE PPF71B.DAT", "$ DIRECTORY/FULL PPF71B.DAT")[1:]...)

	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}

	for i := range want {
		if g, w := maskPPF(got[i]), maskPPF(strings.ReplaceAll(want[i], "SIMVAX", "GOVAX")); g != w {
			t.Errorf("line %d:\n got %s\nwant %s", i+1, got[i], want[i])
		}
	}
}

// linesBetween returns log's lines from the one that is first up to the
// one that is last, not including it.
func linesBetween(t *testing.T, log []string, first, last string) []string {
	t.Helper()

	for i, line := range log {
		if line != first {
			continue
		}

		for j := i + 1; j < len(log); j++ {
			if log[j] == last {
				return log[i:j]
			}
		}
	}

	t.Fatalf("no %q ... %q in the log", first, last)

	return nil
}

// ppfMasks are what VMS's log and govax's may differ in: the entries'
// and streams' numbers (the index in an IFI, ISI, or word), the string's
// length (the node's name), STVs, ALQ, the channel, and the disk's
// characteristics from $GETDVIW.
var ppfMasks = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`\b(ifi|isi|00) ([0-9A-F]{3})[0-9A-F]\b`), "$1 $2?"},
	{regexp.MustCompile(`\b(80|81)[0-9A-F]{2}, string`), "$1??, string"},
	{regexp.MustCompile(`length \d+`), "length ?"},
	{regexp.MustCompile(`stv [0-9A-F]{8}`), "stv ?"},
	{regexp.MustCompile(`alq \d+`), "alq ?"},
	{regexp.MustCompile(`channel \d+`), "channel ?"},
	{regexp.MustCompile(`char [0-9A-F]{8}`), "char ?"},
}

// maskPPF is line with ppfMasks applied.
func maskPPF(line string) string {
	for _, m := range ppfMasks {
		line = m.re.ReplaceAllString(line, m.with)
	}

	return line
}
