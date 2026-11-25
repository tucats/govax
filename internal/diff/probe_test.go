package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// probeCase is one merged-form case of testdata/dif116 this package can run:
// its two files' records and the options its command gave.
type probeCase struct {
	ID, Master, Revision string
	Opts                 Options
	Merged               int32

	// Parallel and Separated say the case asked for /PARALLEL or
	// /SEPARATED (of both files) instead of the merged form.
	Parallel, Separated bool
}

var probeQualifier = regexp.MustCompile(`/([A-Z_]+)=(-?\d+)`)

// readProbe reads cases.txt's files, and the cases whose commands name two
// files and only /MATCH, /WINDOW, /MAXIMUM_DIFFERENCES, and /MERGED.
func readProbe(t *testing.T) (map[string][][]byte, []probeCase) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dif116", "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][][]byte{}

	var (
		cases []probeCase
		file  string
		id    string
	)

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "file "):
			f := strings.Fields(line)
			file, id = f[1], ""
			files[file] = [][]byte{}
		case strings.HasPrefix(line, "|"):
			files[file] = append(files[file], []byte(probeUnescape(line[1:])))
		case strings.HasPrefix(line, "=lines "):
			f := strings.Fields(line)
			n, _ := strconv.Atoi(f[2])

			for i := 1; i <= n; i++ {
				files[file] = append(files[file], []byte(fmt.Sprintf("%s %d", f[1], i)))
			}
		case strings.HasPrefix(line, "=at "):
			f := strings.SplitN(line, " ", 3)
			k, _ := strconv.Atoi(f[1])
			files[file][k-1] = []byte(probeUnescape(f[2]))
		case strings.HasPrefix(line, "case "):
			id = strings.Fields(line)[1]
		case strings.HasPrefix(line, "  command DIFFERENCES") && id != "":
			cmd := strings.TrimPrefix(line, "  command DIFFERENCES")
			f := strings.Fields(cmd)
			if len(f) == 2 {
				f = append([]string{""}, f...)
			}

			if len(f) != 3 {
				continue
			}

			c := probeCase{ID: id, Master: f[1], Revision: f[2], Opts: Options{Match: 3}, Merged: 1}
			rest := probeQualifier.ReplaceAllStringFunc(f[0], func(q string) string {
				m := probeQualifier.FindStringSubmatch(q)
				n, _ := strconv.Atoi(m[2])

				switch m[1] {
				case "MATCH":
					c.Opts.Match = int32(n)
				case "WINDOW":
					c.Opts.Window = n
				case "MAXIMUM_DIFFERENCES":
					c.Opts.MaxDifferences = n
				case "MERGED":
					c.Merged = int32(n)
				default:
					return q
				}

				return ""
			})

			for _, form := range []string{"/PARALLEL", "/SEPARATED"} {
				if strings.Contains(rest, form) {
					rest = strings.Replace(rest, form, "", 1)
					c.Parallel = c.Parallel || form == "/PARALLEL"
					c.Separated = c.Separated || form == "/SEPARATED"
				}
			}

			if rest == "" {
				cases = append(cases, c)
			}
		}
	}

	return files, cases
}

// probeSections are the probe's sections whose cases this package can run
// whole: two files named plainly, numeric qualifiers, the merged form, and
// the generated pairs run /PARALLEL (PG) and /SEPARATED (EG). (NAMES, FORMS,
// IGNORING, and FORMATS need the command or later subtasks.)
var probeSections = map[byte]bool{
	'A': true, 'M': true, 'L': true, 'G': true, 'H': true, 'X': true, 'Z': true, 'P': true, 'E': true,
}

// TestProbeListings holds the comparison and the listing, up to the echo,
// to every listing VMS 7.1 made of the probe's LAYOUT, MATCHING, LIMITS, and
// GENERATED cases (testdata/dif116/vax/out), merged, parallel (PGnnn), and
// separated (EGnnn). The echo, and the cases' messages and statuses, are
// internal/console's TestDif116MergedOracle's.
func TestProbeListings(t *testing.T) {
	files, cases := readProbe(t)
	ran := 0

	for _, c := range cases {
		if !probeSections[c.ID[0]] {
			continue
		}

		want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dif116", "vax", "out", c.ID+".DIF"))
		if err != nil {
			continue
		}

		ran++

		w := string(want)
		if i := strings.Index(w, "\nDIFFERENCES "); i >= 0 {
			w = w[:i+1]
		}

		a, b := files[c.Master], files[c.Revision]
		res := Compare(a, b, c.Opts)
		l := NewListing(132)
		List(l, Input{Name: "DUA1:[DIF116.IN]" + c.Master + ";1", Records: a},
			Input{Name: "DUA1:[DIF116.IN]" + c.Revision + ";1", Records: b}, res, Forms{
				Merged:            !c.Parallel && !c.Separated,
				MergedStyle:       Style{After: c.Merged},
				Parallel:          c.Parallel,
				SeparatedMaster:   c.Separated,
				SeparatedRevision: c.Separated,
			})
		Summary(l, res)

		if g := l.Text(); g != w {
			t.Errorf("%s %+v\nmaster:   %s\nrevision: %s\ngovax:\n%s\nVMS:\n%s", c.ID, c.Opts, show(a), show(b), g, w)
		}
	}

	if ran != 591 {
		t.Errorf("%d of the probe's listings ran, not 591", ran)
	}
}

// show is a file's records, numbered, on one line.
func show(r [][]byte) string {
	s := make([]string, len(r))
	for i, x := range r {
		s[i] = fmt.Sprintf("%d:%s", i+1, x)
	}

	return strings.Join(s, " ")
}

// probeUnescape undoes cases.txt's escapes: \\, \t, and \xNN.
func probeUnescape(s string) string {
	var b strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])

			continue
		}

		switch s[i+1] {
		case 't':
			b.WriteByte('\t')
			i++
		case 'x':
			v, _ := strconv.ParseUint(s[i+2:i+4], 16, 8)
			b.WriteByte(byte(v))
			i += 3
		default:
			b.WriteByte(s[i+1])
			i++
		}
	}

	return b.String()
}
