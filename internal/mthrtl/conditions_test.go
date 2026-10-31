package mthrtl_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/vmsdef"
)

// The conditions each of the manual's routine pages lists, page by page, from a
// real program: a check that every condition a page names is one its routine
// signals, and the signal array is the right size. This is one call per name and
// condition, for the list's sake. The other tests' cases hold the limits, the values
// a handler continues with, and the JSB entries.

var (
	mthLOGZER = vmsdef.LibrarySymbols["MTH$_LOGZERNEG"]
	mthSQUROO = vmsdef.LibrarySymbols["MTH$_SQUROONEG"]
)

// pageFamily is a routine page's entries, in the order of table1Formats. The
// complex routines have F, D, and G only.
type pageFamily string

// pageRow is a condition one or more pages list: the families, an argument list
// spelled as table1Cases spells them, and whether the caller has FU set.
type pageRow struct {
	families  []pageFamily
	args      []string
	condition uint32
	count     int
	fu        bool
}

// Every family's entries. "H" says the D and G forms (or the H form) take their
// result through a first argument, and "P" that the arguments are a pair's: an
// angle, and two addresses for the results. For these, table1Case's out is H, or F.
const (
	famAcos   pageFamily = "MTH$ACOS MTH$DACOS MTH$GACOS MTH$HACOS"
	famAcosd  pageFamily = "MTH$ACOSD MTH$DACOSD MTH$GACOSD MTH$HACOSD"
	famAsin   pageFamily = "MTH$ASIN MTH$DASIN MTH$GASIN MTH$HASIN"
	famAsind  pageFamily = "MTH$ASIND MTH$DASIND MTH$GASIND MTH$HASIND"
	famAtan   pageFamily = "MTH$ATAN MTH$DATAN MTH$GATAN MTH$HATAN"
	famAtand  pageFamily = "MTH$ATAND MTH$DATAND MTH$GATAND MTH$HATAND"
	famAtan2  pageFamily = "MTH$ATAN2 MTH$DATAN2 MTH$GATAN2 MTH$HATAN2"
	famAtand2 pageFamily = "MTH$ATAND2 MTH$DATAND2 MTH$GATAND2 MTH$HATAND2"
	famAtanh  pageFamily = "MTH$ATANH MTH$DATANH MTH$GATANH MTH$HATANH"
	famCos    pageFamily = "MTH$COS MTH$DCOS MTH$GCOS MTH$HCOS"
	famCosd   pageFamily = "MTH$COSD MTH$DCOSD MTH$GCOSD MTH$HCOSD"
	famCosh   pageFamily = "MTH$COSH MTH$DCOSH MTH$GCOSH MTH$HCOSH"
	famExp    pageFamily = "MTH$EXP MTH$DEXP MTH$GEXP MTH$HEXP"
	famLog    pageFamily = "MTH$ALOG MTH$DLOG MTH$GLOG MTH$HLOG"
	famLog2   pageFamily = "MTH$ALOG2 MTH$DLOG2 MTH$GLOG2 MTH$HLOG2"
	famLog10  pageFamily = "MTH$ALOG10 MTH$DLOG10 MTH$GLOG10 MTH$HLOG10"
	famSin    pageFamily = "MTH$SIN MTH$DSIN MTH$GSIN MTH$HSIN"
	famSind   pageFamily = "MTH$SIND MTH$DSIND MTH$GSIND MTH$HSIND"
	famSinh   pageFamily = "MTH$SINH MTH$DSINH MTH$GSINH MTH$HSINH"
	famSqrt   pageFamily = "MTH$SQRT MTH$DSQRT MTH$GSQRT MTH$HSQRT"
	famTan    pageFamily = "MTH$TAN MTH$DTAN MTH$GTAN MTH$HTAN"
	famTand   pageFamily = "MTH$TAND MTH$DTAND MTH$GTAND MTH$HTAND"
	famTanh   pageFamily = "MTH$TANH MTH$DTANH MTH$GTANH MTH$HTANH"
	famCabs   pageFamily = "MTH$CABS MTH$CDABS MTH$CGABS"
	famCcos   pageFamily = "MTH$CCOS MTH$CDCOS MTH$CGCOS"
	famCsin   pageFamily = "MTH$CSIN MTH$CDSIN MTH$CGSIN"
	famCexp   pageFamily = "MTH$CEXP MTH$CDEXP MTH$CGEXP"
	famClog   pageFamily = "MTH$CLOG MTH$CDLOG MTH$CGLOG"
	famCsqrt  pageFamily = "MTH$CSQRT MTH$CDSQRT MTH$CGSQRT"
	famConjg  pageFamily = "MTH$CONJG MTH$DCONJG MTH$GCONJG"
	famCmplx  pageFamily = "MTH$CMPLX MTH$DCMPLX MTH$GCMPLX"
	famReal   pageFamily = "MTH$REAL MTH$DREAL MTH$GREAL"
	famImag   pageFamily = "MTH$AIMAG MTH$DIMAG MTH$GIMAG"
	famSincos pageFamily = "MTH$SINCOS MTH$DSINCOS MTH$GSINCOS MTH$HSINCOS"
	famSincod pageFamily = "MTH$SINCOSD MTH$DSINCOSD MTH$GSINCOSD MTH$HSINCOSD"
)

