//go:build ignore

// gen.go writes the Phase 77 probe of MTHRTL (docs/PHASE-77 - MTHRTL.md,
// subtask 10): mth77.mar, a MACRO program that calls the library's routines case
// by case and prints what each returned and signaled; mth77.com, which builds and
// runs it on VMS; and exchange.cmd, the govax console script that makes the
// exchange volume. Run it from the repository root:
//
//	go run testdata/mth77/gen.go
//
// TestMTH77Oracle (internal/console/mth77_test.go) compares govax's run with VMS's,
// vax/mth77-vax.txt.
//
// The program is a table of cases and one driver. A case is a routine of its own,
// under a condition handler (HANDLER) that records the first signal array and the
// mechanism array's R0 and R1. What the handler does next matters on VMS: returning
// SS$_CONTINUE from a fault runs the faulting instruction again, and would loop for
// ever. So it continues only from the library's own conditions (facility MTH) and
// the arithmetic traps, whose PC is past the instruction, and it unwinds to the
// driver from everything else, and from any second signal in a case.
//
// A case's line is its title, then
//
//	R=...       the registers the case shows, if the call came back
//	M=...       the longwords of the result area, RES, preset to ^X55555555
//	c=x n=x     the condition, and the signal array's first longword
//	a=x         each signal argument, "ret" if it's the call's return address
//	pc=ret/oth  the signal array's PC: the return address, or another
//	mr=x x      the mechanism array's R0 and R1, as the handler found them
//	h=x         the number of signals, if more than one
//	unw         the handler unwound
//
// A title starting "~" is a line of the sweep at the end: arguments drawn from a
// fixed random sequence, shown in hex, whose results the oracle compares to within
// a count of last bits, rather than exactly. The letter after it is the format.
package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/vaxfloat"
)

const dir = "testdata/mth77"

// How a case reaches its routine.
const (
	byRef = iota // CALLS, arguments by reference
	byVal        // CALLS, arguments by value
	byJSB        // JSB, arguments in R0 up
)

// Handler modes.
const (
	modeContinue = iota // continue from MTH$ conditions and arithmetic traps
	modeUnwind          // unwind from everything
	modeCorrect         // as modeContinue, but store a corrected R0 and R1 first
)

// arg is one argument: words stored in a cell, whose address or value is passed.
type arg struct {
	text  string
	words []uint32
	bad   bool // an address no program can read
	res   int  // with isRes: the result area, at this offset
	isRes bool
	ro    bool // in a psect the program can't write
}

type kase struct {
	title  string
	entry  string
	how    int
	args   []arg
	regs   int
	mem    int
	mode   int
	fu     bool
	marks  bool // preset R1 to R11 with markers
	post   string
	header bool
}

var formats = map[string]vaxfloat.Format{"F": vaxfloat.F, "D": vaxfloat.D, "G": vaxfloat.G, "H": vaxfloat.H}

func wordsOf(f vaxfloat.Format, b vaxfloat.Bits) []uint32 {
	w := []uint32{uint32(b.Lo), uint32(b.Lo >> 32), uint32(b.Hi), uint32(b.Hi >> 32)}

	return w[:f.Size()/4]
}

func packed(f vaxfloat.Format, text string) []uint32 {
	x, err := vaxfloat.Parse(text)
	if err != nil {
		log.Fatalf("%s: %v", text, err)
	}

	v, err := vaxfloat.Round(f, x)
	if err != nil {
		log.Fatalf("%s in %v: %v", text, f, err)
	}

	b, err := vaxfloat.Pack(f, v)
	if err != nil {
		log.Fatal(err)
	}

	return wordsOf(f, b)
}

// fl is a floating argument, in format letter l.
func fl(l, text string) arg { return arg{text: text, words: packed(formats[l], text)} }

// reserved is a reserved operand in format letter l.
func reserved(l string) arg {
	return arg{text: "RSV", words: wordsOf(formats[l], vaxfloat.Reserved(formats[l]))}
}

// cx is a complex argument, its two parts side by side.
func cx(l, r, i string) arg {
	return arg{text: "(" + r + "," + i + ")", words: append(packed(formats[l], r), packed(formats[l], i)...)}
}

// cxr is a complex argument with a reserved operand in one part.
func cxr(l string, realPart bool) arg {
	one, bad := packed(formats[l], "1.0"), wordsOf(formats[l], vaxfloat.Reserved(formats[l]))
	if realPart {
		return arg{text: "(RSV,1.0)", words: append(bad, one...)}
	}

	return arg{text: "(1.0,RSV)", words: append(one, bad...)}
}

func long(x int64) arg { return arg{text: fmt.Sprint(x), words: []uint32{uint32(x)}} }

// word is a word argument. Its high word holds junk, which a routine must not read.
func word(x int64) arg {
	return arg{text: fmt.Sprint(x), words: []uint32{uint32(x)&0xFFFF | 0xBEEF0000}}
}

func bits(words ...uint32) arg {
	return arg{text: fmt.Sprintf("^X%08X", words[0]), words: words}
}

func hex(l string, words []uint32) arg {
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = fmt.Sprintf("%08X", w)
	}

	return arg{text: strings.Join(parts, ""), words: words}
}

var (
	bad      = arg{text: "BAD", bad: true}
	res0     = arg{text: "RES", isRes: true}
	res16    = arg{text: "RES+16", isRes: true, res: 16}
	readOnly = arg{text: "RO", words: []uint32{1}, ro: true}
)

var cases []*kase

func add(c *kase) *kase {
	cases = append(cases, c)

	return c
}

func header(text string) { add(&kase{title: "=== " + text, header: true}) }

func (c *kase) withFU() *kase        { c.fu = true; return c }
func (c *kase) mode1(m int) *kase    { c.mode = m; return c }
func (c *kase) marked() *kase        { c.marks = true; return c }
func (c *kase) show(n int) *kase     { c.regs = n; return c }
func (c *kase) area(n int) *kase     { c.mem = n; return c }
func (c *kase) after(p string) *kase { c.post = p; return c }

// real is an MTH$ routine by reference with a result of format l: R0, R0:R1, or
// through a first argument for H.
func real(entry, l string, args ...arg) *kase {
	c := &kase{entry: entry, how: byRef, args: args}

	switch l {
	case "F":
		c.regs = 1
	case "D", "G":
		c.regs = 2
	case "H":
		c.args = append([]arg{res0}, args...)
		c.mem = 4
	}

	return add(c)
}

// complexResult is an MTH$ routine with a complex result: R0:R1 for F, through a
// first argument for D and G.
func complexResult(entry, l string, args ...arg) *kase {
	if l == "F" {
		return add(&kase{entry: entry, how: byRef, args: args, regs: 2})
	}

	return add(&kase{entry: entry, how: byRef, args: append([]arg{res0}, args...), mem: 4})
}

// integer is an MTH$ routine by reference with its result in R0.
func integer(entry string, args ...arg) *kase {
	return add(&kase{entry: entry, how: byRef, args: args, regs: 1})
}

// value is an OTS$ routine, by value, with its result in regs registers.
func value(entry string, regs int, args ...arg) *kase {
	return add(&kase{entry: entry, how: byVal, args: args, regs: regs, marks: true})
}

// jsb is a JSB entry, its arguments in R0 up, showing regs registers.
func jsb(entry string, regs int, args ...arg) *kase {
	return add(&kase{entry: entry, how: byJSB, args: args, regs: regs})
}

