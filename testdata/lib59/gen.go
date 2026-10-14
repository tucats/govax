//go:build ignore

// gen.go writes the Phase 59 probe of LIBRTL's memory routines
// (docs/PHASE-59, subtask 10): lib59.mar, a MACRO program that calls
// each routine of the phase, case by case, and prints what each
// returned and stored and what it passed the program's own routines
// (TestLib59Oracle compares govax's run with VMS's, vax/lib59-vax.txt);
// lib59.com, which builds and runs it on VMS; and exchange.cmd, the
// govax console script that makes the exchange volume. Run it from the
// repository root:
//
//	go run testdata/lib59/gen.go
//
// The program works as Phase 58's (testdata/lib58/gen.go) does: each
// case runs in a routine of its own, under a condition handler (HANDLER)
// that records the condition it's given and unwinds, so the output tells
// a status a routine returned from a condition it signaled. Its line is
// "name R0=xxxxxxxx", then a line per value it shows; the lines the
// case's routines make (the program's own routines, the zone reports)
// are kept in a log while it runs and printed under its line.
//
// What's new here:
//
//   - Addresses differ from system to system, so the probe shows them as
//     differences ("rel": a block's address less another's), page
//     offsets ("pgo"), or whether a cell was written ("set"); a block's
//     contents as a fill map ("fill": F where a longword holds the fill,
//     "." where it doesn't).
//   - ACTION, the action routine the probe gives LIB$SHOW_VM and
//     LIB$SHOW_VM_ZONE, logs each line it's given as "  | text", which
//     the oracle compares with every 8-digit hex number masked; the
//     lines a routine writes to SYS$OUTPUT itself are between "{{" and
//     "}}", masked the same way.
//   - The user zone routines (UALLOC, UFREE, URESET, UDELETE) log their
//     arguments by what they are: the case's own cells (GVN, GVB), the
//     user zone's control block (an offset from its ID), the stack, or
//     elsewhere, and the user argument's value; then they do as the
//     manual's Example 5-1 does, on the zone the user argument names.
//   - The page routines (GPAGE, FPAGE) log their calls and pass them to
//     LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE.
//   - STATSNAP and STATDIFF bracket a case to show what it changed in
//     LIB$STAT_VM's six figures.
//   - Cases that may damage VMS's zones (double frees, wrong sizes, a
//     changed tag, a deleted zone's ID) come last, each in a zone of its
//     own.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// show is one value a case prints after its R0 line.
type show struct {
	kind  string // mem, vmem, rel, pgo, fill, set, act, at
	label string // the cell ("%" is the case's prefix)
	base  string // rel: the base cell
	count int    // mem, fill, at: longwords
	ones  bool   // fill: the fill is FF, not 00
	off   int    // at: the offset from the cell's address
}

func mem(label string, n int) show  { return show{kind: "mem", label: label, count: n} }
func vmem(label string, n int) show { return show{kind: "vmem", label: label, count: n} }
func rel(label, base string) show   { return show{kind: "rel", label: label, base: base} }
func pgo(label string) show         { return show{kind: "pgo", label: label} }
func set(label string) show         { return show{kind: "set", label: label} }
func act() show                     { return show{kind: "act"} }
func stats() show                   { return show{kind: "mem", label: "SD", count: 6} }

// fill is a fill map of n longwords at the address in label.
func fill(label string, n int, ones bool) show {
	return show{kind: "fill", label: label, count: n, ones: ones}
}

// at is n longwords at off from the address in label, a "~" line.
func at(label string, off, n int) show {
	return show{kind: "at", label: label, off: off, count: n}
}

// probeCase is one call of one routine.
type probeCase struct {
	name  string   // printed first
	data  []string // data lines; "%" is the case's label prefix
	pre   []string // instructions before the call
	call  string   // the routine; "" for none (pre's R0 is the case's)
	args  []string // its arguments, first first
	post  []string // instructions after the call, R0 at (SP)
	show  []show
	raw   bool // the routine writes to SYS$OUTPUT itself: between {{ and }}
	stats bool // show what the case changed in LIB$STAT_VM's figures
}

// with adds shows to a case.
func (c probeCase) with(s ...show) probeCase {
	c.show = append(append([]show(nil), c.show...), s...)

	return c
}

// before adds instructions before a case's own.
func (c probeCase) before(lines ...string) probeCase {
	c.pre = append(append([]string(nil), lines...), c.pre...)

	return c
}

// counted makes a case show what it changed in the statistics.
func (c probeCase) counted() probeCase {
	c.stats = true

	return c.with(stats())
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

// trim drops the trailing omitted ("0") arguments from args.
func trim(args []string) []string {
	for len(args) > 0 && args[len(args)-1] == "0" {
		args = args[:len(args)-1]
	}

	return args
}

// str is a class S text descriptor for s at label.
func str(label, s string) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T\n%s_T:\t.ASCII\t%s", label, len(s), label, label, delimit(s))
}

// delimit is s between delimiters it doesn't contain, as .ASCII takes it.
func delimit(s string) string {
	for _, d := range []string{`"`, "/", "|", "!"} {
		if !strings.Contains(s, d) {
			return d + s + d
		}
	}

	log.Fatalf("no delimiter for %q", s)

	return ""
}

// dyn is an empty dynamic string descriptor, class D, type T.
func dyn(label string) string {
	return fmt.Sprintf("%s:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0", label)
}

// long is a longword cell holding v.
func long(label, v string) string {
	return fmt.Sprintf("%s:\t.LONG\t%s", label, v)
}

// The values the probe's routines are given.
const (
	userArg = "#^X12345678" // a user-arg or user-specified-argument
	failSt  = "^X1230"      // what a routine returns for failure (FAILST)
	untouch = "#^X55555555" // a cell's value before a routine writes it
)

// The data every case may use; each Z_, B_, P_ cell the cases name is
// added, preset to ^X55555555.
var shared = []string{
	"ZERO:\t.LONG\t0",
	"ONE:\t.LONG\t1",
	"TWO:\t.LONG\t2",
	"THREE:\t.LONG\t3",
	"UARGV:\t.LONG\t^X12345678\t; a user zone's user argument",
	"JUNKZ:\t.ADDRESS\tJUNK\t; a zone ID that names no zone",
	"JUNK:\t.BLKB\t256",
}

// ---------------------------------------------------------------------
// The case makers.

// gp is LIB$GET_VM_PAGE of n pages into cell.
func gp(name, n, cell string) probeCase {
	return probeCase{name: name, data: []string{long("%N", n)}, pre: []string{"MOVL\t" + untouch + "," + cell},
		call: "LIB$GET_VM_PAGE", args: []string{"%N", cell}}
}

// fp is LIB$FREE_VM_PAGE of n pages at the address in cell, plus off
// bytes ("" for none).
func fp(name, n, cell, off string) probeCase {
	c := probeCase{name: name, data: []string{long("%N", n)}, call: "LIB$FREE_VM_PAGE", args: []string{"%N", cell}}

	if off != "" {
		c.data = append(c.data, long("%A", "0"))
		c.pre = []string{fmt.Sprintf("ADDL3\t#%s,%s,%%A", off, cell)}
		c.args[1] = "%A"
	}

	return c
}

// za is LIB$CREATE_VM_ZONE's attributes: each a longword's value, ""
// omitted.
type za struct {
	alg, arg, flags, extend, initial, block, align, limit, smallest string
	name                                                             string   // zone-name's text, "" none
	nameData                                                         string   // zone-name's data at %M instead
	tail                                                             []string // arguments after zone-name, as given
	nargs                                                            int      // the argument count, 0 for the last given
}

// cz is LIB$CREATE_VM_ZONE into cell. When it succeeds the zone's
// report at detail level 1 goes to the log; unless keep, the zone is
// then deleted.
func cz(name, cell string, a za, keep bool) probeCase {
	c := probeCase{name: name, pre: []string{"MOVL\t" + untouch + "," + cell}, call: "LIB$CREATE_VM_ZONE"}
	args := []string{cell}

	for i, v := range []string{a.alg, a.arg, a.flags, a.extend, a.initial, a.block, a.align, a.limit, a.smallest} {
		if v == "" {
			args = append(args, "0")

			continue
		}

		c.data = append(c.data, long(fmt.Sprintf("%%V%d", i), v))
		args = append(args, fmt.Sprintf("%%V%d", i))
	}

	switch {
	case a.nameData != "":
		c.data = append(c.data, a.nameData)
		args = append(args, "%M")
	case a.name != "":
		c.data = append(c.data, str("%M", a.name))
		args = append(args, "%M")
	default:
		args = append(args, "0")
	}

	args = append(args, a.tail...)

	c.args = trim(args)
	if a.nargs > 0 {
		for len(args) < a.nargs {
			args = append(args, "0")
		}

		c.args = args[:a.nargs]
	}

	c.post = append([]string{"BLBC\t(SP),%Y"}, showZone(cell, "ONE")...)
	if !keep {
		c.post = append(c.post, "PUSHAL\t"+cell, "CALLS\t#1,G^LIB$DELETE_VM_ZONE")
	}

	c.post = append(c.post, "%Y:")
	c.show = []show{set(cell)}

	return c
}

// showZone is the instructions that log a zone's report at a level
// (a longword's label) through ACTION.
func showZone(cell, level string) []string {
	return []string{"PUSHL\t#0", "PUSHAB\tACTION", "PUSHAL\t" + level, "PUSHAL\t" + cell, "CALLS\t#4,G^LIB$SHOW_VM_ZONE"}
}

// cuz is LIB$CREATE_USER_VM_ZONE into cell, with the user argument (a
// cell's label, or "0") and the four routines (labels, or "0").
func cuz(name, cell, uarg string, routines [4]string, zname string, nargs int) probeCase {
	c := probeCase{name: name, pre: []string{"MOVL\t" + untouch + "," + cell}, call: "LIB$CREATE_USER_VM_ZONE"}
	args := []string{cell, uarg, routines[0], routines[1], routines[2], routines[3], "0"}

	if zname != "" {
		c.data = append(c.data, str("%M", zname))
		args[6] = "%M"
	}

	c.args = trim(args)
	if nargs > 0 {
		for len(args) < nargs {
			args = append(args, "0")
		}

		c.args = args[:nargs]
	}

	c.show = []show{set(cell)}

	return c
}

// monitor is the four routines of Example 5-1's monitoring zone.
var monitor = [4]string{"UALLOC", "UFREE", "URESET", "UDELETE"}

// gv is LIB$GET_VM of n bytes into cell, from zone (a cell's label, ""
// omitted).
func gv(name, n, cell, zone string) probeCase {
	c := probeCase{name: name, data: []string{long("%N", n)}, pre: []string{"MOVL\t" + untouch + "," + cell},
		call: "LIB$GET_VM", args: []string{"%N", cell}}

	if zone != "" {
		c.args = append(c.args, zone)
	}

	return c
}

// fv is LIB$FREE_VM of n bytes ("" omitted) at the address in cell, in
// zone ("" omitted).
func fv(name, n, cell, zone string) probeCase {
	c := probeCase{name: name, call: "LIB$FREE_VM", args: []string{"0", cell}}

	if n != "" {
		c.data = []string{long("%N", n)}
		c.args[0] = "%N"
	}

	if zone != "" {
		c.args = append(c.args, zone)
	}

	return c
}

// fvAt is LIB$FREE_VM of n bytes at off bytes past the address in cell.
func fvAt(name, n, cell, off, zone string) probeCase {
	c := fv(name, n, cell, zone)
	c.data = append(c.data, long("%A", "0"))
	c.pre = []string{fmt.Sprintf("ADDL3\t#%s,%s,%%A", off, cell)}
	c.args[1] = "%A"

	return c
}

