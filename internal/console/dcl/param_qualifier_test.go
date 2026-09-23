package dcl

import "testing"

// paramQualifierTestGrammar is a small, self-contained grammar (not
// internal/bootdata/files/evax.dcl) built specifically to exercise Phase
// 23 subtask 2's parameter-scoped qualifier feature — the DCL engine
// capability COPY's own /HOST qualifier will need once it's implemented
// (docs/PHASE-23.md's "COPY direction and the /HOST qualifier" design
// section), but which no real command uses yet. Shaped after COPY's own
// two-parameter design: SOURCE and DESTINATION each declare their own
// private HOST qualifier via /parameter=, and BINARY is an ordinary
// entry-level qualifier alongside them, so tests can check that an
// unrelated qualifier name still falls through to entry-level matching
// exactly as it always has.
const paramQualifierTestGrammar = `
grammar test

verb copy
    parameter source/id=1/type=$string/prompt="Source"
    qualifier host/id=2/parameter=source

    parameter destination/id=3/type=$string/prompt="Destination"
    qualifier host/id=4/parameter=destination

    qualifier binary/id=5

end
`

// loadParamQualifierTestGrammar parses paramQualifierTestGrammar, failing
// the test immediately if the grammar text itself doesn't parse — every
// test below assumes it's valid.
func loadParamQualifierTestGrammar(t *testing.T) *Grammar {
	t.Helper()

	g, err := ParseGrammar(paramQualifierTestGrammar)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}

	return g
}

// TestLoadParamQualifierGrammar_structure checks that /parameter= attached
// each HOST qualifier statement to the right Parameter's own Qualifiers
// list -- not to the enclosing "copy" Entry's list -- while BINARY (no
// /parameter= switch) landed on the entry as usual.
func TestLoadParamQualifierGrammar_structure(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	copyEntry, ok := g.entries["COPY"]
	if !ok {
		t.Fatal("missing verb COPY")
	}

	if len(copyEntry.Parameters) != 2 {
		t.Fatalf("COPY has %d parameters, want 2", len(copyEntry.Parameters))
	}

	source, destination := copyEntry.Parameters[0], copyEntry.Parameters[1]

	if len(source.Qualifiers) != 1 || source.Qualifiers[0].Name != "HOST" {
		t.Errorf("SOURCE.Qualifiers = %+v, want a single HOST qualifier", source.Qualifiers)
	}

	if len(destination.Qualifiers) != 1 || destination.Qualifiers[0].Name != "HOST" {
		t.Errorf("DESTINATION.Qualifiers = %+v, want a single HOST qualifier", destination.Qualifiers)
	}

	// The two HOST qualifiers are distinct objects (different IDs), not the
	// same Qualifier shared by reference between the two parameters.
	if source.Qualifiers[0] == destination.Qualifiers[0] {
		t.Error("SOURCE and DESTINATION should not share the same Qualifier instance")
	}

	if source.Qualifiers[0].ID != 2 || destination.Qualifiers[0].ID != 4 {
		t.Errorf("HOST qualifier IDs = %d/%d, want 2/4", source.Qualifiers[0].ID, destination.Qualifiers[0].ID)
	}

	// BINARY has no /parameter= switch, so it's still an ordinary
	// entry-level qualifier, exactly like every qualifier before this
	// feature existed.
	if len(copyEntry.Qualifiers) != 1 || copyEntry.Qualifiers[0].Name != "BINARY" {
		t.Errorf("COPY.Qualifiers = %+v, want a single entry-level BINARY qualifier", copyEntry.Qualifiers)
	}
}

// TestParseGrammar_qualifierParameterNotFound checks that /parameter=
// naming a parameter that was never declared on the current entry is a
// grammar-definition-time error (CLI_PARAMNOTFOUND), not something that
// silently falls back to entry-level or is only discovered later while
// parsing a command line.
func TestParseGrammar_qualifierParameterNotFound(t *testing.T) {
	const bad = `
grammar test

verb copy
    parameter source/id=1/type=$string
    qualifier host/id=2/parameter=nosuchparam

end
`

	if _, err := ParseGrammar(bad); err == nil {
		t.Error("expected an error for /parameter= naming an undeclared parameter")
	}
}

// TestParse_paramScopedQualifier_attachedNoSpace checks the plainest case
// from docs/PHASE-23.md's COPY examples: "COPY foo.txt/HOST BAR.TXT",
// where /HOST is written directly against its parameter's value with no
// intervening space. It must land on SOURCE's own HOST, not DESTINATION's,
// and not on any flat, unscoped "HOST" value either.
func TestParse_paramScopedQualifier_attachedNoSpace(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY foo.txt/HOST BAR.TXT`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.ParamPresent("SOURCE", "HOST") {
		t.Error("expected SOURCE/HOST present")
	}

	if r.ParamPresent("DESTINATION", "HOST") {
		t.Error("expected DESTINATION/HOST absent")
	}

	if r.Present("HOST") {
		t.Error("a parameter-scoped qualifier must not also appear in the flat entry-level map")
	}
}

// TestParse_paramScopedQualifier_attachedWithSpace checks that a
// parameter-scoped qualifier is recognized even when whitespace separates
// it from its parameter's value ("COPY FOO.TXT /HOST BAR.TXT") -- the
// design doc's own "the qualifier appears somewhere after that parameter's
// value and before the next parameter begins" rule, not a stricter
// no-whitespace rule.
func TestParse_paramScopedQualifier_attachedWithSpace(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY FOO.TXT /HOST BAR.TXT`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.ParamPresent("SOURCE", "HOST") {
		t.Error("expected SOURCE/HOST present")
	}

	if r.ParamPresent("DESTINATION", "HOST") {
		t.Error("expected DESTINATION/HOST absent")
	}
}

