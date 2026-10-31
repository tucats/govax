//go:build ignore

// gen.go writes the Phase 81 probe of FORRTL's file statements and of RMS's
// relative and fixed-length files (docs/PHASE-81 - FORTRAN files.md, subtask 2).
// VMS's run is FTNRTL's and RMS's requirement; only what FORRTL does is used,
// never its code. Run this from the repository root:
//
//	go run testdata/for81/gen.go
//
// It writes
//
//	for81.mar    FORRTL: OPEN, CLOSE, INQUIRE, and the file statements
//	for81r.mar   RMS: relative and fixed-length files, FOP's bits, block dumps
//	for81.opt    the link options
//	for81.com    builds and runs both, for81b.com under it
//	exchange.cmd, copyout.cmd
//
// FOR$OPEN, FOR$CLOSE, and FOR$INQUIRE take a list of keywords. Each is a
// longword: the keyword's code in byte 0, a mechanism in byte 1, and for the
// immediate mechanisms the value in the high word. The phase doc has the table,
// found by trial over govax.
//
// A case is a routine of its own under HND, which prints each condition,
// continues from one below severe, and unwinds to the driver from a severe one.
// The driver closes units 10 to 19 after every case, so one case can't disturb
// the next. The output is
//
//	#n  title
//	=WHAT r=x          a statement or service, and its R0
//	=c cond n=x a=x x  a condition: the signal array's size and first two arguments
//	=u                 the handler unwound
//	 ...               what the case then shows: a FAB and RAB, an INQUIRE's answers,
//	                   a record, a file's attributes, records, and blocks in hex
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/for81"

// Keyword mechanisms.
const (
	mWordImm  = 2  // the value in the high word
	mWordRef  = 3  // the next argument is a word's address
	mLongVal  = 4  // the next argument is the value
	mLongRef  = 5  // the next argument is a longword's address
	mDescRef  = 7  // the next argument is a descriptor's address
	mByteRef  = 10 // the next argument is a byte's address
	kDispose  = 2
	kErr      = 3
	kAccess   = 4
	kForm     = 5
	kRecl     = 6
	kCC       = 7
	kReadOnly = 8
	kBufCount = 9
	kInitial  = 10
	kExtend   = 11
	kNoSpan   = 12
	kShared   = 13
	kFile     = 14
	kStatus   = 15
	kMaxRec   = 16
	kBlock    = 18
	kOrg      = 19
	kRecType  = 20
	kUserOpen = 21
	kAssoc    = 22
	kKey      = 23
	kBlank    = 24
	kDefault  = 26
)

// A prog collects one program's data, cases, and driver.
type prog struct {
	name  string
	data  strings.Builder
	code  strings.Builder
	drive strings.Builder
	body  *strings.Builder
	n     int
	texts map[string]string
	subs  strings.Builder
	nsub  int
	inner bool
}

func newProg(name string) *prog { return &prog{name: name, texts: map[string]string{}} }

// text is a label for a .ASCID of s.
func (p *prog) text(s string) string {
	if l, ok := p.texts[s]; ok {
		return l
	}

	l := fmt.Sprintf("T%d", len(p.texts))
	p.texts[s] = l
	fmt.Fprintf(&p.data, "%s:\t.ASCID\t~%s~\n", l, s)

	return l
}

func (p *prog) emit(format string, a ...any) {
	fmt.Fprintf(p.body, "\t"+format+"\n", a...)
}

// section prints a heading in the output.
func (p *prog) section(s string) {
	fmt.Fprintf(&p.drive, "\tPUSHAB\t%s\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n", p.text("##### "+s))
}

// kase adds one case.
func (p *prog) kase(title string, f func()) {
	n := p.n
	p.n++
	fmt.Fprintf(&p.drive, "\tPUSHAB\t%s\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n", p.text(fmt.Sprintf("#%d %s", n, title)))
	fmt.Fprintf(&p.drive, "\tCALLS\t#0,L^C%d\n\tCALLS\t#0,L^CLEAN\n", n)
	fmt.Fprintf(&p.code, "\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHND,(FP)\n", n)
	p.body = &p.code
	f()
	p.code.WriteString("\tRET\n")
}

// stmt puts a statement in a routine of its own under HND, so a severe condition
// unwinds only that statement and the case goes on.
func (p *prog) stmt(f func()) {
	if p.inner {
		f()
		return
	}

	id := p.nsub
	p.nsub++
	prev := p.body

	var b strings.Builder

	p.body = &b
	p.inner = true
	f()
	p.inner = false
	p.body = prev
	fmt.Fprintf(&p.subs, "\t.ENTRY\tS%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHND,(FP)\n%s\tRET\n", id, b.String())
	p.emit("CALLS\t#0,L^S%d", id)
}

// call calls a routine with operands: "&x" is x's address, anything else is
// pushed as it stands. It shows R0 under the routine's name, with show set.
func (p *prog) call(routine string, show bool, args ...string) {
	if show {
		p.stmt(func() { p.call1(routine, show, args...) })
		return
	}

	p.call1(routine, show, args...)
}

func (p *prog) call1(routine string, show bool, args ...string) {
	for i := len(args) - 1; i >= 0; i-- {
		if strings.HasPrefix(args[i], "&") {
			p.emit("PUSHAB\t%s", args[i][1:])
		} else {
			p.emit("PUSHL\t%s", args[i])
		}
	}

	p.emit("CALLS\t#%d,G^%s", len(args), routine)

	if show {
		p.shownow(strings.TrimPrefix(routine, "FOR$"))
	}
}

func (p *prog) shownow(what string) {
	p.emit("PUSHL\tR0")
	p.emit("PUSHAB\t%s", p.text(what))
	p.emit("CALLS\t#2,L^SHOW")
}

// A kw is a keyword's arguments.
type kw []string

func imm(code, v int) kw { return kw{fmt.Sprintf("#^X%04X%02X%02X", v&0xFFFF, mWordImm, code)} }
func text(p *prog, code int, s string) kw {
	return kw{fmt.Sprintf("#^X%02X%02X", mDescRef, code), "&" + p.text(s)}
}
func addr(code int, label string) kw {
	return kw{fmt.Sprintf("#^X%02X%02X", mLongVal, code), "&" + label}
}
func ref(code, mech int, label string) kw {
	return kw{fmt.Sprintf("#^X%02X%02X", mech, code), "&" + label}
}
func file(p *prog, name string) kw { return text(p, kFile, name) }
func unit(u int) kw                { return kw{fmt.Sprintf("#^X%04X0101", u&0xFFFF)} }

var errFlag = kw{"#^X00010203"}

func args(kws ...kw) []string {
	var out []string

	for _, k := range kws {
		out = append(out, k...)
	}

	return out
}

// open is FOR$OPEN with ERR= (the condition comes back in R0).
func (p *prog) open(u int, kws ...kw) {
	p.call("FOR$OPEN", true, args(append([]kw{unit(u), errFlag}, kws...)...)...)
}

// openS is FOR$OPEN with no ERR=: the condition is signaled.
func (p *prog) openS(u int, kws ...kw) {
	p.call("FOR$OPEN", true, args(append([]kw{unit(u)}, kws...)...)...)
}

func (p *prog) closeU(u int, kws ...kw) {
	p.call("FOR$CLOSE", true, args(append([]kw{unit(u), errFlag}, kws...)...)...)
}

func (p *prog) closeS(u int, kws ...kw) {
	p.call("FOR$CLOSE", true, args(append([]kw{unit(u)}, kws...)...)...)
}

// fab shows the unit's FAB and RAB, through FOR$RAB.
func (p *prog) fab(u int) {
	p.emit("MOVL\t#%d,UVAL", u)
	p.emit("CALLS\t#0,L^SHOWFR")
}

// stat shows a closed file's attributes as RMS reports them. flags: 1 its records,
// 2 its blocks.
func (p *prog) stat(name string, flags int) {
	p.stmt(func() {
		p.emit("PUSHL\t#%d", flags)
		p.emit("PUSHAB\t%s", p.text(name))
		p.emit("CALLS\t#2,L^DUMPF")
	})
}

// rwF is a formatted REWRITE, rwU an unformatted one, each one statement.
func (p *prog) rwF(u int, s string) {
	p.stmt(func() {
		p.call("FOR$REWRITE_SO", false, fmt.Sprintf("#%d", u), "&FMTA")
		p.call("FOR$IO_T_DS", false, "&"+p.text(s))
		p.call("FOR$IO_END", true)
	})
}

func (p *prog) rwU(u int, lens ...int) {
	p.stmt(func() {
		p.call("FOR$REWRITE_SU", false, fmt.Sprintf("#%d", u))
		p.items(lens...)
		p.call("FOR$IO_END", true)
	})
}

// inq shows all of INQUIRE's answers by unit, inqF by file.
func (p *prog) inq(u int) {
	p.emit("PUSHL\t#0")
	p.emit("PUSHL\t#^X%04X0101", u&0xFFFF)
	p.emit("CALLS\t#2,L^INQA")
}

func (p *prog) inqF(name string) {
	p.emit("PUSHAB\t%s", p.text(name))
	p.emit("PUSHL\t#0")
	p.emit("CALLS\t#2,L^INQA")
}

// wrU writes one unformatted record of an item per length.
func (p *prog) wrU(u int, lens ...int) {
	p.stmt(func() {
		p.call("FOR$WRITE_SU", false, fmt.Sprintf("#%d", u))
		p.items(lens...)
		p.call("FOR$IO_END", true)
	})
}

func (p *prog) items(lens ...int) {
	for _, n := range lens {
		p.emit("MOVW\t#%d,BIGD", n)
		p.call("FOR$IO_T_DS", false, "&BIGD")
	}
}

// rdU reads an unformatted record into one item of n bytes.
func (p *prog) rdU(u, n int) {
	p.stmt(func() {
		p.call("FOR$READ_SU", false, fmt.Sprintf("#%d", u))
		p.rdItem(n)
		p.call("FOR$IO_END", true)
		p.showRd(n)
	})
}

func (p *prog) rdItem(n int) {
	p.emit("CALLS\t#0,L^RDFILL")
	p.emit("MOVW\t#%d,RDD", n)
	p.call("FOR$IO_T_DS", false, "&RDD")
}

func (p *prog) showRd(n int) {
	if n > 32 {
		n = 32
	}

	p.emit("PUSHL\t#%d", n)
	p.emit("PUSHAB\tRDB")
	p.emit("CALLS\t#2,L^HEXS")
}

// wrF writes a formatted record: format (A), a text.
func (p *prog) wrF(u int, s string) {
	p.stmt(func() {
		p.call("FOR$WRITE_SO", false, fmt.Sprintf("#%d", u), "&FMTA")
		p.call("FOR$IO_T_DS", false, "&"+p.text(s))
		p.call("FOR$IO_END", true)
	})
}

func (p *prog) rdF(u, n int) {
	p.stmt(func() {
		p.call("FOR$READ_SO", false, fmt.Sprintf("#%d", u), "&FMTA")
		p.rdItem(n)
		p.call("FOR$IO_END", true)
		p.showRd(n)
	})
}

func (p *prog) wrDU(u, rec int, lens ...int) {
	p.stmt(func() {
		p.call("FOR$WRITE_DU", false, fmt.Sprintf("#%d", u), fmt.Sprintf("#%d", rec))
		p.items(lens...)
		p.call("FOR$IO_END", true)
	})
}

func (p *prog) rdDU(u, rec, n int) {
	p.stmt(func() {
		p.call("FOR$READ_DU", false, fmt.Sprintf("#%d", u), fmt.Sprintf("#%d", rec))
		p.rdItem(n)
		p.call("FOR$IO_END", true)
		p.showRd(n)
	})
}

func (p *prog) wrDO(u, rec int, s string) {
	p.stmt(func() {
		p.call("FOR$WRITE_DO", false, fmt.Sprintf("#%d", u), fmt.Sprintf("#%d", rec), "&FMTA")
		p.call("FOR$IO_T_DS", false, "&"+p.text(s))
		p.call("FOR$IO_END", true)
	})
}

func (p *prog) rdDO(u, rec, n int) {
	p.stmt(func() {
		p.call("FOR$READ_DO", false, fmt.Sprintf("#%d", u), fmt.Sprintf("#%d", rec), "&FMTA")
		p.rdItem(n)
		p.call("FOR$IO_END", true)
		p.showRd(n)
	})
}

func (p *prog) unitCall(routine string, u int) { p.call(routine, true, fmt.Sprintf("#%d", u)) }

func (p *prog) asv() {
	p.emit("CALLS\t#0,L^SHOWASV")
}

