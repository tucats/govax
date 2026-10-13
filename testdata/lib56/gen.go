//go:build ignore

// gen.go writes the Phase 56 probe of LIBRTL's date and time routines
// (docs/PHASE-56, subtask 8): lib56.mar, a MACRO program that calls every
// routine of the phase, case by case, and prints what each returned and
// stored (TestLib56Oracle compares govax's run with VMS's,
// vax/lib56-vax.txt); lib56.com, which builds and runs it on VMS; and
// exchange.cmd, the govax console script that makes the exchange volume.
// Run it from the repository root:
//
//	go run testdata/lib56/gen.go
//
// The program works as Phase 55's (testdata/lib55/gen.go) does: each
// case runs in a routine of its own, under a condition handler (HANDLER)
// that records the condition it's given and unwinds, so the output tells
// a status a routine returned from a condition it signaled. Its line is
// "name R0=xxxxxxxx", then a line per value it shows. A value that
// changes from run to run (the current time) is shown on a line that
// starts "  ~", and the oracle compares only its name.
//
// What's new here: a quadword time is shown as $ASCTIM writes it (show
// kind "time"), with its hex beside it where a tick matters. The program
// defines the logical names the formatting routines read (LIB$DT_FORMAT,
// LIB$DT_INPUT_FORMAT, SYS$LANGUAGE) with $CRELNM in the process table,
// a case at a time, and deletes them after; each such case uses a
// context cell of its own, so a context made earlier can't answer for it.
//
// The program expects LIB$DT_FORMAT, LIB$DT_INPUT_FORMAT, and
// LNM$LANGUAGE_FRENCH to be undefined (lib56.com shows them).
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
	kind  string // "mem" (longwords), "txt" (an S or D string), "vs" (a VS string), "time" (a quadword as $ASCTIM shows it)
	label string // the value's address (a label; "%" is the case's prefix)
	count int    // mem: how many longwords
	vary  bool   // the value changes from run to run: "~" before its name
}

func mem(label string, n int) show  { return show{"mem", label, n, false} }
func vmem(label string, n int) show { return show{"mem", label, n, true} }
func txt(label string) show         { return show{"txt", label, 0, false} }
func vtxt(label string) show        { return show{"txt", label, 0, true} }
func vs(label string) show          { return show{"vs", label, 0, false} }
func vvs(label string) show         { return show{"vs", label, 0, true} }
func tim(label string) show         { return show{"time", label, 0, false} }
func vtim(label string) show        { return show{"time", label, 0, true} }

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

// callLines are the instructions of one call of routine with args.
func callLines(routine string, args ...string) []string {
	return append(push(args), fmt.Sprintf("CALLS\t#%d,G^%s", len(args), routine))
}

