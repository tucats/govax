package dcl

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// negatedTestGrammar has a qualifier that takes a value and has no default.
const negatedTestGrammar = `
grammar test

verb help/id=1
    qualifier   library/id=2    -
                /type=$string
    parameter   topic/id=3      -
                /type=$rest_of_line

end
`

// TestParse_negatedTakesNoValue: as in DCL, a qualifier's /NO form takes
// no value (HELP/NOLIBRARY), while its plain form still needs one.
func TestParse_negatedTakesNoValue(t *testing.T) {
	g, err := ParseGrammar(negatedTestGrammar)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}

	r, err := g.Parse("HELP/NOLIBRARY ALPHA")
	if err != nil {
		t.Fatalf("HELP/NOLIBRARY: %v", err)
	}

	if !r.Present("LIBRARY") || !r.Negated("LIBRARY") || r.String("LIBRARY") != "" {
		t.Errorf("/NOLIBRARY: present %v, negated %v, value %q", r.Present("LIBRARY"), r.Negated("LIBRARY"), r.String("LIBRARY"))
	}

	if r.String("TOPIC") != "ALPHA" {
		t.Errorf("topic %q", r.String("TOPIC"))
	}

	if _, err := g.Parse("HELP/LIBRARY ALPHA"); !errors.Is(err, vmserrors.New(vmserrors.CLI_VALREQ)) {
		t.Errorf("HELP/LIBRARY: %v, want VALREQ", err)
	}
}
