//go:build ignore

// gen.go writes the Phase 57 probe of LIBRTL's CLI routines
// (docs/PHASE-57, subtask 10). Run it from the repository root:
//
//	go run testdata/lib57/gen.go
//
// Most of what the phase left open is what DCL does around an image, so
// the probe is a command procedure, lib57.com, that runs a program,
// lib57.mar, once per group of cases (the group named by a foreign
// command's parameter, "LIB57 SYM"), and shows DCL's state after each
// run: SHOW SYMBOL, $STATUS, F$ENVIRONMENT, SHOW LOGICAL. Its log, with
// SET VERIFY, interleaves the program's lines with DCL's, and
// TestLib57Oracle compares govax's with VMS's section by section.
//
// The program's cases work as Phase 55's (testdata/lib55/gen.go): each
// runs in a routine of its own under a condition handler (HANDLER) that
// records the condition it's given and unwinds, so the output tells a
// returned status from a signaled one. Its line is "name R0=xxxxxxxx",
// then a line per value it shows. A chaining case (LIB$DO_COMMAND,
// LIB$RUN_PROGRAM) that works ends the image, so it prints nothing; it
// is the last of its group.
//
// The files:
//
//   - lib57.mar: the program. With no foreign command text (as in the
//     process NOCLI makes with $CREPRC, which has no CLI), it runs the
//     NOCLIRUN group.
//   - lib57b.mar: the image LIB$RUN_PROGRAM runs: it shows what
//     LIB$GET_FOREIGN and LIB$GET_COMMON give it, and runs itself once
//     more when the common area starts with "AGAIN".
//   - lib57.com: the probe, sections S1 to S9.
//   - lib57s.com: the nested procedure (a group run at command level 2).
//   - lib57c.com: the procedure LIB$DO_COMMAND runs with @.
//   - lib57p.com: the spawned subprocess's procedure (SET NOCONTROL).
//   - lib57i.com: the interactive part's procedure (README.md's table).
//   - def_libcli.mar: $LIBCLIDEF GLOBAL, for govax's macro library.
//   - exchange.cmd: the govax console script that makes the exchange
//     volume.
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
	name string   // printed first (at most 31 characters)
	data []string // data lines; "%" is the case's label prefix
	pre  []string // instructions before the call
	call string   // the routine; "" for none (pre's R0 is the case's)
	args []string // its arguments, first first
	post []string // instructions after the call, R0 preserved
	show []show
}

// group is a set of cases the program runs for one foreign command
// parameter.
type group struct {
	name  string
	cases []probeCase
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
// The text is delimited by "|", which no case's text holds.
func ascid(label, s string) string {
	return fmt.Sprintf("%s:\t.ASCID\t|%s|", label, s)
}

// long is a class S descriptor at label for an n-character text: first,
// then fill repeated to make up the length.
func long(label string, n int, first, fill string) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T\n%s_T:\t.ASCII\t|%s|\n\t.REPEAT\t%d\n\t.ASCII\t|%s|\n\t.ENDR",
		label, n, label, label, first, n-len(first), fill)
}

// dyn is an empty dynamic string descriptor, class D, type T.
func dyn(label string) string {
	return fmt.Sprintf("%s:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0", label)
}

// fixed is a class S string of n dashes at label (its text at label_T).
func fixed(label string, n int) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T\n%s_T:\t.ASCII\t|%s|",
		label, n, label, label, strings.Repeat("-", n))
}

// preset is a longword at label holding ^X55555555, to show whether a
// routine wrote it; presetWord a word holding ^X5555 (in a longword, so
// it shows as one).
func preset(label string) string     { return label + ":\t.LONG\t^X55555555" }
func presetWord(label string) string { return label + ":\t.WORD\t^X5555,0" }

// longword is a longword at label holding v.
func longword(label string, v string) string { return label + ":\t.LONG\t" + v }

// The masks of $LIBCLIDEF (govax's macro library doesn't have it yet).
const (
	ctrlT = "^X00100000"
	ctrlY = "^X02000000"
)

// The data every case may use.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"MYPID:\t.LONG\t0\t\t; this process's PID",
	"MYJPI:\t.WORD\t4,JPI$_PID",
	"\t.ADDRESS\tMYPID",
	"\t.LONG\t0,0",
	"EXHBLK:\t.LONG\t0\t\t; the exit handler's control block",
	"\t.ADDRESS\tEXHRTN",
	"\t.LONG\t1",
	"\t.ADDRESS\tEXHSTS",
	"EXHSTS:\t.LONG\t0",
	"EXHCTL:\t.ASCID\t|  exit handler: status !XL|",
}

// getSym is a LIB$GET_SYMBOL of name into a 40-character fixed string,
// with its resultant length and table indicator preset.
func getSym(label, name string) probeCase {
	return probeCase{
		name: label,
		data: []string{ascid("%N", name), fixed("%B", 40), presetWord("%L"), preset("%T")},
		call: "LIB$GET_SYMBOL",
		args: []string{"%N", "%B", "%L", "%T"},
		show: []show{txt("%B"), mem("%L", 1), mem("%T", 1)},
	}
}

// setSym is a LIB$SET_SYMBOL of name to value, in table (a longword's
// value, or "" to omit the argument).
func setSym(label, name, value, table string) probeCase {
	c := probeCase{
		name: label,
		data: []string{ascid("%N", name), ascid("%V", value)},
		call: "LIB$SET_SYMBOL",
		args: []string{"%N", "%V"},
	}

	if table != "" {
		c.data = append(c.data, longword("%T", table))
		c.args = append(c.args, "%T")
	}

	return c
}

// setLong is a LIB$SET_SYMBOL with a long name or value: the descriptor
// lines given.
func setLong(label string, name, value string) probeCase {
	return probeCase{
		name: label,
		data: []string{name, value},
		call: "LIB$SET_SYMBOL",
		args: []string{"%N", "%V"},
	}
}

// delSym is a LIB$DELETE_SYMBOL of name from table ("" to omit it).
func delSym(label, name, table string) probeCase {
	c := probeCase{
		name: label,
		data: []string{ascid("%N", name)},
		call: "LIB$DELETE_SYMBOL",
		args: []string{"%N"},
	}

	if table != "" {
		c.data = append(c.data, longword("%T", table))
		c.args = append(c.args, "%T")
	}

	return c
}

// setLnm is a LIB$SET_LOGICAL. value and table are texts ("-" to omit
// the argument), attr a longword's value ("" to omit it), and items the
// item list's strings (none: no item list).
func setLnm(label, name, value, table, attr string, items ...string) probeCase {
	c := probeCase{
		name: label,
		data: []string{ascid("%N", name)},
		call: "LIB$SET_LOGICAL",
		args: []string{"%N", "0", "0", "0", "0"},
	}

	if value != "-" {
		c.data = append(c.data, ascid("%V", value))
		c.args[1] = "%V"
	}

	if table != "-" {
		c.data = append(c.data, ascid("%T", table))
		c.args[2] = "%T"
	}

	if attr != "" {
		c.data = append(c.data, longword("%A", attr))
		c.args[3] = "%A"
	}

	if len(items) > 0 {
		il := "%IL:"
		for i, s := range items {
			il += fmt.Sprintf("\t.WORD\t%d,LNM$_STRING\n\t.ADDRESS\t%%I%d\n\t.LONG\t0\n", len(s), i)
		}

		il += "\t.LONG\t0"
		c.data = append(c.data, il)

		for i, s := range items {
			c.data = append(c.data, fmt.Sprintf("%%I%d:\t.ASCII\t|%s|", i, s))
		}

		c.args[4] = "%IL"
	}

	// Trailing omitted arguments aren't passed at all.
	for len(c.args) > 1 && c.args[len(c.args)-1] == "0" {
		c.args = c.args[:len(c.args)-1]
	}

	return c
}

