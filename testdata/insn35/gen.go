//go:build ignore

// gen writes the Phase 35 instruction oracle (docs/PHASE-35.md, subtask
// 3): five VAX MACRO probe programs, one per family of instructions, the
// command procedure that assembles, links, and runs them on VMS, and the
// govax console script that builds their exchange volume. Run it from the
// repository root:
//
//	go run testdata/insn35/gen.go
//
// Each probe runs a table of cases. A case is one instruction with its
// operands, run in a routine of its own (Cn) that establishes the probe's
// condition handler. Before the instruction, Cn copies any modify
// operand's starting value into DST, loads R0-R11 with a known pattern
// (and the case's own values), and sets the PSW: by default N, Z, V, and C
// all set (so a condition code an instruction leaves alone shows as 1)
// and IV, FU, and DV clear. Right after the instruction it saves the PSL
// with MOVPSL, then R0-R11.
//
// The handler records the signal array. For a trap (the PC already past
// the instruction: integer, floating, or decimal overflow, divide by zero,
// or floating underflow), it returns SS$_CONTINUE, so the instruction's
// results are still saved; for anything else (a fault: a reserved
// operand, reserved addressing mode, or floating fault), it unwinds to
// the main program, and the case has no results. A second condition in
// one case unwinds too, so an emulator that reports a trap as a fault
// (and so runs the instruction again on SS$_CONTINUE) can't loop.
//
// Every probe writes one variable-length record per case to its .DMP
// file, 260 bytes, longwords little-endian:
//
//	  0  "I35R"
//	  4  the case number
//	  8  the case's name, 32 bytes, blank-padded
//	 40  flags: 1 the instruction completed, 2 a branch was taken,
//	     4 a condition was signalled, 8 a second condition was
//	     signalled (and the case unwound)
//	 44  the PSL just after the instruction
//	 48  R0 through R11
//	 96  the signal array: its longword count, then up to 8 of its
//	     longwords (condition, arguments, PC, PSL)
//	132  DST, 128 bytes: write and modify operands are at DST, DST+32,
//	     DST+64, and DST+96; every other byte starts as ^XAA
//
// Every operand is in memory unless a case says otherwise, so the
// results are DST's bytes; register-mode and implied register results
// are in R0-R11. Floating operands are written as bytes, not with the
// assembler's floating directives, so each case's bits are exact
// (reserved operands, all the low fraction bits set, ties), and the
// probes assemble the same way on VMS and on govax.
package main

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// op is one operand of a case's instruction.
type op struct {
	kind byte   // 'r' data read, 'm' modify (DST, preset), 'w' write (DST), 't' text
	data []byte // 'r' and 'm'
	text string // 't': the operand as written, such as "#3" or "R6"
}

func in(b []byte) op    { return op{kind: 'r', data: b} }
func mod(b []byte) op   { return op{kind: 'm', data: b} }
func out() op           { return op{kind: 'w'} }
func lit(s string) op   { return op{kind: 't', text: s} }
func length(n int) op   { return lit(fmt.Sprintf("#%d", n)) }
func cat(b ...[]byte) []byte {
	var out []byte
	for _, x := range b {
		out = append(out, x...)
	}

	return out
}

// regset presets a register before the instruction: to a value, or to
// an address.
type regset struct {
	reg   int
	value uint32
	addr  string // an address expression; D_0 is the case's first data operand
}

// testCase is one instruction.
type testCase struct {
	name   string
	insn   string
	ops    []op
	branch bool     // the instruction ends with a branch displacement
	raw    string   // the instruction, written out (insn and ops unused)
	regs   []regset // presets
	cc     int      // the starting N, Z, V, C (default ^XF)
	ccSet  bool
	psw    int      // IV, FU, DV (the PSW bits)
	post   []string // lines after the PSL is saved, before the registers are
}

func c(name, insn string, ops ...op) testCase {
	return testCase{name: name, insn: insn, ops: ops}
}

func (t testCase) br() testCase { t.branch = true; return t }

func (t testCase) withCC(cc int) testCase { t.cc, t.ccSet = cc, true; return t }

func (t testCase) iv() testCase { t.psw |= 0x20; return t }
func (t testCase) fu() testCase { t.psw |= 0x40; return t }
func (t testCase) dv() testCase { t.psw |= 0x80; return t }

func (t testCase) reg(n int, v uint32) testCase {
	t.regs = append(t.regs, regset{reg: n, value: v})
	return t
}

func (t testCase) regAddr(n int, addr string) testCase {
	t.regs = append(t.regs, regset{reg: n, addr: addr})
	return t
}

func (t testCase) then(lines ...string) testCase {
	t.post = append(t.post, lines...)
	return t
}

func rawCase(name, line string) testCase { return testCase{name: name, raw: line} }

// section is one probe program.
type section struct {
	name  string // the program's name, and its .MAR and .DMP files'
	title string
	cases []testCase
}

func main() {
	dir := filepath.Join("testdata", "insn35")

	sections := []section{
		{"P35FD", "F_floating and D_floating", fdCases()},
		{"P35G", "G_floating", floatCases(gFloat)},
		{"P35H", "H_floating", floatCases(hFloat)},
		{"P35O", "octawords", octaCases()},
		{"P35P", "packed decimal", packedCases()},
	}

	for _, s := range sections {
		checkNames(s)
		write(filepath.Join(dir, strings.ToLower(s.name)+".mar"), probe(s))
	}

	write(filepath.Join(dir, "insn35.com"), procedure(sections))
	write(filepath.Join(dir, "exchange.cmd"), exchange(sections))
}

func write(path, text string) {
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func checkNames(s section) {
	seen := map[string]bool{}

	for _, c := range s.cases {
		if len(c.name) > 32 {
			fmt.Fprintf(os.Stderr, "%s: case name over 32 characters: %q\n", s.name, c.name)
			os.Exit(1)
		}

		if seen[c.name] {
			fmt.Fprintf(os.Stderr, "%s: duplicate case name %q\n", s.name, c.name)
			os.Exit(1)
		}

		seen[c.name] = true
	}
}

// ---------------------------------------------------------------------
// Floating values.

// floatFormat describes one of the four VAX floating formats: a sign
// bit, a biased exponent, and a fraction with a hidden leading 1, stored
// as 16-bit words with the most significant word first.
type floatFormat struct {
	letter string
	ebits  int // exponent bits
	fbits  int // stored fraction bits
	bias   int
}

var (
	fFloat = floatFormat{"F", 8, 23, 128}
	dFloat = floatFormat{"D", 8, 55, 128}
	gFloat = floatFormat{"G", 11, 52, 1024}
	hFloat = floatFormat{"H", 15, 112, 16384}
)

func (f floatFormat) bits() int  { return 1 + f.ebits + f.fbits }
func (f floatFormat) bytes() int { return f.bits() / 8 }

// value parses a floating expression: terms joined by + and -, each a
// product (*) of factors: a decimal number, a quotient a/b, 2^k, or
// "max" or "min" (the format's largest and smallest positive values).
func (f floatFormat) value(expr string) *big.Float {
	sum := newFloat()
	sign := 1
	start := 0

	flush := func(end int) {
		term := strings.TrimSpace(expr[start:end])
		if term == "" {
			return
		}

		v := f.term(term)
		if sign < 0 {
			v.Neg(v)
		}

		sum.Add(sum, v)
	}

	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if (ch == '+' || ch == '-') && i > 0 && expr[i-1] != '^' && expr[i-1] != 'e' {
			flush(i)
			sign = 1
			if ch == '-' {
				sign = -1
			}

			start = i + 1
		} else if (ch == '+' || ch == '-') && i == 0 {
			if ch == '-' {
				sign = -1
			}

			start = 1
		}
	}

	flush(len(expr))

	return sum
}

func newFloat() *big.Float { return new(big.Float).SetPrec(2000) }

func (f floatFormat) term(term string) *big.Float {
	product := newFloat().SetInt64(1)

	for _, factor := range strings.Split(term, "*") {
		product.Mul(product, f.factor(strings.TrimSpace(factor)))
	}

	return product
}

func (f floatFormat) factor(s string) *big.Float {
	switch {
	case s == "max":
		// (1 - 2^-(fbits+1)) * 2^(emax-bias)
		v := newFloat().SetMantExp(newFloat().SetInt64(1), -(f.fbits + 1))
		v.Sub(newFloat().SetInt64(1), v)

		return v.SetMantExp(v, (1<<f.ebits)-1-f.bias)
	case s == "min":
		return newFloat().SetMantExp(newFloat().SetFloat64(0.5), 1-f.bias)
	case strings.HasPrefix(s, "2^"):
		k, err := strconv.Atoi(s[2:])
		if err != nil {
			panic(s)
		}

		return newFloat().SetMantExp(newFloat().SetInt64(1), k)
	case strings.Contains(s, "/"):
		a, b, _ := strings.Cut(s, "/")

		return newFloat().Quo(f.factor(a), f.factor(b))
	}

	v, _, err := big.ParseFloat(s, 10, 2000, big.ToNearestEven)
	if err != nil {
		panic(s)
	}

	return v
}

// encode returns expr in format f's memory layout, rounded to the
// format's precision half away from zero, as the architecture rounds.
func (f floatFormat) encode(expr string) []byte {
	v := f.value(expr)
	bits := new(big.Int)

	if v.Sign() != 0 {
		r := new(big.Float).SetMode(big.ToNearestAway).SetPrec(uint(f.fbits + 1)).Set(v)
		mant := new(big.Float)
		exp := r.MantExp(mant) // r = mant * 2^exp, 0.5 <= |mant| < 1
		e := exp + f.bias

		if e < 1 || e >= 1<<f.ebits {
			panic(fmt.Sprintf("%s out of %s_floating's range", expr, f.letter))
		}

		mant.Abs(mant)
		mant.SetMantExp(mant, f.fbits+1)

		frac, _ := mant.Int(nil)
		frac.Sub(frac, new(big.Int).Lsh(big.NewInt(1), uint(f.fbits)))

		bits.SetInt64(int64(e))
		bits.Lsh(bits, uint(f.fbits))
		bits.Or(bits, frac)

		if r.Sign() < 0 {
			bits.SetBit(bits, f.bits()-1, 1)
		}
	}

	return f.layout(bits)
}

