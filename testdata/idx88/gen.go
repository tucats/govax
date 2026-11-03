//go:build ignore

// gen.go writes the Phase 88 probe of RMS's indexed files (docs/PHASE-88 - keyed
// index files.md, subtask 2). VMS's run and the files it makes are the
// requirement for internal/isam and internal/rms. Run this from the repository
// root:
//
//	go run testdata/idx88/gen.go
//
// It writes
//
//	idx88.mar     RMS: every key shape, $CREATE's refusals, growth, finding,
//	              changing, sequence, and locks
//	idx88.com     builds and runs it, then ANALYZE/RMS_FILE on every file
//	idx88w.dat    the commands of ANALYZE/RMS_FILE/INTERACTIVE's scripted walks
//	def_*.mar     one internal definition macro each, GLOBAL, for its values
//	exchange.cmd, copyout.cmd
//
// A case is a routine of its own under HND, which prints a condition and unwinds
// to the driver, so a fault costs only that case. The driver closes every file
// after each case. The output is
//
//	#n  title
//	=WHAT r=x            a service, and its R0
//	 fab ... / rab ...   the control block after it
//	 got n= rfa= |text|  a record read: its size, RFA, and first bytes
//	 sum / key / area    what $OPEN and $DISPLAY say of a file
//	 vbn n               a file's blocks, by block I/O
//	   ooo hhhh...       one line of 32 bytes, in hex; all-zero lines left out
//	 mark text           a note from the case, saying what the next lines show
//
// Each record of the common layout is made by MKREC from a number n, so a line's
// bytes say which record they are (the Fields comment below).
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/idx88"

// Fields of MKREC's record, for n (FIELD offset, size):
const (
	fP    = 0  // string, "P" and 7 digits of n: the usual primary key
	fBN4  = 8  // unsigned longword n
	fIN4  = 12 // signed longword -n
	fA    = 16 // string, "A" and 7 digits of n/3: three records share each value
	fB    = 24 // string, "B" and 7 digits of 9999999-n: runs the other way
	fPAC  = 32 // packed decimal of n, 7 digits, 4 bytes
	fBN8  = 36 // unsigned quadword n
	fIN8  = 44 // signed quadword -n
	fRest = 52 // letters, with a run of blanks at 60 to 99, for data compression
)

// key is one XABKEY.
type key struct {
	ref              int
	dtp              string // STG, DSTG, BN2, ...; STG when empty
	pos, siz         []int
	flg              string // the macro's FLG=, "" for its default
	dan, ian, lan    int
	nul, dfl, ifl    int
	name             string
	prolog           int
	bad              func(p *prog, label string) // changes the XAB at run time, for a refusal
	hasNul           bool
}

// area is one XABALL.
type area struct {
	aid, bkz, alq, deq int
	aop                string
}

// spec is what a $CREATE asks for.
type spec struct {
	keys     []key
	areas    []area
	org      string // FAB$C_IDX unless given
	rfm      string // FAB$C_VAR unless given
	rat      string // FAB$M_CR unless given; "0" for none
	mrs, bks int
	alq, deq int
}

// prog collects the data, cases, and driver.
type prog struct {
	data  strings.Builder
	code  strings.Builder
	drive strings.Builder
	body  *strings.Builder
	n     int
	nxab  int
	texts map[string]string
	strs  map[string]string
	names map[string]string // a case's title to its file, for the scripted walks
	files []string          // every file a case makes
}

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

// str is a label for a .ASCII of s, a key's bytes.
func (p *prog) str(s string) string {
	if l, ok := p.strs[s]; ok {
		return l
	}

	l := fmt.Sprintf("Y%d", len(p.strs))
	p.strs[s] = l
	fmt.Fprintf(&p.data, "%s:\t.ASCII\t~%s~\n", l, s)

	return l
}

func (p *prog) emit(format string, a ...any) { fmt.Fprintf(p.body, "\t"+format+"\n", a...) }

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

// call calls a local routine: an int is pushed by value, "&x" as x's address, and
// any other string as it stands.
func (p *prog) call(routine string, args ...any) {
	for i := len(args) - 1; i >= 0; i-- {
		switch a := args[i].(type) {
		case int:
			p.emit("PUSHL\t#%d", a)
		case string:
			if strings.HasPrefix(a, "&") {
				p.emit("PUSHAB\t%s", a[1:])
			} else {
				p.emit("PUSHL\t%s", a)
			}
		}
	}

	p.emit("CALLS\t#%d,L^%s", len(args), routine)
}

func (p *prog) mark(s string) { p.call("MARK", "&"+p.text(s)) }

// xabs writes a spec's XAB chain into the data, returning its first label.
func (p *prog) xabs(s spec) (string, []string) {
	next := "0"

	var labels []string

	for i := len(s.areas) - 1; i >= 0; i-- {
		a := s.areas[i]
		p.nxab++
		l := fmt.Sprintf("X%d", p.nxab)
		fmt.Fprintf(&p.data, "\t.ALIGN\tLONG\n%s:\t$XABALL\tAID=%d,ALQ=%d,BKZ=%d,DEQ=%d,NXT=%s", l, a.aid, a.alq, a.bkz, a.deq, next)
		if a.aop != "" {
			fmt.Fprintf(&p.data, ",AOP=<%s>", a.aop)
		}
		p.data.WriteString("\n")
		next = l
	}

	labels = make([]string, len(s.keys))

	for i := len(s.keys) - 1; i >= 0; i-- {
		k := s.keys[i]
		p.nxab++
		l := fmt.Sprintf("X%d", p.nxab)
		labels[i] = l

		dtp := k.dtp
		if dtp == "" {
			dtp = "STG"
		}

		knm := ""

		if k.name != "" {
			knm = fmt.Sprintf(",KNM=%sN", l)
			fmt.Fprintf(&p.data, "%sN:\t.ASCII\t~%-32s~\n", l, k.name)
		}

		var extra strings.Builder

		if k.flg != "" {
			fmt.Fprintf(&extra, ",FLG=<%s>", k.flg)
		}

		if k.dan != 0 {
			fmt.Fprintf(&extra, ",DAN=%d", k.dan)
		}

		if k.ian != 0 {
			fmt.Fprintf(&extra, ",IAN=%d", k.ian)
		}

		if k.lan != 0 {
			fmt.Fprintf(&extra, ",LAN=%d", k.lan)
		}

		if k.hasNul {
			fmt.Fprintf(&extra, ",NUL=%d", k.nul)
		}

		if k.dfl != 0 {
			fmt.Fprintf(&extra, ",DFL=%d", k.dfl)
		}

		if k.ifl != 0 {
			fmt.Fprintf(&extra, ",IFL=%d", k.ifl)
		}

		if k.prolog != 0 {
			fmt.Fprintf(&extra, ",PROLOG=%d", k.prolog)
		}

		fmt.Fprintf(&p.data, "\t.ALIGN\tLONG\n%s:\t$XABKEY\tREF=%d,DTP=%s,POS=<%s>,SIZ=<%s>%s%s,NXT=%s\n",
			l, k.ref, dtp, list(k.pos), list(k.siz), extra.String(), knm, next)
		next = l
	}

	return next, labels
}

func list(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = fmt.Sprint(n)
	}

	return strings.Join(s, ",")
}

// create emits a $CREATE of name, by spec.
func (p *prog) create(name string, s spec) {
	first, labels := p.xabs(s)

	for i, k := range s.keys {
		if k.bad != nil {
			k.bad(p, labels[i])
		}
	}

	org, rfm, rat := s.org, s.rfm, s.rat
	if org == "" {
		org = "FAB$C_IDX"
	}

	if rfm == "" {
		rfm = "FAB$C_VAR"
	}

	if rat == "" {
		rat = "FAB$M_CR"
	}

	p.emit("MOVB\t#%s,ORGV", org)
	p.emit("MOVB\t#%s,RFMV", rfm)
	p.emit("MOVB\t#%s,RATV", rat)
	p.emit("MOVW\t#%d,MRSV", s.mrs)
	p.emit("MOVB\t#%d,BKSV", s.bks)
	p.emit("MOVL\t#%d,ALQV", s.alq)
	p.emit("MOVW\t#%d,DEQV", s.deq)
	if first == "0" {
		p.emit("CLRL\tXABV")
	} else {
		p.emit("MOVAB\t%s,XABV", first)
	}

	p.call("CRE", "&"+p.text(name))
}

func (p *prog) closeF()          { p.call("CLO") }
func (p *prog) open(name string) { p.call("OPN", "&"+p.text(name), 0) }
func (p *prog) reopen(n string)  { p.call("OPN", "&"+p.text(n), 1) } // no key report
func (p *prog) dump(name string) { p.call("DUMPF", "&"+p.text(name)) }

// put writes record n of size bytes in the common layout, by key: in sequence a
// record out of order is RMS$_SEQ, which round 1's first full run found out.
func (p *prog) put(n, size int) { p.call("PUTN", n, size, 0, 1) }

// putK is put by RAC KEY, with options.
func (p *prog) putK(n, size int, rop string) { p.call("PUTN", n, size, rop, 1) }

// puts writes count records quietly: first, first+step, ..., modulo mod if not 0.
func (p *prog) puts(first, count, step, mod, size int) {
	p.call("PUTS", first, count, step, mod, size, 1)
}

// mk fills the record buffer with record n, for a patch and then putb or updb.
func (p *prog) mk(n int) { p.call("MKREC", n) }

// patch overwrites bytes of the record buffer from a string.
func (p *prog) patch(off int, s string) {
	p.emit("MOVC3\t#%d,%s,BIGB+%d", len(s), p.str(s), off)
}

// patchL overwrites a longword of the record buffer.
func (p *prog) patchL(off, v int) { p.emit("MOVL\t#%d,BIGB+%d", v, off) }

func (p *prog) putb(size int, rac int)   { p.call("PUTB", size, 0, rac) }
func (p *prog) updb(size int)            { p.call("UPDB", size, 0) }
func (p *prog) del(rop string)           { p.call("DEL", rop) }
func (p *prog) get(krf int, rop string)  { p.call("GETK", 0, krf, 0, 0, rop, 0) } // in sequence
func (p *prog) find(krf int, rop string) { p.call("GETK", 1, krf, 0, 0, rop, 0) }
func (p *prog) rewind(krf int)           { p.call("REW", krf) }
func (p *prog) connect(krf int, rop string) {
	p.call("CON", krf, rop)
}

// getKey is a keyed $GET (svc 0) or $FIND (svc 1) by bytes.
func (p *prog) getKey(svc, krf int, s string, rop string) {
	p.call("GETK", svc, krf, "&"+p.str(s), len(s), rop, 1)
}

// getField is a keyed $GET by record n's field at off, ksz bytes of it.
func (p *prog) getField(svc, krf, n, off, ksz int, rop string) {
	p.call("MKKEY", n)
	p.call("GETK", svc, krf, fmt.Sprintf("&KEYB+%d", off), ksz, rop, 1)
}

func (p *prog) saveRFA(slot int) { p.call("SAVER", slot) }
func (p *prog) getRFA(slot int)  { p.call("SETR", slot); p.call("GETK", 0, 0, 0, 0, "#0", 2) }

// The ROP options, as numbers: not every name need be known to VMS 7.1's macros.
const (
	ropNone  = "#0"
	ropUIF   = "#^X10"
	ropFDL   = "#^X40"
	ropREV   = 0x80
	ropEOF   = "#^X100"
	ropCDK   = "#^X1000"
	ropLOA   = "#^X2000"
	ropLIM   = "#^X4000"
	ropKGE   = 0x200000
	ropKGT   = 0x400000
	ropNLK   = "#^X100000"
	ropRLK   = "#^X80000"
	ropRRL   = "#^X8"
	ropREA   = "#^X4"
	ropULK   = "#^X40000"
	ropNXR   = "#^X800000"
	ropWAT   = "#^X20000"
	ropTMO   = "#^X2000000"
	shrAll   = 0x0F // SHRPUT, SHRGET, SHRDEL, SHRUPD
	noCmprs  = "DAT_NCMPR,KEY_NCMPR,IDX_NCMPR"
)

