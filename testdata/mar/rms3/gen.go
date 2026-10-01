//go:build ignore

// gen writes the Phase 33 runtime oracle (docs/PHASE-33.md, subtask 1):
// VAX MACRO probe programs that call RMS's name and attribute services
// and write what each call returned to a file, the command procedures that
// build a test tree and run the probes on VMS, and the govax console
// script that builds the exchange volume. Run it from the repository root:
//
//	go run testdata/mar/rms3/gen.go
//
// The probes are written from the RMS Reference Manual; they use govax's
// own macros (or, on VMS, real MACRO's, which Phase 32 showed assemble to
// the same objects).
//
// Every probe writes [OUT]name.DMP, a file of variable-length records. Each
// record is a 12-byte header, then data:
//
//	tag   4 bytes of ASCII naming the data: STAT (R0 after the call), FAB_,
//	      NAM_, ESA_, RSA_ (the string buffers), or an XAB's (XDAT, ...)
//	step  longword: the case number; a $SEARCH loop adds 100 times the
//	      case number to its pass count instead
//	op    longword: the service just called (the op constants below)
//
// The string buffers are filled with ^XEE before each case, so what RMS
// didn't write shows.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const dir = "testdata/mar/rms3"

// The op longword of each record.
const (
	opParse   = 1
	opSearch  = 2
	opOpen    = 3
	opCreate  = 4
	opDisplay = 5
	opClose   = 6
)

// prog is one probe program being written.
type prog struct {
	name    string
	comment string
	defs    []string // extra $xxxDEF macros
	data    []string // extra data lines (block templates and the like)
	code    strings.Builder
	strs    strings.Builder
	nstr    int
	nlabel  int
}

func newProg(name, comment string) *prog {
	return &prog{name: name, comment: comment}
}

// emit adds code lines.
func (p *prog) emit(lines ...string) {
	for _, l := range lines {
		p.code.WriteString(l)
		p.code.WriteByte('\n')
	}
}

// str defines a string and returns its label; the label with _L appended
// is its length.
func (p *prog) str(s string) string {
	p.nstr++
	label := fmt.Sprintf("S%d", p.nstr)
	delim := "|"
	if strings.Contains(s, delim) {
		log.Fatalf("string %q holds the delimiter", s)
	}
	fmt.Fprintf(&p.strs, "%s:\t.ASCII\t%s%s%s\n%s_L = .-%s\n", label, delim, s, delim, label, label)

	return label
}

// label returns a new branch label.
func (p *prog) label() string {
	p.nlabel++

	return fmt.Sprintf("L%d", p.nlabel)
}

// reset puts the working FAB and NAM back to their templates, fills the
// string buffers, and names the file: fn is the file name, dn the
// default name.
func (p *prog) reset(step int, fn, dn string) {
	f, d := p.str(fn), p.str(dn)
	p.emit(
		fmt.Sprintf("\tMOVL\t#%d,STEP", step),
		"\tMOVC3\t#FAB$C_BLN,FABT,FAB1",
		"\tMOVC3\t#NAM$C_BLN,NAMT,NAM1",
		"\tMOVC5\t#0,(SP),#^XEE,#256,ESA",
		"\tMOVC5\t#0,(SP),#^XEE,#256,RSA",
		"\tMOVAB\t"+f+",FAB1+FAB$L_FNA",
		"\tMOVB\t#"+f+"_L,FAB1+FAB$B_FNS",
		"\tMOVAB\t"+d+",FAB1+FAB$L_DNA",
		"\tMOVB\t#"+d+"_L,FAB1+FAB$B_DNS",
	)
}

// call calls a FAB service on FAB1 and records R0 and the op.
func (p *prog) call(svc string, op int) {
	p.emit(
		"\t$"+svc+"\tFAB=FAB1",
		"\tMOVL\tR0,STAT",
		fmt.Sprintf("\tMOVL\t#%d,OP", op),
	)
}

// dumpAll writes STAT and the FAB, NAM, and string buffers.
func (p *prog) dumpAll() {
	p.emit("\tJSB\tDUMPALL")
}

