//go:build ignore

// gen.go writes the Phase 83 probe (docs/PHASE-83 - FORTRAN and VMS.md, subtask 2).
// The simh VAX has no FORTRAN compiler, so what FORRTL.EXE and LIBRTL.EXE do is asked
// from MACRO, and becomes FTNRTL's and LIBRTL's requirement: only what they do is used,
// never their code. Run this from the repository root:
//
//	go run testdata/for83/gen.go
//
// It writes
//
//	for83.mar    part A: FORRTL's system subroutines, as testdata/lib54's cases
//	for83b.mar   part B: conditions and FORRTL's statements, a case per run
//	for83c.mar   part C: LIBRTL's condition routines, a case per run
//	lnk83.mar, def_opc.mar   which definitions are global; $OPCDEF's values
//	for83.com    builds and runs them all (with testdata/str83's program), and
//	             part D, LIBRARY/TEXT; for83r.com runs a program's cases
//	exchange.cmd, copyout.cmd
//
// In parts B and C a case is one image's run, since a case may end its image. It's
// under HND, whose behavior the case chooses, and the output is
//
//	#n title
//	 =c n=x cond=x arg=x x pc=+x psl=x   a condition: the signal array
//	 =m n=x frame=+x depth=n r0=x r1=x   and the mechanism array
//	 =u                                  the handler's call for an unwind
//	 =what r0=x [r1=x]                   a place the case reached
//	 =value x, =read r0=x |text|         what a statement got
//
// pc is relative to the routine under test, and frame to its FP. A file the case wrote
// is typed by the procedure after the run, with $STATUS.
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

// ---------------------------------------------------------------------------
// Part A: FORRTL's system subroutines, in for83.mar, under the harness above.

func guard(label string) string {
	return label + ":\t.ASCII\t\"------------\"\n\t.LONG\t^X55555555"
}

func long(label string, v int) string { return fmt.Sprintf("%s:\t.LONG\t%d", label, v) }
func word(label string, v int) string { return fmt.Sprintf("%s:\t.WORD\t%d", label, v) }

func partA() []probeCase {
	var c []probeCase

	add := func(more ...probeCase) { c = append(c, more...) }

	// $ASCTIM's text first, so the output says what the date and time were.
	add(probeCase{name: "ASCTIM now", data: []string{"%L:\t.WORD\t0", guard("%B"), desc("%D", 23, 14, 1, "%T"), "%T:\t.BLKB\t23"},
		call: "SYS$ASCTIM", args: []string{"%L", "%D", "0", "#0"}, show: []show{txt("%D")}})

	for _, r := range []struct {
		name, ref, ds string
		sizes         []int
	}{{"DATE", "FOR$DATE", "FOR$DATE_T_DS", []int{9, 5, 12, 0, 1}}, {"TIME", "FOR$TIME", "FOR$TIME_T_DS", []int{8, 4, 12, 0, 1}}} {
		add(probeCase{name: r.name + " by ref", data: []string{guard("%B")}, call: r.ref, args: []string{"%B"}, show: []show{txtAt("%B", 16)}})
		add(probeCase{name: r.name + " by ref no args", call: r.ref})
		add(probeCase{name: r.name + " by ref 0", call: r.ref, args: []string{"0"}})
		add(probeCase{name: r.name + " by ref 2 args", data: []string{guard("%B")}, call: r.ref, args: []string{"%B", "%B"}, show: []show{txtAt("%B", 16)}})

		for _, n := range r.sizes {
			add(probeCase{name: fmt.Sprintf("%s desc %d", r.name, n), data: []string{guard("%B"), desc("%D", n, 14, 1, "%B")}, call: r.ds, args: []string{"%D"}, show: []show{txtAt("%B", 16)}})
		}

		add(probeCase{name: r.name + " desc dynamic", data: []string{dyn("%D")}, call: r.ds, args: []string{"%D"}, show: []show{txt("%D")}})
		add(probeCase{name: r.name + " desc varying", data: []string{desc("%D", 12, 14, 11, "%V"), "%V:\t.WORD\t0\n\t.ASCII\t\"------------\""}, call: r.ds, args: []string{"%D"}, show: []show{txtAt("%V", 14)}})
		add(probeCase{name: r.name + " desc array", data: []string{guard("%B"), desc("%D", 4, 14, 4, "%B") + "\n\t.BYTE\t0,0,0,1\n\t.LONG\t12"}, call: r.ds, args: []string{"%D"}, show: []show{txtAt("%B", 16)}})
		add(probeCase{name: r.name + " desc no args", call: r.ds})
	}

	for _, r := range []struct {
		name, routine string
		form          func(string) string
		size          int
	}{{"IDATE words", "FOR$IDATE", func(l string) string { return l + ":\t.WORD\t^X5555,^X5555" }, 4},
		{"JDATE longs", "FOR$JDATE", func(l string) string { return l + ":\t.LONG\t^X55555555,^X55555555" }, 8}} {
		data := []string{r.form("%I"), r.form("%J"), r.form("%K")}
		add(probeCase{name: r.name, data: data, call: r.routine, args: []string{"%I", "%J", "%K"}, show: []show{mem("%I", 3*r.size/4)}})
		add(probeCase{name: r.name + " no args", call: r.routine})
		add(probeCase{name: r.name + " 2 args", data: data, call: r.routine, args: []string{"%I", "%J"}, show: []show{mem("%I", 3*r.size/4)}})
		add(probeCase{name: r.name + " null second", data: data, call: r.routine, args: []string{"%I", "0", "%K"}, show: []show{mem("%I", 3*r.size/4)}})
	}

	// SECNDS: only what doesn't depend on the time shows: the difference between
	// its value and (now - x), in hundreds of seconds, rounded: 0 or a day's 864.
	for _, x := range []struct {
		name string
		pre  string
	}{{"0", "CLRF\t%X"}, {"now", "MOVF\tNOW,%X"}, {"now+1h", "ADDF3\t#3600.0,NOW,%X"}, {"now-1h", "SUBF3\t#3600.0,NOW,%X"},
		{"90000", "MOVF\t#90000.0,%X"}, {"-100", "MOVF\t#-100.0,%X"}, {"1E7", "MOVF\t#1.0E7,%X"}} {
		add(probeCase{name: "SECNDS " + x.name, data: []string{"%X:\t.LONG\t0", "%R:\t.LONG\t0", "%E:\t.LONG\t0", "%N:\t.LONG\t0"},
			pre: []string{"CALLS\t#0,NOWF", x.pre}, call: "FOR$SECNDS", args: []string{"%X"},
			post: []string{"MOVF\tR0,%R", "SUBF3\t%X,NOW,%E", "SUBF3\t%E,%R,%E", "DIVF2\t#100.0,%E", "CVTRFL\t%E,%N", "CLRL\t(SP)"},
			show: []show{mem("%N", 1)}})
	}

	add(probeCase{name: "SECNDS no args", call: "FOR$SECNDS", post: []string{"CLRL\t(SP)"}})
	add(probeCase{name: "SECNDS null", call: "FOR$SECNDS", args: []string{"0"}, post: []string{"CLRL\t(SP)"}})
	add(probeCase{name: "SECNDS hundredths", data: []string{"%X:\t.LONG\t0", "%N:\t.LONG\t0"},
		pre: []string{"CLRF\t%X"}, call: "FOR$SECNDS", args: []string{"%X"}, post: []string{"MOVL\tR0,%N", "CLRL\t(SP)", "MOVL\t#1,%N"}, show: []show{mem("%N", 1)}})

	// MVBITS: from m's bits i up, len of them, to n's bits j up.
	for _, x := range [][5]int{{0x0F, 1, 3, 0, 4}, {-1, 0, 8, 0, 8}, {0x12345678, 8, 8, -1, 16}, {0x12345678, 0, 0, 7, 0}, {-1, 15, 1, 0, 15},
		{1, 0, 1, 0, 15}, {-1, 0, 16, 0, 0}, {-1, 0, 17, 0, 0}, {-1, 16, 1, 0, 0}, {-1, 0, 1, 0, 16}, {-1, 0, -1, 0, 0}, {-1, -1, 1, 0, 0}, {-1, 0, 1, 0, -1}} {
		add(probeCase{name: fmt.Sprintf("IMVBITS %X %d %d %X %d", uint16(x[0]), x[1], x[2], uint16(x[3]), x[4]),
			data: []string{word("%M", int(int16(x[0]))), word("%I", x[1]), word("%L", x[2]), word("%N", int(int16(x[3]))), word("%G", 0x5555), word("%J", x[4])},
			call: "FOR$IMVBITS", args: []string{"%M", "%I", "%L", "%N", "%J"}, show: []show{mem("%N", 1)}})
	}

	for _, x := range [][5]int{{0x0F, 1, 3, 0, 4}, {-1, 0, 8, 0, 8}, {0x12345678, 8, 8, -1, 16}, {0x12345678, 0, 0, 7, 0}, {-1, 31, 1, 0, 31},
		{1, 0, 1, 0, 31}, {-1, 0, 32, 0, 0}, {0x12345678, 0, 32, 0, 0}, {-1, 0, 33, 0, 0}, {-1, 32, 1, 0, 0}, {-1, 0, 1, 0, 32}, {-1, 0, -1, 0, 0}, {-1, -1, 1, 0, 0}, {-1, 0, 1, 0, -1}} {
		add(probeCase{name: fmt.Sprintf("JMVBITS %X %d %d %X %d", uint32(x[0]), x[1], x[2], uint32(x[3]), x[4]),
			data: []string{long("%M", x[0]), long("%I", x[1]), long("%L", x[2]), long("%N", x[3]), long("%G", 0x55555555), long("%J", x[4])},
			call: "FOR$JMVBITS", args: []string{"%M", "%I", "%L", "%N", "%J"}, show: []show{mem("%N", 2)}})
	}

	add(probeCase{name: "JMVBITS same place", data: []string{long("%M", 0x000000FF), long("%I", 0), long("%L", 8), long("%J", 4)},
		call: "FOR$JMVBITS", args: []string{"%M", "%I", "%L", "%M", "%J"}, show: []show{mem("%M", 1)}})
	add(probeCase{name: "JMVBITS no args", call: "FOR$JMVBITS"})
	add(probeCase{name: "JMVBITS 4 args", data: []string{long("%M", 1), long("%I", 0), long("%L", 1), long("%N", 0)},
		call: "FOR$JMVBITS", args: []string{"%M", "%I", "%L", "%N"}, show: []show{mem("%N", 1)}})

	// RAN is MTH$RANDOM's generator (D.4.7): the sequence and the seed's changes.
	for _, seed := range []int{1, 12345, -1, 0} {
		add(probeCase{name: fmt.Sprintf("RANDOM seed %d", seed), data: []string{long("%S", seed), "%R:\t.LONG\t0"}, call: "MTH$RANDOM", args: []string{"%S"},
			post: []string{"MOVL\tR0,%R"}, show: []show{mem("%S", 1), mem("%R", 1)}})
	}

	return c
}

