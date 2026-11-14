package dcl

import (
	"bufio"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// securityTestGrammar has SET SECURITY's and CREATE/DIRECTORY's shapes
// (docs/planning/PHASE-93 - file protection.md, Decision 9): a protection
// code and a UIC as qualifiers, a protection code as a positional
// parameter (SET PROTECTION=(code)), and a UIC as a positional
// qualifier.
const securityTestGrammar = `
grammar test

verb set
    parameter file/id=1/type=$string/prompt="File"
    qualifier protection/id=2/type=$protection
    qualifier owner/id=3/type=$uic
    qualifier by_owner/id=4/parameter=file/type=$uic/default=""

verb setprot
    parameter code/id=10/type=$protection
    parameter file/id=11/type=$string
    qualifier default/id=12

end
`

func securityGrammar(t *testing.T) *Grammar {
	t.Helper()

	g, err := ParseGrammar(securityTestGrammar)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}

	return g
}

// TestProtectionValues: what $PROTECTION hands the command, an element per
// category as typed, uppercased and without blanks, from a parenthesized
// list or a single category.
func TestProtectionValues(t *testing.T) {
	g := securityGrammar(t)

	for line, want := range map[string][]string{
		"SET/PROTECTION=(S:RWED,O:RWED,G:RE,W) F":  {"S:RWED", "O:RWED", "G:RE", "W"},
		"SET/PROTECTION=(s: rwed) F":               {"S:RWED"},
		"SET/PROTECTION=S:R F":                     {"S:R"},
		"SET/PROTECTION=(SYSTEM=RW)/OWNER=[1,4] F": {"SYSTEM=RW"},
		"SET/PROTECTION:(W:RE , G:RE) F":           {"W:RE", "G:RE"},
		"SETPROT (S:RQ) F":                         {"S:RQ"},
		"SETPROT (G:RWED)/DEFAULT":                 {"G:RWED"},
	} {
		r, err := g.Parse(line)
		if err != nil {
			t.Errorf("%s: %v", line, err)

			continue
		}

		name := "PROTECTION"
		if strings.HasPrefix(line, "SETPROT") {
			name = "CODE"
		}

		if got := r.List(name); !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", line, got, want)
		}

		if r.String("FILE") != "F" && !r.Present("DEFAULT") {
			t.Errorf("%s: the file is %q", line, r.String("FILE"))
		}
	}
}

// TestUICValues: what $UIC hands the command, as a qualifier and as a
// positional one.
func TestUICValues(t *testing.T) {
	g := securityGrammar(t)

	for line, want := range map[string]string{
		"SET/OWNER=[1,4] F":              "[1,4]",
		"SET/OWNER=<200,1> F":            "<200,1>",
		"SET/OWNER=[prot93,*] F":         "[PROT93,*]",
		"SET/OWNER=SMITH F":              "SMITH",
		"SET/OWNER=[1,4]/PROTECTION=W F": "[1,4]",
	} {
		r, err := g.Parse(line)
		if err != nil {
			t.Errorf("%s: %v", line, err)

			continue
		}

		if got := r.String("OWNER"); got != want {
			t.Errorf("%s: %q, want %q", line, got, want)
		}
	}

	r, err := g.Parse("SET F/BY_OWNER=[200,*]")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.ParamString("FILE", "BY_OWNER"); got != "[200,*]" {
		t.Errorf("BY_OWNER = %q", got)
	}
}

