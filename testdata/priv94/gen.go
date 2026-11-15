//go:build ignore

// gen writes Phase 94's probe of privileges (README.md;
// docs/planning/PHASE-94 - privileges.md, Part D). priv94.mar calls each
// privileged system service case by case, each case a routine of its own
// under a handler, with exactly the privileges the case names enabled,
// from user mode or (through $CMEXEC and $CMKRNL) from executive or kernel
// mode, and prints what the service returned and what it did. The
// procedures run it as SYSTEM, as PRIV94 (authorized for every privilege,
// holding only TMPMBX and NETMBX), and as PRIV94N (authorized for only
// those two), with DCL's side in priv94d.com. Run it from the repository
// root:
//
//	go run testdata/priv94/gen.go
//
// A case's body is the only code that runs with the case's privileges
// and in its mode. Everything around it (setting up, showing what the
// body did, and undoing it) runs in user mode with every privilege the
// process is authorized for, so a case leaves nothing behind whatever
// its privileges. A body that runs in kernel mode must not fault: the
// program's handler can't catch a kernel-mode exception, and VMS would
// crash. So bodies touch only the program's own data, by name.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/priv94"

// disk is the system logical name priv94.com defines for the exchange
// volume.
const disk = "PRIV94$DISK:"

// ---------------------------------------------------------------------
// Privileges, by $PRVDEF's bit numbers (vmsdef's PRV$V_ values, VMS's).

var privBits = map[string]uint{
	"CMKRNL": 0, "CMEXEC": 1, "SYSNAM": 2, "GRPNAM": 3, "ALLSPOOL": 4,
	"DETACH": 5, "DIAGNOSE": 6, "LOG_IO": 7, "GROUP": 8, "ACNT": 9,
	"PRMCEB": 10, "PRMMBX": 11, "PSWAPM": 12, "ALTPRI": 13, "SETPRV": 14,
	"TMPMBX": 15, "WORLD": 16, "MOUNT": 17, "OPER": 18, "EXQUOTA": 19,
	"NETMBX": 20, "VOLPRO": 21, "PHY_IO": 22, "BUGCHK": 23, "PRMGBL": 24,
	"SYSGBL": 25, "PFNMAP": 26, "SHMEM": 27, "SYSPRV": 28, "BYPASS": 29,
	"SYSLCK": 30, "SHARE": 31, "UPGRADE": 32, "DOWNGRADE": 33,
	"GRPPRV": 34, "READALL": 35, "IMPORT": 36, "AUDIT": 37, "SECURITY": 38,
}

func mask(privs []string) uint64 {
	var m uint64

	for _, p := range privs {
		b, ok := privBits[p]
		if !ok {
			log.Fatalf("no privilege %s", p)
		}

		m |= 1 << b
	}

	return m
}

// quad is a quadword mask as two .LONGs.
func quad(m uint64) string {
	return fmt.Sprintf(".LONG\t^X%08X,^X%08X", uint32(m), uint32(m>>32))
}

// ---------------------------------------------------------------------
// Cases.

// kase is one case: a routine of its own (C<n>), run when its part's
// letter is in PRIV94_PART. Its body is a routine too (B<n>), called in
// the case's mode with the case's privileges. Data and code may use "%"
// for the case's label prefix.
type kase struct {
	part  byte
	mode  byte // U, E, or K
	privs []string
	title string

	// needs is the holders the case acts on: "G" (the holder in group
	// 200) or "W" (the one in group 300). Without them the case says so
	// and does nothing.
	needs string

	data, setup, body, show, undo []string

	// keep is a case whose privileges are left as its body set them
	// (part R, the rundown).
	keep bool

	// plain is a case run with the process's own privileges, untouched:
	// parts F, H, and Z, which set the run up and take it down.
	plain bool

	// all is a case whose body runs with every privilege (round 2).
	all bool
}

var cases []*kase

// add makes a case. mode is "U", "E", or "K"; privs are the privileges
// the body runs with, and no others.
func add(part byte, mode string, privs []string, format string, args ...any) *kase {
	k := &kase{part: part, mode: mode[0], privs: privs, title: fmt.Sprintf(format, args...)}
	cases = append(cases, k)

	return k
}

func (k *kase) d(format string, args ...any) { k.data = append(k.data, fmt.Sprintf(format, args...)) }
func (k *kase) s(format string, args ...any) { k.setup = append(k.setup, fmt.Sprintf(format, args...)) }
func (k *kase) b(format string, args ...any) { k.body = append(k.body, fmt.Sprintf(format, args...)) }
func (k *kase) w(format string, args ...any) { k.show = append(k.show, fmt.Sprintf(format, args...)) }
func (k *kase) u(format string, args ...any) { k.undo = append(k.undo, fmt.Sprintf(format, args...)) }

// ascid is a class S descriptor and its text.
func (k *kase) ascid(label, text string) { k.d("%s:\t.ASCID\t/%s/", label, text) }

// The privilege sets cases use.
var (
	none = []string{}
)

func p(names ...string) []string { return names }

// privText is a privilege set as a case's title shows it.
func privText(privs []string) string {
	if len(privs) == 0 {
		return "[]"
	}

	return "[" + strings.Join(privs, ",") + "]"
}

// modeText is a mode letter as a title shows it.
func modeText(m byte) string {
	return map[byte]string{'U': "user", 'E': "exec", 'K': "kernel"}[m]
}

// ---------------------------------------------------------------------
// Names. Every logical name, table, mailbox, cluster, and section a case
// makes is named PRIV94_ and a letter and number of its own, so no two
// cases share one; names lists them for part Z's sweep.

var (
	names   []string
	counter = map[string]int{}
)

func name(kind string) string {
	counter[kind]++
	n := fmt.Sprintf("PRIV94_%s%d", kind, counter[kind])

	if kind == "L" || kind == "T" || kind == "M" {
		names = append(names, n)
	}

	return n
}

// The logical name tables, by their descriptors in the program's data.
var tables = map[string]string{
	"LNM$SYSTEM_TABLE":      "SYSTAB",
	"LNM$GROUP":             "GRPTAB",
	"LNM$JOB":               "JOBTAB",
	"LNM$PROCESS_TABLE":     "PRCTAB",
	"PRIV94_OT":             "OTTAB",
	"PRIV94_PT":             "PTTAB",
	"LNM$SYSTEM_DIRECTORY":  "SYSDIR",
	"LNM$PROCESS_DIRECTORY": "PRCDIR",
	"LNM$FILE_DEV":          "FILEDEV",
	"LNM$TEMPORARY_MAILBOX": "TMPMBXTAB",
	"LNM$PERMANENT_MAILBOX": "PRMMBXTAB",
}

// modeCell is the byte cell holding an access mode, for the services
// that take it by reference.
var modeCell = map[int]string{0: "MODEK", 1: "MODEE", 2: "MODES", 3: "MODEU"}

// ---------------------------------------------------------------------
// The cases themselves.

var modeNames = map[int]string{0: "kernel", 1: "exec", 2: "super", 3: "user"}

// crelnm: $CRELNM of a new name in table at acmode.
func crelnm(part byte, mode string, privs []string, table string, acmode int) {
	n := name("L")
	k := add(part, mode, privs, "$CRELNM %s %s-mode name", table, modeNames[acmode])
	k.ascid("%N", n)
	k.b("$CRELNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s,ITMLST=EQVLST", tables[table], modeCell[acmode])
	k.w("PUSHAQ\t%%N")
	k.w("PUSHAQ\t%s", tables[table])
	k.w("CALLS\t#2,LNMSHOW")
	k.u("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s", tables[table], modeCell[acmode])
}

// dellnm: $DELLNM of a name the setup made in table at acmode.
func dellnm(part byte, mode string, privs []string, table string, acmode int) {
	n := name("L")
	k := add(part, mode, privs, "$DELLNM %s %s-mode name", table, modeNames[acmode])
	k.ascid("%N", n)
	k.s("$CRELNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s,ITMLST=EQVLST", tables[table], modeCell[acmode])
	k.s("SAY\t</  made !XL/>,R0")
	k.b("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s", tables[table], modeCell[acmode])
	k.w("PUSHAQ\t%%N")
	k.w("PUSHAQ\t%s", tables[table])
	k.w("CALLS\t#2,LNMSHOW")
	k.u("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s", tables[table], modeCell[acmode])
}

// crelnt: $CRELNT of a new table under parent, in the caller's mode.
func crelnt(part byte, mode string, privs []string, parent string) {
	n := name("T")
	k := add(part, mode, privs, "$CRELNT under %s", parent)
	k.ascid("%N", n)
	k.b("$CRELNT_S TABNAM=%%N,PARTAB=%s,QUOTA=QUOTA", tables[parent])
	k.w("PUSHAQ\t%%N")
	k.w("PUSHAQ\t%s", tables[parent])
	k.w("CALLS\t#2,LNMSHOW")

	// A table made from an inner mode is that mode's: delete it at its
	// own mode, which every privilege allows from user mode.
	am := map[byte]int{'U': 3, 'E': 1, 'K': 0}[k.mode]
	k.u("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s", tables[parent], modeCell[am])
}

// trnlnm: $TRNLNM of a name the fixtures put in table.
func trnlnm(part byte, privs []string, table, lnm string) {
	k := add(part, "U", privs, "$TRNLNM %s in %s", lnm, table)
	k.ascid("%N", lnm)
	k.b("CLRL\tRES1")
	k.b("$TRNLNM_S TABNAM=%s,LOGNAM=%%N,ITMLST=TRNLST", tables[table])
	k.w("MOVZWL\tTRNLEN,R1")
	k.w("SAY\t</  value !AD/>,R1,#TRNBUF")
}

// crelog: $CRELOG of a new name in table tbl (0 system, 1 group, 2 process).
func crelog(part byte, mode string, privs []string, tbl int) {
	n := name("L")
	table := []string{"LNM$SYSTEM_TABLE", "LNM$GROUP", "LNM$PROCESS_TABLE"}[tbl]
	k := add(part, mode, privs, "$CRELOG table %d (%s)", tbl, table)
	k.ascid("%N", n)
	k.b("$CRELOG_S TBLFLG=#%d,LOGNAM=%%N,EQLNAM=EQVD,ACMODE=#3", tbl)
	k.w("PUSHAQ\t%%N")
	k.w("PUSHAQ\t%s", tables[table])
	k.w("CALLS\t#2,LNMSHOW")
	k.u("$DELLOG_S TBLFLG=#%d,LOGNAM=%%N,ACMODE=#3", tbl)
}

// dellog: $DELLOG of a name the setup made in table tbl.
func dellog(part byte, mode string, privs []string, tbl int) {
	n := name("L")
	table := []string{"LNM$SYSTEM_TABLE", "LNM$GROUP", "LNM$PROCESS_TABLE"}[tbl]
	k := add(part, mode, privs, "$DELLOG table %d (%s)", tbl, table)
	k.ascid("%N", n)
	k.s("$CRELOG_S TBLFLG=#%d,LOGNAM=%%N,EQLNAM=EQVD,ACMODE=#3", tbl)
	k.s("SAY\t</  made !XL/>,R0")
	k.b("$DELLOG_S TBLFLG=#%d,LOGNAM=%%N,ACMODE=#3", tbl)
	k.w("PUSHAQ\t%%N")
	k.w("PUSHAQ\t%s", tables[table])
	k.w("CALLS\t#2,LNMSHOW")
	k.u("$DELLOG_S TBLFLG=#%d,LOGNAM=%%N,ACMODE=#3", tbl)
}

func nameCases() {
	sets := [][]string{none, p("SYSNAM"), p("GRPNAM"), p("SYSPRV")}

	for _, t := range []string{"LNM$SYSTEM_TABLE", "LNM$GROUP", "PRIV94_OT", "PRIV94_PT"} {
		for _, s := range sets {
			crelnm('L', "U", s, t, 3)
		}
	}

	crelnm('L', "U", none, "LNM$JOB", 3)
	crelnm('L', "U", none, "LNM$PROCESS_TABLE", 3)

	// Names at inner modes, asked for from user mode.
	for _, s := range [][]string{none, p("SYSNAM")} {
		crelnm('L', "U", s, "LNM$PROCESS_TABLE", 1)
		crelnm('L', "U", s, "LNM$PROCESS_TABLE", 0)
		crelnm('L', "U", s, "LNM$SYSTEM_TABLE", 1)
	}

	// From inner modes without the privileges (A.3, 1).
	for _, m := range []string{"E", "K"} {
		crelnm('L', m, none, "LNM$SYSTEM_TABLE", 3)
		crelnm('L', m, none, "LNM$GROUP", 3)
		crelnm('L', m, none, "LNM$PROCESS_TABLE", 1)
		crelnm('L', m, none, "LNM$SYSTEM_TABLE", 1)
		crelnm('L', m, none, "PRIV94_PT", 3)
	}

	for _, t := range []string{"LNM$SYSTEM_TABLE", "LNM$GROUP"} {
		for _, s := range sets {
			dellnm('L', "U", s, t, 3)
		}

		dellnm('L', "E", none, t, 3)
	}

	dellnm('L', "U", none, "LNM$PROCESS_TABLE", 1)
	dellnm('L', "U", p("SYSNAM"), "LNM$PROCESS_TABLE", 1)
	dellnm('L', "U", none, "PRIV94_OT", 3)
	dellnm('L', "U", none, "PRIV94_PT", 3)

	for _, s := range [][]string{none, p("SYSPRV"), p("SYSNAM"), p("SYSPRV", "SYSNAM")} {
		crelnt('L', "U", s, "LNM$SYSTEM_DIRECTORY")
	}

	crelnt('L', "E", none, "LNM$SYSTEM_DIRECTORY")
	crelnt('L', "K", none, "LNM$SYSTEM_DIRECTORY")
	crelnt('L', "U", none, "LNM$PROCESS_DIRECTORY")
	crelnt('L', "U", none, "PRIV94_OT")
	crelnt('L', "U", none, "PRIV94_PT")

	for _, s := range [][]string{none, p("SYSPRV"), p("READALL"), p("BYPASS"), p("SYSNAM")} {
		trnlnm('L', s, "PRIV94_PT", "PRIV94_PTN")
	}

	trnlnm('L', none, "PRIV94_OT", "PRIV94_OTN")
	trnlnm('L', none, "LNM$SYSTEM_TABLE", "PRIV94$DISK")

	for tbl, s := range [][][]string{
		{none, p("SYSNAM"), p("GRPNAM")},
		{none, p("GRPNAM"), p("SYSNAM")},
		{none},
	} {
		for _, set := range s {
			crelog('L', "U", set, tbl)
		}
	}

	crelog('L', "K", none, 0)
	dellog('L', "U", none, 0)
	dellog('L', "U", p("SYSNAM"), 0)
	dellog('L', "U", none, 1)
	dellog('L', "U", p("GRPNAM"), 1)
}

// crembx: $CREMBX, temporary or permanent, with a logical name.
func crembx(part byte, mode string, privs []string, perm int) {
	n := name("M")
	k := add(part, mode, privs, "$CREMBX prmflg %d", perm)
	k.ascid("%N", n)
	k.b("$CREMBX_S PRMFLG=#%d,CHAN=CHAN,MAXMSG=#64,BUFQUO=#256,ACMODE=#3,LOGNAM=%%N", perm)
	k.w("PUSHAQ\t%%N")
	k.w("PUSHAQ\tFILEDEV")
	k.w("CALLS\t#2,LNMSHOW")
	k.u("CALLS\t#0,DROPMBX")
}

