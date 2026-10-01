//go:build ignore

// gen writes the Phase 32 oracle fixtures (docs/PHASE-32.md, subtask 3;
// docs/RMS-MACROS.md's oracle questions): VAX MACRO programs that call
// each RMS macro in each form, for real MACRO to assemble on VMS, plus
// the command procedures that do it and the govax console script that
// builds the exchange volume. Run it from the repository root:
//
//	go run testdata/mar/rms/gen.go
//
// Everything here is written by govax from its own tables
// (internal/vmsdef) and the RMS Reference Manual; nothing comes from VMS's
// macro library. The programs are assembled with /NOLIST, so no listing
// of an expansion is ever made.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

const dir = "testdata/mar/rms"

// fixture is one program and how it's run: probes is the main command
// procedure's, errors the error probes', whose messages go to their own
// log for the author to audit.
type fixture struct {
	name   string
	text   string
	errors bool
}

var fixtures []fixture

func add(name string, errs bool, title string, body string) {
	text := fmt.Sprintf("\t.TITLE\t%s\t%s\n\t.IDENT\t/V1.0/\n;\n; Written by testdata/mar/rms/gen.go (docs/PHASE-32.md, subtask 3).\n;\n%s\t.END\n",
		strings.ToUpper(name), title, body)
	fixtures = append(fixtures, fixture{name: name, text: text, errors: errs})
}

// Manual-derived names for the blocks govax has no values for yet
// (docs/RMS-MACROS.md, O9): the RMS Reference Manual's chapters 12, 14,
// and 18 name these fields, and its appendix A these keywords.
var manualXAB = []string{
	"XAB$C_KEY", "XAB$C_KEYLEN", "XAB$C_SUM", "XAB$C_SUMLEN", "XAB$C_ITM", "XAB$C_ITMLEN",
	"XAB$K_SENSEMODE", "XAB$K_SETMODE", "XAB$L_ITEMLIST", "XAB$B_MODE",
	"XAB$L_COLNAM", "XAB$L_COLSIZ", "XAB$L_COLTBL", "XAB$B_DAN", "XAB$B_DBS", "XAB$B_DTP",
	"XAB$B_FLG", "XAB$B_IAN", "XAB$B_IBS", "XAB$B_LAN", "XAB$B_LVL", "XAB$B_NSG",
	"XAB$B_NUL", "XAB$B_PROLOG", "XAB$B_REF", "XAB$B_KREF", "XAB$B_TKS", "XAB$L_DVB",
	"XAB$L_KNM", "XAB$L_RVB", "XAB$W_DFL", "XAB$W_IFL", "XAB$W_LRL", "XAB$W_MRL",
	"XAB$B_NOA", "XAB$B_NOK", "XAB$W_PVN",
	"XAB$M_CHG", "XAB$M_DUP", "XAB$M_NUL", "XAB$M_DAT_NCMPR", "XAB$M_IDX_NCMPR", "XAB$M_KEY_NCMPR",
	"XAB$V_CHG", "XAB$V_DUP", "XAB$V_NUL", "XAB$V_DAT_NCMPR", "XAB$V_IDX_NCMPR", "XAB$V_KEY_NCMPR",
}

var dtp = []string{"BN2", "DBN2", "BN4", "DBN4", "BN8", "DBN8", "IN2", "DIN2", "IN4", "DIN4",
	"IN8", "DIN8", "COL", "DCOL", "PAC", "DPAC", "STG", "DSTG"}

func init() {
	for i := 0; i < 8; i++ {
		manualXAB = append(manualXAB, fmt.Sprintf("XAB$W_POS%d", i), fmt.Sprintf("XAB$B_SIZ%d", i))
	}

	for _, d := range dtp {
		manualXAB = append(manualXAB, "XAB$C_"+d)
	}
}