// txtAt shows n bytes at a label as a string.
func txtAt(label string, n int) show { return show{"raw", label, n} }

// ---------------------------------------------------------------------------
// Parts B and C: a program of many cases, one per run, chosen by a foreign
// command's parameter, since a case may end its image. Each is under a handler
// whose behavior a case chooses.

const (
	mContinue = iota
	mResignal
	mUnwind
	mUnwindDepth
	mUnwindPC
	mSetR0Continue
	mSetR0Unwind
	mHook
	mFixup
	mFixupNew
	mSigToRet
	mSigToStop
)

const (
	kErr     = 3
	kFile    = 14
	kStatus  = 15
	kUnit    = 1
	mImmWord = 2
	mDescRef = 7
)

type runProg struct {
	name  string
	data  strings.Builder
	code  strings.Builder
	cases []string
	body  *strings.Builder
	texts map[string]string
	nsub  int
	nlab  int
}

func newRun(name string) *runProg { return &runProg{name: name, texts: map[string]string{}} }

// text is a label for a .ASCID of s.
func (p *runProg) text(s string) string {
	if l, ok := p.texts[s]; ok {
		return l
	}

	l := fmt.Sprintf("T%d", len(p.texts))
	p.texts[s] = l
	fmt.Fprintf(&p.data, "%s:\t.ASCID\t~%s~\n", l, s)

	return l
}

func (p *runProg) emit(format string, a ...any) { fmt.Fprintf(p.body, "\t"+format+"\n", a...) }

func (p *runProg) dataf(format string, a ...any) { fmt.Fprintf(&p.data, format+"\n", a...) }

// kase adds a case: the routine the driver calls for its number.
func (p *runProg) kase(title string, f func()) {
	n := len(p.cases) + 1
	p.cases = append(p.cases, title)
	fmt.Fprintf(&p.code, "; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n", n, title, n)
	p.body = &p.code
	p.emit("PUSHAB\t%s", p.text(fmt.Sprintf("#%d %s", n, title)))
	p.emit("CALLS\t#1,G^LIB$PUT_OUTPUT")
	f()
	p.code.WriteString("\tRET\n")
}

// sub runs f in a routine of its own, whose handler has the given mode, so an
// unwind ends only it. It then says it came back.
func (p *runProg) sub(mode int, f func()) {
	id := p.nsub
	p.nsub++
	prev := p.body

	var b strings.Builder

	p.body = &b
	f()
	p.body = prev
	fmt.Fprintf(&p.code, "\tBRW\tB%d\n\t.ENTRY\tS%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHND,(FP)\n\tMOVL\tFP,SFP\n\tMOVAB\tS%d,SADDR\n%s\tRET\nB%d:\n", id, id, id, b.String(), id)
	p.emit("MOVL\t#%d,HMODE", mode)
	p.emit("CALLS\t#0,L^S%d", id)
	p.said("back")
}

// said prints what and R0; both prints R1 too, for a case that set it up.
func (p *runProg) said(what string) {
	p.emit("PUSHL\tR0")
	p.emit("PUSHAB\t%s", p.text(what))
	p.emit("CALLS\t#2,L^SHOW")
}

func (p *runProg) both(what string) {
	p.emit("PUSHL\tR1")
	p.emit("PUSHL\tR0")
	p.emit("PUSHAB\t%s", p.text(what))
	p.emit("CALLS\t#3,L^SHOW2")
}

// call calls a routine with operands: "&x" is x's address, anything else pushed
// as it stands.
func (p *runProg) call(routine string, args ...string) {
	for i := len(args) - 1; i >= 0; i-- {
		if strings.HasPrefix(args[i], "&") {
			p.emit("PUSHAB\t%s", args[i][1:])
		} else {
			p.emit("PUSHL\t%s", args[i])
		}
	}

	p.emit("CALLS\t#%d,G^%s", len(args), routine)
}

// sentinels sets R0 and R1 before a call, to see what the call leaves there.
func (p *runProg) sentinels() {
	p.emit("MOVL\t#^X11111111,R0")
	p.emit("MOVL\t#^X22222222,R1")
}

// open opens unit u on file for what status says, with no ERR=: a failure is
// signaled, as a program with no specifier gets it.
func (p *runProg) open(u int, file, status string) {
	p.call("FOR$OPEN", fmt.Sprintf("#^X%04X%02X%02X", u, mImmWord, kUnit),
		fmt.Sprintf("#^X%02X%02X", mDescRef, kFile), "&"+p.text(file),
		fmt.Sprintf("#^X%02X%02X", mDescRef, kStatus), "&"+p.text(status))
}

func (p *runProg) closeU(u int) {
	p.call("FOR$CLOSE", fmt.Sprintf("#^X%04X%02X%02X", u, mImmWord, kUnit))
}

// write is a whole formatted WRITE of s on unit u.
func (p *runProg) write(u int, s string) {
	p.call("FOR$WRITE_SO", fmt.Sprintf("#%d", u), "&FMTA")
	p.call("FOR$IO_T_DS", "&"+p.text(s))
	p.call("FOR$IO_END")
}

// read is a whole formatted READ on unit u, showing what it got.
func (p *runProg) read(u int) {
	p.call("FOR$READ_SO", fmt.Sprintf("#%d", u), "&FMTA")
	p.call("FOR$IO_T_DS", "&RBD")
	p.call("FOR$IO_END")
	p.emit("PUSHL\tR0")
	p.emit("CALLS\t#1,L^SHOWRB")
}

// forrtlCases is part B, on VMS's FORRTL.
func forrtlCases() *runProg {
	p := newRun("FOR83B")

	const (
		data = "FOR83D.DAT"
		w    = "FOR83W.DAT"
		x    = "FOR83X.DAT"
	)

	errors := []struct {
		name string
		f    func()
	}{
		{"FILNOTFOU", func() { p.open(12, "NOSUCH.DAT", "OLD") }},
		{"ENDDURREA", func() {
			p.open(12, "FOR83E.DAT", "OLD")
			p.read(12)
		}},
		{"INPCONERR", func() {
			p.open(12, data, "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTI4")
			p.call("FOR$IO_L_R", "&LVAL")
			p.call("FOR$IO_END")
			p.emit("PUSHL\tLVAL")
			p.emit("CALLS\t#1,L^SHOWV")
		}},
		{"OUTCONERR", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SO", "#12", "&FMTI1")
			p.call("FOR$IO_L_V", "#99")
			p.call("FOR$IO_END")
		}},
		{"FORVARMIS", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SO", "#12", "&FMTI4")
			p.call("FOR$IO_T_DS", "&"+p.text("ABC"))
			p.call("FOR$IO_END")
		}},
		{"INVLOGUNI", func() { p.write(200, "X") }},
		{"SYNERRFOR", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SO", "#12", "&FMTBAD")
			p.call("FOR$IO_T_DS", "&"+p.text("X"))
			p.call("FOR$IO_END")
		}},
		{"INFFORLOO", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SO", "#12", "&FMTLOOP")
			p.call("FOR$IO_T_DS", "&"+p.text("X"))
			p.call("FOR$IO_END")
		}},
		{"REWRITE no open", func() { p.call("FOR$REWIND", "#17"); p.call("FOR$REWRITE_SO", "#17", "&FMTA"); p.call("FOR$IO_END") }},
	}

	// Each error under each thing a handler can do. An error's next statement says
	// whether the unit is still good.
	for _, e := range errors {
		for _, m := range []struct {
			name string
			mode int
		}{{"continue", mContinue}, {"unwind", mUnwind}, {"SIG_TO_RET", mSigToRet}} {
			e, m := e, m
			p.kase(e.name+" "+m.name, func() {
				p.sub(m.mode, func() {
					e.f()
					p.said("after statement")
				})
				p.call("FOR$INQUIRE", fmt.Sprintf("#^X%04X%02X%02X", 12, mImmWord, kUnit), "#^X051F", "&LVAL")
				p.emit("PUSHL\tLVAL")
				p.emit("CALLS\t#1,L^SHOWV")
				p.write(13, "unit 13 still works")
				p.closeU(12)
				p.closeU(13)
			})
		}
	}

	// A condition between a statement's items, on a statement of each kind, and
	// what the unit then holds; the procedure TYPEs its file.
	stmts := []struct {
		name string
		f    func()
	}{
		{"formatted WRITE", func() {
			p.open(12, w, "NEW")
			p.write(12, "before")
			p.call("FOR$WRITE_SO", "#12", "&FMTAA")
			p.call("FOR$IO_T_DS", "&"+p.text("partial"))
		}},
		{"list WRITE", func() {
			p.open(12, w, "NEW")
			p.write(12, "before")
			p.call("FOR$WRITE_SL", "#12")
			p.call("FOR$IO_T_DS", "&"+p.text("partial"))
		}},
		{"unformatted WRITE", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SU", "#14")
			p.call("FOR$IO_T_DS", "&"+p.text("partial"))
		}},
		{"formatted READ", func() {
			p.open(12, data, "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTA")
			p.call("FOR$IO_T_DS", "&RBD")
		}},
	}

	for _, s := range stmts {
		s := s

		p.kase(s.name+": signal, unwind, next statements", func() {
			p.sub(mUnwind, func() {
				s.f()
				p.call("LIB$SIGNAL", "#^X08018003")
				p.said("not after unwind")
			})
			if strings.Contains(s.name, "READ") {
				p.read(12)
				p.read(12)
			} else {
				p.write(12, "after")
			}
			p.closeU(12)
		})
		p.kase(s.name+": signal, continue, end", func() {
			p.sub(mContinue, func() {
				s.f()
				p.call("LIB$SIGNAL", "#^X08018003")
				p.said("continued")
				p.call("FOR$IO_END")
				p.said("IO_END")
			})
			p.closeU(12)
		})
	}

	// I/O from a handler or an AST during a statement.
	hooks := []struct {
		name string
		f    func()
	}{
		{"same unit", func() { p.write(12, "inner") }},
		{"another unit", func() { p.write(13, "inner"); p.closeU(13) }},
		{"internal file", func() {
			p.call("FOR$WRITE_IO", "&IFD", "&FMTA")
			p.call("FOR$IO_T_DS", "&"+p.text("inner"))
			p.call("FOR$IO_END")
		}},
		{"unit 6", func() { p.write(6, "inner on 6") }},
		{"CLOSE of the unit", func() { p.closeU(12) }},
	}

	for _, h := range hooks {
		h := h
		hook := fmt.Sprintf("H%d", p.nlab)
		p.nlab++
		fmt.Fprintf(&p.code, "\t.ENTRY\t%s,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tINNER,(FP)\n", hook)
		p.body = &p.code
		h.f()
		p.said("hook's statement")
		p.code.WriteString("\tRET\n")

		for _, stmt := range []string{"handler", "AST"} {
			stmt := stmt

			p.kase(stmt+" with "+h.name+" during a WRITE", func() {
				p.sub(mHook, func() {
					p.emit("MOVAB\t%s,HOOK", hook)
					p.open(12, w, "NEW")
					p.call("FOR$WRITE_SO", "#12", "&FMTAA")
					p.call("FOR$IO_T_DS", "&"+p.text("first"))

					if stmt == "handler" {
						p.call("LIB$SIGNAL", "#^X08018003")
					} else {
						p.call("SYS$DCLAST", "#ASTHOOK", "#0", "#0")
					}

					p.said("after")
					p.call("FOR$IO_T_DS", "&"+p.text("second"))
					p.call("FOR$IO_END")
					p.said("IO_END")
				})
				p.closeU(12)
			})
		}
	}

	// AST statements that aren't in anything: control.
	p.kase("AST with a whole WRITE, none in progress", func() {
		p.sub(mContinue, func() {
			p.open(12, w, "NEW")
			p.emit("MOVAB\tH0,HOOK")
			p.call("SYS$DCLAST", "#ASTHOOK", "#0", "#0")
			p.said("after")
		})
		p.closeU(12)
	})

	// Ending the image: FOR$EXIT's forms, and what happens to open units.
	endings := []struct {
		name string
		f    func()
	}{
		{"EXIT no args", func() { p.call("FOR$EXIT") }},
		{"EXIT 3", func() { p.call("FOR$EXIT", "&LTHREE") }},
		{"EXIT 2", func() { p.call("FOR$EXIT", "&LTWO") }},
		{"EXIT 0", func() { p.call("FOR$EXIT", "&LZERO") }},
		{"EXIT null", func() { p.call("FOR$EXIT", "#0") }},
		{"EXIT_W 3", func() { p.call("FOR$EXIT_W", "&WTHREE") }},
		{"EXIT_W -1", func() { p.call("FOR$EXIT_W", "&WMINUS") }},
		{"EXIT_W no args", func() { p.call("FOR$EXIT_W") }},
		{"STOP", func() { p.call("FOR$STOP") }},
		{"plain RET", func() {}},
		{"SYS$EXIT", func() { p.call("SYS$EXIT", "#3") }},
	}

	for _, e := range endings {
		e := e

		p.kase(e.name+", a unit open", func() {
			p.sub(mContinue, func() {
				p.open(12, w, "NEW")
				p.write(12, "record")
				e.f()
				p.said("returned")
			})
		})
	}

	// Exit handlers and FORRTL's: which runs first decides whether a unit's still
	// open in ours. Declared before the first OPEN, and after.
	for _, order := range []string{"before", "after"} {
		for _, act := range []string{"WRITE to it", "WRITE to a new unit", "only"} {
			order, act := order, act

			p.kase(fmt.Sprintf("exit handler declared %s the OPEN, %s", order, act), func() {
				switch act {
				case "WRITE to it":
					p.emit("MOVL\t#1,XMODE")
				case "WRITE to a new unit":
					p.emit("MOVL\t#2,XMODE")
				}

				if order == "before" {
					p.call("SYS$DCLEXH", "&XBLK")
				}

				p.sub(mContinue, func() {
					p.open(12, w, "NEW")
					p.write(12, "from the program")
				})

				if order == "after" {
					p.call("SYS$DCLEXH", "&XBLK")
				}
			})
		}
	}

	p.kase("a severe error ends the image, exit handler writes", func() {
		p.emit("MOVL\t#2,XMODE")
		p.open(12, w, "NEW")
		p.call("SYS$DCLEXH", "&XBLK")
		p.open(14, "NOSUCH.DAT", "OLD")
	})

	return p
}

