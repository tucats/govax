package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// runFileProcedure runs lines as a command procedure on a console with
// a fresh volume mounted on DUA0: and the default directory DUA0:[000000]
// (unless onHost), and returns what it wrote and the console. typed is
// what the terminal (SYS$COMMAND) has to read. A failure the procedure
// has shown is not an error here.
func runFileProcedure(t *testing.T, onHost bool, typed string, lines ...string) (string, *Console) {
	t.Helper()

	d, c, buf := newCommandDispatcher(t)
	c.In = strings.NewReader(typed)

	if !onHost {
		mountFreshContainer(t, c, "DUA0")

		if err := c.SetDefault("DUA0:[000000]"); err != nil {
			t.Fatalf("SET DEFAULT: %v", err)
		}
	}

	proc := writeProcedure(t, t.TempDir(), "files.com", lines...)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@files: %v", err)
	}

	return buf.String(), c
}

// TestDCLFile_host: OPEN/WRITE makes a host file of lines, and OPEN/READ
// reads them back, a READ loop ending at /END_OF_FILE's label.
func TestDCLFile_host(t *testing.T) {
	t.Chdir(t.TempDir())

	out, _ := runFileProcedure(t, true, "",
		`$ OPEN/WRITE OUT NOTES`,
		`$ WRITE OUT "Line one: ", 12, " and ", 7 * 3`,
		`$ WRITE OUT "  Mixed Case  "`,
		`$ CLOSE OUT`,
		`$ OPEN IN NOTES`,
		`$ LOOP:`,
		`$ READ/END_OF_FILE=DONE IN RECORD`,
		`$ WRITE SYS$OUTPUT "[", RECORD, "]"`,
		`$ GOTO LOOP`,
		`$ DONE:`,
		`$ CLOSE IN`,
	)

	if want := "[Line one: 12 and 21]\n[  Mixed Case  ]\n"; out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}

	// A host file without a type gets .DAT, as a volume file does.
	data, err := os.ReadFile("NOTES.DAT")
	if err != nil {
		t.Fatal(err)
	}

	if want := "Line one: 12 and 21\n  Mixed Case  \n"; string(data) != want {
		t.Errorf("NOTES.DAT = %q, want %q", data, want)
	}
}

// TestDCLFile_volume: OPEN/WRITE makes the next version of a file, its
// logical name equated to the volume's physical device name, with the
// node's, until CLOSE; TYPE shows the records as lines
// (rms.TestOpenDCLFile checks the file's attributes).
func TestDCLFile_volume(t *testing.T) {
	out, _ := runFileProcedure(t, false, "",
		`$ OPEN/WRITE OUT RESULTS`,
		`$ SHOW LOGICAL OUT`,
		`$ WRITE OUT "first"`,
		`$ WRITE OUT "second"`,
		`$ CLOSE OUT`,
		`$ SHOW LOGICAL OUT`,
		`$ OPEN/WRITE OUT RESULTS.DAT`,
		`$ WRITE OUT "newer"`,
		`$ CLOSE OUT`,
		`$ TYPE RESULTS.DAT;1`,
		`$ TYPE RESULTS.DAT`,
	)

	want := `  "OUT" = "_GOVAX$DUA0" (LNM$PROCESS_TABLE)` + "\n" +
		"%SHOW-S-NOTRAN, no translation for logical name OUT\n" +
		"first\nsecond\nnewer\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}
}

// TestDCLFile_appendAndUpdate: /APPEND writes at the end of a file;
// /READ/WRITE reads from its start, WRITE/UPDATE replaces the record READ
// read with one of the same length, and WRITE adds a record only once
// READ has reached the end of the file (RMS$_NEF before that).
func TestDCLFile_appendAndUpdate(t *testing.T) {
	out, _ := runFileProcedure(t, false, "",
		`$ SET NOON`,
		`$ OPEN/WRITE OUT LIST`,
		`$ WRITE OUT "alpha"`,
		`$ CLOSE OUT`,
		`$ OPEN/APPEND OUT LIST`,
		`$ WRITE OUT "beta"`,
		`$ CLOSE OUT`,
		`$ OPEN/READ/WRITE IO LIST`,
		`$ READ IO R`,
		`$ WRITE/UPDATE IO "ALPHA"`,
		`$ WRITE IO "too soon"`,
		`$ READ IO R`,
		`$ WRITE/UPDATE IO "longer"`,
		`$ READ/END_OF_FILE=END IO R`,
		`$ END:`,
		`$ WRITE IO "gamma"`,
		`$ CLOSE IO`,
		`$ TYPE LIST.DAT`,
	)

	want := "%RMS-F-NEF, not positioned to EOF on $PUT (sequential organization only)\n" +
		"%RMS-F-RSZ, invalid record size\n" +
		"ALPHA\nbeta\ngamma\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}
}

