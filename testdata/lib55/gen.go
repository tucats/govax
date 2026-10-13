//go:build ignore

// gen.go writes the Phase 55 probe of LIBRTL's service jackets, item
// routines, resource allocators, timers, miscellaneous routines, and
// protection-text routines (docs/PHASE-55, subtask 8): lib55.mar, a
// MACRO program that calls every routine of the phase, case by case,
// and prints what each returned and stored (TestLib55Oracle compares
// govax's run with VMS's, vax/lib55-vax.txt); lib55.com, which builds
// and runs it on VMS; and exchange.cmd, the govax console script that
// makes the exchange volume. Run it from the repository root:
//
//	go run testdata/lib55/gen.go
//
// The program works as Phase 54's (testdata/lib54/gen.go) does: each
// case runs in a routine of its own, under a condition handler (HANDLER)
// that records the condition it's given and unwinds, so the output tells
// a status a routine returned from a condition it signaled. Its line is
// "name R0=xxxxxxxx", then a line per value it shows.
//
// What's new here: many of the phase's values change from run to run
// (times, PIDs, counts, names of this system's own). A line showing one
// starts "  ~" (vmem, vtxt), and the oracle compares only its name, so
// the VMS run still shows the value for the record. A text whose digits
// vary but whose layout matters (SHOW_TIMER's line) is shown a second
// time with every digit made a 9 (masked). Elapsed times are shown as
// a bucket: 1 if at least so many hundredths of a second passed.
//
// The program expects LIB55_DISK to name the exchange volume's device
// (lib55.com defines it), and SYS$LANGUAGE, LIB55_NAME, and LIB55_NONE
// to be undefined.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// show is one value a case prints after its R0 line.
type show struct {
	kind  string // "mem" (longwords), "txt" (an S or D string), "vs" (a VS string), "mask" (txt, digits made 9s)
	label string // the value's address (a label; "%" is the case's prefix)
	count int    // mem: how many longwords
	vary  bool   // the value changes from run to run: "~" before its name
}

func mem(label string, n int) show  { return show{"mem", label, n, false} }
func vmem(label string, n int) show { return show{"mem", label, n, true} }
func txt(label string) show         { return show{"txt", label, 0, false} }
func vtxt(label string) show        { return show{"txt", label, 0, true} }
func vs(label string) show          { return show{"vs", label, 0, false} }
func masked(label string) show      { return show{"mask", label, 0, false} }

// probeCase is one call of one routine.
type probeCase struct {
	name string   // printed first (at most 31 characters)
	data []string // data lines; "%" is the case's label prefix
	pre  []string // instructions before the call
	call string   // the routine; "" for none (pre's R0 is the case's)
	args []string // its arguments, first first
	post []string // instructions after the call, R0 preserved
	show []show
}

// push returns the instructions that push args, last first, as a CALLS
// wants them: "#n" is pushed as a value, "=label" as the longword at
// label, "0" omits an optional argument (a 0 address), and anything else
// is an address (PUSHAB).
func push(args []string) []string {
	var lines []string

	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]

		switch {
		case a == "0":
			lines = append(lines, "PUSHL\t#0")
		case strings.HasPrefix(a, "#"):
			lines = append(lines, "PUSHL\t"+a)
		case strings.HasPrefix(a, "="):
			lines = append(lines, "PUSHL\t"+a[1:])
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

// desc is a descriptor at label: length, data type, class, then the
// pointer (a label, or 0).
func desc(label string, length, dtype, class int, ptr string) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t%d,%d\n\t.ADDRESS\t%s", label, length, dtype, class, ptr)
}

// dyn is an empty dynamic string descriptor, class D, type T.
func dyn(label string) string {
	return fmt.Sprintf("%s:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0", label)
}

// fixed is a class S string of n dashes at label (its text at label_T).
func fixed(label string, n int) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T\n%s_T:\t.ASCII\t\"%s\"",
		label, n, label, label, strings.Repeat("-", n))
}

// varying is a class VS string of at most n bytes at label, CURLEN
// preset to ^X5555 and its text to dashes.
func varying(label string, n int) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,11\n\t.ADDRESS\t%s_V\n%s_V:\t.WORD\t^X5555\n\t.ASCII\t\"%s\"",
		label, n, label, label, strings.Repeat("-", n))
}

// nonzero sets %Z to 1 when the longword at addr isn't 0 (a value's
// presence, which is all the probe may show of it).
func nonzero(addr string) []string {
	return []string{"CLRL\t%Z", "TSTL\t" + addr, "BEQL\t%NZ", "MOVL\t#1,%Z", "%NZ:"}
}

// startClock is the pre instructions that record the time in %T0;
// elapsed's post instructions compute the hundredths since then in %H
// and set %Q to 1 if they're at least least.
var startClock = []string{"$GETTIM_S\tTIMADR=%T0"}

func elapsed(least int) []string {
	return []string{
		"$GETTIM_S\tTIMADR=%T1",
		"MOVQ\t%T1,R1", "SUBL2\t%T0,R1", "SBWC\t%T0+4,R2",
		"EDIV\t#100000,R1,%H,R3",
		"CLRL\t%Q", fmt.Sprintf("CMPL\t%%H,#%d", least), "BLSS\t%QX", "MOVL\t#1,%Q", "%QX:",
	}
}

// clockData is the data elapsed uses.
var clockData = []string{"%T0:\t.QUAD\t0", "%T1:\t.QUAD\t0", "%H:\t.LONG\t0", "%Q:\t.LONG\t0"}

// vmsTime is t as a VMS absolute time: 100ns units since 17-NOV-1858.
func vmsTime(t time.Time) int64 {
	return int64(t.Sub(time.Date(1858, 11, 17, 0, 0, 0, 0, time.UTC)) / 100)
}

// quad is a .LONG pair for the quadword v.
func quad(v int64) string {
	return fmt.Sprintf(".LONG\t^X%08X,^X%08X", uint32(v), uint32(uint64(v)>>32))
}

// The fixed times the jackets format: an absolute time, a delta of
// 1 02:03:04.05, and a delta of 10,001 days (too long).
var (
	absTime  = vmsTime(time.Date(2026, 10, 13, 12, 34, 56, 780_000_000, time.UTC))
	deltaOK  = -int64(((1*24+2)*3600+3*60+4)*10_000_000 + 5*100_000)
	deltaBad = -int64(10_001) * 86_400 * 10_000_000
)

