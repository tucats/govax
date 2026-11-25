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
	// /SEPARATED (of both files) instead of the merged form, and Merged
	// that it asked for the merged form too.
	Parallel, Separated, AlsoMerged bool

	// Width is /WIDTH, Radixes /MODE's, and SLP /SLP.
	Width   int
	Radixes []Radix
	SLP     bool

	// Ignore is /IGNORE's and /COMMENT_DELIMITER's (the master's type's
	// characters filled in by readProbe's caller).
	Ignore       Ignore
	TypeComments bool
}

var (
	probeIgnore    = regexp.MustCompile(`/IGNORE=(\([A-Z_=0-9,]+\)|[A-Z_=0-9]+)`)
	probeDelimiter = regexp.MustCompile(`/COMMENT_DELIMITER(=(\([^)]*\)|"[^"]*"|[A-Za-z]+))?`)
)

// probeCommentKeywords are /COMMENT_DELIMITER's keywords.
var probeCommentKeywords = map[string]byte{
	"COLON": ':', "COMMA": ',', "EXCLAMATION": '!', "FORM_FEED": '\f', "LEFT": '[',
	"RIGHT": ']', "SEMI_COLON": ';', "SLASH": '/', "SPACE": ' ', "TAB": '\t',
}

// probeIgnoreOf reads a command's /IGNORE and /COMMENT_DELIMITER into c,
// returning the command without them.
func probeIgnoreOf(c *probeCase, cmd string) string {
	cmd = probeIgnore.ReplaceAllStringFunc(cmd, func(q string) string {
		for k := range strings.SplitSeq(strings.Trim(probeIgnore.FindStringSubmatch(q)[1], "()"), ",") {
			name, value, _ := strings.Cut(k, "=")

			switch name {
			case "SPACING":
				c.Ignore.Spacing = true
			case "TRAILING_SPACES":
				c.Ignore.TrailingSpaces = true
			case "BLANK_LINES":
				c.Ignore.BlankLines = true
			case "CASE":
				c.Ignore.Case = true
			case "FORM_FEEDS":
				c.Ignore.FormFeeds = true
			case "HEADER":
				c.Ignore.Header = 2
				if value != "" {
					c.Ignore.Header, _ = strconv.Atoi(value)
				}
			case "COMMENTS":
				c.TypeComments = c.Ignore.Comments == nil
				c.Ignore.Comments = []Delimiter{}
			case "EXACT":
				c.Ignore.Shown = Exact
			case "PRETTY":
				c.Ignore.Shown = PrettyForm
			}
		}

		return ""
	})

	return probeDelimiter.ReplaceAllStringFunc(cmd, func(q string) string {
		c.Ignore.Comments, c.TypeComments = []Delimiter{}, true

		value := probeDelimiter.FindStringSubmatch(q)[2]
		if value == "" {
			return ""
		}

		c.TypeComments = false

		for v := range strings.SplitSeq(strings.Trim(value, "()"), ",") {
			ch := byte(0)

			switch {
			case strings.HasPrefix(v, `"`):
				ch = v[1]
			case len(v) == 1:
				ch = strings.ToUpper(v)[0]
			default:
				for k, x := range probeCommentKeywords {
					if strings.HasPrefix(k, v) {
						ch = x
					}
				}
			}

			c.Ignore.Comments = append(c.Ignore.Comments, Delimiter{Char: ch})
		}

		return ""
	})
}

var probeModes = regexp.MustCompile(`/MODE=(\([A-Z,]+\)|[A-Z]+)`)