// forms calls fn with each of a family's names, and its format's letter.
func forms(names string, fn func(name, l string)) {
	fields := strings.Fields(names)
	letters := []string{"F", "D", "G", "H"}

	for i, name := range fields {
		fn(name, letters[i])
	}
}

// The largest and smallest arguments in each format's terms.
var huge = map[string]string{"F": "1.5E38", "D": "1.5E38", "G": "8.0E307", "H": "5.0E4931"}
var tiny = map[string]string{"F": "3.0E-39", "D": "3.0E-39", "G": "6.0E-309", "H": "9.0E-4933"}

func randomCases() {
	header("MTH$RANDOM")

	for _, seed := range []int64{0, 1, 2, 0x12345678, 0xFFFFFFFF, 0x777653FB, 0x80000000, 0xFFFFFF00} {
		c := add(&kase{entry: "MTH$RANDOM", how: byRef, args: []arg{long(seed)}, regs: 2, mem: 1, marks: true})
		c.post = "seed"
	}

	// A sequence from one seed, each case's seed the last one's by the manual's
	// formula, so the line's seed shows whether VMS's formula is that one.
	seed := uint32(1)
	for range 12 {
		c := add(&kase{entry: "MTH$RANDOM", how: byRef, args: []arg{long(int64(seed))}, regs: 1, mem: 1})
		c.post = "seed"
		seed = seed*69069 + 1
	}

	add(&kase{entry: "MTH$RANDOM", how: byRef, regs: 1, title: "MTH$RANDOM()"})
	add(&kase{entry: "MTH$RANDOM", how: byRef, args: []arg{bad}, regs: 1})
	add(&kase{entry: "MTH$RANDOM", how: byRef, args: []arg{readOnly}, regs: 1})
}

func realConditions() {
	header("square root, exponential, logarithm")

	forms("MTH$SQRT MTH$DSQRT MTH$GSQRT MTH$HSQRT", func(n, l string) {
		real(n, l, fl(l, "4.0"))
		real(n, l, fl(l, "-1.0"))
		real(n, l, reserved(l))
		real(n, l, bad)
	})

	forms("MTH$EXP MTH$DEXP MTH$GEXP MTH$HEXP", func(n, l string) {
		real(n, l, fl(l, "0.0"))
		real(n, l, fl(l, huge[l]))
		real(n, l, fl(l, "-"+huge[l]))
		real(n, l, fl(l, "-"+huge[l])).withFU()
		real(n, l, fl(l, huge[l])).mode1(modeCorrect)
	})

	// The limits: the manual's figures, and either side of the format's.
	for _, x := range []string{"88.02", "88.03", "88.72", "88.73", "-88.72", "-88.73"} {
		real("MTH$EXP", "F", fl("F", x)).withFU()
		real("MTH$DEXP", "D", fl("D", x)).withFU()
	}

	for _, x := range []string{"709.08", "709.09", "709.78", "709.79", "-709.77", "-709.78", "-709.79"} {
		real("MTH$GEXP", "G", fl("G", x)).withFU()
	}

	for _, x := range []string{"11355.82", "11355.84", "11356.52", "11356.53", "-11356.52", "-11356.53"} {
		real("MTH$HEXP", "H", fl("H", x)).withFU()
	}

	for _, fam := range []string{
		"MTH$ALOG MTH$DLOG MTH$GLOG MTH$HLOG", "MTH$ALOG2 MTH$DLOG2 MTH$GLOG2 MTH$HLOG2",
		"MTH$ALOG10 MTH$DLOG10 MTH$GLOG10 MTH$HLOG10",
	} {
		forms(fam, func(n, l string) {
			real(n, l, fl(l, "8.0"))
			real(n, l, fl(l, "0.0"))
			real(n, l, fl(l, "-1.0"))
		})
	}

	header("trigonometric functions")

	for _, fam := range []string{"MTH$SIN MTH$DSIN MTH$GSIN MTH$HSIN", "MTH$COS MTH$DCOS MTH$GCOS MTH$HCOS", "MTH$TAN MTH$DTAN MTH$GTAN MTH$HTAN"} {
		forms(fam, func(n, l string) {
			real(n, l, fl(l, "1.0"))
			real(n, l, fl(l, "1.0E6"))
			real(n, l, fl(l, huge[l])) // does a huge angle signal SIGLOSMAT?
			real(n, l, reserved(l))
		})
	}

	forms("MTH$TAN MTH$DTAN MTH$GTAN MTH$HTAN", func(n, l string) {
		real(n, l, fl(l, "1.5707963267948966192313216916397514421")) // the nearest to pi/2
	})

	for _, fam := range []string{"MTH$SIND MTH$DSIND MTH$GSIND MTH$HSIND", "MTH$TAND MTH$DTAND MTH$GTAND MTH$HTAND"} {
		forms(fam, func(n, l string) {
			real(n, l, fl(l, tiny[l]))
			real(n, l, fl(l, tiny[l])).withFU()
		})
	}

	for _, fam := range []string{"MTH$SIND MTH$DSIND MTH$GSIND MTH$HSIND", "MTH$COSD MTH$DCOSD MTH$GCOSD MTH$HCOSD", "MTH$TAND MTH$DTAND MTH$GTAND MTH$HTAND"} {
		forms(fam, func(n, l string) {
			for _, x := range []string{"30.0", "45.0", "90.0", "180.0", "270.0", "360.0", "450.0"} {
				real(n, l, fl(l, x))
			}
		})
	}

	forms("MTH$SINCOS MTH$DSINCOS MTH$GSINCOS MTH$HSINCOS", func(n, l string) {
		add(&kase{entry: n, how: byRef, args: []arg{fl(l, "1.0"), res0, res16}, mem: 8})
		add(&kase{entry: n, how: byRef, args: []arg{reserved(l), res0, res16}, mem: 8})
	})

	forms("MTH$SINCOSD MTH$DSINCOSD MTH$GSINCOSD MTH$HSINCOSD", func(n, l string) {
		add(&kase{entry: n, how: byRef, args: []arg{fl(l, "90.0"), res0, res16}, mem: 8})
		add(&kase{entry: n, how: byRef, args: []arg{fl(l, tiny[l]), res0, res16}, mem: 8})
		add(&kase{entry: n, how: byRef, args: []arg{fl(l, tiny[l]), res0, res16}, mem: 8}).withFU()
	})

	for _, fam := range []string{
		"MTH$ASIN MTH$DASIN MTH$GASIN MTH$HASIN", "MTH$ACOS MTH$DACOS MTH$GACOS MTH$HACOS",
		"MTH$ASIND MTH$DASIND MTH$GASIND MTH$HASIND", "MTH$ACOSD MTH$DACOSD MTH$GACOSD MTH$HACOSD",
	} {
		forms(fam, func(n, l string) {
			real(n, l, fl(l, "0.5"))
			real(n, l, fl(l, "1.0"))
			real(n, l, fl(l, "2.0"))
		})
	}

	header("ATAN2 and the hyperbolic functions")

	for _, fam := range []string{"MTH$ATAN2 MTH$DATAN2 MTH$GATAN2 MTH$HATAN2", "MTH$ATAND2 MTH$DATAND2 MTH$GATAND2 MTH$HATAND2"} {
		forms(fam, func(n, l string) {
			real(n, l, fl(l, "1.0"), fl(l, "2.0"))
			real(n, l, fl(l, "0.0"), fl(l, "0.0"))
			real(n, l, fl(l, "0.0"), fl(l, "-1.0"))
			real(n, l, fl(l, "-1.0"), fl(l, "0.0"))
			real(n, l, fl(l, tiny[l]), fl(l, huge[l]))
			real(n, l, fl(l, tiny[l]), fl(l, huge[l])).withFU()
		})
	}

	for _, fam := range []string{"MTH$SINH MTH$DSINH MTH$GSINH MTH$HSINH", "MTH$COSH MTH$DCOSH MTH$GCOSH MTH$HCOSH"} {
		forms(fam, func(n, l string) {
			real(n, l, fl(l, "1.0"))
			real(n, l, fl(l, huge[l]))
		})
	}

	forms("MTH$TANH MTH$DTANH MTH$GTANH MTH$HTANH", func(n, l string) {
		real(n, l, fl(l, "1.0"))
		real(n, l, fl(l, huge[l]))
	})

	forms("MTH$ATANH MTH$DATANH MTH$GATANH MTH$HATANH", func(n, l string) {
		real(n, l, fl(l, "0.5"))
		real(n, l, fl(l, "1.0"))
		real(n, l, fl(l, "-2.0"))
	})
}

