//go:build ignore

// gen.go writes round 2 of the Phase 88 probe (docs/PHASE-88 - keyed index
// files.md, subtask 12): FORRTL on indexed files, its behavior being FTNRTL's
// requirement. Only what FORRTL does is used, never its code. Run this from the
// repository root:
//
//	go run testdata/for88/gen.go
//
// It writes for88.mar (FORRTL's OPEN, INQUIRE, keyed READ, WRITE, REWRITE,
// DELETE, UNLOCK, and positioning on indexed files), for88.opt, for88.com,
// for88b.com, exchange.cmd, and copyout.cmd. The output has the form of
// testdata/for81's, which this file's helpers come from.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/for88"

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
	nkeys int
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

// Key type codes: FOR$OPEN's KEY= block, found by trial over govax (2026-11-04).
// The value is the address of a block: a word nobody looks at, a word of the
// count of the longwords after it, three per key: a descriptor type (0 or 14 a
// string, 7 or 8 a signed integer of the key's length, 3 or 4 unsigned), with
// 512 added for a descending key; the first byte; the last byte.
const (
	tStg  = 14
	tInt  = 8
	tDesc = 512
)

// A record of the standard file, 16 bytes: a string at 1:4 (the primary key, key
// 0), a longword at 5:8 (key 1), a descending string at 9:12 (key 2), a longword.
type stdRec struct {
	k0   string
	k1   int
	k2   string
	rest int
}

var stdRecs = []stdRec{
	{"BBBB", 20, "pppp", 1}, {"DDDD", 40, "qqqq", 2}, {"FFFF", 60, "rrrr", 3}, {"HHHH", 80, "ssss", 4},
	{"BCDE", 25, "pqrs", 5}, {"JJJJ", -10, "tttt", 6}, {"LLLL", 100, "pppp", 7}, {"NNNN", 40, "uuuu", 8},
}

// keys is a label for a KEY= block of triples.
func (p *prog) keys(triples ...int) string {
	return p.keysW(0, len(triples), triples...)
}

func (p *prog) keysW(w0, count int, triples ...int) string {
	l := fmt.Sprintf("KL%d", p.nkeys)
	p.nkeys++
	fmt.Fprintf(&p.data, "\t.ALIGN\tLONG\n%s:\t.WORD\t%d,%d\n", l, w0, count)

	for _, v := range triples {
		fmt.Fprintf(&p.data, "\t.LONG\t%d\n", v)
	}

	return l
}

// rec is a label for the descriptor of a record's bytes.
func (p *prog) rec(k0 string, k1 int, k2 string, rest int) string {
	l := fmt.Sprintf("RC%d", p.nkeys)
	p.nkeys++
	fmt.Fprintf(&p.data, "\t.ALIGN\tLONG\n%sB:\t.ASCII\t~%s~\n\t.LONG\t%d\n\t.ASCII\t~%s~\n\t.LONG\t%d\n", l, k0, k1, k2, rest)
	fmt.Fprintf(&p.data, "%s:\t.WORD\t16\n\t.BYTE\t14,1\n\t.ADDRESS %sB\n", l, l)

	return l
}

// intKey is a label for the descriptor of an integer key value, of dtype 7 or 8.
func (p *prog) intKey(v, dtype int) string {
	l := fmt.Sprintf("IK%d", p.nkeys)
	p.nkeys++
	fmt.Fprintf(&p.data, "\t.ALIGN\tLONG\n%sB:\t.LONG\t%d\n%s:\t.WORD\t%d\n\t.BYTE\t%d,1\n\t.ADDRESS %sB\n", l, v, l, map[int]int{7: 2, 8: 4}[dtype], dtype, l)

	return l
}

// arrKey is a label for a class A descriptor of a BYTE array holding s.
func (p *prog) arrKey(s string) string {
	l := fmt.Sprintf("AK%d", p.nkeys)
	p.nkeys++
	fmt.Fprintf(&p.data, "\t.ALIGN\tLONG\n%sB:\t.ASCII\t~%s~\n\t.ALIGN\tLONG\n%s:\t.WORD\t1\n\t.BYTE\t2,4\n\t.ADDRESS %sB\n\t.BYTE\t0,0,^XC0,1\n\t.LONG\t%d\n\t.ADDRESS %sB\n\t.LONG\t%d\n\t.LONG\t1,%d\n",
		l, s, l, l, len(s), l, len(s), len(s))

	return l
}

func key(label string) kw { return addr(kKey, label) }

var uo = kw{fmt.Sprintf("#^X%02X%02X", mLongVal, kUserOpen), "&UOLOG"}

func acc(v int) kw  { return imm(kAccess, v) }
func form(v int) kw { return imm(kForm, v) }
func recl(v int) kw { return imm(kRecl, v) }
func stat(v int) kw { return imm(kStatus, v) }
func org(v int) kw  { return imm(kOrg, v) }

const (
	sOld, sNew, sScratch, sUnknown = 1, 2, 3, 4
	aDirect, aSeq, aAppend, aKeyed = 1, 2, 3, 4
	fFmt, fUnf                     = 1, 2
	oSeq, oRel, oIdx               = 1, 2, 3
)

// stdKeys is the standard file's KEY=.
func (p *prog) stdKeys() kw {
	return key(p.keys(tStg, 1, 4, tInt, 5, 8, tStg+tDesc, 9, 12))
}

// mkStd makes a standard file on unit u, loaded with stdRecs in that order, keyed
// access, unformatted, and leaves it open.
func (p *prog) mkStd(u int, name string) {
	p.open(u, file(p, name), stat(sNew), org(oIdx), acc(aKeyed), form(fUnf), recl(4), p.stdKeys())

	for _, r := range stdRecs {
		p.wrRec(u, p.rec(r.k0, r.k1, r.k2, r.rest))
	}
}

