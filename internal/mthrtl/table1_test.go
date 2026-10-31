package mthrtl_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The routines of the manual's Table 1-1, and MTH$UMAX's and MTH$CVT_x_x's pages,
// from a real program: every entry, whether it takes floating values or integers,
// for answers, each condition, and a handler that continues from it.

// table1Type names a type of argument or result: a floating format's letter, or W,
// L, or U for a word, a longword, or an unsigned longword.
type table1Type string

func (t table1Type) format() (vaxfloat.Format, bool) {
	switch t {
	case "F":
		return vaxfloat.F, true
	case "D":
		return vaxfloat.D, true
	case "G":
		return vaxfloat.G, true
	case "H":
		return vaxfloat.H, true
	}

	return 0, false
}

// directive is the MACRO directive that holds a literal of this type.
func (t table1Type) directive() string {
	switch t {
	case "W":
		return ".WORD"
	case "L", "U":
		return ".LONG"
	}

	return "." + string(t) + "_FLOATING"
}

// table1Case is one call. An argument is a literal of type in, or one of:
//
//	""      a reserved operand
//	"HUGE"  a value near the format's largest (with - before it, negative)
//	"TINY"  a value near its smallest
//	"BAD"   an address no program can read, in place of the argument
//	"^X.."  a longword's bits, for a value no literal spells exactly
//
// want is the result as a number, as the result's type holds it, and "*" means
// don't look; empty is the reserved operand.
type table1Case struct {
	entry     string
	in, out   table1Type
	args      []string
	fu        bool
	want      string
	condition uint32
	count     int // the signal array's first longword, when there's a condition
}

var (
	ssROPRAND = vmsdef.Symbols["SS$_ROPRAND"]
	ssACCVIO  = vmsdef.Symbols["SS$_ACCVIO"]
	ssFLTOVF  = vmsdef.Symbols["SS$_FLTOVF_F"]
	ssFLTDIV  = vmsdef.Symbols["SS$_FLTDIV_F"]
	ssFLTUND  = vmsdef.Symbols["SS$_FLTUND_F"]
	ssINTOVF  = vmsdef.Symbols["SS$_INTOVF"]
	ssINTDIV  = vmsdef.Symbols["SS$_INTDIV"]
	mthFLOOVE = vmsdef.LibrarySymbols["MTH$_FLOOVEMAT"]
	mthFLOUND = vmsdef.LibrarySymbols["MTH$_FLOUNDMAT"]
)

// table1Formats are the four floating formats, with their spellings of HUGE and TINY.
var table1Formats = []struct {
	t          table1Type
	huge, tiny string
}{
	{"F", "1.5E38", "3.0E-39"},
	{"D", "1.5E38", "3.0E-39"},
	{"G", "8.0E307", "6.0E-309"},
	{"H", "5.0E4931", "9.0E-4933"},
}

// spell puts the format's own words for HUGE and TINY in a literal.
func spell(arg, huge, tiny string) string {
	return strings.NewReplacer("HUGE", huge, "TINY", tiny).Replace(arg)
}

