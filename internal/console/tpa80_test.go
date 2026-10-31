package console_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 80 probe of LIB$TPARSE, LIB$TABLE_PARSE, and $FILESCAN
// (docs/PHASE-80 - FORTRAN formatted IO.md, subtask 10): testdata/tpa80's
// tpa80.mar runs them case by case, and prints what each returned and stored.
// vax/tpa80-vax.txt is its output on VMS, which govax's must match line for
// line, but for expectedTPA80Differences.

var tpa80Dir = filepath.Join("..", "..", "testdata", "tpa80")

// runTPA80 assembles, links, and runs the probe, and returns its output, a
// line an element.
func runTPA80(t *testing.T) []string {
	t.Helper()

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "tpa80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 TPA80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range []string{"tpa80.mar", "tpa80b.mar"} {
		src, err := filepath.Abs(filepath.Join(tpa80Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + strings.ToUpper(name))
	}

	if out := dispatch("MACRO/NOLIST TPA80"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("MACRO/NOLIST TPA80B"); out != "" {
		t.Fatalf("MACRO TPA80B: %s", out)
	}

	if out := dispatch("LINK/NOMAP TPA80"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	return strings.Split(dispatch("RUN TPA80"), "|")
}

// expectedTPA80Differences are the cases whose reports differ from VMS's, by
// their first word, with why.
var expectedTPA80Differences = map[string]string{
	// A keyword index past the vector: the parser reads whatever is there, so
	// it depends on what follows the vector in the image. On VMS that was a
	// keyword that didn't match, and here an offset to an unmapped address.
	"RAW219": "the keyword vector's neighbors",
}

// withoutExpected removes the cases expectedTPA80Differences names.
func withoutExpected(lines []string) []string {
	var out []string

	skip := false

	for _, line := range lines {
		if !strings.HasPrefix(line, " ") {
			first, _, _ := strings.Cut(line, " ")
			_, skip = expectedTPA80Differences[first]
		}

		if !skip {
			out = append(out, line)
		}
	}

	return out
}

// TestTPA80Oracle holds the probe's output to VMS's.
func TestTPA80Oracle(t *testing.T) {
	got := runTPA80(t)

	if path := os.Getenv("TPA80_OUT"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(got, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(strings.Join(got, "\n"), "##### the end of the cases") {
		t.Fatalf("the program didn't reach its end: %d lines, last %q", len(got), got[len(got)-1])
	}

	want, err := os.ReadFile(filepath.Join(tpa80Dir, "vax", "tpa80-vax.txt"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("no VMS run yet (vax/tpa80-vax.txt)")
	}

	if err != nil {
		t.Fatal(err)
	}

	// The console's output has no blank lines, so neither does what's compared.
	var lines []string

	for line := range strings.SplitSeq(strings.ReplaceAll(string(want), "\r", ""), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}

	got, lines = withoutExpected(got), withoutExpected(lines)

	for i := range max(len(got), len(lines)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(lines) {
			w = lines[i]
		}

		if g != w {
			t.Errorf("line %d:\n vms   %q\n govax %q", i+1, w, g)
		}
	}
}

// tpa80Cases are what the manual's words and examples say of some cases: the
// case's line and text its report must hold. They're checked whether or not
// VMS's run is here.
var tpa80Cases = []struct{ name, want string }{
	{"CHAR <A>", "R0=00000001"},
	{"CHAR <B>", "R0=00158284"},
	{"CHAR < A>", "R0=00000001"},
	{"KW <ABCD>", "R0=00158284"},
	{"KW <ABC+>", `rest=1@3 "+"`},
	{"KW1 <AB>", "R0=00158284"},
	{"DECIMAL <4294967295>", "num=FFFFFFFF"},
	{"DECIMAL <4294967296>", "R0=00158284"},
	{"UIC <[*,2]>", "num=3FFF0002"},
	{"FILESPEC <DEV:[DIR]A.B;1 X>", `tok=14@0 "DEV:[DIR]A.B;1"`},
	{"FAILA <ABC D>", `tok=3@0 "ABC"`},
	{"REJECT <A>", "param=00000001"},
	{"ALT <A>", "R0=00001230"},
	{"ODD <A>", "R0=00000001"},
	{"SIG <A>", "signal=00001234"},
	{"MASK <A>", "cell=5555555F"},
	{"QUOTE </ABC/> opt 1", "cell=00000003 desc @1"},
	{"LOOK <10/OCTAL>", "R0=00000001"},
	{"LOOK <99/OCTAL>", "R0=00158284"},
	{"LOOK <10/HEX>", "cell=00000006 desc @0"},
	{"LOOK <ZZ/HEX>", "R0=00158284"},
	{"SUBNEST <((A))>", "cell=00000005 desc @0"},
	{"SUBNEST <((A)>", "R0=00158284"},
	{"ABR <BET> opt 2", "opt=00010002"},
	{"ABR <CAT>", "R0=00000001"},
	{"ABR <C>", `tok=0@1 ""`},
	{"ABR <A> opt 4", "rest=0@1"},
	{"ABR <BE> opt 2000000", "rest=0@2"},
	{"ABR2 <GET> opt 2", "R0=00000001"},
	{"RAW250 <12>", "R0=0015828C"},
	{"count 7", "R0=00000114"},
}

// TestTPA80Manual holds the cases to the manuals, with or without VMS's run.
func TestTPA80Manual(t *testing.T) {
	blocks := map[string]string{}

	var name string

	for _, line := range runTPA80(t) {
		if !strings.HasPrefix(line, " ") {
			name = line
		}

		// The first report of a name is LIB$TPARSE's.
		if !strings.HasPrefix(line, " ") && blocks[name] != "" {
			name = ""
		}

		if name != "" {
			blocks[name] += line + "\n"
		}
	}

	for _, c := range tpa80Cases {
		if !strings.Contains(blocks[c.name], c.want) {
			t.Errorf("%s: no %q in:\n%s", c.name, c.want, blocks[c.name])
		}
	}
}
