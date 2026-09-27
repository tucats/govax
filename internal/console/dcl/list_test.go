package dcl

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// listTestGrammar exercises docs/PHASE-25.md subtask 5's grammar features
// with DEFINE/ASSIGN-shaped commands: /list parameters and qualifiers,
// DISALLOW ANY2(...), and a verb that has its own parameters as well as a
// /syntax= redirect qualifier.
const listTestGrammar = `
grammar test

type attrs
    keyword concealed/id=1
    keyword terminal/id=2

syntax define_device
    parameter name/id=10/type=$name/prompt="Device"
    qualifier cylinders/id=11/type=$integer

verb define
    parameter name/id=1/type=$any/prompt="Name"
    parameter value/id=2/type=$any/list/prompt="Value"
    qualifier device/syntax=define_device
    qualifier process/id=3
    qualifier group/id=4
    qualifier system/id=5
    qualifier table/id=6/type=$name
    qualifier translation_attributes/id=7/type=attrs/list
    qualifier tags/id=8/type=$any/list/default=NONE
    disallow any2(process, group, system, table)

verb assign
    parameter value/id=1/type=$any/list/prompt="Value"
    parameter name/id=2/type=$any/prompt="Name"

end
`

func loadListTestGrammar(t *testing.T) *Grammar {
	t.Helper()

	g, err := ParseGrammar(listTestGrammar)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}

	return g
}

func wantCLIStatus(t *testing.T, err error, code uint32) {
	t.Helper()

	var ve vmserrors.VMSError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want status %#x", err, code)
	}

	for e := error(ve); e != nil; e = errors.Unwrap(e) {
		if v, ok := e.(vmserrors.VMSError); ok && v.Status == code {
			return
		}
	}

	t.Fatalf("err = %v, want status %v in its chain", err, vmserrors.New(code))
}

func TestParse_listParameter(t *testing.T) {
	g := loadListTestGrammar(t)

	tests := []struct {
		line string
		want []string
	}{
		{"DEFINE X A", []string{"A"}},
		{"DEFINE X A,B", []string{"A", "B"}},
		{"DEFINE X A, B", []string{"A", "B"}},
		{"DEFINE X A ,B ,  C", []string{"A", "B", "C"}},
		{`DEFINE X "a,b", c`, []string{"a,b", "C"}},
		{`DEFINE X "" ,B`, []string{"", "B"}},
		{"define x [JONES.HISTORY],[JONES.WORKFILES]", []string{"[JONES.HISTORY]", "[JONES.WORKFILES]"}},
		{"DEFINE X DISK1:[FRED], DISK2:[GLADYS] /PROCESS", []string{"DISK1:[FRED]", "DISK2:[GLADYS]"}},
		{"DEFINE/PROCESS X A,B/SYSTEM", nil}, // DISALLOW; checked below
	}

	for _, tt := range tests[:len(tests)-1] {
		r, err := g.Parse(tt.line)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.line, err)
		}

		if got := r.List("VALUE"); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Parse(%q): List(VALUE) = %q, want %q", tt.line, got, tt.want)
		}

		if got := r.String("VALUE"); got != tt.want[0] {
			t.Errorf("Parse(%q): String(VALUE) = %q, want the first element %q", tt.line, got, tt.want[0])
		}
	}

	// A list parameter followed by another parameter (ASSIGN's order).
	r, err := g.Parse("ASSIGN A, B X")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.List("VALUE"); !reflect.DeepEqual(got, []string{"A", "B"}) || r.String("NAME") != "X" {
		t.Errorf("ASSIGN: VALUE=%q NAME=%q", got, r.String("NAME"))
	}

	for _, line := range []string{"DEFINE X A,,B", "DEFINE X A,", "DEFINE X A, /PROCESS"} {
		_, err := g.Parse(line)
		wantCLIStatus(t, err, vmserrors.CLI_EMPTYELEMENT)
	}
}