func prim(pos, siz int) key { return key{ref: 0, pos: []int{pos}, siz: []int{siz}} }

func alt(ref, pos, siz int, flg string) key {
	return key{ref: ref, pos: []int{pos}, siz: []int{siz}, flg: flg}
}

// typed is an alternate key of a data type of its own.
func typed(ref, pos, siz int, dtp, flg string) key {
	k := alt(ref, pos, siz, flg)
	k.dtp = dtp
	return k
}

// cases writes every case.
func cases() *prog {
	p := &prog{texts: map[string]string{}, strs: map[string]string{}, names: map[string]string{}}
	fn := 0
	fresh := func(prefix string) string {
		fn++
		name := fmt.Sprintf("%s%d", prefix, fn)
		p.files = append(p.files, name)
		return name + ".IDX"
	}

	p.section("the helpers themselves, on a sequential file (for a rehearsal over any RMS)")

	p.kase("helpers on a sequential file", func() {
		n := "HELPERS.DAT"
		p.create(n, spec{org: "FAB$C_SEQ", mrs: 120})
		p.call("PUTN", 1, 80, 0, 0)
		p.call("PUTS", 2, 5, 1, 0, 100, 0)
		p.closeF()
		p.open(n)
		p.get(0, ropNone)
		p.saveRFA(0)
		p.get(0, ropNone)
		p.getRFA(0)
		p.closeF()
		p.dump(n)
	})

	p.section("key shapes: each type and flag, five records, read back and dumped")

	shape := func(title string, s spec, size int, recs ...int) {
		name := fresh("S")
		p.kase(title, func() {
			if s.mrs == 0 {
				s.mrs = 120
			}
			if len(recs) == 0 {
				recs = []int{5, 1, 3, 2, 4}
			}
			p.names[title] = name
			p.create(name, s)
			for _, r := range recs {
				if r >= 0 {
					p.put(r, size)
				}
			}
			p.closeF()
			p.open(name)
			p.get(0, ropNone)
			p.closeF()
			p.dump(name)
		})
	}
	shape("STG 8, defaults", spec{keys: []key{prim(fP, 8)}}, 100)
	shape("STG 8, no records", spec{keys: []key{prim(fP, 8)}}, 100, -1)
	shape("STG 4 (short of compression's 6)", spec{keys: []key{prim(4, 4)}}, 100)
	shape("STG 5", spec{keys: []key{prim(3, 5)}}, 100)
	shape("STG 6", spec{keys: []key{prim(2, 6)}}, 100)
	shape("STG 8 at position 4 of the record", spec{keys: []key{prim(16, 8)}}, 100)
	for _, f := range []string{"DAT_NCMPR", "KEY_NCMPR", "IDX_NCMPR", "DAT_NCMPR,KEY_NCMPR", noCmprs} {
		k := prim(fP, 8)
		k.flg = f
		shape("STG 8, FLG="+f, spec{keys: []key{k}}, 100)
	}
	shape("FIX 100, defaults", spec{keys: []key{prim(fP, 8)}, rfm: "FAB$C_FIX", mrs: 100}, 100)
	func() {
		k := prim(fP, 8)
		k.flg = noCmprs
		shape("FIX 100, no compression", spec{keys: []key{k}, rfm: "FAB$C_FIX", mrs: 100}, 100)
	}()
	shape("VAR, MRS 0", spec{keys: []key{prim(fP, 8)}, mrs: -1}, 100)
	shape("RAT 0", spec{keys: []key{prim(fP, 8)}, rat: "0"}, 100)

	for _, t := range []struct {
		dtp      string
		pos, siz int
	}{
		{"BN2", fBN4, 2}, {"BN4", fBN4, 4}, {"BN8", fBN8, 8},
		{"IN2", fIN4, 2}, {"IN4", fIN4, 4}, {"IN8", fIN8, 8},
		{"PAC", fPAC, 4}, {"DSTG", fP, 8},
		{"DBN2", fBN4, 2}, {"DBN4", fBN4, 4}, {"DBN8", fBN8, 8},
		{"DIN2", fIN4, 2}, {"DIN4", fIN4, 4}, {"DIN8", fIN8, 8},
		{"DPAC", fPAC, 4},
	} {
		k := prim(t.pos, t.siz)
		k.dtp = t.dtp
		shape("primary "+t.dtp, spec{keys: []key{k}}, 100)
	}
	shape("primary with DUP", spec{keys: []key{func() key { k := prim(fA, 8); k.flg = "DUP"; return k }()}}, 100)
	shape("segmented STG, primary", spec{keys: []key{{pos: []int{fA, fP}, siz: []int{4, 4}}}}, 100)
	shape("segmented STG, 8 segments", spec{keys: []key{{pos: []int{0, 2, 4, 6, 16, 18, 20, 22}, siz: []int{2, 2, 2, 2, 2, 2, 2, 2}}}}, 100)
	shape("segmented STG, overlapping, primary", spec{keys: []key{{pos: []int{0, 2}, siz: []int{4, 4}}}}, 100)
	shape("two keys, the alternate by the macro's defaults",
		spec{keys: []key{prim(fP, 8), alt(1, fA, 8, "")}}, 100)
	shape("three keys, numeric alternates",
		spec{keys: []key{prim(fP, 8), typed(1, fBN4, 4, "BN4", ""), typed(2, fIN4, 4, "IN4", "CHG")}}, 100)
	func() {
		a := alt(1, fA, 8, "DUP")
		a.name = "ALTERNATE"
		k := prim(fP, 8)
		k.name = "PRIMARY"
		shape("named keys", spec{keys: []key{k, a}}, 100)
	}()
	for _, f := range []string{"DUP", "CHG", "DUP,CHG", "NUL", "DUP,NUL", "DUP,CHG,NUL", "DAT_NCMPR", "KEY_NCMPR,IDX_NCMPR"} {
		shape("alternate STG, FLG="+f, spec{keys: []key{prim(fP, 8), alt(1, fA, 8, f)}}, 100)
	}
	func() {
		shape("alternate BN4, NUL", spec{keys: []key{prim(fP, 8), typed(1, fBN4, 4, "BN4", "NUL")}}, 100)
		b := alt(1, 200, 8, "DUP")
		shape("an alternate past some records' ends", spec{keys: []key{prim(fP, 8), b}, mrs: 300}, 100)
	}()
	for _, b := range []int{1, 2, 3, 4, 7, 63} {
		shape(fmt.Sprintf("BKS %d", b), spec{keys: []key{prim(fP, 8)}, bks: b}, 100)
	}
	shape("MRS 5000, BKS 0", spec{keys: []key{prim(fP, 8)}, mrs: 5000}, 100)
	shape("MRS 32224", spec{keys: []key{prim(fP, 8)}, mrs: 32224, bks: 63}, 100)
	shape("ALQ 50, DEQ 7", spec{keys: []key{prim(fP, 8)}, alq: 50, deq: 7}, 100)
	func() {
		k := prim(fP, 8)
		k.dfl, k.ifl = 200, 300
		shape("DFL 200, IFL 300", spec{keys: []key{k}}, 100)
		k = prim(fP, 8)
		k.prolog = 3
		shape("PROLOG=3", spec{keys: []key{k}}, 100)
		k = prim(fP, 8)
		k.prolog = 2
		shape("PROLOG=2", spec{keys: []key{k}}, 100)
		k = prim(fP, 8)
		k.prolog = 1
		shape("PROLOG=1", spec{keys: []key{k}}, 100)
		kk := prim(fBN8, 8)
		kk.dtp, kk.prolog = "BN8", 2
		shape("PROLOG=2 with BN8", spec{keys: []key{kk}}, 100)
	}()

	p.section("areas: XABALLs and each key's DAN, LAN, IAN")

	func() {
		k0 := prim(fP, 8)
		k0.ian, k0.lan = 1, 1
		a1 := alt(1, fA, 8, "DUP")
		a1.dan, a1.ian, a1.lan = 2, 2, 2
		shape("3 areas: data, index, alternate", spec{keys: []key{k0, a1}, areas: []area{
			{aid: 0, bkz: 3, alq: 9}, {aid: 1, bkz: 2, alq: 4}, {aid: 2, bkz: 2, alq: 6},
		}}, 100)
		k1 := prim(fP, 8)
		k1.ian, k1.lan = 2, 1
		shape("3 areas, LAN apart from IAN", spec{keys: []key{k1}, areas: []area{
			{aid: 0, bkz: 2, alq: 8}, {aid: 1, bkz: 2, alq: 4}, {aid: 2, bkz: 2, alq: 4},
		}}, 100)
		shape("1 XABALL, AID 0, CBT, DEQ", spec{keys: []key{prim(fP, 8)}, areas: []area{
			{aid: 0, bkz: 2, alq: 12, deq: 4, aop: "CBT"},
		}}, 100)
		shape("1 XABALL, CTG", spec{keys: []key{prim(fP, 8)}, areas: []area{
			{aid: 0, bkz: 2, alq: 12, aop: "CTG"},
		}}, 100)
		k2 := prim(fP, 8)
		k2.ian, k2.lan = 1, 1
		name := fresh("S")
		p.kase("an area filled past its allocation, DEQ 2", func() {
			p.create(name, spec{mrs: 120, keys: []key{k2}, areas: []area{
				{aid: 0, bkz: 1, alq: 2, deq: 2}, {aid: 1, bkz: 1, alq: 1, deq: 1},
			}})
			p.puts(1, 40, 1, 0, 100)
			p.closeF()
			p.open(name)
			p.closeF()
			p.dump(name)
		})
	}()

	p.section("$CREATE's refusals: one bad field at a time, with STS and STV")

	refuse := func(title string, s spec) {
		name := fresh("E")
		if s.mrs == 0 {
			s.mrs = 120
		}
		p.kase(title, func() {
			p.create(name, s)
			p.closeF()
		})
	}
	poke := func(k key, field string, v int, op string) key {
		k.bad = func(p *prog, l string) { p.emit("%s\t#%d,%s+%s", op, v, l, field) }
		return k
	}
	refuse("no XABKEY", spec{})
	refuse("XABKEY for key 1 only", spec{keys: []key{alt(1, fA, 8, "")}})
	refuse("keys 0 and 2", spec{keys: []key{prim(fP, 8), alt(2, fA, 8, "")}})
	refuse("two key 0s", spec{keys: []key{prim(fP, 8), prim(fA, 8)}})
	refuse("alternate before primary", spec{keys: []key{alt(1, fA, 8, ""), prim(fP, 8)}})
	// By poking: the macro itself refuses CHG on a primary key at assembly time.
	refuse("primary with CHG", spec{keys: []key{poke(prim(fP, 8), "XAB$B_FLG", 2, "MOVB")}})
	refuse("primary with NUL", spec{keys: []key{poke(prim(fP, 8), "XAB$B_FLG", 4, "MOVB")}})
	refuse("primary past MRS", spec{keys: []key{prim(fP+200, 8)}})
	refuse("size 0", spec{keys: []key{poke(prim(fP, 8), "XAB$B_SIZ0", 0, "MOVB")}})
	refuse("DTP 9", spec{keys: []key{poke(prim(fP, 8), "XAB$B_DTP", 9, "MOVB")}})
	refuse("DTP 200", spec{keys: []key{poke(prim(fP, 8), "XAB$B_DTP", 200, "MOVB")}})
	refuse("FLG all ones", spec{keys: []key{poke(prim(fP, 8), "XAB$B_FLG", 255, "MOVB")}})
	refuse("DAN 3 with no XABALL", spec{keys: []key{func() key { k := prim(fP, 8); k.dan = 3; return k }()}})
	refuse("IAN 3 with no XABALL", spec{keys: []key{func() key { k := prim(fP, 8); k.ian = 3; return k }()}})
	refuse("LAN 3 with no XABALL", spec{keys: []key{func() key { k := prim(fP, 8); k.lan = 3; return k }()}})
	refuse("DFL past the bucket", spec{keys: []key{func() key { k := prim(fP, 8); k.dfl = 30000; return k }()}})
	refuse("IFL past the bucket", spec{keys: []key{func() key { k := prim(fP, 8); k.ifl = 30000; return k }()}})
	refuse("DFL below half the bucket", spec{keys: []key{func() key { k := prim(fP, 8); k.dfl = 10; return k }()}})
	refuse("KNM a bad address", spec{keys: []key{poke(prim(fP, 8), "XAB$L_KNM", 16, "MOVL")}})
	// These too by poking: real MACRO's $XABKEY refuses each at assembly time
	// (the first run's IDX88M.LOG).
	refuse("BN4 of size 2", spec{keys: []key{poke(typed(0, fBN4, 4, "BN4", ""), "XAB$B_SIZ0", 2, "MOVB")}})
	refuse("BN2 of two segments", spec{keys: []key{func() key {
		k := typed(0, fBN4, 2, "BN2", "")
		k.bad = func(p *prog, l string) {
			p.emit("MOVB\t#1,%s+XAB$B_SIZ0", l)
			p.emit("MOVB\t#1,%s+XAB$B_SIZ0+1", l)
			p.emit("MOVW\t#%d,%s+XAB$W_POS0+2", fIN4, l)
		}
		return k
	}()}})
	refuse("PAC of size 17", spec{keys: []key{poke(typed(0, fPAC, 4, "PAC", ""), "XAB$B_SIZ0", 17, "MOVB")}})
	refuse("STG 256 by NSG 2", spec{mrs: 400, keys: []key{poke(key{pos: []int{0, 100}, siz: []int{200, 10}}, "XAB$B_SIZ0+1", 56, "MOVB")}})
	refuse("BKS 64", spec{keys: []key{prim(fP, 8)}, bks: 64})
	refuse("BKS 1 and MRS 600", spec{keys: []key{prim(fP, 8)}, bks: 1, mrs: 600})
	refuse("MRS 32225", spec{keys: []key{prim(fP, 8)}, bks: 63, mrs: 32225})
	refuse("MRS 32234", spec{keys: []key{prim(fP, 8)}, bks: 63, mrs: 32234})
	refuse("RFM VFC", spec{keys: []key{prim(fP, 8)}, rfm: "FAB$C_VFC"})
	refuse("RFM STM", spec{keys: []key{prim(fP, 8)}, rfm: "FAB$C_STM"})
	refuse("RFM UDF", spec{keys: []key{prim(fP, 8)}, rfm: "FAB$C_UDF"})
	refuse("FIX with MRS 0", spec{keys: []key{prim(fP, 8)}, rfm: "FAB$C_FIX", mrs: -1})
	refuse("FIX shorter than the key", spec{keys: []key{prim(fP, 8)}, rfm: "FAB$C_FIX", mrs: 6})
	refuse("XABALL AID 1 only", spec{keys: []key{prim(fP, 8)}, areas: []area{{aid: 1, bkz: 2, alq: 4}}})
	refuse("XABALLs 0 and 2", spec{keys: []key{prim(fP, 8)}, areas: []area{{aid: 0, bkz: 2, alq: 4}, {aid: 2, bkz: 2, alq: 4}}})
	refuse("XABALL BKZ 64", spec{keys: []key{prim(fP, 8)}, areas: []area{{aid: 0, bkz: 64, alq: 4}}})
	refuse("PROLOG 9", spec{keys: []key{func() key { k := prim(fP, 8); k.prolog = 9; return k }()}})
	func() {
		name := fresh("E")
		p.kase("FOP CIF over an existing file, and $CREATE again without it", func() {
			s := spec{mrs: 120, keys: []key{prim(fP, 8)}}
			p.create(name, s)
			p.put(1, 100)
			p.closeF()
			p.emit("MOVL\t#FAB$M_CIF,FOPV")
			p.create(name, s)
			p.emit("CLRL\tFOPV")
			p.get(0, ropNone)
			p.closeF()
			p.create(name, s)
			p.closeF()
		})
	}()

	p.section("growth: a split's place, IDs, RRVs, and check characters, one step at a time")

	grow := func(title string, s spec, size int, order []int) {
		name := fresh("G")
		p.kase(title, func() {
			p.create(name, s)
			for i, r := range order {
				p.put(r, size)
				p.closeF()
				p.mark(fmt.Sprintf("after record %d, the %dth", r, i+1))
				p.dump(name)
				p.reopen(name)
			}
			p.closeF()
			p.open(name)
			p.mark("every record in order, with its RFA")
			p.call("WALK", 0, len(order))
			p.closeF()
		})
	}
	ord := func(first, n, step, mod int) []int {
		var o []int
		for i := 0; i < n; i++ {
			v := first + i*step
			if mod != 0 {
				v = v%mod + 1
			}
			o = append(o, v)
		}
		return o
	}
	small := func(flg string, keys ...key) spec {
		k := prim(fP, 8)
		k.flg = flg
		return spec{mrs: 200, bks: 1, keys: append([]key{k}, keys...)}
	}
	grow("in order, no compression, BKS 1, record 150", small(noCmprs), 150, ord(1, 16, 1, 0))
	grow("reversed, no compression, BKS 1, record 150", small(noCmprs), 150, ord(16, 16, -1, 0))
	grow("scattered, no compression, BKS 1, record 150", small(noCmprs), 150, ord(0, 16, 7, 17))
	grow("in order, compressed, BKS 1, record 150", small(""), 150, ord(1, 16, 1, 0))
	grow("scattered, compressed, BKS 1, record 150", small(""), 150, ord(0, 16, 7, 17))
	grow("FIX 150, no compression, BKS 1, scattered", func() spec {
		s := small(noCmprs)
		s.rfm, s.mrs = "FAB$C_FIX", 150
		return s
	}(), 150, ord(0, 12, 7, 17))
	grow("BKS 2, record 300, scattered", func() spec {
		s := small(noCmprs)
		s.bks, s.mrs = 2, 400
		return s
	}(), 300, ord(0, 12, 7, 17))
	grow("an alternate with duplicates, BKS 1", small(noCmprs, alt(1, fA, 8, "DUP,"+noCmprs)), 150, ord(1, 16, 1, 0))
	grow("an alternate, no duplicates, numeric, BKS 1", small(noCmprs, typed(1, fBN4, 4, "BN4", "CHG")), 150, ord(0, 16, 7, 17))
	func() {
		k := prim(fA, 8)
		k.flg = "DUP," + noCmprs
		grow("a duplicate primary key, past one bucket", spec{mrs: 200, bks: 1, keys: []key{k}}, 150, []int{3, 4, 5, 3, 4, 5, 3, 4, 5, 3, 4, 5})
		k2 := prim(fP, 8)
		k2.flg = noCmprs
		name := fresh("G")
		p.kase("one alternate value on 400 records: its pointers past one SIDR bucket", func() {
			p.create(name, spec{mrs: 200, bks: 1, keys: []key{k2, alt(1, 100, 4, "DUP,"+noCmprs)}})
			p.puts(1, 400, 1, 0, 150)
			p.closeF()
			p.open(name)
			p.closeF()
			p.dump(name)
			p.reopen(name)
			p.call("WALK", 1, 400)
			p.closeF()
		})
	}()
	for _, g := range []struct {
		title string
		flg   string
		count int
		size  int
		step  int
		mod   int
	}{
		{"3 index levels: no compression", noCmprs, 1200, 200, 1, 0},
		{"3 index levels: compressed", "", 1500, 200, 1, 0},
		{"scattered 500, compressed", "", 500, 100, 211, 997},
		{"scattered 500, no compression", noCmprs, 500, 100, 211, 997},
	} {
		g := g
		name := fresh("G")
		p.kase(g.title, func() {
			k := prim(fP, 8)
			k.flg = g.flg
			p.create(name, spec{mrs: 400, bks: 1, keys: []key{k, alt(1, fA, 8, "DUP")}})
			p.puts(1, g.count, g.step, g.mod, g.size)
			p.closeF()
			p.open(name)
			p.call("WALK", 0, g.count)
			p.closeF()
			p.dump(name)
		})
	}
	func() {
		k := prim(fP, 8)
		k.dfl, k.ifl = 300, 300
		name := fresh("G")
		p.kase("ROP LOA with fill sizes, then filling past them without it", func() {
			p.create(name, spec{mrs: 200, bks: 1, keys: []key{k}})
			p.emit("MOVL\t%s,ROPV", ropLOA)
			p.puts(1, 8, 2, 0, 100)
			p.emit("CLRL\tROPV")
			p.closeF()
			p.dump(name)
			p.reopen(name)
			p.puts(2, 8, 2, 0, 100)
			p.closeF()
			p.dump(name)
		})
	}()

	p.section("finding: the manual's Tables 7-3 and 7-4")

	combos := []struct {
		name string
		rop  int
	}{
		{"0", 0}, {"KGE", ropKGE}, {"KGT", ropKGT}, {"KGE+KGT", ropKGE | ropKGT},
		{"REV", ropREV}, {"REV+KGE", ropREV | ropKGE}, {"REV+KGT", ropREV | ropKGT}, {"REV+KGE+KGT", ropREV | ropKGE | ropKGT},
	}
	for _, d := range []string{"STG", "DSTG"} {
		d := d
		name := fresh("F")
		p.kase("keys B, K, Q; "+d+"; keys A, K, Z under every option", func() {
			k := prim(fP, 1)
			k.dtp = d
			p.create(name, spec{mrs: 120, keys: []key{k}})
			for i, c := range []string{"B", "K", "Q"} {
				p.mk(i + 1)
				p.patch(0, c)
				p.putb(100, 1)
			}
			for _, c := range combos {
				for _, s := range []string{"A", "K", "Z"} {
					p.mark("ROP " + c.name + ", key " + s)
					p.getKey(0, 0, s, fmt.Sprintf("#%d", c.rop))
				}
			}
			p.closeF()
		})
	}

	p.section("finding: exact, generic, numeric, and each key's order")

	fbase := fresh("F")
	p.kase("the file for this section: 300 records of every key type", func() {
		p.create(fbase, spec{mrs: 200, keys: []key{
			prim(fP, 8),
			typed(1, fBN4, 4, "BN4", ""), typed(2, fIN4, 4, "IN4", ""),
			alt(3, fA, 8, "DUP"), typed(4, fB, 8, "DSTG", ""),
			typed(5, fPAC, 4, "PAC", ""), typed(6, fBN8, 8, "BN8", ""), typed(7, fIN8, 8, "DIN8", ""),
		}})
		p.puts(0, 300, 3, 301, 120)
		p.closeF()
		p.open(fbase)
		p.closeF()
	})
	onFile := func(title string, f func()) {
		p.kase(title, func() {
			p.reopen(fbase)
			f()
			p.closeF()
		})
	}
	onFile("exact: each key, a record there and a record not", func() {
		for _, c := range []struct{ krf, off, size int }{{0, fP, 8}, {1, fBN4, 4}, {2, fIN4, 4}, {3, fA, 8}, {4, fB, 8}, {5, fPAC, 4}, {6, fBN8, 8}, {7, fIN8, 8}} {
			p.mark(fmt.Sprintf("key %d: record 100, then 100000", c.krf))
			p.getField(0, c.krf, 100, c.off, c.size, ropNone)
			p.getField(0, c.krf, 100000, c.off, c.size, ropNone)
		}
	})
	onFile("in sequence by each key: from the start, by key of reference", func() {
		for k := 0; k < 8; k++ {
			p.connect(k, ropNone)
			p.mark(fmt.Sprintf("key %d in sequence, the first six", k))
			for i := 0; i < 6; i++ {
				p.get(k, ropNone)
			}
		}
	})
	onFile("keyed then in sequence: which key it goes on by", func() {
		p.getField(0, 3, 100, fA, 8, ropNone)
		p.get(0, ropNone)
		p.get(0, ropNone)
		p.getField(0, 4, 100, fB, 8, ropNone)
		p.get(0, ropNone)
		p.get(3, ropNone)
		p.saveRFA(0)
		p.getRFA(0)
		p.mark("after an RFA access: the primary key, by the manual")
		p.get(0, ropNone)
	})
	onFile("generic: every length, with each option", func() {
		for _, s := range []string{"P", "P0", "P00001", "P0000100", "P000010", "P9", "A000003", "Q"} {
			for _, c := range combos[:3] {
				p.mark("key |" + s + "| " + c.name)
				p.getKey(0, 0, s, fmt.Sprintf("#%d", c.rop))
			}
		}
		for _, c := range combos[4:7] {
			p.mark("REV, generic P00001, " + c.name)
			p.getKey(0, 0, "P00001", fmt.Sprintf("#%d", c.rop))
		}
	})
	onFile("generic on a descending key", func() {
		for _, s := range []string{"B9", "B99999", "A", "C"} {
			for _, c := range combos[:3] {
				p.mark("key 4 |" + s + "| " + c.name)
				p.getKey(0, 4, s, fmt.Sprintf("#%d", c.rop))
			}
		}
	})
	onFile("KSZ: too long, zero, numeric wrong and right, KRF past the keys", func() {
		p.getField(0, 0, 100, fP, 9, ropNone)
		p.getField(0, 0, 100, fP, 0, ropNone)
		p.getField(0, 1, 100, fBN4, 0, ropNone)
		p.getField(0, 1, 100, fBN4, 2, ropNone)
		p.getField(0, 1, 100, fBN4, 8, ropNone)
		p.getField(0, 5, 100, fPAC, 0, ropNone)
		p.getField(0, 8, 100, fP, 8, ropNone)
		p.getField(0, 255, 100, fP, 8, ropNone)
		p.call("GETK", 0, 0, 16, 8, ropNone, 1)
	})
	onFile("numeric options and the signs", func() {
		for _, c := range combos[:3] {
			for _, n := range []int{100, 101, 100000} {
				p.mark(fmt.Sprintf("BN4 near %d, %s", n, c.name))
				p.getField(0, 1, n, fBN4, 4, fmt.Sprintf("#%d", c.rop))
				p.mark(fmt.Sprintf("IN4 near -%d, %s", n, c.name))
				p.getField(0, 2, n, fIN4, 4, fmt.Sprintf("#%d", c.rop))
				p.mark(fmt.Sprintf("DIN8 near -%d, %s", n, c.name))
				p.getField(0, 7, n, fIN8, 8, fmt.Sprintf("#%d", c.rop))
			}
		}
	})
	onFile("LIM, CDK, USZ too small", func() {
		p.getField(1, 3, 100, fA, 8, ropNone)
		p.mark("LIM on the duplicate set's key, in sequence")
		for i := 0; i < 5; i++ {
			p.call("MKKEY", 100)
			p.call("GETK", 0, 3, fmt.Sprintf("&KEYB+%d", fA), 8, ropLIM, 0)
		}
		p.mark("CDK: a key with duplicates, then one without")
		p.getField(0, 3, 100, fA, 8, ropCDK)
		p.get(3, ropCDK)
		p.get(3, ropCDK)
		p.getField(0, 0, 100, fP, 8, ropCDK)
		p.mark("USZ 10")
		p.emit("MOVW\t#10,USZV")
		p.getField(0, 0, 100, fP, 8, ropNone)
		p.emit("MOVW\t#32000,USZV")
	})
	onFile("$REWIND by key, $FIND then $GET, Table 8-3's cases", func() {
		p.rewind(3)
		p.get(3, ropNone)
		p.rewind(0)
		p.get(0, ropNone)
		p.get(0, ropNone)
		p.find(0, ropNone)
		p.get(0, ropNone)
		p.get(0, ropNone)
		p.getField(1, 0, 100, fP, 8, ropNone)
		p.get(0, ropNone)
		p.get(0, ropNone)
		p.getField(1, 0, 100, fP, 8, ropNone)
		p.getField(1, 0, 200, fP, 8, ropNone)
		p.get(0, ropNone)
		p.get(0, ropNone)
		p.mark("a $GET at the end")
		p.getField(0, 0, 9999999, fP, 8, fmt.Sprintf("#%d", ropKGE))
		p.get(0, ropNone)
		p.get(0, ropNone)
	})
	onFile("not allowed on an indexed file", func() {
		p.call("TRU")
		p.call("SEQW", 1, 100)
	})

	p.section("sequence: $PUT in sequence and at the end")

	seqName := fresh("Q")
	p.kase("RAC SEQ: in order, out of order, equal", func() {
		p.create(seqName, spec{mrs: 120, keys: []key{prim(fP, 8)}})
		for _, r := range []int{10, 20, 15, 20, 30, 25} {
			p.call("PUTN", r, 100, 0, 0)
		}
		p.closeF()
		p.dump(seqName)
	})
	seqBig := fresh("Q")
	p.kase("RAC SEQ past several buckets, to set beside the same load by key", func() {
		k := prim(fP, 8)
		k.flg = noCmprs
		p.create(seqBig, spec{mrs: 200, bks: 1, keys: []key{k, alt(1, fA, 8, "DUP")}})
		p.call("PUTS", 1, 60, 1, 0, 150, 0)
		p.closeF()
		p.dump(seqBig)
	})
	keyBig := fresh("Q")
	p.kase("the same load, RAC KEY", func() {
		k := prim(fP, 8)
		k.flg = noCmprs
		p.create(keyBig, spec{mrs: 200, bks: 1, keys: []key{k, alt(1, fA, 8, "DUP")}})
		p.call("PUTS", 1, 60, 1, 0, 150, 1)
		p.closeF()
		p.dump(keyBig)
	})
	dseq := fresh("Q")
	p.kase("RAC SEQ, descending", func() {
		k := prim(fP, 8)
		k.dtp = "DSTG"
		p.create(dseq, spec{mrs: 120, keys: []key{k}})
		for _, r := range []int{30, 20, 25, 10} {
			p.call("PUTN", r, 100, 0, 0)
		}
		p.closeF()
	})
	eof := fresh("Q")
	p.kase("ROP EOF", func() {
		p.create(eof, spec{mrs: 120, keys: []key{prim(fP, 8), alt(1, fA, 8, "")}})
		p.puts(10, 5, 10, 0, 100)
		p.closeF()
		p.reopen(eof)
		p.connect(0, ropEOF)
		p.get(0, ropNone)
		p.call("PUTN", 45, 100, 0, 0)
		p.call("PUTN", 60, 100, 0, 0)
		p.connect(1, ropEOF)
		p.connect(0, ropEOF)
		p.getField(0, 0, 20, fP, 8, ropNone)
		p.closeF()
	})

	p.section("changing: $UPDATE and $DELETE")

	change := func(title string, keys []key, f func(name string)) {
		name := fresh("U")
		p.kase(title, func() {
			p.create(name, spec{mrs: 300, keys: keys})
			p.puts(10, 10, 10, 0, 100)
			f(name)
			p.closeF()
			p.mark("then, by every key")
			p.reopen(name)
			for i := range keys {
				p.connect(i, ropNone)
				p.call("WALK", i, 12)
			}
			p.closeF()
			p.dump(name)
		})
	}
	two := []key{prim(fP, 8), alt(1, fA, 8, "DUP,CHG"), typed(2, fBN4, 4, "BN4", "")}
	three := []key{prim(fP, 8), alt(1, fA, 8, "DUP"), alt(2, fB, 8, "CHG")}
	change("$UPDATE: the same, longer, shorter", two, func(string) {
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.updb(100)
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.updb(250)
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.updb(60)
		p.getField(0, 0, 50, fP, 8, ropNone)
	})
	change("$UPDATE with no current record, and after a failure", two, func(string) {
		p.mk(50)
		p.updb(100)
		p.getField(0, 0, 55, fP, 8, ropNone)
		p.mk(50)
		p.updb(100)
	})
	change("$UPDATE: the primary key, each alternate; CHG and DUP", []key{prim(fP, 8), alt(1, fA, 8, "DUP"), alt(2, fB, 8, "CHG"), typed(3, fBN4, 4, "BN4", "DUP,CHG")}, func(string) {
		p.mark("the primary key changed")
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.patch(fP, "X0000000")
		p.updb(100)
		p.mark("key 1, no CHG, changed")
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.patch(fA, "X0000000")
		p.updb(100)
		p.mark("key 2, CHG, no DUP, changed to a new value")
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.patch(fB, "X0000000")
		p.updb(100)
		p.mark("key 2 changed to another's value")
		p.getField(0, 0, 60, fP, 8, ropNone)
		p.mk(60)
		p.patch(fB, "X0000000")
		p.updb(100)
		p.mark("key 3, DUP and CHG, changed to another's value")
		p.getField(0, 0, 70, fP, 8, ropNone)
		p.mk(70)
		p.patchL(fBN4, 50)
		p.updb(100)
	})
	change("$UPDATE: too short for the primary key, and for alternates", []key{prim(fP, 8), alt(1, fA, 8, "DUP,CHG"), alt(2, fB, 8, "DUP")}, func(string) {
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.updb(4)
		p.mark("too short for key 2, which has no CHG")
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.mk(50)
		p.updb(20)
		p.mark("too short for key 1 too; it has CHG")
		p.getField(0, 0, 60, fP, 8, ropNone)
		p.mk(60)
		p.updb(10)
	})
	change("$PUT with UIF", two, func(string) {
		p.mark("UIF over an existing record, with a changed alternate")
		p.mk(50)
		p.patch(fA, "X0000000")
		p.call("PUTB", 150, ropUIF, 1)
		p.mark("UIF where there is none")
		p.mk(55)
		p.call("PUTB", 150, ropUIF, 1)
		p.mark("no UIF over an existing record")
		p.mk(50)
		p.call("PUTB", 150, ropNone, 1)
	})
	change("$DELETE, then finding by each key, and putting it back", three, func(string) {
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.saveRFA(0)
		p.del(ropNone)
		p.mark("again")
		p.del(ropNone)
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.getField(0, 1, 50, fA, 8, ropNone)
		p.getField(0, 2, 50, fB, 8, ropNone)
		p.mark("by the old RFA")
		p.getRFA(0)
		p.mark("back in")
		p.put(50, 100)
		p.getField(0, 2, 50, fB, 8, ropNone)
	})
	change("$DELETE with FDL, then by the alternate", three, func(string) {
		p.getField(0, 0, 50, fP, 8, ropNone)
		p.del(ropFDL)
		p.getField(0, 1, 50, fA, 8, ropNone)
		p.getField(0, 2, 50, fB, 8, ropNone)
	})
	func() {
		a := alt(1, fA, 8, "DUP,CHG,NUL")
		a.hasNul, a.nul = true, 32
		change("a null alternate value: not in that index, then given one", []key{prim(fP, 8), a}, func(string) {
			p.mk(55)
			p.patch(fA, "        ")
			p.putb(100, 1)
			p.mark("by the null value, and by a real one")
			p.getKey(0, 1, "        ", ropNone)
			p.getField(0, 0, 55, fP, 8, ropNone)
			p.mk(55)
			p.updb(100)
			p.getField(0, 1, 55, fA, 8, ropNone)
		})
	}()
	change("$DELETE found through an alternate", three, func(string) {
		p.getField(0, 2, 60, fB, 8, ropNone)
		p.del(ropNone)
		p.get(2, ropNone)
	})
	func() {
		name := fresh("U")
		p.names["split"] = name
		p.kase("RFAs across a split, and a record moved twice", func() {
			k := prim(fP, 8)
			k.flg = noCmprs
			p.create(name, spec{mrs: 400, bks: 1, keys: []key{k, alt(1, fA, 8, "DUP")}})
			for _, r := range []int{10, 20, 30} {
				p.put(r, 150)
				p.saveRFA(r / 10)
			}
			p.mark("forcing splits")
			for _, r := range []int{11, 12, 21, 22, 23, 24, 25} {
				p.put(r, 150)
			}
			p.getRFA(1)
			p.getRFA(2)
			p.getRFA(3)
			p.closeF()
			p.dump(name)
			p.reopen(name)
			p.mark("grown past its bucket by $UPDATE")
			p.getField(0, 0, 20, fP, 8, ropNone)
			p.mk(20)
			p.updb(390)
			p.getRFA(2)
			p.closeF()
			p.dump(name)
			p.reopen(name)
			p.mark("deleted: its RRV")
			p.getRFA(2)
			p.del(ropNone)
			p.getRFA(2)
			p.closeF()
			p.dump(name)
		})
		name = fresh("U")
		p.kase("deleted records' space reused", func() {
			k := prim(fP, 8)
			k.flg = noCmprs
			p.create(name, spec{mrs: 400, bks: 1, keys: []key{k}})
			p.puts(10, 3, 10, 0, 150)
			p.getField(0, 0, 20, fP, 8, ropNone)
			p.del(ropNone)
			p.closeF()
			p.dump(name)
			p.reopen(name)
			p.mark("a new record in its place, the same key and then another")
			p.put(20, 150)
			p.put(21, 150)
			p.closeF()
			p.dump(name)
		})
		name = fresh("U")
		p.kase("every record of a bucket deleted", func() {
			k := prim(fP, 8)
			k.flg = noCmprs
			p.create(name, spec{mrs: 400, bks: 1, keys: []key{k}})
			p.puts(1, 12, 1, 0, 150)
			for r := 1; r <= 12; r++ {
				p.getField(0, 0, r, fP, 8, ropNone)
				p.del(ropNone)
			}
			p.closeF()
			p.dump(name)
			p.reopen(name)
			p.get(0, ropNone)
			p.mark("written again")
			p.puts(1, 4, 1, 0, 150)
			p.closeF()
			p.dump(name)
		})
	}()

	p.section("locks: two streams, last, since a wait could hang")

	lockName := fresh("L")
	p.kase("the file", func() {
		p.create(lockName, spec{mrs: 120, keys: []key{prim(fP, 8), alt(1, fA, 8, "")}})
		p.puts(10, 5, 10, 0, 100)
		p.closeF()
	})
	locks := func(title string, f func()) {
		p.kase(title, func() {
			p.emit("MOVB\t#%d,SHRV", shrAll)
			p.reopen(lockName)
			p.call("OPN2", "&"+p.text(lockName))
			f()
			p.call("CUR1")
			p.emit("CLRB\tSHRV")
		})
	}
	on2 := func(f func()) { p.call("CUR2"); f(); p.call("CUR1") }
	locks("a record held; the other stream's read, by each option", func() {
		p.getField(0, 0, 20, fP, 8, ropNone)
		on2(func() {
			p.getField(0, 0, 20, fP, 8, ropNone)
			p.mark("NLK")
			p.getField(0, 0, 20, fP, 8, ropNLK)
			p.mark("RRL")
			p.getField(0, 0, 20, fP, 8, ropRRL)
			p.mark("NLK + RRL")
			p.getField(0, 0, 20, fP, 8, "#^X100008")
			p.mark("REA")
			p.getField(0, 0, 20, fP, 8, ropREA)
			p.mark("NXR")
			p.getField(0, 0, 20, fP, 8, ropNXR)
			p.mark("TMO 1")
			p.emit("MOVB\t#1,TMOV")
			p.getField(0, 0, 20, fP, 8, ropTMO)
			p.emit("CLRB\tTMOV")
			p.mark("by the alternate, and an RFA")
			p.getField(0, 1, 20, fA, 8, ropNone)
			p.mark("past it, in sequence")
			p.connect(0, ropNone)
			p.get(0, ropNone)
			p.get(0, ropNone)
			p.get(0, ropNone)
		})
		p.call("FRE")
		on2(func() {
			p.mark("after $FREE")
			p.getField(0, 0, 20, fP, 8, ropNone)
			p.call("FRE")
		})
	})
	locks("$RELEASE, ULK, $FREE; $UPDATE of a record another holds", func() {
		p.getField(0, 0, 20, fP, 8, ropULK)
		p.getField(0, 0, 30, fP, 8, ropULK)
		on2(func() {
			p.mark("ULK: both still held?")
			p.getField(0, 0, 20, fP, 8, ropNone)
			p.getField(0, 0, 30, fP, 8, ropNone)
		})
		p.call("RLS")
		on2(func() {
			p.mark("after $RELEASE: the current one")
			p.getField(0, 0, 20, fP, 8, ropNone)
			p.getField(0, 0, 30, fP, 8, ropNone)
			p.call("FRE")
		})
		p.call("FRE")
	})
	locks("$PUT and $DELETE against a lock", func() {
		p.getField(0, 0, 20, fP, 8, ropNone)
		on2(func() {
			p.mark("UIF over the held record")
			p.mk(20)
			p.call("PUTB", 100, ropUIF, 1)
			p.mark("a new record beside it")
			p.put(21, 100)
			p.getField(0, 0, 20, fP, 8, ropNLK)
			p.del(ropNone)
		})
	})

	return p
}

