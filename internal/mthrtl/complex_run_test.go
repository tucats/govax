package mthrtl_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// The complex functions and parts from a real program: every entry, its answer in
// the registers or memory its page gives, each condition continued from a handler,
// the signal array's size, a reserved operand in each part, and arguments it can't
// read or write.

// complexKind says what a routine takes and gives.
type complexKind int

const (
	complexToComplex complexKind = iota // the address of z; a complex result
	complexToReal                       // the address of z; one value in registers
	partsToComplex                      // the addresses of r and i; a complex result
)

// complexCase is one call. re and im are the argument's parts, each a literal of
// format f, "" for a reserved operand, or "^X.." for a part with that first longword
// and the rest clear. want is the result: its two parts, or for
// complexToReal only want[0], each a number, "" for the reserved operand, or "*" not
// to look.
type complexCase struct {
	entry     string
	f         table1Type
	kind      complexKind
	re, im    string
	want      [2]string
	condition uint32
	count     int
	fu        bool // the caller has set PSW<FU>
	badIn     bool // the argument's address is one no program can reach
	badOut    bool // and the result's
	inPlace   bool // the result's address is the argument's
}

var mthINVARG = vmsdef.LibrarySymbols["MTH$_INVARGMAT"]

// The routines, each family in format order F, D, G.
const (
	familyAbs  = "CABS CDABS CGABS"
	familyExp  = "CEXP CDEXP CGEXP"
	familyLog  = "CLOG CDLOG CGLOG"
	familySin  = "CSIN CDSIN CGSIN"
	familyCos  = "CCOS CDCOS CGCOS"
	familySqrt = "CSQRT CDSQRT CGSQRT"
	familyMake = "CMPLX DCMPLX GCMPLX"
	familyConj = "CONJG DCONJG GCONJG"
	familyReal = "REAL DREAL GREAL"
	familyImag = "AIMAG DIMAG GIMAG"
)

var complexFamilyFormats = []table1Type{"F", "D", "G"}

// complexLimits are each format's words for its large numbers, in a case's literals:
//
//	OVER  past the exponential's limit (and within COSH's)
//	NEAR  within the exponential's limit
//	HUGE  twice over past the range's modulus
//	BIG   a modulus that fits, but with a sum beside it that doesn't
//	LOW   half of that, whose sum fits
var complexLimits = map[table1Type]*strings.Replacer{
	"F": strings.NewReplacer("OVER", "88.5", "NEAR", "88.0", "HUGE", "1.5E38", "BIG", "1.0E38", "LOW", "5.0E37"),
	"D": strings.NewReplacer("OVER", "88.5", "NEAR", "88.0", "HUGE", "1.5E38", "BIG", "1.0E38", "LOW", "5.0E37"),
	"G": strings.NewReplacer("OVER", "709.5", "NEAR", "709.0", "HUGE", "8.0E307", "BIG", "6.0E307", "LOW", "3.0E307"),
}

// The results of a call that gave up: failed, what a condition leaves, and for a
// reserved argument unread, which leaves a result in memory as it was.
func failed(f table1Type) [2]string {
	if f == "F" {
		return [2]string{"", "0.0"}
	}

	return [2]string{"", ""}
}

func unread(f table1Type) [2]string {
	if f == "F" {
		return [2]string{"", "0.0"}
	}

	return [2]string{"*", "*"}
}