// The FORRTL program's cases.
func forrtl() *prog {
	p := newProg("FOR81")

	names := 0
	fresh := func() string {
		names++
		return fmt.Sprintf("F%d.DAT", names)
	}

	p.section("the keyword codes, value 1 and value 7, on default names")

	for c := 0; c <= 31; c++ {
		if c == kUserOpen || c == kAssoc || c == kFile || c == kDefault || c == kKey {
			continue
		}

		for _, v := range []int{1, 7} {
			c, v := c, v
			p.kase(fmt.Sprintf("OPEN code %d value %d", c, v), func() {
				p.open(10, imm(c, v))
				p.fab(10)
				p.closeU(10, imm(kDispose, 2))
			})
		}
	}

	p.section("the mechanisms, on UNIT, with a longword of 13 at the argument")

	for m := 0; m <= 12; m++ {
		m := m
		p.kase(fmt.Sprintf("OPEN unit mechanism %d", m), func() {
			p.emit("MOVL\t#13,VAL")
			p.call("FOR$OPEN", true, errFlag[0], fmt.Sprintf("#^X%02X01", m), "&VAL")
			p.closeU(13, imm(kDispose, 2))
		})
	}

	p.section("every value of the keywords that name one, by code, with RECL=10")

	enum := func(what string, code, last int, extra ...kw) {
		for v := 1; v <= last; v++ {
			v := v
			name := fresh()
			p.kase(fmt.Sprintf("%s=%d", what, v), func() {
				p.open(10, append([]kw{file(p, name), imm(kRecl, 10), imm(code, v)}, extra...)...)
				p.fab(10)
				p.closeU(10)
				p.stat(name, 0)
			})
		}
	}
	enum("DISPOSE", kDispose, 7)
	enum("ACCESS", kAccess, 5)
	enum("FORM", kForm, 3)
	enum("CC", kCC, 4)
	enum("STATUS", kStatus, 5)
	enum("ORGANIZATION", kOrg, 4)
	enum("RECORDTYPE", kRecType, 7)
	enum("RECORDTYPE unformatted", kRecType, 6, imm(kForm, 2))
	enum("BLANK", kBlank, 3)
	enum("RECORDSIZE?27", 27, 10)

	p.section("each keyword's text, by descriptor")

	texts := func(what string, code int, values ...string) {
		for _, s := range values {
			s := s
			name := fresh()
			p.kase(fmt.Sprintf("%s='%s'", what, s), func() {
				p.open(10, file(p, name), imm(kRecl, 10), text(p, code, s))
				p.fab(10)
				p.closeU(10)
				p.stat(name, 0)
			})
		}
	}
	texts("STATUS", kStatus, "OLD", "NEW", "SCRATCH", "UNKNOWN", "new", "NEW   ", " NEW", "NEWS", "NE", "")
	texts("DISPOSE", kDispose, "KEEP", "SAVE", "DELETE", "PRINT", "PRINT/DELETE", "SUBMIT", "SUBMIT/DELETE", "SUBMIT/KEEP", "print/delete")
	texts("ACCESS", kAccess, "SEQUENTIAL", "DIRECT", "KEYED", "APPEND", "OTHER")
	texts("FORM", kForm, "FORMATTED", "UNFORMATTED", "BINARY")
	texts("CC", kCC, "FORTRAN", "LIST", "NONE", "PRINT")
	texts("ORGANIZATION", kOrg, "SEQUENTIAL", "RELATIVE", "INDEXED", "STREAM")
	texts("RECORDTYPE", kRecType, "FIXED", "VARIABLE", "SEGMENTED", "STREAM", "STREAM_CR", "STREAM_LF", "VFC")
	texts("BLANK", kBlank, "NULL", "ZERO", "BLANK")

	p.section("numeric keywords' limits")

	nums := func(what string, code int, values []int, extra ...kw) {
		for _, v := range values {
			v := v
			name := fresh()
			p.kase(fmt.Sprintf("%s=%d", what, v), func() {
				p.emit("MOVL\t#%d,VAL", v)
				p.open(10, append([]kw{file(p, name), ref(code, mLongRef, "VAL")}, extra...)...)
				p.fab(10)
				p.closeU(10)
				p.stat(name, 0)
			})
		}
	}
	reclv := []int{-1, 0, 1, 2, 3, 80, 133, 134, 255, 256, 2048, 8191, 8192, 16383, 16384, 32765, 32766, 32767, 32768, 65535, 65536}
	nums("RECL formatted var", kRecl, reclv)
	nums("RECL formatted fixed", kRecl, reclv, imm(kRecType, 1))
	nums("RECL unformatted var", kRecl, reclv, imm(kForm, 2), imm(kRecType, 2))
	nums("RECL unformatted segmented", kRecl, reclv, imm(kForm, 2))
	nums("RECL direct", kRecl, reclv, imm(kAccess, 1))
	nums("RECL direct formatted", kRecl, reclv, imm(kAccess, 1), imm(kForm, 1))
	nums("BUFFERCOUNT", kBufCount, []int{-128, -1, 0, 1, 2, 127, 128, 255, 256})
	nums("BLOCKSIZE", kBlock, []int{-1, 0, 1, 512, 513, 1024, 32256, 32257, 65024, 65536})
	nums("INITIALSIZE", kInitial, []int{-1, 0, 1, 2, 100})
	nums("EXTENDSIZE", kExtend, []int{-1, 0, 1, 50, 65535, 65536})
	nums("MAXREC", kMaxRec, []int{-1, 0, 1, 100}, imm(kAccess, 1), imm(kRecl, 10))
	nums("MAXREC sequential", kMaxRec, []int{100})
	for _, c := range []int{0, 17, 25, 28} {
		nums(fmt.Sprintf("code %d", c), c, []int{50})
	}

	p.section("combinations of keywords, each with what it leaves in the FAB")

	combo := func(title string, kws ...kw) {
		name := fresh()
		p.kase(title, func() {
			p.open(10, append([]kw{file(p, name)}, kws...)...)
			p.fab(10)
			p.closeU(10)
			p.stat(name, 0)
		})
	}
	combo("nothing but a file")
	combo("unformatted", imm(kForm, 2))
	combo("formatted", imm(kForm, 1))
	combo("direct, no RECL", imm(kAccess, 1))
	combo("direct RECL=4", imm(kAccess, 1), imm(kRecl, 4))
	combo("direct RECL=4 formatted", imm(kAccess, 1), imm(kRecl, 4), imm(kForm, 1))
	combo("direct RECL=4 variable", imm(kAccess, 1), imm(kRecl, 4), imm(kRecType, 2))
	combo("direct RECL=4 segmented", imm(kAccess, 1), imm(kRecl, 4), imm(kRecType, 3))
	combo("append", imm(kAccess, 4))
	combo("keyed", imm(kAccess, 3))
	combo("segmented formatted", imm(kRecType, 3), imm(kForm, 1))
	combo("segmented unformatted", imm(kRecType, 3), imm(kForm, 2))
	combo("stream unformatted", imm(kRecType, 4), imm(kForm, 2))
	combo("relative, no RECL", imm(kOrg, 2))
	combo("relative RECL=10", imm(kOrg, 2), imm(kRecl, 10))
	combo("relative RECL=10 unformatted", imm(kOrg, 2), imm(kRecl, 10), imm(kForm, 2))
	combo("relative RECL=10 direct", imm(kOrg, 2), imm(kRecl, 10), imm(kAccess, 1))
	combo("relative RECL=10 variable", imm(kOrg, 2), imm(kRecl, 10), imm(kRecType, 2))
	combo("relative RECL=10 stream", imm(kOrg, 2), imm(kRecl, 10), imm(kRecType, 4))
	combo("relative RECL=10 MAXREC=5", imm(kOrg, 2), imm(kRecl, 10), imm(kMaxRec, 5))
	combo("relative RECL=11 BLOCKSIZE=1024", imm(kOrg, 2), imm(kRecl, 11), imm(kBlock, 1024))
	combo("indexed", imm(kOrg, 3), imm(kRecl, 10))
	combo("readonly, new", imm(kReadOnly, 1))
	combo("readonly, unknown", imm(kReadOnly, 1), imm(kStatus, 4))
	combo("readonly, scratch", imm(kReadOnly, 1), imm(kStatus, 3))
	combo("scratch with a file", imm(kStatus, 3))
	combo("scratch, keep", imm(kStatus, 3), imm(kDispose, 1))
	combo("scratch, print", imm(kStatus, 3), imm(kDispose, 3))
	combo("nospanblocks", imm(kNoSpan, 1))
	combo("shared", imm(kShared, 1))
	combo("readonly, shared, delete", imm(kStatus, 4), imm(kDispose, 2), imm(kShared, 1))
	combo("CC LIST unformatted", imm(kCC, 2), imm(kForm, 2))
	combo("CC FORTRAN unformatted", imm(kCC, 1), imm(kForm, 2))
	combo("BLANK unformatted", imm(kBlank, 2), imm(kForm, 2))
	combo("key without indexed", ref(kKey, mLongRef, "VAL"))
	combo("default file", text(p, kDefault, "SYS$LOGIN:.XYZ"))

	p.kase("no file, a default file", func() {
		p.open(10, text(p, kDefault, ".XYZ"))
		p.fab(10)
		p.inq(10)
		p.closeU(10, imm(kDispose, 2))
	})
	p.kase("signaled, not returned: INCRECLEN", func() {
		p.openS(10, imm(kRecType, 1))
	})
	p.kase("signaled, not returned: invalid unit", func() {
		p.openS(120)
	})
	p.kase("signaled: unit -1 opened", func() {
		p.openS(-1)
	})
	p.kase("a bad file name, no ERR", func() {
		p.openS(10, file(p, "A[B"))
	})
	p.kase("a missing device", func() {
		p.open(10, file(p, "NOSUCH:[X]Y.DAT"))
	})
	p.kase("OLD and missing, no ERR", func() {
		p.openS(10, file(p, "MISSING.DAT"), imm(kStatus, 1))
	})

	p.section("USEROPEN: what a program's routine is given")

	for mode, title := range []string{"$CREATE and $CONNECT", "$OPEN and $CONNECT", "fails with RMS$_FNF, no call", "$CREATE, no $CONNECT", "RFM changed to STM first"} {
		mode, title := mode, title
		name := fresh()
		p.kase("USEROPEN "+title, func() {
			p.emit("MOVL\t#%d,UOMODE", mode)
			kws := []kw{file(p, name), addr(kUserOpen, "UOLOG")}
			if mode == 1 {
				p.open(11, file(p, name))
				p.closeU(11)
				kws = append(kws, imm(kStatus, 1))
			}
			p.open(10, kws...)
			p.closeU(10)
			p.stat(name, 0)
		})
	}

	uo := func(title string, kws ...kw) {
		p.kase("USEROPEN "+title, func() {
			p.emit("CLRL\tUOMODE")
			p.open(10, append([]kw{addr(kUserOpen, "UOLOG")}, kws...)...)
			p.closeU(10, imm(kDispose, 2))
		})
	}
	uo("defaults")
	uo("a file and a default", file(p, "UO.DAT"), text(p, kDefault, ".XYZ"))
	uo("unformatted", imm(kForm, 2))
	uo("direct", imm(kAccess, 1), imm(kRecl, 4))
	uo("direct formatted", imm(kAccess, 1), imm(kRecl, 40), imm(kForm, 1))
	uo("append", imm(kAccess, 4))
	uo("relative", imm(kOrg, 2), imm(kRecl, 10))
	uo("relative direct", imm(kOrg, 2), imm(kRecl, 10), imm(kAccess, 1), imm(kMaxRec, 9))
	uo("fixed", imm(kRecType, 1), imm(kRecl, 10))
	uo("segmented", imm(kRecType, 3), imm(kForm, 2))
	uo("stream", imm(kRecType, 4))
	uo("stream cr", imm(kRecType, 5))
	uo("stream lf", imm(kRecType, 6))
	uo("CC LIST", imm(kCC, 2))
	uo("CC NONE", imm(kCC, 3))
	uo("scratch", imm(kStatus, 3))
	uo("unknown", imm(kStatus, 4))
	uo("shared", imm(kShared, 1))
	uo("nospanblocks", imm(kNoSpan, 1))
	uo("the sizes", imm(kInitial, 7), imm(kExtend, 9), imm(kBlock, 1024), imm(kBufCount, 3))
	uo("code 0", imm(0, 7))
	uo("code 17", imm(17, 7))
	uo("code 25", imm(25, 7))
	uo("code 27", imm(27, 7))
	uo("code 28", imm(28, 7))
	uo("unit 5", imm(1, 5))
	uo("unit 6", imm(1, 6))
	uo("DISPOSE delete", imm(kDispose, 2))

	p.section("STATUS against files that exist, and attributes taken from them")

	p.kase("OLD takes a file's attributes", func() {
		p.open(11, file(p, "OLD1.DAT"), imm(kRecType, 1), imm(kRecl, 20), imm(kForm, 2))
		p.closeU(11)
		p.open(10, file(p, "OLD1.DAT"), imm(kStatus, 1))
		p.fab(10)
		p.inq(10)
	})
	mismatch := func(title string, kws ...kw) {
		p.kase("OLD1.DAT (FIX 20 unformatted), "+title, func() {
			p.open(10, append([]kw{file(p, "OLD1.DAT"), imm(kStatus, 1)}, kws...)...)
			p.fab(10)
			p.closeU(10)
		})
	}
	mismatch("formatted", imm(kForm, 1))
	mismatch("unformatted", imm(kForm, 2))
	mismatch("RECL=5", imm(kRecl, 5))
	mismatch("RECL=20", imm(kRecl, 20))
	mismatch("RECL=99", imm(kRecl, 99))
	mismatch("variable", imm(kRecType, 2))
	mismatch("fixed", imm(kRecType, 1))
	mismatch("segmented", imm(kRecType, 3))
	mismatch("relative", imm(kOrg, 2), imm(kRecl, 20))
	mismatch("direct", imm(kAccess, 1))
	mismatch("direct RECL=5", imm(kAccess, 1), imm(kRecl, 5))
	mismatch("CC LIST", imm(kCC, 2))
	mismatch("append", imm(kAccess, 4))
	mismatch("readonly", imm(kReadOnly, 1))
	p.kase("NEW over a file: a new version", func() {
		p.open(10, file(p, "VER.DAT"), imm(kStatus, 2))
		p.inq(10)
		p.closeU(10)
		p.open(10, file(p, "VER.DAT"), imm(kStatus, 2))
		p.inq(10)
	})
	p.kase("UNKNOWN finds it, then makes one", func() {
		p.open(10, file(p, "VER.DAT"), imm(kStatus, 4))
		p.inq(10)
		p.closeU(10)
		p.open(10, file(p, "UNK.DAT"), imm(kStatus, 4))
		p.inq(10)
	})
	p.kase("no STATUS, the file exists", func() {
		p.open(10, file(p, "VER.DAT"))
		p.inq(10)
	})
	p.kase("OLD with a version that's missing", func() {
		p.open(10, file(p, "VER.DAT;99"), imm(kStatus, 1))
	})
	p.kase("a directory's name", func() {
		p.open(10, file(p, "[NOSUCH]VER.DAT"), imm(kStatus, 2))
	})
	p.kase("two units, one file, not shared", func() {
		p.open(10, file(p, "TWO.DAT"), imm(kStatus, 4))
		p.open(11, file(p, "TWO.DAT"), imm(kStatus, 1))
	})
	p.kase("two units, one file, shared", func() {
		p.open(10, file(p, "TWO.DAT"), imm(kStatus, 1), imm(kShared, 1))
		p.fab(10)
		p.open(11, file(p, "TWO.DAT"), imm(kStatus, 1), imm(kShared, 1))
		p.fab(11)
	})
	p.kase("two units, one file, readonly", func() {
		p.open(10, file(p, "TWO.DAT"), imm(kStatus, 1), imm(kReadOnly, 1))
		p.fab(10)
		p.open(11, file(p, "TWO.DAT"), imm(kStatus, 1), imm(kReadOnly, 1))
		p.fab(11)
		p.wrF(10, "A READONLY WRITE")
		p.rdF(10, 8)
	})

	p.section("OPEN on a connected unit")

	p.kase("the same file, a new BLANK", func() {
		p.open(10, file(p, "CONN1.DAT"), imm(kStatus, 4))
		p.inq(10)
		p.open(10, file(p, "CONN1.DAT"), imm(kBlank, 2))
		p.inq(10)
	})
	p.kase("the same file, no name", func() {
		p.open(10, file(p, "CONN1.DAT"), imm(kStatus, 4))
		p.open(10, imm(kBlank, 2))
		p.inq(10)
	})
	p.kase("the same file, a new FORM", func() {
		p.open(10, file(p, "CONN1.DAT"), imm(kStatus, 4))
		p.open(10, file(p, "CONN1.DAT"), imm(kForm, 2))
		p.inq(10)
	})
	p.kase("the same file, a new RECL", func() {
		p.open(10, file(p, "CONN1.DAT"), imm(kStatus, 4))
		p.open(10, file(p, "CONN1.DAT"), imm(kRecl, 40))
		p.inq(10)
	})
	p.kase("the same file, a new DISPOSE", func() {
		p.open(10, file(p, "CONN2.DAT"), imm(kStatus, 4))
		p.open(10, file(p, "CONN2.DAT"), imm(kDispose, 2))
		p.closeU(10)
		p.stat("CONN2.DAT", 0)
	})
	p.kase("the same file, STATUS='NEW'", func() {
		p.open(10, file(p, "CONN1.DAT"), imm(kStatus, 4))
		p.open(10, file(p, "CONN1.DAT"), imm(kStatus, 2))
		p.inq(10)
	})
	p.kase("another file: the first closes by its disposition", func() {
		p.open(10, file(p, "CONN3.DAT"), imm(kStatus, 4), imm(kDispose, 2))
		p.open(10, file(p, "CONN4.DAT"), imm(kStatus, 4))
		p.inq(10)
		p.closeU(10)
		p.stat("CONN3.DAT", 0)
		p.stat("CONN4.DAT", 0)
	})
	p.kase("another file that can't open: the first stays?", func() {
		p.open(10, file(p, "CONN5.DAT"), imm(kStatus, 4))
		p.open(10, file(p, "MISSING.DAT"), imm(kStatus, 1))
		p.inq(10)
	})
	p.kase("after an implicit open", func() {
		p.wrF(12, "IMPLICIT")
		p.inq(12)
		p.open(12, imm(kBlank, 2))
		p.inq(12)
		p.open(12, file(p, "CONN6.DAT"))
		p.inq(12)
	})

	p.section("CLOSE")

	for v, title := range []string{"", "KEEP", "DELETE", "PRINT", "SUBMIT", "PRINT/DELETE", "SUBMIT/DELETE"} {
		if v == 0 {
			continue
		}
		v, title := v, title
		name := fresh()
		p.kase("CLOSE DISPOSE="+title, func() {
			p.open(10, file(p, name), imm(kStatus, 4))
			p.wrF(10, "$ EXIT")
			p.closeU(10, imm(kDispose, v))
			p.stat(name, 0)
		})
		name = fresh()
		p.kase("OPEN DISPOSE="+title+", no ERR, a plain CLOSE", func() {
			p.openS(10, file(p, name), imm(kStatus, 4), imm(kDispose, v))
			p.wrF(10, "$ EXIT")
			p.closeS(10)
			p.stat(name, 0)
		})
		name = fresh()
		p.kase("CLOSE '"+title+"' as text", func() {
			p.open(10, file(p, name), imm(kStatus, 4))
			p.wrF(10, "$ EXIT")
			p.closeU(10, text(p, kDispose, title))
			p.stat(name, 0)
		})
	}

	close2 := func(title string, open []kw, cl ...kw) {
		name := fresh()
		p.kase(title, func() {
			p.open(10, append([]kw{file(p, name)}, open...)...)
			p.closeU(10, cl...)
			p.stat(name, 0)
		})
	}
	close2("OPEN DELETE, CLOSE KEEP", []kw{imm(kDispose, 2)}, imm(kDispose, 1))
	close2("OPEN KEEP, CLOSE DELETE", []kw{imm(kDispose, 1)}, imm(kDispose, 2))
	close2("SCRATCH, CLOSE KEEP", []kw{imm(kStatus, 3)}, imm(kDispose, 1))
	close2("SCRATCH, CLOSE PRINT", []kw{imm(kStatus, 3)}, imm(kDispose, 3))
	close2("READONLY, CLOSE DELETE", []kw{imm(kStatus, 4)}, imm(kDispose, 2))
	close2("CLOSE by STATUS's code", []kw{imm(kStatus, 4)}, imm(kStatus, 2))
	close2("CLOSE bad text", []kw{imm(kStatus, 4)}, text(p, kDispose, "OTHER"))
	close2("CLOSE out of range", []kw{imm(kStatus, 4)}, imm(kDispose, 7))
	close2("CLOSE RECL", []kw{imm(kStatus, 4)}, imm(kRecl, 5))
	p.kase("READONLY file, CLOSE DELETE", func() {
		p.open(11, file(p, "RO1.DAT"), imm(kStatus, 4))
		p.closeU(11)
		p.open(10, file(p, "RO1.DAT"), imm(kStatus, 1), imm(kReadOnly, 1))
		p.fab(10)
		p.closeU(10, imm(kDispose, 2))
		p.stat("RO1.DAT", 0)
	})
	p.kase("READONLY, OPEN DELETE", func() {
		p.open(11, file(p, "RO2.DAT"), imm(kStatus, 4))
		p.closeU(11)
		p.open(10, file(p, "RO2.DAT"), imm(kStatus, 1), imm(kReadOnly, 1), imm(kDispose, 2))
		p.closeU(10)
		p.stat("RO2.DAT", 0)
	})
	p.kase("CLOSE a unit not open", func() { p.closeU(19) })
	p.kase("CLOSE a unit not open, no ERR", func() { p.closeS(19) })
	p.kase("CLOSE an invalid unit", func() { p.closeU(120) })
	p.kase("CLOSE an invalid unit, no ERR", func() { p.closeS(120) })
	p.kase("CLOSE twice", func() {
		p.open(10, file(p, "TWICE.DAT"), imm(kStatus, 4))
		p.closeU(10)
		p.closeU(10)
	})
	p.kase("scratch's name, and INQUIRE of it", func() {
		p.open(10, imm(kStatus, 3))
		p.fab(10)
		p.inq(10)
		p.wrF(10, "SCRATCH")
		p.call("FOR$REWIND", true, "#10")
		p.rdF(10, 8)
	})

	p.section("INQUIRE")

	p.kase("an unopened unit", func() { p.inq(19) })
	p.kase("an invalid unit", func() { p.inq(120) })
	p.kase("unit 5", func() { p.inq(5) })
	p.kase("unit 6", func() { p.inq(6) })
	p.kase("a missing file", func() { p.inqF("MISSING.DAT") })
	p.kase("a file that exists, closed", func() { p.inqF("OLD1.DAT") })
	p.kase("a file by a wildcard", func() { p.inqF("OLD*.DAT") })
	p.kase("a file on a missing device", func() { p.inqF("NOSUCH:[X]Y.DAT") })
	p.kase("a bad file name", func() { p.inqF("A[B") })
	p.kase("a file that's open", func() {
		p.open(10, file(p, "INQ1.DAT"), imm(kStatus, 4))
		p.inqF("INQ1.DAT")
		p.inqF("[]INQ1.DAT;")
	})
	p.kase("neither unit nor file", func() {
		p.call("FOR$INQUIRE", true, errFlag[0], "#^X0520", "&IQL")
	})
	p.kase("both unit and file", func() {
		p.open(10, file(p, "INQ1.DAT"), imm(kStatus, 4))
		p.call("FOR$INQUIRE", true, unit(19)[0], errFlag[0], "#^X070E", "&"+p.text("INQ1.DAT"), "#^X0520", "&IQL")
		p.emit("PUSHL\tIQL")
		p.emit("PUSHAB\t%s", p.text("NUMBER"))
		p.emit("CALLS\t#2,L^SHOW")
	})
	connected := func(title string, kws ...kw) {
		p.kase("connected: "+title, func() {
			p.open(10, append([]kw{imm(kDispose, 2)}, kws...)...)
			p.inq(10)
		})
	}
	connected("defaults")
	connected("unformatted", imm(kForm, 2))
	connected("direct", imm(kAccess, 1), imm(kRecl, 4))
	connected("direct formatted", imm(kAccess, 1), imm(kRecl, 40), imm(kForm, 1))
	connected("append", imm(kAccess, 4))
	connected("relative", imm(kOrg, 2), imm(kRecl, 10))
	connected("relative direct", imm(kOrg, 2), imm(kRecl, 10), imm(kAccess, 1))
	connected("fixed", imm(kRecType, 1), imm(kRecl, 10))
	connected("segmented", imm(kRecType, 3), imm(kForm, 2))
	connected("stream", imm(kRecType, 4))
	connected("stream cr", imm(kRecType, 5))
	connected("stream lf", imm(kRecType, 6))
	connected("CC LIST", imm(kCC, 2))
	connected("CC NONE", imm(kCC, 3))
	connected("BLANK NULL", imm(kBlank, 1))
	connected("BLANK ZERO", imm(kBlank, 2))
	connected("scratch", imm(kStatus, 3))
	connected("shared", imm(kShared, 1))
	connected("RECL 133", imm(kRecl, 133))
	p.kase("connected: readonly", func() {
		p.open(10, file(p, "OLD1.DAT"), imm(kStatus, 1), imm(kReadOnly, 1))
		p.inq(10)
	})
	p.kase("narrow places: a word, a byte, a short text", func() {
		p.open(10, imm(kAccess, 1), imm(kRecl, 4), imm(kDispose, 2))
		p.emit("MOVL\t#^X12345678,IQL")
		p.call("FOR$INQUIRE", true, unit(10)[0], fmt.Sprintf("#^X%02X29", mWordRef), "&IQL")
		p.emit("PUSHL\tIQL\n\tPUSHAB\t%s\n\tCALLS\t#2,L^SHOW", p.text("RECL word"))
		p.emit("MOVL\t#^X12345678,IQL")
		p.call("FOR$INQUIRE", true, unit(10)[0], fmt.Sprintf("#^X%02X29", mByteRef), "&IQL")
		p.emit("PUSHL\tIQL\n\tPUSHAB\t%s\n\tCALLS\t#2,L^SHOW", p.text("RECL byte"))
		p.emit("MOVL\t#^X12345678,IQL")
		p.call("FOR$INQUIRE", true, unit(10)[0], fmt.Sprintf("#^X%02X1E", mWordRef), "&IQL")
		p.emit("PUSHL\tIQL\n\tPUSHAB\t%s\n\tCALLS\t#2,L^SHOW", p.text("EXIST word"))
		p.emit("MOVL\t#^X12345678,IQL")
		p.call("FOR$INQUIRE", true, unit(10)[0], fmt.Sprintf("#^X%02X1E", mByteRef), "&IQL")
		p.emit("PUSHL\tIQL\n\tPUSHAB\t%s\n\tCALLS\t#2,L^SHOW", p.text("EXIST byte"))
		p.emit("MOVC5\t#0,IQT,#^A/*/,#64,IQT")
		p.call("FOR$INQUIRE", true, unit(10)[0], "#^X0423", "&SHORTD")
		p.emit("PUSHL\t#8\n\tPUSHAB\tIQT\n\tCALLS\t#2,L^HEXS")
		p.emit("MOVC5\t#0,IQT,#^A/*/,#64,IQT")
		p.call("FOR$INQUIRE", true, unit(10)[0], "#^X0722", "&IQD")
		p.emit("PUSHL\t#24\n\tPUSHAB\tIQT\n\tCALLS\t#2,L^HEXS")
	})
	p.kase("NEXTREC moves", func() {
		p.open(10, imm(kAccess, 1), imm(kRecl, 4), imm(kDispose, 2))
		p.inq(10)
		p.wrDU(10, 3, 16)
		p.inq(10)
		p.rdDU(10, 1, 16)
		p.inq(10)
	})
	p.kase("after an implicit unformatted open", func() {
		p.wrU(13, 4)
		p.inq(13)
		p.closeU(13, imm(kDispose, 2))
	})
	p.kase("after an implicit formatted open", func() {
		p.wrF(13, "TEXT")
		p.inq(13)
		p.closeU(13, imm(kDispose, 2))
	})

	p.section("default names, and the FORnnn logical names FOR81.COM defines")

	for _, u := range []int{0, 1, 5, 6, 11, 99, 100, 119} {
		u := u
		p.kase(fmt.Sprintf("USEROPEN unit %d", u), func() {
			p.emit("CLRL\tUOMODE")
			p.open(u, addr(kUserOpen, "UOLOG"))
			p.inq(u)
			if u != 5 && u != 6 {
				p.closeU(u, imm(kDispose, 2))
			} else {
				p.closeU(u)
			}
		})
	}
	for _, u := range []int{-4, -3, -2, -1} {
		u := u
		p.kase(fmt.Sprintf("implicit unit %d", u), func() {
			p.wrF(u, " LINE")
		})
	}

	p.section("unformatted sequential I/O: segments")

	seg := func(title string, kws []kw, sizes ...int) {
		name := fresh()
		p.kase(title, func() {
			p.open(10, append([]kw{file(p, name), imm(kForm, 2), imm(kStatus, 4)}, kws...)...)
			p.fab(10)
			for _, n := range sizes {
				p.wrU(10, n)
			}
			p.closeU(10)
			p.stat(name, 1)
		})
	}
	seg("segmented small", nil, 1, 2, 3, 4, 5, 6, 7, 8)
	seg("segmented zero", nil, 0)
	seg("segmented around 2044", nil, 2040, 2041, 2042, 2043, 2044, 2045, 2046, 2047, 2048)
	seg("segmented around 4092", nil, 4088, 4092, 4096)
	seg("segmented 8000", nil, 8000)
	seg("segmented 32000", nil, 32000)
	seg("segmented RECL=25 (100 bytes)", []kw{imm(kRecl, 25)}, 50, 100, 101, 400)
	seg("segmented RECL=200", []kw{imm(kRecl, 200)}, 800, 801)
	seg("variable", []kw{imm(kRecType, 2)}, 1, 2, 3, 2048)
	seg("variable RECL=25", []kw{imm(kRecType, 2), imm(kRecl, 25)}, 100, 101)
	seg("variable 8000", []kw{imm(kRecType, 2)}, 8000)
	seg("fixed RECL=25", []kw{imm(kRecType, 1), imm(kRecl, 25)}, 50, 100, 101)
	seg("stream", []kw{imm(kRecType, 4)}, 10)
	seg("nospanblocks", []kw{imm(kNoSpan, 1)}, 600, 600)
	seg("segmented bufferless", []kw{imm(kBlock, 1024), imm(kBufCount, 2)}, 1500)

	p.kase("many items in a record", func() {
		p.open(10, file(p, "ITEMS.DAT"), imm(kForm, 2), imm(kStatus, 4))
		p.wrU(10, 4, 4, 4)
		p.wrU(10, 2000, 2000, 2000)
		p.wrU(10)
		p.call("FOR$REWIND", true, "#10")
		p.rdU(10, 12)
		p.rdU(10, 6000)
		p.rdU(10, 4)
		p.rdU(10, 4)
		p.closeU(10)
		p.stat("ITEMS.DAT", 1)
	})
	p.kase("reading: shorter, longer, past the end", func() {
		p.open(10, file(p, "RDSEG.DAT"), imm(kForm, 2), imm(kStatus, 4))
		p.wrU(10, 8)
		p.wrU(10, 8)
		p.wrU(10, 8)
		p.wrU(10, 3000)
		p.call("FOR$REWIND", true, "#10")
		p.rdU(10, 4)
		p.rdU(10, 8)
		p.rdU(10, 12)
		p.rdU(10, 3000)
		p.rdU(10, 8)
		p.rdU(10, 8)
		p.closeU(10)
	})
	p.kase("reading: two items, with ERRSNS", func() {
		p.open(10, file(p, "RDSEG.DAT"), imm(kStatus, 1))
		p.fab(10)
		p.stmt(func() {
			p.call("FOR$READ_SU", false, "#10")
			p.rdItem(4)
			p.rdItem(4)
			p.rdItem(4)
			p.call("FOR$IO_END", true)
			p.showRd(4)
		})
	})
	p.kase("BACKSPACE over segments, then overwrite", func() {
		p.open(10, file(p, "BKSEG.DAT"), imm(kForm, 2), imm(kStatus, 4))
		p.wrU(10, 8)
		p.wrU(10, 5000)
		p.wrU(10, 12)
		p.unitCall("FOR$BACKSPACE", 10)
		p.unitCall("FOR$BACKSPACE", 10)
		p.rdU(10, 5000)
		p.unitCall("FOR$BACKSPACE", 10)
		p.wrU(10, 16)
		p.closeU(10)
		p.stat("BKSEG.DAT", 1)
	})
	p.kase("a formatted statement on an unformatted unit and back", func() {
		p.open(10, file(p, "MIX.DAT"), imm(kForm, 2), imm(kStatus, 4))
		p.wrF(10, "TEXT")
		p.closeU(10)
		p.open(10, file(p, "MIX.DAT"), imm(kForm, 1), imm(kStatus, 4))
		p.wrU(10, 8)
		p.closeU(10)
		p.stat("MIX.DAT", 1)
	})
	p.kase("a segmented reader on a file of unsegmented records", func() {
		p.open(10, file(p, "NOTSEG.DAT"), imm(kForm, 2), imm(kRecType, 2), imm(kStatus, 4))
		p.wrU(10, 8)
		p.closeU(10)
		p.open(10, file(p, "NOTSEG.DAT"), imm(kForm, 2), imm(kRecType, 3), imm(kStatus, 1))
		p.fab(10)
		p.rdU(10, 8)
	})

	p.section("REWIND, BACKSPACE, ENDFILE")

	p.kase("formatted: a walk", func() {
		p.open(10, file(p, "WALK.DAT"), imm(kStatus, 4))
		p.wrF(10, " ONE")
		p.wrF(10, " TWO")
		p.wrF(10, " THREE")
		p.unitCall("FOR$BACKSPACE", 10)
		p.rdF(10, 8)
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.unitCall("FOR$BACKSPACE", 10)
		p.unitCall("FOR$BACKSPACE", 10)
		p.unitCall("FOR$BACKSPACE", 10)
		p.rdF(10, 8)
		p.wrF(10, " REPLACED")
		p.closeU(10)
		p.stat("WALK.DAT", 1)
	})
	p.kase("write after read cuts the file", func() {
		p.open(10, file(p, "WALK2.DAT"), imm(kStatus, 4))
		p.wrF(10, " ONE")
		p.wrF(10, " TWO")
		p.wrF(10, " THREE")
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 8)
		p.wrF(10, " NEW")
		p.rdF(10, 8)
		p.closeU(10)
		p.stat("WALK2.DAT", 1)
	})
	p.kase("ENDFILE: reading it, past it, and back", func() {
		p.open(10, file(p, "EOFS.DAT"), imm(kStatus, 4))
		p.wrF(10, " ONE")
		p.unitCall("FOR$ENDFILE", 10)
		p.wrF(10, " TWO")
		p.unitCall("FOR$ENDFILE", 10)
		p.unitCall("FOR$ENDFILE", 10)
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.rdF(10, 8)
		p.unitCall("FOR$BACKSPACE", 10)
		p.rdF(10, 8)
		p.closeU(10)
		p.stat("EOFS.DAT", 1)
	})
	p.kase("ENDFILE unformatted", func() {
		p.open(10, file(p, "EOFU.DAT"), imm(kForm, 2), imm(kStatus, 4))
		p.wrU(10, 8)
		p.unitCall("FOR$ENDFILE", 10)
		p.unitCall("FOR$REWIND", 10)
		p.rdU(10, 8)
		p.rdU(10, 8)
		p.rdU(10, 8)
		p.closeU(10)
		p.stat("EOFU.DAT", 1)
	})
	p.kase("on a direct unit", func() {
		p.open(10, file(p, "DIRW.DAT"), imm(kAccess, 1), imm(kRecl, 4), imm(kStatus, 4))
		p.wrDU(10, 1, 16)
		p.unitCall("FOR$REWIND", 10)
		p.unitCall("FOR$BACKSPACE", 10)
		p.unitCall("FOR$ENDFILE", 10)
		p.closeU(10)
		p.stat("DIRW.DAT", 3)
	})
	p.kase("on a unit not open", func() {
		p.unitCall("FOR$REWIND", 18)
		p.unitCall("FOR$BACKSPACE", 18)
		p.inq(18)
		p.unitCall("FOR$ENDFILE", 18)
		p.inq(18)
		p.closeU(18, imm(kDispose, 2))
	})
	p.kase("on an invalid unit", func() {
		p.unitCall("FOR$REWIND", 120)
		p.unitCall("FOR$BACKSPACE", 120)
		p.unitCall("FOR$ENDFILE", 120)
	})
	p.kase("on a readonly unit", func() {
		p.open(10, file(p, "WALK.DAT"), imm(kStatus, 1), imm(kReadOnly, 1))
		p.rdF(10, 8)
		p.unitCall("FOR$BACKSPACE", 10)
		p.unitCall("FOR$REWIND", 10)
		p.unitCall("FOR$ENDFILE", 10)
	})
	p.kase("on an append unit", func() {
		p.open(10, file(p, "WALK.DAT"), imm(kStatus, 1), imm(kAccess, 4))
		p.fab(10)
		p.wrF(10, " APPENDED")
		p.unitCall("FOR$BACKSPACE", 10)
		p.rdF(10, 12)
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 12)
		p.closeU(10)
		p.stat("WALK.DAT", 1)
	})
	p.kase("on a terminal unit's file", func() {
		p.unitCall("FOR$REWIND", 6)
		p.unitCall("FOR$BACKSPACE", 6)
	})

	p.section("direct access on a sequential file")

	p.kase("unformatted: gaps, past the end, limits, ASV", func() {
		p.open(10, file(p, "DIR1.DAT"), imm(kAccess, 1), imm(kRecl, 2), imm(kStatus, 4), ref(kAssoc, mLongRef, "ASVL"))
		p.fab(10)
		p.asv()
		p.wrDU(10, 1, 8)
		p.asv()
		p.wrDU(10, 4, 8)
		p.asv()
		p.rdDU(10, 2, 8)
		p.asv()
		p.rdDU(10, 3, 8)
		p.rdDU(10, 4, 8)
		p.asv()
		p.rdDU(10, 5, 8)
		p.asv()
		p.rdDU(10, 0, 8)
		p.rdDU(10, -1, 8)
		p.wrDU(10, 0, 8)
		p.wrDU(10, -1, 8)
		p.wrDU(10, 2, 4)
		p.wrDU(10, 3, 12)
		p.rdDU(10, 2, 4)
		p.rdDU(10, 2, 12)
		p.closeU(10)
		p.stat("DIR1.DAT", 3)
	})
	p.kase("formatted: padding and overflow", func() {
		p.open(10, file(p, "DIR2.DAT"), imm(kAccess, 1), imm(kRecl, 11), imm(kForm, 1), imm(kStatus, 4))
		p.fab(10)
		p.wrDO(10, 1, "ABC")
		p.wrDO(10, 2, "ABCDEFGHIJK")
		p.wrDO(10, 3, "ABCDEFGHIJKL")
		p.rdDO(10, 1, 11)
		p.rdDO(10, 2, 11)
		p.rdDO(10, 3, 11)
		p.rdDO(10, 9, 11)
		p.closeU(10)
		p.stat("DIR2.DAT", 3)
	})
	p.kase("odd lengths", func() {
		p.open(10, file(p, "DIR3.DAT"), imm(kAccess, 1), imm(kRecl, 5), imm(kForm, 1), imm(kStatus, 4))
		p.wrDO(10, 1, "AAAAA")
		p.wrDO(10, 2, "BBBBB")
		p.wrDO(10, 5, "CCCCC")
		p.rdDO(10, 2, 5)
		p.rdDO(10, 5, 5)
		p.closeU(10)
		p.stat("DIR3.DAT", 3)
	})
	p.kase("MAXREC", func() {
		p.open(10, file(p, "DIR4.DAT"), imm(kAccess, 1), imm(kRecl, 1), imm(kMaxRec, 3), imm(kStatus, 4))
		p.fab(10)
		p.wrDU(10, 3, 4)
		p.wrDU(10, 4, 4)
		p.rdDU(10, 3, 4)
		p.rdDU(10, 4, 4)
		p.closeU(10)
		p.stat("DIR4.DAT", 2)
		p.open(10, file(p, "DIR4.DAT"), imm(kAccess, 1), imm(kRecl, 1), imm(kMaxRec, 2), imm(kStatus, 1))
		p.fab(10)
		p.rdDU(10, 3, 4)
	})
	p.kase("a sequential statement on a direct unit", func() {
		p.open(10, file(p, "DIR5.DAT"), imm(kAccess, 1), imm(kRecl, 1), imm(kStatus, 4))
		p.wrDU(10, 2, 4)
		p.wrU(10, 4)
		p.rdU(10, 4)
		p.rdDU(10, 2, 4)
		p.rdU(10, 4)
	})
	p.kase("a direct statement on a sequential unit, a file of FIX records", func() {
		p.open(10, file(p, "DIR6.DAT"), imm(kRecType, 1), imm(kRecl, 4), imm(kForm, 2), imm(kStatus, 4))
		p.wrU(10, 4)
		p.wrDU(10, 2, 4)
		p.rdDU(10, 1, 4)
	})
	p.kase("direct on a VAR file", func() {
		p.open(10, file(p, "DIR7.DAT"), imm(kRecType, 2), imm(kStatus, 4))
		p.wrF(10, "TEXT")
		p.closeU(10)
		p.open(10, file(p, "DIR7.DAT"), imm(kAccess, 1), imm(kRecl, 4), imm(kStatus, 1))
		p.rdDU(10, 1, 4)
	})
	p.kase("direct, readonly", func() {
		p.open(10, file(p, "DIR1.DAT"), imm(kAccess, 1), imm(kRecl, 2), imm(kStatus, 1), imm(kReadOnly, 1))
		p.fab(10)
		p.rdDU(10, 1, 8)
		p.wrDU(10, 1, 8)
	})
	p.kase("FIND", func() {
		p.open(10, file(p, "DIR1.DAT"), imm(kAccess, 1), imm(kRecl, 2), imm(kStatus, 1), ref(kAssoc, mLongRef, "ASVL"))
		p.asv()
		p.call("FOR$FIND", true, "#10", "#2")
		p.asv()
		p.inq(10)
		p.rdDU(10, 3, 8)
		p.call("FOR$FIND", true, "#10", "#99")
		p.asv()
		p.call("FOR$FIND", true, "#10", "#0")
		p.asv()
	})
	p.kase("FIND on a sequential unit, and one not open", func() {
		p.open(10, file(p, "WALK.DAT"), imm(kStatus, 1))
		p.call("FOR$FIND", true, "#10", "#2")
		p.call("FOR$FIND", true, "#18", "#2")
	})
	p.kase("the ASV a word", func() {
		p.open(10, imm(kAccess, 1), imm(kRecl, 2), ref(kAssoc, mWordRef, "ASVL"), imm(kDispose, 2))
		p.asv()
		p.wrDU(10, 7, 8)
		p.asv()
	})

	p.section("DEFINE FILE")

	def := func(title, routine string, u, m, n int, then func()) {
		p.kase(title, func() {
			p.call(routine, true, fmt.Sprintf("#%d", u), fmt.Sprintf("#%d", m), fmt.Sprintf("#%d", n), "&ASVL")
			p.asv()
			then()
		})
	}
	def("W, then a direct write", "FOR$DEF_FILE_W", 14, 10, 8, func() {
		p.emit("CLRL\tUOMODE")
		p.inq(14)
		p.wrDU(14, 3, 16)
		p.asv()
		p.fab(14)
		p.inq(14)
		p.closeU(14)
		p.stat("FOR014.DAT", 2)
	})
	def("L", "FOR$DEF_FILE", 15, 10, 8, func() {
		p.wrDU(15, 3, 16)
		p.asv()
		p.fab(15)
		p.inq(15)
		p.closeU(15)
		p.stat("FOR015.DAT", 0)
	})
	def("on an open unit", "FOR$DEF_FILE_W", 10, 10, 8, func() {})
	p.kase("on an open unit, really", func() {
		p.open(10, imm(kDispose, 2))
		p.call("FOR$DEF_FILE_W", true, "#10", "#10", "#8", "&ASVL")
		p.fab(10)
	})
	p.kase("twice on a unit", func() {
		p.call("FOR$DEF_FILE_W", true, "#16", "#10", "#8", "&ASVL")
		p.call("FOR$DEF_FILE_W", true, "#16", "#10", "#4", "&ASVL")
		p.wrDU(16, 1, 16)
		p.fab(16)
		p.closeU(16, imm(kDispose, 2))
	})
	p.kase("an OPEN after one", func() {
		p.call("FOR$DEF_FILE_W", true, "#16", "#10", "#8", "&ASVL")
		p.open(16, imm(kDispose, 2))
		p.fab(16)
	})
	p.kase("limits", func() {
		p.call("FOR$DEF_FILE_W", true, "#120", "#10", "#8", "&ASVL")
		p.call("FOR$DEF_FILE_W", true, "#16", "#0", "#8", "&ASVL")
		p.call("FOR$DEF_FILE_W", true, "#17", "#-1", "#8", "&ASVL")
		p.call("FOR$DEF_FILE_W", true, "#18", "#10", "#0", "&ASVL")
		p.call("FOR$DEF_FILE_W", true, "#19", "#10", "#40000", "&ASVL")
		p.fab(19)
	})

	p.section("relative files")

	p.kase("direct, unformatted: gaps, delete, rewrite", func() {
		p.open(10, file(p, "REL1.DAT"), imm(kOrg, 2), imm(kRecl, 2), imm(kAccess, 1), imm(kStatus, 4), ref(kAssoc, mLongRef, "ASVL"))
		p.fab(10)
		p.inq(10)
		p.wrDU(10, 1, 8)
		p.asv()
		p.wrDU(10, 2, 8)
		p.wrDU(10, 5, 8)
		p.asv()
		p.rdDU(10, 1, 8)
		p.asv()
		p.rdDU(10, 3, 8)
		p.asv()
		p.rdDU(10, 6, 8)
		p.asv()
		p.wrDU(10, 2, 8)
		p.call("FOR$DELETE_D", true, "#10", "#2")
		p.asv()
		p.rdDU(10, 2, 8)
		p.call("FOR$DELETE_D", true, "#10", "#2")
		p.wrDU(10, 2, 8)
		p.rdDU(10, 2, 8)
		p.rwU(10, 8)
		p.call("FOR$DELETE", true, "#10")
		p.call("FOR$DELETE", true, "#10")
		p.call("FOR$UNLOCK", true, "#10")
		p.closeU(10)
		p.stat("REL1.DAT", 3)
	})
	p.kase("sequential: writes follow the last, reads skip what's missing", func() {
		p.open(10, file(p, "REL1.DAT"), imm(kOrg, 2), imm(kStatus, 1))
		p.fab(10)
		p.inq(10)
		p.rdU(10, 8)
		p.rdU(10, 8)
		p.rdU(10, 8)
		p.call("FOR$DELETE", true, "#10")
		p.unitCall("FOR$REWIND", 10)
		p.rdU(10, 8)
		p.rdU(10, 8)
		p.wrU(10, 8)
		p.unitCall("FOR$BACKSPACE", 10)
		p.unitCall("FOR$ENDFILE", 10)
		p.closeU(10)
		p.stat("REL1.DAT", 3)
	})
	p.kase("formatted, FIX, odd", func() {
		p.open(10, file(p, "REL2.DAT"), imm(kOrg, 2), imm(kRecl, 11), imm(kAccess, 1), imm(kForm, 1), imm(kStatus, 4))
		p.fab(10)
		p.wrDO(10, 1, "AAA")
		p.wrDO(10, 3, "CCCCCCCCCCC")
		p.wrDO(10, 4, "DDDDDDDDDDDD")
		p.rdDO(10, 1, 11)
		p.rdDO(10, 3, 11)
		p.rwF(10, "REWRITTEN")
		p.rdDO(10, 3, 11)
		p.closeU(10)
		p.stat("REL2.DAT", 3)
	})
	p.kase("VAR records", func() {
		p.open(10, file(p, "REL3.DAT"), imm(kOrg, 2), imm(kRecl, 40), imm(kRecType, 2), imm(kAccess, 1), imm(kForm, 1), imm(kStatus, 4))
		p.fab(10)
		p.wrDO(10, 1, "A")
		p.wrDO(10, 2, "BBBBBB")
		p.wrDO(10, 4, "DDDD")
		p.rdDO(10, 2, 40)
		p.rwF(10, "LONGER REWRITE")
		p.closeU(10)
		p.stat("REL3.DAT", 3)
	})
	p.kase("MAXREC", func() {
		p.open(10, file(p, "REL4.DAT"), imm(kOrg, 2), imm(kRecl, 2), imm(kAccess, 1), imm(kMaxRec, 3), imm(kStatus, 4))
		p.fab(10)
		p.wrDU(10, 3, 8)
		p.wrDU(10, 4, 8)
		p.rdDU(10, 4, 8)
		p.closeU(10)
		p.stat("REL4.DAT", 2)
		p.open(10, file(p, "REL4.DAT"), imm(kOrg, 2), imm(kAccess, 1), imm(kStatus, 1))
		p.fab(10)
	})
	p.kase("a relative file by the wrong keywords", func() {
		p.open(10, file(p, "REL1.DAT"), imm(kStatus, 1))
		p.fab(10)
		p.closeU(10)
		p.open(10, file(p, "REL1.DAT"), imm(kStatus, 1), imm(kOrg, 1))
		p.closeU(10)
		p.open(10, file(p, "WALK.DAT"), imm(kStatus, 1), imm(kOrg, 2))
		p.closeU(10)
		p.open(10, file(p, "REL1.DAT"), imm(kStatus, 1), imm(kOrg, 2), imm(kRecl, 3))
		p.closeU(10)
		p.open(10, file(p, "REL1.DAT"), imm(kStatus, 1), imm(kOrg, 2), imm(kRecType, 2))
		p.closeU(10)
		p.open(10, file(p, "REL1.DAT"), imm(kStatus, 1), imm(kOrg, 2), imm(kAccess, 4))
		p.closeU(10)
	})
	p.kase("DELETE, REWRITE, UNLOCK on sequential files and units not open", func() {
		p.open(10, file(p, "WALK2.DAT"), imm(kStatus, 1))
		p.rdF(10, 8)
		p.call("FOR$DELETE", true, "#10")
		p.call("FOR$DELETE_D", true, "#10", "#1")
		p.rwF(10, "TEXTX")
		p.call("FOR$UNLOCK", true, "#10")
		p.closeU(10)
		p.call("FOR$DELETE", true, "#18")
		p.call("FOR$UNLOCK", true, "#18")
		p.rwU(18)
	})
	p.kase("REWRITE on a sequential FIX file, same and other lengths", func() {
		p.open(10, file(p, "RWF.DAT"), imm(kRecType, 1), imm(kRecl, 4), imm(kStatus, 4))
		p.wrF(10, "ABCD")
		p.wrF(10, "EFGH")
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 4)
		p.rwF(10, "WXYZ")
		p.rdF(10, 4)
		p.rwF(10, "Q")
		p.closeU(10)
		p.stat("RWF.DAT", 1)
	})
	p.kase("REWRITE on a sequential VAR file", func() {
		p.open(10, file(p, "RWV.DAT"), imm(kStatus, 4))
		p.wrF(10, "ABCD")
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 4)
		p.rwF(10, "WXYZ")
		p.unitCall("FOR$REWIND", 10)
		p.rdF(10, 4)
		p.rwF(10, "WX")
		p.closeU(10)
		p.stat("RWV.DAT", 1)
	})

	p.section("keyed statements and indexed keywords, for their conditions")

	p.kase("keyed READ on a sequential unit", func() {
		p.open(10, file(p, "WALK2.DAT"), imm(kStatus, 1))
		p.call("FOR$READ_KU", true, "#10", "&"+p.text("KEY"), "#0", "#0")
		p.call("FOR$IO_END", true)
	})
	p.kase("indexed, new, no key", func() {
		p.open(10, file(p, "IDX1.DAT"), imm(kOrg, 3), imm(kRecl, 10), imm(kStatus, 2))
		p.fab(10)
	})

	p.section("last, since a wait for a record could hang: a record locked by another unit")

	p.kase("records locked between two units", func() {
		p.open(10, file(p, "REL5.DAT"), imm(kOrg, 2), imm(kRecl, 2), imm(kAccess, 1), imm(kStatus, 4), imm(kShared, 1))
		p.fab(10)
		p.wrDU(10, 1, 8)
		p.wrDU(10, 2, 8)
		p.open(11, file(p, "REL5.DAT"), imm(kOrg, 2), imm(kAccess, 1), imm(kStatus, 1), imm(kShared, 1))
		p.fab(11)
		p.rdDU(10, 1, 8)
		p.rdDU(11, 1, 8)
		p.rdDU(11, 2, 8)
		p.rdDU(10, 2, 8)
		p.call("FOR$UNLOCK", true, "#11")
		p.rdDU(10, 2, 8)
		p.call("FOR$UNLOCK", true, "#10")
		p.call("FOR$DELETE_D", true, "#10", "#2")
		p.rdDU(11, 2, 8)
	})

	return p
}