// seq is LIB$GET_VM of each size in zone, into cells prefix1, prefix2,
// ...: the first shows its page offset, the rest their distance from it.
func seq(name, prefix, zone string, sizes ...string) []probeCase {
	var out []probeCase

	for i, n := range sizes {
		cell := fmt.Sprintf("%s%d", prefix, i+1)
		c := gv(fmt.Sprintf("%s %d: %s", name, i+1, n), n, cell, zone)

		if i == 0 {
			c = c.with(pgo(cell))
		} else {
			c = c.with(rel(cell, prefix+"1"))
		}

		out = append(out, c)
	}

	return out
}

// zoneOp is LIB$RESET_VM_ZONE or LIB$DELETE_VM_ZONE (op) of zone.
func zoneOp(name, op, zone string) probeCase {
	return probeCase{name: name, call: op, args: []string{zone}}
}

// svz is LIB$SHOW_VM_ZONE of zone at level ("" omitted) to ACTION.
func svz(name, zone, level string) probeCase {
	c := probeCase{name: name, call: "LIB$SHOW_VM_ZONE", args: []string{zone, "0", "ACTION", userArg}}

	if level != "" {
		c.data = []string{long("%L", level)}
		c.args[1] = "%L"
	}

	return c.with(act())
}

// verify is LIB$VERIFY_VM_ZONE of zone.
func verify(name, zone string) probeCase {
	return probeCase{name: name, call: "LIB$VERIFY_VM_ZONE", args: []string{zone}}
}

// findAll lists every zone (FINDALL): each one's report at level 0, then
// the status that ended the scan.
func findAll(name string) probeCase {
	return probeCase{name: name, pre: []string{"CALLS\t#0,FINDALL"}}
}

// user sets the zones the user zone routines report against: the user
// zone and the real zone its routines use ("" for none: then the user
// argument's value is shown).
func user(c probeCase, uz, rz string) probeCase {
	lines := []string{"MOVL\t" + uz + ",UZCUR", "MOVL\t" + untouch + ",RZCUR"}
	if rz != "" {
		lines[1] = "MOVL\t" + rz + ",RZCUR"
	}

	return c.before(lines...)
}

// ugv is LIB$GET_VM of n bytes from a user zone, through GVN and GVB.
func ugv(name, n, uz, rz string) probeCase {
	c := probeCase{name: name, pre: []string{"MOVL\t#" + n + ",GVN", "MOVL\t" + untouch + ",GVB"},
		call: "LIB$GET_VM", args: []string{"GVN", "GVB", uz}, show: []show{set("GVB")}}

	return user(c, uz, rz)
}

// ufv is LIB$FREE_VM of n bytes ("" omitted) at GVB in a user zone.
func ufv(name, n, uz, rz string) probeCase {
	c := probeCase{name: name, call: "LIB$FREE_VM", args: []string{"0", "GVB", uz}}

	if n != "" {
		c.pre = []string{"MOVL\t#" + n + ",GVN"}
		c.args[0] = "GVN"
	}

	return user(c, uz, rz)
}

// statVM is LIB$STAT_VM of code into a cell preset to ^X55555555.
func statVM(name, code string) probeCase {
	return probeCase{name: name, data: []string{long("%C", code), long("%V", "^X55555555")},
		call: "LIB$STAT_VM", args: []string{"%C", "%V"}, show: []show{mem("%V", 1)}}
}

// showVM is LIB$SHOW_VM of code ("" omitted) to ACTION.
func showVM(name, code string) probeCase {
	c := probeCase{name: name, call: "LIB$SHOW_VM", args: []string{"0", "ACTION", userArg}}

	if code != "" {
		c.data = []string{long("%C", code)}
		c.args[0] = "%C"
	}

	return c.with(act())
}

// cmalloc is one of the C library's routines, its arguments by value
// and its result in R0: an address result goes to cell ("" for none) and
// R0 shows only whether it's 0.
func cmalloc(name, routine, cell string, args ...string) probeCase {
	c := probeCase{name: name, call: routine, args: args}

	if cell != "" {
		c.pre = []string{"MOVL\t" + untouch + "," + cell}
		c.post = []string{"MOVL\t(SP)," + cell, "BEQL\t%Y", "MOVL\t#1,(SP)", "%Y:"}
	}

	return c
}

// expand flattens the cases' groups.
func expand(groups ...any) []probeCase {
	var out []probeCase

	for _, g := range groups {
		switch g := g.(type) {
		case probeCase:
			out = append(out, g)
		case []probeCase:
			out = append(out, g...)
		default:
			log.Fatalf("bad group %T", g)
		}
	}

	return out
}

// ---------------------------------------------------------------------
// The cases, in the order they run.

