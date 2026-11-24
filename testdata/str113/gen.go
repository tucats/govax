//go:build ignore

// gen.go writes the Phase 113 probe of LIBRTL's STR$ arithmetic routines
// (docs/planning/PHASE-113 - Fortran errata.md, subtask 7): str113.mar, a MACRO
// program that calls STR$ADD, STR$MUL, STR$DIVIDE, STR$RECIP, STR$ROUND, and
// STR$ELEMENT_R8, case by case, and prints what each returned, signaled, and
// stored. It is laid out as testdata/str83's: each case is a routine under a
// handler that records a signaled condition and unwinds, so a status returned
// shows apart from one signaled, and JSB cases load their arguments into R0,
// R1, ... and show R1 to R6. A result's sign and exponent start as 99 each, so a
// case shows whether the routine wrote them. Run this from the repository root:
//
//	go run testdata/str113/gen.go
//
// It runs in Phase 113's probe round (subtask 12); its README has the steps.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// show is one value a case prints after its R0 line.
type show struct {
	kind  string // "mem" (longwords), "txt" (an S or D string), "vs" (a VS string)
	label string // the value's address (a label; "%" is the case's prefix)
	count int    // mem: how many longwords
}

func mem(label string, n int) show { return show{"mem", label, n} }
func txt(label string) show        { return show{"txt", label, 0} }
func vs(label string) show         { return show{"vs", label, 0} }

// probeCase is one call of one routine.
type probeCase struct {
	name string   // printed first (at most 31 characters)
	data []string // data lines; "%" is the case's label prefix
	pre  []string // instructions before the call
	call string   // the routine; "" for none (the case only shows values)
	jsb  bool     // reached by JSB, args in R0, R1, ...
	args []string // its arguments, first first
	post []string // instructions after the call, R0 preserved
	show []show
}

// push returns the instructions that push args, last first, as a CALLS
// wants them: "#n" is pushed as a value, "0" omits an optional argument
// (a 0 address), and anything else is an address (PUSHAB).
func push(args []string) []string {
	var lines []string

	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]

		switch {
		case a == "0":
			lines = append(lines, "PUSHL\t#0")
		case strings.HasPrefix(a, "#"):
			lines = append(lines, "PUSHL\t"+a)
		default:
			lines = append(lines, "PUSHAB\t"+a)
		}
	}

	return lines
}

// registers returns the instructions that load a JSB routine's args into
// R0, R1, ...: "#n" as a value, anything else as an address. R1 to R6
// are set to ^X5A5A5A51 to ^X5A5A5A56 first, so the case shows which the
// routine changed (an address argument's register shown relative to its
// label: relativeArgs).
func registers(args []string) []string {
	var lines []string

	for r := 1; r <= 6; r++ {
		lines = append(lines, fmt.Sprintf("MOVL\t#^X5A5A5A5%d,R%d", r, r))
	}

	for r, a := range args {
		if strings.HasPrefix(a, "#") {
			lines = append(lines, fmt.Sprintf("MOVL\t%s,R%d", a, r))
		} else {
			lines = append(lines, fmt.Sprintf("MOVAB\t%s,R%d", a, r))
		}
	}

	return lines
}

// saveRegisters stores R1 to R6, after a JSB, at %REGS.
var saveRegisters = []string{"MOVQ\tR1,%REGS", "MOVQ\tR3,%REGS+8", "MOVQ\tR5,%REGS+16"}

// relativeArgs makes each saved register R1 to R6 that held an address
// argument relative to that argument's label, so it doesn't depend on
// where the program was loaded: an unchanged one shows 0.
func relativeArgs(args []string) []string {
	var lines []string

	for r, a := range args {
		if r > 0 && !strings.HasPrefix(a, "#") {
			lines = append(lines, fmt.Sprintf("SUBL2\t#%s,%%REGS+%d", a, 4*(r-1)))
		}
	}

	return lines
}