// probeRadixes are /MODE's keywords as the probe abbreviates them.
var probeRadixes = map[string]Radix{
	"HEX": Hexadecimal, "HEXADECIMAL": Hexadecimal, "OCTAL": Octal, "ASCII": ASCII,
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

			c := probeCase{ID: id, Master: f[1], Revision: f[2], Opts: Options{Match: 3}, Merged: 1, Width: 132}
			rest := probeModes.ReplaceAllStringFunc(f[0], func(q string) string {
				for k := range strings.SplitSeq(strings.Trim(probeModes.FindStringSubmatch(q)[1], "()"), ",") {
					c.Radixes = append(c.Radixes, probeRadixes[k])
				}

				return ""
			})
			rest = probeQualifier.ReplaceAllStringFunc(rest, func(q string) string {
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
				case "WIDTH":
					c.Width = n
				default:
					return q
				}

				return ""
			})

			for _, form := range []string{"/PARALLEL", "/SEPARATED", "/MERGED", "/SLP"} {
				if strings.Contains(rest, form) {
					rest = strings.Replace(rest, form, "", 1)
					c.Parallel = c.Parallel || form == "/PARALLEL"
					c.Separated = c.Separated || form == "/SEPARATED"
					c.AlsoMerged = c.AlsoMerged || form == "/MERGED"
					c.SLP = c.SLP || form == "/SLP"
				}
			}

			c.Opts.BothSides = c.SLP
			rest = probeIgnoreOf(&c, rest)

			if rest == "" {
				cases = append(cases, c)
			}
		}
	}

	return files, cases
}

// probeSections are the probe's sections whose cases this package can run
// whole: two files named plainly, numeric qualifiers, /MODE, /IGNORE,
// /COMMENT_DELIMITER, and the merged, parallel, separated, and SLP forms (the
// FORMS, IGNORING, and FORMATS sections' cases that ask for nothing else).
// NAMES needs the command, and change bars aren't read here.
var probeSections = map[byte]bool{
	'A': true, 'M': true, 'L': true, 'G': true, 'H': true, 'X': true, 'Z': true, 'P': true, 'E': true,
	'F': true, 'S': true, 'R': true, 'I': true,
}

// probeListings is how many of the probe's listings TestProbeListings holds.
const probeListings = 770

// TestProbeListings holds the comparison and the listing, up to the echo,
// to every listing VMS 7.1 made of the probe's LAYOUT, MATCHING, LIMITS, and
// GENERATED cases (testdata/dif116/vax/out), merged, parallel (PGnnn),
// separated (EGnnn), and SLP (SAnn, SGnnn), and the FORMS, IGNORING, and
// FORMATS cases it can read, /MODE's among them. The echo, and the cases'
// messages and statuses, are internal/console's oracles'.
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

		w := string(want)
		if i := strings.Index(w, "\nDIFFERENCES "); i >= 0 {
			w = w[:i+1]
		}

		a, b := files[c.Master], files[c.Revision]
		if a == nil || b == nil {
			continue
		}

		ran++

		ig := c.Ignore
		if c.TypeComments {
			ig.Comments = append(ig.Comments, DefaultDelimiters(c.Master[strings.LastIndexByte(c.Master, '.')+1:])...)
		}

		keysA, master := ig.Prepare(Input{Name: "DUA1:[DIF116.IN]" + c.Master + ";1", Records: a})
		keysB, revision := ig.Prepare(Input{Name: "DUA1:[DIF116.IN]" + c.Revision + ";1", Records: b})

		res := Compare(keysA, keysB, c.Opts)
		l := NewListing(c.Width)

		if c.SLP {
			l.Lines = SLP(master, revision, res)
		} else {
			List(l, master, revision, res, Forms{
				Merged:            c.AlsoMerged || (!c.Parallel && !c.Separated),
				MergedStyle:       Style{After: c.Merged},
				Parallel:          c.Parallel,
				SeparatedMaster:   c.Separated,
				SeparatedRevision: c.Separated,
				Radixes:           c.Radixes,
			})
			Summary(l, res)
		}

		if g := l.Text(); g != w {
			t.Errorf("%s %+v\nmaster:   %s\nrevision: %s\ngovax:\n%s\nVMS:\n%s", c.ID, c.Opts, show(a), show(b), g, w)
		}
	}

	if ran != probeListings {
		t.Errorf("%d of the probe's listings ran, not %d", ran, probeListings)
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