// table1Cases makes the program's calls.
func table1Cases() []table1Case {
	var cases []table1Case

	// floating is one row for a family of one entry per format, names in format
	// order, and the JSB entries beside them when jsb names them.
	floating := func(names, jsb []string, args []string, want string, condition uint32, count int, fu bool) {
		for i, f := range table1Formats {
			spelled := make([]string, len(args))
			for j, a := range args {
				spelled[j] = spell(a, f.huge, f.tiny)
			}

			c := table1Case{
				entry: names[i], in: f.t, out: f.t, args: spelled, want: want,
				condition: condition, count: count, fu: fu,
			}
			cases = append(cases, c)

			if jsb != nil {
				c.entry = jsb[i]
				cases = append(cases, c)
			}
		}
	}

	names := func(pattern string) []string {
		return strings.Fields(pattern)
	}

	abs := names("MTH$ABS MTH$DABS MTH$GABS MTH$HABS")
	floating(abs, nil, []string{"-1.5"}, "1.5", 0, 0, false)
	floating(abs, nil, []string{"1.5"}, "1.5", 0, 0, false)
	floating(abs, nil, []string{"0.0"}, "0", 0, 0, false)
	floating(abs, nil, []string{""}, "", ssROPRAND, 3, false)

	dim := names("MTH$DIM MTH$DDIM MTH$GDIM MTH$HDIM")
	floating(dim, nil, []string{"3.5", "1.0"}, "2.5", 0, 0, false)
	floating(dim, nil, []string{"1.0", "3.5"}, "0", 0, 0, false)
	floating(dim, nil, []string{"HUGE", "-HUGE"}, "", ssFLTOVF, 3, false)
	floating(dim, nil, []string{"", "1.0"}, "", ssROPRAND, 3, false)
	floating(dim, nil, []string{"1.0", ""}, "", ssROPRAND, 3, false)

	sign := names("MTH$SIGN MTH$DSIGN MTH$GSIGN MTH$HSIGN")
	floating(sign, nil, []string{"1.5", "-2.0"}, "-1.5", 0, 0, false)
	floating(sign, nil, []string{"-1.5", "2.0"}, "1.5", 0, 0, false)
	floating(sign, nil, []string{"-1.5", "0.0"}, "1.5", 0, 0, false)
	floating(sign, nil, []string{"", "1.0"}, "", ssROPRAND, 3, false)

	floor, floorJSB := names("MTH$FLOOR MTH$DFLOOR MTH$GFLOOR MTH$HFLOOR"),
		names("MTH$FLOOR_R1 MTH$DFLOOR_R3 MTH$GFLOOR_R3 MTH$HFLOOR_R7")
	floating(floor, floorJSB, []string{"1.5"}, "1", 0, 0, false)
	floating(floor, floorJSB, []string{"-1.5"}, "-2", 0, 0, false)
	floating(floor, floorJSB, []string{"-1.0"}, "-1", 0, 0, false)
	floating(floor, floorJSB, []string{"0.0"}, "0", 0, 0, false)
	floating(floor, floorJSB, []string{""}, "", ssROPRAND, 3, false)

	aint, aintJSB := names("MTH$AINT MTH$DINT MTH$GINT MTH$HINT"),
		names("MTH$AINT_R2 MTH$DINT_R4 MTH$GINT_R4 MTH$HINT_R8")
	floating(aint, aintJSB, []string{"1.75"}, "1", 0, 0, false)
	floating(aint, aintJSB, []string{"-1.75"}, "-1", 0, 0, false)
	floating(aint, aintJSB, []string{""}, "", ssROPRAND, 3, false)

	anint := names("MTH$ANINT MTH$DNINT MTH$GNINT MTH$HNINT")
	floating(anint, nil, []string{"1.5"}, "2", 0, 0, false)
	floating(anint, nil, []string{"-1.5"}, "-2", 0, 0, false)
	floating(anint, nil, []string{"1.25"}, "1", 0, 0, false)
	floating(anint, nil, []string{""}, "", ssROPRAND, 3, false)

	mod := names("MTH$AMOD MTH$DMOD MTH$GMOD MTH$HMOD")
	floating(mod, nil, []string{"5.5", "2.0"}, "1.5", 0, 0, false)
	floating(mod, nil, []string{"-5.5", "2.0"}, "-1.5", 0, 0, false)
	floating(mod, nil, []string{"5.5", "-2.0"}, "1.5", 0, 0, false)
	floating(mod, nil, []string{"1.0", "0.0"}, "", mthINVARG, 5, false)
	floating(mod, nil, []string{"HUGE", "TINY"}, "0", 0, 0, false)
	floating(mod, nil, []string{"", "1.0"}, "", ssROPRAND, 3, false)
	floating(mod, nil, []string{"1.0", ""}, "", ssROPRAND, 3, false)

	// F_floating's smallest value times 2, less a little more than the smallest: a
	// remainder below the format's range.
	for _, fu := range []bool{false, true} {
		c := table1Case{
			entry: "MTH$AMOD", in: "F", out: "F", args: []string{"^X00000100", "^X00010080"},
			want: "0", fu: fu,
		}
		if fu {
			c.condition, c.count = ssFLTUND, 3
		}

		cases = append(cases, c)
	}

	integer := func(entry string, in, out table1Type, args []string, want string, condition uint32) {
		count := 0
		if condition != 0 {
			count = 3
		}

		cases = append(cases, table1Case{
			entry: entry, in: in, out: out, args: args, want: want, condition: condition, count: count,
		})
	}

	for _, row := range []struct {
		entry     string
		t         table1Type
		args      string
		want      string
		condition uint32
	}{
		{"MTH$IIABS", "W", "-5", "5", 0},
		{"MTH$IIABS", "W", "-32768", "-32768", ssINTOVF},
		{"MTH$JIABS", "L", "-5", "5", 0},
		{"MTH$JIABS", "L", "-2147483648", "-2147483648", ssINTOVF},
		{"MTH$IIDIM", "W", "5 3", "2", 0},
		{"MTH$IIDIM", "W", "3 5", "0", 0},
		{"MTH$IIDIM", "W", "32767 -1", "-32768", ssINTOVF},
		{"MTH$JIDIM", "L", "5 3", "2", 0},
		{"MTH$JIDIM", "L", "3 5", "0", 0},
		{"MTH$JIDIM", "L", "2147483647 -1", "-2147483648", ssINTOVF},
		{"MTH$IISIGN", "W", "5 -1", "-5", 0},
		{"MTH$IISIGN", "W", "-32768 1", "-32768", 0},
		{"MTH$JISIGN", "L", "-5 1", "5", 0},
		{"MTH$JISIGN", "L", "-2147483648 1", "-2147483648", 0},
		{"MTH$IMOD", "W", "7 3", "1", 0},
		{"MTH$IMOD", "W", "-7 3", "-1", 0},
		{"MTH$IMOD", "W", "7 -3", "1", 0},
		{"MTH$IMOD", "W", "-32768 -1", "0", 0},
		{"MTH$IMOD", "W", "7 0", "0", ssINTDIV},
		{"MTH$JMOD", "L", "7 3", "1", 0},
		{"MTH$JMOD", "L", "-7 3", "-1", 0},
		{"MTH$JMOD", "L", "-2147483648 -1", "0", ssINTOVF},
		{"MTH$JMOD", "L", "-7 0", "0", ssINTDIV},
		{"MTH$IIAND", "W", "^XF0F0 ^XFF00", "-4096", 0},
		{"MTH$JIAND", "L", "^XF0F0 ^XFF00", "61440", 0},
		{"MTH$IIOR", "W", "^X00F0 ^X0F00", "4080", 0},
		{"MTH$JIOR", "L", "^X00F0 ^X0F00", "4080", 0},
		{"MTH$IIEOR", "W", "^X00FF ^X0FF0", "3855", 0},
		{"MTH$JIEOR", "L", "^X00FF ^X0FF0", "3855", 0},
		{"MTH$INOT", "W", "0", "-1", 0},
		{"MTH$JNOT", "L", "-1", "0", 0},
		{"MTH$IISHFT", "W", "1 15", "-32768", 0},
		{"MTH$IISHFT", "W", "-1 -15", "1", 0},
		{"MTH$IISHFT", "W", "1 16", "0", 0},
		{"MTH$JISHFT", "L", "1 31", "-2147483648", 0},
		{"MTH$JISHFT", "L", "-1 -31", "1", 0},
		{"MTH$JISHFT", "L", "1 32", "0", 0},
		{"MTH$IMAX0", "W", "1 -2 3", "3", 0},
		{"MTH$IMAX0", "W", "-2", "-2", 0},
		{"MTH$JMAX0", "L", "1 -2 3 2147483647", "2147483647", 0},
		{"MTH$IMIN0", "W", "1 -2 3", "-2", 0},
		{"MTH$JMIN0", "L", "1 -2 3 -2147483648", "-2147483648", 0},
		{"MTH$UMAX", "U", "1 ^XFFFFFFFF 3", "4294967295", 0},
		{"MTH$UMIN", "U", "^XFFFFFFFF 7 ^X80000000", "7", 0},
		{"MTH$IMAX0", "W", "", "0", 0},
		{"MTH$JMIN0", "L", "", "0", 0},
		{"MTH$UMAX", "U", "", "0", 0},
	} {
		integer(row.entry, row.t, row.t, strings.Fields(row.args), row.want, row.condition)
	}

	// A floating argument, and an integer result, or the reverse.
	for _, row := range []struct {
		entry     string
		in, out   table1Type
		args      string
		want      string
		condition uint32
	}{
		{"MTH$SGN", "F", "L", "-2.5", "-1", 0},
		{"MTH$SGN", "F", "L", "2.5", "1", 0},
		{"MTH$SGN", "F", "L", "0.0", "0", 0},
		{"MTH$SGN", "D", "L", "-2.5", "-1", 0},
		{"MTH$SGN", "D", "L", "2.5", "1", 0},

		{"MTH$IIFIX", "F", "W", "-2.75", "-2", 0},
		{"MTH$IINT", "F", "W", "2.75", "2", 0},
		{"MTH$IINT", "F", "W", "65537.0", "1", ssINTOVF},
		{"MTH$JIFIX", "F", "L", "-2.75", "-2", 0},
		{"MTH$JINT", "F", "L", "2.75", "2", 0},
		{"MTH$JINT", "F", "L", "2147483648.0", "-2147483648", ssINTOVF},
		{"MTH$IIDINT", "D", "W", "-2.75", "-2", 0},
		{"MTH$IIDINT", "D", "W", "32768.0", "-32768", ssINTOVF},
		{"MTH$JIDINT", "D", "L", "2147483647.5", "2147483647", 0},
		{"MTH$JIDINT", "D", "L", "2147483648.0", "-2147483648", ssINTOVF},
		{"MTH$IIGINT", "G", "W", "-2.75", "-2", 0},
		{"MTH$IIGINT", "G", "W", "32768.0", "-32768", ssINTOVF},
		{"MTH$JIGINT", "G", "L", "-2.75", "-2", 0},
		{"MTH$JIGINT", "G", "L", "2147483648.0", "-2147483648", ssINTOVF},
		{"MTH$IIHINT", "H", "W", "-2.75", "-2", 0},
		{"MTH$IIHINT", "H", "W", "32768.0", "-32768", ssINTOVF},
		{"MTH$JIHINT", "H", "L", "-2.75", "-2", 0},
		{"MTH$JIHINT", "H", "L", "2147483648.0", "-2147483648", ssINTOVF},

		{"MTH$ININT", "F", "W", "2.5", "3", 0},
		{"MTH$ININT", "F", "W", "-2.5", "-3", 0},
		{"MTH$ININT", "F", "W", "32767.5", "-32768", ssINTOVF},
		{"MTH$JNINT", "F", "L", "2.5", "3", 0},
		{"MTH$JNINT", "F", "L", "2147483648.0", "-2147483648", ssINTOVF},
		{"MTH$IIDNNT", "D", "W", "2.5", "3", 0},
		{"MTH$IIDNNT", "D", "W", "32767.5", "-32768", ssINTOVF},
		{"MTH$JIDNNT", "D", "L", "-2.5", "-3", 0},
		{"MTH$JIDNNT", "D", "L", "2147483647.5", "-2147483648", ssINTOVF},
		{"MTH$IIGNNT", "G", "W", "2.5", "3", 0},
		{"MTH$IIGNNT", "G", "W", "32767.5", "-32768", ssINTOVF},
		{"MTH$JIGNNT", "G", "L", "-2.5", "-3", 0},
		{"MTH$JIGNNT", "G", "L", "2147483647.5", "-2147483648", ssINTOVF},
		{"MTH$IIHNNT", "H", "W", "2.5", "3", 0},
		{"MTH$IIHNNT", "H", "W", "32767.5", "-32768", ssINTOVF},
		{"MTH$JIHNNT", "H", "L", "-2.5", "-3", 0},
		{"MTH$JIHNNT", "H", "L", "2147483647.5", "-2147483648", ssINTOVF},

		{"MTH$IMAX1", "F", "W", "1.5 -2.0 2.75", "2", 0},
		{"MTH$IMAX1", "F", "W", "1.5 40000.0", "-25536", ssINTOVF},
		{"MTH$JMAX1", "F", "L", "1.5 -2.0 2.75", "2", 0},
		{"MTH$JMAX1", "F", "L", "1.5 3000000000.0", "-1294967296", ssINTOVF},
		{"MTH$IMIN1", "F", "W", "1.5 -2.75 2.0", "-2", 0},
		{"MTH$IMIN1", "F", "W", "1.5 -40000.0", "25536", ssINTOVF},
		{"MTH$JMIN1", "F", "L", "1.5 -2.75 2.0", "-2", 0},
		{"MTH$JMIN1", "F", "L", "1.5 -3000000000.0", "1294967296", ssINTOVF},

		{"MTH$AIMAX0", "W", "F", "1 -2 3", "3", 0},
		{"MTH$AJMAX0", "L", "F", "1 -2 100000", "100000", 0},
		{"MTH$AJMAX0", "L", "F", "16777216 16777217", "16777218", 0}, // picked in full, then rounded
		{"MTH$AIMIN0", "W", "F", "1 -2 3", "-2", 0},
		{"MTH$AJMIN0", "L", "F", "1 -100000 3", "-100000", 0},

		{"MTH$FLOATI", "W", "F", "-7", "-7", 0},
		{"MTH$DFLOTI", "W", "D", "-7", "-7", 0},
		{"MTH$GFLOTI", "W", "G", "-7", "-7", 0},
		{"MTH$FLOATJ", "L", "F", "16777217", "16777218", 0}, // a tie: away from zero
		{"MTH$DFLOTJ", "L", "D", "16777217", "16777217", 0},
		{"MTH$GFLOTJ", "L", "G", "-2147483648", "-2147483648", 0},

		{"MTH$DBLE", "F", "D", "1.5", "1.5", 0},
		{"MTH$GDBLE", "F", "G", "1.5", "1.5", 0},
		{"MTH$SNGL", "D", "F", "1.5", "1.5", 0},
		{"MTH$SNGL", "D", "F", "16777217.0", "16777218", 0},
		{"MTH$SNGLG", "G", "F", "1.5", "1.5", 0},
		{"MTH$SNGLG", "G", "F", "1.0E300", "", ssFLTOVF},
		{"MTH$DPROD", "F", "D", "1.5 2.5", "3.75", 0},
		{"MTH$DPROD", "F", "D", "1.0E30 1.0E30", "", ssFLTOVF},
		{"MTH$DPROD", "F", "D", "1.0E-30 1.0E-30", "0", 0}, // no footnote, and no FU
		{"MTH$GPROD", "F", "G", "1.5 2.5", "3.75", 0},
		{"MTH$GPROD", "F", "G", "1.0E30 1.0E30", "", 0},
		{"MTH$CVT_D_G", "D", "G", "1.5", "1.5", 0},
		{"MTH$CVT_G_D", "G", "D", "1.5", "1.5", 0},
		{"MTH$CVT_G_D", "G", "D", "1.0E300", "", mthFLOOVE},
		{"MTH$CVT_G_D", "G", "D", "1.0E-300", "0", 0},
	} {
		integer(row.entry, row.in, row.out, strings.Fields(row.args), row.want, row.condition)
	}

	// GPROD of 10^30 by itself is 10^60 (of the F values, so not exactly): look at
	// the answer only as far as that it's no condition.
	for i := range cases {
		if cases[i].entry == "MTH$GPROD" && cases[i].want == "" {
			cases[i].want = "*"
		}

		if cases[i].condition == mthFLOOVE {
			cases[i].count = 5
		}
	}

	// MTH$SNGLG and MTH$CVT_G_D below the range: quietly, and with FU set, which only
	// MTH$CVT_G_D minds on VMS 7.1. MTH$DPROD and MTH$DIM have no underflow, so FU makes
	// no difference to them.
	cases = append(cases,
		table1Case{entry: "MTH$DPROD", in: "F", out: "D", args: []string{"1.0E-30", "1.0E-30"}, want: "0", fu: true},
		table1Case{entry: "MTH$DIM", in: "F", out: "F", args: []string{"^X00010080", "^X00000080"}, want: "0", fu: true},
		table1Case{entry: "MTH$SNGLG", in: "G", out: "F", args: []string{"1.0E-300"}, want: "0"},
		table1Case{entry: "MTH$SNGLG", in: "G", out: "F", args: []string{"1.0E-300"}, want: "0", fu: true},
		table1Case{entry: "MTH$CVT_G_D", in: "G", out: "D", args: []string{"1.0E-300"}, want: "0", fu: true, condition: mthFLOUND, count: 5},
	)

	// The floating maximums: any number of arguments, H's after its result.
	for i, f := range table1Formats {
		maximum := names("MTH$AMAX1 MTH$DMAX1 MTH$GMAX1 MTH$HMAX1")[i]
		minimum := names("MTH$AMIN1 MTH$DMIN1 MTH$GMIN1 MTH$HMIN1")[i]

		for _, c := range []table1Case{
			{entry: maximum, args: []string{"1.5", "-2.0", "3.5"}, want: "3.5"},
			{entry: maximum, args: []string{"-1.5"}, want: "-1.5"},
			{entry: maximum, args: []string{"1.5", "", "3.5"}, want: "", condition: ssROPRAND, count: 3},
			{entry: maximum, args: nil, want: "0"},
			{entry: minimum, args: []string{"1.5", "-2.0", "3.5"}, want: "-2"},
			{entry: minimum, args: []string{"HUGE"}, want: "*"},
			{entry: minimum, args: []string{"1.5", ""}, want: "", condition: ssROPRAND, count: 3},
			{entry: minimum, args: nil, want: "0"},
		} {
			c.in, c.out = f.t, f.t
			for j, a := range c.args {
				c.args[j] = spell(a, f.huge, f.tiny)
			}

			cases = append(cases, c)
		}
	}

	// Arguments it can't reach, in each kind of routine.
	for _, c := range []table1Case{
		{entry: "MTH$ABS", in: "F", out: "F"},
		{entry: "MTH$HABS", in: "H", out: "H"},
		{entry: "MTH$JIABS", in: "L", out: "L"},
		{entry: "MTH$IIFIX", in: "F", out: "W"},
		{entry: "MTH$JMOD", in: "L", out: "L"},
		{entry: "MTH$JMAX0", in: "L", out: "L"},
		{entry: "MTH$HMAX1", in: "H", out: "H"},
		{entry: "MTH$DFLOTI", in: "W", out: "D"},
	} {
		c.args = []string{"BAD"}
		if c.entry == "MTH$JMOD" {
			c.args = []string{"BAD", "BAD"}
		}

		c.want, c.condition, c.count = "*", ssACCVIO, 5
		cases = append(cases, c)
	}

	return cases
}