// ascid is a class S text descriptor for s at label, its text after it. A
// byte past ASCII, or a "%" (which prefix would take for the case's label
// prefix), is an angle-bracket term between the quoted runs; no run is empty
// unless s is.
func ascid(label, s string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s:\t.ASCID\t", label)

	if s == "" {
		b.WriteString(`""`)

		return b.String()
	}

	open := false

	for i := 0; i < len(s); i++ {
		if s[i] < 0x80 && s[i] != '%' {
			if !open {
				b.WriteByte('"')
				open = true
			}

			b.WriteByte(s[i])

			continue
		}

		if open {
			b.WriteByte('"')
			open = false
		}

		fmt.Fprintf(&b, "<%d>", s[i])
	}

	if open {
		b.WriteByte('"')
	}

	return b.String()
}

// desc is a descriptor at label: length, data type, class, then the
// pointer (a label, or 0).
func desc(label string, length, dtype, class int, ptr string) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t%d,%d\n\t.ADDRESS\t%s", label, length, dtype, class, ptr)
}

// dyn is an empty dynamic string descriptor, class D, type T.
func dyn(label string) string {
	return fmt.Sprintf("%s:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0", label)
}

// sd is a class SD descriptor: a scale after the pointer.
func sd(label string, length, dtype int, ptr string, scale int) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t%d,9\n\t.ADDRESS\t%s\n\t.BYTE\t%d,0,0,0", label, length, dtype, ptr, scale)
}

// nonzero sets %Z to 1 when the longword at addr isn't 0 (a pointer's
// presence, which is all the probe may show of it).
func nonzero(addr string) []string {
	return []string{"CLRL\t%Z", "TSTL\t" + addr, "BEQL\t%NZ", "MOVL\t#1,%Z", "%NZ:"}
}

// The destinations a result can have: D an empty dynamic string, S3 a fixed
// string of three dashes, V4 a varying string of at most 4 bytes.
func destS(n int) (string, show) {
	return ascid("%D", strings.Repeat("-", n)), txt("%D")
}

func destD() (string, show) { return dyn("%D"), txt("%D") }

func destV(n int) (string, show) {
	return desc("%D", n, 14, 11, "%V") + "\n%V:\t.WORD\t0\n\t.ASCII\t\"" + strings.Repeat("?", n) + "\"\n\t.WORD\t^X5555", vs("%D")
}

// long is the data line of a longword.
func long(label string, v int) string { return fmt.Sprintf("%s:\t.LONG\t%d", label, v) }

// num is a number's three arguments at %<x>S (sign), %<x>E (exponent), and
// %<x>D (the digits' class S descriptor), and their labels.
type num struct {
	sign, exp int
	digits    string
}

func (n num) data(x string) []string {
	return []string{long("%"+x+"S", n.sign), long("%"+x+"E", n.exp), ascid("%"+x+"D", n.digits)}
}

func (n num) args(x string) []string {
	return []string{"%" + x + "S", "%" + x + "E", "%" + x + "D"}
}

// resultData is the result's sign and exponent, 99 each until the routine
// writes them, and its digits' destination; resultShows shows them.
func resultData(dest string) ([]string, show) {
	var d string

	var sh show

	switch dest {
	case "S3":
		d, sh = destS(3)
	case "V4":
		d, sh = destV(4)
	default:
		d, sh = destD()
	}

	return []string{"%CS:\t.LONG\t99", "%CE:\t.LONG\t99", d}, sh
}

var resultArgs = []string{"%CS", "%CE", "%D"}

// binary is routine(a, b, c), ADD's, MUL's, and RECIP's form, into dest.
func binary(name, routine string, a, b num, dest string) probeCase {
	rd, sh := resultData(dest)
	data := append(append(a.data("A"), b.data("B")...), rd...)
	args := append(append(a.args("A"), b.args("B")...), resultArgs...)

	return probeCase{name: name, data: data, call: routine, args: args, show: []show{mem("%CS", 2), sh}}
}