// runtimeData is the program's fixed data.
const runtimeData = `	$SSDEF
	$FABDEF
	$RABDEF
	$XABDEF
	$XABKEYDEF
	$XABSUMDEF
	$XABALLDEF
	.PSECT	D,NOEXE,WRT,LONG
ORGV:	.BYTE	0
RFMV:	.BYTE	0
RATV:	.BYTE	0
BKSV:	.BYTE	0
SHRV:	.BYTE	0
TMOV:	.BYTE	0
MRSV:	.WORD	0
DEQV:	.WORD	0
USZV:	.WORD	32000
	.ALIGN	LONG
ALQV:	.LONG	0
FOPV:	.LONG	0
XABV:	.LONG	0
ROPV:	.LONG	0
CURR:	.ADDRESS RB
RFAT:	.BLKW	3*8
BD:	.WORD	256
	.BYTE	14,1
	.ADDRESS BF
LD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BF
BF:	.BLKB	256
D8A:	.WORD	8
	.BYTE	14,1
	.ADDRESS BIGB
D8B:	.WORD	8
	.BYTE	14,1
	.ADDRESS BIGB+16
D8C:	.WORD	8
	.BYTE	14,1
	.ADDRESS BIGB+24
HX:	.ASCII	/0123456789ABCDEF/
HL:	.BLKB	100
HLD:	.WORD	0
	.BYTE	14,1
	.ADDRESS HL
BLK:	.BLKB	512
KNMB:	.BLKB	32
KEYB:	.BLKB	64
RDB:	.BLKB	33000
BIGB:	.BLKB	33000
MP:	.ASCID	~P!7ZL~
MA:	.ASCID	~A!7ZL~
MB:	.ASCID	~B!7ZL~
MSHOW:	.ASCID	~=!AS r=!XL~
MPUTN:	.ASCID	~=$PUT n=!UL size=!UW r=!XL~
MCOND:	.ASCID	~=c !XL !XL !XL~
MFAB:	.ASCID	~ fab org=!XB rfm=!XB rat=!XB mrs=!UW bks=!UB alq=!UL deq=!UW fop=!XL sts=!XL stv=!XL~
MRAB:	.ASCID	~ rab rfa=!UL,!UW rsz=!UW sts=!XL stv=!XL krf=!UB~
MGOT:	.ASCID	~ got n=!UW rfa=!UL,!UW sts=!XL |!AF|~
MSUM:	.ASCID	~ sum noa=!UB nok=!UB pvn=!UW~
MKEY1:	.ASCID	~ key ref=!UB dtp=!XB flg=!XB nsg=!UB nul=!UB tks=!UB lvl=!UB ian=!UB lan=!UB dan=!UB ibs=!UB dbs=!UB dfl=!UW ifl=!UW mrl=!UW rvb=!UL dvb=!UL plg=!UB~
MKEY2:	.ASCID	~  pos=!UW,!UW,!UW,!UW,!UW,!UW,!UW,!UW siz=!UB,!UB,!UB,!UB,!UB,!UB,!UB,!UB knm=|!AF|~
MALL:	.ASCID	~ area aid=!UB aln=!UB aop=!XB bkz=!UB alq=!UL deq=!UW vol=!UW loc=!UL~
MVB:	.ASCID	~ vbn !UL~
MMK:	.ASCID	~ mark !AS~
MPUTS:	.ASCID	~ puts ok=!UL fail=!UL last=!XL rfa=!UL,!UW~
MWALK:	.ASCID	~ walk key !UL: !UL read, last r=!XL~
MWREC:	.ASCID	~  !AF rfa=!UL,!UW n=!UW~
LCRE:	.ASCID	~$CREATE~
LOPN:	.ASCID	~$OPEN~
LCON:	.ASCID	~$CONNECT~
LDIS:	.ASCID	~$DISCONNECT~
LDSP:	.ASCID	~$DISPLAY~
LCLO:	.ASCID	~$CLOSE~
LPUT:	.ASCID	~$PUT~
LGET:	.ASCID	~$GET~
LFND:	.ASCID	~$FIND~
LUPD:	.ASCID	~$UPDATE~
LDEL:	.ASCID	~$DELETE~
LREW:	.ASCID	~$REWIND~
LFRE:	.ASCID	~$FREE~
LRLS:	.ASCID	~$RELEASE~
LTRU:	.ASCID	~$TRUNCATE~
LDO:	.ASCID	~dump $OPEN~
LDE:	.ASCID	~dump $READ~
	.ALIGN	LONG
FB:	$FAB	FAC=<GET,PUT,UPD,DEL>
	.ALIGN	LONG
RB:	$RAB	FAB=FB
	.ALIGN	LONG
FB2:	$FAB	FAC=<GET,PUT,UPD,DEL>,SHR=<GET,PUT,UPD,DEL>
	.ALIGN	LONG
RB2:	$RAB	FAB=FB2
	.ALIGN	LONG
FB3:	$FAB	FAC=<BIO,GET>,SHR=<GET>
	.ALIGN	LONG
RB3:	$RAB	FAB=FB3,ROP=<BIO>,UBF=BLK,USZ=512
	.ALIGN	LONG
FBT:	.BLKB	FAB$C_BLN
RBT:	.BLKB	RAB$C_BLN
	.ALIGN	LONG
OSUM:	$XABSUM
	.ALIGN	LONG
OKEY:	$XABKEY	KNM=KNMB
	.ALIGN	LONG
OALL:	$XABALL
	.ALIGN	LONG
`