var jsbEntryName = regexp.MustCompile(`_R[0-9]+$`)

// table1Program is the MACRO for the cases. A call's arguments are pushed last
// first, and an H result, the routine's first argument, last of all. After each
// call SHOW prints R0 to R3 and what the handler saw.
func table1Program(cases []table1Case) string {
	var data, code strings.Builder

	for i, c := range cases {
		fmt.Fprintf(&code, "\t; %d. %s\n\tCLRL\tSCOND\n\tCLRL\tSCOUNT\n", i, c.entry)

		if c.fu {
			code.WriteString("\tBISPSW\t#^X40\n")
		}

		labels := make([]string, len(c.args))

		for j, a := range c.args {
			labels[j] = fmt.Sprintf("A%d_%d", i, j)

			switch {
			case a == "BAD":
				labels[j] = ""
			case a == "":
				fmt.Fprintf(&data, "%s:\t.LONG\t^X8000,0,0,0\n", labels[j])
			case strings.HasPrefix(a, "^X") && c.in != "W" && c.in != "L" && c.in != "U":
				fmt.Fprintf(&data, "%s:\t.LONG\t%s,0,0,0\n", labels[j], a)
			default:
				fmt.Fprintf(&data, "%s:\t%s\t%s\n", labels[j], c.in.directive(), a)
			}
		}

		if jsbEntryName.MatchString(c.entry) {
			switch c.in {
			case "F":
				fmt.Fprintf(&code, "\tMOVL\t%s,R0\n", labels[0])
			case "H":
				fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n\tMOVQ\t%s+8,R2\n", labels[0], labels[0])
			default:
				fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n", labels[0])
			}

			fmt.Fprintf(&code, "\tJSB\tG^%s\n", c.entry)
		} else {
			count := len(c.args)

			for j := len(c.args) - 1; j >= 0; j-- {
				if labels[j] == "" {
					code.WriteString("\tPUSHL\t#^X3FFF0000\n")
				} else {
					fmt.Fprintf(&code, "\tPUSHAL\t%s\n", labels[j])
				}
			}

			if c.out == "H" {
				// What a reserved argument leaves in the result: this.
				code.WriteString("\tMOVL\t#^X8000,HRES\n\tCLRL\tHRES+4\n\tCLRQ\tHRES+8\n\tPUSHAL\tHRES\n")

				count++
			}

			fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n", count, c.entry)

			if c.out == "H" {
				code.WriteString("\tMOVQ\tHRES,R0\n\tMOVQ\tHRES+8,R2\n")
			}
		}

		if c.fu {
			code.WriteString("\tBICPSW\t#^X40\n")
		}

		code.WriteString("\tJSB\tSHOW\n")
	}

	return showProgram("TABLE1", data.String(), code.String())
}