// oldStd opens a standard file made earlier.
func (p *prog) oldStd(u int, name string, kws ...kw) {
	p.open(u, append([]kw{file(p, name), stat(sOld), org(oIdx), acc(aKeyed), form(fUnf)}, kws...)...)
}

// wrRec is one unformatted WRITE of a record.
func (p *prog) wrRec(u int, label string) {
	p.stmt(func() {
		p.call("FOR$WRITE_SU", false, fmt.Sprintf("#%d", u))
		p.call("FOR$IO_T_DS", false, "&"+label)
		p.call("FOR$IO_END", true)
	})
}

// rwRec is one unformatted REWRITE of a record.
func (p *prog) rwRec(u int, label string) {
	p.stmt(func() {
		p.call("FOR$REWRITE_SU", false, fmt.Sprintf("#%d", u))
		p.call("FOR$IO_T_DS", false, "&"+label)
		p.call("FOR$IO_END", true)
	})
}

// rdK is a keyed unformatted READ into 16 bytes, showing what it read and the RAB.
func (p *prog) rdK(u int, keyLabel string, id, match int) {
	p.stmt(func() {
		p.call("FOR$READ_KU", false, fmt.Sprintf("#%d", u), "&"+keyLabel, fmt.Sprintf("#%d", id), fmt.Sprintf("#%d", match))
		p.rdItem(16)
		p.call("FOR$IO_END", true)
		p.showRd(16)
	})
	p.fab(u)
}

// text key: p.text's .ASCID is a class S string descriptor.
func (p *prog) tk(s string) string { return p.text(s) }