func tableCases() {
	header("Table 1-1")

	forms("MTH$AMOD MTH$DMOD MTH$GMOD MTH$HMOD", func(n, l string) {
		real(n, l, fl(l, "7.5"), fl(l, "2.0"))
		real(n, l, fl(l, "-7.5"), fl(l, "2.0"))
		real(n, l, fl(l, "7.0"), fl(l, "0.0")).mode1(modeUnwind)
		real(n, l, fl(l, huge[l]), fl(l, tiny[l])).mode1(modeUnwind)
		real(n, l, fl(l, tiny[l]), fl(l, huge[l]))
		real(n, l, fl(l, tiny[l]), fl(l, huge[l])).withFU()
		real(n, l, fl(l, "1.0E30"), fl(l, "3.0")) // more bits of quotient than the format has
	})

	for _, c := range [][2]string{{"MTH$ANINT", "F"}, {"MTH$DNINT", "D"}, {"MTH$GNINT", "G"}, {"MTH$HNINT", "H"}} {
		real(c[0], c[1], fl(c[1], "2.5"))
		real(c[0], c[1], fl(c[1], "-2.5"))
		real(c[0], c[1], fl(c[1], "0.5"))
	}

	real("MTH$ANINT", "F", bits(0xFFFF3FFF)) // the value just below 0.5

	for _, name := range []string{"MTH$JNINT", "MTH$JIFIX"} {
		integer(name, fl("F", "1.0E30")).mode1(modeUnwind)
	}

	integer("MTH$ININT", fl("F", "40000.0")).mode1(modeUnwind)
	integer("MTH$IIFIX", fl("F", "40000.0")).mode1(modeUnwind)

	// Words and longwords: overflow, a zero divisor, and R0's high word.
	integer("MTH$JMOD", long(-2147483648), long(-1)).mode1(modeUnwind)
	integer("MTH$IMOD", word(-32768), word(-1)).mode1(modeUnwind)
	integer("MTH$JMOD", long(7), long(0)).mode1(modeUnwind)
	integer("MTH$IMOD", word(7), word(0)).mode1(modeUnwind)
	integer("MTH$JMOD", long(7), long(0))
	integer("MTH$IMOD", word(7), word(0))
	integer("MTH$IIABS", word(-32768))
	integer("MTH$JIABS", long(-2147483648))
	integer("MTH$IIABS", word(-5))
	integer("MTH$IISIGN", word(-32768), word(1))
	integer("MTH$JISIGN", long(-2147483648), long(1))
	integer("MTH$IIAND", word(0x0F0F), word(0x00FF))
	integer("MTH$IIOR", word(-1), word(0))
	integer("MTH$INOT", word(0))
	integer("MTH$IISHFT", word(1), word(15))
	integer("MTH$IMAX0", word(3), word(-4))
	integer("MTH$SGN", fl("F", "-2.0"))
	integer("MTH$SGN", fl("F", "0.0"))
	integer("MTH$UMAX", long(-1), long(1))
	integer("MTH$UMIN", long(-1), long(1))

	// With no arguments.
	for _, n := range []string{"MTH$AMAX1", "MTH$AMIN1", "MTH$IMAX0", "MTH$JMAX0", "MTH$UMAX"} {
		add(&kase{entry: n, how: byRef, regs: 1, title: n + "()"})
	}

	// Conversions, out of range, with and without FU.
	real("MTH$SNGLG", "F", fl("G", "1.0E300"))
	real("MTH$SNGLG", "F", fl("G", "1.0E-300"))
	real("MTH$SNGLG", "F", fl("G", "1.0E-300")).withFU()
	real("MTH$SNGL", "F", fl("D", "1.5"))
	real("MTH$CVT_G_D", "D", fl("G", "1.0E300"))
	real("MTH$CVT_G_D", "D", fl("G", "1.0E-300"))
	real("MTH$CVT_G_D", "D", fl("G", "1.0E-300")).withFU()
	real("MTH$CVT_D_G", "G", fl("D", "1.5"))
	real("MTH$DPROD", "D", fl("F", "1.0E30"), fl("F", "1.0E30")).mode1(modeUnwind)
	real("MTH$DPROD", "D", fl("F", "1.0E-30"), fl("F", "1.0E-30"))
	real("MTH$DPROD", "D", fl("F", "1.0E-30"), fl("F", "1.0E-30")).withFU().mode1(modeUnwind)
	real("MTH$DIM", "F", fl("F", "1.0E38"), fl("F", "-1.0E38")).mode1(modeUnwind)

	// The array conversions: R0, the default count, and an element out of range.
	array := func(words ...uint32) arg { return arg{text: "ARRAY", words: words} }
	gHuge, gOK := packed(vaxfloat.G, "1.0E300"), packed(vaxfloat.G, "1.5")
	add(&kase{entry: "MTH$CVT_GA_DA", how: byRef, args: []arg{array(gOK...), res0}, regs: 1, mem: 4})
	add(&kase{entry: "MTH$CVT_GA_DA", how: byRef, args: []arg{array(append(append(append([]uint32{}, gOK...), gHuge...), gOK...)...), res0, long(3)}, regs: 1, mem: 6})
	add(&kase{entry: "MTH$CVT_DA_GA", how: byRef, args: []arg{array(packed(vaxfloat.D, "1.5")...), res0, long(1)}, regs: 1, mem: 2})
}

