//go:build ignore

// gen.go writes the Phase 83 probe of LIBRTL's STR$ routines (docs/PHASE-83 -
// FORTRAN and VMS.md): str83.mar, a MACRO program that calls every string routine,
// case by case, and prints what each returned, signaled, and stored. It is laid
// out as testdata/lib54's: each case is a routine under a handler that records a
// signaled condition and unwinds, so a status returned shows apart from one
// signaled, and JSB cases load their arguments into R0, R1, ... and show R1 to R6.
// The last cases ask $GETMSG for the facility's message texts. Run this from the
// repository root:
//
//	go run testdata/str83/gen.go
//
// The volume and the procedure that runs it are testdata/for83's.
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
	var b strings.Builder

	fmt.Fprintf(&b, "%s:\t.ASCID\t\"", label)

	for i := 0; i < len(s); i++ {
		if s[i] < 0x80 {
			b.WriteByte(s[i])
		} else {
			fmt.Fprintf(&b, "\"<%d>\"", s[i])
		}
	}

	b.WriteByte('"')

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

// The status codes whose texts the program asks for, by $GETMSG: VMS's STR
// facility has no message table in govax yet.
var messageCodes = []struct {
	name string
	code uint32
}{
	{"DIVBY_ZER", 0x24804c}, {"ERRFREDYN", 0x24807c}, {"FATINTERR", 0x248044}, {"ILLSTRCLA", 0x248054},
	{"ILLSTRPOS", 0x248409}, {"ILLSTRSPE", 0x248411}, {"INSVIRMEM", 0x24806c}, {"INVDELIM", 0x248210},
	{"MATCH", 0x248419}, {"NEGSTRLEN", 0x248401}, {"NOELEM", 0x248218}, {"NOMATCH", 0x248208},
	{"NORMAL", 0x240001}, {"STRIS_INT", 0x24805c}, {"STRTOOLON", 0x248074}, {"TRU", 0x248200},
	{"WRONUMARG", 0x248064},
}

// The destinations a case can have: S a fixed string of n dashes, D an empty
// dynamic string, V a varying string of at most n bytes.
func destS(n int) (string, show) {
	return ascid("%D", strings.Repeat("-", n)), txt("%D")
}

func destD() (string, show) { return dyn("%D"), txt("%D") }

func destV(n int) (string, show) {
	return desc("%D", n, 14, 11, "%V") + "\n%V:\t.WORD\t0\n\t.ASCII\t\"" + strings.Repeat("?", n) + "\"\n\t.WORD\t^X5555", vs("%D")
}

// dests are the three kinds, with short and long fixed ones, by a name's suffix.
func dests() map[string]func() (string, show) {
	return map[string]func() (string, show){
		"S3": func() (string, show) { return destS(3) },
		"S8": func() (string, show) { return destS(8) },
		"D":  destD,
		"V4": func() (string, show) { return destV(4) },
	}
}

var destOrder = []string{"S3", "S8", "D", "V4"}

// long is the data line of a longword, word the data line of a word.
func long(label string, v int) string { return fmt.Sprintf("%s:\t.LONG\t%d", label, v) }
func word(label string, v int) string { return fmt.Sprintf("%s:\t.WORD\t%d", label, v) }

// everyDest makes a case per destination kind of routine(dest, source, more...).
func everyDest(name, routine, source string, data []string, more ...string) []probeCase {
	var out []probeCase

	for _, k := range destOrder {
		d, sh := dests()[k]()
		out = append(out, probeCase{
			name: name + " " + k,
			data: append([]string{d, ascid("%S", source)}, data...),
			call: routine,
			args: append([]string{"%D", "%S"}, more...),
			show: []show{sh},
		})
	}

	return out
}