// skipIfFailed branches to l if STAT is a failure, however far l is.
func (p *prog) skipIfFailed(l string) {
	ok := p.label()
	p.emit("\tBLBS\tSTAT,"+ok, "\tBRW\t"+l, ok+":")
}

// closeIfOpen closes FAB1, without its XABs, if the last call succeeded,
// and records the close's R0.
func (p *prog) closeIfOpen() {
	l := p.label()
	p.skipIfFailed(l)
	p.emit(
		"\tCLRL\tFAB1+FAB$L_XAB",
		"\t$CLOSE\tFAB=FAB1",
		"\tMOVL\tR0,STAT",
		fmt.Sprintf("\tMOVL\t#%d,OP", opClose),
		"\tDUMP\tSTAT,STAT,4",
		l+":",
	)
}

// write produces the program's source.
func (p *prog) write() {
	var b strings.Builder
	fmt.Fprintf(&b, "\t.TITLE\t%s\tPhase 33 runtime oracle\n\t.IDENT\t/V1.0/\n\n", p.name)
	b.WriteString(p.comment)
	b.WriteString("\n\t$FABDEF\n\t$NAMDEF\n\t$RABDEF\n")
	for _, d := range p.defs {
		b.WriteString("\t" + d + "\n")
	}
	b.WriteString(`
; DUMP writes one record: TAG, the step, the op, and LEN bytes at ADDR.
	.MACRO	DUMP	TAG,ADDR,LEN
	MOVL	#^A/TAG/,REC
	MOVL	STEP,REC+4
	MOVL	OP,REC+8
	MOVC3	#LEN,ADDR,REC+12
	MOVZWL	#LEN+12,R6
	JSB	PUTREC
	.ENDM	DUMP

	.PSECT	DATA,NOEXE,WRT,LONG
`)
	fmt.Fprintf(&b, "OFAB:\t$FAB\tFNM=<[OUT]%s.DMP>,FAC=PUT,RFM=VAR,MRS=1024\n", p.name)
	b.WriteString("ORAB:\t$RAB\tFAB=OFAB,RBF=REC\n")
	for _, d := range p.data {
		b.WriteString(d + "\n")
	}
	b.WriteString(`	.ALIGN	LONG
FAB1:	.BLKB	FAB$C_BLN
NAMT:	$NAM	ESA=ESA,ESS=255,RSA=RSA,RSS=255
NAM1:	.BLKB	NAM$C_BLN
STEP:	.LONG	0
OP:	.LONG	0
STAT:	.LONG	0
ESA:	.BLKB	256
RSA:	.BLKB	256
REC:	.BLKB	1024
`)
	b.WriteString(p.strs.String())
	fmt.Fprintf(&b, `
	.PSECT	CODE,EXE,NOWRT,LONG
	.ENTRY	%s,^M<R2,R3,R4,R5,R6,R7>
	$CREATE	FAB=OFAB
	BLBS	R0,1$
	RET
1$:	$CONNECT RAB=ORAB
	BLBS	R0,2$
	RET
2$:
`, p.name)
	b.WriteString(p.code.String())
	b.WriteString(`	$CLOSE	FAB=OFAB
	RET

; PUTREC writes REC's first R6 bytes to the output file.
PUTREC:	MOVW	R6,ORAB+RAB$W_RSZ
	$PUT	RAB=ORAB
	RSB

; DUMPALL writes STAT, FAB1, NAM1, and the two string buffers.
DUMPALL:
	DUMP	STAT,STAT,4
	DUMP	FAB_,FAB1,FAB$C_BLN
	DUMP	NAM_,NAM1,NAM$C_BLN
	DUMP	ESA_,ESA,256
	DUMP	RSA_,RSA,256
	RSB

`)
	fmt.Fprintf(&b, "\t.END\t%s\n", p.name)

	path := filepath.Join(dir, strings.ToLower(p.name)+".mar")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}