// layout stores a format's logical bits (sign, exponent, fraction, most
// significant first) as memory holds them: 16-bit words, the most
// significant word at the lowest address, each word's low byte first.
func (f floatFormat) layout(bits *big.Int) []byte {
	words := f.bits() / 16
	out := make([]byte, 0, words*2)

	for i := 0; i < words; i++ {
		w := new(big.Int).Rsh(bits, uint(f.bits()-16*(i+1)))
		v := w.Uint64() & 0xFFFF
		out = append(out, byte(v), byte(v>>8))
	}

	return out
}

// words returns a value given as its 16-bit words, most significant
// first, padded with zero words to the format's size: for reserved
// operands and other patterns encode can't make.
func (f floatFormat) words(w ...uint16) []byte {
	out := make([]byte, 0, f.bytes())

	for i := 0; i < f.bytes()/2; i++ {
		var v uint16
		if i < len(w) {
			v = w[i]
		}

		out = append(out, byte(v), byte(v>>8))
	}

	return out
}

// reserved is the format's reserved operand: sign 1, exponent 0.
func (f floatFormat) reserved() []byte { return f.words(0x8000) }

// dirtyZero has sign 0 and exponent 0 but a nonzero fraction: the
// architecture treats it as zero.
func (f floatFormat) dirtyZero() []byte { return f.words(0x0001, 0x1234) }

// l returns v as a longword, w as a word, b as a byte.
func l(v int64) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
func w(v int64) []byte { return []byte{byte(v), byte(v >> 8)} }
func b(v ...byte) []byte { return v }

// ---------------------------------------------------------------------
// The floating cases.

// fdCases are the F_floating and D_floating cases: the instructions this
// phase adds (EMODF/D, POLYF/D, CVTFD, CVTDF) and the existing ones
// whose results the new floating core changes (D's low fraction bits,
// rounding ties).
func fdCases() []testCase {
	F, D := fFloat, dFloat
	f, d := F.encode, D.encode

	cases := []testCase{
		c("MOVF 1.5", "MOVF", in(f("1.5")), out()),
		c("MOVF -0.75", "MOVF", in(f("-0.75")), out()),
		c("MOVF max", "MOVF", in(f("max")), out()),
		c("MOVF min", "MOVF", in(f("min")), out()),
		c("MOVF zero", "MOVF", in(f("0")), out()),
		c("MOVF dirty zero", "MOVF", in(F.dirtyZero()), out()),
		c("MOVF reserved", "MOVF", in(F.reserved()), out()),
		c("MOVF literal 1.5", "MOVF", lit("#1.5"), out()),
		c("MOVF literal 0.5", "MOVF", lit("#0.5"), out()),
		c("MOVF immediate 1.1", "MOVF", lit("#1.1"), out()),
		c("MOVD low bits set", "MOVD", in(d("1-2^-56")), out()),
		c("MOVD 1/3", "MOVD", in(d("1/3")), out()),
		c("MOVD -1/3 to R6", "MOVD", in(d("-1/3")), lit("R6")),
		c("MOVD dirty zero", "MOVD", in(D.dirtyZero()), out()),
		c("MOVD reserved", "MOVD", in(D.reserved()), out()),
		c("MOVD literal 1.5", "MOVD", lit("#1.5"), out()),
		c("MOVD immediate 1.1", "MOVD", lit("#1.1"), out()),
		c("MNEGF 1/3", "MNEGF", in(f("1/3")), out()),
		c("MNEGF zero", "MNEGF", in(f("0")), out()),
		c("MNEGD low bits set", "MNEGD", in(d("1-2^-56")), out()),
		c("MNEGD reserved", "MNEGD", in(D.reserved()), out()),
		c("TSTF negative", "TSTF", in(f("-2"))),
		c("TSTF reserved", "TSTF", in(F.reserved())),
		c("TSTD zero", "TSTD", in(d("0"))),
		c("CMPF less", "CMPF", in(f("1")), in(f("2"))),
		c("CMPF equal", "CMPF", in(f("-3")), in(f("-3"))),
		c("CMPD low bit differs", "CMPD", in(d("1-2^-56")), in(d("1-2^-55"))),
		c("CMPD reserved", "CMPD", in(d("1")), in(D.reserved())),

		// Rounding: VAX rounds half away from zero; IEEE (and float64)
		// rounds half to even, which gives the other answer here.
		c("ADDF2 tie", "ADDF2", in(f("2^-24")), mod(f("1"))),
		c("ADDF3 negative tie", "ADDF3", in(f("-1")), in(f("-2^-24")), out()),
		c("ADDF3 above tie", "ADDF3", in(f("1")), in(f("2^-24+2^-30")), out()),
		c("SUBF3 tie", "SUBF3", in(f("2^-25")), in(f("1")), out()),
		c("SUBF3 cancel", "SUBF3", in(f("1.5")), in(f("1.5")), out()),
		c("MULF3 tie", "MULF3", in(f("1+2^-12")), in(f("1+2^-12")), out()),
		c("MULF2 -3 x 1/3", "MULF2", in(f("-3")), mod(f("1/3"))),
		c("DIVF3 1/3", "DIVF3", in(f("3")), in(f("1")), out()),
		c("DIVF2 by zero", "DIVF2", in(f("0")), mod(f("1"))),
		c("MULF3 overflow", "MULF3", in(f("max")), in(f("2")), out()),
		c("MULF3 underflow", "MULF3", in(f("min")), in(f("0.5")), out()),
		c("MULF3 underflow, FU", "MULF3", in(f("min")), in(f("0.5")), out()).fu(),
		c("ADDF3 reserved", "ADDF3", in(F.reserved()), in(f("1")), out()),
		c("ADDF3 dirty zero", "ADDF3", in(F.dirtyZero()), in(f("1")), out()),
		c("ADDD2 tie", "ADDD2", in(d("2^-56")), mod(d("1"))),
		c("ADDD3 negative tie", "ADDD3", in(d("-1")), in(d("-2^-56")), out()),
		c("ADDD3 low bits", "ADDD3", in(d("1/3")), in(d("1/3")), out()),
		c("SUBD3 low bits", "SUBD3", in(d("2^-55")), in(d("1")), out()),
		c("MULD3 tie", "MULD3", in(d("1+2^-28")), in(d("1+2^-28")), out()),
		c("MULD3 1/3 x 3", "MULD3", in(d("1/3")), in(d("3")), out()),
		c("DIVD3 1/3", "DIVD3", in(d("3")), in(d("1")), out()),
		c("DIVD3 2/3", "DIVD3", in(d("3")), in(d("2")), out()),
		c("DIVD3 1/10", "DIVD3", in(d("10")), in(d("1")), out()),
		c("DIVD2 by zero", "DIVD2", in(d("0")), mod(d("1"))),
		c("MULD3 overflow", "MULD3", in(d("max")), in(d("2")), out()),
		c("MULD3 underflow", "MULD3", in(d("min")), in(d("0.5")), out()),
		c("MULD3 underflow, FU", "MULD3", in(d("min")), in(d("0.5")), out()).fu(),
		c("ADDD3 literal and low bits", "ADDD3", lit("#0.5"), in(d("1-2^-56")), out()),

		// Conversions.
		c("CVTFD 1/3", "CVTFD", in(f("1/3")), out()),
		c("CVTFD max", "CVTFD", in(f("max")), out()),
		c("CVTFD reserved", "CVTFD", in(F.reserved()), out()),
		c("CVTFD dirty zero", "CVTFD", in(F.dirtyZero()), out()),
		c("CVTDF 1/3", "CVTDF", in(d("1/3")), out()),
		c("CVTDF tie", "CVTDF", in(d("1+2^-24")), out()),
		c("CVTDF negative tie", "CVTDF", in(d("-1-2^-24")), out()),
		c("CVTDF below tie", "CVTDF", in(d("1+2^-24-2^-50")), out()),
		c("CVTDF max rounds over", "CVTDF", in(d("max")), out()),
		c("CVTDF reserved", "CVTDF", in(D.reserved()), out()),
		c("CVTRFL 2.5", "CVTRFL", in(f("2.5")), out()),
		c("CVTRFL -2.5", "CVTRFL", in(f("-2.5")), out()),
		c("CVTRFL -0.5", "CVTRFL", in(f("-0.5")), out()),
		c("CVTFL -2.9", "CVTFL", in(f("-2.9")), out()),
		c("CVTFL overflow", "CVTFL", in(f("3e9")), out()),
		c("CVTFL overflow, IV", "CVTFL", in(f("3e9")), out()).iv(),
		c("CVTFB overflow", "CVTFB", in(f("200")), out()),
		c("CVTFW -40000", "CVTFW", in(f("-40000")), out()),
		c("CVTLF tie", "CVTLF", in(l(16777217)), out()),
		c("CVTLF 2^31-1", "CVTLF", in(l(2147483647)), out()),
		c("CVTLD 2^31-1", "CVTLD", in(l(2147483647)), out()),
		c("CVTRDL -2.5", "CVTRDL", in(d("-2.5")), out()),
		c("CVTRDL low bits", "CVTRDL", in(d("1000000+1/2-2^-30")), out()),
		c("CVTDL reserved", "CVTDL", in(D.reserved()), out()),

		c("ACBF taken", "ACBF", in(f("3")), in(f("1")), mod(f("2"))).br(),
		c("ACBF not taken", "ACBF", in(f("3")), in(f("1")), mod(f("3"))).br(),
		c("ACBF negative step", "ACBF", in(f("1")), in(f("-0.5")), mod(f("1.5"))).br(),
		c("ACBD low bits", "ACBD", in(d("1")), in(d("2^-55")), mod(d("1-2^-55"))).br(),
	}

	cases = append(cases, emodCases(F, "B")...)
	cases = append(cases, emodCases(D, "B")...)
	cases = append(cases, polyCases(F)...)
	cases = append(cases, polyCases(D)...)

	return cases
}