// runtimeCode is the helpers every case uses.
const runtimeCode = `	.PSECT	C,EXE,NOWRT,LONG
; OUT(control string, values...): one line, by $FAOL.
	.ENTRY	OUT,^M<R2>
	MOVW	#256,BD
	MOVL	4(AP),R2
	MOVAB	8(AP),R0
	$FAOL_S	CTRSTR=(R2),OUTLEN=LD,OUTBUF=BD,PRMLST=(R0)
	PUSHAB	LD
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
; HND prints a condition and unwinds to the driver: a fault costs one case.
	.ENTRY	HND,^M<R2>
	MOVL	4(AP),R2
	CMPL	4(R2),#SS$_UNWIND
	BEQL	9$
	PUSHL	12(R2)
	PUSHL	8(R2)
	PUSHL	4(R2)
	PUSHAB	MCOND
	CALLS	#4,OUT
	$UNWIND_S
9$:	MOVL	#SS$_RESIGNAL,R0
	RET
	.ENTRY	MARK,^M<>
	PUSHL	4(AP)
	PUSHAB	MMK
	CALLS	#2,OUT
	RET
; SHOW(label, status): the status stays in R0.
	.ENTRY	SHOW,^M<>
	PUSHL	8(AP)
	PUSHL	4(AP)
	PUSHAB	MSHOW
	CALLS	#3,OUT
	MOVL	8(AP),R0
	RET
	.ENTRY	SHOWF,^M<R2>
	MOVAB	FB,R2
	PUSHL	FAB$L_STV(R2)
	PUSHL	FAB$L_STS(R2)
	PUSHL	FAB$L_FOP(R2)
	MOVZWL	FAB$W_DEQ(R2),-(SP)
	PUSHL	FAB$L_ALQ(R2)
	MOVZBL	FAB$B_BKS(R2),-(SP)
	MOVZWL	FAB$W_MRS(R2),-(SP)
	MOVZBL	FAB$B_RAT(R2),-(SP)
	MOVZBL	FAB$B_RFM(R2),-(SP)
	MOVZBL	FAB$B_ORG(R2),-(SP)
	PUSHAB	MFAB
	CALLS	#11,OUT
	RET
	.ENTRY	SHOWR,^M<R2>
	MOVL	CURR,R2
	MOVZBL	RAB$B_KRF(R2),-(SP)
	PUSHL	RAB$L_STV(R2)
	PUSHL	RAB$L_STS(R2)
	MOVZWL	RAB$W_RSZ(R2),-(SP)
	MOVZWL	RAB$W_RFA+4(R2),-(SP)
	PUSHL	RAB$W_RFA(R2)
	PUSHAB	MRAB
	CALLS	#7,OUT
	RET
; SHOWG: a record read.
	.ENTRY	SHOWG,^M<R2,R3>
	MOVL	CURR,R2
	MOVZWL	RAB$W_RSZ(R2),R3
	CMPL	R3,#60
	BLEQ	10$
	MOVL	#60,R3
10$:	PUSHAB	RDB
	PUSHL	R3
	PUSHL	RAB$L_STS(R2)
	MOVZWL	RAB$W_RFA+4(R2),-(SP)
	PUSHL	RAB$W_RFA(R2)
	MOVZWL	RAB$W_RSZ(R2),-(SP)
	PUSHAB	MGOT
	CALLS	#7,OUT
	RET
; SHOWK: the key XAB OKEY, after $DISPLAY.
	.ENTRY	SHOWK,^M<R2>
	MOVAB	OKEY,R2
	MOVZBL	XAB$B_PROLOG(R2),-(SP)
	PUSHL	XAB$L_DVB(R2)
	PUSHL	XAB$L_RVB(R2)
	MOVZWL	XAB$W_MRL(R2),-(SP)
	MOVZWL	XAB$W_IFL(R2),-(SP)
	MOVZWL	XAB$W_DFL(R2),-(SP)
	MOVZBL	XAB$B_DBS(R2),-(SP)
	MOVZBL	XAB$B_IBS(R2),-(SP)
	MOVZBL	XAB$B_DAN(R2),-(SP)
	MOVZBL	XAB$B_LAN(R2),-(SP)
	MOVZBL	XAB$B_IAN(R2),-(SP)
	MOVZBL	XAB$B_LVL(R2),-(SP)
	MOVZBL	XAB$B_TKS(R2),-(SP)
	MOVZBL	XAB$B_NUL(R2),-(SP)
	MOVZBL	XAB$B_NSG(R2),-(SP)
	MOVZBL	XAB$B_FLG(R2),-(SP)
	MOVZBL	XAB$B_DTP(R2),-(SP)
	MOVZBL	XAB$B_REF(R2),-(SP)
	PUSHAB	MKEY1
	CALLS	#19,OUT
	PUSHAB	KNMB
	PUSHL	#32
	MOVZBL	XAB$B_SIZ0+7(R2),-(SP)
	MOVZBL	XAB$B_SIZ0+6(R2),-(SP)
	MOVZBL	XAB$B_SIZ0+5(R2),-(SP)
	MOVZBL	XAB$B_SIZ0+4(R2),-(SP)
	MOVZBL	XAB$B_SIZ0+3(R2),-(SP)
	MOVZBL	XAB$B_SIZ0+2(R2),-(SP)
	MOVZBL	XAB$B_SIZ0+1(R2),-(SP)
	MOVZBL	XAB$B_SIZ0(R2),-(SP)
	MOVZWL	XAB$W_POS0+14(R2),-(SP)
	MOVZWL	XAB$W_POS0+12(R2),-(SP)
	MOVZWL	XAB$W_POS0+10(R2),-(SP)
	MOVZWL	XAB$W_POS0+8(R2),-(SP)
	MOVZWL	XAB$W_POS0+6(R2),-(SP)
	MOVZWL	XAB$W_POS0+4(R2),-(SP)
	MOVZWL	XAB$W_POS0+2(R2),-(SP)
	MOVZWL	XAB$W_POS0(R2),-(SP)
	PUSHAB	MKEY2
	CALLS	#19,OUT
	RET
	.ENTRY	SHOWA,^M<R2>
	MOVAB	OALL,R2
	PUSHL	XAB$L_LOC(R2)
	MOVZWL	XAB$W_VOL(R2),-(SP)
	MOVZWL	XAB$W_DEQ(R2),-(SP)
	PUSHL	XAB$L_ALQ(R2)
	MOVZBL	XAB$B_BKZ(R2),-(SP)
	MOVZBL	XAB$B_AOP(R2),-(SP)
	MOVZBL	XAB$B_ALN(R2),-(SP)
	MOVZBL	XAB$B_AID(R2),-(SP)
	PUSHAB	MALL
	CALLS	#9,OUT
	RET
; RST: the FAB and RAB as new, with what the case asked to share. RAB fields left
; by a failure can't carry over.
	.ENTRY	RST,^M<R2,R3,R4,R5>
	MOVC3	#FAB$C_BLN,FBT,FB
	MOVC3	#RAB$C_BLN,RBT,RB
	MOVB	SHRV,FB+FAB$B_SHR
	MOVAB	RB,CURR
	RET
	.ENTRY	NAMEF,^M<>
	MOVL	4(AP),R0
	MOVB	(R0),FB+FAB$B_FNS
	MOVL	4(R0),FB+FAB$L_FNA
	RET
; CRE(name): $CREATE by the values the case set, then $CONNECT.
	.ENTRY	CRE,^M<R2>
	CALLS	#0,RST
	PUSHL	4(AP)
	CALLS	#1,NAMEF
	MOVB	ORGV,FB+FAB$B_ORG
	MOVB	RFMV,FB+FAB$B_RFM
	MOVB	RATV,FB+FAB$B_RAT
	MOVW	MRSV,R0
	BGEQ	5$
	CLRL	R0
5$:	MOVW	R0,FB+FAB$W_MRS
	MOVB	BKSV,FB+FAB$B_BKS
	MOVL	ALQV,FB+FAB$L_ALQ
	MOVW	DEQV,FB+FAB$W_DEQ
	MOVL	FOPV,FB+FAB$L_FOP
	MOVL	XABV,FB+FAB$L_XAB
	PUSHAB	FB
	CALLS	#1,G^SYS$CREATE
	PUSHL	R0
	PUSHAB	LCRE
	CALLS	#2,SHOW
	MOVL	R0,R2
	CALLS	#0,SHOWF
	CLRL	FB+FAB$L_XAB
	BLBC	R2,9$
	PUSHAB	RB
	CALLS	#1,G^SYS$CONNECT
	PUSHL	R0
	PUSHAB	LCON
	CALLS	#2,SHOW
9$:	RET
; OPN(name, quiet): $OPEN, what $DISPLAY says of each key and area, and $CONNECT.
	.ENTRY	OPN,^M<R2,R3>
	CALLS	#0,RST
	PUSHL	4(AP)
	CALLS	#1,NAMEF
	MOVAB	OSUM,FB+FAB$L_XAB
	PUSHAB	FB
	CALLS	#1,G^SYS$OPEN
	PUSHL	R0
	PUSHAB	LOPN
	CALLS	#2,SHOW
	BLBS	R0,5$
	CALLS	#0,SHOWF
	RET
5$:	TSTL	8(AP)
	BEQL	6$
	BRW	50$
6$:	CALLS	#0,SHOWF
	MOVZWL	OSUM+XAB$W_PVN,-(SP)
	MOVZBL	OSUM+XAB$B_NOK,-(SP)
	MOVZBL	OSUM+XAB$B_NOA,-(SP)
	PUSHAB	MSUM
	CALLS	#4,OUT
	CLRL	R2
10$:	CMPB	R2,OSUM+XAB$B_NOK
	BGEQU	20$
	MOVB	R2,OKEY+XAB$B_REF
	MOVAB	OKEY,FB+FAB$L_XAB
	PUSHAB	FB
	CALLS	#1,G^SYS$DISPLAY
	BLBS	R0,15$
	PUSHL	R0
	PUSHAB	LDSP
	CALLS	#2,SHOW
15$:	CALLS	#0,SHOWK
	INCL	R2
	BRB	10$
20$:	CLRL	R2
30$:	CMPB	R2,OSUM+XAB$B_NOA
	BGEQU	50$
	MOVB	R2,OALL+XAB$B_AID
	MOVAB	OALL,FB+FAB$L_XAB
	PUSHAB	FB
	CALLS	#1,G^SYS$DISPLAY
	BLBS	R0,35$
	PUSHL	R0
	PUSHAB	LDSP
	CALLS	#2,SHOW
35$:	CALLS	#0,SHOWA
	INCL	R2
	BRB	30$
50$:	CLRL	FB+FAB$L_XAB
	PUSHAB	RB
	CALLS	#1,G^SYS$CONNECT
	PUSHL	R0
	PUSHAB	LCON
	CALLS	#2,SHOW
	RET
; OPN2(name): the second stream, on a FAB of its own.
	.ENTRY	OPN2,^M<>
	MOVL	4(AP),R0
	MOVB	(R0),FB2+FAB$B_FNS
	MOVL	4(R0),FB2+FAB$L_FNA
	PUSHAB	FB2
	CALLS	#1,G^SYS$OPEN
	PUSHL	R0
	PUSHAB	LOPN
	CALLS	#2,SHOW
	PUSHAB	RB2
	CALLS	#1,G^SYS$CONNECT
	PUSHL	R0
	PUSHAB	LCON
	CALLS	#2,SHOW
	RET
	.ENTRY	CUR1,^M<>
	MOVAB	RB,CURR
	RET
	.ENTRY	CUR2,^M<>
	MOVAB	RB2,CURR
	RET
	.ENTRY	CLO,^M<>
	CLRL	FB+FAB$L_XAB
	PUSHAB	FB
	CALLS	#1,G^SYS$CLOSE
	PUSHL	R0
	PUSHAB	LCLO
	CALLS	#2,SHOW
	RET
; CLEAN closes whatever a case left open, and puts the shared values back.
	.ENTRY	CLEAN,^M<>
	CLRL	FB+FAB$L_XAB
	TSTW	FB+FAB$W_IFI
	BEQL	10$
	PUSHAB	FB
	CALLS	#1,G^SYS$CLOSE
10$:	TSTW	FB2+FAB$W_IFI
	BEQL	20$
	PUSHAB	FB2
	CALLS	#1,G^SYS$CLOSE
20$:	TSTW	FB3+FAB$W_IFI
	BEQL	30$
	PUSHAB	FB3
	CALLS	#1,G^SYS$CLOSE
30$:	CLRB	SHRV
	CLRL	FOPV
	CLRL	ROPV
	CLRB	TMOV
	MOVW	#32000,USZV
	MOVAB	RB,CURR
	RET
; HEXL(offset): one line of BLK in hex, unless it's all zeros.
	.ENTRY	HEXL,^M<R2,R3,R4,R5>
	MOVL	4(AP),R2
	MOVAB	BLK[R2],R3
	SKPC	#0,#32,(R3)
	BEQL	90$
	MOVAB	HL,R4
	MOVB	#32,(R4)+
	MOVB	#32,(R4)+
	MOVB	#32,(R4)+
	EXTZV	#8,#4,R2,R0
	MOVB	HX[R0],(R4)+
	EXTZV	#4,#4,R2,R0
	MOVB	HX[R0],(R4)+
	EXTZV	#0,#4,R2,R0
	MOVB	HX[R0],(R4)+
	MOVB	#32,(R4)+
	MOVL	#32,R5
10$:	MOVZBL	(R3)+,R0
	EXTZV	#4,#4,R0,R1
	MOVB	HX[R1],(R4)+
	EXTZV	#0,#4,R0,R1
	MOVB	HX[R1],(R4)+
	SOBGTR	R5,10$
	MOVAB	HL,R1
	SUBL3	R1,R4,R0
	MOVW	R0,HLD
	PUSHAB	HLD
	CALLS	#1,G^LIB$PUT_OUTPUT
90$:	RET
; DUMPF(name): every block of a closed file.
	.ENTRY	DUMPF,^M<R2,R3,R4>
	MOVL	4(AP),R0
	MOVB	(R0),FB3+FAB$B_FNS
	MOVL	4(R0),FB3+FAB$L_FNA
	PUSHAB	FB3
	CALLS	#1,G^SYS$OPEN
	BLBS	R0,10$
	PUSHL	R0
	PUSHAB	LDO
	CALLS	#2,SHOW
	RET
10$:	PUSHAB	RB3
	CALLS	#1,G^SYS$CONNECT
	MOVL	#1,R3
20$:	MOVL	R3,RB3+RAB$L_BKT
	PUSHAB	RB3
	CALLS	#1,G^SYS$READ
	BLBC	R0,40$
	PUSHL	R3
	PUSHAB	MVB
	CALLS	#2,OUT
	CLRL	R4
30$:	PUSHL	R4
	CALLS	#1,HEXL
	ACBL	#511,#32,R4,30$
	INCL	R3
	BRB	20$
40$:	PUSHL	R0
	PUSHAB	LDE
	CALLS	#2,SHOW
	PUSHAB	FB3
	CALLS	#1,G^SYS$CLOSE
	RET
; MKREC(n): the common record in BIGB, whose tail main's set.
	.ENTRY	MKREC,^M<R2,R3,R4,R5,R6>
	MOVL	4(AP),R2
	$FAO_S	CTRSTR=MP,OUTBUF=D8A,P1=R2
	MOVL	R2,BIGB+8
	MNEGL	R2,BIGB+12
	DIVL3	#3,R2,R3
	$FAO_S	CTRSTR=MA,OUTBUF=D8B,P1=R3
	SUBL3	R2,#9999999,R3
	$FAO_S	CTRSTR=MB,OUTBUF=D8C,P1=R3
	CVTLP	R2,#7,BIGB+32
	MOVL	4(AP),R2		; CVTLP used R0 to R3
	MOVL	R2,BIGB+36
	CLRL	BIGB+40
	MNEGL	R2,BIGB+44
	MOVL	#-1,BIGB+48
	TSTL	R2
	BNEQ	9$
	CLRL	BIGB+48
9$:	RET
; MKKEY(n): the same record's fields in KEYB, for a key.
	.ENTRY	MKKEY,^M<R2,R3,R4,R5>
	MOVC3	#52,BIGB,RDB
	PUSHL	4(AP)
	CALLS	#1,MKREC
	MOVC3	#52,BIGB,KEYB
	MOVC3	#52,RDB,BIGB
	RET
; PUTB(size, rop, rac): the record in BIGB.
	.ENTRY	PUTB,^M<R2>
	MOVL	CURR,R2
	MOVW	4(AP),RAB$W_RSZ(R2)
	MOVAB	BIGB,RAB$L_RBF(R2)
	MOVL	8(AP),RAB$L_ROP(R2)
	MOVB	12(AP),RAB$B_RAC(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$PUT
	PUSHL	R0
	PUSHAB	LPUT
	CALLS	#2,SHOW
	CALLS	#0,SHOWR
	RET
; PUTN(n, size, rop, rac).
	.ENTRY	PUTN,^M<R2>
	PUSHL	4(AP)
	CALLS	#1,MKREC
	MOVL	CURR,R2
	MOVW	8(AP),RAB$W_RSZ(R2)
	MOVAB	BIGB,RAB$L_RBF(R2)
	MOVL	12(AP),RAB$L_ROP(R2)
	MOVB	16(AP),RAB$B_RAC(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$PUT
	PUSHL	R0
	MOVZWL	8(AP),-(SP)
	PUSHL	4(AP)
	PUSHAB	MPUTN
	CALLS	#4,OUT
	CALLS	#0,SHOWR
	RET
; PUTS(first, count, step, mod, size, rac): many records; only a failure shows.
	.ENTRY	PUTS,^M<R2,R3,R4,R5,R6,R7,R8>
	MOVL	CURR,R2
	CLRL	R3
	CLRL	R5
	CLRL	R6
	MOVL	#1,R7
10$:	CMPL	R3,8(AP)
	BGEQ	50$
	MULL3	R3,12(AP),R4
	ADDL2	4(AP),R4
	TSTL	16(AP)
	BEQL	20$
	EMUL	#0,#0,R4,R0
	EDIV	16(AP),R0,R0,R4
	INCL	R4
20$:	PUSHL	R4
	CALLS	#1,MKREC
	MOVW	20(AP),RAB$W_RSZ(R2)
	MOVAB	BIGB,RAB$L_RBF(R2)
	MOVL	ROPV,RAB$L_ROP(R2)
	MOVB	24(AP),RAB$B_RAC(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$PUT
	MOVL	R0,R7
	BLBS	R0,30$
	INCL	R6
	PUSHL	R0
	MOVZWL	20(AP),-(SP)
	PUSHL	R4
	PUSHAB	MPUTN
	CALLS	#4,OUT
	BRB	40$
30$:	INCL	R5
40$:	INCL	R3
	BRB	10$
50$:	MOVZWL	RAB$W_RFA+4(R2),-(SP)
	PUSHL	RAB$W_RFA(R2)
	PUSHL	R7
	PUSHL	R6
	PUSHL	R5
	PUSHAB	MPUTS
	CALLS	#6,OUT
	RET
; UPDB(size, rop): the record in BIGB over the current one.
	.ENTRY	UPDB,^M<R2>
	MOVL	CURR,R2
	MOVW	4(AP),RAB$W_RSZ(R2)
	MOVAB	BIGB,RAB$L_RBF(R2)
	MOVL	8(AP),RAB$L_ROP(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$UPDATE
	PUSHL	R0
	PUSHAB	LUPD
	CALLS	#2,SHOW
	CALLS	#0,SHOWR
	RET
	.ENTRY	DEL,^M<R2>
	MOVL	CURR,R2
	MOVL	4(AP),RAB$L_ROP(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$DELETE
	PUSHL	R0
	PUSHAB	LDEL
	CALLS	#2,SHOW
	CALLS	#0,SHOWR
	RET
; GETK(svc, krf, key address, ksz, rop, rac): 0 $GET, 1 $FIND; RAC 0 SEQ, 1 KEY, 2 RFA.
	.ENTRY	GETK,^M<R2>
	MOVL	CURR,R2
	MOVB	8(AP),RAB$B_KRF(R2)
	MOVL	12(AP),RAB$L_KBF(R2)
	MOVB	16(AP),RAB$B_KSZ(R2)
	MOVL	20(AP),RAB$L_ROP(R2)
	MOVB	24(AP),RAB$B_RAC(R2)
	MOVB	TMOV,RAB$B_TMO(R2)
	TSTL	4(AP)
	BNEQ	20$
	MOVW	USZV,RAB$W_USZ(R2)
	MOVAB	RDB,RAB$L_UBF(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$GET
	PUSHL	R0
	PUSHAB	LGET
	CALLS	#2,SHOW
	BLBC	R0,10$
	CALLS	#0,SHOWG
	RET
10$:	CALLS	#0,SHOWR
	RET
20$:	PUSHL	R2
	CALLS	#1,G^SYS$FIND
	PUSHL	R0
	PUSHAB	LFND
	CALLS	#2,SHOW
	CALLS	#0,SHOWR
	RET
; WALK(krf, most): every record, in the stream's order, with its RFA.
	.ENTRY	WALK,^M<R2,R3,R4>
	MOVL	CURR,R2
	MOVB	4(AP),RAB$B_KRF(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$REWIND
	CLRL	R3
	MOVL	#1,R4
10$:	CMPL	R3,8(AP)
	BGEQ	50$
	CLRL	RAB$L_ROP(R2)
	MOVB	#RAB$C_SEQ,RAB$B_RAC(R2)
	MOVW	USZV,RAB$W_USZ(R2)
	MOVAB	RDB,RAB$L_UBF(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$GET
	MOVL	R0,R4
	BLBC	R0,50$
	INCL	R3
	MOVZWL	RAB$W_RSZ(R2),-(SP)
	MOVZWL	RAB$W_RFA+4(R2),-(SP)
	PUSHL	RAB$W_RFA(R2)
	PUSHAB	RDB
	PUSHL	#8
	PUSHAB	MWREC
	CALLS	#6,OUT
	BRB	10$
50$:	PUSHL	R4
	PUSHL	R3
	MOVZBL	4(AP),-(SP)
	PUSHAB	MWALK
	CALLS	#4,OUT
	RET
; SAVER(slot) keeps the stream's RFA, SETR(slot) puts it back for an RFA access.
	.ENTRY	SAVER,^M<R2>
	MOVL	CURR,R2
	MULL3	#6,4(AP),R0
	MOVAB	RFAT,R1
	ADDL2	R0,R1
	MOVL	RAB$W_RFA(R2),(R1)
	MOVW	RAB$W_RFA+4(R2),4(R1)
	RET
	.ENTRY	SETR,^M<R2>
	MOVL	CURR,R2
	MULL3	#6,4(AP),R0
	MOVAB	RFAT,R1
	ADDL2	R0,R1
	MOVL	(R1),RAB$W_RFA(R2)
	MOVW	4(R1),RAB$W_RFA+4(R2)
	RET
	.ENTRY	REW,^M<R2>
	MOVL	CURR,R2
	MOVB	4(AP),RAB$B_KRF(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$REWIND
	PUSHL	R0
	PUSHAB	LREW
	CALLS	#2,SHOW
	CALLS	#0,SHOWR
	RET
; CON(krf, rop): the stream disconnected and connected again.
	.ENTRY	CON,^M<R2>
	MOVL	CURR,R2
	PUSHL	R2
	CALLS	#1,G^SYS$DISCONNECT
	PUSHL	R0
	PUSHAB	LDIS
	CALLS	#2,SHOW
	MOVB	4(AP),RAB$B_KRF(R2)
	MOVL	8(AP),RAB$L_ROP(R2)
	PUSHL	R2
	CALLS	#1,G^SYS$CONNECT
	PUSHL	R0
	PUSHAB	LCON
	CALLS	#2,SHOW
	RET
	.ENTRY	FRE,^M<R2>
	MOVL	CURR,R2
	PUSHL	R2
	CALLS	#1,G^SYS$FREE
	PUSHL	R0
	PUSHAB	LFRE
	CALLS	#2,SHOW
	RET
	.ENTRY	RLS,^M<R2>
	MOVL	CURR,R2
	PUSHL	R2
	CALLS	#1,G^SYS$RELEASE
	PUSHL	R0
	PUSHAB	LRLS
	CALLS	#2,SHOW
	RET
	.ENTRY	TRU,^M<R2>
	MOVL	CURR,R2
	PUSHL	R2
	CALLS	#1,G^SYS$TRUNCATE
	PUSHL	R0
	PUSHAB	LTRU
	CALLS	#2,SHOW
	RET
; SEQW(n, size): a $PUT in sequence at the stream's place.
	.ENTRY	SEQW,^M<>
	CLRL	-(SP)
	CLRL	-(SP)
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#4,PUTN
	RET
`

