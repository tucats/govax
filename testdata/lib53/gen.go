//go:build ignore

// gen.go writes the Phase 53 probe of LIBRTL's instruction routines
// (docs/PHASE-53, subtasks 12 and 13): lib53.mar, a MACRO program that
// calls every routine of the phase, case by case, and prints what each
// returned and stored; lib53.com, which builds and runs it on VMS; and
// exchange.cmd, the govax console script that makes the exchange volume.
// Run it from the repository root:
//
//	go run testdata/lib53/gen.go
//
// Each case runs in a routine of its own, under a condition handler
// (HANDLER) that records the condition it's given, and an access
// violation's reason mask and address, then unwinds to the case's
// caller with the condition in R0. So the output tells a status a
// routine returned from a condition it signaled: the case's line ends
// "signal" and the condition when there was one.
//
// The output is one line per case, "name R0=xxxxxxxx", then a line per
// value it shows: longwords in hex, or a string's length and text.
// Nothing printed depends on where the program was loaded: a queue
// routine's remque-address is printed relative to the queue's header.
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
	call string   // the routine
	args []string // its arguments, first first (push)
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

// ascid is a class S text descriptor for s at label, its text after it.
func ascid(label, s string) string {
	return fmt.Sprintf("%s:\t.ASCID\t\"%s\"", label, s)
}

// queueRelative makes a queue case's remque-address relative to the
// header, so it doesn't depend on where the program was loaded.
var queueRelative = []string{"MOVAQ\tQHDR,R1", "SUBL2\tR1,QREM"}

// The data every case may use: the queue and its entries (quadword
// aligned), the 128-bit array for LIB$BBSSI and LIB$BBCCI, a table that
// uppercases letters, one that marks digits (bit 0) and letters (bit 1),
// and the CRC table.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"QHDR:\t.QUAD\t0",
	"QE1:\t.QUAD\t0",
	"QE2:\t.QUAD\t0",
	"QE3:\t.QUAD\t0",
	"QBUSY:\t.LONG\t1,0\t\t; a header with its secondary interlock set",
	"QREM:\t.LONG\t0",
	"QRETRY1:\t.LONG\t1",
	"QRETRY3:\t.LONG\t3",
	"BITS:\t.LONG\t0,0,0,0",
	"POS42:\t.LONG\t42",
	"POS104:\t.LONG\t104",
	"POSM1:\t.LONG\t-1",
	"UPCASE:",
	"\t.REPT\t97",
	"\t.BYTE\t.-UPCASE",
	"\t.ENDR",
	"\t.REPT\t26",
	"\t.BYTE\t.-UPCASE-32",
	"\t.ENDR",
	"\t.REPT\t133",
	"\t.BYTE\t.-UPCASE",
	"\t.ENDR",
	"UPDESC:\t.WORD\t256\n\t.BYTE\t14,1\n\t.ADDRESS\tUPCASE",
	"CLASSTAB:\t.BYTE\t0[48]\n\t.BYTE\t1[10]\n\t.BYTE\t0[7]\n\t.BYTE\t2[26]\n\t.BYTE\t0[165]",
	"MASK1:\t.BYTE\t1",
	"MASK2:\t.BYTE\t2",
	"MASK3:\t.BYTE\t3",
	"\t.ALIGN\tLONG",
	"CRCPOLY:\t.LONG\t^O120001\t; CRC-16 (the manual's example)",
	"CRCTAB:\t.BLKL\t16",
	"CRC0:\t.LONG\t0",
	"CRC1234:\t.LONG\t^X1234",
	"STAR:\t.ASCID\t\"*\"",
	"DOT:\t.ASCID\t\".\"",
	"ARGS:\t.LONG\t2,3,4\t\t; LIB$CALLG's argument list",
}