func complexCases() {
	header("complex functions")

	forms("MTH$CABS MTH$CDABS MTH$CGABS", func(n, l string) {
		real(n, l, cx(l, "3.0", "4.0"))
		real(n, l, cx(l, huge[l], huge[l]))
		real(n, l, cxr(l, false))
	})

	forms("MTH$CEXP MTH$CDEXP MTH$CGEXP", func(n, l string) {
		complexResult(n, l, cx(l, "1.0", "1.0"))
		complexResult(n, l, cx(l, huge[l], "1.0"))
		complexResult(n, l, cx(l, "-"+huge[l], "1.0"))
		complexResult(n, l, cx(l, "-"+huge[l], "1.0")).withFU()
	})

	complexResult("MTH$CEXP", "F", cx("F", "88.5", "0.0"))
	complexResult("MTH$CEXP", "F", cx("F", "88.5", "1.5"))

	for _, fam := range []string{"MTH$CSIN MTH$CDSIN MTH$CGSIN", "MTH$CCOS MTH$CDCOS MTH$CGCOS"} {
		forms(fam, func(n, l string) {
			complexResult(n, l, cx(l, "1.0", "1.0"))
			complexResult(n, l, cx(l, "1.0", huge[l]))
		})
	}

	// Where CSIN's and CCOS' overflow is: EXP's 88.029 or COSH's 88.722.
	for _, i := range []string{"88.02", "88.03", "88.5", "88.72", "88.73"} {
		complexResult("MTH$CSIN", "F", cx("F", "1.0", i))
		complexResult("MTH$CCOS", "F", cx("F", "1.0", i))
		complexResult("MTH$CCOS", "F", cx("F", "1.0E-5", i))
	}

	forms("MTH$CLOG MTH$CDLOG MTH$CGLOG", func(n, l string) {
		complexResult(n, l, cx(l, "3.0", "4.0"))
		complexResult(n, l, cx(l, "0.0", "0.0"))
		complexResult(n, l, cx(l, huge[l], huge[l])).mode1(modeUnwind)
	})

	forms("MTH$CSQRT MTH$CDSQRT MTH$CGSQRT", func(n, l string) {
		complexResult(n, l, cx(l, "3.0", "4.0"))
		complexResult(n, l, cx(l, "0.0", "0.0"))
		complexResult(n, l, cx(l, "-1.0", "0.0"))
		complexResult(n, l, cx(l, huge[l], "0.0")).mode1(modeUnwind)
		complexResult(n, l, cxr(l, true))
	})

	forms("MTH$CONJG MTH$DCONJG MTH$GCONJG", func(n, l string) {
		complexResult(n, l, cx(l, "1.0", "2.0"))
		complexResult(n, l, cxr(l, true))
		complexResult(n, l, cxr(l, false))
	})

	// REAL and AIMAG: do they fault on the part they don't return?
	forms("MTH$REAL MTH$DREAL MTH$GREAL", func(n, l string) {
		real(n, l, cx(l, "1.0", "2.0"))
		real(n, l, cxr(l, true))
		real(n, l, cxr(l, false))
	})

	forms("MTH$AIMAG MTH$DIMAG MTH$GIMAG", func(n, l string) {
		real(n, l, cx(l, "1.0", "2.0"))
		real(n, l, cxr(l, true))
		real(n, l, cxr(l, false))
	})

	forms("MTH$CMPLX MTH$DCMPLX MTH$GCMPLX", func(n, l string) {
		complexResult(n, l, fl(l, "1.0"), fl(l, "2.0"))
		complexResult(n, l, reserved(l), fl(l, "2.0"))
	})
}

func otsCases() {
	header("OTS$ powers")

	rr := func(entry, l string, regs int, x, y string) *kase {
		return value(entry, regs, fl(l, x), fl(l, y))
	}

	for _, c := range []struct {
		entry, l string
		regs     int
	}{{"OTS$POWRR", "F", 1}, {"OTS$POWDD", "D", 2}, {"OTS$POWGG", "G", 2}, {"OTS$POWHH_R3", "H", 4}} {
		rr(c.entry, c.l, c.regs, "2.0", "0.5")
		rr(c.entry, c.l, c.regs, "0.0", "0.0")
		rr(c.entry, c.l, c.regs, "0.0", "-1.0")
		rr(c.entry, c.l, c.regs, "0.0", "2.0")
		rr(c.entry, c.l, c.regs, "-2.0", "1.0")
		rr(c.entry, c.l, c.regs, "-2.0", "2.0")
		rr(c.entry, c.l, c.regs, huge[c.l], huge[c.l])
		rr(c.entry, c.l, c.regs, huge[c.l], "-"+huge[c.l])
		rr(c.entry, c.l, c.regs, huge[c.l], "-"+huge[c.l]).withFU()
		value(c.entry, c.regs, reserved(c.l), fl(c.l, "1.0")).mode1(modeUnwind)
	}

	for _, c := range []struct {
		entry, l string
		regs     int
	}{{"OTS$POWRJ", "F", 1}, {"OTS$POWDJ", "D", 2}, {"OTS$POWGJ", "G", 2}, {"OTS$POWHJ_R3", "H", 4}} {
		for _, y := range []int64{0, 1, 5, -5, -1} {
			value(c.entry, c.regs, fl(c.l, "3.0"), long(y))
		}

		value(c.entry, c.regs, fl(c.l, "2.0"), long(-127))
		value(c.entry, c.regs, fl(c.l, "2.0"), long(100000)).mode1(modeUnwind)
		value(c.entry, c.regs, fl(c.l, "0.5"), long(100000))
		value(c.entry, c.regs, fl(c.l, "0.5"), long(100000)).withFU()
		value(c.entry, c.regs, fl(c.l, "0.5"), long(-100000)).mode1(modeUnwind)
		value(c.entry, c.regs, fl(c.l, "0.0"), long(0))
		value(c.entry, c.regs, fl(c.l, "0.0"), long(-1))
	}

	value("OTS$POWRLU", 1, fl("F", "3.0"), long(5))
	value("OTS$POWDLU", 2, fl("D", "3.0"), long(5))
	value("OTS$POWGLU", 2, fl("G", "3.0"), long(5))
	value("OTS$POWHLU_R3", 4, fl("H", "3.0"), long(5))
	value("OTS$POWRLU", 1, fl("F", "0.0"), long(0))
	value("OTS$POWRD", 2, fl("F", "2.0"), fl("D", "0.5"))
	value("OTS$POWDR", 2, fl("D", "2.0"), fl("F", "0.5"))

	// The integer powers: negative bases and exponents, and overflow.
	for _, p := range [][2]int64{{2, 10}, {-2, 3}, {-2, 4}, {2, -1}, {1, -3}, {-1, -3}, {-1, -4}, {0, 0}, {0, -1}, {0, 3}, {200, 2}} {
		value("OTS$POWII", 1, word(p[0]), word(p[1]))
	}

	for _, p := range [][2]int64{{2, 10}, {-2, 3}, {-2, 4}, {2, -1}, {1, -3}, {-1, -3}, {-1, -4}, {0, 0}, {0, -1}, {0, 3}, {65536, 2}, {2, 31}} {
		value("OTS$POWJJ", 1, long(p[0]), long(p[1]))
	}

	value("OTS$POWLULU", 1, long(3), long(5))
	value("OTS$POWLULU", 1, long(65536), long(2))
	value("OTS$POWLULU", 1, long(0), long(0))

	header("OTS$ complex")

	for _, c := range []struct {
		l, entry, power string
		regs            int
	}{
		{"F", "OTS$POWCC", "OTS$POWCJ", 2}, {"D", "OTS$POWCDCD_R3", "OTS$POWCDJ_R3", 4}, {"G", "OTS$POWCGCG_R3", "OTS$POWCGJ_R3", 4},
	} {
		value(c.entry, c.regs, cx(c.l, "1.0", "1.0"), cx(c.l, "2.0", "0.5"))
		value(c.entry, c.regs, cx(c.l, "-1.0", "0.0"), cx(c.l, "2.0", "0.0"))
		value(c.entry, c.regs, cx(c.l, "0.0", "0.0"), cx(c.l, "2.0", "0.0"))
		value(c.entry, c.regs, cx(c.l, "0.0", "0.0"), cx(c.l, "0.0", "0.0"))
		value(c.entry, c.regs, cx(c.l, huge[c.l], "0.0"), cx(c.l, huge[c.l], "0.0")).mode1(modeUnwind)
		value(c.power, c.regs, cx(c.l, "1.0", "1.0"), long(5))
		value(c.power, c.regs, cx(c.l, "1.0", "1.0"), long(0))
		value(c.power, c.regs, cx(c.l, "2.0", "0.0"), long(-1))
		value(c.power, c.regs, cx(c.l, "1.0", "1.0"), long(-3))
		value(c.power, c.regs, cx(c.l, "0.0", "0.0"), long(0))
		value(c.power, c.regs, cx(c.l, "0.0", "0.0"), long(-1)).mode1(modeUnwind)
		value(c.power, c.regs, cx(c.l, huge[c.l], huge[c.l]), long(2)).mode1(modeUnwind)
	}

	for _, c := range []struct {
		l, div, mul string
		regs        int
	}{{"F", "OTS$DIVC", "", 2}, {"D", "OTS$DIVCD_R3", "OTS$MULCD_R3", 4}, {"G", "OTS$DIVCG_R3", "OTS$MULCG_R3", 4}} {
		value(c.div, c.regs, cx(c.l, "1.0", "2.0"), cx(c.l, "3.0", "4.0"))
		value(c.div, c.regs, cx(c.l, "1.0", "2.0"), cx(c.l, "0.0", "0.0")).mode1(modeUnwind)
		value(c.div, c.regs, cx(c.l, huge[c.l], huge[c.l]), cx(c.l, tiny[c.l], tiny[c.l])).mode1(modeUnwind)
		value(c.div, c.regs, cx(c.l, tiny[c.l], tiny[c.l]), cx(c.l, huge[c.l], huge[c.l]))
		value(c.div, c.regs, cx(c.l, tiny[c.l], tiny[c.l]), cx(c.l, huge[c.l], huge[c.l])).withFU()

		if c.mul != "" {
			value(c.mul, c.regs, cx(c.l, "1.0", "2.0"), cx(c.l, "3.0", "4.0"))
			value(c.mul, c.regs, cx(c.l, huge[c.l], huge[c.l]), cx(c.l, huge[c.l], huge[c.l]))
			value(c.mul, c.regs, cx(c.l, tiny[c.l], tiny[c.l]), cx(c.l, tiny[c.l], tiny[c.l]))
			value(c.mul, c.regs, cx(c.l, tiny[c.l], tiny[c.l]), cx(c.l, tiny[c.l], tiny[c.l])).withFU()
		}
	}
}

