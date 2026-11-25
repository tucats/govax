// gen writes Phase 117's probe of SUMSLP (EDIT/SUM) and MACRO/UPDATE:
// cases.txt (every input file and every case, the one record of what the
// probe asks, which govax's oracle tests read), the input files under in/
// (gitignored, made again from cases.txt's data), the command procedures
// sum117.com and sum117t.com, and the govax console scripts exchange.cmd and
// copyout.cmd. Run it from the repository root:
//
//	go run testdata/sum117/gen.go
//
// See README.md, and docs/planning/PHASE-117 - SUMSLP.md's Part D.
//
// What the probe asks comes from the OpenVMS SUMSLP Utility Manual
// (AA-PS6EA-TE, May 1993): its operators (- \ % / <), the command form
// -locator1,locator2,/audittrail/;comment, the period locator, /LISTING,
// /OUTPUT, /HEADER, the positional /UPDATE with its defaults, the
// precedence of several update files (the last listed highest), and the
// 32,767-line limit. Each section asks what the manual leaves open.
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/sum117"

// A file is one input file in [SUM117.IN]: its name and its records.
// Format says how exchange.cmd makes it: "" is variable-length records
// copied as text, "FIX n" fixed records of n bytes copied as binary (so a
// record may hold any byte), "VFC" and "STMLF" text made into those formats.
type file struct {
	Name    string
	Format  string
	Records []string
}

// A step is one DCL command of a case, after which the procedure writes
// $STATUS and its message. Data, if any, are the lines that follow the
// command in the procedure (SYS$INPUT for /UPDATE=SYS$INPUT).
type step struct {
	Command string
	Data    []string
}

// A probeCase is one question: DCL lines run before (Pre), the steps, and
// DCL lines run after (Post). Every step's output that names SUM$OUT:ID.*
// comes back in out/.
type probeCase struct {
	ID      string
	Section string
	Title   string
	Pre     []string
	Steps   []step
	Post    []string
}

var (
	files []file
	cases []probeCase
	named = map[string]bool{}

	// round1Files and round1Cases are how many files and cases round 1
	// had: what follows them is round 2's (the VAX ran round 1 as it was,
	// so its files, cases, and procedure never change).
	round1Files, round1Cases int
)

// addFile adds an input file. A name may be added only once.
func addFile(name string, records ...string) {
	if named[name] {
		log.Fatalf("%s added twice", name)
	}

	files = append(files, file{Name: name, Records: records})
	named[name] = true
}

// addFormatted adds a file of a record format other than variable.
func addFormatted(name, format string, records ...string) {
	if named[name] {
		log.Fatalf("%s added twice", name)
	}

	files = append(files, file{Name: name, Format: format, Records: records})
	named[name] = true
}

// addCase adds a case of plain steps.
func addCase(section, id, title string, commands ...string) {
	c := probeCase{ID: id, Section: section, Title: title}
	for _, cmd := range commands {
		c.Steps = append(c.Steps, step{Command: cmd})
	}

	cases = append(cases, c)
}

// sumCase adds a case running EDIT/SUM with the output in SUM$OUT:ID.OUT and
// the listing in SUM$OUT:ID.LIS, quals after them, on src with the update
// files upds (none: no /UPDATE; one "*": a bare /UPDATE).
func sumCase(section, id, title, quals, src string, upds ...string) {
	addCase(section, id, title, sumCommand(id, quals, src, upds...))
}

// sumCommand is sumCase's command.
func sumCommand(id, quals, src string, upds ...string) string {
	return fmt.Sprintf("EDIT/SUM/OUTPUT=SUM$OUT:%s.OUT/LISTING=SUM$OUT:%s.LIS%s %s%s", id, id, quals, src, update(upds...))
}

// update is a /UPDATE qualifier for upds.
func update(upds ...string) string {
	switch {
	case len(upds) == 0:
		return ""
	case len(upds) == 1 && upds[0] == "*":
		return "/UPDATE"
	case len(upds) == 1:
		return "/UPDATE=" + upds[0]
	default:
		return "/UPDATE=(" + strings.Join(upds, ",") + ")"
	}
}

// lines makes n records "prefix 1" to "prefix n".
func lines(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s %d", prefix, i+1)
	}

	return out
}

// edit returns records with what is at [from, to) replaced by with.
func edit(records []string, from, to int, with ...string) []string {
	out := append([]string{}, records[:from]...)
	out = append(out, with...)

	return append(out, records[to:]...)
}

// s10 is the source most cases edit: "source line 1" to "source line 10".
var s10 = lines("source line", 10)

func main() {
	sources()
	help()
	command()
	basics()
	syntax()
	order()
	merge()
	audit()
	listing()
	formats()
	roundtrip()
	macro()

	round1Files, round1Cases = len(files), len(cases)

	clash2()
	numbers()
	pastEnd()
	lengths()
	pages()
	slpVerb()
	dates()

	writeCases()
	writeInputs()
	writeProcedure()
	writeTerminal()
	writeExchange()
	writeCopyout()
	writeRound2Scripts()
}

// sources adds the input files many sections share.
func sources() {
	addFile("S10.SRC", s10...)
	addFile("S03.SRC", lines("short line", 3)...)
	addFile("EMPTY.SRC")
	addFile("S10.UPD", "-4,4", "S10.UPD replaced line 4", "/")
}

// help writes the online help the manual points to into the log first, and
// whatever HELP/MESSAGE knows of SUMSLP's facility.
func help() {
	const s = "HELP"

	addCase(s, "H01", "HELP EDIT /SUM, every subtopic", "HELP/NOPROMPT EDIT /SUM...")
	addCase(s, "H02", "HELP SUMSLP, every subtopic", "HELP/NOPROMPT SUMSLP...")
	addCase(s, "H03", "HELP EDIT, the topic's own text and subtopic list", "HELP/NOPROMPT EDIT")
	addCase(s, "H04", "HELP MACRO /UPDATE", "HELP/NOPROMPT MACRO /UPDATE...")
	addCase(s, "H05", "HELP/MESSAGE of the SUM facility", "HELP/MESSAGE/FACILITY=SUM")
	addCase(s, "H06", "HELP/MESSAGE of the SUMSLP facility", "HELP/MESSAGE/FACILITY=SUMSLP")
}

