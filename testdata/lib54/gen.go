//go:build ignore

// gen.go writes the Phase 54 probe of LIBRTL's string, descriptor, and
// conversion routines (docs/PHASE-54, subtask 11): lib54.mar, a MACRO
// program that calls every routine of the phase, case by case, and
// prints what each returned and stored (TestLib54Oracle compares govax's
// run with VMS's, vax/lib54-vax.txt); lib54.com, which builds and runs
// it on VMS; and exchange.cmd, the govax console script that makes the
// exchange volume. Run it from the repository root:
//
//	go run testdata/lib54/gen.go
//
// The program works as Phase 53's (testdata/lib53/gen.go) does: each
// case runs in a routine of its own, under a condition handler (HANDLER)
// that records the condition it's given and unwinds, so the output tells
// a status a routine returned from a condition it signaled. Its line is
// "name R0=xxxxxxxx", then a line per value it shows.
//
// What's new here: a JSB case loads its arguments into R0, R1, ... and
// JSBs to the routine, with R1 to R6 set to known values first, and
// shows R1 to R6 after, so the probe sees which registers the routine
// changes; a case may run instructions before its call (pre); and a case
// with no routine only shows values (the translation tables). Nothing
// printed depends on where the program was loaded: a dynamic string's
// pointer is shown only as whether it's 0 (or the same as before), and
// an address a routine returns as an offset from a label.
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

// ascid is a class S text descriptor for s at label, its text after it.
func ascid(label, s string) string {
	return fmt.Sprintf("%s:\t.ASCID\t\"%s\"", label, s)
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

// The data every case may use.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"DYN1:\t.LONG\t0,0\t\t; dynamic strings the dynamic-string cases share",
	"DYN2:\t.LONG\t0,0",
	"DYN3:\t.LONG\t0,0",
	"SAVEP:\t.LONG\t0",
	"LEN10:\t.WORD\t10",
	"LEN5:\t.WORD\t5",
	"LEN0:\t.WORD\t0",
	"LEN200:\t.WORD\t200",
	"COUNT2:\t.LONG\t2",
	"COUNT0:\t.LONG\t0",
	"TABLE:\t.BLKB\t256\t\t; a copy of a translation table, to show",
	"KEYTAB:\t$LIB_KEY_TABLE\t<<ADD,1>,<DELETE,2>,<EXIT,3>,<DEL,4>,<EXTRA,5>>",
	"KEYTAB2:\t$LIB_KEY_TABLE\t<<AB,1>,<C,2>>",
	"ODDTAB:\t.LONG\t3,0,0,0",
}

// tableDump is the cases that copy a translation table to TABLE and show
// it, 16 lines of four longwords.
func tableDump(name, table string) probeCase {
	c := probeCase{name: name, pre: []string{"MOVC3\t#256," + table + ",TABLE"}}

	for i := range 16 {
		c.show = append(c.show, mem(fmt.Sprintf("TABLE+%d", 16*i), 4))
	}

	return c
}