// vector is a vector of class S descriptors at label, one for each of
// names (an empty name a zero-length descriptor), the texts after it.
func vector(label string, names ...string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s:", label)

	for i, name := range names {
		fmt.Fprintf(&b, "\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_%d\n", len(name), label, i)
	}

	for i, name := range names {
		fmt.Fprintf(&b, "%s_%d:\t.ASCII\t\"%s\"\n", label, i, name)
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// The data every case may use.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"ABSTIM:\t" + quad(absTime),
	"DELTIM:\t" + quad(deltaOK),
	"BADTIM:\t" + quad(deltaBad),
	"ONE:\t.LONG\t1",
	"ZERO:\t.LONG\t0",
	"FIFTEEN:\t.LONG\t15",
	"ACCNAM:\t.LONG\t0\t\t; LIB$GET_ACCNAM's FILE table, if VMS has it",
	"HANDLE:\t.LONG\t0\t\t; a heap timer block",
	"ASTVAL:\t.LONG\t^X55555555\t; LIB$AST_IN_PROG's value in an AST",
	"TIMSTAT:\t.LONG\t1\t\t; what TIMACT returns",
	"TIMARG:\t.LONG\t^X55555555\t; TIMACT's second argument",
	"TIMCNT:\t.LONG\t0\t\t; TIMACT's argument count",
	"TIMDSC:\t.WORD\t0\t\t; TIMACT's copy of the line",
	"\t.BYTE\t14,1",
	"\t.ADDRESS\tTIMBUF",
	"TIMBUF:\t.BLKB\t200",
	"DISKNAM:\t.ASCID\t\"LIB55_DISK\"",
	"PROCTAB:\t.ASCID\t\"LNM$PROCESS_TABLE\"",
	"LANGNAM:\t.ASCID\t\"SYS$LANGUAGE\"",
	"TRNNAM:\t.ASCID\t\"LIB55_NAME\"",
	"FRENCH:\t.ASCII\t\"FRENCH\"",
	"HELLO:\t.ASCII\t\"Hello World\"",
	"LANGITM:\t.WORD\t6,LNM$_STRING",
	"\t.ADDRESS\tFRENCH",
	"\t.LONG\t0,0",
	"TRNITM:\t.WORD\t11,LNM$_STRING",
	"\t.ADDRESS\tHELLO",
	"\t.LONG\t0,0",
	// Custom ownership names and separators for FORMAT_SOGW_PROT.
	vector("OWNNAMS", "S", "O", "G", "W"),
	"EQUALS:\t.ASCID\t\"=\"",
	"COMMA:\t.ASCID\t\",\"",
	// An access-name table of the program's own: ALPHA, BETA, an
	// undefined access, DELTA.
	vector("MYTABLE", "ALPHA", "BETA", "", "DELTA"),
}

// The cases, in the order they run.
var cases = []probeCase{
	// The jackets (subtask 2).
	{name: "SYS_FAO into D", data: []string{ascid("%C", "!UL-!AS"), ascid("%A", "abc"), dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D", "#42", "%A"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_FAO into S 12", data: []string{ascid("%C", "!UL-!AS"), ascid("%A", "abc"), fixed("%D", 12), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D", "#42", "%A"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_FAO into S 4", data: []string{ascid("%C", "!UL-!AS"), ascid("%A", "abc"), fixed("%D", 4), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D", "#42", "%A"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_FAO into VS 4", data: []string{ascid("%C", "!UL-!AS"), ascid("%A", "abc"), varying("%D", 4), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D", "#42", "%A"}, show: []show{vs("%D"), mem("%L", 1)}},
	{name: "SYS_FAO into VS 20", data: []string{ascid("%C", "!UL-!AS"), ascid("%A", "abc"), varying("%D", 20), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D", "#42", "%A"}, show: []show{vs("%D"), mem("%L", 1)}},
	{name: "SYS_FAO 300 into D", data: []string{ascid("%C", "!300*X"), dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D"}, show: []show{mem("%D", 1), mem("%L", 1)}},
	{name: "SYS_FAO 300 into VS 400", data: []string{ascid("%C", "!300*X"), varying("%D", 400), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D"}, show: []show{mem("%D_V", 1), mem("%L", 1)}},
	{name: "SYS_FAO 300 into S 400", data: []string{ascid("%C", "!300*X"), fixed("%D", 400), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAO", args: []string{"%C", "%L", "%D"}, show: []show{mem("%L", 1)}},
	{name: "SYS_FAO no length", data: []string{ascid("%C", "!XW"), dyn("%D")},
		call: "LIB$SYS_FAO", args: []string{"%C", "0", "%D", "#^XABCD"}, show: []show{txt("%D")}},
	{name: "SYS_FAOL into D", data: []string{ascid("%C", "!UL/!UL/!AS"), ascid("%A", "xyz"), "%P:\t.LONG\t7,8\n\t.ADDRESS\t%A", dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAOL", args: []string{"%C", "%L", "%D", "%P"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_FAOL into S 3", data: []string{ascid("%C", "!UL/!UL"), "%P:\t.LONG\t7,8", fixed("%D", 3), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_FAOL", args: []string{"%C", "%L", "%D", "%P"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM into D", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "ABSTIM"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM into S 30", data: []string{fixed("%D", 30), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "ABSTIM"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM into S 12", data: []string{fixed("%D", 12), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "ABSTIM"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM into VS 8", data: []string{varying("%D", 8), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "ABSTIM"}, show: []show{vs("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM flags 1", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "ABSTIM", "ONE"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM delta", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "DELTIM"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM delta flags 1", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "DELTIM", "ONE"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM delta into S 20", data: []string{fixed("%D", 20), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "DELTIM"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_ASCTIM 10001 days", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D", "BADTIM"}, show: []show{mem("%D", 1), mem("%L", 1)}},
	{name: "SYS_ASCTIM now", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_ASCTIM", args: []string{"%L", "%D"}, show: []show{vtxt("%D"), mem("%L", 1)}},
	{name: "SYS_GETMSG NOPRIV", data: []string{"%I:\t.LONG\tSS$_NOPRIV", dyn("%D"), "%L:\t.LONG\t^X5555", "%A:\t.LONG\t^X55555555"},
		call: "LIB$SYS_GETMSG", args: []string{"%I", "%L", "%D", "0", "%A"}, show: []show{txt("%D"), mem("%L", 1), mem("%A", 1)}},
	{name: "SYS_GETMSG flags 1", data: []string{"%I:\t.LONG\tSS$_NOPRIV", dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_GETMSG", args: []string{"%I", "%L", "%D", "ONE"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_GETMSG ACCVIO array", data: []string{"%I:\t.LONG\tSS$_ACCVIO", dyn("%D"), "%A:\t.LONG\t^X55555555"},
		call: "LIB$SYS_GETMSG", args: []string{"%I", "0", "%D", "FIFTEEN", "%A"}, show: []show{txt("%D"), mem("%A", 1)}},
	{name: "SYS_GETMSG into S 10", data: []string{"%I:\t.LONG\tSS$_NOPRIV", fixed("%D", 10), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_GETMSG", args: []string{"%I", "%L", "%D"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_GETMSG unknown", data: []string{"%I:\t.LONG\t^X0FFF8002", dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_GETMSG", args: []string{"%I", "%L", "%D"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "define LIB55_NAME", data: []string{"%S:\t.LONG\t0"},
		pre:  []string{"$CRELNM_S\tTABNAM=PROCTAB,LOGNAM=TRNNAM,ITMLST=TRNITM", "MOVL\tR0,%S"},
		show: []show{mem("%S", 1)}},
	{name: "SYS_TRNLOG into D", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555", "%T:\t.LONG\t^X55555555", "%M:\t.LONG\t^X55555555"},
		call: "LIB$SYS_TRNLOG", args: []string{"TRNNAM", "%L", "%D", "%T", "%M"}, show: []show{txt("%D"), mem("%L", 1), mem("%T", 1), mem("%M", 1)}},
	{name: "SYS_TRNLOG into S 5", data: []string{fixed("%D", 5), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_TRNLOG", args: []string{"TRNNAM", "%L", "%D"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_TRNLOG into VS 30", data: []string{varying("%D", 30), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_TRNLOG", args: []string{"TRNNAM", "%L", "%D"}, show: []show{vs("%D"), mem("%L", 1)}},
	{name: "SYS_TRNLOG dsbmsk 2", data: []string{dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_TRNLOG", args: []string{"TRNNAM", "%L", "%D", "0", "0", "#2"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "SYS_TRNLOG undefined", data: []string{ascid("%N", "LIB55_NONE"), dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$SYS_TRNLOG", args: []string{"%N", "%L", "%D"}, show: []show{txt("%D"), mem("%L", 1)}},

	// The item routines (subtask 3).
	jpi("GETJPI STATE", "JPI$_STATE", false),
	jpi("GETJPI MODE", "JPI$_MODE", false),
	jpi("GETJPI ASTEN", "JPI$_ASTEN", false),
	jpi("GETJPI ASTACT", "JPI$_ASTACT", false),
	jpi("GETJPI PRCCNT", "JPI$_PRCCNT", false),
	jpi("GETJPI PID", "JPI$_PID", true),
	jpi("GETJPI UIC", "JPI$_UIC", true),
	jpi("GETJPI CURPRIV", "JPI$_CURPRIV", true),
	jpi("GETJPI LOGINTIM", "JPI$_LOGINTIM", true),
	jpi("GETJPI PRI", "JPI$_PRI", true),
	jpi("GETJPI OWNER", "JPI$_OWNER", false),
	jpi("GETJPI USERNAME", "JPI$_USERNAME", true),
	jpi("GETJPI TERMINAL", "JPI$_TERMINAL", true),
	{name: "GETJPI USERNAME no string", data: []string{"%I:\t.LONG\tJPI$_USERNAME", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "0", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETJPI STATE no outputs", data: []string{"%I:\t.LONG\tJPI$_STATE"},
		call: "LIB$GETJPI", args: []string{"%I"}},
	{name: "GETJPI MODE into S 5", data: []string{"%I:\t.LONG\tJPI$_MODE", fixed("%D", 5), "%L:\t.LONG\t^X5555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "0", "0", "%D", "%L"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "GETJPI STATE into VS 2", data: []string{"%I:\t.LONG\tJPI$_STATE", varying("%D", 2), "%L:\t.LONG\t^X5555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "0", "0", "%D", "%L"}, show: []show{vs("%D"), mem("%L", 1)}},
	{name: "GETJPI pid 0 written", data: []string{"%I:\t.LONG\tJPI$_STATE", "%P:\t.LONG\t0", "%Z:\t.LONG\t0", "%E:\t.LONG\t0"},
		call: "LIB$GETJPI", args: []string{"%I", "%P"}, post: append(nonzero("%P"), "$GETJPIW_S\tITMLST=%IL", "CMPL\t%P,%MY", "BNEQ\t%NE", "MOVL\t#1,%E", "%NE:"),
		show: []show{mem("%Z", 1), mem("%E", 1)}},
	{name: "GETJPI no such name", data: []string{"%I:\t.LONG\tJPI$_STATE", ascid("%N", "LIB55_NOSUCH"), "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "%N", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETJPI no such pid", data: []string{"%I:\t.LONG\tJPI$_STATE", "%P:\t.LONG\t^X7FFF0000", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETJPI", args: []string{"%I", "%P", "0", "%V"}, show: []show{mem("%P", 1), mem("%V", 1)}},
	{name: "GETJPI item 0", data: []string{"%I:\t.LONG\t0", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "0", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETJPI no arguments", call: "LIB$GETJPI"},
	{name: "GETJPI 7 arguments", data: []string{"%I:\t.LONG\tJPI$_STATE", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "0", "%V", "0", "0", "0"}, show: []show{mem("%V", 1)}},
	dvi("GETDVI DEVCLASS", "DVI$_DEVCLASS", false),
	dvi("GETDVI ACPTYPE", "DVI$_ACPTYPE", false),
	dvi("GETDVI VPROT", "DVI$_VPROT", false),
	dvi("GETDVI OWNUIC", "DVI$_OWNUIC", false),
	dvi("GETDVI MNT", "DVI$_MNT", false),
	dvi("GETDVI REC", "DVI$_REC", false),
	dvi("GETDVI PID", "DVI$_PID", false),
	dvi("GETDVI MAXBLOCK", "DVI$_MAXBLOCK", false),
	dvi("GETDVI VOLNAM", "DVI$_VOLNAM", false),
	dvi("GETDVI DEVNAM", "DVI$_DEVNAM", true),
	dvi("GETDVI ACPPID", "DVI$_ACPPID", true),
	{name: "GETDVI by channel", data: []string{"%I:\t.LONG\tDVI$_ACPTYPE", "%C:\t.LONG\t0", dyn("%D"), "%S:\t.LONG\t0"},
		pre:  []string{"$ASSIGN_S\tDEVNAM=DISKNAM,CHAN=%C", "MOVL\tR0,%S"},
		call: "LIB$GETDVI", args: []string{"%I", "%C", "0", "0", "%D"}, post: []string{"$DASSGN_S\tCHAN=%C"},
		show: []show{mem("%S", 1), txt("%D")}},
	{name: "GETDVI channel and bad name", data: []string{"%I:\t.LONG\tDVI$_DEVCLASS", "%C:\t.LONG\t0", ascid("%N", "LIB55_NOSUCH:"), "%V:\t.LONG\t^X55555555"},
		pre:  []string{"$ASSIGN_S\tDEVNAM=DISKNAM,CHAN=%C"},
		call: "LIB$GETDVI", args: []string{"%I", "%C", "%N", "%V"}, post: []string{"$DASSGN_S\tCHAN=%C"},
		show: []show{mem("%V", 1)}},
	{name: "GETDVI neither", data: []string{"%I:\t.LONG\tDVI$_DEVCLASS", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETDVI", args: []string{"%I", "0", "0", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETDVI no such device", data: []string{"%I:\t.LONG\tDVI$_DEVCLASS", ascid("%N", "XYZ7:"), "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETDVI", args: []string{"%I", "0", "%N", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETDVI name with file", data: []string{"%I:\t.LONG\tDVI$_DEVCLASS", ascid("%N", "LIB55_DISK:[000000]X.Y"), "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETDVI", args: []string{"%I", "0", "%N", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETDVI NL DEVCLASS", data: []string{"%I:\t.LONG\tDVI$_DEVCLASS", ascid("%N", "NLA0:"), "%V:\t.LONG\t^X55555555", dyn("%D")},
		call: "LIB$GETDVI", args: []string{"%I", "0", "%N", "%V", "%D"}, show: []show{mem("%V", 1), txt("%D")}},
	{name: "GETDVI NL MNT", data: []string{"%I:\t.LONG\tDVI$_MNT", ascid("%N", "NLA0:"), "%V:\t.LONG\t^X55555555", dyn("%D")},
		call: "LIB$GETDVI", args: []string{"%I", "0", "%N", "%V", "%D"}, show: []show{mem("%V", 1), txt("%D")}},
	{name: "GETDVI NL ACPTYPE", data: []string{"%I:\t.LONG\tDVI$_ACPTYPE", ascid("%N", "NLA0:"), "%V:\t.LONG\t^X55555555", dyn("%D")},
		call: "LIB$GETDVI", args: []string{"%I", "0", "%N", "%V", "%D"}, show: []show{mem("%V", 1), txt("%D")}},
	syi("GETSYI CLUSTER_MEMBER", "SYI$_CLUSTER_MEMBER", false),
	syi("GETSYI NODE_CSID", "SYI$_NODE_CSID", false),
	syi("GETSYI VERSION", "SYI$_VERSION", true),
	syi("GETSYI BOOTTIME", "SYI$_BOOTTIME", true),
	syi("GETSYI NODENAME", "SYI$_NODENAME", true),
	syi("GETSYI CPU", "SYI$_CPU", true),
	{name: "GETSYI csid 0 no node", data: []string{"%I:\t.LONG\tSYI$_NODE_CSID", "%C:\t.LONG\t0", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETSYI", args: []string{"%I", "%V", "0", "0", "%C"}, show: []show{mem("%C", 1), mem("%V", 1)}},
	{name: "GETSYI no such node", data: []string{"%I:\t.LONG\tSYI$_NODE_CSID", ascid("%N", "NOSUCH"), "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETSYI", args: []string{"%I", "%V", "0", "0", "0", "%N"}, show: []show{mem("%V", 1)}},
	{name: "GETSYI VERSION no string", data: []string{"%I:\t.LONG\tSYI$_VERSION", "%V:\t.LONG\t^X55555555"},
		call: "LIB$GETSYI", args: []string{"%I", "%V"}, show: []show{mem("%V", 1)}},
	{name: "GETSYI no arguments", call: "LIB$GETSYI"},

	// Event flags and LUNs (subtask 4), from the image's initial state.
	ef("GET_EF", "LIB$GET_EF", "0"),
	ef("GET_EF again", "LIB$GET_EF", "0"),
	ef("RESERVE_EF 1", "LIB$RESERVE_EF", "1"),
	ef("RESERVE_EF 23", "LIB$RESERVE_EF", "23"),
	ef("RESERVE_EF 63", "LIB$RESERVE_EF", "63"),
	ef("RESERVE_EF 63 again", "LIB$RESERVE_EF", "63"),
	ef("FREE_EF 63", "LIB$FREE_EF", "63"),
	ef("FREE_EF 63 again", "LIB$FREE_EF", "63"),
	ef("FREE_EF 5", "LIB$FREE_EF", "5"),
	ef("FREE_EF 0", "LIB$FREE_EF", "0"),
	ef("FREE_EF 24", "LIB$FREE_EF", "24"),
	ef("RESERVE_EF 31", "LIB$RESERVE_EF", "31"),
	ef("FREE_EF 64", "LIB$FREE_EF", "64"),
	ef("RESERVE_EF -1", "LIB$RESERVE_EF", "-1"),
	ef("GET_EF after FREE_EF 5", "LIB$GET_EF", "0"),
	ef("FREE_EF 6", "LIB$FREE_EF", "6"),
	ef("FREE_EF 7", "LIB$FREE_EF", "7"),
	ef("GET_EF after FREE_EF 6 7", "LIB$GET_EF", "0"),
	exhaust("GET_EF until none", "LIB$GET_EF"),
	freeAll("FREE_EF 32 to 63", "LIB$FREE_EF", 32, 63),
	ef("GET_LUN", "LIB$GET_LUN", "0"),
	ef("GET_LUN again", "LIB$GET_LUN", "0"),
	ef("FREE_LUN 100", "LIB$FREE_LUN", "100"),
	ef("FREE_LUN 100 again", "LIB$FREE_LUN", "100"),
	ef("FREE_LUN 99", "LIB$FREE_LUN", "99"),
	ef("FREE_LUN 120", "LIB$FREE_LUN", "120"),
	ef("GET_LUN after FREE_LUN 100", "LIB$GET_LUN", "0"),
	exhaust("GET_LUN until none", "LIB$GET_LUN"),
	freeAll("FREE_LUN 100 to 119", "LIB$FREE_LUN", 100, 119),

	// Timers (subtask 5). The static block first.
	{name: "STAT_TIMER before INIT", data: []string{"%C:\t.LONG\t2", "%V:\t.LONG\t^X55555555"},
		call: "LIB$STAT_TIMER", args: []string{"%C", "%V"}, show: []show{mem("%V", 1)}},
	{name: "SHOW_TIMER before INIT", pre: []string{"CLRW\tTIMDSC"},
		call: "LIB$SHOW_TIMER", args: []string{"0", "0", "TIMACT"}, show: []show{mem("TIMDSC", 1)}},
	{name: "INIT_TIMER static", call: "LIB$INIT_TIMER"},
	{name: "STAT_TIMER 1", data: []string{"%C:\t.LONG\t1", "%V:\t.LONG\t^X55555555,^X55555555", "%N:\t.LONG\t0"},
		call: "LIB$STAT_TIMER", args: []string{"%C", "%V"},
		post: []string{"CLRL\t%N", "TSTL\t%V+4", "BGEQ\t%P", "MOVL\t#1,%N", "%P:"},
		show: []show{mem("%N", 1), vmem("%V", 2)}},
	statTimer("STAT_TIMER 2", 2),
	statTimer("STAT_TIMER 3", 3),
	statTimer("STAT_TIMER 4", 4),
	statTimer("STAT_TIMER 5", 5),
	statTimer("STAT_TIMER 0", 0),
	statTimer("STAT_TIMER 6", 6),
	showTimer("SHOW_TIMER", "0", "#^X1234"),
	showTimer("SHOW_TIMER 1", "%C", "0", "%C:\t.LONG\t1"),
	showTimer("SHOW_TIMER 2", "%C", "0", "%C:\t.LONG\t2"),
	showTimer("SHOW_TIMER 3", "%C", "0", "%C:\t.LONG\t3"),
	showTimer("SHOW_TIMER 4", "%C", "0", "%C:\t.LONG\t4"),
	showTimer("SHOW_TIMER 5", "%C", "0", "%C:\t.LONG\t5"),
	showTimer("SHOW_TIMER code 0", "%C", "0", "%C:\t.LONG\t0"),
	showTimer("SHOW_TIMER 6", "%C", "0", "%C:\t.LONG\t6"),
	{name: "SHOW_TIMER action's status", pre: []string{"MOVL\t#^X10000003,TIMSTAT"},
		call: "LIB$SHOW_TIMER", args: []string{"0", "0", "TIMACT"}, post: []string{"MOVL\t#1,TIMSTAT"}},
	{name: "SHOW_TIMER action failing", pre: []string{"MOVL\t#^X10000002,TIMSTAT"},
		call: "LIB$SHOW_TIMER", args: []string{"0", "0", "TIMACT"}, post: []string{"MOVL\t#1,TIMSTAT"}},
	{name: "INIT_TIMER heap", data: []string{"%Z:\t.LONG\t0"},
		call: "LIB$INIT_TIMER", args: []string{"HANDLE"}, post: nonzero("HANDLE"), show: []show{mem("%Z", 1)}},
	{name: "heap block", data: []string{"%B:\t.LONG\t0,0,0,0,0,0,0,0"},
		pre:  []string{"MOVL\tHANDLE,R1", "BEQL\t%X", "MOVC3\t#32,(R1),%B", "%X:\tCLRL\tR0"},
		show: []show{vmem("%B", 8)}},
	{name: "STAT_TIMER heap 3", data: []string{"%C:\t.LONG\t3", "%V:\t.LONG\t^X55555555"},
		call: "LIB$STAT_TIMER", args: []string{"%C", "%V", "HANDLE"}, show: []show{vmem("%V", 1)}},
	{name: "SHOW_TIMER heap", pre: []string{"CLRW\tTIMDSC"},
		call: "LIB$SHOW_TIMER", args: []string{"HANDLE", "0", "TIMACT"}, show: []show{masked("TIMDSC")}},
	{name: "INIT_TIMER heap again", data: []string{"%S:\t.LONG\t0", "%E:\t.LONG\t0"},
		pre: []string{"MOVL\tHANDLE,%S"}, call: "LIB$INIT_TIMER", args: []string{"HANDLE"},
		post: []string{"CLRL\t%E", "CMPL\tHANDLE,%S", "BNEQ\t%X", "MOVL\t#1,%E", "%X:"}, show: []show{mem("%E", 1)}},
	{name: "INIT_TIMER not a block", data: []string{"%H:\t.LONG\t^X200"},
		call: "LIB$INIT_TIMER", args: []string{"%H"}, show: []show{mem("%H", 1)}},
	{name: "STAT_TIMER not a block", data: []string{"%C:\t.LONG\t3", "%V:\t.LONG\t^X55555555", "%H:\t.LONG\t^X200"},
		call: "LIB$STAT_TIMER", args: []string{"%C", "%V", "%H"}, show: []show{mem("%V", 1)}},
	{name: "FREE_TIMER", call: "LIB$FREE_TIMER", args: []string{"HANDLE"}, show: []show{mem("HANDLE", 1)}},
	{name: "FREE_TIMER again", call: "LIB$FREE_TIMER", args: []string{"HANDLE"}, show: []show{mem("HANDLE", 1)}},
	{name: "SHOW_TIMER handle 0", pre: []string{"CLRW\tTIMDSC"},
		call: "LIB$SHOW_TIMER", args: []string{"HANDLE", "0", "TIMACT"}, show: []show{mem("TIMDSC", 1)}},

	// The miscellaneous routines (subtask 6).
	{name: "AST_IN_PROG", call: "LIB$AST_IN_PROG"},
	{name: "AST_IN_PROG in an AST", pre: []string{"$DCLAST_S\tASTADR=ASTRTN"}, show: []show{mem("ASTVAL", 1)}},
	{name: "ASN_WTH_MBX NLA0", data: []string{ascid("%N", "NLA0:"), "%C:\t.LONG\t0", "%M:\t.LONG\t0", "%Z:\t.LONG\t0", "%Y:\t.LONG\t0"},
		call: "LIB$ASN_WTH_MBX", args: []string{"%N", "0", "0", "%C", "%M"},
		post: append(append(nonzero("%C"), "CLRL\t%Y", "TSTL\t%M", "BEQL\t%NY", "MOVL\t#1,%Y", "%NY:"),
			"$DASSGN_S\tCHAN=%C", "$DASSGN_S\tCHAN=%M"),
		show: []show{mem("%Z", 1), mem("%Y", 1)}},
	{name: "ASN_WTH_MBX size 100", data: []string{ascid("%N", "NLA0:"), "%S:\t.LONG\t100", "%Q:\t.LONG\t500", "%C:\t.LONG\t0", "%M:\t.LONG\t0",
		"%I:\t.LONG\tDVI$_DEVBUFSIZ", "%V:\t.LONG\t^X55555555", "%R:\t.LONG\t0"},
		call: "LIB$ASN_WTH_MBX", args: []string{"%N", "%S", "%Q", "%C", "%M"},
		post: []string{"PUSHAB\t%V", "PUSHL\t#0", "PUSHAB\t%M", "PUSHAB\t%I", "CALLS\t#4,G^LIB$GETDVI", "MOVL\tR0,%R",
			"$DASSGN_S\tCHAN=%C", "$DASSGN_S\tCHAN=%M"},
		show: []show{mem("%R", 1), mem("%V", 1)}},
	{name: "ASN_WTH_MBX no such device", data: []string{ascid("%N", "XYZ7:"), "%C:\t.LONG\t0", "%M:\t.LONG\t0",
		"%I:\t.LONG\tDVI$_DEVCLASS", "%V:\t.LONG\t^X55555555", "%R:\t.LONG\t0"},
		call: "LIB$ASN_WTH_MBX", args: []string{"%N", "0", "0", "%C", "%M"},
		post: []string{"PUSHAB\t%V", "PUSHL\t#0", "PUSHAB\t%M", "PUSHAB\t%I", "CALLS\t#4,G^LIB$GETDVI", "MOVL\tR0,%R"},
		show: []show{mem("%R", 1), mem("%V", 1)}},
	{name: "GET_USERS_LANGUAGE", data: []string{dyn("%D")},
		call: "LIB$GET_USERS_LANGUAGE", args: []string{"%D"}, show: []show{txt("%D")}},
	{name: "define SYS$LANGUAGE", data: []string{"%S:\t.LONG\t0"},
		pre:  []string{"$CRELNM_S\tTABNAM=PROCTAB,LOGNAM=LANGNAM,ITMLST=LANGITM", "MOVL\tR0,%S"},
		show: []show{mem("%S", 1)}},
	{name: "GET_USERS_LANGUAGE FRENCH", data: []string{dyn("%D")},
		call: "LIB$GET_USERS_LANGUAGE", args: []string{"%D"}, show: []show{txt("%D")}},
	{name: "GET_USERS_LANGUAGE into S 3", data: []string{fixed("%D", 3)},
		call: "LIB$GET_USERS_LANGUAGE", args: []string{"%D"}, post: []string{"$DELLNM_S\tTABNAM=PROCTAB,LOGNAM=LANGNAM"},
		show: []show{txt("%D")}},
	waitCase("WAIT 0", "%S:\t.F_FLOATING\t0.0", []string{"%S"}, 0),
	waitCase("WAIT 0.25", "%S:\t.F_FLOATING\t0.25", []string{"%S"}, 20),
	waitCase("WAIT 0.004", "%S:\t.F_FLOATING\t0.004", []string{"%S"}, 1),
	waitCase("WAIT -1", "%S:\t.F_FLOATING\t-1.0", []string{"%S"}, 0),
	waitCase("WAIT 100001", "%S:\t.F_FLOATING\t100001.0", []string{"%S"}, 0),
	waitCase("WAIT reserved operand", "%S:\t.LONG\t^X8000", []string{"%S"}, 0),
	waitCase("WAIT 4 arguments", "%S:\t.F_FLOATING\t0.0", []string{"%S", "0", "0", "0"}, 0),
	waitCase("WAIT D 0.5", "%S:\t.D_FLOATING\t0.5\n%T:\t.LONG\t1", []string{"%S", "0", "%T"}, 40),
	waitCase("WAIT G 0.5", "%S:\t.G_FLOATING\t0.5\n%T:\t.LONG\t2", []string{"%S", "0", "%T"}, 40),
	waitCase("WAIT IEEE S 0.25", "%S:\t.LONG\t^X3E800000\n%T:\t.LONG\t4", []string{"%S", "0", "%T"}, 20),
	waitCase("WAIT float type 6", "%S:\t.F_FLOATING\t0.0\n%T:\t.LONG\t6", []string{"%S", "0", "%T"}, 0),
	{name: "WAIT no arguments", call: "LIB$WAIT"},
	{name: "WAIT 2 after $WAKE", data: append([]string{"%S:\t.F_FLOATING\t2.0"}, clockData...),
		pre: append([]string{"$WAKE_S"}, startClock...), call: "LIB$WAIT", args: []string{"%S"},
		post: append(elapsed(100), "$CANWAK_S"), show: []show{mem("%Q", 1), vmem("%H", 1)}},
	{name: "WAIT NOWAKE after $WAKE", data: append([]string{"%S:\t.F_FLOATING\t0.5", "%F:\t.LONG\t1"}, clockData...),
		pre: append([]string{"$WAKE_S"}, startClock...), call: "LIB$WAIT", args: []string{"%S", "%F"},
		post: append(elapsed(40), "$CANWAK_S"), show: []show{mem("%Q", 1), vmem("%H", 1)}},

	// The protection text (subtask 7). LIB$GET_ACCNAM's FILE table first,
	// if VMS has the routine: ACCNAM stays 0 if it doesn't.
	{name: "GET_ACCNAM", call: "LIB$GET_ACCNAM", args: []string{"0", "0", "ACCNAM"}, post: nonzero("ACCNAM"),
		data: []string{"%Z:\t.LONG\t0"}, show: []show{mem("%Z", 1)}},
	accessName("ACCNAM 0", 0), accessName("ACCNAM 1", 1), accessName("ACCNAM 2", 2),
	accessName("ACCNAM 3", 3), accessName("ACCNAM 4", 4), accessName("ACCNAM 5", 5),
	formatCase("FORMAT EC00", "^XEC00", nil),
	formatCase("FORMAT 0000", "^X0000", nil),
	formatCase("FORMAT FF00", "^XFF00", nil),
	formatCase("FORMAT FFFF", "^XFFFF", nil),
	formatCase("FORMAT 0A5F", "^X0A5F", nil),
	formatCase("FORMAT custom names", "^XEC00", []string{"0", "OWNNAMS", "EQUALS", "COMMA"}),
	formatCase("FORMAT own table", "^X0000", []string{"MYTABLE"}),
	formatCase("FORMAT VMS table", "^X5A00", []string{"=ACCNAM"}),
	{name: "FORMAT into S 10", data: []string{"%M:\t.WORD\t^XEC00", fixed("%D", 10), "%L:\t.LONG\t^X5555"},
		call: "LIB$FORMAT_SOGW_PROT", args: []string{"%M", "0", "0", "0", "0", "%D", "%L"}, show: []show{txt("%D"), mem("%L", 1)}},
	{name: "FORMAT into VS 12", data: []string{"%M:\t.WORD\t^XEC00", varying("%D", 12), "%L:\t.LONG\t^X5555"},
		call: "LIB$FORMAT_SOGW_PROT", args: []string{"%M", "0", "0", "0", "0", "%D", "%L"}, show: []show{vs("%D"), mem("%L", 1)}},
	{name: "FORMAT 5 arguments", data: []string{"%M:\t.WORD\t^XEC00"},
		call: "LIB$FORMAT_SOGW_PROT", args: []string{"%M", "0", "0", "0", "0"}},
	{name: "FORMAT no mask", data: []string{dyn("%D")},
		call: "LIB$FORMAT_SOGW_PROT", args: []string{"0", "0", "0", "0", "0", "%D"}},
	parseCase("PARSE full", "SYSTEM=RWED,OWNER:RWED,GROUP,WORLD:R"),
	parseCase("PARSE spaced", "System: RWED, Owner: RWED, Group: RW, World: R"),
	parseCase("PARSE lowercase", "s:rwed,o:rwed,g:rw,w:r"),
	parseCase("PARSE blanks", " G r o u p = R E "),
	parseCase("PARSE W colon", "W:"),
	parseCase("PARSE WORLD", "WORLD"),
	parseCase("PARSE OWN:D", "OWN:D"),
	parseCase("PARSE repeated", "G:R,G:W"),
	parseCase("PARSE empty", ""),
	parseCase("PARSE SYX", "SYX:R"),
	parseCase("PARSE bad letter", "S:RX"),
	parseCase("PARSE semicolon", "S:R;W:R"),
	parseCase("PARSE no name", ":R"),
	parseCase("PARSE trailing comma", "S:R,"),
	parseCase("PARSE empty item", "S:R,,W"),
	parseCase("PARSE SYSTEMS", "SYSTEMS:R"),
	parseCase("PARSE second colon", "S:R:W"),
	parseCase("PARSE C", "S:C"),
	parseCase("PARSE name no separator", "SYSTEM RW"),
	{name: "PARSE own table", data: []string{ascid("%S", "S:AD,W:B"), "%P:\t.WORD\t^X5555", "%O:\t.WORD\t^X5555", "%E:\t.LONG\t^X5555"},
		call: "LIB$PARSE_SOGW_PROT", args: []string{"%S", "MYTABLE", "%P", "%O", "%E"}, show: []show{mem("%P", 2)}},
	{name: "PARSE 3 arguments", data: []string{ascid("%S", "S:R"), "%P:\t.LONG\t^X5555"},
		call: "LIB$PARSE_SOGW_PROT", args: []string{"%S", "0", "%P"}},
	accessCase("ACCESS RWED System", "RWED", "^X000F"),
	accessCase("ACCESS RWE Group", "RWE", "^X0F00"),
	accessCase("ACCESS d Owner", "d", "^X00F0"),
	accessCase("ACCESS R W World", "R W", "^XF000"),
	accessCase("ACCESS empty", "", "^X000F"),
	accessCase("ACCESS RWX", "RWX", "^X000F"),
	accessCase("ACCESS R,W", "R,W", "^X000F"),
	accessCase("ACCESS C", "C", "^X000F"),
	accessCase("ACCESS category 0", "R", "0"),
	accessCase("ACCESS category 00FF", "R", "^X00FF"),
}

// jpi is a LIB$GETJPI case of the caller's item, with every output: the
// value (two longwords, for the quadword items), the text, and its
// length. vary marks the value and text as changing from run to run.
func jpi(name, item string, vary bool) probeCase {
	c := probeCase{name: name,
		data: []string{"%I:\t.LONG\t" + item, "%V:\t.LONG\t^X55555555,^X55555555", dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$GETJPI", args: []string{"%I", "0", "0", "%V", "%D", "%L"}}

	c.show = values(vary)

	return c
}

// dvi is a LIB$GETDVI case of item for LIB55_DISK, every output shown.
func dvi(name, item string, vary bool) probeCase {
	c := probeCase{name: name,
		data: []string{"%I:\t.LONG\t" + item, "%V:\t.LONG\t^X55555555,^X55555555", dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$GETDVI", args: []string{"%I", "0", "DISKNAM", "%V", "%D", "%L"}}

	c.show = values(vary)

	return c
}

// syi is a LIB$GETSYI case of item for this node, every output shown.
func syi(name, item string, vary bool) probeCase {
	c := probeCase{name: name,
		data: []string{"%I:\t.LONG\t" + item, "%V:\t.LONG\t^X55555555,^X55555555", dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$GETSYI", args: []string{"%I", "%V", "%D", "%L"}}

	c.show = values(vary)

	return c
}

// values is an item case's shows: the value's two longwords, the text,
// and its length (shown even when the rest vary, unless vary).
func values(vary bool) []show {
	if vary {
		return []show{vmem("%V", 2), vtxt("%D"), vmem("%L", 1)}
	}

	return []show{mem("%V", 2), txt("%D"), mem("%L", 1)}
}

// ef is an event flag or LUN routine's case on the longword n (preset;
// the routine may write it).
func ef(name, routine, n string) probeCase {
	return probeCase{name: name, data: []string{"%N:\t.LONG\t" + n}, call: routine, args: []string{"%N"}, show: []show{mem("%N", 1)}}
}

// exhaust calls routine (GET_EF or GET_LUN) until it fails, showing how
// many it gave, the last number written, and the failing status (R0).
func exhaust(name, routine string) probeCase {
	return probeCase{name: name, data: []string{"%N:\t.LONG\t0", "%E:\t.LONG\t0"},
		pre:  []string{"CLRL\t%N", "%A:\tPUSHAB\t%E", "CALLS\t#1,G^" + routine, "BLBC\tR0,%B", "INCL\t%N", "BRB\t%A", "%B:"},
		show: []show{mem("%N", 1), mem("%E", 1)}}
}

// freeAll calls routine (FREE_EF or FREE_LUN) for first to last,
// showing how many it freed.
func freeAll(name, routine string, first, last int) probeCase {
	return probeCase{name: name, data: []string{"%N:\t.LONG\t0", "%E:\t.LONG\t0"},
		pre: []string{"CLRL\t%N", fmt.Sprintf("MOVL\t#%d,%%E", first), "%A:\tPUSHAB\t%E", "CALLS\t#1,G^" + routine,
			"BLBC\tR0,%B", "INCL\t%N", fmt.Sprintf("%%B:\tAOBLEQ\t#%d,%%E,%%A", last), "MOVL\t#1,R0"},
		show: []show{mem("%N", 1)}}
}

// statTimer is a LIB$STAT_TIMER case of code on the static block.
func statTimer(name string, code int) probeCase {
	return probeCase{name: name, data: []string{fmt.Sprintf("%%C:\t.LONG\t%d", code), "%V:\t.LONG\t^X55555555"},
		call: "LIB$STAT_TIMER", args: []string{"%C", "%V"}, show: []show{vmem("%V", 1)}}
}

// showTimer is a LIB$SHOW_TIMER case on the static block with code (an
// address, or "0") and the user argument, its line passed to TIMACT:
// shown with its digits masked, then as it was.
func showTimer(name, code, userArg string, data ...string) probeCase {
	return probeCase{name: name, data: data, pre: []string{"CLRW\tTIMDSC", "MOVL\t#^X55555555,TIMARG", "CLRL\tTIMCNT"},
		call: "LIB$SHOW_TIMER", args: []string{"0", code, "TIMACT", userArg},
		show: []show{masked("TIMDSC"), mem("TIMCNT", 1), mem("TIMARG", 1), vtxt("TIMDSC")}}
}

// waitCase is a LIB$WAIT case on the seconds (and type) in data, with
// the time it took: 1 if at least least hundredths of a second.
func waitCase(name, data string, args []string, least int) probeCase {
	return probeCase{name: name, data: append([]string{data}, clockData...), pre: startClock,
		call: "LIB$WAIT", args: args, post: elapsed(least), show: []show{mem("%Q", 1), vmem("%H", 1)}}
}

// accessName shows entry i of LIB$GET_ACCNAM's table, copied to a
// dynamic string (nothing when ACCNAM is 0).
func accessName(name string, i int) probeCase {
	return probeCase{name: name, data: []string{dyn("%D")},
		pre: []string{"MOVL\tACCNAM,R1", "BEQL\t%X", "PUSHAB\t%D", fmt.Sprintf("PUSHAB\t%d(R1)", 8*i),
			"CALLS\t#2,G^LIB$SCOPY_DXDX", "%X:\tCLRL\tR0"},
		show: []show{txt("%D")}}
}

// formatCase is a LIB$FORMAT_SOGW_PROT case of mask into a dynamic
// string, more giving the access-names, ownership-names, and separators.
func formatCase(name, mask string, more []string) probeCase {
	args := []string{"%M", "0", "0", "0", "0", "%D", "%L"}
	copy(args[1:], more)

	return probeCase{name: name, data: []string{"%M:\t.WORD\t" + mask, dyn("%D"), "%L:\t.LONG\t^X5555"},
		call: "LIB$FORMAT_SOGW_PROT", args: args, show: []show{txt("%D"), mem("%L", 1)}}
}

// parseCase is a LIB$PARSE_SOGW_PROT case of s, the masks and end
// position preset to ^X5555 and shown as two longwords: ownership mask
// and protection mask, then the end position.
func parseCase(name, s string) probeCase {
	src := ascid("%S", s)
	if s == "" {
		src = desc("%S", 0, 14, 1, "%S")
	}

	return probeCase{name: name, data: []string{src, "%P:\t.WORD\t^X5555", "%O:\t.WORD\t^X5555", "%E:\t.LONG\t^X5555"},
		call: "LIB$PARSE_SOGW_PROT", args: []string{"%S", "0", "%P", "%O", "%E"}, show: []show{mem("%P", 2)}}
}

// accessCase is a LIB$PARSE_ACCESS_CODE case of s for category, the
// mask and end position preset to ^X5555 and shown as two longwords.
func accessCase(name, s, category string) probeCase {
	src := ascid("%S", s)
	if s == "" {
		src = desc("%S", 0, 14, 1, "%S")
	}

	return probeCase{name: name, data: []string{src, "%C:\t.WORD\t" + category, "%M:\t.LONG\t^X5555", "%E:\t.LONG\t^X5555"},
		call: "LIB$PARSE_ACCESS_CODE", args: []string{"%S", "0", "%C", "%M", "%E"}, show: []show{mem("%M", 2)}}
}

// The program's fixed parts: its data (the output line, the FAO control
// strings), the handler, the action and AST routines, and the routines
// that print.
const programHead = `	.TITLE	LIB55	Phase 55 LIBRTL jackets and resources probe
	.IDENT	/V1.0/

; Written by testdata/lib55/gen.go: do not edit. Calls each of Phase
; 55's LIBRTL routines, case by case, each in a routine of its own under
; HANDLER, and prints with LIB$PUT_OUTPUT what it returned in R0, any
; condition it signaled, and what it stored. See gen.go.

	$SSDEF
	$JPIDEF
	$DVIDEF
	$SYIDEF
	$LNMDEF

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
MASKBUF:	.BLKB	200
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
VMEMCTL:	.ASCID	"  ~!AC:"
LWCTL:	.ASCID	" !XL"
TXTCTL:	.ASCID	/  !AC: !UL "!AF"/
VTXTCTL:	.ASCID	/  ~!AC: !UL "!AF"/
MASKCTL:	.ASCID	/  !AC masked: !UL "!AF"/
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

; TIMACT is LIB$SHOW_TIMER's action routine: it copies the line (its
; first argument, a descriptor) to TIMBUF, records its argument count
; and second argument, and returns TIMSTAT.
	.ENTRY	TIMACT,^M<R2,R3,R4,R5>
	MOVZBL	(AP),TIMCNT
	MOVL	4(AP),R0
	MOVW	(R0),TIMDSC
	MOVC3	(R0),@4(R0),TIMBUF
	CMPL	TIMCNT,#2
	BLSS	1$
	MOVL	8(AP),TIMARG
1$:	MOVL	TIMSTAT,R0
	RET

; ASTRTN is an AST routine: it records LIB$AST_IN_PROG's value.
	.ENTRY	ASTRTN,^M<>
	CALLS	#0,G^LIB$AST_IN_PROG
	MOVL	R0,ASTVAL
	RET

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

; SHOWMEM name, address, count, control: count longwords in hex, after
; the name as control (MEMCTL or VMEMCTL) shows it.
	.ENTRY	SHOWMEM,^M<R2,R3,R4>
	MOVL	8(AP),R2
	MOVL	12(AP),R3
	$FAO_S	CTRSTR=@16(AP),OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
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

; SHOWTXT name, descriptor, control: a class S or D string's length and
; text, as control (TXTCTL or VTXTCTL) shows them.
	.ENTRY	SHOWTXT,^M<R2>
	MOVL	8(AP),R2
	MOVZWL	(R2),R0
	$FAO_S	CTRSTR=@12(AP),OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=4(R2)
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

; SHOWMASK name, descriptor: a class S string's length and text, every
; digit shown as a 9.
	.ENTRY	SHOWMASK,^M<R2,R3,R4>
	MOVL	8(AP),R2
	MOVZWL	(R2),R3
	MOVL	4(R2),R4
	CLRL	R1
1$:	CMPL	R1,R3
	BGEQ	3$
	MOVB	(R4)[R1],R0
	CMPB	R0,#^A"0"
	BLSSU	2$
	CMPB	R0,#^A"9"
	BGTRU	2$
	MOVB	#^A"9",R0
2$:	MOVB	R0,MASKBUF[R1]
	INCL	R1
	BRB	1$
3$:	$FAO_S	CTRSTR=MASKCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,P3=R3,P4=#MASKBUF
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

		// The "pid 0 written" case asks $GETJPIW for the PID itself.
		if strings.Contains(strings.Join(c.post, "\n"), "%IL") {
			fmt.Fprintf(&b, "C%d_MY:\t.LONG\t0\nC%d_IL:\t.WORD\t4,JPI$_PID\n\t.ADDRESS\tC%d_MY\n\t.LONG\t0,0\n", n, n, n)
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

		if c.call != "" {
			for _, line := range push(c.args) {
				b.WriteString("\t" + prefix(line, n) + "\n")
			}

			fmt.Fprintf(&b, "\tCALLS\t#%d,G^%s\n", len(c.args), c.call)
		}

		// The post instructions keep R0: it's saved around them.
		b.WriteString("\tPUSHL\tR0\n")

		for _, line := range c.post {
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
			memCtl, txtCtl := "MEMCTL", "TXTCTL"
			if sh.vary {
				memCtl, txtCtl = "VMEMCTL", "VTXTCTL"
			}

			switch sh.kind {
			case "mem":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#4,SHOWMEM\n",
					memCtl, sh.count, prefix(sh.label, n), n, k+1)
			case "txt":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#3,SHOWTXT\n",
					txtCtl, prefix(sh.label, n), n, k+1)
			case "vs":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWVS\n", prefix(sh.label, n), n, k+1)
			case "mask":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWMASK\n", prefix(sh.label, n), n, k+1)
			}
		}
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// command is lib55.com: it builds and runs the probe, the program's
// output in its own file. It defines LIB55_DISK as the exchange
// volume's device, shows that the names the program defines aren't
// defined yet, and shows the DCL side of subtask 3 (F$GETDVI's VPROT
// and ACPTYPE, F$GETJPI's STATE and MODE).
const command = `$ ! LIB55.COM - the Phase 55 probe of LIBRTL's jackets and resources.
$ ! Written by testdata/lib55/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB55/OUTPUT=LIB55.LOG
$ !
$ ! LIB55.OUT gets the program's output; LIB55.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE LIB55_DISK 'DEV'
$ SHOW LOGICAL SYS$LANGUAGE
$ SHOW LOGICAL LIB55_NAME
$ SHOW LOGICAL LIB55_NONE
$ WRITE SYS$OUTPUT "VPROT=", F$GETDVI(DEV,"VPROT")
$ WRITE SYS$OUTPUT "ACPTYPE=", F$GETDVI(DEV,"ACPTYPE")
$ WRITE SYS$OUTPUT "NL ACPTYPE=", F$GETDVI("NLA0:","ACPTYPE")
$ WRITE SYS$OUTPUT "OWNUIC=", F$GETDVI(DEV,"OWNUIC")
$ WRITE SYS$OUTPUT "STATE=", F$GETJPI("","STATE")
$ WRITE SYS$OUTPUT "MODE=", F$GETJPI("","MODE")
$ WRITE SYS$OUTPUT "OWNER=", F$GETJPI("","OWNER")
$ WRITE SYS$OUTPUT "BOOTTIME=", F$GETSYI("BOOTTIME")
$ DEFINE/USER_MODE SYS$OUTPUT LIB55.MLG
$ MACRO/NOLIST LIB55
$ LINK/NOMAP LIB55
$ DEFINE/USER_MODE SYS$OUTPUT LIB55.OUT
$ RUN LIB55
$ SHOW SYMBOL $STATUS
$ DEASSIGN LIB55_DISK
$ DIRECTORY/SIZE/DATE
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 55 probe's exchange volume with
! govax. Written by testdata/lib55/gen.go. Run from the repository root:
!
!     govax console < testdata/lib55/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib55-exchange.dsk" /DEVICE=RD51 LIB55
MOUNT/WRITE DUA1 "testdata/disks/lib55-exchange.dsk"
COPY "testdata/lib55/lib55.mar"/HOST DUA1:[000000]LIB55.MAR
COPY "testdata/lib55/lib55.com"/HOST DUA1:[000000]LIB55.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib55")

	for name, text := range map[string]string{
		"lib55.mar":    program(),
		"lib55.com":    command,
		"exchange.cmd": exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