// command is A.3's first question: the qualifiers, the names, the errors.
func command() {
	const s = "COMMAND"

	addFile("C01.UPD", "-5,5", "C01 replaced line 5", "/")
	sumCase(s, "C01", "an update file named without its type (.UPD the default)", "", "S10.SRC", "C01")
	sumCase(s, "C02", "a bare /UPDATE: the input's name, type .UPD", "", "S10.SRC", "*")
	sumCase(s, "C03", "no /UPDATE: the input copied, and its numbered listing", "", "S10.SRC")

	addFile("C04A.UPD", "-2,2", "C04A replaced line 2", "/")
	addFile("C04B.ENH", "-5,5", "C04B replaced line 5", "/")
	addFile("C04C.ENH", "-8,8", "C04C replaced line 8", "/")
	sumCase(s, "C04", "three update files, defaults from the one before (C04C is .ENH)", "", "S10.SRC", "C04A", "C04B.ENH", "C04C")

	addCase(s, "C05", "/UPDATE after the verb (the manual: a file qualifier only)",
		"EDIT/SUM/UPDATE=C01/OUTPUT=SUM$OUT:C05.OUT/LISTING=SUM$OUT:C05.LIS S10.SRC")
	addCase(s, "C06", "/LISTING after the file (the manual: a command qualifier only)",
		"EDIT/SUM/OUTPUT=SUM$OUT:C06.OUT S10.SRC/LISTING=SUM$OUT:C06.LIS/UPDATE=C01")
	addCase(s, "C07", "/OUTPUT after the file",
		"EDIT/SUM/LISTING=SUM$OUT:C07.LIS S10.SRC/OUTPUT=SUM$OUT:C07.OUT/UPDATE=C01")

	// The default names, in a scratch directory of copies so [SUM117.IN]
	// gets no new versions.
	scratch := []string{
		"COPY S10.SRC SUM$SCR:",
		"COPY C01.UPD SUM$SCR:",
		"COPY S10.UPD SUM$SCR:",
		"SET DEFAULT SUM$SCR:",
	}
	scratchEnd := []string{
		"DIRECTORY/SIZE/DATE SUM$SCR:",
		"SET DEFAULT SUM$TOP:[SUM117.IN]",
	}
	keep := func(id string, names ...string) []string {
		var out []string
		for i, n := range names {
			out = append(out, fmt.Sprintf("IF F$SEARCH(\"SUM$SCR:%s\") .NES. \"\" THEN COPY SUM$SCR:%s SUM$OUT:%s.K%d", n, n, id, i+1))
		}

		return append(out, "DELETE SUM$SCR:*.*;*/NOLOG")
	}
	scr := func(id, title, cmd string, names ...string) {
		post := append(append([]string{}, scratchEnd...), keep(id, names...)...)
		cases = append(cases, probeCase{ID: id, Section: s, Title: title, Pre: scratch, Steps: []step{{Command: cmd}}, Post: post})
	}

	scr("C08", "no /OUTPUT: the output's name (S10.SRC;2, the manual says)",
		"EDIT/SUM S10.SRC/UPDATE=C01", "S10.SRC;2")
	scr("C09", "a bare /LISTING: the listing's name (S10.LIS)",
		"EDIT/SUM/LISTING S10.SRC/UPDATE=C01", "S10.LIS;1", "S10.SRC;2")
	scr("C10", "a bare /OUTPUT", "EDIT/SUM/OUTPUT S10.SRC/UPDATE=C01", "S10.SRC;2")
	scr("C11", "/NOOUTPUT with a listing", "EDIT/SUM/NOOUTPUT/LISTING S10.SRC/UPDATE=C01", "S10.LIS;1", "S10.SRC;2")
	scr("C12", "/OUTPUT naming only a type", "EDIT/SUM/OUTPUT=.NEW/LISTING=.LST S10.SRC/UPDATE=C01", "S10.NEW;1", "S10.LST;1")
	scr("C13", "the input named with a version, ;1, and no /OUTPUT", "EDIT/SUM S10.SRC;1/UPDATE=C01", "S10.SRC;2")
	scr("C14", "a bare /UPDATE and no /OUTPUT, in the input's directory", "EDIT/SUM S10.SRC/UPDATE", "S10.SRC;2")

	addCase(s, "C15", "the input in another directory: the update file's default directory",
		"SET DEFAULT SUM$SCR:",
		"EDIT/SUM/OUTPUT=SUM$OUT:C15.OUT/LISTING=SUM$OUT:C15.LIS [-.IN]S10.SRC/UPDATE=C01",
		"SET DEFAULT SUM$TOP:[SUM117.IN]")
	addCase(s, "C16", "an update file in another directory, then one with no directory",
		"EDIT/SUM/OUTPUT=SUM$OUT:C16.OUT/LISTING=SUM$OUT:C16.LIS S10.SRC/UPDATE=([-.IN2]C16A,C16B)")
	addFile("[SUM117.IN2]C16A.UPD", "-3,3", "C16A, in [SUM117.IN2], replaced line 3", "/")
	addFile("[SUM117.IN2]C16B.UPD", "-6,6", "C16B, in [SUM117.IN2], replaced line 6", "/")
	addFile("C16B.UPD", "-7,7", "C16B, in [SUM117.IN], replaced line 7", "/")

	addCase(s, "C17", "the input file missing",
		"EDIT/SUM/OUTPUT=SUM$OUT:C17.OUT/LISTING=SUM$OUT:C17.LIS NOSUCH.SRC/UPDATE=C01")
	sumCase(s, "C18", "the update file missing", "", "S10.SRC", "NOSUCH")
	sumCase(s, "C19", "one of two update files missing", "", "S10.SRC", "NOSUCH", "C01")
	addCase(s, "C20", "a wildcard in the input",
		"EDIT/SUM/OUTPUT=SUM$OUT:C20.OUT/LISTING=SUM$OUT:C20.LIS S1*.SRC/UPDATE=C01")
	sumCase(s, "C21", "a wildcard in an update file", "", "S10.SRC", "C0*")
	sumCase(s, "C22", "the same update file twice", "", "S10.SRC", "C01", "C01")
	addCase(s, "C23", "EDIT/SLP, the older name",
		"EDIT/SLP/OUTPUT=SUM$OUT:C23.OUT/LISTING=SUM$OUT:C23.LIS S10.SRC/UPDATE=C01")
	sumCase(s, "C24", "/NOLISTING after /LISTING, and /NOHEADER", "/NOLISTING/NOHEADER", "S10.SRC", "C01")
	addCase(s, "C25", "a qualifier SUMSLP doesn't have",
		"EDIT/SUM/READ_ONLY/OUTPUT=SUM$OUT:C25.OUT S10.SRC/UPDATE=C01")
	addCase(s, "C26", "EDIT/SUM with EDIT/TECO's qualifier",
		"EDIT/SUM/TECO/OUTPUT=SUM$OUT:C26.OUT S10.SRC/UPDATE=C01")
	addCase(s, "C27", "two input files",
		"EDIT/SUM/OUTPUT=SUM$OUT:C27.OUT S10.SRC,S03.SRC/UPDATE=C01")
	addCase(s, "C28", "the input a directory file",
		"EDIT/SUM/OUTPUT=SUM$OUT:C28.OUT [SUM117]IN.DIR/UPDATE=C01")

	cases = append(cases, probeCase{ID: "C29", Section: s, Title: "the update file SYS$INPUT, its lines in the procedure",
		Steps: []step{{Command: "EDIT/SUM/OUTPUT=SUM$OUT:C29.OUT/LISTING=SUM$OUT:C29.LIS S10.SRC/UPDATE=SYS$INPUT",
			Data: []string{"-3", "C29 inserted after line 3", "-4,4", "C29 replaced line 4", "/"}}}})
	cases = append(cases, probeCase{ID: "C30", Section: s, Title: "SYS$INPUT with no terminator: the next $ line ends it",
		Steps: []step{{Command: "EDIT/SUM/OUTPUT=SUM$OUT:C30.OUT/LISTING=SUM$OUT:C30.LIS S10.SRC/UPDATE=SYS$INPUT",
			Data: []string{"-3", "C30 inserted after line 3"}}}})

	addFile("C31.UPD", "-5,5", "C31 replaced line 5", "/")
	addCase(s, "C31", "the output the same file as the input, named with no version",
		"COPY S10.SRC SUM$SCR:",
		"COPY C31.UPD SUM$SCR:",
		"EDIT/SUM/OUTPUT=SUM$SCR:S10.SRC SUM$SCR:S10.SRC/UPDATE=SUM$SCR:C31",
		"DIRECTORY/SIZE SUM$SCR:",
		"COPY SUM$SCR:S10.SRC;2 SUM$OUT:C31.OUT",
		"DELETE SUM$SCR:*.*;*/NOLOG")
}

// basics is A.3's second question: every edit command on one update file.
func basics() {
	const s = "BASICS"

	b := func(id, title string, upd ...string) {
		addFile(id+".UPD", upd...)
		sumCase(s, id, title, "", "S10.SRC", id)
	}

	b("B01", "replace one record", "-5,5", "B01 replaced line 5", "/")
	b("B02", "replace five with two", "-3,7", "B02 first", "B02 second", "/")
	b("B03", "replace one with three", "-4,4", "B03 first", "B03 second", "B03 third", "/")
	b("B04", "replace all", "-1,10", "B04 everything", "/")
	b("B05", "insert after the first", "-1", "B05 after line 1", "/")
	b("B06", "insert after the fifth", "-5", "B06 after line 5", "B06 again", "/")
	b("B07", "insert after the last", "-10", "B07 after line 10", "/")
	b("B08", "a bare minus (DIFFERENCES/SLP's insert at the start)", "-", "B08 before line 1?", "/")
	b("B09", "insert after line 0", "-0", "B09 before line 1?", "/")
	b("B10", "delete one", "-5,5", "/")
	b("B11", "delete four", "-3,6", "/")
	b("B12", "delete the last", "-10,10", "/")
	b("B13", "delete all", "-1,10", "/")
	b("B14", "delete the first", "-1,1", "/")
	b("B15", "an insert, a replace, a delete", "-2", "B15 after line 2", "-4,5", "B15 replaced 4 and 5", "-8,9", "/")
	b("B16", "only the terminator", "/")
	b("B17", "no terminator", "-5,5", "B17 replaced line 5")
	b("B18", "records after the terminator", "-5,5", "B18 replaced line 5", "/", "-7,7", "B18 after the terminator")
	b("B19", "an empty update file")
	b("B20", "an inserted record starting with a minus", "-5", "-B20 a minus first", "/")
	b("B21", "a minus made a character by <", "-5", "<-B21 a minus first", "/")
	b("B22", "a slash made a character by <", "-5", "</B22 a slash first", "/")
	b("B23", "< made a character by <", "-5", "<<B23 a less-than first", "/")
	b("B24", "< before a character that is no operator", "-5", "<B24 a less-than first", "/")
	b("B25", "a record starting with a backslash", "-5", "\\B25 a backslash first", "B25 the next record", "/")
	b("B26", "a record starting with a percent sign", "-5", "\\", "B26 after a backslash alone", "%B26 a percent first", "B26 the next record", "/")
	b("B27", "a backslash alone, then text, then a percent alone", "-3", "\\", "B27 suppressed?", "%", "B27 enabled?", "/")
	b("B28", "< making a backslash and a percent characters", "-5", "<\\B28 backslash", "<%B28 percent", "/")
	b("B29", "records with a leading blank, a tab, and nothing",
		"-5", " B29 a leading blank", "\tB29 a leading tab", "", "B29 after an empty record", "/")
	b("B30", "a record of 300 characters", "-5", strings.Repeat("B30 long ", 33)+"end", "/")
	b("B31", "the period locator: delete line 2 (the manual's example)", "-2,.", "/")
	b("B32", "the period after a move: insert at the current line", "-3", "B32 after 3", "-.", "B32 after the current", "/")
	b("B33", "a period alone", "-.", "B33 at the start?", "/")
	b("B34", "a comment after a one-locator command", "-5;B34 a comment", "B34 after line 5", "/")
	b("B35", "a comment after a two-locator command", "-5,5;B35 a comment", "B35 replaced line 5", "/")
	b("B36", "a comment after the audit trail", "-5,5,/;B36/;a comment", "B36 replaced line 5", "/")
	b("B37", "a comment with no commas before it", "-5,5 ;B37 a comment", "B37 replaced line 5", "/")
	b("B38", "a trailing comma", "-5,", "B38 after line 5?", "/")
	b("B39", "the terminator followed by text", "-5,5", "B39 replaced line 5", "/ trailing text")
	b("B40", "a terminator with a leading blank (not a terminator?)", "-5,5", "B40 replaced line 5", " /", "/")
	b("B41", "a record of only <", "-5", "<", "B41 after <", "/")
}

