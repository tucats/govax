package mthrtl_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// The OTS$ routines from a real program: every entry, its arguments by value in the
// argument list, its result in R0 and up, each condition continued from a handler,
// the signal array's size, FU, and a reserved operand in each place.

var mthUNDEXP = vmsdef.LibrarySymbols["MTH$_UNDEXP"]

// otsArg is one argument: a literal of its type, "" for a reserved operand, or for
// an integer type a longword's text as MACRO spells it.
type otsArg struct {
	t table1Type
	v string
}

// otsCase is one call. want is the result's values, each of type out, "" for the
// reserved operand and "*" not to look. A result that doesn't use R2 and R3 must
// leave them as they were.
type otsCase struct {
	entry     string
	args      []otsArg
	out       table1Type
	want      []string
	condition uint32
	count     int
	fu        bool
}

// words is how many longwords a value of type t takes.
func (t table1Type) words() int {
	switch t {
	case "H":
		return 4
	case "D", "G":
		return 2
	}

	return 1
}

// The routines by format.
var realPowerNames = map[table1Type]struct{ floating, signed, unsigned string }{
	"F": {"OTS$POWRR", "OTS$POWRJ", "OTS$POWRLU"},
	"D": {"OTS$POWDD", "OTS$POWDJ", "OTS$POWDLU"},
	"G": {"OTS$POWGG", "OTS$POWGJ", "OTS$POWGLU"},
	"H": {"OTS$POWHH_R3", "OTS$POWHJ_R3", "OTS$POWHLU_R3"},
}

var complexNames = map[table1Type]struct{ power, integer, divide, multiply string }{
	"F": {"OTS$POWCC", "OTS$POWCJ", "OTS$DIVC", ""},
	"D": {"OTS$POWCDCD_R3", "OTS$POWCDJ_R3", "OTS$DIVCD_R3", "OTS$MULCD_R3"},
	"G": {"OTS$POWCGCG_R3", "OTS$POWCGJ_R3", "OTS$DIVCG_R3", "OTS$MULCG_R3"},
}

// failedComplex is what a condition leaves in a complex result's registers: the
// reserved operand in the real part, and everything else clear.
func failedComplex() []string { return []string{"", "0.0"} }