// divide is STR$DIVIDE(a, b, total-digits, indicator, c).
func divide(name string, a, b num, places, indicator int) probeCase {
	rd, sh := resultData("D")
	data := append(append(a.data("A"), b.data("B")...), long("%P0", places), long("%P1", indicator))
	data = append(data, rd...)
	args := append(append(a.args("A"), b.args("B")...), "%P0", "%P1")
	args = append(args, resultArgs...)

	return probeCase{name: name, data: data, call: "STR$DIVIDE", args: args, show: []show{mem("%CS", 2), sh}}
}

// round is STR$ROUND(places, flags, a, c).
func round(name string, places, flags int, a num) probeCase {
	rd, sh := resultData("D")
	data := append(append([]string{long("%P0", places), long("%P1", flags)}, a.data("A")...), rd...)
	args := append(append([]string{"%P0", "%P1"}, a.args("A")...), resultArgs...)

	return probeCase{name: name, data: data, call: "STR$ROUND", args: args, show: []show{mem("%CS", 2), sh}}
}

// jsb is a JSB case: what it stores in %D, and R1 to R6.
func jsb(name, routine string, data []string, args []string, shows ...show) probeCase {
	return probeCase{name: name + " (JSB)", data: append(data, "%REGS:\t.BLKL\t6"), call: routine, jsb: true, args: args,
		show: append(shows, mem("%REGS", 6))}
}

// The manuals' operands: A = -1000, B = .0002.
var (
	manualA = num{1, 3, "1"}
	manualB = num{0, -4, "2"}
)

