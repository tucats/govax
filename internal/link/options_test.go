package link

import (
	"errors"
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
		// The shareable image's cluster, named once LINK reads it.
		Clusters: []Cluster{{Shareable: true}},
	}

	if !reflect.DeepEqual(o, want) {
		t.Errorf("got  %+v\nwant %+v", o, want)
	}

	if o.NameImage("LIBRTL"); o.Clusters[0].Image != "LIBRTL" {
		t.Errorf("the image's cluster is %+v", o.Clusters[0])
	}

	for _, bad := range []string{
		"CLUSTER=,,,B",            // no cluster name
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

// TestParseLayoutOptions reads the probe's layout options
// (testdata/link68's SHR4, SHR5, and EXE1) and the rest of VAX LINK's.
func TestParseLayoutOptions(t *testing.T) {
	o, err := ParseOptions([]string{
		"PROTECT=YES",
		"CLUSTER=FIRST,,,XFR1.OBJ",
		"COLLECT=FIRST,SHR1_RO",
		"PROTECT=NO",
		"SHR1.OBJ",
		"CLUSTER=SECOND,%X40000,8,EXE1B.OBJ,LIB/LIBRARY",
		"PSECT_ATTR=SHR1_RO,NOSHR",
		"PSECT_ATTRIBUTE=EXE1_DATA,nowrt,PAGE,LCL",
		"PSECT_ATTR=X,3",
		"BASE=%X10000",
		"DZRO_MIN=1",
		"ISD_MAX=20",
		"IOSEGMENT=40",
		"GSMATCH=ALWAYS,0,0",
		"GSM=LEQ,1,%X3E8 ! the last one wins",
		"UNIVERSAL=SHR1_COUNT,shr1_table, -",
		"  SHR1_ORIGIN",
		"UNIV=SHR1_BASE",
		"UNSUPPORTED=1",
	})
	if err != nil {
		t.Fatal(err)
	}

	u32 := func(n uint32) *uint32 { return &n }

	want := &OptionsFile{
		Files: []InputFile{
			{Name: "XFR1.OBJ", Cluster: "FIRST"},
			{Name: "SHR1.OBJ"},
			{Name: "EXE1B.OBJ", Cluster: "SECOND"},
			{Name: "LIB", Library: true, Cluster: "SECOND"},
		},
		Universals: []string{"SHR1_COUNT", "SHR1_TABLE", "SHR1_ORIGIN", "SHR1_BASE"},
		GSMatch:    &GSMatch{Match: MatchLEQ, Major: 1, Minor: 1000},
		Clusters: []Cluster{
			{Name: "FIRST", Protect: true},
			{Name: "SECOND", Base: u32(0x40000), PFC: 8},
		},
		Collects: []Collect{{Cluster: "FIRST", Psects: []string{"SHR1_RO"}, Protect: true}},
		PsectAttrs: []PsectAttr{
			{Name: "SHR1_RO", Attributes: []string{"NOSHR"}},
			{Name: "EXE1_DATA", Attributes: []string{"NOWRT", "LCL"}, Align: u32(9)},
			{Name: "X", Align: u32(3)},
		},
		Base:        u32(0x10000),
		DZROMin:     u32(1),
		ISDMax:      u32(20),
		IOSegment:   &IOSegment{Pages: 40, P0Bufs: true},
		Unsupported: u32(1),
	}

	if !reflect.DeepEqual(o, want) {
		t.Errorf("got  %+v\nwant %+v", o, want)
	}

	// Only UNSUPPORTED=, which neither manual describes, is refused.
	if got := o.Unimplemented(); !reflect.DeepEqual(got, []string{"UNSUPPORTED="}) {
		t.Errorf("unimplemented %v", got)
	}

	var plain OptionsFile
	if got := plain.Unimplemented(); len(got) != 0 {
		t.Errorf("unimplemented %v", got)
	}
}

// TestOptionErrors checks the messages of lines in error: the probe's
// (testdata/link68's log), word for word, and govax's others.
func TestOptionErrors(t *testing.T) {
	for _, c := range []struct {
		line, file string
		want       string
		status     uint32
	}{
		{"GSMATCH=NEVER,1,1000", "DUA1:[000000]GSM.OPT;7",
			"LINK-F-OPTSYNERR, syntax error in options file DUA1:[000000]GSM.OPT;7\n-LINK-E-OPTLIN, options line in error\n\tGSMATCH='NEVER',1,1000", 0x006483B4},
		{"GSMATCH=SOMETIMES,1,2", "SYS$INPUT:.;",
			"LINK-F-OPTSYNERR, syntax error in options file SYS$INPUT:.;\n-LINK-E-OPTLIN, options line in error\n\tGSMATCH='SOMETIMES',1,2", 0x006483B4},
		{"GSMATCH=LEQUAL,300,1", "SYS$INPUT:.;",
			"LINK-F-OPTVALERR, GSMATCH option value 300. is out of range (0. to 255.)\n\tin options file SYS$INPUT:.;\n-LINK-E-OPTLIN, options line in error\n\tGSMATCH=LEQUAL,'300',1", 0x006483BC},
		{"UNIVERSALS=SHR1_COUNT", "SYS$INPUT:.;",
			"LINK-F-OPTSYNERR, syntax error in options file SYS$INPUT:.;\n-LINK-E-OPTLIN, options line in error\n\t'UNIVERSALS'=SHR1_COUNT", 0x006483B4},
	} {
		_, err := ParseOptions([]string{"SHR1.OBJ", c.line})

		var oe *OptionError
		if !errors.As(err, &oe) {
			t.Errorf("%s: err %v", c.line, err)

			continue
		}

		if got := oe.Message(c.file); got != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.line, got, c.want)
		}

		if oe.Status() != c.status {
			t.Errorf("%s: status %08X", c.line, oe.Status())
		}
	}

	// govax's own checks: the quoted part of each line.
	for line, quoted := range map[string]string{
		"GSMATCH=LEQUAL,1":           "GSMATCH=LEQUAL,'1'",
		"GSMATCH=LEQUAL,1,2,3":       "GSMATCH=LEQUAL,1,2,'3'",
		"GSMATCH=LEQUAL,1,%X1000000": "GSMATCH=LEQUAL,1,'%X1000000'",
		"STACK = lots":               "STACK = 'lots'",
		"STACK=0":                    "STACK='0'",
		"S=1":                        "'S'=1",
		"U=X":                        "'U'=X",
		"CLUSTER=,,,B":               "CLUSTER='',,,B",
		"CLUSTER=C,,200":             "CLUSTER=C,,'200'",
		"CLUSTER=C,,,B/NOSUCH":       "CLUSTER=C,,,'B/NOSUCH'",
		"COLLECT=C":                  "COLLECT='C'",
		"PSECT_ATTR=X,SHINY":         "PSECT_ATTR=X,'SHINY'",
		"PSECT_ATTR=X,10":            "PSECT_ATTR=X,'10'",
		"PROTECT=MAYBE":              "PROTECT='MAYBE'",
		"IOSEGMENT=40,SOME":          "IOSEGMENT=40,'SOME'",
		"UNIVERSAL=A,,B":             "UNIVERSAL=A,'',B",
		"A, B/NOSUCH":                "A, 'B/NOSUCH'",
	} {
		_, err := ParseOptions([]string{line})

		var oe *OptionError
		if !errors.As(err, &oe) {
			t.Errorf("%s: err %v", line, err)
		} else if oe.Line != quoted {
			t.Errorf("%s: line %q, want %q", line, oe.Line, quoted)
		}
	}

	// /SHAREABLE=COPY is VMS's, but not govax's: not a syntax error.
	_, err := ParseOptions([]string{"X/SHAREABLE=COPY"})

	var oe *OptionError
	if err == nil || errors.As(err, &oe) {
		t.Errorf("/SHAREABLE=COPY: %v", err)
	}
}