// goneCases is a third round on FORRTL, for Phase 83's subtask 7, which needs no
// VAX either: what's left of a statement a handler unwound from. A statement never
// reaches its FOR$IO_END then, so a later statement or item has to tell that it's
// gone. It may start where the gone statement's frame was, by a different call or
// the same one, and it may be deeper than that frame, or inside an outer statement.
func goneCases() *runProg {
	p := newRun("FOR83G")

	const w = "FOR83W.DAT"

	// DEEP is three calls down, its statement on the unit in R2's place; each entry
	// saves what the case's own routine saves, so that frames coincide.
	entry := "^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>"
	p.body = &p.code
	fmt.Fprintf(&p.code, "\t.ENTRY\tDEEP,%s\n\tCALLS\t#0,DEEP2\n\tRET\n\t.ENTRY\tDEEP2,%s\n\tCALLS\t#0,DEEP3\n\tRET\n\t.ENTRY\tDEEP3,%s\n", entry, entry, entry)
	p.call("FOR$WRITE_SO", "UNIT", "&FMTA")
	p.call("FOR$IO_T_DS", "&"+p.text("deep"))
	p.call("FOR$IO_END")
	p.said("deep statement")
	p.code.WriteString("\tRET\n")

	// begun is a statement on unit u, an item in, that a signal then leaves.
	begun := func(u int) {
		p.call("FOR$WRITE_SO", fmt.Sprintf("#%d", u), "&FMTAA")
		p.call("FOR$IO_T_DS", "&"+p.text("gone"))
		p.call("LIB$SIGNAL", "#^X08018003")
		p.said("NOT REACHED")
	}

	deep := func(u int) {
		p.emit("MOVL\t#%d,UNIT", u)
		p.emit("CALLS\t#0,DEEP")
	}

	// AGAIN is called twice from one place: the second call's frame is the first's.
	fmt.Fprintf(&p.code, "\t.ENTRY\tAGAIN,%s\n\tMOVAB\tHND,(FP)\n\tMOVL\tFP,SFP\n\tMOVAB\tAGAIN,SADDR\n\tTSTL\tTURN\n\tBNEQ\t10$\n", entry)
	begun(12)
	p.code.WriteString("10$:\tMOVL\t#12,UNIT\n\tCALLS\t#0,DEEP\n\tRET\n")

	p.kase("gone at depth 1, then the same unit from deeper", func() {
		p.open(12, w, "NEW")
		p.sub(mUnwind, func() { begun(12) })
		deep(12)
		p.closeU(12)
	})

	p.kase("gone, then the same unit from the same routine called again", func() {
		p.open(12, w, "NEW")
		p.emit("MOVL\t#%d,HMODE", mUnwind)
		p.emit("CLRL\tTURN")
		p.emit("CALLS\t#0,AGAIN")
		p.said("first call")
		p.emit("MOVL\t#1,TURN")
		p.emit("CALLS\t#0,AGAIN")
		p.said("second call")
		p.closeU(12)
	})

	p.kase("gone, then the same unit from the frame above", func() {
		p.open(12, w, "NEW")
		p.sub(mUnwind, func() { begun(12) })
		p.write(12, "above")
		p.said("above")
		p.closeU(12)
	})

	p.kase("gone inside an outer statement, which goes on", func() {
		p.open(12, w, "NEW")
		p.call("FOR$WRITE_SO", "#12", "&FMTAA")
		p.call("FOR$IO_T_DS", "&"+p.text("outer"))
		p.sub(mUnwind, func() { begun(13) })
		p.call("FOR$IO_T_DS", "&"+p.text("second"))
		p.call("FOR$IO_END")
		p.said("outer's end")
		p.closeU(12)
		p.closeU(13)
	})

	p.kase("gone inside an outer statement, then its unit from deeper", func() {
		p.open(12, w, "NEW")
		p.call("FOR$WRITE_SO", "#12", "&FMTAA")
		p.call("FOR$IO_T_DS", "&"+p.text("outer"))
		p.sub(mUnwind, func() { begun(13) })
		deep(13)
		p.call("FOR$IO_T_DS", "&"+p.text("second"))
		p.call("FOR$IO_END")
		p.said("outer's end")
		p.closeU(12)
		p.closeU(13)
	})

	// The controls: nothing gone, so these are recursive.
	p.kase("live, then its unit from deeper", func() {
		p.emit("MOVAB\tINNER,(FP)")
		p.open(12, w, "NEW")
		p.call("FOR$WRITE_SO", "#12", "&FMTAA")
		p.call("FOR$IO_T_DS", "&"+p.text("outer"))
		deep(12)
		p.said("NOT REACHED")
	})

	p.kase("gone, then called again with nothing between", func() {
		p.open(12, w, "NEW")
		p.emit("MOVL\t#%d,HMODE", mUnwind)
		p.emit("CLRL\tTURN")
		p.emit("CALLS\t#0,AGAIN")
		p.emit("MOVL\t#1,TURN")
		p.emit("CALLS\t#0,AGAIN")
		p.said("second call")
		p.closeU(12)
	})

	// An AST's statement, gone: the interrupted statement goes on.
	hook := "GONEAST"
	fmt.Fprintf(&p.code, "\t.ENTRY\t%s,%s\n", hook, entry)
	p.body = &p.code
	p.sub(mUnwind, func() { begun(13) })
	p.code.WriteString("\tRET\n")

	p.kase("gone in an AST, during a statement", func() {
		p.emit("MOVAB\t%s,HOOK", hook)
		p.open(12, w, "NEW")
		p.call("FOR$WRITE_SO", "#12", "&FMTAA")
		p.call("FOR$IO_T_DS", "&"+p.text("outer"))
		p.call("SYS$DCLAST", "#ASTHOOK", "#0", "#0")
		p.said("after")
		p.call("FOR$IO_T_DS", "&"+p.text("second"))
		p.call("FOR$IO_END")
		p.said("outer's end")
		p.closeU(12)
		p.closeU(13)
	})

	// Whether unit 6's first use is what declares FORRTL's exit handler: if so, this
	// one runs first, and unit 12 is still open in it.
	p.kase("exit handler declared after a WRITE to unit 6, before the OPEN", func() {
		p.emit("MOVL\t#1,XMODE")
		p.write(6, " on 6")
		p.call("SYS$DCLEXH", "&XBLK")
		p.open(12, w, "NEW")
		p.write(12, "from the program")
	})

	p.dataf("UNIT:\t.LONG\t0\nTURN:\t.LONG\t0")

	return p
}