// syntax is the second question's errors: each with its message.
func syntax() {
	const s = "SYNTAX"

	y := func(id, title string, upd ...string) {
		addFile(id+".UPD", upd...)
		sumCase(s, id, title, "", "S10.SRC", id)
	}

	y("Y01", "a blank after the minus", "- 5,5", "Y01 text", "/")
	y("Y02", "leading zeros", "-05,05", "Y02 text", "/")
	y("Y03", "a plus sign", "-+5,5", "Y03 text", "/")
	y("Y04", "blanks around the comma", "-5 , 5", "Y04 text", "/")
	y("Y05", "a number past 32767", "-99999", "Y05 text", "/")
	y("Y06", "32767 on a ten-line file", "-32767", "Y06 text", "/")
	y("Y07", "three commas", "-5,,,", "Y07 text", "/")
	y("Y08", "a letter for a locator", "-A", "Y08 text", "/")
	y("Y09", "a letter for the second locator", "-5,A", "Y09 text", "/")
	y("Y10", "an unterminated audit trail", "-5,5,/;Y10", "Y10 text", "/")
	y("Y11", "two numbers with a blank between", "-5 6", "Y11 text", "/")
	y("Y12", "insert after a line past the end", "-11", "Y12 text", "/")
	y("Y13", "replace lines past the end", "-11,12", "Y13 text", "/")
	y("Y14", "a range running past the end", "-9,11", "Y14 text", "/")
	y("Y15", "a tab after the minus", "-\t5,5", "Y15 text", "/")
	y("Y16", "an audit trail with no slashes", "-5,5,;Y16", "Y16 text", "/")
	y("Y17", "an audit trail without its two commas (one locator)", "-5,/;Y17/", "Y17 text", "/")
	y("Y18", "an empty audit trail", "-5,5,//", "Y18 text", "/")
	y("Y19", "a negative-looking second locator", "-5,-6", "Y19 text", "/")
	y("Y20", "a hexadecimal-looking number", "-%X5", "Y20 text", "/")
	y("Y21", "an error after good commands", "-2,2", "Y21 good", "-B", "Y21 text", "/")
	y("Y22", "a minus alone with an audit", "-,,/;Y22/", "Y22 inserted where?", "/")
	y("Y23", "a lowercase-looking locator with blanks after", "-5   ", "Y23 text", "/")
	y("Y24", "a range of zero to five", "-0,5", "Y24 text", "/")
}

// order is A.3's third question: commands out of order and overlapping in
// one file.
func order() {
	const s = "ORDER"

	o := func(id, title string, upd ...string) {
		addFile(id+".UPD", upd...)
		sumCase(s, id, title, "", "S10.SRC", id)
	}

	o("O01", "an insert before an earlier one", "-7", "O01 after 7", "-3", "O01 after 3", "/")
	o("O02", "overlapping ranges", "-3,6", "O02 replaced 3 to 6", "-5,8", "O02 replaced 5 to 8", "/")
	o("O03", "two inserts at one place", "-5", "O03 first", "-5", "O03 second", "/")
	o("O04", "a reversed range", "-9,5", "O04 text", "/")
	o("O05", "an insert after a line deleted earlier", "-3,6", "-4", "O05 after 4", "/")
	o("O06", "a replace, then an insert after the replaced line", "-5,5", "O06 replaced 5", "-5", "O06 after 5", "/")
	o("O07", "an insert, then a replace of that line", "-5", "O07 after 5", "-5,5", "O07 replaced 5", "/")
	o("O08", "a period range from the current line", "-3", "O08 after 3", "-.,5", "O08 replaced to 5", "/")
	o("O09", "the same range replaced twice", "-4,4", "O09 first", "-4,4", "O09 second", "/")
	o("O10", "a replace then a replace of the line before", "-6,6", "O10 replaced 6", "-5,5", "O10 replaced 5", "/")
	o("O11", "adjacent replaces", "-4,4", "O11 replaced 4", "-5,5", "O11 replaced 5", "/")
	o("O12", "the last line deleted, then inserted after", "-10,10", "-10", "O12 after 10", "/")
}

// merge is A.3's fourth question: several update files merged.
func merge() {
	const s = "MERGE"

	upd := func(name string, records ...string) { addFile(name+".UPD", records...) }

	upd("G01A", "-2,2", "G01A replaced 2", "/")
	upd("G01B", "-8,8", "G01B replaced 8", "/")
	sumCase(s, "G01", "disjoint edits", "", "S10.SRC", "G01A", "G01B")

	pair := func(id, title string) {
		sumCase(s, id, title+", A then B", "", "S10.SRC", id+"A", id+"B")
		sumCase(s, id+"R", title+", B then A", "", "S10.SRC", id+"B", id+"A")
	}

	upd("G02A", "-5", "G02A after 5", "G02A again", "/")
	upd("G02B", "-5", "G02B after 5", "/")
	pair("G02", "two inserts at one place")

	upd("G03A", "-5,5", "G03A replaced 5", "/")
	upd("G03B", "-5,5", "G03B replaced 5", "/")
	pair("G03", "two replaces of one line")

	upd("G04A", "-5,5", "G04A replaced 5", "/")
	upd("G04B", "-5", "G04B after 5", "/")
	pair("G04", "a replace and an insert after the same line")

	upd("G05A", "-4,6", "/")
	upd("G05B", "-5", "G05B after 5", "/")
	pair("G05", "a delete of 4 to 6 and an insert after 5")

	upd("G06A", "-3,7", "G06A replaced 3 to 7", "/")
	upd("G06B", "-5,5", "G06B replaced 5", "/")
	pair("G06", "a range and a replace inside it")

	upd("G07A", "-3,6", "G07A replaced 3 to 6", "/")
	upd("G07B", "-5,8", "G07B replaced 5 to 8", "/")
	pair("G07", "ranges that overlap")

	upd("G08A", "-10,10", "/")
	upd("G08B", "-10", "G08B after 10", "/")
	pair("G08", "a delete of the last line and an insert after it")

	upd("G09A", "-4,4", "G09A replaced 4", "/")
	upd("G09B", "-4", "G09B after 4", "/")
	pair("G09", "a replace and an insert, the insert the same line's")

	upd("G10A", "-5,5", "G10A replaced 5", "/")
	upd("G10B", "-4", "G10B after 4", "/")
	pair("G10", "a replace and an insert after the line before")

	upd("G11A", "-", "G11A at the start", "/")
	upd("G11B", "-", "G11B at the start", "/")
	pair("G11", "two bare minus inserts")

	upd("G12A", "-5,5,/;AAA/", "G12A replaced 5", "-8", "G12A after 8", "/")
	upd("G12B", "-2,,/;BBB/", "G12B after 2", "-8", "G12B after 8", "/")
	pair("G12", "each file's audit trail")

	upd("G13A", "-5", "G13A after 5", "/")
	upd("G13B", "-5", "G13B after 5", "/")
	upd("G13C", "-5", "G13C after 5", "/")
	sumCase(s, "G13", "three inserts at one place", "", "S10.SRC", "G13A", "G13B", "G13C")
	sumCase(s, "G13R", "three inserts at one place, the other order", "", "S10.SRC", "G13C", "G13B", "G13A")

	upd("G14A", "-5,5", "G14A replaced 5", "/")
	upd("G14B", "-4,6", "/")
	upd("G14C", "-5", "G14C after 5", "/")
	sumCase(s, "G14", "a replace, a delete around it, an insert", "", "S10.SRC", "G14A", "G14B", "G14C")
	sumCase(s, "G14R", "the same, the other order", "", "S10.SRC", "G14C", "G14B", "G14A")

	upd("G15A", "-3,3", "G15A replaced 3", "-6", "G15A after 6", "-9,9", "/")
	upd("G15B", "-1", "G15B after 1", "-6,6", "G15B replaced 6", "-9", "G15B after 9", "/")
	pair("G15", "several edits each, some meeting")

	sumCase(s, "G16", "one file twice", "", "S10.SRC", "G02A", "G02A")

	upd("G17A", "-5", "G17A after 5", "/")
	upd("G17B", "-5,5", "/")
	pair("G17", "an insert after a line the other deletes")

	upd("G18A", "-3,5", "G18A replaced 3 to 5", "/")
	upd("G18B", "-5,7", "G18B replaced 5 to 7", "/")
	pair("G18", "ranges sharing one line")
}