// to8 is one case into a fixed string of 8: routine(dest, source, more...).
func to8(name, routine, source string, data []string, more ...string) probeCase {
	d, sh := destS(8)

	return probeCase{
		name: name,
		data: append([]string{d, ascid("%S", source)}, data...),
		call: routine,
		args: append([]string{"%D", "%S"}, more...),
		show: []show{sh},
	}
}

// pos is to8 for the routines that take positions by reference.
func pos(name, routine, source string, positions ...int) probeCase {
	var data, more []string

	for i, v := range positions {
		label := fmt.Sprintf("%%P%d", i)
		data = append(data, long(label, v))
		more = append(more, label)
	}

	return to8(name, routine, source, data, more...)
}

// two is routine(a, b), whose value is R0.
func two(name, routine, a, b string) probeCase {
	return probeCase{name: name, data: []string{ascid("%A", a), ascid("%B", b)}, call: routine, args: []string{"%A", "%B"}}
}

// jsb is a JSB case: what it stores in %D, and R1 to R6.
func jsb(name, routine string, data []string, args []string, shows ...show) probeCase {
	return probeCase{name: name + " (JSB)", data: append(data, "%REGS:\t.BLKL\t6"), call: routine, jsb: true, args: args,
		show: append(shows, mem("%REGS", 6))}
}