func cases() *prog {
	p := newProg("FOR88")

	p.section("KEY=: what FORRTL asks RMS for")

	for _, c := range []struct {
		title string
		list  []int
	}{
		{"string, type 0", []int{0, 1, 4}}, {"string, type 14", []int{tStg, 1, 4}},
		{"string, descending, type 512", []int{tDesc, 1, 4}}, {"string, descending, type 526", []int{tStg + tDesc, 1, 4}},
		{"integer, type 8, 4 bytes", []int{tInt, 1, 4}}, {"integer, type 8, 2 bytes", []int{tInt, 1, 2}},
		{"integer, type 7, 2 bytes", []int{7, 1, 2}}, {"integer, type 7, 4 bytes", []int{7, 1, 4}},
		{"integer, descending, 4", []int{tInt + tDesc, 1, 4}}, {"integer, descending, 2", []int{tInt + tDesc, 1, 2}},
		{"unsigned, type 4, 4 bytes", []int{4, 1, 4}}, {"unsigned, type 3, 2 bytes", []int{3, 1, 2}},
		{"integer, 3 bytes", []int{tInt, 1, 3}}, {"integer, 8 bytes", []int{tInt, 1, 8}},
		{"type 1", []int{1, 1, 4}}, {"type 2", []int{2, 1, 4}}, {"type 9", []int{9, 1, 4}}, {"type 256+14", []int{256 + tStg, 1, 4}},
		{"two keys", []int{tStg, 1, 4, tInt, 5, 8}},
		{"three keys, overlapping", []int{tStg, 1, 8, tStg, 3, 6, tInt, 5, 8}},
		{"the same key twice", []int{tStg, 1, 4, tStg, 1, 4}},
		{"a primary key not first in the record", []int{tStg, 9, 16, tInt, 1, 4}},
		{"the first byte 0", []int{tStg, 0, 4}}, {"the last before the first", []int{tStg, 5, 4}},
		{"the first negative", []int{tStg, -1, 4}},
		{"one byte", []int{tStg, 7, 7}}, {"to the record's end", []int{tStg, 37, 40}},
		{"past the record's end", []int{tStg, 37, 41}}, {"wholly past it", []int{tStg, 100, 104}},
	} {
		p.kase("KEY "+c.title, func() {
			p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), acc(aKeyed), form(fFmt), recl(40), key(p.keys(c.list...)), uo)
			p.fab(10)
		})
	}

	p.kase("KEY of 255 and 256 bytes, in a long record", func() {
		p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), acc(aKeyed), form(fFmt), recl(400), key(p.keys(tStg, 1, 255)), uo)
		p.closeU(10)
		p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), acc(aKeyed), form(fFmt), recl(400), key(p.keys(tStg, 1, 256)), uo)
	})
	p.kase("KEY's first word 5", func() {
		p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), recl(40), key(p.keysW(5, 3, tStg, 1, 4)), uo)
	})
	p.kase("KEY's count 0", func() {
		p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), recl(40), key(p.keysW(0, 0)), uo)
	})
	p.kase("KEY's count 4", func() {
		p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), recl(40), key(p.keysW(0, 4, tStg, 1, 4, 0)), uo)
	})
	p.kase("KEY by mechanism 5", func() {
		p.open(10, file(p, "A.IDX"), stat(sScratch), org(oIdx), recl(40), ref(kKey, mLongRef, p.keys(tStg, 1, 4)), uo)
	})

	p.section("OPEN of a new indexed file: the keywords around KEY=")

	for _, c := range []struct {
		title string
		kws   func() []kw
	}{
		{"KEY, nothing else", func() []kw { return []kw{key(p.keys(tStg, 1, 4))} }},
		{"KEY, RECL", func() []kw { return []kw{recl(40), key(p.keys(tStg, 1, 4))} }},
		{"KEY, ORG", func() []kw { return []kw{org(oIdx), key(p.keys(tStg, 1, 4))} }},
		{"KEY, ORG, RECL", func() []kw { return []kw{org(oIdx), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ORG, RECL, no KEY", func() []kw { return []kw{org(oIdx), recl(40)} }},
		{"KEY, RECL, sequential", func() []kw { return []kw{org(oSeq), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"KEY, RECL, relative", func() []kw { return []kw{org(oRel), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS KEYED, sequential", func() []kw { return []kw{org(oSeq), acc(aKeyed), recl(40)} }},
		{"ACCESS KEYED, relative", func() []kw { return []kw{org(oRel), acc(aKeyed), recl(40)} }},
		{"ACCESS KEYED, no ORG", func() []kw { return []kw{acc(aKeyed), recl(40)} }},
		{"ACCESS KEYED, KEY, no ORG", func() []kw { return []kw{acc(aKeyed), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS SEQUENTIAL", func() []kw { return []kw{org(oIdx), acc(aSeq), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS DIRECT", func() []kw { return []kw{org(oIdx), acc(aDirect), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS APPEND", func() []kw { return []kw{org(oIdx), acc(aAppend), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS KEYED", func() []kw { return []kw{org(oIdx), acc(aKeyed), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS KEYED, FORMATTED", func() []kw { return []kw{org(oIdx), acc(aKeyed), form(fFmt), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"ACCESS KEYED, UNFORMATTED", func() []kw { return []kw{org(oIdx), acc(aKeyed), form(fUnf), recl(10), key(p.keys(tStg, 1, 4))} }},
		{"no RECL, RECORDTYPE VARIABLE", func() []kw { return []kw{org(oIdx), imm(kRecType, 2), key(p.keys(tStg, 1, 4))} }},
		{"RECORDTYPE FIXED", func() []kw { return []kw{org(oIdx), imm(kRecType, 1), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"RECORDTYPE VARIABLE", func() []kw { return []kw{org(oIdx), imm(kRecType, 2), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"RECORDTYPE SEGMENTED", func() []kw { return []kw{org(oIdx), imm(kRecType, 3), recl(40), form(fUnf), key(p.keys(tStg, 1, 4))} }},
		{"RECORDTYPE STREAM", func() []kw { return []kw{org(oIdx), imm(kRecType, 4), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"INITIALSIZE, EXTENDSIZE", func() []kw {
			return []kw{org(oIdx), imm(kInitial, 30), imm(kExtend, 9), recl(40), key(p.keys(tStg, 1, 4))}
		}},
		{"BLOCKSIZE", func() []kw { return []kw{org(oIdx), imm(kBlock, 1024), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"BUFFERCOUNT", func() []kw { return []kw{org(oIdx), imm(kBufCount, 3), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"MAXREC", func() []kw { return []kw{org(oIdx), imm(kMaxRec, 3), recl(40), key(p.keys(tStg, 1, 4))} }},
		{"NOSPANBLOCKS", func() []kw { return []kw{org(oIdx), kw{"#^X00C"}, recl(40), key(p.keys(tStg, 1, 4))} }},
		{"SHARED", func() []kw { return []kw{org(oIdx), kw{"#^X00D"}, recl(40), key(p.keys(tStg, 1, 4))} }},
		{"READONLY", func() []kw { return []kw{org(oIdx), kw{"#^X008"}, recl(40), key(p.keys(tStg, 1, 4))} }},
	} {
		for _, st := range []int{sNew, sUnknown} {
			name := fmt.Sprintf("B%d.IDX", p.n)
			title := fmt.Sprintf("OPEN status %d: %s", st, c.title)
			p.kase(title, func() {
				p.open(10, append([]kw{file(p, name), stat(st), uo}, c.kws()...)...)
				p.fab(10)
				p.closeU(10)
				p.stat(name, 0)
			})
		}
	}

	p.section("The standard file, and OPEN of an existing one")

	p.kase("make STD.IDX, write 8 records", func() {
		p.mkStd(10, "STD.IDX")
		p.fab(10)
		p.inq(10)
		p.closeU(10)
		p.stat("STD.IDX", 1)
	})

	for _, c := range []struct {
		title string
		kws   func() []kw
	}{
		{"nothing", func() []kw { return nil }},
		{"ORG", func() []kw { return []kw{org(oIdx)} }},
		{"ORG sequential", func() []kw { return []kw{org(oSeq)} }},
		{"ORG relative", func() []kw { return []kw{org(oRel)} }},
		{"ACCESS KEYED", func() []kw { return []kw{acc(aKeyed)} }},
		{"ACCESS KEYED, ORG", func() []kw { return []kw{acc(aKeyed), org(oIdx)} }},
		{"ACCESS SEQUENTIAL, ORG", func() []kw { return []kw{acc(aSeq), org(oIdx)} }},
		{"ACCESS DIRECT, ORG", func() []kw { return []kw{acc(aDirect), org(oIdx)} }},
		{"ACCESS APPEND, ORG", func() []kw { return []kw{acc(aAppend), org(oIdx)} }},
		{"RECL right", func() []kw { return []kw{org(oIdx), form(fUnf), recl(4)} }},
		{"RECL wrong", func() []kw { return []kw{org(oIdx), form(fUnf), recl(5)} }},
		{"FORMATTED", func() []kw { return []kw{org(oIdx), acc(aKeyed), form(fFmt)} }},
		{"UNFORMATTED", func() []kw { return []kw{org(oIdx), acc(aKeyed), form(fUnf)} }},
		{"RECORDTYPE VARIABLE", func() []kw { return []kw{org(oIdx), imm(kRecType, 2)} }},
		{"READONLY", func() []kw { return []kw{org(oIdx), kw{"#^X008"}} }},
		{"SHARED", func() []kw { return []kw{org(oIdx), kw{"#^X00D"}} }},
		{"KEY the same", func() []kw { return []kw{org(oIdx), p.stdKeys()} }},
		{"KEY the same, no ORG", func() []kw { return []kw{p.stdKeys()} }},
		{"KEY of fewer keys", func() []kw { return []kw{org(oIdx), key(p.keys(tStg, 1, 4))} }},
		{"KEY of more keys", func() []kw {
			return []kw{org(oIdx), key(p.keys(tStg, 1, 4, tInt, 5, 8, tStg+tDesc, 9, 12, tInt, 13, 16))}
		}},
		{"KEY of another place", func() []kw { return []kw{org(oIdx), key(p.keys(tStg, 2, 5, tInt, 5, 8, tStg+tDesc, 9, 12))} }},
		{"KEY of another length", func() []kw { return []kw{org(oIdx), key(p.keys(tStg, 1, 3, tInt, 5, 8, tStg+tDesc, 9, 12))} }},
		{"KEY of another type", func() []kw { return []kw{org(oIdx), key(p.keys(tInt, 1, 4, tInt, 5, 8, tStg+tDesc, 9, 12))} }},
		{"KEY of another direction", func() []kw { return []kw{org(oIdx), key(p.keys(tStg+tDesc, 1, 4, tInt, 5, 8, tStg+tDesc, 9, 12))} }},
		{"KEY's alternate in another direction", func() []kw { return []kw{org(oIdx), key(p.keys(tStg, 1, 4, tInt+tDesc, 5, 8, tStg+tDesc, 9, 12))} }},
	} {
		for _, st := range []int{sOld, sUnknown} {
			title := fmt.Sprintf("OPEN status %d of STD.IDX: %s", st, c.title)
			p.kase(title, func() {
				// USEROPEN's $OPEN: its $CREATE of a file with FOP CIF would do too,
				// but with STATUS='OLD' it would make a new, empty version.
				p.emit("MOVL\t#1,UOMODE")
				p.open(10, append([]kw{file(p, "STD.IDX"), stat(st), uo}, c.kws()...)...)
				p.emit("CLRL\tUOMODE")
				p.fab(10)
			})
		}
	}

	p.kase("INQUIRE by unit, ACCESS KEYED", func() {
		p.oldStd(10, "STD.IDX")
		p.inq(10)
	})
	p.kase("INQUIRE by unit, ACCESS SEQUENTIAL", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aSeq))
		p.inq(10)
	})
	p.kase("INQUIRE by file, closed", func() {
		p.inqF("STD.IDX")
	})
	p.kase("a scratch indexed file: CLOSE deletes it", func() {
		p.open(10, file(p, "SCR.IDX"), stat(sScratch), org(oIdx), acc(aKeyed), recl(40), key(p.keys(tStg, 1, 4)))
		p.closeU(10)
		p.stat("SCR.IDX", 0)
	})
	p.kase("CLOSE with DISPOSE='DELETE'", func() {
		p.open(10, file(p, "DLT.IDX"), stat(sNew), org(oIdx), acc(aKeyed), recl(40), key(p.keys(tStg, 1, 4)))
		p.closeU(10, imm(kDispose, 2))
		p.stat("DLT.IDX", 0)
	})
	p.kase("no file name: the default for an indexed file", func() {
		p.open(13, stat(sNew), org(oIdx), acc(aKeyed), recl(40), key(p.keys(tStg, 1, 4)), uo)
		p.inq(13)
	})

	p.section("Keyed READs: each match on each key")

	// The values each key is searched for: one found, one between records, and
	// one past the last in the key's order, on each of the file's three keys.
	type search struct {
		id    int
		name  string
		value func() string
	}

	searches := []search{
		{0, "'DDDD'", func() string { return p.tk("DDDD") }}, {0, "'CCCC'", func() string { return p.tk("CCCC") }},
		{0, "'ZZZZ'", func() string { return p.tk("ZZZZ") }}, {0, "'AAAA'", func() string { return p.tk("AAAA") }},
		{0, "generic 'B'", func() string { return p.tk("B") }}, {0, "generic 'BC'", func() string { return p.tk("BC") }},
		{0, "generic 'C'", func() string { return p.tk("C") }},
		{1, "40", func() string { return p.intKey(40, 8) }}, {1, "50", func() string { return p.intKey(50, 8) }},
		{1, "-10", func() string { return p.intKey(-10, 8) }}, {1, "1000", func() string { return p.intKey(1000, 8) }},
		{1, "word 60", func() string { return p.intKey(60, 7) }},
		{2, "'qqqq'", func() string { return p.tk("qqqq") }}, {2, "'q'", func() string { return p.tk("q") }},
		{2, "'pppp'", func() string { return p.tk("pppp") }}, {2, "'zzzz'", func() string { return p.tk("zzzz") }},
		{2, "'aaaa'", func() string { return p.tk("aaaa") }},
	}

	for _, s := range searches {
		for m := 0; m <= 6; m++ {
			p.kase(fmt.Sprintf("READ_KU key %d of %s, match %d", s.id, s.name, m), func() {
				p.oldStd(10, "STD.IDX")
				p.rdK(10, s.value(), s.id, m)
			})
		}
	}

	p.section("Keyed READs: their arguments and failures")

	p.kase("match 7, and -1", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.tk("DDDD"), 0, 7)
		p.rdK(10, p.tk("DDDD"), 0, -1)
	})
	p.kase("KEYID: -1 keeps the last, and each key's number past the last", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.tk("DDDD"), -1, 0)
		p.rdK(10, p.intKey(40, 8), 1, 0)
		p.rdK(10, p.intKey(60, 8), -1, 0)
		p.rdK(10, p.tk("qqqq"), 2, 0)
		p.rdK(10, p.tk("qqqq"), -1, 0)
		p.rdK(10, p.tk("DDDD"), 3, 0)
		p.rdK(10, p.tk("qqqq"), -1, 0)
		p.rdK(10, p.tk("DDDD"), -2, 0)
		p.rdK(10, p.tk("DDDD"), 300, 0)
	})
	p.kase("fewer arguments", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.intKey(40, 8), 1, 0)
		p.stmt(func() {
			p.call("FOR$READ_KU", false, "#10", "&"+p.tk("DDDD"))
			p.rdItem(16)
			p.call("FOR$IO_END", true)
			p.showRd(16)
		})
		p.fab(10)
		p.stmt(func() {
			p.call("FOR$READ_KU", false, "#10", "&"+p.tk("DDDD"), "#0")
			p.rdItem(16)
			p.call("FOR$IO_END", true)
			p.showRd(16)
		})
		p.fab(10)
	})
	p.kase("wrong types: string on integer, integer on string, too long, empty", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.tk("DDDD"), 1, 0)
		p.rdK(10, p.intKey(40, 8), 0, 0)
		p.rdK(10, p.intKey(40, 7), 0, 0)
		p.rdK(10, p.tk("DDDDD"), 0, 0)
		p.rdK(10, p.tk("DDDDD"), 2, 0)
		p.rdK(10, p.tk(""), 0, 0)
	})
	p.kase("a word on a longword key, a negative word, and a word out of range", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.intKey(-10, 7), 1, 0)
		p.rdK(10, p.intKey(40, 7), 1, 0)
		p.rdK(10, p.intKey(70000, 8), 1, 1)
	})
	p.kase("a BYTE array's descriptor", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.arrKey("DDDD"), 0, 0)
		p.rdK(10, p.arrKey("BC"), 0, 0)
		p.rdK(10, p.arrKey("DDDDD"), 0, 0)
		p.rdK(10, p.arrKey("DDDD"), 1, 0)
	})
	p.kase("a formatted keyed READ", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fFmt))
		p.stmt(func() {
			p.call("FOR$READ_KO", false, "#10", "&FMTA", "&"+p.tk("DDDD"), "#0", "#0")
			p.rdItem(16)
			p.call("FOR$IO_END", true)
			p.showRd(16)
		})
		p.fab(10)
	})
	p.kase("an unformatted READ on a formatted unit, and the reverse", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fFmt))
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.closeU(10)
		p.oldStd(10, "STD.IDX")
		p.stmt(func() {
			p.call("FOR$READ_KO", false, "#10", "&FMTA", "&"+p.tk("DDDD"), "#0", "#0")
			p.rdItem(16)
			p.call("FOR$IO_END", true)
			p.showRd(16)
		})
	})
	p.kase("items: fewer bytes, and more than the record's", func() {
		p.oldStd(10, "STD.IDX")
		p.stmt(func() {
			p.call("FOR$READ_KU", false, "#10", "&"+p.tk("DDDD"), "#0", "#0")
			p.rdItem(4)
			p.call("FOR$IO_END", true)
			p.showRd(4)
		})
		p.stmt(func() {
			p.call("FOR$READ_KU", false, "#10", "&"+p.tk("DDDD"), "#0", "#0")
			p.rdItem(20)
			p.call("FOR$IO_END", true)
			p.showRd(20)
		})
	})
	p.kase("a keyed READ on a unit opened ACCESS SEQUENTIAL, and DIRECT", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aSeq), form(fUnf))
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.closeU(10)
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aDirect), form(fUnf))
		p.rdK(10, p.tk("DDDD"), 0, 0)
	})
	p.kase("a keyed READ of a sequential file, of a relative one, and of no unit", func() {
		p.open(10, file(p, "SQ.DAT"), stat(sNew), org(oSeq), recl(40))
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.closeU(10)
		p.open(10, file(p, "RL.DAT"), stat(sNew), org(oRel), acc(aDirect), form(fUnf), recl(4))
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.closeU(10)
		p.rdK(18, p.tk("DDDD"), 0, 0)
	})
	p.kase("no ERR=: signaled conditions", func() {
		p.openS(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fUnf))
		p.rdK(10, p.tk("CCCC"), 0, 0)
		p.rdK(10, p.tk("DDDD"), 9, 0)
		p.rdK(10, p.tk("DDDD"), 0, 5)
		p.openS(11, file(p, "NOSUCH.IDX"), stat(sOld), org(oIdx))
		p.openS(11, file(p, "A.IDX"), stat(sNew), org(oIdx), recl(40))
	})

	p.section("Sequential READs after keyed ones")

	p.kase("from the start, by the primary key, to the end", func() {
		p.oldStd(10, "STD.IDX")

		for i := 0; i < 9; i++ {
			p.rdU(10, 16)
		}

		p.fab(10)
	})
	p.kase("ACCESS SEQUENTIAL, to the end", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aSeq), form(fUnf))

		for i := 0; i < 9; i++ {
			p.rdU(10, 16)
		}

		p.fab(10)
	})

	for id, v := range []func() string{
		func() string { return p.tk("DDDD") },
		func() string { return p.intKey(25, 8) },
		func() string { return p.tk("rrrr") },
	} {
		p.kase(fmt.Sprintf("after a keyed READ by key %d", id), func() {
			p.oldStd(10, "STD.IDX")
			p.rdK(10, v(), id, 0)

			for i := 0; i < 9; i++ {
				p.rdU(10, 16)
			}

			p.fab(10)
		})
	}

	p.kase("after a failed keyed READ", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.tk("CCCC"), 0, 0)
		p.rdU(10, 16)
		p.rdU(10, 16)
	})
	p.kase("a keyed READ after the end", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.tk("NNNN"), 0, 0)
		p.rdU(10, 16)
		p.rdU(10, 16)
		p.rdK(10, p.tk("BBBB"), 0, 0)
		p.rdU(10, 16)
	})
	p.kase("duplicates: 40 twice, 'pppp' twice", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.intKey(40, 8), 1, 0)
		p.rdU(10, 16)
		p.rdU(10, 16)
		p.rdK(10, p.tk("pppp"), 2, 0)
		p.rdU(10, 16)
		p.rdU(10, 16)
	})
	p.kase("a formatted sequential READ", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fFmt))
		p.rdF(10, 16)
		p.rdF(10, 16)
	})

	p.section("WRITE")

	p.kase("keyed access: out of order, a duplicate primary, duplicate alternates", func() {
		p.open(10, file(p, "W1.IDX"), stat(sNew), org(oIdx), acc(aKeyed), form(fUnf), recl(4), p.stdKeys())
		p.wrRec(10, p.rec("MMMM", 1, "aaaa", 1))
		p.fab(10)
		p.wrRec(10, p.rec("CCCC", 2, "bbbb", 2))
		p.fab(10)
		p.wrRec(10, p.rec("MMMM", 3, "cccc", 3))
		p.fab(10)
		p.wrRec(10, p.rec("EEEE", 1, "aaaa", 4))
		p.fab(10)
		p.closeU(10)
		p.stat("W1.IDX", 1)
	})
	p.kase("sequential access: in order, out of order, a duplicate", func() {
		p.open(10, file(p, "W2.IDX"), stat(sNew), org(oIdx), acc(aSeq), form(fUnf), recl(4), p.stdKeys())
		p.fab(10)
		p.wrRec(10, p.rec("CCCC", 1, "aaaa", 1))
		p.fab(10)
		p.wrRec(10, p.rec("MMMM", 2, "bbbb", 2))
		p.fab(10)
		p.wrRec(10, p.rec("DDDD", 3, "cccc", 3))
		p.fab(10)
		p.wrRec(10, p.rec("MMMM", 4, "dddd", 4))
		p.fab(10)
		p.closeU(10)
		p.stat("W2.IDX", 1)
	})
	p.kase("short records: fixed, unformatted, padded; items past the record", func() {
		p.open(10, file(p, "W3.IDX"), stat(sNew), org(oIdx), acc(aKeyed), form(fUnf), recl(4), p.stdKeys())
		p.wrU(10, 8)
		p.wrU(10, 12)
		p.wrU(10, 16)
		p.wrU(10, 20)
		p.closeU(10)
		p.stat("W3.IDX", 1)
	})
	p.kase("short records: fixed, formatted, padded", func() {
		p.open(10, file(p, "W4.IDX"), stat(sNew), org(oIdx), acc(aKeyed), form(fFmt), recl(16), p.stdKeys())
		p.wrF(10, "AAAAbbbbcc")
		p.wrF(10, "CCCCbbbbccccdddd")
		p.wrF(10, "DDDDbbbbccccddddeeee")
		p.closeU(10)
		p.stat("W4.IDX", 1)
	})
	p.kase("variable records: shorter than an alternate key, than the primary key", func() {
		p.open(10, file(p, "W5.IDX"), stat(sNew), org(oIdx), acc(aKeyed), form(fFmt), imm(kRecType, 2), recl(40), key(p.keys(tStg, 1, 4, tStg, 9, 12)), uo)
		p.fab(10)
		p.wrF(10, "AAAA")
		p.wrF(10, "BBBBbbbbcccc")
		p.wrF(10, "CC")
		p.wrF(10, "DDDDbbbbcc")
		p.closeU(10)
		p.stat("W5.IDX", 1)
		p.open(10, file(p, "W5.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fFmt))
		p.rdK(10, p.tk("cccc"), 1, 0)
		p.rdK(10, p.tk("cc"), 1, 0)
	})
	p.kase("a READONLY unit, and a WRITE after a keyed READ", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fUnf), kw{"#^X008"})
		p.fab(10)
		p.wrRec(10, p.rec("PPPP", 1, "aaaa", 1))
		p.closeU(10)
		p.mkStd(10, "W6.IDX")
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.wrRec(10, p.rec("PPPP", 1, "aaaa", 1))
		p.fab(10)
		p.rdU(10, 16)
		p.closeU(10)
		p.stat("W6.IDX", 1)
	})
	p.kase("many records, past a bucket: ten buckets' worth", func() {
		p.open(10, file(p, "W7.IDX"), stat(sNew), org(oIdx), acc(aKeyed), form(fUnf), recl(4), p.stdKeys())

		for i := 0; i < 300; i++ {
			p.wrRec(10, p.rec(fmt.Sprintf("%04d", (i*37)%1000), i%7, fmt.Sprintf("%04d", i), i))
		}

		p.closeU(10)
		p.stat("W7.IDX", 0)
	})

	p.section("REWRITE")

	p.kase("the same keys, then a changed alternate, then a changed primary", func() {
		p.mkStd(10, "R1.IDX")
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 99))
		p.fab(10)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rwRec(10, p.rec("DDDD", 41, "yyyy", 98))
		p.fab(10)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rwRec(10, p.rec("EEEE", 41, "yyyy", 97))
		p.fab(10)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.closeU(10)
		p.stat("R1.IDX", 1)
	})
	p.kase("with no current record: at first, twice, after a WRITE, after a failed READ", func() {
		p.mkStd(10, "R2.IDX")
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 96))
		p.closeU(10)
		p.oldStd(10, "R2.IDX")
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 95))
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 94))
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 93))
		p.rdK(10, p.tk("CCCC"), 0, 0)
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 92))
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.wrRec(10, p.rec("PPPP", 1, "aaaa", 1))
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 91))
		p.closeU(10)
		p.stat("R2.IDX", 1)
	})
	p.kase("after a sequential READ, shorter, longer, and formatted", func() {
		p.mkStd(10, "R3.IDX")
		p.closeU(10)
		p.oldStd(10, "R3.IDX")
		p.rdU(10, 16)
		p.rwRec(10, p.rec("BBBB", 20, "pppp", 90))
		p.rdU(10, 16)
		p.rwU(10, 12)
		p.rdU(10, 16)
		p.rwU(10, 20)
		p.closeU(10)
		p.open(10, file(p, "R3.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fFmt))
		p.rdF(10, 16)
		p.rwF(10, "BBBBxxxxyyyy")
		p.closeU(10)
		p.stat("R3.IDX", 1)
	})
	p.kase("variable records: longer and shorter", func() {
		p.open(10, file(p, "R4.IDX"), stat(sNew), org(oIdx), acc(aKeyed), form(fFmt), imm(kRecType, 2), recl(40), key(p.keys(tStg, 1, 4)))
		p.wrF(10, "AAAAbbbb")
		p.wrF(10, "BBBBbbbb")
		p.closeU(10)
		p.open(10, file(p, "R4.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fFmt))
		p.rdF(10, 8)
		p.rwF(10, "AAAAbbbbccccdddd")
		p.rdF(10, 8)
		p.rwF(10, "BBBB")
		p.closeU(10)
		p.stat("R4.IDX", 1)
	})
	p.kase("a READONLY unit", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fUnf), kw{"#^X008"})
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rwRec(10, p.rec("DDDD", 40, "qqqq", 1))
	})

	p.section("DELETE and UNLOCK")

	p.kase("DELETE the current record, again, and read on", func() {
		p.mkStd(10, "D1.IDX")
		p.closeU(10)
		p.oldStd(10, "D1.IDX")
		p.unitCall("FOR$DELETE", 10)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.unitCall("FOR$DELETE", 10)
		p.fab(10)
		p.unitCall("FOR$DELETE", 10)
		p.rdU(10, 16)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rdK(10, p.intKey(40, 8), 1, 0)
		p.rdU(10, 16)
		p.unitCall("FOR$DELETE", 10)
		p.closeU(10)
		p.stat("D1.IDX", 1)
	})
	p.kase("DELETE by number, FIND, and direct access on an indexed unit", func() {
		p.oldStd(10, "STD.IDX")
		p.call("FOR$DELETE_D", true, "#10", "#2")
		p.call("FOR$FIND", true, "#10", "#2")
		p.rdDU(10, 2, 16)
		p.wrDU(10, 2, 16)
	})
	p.kase("DELETE on a sequential-access unit, a READONLY unit, a WRITE first", func() {
		p.mkStd(10, "D2.IDX")
		p.closeU(10)
		p.open(10, file(p, "D2.IDX"), stat(sOld), org(oIdx), acc(aSeq), form(fUnf))
		p.rdU(10, 16)
		p.unitCall("FOR$DELETE", 10)
		p.closeU(10)
		p.open(10, file(p, "D2.IDX"), stat(sOld), org(oIdx), acc(aSeq), form(fUnf), kw{"#^X008"})
		p.rdU(10, 16)
		p.unitCall("FOR$DELETE", 10)
		p.closeU(10)
		p.oldStd(10, "D2.IDX")
		p.rdK(10, p.tk("FFFF"), 0, 0)
		p.wrRec(10, p.rec("PPPP", 1, "aaaa", 1))
		p.unitCall("FOR$DELETE", 10)
		p.closeU(10)
		p.stat("D2.IDX", 1)
	})
	p.kase("two units on one file: a record locked, UNLOCK, and a WRITE past it", func() {
		p.mkStd(10, "L1.IDX")
		p.closeU(10)
		p.oldStd(10, "L1.IDX", kw{"#^X00D"})
		p.oldStd(11, "L1.IDX", kw{"#^X00D"})
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rdK(11, p.tk("DDDD"), 0, 0)
		p.rdK(11, p.tk("FFFF"), 0, 0)
		p.unitCall("FOR$UNLOCK", 10)
		p.rdK(11, p.tk("DDDD"), 0, 0)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rdU(10, 16)
		p.unitCall("FOR$UNLOCK", 11)
		p.rdK(11, p.tk("DDDD"), 0, 0)
		p.unitCall("FOR$UNLOCK", 11)
		p.unitCall("FOR$UNLOCK", 11)
		p.unitCall("FOR$UNLOCK", 18)
	})
	p.kase("not shared: a second unit", func() {
		p.oldStd(10, "STD.IDX")
		p.oldStd(11, "STD.IDX")
	})
	p.kase("a READONLY unit's locks", func() {
		p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fUnf), kw{"#^X008"}, kw{"#^X00D"})
		p.open(11, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(aKeyed), form(fUnf), kw{"#^X008"}, kw{"#^X00D"})
		p.fab(10)
		p.rdK(10, p.tk("DDDD"), 0, 0)
		p.rdK(11, p.tk("DDDD"), 0, 0)
	})

	p.section("REWIND, BACKSPACE, ENDFILE")

	for _, access := range []int{aKeyed, aSeq} {
		p.kase(fmt.Sprintf("access %d", access), func() {
			p.open(10, file(p, "STD.IDX"), stat(sOld), org(oIdx), acc(access), form(fUnf))
			p.rdU(10, 16)
			p.rdU(10, 16)
			p.unitCall("FOR$REWIND", 10)
			p.rdU(10, 16)
			p.unitCall("FOR$BACKSPACE", 10)
			p.rdU(10, 16)
			p.unitCall("FOR$ENDFILE", 10)
			p.rdU(10, 16)
			p.fab(10)
		})
	}

	p.kase("REWIND after a keyed READ by an alternate key: which order?", func() {
		p.oldStd(10, "STD.IDX")
		p.rdK(10, p.intKey(40, 8), 1, 0)
		p.unitCall("FOR$REWIND", 10)
		p.fab(10)
		p.rdU(10, 16)
		p.rdU(10, 16)
		p.rdK(10, p.tk("DDDD"), -1, 0)
		p.fab(10)
	})

	return p
}

