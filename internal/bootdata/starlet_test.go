package bootdata

import (
	"bytes"
	"io/fs"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/lbr"
)

// TestStarletMatchesSource: the committed STARLET.MLB is what its source
// builds, so the two can't drift apart. If this fails, run
// "go generate ./internal/bootdata".
func TestStarletMatchesSource(t *testing.T) {
	src, err := StarletSources(FS)
	if err != nil {
		t.Fatal(err)
	}

	want, err := fs.ReadFile(FS, StarletLibrary)
	if err != nil {
		t.Fatal(err)
	}

	got, err := BuildStarlet(src)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("%s doesn't match %s: run go generate ./internal/bootdata", StarletLibrary, StarletSource)
	}
}

// TestStarletModules: the library reads back as a macro library holding
// the macros its source defines, each module's text from .MACRO to .ENDM,
// squeezed of comments.
func TestStarletModules(t *testing.T) {
	data, err := fs.ReadFile(FS, StarletLibrary)
	if err != nil {
		t.Fatal(err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	if l.Type != lbr.TypeMacro {
		t.Fatalf("type %s, want macro", l.Type)
	}

	var names []string
	for _, k := range l.Indexes[0].Keys {
		names = append(names, k.Name)
	}

	for _, want := range []string{"$EXIT_S", "$ASSIGN_S", "$DASSGN_S", "$QIO_S", "$QIOW_S", "$PUSHADR"} {
		if !slices.Contains(names, want) {
			t.Errorf("no module %s in %v", want, names)
		}
	}

	rfa, _ := l.Lookup("$EXIT_S")

	m, err := l.Module(rfa)
	if err != nil {
		t.Fatal(err)
	}

	var lines []string
	for _, r := range m.Records {
		lines = append(lines, string(r))
	}

	want := []string{
		"\t.MACRO\t$EXIT_S\tCODE=#1",
		"\t.GLOBL\tSYS$EXIT",
		"\tPUSHL\tCODE",
		"\tCALLS\t#1,G^SYS$EXIT",
		"\t.ENDM\t$EXIT_S",
	}
	if !slices.Equal(lines, want) {
		t.Errorf("$EXIT_S is %q, want %q", lines, want)
	}
}

// TestBuildStarletErrors: a source the librarian warns about, or can't
// read, is refused.
func TestBuildStarletErrors(t *testing.T) {
	for _, src := range []string{
		"; no macros at all\n",
		"\t.MACRO\tA\n\t.BYTE\t1\n",                    // no .ENDM
		"\t.MACRO\tA\n\t.ENDM\tB\n",                    // .ENDM names another macro
		"\t.MACRO\tA\n\t.ENDM\n\t.MACRO\tA\n\t.ENDM\n", // defined twice
	} {
		if _, err := BuildStarlet(src); err == nil {
			t.Errorf("BuildStarlet(%q) succeeded, want an error", src)
		}
	}
}