// complexCases makes the program's calls.
func complexCases() []complexCase {
	var cases []complexCase

	// add makes one case per name in names, the format following its place in the
	// family, mod finishing each.
	add := func(names string, kind complexKind, re, im string, mod func(*complexCase)) {
		for i, name := range strings.Fields(names) {
			f := complexFamilyFormats[i]
			c := complexCase{
				entry: "MTH$" + name, f: f, kind: kind,
				re: complexLimits[f].Replace(re), im: complexLimits[f].Replace(im),
			}

			mod(&c)
			cases = append(cases, c)
		}
	}

	answer := func(re, im string) func(*complexCase) {
		return func(c *complexCase) { c.want = [2]string{re, im} }
	}

	// Exact answers, and ones of many digits, the imaginary part different from the
	// real, so a routine for its neighbor gets caught, and both orders of the parts.
	add(familyAbs, complexToReal, "-3.0", "4.0", answer("5.0", ""))
	add(familyAbs, complexToReal, "0.0", "0.0", answer("0.0", ""))
	add(familyExp, complexToComplex, "0.0", "0.0", answer("1.0", "0.0"))
	add(familyExp, complexToComplex, "1.0", "1.0", answer("1.468693939915885157", "2.287355287178842391"))
	add(familyLog, complexToComplex, "1.0", "0.0", answer("0.0", "0.0"))
	add(familyLog, complexToComplex, "2.0", "1.0", answer("0.804718956217050187", "0.463647609000806116"))
	add(familySin, complexToComplex, "0.0", "0.0", answer("0.0", "0.0"))
	add(familySin, complexToComplex, "1.0", "1.0", answer("1.298457581415977295", "0.634963914784736108"))
	add(familyCos, complexToComplex, "0.0", "0.0", answer("1.0", "0.0"))
	add(familyCos, complexToComplex, "1.0", "1.0", answer("0.833730025131149049", "-0.988897705762865096"))
	add(familySqrt, complexToComplex, "3.0", "-4.0", answer("2.0", "-1.0"))
	add(familySqrt, complexToComplex, "-3.0", "4.0", answer("1.0", "2.0"))
	add(familySqrt, complexToComplex, "-3.0", "-4.0", answer("1.0", "-2.0"))
	add(familySqrt, complexToComplex, "0.0", "0.0", answer("0.0", "0.0"))
	add(familyMake, partsToComplex, "1.5", "-2.5", answer("1.5", "-2.5"))
	add(familyConj, complexToComplex, "1.5", "2.5", answer("1.5", "-2.5"))
	add(familyConj, complexToComplex, "1.5", "-2.5", answer("1.5", "2.5"))
	add(familyReal, complexToReal, "1.5", "2.5", answer("1.5", ""))
	add(familyImag, complexToReal, "1.5", "2.5", answer("2.5", ""))

	// A result written over its argument, by the forms with results in memory.
	inPlace := func(re, im string) func(*complexCase) {
		return func(c *complexCase) { c.want, c.inPlace = [2]string{re, im}, c.f != "F" }
	}

	add(familySqrt, complexToComplex, "3.0", "4.0", inPlace("2.0", "1.0"))
	add(familyExp, complexToComplex, "0.0", "0.0", inPlace("1.0", "0.0"))
	add(familyConj, complexToComplex, "1.5", "2.5", inPlace("1.5", "-2.5"))

	// The conditions, each continued from the handler.
	gives := func(condition uint32, count int) func(*complexCase) {
		return func(c *complexCase) {
			c.want, c.condition, c.count = failed(c.f), condition, count
		}
	}

	nothing := func(c *complexCase) { c.want = [2]string{"*", "*"} }

	add(familyAbs, complexToReal, "HUGE", "HUGE", gives(mthFLOOVE, 5))
	add(familyAbs, complexToReal, "BIG", "BIG", nothing)
	add(familyExp, complexToComplex, "OVER", "1.0", gives(mthFLOOVE, 5))
	add(familyExp, complexToComplex, "NEAR", "1.0", nothing)
	add(familySin, complexToComplex, "1.0", "OVER", gives(mthFLOOVE, 5))
	add(familySin, complexToComplex, "1.0", "-OVER", gives(mthFLOOVE, 5))
	add(familySin, complexToComplex, "1.0", "NEAR", nothing)
	add(familyCos, complexToComplex, "1.0", "OVER", gives(mthFLOOVE, 5))
	add(familyCos, complexToComplex, "1.0", "-OVER", gives(mthFLOOVE, 5))
	add(familyCos, complexToComplex, "1.0", "-NEAR", nothing)
	add(familyLog, complexToComplex, "0.0", "0.0", gives(mthINVARG, 5))
	add(familyLog, complexToComplex, "HUGE", "HUGE", func(c *complexCase) {
		if c.f == "F" {
			nothing(c)
		} else {
			gives(ssFLTOVF, 3)(c)
		}
	})
	add(familySqrt, complexToComplex, "BIG", "0.0", gives(ssFLTOVF, 3))
	add(familySqrt, complexToComplex, "HUGE", "HUGE", gives(ssFLTOVF, 3))
	add(familySqrt, complexToComplex, "LOW", "LOW", nothing)

	// An exponential too small for the format, and the same with underflow enabled:
	// no page lists MTH$_FLOUNDMAT.
	zero := func(fu bool) func(*complexCase) {
		return func(c *complexCase) { c.want, c.fu = [2]string{"0.0", "0.0"}, fu }
	}

	add(familyExp, complexToComplex, "-30000.0", "1.0", zero(false))
	add(familyExp, complexToComplex, "-30000.0", "1.0", zero(true))

	// A reserved operand in each part of what each routine reads.
	reserved := func(c *complexCase) {
		c.want, c.condition, c.count = unread(c.f), ssROPRAND, 3
		if c.kind == complexToReal {
			c.want = [2]string{"", "*"}
		}
	}

	for _, family := range []struct {
		names string
		kind  complexKind
	}{
		{familyAbs, complexToReal}, {familyExp, complexToComplex}, {familyLog, complexToComplex},
		{familySin, complexToComplex}, {familyCos, complexToComplex}, {familySqrt, complexToComplex},
		{familyMake, partsToComplex}, {familyConj, complexToComplex},
	} {
		add(family.names, family.kind, "", "1.0", reserved)
		add(family.names, family.kind, "1.0", "", reserved)
	}

	// MTH$REAL and MTH$AIMAG read only the part they return.
	add(familyReal, complexToReal, "", "1.0", reserved)
	add(familyReal, complexToReal, "1.5", "", answer("1.5", ""))
	add(familyImag, complexToReal, "1.0", "", reserved)
	add(familyImag, complexToReal, "", "1.5", answer("1.5", ""))

	// A part with the bits ^X8070 is a reserved operand in D, its exponent being clear,
	// and an ordinary value in G, whose exponent is 3 bits longer. These routines pass
	// such a value on unchanged, so only this tells which format one reads.
	for _, family := range []struct {
		d, g string
		kind complexKind
		re   string
		im   string
	}{
		{"MTH$DREAL", "MTH$GREAL", complexToReal, "^X8070", "1.0"},
		{"MTH$DIMAG", "MTH$GIMAG", complexToReal, "1.0", "^X8070"},
		{"MTH$DCMPLX", "MTH$GCMPLX", partsToComplex, "1.0", "^X8070"},
		{"MTH$DCONJG", "MTH$GCONJG", complexToComplex, "^X8070", "1.0"},
	} {
		cases = append(cases, complexCase{
			entry: family.d, f: "D", kind: family.kind, re: family.re, im: family.im,
			want: [2]string{"*", "*"}, condition: ssROPRAND, count: 3,
		}, complexCase{
			entry: family.g, f: "G", kind: family.kind, re: family.re, im: family.im,
			want: [2]string{"*", "*"},
		})
	}

	// Addresses it can't use: the argument's in every routine, and a result's in the
	// D and G forms, whether the routine has a result or a condition to leave there.
	bad := func(c *complexCase) {
		c.badIn, c.want, c.condition, c.count = true, [2]string{"*", "*"}, ssACCVIO, 5
	}

	badResult := func(c *complexCase) {
		c.want = [2]string{"*", "*"}

		if c.f != "F" {
			c.badOut, c.condition, c.count = true, ssACCVIO, 5
		}
	}

	for _, family := range []struct {
		names string
		kind  complexKind
	}{
		{familyAbs, complexToReal}, {familyExp, complexToComplex}, {familyLog, complexToComplex},
		{familySin, complexToComplex}, {familyCos, complexToComplex}, {familySqrt, complexToComplex},
		{familyMake, partsToComplex}, {familyConj, complexToComplex}, {familyReal, complexToReal},
		{familyImag, complexToReal},
	} {
		add(family.names, family.kind, "1.0", "1.0", bad)

		if family.kind != complexToReal {
			add(family.names, family.kind, "1.0", "1.0", badResult)
		}
	}

	add(familyExp, complexToComplex, "OVER", "1.0", func(c *complexCase) {
		gives(mthFLOOVE, 5)(c)
		c.want = [2]string{"*", "*"}

		if c.f != "F" {
			c.badOut, c.condition = true, ssACCVIO
		}
	})

	return cases
}