// showProgram is a program of calls: the data and code given, a handler that keeps
// the signal array's size and condition, and SHOW, which prints R0 to R3 and those
// two.
func showProgram(title, data, code string) string {
	return `	.TITLE	` + title + `
	.PSECT	DATA,WRT,NOEXE,LONG
HRES:	.BLKL	4
SCOND:	.BLKL	1
SCOUNT:	.BLKL	1
CTL:	.ASCID	/!XL !XL !XL !XL c=!XL n=!UL/
BUF:	.BLKB	132
OUT:	.LONG	132
	.ADDRESS BUF
LEN:	.BLKL	1
LINE:	.BLKL	2
` + data + `	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	MOVL	(R0),SCOUNT
	MOVL	4(R0),SCOND
	MOVL	#1,R0
	RET
SHOW:	PUSHL	SCOUNT
	PUSHL	SCOND
	PUSHL	R3
	PUSHL	R2
	PUSHL	R1
	PUSHL	R0
	PUSHAL	OUT
	PUSHAL	LEN
	PUSHAQ	CTL
	CALLS	#9,G^SYS$FAO
	MOVL	LEN,LINE
	MOVAB	BUF,LINE+4
	PUSHAQ	LINE
	CALLS	#1,G^LIB$PUT_OUTPUT
	RSB
	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7,R8,R9>
	MOVAL	HANDLER,(FP)
` + code + `	MOVL	#1,R0
	RET
	.END	START
`
}

