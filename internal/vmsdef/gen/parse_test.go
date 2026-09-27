package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSDL(t *testing.T) {
	src := `
{ comment line
module $XDEF;
aggregate XDEF structure prefix X$;
    A bitfield mask;			/* bit 0
    B bitfield;				{ bit 1, no mask
    FILL_0 bitfield length 2 fill;
    C bitfield length 3 mask;		/* bits 4-6
end XDEF;
constant "MAX" equals 10 prefix X$ tag C;
constant (ONE, "TWO", THREE) equals 1 increment 1 prefix X$ tag "";
constant "CHAIN" equals -1 prefix X$ tag "";
end_module $XDEF;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{
		"X$V_A":    0,
		"X$M_A":    0x1,
		"X$V_B":    1,
		"X$V_C":    4,
		"X$S_C":    3,
		"X$M_C":    0x70,
		"X$C_MAX":  10,
		"X$_ONE":   1,
		"X$_TWO":   2,
		"X$_THREE": 3,
		"X$_CHAIN": 0xFFFFFFFF,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL =\n%v\nwant\n%v", got, want)
	}
}

// TestParseSDL_union covers $DEVDEF's shape: a union of structures, each
// numbering its bitfields from bit 0 under the union's prefix.
func TestParseSDL_union(t *testing.T) {
	src := `
module $YDEF;
aggregate YDEF union prefix Y$;
    YDEF_BITS0 structure fill;
        A bitfield mask;
        B bitfield mask;
    end YDEF_BITS0;
    YDEF_BITS1 structure fill;
        "2P" bitfield mask;
        C bitfield length 2 mask;
    end YDEF_BITS1;
end YDEF;
end_module $YDEF;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{
		"Y$V_A": 0, "Y$M_A": 0x1,
		"Y$V_B": 1, "Y$M_B": 0x2,
		"Y$V_2P": 0, "Y$M_2P": 0x1,
		"Y$V_C": 1, "Y$S_C": 2, "Y$M_C": 0x6,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL =\n%v\nwant\n%v", got, want)
	}
}

func TestParseSDL_rejectsUnsupported(t *testing.T) {
	cases := map[string]string{
		"unknown statement":   "item FOO longword;",
		"non-bitfield member": "aggregate X structure prefix X$; FOO longword; end X;",
		"missing tag":         "constant FOO equals 1 prefix X$;",
		"unknown keyword":     "constant FOO equals 1 prefix X$ tag C counter #n;",
		"unterminated":        "aggregate X structure prefix X$; A bitfield mask;",
		"over 32 bits":        "aggregate X structure prefix X$; A bitfield length 32 fill; B bitfield mask; end X;",
		"conflicting value":   "constant FOO equals 1 prefix X$ tag C; constant FOO equals 2 prefix X$ tag C;",
		"bitfield in union":   "aggregate X union prefix X$; A bitfield mask; end X;",
		"unterminated member": "aggregate X union prefix X$; M structure fill; A bitfield mask; end M;",
	}

	for name, src := range cases {
		if _, err := parseSDL(src); err == nil {
			t.Errorf("%s: parseSDL(%q) succeeded, want an error", name, src)
		}
	}
}

func TestParseBlissLiterals(t *testing.T) {
	src := strings.Join([]string{
		";+",
		"; $SSDEF",
		";-",
		"LITERAL",
		"\tSYSTEM$_FACILITY,\tI,\t0",
		"\tSS$_NORMAL,\t\tI,\t1",
		"\tSS$_CONTINUE,\t\tI,\t1",
		"\tSS$_ACCVIO,\t\tI,\t12",
		"",
	}, "\n")

	got, err := parseBlissLiterals(src, "SS$_")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{"SS$_NORMAL": 1, "SS$_CONTINUE": 1, "SS$_ACCVIO": 12}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseBlissLiterals = %v, want %v", got, want)
	}
}

func TestParseBlissLiterals_rejectsUnrecognised(t *testing.T) {
	for _, src := range []string{
		"\tSS$_NORMAL = 1;",
		"\tSS$_NORMAL,\tI,\t1\n\tSS$_NORMAL,\tI,\t2",
	} {
		if _, err := parseBlissLiterals(src, "SS$_"); err == nil {
			t.Errorf("parseBlissLiterals(%q) succeeded, want an error", src)
		}
	}
}