// The RMS program's cases.
func rmsCases() *prog {
	p := newProg("FOR81R")

	fresh := 0
	name := func(prefix string) string {
		fresh++
		return fmt.Sprintf("%s%d.DAT", prefix, fresh)
	}

	// reset readies the FAB and RAB, naming a file.
	reset := func(file string) {
		p.emit("CALLS\t#0,L^RRESET")
		p.emit("MOVAB\t%s,R0", p.text(file))
		p.emit("MOVB\t(R0),FB+FAB$B_FNS")
		p.emit("MOVL\t4(R0),FB+FAB$L_FNA")
	}
	set := func(op, field, val string) { p.emit("%s\t#%s,%s", op, val, field) }
	svc := func(macro, block, target string) {
		p.emit("%s\t%s=%s", macro, block, target)
		p.shownow(macro)
	}
	create := func(file, org, rfm, rat string, mrs, mrn, bks int, fop string) {
		reset(file)
		set("MOVB", "FB+FAB$B_ORG", org)
		set("MOVB", "FB+FAB$B_RFM", rfm)
		set("MOVB", "FB+FAB$B_RAT", rat)
		set("MOVW", "FB+FAB$W_MRS", fmt.Sprint(mrs))
		set("MOVL", "FB+FAB$L_MRN", fmt.Sprint(mrn))
		set("MOVB", "FB+FAB$B_BKS", fmt.Sprint(bks))
		if fop != "" {
			set("BISL2", "FB+FAB$L_FOP", fop)
		}
		svc("$CREATE", "FAB", "FB")
		p.emit("CALLS\t#0,L^SHOWF")
		svc("$CONNECT", "RAB", "RB")
	}
	open := func(file string) {
		reset(file)
		svc("$OPEN", "FAB", "FB")
		p.emit("CALLS\t#0,L^SHOWF")
		svc("$CONNECT", "RAB", "RB")
	}
	closef := func() { svc("$CLOSE", "FAB", "FB") }
	put := func(rec, n int, rop string) {
		p.emit("MOVW\t#%d,RB+RAB$W_RSZ", n)
		p.emit("MOVAB\tBIGB,RB+RAB$L_RBF")
		p.emit("MOVL\t#%s,RB+RAB$L_ROP", rop)
		if rec > 0 || rec < -1 {
			p.emit("MOVL\t#%d,RKEY", rec)
			p.emit("MOVB\t#RAB$C_KEY,RB+RAB$B_RAC")
		} else if rec == 0 {
			p.emit("CLRL\tRKEY")
			p.emit("MOVB\t#RAB$C_KEY,RB+RAB$B_RAC")
		} else {
			p.emit("MOVB\t#RAB$C_SEQ,RB+RAB$B_RAC")
		}
		svc("$PUT", "RAB", "RB")
		p.emit("CALLS\t#0,L^SHOWR")
	}
	get := func(rec int, find bool) {
		p.emit("CLRL\tRB+RAB$L_ROP")
		if rec > 0 {
			p.emit("MOVL\t#%d,RKEY", rec)
			p.emit("MOVB\t#RAB$C_KEY,RB+RAB$B_RAC")
		} else if rec == 0 {
			p.emit("CLRL\tRKEY")
			p.emit("MOVB\t#RAB$C_KEY,RB+RAB$B_RAC")
		} else {
			p.emit("MOVB\t#RAB$C_SEQ,RB+RAB$B_RAC")
		}
		if find {
			svc("$FIND", "RAB", "RB")
			p.emit("CALLS\t#0,L^SHOWR")
			return
		}
		p.emit("MOVW\t#512,RB+RAB$W_USZ")
		p.emit("MOVAB\tRDB,RB+RAB$L_UBF")
		svc("$GET", "RAB", "RB")
		p.emit("CALLS\t#0,L^SHOWG")
	}
	update := func(n int) {
		p.emit("MOVW\t#%d,RB+RAB$W_RSZ", n)
		p.emit("MOVAB\tBIGB,RB+RAB$L_RBF")
		svc("$UPDATE", "RAB", "RB")
		p.emit("CALLS\t#0,L^SHOWR")
	}
	del := func() { svc("$DELETE", "RAB", "RB"); p.emit("CALLS\t#0,L^SHOWR") }
	free := func() { svc("$FREE", "RAB", "RB") }
	rewind := func() { svc("$REWIND", "RAB", "RB") }
	uif := "RAB$M_UIF"
	none := "0"

	p.section("$PARSE with no NAM, as FORRTL's first call is")

	p.kase("$PARSE, FAB$L_NAM 0", func() {
		reset("NONAM.DAT")
		p.emit("CLRL\tFB+FAB$L_NAM")
		p.emit("MOVL\t#FAB$M_DFW,FB+FAB$L_FOP")
		svc("$PARSE", "FAB", "FB")
	})
	p.kase("$PARSE, FAB$L_NAM 0, a bad name", func() {
		reset("A[B")
		p.emit("CLRL\tFB+FAB$L_NAM")
		svc("$PARSE", "FAB", "FB")
	})

	p.section("relative files: the file's layout")

	rel := func(title string, rfm, rat string, mrs, mrn, bks int, f func()) {
		n := name("R")
		p.kase(title, func() {
			create(n, "FAB$C_REL", rfm, rat, mrs, mrn, bks, "")
			if f != nil {
				f()
			}
			closef()
			p.stat(n, 3)
		})
	}
	rel("REL FIX 10, empty", "FAB$C_FIX", "0", 10, 0, 0, nil)
	rel("REL FIX 10, records 1 2 5", "FAB$C_FIX", "0", 10, 0, 0, func() {
		put(1, 10, none)
		put(2, 10, none)
		put(5, 10, none)
	})
	rel("REL FIX 11 (odd)", "FAB$C_FIX", "0", 11, 0, 0, func() {
		put(1, 11, none)
		put(2, 11, none)
	})
	rel("REL FIX 1", "FAB$C_FIX", "0", 1, 0, 0, func() { put(1, 1, none); put(2, 1, none) })
	rel("REL FIX 512", "FAB$C_FIX", "0", 512, 0, 0, func() { put(1, 512, none); put(3, 512, none) })
	rel("REL FIX 10, RAT FTN", "FAB$C_FIX", "FAB$M_FTN", 10, 0, 0, func() { put(1, 10, none) })
	rel("REL FIX 10, RAT CR", "FAB$C_FIX", "FAB$M_CR", 10, 0, 0, func() { put(1, 10, none) })
	rel("REL FIX 10, BKS 1", "FAB$C_FIX", "0", 10, 0, 1, func() { put(1, 10, none) })
	rel("REL FIX 10, BKS 4", "FAB$C_FIX", "0", 10, 0, 4, func() { put(1, 10, none); put(100, 10, none) })
	rel("REL FIX 10, MRN 3", "FAB$C_FIX", "0", 10, 3, 0, func() { put(3, 10, none); put(4, 10, none); get(4, false) })
	rel("REL FIX 10, past the first bucket", "FAB$C_FIX", "0", 10, 0, 0, func() {
		put(46, 10, none)
		put(47, 10, none)
		put(1000, 10, none)
		get(47, false)
	})
	rel("REL FIX 100, a bucket's fill", "FAB$C_FIX", "0", 100, 0, 0, func() {
		for i := 1; i <= 12; i++ {
			put(i, 100, none)
		}
	})
	rel("REL VAR 40", "FAB$C_VAR", "0", 40, 0, 0, func() {
		put(1, 1, none)
		put(2, 6, none)
		put(4, 40, none)
		put(5, 0, none)
		put(6, 41, none)
		get(2, false)
		get(4, false)
		get(5, false)
	})
	rel("REL VAR 0 MRS", "FAB$C_VAR", "0", 0, 0, 0, func() { put(1, 10, none) })
	rel("REL VFC 40", "FAB$C_VFC", "0", 40, 0, 0, func() { put(1, 5, none) })
	rel("REL FIX size errors", "FAB$C_FIX", "0", 10, 0, 0, func() {
		put(1, 9, none)
		put(1, 11, none)
		put(1, 0, none)
		put(0, 10, none)
	})
	rel("REL FIX 0", "FAB$C_FIX", "0", 0, 0, 0, nil)
	rel("REL STM", "FAB$C_STM", "0", 10, 0, 0, nil)
	rel("REL, FIX 40000", "FAB$C_FIX", "0", 40000, 0, 0, nil)

	p.section("relative files: every operation")

	opsFile := name("R")
	p.kase("build, and all the statuses", func() {
		create(opsFile, "FAB$C_REL", "FAB$C_FIX", "0", 10, 0, 0, "")
		put(1, 10, none)
		put(2, 10, none)
		put(5, 10, none)
		put(1, 10, none)
		put(1, 10, uif)
		put(3, 10, uif)
		put(-1, 10, none)
		put(-1, 10, none)
		rewind()
		get(-1, false)
		get(-1, false)
		get(4, false)
		get(9, false)
		get(99, false)
		get(0, false)
		get(2, true)
		get(-1, false)
		get(1, false)
		update(10)
		update(9)
		get(2, false)
		del()
		del()
		get(2, false)
		rewind()
		get(-1, false)
		get(-1, false)
		get(-1, false)
		put(2, 10, none)
		get(2, false)
		free()
		get(-1, false)
		get(-1, false)
		get(-1, false)
		get(-1, false)
		get(-1, false)
		get(-1, false)
		closef()
		p.stat(opsFile, 3)
	})
	p.kase("reopen: attributes, and statuses by RFA", func() {
		open(opsFile)
		get(5, false)
		p.emit("MOVB\t#RAB$C_RFA,RB+RAB$B_RAC")
		p.emit("CLRL\tRB+RAB$L_ROP")
		svc("$GET", "RAB", "RB")
		p.emit("CALLS\t#0,L^SHOWG")
		closef()
	})
	p.kase("$TRUNCATE on a relative file", func() {
		open(opsFile)
		get(2, false)
		svc("$TRUNCATE", "RAB", "RB")
		closef()
	})
	p.kase("a locked record: two streams", func() {
		reset(opsFile)
		p.emit("BISB2\t#FAB$M_SHRPUT!FAB$M_SHRGET!FAB$M_SHRDEL!FAB$M_SHRUPD,FB+FAB$B_SHR")
		p.emit("MOVB\t#FAB$M_GET!FAB$M_PUT!FAB$M_DEL!FAB$M_UPD,FB+FAB$B_FAC")
		svc("$OPEN", "FAB", "FB")
		svc("$CONNECT", "RAB", "RB")
		get(1, false)
		closef()
	})

	p.section("sequential FIX: keyed $PUT, $GET, and padding")

	fix := func(title string, mrs int, f func()) {
		n := name("S")
		p.kase(title, func() {
			create(n, "FAB$C_SEQ", "FAB$C_FIX", "0", mrs, 0, 0, "")
			if f != nil {
				f()
			}
			closef()
			p.stat(n, 3)
		})
	}
	fix("FIX 16, sequential", 16, func() { put(-1, 16, none); put(-1, 16, none) })
	fix("FIX 5, sequential", 5, func() { put(-1, 5, none); put(-1, 5, none); put(-1, 5, none) })
	fix("FIX 7, 100 sequential, across blocks", 7, func() {
		for i := 0; i < 100; i++ {
			put(-1, 7, none)
		}
	})
	fix("FIX 513, sequential, across blocks", 513, func() { put(-1, 513, none); put(-1, 513, none) })
	fix("FIX 16, keyed: UIF off, on, gaps, past the end", 16, func() {
		put(1, 16, none)
		put(1, 16, none)
		put(1, 16, uif)
		put(2, 16, uif)
		put(5, 16, uif)
		put(9, 16, none)
		put(0, 16, uif)
		put(3, 8, uif)
		get(3, false)
		get(4, false)
		get(5, false)
		get(9, false)
		get(10, false)
		get(99, false)
		get(0, false)
	})
	fix("FIX 5, keyed", 5, func() {
		put(1, 5, uif)
		put(3, 5, uif)
		get(1, false)
		get(2, false)
		get(3, false)
		get(4, false)
	})
	fix("FIX 16, keyed then sequential", 16, func() {
		put(4, 16, uif)
		put(-1, 16, none)
		put(2, 16, uif)
		get(-1, false)
		get(-1, false)
		rewind()
		get(-1, false)
		get(2, true)
		update(16)
		get(3, false)
		update(15)
	})
	fix("FIX 16, $TRUNCATE", 16, func() {
		put(-1, 16, none)
		put(-1, 16, none)
		put(-1, 16, none)
		rewind()
		get(-1, false)
		svc("$TRUNCATE", "RAB", "RB")
		get(-1, false)
		put(-1, 16, none)
	})
	fix("FIX 16, $DELETE", 16, func() { put(-1, 16, none); rewind(); get(-1, false); del() })
	n := name("V")
	p.kase("VAR: keyed $PUT and $GET refused", func() {
		create(n, "FAB$C_SEQ", "FAB$C_VAR", "0", 0, 0, 0, "")
		put(-1, 5, none)
		put(1, 5, uif)
		get(1, false)
		closef()
		p.stat(n, 3)
	})
	n2 := name("V")
	p.kase("VAR: odd records, blocks", func() {
		create(n2, "FAB$C_SEQ", "FAB$C_VAR", "0", 0, 0, 0, "")
		put(-1, 1, none)
		put(-1, 3, none)
		put(-1, 4, none)
		put(-1, 600, none)
		closef()
		p.stat(n2, 3)
	})
	n3 := name("V")
	p.kase("VAR: no span", func() {
		create(n3, "FAB$C_SEQ", "FAB$C_VAR", "FAB$M_BLK", 0, 0, 0, "")
		put(-1, 300, none)
		put(-1, 300, none)
		put(-1, 600, none)
		closef()
		p.stat(n3, 3)
	})
	n4 := name("S")
	p.kase("FIX: no span", func() {
		create(n4, "FAB$C_SEQ", "FAB$C_FIX", "FAB$M_BLK", 300, 0, 0, "")
		put(-1, 300, none)
		put(-1, 300, none)
		closef()
		p.stat(n4, 3)
	})
	n5 := name("S")
	p.kase("a short last record: FIX 16, FFB in the middle, read to the end", func() {
		create(n5, "FAB$C_SEQ", "FAB$C_FIX", "0", 16, 0, 0, "")
		put(-1, 16, none)
		closef()
		open(n5)
		get(-1, false)
		get(-1, false)
		closef()
	})
	n6 := name("S")
	p.kase("MRS over the file's on $OPEN, and bad attributes on $CREATE", func() {
		create(n6, "FAB$C_SEQ", "FAB$C_FIX", "0", 0, 0, 0, "")
		reset(n6)
		p.emit("MOVB\t#99,FB+FAB$B_ORG")
		svc("$CREATE", "FAB", "FB")
		reset(n6)
		p.emit("MOVB\t#99,FB+FAB$B_RFM")
		svc("$CREATE", "FAB", "FB")
	})

	p.section("FOP: what each bit leaves behind")

	fopCase := func(title, fop string, closeFop string) {
		n := name("P")
		p.kase(title, func() {
			create(n, "FAB$C_SEQ", "FAB$C_VAR", "FAB$M_CR", 0, 0, 0, fop)
			put(-1, 6, none)
			p.emit("CALLS\t#0,L^SHOWN")
			if closeFop != "" {
				p.emit("BISL2\t#%s,FB+FAB$L_FOP", closeFop)
			}
			closef()
			p.stat(n, 0)
		})
	}
	fopCase("nothing", "", "")
	fopCase("TMP", "FAB$M_TMP", "")
	fopCase("TMD", "FAB$M_TMD", "")
	fopCase("DLT at $CREATE", "FAB$M_DLT", "")
	fopCase("DLT at $CLOSE", "", "FAB$M_DLT")
	fopCase("SPL at $CREATE", "FAB$M_SPL", "")
	fopCase("SPL at $CLOSE", "", "FAB$M_SPL")
	fopCase("SPL and DLT at $CLOSE", "", "FAB$M_SPL!FAB$M_DLT")
	fopCase("SCF at $CLOSE", "", "FAB$M_SCF")
	fopCase("SCF and DLT at $CLOSE", "", "FAB$M_SCF!FAB$M_DLT")
	fopCase("CIF", "FAB$M_CIF", "")
	fopCase("MXV", "FAB$M_MXV", "")
	fopCase("CTG", "FAB$M_CTG", "")
	fopCase("DFW", "FAB$M_DFW", "")
	fopCase("NEF", "FAB$M_NEF", "")
	fopCase("OFP", "FAB$M_OFP", "")

	p.kase("TMP and $OPEN by NAM's FID", func() {
		create("TMPF.DAT", "FAB$C_SEQ", "FAB$C_VAR", "0", 0, 0, 0, "FAB$M_TMP")
		p.emit("CALLS\t#0,L^SHOWN")
		closef()
		p.stat("TMPF.DAT", 0)
	})
	p.kase("CIF: found, and the FAB's attributes it then has", func() {
		create("CIF1.DAT", "FAB$C_SEQ", "FAB$C_FIX", "0", 20, 0, 0, "FAB$M_CIF")
		closef()
		create("CIF1.DAT", "FAB$C_SEQ", "FAB$C_VAR", "0", 0, 0, 0, "FAB$M_CIF")
		closef()
		p.stat("CIF1.DAT", 0)
	})
	p.kase("DEQ: where it's kept, and ALQ returned", func() {
		reset("DEQ1.DAT")
		p.emit("MOVB\t#FAB$C_VAR,FB+FAB$B_RFM")
		p.emit("MOVW\t#50,FB+FAB$W_DEQ")
		p.emit("MOVL\t#7,FB+FAB$L_ALQ")
		svc("$CREATE", "FAB", "FB")
		p.emit("CALLS\t#0,L^SHOWF")
		closef()
		p.stat("DEQ1.DAT", 0)
	})
	p.kase("a FIX file's RAT and record size errors at $PUT", func() {
		create("ERR1.DAT", "FAB$C_SEQ", "FAB$C_FIX", "0", 10, 0, 0, "")
		put(-1, 9, none)
		put(-1, 11, none)
		put(-1, 0, none)
		put(-1, 10, none)
		closef()
		p.stat("ERR1.DAT", 3)
	})

	return p
}