// write assembles the program.
func (p *prog) write() string {
	var b strings.Builder

	b.WriteString("\t.TITLE\tIDX88 - Phase 88's probe of RMS's indexed files\n; Written by testdata/idx88/gen.go.\n")
	b.WriteString(runtimeData)
	b.WriteString(p.data.String())
	b.WriteString(runtimeCode)
	b.WriteString(p.code.String())
	b.WriteString("\t.ENTRY\tMAIN,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n")
	b.WriteString("\tMOVC3\t#FAB$C_BLN,FB,FBT\n\tMOVC3\t#RAB$C_BLN,RB,RBT\n")
	// The record's tail: letters, with a run of blanks at 60 to 99.
	b.WriteString("\tMOVL\t#52,R3\n1$:\tEMUL\t#0,#0,R3,R0\n\tEDIV\t#26,R0,R0,R2\n\tADDB3\t#97,R2,BIGB[R3]\n\tAOBLSS\t#33000,R3,1$\n")
	b.WriteString("\tMOVC5\t#0,BIGB,#32,#40,BIGB+60\n")
	b.WriteString(p.drive.String())
	b.WriteString("\tPUSHAB\tENDM\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n\tMOVL\t#1,R0\n\tRET\n")
	b.WriteString("\t.PSECT\tD\nENDM:\t.ASCID\t~##### the end of the cases~\n")
	b.WriteString("\t.END\tMAIN\n")

	return b.String()
}