func otsCases() []otsCase {
	var cases []otsCase

	add := func(entry string, out table1Type, condition uint32, count int, fu bool, want []string, args ...otsArg) {
		cases = append(cases, otsCase{entry, args, out, want, condition, count, fu})
	}

	one := func(v string) []string { return []string{v} }
	floats := func(t table1Type, values ...string) []otsArg {
		args := make([]otsArg, len(values))
		for i, v := range values {
			args[i] = otsArg{t, v}
		}

		return args
	}

	const (
		sqrt2 = "1.41421356237309504880168872420969807856967187537694807317668"
		third = "0.333333333333333333333333333333333333333333333333333333333333"
	)

	ssFLTDIV := vmsdef.Symbols["SS$_FLTDIV_F"]
	ssFLTOVF := vmsdef.Symbols["SS$_FLTOVF_F"]
	ssINTOVF := vmsdef.Symbols["SS$_INTOVF"]
	ssROPRAND := vmsdef.Symbols["SS$_ROPRAND"]

	for _, t := range []table1Type{"F", "D", "G", "H"} {
		names := realPowerNames[t]

		// A floating exponent.
		pow := func(want []string, condition uint32, count int, fu bool, base, y string) {
			add(names.floating, t, condition, count, fu, want, floats(t, base, y)...)
		}

		pow(one(sqrt2), 0, 0, false, "2.0", "0.5")
		pow(one("64.0"), 0, 0, false, "8.0", "2.0")
		pow(one(third), 0, 0, false, "9.0", "-0.5")
		pow(one("0.0"), 0, 0, false, "0.0", "2.0")
		pow(one("1.0"), 0, 0, false, "3.0", "0.0")
		pow(one(""), mthUNDEXP, 5, false, "-2.0", "2.0")
		pow(one(""), mthUNDEXP, 5, false, "-2.0", "0.0")
		pow(one(""), mthUNDEXP, 5, false, "0.0", "0.0")
		pow(one(""), mthUNDEXP, 5, false, "0.0", "-1.0")
		pow(one(""), mthFLOOVE, 5, false, "10.0", "6000.0")
		pow(one("0.0"), 0, 0, false, "10.0", "-6000.0")
		pow(one("0.0"), mthFLOUND, 5, true, "10.0", "-6000.0")
		pow(one("*"), ssROPRAND, 3, false, "", "2.0")
		pow(one("*"), ssROPRAND, 3, false, "2.0", "")

		// An integer exponent, signed and unsigned.
		integer := func(entry string, u table1Type, want []string, condition uint32, count int, fu bool, base, n string) {
			add(entry, t, condition, count, fu, want, otsArg{t, base}, otsArg{u, n})
		}

		for _, kind := range []struct {
			entry string
			u     table1Type
		}{{names.signed, "L"}, {names.unsigned, "U"}} {
			e, u := kind.entry, kind.u

			integer(e, u, one("1024.0"), 0, 0, false, "2.0", "10")
			integer(e, u, one("-8.0"), 0, 0, false, "-2.0", "3")
			integer(e, u, one("1.0"), 0, 0, false, "-2.0", "0")
			integer(e, u, one("0.0"), 0, 0, false, "0.0", "3")
			integer(e, u, one(""), mthUNDEXP, 5, false, "0.0", "0")
			integer(e, u, one(""), mthFLOOVE, 5, false, "10.0", "100000")
			integer(e, u, one("0.0"), 0, 0, false, "0.1", "100000")
			integer(e, u, one("0.0"), mthFLOUND, 5, true, "0.1", "100000")
			integer(e, u, one("*"), ssROPRAND, 3, false, "", "2")
			integer(e, u, one("1.0"), 0, 0, false, "1.0", "^XFFFFFFFF")
		}

		// Only the signed ones have these; the same longword means something else to
		// the unsigned ones.
		s, l := names.signed, table1Type("L")
		integer(s, l, one("0.25"), 0, 0, false, "2.0", "-2")
		integer(s, l, one("-0.125"), 0, 0, false, "-2.0", "-3")
		integer(s, l, one(""), mthUNDEXP, 5, false, "0.0", "-1")
		integer(s, l, one(""), ssFLTDIV, 3, false, "0.5", "-100000")
		integer(s, l, one("*"), 0, 0, false, "1.0", "-1")
		integer(names.unsigned, "U", one("0.0"), 0, 0, false, "0.5", "^X80000000")
		integer(s, l, one(""), ssFLTDIV, 3, false, "0.5", "^X80000000")
	}

	// The order: 1.0 over the product, whose overflow comes first, the base's
	// reciprocal being in range.
	add("OTS$POWRJ", "F", mthFLOOVE, 5, false, one(""), otsArg{"F", "2.0"}, otsArg{"L", "-127"})
	add("OTS$POWRJ", "F", 0, 0, false, one("1.1754943508222875E-38"), otsArg{"F", "2.0"}, otsArg{"L", "-126"})

	// The same bits as FORTRAN's folding has for these constants
	// (sema_test.go's TestParameterValues).
	add("OTS$POWRJ", "F", 0, 0, false, one("0.004115226337448559670781893"), otsArg{"F", "3.0"}, otsArg{"L", "-5"})
	add("OTS$POWDJ", "D", 0, 0, false, one("0.00291545189504373177842565597667638"), otsArg{"D", "7.0"}, otsArg{"L", "-3"})

	// The mixed formats: the F side's bits can't hold what the D side's have.
	const d40 = "1.0000000000009094947017729282379150390625"

	add("OTS$POWRD", "D", 0, 0, false, one("2.00000000000126082737653662442271986"), otsArg{"F", "2.0"}, otsArg{"D", d40})
	add("OTS$POWDR", "D", 0, 0, false, one("1.00000000000045474735088636072138095"), otsArg{"D", d40}, otsArg{"F", "0.5"})
	add("OTS$POWRD", "D", mthUNDEXP, 5, false, one(""), otsArg{"F", "-2.0"}, otsArg{"D", "1.0"})
	add("OTS$POWDR", "D", mthUNDEXP, 5, false, one(""), otsArg{"D", "0.0"}, otsArg{"F", "0.0"})
	add("OTS$POWRD", "D", mthFLOOVE, 5, false, one(""), otsArg{"F", "10.0"}, otsArg{"D", "1000.0"})
	add("OTS$POWDR", "D", mthFLOOVE, 5, false, one(""), otsArg{"D", "10.0"}, otsArg{"F", "1000.0"})
	add("OTS$POWRD", "D", mthFLOUND, 5, true, one("0.0"), otsArg{"F", "10.0"}, otsArg{"D", "-1000.0"})
	add("OTS$POWDR", "D", 0, 0, false, one("0.0"), otsArg{"D", "10.0"}, otsArg{"F", "-1000.0"})
	add("OTS$POWRD", "D", ssROPRAND, 3, false, one("*"), otsArg{"F", ""}, otsArg{"D", "1.0"})
	add("OTS$POWRD", "D", ssROPRAND, 3, false, one("*"), otsArg{"F", "2.0"}, otsArg{"D", ""})
	add("OTS$POWDR", "D", ssROPRAND, 3, false, one("*"), otsArg{"D", ""}, otsArg{"F", "1.0"})
	add("OTS$POWDR", "D", ssROPRAND, 3, false, one("*"), otsArg{"D", "2.0"}, otsArg{"F", ""})

	// Integers. The high word of a word argument's longword isn't the argument's.
	integers := func(entry string, t table1Type, values ...[3]string) {
		for _, v := range values {
			add(entry, t, 0, 0, false, one(v[2]), otsArg{"L", v[0]}, otsArg{"L", v[1]})
		}
	}

	integers("OTS$POWII", "W",
		[3]string{"2", "10", "1024"}, [3]string{"-2", "3", "-8"}, [3]string{"7", "0", "1"},
		[3]string{"2", "-1", "0"}, [3]string{"1", "-5", "1"}, [3]string{"-1", "-3", "-1"},
		[3]string{"2", "14", "16384"}, [3]string{"-2", "15", "-32768"},
		[3]string{"^X00010003", "^X00020002", "9"})
	add("OTS$POWII", "W", ssINTOVF, 3, false, one("-32768"), otsArg{"L", "2"}, otsArg{"L", "15"})
	add("OTS$POWII", "W", mthUNDEXP, 5, false, one("32768"), otsArg{"L", "0"}, otsArg{"L", "0"})
	add("OTS$POWII", "W", mthUNDEXP, 5, false, one("32768"), otsArg{"L", "0"}, otsArg{"L", "-1"})

	integers("OTS$POWJJ", "L",
		[3]string{"2", "30", "1073741824"}, [3]string{"-2", "31", "-2147483648"}, [3]string{"7", "0", "1"},
		[3]string{"2", "-1", "0"}, [3]string{"-1", "^X7FFFFFFF", "-1"}, [3]string{"^X00010000", "1", "65536"})
	add("OTS$POWJJ", "L", ssINTOVF, 3, false, one("-2147483648"), otsArg{"L", "2"}, otsArg{"L", "31"})
	add("OTS$POWJJ", "L", ssINTOVF, 3, false, one("3476558801"), otsArg{"L", "3"}, otsArg{"L", "100"})
	add("OTS$POWJJ", "L", mthUNDEXP, 5, false, one("32768"), otsArg{"L", "0"}, otsArg{"L", "0"})

	integers("OTS$POWLULU", "U",
		[3]string{"2", "31", "2147483648"}, [3]string{"2", "32", "0"}, [3]string{"3", "100", "3476558801"},
		[3]string{"^XFFFFFFFF", "2", "1"}, [3]string{"0", "5", "0"}, [3]string{"5", "0", "1"})
	add("OTS$POWLULU", "U", mthUNDEXP, 5, false, one("32768"), otsArg{"L", "0"}, otsArg{"L", "0"})

	// Complex.
	for _, t := range []table1Type{"F", "D", "G"} {
		names := complexNames[t]
		pair := func(values ...string) []otsArg { return floats(t, values...) }
		failed := failedComplex()

		add(names.power, t, 0, 0, false,
			[]string{"-0.463956500815208434911673517153658", "-0.199530085331553850016755433099421"},
			pair("2.0", "3.0", "1.0", "2.0")...)
		add(names.power, t, 0, 0, false, []string{"2.0", "3.0"}, pair("2.0", "3.0", "1.0", "0.0")...)
		add(names.power, t, 0, 0, false, []string{"1.0", "0.0"}, pair("2.0", "3.0", "0.0", "0.0")...)
		add(names.power, t, mthINVARG, 5, false, failed, pair("0.0", "0.0", "1.0", "0.0")...)
		add(names.power, t, mthFLOOVE, 5, false, failed, pair("10.0", "0.0", "1000.0", "0.0")...)
		add(names.power, t, 0, 0, true, []string{"0.0", "0.0"}, pair("10.0", "0.0", "-1000.0", "0.0")...)

		for place := range 4 {
			args := pair("2.0", "3.0", "1.0", "2.0")
			args[place].v = ""
			add(names.power, t, ssROPRAND, 3, false, []string{"*", "*"}, args...)
		}

		integer := func(want []string, condition uint32, count int, re, im, n string) {
			add(names.integer, t, condition, count, false, want, otsArg{t, re}, otsArg{t, im}, otsArg{"L", n})
		}

		integer([]string{"-5.0", "12.0"}, 0, 0, "2.0", "3.0", "2")
		integer([]string{"2.0", "3.0"}, 0, 0, "2.0", "3.0", "1")
		integer([]string{"1.0", "0.0"}, 0, 0, "2.0", "3.0", "0")
		integer([]string{"0.0", "-1.0"}, 0, 0, "0.0", "1.0", "-1")
		integer([]string{"0.153846153846153846153846153846154", "-0.230769230769230769230769230769231"}, 0, 0, "2.0", "3.0", "-1")
		integer([]string{"0.0", "0.0"}, 0, 0, "0.0", "0.0", "3")
		integer(failed, mthUNDEXP, 5, "0.0", "0.0", "0")
		integer(failed, mthUNDEXP, 5, "0.0", "0.0", "-1")
		integer(failed, ssFLTOVF, 3, "10.0", "10.0", "100000")
		integer(failed, ssFLTDIV, 3, "0.1", "0.1", "-100000")
		integer([]string{"*", "*"}, ssROPRAND, 3, "", "3.0", "2")
		integer([]string{"*", "*"}, ssROPRAND, 3, "2.0", "", "2")

		large := "1.0E38"
		if t == "G" {
			large = "1.0E306"
		}

		add(names.divide, t, 0, 0, false, []string{"6.0", "-2.0"}, pair("8.0", "4.0", "1.0", "1.0")...)
		add(names.divide, t, 0, 0, false, []string{"0.44", "0.08"}, pair("1.0", "2.0", "3.0", "4.0")...)
		add(names.divide, t, ssFLTDIV, 3, false, failed, pair("1.0", "1.0", "0.0", "0.0")...)
		add(names.divide, t, ssFLTOVF, 3, false, failed, pair(large, "0.0", "0.01", "0.0")...)

		for place := range 4 {
			args := pair("8.0", "4.0", "1.0", "1.0")
			args[place].v = ""
			add(names.divide, t, ssROPRAND, 3, false, []string{"*", "*"}, args...)
		}

		if names.multiply == "" {
			continue // the compiler writes an F complex product in line
		}

		add(names.multiply, t, 0, 0, false, []string{"4.0", "32.0"}, pair("8.0", "4.0", "2.0", "3.0")...)
		add(names.multiply, t, mthFLOOVE, 5, false, failed, pair(large, "0.0", large, "0.0")...)

		for place := range 4 {
			args := pair("8.0", "4.0", "2.0", "3.0")
			args[place].v = ""
			add(names.multiply, t, ssROPRAND, 3, false, []string{"*", "*"}, args...)
		}
	}

	return cases
}