// vmsTime is t as a VMS quadword (100 ns units since 17-Nov-1858), split
// into longwords.
func vmsTime(t time.Time) (lo, hi uint32) {
	base := time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)
	q := uint64(t.Sub(base)/time.Second) * 10_000_000

	return uint32(q), uint32(q >> 32)
}

// pcase is a $PARSE or $OPEN case: the names, and MACRO lines run after
// the reset and before the call.
type pcase struct {
	fn, dn string
	pre    []string
}

func parseProg() {
	p := newProg("PARSE", `; $PARSE: each case's result, without a search. Cases 13 and 14 give a
; related file (NAM$L_RLF) whose resultant string is [TEST]B.TXT;1, without
; and with FAB$V_OFP.
`)
	p.data = append(p.data,
		"FABT:\t$FAB\tNAM=NAM1",
		"RNAM:\t$NAM\tRSA=RSTR,RSS=64",
		"RSTR:\t.ASCII\t/[TEST]B.TXT;1/",
		"RSTRL = .-RSTR",
		"\t.BLKB\t64",
	)
	rlf := []string{"\tMOVB\t#RSTRL,RNAM+NAM$B_RSL", "\tMOVAB\tRNAM,NAM1+NAM$L_RLF"}
	cases := []pcase{
		{"[TEST]A.DAT", "", nil},
		{"A", "[TEST].DAT", nil},
		{"[TEST]*.DAT;*", "", nil},
		{"[TEST...]*.*", "", nil},
		{"[TEST.SUB]C", ".DAT", nil},
		{"[NOSUCH]X.Y", "", nil},
		{"ZZA0:X.Y", "", nil},
		{"[TEST]A.DAT", "", []string{"\tMOVB\t#NAM$M_SYNCHK,NAM1+NAM$B_NOP"}},
		{"[TEST]A.DAT", "", []string{"\tMOVB\t#10,NAM1+NAM$B_ESS"}},
		{"A.B.C", "", nil},
		{"TST:A.DAT", "", nil},
		{"TSL:C.DAT", "", nil},
		{"X", "", rlf},
		{"X", "", append([]string{"\tBISL2\t#FAB$M_OFP,FAB1+FAB$L_FOP"}, rlf...)},
		{"", "[TEST]A.DAT", nil},
		{"[TEST]A.DAT;2", "", nil},
		{"[TEST]A.DAT;-1", "", nil},
		{"[test]a.dat", "", nil},
		{"[TEST]A%.DAT", "", nil},
		{"[-]A.DAT", "[TEST.SUB]", nil},
		{"[.SUB]C.DAT", "[TEST]", nil},
		{"SYS$DISK:[TEST]A.DAT", "", nil},
		{"[TEST]ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMN.DAT", "", nil},
		{"[TEST]A.DAT;32768", "", nil},
		{"[TEST]", "", nil},
		{"A.DAT", "", nil},
		{"[TEST]A.DAT", "", []string{"\tCLRB\tNAM1+NAM$B_ESS"}},
		{"[TEST]A.DAT", "", []string{"\tMOVB\t#NAM$M_SYNCHK,NAM1+NAM$B_NOP", "\tMOVB\t#2,NAM1+NAM$B_ESS"}},
		{"[TEST]A.DAT", "", []string{"\tMOVW\t#1,FAB1+FAB$W_IFI"}},
		{"[TEST]A.DAT", "", []string{"\tMOVB\t#NAM$C_BID+1,NAM1+NAM$B_BID"}},
	}
	for i, c := range cases {
		p.reset(i+1, c.fn, c.dn)
		p.emit(c.pre...)
		p.call("PARSE", opParse)
		p.dumpAll()
	}
	p.write()
}