func candidates(prefixes ...string) []string {
	seen := map[string]bool{}

	// A MACRO-32 symbol is at most 31 characters, so a longer name (one
	// BLISS listing has) is no macro's to define.
	for _, n := range vmsdef.SymbolNames(prefixes...) {
		if len(n) <= 31 {
			seen[strings.ToUpper(n)] = true
		}
	}

	for _, p := range prefixes {
		if p == "XAB$" {
			for _, n := range manualXAB {
				seen[n] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}

	sort.Strings(out)

	return out
}

// The definition probes (O9, O13): each $xxxDEF alone, then a .LONG of
// every candidate name. One the macro defines is data; one it doesn't is
// an external reference in the object's GSD.
func definitionProbes() {
	defs := []struct {
		macro    string
		prefixes []string
	}{
		{"FAB", []string{"FAB$"}}, {"RAB", []string{"RAB$"}}, {"NAM", []string{"NAM$"}},
		{"XAB", []string{"XAB$"}}, {"XABALL", []string{"XAB$"}}, {"XABDAT", []string{"XAB$"}},
		{"XABFHC", []string{"XAB$"}}, {"XABITM", []string{"XAB$"}}, {"XABKEY", []string{"XAB$"}},
		{"XABPRO", []string{"XAB$"}}, {"XABRDT", []string{"XAB$"}}, {"XABSUM", []string{"XAB$"}},
		{"XABTRM", []string{"XAB$"}}, {"RMS", []string{"RMS$"}},
		{"SS", []string{"SS$_"}}, {"IO", []string{"IO$"}}, {"JPI", []string{"JPI$"}},
		{"DVI", []string{"DVI$"}}, {"SYI", []string{"SYI$"}}, {"LNM", []string{"LNM$"}},
		{"DEV", []string{"DEV$"}}, {"TT", []string{"TT$", "TT2$"}}, {"TT2", []string{"TT2$"}},
		{"PRT", []string{"PRT$"}}, {"PRV", []string{"PRV$"}}, {"BRK", []string{"BRK$"}},
		{"FIB", []string{"FIB$"}}, {"ATR", []string{"ATR$"}}, {"STATE", []string{"SCH$"}},
	}

	for _, d := range defs {
		var b strings.Builder

		fmt.Fprintf(&b, "\t$%sDEF\n\t.PSECT\tDATA,NOEXE,WRT,LONG\n", d.macro)

		for _, n := range candidates(d.prefixes...) {
			fmt.Fprintf(&b, "\t.LONG\t%s\n", n)
		}

		add("def_"+strings.ToLower(d.macro), false, "$"+d.macro+"DEF's names", b.String())
	}

	add("def_twice", false, "$FABDEF called twice",
		"\t$FABDEF\n\t$FABDEF\n\t.PSECT\tDATA,NOEXE,WRT,LONG\n\t.LONG\tFAB$L_FNA\n")
}

// Labels the probes' address arguments refer to.
const labels = `
	.PSECT	LABELS,NOEXE,WRT,LONG
FAB1:	.BLKB	80
FAB2:	.BLKB	80
NAM1:	.BLKB	96
NAM2:	.BLKB	96
XAB1:	.BLKB	64
XAB2:	.BLKB	64
KBUF:	.BLKB	16
PBUF:	.BLKB	16
RBUF:	.BLKB	16
HBUF:	.BLKB	16
UBUF:	.BLKB	16
EBUF:	.BLKB	16
SBUF:	.BLKB	16
CTAB:	.BLKB	16
KNAM:	.BLKB	32
ABUF:	.BLKB	16
ILST:	.BLKB	16
NAMA:	.ASCII	/A.DAT/
NAMB:	.ASCII	/SYS$DISK:[]/
VALW:	.WORD	300
VALL:	.LONG	70000
QUAD:	.QUAD	^X0123456789ABCDEF
TRIP:	.WORD	1,2,3
DVIB:	.BLKB	16
PROW:	.WORD	^XFF00
UICL:	.LONG	^X00FF00FF
`

// The initialization probes (O2-O8, O10, O11).
type block struct {
	macro string
	all   string   // every keyword, with distinctive values
	more  []string // further single-block variants
}

var blocks = []block{
	{"FAB", "ALQ=500, BKS=4, BLS=512, CHAN_MODE=2, CTX=12345, DEQ=7, -\n\t\tDNM=<SYS$DISK:[].DAT>, FAC=<GET,PUT,UPD>, FNM=<A.DAT>, -\n\t\tFOP=<CTG,SUP>, FSZ=3, GBC=9, LNM_MODE=3, MRN=1000, MRS=132, -\n\t\tNAM=NAM1, ORG=REL, RAT=<CR,BLK>, RFM=FIX, RTV=8, -\n\t\tSHR=<GET,PUT>, XAB=XAB1",
		[]string{"FNA=NAMA, FNS=5, DNA=NAMB, DNS=11", "FNM=<A.DAT>", "DNM=<B.DAT>", "FAC=GET", "FAC=PUT"}},
	{"RAB", "BKT=17, CTX=99, FAB=FAB1, KBF=KBUF, KRF=2, KSZ=8, MBC=16, -\n\t\tMBF=3, PBF=PBUF, PSZ=4, RAC=KEY, RBF=RBUF, RHB=HBUF, -\n\t\tROP=<LOC,RAH,WBH>, RSZ=80, TMO=10, UBF=UBUF, USZ=200, XAB=XAB1",
		[]string{"FAB=FAB1"}},
	{"NAM", "ESA=EBUF, ESS=255, NOP=<PWD,SYNCHK>, RLF=NAM2, RSA=RBUF, RSS=128", nil},
	{"XABALL", "AID=1, ALN=LBN, ALQ=100, AOP=<CTG,HRD>, BKZ=2, DEQ=10, -\n\t\tLOC=500, NXT=XAB2, RFI=<1,2,3>, VOL=1", nil},
	{"XABDAT", "NXT=XAB2", nil},
	{"XABFHC", "NXT=XAB2", nil},
	{"XABITM", "ITEMLIST=ILST, MODE=SETMODE, NXT=XAB2", []string{"ITEMLIST=ILST"}},
	{"XABKEY", "COLTBL=CTAB, DAN=1, DFL=200, DTP=STG, FLG=<CHG,DUP>, IAN=2, -\n\t\tIFL=300, KNM=KNAM, LAN=3, NUL=32, NXT=XAB2, POS=<0,10>, -\n\t\tPROLOG=3, REF=1, SIZ=<4,6>",
		[]string{"REF=0, POS=0, SIZ=10", "REF=1, POS=0, SIZ=10", "REF=1, POS=0, SIZ=10, FLG=CHG",
			"POS=<1,2,3,4,5,6,7,8>, SIZ=<1,2,3,4,5,6,7,8>"}},
	{"XABPRO", "ACLBUF=ABUF, ACLCTX=<7>, ACLSIZ=64, MTACC=^A/Z/, NXT=XAB2, -\n\t\tPRO=<RWED,RWED,RE,R>, PROT_OPT=<PROPAGATE>, UIC=<377,377>",
		[]string{"PRO=<RW,,R>", "PRO=<,,,>", "PRO=<RWED,RWED,RWED,RWED>", "PRO=<,,,R>", "PRO=<R>", "UIC=<1,2>"}},
	{"XABRDT", "NXT=XAB2", nil},
	{"XABSUM", "NXT=XAB2", nil},
	{"XABTRM", "ITMLST=ILST, ITMLST_LEN=24, NXT=XAB2", nil},
}

// options lists each option or single-choice keyword's values, one block
// apiece (docs/RMS-MACROS.md's encodings).
var options = map[string][][2]string{
	"FAB": {
		{"FAC", "BIO BRO DEL GET PUT TRN UPD"},
		{"FOP", "CBT CIF CTG DFW DLT MXV NAM NEF NFS OFP POS RCK RWC RWO SCF SPL SQO SUP TEF TMD TMP UFO WCK"},
		{"ORG", "IDX REL SEQ"}, {"RAT", "BLK CR FTN PRN"},
		{"RFM", "FIX STM STMCR STMLF UDF VAR VFC"}, {"SHR", "DEL GET MSE NIL PUT UPD UPI"},
	},
	"RAB": {
		{"RAC", "KEY RFA SEQ"},
		{"ROP", "ASY BIO CCO CDK CVT EOF EQNXT ETO FDL KGE KGT LIM LOA LOC NLK NXR NXT PMT PTA RAH REA REV RLK RNE RNF RRL TMO TPT UIF ULK WAT WBH"},
	},
	"NAM":    {{"NOP", "NOCONCEAL PWD SRCHXABS SYNCHK"}},
	"XABALL": {{"ALN", "ANY CYL LBN RFI VBN"}, {"AOP", "CBT CTG HRD ONC"}},
	"XABITM": {{"MODE", "SENSEMODE SETMODE"}},
	"XABKEY": {{"DTP", strings.Join(dtp, " ")}, {"FLG", "CHG DAT_NCMPR DUP IDX_NCMPR KEY_NCMPR NUL"}},
	"XABPRO": {{"PROT_OPT", "PROPAGATE"}},
}

func initProbes() {
	for _, bl := range blocks {
		m := strings.ToLower(bl.macro)
		data := "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"

		// Defaults only, and a use of one of the block's symbols (O2).
		sym := map[string]string{"FAB": "FAB$L_FNA", "RAB": "RAB$L_FAB", "NAM": "NAM$L_RSA"}[bl.macro]
		if sym == "" {
			sym = "XAB$L_NXT"
		}

		add("init_"+m, false, "$"+bl.macro+" with defaults",
			data+"B1:\t$"+bl.macro+"\n\t.LONG\t"+sym+"\n")

		add("init_"+m+"_all", false, "$"+bl.macro+" with every keyword",
			data+"B1:\t$"+bl.macro+"\t"+bl.all+"\n"+labels)

		var b strings.Builder

		b.WriteString(data)

		n := 1
		block := func(args string) {
			fmt.Fprintf(&b, "\t.ALIGN\tLONG\nB%d:\t$%s\t%s\n", n, bl.macro, args)
			n++
		}

		for _, v := range bl.more {
			block(v)
		}

		for _, o := range options[bl.macro] {
			for _, kw := range strings.Fields(o[1]) {
				block(o[0] + "=" + kw)
			}
		}

		if n > 1 {
			b.WriteString(labels)
			add("init_"+m+"_opt", false, "$"+bl.macro+" with each option alone", b.String())
		}
	}
}

// The store probes (O12): each _STORE macro, each argument kind, in code.
var stores = map[string][]string{
	"FAB": {"FAB=FAB1, ALQ=#500", "FAB=R6, ALQ=#500", "ALQ=#500", "FAB=FAB1, ALQ=R7", "FAB=FAB1, ALQ=VALL",
		"FAB=FAB1, BKS=#4", "FAB=FAB1, BLS=#512", "FAB=FAB1, CHAN_MODE=#2", "FAB=FAB1, CTX=VALL",
		"FAB=FAB1, DEQ=#7", "FAB=FAB1, DNA=NAMB", "FAB=FAB1, DNA=R8", "FAB=FAB1, DNS=#11",
		"FAB=FAB1, FAC=GET", "FAB=FAB1, FAC=<GET,PUT>", "FAB=FAB1, FNA=NAMA", "FAB=FAB1, FNS=#5",
		"FAB=FAB1, FOP=<CTG,SUP>", "FAB=FAB1, FSZ=#3", "FAB=FAB1, GBC=#9", "FAB=FAB1, LNM_MODE=#3",
		"FAB=FAB1, MRN=#1000", "FAB=FAB1, MRS=#132", "FAB=FAB1, NAM=NAM1", "FAB=FAB1, ORG=REL",
		"FAB=FAB1, RAT=<CR,BLK>", "FAB=FAB1, RFM=FIX", "FAB=FAB1, RTV=#8", "FAB=FAB1, SHR=<GET,PUT>",
		"FAB=FAB1, XAB=XAB1", "FAB=FAB1, ALQ=#500, FAC=GET, FNA=NAMA"},
	"RAB": {"RAB=RBUF, BKT=#17", "RAB=R6, BKT=#17", "BKT=#17", "RAB=RBUF, CTX=VALL", "RAB=RBUF, FAB=FAB1",
		"RAB=RBUF, KBF=KBUF", "RAB=RBUF, KRF=#2", "RAB=RBUF, KSZ=#8", "RAB=RBUF, MBC=#16", "RAB=RBUF, MBF=#3",
		"RAB=RBUF, PBF=PBUF", "RAB=RBUF, PSZ=#4", "RAB=RBUF, RAC=KEY", "RAB=RBUF, RBF=UBUF",
		"RAB=RBUF, RFA=TRIP", "RAB=RBUF, RFA=R2", "RAB=RBUF, RHB=HBUF", "RAB=RBUF, ROP=<LOC,RAH>",
		"RAB=RBUF, RSZ=#80", "RAB=RBUF, TMO=#10", "RAB=RBUF, UBF=UBUF", "RAB=RBUF, USZ=#200", "RAB=RBUF, XAB=XAB1"},
	"NAM": {"NAM=NAM1, DID=TRIP", "NAM=NAM1, DID=R2", "NAM=NAM1, DVI=DVIB", "NAM=NAM1, ESA=EBUF",
		"NAM=NAM1, ESS=#255", "NAM=NAM1, FID=TRIP", "NAM=NAM1, FID=R2", "NAM=NAM1, NOP=<PWD,SYNCHK>",
		"NAM=NAM1, RLF=NAM2", "NAM=NAM1, RSA=RBUF", "NAM=NAM1, RSS=#128", "NAM=R6, RSS=#128", "RSS=#128"},
	"XABALL": {"XAB=XAB1, AID=#1", "XAB=XAB1, ALN=LBN", "XAB=XAB1, ALQ=#100", "XAB=XAB1, AOP=<CTG,HRD>",
		"XAB=XAB1, BKZ=#2", "XAB=XAB1, DEQ=#10", "XAB=XAB1, LOC=#500", "XAB=XAB1, NXT=XAB2",
		"XAB=XAB1, RFI=TRIP", "XAB=XAB1, RFI=R2", "XAB=XAB1, VOL=#1", "XAB=R6, VOL=#1", "VOL=#1"},
	"XABDAT": {"XAB=XAB1, CDT=QUAD", "XAB=XAB1, CDT=R2", "XAB=XAB1, EDT=QUAD", "XAB=XAB1, RDT=QUAD",
		"XAB=XAB1, RVN=#3", "XAB=XAB1, NXT=XAB2"},
	"XABFHC": {"XAB=XAB1, NXT=XAB2"},
	"XABKEY": {"XAB=XAB1, COLTBL=#CTAB", "XAB=XAB1, DAN=#1", "XAB=XAB1, DFL=#200", "XAB=XAB1, DTP=STG",
		"XAB=XAB1, FLG=<CHG,DUP>", "XAB=XAB1, IAN=#2", "XAB=XAB1, IFL=#300", "XAB=XAB1, KNM=KNAM",
		"XAB=XAB1, LAN=#3", "XAB=XAB1, NUL=#32", "XAB=XAB1, NXT=XAB2", "XAB=XAB1, POS=<#0,#10>",
		"XAB=XAB1, POS0=#5", "XAB=XAB1, PROLOG=#3", "XAB=XAB1, REF=#1", "XAB=XAB1, SIZ=<#4,#6>",
		"XAB=XAB1, SIZ0=#7"},
	"XABPRO": {"XAB=XAB1, ACLBUF=ABUF", "XAB=XAB1, ACLCTX=#7", "XAB=XAB1, ACLSIZ=#64",
		"XAB=XAB1, MTACC=#^A/Z/", "XAB=XAB1, NXT=XAB2", "XAB=XAB1, PRO=<RWED,RWED,RE,R>",
		"XAB=XAB1, PRO=PROW", "XAB=XAB1, PROT_OPT=<PROPAGATE>", "XAB=XAB1, UIC=<377,377>",
		"XAB=XAB1, UIC=UICL"},
	"XABRDT": {"XAB=XAB1, RDT=QUAD", "XAB=XAB1, RDT=R2", "XAB=XAB1, RVN=#3", "XAB=XAB1, NXT=XAB2"},
	"XABSUM": {"XAB=XAB1, NXT=XAB2"},
	"XABTRM": {"XAB=XAB1, ITMLST=ILST", "XAB=XAB1, ITMLST_LEN=#24", "XAB=XAB1, NXT=XAB2"},
}

func storeProbes() {
	names := make([]string, 0, len(stores))
	for m := range stores {
		names = append(names, m)
	}

	sort.Strings(names)

	for _, m := range names {
		var b strings.Builder

		fmt.Fprintf(&b, "\t.PSECT\tCODE,EXE,NOWRT,LONG\n\t.ENTRY\tSTORE,^M<R2,R3,R6,R7,R8>\n")

		for _, args := range stores[m] {
			fmt.Fprintf(&b, "\t$%s_STORE\t%s\n", m, args)
		}

		fmt.Fprintf(&b, "\tRET\n%s", labels)
		add("store_"+strings.ToLower(m), false, "$"+m+"_STORE in each form", b.String())
	}
}

// The service probes (O14): every service, every form.
func serviceProbes() {
	fab := strings.Fields("CLOSE CREATE DISPLAY ENTER ERASE EXTEND OPEN PARSE REMOVE SEARCH")
	rab := strings.Fields("CONNECT DELETE DISCONNECT FIND FLUSH FREE GET NXTVOL PUT READ RELEASE REWIND SPACE TRUNCATE UPDATE WRITE")

	var b strings.Builder

	fmt.Fprintf(&b, "\t.PSECT\tCODE,EXE,NOWRT,LONG\n\t.ENTRY\tSERVICES,^M<R6>\n")

	forms := func(svc, kw, blk string) {
		for _, args := range []string{"", kw + "=" + blk, kw + "=R6", kw + "=" + blk + ", ERR=ERRAST",
			kw + "=" + blk + ", SUC=SUCAST", kw + "=" + blk + ", ERR=ERRAST, SUC=SUCAST", kw + "=(R6)"} {
			fmt.Fprintf(&b, "\t$%s\t%s\n", svc, args)
		}
	}

	for _, s := range fab {
		forms(s, "FAB", "FAB1")
	}

	for _, s := range rab {
		forms(s, "RAB", "RBUF")
	}

	for _, args := range []string{"", "OLDFAB=FAB1, NEWFAB=FAB2", "OLDFAB=FAB1, ERR=ERRAST, NEWFAB=FAB2",
		"OLDFAB=FAB1, SUC=SUCAST, NEWFAB=FAB2", "OLDFAB=FAB1, ERR=ERRAST, SUC=SUCAST, NEWFAB=FAB2",
		"OLDFAB=R6, NEWFAB=FAB2"} {
		fmt.Fprintf(&b, "\t$RENAME\t%s\n", args)
	}

	for _, args := range []string{"", "RAB=RBUF", "RAB=R6"} {
		fmt.Fprintf(&b, "\t$WAIT\t%s\n", args)
	}

	fmt.Fprintf(&b, "\tRET\n\t.ENTRY\tERRAST,^M<>\n\tRET\n\t.ENTRY\tSUCAST,^M<>\n\tRET\n%s", labels)
	add("services", false, "every RMS service macro in each form", b.String())
}

// The error probes (O1, O5, O13's global form): assembled into their own
// log, which the author audits before Claude reads it.
func errorProbes() {
	data := "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"

	for name, body := range map[string]string{
		"err_org":        data + "B1:\t$FAB\tORG=BOGUS\n",
		"err_fac":        data + "B1:\t$FAB\tFAC=<GET,BOGUS>\n",
		"err_shr_nql":    data + "B1:\t$FAB\tSHR=NQL\n",
		"err_rop2":       data + "B1:\t$RAB\tROP_2=<NQL,NODLCKWT,NODLCKBLK>\n",
		"err_nop":        data + "B1:\t$NAM\tNOP=NO_SHORT_UPCASE\n",
		"err_keyword":    data + "B1:\t$FAB\tBOGUS=1\n",
		"err_def_global": "\t$FABDEF\tGLOBAL\n" + data + "\t.LONG\tFAB$L_FNA\n",
		"err_ss_global":  "\t$SSDEF\tGLOBAL\n" + data + "\t.LONG\tSS$_NORMAL\n",
		"err_xabdat_edt": data + "B1:\t$XABDAT\tEDT=QUAD\n" + labels,
		"err_store_reg":  "\t.PSECT\tCODE,EXE,NOWRT\n\t.ENTRY\tS,^M<>\n\t$RAB_STORE\tRAB=RBUF, RFA=R12\n\tRET\n" + labels,
	} {
		add(name, true, "an error probe", body)
	}
}

func main() {
	definitionProbes()
	initProbes()
	storeProbes()
	serviceProbes()
	errorProbes()

	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].name < fixtures[j].name })

	var probes, errs, copies []string

	for _, f := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, f.name+".mar"), []byte(f.text), 0o644); err != nil {
			log.Fatal(err)
		}

		up := strings.ToUpper(f.name)
		if f.errors {
			errs = append(errs, up)
		} else {
			probes = append(probes, up)
		}

		copies = append(copies, fmt.Sprintf("COPY \"%s/%s.mar\"/HOST DUA1:[000000]%s.MAR", dir, f.name, up))
	}

	writeCOM("rms.com", "RMS.COM - the Phase 32 oracle's probes (docs/PHASE-32.md, subtask 3).", "RMS", probes)
	writeCOM("rmserr.com", "RMSERR.COM - the Phase 32 oracle's error probes. Their messages go to\n$ ! ERRORS.LOG, which the author audits for macro text before Claude reads it.", "ERRORS", errs)

	exchange := "! EXCHANGE.CMD - builds the Phase 32 oracle's exchange volume with govax\n" +
		"! (docs/PHASE-32.md, subtask 3). Written by testdata/mar/rms/gen.go. Run\n" +
		"! from the repository root:\n!\n!     govax console < testdata/mar/rms/exchange.cmd\n!\n" +
		"INITIALIZE/CONTAINER \"testdata/disks/rms-exchange.dsk\" /DEVICE=RD53 RMSXCHG\n" +
		"MOUNT/WRITE DUA1 \"testdata/disks/rms-exchange.dsk\"\n" +
		strings.Join(copies, "\n") + "\n" +
		"COPY \"" + dir + "/rms.com\"/HOST DUA1:[000000]RMS.COM\n" +
		"COPY \"" + dir + "/rmserr.com\"/HOST DUA1:[000000]RMSERR.COM\n" +
		"DIRECTORY DUA1:[000000]\nDISMOUNT DUA1\n"

	if err := os.WriteFile(filepath.Join(dir, "exchange.cmd"), []byte(exchange), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("gen: %d probes, %d error probes\n", len(probes), len(errs))
}

func writeCOM(file, header, logName string, names []string) {
	var b strings.Builder

	fmt.Fprintf(&b, "$ ! %s\n$ ! Written by testdata/mar/rms/gen.go. Run it with the exchange volume as\n", header)
	fmt.Fprintf(&b, "$ ! the default directory:\n$ !\n$ !     @%s/OUTPUT=%s.LOG\n$ !\n", strings.TrimSuffix(strings.ToUpper(file), ".COM"), logName)
	fmt.Fprintf(&b, "$ ! Each program is assembled with /NOLIST, so no listing of a macro\n$ ! expansion is made, and its object is analyzed.\n$ !\n")
	b.WriteString("$ SET NOON\n$ SET VERIFY\n")

	for _, n := range names {
		fmt.Fprintf(&b, "$ MACRO/NOLIST %s\n$ ANALYZE/OBJECT/OUTPUT=%s.ANL %s.OBJ\n", n, n, n)
	}

	b.WriteString("$ DIRECTORY/SIZE=ALL *.OBJ\n")

	if err := os.WriteFile(filepath.Join(dir, file), []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}