// TestParse_paramScopedQualifier_secondParameter mirrors the no-space case
// above but with /HOST trailing the *second* parameter instead of the
// first ("COPY FOO.TXT BAR.TXT/HOST"), checking that the "most recently
// filled parameter" tracking correctly moved on from SOURCE to
// DESTINATION in between.
func TestParse_paramScopedQualifier_secondParameter(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY FOO.TXT BAR.TXT/HOST`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.ParamPresent("SOURCE", "HOST") {
		t.Error("expected SOURCE/HOST absent")
	}

	if !r.ParamPresent("DESTINATION", "HOST") {
		t.Error("expected DESTINATION/HOST present")
	}
}

// TestParse_paramScopedQualifier_fallsThroughToEntryLevel checks that a
// qualifier name absent from the most-recently-filled parameter's own list
// (BINARY isn't declared under SOURCE at all) correctly falls through to
// the ordinary entry-level lookup, unaffected by a parameter having just
// been filled in.
func TestParse_paramScopedQualifier_fallsThroughToEntryLevel(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY FOO.TXT/BINARY BAR.TXT`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.Present("BINARY") {
		t.Error("expected entry-level BINARY qualifier present")
	}

	if r.ParamPresent("SOURCE", "BINARY") {
		t.Error("BINARY was never declared under SOURCE, so it must not appear there")
	}
}

// TestParse_paramScopedQualifier_negated checks that the automatic
// "NO"-prefix negation rule (matchQualifier, already exercised for
// entry-level qualifiers by TestParse_negatedQualifier) also applies when
// the match is against a parameter's own qualifier list.
func TestParse_paramScopedQualifier_negated(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY FOO.TXT/NOHOST BAR.TXT`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.ParamPresent("SOURCE", "HOST") || !r.ParamNegated("SOURCE", "HOST") {
		t.Errorf("expected SOURCE/HOST present+negated, got present=%v negated=%v",
			r.ParamPresent("SOURCE", "HOST"), r.ParamNegated("SOURCE", "HOST"))
	}
}

// TestParse_paramScopedQualifier_bothParameters checks that COPY's two
// independent HOST occurrences can both be present at once, each recorded
// under its own parameter -- the actual host-to-container-to-host round
// trip shape wouldn't make sense for a real COPY, but nothing in the
// engine itself should stop both from being set on one command line.
func TestParse_paramScopedQualifier_bothParameters(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY FOO.TXT/HOST BAR.TXT/HOST`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.ParamPresent("SOURCE", "HOST") {
		t.Error("expected SOURCE/HOST present")
	}

	if !r.ParamPresent("DESTINATION", "HOST") {
		t.Error("expected DESTINATION/HOST present")
	}
}

// TestParse_paramScopedQualifier_quotedValueThenQualifier regresses the
// design doc's "Quoting" section against the new parameter-scoped-qualifier
// code path specifically: a quoted parameter value that itself contains a
// '/' (e.g. a host path) must not confuse the parser into reading part of
// the quoted string as a qualifier, and a real qualifier following it
// (space-separated, since the value needed quoting in the first place) must
// still resolve correctly against that parameter's own list.
func TestParse_paramScopedQualifier_quotedValueThenQualifier(t *testing.T) {
	g := loadParamQualifierTestGrammar(t)

	r, err := g.Parse(`COPY "/Users/tom/foo.txt" /HOST BAR.TXT`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := r.String("SOURCE"); got != "/Users/tom/foo.txt" {
		t.Errorf("SOURCE=%q, want \"/Users/tom/foo.txt\" (case preserved, embedded '/' intact)", got)
	}

	if !r.ParamPresent("SOURCE", "HOST") {
		t.Error("expected SOURCE/HOST present")
	}
}

// TestLoadEvaxGrammar_unaffectedByParamQualifierFeature is Phase 23 subtask
// 2's regression check that the real, full evax.dcl grammar file -- which
// declares no parameter-scoped qualifiers at all -- parses identically to
// before this feature was added: every qualifier in it still ends up
// exactly where it always did, on its enclosing entry, never misattributed
// to whatever parameter happened to precede it in the file.
func TestLoadEvaxGrammar_unaffectedByParamQualifierFeature(t *testing.T) {
	g := loadEvaxGrammar(t)

	for _, e := range g.entries {
		for _, p := range e.Parameters {
			if len(p.Qualifiers) != 0 {
				t.Errorf("%s parameter %s unexpectedly has parameter-scoped qualifiers %+v (evax.dcl declares none)",
					e.Name, p.Name, p.Qualifiers)
			}
		}
	}

	// Spot check: DEFINE/DEVICE's CLUSTER-style run of qualifiers
	// immediately following its required NAME parameter (evax.dcl lines
	// ~62-83) must still be entry-level, not accidentally scoped to NAME,
	// confirming plain statement adjacency was never the signal.
	defineDevice, ok := g.entries["DEFINE_DEVICE"]
	if !ok {
		t.Fatal("missing syntax DEFINE_DEVICE")
	}

	if _, _, err := defineDevice.qualifier("CLUSTER"); err != nil {
		t.Errorf("DEFINE_DEVICE should have an entry-level CLUSTER qualifier: %v", err)
	}

	if len(defineDevice.Parameters) == 0 || len(defineDevice.Parameters[0].Qualifiers) != 0 {
		t.Error("DEFINE_DEVICE's NAME parameter should have no parameter-scoped qualifiers of its own")
	}
}