// continueCases is a second round on FORRTL for Phase 83's subtask 6, which
// needs no VAX: whether FORRTL goes on from a condition is in FORRTL's own
// code, so it runs over govax. Each statement errs under a handler that
// continues; if the image gets to "=after statement", FORRTL went on, and the
// values show what the statement did with its items.
func continueCases() *runProg {
	p := newRun("FOR83E")

	p.dataf("LVAL2:\t.LONG\t^X55555555\nLONEF:\t.FLOAT\t1.0\nISHORT:\t.WORD\t4\n\t.BYTE\t14,1\n\t.ADDRESS IBUF")
	p.dataf("FMTII:\t.ASCIZ\t~(I4,1X,I4)~\nFMTII2:\t.ASCIZ\t~(I4,I1)~\nFMTFI:\t.ASCIZ\t~(F4.0,1X,I4)~\nFMTOI:\t.ASCIZ\t~(O4,1X,I4)~\nFMTZI:\t.ASCIZ\t~(Z4,1X,I4)~")
	p.dataf("FMTLI:\t.ASCIZ\t~(L4,1X,I4)~\nFMTI4A:\t.ASCIZ\t~(I4,A)~\nFMTI4I:\t.ASCIZ\t~(I4,I4)~\nFMTLONG: .ASCIZ\t~(140X,A,A)~\nFMTEND:\t.ASCIZ\t~(I4)~")

	const (
		bad  = "FOR83F.DAT"
		long = "FOR83G.DAT"
		w    = "FOR83W.DAT"
	)

	// values shows LVAL and LVAL2 after a statement that sets them.
	values := func() {
		p.said("after statement")
		p.emit("PUSHL\tLVAL")
		p.emit("CALLS\t#1,L^SHOWV")
		p.emit("PUSHL\tLVAL2")
		p.emit("CALLS\t#1,L^SHOWV")
	}

	read := func(begin []string, items ...string) func() {
		return func() {
			p.open(12, bad, "OLD")
			p.call(begin[0], begin[1:]...)

			for _, item := range items {
				p.call(strings.Fields(item)[0], strings.Fields(item)[1])
			}

			p.call("FOR$IO_END")
			values()
		}
	}

	fmtRead := func(format string, items ...string) func() {
		return read([]string{"FOR$READ_SO", "#12", "&" + format}, items...)
	}

	for _, c := range []struct {
		name string
		f    func()
	}{
		{"INPCONERR I, then an item", fmtRead("FMTII", "FOR$IO_L_R &LVAL", "FOR$IO_L_R &LVAL2")},
		{"INPCONERR I, last item", fmtRead("FMTII", "FOR$IO_L_R &LVAL2", "FOR$IO_L_R &LVAL")},
		{"INPCONERR F, then an item", fmtRead("FMTFI", "FOR$IO_F_R &LVAL", "FOR$IO_L_R &LVAL2")},
		{"INPCONERR O, then an item", fmtRead("FMTOI", "FOR$IO_L_R &LVAL", "FOR$IO_L_R &LVAL2")},
		{"INPCONERR O, a digit too large, then an item", func() {
			p.open(12, "FOR83H.DAT", "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTOI")
			p.call("FOR$IO_L_R", "&LVAL")
			p.call("FOR$IO_L_R", "&LVAL2")
			p.call("FOR$IO_END")
			values()
		}},
		{"INPCONERR Z, a letter, then an item", func() {
			p.open(12, bad, "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTZI")
			p.call("FOR$IO_L_R", "&LVAL")
			p.call("FOR$IO_L_R", "&LVAL2")
			p.call("FOR$IO_END")
			values()
		}},
		{"INPCONERR L, then an item", fmtRead("FMTLI", "FOR$IO_L_R &LVAL", "FOR$IO_L_R &LVAL2")},
		{"INPCONERR twice in a record", fmtRead("FMTII2", "FOR$IO_L_R &LVAL", "FOR$IO_L_R &LVAL2")},
		{"list-directed, a bad integer, then an item", read([]string{"FOR$READ_SL", "#12"}, "FOR$IO_L_R &LVAL", "FOR$IO_L_R &LVAL2")},
		{"FORVARMIS on output", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SO", "#12", "&FMTI4A")
			p.call("FOR$IO_F_R", "&LONEF")
			p.call("FOR$IO_T_DS", "&"+p.text("NEXT"))
			p.call("FOR$IO_END")
			p.said("after statement")
		}},
		{"FORVARMIS on input", func() {
			p.open(12, bad, "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTI4I")
			p.call("FOR$IO_F_R", "&LVAL")
			p.call("FOR$IO_L_R", "&LVAL2")
			p.call("FOR$IO_END")
			values()
		}},
		{"FORVARMIS on input, a field read", func() {
			p.open(12, "FOR83H.DAT", "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTI4I")
			p.call("FOR$IO_F_R", "&LVAL")
			p.call("FOR$IO_END")
			values()
		}},
		{"FORVARMIS on input, an integer under F", func() {
			p.open(12, bad, "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTFI")
			p.call("FOR$IO_L_R", "&LVAL")
			p.call("FOR$IO_L_R", "&LVAL2")
			p.call("FOR$IO_END")
			values()
		}},
		{"OUTSTAOVE on a unit", func() {
			p.open(12, w, "NEW")
			p.call("FOR$WRITE_SO", "#12", "&FMTLONG")
			p.call("FOR$IO_T_DS", "&"+p.text("PAST"))
			p.call("FOR$IO_T_DS", "&"+p.text("NEXT"))
			p.call("FOR$IO_END")
			p.said("after statement")
		}},
		{"OUTSTAOVE in an internal record", func() {
			p.call("FOR$WRITE_IO", "&ISHORT", "&FMTI4I")
			p.call("FOR$IO_L_V", "#1")
			p.call("FOR$IO_L_V", "#2")
			p.call("FOR$IO_END")
			p.said("after statement")
		}},
		{"INPRECTOO", func() {
			p.open(12, long, "OLD")
			p.call("FOR$READ_SO", "#12", "&FMTA")
			p.call("FOR$IO_T_DS", "&RBD")
			p.call("FOR$IO_END")
			p.emit("PUSHL\tR0")
			p.emit("CALLS\t#1,L^SHOWRB")
			p.said("after statement")
		}},
		{"ERRDURWRI, a file opened to read", func() {
			p.open(12, bad, "OLD")
			p.call("FOR$WRITE_SO", "#12", "&FMTA")
			p.call("FOR$IO_T_DS", "&"+p.text("X"))
			p.call("FOR$IO_END")
			p.said("after statement")
		}},
		{"ERR with no handler's choice: an INPCONERR at FOR$IO_END", fmtRead("FMTEND")},
	} {
		c := c

		p.kase(c.name, func() {
			p.emit("MOVL\t#^X55555555,LVAL")
			p.emit("MOVL\t#^X66666666,LVAL2")
			p.sub(mContinue, c.f)
			p.closeU(12)
		})
	}

	return p
}

// cond is a condition with facility 0x0401, facility-specific, message n, and the
// severity in its low 3 bits.
func cond(n, sev int) string { return fmt.Sprintf("#^X%08X", 0x08018000+n*8+sev) }