// wantRegisters is what a case should leave in R0 and up: the longwords of the
// result's type, as text.
func wantRegisters(t *testing.T, c table1Case) []string {
	t.Helper()

	f, isFloat := c.out.format()
	if !isFloat {
		i, err := strconv.ParseInt(c.want, 10, 64)
		if err != nil {
			t.Fatalf("%s: %q: %v", c.entry, c.want, err)
		}

		if c.out == "W" { // a word result's high word is clear
			i &= 0xFFFF
		}

		return []string{fmt.Sprintf("%08X", uint32(i))}
	}

	bits := vaxfloat.Reserved(f)

	if c.want != "" {
		x, err := vaxfloat.Parse(c.want)
		if err != nil {
			t.Fatal(err)
		}

		v, err := vaxfloat.Round(f, x)
		if err != nil {
			t.Fatal(err)
		}

		bits, err = vaxfloat.Pack(f, v)
		if err != nil {
			t.Fatal(err)
		}
	}

	words := []uint32{uint32(bits.Lo), uint32(bits.Lo >> 32), uint32(bits.Hi), uint32(bits.Hi >> 32)}[:f.Size()/4]
	text := make([]string, len(words))

	for i, w := range words {
		text[i] = fmt.Sprintf("%08X", w)
	}

	return text
}