// TestDCLFile_failures: each failure's message and $STATUS, as VMS 7.3
// showed them (testdata/dcl50's F3 and F4). OPEN's is DCL's OPENIN (or
// OPENOUT, for a new file) and RMS's status, which $STATUS holds marked
// shown; a record operation's is RMS's message, its status not marked; a
// name OPEN didn't define is CLI$_UNDFIL.
func TestDCLFile_failures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []string
		want    string
	}{
		{"missing file", []string{`$ OPEN IN NOSUCH`},
			"%DCL-E-OPENIN, error opening DUA0:[000000]NOSUCH.DAT; as input\n-RMS-E-FNF, file not found\n" +
				`  $STATUS == "%X10018292"`},
		{"missing directory", []string{`$ OPEN/WRITE OUT [NODIR]X`},
			"%DCL-E-OPENOUT, error opening DUA0:[NODIR]X.DAT; as output\n-RMS-E-DNF, directory not found\n" +
				"-SYSTEM-W-NOSUCHFILE, no such file\n" +
				`  $STATUS == "%X1001C04A"`},
		{"append to a missing file", []string{`$ OPEN/APPEND OUT NOSUCH`},
			"%DCL-E-OPENIN, error opening DUA0:[000000]NOSUCH.DAT; as input\n-RMS-E-FNF, file not found\n" +
				`  $STATUS == "%X10018292"`},
		{"wildcard", []string{`$ OPEN IN *.DAT`},
			"%DCL-E-OPENIN, error opening DUA0:[000000]*.DAT; as input\n-RMS-F-WLD, invalid wildcard operation\n" +
				`  $STATUS == "%X10018744"`},
		{"read past the end", []string{`$ OPEN/WRITE OUT EMPTY`, `$ CLOSE OUT`, `$ OPEN IN EMPTY`, `$ READ IN X`},
			"%RMS-E-EOF, end of file detected\n" + `  $STATUS == "%X0001827A"`},
		{"write a file opened to read", []string{`$ OPEN/WRITE OUT EMPTY`, `$ CLOSE OUT`, `$ OPEN IN EMPTY`, `$ WRITE IN "x"`},
			"%RMS-F-FAC, record operation not permitted by specified file access (FAC)\n" + `  $STATUS == "%X00018514"`},
		{"read a file opened to write", []string{`$ OPEN/WRITE OUT NEW`, `$ READ OUT X`},
			"%RMS-E-EOF, end of file detected\n" + `  $STATUS == "%X0001827A"`},
		{"write SYS$OUTPUT with a colon", []string{`$ WRITE SYS$OUTPUT: "x"`},
			"%DCL-W-UNDFIL, file has not been opened by DCL - check logical name\n" + `  $STATUS == "%X00038188"`},
		// The segment line is cut to 255 characters, as VMS 7.3's was.
		{"an item too long", []string{`$ S = F$FAO("!300*X")`, `$ WRITE SYS$OUTPUT S`},
			"%DCL-W-TKNOVF, command element is too long - shorten\n \\" + strings.Repeat("X", 253) + "\n" +
				`  $STATUS == "%X000382A0"`},
		{"read a name never opened", []string{`$ READ NOSUCH X`},
			"%DCL-W-UNDFIL, file has not been opened by DCL - check logical name\n" + `  $STATUS == "%X00038188"`},
		{"write a name never opened", []string{`$ WRITE NOSUCH "x"`},
			"%DCL-W-UNDFIL, file has not been opened by DCL - check logical name\n" + `  $STATUS == "%X00038188"`},
		{"close a name never opened", []string{`$ CLOSE NOSUCH`},
			"%DCL-W-UNDFIL, file has not been opened by DCL - check logical name\n" + `  $STATUS == "%X00038188"`},
		{"close/nolog", []string{`$ CLOSE/NOLOG NOSUCH`}, `  $STATUS == "%X00030001"`},
		{"write and append", []string{`$ OPEN/WRITE/APPEND OUT X`},
			"%DCL-W-CONFLICT, illegal combination of command elements - check documentation\n \\WRITE\\\n" +
				`  $STATUS == "%X00038258"`},
		{"no file", []string{`$ OPEN IN`},
			"%DCL-W-INSFPRM, missing command parameters - supply all required parameters\n" + `  $STATUS == "%X00038048"`},
		{"no data", []string{`$ WRITE SYS$OUTPUT`},
			"%DCL-W-INSFPRM, missing command parameters - supply all required parameters\n" + `  $STATUS == "%X00038048"`},
		{"bad symbol", []string{`$ READ SYS$INPUT 9X`},
			"%DCL-W-IVSYMB, invalid symbol name - start name with an alphabetic character\n \\9X\\\n" +
				`  $STATUS == "%X00038080"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := append(append([]string{`$ SET NOON`}, tc.command...), `$ SHOW SYMBOL $STATUS`)
			out, _ := runFileProcedure(t, false, "", lines...)

			if got := strings.TrimSuffix(out, "\n"); got != tc.want {
				t.Errorf("%s wrote:\n%s\nwant:\n%s", tc.name, got, tc.want)
			}
		})
	}
}

// TestDCLFile_branches: /ERROR and /END_OF_FILE go to their labels with
// $STATUS the failure's (OPEN's marked shown, as VMS 7.3's was) and no
// message, and the ON action isn't taken: without SET NOON, an error
// would otherwise end the procedure.
func TestDCLFile_branches(t *testing.T) {
	out, _ := runFileProcedure(t, false, "",
		`$ OPEN/ERROR=NOFILE IN NOSUCH`,
		`$ WRITE SYS$OUTPUT "not here"`,
		`$ NOFILE:`,
		`$ SHOW SYMBOL $STATUS`,
		`$ OPEN/WRITE OUT ONE`,
		`$ WRITE OUT "only"`,
		`$ CLOSE OUT`,
		`$ OPEN IN ONE`,
		`$ READ/END_OF_FILE=EOF1 IN X`,
		`$ READ/END_OF_FILE=EOF1 IN X`,
		`$ WRITE SYS$OUTPUT "not here"`,
		`$ EOF1:`,
		`$ SHOW SYMBOL $STATUS`,
		`$ READ/ERROR=ERR1 IN X`,
		`$ WRITE SYS$OUTPUT "not here"`,
		`$ ERR1:`,
		`$ SHOW SYMBOL $STATUS`,
		`$ CLOSE IN`,
		`$ CLOSE/ERROR=ERR2 IN`,
		`$ WRITE SYS$OUTPUT "not here"`,
		`$ ERR2:`,
		`$ SHOW SYMBOL $STATUS`,
		`$ WRITE SYS$OUTPUT "X is ", X`,
	)

	want := `  $STATUS == "%X10018292"` + "\n" +
		`  $STATUS == "%X0001827A"` + "\n" +
		`  $STATUS == "%X0001827A"` + "\n" +
		`  $STATUS == "%X00038188"` + "\n" +
		"X is only\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}
}

// TestDCLFile_statuses: what OPEN, WRITE, CLOSE, READ, and INQUIRE leave
// in $STATUS, as VMS 7.3 showed it (testdata/dcl50's F1, F10, and Q1):
// %X00000001, RMS$_NORMAL, RMS$_NORMAL, CLI$_NORMAL, %X00000001.
func TestDCLFile_statuses(t *testing.T) {
	out, _ := runFileProcedure(t, false, "",
		`$ OPEN/WRITE OUT A`,
		`$ SHOW SYMBOL $STATUS`,
		`$ WRITE OUT "x"`,
		`$ SHOW SYMBOL $STATUS`,
		`$ CLOSE OUT`,
		`$ SHOW SYMBOL $STATUS`,
		`$ OPEN IN A`,
		`$ READ IN X`,
		`$ SHOW SYMBOL $STATUS`,
		`$ CLOSE IN`,
		`$ INQUIRE Y`,
		`$ SHOW SYMBOL $STATUS`,
	)

	want := `  $STATUS == "%X00000001"` + "\n" + `  $STATUS == "%X00010001"` + "\n" + `  $STATUS == "%X00010001"` + "\n" +
		`  $STATUS == "%X00030001"` + "\n" + "Y: " + `  $STATUS == "%X00000001"` + "\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}
}

// TestDCLFile_processPermanent: SYS$OUTPUT, and a logical name for it,
// need no OPEN; READ SYS$INPUT reads the procedure's next record,
// whatever it is, and is at its end of file at the procedure's end; READ
// SYS$COMMAND reads the terminal, prompting.
func TestDCLFile_processPermanent(t *testing.T) {
	out, c := runFileProcedure(t, false, "typed Line\n",
		`$ DEFINE OUT SYS$OUTPUT`,
		`$ WRITE OUT "through OUT"`,
		`$ READ SYS$INPUT A`,
		`$ WRITE SYS$OUTPUT "this is a record"`,
		`$ READ SYS$INPUT B`,
		`data, as it is`,
		`$ READ/PROMPT="Say: " SYS$COMMAND C`,
		`$ READ/END_OF_FILE=AT_END SYS$COMMAND D`,
		`$ AT_END:`,
		`$ SHOW SYMBOL A`,
		`$ SHOW SYMBOL B`,
		`$ SHOW SYMBOL C`,
		`$ READ/END_OF_FILE=DONE SYS$INPUT E`,
	)

	want := "through OUT\n" +
		"Say: Data:\t" +
		`  A = "$ WRITE SYS$OUTPUT "this is a record""` + "\n" +
		`  B = "data, as it is"` + "\n" +
		`  C = "typed Line"` + "\n" +
		"%DCL-W-USGOTO, target of GOTO not found - check spelling and presence of label\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}

	// The last READ reached the procedure's end of file, and went to a
	// label that isn't there.
	if got := c.Status() &^ stsInhibitMsg; got != conditionValue(vmserrors.New(vmserrors.CLI_USGOTO)) {
		t.Errorf("$STATUS %08X, want CLI$_USGOTO's", got)
	}
}

// TestDCLFile_inquire: INQUIRE prompts at the terminal, with ": " unless
// /NOPUNCTUATION, and treats the reply as a string assignment's value,
// after substituting its symbols.
func TestDCLFile_inquire(t *testing.T) {
	out, c := runFileProcedure(t, false, "hello  'X'   \"Mixed  Case\"\nyes\nraw\n",
		`$ X = "there"`,
		`$ INQUIRE ANSWER "Your answer"`,
		`$ INQUIRE/GLOBAL FLAG`,
		`$ INQUIRE/NOPUNCTUATION LAST "> "`,
		`$ SHOW SYMBOL ANSWER`,
		`$ SHOW SYMBOL FLAG`,
		`$ SHOW SYMBOL LAST`,
	)

	want := "Your answer: FLAG: > " +
		`  ANSWER = "HELLO THERE Mixed  Case"` + "\n" +
		`  FLAG == "YES"` + "\n" +
		`  LAST = "RAW"` + "\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}

	if _, ok := c.DCLSymbol("ANSWER"); ok {
		t.Error("ANSWER, a local symbol of the procedure, outlived it")
	}
}

// TestDCLFile_staysOpen: a file a procedure opens stays open after it
// ends, and EndSession closes what is left, its records written.
func TestDCLFile_staysOpen(t *testing.T) {
	dir := t.TempDir()
	inner := writeProcedure(t, dir, "inner.com", `$ OPEN/WRITE KEEP KEPT`, `$ WRITE KEEP "from inner"`)

	_, c := runFileProcedure(t, false, "",
		`$ @`+inner,
		`$ WRITE KEEP "from outer"`,
	)

	if _, open := c.openFile("KEEP"); !open {
		t.Fatal("KEEP was closed when inner.com ended")
	}

	c.EndSession()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]KEPT.DAT"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(joinRecords(records)); got != "from inner\nfrom outer\n" {
		t.Errorf("KEPT.DAT holds %q", got)
	}
}

// TestDCLFile_logicalName: the logical name is the way to the file: TYPE
// of it types the file from where READ has got to, using its records up;
// once the name is deassigned, READ and CLOSE can't reach the file (VMS
// 7.3, testdata/dcl50's F2 and F11).
func TestDCLFile_logicalName(t *testing.T) {
	out, c := runFileProcedure(t, false, "",
		`$ SET NOON`,
		`$ OPEN/WRITE OUT THREE`,
		`$ WRITE OUT "one"`,
		`$ WRITE OUT "two"`,
		`$ WRITE OUT "three"`,
		`$ CLOSE OUT`,
		`$ OPEN IN THREE`,
		`$ READ IN X`,
		`$ TYPE IN`,
		`$ READ IN X`,
		`$ DEASSIGN IN`,
		`$ READ IN X`,
		`$ CLOSE IN`,
	)

	want := "two\nthree\n" +
		"%RMS-E-EOF, end of file detected\n" +
		"%DCL-W-UNDFIL, file has not been opened by DCL - check logical name\n" +
		"%DCL-W-UNDFIL, file has not been opened by DCL - check logical name\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}

	if len(c.dclOrphans) != 1 {
		t.Errorf("%d files out of reach, want 1 (still open)", len(c.dclOrphans))
	}
}

// TestDCLFile_terminal: under /OUTPUT, INQUIRE's prompt and WRITE
// SYS$ERROR and SYS$COMMAND go to the terminal, and procedure
// verification shows INQUIRE's reply, substituted, in the log (VMS 7.3,
// testdata/dcl50's Q11).
func TestDCLFile_terminal(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	c.In = strings.NewReader("it is 'X'\n")

	dir := t.TempDir()
	log := filepath.Join(dir, "inner.log")
	inner := writeProcedure(t, dir, "inner.com",
		`$ X = "here"`,
		`$ SET VERIFY`,
		`$ INQUIRE ANSWER "Prompt"`,
		`$ WRITE SYS$OUTPUT "to the log"`,
		`$ WRITE SYS$ERROR "to SYS$ERROR"`,
		`$ WRITE SYS$COMMAND "to SYS$COMMAND"`,
		`$ SET NOVERIFY`,
	)

	if err := d.Dispatch("@" + inner + `/OUTPUT="` + log + `"`); err != nil {
		t.Fatal(err)
	}

	if want := "Prompt: to SYS$ERROR\nto SYS$COMMAND\n"; buf.String() != want {
		t.Errorf("the terminal showed %q, want %q", buf.String(), want)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	want := `$ INQUIRE ANSWER "Prompt"` + "\nit is here\n" + `$ WRITE SYS$OUTPUT "to the log"` + "\nto the log\n" +
		`$ WRITE SYS$ERROR "to SYS$ERROR"` + "\n" + `$ WRITE SYS$COMMAND "to SYS$COMMAND"` + "\n" + "$ SET NOVERIFY\n"
	if string(data) != want {
		t.Errorf("the log holds:\n%s\nwant:\n%s", data, want)
	}
}

// joinRecords is records as lines of text.
func joinRecords(records [][]byte) []byte {
	var out []byte

	for _, r := range records {
		out = append(append(out, r...), '\n')
	}

	return out
}

// TestDCLFile_sameName: an OPEN of a logical name OPEN already has a file
// for is passed over, with no message, and READ goes on reading the first
// file (the DCL Dictionary).
func TestDCLFile_sameName(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, "FIRST.DAT", "from first\n")
	writeFile(t, "SECOND.DAT", "from second\n")

	out, _ := runFileProcedure(t, true, "",
		`$ OPEN IN FIRST`,
		`$ OPEN IN SECOND`,
		`$ READ IN X`,
		`$ WRITE SYS$OUTPUT X`,
	)

	if want := "from first\n"; out != want {
		t.Errorf("the procedure wrote %q, want %q", out, want)
	}
}

// TestDCLFile_sharing: DCL's files are among a volume file's RMS openers.
// A file OPEN/WRITE has open (without /SHARE) keeps every other opener
// out: a reader, a /SHARE reader, and TYPE (VMS 7.3, testdata/dcl50's
// F9).
func TestDCLFile_sharing(t *testing.T) {
	out, _ := runFileProcedure(t, false, "",
		`$ SET NOON`,
		`$ OPEN/WRITE OUT LOG`,
		`$ WRITE OUT "written"`,
		`$ OPEN IN LOG`,
		`$ OPEN/SHARE IN LOG`,
		`$ TYPE LOG.DAT`,
		`$ SHOW SYMBOL $STATUS`,
		`$ CLOSE OUT`,
		`$ TYPE LOG.DAT`,
	)

	locked := "%DCL-E-OPENIN, error opening DUA0:[000000]LOG.DAT; as input\n" +
		"-RMS-E-FLK, file currently locked by another user\n"
	want := locked + locked +
		"%TYPE-W-OPENIN, error opening DUA0:[000000]LOG.DAT;1 as input\n" +
		"-RMS-E-FLK, file currently locked by another user\n" +
		`  $STATUS == "%X10951098"` + "\n" +
		"written\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}
}

// TestDCLFile_ownCommand: OPEN is one of DCL's own commands, so a data
// line after it is read as a command, not skipped.
func TestDCLFile_ownCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, filepath.Join(".", "A.DAT"), "x\n")

	out, _ := runFileProcedure(t, true, "",
		`$ OPEN IN A`,
		`WRITE SYS$OUTPUT "read as a command"`,
	)

	if want := "read as a command\n"; out != want {
		t.Errorf("the procedure wrote %q, want %q", out, want)
	}
}

// TestSplitDCLList: WRITE's items are split at commas outside quotes and
// parentheses.
func TestSplitDCLList(t *testing.T) {
	got := splitDCLList(`"a,b", F$EXTRACT(0, 1, X) , 3`)
	want := []string{`"a,b"`, `F$EXTRACT(0, 1, X)`, `3`}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("splitDCLList = %q, want %q", got, want)
	}
}