func jsbCases() {
	header("JSB entries the manuals give")

	for _, c := range []struct {
		entry, l string
		regs     int
		x        string
	}{
		{"MTH$SIN_R4", "F", 1, "1.0"}, {"MTH$DSIN_R7", "D", 2, "1.0"}, {"MTH$GSIN_R7", "G", 2, "1.0"}, {"MTH$HSIN_R5", "H", 4, "1.0"},
		{"MTH$EXP_R4", "F", 1, "1.0"}, {"MTH$DEXP_R6", "D", 2, "1.0"}, {"MTH$GEXP_R6", "G", 2, "1.0"}, {"MTH$HEXP_R6", "H", 4, "1.0"},
		{"MTH$EXP_R4", "F", 2, "1.0E30"}, {"MTH$DEXP_R6", "D", 2, "1.0E30"}, {"MTH$GEXP_R6", "G", 2, "1.0E300"}, {"MTH$HEXP_R6", "H", 4, "1.0E4000"},
		{"MTH$SQRT_R3", "F", 1, "2.0"}, {"MTH$DSQRT_R5", "D", 2, "2.0"}, {"MTH$GSQRT_R5", "G", 2, "2.0"}, {"MTH$HSQRT_R8", "H", 4, "2.0"},
		{"MTH$SQRT_R3", "F", 2, "-1.0"}, {"MTH$DSQRT_R5", "D", 2, "-1.0"}, {"MTH$GSQRT_R5", "G", 2, "-1.0"}, {"MTH$HSQRT_R8", "H", 4, "-1.0"},
		{"MTH$SINCOS_R5", "F", 2, "1.0"}, {"MTH$DSINCOS_R7", "D", 4, "1.0"}, {"MTH$GSINCOS_R7", "G", 4, "1.0"}, {"MTH$HSINCOS_R7", "H", 8, "1.0"},
		{"MTH$FLOOR_R1", "F", 1, "-1.5"}, {"MTH$HFLOOR_R7", "H", 4, "-1.5"}, {"MTH$AINT_R2", "F", 1, "-1.5"}, {"MTH$HINT_R8", "H", 4, "-1.5"},
	} {
		jsb(c.entry, c.regs, fl(c.l, c.x))
	}

	header("JSB entries no manual gives")

	// Their registers are the question. The arguments are in R0 up, as section 1.2
	// has it for one argument, and every register is shown, with markers in the
	// others, so whatever they take or leave can be seen. A second argument follows
	// the first in the registers, the values chosen so a swap shows.
	one := []struct {
		entry, l, x string
	}{
		{"MTH$SQRT_R2", "F", "4.0"}, {"MTH$HTAN_R7", "H", "1.0"}, {"MTH$HTAND_R7", "H", "45.0"},
		{"MTH$COSH_R4", "F", "1.0"}, {"MTH$DCOSH_R7", "D", "1.0"}, {"MTH$GCOSH_R7", "G", "1.0"},
		{"MTH$SINH_R5", "F", "1.0"}, {"MTH$DSINH_R8", "D", "1.0"}, {"MTH$GSINH_R8", "G", "1.0"},
		{"MTH$TANH_R5", "F", "1.0"}, {"MTH$DTANH_R9", "D", "1.0"}, {"MTH$GTANH_R9", "G", "1.0"},
		{"MTH$SGN_R1", "F", "-2.0"},
	}

	for _, c := range one {
		jsb(c.entry, 12, fl(c.l, c.x)).marked()
	}

	two := []struct{ entry, l, x, y string }{
		{"MTH$ATAN2_R4", "F", "1.0", "2.0"}, {"MTH$DATAN2_R7", "D", "1.0", "2.0"}, {"MTH$GATAN2_R7", "G", "1.0", "2.0"},
		{"MTH$ATAND2_R4", "F", "1.0", "2.0"}, {"MTH$DATAND2_R7", "D", "1.0", "2.0"}, {"MTH$GATAND2_R7", "G", "1.0", "2.0"},
		{"MTH$AMOD_R5", "F", "7.5", "2.0"}, {"MTH$DMOD_R9", "D", "7.5", "2.0"}, {"MTH$GMOD_R9", "G", "7.5", "2.0"},
		{"MTH$SIGN_R1", "F", "1.5", "-2.0"}, {"MTH$DSIGN_R3", "D", "1.5", "-2.0"}, {"MTH$GSIGN_R3", "G", "1.5", "-2.0"},
	}

	for _, c := range two {
		jsb(c.entry, 12, fl(c.l, c.x), fl(c.l, c.y)).marked()
	}

	for _, c := range []struct{ entry, l string }{
		{"MTH$CABS_R4", "F"}, {"MTH$CDABS_R6", "D"}, {"MTH$CGABS_R6", "G"},
		{"MTH$CEXP_R6", "F"}, {"MTH$CDEXP_R9", "D"}, {"MTH$CGEXP_R9", "G"},
		{"MTH$CLOG_R6", "F"}, {"MTH$CDLOG_R9", "D"}, {"MTH$CGLOG_R9", "G"},
		{"MTH$CSIN_R8", "F"}, {"MTH$CDSIN_R9", "D"}, {"MTH$CGSIN_R9", "G"},
		{"MTH$CCOS_R8", "F"}, {"MTH$CDCOS_R9", "D"}, {"MTH$CGCOS_R9", "G"},
		{"MTH$CSQRT_R7", "F"}, {"MTH$CDSQRT_R9", "D"}, {"MTH$CGSQRT_R9", "G"},
	} {
		jsb(c.entry, 12, cx(c.l, "3.0", "4.0")).marked()
	}
}

