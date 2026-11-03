package fortran

import (
	"slices"
	"strings"
	"testing"
)

// TestDebugKeywords holds what each form of /DEBUG means, next to MACRO's.
func TestDebugKeywords(t *testing.T) {
	for _, c := range []struct {
		keywords []string
		want     Debug
	}{
		{nil, Debug{Symbols: true}},
		{[]string{"ALL"}, Debug{Symbols: true}},
		{[]string{"NONE"}, Debug{NoTraceback: true}},
		{[]string{"SYMBOLS"}, Debug{Symbols: true, NoTraceback: true}},
		{[]string{"TRACEBACK"}, Debug{}},
		{[]string{"SYMBOLS", "TRACEBACK"}, Debug{Symbols: true}},
		{[]string{"SYM", "TRACE"}, Debug{Symbols: true}},
		{[]string{"NOTRACEBACK"}, Debug{NoTraceback: true}},
		{[]string{"NOSYMBOLS"}, Debug{}},
		{[]string{"ALL", "NOTRACEBACK"}, Debug{Symbols: true, NoTraceback: true}},
		{[]string{"ALL", "NOSYMBOLS"}, Debug{}},
	} {
		got, err := Debug{}.With(c.keywords)
		if err != nil || got != c.want {
			t.Errorf("%v: %+v, %v, want %+v", c.keywords, got, err, c.want)
		}
	}

	if _, err := (Debug{}).With([]string{"LINES"}); err == nil {
		t.Error("LINES was taken as a keyword")
	}
}

// TestDebugDirectives holds what the MACRO file says for each setting, and the remark that
// /DEBUG with /OPTIMIZE gets, which doesn't make the compilation a warning.
func TestDebugDirectives(t *testing.T) {
	directives := func(r Result) []string {
		var found []string

		for _, line := range r.MACRO {
			if f := strings.Fields(line); len(f) > 0 && (f[0] == ".ENABLE" || f[0] == ".DISABLE") {
				found = append(found, f[0]+" "+f[1])
			}
		}

		return found
	}

	source := File{Spec: "T.FOR", Name: "T", Lines: []string{"      I = 1", "      END"}}

	for _, c := range []struct {
		debug Debug
		want  []string
	}{
		{Debug{}, nil},
		{Debug{Symbols: true}, []string{".ENABLE DEBUG"}},
		{Debug{NoTraceback: true}, []string{".DISABLE TRACEBACK"}},
		{Debug{Symbols: true, NoTraceback: true}, []string{".ENABLE DEBUG", ".DISABLE TRACEBACK"}},
	} {
		r := Compile(Options{Debug: c.debug}, source)
		if got := directives(r); !slices.Equal(got, c.want) {
			t.Errorf("%+v: %q, want %q", c.debug, got, c.want)
		}

		if len(r.Messages) != 0 {
			t.Errorf("%+v: %v", c.debug, r.Messages)
		}
	}

	r := Compile(Options{Debug: Debug{Symbols: true}, Optimize: true}, source)
	if len(r.Messages) != 1 || r.Messages[0].Ident != "DEBUGOPTIMIZE" || r.Warnings != 0 {
		t.Errorf("under /OPTIMIZE: %v, %d warnings", r.Messages, r.Warnings)
	}
}
