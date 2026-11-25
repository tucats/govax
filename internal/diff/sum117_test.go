package diff

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// sum117Escape reads testdata/sum117/cases.txt's record escapes: \\, \t,
// and \xNN.
var sum117Escape = regexp.MustCompile(`\\(x[0-9A-F]{2}|t|\\)`)

// readSum117Pairs reads testdata/sum117/cases.txt's ROUNDTRIP pairs: each
// case's master (ID.A) and revision (ID.B), by case ID.
func readSum117Pairs(t *testing.T) map[string][2][][]byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sum117", "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][][]byte{}
	cur := ""

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "file "):
			cur = strings.Fields(line)[1]
			files[cur] = [][]byte{}
		case strings.HasPrefix(line, "|") && cur != "":
			r := sum117Escape.ReplaceAllStringFunc(line[1:], func(e string) string {
				switch e {
				case `\t`:
					return "\t"
				case `\\`:
					return `\`
				}

				n, _ := strconv.ParseUint(e[2:], 16, 8)

				return string([]byte{byte(n)})
			})
			files[cur] = append(files[cur], []byte(r))
		case strings.HasPrefix(line, "case "):
			cur = ""
		}
	}

	pairs := map[string][2][][]byte{}

	for name, a := range files {
		id, ok := strings.CutSuffix(name, ".A")
		if !ok || (id[0] != 'R' && id[0] != 'T') {
			continue
		}

		pairs[id] = [2][][]byte{a, files[id+".B"]}
	}

	return pairs
}

// TestSum117Scripts holds /SLP's script to the 136 VMS 7.1 wrote in
// testdata/sum117's ROUNDTRIP section, byte for byte: among them revision
// records that start with an SLP operator, which VMS writes after a "<".
func TestSum117Scripts(t *testing.T) {
	pairs := readSum117Pairs(t)
	if len(pairs) != 136 {
		t.Fatalf("%d pairs, want 136", len(pairs))
	}

	for id, p := range pairs {
		want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sum117", "vax", "out", id+".DIF"))
		if err != nil {
			t.Fatal(err)
		}

		res := Compare(p[0], p[1], Options{Match: 3, BothSides: true})
		got := strings.Join(SLP(Input{Records: p[0]}, Input{Records: p[1]}, res), "\n") + "\n"

		if got != string(want) {
			t.Errorf("%s: script\n%s\nwant\n%s", id, got, want)
		}
	}
}