// TestOptionsParserStops reads a file a line at a time, as LINK does:
// the error comes with the line in error, and a continued statement is
// read when its last line comes.
func TestOptionsParserStops(t *testing.T) {
	var p OptionsParser

	for _, line := range []string{"SHR1.OBJ", "UNIVERSAL=A, -", "  B"} {
		if err := p.Line(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if err := p.Line("GSMATCH=NEVER,1,1"); err == nil {
		t.Error("no error for NEVER")
	}

	if err := p.Line("UNIVERSAL=C -"); err != nil {
		t.Fatal(err)
	}

	// The file ends in the middle of a statement: End reads it.
	o, err := p.End()
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"A", "B", "C"}; !reflect.DeepEqual(o.Universals, want) {
		t.Errorf("universals %v", o.Universals)
	}
}

// TestMergeOptions takes a second options file's options: GSMATCH= and
// the other values replace, lists add up.
func TestMergeOptions(t *testing.T) {
	first, err := ParseOptions([]string{"UNIVERSAL=A", "GSMATCH=LEQUAL,1,1", "STACK=10", "BASE=%X200"})
	if err != nil {
		t.Fatal(err)
	}

	second, err := ParseOptions([]string{"UNIVERSAL=B", "GSMATCH=EQUAL,2,5"})
	if err != nil {
		t.Fatal(err)
	}

	first.Merge(second)

	if !reflect.DeepEqual(first.Universals, []string{"A", "B"}) || *first.GSMatch != (GSMatch{MatchEqual, 2, 5}) ||
		first.Stack != 10 || *first.Base != 0x200 {
		t.Errorf("merged %+v", first)
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