// complexResult are the families whose D and G forms have a result too long for
// R0:R1.
var complexResult = []pageFamily{famCcos, famCsin, famCexp, famClog, famCsqrt, famConjg, famCmplx}

// pageRows is the manual's lists. A routine's reserved operand is in every list.
// MTH$HTAN's overflow is the one condition left out: the phase doc's subtask 4 found
// no argument in any format reaches it, a question for the probe. So are the pages'
// conditions in Table 1-1's routines and MTH$CVT_x_x, which TestTable1FromAProgram
// holds by name.
func pageRows() []pageRow {
	rop := func(families ...pageFamily) pageRow {
		return pageRow{families: families, args: []string{""}, condition: ssROPRAND, count: 3}
	}

	const severe = 5 // the condition, its one argument, the user PC, and PC and PSL

	return []pageRow{
		rop(famAcos, famAcosd, famAsin, famAsind, famAtan, famAtand, famAtanh, famCos, famCosd, famCosh,
			famExp, famLog, famLog2, famLog10, famSin, famSind, famSinh, famSqrt, famTan, famTand, famTanh,
			famCabs, famCcos, famCsin, famCexp, famClog, famCsqrt, famConjg, famReal),
		{families: []pageFamily{famAtan2, famAtand2}, args: []string{"", "1.0"}, condition: ssROPRAND, count: 3},
		{families: []pageFamily{famAtan2, famAtand2}, args: []string{"1.0", ""}, condition: ssROPRAND, count: 3},
		{families: []pageFamily{famCmplx}, args: []string{"", "1.0"}, condition: ssROPRAND, count: 3},
		{families: []pageFamily{famCmplx}, args: []string{"1.0", ""}, condition: ssROPRAND, count: 3},
		{families: []pageFamily{famSincos, famSincod}, args: []string{"", "0.0", "0.0"}, condition: ssROPRAND, count: 3},

		{families: []pageFamily{famAcos, famAcosd, famAsin, famAsind}, args: []string{"2.0"}, condition: mthINVARG, count: severe},
		{families: []pageFamily{famAtan2, famAtand2}, args: []string{"0.0", "0.0"}, condition: mthINVARG, count: severe},
		{families: []pageFamily{famAtanh}, args: []string{"1.0"}, condition: mthINVARG, count: severe},
		{families: []pageFamily{famClog}, args: []string{"0.0,0.0"}, condition: mthINVARG, count: severe},

		{families: []pageFamily{famLog, famLog2, famLog10}, args: []string{"0.0"}, condition: mthLOGZER, count: severe},
		{families: []pageFamily{famLog, famLog2, famLog10}, args: []string{"-1.0"}, condition: mthLOGZER, count: severe},
		{families: []pageFamily{famSqrt}, args: []string{"-1.0"}, condition: mthSQUROO, count: severe},

		{families: []pageFamily{famCosh, famSinh, famExp}, args: []string{"HUGE"}, condition: mthFLOOVE, count: severe},
		{families: []pageFamily{famTand}, args: []string{"90.0"}, condition: mthFLOOVE, count: severe},
		{families: []pageFamily{famCabs}, args: []string{"HUGE,HUGE"}, condition: mthFLOOVE, count: severe},
		{families: []pageFamily{famCexp}, args: []string{"HUGE,0.0"}, condition: mthFLOOVE, count: severe},
		{families: []pageFamily{famCcos, famCsin}, args: []string{"0.0,HUGE"}, condition: mthFLOOVE, count: severe},

		{families: []pageFamily{famExp}, args: []string{"-HUGE"}, condition: mthFLOUND, count: severe, fu: true},
		{families: []pageFamily{famSind, famTand}, args: []string{"TINY"}, condition: mthFLOUND, count: severe, fu: true},
		{families: []pageFamily{famSincod}, args: []string{"TINY", "0.0", "0.0"}, condition: mthFLOUND, count: severe, fu: true},

		{families: []pageFamily{"MTH$CDLOG MTH$CGLOG"}, args: []string{"HUGE,HUGE"}, condition: ssFLTOVF, count: 3},
		{families: []pageFamily{famCsqrt}, args: []string{"HUGE,0.0"}, condition: ssFLTOVF, count: 3},
	}
}