func mailboxCases() {
	for _, s := range [][]string{none, p("TMPMBX")} {
		crembx('M', "U", s, 0)
	}

	for _, s := range [][]string{none, p("TMPMBX"), p("PRMMBX")} {
		crembx('M', "U", s, 1)
	}

	crembx('M', "E", none, 0)
	crembx('M', "E", none, 1)
	crembx('M', "K", none, 1)

	// $DELMBX of a permanent mailbox the setup made.
	for _, m := range []struct {
		mode  string
		privs []string
	}{{"U", none}, {"U", p("PRMMBX")}, {"E", none}, {"K", none}} {
		n := name("M")
		k := add('M', m.mode, m.privs, "$DELMBX of its own permanent mailbox")
		k.ascid("%N", n)
		k.s("$CREMBX_S PRMFLG=#1,CHAN=CHAN,MAXMSG=#64,BUFQUO=#256,ACMODE=#3,LOGNAM=%%N")
		k.s("SAY\t</  made !XL/>,R0")
		k.b("$DELMBX_S CHAN=CHAN")
		k.w("PUSHAQ\t%%N")
		k.w("PUSHAQ\tFILEDEV")
		k.w("CALLS\t#2,LNMSHOW")
		k.u("CALLS\t#0,DROPMBX")
	}

	// IO$_SETMODE!IO$M_SETPROT on the fixtures' permanent mailbox, which
	// SYSTEM owns, setting the protection it already has (0, all access).
	for _, s := range [][]string{none, p("BYPASS"), p("SYSPRV"), p("GRPPRV"), p("OPER")} {
		k := add('M', "U", s, "IO$_SETMODE!IO$M_SETPROT on SYSTEM's mailbox PRIV94_PMBX")
		k.s("$ASSIGN_S DEVNAM=PMBXNAM,CHAN=CHAN")
		k.s("SAY\t</  assign !XL/>,R0")
		k.b("CLRQ\tIOSB")
		k.b("$QIOW_S CHAN=CHAN,FUNC=#IO$_SETMODE!IO$M_SETPROT,IOSB=IOSB,P2=#0")
		k.w("CALLS\t#0,SHOWIOSB")
		k.u("$DASSGN_S CHAN=CHAN")
		k.u("CLRL\tCHAN")
	}

	k := add('M', "U", none, "IO$_SETMODE!IO$M_SETPROT on its own temporary mailbox")
	k.s("$CREMBX_S CHAN=CHAN,MAXMSG=#64,BUFQUO=#256,ACMODE=#3")
	k.s("SAY\t</  made !XL/>,R0")
	k.b("CLRQ\tIOSB")
	k.b("$QIOW_S CHAN=CHAN,FUNC=#IO$_SETMODE!IO$M_SETPROT,IOSB=IOSB,P2=#^XFF00")
	k.w("CALLS\t#0,SHOWIOSB")
	k.u("CALLS\t#0,DROPMBX")
}

func eventFlagCases() {
	for _, c := range []struct {
		mode  string
		privs []string
		perm  int
	}{
		{"U", none, 0}, {"U", none, 1}, {"U", p("PRMCEB"), 1}, {"E", none, 1}, {"K", none, 1},
	} {
		n := name("E")
		k := add('E', c.mode, c.privs, "$ASCEFC perm %d", c.perm)
		k.ascid("%N", n)
		k.b("$ASCEFC_S EFN=#64,NAME=%%N,PERM=#%d", c.perm)
		k.u("$DLCEFC_S NAME=%%N")
		k.u("SAY\t</  undo dlcefc !XL/>,R0")
		k.u("$DACEFC_S EFN=#64")
	}

	for _, c := range []struct {
		mode  string
		privs []string
	}{{"U", none}, {"U", p("PRMCEB")}, {"E", none}} {
		n := name("E")
		k := add('E', c.mode, c.privs, "$DLCEFC of its own permanent cluster")
		k.ascid("%N", n)
		k.s("$ASCEFC_S EFN=#64,NAME=%%N,PERM=#1")
		k.s("SAY\t</  made !XL/>,R0")
		k.b("$DLCEFC_S NAME=%%N")
		k.u("$DLCEFC_S NAME=%%N")
		k.u("SAY\t</  undo dlcefc !XL/>,R0")
		k.u("$DACEFC_S EFN=#64")
	}
}

func sectionCases() {
	type sec struct {
		mode  string
		privs []string
		flags string
	}

	base := "SEC$M_GBL!SEC$M_PAGFIL!SEC$M_WRT!SEC$M_DZRO!SEC$M_EXPREG"

	for _, c := range []sec{
		{"U", none, ""},
		{"U", none, "SYSGBL"}, {"U", p("SYSGBL"), "SYSGBL"},
		{"U", none, "PERM"}, {"U", p("PRMGBL"), "PERM"},
		{"U", p("SYSGBL"), "SYSGBL!PERM"}, {"U", p("PRMGBL"), "SYSGBL!PERM"},
		{"U", p("SYSGBL", "PRMGBL"), "SYSGBL!PERM"},
		{"E", none, "SYSGBL"}, {"K", none, "SYSGBL"}, {"K", none, "PERM"},
	} {
		n := name("G")
		flags := base
		gbl := "SEC$M_GBL"

		for _, f := range strings.Split(c.flags, "!") {
			if f != "" {
				flags += "!SEC$M_" + f
				if f == "SYSGBL" {
					gbl += "!SEC$M_SYSGBL"
				}
			}
		}

		title := "$CRMPSC global page-file section"
		if c.flags != "" {
			title += " " + c.flags
		}

		k := add('G', c.mode, c.privs, "%s", title)
		k.ascid("%N", n)
		k.b("CLRQ\tRANGE")
		k.b("$CRMPSC_S INADR=INADR,RETADR=RANGE,ACMODE=#3,FLAGS=#%s,GSDNAM=%%N,PAGCNT=#1", flags)
		k.u("CALLS\t#0,DROPRANGE")
		k.u("$DGBLSC_S FLAGS=#%s,GSDNAM=%%N", gbl)
		k.u("SAY\t</  undo dgblsc !XL/>,R0")
	}

	for _, c := range []sec{
		{"U", none, "PERM"}, {"U", p("PRMGBL"), "PERM"},
		{"U", none, "SYSGBL!PERM"}, {"U", p("SYSGBL"), "SYSGBL!PERM"},
		{"U", p("PRMGBL"), "SYSGBL!PERM"}, {"E", none, "SYSGBL!PERM"},
	} {
		n := name("G")
		flags := base + "!SEC$M_PERM"
		gbl := "SEC$M_GBL"

		if strings.Contains(c.flags, "SYSGBL") {
			flags += "!SEC$M_SYSGBL"
			gbl += "!SEC$M_SYSGBL"
		}

		k := add('G', c.mode, c.privs, "$DGBLSC of its own %s section", c.flags)
		k.ascid("%N", n)
		k.s("CLRQ\tRANGE")
		k.s("$CRMPSC_S INADR=INADR,RETADR=RANGE,ACMODE=#3,FLAGS=#%s,GSDNAM=%%N,PAGCNT=#1", flags)
		k.s("SAY\t</  made !XL/>,R0")
		k.s("CALLS\t#0,DROPRANGE")
		k.b("$DGBLSC_S FLAGS=#%s,GSDNAM=%%N", gbl)
		k.u("$DGBLSC_S FLAGS=#%s,GSDNAM=%%N", gbl)
		k.u("SAY\t</  undo dgblsc !XL/>,R0")
	}
}

func lockCases() {
	for _, c := range []struct {
		mode  string
		privs []string
		sys   bool
	}{
		{"U", none, false}, {"U", none, true}, {"U", p("SYSLCK"), true},
		{"E", none, true}, {"K", none, true},
	} {
		flags := "#0"
		title := "$ENQW of a process resource"

		if c.sys {
			flags = "#LCK$M_SYSTEM"
			title = "$ENQW of a system-wide resource"
		}

		k := add('Q', c.mode, c.privs, "%s", title)
		k.b("CLRQ\tLKSB")
		k.b("CLRL\tRES1")
		k.b("$ENQW_S LKMODE=#LCK$K_EXMODE,LKSB=LKSB,FLAGS=%s,RESNAM=RESNAM", flags)
		k.b("BLBC\tR0,9$")
		k.b("MOVL\tR0,R2")
		k.b("$DEQ_S LKID=LKSB+4")
		k.b("MOVL\tR0,RES1")
		k.b("MOVL\tR2,R0")
		k.b("9$:")
		k.w("MOVZWL\tLKSB,R1")
		k.w("SAY\t</  lksb !XL deq !XL/>,R1,RES1")
	}
}

// target is a process a case acts on: G (the holder in group 200), W
// (the holder in group 300), or S (the process itself).
func pidOf(t string) string {
	return map[string]string{"G": "GPID", "W": "WPID", "S": "SELFPID"}[t]
}

func processCases() {
	sets := [][]string{none, p("GROUP"), p("WORLD")}

	for _, t := range []string{"G", "W"} {
		pid := pidOf(t)

		for _, s := range sets {
			k := add('P', "U", s, "$WAKE holder %s", t)
			k.needs = t
			k.b("$WAKE_S PIDADR=%s", pid)

			k = add('P', "U", s, "$SCHDWK holder %s", t)
			k.needs = t
			k.b("$SCHDWK_S PIDADR=%s,DAYTIM=DELTA", pid)
			k.u("$CANWAK_S PIDADR=%s", pid)

			k = add('P', "U", s, "$CANWAK holder %s", t)
			k.needs = t
			k.b("$CANWAK_S PIDADR=%s", pid)

			k = add('P', "U", s, "$SUSPND holder %s", t)
			k.needs = t
			k.b("$SUSPND_S PIDADR=%s", pid)
			k.u("$RESUME_S PIDADR=%s", pid)
			k.u("SAY\t</  undo resume !XL/>,R0")

			k = add('P', "U", s, "$RESUME holder %s (not suspended)", t)
			k.needs = t
			k.b("$RESUME_S PIDADR=%s", pid)

			k = add('P', "U", s, "$SETPRI holder %s to 3 (below its base)", t)
			k.needs = t
			k.b("MNEGL\t#1,RES1")
			k.b("$SETPRI_S PIDADR=%s,PRI=#3,PRVPRI=RES1", pid)
			k.w("SAY\t</  prvpri !SL/>,RES1")
			k.w("PUSHAL\t%s", pid)
			k.w("CALLS\t#1,SHOWPRI")
			k.u("$SETPRI_S PIDADR=%s,PRI=#4", pid)

			k = add('P', "U", s, "$GETJPIW holder %s", t)
			k.needs = t
			k.b("CLRW\tNAMELEN")
			k.b("$GETJPIW_S PIDADR=%s,ITMLST=NAMELST", pid)
			k.w("MOVZWL\tNAMELEN,R1")
			k.w("SAY\t</  prcnam !AD/>,R1,#NAMEBUF")
		}

		k := add('P', "U", p("GROUP", "ALTPRI"), "$SETPRI holder %s to 6 (above its base)", t)
		k.needs = t
		k.b("MNEGL\t#1,RES1")
		k.b("$SETPRI_S PIDADR=%s,PRI=#6,PRVPRI=RES1", pid)
		k.w("SAY\t</  prvpri !SL/>,RES1")
		k.w("PUSHAL\t%s", pid)
		k.w("CALLS\t#1,SHOWPRI")
		k.u("$SETPRI_S PIDADR=%s,PRI=#4", pid)

		k = add('P', "U", p("WORLD"), "$SETPRI holder %s to 6 (above its base)", t)
		k.needs = t
		k.b("MNEGL\t#1,RES1")
		k.b("$SETPRI_S PIDADR=%s,PRI=#6,PRVPRI=RES1", pid)
		k.w("SAY\t</  prvpri !SL/>,RES1")
		k.w("PUSHAL\t%s", pid)
		k.w("CALLS\t#1,SHOWPRI")
		k.u("$SETPRI_S PIDADR=%s,PRI=#4", pid)

		for _, m := range []string{"E", "K"} {
			k = add('P', m, none, "$WAKE holder %s", t)
			k.needs = t
			k.b("$WAKE_S PIDADR=%s", pid)

			k = add('P', m, none, "$SETPRI holder %s to 3", t)
			k.needs = t
			k.b("MNEGL\t#1,RES1")
			k.b("$SETPRI_S PIDADR=%s,PRI=#3,PRVPRI=RES1", pid)
			k.w("SAY\t</  prvpri !SL/>,RES1")
			k.w("PUSHAL\t%s", pid)
			k.w("CALLS\t#1,SHOWPRI")
			k.u("$SETPRI_S PIDADR=%s,PRI=#4", pid)

			k = add('P', m, none, "$SUSPND holder %s", t)
			k.needs = t
			k.b("$SUSPND_S PIDADR=%s", pid)
			k.u("$RESUME_S PIDADR=%s", pid)
			k.u("SAY\t</  undo resume !XL/>,R0")
		}
	}

	// The process itself: raising its own priority.
	for _, s := range [][]string{none, p("ALTPRI")} {
		k := add('P', "U", s, "$SETPRI itself to its base + 2")
		k.b("ADDL3\t#2,BASEPRI,R2")
		k.b("MNEGL\t#1,RES1")
		k.b("$SETPRI_S PRI=R2,PRVPRI=RES1")
		k.w("SAY\t</  prvpri !SL/>,RES1")
		k.w("PUSHAL\tSELFPID")
		k.w("CALLS\t#1,SHOWPRI")
		k.u("$SETPRI_S PRI=BASEPRI")
	}

	k := add('P', "U", none, "$SETPRI itself to its base - 1")
	k.b("SUBL3\t#1,BASEPRI,R2")
	k.b("MNEGL\t#1,RES1")
	k.b("$SETPRI_S PRI=R2,PRVPRI=RES1")
	k.w("SAY\t</  prvpri !SL/>,RES1")
	k.w("PUSHAL\tSELFPID")
	k.w("CALLS\t#1,SHOWPRI")
	k.u("$SETPRI_S PRI=BASEPRI")

	k = add('P', "K", none, "$SETPRI itself to its base + 2")
	k.b("ADDL3\t#2,BASEPRI,R2")
	k.b("MNEGL\t#1,RES1")
	k.b("$SETPRI_S PRI=R2,PRVPRI=RES1")
	k.w("SAY\t</  prvpri !SL/>,RES1")
	k.w("PUSHAL\tSELFPID")
	k.w("CALLS\t#1,SHOWPRI")
	k.u("$SETPRI_S PRI=BASEPRI")

	// A wildcard $GETJPI: which of the processes it can see (A.3, 5).
	for _, s := range sets {
		k := add('P', "U", s, "$GETJPIW wildcard walk")
		k.b("CALLS\t#0,WALK")
		k.w("SAY\t</  self !UL G !UL W !UL nopriv !UL last !XL/>,SEENS,SEENG,SEENW,SEENNP,RES1")
		k.w("SAY\t</  ~ others !UL/>,SEENO")
	}
}

// killCases are part K's: $FORCEX and $DELPRC of the holders, refused
// first, then with the privilege, which ends them (the procedures make
// them again with part H).
func killCases() {
	for _, t := range []string{"G", "W"} {
		pid := pidOf(t)

		k := add('K', "U", none, "$FORCEX holder %s", t)
		k.needs = t
		k.b("$FORCEX_S PIDADR=%s,CODE=#44", pid)

		k = add('K', "U", none, "$DELPRC holder %s", t)
		k.needs = t
		k.b("$DELPRC_S PIDADR=%s", pid)

		k = add('K', "K", none, "$DELPRC holder %s", t)
		k.needs = t
		k.b("$DELPRC_S PIDADR=%s", pid)
		k.w("CALLS\t#0,PAUSE")
		k.w("PUSHAL\t%s", pid)
		k.w("CALLS\t#1,SHOWPRI")
	}

	for _, c := range []struct {
		t, service string
		privs      []string
	}{
		{"W", "FORCEX", p("GROUP")},
		{"G", "FORCEX", p("GROUP")},
		{"W", "DELPRC", p("WORLD")},
		{"G", "DELPRC", p("WORLD")},
	} {
		pid := pidOf(c.t)
		k := add('K', "U", c.privs, "$%s holder %s", c.service, c.t)
		k.needs = c.t

		if c.service == "FORCEX" {
			k.b("$FORCEX_S PIDADR=%s,CODE=#44", pid)
		} else {
			k.b("$DELPRC_S PIDADR=%s", pid)
		}

		k.w("CALLS\t#0,PAUSE")
		k.w("PUSHAL\t%s", pid)
		k.w("CALLS\t#1,SHOWPRI")
	}
}

// creprc: a $CREPRC of the holder image, its new process shown and then
// deleted.
func creprc(mode string, privs []string, title string, args string) {
	n := name("C")
	k := add('C', mode, privs, "$CREPRC %s", title)
	k.ascid("%N", n)
	k.b("CLRL\tCPID")
	k.b("$CREPRC_S PIDADR=CPID,IMAGE=HIMAGE,PRCNAM=%%N%s", args)
	k.w("PUSHAL\tCPID")
	k.w("CALLS\t#1,SHOWCHILD")
	k.u("CALLS\t#0,DROPCHILD")
}

