package console

import (
	"bytes"
	"testing"
)

func evalTest(t *testing.T, radix int, syms map[string]uint32, expr string) uint32 {
	t.Helper()
	st := NewSymbolTable()

	for k, v := range syms {
		st.Set(k, v, SymbolUser)
	}

	e := &Evaluator{Symbols: st, Radix: radix, Here: 0x1000}

	v, rest, err := e.Eval(expr)
	if err != nil {
		t.Fatalf("Eval(%q): %v", expr, err)
	}

	if rest != "" {
		t.Fatalf("Eval(%q): unparsed remainder %q", expr, rest)
	}

	return v
}

func TestEvaluator_defaultRadix(t *testing.T) {
	if got := evalTest(t, 16, nil, "200"); got != 0x200 {
		t.Errorf("got %#x, want 0x200", got)
	}

	if got := evalTest(t, 10, nil, "200"); got != 200 {
		t.Errorf("got %d, want 200", got)
	}
}

func TestEvaluator_radixPrefix(t *testing.T) {
	cases := map[string]uint32{
		"^D4096": 4096,
		"^X1F":   0x1F,
		"0X1F":   0x1F,
		"^O17":   15,
		"^B101":  5,
	}
	
	for expr, want := range cases {
		if got := evalTest(t, 16, nil, expr); got != want {
			t.Errorf("Eval(%q) = %#x, want %#x", expr, got, want)
		}
	}
}

func TestEvaluator_symbol(t *testing.T) {
	got := evalTest(t, 16, map[string]uint32{"FOO": 0x1234}, "FOO")
	if got != 0x1234 {
		t.Errorf("got %#x, want 0x1234", got)
	}
}

func TestEvaluator_undefinedSymbol(t *testing.T) {
	e := &Evaluator{Symbols: NewSymbolTable(), Radix: 16}
	if _, _, err := e.Eval("NOSUCH"); err == nil {
		t.Error("expected an error for an undefined symbol")
	}
}

func TestEvaluator_here(t *testing.T) {
	if got := evalTest(t, 16, nil, "."); got != 0x1000 {
		t.Errorf("got %#x, want 0x1000", got)
	}
}

func TestEvaluator_arithmeticAndPrecedence(t *testing.T) {
	cases := map[string]uint32{
		"1+2*3":   7,
		"(1+2)*3": 9,
		"10-2-3":  5,
		"200*2/4": 100,
	}
	for expr, want := range cases {
		if got := evalTest(t, 10, nil, expr); got != want {
			t.Errorf("Eval(%q) = %d, want %d", expr, got, want)
		}
	}
	if got := evalTest(t, 16, map[string]uint32{"FOO": 0x1234}, "FOO+4"); got != 0x1238 {
		t.Errorf("Eval(FOO+4) = %#x, want 0x1238", got)
	}
}

func TestEvaluator_comparisons(t *testing.T) {
	cases := map[string]uint32{
		"1=1":  1,
		"1=2":  0,
		"1<>2": 1,
		"1<2":  1,
		"2<=2": 1,
		"3>2":  1,
		"2>=3": 0,
	}
	for expr, want := range cases {
		if got := evalTest(t, 10, nil, expr); got != want {
			t.Errorf("Eval(%q) = %d, want %d", expr, got, want)
		}
	}
}

func TestEvaluator_divisionByZero(t *testing.T) {
	e := &Evaluator{Symbols: NewSymbolTable(), Radix: 10}
	if _, _, err := e.Eval("1/0"); err == nil {
		t.Error("expected division-by-zero error")
	}
}