// defs are the internal definition macros whose values round 1 captures (the
// author's answer 5): one object each, values only, no listing.
var defs = []string{"BKT", "IRC", "KEY", "AREA", "PLG"}

func defModule(name string) string {
	return fmt.Sprintf("\t.TITLE\tDEF_%s\t$%sDEF's values, in the object's global symbols\n"+
		"; Written by testdata/idx88/gen.go.\n"+
		"\t.LIBRARY\t/SYS$LIBRARY:LIB.MLB/\n"+
		"\t$%sDEF\tGLOBAL\n\t.END\n", name, name, name)
}

// The scripted walks of ANALYZE/RMS_FILE/INTERACTIVE: each DOWN goes a level into
// the file, NEXT along, and UP back, and each stop prints VMS's names for a
// structure's fields.
const walk = `DOWN
DOWN
DOWN AREAS
NEXT
UP
DOWN KEYS
DOWN INDEX
DOWN
NEXT
UP
UP
DOWN DATA
DOWN
DOWN BYTES
UP
DOWN RRV
UP
NEXT
DOWN
UP
NEXT
UP
UP
NEXT
DOWN INDEX
DOWN
UP
UP
DOWN DATA
DOWN
DOWN
NEXT
UP
UP
UP
UP
UP
EXIT
`