// floatCases are the G_floating or H_floating cases: every instruction
// of the format, and its conversions.
func floatCases(fm floatFormat) []testCase {
	x := fm.letter
	v := fm.encode
	n := func(name string) string { return strings.ReplaceAll(name, "#", x) }
	F, D, G := fFloat, dFloat, gFloat
	half := fm.fbits + 1 // 1 + 2^-half is a tie at 1.0

	tie := fmt.Sprintf("2^-%d", half)
	big := "2^200"
	small := "2^-200"

	if fm == hFloat {
		big, small = "2^2000", "2^-2000"
	}

	cases := []testCase{
		c(n("MOV# 1.5"), "MOV"+x, in(v("1.5")), out()),
		c(n("MOV# -1/3"), "MOV"+x, in(v("-1/3")), out()),
		c(n("MOV# max"), "MOV"+x, in(v("max")), out()),
		c(n("MOV# min"), "MOV"+x, in(v("min")), out()),
		c(n("MOV# zero"), "MOV"+x, in(v("0")), out()),
		c(n("MOV# dirty zero"), "MOV"+x, in(fm.dirtyZero()), out()),
		c(n("MOV# reserved"), "MOV"+x, in(fm.reserved()), out()),
		c(n("MOV# to R6"), "MOV"+x, in(v("1/3")), lit("R6")),
		c(n("MOV# from R2"), "MOV"+x, lit("R2"), out()),
		c(n("MOV# (R2)+"), "MOV"+x, in(cat(v("1"), v("2"))), out()).
			regAddr(2, "D_0").then("SUBL2\t#D_0,R2"),
		c(n("MOV# literal 0.5"), "MOV"+x, lit("#0.5"), out()),
		c(n("MOV# literal 1.5"), "MOV"+x, lit("#1.5"), out()),
		c(n("MOV# literal 120.0"), "MOV"+x, lit("#120.0"), out()),
		c(n("MOV# immediate 1.1"), "MOV"+x, lit("#1.1"), out()),
		c(n("MNEG# 1/3"), "MNEG"+x, in(v("1/3")), out()),
		c(n("MNEG# zero"), "MNEG"+x, in(v("0")), out()),
		c(n("MNEG# reserved"), "MNEG"+x, in(fm.reserved()), out()),
		c(n("TST# negative"), "TST"+x, in(v("-2"))),
		c(n("TST# zero"), "TST"+x, in(v("0"))),
		c(n("TST# dirty zero"), "TST"+x, in(fm.dirtyZero())),
		c(n("TST# reserved"), "TST"+x, in(fm.reserved())),
		c(n("CMP# less"), "CMP"+x, in(v("1")), in(v("2"))),
		c(n("CMP# greater"), "CMP"+x, in(v("2")), in(v("-2"))),
		c(n("CMP# equal"), "CMP"+x, in(v("-1/3")), in(v("-1/3"))),
		c(n("CMP# low bit differs"), "CMP"+x, in(v("1-2^-"+strconv.Itoa(fm.fbits+1))), in(v("1-2^-"+strconv.Itoa(fm.fbits)))),
		c(n("CMP# reserved"), "CMP"+x, in(v("1")), in(fm.reserved())),
		c(n("CMP# literal"), "CMP"+x, lit("#0.5"), in(v("0.5"))),

		c(n("ADD#2 tie"), "ADD"+x+"2", in(v(tie)), mod(v("1"))),
		c(n("ADD#3 negative tie"), "ADD"+x+"3", in(v("-1")), in(v("-"+tie)), out()),
		c(n("ADD#3 1/3 + 1/3"), "ADD"+x+"3", in(v("1/3")), in(v("1/3")), out()),
		c(n("ADD#3 literal"), "ADD"+x+"3", lit("#1.5"), in(v("1/3")), out()),
		c(n("ADD#3 reserved"), "ADD"+x+"3", in(fm.reserved()), in(v("1")), out()),
		c(n("ADD#3 overflow"), "ADD"+x+"3", in(v("max")), in(v("max")), out()),
		c(n("SUB#3 tie"), "SUB"+x+"3", in(v(fmt.Sprintf("2^-%d", half+1))), in(v("1")), out()),
		c(n("SUB#3 cancel"), "SUB"+x+"3", in(v("1/3")), in(v("1/3")), out()),
		c(n("SUB#2 underflow"), "SUB"+x+"2", in(v("min")), mod(v("min*1.5"))),
		c(n("SUB#2 underflow, FU"), "SUB"+x+"2", in(v("min")), mod(v("min*1.5"))).fu(),
		c(n("MUL#3 tie"), "MUL"+x+"3", in(v(mulTie(fm, 0))), in(v(mulTie(fm, 1))), out()),
		c(n("MUL#3 1/3 x 3"), "MUL"+x+"3", in(v("1/3")), in(v("3")), out()),
		c(n("MUL#2 -1.5 x 1.5"), "MUL"+x+"2", in(v("-1.5")), mod(v("1.5"))),
		c(n("MUL#3 overflow"), "MUL"+x+"3", in(v("max")), in(v("2")), out()),
		c(n("MUL#3 underflow"), "MUL"+x+"3", in(v("min")), in(v("0.5")), out()),
		c(n("MUL#3 underflow, FU"), "MUL"+x+"3", in(v("min")), in(v("0.5")), out()).fu(),
		c(n("DIV#3 1/3"), "DIV"+x+"3", in(v("3")), in(v("1")), out()),
		c(n("DIV#3 2/3"), "DIV"+x+"3", in(v("3")), in(v("2")), out()),
		c(n("DIV#3 1/10"), "DIV"+x+"3", in(v("10")), in(v("1")), out()),
		c(n("DIV#2 by zero"), "DIV"+x+"2", in(v("0")), mod(v("1"))),
		c(n("DIV#3 overflow"), "DIV"+x+"3", in(v("0.5")), in(v("max")), out()),
		c(n("DIV#3 underflow, FU"), "DIV"+x+"3", in(v("2")), in(v("min")), out()).fu(),

		c(n("CVT#B -100.7"), "CVT"+x+"B", in(v("-100.7")), out()),
		c(n("CVT#B overflow"), "CVT"+x+"B", in(v("200")), out()),
		c(n("CVT#W -40000"), "CVT"+x+"W", in(v("-40000")), out()),
		c(n("CVT#L 2^31-1"), "CVT"+x+"L", in(v("2147483647.9")), out()),
		c(n("CVT#L overflow"), "CVT"+x+"L", in(v("2^31")), out()),
		c(n("CVT#L overflow, IV"), "CVT"+x+"L", in(v("2^31")), out()).iv(),
		c(n("CVT#L reserved"), "CVT"+x+"L", in(fm.reserved()), out()),
		c(n("CVTR#L 2.5"), "CVTR"+x+"L", in(v("2.5")), out()),
		c(n("CVTR#L -2.5"), "CVTR"+x+"L", in(v("-2.5")), out()),
		c(n("CVTR#L overflow"), "CVTR"+x+"L", in(v("2147483647.5")), out()),
		c(n("CVTB# -128"), "CVTB"+x, in(b(0x80)), out()),
		c(n("CVTW# 32767"), "CVTW"+x, in(w(32767)), out()),
		c(n("CVTL# -2^31"), "CVTL"+x, in(l(-2147483648)), out()),
		c(n("CVTL# 2^31-1"), "CVTL"+x, in(l(2147483647)), out()),
		c(n("CVTL# zero"), "CVTL"+x, in(l(0)), out()),

		c(n("ACB# taken"), "ACB"+x, in(v("3")), in(v("1")), mod(v("2"))).br(),
		c(n("ACB# not taken"), "ACB"+x, in(v("3")), in(v("1")), mod(v("3"))).br(),
		c(n("ACB# negative step"), "ACB"+x, in(v("1")), in(v("-0.5")), mod(v("1.5"))).br(),
		c(n("ACB# literals"), "ACB"+x, lit("#10.0"), lit("#1.0"), mod(v("9"))).br(),
		c(n("ACB# reserved"), "ACB"+x, in(fm.reserved()), in(v("1")), mod(v("1"))).br(),
	}

	// Conversions between formats.
	if fm == gFloat {
		cases = append(cases,
			c("CVTGF 1/3", "CVTGF", in(v("1/3")), out()),
			c("CVTGF tie", "CVTGF", in(v("1+2^-24")), out()),
			c("CVTGF negative tie", "CVTGF", in(v("-1-2^-24")), out()),
			c("CVTGF F's max", "CVTGF", in(v(F.maxExpr())), out()),
			c("CVTGF max rounds over", "CVTGF", in(v(F.maxExpr()+"+2^102")), out()),
			c("CVTGF overflow", "CVTGF", in(v(big)), out()),
			c("CVTGF underflow", "CVTGF", in(v(small)), out()),
			c("CVTGF underflow, FU", "CVTGF", in(v(small)), out()).fu(),
			c("CVTGF reserved", "CVTGF", in(fm.reserved()), out()),
			c("CVTFG 1/3", "CVTFG", in(F.encode("1/3")), out()),
			c("CVTFG max", "CVTFG", in(F.encode("max")), out()),
			c("CVTFG min", "CVTFG", in(F.encode("min")), out()),
			c("CVTFG reserved", "CVTFG", in(F.reserved()), out()),
		)
	} else {
		cases = append(cases,
			c("CVTHF 1/3", "CVTHF", in(v("1/3")), out()),
			c("CVTHF tie", "CVTHF", in(v("1+2^-24")), out()),
			c("CVTHF overflow", "CVTHF", in(v(big)), out()),
			c("CVTHF underflow", "CVTHF", in(v(small)), out()),
			c("CVTHF underflow, FU", "CVTHF", in(v(small)), out()).fu(),
			c("CVTHF reserved", "CVTHF", in(fm.reserved()), out()),
			c("CVTHD 1/3", "CVTHD", in(v("1/3")), out()),
			c("CVTHD tie", "CVTHD", in(v("1+2^-56")), out()),
			c("CVTHD overflow", "CVTHD", in(v(big)), out()),
			c("CVTHD underflow, FU", "CVTHD", in(v(small)), out()).fu(),
			c("CVTHG 1/3", "CVTHG", in(v("1/3")), out()),
			c("CVTHG tie", "CVTHG", in(v("1+2^-53")), out()),
			c("CVTHG negative tie", "CVTHG", in(v("-1-2^-53")), out()),
			c("CVTHG overflow", "CVTHG", in(v(big)), out()),
			c("CVTHG underflow", "CVTHG", in(v(small)), out()),
			c("CVTHG underflow, FU", "CVTHG", in(v(small)), out()).fu(),
			c("CVTHG reserved", "CVTHG", in(fm.reserved()), out()),
			c("CVTFH 1/3", "CVTFH", in(F.encode("1/3")), out()),
			c("CVTFH max", "CVTFH", in(F.encode("max")), out()),
			c("CVTFH reserved", "CVTFH", in(F.reserved()), out()),
			c("CVTDH low bits", "CVTDH", in(D.encode("1-2^-56")), out()),
			c("CVTDH reserved", "CVTDH", in(D.reserved()), out()),
			c("CVTGH 1/3", "CVTGH", in(G.encode("1/3")), out()),
			c("CVTGH max", "CVTGH", in(G.encode("max")), out()),
			c("CVTGH to R6", "CVTGH", in(G.encode("-1/3")), lit("R6")),
			c("CVTGH reserved", "CVTGH", in(G.reserved()), out()),
		)
	}

	cases = append(cases, emodCases(fm, "W")...)
	cases = append(cases, polyCases(fm)...)

	return cases
}