// delLnm is a LIB$DELETE_LOGICAL of name, from table ("-" to omit it).
func delLnm(label, name, table string) probeCase {
	c := probeCase{
		name: label,
		data: []string{ascid("%N", name)},
		call: "LIB$DELETE_LOGICAL",
		args: []string{"%N"},
	}

	if table != "-" {
		c.data = append(c.data, ascid("%T", table))
		c.args = append(c.args, "%T")
	}

	return c
}

// getCommon is a LIB$GET_COMMON into a dynamic string, with its
// resultant length.
func getCommon(label string) probeCase {
	return probeCase{
		name: label,
		data: []string{dyn("%D"), presetWord("%L")},
		call: "LIB$GET_COMMON",
		args: []string{"%D", "%L"},
		show: []show{txt("%D"), mem("%L", 1)},
	}
}

// putCommon is a LIB$PUT_COMMON of the descriptor line given, with its
// resultant length.
func putCommon(label, source string) probeCase {
	return probeCase{
		name: label,
		data: []string{source, presetWord("%L")},
		call: "LIB$PUT_COMMON",
		args: []string{"%S", "%L"},
		show: []show{mem("%L", 1)},
	}
}

// ctrl is a LIB$DISABLE_CTRL or LIB$ENABLE_CTRL of mask, with old-mask
// preset (or omitted, when noOld).
func ctrl(label, routine, mask string, noOld bool) probeCase {
	c := probeCase{
		name: label,
		data: []string{longword("%M", mask), preset("%O")},
		call: routine,
		args: []string{"%M", "%O"},
		show: []show{mem("%O", 1)},
	}

	if noOld {
		c.args = c.args[:1]
	}

	return c
}

// doCommand is a LIB$DO_COMMAND of the descriptor line given.
func doCommand(label, command string) probeCase {
	return probeCase{name: label, data: []string{command}, call: "LIB$DO_COMMAND", args: []string{"%C"}}
}

// runProgram is a LIB$RUN_PROGRAM of the descriptor line given.
func runProgram(label, program string) probeCase {
	return probeCase{name: label, data: []string{program}, call: "LIB$RUN_PROGRAM", args: []string{"%P"}}
}

// exitHandler declares the exit handler that prints its status.
var exitHandler = probeCase{name: "DCLEXH", pre: []string{"$DCLEXH_S\tDESBLK=EXHBLK"}}

// attach is a LIB$ATTACH of the PID that pre leaves in %P, shown after
// unless it's this process's own (which differs from system to system).
func attach(label string, pre ...string) probeCase {
	c := probeCase{
		name: label,
		data: []string{longword("%P", "0")},
		pre:  pre,
		call: "LIB$ATTACH",
		args: []string{"%P"},
		show: []show{mem("%P", 1)},
	}

	if len(pre) > 0 && pre[0] == getMyPID[0] {
		c.show = nil
	}

	return c
}

// getMyPID is the pre instructions that put this process's PID in %P.
var getMyPID = []string{"$GETJPIW_S\tITMLST=MYJPI", "MOVL\tMYPID,%P"}

// pidFromSymbol is the pre instructions that put LIB57_PID's value, in
// hex, in %P.
var pidFromSymbol = []string{
	"PUSHAW\t%L",
	"PUSHAQ\t%B",
	"PUSHAQ\t%S",
	"CALLS\t#3,G^LIB$GET_SYMBOL",
	"BLBC\tR0,%X",
	"PUSHAL\t%P",
	"PUSHAB\t%B_T",
	"MOVZWL\t%L,-(SP)",
	"CALLS\t#3,G^LIB$CVT_HTB",
	"%X:",
}

var pidSymbolData = []string{ascid("%S", "LIB57_PID"), fixed("%B", 16), presetWord("%L")}