func allCases() []probeCase {
	return expand(
		// What's there when the program starts.
		probeCase{name: "STAT at start", pre: []string{"CALLS\t#0,STATREAD"}, show: []show{vmem("SV", 6)}},
		findAll("FIND at start"),
		probeCase{name: "FIND the first two", call: "LIB$FIND_VM_ZONE", args: []string{"%C", "Z_DEFAULT"},
			data: []string{long("%C", "0"), long("%S", "0")},
			post: []string{"PUSHAL\tZ_STRING", "PUSHAL\t%C", "CALLS\t#2,G^LIB$FIND_VM_ZONE", "MOVL\tR0,%S"},
			show: []show{set("Z_DEFAULT"), set("Z_STRING")}}.with(mem("%S", 1)),
		svz("SVZ zone 0 level 1", "ZERO", "1"),
		svz("SVZ default by ID level 1", "Z_DEFAULT", "1"),
		svz("SVZ string zone level 1", "Z_STRING", "1"),

		// Pages (GET_VM_PAGE, FREE_VM_PAGE).
		gp("GP 1", "1", "P_1").with(pgo("P_1")).counted(),
		gp("GP 1 second", "1", "P_2").with(rel("P_2", "P_1")),
		gp("GP 1 third", "1", "P_3").with(rel("P_3", "P_1")),
		gp("GP 4", "4", "P_4").with(rel("P_4", "P_1")),
		fp("FP the second", "1", "P_2", "").counted(),
		gp("GP 1 after a free", "1", "P_5").with(rel("P_5", "P_1")),
		fp("FP upper half of 4", "2", "P_4", "1024"),
		fp("FP lower half of 4", "2", "P_4", ""),
		gp("GP 3 after the halves", "3", "P_6").with(rel("P_6", "P_1")),
		fp("FP 3 across groups", "3", "P_1", "").counted(),
		fp("FP freed already", "1", "P_1", "").counted(),
		fp("FP unaligned", "1", "P_6", "4"),
		fp("FP inside a page", "1", "P_6", "256"),
		fp("FP count 0", "0", "P_6", ""),
		fp("FP count -1", "-1", "P_6", ""),
		probeCase{name: "FP never allocated", data: []string{long("%N", "1"), long("%A", "^X3FFF0000")},
			call: "LIB$FREE_VM_PAGE", args: []string{"%N", "%A"}},
		gp("GP 0", "0", "P_7").with(set("P_7")).counted(),
		gp("GP -1", "-1", "P_7").with(set("P_7")),
		gp("GP more than P0", "2097152", "P_7").with(set("P_7")).counted(),
		gp("GP before $EXPREG", "1", "P_7").with(rel("P_7", "P_1")),
		probeCase{name: "$EXPREG 1 page", data: []string{"%R:\t.LONG\t0,0"},
			pre:  []string{"$EXPREG_S\tPAGCNT=#1,RETADR=%R"},
			show: []show{rel("%R", "P_7")}},
		gp("GP after $EXPREG", "1", "P_8").with(rel("P_8", "P_7")),
		fp("FP the 3", "3", "P_6", ""),
		fp("FP the one before $EXPREG", "1", "P_7", ""),
		fp("FP the one after", "1", "P_8", ""),
		probeCase{name: "FP 1 argument", data: []string{long("%N", "1")}, call: "LIB$FREE_VM_PAGE", args: []string{"%N"}},
		probeCase{name: "GP 3 arguments", data: []string{long("%N", "1")}, call: "LIB$GET_VM_PAGE",
			args: []string{"%N", "P_9", "0"}}.with(set("P_9")),
		fp("FP the 3-argument page", "1", "P_9", ""),

		// CREATE_VM_ZONE's attributes, each zone shown at level 1.
		cz("CZ defaults", "Z_DEF", za{}, true),
		svz("SVZ defaults level 0", "Z_DEF", "0"),
		svz("SVZ defaults level 2", "Z_DEF", "2"),
		svz("SVZ defaults level 3", "Z_DEF", "3"),
		zoneOp("DZ defaults", "LIB$DELETE_VM_ZONE", "Z_DEF"),
		cz("CZ algorithm 0", "Z_T", za{alg: "0"}, false),
		cz("CZ algorithm 1", "Z_T", za{alg: "1"}, false),
		cz("CZ algorithm 5", "Z_T", za{alg: "5"}, false),
		cz("CZ first fit, argument 5", "Z_T", za{alg: "1", arg: "5"}, false),
		cz("CZ quick fit, no argument", "Z_T", za{alg: "2"}, false),
		cz("CZ quick fit 0", "Z_T", za{alg: "2", arg: "0"}, false),
		cz("CZ quick fit 1", "Z_T", za{alg: "2", arg: "1"}, false),
		cz("CZ quick fit 128", "Z_T", za{alg: "2", arg: "128"}, false),
		cz("CZ quick fit 129", "Z_T", za{alg: "2", arg: "129"}, false),
		cz("CZ frequent sizes, no argument", "Z_T", za{alg: "3"}, false),
		cz("CZ frequent sizes 0", "Z_T", za{alg: "3", arg: "0"}, false),
		cz("CZ frequent sizes 16", "Z_T", za{alg: "3", arg: "16"}, false),
		cz("CZ frequent sizes 17", "Z_T", za{alg: "3", arg: "17"}, false),
		cz("CZ fixed, no argument", "Z_T", za{alg: "4"}, false),
		cz("CZ fixed 0", "Z_T", za{alg: "4", arg: "0"}, false),
		cz("CZ fixed 1", "Z_T", za{alg: "4", arg: "1"}, false),
		cz("CZ fixed -1", "Z_T", za{alg: "4", arg: "-1"}, false),
		cz("CZ fixed 10000", "Z_T", za{alg: "4", arg: "10000"}, false),
		cz("CZ flags 1", "Z_T", za{flags: "1"}, false),
		cz("CZ flags 6, both GET fills", "Z_T", za{flags: "6"}, false),
		cz("CZ flags 24, both FREE fills", "Z_T", za{flags: "24"}, false),
		cz("CZ flags 63", "Z_T", za{flags: "63"}, false),
		cz("CZ flags 32", "Z_T", za{flags: "32"}, false),
		cz("CZ NO_EXTEND, no initial", "Z_T", za{flags: "64"}, false),
		cz("CZ NO_EXTEND, initial 4", "Z_T", za{flags: "64", initial: "4"}, false),
		cz("CZ TAIL_LARGE", "Z_T", za{flags: "128"}, false),
		cz("CZ flags 256", "Z_T", za{flags: "256"}, false),
		cz("CZ flags 512", "Z_T", za{flags: "512"}, false),
		cz("CZ flags 1<<31", "Z_T", za{flags: "^X80000000"}, false),
		cz("CZ extend 0", "Z_T", za{extend: "0"}, false),
		cz("CZ extend 1", "Z_T", za{extend: "1"}, false),
		cz("CZ extend 1024", "Z_T", za{extend: "1024"}, false),
		cz("CZ extend 1025", "Z_T", za{extend: "1025"}, false),
		cz("CZ extend -1", "Z_T", za{extend: "-1"}, false),
		cz("CZ initial -1", "Z_T", za{initial: "-1"}, false),
		cz("CZ initial 1", "Z_T", za{initial: "1"}, false),
		cz("CZ initial 20", "Z_T", za{initial: "20"}, false),
		cz("CZ block 0", "Z_T", za{block: "0"}, false),
		cz("CZ block 4", "Z_T", za{block: "4"}, false),
		cz("CZ block 12", "Z_T", za{block: "12"}, false),
		cz("CZ block 16", "Z_T", za{block: "16"}, false),
		cz("CZ block 512", "Z_T", za{block: "512"}, false),
		cz("CZ block 1024", "Z_T", za{block: "1024"}, false),
		cz("CZ align 0", "Z_T", za{align: "0"}, false),
		cz("CZ align 2", "Z_T", za{align: "2"}, false),
		cz("CZ align 4", "Z_T", za{align: "4"}, false),
		cz("CZ align 12", "Z_T", za{align: "12"}, false),
		cz("CZ align 512", "Z_T", za{align: "512"}, false),
		cz("CZ align 1024", "Z_T", za{align: "1024"}, false),
		cz("CZ block 8 align 64", "Z_T", za{block: "8", align: "64"}, false),
		cz("CZ tags, block 8 align 4", "Z_T", za{flags: "1", block: "8", align: "4"}, false),
		cz("CZ limit 10, no initial", "Z_T", za{limit: "10"}, false),
		cz("CZ limit 10, initial 4", "Z_T", za{limit: "10", initial: "4"}, false),
		cz("CZ limit 10, initial 20", "Z_T", za{limit: "10", initial: "20"}, false),
		cz("CZ limit 10, initial 10", "Z_T", za{limit: "10", initial: "10"}, false),
		cz("CZ limit 32767, initial 1", "Z_T", za{limit: "32767", initial: "1"}, false),
		cz("CZ limit 32768, initial 1", "Z_T", za{limit: "32768", initial: "1"}, false),
		cz("CZ limit -1, initial 1", "Z_T", za{limit: "-1", initial: "1"}, false),
		cz("CZ quick fit 4, smallest 4", "Z_T", za{alg: "2", arg: "4", smallest: "4"}, false),
		cz("CZ quick fit 4, smallest 12", "Z_T", za{alg: "2", arg: "4", smallest: "12"}, false),
		cz("CZ quick fit 4, smallest 16", "Z_T", za{alg: "2", arg: "4", smallest: "16"}, false),
		cz("CZ quick fit 4, smallest 0", "Z_T", za{alg: "2", arg: "4", smallest: "0"}, false),
		cz("CZ quick fit 4, block 16 smallest 8", "Z_T", za{alg: "2", arg: "4", block: "16", smallest: "8"}, false),
		cz("CZ first fit, smallest 16", "Z_T", za{smallest: "16"}, false),
		cz("CZ name", "Z_T", za{name: "PROBE59 ZONE"}, false),
		cz("CZ empty name", "Z_T", za{nameData: "%M:\t.WORD\t0\n\t.BYTE\t14,1\n\t.LONG\t0"}, false),
		cz("CZ name class D", "Z_T", za{nameData: "%M:\t.WORD\t5\n\t.BYTE\t14,2\n\t.ADDRESS\t%M_T\n%M_T:\t.ASCII\t/DYNAM/"}, false),
		cz("CZ name class 99", "Z_T", za{nameData: "%M:\t.WORD\t5\n\t.BYTE\t14,99\n\t.ADDRESS\t%M_T\n%M_T:\t.ASCII\t/BADCL/"}, false),
		cz("CZ 40-character name", "Z_T", za{name: strings.Repeat("N", 40)}, false),
		cz("CZ 12 arguments", "Z_T", za{name: "TWELVE", nargs: 12}, false),
		cz("CZ 14 arguments", "Z_T", za{name: "FOURTEEN", nargs: 14}, false),
		cz("CZ 15 arguments", "Z_T", za{name: "FIFTEEN", nargs: 15}, false),
		cz("CZ number-of-areas 3", "Z_T", za{name: "AREAS", tail: []string{"THREE"}}, false),
		probeCase{name: "CZ no arguments", call: "LIB$CREATE_VM_ZONE"},
		cz("CZ zone-id only", "Z_T", za{nargs: 1}, false),

		// GET_VM and FREE_VM: first fit.
		cz("CZ first fit", "Z_FF", za{name: "FIRST"}, true),
		seq("GV FF", "B_FF", "Z_FF", "1", "8", "9", "16", "24", "100"),
		fv("FV FF 9", "9", "B_FF3", "Z_FF"),
		gv("GV FF 9 again", "9", "B_FF7", "Z_FF").with(rel("B_FF7", "B_FF1")),
		fv("FV FF 8", "8", "B_FF2", "Z_FF"),
		fv("FV FF 16", "16", "B_FF4", "Z_FF"),
		gv("GV FF 40 into the hole", "40", "B_FF8", "Z_FF").with(rel("B_FF8", "B_FF1")),
		fv("FV FF size omitted", "", "B_FF5", "Z_FF"),
		gv("GV FF 24 after", "24", "B_FF9", "Z_FF").with(rel("B_FF9", "B_FF1")),
		gv("GV FF 0", "0", "B_T", "Z_FF").with(set("B_T")),
		gv("GV FF -1", "-1", "B_T", "Z_FF").with(set("B_T")),
		gv("GV FF 8000", "8000", "B_FF10", "Z_FF").with(rel("B_FF10", "B_FF1"), pgo("B_FF10")),
		gv("GV FF 20000", "20000", "B_FF11", "Z_FF").with(pgo("B_FF11")),
		gv("GV FF 16 after the areas", "16", "B_FF12", "Z_FF").with(rel("B_FF12", "B_FF1")),
		svz("SVZ FF level 2", "Z_FF", "2"),
		svz("SVZ FF level 3", "Z_FF", "3"),
		verify("VZ FF", "Z_FF"),

		// The default zone, omitted and by 0.
		gv("GV default omitted", "16", "B_D1", "").with(pgo("B_D1")).counted(),
		gv("GV default zone 0", "16", "B_D2", "ZERO").with(rel("B_D2", "B_D1")),
		gv("GV default by ID", "16", "B_D3", "Z_DEFAULT").with(rel("B_D3", "B_D1")),
		fv("FV default block in FF", "16", "B_D2", "Z_FF"),
		fv("FV FF block in default", "16", "B_FF12", "ZERO"),
		fv("FV default zone 0", "16", "B_D2", "ZERO").counted(),
		fv("FV default omitted", "16", "B_D1", ""),
		fv("FV default by ID", "16", "B_D3", "Z_DEFAULT"),
		gv("GV string zone", "16", "B_S1", "Z_STRING").with(set("B_S1")).counted(),
		fv("FV string zone", "16", "B_S1", "Z_STRING").counted(),

		// Quick fit: lists for 16, 24, 32, 40.
		cz("CZ quick fit", "Z_QF", za{alg: "2", arg: "4", smallest: "16", name: "QUICK"}, true),
		seq("GV QF", "B_QF", "Z_QF", "16", "24", "8", "48", "40"),
		fv("FV QF 16", "16", "B_QF1", "Z_QF"),
		gv("GV QF 16 again", "16", "B_QF6", "Z_QF").with(rel("B_QF6", "B_QF1")),
		fv("FV QF 24", "24", "B_QF2", "Z_QF"),
		fv("FV QF 8", "8", "B_QF3", "Z_QF"),
		fv("FV QF 40", "40", "B_QF5", "Z_QF"),
		svz("SVZ QF level 2", "Z_QF", "2"),
		gv("GV QF 8 again", "8", "B_QF7", "Z_QF").with(rel("B_QF7", "B_QF1")),
		gv("GV QF 20", "20", "B_QF8", "Z_QF").with(rel("B_QF8", "B_QF1")),
		gv("GV QF 40 again", "40", "B_QF9", "Z_QF").with(rel("B_QF9", "B_QF1")),
		svz("SVZ QF level 3", "Z_QF", "3"),
		verify("VZ QF", "Z_QF"),

		// Frequent sizes: two lists.
		cz("CZ frequent sizes", "Z_FS", za{alg: "3", arg: "2", name: "FREQUENT"}, true),
		seq("GV FS", "B_FS", "Z_FS", "16", "32", "48", "16"),
		fv("FV FS 16", "16", "B_FS1", "Z_FS"),
		fv("FV FS 32", "32", "B_FS2", "Z_FS"),
		fv("FV FS 48", "48", "B_FS3", "Z_FS"),
		svz("SVZ FS level 2", "Z_FS", "2"),
		gv("GV FS 48 again", "48", "B_FS5", "Z_FS").with(rel("B_FS5", "B_FS1")),
		gv("GV FS 32 again", "32", "B_FS6", "Z_FS").with(rel("B_FS6", "B_FS1")),
		gv("GV FS 16 again", "16", "B_FS7", "Z_FS").with(rel("B_FS7", "B_FS1")),
		svz("SVZ FS level 3", "Z_FS", "3"),

		// Fixed size, 24 bytes.
		cz("CZ fixed 24", "Z_FX", za{alg: "4", arg: "24", name: "FIXED"}, true),
		seq("GV FX", "B_FX", "Z_FX", "24", "16", "1"),
		gv("GV FX 32", "32", "B_T", "Z_FX").with(set("B_T")),
		gv("GV FX 0", "0", "B_T", "Z_FX").with(set("B_T")),
		fv("FV FX 24", "24", "B_FX1", "Z_FX"),
		gv("GV FX 24 again", "24", "B_FX4", "Z_FX").with(rel("B_FX4", "B_FX1")),
		fv("FV FX 16", "16", "B_FX2", "Z_FX"),
		fv("FV FX size omitted", "", "B_FX3", "Z_FX"),
		gv("GV FX 8 after", "8", "B_FX5", "Z_FX").with(rel("B_FX5", "B_FX1")),
		svz("SVZ FX level 2", "Z_FX", "2"),
		svz("SVZ FX level 3", "Z_FX", "3"),

		// Block size and alignment.
		cz("CZ block 32", "Z_BS", za{block: "32", name: "BLOCK32"}, true),
		seq("GV BS", "B_BS", "Z_BS", "1", "33", "1"),
		cz("CZ align 64", "Z_AL", za{align: "64", name: "ALIGN64"}, true),
		seq("GV AL", "B_AL", "Z_AL", "8", "8", "72"),
		cz("CZ block 16 align 4", "Z_BA", za{block: "16", align: "4", name: "B16A4"}, true),
		seq("GV BA", "B_BA", "Z_BA", "1", "17", "1"),

		// Boundary tags.
		cz("CZ tags", "Z_TG", za{flags: "1", block: "8", align: "4", name: "TAGS"}, true),
		seq("GV TG", "B_TG", "Z_TG", "1", "8", "9", "16"),
		probeCase{name: "the tags of a block of 8"}.with(at("B_TG2", -8, 4), at("B_TG2", 8, 2)),
		fv("FV TG size omitted", "", "B_TG2", "Z_TG"),
		fv("FV TG 9", "9", "B_TG3", "Z_TG"),
		gv("GV TG 8 again", "8", "B_TG5", "Z_TG").with(rel("B_TG5", "B_TG1")),
		gv("GV TG 24", "24", "B_TG6", "Z_TG").with(rel("B_TG6", "B_TG1")),
		svz("SVZ TG level 3", "Z_TG", "3"),
		cz("CZ tags, defaults", "Z_TD", za{flags: "1", name: "TAGS8"}, true),
		seq("GV TD", "B_TD", "Z_TD", "1", "8", "9"),

		// Fills.
		cz("CZ GET_FILL1 FREE_FILL0", "Z_F1", za{flags: "12", name: "FILL10"}, true),
		gv("GV F1 32", "32", "B_F11", "Z_F1").with(fill("B_F11", 8, true)),
		gv("GV F1 32 second", "32", "B_F12", "Z_F1").with(rel("B_F12", "B_F11"), fill("B_F12", 8, true)),
		gv("GV F1 32 third", "32", "B_F13", "Z_F1"),
		fv("FV F1 first", "32", "B_F11", "Z_F1").with(fill("B_F11", 8, false)),
		fv("FV F1 second", "32", "B_F12", "Z_F1").with(fill("B_F11", 16, false)),
		gv("GV F1 8", "8", "B_F14", "Z_F1").with(rel("B_F14", "B_F11"), fill("B_F11", 16, false)),
		cz("CZ GET_FILL0 FREE_FILL1", "Z_F0", za{flags: "18", name: "FILL01"}, true),
		gv("GV F0 40", "40", "B_F01", "Z_F0").with(fill("B_F01", 10, false)),
		gv("GV F0 40 second", "40", "B_F02", "Z_F0"),
		fv("FV F0 40", "40", "B_F01", "Z_F0").with(fill("B_F01", 10, true)),
		cz("CZ fixed 16, FREE_FILL1", "Z_FF1", za{alg: "4", arg: "16", flags: "16", name: "FIXFILL"}, true),
		gv("GV FF1 16", "16", "B_XF1", "Z_FF1"),
		gv("GV FF1 16 second", "16", "B_XF2", "Z_FF1"),
		fv("FV FF1 16", "16", "B_XF1", "Z_FF1").with(fill("B_XF1", 4, true)),
		cz("CZ tags, FREE_FILL1", "Z_TF1", za{flags: "17", name: "TAGFILL"}, true),
		gv("GV TF1 16", "16", "B_TF1", "Z_TF1"),
		gv("GV TF1 16 second", "16", "B_TF2", "Z_TF1"),
		fv("FV TF1 16", "", "B_TF1", "Z_TF1").with(fill("B_TF1", 4, true), at("B_TF1", -8, 2), at("B_TF1", 16, 2)),

		// The page limit, NO_EXTEND, EXTEND_AREA, TAIL_LARGE.
		cz("CZ limit 4, initial 2, extend 1", "Z_PL", za{initial: "2", limit: "4", extend: "1", name: "LIMIT"}, true),
		seq("GV PL", "B_PL", "Z_PL", "600", "600", "600", "600"),
		svz("SVZ PL level 2", "Z_PL", "2"),
		cz("CZ NO_EXTEND, initial 2", "Z_NE", za{flags: "64", initial: "2", name: "NOEXTEND"}, true),
		seq("GV NE", "B_NE", "Z_NE", "400", "400", "400"),
		svz("SVZ NE level 2", "Z_NE", "2"),
		cz("CZ EXTEND_AREA, initial 1, extend 1", "Z_EA", za{flags: "32", initial: "1", extend: "1", name: "EXTAREA"}, true),
		seq("GV EA", "B_EA", "Z_EA", "400", "400", "400"),
		svz("SVZ EA level 2", "Z_EA", "2"),
		cz("CZ initial 1, extend 1", "Z_NA", za{initial: "1", extend: "1", name: "NOEXTAREA"}, true),
		seq("GV NA", "B_NA", "Z_NA", "400", "400", "400"),
		svz("SVZ NA level 2", "Z_NA", "2"),
		cz("CZ TAIL_LARGE, initial 1, extend 1", "Z_TL", za{flags: "128", initial: "1", extend: "1", name: "TAIL"}, true),
		seq("GV TL", "B_TL", "Z_TL", "100", "2000", "100", "600", "100"),
		svz("SVZ TL level 2", "Z_TL", "2"),
		cz("CZ initial 1, extend 1, no TAIL_LARGE", "Z_HL", za{initial: "1", extend: "1", name: "HEAD"}, true),
		seq("GV HL", "B_HL", "Z_HL", "100", "2000", "100", "600", "100"),
		svz("SVZ HL level 2", "Z_HL", "2"),

		// RESET_VM_ZONE and DELETE_VM_ZONE.
		cz("CZ reset, FREE_FILL1", "Z_RS", za{flags: "16", name: "RESET"}, true),
		seq("GV RS", "B_RS", "Z_RS", "16", "16", "100"),
		zoneOp("RZ RS", "LIB$RESET_VM_ZONE", "Z_RS").with(fill("B_RS1", 4, true), fill("B_RS3", 25, true)),
		seq("GV RS after reset", "B_RA", "Z_RS", "16", "16", "100", "16"),
		probeCase{name: "the blocks after reset"}.with(rel("B_RA1", "B_RS1"), rel("B_RA2", "B_RS2"), rel("B_RA3", "B_RS3")),
		svz("SVZ RS level 2", "Z_RS", "2"),
		cz("CZ delete, FREE_FILL1", "Z_DL", za{flags: "16", name: "DELETE"}, true),
		gv("GV DL 64", "64", "B_DL1", "Z_DL"),
		zoneOp("DZ DL", "LIB$DELETE_VM_ZONE", "Z_DL").with(fill("B_DL1", 16, true)).counted(),
		gp("GP after DZ", "1", "P_10").with(set("P_10")),
		probeCase{name: "the page and the deleted block"}.with(rel("B_DL1", "P_10")),
		fp("FP after DZ", "1", "P_10", ""),
		zoneOp("DZ zone 0", "LIB$DELETE_VM_ZONE", "ZERO"),
		zoneOp("RZ zone 0", "LIB$RESET_VM_ZONE", "ZERO"),
		probeCase{name: "DZ zone-id omitted", call: "LIB$DELETE_VM_ZONE", args: []string{"0"}},
		probeCase{name: "RZ zone-id omitted", call: "LIB$RESET_VM_ZONE", args: []string{"0"}},
		cz("CZ for RZ twice", "Z_R2", za{name: "RESET2"}, true),
		zoneOp("RZ twice 1", "LIB$RESET_VM_ZONE", "Z_R2"),
		zoneOp("RZ twice 2", "LIB$RESET_VM_ZONE", "Z_R2"),
		zoneOp("DZ after RZ", "LIB$DELETE_VM_ZONE", "Z_R2"),
		cz("CZ for DZ with blocks", "Z_DB", za{name: "DELBLOCKS"}, true),
		gv("GV DB 100", "100", "B_DB1", "Z_DB"),
		gv("GV DB 200", "200", "B_DB2", "Z_DB"),
		zoneOp("DZ with blocks", "LIB$DELETE_VM_ZONE", "Z_DB").counted(),

		// User zones: the manual's Example 5-1.
		cz("CZ the monitor's real zone", "Z_R1", za{name: "REAL1"}, true),
		cuz("CUZ monitor", "Z_U1", "Z_R1", monitor, "MONITOR", 0),
		ugv("UGV 16", "16", "Z_U1", "Z_R1").with(pgo("GVB")).counted(),
		ufv("UFV 16", "16", "Z_U1", "Z_R1").counted(),
		ugv("UGV 24", "24", "Z_U1", "Z_R1"),
		ufv("UFV size omitted", "", "Z_U1", "Z_R1"),
		ugv("UGV 0", "0", "Z_U1", "Z_R1"),
		ugv("UGV -1", "-1", "Z_U1", "Z_R1"),
		ugv("UGV 32", "32", "Z_U1", "Z_R1"),
		ufv("UFV 0", "0", "Z_U1", "Z_R1"),
		ufv("UFV 32", "32", "Z_U1", "Z_R1"),
		ufv("UFV 32 again", "32", "Z_U1", "Z_R1"),
		ugv("UGV allocation fails", "16", "Z_U1", "Z_R1").before("CLRL\tUBITS"),
		user(zoneOp("URZ monitor", "LIB$RESET_VM_ZONE", "Z_U1"), "Z_U1", "Z_R1"),
		user(svz("SVZ monitor level 0", "Z_U1", "0"), "Z_U1", "Z_R1"),
		user(svz("SVZ monitor level 3", "Z_U1", "3"), "Z_U1", "Z_R1"),
		user(verify("VZ monitor", "Z_U1"), "Z_U1", "Z_R1"),
		findAll("FIND with a user zone"),
		user(zoneOp("UDZ monitor", "LIB$DELETE_VM_ZONE", "Z_U1"), "Z_U1", "Z_R1"),
		findAll("FIND after the monitor"),
		cuz("CUZ no routines", "Z_U2", "UARGV", [4]string{"0", "0", "0", "0"}, "NOROUTINES", 0),
		ugv("UGV no routine", "16", "Z_U2", ""),
		ufv("UFV no routine", "16", "Z_U2", ""),
		user(zoneOp("URZ no routine", "LIB$RESET_VM_ZONE", "Z_U2"), "Z_U2", ""),
		user(zoneOp("UDZ no routine", "LIB$DELETE_VM_ZONE", "Z_U2"), "Z_U2", ""),
		user(svz("SVZ no routines level 1", "Z_U2", "1"), "Z_U2", ""),
		findAll("FIND after UDZ no routine"),
		cz("CZ a real zone for a failing delete", "Z_R3", za{name: "REAL3"}, true),
		cuz("CUZ failing delete", "Z_U3", "Z_R3", monitor, "FAILDELETE", 0),
		user(zoneOp("UDZ fails", "LIB$DELETE_VM_ZONE", "Z_U3").before("CLRL\tDBITS"), "Z_U3", "Z_R3"),
		ugv("UGV after a failed delete", "16", "Z_U3", "Z_R3"),
		user(zoneOp("UDZ again", "LIB$DELETE_VM_ZONE", "Z_U3"), "Z_U3", "Z_R3"),
		cz("CZ a real zone for Example 5-1's delete", "Z_R4", za{name: "REAL4"}, true),
		cuz("CUZ with LIB$DELETE_VM_ZONE", "Z_U4", "Z_R4",
			[4]string{"UALLOC", "UFREE", "URESET", "G^LIB$DELETE_VM_ZONE"}, "EXAMPLE", 0),
		user(zoneOp("UDZ by LIB$DELETE_VM_ZONE", "LIB$DELETE_VM_ZONE", "Z_U4"), "Z_U4", "Z_R4"),
		findAll("FIND after Example 5-1's delete"),
		cuz("CUZ zone-id only", "Z_U5", "0", [4]string{"0", "0", "0", "0"}, "", 1),
		user(svz("SVZ zone-id only level 1", "Z_U5", "1"), "Z_U5", ""),
		cuz("CUZ user argument omitted", "Z_U6", "0", monitor, "NOUSERARG", 0),
		ugv("UGV user argument omitted", "16", "Z_U6", ""),
		cuz("CUZ 8 arguments", "Z_T", "UARGV", [4]string{"0", "0", "0", "0"}, "EIGHT", 8),
		probeCase{name: "CUZ name class 99", data: []string{"%M:\t.WORD\t5\n\t.BYTE\t14,99\n\t.ADDRESS\t%M_T\n%M_T:\t.ASCII\t/BADCL/"},
			pre: []string{"MOVL\t" + untouch + ",Z_T"}, call: "LIB$CREATE_USER_VM_ZONE",
			args: []string{"Z_T", "UARGV", "0", "0", "0", "0", "%M"}}.with(set("Z_T")),
		probeCase{name: "CUZ no arguments", call: "LIB$CREATE_USER_VM_ZONE"},

		// A zone's own get-page and free-page routines.
		cz("CZ page routines at 12 and 13", "Z_G1",
			za{initial: "2", extend: "1", name: "PAGES1213", tail: []string{"GPAGE", "FPAGE"}}, true),
		gv("GV G1 600", "600", "B_G1", "Z_G1"),
		gv("GV G1 2000", "2000", "B_G2", "Z_G1"),
		zoneOp("DZ G1", "LIB$DELETE_VM_ZONE", "Z_G1"),
		cz("CZ page routines at 13 and 14", "Z_G2",
			za{initial: "2", extend: "1", name: "PAGES1314", tail: []string{"0", "GPAGE", "FPAGE"}}, true),
		gv("GV G2 600", "600", "B_G3", "Z_G2"),
		zoneOp("DZ G2", "LIB$DELETE_VM_ZONE", "Z_G2"),
		cz("CZ get-page at 13 of 13", "Z_G3",
			za{extend: "1", name: "PAGES13", tail: []string{"0", "GPAGE"}}, true),
		gv("GV G3 16", "16", "B_G4", "Z_G3"),
		zoneOp("DZ G3", "LIB$DELETE_VM_ZONE", "Z_G3"),
		cz("CZ get-page only", "Z_G4", za{initial: "1", extend: "1", name: "GETONLY", tail: []string{"GPAGE"}}, true),
		zoneOp("DZ G4", "LIB$DELETE_VM_ZONE", "Z_G4"),
		cz("CZ get-page fails", "Z_G5",
			za{initial: "1", name: "GETFAILS", tail: []string{"GPAGE", "FPAGE"}}, true).before("CLRL\tPBITS"),
		cz("CZ get-page fails later", "Z_G6",
			za{extend: "1", name: "GETFAILS2", tail: []string{"GPAGE", "FPAGE"}}, true),
		gv("GV G6 when get-page fails", "16", "B_G5", "Z_G6").before("CLRL\tPBITS"),
		gv("GV G6", "16", "B_G6", "Z_G6"),
		zoneOp("DZ G6, free-page fails", "LIB$DELETE_VM_ZONE", "Z_G6").before("CLRL\tPBITS"),
		findAll("FIND after the page routines"),

		// The statistics.
		gv("STAT GV 9", "9", "B_ST1", "Z_FF").counted(),
		fv("STAT FV 9", "9", "B_ST1", "Z_FF").counted(),
		gv("STAT GV 9 tagged", "9", "B_ST2", "Z_TG").counted(),
		fv("STAT FV tagged, size omitted", "", "B_ST2", "Z_TG").counted(),
		gv("STAT GV fails", "0", "B_T", "Z_FF").counted(),
		fv("STAT FV fails", "9", "B_ST1", "Z_FF").counted(),
		probeCase{name: "STAT SGET1_DD", data: []string{long("%N", "20")}, call: "LIB$SGET1_DD",
			args: []string{"%N", "DYNSTR"}}.counted(),
		probeCase{name: "STAT SFREE1_DD", call: "LIB$SFREE1_DD", args: []string{"DYNSTR"}}.counted(),
		probeCase{name: "STAT GET_VM_PAGE 2 and FREE 1", data: []string{long("%N", "2")}, call: "LIB$GET_VM_PAGE",
			args: []string{"%N", "P_11"}, post: []string{"PUSHAL\tP_11", "PUSHAL\tONE", "CALLS\t#2,G^LIB$FREE_VM_PAGE"}}.counted(),
		fp("STAT FREE_VM_PAGE the other", "1", "P_11", "512").counted(),
		statVM("STAT_VM 0", "0"),
		statVM("STAT_VM 1", "1"),
		statVM("STAT_VM 4", "4"),
		statVM("STAT_VM 8", "8"),
		statVM("STAT_VM -1", "-1"),
		probeCase{name: "STAT_VM value omitted", data: []string{long("%C", "1")}, call: "LIB$STAT_VM", args: []string{"%C", "0"}},
		probeCase{name: "STAT_VM 1 argument", data: []string{long("%C", "1")}, call: "LIB$STAT_VM", args: []string{"%C"}},
		probeCase{name: "SHOW_VM to SYS$OUTPUT", call: "LIB$SHOW_VM", raw: true},
		probeCase{name: "SHOW_VM 4 to SYS$OUTPUT", data: []string{long("%C", "4")}, call: "LIB$SHOW_VM",
			args: []string{"%C"}, raw: true},
		showVM("SHOW_VM omitted", ""),
		showVM("SHOW_VM 0", "0"),
		showVM("SHOW_VM 1", "1"),
		showVM("SHOW_VM 2", "2"),
		showVM("SHOW_VM 3", "3"),
		showVM("SHOW_VM 4", "4"),
		showVM("SHOW_VM 5", "5"),
		showVM("SHOW_VM 6", "6"),
		showVM("SHOW_VM 7", "7"),
		showVM("SHOW_VM 8", "8"),
		showVM("SHOW_VM -1", "-1"),
		showVM("SHOW_VM 0 action fails", "0").before("CLRL\tACTBITS"),
		showVM("SHOW_VM 4 action fails", "4").before("CLRL\tACTBITS"),
		probeCase{name: "SHOW_VM user argument omitted", data: []string{long("%C", "1")}, call: "LIB$SHOW_VM",
			args: []string{"%C", "ACTION"}}.with(act()),
		probeCase{name: "SHOW_VM 4 arguments", data: []string{long("%C", "1")}, call: "LIB$SHOW_VM",
			args: []string{"%C", "ACTION", userArg, "#7"}}.with(act()),

		// FIND, SHOW, and VERIFY.
		findAll("FIND all"),
		probeCase{name: "FIND 1 argument", data: []string{long("%C", "0")}, call: "LIB$FIND_VM_ZONE", args: []string{"%C"}},
		probeCase{name: "FIND 3 arguments", data: []string{long("%C", "0")}, call: "LIB$FIND_VM_ZONE",
			args: []string{"%C", "Z_T", "0"}},
		probeCase{name: "FIND context 2", data: []string{long("%C", "0"), long("%Z", "0")},
			pre:  []string{"PUSHAL\t%Z", "PUSHAL\t%C", "CALLS\t#2,G^LIB$FIND_VM_ZONE"},
			call: "LIB$FIND_VM_ZONE", args: []string{"%C", "%Z"},
			post: []string{"CLRL\t%E", "CMPL\t%Z,Z_STRING", "BNEQ\t%Y", "MOVL\t#1,%E", "%Y:"},
			show: []show{mem("%E", 1)}}.withData(long("%E", "0")),
		svz("SVZ FF default level", "Z_FF", ""),
		probeCase{name: "SVZ FF to SYS$OUTPUT level 1", data: []string{long("%L", "1")}, call: "LIB$SHOW_VM_ZONE",
			args: []string{"Z_FF", "%L"}, raw: true},
		probeCase{name: "SVZ FF zone-id only", call: "LIB$SHOW_VM_ZONE", args: []string{"Z_FF"}, raw: true},
		svz("SVZ FF level 4", "Z_FF", "4"),
		svz("SVZ FF level -1", "Z_FF", "-1"),
		svz("SVZ junk zone", "JUNKZ", "1"),
		svz("SVZ action fails", "Z_FF", "1").before("MOVL\t#^X55555553,ACTBITS"),
		probeCase{name: "SVZ zone-id omitted", data: []string{long("%L", "0")}, call: "LIB$SHOW_VM_ZONE",
			args: []string{"0", "%L", "ACTION", userArg}}.with(act()),
		probeCase{name: "SVZ no arguments", call: "LIB$SHOW_VM_ZONE"},
		probeCase{name: "SVZ 5 arguments", data: []string{long("%L", "0")}, call: "LIB$SHOW_VM_ZONE",
			args: []string{"Z_FF", "%L", "ACTION", userArg, "#0"}}.with(act()),
		verify("VZ default", "Z_DEFAULT"),
		verify("VZ string", "Z_STRING"),
		verify("VZ zone 0", "ZERO"),
		probeCase{name: "VZ zone-id omitted", call: "LIB$VERIFY_VM_ZONE", args: []string{"0"}},
		verify("VZ junk zone", "JUNKZ"),
		probeCase{name: "VZ no arguments", call: "LIB$VERIFY_VM_ZONE"},
		probeCase{name: "VZ 2 arguments", call: "LIB$VERIFY_VM_ZONE", args: []string{"Z_FF", "0"}},

		// The C library's routines, by value.
		cmalloc("VM_MALLOC 24", "LIB$VM_MALLOC", "B_M1", "#24").with(pgo("B_M1")).counted(),
		cmalloc("VM_MALLOC 0", "LIB$VM_MALLOC", "B_M2", "#0").with(rel("B_M2", "B_M1")),
		cmalloc("VM_MALLOC 24 again", "LIB$VM_MALLOC", "B_M3", "#24").with(rel("B_M3", "B_M1")),
		cmalloc("VM_FREE first", "LIB$VM_FREE", "", "=B_M1").counted(),
		cmalloc("VM_FREE 0", "LIB$VM_FREE", "", "#0"),
		cmalloc("VM_MALLOC 16 after", "LIB$VM_MALLOC", "B_M4", "#16").with(rel("B_M4", "B_M1")),
		cmalloc("VM_CALLOC 3, 8", "LIB$VM_CALLOC", "B_M5", "#3", "#8").with(rel("B_M5", "B_M1"), fill("B_M5", 6, false)),
		cmalloc("VM_REALLOC to 100", "LIB$VM_REALLOC", "B_M6", "=B_M5", "#100").with(rel("B_M6", "B_M5")),
		cmalloc("VM_REALLOC to 8", "LIB$VM_REALLOC", "B_M7", "=B_M6", "#8").with(rel("B_M7", "B_M6")),
		cmalloc("VM_REALLOC of 0", "LIB$VM_REALLOC", "B_M8", "#0", "#16").with(rel("B_M8", "B_M1")),
		cmalloc("VM_REALLOC to 0", "LIB$VM_REALLOC", "B_T", "=B_M8", "#0"),
		probeCase{name: "the C blocks and the default zone's"}.with(rel("B_M1", "B_D1")),
		cmalloc("VM_FREE a default zone block", "LIB$VM_FREE", "", "=B_M3"),
		gv("GV default after the C blocks", "16", "B_D4", "").with(rel("B_D4", "B_M3")),
		probeCase{name: "FV a C block", data: []string{long("%N", "16")}, call: "LIB$FREE_VM", args: []string{"%N", "B_M4"}},
		findAll("FIND after the C routines"),

		// Last, the cases that may damage VMS's zones, each in one of
		// its own.
		cz("CZ for a double free", "Z_X1", za{name: "DOUBLE"}, true),
		gv("GV X1 32", "32", "B_X1", "Z_X1"),
		gv("GV X1 32 second", "32", "B_X2", "Z_X1"),
		fv("FV X1 first", "32", "B_X1", "Z_X1"),
		fv("FV X1 first again", "32", "B_X1", "Z_X1"),
		verify("VZ X1", "Z_X1"),
		cz("CZ for an interior free", "Z_X2", za{name: "INTERIOR"}, true),
		gv("GV X2 64", "64", "B_X3", "Z_X2"),
		gv("GV X2 64 second", "64", "B_X4", "Z_X2"),
		fvAt("FV X2 interior", "56", "B_X3", "8", "Z_X2"),
		fvAt("FV X2 unaligned", "60", "B_X3", "4", "Z_X2"),
		fvAt("FV X2 past the areas", "8", "B_X3", "65536", "Z_X2"),
		verify("VZ X2", "Z_X2"),
		cz("CZ for a larger free", "Z_X3", za{name: "LARGER"}, true),
		gv("GV X3 32", "32", "B_X5", "Z_X3"),
		gv("GV X3 32 second", "32", "B_X6", "Z_X3"),
		gv("GV X3 32 third", "32", "B_X7", "Z_X3"),
		fv("FV X3 64 of 32", "64", "B_X5", "Z_X3"),
		gv("GV X3 64", "64", "B_X8", "Z_X3").with(rel("B_X8", "B_X5")),
		verify("VZ X3", "Z_X3"),
		cz("CZ for a smaller free", "Z_X4", za{name: "SMALLER"}, true),
		gv("GV X4 64", "64", "B_X9", "Z_X4"),
		gv("GV X4 64 second", "64", "B_X10", "Z_X4"),
		fv("FV X4 32 of 64", "32", "B_X9", "Z_X4"),
		gv("GV X4 32", "32", "B_X11", "Z_X4").with(rel("B_X11", "B_X9")),
		gv("GV X4 32 second", "32", "B_X12", "Z_X4").with(rel("B_X12", "B_X9")),
		svz("SVZ X4 level 3", "Z_X4", "3"),
		cz("CZ fixed, for a double free", "Z_X5", za{alg: "4", arg: "16", name: "FIXDOUBLE"}, true),
		gv("GV X5 16", "16", "B_X13", "Z_X5"),
		fv("FV X5", "16", "B_X13", "Z_X5"),
		fv("FV X5 again", "16", "B_X13", "Z_X5"),
		gv("GV X5 after", "16", "B_X14", "Z_X5").with(rel("B_X14", "B_X13")),
		gv("GV X5 after, second", "16", "B_X15", "Z_X5").with(rel("B_X15", "B_X13")),
		svz("SVZ X5 level 3", "Z_X5", "3"),
		verify("VZ X5", "Z_X5"),
		cz("CZ quick fit, for a double free", "Z_X6", za{alg: "2", arg: "4", smallest: "16", name: "QFDOUBLE"}, true),
		gv("GV X6 16", "16", "B_X16", "Z_X6"),
		fv("FV X6", "16", "B_X16", "Z_X6"),
		fv("FV X6 again", "16", "B_X16", "Z_X6"),
		gv("GV X6 after", "16", "B_X17", "Z_X6").with(rel("B_X17", "B_X16")),
		gv("GV X6 after, second", "16", "B_X18", "Z_X6").with(rel("B_X18", "B_X16")),
		svz("SVZ X6 level 3", "Z_X6", "3"),
		cz("CZ tags, for a changed tag", "Z_X7", za{flags: "1", name: "BADTAG"}, true),
		gv("GV X7 16", "16", "B_X19", "Z_X7"),
		gv("GV X7 16 second", "16", "B_X20", "Z_X7"),
		fv("FV X7 changed tag", "", "B_X19", "Z_X7").before("MOVL\tB_X19,R1", "INCL\t-4(R1)"),
		fv("FV X7 changed tag, size given", "16", "B_X19", "Z_X7"),
		fv("FV X7 changed end tag", "", "B_X20", "Z_X7").before("MOVL\tB_X20,R1", "INCL\t16(R1)"),
		verify("VZ X7", "Z_X7"),
		cz("CZ FREE_FILL1, for an overwritten free block", "Z_X8", za{flags: "16", name: "OVERWRITE"}, true),
		gv("GV X8 64", "64", "B_X21", "Z_X8"),
		gv("GV X8 64 second", "64", "B_X22", "Z_X8"),
		fv("FV X8", "64", "B_X21", "Z_X8"),
		verify("VZ X8 sound", "Z_X8"),
		probeCase{name: "write in X8's free block", pre: []string{"MOVL\tB_X21,R1", "CLRL\t32(R1)", "MOVL\t#1,R0"}},
		verify("VZ X8 overwritten", "Z_X8"),
		svz("SVZ X8 level 3", "Z_X8", "3"),
		findAll("FIND with X8 overwritten"),
		cz("CZ for FREE_VM_PAGE of an area", "Z_X9", za{initial: "1", name: "AREAPAGE"}, true),
		gv("GV X9 16", "16", "B_X23", "Z_X9"),
		probeCase{name: "FP X9's area page", data: []string{long("%A", "0")},
			pre: []string{"BICL3\t#^X1FF,B_X23,%A"}, call: "LIB$FREE_VM_PAGE", args: []string{"ONE", "%A"}},
		gv("GV X9 after", "16", "B_X24", "Z_X9").with(rel("B_X24", "B_X23")),
		verify("VZ X9", "Z_X9"),

		// Deleted zones' IDs.
		gv("GV deleted zone", "16", "B_T", "Z_DL").with(set("B_T")),
		fv("FV deleted zone", "64", "B_DL1", "Z_DL"),
		zoneOp("RZ deleted zone", "LIB$RESET_VM_ZONE", "Z_DL"),
		zoneOp("DZ deleted zone", "LIB$DELETE_VM_ZONE", "Z_DL"),
		svz("SVZ deleted zone", "Z_DL", "1"),
		verify("VZ deleted zone", "Z_DL"),
		gv("GV the monitor's deleted real zone", "16", "B_T", "Z_R1").with(set("B_T")),
		ugv("UGV the deleted monitor", "16", "Z_U1", "Z_R1"),
		user(zoneOp("UDZ the deleted monitor", "LIB$DELETE_VM_ZONE", "Z_U1"), "Z_U1", "Z_R1"),

		// Junk zone IDs and the system zones.
		gv("GV junk zone", "16", "B_T", "JUNKZ").with(set("B_T")),
		fv("FV junk zone", "16", "B_FF9", "JUNKZ"),
		zoneOp("RZ junk zone", "LIB$RESET_VM_ZONE", "JUNKZ"),
		zoneOp("DZ junk zone", "LIB$DELETE_VM_ZONE", "JUNKZ"),
		zoneOp("RZ the default zone by ID", "LIB$RESET_VM_ZONE", "Z_DEFAULT"),
		zoneOp("DZ the default zone by ID", "LIB$DELETE_VM_ZONE", "Z_DEFAULT"),
		zoneOp("RZ the string zone", "LIB$RESET_VM_ZONE", "Z_STRING"),
		zoneOp("DZ the string zone", "LIB$DELETE_VM_ZONE", "Z_STRING"),
		findAll("FIND after the system zones"),

		// The argument counts.
		probeCase{name: "GV 1 argument", data: []string{long("%N", "16")}, call: "LIB$GET_VM", args: []string{"%N"}},
		probeCase{name: "GV 4 arguments", data: []string{long("%N", "16")}, call: "LIB$GET_VM",
			args: []string{"%N", "B_T", "Z_FF", "0"}}.with(set("B_T")),
		probeCase{name: "FV 1 argument", data: []string{long("%N", "16")}, call: "LIB$FREE_VM", args: []string{"%N"}},
		probeCase{name: "GV no arguments", call: "LIB$GET_VM"},
		probeCase{name: "DZ no arguments", call: "LIB$DELETE_VM_ZONE"},
		probeCase{name: "RZ no arguments", call: "LIB$RESET_VM_ZONE"},
		probeCase{name: "GP no arguments", call: "LIB$GET_VM_PAGE"},
		probeCase{name: "STAT_VM no arguments", call: "LIB$STAT_VM"},

		// A zone whose control block the program changed.
		cz("CZ for a changed control block", "Z_XC", za{name: "HEADER"}, true),
		probeCase{name: "change XC's first longword", pre: []string{"MOVL\tZ_XC,R1", "INCL\t(R1)", "MOVL\t#1,R0"}},
		verify("VZ XC", "Z_XC"),
		gv("GV XC", "16", "B_T", "Z_XC").with(set("B_T")),
		findAll("FIND with XC changed"),
		probeCase{name: "STAT at the end", pre: []string{"CALLS\t#0,STATREAD"}, show: []show{vmem("SV", 6)}},
	)
}