func createCases() {
	for _, s := range [][]string{none, p("TMPMBX", "NETMBX"), p("SETPRV"), p("OPER")} {
		creprc("U", s, "subprocess, no prvadr", ",BASPRI=#4")
	}

	for _, s := range [][]string{none, p("SETPRV")} {
		creprc("U", s, "subprocess, prvadr every privilege", ",PRVADR=ALLQ,BASPRI=#4")
		creprc("U", s, "subprocess, prvadr OPER", ",PRVADR=OPERQ,BASPRI=#4")
	}

	creprc("U", p("OPER"), "subprocess, prvadr OPER", ",PRVADR=OPERQ,BASPRI=#4")

	for _, s := range [][]string{none, p("DETACH"), p("CMKRNL")} {
		creprc("U", s, "detached, UIC [200,1]", ",UIC=#^X00800001,BASPRI=#4")
		creprc("U", s, "detached, UIC [300,1]", ",UIC=#^X00C00001,BASPRI=#4")
	}

	creprc("U", none, "detached, its own UIC", ",UIC=OWNUIC,BASPRI=#4")
	creprc("U", none, "detached flag, no UIC", ",STSFLG=#PRC$M_DETACH,BASPRI=#4")

	for _, s := range [][]string{none, p("PSWAPM")} {
		creprc("U", s, "subprocess, PRC$M_PSWAPM", ",STSFLG=#PRC$M_PSWAPM,BASPRI=#4")
	}

	for _, s := range [][]string{none, p("ACNT")} {
		creprc("U", s, "subprocess, PRC$M_NOACNT", ",STSFLG=#PRC$M_NOACNT,BASPRI=#4")
	}

	for _, s := range [][]string{none, p("ALTPRI")} {
		creprc("U", s, "subprocess, base priority 8", ",BASPRI=#8")
	}

	creprc("E", none, "detached, UIC [300,1]", ",UIC=#^X00C00001,BASPRI=#4")
	creprc("K", none, "detached, UIC [300,1]", ",UIC=#^X00C00001,BASPRI=#4")
	creprc("K", none, "subprocess, prvadr every privilege", ",PRVADR=ALLQ,BASPRI=#4")
}

// setprv: a $SETPRV case. Its body records the masks after the call.
func setprv(part byte, mode string, privs []string, title, args string) *kase {
	k := add(part, mode, privs, "$SETPRV %s", title)
	k.b("MNEGL\t#1,PRVQ")
	k.b("MNEGL\t#1,PRVQ+4")
	k.b("$SETPRV_S %s,PRVPRV=PRVQ", args)
	k.b("MOVL\tR0,R2")
	k.b("CALLS\t#0,MASKS")
	k.b("MOVL\tR2,R0")
	k.w("SAY\t</  prvprv !XL!XL/>,PRVQ+4,PRVQ")
	k.w("CALLS\t#0,SHOWMASKS")
	k.u("CALLS\t#0,RESTOREPROC")

	return k
}

func setprvCases() {
	setprv('S', "U", none, "enable OPER", "ENBFLG=#1,PRVADR=OPERQ")
	setprv('S', "U", none, "enable SETPRV", "ENBFLG=#1,PRVADR=SETPRVQ")
	setprv('S', "U", none, "enable OPER and TMPMBX", "ENBFLG=#1,PRVADR=OPERTMPQ")
	setprv('S', "U", none, "enable every privilege", "ENBFLG=#1,PRVADR=ALLQ")
	setprv('S', "U", none, "enable OPER permanently", "ENBFLG=#1,PRVADR=OPERQ,PRMFLG=#1")
	setprv('S', "U", none, "enable TMPMBX permanently", "ENBFLG=#1,PRVADR=TMPQ,PRMFLG=#1")
	setprv('S', "U", none, "disable TMPMBX permanently", "ENBFLG=#0,PRVADR=TMPQ,PRMFLG=#1")
	setprv('S', "U", none, "disable every privilege permanently", "ENBFLG=#0,PRVADR=ALLQ,PRMFLG=#1")
	setprv('S', "U", none, "no prvadr, enable", "ENBFLG=#1")
	setprv('S', "U", none, "no prvadr, enable permanently", "ENBFLG=#1,PRMFLG=#1")
	setprv('S', "U", none, "enbflg 2", "ENBFLG=#2,PRVADR=OPERQ")
	setprv('S', "U", none, "prmflg 2", "ENBFLG=#1,PRVADR=OPERQ,PRMFLG=#2")
	setprv('S', "U", p("SETPRV"), "enable OPER", "ENBFLG=#1,PRVADR=OPERQ")
	setprv('S', "U", p("SETPRV"), "enable every privilege permanently", "ENBFLG=#1,PRVADR=ALLQ,PRMFLG=#1")
	setprv('S', "E", none, "enable OPER", "ENBFLG=#1,PRVADR=OPERQ")
	setprv('S', "E", none, "enable OPER permanently", "ENBFLG=#1,PRVADR=OPERQ,PRMFLG=#1")
	setprv('S', "K", none, "enable every privilege", "ENBFLG=#1,PRVADR=ALLQ")
}

// rundownCases is part R: privileges an image leaves behind, which the
// procedures look at after it ends (A.3, 10). OPER is enabled
// permanently, ALTPRI for the image only, and neither is undone.
func rundownCases() {
	k := add('R', "U", none, "$SETPRV OPER permanently and ALTPRI for the image, left so")
	k.keep = true
	k.b("$SETPRV_S ENBFLG=#1,PRVADR=OPERQ,PRMFLG=#1")
	k.b("MOVL\tR0,RES1")
	k.b("$SETPRV_S ENBFLG=#1,PRVADR=ALTPRIQ")
	k.b("MOVL\tR0,R2")
	k.b("CALLS\t#0,MASKS")
	k.b("MOVL\tR2,R0")
	k.w("SAY\t</  first !XL/>,RES1")
	k.w("CALLS\t#0,SHOWMASKS")
}

func operatorCases() {
	for _, c := range []struct {
		privs  []string
		target string
		what   string
	}{
		{none, "1", "CENTRAL"},
		{p("OPER"), "1", "CENTRAL"},
		{none, "^X100", "SECURITY"},
		{p("SECURITY"), "^X100", "SECURITY"},
		{p("OPER"), "^X100", "SECURITY"},
	} {
		k := add('O', "U", c.privs, "$SNDOPR request to %s", c.what)
		k.d("%%M:\t.BYTE\t3")
		k.d("\t.BYTE\t<%s>&^XFF,<<%s>@-8>&^XFF,0", c.target, c.target)
		k.d("\t.LONG\t0")
		k.d("\t.ASCII\t/PRIV94 PROBE MESSAGE, PLEASE IGNORE/")
		k.d("%%ML = . - %%M")
		k.d("%%MD:\t.LONG\t%%ML")
		k.d("\t.ADDRESS %%M")
		k.b("$SNDOPR_S MSGBUF=%%MD")
	}

	k := add('O', "K", none, "$SNDOPR request to SECURITY")
	k.d("%%M:\t.BYTE\t3,0,1,0")
	k.d("\t.LONG\t0")
	k.d("\t.ASCII\t/PRIV94 PROBE MESSAGE, PLEASE IGNORE/")
	k.d("%%ML = . - %%M")
	k.d("%%MD:\t.LONG\t%%ML")
	k.d("\t.ADDRESS %%M")
	k.b("$SNDOPR_S MSGBUF=%%MD")

	for _, c := range []struct {
		mode  string
		privs []string
		how   string
		args  string
	}{
		{"U", none, "every terminal", "SNDTYP=#BRK$C_ALLTERMS"},
		{"U", p("OPER"), "every terminal", "SNDTYP=#BRK$C_ALLTERMS"},
		{"U", none, "every user", "SNDTYP=#BRK$C_ALLUSERS"},
		{"U", p("OPER"), "every user", "SNDTYP=#BRK$C_ALLUSERS"},
		{"U", none, "user SYSTEM", "SENDTO=SYSUSER,SNDTYP=#BRK$C_USERNAME"},
		{"U", p("WORLD"), "user SYSTEM", "SENDTO=SYSUSER,SNDTYP=#BRK$C_USERNAME"},
		{"U", p("OPER"), "user SYSTEM", "SENDTO=SYSUSER,SNDTYP=#BRK$C_USERNAME"},
		{"U", none, "its own terminal", "SENDTO=TTNAME,SNDTYP=#BRK$C_DEVICE"},
		{"K", none, "every terminal", "SNDTYP=#BRK$C_ALLTERMS"},
	} {
		k := add('O', c.mode, c.privs, "$BRKTHRUW to %s", c.how)
		k.b("CLRQ\tIOSB")
		k.b("$BRKTHRUW_S MSGBUF=BRKMSG,%s,IOSB=IOSB,TIMOUT=#5", c.args)
		k.w("CALLS\t#0,SHOWIOSB")
	}
}

func pageCases() {
	for _, c := range []struct {
		mode  string
		privs []string
	}{{"U", none}, {"U", p("PSWAPM")}, {"K", none}} {
		k := add('W', c.mode, c.privs, "$LCKPAG of a page of its data")
		k.b("$LCKPAG_S INADR=PAGEADR")
		k.u("$ULKPAG_S INADR=PAGEADR")
		k.u("SAY\t</  undo ulkpag !XL/>,R0")
	}

	for _, s := range [][]string{none, p("PSWAPM")} {
		k := add('W', "U", s, "$ULKPAG of a page it locked")
		k.s("$LCKPAG_S INADR=PAGEADR")
		k.s("SAY\t</  made !XL/>,R0")
		k.b("$ULKPAG_S INADR=PAGEADR")
		k.u("$ULKPAG_S INADR=PAGEADR")
		k.u("SAY\t</  undo ulkpag !XL/>,R0")
	}

	k := add('W', "U", none, "$LKWSET of a page of its data")
	k.b("$LKWSET_S INADR=PAGEADR")
	k.u("$ULWSET_S INADR=PAGEADR")

	// $SETSWM has no _S macro in govax's STARLET, so the call is made by
	// hand: one argument, swpflg, by value.
	for _, c := range []struct {
		mode  string
		privs []string
	}{{"U", none}, {"U", p("PSWAPM")}, {"K", none}} {
		k := add('W', c.mode, c.privs, "$SETSWM 1 (no swapping)")
		k.b("PUSHL\t#1")
		k.b("CALLS\t#1,G^SYS$SETSWM")
		k.u("PUSHL\t#0")
		k.u("CALLS\t#1,G^SYS$SETSWM")
	}
}

func diskCases() {
	for _, c := range []struct {
		mode  string
		privs []string
		fn    string
	}{
		{"U", none, "READLBLK"}, {"U", p("LOG_IO"), "READLBLK"}, {"U", p("PHY_IO"), "READLBLK"},
		{"U", none, "READPBLK"}, {"U", p("LOG_IO"), "READPBLK"}, {"U", p("PHY_IO"), "READPBLK"},
		{"K", none, "READLBLK"},
	} {
		k := add('D', c.mode, c.privs, "IO$_%s block 1 of %s", c.fn, disk)
		k.s("$ASSIGN_S DEVNAM=DISKNAM,CHAN=CHAN")
		k.s("SAY\t</  assign !XL/>,R0")
		k.b("CLRQ\tIOSB")
		k.b("$QIOW_S CHAN=CHAN,FUNC=#IO$_%s,IOSB=IOSB,P1=BUF,P2=#512,P3=#1", c.fn)
		k.w("CALLS\t#0,SHOWIOSB")
		k.u("$DASSGN_S CHAN=CHAN")
		k.u("CLRL\tCHAN")
	}
}

func terminalCases() {
	for _, c := range []struct {
		mode  string
		privs []string
		fn    string
	}{
		{"U", none, "SETMODE"}, {"U", p("LOG_IO"), "SETMODE"},
		{"U", none, "SETCHAR"}, {"U", p("LOG_IO"), "SETCHAR"}, {"U", p("PHY_IO"), "SETCHAR"},
		{"K", none, "SETCHAR"},
	} {
		k := add('T', c.mode, c.privs, "IO$_%s of TT: with its own characteristics", c.fn)
		k.s("$ASSIGN_S DEVNAM=TTNAME,CHAN=CHAN")
		k.s("SAY\t</  assign !XL/>,R0")
		k.s("$QIOW_S CHAN=CHAN,FUNC=#IO$_SENSEMODE,IOSB=IOSB,P1=CHARBUF,P2=#12")
		k.s("MOVZWL\tIOSB,R1")
		k.s("SAY\t</  sense !XL !XW/>,R0,R1")
		k.b("CLRQ\tIOSB")
		k.b("$QIOW_S CHAN=CHAN,FUNC=#IO$_%s,IOSB=IOSB,P1=CHARBUF,P2=#12", c.fn)
		k.w("CALLS\t#0,SHOWIOSB")
		k.u("$DASSGN_S CHAN=CHAN")
		k.u("CLRL\tCHAN")
	}
}

func modeCases() {
	for _, s := range [][]string{none, p("CMKRNL"), p("CMEXEC")} {
		k := add('X', "U", s, "$CMKRNL of a routine that returns 1")
		k.b("$CMKRNL_S ROUTIN=TRIVIAL")

		k = add('X', "U", s, "$CMEXEC of a routine that returns 1")
		k.b("$CMEXEC_S ROUTIN=TRIVIAL")
	}
}

// setupCases are part F (the fixtures: two name tables and a permanent
// mailbox, which SYSTEM's run makes for every identity's), H (the
// holders), and Z (taking it all down).
func setupCases() {
	k := add('F', "U", nil, "fixtures")
	k.plain = true
	k.b("CALLS\t#0,FIXTURES")

	k = add('H', "U", nil, "holders")
	k.plain = true
	k.b("CALLS\t#0,HOLDERS")

	k = add('Z', "U", nil, "the end")
	k.plain = true
	k.b("CALLS\t#0,THEEND")
}

// ---------------------------------------------------------------------
// The program.

// round is what tells the two rounds' programs apart: round 1's is
// PRIV94, round 2's PRIV94B, each with its own default parts and, for
// round 2, routines of its own after the shared ones.
type round struct {
	title, parts string
	extra        string
}