const com = `$ ! FOR88.COM - round 2 of the Phase 88 probe: FORRTL on indexed files.
$ ! Written by testdata/for88/gen.go. Run it with the exchange volume's [000000]
$ ! as the default directory:
$ !
$ !     @FOR88/OUTPUT=FOR88.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST FOR88
$ LINK/NOMAP FOR88,FOR88/OPTIONS
$ ! Through a procedure with /OUTPUT, so that SYS$OUTPUT is a process-permanent file.
$ @FOR88B/OUTPUT=FOR88-VAX.TXT
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE=ALL/DATE *.IDX
$ DIRECTORY/FULL STD.IDX
$ ANALYZE/RMS_FILE/FDL/OUTPUT=STD.FDL STD.IDX
$ TYPE STD.FDL
$ ANALYZE/RMS_FILE/CHECK W7.IDX
$ ! The files FORRTL made, for comparing with FTNRTL's.
$ BACKUP *.IDX;* FOR88.BCK/SAVE_SET/LOG
$ EXIT
`

const copyout = `! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for88/gen.go. Run from the repository root:
!
!     govax console < testdata/for88/copyout.cmd
!
MOUNT DUA5 "testdata/disks/for88-exchange.dsk"
COPY DUA5:[000000]FOR88-VAX.TXT "testdata/for88/vax/for88-vax.txt"/HOST
COPY DUA5:[000000]FOR88.LOG "testdata/for88/vax/for88.log"/HOST
COPY DUA5:[000000]FOR88.BCK "testdata/for88/vax/for88.bck"/HOST
DISMOUNT DUA5
`