// sweep draws arguments from a fixed sequence: a fraction of the format's full
// precision, and a binary exponent from low to high.
type sweep struct {
	r *rand.Rand
}

func (s sweep) draw(l string, low, high int, signed bool) []uint32 {
	f := formats[l]
	prec := int(f.Precision())
	m := new(big.Int).SetUint64(1)

	for range prec - 1 {
		m.Lsh(m, 1)

		if s.r.IntN(2) == 1 {
			m.SetBit(m, 0, 1)
		}
	}

	x := new(big.Float).SetPrec(uint(prec)).SetInt(m)
	x.SetMantExp(x, low+s.r.IntN(high-low+1)-prec)

	if signed && s.r.IntN(2) == 1 {
		x.Neg(x)
	}

	v, err := vaxfloat.Round(f, vaxfloat.FromBig(x))
	if err != nil {
		log.Fatal(err)
	}

	b, err := vaxfloat.Pack(f, v)
	if err != nil {
		log.Fatal(err)
	}

	return wordsOf(f, b)
}

func title(l, entry string, args []arg) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.text
	}

	return "~" + l + " " + entry + " " + strings.Join(parts, " ")
}

func sweepCases() {
	header("sweep")

	const count = 8

	s := sweep{rand.New(rand.NewPCG(77, 1977))}

	real1 := func(names string, low, high int, signed bool) {
		forms(names, func(n, l string) {
			for range count {
				a := hex(l, s.draw(l, low, high, signed))
				real(n, l, a).title = title(l, n, []arg{a})
			}
		})
	}

	real1("MTH$SQRT MTH$DSQRT MTH$GSQRT MTH$HSQRT", -20, 20, false)
	real1("MTH$EXP MTH$DEXP MTH$GEXP MTH$HEXP", -6, 6, true)
	real1("MTH$ALOG MTH$DLOG MTH$GLOG MTH$HLOG", -30, 30, false)
	real1("MTH$ALOG2 MTH$DLOG2 MTH$GLOG2 MTH$HLOG2", -30, 30, false)
	real1("MTH$ALOG10 MTH$DLOG10 MTH$GLOG10 MTH$HLOG10", -30, 30, false)
	real1("MTH$SIN MTH$DSIN MTH$GSIN MTH$HSIN", -10, 8, true)
	real1("MTH$COS MTH$DCOS MTH$GCOS MTH$HCOS", -10, 8, true)
	real1("MTH$TAN MTH$DTAN MTH$GTAN MTH$HTAN", -10, 8, true)
	real1("MTH$SIND MTH$DSIND MTH$GSIND MTH$HSIND", -4, 9, true)
	real1("MTH$COSD MTH$DCOSD MTH$GCOSD MTH$HCOSD", -4, 9, true)
	real1("MTH$TAND MTH$DTAND MTH$GTAND MTH$HTAND", -4, 9, true)
	real1("MTH$ASIN MTH$DASIN MTH$GASIN MTH$HASIN", -10, -1, true)
	real1("MTH$ACOS MTH$DACOS MTH$GACOS MTH$HACOS", -10, -1, true)
	real1("MTH$ATAN MTH$DATAN MTH$GATAN MTH$HATAN", -10, 10, true)
	real1("MTH$ASIND MTH$DASIND MTH$GASIND MTH$HASIND", -10, -1, true)
	real1("MTH$ACOSD MTH$DACOSD MTH$GACOSD MTH$HACOSD", -10, -1, true)
	real1("MTH$ATAND MTH$DATAND MTH$GATAND MTH$HATAND", -10, 10, true)
	real1("MTH$SINH MTH$DSINH MTH$GSINH MTH$HSINH", -6, 5, true)
	real1("MTH$COSH MTH$DCOSH MTH$GCOSH MTH$HCOSH", -6, 5, true)
	real1("MTH$TANH MTH$DTANH MTH$GTANH MTH$HTANH", -6, 3, true)
	real1("MTH$ATANH MTH$DATANH MTH$GATANH MTH$HATANH", -10, -1, true)

	for _, names := range []string{"MTH$ATAN2 MTH$DATAN2 MTH$GATAN2 MTH$HATAN2", "MTH$ATAND2 MTH$DATAND2 MTH$GATAND2 MTH$HATAND2"} {
		forms(names, func(n, l string) {
			for range count {
				a, b := hex(l, s.draw(l, -5, 5, true)), hex(l, s.draw(l, -5, 5, true))
				real(n, l, a, b).title = title(l, n, []arg{a, b})
			}
		})
	}

	for _, names := range []string{"MTH$SINCOS MTH$DSINCOS MTH$GSINCOS MTH$HSINCOS", "MTH$SINCOSD MTH$DSINCOSD MTH$GSINCOSD MTH$HSINCOSD"} {
		forms(names, func(n, l string) {
			for range count {
				a := hex(l, s.draw(l, -4, 8, true))
				c := add(&kase{entry: n, how: byRef, args: []arg{a, res0, res16}, mem: 8})
				c.title = title(l, n, []arg{a})
			}
		})
	}

	cxPair := func(l string) arg {
		return hex(l, append(s.draw(l, -4, 3, true), s.draw(l, -4, 3, true)...))
	}

	forms("MTH$CABS MTH$CDABS MTH$CGABS", func(n, l string) {
		for range count {
			a := cxPair(l)
			real(n, l, a).title = title(l, n, []arg{a})
		}
	})

	for _, names := range []string{"MTH$CEXP MTH$CDEXP MTH$CGEXP", "MTH$CLOG MTH$CDLOG MTH$CGLOG", "MTH$CSIN MTH$CDSIN MTH$CGSIN", "MTH$CCOS MTH$CDCOS MTH$CGCOS", "MTH$CSQRT MTH$CDSQRT MTH$CGSQRT"} {
		forms(names, func(n, l string) {
			for range count {
				a := cxPair(l)
				complexResult(n, l, a).title = title(l, n, []arg{a})
			}
		})
	}

	for _, c := range []struct {
		entry, l string
		regs     int
	}{{"OTS$POWRR", "F", 1}, {"OTS$POWDD", "D", 2}, {"OTS$POWGG", "G", 2}, {"OTS$POWHH_R3", "H", 4}} {
		for range count {
			a, b := hex(c.l, s.draw(c.l, -4, 4, false)), hex(c.l, s.draw(c.l, -3, 3, true))
			value(c.entry, c.regs, a, b).title = title(c.l, c.entry, []arg{a, b})
		}
	}

	for _, c := range []struct {
		entry, l string
		regs     int
	}{{"OTS$POWRJ", "F", 1}, {"OTS$POWDJ", "D", 2}, {"OTS$POWGJ", "G", 2}, {"OTS$POWHJ_R3", "H", 4}} {
		for range count {
			a, b := hex(c.l, s.draw(c.l, -2, 2, true)), long(int64(s.r.IntN(41)-20))
			value(c.entry, c.regs, a, b).title = title(c.l, c.entry, []arg{a, b})
		}
	}

	for _, c := range []struct {
		entry, l string
		regs     int
	}{{"OTS$POWCC", "F", 2}, {"OTS$POWCDCD_R3", "D", 4}, {"OTS$POWCGCG_R3", "G", 4}, {"OTS$DIVC", "F", 2}, {"OTS$DIVCD_R3", "D", 4}, {"OTS$DIVCG_R3", "G", 4}, {"OTS$MULCD_R3", "D", 4}, {"OTS$MULCG_R3", "G", 4}} {
		for range count {
			a, b := cxPair(c.l), cxPair(c.l)
			value(c.entry, c.regs, a, b).title = title(c.l, c.entry, []arg{a, b})
		}
	}
}

