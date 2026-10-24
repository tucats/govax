//go:build ignore

// gen.go writes the Phase 72 probe of LIBRTL's OTS$ routines
// (docs/PHASE-72 - legacy images.md, subtask 6): ots72.mar, a MACRO
// program that calls OTS$'s floating conversions, moves, and dynamic
// strings case by case and prints what each returned, signaled, and
// stored; ots72t.mar, a study of how VMS's FORRTL calls LIBRTL's
// undocumented OTS$$CVT_x_T_R8 routines (README.md); ots72.com, which
// builds and runs both on VMS; and exchange.cmd and copyout.cmd, the
// govax console scripts that make the exchange volume and bring the
// results back. Run it from the repository root:
//
//	go run testdata/ots72/gen.go
//
// ots72.mar works as Phase 54's probe (testdata/lib54/gen.go) does: each
// case runs in a routine of its own under a condition handler that
// records the condition it's given and unwinds, so the output tells a
// status a routine returned from a condition it signaled. A JSB case
// sets R1 to R6 to known values, loads its arguments into R0, R1, ...,
// and shows R1 to R6 after.
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
	kind  string // "mem" (longwords), "txt" (an S or D string)
	label string // the value's address (a label; "%" is the case's prefix)
	count int    // mem: how many longwords
}

func mem(label string, n int) show { return show{"mem", label, n} }
func txt(label string) show        { return show{"txt", label, 0} }

// probeCase is one call of one routine.
type probeCase struct {
	name string         // printed first (at most 31 characters)
	data []string       // data lines; "%" is the case's label prefix
	pre  []string       // instructions before the call
	call string         // the routine
	jsb  bool           // reached by JSB, args in R0, R1, ...
	args []string       // its arguments, first first
	post []string       // instructions after the call, R0 preserved
	rel  map[int]string // JSB: the label a result register is relative to
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
// are set to ^X5A5A5A51 to ^X5A5A5A56 first.
func registers(args []string) []string {
	var lines []string

	for r := 1; r <= 6; r++ {
		lines = append(lines, fmt.Sprintf("MOVL\t#^X5A5A5A5%d,R%d", r, r))
	}

	for r, a := range args {
		switch {
		case a == "-": // left as it was set
		case strings.HasPrefix(a, "#"):
			lines = append(lines, fmt.Sprintf("MOVL\t%s,R%d", a, r))
		default:
			lines = append(lines, fmt.Sprintf("MOVAB\t%s,R%d", a, r))
		}
	}

	return lines
}

// saveRegisters stores R0 to R6, after a JSB, at %REGS.
var saveRegisters = []string{"MOVL\tR0,%REGS", "MOVQ\tR1,%REGS+4", "MOVQ\tR3,%REGS+12", "MOVQ\tR5,%REGS+20"}

// relativeArgs makes each saved register R1 to R6 that holds a P0
// address (^X200 to ^X3FFFFFFF: not 0, a small count, or one of the
// ^X5A5A5A5n presets) relative to a label, so it doesn't depend on where
// the program was loaded: an address argument's register to its label,
// and the registers in rel to theirs (a routine's results: one byte
// beyond a string, say).
func relativeArgs(args []string, rel map[int]string) []string {
	var lines []string

	for r := 1; r <= 6; r++ {
		label := rel[r]
		if label == "" && r < len(args) && args[r] != "-" && !strings.HasPrefix(args[r], "#") {
			label = args[r]
		}

		if label != "" {
			cell := fmt.Sprintf("%%REGS+%d", 4*r)
			skip := fmt.Sprintf("%%K%d", r)
			lines = append(lines,
				fmt.Sprintf("CMPL\t%s,#^X200", cell), "BLSSU\t"+skip,
				fmt.Sprintf("CMPL\t%s,#^X40000000", cell), "BGEQU\t"+skip,
				fmt.Sprintf("SUBL2\t#%s,%s", label, cell), skip+":")
		}
	}

	return lines
}

// textDesc is a class S descriptor at label for s, its text at label_T,
// written as bytes so a tab or quote needs no quoting.
func textDesc(label, s string) []string {
	lines := []string{fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T", label, len(s), label)}

	if s == "" {
		return append(lines, label+"_T:")
	}

	bytes := make([]string, len(s))
	for i := range len(s) {
		bytes[i] = fmt.Sprintf("^X%02X", s[i])
	}

	return append(lines, fmt.Sprintf("%s_T:\t.BYTE\t%s", label, strings.Join(bytes, ",")))
}

// outDesc is a class S descriptor at label for a string of n bytes, each
// a ~ so the output shows what a routine didn't write.
func outDesc(label string, n int) []string {
	return []string{
		fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T", label, n, label),
		fmt.Sprintf("%s_T:\t.BLKB\t0\n\t.REPEAT\t%d\n\t.BYTE\t^A\"~\"\n\t.ENDR", label, n),
	}
}

// desc is a descriptor at label: length, data type, class, then the
// pointer (a label, or 0).
func desc(label string, length, dtype, class int, ptr string) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t%d,%d\n\t.ADDRESS\t%s", label, length, dtype, class, ptr)
}

// nonzero sets %Z to 1 when the longword at addr isn't 0 (a pointer's
// presence, which is all the probe may show of it).
func nonzero(addr string) []string {
	return []string{"CLRL\t%Z", "TSTL\t" + addr, "BEQL\t%NZ", "MOVL\t#1,%Z", "%NZ:"}
}

// The floating formats: their directive, routine letter, and size in
// longwords.
type format struct {
	directive string
	letter    string
	longs     int
}

var (
	fmtF = format{".F_FLOATING", "F", 1}
	fmtD = format{".D_FLOATING", "D", 2}
	fmtG = format{".G_FLOATING", "G", 2}
	fmtH = format{".H_FLOATING", "H", 4}
)

// cnvout is a CNVOUT case: value (a literal, or "reserved") converted
// into a string of width bytes with digits digits in the fraction.
func cnvout(f format, value string, width, digits int) probeCase {
	routine := "OTS$CNVOUT"
	if f.letter != "D" {
		routine += "_" + f.letter
	}

	v := fmt.Sprintf("%%V:\t%s\t%s", f.directive, value)
	if value == "reserved" {
		v = "%V:\t.LONG\t^X8000,0,0,0"
	}

	return probeCase{
		name: fmt.Sprintf("CNVOUT%s %s %d.%d", strings.TrimPrefix(routine, "OTS$CNVOUT"), value, width, digits),
		data: append([]string{v}, outDesc("%D", width)...),
		call: routine, args: []string{"%V", "%D", fmt.Sprintf("#%d", digits)},
		show: []show{txt("%D")},
	}
}

// cvt is a CVT_T_z case: text into format f, with the trailing
// arguments args (digits-in-fraction, scale, flags as "#n", "E" for the
// case's extension-bits longword); the value is shown in hex.
func cvt(f format, name, text string, args ...string) probeCase {
	c := probeCase{
		name: "CVT_T_" + f.letter + " " + name,
		data: append(textDesc("%S", text), "%V:\t.LONG\t^XEEEEEEEE,^XEEEEEEEE,^XEEEEEEEE,^XEEEEEEEE", "%E:\t.LONG\t^XEEEEEEEE"),
		call: "OTS$CVT_T_" + f.letter,
		args: []string{"%S", "%V"},
		show: []show{mem("%V", f.longs)},
	}

	for _, a := range args {
		if a == "E" {
			c.args = append(c.args, "%E")
			c.show = append(c.show, mem("%E", 1))
		} else {
			c.args = append(c.args, a)
		}
	}

	return c
}

// The data every case may use.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"DYN1:\t.WORD\t0\n\t.BYTE\t0,0\n\t.LONG\t0\t; dynamic strings the dynamic-string cases share",
	"DYN2:\t.WORD\t0\n\t.BYTE\t0,0\n\t.LONG\t0",
	"DYN3:\t.WORD\t0\n\t.BYTE\t0,0\n\t.LONG\t0",
	"SAVEP:\t.LONG\t0",
	"HELLO:\t.ASCII\t\"HELLO, WORLD\"",
	"HELLOD:\t.WORD\t12\n\t.BYTE\t14,1\n\t.ADDRESS\tHELLO",
}