// maxExpr is the format's largest value as an expression any format with
// a wider fraction can hold exactly.
func (f floatFormat) maxExpr() string {
	return fmt.Sprintf("2^%d-2^%d", (1<<f.ebits)-1-f.bias, (1<<f.ebits)-1-f.bias-f.fbits-1)
}

// mulTie returns the factors (1+2^-a)(1+2^-b) whose product is exactly
// half way between two of the format's values just above 1.
func mulTie(f floatFormat, which int) string {
	half := f.fbits + 1 // 2^-half is half a unit in the last place at 1.0
	a := half / 2
	bb := half - a

	if which == 0 {
		return fmt.Sprintf("1+2^-%d", a)
	}

	return fmt.Sprintf("1+2^-%d", bb)
}

// emodCases are one format's EMOD cases. ext is the extension operand's
// size: a byte for F and D, a word for G and H.
func emodCases(fm floatFormat, ext string) []testCase {
	x := fm.letter
	v := fm.encode
	name := func(s string) string { return "EMOD" + x + " " + s }
	insn := "EMOD" + x
	e := func(n int64) op {
		if ext == "B" {
			return in(b(byte(n)))
		}

		return in(w(n))
	}

	return []testCase{
		c(name("1.5 x 2"), insn, in(v("1.5")), e(0), in(v("2")), out(), out()),
		c(name("pi x 10"), insn, in(v("3.14159265358979323846264338327950288")), e(0), in(v("10")), out(), out()),
		c(name("-2.5 x 1.5"), insn, in(v("-2.5")), e(0), in(v("1.5")), out(), out()),
		c(name("0.75 x 0.5"), insn, in(v("0.75")), e(0), in(v("0.5")), out(), out()),
		c(name("1/3 x 3"), insn, in(v("1/3")), e(0), in(v("3")), out(), out()),
		c(name("1/3 x 3, ext all ones"), insn, in(v("1/3")), e(-1), in(v("3")), out(), out()),
		c(name("1/3 x 3, ext high bit"), insn, in(v("1/3")), e(0x80), in(v("3")), out(), out()),
		c(name("1/3 x 3, ext low bit"), insn, in(v("1/3")), e(1), in(v("3")), out(), out()),
		c(name("small fraction"), insn, in(v(fmt.Sprintf("1+2^-%d", fm.fbits))), e(0), in(v("1000")), out(), out()),
		c(name("int overflow"), insn, in(v("2^40+0.25")), e(0), in(v("1")), out(), out()),
		c(name("int overflow, IV"), insn, in(v("2^40+0.25")), e(0), in(v("1")), out(), out()).iv(),
		c(name("-2^31"), insn, in(v("-2^31")), e(0), in(v("1")), out(), out()),
		c(name("zero"), insn, in(v("0")), e(-1), in(v("5")), out(), out()),
		c(name("underflow"), insn, in(v("min")), e(0), in(v("0.5")), out(), out()),
		c(name("underflow, FU"), insn, in(v("min")), e(0), in(v("0.5")), out(), out()).fu(),
		c(name("reserved multiplier"), insn, in(fm.reserved()), e(0), in(v("1")), out(), out()),
		c(name("reserved multiplicand"), insn, in(v("1")), e(0), in(fm.reserved()), out(), out()),
	}
}

// polyCases are one format's POLY cases. The table lists the
// coefficients from the highest degree's down to the constant.
func polyCases(fm floatFormat) []testCase {
	x := fm.letter
	v := fm.encode
	name := func(s string) string { return "POLY" + x + " " + s }
	insn := "POLY" + x
	table := func(exprs ...string) op {
		var t []byte
		for _, e := range exprs {
			t = append(t, v(e)...)
		}

		return in(t)
	}

	ones := make([]string, 32)
	for i := range ones {
		ones[i] = "1"
	}

	return []testCase{
		c(name("degree 0"), insn, in(v("2")), length(0), table("5")),
		c(name("2x^2+... at 2"), insn, in(v("2")), length(2), table("1", "2", "3")),
		c(name("degree 3 at 0.5"), insn, in(v("0.5")), length(3), table("1", "-2", "3", "-4")),
		c(name("degree 2 at -1.5"), insn, in(v("-1.5")), length(2), table("1", "1", "1")),
		c(name("degree 1 at 1/3"), insn, in(v("1/3")), length(1), table("3", "-1")),
		c(name("degree 2 at 1/3"), insn, in(v("1/3")), length(2), table("1/7", "1/5", "1/3")),
		c(name("degree 31"), insn, in(v("0.5")), length(31), table(ones...)),
		c(name("degree 32"), insn, in(v("0.5")), length(32), table(ones...)),
		c(name("literal argument"), insn, lit("#0.5"), length(2), table("1", "2", "3")),
		c(name("reserved argument"), insn, in(fm.reserved()), length(1), table("1", "1")),
		c(name("reserved coefficient"), insn, in(v("1")), length(2), in(cat(v("1"), fm.reserved(), v("1")))),
		c(name("overflow"), insn, in(v("max")), length(1), table("2", "0")),
		c(name("underflow"), insn, in(v("min")), length(1), table("0.5", "0")),
		c(name("underflow, FU"), insn, in(v("min")), length(1), table("0.5", "0")).fu(),
	}
}

// ---------------------------------------------------------------------
// The octaword cases.

