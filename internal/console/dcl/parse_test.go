package dcl

import "testing"

// TestResult_keywordValueDiscriminator regresses a bug found while wiring
// Phase 09's DEFINE/DEVICE command: a keyword-typed qualifier's matched
// value used to be indistinguishable, internally, from a plain string
// value (both set isString true with no further tag), so Int() — which is
// documented to return a keyword's matched ID — always returned 0 for one,
// and String()/Keyword() didn't cleanly separate "a real string" from "a
// keyword's display name" either. DEVCLASS (testdata/dcl/evax.dcl's
// define_device syntax) is a real keyword-typed qualifier (type
// dev_class), giving a concrete case to check all three accessors against.
func TestResult_keywordValueDiscriminator(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse(`DEFINE/DEVICE DKA0/DEVCLASS=DISK`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := r.Int("DEVCLASS"); got != 1 {
		t.Errorf(`Int("DEVCLASS") = %d, want 1 (dev_class's disk keyword ID)`, got)
	}

	if got := r.Keyword("DEVCLASS"); got != "DISK" {
		t.Errorf(`Keyword("DEVCLASS") = %q, want "DISK"`, got)
	}

	if got := r.String("DEVCLASS"); got != "" {
		t.Errorf(`String("DEVCLASS") = %q, want "" (DEVCLASS is a keyword value, not a plain string)`, got)
	}

	// A genuine string-typed field must be unaffected by the fix.
	if got := r.String("NAME"); got != "DKA0" {
		t.Errorf(`String("NAME") = %q, want "DKA0"`, got)
	}

	if got := r.Keyword("NAME"); got != "" {
		t.Errorf(`Keyword("NAME") = %q, want "" (NAME is a plain string, not a keyword)`, got)
	}
}

func TestParse_showRegisters(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("SHOW REG")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Verb != "SHOW" || r.Active != "SHOW_REG" {
		t.Errorf("Verb=%s Active=%s, want SHOW/SHOW_REG", r.Verb, r.Active)
	}
}

func TestParse_showMemoryFull(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("SHOW MEMORY/FULL")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Active != "SHOW_MEMORY" {
		t.Errorf("Active=%s, want SHOW_MEMORY", r.Active)
	}

	if !r.Present("FULL") {
		t.Error("expected FULL qualifier present")
	}
}

func TestParse_showAbbreviated(t *testing.T) {
	g := loadEvaxGrammar(t)
	// SH is unambiguous for SHOW; MEM is unambiguous for MEMORY.
	r, err := g.Parse("SH MEM")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Active != "SHOW_MEMORY" {
		t.Errorf("Active=%s, want SHOW_MEMORY", r.Active)
	}
}

func TestParse_clearBreakpointRestOfLine(t *testing.T) {
	g := loadEvaxGrammar(t)
	
	r, err := g.Parse("CLEAR BREAKPOINT 200")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Active != "CLEAR_BREAKPOINT" {
		t.Errorf("Active=%s, want CLEAR_BREAKPOINT", r.Active)
	}

	if got := r.String("BREAK_ADDR"); got != "200" {
		t.Errorf("BREAK_ADDR=%q, want 200", got)
	}
}

func TestParse_clearSymbolAll(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("CLEAR SYMBOL/ALL")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Active != "CLEAR_SYM_ALL" {
		t.Errorf("Active=%s, want CLEAR_SYM_ALL", r.Active)
	}
}

func TestParse_defineLogical(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("DEFINE/LOGICAL TT TTA0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Active != "DEFINE_LOGICAL" {
		t.Errorf("Active=%s, want DEFINE_LOGICAL", r.Active)
	}

	if got := r.String("NAME"); got != "TT" {
		t.Errorf("NAME=%q, want TT", got)
	}

	if got := r.String("VALUE"); got != "TTA0" {
		t.Errorf("VALUE=%q, want TTA0", got)
	}
}

func TestParse_vminitQualifiers(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("VMINIT/P0=2048/P1=8192/S0=2048/KSP=20")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Active != "VMINIT" {
		t.Errorf("Active=%s, want VMINIT", r.Active)
	}

	if got := r.Int("P0"); got != 2048 {
		t.Errorf("P0=%d, want 2048", got)
	}

	if got := r.Int("P1"); got != 8192 {
		t.Errorf("P1=%d, want 8192", got)
	}

	if got := r.Int("S0"); got != 2048 {
		t.Errorf("S0=%d, want 2048", got)
	}

	if got := r.Int("KSP"); got != 20 {
		t.Errorf("KSP=%d, want 20", got)
	}
	// ESP/SSP/ISP weren't given, so their /default=4 should apply.
	if got := r.Int("ESP"); got != 4 {
		t.Errorf("ESP=%d, want default 4", got)
	}
}

func TestParse_quitAliasesExit(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("QUIT")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Verb != "EXIT" {
		t.Errorf("Verb=%s, want EXIT (QUIT is an alias)", r.Verb)
	}
}

func TestParse_aboutHasEntryPoint(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("ABOUT")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.EntryPoint != "EXE$ABOUT" {
		t.Errorf("EntryPoint=%q, want EXE$ABOUT", r.EntryPoint)
	}
}

func TestParse_ambiguousVerb(t *testing.T) {
	g := loadEvaxGrammar(t)
	// C and CA... let's find a genuinely ambiguous abbreviation: "CA" could
	// only mean CALL among {define,about,forth,exit,quit,test,call,clear,
	// show,vminit} - not ambiguous. Use a single letter that's ambiguous.
	if _, err := g.Parse("C"); err == nil {
		t.Error("expected ambiguous verb error for \"C\"")
	}
}

func TestParse_missingRequiredParameter(t *testing.T) {
	g := loadEvaxGrammar(t)

	if _, err := g.Parse("DEFINE/LOGICAL"); err == nil {
		t.Error("expected error for missing required NAME/VALUE parameters")
	}
}

func TestParse_disallowCombination(t *testing.T) {
	g := loadEvaxGrammar(t)

	if _, err := g.Parse("SHOW PAGE/READ/WRITE 200"); err == nil {
		t.Error("expected DISALLOW error for /READ/WRITE combination")
	}
}

func TestParse_negatedQualifier(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse("SHOW BREAK/NOFAULTS")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.Present("FAULTS") || !r.Negated("FAULTS") {
		t.Errorf("expected FAULTS present+negated, got present=%v negated=%v", r.Present("FAULTS"), r.Negated("FAULTS"))
	}
}

func TestParse_unknownVerb(t *testing.T) {
	g := loadEvaxGrammar(t)
	if _, err := g.Parse("BOGUS"); err == nil {
		t.Error("expected unrecognized verb error")
	}
}

func TestParse_testRestOfLine(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse(`TEST foo/bar baz`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := r.String("WHAT"); got != "FOO/BAR BAZ" {
		t.Errorf("WHAT=%q, want \"FOO/BAR BAZ\"", got)
	}
}

func TestParse_quotedStringPreservesCase(t *testing.T) {
	g := loadEvaxGrammar(t)

	r, err := g.Parse(`VMINIT/DEBUG`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.Present("DEBUG") {
		t.Error("expected DEBUG present")
	}
}