func TestTable1FromAProgram(t *testing.T) {
	_, d, out := boot(t)
	cases := table1Cases()

	build(t, d, out, "TABLE1", table1Program(cases))

	if err := d.Dispatch("RUN TABLE1"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(cases), out)
	}

	for i, c := range cases {
		fields := strings.Fields(lines[i])
		name := fmt.Sprintf("%d. %s%v", i, c.entry, c.args)

		if c.want != "*" {
			want := wantRegisters(t, c)
			if got := strings.Join(fields[:len(want)], " "); got != strings.Join(want, " ") {
				t.Errorf("%s: registers %s, want %s", name, got, strings.Join(want, " "))
			}
		}

		if got, want := fields[4], fmt.Sprintf("c=%08X", c.condition); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}

		if got, want := fields[5], fmt.Sprintf("n=%d", c.count); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
}

// TestTable1Covers: the program calls every entry this file is about, each of the
// survey's families in full, so a table line with a wrong routine can't hide.
func TestTable1Covers(t *testing.T) {
	called := map[string]bool{"MTH$CVT_DA_GA": true, "MTH$CVT_GA_DA": true} // TestConvertArray's

	for _, c := range table1Cases() {
		called[c.entry] = true
	}

	for _, stem := range strings.Fields(`
		ABS DABS GABS HABS IIABS JIABS
		DIM DDIM GDIM HDIM IIDIM JIDIM
		SIGN DSIGN GSIGN HSIGN IISIGN JISIGN SGN
		FLOOR DFLOOR GFLOOR HFLOOR FLOOR_R1 DFLOOR_R3 GFLOOR_R3 HFLOOR_R7
		AINT DINT GINT HINT AINT_R2 DINT_R4 GINT_R4 HINT_R8
		IIFIX IINT JIFIX JINT IIDINT JIDINT IIGINT JIGINT IIHINT JIHINT
		ANINT DNINT GNINT HNINT ININT JNINT IIDNNT JIDNNT IIGNNT JIGNNT IIHNNT JIHNNT
		AMOD DMOD GMOD HMOD IMOD JMOD
		AIMAX0 AJMAX0 IMAX0 JMAX0 AMAX1 DMAX1 GMAX1 HMAX1 IMAX1 JMAX1 UMAX
		AIMIN0 AJMIN0 IMIN0 JMIN0 AMIN1 DMIN1 GMIN1 HMIN1 IMIN1 JMIN1 UMIN
		DBLE GDBLE SNGL SNGLG FLOATI DFLOTI GFLOTI FLOATJ DFLOTJ GFLOTJ
		DPROD GPROD CVT_D_G CVT_G_D CVT_DA_GA CVT_GA_DA
		IIAND JIAND IIOR JIOR IIEOR JIEOR INOT JNOT IISHFT JISHFT`) {
		if !called["MTH$"+stem] {
			t.Errorf("no case calls MTH$%s", stem)
		}
	}
}

