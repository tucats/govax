package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Tests of the lexical functions (dcllexical.go and the dcllex*.go files),
// from the User's Manual's chapter 15 examples where it has them.

// lexCase is one lexical function call and the value it should have:
// integer says whether that's an integer, or the error it should be
// instead: code, a govax status, or vms, a VMS condition value (a system
// service's status).
type lexCase struct {
	text    string
	integer bool
	want    string
	code    uint32
	vms     string
}

// checkLexical evaluates each case's expression on c's symbols, checking
// its value or its error.
func checkLexical(t *testing.T, c *Console, cases []lexCase) {
	t.Helper()

	for _, tc := range cases {
		v, err := evaluateDCLExpression(upcaseOutsideQuotes(tc.text), &c.dclSymbols, c)

		switch {
		case tc.vms != "":
			if want := vmsdef.Symbols[tc.vms]; err == nil || conditionValue(err) != want {
				t.Errorf("%s: %v (value %q), want %s", tc.text, err, v.String(), tc.vms)
			}
		case tc.code != 0:
			if !hasStatus(err, tc.code) {
				t.Errorf("%s: %v (value %q), want %v", tc.text, err, v.String(), vmserrors.New(tc.code))
			}
		case err != nil:
			t.Errorf("%s: %v", tc.text, err)
		case v.integer != tc.integer || v.String() != tc.want:
			t.Errorf("%s = %q (integer %v), want %q (integer %v)", tc.text, v.String(), v.integer, tc.want, tc.integer)
		}
	}
}

// lexConsole is a console with its process, and symbols the cases use.
func lexConsole(t *testing.T) *Console {
	t.Helper()

	c, _ := newTestConsole(t)

	for _, a := range []struct{ name, op, text string }{
		{"PROT", "=", `"SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD"`},
		{"FILE", "=", `"LOGIN.COM;3"`},
		{"UIC", "=", `"[WRITERS,SMITH]"`},
		{"NUM", "=", "12"},
		{"NUMSTR", "=", `"12"`},
		{"HEXSTR", "=", `"-%X1F"`},
		{"WORD", "=", `"twelve"`},
	} {
		if err := c.assignSymbol(a.name, a.op, a.text); err != nil {
			t.Fatalf("%s %s %s: %v", a.name, a.op, a.text, err)
		}
	}

	return c
}

// TestLexicalStrings: F$EXTRACT, F$LOCATE, F$ELEMENT, F$CVSI, F$CVUI, and
// F$TYPE, with the User's Manual's examples (15.6, 15.7).
func TestLexicalStrings(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		// 15.6.1: a semicolon's offset, or the length when there's none.
		{text: `F$LOCATE(";", FILE)`, integer: true, want: "9"},
		{text: `F$LOCATE(";", "LOGIN.COM")`, integer: true, want: "9"},
		{text: `F$LOCATE("", "ABC")`, integer: true, want: "0"},

		// 15.6.2: the group of a UIC, and the fourth element of a
		// protection.
		{text: `F$EXTRACT(1, F$LOCATE(",", UIC) - 1, UIC)`, want: "WRITERS"},
		{text: `F$EXTRACT(4, 100, "abcdef")`, want: "ef"},
		{text: `F$EXTRACT(10, 2, "abc")`, want: ""},
		{text: `F$EXTRACT(-1, 2, "abc")`, code: vmserrors.CLI_INVRANGE},
		{text: `F$ELEMENT(3, ",", PROT)`, want: " WORLD"},
		{text: `F$ELEMENT(0, "/", "A/B/C")`, want: "A"},
		{text: `F$ELEMENT(5, "/", "A/B/C")`, want: "/"},
		{text: `F$ELEMENT(1, "/", "A//C")`, want: ""},
		{text: `F$ELEMENT(0, "/", "")`, want: ""},
		{text: `F$ELEMENT(0, "//", "A")`, code: vmserrors.CLI_IVVALU},
		{text: `F$ELEMENT(-1, "/", "A")`, code: vmserrors.CLI_INVRANGE},

		// 15.7: bit fields, signed and unsigned, from the string's
		// first bit.
		{text: `F$CVUI(0, 8, "A")`, integer: true, want: "65"},
		{text: `F$CVUI(0, 16, "AB")`, integer: true, want: "16961"},
		{text: `F$CVUI(4, 8, "AB")`, integer: true, want: "36"},
		{text: `F$CVSI(0, 8, "` + "\xff" + `")`, integer: true, want: "-1"},
		{text: `F$CVUI(0, 8, "` + "\xff" + `")`, integer: true, want: "255"},
		{text: `F$CVSI(0, 3, "` + "\x04" + `")`, integer: true, want: "-4"},
		{text: `F$CVUI(0, 9, "A")`, code: vmserrors.CLI_INVRANGE},
		{text: `F$CVUI(0, 33, "ABCDE")`, code: vmserrors.CLI_INVRANGE},

		// 15.7.1 and 15.7.3: a symbol's type, "" for none; a string
		// that is an integer is INTEGER.
		{text: "F$TYPE(NUM)", want: "INTEGER"},
		{text: "F$TYPE(NUMSTR)", want: "INTEGER"},
		{text: "F$TYPE(HEXSTR)", want: "INTEGER"},
		{text: "F$TYPE(WORD)", want: "STRING"},
		{text: "F$TYPE(NOSUCH)", want: ""},
		{text: "F$TYPE(nu)", want: ""},
		{text: `F$TYPE("NUM")`, code: vmserrors.CLI_IVSYMB},

		// 15.7.2's example: F$INTEGER of an expression's text.
		{text: `F$INTEGER("9 + 7")`, integer: true, want: "0"},
		{text: `F$INTEGER(9 + 7)`, integer: true, want: "16"},
	})
}