// withData adds data lines to a case.
func (c probeCase) withData(lines ...string) probeCase {
	c.data = append(append([]string(nil), c.data...), lines...)

	return c
}

// ---------------------------------------------------------------------
// The program's fixed parts.

const programHead = `	.TITLE	LIB59	Phase 59 LIBRTL memory routines probe
	.IDENT	/V1.0/

; Written by testdata/lib59/gen.go: do not edit. Calls each of Phase
; 59's LIBRTL routines, case by case, each in a routine of its own under
; HANDLER, and prints with LIB$PUT_OUTPUT what it returned in R0, any
; condition it signaled, what it stored, and what it passed the routines
; the program gave it. See gen.go.

	$SSDEF

LOGSIZE = 60000				; the log's size in bytes
FAILST = ` + failSt + `			; what a routine returns for failure

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
INCALL:	.LONG	0		; a case is running: lines go to the log
LOGEND:	.LONG	0		; the log's length
LOGLOST:	.LONG	0		; lines the log had no room for
UBITS:	.LONG	-1		; the user zone routines' answers, a bit a call
DBITS:	.LONG	-1		; the user delete routine's
PBITS:	.LONG	-1		; the page routines'
ACTBITS:	.LONG	-1		; the action routine's
ACTN:	.LONG	0		; the action routine's calls
ACTARGC:	.LONG	0		; its last call's argument count
ACTARG:	.LONG	0		; and user argument
ACTDSC:	.LONG	0		; and its descriptor's type and class
UZCUR:	.LONG	0		; the user zone the routines report against
RZCUR:	.LONG	0		; and its real zone
GVN:	.LONG	0		; a user zone case's number-of-bytes
GVB:	.LONG	0		; and base-address
FACTX:	.LONG	0		; FINDALL's context
FAZONE:	.LONG	0		; and zone
T1:	.LONG	0
T2:	.LONG	0
T3:	.LONG	0
STATK:	.LONG	0		; a LIB$STAT_VM code
STATCODES:	.LONG	1,2,3,5,6,7
SV:	.BLKL	6		; the figures now
SN:	.BLKL	6		; at the snapshot
SD:	.BLKL	6		; the difference
DYNSTR:	.WORD	0		; a dynamic string
	.BYTE	14,2
	.LONG	0
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
LOGDSC:	.WORD	0
	.BYTE	14,1
	.LONG	0
LINE:	.BLKB	256
RAWB:	.ASCID	"{{"
RAWE:	.ASCID	"}}"
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
VMEMCTL:	.ASCID	"  ~!AC:"
LWCTL:	.ASCID	" !XL"
RELCTL:	.ASCID	"  !AC: !XL"
UNTCTL:	.ASCID	"  !AC: untouched"
SETCTL:	.ASCID	"  !AC: set"
FILLCTL:	.ASCID	"  !AC: !AD"
ACTSCTL:	.ASCID	"  action: calls=!UL argc=!XL arg=!XL dsc=!XL"
LOSTCTL:	.ASCID	"  log: !UL lines lost"
ACTCTL:	.ASCID	"  | !AS"
FINDCTL:	.ASCID	"  = end !XL after !UL"
CBHCTL:	.ASCID	"  > !AC !UL"
CB0CTL:	.ASCID	"  >  !UL: 0"
UNCTL:	.ASCID	"  >  !UL: GVN !SL"
UBCTL:	.ASCID	"  >  !UL: GVB"
UZRCTL:	.ASCID	"  >  !UL: zone+!XL the real zone"
UZVCTL:	.ASCID	"  >  !UL: zone+!XL value !XL"
USRCTL:	.ASCID	"  >  !UL: stack, the real zone"
USVCTL:	.ASCID	"  >  !UL: stack, value !XL"
UECTL:	.ASCID	"  >  !UL: elsewhere"
USTCTL:	.ASCID	"  >   status !XL"
GPCTL:	.ASCID	"  > !AC !UL n=!SL"
NM_UALLOC:	.ASCIC	"alloc"
NM_UFREE:	.ASCIC	"free"
NM_URESET:	.ASCIC	"reset"
NM_UDELETE:	.ASCIC	"delete"
NM_GPAGE:	.ASCIC	"get-page"
NM_FPAGE:	.ASCIC	"free-page"
	.ALIGN	LONG
LOGBUF:	.BLKB	LOGSIZE
`

