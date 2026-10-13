package librtl

import (
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
)

// The context tests' arguments: the user-context cell, a copy of it, the
// component code, and the init-string's descriptor and text.
const (
	dtCell         = 0x2000
	dtCellCopy     = 0x2004
	dtComponentArg = 0x2008
	dtInitDesc     = 0x2010
	dtInitText     = 0x2100
)

// initContext calls LIB$INIT_DATE_TIME_CONTEXT on the cell at dtCell with
// component code and init-string text, returning its status.
func initContext(t *testing.T, env *corevms.Environment, code int32, text string) uint32 {
	t.Helper()

	putLongword(t, env, dtComponentArg, uint32(code))
	putDescriptor(t, env, dtInitDesc, dtInitText, text)

	return call(t, env, "LIB$INIT_DATE_TIME_CONTEXT", dtCell, dtComponentArg, dtInitDesc)
}

// defineList defines a process logical name at mode in table.
func defineList(t *testing.T, env *corevms.Environment, table, name string, mode lnm.Mode, values ...string) {
	t.Helper()

	var eqv []lnm.Equivalence
	for _, v := range values {
		eqv = append(eqv, lnm.Equivalence{Value: v})
	}

	if _, err := env.Logicals.Define(table, name, mode, 0, eqv); err != nil {
		t.Fatalf("define %s: %v", name, err)
	}
}

// elementTexts is the texts of an output format's elements.
func elementTexts(elements []dtElement) []string {
	var out []string
	for _, e := range elements {
		out = append(out, e.text)
	}

	return out
}

// TestInitDateTimeContext: a zero cell gets a new context, tagged on the
// heap, which later calls find; each component's spellings or formats go
// into it, and its settings come from them alone.
func TestInitDateTimeContext(t *testing.T) {
	env := fixture(t)
	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "ENGLISH")

	if got := initContext(t, env, componentMonthAbb, "|jan|feb|mar|apr|mai|jun|jul|aug|sep|okt|nov|dez|"); got != ssNormal {
		t.Fatalf("MONTH_NAME_ABB: %#x", got)
	}

	block := longword(t, env, dtCell)
	if block == 0 || longword(t, env, block) != dtContextTag {
		t.Fatalf("cell %#x, tag %#x", block, longword(t, env, block))
	}

	// More elements than the component has are ignored.
	if got := initContext(t, env, componentOutputFormat, "/!H04:!M0//!D0.!MN0.!Y4/"); got != ssNormal {
		t.Errorf("a doubled delimiter's three elements: %#x, want SS$_NORMAL", got)
	}

	if got := initContext(t, env, componentOutputFormat, "/!H04:!M0/!D0.!MN0.!Y4/"); got != ssNormal {
		t.Fatalf("OUTPUT_FORMAT: %#x", got)
	}

	if longword(t, env, dtCell) != block {
		t.Errorf("the second call moved the context to %#x", longword(t, env, dtCell))
	}

	c, status, err := dateContextFor(env, dtCell)
	if err != nil || status != ssNormal {
		t.Fatalf("dateContextFor: %#x, %v", status, err)
	}

	s := c.settings(env)

	if got := elementTexts(s.output); !slices.Equal(got, []string{"!H04:!M0", "!D0.!MN0.!Y4"}) {
		t.Errorf("output %q: the time first, as given", got)
	}

	// INIT's spellings are used as given, whatever the case asked for.
	if got := s.spellings[componentMonthAbb]; got[caseUpper][4] != "mai" || got[caseLower][9] != "okt" || got[caseCapital][11] != "dez" {
		t.Errorf("abbreviations: %q", got)
	}

	if got := s.spellings[componentMonthName][caseCapital][0]; got != "January" {
		t.Errorf("a month name not given: %q, want English's", got)
	}

	if s.outputWarning() != ssNormal {
		t.Errorf("output status %#x", s.outputWarning())
	}

	if s.inputWarning() != libDefForUse || s.inputText != standardInputFormat {
		t.Errorf("no input format: %#x, %q; want DEFFORUSE, the standard one", s.inputWarning(), s.inputText)
	}

	if got := initContext(t, env, componentInputFormat, "|!Y4-!MN0-!D0 !H04:!M0:!S0.!C2|"); got != ssNormal {
		t.Fatalf("INPUT_FORMAT: %#x", got)
	}

	if s := c.settings(env); s.inputText != "!Y4-!MN0-!D0 !H04:!M0:!S0.!C2" || s.inputWarning() != ssNormal {
		t.Errorf("input %q, %#x", s.inputText, s.inputWarning())
	}
}