// audit is A.3's fifth question: the audit trail in the output and the
// listing.
func audit() {
	const s = "AUDIT"

	addFile("W10.SRC",
		"short",
		"\tMOVL\t#1,R0\t\t; a tab-laid line",
		strings.Repeat("x", 70),
		strings.Repeat("y", 79),
		strings.Repeat("z", 80),
		strings.Repeat("w", 100),
		strings.Repeat("v", 132),
		strings.Repeat("u", 140),
		"line 9   ",
		"line 10")

	u := func(id, title, src string, upd ...string) {
		addFile(id+".UPD", upd...)
		sumCase(s, id, title, "", src, id)
	}

	u("U01", "the default audit trail on inserts and replaces", "S10.SRC", "-2", "U01 after 2", "-5,6", "U01 replaced 5 and 6", "/")
	u("U02", "an audit trail on a replace", "S10.SRC", "-5,5,/;U02 AUDIT/", "U02 replaced 5", "/")
	u("U03", "the audit trail changed between commands", "S10.SRC",
		"-2", "U03 default", "-,,/;U03 NEW/", "-5", "U03 after 5", "-7,7", "U03 replaced 7", "/")
	u("U04", "audit trails on records of every width", "W10.SRC",
		"-1,9,/;U04/", "short", "\tMOVL\t#2,R0\t\t; a tab-laid line",
		strings.Repeat("x", 70), strings.Repeat("y", 79), strings.Repeat("z", 80),
		strings.Repeat("w", 100), strings.Repeat("v", 132), strings.Repeat("u", 140), "line 9   ", "/")
	u("U05", "deletions' markers, the last at the end", "S10.SRC", "-2,3", "-9,10", "/")
	u("U06", "a long audit trail", "S10.SRC", "-5,5,/;"+strings.Repeat("A", 60)+"/", "U06 replaced 5", "/")
	u("U07", "an audit trail with no comment character", "S10.SRC", "-5,5,/U07 AUDIT/", "U07 replaced 5", "/")
	u("U08", "an audit trail holding commas and a semicolon", "S10.SRC", "-5,5,/;U08, a, b; c/", "U08 replaced 5", "/")
	u("U09", "suppressed and enabled within one insert", "S10.SRC",
		"-5,,/;U09/", "U09 one", "\\U09 two, suppressed?", "U09 three", "%U09 four, enabled?", "U09 five", "/")
	u("U10", "an audit trail on a delete", "S10.SRC", "-5,6,/;U10/", "/")

	addFile("U11.UPD", "-5,5", "U11 replaced 5", "-8", "U11 after 8", "/")
	sumCase(s, "U11", "/HEADER: the output's record headers", "/HEADER", "S10.SRC", "U11")
	addCase(s, "U11D", "/HEADER: the output's records dumped",
		"DUMP/RECORD SUM$OUT:U11.OUT", "DIRECTORY/FULL SUM$OUT:U11.OUT")
	sumCase(s, "U12", "/HEADER with audit trails", "/HEADER", "S10.SRC", "U02")
	addCase(s, "U12D", "/HEADER with audit trails, dumped", "DUMP/RECORD SUM$OUT:U12.OUT")
	sumCase(s, "U13", "/HEADER with no update", "/HEADER", "S10.SRC")
	addCase(s, "U13D", "/HEADER with no update, dumped", "DUMP/RECORD SUM$OUT:U13.OUT")
	sumCase(s, "U14", "/HEADER with two files merged", "/HEADER", "S10.SRC", "G12A", "G12B")
	addCase(s, "U14D", "/HEADER with two files merged, dumped", "DUMP/RECORD SUM$OUT:U14.OUT")
}

// listing is A.3's sixth question: the listing's layout.
func listing() {
	const s = "LISTING"

	sumCase(s, "L01", "a listing of a file with no update", "", "W10.SRC")
	sumCase(s, "L02", "a listing of an empty file", "", "EMPTY.SRC")

	addFile("L03.SRC", lines("listing record", 1200)...)
	addFile("L03.UPD", "-9", "L03 after 9", "-99,100", "L03 replaced 99 and 100", "-999,1001", "-1200", "L03 after 1200", "/")
	sumCase(s, "L03", "line numbers past 999", "", "L03.SRC", "L03")

	addFile("L04.SRC", lines("big record", 12000)...)
	addFile("L04.UPD", "-9999", "L04 after 9999", "-10000,10001", "L04 replaced 10000 and 10001", "/")
	sumCase(s, "L04", "line numbers past 9999", "", "L04.SRC", "L04")

	addFile("L05.UPD", "-5", "L05 one", "L05 two", "L05 three", "L05 four", "L05 five",
		"L05 six", "L05 seven", "L05 eight", "L05 nine", "L05 ten", "L05 eleven", "/")
	sumCase(s, "L05", "more than nine records inserted together", "", "S10.SRC", "L05")
	sumCase(s, "L06", "an empty file updated", "", "EMPTY.SRC", "B08")
	sumCase(s, "L07", "an empty file, an insert after line 0", "", "EMPTY.SRC", "B09")
	sumCase(s, "L08", "an empty file, an insert after line 1", "", "EMPTY.SRC", "B05")

	addFile("LIMIT1.SRC", lines("limit", 32767)...)
	addFile("LIMIT2.SRC", lines("limit", 32768)...)
	addFile("L09.UPD", "-32767", "L09 after the last", "/")
	addCase(s, "L09", "a file of 32,767 lines",
		"EDIT/SUM/OUTPUT=SUM$OUT:L09.OUT LIMIT1.SRC/UPDATE=L09")
	addCase(s, "L10", "a file of 32,768 lines",
		"EDIT/SUM/OUTPUT=SUM$OUT:L10.OUT LIMIT2.SRC/UPDATE=L09")
	addCase(s, "L10D", "the outputs' sizes", "DIRECTORY/SIZE SUM$OUT:L09.OUT,L10.OUT")
}

// formats is A.3's eighth question: record formats other than variable, and
// records holding any byte.
func formats() {
	const s = "FORMATS"

	text := []string{"vfc line 1", "vfc line 2", "vfc line 3", "vfc line 4"}
	addFormatted("F01.VFC", "VFC", text...)
	addFormatted("F02.STM", "STMLF", "stream line 1", "stream line 2", "stream line 3", "stream line 4")

	var fixed []string
	for i := 0; i < 8; i++ {
		var b strings.Builder
		for j := 0; j < 32; j++ {
			b.WriteByte(byte(i*32 + j))
		}

		fixed = append(fixed, b.String())
	}

	addFormatted("F03.FIX", "FIX 32", fixed...)
	addFormatted("F04.FIX", "FIX 10", "fixed 0001", "fixed 0002", "fixed 0003", "fixed 0004")
	addFile("F01.UPD", "-2,2", "F01 replaced 2", "-4", "F01 after 4", "/")

	f := func(id, title, src string) {
		sumCase(s, id, title, "", src, "F01")
		addCase(s, id+"D", title+": the output's attributes",
			fmt.Sprintf("DIRECTORY/FULL SUM$OUT:%s.OUT", id), fmt.Sprintf("DUMP/RECORD SUM$OUT:%s.OUT", id))
	}

	f("F01", "a VFC input", "F01.VFC")
	f("F02", "a STREAM_LF input", "F02.STM")
	f("F03", "a fixed input holding every byte", "F03.FIX")
	f("F04", "a fixed input of text", "F04.FIX")
	f("F05", "a variable input (S10)", "S10.SRC")
	addCase(s, "F05S", "a variable input: the source's attributes", "DIRECTORY/FULL S10.SRC")

	addFormatted("F06.UPD", "STMLF", "-2,2", "F06 replaced 2, the update STREAM_LF", "/")
	sumCase(s, "F06", "a STREAM_LF update file", "", "S10.SRC", "F06")
	addFormatted("F07.UPD", "VFC", "-2,2", "F07 replaced 2, the update VFC", "/")
	sumCase(s, "F07", "a VFC update file", "", "S10.SRC", "F07")

	addFile("F08.UPD", "-2,2", "F08 \x01\x02 control \x1b[1m bytes \x7f\xa0\xff", "/")
	sumCase(s, "F08", "inserted records holding control and 8-bit bytes", "", "S10.SRC", "F08")
	addCase(s, "F08D", "those records dumped", "DUMP/RECORD SUM$OUT:F08.OUT")

	addFile("F09.SRC", "tab\there", "form\ffeed", "", "trailing   ", "\tlead tab", "end")
	addFile("F09.UPD", "-3,3", "F09 replaced the empty record", "/")
	sumCase(s, "F09", "a source with tabs, a form feed, an empty record, trailing blanks", "", "F09.SRC", "F09")
	addCase(s, "F09D", "that output dumped", "DUMP/RECORD SUM$OUT:F09.OUT")
}