func program(r round) string {
	var b strings.Builder

	head := strings.Replace(programHead, ".TITLE\tPRIV94\t", ".TITLE\t"+r.title+"\t", 1)
	head = strings.Replace(head, "DEFPART: .ASCII\t/LMEGQPCSOWDTX/\nDEFLEN\t= 13",
		fmt.Sprintf("DEFPART: .ASCII\t/%s/\nDEFLEN\t= %d", r.parts, len(r.parts)), 1)
	b.WriteString(head)

	for i, k := range cases {
		n := i + 1
		pre := fmt.Sprintf("C%d_", n)
		title := fmt.Sprintf("== %d %c %s %s %s", n, k.part, modeText(k.mode), privText(k.privs), k.title)

		if k.all {
			title = fmt.Sprintf("== %d %c %s [ALL] %s", n, k.part, modeText(k.mode), k.title)
		}

		if k.plain {
			title = fmt.Sprintf("== %d %c %s", n, k.part, k.title)
		}

		fmt.Fprintf(&b, "\n; %d: %s\n\t.ALIGN\tLONG\n", n, title)
		fmt.Fprintf(&b, "%sXT:\t.ASCID\t\"%s\"\n", pre, title)

		m := mask(k.privs)
		inner := uint64(0)

		switch k.mode {
		case 'E':
			inner = mask(p("CMEXEC"))
		case 'K':
			inner = mask(p("CMKRNL"))
		}

		if k.all {
			m = ^uint64(0)
		}

		fmt.Fprintf(&b, "%sXM:\t%s\n", pre, quad(m|inner))
		fmt.Fprintf(&b, "%sXD:\t%s\n", pre, quad(inner&^m))
		fmt.Fprintf(&b, "%sXA:\t.LONG\t1\n\t.ADDRESS B%d\n", pre, n)

		for _, l := range k.data {
			b.WriteString(strings.ReplaceAll(l, "%", pre) + "\n")
		}
	}

	b.WriteString("\n; The cases and their parts.\n\t.ALIGN\tLONG\nCASES:\n")

	for i, k := range cases {
		fmt.Fprintf(&b, "\t.ADDRESS C%d\n\t.LONG\t^A/%c/\n", i+1, k.part)
	}

	b.WriteString("\t.LONG\t0,0\n")
	b.WriteString(sweep())
	b.WriteString(programCode)
	b.WriteString(r.extra)

	lines := func(list []string, pre string) {
		for _, l := range list {
			l = strings.ReplaceAll(l, "%", pre)
			if strings.HasSuffix(l, ":") {
				b.WriteString(l + "\n")
			} else {
				b.WriteString("\t" + l + "\n")
			}
		}
	}

	for i, k := range cases {
		n := i + 1
		pre := fmt.Sprintf("C%d_", n)

		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, k.title, n)
		fmt.Fprintf(&b, "\tSAY\t</!AS/>,#%sXT\n", pre)

		if k.plain {
			fmt.Fprintf(&b, "\tCALLS\t#0,B%d\n\tPUSHL\tR0\n\tCALLS\t#1,SHOWSTAT\n\tRET\n", n)
			fmt.Fprintf(&b, "\t.ENTRY\tB%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n", n)
			lines(k.body, pre)
			b.WriteString("\tRET\n")

			continue
		}

		if k.needs != "" {
			fmt.Fprintf(&b, "\tTSTL\t%s\n\tBNEQ\t1$\n\tSAY\t</  no holder %s/>\n\tRET\n1$:\n", pidOf(k.needs), k.needs)
		}

		b.WriteString("\tCALLS\t#0,SETALL\n")
		lines(k.setup, pre)
		fmt.Fprintf(&b, "\tPUSHAQ\t%sXM\n\tCALLS\t#1,SETPRIVS\n", pre)

		switch k.mode {
		case 'U':
			fmt.Fprintf(&b, "\tCALLS\t#0,B%d\n", n)
		case 'E':
			fmt.Fprintf(&b, "\tMOVQ\t%sXD,DROPQ\n\tCLRQ\tPREVQ\n\t$CMEXEC_S ROUTIN=INNER,ARGLST=%sXA\n", pre, pre)
		case 'K':
			fmt.Fprintf(&b, "\tMOVQ\t%sXD,DROPQ\n\tCLRQ\tPREVQ\n\t$CMKRNL_S ROUTIN=INNER,ARGLST=%sXA\n", pre, pre)
		}

		b.WriteString("\tMOVL\tR0,STAT\n")

		if k.keep {
			b.WriteString("\tMOVL\t#1,KEEP\n")
		} else {
			b.WriteString("\tCALLS\t#0,SETALL\n")
		}

		b.WriteString("\tPUSHL\tSTAT\n\tCALLS\t#1,SHOWSTAT\n")

		if k.mode != 'U' {
			b.WriteString("\tCALLS\t#0,SHOWINNER\n")
		}

		lines(k.show, pre)
		lines(k.undo, pre)

		if !k.keep {
			b.WriteString("\tCALLS\t#0,RESTORE\n")
		}

		b.WriteString("\tRET\n")

		fmt.Fprintf(&b, "\t.ENTRY\tB%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n", n)
		lines(k.body, pre)
		b.WriteString("\tRET\n")
	}

	b.WriteString("\n\t.END\tSTART\n")

	return b.String()
}

// sweep is part Z's list of every name a case may have left in the
// shareable tables, if a case didn't reach its undo.
func sweep() string {
	var b strings.Builder

	b.WriteString("\n; Every name the cases make, for THEEND's sweep.\n\t.ALIGN\tLONG\nSWEEPN:\n")

	for i := range names {
		fmt.Fprintf(&b, "\t.ADDRESS SW%d\n", i)
	}

	b.WriteString("\t.LONG\t0\n")

	for i, n := range names {
		fmt.Fprintf(&b, "SW%d:\t.ASCID\t/%s/\n", i, n)
	}

	return b.String()
}

const programHead = `	.TITLE	PRIV94	Phase 94's privileges probe
	.IDENT	/V1.0/
;
; Written by testdata/priv94/gen.go: do not edit. Phase 94's probe
; (docs/planning/PHASE-94 - privileges.md, Part D): calls each privileged
; system service case by case, each case a routine of its own under
; HANDLER, its body (B<n>) run with exactly the case's privileges and in
; its mode (user, or exec or kernel through $CMEXEC and $CMKRNL, the
; privilege that gets there taken away again once there), and prints
; what the service returned and what it did. PRIV94_PART names the parts
; to run, a letter each (LMEGQPCSOWDTX if it isn't defined):
;
;	L logical names		M mailboxes		E event flag clusters
;	G global sections	Q locks			P other processes
;	C $CREPRC		S $SETPRV		O $SNDOPR and $BRKTHRU
;	W pages and swapping	D disk I/O		T terminal I/O
;	X $CMKRNL and $CMEXEC
;	F the fixtures		H the holders		Z the end
;	K $FORCEX and $DELPRC of the holders (which ends them)
;	R privileges left for image rundown to undo
;
; PRIV94_ONLY, a case's number, runs that case alone, whatever its part.
; Around each body (setting up, showing, and undoing) the program has
; every privilege the process is authorized for; it starts and ends
; with the ones it was run with.
;
	$SSDEF
	$LNMDEF
	$JPIDEF
	$IODEF
	$LCKDEF
	$SECDEF
	$PRCDEF
	$BRKDEF

; SAY text, a1, ... a10: format text with $FAOL and the arguments (each
; a longword, pushed by value) and write the line.
	.MACRO	SAY	TEXT,A1,A2,A3,A4,A5,A6,A7,A8,A9,A10,?LBL,?OVER
	BRW	OVER
LBL:	.ASCID	TEXT
OVER:
	$$N = 1
	.IRP	ARG,<A10,A9,A8,A7,A6,A5,A4,A3,A2,A1>
	.IF	NOT_BLANK,<ARG>
	PUSHL	ARG
	$$N = $$N + 1
	.ENDC
	.ENDR
	PUSHAQ	LBL
	CALLS	#$$N,SAYR
	.ENDM	SAY

	.PSECT	PAGEDATA,WRT,NOEXE,PAGE
LKPAGE:	.BLKB	512		; the page $LCKPAG and $LKWSET lock
BUF:	.BLKB	512		; a disk block

	.PSECT	DATA,WRT,NOEXE,LONG
SIGCOND: .LONG	0		; the condition HANDLER was given
SIGARG1: .LONG	0		; an access violation's reason mask
SIGARG2: .LONG	0		; and address
STAT:	.LONG	0		; a body's R0
KEEP:	.LONG	0		; a case left its privileges (part R)
RES1:	.LONG	0		; what a body found
CHAN:	.LONG	0		; a case's channel
CPID:	.LONG	0		; a case's new process
TPID:	.LONG	0		; a process SHOWPRI and SHOWCHILD look at
GPID:	.LONG	0		; the holder in group 200
WPID:	.LONG	0		; the holder in group 300
SELFPID: .LONG	0
OWNUIC:	.LONG	0
BASEPRI: .LONG	0
WILD:	.LONG	0		; WALK's context
WPIDV:	.LONG	0		; WALK's process
SEENS:	.LONG	0		; WALK's counts: itself, the holders,
SEENG:	.LONG	0		; the processes it may not see, and the
SEENW:	.LONG	0		; others
SEENNP:	.LONG	0
SEENO:	.LONG	0
IOSB:	.QUAD	0
LKSB:	.QUAD	0
	.BLKL	4
RANGE:	.LONG	0,0
INADR:	.LONG	^X200,^X200
PAGEADR: .ADDRESS LKPAGE,LKPAGE
DELTA:	.LONG	-100000000,-1	; ten seconds
QUOTA:	.LONG	2000
CHARBUF: .BLKB	12

ALLQ:	.LONG	-1,-1
OPERQ:	.LONG	^X40000,0
SETPRVQ: .LONG	^X4000,0
TMPQ:	.LONG	^X8000,0
OPERTMPQ: .LONG	^X48000,0
ALTPRIQ: .LONG	^X2000,0
INITQ:	.QUAD	0		; CURPRIV when the program started
INITPROC: .QUAD	0		; PROCPRIV when the program started
DROPQ:	.QUAD	0		; INNER's privilege to take away
PREVQ:	.QUAD	0		; and the privileges before it did
PRVQ:	.QUAD	0		; a $SETPRV case's prvprv

MODEK:	.BYTE	0
MODEE:	.BYTE	1
MODES:	.BYTE	2
MODEU:	.BYTE	3

SYSTAB:	.ASCID	/LNM$SYSTEM_TABLE/
GRPTAB:	.ASCID	/LNM$GROUP/
JOBTAB:	.ASCID	/LNM$JOB/
PRCTAB:	.ASCID	/LNM$PROCESS_TABLE/
OTTAB:	.ASCID	/PRIV94_OT/
PTTAB:	.ASCID	/PRIV94_PT/
SYSDIR:	.ASCID	/LNM$SYSTEM_DIRECTORY/
PRCDIR:	.ASCID	/LNM$PROCESS_DIRECTORY/
FILEDEV: .ASCID	/LNM$FILE_DEV/
TMPMBXTAB: .ASCID /LNM$TEMPORARY_MAILBOX/
PRMMBXTAB: .ASCID /LNM$PERMANENT_MAILBOX/
G200TAB: .ASCID	/LNM$GROUP_000200/
G001TAB: .ASCID	/LNM$GROUP_000001/
OTN:	.ASCID	/PRIV94_OTN/
PTN:	.ASCID	/PRIV94_PTN/
PMBXNAM: .ASCID	/PRIV94_PMBX/
GPIDNAM: .ASCID	/PRIV94_G_PID/
WPIDNAM: .ASCID	/PRIV94_W_PID/
GNAME:	.ASCID	/PRIV94_G/
WNAME:	.ASCID	/PRIV94_W/
HIMAGE:	.ASCID	/PRIV94$DISK:[000000]PRIV94H.EXE/
DISKNAM: .ASCID	/PRIV94$DISK:/
TTNAME:	.ASCID	/TT:/
SYSUSER: .ASCID	/SYSTEM/
RESNAM:	.ASCID	/PRIV94_RESOURCE/
EQVD:	.ASCID	/PRIV94/
BRKMSG:	.ASCID	/PRIV94 PROBE BROADCAST, PLEASE IGNORE/
PARTNAM: .ASCID	/PRIV94_PART/
ONLYNAM: .ASCID	/PRIV94_ONLY/

	.ALIGN	LONG
EQVLST:	.WORD	6,LNM$_STRING
	.ADDRESS EQVTXT
	.LONG	0,0
EQVTXT:	.ASCII	/PRIV94/

TRNBUF:	.BLKB	64
TRNLEN:	.WORD	0,0
TRNTAB:	.BLKB	32
TABLEN:	.WORD	0,0
TRNMODE: .BYTE	0,0,0,0
	.ALIGN	LONG
TRNLST:	.WORD	64,LNM$_STRING
	.ADDRESS TRNBUF,TRNLEN
	.WORD	1,LNM$_ACMODE
	.ADDRESS TRNMODE,0
	.WORD	32,LNM$_TABLE
	.ADDRESS TRNTAB,TABLEN
	.LONG	0

PARTBUF: .BLKB	32
PARTLEN: .LONG	0
PARTLST: .WORD	32,LNM$_STRING
	.ADDRESS PARTBUF,PARTLEN
	.LONG	0
DEFPART: .ASCII	/LMEGQPCSOWDTX/
DEFLEN	= 13
ONLY:	.LONG	0
PIDBUF:	.BLKB	16
PIDLEN:	.LONG	0
PIDLST:	.WORD	16,LNM$_STRING
	.ADDRESS PIDBUF,PIDLEN
	.LONG	0
PIDTXT:	.BLKB	8
PIDTD:	.LONG	8
	.ADDRESS PIDTXT
PIDEQV:	.WORD	8,LNM$_STRING
	.ADDRESS PIDTXT
	.LONG	0,0

; The masks MASKS fetches: AUTHPRIV, PROCPRIV, CURPRIV, IMAGPRIV.
MASKBUF: .BLKQ	4
MASKLST: .WORD	8,JPI$_AUTHPRIV
	.ADDRESS MASKBUF,0
	.WORD	8,JPI$_PROCPRIV
	.ADDRESS MASKBUF+8,0
	.WORD	8,JPI$_CURPRIV
	.ADDRESS MASKBUF+16,0
	.WORD	8,JPI$_IMAGPRIV
	.ADDRESS MASKBUF+24,0
	.LONG	0

SELFLST: .WORD	4,JPI$_PID
	.ADDRESS SELFPID,0
	.WORD	4,JPI$_UIC
	.ADDRESS OWNUIC,0
	.WORD	4,JPI$_PRIB
	.ADDRESS BASEPRI,0
	.WORD	12,JPI$_USERNAME
	.ADDRESS USERBUF,0
	.LONG	0
USERBUF: .BLKB	12

NAMEBUF: .BLKB	16
NAMELEN: .WORD	0,0
NAMELST: .WORD	16,JPI$_PRCNAM
	.ADDRESS NAMEBUF,NAMELEN
	.LONG	0

PRI:	.LONG	0
PRIB:	.LONG	0
CUIC:	.LONG	0
CMASK:	.BLKQ	3
PRILST:	.WORD	4,JPI$_PRI
	.ADDRESS PRI,0
	.WORD	4,JPI$_PRIB
	.ADDRESS PRIB,0
	.LONG	0
CHILDLST: .WORD	4,JPI$_PRIB
	.ADDRESS PRIB,0
	.WORD	4,JPI$_UIC
	.ADDRESS CUIC,0
	.WORD	8,JPI$_AUTHPRIV
	.ADDRESS CMASK,0
	.WORD	8,JPI$_PROCPRIV
	.ADDRESS CMASK+8,0
	.WORD	8,JPI$_CURPRIV
	.ADDRESS CMASK+16,0
	.LONG	0
WALKLST: .WORD	4,JPI$_PID
	.ADDRESS WPIDV,0
	.LONG	0

OUTD:	.LONG	300
	.ADDRESS OUTBUF
OUTBUF:	.BLKB	300
OUTLEN:	.WORD	0,0
MSGBUF:	.BLKB	256
MSGD:	.LONG	256
	.ADDRESS MSGBUF
MSGLEN:	.WORD	0,0
`