func TestParse_listQualifier(t *testing.T) {
	g := loadListTestGrammar(t)

	tests := []struct {
		line string
		want []string
	}{
		{"DEFINE/TRANSLATION_ATTRIBUTES=CONCEALED X A", []string{"CONCEALED"}},
		{"DEFINE/TRANS=(CONC,TERM) X A", []string{"CONCEALED", "TERMINAL"}},
		{"DEFINE/TRANS=( TERMINAL , CONCEALED ) X A", []string{"TERMINAL", "CONCEALED"}},
		{"DEFINE X A /TRANS=(TERMINAL)", []string{"TERMINAL"}},
	}

	for _, tt := range tests {
		r, err := g.Parse(tt.line)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.line, err)
		}

		if got := r.List("TRANSLATION_ATTRIBUTES"); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Parse(%q) = %q, want %q", tt.line, got, tt.want)
		}

		if got := r.Keyword("TRANSLATION_ATTRIBUTES"); got != tt.want[0] {
			t.Errorf("Parse(%q): Keyword = %q, want %q", tt.line, got, tt.want[0])
		}

		if got := r.List("VALUE"); !reflect.DeepEqual(got, []string{"A"}) {
			t.Errorf("Parse(%q): VALUE = %q", tt.line, got)
		}
	}

	// A string list qualifier, with a quoted element, and its default.
	r, err := g.Parse(`DEFINE/TAGS=("x,y",Z) X A`)
	if err != nil {
		t.Fatal(err)
	}

	if got := r.List("TAGS"); !reflect.DeepEqual(got, []string{"x,y", "Z"}) {
		t.Errorf("TAGS = %q", got)
	}

	r, err = g.Parse("DEFINE X A")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.List("TAGS"); !reflect.DeepEqual(got, []string{"NONE"}) {
		t.Errorf("defaulted TAGS = %q, want [NONE]", got)
	}

	if got := r.List("TRANSLATION_ATTRIBUTES"); got != nil {
		t.Errorf("absent TRANSLATION_ATTRIBUTES = %q, want nil", got)
	}

	errs := []struct {
		line string
		code uint32
	}{
		{"DEFINE/TRANS=(CONC X A", vmserrors.CLI_NEEDPAREN},
		{"DEFINE/TRANS=(CONC,) X A", vmserrors.CLI_EMPTYELEMENT},
		{"DEFINE/TRANS=() X A", vmserrors.CLI_EMPTYELEMENT},
		{"DEFINE X A/TRANS=", vmserrors.CLI_EMPTYELEMENT},
		{"DEFINE/TRANS=(BOGUS) X A", vmserrors.CLI_UNRECOGNIZED},
		{"DEFINE/TRANS=(NOTERMINAL) X A", vmserrors.CLI_NONEGATE},
	}

	for _, e := range errs {
		_, err := g.Parse(e.line)
		wantCLIStatus(t, err, e.code)
	}
}

func TestParse_disallowAny2(t *testing.T) {
	g := loadListTestGrammar(t)

	if n := len(g.entries["DEFINE"].Disallows); n != 6 {
		t.Errorf("ANY2 of 4 qualifiers made %d disallows, want 6", n)
	}

	for _, line := range []string{"DEFINE/PROCESS/SYSTEM X A", "DEFINE/GROUP/TABLE=T X A", "DEFINE X A/SYSTEM/TABLE=T"} {
		_, err := g.Parse(line)
		wantCLIStatus(t, err, vmserrors.CLI_BADQUALIFIERCOMBO)
	}

	for _, line := range []string{"DEFINE/PROCESS X A", "DEFINE/TABLE=T X A", "DEFINE X A"} {
		if _, err := g.Parse(line); err != nil {
			t.Errorf("Parse(%q): %v", line, err)
		}
	}

	for _, bad := range []string{"any2()", "any2(a)", "any2(a,,b)"} {
		src := "grammar g\nverb v\n    qualifier a\n    qualifier b\n    disallow " + bad + "\nend\n"
		if _, err := ParseGrammar(src); err == nil {
			t.Errorf("DISALLOW %s parsed", bad)
		}
	}

	src := "grammar g\nverb v\n    qualifier a\n    disallow any2(a, nosuch)\nend\n"
	if _, err := ParseGrammar(src); err == nil {
		t.Errorf("DISALLOW naming an unknown qualifier parsed")
	}
}

// TestParse_verbParametersWithRedirect checks that a verb with its own
// parameters can also carry a /syntax= redirect qualifier: DEFINE X A
// fills DEFINE's parameters, DEFINE/DEVICE switches to define_device's.
func TestParse_verbParametersWithRedirect(t *testing.T) {
	g := loadListTestGrammar(t)

	r, err := g.Parse("DEFINE X A")
	if err != nil {
		t.Fatal(err)
	}

	if r.Active != "DEFINE" || r.String("NAME") != "X" {
		t.Errorf("DEFINE X A: active %s, NAME %q", r.Active, r.String("NAME"))
	}

	r, err = g.Parse("DEFINE/DEVICE DKA0/CYLINDERS=10")
	if err != nil {
		t.Fatal(err)
	}

	if r.Active != "DEFINE_DEVICE" || r.String("NAME") != "DKA0" || r.Int("CYLINDERS") != 10 {
		t.Errorf("DEFINE/DEVICE: active %s, NAME %q, CYLINDERS %d", r.Active, r.String("NAME"), r.Int("CYLINDERS"))
	}

	// DEFINE's own required parameters don't apply once redirected, and
	// still do when not.
	_, err = g.Parse("DEFINE X")
	wantCLIStatus(t, err, vmserrors.CLI_MISSINGPARAMETER)
}

