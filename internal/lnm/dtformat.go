package lnm

import "fmt"

// DTFormatTableName is the table of the date and time formats LIBRTL's
// formatting routines (LIB$FORMAT_DATE_TIME and its neighbors) read: a
// program's LIB$DT_FORMAT names a date format and a time format here, as
// LIB$DATE_FORMAT_nnn and LIB$TIME_FORMAT_nnn, and each name's
// equivalence string is the format itself, written in the routines'
// mnemonics ("!DB-!MAAU-!Y4").
const DTFormatTableName = "LNM$DT_FORMAT_TABLE"

// DateFormats and TimeFormats are DIGITAL's predefined formats, the 5.0
// LIB$ manual's Tables 3-3 and 3-4: DateFormats[0] is LIB$DATE_FORMAT_001,
// and so on (docs/PHASE-56, "Formatting: mnemonics and predefined
// formats", which shows each with the manual's example). The OCR's "O"
// in the mnemonics is the digit 0, and its blanks are corrected from the
// examples. A user may add LIB$DATE_FORMAT_501 to _999 (and the time
// formats' alike) with DCL.
var (
	DateFormats = []string{
		"!DB-!MAAU-!Y4", "!DB !MAU !Y4", "!DB.!MAU !Y4", "!DB.!MAU.!Y4",
		"!DB !MAU !Y2", "!DB !MAAU !Y2", "!DB.!MAAU !Y2", "!DB.!MAAU.!Y2",
		"!DB !MAAU !Y4", "!DB.!MAAU !Y4", "!DB.!MAAU.!Y4", "!MAU !DD, !Y4",
		"!MN0/!D0/!Y2", "!MN0-!D0-!Y2", "!MN0.!D0.!Y2", "!MN0 !D0 !Y2",
		"!D0/!MN0/!Y2", "!D0/!MN0-!Y2", "!D0-!MN0-!Y2", "!D0.!MN0.!Y2",
		"!D0 !MN0 !Y2", "!Y2/!MN0/!D0", "!Y2-!MN0-!D0", "!Y2.!MN0.!D0",
		"!Y2 !MN0 !D0", "!Y2!MN0!D0", "/!Y2.!MN0.!D0", "!MN0/!D0/!Y4",
		"!MN0-!D0-!Y4", "!MN0.!D0.!Y4", "!MN0 !D0 !Y4", "!D0/!MN0/!Y4",
		"!D0-!MN0-!Y4", "!D0.!MN0.!Y4", "!D0 !MN0 !Y4", "!Y4/!MN0/!D0",
		"!Y4-!MN0-!D0", "!Y4.!MN0.!D0", "!Y4 !MN0 !D0", "!Y4!MN0!D0",
	}

	TimeFormats = []string{
		"!H04:!M0:!S0.!C2", "!H04:!M0:!S0", "!H04.!M0.!S0", "!H04 !M0 !S0",
		"!H04:!M0", "!H04.!M0", "!H04 !M0", "!HH4:!M0",
		"!HH4.!M0", "!HH4 !M0", "!H02:!M0 !MIU", "!HH2:!M0 !MIU",
		"!H04!M0", "!H04H!M0m", "kl !H04.!M0", "!H04H!M0'",
		"!H04.!M0 h", "h !H04.!M0", "!HH4 h !MM", "!HH4 h !MM min !SS s",
	}
)

// DateFormatName and TimeFormatName are the logical names of the nth
// date and time formats, counting from 1: LIB$DATE_FORMAT_001.
func DateFormatName(n int) string { return fmt.Sprintf("LIB$DATE_FORMAT_%03d", n) }

func TimeFormatName(n int) string { return fmt.Sprintf("LIB$TIME_FORMAT_%03d", n) }

// newDTFormatTable creates LNM$DT_FORMAT_TABLE in the system directory,
// holding the predefined formats. On VMS a startup procedure
// (LIB$DT_STARTUP) makes it; govax makes it with the rest of the shared
// tables, as system data a user's own vax.init needn't repeat (the
// author's choice, 2026-10-13). The table and its names are executive
// mode, as the manual asks of a format name, so a translation at any mode
// sees them and a user's own names (DEFINE/EXECUTIVE) go beside them.
// That the table's parent is the system directory, and that it is
// shareable, is unconfirmed: the probe shows VMS's.
func (db *Database) newDTFormatTable() {
	t := newTable(DTFormatTableName, Executive, db.SystemDirectory)
	t.permanent = true

	db.catalog(t, db.SystemDirectory, Executive, 0)

	define := func(name, format string) {
		t.add(&Entry{Name: name, Mode: Executive, Equivalences: []Equivalence{{Value: format}}})
	}

	for i, f := range DateFormats {
		define(DateFormatName(i+1), f)
	}

	for i, f := range TimeFormats {
		define(TimeFormatName(i+1), f)
	}
}

// The formats VMS V7.1's system table names by default, even without
// LIB$DT_STARTUP (testdata/lib56's log): LIB$DT_FORMAT the standard
// output format's two names, and LIB$DT_INPUT_FORMAT the standard input
// format, the two together $ASCTIM's text.
var (
	defaultOutputFormat = []string{DateFormatName(1), TimeFormatName(1)}
	defaultInputFormat  = DateFormats[0] + " " + TimeFormats[0]
)

// defineDTFormatNames defines LIB$DT_FORMAT and LIB$DT_INPUT_FORMAT in
// LNM$SYSTEM_TABLE as VMS V7.1 has them, so a formatting routine with
// nothing of the user's own finds the standard format without
// LIB$_DEFFORUSE. They are executive mode, as a startup procedure's
// would be (unconfirmed: SHOW LOGICAL doesn't show the mode); a user's
// DEFINE in the process table goes before them.
func (db *Database) defineDTFormatNames() {
	for _, t := range db.shared.tables {
		if t.Name != SystemTableName {
			continue
		}

		var output []Equivalence
		for _, name := range defaultOutputFormat {
			output = append(output, Equivalence{Value: name})
		}

		t.add(&Entry{Name: "LIB$DT_FORMAT", Mode: Executive, Equivalences: output})
		t.add(&Entry{Name: "LIB$DT_INPUT_FORMAT", Mode: Executive, Equivalences: []Equivalence{{Value: defaultInputFormat}}})
	}
}