// TestInitDateTimeContextErrors: each status INIT returns for a bad
// component or string. INIT allocates the context first, so a cell
// holding 0 gets one even when the call fails, as on VMS V7.1, and it
// doesn't check a format's mnemonics or fields.
func TestInitDateTimeContextErrors(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct {
		name string
		code int32
		text string
		want uint32
	}{
		{"an unnamed code", 5, "|X|", libIllComponent},
		{"past the last", 19, "|X|", libIllComponent},
		{"negative", -1, "|X|", libIllComponent},
		{"empty", componentInputFormat, "", libEleTooLon},
		{"one delimiter, output", componentOutputFormat, "|", libEleTooLon},
		{"one delimiter, input", componentInputFormat, "|", ssNormal},
		{"one delimiter, spellings", componentMeridiem, "|", libNumElements},
		{"no closing delimiter", componentMeridiem, "|AM|PM", libIllIniStr},
		{"too few", componentMeridiem, "|AM|", libNumElements},
		{"too many", componentWeekdayAbb, "|1|2|3|4|5|6|7|8|", ssNormal},
		{"255 bytes", componentLanguage, "|" + strings.Repeat("E", 255) + "|", ssNormal},
		{"256 bytes", componentLanguage, "|" + strings.Repeat("E", 256) + "|", libEleTooLon},
		{"unknown mnemonic", componentInputFormat, "|!DB-!MAAU-!Y5|", ssNormal},
		{"mixed element", componentOutputFormat, "|!DB-!MAAU-!Y4 !H04|!H04|", ssNormal},
		{"both empty", componentOutputFormat, "|||", ssNormal},
		{"empty language", componentLanguage, "| |", ssNormal},
	} {
		putLongword(t, env, dtCell, 0)

		if got := initContext(t, env, c.code, c.text); got != c.want {
			t.Errorf("%s: %#x, want %#x", c.name, got, c.want)
		}

		if longword(t, env, dtCell) == 0 {
			t.Errorf("%s: no context was allocated", c.name)
		}
	}

	putLongword(t, env, dtCell, 0)

	// One part alone is an output format.
	if got := initContext(t, env, componentOutputFormat, "||!H04:!M0|"); got != ssNormal {
		t.Errorf("time alone: %#x", got)
	}

	// A cell holding the address of no context.
	putLongword(t, env, dtCell, dtInitText)

	if got := initContext(t, env, componentInputFormat, "|!D0|"); got != libInvArg {
		t.Errorf("a cell naming no context: %#x, want LIB$_INVARG", got)
	}
}

// TestDateContextNames: an uninitialized context reads the logical
// names, falling back to English and the standard formats with their
// warnings.
func TestDateContextNames(t *testing.T) {
	env := fixture(t)

	s := (&dateContext{}).resolve(env)

	if s.outputWarning() != libEnglUsed || s.languageStatus != libEnglUsed {
		t.Errorf("nothing defined: %#x, want ENGLUSED", s.outputWarning())
	}

	if s.outputStatus != libDefForUse || s.inputStatus != libDefForUse {
		t.Errorf("nothing defined: format statuses %#x, %#x", s.outputStatus, s.inputStatus)
	}

	if got := elementTexts(s.output); !slices.Equal(got, []string{"!DB-!MAAU-!Y4", "!H04:!M0:!S0.!C2"}) {
		t.Errorf("standard output %q", got)
	}

	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "english")
	defineList(t, env, lnm.ProcessTableName, "LIB$DT_FORMAT", lnm.Supervisor, "LIB$TIME_FORMAT_012", "LIB$DATE_FORMAT_006")
	defineList(t, env, lnm.ProcessTableName, "LIB$DT_INPUT_FORMAT", lnm.Supervisor, "!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU")

	s = (&dateContext{}).resolve(env)

	if s.outputWarning() != ssNormal || s.inputWarning() != ssNormal {
		t.Errorf("all defined: %#x, %#x", s.outputWarning(), s.inputWarning())
	}

	if got := elementTexts(s.output); !slices.Equal(got, []string{"!HH2:!M0 !MIU", "!DB !MAAU !Y2"}) {
		t.Errorf("LIB$DT_FORMAT's order: %q", got)
	}

	if s.inputText != "!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU" {
		t.Errorf("input %q", s.inputText)
	}

	// A user's own format, added to the table, is used; a name that
	// isn't a format makes the whole format the standard one.
	defineList(t, env, lnm.DTFormatTableName, "LIB$DATE_FORMAT_501", lnm.Executive, "!Y4.!MN0.!D0")
	defineList(t, env, lnm.ProcessTableName, "LIB$DT_FORMAT", lnm.Supervisor, "LIB$DATE_FORMAT_501")

	s = (&dateContext{}).resolve(env)

	if got := elementTexts(s.output); !slices.Equal(got, []string{"!Y4.!MN0.!D0"}) || s.outputWarning() != ssNormal {
		t.Errorf("a user's format: %q, %#x", got, s.outputWarning())
	}

	defineList(t, env, lnm.ProcessTableName, "LIB$DT_FORMAT", lnm.Supervisor, "LIB$DATE_FORMAT_501", "LIB$TIME_FORMAT_999")

	s = (&dateContext{}).resolve(env)

	if got := elementTexts(s.output); !slices.Equal(got, []string{"!DB-!MAAU-!Y4", "!H04:!M0:!S0.!C2"}) || s.outputWarning() != libDefForUse {
		t.Errorf("a missing format: %q, %#x", got, s.outputWarning())
	}

	// A language with a table that has only some of its names.
	if _, _, err := env.Logicals.CreateTable("LNM$LANGUAGE_FRENCH", lnm.SystemTableName, lnm.Executive, 0); err != nil {
		t.Fatal(err)
	}

	defineList(t, env, lnm.ProcessTableName, "SYS$LANGUAGE", lnm.Supervisor, "FRENCH")

	for _, c := range []struct{ suffix, v string }{{"_U", "MATIN"}, {"_L", "matin"}, {"_C", "Matin"}} {
		defineList(t, env, "LNM$LANGUAGE_FRENCH", "LIB$MI"+c.suffix, lnm.Executive, c.v, c.v+"!")
	}

	s = (&dateContext{}).resolve(env)

	if got := s.spellings[componentMeridiem][caseCapital]; !slices.Equal(got, []string{"Matin", "Matin!"}) {
		t.Errorf("French meridiem %q", got)
	}

	if got := s.spellings[componentMonthName][caseUpper][0]; got != "JANUARY" || s.outputWarning() != libEnglUsed {
		t.Errorf("French months: %q, %#x; want English's, ENGLUSED", got, s.outputWarning())
	}
}