// ascid is a class S text descriptor for s at label, its text after it.
// A tab in s is written as <9>, MACRO's way of putting a byte's value in
// a string.
func ascid(label, s string) string {
	if s == "" {
		return fmt.Sprintf("%s:\t.WORD\t0\n\t.BYTE\t14,1\n\t.ADDRESS\t%s", label, label)
	}

	return fmt.Sprintf("%s:\t.ASCID\t\"%s\"", label, strings.ReplaceAll(s, "\t", "\"<9>\""))
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

// vmsTime is the VMS absolute time of a local date and time: 100ns units
// since 17-NOV-1858.
func vmsTime(year int, month time.Month, day, hour, minute, second, ticks int) int64 {
	t := time.Date(year, month, day, hour, minute, second, 0, time.UTC)
	base := time.Date(1858, 11, 17, 0, 0, 0, 0, time.UTC)

	// In seconds, then ticks: a time.Duration can't span the years to 9999.
	return (t.Unix()-base.Unix())*10_000_000 + int64(ticks)
}

// ticks of 100 ns.
const (
	second = int64(10_000_000)
	minute = 60 * second
	hour   = 60 * minute
	day    = 24 * hour
)

// quad is a .LONG pair for the quadword v.
func quad(v int64) string {
	return fmt.Sprintf(".LONG\t^X%08X,^X%08X", uint32(v), uint32(uint64(v)>>32))
}

// unchanged is a quadword the routines' results are preset to, so a
// result left alone shows.
const unchanged = ".LONG\t^XEEEEEEEE,^XEEEEEEEE"

// The times the cases use, as labels in the shared data.
var times = []struct {
	label string
	v     int64
}{
	{"ZEROQ", 0},
	{"T1", vmsTime(1988, 1, 13, 9, 13, 25, 1_400_000)},       // the manual's examples: a Wednesday
	{"TMORN", vmsTime(1988, 1, 13, 0, 5, 0, 0)},              // just after midnight
	{"TNOON", vmsTime(1988, 1, 13, 12, 30, 0, 0)},            // just after noon
	{"TEVE", vmsTime(1988, 1, 13, 23, 45, 0, 0)},             // late evening
	{"TNEWYR", vmsTime(1988, 1, 1, 0, 0, 0, 0)},              // the first moment of a leap year
	{"TENDYR", vmsTime(1988, 12, 31, 23, 59, 59, 9_900_000)}, // its last hundredth
	{"TSEP", vmsTime(1988, 9, 21, 23, 24, 0, 0)},             // the manual's GET_MAXIMUM_DATE_LENGTH example
	{"TODD", vmsTime(2005, 2, 3, 5, 6, 7, 1_234_567)},        // a Thursday, one-digit day, a fraction to the tick
	{"TMAX", vmsTime(9999, 12, 31, 23, 0, 0, 0)},
	{"TBASE1", vmsTime(1858, 11, 18, 0, 0, 0, 0)}, // a day after the base date
	{"TOVF1", int64(1)<<31*1_000_000_000 - 1},     // LIB$DAY's last countable tick (govax)
	{"TOVF2", int64(1) << 31 * 1_000_000_000},     // and the next
	{"DLT1", -(1*day + 2*hour + 3*minute + 4*second + 500_000)},
	{"DLT2D1H", -(2*day + hour)},
	{"DLT3D", -3 * day},
	{"DLTHALF", -day / 2},
	{"DLT1H", -hour},
	{"DLT2H", -2 * hour},
	{"DLT1T", -1},
	{"DLT9999", -9999 * day},
	{"DLT10K", -10_000 * day},
	{"DLT1D", -day},
	{"DLTHUGE", -(int64(1) << 62)}, // far past any calendar
}

// The data every case may use.
var shared = func() []string {
	lines := []string{"\t.ALIGN\tQUAD"}

	for _, t := range times {
		lines = append(lines, fmt.Sprintf("%s:\t%s", t.label, quad(t.v)))
	}

	return append(lines,
		"TODAYT:\t.QUAD\t0\t\t; CONVERT_DATE_STRING's TODAY",
		"PROCTAB:\t.ASCID\t\"LNM$PROCESS_TABLE\"",
		"DTFNAM:\t.ASCID\t\"LIB$DT_FORMAT\"",
		"DTINAM:\t.ASCID\t\"LIB$DT_INPUT_FORMAT\"",
		"LANGNAM:\t.ASCID\t\"SYS$LANGUAGE\"",
		// Defaults vectors: Table 3-2's current date, and one with a time.
		"DEFV:\t.WORD\t1988,2,25,0,0,0,0",
		"DEFV2:\t.WORD\t2026,6,15,8,30,0,50",
	)
}()

// The LIB$K_ codes of the components.
const (
	kMonthName   = 0
	kOutput      = 1
	kInput       = 2
	kMonthAbb    = 3
	kLanguage    = 4
	kWeekday     = 6
	kWeekdayAbb  = 9
	kMeridiem    = 12
	kRelativeDay = 15
	kMnemonics   = 18
)

// --- Arithmetic (subtask 2) ---

// arith is an ADD_TIMES or SUB_TIMES case of two times.
func arith(name, routine, a, b string) probeCase {
	return probeCase{name: name, data: []string{"%R:\t" + unchanged},
		call: routine, args: []string{a, b, "%R"}, show: []show{tim("%R"), mem("%R", 2)}}
}

// mult is a MULT_DELTA_TIME or MULTF_DELTA_TIME case: the multiplier (a
// .LONG or .F_FLOATING line's operand) times a copy of the time.
func mult(name, routine, multiplier, timeLabel string) probeCase {
	directive := ".LONG"
	if routine == "LIB$MULTF_DELTA_TIME" && !strings.HasPrefix(multiplier, "^X") {
		directive = ".F_FLOATING"
	}

	return probeCase{name: name, data: []string{"%M:\t" + directive + "\t" + multiplier, "%R:\t.QUAD\t0"},
		pre:  []string{"MOVQ\t" + timeLabel + ",%R"},
		call: routine, args: []string{"%M", "%R"}, show: []show{tim("%R"), mem("%R", 2)}}
}

func arithmetic() []probeCase {
	return []probeCase{
		arith("ADD delta delta", "LIB$ADD_TIMES", "DLT1H", "DLT2H"),
		arith("ADD abs delta", "LIB$ADD_TIMES", "T1", "DLT1H"),
		arith("ADD delta abs", "LIB$ADD_TIMES", "DLT1H", "T1"),
		arith("ADD abs abs", "LIB$ADD_TIMES", "T1", "TNOON"),
		arith("ADD 9999 days 1 day", "LIB$ADD_TIMES", "DLT9999", "DLT1D"),
		arith("ADD 10000 days 1 tick", "LIB$ADD_TIMES", "DLT10K", "DLT1T"),
		arith("ADD year 9999 2 days", "LIB$ADD_TIMES", "TMAX", "DLT2D1H"),
		arith("ADD zero delta", "LIB$ADD_TIMES", "ZEROQ", "DLT1H"),
		arith("ADD overflow", "LIB$ADD_TIMES", "TMAX", "DLTHUGE"),
		arith("ADD huge deltas", "LIB$ADD_TIMES", "DLTHUGE", "DLTHUGE"),
		{name: "ADD 2 arguments", data: []string{"%R:\t" + unchanged},
			call: "LIB$ADD_TIMES", args: []string{"DLT1H", "DLT2H"}, show: []show{mem("%R", 2)}},
		arith("SUB abs later earlier", "LIB$SUB_TIMES", "TNOON", "T1"),
		arith("SUB abs earlier later", "LIB$SUB_TIMES", "T1", "TNOON"),
		arith("SUB abs equal", "LIB$SUB_TIMES", "T1", "T1"),
		arith("SUB abs delta", "LIB$SUB_TIMES", "T1", "DLT1H"),
		arith("SUB delta delta", "LIB$SUB_TIMES", "DLT2H", "DLT1H"),
		arith("SUB delta shorter longer", "LIB$SUB_TIMES", "DLT1H", "DLT2H"),
		arith("SUB delta equal", "LIB$SUB_TIMES", "DLT1H", "DLT1H"),
		arith("SUB delta abs", "LIB$SUB_TIMES", "DLT1H", "T1"),
		arith("SUB abs 30000 days apart", "LIB$SUB_TIMES", "T1", "TBASE1"),
		arith("SUB before base date", "LIB$SUB_TIMES", "TBASE1", "DLT2D1H"),
		mult("MULT by 3", "LIB$MULT_DELTA_TIME", "3", "DLT1H"),
		mult("MULT by 0", "LIB$MULT_DELTA_TIME", "0", "DLT1H"),
		mult("MULT by -2", "LIB$MULT_DELTA_TIME", "-2", "DLT1H"),
		mult("MULT abs", "LIB$MULT_DELTA_TIME", "2", "T1"),
		mult("MULT past 10000 days", "LIB$MULT_DELTA_TIME", "2", "DLT9999"),
		mult("MULT zero quad", "LIB$MULT_DELTA_TIME", "2", "ZEROQ"),
		mult("MULTF by 1.5", "LIB$MULTF_DELTA_TIME", "1.5", "DLT1H"),
		mult("MULTF by 0.0", "LIB$MULTF_DELTA_TIME", "0.0", "DLT1H"),
		mult("MULTF by -1.5", "LIB$MULTF_DELTA_TIME", "-1.5", "DLT1H"),
		mult("MULTF 1 tick by 0.5", "LIB$MULTF_DELTA_TIME", "0.5", "DLT1T"),
		mult("MULTF 1 tick by 1.5", "LIB$MULTF_DELTA_TIME", "1.5", "DLT1T"),
		mult("MULTF 1 tick by 2.5", "LIB$MULTF_DELTA_TIME", "2.5", "DLT1T"),
		mult("MULTF by 1/3", "LIB$MULTF_DELTA_TIME", "0.33333333", "DLT1H"),
		mult("MULTF reserved", "LIB$MULTF_DELTA_TIME", "^X8000", "DLT1H"),
		mult("MULT overflow", "LIB$MULT_DELTA_TIME", "2147483647", "DLT1D"),
		mult("MULTF overflow", "LIB$MULTF_DELTA_TIME", "1.0E30", "DLT1D"),
		mult("MULTF 1 tick by 1.4", "LIB$MULTF_DELTA_TIME", "1.4", "DLT1T"),
		mult("MULTF abs", "LIB$MULTF_DELTA_TIME", "2.0", "T1"),
		mult("MULTF past 10000 days", "LIB$MULTF_DELTA_TIME", "1.5", "DLT9999"),
	}
}

// --- Conversion (subtask 3) ---

// The operations' names, by LIB$K_ code.
var operations = []string{"", "MONTH_OF_YEAR", "DAY_OF_YEAR", "HOUR_OF_YEAR", "MINUTE_OF_YEAR", "SECOND_OF_YEAR",
	"DAY_OF_MONTH", "HOUR_OF_MONTH", "MINUTE_OF_MONTH", "SECOND_OF_MONTH", "DAY_OF_WEEK", "HOUR_OF_WEEK",
	"MINUTE_OF_WEEK", "SECOND_OF_WEEK", "HOUR_OF_DAY", "MINUTE_OF_DAY", "SECOND_OF_DAY", "MINUTE_OF_HOUR",
	"SECOND_OF_HOUR", "SECOND_OF_MINUTE", "JULIAN_DATE", "DELTA_WEEKS", "DELTA_DAYS", "DELTA_HOURS",
	"DELTA_MINUTES", "DELTA_SECONDS", "DELTA_WEEKS_F", "DELTA_DAYS_F", "DELTA_HOURS_F", "DELTA_MINUTES_F",
	"DELTA_SECONDS_F"}

// cvtFrom is a CVT_FROM_INTERNAL_TIME (or, floating, CVTF_FROM) case of
// operation op on the time at timeLabel ("" for none).
func cvtFrom(name string, op int, timeLabel string, floating bool) probeCase {
	routine := "LIB$CVT_FROM_INTERNAL_TIME"
	if floating {
		routine = "LIB$CVTF_FROM_INTERNAL_TIME"
	}

	args := []string{"%O", "%R"}
	if timeLabel != "" {
		args = append(args, timeLabel)
	}

	return probeCase{name: name, data: []string{fmt.Sprintf("%%O:\t.LONG\t%d", op), "%R:\t.LONG\t^X55555555"},
		call: routine, args: args, show: []show{mem("%R", 1)}}
}

// cvtTo is a CVT_TO_INTERNAL_TIME (or CVTF_TO) case of operation op on
// value (a .LONG or .F_FLOATING operand).
func cvtTo(name string, op int, value string, floating bool) probeCase {
	routine, directive := "LIB$CVT_TO_INTERNAL_TIME", ".LONG"
	if floating {
		routine = "LIB$CVTF_TO_INTERNAL_TIME"
		if !strings.HasPrefix(value, "^X") {
			directive = ".F_FLOATING"
		}
	}

	return probeCase{name: name, data: []string{fmt.Sprintf("%%O:\t.LONG\t%d", op), "%V:\t" + directive + "\t" + value, "%R:\t" + unchanged},
		call: routine, args: []string{"%O", "%V", "%R"}, show: []show{tim("%R"), mem("%R", 2)}}
}

// vectim is a CVT_VECTIM case of seven words.
func vectim(name string, words ...int) probeCase {
	var w []string
	for _, n := range words {
		w = append(w, fmt.Sprint(n))
	}

	return probeCase{name: name, data: []string{"%V:\t.WORD\t" + strings.Join(w, ","), "%R:\t" + unchanged},
		call: "LIB$CVT_VECTIM", args: []string{"%V", "%R"}, show: []show{tim("%R"), mem("%R", 2)}}
}

func conversion() []probeCase {
	var cs []probeCase

	// Every absolute operation on three times.
	for _, t := range []struct{ label, short string }{{"T1", "T1"}, {"TNEWYR", "NEWYR"}, {"TENDYR", "ENDYR"}} {
		for op := 1; op <= 20; op++ {
			cs = append(cs, cvtFrom(fmt.Sprintf("FROM %s %s", operations[op], t.short), op, t.label, false))
		}
	}

	for op := 21; op <= 25; op++ {
		cs = append(cs, cvtFrom("FROM "+operations[op]+" DLT1", op, "DLT1", false))
	}

	cs = append(cs,
		cvtFrom("FROM DELTA_DAYS 2D1H", 22, "DLT2D1H", false),
		cvtFrom("FROM DELTA_DAYS abs", 22, "T1", false),
		cvtFrom("FROM DAY_OF_YEAR delta", 2, "DLT1", false),
		cvtFrom("FROM DELTA_DAYS now", 22, "", false),
		cvtFrom("FROM op 0", 0, "T1", false),
		cvtFrom("FROM op 26", 26, "DLT1", false),
		cvtFrom("FROM op 31", 31, "T1", false),
		cvtFrom("FROM DAY_OF_YEAR zero quad", 2, "ZEROQ", false),
		cvtFrom("FROM JULIAN_DATE zero quad", 20, "ZEROQ", false),
		cvtFrom("FROM DAY_OF_WEEK zero quad", 10, "ZEROQ", false),
		cvtFrom("FROM DELTA_DAYS 10000 days", 22, "DLT10K", false),
	)

	for op := 26; op <= 30; op++ {
		cs = append(cs, cvtFrom("FROMF "+operations[op]+" DLT1", op, "DLT1", true))
	}

	cs = append(cs,
		cvtFrom("FROMF DELTA_DAYS_F 1 tick", 27, "DLT1T", true),
		cvtFrom("FROMF DELTA_DAYS_F abs", 27, "T1", true),
		cvtFrom("FROMF op 22", 22, "DLT1", true),
		cvtFrom("FROMF no time", 27, "", true),
		cvtTo("TO DELTA_DAYS 3", 22, "3", false),
		cvtTo("TO DELTA_DAYS 0", 22, "0", false),
		cvtTo("TO DELTA_DAYS -1", 22, "-1", false),
		cvtTo("TO DELTA_DAYS 9999", 22, "9999", false),
		cvtTo("TO DELTA_DAYS 10000", 22, "10000", false),
		cvtTo("TO DELTA_WEEKS 1428", 21, "1428", false),
		cvtTo("TO DELTA_WEEKS 1429", 21, "1429", false),
		cvtTo("TO DELTA_SECONDS 86400", 25, "86400", false),
		cvtTo("TO DELTA_HOURS 5", 23, "5", false),
		cvtTo("TO DELTA_MINUTES 90", 24, "90", false),
		cvtTo("TO op 1", 1, "3", false),
		cvtTo("TO op 26", 26, "3", false),
		cvtTo("TOF DELTA_HOURS_F 1.5", 28, "1.5", true),
		cvtTo("TOF DELTA_DAYS_F 0.0", 27, "0.0", true),
		cvtTo("TOF DELTA_DAYS_F -1.0", 27, "-1.0", true),
		cvtTo("TOF DELTA_DAYS_F 10000", 27, "10000.0", true),
		cvtTo("TOF DELTA_DAYS_F 9999.5", 27, "9999.5", true),
		cvtTo("TOF DELTA_SECONDS_F 1/3", 30, "0.33333333", true),
		cvtTo("TOF DELTA_SECONDS_F 1e-8", 30, "0.00000001", true),
		cvtTo("TOF DELTA_SECONDS_F 1.5e-7", 30, "0.00000015", true),
		cvtTo("TOF reserved", 27, "^X8000", true),
		cvtTo("TOF DELTA_SECONDS_F 2.7e-7", 30, "0.00000027", true),
		cvtTo("TOF DELTA_SECONDS_F 1.9", 30, "1.9999999", true),
		cvtTo("TOF DELTA_DAYS_F 1e30", 27, "1.0E30", true),
		cvtTo("TO DELTA_WEEKS max", 21, "2147483647", false),
		cvtTo("TO DELTA_SECONDS max", 25, "2147483647", false),
		cvtTo("TOF op 22", 22, "1.0", true),
		vectim("VECTIM abs", 1988, 1, 13, 9, 13, 25, 14),
		vectim("VECTIM delta", 0, 0, 3, 1, 2, 3, 4),
		vectim("VECTIM zeros", 0, 0, 0, 0, 0, 0, 0),
		vectim("VECTIM 29 Feb 1988", 1988, 2, 29, 0, 0, 0, 0),
		vectim("VECTIM 29 Feb 1987", 1987, 2, 29, 0, 0, 0, 0),
		vectim("VECTIM month 13", 1988, 13, 1, 0, 0, 0, 0),
		vectim("VECTIM month 0", 1988, 0, 1, 0, 0, 0, 0),
		vectim("VECTIM day 0", 1988, 1, 0, 0, 0, 0, 0),
		vectim("VECTIM 31 Apr", 1988, 4, 31, 0, 0, 0, 0),
		vectim("VECTIM hour 24", 1988, 1, 1, 24, 0, 0, 0),
		vectim("VECTIM minute 60", 1988, 1, 1, 0, 60, 0, 0),
		vectim("VECTIM second 60", 1988, 1, 1, 0, 0, 60, 0),
		vectim("VECTIM hundredths 100", 1988, 1, 1, 0, 0, 0, 100),
		vectim("VECTIM year 0 month 1", 0, 1, 1, 0, 0, 0, 0),
		vectim("VECTIM 16 Nov 1858", 1858, 11, 16, 0, 0, 0, 0),
		vectim("VECTIM 17 Nov 1858", 1858, 11, 17, 0, 0, 0, 0),
		vectim("VECTIM year 1700", 1700, 1, 1, 0, 0, 0, 0),
		vectim("VECTIM year 10000", 10000, 1, 1, 0, 0, 0, 0),
		vectim("VECTIM delta 9999 days", 0, 0, 9999, 23, 59, 59, 99),
		vectim("VECTIM delta 10000 days", 0, 0, 10000, 0, 0, 0, 0),
		vectim("VECTIM delta hour 24", 0, 0, 1, 24, 0, 0, 0),
		vectim("VECTIM year 30000", 30000, 1, 1, 0, 0, 0, 0),
		vectim("VECTIM year 65535", 65535, 12, 31, 0, 0, 0, 0),
		vectim("VECTIM delta 65535 days", 0, 0, 65535, 0, 0, 0, 0),
		probeCase{name: "VECTIM 1 argument", data: []string{"%V:\t.WORD\t1988,1,13,9,13,25,14"},
			call: "LIB$CVT_VECTIM", args: []string{"%V"}},
	)

	return cs
}

// --- The simple routines (subtask 4) ---

// dayCase is a LIB$DAY case of the time at timeLabel ("" omitted, "#0" 0
// by value), both results preset.
func dayCase(name, timeLabel string) probeCase {
	args := []string{"%N"}
	if timeLabel != "" {
		args = append(args, timeLabel, "%T")
	}

	vary := timeLabel == "" || timeLabel == "#0"
	sh := []show{mem("%N", 1), mem("%T", 1)}

	if vary {
		sh = []show{vmem("%N", 1), vmem("%T", 1)}
	}

	return probeCase{name: name, data: []string{"%N:\t.LONG\t^X55555555", "%T:\t.LONG\t^X55555555"},
		call: "LIB$DAY", args: args, show: sh}
}

// weekdayCase is a LIB$DAY_OF_WEEK case of the time at timeLabel ("0"
// for none).
func weekdayCase(name, timeLabel string) probeCase {
	sh := mem("%N", 1)
	if timeLabel == "0" {
		sh = vmem("%N", 1)
	}

	return probeCase{name: name, data: []string{"%N:\t.LONG\t^X55555555"},
		call: "LIB$DAY_OF_WEEK", args: []string{timeLabel, "%N"}, show: []show{sh}}
}

func simple() []probeCase {
	return []probeCase{
		{name: "DATE_TIME into D", data: []string{dyn("%D")},
			call: "LIB$DATE_TIME", args: []string{"%D"}, show: []show{mem("%D", 1), vtxt("%D")}},
		{name: "DATE_TIME into S 23", data: []string{fixed("%D", 23)},
			call: "LIB$DATE_TIME", args: []string{"%D"}, show: []show{vtxt("%D")}},
		{name: "DATE_TIME into S 11", data: []string{fixed("%D", 11)},
			call: "LIB$DATE_TIME", args: []string{"%D"}, show: []show{vtxt("%D")}},
		{name: "DATE_TIME into S 30", data: []string{fixed("%D", 30)},
			call: "LIB$DATE_TIME", args: []string{"%D"}, show: []show{vtxt("%D")}},
		{name: "DATE_TIME into VS 30", data: []string{varying("%D", 30), "%N:\t.LONG\t0"},
			call: "LIB$DATE_TIME", args: []string{"%D"}, post: []string{"MOVZWL\t%D_V,%N"}, show: []show{mem("%N", 1), vvs("%D")}},
		{name: "DATE_TIME into VS 10", data: []string{varying("%D", 10)},
			call: "LIB$DATE_TIME", args: []string{"%D"}, show: []show{vvs("%D")}},
		dayCase("DAY T1", "T1"),
		dayCase("DAY zero quad", "ZEROQ"),
		dayCase("DAY base plus 1", "TBASE1"),
		dayCase("DAY delta 2D1H", "DLT2D1H"),
		dayCase("DAY delta half day", "DLTHALF"),
		dayCase("DAY delta 1 tick", "DLT1T"),
		dayCase("DAY last before overflow", "TOVF1"),
		dayCase("DAY overflow", "TOVF2"),
		dayCase("DAY year 9999", "TMAX"),
		dayCase("DAY huge delta", "DLTHUGE"),
		dayCase("DAY now", ""),
		dayCase("DAY 0 by value", "#0"),
		weekdayCase("DAY_OF_WEEK T1", "T1"),
		weekdayCase("DAY_OF_WEEK zero quad", "ZEROQ"),
		weekdayCase("DAY_OF_WEEK TODD", "TODD"),
		weekdayCase("DAY_OF_WEEK delta 3 days", "DLT3D"),
		weekdayCase("DAY_OF_WEEK delta 1 hour", "DLT1H"),
		weekdayCase("DAY_OF_WEEK delta 2D1H", "DLT2D1H"),
		weekdayCase("DAY_OF_WEEK now", "0"),
	}
}

// --- Formatting: names, contexts, output, and input (subtasks 5 to 7) ---

// defineName is the data and instructions that define the process
// logical name at nameLabel (DTFNAM, DTINAM, LANGNAM) with values, its
// item list at %<tag>.
func defineName(tag, nameLabel string, values ...string) (data, pre []string) {
	list := "%" + tag

	var items strings.Builder

	fmt.Fprintf(&items, "%s:", list)

	for i, v := range values {
		fmt.Fprintf(&items, "\t.WORD\t%d,LNM$_STRING\n\t.ADDRESS\t%s_%d\n\t.LONG\t0\n", len(v), list, i)
	}

	items.WriteString("\t.LONG\t0")

	data = []string{items.String()}

	for i, v := range values {
		data = append(data, fmt.Sprintf("%s_%d:\t.ASCII\t\"%s\"", list, i, v))
	}

	return data, []string{fmt.Sprintf("$CRELNM_S\tTABNAM=PROCTAB,LOGNAM=%s,ITMLST=%s", nameLabel, list)}
}

// deleteName deletes the process logical name at nameLabel, whatever
// $DELLNM says.
func deleteName(nameLabel string) []string {
	return []string{"PUSHL\t#0", "PUSHAB\t" + nameLabel, "PUSHAB\tPROCTAB", "CALLS\t#3,G^SYS$DELLNM"}
}

// names is what a case defines and deletes around its calls: the
// logical names' values (nil to leave the name alone, an empty list to
// delete it first).
type names struct {
	format, input, language []string
}

// apply is the data, the instructions before, and the instructions
// after a case that sets n.
func (n names) apply() (data, pre, post []string) {
	for _, nm := range []struct {
		tag, label string
		values     []string
	}{{"NF", "DTFNAM", n.format}, {"NI", "DTINAM", n.input}, {"NL", "LANGNAM", n.language}} {
		switch {
		case nm.values == nil:
		case len(nm.values) == 0:
			pre = append(pre, deleteName(nm.label)...)
		default:
			d, p := defineName(nm.tag, nm.label, nm.values...)
			data = append(data, d...)
			pre = append(pre, p...)
			post = append(post, deleteName(nm.label)...)
		}
	}

	return data, pre, post
}

// initStep is one LIB$INIT_DATE_TIME_CONTEXT call on the case's cell
// (%X): a component and its init-string.
type initStep struct {
	component int
	text      string
}

// initLines is the data and instructions of a case's INIT calls: on a
// failure the case ends, its R0 INIT's status.
func initLines(steps []initStep) (data, pre []string) {
	for i, s := range steps {
		data = append(data, fmt.Sprintf("%%K%d:\t.LONG\t%d", i, s.component), ascid(fmt.Sprintf("%%J%d", i), s.text))
		pre = append(pre, callLines("LIB$INIT_DATE_TIME_CONTEXT", "%X", fmt.Sprintf("%%K%d", i), fmt.Sprintf("%%J%d", i))...)
		pre = append(pre, fmt.Sprintf("BLBS\tR0,%%I%d", i), "RET", fmt.Sprintf("%%I%d:", i))
	}

	return data, pre
}

// setup is a formatting case's context: the names it defines, the INIT
// steps on its cell, and whether it uses the static context (no cell).
type setup struct {
	names  names
	inits  []initStep
	static bool
}

// build is the data and instructions a case's setup needs, and the
// context argument its calls pass.
func (s setup) build() (data, pre, post []string, cell string) {
	data, pre, post = s.names.apply()
	data = append(data, "%X:\t.LONG\t0")

	d, p := initLines(s.inits)
	data = append(data, d...)
	pre = append(pre, p...)

	cell = "%X"
	if s.static {
		cell = "0"
	}

	return data, pre, post, cell
}

// formatCall is one LIB$FORMAT_DATE_TIME call of a format case: the time
// (a label, "#0" by value, or "" omitted), the output string ("D" for a
// dynamic one, "Sn" fixed of n bytes), and the flags ("" omitted).
type formatCall struct {
	time, output, flags string
}

// formatCase is a case of one or more FORMAT_DATE_TIME calls with one
// setup. Each call's text, length, and status are shown; R0 is the first
// call's.
func formatCase(name string, s setup, calls ...formatCall) probeCase {
	data, pre, post, cell := s.build()

	var sh []show

	for i, fc := range calls {
		d, l, st, f := fmt.Sprintf("%%D%d", i), fmt.Sprintf("%%L%d", i), fmt.Sprintf("%%S%d", i), fmt.Sprintf("%%F%d", i)

		switch {
		case fc.output == "D":
			data = append(data, dyn(d))
		default:
			var n int
			fmt.Sscanf(fc.output, "S%d", &n)
			data = append(data, fixed(d, n))
		}

		data = append(data, l+":\t.LONG\t^X55555555", st+":\t.LONG\t0")

		flags := "0"
		if fc.flags != "" {
			data = append(data, f+":\t.LONG\t"+fc.flags)
			flags = f
		}

		t := fc.time
		if t == "" {
			t = "0"
		}

		pre = append(pre, callLines("LIB$FORMAT_DATE_TIME", d, t, cell, l, flags)...)
		pre = append(pre, "MOVL\tR0,"+st)

		// The current time's text, and so its length, vary.
		if fc.time == "" || fc.time == "#0" {
			sh = append(sh, vtxt(d), vmem(l, 1))
		} else {
			sh = append(sh, txt(d), mem(l, 1))
		}

		sh = append(sh, mem(st, 1))
	}

	pre = append(pre, post...)
	pre = append(pre, "MOVL\t%S0,R0")

	return probeCase{name: name, data: data, pre: pre, show: sh}
}

// maxCase is a GET_MAXIMUM_DATE_LENGTH case with flags ("" omitted).
func maxCase(name string, s setup, flags string) probeCase {
	data, pre, post, cell := s.build()
	data = append(data, "%L:\t.LONG\t^X55555555")

	f := "0"
	if flags != "" {
		data = append(data, "%F:\t.LONG\t"+flags)
		f = "%F"
	}

	return probeCase{name: name, data: data, pre: pre, call: "LIB$GET_MAXIMUM_DATE_LENGTH",
		args: []string{"%L", cell, f}, post: post, show: []show{mem("%L", 1)}}
}

// getFormatCase is a GET_DATE_FORMAT case into a dynamic string, or a
// fixed one of size bytes.
func getFormatCase(name string, s setup, size int) probeCase {
	data, pre, post, cell := s.build()

	if size == 0 {
		data = append(data, dyn("%D"))
	} else {
		data = append(data, fixed("%D", size))
	}

	return probeCase{name: name, data: data, pre: pre, call: "LIB$GET_DATE_FORMAT",
		args: []string{"%D", cell}, post: post, show: []show{txt("%D")}}
}

// initCase is a case of INIT_DATE_TIME_CONTEXT calls on a fresh cell,
// each one's status shown, then the cell's being nonzero; R0 is the last
// call's.
func initCase(name string, steps ...initStep) probeCase {
	data := []string{"%X:\t.LONG\t0", "%Z:\t.LONG\t0"}

	var (
		pre []string
		sh  []show
	)

	for i, s := range steps {
		st := fmt.Sprintf("%%S%d", i)
		data = append(data, fmt.Sprintf("%%K%d:\t.LONG\t%d", i, s.component), ascid(fmt.Sprintf("%%J%d", i), s.text), st+":\t.LONG\t0")
		pre = append(pre, callLines("LIB$INIT_DATE_TIME_CONTEXT", "%X", fmt.Sprintf("%%K%d", i), fmt.Sprintf("%%J%d", i))...)
		pre = append(pre, "MOVL\tR0,"+st)
		sh = append(sh, mem(st, 1))
	}

	pre = append(pre, "CLRL\t%Z", "TSTL\t%X", "BEQL\t%NZ", "MOVL\t#1,%Z", "%NZ:",
		"PUSHAB\t%X", "CALLS\t#1,G^LIB$FREE_DATE_TIME_CONTEXT",
		fmt.Sprintf("MOVL\t%%S%d,R0", len(steps)-1))

	return probeCase{name: name, data: data, pre: pre, show: append(sh, mem("%Z", 1))}
}

// convertCase is a CONVERT_DATE_STRING case of input with setup s, the
// flags ("" omitted), and the defaults vector ("" omitted): the result
// as a time and in hex, and the defaulted fields.
func convertCase(name string, s setup, input, flags, defaults string) probeCase {
	data, pre, post, cell := s.build()
	data = append(data, ascid("%I", input), "%R:\t"+unchanged, "%M:\t.LONG\t^X55555555")

	args := []string{"%I", "%R", cell, "0", "0", "%M"}

	if flags != "" {
		data = append(data, "%F:\t.LONG\t"+flags)
		args[3] = "%F"
	}

	if defaults != "" {
		args[4] = defaults
	}

	return probeCase{name: name, data: data, pre: pre, call: "LIB$CONVERT_DATE_STRING", args: args, post: post,
		show: []show{tim("%R"), mem("%R", 2), mem("%M", 1)}}
}

// relativeCase is a CONVERT_DATE_STRING case whose result depends on
// today: shown as its difference from TODAY's (TODAYT), a delta, or 0
// when it fails (the status the post instructions keep at (SP)).
func relativeCase(name string, s setup, input, flags, defaults string) probeCase {
	c := convertCase(name, s, input, flags, defaults)
	c.data = append(c.data, "%Q:\t.QUAD\t0")
	c.post = append(c.post, "BLBC\t(SP),%QX", "MOVQ\t%R,%Q", "SUBL2\tTODAYT,%Q", "SBWC\tTODAYT+4,%Q+4", "%QX:")
	c.show = []show{vtim("%R"), mem("%Q", 2), mem("%M", 1)}

	return c
}

// The predefined formats' names, by number.
func dateName(n int) string { return fmt.Sprintf("LIB$DATE_FORMAT_%03d", n) }
func timeName(n int) string { return fmt.Sprintf("LIB$TIME_FORMAT_%03d", n) }

// english is a setup with SYS$LANGUAGE ENGLISH in the process table, and
// LIB$DT_FORMAT and LIB$DT_INPUT_FORMAT as given (nil: deleted).
func english(format, input []string) setup {
	if format == nil {
		format = []string{}
	}

	if input == nil {
		input = []string{}
	}

	return setup{names: names{format: format, input: input, language: []string{"ENGLISH"}}}
}

// The output formats of the INIT cases.
const (
	manualOutput = "|!MAU !DD, !Y4|!HH2:!M0 !MIU|"
	manualInput  = "!MAU !DD, !Y4 !H02:!M0:!S0:!C2 !MIU"
)

func formatting() []probeCase {
	var cs []probeCase

	// Every predefined format, a date and a time in each case, on four
	// times of day.
	for n := 1; n <= 40; n++ {
		tn := (n-1)%20 + 1
		cs = append(cs, formatCase(fmt.Sprintf("FORMAT date %03d time %03d", n, tn),
			english([]string{dateName(n), timeName(tn)}, nil),
			formatCall{"TMORN", "D", ""}, formatCall{"T1", "D", ""}, formatCall{"TNOON", "D", ""}, formatCall{"TEVE", "D", ""}))
	}

	timeFirst := english([]string{timeName(12), dateName(12)}, nil)

	cs = append(cs,
		// The names: nothing defined, the static context; a fresh one.
		formatCase("FORMAT static nothing defined", setup{static: true, names: names{format: []string{}, input: []string{}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT nothing defined", setup{names: names{format: []string{}, input: []string{}}},
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "3"}),
		formatCase("FORMAT ENGLISH nothing defined", english(nil, nil), formatCall{"T1", "D", ""}),
		formatCase("FORMAT FRENCH no table", setup{names: names{format: []string{dateName(2), timeName(2)}, language: []string{"FRENCH"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT FRENCH nothing defined", setup{names: names{format: []string{}, language: []string{"FRENCH"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT time first", timeFirst,
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "0"}, formatCall{"T1", "D", "1"}, formatCall{"T1", "D", "2"},
			formatCall{"T1", "D", "3"}, formatCall{"T1", "D", "4"}, formatCall{"T1", "D", "5"}),
		formatCase("FORMAT time only", english([]string{timeName(5)}, nil),
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "3"}, formatCall{"T1", "D", "2"}),
		formatCase("FORMAT date only", english([]string{dateName(13)}, nil),
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "3"}, formatCall{"T1", "D", "1"}),
		formatCase("FORMAT undefined format", english([]string{dateName(999), timeName(2)}, nil),
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT two dates", english([]string{dateName(1), dateName(13)}, nil),
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT three names", english([]string{dateName(1), timeName(2), timeName(5)}, nil),
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT a delta", timeFirst, formatCall{"DLT1H", "D", ""}),
		formatCase("FORMAT zero quad", timeFirst, formatCall{"ZEROQ", "D", ""}),
		formatCase("FORMAT now", timeFirst, formatCall{"", "D", ""}, formatCall{"#0", "D", ""}),
		formatCase("FORMAT into S 11", english([]string{dateName(1), timeName(1)}, nil),
			formatCall{"T1", "S11", ""}, formatCall{"T1", "S30", ""}, formatCall{"T1", "S23", ""}),
		formatCase("FORMAT TODD", english([]string{dateName(2), timeName(1)}, nil), formatCall{"TODD", "D", ""}),
		formatCase("FORMAT year 9999", english([]string{dateName(40), timeName(2)}, nil), formatCall{"TMAX", "D", ""}),
	)

	// LIB$DT_FORMAT changed between two calls on one context: read once,
	// or every call.
	{
		data, pre, post := english([]string{dateName(1), timeName(2)}, nil).names.apply()
		d2, p2 := defineName("NF2", "DTFNAM", dateName(13), timeName(5))
		data = append(append(data, d2...), "%X:\t.LONG\t0", dyn("%D0"), dyn("%D1"), "%S0:\t.LONG\t0", "%S1:\t.LONG\t0")
		pre = append(pre, callLines("LIB$FORMAT_DATE_TIME", "%D0", "T1", "%X")...)
		pre = append(pre, "MOVL\tR0,%S0")
		pre = append(pre, p2...)
		pre = append(pre, callLines("LIB$FORMAT_DATE_TIME", "%D1", "T1", "%X")...)
		pre = append(pre, "MOVL\tR0,%S1")
		pre = append(pre, post...)
		pre = append(pre, "MOVL\t%S0,R0")
		cs = append(cs, probeCase{name: "FORMAT names changed", data: data, pre: pre,
			show: []show{txt("%D0"), mem("%S0", 1), txt("%D1"), mem("%S1", 1)}})
	}

	// INIT's output formats and spellings: every mnemonic, a few at a
	// time (a line of output holds 132 bytes).
	for i, f := range []string{
		"|!WU !WAU !WC !WAC !WL !WAL|!H04|",
		"|!DD !DB !D0 !MAU !MAAU !MAC !MAAC !MAL !MAAL|!H04|",
		"|!MN0 !MNM !MNB !Y4 !Y3 !Y2 !Y1|!H04 !HH4 !HB4 !H02 !HH2 !HB2 !MIU !MIC !MIL|",
		"|!D0|!M0 !MM !MB !S0 !SS !SB !C7 !C6 !C5 !C4 !C3 !C2 !C1|",
	} {
		cs = append(cs, formatCase(fmt.Sprintf("FORMAT mnemonics %d", i+1), setup{inits: []initStep{{kOutput, f}}},
			formatCall{"TODD", "D", ""}, formatCall{"TNOON", "D", ""}, formatCall{"TMORN", "D", ""}))
	}

	cs = append(cs,
		formatCase("FORMAT manual output", setup{inits: []initStep{{kOutput, manualOutput}}},
			formatCall{"TSEP", "D", ""}, formatCall{"TMORN", "D", ""}, formatCall{"TNOON", "D", ""}),
		formatCase("FORMAT output time first", setup{inits: []initStep{{kOutput, "|!H04:!M0|!DB-!MAAU-!Y4|"}}},
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "2"}),
		formatCase("FORMAT output empty date", setup{inits: []initStep{{kOutput, "||!H04:!M0|"}}},
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "3"}),
		formatCase("FORMAT output punctuation", setup{inits: []initStep{{kOutput, "|<!D0>|[!H04]|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT custom months", setup{inits: []initStep{
			{kOutput, "|!MAU !MAL !MAC !MAAU|!H04|"},
			{kMonthName, "|Janvier|Fevrier|Mars|Avril|Mai|Juin|Juillet|Aout|Septembre|Octobre|Novembre|Decembre|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT custom lower months", setup{inits: []initStep{
			{kOutput, "|!MAU !MAL !MAC|!H04|"},
			{kMonthName, "|janvier|fevrier|mars|avril|mai|juin|juillet|aout|septembre|octobre|novembre|decembre|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT custom meridiem", setup{inits: []initStep{
			{kOutput, "|!DD|!HH2 !MIU !MIL !MIC|"}, {kMeridiem, "|am|pm|"}}},
			formatCall{"T1", "D", ""}, formatCall{"TEVE", "D", ""}),
		formatCase("FORMAT custom weekdays", setup{inits: []initStep{
			{kOutput, "|!WU !WAC|!H04|"},
			{kWeekday, "|Lundi|Mardi|Mercredi|Jeudi|Vendredi|Samedi|Dimanche|"},
			{kWeekdayAbb, "|lu|ma|me|je|ve|sa|di|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT INIT no output format", setup{inits: []initStep{
			{kMonthName, "|A|B|C|D|E|F|G|H|I|J|K|L|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT INIT language", setup{inits: []initStep{{kLanguage, "|ENGLISH|"}, {kOutput, "|!MAC !DD|!H04|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT INIT language FRENCH", setup{inits: []initStep{{kLanguage, "|FRENCH|"}, {kOutput, "|!MAC !DD|!H04|"}}},
			formatCall{"T1", "D", ""}),
		formatCase("FORMAT INIT then names", setup{
			names: names{format: []string{dateName(13), timeName(5)}, language: []string{"ENGLISH"}},
			inits: []initStep{{kMonthName, "|A|B|C|D|E|F|G|H|I|J|K|L|"}}},
			formatCall{"T1", "D", ""}),

		maxCase("MAX manual", setup{inits: []initStep{{kOutput, manualOutput}}}, ""),
		maxCase("MAX manual flags 1", setup{inits: []initStep{{kOutput, manualOutput}}}, "1"),
		maxCase("MAX manual flags 2", setup{inits: []initStep{{kOutput, manualOutput}}}, "2"),
		maxCase("MAX manual flags 0", setup{inits: []initStep{{kOutput, manualOutput}}}, "0"),
		maxCase("MAX standard", english(nil, nil), ""),
		maxCase("MAX weekday", setup{inits: []initStep{{kOutput, "|!WU !DD|!H04|"}}}, ""),
		maxCase("MAX abbreviations", setup{inits: []initStep{{kOutput, "|!WAC !MAAU !DB|!C7 !MIL|"}}}, ""),
		maxCase("MAX blank day", setup{inits: []initStep{{kOutput, "|!DD|!HH4|"}}}, ""),
		formatCase("FORMAT 13 months", setup{inits: []initStep{
			{kOutput, "|!MAU|!H04|"}, {kMonthName, "|A|B|C|D|E|F|G|H|I|J|K|L|M|"}}},
			formatCall{"TENDYR", "D", ""}),
		formatCase("FORMAT unknown mnemonic", setup{inits: []initStep{{kOutput, "|!DB-!XX|!H04|"}}}, formatCall{"T1", "D", ""}),
		formatCase("FORMAT lower case mnemonic", setup{inits: []initStep{{kOutput, "|!d0|!h04|"}}}, formatCall{"T1", "D", ""}),
		formatCase("FORMAT mixed element", setup{inits: []initStep{{kOutput, "|!D0 !H04|!M0|"}}},
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "2"}, formatCall{"T1", "D", "1"}),
		formatCase("FORMAT two dates", setup{inits: []initStep{{kOutput, "|!D0|!MN0|"}}},
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "2"}, formatCall{"T1", "D", "1"}),
		formatCase("FORMAT no fields element", setup{inits: []initStep{{kOutput, "|abc|!H04|"}}},
			formatCall{"T1", "D", ""}, formatCall{"T1", "D", "2"}),
		formatCase("FORMAT both empty", setup{inits: []initStep{{kOutput, "|||"}}}, formatCall{"T1", "D", ""}),
		formatCase("FORMAT one character", setup{inits: []initStep{{kOutput, "|"}}}, formatCall{"T1", "D", ""}),
		formatCase("FORMAT language blank", setup{inits: []initStep{{kLanguage, "| |"}, {kOutput, "|!MAC|!H04|"}}},
			formatCall{"T1", "D", ""}),
		maxCase("MAX unknown mnemonic", setup{inits: []initStep{{kOutput, "|!DB-!XX|!H04|"}}}, ""),
		maxCase("MAX date 012 time 020", english([]string{dateName(12), timeName(20)}, nil), ""),

		getFormatCase("GET_FORMAT manual 1", setup{inits: []initStep{{kInput, "|!MAU !DD, !Y4 !H04:!M0:!S0:!C2|"}}}, 0),
		getFormatCase("GET_FORMAT manual 2", setup{inits: []initStep{{kInput, "|!MN0-!D0-!Y2 !H04:!M0:!S0.!C2|"}}}, 0),
		getFormatCase("GET_FORMAT manual 3", setup{inits: []initStep{{kInput, "|!MN0/!D0/!Y2 !H02:!M0:!S0.!C2 !MIU|"}}}, 0),
		getFormatCase("GET_FORMAT blanks", setup{inits: []initStep{{kInput, "|  !MN0/!D0/!Y2   !H04:!M0:!S0.!C2  |"}}}, 0),
		getFormatCase("GET_FORMAT no fraction", setup{inits: []initStep{{kInput, "|!D0-!MN0-!Y4 !H04:!M0:!S0|"}}}, 0),
		getFormatCase("GET_FORMAT weekday", setup{inits: []initStep{{kInput, "|!WU !D0-!MN0-!Y4 !H04:!M0:!S0.!C2|"}}}, 0),
		getFormatCase("GET_FORMAT mnemonics", setup{inits: []initStep{
			{kInput, "|!MAU !DD, !Y4 !H02:!M0:!S0.!C2 !MIU|"}, {kMnemonics, "|JJJJ|MM|TT|SS|MM|SS|HH|AM/PM|MONAT|"}}}, 0),
		getFormatCase("GET_FORMAT names", english(nil, []string{"!D0.!MN0.!Y4 !H04:!M0:!S0.!C2"}), 0),
		getFormatCase("GET_FORMAT nothing defined", english(nil, nil), 0),
		getFormatCase("GET_FORMAT into S 5", english(nil, nil), 5),

		initCase("INIT each component",
			initStep{kMonthName, "|JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|"},
			initStep{kOutput, "|!DB-!MAAU-!Y4|!H04:!M0:!S0.!C2|"},
			initStep{kInput, "|!DB-!MAAU-!Y4 !H04:!M0:!S0.!C2|"},
			initStep{kMonthAbb, "|JA|FE|MR|AP|MY|JN|JL|AU|SE|OC|NO|DE|"},
			initStep{kLanguage, "|ENGLISH|"},
			initStep{kWeekday, "|MO|TU|WE|TH|FR|SA|SU|"},
			initStep{kWeekdayAbb, "|M|T|W|T|F|S|S|"},
			initStep{kMeridiem, "|A|P|"},
			initStep{kRelativeDay, "|Y|T|M|"},
			initStep{kMnemonics, "|YYYY|MM|DD|HH|MM|SS|CC|AM/PM|MONTH|"}),
		initCase("INIT unnamed components",
			initStep{5, "|X|"}, initStep{7, "|X|"}, initStep{19, "|X|"}, initStep{-1, "|X|"}, initStep{100, "|X|"}),
		initCase("INIT 11 months", initStep{kMonthName, "|A|B|C|D|E|F|G|H|I|J|K|"}),
		initCase("INIT 13 months", initStep{kMonthName, "|A|B|C|D|E|F|G|H|I|J|K|L|M|"}),
		initCase("INIT no closing delimiter", initStep{kInput, "|!DB-!MAAU-!Y4 !H04:!M0:!S0.!C2"}),
		initCase("INIT empty string", initStep{kInput, ""}),
		initCase("INIT one character", initStep{kInput, "|"}),
		initCase("INIT empty input", initStep{kInput, "||"}),
		initCase("INIT other delimiter", initStep{kMeridiem, "/AM/PM/"}),
		initCase("INIT unknown mnemonic", initStep{kOutput, "|!DB-!XX|!H04|"}),
		initCase("INIT lower case mnemonic", initStep{kOutput, "|!d0|!h04|"}),
		initCase("INIT mixed element", initStep{kOutput, "|!D0 !H04|!M0|"}),
		initCase("INIT two dates", initStep{kOutput, "|!D0|!MN0|"}),
		initCase("INIT no fields element", initStep{kOutput, "|abc|!H04|"}),
		initCase("INIT both empty", initStep{kOutput, "|||"}),
		initCase("INIT input bad mnemonic", initStep{kInput, "|!DB-!XX|"}),
		initCase("INIT input few fields", initStep{kInput, "|!D0|"}),
		initCase("INIT language blank", initStep{kLanguage, "| |"}),
		initCase("INIT empty months", initStep{kMonthName, "|||||||||||||"}),
		initCase("INIT element 255", initStep{kLanguage, "|" + strings.Repeat("E", 255) + "|"}),
		initCase("INIT element 256", initStep{kLanguage, "|" + strings.Repeat("E", 256) + "|"}),
		probeCase{name: "INIT no context", data: []string{"%K:\t.LONG\t1", ascid("%J", "|!D0|!H04|")},
			call: "LIB$INIT_DATE_TIME_CONTEXT", args: []string{"0", "%K", "%J"}},
		probeCase{name: "INIT 2 arguments", data: []string{"%X:\t.LONG\t0", "%K:\t.LONG\t1"},
			call: "LIB$INIT_DATE_TIME_CONTEXT", args: []string{"%X", "%K"}},
	)

	// FREE: a zero cell, a context, a stale copy, the static one.
	{
		data := []string{"%X:\t.LONG\t0", "%Y:\t.LONG\t0", "%K:\t.LONG\t2", ascid("%J", "|!DB-!MAAU-!Y4 !H04:!M0:!S0.!C2|"),
			"%S0:\t.LONG\t0", "%S1:\t.LONG\t0", "%S2:\t.LONG\t0", "%S3:\t.LONG\t0", "%Z:\t.LONG\t0"}
		var pre []string
		pre = append(pre, callLines("LIB$FREE_DATE_TIME_CONTEXT", "%X")...)
		pre = append(pre, "MOVL\tR0,%S0")
		pre = append(pre, callLines("LIB$INIT_DATE_TIME_CONTEXT", "%X", "%K", "%J")...)
		pre = append(pre, "MOVL\t%X,%Y")
		pre = append(pre, callLines("LIB$FREE_DATE_TIME_CONTEXT", "%X")...)
		pre = append(pre, "MOVL\tR0,%S1", "CLRL\t%Z", "TSTL\t%X", "BEQL\t%NZ", "MOVL\t#1,%Z", "%NZ:")
		pre = append(pre, callLines("LIB$FREE_DATE_TIME_CONTEXT", "%Y")...)
		pre = append(pre, "MOVL\tR0,%S2")
		pre = append(pre, callLines("LIB$FREE_DATE_TIME_CONTEXT")...)
		pre = append(pre, "MOVL\tR0,%S3", "MOVL\t%S0,R0")
		cs = append(cs, probeCase{name: "FREE", data: data, pre: pre,
			show: []show{mem("%S0", 1), mem("%S1", 1), mem("%Z", 1), mem("%S2", 1), mem("%S3", 1)}})
	}

	return cs
}

func input() []probeCase {
	std := english(nil, nil)
	manual12 := setup{inits: []initStep{{kInput, "|" + manualInput + "|"}}}
	y2 := setup{inits: []initStep{{kInput, "|!MN0/!D0/!Y2 !H04:!M0:!S0.!C2|"}}}
	packed := setup{inits: []initStep{{kInput, "|!Y4!MN0!D0 !H04!M0!S0.!C2|"}}}
	ticks := setup{inits: []initStep{{kInput, "|!DB-!MAAU-!Y4 !H04:!M0:!S0.!C7|"}}}
	timeFirst := setup{inits: []initStep{{kInput, "|!H04:!M0:!S0.!C2 !D0-!MN0-!Y4|"}}}
	numeric := setup{inits: []initStep{{kInput, "|!D0.!MN0.!Y4 !H04:!M0:!S0.!C2|"}}}

	var cs []probeCase

	// The manual's Table 3-2, every field omittable, its current date
	// from the defaults vector.
	for _, in := range []string{"31", "21", "-MAR", "-SEPTEMBER", "-1988", "23:", ":45:", "::23", ".01"} {
		cs = append(cs, convertCase("CONVERT "+in, std, in, "^X7F", "DEFV"))
	}

	cs = append(cs,
		convertCase("CONVERT whole", std, "13-JAN-1988 09:13:25.14", "", ""),
		convertCase("CONVERT whole flags 0", std, "13-JAN-1988 09:13:25.14", "0", ""),
		convertCase("CONVERT date only", std, "13-JAN-1988", "", ""),
		convertCase("CONVERT no seconds", std, "13-JAN-1988 09:13", "", ""),
		convertCase("CONVERT no seconds colon", std, "13-JAN-1988 09:13:", "", ""),
		convertCase("CONVERT one-digit hour", std, "13-JAN-1988 9", "", ""),
		convertCase("CONVERT one-digit fraction", std, "13-JAN-1988 09:13:25.1", "", ""),
		convertCase("CONVERT three-digit fraction", std, "13-JAN-1988 09:13:25.149", "", ""),
		convertCase("CONVERT lower case", std, "13-jan-1988 09:13:25.14", "", ""),
		convertCase("CONVERT full month", std, "13-JANUARY-1988", "", ""),
		convertCase("CONVERT month ja", std, "13-JA-1988", "", ""),
		convertCase("CONVERT month ju", std, "13-JU-1988", "", ""),
		convertCase("CONVERT month ma", std, "13-MA-1988", "", ""),
		convertCase("CONVERT month jun", std, "13-JUN-1988", "", ""),
		convertCase("CONVERT blanks", std, "   9-OCT-1990   01:02:03.04   ", "", ""),
		convertCase("CONVERT tab", std, "9-OCT-1990\t01:02:03.04", "", ""),
		convertCase("CONVERT two blanks", std, "9-OCT-1990  01:02:03.04", "", ""),
		convertCase("CONVERT no blank", std, "9-OCT-199001:02:03.04", "", ""),
		convertCase("CONVERT blank in date", std, "9 -OCT-1990", "", ""),
		convertCase("CONVERT 29 Feb 1989", std, "29-FEB-1989", "", ""),
		convertCase("CONVERT hour 24", std, "13-JAN-1988 24:00", "", ""),
		convertCase("CONVERT minute 60", std, "13-JAN-1988 09:60", "", ""),
		convertCase("CONVERT year 1858", std, "16-NOV-1858", "", ""),
		convertCase("CONVERT year 88", std, "13-JAN-88", "", ""),
		convertCase("CONVERT slashes", std, "13/JAN/1988", "", ""),
		convertCase("CONVERT hello", std, "hello", "", ""),
		convertCase("CONVERT empty", std, "", "", ""),
		convertCase("CONVERT empty flags 7F", std, "", "^X7F", "DEFV"),
		convertCase("CONVERT trailing junk", std, "13-JAN-1988 09:13:25.14x", "", ""),
		convertCase("CONVERT -MAR default flags", std, "-MAR", "", "DEFV"),
		convertCase("CONVERT no seconds flags 0", std, "13-JAN-1988 09:13", "0", ""),
		convertCase("CONVERT high flag bits", std, "-MAR", "^XFFFFFF7F", "DEFV"),
		convertCase("CONVERT DEFV2 -MAR", std, "-MAR", "^X7F", "DEFV2"),
		convertCase("CONVERT DEFV2 :45", std, ":45", "^X7F", "DEFV2"),
		convertCase("CONVERT DEFV2 date only", std, "13-JAN-1988", "", "DEFV2"),

		convertCase("CONVERT 12-hour am", manual12, "feb 25, 1988 04:39:02:55 am", "", ""),
		convertCase("CONVERT 12-hour pm", manual12, "FEBRUARY 25, 1988 04:39:02:55 PM", "", ""),
		convertCase("CONVERT 12-hour 12 am", manual12, "Feb 25, 1988 12:00:00:00 AM", "", ""),
		convertCase("CONVERT 12-hour 12 pm", manual12, "Feb 25, 1988 12:30:00:00 PM", "", ""),
		convertCase("CONVERT 12-hour p", manual12, "Feb 25, 1988 12:30:00:00 p", "", ""),
		convertCase("CONVERT 12-hour short", manual12, "Feb 25, 1988 4:39 pm", "", ""),
		convertCase("CONVERT 12-hour no meridiem", manual12, "Feb 25, 1988 4:39", "", ""),
		convertCase("CONVERT 12-hour pm only", manual12, "Feb 25, 1988 pm", "", ""),
		convertCase("CONVERT 12-hour 13 pm", manual12, "Feb 25, 1988 13:00:00:00 PM", "", ""),
		convertCase("CONVERT 12-hour 0 am", manual12, "Feb 25, 1988 0:00:00:00 AM", "", ""),
		convertCase("CONVERT 12-hour no blank", manual12, "Feb 25,1988", "", ""),
		convertCase("CONVERT y2 88", y2, "01/13/88 09:13:25.14", "", ""),
		convertCase("CONVERT y2 88 DEFV2", y2, "01/13/88", "", "DEFV2"),
		convertCase("CONVERT y2 short", y2, "1/3/8", "", "DEFV"),
		convertCase("CONVERT packed", packed, "19880113 091325.14", "", ""),
		convertCase("CONVERT packed date", packed, "19880113", "", ""),
		convertCase("CONVERT ticks", ticks, "13-JAN-1988 09:13:25.1234567", "", ""),
		convertCase("CONVERT ticks short", ticks, "13-JAN-1988 09:13:25.12", "", ""),
		convertCase("CONVERT ticks 8 digits", ticks, "13-JAN-1988 09:13:25.12345678", "", ""),
		convertCase("CONVERT time first", timeFirst, "09:13:25.14 13-01-1988", "", ""),
		convertCase("CONVERT time first date only", timeFirst, "13-01-1988", "", ""),
		convertCase("CONVERT numeric", numeric, "13.01.1988 09:13:25.14", "", ""),
		convertCase("CONVERT numeric one-digit", numeric, "3.1.1988", "", ""),
		convertCase("CONVERT names", english(nil, []string{"!D0.!MN0.!Y4 !H04:!M0:!S0.!C2"}), "13.01.1988 09:13:25.14", "", ""),
		convertCase("CONVERT names standard", english(nil, nil), "13-JAN-1988", "", ""),
		convertCase("CONVERT static nothing", setup{static: true, names: names{format: []string{}, input: []string{}}}, "13-JAN-1988", "", ""),
		convertCase("CONVERT names no fraction", english(nil, []string{"!D0.!MN0.!Y4 !H04:!M0:!S0"}), "13.01.1988 09:13:25", "", ""),

		convertCase("CONVERT ILL few fields", setup{inits: []initStep{{kInput, "|!D0-!MN0-!Y4 !H04:!M0:!S0|"}}}, "13-01-1988", "", ""),
		convertCase("CONVERT ILL mixed groups", setup{inits: []initStep{{kInput, "|!D0-!MN0 !H04:!M0:!S0.!C2 !Y4|"}}}, "13-01 09:13:25.14 1988", "", ""),
		convertCase("CONVERT ILL 24-hour meridiem", setup{inits: []initStep{{kInput, "|!D0-!MN0-!Y4 !H04:!M0:!S0.!C2 !MIU|"}}}, "13-01-1988", "", ""),
		convertCase("CONVERT ILL 12-hour no meridiem", setup{inits: []initStep{{kInput, "|!D0-!MN0-!Y4 !H02:!M0:!S0.!C2|"}}}, "13-01-1988", "", ""),
		convertCase("CONVERT ILL weekday", setup{inits: []initStep{{kInput, "|!WAU !D0-!MN0-!Y4 !H04:!M0:!S0.!C2|"}}}, "WED 13-01-1988", "", ""),
		convertCase("CONVERT ILL lower case", setup{inits: []initStep{{kInput, "|!d0-!mn0-!y4 !h04:!m0:!s0.!c2|"}}}, "13-01-1988", "", ""),
		convertCase("CONVERT ILL unknown", setup{inits: []initStep{{kInput, "|!DB-!XX|"}}}, "13-01-1988", "", ""),
		convertCase("CONVERT ILL empty", setup{inits: []initStep{{kInput, "||"}}}, "13-01-1988", "", ""),
		convertCase("CONVERT ILL two months", setup{inits: []initStep{{kInput, "|!D0-!MN0-!MAAU-!Y4 !H04:!M0:!S0.!C2|"}}}, "13-01-JAN-1988", "", ""),

		probeCase{name: "CONVERT 1 argument", data: []string{ascid("%I", "13-JAN-1988")},
			call: "LIB$CONVERT_DATE_STRING", args: []string{"%I"}},
		probeCase{name: "CONVERT 7 arguments", data: []string{ascid("%I", "13-JAN-1988"), "%R:\t" + unchanged},
			call: "LIB$CONVERT_DATE_STRING", args: []string{"%I", "%R", "0", "0", "0", "0", "0"}, show: []show{mem("%R", 2)}},
		probeCase{name: "CONVERT no result", data: []string{ascid("%I", "13-JAN-1988")},
			call: "LIB$CONVERT_DATE_STRING", args: []string{"%I", "0"}},
		probeCase{name: "CONVERT no string", data: []string{"%R:\t" + unchanged},
			call: "LIB$CONVERT_DATE_STRING", args: []string{"0", "%R"}, show: []show{mem("%R", 2)}},
	)

	// The relative days, and the date the defaults give, against TODAY.
	today := convertCase("CONVERT TODAY", std, "TODAY", "", "")
	today.post = append(today.post, "MOVQ\t%R,TODAYT")
	today.show = []show{vtim("%R"), mem("%M", 1)}

	cs = append(cs, today,
		relativeCase("CONVERT YESTERDAY", std, "YESTERDAY", "", ""),
		relativeCase("CONVERT TOMORROW", std, "TOMORROW", "", ""),
		relativeCase("CONVERT today lower", std, "today", "", ""),
		relativeCase("CONVERT tom", std, "tom", "", ""),
		relativeCase("CONVERT y", std, "y", "", ""),
		relativeCase("CONVERT T", std, "T", "", ""),
		relativeCase("CONVERT TO", std, "TO", "", ""),
		relativeCase("CONVERT TODAY 10:00", std, "TODAY 10:00", "", ""),
		relativeCase("CONVERT TODAY DEFV2", std, "TODAY", "", "DEFV2"),
		relativeCase("CONVERT TODAY flags 0", std, "TODAY", "0", ""),
		relativeCase("CONVERT 10:00 no defaults", std, "10:00", "^X7F", ""),
	)

	// Today's year and day, March's month: only the fields defaulted
	// are the same every run.
	march := convertCase("CONVERT -MAR no defaults", std, "-MAR", "^X7F", "")
	march.show = []show{vtim("%R"), mem("%M", 1)}
	cs = append(cs, march)

	return cs
}

// cases is every case, in the order they run.
var cases = func() []probeCase {
	var cs []probeCase

	for _, group := range [][]probeCase{arithmetic(), conversion(), simple(), formatting(), input()} {
		cs = append(cs, group...)
	}

	return cs
}()

// The program's fixed parts: its data (the output line, the FAO control
// strings), the handler, and the routines that print.
const programHead = `	.TITLE	LIB56	Phase 56 LIBRTL date and time probe
	.IDENT	/V1.0/

; Written by testdata/lib56/gen.go: do not edit. Calls each of Phase
; 56's LIBRTL routines, case by case, each in a routine of its own under
; HANDLER, and prints with LIB$PUT_OUTPUT what it returned in R0, any
; condition it signaled, and what it stored. See gen.go.

	$SSDEF
	$LNMDEF

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
LINLEN:	.WORD	0
TIMLEN:	.WORD	0
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
TIMDSC:	.WORD	23
	.BYTE	14,1
	.ADDRESS	TIMBUF
LINE:	.BLKB	132
TIMBUF:	.BLKB	23
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
VMEMCTL:	.ASCID	"  ~!AC:"
LWCTL:	.ASCID	" !XL"
TXTCTL:	.ASCID	/  !AC: !UL "!AF"/
VTXTCTL:	.ASCID	/  ~!AC: !UL "!AF"/
TIMCTL:	.ASCID	/  !AC: "!AD"/
VTIMCTL:	.ASCID	/  ~!AC: "!AD"/
BADTCTL:	.ASCID	/  !AC: ASCTIM !XL/
VBADTCTL:	.ASCID	/  ~!AC: ASCTIM !XL/
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

; SHOWVS name, descriptor, control: a varying string's CURLEN and text.
	.ENTRY	SHOWVS,^M<R2>
	MOVL	8(AP),R2
	MOVL	4(R2),R2		; CURLEN, then the text
	MOVZWL	(R2)+,R0
	$FAO_S	CTRSTR=@12(AP),OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=R2
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; SHOWTIM name, address, control, bad: the quadword time at address as
; $ASCTIM writes it, after the name as control (TIMCTL or VTIMCTL) shows
; it, or $ASCTIM's failure as bad (BADTCTL or VBADTCTL) shows it.
	.ENTRY	SHOWTIM,^M<>
	PUSHL	#0
	PUSHL	8(AP)
	PUSHAQ	TIMDSC
	PUSHAW	TIMLEN
	CALLS	#4,G^SYS$ASCTIM
	BLBC	R0,1$
	MOVZWL	TIMLEN,R0
	$FAO_S	CTRSTR=@12(AP),OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=#TIMBUF
	BRB	2$
1$:	$FAO_S	CTRSTR=@16(AP),OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0
2$:	MOVW	LINLEN,OUTDSC
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

		if len(c.name) > 31 {
			log.Fatalf("case %d's name %q is too long", n, c.name)
		}

		fmt.Fprintf(&b, "C%d_NAME:\t.ASCIC\t\"%s\"\n", n, c.name)

		for k, sh := range c.show {
			fmt.Fprintf(&b, "C%d_W%d:\t.ASCIC\t\"%s\"\n", n, k+1, shownName(sh))
		}
	}

	b.WriteString(programCode)

	// Each case's routine.
	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, c.name, n)

		for _, line := range c.pre {
			b.WriteString(tab(prefix(line, n)) + "\n")
		}

		if c.call != "" {
			for _, line := range callLines(c.call, c.args...) {
				b.WriteString("\t" + prefix(line, n) + "\n")
			}
		}

		// The post instructions keep R0: it's saved around them.
		b.WriteString("\tPUSHL\tR0\n")

		for _, line := range c.post {
			b.WriteString(tab(prefix(line, n)) + "\n")
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
			memCtl, txtCtl, timCtl, badCtl := "MEMCTL", "TXTCTL", "TIMCTL", "BADTCTL"
			if sh.vary {
				memCtl, txtCtl, timCtl, badCtl = "VMEMCTL", "VTXTCTL", "VTIMCTL", "VBADTCTL"
			}

			switch sh.kind {
			case "mem":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_W%d\n\tCALLS\t#4,SHOWMEM\n",
					memCtl, sh.count, prefix(sh.label, n), n, k+1)
			case "txt":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_W%d\n\tCALLS\t#3,SHOWTXT\n",
					txtCtl, prefix(sh.label, n), n, k+1)
			case "vs":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_W%d\n\tCALLS\t#3,SHOWVS\n",
					txtCtl, prefix(sh.label, n), n, k+1)
			case "time":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHAQ\t%s\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_W%d\n\tCALLS\t#4,SHOWTIM\n",
					badCtl, timCtl, prefix(sh.label, n), n, k+1)
			}
		}
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// tab indents an instruction, but not a label that begins a line.
func tab(line string) string {
	if strings.HasSuffix(line, ":") {
		return line
	}

	return "\t" + line
}

// command is lib56.com: it builds and runs the probe, the program's
// output in its own file. It shows the names the program expects
// undefined, VMS's predefined formats (LNM$DT_FORMAT_TABLE), and the
// language tables, if any.
const command = `$ ! LIB56.COM - the Phase 56 probe of LIBRTL's date and time routines.
$ ! Written by testdata/lib56/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB56/OUTPUT=LIB56.LOG
$ !
$ ! LIB56.OUT gets the program's output; LIB56.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ SHOW LOGICAL SYS$LANGUAGE
$ SHOW LOGICAL LIB$DT_FORMAT
$ SHOW LOGICAL LIB$DT_INPUT_FORMAT
$ SHOW LOGICAL/TABLE=LNM$SYSTEM_DIRECTORY LNM$DT*
$ SHOW LOGICAL/TABLE=LNM$SYSTEM_DIRECTORY LNM$LANGUAGE*
$ SHOW LOGICAL/TABLE=LNM$DT_FORMAT_TABLE *
$ ! The predefined formats: VMS's own startup procedure, if this system
$ ! has it and hasn't run it. It defines names until the system reboots.
$ IF F$TRNLNM("LIB$DATE_FORMAT_002","LNM$DT_FORMAT_TABLE") .NES. "" THEN GOTO HAVE_FORMATS
$ STARTUP = F$SEARCH("SYS$STARTUP:LIB$DT_STARTUP.COM")
$ IF STARTUP .EQS. "" THEN STARTUP = F$SEARCH("SYS$MANAGER:LIB$DT_STARTUP.COM")
$ WRITE SYS$OUTPUT "LIB$DT_STARTUP: ", STARTUP
$ ! Its lines are DIGITAL's file, not output: not shown.
$ SET NOVERIFY
$ IF STARTUP .NES. "" THEN @'STARTUP'
$ SET VERIFY
$ HAVE_FORMATS:
$ SHOW LOGICAL/TABLE=LNM$DT_FORMAT_TABLE *
$ SHOW LOGICAL SYS$LANGUAGE
$ SHOW LOGICAL LIB$DT_FORMAT
$ SHOW LOGICAL LIB$DT_INPUT_FORMAT
$ SHOW LOGICAL/TABLE=LNM$SYSTEM_DIRECTORY LNM$LANGUAGE*
$ SHOW LOGICAL/TABLE=LNM$LANGUAGE_ENGLISH *
$ SHOW LOGICAL/TABLE=LNM$LANGUAGE_FRENCH *
$ WRITE SYS$OUTPUT F$TIME()
$ DEFINE/USER_MODE SYS$OUTPUT LIB56.MLG
$ MACRO/NOLIST LIB56
$ LINK/NOMAP LIB56
$ DEFINE/USER_MODE SYS$OUTPUT LIB56.OUT
$ RUN LIB56
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT F$TIME()
$ DIRECTORY/SIZE/DATE
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 56 probe's exchange volume with
! govax. Written by testdata/lib56/gen.go. Run from the repository root:
!
!     govax console < testdata/lib56/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib56-exchange.dsk" /DEVICE=RD51 LIB56
MOUNT/WRITE DUA1 "testdata/disks/lib56-exchange.dsk"
COPY "testdata/lib56/lib56.mar"/HOST DUA1:[000000]LIB56.MAR
COPY "testdata/lib56/lib56.com"/HOST DUA1:[000000]LIB56.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib56")

	for name, text := range map[string]string{
		"lib56.mar":    program(),
		"lib56.com":    command,
		"exchange.cmd": exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
