package authorize

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
)

// uafHelp opens govax's own UAFHELP.HLB.
func uafHelp(t *testing.T) help.Library {
	t.Helper()

	data, err := fs.ReadFile(bootdata.FS, bootdata.UAFHelpLibrary)
	if err != nil {
		t.Fatal(err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	return help.NewLBRLibrary(l)
}

// TestEveryAuthorizeQualifierHasHelp: each of AUTHORIZE's commands has a
// module in UAFHELP.HLB, and each of its qualifiers a qualifier line
// there (docs/planning/PHASE-95 - authentication.md, Decision 13), as
// DCL's verbs have in HELPLIB.HLB.
func TestEveryAuthorizeQualifierHasHelp(t *testing.T) {
	lib := uafHelp(t)

	g, err := loadGrammar()
	if err != nil {
		t.Fatal(err)
	}

	verbs, alias := g.Verbs()

	for i, verb := range verbs {
		if alias[i] {
			continue
		}

		m := lib.Module(verb)
		if m == nil {
			t.Errorf("%s has no module in UAFHELP.HLB", verb)

			continue
		}

		keys := map[string]bool{}

		var walk func(*help.Topic)

		walk = func(tp *help.Topic) {
			if strings.HasPrefix(tp.Key, "/") {
				keys[strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(tp.Key, "/"), "[NO]"))] = true
			}

			for _, s := range tp.Slashes {
				walk(s)
			}

			for _, s := range tp.Subtopics {
				walk(s)
			}
		}

		walk(m)

		for _, q := range g.Qualifiers(verb) {
			if !keys[strings.ToUpper(q)] {
				t.Errorf("%s/%s has no qualifier line in its help", verb, q)
			}
		}
	}
}