const exchange = `! EXCHANGE.CMD - builds round 2's exchange volume with govax.
! Written by testdata/for88/gen.go. Run from the repository root:
!
!     govax console < testdata/for88/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for88-exchange.dsk" /DEVICE=RD54 FOR88
MOUNT/WRITE DUA5 "testdata/disks/for88-exchange.dsk"
COPY "testdata/for88/for88.mar"/HOST DUA5:[000000]FOR88.MAR
COPY "testdata/for88/for88.opt"/HOST DUA5:[000000]FOR88.OPT
COPY "testdata/for88/for88.com"/HOST DUA5:[000000]FOR88.COM
COPY "testdata/for88/for88b.com"/HOST DUA5:[000000]FOR88B.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
`

func main() {
	p := cases()

	files := map[string]string{
		"for88.mar":    p.write("Phase 88's round 2: FORRTL on indexed files", forrtlCode),
		"for88.opt":    "SYS$SHARE:FORRTL/SHAREABLE\n",
		"for88.com":    com,
		"for88b.com":   "$ SET NOVERIFY\n$ RUN FOR88\n$ EXIT $STATUS\n",
		"exchange.cmd": exchange,
		"copyout.cmd":  copyout,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("for88.mar: %d cases\n", p.n)
}

const runtimeData = `	$SSDEF
	$FABDEF
	$RABDEF
	$NAMDEF
	$XABDEF
	$XABFHCDEF
	$XABKEYDEF
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
MRAB:	.ASCID	~ rab rop=!XL rac=!UB mbc=!UB mbf=!SB rsz=!UW usz=!UW ksz=!UB krf=!UB~
MUOK:	.ASCID	~ uo key ref=!UB dtp=!UB flg=!XB pos=!UW siz=!UB nsg=!UB dbs=!UB ibs=!UB dan=!UB ian=!UB lan=!UB dfl=!UW ifl=!UW nul=!UB knm=!XL~
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
	MOVZBL	RAB$B_KRF(R3),-(SP)
	MOVZBL	RAB$B_KSZ(R3),-(SP)
	MOVZWL	RAB$W_USZ(R3),-(SP)
	MOVZWL	RAB$W_RSZ(R3),-(SP)
	CVTBL	RAB$B_MBF(R3),-(SP)
	MOVZBL	RAB$B_MBC(R3),-(SP)
	MOVZBL	RAB$B_RAC(R3),-(SP)
	PUSHL	RAB$L_ROP(R3)
	PUSHAB	MRAB
	CALLS	#9,OUT
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
	CMPB	XAB$B_COD(R5),#XAB$C_KEY
	BNEQ	35$
	PUSHL	XAB$L_KNM(R5)
	MOVZBL	XAB$B_NUL(R5),-(SP)
	MOVZWL	XAB$W_IFL(R5),-(SP)
	MOVZWL	XAB$W_DFL(R5),-(SP)
	MOVZBL	XAB$B_LAN(R5),-(SP)
	MOVZBL	XAB$B_IAN(R5),-(SP)
	MOVZBL	XAB$B_DAN(R5),-(SP)
	MOVZBL	XAB$B_IBS(R5),-(SP)
	MOVZBL	XAB$B_DBS(R5),-(SP)
	MOVZBL	XAB$B_NSG(R5),-(SP)
	MOVZBL	XAB$B_SIZ0(R5),-(SP)
	MOVZWL	XAB$W_POS0(R5),-(SP)
	MOVZBL	XAB$B_FLG(R5),-(SP)
	MOVZBL	XAB$B_DTP(R5),-(SP)
	MOVZBL	XAB$B_REF(R5),-(SP)
	PUSHAB	MUOK
	CALLS	#16,OUT
35$:	MOVL	XAB$L_NXT(R5),R5
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

func (p *prog) write(title, extra string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\t.TITLE\t%s - %s\n; Written by testdata/for88/gen.go.\n", p.name, title)
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
