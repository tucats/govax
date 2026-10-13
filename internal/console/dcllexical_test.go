package console

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/dclshell"
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
		value, integer, err := c.EvaluateExpression(tc.text)

		switch {
		case tc.vms != "":
			if want := vmsdef.Symbols[tc.vms]; err == nil || dclshell.ConditionValue(err) != want {
				t.Errorf("%s: %v (value %q), want %s", tc.text, err, value, tc.vms)
			}
		case tc.code != 0:
			if !hasStatus(err, tc.code) {
				t.Errorf("%s: %v (value %q), want %v", tc.text, err, value, vmserrors.New(tc.code))
			}
		case err != nil:
			t.Errorf("%s: %v", tc.text, err)
		case integer != tc.integer || value != tc.want:
			t.Errorf("%s = %q (integer %v), want %q (integer %v)", tc.text, value, integer, tc.want, tc.integer)
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
		if err := c.AssignSymbol(a.name, a.op, a.text); err != nil {
			t.Fatalf("%s %s %s: %v", a.name, a.op, a.text, err)
		}
	}

	return c
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
		// each; image-value alone changes nothing (VMS 7.3).
		{text: `F$VERIFY(1)`, integer: true, want: "0"},
		{text: `F$ENVIRONMENT("VERIFY_IMAGE")`, want: "TRUE"},
		{text: `F$VERIFY(0, 1)`, integer: true, want: "1"},
		{text: `F$ENVIRONMENT("VERIFY_PROCEDURE")`, want: "FALSE"},
		{text: `F$ENVIRONMENT("VERIFY_IMAGE")`, want: "TRUE"},
		{text: `F$VERIFY(, 0)`, integer: true, want: "0"},
		{text: `F$ENVIRONMENT("VERIFY_IMAGE")`, want: "TRUE"},
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