// roundtrip is A.3's ninth question: DIFFERENCES/SLP's script applied by
// SUMSLP gives the revision back. Each pair: the script, the update, and a
// DIFFERENCES of the result with the revision (SUCCESS if identical).
func roundtrip() {
	const s = "ROUNDTRIP"

	trip := func(id, title string, master, revision []string) {
		addFile(id+".A", master...)
		addFile(id+".B", revision...)
		addCase(s, id, title,
			fmt.Sprintf("DIFFERENCES/SLP/OUTPUT=SUM$OUT:%s.DIF %s.A %s.B", id, id, id),
			fmt.Sprintf("EDIT/SUM/OUTPUT=SUM$OUT:%s.OUT %s.A/UPDATE=SUM$OUT:%s.DIF", id, id, id),
			fmt.Sprintf("DIFFERENCES/OUTPUT=SUM$OUT:%s.CMP SUM$OUT:%s.OUT %s.B", id, id, id))
	}

	m := lines("record", 8)
	trip("R01", "a replace", m, edit(m, 3, 4, "changed"))
	trip("R02", "an insert at the start (a bare minus)", m, edit(m, 0, 0, "first", "second"))
	trip("R03", "an insert at the end", m, edit(m, 8, 8, "after"))
	trip("R04", "a delete", m, edit(m, 2, 5))
	trip("R05", "identical files (only the terminator)", m, m)
	trip("R06", "the master empty", nil, m)
	trip("R07", "the revision empty", m, nil)
	trip("R08", "both empty", nil, nil)
	trip("R09", "a revision record starting with a minus", m, edit(m, 3, 4, "-starts with a minus"))
	trip("R10", "a revision record that is a slash", m, edit(m, 3, 4, "/"))
	trip("R11", "a revision record starting with <", m, edit(m, 3, 4, "<starts with less-than"))
	trip("R12", "a revision record starting with a backslash", m, edit(m, 3, 4, "\\starts with a backslash"))
	trip("R13", "a revision record starting with a percent sign", m, edit(m, 3, 4, "%starts with a percent"))
	trip("R14", "a revision record like an edit command", m, edit(m, 3, 4, "-3,4"))
	trip("R15", "an empty revision record", m, edit(m, 3, 4, ""))
	trip("R16", "several sections", m, edit(edit(edit(m, 6, 7, "six"), 3, 3, "ins"), 0, 1))

	rng := rand.New(rand.NewPCG(117, 2026))
	pick := func(words []string) string { return words[rng.IntN(len(words))] }
	words := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}

	mutate := func(m []string, k int) []string {
		r := append([]string{}, m...)

		for ; k > 0; k-- {
			switch op := rng.IntN(5); {
			case op == 0 || len(r) == 0:
				at := rng.IntN(len(r) + 1)
				var run []string
				for n := 1 + rng.IntN(3); n > 0; n-- {
					run = append(run, pick(words))
				}

				r = edit(r, at, at, run...)
			case op == 1:
				at := rng.IntN(len(r))
				r = edit(r, at, min(len(r), at+1+rng.IntN(3)))
			case op == 2 || op == 3:
				at := rng.IntN(len(r))
				r = edit(r, at, at+1, pick(words))
			default:
				at := rng.IntN(len(r))
				end := min(len(r), at+1+rng.IntN(3))
				block := append([]string{}, r[at:end]...)
				r = edit(r, at, end)
				to := rng.IntN(len(r) + 1)
				r = edit(r, to, to, block...)
			}
		}

		return r
	}

	for i := 1; i <= 120; i++ {
		n := rng.IntN(16)
		if i > 100 {
			n = 30 + rng.IntN(31)
		}

		var a []string
		for j := 0; j < n; j++ {
			a = append(a, pick(words[:4+i%5]))
		}

		trip(fmt.Sprintf("T%03d", i), "generated", a, mutate(a, 1+rng.IntN(5)))
	}
}

// macro is A.3's tenth question: MACRO/UPDATE.
func macro() {
	const s = "MACRO"

	addFile("SUMA.MAR",
		"\t.TITLE\tSUMA\tSUMSLP probe module A",
		"\t.IDENT\t/V1.0/",
		"\t.PSECT\tDATA,NOEXE,WRT",
		"VALUE:\t.LONG\t1\t\t; a value",
		"\t.PSECT\tCODE,EXE,NOWRT",
		"\t.ENTRY\tSTART,^M<>",
		"\tMOVL\tVALUE,R0\t; return it",
		"\tRET",
		"\t.END\tSTART")
	addFile("SUMB.MAR",
		"\t.TITLE\tSUMB\tSUMSLP probe module B",
		"\t.PSECT\tCODE,EXE,NOWRT",
		"\t.ENTRY\tSUMB,^M<>",
		"\tMOVL\t#2,R0",
		"\tRET",
		"\t.END")
	addFile("SUMP.MAR",
		"\t.MACRO\tLOADIT\tN",
		"\tMOVL\t#N,R1",
		"\t.ENDM\tLOADIT")
	addFile("SUMA.UPD", "-4,4,/; M01/", "VALUE:\t.LONG\t7\t\t; changed by SUMA.UPD", "/")
	addFile("M02.UPD", "-7", "\tADDL2\t#10,R0\t; inserted by M02", "/")
	addFile("M03A.UPD", "-4,4", "VALUE:\t.LONG\t3", "/")
	addFile("M03B.UPD", "-7", "\tINCL\tR0", "/")
	addFile("M04.UPD", "-7", "\tCLRL\tR1\t; M04", "/")
	addFile("M05.UPD", "-3", "\t.MACRO\tCLEARIT", "\tCLRL\tR2", "\t.ENDM\tCLEARIT", "/")
	addFile("M06.UPD", "-6", "\tMOVL\t#1\t; M06, an operand missing", "/")
	addFile("M10.UPD", "-6,,/;M10 FIRST/", "\tNOP", "-7,7,/;M10 SECOND/", "\tMOVL\tVALUE,R0", "\tINCL\tR0", "/")
	addFile("M12.UPD", "-6", "\\", "\tNOP", "%", "\tNOP", "/")

	mac := func(id, title, cmd string) { addCase(s, id, title, cmd) }
	out := func(id string) string {
		return fmt.Sprintf("/LIST=SUM$OUT:%s.LIS/OBJECT=SUM$OUT:%s.OBJ", id, id)
	}

	mac("M01", "the default update file, SUMA.UPD", "MACRO"+out("M01")+" SUMA/UPDATE")
	mac("M02", "a named update file", "MACRO"+out("M02")+" SUMA/UPDATE=M02")
	mac("M03", "two update files merged", "MACRO"+out("M03")+" SUMA/UPDATE=(M03A,M03B)")
	mac("M04", "/UPDATE on the first of a list",
		"MACRO SUMA/UPDATE=M04/LIST=SUM$OUT:M04A.LIS/OBJECT=SUM$OUT:M04A.OBJ,SUMB/LIST=SUM$OUT:M04B.LIS/OBJECT=SUM$OUT:M04B.OBJ")
	mac("M05", "/UPDATE on the first of a concatenation", "MACRO"+out("M05")+" SUMP/UPDATE=M05+SUMA")
	mac("M06", "an update that makes an error", "MACRO"+out("M06")+" SUMA/UPDATE=M06")
	mac("M07", "a missing update file", "MACRO"+out("M07")+" SUMB/UPDATE")
	mac("M08", "/NOUPDATE after /UPDATE", "MACRO"+out("M08")+" SUMA/UPDATE=M02/NOUPDATE")
	mac("M09", "/UPDATE with no listing", "MACRO/NOLIST/OBJECT=SUM$OUT:M09.OBJ SUMA/UPDATE=M02")
	mac("M10", "audit trails in the listing", "MACRO"+out("M10")+" SUMA/UPDATE=M10")
	mac("M11", "/UPDATE after the verb", "MACRO/UPDATE=M02"+out("M11")+" SUMA")
	mac("M12", "the audit trail suppressed and enabled", "MACRO"+out("M12")+" SUMA/UPDATE=M12")
	mac("M13", "an update file of SUMSLP syntax errors", "MACRO"+out("M13")+" SUMA/UPDATE=Y08")
	mac("M14", "/UPDATE with /DEBUG", "MACRO/DEBUG"+out("M14")+" SUMA/UPDATE=M02")
	mac("M15", "no /UPDATE: the plain listing beside M02's", "MACRO"+out("M15")+" SUMA")
	mac("M16", "/UPDATE with a missing and a present file", "MACRO"+out("M16")+" SUMA/UPDATE=(NOSUCH,M02)")
}