// otsProgram makes the program: each case's arguments are data, pushed a longword at
// a time, the last argument's last longword first, so that they lie in order above
// the argument count, which is what "by value" means.
func otsProgram(cases []otsCase) string {
	var data, code strings.Builder

	for i, c := range cases {
		label := fmt.Sprintf("A%d", i)

		fmt.Fprintf(&data, "%s:\n", label)

		count := 0

		for _, a := range c.args {
			count += a.t.words()

			switch {
			case a.v == "":
				fmt.Fprintf(&data, "\t.LONG\t^X8000\n")

				for range a.t.words() - 1 {
					data.WriteString("\t.LONG\t0\n")
				}
			case a.t == "L" || a.t == "U":
				fmt.Fprintf(&data, "\t.LONG\t%s\n", a.v)
			default:
				fmt.Fprintf(&data, "\t%s\t%s\n", a.t.directive(), a.v)
			}
		}

		fmt.Fprintf(&code, "\t; %d. %s\n\tCLRL\tSCOND\n\tCLRL\tSCOUNT\n", i, c.entry)
		code.WriteString("\tMOVL\t#^X12345678,R1\n\tMOVL\t#^X12345678,R2\n\tMOVL\t#^X12345678,R3\n")

		if c.fu {
			code.WriteString("\tBISPSW\t#^X40\n")
		}

		for w := count - 1; w >= 0; w-- {
			fmt.Fprintf(&code, "\tPUSHL\t%s+%d\n", label, 4*w)
		}

		fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n", count, c.entry)

		if c.fu {
			code.WriteString("\tBICPSW\t#^X40\n")
		}

		code.WriteString("\tJSB\tSHOW\n")
	}

	return showProgram("OTSP", data.String(), code.String())
}