const programCode = `
	.PSECT	CODE,NOWRT,EXE

; HANDLER records the condition (and an access violation's reason mask
; and address), then unwinds to the caller of the routine that set it up,
; which gets the condition in R0. It ignores SS$_UNWIND.
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

; EMIT writes the line in LINE (LINLEN long): to the log while a case
; runs (INCALL), else to SYS$OUTPUT.
	.ENTRY	EMIT,^M<R2,R3,R4,R5>
	MOVZWL	LINLEN,R2
	TSTL	INCALL
	BEQL	2$
	ADDL3	LOGEND,R2,R3
	ADDL2	#2,R3
	CMPL	R3,#LOGSIZE
	BGTRU	1$
	MOVAB	LOGBUF,R4
	ADDL2	LOGEND,R4
	MOVW	R2,(R4)+
	MOVL	R3,LOGEND
	MOVC3	R2,LINE,(R4)
	RET
1$:	INCL	LOGLOST
	RET
2$:	MOVW	R2,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; FLUSH writes the log's lines to SYS$OUTPUT and empties it.
	.ENTRY	FLUSH,^M<R2,R3>
	MOVAB	LOGBUF,R2
	ADDL3	R2,LOGEND,R3
1$:	CMPL	R2,R3
	BGEQU	2$
	MOVZWL	(R2)+,R0
	MOVW	R0,LOGDSC
	MOVL	R2,LOGDSC+4
	ADDL2	R0,R2
	PUSHAQ	LOGDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	BRB	1$
2$:	CLRL	LOGEND
	TSTL	LOGLOST
	BEQL	3$
	$FAO_S	CTRSTR=LOSTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=LOGLOST
	CLRL	LOGLOST
	CALLS	#0,EMIT
3$:	RET

; RAWMARK marker: writes the marker straight to SYS$OUTPUT.
	.ENTRY	RAWMARK,^M<>
	PUSHL	4(AP)
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; RESETCB puts the routines' answers back as they start, success, and
; the action routine's record.
	.ENTRY	RESETCB,^M<>
	MNEGL	#1,UBITS
	MNEGL	#1,DBITS
	MNEGL	#1,PBITS
	MNEGL	#1,ACTBITS
	CLRL	ACTN
	MOVL	#^X55555555,ACTARGC
	MOVL	#^X55555555,ACTARG
	MOVL	#^X55555555,ACTDSC
	RET

; CBSTAT cell: the status a routine returns, from the cell's low bit (1
; is success, 0 FAILST), and the cell rotated a bit to the right.
	.ENTRY	CBSTAT,^M<>
	MOVL	4(AP),R1
	MOVL	#1,R0
	BLBS	(R1),1$
	MOVL	#FAILST,R0
1$:	ROTL	#-1,(R1),(R1)
	RET

; ACTION string, user-arg: LIB$SHOW_VM's and LIB$SHOW_VM_ZONE's action
; routine. Logs the line ("  | text"), records the call, and returns
; ACTBITS's next bit.
	.ENTRY	ACTION,^M<R2>
	INCL	ACTN
	MOVZBL	(AP),ACTARGC
	CLRL	ACTARG
	CMPB	(AP),#2
	BLSS	1$
	MOVL	8(AP),ACTARG
1$:	MOVL	4(AP),R2
	MOVZWL	2(R2),ACTDSC
	$FAO_S	CTRSTR=ACTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R2
	CALLS	#0,EMIT
	PUSHAB	ACTBITS
	CALLS	#1,CBSTAT
	RET

; FINDALL: each zone LIB$FIND_VM_ZONE finds, from context 0, shown at
; detail level 0 through ACTION; then the status that ended the scan and
; how many it found ("  = end").
	.ENTRY	FINDALL,^M<R2,R3>
	CLRL	FACTX
	CLRL	R3
1$:	PUSHAL	FAZONE
	PUSHAL	FACTX
	CALLS	#2,G^LIB$FIND_VM_ZONE
	BLBC	R0,2$
	INCL	R3
	CMPL	R3,#60
	BGTR	2$
	PUSHL	#0
	PUSHAB	ACTION
	PUSHAL	ZERO
	PUSHAL	FAZONE
	CALLS	#4,G^LIB$SHOW_VM_ZONE
	BRB	1$
2$:	MOVL	R0,R2
	$FAO_S	CTRSTR=FINDCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R2,P2=R3
	CALLS	#0,EMIT
	MOVL	R2,R0
	RET

; UPRINT name, ap: a user zone routine's line ("  > name argc"), then a
; line for each argument, by what it is: 0; GVN (with its value) or GVB,
; the case's cells; an address in the user zone's control block (UZCUR)
; or on the stack, with the longword there (the real zone, RZCUR, or a
; value); or elsewhere.
	.ENTRY	UPRINT,^M<R2,R3,R4,R5>
	MOVL	8(AP),R4
	MOVZBL	(R4),R5
	$FAO_S	CTRSTR=CBHCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R5
	CALLS	#0,EMIT
	CLRL	R3
1$:	CMPL	R3,R5
	BLSS	2$
	RET
2$:	MOVL	4(R4)[R3],R2
	ADDL3	#1,R3,T1
	TSTL	R2
	BNEQ	3$
	$FAO_S	CTRSTR=CB0CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1
	BRW	90$
3$:	MOVAL	GVN,R0
	CMPL	R2,R0
	BNEQ	4$
	$FAO_S	CTRSTR=UNCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1,P2=GVN
	BRW	90$
4$:	MOVAL	GVB,R0
	CMPL	R2,R0
	BNEQ	5$
	$FAO_S	CTRSTR=UBCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1
	BRW	90$
5$:	SUBL3	UZCUR,R2,T2
	CMPL	T2,#256
	BGEQU	7$
	MOVL	(R2),T3
	CMPL	T3,RZCUR
	BNEQ	6$
	$FAO_S	CTRSTR=UZRCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1,P2=T2
	BRW	90$
6$:	$FAO_S	CTRSTR=UZVCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1,P2=T2,P3=T3
	BRW	90$
7$:	SUBL3	SP,R2,T2
	CMPL	T2,#^X10000
	BGEQU	9$
	MOVL	(R2),T3
	CMPL	T3,RZCUR
	BNEQ	8$
	$FAO_S	CTRSTR=USRCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1
	BRW	90$
8$:	$FAO_S	CTRSTR=USVCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1,P2=T3
	BRW	90$
9$:	$FAO_S	CTRSTR=UECTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1
90$:	CALLS	#0,EMIT
	INCL	R3
	BRW	1$

; USTATUS: logs the status in R0 ("  >   status") and keeps it.
	.ENTRY	USTATUS,^M<R2>
	MOVL	R0,R2
	$FAO_S	CTRSTR=USTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R2
	CALLS	#0,EMIT
	MOVL	R2,R0
	RET

; The user zone routines: each logs its call and arguments, then, if
; its bit says so (UBITS, or DBITS for delete), does the operation on
; the zone its user argument names, as Example 5-1's routines do;
; otherwise it returns FAILST.
	.ENTRY	UALLOC,^M<>
	PUSHL	AP
	PUSHAB	NM_UALLOC
	CALLS	#2,UPRINT
	PUSHAB	UBITS
	CALLS	#1,CBSTAT
	BLBC	R0,9$
	PUSHL	12(AP)
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#3,G^LIB$GET_VM
	CALLS	#0,USTATUS
9$:	RET

	.ENTRY	UFREE,^M<>
	PUSHL	AP
	PUSHAB	NM_UFREE
	CALLS	#2,UPRINT
	PUSHAB	UBITS
	CALLS	#1,CBSTAT
	BLBC	R0,9$
	PUSHL	12(AP)
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#3,G^LIB$FREE_VM
	CALLS	#0,USTATUS
9$:	RET

	.ENTRY	URESET,^M<>
	PUSHL	AP
	PUSHAB	NM_URESET
	CALLS	#2,UPRINT
	PUSHAB	UBITS
	CALLS	#1,CBSTAT
	BLBC	R0,9$
	PUSHL	4(AP)
	CALLS	#1,G^LIB$RESET_VM_ZONE
	CALLS	#0,USTATUS
9$:	RET

	.ENTRY	UDELETE,^M<>
	PUSHL	AP
	PUSHAB	NM_UDELETE
	CALLS	#2,UPRINT
	PUSHAB	DBITS
	CALLS	#1,CBSTAT
	BLBC	R0,9$
	PUSHL	4(AP)
	CALLS	#1,G^LIB$DELETE_VM_ZONE
	CALLS	#0,USTATUS
9$:	RET

; GPAGE and FPAGE, a zone's get-page and free-page routines: each logs
; its call ("  > get-page argc n=count") and, if PBITS says so, passes
; it to LIB$GET_VM_PAGE or LIB$FREE_VM_PAGE, logging the status;
; otherwise it returns FAILST.
	.ENTRY	GPAGE,^M<R2>
	MOVAB	NM_GPAGE,R2
	BSBW	PAGELOG
	PUSHAB	PBITS
	CALLS	#1,CBSTAT
	BLBC	R0,9$
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#2,G^LIB$GET_VM_PAGE
	CALLS	#0,USTATUS
9$:	RET

	.ENTRY	FPAGE,^M<R2>
	MOVAB	NM_FPAGE,R2
	BSBW	PAGELOG
	PUSHAB	PBITS
	CALLS	#1,CBSTAT
	BLBC	R0,9$
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#2,G^LIB$FREE_VM_PAGE
	CALLS	#0,USTATUS
9$:	RET

; PAGELOG: the page routine's line, R2 its name, AP its arguments.
PAGELOG:	MOVZBL	(AP),T1
	CLRL	T2
	TSTL	4(AP)
	BEQL	1$
	MOVL	@4(AP),T2
1$:	$FAO_S	CTRSTR=GPCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R2,P2=T1,P3=T2
	CALLS	#0,EMIT
	RSB

; STATREAD: LIB$STAT_VM's six figures (codes 1 to 3, 5 to 7) into SV;
; R0 the last call's status. STATSNAP keeps them in SN, and STATDIFF
; puts what changed since in SD.
	.ENTRY	STATREAD,^M<R2>
	CLRL	R2
1$:	MOVL	STATCODES[R2],STATK
	PUSHAL	SV[R2]
	PUSHAL	STATK
	CALLS	#2,G^LIB$STAT_VM
	AOBLSS	#6,R2,1$
	RET

	.ENTRY	STATSNAP,^M<R2,R3,R4,R5>
	CALLS	#0,STATREAD
	MOVC3	#24,SV,SN
	RET

	.ENTRY	STATDIFF,^M<R2>
	CALLS	#0,STATREAD
	CLRL	R2
1$:	SUBL3	SN[R2],SV[R2],SD[R2]
	AOBLSS	#6,R2,1$
	RET

; SHOWR0 name, r0: the case's line.
	.ENTRY	SHOWR0,^M<>
	TSTL	SIGCOND
	BNEQ	1$
	$FAO_S	CTRSTR=R0CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=8(AP)
	BRB	2$
1$:	$FAO_S	CTRSTR=SIGCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=8(AP),-
		P3=SIGCOND,P4=SIGARG1,P5=SIGARG2
2$:	CALLS	#0,EMIT
	RET

; SHOWMEM name, address, count, control: count longwords in hex, after
; the name as control (MEMCTL or VMEMCTL) shows it. SHOWAT is the same
; under HANDLER, for an address that may not be readable.
	.ENTRY	SHOWAT,^M<R2,R3,R4>
	MOVAB	HANDLER,(FP)
	BRB	SHOWLW
	.ENTRY	SHOWMEM,^M<R2,R3,R4>
SHOWLW:	MOVL	8(AP),R2
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
2$:	MOVW	R4,LINLEN
	CALLS	#0,EMIT
	RET

; SHOWREL name, value, base: the value less the base, or "untouched".
	.ENTRY	SHOWREL,^M<>
	CMPL	8(AP),#^X55555555
	BEQL	1$
	SUBL3	12(AP),8(AP),T1
	$FAO_S	CTRSTR=RELCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=T1
	BRB	2$
1$:	$FAO_S	CTRSTR=UNTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
2$:	CALLS	#0,EMIT
	RET

; SHOWPGO name, value: the value's offset in its page, or "untouched".
	.ENTRY	SHOWPGO,^M<>
	CMPL	8(AP),#^X55555555
	BEQL	1$
	BICL3	#^XFFFFFE00,8(AP),T1
	$FAO_S	CTRSTR=RELCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=T1
	BRB	2$
1$:	$FAO_S	CTRSTR=UNTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
2$:	CALLS	#0,EMIT
	RET

; SHOWSET name, value: "set", or "untouched" for ^X55555555.
	.ENTRY	SHOWSET,^M<>
	CMPL	8(AP),#^X55555555
	BEQL	1$
	$FAO_S	CTRSTR=SETCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
	BRB	2$
1$:	$FAO_S	CTRSTR=UNTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
2$:	CALLS	#0,EMIT
	RET

; SHOWFILL name, address, count, fill: a character for each of count
; longwords at the address, F where it holds the fill, "." where it
; doesn't; "untouched" for an address of ^X55555555. Under HANDLER, so
; a page that can't be read ends only this line.
	.ENTRY	SHOWFILL,^M<R2,R3,R4>
	MOVAB	HANDLER,(FP)
	MOVL	8(AP),R2
	CMPL	R2,#^X55555555
	BNEQ	1$
	$FAO_S	CTRSTR=UNTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
	BRB	4$
1$:	MOVL	12(AP),R3
	MOVAB	FILLMAP,R4
2$:	MOVB	#^A"F",(R4)
	CMPL	(R2)+,16(AP)
	BEQL	3$
	MOVB	#^A".",(R4)
3$:	INCL	R4
	SOBGTR	R3,2$
	$FAO_S	CTRSTR=FILLCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=12(AP),P3=#FILLMAP
4$:	CALLS	#0,EMIT
	RET

; SHOWACT: the action routine's record.
	.ENTRY	SHOWACT,^M<>
	$FAO_S	CTRSTR=ACTSCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=ACTN,P2=ACTARGC,P3=ACTARG,P4=ACTDSC
	CALLS	#0,EMIT
	RET
`