// The cases, in the order they run.
var cases = []probeCase{
	// Dynamic strings (subtask 3), on DYN1 to DYN3 in sequence. A
	// pointer is shown only as Z: 1 if it isn't 0.
	{name: "SGET1_DD 10", call: "LIB$SGET1_DD", args: []string{"LEN10", "DYN1"}, post: nonzero("DYN1+4"),
		data: []string{"%Z:\t.LONG\t0"}, show: []show{mem("DYN1", 1), mem("%Z", 1)}},
	{name: "SGET1_DD 5, kept", data: []string{"%Z:\t.LONG\t0"}, pre: []string{"MOVL\tDYN1+4,SAVEP"},
		call: "LIB$SGET1_DD", args: []string{"LEN5", "DYN1"},
		post: []string{"CLRL\t%Z", "CMPL\tDYN1+4,SAVEP", "BNEQ\t%NZ", "MOVL\t#1,%Z", "%NZ:"},
		show: []show{mem("DYN1", 1), mem("%Z", 1)}},
	{name: "SGET1_DD 200", call: "LIB$SGET1_DD", args: []string{"LEN200", "DYN1"}, post: nonzero("DYN1+4"),
		data: []string{"%Z:\t.LONG\t0"}, show: []show{mem("DYN1", 1), mem("%Z", 1)}},
	{name: "SGET1_DD 0", call: "LIB$SGET1_DD", args: []string{"LEN0", "DYN2"}, post: nonzero("DYN2+4"),
		data: []string{"%Z:\t.LONG\t0"}, show: []show{mem("DYN2", 1), mem("%Z", 1)}},
	{name: "SGET1_DD class S desc", data: []string{desc("%D", 3, 14, 1, "0"), "%Z:\t.LONG\t0"},
		call: "LIB$SGET1_DD", args: []string{"LEN5", "%D"}, post: nonzero("%D+4"), show: []show{mem("%D", 1), mem("%Z", 1)}},
	{name: "SFREE1_DD", call: "LIB$SFREE1_DD", args: []string{"DYN1"}, show: []show{mem("DYN1", 2)}},
	{name: "SFREE1_DD empty", call: "LIB$SFREE1_DD", args: []string{"DYN1"}, show: []show{mem("DYN1", 2)}},
	{name: "SFREE1_DD static", data: []string{"%D:\t.WORD\t4\n\t.BYTE\t14,2\n\t.ADDRESS\t%T", "%T:\t.ASCII\t\"ABCD\"", "%Z:\t.LONG\t0"},
		call: "LIB$SFREE1_DD", args: []string{"%D"}, post: nonzero("%D+4"), show: []show{mem("%D", 1), mem("%Z", 1)}},
	{name: "SGET1_DD_R6 7 (JSB)", data: []string{"%Z:\t.LONG\t0", "%REGS:\t.BLKL\t6"},
		call: "LIB$SGET1_DD_R6", jsb: true, args: []string{"#^XABCD0007", "DYN3"}, post: nonzero("DYN3+4"),
		show: []show{mem("DYN3", 1), mem("%Z", 1), mem("%REGS", 6)}},
	{name: "SFREEN_DD 2", call: "LIB$SFREEN_DD", args: []string{"COUNT2", "DYN2"}, show: []show{mem("DYN2", 4)}},
	{name: "SFREEN_DD 0", data: []string{"%D:\t.WORD\t1\n\t.BYTE\t14,2\n\t.ADDRESS\t%D", "%Z:\t.LONG\t0"},
		call: "LIB$SFREEN_DD", args: []string{"COUNT0", "%D"}, post: nonzero("%D+4"), show: []show{mem("%D", 1), mem("%Z", 1)}},
	{name: "SGET1_DD_R6 3 (JSB)", data: []string{"%REGS:\t.BLKL\t6"},
		call: "LIB$SGET1_DD_R6", jsb: true, args: []string{"#3", "DYN1"}, show: []show{mem("DYN1", 1), mem("%REGS", 6)}},
	{name: "SFREE1_DD6 (JSB)", data: []string{"%REGS:\t.BLKL\t6"},
		call: "LIB$SFREE1_DD6", jsb: true, args: []string{"DYN1"}, show: []show{mem("DYN1", 2), mem("%REGS", 6)}},
	{name: "SFREEN_DD6 1 (JSB)", data: []string{"%REGS:\t.BLKL\t6"},
		call: "LIB$SFREEN_DD6", jsb: true, args: []string{"#1", "DYN3"}, show: []show{mem("DYN3", 2), mem("%REGS", 6)}},

	// Copying and descriptors (subtask 4).
	{name: "SCOPY_DXDX into S 8", data: []string{ascid("%S", "HELLO"), ascid("%D", "12345678")},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX into S 3", data: []string{ascid("%S", "HELLO"), ascid("%D", "123")},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX into D", data: []string{ascid("%S", "HELLO"), dyn("%D")},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX into VS 4", data: []string{ascid("%S", "HELLO"), desc("%D", 4, 14, 11, "%V"), "%V:\t.WORD\t0\n\t.ASCII\t\"????\"\n\t.WORD\t^X5555"},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{vs("%D"), mem("%V", 2)}},
	{name: "SCOPY_DXDX from VS", data: []string{desc("%S", 10, 14, 11, "%V"), "%V:\t.WORD\t3\n\t.ASCII\t\"ABCDEFGHIJ\"", ascid("%D", "--------")},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX from A", data: []string{desc("%S", 1, 14, 4, "%T") + "\n\t.BYTE\t0,0,0,1\n\t.LONG\t4", "%T:\t.ASCII\t\"WXYZ\"", ascid("%D", "--------")},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX from class 3", data: []string{desc("%S", 3, 14, 3, "%T"), "%T:\t.ASCII\t\"ABC\"", ascid("%D", "--------")},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX into class 3", data: []string{ascid("%S", "HELLO"), desc("%D", 4, 14, 3, "%T"), "%T:\t.ASCII\t\"----\""},
		call: "LIB$SCOPY_DXDX", args: []string{"%S", "%D"}, show: []show{mem("%T", 1)}},
	{name: "SCOPY_R_DX 4", data: []string{"%L:\t.WORD\t4", "%T:\t.ASCII\t\"ABCDEFGH\"", ascid("%D", "------")},
		call: "LIB$SCOPY_R_DX", args: []string{"%L", "%T+2", "%D"}, show: []show{txt("%D")}},
	{name: "SCOPY_DXDX6 (JSB)", data: []string{ascid("%S", "JSB"), ascid("%D", "-----"), "%REGS:\t.BLKL\t6"},
		call: "LIB$SCOPY_DXDX6", jsb: true, args: []string{"%S", "%D"}, show: []show{txt("%D"), mem("%REGS", 6)}},
	{name: "SCOPY_R_DX6 (JSB)", data: []string{"%T:\t.ASCII\t\"ABCDEFGH\"", ascid("%D", "--"), "%REGS:\t.BLKL\t6"},
		call: "LIB$SCOPY_R_DX6", jsb: true, args: []string{"#^X10005", "%T+3", "%D"}, show: []show{txt("%D"), mem("%REGS", 6)}},
	{name: "ANALYZE_SDESC S", data: []string{ascid("%S", "HELLO"), "%L:\t.LONG\t-1", "%A:\t.LONG\t0"},
		call: "LIB$ANALYZE_SDESC", args: []string{"%S", "%L", "%A"}, post: []string{"SUBL2\t#%S+8,%A"}, show: []show{mem("%L", 2)}},
	{name: "ANALYZE_SDESC VS", data: []string{desc("%S", 10, 14, 11, "%V"), "%V:\t.WORD\t3\n\t.ASCII\t\"ABCDEFGHIJ\"", "%L:\t.LONG\t-1", "%A:\t.LONG\t0"},
		call: "LIB$ANALYZE_SDESC", args: []string{"%S", "%L", "%A"}, post: []string{"SUBL2\t#%V,%A"}, show: []show{mem("%L", 2)}},
	{name: "ANALYZE_SDESC A", data: []string{desc("%S", 1, 14, 4, "%T") + "\n\t.BYTE\t0,0,0,1\n\t.LONG\t300", "%T:\t.LONG\t0", "%L:\t.LONG\t-1", "%A:\t.LONG\t0"},
		call: "LIB$ANALYZE_SDESC", args: []string{"%S", "%L", "%A"}, post: []string{"SUBL2\t#%T,%A"}, show: []show{mem("%L", 2)}},
	{name: "ANALYZE_SDESC class 3", data: []string{desc("%S", 3, 14, 3, "%L"), "%L:\t.LONG\t-1", "%A:\t.LONG\t^X12345678"},
		call: "LIB$ANALYZE_SDESC", args: []string{"%S", "%L", "%A"}, show: []show{mem("%L", 2)}},
	{name: "ANALYZE_SDESC_R2 (JSB)", data: []string{ascid("%S", "HELLO"), "%REGS:\t.BLKL\t6"},
		call: "LIB$ANALYZE_SDESC_R2", jsb: true, args: []string{"%S"}, post: []string{"SUBL2\t#%S+8,%REGS+4"}, show: []show{mem("%REGS", 6)}},
	{name: "ANALYZE_SDESC_R2 class 3", data: []string{desc("%S", 3, 14, 3, "%S"), "%REGS:\t.BLKL\t6"},
		call: "LIB$ANALYZE_SDESC_R2", jsb: true, args: []string{"%S"}, show: []show{mem("%REGS", 6)}},

	// The string functions (subtask 5).
	{name: "LEN S", data: []string{ascid("%S", "ONE DAY")}, call: "LIB$LEN", args: []string{"%S"}},
	{name: "LEN VS", data: []string{desc("%S", 10, 14, 11, "%V"), "%V:\t.WORD\t3\n\t.ASCII\t\"ABCDEFGHIJ\""}, call: "LIB$LEN", args: []string{"%S"}},
	{name: "LEN A", data: []string{desc("%S", 1, 14, 4, "%S") + "\n\t.BYTE\t0,0,0,1\n\t.LONG\t300"}, call: "LIB$LEN", args: []string{"%S"}},
	{name: "LEN class 3", data: []string{desc("%S", 5, 14, 3, "%S")}, call: "LIB$LEN", args: []string{"%S"}},
	{name: "CHAR * into S 3", data: []string{"%C:\t.BYTE\t^A\"*\"", ascid("%D", "---")},
		call: "LIB$CHAR", args: []string{"%D", "%C"}, show: []show{txt("%D")}},
	{name: "CHAR into S 0", data: []string{"%C:\t.BYTE\t^A\"*\"", desc("%D", 0, 14, 1, "%C")},
		call: "LIB$CHAR", args: []string{"%D", "%C"}},
	{name: "CHAR E9 into D", data: []string{"%C:\t.BYTE\t^XE9", dyn("%D"), "%B:\t.LONG\t0"},
		call: "LIB$CHAR", args: []string{"%D", "%C"}, post: []string{"MOVL\t%D+4,R1", "MOVB\t(R1),%B"}, show: []show{mem("%D", 1), mem("%B", 1)}},
	{name: "ICHAR Pencil", data: []string{ascid("%S", "Pencil")}, call: "LIB$ICHAR", args: []string{"%S"}},
	{name: "ICHAR pencil", data: []string{ascid("%S", "pencil")}, call: "LIB$ICHAR", args: []string{"%S"}},
	{name: "ICHAR E9", data: []string{desc("%S", 1, 14, 1, "%T"), "%T:\t.BYTE\t^XE9"}, call: "LIB$ICHAR", args: []string{"%S"}},
	{name: "ICHAR empty", data: []string{desc("%S", 0, 14, 1, "%S")}, call: "LIB$ICHAR", args: []string{"%S"}},
	{name: "INDEX DAY", data: []string{ascid("%S", "ONE DAY AT A TIME"), ascid("%U", "DAY")}, call: "LIB$INDEX", args: []string{"%S", "%U"}},
	{name: "INDEX none", data: []string{ascid("%S", "ONE DAY AT A TIME"), ascid("%U", "DAYS")}, call: "LIB$INDEX", args: []string{"%S", "%U"}},
	{name: "INDEX empty sub", data: []string{ascid("%S", "ABC"), desc("%U", 0, 14, 1, "%S")}, call: "LIB$INDEX", args: []string{"%S", "%U"}},
	{name: "INDEX both empty", data: []string{desc("%S", 0, 14, 1, "%S")}, call: "LIB$INDEX", args: []string{"%S", "%S"}},
	{name: "INDEX empty source", data: []string{desc("%S", 0, 14, 1, "%S"), ascid("%U", "A")}, call: "LIB$INDEX", args: []string{"%S", "%U"}},

	// Translation (subtasks 2 and 6): the tables themselves, then the
	// routines.
	tableDump("AB_ASC_EBC", "G^LIB$AB_ASC_EBC"),
	tableDump("AB_EBC_ASC", "G^LIB$AB_EBC_ASC"),
	tableDump("AB_UPCASE", "G^LIB$AB_UPCASE"),
	{name: "TRA_ASC_EBC Az09", data: []string{ascid("%S", "Az09"), ascid("%D", "----")},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, show: []show{mem("%D+8", 1)}},
	{name: "TRA_ASC_EBC 80", data: []string{desc("%S", 3, 14, 1, "%T"), "%T:\t.BYTE\t^X41,^X80,^X1A", ascid("%D", "---")},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, show: []show{mem("%D+8", 1)}},
	{name: "TRA_ASC_EBC backslash", data: []string{ascid("%S", "\\"), ascid("%D", "-")},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, show: []show{mem("%D+8", 1)}},
	{name: "TRA_ASC_EBC into S 5", data: []string{ascid("%S", "Az09"), ascid("%D", "-----")},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "TRA_ASC_EBC into D", data: []string{ascid("%S", "Az09"), dyn("%D"), "%B:\t.LONG\t0"},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, post: []string{"MOVL\t%D+4,R1", "MOVL\t(R1),%B"}, show: []show{mem("%D", 1), mem("%B", 1)}},
	{name: "TRA_ASC_EBC into VS 3", data: []string{ascid("%S", "Az09"), desc("%D", 3, 14, 11, "%V"), "%V:\t.WORD\t0\n\t.ASCII\t\"???\""},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, show: []show{mem("%V", 2)}},
	{name: "TRA_ASC_EBC into VS 8", data: []string{ascid("%S", "Az09"), desc("%D", 8, 14, 11, "%V"), "%V:\t.WORD\t0\n\t.ASCII\t\"????????\""},
		call: "LIB$TRA_ASC_EBC", args: []string{"%S", "%D"}, show: []show{mem("%V", 3)}},
	{name: "TRA_EBC_ASC Az09", data: []string{desc("%S", 4, 14, 1, "%T"), "%T:\t.BYTE\t^XC1,^XA9,^XF0,^XF9", ascid("%D", "----")},
		call: "LIB$TRA_EBC_ASC", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "TRA_EBC_ASC 04 E0", data: []string{desc("%S", 3, 14, 1, "%T"), "%T:\t.BYTE\t^XC1,^X04,^XE0", ascid("%D", "---")},
		call: "LIB$TRA_EBC_ASC", args: []string{"%S", "%D"}, show: []show{txt("%D")}},
	{name: "TRA_EBC_ASC 3F", data: []string{desc("%S", 1, 14, 1, "%T"), "%T:\t.BYTE\t^X3F", ascid("%D", "-")},
		call: "LIB$TRA_EBC_ASC", args: []string{"%S", "%D"}, show: []show{mem("%D+8", 1)}},

	// Numeric text (subtask 7): the count by value, the text by
	// reference, the result longword (preset to ^X55555555).
	cvtTB("CVT_DTB 12345", "LIB$CVT_DTB", "12345", 5),
	cvtTB("CVT_DTB -42", "LIB$CVT_DTB", "-42", 3),
	cvtTB("CVT_DTB +7", "LIB$CVT_DTB", "+7", 2),
	cvtTB("CVT_DTB 4294967295", "LIB$CVT_DTB", "4294967295", 10),
	cvtTB("CVT_DTB 4294967296", "LIB$CVT_DTB", "4294967296", 10),
	cvtTB("CVT_DTB -2147483648", "LIB$CVT_DTB", "-2147483648", 11),
	cvtTB("CVT_DTB blank", "LIB$CVT_DTB", "12 3", 4),
	cvtTB("CVT_DTB 1-2", "LIB$CVT_DTB", "1-2", 3),
	cvtTB("CVT_DTB count 0", "LIB$CVT_DTB", "9", 0),
	cvtTB("CVT_DTB sign alone", "LIB$CVT_DTB", "-", 1),
	cvtTB("CVT_DTB count -1", "LIB$CVT_DTB", "9", -1),
	cvtTB("CVT_DTB 3 of 123X", "LIB$CVT_DTB", "123X", 3),
	cvtTB("CVT_HTB 7FFFFFFF", "LIB$CVT_HTB", "7FFFFFFF", 8),
	cvtTB("CVT_HTB dead", "LIB$CVT_HTB", "dead", 4),
	cvtTB("CVT_HTB -1", "LIB$CVT_HTB", "-1", 2),
	cvtTB("CVT_HTB 100000000", "LIB$CVT_HTB", "100000000", 9),
	cvtTB("CVT_OTB 777", "LIB$CVT_OTB", "777", 3),
	cvtTB("CVT_OTB 8", "LIB$CVT_OTB", "8", 1),

	// LIB$LOOKUP_KEY (subtask 8) on KEYTAB: ADD 1, DELETE 2, EXIT 3,
	// DEL 4, EXTRA 5. The key value is preset to -1, the keyword string
	// to dashes, the length to ^X5555.
	lookup("LOOKUP_KEY ADD", "ADD"),
	lookup("LOOKUP_KEY A", "A"),
	lookup("LOOKUP_KEY DELE", "DELE"),
	lookup("LOOKUP_KEY DEL", "DEL"),
	lookup("LOOKUP_KEY D", "D"),
	lookup("LOOKUP_KEY EX", "EX"),
	lookup("LOOKUP_KEY EXT", "EXT"),
	lookup("LOOKUP_KEY add", "add"),
	lookup("LOOKUP_KEY Q", "Q"),
	lookup("LOOKUP_KEY ADDS", "ADDS"),
	{name: "LOOKUP_KEY empty", data: []string{desc("%S", 0, 14, 1, "%S"), "%V:\t.LONG\t-1"},
		call: "LIB$LOOKUP_KEY", args: []string{"%S", "KEYTAB", "%V"}, show: []show{mem("%V", 1)}},
	{name: "LOOKUP_KEY into S 3", data: []string{ascid("%S", "DELE"), ascid("%K", "---"), "%L:\t.LONG\t^X5555"},
		call: "LIB$LOOKUP_KEY", args: []string{"%S", "KEYTAB", "0", "%K", "%L"}, show: []show{txt("%K"), mem("%L", 1)}},
	{name: "LOOKUP_KEY into D", data: []string{ascid("%S", "EXI"), dyn("%K")},
		call: "LIB$LOOKUP_KEY", args: []string{"%S", "KEYTAB", "0", "%K"}, show: []show{txt("%K")}},
	{name: "LOOKUP_KEY 1 argument", data: []string{ascid("%S", "ADD")},
		call: "LIB$LOOKUP_KEY", args: []string{"%S"}},
	{name: "LOOKUP_KEY odd count", data: []string{ascid("%S", "ADD")},
		call: "LIB$LOOKUP_KEY", args: []string{"%S", "ODDTAB"}},
	// $LIB_KEY_TABLE's layout: KEYTAB2 is <<AB,1>,<C,2>>; its strings'
	// addresses are shown as offsets from the table.
	{name: "$LIB_KEY_TABLE layout", data: []string{"%O:\t.LONG\t0,0"},
		post: []string{"SUBL3\t#KEYTAB2,KEYTAB2+4,%O", "SUBL3\t#KEYTAB2,KEYTAB2+12,%O+4"},
		show: []show{mem("KEYTAB2", 1), mem("KEYTAB2+8", 1), mem("KEYTAB2+16", 1), mem("%O", 2), mem("KEYTAB2+20", 2)}},

	// Locale (subtask 9): no logical names defined (the procedure checks).
	{name: "DIGIT_SEP", data: []string{ascid("%D", "---"), "%L:\t.LONG\t^X5555"},
		call: "LIB$DIGIT_SEP", args: []string{"%D", "%L"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "RADIX_POINT", data: []string{ascid("%D", "---"), "%L:\t.LONG\t^X5555"},
		call: "LIB$RADIX_POINT", args: []string{"%D", "%L"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "CURRENCY", data: []string{ascid("%D", "---"), "%L:\t.LONG\t^X5555"},
		call: "LIB$CURRENCY", args: []string{"%D", "%L"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "CURRENCY into D", data: []string{dyn("%D")}, call: "LIB$CURRENCY", args: []string{"%D"}, show: []show{txt("%D")}},
	{name: "CURRENCY into S 0", data: []string{desc("%D", 0, 14, 1, "%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$CURRENCY", args: []string{"%D", "%L"}, show: []show{mem("%L", 1)}},
	{name: "LP_LINES", call: "LIB$LP_LINES"},

	// LIB$CVT_DX_DX (subtask 10), in LIBRTL2. Data types: 2 BU, 3 WU,
	// 6 B, 7 W, 8 L, 9 Q, 10 F, 11 D, 14 T, 21 P, 27 G, 28 H, 37 VT.
	// Classes: 1 S, 2 D, 3 V, 4 A, 9 SD, 11 VS.
	cvtToText("CVT L 1234", desc("%S", 4, 8, 1, "%V"), "%V:\t.LONG\t1234", 12),
	cvtToText("CVT L -42", desc("%S", 4, 8, 1, "%V"), "%V:\t.LONG\t-42", 12),
	cvtToText("CVT L 1234 scale -2", sd("%S", 4, 8, "%V", -2), "%V:\t.LONG\t1234", 12),
	cvtToText("CVT L 5 scale -3", sd("%S", 4, 8, "%V", -3), "%V:\t.LONG\t5", 12),
	cvtToText("CVT W 12 scale 2", sd("%S", 2, 7, "%V", 2), "%V:\t.WORD\t12", 12),
	cvtToText("CVT WU FFFF", desc("%S", 2, 3, 1, "%V"), "%V:\t.WORD\t^XFFFF", 12),
	cvtToText("CVT B 80", desc("%S", 1, 6, 1, "%V"), "%V:\t.BYTE\t^X80", 12),
	cvtToText("CVT BU 80", desc("%S", 1, 2, 1, "%V"), "%V:\t.BYTE\t^X80", 12),
	cvtToText("CVT Q min", desc("%S", 8, 9, 1, "%V"), "%V:\t.LONG\t0,^X80000000", 24),
	cvtToText("CVT L 123456 into 3", desc("%S", 4, 8, 1, "%V"), "%V:\t.LONG\t123456", 3),
	cvtToText("CVT F 1.5", desc("%S", 4, 10, 1, "%V"), "%V:\t.F_FLOATING\t1.5", 20),
	cvtToText("CVT F 1.5 into 10", desc("%S", 4, 10, 1, "%V"), "%V:\t.F_FLOATING\t1.5", 10),
	cvtToText("CVT F -0.001", desc("%S", 4, 10, 1, "%V"), "%V:\t.F_FLOATING\t-0.001", 20),
	cvtToText("CVT F 0", desc("%S", 4, 10, 1, "%V"), "%V:\t.F_FLOATING\t0.0", 20),
	cvtToText("CVT D 123.25", desc("%S", 8, 11, 1, "%V"), "%V:\t.D_FLOATING\t123.25", 30),
	cvtToText("CVT G -123.456", desc("%S", 8, 27, 1, "%V"), "%V:\t.G_FLOATING\t-123.456", 30),
	cvtToText("CVT H 2", desc("%S", 16, 28, 1, "%V"), "%V:\t.H_FLOATING\t2.0", 50),
	cvtFromText("CVT -12 34 to L", " -12 34", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 12.5 to L", "12.5", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT -12.5 to L", "-12.5", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 1E3 to W", "1E3", desc("%D", 2, 7, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 12.34 to L scale -2", "12.34", sd("%D", 4, 8, "%R", -2), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 70000 to W", "70000", desc("%D", 2, 7, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT -1 to WU", "-1", desc("%D", 2, 3, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 12X to L", "12X", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT blanks to L", "   ", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 1.5 to F", "1.5", desc("%D", 4, 10, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 1D2 to D", "1D2", desc("%D", 8, 11, 1, "%R"), "%R:\t.LONG\t^X55555555,^X55555555"),
	cvtFromText("CVT 1E40 to F", "1E40", desc("%D", 4, 10, 1, "%R"), "%R:\t.LONG\t^X55555555"),
	cvtFromText("CVT 12345 to S 3", "12345", desc("%D", 3, 14, 1, "%R"), "%R:\t.ASCII\t\"---\""),
	cvtAtomic("CVT F -2.5 to L", desc("%S", 4, 10, 1, "%V"), "%V:\t.F_FLOATING\t-2.5", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT L 7 to H", desc("%S", 4, 8, 1, "%V"), "%V:\t.LONG\t7", desc("%D", 16, 28, 1, "%R"), "%R:\t.LONG\t-1,-1,-1,-1", 4),
	cvtAtomic("CVT L 300 to B", desc("%S", 4, 8, 1, "%V"), "%V:\t.LONG\t300", desc("%D", 1, 6, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT L -1 to BU", desc("%S", 4, 8, 1, "%V"), "%V:\t.LONG\t-1", desc("%D", 1, 2, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT H 1E100 to F", desc("%S", 16, 28, 1, "%V"), "%V:\t.H_FLOATING\t1.0E100", desc("%D", 4, 10, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT H 1E-100 to F", desc("%S", 16, 28, 1, "%V"), "%V:\t.H_FLOATING\t1.0E-100", desc("%D", 4, 10, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT reserved F to L", desc("%S", 4, 10, 1, "%V"), "%V:\t.LONG\t^X8000", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT VT -9 to L", desc("%S", 10, 37, 11, "%V"), "%V:\t.WORD\t2\n\t.ASCII\t\"-9xyz\"", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT class V", desc("%S", 4, 8, 3, "%V"), "%V:\t.LONG\t1", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT packed", desc("%S", 3, 21, 1, "%V"), "%V:\t.LONG\t^X1C", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT L class D", desc("%S", 4, 8, 2, "%V"), "%V:\t.LONG\t1", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT VT class S", desc("%S", 1, 37, 1, "%V"), "%V:\t.ASCII\t\"1\"", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
	cvtAtomic("CVT BU array", desc("%S", 1, 2, 4, "%V")+"\n\t.BYTE\t0,0,0,1\n\t.LONG\t2", "%V:\t.ASCII\t\"42\"", desc("%D", 4, 8, 1, "%R"), "%R:\t.LONG\t^X55555555", 1),
}

// cvtTB is a LIB$CVT_xTB case: count by value, the text, the result.
func cvtTB(name, routine, text string, count int) probeCase {
	return probeCase{name: name, data: []string{fmt.Sprintf("%%T:\t.ASCII\t\"%s\"", text), "%R:\t.LONG\t^X55555555"},
		call: routine, args: []string{fmt.Sprintf("#%d", count), "%T", "%R"}, show: []show{mem("%R", 1)}}
}

// lookup is a LIB$LOOKUP_KEY case on KEYTAB, with every output.
func lookup(name, search string) probeCase {
	return probeCase{name: name, data: []string{ascid("%S", search), "%V:\t.LONG\t-1", ascid("%K", "--------"), "%L:\t.LONG\t^X5555"},
		call: "LIB$LOOKUP_KEY", args: []string{"%S", "KEYTAB", "%V", "%K", "%L"}, show: []show{mem("%V", 1), txt("%K"), mem("%L", 1)}}
}

// cvtToText is a LIB$CVT_DX_DX case from the item %S (whose data line is
// value) to a text string of width bytes, preset to dashes.
func cvtToText(name, src, value string, width int) probeCase {
	return probeCase{name: name, data: []string{src, value, desc("%D", width, 14, 1, "%T"), fmt.Sprintf("%%T:\t.ASCII\t\"%s\"", strings.Repeat("-", width)), "%L:\t.LONG\t^X5555"},
		call: "LIB$CVT_DX_DX", args: []string{"%S", "%D", "%L"}, show: []show{txt("%D"), mem("%L", 1)}}
}

// cvtFromText is a LIB$CVT_DX_DX case from text to the item %D (whose
// data line is result), its first longwords shown.
func cvtFromText(name, text, dst, result string) probeCase {
	n := strings.Count(result, ",") + 1

	return probeCase{name: name, data: []string{ascid("%S", text), dst, result, "%L:\t.LONG\t^X5555"},
		call: "LIB$CVT_DX_DX", args: []string{"%S", "%D", "%L"}, show: []show{mem("%R", n), mem("%L", 1)}}
}

// cvtAtomic is a LIB$CVT_DX_DX case between two items, n longwords of
// the destination's data shown.
func cvtAtomic(name, src, value, dst, result string, n int) probeCase {
	return probeCase{name: name, data: []string{src, value, dst, result},
		call: "LIB$CVT_DX_DX", args: []string{"%S", "%D"}, show: []show{mem("%R", n)}}
}

// The program's fixed parts: its data (the output line, the FAO control
// strings), the handler, and the routines that print.
const programHead = `	.TITLE	LIB54	Phase 54 LIBRTL strings and conversion probe
	.IDENT	/V1.0/

; Written by testdata/lib54/gen.go: do not edit. Calls each of Phase
; 54's LIBRTL routines, case by case, each in a routine of its own under
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

// command is lib54.com: it builds and runs the probe, the program's
// output in its own file. It shows the locale logical names first: the
// program's locale cases assume none is defined.
const command = `$ ! LIB54.COM - the Phase 54 probe of LIBRTL's strings and conversion.
$ ! Written by testdata/lib54/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB54/OUTPUT=LIB54.LOG
$ !
$ ! LIB54.OUT gets the program's output; LIB54.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ SHOW LOGICAL SYS$DIGIT_SEP
$ SHOW LOGICAL SYS$RADIX_POINT
$ SHOW LOGICAL SYS$CURRENCY
$ SHOW LOGICAL SYS$LP_LINES
$ DEFINE/USER_MODE SYS$OUTPUT LIB54.MLG
$ MACRO/NOLIST LIB54
$ LINK/NOMAP LIB54
$ DEFINE/USER_MODE SYS$OUTPUT LIB54.OUT
$ RUN LIB54
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE/DATE
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 54 probe's exchange volume with
! govax. Written by testdata/lib54/gen.go. Run from the repository root:
!
!     govax console < testdata/lib54/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib54-exchange.dsk" /DEVICE=RD51 LIB54
MOUNT/WRITE DUA1 "testdata/disks/lib54-exchange.dsk"
COPY "testdata/lib54/lib54.mar"/HOST DUA1:[000000]LIB54.MAR
COPY "testdata/lib54/lib54.com"/HOST DUA1:[000000]LIB54.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib54")

	for name, text := range map[string]string{
		"lib54.mar":    program(),
		"lib54.com":    command,
		"exchange.cmd": exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