func searchProg() {
	p := newProg("SEARCH", `; $SEARCH: each case is a $PARSE, then $SEARCH until it fails (at most
; 30 times). The last case calls $SEARCH with no $PARSE first.
`)
	p.data = append(p.data, "FABT:\t$FAB\tNAM=NAM1")
	specs := []string{
		"[TEST]*.DAT;*",
		"[TEST]A.DAT",
		"[TEST...]*.*;*",
		"[TEST]*.NONE",
		"[TEST.EMPTY]*.*;*",
		"TSL:*.DAT",
		"[TEST]A.DAT;*",
		"[TEST]%.DAT",
		"[*]*.DAT",
		"[TEST]A.DAT;-1",
		"[TEST]A.DAT;0",
		"[TEST]*.*",
		"[TEST]NONE.DAT",
		"[NOSUCH]*.*",
		"[TEST]*.DAT;2",
	}
	for i, s := range specs {
		step := i + 1
		p.reset(step, s, "")
		p.call("PARSE", opParse)
		p.dumpAll()
		loop, done := p.label(), p.label()
		p.skipIfFailed(done)
		p.emit(
			"\tCLRL\tR7",
			loop+":\tINCL\tR7",
			fmt.Sprintf("\tADDL3\t#%d,R7,STEP", 100*step),
		)
		p.call("SEARCH", opSearch)
		p.dumpAll()
		p.emit(
			"\tBLBC\tSTAT,"+done,
			"\tCMPL\tR7,#30",
			"\tBLSS\t"+loop,
			done+":",
		)
	}
	p.reset(len(specs)+1, "[TEST]A.DAT", "")
	p.call("SEARCH", opSearch)
	p.dumpAll()
	p.write()
}

func openProg() {
	p := newProg("OPEN", `; $OPEN with a NAM: the NAM and strings after each open, then $CLOSE's R0.
`)
	p.data = append(p.data, "FABT:\t$FAB\tFAC=GET,NAM=NAM1")
	cases := []pcase{
		{"[TEST]A.DAT", "", nil},
		{"[TEST]A.DAT;1", "", nil},
		{"A", "[TEST].DAT", nil},
		{"[TEST]NONE.DAT", "", nil},
		{"[NOSUCH]X.DAT", "", nil},
		{"[TEST]*.DAT", "", nil},
		{"TSL:A.DAT", "", nil},
		{"TST:B.TXT", "", nil},
		{"[TEST]A.DAT;-1", "", nil},
		{"[TEST.SUB]C.DAT", "", []string{"\tMOVB\t#10,NAM1+NAM$B_RSS"}},
		{"[TEST]A.DAT", "", []string{"\tCLRB\tNAM1+NAM$B_ESS", "\tCLRB\tNAM1+NAM$B_RSS"}},
		{"[TEST]A.DAT", "", []string{"\tCLRL\tFAB1+FAB$L_NAM"}},
		{"[TEST]F.DAT", "", nil},
		{"[000000]TEST.DIR", "", nil},
		{"[TEST]A.DAT;3", "", nil},
		{"[TEST]A.DAT;4", "", nil},
	}
	for i, c := range cases {
		p.reset(i+1, c.fn, c.dn)
		p.emit(c.pre...)
		p.call("OPEN", opOpen)
		p.dumpAll()
		p.closeIfOpen()
	}
	p.write()
}

// xabChain is a chain of the six readable XABs, with a template copy laid
// out the same way, so a case can reset it with one MOVC3.
var xabChain = []string{
	"XDAT:\t$XABDAT\tNXT=XRDT",
	"XRDT:\t$XABRDT\tNXT=XFHC",
	"XFHC:\t$XABFHC\tNXT=XPRO",
	"XPRO:\t$XABPRO\tNXT=XALL",
	"XALL:\t$XABALL\tNXT=XSUM",
	"XSUM:\t$XABSUM",
	"XEND:",
	"TDAT:\t$XABDAT\tNXT=XRDT",
	"\t$XABRDT\tNXT=XFHC",
	"\t$XABFHC\tNXT=XPRO",
	"\t$XABPRO\tNXT=XALL",
	"\t$XABALL\tNXT=XSUM",
	"\t$XABSUM",
}

var xabDefs = []string{"$XABDEF", "$XABDATDEF", "$XABRDTDEF", "$XABFHCDEF", "$XABPRODEF", "$XABALLDEF", "$XABSUMDEF", "$XABKEYDEF"}