// complexProgram is the MACRO for the cases. A result in memory starts as words a
// routine wouldn't write, and goes in R0 to R3 for SHOW.
func complexProgram(cases []complexCase) string {
	var data, code strings.Builder

	const unreachable = "\tPUSHL\t#^X3FFF0000\n"

	for i, c := range cases {
		label := fmt.Sprintf("Z%d", i)
		size := map[table1Type]int{"F": 4, "D": 8, "G": 8}[c.f]

		fmt.Fprintf(&data, "%s:\n", label)

		for j, part := range []string{c.re, c.im} {
			// Parts given one by one aren't a complex number in memory: a word between
			// them shows a routine that thinks they are.
			if j == 1 && c.kind == partsToComplex {
				data.WriteString("\t.LONG\t^X8000\n")
			}

			switch {
			case part == "":
				part = "^X8000"

				fallthrough
			case strings.HasPrefix(part, "^X"):
				fmt.Fprintf(&data, "\t.LONG\t%s\n", part)

				if size == 8 {
					data.WriteString("\t.LONG\t0\n")
				}
			default:
				fmt.Fprintf(&data, "\t%s\t%s\n", c.f.directive(), part)
			}
		}

		fmt.Fprintf(&code, "\t; %d. %s\n\tCLRL\tSCOND\n\tCLRL\tSCOUNT\n\tMOVL\t#^X12345678,R1\n", i, c.entry)

		if c.fu {
			code.WriteString("\tBISPSW\t#^X40\n")
		}

		count := 1

		switch {
		case c.badIn && c.kind == partsToComplex:
			code.WriteString(unreachable + unreachable)

			count = 2
		case c.badIn:
			code.WriteString(unreachable)
		case c.kind == partsToComplex:
			fmt.Fprintf(&code, "\tPUSHAL\t%s+%d\n\tPUSHAL\t%s\n", label, size+4, label)

			count = 2
		default:
			fmt.Fprintf(&code, "\tPUSHAL\t%s\n", label)
		}

		inMemory := c.f != "F" && c.kind != complexToReal
		result := "HRES"

		if inMemory {
			switch {
			case c.badOut:
				code.WriteString(unreachable)
			case c.inPlace:
				fmt.Fprintf(&code, "\tPUSHAL\t%s\n", label)

				result = label
			default:
				code.WriteString("\tMOVL\t#^XDEADBEEF,HRES\n\tMOVL\t#^XDEADBEEF,HRES+4\n")
				code.WriteString("\tMOVL\t#^XDEADBEEF,HRES+8\n\tMOVL\t#^XDEADBEEF,HRES+12\n\tPUSHAL\tHRES\n")
			}

			count++
		}

		fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n", count, c.entry)

		if c.fu {
			code.WriteString("\tBICPSW\t#^X40\n")
		}

		if inMemory {
			fmt.Fprintf(&code, "\tMOVQ\t%s,R0\n\tMOVQ\t%s+8,R2\n", result, result)
		}

		code.WriteString("\tJSB\tSHOW\n")
	}

	return showProgram("CPLX", data.String(), code.String())
}