func octaCases() []testCase {
	o := func(hi, lo uint64) []byte {
		out := make([]byte, 16)
		for i := 0; i < 8; i++ {
			out[i] = byte(lo >> (8 * i))
			out[8+i] = byte(hi >> (8 * i))
		}

		return out
	}
	pattern := o(0x0F0E0D0C0B0A0908, 0x0706050403020100)
	second := o(0x1F1E1D1C1B1A1918, 0x1716151413121110)

	return []testCase{
		c("CLRO", "CLRO", out()),
		c("CLRO R6", "CLRO", lit("R6")),
		c("CLRO, CC clear", "CLRO", out()).withCC(0),
		c("MOVO pattern", "MOVO", in(pattern), out()),
		c("MOVO pattern, CC clear", "MOVO", in(pattern), out()).withCC(0),
		c("MOVO zero", "MOVO", in(o(0, 0)), out()),
		c("MOVO negative", "MOVO", in(o(0x8000000000000000, 0)), out()),
		c("MOVO low bit", "MOVO", in(o(0, 1)), out()),
		c("MOVO bit 64", "MOVO", in(o(1, 0)), out()),
		c("MOVO to R6", "MOVO", in(pattern), lit("R6")),
		c("MOVO from R2", "MOVO", lit("R2"), out()),
		c("MOVO R2 to R6", "MOVO", lit("R2"), lit("R6")),
		c("MOVO R6 to R4 (overlap)", "MOVO", lit("R6"), lit("R4")),
		c("MOVO to (R2)+", "MOVO", in(pattern), lit("(R2)+")).
			regAddr(2, "DST").then("SUBL2\t#DST,R2"),
		c("MOVO to -(R2)", "MOVO", in(second), lit("-(R2)")).
			regAddr(2, "DST+32").then("SUBL2\t#DST,R2"),
		c("MOVO (R2)+ source", "MOVO", lit("(R2)+"), out()).
			regAddr(2, "D_X").then("SUBL2\t#D_X,R2"),
		c("MOVO -(R2) source", "MOVO", lit("-(R2)"), out()).
			regAddr(2, "D_X+32").then("SUBL2\t#D_X,R2"),
		c("MOVO indexed", "MOVO", lit("D_X[R4]"), out()).reg(4, 1),
		c("MOVO to indexed", "MOVO", in(pattern), lit("DST[R4]")).reg(4, 1),
		c("MOVO literal", "MOVO", lit("#1"), out()),
		c("MOVO literal 63", "MOVO", lit("#63"), out()),
		c("MOVO immediate -1", "MOVO", lit("#-1"), out()),
		c("MOVO immediate ^X12345678", "MOVO", lit("#^X12345678"), out()),
		c("MOVO immediate ^X80000000", "MOVO", lit("#^X80000000"), out()),
		c("MOVAO indexed", "MOVAO", lit("D_X[R4]"), out()).reg(4, 2).
			then("SUBL2\t#D_X,DST"),
		c("MOVAO (R2)+", "MOVAO", lit("(R2)+"), out()).
			regAddr(2, "D_X").then("SUBL2\t#D_X,DST", "SUBL2\t#D_X,R2"),
		c("PUSHAO indexed", "PUSHAO", lit("D_X[R4]")).reg(4, 3).
			then("SUBL3\t#D_X,(SP)+,DST"),
		// R12 is AP: an octaword from R12 would span AP, FP, SP, and PC.
		// The manual leaves it UNPREDICTABLE; govax faults. Written as
		// bytes, MOVO R12,R6, so no assembler has to accept it.
		rawCase("MOVO R12 to R6", ".BYTE\t^XFD,^X7D,^X5C,^X56\t; MOVO R12,R6"),
	}
}

// ---------------------------------------------------------------------
// The packed decimal cases.

// p returns a packed decimal string of n digits: s is a sign and digits
// ("+123", "-0"), the sign written as C or D, the preferred codes.
func p(s string, n int) []byte {
	sign := byte(0xC)
	if s[0] == '-' {
		sign = 0xD
	}

	return ps(strings.TrimLeft(s, "+-"), n, sign)
}

// ps returns a packed decimal string of n digits with the sign nibble
// sign, so a case can use the other sign codes. A digit given as a
// letter A-F is that nibble, for invalid-digit cases.
func ps(digits string, n int, sign byte) []byte {
	if len(digits) > n {
		panic(digits)
	}

	digits = strings.Repeat("0", n-len(digits)) + digits
	nibbles := make([]byte, 0, n+2)

	if n%2 == 0 {
		nibbles = append(nibbles, 0)
	}

	for _, ch := range digits {
		v, err := strconv.ParseUint(string(ch), 16, 8)
		if err != nil {
			panic(digits)
		}

		nibbles = append(nibbles, byte(v))
	}

	nibbles = append(nibbles, sign)
	out := make([]byte, len(nibbles)/2)

	for i := range out {
		out[i] = nibbles[2*i]<<4 | nibbles[2*i+1]
	}

	return out
}

// pt returns a packed decimal string as bytes, for cases with a nonzero
// unused high nibble.
func pt(v ...byte) []byte { return v }

// nines returns n nines.
func nines(n int) string { return strings.Repeat("9", n) }

// Pattern operators for EDITPC (the manual's EO$ operators).
const (
	eoEnd         = 0x00
	eoEndFloat    = 0x01
	eoClearSignif = 0x02
	eoSetSignif   = 0x03
	eoStoreSign   = 0x04
	eoLoadFill    = 0x40
	eoLoadSign    = 0x41
	eoLoadPlus    = 0x42
	eoLoadMinus   = 0x43
	eoInsert      = 0x44
	eoBlankZero   = 0x45
	eoReplaceSign = 0x46
	eoAdjustInput = 0x47
	eoFill        = 0x80
	eoMove        = 0x90
	eoFloat       = 0xA0
)

// cvtptTable translates a packed string's last byte (its last digit and
// sign) to the trailing numeric string's last character: an overpunched
// sign, {ABCDEFGHI for plus and }JKLMNOPQR for minus.
func cvtptTable() []byte {
	t := make([]byte, 256)
	for i := range t {
		t[i] = '?'
	}

	for d := 0; d < 10; d++ {
		for _, s := range []int{0xA, 0xC, 0xE, 0xF} {
			t[d<<4|s] = "{ABCDEFGHI"[d]
		}

		for _, s := range []int{0xB, 0xD} {
			t[d<<4|s] = "}JKLMNOPQR"[d]
		}
	}

	return t
}

// cvttpTable translates a trailing numeric string's last character to a
// packed byte, the reverse of cvtptTable; a plain digit is plus. Other
// characters give ^XFF, an invalid digit and sign.
func cvttpTable() []byte {
	t := make([]byte, 256)
	for i := range t {
		t[i] = 0xFF
	}

	for d := 0; d < 10; d++ {
		t['0'+d] = byte(d<<4 | 0xC)
		t["{ABCDEFGHI"[d]] = byte(d<<4 | 0xC)
		t["}JKLMNOPQR"[d]] = byte(d<<4 | 0xD)
	}

	return t
}