// librtlCases is part C: LIBRTL's condition routines, with no FORRTL.
func librtlCases() *runProg {
	p := newRun("FOR83C")

	// LIB$SIGNAL and what the handler sees and leaves.
	for _, c := range []struct {
		name string
		mode int
		args []string
	}{
		{"SIGNAL continue", mContinue, []string{cond(0, 3)}},
		{"SIGNAL error continue", mContinue, []string{cond(0, 2)}},
		{"SIGNAL severe continue", mContinue, []string{cond(0, 4)}},
		{"SIGNAL warning continue", mContinue, []string{cond(0, 0)}},
		{"SIGNAL success continue", mContinue, []string{cond(0, 1)}},
		{"SIGNAL with FAO args", mContinue, []string{cond(1, 2), "#2", "#7", "#^XABCD"}},
		{"SIGNAL two conditions", mContinue, []string{cond(1, 2), "#0", cond(2, 2), "#0"}},
		{"SIGNAL handler sets R0 R1", mSetR0Continue, []string{cond(0, 3)}},
		{"SIGNAL unwind", mUnwind, []string{cond(0, 2)}},
		{"SIGNAL unwind depth", mUnwindDepth, []string{cond(0, 2)}},
		{"SIGNAL unwind new PC", mUnwindPC, []string{cond(0, 2)}},
		{"SIGNAL unwind R0 set", mSetR0Unwind, []string{cond(0, 2)}},
		{"SIGNAL system code", mContinue, []string{"#^X0C"}},
		{"STOP unwind", mUnwind, nil},
		{"STOP unwind depth", mUnwindDepth, nil},
		{"STOP severity rule, unwind", mUnwind, nil},
		{"STOP with FAO args, unwind", mUnwind, nil},
		{"STOP continue", mContinue, nil},
		{"STOP handler sets R0, continue", mSetR0Continue, nil},
	} {
		c := c
		label := fmt.Sprintf("NP%d", p.nlab)
		p.nlab++

		p.kase(c.name, func() {
			p.sub(c.mode, func() {
				switch {
				case c.name == "STOP severity rule, unwind":
					p.call("LIB$STOP", cond(0, 1))
				case c.name == "STOP with FAO args, unwind":
					p.call("LIB$STOP", cond(1, 2), "#1", "#7")
				case strings.HasPrefix(c.name, "STOP"):
					p.call("LIB$STOP", cond(0, 2))
				default:
					p.emit("MOVAB\t%s,HNEWPC", label)
					for i := len(c.args) - 1; i >= 0; i-- {
						p.emit("PUSHL\t%s", c.args[i])
					}
					p.sentinels()
					p.emit("CALLS\t#%d,G^LIB$SIGNAL", len(c.args))
				}
				p.both("after call")
				p.emit("RET")
				p.emit("%s:", label)
				p.both("at new PC")
			})
		})
	}

	p.kase("SIGNAL no args", func() { p.sub(mContinue, func() { p.sentinels(); p.emit("CALLS\t#0,G^LIB$SIGNAL"); p.both("after call") }) })
	p.kase("STOP no args", func() { p.sub(mUnwind, func() { p.sentinels(); p.emit("CALLS\t#0,G^LIB$STOP"); p.both("after call") }) })

	p.kase("resignal to the outer frame", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mContinue)
		p.sub(mResignal, func() { p.call("LIB$SIGNAL", cond(0, 2)); p.said("after call") })
	})
	p.kase("resignal, outer unwinds", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mUnwind)
		p.sub(mResignal, func() { p.call("LIB$SIGNAL", cond(0, 2)); p.said("after call") })
	})

	// ESTABLISH and REVERT.
	p.kase("ESTABLISH and REVERT", func() {
		p.sub(mContinue, func() {
			p.emit("MOVAB\tHND,R2")
			p.call("LIB$REVERT")
			p.emit("SUBL2\tR2,R0")
			p.said("REVERT with the frame's handler, minus HND")
			p.emit("MOVL\t(FP),R0")
			p.said("0(FP)")
			p.call("LIB$REVERT")
			p.said("REVERT with none")
			p.call("LIB$ESTABLISH", "R2")
			p.said("ESTABLISH with none")
			p.emit("SUBL3\tR2,(FP),R0")
			p.said("0(FP) minus HND")
			p.call("LIB$ESTABLISH", "#^X1000")
			p.emit("SUBL2\tR2,R0")
			p.said("ESTABLISH again, old minus HND")
			p.emit("MOVL\t(FP),R0")
			p.said("0(FP)")
			p.call("LIB$ESTABLISH", "#0")
			p.said("ESTABLISH 0")
			p.emit("MOVL\t(FP),R0")
			p.said("0(FP)")
		})
	})
	p.kase("ESTABLISH no args", func() { p.sub(mContinue, func() { p.emit("MOVAB\tHND,R2"); p.emit("CALLS\t#0,G^LIB$ESTABLISH"); p.emit("SUBL2\tR2,R0"); p.said("R0 minus HND"); p.emit("MOVL\t(FP),R0"); p.said("0(FP)") }) })
	p.kase("REVERT then signal goes outward", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mContinue)
		p.sub(mContinue, func() { p.call("LIB$REVERT"); p.call("LIB$SIGNAL", cond(0, 2)); p.said("after call") })
	})

	// MATCH_COND, whose value is the list's position: all in one case, as they stop
	// nothing. 0x0801 is a facility; 0x0C a system code, with no facility bit.
	matches := []struct {
		name string
		v    int
		list []int
	}{
		{"first", 0x08018003, []int{0x08018003, 0x08018013}}, {"second", 0x08018013, []int{0x08018003, 0x08018013}},
		{"none", 0x08018023, []int{0x08018003, 0x08018013}}, {"severity ignored", 0x08018004, []int{0x08018003}},
		{"control bits ignored", 0x18018003, []int{0x08018003}}, {"facility differs", 0x08018003, []int{0x08028003}},
		{"message differs", 0x08018003, []int{0x08018013}}, {"system, facility differs", 0x0C, []int{0x0802000C}},
		{"system, severity", 0x0C, []int{0x0B}}, {"list has the facility bit", 0x0C, []int{0x08018008}}, {"zero", 0, []int{0}},
		{"no list", 0x08018003, nil},
	}

	p.kase("MATCH_COND", func() {
		p.sub(mUnwind, func() {
			for _, m := range matches {
				p.dataf("MV%d:\t.LONG\t%d", p.nlab, m.v)
				args := []string{fmt.Sprintf("&MV%d", p.nlab)}
				p.nlab++

				for _, v := range m.list {
					p.dataf("MV%d:\t.LONG\t%d", p.nlab, v)
					args = append(args, fmt.Sprintf("&MV%d", p.nlab))
					p.nlab++
				}

				p.call("LIB$MATCH_COND", args...)
				p.said(m.name)
			}
		})
	})
	p.kase("MATCH_COND no args", func() { p.sub(mUnwind, func() { p.emit("CALLS\t#0,G^LIB$MATCH_COND"); p.said("R0") }) })

	// SIG_TO_RET and SIG_TO_STOP, as handlers and called from one.
	p.kase("SIG_TO_RET as the handler", func() {
		p.sub(mContinue, func() {
			p.emit("MOVAB\tG^LIB$SIG_TO_RET,(FP)")
			p.call("LIB$SIGNAL", cond(0, 2))
			p.said("not here")
		})
	})
	p.kase("SIG_TO_RET called by the handler", func() {
		p.sub(mSigToRet, func() { p.call("LIB$SIGNAL", cond(0, 2)); p.said("not here") })
	})
	p.kase("SIG_TO_RET on STOP", func() {
		p.sub(mSigToRet, func() { p.call("LIB$STOP", cond(0, 2)); p.said("not here") })
	})
	p.kase("SIG_TO_RET no args", func() { p.sub(mContinue, func() { p.emit("CALLS\t#0,G^LIB$SIG_TO_RET"); p.said("R0") }) })
	p.kase("SIG_TO_RET one arg", func() { p.sub(mContinue, func() { p.call("LIB$SIG_TO_RET", "&MFAKE"); p.said("R0") }) })

	p.kase("SIG_TO_STOP as the handler, outer continues", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mContinue)
		p.sub(mContinue, func() {
			p.emit("MOVAB\tG^LIB$SIG_TO_STOP,(FP)")
			p.call("LIB$SIGNAL", cond(0, 2))
			p.said("after call")
		})
	})
	p.kase("SIG_TO_STOP as the handler, outer unwinds", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mUnwind)
		p.sub(mContinue, func() {
			p.emit("MOVAB\tG^LIB$SIG_TO_STOP,(FP)")
			p.call("LIB$SIGNAL", cond(0, 1))
			p.said("after call")
		})
	})
	p.kase("SIG_TO_STOP called by the handler, resignal, outer unwinds", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mUnwind)
		p.sub(mSigToStop, func() { p.call("LIB$SIGNAL", cond(0, 3)); p.said("after call") })
	})
	p.kase("SIG_TO_STOP on a fake array", func() {
		p.sub(mContinue, func() {
			p.call("LIB$SIG_TO_STOP", "&SFAKE", "&MFAKE")
			p.said("R0")
			p.emit("MOVL\tSFAKE+4,R0")
			p.said("condition now")
		})
	})
	p.kase("SIG_TO_STOP on SS$_UNWIND", func() {
		p.sub(mContinue, func() { p.call("LIB$SIG_TO_STOP", "&SUNW", "&MFAKE"); p.said("R0"); p.emit("MOVL\tSUNW+4,R0"); p.said("condition now") })
	})
	p.kase("SIG_TO_STOP no args", func() { p.sub(mContinue, func() { p.emit("CALLS\t#0,G^LIB$SIG_TO_STOP"); p.said("R0") }) })

	// FIXUP_FLT: a reserved operand in each place and format.
	fixups := []struct {
		name string
		f    func()
	}{
		{"MOVF register", func() { p.emit("MOVL\t#^X8000,R6"); p.emit("MOVF\tR6,R7"); p.emit("MOVL\tR6,R0"); p.emit("MOVL\tR7,R1") }},
		{"MOVF memory", func() { p.emit("MOVL\t#^X8000,FL1"); p.emit("MOVF\tFL1,R7"); p.emit("MOVL\tFL1,R0"); p.emit("MOVL\tR7,R1") }},
		{"MOVF deferred", func() { p.emit("MOVL\t#^X8000,FL1"); p.emit("MOVAB\tFL1,R8"); p.emit("MOVF\t(R8),R7"); p.emit("MOVL\tFL1,R0"); p.emit("MOVL\tR7,R1") }},
		{"MOVF autoincrement", func() {
			p.emit("MOVL\t#^X8000,FL1")
			p.emit("MOVAB\tFL1,R8")
			p.emit("MOVF\t(R8)+,R7")
			p.emit("SUBL3\t#FL1,R8,R0")
			p.emit("MOVL\tR7,R1")
		}},
		{"MOVF autodecrement", func() {
			p.emit("MOVL\t#^X8000,FL1")
			p.emit("MOVAB\tFL1+4,R8")
			p.emit("MOVF\t-(R8),R7")
			p.emit("SUBL3\t#FL1,R8,R0")
			p.emit("MOVL\tR7,R1")
		}},
		{"MOVF indexed", func() { p.emit("MOVL\t#^X8000,FL1+4"); p.emit("MOVL\t#1,R8"); p.emit("MOVF\tFL1[R8],R7"); p.emit("MOVL\tFL1+4,R0"); p.emit("MOVL\tR7,R1") }},
		{"MOVF immediate", func() { p.emit(".BYTE\t^X50,^X8F,0,^X80,0,0,^X57"); p.emit("CLRL\tR0"); p.emit("MOVL\tR7,R1") }},
		{"ADDF2 first", func() { p.emit("MOVL\t#^X8000,FL1"); p.emit("MOVF\t#1.0,FL2"); p.emit("ADDF2\tFL1,FL2"); p.emit("MOVL\tFL1,R0"); p.emit("MOVL\tFL2,R1") }},
		{"ADDF2 second", func() { p.emit("MOVL\t#^X8000,FL2"); p.emit("MOVF\t#1.0,FL1"); p.emit("ADDF2\tFL1,FL2"); p.emit("MOVL\tFL1,R0"); p.emit("MOVL\tFL2,R1") }},
		{"MULF3 both", func() { p.emit("MOVL\t#^X8000,FL1"); p.emit("MOVL\t#^X8000,FL2"); p.emit("MULF3\tFL1,FL2,R7"); p.emit("MOVL\tFL1,R0"); p.emit("MOVL\tR7,R1") }},
		{"MOVD", func() { p.emit("MOVQ\tDQ1,DL1"); p.emit("MOVD\tDL1,R6"); p.emit("MOVL\tDL1,R0"); p.emit("MOVL\tDL1+4,R1") }},
		{"MOVG", func() { p.emit("MOVQ\tDQ1,DL1"); p.emit("MOVG\tDL1,R6"); p.emit("MOVL\tDL1,R0"); p.emit("MOVL\tDL1+4,R1") }},
		{"MOVH", func() { p.emit("MOVQ\tDQ1,DL1"); p.emit("MOVH\tDL1,R6"); p.emit("MOVL\tDL1,R0"); p.emit("MOVL\tDL1+4,R1") }},
		{"CVTFL", func() { p.emit("MOVL\t#^X8000,FL1"); p.emit("CVTFL\tFL1,R7"); p.emit("MOVL\tFL1,R0"); p.emit("MOVL\tR7,R1") }},
		{"POLYF table", func() { p.emit("MOVF\t#1.0,FL1"); p.emit("MOVL\t#^X8000,FL2"); p.emit("POLYF\tFL1,#1,FL2"); p.emit("MOVL\tFL2,R1") }},
		{"reserved decimal", func() { p.emit("CVTPL\t#32,PBAD,R7"); p.emit("CLRL\tR0"); p.emit("MOVL\tR7,R1") }},
	}

	for _, f := range fixups {
		for _, m := range []struct {
			name string
			mode int
		}{{"FIXUP_FLT as handler", -1}, {"handler calls it", mFixup}, {"with a new operand", mFixupNew}} {
			f, m := f, m

			p.kase(f.name+", "+m.name, func() {
				mode := m.mode
				if mode < 0 {
					mode = mContinue
				}

				p.emit("MOVAB\tHND2,(FP)")
				p.emit("MOVL\t#%d,HMODE2", mUnwind)
				p.sub(mode, func() {
					if m.mode < 0 {
						p.emit("MOVAB\tG^LIB$FIXUP_FLT,(FP)")
					}

					f.f()
					p.both("operand and result")
				})
			})
		}
	}

	p.kase("FIXUP_FLT with a reserved new operand", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mUnwind)
		p.emit("MOVL\t#^X8000,NEWF")
		p.sub(mFixupNew, func() { p.emit("MOVL\t#^X8000,FL1"); p.emit("MOVF\tFL1,R7"); p.said("result") })
	})
	p.kase("FIXUP_FLT of another condition", func() {
		p.emit("MOVAB\tHND2,(FP)")
		p.emit("MOVL\t#%d,HMODE2", mUnwind)
		p.sub(mFixup, func() { p.call("LIB$SIGNAL", cond(0, 2)); p.said("after call") })
	})
	p.kase("FIXUP_FLT no args", func() { p.sub(mContinue, func() { p.emit("CALLS\t#0,G^LIB$FIXUP_FLT"); p.said("R0") }) })
	p.kase("FIXUP_FLT one arg", func() { p.sub(mContinue, func() { p.call("LIB$FIXUP_FLT", "&SFAKE"); p.said("R0") }) })

	// INT_OVER, DEC_OVER, and FLT_UNDER: the old setting, and what each does to its
	// caller, its callees, and its caller's caller. Each trap shows as the handler's
	// line, its PC relative to the routine.
	for _, r := range []struct {
		name, routine, mask, code string
	}{
		{"INT_OVER", "LIB$INT_OVER", "IV", "ADDL3\t#^X7FFFFFFF,#^X7FFFFFFF,R7"},
		{"DEC_OVER", "LIB$DEC_OVER", "DV", "ADDP4\t#3,PMAX,#1,PTWO"},
		{"FLT_UNDER", "LIB$FLT_UNDER", "", "MULF3\tFTINY,FTINY,R7"},
	} {
		r := r
		bis := ""

		if r.mask == "" {
			bis = "BISPSW\t#^X40"
		}

		id := p.nlab
		p.nlab++
		mask := "^M<R2,R3,R4,R5,R6,R7>"

		if r.mask != "" {
			mask = "^M<R2,R3,R4,R5,R6,R7," + r.mask + ">"
		}

		// A routine that does the operation, its mask or PSW enabling the trap or not.
		fmt.Fprintf(&p.code, "\t.ENTRY\tON%d,%s\n", id, mask)

		if bis != "" {
			fmt.Fprintf(&p.code, "\t%s\n", bis)
		}

		fmt.Fprintf(&p.code, "\t%s\n\tRET\n\t.ENTRY\tOFF%d,^M<R2,R3,R4,R5,R6,R7>\n\t%s\n\tRET\n", r.code, id, r.code)

		p.kase(r.name+": the settings it returns", func() {
			p.sub(mContinue, func() {
				for _, v := range []string{"LONE", "LZERO", "LONE", "LTWO", "LTHREE", "LZERO"} {
					p.call(r.routine, "&"+v)
					p.said("set from " + v)
				}
			})
		})
		p.kase(r.name+": in a routine enabled by its mask", func() {
			p.sub(mContinue, func() {
				p.emit("%s", "CALLS\t#0,L^ON"+fmt.Sprint(id))
				p.said("enabled routine")
				p.emit("CALLS\t#0,L^OFF%d", id)
				p.said("disabled routine")
			})
		})
		p.kase(r.name+": enable, then the operation here", func() {
			p.sub(mContinue, func() {
				p.call(r.routine, "&LONE")
				p.said("old")
				p.emit("%s", r.code)
				p.said("did it")
				p.call(r.routine, "&LZERO")
				p.said("old")
				p.emit("%s", r.code)
				p.said("did it again")
			})
		})
		p.kase(r.name+": enable, then call a routine", func() {
			p.sub(mContinue, func() {
				p.call(r.routine, "&LONE")
				p.emit("CALLS\t#0,L^OFF%d", id)
				p.said("callee with its own frame")
				p.emit("%s", r.code)
				p.said("here, after")
			})
		})
		p.kase(r.name+": no args", func() { p.sub(mContinue, func() { p.emit("CALLS\t#0,G^%s", r.routine); p.said("R0") }) })
		p.kase(r.name+": null", func() { p.sub(mContinue, func() { p.call(r.routine, "#0"); p.said("R0") }) })
	}

	return p
}