// dumpChain writes each XAB of the chain.
func (p *prog) dumpChain() {
	p.emit(
		"\tDUMP\tXDAT,XDAT,XAB$C_DATLEN",
		"\tDUMP\tXRDT,XRDT,XAB$C_RDTLEN",
		"\tDUMP\tXFHC,XFHC,XAB$C_FHCLEN",
		"\tDUMP\tXPRO,XPRO,XAB$C_PROLEN",
		"\tDUMP\tXALL,XALL,XAB$C_ALLLEN",
		"\tDUMP\tXSUM,XSUM,XAB$C_SUMLEN",
	)
}

func xabProg() {
	p := newProg("XAB", `; XABs on $OPEN and $DISPLAY: each case resets the chain DAT, RDT, FHC,
; PRO, ALL, SUM (filled with ^XEE past each XAB's COD and BLN, so what RMS
; writes shows), opens a file, and writes the FAB, NAM, and every XAB.
`)
	p.defs = xabDefs
	p.data = append(p.data, "FABT:\t$FAB\tFAC=GET,NAM=NAM1")
	p.data = append(p.data, xabChain...)
	p.data = append(p.data,
		"XKEY:\t$XABKEY",
		"DUP1:\t$XABDAT\tNXT=DUP2",
		"DUP2:\t$XABDAT",
	)
	resetChain := []string{
		"\tMOVC3\t#XEND-XDAT,TDAT,XDAT",
		"\tMOVAB\tXDAT,FAB1+FAB$L_XAB",
	}
	type xcase struct {
		fn      string
		pre     []string
		display bool
	}
	cases := []xcase{
		{"[TEST]B.TXT", resetChain, false},
		{"[TEST]F.DAT", resetChain, false},
		{"[TEST.SUB]C.DAT", nil, true},
		{"[TEST]A.DAT;1", resetChain, false},
		{"[TEST]B.TXT", append(append([]string{}, resetChain...), "\tMOVB\t#99,XDAT+XAB$B_COD"), false},
		{"[TEST]B.TXT", append(append([]string{}, resetChain...), "\tMOVB\t#4,XDAT+XAB$B_BLN"), false},
		{"[TEST]B.TXT", []string{"\tMOVAB\tXKEY,FAB1+FAB$L_XAB"}, false},
		{"[TEST]B.TXT", []string{"\tMOVAB\tDUP1,FAB1+FAB$L_XAB"}, false},
		{"[000000]TEST.DIR", resetChain, false},
		{"[TEST]A.DAT", resetChain, true},
	}
	for i, c := range cases {
		p.reset(i+1, c.fn, "")
		p.emit(c.pre...)
		p.call("OPEN", opOpen)
		p.dumpAll()
		if c.display {
			l := p.label()
			p.skipIfFailed(l)
			p.emit(resetChain[0], resetChain[1])
			p.call("DISPLAY", opDisplay)
			p.dumpAll()
			p.emit(l + ":")
		}
		p.dumpChain()
		p.emit("\tDUMP\tXKEY,XKEY,XAB$C_KEYLEN", "\tDUMP\tDUP1,DUP1,XAB$C_DATLEN", "\tDUMP\tDUP2,DUP2,XAB$C_DATLEN")
		p.closeIfOpen()
	}
	p.write()
}