// pageCases makes one call per name in each row, and those for the imaginary part,
// which reads what the others don't: a reserved operand in the second part.
func pageCases() []table1Case {
	var cases []table1Case

	for _, row := range pageRows() {
		for _, family := range row.families {
			names := strings.Fields(string(family))
			offset := 0

			if len(names) == 2 { // MTH$CDLOG and MTH$CGLOG alone: F has no such condition
				offset = 1
			}

			for i, name := range names {
				f := table1Formats[i+offset]
				out := f.t

				if slices.Contains(complexResult, family) || family == "MTH$CDLOG MTH$CGLOG" {
					if f.t != "F" {
						out = "H"
					}
				}

				args := make([]string, len(row.args))
				for j, a := range row.args {
					args[j] = spell(a, f.huge, f.tiny)
				}

				if family == famSincos || family == famSincod {
					out = "F" // through the second and third arguments
				}

				cases = append(cases, table1Case{
					entry: name, in: f.t, out: out, args: args, want: "*",
					condition: row.condition, count: row.count, fu: row.fu,
				})
			}
		}
	}

	return append(cases,
		table1Case{entry: "MTH$AIMAG", in: "F", out: "F", args: []string{"^X4080,^X8000"}, want: "*", condition: ssROPRAND, count: 3},
		table1Case{entry: "MTH$DIMAG", in: "D", out: "D", args: []string{"^X4080,0,^X8000"}, want: "*", condition: ssROPRAND, count: 3},
		table1Case{entry: "MTH$GIMAG", in: "G", out: "G", args: []string{"^X4010,0,^X8000"}, want: "*", condition: ssROPRAND, count: 3},
	)
}

// TestPagesConditions runs them.
func TestPagesConditions(t *testing.T) {
	_, d, out := boot(t)
	cases := pageCases()

	build(t, d, out, "PAGES", table1Program(cases))

	if err := d.Dispatch("RUN PAGES"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(cases), out)
	}

	for i, c := range cases {
		fields := strings.Fields(lines[i])
		name := fmt.Sprintf("%d. %s%v", i, c.entry, c.args)

		if got, want := fields[4], fmt.Sprintf("c=%08X", c.condition); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}

		if got, want := fields[5], fmt.Sprintf("n=%d", c.count); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
}