const runHead = `	.TITLE	%[1]s	Phase 83's probe of conditions
	.IDENT	/V1.0/

; Written by testdata/for83/gen.go: do not edit. One case per run, by the number
; on the command line, since a case may end its image. See gen.go.

	$SSDEF

	.PSECT	DATA,NOEXE,WRT,QUAD
HMODE:	.LONG	0		; what HND does
HMODE2:	.LONG	0		; what HND2 does
HCOUNT:	.LONG	0		; how many conditions, to stop a loop
HOOK:	.LONG	0		; a routine for mHook, and for the AST
HNEWPC:	.LONG	0		; where mUnwindPC goes
SFP:	.LONG	0		; the frame of the routine under test
SADDR:	.LONG	0		; and its address
XMODE:	.LONG	0		; what the exit handler does
XST:	.LONG	0		; the exit status it's given
XBLK:	.LONG	0
	.ADDRESS XHAND
	.LONG	1
	.ADDRESS XST
NEWF:	.FLOAT	1.5		; a new operand
LZERO:	.LONG	0
LONE:	.LONG	1
LTWO:	.LONG	2
LTHREE:	.LONG	3
WTHREE:	.WORD	3
WMINUS:	.WORD	-1
LVAL:	.LONG	^X55555555
FL1:	.LONG	0,0
FL2:	.LONG	0,0
	.ALIGN	QUAD
DQ1:	.LONG	^X8000,0
DL1:	.LONG	0,0,0,0
PBAD:	.BYTE	^XAA,^XAA
PMAX:	.PACKED	999
PTWO:	.PACKED	2
FTINY:	.FLOAT	1.0E-30
SFAKE:	.LONG	3,^X08018002,0,0
SUNW:	.LONG	3,SS$_UNWIND,0,0
MFAKE:	.LONG	4,0,0,0,0
FMTA:	.ASCIZ	~(A)~
FMTAA:	.ASCIZ	~(A,A)~
FMTI1:	.ASCIZ	~(I1)~
FMTI4:	.ASCIZ	~(I4)~
FMTBAD:	.ASCIZ	~(Q9~
FMTLOOP: .ASCIZ	~(/)~
RBUF:	.BLKB	40
RBD:	.WORD	40
	.BYTE	14,1
	.ADDRESS RBUF
IBUF:	.BLKB	20
IFD:	.WORD	20
	.BYTE	14,1
	.ADDRESS IBUF
PARAM:	.BLKB	16
PARD:	.WORD	16
	.BYTE	14,1
	.ADDRESS PARAM
PARL:	.WORD	0
OUTL:	.WORD	0
OUTB:	.BLKB	200
OUTD:	.WORD	200
	.BYTE	14,1
	.ADDRESS OUTB
ONE:	.WORD	0
	.BYTE	14,1
	.ADDRESS OUTB
MSHOW:	.ASCID	~ =!AS r0=!XL~
MSHOW2:	.ASCID	~ =!AS r0=!XL r1=!XL~
MSIG:	.ASCID	~ =c n=!UL cond=!XL arg=!XL !XL pc=+!XL psl=!XL~
MMECH:	.ASCID	~ =m n=!UL frame=+!XL depth=!SL r0=!XL r1=!XL~
MUNW:	.ASCID	~ =u the unwind's call of the handler~
MLOOP:	.ASCID	~ =too many conditions; the program stops itself~
MHAS:	.ASCID	~ =h !AS r0=!XL~
MFIX:	.ASCID	~FIXUP_FLT~
MRET:	.ASCID	~SIG_TO_RET~
MSTOP:	.ASCID	~SIG_TO_STOP~
MCOND:	.ASCID	~ =cond now !XL~
MV:	.ASCID	~ =value !XL~
MRB:	.ASCID	~ =read r0=!XL |!AD|~
MXH:	.ASCID	~ =exit handler status=!XL~
MBAD:	.ASCID	~ =no such case~
MINNER:	.ASCID	~ =i n=!UL cond=!XL~

	.PSECT	CODE,NOWRT,EXE
; OUT(control, args...)
	.ENTRY	OUT,^M<R2,R3,R4,R5>
	$FAOL_S	CTRSTR=@4(AP),OUTLEN=ONE,OUTBUF=OUTD,PRMLST=8(AP)
	PUSHAB	ONE
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
; SHOW(what, r0), SHOW2(what, r0, r1)
	.ENTRY	SHOW,^M<>
	PUSHL	8(AP)
	PUSHL	4(AP)
	PUSHAB	MSHOW
	CALLS	#3,OUT
	RET
	.ENTRY	SHOW2,^M<>
	PUSHL	12(AP)
	PUSHL	8(AP)
	PUSHL	4(AP)
	PUSHAB	MSHOW2
	CALLS	#4,OUT
	RET
	.ENTRY	SHOWV,^M<>
	PUSHL	4(AP)
	PUSHAB	MV
	CALLS	#2,OUT
	RET
; SHOWRB(r0): what a READ left, and fills the buffer for the next.
	.ENTRY	SHOWRB,^M<R2,R3,R4,R5>
	PUSHAB	RBUF
	PUSHL	#40
	PUSHL	4(AP)
	PUSHAB	MRB
	CALLS	#4,OUT
	MOVC5	#0,RBUF,#^A/./,#40,RBUF
	RET

; HND and HND2 are the handler, with their own modes. HCORE shows the arrays,
; then does what the mode says.
	.ENTRY	HND,^M<R2,R3,R4,R5,R6,R7>
	MOVL	HMODE,R6
	JSB	HCORE
	RET
	.ENTRY	HND2,^M<R2,R3,R4,R5,R6,R7>
	MOVL	HMODE2,R6
	JSB	HCORE
	RET
HCORE:	MOVL	4(AP),R2
	MOVL	8(AP),R3
	CMPL	4(R2),#SS$_UNWIND
	BNEQ	10$
	PUSHAB	MUNW
	CALLS	#1,OUT
	MOVL	#1,R0
	RSB
10$:	AOBLEQ	#12,HCOUNT,20$
	PUSHAB	MLOOP
	CALLS	#1,OUT
	$EXIT_S	#3
20$:	MOVL	(R2),R5			; PC and PSL are the last two
	PUSHL	(R2)[R5]
	DECL	R5
	SUBL3	SADDR,(R2)[R5],-(SP)
	CLRQ	-(SP)
	CMPL	(R2),#3
	BLEQ	26$
	MOVL	8(R2),(SP)
	CMPL	(R2),#4
	BLEQ	26$
	MOVL	12(R2),4(SP)
26$:	PUSHL	4(R2)
	PUSHL	(R2)
	PUSHAB	MSIG
	CALLS	#7,OUT
	PUSHL	16(R3)
	PUSHL	12(R3)
	PUSHL	8(R3)
	SUBL3	SFP,4(R3),-(SP)
	PUSHL	(R3)
	PUSHAB	MMECH
	CALLS	#6,OUT
	CASEL	R6,#0,#11
30$:	.WORD	40$-30$,41$-30$,42$-30$,43$-30$,44$-30$,45$-30$,46$-30$
	.WORD	47$-30$,48$-30$,49$-30$,50$-30$,51$-30$
40$:	MOVL	#SS$_CONTINUE,R0
	RSB
41$:	MOVL	#SS$_RESIGNAL,R0
	RSB
42$:	$UNWIND_S
	RSB
43$:	CLRL	-(SP)			; by hand: the service's own argument list
	PUSHAB	8(R3)
	CALLS	#2,G^SYS$UNWIND
	RSB
44$:	PUSHL	HNEWPC
	CLRL	-(SP)
	CALLS	#2,G^SYS$UNWIND
	RSB
45$:	MOVL	#^X33333333,12(R3)
	MOVL	#^X44444444,16(R3)
	MOVL	#SS$_CONTINUE,R0
	RSB
46$:	MOVL	#^X33333333,12(R3)
	MOVL	#^X44444444,16(R3)
	$UNWIND_S
	RSB
47$:	CALLS	#0,@HOOK
	MOVL	#SS$_CONTINUE,R0
	RSB
48$:	CALLG	(AP),G^LIB$FIXUP_FLT
	BRB	60$
49$:	PUSHAB	NEWF
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#3,G^LIB$FIXUP_FLT
60$:	MOVL	R0,R7
	PUSHL	R0
	PUSHAB	MFIX
	PUSHAB	MHAS
	CALLS	#3,OUT
	MOVL	R7,R0
	RSB
50$:	CALLG	(AP),G^LIB$SIG_TO_RET
	MOVL	R0,R7
	PUSHL	R0
	PUSHAB	MRET
	PUSHAB	MHAS
	CALLS	#3,OUT
	MOVL	R7,R0
	RSB
51$:	CALLG	(AP),G^LIB$SIG_TO_STOP
	MOVL	R0,R7
	PUSHL	R0
	PUSHAB	MSTOP
	PUSHAB	MHAS
	CALLS	#3,OUT
	PUSHL	4(R2)
	PUSHAB	MCOND
	CALLS	#2,OUT
	MOVL	#SS$_RESIGNAL,R0
	RSB
; INNER is the handler over a hook's statement: it shows and continues.
	.ENTRY	INNER,^M<R2>
	MOVL	4(AP),R2
	CMPL	4(R2),#SS$_UNWIND
	BEQL	10$
	AOBLEQ	#12,HCOUNT,5$
	PUSHAB	MLOOP
	CALLS	#1,OUT
	$EXIT_S	#3
5$:	PUSHL	4(R2)
	PUSHL	(R2)
	PUSHAB	MINNER
	CALLS	#3,OUT
10$:	MOVL	#SS$_CONTINUE,R0
	RET
; ASTHOOK is an AST routine: it calls the hook.
	.ENTRY	ASTHOOK,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	CALLS	#0,@HOOK
	RET
`