// The groups, in the order lib57.com runs them.
var groups = []group{
	// S1: symbols, at command level 1. lib57.com defines G1, L1,
	// ABBR*EV, NUM, BOTH (local and global), AMB*IG, DEL_L, DEL_G,
	// DEL_W, and DEL_AB*BR first.
	{"SYM", []probeCase{
		getSym("GET G1", "G1"),
		getSym("GET L1", "L1"),
		getSym("GET l1 lowercase", "l1"),
		getSym("GET trailing blank", "L1 "),
		getSym("GET leading blank", " L1"),
		getSym("GET ABBR", "ABBR"),
		getSym("GET ABBREV", "ABBREV"),
		getSym("GET ABBREVI", "ABBREVI"),
		getSym("GET AB", "AB"),
		getSym("GET NUM", "NUM"),
		getSym("GET BOTH", "BOTH"),
		getSym("GET $STATUS", "$STATUS"),
		getSym("GET $SEVERITY", "$SEVERITY"),
		getSym("GET P1", "P1"),
		getSym("GET P9", "P9"),
		getSym("GET NOSUCH", "NOSUCH"),
		getSym("GET empty", ""),
		getSym("GET 1X", "1X"),
		getSym("GET _X", "_X"),
		getSym("GET -X", "-X"),
		getSym("GET $X", "$X"),
		getSym("GET *", "*"),
		getSym("GET X*Y", "X*Y"),
		{
			name: "GET 255-char name",
			data: []string{long("%N", 255, "Q", "Q"), fixed("%B", 40), presetWord("%L"), preset("%T")},
			call: "LIB$GET_SYMBOL", args: []string{"%N", "%B", "%L", "%T"},
			show: []show{mem("%L", 1), mem("%T", 1)},
		},
		{
			name: "GET 256-char name",
			data: []string{long("%N", 256, "Q", "Q"), fixed("%B", 40), presetWord("%L"), preset("%T")},
			call: "LIB$GET_SYMBOL", args: []string{"%N", "%B", "%L", "%T"},
			show: []show{mem("%L", 1), mem("%T", 1)},
		},
		{
			name: "GET into 5 characters",
			data: []string{ascid("%N", "G1"), fixed("%B", 5), presetWord("%L"), preset("%T")},
			call: "LIB$GET_SYMBOL", args: []string{"%N", "%B", "%L", "%T"},
			show: []show{txt("%B"), mem("%L", 1), mem("%T", 1)},
		},
		{
			name: "GET 2 arguments",
			data: []string{ascid("%N", "L1"), fixed("%B", 20)},
			call: "LIB$GET_SYMBOL", args: []string{"%N", "%B"},
			show: []show{txt("%B")},
		},
		{
			name: "GET 1 argument",
			data: []string{ascid("%N", "L1")},
			call: "LIB$GET_SYMBOL", args: []string{"%N"},
		},
		{
			name: "GET into dynamic",
			data: []string{ascid("%N", "G1"), dyn("%D"), presetWord("%L"), preset("%T")},
			call: "LIB$GET_SYMBOL", args: []string{"%N", "%D", "%L", "%T"},
			show: []show{txt("%D"), mem("%L", 1), mem("%T", 1)},
		},

		setSym("SET S_LOCAL", "S_LOCAL", "set local", ""),
		setSym("SET S_GLOBAL table 2", "S_GLOBAL", "set global", "2"),
		setSym("SET S_ONE table 1", "S_ONE", "set in table 1", "1"),
		setSym("SET S_ZERO table 0", "S_ZERO", "table 0", "0"),
		setSym("SET S_THREE table 3", "S_THREE", "table 3", "3"),
		setSym("SET s_lower", "s_lower", "lowercase name", ""),
		setSym("SET S_AB*BREV", "S_AB*BREV", "abbreviated", ""),
		setSym("SET _UNDER", "_UNDER", "underscore", ""),
		setSym("SET $DOLLAR", "$DOLLAR", "dollar", ""),
		setSym("SET 1DIGIT", "1DIGIT", "digit", ""),
		setSym("SET -DASH", "-DASH", "dash", ""),
		setSym("SET empty name", "", "empty", ""),
		setSym("SET S BLANK", "S BLANK", "embedded blank", ""),
		setSym("SET S_DASH-IN", "S_DASH-IN", "dash inside", ""),
		setLong("SET 255-char name", long("%N", 255, "S", "N"), ascid("%V", "long name")),
		setLong("SET 256-char name", long("%N", 256, "S", "N"), ascid("%V", "longer name")),
		setLong("SET S_V255", ascid("%N", "S_V255"), long("%V", 255, "<", "v")),
		setLong("SET S_V256", ascid("%N", "S_V256"), long("%V", 256, "<", "v")),
		setLong("SET S_V1024", ascid("%N", "S_V1024"), long("%V", 1024, "<", "v")),
		setLong("SET S_V4097", ascid("%N", "S_V4097"), long("%V", 4097, "<", "v")),
		setSym("SET $STATUS table 2", "$STATUS", "%X00000003", "2"),
		setSym("SET AM*BX", "AM*BX", "overlaps AMB*IG", ""),
		setSym("SET AMBIG", "AMBIG", "replaced", ""),
		setSym("SET S_EMPTY", "S_EMPTY", "", ""),
		setSym("SET S_CASE", "S_CASE", "Mixed Case value", ""),
		setSym("SET G1 local", "G1", "local G1", ""),
		{
			name: "SET 1 argument",
			data: []string{ascid("%N", "S_ONEARG")},
			call: "LIB$SET_SYMBOL", args: []string{"%N"},
		},

		delSym("DELETE DEL_L", "DEL_L", ""),
		delSym("DELETE DEL_G table 2", "DEL_G", "2"),
		delSym("DELETE DEL_W table 1", "DEL_W", "1"),
		delSym("DELETE NOSUCH", "NOSUCH", ""),
		delSym("DELETE DEL_AB", "DEL_AB", ""),
		delSym("DELETE DEL_L again", "DEL_L", ""),
		delSym("DELETE L1 table 0", "L1", "0"),
		delSym("DELETE L1 table 3", "L1", "3"),
		delSym("DELETE empty name", "", ""),
		delSym("DELETE num lowercase", "num", ""),
	}},

	// S2: at command level 2, from LIB57S.COM (P1 SYMOUT, P2 ""); level 1
	// has LOCOUT and LOCDEL.
	{"SYMOUT", []probeCase{
		getSym("GET LOCOUT", "LOCOUT"),
		getSym("GET P1", "P1"),
		getSym("GET P2", "P2"),
		setSym("SET S_INNER", "S_INNER", "inner", ""),
		setSym("SET S_INNERG table 2", "S_INNERG", "inner global", "2"),
		setSym("SET LOCOUT", "LOCOUT", "set at level 2", ""),
		delSym("DELETE LOCDEL", "LOCDEL", ""),
	}},

	// S3: logical names. lib57.com defines LIB57_D (process, supervisor),
	// LIB57_JD (job), and LIB57_U (process, user mode) first.
	{"LNM", []probeCase{
		setLnm("SET LIB57_A", "LIB57_A", "value a", "-", ""),
		setLnm("SET LIB57_A again", "LIB57_A", "value a2", "-", ""),
		setLnm("SET lib57_lower", "lib57_lower", "lowercase", "-", ""),
		setLnm("SET trailing blank", "LIB57_TRAIL ", "trailing blank", "-", ""),
		setLnm("SET LIB57_B LNM$JOB", "LIB57_B", "in the job table", "LNM$JOB", ""),
		setLnm("SET LIB57_C LNM$GROUP", "LIB57_C", "in the group table", "LNM$GROUP", ""),
		setLnm("SET LIB57_E no such table", "LIB57_E", "e", "LIB57_NO_TABLE", ""),
		setLnm("SET LIB57_F lowercase table", "LIB57_F", "f", "lnm$job", ""),
		setLnm("SET LIB57_LIST item list", "LIB57_LIST", "-", "-", "", "one", "two"),
		setLnm("SET LIB57_BOTH value and items", "LIB57_BOTH", "value", "-", "", "item"),
		setLnm("SET LIB57_NONE neither", "LIB57_NONE", "-", "-", ""),
		setLnm("SET LIB57_CONF CONFINE", "LIB57_CONF", "confined", "-", "LNM$M_CONFINE"),
		setLnm("SET LIB57_CONC CONCEALED", "LIB57_CONC", "CONCEALED_VALUE", "-", "LNM$M_CONCEALED"),
		setLnm("SET LIB57_TERM TERMINAL", "LIB57_TERM", "terminal", "-", "LNM$M_TERMINAL"),
		setLnm("SET LIB57_NOAL NO_ALIAS", "LIB57_NOAL", "no alias", "-", "LNM$M_NO_ALIAS"),
		setLnm("SET LIB57_BAD bad attribute", "LIB57_BAD", "bad", "-", "^X00010000"),
		setLnm("SET LIB57_ZERO attributes 0", "LIB57_ZERO", "zero", "-", "0"),
		setLnm("SET LIB57_ILA items CONCEALED", "LIB57_ILA", "-", "-", "LNM$M_CONCEALED", "item"),
		setLnm("SET LIB57_ILC items CONFINE", "LIB57_ILC", "-", "-", "LNM$M_CONFINE", "item"),
		setLnm("SET empty name", "", "empty name", "-", ""),
		setLnm("SET LIB57_EV empty value", "LIB57_EV", "", "-", ""),
		{
			name: "SET LIB57_V256 256 characters",
			data: []string{ascid("%N", "LIB57_V256"), long("%V", 256, "<", "v")},
			call: "LIB$SET_LOGICAL", args: []string{"%N", "%V"},
		},
		{
			name: "SET LIB57_V255 255 characters",
			data: []string{ascid("%N", "LIB57_V255"), long("%V", 255, "<", "v")},
			call: "LIB$SET_LOGICAL", args: []string{"%N", "%V"},
		},
		delLnm("DELETE LIB57_D", "LIB57_D", "-"),
		delLnm("DELETE LIB57_U user mode", "LIB57_U", "-"),
		delLnm("DELETE LIB57_NOSUCH", "LIB57_NOSUCH", "-"),
		delLnm("DELETE empty name", "", "-"),
		delLnm("DELETE LIB57_JD LNM$JOB", "LIB57_JD", "LNM$JOB"),
		delLnm("DELETE lib57_a lowercase", "lib57_a", "-"),
	}},

	// S4: the common area: COM1, then COM2 in the next image, and again
	// in a spawned subprocess.
	{"COM1", []probeCase{
		getCommon("GET first"),
		putCommon("PUT empty", ascid("%S", "")),
		getCommon("GET after empty"),
		putCommon("PUT 252", long("%S", 252, "<", "c")),
		getCommon("GET 252"),
		putCommon("PUT 253", long("%S", 253, "<", "d")),
		getCommon("GET after 253"),
		{
			name: "GET into 10 characters",
			data: []string{fixed("%B", 10), presetWord("%L")},
			call: "LIB$GET_COMMON", args: []string{"%B", "%L"},
			show: []show{txt("%B"), mem("%L", 1)},
		},
		{
			name: "PUT 1 argument",
			data: []string{ascid("%S", "short")},
			call: "LIB$PUT_COMMON", args: []string{"%S"},
		},
		{
			name: "GET 1 argument into 20",
			data: []string{fixed("%B", 20)},
			call: "LIB$GET_COMMON", args: []string{"%B"},
			show: []show{txt("%B")},
		},
		putCommon("PUT for the next image", ascid("%S", "for the next image")),
	}},
	{"COM2", []probeCase{getCommon("GET in the next image")}},

	// S5: LIB$DO_COMMAND, one group per case.
	{"DOSHOW", []probeCase{doCommand("DO SHOW SYMBOL", ascid("%C", "SHOW SYMBOL CHAINED"))}},
	{"DOAT", []probeCase{doCommand("DO @LIB57C", ascid("%C", "@LIB57C one TWO"))}},
	{"DOEMPTY", []probeCase{doCommand("DO empty", ascid("%C", ""))}},
	{"DOBLANK", []probeCase{doCommand("DO blanks", ascid("%C", "   "))}},
	{"DOFAIL", []probeCase{doCommand("DO FROBNICATE", ascid("%C", "FROBNICATE"))}},
	{"DOWRITE", []probeCase{doCommand("DO WRITE", ascid("%C", `WRITE SYS$OUTPUT "chained write"`))}},
	{"DO255", []probeCase{doCommand("DO 255 characters", long("%C", 255, "SHOW SYMBOL CHAINED ! ", "x"))}},
	{"DO256", []probeCase{doCommand("DO 256 characters", long("%C", 256, "SHOW SYMBOL CHAINED ! ", "x"))}},
	{"DOEXH", []probeCase{exitHandler, doCommand("DO after DCLEXH", ascid("%C", "SHOW SYMBOL CHAINED"))}},
	{"DOAGAIN", []probeCase{doCommand("DO LIB57 DOSHOW", ascid("%C", "LIB57 DOSHOW"))}},
	{"DOLOCAL", []probeCase{doCommand("DO a local symbol", ascid("%C", "CHAINED_LOCAL = 7"))}},
	{"DOEXIT", []probeCase{doCommand("DO EXIT 3", ascid("%C", "EXIT 3"))}},
	{"DOMSG", []probeCase{
		{
			name: "PUT_OUTPUT before",
			data: []string{ascid("%S", "before the chain")},
			call: "LIB$PUT_OUTPUT", args: []string{"%S"},
		},
		doCommand("DO with 2 arguments", ascid("%C", "SHOW SYMBOL CHAINED")),
	}},

	// S6: LIB$RUN_PROGRAM.
	{"RUNB", []probeCase{
		putCommon("PUT from LIB57", ascid("%S", "from LIB57")),
		runProgram("RUN LIB57B", ascid("%P", "LIB57B")),
	}},
	{"RUNAGAIN", []probeCase{
		putCommon("PUT AGAIN", ascid("%S", "AGAIN")),
		runProgram("RUN LIB57B again", ascid("%P", "LIB57B")),
	}},
	{"RUNEXH", []probeCase{
		putCommon("PUT after DCLEXH", ascid("%S", "after an exit handler")),
		exitHandler,
		runProgram("RUN after DCLEXH", ascid("%P", "LIB57B")),
	}},
	{"RUNSPEC", []probeCase{runProgram("RUN a full name", ascid("%P", "SYS$DISK:[]LIB57B.EXE;"))}},
	{"RUNNONE", []probeCase{runProgram("RUN NOSUCH_IMAGE", ascid("%P", "NOSUCH_IMAGE"))}},
	{"RUNEMPTY", []probeCase{runProgram("RUN empty", ascid("%P", ""))}},
	{"RUNARGS", []probeCase{runProgram("RUN with text after", ascid("%P", "LIB57B text after"))}},
	{"RUN256", []probeCase{runProgram("RUN 256 characters", long("%P", 256, "LIB57B", " "))}},

	// S7: the control keys, a group per change; lib57.com shows
	// F$ENVIRONMENT's CONTROL, NOCONTROL, and ON_CONTROL_Y after each.
	{"CTLQ", []probeCase{
		ctrl("DISABLE 0", "LIB$DISABLE_CTRL", "0", false),
		ctrl("ENABLE 0", "LIB$ENABLE_CTRL", "0", false),
	}},
	{"CTLET", []probeCase{ctrl("ENABLE T", "LIB$ENABLE_CTRL", ctrlT, false)}},
	{"CTLDT", []probeCase{ctrl("DISABLE T", "LIB$DISABLE_CTRL", ctrlT, false)}},
	{"CTLDY", []probeCase{ctrl("DISABLE Y", "LIB$DISABLE_CTRL", ctrlY, false)}},
	{"CTLEY", []probeCase{ctrl("ENABLE Y", "LIB$ENABLE_CTRL", ctrlY, false)}},
	{"CTLBOTH", []probeCase{
		ctrl("DISABLE T and Y", "LIB$DISABLE_CTRL", ctrlT+"!"+ctrlY, false),
		ctrl("ENABLE T and Y", "LIB$ENABLE_CTRL", ctrlT+"!"+ctrlY, false),
		ctrl("DISABLE T, no old-mask", "LIB$DISABLE_CTRL", ctrlT, true),
	}},
	{"CTLBAD", []probeCase{
		ctrl("DISABLE bit 0", "LIB$DISABLE_CTRL", "^X00000001", false),
		ctrl("ENABLE bit 0", "LIB$ENABLE_CTRL", "^X00000001", false),
		ctrl("DISABLE Y and bit 0", "LIB$DISABLE_CTRL", ctrlY+"!1", false),
		ctrl("ENABLE bit 30", "LIB$ENABLE_CTRL", "^X40000000", false),
		ctrl("ENABLE Ctrl/C bit", "LIB$ENABLE_CTRL", "^X00000008", false),
	}},

	// S8: LIB$ATTACH's refusals (S9 and the interactive part do the rest).
	{"ATTSELF", []probeCase{attach("ATTACH self", getMyPID...)}},
	{"ATTNONE", []probeCase{attach("ATTACH no such process", "MOVL\t#^X7FFFFFFF,%P")}},
	{"ATTZERO", []probeCase{attach("ATTACH 0")}},

	// S9: no CLI. NOCLI makes a process with $CREPRC (no CLI) that runs
	// this image with no command text, so NOCLIRUN; its SYS$INPUT is
	// LIB57N.DAT, for GET_COMMAND.
	{"NOCLI", []probeCase{{
		name: "CREPRC LIB57",
		pre:  []string{"CALLS\t#0,NOCLIPARENT"},
		show: []show{mem("FINALSTS", 1)},
	}}},
	{"NOCLIRUN", []probeCase{
		{name: "GET_FOREIGN", pre: []string{"MOVL\tFORSTAT,R0"}},
		getSym("GET_SYMBOL", "LIB57_DIR"),
		setSym("SET_SYMBOL", "NOCLI", "no CLI", ""),
		delSym("DELETE_SYMBOL", "NOCLI", ""),
		setLnm("SET_LOGICAL", "LIB57_N", "no CLI", "-", ""),
		delLnm("DELETE_LOGICAL", "LIB57_N", "-"),
		putCommon("PUT_COMMON", ascid("%S", "in a process with no CLI")),
		getCommon("GET_COMMON"),
		ctrl("DISABLE_CTRL", "LIB$DISABLE_CTRL", ctrlY, false),
		ctrl("ENABLE_CTRL", "LIB$ENABLE_CTRL", ctrlY, false),
		attach("ATTACH self", getMyPID...),
		{name: "PAUSE", call: "LIB$PAUSE"},
		{
			name: "GET_COMMAND",
			data: []string{ascid("%P", "Command: "), dyn("%D"), presetWord("%L")},
			call: "LIB$GET_COMMAND", args: []string{"%D", "%P", "%L"},
			show: []show{txt("%D"), mem("%L", 1)},
		},
		doCommand("DO_COMMAND", ascid("%C", "SHOW TIME")),
		runProgram("RUN_PROGRAM", ascid("%P", "LIB57B")),
	}},

	// The interactive part (lib57i.com and README.md's table).
	{"PAUSE", []probeCase{{name: "PAUSE", call: "LIB$PAUSE"}}},
	{"GETCMD", []probeCase{
		{
			name: "GET_COMMAND with a prompt",
			data: []string{ascid("%P", "Command: "), dyn("%D"), presetWord("%L")},
			call: "LIB$GET_COMMAND", args: []string{"%D", "%P", "%L"},
			show: []show{txt("%D"), mem("%L", 1)},
		},
		{
			name: "GET_INPUT after it",
			data: []string{dyn("%D"), presetWord("%L")},
			call: "LIB$GET_INPUT", args: []string{"%D", "0", "%L"},
			show: []show{txt("%D"), mem("%L", 1)},
		},
		{
			name: "GET_COMMAND into 5",
			data: []string{ascid("%P", "Short: "), fixed("%B", 5), presetWord("%L")},
			call: "LIB$GET_COMMAND", args: []string{"%B", "%P", "%L"},
			show: []show{txt("%B"), mem("%L", 1)},
		},
		{
			name: "GET_COMMAND no prompt",
			data: []string{dyn("%D")},
			call: "LIB$GET_COMMAND", args: []string{"%D"},
			show: []show{txt("%D")},
		},
	}},
	{"DOSHORT", []probeCase{doCommand("DO SHOW alone", ascid("%C", "SHOW"))}},
	{"ATTACH", []probeCase{{
		name: "ATTACH LIB57_PID",
		data: append(append([]string{}, pidSymbolData...), longword("%P", "0")),
		pre:  pidFromSymbol,
		call: "LIB$ATTACH",
		args: []string{"%P"},
		show: []show{mem("%P", 1)},
	}}},
}