// cases are ots72.mar's, in the order they run.
var cases = buildCases()

func buildCases() []probeCase {
	var c []probeCase

	// CNVOUT: rounding, signs, zero, the leading zero, asterisks, digits
	// past the format's precision, the exponent's field, and reserved
	// operands.
	c = append(c,
		cnvout(fmtD, "3.14159", 12, 4),
		cnvout(fmtD, "-0.0012345", 11, 3),
		cnvout(fmtD, "0.0", 8, 2),
		cnvout(fmtD, "9.9996", 10, 3),
		cnvout(fmtD, "0.5", 7, 0),
		cnvout(fmtD, "0.04", 7, 0),
		cnvout(fmtD, "3.14159", 9, 4),
		cnvout(fmtD, "3.14159", 8, 4),
		cnvout(fmtD, "-3.14159", 9, 4),
		cnvout(fmtD, "3.14159", 4, 4),
		cnvout(fmtD, "0.15", 8, 1),
		cnvout(fmtD, "0.25", 8, 1),
		cnvout(fmtD, "1.0E38", 14, 5),
		cnvout(fmtD, "2.9E-39", 14, 5),
		cnvout(fmtD, "3.14159", 30, 20),
		cnvout(fmtD, "1.0", 24, 16),
		cnvout(fmtD, "reserved", 10, 2),
		cnvout(fmtG, "1.0E100", 10, 4),
		cnvout(fmtG, "-2.5E-200", 12, 5),
		cnvout(fmtG, "8.9E307", 12, 3),
		cnvout(fmtG, "3.14159", 22, 15),
		cnvout(fmtG, "1.0E99", 10, 2),
		cnvout(fmtG, "reserved", 10, 2),
		cnvout(fmtH, "1.25", 12, 3),
		cnvout(fmtH, "1.0E1000", 14, 4),
		cnvout(fmtH, "1.0E-1000", 14, 4),
		cnvout(fmtH, "1.0E100", 12, 4),
		cnvout(fmtH, "3.14159", 40, 33),
		cnvout(fmtH, "reserved", 10, 2),
	)

	// CNVOUT with no digits-in-fraction argument.
	c = append(c, probeCase{name: "CNVOUT 2 args",
		data: append([]string{"%V:\t.D_FLOATING\t2.5"}, outDesc("%D", 10)...),
		call: "OTS$CNVOUT", args: []string{"%V", "%D"}, show: []show{txt("%D")}})

	// CVT_T_F: the manual's example (five digits in the fraction), then
	// each flag and the syntax's edges.
	c = append(c,
		cvt(fmtF, "1234567+23", "1234567+23", "#5", "#0", "#0"),
		cvt(fmtF, "8.786534+3", "8.786534+3", "#5", "#0", "#0"),
		cvt(fmtF, "-983476E-3", "-983476E-3", "#5", "#0", "#0"),
		cvt(fmtF, "-23.734532", "-23.734532", "#5", "#0", "#0"),
		cvt(fmtF, "45 blanks zero", "45        ", "#5", "#0", "#0"),
		cvt(fmtF, "45 blanks ignored", "45        ", "#5", "#0", "#1"),
		cvt(fmtF, "2 args 1.5", "1.5"),
		cvt(fmtF, "3 args 15", "15", "#1"),
		cvt(fmtF, "-1.5D2", "  -1.5D2"),
		cvt(fmtF, "1.5q-1", "1.5q-1"),
		cvt(fmtF, "1.5e1", "1.5e1"),
		cvt(fmtF, "1.5D2 only E", "1.5D2", "#0", "#0", "#2"),
		cvt(fmtF, "1.5e2 only E", "1.5e2", "#0", "#0", "#2"),
		cvt(fmtF, "1.5+2", "1.5+2"),
		cvt(fmtF, "1.5+2 need letter", "1.5+2", "#0", "#0", "#32"),
		cvt(fmtF, "1.5H2", "1.5H2"),
		cvt(fmtF, "1.5G2", "1.5G2"),
		cvt(fmtF, "1.5 scale 2", "1.5", "#0", "#2", "#0"),
		cvt(fmtF, "1.5 scale -2", "1.5", "#0", "#-2", "#0"),
		cvt(fmtF, "1.5E1 scale 2", "1.5E1", "#0", "#2", "#0"),
		cvt(fmtF, "1.5E1 scale 2 always", "1.5E1", "#0", "#2", "#64"),
		cvt(fmtF, "1.5 fraction 3", "1.5", "#3", "#0", "#0"),
		cvt(fmtF, "15 fraction 3", "15", "#3", "#0", "#0"),
		cvt(fmtF, "1 tab 5", "1\t5"),
		cvt(fmtF, "1 tab 5 ignored", "1\t5", "#0", "#0", "#16"),
		cvt(fmtF, "tab 15", "\t15"),
		cvt(fmtF, "tab 15 ignored", "\t15", "#0", "#0", "#16"),
		cvt(fmtF, "12X", "12X"),
		cvt(fmtF, "1..2", "1..2"),
		cvt(fmtF, "1.2.", "1.2."),
		cvt(fmtF, "empty", ""),
		cvt(fmtF, "blanks", "   "),
		cvt(fmtF, "+", "+"),
		cvt(fmtF, "-", "-"),
		cvt(fmtF, ".", "."),
		cvt(fmtF, "E5", "E5"),
		cvt(fmtF, "1E", "1E"),
		cvt(fmtF, "1E blank 2", "1E 2"),
		cvt(fmtF, "1 blank E2", "1 E2"),
		cvt(fmtF, "1E+ 2", "1E+ 2"),
		cvt(fmtF, "1E2 blanks", "1E2  "),
		cvt(fmtF, "1E2 blanks ignored", "1E2  ", "#0", "#0", "#1"),
		cvt(fmtF, "- 1.5", "- 1.5"),
		cvt(fmtF, "+-1.5", "+-1.5"),
		cvt(fmtF, "1.5-", "1.5-"),
		cvt(fmtF, "1E40", "1E40"),
		cvt(fmtF, "1.7E38", "1.7E38"),
		cvt(fmtF, "1E-40", "1E-40"),
		cvt(fmtF, "1E-40 underflow error", "1E-40", "#0", "#0", "#4"),
		cvt(fmtF, "2.9E-39", "2.9E-39"),
		cvt(fmtF, "-0", "-0"),
		cvt(fmtF, "0.1", "0.1"),
		cvt(fmtF, "0.1 truncated", "0.1", "#0", "#0", "#8"),
		cvt(fmtF, "-0.1 truncated", "-0.1", "#0", "#0", "#8"),
		cvt(fmtF, "0.1 extension", "0.1", "#0", "#0", "#0", "E"),
		cvt(fmtF, "-0.1 extension", "-0.1", "#0", "#0", "#0", "E"),
		cvt(fmtF, "1.5 extension", "1.5", "#0", "#0", "#0", "E"),
		cvt(fmtF, "1E40 extension", "1E40", "#0", "#0", "#0", "E"),
		cvt(fmtF, "12X extension", "12X", "#0", "#0", "#0", "E"),
		cvt(fmtF, "0.15", "0.15"),
		cvt(fmtF, "16777217", "16777217"),
		cvt(fmtF, "16777217 truncated", "16777217", "#0", "#0", "#8"),
		cvt(fmtF, "1E99999", "1E99999"),
		cvt(fmtF, "1E-99999", "1E-99999"),
		cvt(fmtF, "0E99999", "0E99999"),
	)

	// The other formats.
	const pi = "3.14159265358979323846264338327950288"

	c = append(c,
		cvt(fmtD, "1.5D2", "1.5D2"),
		cvt(fmtD, "pi", pi),
		cvt(fmtD, "0.1", "0.1"),
		cvt(fmtD, "0.1 extension", "0.1", "#0", "#0", "#0", "E"),
		cvt(fmtD, "1.7E38", "1.7E38"),
		cvt(fmtD, "1E39", "1E39"),
		cvt(fmtD, "1234567+23", "1234567+23", "#5", "#0", "#0"),
		cvt(fmtG, "pi", pi),
		cvt(fmtG, "0.1 extension", "0.1", "#0", "#0", "#0", "E"),
		cvt(fmtG, "1E300", "1E300"),
		cvt(fmtG, "1E-300", "1E-300"),
		cvt(fmtG, "8.9E307", "8.9E307"),
		cvt(fmtG, "1E309", "1E309"),
		cvt(fmtG, "1E-309", "1E-309"),
		cvt(fmtH, "pi", pi),
		cvt(fmtH, "0.1 extension", "0.1", "#0", "#0", "#0", "E"),
		cvt(fmtH, "-1.25E4000", "-1.25E4000"),
		cvt(fmtH, "1E4933", "1E4933"),
		cvt(fmtH, "1E-4933", "1E-4933"),
		cvt(fmtH, "1E99999999", "1E99999999"),
	)

	// A bad input descriptor.
	c = append(c, probeCase{name: "CVT_T_D class 3",
		data: []string{desc("%S", 3, 14, 3, "%T"), "%T:\t.ASCII\t\"1.5\"", "%V:\t.LONG\t^XEEEEEEEE,^XEEEEEEEE"},
		call: "OTS$CVT_T_D", args: []string{"%S", "%V"}, show: []show{mem("%V", 2)}})

	// The moves. MOVE3_R5's destination goes in R2 and R3, two buffers,
	// to see which register it takes.
	c = append(c,
		probeCase{name: "MOVE3 overlapping", data: []string{"%B:\t.ASCII\t\"ABCDEFGH\""},
			call: "OTS$MOVE3", args: []string{"#5", "%B", "%B+2"}, show: []show{mem("%B", 2)}},
		probeCase{name: "MOVE3 below", data: []string{"%B:\t.ASCII\t\"ABCDEFGH\""},
			call: "OTS$MOVE3", args: []string{"#5", "%B+2", "%B"}, show: []show{mem("%B", 2)}},
		probeCase{name: "MOVE3 0", data: []string{"%B:\t.ASCII\t\"ABCDEFGH\""},
			call: "OTS$MOVE3", args: []string{"#0", "%B", "%B+4"}, show: []show{mem("%B", 2)}},
		probeCase{name: "MOVE5 filled", data: []string{"%S:\t.ASCII\t\"ABC\"", "%B:\t.ASCII\t\"xxxxxxxx\""},
			call: "OTS$MOVE5", args: []string{"#3", "%S", "#^A\"*\"", "#6", "%B"}, show: []show{mem("%B", 2)}},
		probeCase{name: "MOVE5 short", data: []string{"%S:\t.ASCII\t\"ABCDEF\"", "%B:\t.ASCII\t\"xxxxxxxx\""},
			call: "OTS$MOVE5", args: []string{"#6", "%S", "#^A\"*\"", "#2", "%B"}, show: []show{mem("%B", 2)}},
		probeCase{name: "MOVE5 overlapping", data: []string{"%B:\t.ASCII\t\"ABCDEFGH\""},
			call: "OTS$MOVE5", args: []string{"#4", "%B", "#^A\"*\"", "#6", "%B+1"}, show: []show{mem("%B", 2)}},
		probeCase{name: "MOVE3_R5 (JSB)", data: []string{"%S:\t.ASCII\t\"ABCD\"", "%B2:\t.ASCII\t\"........\"", "%B3:\t.ASCII\t\"........\"", "%REGS:\t.BLKL\t7"},
			call: "OTS$MOVE3_R5", jsb: true, args: []string{"#3", "%S", "%B2", "%B3"},
			show: []show{mem("%B2", 2), mem("%B3", 2), mem("%REGS", 7)}},
		probeCase{name: "MOVE5_R5 (JSB)", data: []string{"%S:\t.ASCII\t\"ABCD\"", "%B:\t.ASCII\t\"........\"", "%REGS:\t.BLKL\t7"},
			call: "OTS$MOVE5_R5", jsb: true, args: []string{"#3", "%S", "#^A\"-\"", "#5", "%B"}, rel: map[int]string{3: "%B"},
			show: []show{mem("%B", 2), mem("%REGS", 7)}},
		probeCase{name: "MOVE5_R5 short (JSB)", data: []string{"%S:\t.ASCII\t\"ABCDEF\"", "%B:\t.ASCII\t\"........\"", "%REGS:\t.BLKL\t7"},
			call: "OTS$MOVE5_R5", jsb: true, args: []string{"#6", "%S", "#^A\"-\"", "#2", "%B"}, rel: map[int]string{3: "%B"},
			show: []show{mem("%B", 2), mem("%REGS", 7)}},
	)

	// The copies.
	c = append(c,
		probeCase{name: "SCOPY_DXDX into S 5", data: outDesc("%D", 5),
			call: "OTS$SCOPY_DXDX", args: []string{"HELLOD", "%D"}, show: []show{txt("%D")}},
		probeCase{name: "SCOPY_DXDX into S 15", data: outDesc("%D", 15),
			call: "OTS$SCOPY_DXDX", args: []string{"HELLOD", "%D"}, show: []show{txt("%D")}},
		probeCase{name: "SCOPY_DXDX into D", data: []string{"%D:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0", "%Z:\t.LONG\t0"},
			call: "OTS$SCOPY_DXDX", args: []string{"HELLOD", "%D"}, post: nonzero("%D+4"),
			show: []show{txt("%D"), mem("%Z", 1)}},
		probeCase{name: "SCOPY_DXDX into VS 4", data: []string{desc("%D", 4, 14, 11, "%V"), "%V:\t.WORD\t0\n\t.ASCII\t\"????\"\n\t.WORD\t^X5555"},
			call: "OTS$SCOPY_DXDX", args: []string{"HELLOD", "%D"}, show: []show{mem("%V", 2)}},
		probeCase{name: "SCOPY_DXDX from class 3", data: append([]string{desc("%S", 3, 14, 3, "HELLO")}, outDesc("%D", 5)...),
			call: "OTS$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
		probeCase{name: "SCOPY_DXDX into class 3", data: []string{desc("%D", 4, 14, 3, "%T"), "%T:\t.ASCII\t\"----\""},
			call: "OTS$SCOPY_DXDX", args: []string{"HELLOD", "%D"}, show: []show{mem("%T", 1)}},
		probeCase{name: "SCOPY_R_DX 4", data: outDesc("%D", 6),
			call: "OTS$SCOPY_R_DX", args: []string{"#4", "HELLO+7", "%D"}, show: []show{txt("%D")}},
		probeCase{name: "SCOPY_R_DX high word", data: outDesc("%D", 6),
			call: "OTS$SCOPY_R_DX", args: []string{"#^X10003", "HELLO", "%D"}, show: []show{txt("%D")}},
		probeCase{name: "SCOPY_DXDX6 (JSB)", data: append(outDesc("%D", 5), "%REGS:\t.BLKL\t7"),
			call: "OTS$SCOPY_DXDX6", jsb: true, args: []string{"HELLOD", "%D"}, rel: map[int]string{1: "HELLO", 3: "%D_T"}, show: []show{txt("%D"), mem("%REGS", 7)}},
		probeCase{name: "SCOPY_DXDX6 longer (JSB)", data: append(outDesc("%D", 15), "%REGS:\t.BLKL\t7"),
			call: "OTS$SCOPY_DXDX6", jsb: true, args: []string{"HELLOD", "%D"}, rel: map[int]string{1: "HELLO", 3: "%D_T"}, show: []show{txt("%D"), mem("%REGS", 7)}},
		probeCase{name: "SCOPY_R_DX6 (JSB)", data: append(outDesc("%D", 2), "%REGS:\t.BLKL\t7"),
			call: "OTS$SCOPY_R_DX6", jsb: true, args: []string{"#^X10005", "HELLO", "%D"}, rel: map[int]string{3: "%D_T"}, show: []show{txt("%D"), mem("%REGS", 7)}},
	)

	// Dynamic strings, on DYN1 to DYN3 in sequence. A pointer is shown
	// only as Z: 1 if it isn't 0.
	c = append(c,
		probeCase{name: "SFREE1_DD", call: "OTS$SFREE1_DD", args: []string{"DYN1"}, show: []show{mem("DYN1", 2)}},
		probeCase{name: "SFREE1_DD empty", call: "OTS$SFREE1_DD", args: []string{"DYN1"}, show: []show{mem("DYN1", 2)}},
		probeCase{name: "SGET1_DD 10", data: []string{"%Z:\t.LONG\t0"},
			call: "OTS$SGET1_DD", args: []string{"#10", "DYN1"}, post: nonzero("DYN1+4"), show: []show{mem("DYN1", 1), mem("%Z", 1)}},
		probeCase{name: "SGET1_DD 5, kept", data: []string{"%Z:\t.LONG\t0"}, pre: []string{"MOVL\tDYN1+4,SAVEP"},
			call: "OTS$SGET1_DD", args: []string{"#5", "DYN1"},
			post: []string{"CLRL\t%Z", "CMPL\tDYN1+4,SAVEP", "BNEQ\t%NZ", "MOVL\t#1,%Z", "%NZ:"},
			show: []show{mem("DYN1", 1), mem("%Z", 1)}},
		probeCase{name: "SGET1_DD 0", data: []string{"%Z:\t.LONG\t0"},
			call: "OTS$SGET1_DD", args: []string{"#0", "DYN2"}, post: nonzero("DYN2+4"), show: []show{mem("DYN2", 1), mem("%Z", 1)}},
		probeCase{name: "SGET1_DD high word", data: []string{"%Z:\t.LONG\t0"},
			call: "OTS$SGET1_DD", args: []string{"#^X10003", "DYN3"}, post: nonzero("DYN3+4"), show: []show{mem("DYN3", 1), mem("%Z", 1)}},
		probeCase{name: "SFREEN_DD 3", call: "OTS$SFREEN_DD", args: []string{"#3", "DYN1"}, show: []show{mem("DYN1", 6)}},
		probeCase{name: "SFREE1_DD static", data: []string{"%D:\t.WORD\t4\n\t.BYTE\t14,2\n\t.ADDRESS\t%T", "%T:\t.ASCII\t\"ABCD\"", "%Z:\t.LONG\t0"},
			call: "OTS$SFREE1_DD", args: []string{"%D"}, post: nonzero("%D+4"), show: []show{mem("%D", 1), mem("%Z", 1)}},
		probeCase{name: "SGET1_DD_R6 7 (JSB)", data: []string{"%Z:\t.LONG\t0", "%REGS:\t.BLKL\t7"},
			call: "OTS$SGET1_DD_R6", jsb: true, args: []string{"#^XABCD0007", "DYN3"}, post: nonzero("DYN3+4"),
			show: []show{mem("DYN3", 1), mem("%Z", 1), mem("%REGS", 7)}},
		probeCase{name: "SFREE1_DD6 (JSB)", data: []string{"%REGS:\t.BLKL\t7"},
			call: "OTS$SFREE1_DD6", jsb: true, args: []string{"DYN3"}, show: []show{mem("DYN3", 2), mem("%REGS", 7)}},
		probeCase{name: "SGET1_DD_R6 3 (JSB)", data: []string{"%REGS:\t.BLKL\t7"},
			call: "OTS$SGET1_DD_R6", jsb: true, args: []string{"#3", "DYN2"}, show: []show{mem("DYN2", 1), mem("%REGS", 7)}},
		probeCase{name: "SFREEN_DD6 1 (JSB)", data: []string{"%REGS:\t.BLKL\t7"},
			call: "OTS$SFREEN_DD6", jsb: true, args: []string{"#1", "DYN2"}, show: []show{mem("DYN2", 2), mem("%REGS", 7)}},
	)

	return c
}

// The program's fixed parts: its data (the output line, the FAO control
// strings), the handler, and the routines that print.
const programHead = `	.TITLE	%s	%s
	.IDENT	/V1.0/

; Written by testdata/ots72/gen.go: do not edit. See gen.go and
; README.md.

	$SSDEF

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
LINLEN:	.WORD	0
	.ALIGN	LONG
LINDSC:	.WORD	256
	.BYTE	14,1
	.ADDRESS	LINE
PARTDSC:	.WORD	9
	.BYTE	14,1
	.LONG	0
OUTDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS	LINE
LINE:	.BLKB	256
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

// caseData writes the cases' data: each case's lines, name, and the
// names of the values it shows.
func caseData(b *strings.Builder, cases []probeCase) {
	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(b, "\n; %d: %s\n\t.ALIGN\tQUAD\n", n, c.name)

		for _, line := range c.data {
			b.WriteString(prefix(line, n) + "\n")
		}

		fmt.Fprintf(b, "C%d_NAME:\t.ASCIC\t\"%s\"\n", n, c.name)

		for k, sh := range c.show {
			fmt.Fprintf(b, "C%d_S%d:\t.ASCIC\t\"%s\"\n", n, k+1, shownName(sh))
		}
	}
}

// showLines writes the calls that print case n's values.
func showLines(b *strings.Builder, c probeCase, n int) {
	for k, sh := range c.show {
		switch sh.kind {
		case "mem":
			fmt.Fprintf(b, "\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#3,SHOWMEM\n",
				sh.count, prefix(sh.label, n), n, k+1)
		case "txt":
			fmt.Fprintf(b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWTXT\n", prefix(sh.label, n), n, k+1)
		}
	}
}

func program() string {
	var b strings.Builder

	fmt.Fprintf(&b, programHead, "OTS72", "Phase 72 OTS$ probe")
	b.WriteString("\n; Shared data.\n")

	for _, line := range shared {
		b.WriteString(line + "\n")
	}

	caseData(&b, cases)
	b.WriteString(programCode)

	// Each case's routine.
	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, c.name, n)

		for _, line := range c.pre {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		var post []string

		if c.jsb {
			for _, line := range registers(c.args) {
				b.WriteString("\t" + prefix(line, n) + "\n")
			}

			fmt.Fprintf(&b, "\tJSB\tG^%s\n", c.call)

			post = append(append([]string(nil), saveRegisters...), relativeArgs(c.args, c.rel)...)
		} else {
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
		showLines(&b, c, n)
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// studyCase is one of ots72t.mar's FORRTL calls: FOR$CVT_<f>_T<kind>
// (kind D, E, F, or G) of value, into a string of width bytes, with
// FORRTL's arguments digits-in-fraction, scale-factor, digits-in-int,
// digits-in-exp, and flags.
type studyCase struct {
	f                    format
	kind                 string
	value                string
	width                int
	frac, scale, intDigs int
	expDigs, flags       int
}

func (s studyCase) name() string {
	return fmt.Sprintf("%s_T%s %s %d.%d %d %d %d %d", s.f.letter, s.kind, s.value, s.width, s.frac, s.scale, s.intDigs, s.expDigs, s.flags)
}

var studyCases = []studyCase{
	{fmtD, "F", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtD, "F", "3.14159", 20, 9, 0, 0, 2, 0},
	{fmtD, "F", "-3.14159", 20, 4, 0, 0, 2, 0},
	{fmtD, "F", "0.0", 20, 4, 0, 0, 2, 0},
	{fmtD, "F", "1234567.875", 20, 2, 0, 0, 2, 0},
	{fmtD, "F", "0.000123456", 20, 8, 0, 0, 2, 0},
	{fmtD, "F", "3.14159", 20, 4, 2, 0, 2, 0},
	{fmtD, "F", "3.14159", 20, 4, 0, 0, 2, 1},
	{fmtD, "F", "1.0E20", 30, 2, 0, 0, 2, 0},
	{fmtD, "E", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtD, "E", "3.14159", 20, 9, 1, 0, 2, 0},
	{fmtD, "E", "-3.14159E-10", 20, 6, 0, 0, 2, 0},
	{fmtD, "E", "3.14159", 20, 4, -2, 0, 2, 0},
	{fmtD, "E", "3.14159", 20, 4, 0, 0, 3, 0},
	{fmtD, "E", "0.0", 20, 4, 0, 0, 2, 0},
	{fmtD, "D", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtD, "G", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtD, "G", "314159.0", 20, 4, 0, 0, 2, 0},
	{fmtD, "G", "0.0314159", 20, 4, 0, 0, 2, 0},
	{fmtD, "G", "3.14159E10", 20, 4, 0, 0, 2, 0},
	{fmtD, "G", "0.0", 20, 4, 0, 0, 2, 0},
	{fmtF, "F", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtF, "E", "3.14159", 20, 9, 0, 0, 2, 0},
	{fmtG, "F", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtG, "E", "1.0E200", 20, 4, 0, 0, 3, 0},
	{fmtG, "E", "3.14159", 30, 20, 0, 0, 2, 0},
	{fmtH, "F", "3.14159", 20, 4, 0, 0, 2, 0},
	{fmtH, "E", "1.0E2000", 20, 4, 0, 0, 4, 0},
	{fmtH, "E", "3.14159", 50, 40, 0, 0, 2, 0},
}

const studyHead = `
; The tracer's state. TRACER, the case routine's handler, takes the
; trace trap after each instruction while PSL<T> is set; when the next
; instruction is the routine at TRCPC, it resumes at CAPTURE instead,
; which records the registers and the caller's frame, puts RETCAP as the
; JSB's return address, and jumps to the routine; RETCAP records them
; again and returns to the real return address. Tracing stops once the
; routine is caught.
TRCPC:	.LONG	0		; the routine to catch
TRCSTATE:	.LONG	0		; 0 tracing, 1 caught, 2 returned
SAVERA:	.LONG	0		; the JSB's return address
SAVE1:	.BLKL	15		; R0 to R11, AP, FP, SP at the JSB
SAVE2:	.BLKL	15		; and at the RSB
VALUE1:	.BLKL	4		; 16 bytes at R0, at the JSB
FRAME1:	.BLKL	48		; R1-176 to R1+16, at the JSB
FRAME2:	.BLKL	48		; and at the RSB
TRCNAME:	.ASCIC	"JSB R0-R11 AP FP SP"
TRCNAME2:	.ASCIC	"RSB R0-R11 AP FP SP"
VALNAME:	.ASCIC	"value at R0"
STATNAME:	.ASCIC	"tracer state"
SELFVAL:	.LONG	^X11111111,^X22222222,^X33333333,^X44444444
SELFNAME:	.ASCIC	"tracer self-test"
`

const studyCode = `
; TRACER is each case routine's handler: SS$_TBIT moves the trace on
; (above), anything else is recorded and unwound as HANDLER does.
	.ENTRY	TRACER,^M<R2>
	MOVL	4(AP),R0		; the signal array
	CMPL	4(R0),#SS$_TBIT
	BEQL	1$
	CMPL	4(R0),#SS$_UNWIND	; anything else: HANDLER's work
	BEQL	9$
	MOVL	4(R0),SIGCOND
	CLRQ	SIGARG1
	CMPL	SIGCOND,#SS$_ACCVIO
	BNEQ	8$
	MOVQ	8(R0),SIGARG1
8$:	MOVL	8(AP),R1
	MOVL	SIGCOND,12(R1)
	$UNWIND_S
9$:	RET
1$:	MOVL	(R0),R1			; its count: the PC is at n-1, the PSL at n
	MOVAL	-4(R0)[R1],R2		; the PC's address
	TSTL	TRCSTATE
	BNEQ	5$			; caught already: stop tracing
	CMPL	(R2),TRCPC
	BNEQ	7$			; not yet: trace on
	MOVAB	CAPTURE,(R2)		; resume at CAPTURE
	MOVL	#1,TRCSTATE
5$:	BICL2	#^X40000010,4(R2)	; PSL<T> and <TP> off
7$:	MOVL	#SS$_CONTINUE,R0
	RET

; CAPTURE runs in the routine's place, SP at the JSB's return address.
CAPTURE:
	MOVQ	R0,SAVE1
	MOVQ	R2,SAVE1+8
	MOVQ	R4,SAVE1+16
	MOVQ	R6,SAVE1+24
	MOVQ	R8,SAVE1+32
	MOVQ	R10,SAVE1+40
	MOVL	AP,SAVE1+48
	MOVL	FP,SAVE1+52
	MOVL	SP,SAVE1+56
	MOVL	(SP),SAVERA
	MOVAB	RETCAP,(SP)
	MOVQ	(R0),VALUE1
	MOVQ	8(R0),VALUE1+8
	MOVC3	#192,-176(R1),FRAME1
	MOVQ	SAVE1,R0
	MOVQ	SAVE1+8,R2
	MOVQ	SAVE1+16,R4
	JMP	@TRCPC

; RETCAP is where the routine's RSB returns.
RETCAP:
	MOVQ	R0,SAVE2
	MOVQ	R2,SAVE2+8
	MOVQ	R4,SAVE2+16
	MOVQ	R6,SAVE2+24
	MOVQ	R8,SAVE2+32
	MOVQ	R10,SAVE2+40
	MOVL	AP,SAVE2+48
	MOVL	FP,SAVE2+52
	MOVL	SP,SAVE2+56
	MOVL	SAVE1+4,R6
	MOVC3	#192,-176(R6),FRAME2
	MOVQ	SAVE2,R0
	MOVQ	SAVE2+8,R2
	MOVQ	SAVE2+16,R4
	MOVL	SAVE2+24,R6
	MOVL	#2,TRCSTATE
	JMP	@SAVERA

; The tracer's self-test: SELFWRAP, called with PSL<T> set, JSBs to
; SELFRTN with R1 its FP, as FORRTL calls the conversion routines;
; SELFRTN sets R2 and stores a longword below R1. Caught, it shows
; tracer state 2, R2 ^X12345678 at the RSB, and ^X87654321 at -4.
	.ENTRY	SELFWRAP,^M<R2>
	SUBL2	#8,SP
	MOVL	FP,R1
	MOVAB	SELFVAL,R0
	JSB	SELFRTN
	RET
SELFRTN:
	MOVL	#^X12345678,R2
	MOVL	#^X87654321,-4(R1)
	RSB

; SHOWFRAME name, frame: the 48 longwords from R1-176, a line of four
; each, named by the first one's offset from R1 (the caller's FP).
	.ENTRY	SHOWFRAME,^M<R2,R3>
	MOVL	8(AP),R2
	MOVL	#-176,R3
