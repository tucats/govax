package dcl

import (
	"path/filepath"
	"runtime"
	"testing"
)

// evaxGrammarPath locates internal/bootdata/files/evax.dcl relative to this
// source file, so tests work regardless of the package under test's working
// directory. This is the grammar govax actually parses at runtime (Phase
// 15's embedded-fallback mechanism) -- not testdata/dcl/evax.dcl, which
// stays a pure, untouched `git archive` import from the upstream C repo and
// has diverged from this file since Phase 22 added MOUNT/DISMOUNT (see
// docs/PHASE-22.md, "Grammar file" design decision).
func evaxGrammarPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "bootdata", "files", "evax.dcl")
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

	wantVerbs := []string{"DEFINE", "ABOUT", "FORTH", "EXIT", "QUIT", "TEST", "CALL", "CLEAR", "VMINIT", "SHOW", "MOUNT", "DISMOUNT"}
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

	if kw.Syntax != "SHOW_MEMORY" { //nolint:goconst
		t.Errorf("MEMORY keyword syntax = %q, want SHOW_MEMORY", kw.Syntax)
	}
}

func TestLoadEvaxGrammar_verbCount(t *testing.T) {
	g := loadEvaxGrammar(t)
	// define, about, forth, exit, quit, test, call, clear, show, vminit,
	// mount, dismount (the last two are a govax-native Phase 22 addition
	// with no testdata/dcl/evax.dcl counterpart).
	if len(g.verbOrder) != 12 {
		t.Errorf("got %d verbs, want 12: %v", len(g.verbOrder), verbNames(g))
	}
}

// TestLoadEvaxGrammar_mountDismount regresses Phase 22's MOUNT/DISMOUNT
// grammar addition (internal/bootdata/files/evax.dcl only -- see
// docs/PHASE-22.md's "Grammar file" design decision): both verbs parse,
// MOUNT takes a required DEVICE and FILE parameter plus an optional WRITE
// switch, and DISMOUNT takes just a required DEVICE parameter.
func TestLoadEvaxGrammar_mountDismount(t *testing.T) {
	g := loadEvaxGrammar(t)

	mount, ok := g.entries["MOUNT"]
	if !ok {
		t.Fatal("missing verb MOUNT")
	}

	if len(mount.Parameters) != 2 {
		t.Fatalf("MOUNT has %d parameters, want 2", len(mount.Parameters))
	}

	if mount.Parameters[0].Name != "DEVICE" || !mount.Parameters[0].required() {
		t.Errorf("MOUNT parameter 0 = %+v, want required DEVICE", mount.Parameters[0])
	}

	if mount.Parameters[1].Name != "FILE" || !mount.Parameters[1].required() {
		t.Errorf("MOUNT parameter 1 = %+v, want required FILE", mount.Parameters[1])
	}

	if _, _, err := mount.qualifier("WRITE"); err != nil {
		t.Errorf("MOUNT should have a WRITE qualifier: %v", err)
	}

	dismount, ok := g.entries["DISMOUNT"]
	if !ok {
		t.Fatal("missing verb DISMOUNT")
	}

	if len(dismount.Parameters) != 1 || dismount.Parameters[0].Name != "DEVICE" {
		t.Errorf("DISMOUNT parameters = %+v, want a single required DEVICE", dismount.Parameters)
	}
}

func verbNames(g *Grammar) []string {
	names := make([]string, len(g.verbOrder))
	for i, e := range g.verbOrder {
		names[i] = e.Name
	}

	return names
}