func namfidProg() {
	p := newProg("NAMFID", `; $OPEN by name block (FAB$V_NAM). Case 1 finds [TEST]B.TXT with $PARSE
; and $SEARCH and saves the NAM; the rest open from its FID, DID, and DVI.
`)
	p.data = append(p.data,
		"FABT:\t$FAB\tFAC=GET,NAM=NAM1",
		"NSAV:\t.BLKB\tNAM$C_BLN",
	)
	p.reset(1, "[TEST]B.TXT", "")
	p.call("PARSE", opParse)
	p.dumpAll()
	p.call("SEARCH", opSearch)
	p.dumpAll()
	p.emit("\tMOVC3\t#NAM$C_BLN,NAM1,NSAV")

	nam := "\tBISL2\t#FAB$M_NAM,FAB1+FAB$L_FOP"
	fid := "\tMOVC3\t#6,NSAV+NAM$W_FID,NAM1+NAM$W_FID"
	did := "\tMOVC3\t#6,NSAV+NAM$W_DID,NAM1+NAM$W_DID"
	dvi := "\tMOVC3\t#16,NSAV+NAM$T_DVI,NAM1+NAM$T_DVI"
	cases := []pcase{
		{"", "", []string{nam, fid, dvi}},
		{"B.TXT", "", []string{nam, did, dvi}},
		{"", "", []string{nam, dvi, "\tMOVW\t#999,NAM1+NAM$W_FID", "\tMOVW\t#7,NAM1+NAM$W_FID_SEQ"}},
		{"", "", []string{nam, fid}},
		{"[TEST.SUB]C.DAT", "", []string{nam, did, dvi}},
		{"[TEST]A.DAT", "", []string{nam, fid, dvi}},
		{"[TEST]A.DAT", "", []string{fid, dvi}},
		{"NONE.DAT", "", []string{nam, did, dvi}},
		{"*.TXT", "", []string{nam, did, dvi}},
	}
	for i, c := range cases {
		p.reset(i+2, c.fn, c.dn)
		p.emit(c.pre...)
		p.call("OPEN", opOpen)
		p.dumpAll()
		p.closeIfOpen()
	}
	p.write()
}