// escape writes a record for cases.txt: backslash, a byte below a space or
// above a tilde as \xNN (but tab, \t), and a trailing blank as \x20, so the
// file is plain text an editor can't change.
func escape(r string) string {
	var b strings.Builder

	end := len(strings.TrimRight(r, " "))

	for i := 0; i < len(r); i++ {
		c := r[i]

		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < ' ' || c > '~' || (c == ' ' && i >= end):
			fmt.Fprintf(&b, `\x%02X`, c)
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// writeCases writes cases.txt, the probe's whole content: the files, then
// the cases, in the order the procedure runs them.
func writeCases() {
	var b strings.Builder

	b.WriteString(`# Phase 117's probe of SUMSLP (EDIT/SUM) and MACRO/UPDATE: every input
# file and every case. Written by testdata/sum117/gen.go; read by govax's
# oracle tests.
#
# file NAME [FORMAT]   an input file in [SUM117.IN] (or the directory its
#                      name gives); its records follow, one per "|" line,
#                      with \\, \t, and \xNN escapes, or as a rule
#                      "=lines PREFIX N" (records "PREFIX 1" to "PREFIX N");
#                      FORMAT is FIX n, VFC, or STMLF (variable when absent)
# case ID SECTION      a case; then
#   title TEXT         what it asks
#   pre COMMAND        a DCL line run before it
#   step COMMAND       a DCL line whose $STATUS the log shows
#   data LINE          a line after the step (its SYS$INPUT), escaped
#   post COMMAND       a DCL line run after it
# round 2              what follows is round 2's (sum117b.com)
#
# Outputs go to [SUM117.OUT] as ID.OUT (the edited file), ID.LIS (the
# listing), ID.DIF and ID.CMP (ROUNDTRIP), ID.OBJ (MACRO), ID.Kn (a file
# kept from the scratch directory).
#
`)

	writeEntries(&b, files[:round1Files], cases[:round1Cases])
	b.WriteString("round 2\n")
	writeEntries(&b, files[round1Files:], cases[round1Cases:])

	write("cases.txt", b.String())
}

// writeEntries writes files' and cases' entries for cases.txt.
func writeEntries(b *strings.Builder, files []file, cases []probeCase) {
	for _, f := range files {
		fmt.Fprintf(b, "file %s", f.Name)
		if f.Format != "" {
			fmt.Fprintf(b, " %s", f.Format)
		}

		b.WriteString("\n")

		if rule, ok := linesRule(f.Records); ok {
			fmt.Fprintf(b, "%s\n", rule)
			continue
		}

		for _, r := range f.Records {
			fmt.Fprintf(b, "|%s\n", escape(r))
		}
	}

	for _, c := range cases {
		fmt.Fprintf(b, "case %s %s\n", c.ID, c.Section)
		fmt.Fprintf(b, "  title %s\n", c.Title)

		for _, p := range c.Pre {
			fmt.Fprintf(b, "  pre %s\n", p)
		}

		for _, st := range c.Steps {
			fmt.Fprintf(b, "  step %s\n", st.Command)

			for _, d := range st.Data {
				fmt.Fprintf(b, "  data %s\n", escape(d))
			}
		}

		for _, p := range c.Post {
			fmt.Fprintf(b, "  post %s\n", p)
		}
	}

}

// linesRule describes a big file of "PREFIX 1" to "PREFIX n" as a rule.
func linesRule(records []string) (string, bool) {
	if len(records) <= 1000 {
		return "", false
	}

	prefix, _, _ := strings.Cut(records[0], " 1")
	for i, r := range records {
		if r != fmt.Sprintf("%s %d", prefix, i+1) {
			return "", false
		}
	}

	return fmt.Sprintf("=lines %s %d", prefix, len(records)), true
}

// hostName is the name exchange.cmd copies a file from: in/NNNN, its place
// in the file list.
func hostName(i int) string { return fmt.Sprintf("in/%04d", i+1) }

// writeInputs writes in/: each file as the host form exchange.cmd copies,
// text lines or, for a fixed file, its records' bytes end to end.
func writeInputs() {
	in := filepath.Join(dir, "in")
	if err := os.RemoveAll(in); err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(in, 0o755); err != nil {
		log.Fatal(err)
	}

	for i, f := range files {
		var b strings.Builder

		for _, r := range f.Records {
			b.WriteString(r)

			if !strings.HasPrefix(f.Format, "FIX") {
				if strings.ContainsAny(r, "\n") || strings.HasSuffix(r, "\r") {
					log.Fatalf("%s: a text record can't hold LF or end in CR", f.Name)
				}

				b.WriteString("\n")
			}
		}

		write(hostName(i), b.String())
	}
}

// procedureHead starts sum117.com.
const procedureHead = `$ ! SUM117.COM - Phase 117's probe of SUMSLP (EDIT/SUM) and MACRO/UPDATE.
$ ! Written by testdata/sum117/gen.go. Run it as SYSTEM, with the
$ ! exchange volume mounted, from its [SUM117] directory:
$ !
$ !     $ SET DEFAULT DUA1:[SUM117]
$ !     $ @SUM117/OUTPUT=SUM117.LOG
$ !
$ ! It makes its outputs in [SUM117.OUT] and [SUM117.SCR], and writes each
$ ! step's status to the log. It changes nothing outside [SUM117...].
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME"), " ", F$GETSYI("NODENAME")
$ WRITE SYS$OUTPUT F$TIME()
$ TOP = F$PARSE("[000000]",,,"DEVICE")
$ DEFINE SUM$TOP 'TOP'
$ DEFINE SUM$OUT SUM$TOP:[SUM117.OUT]
$ DEFINE SUM$SCR SUM$TOP:[SUM117.SCR]
$ SHOW LOGICAL SUM$TOP
$ IF F$SEARCH("SUM$TOP:[SUM117]OUT.DIR") .EQS. "" THEN CREATE/DIRECTORY SUM$TOP:[SUM117.OUT]
$ IF F$SEARCH("SUM$TOP:[SUM117]SCR.DIR") .EQS. "" THEN CREATE/DIRECTORY SUM$TOP:[SUM117.SCR]
$ DELETE SUM$TOP:[SUM117.OUT]*.*;*/NOLOG
$ DELETE SUM$TOP:[SUM117.SCR]*.*;*/NOLOG
$ SET DEFAULT SUM$TOP:[SUM117.IN]
$ SHOW DEFAULT
$ DIRECTORY/SIZE S10.SRC
$ TYPE S10.SRC
`

// procedureHeadB starts sum117b.com, round 2: the same names, round 1's
// outputs left where they are.
const procedureHeadB = `$ ! SUM117B.COM - Phase 117's probe of SUMSLP, round 2: what round 1 left
$ ! open. Written by testdata/sum117/gen.go. Run it as SYSTEM, with the
$ ! exchange volume mounted, from its [SUM117] directory:
$ !
$ !     $ SET DEFAULT DUA1:[SUM117]
$ !     $ @SUM117B/OUTPUT=SUM117B.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME"), " ", F$GETSYI("NODENAME")
$ WRITE SYS$OUTPUT F$TIME()
$ TOP = F$PARSE("[000000]",,,"DEVICE")
$ DEFINE SUM$TOP 'TOP'
$ DEFINE SUM$OUT SUM$TOP:[SUM117.OUT]
$ DEFINE SUM$SCR SUM$TOP:[SUM117.SCR]
$ DELETE SUM$TOP:[SUM117.SCR]*.*;*/NOLOG
$ SET DEFAULT SUM$TOP:[SUM117.IN]
$ SHOW DEFAULT
`

// procedureTail ends sum117.com: the outputs listed.
const procedureTail = `$ !
$ WRITE SYS$OUTPUT "######## THE OUTPUTS"
$ DIRECTORY/SIZE/DATE SUM$OUT:
$ SET DEFAULT SUM$TOP:[SUM117]
$ WRITE SYS$OUTPUT "SUM117 FINISHED"
$ SET NOVERIFY
$ EXIT
`

// statusLine writes a step's status: $STATUS is a string, so it's written
// as it is (Phase 116's round 1 printed the string's address).
const statusLine = "$ S = $STATUS\n$ WRITE SYS$OUTPUT \"STATUS \", S, \" \", F$MESSAGE(S)\n"

// writeProcedure writes sum117.com: each case under a heading, each step's
// status and F$MESSAGE's text of it.
func writeProcedure() {
	write("sum117.com", procedure(procedureHead, cases[:round1Cases], "SUM117"))
	write("sum117b.com", procedure(procedureHeadB, cases[round1Cases:], "SUM117B"))
}

// procedure writes a run of cases after head.
func procedure(head string, cases []probeCase, name string) string {
	var b strings.Builder

	b.WriteString(head)

	section := ""

	for _, c := range cases {
		if c.Section != section {
			section = c.Section
			fmt.Fprintf(&b, "$ !\n$ WRITE SYS$OUTPUT \"######## %s\"\n", section)
		}

		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"#### CASE %s: %s\"\n", c.ID, strings.ReplaceAll(c.Title, "\"", "\"\""))

		for _, p := range c.Pre {
			fmt.Fprintf(&b, "$ %s\n", dclLine(p))
		}

		for _, st := range c.Steps {
			fmt.Fprintf(&b, "$ %s\n", dclLine(st.Command))

			for _, d := range st.Data {
				if strings.HasPrefix(d, "$") {
					log.Fatalf("%s: a data line can't start with $", c.ID)
				}

				b.WriteString(d + "\n")
			}

			b.WriteString(statusLine)
		}

		for _, p := range c.Post {
			fmt.Fprintf(&b, "$ %s\n", dclLine(p))
		}
	}

	b.WriteString(strings.ReplaceAll(procedureTail, "SUM117 FINISHED", name+" FINISHED"))

	return b.String()
}

// dclLine breaks a command longer than 200 characters into continuation
// lines, as DCL reads them: before a slash or after a blank or comma. (DCL
// reads a procedure's records into a buffer of about 255 bytes; Phase 116's
// round 1 stopped at a longer one.)
func dclLine(command string) string {
	if len(command) <= 200 {
		return command
	}

	var out strings.Builder

	for len(command) > 150 {
		cut := strings.LastIndexAny(command[1:150], "/ ,") + 1
		if cut == 0 {
			log.Fatalf("no place to break %q", command)
		}

		if command[cut] == ' ' || command[cut] == ',' {
			cut++
		}

		out.WriteString(command[:cut] + "-\n")
		command = command[cut:]
	}

	out.WriteString(command)

	return out.String()
}

// writeTerminal writes sum117t.com, the cases at a terminal, run by hand.
func writeTerminal() {
	write("sum117t.com", `$ ! SUM117T.COM - Phase 117's probe: SUMSLP at a terminal.
$ ! Written by testdata/sum117/gen.go. Run it at a terminal, from the
$ ! exchange volume's [SUM117] directory, and keep everything the
$ ! terminal shows (README.md, "The terminal"):
$ !
$ !     $ SET DEFAULT DUA1:[SUM117]
$ !     $ @SUM117T
$ !
$ SET NOON
$ SET VERIFY
$ SET DEFAULT [.IN]
$ ! The listing and the output at the terminal.
$ EDIT/SUM/OUTPUT=TT:/LISTING=TT: S10.SRC/UPDATE=C01
$ SHOW SYMBOL $STATUS
$ EDIT/SUM/NOOUTPUT/LISTING=SYS$OUTPUT S10.SRC/UPDATE=G15A
$ ! No parameter: answer the prompt with S10.SRC/UPDATE=C01
$ EDIT/SUM/OUTPUT=TT:
$ SHOW SYMBOL $STATUS
$ ! The update typed at the terminal: type these lines, then Ctrl/Z
$ ! (and a second time without the "/", to see what Ctrl/Z alone does):
$ !     -3
$ !     typed after line 3
$ !     /
$ EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=SYS$COMMAND
$ SHOW SYMBOL $STATUS
$ EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=SYS$COMMAND
$ SHOW SYMBOL $STATUS
$ ! An error's messages at the terminal.
$ EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=Y08
$ SHOW SYMBOL $STATUS
$ SET DEFAULT [-]
$ SET NOVERIFY
`)
}

// writeExchange writes exchange.cmd, which builds the exchange volume.
func writeExchange() {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds Phase 117's probe exchange volume with govax.
! Written by testdata/sum117/gen.go. Run from the repository root, after
! gen.go has written in/:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/sum117-exchange.dsk" /DEVICE=RD53 SUM117
MOUNT/WRITE DUA5 "testdata/disks/sum117-exchange.dsk"
CREATE/DIRECTORY DUA5:[SUM117]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SUM117.IN]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SUM117.IN2]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SUM117.OUT]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SUM117.SCR]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
COPY "testdata/sum117/sum117.com"/HOST DUA5:[SUM117]SUM117.COM
COPY "testdata/sum117/sum117t.com"/HOST DUA5:[SUM117]SUM117T.COM
`)

	writeCopies(&b, 0, files[:round1Files])

	b.WriteString("DIRECTORY/SIZE DUA5:[SUM117...]\nDISMOUNT DUA5\n")

	write("exchange.cmd", b.String())
}

// writeCopies writes exchange.cmd's COPY of each file, the first being
// file number first.
func writeCopies(b *strings.Builder, first int, files []file) {
	for n, f := range files {
		i := first + n
		target := "DUA5:[SUM117.IN]" + f.Name
		if strings.HasPrefix(f.Name, "[") {
			target = "DUA5:" + f.Name
		}

		quals := "/HOST"

		switch {
		case f.Format == "VFC":
			quals += "/RECORD_FORMAT=VFC"
		case f.Format == "STMLF":
			quals += "/RECORD_FORMAT=STREAM_LF"
		case strings.HasPrefix(f.Format, "FIX "):
			quals += "/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=" + strings.TrimPrefix(f.Format, "FIX ")
		}

		fmt.Fprintf(b, "COPY \"%s/%s\"%s %s/QUIET\n", dir, hostName(i), quals, target)
	}

}

// writeCopyout writes copyout.cmd, which brings the run's results back.
func writeCopyout() {
	write("copyout.cmd", `! COPYOUT.CMD - copies Phase 117's probe results off sum117-exchange.dsk
