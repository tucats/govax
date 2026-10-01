package rms

import (
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
		{"DUA1:[TEST]A.DAT;3", fileName{"DUA1:", "[TEST]", "A", ".DAT", ";3"}, 0},
		{"a.dat", fileName{"", "", "A", ".DAT", ""}, 0},
		{"<TEST.SUB>X", fileName{"", "[TEST.SUB]", "X", "", ""}, 0},
		{"A.B.3", fileName{"", "", "A", ".B", ";3"}, 0},
		{"A.", fileName{"", "", "A", ".", ""}, 0},
		{"A;", fileName{"", "", "A", "", ";"}, 0},
		{"[TEST...]*.*;*", fileName{"", "[TEST...]", "*", ".*", ";*"}, 0},
		{"_DUA1:X", fileName{"_DUA1:", "", "X", "", ""}, 0},
		{"A.B.C", fileName{}, rmsInvalidVersion},
		{"A;32768", fileName{}, rmsInvalidVersion},
		{"[TEST", fileName{}, rmsDirError},
		{"[A..B]", fileName{}, rmsDirError},
		{"NODE::X", fileName{}, rmsSyntaxError},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMN.DAT", fileName{}, rmsFileNameError},
		{"A.ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMN", fileName{}, rmsTypeError},
		{"A&B", fileName{}, rmsFileNameError},
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
		{"000000.TEST", "", "[TEST]"},
		{"TEST...", "", "[TEST...]"},
		{".SUB", "TEST", "[TEST.SUB]"},
		{"-", "TEST.SUB", "[TEST]"},
		{"-.X", "TEST.SUB", "[TEST.X]"},
		{"--", "TEST.SUB", "[000000]"},
		{"", "TEST", "[TEST]"},
		{"...", "TEST", "[TEST...]"},
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
		{nameInputs{Primary: "[.SUB]C", Default: "[TEST].DAT"}, "DUA1:[TEST.SUB]C.DAT;", fnbExpDir | fnbExpName | 1<<fnbDirLvls},
		{nameInputs{Primary: "TST:A.DAT"}, "DUA1:[TEST]A.DAT;", fnbExpDev | fnbExpDir | fnbExpName | fnbExpType},
		{nameInputs{Primary: "TSL:C.DAT"}, "DUA1:[TEST.SUB]C.DAT;", fnbExpDev | fnbExpDir | fnbExpName | fnbExpType | fnbSearchList | 1<<fnbDirLvls},
		{nameInputs{Primary: "X", Related: "DUA2:[R]B.TXT;1"}, "DUA2:[R]X.TXT;", fnbExpName},
		{nameInputs{Primary: "X", Related: "DUA2:[R]B.TXT;1", OFP: true}, "DUA1:[R]X.TXT;", fnbExpName},
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