// prefix substitutes the case's label prefix for "%".
func prefix(s string, n int) string {
	return strings.ReplaceAll(s, "%", fmt.Sprintf("C%d_", n))
}

// shownName is how a shown value is named in the output.
func shownName(s show) string {
	name := strings.ReplaceAll(s.label, "%", "")

	switch s.kind {
	case "rel":
		return name + "-" + strings.ReplaceAll(s.base, "%", "")
	case "pgo":
		return name + " in its page"
	case "fill":
		if s.ones {
			return name + " FF"
		}

		return name + " 00"
	case "at":
		return fmt.Sprintf("%s%+d", name, s.off)
	}

	return name
}

// cellPattern finds the shared cells the cases name.
var cellPattern = regexp.MustCompile(`\b[ZBP]_[A-Z0-9_]+\b`)

func program(cases []probeCase) string {
	var b strings.Builder

	b.WriteString(programHead)
	b.WriteString("\n; Shared data.\n")

	for _, line := range shared {
		b.WriteString(line + "\n")
	}

	b.WriteString("FILLMAP:\t.BLKB\t64\n")

	cells := map[string]bool{}

	for _, c := range cases {
		text := strings.Join(c.pre, " ") + strings.Join(c.args, " ") + strings.Join(c.post, " ")
		for _, s := range c.show {
			text += " " + s.label + " " + s.base
		}

		for _, m := range cellPattern.FindAllString(text, -1) {
			cells[m] = true
		}
	}

	names := make([]string, 0, len(cells))
	for name := range cells {
		names = append(names, name)
	}

	sort.Strings(names)
	b.WriteString("; The cells the cases share, preset to ^X55555555.\n")

	for _, name := range names {
		fmt.Fprintf(&b, "%s:\t.LONG\t^X55555555\n", name)
	}

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ALIGN\tQUAD\n", n, c.name)

		for _, line := range c.data {
			b.WriteString(prefix(line, n) + "\n")
		}

		fmt.Fprintf(&b, "C%d_NAME:\t.ASCIC\t%s\n", n, delimit(c.name))

		for k, sh := range c.show {
			if sh.kind != "act" {
				fmt.Fprintf(&b, "C%d_S%d:\t.ASCIC\t%s\n", n, k+1, delimit(shownName(sh)))
			}
		}
	}

	b.WriteString(programCode)

	// Each case's routine.
	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, c.name, n)

		if c.stats {
			b.WriteString("\tCALLS\t#0,STATSNAP\n")
		}

		for _, line := range c.pre {
			b.WriteString(prefixLine(line, n))
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
			b.WriteString(prefixLine(line, n))
		}

		if c.stats {
			b.WriteString("\tCALLS\t#0,STATDIFF\n")
		}

		b.WriteString("\tMOVL\t(SP)+,R0\n\tRET\n")
	}

	// The main routine: each case, then its lines.
	b.WriteString("\n\t.ENTRY\tSTART,^M<>\n")

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n\tCLRL\tSIGCOND\t\t\t; %d: %s\n\tCALLS\t#0,RESETCB\n", n, c.name)

		if c.raw {
			b.WriteString("\tPUSHAQ\tRAWB\n\tCALLS\t#1,RAWMARK\n")
		}

		fmt.Fprintf(&b, "\tMOVL\t#1,INCALL\n\tCALLS\t#0,C%d\n\tCLRL\tINCALL\n", n)

		if c.raw {
			b.WriteString("\tPUSHL\tR0\n\tPUSHAQ\tRAWE\n\tCALLS\t#1,RAWMARK\n\tMOVL\t(SP)+,R0\n")
		}

		fmt.Fprintf(&b, "\tPUSHL\tR0\n\tPUSHAB\tC%d_NAME\n\tCALLS\t#2,SHOWR0\n\tCALLS\t#0,FLUSH\n", n)

		for k, sh := range c.show {
			label := prefix(sh.label, n)
			name := fmt.Sprintf("C%d_S%d", n, k+1)

			switch sh.kind {
			case "mem", "vmem":
				ctl := "MEMCTL"
				if sh.kind == "vmem" {
					ctl = "VMEMCTL"
				}

				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\t%s\n\tCALLS\t#4,SHOWMEM\n", ctl, sh.count, label, name)
			case "at":
				fmt.Fprintf(&b, "\tADDL3\t#%d,%s,T1\n\tPUSHAQ\tVMEMCTL\n\tPUSHL\t#%d\n\tPUSHL\tT1\n\tPUSHAB\t%s\n\tCALLS\t#4,SHOWAT\n",
					sh.off, label, sh.count, name)
			case "rel":
				fmt.Fprintf(&b, "\tPUSHL\t%s\n\tPUSHL\t%s\n\tPUSHAB\t%s\n\tCALLS\t#3,SHOWREL\n", prefix(sh.base, n), label, name)
			case "pgo":
				fmt.Fprintf(&b, "\tPUSHL\t%s\n\tPUSHAB\t%s\n\tCALLS\t#2,SHOWPGO\n", label, name)
			case "set":
				fmt.Fprintf(&b, "\tPUSHL\t%s\n\tPUSHAB\t%s\n\tCALLS\t#2,SHOWSET\n", label, name)
			case "fill":
				pattern := "#0"
				if sh.ones {
					pattern = "#-1"
				}

				fmt.Fprintf(&b, "\tPUSHL\t%s\n\tPUSHL\t#%d\n\tPUSHL\t%s\n\tPUSHAB\t%s\n\tCALLS\t#4,SHOWFILL\n", pattern, sh.count, label, name)
			case "act":
				b.WriteString("\tCALLS\t#0,SHOWACT\n")
			default:
				log.Fatalf("%s: bad show %q", c.name, sh.kind)
			}
		}
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// prefixLine is an instruction line of a case's routine: a line that
// ends in ":" is a label, in column 1.
func prefixLine(line string, n int) string {
	line = prefix(line, n)
	if strings.HasSuffix(line, ":") {
		return line + "\n"
	}

	return "\t" + line + "\n"
}

