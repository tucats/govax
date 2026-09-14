package dcl

import (
	"path/filepath"
	"runtime"
	"testing"
)

// evaxGrammarPath locates testdata/dcl/evax.dcl relative to this source
// file, so tests work regardless of the package under test's working
// directory.
func evaxGrammarPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "dcl", "evax.dcl")
}

func loadEvaxGrammar(t *testing.T) *Grammar {
	t.Helper()
	g, err := LoadGrammarFile(evaxGrammarPath(t))
	if err != nil {
		t.Fatalf("LoadGrammarFile: %v", err)
	}
	return g
}

func TestLoadEvaxGrammar(t *testing.T) {
	g := loadEvaxGrammar(t)

	if g.Name != "EVAX" {
		t.Errorf("grammar name = %q, want EVAX", g.Name)
	}

	wantVerbs := []string{"DEFINE", "ABOUT", "FORTH", "EXIT", "QUIT", "TEST", "CALL", "CLEAR", "VMINIT", "SHOW"}
	for _, v := range wantVerbs {
		if _, ok := g.entries[v]; !ok {
			t.Errorf("missing verb %s", v)
		}
	}

	quit := g.entries["QUIT"]
	if quit.aliasRef == nil || quit.aliasRef.Name != "EXIT" {
		t.Errorf("QUIT alias not resolved to EXIT: %+v", quit)
	}

	show := g.entries["SHOW"]
	if len(show.Parameters) != 1 {
		t.Fatalf("SHOW has %d parameters, want 1", len(show.Parameters))
	}
	if show.Parameters[0].Type != TypeKeyword || show.Parameters[0].TypeName != "SHOW_TYPES" {
		t.Errorf("SHOW parameter = %+v", show.Parameters[0])
	}
	if show.Parameters[0].typeRef == nil {
		t.Error("SHOW parameter type not resolved")
	}

	showMemory, ok := g.entries["SHOW_MEMORY"]
	if !ok {
		t.Fatal("missing syntax SHOW_MEMORY")
	}
	if _, _, err := showMemory.qualifier("FULL"); err != nil {
		t.Errorf("SHOW_MEMORY should have a FULL qualifier: %v", err)
	}

	// Sanity check a keyword-driven redirect resolved during validate().
	showTypes := g.types["SHOW_TYPES"]
	kw, _, err := showTypes.lookup("MEMORY")
	if err != nil {
		t.Fatalf("lookup MEMORY keyword: %v", err)
	}
	if kw.Syntax != "SHOW_MEMORY" {
		t.Errorf("MEMORY keyword syntax = %q, want SHOW_MEMORY", kw.Syntax)
	}
}

func TestLoadEvaxGrammar_verbCount(t *testing.T) {
	g := loadEvaxGrammar(t)
	// define, about, forth, exit, quit, test, call, clear, show, vminit
	if len(g.verbOrder) != 10 {
		t.Errorf("got %d verbs, want 10: %v", len(g.verbOrder), verbNames(g))
	}
}

func verbNames(g *Grammar) []string {
	names := make([]string, len(g.verbOrder))
	for i, e := range g.verbOrder {
		names[i] = e.Name
	}
	return names
}