// The run-time code both programs share.
const runtimeData = `	$SSDEF
	$FABDEF
	$RABDEF
	$NAMDEF
	$XABDEF
	$XABFHCDEF
	.PSECT	D,NOEXE,WRT,LONG
VAL:	.LONG	0,0
UVAL:	.LONG	0
ASVL:	.LONG	-1
UOMODE:	.LONG	0
LN:	.LONG	0
LD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BF
BD:	.WORD	400
	.BYTE	14,1
	.ADDRESS BF
BF:	.BLKB	400
RKEY:	.LONG	0
IQL:	.LONG	0,0
IQT:	.BLKB	64
IQD:	.WORD	64
	.BYTE	14,1
	.ADDRESS IQT
SHORTD:	.WORD	4
	.BYTE	14,1
	.ADDRESS IQT
BIGD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BIGB
RDD:	.WORD	0
	.BYTE	14,1
	.ADDRESS RDB
FMTA:	.ASCIZ	~(A)~
	.ALIGN	LONG
BIGB:	.BLKB	33000
RDB:	.BLKB	33000
BLK:	.BLKB	512
HX:	.ASCII	/0123456789ABCDEF/
HL:	.BLKB	100
HLD:	.WORD	0
	.BYTE	14,1
	.ADDRESS HL
MSHOW:	.ASCID	~=!AS r=!XL~
MCOND:	.ASCID	~=c !XL n=!UL a=!XL !XL~
MUNW:	.ASCID	~=u~
MFAB:	.ASCID	~ fab fac=!XB shr=!XB org=!XB rfm=!XB rat=!XB mrs=!UW mrn=!UL alq=!UL deq=!UW fop=!XL bks=!UB bls=!UW fsz=!UB~
MRAB:	.ASCID	~ rab rop=!XL rac=!UB mbc=!UB mbf=!SB rsz=!UW usz=!UW ksz=!UB~
MNORAB:	.ASCID	~ no rab: !XL~
MASV:	.ASCID	~ asv=!XL~
MIQL:	.ASCID	~ q!UL r=!XL v=!XL~
MIQT:	.ASCID	~ q!UL r=!XL |!AD|~
MST:	.ASCID	~ stat !AS: r=!XL~
MST2:	.ASCID	~ stat !AS: org=!XB rfm=!XB rat=!XB mrs=!UW mrn=!UL alq=!UL deq=!UW bks=!UB fop=!XL lrl=!UW hbk=!UL ebk=!UL ffb=!UW~
MRC:	.ASCID	~ rec !UL: r=!XL n=!UW~
MVB:	.ASCID	~ vbn !UL~
MGET:	.ASCID	~ got n=!UW rfa=!UL,!UW stv=!XL bkt=!UL~
MPUT:	.ASCID	~ rfa=!UL,!UW stv=!XL bkt=!UL~
MFST:	.ASCID	~ fab now org=!XB rfm=!XB rat=!XB mrs=!UW mrn=!UL alq=!UL deq=!UW bks=!UB fop=!XL stv=!XL~
MNAM:	.ASCID	~ nam |!AD| fnb=!XL~
MUOA:	.ASCID	~ uo n=!UL unit=!SL fab?=!UL rab?=!UL~
MUOF:	.ASCID	~ uo fna=|!AD| dna=|!AD| nam=!UL~
MUOX:	.ASCID	~ uo xab cod=!UB bln=!UB~
MUOS:	.ASCID	~=uo-rms r=!XL stv=!XL~
	.ALIGN	LONG
FB2:	$FAB	FAC=<GET>,SHR=<GET,PUT,UPD,DEL>,XAB=FH2
	.ALIGN	LONG
RB2:	$RAB	FAB=FB2
	.ALIGN	LONG
FH2:	$XABFHC
	.ALIGN	LONG
FB3:	$FAB	FAC=<BIO,GET>,SHR=<GET,PUT,UPD,DEL>
	.ALIGN	LONG
RB3:	$RAB	FAB=FB3,ROP=<BIO>
	.ALIGN	LONG
FB:	$FAB	FAC=<GET,PUT,UPD,DEL>,NAM=NB
	.ALIGN	LONG
RB:	$RAB	FAB=FB,KBF=RKEY,KSZ=4
	.ALIGN	LONG
NB:	$NAM	RSA=NBR,RSS=128,ESA=NBE,ESS=128
NBR:	.BLKB	128
NBE:	.BLKB	128
	.ALIGN	LONG
FBT:	.BLKB	FAB$C_BLN
RBT:	.BLKB	RAB$C_BLN
	.ALIGN	LONG
`

