package format

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestVMSSyntax holds Parse to VMS's run of the probe (testdata/for80):
// each output case's format, put to FORRTL as a run-time format, was
// refused with SYNERRFOR (FOR number 62) or accepted, and Parse must
// refuse or accept it the same way.
func TestVMSSyntax(t *testing.T) {
	file, err := os.Open("../../../testdata/for80/vax/for80-vax.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var (
		caseLine = regexp.MustCompile(`^\|#(\d+) out (.*)\|$`)
		items    = regexp.MustCompile(`( [a-z]{1,2}=\S*)+$`)
		result   = regexp.MustCompile(`^\|=(\d+) r=\S+ e=(\d+) `)
		formats  = map[string]string{}
		checked  = 0
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if m := caseLine.FindStringSubmatch(line); m != nil {
			text := items.ReplaceAllString(m[2], "")
			if !strings.Contains(text, "..") {
				formats[m[1]] = text
			}

			continue
		}

		m := result.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		text, ok := formats[m[1]]
		if !ok {
			continue
		}

		delete(formats, m[1])

		_, err := Parse(text, Options{RunTime: true})
		refused := m[2] == "62"

		switch {
		case refused && err == nil:
			t.Errorf("case %s: %q was refused by VMS, and Parse accepts it", m[1], text)
		case !refused && err != nil:
			t.Errorf("case %s: %q was accepted by VMS, and Parse refuses it: %v", m[1], text, err)
		}

		checked++
	}

	if checked < 400 {
		t.Errorf("only %d cases checked", checked)
	}
}

// dump shows a format's items as a list: each code with its numbers,
// groups in parentheses.
func dump(items []*Item) string {
	parts := make([]string, 0, len(items))

	for _, it := range items {
		var b strings.Builder

		if it.Repeat.Set && it.Repeat.N != 1 || it.Repeat.VFE > 0 {
			b.WriteString(it.Repeat.String())
		}

		switch it.Code {
		case Group:
			b.WriteString("(" + dump(it.Items) + ")")
		case Text:
			b.WriteString(strconv.Quote(it.Text))
		case X, P:
			b.WriteString(it.N.String() + it.Code.String())
		case T, TL, TR:
			b.WriteString(it.Code.String() + it.N.String())
		default:
			b.WriteString(it.Code.String() + it.W.String())

			if it.M.Set || it.M.VFE > 0 {
				b.WriteString("." + it.M.String())
			}

			if it.D.Set || it.D.VFE > 0 {
				b.WriteString("." + it.D.String())
			}

			if it.E.Set || it.E.VFE > 0 {
				b.WriteString("E" + it.E.String())
			}
		}

		b.WriteString("@" + strconv.Itoa(it.Offset))
		parts = append(parts, b.String())
	}

	return strings.Join(parts, " ")
}

func TestParse(t *testing.T) {
	cases := []struct{ text, want string }{
		{"(I5, 2X, F8.2)", "I5@1 2X@5 F8.2@9"},
		{"( 1 X , I 3 )", "1X@2 I3@8"},
		{"(3I4.2,2(1X,E12.4E3),G)", "3I4.2@1 2(1X@9 E12.4E3@12)@7 G@21"},
		{"(1PE12.3,-2P,F8.2)", "1P@1 E12.3@3 -2P@9 F8.2@13"},
		{"(2P3I3)", "2P@1 3I3@3"},
		{"('IT''S',3HA'B,1X'C'I2)", `"IT'S"@1 "A'B"@9 1X@15 "C"@17 I2@20`},
		{"(1X,3HABC2HDE)", `1X@1 "ABC"@4 "DE"@9`},
		{"(I3/I3:$)", "I3@1 /@3 I3@4 :@6 $@7"},
		{"(T5,TL2,TR3,Q,S,SP,SS,BN,BZ)", "T5@1 TL2@4 TR3@8 Q@12 S@14 SP@16 SS@19 BN@22 BZ@25"},
		{"(X,P,I,O,Z,L,A,F,E,D)", "1X@1 0P@3 I@5 O@7 Z@9 L@11 A@13 F@15 E@17 D@19"},
		{"(1X,I<J+1>,<N>F8.<K>)", "1X@1 I<1>@4 <2>F8.<3>@11"},
		{"(i3,e8.2e2)", "I3@1 E8.2E2@4"},
		{"(,1X,,I3,)", "1X@2 I3@6"},
		{"(1X,())", "1X@1 ()@4"},
	}

	for _, c := range cases {
		f, err := Parse(c.text, Options{})
		if err != nil {
			t.Errorf("%q: %v", c.text, err)

			continue
		}

		if got := dump(f.Items); got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.text, got, c.want)
		}
	}
}

func TestVFEs(t *testing.T) {
	f, err := Parse("(I<J+1>, <N> F8.2)", Options{})
	if err != nil {
		t.Fatal(err)
	}

	want := []VFE{{"J+1", 3}, {"N", 10}}
	if len(f.VFEs) != len(want) || f.VFEs[0] != want[0] || f.VFEs[1] != want[1] {
		t.Errorf("got %v, want %v", f.VFEs, want)
	}
}

// TestErrors checks where an error is reported, and that a FORMAT
// statement, unlike a run-time format, may have nothing after its last
// parenthesis.
func TestErrors(t *testing.T) {
	cases := []struct {
		text   string
		offset int
		has    string
	}{
		{"(1X I3)", 4, "comma"},
		{"(1X,I3", 6, "no closing parenthesis"},
		{"(1X,I3)XYZ", 7, "follows it"},
		{"(1X,F8)", 6, "digits after the point"},
		{"(1X,J3)", 4, "not an edit descriptor"},
		{"(1X,'A)", 4, "no closing apostrophe"},
		{"(1X,0I3)", 4, "at least 1"},
		{"(1X,128PF8.2)", 4, "scale factor"},
		{"(1X,-1I3)", 4, "sign"},
		{"(1X,E12.4E256)", 10, "exponent"},
		{"(1X,G8.2E0)", 9, "exponent"},
		{"(1X,T)", 5, "column count"},
		{"(1X,3BN)", 4, "repeat count"},
		{"(1X,2'A')", 4, "repeat count"},
		{"(1X,0HA)", 4, "at least one"},
		{"(1X,<N>HA)", 4, "count must be a number"},
		{"(1X,'')", 4, "at least one"},
		{"((((((((((I3))))))))))", 9, "nested"},
		{"I3", 0, "start with ("},
	}

	for _, c := range cases {
		_, err := Parse(c.text, Options{})

		e, ok := err.(*Error)
		if !ok {
			t.Errorf("%q: got %v, want an error", c.text, err)

			continue
		}

		if e.Offset != c.offset || !strings.Contains(e.Text, c.has) {
			t.Errorf("%q: got %d %q, want %d and %q", c.text, e.Offset, e.Text, c.offset, c.has)
		}
	}

	if _, err := Parse("(1X,I3)XYZ", Options{RunTime: true}); err != nil {
		t.Errorf("a run-time format's trailing text: %v", err)
	}

	if _, err := Parse("(I<N>)", Options{RunTime: true}); err == nil {
		t.Error("a run-time format's variable format expression was accepted")
	}
}