const frame = `	.TITLE	MTH77
	.IDENT	/1/
; MTH77 - the Phase 77 probe of MTHRTL. Written by testdata/mth77/gen.go.
	$SSDEF
	.PSECT	RODATA,NOWRT,NOEXE,LONG
RO:	.LONG	1
	.PSECT	DATA,WRT,NOEXE,LONG
HEXTAB:	.ASCII	/0123456789ABCDEF/
TXTR:	.ASCII	/ R=/
TXTM:	.ASCII	/ M=/
TXTC:	.ASCII	/ c=/
TXTN:	.ASCII	/ n=/
TXTA:	.ASCII	/ a=/
TXTRET:	.ASCII	/ret /
TXTPCR:	.ASCII	/ pc=ret/
TXTPCO:	.ASCII	/ pc=oth/
TXTMR:	.ASCII	/ mr=/
TXTH:	.ASCII	/ h=/
TXTUNW:	.ASCII	/ unw/
EXPECT:	.BLKL	1		; the call's return address
MODE:	.BLKL	1		; what HANDLER does
HCOUNT:	.BLKL	1		; the signals in this case
UNWOUND:.BLKL	1		; HANDLER unwound
SAVED:	.BLKL	1		; the call came back
SIG:	.BLKL	12		; the first signal array
MECH0:	.BLKL	1		; the mechanism array's R0
MECH1:	.BLKL	1		; and R1
REGS:	.BLKL	12		; R0 to R11 after the call
RES:	.BLKL	8		; the result area
LINE:	.BLKB	500
LDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS LINE

	.PSECT	CODE,NOWRT,EXE,LONG
; HANDLER: see gen.go. It records the first signal, and continues from the library's
; conditions and the arithmetic traps, whose PC is past the instruction. Anything else
; would run the instruction again, so it unwinds to the driver, with the condition in R0.
	.ENTRY	HANDLER,^M<R2,R3>
	MOVL	4(AP),R2		; the signal array
	MOVL	8(AP),R3		; the mechanism array
	CMPL	4(R2),#SS$_UNWIND
	BNEQ	1$
	BRW	9$
1$:	INCL	HCOUNT
	CMPL	HCOUNT,#1
	BNEQ	3$
	MOVL	12(R3),MECH0		; CHF$L_MCH_SAVR0
	MOVL	16(R3),MECH1		; CHF$L_MCH_SAVR1
	CLRL	R0
2$:	CMPL	R0,(R2)			; the signal array's longwords, at most 12
	BGTR	3$
	CMPL	R0,#12
	BGEQ	3$
	MOVL	(R2)[R0],SIG[R0]
	INCL	R0
	BRB	2$
3$:	CMPL	HCOUNT,#1
	BNEQ	8$
	CMPL	MODE,#1
	BEQL	8$
	EXTZV	#16,#12,4(R2),R0
	CMPL	R0,#22			; the MTH facility
	BEQL	5$
	CMPL	4(R2),#SS$_INTOVF
	BEQL	5$
	CMPL	4(R2),#SS$_INTDIV
	BEQL	5$
	CMPL	4(R2),#SS$_FLTOVF
	BEQL	5$
	CMPL	4(R2),#SS$_FLTDIV
	BEQL	5$
	CMPL	4(R2),#SS$_FLTUND
	BNEQ	8$
5$:	CMPL	MODE,#2
	BNEQ	6$
	MOVL	#^X4180,12(R3)		; 1.5, in place of the reserved operand
	CLRL	16(R3)
6$:	MOVL	#SS$_CONTINUE,R0
	RET
8$:	MOVL	#1,UNWOUND
	MOVL	4(R2),12(R3)
	$UNWIND_S
9$:	RET

; SAVE keeps R0 to R11, as the call left them.
SAVE:	MOVL	R0,REGS
	MOVL	R1,REGS+4
	MOVL	R2,REGS+8
	MOVL	R3,REGS+12
	MOVL	R4,REGS+16
	MOVL	R5,REGS+20
	MOVL	R6,REGS+24
	MOVL	R7,REGS+28
	MOVL	R8,REGS+32
	MOVL	R9,REGS+36
	MOVL	R10,REGS+40
	MOVL	R11,REGS+44
	MOVL	#1,SAVED
	RSB

; MARK puts a marker in R1 to R11, so what a routine changes shows.
MARK:	MOVL	#^XA1A1A1A1,R1
	MOVL	#^XA2A2A2A2,R2
	MOVL	#^XA3A3A3A3,R3
	MOVL	#^XA4A4A4A4,R4
	MOVL	#^XA5A5A5A5,R5
	MOVL	#^XA6A6A6A6,R6
	MOVL	#^XA7A7A7A7,R7
	MOVL	#^XA8A8A8A8,R8
	MOVL	#^XA9A9A9A9,R9
	MOVL	#^XAAAAAAAA,R10
	MOVL	#^XABABABAB,R11
	RSB

; PUTHEX writes R1 in hex at (R3) and moves R3 on. It changes R0 and R2 too.
PUTHEX:	MOVL	#28,R2
1$:	EXTZV	R2,#4,R1,R0
	MOVB	HEXTAB[R0],(R3)+
	SUBL2	#4,R2
	BGEQ	1$
	RSB

; START runs the table's cases. Each entry is a case's routine, its title, and four
; bytes: the registers and result longwords to show, the handler's mode, and whether
; the entry is only a heading.
	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAL	TABLE,R6
NEXT:	MOVL	(R6)+,R7
	BNEQ	10$
	MOVL	#1,R0
	RET
10$:	MOVL	(R6)+,R8
	MOVZBL	(R6),R9
	MOVZBL	1(R6),R10
	MOVZBL	2(R6),MODE
	MOVZBL	3(R6),R11
	ADDL2	#4,R6
	TSTL	R11
	BNEQ	20$
	CLRL	HCOUNT
	CLRL	UNWOUND
	CLRL	SAVED
	CLRL	EXPECT
	MOVC5	#0,(SP),#^X55,#32,RES
	CALLS	#0,(R7)
20$:	MOVAB	LINE,R3
	MOVZWL	(R8),R0
	MOVC3	R0,@4(R8),(R3)
	TSTL	R11
	BEQL	21$
	BRW	PRINT
21$:	TSTL	SAVED
	BEQL	30$
	TSTL	R9
	BEQL	25$
	MOVC3	#3,TXTR,(R3)
	CLRL	R4
22$:	CMPL	R4,R9
	BGEQ	25$
	TSTL	R4
	BEQL	23$
	MOVB	#^A/ /,(R3)+
23$:	MOVL	REGS[R4],R1
	BSBW	PUTHEX
	INCL	R4
	BRB	22$
25$:	TSTL	R10
	BEQL	30$
	MOVC3	#3,TXTM,(R3)
	CLRL	R4
26$:	CMPL	R4,R10
	BGEQ	30$
	TSTL	R4
	BEQL	27$
	MOVB	#^A/ /,(R3)+
27$:	MOVL	RES[R4],R1
	BSBW	PUTHEX
	INCL	R4
	BRB	26$
30$:	TSTL	HCOUNT
	BNEQ	31$
	BRW	PRINT
31$:	MOVC3	#3,TXTC,(R3)
	MOVL	SIG+4,R1
	BSBW	PUTHEX
	MOVC3	#3,TXTN,(R3)
	MOVL	SIG,R1
	BSBW	PUTHEX
	MOVL	#2,R7			; the arguments between the condition and the PC; MOVC3 changes R4
32$:	SUBL3	#2,SIG,R0
	CMPL	R7,R0
	BGTR	36$
	CMPL	R7,#11
	BGEQ	36$
	MOVC3	#3,TXTA,(R3)
	MOVL	SIG[R7],R1
	CMPL	R1,EXPECT
	BNEQ	33$
	MOVC3	#3,TXTRET,(R3)
	BRB	35$
33$:	BSBW	PUTHEX
35$:	INCL	R7
	BRB	32$
36$:	SUBL3	#1,SIG,R4		; the PC
	BLSS	38$
	CMPL	R4,#12
	BGEQ	38$
	CMPL	SIG[R4],EXPECT
	BNEQ	37$
	MOVC3	#7,TXTPCR,(R3)
	BRB	38$
37$:	MOVC3	#7,TXTPCO,(R3)
38$:	MOVC3	#4,TXTMR,(R3)
	MOVL	MECH0,R1
	BSBW	PUTHEX
	MOVB	#^A/ /,(R3)+
	MOVL	MECH1,R1
	BSBW	PUTHEX
	CMPL	HCOUNT,#1
	BEQL	39$
	MOVC3	#3,TXTH,(R3)
	MOVL	HCOUNT,R1
	BSBW	PUTHEX
39$:	TSTL	UNWOUND
	BEQL	PRINT
	MOVC3	#4,TXTUNW,(R3)
PRINT:	MOVAB	LINE,R0
	SUBL3	R0,R3,R1
	MOVW	R1,LDSC
	PUSHAQ	LDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	BRW	NEXT

`

