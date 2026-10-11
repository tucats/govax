package console

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Tests of verification (dclverify.go): SET VERIFY, SET PREFIX, and how
// a procedure's lines are shown, from the DCL Dictionary's SET VERIFY,
// SET PREFIX, and F$VERIFY and the User's Manual's 13.5.5.

// TestSetVerifyKeywords: SET VERIFY and SET NOVERIFY change both
// settings; a keyword changes only its own (the Dictionary's example 1),
// abbreviated or not; SET NOVERIFY with keywords is CLI_CONFLICT, its
// segment the word as typed (VMS 7.3, testdata/dcl50's round 2).
func TestSetVerifyKeywords(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	for _, tc := range []struct {
		command          string
		procedure, image bool
		status           uint32
		segment          string
	}{
		{"SET VERIFY", true, true, 0, ""},
		{"SET NOVERIFY", false, false, 0, ""},
		{"SET VERIFY=PROCEDURE", true, false, 0, ""},
		{"SET VERIFY=IMAGE", true, true, 0, ""},
		{"SET VERIFY=(NOPROCEDURE, NOIMAGE)", false, false, 0, ""},
		{"SET VERIFY = (IMAGE)", false, true, 0, ""},
		{"SET VERIFY=(PROC,NOIMAG)", true, false, 0, ""},
		{"SET VERIFY=(NOPROCE,IMA)", false, true, 0, ""},
		{"SET VERI", true, true, 0, ""},
		{"SET NOVERIFY=IMAGE", true, true, vmserrors.CLI_CONFLICT, "NOVERIFY"},
		{"SET NOVERI=(PROCEDURE)", true, true, vmserrors.CLI_CONFLICT, "NOVERI"},
		{"SET VERIFY=(NO)", true, true, vmserrors.CLI_IVKEYW, "NO"},
		{"SET VERIFY=(BOGUS)", true, true, vmserrors.CLI_IVKEYW, "BOGUS"},
	} {
		err := d.Dispatch(tc.command)

		if tc.status != 0 {
			if !hasStatus(err, tc.status) || !strings.Contains(err.Error(), `\`+tc.segment+`\`) {
				t.Errorf("%s: %v, want %s with segment %s", tc.command, err, vmserrors.New(tc.status), tc.segment)
			}
		} else if err != nil {
			t.Errorf("%s: %v", tc.command, err)
		}

		if c.Verify != tc.procedure || c.verifyImage != tc.image {
			t.Errorf("%s: procedure %v image %v, want %v %v", tc.command, c.Verify, c.verifyImage, tc.procedure, tc.image)
		}
	}
}

// TestVerifyProcedureLines: with verification on, each line is shown as
// it is in the file (its "$", indentation, label, and comment), after
// apostrophe substitution and before its output; comment lines too; a
// continued command a record at a time; and nothing of an IF branch that
// doesn't run.
func TestVerifyProcedureLines(t *testing.T) {
	out := runFlow(t,
		"$ SET VERIFY",
		"$ N = 2",
		"$!  a comment",
		`$   PRINT "N is ''N'"`,
		"$ LOOP: N = N - 1",
		`$ IF N .GT. 0 THEN GOTO LOOP ! again`,
		`$ X = "one" + -`,
		`  " two"`,
		`$ PRINT "''X'"`,
		"$ IF N .EQ. 0",
		"$ THEN",
		`$   PRINT "zero"`,
		"$ ELSE",
		`$   PRINT "not zero"`,
		"$ ENDIF",
		"$ X := 'N'",
		"$ SET NOVERIFY",
		`$ PRINT "quiet"`)

	want := strings.Join([]string{
		"$ N = 2",
		"$!  a comment",
		`$   PRINT "N is 2"`,
		`N is 2`,
		"$ LOOP: N = N - 1",
		`$ IF N .GT. 0 THEN GOTO LOOP ! again`,
		"$ LOOP: N = N - 1",
		`$ IF N .GT. 0 THEN GOTO LOOP ! again`,
		`$ X = "one" + -`,
		`  " two"`,
		`$ PRINT "one two"`,
		"one two",
		"$ IF N .EQ. 0",
		"$ THEN",
		`$   PRINT "zero"`,
		"zero",
		"$ ELSE",
		"$ ENDIF",
		"$ X := 0",
		"$ SET NOVERIFY",
		"quiet",
	}, "\n") + "\n"

	if out != want {
		t.Errorf("output:\n%s\nwant:\n%s", out, want)
	}
}

// TestVerifyLastsAfterProcedure: a procedure's SET VERIFY stays in
// effect after it ends (the Dictionary's SET VERIFY), and a CALL's lines
// are shown as any others, starting with its SUBROUTINE line (VMS 7.3).
func TestVerifyLastsAfterProcedure(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)

	path := writeProcedure(t, t.TempDir(), "v.com",
		"$ SET VERIFY",
		"$ CALL SUB",
		"$ EXIT",
		"$ SUB: SUBROUTINE",
		`$ PRINT "in sub"`,
		"$ ENDSUBROUTINE")

	if err := d.Dispatch("@" + path); err != nil {
		t.Fatal(err)
	}

	want := "$ CALL SUB\n$ SUB: SUBROUTINE\n$ PRINT \"in sub\"\nin sub\n$ ENDSUBROUTINE\n$ EXIT\n"
	if buf.String() != want || !c.Verify {
		t.Errorf("output %q (verify %v), want %q, still on", buf.String(), c.Verify, want)
	}

	// The terminal's own commands are never shown.
	buf.Reset()

	if err := d.Dispatch(`PRINT "typed"`); err != nil || buf.String() != "typed\n" {
		t.Errorf("terminal: %q, %v", buf.String(), err)
	}
}

// TestVerifyInComment: F$VERIFY between apostrophes is performed in a
// comment, before the line would be shown, so "'F$VERIFY(0)'" hides
// itself and "'F$VERIFY(1)'" shows itself (the Dictionary's F$VERIFY),
// replaced by its value; its closing apostrophe may be missing (VMS
// 7.3). Nothing else in a comment is substituted.
func TestVerifyInComment(t *testing.T) {
	out := runFlow(t,
		"$ ! 'F$VERIFY(1)'",
		`$ PRINT "shown"`,
		"$ ! 'F$VER(0)' hides this line",
		`$ PRINT "hidden"`,
		`$ X = 1 ! 'f$verify(1)' and 'X'`,
		`$ PRINT "shown again"`,
		"$ ! 'F$VERIFY(0) without its apostrophe",
		`$ PRINT "hidden again"`,
		"$ V = 'F$VERIFY(0)'",
		`$ PRINT "V=''V'"`)

	want := strings.Join([]string{
		"$ ! 0",
		`$ PRINT "shown"`,
		"shown",
		"hidden",
		`$ X = 1 ! 0 and 'X'`,
		`$ PRINT "shown again"`,
		"shown again",
		"hidden again",
		"V=0",
	}, "\n") + "\n"

	if out != want {
		t.Errorf("output:\n%s\nwant:\n%s", out, want)
	}
}

// TestVerifyContinued: a continued command is shown a record at a time,
// each record substituted (VMS 7.3), and the THEN of a false IF isn't
// shown.
func TestVerifyContinued(t *testing.T) {
	out := runFlow(t,
		"$ SET VERIFY",
		`$ N = "MYFILE"`,
		"$ A := 'N' - ! a comment",
		"  .DAT",
		`$ L1: A = "''N'" + -`,
		`     ".DAT"`,
		"$ IF 0",
		"$ THEN",
		`$   PRINT "no"`,
		"$ ENDIF",
		"$ SET NOVERIFY")

	want := strings.Join([]string{
		`$ N = "MYFILE"`,
		"$ A := MYFILE - ! a comment",
		"  .DAT",
		`$ L1: A = "MYFILE" + -`,
		`     ".DAT"`,
		"$ IF 0",
		"$ ENDIF",
		"$ SET NOVERIFY",
	}, "\n") + "\n"

	if out != want {
		t.Errorf("output:\n%s\nwant:\n%s", out, want)
	}
}

// TestVerifyPrefix: SET PREFIX's control string, formatted, before each
// verified command line, and blanks as long as it before a continuation
// (the Dictionary's SET PREFIX); F$ENVIRONMENT("VERIFY_PREFIX") returns
// it, and SET NOPREFIX takes it away.
func TestVerifyPrefix(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	now := vmsdef.Time(time.Date(2026, 10, 10, 17, 52, 30, 0, time.UTC))
	c.RTL.Clock = func() uint64 { return now }

	path := writeProcedure(t, t.TempDir(), "p.com",
		`$ SET PREFIX "(!5%T) "`,
		"$ SET VERIFY",
		`$ P = F$ENVIRONMENT("VERIFY_PREFIX")`,
		`$ X = P + -`,
		`  "."`,
		`$ PRINT "''X'"`,
		"$ SET NOPREFIX",
		"$ SET NOVERIFY")

	if err := d.Dispatch("@" + path); err != nil {
		t.Fatal(err)
	}

	want := strings.Join([]string{
		`(17:52) $ P = F$ENVIRONMENT("VERIFY_PREFIX")`,
		`(17:52) $ X = P + -`,
		`          "."`,
		`(17:52) $ PRINT "(!5%T) ."`,
		"(!5%T) .",
		"(17:52) $ SET NOPREFIX",
		"$ SET NOVERIFY",
	}, "\n") + "\n"

	if buf.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), want)
	}

	if err := c.SetPrefix(strings.Repeat("x", 65)); !hasStatus(err, vmserrors.CLI_IVVALU) {
		t.Errorf("a 65-character prefix: %v, want IVVALU", err)
	}

	// A directive that takes an argument, other than the time's, or a
	// malformed one, is INVPREFIX, and the prefix stays (VMS 7.3).
	if err := c.SetPrefix("!8%D !/!_!^!!!3*- "); err != nil {
		t.Errorf("a prefix of argumentless directives: %v", err)
	}

	for _, bad := range []string{"[!UL|!AS|!XL] ", "!%D !", "!#%T", "!+"} {
		if err := c.SetPrefix(bad); !hasStatus(err, vmserrors.CLI_INVPREFIX) {
			t.Errorf("SET PREFIX %q: %v, want INVPREFIX", bad, err)
		}
	}

	if c.verifyPrefix != "!8%D !/!_!^!!!3*- " {
		t.Errorf("prefix %q after refused ones", c.verifyPrefix)
	}
}

// TestVerifyImageData: image verification shows the data lines given to
// what reads the procedure's input, here the interactive assembler, and
// procedure verification alone doesn't.
func TestVerifyImageData(t *testing.T) {
	for _, tc := range []struct {
		setting string
		shown   bool
	}{
		{"SET VERIFY", true},
		{"SET VERIFY=(PROCEDURE,NOIMAGE)", false},
	} {
		out := runFlow(t,
			"$ "+tc.setting,
			"$ ASM",
			"        NOP",
			"$ SET NOVERIFY")

		if got := strings.Contains(out, "\n        NOP\n"); got != tc.shown {
			t.Errorf("%s: data line shown %v, want %v; output %q", tc.setting, got, tc.shown, out)
		}
	}
}