// otsWant is what a case should leave in R0 to R3, as text, "*" not to look.
func otsWant(t *testing.T, c otsCase) []string {
	t.Helper()

	var words []string

	for _, v := range c.want {
		if v == "*" {
			for range c.out.words() {
				words = append(words, "*")
			}

			continue
		}

		words = append(words, wantRegisters(t, table1Case{entry: c.entry, out: c.out, want: v})...)
	}

	for len(words) < 4 {
		words = append(words, "12345678")
	}

	if len(c.want) == 1 && c.want[0] == "*" {
		return []string{"*", "*", "*", "*"}
	}

	// A condition's dispatch leaves R1 clear for the reserved operand, whatever size
	// the result: only an F result doesn't care.
	if c.condition != 0 && len(words) > 1 && words[1] == "12345678" {
		words[1] = "*"
	}

	return words
}

func TestOTSFromAProgram(t *testing.T) {
	_, d, out := boot(t)
	cases := otsCases()

	build(t, d, out, "OTSP", otsProgram(cases))

	if err := d.Dispatch("RUN OTSP"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(cases), out)
	}

	for i, c := range cases {
		fields := strings.Fields(lines[i])
		name := fmt.Sprintf("%d. %s%v", i, c.entry, c.args)

		for j, want := range otsWant(t, c) {
			if want != "*" && fields[j] != want {
				t.Errorf("%s: R%d is %s, want %s", name, j, fields[j], want)
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

// TestOTSCovers: the program calls every OTS$ name in the transfer vector, all 28.
func TestOTSCovers(t *testing.T) {
	called := map[string]bool{}

	for _, c := range otsCases() {
		called[c.entry] = true
	}

	var names []string

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image == "MTHRTL" && strings.HasPrefix(name, "OTS$") {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	if len(names) != 28 {
		t.Errorf("%d OTS$ names, want the survey's 28: %v", len(names), names)
	}

	for _, name := range names {
		if !called[name] {
			t.Errorf("no case calls %s", name)
		}
	}
}

// TestOTSUnhandled: with no handler, the message and the image's end.
func TestOTSUnhandled(t *testing.T) {
	for _, c := range []struct {
		name, data, call, message string
	}{
		{"POWRR", "A:\t.FLOAT\t-2.0,1.0", "PUSHL\tA+4\n\tPUSHL\tA\n\tCALLS\t#2,G^OTS$POWRR", "%MTH-F-UNDEXP, "},
		{"POWJJ", "A:\t.LONG\t2,31", "PUSHL\tA+4\n\tPUSHL\tA\n\tCALLS\t#2,G^OTS$POWJJ", "%SYSTEM-F-INTOVF, "},
		{"DIVC", "A:\t.FLOAT\t1.0,1.0,0.0,0.0", "PUSHL\tA+12\n\tPUSHL\tA+8\n\tPUSHL\tA+4\n\tPUSHL\tA\n\tCALLS\t#4,G^OTS$DIVC", "%SYSTEM-F-FLTDIV_F, "},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, d, out := boot(t)

			build(t, d, out, "OTSEND", `	.TITLE	OTSEND
	.PSECT	DATA,WRT,NOEXE,LONG
`+c.data+`
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	`+c.call+`
	CALLS	#0,G^MTH$RANDOM
	RET
	.END	START
`)

			_ = d.Dispatch("RUN OTSEND") // the image's end is what's looked at

			text := out.String()

			if !strings.Contains(text, c.message) {
				t.Errorf("no %s message; RUN said:\n%s", c.message, text)
			}

			if strings.Contains(text, "MTH$RANDOM") {
				t.Errorf("the image went on after a severe condition:\n%s", text)
			}
		})
	}
}