// TestDateContextLanguage: LIB$K_LANGUAGE makes an initialized context
// take its language's spellings from that language's table.
func TestDateContextLanguage(t *testing.T) {
	env := fixture(t)

	if _, _, err := env.Logicals.CreateTable("LNM$LANGUAGE_GERMAN", lnm.SystemTableName, lnm.Executive, 0); err != nil {
		t.Fatal(err)
	}

	defineList(t, env, "LNM$LANGUAGE_GERMAN", "LIB$FORMAT_MNEMONICS", lnm.Executive,
		"JJJJ", "MM", "TT", "SS", "MM", "SS", "HH", " ", "MONAT")

	if got := initContext(t, env, componentLanguage, "|german|"); got != ssNormal {
		t.Fatalf("LANGUAGE: %#x", got)
	}

	c, _, _ := dateContextFor(env, dtCell)
	s := c.settings(env)

	if got := s.spellings[componentMnemonics][caseUpper]; got[0] != "JJJJ" || got[8] != "MONAT" {
		t.Errorf("mnemonics %q", got)
	}

	if s.languageStatus != libEnglUsed {
		t.Errorf("the names the table lacks: %#x, want ENGLUSED", s.languageStatus)
	}
}

// TestFreeDateTimeContext: FREE frees the block and zeroes the cell; a
// stale copy of the cell is LIB$_BADBLOADR; a zero cell and the static
// context need nothing.
func TestFreeDateTimeContext(t *testing.T) {
	env := fixture(t)

	if got := initContext(t, env, componentMeridiem, "|vorm|nachm|"); got != ssNormal {
		t.Fatalf("INIT: %#x", got)
	}

	block := longword(t, env, dtCell)
	putLongword(t, env, dtCellCopy, block)

	if got := call(t, env, "LIB$FREE_DATE_TIME_CONTEXT", dtCell); got != ssNormal {
		t.Errorf("FREE: %#x", got)
	}

	if longword(t, env, dtCell) != 0 || longword(t, env, block) == dtContextTag {
		t.Errorf("cell %#x, tag %#x", longword(t, env, dtCell), longword(t, env, block))
	}

	if _, ok := env.BlockSize(block); ok {
		t.Errorf("the block is still allocated")
	}

	if got := call(t, env, "LIB$FREE_DATE_TIME_CONTEXT", dtCellCopy); got != libBadBlockAddress {
		t.Errorf("a stale copy: %#x, want BADBLOADR", got)
	}

	if got := call(t, env, "LIB$FREE_DATE_TIME_CONTEXT", dtCell); got != ssNormal {
		t.Errorf("a zero cell: %#x", got)
	}

	static, _, _ := dateContextFor(env, 0)
	static.settings(env)

	if got := call(t, env, "LIB$FREE_DATE_TIME_CONTEXT"); got != ssNormal || imageContexts(env).static != nil {
		t.Errorf("the static context: %#x", got)
	}

	// After FREE the cell is 0 again, so the next use makes a new context.
	if got := initContext(t, env, componentMeridiem, "|a|p|"); got != ssNormal || longword(t, env, dtCell) == 0 {
		t.Errorf("a new context: %#x", got)
	}
}
