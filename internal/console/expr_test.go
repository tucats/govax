package console

import "testing"

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