func buildCases() []probeCase {
	var c []probeCase

	add := func(more ...probeCase) { c = append(c, more...) }

	// STR$COPY_DX and the destination kinds.
	add(everyDest("COPY_DX", "STR$COPY_DX", "HELLO", nil)...)
	add(everyDest("COPY_DX empty", "STR$COPY_DX", "", nil)...)
	add(to8("COPY_DX from VS", "STR$COPY_DX", "x", []string{desc("%T", 10, 14, 11, "%U"), "%U:\t.WORD\t3\n\t.ASCII\t\"ABCDEFGHIJ\""}))
	c[len(c)-1].args = []string{"%D", "%T"}
	add(probeCase{name: "COPY_DX from class 99", data: []string{ascid("%D", "--------"), desc("%S", 3, 14, 99, "%T"), "%T:\t.ASCII\t\"ABC\""},
		call: "STR$COPY_DX", args: []string{"%D", "%S"}, show: []show{txt("%D")}})
	add(probeCase{name: "COPY_DX into class 99", data: []string{ascid("%S", "ABC"), desc("%D", 3, 14, 99, "%T"), "%T:\t.ASCII\t\"---\""},
		call: "STR$COPY_DX", args: []string{"%D", "%S"}, show: []show{mem("%T", 1)}})
	add(probeCase{name: "COPY_DX no args", call: "STR$COPY_DX"})
	add(probeCase{name: "COPY_DX one arg", data: []string{ascid("%D", "---")}, call: "STR$COPY_DX", args: []string{"%D"}})
	add(probeCase{name: "COPY_DX dynamic kept", data: []string{ascid("%S", "ABCDEFGHIJ"), ascid("%T", "XY"), dyn("%D")},
		pre: []string{"PUSHAB\t%S", "PUSHAB\t%D", "CALLS\t#2,G^STR$COPY_DX", "MOVL\t%D+4,SAVEP"},
		call: "STR$COPY_DX", args: []string{"%D", "%T"},
		post: []string{"CLRL\t%Z", "CMPL\t%D+4,SAVEP", "BNEQ\t%NZ", "MOVL\t#1,%Z", "%NZ:"},
		show: []show{txt("%D"), mem("%Z", 1)}})
	c[len(c)-1].data = append(c[len(c)-1].data, "%Z:\t.LONG\t0")

	// STR$COPY_R.
	add(probeCase{name: "COPY_R 4", data: []string{word("%L", 4), "%T:\t.ASCII\t\"ABCDEFGH\"", ascid("%D", "------")},
		call: "STR$COPY_R", args: []string{"%D", "%L", "%T+2"}, show: []show{txt("%D")}})
	add(probeCase{name: "COPY_R 0", data: []string{word("%L", 0), "%T:\t.ASCII\t\"ABCDEFGH\"", dyn("%D")},
		call: "STR$COPY_R", args: []string{"%D", "%L", "%T"}, show: []show{txt("%D")}})
	add(probeCase{name: "COPY_R 8 into 3", data: []string{word("%L", 8), "%T:\t.ASCII\t\"ABCDEFGH\"", ascid("%D", "---")},
		call: "STR$COPY_R", args: []string{"%D", "%L", "%T"}, show: []show{txt("%D")}})

	// Appending and prefixing, on destinations that hold something.
	for _, r := range []struct{ name, routine string }{{"APPEND", "STR$APPEND"}, {"PREFIX", "STR$PREFIX"}} {
		add(probeCase{name: r.name + " dynamic", data: []string{dyn("%D"), ascid("%S", "ABC"), ascid("%T", "XY")},
			pre: []string{"PUSHAB\t%S", "PUSHAB\t%D", "CALLS\t#2,G^STR$COPY_DX"}, call: r.routine, args: []string{"%D", "%T"}, show: []show{txt("%D")}})
		add(probeCase{name: r.name + " empty dynamic", data: []string{dyn("%D"), ascid("%T", "XY")}, call: r.routine, args: []string{"%D", "%T"}, show: []show{txt("%D")}})
		add(probeCase{name: r.name + " fixed fits", data: []string{ascid("%D", "ABCDE"), ascid("%T", "XY")}, call: r.routine, args: []string{"%D", "%T"}, show: []show{txt("%D")}})
		add(probeCase{name: r.name + " varying", data: []string{desc("%D", 6, 14, 11, "%V"), "%V:\t.WORD\t3\n\t.ASCII\t\"ABC???\"", ascid("%T", "XY")}, call: r.routine, args: []string{"%D", "%T"}, show: []show{vs("%D")}})
		add(probeCase{name: r.name + " varying full", data: []string{desc("%D", 4, 14, 11, "%V"), "%V:\t.WORD\t3\n\t.ASCII\t\"ABC?\"", ascid("%T", "XY")}, call: r.routine, args: []string{"%D", "%T"}, show: []show{vs("%D")}})
	}

	// STR$CONCAT.
	add(probeCase{name: "CONCAT 2 D", data: []string{dyn("%D"), ascid("%A", "ABC"), ascid("%B", "DE")}, call: "STR$CONCAT", args: []string{"%D", "%A", "%B"}, show: []show{txt("%D")}})
	add(probeCase{name: "CONCAT 3 S8", data: []string{ascid("%D", "--------"), ascid("%A", "ABC"), ascid("%B", "DE"), ascid("%C", "F")}, call: "STR$CONCAT", args: []string{"%D", "%A", "%B", "%C"}, show: []show{txt("%D")}})
	add(probeCase{name: "CONCAT 3 S3", data: []string{ascid("%D", "---"), ascid("%A", "ABC"), ascid("%B", "DE"), ascid("%C", "F")}, call: "STR$CONCAT", args: []string{"%D", "%A", "%B", "%C"}, show: []show{txt("%D")}})
	add(probeCase{name: "CONCAT 3 V4", data: []string{desc("%D", 4, 14, 11, "%V"), "%V:\t.WORD\t0\n\t.ASCII\t\"????\"", ascid("%A", "ABC"), ascid("%B", "DE"), ascid("%C", "F")}, call: "STR$CONCAT", args: []string{"%D", "%A", "%B", "%C"}, show: []show{vs("%D")}})
	add(probeCase{name: "CONCAT 1", data: []string{dyn("%D"), ascid("%A", "ABC")}, call: "STR$CONCAT", args: []string{"%D", "%A"}, show: []show{txt("%D")}})
	add(probeCase{name: "CONCAT dest only", data: []string{dyn("%D")}, call: "STR$CONCAT", args: []string{"%D"}, show: []show{txt("%D")}})
	add(probeCase{name: "CONCAT no args", call: "STR$CONCAT"})

	// Comparisons, which return -1, 0, or 1.
	for _, r := range []struct{ name, routine string }{{"COMPARE", "STR$COMPARE"}, {"COMPARE_EQL", "STR$COMPARE_EQL"}, {"CASE_BLIND", "STR$CASE_BLIND_COMPARE"}} {
		add(two(r.name+" equal", r.routine, "ABC", "ABC"))
		add(two(r.name+" less", r.routine, "ABC", "ABD"))
		add(two(r.name+" greater", r.routine, "ABD", "ABC"))
		add(two(r.name+" shorter", r.routine, "AB", "ABC"))
		add(two(r.name+" longer", r.routine, "ABC", "AB"))
		add(two(r.name+" blanks", r.routine, "AB ", "AB"))
		add(two(r.name+" blanks 2", r.routine, "AB", "AB  "))
		add(two(r.name+" below blank", r.routine, "AB", "AB\t"))
		add(two(r.name+" case", r.routine, "abc", "ABC"))
		add(two(r.name+" empty", r.routine, "", ""))
		add(two(r.name+" one empty", r.routine, "", "A"))
		add(two(r.name+" high bit", r.routine, "A", "\xc1"))
	}

	for _, r := range []struct {
		name string
		args []string
	}{{"no flags", []string{"%A", "%B"}}, {"flags 0", []string{"%A", "%B", "#0"}}, {"flags 1", []string{"%A", "%B", "#1"}},
		{"flags 1 lang 1", []string{"%A", "%B", "#1", "#1"}}, {"flags 0 lang 2", []string{"%A", "%B", "#0", "#2"}},
		{"flags 0 lang 99", []string{"%A", "%B", "#0", "#99"}}, {"flags 2", []string{"%A", "%B", "#2"}}} {
		add(probeCase{name: "MULTI " + r.name, data: []string{ascid("%A", "abc"), ascid("%B", "ABC")}, call: "STR$COMPARE_MULTI", args: r.args})
		add(probeCase{name: "MULTI less " + r.name, data: []string{ascid("%A", "abc"), ascid("%B", "ABD")}, call: "STR$COMPARE_MULTI", args: r.args})
	}

	add(two("MULTI blanks", "STR$COMPARE_MULTI", "AB ", "AB"))

	// Parts of a string, by position; position rules at both ends.
	for _, p := range [][2]int{{2, 4}, {1, 5}, {0, 2}, {-1, 2}, {4, 2}, {3, 3}, {6, 8}, {5, 9}, {2, 0}} {
		add(pos(fmt.Sprintf("POS_EXTR %d %d", p[0], p[1]), "STR$POS_EXTR", "ABCDE", p[0], p[1]))
	}

	for _, p := range [][2]int{{2, 3}, {1, 5}, {0, 2}, {-1, 3}, {4, 5}, {2, 0}, {2, -1}, {6, 1}, {5, 1}} {
		add(pos(fmt.Sprintf("LEN_EXTR %d %d", p[0], p[1]), "STR$LEN_EXTR", "ABCDE", p[0], p[1]))
	}

	for _, p := range []int{3, 0, -1, 1, 5, 6, 99} {
		add(pos(fmt.Sprintf("LEFT %d", p), "STR$LEFT", "ABCDE", p))
		add(pos(fmt.Sprintf("RIGHT %d", p), "STR$RIGHT", "ABCDE", p))
	}

	for _, p := range [][2]int{{2, 3}, {1, 1}, {0, 2}, {4, 9}, {3, 2}, {6, 6}, {-1, 2}} {
		add(probeCase{name: fmt.Sprintf("REPLACE %d %d", p[0], p[1]), data: []string{dyn("%D"), ascid("%S", "ABCDE"), ascid("%R", "xyz"), long("%P0", p[0]), long("%P1", p[1])},
			call: "STR$REPLACE", args: []string{"%D", "%S", "%P0", "%P1", "%R"}, show: []show{txt("%D")}})
	}

	add(probeCase{name: "REPLACE into S3", data: []string{ascid("%D", "---"), ascid("%S", "ABCDE"), ascid("%R", "xyz"), long("%P0", 2), long("%P1", 3)},
		call: "STR$REPLACE", args: []string{"%D", "%S", "%P0", "%P1", "%R"}, show: []show{txt("%D")}})

	// Searches, whose value is a position or a flag.
	for _, x := range []struct {
		name, s, t string
		start      int
	}{{"found", "ABCDE", "CD", 0}, {"not found", "ABCDE", "XY", 0}, {"first", "ABCAB", "AB", 0}, {"from 2", "ABCAB", "AB", 2}, {"from 5", "ABCAB", "AB", 5},
		{"from 0", "ABCAB", "AB", -2}, {"from past", "ABCAB", "AB", 9}, {"empty sub", "ABC", "", 0}, {"empty source", "", "A", 0}, {"both empty", "", "", 0}} {
		args := []string{"%S", "%T"}
		data := []string{ascid("%S", x.s), ascid("%T", x.t)}

		if x.start != 0 {
			args = append(args, "%P")
			data = append(data, long("%P", x.start))
		}

		add(probeCase{name: "POSITION " + x.name, data: data, call: "STR$POSITION", args: args})
	}

	for _, x := range [][2]string{{"ABCDE", "XDC"}, {"ABCDE", "XY"}, {"", "A"}, {"ABC", ""}, {"ABC", "A"}} {
		add(two("IN_SET "+x[0]+" "+x[1], "STR$FIND_FIRST_IN_SET", x[0], x[1]))
		add(two("NOT_IN "+x[0]+" "+x[1], "STR$FIND_FIRST_NOT_IN_SET", x[0], x[1]))
	}

	add(two("NOT_IN all", "STR$FIND_FIRST_NOT_IN_SET", "AAB", "BA"))

	for _, x := range []struct {
		name string
		subs []string
	}{{"second", []string{"XX", "CD"}}, {"nearest", []string{"DE", "BC"}}, {"tied", []string{"ABC", "AB"}}, {"none", []string{"XX", "YY"}}, {"empty", []string{""}}, {"one", []string{"E"}}} {
		data := []string{ascid("%S", "ABCDE"), "%I:\t.LONG\t^X55555555", "%J:\t.LONG\t^X55555555"}
		args := []string{"%S", "%I", "%J"}

		for i, s := range x.subs {
			l := fmt.Sprintf("%%T%d", i)
			data = append(data, ascid(l, s))
			args = append(args, l)
		}

		add(probeCase{name: "SUBSTRING " + x.name, data: data, call: "STR$FIND_FIRST_SUBSTRING", args: args, show: []show{mem("%I", 2)}})
	}

	for _, x := range [][2]string{{"abc", "abc"}, {"abc", "a*"}, {"abc", "*c"}, {"abc", "a%c"}, {"abc", "a%"}, {"abc", "ABC"}, {"abc", "*"}, {"", "*"}, {"", ""}, {"abc", ""}, {"abcabc", "*b*c"}, {"a*c", "a\\*c"}, {"abc", "%%%"}, {"abc", "%%%%"}} {
		add(two("WILD "+x[0]+" "+x[1], "STR$MATCH_WILD", x[0], x[1]))
	}

	// Building and changing strings.
	for _, x := range []struct {
		name string
		args []string
	}{{"both", []string{"%D", "%C", "%A"}}, {"count", []string{"%D", "%C"}}, {"none", []string{"%D"}}, {"char", []string{"%D", "0", "%A"}}} {
		for _, k := range destOrder {
			d, sh := dests()[k]()
			add(probeCase{name: "DUPL " + x.name + " " + k, data: []string{d, long("%C", 5), "%A:\t.ASCII\t\"Z\""}, call: "STR$DUPL_CHAR", args: x.args, show: []show{sh}})
		}
	}

	add(probeCase{name: "DUPL count 0", data: []string{dyn("%D"), long("%C", 0), "%A:\t.ASCII\t\"Z\""}, call: "STR$DUPL_CHAR", args: []string{"%D", "%C", "%A"}, show: []show{txt("%D")}})
	add(probeCase{name: "DUPL count -1", data: []string{dyn("%D"), long("%C", -1), "%A:\t.ASCII\t\"Z\""}, call: "STR$DUPL_CHAR", args: []string{"%D", "%C", "%A"}, show: []show{txt("%D")}})
	add(probeCase{name: "DUPL count 70000", data: []string{dyn("%D"), long("%C", 70000), "%A:\t.ASCII\t\"Z\""}, call: "STR$DUPL_CHAR", args: []string{"%D", "%C", "%A"}, show: []show{mem("%D", 1)}})

	for _, n := range []int{0, 1, 2, 3, -1} {
		add(probeCase{name: fmt.Sprintf("ELEMENT %d", n), data: []string{dyn("%D"), ascid("%S", "AB,C,,D"), ascid("%T", ","), long("%N", n)},
			call: "STR$ELEMENT", args: []string{"%D", "%N", "%T", "%S"}, show: []show{txt("%D")}})
	}

	add(probeCase{name: "ELEMENT 4", data: []string{dyn("%D"), ascid("%S", "AB,C,,D"), ascid("%T", ","), long("%N", 4)},
		call: "STR$ELEMENT", args: []string{"%D", "%N", "%T", "%S"}, show: []show{txt("%D")}})
	add(probeCase{name: "ELEMENT long delim", data: []string{dyn("%D"), ascid("%S", "AB,C"), ascid("%T", ",,"), long("%N", 0)},
		call: "STR$ELEMENT", args: []string{"%D", "%N", "%T", "%S"}, show: []show{txt("%D")}})
	add(probeCase{name: "ELEMENT empty delim", data: []string{dyn("%D"), ascid("%S", "AB,C"), ascid("%T", ""), long("%N", 0)},
		call: "STR$ELEMENT", args: []string{"%D", "%N", "%T", "%S"}, show: []show{txt("%D")}})
	add(probeCase{name: "ELEMENT into S3", data: []string{ascid("%D", "---"), ascid("%S", "ABCDE,F"), ascid("%T", ","), long("%N", 0)},
		call: "STR$ELEMENT", args: []string{"%D", "%N", "%T", "%S"}, show: []show{txt("%D")}})

	add(everyDest("UPCASE", "STR$UPCASE", "aBc1d", nil)...)
	add(probeCase{name: "UPCASE one arg", data: []string{ascid("%D", "abc")}, call: "STR$UPCASE", args: []string{"%D"}, show: []show{txt("%D")}})
	add(probeCase{name: "UPCASE high bytes", data: []string{ascid("%S", "\xe0\xe9z"), ascid("%D", "---")}, call: "STR$UPCASE", args: []string{"%D", "%S"}, show: []show{mem("%D+4", 0)}})
	c[len(c)-1].post = []string{"MOVL\t%D+4,R0", "MOVL\t(R0),%W"}
	c[len(c)-1].data = append(c[len(c)-1].data, "%W:\t.LONG\t0")
	c[len(c)-1].show = []show{mem("%W", 1)}

	for i, s := range []string{"AB  ", "AB\t ", "AB", "   ", "", " A B "} {
		d, sh := destS(8)
		add(probeCase{name: fmt.Sprintf("TRIM form %d", i+1), data: []string{d, ascid("%S", s), "%L:\t.WORD\t^X5555"}, call: "STR$TRIM", args: []string{"%D", "%S", "%L"}, show: []show{sh, mem("%L", 1)}})
	}

	add(everyDest("TRIM", "STR$TRIM", "ABCDE  ", nil)...)
	add(probeCase{name: "TRIM dest short", data: []string{ascid("%D", "--"), ascid("%S", "ABCD  "), "%L:\t.WORD\t^X5555"}, call: "STR$TRIM", args: []string{"%D", "%S", "%L"}, show: []show{txt("%D"), mem("%L", 1)}})

	for _, x := range []struct{ name, tr, ma string }{{"plain", "xyz", "ABC"}, {"short translate", "x", "ABC"}, {"none match", "xyz", "QRS"}, {"empty match", "xyz", ""}, {"empty translate", "", "ABC"}} {
		add(probeCase{name: "TRANSLATE " + x.name, data: []string{dyn("%D"), ascid("%S", "ABCDA"), ascid("%T", x.tr), ascid("%M", x.ma)},
			call: "STR$TRANSLATE", args: []string{"%D", "%S", "%T", "%M"}, show: []show{txt("%D")}})
	}

	add(probeCase{name: "TRANSLATE S3", data: []string{ascid("%D", "---"), ascid("%S", "ABCDA"), ascid("%T", "xy"), ascid("%M", "AB")},
		call: "STR$TRANSLATE", args: []string{"%D", "%S", "%T", "%M"}, show: []show{txt("%D")}})

	// Descriptors and dynamic strings.
	add(probeCase{name: "ANALYZE S", data: []string{ascid("%S", "ABC"), "%L:\t.WORD\t^X5555", "%P:\t.LONG\t0", "%Z:\t.LONG\t0"}, call: "STR$ANALYZE_SDESC", args: []string{"%S", "%L", "%P"},
		post: []string{"SUBL3\t%S+4,%P,%Z"}, show: []show{mem("%L", 1), mem("%Z", 1)}})
	add(probeCase{name: "ANALYZE VS", data: []string{desc("%S", 10, 14, 11, "%V"), "%V:\t.WORD\t3\n\t.ASCII\t\"ABCDEFGHIJ\"", "%L:\t.WORD\t^X5555", "%P:\t.LONG\t0", "%Z:\t.LONG\t0"}, call: "STR$ANALYZE_SDESC", args: []string{"%S", "%L", "%P"},
		post: []string{"SUBL3\t#%V,%P,%Z"}, show: []show{mem("%L", 1), mem("%Z", 1)}})
	add(probeCase{name: "ANALYZE class 99", data: []string{desc("%S", 10, 14, 99, "%S"), "%L:\t.WORD\t^X5555", "%P:\t.LONG\t0"}, call: "STR$ANALYZE_SDESC", args: []string{"%S", "%L", "%P"}, show: []show{mem("%L", 1)}})
	add(probeCase{name: "GET1_DX 10", data: []string{dyn("%D"), word("%L", 10), "%Z:\t.LONG\t0"}, call: "STR$GET1_DX", args: []string{"%L", "%D"}, post: nonzero("%D+4"), show: []show{mem("%D", 1), mem("%Z", 1)}})
	add(probeCase{name: "GET1_DX 0", data: []string{dyn("%D"), word("%L", 0), "%Z:\t.LONG\t0"}, call: "STR$GET1_DX", args: []string{"%L", "%D"}, post: nonzero("%D+4"), show: []show{mem("%D", 1), mem("%Z", 1)}})
	add(probeCase{name: "GET1_DX class S", data: []string{ascid("%D", "ABC"), word("%L", 10)}, call: "STR$GET1_DX", args: []string{"%L", "%D"}, show: []show{mem("%D", 1)}})
	add(probeCase{name: "FREE1_DX", data: []string{dyn("%D"), word("%L", 10)}, pre: []string{"PUSHAB\t%D", "PUSHAB\t%L", "CALLS\t#2,G^STR$GET1_DX"}, call: "STR$FREE1_DX", args: []string{"%D"}, show: []show{mem("%D", 2)}})
	add(probeCase{name: "FREE1_DX empty", data: []string{dyn("%D")}, call: "STR$FREE1_DX", args: []string{"%D"}, show: []show{mem("%D", 2)}})
	add(probeCase{name: "FREE1_DX class S", data: []string{ascid("%D", "ABC")}, call: "STR$FREE1_DX", args: []string{"%D"}, show: []show{mem("%D", 1)}})

	// The JSB entries, by value, with R1 to R6 shown.
	add(jsb("COPY_DX", "STR$COPY_DX_R8", []string{ascid("%D", "-----"), ascid("%S", "ABC")}, []string{"%D", "%S"}, txt("%D")))
	add(jsb("COPY_R", "STR$COPY_R_R8", []string{ascid("%D", "-----"), "%T:\t.ASCII\t\"ABC\""}, []string{"%D", "#3", "%T"}, txt("%D")))
	add(jsb("DUPL_CHAR", "STR$DUPL_CHARR8", []string{ascid("%D", "-----")}, []string{"%D", "#3", "#^A/Z/"}, txt("%D")))
	add(jsb("LEFT", "STR$LEFT_R8", []string{ascid("%D", "-----"), ascid("%S", "ABCDE")}, []string{"%D", "%S", "#2"}, txt("%D")))
	add(jsb("RIGHT", "STR$RIGHT_R8", []string{ascid("%D", "-----"), ascid("%S", "ABCDE")}, []string{"%D", "%S", "#4"}, txt("%D")))
	add(jsb("LEN_EXTR", "STR$LEN_EXTR_R8", []string{ascid("%D", "-----"), ascid("%S", "ABCDE")}, []string{"%D", "%S", "#2", "#2"}, txt("%D")))
	add(jsb("POS_EXTR", "STR$POS_EXTR_R8", []string{ascid("%D", "-----"), ascid("%S", "ABCDE")}, []string{"%D", "%S", "#2", "#3"}, txt("%D")))
	add(jsb("REPLACE", "STR$REPLACE_R8", []string{dyn("%D"), ascid("%S", "ABCDE"), ascid("%R", "x")}, []string{"%D", "%S", "#2", "#3", "%R"}, txt("%D")))
	add(jsb("POSITION", "STR$POSITION_R6", []string{ascid("%S", "ABCDE"), ascid("%T", "CD")}, []string{"%S", "%T", "#1"}))
	add(jsb("GET1_DX", "STR$GET1_DX_R4", []string{dyn("%D")}, []string{"#7", "%D"}, mem("%D", 1)))
	add(jsb("FREE1_DX", "STR$FREE1_DX_R4", []string{dyn("%D")}, []string{"%D"}, mem("%D", 2)))
	add(jsb("ANALYZE", "STR$ANALYZE_SDESC_R1", []string{ascid("%S", "ABC")}, []string{"%S"}))
	c[len(c)-1].post = []string{"SUBL2\t%S+4,%REGS"}

	// The facility's message texts.
	for _, m := range messageCodes {
		add(probeCase{name: "GETMSG " + m.name, data: []string{"%L:\t.WORD\t0", "%B:\t.BLKB\t132", desc("%D", 132, 14, 1, "%B"), "%F:\t.BLKB\t4"},
			call: "SYS$GETMSG", args: []string{fmt.Sprintf("#^X%X", m.code), "%L", "%D", "#15", "%F"},
			post: []string{"MOVW\t%L,%D"}, show: []show{txt("%D"), mem("%F", 1)}})
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
const programHead = `	.TITLE	STR83	Phase 83 STR$ probe
	.IDENT	/V1.0/

; Written by testdata/str83/gen.go: do not edit. Calls each of LIBRTL's
; STR$ string routines, case by case, each in a routine of its own under
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
	file := filepath.Join("testdata", "str83", "str83.mar")
	if err := os.WriteFile(file, []byte(program()), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(cases), "cases")
}