func TestEvaluator_definedFunction(t *testing.T) {
	if got := evalTest(t, 16, map[string]uint32{"FOO": 1}, `DEFINED("FOO")`); got != 1 {
		t.Errorf(`Eval(DEFINED("FOO")) = %d, want 1`, got)
	}
	if got := evalTest(t, 16, nil, `DEFINED("NOSUCH")`); got != 0 {
		t.Errorf(`Eval(DEFINED("NOSUCH")) = %d, want 0`, got)
	}
	// Case-insensitive function name, matching asm_function()'s own
	// upcase-before-compare handling.
	if got := evalTest(t, 16, map[string]uint32{"FOO": 1}, `defined("FOO")`); got != 1 {
		t.Errorf(`Eval(defined("FOO")) = %d, want 1`, got)
	}
	// DEFINED() composes with the rest of the expression grammar, since
	// cmdIf (dispatch.go) evaluates it as one ordinary expression.
	if got := evalTest(t, 16, nil, `DEFINED("NOSUCH")=0`); got != 1 {
		t.Errorf(`Eval(DEFINED("NOSUCH")=0) = %d, want 1`, got)
	}
}

func TestEvaluator_trailingRemainder(t *testing.T) {
	// A second address (e.g. EXAMINE's optional end-address) is left
	// unconsumed for the caller to parse separately.
	e := &Evaluator{Symbols: NewSymbolTable(), Radix: 16}
	v, rest, err := e.Eval("200 300")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if v != 0x200 || rest != " 300" {
		t.Errorf("v=%#x rest=%q, want v=0x200 rest=\" 300\"", v, rest)
	}
}