func createProg() {
	edt0, edt1 := vmsTime(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC))
	rdt0, rdt1 := vmsTime(time.Date(2031, time.February, 2, 12, 0, 0, 0, time.UTC))
	p := newProg("CREATE", `; $CREATE with a NAM and XABs, and $CLOSE's XABRDT and XABPRO. Files go
; in [CRE], which RUN.COM empties first. Case 1's chain gives a
; protection, an allocation, and an expiration date (1-JAN-2030); case 7
; closes with a revision date (2-FEB-2031 12:00), a revision count, and a
; new protection, then reopens the file to read them back.
`)
	p.defs = xabDefs
	p.data = append(p.data,
		"FABT:\t$FAB\tFAC=PUT,NAM=NAM1,RFM=VAR",
		"CPRO:\t$XABPRO\tPRO=<RWED,RWED,RE,>,NXT=CALL",
		"CALL:\t$XABALL\tALQ=7,DEQ=3,NXT=CDAT",
		"CDAT:\t$XABDAT",
		"KRDT:\t$XABRDT\tNXT=KPRO",
		"KPRO:\t$XABPRO\tPRO=<RWED,RWED,,>",
		"RRDT:\t$XABRDT\tNXT=RPRO",
		"RPRO:\t$XABPRO\tNXT=RDAT",
		"RDAT:\t$XABDAT\tNXT=RALL",
		"RALL:\t$XABALL\tNXT=RFHC",
		"RFHC:\t$XABFHC",
	)
	create := func(step int, fn, dn string, pre ...string) {
		p.reset(step, fn, dn)
		p.emit(pre...)
		p.call("CREATE", opCreate)
		p.dumpAll()
	}
	chain := func(c ...string) {
		p.emit(c...)
		p.emit(
			"\tDUMP\tCPRO,CPRO,XAB$C_PROLEN",
			"\tDUMP\tCALL,CALL,XAB$C_ALLLEN",
			"\tDUMP\tCDAT,CDAT,XAB$C_DATLEN",
		)
	}

	create(1, "[CRE]N1.DAT", "",
		"\tMOVAB\tCPRO,FAB1+FAB$L_XAB",
		fmt.Sprintf("\tMOVL\t#^X%08X,CDAT+XAB$Q_EDT", edt0),
		fmt.Sprintf("\tMOVL\t#^X%08X,CDAT+XAB$Q_EDT+4", edt1))
	chain()
	p.closeIfOpen()
	create(2, "[CRE]N1.DAT", "")
	p.closeIfOpen()
	create(3, "[CRE]N1.DAT;1", "")
	p.closeIfOpen()
	create(4, "N1", "[CRE].DAT", "\tBISL2\t#FAB$M_CIF,FAB1+FAB$L_FOP")
	p.closeIfOpen()
	create(5, "[CRE]N2.DAT;5", "")
	p.closeIfOpen()
	create(6, "[CRE]N2.DAT;3", "")
	p.closeIfOpen()

	// Case 7: close with KRDT and KPRO, then reopen.
	create(7, "[CRE]N3.DAT", "")
	l := p.label()
	p.skipIfFailed(l)
	p.emit(
		fmt.Sprintf("\tMOVL\t#^X%08X,KRDT+XAB$Q_RDT", rdt0),
		fmt.Sprintf("\tMOVL\t#^X%08X,KRDT+XAB$Q_RDT+4", rdt1),
		"\tMOVW\t#9,KRDT+XAB$W_RVN",
		"\tMOVAB\tKRDT,FAB1+FAB$L_XAB",
		"\t$CLOSE\tFAB=FAB1",
		"\tMOVL\tR0,STAT",
		fmt.Sprintf("\tMOVL\t#%d,OP", opClose),
		"\tDUMP\tSTAT,STAT,4",
		"\tDUMP\tKRDT,KRDT,XAB$C_RDTLEN",
		"\tDUMP\tKPRO,KPRO,XAB$C_PROLEN",
		l+":",
	)
	p.reset(107, "[CRE]N3.DAT", "")
	p.emit("\tMOVB\t#FAB$M_GET,FAB1+FAB$B_FAC", "\tMOVAB\tRRDT,FAB1+FAB$L_XAB")
	p.call("OPEN", opOpen)
	p.dumpAll()
	p.emit(
		"\tDUMP\tRRDT,RRDT,XAB$C_RDTLEN",
		"\tDUMP\tRPRO,RPRO,XAB$C_PROLEN",
		"\tDUMP\tRDAT,RDAT,XAB$C_DATLEN",
		"\tDUMP\tRALL,RALL,XAB$C_ALLLEN",
		"\tDUMP\tRFHC,RFHC,XAB$C_FHCLEN",
	)
	p.closeIfOpen()

	create(8, "[NOSUCH]N.DAT", "")
	p.closeIfOpen()
	create(9, "[CRE]*.DAT", "")
	p.closeIfOpen()
	create(10, "[CRE]N4", ".LIS")
	p.closeIfOpen()

	// Case 11: reopen case 1's file to read back its XABs.
	p.reset(11, "[CRE]N1.DAT;1", "")
	p.emit("\tMOVB\t#FAB$M_GET,FAB1+FAB$B_FAC", "\tMOVAB\tRRDT,FAB1+FAB$L_XAB")
	p.call("OPEN", opOpen)
	p.dumpAll()
	p.emit(
		"\tDUMP\tRRDT,RRDT,XAB$C_RDTLEN",
		"\tDUMP\tRPRO,RPRO,XAB$C_PROLEN",
		"\tDUMP\tRDAT,RDAT,XAB$C_DATLEN",
		"\tDUMP\tRALL,RALL,XAB$C_ALLLEN",
		"\tDUMP\tRFHC,RFHC,XAB$C_FHCLEN",
	)
	p.closeIfOpen()
	p.write()
}

// The probes, in the order RUN.COM runs them: CREATE last, so the others
// never see [CRE]'s files.
var probes = []string{"PARSE", "SEARCH", "OPEN", "XAB", "NAMFID", "CREATE"}

