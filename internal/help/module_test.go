package help

import (
	"reflect"
	"strings"
	"testing"
)

func records(lines ...string) [][]byte {
	out := make([][]byte, len(lines))
	for i, l := range lines {
		out[i] = []byte(l)
	}

	return out
}

func TestParseModuleTree(t *testing.T) {
	root := ParseModule(records(
		"text before the first key is dropped",
		"1 COPY",
		"   Text of COPY.",
		"2 Parameters",
		"   Text of COPY's parameters.",
		"2 Qualifiers",
		"   Before the qualifier lines.",
		"/LOG",
		"   Text of /LOG.",
		"/NOCONFIRM",
		"   Text of /NOCONFIRM.",
		" /INDENTED is text",
		"3 /SUB",
		"4 Deeper",
		"2\tTabbed",
		"2Nospace is text",
		"0 zero is text",
	))

	if root == nil || root.Key != "COPY" || root.Level != 1 {
		t.Fatalf("root %+v", root)
	}

	if want := []string{"   Text of COPY."}; !reflect.DeepEqual(root.Text, want) {
		t.Errorf("COPY's text %q", root.Text)
	}

	var keys []string
	for _, c := range root.Subtopics {
		keys = append(keys, c.Key)
	}

	if want := []string{"Parameters", "Qualifiers", "Tabbed"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("subtopics %q, want %q", keys, want)
	}

	q := root.Subtopics[1]
	if len(q.Text) != 6 || len(q.Slashes) != 2 {
		t.Fatalf("Qualifiers: %d lines, %d qualifier lines", len(q.Text), len(q.Slashes))
	}

	if s := q.Slashes[1]; s.Key != "/NOCONFIRM" || !s.Slash || s.Level != 3 ||
		!reflect.DeepEqual(s.Text, []string{"   Text of /NOCONFIRM.", " /INDENTED is text"}) {
		t.Errorf("/NOCONFIRM %+v", s)
	}

	if len(q.Subtopics) != 1 || q.Subtopics[0].Key != "/SUB" || q.Subtopics[0].Subtopics[0].Key != "Deeper" {
		t.Errorf("Qualifiers' subtopics %+v", q.Subtopics)
	}

	if tab := root.Subtopics[2]; len(tab.Text) != 2 {
		t.Errorf("Tabbed's text %q", tab.Text)
	}

	want := []Entry{{"Parameters", false}, {"Qualifiers", false}, {"/LOG", true}, {"/NOCONFIRM", true}, {"Tabbed", false}}
	if got := root.list(); !reflect.DeepEqual(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}

	if ParseModule(records("no key lines", "at all")) != nil {
		t.Error("a module with no key lines parsed")
	}
}

func TestSplit(t *testing.T) {
	cases := map[string][]string{
		"":                      nil,
		"  ALPHA   Parameters ": {"ALPHA", "Parameters"},
		"ALPHA/BRIEF":           {"ALPHA", "/BRIEF"},
		"ODD Slash/Inside":      {"ODD", "Slash", "/Inside"},
		"ALPHA /B/C":            {"ALPHA", "/B", "/C"},
		`"ALPHA Parameters" X`:  {`"ALPHA Parameters"`, "X"},
		`"A/B"`:                 {`"A/B"`},
		"ODD (":                 {"ODD"},
		"ODD @_(Paren_Name)":    {"ODD", "@_(Paren_Name)"},
		"A\tB":                  {"A", "B"},
	}

	for in, want := range cases {
		if got := Split(in); !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		word, key string
		want      bool
	}{
		{"alp", "ALPHA", true},
		{"ALPHA", "ALPHA", true},
		{"ALPHAS", "ALPHA", false},
		{"/B", "/BRIEF", true},
		{"/", "/BRIEF", false},
		{"", "ALPHA", false},
		{"/FLAG", "/[NO]FLAG", false},
		{"*", "/DIRECT", true},
		{"/*", "/DIRECT", true},
		{"AL%HA", "ALPHA", true},
		{"%%%%", "ALPHA", false},
		{"*S", "Parameters", true},
		{"L%", "L2", true},
		{"%", "L2", false},
		{"Two", "Two Words", true},
		{"Two_Words", "Two Words", false},
	}

	for _, c := range cases {
		if got := matches(c.word, c.key); got != c.want {
			t.Errorf("matches(%q, %q) = %v", c.word, c.key, got)
		}
	}
}

// TestListAt80 lays out lists at a terminal's 80 columns: seven cells of
// short keys to a line, at every depth's indentation.
func TestListAt80(t *testing.T) {
	var list []Entry
	for _, k := range strings.Fields("A1 A2 A3 A4 A5 A6 A7 A8 A9") {
		list = append(list, Entry{Key: k})
	}

	got := listLines(list, 1, 80)
	want := []string{
		"  A1         A2         A3         A4         A5         A6         A7",
		"  A8         A9",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestMemoryLibrary looks up topics in help text held in memory.
func TestMemoryLibrary(t *testing.T) {
	lib := NewMemoryLibrary([]string{
		"1 ZETA", "   Text of ZETA.",
		"1 alpha", "   Text of alpha.", "2 Sub", "   Text of alpha Sub.",
	})

	if got := lib.Names(); !reflect.DeepEqual(got, []string{"alpha", "ZETA"}) {
		t.Fatalf("names %q", got)
	}

	d, err := Lookup(lib, Split("ALPHA SUB"), Options{})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"", "ALPHA", "", "  Sub", "", "       Text of alpha Sub.", ""}
	if got := Render(d, 80); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}

	if _, err := Lookup(lib, Split("ALPHA,SUB"), Options{}); err != ErrInvalidKey {
		t.Errorf("a comma in the first word: %v", err)
	}
}