func writeProgram() string {
	var data, code, table strings.Builder

	data.WriteString("\t.PSECT\tDATA\n")
	table.WriteString("\t.PSECT\tDATA\nTABLE:\n")

	for i, c := range cases {
		name := c.title
		if name == "" {
			parts := make([]string, len(c.args))
			for j, a := range c.args {
				parts[j] = a.text
			}

			name = c.entry + "(" + strings.Join(parts, ",") + ")"
		}

		if c.fu {
			name += " FU"
		}

		if c.mode == modeUnwind {
			name += " UW"
		}

		if c.mode == modeCorrect {
			name += " CORR"
		}

		fmt.Fprintf(&data, "N%d:\t.ASCID\t\"%s\"\n", i, name)

		if c.header {
			fmt.Fprintf(&table, "\t.LONG\t1\n\t.ADDRESS N%d\n\t.BYTE\t0,0,0,1\n", i)

			continue
		}

		fmt.Fprintf(&table, "\t.ADDRESS C%d\n\t.ADDRESS N%d\n\t.BYTE\t%d,%d,%d,0\n", i, i, c.regs, c.mem, c.mode)

		fmt.Fprintf(&code, "\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", i)

		if c.fu {
			code.WriteString("\tBISPSW\t#^X40\n")
		}

		if c.marks {
			code.WriteString("\tJSB\tMARK\n")
		}

		fmt.Fprintf(&code, "\tMOVAB\t1$,EXPECT\n")

		count := 0

		switch c.how {
		case byJSB:
			r := 0

			for _, a := range c.args {
				for _, w := range a.words {
					fmt.Fprintf(&code, "\tMOVL\t#^X%08X,R%d\n", w, r)
					r++
				}
			}

			fmt.Fprintf(&code, "\tJSB\tG^%s\n", c.entry)
		case byVal:
			for j := len(c.args) - 1; j >= 0; j-- {
				for k := len(c.args[j].words) - 1; k >= 0; k-- {
					fmt.Fprintf(&code, "\tPUSHL\t#^X%08X\n", c.args[j].words[k])
					count++
				}
			}

			fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n", count, c.entry)
		case byRef:
			for j := len(c.args) - 1; j >= 0; j-- {
				a := c.args[j]

				switch {
				case a.bad:
					code.WriteString("\tPUSHL\t#^X3FFF0000\n")
				case a.ro:
					code.WriteString("\tPUSHAL\tRO\n")
				case a.isRes:
					fmt.Fprintf(&code, "\tPUSHAL\tRES+%d\n", a.res)
				default:
					label := fmt.Sprintf("A%d_%d", i, j)
					texts := make([]string, len(a.words))

					for k, w := range a.words {
						texts[k] = fmt.Sprintf("^X%08X", w)
					}

					fmt.Fprintf(&data, "%s:\t.LONG\t%s\n", label, strings.Join(texts, ","))
					fmt.Fprintf(&code, "\tPUSHAL\t%s\n", label)
				}

				count++
			}

			fmt.Fprintf(&code, "\tCALLS\t#%d,G^%s\n", count, c.entry)
		}

		code.WriteString("1$:\tJSB\tSAVE\n")

		if c.post == "seed" { // the seed, as the call left it
			fmt.Fprintf(&code, "\tMOVL\tA%d_0,RES\n", i)
		}

		code.WriteString("\tMOVL\t#1,R0\n\tRET\n")
	}

	table.WriteString("\t.LONG\t0\n")

	return frame + code.String() + data.String() + table.String() + "\t.END\tSTART\n"
}

const procedure = `$ ! MTH77.COM - the Phase 77 probe of MTHRTL.
$ ! Written by testdata/mth77/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @MTH77/OUTPUT=MTH77.LOG
$ !
$ ! MTH77.OUT gets the program's output; MTH77.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEFINE/USER_MODE SYS$OUTPUT MTH77.MLG
$ MACRO/NOLIST MTH77
$ LINK/NOMAP MTH77
$ DEFINE/USER_MODE SYS$OUTPUT MTH77.OUT
$ RUN MTH77
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE MTH77.*
$ EXIT
`

const exchange = `! EXCHANGE.CMD - builds the Phase 77 probe's exchange volume with
! govax. Written by testdata/mth77/gen.go. Run from the repository root:
!
!     govax console < testdata/mth77/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mth77-exchange.dsk" /DEVICE=RD51 MTH77
MOUNT/WRITE DUA2 "testdata/disks/mth77-exchange.dsk"
COPY "testdata/mth77/mth77.mar"/HOST DUA2:[000000]MTH77.MAR
COPY "testdata/mth77/mth77.com"/HOST DUA2:[000000]MTH77.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`

const copyout = `! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/mth77/gen.go. Run from the repository root:
!
!     govax console < testdata/mth77/copyout.cmd
!
MOUNT DUA2 "testdata/disks/mth77-exchange.dsk"
COPY DUA2:[000000]MTH77.OUT "testdata/mth77/vax/mth77-vax.txt"/HOST
COPY DUA2:[000000]MTH77.LOG "testdata/mth77/vax/mth77.log"/HOST
DISMOUNT DUA2
`

func main() {
	randomCases()
	realConditions()
	tableCases()
	complexCases()
	otsCases()
	jsbCases()
	sweepCases()

	for name, text := range map[string]string{"mth77.mar": writeProgram(), "mth77.com": procedure, "exchange.cmd": exchange, "copyout.cmd": copyout} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("%d cases\n", len(cases))
}