const programHead = `	.TITLE	LIB57	Phase 57 LIBRTL CLI routines probe
	.IDENT	/V1.0/

; Written by testdata/lib57/gen.go: do not edit. Runs the group of cases
; its foreign command's parameter names (or, with none, NOCLIRUN), each case in a routine of its own under HANDLER, and
; prints with LIB$PUT_OUTPUT what it returned in R0, any condition it
; signaled, and what it stored. See gen.go.

	$SSDEF
	$JPIDEF
	$DVIDEF
	$LNMDEF
	$IODEF
	$ACCDEF

; CHECK: return the service's failure status in R0.
	.MACRO	CHECK	?OK
	BLBS	R0,OK
	BRW	FAIL
OK:
	.ENDM

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
LINLEN:	.WORD	0
	.ALIGN	LONG
LINDSC:	.WORD	400
	.BYTE	14,1
	.ADDRESS	LINE
PARTDSC:	.WORD	9
	.BYTE	14,1
	.LONG	0
OUTDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS	LINE
LINE:	.BLKB	400
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
LWCTL:	.ASCID	" !XL"
TXTCTL:	.ASCID	/  !AC: !UL "!AF"/
NOGRP:	.ASCID	/LIB57: no group "!AS"/

; The group's name: the foreign command's text, or NOCLIRUN.
FORSTAT:	.LONG	0		; LIB$GET_FOREIGN's status
SELLEN:	.WORD	0
SELDSC:	.WORD	80
	.BYTE	14,1
	.ADDRESS	SELBUF
SELOUT:	.WORD	0
	.BYTE	14,1
	.ADDRESS	SELBUF
SELBUF:	.BLKB	80
CHILDGRP:	.ASCII	"NOCLIRUN"

; NOCLIPARENT's data: the names it makes from LIB57_DIR, the termination
; mailbox, and the child's final status.
DIRSYM:	.ASCID	"LIB57_DIR"
DIRDSC:	.WORD	128
	.BYTE	14,1
	.ADDRESS	DIRBUF
DIRBUF:	.BLKB	128
DIRLEN:	.WORD	0
IMGCTL:	.ASCID	"!ASLIB57.EXE"
INCTL:	.ASCID	"!ASLIB57N.DAT"
OUTCTL:	.ASCID	"!ASLIB57N.OUT"
ERRCTL:	.ASCID	"!ASLIB57N.ERR"
IMGDSC:	.WORD	160
	.BYTE	14,1
	.ADDRESS	IMGBUF
IMGBUF:	.BLKB	160
INDSC:	.WORD	160
	.BYTE	14,1
	.ADDRESS	INBUF
INBUF:	.BLKB	160
OUTDSC2:	.WORD	160
	.BYTE	14,1
	.ADDRESS	OUTBUF2
OUTBUF2:	.BLKB	160
ERRDSC:	.WORD	160
	.BYTE	14,1
	.ADDRESS	ERRBUF
ERRBUF:	.BLKB	160
CHNAME:	.ASCID	"LIB57_NOCLI"
MBXCHAN:	.WORD	0,0
MBXUNIT:	.LONG	0
DVIITEMS:	.WORD	4,DVI$_UNIT
	.ADDRESS	MBXUNIT
	.LONG	0,0
CHILDPID:	.LONG	0
TERMMSG:	.BLKB	ACC$K_TERMLEN
IOSB:	.QUAD	0
FINALSTS:	.LONG	^X55555555
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

; EXHRTN is the exit handler DCLEXH declares: it prints the image's
; exit status.
	.ENTRY	EXHRTN,^M<>
	$FAO_S	CTRSTR=EXHCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=@4(AP)
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; NOCLIPARENT makes a process, by $CREPRC, that runs this image with
; LIB57N.DAT as its SYS$INPUT and LIB57N.OUT as its SYS$OUTPUT (the
; names in LIB57_DIR, a DCL symbol), and waits for its termination
; message, leaving its final status in FINALSTS.
	.ENTRY	NOCLIPARENT,^M<>
	PUSHAW	DIRLEN
	PUSHAQ	DIRDSC
	PUSHAQ	DIRSYM
	CALLS	#3,G^LIB$GET_SYMBOL
	CHECK
	MOVW	DIRLEN,DIRDSC
	$FAO_S	CTRSTR=IMGCTL,OUTLEN=IMGDSC,OUTBUF=IMGDSC,P1=#DIRDSC
	CHECK
	$FAO_S	CTRSTR=INCTL,OUTLEN=INDSC,OUTBUF=INDSC,P1=#DIRDSC
	CHECK
	$FAO_S	CTRSTR=OUTCTL,OUTLEN=OUTDSC2,OUTBUF=OUTDSC2,P1=#DIRDSC
	CHECK
	$FAO_S	CTRSTR=ERRCTL,OUTLEN=ERRDSC,OUTBUF=ERRDSC,P1=#DIRDSC
	CHECK
	$CREMBX_S	CHAN=MBXCHAN,MAXMSG=#128,BUFQUO=#512
	CHECK
	$GETDVIW_S	CHAN=MBXCHAN,ITMLST=DVIITEMS
	CHECK
	$CREPRC_S	PIDADR=CHILDPID,IMAGE=IMGDSC,INPUT=INDSC,OUTPUT=OUTDSC2,-
			ERROR=ERRDSC,PRCNAM=CHNAME,MBXUNT=MBXUNIT
	CHECK
	$QIOW_S	CHAN=MBXCHAN,FUNC=#IO$_READVBLK,IOSB=IOSB,-
		P1=TERMMSG,P2=#ACC$K_TERMLEN
	CHECK
	MOVL	TERMMSG+ACC$L_FINALSTS,FINALSTS
	MOVL	#SS$_NORMAL,R0
FAIL:	RET

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

; SHOWMEM name, address, count: count longwords in hex, after the name.
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

func program() string {
	var (
		b     strings.Builder
		cases []probeCase
		first []int // each group's first case number
	)

	for _, g := range groups {
		first = append(first, len(cases)+1)
		cases = append(cases, g.cases...)
	}

	b.WriteString(programHead)
	b.WriteString("\n; Shared data.\n")

	for _, line := range shared {
		b.WriteString(line + "\n")
	}

	for i, g := range groups {
		fmt.Fprintf(&b, "G%d_NAME:\t.ASCII\t\"%s\"\n", i+1, g.name)
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

	// Each group's routine: its cases, each followed by its lines.
	for gi, g := range groups {
		fmt.Fprintf(&b, "\n; Group %s.\n\t.ENTRY\tG%d,^M<>\n", g.name, gi+1)

		for k, c := range g.cases {
			n := first[gi] + k
			fmt.Fprintf(&b, "\tCLRL\tSIGCOND\t\t\t; %d: %s\n\tCALLS\t#0,C%d\n\tPUSHL\tR0\n\tPUSHAB\tC%d_NAME\n\tCALLS\t#2,SHOWR0\n",
				n, c.name, n, n)

			for s, sh := range c.show {
				switch sh.kind {
				case "mem":
					fmt.Fprintf(&b, "\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#3,SHOWMEM\n",
						sh.count, prefix(sh.label, n), n, s+1)
				case "txt":
					fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWTXT\n", prefix(sh.label, n), n, s+1)
				}
			}
		}

		b.WriteString("\tRET\n")
	}

	// The main routine: the group's name, then its routine.
	b.WriteString(`
	.ENTRY	START,^M<R2,R3,R4,R5>
	PUSHAW	SELLEN
	PUSHL	#0
	PUSHAQ	SELDSC
	CALLS	#3,G^LIB$GET_FOREIGN
	MOVL	R0,FORSTAT
	BLBC	R0,1$
	TSTW	SELLEN
	BNEQ	2$