// vminitConsoleForStrings returns a booted Console (INIT + VMINIT with the
// DCL grammar's default 8-page string pool) whose Evaluator can exercise
// parseQuotedString -- the string-literal cases below all need real
// backing memory and the CONSOLE$STRINGPOOL* symbols VMInit defines, unlike
// every other Evaluator test above, which only needs a bare SymbolTable.
func vminitConsoleForStrings(t *testing.T) *Console {
	t.Helper()

	c := New(&bytes.Buffer{})
	if err := c.Init(4096 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := c.VMInit(2000, 100, 0, 4, 4, 4, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	return c
}

// TestEvaluator_quotedString matches asm_expr3's own '"' case
// (reference/eVAX/eVAX/Source/Assembler/asm_expr.c): a quoted string
// literal builds a VAX string descriptor in the console string pool and
// evaluates to that descriptor's address.
func TestEvaluator_quotedString(t *testing.T) {
	c := vminitConsoleForStrings(t)
	ev := c.Evaluator()

	desc, rest, err := ev.Eval(`"Hello"`)
	if err != nil {
		t.Fatalf(`Eval("Hello"): %v`, err)
	}
	if rest != "" {
		t.Errorf("rest = %q, want empty", rest)
	}

	length, err := c.Mem.LoadLongword(c.CPU, desc)
	if err != nil {
		t.Fatalf("load descriptor length: %v", err)
	}
	if length != 5 {
		t.Errorf("descriptor length = %d, want 5", length)
	}

	dataAddr, err := c.Mem.LoadLongword(c.CPU, desc+4)
	if err != nil {
		t.Fatalf("load descriptor data address: %v", err)
	}

	got := make([]byte, 5)
	if err := c.Mem.Load(c.CPU, dataAddr, got); err != nil {
		t.Fatalf("load string data: %v", err)
	}
	if string(got) != "Hello" {
		t.Errorf("string data = %q, want %q", got, "Hello")
	}
}

// TestEvaluator_quotedStringEscapes matches asm_expr3's \n/\r/\t escape
// handling for a quoted string literal.
func TestEvaluator_quotedStringEscapes(t *testing.T) {
	c := vminitConsoleForStrings(t)
	ev := c.Evaluator()

	desc, _, err := ev.Eval(`"a\nb\rc\td"`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}

	length, err := c.Mem.LoadLongword(c.CPU, desc)
	if err != nil {
		t.Fatalf("load descriptor length: %v", err)
	}
	if length != 7 {
		t.Fatalf("descriptor length = %d, want 7", length)
	}

	dataAddr, err := c.Mem.LoadLongword(c.CPU, desc+4)
	if err != nil {
		t.Fatalf("load descriptor data address: %v", err)
	}

	got := make([]byte, 7)
	if err := c.Mem.Load(c.CPU, dataAddr, got); err != nil {
		t.Fatalf("load string data: %v", err)
	}
	if string(got) != "a\nb\rc\td" {
		t.Errorf("string data = %q, want %q", got, "a\nb\rc\td")
	}
}

// TestEvaluator_quotedStringChain confirms successive string literals link
// onto CONSOLE$STRINGPOOL_BASE's chain in order, matching ShowString's own
// walk (show.go) of the same linked list asm_expr3 builds.
func TestEvaluator_quotedStringChain(t *testing.T) {
	c := vminitConsoleForStrings(t)
	ev := c.Evaluator()

	desc1, _, err := ev.Eval(`"first"`)
	if err != nil {
		t.Fatalf("Eval first: %v", err)
	}
	desc2, _, err := ev.Eval(`"second"`)
	if err != nil {
		t.Fatalf("Eval second: %v", err)
	}

	base, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		t.Fatal("expected CONSOLE$STRINGPOOL_BASE to be defined")
	}

	head, err := c.Mem.LoadLongword(c.CPU, base)
	if err != nil {
		t.Fatalf("load chain head: %v", err)
	}
	if head != desc1 {
		t.Errorf("chain head = %#x, want first descriptor %#x", head, desc1)
	}

	next, err := c.Mem.LoadLongword(c.CPU, desc1+8)
	if err != nil {
		t.Fatalf("load first's next link: %v", err)
	}
	if next != desc2 {
		t.Errorf("first's next link = %#x, want second descriptor %#x", next, desc2)
	}

	terminator, err := c.Mem.LoadLongword(c.CPU, desc2+8)
	if err != nil {
		t.Fatalf("load second's next link: %v", err)
	}
	if terminator != 0 {
		t.Errorf("second's next link = %#x, want 0 (chain terminator)", terminator)
	}
}

// TestEvaluator_quotedStringUnterminated matches asm_expr3's own leniency:
// a missing closing quote is not an error, the literal just runs to the end
// of the input.
func TestEvaluator_quotedStringUnterminated(t *testing.T) {
	c := vminitConsoleForStrings(t)
	ev := c.Evaluator()

	desc, rest, err := ev.Eval(`"unterminated`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if rest != "" {
		t.Errorf("rest = %q, want empty", rest)
	}

	length, err := c.Mem.LoadLongword(c.CPU, desc)
	if err != nil {
		t.Fatalf("load descriptor length: %v", err)
	}
	if length != uint32(len("unterminated")) {
		t.Errorf("descriptor length = %d, want %d", length, len("unterminated"))
	}
}

// TestEvaluator_quotedStringNoPool matches asm_expr3's own get_symbol
// failure path when CONSOLE$STRINGPOOL* isn't defined yet (no VMINIT).
func TestEvaluator_quotedStringNoPool(t *testing.T) {
	e := &Evaluator{Symbols: NewSymbolTable(), Radix: 16}
	if _, _, err := e.Eval(`"Hello"`); err == nil {
		t.Error("expected an error with no string pool defined")
	}
}

// TestEvaluator_quotedStringOverflow matches asm_expr3's own VAX_ASMSPOVF
// check: a string that would run the pool's bump pointer past
// CONSOLE$STRINGPOOL_BASE + CONSOLE$STRINGPOOL_SIZE + 16 fails instead of
// overrunning the pool's backing storage.
func TestEvaluator_quotedStringOverflow(t *testing.T) {
	c := vminitConsoleForStrings(t)
	ev := c.Evaluator()

	size, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_SIZE")
	if !ok {
		t.Fatal("expected CONSOLE$STRINGPOOL_SIZE to be defined")
	}

	huge := `"` + string(make([]byte, size+64)) + `"`
	if _, _, err := ev.Eval(huge); err == nil {
		t.Error("expected a string pool overflow error")
	}
}