func packedCases() []testCase {
	var cases []testCase
	add := func(t ...testCase) { cases = append(cases, t...) }

	// MOVP len, src, dst
	add(
		c("MOVP +123", "MOVP", length(3), in(p("+123", 3)), out()),
		c("MOVP -123", "MOVP", length(3), in(p("-123", 3)), out()),
		c("MOVP -0", "MOVP", length(3), in(p("-0", 3)), out()),
		c("MOVP +0, CC clear", "MOVP", length(3), in(p("+0", 3)), out()).withCC(0),
		c("MOVP even length", "MOVP", length(4), in(p("-1234", 4)), out()),
		c("MOVP sign A", "MOVP", length(3), in(ps("123", 3, 0xA)), out()),
		c("MOVP sign B", "MOVP", length(3), in(ps("123", 3, 0xB)), out()),
		c("MOVP sign E", "MOVP", length(3), in(ps("123", 3, 0xE)), out()),
		c("MOVP sign F", "MOVP", length(3), in(ps("123", 3, 0xF)), out()),
		c("MOVP bad digit", "MOVP", length(3), in(ps("1A3", 3, 0xC)), out()),
		c("MOVP bad sign", "MOVP", length(3), in(ps("123", 3, 0x9)), out()),
		c("MOVP even length, high nibble", "MOVP", length(2), in(pt(0x51, 0x2C)), out()),
		c("MOVP length 0", "MOVP", length(0), in(pt(0x0D)), out()),
		c("MOVP length 31", "MOVP", length(31), in(p("-"+nines(31), 31)), out()),
		c("MOVP length 32", "MOVP", length(32), in(p("+1", 31)), out()),
	)

	// CMPP3 len, src1, src2; CMPP4 len1, src1, len2, src2
	add(
		c("CMPP3 equal", "CMPP3", length(3), in(p("+123", 3)), in(p("+123", 3))),
		c("CMPP3 less", "CMPP3", length(3), in(p("+122", 3)), in(p("+123", 3))),
		c("CMPP3 greater", "CMPP3", length(3), in(p("+124", 3)), in(p("+123", 3))),
		c("CMPP3 negative less", "CMPP3", length(3), in(p("-124", 3)), in(p("-123", 3))),
		c("CMPP3 -0 vs +0", "CMPP3", length(3), in(p("-0", 3)), in(p("+0", 3))),
		c("CMPP3 sign F vs C", "CMPP3", length(3), in(ps("5", 3, 0xF)), in(p("+5", 3))),
		c("CMPP3 bad digit", "CMPP3", length(3), in(ps("12B", 3, 0xC)), in(p("+123", 3))),
		c("CMPP4 different lengths", "CMPP4", length(1), in(p("+7", 1)), length(5), in(p("+7", 5))),
		c("CMPP4 less", "CMPP4", length(5), in(p("-12345", 5)), length(1), in(p("+0", 1))),
		c("CMPP4 greater", "CMPP4", length(31), in(p("+"+nines(31), 31)), length(30), in(p("+"+nines(30), 30))),
		c("CMPP4 length 32", "CMPP4", length(32), in(p("+1", 31)), length(1), in(p("+1", 1))),
	)

	// CVTLP src, dstlen, dstaddr; CVTPL srclen, srcaddr, dst
	add(
		c("CVTLP 0", "CVTLP", in(l(0)), length(3), out()),
		c("CVTLP 123", "CVTLP", in(l(123)), length(3), out()),
		c("CVTLP -123", "CVTLP", in(l(-123)), length(4), out()),
		c("CVTLP 2^31-1", "CVTLP", in(l(2147483647)), length(10), out()),
		c("CVTLP -2^31", "CVTLP", in(l(-2147483648)), length(10), out()),
		c("CVTLP overflow", "CVTLP", in(l(12345)), length(3), out()),
		c("CVTLP overflow, DV", "CVTLP", in(l(-12345)), length(3), out()).dv(),
		c("CVTLP length 0", "CVTLP", in(l(0)), length(0), out()),
		c("CVTLP length 0 overflow", "CVTLP", in(l(1)), length(0), out()),
		c("CVTLP literal", "CVTLP", lit("#42"), length(2), out()),
		c("CVTPL 123", "CVTPL", length(3), in(p("+123", 3)), out()),
		c("CVTPL -123", "CVTPL", length(3), in(p("-123", 3)), out()),
		c("CVTPL -0", "CVTPL", length(3), in(p("-0", 3)), out()),
		c("CVTPL 2^31-1", "CVTPL", length(10), in(p("+2147483647", 10)), out()),
		c("CVTPL -2^31", "CVTPL", length(10), in(p("-2147483648", 10)), out()),
		c("CVTPL overflow", "CVTPL", length(10), in(p("+2147483648", 10)), out()),
		c("CVTPL overflow, IV", "CVTPL", length(10), in(p("+2147483648", 10)), out()).iv(),
		c("CVTPL 31 digits", "CVTPL", length(31), in(p("+1"+strings.Repeat("0", 30), 31)), out()),
		c("CVTPL to R6", "CVTPL", length(3), in(p("-456", 3)), lit("R6")),
		c("CVTPL to R1", "CVTPL", length(3), in(p("-456", 3)), lit("R1")),
		c("CVTPL bad digit", "CVTPL", length(3), in(ps("4C6", 3, 0xC)), out()),
	)

	// ADDP4 addlen, addaddr, sumlen, sumaddr
	add(
		c("ADDP4 12+34", "ADDP4", length(2), in(p("+12", 2)), length(3), mod(p("+34", 3))),
		c("ADDP4 carry", "ADDP4", length(3), in(p("+999", 3)), length(5), mod(p("+1", 5))),
		c("ADDP4 mixed signs", "ADDP4", length(3), in(p("-500", 3)), length(3), mod(p("+123", 3))),
		c("ADDP4 to -0", "ADDP4", length(3), in(p("-123", 3)), length(3), mod(p("+123", 3))),
		c("ADDP4 negatives", "ADDP4", length(3), in(p("-123", 3)), length(3), mod(p("-877", 3))),
		c("ADDP4 overflow", "ADDP4", length(3), in(p("+999", 3)), length(3), mod(p("+1", 3))),
		c("ADDP4 overflow, DV", "ADDP4", length(3), in(p("+999", 3)), length(3), mod(p("+1", 3))).dv(),
		c("ADDP4 31 digits", "ADDP4", length(31), in(p("+"+nines(31), 31)), length(31), mod(p("-1", 31))),
		c("ADDP4 sign F", "ADDP4", length(1), in(ps("1", 1, 0xF)), length(1), mod(ps("2", 1, 0xA))),
		c("ADDP4 bad digit", "ADDP4", length(1), in(ps("A", 1, 0xC)), length(1), mod(p("+2", 1))),
		c("ADDP4 bad sign", "ADDP4", length(1), in(ps("1", 1, 0x3)), length(1), mod(p("+2", 1))),
		c("ADDP4 length 0", "ADDP4", length(0), in(pt(0x0C)), length(3), mod(p("-5", 3))),
	)

	// ADDP6 add1len, add1addr, add2len, add2addr, sumlen, sumaddr
	add(
		c("ADDP6 123+877", "ADDP6", length(3), in(p("+123", 3)), length(3), in(p("+877", 3)), length(4), out()),
		c("ADDP6 short sum", "ADDP6", length(3), in(p("+123", 3)), length(3), in(p("+877", 3)), length(3), out()),
		c("ADDP6 short sum, DV", "ADDP6", length(3), in(p("+123", 3)), length(3), in(p("+877", 3)), length(3), out()).dv(),
		c("ADDP6 -5 + +5", "ADDP6", length(1), in(p("-5", 1)), length(1), in(p("+5", 1)), length(1), out()),
		c("ADDP6 mixed lengths", "ADDP6", length(7), in(p("-1234567", 7)), length(2), in(p("+89", 2)), length(9), out()),
		c("ADDP6 length 0 sum", "ADDP6", length(1), in(p("+0", 1)), length(1), in(p("-0", 1)), length(0), out()),
		c("ADDP6 sum length 32", "ADDP6", length(1), in(p("+1", 1)), length(1), in(p("+1", 1)), length(32), out()),
	)

	// SUBP4 sublen, subaddr, diflen, difaddr: dif = dif - sub
	// SUBP6 sublen, subaddr, minlen, minaddr, diflen, difaddr: dif = min - sub
	add(
		c("SUBP4 50-8", "SUBP4", length(1), in(p("+8", 1)), length(2), mod(p("+50", 2))),
		c("SUBP4 below zero", "SUBP4", length(2), in(p("+51", 2)), length(2), mod(p("+50", 2))),
		c("SUBP4 to zero", "SUBP4", length(2), in(p("-50", 2)), length(2), mod(p("-50", 2))),
		c("SUBP4 overflow", "SUBP4", length(2), in(p("-50", 2)), length(2), mod(p("+50", 2))),
		c("SUBP6 100-1", "SUBP6", length(1), in(p("+1", 1)), length(3), in(p("+100", 3)), length(3), out()),
		c("SUBP6 -1-100", "SUBP6", length(3), in(p("+100", 3)), length(1), in(p("-1", 1)), length(5), out()),
		c("SUBP6 overflow, DV", "SUBP6", length(3), in(p("-999", 3)), length(3), in(p("+999", 3)), length(3), out()).dv(),
	)

	// MULP mulrlen, mulraddr, muldlen, muldaddr, prodlen, prodaddr
	add(
		c("MULP 12x12", "MULP", length(2), in(p("+12", 2)), length(2), in(p("+12", 2)), length(4), out()),
		c("MULP -12x12", "MULP", length(2), in(p("-12", 2)), length(2), in(p("+12", 2)), length(4), out()),
		c("MULP -12x-12", "MULP", length(2), in(p("-12", 2)), length(2), in(p("-12", 2)), length(4), out()),
		c("MULP by zero", "MULP", length(2), in(p("-12", 2)), length(1), in(p("+0", 1)), length(3), out()),
		c("MULP 15x15 digits", "MULP", length(15), in(p("+"+nines(15), 15)), length(15), in(p("+"+nines(15), 15)), length(30), out()),
		c("MULP overflow", "MULP", length(3), in(p("+999", 3)), length(3), in(p("+999", 3)), length(5), out()),
		c("MULP overflow, DV", "MULP", length(3), in(p("+999", 3)), length(3), in(p("-999", 3)), length(5), out()).dv(),
		c("MULP 31 digits", "MULP", length(1), in(p("+2", 1)), length(31), in(p("+4"+nines(30), 31)), length(31), out()),
	)

	// DIVP divrlen, divraddr, divdlen, divdaddr, quolen, quoaddr: quo = divd / divr
	add(
		c("DIVP 100/7", "DIVP", length(1), in(p("+7", 1)), length(3), in(p("+100", 3)), length(3), out()),
		c("DIVP -100/7", "DIVP", length(1), in(p("+7", 1)), length(3), in(p("-100", 3)), length(3), out()),
		c("DIVP 100/-7", "DIVP", length(1), in(p("-7", 1)), length(3), in(p("+100", 3)), length(3), out()),
		c("DIVP 6/7", "DIVP", length(1), in(p("+7", 1)), length(1), in(p("-6", 1)), length(3), out()),
		c("DIVP by zero", "DIVP", length(1), in(p("+0", 1)), length(3), in(p("+100", 3)), length(3), out()),
		c("DIVP by -0", "DIVP", length(1), in(p("-0", 1)), length(3), in(p("+100", 3)), length(3), out()),
		c("DIVP overflow", "DIVP", length(1), in(p("+1", 1)), length(3), in(p("+100", 3)), length(2), out()),
		c("DIVP overflow, DV", "DIVP", length(1), in(p("+1", 1)), length(3), in(p("+100", 3)), length(2), out()).dv(),
		c("DIVP 31 digits", "DIVP", length(3), in(p("+997", 3)), length(31), in(p("+"+nines(31), 31)), length(31), out()),
		c("DIVP bad divisor digit", "DIVP", length(1), in(ps("E", 1, 0xC)), length(3), in(p("+100", 3)), length(3), out()),
	)

	// ASHP cnt, srclen, srcaddr, round, dstlen, dstaddr
	add(
		c("ASHP left 2", "ASHP", lit("#2"), length(3), in(p("+123", 3)), lit("#0"), length(5), out()),
		c("ASHP right 2", "ASHP", lit("#-2"), length(5), in(p("+12345", 5)), lit("#0"), length(5), out()),
		c("ASHP right 2, round 5", "ASHP", lit("#-2"), length(5), in(p("+12350", 5)), lit("#5"), length(5), out()),
		c("ASHP right 2, round 5, neg", "ASHP", lit("#-2"), length(5), in(p("-12350", 5)), lit("#5"), length(5), out()),
		c("ASHP right 2, round 5, below", "ASHP", lit("#-2"), length(5), in(p("+12349", 5)), lit("#5"), length(5), out()),
		c("ASHP right 1, round 9", "ASHP", lit("#-1"), length(3), in(p("+121", 3)), lit("#9"), length(3), out()),
		c("ASHP round 12", "ASHP", lit("#-1"), length(3), in(p("+125", 3)), lit("#12"), length(3), out()),
		c("ASHP left overflow", "ASHP", lit("#3"), length(3), in(p("+123", 3)), lit("#0"), length(5), out()),
		c("ASHP left overflow, DV", "ASHP", lit("#3"), length(3), in(p("+123", 3)), lit("#0"), length(5), out()).dv(),
		c("ASHP right all digits", "ASHP", lit("#-31"), length(3), in(p("+123", 3)), lit("#5"), length(3), out()),
		c("ASHP to zero is +0", "ASHP", lit("#-3"), length(3), in(p("-123", 3)), lit("#0"), length(3), out()),
		c("ASHP shorter dst", "ASHP", lit("#0"), length(5), in(p("+00123", 5)), lit("#0"), length(3), out()),
		c("ASHP count from memory", "ASHP", in(b(0xFE)), length(5), in(p("+12345", 5)), lit("#5"), length(5), out()),
	)

	// CVTPS srclen, srcaddr, dstlen, dstaddr; CVTSP srclen, srcaddr, dstlen, dstaddr
	add(
		c("CVTPS +123", "CVTPS", length(3), in(p("+123", 3)), length(5), out()),
		c("CVTPS -123", "CVTPS", length(3), in(p("-123", 3)), length(3), out()),
		c("CVTPS -0", "CVTPS", length(3), in(p("-0", 3)), length(3), out()),
		c("CVTPS overflow", "CVTPS", length(3), in(p("-123", 3)), length(2), out()),
		c("CVTPS overflow, DV", "CVTPS", length(3), in(p("-123", 3)), length(2), out()).dv(),
		c("CVTPS length 0", "CVTPS", length(3), in(p("+0", 3)), length(0), out()),
		c("CVTSP +123", "CVTSP", length(3), in([]byte("+123")), length(3), out()),
		c("CVTSP -123", "CVTSP", length(3), in([]byte("-123")), length(5), out()),
		c("CVTSP blank sign", "CVTSP", length(3), in([]byte(" 123")), length(3), out()),
		c("CVTSP -0", "CVTSP", length(2), in([]byte("-00")), length(2), out()),
		c("CVTSP bad sign", "CVTSP", length(3), in([]byte("*123")), length(3), out()),
		c("CVTSP bad digit", "CVTSP", length(3), in([]byte("+1X3")), length(3), out()),
		c("CVTSP overflow", "CVTSP", length(4), in([]byte("+1234")), length(3), out()),
		c("CVTSP length 0", "CVTSP", length(0), in([]byte("-")), length(1), out()),
	)

	// CVTPT srclen, srcaddr, tbladdr, dstlen, dstaddr
	// CVTTP srclen, srcaddr, tbladdr, dstlen, dstaddr
	ptTable := in(cvtptTable())
	tpTable := in(cvttpTable())
	add(
		c("CVTPT +123", "CVTPT", length(3), in(p("+123", 3)), ptTable, length(3), out()),
		c("CVTPT -123", "CVTPT", length(3), in(p("-123", 3)), ptTable, length(5), out()),
		c("CVTPT -0", "CVTPT", length(3), in(p("-0", 3)), ptTable, length(3), out()),
		c("CVTPT even length", "CVTPT", length(4), in(p("-1234", 4)), ptTable, length(4), out()),
		c("CVTPT overflow", "CVTPT", length(3), in(p("+123", 3)), ptTable, length(2), out()),
		c("CVTPT overflow, DV", "CVTPT", length(3), in(p("+123", 3)), ptTable, length(2), out()).dv(),
		c("CVTPT length 0", "CVTPT", length(1), in(p("+0", 1)), ptTable, length(0), out()),
		c("CVTTP 12C", "CVTTP", length(3), in([]byte("12C")), tpTable, length(3), out()),
		c("CVTTP 12L", "CVTTP", length(3), in([]byte("12L")), tpTable, length(3), out()),
		c("CVTTP plain digit", "CVTTP", length(3), in([]byte("123")), tpTable, length(5), out()),
		c("CVTTP -0", "CVTTP", length(2), in([]byte("0}")), tpTable, length(2), out()),
		c("CVTTP bad table entry", "CVTTP", length(3), in([]byte("12*")), tpTable, length(3), out()),
		c("CVTTP bad digit", "CVTTP", length(3), in([]byte("1*3")), tpTable, length(3), out()),
		c("CVTTP overflow", "CVTTP", length(4), in([]byte("123D")), tpTable, length(3), out()),
	)

	// EDITPC srclen, srcaddr, pattern, dstaddr
	floatSign := in(b(eoFloat|3, eoInsert, ',', eoFloat|2, eoEndFloat, eoMove|1, eoInsert, '.', eoMove|2, eoEnd))
	check := in(b(eoLoadFill, '*', eoMove|3, eoInsert, ',', eoMove|1, eoSetSignif, eoMove|1, eoInsert, '.', eoMove|2, eoEnd))
	crdb := in(b(eoMove|5, eoLoadPlus, ' ', eoLoadMinus, '-', eoStoreSign, eoInsert, 'C', eoInsert, 'R', eoEnd))
	add(
		c("EDITPC float sign +", "EDITPC", length(8), in(p("+00012345", 8)), floatSign, out()),
		c("EDITPC float sign -", "EDITPC", length(8), in(p("-12345678", 8)), floatSign, out()),
		c("EDITPC float sign 0", "EDITPC", length(8), in(p("+0", 8)), floatSign, out()),
		c("EDITPC check protect", "EDITPC", length(7), in(p("+0001234", 7)), check, out()),
		c("EDITPC check protect 0", "EDITPC", length(7), in(p("+0", 7)), check, out()),
		c("EDITPC CR -", "EDITPC", length(5), in(p("-120", 5)), crdb, out()),
		c("EDITPC CR +", "EDITPC", length(5), in(p("+120", 5)), crdb, out()),
		c("EDITPC blank zero", "EDITPC", length(5), in(p("+0", 5)),
			in(b(eoMove|5, eoBlankZero, 5, eoEnd)), out()),
		c("EDITPC blank zero, nonzero", "EDITPC", length(5), in(p("+7", 5)),
			in(b(eoMove|5, eoBlankZero, 5, eoEnd)), out()),
		c("EDITPC replace sign -0", "EDITPC", length(3), in(p("-0", 3)),
			in(b(eoMove|3, eoReplaceSign, 3, eoEnd)), out()),
		c("EDITPC fill and signif", "EDITPC", length(4), in(p("+0012", 4)),
			in(b(eoLoadFill, '#', eoFill|2, eoMove|2, eoClearSignif, eoMove|1, eoSetSignif, eoMove|1, eoEnd)), out()),
		c("EDITPC load sign", "EDITPC", length(3), in(p("-5", 3)),
			in(b(eoLoadSign, '<', eoFloat|3, eoEndFloat, eoEnd)), out()),
		c("EDITPC store sign first", "EDITPC", length(3), in(p("+5", 3)),
			in(b(eoStoreSign, eoMove|3, eoEnd)), out()),
		c("EDITPC adjust input, pad", "EDITPC", length(3), in(p("+123", 3)),
			in(b(eoAdjustInput, 5, eoMove|5, eoEnd)), out()),
		c("EDITPC adjust input, drop", "EDITPC", length(5), in(p("+00123", 5)),
			in(b(eoAdjustInput, 3, eoMove|3, eoEnd)), out()),
		c("EDITPC adjust input, overflow", "EDITPC", length(5), in(p("+12345", 5)),
			in(b(eoAdjustInput, 3, eoMove|3, eoEnd)), out()),
		c("EDITPC adjust input, DV", "EDITPC", length(5), in(p("+12345", 5)),
			in(b(eoAdjustInput, 3, eoMove|3, eoEnd)), out()).dv(),
		c("EDITPC digits left over", "EDITPC", length(5), in(p("+12345", 5)),
			in(b(eoMove|3, eoEnd)), out()),
		c("EDITPC digits run out", "EDITPC", length(2), in(p("+12", 2)),
			in(b(eoMove|3, eoEnd)), out()),
		c("EDITPC reserved operator", "EDITPC", length(1), in(p("+1", 1)),
			in(b(eoMove|1, 0x3F, eoEnd)), out()),
		c("EDITPC length 32", "EDITPC", length(32), in(p("+1", 31)),
			in(b(eoMove|1, eoEnd)), out()),
		c("EDITPC bad digit", "EDITPC", length(3), in(ps("1B3", 3, 0xC)),
			in(b(eoMove|3, eoEnd)), out()),
	)

	return cases
}