const com = `$ ! IDX88.COM - the Phase 88 probe of RMS's indexed files.
$ ! Written by testdata/idx88/gen.go. Run it with the exchange volume's [000000]
$ ! as the default directory:
$ !
$ !     @IDX88/OUTPUT=IDX88.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ !
$ ! The internal definitions' values. MACRO's own output goes to a log of its own and
$ ! nowhere else, SYS$ERROR being NL:, so that IDX88.LOG can't hold a macro's text.
$ DEFINE/USER_MODE SYS$OUTPUT DEFS.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST DEF_BKT
$ DEFINE/USER_MODE SYS$OUTPUT DEFS2.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST DEF_IRC
$ DEFINE/USER_MODE SYS$OUTPUT DEFS3.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST DEF_KEY
$ DEFINE/USER_MODE SYS$OUTPUT DEFS4.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST DEF_AREA
$ DEFINE/USER_MODE SYS$OUTPUT DEFS5.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST DEF_PLG
$ DIRECTORY/SIZE DEF_*.OBJ
$ !
$ ! The program.
$ DEFINE/USER_MODE SYS$OUTPUT IDX88M.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST IDX88
$ LINK/NOMAP IDX88
$ DEFINE/USER_MODE SYS$OUTPUT IDX88-VAX.TXT
$ RUN IDX88
$ SHOW SYMBOL $STATUS
$ !
$ ! Every file: VMS's own account of it.
$ DIRECTORY/FULL/OUTPUT=IDX88DIR.TXT *.IDX
$ LOOP:
$ F = F$SEARCH("*.IDX")
$ IF F .EQS. "" THEN GOTO DONE
$ N = F$PARSE(F,,,"NAME")
$ ANALYZE/RMS_FILE/OUTPUT='N'.ANL 'F'
$ ANALYZE/RMS_FILE/CHECK/OUTPUT='N'.CHK 'F'
$ ANALYZE/RMS_FILE/FDL/OUTPUT='N'.FDL 'F'
$ GOTO LOOP
$ DONE:
$ !
$ ! A walk with VMS's names, on three kinds of file. The commands are IDX88W.DAT.
$ DEFINE/USER_MODE SYS$INPUT IDX88W.DAT
$ DEFINE/USER_MODE SYS$OUTPUT IDX88W1.TXT
$ ANALYZE/RMS_FILE/INTERACTIVE @WALK1@
$ DEFINE/USER_MODE SYS$INPUT IDX88W.DAT
$ DEFINE/USER_MODE SYS$OUTPUT IDX88W2.TXT
$ ANALYZE/RMS_FILE/INTERACTIVE @WALK2@
$ DEFINE/USER_MODE SYS$INPUT IDX88W.DAT
$ DEFINE/USER_MODE SYS$OUTPUT IDX88W3.TXT
$ ANALYZE/RMS_FILE/INTERACTIVE @WALK3@
$ !
$ ! The files themselves, of our own data, in a save set.
$ BACKUP *.IDX;* IDX88.BCK/SAVE_SET
$ BACKUP/LIST=IDX88BCK.TXT IDX88.BCK/SAVE_SET
$ EXIT
`