const programCode = `
	.PSECT	CODE,NOWRT,EXE,LONG

	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7,R8,R9>
	$GETJPIW_S ITMLST=SELFLST
	CALLS	#0,MASKS
	MOVQ	MASKBUF+16,INITQ
	MOVQ	MASKBUF+8,INITPROC
	SAY	</user !12AD uic !%U base priority !UL/>,#12,#USERBUF,OWNUIC,BASEPRI
	CALLS	#0,SHOWMASKS
	$TRNLNM_S TABNAM=FILEDEV,LOGNAM=PARTNAM,ITMLST=PARTLST
	BLBS	R0,2$
	MOVC3	#DEFLEN,DEFPART,PARTBUF
	MOVL	#DEFLEN,PARTLEN
2$:	MOVZWL	PARTLEN,PARTLEN
	$TRNLNM_S TABNAM=FILEDEV,LOGNAM=ONLYNAM,ITMLST=PIDLST
	BLBC	R0,3$
	PUSHAL	ONLY
	PUSHAB	PIDBUF
	MOVZWL	PIDLEN,-(SP)
	CALLS	#3,G^LIB$CVT_DTB
	SAY	</case !UL/>,ONLY
	BRB	4$
3$:	SAY	</parts !AD/>,PARTLEN,#PARTBUF
4$:	CALLS	#0,READPIDS
	MOVAB	CASES,R6
	CLRL	R9
10$:	MOVL	(R6)+,R7
	BNEQ	13$
	BRW	90$
13$:
	MOVL	(R6)+,R8
	INCL	R9
	TSTL	ONLY
	BEQL	11$
	CMPL	R9,ONLY
	BNEQ	10$
	BRB	12$
11$:	LOCC	R8,PARTLEN,PARTBUF
	BEQL	10$
12$:	CLRL	SIGCOND
	CALLS	#0,(R7)
	TSTL	SIGCOND
	BEQL	10$
	CALLS	#0,SHOWSIG
	BRB	10$
90$:	TSTL	KEEP
	BNEQ	91$
	CALLS	#0,RESTORE
91$:	CALLS	#0,MASKS
	SAY	</end of cases/>
	CALLS	#0,SHOWMASKS
	MOVL	#SS$_NORMAL,R0
	RET

; SHOWSIG: a condition that reached a case's handler, and the
; privileges the program started with back again.
	.ENTRY	SHOWSIG,^M<>
	SAY	</  signal !XL !XL !XL/>,SIGCOND,SIGARG1,SIGARG2
	TSTL	KEEP
	BNEQ	1$
	CALLS	#0,RESTORE
1$:	RET

; HANDLER records the condition (and an access violation's reason mask
; and address), then unwinds to the caller of the routine that
; established it, which gets the condition in R0. It ignores SS$_UNWIND.
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	CMPL	4(R0),#SS$_UNWIND
	BEQL	9$
	MOVL	4(R0),SIGCOND
	CLRQ	SIGARG1
	CMPL	SIGCOND,#SS$_ACCVIO
	BNEQ	1$
	MOVQ	8(R0),SIGARG1
1$:	MOVL	8(AP),R1
	MOVL	SIGCOND,12(R1)
	$UNWIND_S
9$:	RET

; SAYR text, a1, ...: format and write one line.
	.ENTRY	SAYR,^M<R2,R3,R4,R5>
	MOVL	#300,OUTD
	$FAOL_S	CTRSTR=@4(AP),OUTLEN=OUTLEN,OUTBUF=OUTD,PRMLST=8(AP)
	MOVZWL	OUTLEN,OUTD
	PUSHAQ	OUTD
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#300,OUTD
	RET

; SHOWSTAT status: the status in hex and its message.
	.ENTRY	SHOWSTAT,^M<R2>
	MOVL	#256,MSGD
	$GETMSG_S MSGID=4(AP),MSGLEN=MSGLEN,BUFADR=MSGD,FLAGS=#15
	BLBS	R0,1$
	CLRW	MSGLEN
1$:	MOVZWL	MSGLEN,R2
	SAY	</  R0 !XL !AD/>,4(AP),R2,#MSGBUF
	RET

; SHOWIOSB: an I/O status block's status and second word.
	.ENTRY	SHOWIOSB,^M<R2,R3>
	MOVZWL	IOSB,R2
	MOVZWL	IOSB+2,R3
	SAY	</  iosb !XW !XW !XL/>,R2,R3,IOSB+4
	RET

; MASKS fetches the process's four privilege masks into MASKBUF; it is
; safe in any mode. SHOWMASKS prints them, each as 16 hex digits.
	.ENTRY	MASKS,^M<>
	CLRQ	MASKBUF
	CLRQ	MASKBUF+8
	CLRQ	MASKBUF+16
	CLRQ	MASKBUF+24
	$GETJPIW_S ITMLST=MASKLST
	RET
	.ENTRY	SHOWMASKS,^M<>
	SAY	</  auth !XL!XL proc !XL!XL/>,MASKBUF+4,MASKBUF,MASKBUF+12,MASKBUF+8
	SAY	</  cur  !XL!XL imag !XL!XL/>,MASKBUF+20,MASKBUF+16,MASKBUF+28,MASKBUF+24
	RET

; SETPRIVS mask: the privileges enabled become exactly those of the mask
; (as far as the process may enable them).
	.ENTRY	SETPRIVS,^M<R2>
	$SETPRV_S ENBFLG=#0,PRVADR=ALLQ
	MOVL	4(AP),R2
	$SETPRV_S ENBFLG=#1,PRVADR=(R2)
	CMPL	R0,#SS$_NORMAL
	BEQL	1$
	SAY	</  setprv !XL/>,R0
1$:	RET

; SETALL: every privilege the process is authorized for. RESTORE: the
; privileges the program started with. RESTOREPROC: the permanent ones
; too.
	.ENTRY	SETALL,^M<>
	$SETPRV_S ENBFLG=#1,PRVADR=ALLQ
	RET
	.ENTRY	RESTORE,^M<>
	$SETPRV_S ENBFLG=#0,PRVADR=ALLQ
	$SETPRV_S ENBFLG=#1,PRVADR=INITQ
	RET
	.ENTRY	RESTOREPROC,^M<>
	$SETPRV_S ENBFLG=#1,PRVADR=ALLQ
	$SETPRV_S ENBFLG=#0,PRVADR=ALLQ,PRMFLG=#1
	$SETPRV_S ENBFLG=#1,PRVADR=INITPROC,PRMFLG=#1
	$SETPRV_S ENBFLG=#1,PRVADR=ALLQ
	RET

; INNER body: in exec or kernel mode, take away the privilege that got
; here (DROPQ), keeping what was enabled before in PREVQ, then run the
; body. SHOWINNER prints what the body ran with.
	.ENTRY	INNER,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	$SETPRV_S ENBFLG=#0,PRVADR=DROPQ,PRVPRV=PREVQ
	CALLS	#0,@4(AP)
	RET
	.ENTRY	SHOWINNER,^M<>
	BICL2	DROPQ,PREVQ
	BICL2	DROPQ+4,PREVQ+4
	SAY	</  inner cur !XL!XL/>,PREVQ+4,PREVQ
	RET

; TRIVIAL: a routine for $CMKRNL and $CMEXEC to call.
	.ENTRY	TRIVIAL,^M<>
	MOVL	#SS$_NORMAL,R0
	RET

; LNMSHOW table, name: what the name translates to in the table, its
; mode, and the table it's in.
	.ENTRY	LNMSHOW,^M<R2,R3>
	CLRW	TRNLEN
	CLRW	TABLEN
	CLRB	TRNMODE
	$TRNLNM_S TABNAM=@4(AP),LOGNAM=@8(AP),ITMLST=TRNLST
	BLBS	R0,1$
	SAY	</  lnm !XL/>,R0
	RET
1$:	MOVZWL	TRNLEN,R2
	MOVZWL	TABLEN,R3
	MOVZBL	TRNMODE,R1
	SAY	</  lnm !AD mode !UL in !AD/>,R2,#TRNBUF,R1,R3,#TRNTAB
	RET

; SHOWPRI pid: a process's priority and base priority.
	.ENTRY	SHOWPRI,^M<>
	MOVL	@4(AP),TPID
	MNEGL	#1,PRI
	MNEGL	#1,PRIB
	$GETJPIW_S PIDADR=TPID,ITMLST=PRILST
	BLBS	R0,1$
	SAY	</  jpi !XL/>,R0
	RET
1$:	SAY	</  pri !UL prib !UL/>,PRI,PRIB
	RET

; SHOWCHILD pid: a process a case made: its base priority, UIC (shown
; only as the same as the program's or not), and privileges.
	.ENTRY	SHOWCHILD,^M<R2>
	MOVL	@4(AP),TPID
	BNEQ	1$
	SAY	</  no process/>
	RET
1$:	CLRQ	CMASK
	CLRQ	CMASK+8
	CLRQ	CMASK+16
	$GETJPIW_S PIDADR=TPID,ITMLST=CHILDLST
	BLBS	R0,2$
	SAY	</  jpi !XL/>,R0
	RET
2$:	SAY	</  child prib !UL uic !%U/>,PRIB,CUIC
	SAY	</  child auth !XL!XL proc !XL!XL cur !XL!XL/>,CMASK+4,CMASK,CMASK+12,CMASK+8,CMASK+20,CMASK+16
	RET

; DROPCHILD: delete a case's new process.
	.ENTRY	DROPCHILD,^M<>
	TSTL	CPID
	BEQL	1$
	$DELPRC_S PIDADR=CPID
	CLRL	CPID
1$:	RET

; DROPMBX: delete a case's mailbox and its channel.
	.ENTRY	DROPMBX,^M<>
	TSTL	CHAN
	BEQL	1$
	$DELMBX_S CHAN=CHAN
	$DASSGN_S CHAN=CHAN
	CLRL	CHAN
1$:	RET

; DROPRANGE: delete the pages a section case mapped.
	.ENTRY	DROPRANGE,^M<>
	TSTL	RANGE
	BEQL	1$
	$DELTVA_S INADR=RANGE
	CLRQ	RANGE
1$:	RET

; PAUSE: two seconds, for a process being deleted to go.
	.ENTRY	PAUSE,^M<>
	$SCHDWK_S DAYTIM=TWOSEC
	$HIBER_S
	RET
TWOSEC:	.LONG	-20000000,-1

; WALK: a wildcard $GETJPI over every process, counting what it sees.
	.ENTRY	WALK,^M<R2>
	MNEGL	#1,WILD
	CLRL	SEENS
	CLRL	SEENG
	CLRL	SEENW
	CLRL	SEENNP
	CLRL	SEENO
	MOVZWL	#1000,R2
1$:	$GETJPIW_S PIDADR=WILD,ITMLST=WALKLST
	MOVL	R0,RES1
	CMPL	R0,#SS$_NOMOREPROC
	BEQL	9$
	CMPL	R0,#SS$_NOPRIV
	BNEQ	2$
	INCL	SEENNP
	BRB	8$
2$:	BLBC	R0,8$
	CMPL	WPIDV,SELFPID
	BNEQ	3$
	INCL	SEENS
	BRB	8$
3$:	CMPL	WPIDV,GPID
	BNEQ	4$
	INCL	SEENG
	BRB	8$
4$:	CMPL	WPIDV,WPID
	BNEQ	5$
	INCL	SEENW
	BRB	8$
5$:	INCL	SEENO
8$:	DECL	R2
	BLEQ	9$
	BRW	1$
9$:	MOVL	#SS$_NORMAL,R0
	RET

; READPIDS: the holders' PIDs, from the names HOLDERS defined.
	.ENTRY	READPIDS,^M<>
	CLRL	GPID
	CLRL	WPID
	PUSHAQ	GPIDNAM
	PUSHAL	GPID
	CALLS	#2,READPID
	PUSHAQ	WPIDNAM
	PUSHAL	WPID
	CALLS	#2,READPID
	RET
	.ENTRY	READPID,^M<>
	$TRNLNM_S TABNAM=SYSTAB,LOGNAM=@8(AP),ITMLST=PIDLST
	BLBC	R0,1$
	PUSHL	4(AP)
	PUSHAB	PIDBUF
	MOVZWL	PIDLEN,-(SP)
	CALLS	#3,G^LIB$CVT_HTB
1$:	RET

; FIXTURES (part F, SYSTEM's run): the tables PRIV94_OT (open to all)
; and PRIV94_PT (closed to the group and the world), each with a name in
; it, and the permanent mailbox PRIV94_PMBX, open to all.
	.ENTRY	FIXTURES,^M<>
	$CRELNT_S TABNAM=OTTAB,PARTAB=SYSDIR,QUOTA=QUOTA,PROMSK=OPENPROT
	SAY	</  PRIV94_OT !XL/>,R0
	$CRELNT_S TABNAM=PTTAB,PARTAB=SYSDIR,QUOTA=QUOTA,PROMSK=SHUTPROT
	SAY	</  PRIV94_PT !XL/>,R0
	$CRELNM_S TABNAM=OTTAB,LOGNAM=OTN,ITMLST=EQVLST
	SAY	</  PRIV94_OTN !XL/>,R0
	$CRELNM_S TABNAM=PTTAB,LOGNAM=PTN,ITMLST=EQVLST
	SAY	</  PRIV94_PTN !XL/>,R0
	$CREMBX_S PRMFLG=#1,CHAN=CHAN,MAXMSG=#64,BUFQUO=#256,PROMSK=#0,LOGNAM=PMBXNAM
	SAY	</  PRIV94_PMBX !XL/>,R0
	$DASSGN_S CHAN=CHAN
	CLRL	CHAN
	MOVL	#SS$_NORMAL,R0
	RET
OPENPROT: .WORD	0
SHUTPROT: .WORD	^XFF00

; HOLDERS (part H): the holder processes, PRIV94_G in group 200 and
; PRIV94_W in group 300, each running PRIV94H (which hibernates), and
; their PIDs in the system table as PRIV94_G_PID and PRIV94_W_PID. Any
; holders already there are deleted first.
	.ENTRY	HOLDERS,^M<R2>
	CLRL	R2
	TSTL	GPID
	BEQL	1$
	$DELPRC_S PIDADR=GPID
	INCL	R2
1$:	TSTL	WPID
	BEQL	2$
	$DELPRC_S PIDADR=WPID
	INCL	R2
2$:	TSTL	R2
	BEQL	3$
	CALLS	#0,PAUSE
3$:	$CREPRC_S PIDADR=GPID,IMAGE=HIMAGE,PRCNAM=GNAME,UIC=#^X00800001,BASPRI=#4
	SAY	</  PRIV94_G !XL/>,R0
	$CREPRC_S PIDADR=WPID,IMAGE=HIMAGE,PRCNAM=WNAME,UIC=#^X00C00001,BASPRI=#4
	SAY	</  PRIV94_W !XL/>,R0
	PUSHAQ	GPIDNAM
	PUSHL	GPID
	CALLS	#2,DEFPID
	PUSHAQ	WPIDNAM
	PUSHL	WPID
	CALLS	#2,DEFPID
	MOVL	#SS$_NORMAL,R0
	RET
	.ENTRY	DEFPID,^M<>
	$FAO_S	CTRSTR=PIDFMT,OUTBUF=PIDTD,P1=4(AP)
	$CRELNM_S TABNAM=SYSTAB,LOGNAM=@8(AP),ITMLST=PIDEQV
	RET
PIDFMT:	.ASCID	/!XL/

; THEEND (part Z): the holders, the fixtures, and every name a case may
; have left.
	.ENTRY	THEEND,^M<R2,R3,R4>
	TSTL	GPID
	BEQL	1$
	$DELPRC_S PIDADR=GPID
	SAY	</  PRIV94_G !XL/>,R0
1$:	TSTL	WPID
	BEQL	2$
	$DELPRC_S PIDADR=WPID
	SAY	</  PRIV94_W !XL/>,R0
2$:	$DELLNM_S TABNAM=SYSTAB,LOGNAM=GPIDNAM
	$DELLNM_S TABNAM=SYSTAB,LOGNAM=WPIDNAM
	$ASSIGN_S DEVNAM=PMBXNAM,CHAN=CHAN
	BLBC	R0,3$
	$DELMBX_S CHAN=CHAN
	SAY	</  PRIV94_PMBX !XL/>,R0
	$DASSGN_S CHAN=CHAN
	CLRL	CHAN
3$:	$DELLNM_S TABNAM=SYSDIR,LOGNAM=OTTAB
	SAY	</  PRIV94_OT !XL/>,R0
	$DELLNM_S TABNAM=SYSDIR,LOGNAM=PTTAB
	SAY	</  PRIV94_PT !XL/>,R0
	CLRL	R4
	MOVAB	SWEEPN,R2
10$:	MOVL	(R2)+,R3
	BEQL	19$
	$DELLNM_S TABNAM=SYSTAB,LOGNAM=(R3),ACMODE=MODEK
	BLBC	R0,11$
	INCL	R4
11$:	$DELLNM_S TABNAM=G200TAB,LOGNAM=(R3),ACMODE=MODEK
	BLBC	R0,12$
	INCL	R4
12$:	$DELLNM_S TABNAM=G001TAB,LOGNAM=(R3),ACMODE=MODEK
	BLBC	R0,13$
	INCL	R4
13$:	$DELLNM_S TABNAM=SYSDIR,LOGNAM=(R3),ACMODE=MODEK
	BLBC	R0,14$
	INCL	R4
14$:	BRB	10$
19$:	SAY	</  ~ swept !UL/>,R4
	MOVL	#SS$_NORMAL,R0
	RET
`

// holder is PRIV94H.MAR: a process that hibernates until it is deleted
// or forced to exit.
const holder = `	.TITLE	PRIV94H	Phase 94's holder process
	.IDENT	/V1.0/
;
; Written by testdata/priv94/gen.go: do not edit. The program the probe's
; holder processes run: it hibernates, and goes on hibernating whenever
; it is woken, until it is deleted or forced to exit.
;
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
1$:	$HIBER_S
	BRB	1$
	.END	START
`

