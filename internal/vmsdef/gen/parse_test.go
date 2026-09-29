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

// TestParseSDL_jpidefForms covers the forms $JPIDEF adds: the "$" in the
// tag, %x hex, NAME@N shifted values, and a bitfield structure nested in a
// structure aggregate.
func TestParseSDL_jpidefForms(t *testing.T) {
	src := `
module $ZDEF;
constant ALL equals %x80000000 prefix Z tag $K;
constant PCBTYPE equals 3 prefix Z tag $C;
constant(
      FIRST		/* comment between entries
    , SECOND
    ) equals Z$C_PCBTYPE@8 increment 1 prefix Z tag $;
aggregate ZCTLDEF structure prefix Z$;
    ZFLGS structure longword unsigned fill;
	A bitfield mask;
	FILL1 bitfield LENGTH 2 mask;
	B bitfield mask;
    end ZFLGS;
end ZCTLDEF;
end_module $ZDEF;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{
		"Z$K_ALL":     0x80000000,
		"Z$C_PCBTYPE": 3,
		"Z$_FIRST":    0x300,
		"Z$_SECOND":   0x301,
		"Z$V_A":       0, "Z$M_A": 0x1,
		"Z$V_FILL1": 1, "Z$S_FILL1": 2, "Z$M_FILL1": 0x6,
		"Z$V_B": 3, "Z$M_B": 0x8,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL =\n%v\nwant\n%v", got, want)
	}
}

// TestParseSDL_localSymbols covers $IODEF's "#NAME = V" local symbols,
// used as bitfield lengths alone and in "N-#NAME" expressions, and its
// constants inside an aggregate, which take SDL's default prefix and tag.
func TestParseSDL_localSymbols(t *testing.T) {
	src := `
module $QDEF;
#fcode_size = 6;
aggregate QDEF union prefix Q$;
    FCODE_STRUCTURE structure fill;
        FCODE bitfield mask length #fcode_size;
        FMODIFIERS bitfield mask length 16-#fcode_size;
    end FCODE_STRUCTURE;
    READ_MODIFIERS structure fill;
        fcode_fill bitfield length #fcode_size fill;
        NOECHO bitfield mask;
    end READ_MODIFIERS;
    constant LOOPTEST equals 57344;
end QDEF;
end_module $QDEF;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{
		"Q$V_FCODE": 0, "Q$S_FCODE": 6, "Q$M_FCODE": 0x3F,
		"Q$V_FMODIFIERS": 6, "Q$S_FMODIFIERS": 10, "Q$M_FMODIFIERS": 0xFFC0,
		"Q$V_NOECHO": 6, "Q$M_NOECHO": 0x40,
		"Q$K_LOOPTEST": 57344,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL =\n%v\nwant\n%v", got, want)
	}
}

// TestParseSDL_binaryParenthesized checks $PRTDEF's form, binary
// literals in parentheses, and $BRKDEF's reference to an earlier
// constant by its full name.
func TestParseSDL_binaryParenthesized(t *testing.T) {
	src := `
module $PDEF;
constant NA	  equals (%B0000) prefix P tag $C;	/* No Access
constant UR	  equals (%B1111) prefix P tag $C;
constant KW	  equals %b0010 prefix P tag $C;
constant MAXP	  equals P$C_UR prefix P tag $C;
end_module $PDEF;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{"P$C_NA": 0, "P$C_UR": 15, "P$C_KW": 2, "P$C_MAXP": 15}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL = %v, want %v", got, want)
	}
}

// TestParseSDL_recordLayout covers the record-layout forms objfmt.sdl
// uses (docs/PHASE-27.md): typed fields and their offsets, a flags word
// overlaid by its bits, "equals .", character lengths and a fill
// dimension, a tag override, ifsymbol blocks, and an origin field.
func TestParseSDL_recordLayout(t *testing.T) {
	src := `
module $RDEF;
aggregate RDEF structure prefix R$ origin FILL_1;
    TYP_OVERLAY union fill;
        TYP byte unsigned;
        TYP_FIELDS structure fill;
ifsymbol not_h_files;
            START character length 0 tag T;
end_ifsymbol;
            FILL_1 byte fill prefix RDEF tag $$;
        end TYP_FIELDS;
    end TYP_OVERLAY;
    "ALIGN" byte unsigned;
    FLAGS_OVERLAY union fill;
        FLAGS word unsigned;
        FLAGS_BITS structure fill;
            PIC bitfield mask;
            WID bitfield length 3;
            REL bitfield mask;
        end FLAGS_BITS;
    end FLAGS_OVERLAY;
    ALLOC longword unsigned;
    PAD byte dimension 2 fill;
    constant NAME equals . prefix R$ tag K;
    NAME character length 31;
    WORDS union word unsigned;
        A bitfield mask;
    end WORDS;
    Q quadword tag X;
end RDEF;
end_module $RDEF;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	// With "origin FILL_1" (at offset 0) the offsets are unchanged.
	want := map[string]uint32{
		"R$B_TYP": 0, "R$T_START": 0, "R$B_ALIGN": 1, "R$W_FLAGS": 2,
		"R$V_PIC": 0, "R$M_PIC": 1, "R$V_WID": 1, "R$S_WID": 3,
		"R$V_REL": 4, "R$M_REL": 0x10,
		"R$L_ALLOC": 4, "R$K_NAME": 10, "R$T_NAME": 10, "R$S_NAME": 31,
		"R$W_WORDS": 41, "R$V_A": 0, "R$M_A": 1, "R$X_Q": 43,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL =\n%v\nwant\n%v", got, want)
	}
}

