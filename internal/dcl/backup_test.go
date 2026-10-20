package dcl

import (
	"slices"
	"testing"
)

// backupTestGrammar is BACKUP's shape (docs/PHASE-69 - backup save
// sets.md, subtask 6): a list input specifier and an output specifier,
// each with qualifiers of its own, some of them taking lists, beside the
// command's own qualifiers.
const backupTestGrammar = `
grammar test

verb backup
    parameter input/id=1/type=$string/list/prompt="From"
    qualifier save_set/id=2/parameter=input
    qualifier select/id=3/parameter=input/type=$string/list
    qualifier by_owner/id=4/parameter=input/type=$string/default=""

    parameter output/id=5/type=$string
    qualifier save_set/id=6/parameter=output
    qualifier by_owner/id=7/parameter=output/type=$string/default=""
    qualifier comment/id=8/parameter=output/type=$string

    qualifier list/id=9/type=$string/default=""
    qualifier log/id=10

end
`

// TestBackupShapedCommands parses the probe's command lines (testdata/
// backup69) against backupTestGrammar.
func TestBackupShapedCommands(t *testing.T) {
	g, err := ParseGrammar(backupTestGrammar)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}

	parse := func(line string) *Result {
		t.Helper()

		r, err := g.Parse(line)
		if err != nil {
			t.Fatalf("Parse(%q): %v", line, err)
		}

		return r
	}

	r := parse("BACKUP/LOG [SETS]B69.BCK/SAVE_SET/SELECT=([B69]TEXT.TXT,[B69]OWNED.TXT) [R10A]/BY_OWNER=ORIGINAL")
	if got := r.List("INPUT"); !slices.Equal(got, []string{"[SETS]B69.BCK"}) {
		t.Errorf("INPUT = %q", got)
	}

	if !r.Present("LOG") || !r.ParamPresent("INPUT", "SAVE_SET") || r.ParamPresent("OUTPUT", "SAVE_SET") {
		t.Error("LOG and INPUT's SAVE_SET should be present, OUTPUT's SAVE_SET not")
	}

	if got := r.ParamList("INPUT", "SELECT"); !slices.Equal(got, []string{"[B69]TEXT.TXT", "[B69]OWNED.TXT"}) {
		t.Errorf("INPUT's SELECT = %q", got)
	}

	if got := r.ParamString("OUTPUT", "BY_OWNER"); got != "ORIGINAL" {
		t.Errorf("OUTPUT's BY_OWNER = %q", got)
	}

	if !r.ParamDefaulted("INPUT", "BY_OWNER") || r.ParamDefaulted("OUTPUT", "BY_OWNER") {
		t.Error("INPUT's BY_OWNER should be defaulted, OUTPUT's given")
	}

	// A value without parentheses is a one-element list.
	r = parse("BACKUP [SETS]B69.BCK/SAVE_SET/SELECT=*.TXT [R5...]")
	if got := r.ParamList("INPUT", "SELECT"); !slices.Equal(got, []string{"*.TXT"}) {
		t.Errorf("INPUT's SELECT = %q", got)
	}

	if got := r.String("OUTPUT"); got != "[R5...]" {
		t.Errorf("OUTPUT = %q", got)
	}

	r = parse("BACKUP [B69]TEXT.TXT,[B69.SUB1]A.TXT [SETS]LIST.BCK/SAVE_SET")
	if got := r.List("INPUT"); !slices.Equal(got, []string{"[B69]TEXT.TXT", "[B69.SUB1]A.TXT"}) {
		t.Errorf("INPUT = %q", got)
	}

	if !r.ParamPresent("OUTPUT", "SAVE_SET") || r.ParamPresent("INPUT", "SAVE_SET") {
		t.Error("only OUTPUT's SAVE_SET should be present")
	}

	r = parse("BACKUP [B69...]*.*/BY_OWNER=[200,201] [SETS]B69BYO.BCK/SAVE_SET")
	if got := r.ParamString("INPUT", "BY_OWNER"); got != "[200,201]" {
		t.Errorf("INPUT's BY_OWNER = %q", got)
	}

	r = parse(`BACKUP [B69...]*.* [SETS]B69COM.BCK/SAVE_SET/COMMENT="Phase 69's probe, with a comment"`)
	if got := r.ParamString("OUTPUT", "COMMENT"); got != "Phase 69's probe, with a comment" {
		t.Errorf("OUTPUT's COMMENT = %q", got)
	}

	r = parse("BACKUP/LIST=[SETS]TEXT.LIS [SETS]TEXT.BCK/SAVE_SET")
	if got := r.String("LIST"); got != "[SETS]TEXT.LIS" || r.Present("OUTPUT") {
		t.Errorf("LIST = %q, OUTPUT present %v", got, r.Present("OUTPUT"))
	}

	r = parse("BACKUP/LIST [SETS]TEXT.BCK/SAVE_SET")
	if !r.Present("LIST") || r.String("LIST") != "" {
		t.Errorf("LIST = %q, present %v", r.String("LIST"), r.Present("LIST"))
	}
}
