package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vmserrors"
)

// docs/PHASE-82 - FORTRAN DEC extensions.md: testdata/link82 asks what LINK does
// with one program section given different attributes by different modules.
// TestLink82Oracle runs its LINK82.COM on govax, as the author ran it on VMS V7.1,
// and holds govax's log, case by case, and maps to VMS's.

// link82Dir holds the probe.
var link82Dir = filepath.Join("..", "..", "testdata", "link82")

// link82Cases are the cases whose lines in the log needn't match VMS's, and why.
var link82Cases = map[string]string{
	"C22": "govax's MACRO writes no object after VNORD's error, where VMS's does, and VMS's LINK refuses that object (ERRORS, NOIMGFIL)",
	"C23": "as C22",
	"C29": "the first module makes BLK VEC: VMS won't activate the image (BADVEC), and govax has no protected images (docs/DEVIATIONS.md)",
}

// link82Maps are the maps not compared, and why.
var link82Maps = map[string]string{
	"c22.map": "as C22 in link82Cases",
	"c23.map": "as C22 in link82Cases",
	"c26.map": "VMS shows VABS's contribution of no bytes to BLK, a REL section, and govax none (as it leaves out a module's other empty contributions); one case gives no rule",
	"c27.map": "VMS writes a map of a link that made no image, and govax none",
}

// link82Sections splits a log into its cases, by their === lines, each case's
// lines without the procedure's echo: what follows from the case's first line
// that isn't the procedure's own, LINK's messages, $STATUS, and RUN's messages.
func link82Sections(text string) map[string][]string {
	sections := map[string][]string{}
	name := ""

	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(line, "=== "):
			name = strings.TrimSuffix(strings.Fields(line)[1], ":")
		case name != "" && !strings.HasPrefix(line, "$ "):
			sections[name] = append(sections[name], line)
		}
	}

	return sections
}

// link82Status is a line of $STATUS.
var link82Status = regexp.MustCompile(`^  \$STATUS == `)

// TestLink82Oracle runs the probe in both paging modes.
func TestLink82Oracle(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		if paging {
			withSettings(t, map[string]string{machine.PagingSetting: "true"})
		}

		c, buf := newTestConsole(t)
		c.Paths = respath.New(nil, bootdata.FS)
		d := NewDispatcher(c, loadEvaxGrammar(t), nil)
		installPlainDebugger(c)

		if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
			t.Fatalf("vax.init: %v", err)
		}

		path := filepath.Join(t.TempDir(), "link82.dsk")
		if err := c.cmd().InitializeContainer(path, 21600, "LINK82", 0, "RD51"); err != nil {
			t.Fatal(err)
		}

		if err := c.cmd().Mount("DUA2:", path, true); err != nil {
			t.Fatal(err)
		}

		script := link68Read(t, filepath.Join(link82Dir, "exchange.cmd"))

		for line := range strings.SplitSeq(script, "\n") {
			if strings.HasPrefix(line, "COPY ") {
				shareFindRun(t, d, strings.ReplaceAll(line, "testdata/link82/", filepath.ToSlash(link82Dir)+"/"))
			}
		}

		shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
		buf.Reset()

		if err := d.Dispatch("@LINK82/OUTPUT=LINK82.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("@LINK82: %v\n%s", err, buf)
		}

		out := t.TempDir()

		copyout := link68Read(t, filepath.Join(link82Dir, "copyout.cmd"))
		for line := range strings.SplitSeq(copyout, "\n") {
			if strings.HasPrefix(line, "COPY ") {
				_ = d.Dispatch(strings.ReplaceAll(line, "testdata/link82/vax/", out+string(filepath.Separator)))
			}
		}

		t.Run("log", func(t *testing.T) {
			want := link82Sections(link68Read(t, filepath.Join(link82Dir, "vax", "link82.log")))
			got := link82Sections(link68Read(t, filepath.Join(out, "link82.log")))

			if len(got) != len(want) {
				t.Errorf("%d cases, VMS's %d", len(got), len(want))
			}

			for name, lines := range want {
				if _, listed := link82Cases[name]; listed {
					continue
				}

				if g, w := strings.Join(got[name], "\n"), strings.Join(lines, "\n"); g != w {
					t.Errorf("%s:\n%s\nVMS's:\n%s", name, g, w)
				}
			}
		})

		t.Run("statuses", func(t *testing.T) {
			// MACRO's messages are in its own words, and govax's error status is
			// its own, so only each module's severity is held to VMS's: VNORD's
			// error among them.
			statuses := func(file string) []string {
				var list []string

				for line := range strings.SplitSeq(strings.ReplaceAll(link68Read(t, file), "\r", ""), "\n") {
					if link82Status.MatchString(line) {
						hex := strings.TrimSuffix(line[strings.Index(line, "%X")+2:], `"`)

						status, err := strconv.ParseUint(hex, 16, 32)
						if err != nil {
							t.Fatal(err)
						}

						list = append(list, strconv.FormatUint(status&7, 10))
					}
				}

				return list
			}

			got := statuses(filepath.Join(out, "link82.mlg"))
			want := statuses(filepath.Join(link82Dir, "vax", "link82.mlg"))

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("MACRO's statuses:\n%s\nVMS's:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})

		maps, err := filepath.Glob(filepath.Join(link82Dir, "vax", "*.map"))
		if err != nil || len(maps) == 0 {
			t.Fatalf("no maps: %v", err)
		}

		for _, file := range maps {
			name := filepath.Base(file)
			if _, listed := link82Maps[name]; listed {
				continue
			}

			t.Run(name, func(t *testing.T) {
				if _, err := os.Stat(filepath.Join(out, name)); err != nil {
					t.Fatalf("govax's run made no %s", name)
				}

				link68CompareLines(t, name, link68MaskText(link68Read(t, filepath.Join(out, name)), name, name),
					link68MaskText(link68Read(t, file), name, name), link68TextDifferences)
			})
		}
	})
}