// TestConditionMessages: each of the library's conditions a routine signals, with
// no handler, is the message file's text, with the user PC on its second line, and
// ends the image. Its severity is F whatever the message file's value says of it, as
// all of them are severe.
func TestConditionMessages(t *testing.T) {
	for _, c := range []struct {
		ident string
		data  string
		call  string
	}{
		{"WRONUMARG", "", "CALLS\t#0,G^MTH$AMAX1"},
		{"INVARGMAT", "X:\t.FLOAT\t2.0", "PUSHAL\tX\n\tCALLS\t#1,G^MTH$ASIN"},
		{"UNDEXP", "A:\t.FLOAT\t0.0,-1.0", "PUSHL\tA+4\n\tPUSHL\tA\n\tCALLS\t#2,G^OTS$POWRR"},
		{"LOGZERNEG", "X:\t.FLOAT\t0.0", "PUSHAL\tX\n\tCALLS\t#1,G^MTH$ALOG"},
		{"SQUROONEG", "X:\t.FLOAT\t-1.0", "PUSHAL\tX\n\tCALLS\t#1,G^MTH$SQRT"},
		{"FLOOVEMAT", "X:\t.FLOAT\t1.0E30", "PUSHAL\tX\n\tCALLS\t#1,G^MTH$EXP"},
		{"FLOUNDMAT", "X:\t.FLOAT\t-1.0E30", "BISPSW\t#^X40\n\tPUSHAL\tX\n\tCALLS\t#1,G^MTH$EXP"},
	} {
		t.Run(c.ident, func(t *testing.T) {
			_, d, out := boot(t)

			build(t, d, out, "MTHMSG", `	.TITLE	MTHMSG
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

			_ = d.Dispatch("RUN MTHMSG") // the image's end is what's looked at

			text := out.String()
			value := vmsdef.LibrarySymbols["MTH$_"+c.ident]
			message, ok := vmsdef.Messages[value&^7]

			if !ok {
				t.Fatalf("no message for %#x", value)
			}

			first, _, _ := strings.Cut(message.Text, "!/")

			if want := "%MTH-F-" + c.ident + ", " + first; !strings.Contains(text, want) {
				t.Errorf("no %q; RUN said:\n%s", want, text)
			}

			if !strings.Contains(text, "user PC ") {
				t.Errorf("no user PC; RUN said:\n%s", text)
			}

			if strings.Contains(text, "WENT ON") {
				t.Errorf("the image went on after a severe condition:\n%s", text)
			}
		})
	}
}

// TestRandomFromAProgram runs MTH$RANDOM: a sequence through one seed, the two ends
// of its range, and arguments it can't reach. R1 shows the seed left in memory, and
// R2 the read-only longword.
func TestRandomFromAProgram(t *testing.T) {
	_, d, out := boot(t)

	var code strings.Builder

	call := func(setup, push string) {
		code.WriteString("\tCLRL\tSCOND\n\tCLRL\tSCOUNT\n" + setup + push)
		code.WriteString("\tMOVL\tSEED,R1\n\tMOVL\tFIXED,R2\n\tCLRL\tR3\n\tJSB\tSHOW\n")
	}

	normal := "\tPUSHAL\tSEED\n\tCALLS\t#1,G^MTH$RANDOM\n"

	call("\tMOVL\t#1,SEED\n", normal) // the seed goes on from each to the next
	call("", normal)
	call("", normal)
	call("\tMOVL\t#^XFFFFFFFF,SEED\n", normal) // ^XFFFEF234 next
	call("\tCLRL\tSEED\n", normal)             // 1 next, its high bits 0
	call("\tMOVL\t#^X777653FB,SEED\n", normal) // ^XFFFFFF00 next, the top of the range
	call("\tMOVL\t#7,SEED\n", "\tCALLS\t#0,G^MTH$RANDOM\n")
	call("\tMOVL\t#7,SEED\n", "\tPUSHL\t#^X3FFF0000\n\tCALLS\t#1,G^MTH$RANDOM\n")
	call("\tMOVL\t#7,SEED\n", "\tPUSHAL\tFIXED\n\tCALLS\t#1,G^MTH$RANDOM\n")

	source := strings.Replace(showProgram("RANDOM", "SEED:\t.BLKL\t1\n", code.String()),
		"\t.ENTRY\tHANDLER", "FIXED:\t.LONG\t1\n\t.ENTRY\tHANDLER", 1)

	// FIXED, in the code psect, can be read and not written.
	build(t, d, out, "RANDOM", source)

	if err := d.Dispatch("RUN RANDOM"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	const accvio = 5 // the condition, the reason mask, the address, and PC and PSL

	// An F value's words are swapped in a longword: the sign, exponent, and high
	// fraction are the low-order word. These were worked out apart from the code.
	// For the first, 269/2^24: 269 is 9 bits, so the value is .100001101 times
	// 2^-15, its exponent field is 128-15 = ^X71 (^X3880 in place), and past the
	// hidden bit its fraction is 0000110 in the first word, then a 1 at the top of
	// the second.
	want := []struct{ r0, r1 string }{
		{"80003886", "00010DCE"},
		{"CC183EE2", "1C5983F7"},
		{"59374043", "C35937CC"},
		{"FEF2407F", "FFFEF234"},
		{"00000000", "00000001"},
		{"FFFF407F", "FFFFFF00"},
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 9 {
		t.Fatalf("%d lines, want 9:\n%s", len(lines), out)
	}

	for i, w := range want {
		fields := strings.Fields(lines[i])
		if fields[0] != w.r0 || fields[1] != w.r1 || fields[4] != "c=00000000" {
			t.Errorf("call %d: %s; want R0 %s, seed %s, no condition", i, lines[i], w.r0, w.r1)
		}
	}

	for i, line := range lines[6:] {
		fields := strings.Fields(line)

		if got, want := fields[4], fmt.Sprintf("c=%08X", ssACCVIO); got != want {
			t.Errorf("failing call %d: %s, want %s", i, line, want)
		}

		if got, want := fields[5], fmt.Sprintf("n=%d", accvio); got != want {
			t.Errorf("failing call %d: %s, want %s", i, line, want)
		}

		if fields[1] != "00000007" {
			t.Errorf("failing call %d changed the seed: %s", i, line)
		}
	}

	if fields := strings.Fields(lines[8]); fields[2] != "00000001" {
		t.Errorf("the read-only seed: %s, want it left at 1", lines[8])
	}
}

// TestRandomSettingFromAProgram runs a program twice with the setting on, each setting
// its seed to the same value, and once more with it off: the last is the formula's,
// the first two have nothing to do with it or each other. A seed it can't write is an
// access violation either way.
func TestRandomSettingFromAProgram(t *testing.T) {
	const key = "vax.runtime.random"

	old, had := settings.Get(key), settings.Exists(key)

	t.Cleanup(func() {
		if had {
			settings.Set(key, old)
		} else {
			_ = settings.Delete(key)
		}
	})

	_, d, out := boot(t)

	code := "\tMOVL\t#1,SEED\n\tPUSHAL\tSEED\n\tCALLS\t#1,G^MTH$RANDOM\n\tMOVL\tSEED,R1\n\tCLRQ\tR2\n\tJSB\tSHOW\n" +
		"\tCLRL\tSCOND\n\tPUSHAL\tFIXED\n\tCALLS\t#1,G^MTH$RANDOM\n\tJSB\tSHOW\n"
	source := strings.Replace(showProgram("RANDSET", "SEED:\t.BLKL\t1\n", code),
		"\t.ENTRY\tHANDLER", "FIXED:\t.LONG\t1\n\t.ENTRY\tHANDLER", 1)

	build(t, d, out, "RANDSET", source)

	run := func(value string) []string {
		t.Helper()
		settings.Set(key, value)
		out.Reset()

		if err := d.Dispatch("RUN RANDSET"); err != nil {
			t.Fatalf("RUN: %v\n%s", err, out)
		}

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("%d lines, want 2:\n%s", len(lines), out)
		}

		if got, want := strings.Fields(lines[1])[4], fmt.Sprintf("c=%08X", ssACCVIO); got != want {
			t.Errorf("the seed it can't write, with %s: %s, want %s", value, lines[1], want)
		}

		return strings.Fields(lines[0])
	}

	first, second := run("true"), run("true")
	formula := run("false")

	if formula[0] != "80003886" || formula[1] != "00010DCE" {
		t.Errorf("off: R0 %s, seed %s; want the formula's", formula[0], formula[1])
	}

	if first[1] == second[1] || first[1] == formula[1] || second[1] == formula[1] {
		t.Errorf("on: seeds %s and %s from one starting seed; the formula gives %s", first[1], second[1], formula[1])
	}
}

// TestUnwindFromJSB: a handler that unwinds from a condition a JSB entry signaled
// removes the frame the handler is in and no other, as it does from a CALL entry. The
// program's outer routine goes on and prints; removing one frame too many would end it
// at START's caller.
func TestUnwindFromJSB(t *testing.T) {
	_, d, out := boot(t)

	build(t, d, out, "UNWJSB", `	.TITLE	UNWJSB
	$SSDEF
	.PSECT	DATA,WRT,NOEXE,LONG
BACK:	.ASCID	/BACK IN START/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	CMPL	4(R0),#SS$_UNWIND
	BEQL	1$
	$UNWIND_S
1$:	RET
	.ENTRY	INNER,^M<>
	MOVAB	HANDLER,(FP)
	MOVL	#^X8000,R0	; a reserved operand
	JSB	G^MTH$SQRT_R3
	RET
	.ENTRY	START,^M<>
	CALLS	#0,INNER
	PUSHAQ	BACK
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
	.END	START
`)

	if err := d.Dispatch("RUN UNWJSB"); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out)
	}

	if !strings.Contains(out.String(), "BACK IN START") {
		t.Errorf("START didn't go on; RUN said:\n%s", out)
	}
}