// ---------------------------------------------------------------------
// The programs.

// registerPattern is R0-R11's value before a case's instruction, unless
// the case sets one: ^X5A5A5Ann, nn the register's number.
const registerPattern = 0x5A5A5A00

func probe(s section) string {
	var data, main, routines, loads strings.Builder

	for r := 0; r < 12; r++ {
		fmt.Fprintf(&loads, "\tMOVL\t#^X%08X,R%d\n", registerPattern|r, r)
	}

	// D_X is shared data for the octaword cases that address memory
	// through a register: two octawords and a third for the indexed
	// cases' offsets.
	fmt.Fprintf(&data, "D_X:\t.LONG\t^X03020100,^X07060504,^X0B0A0908,^X0F0E0D0C\n")
	fmt.Fprintf(&data, "\t.LONG\t^X13121110,^X17161514,^X1B1A1918,^X1F1E1D1C\n")
	fmt.Fprintf(&data, "\t.LONG\t^X23222120,^X27262524,^X2B2A2928,^X2F2E2D2C\n")
	fmt.Fprintf(&data, "\t.LONG\t^X33323130,^X37363534,^X3B3A3938,^X3F3E3D3C\n")

	for i, t := range s.cases {
		n := i + 1

		fmt.Fprintf(&data, "; %d: %s\n", n, t.name)
		fmt.Fprintf(&data, "N%d:\t.ASCII\t|%-32s|\n", n, t.name)

		fmt.Fprintf(&main, "; %d: %s\n", n, t.name)
		fmt.Fprintf(&main, "\tMOVL\t#%d,STEP\n\tMOVC3\t#32,N%d,NAME\n\tJSB\tSETUP\n\tCALLS\t#0,C%d\n\tJSB\tSAVE\n", n, n, n)

		routines.WriteString(caseRoutine(&data, n, t))
	}

	return fmt.Sprintf(`	.TITLE	%[1]s	Phase 35 %[2]s probe
	.IDENT	/V1.0/

; Written by testdata/insn35/gen.go: do not edit. Runs each case's
; instruction and writes, per case, its name, whether it completed, the
; PSL after it, R0-R11, any signal array, and DST to %[1]s.DMP. See
; gen.go for the record's layout.

	$SSDEF

	.PSECT	DATA,NOEXE,WRT,LONG
OFAB:	$FAB	FNM=<%[1]s.DMP>,FAC=PUT,RFM=VAR,MRS=512
ORAB:	$RAB	FAB=OFAB,RBF=REC,RSZ=260
	.ALIGN	LONG
REC:	.ASCII	/I35R/
STEP:	.LONG	0
NAME:	.BLKB	32
FLAGS:	.LONG	0
PSLV:	.LONG	0
REGV:	.BLKL	12
SIGV:	.BLKL	9
DST:	.BLKB	128
	.ALIGN	LONG
%[3]s
	.PSECT	CODE,EXE,NOWRT,LONG
	.ENTRY	%[1]s,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	$CREATE	FAB=OFAB
	BLBS	R0,1$
	RET
1$:	$CONNECT RAB=ORAB
	BLBS	R0,2$
	RET
2$:
%[4]s	$CLOSE	FAB=OFAB
	MOVL	#1,R0
	RET

; SETUP clears a case's results and fills DST with ^XAA.
SETUP:	MOVC5	#0,(SP),#0,#<4*(1+1+12+9)>,FLAGS
	MOVC5	#0,(SP),#^XAA,#128,DST
	RSB

; SAVE writes the case's record.
SAVE:	$PUT	RAB=ORAB
	RSB

; LOADREGS sets R0-R11 to the register pattern.
LOADREGS:
%[5]s	RSB

; SAVEREGS saves R0-R11 and marks the case complete.
SAVEREGS:
	MOVQ	R0,REGV
	MOVQ	R2,REGV+8
	MOVQ	R4,REGV+16
	MOVQ	R6,REGV+24
	MOVQ	R8,REGV+32
	MOVQ	R10,REGV+40
	BISL2	#1,FLAGS
	RSB

; HANDLER records the signal array. A trap continues, so the case's
; results are saved; anything else unwinds to the main program. A second
; condition in the same case (an emulator that reports a trap as a fault
; would run the instruction again, and signal again, forever) also
; unwinds, keeping the first signal array.
	.ENTRY	HANDLER,^M<R2,R3,R4,R5>
	MOVL	4(AP),R2
	CMPL	4(R2),#SS$_UNWIND
	BNEQ	10$
	RET
10$:	BBCS	#2,FLAGS,15$
	BISL2	#8,FLAGS
	BRB	50$
15$:	MOVL	(R2),SIGV
	MOVL	(R2),R3
	CMPL	R3,#8
	BLEQ	20$
	MOVL	#8,R3
20$:	MOVAL	SIGV+4,R4
	MOVAL	4(R2),R5
30$:	MOVL	(R5)+,(R4)+
	SOBGTR	R3,30$
	MOVL	4(R2),R3
	CMPL	R3,#SS$_INTOVF
	BEQL	40$
	CMPL	R3,#SS$_INTDIV
	BEQL	40$
	CMPL	R3,#SS$_FLTOVF
	BEQL	40$
	CMPL	R3,#SS$_FLTDIV
	BEQL	40$
	CMPL	R3,#SS$_FLTUND
	BEQL	40$
	CMPL	R3,#SS$_DECOVF
	BEQL	40$
50$:	CLRQ	-(SP)
	CALLS	#2,G^SYS$UNWIND
	RET
40$:	MOVL	#SS$_CONTINUE,R0
	RET

%[6]s	.END	%[1]s
`, s.name, s.title, data.String(), main.String(), loads.String(), routines.String())
}

