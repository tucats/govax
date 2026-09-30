package dcl

import (
	"reflect"
	"testing"
)

// positionalTestGrammar is shaped after LINK: a list of input files, each
// of which can carry its own /LIBRARY, /INCLUDE=(...), and /OPTIONS, a
// /HOST that applies to the whole list, and command qualifiers.
const positionalTestGrammar = `
grammar test

verb link
    parameter files/id=1/type=$string/list/prompt="File"
    qualifier host/id=2/parameter=files
    qualifier library/id=3/parameter=files/placement=positional
    qualifier include/id=4/parameter=files/placement=positional/type=$string/list
    qualifier options/id=5/parameter=files/placement=positional
    qualifier map/id=6/type=$string/default=""
    qualifier brief/id=7

end
`

func TestPositionalQualifiers(t *testing.T) {
	g, err := ParseGrammar(positionalTestGrammar)
	if err != nil {
		t.Fatal(err)
	}

	type item struct {
		library, options bool
		include          []string
	}

	cases := []struct {
		line        string
		files       []string
		items       []item
		host, brief bool
		mapped      bool
	}{
		{"LINK MAIN", []string{"MAIN"}, []item{{}}, false, false, false},
		{"LINK MAIN,MYLIB/LIBRARY,OPTS/OPTIONS", []string{"MAIN", "MYLIB", "OPTS"},
			[]item{{}, {library: true}, {options: true}}, false, false, false},
		{"LINK MYLIB/LIBRARY/INCLUDE=(A,B), MAIN /MAP", []string{"MYLIB", "MAIN"},
			[]item{{library: true, include: []string{"A", "B"}}, {}}, false, false, true},
		{"LINK MAIN/MAP,LIB/INCLUDE=X/BRIEF", []string{"MAIN", "LIB"},
			[]item{{}, {include: []string{"X"}}}, false, true, true},
		{`LINK "a.obj","b.olb"/LIB/HOST`, []string{"a.obj", "b.olb"},
			[]item{{}, {library: true}}, true, false, false},
	}

	for _, c := range cases {
		r, err := g.Parse(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.line, err)

			continue
		}

		if got := r.List("FILES"); !reflect.DeepEqual(got, c.files) {
			t.Errorf("%s: files %q, want %q", c.line, got, c.files)
		}

		items := r.Items("FILES")
		if len(items) != len(c.items) {
			t.Errorf("%s: %d items, want %d", c.line, len(items), len(c.items))

			continue
		}

		for i, want := range c.items {
			got := item{library: items[i].Present("LIBRARY"), options: items[i].Present("OPTIONS"), include: items[i].List("INCLUDE")}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: item %d %+v, want %+v", c.line, i, got, want)
			}
		}

		if r.ParamPresent("FILES", "HOST") != c.host || r.Present("BRIEF") != c.brief || (r.Present("MAP") && !r.Defaulted("MAP")) != c.mapped {
			t.Errorf("%s: host %v brief %v map %v", c.line, r.ParamPresent("FILES", "HOST"), r.Present("BRIEF"), !r.Defaulted("MAP"))
		}
	}

	for _, bad := range []string{"LINK MAIN,/LIBRARY", "LINK MAIN/LIBRARY=X", "LINK MAIN/INCLUDE"} {
		if _, err := g.Parse(bad); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

func TestPositionalQualifierDefinitions(t *testing.T) {
	for _, text := range []string{
		"verb v\nqualifier q/placement=positional\nend\n",
		"verb v\nparameter p/type=$string\nqualifier q/parameter=p/placement=positional\nend\n",
		"verb v\nparameter p/type=$string/list\nqualifier q/parameter=p/placement=local\nend\n",
	} {
		if _, err := ParseGrammar(text); err == nil {
			t.Errorf("no error for:\n%s", text)
		}
	}
}