1$:	MOVC3	#8,CHILDGRP,SELBUF	; no command text: NOCLI's child
	MOVW	#8,SELLEN
2$:	MOVW	SELLEN,SELOUT
	PUSHAQ	SELOUT
	PUSHAQ	SELOUT
	CALLS	#2,G^STR$UPCASE
`)

	for i, g := range groups {
		fmt.Fprintf(&b, "\tCMPC5\tSELLEN,SELBUF,#^A/ /,#%d,G%d_NAME\n\tBEQL\t%d$\n\tBRW\t%d$\n%d$:\tCALLS\t#0,G%d\n\tBRW\t99$\n%d$:\n",
			len(g.name), i+1, 2000+i, 1000+i, 2000+i, i+1, 1000+i)
	}

	b.WriteString(`	$FAO_S	CTRSTR=NOGRP,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=#SELOUT
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
99$:	MOVL	#SS$_NORMAL,R0
	RET
	.END	START
`)

	return b.String()
}

// chained is lib57b.mar: the image LIB$RUN_PROGRAM runs.
const chained = `	.TITLE	LIB57B	Phase 57 probe: the image LIB$RUN_PROGRAM runs
	.IDENT	/V1.0/

; Written by testdata/lib57/gen.go: do not edit. Shows what
; LIB$GET_FOREIGN and LIB$GET_COMMON give it; when the common area starts
; with "AGAIN", it puts "second time" there and runs itself once more
; with LIB$RUN_PROGRAM.

	.PSECT	DATA,NOEXE,WRT,QUAD