! into testdata/sum117/vax (README.md). Written by testdata/sum117/gen.go.
! Run from the repository root after the VAX runs:
!
!     mkdir -p testdata/sum117/vax/out
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/copyout.cmd
!
MOUNT DUA5 "testdata/disks/sum117-exchange.dsk"
DIRECTORY/SIZE DUA5:[SUM117],[SUM117.OUT],[SUM117.SCR]
COPY DUA5:[SUM117]SUM117.LOG "testdata/sum117/vax/sum117.log"/HOST/QUIET
COPY DUA5:[SUM117.OUT]*.* "testdata/sum117/vax/out/"/HOST/QUIET
DISMOUNT DUA5
`)
}

// write writes one file under dir.
func write(name, text string) {
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

// writeRound2Scripts writes exchange2.cmd, which puts round 2's files and
// procedure on the exchange volume beside round 1's results, and
// copyout2.cmd, which brings round 2's log and every output back.
func writeRound2Scripts() {
	var b strings.Builder

	b.WriteString(`! EXCHANGE2.CMD - adds Phase 117's round 2 to sum117-exchange.dsk.
! Written by testdata/sum117/gen.go. Run from the repository root, after
! gen.go has written in/:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/sum117-exchange.dsk"
CREATE/DIRECTORY DUA5:[SUM117.IN.A_LONG_DIRECTORY_NAME_OF_THIRTY_NINE_CH]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
COPY "testdata/sum117/sum117b.com"/HOST DUA5:[SUM117]SUM117B.COM
`)
	writeCopies(&b, round1Files, files[round1Files:])
	b.WriteString("DIRECTORY/SIZE DUA5:[SUM117]\nDISMOUNT DUA5\n")

	write("exchange2.cmd", b.String())

	write("copyout2.cmd", `! COPYOUT2.CMD - copies Phase 117's round 2 results off sum117-exchange.dsk
