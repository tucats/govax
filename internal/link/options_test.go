package link

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseOptions(t *testing.T) {
	o, err := ParseOptions([]string{
		"! A comment line",
		"MAIN, SUB    ! two objects",
		"MYLIB/LIBRARY/INCLUDE=(ONE, two), -",
		"  OTHER/LIB/INCL=THREE",
		`SYS$LIBRARY:LIBRTL/SHAREABLE, "odd,name.obj"/SELECTIVE`,
		"STACK=%X20",
		"ident = V2.0",
		`NAME="PROG"`,
		"SYMBOL=LIMIT,100",
		"symb=MASK, %O17",
		"",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := &OptionsFile{
		Files: []InputFile{
			{Name: "MAIN"},
			{Name: "SUB"},
			{Name: "MYLIB", Library: true, Include: []string{"ONE", "TWO"}},
			{Name: "OTHER", Library: true, Include: []string{"THREE"}},
			{Name: "SYS$LIBRARY:LIBRTL", Shareable: true},
			{Name: "odd,name.obj", Selective: true},
		},
		Stack:   32,
		Ident:   "V2.0",
		Name:    "PROG",
		Symbols: []Symbol{{"LIMIT", 100}, {"MASK", 15}},
	}

	if !reflect.DeepEqual(o, want) {
		t.Errorf("got  %+v\nwant %+v", o, want)
	}

	for _, bad := range []string{
		"CLUSTER=A,,,B",           // not supported
		"S=1",                     // ambiguous: STACK, SYMBOL
		"STACK=lots",              // not a number
		"SYMBOL=X",                // no value
		"IDENTIFICATION=SIXTEEN_CHARS_XX", // too long
		"A/NOSUCH",
		"A/INCLUDE",
		"A/LIBRARY=X",
		"A/SHAREABLE=COPY",
		"/LIBRARY",
	} {
		if _, err := ParseOptions([]string{bad}); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// TestLinkOptionsSymbolsAndIdent links with SYMBOL= definitions, one of
// which a module also defines, and IDENTIFICATION=: the options' values
// win, and the map lists the symbols.
func TestLinkOptionsSymbolsAndIdent(t *testing.T) {
	m := macroModule(t, ".TITLE T\n.IDENT /V1/\nLIMIT == 5\n.PSECT CODE,NOWRT,EXE\n.ENTRY START,^M<>\nMOVL #LIMIT,R0\nADDL2 #EXTRA,R0\nRET\n.END START")

	img, err := Link([]Input{{File: "t.obj", Module: m}}, Options{
		ImageName: "T",
		Ident:     "V2.0",
		Symbols:   []Symbol{{"LIMIT", 100}, {"EXTRA", 7}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if g := img.l.symbols["LIMIT"]; g.value != 100 {
		t.Errorf("LIMIT = %d", g.value)
	}

	if img.l.imageID != "V2.0" {
		t.Errorf("ident %q", img.l.imageID)
	}

	text := strings.Join(img.Map(MapOptions{ImageFile: "T.EXE", MapFile: "T.MAP"}), "\n")
	for _, want := range []string{"EXTRA           00000007", "LIMIT           00000064", "T V2.0\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the map:\n%s", want, text)
		}
	}
}