const runtimeCode = `	.PSECT	C,EXE,NOWRT,LONG
; OUT(control, values...) writes a line.
	.ENTRY	OUT,^M<R2,R3,R4,R5>
	$FAOL_S	CTRSTR=@4(AP),OUTLEN=LD,OUTBUF=BD,PRMLST=8(AP)
	PUSHAB	LD
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
; SHOW(what, r0)
	.ENTRY	SHOW,^M<>
	PUSHL	8(AP)
	PUSHL	4(AP)
	PUSHAB	MSHOW
	CALLS	#3,OUT
	RET
; HND prints a condition, continues below severe, and unwinds from a severe one.
HND:	.WORD	^M<R2>
	MOVL	4(AP),R2
	CMPL	4(R2),#SS$_UNWIND
	BEQL	50$
	PUSHL	12(R2)
	PUSHL	8(R2)
	PUSHL	(R2)
	PUSHL	4(R2)
	PUSHAB	MCOND
	CALLS	#5,OUT
	BICL3	#^XFFFFFFF8,4(R2),R0
	CMPL	R0,#4
	BEQL	40$
	MOVL	#SS$_CONTINUE,R0
	RET
40$:	PUSHAB	MUNW
	CALLS	#1,G^LIB$PUT_OUTPUT
	$UNWIND_S
50$:	MOVL	#SS$_CONTINUE,R0
	RET
; HEXS(address, count) writes bytes in hex, 32 to a line, with an offset.
	.ENTRY	HEXS,^M<R2,R3,R4,R5,R6,R7>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
	CLRL	R7
10$:	TSTL	R3
	BGTR	11$
	BRW	90$
11$:	MOVAB	HL,R4
	MOVB	#^A/ /,(R4)+
	MOVB	#^A/ /,(R4)+
	ASHL	#-8,R7,R0
	EXTZV	#4,#4,R0,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#0,#4,R0,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#4,#4,R7,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#0,#4,R7,R1
	MOVB	HX[R1],(R4)+
	MOVB	#^A/:/,(R4)+
	MOVB	#^A/ /,(R4)+
	MOVL	#32,R5
20$:	TSTL	R3
	BLEQ	30$
	MOVZBL	(R2)+,R6
	EXTZV	#4,#4,R6,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#0,#4,R6,R1
	MOVB	HX[R1],(R4)+
	DECL	R3
	INCL	R7
	SOBGTR	R5,20$
30$:	MOVAB	HL,R0
	SUBL3	R0,R4,R0
	MOVW	R0,HLD
	PUSHAB	HLD
	CALLS	#1,G^LIB$PUT_OUTPUT
	BRW	10$
90$:	RET
; DUMPF(name, flags): a file's attributes by $OPEN and XABFHC; flag 1, its records
; in hex (the first 32 bytes of each, the first 40 records); flag 2, its blocks in
; hex, by block I/O (all-zero lines left out, the first 16 blocks).
	.ENTRY	DUMPF,^M<R2,R3,R4,R5,R6,R7>
	MOVL	4(AP),R2
	MOVB	(R2),FB2+FAB$B_FNS
	MOVL	4(R2),FB2+FAB$L_FNA
	$OPEN	FAB=FB2
	BLBS	R0,10$
	PUSHL	R0
	PUSHL	R2
	PUSHAB	MST
	CALLS	#3,OUT
	RET
10$:	MOVZWL	FH2+XAB$W_FFB,-(SP)
	PUSHL	FH2+XAB$L_EBK
	PUSHL	FH2+XAB$L_HBK
	MOVZWL	FH2+XAB$W_LRL,-(SP)
	PUSHL	FB2+FAB$L_FOP
	MOVZBL	FB2+FAB$B_BKS,-(SP)
	MOVZWL	FB2+FAB$W_DEQ,-(SP)
	PUSHL	FB2+FAB$L_ALQ
	PUSHL	FB2+FAB$L_MRN
	MOVZWL	FB2+FAB$W_MRS,-(SP)
	MOVZBL	FB2+FAB$B_RAT,-(SP)
	MOVZBL	FB2+FAB$B_RFM,-(SP)
	MOVZBL	FB2+FAB$B_ORG,-(SP)
	PUSHL	R2
	PUSHAB	MST2
	CALLS	#15,OUT
	BLBS	8(AP),20$
	BRW	50$
20$:	$CONNECT RAB=RB2
	MOVL	#1,R6
30$:	MOVL	#33000,R0
	MOVW	R0,RB2+RAB$W_USZ
	MOVAB	RDB,RB2+RAB$L_UBF
	$GET	RAB=RB2
	MOVL	R0,R7
	MOVZWL	RB2+RAB$W_RSZ,-(SP)
	PUSHL	R7
	PUSHL	R6
	PUSHAB	MRC
	CALLS	#4,OUT
	BLBC	R7,40$
	MOVZWL	RB2+RAB$W_RSZ,R3
	CMPL	R3,#32
	BLEQ	35$
	MOVL	#32,R3
35$:	PUSHL	R3
	PUSHAB	RDB
	CALLS	#2,HEXS
	AOBLEQ	#40,R6,30$
40$:
50$:	$CLOSE	FAB=FB2
	BBC	#1,8(AP),59$
	BRW	60$
59$:	RET
60$:	MOVB	(R2),FB3+FAB$B_FNS
	MOVL	4(R2),FB3+FAB$L_FNA
	$OPEN	FAB=FB3
	BLBS	R0,62$
	PUSHL	R0
	PUSHL	R2
	PUSHAB	MST
	CALLS	#3,OUT
	RET
62$:	$CONNECT RAB=RB3
	MOVL	#1,R6
70$:	MOVL	R6,RB3+RAB$L_BKT
	MOVW	#512,RB3+RAB$W_USZ
	MOVAB	BLK,RB3+RAB$L_UBF
	$READ	RAB=RB3
	BLBC	R0,80$
	PUSHL	R6
	PUSHAB	MVB
	CALLS	#2,OUT
	CLRL	R3
72$:	MOVAB	BLK[R3],R4
	MOVL	#8,R5
74$:	TSTL	(R4)+
	BNEQ	76$
	SOBGTR	R5,74$
	BRB	78$
76$:	PUSHL	#32
	PUSHAB	BLK[R3]
	PUSHL	R3
	CALLS	#3,HEXO
78$:	ACBL	#511,#32,R3,72$
	AOBLEQ	#16,R6,70$
80$:	$CLOSE	FAB=FB3
	RET
; HEXO(offset, address, count): one line of block dump, with the block's offset.
	.ENTRY	HEXO,^M<R2,R3,R4,R5,R6,R7>
	MOVL	8(AP),R2
	MOVL	12(AP),R3
	MOVL	4(AP),R7
	MOVAB	HL,R4
	MOVB	#^A/ /,(R4)+
	MOVB	#^A/ /,(R4)+
	EXTZV	#8,#4,R7,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#4,#4,R7,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#0,#4,R7,R1
	MOVB	HX[R1],(R4)+
	MOVB	#^A/:/,(R4)+
	MOVB	#^A/ /,(R4)+
20$:	MOVZBL	(R2)+,R6
	EXTZV	#4,#4,R6,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#0,#4,R6,R1
	MOVB	HX[R1],(R4)+
	SOBGTR	R3,20$
	MOVAB	HL,R0
	SUBL3	R0,R4,R0
	MOVW	R0,HLD
	PUSHAB	HLD
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
; RDFILL fills the read buffer with 55s.
	.ENTRY	RDFILL,^M<R2,R3,R4,R5>
	MOVC5	#0,RDB,#^X55,#64,RDB
	RET
`