const buildCom = `$ ! BUILD.COM - builds the Phase 33 oracle's test tree. Written by
$ ! testdata/mar/rms3/gen.go. Run it once, with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @BUILD/OUTPUT=BUILD.LOG
$ !
$ SET VERIFY
$ CREATE/DIRECTORY [TEST]
$ CREATE/DIRECTORY [TEST.SUB]
$ CREATE/DIRECTORY [TEST.EMPTY]
$ CREATE/DIRECTORY [OUT]
$ CREATE/DIRECTORY [CRE]
$ CREATE [TEST]A.DAT
A.DAT, the first version
$ CREATE [TEST]A.DAT
A.DAT, the second version
with two lines
$ CREATE [TEST]A.DAT
A.DAT, the third version
$ CREATE [TEST]B.TXT
B.TXT holds several lines,
so that its end-of-file block and
first free byte are worth reading.
The fourth line.
The fifth line, which is rather longer than the others before it.
$ SET FILE/PROTECTION=(S:RWED,O:RWED,G:RE,W) [TEST]B.TXT
$ CREATE [TEST]AB.DAT
AB.DAT
$ CREATE [TEST]C.DAT
[TEST]C.DAT
$ CREATE [TEST.SUB]C.DAT
[TEST.SUB]C.DAT
$ CREATE [TEST.SUB]D.DAT
[TEST.SUB]D.DAT
$ CREATE FIX.FDL
FILE
	ORGANIZATION	sequential
	ALLOCATION	10
	EXTENSION	5
RECORD
	FORMAT	fixed
	SIZE	80
	CARRIAGE_CONTROL	none
$ CREATE/FDL=FIX.FDL [TEST]F.DAT
$ DELETE FIX.FDL;*
$ DIRECTORY/FULL [TEST...]
$ DIRECTORY/FILE_ID [000000...]
`

func runCom() string {
	var b strings.Builder
	b.WriteString(`$ ! RUN.COM - assembles, links, and runs the Phase 33 oracle's probes.
$ ! Written by testdata/mar/rms3/gen.go. Run it after BUILD.COM, with the
$ ! exchange volume's [000000] as the default directory:
$ !
$ !     @RUN/OUTPUT=RUN.LOG
$ !
$ ! It empties [OUT] and [CRE] first, so it can be run again.
$ !
$ SET NOON
$ SET VERIFY
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE TST 'DEV'[TEST]
$ DEFINE TSL 'DEV'[TEST.SUB],'DEV'[TEST]
$ IF F$SEARCH("[CRE]*.*;*") .NES. "" THEN DELETE [CRE]*.*;*
$ IF F$SEARCH("[OUT]*.*;*") .NES. "" THEN DELETE [OUT]*.*;*
`)
	for _, p := range probes {
		fmt.Fprintf(&b, "$ MACRO/NOLIST %s\n$ LINK %s\n$ RUN %s\n", p, p, p)
	}
	b.WriteString(`$ DIRECTORY/FULL [CRE]
$ DIRECTORY/FILE_ID [OUT]
$ DEASSIGN TST
$ DEASSIGN TSL
`)

	return b.String()
}

func exchangeCmd() string {
	var b strings.Builder
	b.WriteString(`! EXCHANGE.CMD - builds the Phase 33 oracle's exchange volume with govax.
! Written by testdata/mar/rms3/gen.go. Run from the repository root:
!
!     govax console < testdata/mar/rms3/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/rms3-exchange.dsk" /DEVICE=RD51 RMSXCHG3
MOUNT/WRITE DUA1 "testdata/disks/rms3-exchange.dsk"
`)
	for _, p := range probes {
		fmt.Fprintf(&b, "COPY \"%s/%s.mar\"/HOST DUA1:[000000]%s.MAR\n", dir, strings.ToLower(p), p)
	}
	fmt.Fprintf(&b, "COPY \"%s/build.com\"/HOST DUA1:[000000]BUILD.COM\n", dir)
	fmt.Fprintf(&b, "COPY \"%s/run.com\"/HOST DUA1:[000000]RUN.COM\n", dir)
	b.WriteString("DIRECTORY DUA1:[000000]\nDISMOUNT DUA1\n")

	return b.String()
}

func main() {
	parseProg()
	searchProg()
	openProg()
	xabProg()
	namfidProg()
	createProg()
	for name, text := range map[string]string{
		"build.com":    buildCom,
		"run.com":      runCom(),
		"exchange.cmd": exchangeCmd(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