// installed is PRIV94I.MAR: an image the procedures install with
// privileges (Decision 9), reporting its masks and using SYSNAM.
const installed = `	.TITLE	PRIV94I	Phase 94's installed image
	.IDENT	/V1.0/
;
; Written by testdata/priv94/gen.go: do not edit. An image the probe runs
; plain and installed with privileges (SYSNAM and OPER): it shows the
; process's four privilege masks, then puts a name in the system table
; and takes it out again, which needs SYSNAM.
;
	$SSDEF
	$JPIDEF
	$LNMDEF

	.PSECT	DATA,WRT,NOEXE,LONG
MASKBUF: .BLKQ	4
MASKLST: .WORD	8,JPI$_AUTHPRIV
	.ADDRESS MASKBUF,0
	.WORD	8,JPI$_PROCPRIV
	.ADDRESS MASKBUF+8,0
	.WORD	8,JPI$_CURPRIV
	.ADDRESS MASKBUF+16,0
	.WORD	8,JPI$_IMAGPRIV
	.ADDRESS MASKBUF+24,0
	.LONG	0
SYSTAB:	.ASCID	/LNM$SYSTEM_TABLE/
LNM:	.ASCID	/PRIV94_INSTALLED/
EQVLST:	.WORD	6,LNM$_STRING
	.ADDRESS EQVTXT
	.LONG	0,0
EQVTXT:	.ASCII	/PRIV94/
FMT1:	.ASCID	/PRIV94I auth !XL!XL proc !XL!XL/
FMT2:	.ASCID	/PRIV94I cur  !XL!XL imag !XL!XL/
FMT3:	.ASCID	/PRIV94I crelnm !XL dellnm !XL/
OUTD:	.LONG	200
	.ADDRESS OUTBUF
OUTBUF:	.BLKB	200
OUTLEN:	.WORD	0,0
CRE:	.LONG	0

	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<R2>
	$GETJPIW_S ITMLST=MASKLST
	$FAO_S	CTRSTR=FMT1,OUTLEN=OUTLEN,OUTBUF=OUTD,-
		P1=MASKBUF+4,P2=MASKBUF,P3=MASKBUF+12,P4=MASKBUF+8
	BSBW	PUT
	$FAO_S	CTRSTR=FMT2,OUTLEN=OUTLEN,OUTBUF=OUTD,-
		P1=MASKBUF+20,P2=MASKBUF+16,P3=MASKBUF+28,P4=MASKBUF+24
	BSBW	PUT
	$CRELNM_S TABNAM=SYSTAB,LOGNAM=LNM,ITMLST=EQVLST
	MOVL	R0,CRE
	$DELLNM_S TABNAM=SYSTAB,LOGNAM=LNM
	$FAO_S	CTRSTR=FMT3,OUTLEN=OUTLEN,OUTBUF=OUTD,P1=CRE,P2=R0
	BSBW	PUT
	MOVL	#SS$_NORMAL,R0
	RET
PUT:	MOVZWL	OUTLEN,OUTD
	PUSHAQ	OUTD
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#200,OUTD
	RSB
	.END	START
`

// ---------------------------------------------------------------------
// The procedures.

const systemProc = `$ ! PRIV94.COM - Phase 94's probe of privileges: SYSTEM's run.
$ ! Written by testdata/priv94/gen.go. Run it as SYSTEM, from the exchange
$ ! volume, mounted /SYSTEM (README.md):
$ !
$ !     @DUA1:[000000]PRIV94
$ !
$ ! It defines PRIV94$DISK in the system table for the volume, makes the
$ ! accounts PRIV94 and PRIV94N, builds the programs, makes the fixtures
$ ! and the holder processes, runs SYSTEM's parts, and installs PRIV94I
$ ! with privileges for the accounts' runs. Its log is
$ ! PRIV94$DISK:[PRIV94LOG]PRIV94.LOG.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ DEFINE/SYSTEM/EXECUTIVE_MODE PRIV94$DISK 'DEV'
$ LOGFILE = DEV + "[PRIV94LOG]PRIV94.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "PRIV94 finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME"), " ", F$GETSYI("NODENAME")
$ SHOW LOGICAL PRIV94$DISK
$ WRITE SYS$OUTPUT F$GETJPI("","USERNAME"), " ", F$GETJPI("","UIC")
$ SHOW PROCESS/PRIVILEGES
$ !
$ WRITE SYS$OUTPUT "######## VOLUME"
$ SET VOLUME/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) PRIV94$DISK:
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PRIV94$DISK:[000000]000000.DIR
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PRIV94$DISK:[000000]PRIV94LOG.DIR
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94$DISK:[000000]*.*;*/EXCLUDE=(*.DIR)
$ DIRECTORY/SECURITY PRIV94$DISK:[000000]
$ !
$ WRITE SYS$OUTPUT "######## ACCOUNTS"
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ RUN SYS$SYSTEM:AUTHORIZE
ADD PRIV94/UIC=[200,3]/PASSWORD=PRIV94PW/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER/DEVICE=PRIV94$DISK:/DIRECTORY=[PRIV94LOG]/PRIVILEGES=ALL/DEFPRIVILEGES=(TMPMBX,NETMBX)/ACCOUNT=PRIV94G/OWNER="PHASE 94 PROBE"
ADD PRIV94N/UIC=[200,4]/PASSWORD=PRIV94PW/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER/DEVICE=PRIV94$DISK:/DIRECTORY=[PRIV94LOG]/PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)/ACCOUNT=PRIV94G/OWNER="PHASE 94 PROBE"
SHOW PRIV94/BRIEF
SHOW PRIV94N/BRIEF
SHOW/IDENTIFIER/FULL PRIV94G
EXIT
$ SET DEFAULT 'HOME'
$ !
$ WRITE SYS$OUTPUT "######## BUILD"
$ SET DEFAULT PRIV94$DISK:[000000]
$ DEFINE/USER_MODE SYS$OUTPUT PRIV94.MLG
$ MACRO/NOLIST PRIV94
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT PRIV94.LLG
$ LINK/NOMAP PRIV94
$ SHOW SYMBOL $STATUS
$ IF F$SEARCH("PRIV94.EXE") .EQS. "" THEN EXIT
$ MACRO/NOLIST PRIV94H
$ LINK/NOMAP PRIV94H
$ MACRO/NOLIST PRIV94I
$ LINK/NOMAP/NOTRACEBACK PRIV94I
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94.EXE
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94H.EXE
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94I.EXE
$ DIRECTORY/SECURITY PRIV94$DISK:[000000]*.EXE
$ !
$ WRITE SYS$OUTPUT "######## FIXTURES AND HOLDERS"
$ DEFINE/USER_MODE PRIV94_PART "FH"
$ RUN PRIV94$DISK:[000000]PRIV94
$ SHOW LOGICAL/TABLE=LNM$SYSTEM_TABLE PRIV94_*
$ SHOW LOGICAL/TABLE=PRIV94_OT *
$ SHOW LOGICAL/TABLE=PRIV94_PT *
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_OT
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_PT
$ !
$ @PRIV94$DISK:[000000]PRIV94P SYSTEM
$ !
$ WRITE SYS$OUTPUT "######## INSTALL"
$ RUN PRIV94$DISK:[000000]PRIV94I
$ INSTALL ADD PRIV94$DISK:[000000]PRIV94I/OPEN/HEADER_RESIDENT/PRIVILEGED=(SYSNAM,OPER)
$ SHOW SYMBOL $STATUS
$ INSTALL LIST/FULL PRIV94$DISK:[000000]PRIV94I
$ RUN PRIV94$DISK:[000000]PRIV94I
$ X = F$SETPRV("NOALL")
$ X = F$SETPRV("TMPMBX,NETMBX")
$ RUN PRIV94$DISK:[000000]PRIV94I
$ SHOW PROCESS/PRIVILEGES
$ SET PROCESS/PRIVILEGES=ALL
$ !
$ WRITE SYS$OUTPUT "PRIV94 FINISHED"
$ SET NOVERIFY
$ EXIT
`

// partsProc is PRIV94P.COM, what every identity runs: the program's
// parts, the rundown, DCL's side, the holders' deletion and remaking,
// and the installed image. P1 is SYSTEM, PRIV94, or PRIV94N.
const partsProc = `$ ! PRIV94P.COM - Phase 94's probe: what each identity runs. P1 is the
$ ! identity: SYSTEM (from PRIV94.COM), PRIV94, or PRIV94N (from
$ ! PRIV94U.COM). Written by testdata/priv94/gen.go.
$ !
$ SET NOON
$ WHO = P1
$ WRITE SYS$OUTPUT "######## PROGRAM ''WHO'"
$ IF WHO .EQS. "PRIV94N" THEN DEFINE/USER_MODE PRIV94_PART "SPX"
$ RUN PRIV94$DISK:[000000]PRIV94
$ !
$ WRITE SYS$OUTPUT "######## RUNDOWN ''WHO'"
$ SET PROCESS/PRIVILEGES=(NOALL,TMPMBX,NETMBX)
$ SHOW PROCESS/PRIVILEGES
$ DEFINE/USER_MODE PRIV94_PART "R"
$ RUN PRIV94$DISK:[000000]PRIV94
$ SHOW PROCESS/PRIVILEGES
$ WRITE SYS$OUTPUT F$GETJPI("","CURPRIV")
$ WRITE SYS$OUTPUT F$GETJPI("","PROCPRIV")
$ DEFINE/USER_MODE PRIV94_PART "-"
$ RUN PRIV94$DISK:[000000]PRIV94
$ IF WHO .EQS. "SYSTEM" THEN SET PROCESS/PRIVILEGES=ALL
$ IF WHO .NES. "SYSTEM" THEN SET PROCESS/PRIVILEGES=(NOALL,TMPMBX,NETMBX)
$ SHOW PROCESS/PRIVILEGES
$ !
$ @PRIV94$DISK:[000000]PRIV94D 'WHO'
$ !
$ IF WHO .EQS. "PRIV94N" THEN GOTO INSTALLED
$ WRITE SYS$OUTPUT "######## KILL ''WHO'"
$ DEFINE/USER_MODE PRIV94_PART "K"
$ RUN PRIV94$DISK:[000000]PRIV94
$ X = F$SETPRV("DETACH,SYSNAM,WORLD")
$ DEFINE/USER_MODE PRIV94_PART "H"
$ RUN PRIV94$DISK:[000000]PRIV94
$ X = F$SETPRV(X)
$ SHOW LOGICAL/SYSTEM PRIV94_*_PID
$ !
$ INSTALLED:
$ IF WHO .EQS. "SYSTEM" THEN EXIT
$ WRITE SYS$OUTPUT "######## INSTALLED ''WHO'"
$ SHOW PROCESS/PRIVILEGES
$ RUN PRIV94$DISK:[000000]PRIV94I
$ SHOW PROCESS/PRIVILEGES
$ INSTALL LIST PRIV94$DISK:[000000]PRIV94I
$ SHOW SYMBOL $STATUS
$ INSTALL ADD PRIV94$DISK:[000000]PRIV94H
$ SHOW SYMBOL $STATUS
$ EXIT
`

// dclProc is PRIV94D.COM, DCL's side, run by every identity under one
// privilege set after another.
func dclProc() string {
	var b strings.Builder

	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	w(`$ ! PRIV94D.COM - Phase 94's probe: DCL's side, for each identity. P1 is
$ ! the identity (SYSTEM, PRIV94, or PRIV94N). Each section runs under one
$ ! privilege set after another; the commands' own messages are what it
$ ! asks VMS for. Written by testdata/priv94/gen.go.
$ !
$ SET NOON
$ WHO = P1
$ GPID = F$TRNLNM("PRIV94_G_PID","LNM$SYSTEM_TABLE")
$ WPID = F$TRNLNM("PRIV94_W_PID","LNM$SYSTEM_TABLE")
$ WRITE SYS$OUTPUT "holders: ''GPID' ''WPID'"
$ GRPTABLE = F$FAO("LNM$GROUP_!6OL", F$GETJPI("","GRP"))
$ TTW = F$GETDVI("TT:","DEVBUFSIZ")`)

	section := func(title string, privs []string, body string) {
		set := "TMPMBX,NETMBX"
		if len(privs) > 0 {
			set += "," + strings.Join(privs, ",")
		}

		w(`$ !
$ WRITE SYS$OUTPUT "######## D %s %s ''WHO'"
$ X = F$SETPRV("NOALL")
$ X = F$SETPRV("%s")
$ WRITE SYS$OUTPUT F$GETJPI("","CURPRIV")`, title, privText(privs), set)
		b.WriteString(body)
	}

	names := `$ DEFINE/SYSTEM PRIV94_DS "S"
$ SHOW LOGICAL/FULL/SYSTEM PRIV94_DS
$ DEASSIGN/SYSTEM PRIV94_DS
$ ASSIGN/SYSTEM "A" PRIV94_DA
$ DEASSIGN/SYSTEM PRIV94_DA
$ DEFINE/GROUP PRIV94_DG "G"
$ SHOW LOGICAL/FULL/GROUP PRIV94_DG
$ DEASSIGN/GROUP PRIV94_DG
$ DEFINE/EXECUTIVE_MODE PRIV94_DE "E"
$ SHOW LOGICAL/FULL/PROCESS PRIV94_DE
$ DEASSIGN/EXECUTIVE_MODE PRIV94_DE
$ DEASSIGN PRIV94_DE
$ DEFINE/SYSTEM/EXECUTIVE_MODE PRIV94_DX "X"
$ SHOW LOGICAL/FULL/SYSTEM PRIV94_DX
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE PRIV94_DX
$ DEASSIGN/SYSTEM PRIV94_DX
$ DEFINE/TABLE=PRIV94_OT PRIV94_DO "O"
$ SHOW LOGICAL/FULL/TABLE=PRIV94_OT PRIV94_DO
$ DEASSIGN/TABLE=PRIV94_OT PRIV94_DO
$ DEFINE/TABLE=PRIV94_PT PRIV94_DP "P"
$ SHOW LOGICAL/FULL/TABLE=PRIV94_PT PRIV94_DP
$ DEASSIGN/TABLE=PRIV94_PT PRIV94_DP
$ SHOW LOGICAL/TABLE=PRIV94_PT *
$ WRITE SYS$OUTPUT F$TRNLNM("PRIV94_PTN","PRIV94_PT")
$ CREATE/NAME_TABLE/PARENT_TABLE=LNM$SYSTEM_DIRECTORY PRIV94_DT
$ SHOW LOGICAL/FULL/TABLE=LNM$SYSTEM_DIRECTORY PRIV94_DT
$ DEASSIGN/TABLE=LNM$SYSTEM_DIRECTORY PRIV94_DT
$ CREATE/NAME_TABLE PRIV94_DQ
$ SHOW LOGICAL/FULL/TABLE=LNM$PROCESS_DIRECTORY PRIV94_DQ
$ DEASSIGN/TABLE=LNM$PROCESS_DIRECTORY PRIV94_DQ
`

	for _, s := range [][]string{none, p("SYSNAM"), p("GRPNAM"), p("SYSPRV")} {
		section("NAMES", s, names)
	}

	security := `$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE LNM$SYSTEM_TABLE
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE LNM$SYSTEM_DIRECTORY
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE 'GRPTABLE'
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_OT
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_PT
`
	section("SECURITY", none, security)
	section("SECURITY", p("SECURITY"), security)

	see := `$ SHOW SYSTEM
$ SHOW PROCESS/IDENTIFICATION='GPID'
$ SHOW PROCESS/IDENTIFICATION='WPID'
$ WRITE SYS$OUTPUT "G: ", F$GETJPI(GPID,"PRCNAM"), " W: ", F$GETJPI(WPID,"PRCNAM")
$ GOSUB WALK
$ SET PROCESS/PRIORITY=3/IDENTIFICATION='GPID'
$ WRITE SYS$OUTPUT F$GETJPI(GPID,"PRI")
$ SET PROCESS/PRIORITY=3/IDENTIFICATION='WPID'
$ WRITE SYS$OUTPUT F$GETJPI(WPID,"PRI")
$ SET PROCESS/SUSPEND/IDENTIFICATION='GPID'
$ SET PROCESS/RESUME/IDENTIFICATION='GPID'
$ SET PROCESS/SUSPEND/IDENTIFICATION='WPID'
$ SET PROCESS/RESUME/IDENTIFICATION='WPID'
`
	for _, s := range [][]string{none, p("GROUP"), p("WORLD")} {
		section("SEE", s, see)
	}

	section("PRIORITY", none, "")
	w(`$ P = F$GETJPI("","PRIB")
$ SET PROCESS/PRIORITY='P+2'
$ WRITE SYS$OUTPUT F$GETJPI("","PRI"), " ", F$GETJPI("","PRIB")
$ SET PROCESS/PRIORITY='P'`)
	section("PRIORITY", p("ALTPRI"), "")
	w(`$ SET PROCESS/PRIORITY='P+2'
$ WRITE SYS$OUTPUT F$GETJPI("","PRI"), " ", F$GETJPI("","PRIB")
$ SET PROCESS/PRIORITY='P'`)

	term := `$ SET TERMINAL/WIDTH='TTW'
$ SET TERMINAL/PERMANENT/WIDTH='TTW'
`
	for _, s := range [][]string{none, p("LOG_IO"), p("PHY_IO")} {
		section("TERMINAL", s, term)
	}

	detach := `$ RUN/DETACH/PROCESS_NAME=PRIV94_DD PRIV94$DISK:[000000]PRIV94H
$ RUN/DETACH/UIC=[300,1]/PROCESS_NAME=PRIV94_DU PRIV94$DISK:[000000]PRIV94H
$ GOSUB STOP_DETACHED
`
	for _, s := range [][]string{none, p("DETACH")} {
		section("DETACH", s, detach)
	}

	section("STOP", none, `$ STOP/IDENTIFICATION='GPID'
$ STOP/IDENTIFICATION='WPID'
$ STOP PRIV94_G
$ WRITE SYS$OUTPUT F$GETJPI(GPID,"PRCNAM")
`)

	w(`$ !
$ X = F$SETPRV("NOALL")
$ IF WHO .EQS. "SYSTEM" THEN SET PROCESS/PRIVILEGES=ALL
$ IF WHO .NES. "SYSTEM" THEN X = F$SETPRV("TMPMBX,NETMBX")
$ SHOW PROCESS/PRIVILEGES
$ WRITE SYS$OUTPUT "######## D END ''WHO'"
$ EXIT
$ !
$ ! STOP_DETACHED: stop the processes the DETACH section made, by name,
$ ! with every privilege the process may have.
$ STOP_DETACHED:
$ Y = F$SETPRV("WORLD,GROUP")
$ CTX = ""
$ M = 0
$ STOP_LOOP:
$ PID = F$PID(CTX)
$ IF PID .EQS. "" THEN GOTO STOP_DONE
$ M = M + 1
$ IF M .GT. 500 THEN GOTO STOP_DONE
$ NAME = F$GETJPI(PID,"PRCNAM")
$ IF NAME .EQS. "PRIV94_DD" .OR. NAME .EQS. "PRIV94_DU" THEN STOP/IDENTIFICATION='PID'
$ IF NAME .EQS. "PRIV94_DD" .OR. NAME .EQS. "PRIV94_DU" THEN WRITE SYS$OUTPUT "stopped ", NAME
$ GOTO STOP_LOOP
$ STOP_DONE:
$ Y = F$SETPRV(Y)
$ RETURN
$ !
$ ! WALK: the processes F$PID shows, the holders and itself by name, the
$ ! rest only counted.
$ WALK:
$ CTX = ""
$ N = 0
$ SEEN = ""
$ ME = F$GETJPI("","PID")
$ WALK_LOOP:
$ PID = F$PID(CTX)
$ IF PID .EQS. "" THEN GOTO WALK_DONE
$ N = N + 1
$ IF PID .EQS. GPID THEN SEEN = SEEN + " G"
$ IF PID .EQS. WPID THEN SEEN = SEEN + " W"
$ IF PID .EQS. ME THEN SEEN = SEEN + " SELF"
$ IF N .LT. 500 THEN GOTO WALK_LOOP
$ WALK_DONE:
$ WRITE SYS$OUTPUT "F$PID saw''SEEN'"
$ WRITE SYS$OUTPUT "  ~ F$PID count ", N
$ RETURN`)

	return b.String()
}