// command is lib59.com: it builds and runs the probe, the program's
// output in its own file.
const command = `$ ! LIB59.COM - the Phase 59 probe of LIBRTL's memory routines.
$ ! Written by testdata/lib59/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIB59/OUTPUT=LIB59.LOG
$ !
$ ! LIB59.OUT gets the program's output; LIB59.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ SHOW PROCESS/QUOTAS
$ DEFINE/USER_MODE SYS$OUTPUT LIB59.MLG
$ MACRO/NOLIST LIB59
$ LINK/NOMAP LIB59
$ DEFINE/USER_MODE SYS$OUTPUT LIB59.OUT
$ RUN LIB59
$ SHOW SYMBOL $STATUS
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 59 probe's exchange volume with
! govax. Written by testdata/lib59/gen.go. Run from the repository root:
!
!     govax console < testdata/lib59/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib59-exchange.dsk" /DEVICE=RD51 LIB59
MOUNT/WRITE DUA1 "testdata/disks/lib59-exchange.dsk"
COPY "testdata/lib59/lib59.mar"/HOST DUA1:[000000]LIB59.MAR
COPY "testdata/lib59/lib59.com"/HOST DUA1:[000000]LIB59.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib59")
	all := allCases()

	seen := map[string]bool{}
	for _, c := range all {
		if seen[c.name] {
			log.Fatalf("two cases named %q", c.name)
		}

		seen[c.name] = true
	}

	for name, text := range map[string]string{
		"lib59.mar":    program(all),
		"lib59.com":    command,
		"exchange.cmd": exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("%d cases\n", len(all))
}