// TestLexicalEdit: F$EDIT's edits, which leave quoted text alone.
func TestLexicalEdit(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		{text: `F$EDIT("  a  b	c  ", "TRIM")`, want: "a  b\tc"},
		{text: `F$EDIT("  a  b	c  ", "COMPRESS")`, want: " a b c "},
		{text: `F$EDIT("  a  b	c  ", "COLLAPSE")`, want: "abc"},
		{text: `F$EDIT("  a  b	c  ", "TRIM,COMPRESS")`, want: "a b c"},
		{text: `F$EDIT("  a  b	c  ", "compress, trim")`, want: "a b c"},
		{text: `F$EDIT("abc", "UPCASE")`, want: "ABC"},
		{text: `F$EDIT("ABC", "LOWERCASE")`, want: "abc"},
		{text: `F$EDIT("copy a b ! the files", "UNCOMMENT")`, want: "copy a b "},
		{text: `F$EDIT("copy a b ! the files", "UNCOMMENT,TRIM")`, want: "copy a b"},
		{text: `F$EDIT("a ""x  y"" b", "COLLAPSE,UPCASE")`, want: `A"x  y"B`},
		{text: `F$EDIT("say ""!"" ! now", "UNCOMMENT")`, want: `say "!" `},
		{text: `F$EDIT(" ""a"" ", "TRIM")`, want: `"a"`},
		{text: `F$EDIT("abc", "")`, want: "abc"},
		{text: `F$EDIT("abc", "SHOUT")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalFAO: F$FAO, with 15.6.3's columns and $FAO's directives.
func TestLexicalFAO(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		{text: `F$FAO("!16AS !12AS", "MARCHESAND", "2CA0049C")`, want: "MARCHESAND       2CA0049C    "},
		{text: `F$FAO("Found !UL file!%S", 3)`, want: "Found 3 files"},
		{text: `F$FAO("Found !UL file!%S", 1)`, want: "Found 1 file"},
		{text: `F$FAO("!5ZL|!5UL|!XL|!OB", 42, 42, 255, 8)`, want: "00042|   42|000000FF|010"},
		{text: `F$FAO("!SL", -5)`, want: "-5"},
		{text: `F$FAO("!AS", 12)`, want: "12"},
		{text: `F$FAO("!UL", "7")`, want: "7"},
		{text: `F$FAO("!3*-!AS", "x")`, want: "---x"},
		{text: `F$FAO("!AD", 2, "abcdef")`, want: "ab"},
		{text: `F$FAO("!10<!AS!>|", "ab")`, want: "ab        |"},
		{text: `F$FAO("!AS !-!AS", "ab")`, want: "ab ab"},
		{text: `F$FAO("a!!b")`, want: "a!b"},
		{text: `F$FAO("!AS")`, vms: "SS$_ACCVIO"},
		{text: `F$FAO("!QQ", 1)`, vms: "SS$_BADPARAM"},
	})
}

// TestLexicalEnvironment: F$ENVIRONMENT's items, at the terminal and in a
// procedure, and F$VERIFY.
func TestLexicalEnvironment(t *testing.T) {
	c := lexConsole(t)

	checkLexical(t, c, []lexCase{
		{text: `F$ENVIRONMENT("DEPTH")`, integer: true, want: "0"},
		{text: `F$ENVIRONMENT("depth")`, integer: true, want: "0"},
		{text: `F$ENVIRONMENT("MAX_DEPTH")`, integer: true, want: "32"},
		{text: `F$ENVIRONMENT("PROCEDURE")`, want: ""},
		{text: `F$ENVIRONMENT("INTERACTIVE")`, want: "TRUE"},
		{text: `F$ENVIRONMENT("CAPTIVE")`, want: "FALSE"},
		{text: `F$ENVIRONMENT("ON_SEVERITY")`, want: "NONE"},
		{text: `F$ENVIRONMENT("PROTECTION")`, want: "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD"},
		{text: `F$ENVIRONMENT("PROMPT")`, want: c.Prompt()},
		{text: `F$ENVIRONMENT("DEFAULT")`, want: c.ContainerSession.DefaultString()},
		{text: `F$ENVIRONMENT("VERIFY_PROCEDURE")`, want: "FALSE"},
		{text: `F$ENVIRONMENT("DEP")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$MODE()`, want: "INTERACTIVE"},
		{text: `F$DIRECTORY()`, want: "[000000]"},

		// 15.2.1: F$VERIFY returns the old setting and sets both, or
		// each.
		{text: `F$VERIFY(1)`, integer: true, want: "0"},
		{text: `F$ENVIRONMENT("VERIFY_IMAGE")`, want: "TRUE"},
		{text: `F$VERIFY(0, 1)`, integer: true, want: "1"},
		{text: `F$ENVIRONMENT("VERIFY_PROCEDURE")`, want: "FALSE"},
		{text: `F$ENVIRONMENT("VERIFY_IMAGE")`, want: "TRUE"},
		{text: `F$VERIFY(, 0)`, integer: true, want: "0"},
		{text: `F$ENVIRONMENT("VERIFY_IMAGE")`, want: "FALSE"},
		{text: `F$VERIFY()`, integer: true, want: "0"},
	})

	out := runFlow(t,
		`$ D = F$ENVIRONMENT("DEPTH")`,
		`$ S1 = F$ENVIRONMENT("ON_SEVERITY")`,
		"$ ON WARNING THEN CONTINUE",
		`$ S2 = F$ENVIRONMENT("ON_SEVERITY")`,
		"$ SET NOON",
		`$ S3 = F$ENVIRONMENT("ON_SEVERITY")`,
		`$ P = F$ENVIRONMENT("PROCEDURE")`,
		`$ PRINT "''D' ''S1' ''S2' ''S3' ''F$EXTRACT(F$LENGTH(P) - 10, 10, P)'"`,
		"$ CALL SUB",
		"$ EXIT",
		"$ SUB: SUBROUTINE",
		`$ PRINT "''F$ENVIRONMENT("DEPTH")'"`,
		"$ ENDSUBROUTINE")

	if out != "1 ERROR WARNING NONE status.com\n2\n" {
		t.Errorf("procedure output %q", out)
	}
}

// TestLexicalProcess: F$PROCESS, F$USER, F$PID, F$GETJPI, F$GETSYI,
// F$PRIVILEGE, and F$SETPRV, against the console's process.
func TestLexicalProcess(t *testing.T) {
	c := lexConsole(t)
	env := c.RTL
	pid := fmt.Sprintf("%08X", env.Process.PID)

	if err := c.assignSymbol("CTX", "=", `""`); err != nil {
		t.Fatal(err)
	}

	checkLexical(t, c, []lexCase{
		{text: "F$PROCESS()", want: env.Process.Name},
		{text: "F$USER()", want: env.IdentifierText(env.Process.UIC)},
		{text: `F$GETJPI("", "PID")`, want: pid},
		{text: `F$GETJPI("` + pid + `", "PRCNAM")`, want: env.Process.Name},
		{text: `F$GETJPI("", "UIC")`, want: env.IdentifierText(env.Process.UIC)},
		{text: `F$GETJPI("", "PRIB")`, integer: true, want: fmt.Sprint(env.Process.BasePriority)},
		{text: `F$GETJPI("", "MODE")`, want: "INTERACTIVE"},
		{text: `F$GETJPI("", "JOBTYPE")`, want: "LOCAL"},
		{text: `F$GETJPI("", "STATE")`, want: env.ProcessState(env)},
		{text: `F$GETJPI("", "USERNAME")`, want: fmt.Sprintf("%-12s", env.Process.Username)},
		{text: `F$GETJPI("", "PRCNAMX")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$GETJPI("7FFF0001", "PID")`, vms: "SS$_NONEXPR"},
		{text: `F$GETJPI("XYZ", "PID")`, vms: "SS$_NONEXPR"},

		// 15.3.3: F$PID through the processes, then "".
		{text: "F$PID(CTX)", want: pid},
		{text: "F$PID(CTX)", want: ""},
		{text: "F$PID(CTX)", want: pid},
		{text: "F$PID(NOSUCH)", code: vmserrors.CLI_UNDSYM},

		// 15.3.1: the node name, and the version.
		{text: `F$GETSYI("NODENAME")`, want: env.NodeName},
		{text: `F$GETSYI("VERSION")`, want: "V7.3    "},
		{text: `F$GETSYI("CLUSTER_MEMBER")`, want: "FALSE"},
		{text: `F$GETSYI("NODENAME", "` + strings.ToLower(env.NodeName) + `")`, want: env.NodeName},
		{text: `F$GETSYI("NODENAME", "ELSEWHERE")`, vms: "SS$_NOSUCHNODE"},

		// 15.2: privileges, tested and changed.
		{text: `F$PRIVILEGE("CMKRNL,TMPMBX")`, want: "TRUE"},
		{text: `F$PRIVILEGE("NOCMKRNL")`, want: "FALSE"},
		{text: `F$PRIVILEGE("NOSUCH")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$SETPRV("NOCMKRNL,TMPMBX")`, want: "CMKRNL,TMPMBX"},
		{text: `F$PRIVILEGE("NOCMKRNL,TMPMBX")`, want: "TRUE"},
		{text: `F$GETJPI("", "CURPRIV")`, want: strings.TrimPrefix(privilegeList(env.Process.CurrentPrivileges), "CMKRNL,")},
		{text: `F$SETPRV("CMKRNL")`, want: "NOCMKRNL"},
		{text: `F$PRIVILEGE("CMKRNL")`, want: "TRUE"},
	})

	if s := privilegeList(env.Process.CurrentPrivileges); !strings.HasPrefix(s, "CMKRNL,CMEXEC,") {
		t.Errorf("privileges %q, want CMKRNL, CMEXEC, ... in bit order", s)
	}
}

// TestLexicalGetDVI: F$GETDVI of a device there is, and EXISTS of one
// there isn't.
func TestLexicalGetDVI(t *testing.T) {
	c := lexConsole(t)
	mountFreshContainer(t, c, "DUA0")

	checkLexical(t, c, []lexCase{
		{text: `F$GETDVI("DUA0:", "EXISTS")`, want: "TRUE"},
		{text: `F$GETDVI("DUA9:", "EXISTS")`, want: "FALSE"},
		{text: `F$GETDVI("DUA0:", "MNT")`, want: "TRUE"},
		{text: `F$GETDVI("DUA0:", "VOLNAM")`, want: "DIRVOL"},
		{text: `F$GETDVI("DUA0:", "UNIT")`, integer: true, want: "0"},
		{text: `F$GETDVI("DUA9:", "UNIT")`, vms: "SS$_NOSUCHDEV"},
		{text: `F$GETDVI("DUA0:", "NOSUCH")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalLogicalNames: F$TRNLNM's arguments and items (15.5), and
// F$LOGICAL.
func TestLexicalLogicalNames(t *testing.T) {
	c := lexConsole(t)

	eqv := []lnm.Equivalence{{Value: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"}, {Value: "DISK1:", Attrs: lnm.AttrConcealed}}
	if _, err := c.Logicals.Define(lnm.ProcessTableName, "NAMES", lnm.Supervisor, 0, eqv); err != nil {
		t.Fatal(err)
	}

	checkLexical(t, c, []lexCase{
		{text: `F$TRNLNM("NAMES")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$TRNLNM("names")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$TRNLNM("names",,,,"CASE_SENSITIVE")`, want: ""},
		{text: `F$TRNLNM("NAMES",, 1)`, want: "DISK1:"},
		{text: `F$TRNLNM("NAMES",, 2)`, want: ""},
		{text: `F$TRNLNM("NAMES",,,,, "MAX_INDEX")`, integer: true, want: "1"},
		{text: `F$TRNLNM("NAMES",, 1,,, "CONCEALED")`, want: "TRUE"},
		{text: `F$TRNLNM("NAMES",,,,, "CONCEALED")`, want: "FALSE"},
		{text: `F$TRNLNM("NAMES",,,,, "LENGTH")`, integer: true, want: "30"},
		{text: `F$TRNLNM("NAMES",,,,, "ACCESS_MODE")`, want: "SUPERVISOR"},
		{text: `F$TRNLNM("NAMES",,,,, "TABLE_NAME")`, want: "LNM$PROCESS_TABLE"},
		{text: `F$TRNLNM("NAMES",,,,, "TABLE")`, want: "FALSE"},
		{text: `F$TRNLNM("NAMES", "LNM$SYSTEM")`, want: ""},
		{text: `F$TRNLNM("NAMES", "LNM$PROCESS")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$TRNLNM("NAMES",,, "EXECUTIVE")`, want: ""},
		{text: `F$TRNLNM("NOSUCH")`, want: ""},
		{text: `F$TRNLNM("NAMES", "NO$SUCH_TABLE")`, want: ""},
		{text: `F$TRNLNM("NAMES",,, "USERS")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$TRNLNM("NAMES",,,,, "WHAT")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$LOGICAL("NAMES")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$LOGICAL("NOSUCH")`, want: ""},
	})
}

// TestLexicalMessage: F$MESSAGE's text for a condition value, its
// directives left as they are.
func TestLexicalMessage(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		{text: "F$MESSAGE(1)", want: "%SYSTEM-S-NORMAL, normal successful completion"},
		{text: "F$MESSAGE(%X10000001)", want: "%SYSTEM-S-NORMAL, normal successful completion"},
		{text: "F$MESSAGE(%X38090)", want: "%CLI-W-IVVERB, unrecognized command verb - check validity and spelling"},
		{text: "F$MESSAGE(2)", want: "%NONAME-E-NOMSG, Message number 00000002"},
	})
}

// TestLexicalTime: F$TIME and F$CVTIME, at a fixed time: Friday, 9
// October 2026, 14:23:45.12.
func TestLexicalTime(t *testing.T) {
	c := lexConsole(t)
	now := vmsdef.Time(time.Date(2026, 10, 9, 14, 23, 45, 120_000_000, time.UTC))
	c.RTL.Clock = func() uint64 { return now }

	checkLexical(t, c, []lexCase{
		{text: "F$TIME()", want: " 9-OCT-2026 14:23:45.12"},
		{text: "F$CVTIME()", want: "2026-10-09 14:23:45.12"},
		{text: `F$CVTIME("")`, want: "2026-10-09 14:23:45.12"},
		{text: `F$CVTIME(,"ABSOLUTE")`, want: "9-OCT-2026 14:23:45.12"},
		{text: `F$CVTIME(,"ABSOLUTE","DATE")`, want: "9-OCT-2026"},
		{text: `F$CVTIME(,,"DATE")`, want: "2026-10-09"},
		{text: `F$CVTIME(,,"TIME")`, want: "14:23:45.12"},
		{text: `F$CVTIME(,,"WEEKDAY")`, want: "Friday"},
		{text: `F$CVTIME(,"ABSOLUTE","MONTH")`, want: "OCT"},
		{text: `F$CVTIME(,,"MONTH")`, want: "10"},
		{text: `F$CVTIME(,,"DAYOFYEAR")`, want: "282"},
		{text: `F$CVTIME(,,"HOUR")`, want: "14"},
		{text: `F$CVTIME(,,"HUNDREDTH")`, want: "12"},
		{text: `F$CVTIME("TOMORROW",,"WEEKDAY")`, want: "Saturday"},
		{text: `F$CVTIME("YESTERDAY")`, want: "2026-10-08 00:00:00.00"},
		{text: `F$CVTIME("TODAY:9:30")`, want: "2026-10-09 09:30:00.00"},
		{text: `F$CVTIME("14:00")`, want: "2026-10-09 14:00:00.00"},
		{text: `F$CVTIME("1-JAN-2000")`, want: "2000-01-01 00:00:00.00"},
		{text: `F$CVTIME("1-jan-2000:12:5:6.7")`, want: "2000-01-01 12:05:06.70"},
		{text: `F$CVTIME("1-JAN-2000 12:00")`, want: "2000-01-01 12:00:00.00"},
		{text: `F$CVTIME("-NOV-")`, want: "2026-11-09 00:00:00.00"},
		{text: `F$CVTIME("9-OCT-2026:10:00+1-2:00")`, want: "2026-10-10 12:00:00.00"},
		{text: `F$CVTIME("9-OCT-2026-1-")`, want: "2026-10-08 00:00:00.00"},
		{text: `F$CVTIME("TODAY+12:00")`, want: "2026-10-09 12:00:00.00"},
		{text: `F$CVTIME("TODAY-1-")`, want: "2026-10-08 00:00:00.00"},
		{text: `F$CVTIME("31-FEB-2026")`, code: vmserrors.CLI_IVATIME},
		{text: `F$CVTIME("25:00")`, code: vmserrors.CLI_IVATIME},
		{text: `F$CVTIME("NEXT")`, code: vmserrors.CLI_IVATIME},

		// Delta times.
		{text: `F$CVTIME("1-2:3:4.5","DELTA")`, want: "1 02:03:04.50"},
		{text: `F$CVTIME("2-","DELTA","DAY")`, want: "2"},
		{text: `F$CVTIME("10:30","DELTA","TIME")`, want: "10:30:00.00"},
		{text: `F$CVTIME("10:30","DELTA","MONTH")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CVTIME("1-JAN","DELTA")`, code: vmserrors.CLI_IVDTIME},
		{text: `F$CVTIME(,"SOON")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalHostFiles: F$SEARCH through host files, its streams, and
// F$PARSE of a name with no device to check.
func TestLexicalHostFiles(t *testing.T) {
	c := lexConsole(t)
	dir := t.TempDir()

	for _, name := range []string{"b.txt", "A.TXT", "c.dat"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.assignSymbol("D", "=", `"`+dir+`/"`); err != nil {
		t.Fatal(err)
	}

	a, b := filepath.Join(dir, "A.TXT"), filepath.Join(dir, "b.txt")

	checkLexical(t, c, []lexCase{
		{text: `F$SEARCH(D + "*.TXT")`, want: a},
		{text: `F$SEARCH(D + "*.DAT", 1)`, want: filepath.Join(dir, "c.dat")},
		{text: `F$SEARCH(D + "*.TXT")`, want: b},
		{text: `F$SEARCH(D + "*.DAT", 1)`, want: ""},
		{text: `F$SEARCH(D + "*.TXT")`, want: ""},
		{text: `F$SEARCH(D + "*.TXT")`, want: a},
		{text: `F$SEARCH(D + "%.DAT")`, want: filepath.Join(dir, "c.dat")},
		{text: `F$SEARCH(D + "NOSUCH.*")`, want: ""},
		{text: `F$PARSE("X.Y",,,, "SYNTAX_ONLY")`, want: "[000000]X.Y;"},
		{text: `F$PARSE("X.Y",,,"NAME")`, want: "X"},
		{text: `F$PARSE("X.Y",,,"FILE")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$PARSE("X.Y",,,,"QUICK")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalVolumeFiles: F$PARSE and F$SEARCH on a mounted volume
// (15.4.2's examples).
func TestLexicalVolumeFiles(t *testing.T) {
	c := lexConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.ContainerSession.CreateDirectory("[WORK]", rms.CreateDirectoryOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"[WORK]STATS.DAT", "[WORK]STATS.DAT", "[WORK]NOTES.TXT"} {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: name}, rms.TextRecords, [][]byte{[]byte("x")}); err != nil {
			t.Fatal(err)
		}
	}

	checkLexical(t, c, []lexCase{
		{text: `F$PARSE("STATS.DAT", "DUA0:[WORK]",,, "SYNTAX_ONLY")`, want: "DUA0:[WORK]STATS.DAT;"},
		{text: `F$PARSE("STATS.DAT")`, want: "DUA0:[000000]STATS.DAT;"},
		{text: `F$PARSE("STATS.DAT",,, "NAME")`, want: "STATS"},
		{text: `F$PARSE("[WORK]STATS",".DAT",, "TYPE")`, want: ".DAT"},
		{text: `F$PARSE("[WORK]X",, "OLD.LIS")`, want: "DUA0:[WORK]X.LIS;"},
		{text: `F$PARSE("X;5",,, "VERSION")`, want: ";5"},
		{text: `F$PARSE("X",,, "DEVICE")`, want: "DUA0:"},
		{text: `F$PARSE("[WORK]X",,, "DIRECTORY")`, want: "[WORK]"},
		{text: `F$PARSE("[NOSUCH]X")`, want: ""},
		{text: `F$PARSE("[NOSUCH]X",,,, "SYNTAX_ONLY")`, want: "DUA0:[NOSUCH]X.;"},
		{text: `F$PARSE("DUB0:X")`, want: ""},

		{text: `F$SEARCH("[WORK]*.*")`, want: "DUA0:[WORK]NOTES.TXT;1"},
		{text: `F$SEARCH("[WORK]*.*")`, want: "DUA0:[WORK]STATS.DAT;2"},
		{text: `F$SEARCH("[WORK]*.*")`, want: ""},
		{text: `F$SEARCH("[WORK]STATS.DAT;*")`, want: "DUA0:[WORK]STATS.DAT;2"},
		{text: `F$SEARCH("[WORK]STATS.DAT;*")`, want: "DUA0:[WORK]STATS.DAT;1"},
		{text: `F$SEARCH("[WORK]STATS")`, want: ""},
		{text: `F$SEARCH("[NOSUCH]*.*")`, want: ""},
	})
}

// TestLexicalWithoutConsole: a subprocess's command interpreter, which
// has no console, has the functions that don't ask about the system.
func TestLexicalWithoutConsole(t *testing.T) {
	symbols := &dclSymbolTable{}

	if v, err := evaluateDCLExpression(`F$EDIT(" A ", "TRIM")`, symbols, nil); err != nil || v.String() != "A" {
		t.Errorf("F$EDIT: %q, %v", v.String(), err)
	}

	if _, err := evaluateDCLExpression("F$TIME()", symbols, nil); !hasStatus(err, vmserrors.CLI_LEXNOTIMPL) {
		t.Errorf("F$TIME: %v, want LEXNOTIMPL", err)
	}

	if _, err := evaluateDCLExpression(`F$TRN("X")`, symbols, nil); err == nil || !strings.Contains(err.Error(), "F$TRNLNM") {
		t.Errorf("F$TRN: %v, want LEXNOTIMPL naming F$TRNLNM", err)
	}
}

// TestLexicalTable: every function the table has is one of VMS's, takes
// a sensible number of arguments, and names a symbol argument it has.
func TestLexicalTable(t *testing.T) {
	for name, f := range lexicalFunctions {
		if lexicalName(name) != name {
			t.Errorf("%s isn't one of VMS's lexical functions", name)
		}

		if f.minArgs > f.maxArgs || f.symbolArg > f.maxArgs || f.call == nil {
			t.Errorf("%s: arguments %d to %d, symbol argument %d", name, f.minArgs, f.maxArgs, f.symbolArg)
		}
	}
}
