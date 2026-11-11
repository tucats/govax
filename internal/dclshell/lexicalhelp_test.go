package dclshell

import (
	"io/fs"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
)

// TestEveryLexicalHasHelp: each lexical function DCL knows by name
// (lexicalNames, govax's F$CONFIG and the ones it lacks among them) has
// a topic of its own under HELPLIB.HLB's Lexicals (docs/planning/PHASE-90 - VMS
// help.md, Decision 11), keyed by its whole name.
func TestEveryLexicalHasHelp(t *testing.T) {
	data, err := fs.ReadFile(bootdata.FS, bootdata.HelpLibrary)
	if err != nil {
		t.Fatal(err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	lexicals := help.NewLBRLibrary(l).Module("Lexicals")
	if lexicals == nil {
		t.Fatal("HELPLIB.HLB has no Lexicals module")
	}

	topics := map[string]bool{}
	for _, sub := range lexicals.Subtopics {
		topics[sub.Key] = true
	}

	for _, name := range lexicalNames {
		if !topics[name] {
			t.Errorf("Lexicals has no topic %s", name)
		}

		delete(topics, name)
	}

	for key := range topics {
		t.Errorf("Lexicals' topic %s is no lexical function", key)
	}
}
