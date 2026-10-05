package dcl

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

func TestReadExpression(t *testing.T) {
	cases := []struct {
		in        string
		sep       byte
		tok, rest string
	}{
		{"100", 0, "100", ""},
		{"100 200", 0, "100", " 200"},
		{"X + 4 Y", 0, "X + 4", " Y"},
		{"X+ 4", 0, "X+ 4", ""},
		{"X +4", 0, "X +4", ""},
		{"X/Y", 0, "X/Y", ""},
		{"X / Y", 0, "X / Y", ""},
		{"X /Y", 0, "X", " /Y"},
		{"100 /BYTE", 0, "100", " /BYTE"},
		{"F(1, 2) /STEP", 0, "F(1, 2)", " /STEP"},
		{"(A / B) C", 0, "(A / B)", " C"},
		{`"a b,c" , 3`, 0, `"a b,c"`, " , 3"},
		{`DEFINED("x") THEN SET X=1`, 0, `DEFINED("x")`, " THEN SET X=1"},
		{"X = 1 THEN Y", 0, "X = 1", " THEN Y"},
		{"X=5", '=', "X", "=5"},
		{"X = 5", '=', "X", " = 5"},
		{"A,B", 0, "A", ",B"},
		{"A)", 0, "A", ")"},
		{"^X1F", 0, "^X1F", ""},
		{`"open`, 0, `"open`, ""},
		{"  7  ", 0, "7", "  "},
	}

	for _, c := range cases {
		tok, rest, err := readExpression(c.in, c.sep)
		if err != nil {
			t.Errorf("readExpression(%q): %v", c.in, err)

			continue
		}

		if tok != c.tok || rest != c.rest {
			t.Errorf("readExpression(%q, %q) = %q, %q; want %q, %q", c.in, c.sep, tok, rest, c.tok, c.rest)
		}
	}
}

// expressionTestGrammar exercises Phase 37's additions: $expression
// values, /separator=, /assignment=, /nonegatable keywords, and '@'.
const expressionTestGrammar = `
grammar test

type set_types
    keyword radix/syntax=set_radix/nonegatable
    keyword trace/syntax=set_trace

verb examine
    qualifier byte
    parameter start/type=$expression
    parameter end/type=$expression

verb deposit
    parameter target/type=$expression/separator="="/prompt="Target"
    parameter value/type=$expression/prompt="Value"

verb print
    parameter items/type=$expression/list

verb set/assignment=set_symbol
    qualifier permanent
    parameter what/type=set_types/prompt="What"

syntax set_symbol
    qualifier permanent
    parameter name/type=$name/separator="="/prompt="Name"
    parameter value/type=$expression/prompt="Value"

syntax set_radix
    parameter radix/type=$any/prompt="Radix"

syntax set_trace

verb @/alias=include

verb include
    parameter file/type=$string

end
`

func TestExpressionGrammar(t *testing.T) {
	g, err := ParseGrammar(expressionTestGrammar)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		line   string
		active string
		values map[string]string
	}{
		{"EXAMINE 100 200", "EXAMINE", map[string]string{"START": "100", "END": "200"}},
		{"EXAMINE/BYTE X + 4", "EXAMINE", map[string]string{"START": "X + 4", "END": ""}},
		{"EXAMINE 100 /BYTE", "EXAMINE", map[string]string{"START": "100"}},
		{"EXAMINE 100/2 R0", "EXAMINE", map[string]string{"START": "100/2", "END": "R0"}},
		{"DEPOSIT X=5", "DEPOSIT", map[string]string{"TARGET": "X", "VALUE": "5"}},
		{"DEPOSIT X = 5 + 1", "DEPOSIT", map[string]string{"TARGET": "X", "VALUE": "5 + 1"}},
		{"DEPOSIT X 5", "DEPOSIT", map[string]string{"TARGET": "X", "VALUE": "5"}},
		{`DEPOSIT R0 "Ab"`, "DEPOSIT", map[string]string{"TARGET": "R0", "VALUE": `"Ab"`}},
		{"SET RADIX 16", "SET_RADIX", map[string]string{"RADIX": "16"}},
		{"SET R=5", "SET_SYMBOL", map[string]string{"NAME": "R", "VALUE": "5"}},
		{"SET RADIX = 2", "SET_SYMBOL", map[string]string{"NAME": "RADIX", "VALUE": "2"}},
		{"SET/PERMANENT PC = 200", "SET_SYMBOL", map[string]string{"NAME": "PC", "VALUE": "200"}},
		{"SET X=1 /PERMANENT", "SET_SYMBOL", map[string]string{"NAME": "X", "VALUE": "1"}},
		{"@FILE", "INCLUDE", map[string]string{"FILE": "FILE"}},
		{`@ "vax.init"`, "INCLUDE", map[string]string{"FILE": "vax.init"}},
	}

	for _, c := range cases {
		r, err := g.Parse(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.line, err)

			continue
		}

		if r.Active != c.active {
			t.Errorf("%s: active %s, want %s", c.line, r.Active, c.active)
		}

		for name, want := range c.values {
			if got := r.String(name); got != want {
				t.Errorf("%s: %s = %q, want %q", c.line, name, got, want)
			}
		}
	}

	r, err := g.Parse(`PRINT "x = ", A + 1, (B, C)`)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := r.List("ITEMS"), []string{`"x = "`, "A + 1", "(B, C)"}; !reflect.DeepEqual(got, want) {
		t.Errorf("PRINT items = %q, want %q", got, want)
	}

	r, err = g.Parse("SET/PERMANENT X=1")
	if err != nil {
		t.Fatal(err)
	}

	if !r.Present("PERMANENT") {
		t.Error("SET/PERMANENT X=1: PERMANENT not present")
	}

	r, err = g.Parse("SET NOTRACE")
	if err != nil {
		t.Fatal(err)
	}

	if r.Active != "SET_TRACE" || !r.Negated("WHAT") {
		t.Errorf("SET NOTRACE: active %s, negated %v", r.Active, r.Negated("WHAT"))
	}

	if _, err := g.Parse("SET NORADIX 10"); !errors.Is(err, vmserrors.New(vmserrors.CLI_NONEGATE)) {
		t.Errorf("SET NORADIX: error %v, want CLI_NONEGATE", err)
	}
}

func TestBadSeparator(t *testing.T) {
	_, err := ParseGrammar("grammar g\nverb v\nparameter p/separator=\"==\"\nend\n")
	if err == nil {
		t.Error("two-character separator accepted")
	}

	_, err = ParseGrammar("grammar g\nverb v/assignment=nowhere\nend\n")
	if err == nil {
		t.Error("assignment to a missing syntax accepted")
	}
}
