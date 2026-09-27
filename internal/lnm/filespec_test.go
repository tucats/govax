package lnm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// specs joins the Spec of each result with "|".
func specs(fs []FileSpec) string {
	var s []string
	for _, f := range fs {
		s = append(s, f.Spec)
	}

	return strings.Join(s, "|")
}

func mustTranslateFileSpec(t *testing.T, db *Database, spec string) []FileSpec {
	t.Helper()

	fs, err := db.TranslateFileSpec(spec, User)
	if err != nil {
		t.Fatalf("TranslateFileSpec(%q): %v", spec, err)
	}

	return fs
}

// TestTranslateFileSpec_manualExamples runs the file-spec examples from
// CLRM §2.2 and User's Manual §11.3-11.7, each against a fresh database
// holding its DEFINEs (all in the process table).
func TestTranslateFileSpec_manualExamples(t *testing.T) {
	tests := []struct {
		name    string
		defines [][]string // name, values...
		spec    string
		want    string
	}{
		{"REPORT is a whole file spec",
			[][]string{{"REPORT", "DBA1:WEATHER.SUM"}},
			"REPORT", "DBA1:WEATHER.SUM"},
		{"GO translates through TEST",
			[][]string{{"TEST", "DBA1:"}, {"GO", "TEST:"}},
			"GO:", "DBA1:"},
		{"GO with a file name",
			[][]string{{"TEST", "DBA1:"}, {"GO", "TEST:"}},
			"GO:AVERAGE.OBJ", "DBA1:AVERAGE.OBJ"},
		{"PAY is a whole file spec",
			[][]string{{"PAY", "DISK1:[SALES_STAFF]PAYROLL.DAT"}},
			"PAY", "DISK1:[SALES_STAFF]PAYROLL.DAT"},
		{"PAY_DIR is a device and directory",
			[][]string{{"PAY_DIR", "DISK1:[SALES_STAFF]"}},
			"PAY_DIR:PAYROLL.DAT", "DISK1:[SALES_STAFF]PAYROLL.DAT"},
		{"PAY_DISK is a device",
			[][]string{{"PAY_DISK", "DISK1:"}},
			"PAY_DISK:[SALES_STAFF]PAYROLL.DAT", "DISK1:[SALES_STAFF]PAYROLL.DAT"},
		{"PAY_FILE is joined as text; field merging is the caller's",
			[][]string{{"PAY_FILE", "DISK1:[SALES_STAFF]PAYROLL"}},
			"PAY_FILE:*.DAT", "DISK1:[SALES_STAFF]PAYROLL*.DAT"},
		{"PUP ends the spec, so it is translated",
			[][]string{{"PUP", "DISK1:[DRYSDALE]PUP.TXT"}},
			"PUP", "DISK1:[DRYSDALE]PUP.TXT"},
		{"only DISK, the leftmost component, is translated",
			[][]string{{"PUP", "DISK1:[DRYSDALE]PUP.TXT"}, {"DISK", "DUA1:"}},
			"DISK:PUP", "DUA1:PUP"},
		{"a component ending in ] is not translated",
			[][]string{{"PUP", "DISK1:[DRYSDALE]PUP.TXT"}},
			"[DRYSDALE]PUP", "[DRYSDALE]PUP"},
		{"a component ending in . is not translated",
			[][]string{{"PUP", "DISK1:[DRYSDALE]PUP.TXT"}},
			"PUP.TXT", "PUP.TXT"},
		{"MEMO translates iteratively through DISK",
			[][]string{{"DISK", "DUA1:"}, {"MEMO", "DISK:[JEFF.MEMOS]COMPLAINT.TXT"}},
			"MEMO", "DUA1:[JEFF.MEMOS]COMPLAINT.TXT"},
		{"GETTYSBURG is a search list",
			[][]string{{"GETTYSBURG", "[JONES.HISTORY]", "[JONES.WORKFILES]"}},
			"GETTYSBURG:SPEECH.TXT", "[JONES.HISTORY]SPEECH.TXT|[JONES.WORKFILES]SPEECH.TXT"},
		{"FIFI is a search list of devices and directories",
			[][]string{{"FIFI", "DISK1:[FRED]", "DISK2:[GLADYS]", "DISK3:[MEATBALL.SUB]"}},
			"FIFI:MEMO.LIS", "DISK1:[FRED]MEMO.LIS|DISK2:[GLADYS]MEMO.LIS|DISK3:[MEATBALL.SUB]MEMO.LIS"},
		{"NESTED expands its sublist in place",
			[][]string{{"NESTED", "FRED.DAT", "NEW_LIST", "RICKY.DAT"}, {"NEW_LIST", "ETHEL.DAT", "LUCY.DAT"}},
			"NESTED", "FRED.DAT|ETHEL.DAT|LUCY.DAT|RICKY.DAT"},
		{"an undefined name is left alone",
			nil,
			"SPEECH.TXT", "SPEECH.TXT"},
		{"an undefined device is left alone",
			nil,
			"DBA1:[X]Y.Z", "DBA1:[X]Y.Z"},
		{"the empty spec is left alone",
			nil,
			"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewDatabase(testUIC)
			for _, d := range tt.defines {
				mustDefine(t, db, "LNM$PROCESS", d[0], Supervisor, d[1:]...)
			}

			if got := specs(mustTranslateFileSpec(t, db, tt.spec)); got != tt.want {
				t.Errorf("TranslateFileSpec(%q) = %q, want %q", tt.spec, got, tt.want)
			}
		})
	}
}