// TestConvertArray runs the two array routines: a default count of 1, a count
// given, a count of 0, in place, an element out of range (kept going past, and
// signaled once at the end), and a reserved operand, which ends the loop there.
func TestConvertArray(t *testing.T) {
	_, d, out := boot(t)

	build(t, d, out, "ARRAYS", `	.TITLE	ARRAYS
	.PSECT	DATA,WRT,NOEXE,LONG
IN:	.D_FLOATING	1.5,-2.5,0.75
INO:	.G_FLOATING	1.5
	.G_FLOATING	1.0E300
	.G_FLOATING	-2.5
RES:	.G_FLOATING	1.5
	.LONG	^X8000,0
	.G_FLOATING	2.5
SMALL:	.G_FLOATING	1.0E-300
	.G_FLOATING	-2.5
THREE:	.LONG	3
TWO:	.LONG	2
ZERO:	.LONG	0
DST:	.BLKQ	4
SCOND:	.BLKL	1
SCOUNT:	.BLKL	1
CTL:	.ASCID	/!XL !XL !XL !XL !XL !XL !XL !XL r0=!XL c=!XL n=!UL/
BUF:	.BLKB	132
OUT:	.LONG	132
	.ADDRESS BUF
LEN:	.BLKL	1
LINE:	.BLKL	2
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	INCL	SCOUNT
	MOVL	4(R0),SCOND
	MOVL	#1,R0
	RET
	; SHOW prints DST, R0, and what the handler saw, and clears them.
SHOW:	MOVAB	DST,R1
	PUSHL	SCOUNT
	PUSHL	SCOND
	PUSHL	R0
	PUSHL	28(R1)
	PUSHL	24(R1)
	PUSHL	20(R1)
	PUSHL	16(R1)
	PUSHL	12(R1)
	PUSHL	8(R1)
	PUSHL	4(R1)
	PUSHL	(R1)
	PUSHAL	OUT
	PUSHAL	LEN
	PUSHAQ	CTL
	CALLS	#14,G^SYS$FAO
	MOVL	LEN,LINE
	MOVAB	BUF,LINE+4
	PUSHAQ	LINE
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVC5	#0,DST,#0,#32,DST
	CLRL	SCOND
	CLRL	SCOUNT
	RSB
	.ENTRY	START,^M<R2,R3,R4,R5>
	MOVAL	HANDLER,(FP)
	; 1. No count: one element.
	PUSHAQ	DST
	PUSHAQ	IN
	CALLS	#2,G^MTH$CVT_DA_GA
	BSBW	SHOW
	; 2. Three elements.
	PUSHAL	THREE
	PUSHAQ	DST
	PUSHAQ	IN
	CALLS	#3,G^MTH$CVT_DA_GA
	BSBW	SHOW
	; 3. A count of 0: nothing.
	PUSHAL	ZERO
	PUSHAQ	DST
	PUSHAQ	IN
	CALLS	#3,G^MTH$CVT_DA_GA
	BSBW	SHOW
	; 4. The count's address 0: the default, as if it weren't given.
	PUSHL	#0
	PUSHAQ	DST
	PUSHAQ	IN
	CALLS	#3,G^MTH$CVT_DA_GA
	BSBW	SHOW
	; 5. G to D, the second element too large.
	PUSHAL	THREE
	PUSHAQ	DST
	PUSHAQ	INO
	CALLS	#3,G^MTH$CVT_GA_DA
	BSBW	SHOW
	; 6. A reserved operand second: the first is done, and the third isn't.
	PUSHAL	THREE
	PUSHAQ	DST
	PUSHAQ	RES
	CALLS	#3,G^MTH$CVT_GA_DA
	BSBW	SHOW
	; 7. An input it can't read.
	PUSHL	#^X3FFF0000
	PUSHAQ	DST
	PUSHL	#^X3FFF0000
	CALLS	#3,G^MTH$CVT_GA_DA
	BSBW	SHOW
	; 8. The first element too small for D_floating: quietly 0.0 ...
	PUSHAL	TWO
	PUSHAQ	DST
	PUSHAQ	SMALL
	CALLS	#3,G^MTH$CVT_GA_DA
	BSBW	SHOW
	; 9. ... and with FU set, signaled, the second still converted.
	BISPSW	#^X40
	PUSHAL	TWO
	PUSHAQ	DST
	PUSHAQ	SMALL
	CALLS	#3,G^MTH$CVT_GA_DA
	BICPSW	#^X40
	BSBW	SHOW
	MOVL	#1,R0
	RET
	.END	START
`)

	if err := d.Dispatch("RUN ARRAYS"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	words := func(f vaxfloat.Format, values ...string) string {
		var text []string

		for _, s := range values {
			var b vaxfloat.Bits

			switch s {
			case "reserved":
				b = vaxfloat.Reserved(f)
			case "0":
			default:
				x, err := vaxfloat.Parse(s)
				if err != nil {
					t.Fatal(err)
				}

				v, err := vaxfloat.Round(f, x)
				if err != nil {
					t.Fatal(err)
				}

				b, _ = vaxfloat.Pack(f, v)
			}

			text = append(text, fmt.Sprintf("%08X %08X", uint32(b.Lo), uint32(b.Lo>>32)))
		}

		for len(text) < 4 {
			text = append(text, "00000000 00000000")
		}

		return strings.Join(text, " ")
	}

	wants := []struct {
		memory    string
		condition uint32
		count     int
	}{
		{words(vaxfloat.G, "1.5"), 0, 0},
		{words(vaxfloat.G, "1.5", "-2.5", "0.75"), 0, 0},
		{words(vaxfloat.G), 0, 0},
		{words(vaxfloat.G, "1.5"), 0, 0},
		{words(vaxfloat.D, "1.5", "reserved", "-2.5"), mthFLOOVE, 1},
		{words(vaxfloat.D, "1.5"), ssROPRAND, 1},
		{words(vaxfloat.D), ssACCVIO, 1},
		{words(vaxfloat.D, "0", "-2.5"), 0, 0},
		{words(vaxfloat.D, "0", "-2.5"), mthFLOUND, 1},
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(wants) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(wants), out)
	}

	for i, want := range wants {
		fields := strings.Fields(lines[i])

		if got := strings.Join(fields[:8], " "); got != want.memory {
			t.Errorf("step %d: memory\n  got %s\n want %s", i+1, got, want.memory)
		}

		// With no condition R0 is 0, on VMS 7.1.
		if want.condition == 0 && fields[8] != "r0=00000000" {
			t.Errorf("step %d: %s, want 0", i+1, fields[8])
		}

		if got, wantCond := fields[9], fmt.Sprintf("c=%08X", want.condition); got != wantCond {
			t.Errorf("step %d: %s, want %s", i+1, got, wantCond)
		}

		if got, wantCount := fields[10], fmt.Sprintf("n=%d", want.count); got != wantCount {
			t.Errorf("step %d: %s, want %s", i+1, got, wantCount)
		}
	}
}