func buildCases() []probeCase {
	var c []probeCase

	add := func(more ...probeCase) { c = append(c, more...) }

	// STR$ADD: the manual's example, signs, zero, alignment, leading and
	// trailing zeros, empty digits, and the destination kinds.
	add(binary("ADD manual", "STR$ADD", manualA, manualB, "D"))
	add(binary("ADD 5+7", "STR$ADD", num{0, 0, "5"}, num{0, 0, "7"}, "D"))
	add(binary("ADD 500-7", "STR$ADD", num{0, 2, "5"}, num{1, 0, "7"}, "D"))
	add(binary("ADD -500+7", "STR$ADD", num{1, 2, "5"}, num{0, 0, "7"}, "D"))
	add(binary("ADD 7-7", "STR$ADD", num{0, 0, "7"}, num{1, 0, "7"}, "D"))
	add(binary("ADD 7e3-7e3", "STR$ADD", num{0, 3, "7"}, num{1, 3, "7"}, "D"))
	add(binary("ADD 0-7e-3", "STR$ADD", num{0, 0, "0"}, num{1, -3, "7"}, "D"))
	add(binary("ADD 0e5+7e-3", "STR$ADD", num{0, 5, "0"}, num{0, -3, "7"}, "D"))
	add(binary("ADD -0+0", "STR$ADD", num{1, 2, "0"}, num{0, -2, "000"}, "D"))
	add(binary("ADD 10e1+0e1", "STR$ADD", num{0, 1, "10"}, num{0, 1, "0"}, "D"))
	add(binary("ADD 007+3", "STR$ADD", num{0, 0, "007"}, num{0, 0, "3"}, "D"))
	add(binary("ADD 999+1", "STR$ADD", num{0, 0, "999"}, num{0, 0, "1"}, "D"))
	add(binary("ADD 1e-5+1e5", "STR$ADD", num{0, -5, "1"}, num{0, 5, "1"}, "D"))
	add(binary("ADD long", "STR$ADD", num{0, 0, "123456789012345678901234567890"}, num{0, 0, "987654321098765432109876543210"}, "D"))
	add(binary("ADD empty+5", "STR$ADD", num{0, 0, ""}, num{0, 0, "5"}, "D"))
	add(binary("ADD empty+empty", "STR$ADD", num{0, 0, ""}, num{0, 0, ""}, "D"))
	add(binary("ADD into S3", "STR$ADD", num{0, 0, "12340"}, num{0, 0, "5"}, "S3"))
	add(binary("ADD into S3 short", "STR$ADD", num{0, 0, "1"}, num{0, 0, "5"}, "S3"))
	add(binary("ADD into V4", "STR$ADD", num{0, 0, "12340"}, num{0, 0, "5"}, "V4"))
	add(binary("ADD into V4 short", "STR$ADD", num{0, 0, "1"}, num{0, 0, "5"}, "V4"))
	add(binary("ADD digit A", "STR$ADD", num{0, 0, "12A"}, num{0, 0, "5"}, "D"))
	add(binary("ADD leading blank", "STR$ADD", num{0, 0, " 12"}, num{0, 0, "5"}, "D"))
	add(binary("ADD trailing blank", "STR$ADD", num{0, 0, "12 "}, num{0, 0, "5"}, "D"))
	add(binary("ADD plus sign", "STR$ADD", num{0, 0, "+12"}, num{0, 0, "5"}, "D"))
	add(binary("ADD decimal point", "STR$ADD", num{0, 0, "1.2"}, num{0, 0, "5"}, "D"))
	add(binary("ADD sign 2", "STR$ADD", num{2, 0, "12"}, num{0, 0, "5"}, "D"))
	add(binary("ADD sign -1", "STR$ADD", num{-1, 0, "12"}, num{0, 0, "5"}, "D"))
	add(binary("ADD sign 3 of b", "STR$ADD", num{0, 0, "12"}, num{3, 0, "5"}, "D"))
	add(binary("ADD huge exps", "STR$ADD", num{0, 2147483647, "1"}, num{0, 2147483647, "1"}, "D"))
	add(binary("ADD far apart", "STR$ADD", num{0, 90, "1"}, num{0, 0, "1"}, "D"))

	k := binary("ADD 8 args", "STR$ADD", manualA, manualB, "D")
	k.args = k.args[:8]
	add(k)

	k = binary("ADD into class 99", "STR$ADD", num{0, 0, "1"}, num{0, 0, "5"}, "D")
	k.data[len(k.data)-1] = desc("%D", 3, 14, 99, "%T") + "\n%T:\t.ASCII\t\"---\""
	k.show[1] = mem("%T", 1)
	add(k)

	k = binary("ADD same as A", "STR$ADD", num{0, 0, "15"}, num{0, 0, "5"}, "D")
	k.data = append(k.data, dyn("%AX"))
	k.pre = []string{"PUSHAB\t%AD", "PUSHAB\t%AX", "CALLS\t#2,G^STR$COPY_DX"}
	k.args = []string{"%AS", "%AE", "%AX", "%BS", "%BE", "%BD", "%AS", "%AE", "%AX"}
	k.show = []show{mem("%AS", 2), txt("%AX")}
	add(k)

	// STR$MUL.
	add(binary("MUL manual", "STR$MUL", manualA, manualB, "D"))
	add(binary("MUL 25x4", "STR$MUL", num{0, 0, "25"}, num{0, 0, "4"}, "D"))
	add(binary("MUL -3x-4", "STR$MUL", num{1, 0, "3"}, num{1, 0, "4"}, "D"))
	add(binary("MUL -3x0", "STR$MUL", num{1, 0, "3"}, num{0, 0, "0"}, "D"))
	add(binary("MUL 0e5x0e-2", "STR$MUL", num{0, 5, "0"}, num{1, -2, "0"}, "D"))
	add(binary("MUL 12e2x5e-1", "STR$MUL", num{0, 2, "12"}, num{0, -1, "5"}, "D"))
	add(binary("MUL 007x003", "STR$MUL", num{0, 0, "007"}, num{0, 0, "003"}, "D"))
	add(binary("MUL long", "STR$MUL", num{0, 0, "99999999999999999999"}, num{0, 0, "99999999999999999999"}, "D"))
	add(binary("MUL exp overflow", "STR$MUL", num{0, 2147483647, "1"}, num{0, 1, "1"}, "D"))
	add(binary("MUL exp underflow", "STR$MUL", num{0, -2147483647, "1"}, num{0, -2, "1"}, "D"))

	// STR$DIVIDE: total-digits and the round-truncate-indicator.
	add(divide("DIVIDE manual", manualA, manualB, 3, 0))
	add(divide("DIVIDE manual round", manualA, manualB, 3, 1))
	add(divide("DIVIDE 2/3 4 trunc", num{0, 0, "2"}, num{0, 0, "3"}, 4, 0))
	add(divide("DIVIDE 2/3 4 round", num{0, 0, "2"}, num{0, 0, "3"}, 4, 1))
	add(divide("DIVIDE 1/8 2 round", num{0, 0, "1"}, num{0, 0, "8"}, 2, 1))
	add(divide("DIVIDE -1/8 2 round", num{1, 0, "1"}, num{0, 0, "8"}, 2, 1))
	add(divide("DIVIDE 1/-8 2 trunc", num{0, 0, "1"}, num{1, 0, "8"}, 2, 0))
	add(divide("DIVIDE 10/4 0 round", num{0, 0, "10"}, num{0, 0, "4"}, 0, 1))
	add(divide("DIVIDE 1/3 0", num{0, 0, "1"}, num{0, 0, "3"}, 0, 0))
	add(divide("DIVIDE 1/3 0 round", num{0, 0, "1"}, num{0, 0, "3"}, 0, 1))
	add(divide("DIVIDE 12345/1 -2", num{0, 0, "12345"}, num{0, 0, "1"}, -2, 0))
	add(divide("DIVIDE 12355/1 -2 round", num{0, 0, "12355"}, num{0, 0, "1"}, -2, 1))
	add(divide("DIVIDE 6/2 2", num{0, 0, "6"}, num{0, 0, "2"}, 2, 0))
	add(divide("DIVIDE 6e2/2e-1 0", num{0, 2, "6"}, num{0, -1, "2"}, 0, 0))
	add(divide("DIVIDE 0/5 2", num{0, 0, "0"}, num{0, 0, "5"}, 2, 0))
	add(divide("DIVIDE -0/5 2", num{1, 0, "0"}, num{0, 0, "5"}, 2, 0))
	add(divide("DIVIDE -1/3 0", num{1, 0, "1"}, num{0, 0, "3"}, 0, 0))
	add(divide("DIVIDE 1/0", num{0, 0, "1"}, num{0, 0, "0"}, 2, 0))
	add(divide("DIVIDE 1/empty", num{0, 0, "1"}, num{0, 0, ""}, 2, 0))
	add(divide("DIVIDE 1/0e3", num{0, 0, "1"}, num{0, 3, "000"}, 2, 0))
	add(divide("DIVIDE indicator 2", num{0, 0, "2"}, num{0, 0, "3"}, 4, 2))
	add(divide("DIVIDE indicator 3", num{0, 0, "2"}, num{0, 0, "3"}, 4, 3))
	add(divide("DIVIDE indicator -1", num{0, 0, "2"}, num{0, 0, "3"}, 4, -1))
	add(divide("DIVIDE 1/7 30", num{0, 0, "1"}, num{0, 0, "7"}, 30, 1))
	add(divide("DIVIDE 22/7 9 round", num{0, 0, "22"}, num{0, 0, "7"}, 9, 1))
	add(divide("DIVIDE small 2", num{0, -9, "1"}, num{0, 0, "1"}, 2, 1))
	add(divide("DIVIDE digit A", num{0, 0, "1A"}, num{0, 0, "3"}, 2, 0))

	k = divide("DIVIDE 10 args", manualA, manualB, 3, 0)
	k.args = k.args[:10]
	add(k)

	// STR$RECIP: what B's parts mean, rounding, and trailing zeros.
	add(binary("RECIP manual", "STR$RECIP", manualA, manualB, "D"))
	add(binary("RECIP 3 e-4", "STR$RECIP", num{0, 0, "3"}, num{0, -4, "1"}, "D"))
	add(binary("RECIP 6 e-4", "STR$RECIP", num{0, 0, "6"}, num{0, -4, "1"}, "D"))
	add(binary("RECIP 6 9e-4", "STR$RECIP", num{0, 0, "6"}, num{0, -4, "9"}, "D"))
	add(binary("RECIP 6 25e-4", "STR$RECIP", num{0, 0, "6"}, num{0, -4, "25"}, "D"))
	add(binary("RECIP 6 1e-6", "STR$RECIP", num{0, 0, "6"}, num{0, -6, "1"}, "D"))
	add(binary("RECIP 6 -1e-4", "STR$RECIP", num{0, 0, "6"}, num{1, -4, "1"}, "D"))
	// "RECIP 6 0e-4", a precision limit whose digits are 0, was here: VMS 7.1's
	// STR$RECIP never returned from it (24-NOV-2026, minutes of CPU time, no I/O),
	// so it was taken out for the rest of the round to run.
	add(binary("RECIP 8 e-4", "STR$RECIP", num{0, 0, "8"}, num{0, -4, "1"}, "D"))
	add(binary("RECIP 25e-2 e-4", "STR$RECIP", num{0, -2, "25"}, num{0, -4, "1"}, "D"))
	add(binary("RECIP 3 e2", "STR$RECIP", num{0, 0, "3"}, num{0, 2, "1"}, "D"))
	add(binary("RECIP 3 e0", "STR$RECIP", num{0, 0, "3"}, num{0, 0, "1"}, "D"))
	add(binary("RECIP 1 e-4", "STR$RECIP", num{0, 0, "1"}, num{0, -4, "1"}, "D"))
	add(binary("RECIP -7 e-10", "STR$RECIP", num{1, 0, "7"}, num{0, -10, "1"}, "D"))
	add(binary("RECIP 0", "STR$RECIP", num{0, 0, "0"}, num{0, -4, "1"}, "D"))

	// STR$ROUND: places and flags.
	add(round("ROUND manual", 3, 0, num{0, -4, "9999998"}))
	add(round("ROUND manual trunc", 3, 1, num{0, -4, "9999998"}))
	add(round("ROUND 12345 3", 3, 0, num{0, 0, "12345"}))
	add(round("ROUND 12355 3", 3, 0, num{0, 0, "12355"}))
	add(round("ROUND 125 2", 2, 0, num{0, 0, "125"}))
	add(round("ROUND -125 2", 2, 0, num{1, 0, "125"}))
	add(round("ROUND 12 5", 5, 0, num{0, 0, "12"}))
	add(round("ROUND 95 1", 1, 0, num{0, 0, "95"}))
	add(round("ROUND 0099 1", 1, 0, num{0, 0, "0099"}))
	add(round("ROUND 12000 2", 2, 0, num{0, 0, "12000"}))
	add(round("ROUND 0 3", 3, 0, num{0, 5, "0"}))
	add(round("ROUND -0 3", 3, 0, num{1, 0, "000"}))
	add(round("ROUND 12345 0", 0, 0, num{0, 0, "12345"}))
	add(round("ROUND 12345 -1", -1, 0, num{0, 0, "12345"}))
	add(round("ROUND flags 2", 3, 2, num{0, 0, "12355"}))
	add(round("ROUND flags 3", 3, 3, num{0, 0, "12355"}))
	add(round("ROUND digit A", 3, 0, num{0, 0, "12A55"}))

	k = round("ROUND 7 args", 3, 0, num{0, 0, "12345"})
	k.args = k.args[:7]
	add(k)

	// STR$ELEMENT_R8, which neither manual describes: R1 by value, as STR$LEFT_R8
	// takes its position, then by reference.
	add(jsb("ELEMENT_R8 value", "STR$ELEMENT_R8", []string{dyn("%D"), ascid("%L", ","), ascid("%S", "A,BC,D")},
		[]string{"%D", "#1", "%L", "%S"}, txt("%D")))
	add(jsb("ELEMENT_R8 reference", "STR$ELEMENT_R8", []string{dyn("%D"), long("%N", 1), ascid("%L", ","), ascid("%S", "A,BC,D")},
		[]string{"%D", "%N", "%L", "%S"}, txt("%D")))

	// STR$MATCH_WILD with "%", which testdata/str83's probe couldn't ask: its
	// label prefix replaced every "%" in a case's data (ascid now spares them).
	for _, x := range [][2]string{{"abc", "a%c"}, {"abc", "a%"}, {"abc", "%%%"}, {"abc", "%%%%"}, {"abc", "%%"},
		{"ac", "a%c"}, {"", "%"}, {"abc", "%*"}, {"abc", "*%"}, {"abcd", "a*%d"}, {"a%c", "a%c"}, {"abc", "a\\%c"}} {
		add(probeCase{name: "WILD " + x[0] + " " + x[1], data: []string{ascid("%C", x[0]), ascid("%P", x[1])},
			call: "STR$MATCH_WILD", args: []string{"%C", "%P"}})
	}

	return c
}