// The cases, in the order they run.
var cases = []probeCase{
	// Bit fields (subtask 6). The base is the field's address; the
	// manual words LIB$FFx's as the address of a longword holding it
	// (unconfirmed), which the FFS cases show.
	{name: "EXTV 0:4 of 0F", data: []string{"%F:\t.LONG\t^X0F,0", "%P:\t.LONG\t0", "%S:\t.BYTE\t4"},
		call: "LIB$EXTV", args: []string{"%P", "%S", "%F"}},
	{name: "EXTZV 0:4 of 0F", data: []string{"%F:\t.LONG\t^X0F,0", "%P:\t.LONG\t0", "%S:\t.BYTE\t4"},
		call: "LIB$EXTZV", args: []string{"%P", "%S", "%F"}},
	{name: "EXTV 30:4 across", data: []string{"%F:\t.LONG\t^XC0000000,1", "%P:\t.LONG\t30", "%S:\t.BYTE\t4"},
		call: "LIB$EXTV", args: []string{"%P", "%S", "%F"}},
	{name: "EXTV size 0", data: []string{"%F:\t.LONG\t-1,-1", "%P:\t.LONG\t3", "%S:\t.BYTE\t0"},
		call: "LIB$EXTV", args: []string{"%P", "%S", "%F"}},
	{name: "EXTZV size 32", data: []string{"%F:\t.LONG\t^X89ABCDEF,^X01234567", "%P:\t.LONG\t8", "%S:\t.BYTE\t32"},
		call: "LIB$EXTZV", args: []string{"%P", "%S", "%F"}},
	{name: "EXTV -4:8", data: []string{"%F:\t.LONG\t^XA0000000,^X0000000B", "%P:\t.LONG\t-4", "%S:\t.BYTE\t8"},
		call: "LIB$EXTV", args: []string{"%P", "%S", "%F+4"}},
	{name: "EXTZV size 33", data: []string{"%F:\t.LONG\t0,0", "%P:\t.LONG\t0", "%S:\t.BYTE\t33"},
		call: "LIB$EXTZV", args: []string{"%P", "%S", "%F"}},
	{name: "EXTZV base 0", data: []string{"%P:\t.LONG\t0", "%S:\t.BYTE\t4"},
		call: "LIB$EXTZV", args: []string{"%P", "%S", "#0"}},
	{name: "INSV 4 at 4:3", data: []string{"%F:\t.LONG\t^X0F,0", "%V:\t.LONG\t4", "%P:\t.LONG\t4", "%S:\t.BYTE\t3"},
		call: "LIB$INSV", args: []string{"%V", "%P", "%S", "%F"}, show: []show{mem("%F", 2)}},
	{name: "INSV size 0", data: []string{"%F:\t.LONG\t^X0F,0", "%V:\t.LONG\t-1", "%P:\t.LONG\t4", "%S:\t.BYTE\t0"},
		call: "LIB$INSV", args: []string{"%V", "%P", "%S", "%F"}, show: []show{mem("%F", 2)}},
	{name: "INSV 30:4 across", data: []string{"%F:\t.LONG\t0,0", "%V:\t.LONG\t^XF", "%P:\t.LONG\t30", "%S:\t.BYTE\t4"},
		call: "LIB$INSV", args: []string{"%V", "%P", "%S", "%F"}, show: []show{mem("%F", 2)}},
	{name: "INSV size 33", data: []string{"%F:\t.LONG\t0,0", "%V:\t.LONG\t1", "%P:\t.LONG\t0", "%S:\t.BYTE\t33"},
		call: "LIB$INSV", args: []string{"%V", "%P", "%S", "%F"}, show: []show{mem("%F", 2)}},
	{name: "FFS 4:4 of 4F", data: []string{"%F:\t.LONG\t^X4F,0", "%P:\t.LONG\t4", "%S:\t.BYTE\t4", "%R:\t.LONG\t-1"},
		call: "LIB$FFS", args: []string{"%P", "%S", "%F", "%R"}, show: []show{mem("%R", 1)}},
	{name: "FFC 0:4 of 0F", data: []string{"%F:\t.LONG\t^X0F,0", "%P:\t.LONG\t0", "%S:\t.BYTE\t4", "%R:\t.LONG\t-1"},
		call: "LIB$FFC", args: []string{"%P", "%S", "%F", "%R"}, show: []show{mem("%R", 1)}},
	{name: "FFS size 0", data: []string{"%F:\t.LONG\t-1,0", "%P:\t.LONG\t5", "%S:\t.BYTE\t0", "%R:\t.LONG\t-1"},
		call: "LIB$FFS", args: []string{"%P", "%S", "%F", "%R"}, show: []show{mem("%R", 1)}},
	{name: "FFS -8:8", data: []string{"%F:\t.LONG\t^X20000000,0", "%P:\t.LONG\t-8", "%S:\t.BYTE\t8", "%R:\t.LONG\t-1"},
		call: "LIB$FFS", args: []string{"%P", "%S", "%F+4", "%R"}, show: []show{mem("%R", 1)}},
	{name: "FFC 16:32 across", data: []string{"%F:\t.LONG\t-1,^XFFFFFFF0", "%P:\t.LONG\t16", "%S:\t.BYTE\t32", "%R:\t.LONG\t-1"},
		call: "LIB$FFC", args: []string{"%P", "%S", "%F", "%R"}, show: []show{mem("%R", 1)}},
	{name: "FFC size 33", data: []string{"%F:\t.LONG\t0,0", "%P:\t.LONG\t0", "%S:\t.BYTE\t33", "%R:\t.LONG\t-1"},
		call: "LIB$FFC", args: []string{"%P", "%S", "%F", "%R"}, show: []show{mem("%R", 1)}},

	// Integers (subtask 7): the manual's examples, then the edges.
	{name: "ADDX manual 1", data: []string{"%A:\t.LONG\t^X00000001,^X7FFF407F", "%B:\t.LONG\t^XFFFFFFFF,^X8000BF80", "%R:\t.LONG\t-1,-1,-1"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R"}, show: []show{mem("%R", 3)}},
	{name: "ADDX manual 2", data: []string{"%A:\t.LONG\t^X00000001,^X10000000", "%B:\t.LONG\t^XFFFFFFFF,0", "%R:\t.LONG\t-1,-1,-1"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R"}, show: []show{mem("%R", 3)}},
	{name: "SUBX manual", data: []string{"%A:\t.LONG\t^X00000001,^X10000000", "%B:\t.LONG\t^XFFFFFFFF,0", "%R:\t.LONG\t-1,-1,-1"},
		call: "LIB$SUBX", args: []string{"%A", "%B", "%R"}, show: []show{mem("%R", 3)}},
	{name: "ADDX overflow", data: []string{"%A:\t.LONG\t-1,^X7FFFFFFF", "%B:\t.LONG\t1,0", "%R:\t.LONG\t-1,-1,-1"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R"}, show: []show{mem("%R", 3)}},
	{name: "SUBX overflow", data: []string{"%A:\t.LONG\t0,^X80000000", "%B:\t.LONG\t1,0", "%R:\t.LONG\t-1,-1,-1"},
		call: "LIB$SUBX", args: []string{"%A", "%B", "%R"}, show: []show{mem("%R", 3)}},
	{name: "ADDX length 3", data: []string{"%A:\t.LONG\t-1,-1,0", "%B:\t.LONG\t1,0,0", "%R:\t.LONG\t-1,-1,-1,-1", "%L:\t.LONG\t3"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R", "%L"}, show: []show{mem("%R", 4)}},
	{name: "ADDX length 1", data: []string{"%A:\t.LONG\t^X7FFFFFFF", "%B:\t.LONG\t1", "%R:\t.LONG\t-1,-1", "%L:\t.LONG\t1"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R", "%L"}, show: []show{mem("%R", 2)}},
	{name: "ADDX length 0", data: []string{"%A:\t.LONG\t1,1", "%B:\t.LONG\t1,1", "%R:\t.LONG\t-1,-1", "%L:\t.LONG\t0"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R", "%L"}, show: []show{mem("%R", 2)}},
	{name: "ADDX length -1", data: []string{"%A:\t.LONG\t1,1", "%B:\t.LONG\t1,1", "%R:\t.LONG\t-1,-1", "%L:\t.LONG\t-1"},
		call: "LIB$ADDX", args: []string{"%A", "%B", "%R", "%L"}, show: []show{mem("%R", 2)}},
	{name: "SUBX length -1", data: []string{"%A:\t.LONG\t1,1", "%B:\t.LONG\t1,1", "%R:\t.LONG\t-1,-1", "%L:\t.LONG\t-1"},
		call: "LIB$SUBX", args: []string{"%A", "%B", "%R", "%L"}, show: []show{mem("%R", 2)}},
	{name: "ADDX into addend", data: []string{"%A:\t.LONG\t^X80000000,1"},
		call: "LIB$ADDX", args: []string{"%A", "%A", "%A"}, show: []show{mem("%A", 2)}},
	{name: "EMUL manual", data: []string{"%A:\t.LONG\t^X10000000", "%B:\t.LONG\t^X1000", "%C:\t.LONG\t0", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EMUL", args: []string{"%A", "%B", "%C", "%R"}, show: []show{mem("%R", 2)}},
	{name: "EMUL addend -1", data: []string{"%A:\t.LONG\t3", "%B:\t.LONG\t4", "%C:\t.LONG\t-1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EMUL", args: []string{"%A", "%B", "%C", "%R"}, show: []show{mem("%R", 2)}},
	{name: "EMUL -1 * 5", data: []string{"%A:\t.LONG\t-1", "%B:\t.LONG\t5", "%C:\t.LONG\t0", "%R:\t.LONG\t0,0"},
		call: "LIB$EMUL", args: []string{"%A", "%B", "%C", "%R"}, show: []show{mem("%R", 2)}},
	{name: "EDIV manual", data: []string{"%D:\t.LONG\t4096", "%Q:\t.LONG\t^X12345678,1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EDIV", args: []string{"%D", "%Q", "%R", "%R+4"}, show: []show{mem("%R", 2)}},
	{name: "EDIV -7 / 3", data: []string{"%D:\t.LONG\t3", "%Q:\t.LONG\t-7,-1", "%R:\t.LONG\t0,0"},
		call: "LIB$EDIV", args: []string{"%D", "%Q", "%R", "%R+4"}, show: []show{mem("%R", 2)}},
	{name: "EDIV overflow", data: []string{"%D:\t.LONG\t1", "%Q:\t.LONG\t^X12345678,1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EDIV", args: []string{"%D", "%Q", "%R", "%R+4"}, show: []show{mem("%R", 2)}},
	{name: "EDIV by zero", data: []string{"%D:\t.LONG\t0", "%Q:\t.LONG\t^X12345678,1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EDIV", args: []string{"%D", "%Q", "%R", "%R+4"}, show: []show{mem("%R", 2)}},
	// LIB$ADAWI: the sum and the third argument are the two words of
	// %W's first longword, the sum the low one.
	{name: "ADAWI 5 + 10", data: []string{"%A:\t.WORD\t5", "\t.ALIGN\tLONG", "%W:\t.WORD\t10,^X5555,^X5555,^X5555"},
		call: "LIB$ADAWI", args: []string{"%A", "%W", "%W+2"}, show: []show{mem("%W", 2)}},
	{name: "ADAWI overflow", data: []string{"%A:\t.WORD\t1", "\t.ALIGN\tLONG", "%W:\t.WORD\t^X7FFF,^X5555,^X5555,^X5555"},
		call: "LIB$ADAWI", args: []string{"%A", "%W", "%W+2"}, show: []show{mem("%W", 2)}},
	{name: "ADAWI -1 + 0", data: []string{"%A:\t.WORD\t-1", "\t.ALIGN\tLONG", "%W:\t.WORD\t0,^X5555,^X5555,^X5555"},
		call: "LIB$ADAWI", args: []string{"%A", "%W", "%W+2"}, show: []show{mem("%W", 2)}},
	{name: "ADAWI -1 + 1", data: []string{"%A:\t.WORD\t-1", "\t.ALIGN\tLONG", "%W:\t.WORD\t1,^X5555,^X5555,^X5555"},
		call: "LIB$ADAWI", args: []string{"%A", "%W", "%W+2"}, show: []show{mem("%W", 2)}},
	{name: "ADAWI odd sum", data: []string{"%A:\t.WORD\t1", "\t.ALIGN\tLONG", "%W:\t.WORD\t^X5555,^X5555,^X5555,^X5555"},
		call: "LIB$ADAWI", args: []string{"%A", "%W+1", "%W+4"}, show: []show{mem("%W", 2)}},

	// Floating (subtask 8): EMOD's -2.5 * 1.5 is -3 and -0.75 in each
	// format; POLY's is the manual's FORTRAN example, 27.
	{name: "EMODF -2.5*1.5", data: []string{"%M:\t.F_FLOATING\t-2.5", "%X:\t.BYTE\t0", "%D:\t.F_FLOATING\t1.5", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1"},
		call: "LIB$EMODF", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 2)}},
	{name: "EMODD -2.5*1.5", data: []string{"%M:\t.D_FLOATING\t-2.5", "%X:\t.BYTE\t0", "%D:\t.D_FLOATING\t1.5", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EMODD", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 3)}},
	{name: "EMODG -2.5*1.5", data: []string{"%M:\t.G_FLOATING\t-2.5", "%X:\t.WORD\t0", "%D:\t.G_FLOATING\t1.5", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EMODG", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 3)}},
	{name: "EMODH -2.5*1.5", data: []string{"%M:\t.H_FLOATING\t-2.5", "%X:\t.WORD\t0", "%D:\t.H_FLOATING\t1.5", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1,-1,-1,-1"},
		call: "LIB$EMODH", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 5)}},
	{name: "EMODF ext 80", data: []string{"%M:\t.F_FLOATING\t1.0", "%X:\t.BYTE\t^X80", "%D:\t.F_FLOATING\t3.0", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1"},
		call: "LIB$EMODF", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 2)}},
	{name: "EMODG ext 8000", data: []string{"%M:\t.G_FLOATING\t1.0", "%X:\t.WORD\t^X8000", "%D:\t.G_FLOATING\t3.0", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1,-1"},
		call: "LIB$EMODG", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 3)}},
	{name: "EMODF int overflow", data: []string{"%M:\t.F_FLOATING\t1.0E30", "%X:\t.BYTE\t0", "%D:\t.F_FLOATING\t1.5", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1"},
		call: "LIB$EMODF", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 2)}},
	{name: "EMODF underflow", data: []string{"%M:\t.F_FLOATING\t1.0E-30", "%X:\t.BYTE\t0", "%D:\t.F_FLOATING\t1.0E-30", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1"},
		call: "LIB$EMODF", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 2)}},
	{name: "EMODF reserved", data: []string{"%M:\t.LONG\t^X8000", "%X:\t.BYTE\t0", "%D:\t.F_FLOATING\t1.5", "%I:\t.LONG\t-1", "%R:\t.LONG\t-1"},
		call: "LIB$EMODF", args: []string{"%M", "%X", "%D", "%I", "%R"}, show: []show{mem("%I", 2)}},
	{name: "POLYF manual", data: []string{"%X:\t.F_FLOATING\t2.0", "%N:\t.WORD\t4", "%C:\t.F_FLOATING\t1.0,2.0,-1.0,1.0,-3.0", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYD manual", data: []string{"%X:\t.D_FLOATING\t2.0", "%N:\t.WORD\t4", "%C:\t.D_FLOATING\t1.0,2.0,-1.0,1.0,-3.0", "%R:\t.LONG\t-1,-1"},
		call: "LIB$POLYD", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 2)}},
	{name: "POLYG manual", data: []string{"%X:\t.G_FLOATING\t2.0", "%N:\t.WORD\t4", "%C:\t.G_FLOATING\t1.0,2.0,-1.0,1.0,-3.0", "%R:\t.LONG\t-1,-1"},
		call: "LIB$POLYG", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 2)}},
	{name: "POLYH manual", data: []string{"%X:\t.H_FLOATING\t2.0", "%N:\t.WORD\t4", "%C:\t.H_FLOATING\t1.0,2.0,-1.0,1.0,-3.0", "%R:\t.LONG\t-1,-1,-1,-1"},
		call: "LIB$POLYH", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 4)}},
	{name: "POLYF manual 2", data: []string{"%X:\t.F_FLOATING\t1.0", "%N:\t.WORD\t2", "%C:\t.F_FLOATING\t1.0,2.0,2.0", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF degree 0", data: []string{"%X:\t.F_FLOATING\t2.0", "%N:\t.WORD\t0", "%C:\t.F_FLOATING\t-3.0", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF degree 32", data: []string{"%X:\t.F_FLOATING\t2.0", "%N:\t.WORD\t32", "%C:\t.BLKF\t33", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF degree -1", data: []string{"%X:\t.F_FLOATING\t2.0", "%N:\t.WORD\t-1", "%C:\t.BLKF\t2", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF overflow", data: []string{"%X:\t.F_FLOATING\t1.0E30", "%N:\t.WORD\t2", "%C:\t.F_FLOATING\t1.0,0.0,0.0", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF underflow", data: []string{"%X:\t.F_FLOATING\t1.0E-30", "%N:\t.WORD\t2", "%C:\t.F_FLOATING\t1.0,0.0,0.0", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF underflow, +1", data: []string{"%X:\t.F_FLOATING\t1.0E-30", "%N:\t.WORD\t2", "%C:\t.F_FLOATING\t1.0,0.0,1.0", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},
	{name: "POLYF reserved coef", data: []string{"%X:\t.F_FLOATING\t2.0", "%N:\t.WORD\t1", "%C:\t.LONG\t^X4080,^X8000", "%R:\t.LONG\t-1"},
		call: "LIB$POLYF", args: []string{"%X", "%N", "%C", "%R"}, show: []show{mem("%R", 1)}},

	// Queues and interlocks (subtask 9), on one queue in sequence.
	{name: "REMQHI empty", call: "LIB$REMQHI", args: []string{"QHDR", "QREM"}, post: queueRelative,
		show: []show{mem("QREM", 1), mem("QHDR", 2)}},
	{name: "REMQTI empty", call: "LIB$REMQTI", args: []string{"QHDR", "QREM"}, post: queueRelative,
		show: []show{mem("QREM", 1), mem("QHDR", 2)}},
	{name: "INSQHI E1", call: "LIB$INSQHI", args: []string{"QE1", "QHDR"}, show: []show{mem("QHDR", 4)}},
	{name: "INSQHI E2", call: "LIB$INSQHI", args: []string{"QE2", "QHDR"}, show: []show{mem("QHDR", 6)}},
	{name: "INSQTI E3", call: "LIB$INSQTI", args: []string{"QE3", "QHDR"}, show: []show{mem("QHDR", 8)}},
	{name: "REMQTI E3", call: "LIB$REMQTI", args: []string{"QHDR", "QREM"}, post: queueRelative,
		show: []show{mem("QREM", 1), mem("QHDR", 6)}},
	{name: "REMQHI E2", call: "LIB$REMQHI", args: []string{"QHDR", "QREM"}, post: queueRelative,
		show: []show{mem("QREM", 1), mem("QHDR", 4)}},
	{name: "REMQHI E1", call: "LIB$REMQHI", args: []string{"QHDR", "QREM", "QRETRY1"}, post: queueRelative,
		show: []show{mem("QREM", 1), mem("QHDR", 2)}},
	{name: "INSQTI E1 retry 1", call: "LIB$INSQTI", args: []string{"QE1", "QHDR", "QRETRY1"}, show: []show{mem("QHDR", 4)}},
	{name: "REMQTI E1", call: "LIB$REMQTI", args: []string{"QHDR", "QREM"}, post: queueRelative,
		show: []show{mem("QREM", 1), mem("QHDR", 2)}},
	{name: "INSQHI busy", call: "LIB$INSQHI", args: []string{"QE1", "QBUSY"}, show: []show{mem("QBUSY", 2)}},
	{name: "INSQTI busy retry 3", call: "LIB$INSQTI", args: []string{"QE1", "QBUSY", "QRETRY3"}, show: []show{mem("QBUSY", 2)}},
	{name: "REMQHI busy", data: []string{"%S:\t.LONG\t^X12345678"},
		call: "LIB$REMQHI", args: []string{"QBUSY", "%S"}, show: []show{mem("%S", 1), mem("QBUSY", 2)}},
	{name: "REMQTI busy", data: []string{"%S:\t.LONG\t^X12345678"},
		call: "LIB$REMQTI", args: []string{"QBUSY", "%S"}, show: []show{mem("%S", 1), mem("QBUSY", 2)}},
	{name: "INSQHI odd entry", call: "LIB$INSQHI", args: []string{"QE1+4", "QHDR"}, show: []show{mem("QHDR", 2)}},
	{name: "INSQTI entry=header", call: "LIB$INSQTI", args: []string{"QHDR", "QHDR"}, show: []show{mem("QHDR", 2)}},
	{name: "REMQHI odd header", call: "LIB$REMQHI", args: []string{"QHDR+4", "QREM"}, show: []show{mem("QHDR", 2)}},
	{name: "REMQTI addr=header", call: "LIB$REMQTI", args: []string{"QHDR", "QHDR"}, show: []show{mem("QHDR", 2)}},
	{name: "BBSSI 42", call: "LIB$BBSSI", args: []string{"POS42", "BITS"}, show: []show{mem("BITS", 4)}},
	{name: "BBSSI 42 again", call: "LIB$BBSSI", args: []string{"POS42", "BITS"}, show: []show{mem("BITS", 4)}},
	{name: "BBCCI 42", call: "LIB$BBCCI", args: []string{"POS42", "BITS"}, show: []show{mem("BITS", 4)}},
	{name: "BBCCI 42 again", call: "LIB$BBCCI", args: []string{"POS42", "BITS"}, show: []show{mem("BITS", 4)}},
	{name: "BBSSI 104", call: "LIB$BBSSI", args: []string{"POS104", "BITS"}, show: []show{mem("BITS", 4)}},
	{name: "BBSSI -1 from +4", call: "LIB$BBSSI", args: []string{"POSM1", "BITS+4"}, show: []show{mem("BITS", 4)}},

	// Character strings (subtask 10).
	{name: "LOCC DAY", data: []string{ascid("%C", "DAY"), ascid("%S", "ONE DAY AT A TIME")},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC X", data: []string{ascid("%C", "X"), ascid("%S", "ABCDEFGHIJKLMNOPQRSTUVWXYZ")},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC none", data: []string{ascid("%C", "Q"), ascid("%S", "ABC")},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC empty source", data: []string{ascid("%C", "A"), "%S:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\t%C+8"},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC empty char", data: []string{"%C:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\t%T", "%T:\t.ASCII\t\"B\"", ascid("%S", "ABC")},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC VS source", data: []string{ascid("%C", "L"), "%S:\t.WORD\t10\n\t.BYTE\t14,11\n\t.ADDRESS\t%V", "%V:\t.WORD\t5\n\t.ASCII\t\"HELLOWORLD\""},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC class 3 source", data: []string{ascid("%C", "B"), "%S:\t.WORD\t3\n\t.BYTE\t14,3\n\t.ADDRESS\t%T", "%T:\t.ASCII\t\"ABC\""},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "LOCC source at 0", data: []string{ascid("%C", "B"), "%S:\t.WORD\t5\n\t.BYTE\t14,1\n\t.LONG\t0"},
		call: "LIB$LOCC", args: []string{"%C", "%S"}},
	{name: "SKPC blank", data: []string{ascid("%C", " "), ascid("%S", " ABC")},
		call: "LIB$SKPC", args: []string{"%C", "%S"}},
	{name: "SKPC A in AAA", data: []string{ascid("%C", "A"), ascid("%S", "AAA")},
		call: "LIB$SKPC", args: []string{"%C", "%S"}},
	{name: "SKPC empty source", data: []string{ascid("%C", "A"), "%S:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\t%C+8"},
		call: "LIB$SKPC", args: []string{"%C", "%S"}},
	{name: "MATCHC TIME", data: []string{ascid("%C", "TIME"), ascid("%S", "ONE DAY AT A TIME")},
		call: "LIB$MATCHC", args: []string{"%C", "%S"}},
	{name: "MATCHC partial first", data: []string{ascid("%C", "AAB"), ascid("%S", "AAAAB")},
		call: "LIB$MATCHC", args: []string{"%C", "%S"}},
	{name: "MATCHC none", data: []string{ascid("%C", "DAYS"), ascid("%S", "ONE DAY")},
		call: "LIB$MATCHC", args: []string{"%C", "%S"}},
	{name: "MATCHC longer sub", data: []string{ascid("%C", "ABCD"), ascid("%S", "ABC")},
		call: "LIB$MATCHC", args: []string{"%C", "%S"}},
	{name: "MATCHC empty sub", data: []string{"%C:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\t%S+8", ascid("%S", "ABC")},
		call: "LIB$MATCHC", args: []string{"%C", "%S"}},
	{name: "MATCHC both empty", data: []string{"%C:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\t%C"},
		call: "LIB$MATCHC", args: []string{"%C", "%C"}},
	{name: "SCANC digit", data: []string{ascid("%S", "AB3C")},
		call: "LIB$SCANC", args: []string{"%S", "CLASSTAB", "MASK1"}},
	{name: "SCANC none", data: []string{ascid("%S", "ABC")},
		call: "LIB$SCANC", args: []string{"%S", "CLASSTAB", "MASK1"}},
	{name: "SCANC either", data: []string{ascid("%S", "--X--")},
		call: "LIB$SCANC", args: []string{"%S", "CLASSTAB", "MASK3"}},
	{name: "SPANC digits", data: []string{ascid("%S", "123AB")},
		call: "LIB$SPANC", args: []string{"%S", "CLASSTAB", "MASK1"}},
	{name: "SPANC all letters", data: []string{ascid("%S", "ABCDE")},
		call: "LIB$SPANC", args: []string{"%S", "CLASSTAB", "MASK2"}},
	{name: "MOVC3 overlap", data: []string{"%B:\t.ASCII\t\"ABCDEFGH\"", "%L:\t.WORD\t5"},
		call: "LIB$MOVC3", args: []string{"%L", "%B", "%B+2"}, show: []show{mem("%B", 2)}},
	{name: "MOVC3 overlap down", data: []string{"%B:\t.ASCII\t\"ABCDEFGH\"", "%L:\t.WORD\t5"},
		call: "LIB$MOVC3", args: []string{"%L", "%B+2", "%B"}, show: []show{mem("%B", 2)}},
	{name: "MOVC5 3 into 6", data: []string{"%S:\t.ASCII\t\"xyz\"", "%D:\t.ASCII\t\"12345678\"", "%L:\t.WORD\t3,6", "%F:\t.ASCII\t\"*\""},
		call: "LIB$MOVC5", args: []string{"%L", "%S", "%F", "%L+2", "%D"}, show: []show{mem("%D", 2)}},
	{name: "MOVC5 6 into 2", data: []string{"%S:\t.ASCII\t\"abcdef\"", "%D:\t.ASCII\t\"12345678\"", "%L:\t.WORD\t6,2", "%F:\t.ASCII\t\"*\""},
		call: "LIB$MOVC5", args: []string{"%L", "%S", "%F", "%L+2", "%D"}, show: []show{mem("%D", 2)}},
	{name: "MOVC3 to 0", data: []string{"%S:\t.ASCII\t\"abcdef\"", "%L:\t.WORD\t4"},
		call: "LIB$MOVC3", args: []string{"%L", "%S", "#0"}},
	{name: "MOVTC into 8", data: []string{ascid("%S", "abc.de"), ascid("%D", "12345678")},
		call: "LIB$MOVTC", args: []string{"%S", "STAR", "UPDESC", "%D"}, show: []show{txt("%D")}},
	{name: "MOVTC into 3", data: []string{ascid("%S", "abc.de"), ascid("%D", "123")},
		call: "LIB$MOVTC", args: []string{"%S", "STAR", "UPDESC", "%D"}, show: []show{txt("%D")}},
	{name: "MOVTC into dynamic", data: []string{ascid("%S", "abc.de"), "%D:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0"},
		call: "LIB$MOVTC", args: []string{"%S", "STAR", "UPDESC", "%D"}, show: []show{txt("%D")}},
	{name: "MOVTC into VS 4", data: []string{ascid("%S", "abc.de"), "%D:\t.WORD\t4\n\t.BYTE\t14,11\n\t.ADDRESS\t%V", "%V:\t.WORD\t0\n\t.ASCII\t\"????\"\n\t.WORD\t^X5555"},
		call: "LIB$MOVTC", args: []string{"%S", "STAR", "UPDESC", "%D"}, show: []show{vs("%D"), mem("%V", 2)}},
	{name: "MOVTC into VS 10", data: []string{ascid("%S", "abc.de"), "%D:\t.WORD\t10\n\t.BYTE\t14,11\n\t.ADDRESS\t%V", "%V:\t.WORD\t0\n\t.ASCII\t\"??????????\""},
		call: "LIB$MOVTC", args: []string{"%S", "STAR", "UPDESC", "%D"}, show: []show{vs("%D"), mem("%V", 3)}},
	{name: "MOVTC source=dest", data: []string{ascid("%S", "abc.de")},
		call: "LIB$MOVTC", args: []string{"%S", "STAR", "UPDESC", "%S"}, show: []show{txt("%S")}},
	{name: "MOVTUC stop, fill", data: []string{ascid("%S", "abc.de"), ascid("%D", "????????")},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D", "STAR"}, show: []show{txt("%D")}},
	{name: "MOVTUC stop, no fill", data: []string{ascid("%S", "abc.de"), ascid("%D", "????????")},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D"}, show: []show{txt("%D")}},
	{name: "MOVTUC no stop", data: []string{ascid("%S", "abcde"), ascid("%D", "????????")},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D", "STAR"}, show: []show{txt("%D")}},
	{name: "MOVTUC dest full", data: []string{ascid("%S", "abc.de"), ascid("%D", "??")},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D", "STAR"}, show: []show{txt("%D")}},
	{name: "MOVTUC stop first", data: []string{ascid("%S", ".abc"), ascid("%D", "????")},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D", "STAR"}, show: []show{txt("%D")}},
	{name: "MOVTUC into dynamic", data: []string{ascid("%S", "abc.de"), "%D:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0"},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D", "STAR"}, show: []show{txt("%D")}},
	{name: "MOVTUC into VS 8", data: []string{ascid("%S", "abc.de"), "%D:\t.WORD\t8\n\t.BYTE\t14,11\n\t.ADDRESS\t%V", "%V:\t.WORD\t0\n\t.ASCII\t\"????????\"\n\t.WORD\t^X5555"},
		call: "LIB$MOVTUC", args: []string{"%S", "DOT", "UPDESC", "%D", "STAR"}, show: []show{vs("%D"), mem("%V", 3)}},

	// LIB$CALLG (subtask 2) and the CRC routines (subtask 11).
	{name: "CALLG 3+4", call: "LIB$CALLG", args: []string{"ARGS", "ADDARGS"}},
	{name: "CRC_TABLE CRC-16", call: "LIB$CRC_TABLE", args: []string{"CRCPOLY", "CRCTAB"},
		show: []show{mem("CRCTAB", 8), mem("CRCTAB+32", 8)}},
	{name: "CRC DOVE", data: []string{ascid("%S", "DOVE")}, call: "LIB$CRC", args: []string{"CRCTAB", "CRC0", "%S"}},
	{name: "CRC HOSE", data: []string{ascid("%S", "HOSE")}, call: "LIB$CRC", args: []string{"CRCTAB", "CRC0", "%S"}},
	{name: "CRC 123456789", data: []string{ascid("%S", "123456789")}, call: "LIB$CRC", args: []string{"CRCTAB", "CRC0", "%S"}},
	{name: "CRC empty, 1234", data: []string{"%S:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\tCRC0"},
		call: "LIB$CRC", args: []string{"CRCTAB", "CRC1234", "%S"}},
}

// The program's fixed parts: its data (the output line, the FAO control
// strings), the handler, and the routines that print.
const programHead = `	.TITLE	LIB53	Phase 53 LIBRTL instruction routines probe
	.IDENT	/V1.0/

; Written by testdata/lib53/gen.go: do not edit. Calls each of Phase
; 53's LIBRTL routines, case by case, each in a routine of its own under
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

; ADDARGS a, b: a + b, by value, for LIB$CALLG.
	.ENTRY	ADDARGS,^M<>
	ADDL3	4(AP),8(AP),R0
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

		for _, line := range push(c.args) {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		fmt.Fprintf(&b, "\tCALLS\t#%d,G^%s\n", len(c.args), c.call)

		for _, line := range c.post {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		b.WriteString("\tRET\n")
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

// command is lib53.com: it builds and runs the probe, the program's
// output in its own file, and copies the shareable images whose symbol
// tables Phases 54, 59, and 61 need.
const command = `$ ! LIB53.COM - the Phase 53 probe of LIBRTL's instruction routines.
$ ! Written by testdata/lib53/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB53/OUTPUT=LIB53.LOG
$ !
$ ! LIB53.OUT gets the program's output; LIB53.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE/USER_MODE SYS$OUTPUT LIB53.MLG
$ MACRO/NOLIST LIB53
$ LINK/NOMAP LIB53
$ DEFINE/USER_MODE SYS$OUTPUT LIB53.OUT
$ RUN LIB53
$ SHOW SYMBOL $STATUS
$ ! The shareable images Phases 54, 59, and 61 need the symbols of.
$ ! These are licensed files: they are erased from this volume once
$ ! their symbols are read.
$ IF F$SEARCH("SYS$SHARE:LIBRTL2.EXE") .NES. "" THEN COPY SYS$SHARE:LIBRTL2.EXE []
$ IF F$SEARCH("SYS$SHARE:CRFSHR.EXE") .NES. "" THEN COPY SYS$SHARE:CRFSHR.EXE []
$ IF F$SEARCH("SYS$LIBRARY:CRFSHR.EXE") .NES. "" .AND. F$SEARCH("CRFSHR.EXE") .EQS. "" -
	THEN COPY SYS$LIBRARY:CRFSHR.EXE []
$ DIRECTORY/SIZE/DATE
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 53 probe's exchange volume with
! govax. Written by testdata/lib53/gen.go. Run from the repository root:
!
!     govax console < testdata/lib53/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib53-exchange.dsk" /DEVICE=RD51 LIB53
MOUNT/WRITE DUA1 "testdata/disks/lib53-exchange.dsk"
COPY "testdata/lib53/lib53.mar"/HOST DUA1:[000000]LIB53.MAR
COPY "testdata/lib53/lib53.com"/HOST DUA1:[000000]LIB53.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib53")

	for name, text := range map[string]string{
		"lib53.mar":    program(),
		"lib53.com":    command,
		"exchange.cmd": exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