// TestConsoleGrammar_logicalNameCommands parses the Phase 25 logical-name
// commands against the real console grammar.
func TestConsoleGrammar_logicalNameCommands(t *testing.T) {
	g := loadEvaxGrammar(t)

	type check struct {
		name string
		want []string
	}

	tests := []struct {
		line   string
		active string
		checks []check
		flags  []string
	}{
		{"DEFINE GETTYSBURG [JONES.HISTORY], [JONES.WORKFILES]", "DEFINE",
			[]check{{"NAME", []string{"GETTYSBURG"}}, {"VALUE", []string{"[JONES.HISTORY]", "[JONES.WORKFILES]"}}}, nil},
		{"DEFINE/SYSTEM/EXEC/TRANS=(CONC,TERM) DISK DJA3:", "DEFINE",
			[]check{{"TRANSLATION_ATTRIBUTES", []string{"CONCEALED", "TERMINAL"}}}, []string{"SYSTEM", "EXECUTIVE_MODE"}},
		{"DEFINE/TABLE=LNM$GROUP X Y", "DEFINE", []check{{"TABLE", []string{"LNM$GROUP"}}}, nil},
		{"DEFINE/DEVICE DKA0", "DEFINE_DEVICE", []check{{"NAME", []string{"DKA0"}}}, nil},
		{"ASSIGN DUA1:, DUA2: DISK:", "ASSIGN",
			[]check{{"VALUE", []string{"DUA1:", "DUA2:"}}, {"NAME", []string{"DISK:"}}}, nil},
		{"DEASSIGN/ALL", "DEASSIGN", nil, []string{"ALL"}},
		{"DEASSIGN/USER_MODE DISK", "DEASSIGN", []check{{"NAME", []string{"DISK"}}}, []string{"USER_MODE"}},
		{"CREATE/NAME_TABLE/PARENT=LNM$SYSTEM_DIRECTORY TAB", "CREATE_NAME_TABLE",
			[]check{{"TABLE", []string{"TAB"}}, {"PARENT_TABLE", []string{"LNM$SYSTEM_DIRECTORY"}}}, nil},
		{"SHOW LOGICAL SYS$*, TT /FULL", "SHOW_LOGICAL", []check{{"NAME", []string{"SYS$*", "TT"}}}, []string{"FULL"}},
		{"SHOW LOGICAL/TABLE=(LNM$PROCESS,LNM$SYSTEM)", "SHOW_LOGICAL",
			[]check{{"TABLE", []string{"LNM$PROCESS", "LNM$SYSTEM"}}}, nil},
		{"SHOW TRANSLATION/TABLE=LNM$SYSTEM X", "SHOW_TRANSLATION", []check{{"NAME", []string{"X"}}}, nil},
		{"SHOW TRANSLATION_BUFFER", "SHOW_TB", nil, nil},
	}

	for _, tt := range tests {
		r, err := g.Parse(tt.line)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.line, err)

			continue
		}

		if r.Active != tt.active {
			t.Errorf("Parse(%q): active %s, want %s", tt.line, r.Active, tt.active)
		}

		for _, c := range tt.checks {
			if got := r.List(c.name); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%q): %s = %q, want %q", tt.line, c.name, got, c.want)
			}
		}

		for _, f := range tt.flags {
			if !r.Present(f) {
				t.Errorf("Parse(%q): /%s not present", tt.line, f)
			}
		}
	}

	for _, line := range []string{"DEFINE/PROCESS/SYSTEM X Y", "DEFINE/USER/EXEC X Y", "DEASSIGN/GROUP/TABLE=T X",
		"ASSIGN/SUPER/USER X Y", "CREATE/NAME_TABLE/USER/EXEC T", "SHOW LOGICAL/PROCESS/SYSTEM"} {
		_, err := g.Parse(line)
		wantCLIStatus(t, err, vmserrors.CLI_BADQUALIFIERCOMBO)
	}
}
