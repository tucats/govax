package dcl

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// synonymTestGrammar is LOGOUT's shape: /BRIEF and /FULL are one setting
// (SYNONYM BRIEF NOFULL), /NOFULL being /BRIEF as on VMS 7.1, and /TRACE
// an unrelated switch. LO is an alias verb, as in
// console.dcl, to show the rule follows the alias to LOGOUT.
const synonymTestGrammar = `grammar g
verb logout/id=1
    qualifier brief
    qualifier full
    qualifier trace
    synonym   brief nofull
verb lo/alias=logout
end
`

// TestParse_synonymLastWins checks that both of a synonym pair may be
// given, the last one counting and the other forgotten, so a handler that
// asks Present("BRIEF") or Present("FULL") sees only the last.
func TestParse_synonymLastWins(t *testing.T) {
	g, err := ParseGrammar(synonymTestGrammar)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		line        string
		brief, full bool
	}{
		{"LOGOUT", false, false},
		{"LOGOUT/BRIEF", true, false},
		{"LOGOUT/FULL", false, true},
		{"LOGOUT/FULL/BRIEF", true, false},
		{"LOGOUT/BRIEF/FULL", false, true},
		{"LO/FULL/BR", true, false},               // LO*GOUT := LOGOUT/FULL, then LO/BR
		{"LO/FULL/TRACE/BRIEF/FULL", false, true}, // another qualifier between them
		{"LOGOUT/BRIEF/BRIEF", true, false},
		{"LOGOUT/NOFULL", true, false}, // VMS: a brief logout
		{"LOGOUT/NOBRIEF", false, true},
		{"LO/FULL/NOFULL", true, false},
		{"LO/BRIEF/NOBRIEF", false, true},
	}

	for _, c := range cases {
		r, err := g.Parse(c.line)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.line, err)

			continue
		}

		if got := r.Present("BRIEF"); got != c.brief {
			t.Errorf("Parse(%q): BRIEF present %v, want %v", c.line, got, c.brief)
		}

		if got := r.Present("FULL"); got != c.full {
			t.Errorf("Parse(%q): FULL present %v, want %v", c.line, got, c.full)
		}

		if r.Negated("BRIEF") || r.Negated("FULL") {
			t.Errorf("Parse(%q): a synonym left a negated form", c.line)
		}
	}

	r, err := g.Parse("LO/TRACE/FULL/BRIEF")
	if err != nil {
		t.Fatal(err)
	}

	if !r.Present("TRACE") {
		t.Error("a synonym forgot an unrelated qualifier")
	}

	// TRACE isn't paired, so /NOTRACE stays itself.
	if r, err = g.Parse("LO/NOTRACE"); err != nil || !r.Negated("TRACE") {
		t.Errorf("LO/NOTRACE: %v, negated %v", err, err == nil && r.Negated("TRACE"))
	}

	// A NONEGATABLE member of a pair still refuses its NO form.
	n, err := ParseGrammar("grammar g\nverb v\n    qualifier a/nonegatable\n    qualifier b\n    synonym a nob\nend\n")
	if err != nil {
		t.Fatal(err)
	}

	_, err = n.Parse("V/NOA")
	wantCLIStatus(t, err, vmserrors.CLI_NONEGATE)
}

// TestParse_synonymBadGrammar checks the statement's form and its
// validation against the entry's qualifiers.
func TestParse_synonymBadGrammar(t *testing.T) {
	cases := []struct {
		body string
		code uint32
	}{
		{"qualifier a\nqualifier b\nsynonym a", vmserrors.CLI_BADSYNONYM},
		{"qualifier a\nqualifier b\nsynonym", vmserrors.CLI_BADSYNONYM},
		{"qualifier a\nqualifier b\nsynonym a b", vmserrors.CLI_BADSYNONYM},             // not a negation
		{"qualifier a\nqualifier b\nsynonym a noa", vmserrors.CLI_BADSYNONYM},           // itself
		{"qualifier a\nqualifier b\nsynonym a b c", vmserrors.CLI_BADSYNONYM},           // too many
		{"qualifier a\nsynonym a nonosuch", vmserrors.CLI_SYNONYMNOTFOUND},              // unknown
		{"qualifier alpha\nqualifier b\nsynonym al nob", vmserrors.CLI_SYNONYMNOTFOUND}, // abbreviated
		{"qualifier a/type=$string\nqualifier b\nsynonym a nob", vmserrors.CLI_SYNONYMNOTFOUND},
		{"qualifier a\nqualifier b/alias=a\nsynonym a nob", vmserrors.CLI_SYNONYMNOTFOUND},
	}

	for _, c := range cases {
		_, err := ParseGrammar("grammar g\nverb v\n" + c.body + "\nend\n")
		if err == nil {
			t.Errorf("grammar %q parsed", c.body)

			continue
		}

		if !hasStatus(err, c.code) {
			t.Errorf("grammar %q: %v, want status %#x", c.body, err, c.code)
		}
	}

	if _, err := ParseGrammar("grammar g\nsynonym a nob\nend\n"); err == nil {
		t.Error("a SYNONYM outside a verb parsed")
	}
}

// hasStatus reports whether err's chain has a VMS error with the status.
func hasStatus(err error, code uint32) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if v, ok := e.(vmserrors.VMSError); ok && v.Status == code {
			return true
		}
	}

	return false
}