! into testdata/sum117/vax (README.md). Written by testdata/sum117/gen.go.
! Run from the repository root after the VAX runs round 2:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/copyout2.cmd
!     for f in testdata/sum117/vax/out/*\;*; do mv "$f" "${f%;*}"; done
!
MOUNT DUA5 "testdata/disks/sum117-exchange.dsk"
DIRECTORY/SIZE DUA5:[SUM117],[SUM117.SCR]
COPY DUA5:[SUM117]SUM117B.LOG "testdata/sum117/vax/sum117b.log"/HOST/QUIET
COPY DUA5:[SUM117.OUT]*.* "testdata/sum117/vax/out/"/HOST/QUIET
DISMOUNT DUA5
`)
}

// Round 2: what round 1 left open (the phase doc's A.4).

// clash2 asks why G14's insert was lost: a delete, a replace, and an
// insert meeting at one line, in every order of their files, and the
// pieces two at a time.
func clash2() {
	const s = "CLASH2"

	for _, order := range [][]string{{"G14A", "G14C", "G14B"}, {"G14B", "G14A", "G14C"}, {"G14B", "G14C", "G14A"}, {"G14C", "G14A", "G14B"}} {
		id := "K" + string(order[0][3]) + string(order[1][3]) + string(order[2][3])
		sumCase(s, id, "G14's files in the order "+strings.Join(order, ","), "", "S10.SRC", order...)
	}

	sumCase(s, "K01", "a replace of 5 and a delete of 4 to 6", "", "S10.SRC", "G14A", "G14B")
	sumCase(s, "K01R", "a delete of 4 to 6 and a replace of 5", "", "S10.SRC", "G14B", "G14A")
	sumCase(s, "K02", "a replace of 5 and an insert after 5 (G04 again)", "", "S10.SRC", "G14A", "G14C")

	addFile("K03A.UPD", "-4,6", "K03A replaced 4 to 6", "/")
	addFile("K03B.UPD", "-5", "K03B after 5", "/")
	sumCase(s, "K03", "a replace of 4 to 6 and an insert after 5", "", "S10.SRC", "K03A", "K03B")
	sumCase(s, "K03R", "the same, the other order", "", "S10.SRC", "K03B", "K03A")

	addFile("K04B.UPD", "-5,5", "K04B replaced 5", "/")
	addFile("K04C.UPD", "-6", "K04C after 6", "/")
	sumCase(s, "K04", "a delete of 4 to 6, a replace of 5, an insert after 6", "", "S10.SRC", "G14B", "K04B", "K04C")

	addFile("K05A.UPD", "-3,7", "/")
	addFile("K05B.UPD", "-5", "K05B after 5", "/")
	addFile("K05C.UPD", "-4", "K05C after 4", "/")
	sumCase(s, "K05", "a delete of 3 to 7 and inserts after 5 and 4", "", "S10.SRC", "K05A", "K05B", "K05C")

	addFile("K06.UPD", "-4,6", "-5", "K06 after 5", "-5,5", "K06 replaced 5", "/")
	sumCase(s, "K06", "G14's three edits in one file", "", "S10.SRC", "K06")
	addFile("K07.UPD", "-4,6", "-5", "K07 after 5", "/")
	sumCase(s, "K07", "a delete and an insert inside it, in one file", "", "S10.SRC", "K07")
	addFile("K08.UPD", "-4,6", "-5,5", "K08 replaced 5", "/")
	sumCase(s, "K08", "a delete and a replace inside it, in one file", "", "S10.SRC", "K08")

	addFile("K09A.UPD", "-4,6", "/")
	addFile("K09B.UPD", "-5,5", "/")
	addFile("K09C.UPD", "-5", "K09C after 5", "/")
	sumCase(s, "K09", "a delete of 4 to 6, a delete of 5, an insert after 5", "", "S10.SRC", "K09A", "K09B", "K09C")
	sumCase(s, "K09R", "the same, the other order", "", "S10.SRC", "K09C", "K09B", "K09A")

	addFile("K10B.UPD", "-6,6", "K10B replaced 6", "/")
	sumCase(s, "K10", "a delete of 4 to 6, a replace of its last line, an insert after 5", "", "S10.SRC", "G14B", "K10B", "G14C")
}

// numbers asks what happens to a locator past 32767 (Y05's -99999 went
// after line 1).
func numbers() {
	const s = "NUMBERS"

	n := func(id, title string, upd ...string) {
		addFile(id+".UPD", upd...)
		sumCase(s, id, title, "", "S10.SRC", id)
	}

	n("N01", "32768", "-32768", "N01 text", "/")
	n("N02", "40000", "-40000", "N02 text", "/")
	n("N03", "65535", "-65535", "N03 text", "/")
	n("N04", "65536", "-65536", "N04 text", "/")
	n("N05", "65537", "-65537", "N05 text", "/")
	n("N06", "65541 (5 more than 65536)", "-65541", "N06 text", "/")
	n("N07", "100000", "-100000", "N07 text", "/")
	n("N08", "131077 (5 more than twice 65536)", "-131077", "N08 text", "/")
	n("N09", "99999 after an insert after 3", "-3", "N09 after 3", "-99999", "N09 text", "/")
	n("N10", "a range of 99999 to 99999", "-99999,99999", "N10 text", "/")
	n("N11", "a range of 5 to 65541", "-5,65541", "N11 text", "/")
	n("N12", "a range of 5 to 32768", "-5,32768", "N12 text", "/")
	n("N13", "six digits that are 5", "-000005", "N13 text", "/")
	n("N14", "99999 as the second of two inserts", "-7", "N14 after 7", "-99999", "N14 text", "/")
}

// pastEnd asks what an edit past the input's end does (Y13 read the input
// again, Y12 lost its insert).
func pastEnd() {
	const s = "PASTEND"

	e := func(id, title, src string, upd ...string) {
		addFile(id+".UPD", upd...)
		sumCase(s, id, title, "", src, id)
	}

	e("E01", "a replace of line 11 alone", "S10.SRC", "-11,11", "E01 text", "/")
	e("E02", "a replace of 10 to 11", "S10.SRC", "-10,11", "E02 text", "/")
	e("E03", "a replace of 10 to 12", "S10.SRC", "-10,12", "E03 text", "/")
	e("E04", "a delete of 11 to 12, no text", "S10.SRC", "-11,12", "/")
	e("E05", "a replace of 2 to 5 on three lines", "S03.SRC", "-2,5", "E05 text", "/")
	e("E06", "a replace of 2 to 9 on three lines", "S03.SRC", "-2,9", "E06 text", "/")
	e("E07", "a replace of 4 to 4 on three lines", "S03.SRC", "-4,4", "E07 text", "/")
	e("E08", "an insert after 4 on three lines", "S03.SRC", "-4", "E08 text", "/")
	e("E09", "a replace past the end, then an insert after it", "S03.SRC", "-2,5", "E09 replaced", "-7", "E09 after 7", "/")
	e("E10", "a replace past the end, then a replace of the reread", "S03.SRC", "-2,4", "E10 replaced", "-5,5", "E10 second", "/")
	e("E11", "an insert after 10, then after 11", "S10.SRC", "-10", "E11 after 10", "-11", "E11 after 11", "/")
	e("E12", "the last line deleted, then an insert after 11", "S10.SRC", "-10,10", "-11", "E12 after 11", "/")
	e("E13", "a replace of 1 to 4 on three lines", "S03.SRC", "-1,4", "E13 text", "/")
	e("E14", "an empty input, a replace of 1 to 1", "EMPTY.SRC", "-1,1", "E14 text", "/")
	e("E15", "an empty input, an insert after 1", "EMPTY.SRC", "-1", "E15 text", "/")
}

// lengths asks how long an update file's record and a source's record may
// be (B30's 300 bytes was RTB).
func lengths() {
	const s = "LENGTHS"

	long := func(n int) string {
		r := strings.Repeat("0123456789", n/10+1)[:n-4]
		return fmt.Sprintf("%04d", n) + r
	}

	for _, n := range []int{132, 133, 255, 256, 257, 511, 512, 513} {
		id := fmt.Sprintf("W%03d", n)
		addFile(id+".UPD", "-5", long(n), "/")
		sumCase(s, id, fmt.Sprintf("an update record of %d bytes", n), "", "S10.SRC", id)
	}

	for _, n := range []int{255, 256, 257, 512, 513, 1024, 2048, 4096, 8192, 16384, 32767} {
		id := fmt.Sprintf("X%05d", n)
		addFile(id+".SRC", "first", long(n), "last")
		sumCase(s, id, fmt.Sprintf("a source record of %d bytes", n), "", id+".SRC", "B05")
	}
}

// pages asks how messages and long inserts lay out across the listing's
// pages, and how a long name fills the heading.
func pages() {
	const s = "PAGES"

	addFile("P120.SRC", lines("page record", 120)...)
	addFile("P01A.UPD", "-55", "P01A after 55", "-56", "P01A after 56", "/")
	addFile("P01B.UPD", "-55", "P01B after 55", "-A", "P01B after a bad command", "/")
	sumCase(s, "P01", "messages at the end of page 1", "", "P120.SRC", "P01A", "P01B")

	var many []string
	for i := 1; i <= 70; i++ {
		many = append(many, fmt.Sprintf("P02 inserted %d", i))
	}

	addFile("P02.UPD", append(append([]string{"-50"}, many...), "/")...)
	sumCase(s, "P02", "70 records inserted across a page", "", "P120.SRC", "P02")

	addFile("P03.UPD", "-50,60", "/")
	sumCase(s, "P03", "a deletion across a page break", "", "P120.SRC", "P03")

	long := "[SUM117.IN.A_LONG_DIRECTORY_NAME_OF_THIRTY_NINE_CH]LONG_FILE_NAME_OF_THIRTY_NINE_CHARACTER.LONG_TYPE_OF_THIRTY_NINE_CHARACTERS_XYZ"
	addFile(long, s10...)
	addCase(s, "P04", "a long name in the heading",
		"EDIT/SUM/OUTPUT=SUM$OUT:P04.OUT/LISTING=SUM$OUT:P04.LIS "+long+"/UPDATE=[SUM117.IN]C01")
}

// slpVerb asks what EDIT/SLP is (round 1's C23: IVQUAL on /UPDATE).
func slpVerb() {
	const s = "SLPVERB"

	scratch := []string{"COPY S10.SRC SUM$SCR:", "SET DEFAULT SUM$SCR:"}
	end := []string{"DIRECTORY/SIZE SUM$SCR:", "SET DEFAULT SUM$TOP:[SUM117.IN]", "DELETE SUM$SCR:*.*;*/NOLOG"}

	addCase(s, "V01", "HELP EDIT /SLP", "HELP/NOPROMPT EDIT /SLP")
	cases = append(cases, probeCase{ID: "V02", Section: s, Title: "EDIT/SLP with no other qualifier, its input empty",
		Pre: scratch, Steps: []step{{Command: "EDIT/SLP S10.SRC"}}, Post: end})
	cases = append(cases, probeCase{ID: "V03", Section: s, Title: "EDIT/SLP/OUTPUT",
		Pre: scratch, Steps: []step{{Command: "EDIT/SLP/OUTPUT=SUM$OUT:V03.OUT S10.SRC"}}, Post: end})
	addCase(s, "V04", "EDIT/SLP/SUM", "EDIT/SLP/SUM/OUTPUT=SUM$OUT:V04.OUT S10.SRC/UPDATE=C01")
	addCase(s, "V05", "EDIT/SU, an abbreviation", "EDIT/SU/OUTPUT=SUM$OUT:V05.OUT S10.SRC/UPDATE=C01")
	addCase(s, "V06", "EDIT/S, an abbreviation", "EDIT/S/OUTPUT=SUM$OUT:V06.OUT S10.SRC/UPDATE=C01")
}

// dates asks which of the input's dates the listing's second heading
// line shows: a copy appended to after a wait, so its dates differ.
func dates() {
	const s = "DATES"

	cases = append(cases, probeCase{ID: "D01", Section: s, Title: "the listing's dates for an input revised after its creation",
		Pre: []string{
			"COPY S10.SRC SUM$SCR:D01.SRC",
			"WAIT 00:00:03",
			"OPEN/APPEND D01F SUM$SCR:D01.SRC",
			"WRITE D01F \"appended\"",
			"CLOSE D01F",
			"DIRECTORY/FULL SUM$SCR:D01.SRC",
		},
		Steps: []step{{Command: "EDIT/SUM/OUTPUT=SUM$OUT:D01.OUT/LISTING=SUM$OUT:D01.LIS SUM$SCR:D01.SRC/UPDATE=C01"}},
		Post:  []string{"DELETE SUM$SCR:*.*;*/NOLOG"}})
}