// caseRoutine returns case n's routine, Cn, adding its operands' data
// to data.
func caseRoutine(data *strings.Builder, n int, t testCase) string {
	var r strings.Builder

	fmt.Fprintf(&r, "; %d: %s\n", n, t.name)
	fmt.Fprintf(&r, "\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n", n)
	fmt.Fprintf(&r, "\tMOVAB\tHANDLER,(FP)\n")

	var operands []string

	slot := 0
	first := ""

	for k, o := range t.ops {
		switch o.kind {
		case 'r', 'm':
			label := fmt.Sprintf("D%d_%d", n, k+1)
			if first == "" {
				first = label
			}

			fmt.Fprintf(data, "%s:", label)
			writeBytes(data, o.data)

			if o.kind == 'r' {
				operands = append(operands, label)

				continue
			}

			dst := dstSlot(slot)
			slot++
			fmt.Fprintf(&r, "\tMOVC3\t#%d,%s,%s\n", len(o.data), label, dst)
			operands = append(operands, dst)
		case 'w':
			operands = append(operands, dstSlot(slot))
			slot++
		case 't':
			operands = append(operands, o.text)
		}
	}

	// D_0 in a register preset or a line after the instruction is the
	// case's first data operand.
	sym := func(s string) string { return strings.ReplaceAll(s, "D_0", first) }

	fmt.Fprintf(&r, "\tJSB\tLOADREGS\n")

	for _, rs := range t.regs {
		if rs.addr != "" {
			fmt.Fprintf(&r, "\tMOVAB\t%s,R%d\n", sym(rs.addr), rs.reg)
		} else {
			fmt.Fprintf(&r, "\tMOVL\t#^X%X,R%d\n", rs.value, rs.reg)
		}
	}

	cc := 0xF
	if t.ccSet {
		cc = t.cc
	}

	// Clear N, Z, V, C, IV, FU, and DV (not T), then set the case's.
	fmt.Fprintf(&r, "\tBICPSW\t#^XEF\n")

	if bits := cc | t.psw; bits != 0 {
		fmt.Fprintf(&r, "\tBISPSW\t#^X%02X\n", bits)
	}

	switch {
	case t.raw != "":
		fmt.Fprintf(&r, "\t%s\n", t.raw)
	case t.branch:
		operands = append(operands, "90$")

		fallthrough
	default:
		fmt.Fprintf(&r, "\t%s\t%s\n", t.insn, strings.Join(operands, ","))
	}

	finish := func(flag string) {
		fmt.Fprintf(&r, "\tMOVPSL\tPSLV\n")

		if flag != "" {
			fmt.Fprintf(&r, "\tBISL2\t#%s,FLAGS\n", flag)
		}

		for _, line := range t.post {
			fmt.Fprintf(&r, "\t%s\n", sym(line))
		}

		fmt.Fprintf(&r, "\tJSB\tSAVEREGS\n\tRET\n")
	}

	finish("")

	if t.branch {
		fmt.Fprintf(&r, "90$:")
		finish("2")
	}

	r.WriteString("\n")

	return r.String()
}

func dstSlot(k int) string {
	if k == 0 {
		return "DST"
	}

	return fmt.Sprintf("DST+%d", 32*k)
}

// writeBytes writes data as .BYTE lines of up to 16, after a label
// already written.
func writeBytes(sb *strings.Builder, data []byte) {
	for i := 0; i < len(data); i += 16 {
		end := min(i+16, len(data))
		parts := make([]string, 0, end-i)

		for _, v := range data[i:end] {
			parts = append(parts, fmt.Sprintf("^X%02X", v))
		}

		fmt.Fprintf(sb, "\t.BYTE\t%s\n", strings.Join(parts, ","))
	}

	if len(data) == 0 {
		sb.WriteString("\n")
	}
}

func procedure(sections []section) string {
	var sb strings.Builder

	sb.WriteString(`$ ! INSN35.COM - assembles, links, and runs the Phase 35 instruction
$ ! probes. Written by testdata/insn35/gen.go. Run it with the exchange
$ ! volume's [000000] as the default directory:
$ !
$ !     @INSN35/OUTPUT=INSN35.LOG
$ !
$ ! or, to run some of the probes, name them:
$ !
$ !     @INSN35/OUTPUT=INSN35.LOG P35G,P35H
$ !
$ ! Each probe writes its own .DMP file; running a probe again writes a
$ ! new version of it.
$ !
$ SET NOON
$ SET VERIFY
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ SHOW SYSTEM/NOPROCESS
$ WRITE SYS$OUTPUT F$GETSYI("HW_NAME")
$ LIST = P1
$ IF LIST .EQS. "" THEN LIST = "`)

	names := make([]string, len(sections))
	for i, s := range sections {
		names[i] = s.name
	}

	sb.WriteString(strings.Join(names, ","))
	sb.WriteString(`"
$ I = 0
$ LOOP:
$ NAME = F$ELEMENT(I,",",LIST)
$ IF NAME .EQS. "," THEN GOTO DONE
$ MACRO/NOLIST 'NAME'
$ LINK 'NAME'
$ RUN 'NAME'
$ I = I + 1
$ GOTO LOOP
$ DONE:
$ DIRECTORY/SIZE/DATE *.DMP
`)

	return sb.String()
}

func exchange(sections []section) string {
	var sb strings.Builder

	sb.WriteString(`! EXCHANGE.CMD - builds the Phase 35 instruction probes' exchange
! volume with govax. Written by testdata/insn35/gen.go. Run from the
! repository root:
!
!     govax console < testdata/insn35/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/insn35-exchange.dsk" /DEVICE=RD51 INSN35
MOUNT/WRITE DUA1 "testdata/disks/insn35-exchange.dsk"
`)

	for _, s := range sections {
		lower := strings.ToLower(s.name)
		fmt.Fprintf(&sb, "COPY \"testdata/insn35/%s.mar\"/HOST DUA1:[000000]%s.MAR\n", lower, s.name)
	}

	sb.WriteString(`COPY "testdata/insn35/insn35.com"/HOST DUA1:[000000]INSN35.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`)

	return sb.String()
}