// TestHardwareConditionsUnhandled: with no handler, an integer overflow and an
// integer divide by zero are SYSTEM's messages, and end the image, though the
// program's own frame leaves IV clear: the routine's mask sets it.
func TestHardwareConditionsUnhandled(t *testing.T) {
	for _, c := range []struct {
		name, data, call, message string
	}{
		{"JIABS", "X:\t.LONG\t^X80000000", "PUSHAL\tX\n\tCALLS\t#1,G^MTH$JIABS", "%SYSTEM-F-INTOVF, "},
		{"JMOD", "X:\t.LONG\t7,0", "PUSHAL\tX+4\n\tPUSHAL\tX\n\tCALLS\t#2,G^MTH$JMOD", "%SYSTEM-F-INTDIV, "},
		{"AMOD", "X:\t.FLOAT\t7.0,0.0", "PUSHAL\tX+4\n\tPUSHAL\tX\n\tCALLS\t#2,G^MTH$AMOD", "%MTH-F-INVARGMAT, "},
		{"DPROD", "X:\t.FLOAT\t1.0E30,1.0E30", "PUSHAL\tX+4\n\tPUSHAL\tX\n\tCALLS\t#2,G^MTH$DPROD", "%SYSTEM-F-FLTOVF_F, "},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, d, out := boot(t)

			build(t, d, out, "HARD", `	.TITLE	HARD
	.PSECT	DATA,WRT,NOEXE,LONG
`+c.data+`
WENT:	.ASCID	/WENT ON/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	`+c.call+`
	PUSHAQ	WENT
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.END	START
`)

			_ = d.Dispatch("RUN HARD") // the image's end is what's looked at

			text := out.String()

			if !strings.Contains(text, c.message) {
				t.Errorf("no %s message; RUN said:\n%s", c.message, text)
			}

			if strings.Contains(text, "WENT ON") {
				t.Errorf("the image went on after a severe condition:\n%s", text)
			}
		})
	}
}