const userProc = `$ ! PRIV94U.COM - Phase 94's probe: an account's run. Run it logged in as
$ ! PRIV94, then as PRIV94N (README.md):
$ !
$ !     @PRIV94$DISK:[000000]PRIV94U
$ !
$ ! Its log is PRIV94$DISK:[PRIV94LOG]PRIV94U_<username>.LOG.
$ ! Written by testdata/priv94/gen.go.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ WHO = F$EDIT(F$GETJPI("","USERNAME"),"TRIM")
$ LOGFILE = "PRIV94$DISK:[PRIV94LOG]PRIV94U_" + WHO + ".LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED 'WHO'
$ WRITE SYS$OUTPUT "PRIV94U finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETJPI("","USERNAME"), " ", F$GETJPI("","UIC")
$ SHOW PROCESS/PRIVILEGES
$ !
$ @PRIV94$DISK:[000000]PRIV94P 'P2'
$ !
$ WRITE SYS$OUTPUT "PRIV94U FINISHED"
$ SET NOVERIFY
$ EXIT
`

const endProc = `$ ! PRIV94Z.COM - Phase 94's probe: the end. Run it as SYSTEM after the
$ ! accounts' runs:
$ !
$ !     @PRIV94$DISK:[000000]PRIV94Z
$ !
$ ! It removes PRIV94I from the known file list, deletes the holders, the
$ ! fixtures, and any name a case left, removes the accounts, and
$ ! deassigns PRIV94$DISK. Its log is PRIV94$DISK:[PRIV94LOG]PRIV94Z.LOG.
$ ! Written by testdata/priv94/gen.go.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ LOGFILE = DEV + "[PRIV94LOG]PRIV94Z.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "PRIV94Z finished; the log is ", LOGFILE
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE PRIV94$DISK
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## INSTALL"
$ INSTALL REMOVE PRIV94$DISK:[000000]PRIV94I
$ INSTALL LIST PRIV94$DISK:[000000]PRIV94I
$ RUN PRIV94$DISK:[000000]PRIV94I
$ !
$ WRITE SYS$OUTPUT "######## THE END"
$ DEFINE/USER_MODE PRIV94_PART "Z"
$ RUN PRIV94$DISK:[000000]PRIV94
$ SHOW LOGICAL/SYSTEM PRIV94*
$ SHOW LOGICAL/TABLE=LNM$SYSTEM_DIRECTORY PRIV94*
$ SHOW LOGICAL/TABLE=LNM$GROUP_000200 PRIV94*
$ SHOW SYSTEM
$ !
$ WRITE SYS$OUTPUT "######## REMOVING THE ACCOUNTS"
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ RUN SYS$SYSTEM:AUTHORIZE
REMOVE PRIV94
REMOVE PRIV94N
REMOVE/IDENTIFIER PRIV94G
EXIT
$ SET DEFAULT 'HOME'
$ DIRECTORY PRIV94$DISK:[PRIV94LOG]
$ WRITE SYS$OUTPUT "PRIV94Z FINISHED"
$ SET NOVERIFY
$ EXIT
`

// ---------------------------------------------------------------------
// Round 2 (README.md's "Round 2"): what round 1 left open. PRIV94B.MAR
// runs on the same machinery as PRIV94.MAR, with cases of its own:
// permanent mailboxes with and without a logical name (M), $ULKPAG (U),
// $CRELNT under shareable tables (N), and which modes $DELLNM deletes (D);
// F and Z make and remove the fixtures, and Z also removes the names
// round 1 left in the system table.

func round2MailboxCases() {
	for _, c := range []struct {
		privs []string
		perm  int
		lnm   bool
	}{
		{p("PRMMBX"), 1, false},
		{p("PRMMBX"), 1, true},
		{p("PRMMBX", "SYSNAM"), 1, true},
		{p("PRMMBX", "SYSPRV"), 1, true},
		{p("PRMMBX", "TMPMBX"), 1, true},
		{p("PRMMBX", "TMPMBX"), 1, false},
		{p("TMPMBX"), 0, true},
		{none, 0, false},
	} {
		title := fmt.Sprintf("$CREMBX prmflg %d", c.perm)
		if c.lnm {
			title += " with a logical name"
		} else {
			title += " with no logical name"
		}

		k := add('M', "U", c.privs, "%s", title)

		if c.lnm {
			k.ascid("%N", name("MB"))
			k.b("$CREMBX_S PRMFLG=#%d,CHAN=CHAN,MAXMSG=#64,BUFQUO=#256,ACMODE=#3,LOGNAM=%%N", c.perm)
			k.w("PUSHAQ\t%%N")
			k.w("PUSHAQ\tFILEDEV")
			k.w("CALLS\t#2,LNMSHOW")
		} else {
			k.b("$CREMBX_S PRMFLG=#%d,CHAN=CHAN,MAXMSG=#64,BUFQUO=#256,ACMODE=#3", c.perm)
		}

		k.u("CALLS\t#0,DROPMBX")
	}
}

func round2UnlockCases() {
	locked := func(k *kase) {
		k.s("$LCKPAG_S INADR=PAGEADR")
		k.s("SAY\t</  made !XL/>,R0")
	}
	undo := func(k *kase) {
		k.w("SAY\t</  retadr !XL !XL/>,RANGE,RANGE+4")
		k.u("$ULKPAG_S INADR=PAGEADR")
		k.u("SAY\t</  undo ulkpag !XL/>,R0")
		k.u("CLRQ\tRANGE")
	}

	k := add('U', "U", none, "$ULKPAG of a page it locked, as round 1")
	locked(k)
	k.b("CLRQ\tRANGE")
	k.b("$ULKPAG_S INADR=PAGEADR")
	undo(k)

	k = add('U', "U", none, "$ULKPAG of a page it locked, with retadr")
	locked(k)
	k.b("CLRQ\tRANGE")
	k.b("$ULKPAG_S INADR=PAGEADR,RETADR=RANGE")
	undo(k)

	k = add('U', "U", p("PSWAPM"), "$ULKPAG of a page it locked, with retadr")
	locked(k)
	k.b("CLRQ\tRANGE")
	k.b("$ULKPAG_S INADR=PAGEADR,RETADR=RANGE")
	undo(k)

	k = add('U', "U", none, "$ULKPAG of a page it locked, acmode user")
	locked(k)
	k.b("CLRQ\tRANGE")
	k.b("$ULKPAG_S INADR=PAGEADR,RETADR=RANGE,ACMODE=#3")
	undo(k)

	k = add('U', "U", none, "$ULKPAG of a page it never locked")
	k.b("CLRQ\tRANGE")
	k.b("$ULKPAG_S INADR=PAGEADR,RETADR=RANGE")
	undo(k)

	k = add('U', "U", p("PSWAPM"), "$LCKPAG then $ULKPAG in the same body")
	k.b("CLRQ\tRANGE")
	k.b("$LCKPAG_S INADR=PAGEADR")
	k.b("MOVL\tR0,RES1")
	k.b("$ULKPAG_S INADR=PAGEADR,RETADR=RANGE")
	k.w("SAY\t</  lckpag !XL/>,RES1")
	undo(k)

	k = add('U', "U", nil, "$ULKPAG of a page it locked")
	k.all = true
	locked(k)
	k.b("CLRQ\tRANGE")
	k.b("$ULKPAG_S INADR=PAGEADR,RETADR=RANGE")
	undo(k)
}

func round2TableCases() {
	type tc struct {
		parent string
		privs  []string
		quota  string // "" for none
	}

	for _, c := range []tc{
		{"PRIV94_OT", none, "100"},
		{"PRIV94_OT", p("SYSPRV"), "100"},
		{"PRIV94_OT", p("SYSNAM"), "100"},
		{"PRIV94_OT", p("GRPNAM"), "100"},
		{"PRIV94_OT", none, "2000"},
		{"PRIV94_OT", none, ""},
		{"LNM$GROUP", none, "100"},
		{"LNM$GROUP", p("GRPNAM"), "100"},
		{"LNM$GROUP", p("SYSPRV"), "100"},
		{"LNM$SYSTEM_TABLE", none, "100"},
		{"LNM$SYSTEM_TABLE", p("SYSNAM"), "100"},
		{"LNM$SYSTEM_TABLE", p("SYSPRV"), "100"},
	} {
		q := "no quota"
		if c.quota != "" {
			q = "quota " + c.quota
		}

		k := add('N', "U", c.privs, "$CRELNT under %s, %s", c.parent, q)
		k.ascid("%N", name("TB"))

		if c.quota != "" {
			k.d("%%Q:\t.LONG\t%s", c.quota)
			k.b("$CRELNT_S TABNAM=%%N,PARTAB=%s,QUOTA=%%Q", tables[c.parent])
		} else {
			k.b("$CRELNT_S TABNAM=%%N,PARTAB=%s", tables[c.parent])
		}

		k.w("PUSHAQ\t%%N")
		k.w("PUSHAQ\t%s", tables[c.parent])
		k.w("CALLS\t#2,LNMSHOW")
		k.u("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=MODEU", tables[c.parent])
	}
}

func round2DeleteCases() {
	type dc struct {
		table string
		make  []int
		del   int // -1: no acmode argument
		privs []string
	}

	for _, c := range []dc{
		{"LNM$PROCESS_TABLE", []int{1}, 3, p("SYSNAM")},
		{"LNM$PROCESS_TABLE", []int{1}, 3, none},
		{"LNM$PROCESS_TABLE", []int{3}, 1, p("SYSNAM")},
		{"LNM$PROCESS_TABLE", []int{3}, 0, p("SYSNAM")},
		{"LNM$PROCESS_TABLE", []int{3, 1}, 3, p("SYSNAM")},
		{"LNM$PROCESS_TABLE", []int{3, 1}, 1, p("SYSNAM")},
		{"LNM$PROCESS_TABLE", []int{3, 1}, 3, none},
		{"LNM$PROCESS_TABLE", []int{1}, -1, p("SYSNAM")},
		{"LNM$PROCESS_TABLE", []int{3, 2, 1, 0}, 2, p("SYSNAM")},
		{"LNM$SYSTEM_TABLE", []int{3}, 1, p("SYSNAM")},
		{"LNM$SYSTEM_TABLE", []int{3, 1}, 3, p("SYSNAM")},
	} {
		var made []string
		for _, m := range c.make {
			made = append(made, modeNames[m])
		}

		del := "no acmode"
		if c.del >= 0 {
			del = "acmode " + modeNames[c.del]
		}

		k := add('D', "U", c.privs, "$DELLNM %s, %s, of names at %s", c.table, del, strings.Join(made, " and "))
		k.ascid("%N", name("LB"))
		tab := tables[c.table]

		for _, m := range c.make {
			k.s("$CRELNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s,ITMLST=EQVLST", tab, modeCell[m])
			k.s("SAY\t</  made %s !XL/>,R0", modeNames[m])
		}

		if c.del >= 0 {
			k.b("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s", tab, modeCell[c.del])
		} else {
			k.b("$DELLNM_S TABNAM=%s,LOGNAM=%%N", tab)
		}

		k.w("PUSHAQ\t%%N")
		k.w("PUSHAQ\t%s", tab)
		k.w("CALLS\t#2,LNMMODES")

		for _, m := range []int{3, 2, 1, 0} {
			k.u("$DELLNM_S TABNAM=%s,LOGNAM=%%N,ACMODE=%s", tab, modeCell[m])
		}
	}
}

func round2SetupCases() {
	k := add('F', "U", nil, "fixtures")
	k.plain = true
	k.b("CALLS\t#0,FIXTURES")

	k = add('Z', "U", nil, "the end")
	k.plain = true
	k.b("CALLS\t#0,THEEND2")
}