const forrtlCode = `; SHOWFR shows the FAB and RAB of UVAL's unit, through FOR$RAB.
	.ENTRY	SHOWFR,^M<R2,R3>
	PUSHAB	UVAL
	CALLS	#1,G^FOR$RAB
	MOVL	R0,R3
	BNEQ	10$
	PUSHL	R0
	PUSHAB	MNORAB
	CALLS	#2,OUT
	RET
10$:	MOVL	RAB$L_FAB(R3),R2
	MOVZBL	FAB$B_FSZ(R2),-(SP)
	MOVZWL	FAB$W_BLS(R2),-(SP)
	MOVZBL	FAB$B_BKS(R2),-(SP)
	PUSHL	FAB$L_FOP(R2)
	MOVZWL	FAB$W_DEQ(R2),-(SP)
	PUSHL	FAB$L_ALQ(R2)
	PUSHL	FAB$L_MRN(R2)
	MOVZWL	FAB$W_MRS(R2),-(SP)
	MOVZBL	FAB$B_RAT(R2),-(SP)
	MOVZBL	FAB$B_RFM(R2),-(SP)
	MOVZBL	FAB$B_ORG(R2),-(SP)
	MOVZBL	FAB$B_SHR(R2),-(SP)
	MOVZBL	FAB$B_FAC(R2),-(SP)
	PUSHAB	MFAB
	CALLS	#14,OUT
	CALLS	#0,SHOWR3
	RET
	.ENTRY	SHOWR3,^M<>
	MOVZBL	RAB$B_KSZ(R3),-(SP)
	MOVZWL	RAB$W_USZ(R3),-(SP)
	MOVZWL	RAB$W_RSZ(R3),-(SP)
	CVTBL	RAB$B_MBF(R3),-(SP)
	MOVZBL	RAB$B_MBC(R3),-(SP)
	MOVZBL	RAB$B_RAC(R3),-(SP)
	PUSHL	RAB$L_ROP(R3)
	PUSHAB	MRAB
	CALLS	#8,OUT
	RET
	.ENTRY	SHOWASV,^M<>
	PUSHL	ASVL
	PUSHAB	MASV
	CALLS	#2,OUT
	MNEGL	#1,ASVL
	RET
; INQ1(code, unit keyword or 0, file descriptor or 0): one question. 29 to 33, 41,
; and 42 are numbers and logicals; the others text.
	.ENTRY	INQ1,^M<R2,R3,R4,R5,R6,R7>
	MOVL	4(AP),R7
	MOVL	#^X12345678,IQL
	MOVC5	#0,IQT,#^A/*/,#64,IQT
	CASEL	R7,#29,#19
1$:	.WORD	20$-1$,20$-1$,20$-1$,20$-1$,20$-1$
	.WORD	30$-1$,30$-1$,30$-1$,30$-1$,30$-1$,30$-1$,30$-1$
	.WORD	20$-1$,20$-1$
	.WORD	30$-1$,30$-1$,30$-1$,30$-1$,30$-1$,30$-1$
20$:	PUSHAB	IQL
	BISL3	#^X500,R7,-(SP)
	BRB	40$
30$:	PUSHAB	IQD
	BISL3	#^X400,R7,-(SP)
40$:	PUSHL	#^X00010203
	MOVL	#3,R6
	TSTL	12(AP)
	BEQL	50$
	PUSHL	12(AP)
	PUSHL	#^X070E
	ADDL2	#2,R6
50$:	TSTL	8(AP)
	BEQL	60$
	PUSHL	8(AP)
	INCL	R6
60$:	CALLS	R6,G^FOR$INQUIRE
	MOVL	R0,R6
	CMPB	IQT,#^A/*/
	BNEQ	70$
	PUSHL	IQL
	PUSHL	R6
	PUSHL	R7
	PUSHAB	MIQL
	CALLS	#4,OUT
	RET
70$:	PUSHAB	IQT
	PUSHL	#64
	PUSHL	R6
	PUSHL	R7
	PUSHAB	MIQT
	CALLS	#5,OUT
	RET
; INQA(unit keyword or 0, file descriptor or 0): every question.
	.ENTRY	INQA,^M<R2>
	MOVL	#29,R2
10$:	PUSHL	8(AP)
	PUSHL	4(AP)
	PUSHL	R2
	CALLS	#3,INQ1
	AOBLEQ	#48,R2,10$
	RET
; UOLOG is a USEROPEN routine: it shows what it's given, then by UOMODE opens the
; file. 0 $CREATE and $CONNECT, 1 $OPEN and $CONNECT, 2 fails with RMS$_FNF, 3
; $CREATE alone, 4 $CREATE with RFM changed.
	.ENTRY	UOLOG,^M<R2,R3,R4,R5,R6>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
	MOVL	12(AP),R4
	CLRL	-(SP)
	CMPB	RAB$B_BID(R3),#RAB$C_BID
	BNEQ	10$
	MOVL	#1,(SP)
10$:	CLRL	-(SP)
	CMPB	FAB$B_BID(R2),#FAB$C_BID
	BNEQ	20$
	MOVL	#1,(SP)
20$:	PUSHL	(R4)
	MOVZBL	(AP),-(SP)
	PUSHAB	MUOA
	CALLS	#5,OUT
	MOVZBL	FAB$B_FSZ(R2),-(SP)
	MOVZWL	FAB$W_BLS(R2),-(SP)
	MOVZBL	FAB$B_BKS(R2),-(SP)
	PUSHL	FAB$L_FOP(R2)
	MOVZWL	FAB$W_DEQ(R2),-(SP)
	PUSHL	FAB$L_ALQ(R2)
	PUSHL	FAB$L_MRN(R2)
	MOVZWL	FAB$W_MRS(R2),-(SP)
	MOVZBL	FAB$B_RAT(R2),-(SP)
	MOVZBL	FAB$B_RFM(R2),-(SP)
	MOVZBL	FAB$B_ORG(R2),-(SP)
	MOVZBL	FAB$B_SHR(R2),-(SP)
	MOVZBL	FAB$B_FAC(R2),-(SP)
	PUSHAB	MFAB
	CALLS	#14,OUT
	CALLS	#0,SHOWR3
	MOVL	FAB$L_NAM(R2),R0
	BEQL	25$
	MOVL	#1,R0
25$:	PUSHL	R0
	PUSHL	FAB$L_DNA(R2)
	MOVZBL	FAB$B_DNS(R2),-(SP)
	PUSHL	FAB$L_FNA(R2)
	MOVZBL	FAB$B_FNS(R2),-(SP)
	PUSHAB	MUOF
	CALLS	#6,OUT
	MOVL	FAB$L_XAB(R2),R5
30$:	BEQL	40$
	MOVZBL	XAB$B_BLN(R5),-(SP)
	MOVZBL	XAB$B_COD(R5),-(SP)
	PUSHAB	MUOX
	CALLS	#3,OUT
	MOVL	XAB$L_NXT(R5),R5
	BRB	30$
40$:	MOVL	UOMODE,R6
	CMPL	R6,#2
	BNEQ	42$
	MOVL	#^X18292,R0
	RET
42$:	CMPL	R6,#4
	BNEQ	44$
	MOVB	#FAB$C_STM,FAB$B_RFM(R2)
44$:	CMPL	R6,#1
	BEQL	50$
	$CREATE	FAB=(R2)
	BRB	60$
50$:	$OPEN	FAB=(R2)
60$:	MOVL	R0,R5
	PUSHL	FAB$L_STV(R2)
	PUSHL	R5
	PUSHAB	MUOS
	CALLS	#3,OUT
	BLBC	R5,90$
	CMPL	R6,#3
	BEQL	90$
	$CONNECT RAB=(R3)
	MOVL	R0,R5
	PUSHL	RAB$L_STV(R3)
	PUSHL	R5
	PUSHAB	MUOS
	CALLS	#3,OUT
90$:	MOVL	R5,R0
	RET
; CLEAN closes units 10 to 19 after each case, saying nothing.
CLH:	.WORD	^M<>
	MOVL	4(AP),R0
	CMPL	4(R0),#SS$_UNWIND
	BEQL	50$
	$UNWIND_S
50$:	MOVL	#SS$_CONTINUE,R0
	RET
	.ENTRY	CLEAN,^M<R2>
	MOVAB	CLH,(FP)
	MOVL	#10,R2
10$:	ASHL	#16,R2,R0
	BISL2	#^X0101,R0
	PUSHL	#^X00010203
	PUSHL	R0
	CALLS	#2,G^FOR$CLOSE
	AOBLEQ	#19,R2,10$
	RET
`