// TestLexicalGetDVI: F$GETDVI of a device there is, and EXISTS of one
// there isn't; the items written as text (corevms's item forms, VPROT
// and ACPTYPE as VMS 7.1 wrote them: testdata/lib55's LIB55.LOG).
func TestLexicalGetDVI(t *testing.T) {
	c := lexConsole(t)
	mountFreshContainer(t, c, "DUA0")

	checkLexical(t, c, []lexCase{
		{text: `F$GETDVI("DUA0:", "EXISTS")`, want: "TRUE"},
		{text: `F$GETDVI("DUA9:", "EXISTS")`, want: "FALSE"},
		{text: `F$GETDVI("DUA0:", "MNT")`, want: "TRUE"},
		{text: `F$GETDVI("DUA0:", "VOLNAM")`, want: "DIRVOL"},
		{text: `F$GETDVI("DUA0:", "UNIT")`, integer: true, want: "0"},
		{text: `F$GETDVI("DUA0:", "ACPTYPE")`, want: "F11V2"},
		{text: `F$GETDVI("DUA0:", "VPROT")`, want: "SYSTEM=RWED, OWNER=RWED, GROUP=RWED, WORLD=RWED"},
		// The volume's owner is process 1, which mounted it.
		{text: `F$GETDVI("DUA0:", "PID")`, want: "00000301"},
		{text: `F$GETDVI("DUA0:", "OWNUIC")`, want: "[SYSTEM]"},
		{text: `F$GETDVI("DUA9:", "UNIT")`, vms: "SS$_NOSUCHDEV"},
		{text: `F$GETDVI("DUA0:", "NOSUCH")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalVolumeFiles: F$PARSE and F$SEARCH on a mounted volume
// (15.4.2's examples).
func TestLexicalVolumeFiles(t *testing.T) {
	c := lexConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.cmd().SetDefault("DUA0:[000000]"); err != nil {
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

// TestLexicalProcess: F$PROCESS, F$USER, F$PID, F$GETJPI, F$GETSYI,
// F$PRIVILEGE, and F$SETPRV, against the console's process.
func TestLexicalProcess(t *testing.T) {
	c := lexConsole(t)
	env := c.RTL
	pid := fmt.Sprintf("%08X", env.Process.PID)

	if err := c.AssignSymbol("CTX", "=", `""`); err != nil {
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
		{text: `F$GETJPI("", "JOBTYPE")`, integer: true, want: "3"},
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
		{text: `F$GETJPI("", "CURPRIV")`, want: strings.TrimPrefix(dclshell.PrivilegeList(env.Process.CurrentPrivileges), "CMKRNL,")},
		{text: `F$SETPRV("CMKRNL")`, want: "NOCMKRNL"},
		{text: `F$PRIVILEGE("CMKRNL")`, want: "TRUE"},
	})

	if s := dclshell.PrivilegeList(env.Process.CurrentPrivileges); !strings.HasPrefix(s, "CMKRNL,CMEXEC,") {
		t.Errorf("privileges %q, want CMKRNL, CMEXEC, ... in bit order", s)
	}
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
		{text: `F$CVTIME(,,"DAYOFYEAR")`, code: vmserrors.CLI_IVKEYW},
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
		{text: `F$CVTIME("1-2:3:4.5","DELTA")`, want: "1 2:3:4.5"},
		{text: `F$CVTIME("1-2:3:4.25","DELTA")`, want: "1 2:3:4.25"},
		{text: `F$CVTIME("1 2:3:4","DELTA")`, code: vmserrors.CLI_IVDTIME},
		{text: `F$CVTIME("2-","DELTA","DAY")`, want: "2"},
		{text: `F$CVTIME("10:30","DELTA","TIME")`, want: "10:30:00.00"},
		{text: `F$CVTIME("10:30","DELTA","MONTH")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CVTIME("1-JAN","DELTA")`, code: vmserrors.CLI_IVDTIME},
		{text: `F$CVTIME(,"SOON")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalIdentifier: F$IDENTIFIER both ways (the DCL Dictionary's
// examples), against the rights database govax builds: the process's
// own UIC identifier and the environmental ones.
func TestLexicalIdentifier(t *testing.T) {
	c := lexConsole(t)
	env := c.RTL
	uic := fmt.Sprint(int32(env.Process.UIC))

	checkLexical(t, c, []lexCase{
		{text: `F$IDENTIFIER("` + env.Process.Username + `", "NAME_TO_NUMBER")`, integer: true, want: uic},
		{text: `F$IDENTIFIER("` + strings.ToLower(env.Process.Username) + `", "name_to_number")`, integer: true, want: uic},
		{text: `F$IDENTIFIER(` + uic + `, "NUMBER_TO_NAME")`, want: env.Process.Username},
		{text: `F$IDENTIFIER("INTERACTIVE", "NAME_TO_NUMBER")`, integer: true, want: fmt.Sprint(int32(-0x7FFFFFFD))},
		{text: `F$IDENTIFIER(%X80000001, "NUMBER_TO_NAME")`, want: "BATCH"},
		{text: `F$IDENTIFIER("NOSUCH", "NAME_TO_NUMBER")`, integer: true, want: "0"},
		{text: `F$IDENTIFIER("12", "NAME_TO_NUMBER")`, integer: true, want: "0"},
		{text: `F$IDENTIFIER(%X7FFF0001, "NUMBER_TO_NAME")`, want: ""},
		{text: `F$IDENTIFIER("SYSTEM", "TO_NUMBER")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$IDENT("SYSTEM")`, code: vmserrors.CLI_ARGREQ},
	})
}

// TestLexicalDevice: F$DEVICE through the devices, by name, class, and
// type, each stream keeping its place, and "" after the last.
func TestLexicalDevice(t *testing.T) {
	c := lexConsole(t)
	mountFreshContainer(t, c, "DUA0")
	mountFreshContainer(t, c, "DUA1")

	// MOUNT's disks have no device type; DUA0 is given an RD54's.
	if d, ok := c.RTL.Devices.Find("DUA0"); ok {
		d.DevType = vmsdef.Symbols["DT$_RD54"]
	}

	node := "_" + c.RTL.NodeName + "$"

	checkLexical(t, c, []lexCase{
		{text: `F$DEVICE("DUA*")`, want: node + "DUA0:"},
		{text: `F$DEVICE("DUA*")`, want: node + "DUA1:"},
		{text: `F$DEVICE("DUA*")`, want: ""},
		{text: `F$DEVICE("DUA*")`, want: node + "DUA0:"},
		{text: `F$DEVICE("_DUA%:",,, 2)`, want: node + "DUA0:"},
		{text: `F$DEVICE("*1:", "DISK")`, want: node + "DUA1:"},
		{text: `F$DEVICE("*1:", "DISK")`, want: ""},
		{text: `F$DEVICE("DUA*", "DISK", "RD54")`, want: node + "DUA0:"},
		{text: `F$DEVICE("DUA*", "MAILBOX")`, want: ""},
		{text: `F$DEVICE("NOSUCH*")`, want: ""},
		{text: `F$DEVICE(, "NOSUCHCLASS")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$DEVICE("DUA*",, "RD54")`, vms: "SS$_BADPARAM"},
	})
}

// TestLexicalFileAttributes: F$FILE_ATTRIBUTES's items for a file on a
// mounted volume, a directory, and a host file, and its failures.
func TestLexicalFileAttributes(t *testing.T) {
	c := lexConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.cmd().SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.ContainerSession.CreateDirectory("[WORK]", rms.CreateDirectoryOptions{}); err != nil {
		t.Fatal(err)
	}

	records := [][]byte{[]byte("first line"), []byte("the second, longer line")}
	for range 2 {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "[WORK]QUEST.DAT"}, rms.TextRecords, records); err != nil {
			t.Fatal(err)
		}
	}

	cdt, _, err := c.EvaluateExpression(`F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "CDT")`)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := time.Parse("2-Jan-2006 15:04:05.00", cdt); err != nil {
		t.Errorf("CDT = %q: %v", cdt, err)
	}

	host := t.TempDir() + "/notes.txt"
	writeFile(t, host, strings.Repeat("x", 600)+"\n")

	checkLexical(t, c, []lexCase{
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "ORG")`, want: "SEQ"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "RFM")`, want: "VAR"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "RAT")`, want: "CR"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "LRL")`, integer: true, want: "23"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "EOF")`, integer: true, want: "1"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "DIRECTORY")`, want: "FALSE"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "VERLIMIT")`, integer: true, want: "32767"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "EDT")`, want: "17-NOV-1858 00:00:00.00"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "UIC")`, want: c.RTL.IdentifierText(0x00010004)},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "GRP")`, integer: true, want: "1"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "MBM")`, integer: true, want: "4"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "PRO")`, want: "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD="},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT;1", "FID")`, want: "(" + fidOf(t, c, "[WORK]QUEST.DAT;1") + ")"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "DID")`, want: "(" + fidOf(t, c, "[000000]WORK.DIR;1") + ")"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "DVI")`, want: "_" + c.RTL.NodeName + "$DUA0"},
		{text: `F$FILE_ATTRIBUTES("WORK.DIR", "DIRECTORY")`, want: "TRUE"},
		{text: `F$FILE_ATTRIBUTES("WORK.DIR", "CTG")`, want: "TRUE"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST", "ORG")`, vms: "RMS$_FNF"},
		{text: `F$FILE_ATTRIBUTES("[NODIR]QUEST.DAT", "ORG")`, vms: "RMS$_DNF"},
		{text: `F$FILE_ATTRIBUTES("[WORK]*.DAT", "ORG")`, vms: "RMS$_WLD"},
		{text: `F$FILE_ATTRIBUTES("[WORK]QUEST.DAT", "NOSUCH")`, code: vmserrors.CLI_IVKEYW},

		{text: `F$FILE_ATTRIBUTES("` + host + `", "RFM")`, want: "STMLF"},
		{text: `F$FILE_ATTRIBUTES("` + host + `", "EOF")`, integer: true, want: "2"},
		{text: `F$FILE_ATTRIBUTES("` + host + `", "FFB")`, integer: true, want: "89"},
		{text: `F$FILE_ATTRIBUTES("` + host + `x", "RFM")`, vms: "RMS$_FNF"},
	})
}