// round2Code is PRIV94B's own routines.
const round2Code = `
; LNMMODES table, name: the name's translation asked for at each mode in
; turn (user, supervisor, executive, kernel), with the mode found.
	.ENTRY	LNMMODES,^M<R2,R3,R4,R5>
	MOVAB	MODEU,R4
	MOVL	#4,R5
1$:	CLRW	TRNLEN
	CLRW	TABLEN
	CLRB	TRNMODE
	$TRNLNM_S TABNAM=@4(AP),LOGNAM=@8(AP),ACMODE=(R4),ITMLST=TRNLST
	MOVZBL	(R4),R1
	BLBS	R0,2$
	SAY	</  at !UL: lnm !XL/>,R1,R0
	BRB	3$
2$:	MOVZBL	TRNMODE,R2
	SAY	</  at !UL: mode !UL/>,R1,R2
3$:	DECL	R4
	DECL	R5
	BLEQ	4$
	BRW	1$
4$:	RET

; THEEND2 (part Z of round 2): the fixtures, round 1's three names left in
; the system table, and every name round 2's cases may have left, at
; every mode.
	.ENTRY	THEEND2,^M<R2,R3,R4,R5>
	$ASSIGN_S DEVNAM=PMBXNAM,CHAN=CHAN
	BLBC	R0,1$
	$DELMBX_S CHAN=CHAN
	SAY	</  PRIV94_PMBX !XL/>,R0
	$DASSGN_S CHAN=CHAN
	CLRL	CHAN
1$:	$DELLNM_S TABNAM=SYSDIR,LOGNAM=OTTAB
	SAY	</  PRIV94_OT !XL/>,R0
	$DELLNM_S TABNAM=SYSDIR,LOGNAM=PTTAB
	SAY	</  PRIV94_PT !XL/>,R0
	MOVAB	ROUND1,R2
2$:	MOVL	(R2)+,R3
	BEQL	3$
	$DELLNM_S TABNAM=SYSTAB,LOGNAM=(R3),ACMODE=MODEU
	SAY	</  !AS !XL/>,R3,R0
	BRB	2$
3$:	CLRL	R4
	MOVAB	SWEEPN,R2
10$:	MOVL	(R2)+,R3
	BEQL	19$
	MOVAB	SWEEPT,R5
11$:	TSTL	(R5)
	BEQL	10$
	PUSHL	R3
	PUSHL	(R5)+
	CALLS	#2,DELALL
	ADDL2	R0,R4
	BRB	11$
19$:	SAY	</  ~ swept !UL/>,R4
	MOVL	#SS$_NORMAL,R0
	RET

; DELALL table, name: delete the name at each mode, returning how many
; it deleted.
	.ENTRY	DELALL,^M<R2,R3>
	CLRL	R3
	MOVAB	MODEU,R2
1$:	$DELLNM_S TABNAM=@4(AP),LOGNAM=@8(AP),ACMODE=(R2)
	BLBC	R0,2$
	INCL	R3
2$:	CMPL	R2,#MODEK
	BEQL	3$
	DECL	R2
	BRB	1$
3$:	MOVL	R3,R0
	RET

	.PSECT	DATA,WRT,NOEXE,LONG
	.ALIGN	LONG
ROUND1:	.ADDRESS R1L21,R1L49,R1L51
	.LONG	0
R1L21:	.ASCID	/PRIV94_L21/
R1L49:	.ASCID	/PRIV94_L49/
R1L51:	.ASCID	/PRIV94_L51/
SWEEPT:	.ADDRESS SYSTAB,G200TAB,G001TAB,SYSDIR,OTTAB,PRCTAB
	.LONG	0

	.PSECT	CODE,NOWRT,EXE,LONG
`

// round2 writes PRIV94B.MAR and round 2's procedures.
func round2() {
	cases, names, counter = nil, nil, map[string]int{}

	round2SetupCases()
	round2MailboxCases()
	round2UnlockCases()
	round2TableCases()
	round2DeleteCases()

	write("priv94b.mar", program(round{title: "PRIV94B", parts: "MUND", extra: round2Code}))
	write("priv94r.com", round2System)
	write("priv94v.com", round2User)
	write("priv94e.com", round2DCL)
	write("priv94y.com", round2End)
	write("exchange2.cmd", round2Exchange)
	write("copyout2.cmd", round2Copyout)

	fmt.Printf("round 2: %d cases\n", len(cases))
}

const round2System = `$ ! PRIV94R.COM - Phase 94's probe, round 2: SYSTEM's run.
$ ! Written by testdata/priv94/gen.go. Run it as SYSTEM, from round 1's
$ ! exchange volume with round 2's files added, mounted /SYSTEM
$ ! (README.md's "Round 2"):
$ !
$ !     @DUA1:[000000]PRIV94R
$ !
$ ! It defines PRIV94$DISK again, makes the account PRIV94 again, builds
$ ! PRIV94B, makes the fixtures and two DCL tables, and runs SYSTEM's
$ ! parts. Its log is PRIV94$DISK:[PRIV94LOG]PRIV94R.LOG.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ DEFINE/SYSTEM/EXECUTIVE_MODE PRIV94$DISK 'DEV'
$ LOGFILE = DEV + "[PRIV94LOG]PRIV94R.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "PRIV94R finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("NODENAME")
$ SHOW LOGICAL/SYSTEM PRIV94*
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94$DISK:[000000]PRIV94B.MAR
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94$DISK:[000000]PRIV94V.COM
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94$DISK:[000000]PRIV94E.COM
$ !
$ WRITE SYS$OUTPUT "######## ACCOUNTS"
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ RUN SYS$SYSTEM:AUTHORIZE
ADD PRIV94/UIC=[200,3]/PASSWORD=PRIV94PW/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER/DEVICE=PRIV94$DISK:/DIRECTORY=[PRIV94LOG]/PRIVILEGES=ALL/DEFPRIVILEGES=(TMPMBX,NETMBX)/ACCOUNT=PRIV94G/OWNER="PHASE 94 PROBE"
SHOW PRIV94/BRIEF
EXIT
$ SET DEFAULT 'HOME'
$ !
$ WRITE SYS$OUTPUT "######## BUILD"
$ SET DEFAULT PRIV94$DISK:[000000]
$ DEFINE/USER_MODE SYS$OUTPUT PRIV94B.MLG
$ MACRO/NOLIST PRIV94B
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT PRIV94B.LLG
$ LINK/NOMAP PRIV94B
$ SHOW SYMBOL $STATUS
$ IF F$SEARCH("PRIV94B.EXE") .EQS. "" THEN EXIT
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PRIV94B.EXE
$ !
$ WRITE SYS$OUTPUT "######## FIXTURES"
$ DEFINE/USER_MODE PRIV94_PART "F"
$ RUN PRIV94$DISK:[000000]PRIV94B
$ CREATE/NAME_TABLE/PARENT_TABLE=LNM$SYSTEM_DIRECTORY/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) PRIV94_DO
$ CREATE/NAME_TABLE/PARENT_TABLE=LNM$SYSTEM_DIRECTORY/PROTECTION=(S:RWCD,O:RWCD,G,W) PRIV94_DP
$ DEFINE/TABLE=PRIV94_DP PRIV94_DPN "P"
$ SHOW LOGICAL/FULL/TABLE=LNM$SYSTEM_DIRECTORY PRIV94_D*
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_DO
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_DP
$ !
$ WRITE SYS$OUTPUT "######## PROGRAM SYSTEM"
$ RUN PRIV94$DISK:[000000]PRIV94B
$ !
$ @PRIV94$DISK:[000000]PRIV94E SYSTEM
$ !
$ WRITE SYS$OUTPUT "PRIV94R FINISHED"
$ SET NOVERIFY
$ EXIT
`

const round2User = `$ ! PRIV94V.COM - Phase 94's probe, round 2: PRIV94's run. Run it logged
$ ! in as PRIV94:
$ !
$ !     @PRIV94$DISK:[000000]PRIV94V
$ !
$ ! Its log is PRIV94$DISK:[PRIV94LOG]PRIV94V.LOG.
$ ! Written by testdata/priv94/gen.go.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ LOGFILE = "PRIV94$DISK:[PRIV94LOG]PRIV94V.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "PRIV94V finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETJPI("","USERNAME"), " ", F$GETJPI("","UIC")
$ SHOW PROCESS/PRIVILEGES
$ !
$ WRITE SYS$OUTPUT "######## PROGRAM PRIV94"
$ RUN PRIV94$DISK:[000000]PRIV94B
$ !
$ @PRIV94$DISK:[000000]PRIV94E PRIV94
$ !
$ WRITE SYS$OUTPUT "PRIV94V FINISHED"
$ SET NOVERIFY
$ EXIT
`

// round2DCL is PRIV94E.COM: DCL on tables made at supervisor mode, which
// round 1's user-mode fixtures couldn't show, and SET PROCESS/PRIORITY,
// which round 1's procedure wrote wrongly.
var round2DCL = func() string {
	var b strings.Builder

	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	w(`$ ! PRIV94E.COM - Phase 94's probe, round 2: DCL's side. P1 is the
$ ! identity (SYSTEM or PRIV94). Written by testdata/priv94/gen.go.
$ !
$ SET NOON
$ WHO = P1
$ P = F$GETJPI("","PRIB")
$ HIGHER = P + 2`)

	for _, s := range [][]string{none, p("SYSPRV"), p("SYSNAM"), p("GRPNAM"), p("READALL")} {
		set := "TMPMBX,NETMBX"
		if len(s) > 0 {
			set += "," + strings.Join(s, ",")
		}

		w(`$ !
$ WRITE SYS$OUTPUT "######## E TABLES %s ''WHO'"
$ X = F$SETPRV("NOALL")
$ X = F$SETPRV("%s")
$ WRITE SYS$OUTPUT F$GETJPI("","CURPRIV")
$ DEFINE/TABLE=PRIV94_DO PRIV94_EO "O"
$ SHOW LOGICAL/FULL/TABLE=PRIV94_DO PRIV94_EO
$ DEASSIGN/TABLE=PRIV94_DO PRIV94_EO
$ DEFINE/TABLE=PRIV94_DP PRIV94_EP "P"
$ SHOW LOGICAL/FULL/TABLE=PRIV94_DP PRIV94_EP
$ DEASSIGN/TABLE=PRIV94_DP PRIV94_EP
$ SHOW LOGICAL/TABLE=PRIV94_DP *
$ WRITE SYS$OUTPUT F$TRNLNM("PRIV94_DPN","PRIV94_DP")
$ SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE PRIV94_DP`, privText(s), set)
	}

	for _, s := range [][]string{none, p("ALTPRI")} {
		set := "TMPMBX,NETMBX"
		if len(s) > 0 {
			set += "," + strings.Join(s, ",")
		}

		w(`$ !
$ WRITE SYS$OUTPUT "######## E PRIORITY %s ''WHO'"
$ X = F$SETPRV("NOALL")
$ X = F$SETPRV("%s")
$ WRITE SYS$OUTPUT F$GETJPI("","CURPRIV")
$ SET PROCESS/PRIORITY='HIGHER'
$ WRITE SYS$OUTPUT F$GETJPI("","PRI"), " ", F$GETJPI("","PRIB")
$ SET PROCESS/PRIORITY='P'
$ WRITE SYS$OUTPUT F$GETJPI("","PRI"), " ", F$GETJPI("","PRIB")`, privText(s), set)
	}

	w(`$ !
$ X = F$SETPRV("NOALL")
$ IF WHO .EQS. "SYSTEM" THEN SET PROCESS/PRIVILEGES=ALL
$ IF WHO .NES. "SYSTEM" THEN X = F$SETPRV("TMPMBX,NETMBX")
$ WRITE SYS$OUTPUT "######## E END ''WHO'"
$ EXIT`)

	return b.String()
}()

const round2End = `$ ! PRIV94Y.COM - Phase 94's probe, round 2: the end. Run it as SYSTEM
$ ! after PRIV94's run:
$ !
$ !     @PRIV94$DISK:[000000]PRIV94Y
$ !
$ ! It removes the fixtures, the DCL tables, round 1's three leftover
$ ! names, and every name round 2 may have left, removes the account, and
$ ! deassigns PRIV94$DISK. Its log is PRIV94$DISK:[PRIV94LOG]PRIV94Y.LOG.
$ ! Written by testdata/priv94/gen.go.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ LOGFILE = DEV + "[PRIV94LOG]PRIV94Y.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "PRIV94Y finished; the log is ", LOGFILE
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE PRIV94$DISK
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## THE END"
$ DEFINE/USER_MODE PRIV94_PART "Z"
$ RUN PRIV94$DISK:[000000]PRIV94B
$ DEASSIGN/TABLE=LNM$SYSTEM_DIRECTORY PRIV94_DO
$ DEASSIGN/TABLE=LNM$SYSTEM_DIRECTORY PRIV94_DP
$ SHOW LOGICAL/SYSTEM PRIV94*
$ SHOW LOGICAL/TABLE=LNM$SYSTEM_DIRECTORY PRIV94*
$ SHOW LOGICAL/TABLE=LNM$GROUP_000200 PRIV94*
$ !
$ WRITE SYS$OUTPUT "######## REMOVING THE ACCOUNT"
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ RUN SYS$SYSTEM:AUTHORIZE
REMOVE PRIV94
EXIT
$ SET DEFAULT 'HOME'
$ WRITE SYS$OUTPUT "PRIV94Y FINISHED"
$ SET NOVERIFY
$ EXIT
`

const round2Exchange = `! EXCHANGE2.CMD - adds round 2 of Phase 94's probe to round 1's exchange
! volume. Written by testdata/priv94/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/priv94-exchange.dsk"
COPY "testdata/priv94/priv94b.mar"/HOST DUA5:[000000]PRIV94B.MAR
COPY "testdata/priv94/priv94r.com"/HOST DUA5:[000000]PRIV94R.COM
COPY "testdata/priv94/priv94v.com"/HOST DUA5:[000000]PRIV94V.COM
COPY "testdata/priv94/priv94e.com"/HOST DUA5:[000000]PRIV94E.COM
COPY "testdata/priv94/priv94y.com"/HOST DUA5:[000000]PRIV94Y.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000]PRIV94*.*
DISMOUNT DUA5
`

const round2Copyout = `! COPYOUT2.CMD - copies round 2's results off priv94-exchange.dsk into
! testdata/priv94/vax/round2 (README.md). Written by testdata/priv94/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/copyout2.cmd
!
MOUNT DUA5 "testdata/disks/priv94-exchange.dsk"
COPY DUA5:[PRIV94LOG]PRIV94R.LOG "testdata/priv94/vax/round2/priv94r.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94V.LOG "testdata/priv94/vax/round2/priv94v.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94Y.LOG "testdata/priv94/vax/round2/priv94y.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94B.EXE "testdata/priv94/vax/round2/priv94b.exe"/HOST/QUIET
DISMOUNT DUA5
`

// files are the files the exchange volume holds, by name.
var files = []string{
	"priv94.mar", "priv94h.mar", "priv94i.mar", "priv94.com",
	"priv94p.com", "priv94d.com", "priv94u.com", "priv94z.com",
}

func exchange() string {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds Phase 94's probe exchange volume with govax.
! Written by testdata/priv94/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/priv94-exchange.dsk" /DEVICE=RD53 PRIV94
MOUNT/WRITE DUA5 "testdata/disks/priv94-exchange.dsk"
CREATE/DIRECTORY DUA5:[PRIV94LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
`)

	for _, f := range files {
		fmt.Fprintf(&b, "COPY \"testdata/priv94/%s\"/HOST DUA5:[000000]%s\n", f, strings.ToUpper(f))
	}

	b.WriteString("DIRECTORY/OWNER/PROTECTION DUA5:[000000]\nDISMOUNT DUA5\n")

	return b.String()
}

const copyout = `! COPYOUT.CMD - copies Phase 94's probe results off priv94-exchange.dsk
! into testdata/priv94/vax (README.md). Written by testdata/priv94/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/copyout.cmd
!
! The images come back byte for byte (/BINARY): a later oracle may run
! VMS's own.
MOUNT DUA5 "testdata/disks/priv94-exchange.dsk"
COPY DUA5:[PRIV94LOG]PRIV94.LOG "testdata/priv94/vax/priv94.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94U_PRIV94.LOG "testdata/priv94/vax/priv94u_priv94.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94U_PRIV94N.LOG "testdata/priv94/vax/priv94u_priv94n.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94Z.LOG "testdata/priv94/vax/priv94z.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94.EXE "testdata/priv94/vax/priv94.exe"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94H.EXE "testdata/priv94/vax/priv94h.exe"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94I.EXE "testdata/priv94/vax/priv94i.exe"/HOST/QUIET
DISMOUNT DUA5
`

func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	setupCases()
	nameCases()
	mailboxCases()
	eventFlagCases()
	sectionCases()
	lockCases()
	processCases()
	killCases()
	createCases()
	setprvCases()
	rundownCases()
	operatorCases()
	pageCases()
	diskCases()
	terminalCases()
	modeCases()

	write("priv94.mar", program(round{title: "PRIV94", parts: "LMEGQPCSOWDTX"}))
	write("priv94h.mar", holder)
	write("priv94i.mar", installed)
	write("priv94.com", systemProc)
	write("priv94p.com", partsProc)
	write("priv94d.com", dclProc())
	write("priv94u.com", userProc)
	write("priv94z.com", endProc)
	write("exchange.cmd", exchange())
	write("copyout.cmd", copyout)

	fmt.Printf("%d cases\n", len(cases))

	round2()
}