// complexWant is what a case should leave in R0 and up, as text: the longwords of
// each part it looks at, "*" for those it doesn't.
func complexWant(t *testing.T, c complexCase) []string {
	t.Helper()

	var words []string

	parts := c.want[:]
	if c.kind == complexToReal {
		parts = c.want[:1]
	}

	for _, part := range parts {
		if part == "*" {
			words = append(words, "*")

			if c.f != "F" {
				words = append(words, "*")
			}

			continue
		}

		words = append(words, wantRegisters(t, table1Case{entry: c.entry, out: c.f, want: part})...)
	}

	return words
}

func TestComplexFromAProgram(t *testing.T) {
	_, d, out := boot(t)
	cases := complexCases()

	build(t, d, out, "CPLX", complexProgram(cases))

	if err := d.Dispatch("RUN CPLX"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(cases), out)
	}

	for i, c := range cases {
		fields := strings.Fields(lines[i])
		name := fmt.Sprintf("%d. %s(%q, %q)", i, c.entry, c.re, c.im)

		for j, want := range complexWant(t, c) {
			if want != "*" && fields[j] != want {
				t.Errorf("%s: longword %d is %s, want %s", name, j, fields[j], want)
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

// TestComplexCovers: the program calls every entry of the survey's two families.
func TestComplexCovers(t *testing.T) {
	called := map[string]bool{}

	for _, c := range complexCases() {
		called[c.entry] = true
	}

	for _, family := range []string{
		familyAbs, familyExp, familyLog, familySin, familyCos, familySqrt,
		familyMake, familyConj, familyReal, familyImag,
	} {
		for _, name := range strings.Fields(family) {
			if !called["MTH$"+name] {
				t.Errorf("no case calls MTH$%s", name)
			}
		}
	}
}

// TestManualsComplexExample is section 1.7's COMPLEX_EX1 and COMPLEX_EX2, with the
// registers the manual prints: MTH$CLOG of (2.0, 1.0) through CALLG and CALLS.
func TestManualsComplexExample(t *testing.T) {
	_, d, out := boot(t)

	build(t, d, out, "EX1", showProgram("EX1", `ARGS:	.LONG	1
	.ADDRESS REAL
REAL:	.FLOAT	2
IMAG:	.FLOAT	1
`, `	CALLG	ARGS,G^MTH$CLOG
	JSB	SHOW
	MOVAL	REAL,-(SP)
	CALLS	#1,G^MTH$CLOG
	JSB	SHOW
`))

	if err := d.Dispatch("RUN EX1"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("RUN said:\n%s", out)
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "0210404E 63383FED ") {
			t.Errorf("R0 and R1 are not the manual's 0210404E and 63383FED: %s", line)
		}
	}
}

// TestComplexUnhandled: with no handler, the message and the image's end.
func TestComplexUnhandled(t *testing.T) {
	for _, c := range []struct {
		name, data, call, message string
	}{
		{"CEXP", "Z:\t.FLOAT\t100.0,1.0", "PUSHAL\tZ\n\tCALLS\t#1,G^MTH$CEXP", "%MTH-F-FLOOVEMAT, "},
		{"CDLOG", "R:\t.BLKD\t2\nZ:\t.D_FLOATING\t0.0,0.0", "PUSHAL\tZ\n\tPUSHAL\tR\n\tCALLS\t#2,G^MTH$CDLOG", "%MTH-F-INVARGMAT, "},
		{"CSQRT", "Z:\t.FLOAT\t1.0E38,0.0", "PUSHAL\tZ\n\tCALLS\t#1,G^MTH$CSQRT", "%SYSTEM-F-FLTOVF_F, "},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, d, out := boot(t)

			build(t, d, out, "CPLXEND", `	.TITLE	CPLXEND
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

			_ = d.Dispatch("RUN CPLXEND") // the image's end is what's looked at

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

