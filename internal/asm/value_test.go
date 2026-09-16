package asm

import "testing"

func evalNoForward(t *testing.T, a *Assembler, src string) uint32 {
	t.Helper()
	c := newCursor(src)
	v, err := a.exprNoForward(c)
	if err != nil {
		t.Fatalf("exprNoForward(%q): %v", src, err)
	}
	return v
}

func TestExpressionArithmetic(t *testing.T) {
	cases := []struct {
		src  string
		want uint32
	}{
		{"5", 5},
		{"^D5", 5},
		{"^X10", 0x10},
		{"0X10", 0x10},
		{"10", 0x10}, // default radix is hex
		{"^D10", 10},
		{"2+3", 5},
		{"10-3", 0xD}, // default radix is hex: 0x10-0x3
		{"2*3", 6},
		{"^D10/^D3", 3},
		{"(^D2+^D3)*^D4", 20},
		{"-^D5", 0xFFFFFFFB},
		{"-^D5+^D10", 5}, // unary minus binds tighter than the following '+'
		{"^D5=^D5", 1},
		{"^D5=^D6", 0},
		{"^D5<>^D6", 1},
		{"^D5<^D6", 1},
		{"^D6<^D5", 0},
		{"^D5<=^D5", 1},
		{"^D6>=^D5", 1},
		{"^D6>^D5", 1},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			a := New()
			if got := evalNoForward(t, a, tc.src); got != tc.want {
				t.Errorf("eval(%q) = %#x, want %#x", tc.src, got, tc.want)
			}
		})
	}
}

func TestExpressionHere(t *testing.T) {
	a := New()
	a.deposit = 0x1234
	if got := evalNoForward(t, a, "."); got != 0x1234 {
		t.Errorf(". = %#x, want 0x1234", got)
	}
}

func TestExpressionCharLiteral(t *testing.T) {
	cases := []struct {
		src  string
		want uint32
	}{
		{"'A'", 0x41},
		{"'AB'", 0x4241}, // little-endian: first char is the low byte
		{`'\n'`, 0x0A},
	}
	for _, tc := range cases {
		a := New()
		if got := evalNoForward(t, a, tc.src); got != tc.want {
			t.Errorf("eval(%q) = %#x, want %#x", tc.src, got, tc.want)
		}
	}
}

func TestExpressionMask(t *testing.T) {
	a := New()
	got := evalNoForward(t, a, "^M<R0,R2,R11,IV>")
	want := uint32(1<<0 | 1<<2 | 1<<11 | 1<<15)
	if got != want {
		t.Errorf("mask = %#x, want %#x", got, want)
	}
}

func TestExpressionDivisionByZero(t *testing.T) {
	a := New()
	c := newCursor("^D1/^D0")
	if _, err := a.exprNoForward(c); err == nil {
		t.Fatal("expected division-by-zero error")
	}
}

func TestExpressionDefinedFunction(t *testing.T) {
	a := New()
	if got := evalNoForward(t, a, `DEFINED("NOSUCHSYM")`); got != 0 {
		t.Errorf("DEFINED(undefined) = %d, want 0", got)
	}
	if err := a.setSymbol("MYSYM", 42, SymNone, false); err != nil {
		t.Fatal(err)
	}
	if got := evalNoForward(t, a, `DEFINED("MYSYM")`); got != 1 {
		t.Errorf("DEFINED(defined) = %d, want 1", got)
	}
}

func TestExpressionVerboseFunction(t *testing.T) {
	a := New()
	if got := evalNoForward(t, a, "VERBOSE()"); got != 1 {
		t.Errorf("VERBOSE() = %d, want 1 (default on)", got)
	}

	a.SetVerbose(false)
	if got := evalNoForward(t, a, "VERBOSE"); got != 0 {
		t.Errorf("VERBOSE = %d, want 0", got)
	}
}

func TestRegisterParsing(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"R0", 0}, {"R15", 15}, {"AP", 12}, {"FP", 13}, {"SP", 14}, {"PC", 15},
	}

	for _, tc := range cases {
		c := newCursor(tc.src)

		reg, err := parseRegister(c, 0)
		if err != nil {
			t.Fatalf("parseRegister(%q): %v", tc.src, err)
		}

		if int(reg) != tc.want {
			t.Errorf("parseRegister(%q) = %d, want %d", tc.src, reg, tc.want)
		}
	}
}

func TestRegisterParsingInvalid(t *testing.T) {
	for _, src := range []string{"R16", "AX", "ZP"} {
		c := newCursor(src)
		
		if _, err := parseRegister(c, 0); err == nil {
			t.Errorf("parseRegister(%q): expected error", src)
		}
	}
}