BUFLEN:	.WORD	0
	.ALIGN	LONG
BUFDSC:	.WORD	300
	.BYTE	14,1
	.ADDRESS	BUF
BUF:	.BLKB	300
TXTDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS	BUF
LINLEN:	.WORD	0
	.ALIGN	LONG
LINDSC:	.WORD	400
	.BYTE	14,1
	.ADDRESS	LINE
OUTDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS	LINE
LINE:	.BLKB	400
FORCTL:	.ASCID	/LIB57B GET_FOREIGN R0=!XL !UL "!AS"/
COMCTL:	.ASCID	/LIB57B GET_COMMON R0=!XL !UL "!AS"/
RUNCTL:	.ASCID	/LIB57B RUN_PROGRAM R0=!XL/
AGAIN:	.ASCII	/AGAIN/
SECOND:	.ASCID	/second time/
SELF:	.ASCID	/LIB57B/

	.PSECT	CODE,NOWRT,EXE
	.ENTRY	START,^M<R2,R3,R4,R5>
	CLRW	BUFLEN
	PUSHAW	BUFLEN
	PUSHL	#0
	PUSHAQ	BUFDSC
	CALLS	#3,G^LIB$GET_FOREIGN
	MOVW	BUFLEN,TXTDSC
	$FAO_S	CTRSTR=FORCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R0,P2=BUFLEN,P3=#TXTDSC
	BSBW	PRINT
	CLRW	BUFLEN
	PUSHAW	BUFLEN
	PUSHAQ	BUFDSC
	CALLS	#2,G^LIB$GET_COMMON
	MOVW	BUFLEN,TXTDSC
	$FAO_S	CTRSTR=COMCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R0,P2=BUFLEN,P3=#TXTDSC
	BSBW	PRINT
	CMPC5	BUFLEN,BUF,#0,#5,AGAIN
	BNEQ	9$
	PUSHL	#0
	PUSHAQ	SECOND
	CALLS	#2,G^LIB$PUT_COMMON
	PUSHAQ	SELF
	CALLS	#1,G^LIB$RUN_PROGRAM
	$FAO_S	CTRSTR=RUNCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R0
	BSBW	PRINT