const runExit = `; XHAND is the exit handler. By XMODE it writes a record on unit 12, or on a unit
; opened now.
	.ENTRY	XHAND,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAB	INNER,(FP)
	PUSHL	XST
	PUSHAB	MXH
	CALLS	#2,OUT
	CMPL	XMODE,#1
	BNEQ	10$
	WRITE12
10$:	CMPL	XMODE,#2
	BNEQ	20$
	WRITE15
20$:	RET
`

// write is the program, with its driver.
func (p *runProg) program(link bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, runHead, p.name)

	if link {
		code := runExit
		w12 := "PUSHAB\tFMTA\n\tPUSHL\t#12\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHAB\tXTEXT\n\tCALLS\t#1,G^FOR$IO_T_DS\n\tCALLS\t#0,G^FOR$IO_END\n\tPUSHL\tR0\n\tCALLS\t#1,SHOWV"
		w15 := "PUSHAB\tFMTA\n\tPUSHL\t#15\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHAB\tXTEXT\n\tCALLS\t#1,G^FOR$IO_T_DS\n\tCALLS\t#0,G^FOR$IO_END\n\tPUSHL\tR0\n\tCALLS\t#1,SHOWV"
		code = strings.Replace(code, "WRITE12", w12, 1)
		code = strings.Replace(code, "WRITE15", w15, 1)
		b.WriteString(code)
	} else {
		b.WriteString("\t.ENTRY\tXHAND,^M<>\n\tRET\n")
	}

	b.WriteString(p.code.String())
	b.WriteString("\n\t.ENTRY\tSTART,^M<R2>\n")
	b.WriteString("\tPUSHAB\tPARL\n\tCLRL\t-(SP)\n\tPUSHAB\tPARD\n\tCALLS\t#3,G^LIB$GET_FOREIGN\n")
	b.WriteString("\tMOVW\tPARL,PARD\n\tPUSHL\t#1\n\tPUSHL\t#4\n\tPUSHAB\tLVAL\n\tPUSHAB\tPARD\n\tCALLS\t#4,G^OTS$CVT_TI_L\n")
	b.WriteString("\tMOVL\tLVAL,R2\n")
	fmt.Fprintf(&b, "\tCMPL\tR2,#%d\n\tBGTRU\t90$\n", len(p.cases))
	b.WriteString("\tTSTL\tR2\n\tBEQL\t90$\n\tMOVL\tCTAB-4[R2],R0\n\tCALLS\t#0,(R0)\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n90$:\tPUSHAB\tMBAD\n\tCALLS\t#1,OUT\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n")
	b.WriteString("\n\t.PSECT\tDATA\n")
	b.WriteString(p.data.String())
	b.WriteString("XTEXT:\t.ASCID\t~from the exit handler~\n")
	b.WriteString("CTAB:\n")

	for i := range p.cases {
		fmt.Fprintf(&b, "\t.ADDRESS\tC%d\n", i+1)
	}

	b.WriteString("\n\t.END\tSTART\n")

	return b.String()
}

// The procedures. A case's file is typed after its run, and deleted.
const runCom = `$ ! FOR83R.COM - runs a program's cases, one per image. P1 is the program, P2 how
$ ! many. Written by testdata/for83/gen.go.
$ SET NOON
$ SET NOVERIFY
$ DIR = F$ENVIRONMENT("DEFAULT")
$ PROG :== $'DIR''P1'
$ N = 1
$ LAST = F$INTEGER(P2)
$NEXT:
$ PROG 'N'
$ SHOW SYMBOL $STATUS
$ GOSUB FILES
$ N = N + 1
$ IF N .LE. LAST THEN GOTO NEXT
$ DELETE/SYMBOL/GLOBAL PROG
$ EXIT 1
$FILES:
$ NAMES = "FOR83W.DAT,FOR012.DAT,FOR013.DAT,FOR014.DAT,FOR015.DAT"
$ I = 0
$FILE:
$ NAME = F$ELEMENT(I,",",NAMES)
$ IF NAME .EQS. "," THEN RETURN
$ I = I + 1
$ IF F$SEARCH(NAME) .EQS. "" THEN GOTO FILE
$ WRITE SYS$OUTPUT " file ",NAME,":"
$ TYPE 'NAME'
$ DELETE 'NAME';*
$ GOTO FILE
`

