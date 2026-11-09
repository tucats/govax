package rms

import (
	"slices"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/ods2/filespec"
)

// TestScanName: each field of a specification, and the status for each
// kind of field that isn't well formed.
func TestScanName(t *testing.T) {
	for _, tc := range []struct {
		text string
		want fileName
		sts  uint32
	}{
		{"DUA1:[TEST]A.DAT;3", fileName{"DUA1:", "[TEST]", "A", ".DAT", ";3", ""}, 0},
		{"a.dat", fileName{"", "", "A", ".DAT", "", ""}, 0},
		{"<TEST.SUB>X", fileName{"", "<TEST.SUB>", "X", "", "", ""}, 0},
		{"A.B.3", fileName{"", "", "A", ".B", ";3", ""}, 0},
		{"A.", fileName{"", "", "A", ".", "", ""}, 0},
		{"A;", fileName{"", "", "A", "", ";", ""}, 0},
		{"[TEST...]*.*;*", fileName{"", "[TEST...]", "*", ".*", ";*", ""}, 0},
		{"_DUA1:X", fileName{"_DUA1:", "", "X", "", "", ""}, 0},
		{"A.B.C", fileName{}, rmsSyntaxError},
		{"A;C", fileName{}, rmsSyntaxError}, // VMS 7.1: testdata/lib58
		{"A;32768", fileName{"", "", "A", "", ";32768", ""}, 0},
		{"A;65536", fileName{}, rmsInvalidVersion},
		{"[TEST", fileName{}, rmsSyntaxError}, // VMS 7.1: testdata/lib58
		{"[A..B]", fileName{}, rmsDirError},
		{"NODE::X", fileName{}, rmsSyntaxError},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMN.DAT", fileName{}, rmsFileNameError},
		{"A.ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMN", fileName{}, rmsTypeError},
		{"A&B", fileName{}, rmsFileNameError},

		// Roots, as VMS 7.1 has them (testdata/lnm91).
		{"DUA1:[R1.R2.][SUB]A", fileName{Dev: "DUA1:", Root: "[R1.R2.]", Dir: "[SUB]", Name: "A"}, 0},
		{"[R1.]A", fileName{Root: "[R1.]", Name: "A"}, 0},
		{"<R1.>[SUB]", fileName{Root: "<R1.>", Dir: "[SUB]"}, 0},
		{"[R1.]<SUB>", fileName{Root: "[R1.]", Dir: "<SUB>"}, 0},
		{"[000000.][SUB]A", fileName{Root: "[000000.]", Dir: "[SUB]", Name: "A"}, 0},
		{"[.A.][B]", fileName{Root: "[.A.]", Dir: "[B]"}, 0},
		{"[.]", fileName{Root: "[.]"}, 0},
		{"[A...]", fileName{Dir: "[A...]"}, 0},
		{"[A.][B.]", fileName{}, rmsSyntaxError},
		{"[A...][B]", fileName{}, rmsSyntaxError},
		{"[A..B.]", fileName{}, rmsSyntaxError},
		{"[R*.]", fileName{}, rmsDirError},
		{"[-.A.][B]", fileName{}, rmsDirError},
		{"[A.B.C.D.E.F.G.H.]", fileName{}, rmsDeviceError},
		{"[A.B.C.D.E.F.G.]", fileName{Root: "[A.B.C.D.E.F.G.]"}, 0},
	} {
		got, sts := scanName(tc.text)
		if sts != tc.sts {
			t.Errorf("scanName(%q) status = %#x, want %#x", tc.text, sts, tc.sts)

			continue
		}

		if sts == 0 && got != tc.want {
			t.Errorf("scanName(%q) = %+v, want %+v", tc.text, got, tc.want)
		}
	}
}