9$:	MOVL	#1,R0
	RET

PRINT:	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RSB
	.END	START
`

// defLibcli is def_libcli.mar, the $LIBCLIDEF definition probe, as
// testdata/mp/defs's are.
const defLibcli = `	.TITLE	DEF_LIBCLI	$LIBCLIDEF's names and values
	.IDENT	/V1.0/
;
; A Phase 57 definition probe (testdata/lib57/README.md): the
; table-type indicators and control-key masks of the LIB$ CLI routines.
; With GLOBAL, every name the macro defines is a global symbol, so the
; object's GSD lists each name and its value.
;
	$LIBCLIDEF	GLOBAL
	.END
`

// command is lib57.com, the probe.
const command = `$ ! LIB57.COM - the Phase 57 probe of LIBRTL's CLI routines.
$ ! Written by testdata/lib57/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB57/OUTPUT=LIB57.LOG
$ !
$ ! DEF_LIBCLI.ANL gets the $LIBCLIDEF probe's analysis, LIB57.LOG the
$ ! rest.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ LIB57_DIR == DEV + "[000000]"
$ LIB57 :== $'DEV'[000000]LIB57.EXE
$ MACRO/NOLIST LIB57
$ MACRO/NOLIST LIB57B
$ LINK/NOMAP LIB57
$ LINK/NOMAP LIB57B
$ MACRO/NOLIST DEF_LIBCLI
$ ANALYZE/OBJECT/OUTPUT=DEF_LIBCLI.ANL DEF_LIBCLI.OBJ
$ !
$ ! S1: symbols at command level 1
$ G1 == "global one"
$ L1 = "local one"
$ ABBR*EV = "abbreviated"
$ NUM = -42
$ BOTH == "global both"
$ BOTH = "local both"
$ AMB*IG = "ambiguous"
$ DEL_L = "delete me"
$ DEL_G == "delete me too"
$ DEL_W == "global, deleted from the local table"
$ DEL_AB*BR = "abbreviated, deleted"
$ LIB57 SYM
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL/LOCAL S_LOCAL
$ SHOW SYMBOL/GLOBAL S_GLOBAL
$ SHOW SYMBOL/LOCAL S_ONE
$ SHOW SYMBOL S_ZERO
$ SHOW SYMBOL S_THREE
$ SHOW SYMBOL S_LOWER
$ SHOW SYMBOL S_AB
$ SHOW SYMBOL S_ABBREV
$ SHOW SYMBOL _UNDER
$ SHOW SYMBOL $DOLLAR
$ SHOW SYMBOL S_DASH
$ SHOW SYMBOL S_V255
$ SHOW SYMBOL S_V256
$ WRITE SYS$OUTPUT F$LENGTH(S_V1024)
$ SHOW SYMBOL S_V4097
$ SHOW SYMBOL AMBIG
$ SHOW SYMBOL AM
$ SHOW SYMBOL AMBX
$ SHOW SYMBOL S_EMPTY
$ SHOW SYMBOL S_CASE
$ SHOW SYMBOL/LOCAL G1
$ SHOW SYMBOL/GLOBAL G1
$ SHOW SYMBOL S_ONEARG
$ SHOW SYMBOL DEL_L
$ SHOW SYMBOL DEL_G
$ SHOW SYMBOL DEL_W
$ SHOW SYMBOL DEL_ABBR
$ SHOW SYMBOL L1
$ SHOW SYMBOL NUM
$ !
$ ! S2: symbols at command level 2
$ LOCOUT = "outer local"
$ LOCDEL = "outer, to be deleted"
$ @LIB57S SYMOUT
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL S_INNER
$ SHOW SYMBOL S_INNERG
$ SHOW SYMBOL LOCOUT
$ SHOW SYMBOL LOCDEL
$ !
$ ! S3: logical names
$ DEFINE LIB57_D "to delete"
$ DEFINE/JOB LIB57_JD "job, to delete"
$ DEFINE/USER_MODE LIB57_U "user mode"
$ LIB57 LNM
$ SHOW SYMBOL $STATUS
$ SHOW LOGICAL/PROCESS/FULL LIB57*
$ SHOW LOGICAL/PROCESS "lib57_lower"
$ SHOW LOGICAL/PROCESS "LIB57_TRAIL "
$ SHOW LOGICAL/JOB/FULL LIB57*
$ SHOW LOGICAL/GROUP/FULL LIB57*
$ DEASSIGN/JOB LIB57_B
$ DEASSIGN/JOB LIB57_F
$ DEASSIGN/GROUP LIB57_C
$ !
$ ! S4: the common area
$ LIB57 COM1
$ SHOW SYMBOL $STATUS
$ LIB57 COM2
$ RUN LIB57B
$ SPAWN/OUTPUT=LIB57SP.LOG LIB57 COM2
$ SHOW SYMBOL $STATUS
$ TYPE LIB57SP.LOG
$ !
$ ! S5: LIB$DO_COMMAND
$ CHAINED = "the chained command ran"
$ LIB57 DOSHOW
$ SHOW SYMBOL $STATUS
$ LIB57 DOAT
$ SHOW SYMBOL $STATUS
$ LIB57 DOEMPTY
$ SHOW SYMBOL $STATUS
$ LIB57 DOBLANK
$ SHOW SYMBOL $STATUS
$ LIB57 DOFAIL
$ SHOW SYMBOL $STATUS
$ LIB57 DOWRITE
$ SHOW SYMBOL $STATUS
$ LIB57 DO255
$ SHOW SYMBOL $STATUS
$ LIB57 DO256
$ SHOW SYMBOL $STATUS
$ LIB57 DOEXH
$ SHOW SYMBOL $STATUS
$ LIB57 DOAGAIN
$ SHOW SYMBOL $STATUS
$ @LIB57S DOEXIT
$ SHOW SYMBOL $STATUS
$ LIB57 DOMSG
$ SHOW SYMBOL $STATUS
$ @LIB57S DOLOCAL
$ SHOW SYMBOL CHAINED_LOCAL
$ SET NOVERIFY
$ LIB57 DOSHOW
$ SET VERIFY
$ LIB57 NOGROUP
$ SHOW SYMBOL $STATUS
$ !
$ ! S6: LIB$RUN_PROGRAM
$ LIB57 RUNB
$ SHOW SYMBOL $STATUS
$ LIB57 RUNAGAIN
$ SHOW SYMBOL $STATUS
$ LIB57 RUNEXH
$ SHOW SYMBOL $STATUS
$ LIB57 RUNSPEC
$ SHOW SYMBOL $STATUS
$ LIB57 RUNNONE
$ SHOW SYMBOL $STATUS
$ LIB57 RUNEMPTY
$ SHOW SYMBOL $STATUS
$ LIB57 RUNARGS
$ SHOW SYMBOL $STATUS
$ LIB57 RUN256
$ SHOW SYMBOL $STATUS
$ !
$ ! S7: the control keys
$ SET CONTROL=(Y,T)
$ ON CONTROL_Y THEN GOTO NEVER
$ CTL = "WRITE SYS$OUTPUT F$ENVIRONMENT(""CONTROL""), "" / "", F$ENVIRONMENT(""NOCONTROL""), "" / "", F$ENVIRONMENT(""ON_CONTROL_Y"")"
$ CTL
$ LIB57 CTLQ
$ CTL
$ LIB57 CTLDT
$ CTL
$ LIB57 CTLET
$ CTL
$ LIB57 CTLDY
$ CTL
$ ON CONTROL_Y THEN GOTO NEVER
$ LIB57 CTLEY
$ CTL
$ LIB57 CTLBOTH
$ CTL
$ LIB57 CTLBAD
$ CTL
$ SET CONTROL=(Y,T)
$ SPAWN/OUTPUT=LIB57SC.LOG @LIB57P
$ CTL
$ TYPE LIB57SC.LOG
$ SET NOCONTROL=T
$ !
$ ! S8: LIB$ATTACH's refusals
$ LIB57 ATTSELF
$ SHOW SYMBOL $STATUS
$ LIB57 ATTNONE
$ SHOW SYMBOL $STATUS
$ LIB57 ATTZERO
$ SHOW SYMBOL $STATUS
$ !
$ ! S9: no CLI
$ OPEN/WRITE IN LIB57N.DAT
$ WRITE IN "the command line"
$ CLOSE IN
$ LIB57 NOCLI
$ SHOW SYMBOL $STATUS
$ TYPE LIB57N.OUT
$ IF F$SEARCH("LIB57N.ERR") .NES. "" THEN TYPE LIB57N.ERR
$ !
$ ! The end.
$ SET NOVERIFY
$ EXIT
$ NEVER:
$ WRITE SYS$OUTPUT "ON CONTROL_Y's GOTO: not expected"
$ EXIT
`

// nested is lib57s.com: a group run at command level 2, and DCL's state
// there after it.
const nested = `$ ! LIB57S.COM - the Phase 57 probe's nested procedure: runs LIB57's
$ ! group P1 at command level 2, then shows what it changed there.
$ LIB57 'P1'
$ SHOW SYMBOL $STATUS
$ GOTO 'P1'
$ SYMOUT:
$ SHOW SYMBOL S_INNER
$ SHOW SYMBOL S_INNERG
$ SHOW SYMBOL LOCOUT
$ SHOW SYMBOL LOCDEL
$ EXIT
$ DOLOCAL:
$ SHOW SYMBOL CHAINED_LOCAL
$ EXIT
$ DOEXIT:
$ WRITE SYS$OUTPUT "LIB57S goes on after the chained EXIT"
$ EXIT
`

// atProcedure is lib57c.com, the procedure LIB$DO_COMMAND runs.
const atProcedure = `$ ! LIB57C.COM - the Phase 57 probe's procedure that LIB$DO_COMMAND runs.
$ WRITE SYS$OUTPUT "LIB57C: P1=""", P1, """ P2=""", P2, """ depth ", F$ENVIRONMENT("DEPTH")
$ EXIT
`

// subprocess is lib57p.com, the spawned subprocess's procedure: its own
// SET CONTROL settings.
const subprocess = `$ ! LIB57P.COM - the Phase 57 probe's spawned procedure: the control
$ ! keys in a subprocess.
$ CTL
$ SET NOCONTROL=Y
$ CTL
$ LIB57 CTLEY
$ CTL
$ LIB57 CTLDT
$ CTL
$ EXIT
`

// interactive is lib57i.com, the interactive part's procedure
// (README.md's table).
const interactive = `$ ! LIB57I.COM - the Phase 57 probe's interactive cases, one per P1
$ ! (testdata/lib57/README.md). Run @LIB57 first: it defines LIB57.
$ SET NOON
$ SET VERIFY
$ GOTO 'P1'
$ PAUSE:
$ X = "in LIB57I"
$ LIB57 PAUSE
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "LIB57I goes on"
$ EXIT
$ GETCMD:
$ LIB57 GETCMD
a data line
$ SHOW SYMBOL $STATUS
$ EXIT
$ CTLY:
$ ON CONTROL_Y THEN GOTO CAUGHT
$ LIB57 CTLDY
$ WRITE SYS$OUTPUT "Press Ctrl/Y now (10 seconds)"
$ WAIT 00:00:10
$ LIB57 CTLEY
$ WRITE SYS$OUTPUT "Press Ctrl/Y now (10 seconds)"
$ WAIT 00:00:10
$ WRITE SYS$OUTPUT "no Ctrl/Y"
$ EXIT
$ CAUGHT:
$ WRITE SYS$OUTPUT "ON CONTROL_Y caught it"
$ LIB57 CTLEY
$ EXIT
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 57 probe's exchange volume with
! govax. Written by testdata/lib57/gen.go. Run from the repository root:
!
!     govax console < testdata/lib57/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib57-exchange.dsk" /DEVICE=RD51 LIB57
MOUNT/WRITE DUA1 "testdata/disks/lib57-exchange.dsk"
COPY "testdata/lib57/lib57.mar"/HOST DUA1:[000000]LIB57.MAR
COPY "testdata/lib57/lib57b.mar"/HOST DUA1:[000000]LIB57B.MAR
COPY "testdata/lib57/def_libcli.mar"/HOST DUA1:[000000]DEF_LIBCLI.MAR
COPY "testdata/lib57/lib57.com"/HOST DUA1:[000000]LIB57.COM
COPY "testdata/lib57/lib57s.com"/HOST DUA1:[000000]LIB57S.COM
COPY "testdata/lib57/lib57c.com"/HOST DUA1:[000000]LIB57C.COM
COPY "testdata/lib57/lib57p.com"/HOST DUA1:[000000]LIB57P.COM
COPY "testdata/lib57/lib57i.com"/HOST DUA1:[000000]LIB57I.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib57")

	for name, text := range map[string]string{
		"lib57.mar":      program(),
		"lib57b.mar":     chained,
		"def_libcli.mar": defLibcli,
		"lib57.com":      command,
		"lib57s.com":     nested,
		"lib57c.com":     atProcedure,
		"lib57p.com":     subprocess,
		"lib57i.com":     interactive,
		"exchange.cmd":   exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
