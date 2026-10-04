package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// forthSession assembles, links, and runs testdata/mar/forth.mar, a FORTH
// interpreter, with typed as its terminal input and a fresh volume on
// DUA0: (the default directory) holding files, each of whose lines is a
// record. It returns what the program wrote to the terminal, and the
// console, for looking at the volume afterward.
func forthSession(t *testing.T, typed string, files map[string]string) (string, *Console) {
	t.Helper()

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	c.In = strings.NewReader(typed)

	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	for name, text := range files {
		loc := rms.FileLocation{Name: "DUA0:[000000]" + name}
		if _, err := c.ContainerSession.CreateRecordFile(loc, rms.TextRecords, splitSource(text)); err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
	}

	dir := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "mar", "forth.mar")

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, "forth.obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "forth")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if r0 := runImageBounded(t, c, filepath.Join(dir, "forth.exe"), 20_000_000); r0 != 1 {
		t.Errorf("R0 = %#x, want SS$_NORMAL", r0)
	}

	// What the kernel printed as it booted comes first.
	_, session, _ := strings.Cut(out.String(), "Vforth")

	return "Vforth" + session, c
}

// TestForth_interpreter runs the interpreter's words: arithmetic, stack
// words, definitions with each control structure, variables, constants,
// strings, floating point, and number bases, and the errors that abort a
// line.
func TestForth_interpreter(t *testing.T) {
	typed := strings.Join([]string{
		"2 3 + .",
		"1 2 3 rot . . . cr",
		": sq dup * ; 7 sq .",
		": count 5 0 do i . loop ; count",
		"10 3 mod . -7 2 mod . 5 3 and . 5 3 or .",
		"hex ff . decimal 255 .",
		`." hello world" cr`,
		`: greet ." hi there" cr ; greet`,
		"1.5 2.25 f+ f. 90.0 fsin f. -2.5 f. 30 sin .",
		"variable x 42 x ! x @ .",
		"100 constant c c .",
		`: t 5 > if ." big" else ." small" endif cr ; 9 t 2 t`,
		": cd begin dup . 1 - dup 0 = until drop ; 3 cd cr",
		": odd 1 begin dup 10 < while dup . 2 + repeat drop ; odd cr",
		"foo",
		"5 3 and . 12x",
		": bad if ;",
		"drop",
		"halt",
	}, "\n") + "\n"

	got, _ := forthSession(t, typed, nil)

	want := strings.Join([]string{
		`Vforth V2.0 -- "halt" or Ctrl/Z to exit`,
		"> 5 > 1 3 2 ",
		"> 49 > 0 1 2 3 4 > 1 -1 1 7 > ff 255 > hello world",
		"> hi there",
		"> 3.750000 1.000000 -2.500000 5000 > 42 > 100 > big",
		"small",
		"> 3 2 1 ",
		"> 1 3 5 7 9 ",
		"> foo: not found",
		"> 1 12x: not found",
		">  ';' not matched to ':'",
		">  ?Stack empty",
		"> ",
	}, "\n")

	if got != want {
		t.Errorf("session:\n%s\nwant:\n%s", got, want)
	}
}

// TestForth_files reads definitions from nested input files, which resume
// what they interrupted, writes output to a file, and exits at the end of
// the terminal's input.
func TestForth_files(t *testing.T) {
	files := map[string]string{
		"LIB.FTH":   ": sq dup * ;  ( a comment, which ends at the line's end\n.\" loaded lib\" cr\n",
		"OUTER.FTH": "input inner\n.\" after inner\" cr\n",
		"INNER.FTH": ".\" in inner\" cr 7 .\n",
	}

	typed := strings.Join([]string{
		"input lib 4 sq . cr",
		`output out 5 . cr ." to file" cr outpop`,
		`." back" cr`,
		`input outer ." done" cr`,
		"input nosuch",
	}, "\n") + "\n"

	got, c := forthSession(t, typed, files)

	want := strings.Join([]string{
		`Vforth V2.0 -- "halt" or Ctrl/Z to exit`,
		"> loaded lib",
		"16 ",
		"> > back",
		"> in inner",
		"7 after inner",
		"done",
		">  Could not open input file",
		"> ",
	}, "\n")

	if got != want {
		t.Errorf("session:\n%s\nwant:\n%s", got, want)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]OUT.LIS"}, rms.TextRecords)
	if err != nil {
		t.Fatalf("reading OUT.LIS: %v", err)
	}

	if got, want := string(bytes.Join(records, []byte("|"))), "5 |to file"; got != want {
		t.Errorf("OUT.LIS = %q, want %q", got, want)
	}
}