// TestParseSDL_origin checks that "origin FIELD" makes every byte offset
// (fields and "equals ." constants alike) relative to FIELD, even one
// defined after the field, while bit positions are unaffected.
func TestParseSDL_origin(t *testing.T) {
	src := `
aggregate O structure prefix O$ origin BASE;
    HDR word unsigned;
    BASE byte unsigned;
    constant AT equals . prefix O$ tag K;
    VAL longword unsigned;
    BITS structure fill;
        B bitfield mask;
    end BITS;
end O;
`

	got, err := parseSDL(src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint32{
		"O$W_HDR": 0xFFFFFFFE, "O$B_BASE": 0, "O$K_AT": 1, "O$L_VAL": 1,
		"O$V_B": 0, "O$M_B": 1,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSDL = %v, want %v", got, want)
	}
}

func TestParseSDL_rejectsUnsupported(t *testing.T) {
	cases := map[string]string{
		"unknown statement":    "item FOO longword;",
		"unknown field type":   "aggregate X structure prefix X$; FOO address; end X;",
		"length on a word":     "aggregate X structure prefix X$; FOO word length 2; end X;",
		"character nested":     "aggregate X structure prefix X$; M structure character; end M; end X;",
		"missing origin field": "aggregate X structure prefix X$ origin NOPE; A byte; end X;",
		"dot outside":          "constant FOO equals . prefix X$ tag K;",
		"missing tag":          "constant FOO equals 1 prefix X$;",
		"unknown keyword":      "constant FOO equals 1 prefix X$ tag C counter #n;",
		"unterminated":         "aggregate X structure prefix X$; A bitfield mask;",
		"over 32 bits":         "aggregate X structure prefix X$; A bitfield length 32 fill; B bitfield mask; end X;",
		"conflicting value":    "constant FOO equals 1 prefix X$ tag C; constant FOO equals 2 prefix X$ tag C;",
		"unterminated member":  "aggregate X union prefix X$; M structure fill; A bitfield mask; end M;",
		"undefined shift base": "constant FOO equals X$C_NONE@8 prefix X$ tag C;",
		"undefined local":      "aggregate X structure prefix X$; A bitfield length #n; end X;",
		"bad local statement":  "#n == 6;",
		"length expression":    "#n = 6; aggregate X structure prefix X$; A bitfield length 2*#n; end X;",
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

// TestParseBlissLiterals_vestListing: the VEST listings' "I4" type, as in
// reference/vms/syidef.txt.
func TestParseBlissLiterals_vestListing(t *testing.T) {
	src := ";+\n;\t$SYIDEF\n;-\n\tSYI$C_EXETYPE,\t\t\tI4, 1\n\tSYI$_VERSION,\t\t\tI4, 4096\n" +
		"\tSYI$_spare_bit_1,\t\tI4, 284\n" // a mixed-case name, as in dvidef.txt

	got, err := parseBlissLiterals(src, "SYI$")
	if err != nil {
		t.Fatal(err)
	}

	if want := map[string]uint32{"SYI$C_EXETYPE": 1, "SYI$_VERSION": 4096, "SYI$_spare_bit_1": 284}; !reflect.DeepEqual(got, want) {
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