1$:	PUSHL	#4
	PUSHL	R2
	MOVL	R3,FRMOFF
	$FAO_S	CTRSTR=FRMCTL,OUTLEN=FRMLEN,OUTBUF=FRMDSC,P1=4(AP),P2=FRMOFF
	MOVB	FRMLEN,FRMTXT-1
	PUSHAB	FRMTXT-1
	CALLS	#3,SHOWMEM
	ADDL2	#16,R2
	ADDL2	#16,R3
	CMPL	R3,#16
	BLSS	1$
	RET
`

const studyData = `FRMOFF:	.LONG	0
FRMLEN:	.WORD	0
	.ALIGN	LONG
FRMDSC:	.WORD	31
	.BYTE	14,1
	.ADDRESS	FRMTXT
	.BYTE	0
FRMTXT:	.BLKB	32
FRMCTL:	.ASCID	"!AC frame !SL"
FR1NAME:	.ASCIC	"JSB"
FR2NAME:	.ASCIC	"RSB"
`

// captured is the instructions that print what the tracer caught.
const captured = "\tPUSHL\t#1\n\tPUSHAB\tTRCSTATE\n\tPUSHAB\tSTATNAME\n\tCALLS\t#3,SHOWMEM\n" +
	"\tPUSHL\t#15\n\tPUSHAB\tSAVE1\n\tPUSHAB\tTRCNAME\n\tCALLS\t#3,SHOWMEM\n" +
	"\tPUSHL\t#4\n\tPUSHAB\tVALUE1\n\tPUSHAB\tVALNAME\n\tCALLS\t#3,SHOWMEM\n" +
	"\tPUSHAB\tFRAME1\n\tPUSHAB\tFR1NAME\n\tCALLS\t#2,SHOWFRAME\n" +
	"\tPUSHL\t#15\n\tPUSHAB\tSAVE2\n\tPUSHAB\tTRCNAME2\n\tCALLS\t#3,SHOWMEM\n" +
	"\tPUSHAB\tFRAME2\n\tPUSHAB\tFR2NAME\n\tCALLS\t#2,SHOWFRAME\n"

// study is ots72t.mar: each FORRTL call under the tracer, then what it
// wrote and what the tracer caught.
func study() string {
	var b strings.Builder

	fmt.Fprintf(&b, programHead, "OTS72T", "Phase 72 FORRTL conversion study")
	b.WriteString(studyHead)
	b.WriteString(studyData)

	cs := make([]probeCase, len(studyCases))

	for i, s := range studyCases {
		cs[i] = probeCase{
			name: s.name(),
			data: append([]string{fmt.Sprintf("%%V:\t%s\t%s", s.f.directive, s.value)}, outDesc("%D", s.width)...),
			show: []show{txt("%D")},
		}
	}

	caseData(&b, cs)
	b.WriteString(programCode)
	b.WriteString(studyCode)

	b.WriteString("\n; 0: the tracer's self-test\n\t.ENTRY\tC0,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tTRACER,(FP)\n" +
		"\tMOVAB\tSELFRTN,TRCPC\n\tCLRL\tTRCSTATE\n\tMOVC5\t#0,(SP),#0,#520,SAVE1\n" +
		"\tBISPSW\t#^X10\n\tCALLS\t#0,SELFWRAP\n\tBICPSW\t#^X10\n\tRET\n")

	for i, s := range studyCases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tTRACER,(FP)\n", n, s.name(), n)
		fmt.Fprintf(&b, "\tMOVAB\tG^OTS$$CVT_%s_T_R8,TRCPC\n\tCLRL\tTRCSTATE\n\tMOVC5\t#0,(SP),#0,#520,SAVE1\n", s.f.letter)

		for _, line := range push([]string{"%V", "%D", fmt.Sprintf("#%d", s.frac), fmt.Sprintf("#%d", s.scale),
			fmt.Sprintf("#%d", s.intDigs), fmt.Sprintf("#%d", s.expDigs), fmt.Sprintf("#%d", s.flags)}) {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		fmt.Fprintf(&b, "\tBISPSW\t#^X10\n\tCALLS\t#7,G^FOR$CVT_%s_T%s\n\tBICPSW\t#^X10\n\tRET\n", s.f.letter, s.kind)
	}

	b.WriteString("\n\t.ENTRY\tSTART,^M<>\n")
	b.WriteString("\n\tCLRL\tSIGCOND\t\t\t; 0: the tracer's self-test\n\tCALLS\t#0,C0\n\tPUSHL\tR0\n\tPUSHAB\tSELFNAME\n\tCALLS\t#2,SHOWR0\n")
	b.WriteString(captured)

	for i, c := range cs {
		n := i + 1
		fmt.Fprintf(&b, "\n\tCLRL\tSIGCOND\t\t\t; %d: %s\n\tCALLS\t#0,C%d\n\tPUSHL\tR0\n\tPUSHAB\tC%d_NAME\n\tCALLS\t#2,SHOWR0\n",
			n, c.name, n, n)
		showLines(&b, c, n)
		b.WriteString(captured)
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// command is ots72.com: it builds and runs both programs, each one's
// output in its own file.
const command = `$ ! OTS72.COM - the Phase 72 probe of LIBRTL's OTS$ routines.
$ ! Written by testdata/ots72/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @OTS72/OUTPUT=OTS72.LOG
$ !
$ ! OTS72.OUT and OTS72T.OUT get the programs' output; OTS72.LOG this
$ ! procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE/USER_MODE SYS$OUTPUT OTS72.MLG
$ MACRO/NOLIST OTS72
$ LINK/NOMAP OTS72
$ DEFINE/USER_MODE SYS$OUTPUT OTS72.OUT
$ RUN OTS72
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT OTS72T.MLG
$ MACRO/NOLIST OTS72T
$ LINK/NOMAP OTS72T,SYS$INPUT/OPTIONS
SYS$SHARE:FORRTL/SHAREABLE
$ DEFINE/USER_MODE SYS$OUTPUT OTS72T.OUT
$ RUN OTS72T
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE/DATE
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume, on DUA2 (the work volume's startup may mount DUA1).
const exchange = `! EXCHANGE.CMD - builds the Phase 72 OTS$ probe's exchange volume with
! govax. Written by testdata/ots72/gen.go. Run from the repository root:
!
!     govax console < testdata/ots72/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/ots72-exchange.dsk" /DEVICE=RD51 OTS72
MOUNT/WRITE DUA2 "testdata/disks/ots72-exchange.dsk"
COPY "testdata/ots72/ots72.mar"/HOST DUA2:[000000]OTS72.MAR
COPY "testdata/ots72/ots72t.mar"/HOST DUA2:[000000]OTS72T.MAR
COPY "testdata/ots72/ots72.com"/HOST DUA2:[000000]OTS72.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`

// copyout is copyout.cmd: it brings the VAX's results back into vax/.
const copyout = `! COPYOUT.CMD - copies the Phase 72 OTS$ probe's results off
! ots72-exchange.dsk into testdata/ots72/vax (README.md). Written by
! testdata/ots72/gen.go. Run from the repository root after
! @OTS72/OUTPUT=OTS72.LOG on the VAX:
!
!     govax console < testdata/ots72/copyout.cmd
!
MOUNT DUA2 "testdata/disks/ots72-exchange.dsk"
COPY DUA2:[000000]OTS72.OUT "testdata/ots72/vax/ots72-vax.txt"/HOST/QUIET
COPY DUA2:[000000]OTS72T.OUT "testdata/ots72/vax/ots72t-vax.txt"/HOST/QUIET
COPY DUA2:[000000]OTS72.LOG "testdata/ots72/vax/ots72.log"/HOST/QUIET
DISMOUNT DUA2
`

func main() {
	dir := filepath.Join("testdata", "ots72")

	for name, text := range map[string]string{
		"ots72.mar":    program(),
		"ots72t.mar":   study(),
		"ots72.com":    command,
		"exchange.cmd": exchange,
		"copyout.cmd":  copyout,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