const exchange = `! EXCHANGE.CMD - builds the Phase 88 probe's exchange volume with govax.
! Written by testdata/idx88/gen.go. Run from the repository root:
!
!     govax console < testdata/idx88/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/idx88-exchange.dsk" /DEVICE=RD54 IDX88
MOUNT/WRITE DUA5 "testdata/disks/idx88-exchange.dsk"
COPY "testdata/idx88/idx88.mar"/HOST DUA5:[000000]IDX88.MAR
COPY "testdata/idx88/idx88.com"/HOST DUA5:[000000]IDX88.COM
COPY "testdata/idx88/idx88w.dat"/HOST DUA5:[000000]IDX88W.DAT
COPY "testdata/idx88/def_bkt.mar"/HOST DUA5:[000000]DEF_BKT.MAR
COPY "testdata/idx88/def_irc.mar"/HOST DUA5:[000000]DEF_IRC.MAR
COPY "testdata/idx88/def_key.mar"/HOST DUA5:[000000]DEF_KEY.MAR
COPY "testdata/idx88/def_area.mar"/HOST DUA5:[000000]DEF_AREA.MAR
COPY "testdata/idx88/def_plg.mar"/HOST DUA5:[000000]DEF_PLG.MAR
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
`

const copyout = `! COPYOUT.CMD - copies the VAX run's results off the exchange volume.
! Written by testdata/idx88/gen.go. Run from the repository root:
!
!     govax console < testdata/idx88/copyout.cmd
!
! DEFS*.LOG and IDX88M.LOG, MACRO's own output, stay on the volume until the
! author has read them (the clean-room notes).
MOUNT DUA5 "testdata/disks/idx88-exchange.dsk"
COPY DUA5:[000000]IDX88.LOG "testdata/idx88/vax/idx88.log"/HOST/QUIET
COPY DUA5:[000000]IDX88-VAX.TXT "testdata/idx88/vax/idx88-vax.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88DIR.TXT "testdata/idx88/vax/idx88dir.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88W1.TXT "testdata/idx88/vax/idx88w1.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88W2.TXT "testdata/idx88/vax/idx88w2.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88W3.TXT "testdata/idx88/vax/idx88w3.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88BCK.TXT "testdata/idx88/vax/idx88bck.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88.BCK "testdata/idx88/vax/idx88.bck"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_BKT.OBJ "testdata/idx88/vax/def_bkt.obj"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_IRC.OBJ "testdata/idx88/vax/def_irc.obj"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_KEY.OBJ "testdata/idx88/vax/def_key.obj"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_AREA.OBJ "testdata/idx88/vax/def_area.obj"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_PLG.OBJ "testdata/idx88/vax/def_plg.obj"/HOST/BINARY/QUIET
@REPORTS@DISMOUNT DUA5
`

func main() {
	p := cases()

	// The walks: two keys; a split file with deleted records and RRVs; three areas.
	script := com
	for mark, title := range map[string]string{
		"@WALK1@": "two keys, the alternate by the macro's defaults",
		"@WALK2@": "split",
		"@WALK3@": "3 areas: data, index, alternate",
	} {
		name, ok := p.names[title]
		if !ok {
			log.Fatalf("no file for walk %q", title)
		}

		script = strings.ReplaceAll(script, mark, name)
	}

	var reports strings.Builder

	for _, f := range p.files {
		for _, ext := range []string{"anl", "chk", "fdl"} {
			fmt.Fprintf(&reports, "COPY DUA5:[000000]%s.%s \"testdata/idx88/vax/anl/%s.%s\"/HOST/QUIET\n",
				f, strings.ToUpper(ext), strings.ToLower(f), ext)
		}
	}

	files := map[string]string{
		"idx88.mar":    p.write(),
		"idx88.com":    script,
		"idx88w.dat":   walk,
		"exchange.cmd": exchange,
		"copyout.cmd":  strings.ReplaceAll(copyout, "@REPORTS@", reports.String()),
	}

	for _, d := range defs {
		files["def_"+strings.ToLower(d)+".mar"] = defModule(d)
	}

	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("idx88.mar: %d cases, %d files\n", p.n, len(p.files))
}