// fidOf is the file ID of the file spec names, as DIRECTORY/FILE_ID
// shows it, without the parentheses.
func fidOf(t *testing.T, c *Console, spec string) string {
	t.Helper()

	text, _, err := c.ContainerSession.Directory(spec, rms.DirectoryOptions{File: true})
	if err != nil {
		t.Fatal(err)
	}

	_, after, ok := strings.Cut(text, "(")
	fid, _, ok2 := strings.Cut(after, ")")

	if !ok || !ok2 {
		t.Fatalf("no file ID for %s in %q", spec, text)
	}

	return fid
}

// TestLexicalContext: F$CONTEXT's criteria select the processes F$PID
// returns; the symbol is a PROCESS_CONTEXT until F$PID's "" or CANCEL;
// a context F$PID has used is frozen.
func TestLexicalContext(t *testing.T) {
	c := lexConsole(t)
	env := c.RTL
	pid := fmt.Sprintf("%08X", env.Process.PID)
	user := env.Process.Username

	for _, name := range []string{"CTX", "CTX3", "CTX4"} {
		if err := c.AssignSymbol(name, "=", `""`); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.AssignSymbol("CTX5", "=", `"abc"`); err != nil {
		t.Fatal(err)
	}

	checkLexical(t, c, []lexCase{
		{text: `F$CONTEXT("PROCESS", CTX, "USERNAME", "` + user[:2] + `*,NOBODY", "EQL")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "GRP", 1, "EQL")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "PRI", "1", "GEQ")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "CURPRIV", "CMKRNL,TMPMBX", "ALL")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "JOBTYPE", "LOCAL", "EQL")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "MODE", "INTERACTIVE", "EQL")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "UIC", "` + env.IdentifierText(env.Process.UIC) + `", "EQL")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX, "NODENAME", "*", "EQL")`, want: ""},
		{text: "F$TYPE(CTX)", want: "PROCESS_CONTEXT"},
		{text: "F$PID(CTX)", want: pid},
		{text: `F$CONTEXT("PROCESS", CTX, "PRI", "1", "GEQ")`, code: vmserrors.CLI_FRZNCTX},
		{text: "F$PID(CTX)", want: ""},
		{text: "F$TYPE(CTX)", want: "STRING"},
		{text: "CTX", want: ""},

		// A context nothing meets; CTX2 is undefined at first.
		{text: `F$CONTEXT("PROCESS", CTX2, "PRCNAM", "NOBODY", "EQL")`, want: ""},
		{text: "F$PID(CTX2)", want: ""},
		{text: `F$CONTEXT("PROCESS", CTX3, "USERNAME", "` + user + `", "NEQ")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX3, "UIC", "[1,4]", "EQL")`, want: ""},
		{text: "F$PID(CTX3)", want: ""},

		// CANCEL.
		{text: `F$CONTEXT("PROCESS", CTX4, "PRI", "0", "GTR")`, want: ""},
		{text: `F$CONTEXT("PROCESS", CTX4, "CANCEL")`, want: ""},
		{text: "F$TYPE(CTX4)", want: "STRING"},

		// Failures.
		{text: `F$CONTEXT("QUEUE", CTX6, "PRI", "0", "GTR")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CONTEXT("PROCESS", CTX6, "NOSUCH", "0", "GTR")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CONTEXT("PROCESS", CTX6, "PRI", "0", "ALL")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CONTEXT("PROCESS", CTX6, "PRI", "X", "EQL")`, code: vmserrors.CLI_IVVALU},
		{text: `F$CONTEXT("PROCESS", CTX6, "PRI")`, code: vmserrors.CLI_ARGREQ},
		{text: `F$CONTEXT("PROCESS", CTX6, "CURPRIV", "NOSUCH", "ANY")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CONTEXT("PROCESS", CTX5, "PRI", "0", "GTR")`, code: vmserrors.CLI_NOTCTX},
		{text: `F$CONTEXT("PROCESS", CTX5, "CANCEL")`, code: vmserrors.CLI_NOTCTX},
	})
}