// TestProbeMalformedValues replays the probe's "MALFORMED AND ODD VALUES"
// (testdata/prot93/vax/prot93.log, VMS 7.1): each SET SECURITY line whose
// /PROTECTION or /OWNER DCL refused must be refused by the grammar with
// the same message and segment; each line VMS's SET refused with
// %SET-F-SYNTAX must pass the grammar and be refused by security's
// parsers, with the kind of error SET's second line names; and every
// other line must pass both.
func TestProbeMalformedValues(t *testing.T) {
	g := securityGrammar(t)

	cases := probeSection(t, "######## MALFORMED AND ODD VALUES")

	checked := 0

	for _, c := range cases {
		cmd, ok := strings.CutPrefix(c.command, "$ SET SECURITY")
		if !ok || strings.Contains(cmd, "/ACL") {
			continue
		}

		checked++
		line := "SET" + strings.Replace(cmd, " PROT93$DISK:", " ", 1)
		r, err := g.Parse(line)

		if len(c.messages) > 0 && strings.HasPrefix(c.messages[0], "%DCL-") {
			want := strings.TrimPrefix(strings.Join(c.messages, "\n"), "%")
			if err == nil || err.Error() != want {
				t.Errorf("%s:\n got %v\nwant %s", c.command, err, want)
			}

			continue
		}

		if err != nil {
			t.Errorf("%s: %v", c.command, err)

			continue
		}

		var fault error

		if r.Present("PROTECTION") {
			_, fault = security.ParseProtection(strings.Join(r.List("PROTECTION"), ","), 0)
		}

		if fault == nil && r.Present("OWNER") {
			lookup := func(name string) (uint32, bool) { return 0o10004, name == "SYSTEM" }
			_, fault = security.ParseUIC(r.String("OWNER"), lookup)
		}

		want := ""
		if len(c.messages) > 0 {
			want = c.messages[len(c.messages)-1]
		}

		switch {
		case strings.HasPrefix(want, "-LIB-F-SYNTAXERR"):
			if !errors.As(fault, new(security.ErrBadAccess)) {
				t.Errorf("%s: %v, want bad letters", c.command, fault)
			}
		case strings.HasPrefix(want, "-LIB-F-INVARG"):
			if !errors.Is(fault, security.ErrRepeatedCategory) {
				t.Errorf("%s: %v, want a repeated category", c.command, fault)
			}
		case strings.HasPrefix(want, "%SET-F-SYNTAX"):
			if fault == nil {
				t.Errorf("%s: accepted, VMS refused it", c.command)
			}
		case fault != nil:
			t.Errorf("%s: %v, VMS took it", c.command, fault)
		}
	}

	if checked < 25 {
		t.Errorf("only %d SET SECURITY lines checked", checked)
	}
}

// TestProtectionErrors: the shapes VMS's log has no line for, read as
// DCL's messages.
func TestProtectionErrors(t *testing.T) {
	g := securityGrammar(t)

	for line, want := range map[string]string{
		"SET/PROTECTION=(:R) F":     "IVKEYW",
		"SET/PROTECTION=(S:R F":     "NOPAREN",
		"SET/OWNER=[1,4]X F":        "DIRECT",
		"SET/OWNER=[1,4 /LOG F":     "DIRECT",
		"SETPROT (S:R,,W)":          "VALREQ",
		"SET F/BY_OWNER=[1,2,3]":    "DIRECT",
		"SET F/BY_OWNER=1,2":        "NOLIST",
		"SET/PROTECTION=(GROUPS) F": "IVKEYW",
	} {
		_, err := g.Parse(line)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %s", line, err, want)
		}
	}
}

// probeCase is a command from a probe log and the messages after it.
type probeCase struct {
	command  string
	messages []string
}

// probeSection reads the commands of one section of Phase 93's VMS log,
// each with the % and - lines and segment lines that follow it.
func probeSection(t *testing.T, heading string) []probeCase {
	t.Helper()

	f, err := os.Open("../../testdata/prot93/vax/prot93.log")
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	var (
		cases []probeCase
		in    bool
	)

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()

		switch {
		case line == heading:
			in = true
		case !in:
		case strings.HasPrefix(line, "########"):
			return cases
		case strings.HasPrefix(line, "$ "):
			cases = append(cases, probeCase{command: line})
		case len(cases) > 0 && (strings.HasPrefix(line, "%") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " \\")):
			cases[len(cases)-1].messages = append(cases[len(cases)-1].messages, line)
		}
	}

	t.Fatalf("no section %q", heading)

	return nil
}