// TestTranslateFileSpec_restartsSearchOrder checks that each level of
// translation starts again at the process table, not in the table where
// the previous level was found.
func TestTranslateFileSpec_restartsSearchOrder(t *testing.T) {
	db := NewDatabase(testUIC)
	mustDefine(t, db, "LNM$PROCESS", "A", Supervisor, "B:")
	mustDefine(t, db, "LNM$SYSTEM", "B", Executive, "C:[B]")
	mustDefine(t, db, "LNM$SYSTEM", "C", Executive, "SYSTEM_C:")
	mustDefine(t, db, "LNM$PROCESS", "C", Supervisor, "PROCESS_C:")
	mustDefine(t, db, "LNM$GROUP", "PROCESS_C", Supervisor, "DUA3:")

	if got := specs(mustTranslateFileSpec(t, db, "A:X.DAT")); got != "DUA3:[B]X.DAT" {
		t.Errorf("got %q, want DUA3:[B]X.DAT", got)
	}
}

func TestTranslateFileSpec_underscoreAndNodes(t *testing.T) {
	db := NewDatabase(testUIC)
	mustDefine(t, db, "LNM$PROCESS", "DISK", Supervisor, "DUA1:")
	mustDefine(t, db, "LNM$PROCESS", "TTA0", Supervisor, "NOT_REACHED:")
	mustDefine(t, db, "LNM$PROCESS", "OUT", Supervisor, "_TTA0:")
	mustDefine(t, db, "LNM$PROCESS", "NODE", Supervisor, "OTHER")

	tests := []struct{ spec, want string }{
		{"_DISK:[X]Y.Z", "_DISK:[X]Y.Z"},
		{"_DISK", "_DISK"},
		{"OUT", "_TTA0:"},
		{"OUT:", "_TTA0:"},
		{"NODE::DISK:[X]Y.Z", "NODE::DISK:[X]Y.Z"},
		{"disk:[x]y.z", "DUA1:[x]y.z"},
		{":X", ":X"},
		{"DISK:", "DUA1:"},
	}

	for _, tt := range tests {
		if got := specs(mustTranslateFileSpec(t, db, tt.spec)); got != tt.want {
			t.Errorf("TranslateFileSpec(%q) = %q, want %q", tt.spec, got, tt.want)
		}
	}
}

