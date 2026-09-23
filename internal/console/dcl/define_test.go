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

	wantVerbs := []string{"DEFINE", "ABOUT", "FORTH", "EXIT", "QUIT", "TEST", "CALL", "CLEAR", "VMINIT", "SHOW", "MOUNT", "DISMOUNT", "INITIALIZE", "DIRECTORY", "DELETE", "PURGE", "TYPE"}
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
	// mount, dismount, initialize, directory, delete, purge, type, copy
	// (the last eight are govax-native additions -- Phase 22 for mount/
	// dismount, Phase 23 for initialize/directory/delete/purge/type/copy --
	// with no testdata/dcl/evax.dcl counterpart).
	if len(g.verbOrder) != 18 {
		t.Errorf("got %d verbs, want 18: %v", len(g.verbOrder), verbNames(g))
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

// TestLoadEvaxGrammar_initializeVaxContainer regresses Phase 23 subtask 1's
// INITIALIZE grammar addition: the bare verb has no handler of its own and
// redirects via /VAX and /CONTAINER to two separate syntaxes -- see
// docs/PHASE-23.md's "INITIALIZE: unifying INIT and INITIALIZE under one
// verb" design section.
func TestLoadEvaxGrammar_initializeVaxContainer(t *testing.T) {
	g := loadEvaxGrammar(t)

	initialize, ok := g.entries["INITIALIZE"]
	if !ok {
		t.Fatal("missing verb INITIALIZE")
	}

	if len(initialize.Parameters) != 0 {
		t.Errorf("INITIALIZE has %d parameters, want 0 (it never has its own handler)", len(initialize.Parameters))
	}

	vaxQual, _, err := initialize.qualifier("VAX")
	if err != nil {
		t.Fatalf("INITIALIZE should have a VAX qualifier: %v", err)
	}

	if vaxQual.Syntax != "INITIALIZE_VAX" {
		t.Errorf("VAX qualifier syntax = %q, want INITIALIZE_VAX", vaxQual.Syntax)
	}

	containerQual, _, err := initialize.qualifier("CONTAINER")
	if err != nil {
		t.Fatalf("INITIALIZE should have a CONTAINER qualifier: %v", err)
	}

	if containerQual.Syntax != "INITIALIZE_CONTAINER" {
		t.Errorf("CONTAINER qualifier syntax = %q, want INITIALIZE_CONTAINER", containerQual.Syntax)
	}

	initVax, ok := g.entries["INITIALIZE_VAX"]
	if !ok {
		t.Fatal("missing syntax INITIALIZE_VAX")
	}

	if len(initVax.Parameters) != 1 || initVax.Parameters[0].Name != "PAGES" {
		t.Fatalf("INITIALIZE_VAX parameters = %+v, want a single PAGES parameter", initVax.Parameters)
	}

	if initVax.Parameters[0].Type != TypeRestOfLine {
		t.Errorf("PAGES type = %v, want TypeRestOfLine", initVax.Parameters[0].Type)
	}

	if initVax.Parameters[0].required() {
		t.Error("PAGES should not be formally required (no /prompt=) -- INITIALIZE_VAX's handler checks Present itself, see docs/PHASE-23.md")
	}

	initContainer, ok := g.entries["INITIALIZE_CONTAINER"]
	if !ok {
		t.Fatal("missing syntax INITIALIZE_CONTAINER")
	}

	if len(initContainer.Parameters) != 3 {
		t.Fatalf("INITIALIZE_CONTAINER has %d parameters, want 3 (PATH, SIZE, LABEL)", len(initContainer.Parameters))
	}

	if _, _, err := initContainer.qualifier("CLUSTER"); err != nil {
		t.Errorf("INITIALIZE_CONTAINER should have a CLUSTER qualifier: %v", err)
	}
}

// TestLoadEvaxGrammar_showDefault regresses Phase 23 subtask 3's SHOW
// DEFAULT grammar addition: SHOW_TYPES gained a DEFAULT keyword redirecting
// to a new show_default syntax with no parameters/qualifiers of its own
// (unlike SHOW MEMORY or SHOW BREAK, DEFAULT has nothing to qualify --
// see docs/PHASE-23.md's design section).
// TestLoadEvaxGrammar_directory regresses Phase 23 subtask 5's DIRECTORY
// grammar addition: a single, optional SPEC parameter (no /prompt=, since
// an omitted file spec is a normal way to run DIRECTORY, not a missing
// argument) plus the four FULL/FILE/SIZE/DATE switch qualifiers.
func TestLoadEvaxGrammar_directory(t *testing.T) {
	g := loadEvaxGrammar(t)

	directory, ok := g.entries["DIRECTORY"]
	if !ok {
		t.Fatal("missing verb DIRECTORY")
	}

	if len(directory.Parameters) != 1 || directory.Parameters[0].Name != "SPEC" {
		t.Fatalf("DIRECTORY parameters = %+v, want a single SPEC parameter", directory.Parameters)
	}

	if directory.Parameters[0].required() {
		t.Error("SPEC should not be formally required (no /prompt=) -- a bare DIRECTORY lists the current default directory, see docs/PHASE-23.md")
	}

	for _, name := range []string{"FULL", "FILE", "SIZE", "DATE"} {
		if _, _, err := directory.qualifier(name); err != nil {
			t.Errorf("DIRECTORY should have a %s qualifier: %v", name, err)
		}
	}
}

// TestLoadEvaxGrammar_delete regresses Phase 23 subtask 6's DELETE grammar
// addition: a single, formally required SPEC parameter (/prompt=, since --
// unlike DIRECTORY's SPEC -- a bare DELETE has no sensible "delete
// everything in the current directory" default to fall back to).
func TestLoadEvaxGrammar_delete(t *testing.T) {
	g := loadEvaxGrammar(t)

	del, ok := g.entries["DELETE"]
	if !ok {
		t.Fatal("missing verb DELETE")
	}

	if len(del.Parameters) != 1 || del.Parameters[0].Name != "SPEC" {
		t.Fatalf("DELETE parameters = %+v, want a single SPEC parameter", del.Parameters)
	}

	if !del.Parameters[0].required() {
		t.Error("SPEC should be formally required (/prompt=) -- a bare DELETE has no sensible default, see docs/PHASE-23.md")
	}
}

// TestLoadEvaxGrammar_purge regresses Phase 23 subtask 7's PURGE grammar
// addition: an optional SPEC parameter (no /prompt=, matching DIRECTORY's
// own "a bare verb has a sensible default" SPEC, not DELETE's required
// one) plus an optional, value-taking LIMIT qualifier.
func TestLoadEvaxGrammar_purge(t *testing.T) {
	g := loadEvaxGrammar(t)

	purge, ok := g.entries["PURGE"]
	if !ok {
		t.Fatal("missing verb PURGE")
	}

	if len(purge.Parameters) != 1 || purge.Parameters[0].Name != "SPEC" {
		t.Fatalf("PURGE parameters = %+v, want a single SPEC parameter", purge.Parameters)
	}

	if purge.Parameters[0].required() {
		t.Error("SPEC should not be formally required (no /prompt=) -- a bare PURGE defaults to *.*, see docs/PHASE-23.md")
	}

	if _, _, err := purge.qualifier("LIMIT"); err != nil {
		t.Errorf("PURGE should have a LIMIT qualifier: %v", err)
	}
}

// TestLoadEvaxGrammar_type regresses Phase 23 subtask 8's TYPE grammar
// addition: a single, formally required SPEC parameter (/prompt=, matching
// DELETE's own required SPEC, not DIRECTORY's/PURGE's optional one -- there
// is no sensible "type everything" default).
func TestLoadEvaxGrammar_type(t *testing.T) {
	g := loadEvaxGrammar(t)

	typ, ok := g.entries["TYPE"]
	if !ok {
		t.Fatal("missing verb TYPE")
	}

	if len(typ.Parameters) != 1 || typ.Parameters[0].Name != "SPEC" {
		t.Fatalf("TYPE parameters = %+v, want a single SPEC parameter", typ.Parameters)
	}

	if !typ.Parameters[0].required() {
		t.Error("SPEC should be formally required (/prompt=) -- a bare TYPE has no sensible default, see docs/PHASE-23.md")
	}
}

// TestLoadEvaxGrammar_copy regresses Phase 23 subtask 9's COPY grammar
// addition: two formally required parameters, SOURCE and DESTINATION
// (/prompt= on both -- there is no sensible default for either half of a
// copy), each carrying its own private HOST qualifier attached via
// /parameter= (internal/console/dcl's parameter-scoped-qualifier feature,
// subtask 2) rather than one qualifier shared at the entry level.
func TestLoadEvaxGrammar_copy(t *testing.T) {
	g := loadEvaxGrammar(t)

	cp, ok := g.entries["COPY"]
	if !ok {
		t.Fatal("missing verb COPY")
	}

	if len(cp.Parameters) != 2 {
		t.Fatalf("COPY has %d parameters, want 2", len(cp.Parameters))
	}

	source, destination := cp.Parameters[0], cp.Parameters[1]

	if source.Name != "SOURCE" || !source.required() {
		t.Errorf("COPY parameter 0 = %+v, want required SOURCE", source)
	}

	if destination.Name != "DESTINATION" || !destination.required() {
		t.Errorf("COPY parameter 1 = %+v, want required DESTINATION", destination)
	}

	if len(source.Qualifiers) != 1 || source.Qualifiers[0].Name != "HOST" {
		t.Errorf("SOURCE.Qualifiers = %+v, want a single HOST qualifier", source.Qualifiers)
	}

	if len(destination.Qualifiers) != 1 || destination.Qualifiers[0].Name != "HOST" {
		t.Errorf("DESTINATION.Qualifiers = %+v, want a single HOST qualifier", destination.Qualifiers)
	}

	if len(cp.Qualifiers) != 0 {
		t.Errorf("COPY has %d entry-level qualifiers, want 0 (HOST is parameter-scoped on both SOURCE and DESTINATION, not entry-level)", len(cp.Qualifiers))
	}
}

func TestLoadEvaxGrammar_showDefault(t *testing.T) {
	g := loadEvaxGrammar(t)

	showTypes := g.types["SHOW_TYPES"]

	kw, _, err := showTypes.lookup("DEFAULT")
	if err != nil {
		t.Fatalf("lookup DEFAULT keyword: %v", err)
	}

	if kw.Syntax != "SHOW_DEFAULT" {
		t.Errorf("DEFAULT keyword syntax = %q, want SHOW_DEFAULT", kw.Syntax)
	}

	showDefault, ok := g.entries["SHOW_DEFAULT"]
	if !ok {
		t.Fatal("missing syntax SHOW_DEFAULT")
	}

	if len(showDefault.Parameters) != 0 || len(showDefault.Qualifiers) != 0 {
		t.Errorf("SHOW_DEFAULT has parameters=%+v qualifiers=%+v, want none",
			showDefault.Parameters, showDefault.Qualifiers)
	}
}

func verbNames(g *Grammar) []string {
	names := make([]string, len(g.verbOrder))
	for i, e := range g.verbOrder {
		names[i] = e.Name
	}

	return names
}