const rmsCode = `	.ENTRY	RRESET,^M<R2,R3,R4,R5>
	MOVC3	#FAB$C_BLN,FBT,FB
	MOVC3	#RAB$C_BLN,RBT,RB
	RET
	.ENTRY	CLEAN,^M<>
	RET
; SHOWF is what the FAB holds after a $CREATE or $OPEN.
	.ENTRY	SHOWF,^M<>
	PUSHL	FB+FAB$L_STV
	PUSHL	FB+FAB$L_FOP
	MOVZBL	FB+FAB$B_BKS,-(SP)
	MOVZWL	FB+FAB$W_DEQ,-(SP)
	PUSHL	FB+FAB$L_ALQ
	PUSHL	FB+FAB$L_MRN
	MOVZWL	FB+FAB$W_MRS,-(SP)
	MOVZBL	FB+FAB$B_RAT,-(SP)
	MOVZBL	FB+FAB$B_RFM,-(SP)
	MOVZBL	FB+FAB$B_ORG,-(SP)
	PUSHAB	MFST
	CALLS	#11,OUT
	RET
	.ENTRY	SHOWN,^M<>
	PUSHL	NB+NAM$L_FNB
	PUSHAB	NBR
	MOVZBL	NB+NAM$B_RSL,-(SP)
	PUSHAB	MNAM
	CALLS	#4,OUT
	RET
; SHOWR is what the RAB holds after a $PUT or the like, SHOWG after a $GET, with
; the record in hex.
	.ENTRY	SHOWR,^M<>
	PUSHL	RB+RAB$L_BKT
	PUSHL	RB+RAB$L_STV
	MOVZWL	RB+RAB$W_RFA4,-(SP)
	PUSHL	RB+RAB$L_RFA0
	PUSHAB	MPUT
	CALLS	#5,OUT
	RET
	.ENTRY	SHOWG,^M<R2>
	PUSHL	RB+RAB$L_BKT
	PUSHL	RB+RAB$L_STV
	MOVZWL	RB+RAB$W_RFA4,-(SP)
	PUSHL	RB+RAB$L_RFA0
	MOVZWL	RB+RAB$W_RSZ,-(SP)
	PUSHAB	MGET
	CALLS	#6,OUT
	MOVZWL	RB+RAB$W_RSZ,R2
	CMPL	R2,#32
	BLEQ	10$
	MOVL	#32,R2
10$:	PUSHL	R2
	PUSHAB	RDB
	CALLS	#2,HEXS
	RET
`