// TestDirSpec: directories' elements, relative directories applied, and
// how each is written back.
func TestDirSpec(t *testing.T) {
	for _, tc := range []struct{ body, base, want string }{
		{"TEST", "", "[TEST]"},
		{"000000", "", "[000000]"},
		{"000000.TEST", "", "[000000.TEST]"},
		{".SUB", "", "[000000.SUB]"},
		{"TEST...", "", "[TEST...]"},
		{".SUB", "TEST", "[TEST.SUB]"},
		{"-", "TEST.SUB", "[TEST]"},
		{"-.X", "TEST.SUB", "[TEST.X]"},
		{"--", "TEST.SUB", "[000000]"},
		{"", "TEST", "[TEST]"},
		{"...", "TEST", "[TEST...]"},
		{"SUB.-", "", "[000000]"},  // VMS 7.1: testdata/lnm91
		{".K.-", "JONES", "[JONES]"},
	} {
		d, ok := scanDir(tc.body)
		if !ok {
			t.Errorf("scanDir(%q) failed", tc.body)

			continue
		}

		if d.Relative {
			base, _ := scanDir(tc.base)
			if d, ok = applyDir(d, base); !ok {
				t.Errorf("applyDir(%q, %q) failed", tc.body, tc.base)

				continue
			}
		}

		if got := d.String(); got != tc.want {
			t.Errorf("[%s] on [%s] = %s, want %s", tc.body, tc.base, got, tc.want)
		}
	}

	if _, ok := applyDir(dirSpec{Relative: true, Elems: []string{"-"}}, dirSpec{}); ok {
		t.Error("[-] on the MFD succeeded, want a failure")
	}
}

// TestExpandName: the sources of a name, in their order of precedence.
func TestExpandName(t *testing.T) {
	db := lnm.NewDatabase(0)

	for name, eqv := range map[string][]string{
		"SYS$DISK": {"DUA1:"},
		"TST":      {"DUA1:[TEST]"},
		"TSL":      {"DUA1:[TEST.SUB]", "DUA1:[TEST]"},
	} {
		var e []lnm.Equivalence
		for _, v := range eqv {
			e = append(e, lnm.Equivalence{Value: v})
		}

		if _, err := db.Define(lnm.ProcessTableName, name, lnm.Supervisor, 0, e); err != nil {
			t.Fatal(err)
		}
	}

	ctx := &Context{Logicals: db, Session: &Session{Default: filespec.Spec{Dirs: []string{"HOME"}}}}

	for _, tc := range []struct {
		in   nameInputs
		want string
		fnb  uint32
	}{
		{nameInputs{Primary: "X"}, "DUA1:[HOME]X.;", fnbExpName},
		{nameInputs{Primary: "A", Default: "[TEST].DAT"}, "DUA1:[TEST]A.DAT;", fnbExpName},
		// A relative directory applies to the process default, not the
		// default name's (VMS 7.3, the oracle's PARSE case 21).
		{nameInputs{Primary: "[.SUB]C", Default: "[TEST].DAT"}, "DUA1:[HOME.SUB]C.DAT;", fnbExpDir | fnbExpName | 1<<fnbDirLvls},
		{nameInputs{Primary: "TST:A.DAT"}, "DUA1:[TEST]A.DAT;", fnbExpDev | fnbExpDir | fnbExpName | fnbExpType},
		{nameInputs{Primary: "TSL:C.DAT"}, "DUA1:[TEST.SUB]C.DAT;", fnbExpDev | fnbExpDir | fnbExpName | fnbExpType | fnbSearchList | 1<<fnbDirLvls},
		{nameInputs{Primary: "X", Related: "DUA2:[R]B.TXT;1"}, "DUA2:[R]X.TXT;", fnbExpName},
		// With OFP, the related file gives only the name and type (PARSE
		// case 14).
		{nameInputs{Primary: "X", Related: "DUA2:[R]B.TXT;1", OFP: true}, "DUA1:[HOME]X.TXT;", fnbExpName},
		{nameInputs{Primary: "[TEST]*.DAT;*"}, "DUA1:[TEST]*.DAT;*",
			fnbExpDir | fnbExpName | fnbExpType | fnbExpVer | fnbWildName | fnbWildVer | fnbWildcard},
		{nameInputs{Primary: "[A.*.C]X"}, "DUA1:[A.*.C]X.;",
			fnbExpDir | fnbExpName | fnbWildDir | fnbWildSFD1 | fnbWildcard | 2<<fnbDirLvls},
	} {
		names, sts := ctx.expandName(tc.in)
		if sts != 0 {
			t.Errorf("expandName(%+v) status %#x", tc.in, sts)

			continue
		}

		if got := names[0].String(); got != tc.want {
			t.Errorf("expandName(%+v) = %s, want %s", tc.in, got, tc.want)
		}

		if names[0].FNB != tc.fnb {
			t.Errorf("expandName(%+v) FNB = %#x, want %#x", tc.in, names[0].FNB, tc.fnb)
		}
	}

	names, _ := ctx.expandName(nameInputs{Primary: "TSL:*.DAT"})
	if len(names) != 2 || names[1].String() != "DUA1:[TEST]*.DAT;" {
		t.Errorf("TSL:*.DAT expands to %v, want both elements", names)
	}
}