func TestTranslateFileSpec_terminal(t *testing.T) {
	db := NewDatabase(testUIC)
	mustDefine(t, db, "LNM$PROCESS", "B", Supervisor, "DUA2:")

	eqv := []Equivalence{{Value: "B:", Attrs: AttrTerminal}, {Value: "B:[LIST]"}}
	if _, err := db.Define("LNM$PROCESS", "A", Supervisor, 0, eqv); err != nil {
		t.Fatal(err)
	}

	// TERMINAL is per equivalence string: the first stops at B:, the
	// second goes on through B.
	if got := specs(mustTranslateFileSpec(t, db, "A:X.DAT")); got != "B:X.DAT|DUA2:[LIST]X.DAT" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateFileSpec_concealed(t *testing.T) {
	db := NewDatabase(testUIC)

	conceal := func(name, value string, attrs uint32) {
		t.Helper()

		if _, err := db.Define("LNM$PROCESS", name, Supervisor, 0, []Equivalence{{Value: value, Attrs: attrs}}); err != nil {
			t.Fatal(err)
		}
	}

	conceal("DISK", "DJA3:", AttrConcealed)
	conceal("ROOT", "DISK:", AttrConcealed)
	conceal("TDISK", "DJA4:", AttrConcealed|AttrTerminal)
	mustDefine(t, db, "LNM$PROCESS", "WORK", Supervisor, "DISK:[SAM]")
	mustDefine(t, db, "LNM$PROCESS", "PLAIN", Supervisor, "DUA0:")

	tests := []struct{ spec, wantSpec, wantConcealed, wantDisplay string }{
		{"DISK:[SAM.PUP]X.DAT", "DJA3:[SAM.PUP]X.DAT", "DISK", "DISK:[SAM.PUP]X.DAT"},
		{"WORK:X.DAT", "DJA3:[SAM]X.DAT", "DISK", "DISK:[SAM]X.DAT"},
		{"ROOT:X.DAT", "DJA3:X.DAT", "ROOT", "ROOT:X.DAT"},
		{"TDISK:X.DAT", "DJA4:X.DAT", "TDISK", "TDISK:X.DAT"},
		{"PLAIN:X.DAT", "DUA0:X.DAT", "", "DUA0:X.DAT"},
	}

	for _, tt := range tests {
		fs := mustTranslateFileSpec(t, db, tt.spec)
		if len(fs) != 1 {
			t.Fatalf("TranslateFileSpec(%q) = %v, want one result", tt.spec, fs)
		}

		want := FileSpec{Spec: tt.wantSpec, Concealed: tt.wantConcealed, Display: tt.wantDisplay, Remainder: tt.spec[strings.IndexByte(tt.spec, ':')+1:]}
		if fs[0] != want {
			t.Errorf("TranslateFileSpec(%q) = %+v, want %+v", tt.spec, fs[0], want)
		}
	}
}

func TestTranslateFileSpec_depthLimit(t *testing.T) {
	// L0 -> L1: -> ... -> Ln-1: -> DUA0: takes n translations.
	chain := func(n int) *Database {
		db := NewDatabase(testUIC)
		for i := 0; i < n-1; i++ {
			mustDefine(t, db, "LNM$PROCESS", fmt.Sprintf("L%d", i), Supervisor, fmt.Sprintf("L%d:", i+1))
		}

		mustDefine(t, db, "LNM$PROCESS", fmt.Sprintf("L%d", n-1), Supervisor, "DUA0:")

		return db
	}

	if got := specs(mustTranslateFileSpec(t, chain(MaxDepth), "L0:X")); got != "DUA0:X" {
		t.Errorf("%d levels: got %q, want DUA0:X", MaxDepth, got)
	}

	_, err := chain(MaxDepth+1).TranslateFileSpec("L0:X", User)
	wantStatus(t, err, vmserrors.SS_TOOMANYLNAM)
}

func TestTranslateFileSpec_circular(t *testing.T) {
	db := NewDatabase(testUIC)
	mustDefine(t, db, "LNM$PROCESS", "A", Supervisor, "B:")
	mustDefine(t, db, "LNM$PROCESS", "B", Supervisor, "A:[X]")
	mustDefine(t, db, "LNM$PROCESS", "SELF", Supervisor, "SELF:[X]")

	for _, spec := range []string{"A:Y", "SELF:Y"} {
		_, err := db.TranslateFileSpec(spec, User)
		wantStatus(t, err, vmserrors.SS_TOOMANYLNAM)
	}

	// The same name in two elements of one search list isn't circular.
	mustDefine(t, db, "LNM$PROCESS", "DISK", Supervisor, "DUA0:")
	mustDefine(t, db, "LNM$PROCESS", "BOTH", Supervisor, "DISK:[A]", "DISK:[B]")

	if got := specs(mustTranslateFileSpec(t, db, "BOTH:Y")); got != "DUA0:[A]Y|DUA0:[B]Y" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateFileSpec_accessMode(t *testing.T) {
	db := NewDatabase(testUIC)
	mustDefine(t, db, "LNM$PROCESS", "DISK", User, "USER_DISK:")
	mustDefine(t, db, "LNM$PROCESS", "DISK", Supervisor, "SUPER_DISK:")

	fs, err := db.TranslateFileSpec("DISK:X", Supervisor)
	if err != nil {
		t.Fatal(err)
	}

	if got := specs(fs); got != "SUPER_DISK:X" {
		t.Errorf("at supervisor: got %q", got)
	}

	if got := specs(mustTranslateFileSpec(t, db, "DISK:X")); got != "USER_DISK:X" {
		t.Errorf("at user: got %q", got)
	}
}

func TestTranslateFileSpec_fileDevGone(t *testing.T) {
	db := NewDatabase(testUIC)
	if _, err := db.Delete(SystemDirectoryName, FileDevName, Executive); err != nil {
		t.Fatal(err)
	}

	_, err := db.TranslateFileSpec("DISK:X", User)
	wantStatus(t, err, vmserrors.SS_NOLOGTAB)

	// A spec with no candidate never looks anything up.
	if got := specs(mustTranslateFileSpec(t, db, "[X]Y")); got != "[X]Y" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateFileSpec_remainder(t *testing.T) {
	db := NewDatabase(testUIC)
	mustDefine(t, db, "LNM$PROCESS", "DISK", Supervisor, "DUA1:")
	mustDefine(t, db, "LNM$PROCESS", "MEMO", Supervisor, "DISK:[JEFF.MEMOS]COMPLAINT.TXT")
	mustDefine(t, db, "LNM$PROCESS", "PAY_FILE", Supervisor, "DISK1:[SALES_STAFF]PAYROLL")
	mustDefine(t, db, "LNM$PROCESS", "LIST", Supervisor, "DISK:[A]", "[B]")

	tests := []struct{ spec, want string }{
		{"PAY_FILE:*.DAT", "*.DAT"},
		{"MEMO", ""},
		{"DISK:[X]Y.Z", "[X]Y.Z"},
		{"LIST:F.DAT", "F.DAT|F.DAT"},
		{"UNDEFINED:F.DAT", "UNDEFINED:F.DAT"},
		{"[X]Y.Z", "[X]Y.Z"},
	}

	for _, tt := range tests {
		var got []string

		for _, f := range mustTranslateFileSpec(t, db, tt.spec) {
			if !strings.HasSuffix(f.Spec, f.Remainder) {
				t.Errorf("%q: Remainder %q isn't a suffix of Spec %q", tt.spec, f.Remainder, f.Spec)
			}

			got = append(got, f.Remainder)
		}

		if g := strings.Join(got, "|"); g != tt.want {
			t.Errorf("TranslateFileSpec(%q) remainders = %q, want %q", tt.spec, g, tt.want)
		}
	}
}