func command(b, c int) string {
	return fmt.Sprintf(`$ ! FOR83.COM - the Phase 83 probe. Written by testdata/for83/gen.go. Run it with the
$ ! exchange volume's [000000] as the default directory:
$ !
$ !     @FOR83/OUTPUT=FOR83.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ WRITE SYS$OUTPUT F$TIME()
$ !
$ ! The STR$ routines.
$ DEFINE/USER_MODE SYS$OUTPUT STR83.MLG
$ MACRO/NOLIST STR83
$ LINK/NOMAP STR83
$ DEFINE/USER_MODE SYS$OUTPUT STR83-VAX.TXT
$ RUN STR83
$ SHOW SYMBOL $STATUS
$ !
$ ! Part A: FORRTL's system subroutines.
$ DEFINE/USER_MODE SYS$OUTPUT FOR83.MLG
$ MACRO/NOLIST FOR83
$ LINK/NOMAP FOR83,FOR83/OPTIONS
$ DEFINE/USER_MODE SYS$OUTPUT FOR83-VAX.TXT
$ RUN FOR83
$ SHOW SYMBOL $STATUS
$ !
$ ! Part B's files: FOR83D.DAT holds records, FOR83E.DAT none.
$ CREATE FOR83D.DAT
12X4
LINE 2
LINE 3
$ CREATE FOR83E.DAT
$ DEFINE/USER_MODE SYS$OUTPUT FOR83B.MLG
$ MACRO/NOLIST FOR83B
$ LINK/NOMAP FOR83B,FOR83/OPTIONS
$ @FOR83R/OUTPUT=FOR83B-VAX.TXT FOR83B %[1]d
$ SHOW SYMBOL $STATUS
$ !
$ ! Part C: LIBRTL's condition routines.
$ DEFINE/USER_MODE SYS$OUTPUT FOR83C.MLG
$ MACRO/NOLIST FOR83C
$ LINK/NOMAP FOR83C
$ @FOR83R/OUTPUT=FOR83C-VAX.TXT FOR83C %[2]d
$ SHOW SYMBOL $STATUS
$ !
$ ! Which definitions LINK finds, and $OPCDEF's values, from the object: no listing.
$ MACRO/NOLIST LNK83
$ LINK/NOMAP LNK83
$ SHOW SYMBOL $STATUS
$ MACRO/NOLIST DEF_OPC
$ ANALYZE/OBJECT/OUTPUT=DEF_OPC.ANL DEF_OPC.OBJ
$ !
$ ! Part D: LIBRARY/TEXT. Each library is kept at each step, for its bytes.
$ CREATE TXA.TXT
      INTEGER*4 A
      PARAMETER (A = 1)
$ CREATE TXB.TXT
C     a blank line next, then one of 132 characters, then a tab

C23456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012
	TAB FIRST
$ CREATE TXC.TXT
      C ONE
      C TWO
$ CREATE TXD.TXT
$ LIBRARY/TEXT/CREATE T1.TLB TXA
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S1.TLB
$ LIBRARY/TEXT/LIST/FULL T1.TLB
$ LIBRARY/TEXT/INSERT/LOG T1 TXB.TXT/MODULE=$MOD_B
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S2.TLB
$ LIBRARY/TEXT T1 TXC.TXT/MODULE=CMOD
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S3.TLB
$ LIBRARY/TEXT/LIST/FULL T1
$ LIBRARY/TEXT/INSERT T1 TXC.TXT/MODULE=CMOD
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT/REPLACE T1 TXA.TXT/MODULE=CMOD
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S4.TLB
$ LIBRARY/TEXT/REPLACE/LOG T1 TXB.TXT
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT T1 TXD.TXT
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S5.TLB
$ LIBRARY/TEXT T1 TXA.TXT,TXC.TXT/MODULE=TWO
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT T1 TXA.TXT/MODULE=A_NAME_OF_THIRTY_NINE_CHARACTERS_LONG_
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT T1 TXA.TXT/MODULE=A_NAME_OF_FORTY_CHARACTERS_IS_ONE_TOO_MANY
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT T1 TXA.TXT/MODULE=lower_case
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S6.TLB
$ LIBRARY/TEXT/LIST/NAMES T1
$ LIBRARY/TEXT/EXTRACT=CMOD/OUTPUT=E1.TXT T1
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT/EXTRACT=$MOD_B/OUTPUT=E2.TXT T1
$ DUMP/RECORD E2.TXT
$ LIBRARY/TEXT/EXTRACT=TXD/OUTPUT=E3.TXT T1
$ DIRECTORY/SIZE=ALL E3.TXT
$ LIBRARY/TEXT/EXTRACT=(CMOD,TXA)/OUTPUT=E4.TXT T1
$ TYPE E4.TXT
$ LIBRARY/TEXT/EXTRACT=NOSUCH/OUTPUT=E5.TXT T1
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT/DELETE=CMOD T1
$ SHOW SYMBOL $STATUS
$ COPY T1.TLB S7.TLB
$ LIBRARY/TEXT/COMPRESS/OUTPUT=S8.TLB T1
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT/COMPRESS/DATA=REDUCE/OUTPUT=S9.TLB T1
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT/LIST/FULL S9
$ LIBRARY/TEXT/EXTRACT=$MOD_B/OUTPUT=E6.TXT S9
$ DUMP/RECORD E6.TXT
$ LIBRARY/TEXT/CREATE=(BLOCKS:20,MODULES:40,KEYSIZE:31) T2.TLB TXA,TXC
$ SHOW SYMBOL $STATUS
$ LIBRARY/TEXT/LIST/FULL T2
$ LIBRARY/CREATE/TEXT T3 TXA.TXT/MODULE=M
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE=ALL/DATE *.TLB
$ DUMP/RECORD TXB.TXT
$ DIRECTORY/SIZE=ALL
$ EXIT
`, b, c)
}

const lnkProgram = `	.TITLE	LNK83	which definitions LINK finds with nothing to define them
; Written by testdata/for83/gen.go. Each name is only a reference; the LINK log's
; undefined symbols say which aren't in the system's libraries.
	.PSECT	DATA,NOEXE
	.LONG	SS$_NORMAL,RMS$_EOF,LIB$_NOTFOU,FOR$_ENDDURREA,MTH$_FLOOVEMAT,OTS$_FATINTERR,STR$_NOMATCH
	.LONG	CLI$_NORMAL,DVI$_DEVNAM,JPI$_PID,SYI$_VERSION,IO$_READVBLK,LNM$_STRING,SEC$M_GBL
	.LONG	PSL$C_USER,PSL$M_IV,PSL$V_IV,PRV$V_CMKRNL,PRV$M_CMKRNL,STS$K_ERROR,STS$M_SEVERITY,STS$V_SEVERITY
	.LONG	DSC$K_DTYPE_T,DSC$K_CLASS_S,CHF$L_MCH_DEPTH,CHF$L_SIG_NAME,FAB$L_FOP,RAB$L_ROP,NAM$B_ESL,XAB$B_COD
	.LONG	TT$_VT100,DEV$M_TRM,DC$_TERM,PRC$M_DETACH,PQL$_ASTLM,LCK$K_NLMODE,LKI$_RESNAM,OPC$_RQ_RQST,MSG$_TRMHANGUP
	.ENTRY	START,^M<>
	MOVL	#1,R0
	RET
	.END	START
`

const defProgram = `	.TITLE	DEF_OPC	$OPCDEF's values, in the object's global symbols
; Written by testdata/for83/gen.go.
	$OPCDEF	GLOBAL
	.END
`

const exchange = `! EXCHANGE.CMD - builds the Phase 83 probe's exchange volume with govax.
! Written by testdata/for83/gen.go. Run from the repository root:
!
!     govax console < testdata/for83/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for83-exchange.dsk" /DEVICE=RD54 FOR83
MOUNT/WRITE DUA2 "testdata/disks/for83-exchange.dsk"
COPY "testdata/for83/for83.mar"/HOST DUA2:[000000]FOR83.MAR
COPY "testdata/for83/for83b.mar"/HOST DUA2:[000000]FOR83B.MAR
COPY "testdata/for83/for83c.mar"/HOST DUA2:[000000]FOR83C.MAR
COPY "testdata/for83/lnk83.mar"/HOST DUA2:[000000]LNK83.MAR
COPY "testdata/for83/def_opc.mar"/HOST DUA2:[000000]DEF_OPC.MAR
COPY "testdata/for83/for83.opt"/HOST DUA2:[000000]FOR83.OPT
COPY "testdata/for83/for83.com"/HOST DUA2:[000000]FOR83.COM
COPY "testdata/for83/for83r.com"/HOST DUA2:[000000]FOR83R.COM
COPY "testdata/str83/str83.mar"/HOST DUA2:[000000]STR83.MAR
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`

func copyout() string {
	var b strings.Builder

	b.WriteString(`! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for83/gen.go. Run from the repository root:
!
!     govax console < testdata/for83/copyout.cmd
!
MOUNT DUA2 "testdata/disks/for83-exchange.dsk"
COPY DUA2:[000000]STR83-VAX.TXT "testdata/str83/vax/str83-vax.txt"/HOST
`)

	for _, name := range []string{"FOR83-VAX.TXT", "FOR83B-VAX.TXT", "FOR83C-VAX.TXT", "FOR83.LOG", "DEF_OPC.ANL", "E1.TXT", "E2.TXT", "E4.TXT", "E6.TXT"} {
		fmt.Fprintf(&b, "COPY DUA2:[000000]%s \"testdata/for83/vax/%s\"/HOST\n", name, strings.ToLower(name))
	}

	// The libraries and the object byte for byte: as text they'd gain line ends.
	for i := 1; i <= 9; i++ {
		fmt.Fprintf(&b, "COPY DUA2:[000000]S%d.TLB \"testdata/for83/vax/s%d.tlb\"/HOST/BINARY/QUIET\n", i, i)
	}

	for _, name := range []string{"T1.TLB", "T2.TLB", "T3.TLB", "DEF_OPC.OBJ"} {
		fmt.Fprintf(&b, "COPY DUA2:[000000]%s \"testdata/for83/vax/%s\"/HOST/BINARY/QUIET\n", name, strings.ToLower(name))
	}

	b.WriteString("DISMOUNT DUA2\n")

	return b.String()
}

func main() {
	cases = partA()

	b := forrtlCases()
	c := librtlCases()
	e := continueCases()
	g := goneCases()
	dir := filepath.Join("testdata", "for83")

	for name, text := range map[string]string{
		"for83.mar":    program(),
		"for83b.mar":   b.program(true),
		"for83c.mar":   c.program(false),
		"for83e.mar":   e.program(true),
		"for83g.mar":   g.program(true),
		"for83e.opt":   "FORRTL/SHAREABLE\n",
		"for83d.dat":   "12X4\nLINE 2\nLINE 3\n",
		"for83e.dat":   "",
		"for83f.dat":   "12X4  77\n",
		"for83h.dat":   "9700  77\n",
		"for83g.dat":   strings.Repeat("X", 300) + "\n",
		"lnk83.mar":    lnkProgram,
		"def_opc.mar":  defProgram,
		"for83.opt":    "SYS$SHARE:FORRTL/SHAREABLE\n",
		"for83.com":    command(len(b.cases), len(c.cases)),
		"for83r.com":   runCom,
		"exchange.cmd": exchange,
		"copyout.cmd":  copyout(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("for83.mar: %d cases; for83b.mar: %d runs; for83c.mar: %d runs\n", len(cases), len(b.cases), len(c.cases))
}

// The harness's data and the cases it runs.
var shared = []string{"\t.ALIGN\tQUAD"}

var cases []probeCase


// strings), the handler, and the routines that print.
const programHead = `	.TITLE	FOR83	Phase 83's probe of FORRTL's system subroutines
	.IDENT	/V1.0/

; Written by testdata/for83/gen.go: do not edit. Calls each of FORRTL's system
; subroutines, case by case, each in a routine of its own under
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
NUMT:	.BLKW	7		; $NUMTIM's values
NOW:	.LONG	0		; seconds since midnight, F_floating
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

; SHOWRAW name, address, count: count bytes as text.
	.ENTRY	SHOWRAW,^M<R2>
	$FAO_S	CTRSTR=TXTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=12(AP),P3=12(AP),P4=8(AP)
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; NOWF sets NOW to the time of day in seconds.
	.ENTRY	NOWF,^M<>
	$NUMTIM_S TIMBUF=NUMT
	MOVZWL	NUMT+6,R0
	MULL2	#3600,R0
	MOVZWL	NUMT+8,R1
	MULL2	#60,R1
	ADDL2	R1,R0
	MOVZWL	NUMT+10,R1
	ADDL2	R1,R0
	CVTLF	R0,NOW
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
			case "raw":
				fmt.Fprintf(&b, "\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#3,SHOWRAW\n",
					sh.count, prefix(sh.label, n), n, k+1)
			case "vs":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWVS\n", prefix(sh.label, n), n, k+1)
			}
		}
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