var cases = buildCases()

// The data every case may use.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"SAVEP:\t.LONG\t0",
}

// strings), the handler, and the routines that print.
const programHead = `	.TITLE	STR113	Phase 113 STR$ arithmetic probe
	.IDENT	/V1.0/

; Written by testdata/str113/gen.go: do not edit. Calls each of LIBRTL's
; STR$ arithmetic routines, case by case, each in a routine of its own under
; HANDLER, and prints with LIB$PUT_OUTPUT what it returned in R0, any
; condition it signaled, and what it stored. See gen.go.

	$SSDEF

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
LINLEN:	.WORD	0
	.ALIGN	LONG
LINDSC:	.WORD	132
	.BYTE	14,1
	.ADDRESS	LINE
PARTDSC:	.WORD	9
	.BYTE	14,1
	.LONG	0
OUTDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS	LINE
LINE:	.BLKB	132
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
LWCTL:	.ASCID	" !XL"
TXTCTL:	.ASCID	/  !AC: !UL "!AF"/
`

const programCode = `
	.PSECT	CODE,NOWRT,EXE

; HANDLER records the condition (and an access violation's reason mask
; and address), then unwinds to the case routine's caller, which gets
; the condition in R0. It ignores SS$_UNWIND, the call an unwind makes.
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0		; the signal array
	CMPL	4(R0),#SS$_UNWIND
	BEQL	9$
	MOVL	4(R0),SIGCOND
	CLRQ	SIGARG1
	CMPL	SIGCOND,#SS$_ACCVIO
	BNEQ	1$
	MOVQ	8(R0),SIGARG1
1$:	MOVL	8(AP),R1		; the mechanism array
	MOVL	SIGCOND,12(R1)		; CHF$L_MCH_SAVR0
	$UNWIND_S
9$:	RET

; SHOWR0 name, r0: the case's line.
	.ENTRY	SHOWR0,^M<>
	TSTL	SIGCOND
	BNEQ	1$
	$FAO_S	CTRSTR=R0CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=8(AP)
	BRB	2$
1$:	$FAO_S	CTRSTR=SIGCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=8(AP),-
		P3=SIGCOND,P4=SIGARG1,P5=SIGARG2
2$:	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; SHOWMEM name, address, count: count longwords in hex.
	.ENTRY	SHOWMEM,^M<R2,R3,R4>
	MOVL	8(AP),R2
	MOVL	12(AP),R3
	$FAO_S	CTRSTR=MEMCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
	MOVZWL	LINLEN,R4
1$:	TSTL	R3
	BEQL	2$
	MOVAB	LINE[R4],PARTDSC+4
	$FAO_S	CTRSTR=LWCTL,OUTBUF=PARTDSC,P1=(R2)+
	ADDL2	#9,R4
	DECL	R3
	BRB	1$
2$:	MOVW	R4,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; SHOWTXT name, descriptor: a class S or D string's length and text.
	.ENTRY	SHOWTXT,^M<R2>
	MOVL	8(AP),R2
	MOVZWL	(R2),R0
	$FAO_S	CTRSTR=TXTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=4(R2)
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; SHOWVS name, descriptor: a varying string's CURLEN and text.
	.ENTRY	SHOWVS,^M<R2>
	MOVL	8(AP),R2
	MOVL	4(R2),R2		; CURLEN, then the text
	MOVZWL	(R2)+,R0
	$FAO_S	CTRSTR=TXTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=R2
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
`