// rootedNames is a database with the kinds of name testdata/lnm91 probes.
func rootedNames(t *testing.T) *lnm.Database {
	t.Helper()

	db := lnm.NewDatabase(0)

	for _, d := range []struct {
		name, value string
		attr        uint32
	}{
		{"BASE", "_DUA1:[R1.R2.]", lnm.AttrConcealed | lnm.AttrTerminal},
		{"NOCON", "_DUA1:[R1.R2.]", 0},
		{"CDISK", "_DUA1:", lnm.AttrConcealed},
		{"A1", "BASE:[SUB]", 0},
		{"A2", "BASE:[SUB.]", 0},
		{"A4", "BASE:[SUB]", lnm.AttrConcealed},
		{"P", "_DUA1:[R1]", 0},
		{"N", "_DUA1:[R1]A.DAT", 0},
		{sysDiskName, "DUA1:", 0},
	} {
		eqv := []lnm.Equivalence{{Value: d.value, Attrs: d.attr}}
		if _, err := db.Define(lnm.ProcessTableName, d.name, lnm.Supervisor, 0, eqv); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

// TestRootedExpansion: what a root does to a name, the volume's directory
// beside what's shown; TestLnm91ParseOracle has every case VMS was asked.
func TestRootedExpansion(t *testing.T) {
	db := rootedNames(t)
	base := filespec.Spec{Dirs: []string{"JONES"}}

	for _, tc := range []struct {
		in   nameInputs
		want string
		dirs []string
		sts  uint32
	}{
		{nameInputs{Primary: "BASE:[SUB]A.DAT"}, "BASE:[SUB]A.DAT;", []string{"R1", "R2", "SUB"}, 0},
		{nameInputs{Primary: "BASE:[SUB]A.DAT", NoConceal: true}, "_DUA1:[R1.R2.][SUB]A.DAT;", []string{"R1", "R2", "SUB"}, 0},
		{nameInputs{Primary: "NOCON:[SUB]A"}, "_DUA1:[R1.R2.][SUB]A.;", []string{"R1", "R2", "SUB"}, 0},
		{nameInputs{Primary: "BASE:A"}, "BASE:[JONES]A.;", []string{"R1", "R2", "JONES"}, 0},
		{nameInputs{Primary: "BASE:[000000]A"}, "BASE:[000000]A.;", []string{"R1", "R2"}, 0},
		{nameInputs{Primary: "BASE:[-]A"}, "BASE:[000000]A.;", []string{"R1", "R2"}, 0},
		{nameInputs{Primary: "A1:A"}, "BASE:[SUB]A.;", []string{"R1", "R2", "SUB"}, 0},
		{nameInputs{Primary: "[R1.R2.][SUB]A"}, "DUA1:[R1.R2.][SUB]A.;", []string{"R1", "R2", "SUB"}, 0},
		{nameInputs{Primary: "CDISK:[R1]A"}, "CDISK:[R1]A.;", []string{"R1"}, 0},

		// The root goes with its device.
		{nameInputs{Primary: "A", Default: "BASE:[SUB]"}, "BASE:[SUB]A.;", []string{"R1", "R2", "SUB"}, 0},
		{nameInputs{Primary: "DUA1:A", Default: "BASE:[SUB]"}, "DUA1:[SUB]A.;", []string{"SUB"}, 0},
		{nameInputs{Primary: ".DAT", Related: "BASE:[SUB]X.DAT;1"}, "BASE:[SUB]X.DAT;", []string{"R1", "R2", "SUB"}, 0},

		// A bracketed default decides the brackets.
		{nameInputs{Primary: "DUA1:<SUB>A"}, "DUA1:<SUB>A.;", []string{"SUB"}, 0},
		{nameInputs{Primary: "DUA1:<SUB>A", Default: "[OTHER]"}, "DUA1:[SUB]A.;", []string{"SUB"}, 0},

		// A field on both sides of a translation.
		{nameInputs{Primary: "P:[SUB]A"}, "", nil, rmsDirError},
		{nameInputs{Primary: "A1:[OTHER]A"}, "", nil, rmsDirError},
		{nameInputs{Primary: "N:B.DAT"}, "", nil, rmsFileNameError},
		{nameInputs{Primary: "N:.TXT"}, "", nil, rmsTypeError},
		{nameInputs{Primary: "N:"}, "_DUA1:[R1]A.DAT;", []string{"R1"}, 0},
		{nameInputs{Primary: "N:;5"}, "_DUA1:[R1]A.DAT;5", []string{"R1"}, 0},

		// A concealed name's limits, and a second root.
		{nameInputs{Primary: "A4:A"}, "", nil, rmsDeviceError},
		{nameInputs{Primary: "A2:A"}, "", nil, rmsDeviceError},
		{nameInputs{Primary: "BASE:[A.]X"}, "", nil, rmsDeviceError},
		{nameInputs{Primary: "A", Default: "A4:"}, "", nil, rmsDeviceError},
		{nameInputs{Primary: "A", Related: "A4:X.DAT"}, "", nil, rmsDeviceError},
	} {
		names, sts := expandNames(db, base, tc.in)
		if sts != tc.sts {
			t.Errorf("%+v: status %#x, want %#x", tc.in, sts, tc.sts)

			continue
		}

		if sts != 0 {
			continue
		}

		if got := names[0].String(); got != tc.want {
			t.Errorf("%+v = %s, want %s", tc.in, got, tc.want)
		}

		if got := names[0].spec().Dirs; !slices.Equal(got, tc.dirs) {
			t.Errorf("%+v: directories %v, want %v", tc.in, got, tc.dirs)
		}
	}
}

// TestSearchListStopsAtBadElement: a search list is usable up to an
// element name processing refuses (VMS 7.1: testdata/lnm91's LIST3).
func TestSearchListStopsAtBadElement(t *testing.T) {
	db := rootedNames(t)

	eqv := []lnm.Equivalence{{Value: "NOCON:"}, {Value: "CDISK:[R1]"}}
	if _, err := db.Define(lnm.ProcessTableName, "LIST3", lnm.Supervisor, 0, eqv); err != nil {
		t.Fatal(err)
	}

	names, sts := expandNames(db, filespec.Spec{}, nameInputs{Primary: "LIST3:[SUB]A"})
	if sts != 0 || len(names) != 1 {
		t.Fatalf("names %v, status %#x; want one name", names, sts)
	}

	if got, want := names[0].String(), "_DUA1:[R1.R2.][SUB]A.;"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	if names[0].FNB&fnbSearchList == 0 {
		t.Error("SEARCH_LIST isn't set")
	}
}
