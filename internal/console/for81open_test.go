package console

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// TestFor81OpenOracle holds FTNRTL's OPEN to VMS's FORRTL (Decision 1 of
// docs/PHASE-81 - FORTRAN files.md): each case is one of testdata/for81's, as a
// FORTRAN OPEN with its values in variables, so that FTNRTL decides them and not
// the compiler, and its IOSTAT must be the FOR number of what VMS's FOR$OPEN
// returned in vax/for81-vax.txt. Relative and indexed files wait for their
// subtasks, and USEROPEN too.
func TestFor81OpenOracle(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	// Each case's first status from FOR$OPEN.
	want := map[int]int{}

	for _, m := range regexp.MustCompile(`(?m)^#(\d+) .*\n=OPEN r=([0-9A-F]{8})`).FindAllStringSubmatch(string(text), -1) {
		n, _ := strconv.Atoi(m[1])
		r, _ := strconv.ParseUint(m[2], 16, 32)

		if r&1 != 0 {
			want[n] = 0
		} else {
			want[n] = int(r>>3) & 0xFFF
		}
	}

	type openCase struct {
		number int
		keys   string // the OPEN's keywords after FILE=, S the status's text unless given
	}

	var cases []openCase

	// A keyword's texts, with RECL=10 and STATUS='NEW' but for STATUS's own.
	texts := func(first int, keyword, tail string, values ...string) {
		for i, v := range values {
			if v == "" {
				continue
			}

			cases = append(cases, openCase{first + i, fmt.Sprintf("%s=T('%s'), %s", keyword, v, tail)})
		}
	}
	plain := "STATUS=T('NEW'), RECL=N(10)"

	texts(121, "STATUS", "RECL=N(10)", "OLD", "NEW", "SCRATCH", "UNKNOWN", "new", "NEW   ", " NEW", "NEWS", "NE")
	texts(131, "DISPOSE", plain, "KEEP", "SAVE", "DELETE", "PRINT", "PRINT/DELETE", "SUBMIT", "SUBMIT/DELETE", "SUBMIT/KEEP", "print/delete")
	texts(140, "ACCESS", plain, "SEQUENTIAL", "DIRECT", "KEYED", "APPEND", "OTHER")
	texts(145, "FORM", plain, "FORMATTED", "UNFORMATTED", "BINARY")
	texts(148, "CARRIAGECONTROL", plain, "FORTRAN", "LIST", "NONE", "PRINT")
	texts(152, "ORGANIZATION", plain, "SEQUENTIAL", "", "", "STREAM")
	texts(156, "RECORDTYPE", plain, "FIXED", "VARIABLE", "SEGMENTED", "STREAM", "STREAM_CR", "STREAM_LF", "VFC")
	texts(163, "BLANK", plain, "NULL", "ZERO", "BLANK")

	// RECL's limits, each form's list of values in turn.
	limits := []int{-1, 0, 1, 2, 3, 80, 133, 134, 255, 256, 2048, 8191, 8192, 16383, 16384, 32765, 32766, 32767, 32768, 65535, 65536}
	for i, form := range []string{
		"", "RECORDTYPE=T('FIXED'), ",
		"FORM=T('UNFORMATTED'), RECORDTYPE=T('VARIABLE'), ",
		"FORM=T('UNFORMATTED'), RECORDTYPE=T('SEGMENTED'), ",
		"ACCESS=T('DIRECT'), ",
	} {
		for j, n := range limits {
			cases = append(cases, openCase{166 + i*len(limits) + j, fmt.Sprintf("%sSTATUS=T('NEW'), RECL=N(%d)", form, n)})
		}
	}

	// Combinations (#331 to #366).
	cases = append(cases, []openCase{
		{331, "STATUS=T('NEW')"},
		{332, "STATUS=T('NEW'), FORM=T('UNFORMATTED')"},
		{333, "STATUS=T('NEW'), FORM=T('FORMATTED')"},
		{334, "STATUS=T('NEW'), ACCESS=T('DIRECT')"},
		{335, "STATUS=T('NEW'), ACCESS=T('DIRECT'), RECL=N(4)"},
		{336, "STATUS=T('NEW'), ACCESS=T('DIRECT'), RECL=N(4), FORM=T('FORMATTED')"},
		{337, "STATUS=T('NEW'), ACCESS=T('DIRECT'), RECL=N(4), RECORDTYPE=T('VARIABLE')"},
		{338, "STATUS=T('NEW'), ACCESS=T('DIRECT'), RECL=N(4), RECORDTYPE=T('SEGMENTED')"},
		{341, "STATUS=T('NEW'), RECORDTYPE=T('SEGMENTED'), FORM=T('FORMATTED')"},
		{342, "STATUS=T('NEW'), RECORDTYPE=T('SEGMENTED'), FORM=T('UNFORMATTED')"},
		{343, "STATUS=T('NEW'), RECORDTYPE=T('STREAM'), FORM=T('UNFORMATTED')"},
		{353, "STATUS=T('NEW'), READONLY"},
		{354, "STATUS=T('UNKNOWN'), READONLY"},
		{355, "STATUS=T('SCRATCH'), READONLY"},
		{356, "STATUS=T('SCRATCH')"},
		{357, "STATUS=T('SCRATCH'), DISPOSE=T('KEEP')"},
		{358, "STATUS=T('SCRATCH'), DISPOSE=T('PRINT')"},
		{359, "STATUS=T('NEW'), NOSPANBLOCKS"},
		{360, "STATUS=T('NEW'), SHARED"},
		{361, "STATUS=T('UNKNOWN'), DISPOSE=T('DELETE'), SHARED"},
		{362, "STATUS=T('NEW'), CARRIAGECONTROL=T('LIST'), FORM=T('UNFORMATTED')"},
		{363, "STATUS=T('NEW'), CARRIAGECONTROL=T('FORTRAN'), FORM=T('UNFORMATTED')"},
		{364, "STATUS=T('NEW'), BLANK=T('NULL'), FORM=T('UNFORMATTED')"},
	}...)

	d, c, buf := shareFindConsole(t, false)

	subst := regexp.MustCompile(`([A-Z]+)=T\('([^']*)'\)`)
	number := regexp.MustCompile(`([A-Z]+)=N\((-?\d+)\)`)
	got := map[int]int{}

	// The volume is small: a program of a few cases at a time.
	for chunk := range slices.Chunk(cases, 30) {
		program := []string{"      PROGRAM ORACLE", "      INTEGER N, IOS", "      CHARACTER*20 T1, T2, T3, T4"}

		for _, oc := range chunk {
			if _, ok := want[oc.number]; !ok {
				t.Fatalf("no status for #%d", oc.number)
			}

			// Each text and number in a variable of its own, set just before.
			keys := oc.keys

			for i, m := range subst.FindAllStringSubmatch(keys, -1) {
				program = append(program, fmt.Sprintf("      T%d = '%s'", i+1, m[2]))
				keys = strings.Replace(keys, m[0], fmt.Sprintf("%s=T%d", m[1], i+1), 1)
			}

			for _, m := range number.FindAllStringSubmatch(keys, -1) {
				program = append(program, "      N = "+m[2])
				keys = strings.Replace(keys, m[0], m[1]+"=N", 1)
			}

			program = append(program,
				fmt.Sprintf("      OPEN (10, FILE='F%d.DAT',", oc.number),
				"     1      "+keys+",",
				"     2      IOSTAT=IOS)",
				fmt.Sprintf("      WRITE (6, '(I4,I5)') %d, IOS", oc.number),
				"      CLOSE (10, DISP='DELETE', IOSTAT=IOS)",
				"      CLOSE (10)",
			)
		}

		program = append(program, "      END")

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]ORACLE.FOR"},
			rms.TextRecords, splitSource(strings.Join(program, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		for _, command := range []string{"FORTRAN ORACLE", "MACRO ORACLE", "LINK ORACLE"} {
			if err := d.Dispatch(command); err != nil {
				t.Fatalf("%s: %v\n%s", command, err, buf.String())
			}
		}

		buf.Reset()

		for line := range strings.Lines(shareRun(t, d, buf, "ORACLE")) {
			f := strings.Fields(line)
			if len(f) == 2 {
				n, e1 := strconv.Atoi(f[0])
				s, e2 := strconv.Atoi(f[1])

				if e1 == nil && e2 == nil {
					got[n] = s
				}
			}
		}

		shareFindRun(t, d, "DELETE ORACLE.*;*")
	}

	for _, oc := range cases {
		if s, ok := got[oc.number]; !ok {
			t.Errorf("#%d %s: no answer", oc.number, oc.keys)
		} else if s != want[oc.number] {
			t.Errorf("#%d %s: IOSTAT %d, VMS's %d", oc.number, oc.keys, s, want[oc.number])
		}
	}
}