// prefix substitutes the case's label prefix for "%".
func prefix(s string, n int) string {
	return strings.ReplaceAll(s, "%", fmt.Sprintf("C%d_", n))
}

// shownName is how a shown value is named in the output: its label
// without the case's prefix.
func shownName(s show) string {
	return strings.ReplaceAll(s.label, "%", "")
}

func program() string {
	var b strings.Builder

	b.WriteString(programHead)
	b.WriteString("\n; Shared data.\n")

	for _, line := range shared {
		b.WriteString(line + "\n")
	}

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ALIGN\tQUAD\n", n, c.name)

		for _, line := range c.data {
			b.WriteString(prefix(line, n) + "\n")
		}

		fmt.Fprintf(&b, "C%d_NAME:\t.ASCIC\t\"%s\"\n", n, c.name)

		for k, sh := range c.show {
			fmt.Fprintf(&b, "C%d_S%d:\t.ASCIC\t\"%s\"\n", n, k+1, shownName(sh))
		}
	}

	b.WriteString(programCode)

	// Each case's routine.
	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, c.name, n)

		for _, line := range c.pre {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		var post []string

		switch {
		case c.call == "":
			b.WriteString("\tCLRL\tR0\n")
		case c.jsb:
			for _, line := range registers(c.args) {
				b.WriteString("\t" + prefix(line, n) + "\n")
			}

			fmt.Fprintf(&b, "\tJSB\tG^%s\n", c.call)

			post = append(append([]string(nil), saveRegisters...), relativeArgs(c.args)...)
		default:
			for _, line := range push(c.args) {
				b.WriteString("\t" + prefix(line, n) + "\n")
			}

			fmt.Fprintf(&b, "\tCALLS\t#%d,G^%s\n", len(c.args), c.call)
		}

		// The post instructions keep R0: it's saved around them.
		b.WriteString("\tPUSHL\tR0\n")

		for _, line := range append(post, c.post...) {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		b.WriteString("\tMOVL\t(SP)+,R0\n\tRET\n")
	}

	// The main routine: each case, then its lines.
	b.WriteString("\n\t.ENTRY\tSTART,^M<>\n")

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n\tCLRL\tSIGCOND\t\t\t; %d: %s\n\tCALLS\t#0,C%d\n\tPUSHL\tR0\n\tPUSHAB\tC%d_NAME\n\tCALLS\t#2,SHOWR0\n",
			n, c.name, n, n)

		for k, sh := range c.show {
			switch sh.kind {
			case "mem":
				fmt.Fprintf(&b, "\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#3,SHOWMEM\n",
					sh.count, prefix(sh.label, n), n, k+1)
			case "txt":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWTXT\n", prefix(sh.label, n), n, k+1)
			case "vs":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWVS\n", prefix(sh.label, n), n, k+1)
			}
		}
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

func main() {
	file := filepath.Join("testdata", "str113", "str113.mar")
	if err := os.WriteFile(file, []byte(program()), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(cases), "cases")
}