// write assembles a program's file.
func (p *prog) write(title, extra string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\t.TITLE\t%s - %s\n; Written by testdata/for81/gen.go.\n", p.name, title)
	b.WriteString(runtimeData)
	b.WriteString(p.data.String())
	b.WriteString(runtimeCode)
	b.WriteString(extra)
	b.WriteString(p.subs.String())
	b.WriteString(p.code.String())
	fmt.Fprintf(&b, "\t.ENTRY\tMAIN,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n")
	b.WriteString("\tMOVC3\t#FAB$C_BLN,FB,FBT\n\tMOVC3\t#RAB$C_BLN,RB,RBT\n")
	// BIGB holds its offset's low byte, so a record's bytes show which ones they are.
	b.WriteString("\tCLRL\tR0\n1$:\tMOVB\tR0,BIGB[R0]\n\tAOBLSS\t#33000,R0,1$\n")
	b.WriteString(p.drive.String())
	fmt.Fprintf(&b, "\tPUSHAB\tENDM\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n\tMOVL\t#1,R0\n\tRET\n")
	b.WriteString("\t.PSECT\tD\nENDM:\t.ASCID\t~##### the end of the cases~\n")
	b.WriteString("\t.END\tMAIN\n")

	return b.String()
}

const com = `$ ! FOR81.COM - the Phase 81 probe of FORRTL's file statements and RMS's files.
$ ! Written by testdata/for81/gen.go. Run it with the exchange volume's [000000]
$ ! as the default directory:
$ !
$ !     @FOR81/OUTPUT=FOR81.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST FOR81
$ LINK/NOMAP FOR81,FOR81/OPTIONS
$ MACRO/NOLIST FOR81R
$ LINK/NOMAP FOR81R
$ !
$ ! RMS first: the files' bytes.
$ DEFINE/USER_MODE SYS$OUTPUT FOR81R-VAX.TXT
$ RUN FOR81R
$ SHOW SYMBOL $STATUS
$ !
$ ! FORRTL, through a procedure with /OUTPUT, so that SYS$OUTPUT is a process-permanent
$ ! file: the cases that open SYS$OUTPUT's units then add to this file and don't make
$ ! a new version of it. Unit 11 has a logical name.
$ DEFINE/NOLOG FOR011 NAMED.DAT
$ @FOR81B/OUTPUT=FOR81-VAX.TXT
$ SHOW SYMBOL $STATUS
$ DEASSIGN FOR011
$ !
$ ! The queues' state, for what PRINT and SUBMIT did, and what's left.
$ SHOW QUEUE/ALL
$ DIRECTORY/SIZE=ALL/DATE *.DAT
$ DUMP/BLOCK=COUNT:1 REL1.DAT
$ EXIT
`

func main() {
	f := forrtl()
	r := rmsCases()

	files := map[string]string{
		"for81.mar":  f.write("Phase 81's probe of FORRTL's file statements", forrtlCode),
		"for81r.mar": r.write("Phase 81's probe of RMS's relative and fixed-length files", rmsCode),
		"for81.opt":  "SYS$SHARE:FORRTL/SHAREABLE\n",
		"for81.com":  com,
		"for81b.com": "$ SET NOVERIFY\n$ RUN FOR81\n$ EXIT $STATUS\n",
		"exchange.cmd": `! EXCHANGE.CMD - builds the Phase 81 probe's exchange volume with govax.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for81-exchange.dsk" /DEVICE=RD54 FOR81
MOUNT/WRITE DUA2 "testdata/disks/for81-exchange.dsk"
COPY "testdata/for81/for81.mar"/HOST DUA2:[000000]FOR81.MAR
COPY "testdata/for81/for81r.mar"/HOST DUA2:[000000]FOR81R.MAR
COPY "testdata/for81/for81.opt"/HOST DUA2:[000000]FOR81.OPT
COPY "testdata/for81/for81.com"/HOST DUA2:[000000]FOR81.COM
COPY "testdata/for81/for81b.com"/HOST DUA2:[000000]FOR81B.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`,
		"copyout.cmd": `! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/copyout.cmd
!
MOUNT DUA2 "testdata/disks/for81-exchange.dsk"
COPY DUA2:[000000]FOR81-VAX.TXT "testdata/for81/vax/for81-vax.txt"/HOST
COPY DUA2:[000000]FOR81R-VAX.TXT "testdata/for81/vax/for81r-vax.txt"/HOST
COPY DUA2:[000000]FOR81.LOG "testdata/for81/vax/for81.log"/HOST
DISMOUNT DUA2
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("for81.mar: %d cases; for81r.mar: %d cases\n", f.n, r.n)
}
