//go:build ignore

// gen.go writes the Phase 58 probe of LIBRTL's file routines and the
// routines that call back into the program (docs/PHASE-58, subtask 8):
// lib58.mar, a MACRO program that calls each routine of the phase, case
// by case, and prints what each returned and stored and what it passed
// the program's routines (TestLib58Oracle compares govax's run with
// VMS's, vax/lib58-vax.txt); lib58.com, which builds and runs it on VMS;
// and exchange.cmd, the govax console script that makes the exchange
// volume. Run it from the repository root:
//
//	go run testdata/lib58/gen.go
//
// The program works as Phase 55's (testdata/lib55/gen.go) does: each
// case runs in a routine of its own, under a condition handler (HANDLER)
// that records the condition it's given and unwinds, so the output tells
// a status a routine returned from a condition it signaled. Its line is
// "name R0=xxxxxxxx", then a line per value it shows.
//
// What's new here:
//
//   - The routines the probe gives LIB$FILE_SCAN, LIB$DELETE_FILE, and
//     LIB$RENAME_FILE (FSOK, DFCONF, RFERR, ...) log a line for each call
//     ("  > name argc") and one for each argument, as a shape string says
//     to show it: a descriptor's type, class, and text, a FAB (and its
//     NAM), a longword by reference, or a value. What a routine returns
//     is the next bit of its cell (CONFBITS, ERRBITS, SUCCBITS: 1 is
//     success, 0 the failure FAILST), and a confirm or success routine
//     may do one thing more (CONFACT, SUCCACT): $OPEN and $CLOSE the FAB,
//     signal, call LIB$DELETE_FILE again, or $OPEN and leave it open.
//   - While a case runs, the lines it makes (the routines' and LISTF's)
//     go to a log, printed after the case's R0 line.
//   - LISTF lists the files a wildcard names with LIB$FIND_FILE ("  =
//     name"), to show what a delete or rename left.
//   - The program makes the files it works on, in a tree under [LIB58]
//     on the default device, and expects the volume to be fresh (so
//     versions and the tree's contents are known).
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
	kind  string // "mem" (longwords), "txt" (an S or D string), "vs" (a VS string), "fab" (a FAB and its NAM)
	label string // the value's address (a label; "%" is the case's prefix)
	count int    // mem: how many longwords
	vary  bool   // the value changes from run to run: "~" before its name
}

func mem(label string, n int) show  { return show{"mem", label, n, false} }
func vmem(label string, n int) show { return show{"mem", label, n, true} }
func txt(label string) show         { return show{"txt", label, 0, false} }
func vs(label string) show          { return show{"vs", label, 0, false} }
func fab(label string) show         { return show{"fab", label, 0, false} }

// probeCase is one call of one routine.
type probeCase struct {
	name string   // printed first
	data []string // data lines; "%" is the case's label prefix
	pre  []string // instructions before the call
	call string   // the routine; "" for none (pre's R0 is the case's)
	args []string // its arguments, first first
	post []string // instructions after the call, R0 preserved
	show []show
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

// trim drops the trailing omitted ("0") arguments from args, so the
// argument list ends with the last one given.
func trim(args []string) []string {
	for len(args) > 0 && args[len(args)-1] == "0" {
		args = args[:len(args)-1]
	}

	return args
}

// str is a class S text descriptor for s at label, its text after it in
// .ASCII lines short enough for MACRO.
func str(label, s string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T\n%s_T:", label, len(s), label, label)

	if s == "" {
		return b.String()
	}

	for len(s) > 0 {
		n := min(len(s), 60)
		fmt.Fprintf(&b, "\t.ASCII\t%s\n", delimit(s[:n]))
		s = s[n:]
	}

	return strings.TrimSuffix(b.String(), "\n")
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

// fixed is a class S string of n dashes at label (its text at label_T).
func fixed(label string, n int) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%s_T\n%s_T:\t.ASCII\t\"%s\"",
		label, n, label, label, strings.Repeat("-", n))
}

// varying is a class VS string of at most n bytes at label, CURLEN
// preset to ^X5555 and its text to dashes.
func varying(label string, n int) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t14,11\n\t.ADDRESS\t%s_V\n%s_V:\t.WORD\t^X5555\n\t.ASCII\t\"%s\"",
		label, n, label, label, strings.Repeat("-", n))
}

// fabNam is a FAB at label with its NAM (label_N) and the NAM's buffers,
// for spec and default name dnm ("" for none), FAB$L_CTX ctx.
func fabNam(label, spec, dnm, ctx string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\t.ALIGN\tLONG\n%s:\t$FAB\tNAM=%s_N,CTX=<%s>", label, label, ctx)

	if spec != "" {
		fmt.Fprintf(&b, ",-\n\t\tFNM=<%s>", spec)
	}

	if dnm != "" {
		fmt.Fprintf(&b, ",-\n\t\tDNM=<%s>", dnm)
	}

	fmt.Fprintf(&b, "\n%s_N:\t$NAM\tESA=%s_E,ESS=255,RSA=%s_R,RSS=255\n%s_E:\t.BLKB\t255\n%s_R:\t.BLKB\t255",
		label, label, label, label, label)

	return b.String()
}

// nonzero sets %Z to 1 when the longword at addr isn't 0 (a value's
// presence, which is all the probe may show of an address).
func nonzero(addr string) []string {
	return []string{"CLRL\t%Z", "TSTL\t" + addr, "BEQL\t%NZ", "MOVL\t#1,%Z", "%NZ:"}
}

// The values the probe's routines are given and return.
const (
	userArg = "#^X12345678" // the user-specified-argument
	failSt  = "^X1230"      // what a routine returns for failure (FAILST)
)

// The data every case may use.
var shared = []string{
	"PROTEN:\t.WORD\t^XFFFF\t\t; LIB$CREATE_DIR's protection: every bit given,",
	"PROTVAL:\t.WORD\t0\t\t; and every access allowed (so a directory can go)",
	"STV:\t.LONG\t0\t\t; LIB$FIND_FILE's status-value",
	"ZERO:\t.LONG\t0",
	"ONE:\t.LONG\t1",
	"TWO:\t.LONG\t2",
	"FOUR:\t.LONG\t4",
	"EIGHT:\t.LONG\t8",
	"JUNKCELL:\t.ADDRESS\tJUNK\t; a context longword that names no context",
	"JUNK:\t.BLKB\t256",
	// The context cells sequences of cases share.
	"CTXA:\t.LONG\t0", "CTXB:\t.LONG\t0", "CTXC:\t.LONG\t0", "CTXD:\t.LONG\t0",
	"CTXE:\t.LONG\t0", "CTXF:\t.LONG\t0", "CTXG:\t.LONG\t0", "CTXH:\t.LONG\t0",
	"CTXM:\t.LONG\t0", "CTXN:\t.LONG\t0", "CTXS:\t.LONG\t0", "CTXT:\t.LONG\t0",
	"CTXU:\t.LONG\t0", "CTXR:\t.LONG\t0", "CTXX:\t.LONG\t0",
	// The FABs sequences of FILE_SCAN cases share.
	fabNam("SFA", "[LIB58.B]ONE.TXT", "", "^X0000FA01"),
	fabNam("SFB", "FOUR.TXT", "", "^X0000FA02"),
	fabNam("SFC", "[LIB58.B]ONE.TXT", "", "^X0000FA03"),
	fabNam("SFD", "FOUR.TXT", "", "^X0000FA04"),
	fabNam("SFS", "[LIB58.A]*.TXT", "", "^X0000FA05"),
	fabNam("SFE", "[LIB58...]*.TXT", "", "^X0000FA06"),
	fabNam("SFF", "[LIB58.A]ONE.TXT", "", "^X0000FA07"),
	// The file IDs the FID_TO_NAME cases use: three words each.
	"FIDDEEP:\t.WORD\t0,0,0", "DIDDEEP:\t.WORD\t0,0,0",
	"FIDONE:\t.WORD\t0,0,0", "FIDDIR:\t.WORD\t0,0,0",
	"LOSTFID:\t.WORD\t0,0,0", "LOSTDID:\t.WORD\t0,0,0",
	"GONEFID:\t.WORD\t0,0,0",
	"ACPCHAN:\t.LONG\t0",
	"ACPIOSB:\t.QUAD\t0",
	"ACPFIBD:\t.LONG\t22\t\t; the FIB's descriptor",
	"\t.ADDRESS\tACPFIB",
	"ACPFIB:\t.BLKB\t64",
	str("ORPHNAM", "ORPHAN.TXT;1"),
	str("NESTSPEC", "[LIB58.DEL]N2.TXT"),
	str("LISTNAME", "SYS$DISK:[LIB58.A]"),
	str("LISTNAM2", "SYS$DISK:[LIB58.B]"),
	str("LISTLOG", "LIB58_LIST"),
	str("PROCTAB", "LNM$PROCESS_TABLE"),
	"\t.ALIGN\tLONG",
	"LISTITM:\t.WORD\t18,LNM$_STRING",
	"\t.ADDRESS\tLISTNAME_T",
	"\t.LONG\t0",
	"\t.WORD\t18,LNM$_STRING",
	"\t.ADDRESS\tLISTNAM2_T",
	"\t.LONG\t0",
	"\t.LONG\t0",
}

// The cases, in the order they run.
var cases = []probeCase{
	// The tree the FIND_FILE and FILE_SCAN cases search.
	mkdirs("make directories", "[LIB58]", "[LIB58.A]", "[LIB58.A.SUB]", "[LIB58.B]"),
	mkfiles("make A and B files", "[LIB58.A]ONE.TXT", "[LIB58.A]TWO.TXT", "[LIB58.A]TWO.TXT",
		"[LIB58.A]THREE.DAT", "[LIB58.A.SUB]DEEP.TXT", "[LIB58.B]ONE.TXT", "[LIB58.B]FOUR.TXT"),
	{name: "define LIB58_LIST", data: []string{"%S:\t.LONG\t0"},
		pre:  []string{"$CRELNM_S\tTABNAM=PROCTAB,LOGNAM=LISTLOG,ITMLST=LISTITM", "MOVL\tR0,%S"},
		show: []show{mem("%S", 1)}},

	// FIND_FILE (subtask 3): a wildcard, one file at a time, past the end.
	find("FF A*.TXT 1", "CTXA", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF A*.TXT 2", "CTXA", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF A*.TXT 3", "CTXA", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF A*.TXT 4", "CTXA", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF A*.TXT 5", "CTXA", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF A*.TXT 6", "CTXA", ffArgs{spec: "[LIB58.A]*.TXT"}),
	findEnd("FFE A*.TXT", "CTXA"),
	findEnd("FFE A*.TXT again", "CTXA"),
	// No wildcard, without and with NOWILD.
	find("FF ONE.TXT 1", "CTXB", ffArgs{spec: "[LIB58.A]ONE.TXT"}),
	find("FF ONE.TXT 2", "CTXB", ffArgs{spec: "[LIB58.A]ONE.TXT"}),
	find("FF ONE.TXT 3", "CTXB", ffArgs{spec: "[LIB58.A]ONE.TXT"}),
	findEnd("FFE ONE.TXT", "CTXB"),
	find("FF NOWILD ONE.TXT 1", "CTXC", ffArgs{spec: "[LIB58.A]ONE.TXT", flags: "ONE"}),
	find("FF NOWILD ONE.TXT 2", "CTXC", ffArgs{spec: "[LIB58.A]ONE.TXT", flags: "ONE"}),
	find("FF NOWILD ONE.TXT 3", "CTXC", ffArgs{spec: "[LIB58.A]ONE.TXT", flags: "ONE"}),
	find("FF NOWILD *.TXT", "CTXC", ffArgs{spec: "[LIB58.A]*.TXT", flags: "ONE"}),
	find("FF NOWILD *.TXT again", "CTXC", ffArgs{spec: "[LIB58.A]*.TXT", flags: "ONE"}),
	find("FF NOWILD wild directory", "CTXC", ffArgs{spec: "[LIB58...]ONE.TXT", flags: "ONE"}),
	findEnd("FFE NOWILD", "CTXC"),
	// A wildcard directory.
	find("FF [LIB58...]*.TXT 1", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 2", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 3", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 4", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 5", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 6", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 7", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	find("FF [LIB58...]*.TXT 8", "CTXD", ffArgs{spec: "[LIB58...]*.TXT"}),
	findEnd("FFE [LIB58...]", "CTXD"),
	// A search list.
	find("FF LIB58_LIST:*.TXT 1", "CTXE", ffArgs{spec: "LIB58_LIST:*.TXT"}),
	find("FF LIB58_LIST:*.TXT 2", "CTXE", ffArgs{spec: "LIB58_LIST:*.TXT"}),
	find("FF LIB58_LIST:*.TXT 3", "CTXE", ffArgs{spec: "LIB58_LIST:*.TXT"}),
	find("FF LIB58_LIST:*.TXT 4", "CTXE", ffArgs{spec: "LIB58_LIST:*.TXT"}),
	find("FF LIB58_LIST:*.TXT 5", "CTXE", ffArgs{spec: "LIB58_LIST:*.TXT"}),
	find("FF LIB58_LIST:*.TXT 6", "CTXE", ffArgs{spec: "LIB58_LIST:*.TXT"}),
	find("FF LIB58_LIST:ONE.TXT 1", "CTXE", ffArgs{spec: "LIB58_LIST:ONE.TXT"}),
	find("FF LIB58_LIST:ONE.TXT 2", "CTXE", ffArgs{spec: "LIB58_LIST:ONE.TXT"}),
	find("FF LIB58_LIST:ONE.TXT 3", "CTXE", ffArgs{spec: "LIB58_LIST:ONE.TXT"}),
	findEnd("FFE LIB58_LIST", "CTXE"),
	// A different filespec on the same context, part way.
	find("FF new spec A", "CTXF", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF new spec B 1", "CTXF", ffArgs{spec: "[LIB58.B]*.TXT"}),
	find("FF new spec B 2", "CTXF", ffArgs{spec: "[LIB58.B]*.TXT"}),
	find("FF new spec A again", "CTXF", ffArgs{spec: "[LIB58.A]*.TXT"}),
	find("FF new spec lowercase", "CTXF", ffArgs{spec: "[lib58.a]*.txt"}),
	findEnd("FFE new spec", "CTXF"),
	// Missing files and directories, a bad name, and status-value.
	find("FF missing 1", "CTXG", ffArgs{spec: "[LIB58.A]NONE.TXT"}),
	find("FF missing 2", "CTXG", ffArgs{spec: "[LIB58.A]NONE.TXT"}),
	find("FF missing wildcard", "CTXG", ffArgs{spec: "[LIB58.A]NONE*.TXT"}),
	find("FF missing directory", "CTXG", ffArgs{spec: "[LIB58.NODIR]X.TXT"}),
	find("FF missing wild directory", "CTXG", ffArgs{spec: "[LIB58.NODIR...]X.TXT"}),
	find("FF bad syntax", "CTXG", ffArgs{spec: "[LIB58.A]X.TXT;Q"}),
	find("FF no such device", "CTXG", ffArgs{spec: "XYZ0:[LIB58]X.TXT"}),
	find("FF empty spec", "CTXG", ffArgs{spec: ""}),
	findEnd("FFE missing", "CTXG"),
	// Default and related names.
	find("FF default name", "CTXH", ffArgs{spec: "ONE", def: "[LIB58.A].TXT"}),
	find("FF same spec, related name", "CTXH", ffArgs{spec: "ONE", rel: "[LIB58.B]FOUR.TXT"}),
	findEnd("FFE default", "CTXH"),
	find("FF related name", "CTXH", ffArgs{spec: "ONE", rel: "[LIB58.B]FOUR.TXT"}),
	find("FF related name 2", "CTXH", ffArgs{spec: "ONE", rel: "[LIB58.B]FOUR.TXT"}),
	findEnd("FFE related", "CTXH"),
	find("FF related wildcard", "CTXH", ffArgs{spec: "*", rel: "[LIB58.B]FOUR.TXT"}),
	find("FF related wildcard 2", "CTXH", ffArgs{spec: "*", rel: "[LIB58.B]FOUR.TXT"}),
	findEnd("FFE related wildcard", "CTXH"),
	find("FF related wildcard type", "CTXH", ffArgs{spec: "ONE.*", rel: "[LIB58.B]FOUR.TXT"}),
	findEnd("FFE related wildcard type", "CTXH"),
	find("FF related and default", "CTXH", ffArgs{spec: "ONE", def: ".DAT", rel: "[LIB58.B]FOUR.TXT"}),
	findEnd("FFE related and default", "CTXH"),
	find("FF default and spec type", "CTXH", ffArgs{spec: "[LIB58.A]THREE", def: "[LIB58.B].DAT"}),
	findEnd("FFE default and spec type", "CTXH"),
	// MULTIPLE: the previous call's name as the related name.
	find("FF MULTIPLE B ONE", "CTXM", ffArgs{spec: "[LIB58.B]ONE.TXT", flags: "TWO"}),
	find("FF MULTIPLE FOUR", "CTXM", ffArgs{spec: "FOUR", flags: "TWO"}),
	find("FF MULTIPLE FOUR again", "CTXM", ffArgs{spec: "FOUR", flags: "TWO"}),
	find("FF MULTIPLE [LIB58.A]", "CTXM", ffArgs{spec: "[LIB58.A]", flags: "TWO"}),
	find("FF MULTIPLE TWO", "CTXM", ffArgs{spec: "TWO", flags: "TWO"}),
	find("FF MULTIPLE TWO again", "CTXM", ffArgs{spec: "TWO", flags: "TWO"}),
	find("FF MULTIPLE *", "CTXM", ffArgs{spec: "*", flags: "TWO"}),
	findEnd("FFE MULTIPLE", "CTXM"),
	find("FF MULTIPLE related 1", "CTXN", ffArgs{spec: "ONE", rel: "[LIB58.B]X.TXT", flags: "TWO"}),
	find("FF MULTIPLE related 2", "CTXN", ffArgs{spec: "FOUR", rel: "[LIB58.A]X.DAT", flags: "TWO"}),
	findEnd("FFE MULTIPLE related", "CTXN"),
	find("FF no MULTIPLE 1", "CTXN", ffArgs{spec: "[LIB58.B]ONE.TXT"}),
	find("FF no MULTIPLE 2", "CTXN", ffArgs{spec: "FOUR"}),
	findEnd("FFE no MULTIPLE", "CTXN"),
	// The resultant's class and length, and the name's length.
	find("FF into S 12", "CTXT", ffArgs{spec: "[LIB58.A]ONE.TXT", res: fixed("%D", 12)}),
	findEnd("FFE into S 12", "CTXT"),
	find("FF into S 40", "CTXT", ffArgs{spec: "[LIB58.A]ONE.TXT", res: fixed("%D", 40)}),
	findEnd("FFE into S 40", "CTXT"),
	find("FF into VS 30", "CTXT", ffArgs{spec: "[LIB58.A]ONE.TXT", res: varying("%D", 30), vs: true}),
	findEnd("FFE into VS 30", "CTXT"),
	find("FF 255 characters", "CTXT", ffArgs{spec: "[LIB58.A]" + strings.Repeat("X", 246)}),
	findEnd("FFE 255", "CTXT"),
	find("FF 256 characters", "CTXT", ffArgs{spec: "[LIB58.A]" + strings.Repeat("X", 247)}),
	findEnd("FFE 256", "CTXT"),
	find("FF 40-character name", "CTXT", ffArgs{spec: "[LIB58.A]" + strings.Repeat("X", 40) + ".TXT"}),
	findEnd("FFE 40", "CTXT"),
	// Flag bits past MULTIPLE.
	find("FF flags 4", "CTXT", ffArgs{spec: "[LIB58.A]ONE.TXT", flags: "FOUR"}),
	findEnd("FFE flags 4", "CTXT"),
	find("FF flags 8", "CTXT", ffArgs{spec: "[LIB58.A]ONE.TXT", flags: "EIGHT"}),
	findEnd("FFE flags 8", "CTXT"),
	// The context's checks and the argument list.
	{name: "FFE of 0", data: []string{"%C:\t.LONG\t0"}, call: "LIB$FIND_FILE_END", args: []string{"%C"}, show: []show{mem("%C", 1)}},
	{name: "FF context omitted", data: []string{str("%S", "[LIB58.A]ONE.TXT"), dyn("%D")},
		call: "LIB$FIND_FILE", args: []string{"%S", "%D", "0"}, show: []show{txt("%D")}},
	{name: "FF 3 arguments", data: []string{str("%S", "[LIB58.A]ONE.TXT"), dyn("%D")},
		call: "LIB$FIND_FILE", args: []string{"%S", "%D", "CTXX"}, show: []show{txt("%D")}},
	findEnd("FFE 3 arguments", "CTXX"),
	{name: "FF 8 arguments", data: []string{str("%S", "[LIB58.A]ONE.TXT"), dyn("%D")},
		call: "LIB$FIND_FILE", args: []string{"%S", "%D", "CTXX", "0", "0", "0", "0", "0"}, show: []show{txt("%D")}},
	findEnd("FFE 8 arguments", "CTXX"),
	{name: "FFE no arguments", call: "LIB$FIND_FILE_END"},

	// FILE_SCAN (subtask 4).
	scan("FS A*.TXT", "[LIB58.A]*.TXT", "", "FSOK", "FSERR", ""),
	scan("FS [LIB58...]*.TXT", "[LIB58...]*.TXT", "", "FSOK", "FSERR", ""),
	scan("FS missing file", "[LIB58.A]NONE.TXT", "", "FSOK", "FSERR", ""),
	scan("FS missing wildcard", "[LIB58.A]NONE*.TXT", "", "FSOK", "FSERR", ""),
	scan("FS missing directory", "[LIB58.NODIR]*.TXT", "", "FSOK", "FSERR", ""),
	scan("FS missing wild directory", "[LIB58.NODIR...]*.TXT", "", "FSOK", "FSERR", ""),
	scan("FS search list", "LIB58_LIST:ONE.TXT", "", "FSOK", "FSERR", ""),
	scan("FS default name", "ONE", "[LIB58.B].TXT", "FSOK", "FSERR", ""),
	scan("FS bad default name", "ONE", "[[[", "FSOK", "FSERR", ""),
	scan("FS success 0", "[LIB58.A]*.TXT", "", "0", "FSERR", ""),
	scan("FS error 0", "[LIB58.A]NONE.TXT", "", "FSOK", "0", ""),
	withPre(scan("FS success fails", "[LIB58.A]*.TXT", "", "FSOK", "FSERR", ""), "MOVL\t#^X55555554,SUCCBITS"),
	withPre(scan("FS error fails", "[LIB58...]NONE.TXT", "", "FSOK", "FSERR", ""), "CLRL\tERRBITS"),
	{name: "FS FAB without NAM", data: []string{"\t.ALIGN\tLONG", "%F:\t$FAB\tFNM=<[LIB58.A]ONE.TXT>,CTX=<^X0000FA00>"},
		call: "LIB$FILE_SCAN", args: []string{"%F", "FSOK", "FSERR"}, show: []show{fab("%F")}},
	{name: "FS 5 arguments", call: "LIB$FILE_SCAN", args: []string{"SFF", "FSOK", "FSERR", "0", "0"}, show: []show{fab("SFF")}},
	// Defaulting between calls, with a context and without.
	{name: "FS context 1", call: "LIB$FILE_SCAN", args: []string{"SFA", "FSOK", "FSERR", "CTXS"},
		data: []string{"%Z:\t.LONG\t0"}, post: nonzero("CTXS"), show: []show{fab("SFA"), mem("%Z", 1)}},
	{name: "FS context 2", call: "LIB$FILE_SCAN", args: []string{"SFB", "FSOK", "FSERR", "CTXS"}, show: []show{fab("SFB")}},
	{name: "FS context 2 again", call: "LIB$FILE_SCAN", args: []string{"SFB", "FSOK", "FSERR", "CTXS"}, show: []show{fab("SFB")}},
	{name: "FSE context", call: "LIB$FILE_SCAN_END", args: []string{"0", "CTXS"},
		data: []string{"%Z:\t.LONG\t0"}, post: nonzero("CTXS"), show: []show{mem("%Z", 1)}},
	{name: "FS no context 1", call: "LIB$FILE_SCAN", args: []string{"SFC", "FSOK", "FSERR"}, show: []show{fab("SFC")}},
	{name: "FS no context 2", call: "LIB$FILE_SCAN", args: []string{"SFD", "FSOK", "FSERR"}, show: []show{fab("SFD")}},
	// A success routine that signals, part way through a wildcard; then
	// FILE_SCAN_END, and the scan again.
	{name: "FS signal in success", pre: []string{"MOVL\t#2,SUCCACT"},
		call: "LIB$FILE_SCAN", args: []string{"SFS", "FSOK", "FSERR"}, show: []show{fab("SFS")}},
	{name: "FS after signal", call: "LIB$FILE_SCAN", args: []string{"SFS", "FSOK", "FSERR"}, show: []show{fab("SFS")}},
	{name: "FS signal again", pre: []string{"MOVL\t#2,SUCCACT"},
		call: "LIB$FILE_SCAN", args: []string{"SFS", "FSOK", "FSERR"}, show: []show{fab("SFS")}},
	{name: "FSE after signal", call: "LIB$FILE_SCAN_END", args: []string{"SFS"}, show: []show{fab("SFS")}},
	{name: "FS after FSE", call: "LIB$FILE_SCAN", args: []string{"SFS", "FSOK", "FSERR"}, show: []show{fab("SFS")}},
	// FILE_SCAN_END's arguments.
	{name: "FS context, signal", pre: []string{"MOVL\t#2,SUCCACT"},
		call: "LIB$FILE_SCAN", args: []string{"SFE", "FSOK", "FSERR", "CTXU"},
		data: []string{"%Z:\t.LONG\t0"}, post: nonzero("CTXU"), show: []show{fab("SFE"), mem("%Z", 1)}},
	{name: "FSE fab and context", call: "LIB$FILE_SCAN_END", args: []string{"SFE", "CTXU"},
		data: []string{"%Z:\t.LONG\t0"}, post: nonzero("CTXU"), show: []show{fab("SFE"), mem("%Z", 1)}},
	{name: "FSE fab and context again", call: "LIB$FILE_SCAN_END", args: []string{"SFE", "CTXU"}, show: []show{fab("SFE")}},
	{name: "FSE no arguments", call: "LIB$FILE_SCAN_END"},
	{name: "FSE 0 and 0", call: "LIB$FILE_SCAN_END", args: []string{"0", "0"}},
	{name: "FSE context of 0", data: []string{"%C:\t.LONG\t0"}, call: "LIB$FILE_SCAN_END", args: []string{"0", "%C"}, show: []show{mem("%C", 1)}},

	// DELETE_FILE (subtask 5).
	mkdirs("make DEL directories", "[LIB58.DEL]", "[LIB58.DEL.SUB]"),
	mkfiles("make DEL files", "[LIB58.DEL]D1.TXT", "[LIB58.DEL]D2.TXT", "[LIB58.DEL]D2.TXT", "[LIB58.DEL]D2.TXT",
		"[LIB58.DEL]W1.DAT", "[LIB58.DEL]W2.DAT", "[LIB58.DEL]W3.DAT", "[LIB58.DEL]W4.DAT", "[LIB58.DEL]W5.DAT",
		"[LIB58.DEL]V1.TXT", "[LIB58.DEL]V1.TXT", "[LIB58.DEL]V2.TXT",
		"[LIB58.DEL]O1.TXT", "[LIB58.DEL]Q1.TXT", "[LIB58.DEL]Q2.TXT", "[LIB58.DEL]Q3.TXT", "[LIB58.DEL]Q4.TXT",
		"[LIB58.DEL]Y1.TXT", "[LIB58.DEL]Y2.TXT", "[LIB58.DEL]N1.TXT", "[LIB58.DEL]N2.TXT",
		"[LIB58.DEL]F0.TXT", "[LIB58.DEL]F1.TXT", "[LIB58.DEL]F4.TXT", "[LIB58.DEL]F8.TXT",
		"[LIB58.DEL]E1.TXT", "[LIB58.DEL]E2.TXT", "[LIB58.DEL]C1.TXT", "[LIB58.DEL]C2.TXT",
		"[LIB58.DEL]S1.TXT", "[LIB58.DEL]S2.TXT", "[LIB58.DEL]U1.TXT", "[LIB58.DEL]U2.TXT",
		"[LIB58.DEL]P1.TXT", "[LIB58.DEL.SUB]X.TXT"),
	listCase("DEL list before", "[LIB58.DEL...]*.*;*"),
	del("DF one file", dfArgs{spec: "[LIB58.DEL]D1.TXT;1", succ: true, err: true, user: true}),
	del("DF no version", dfArgs{spec: "[LIB58.DEL]D2.TXT", succ: true, err: true}),
	listCase("DF no version left", "[LIB58.DEL]D2.TXT;*"),
	del("DF ;0", dfArgs{spec: "[LIB58.DEL]D2.TXT;0", succ: true}),
	del("DF ;-1", dfArgs{spec: "[LIB58.DEL]D2.TXT;-1", succ: true}),
	listCase("DF ;0 ;-1 left", "[LIB58.DEL]D2.TXT;*"),
	withPre(del("DF wildcard confirm", dfArgs{spec: "[LIB58.DEL]W*.DAT;*", succ: true, err: true, conf: true, user: true}),
		"MOVL\t#^X55555555,CONFBITS"),
	listCase("DF wildcard left", "[LIB58.DEL]W*.DAT;*"),
	del("DF wildcard all versions", dfArgs{spec: "[LIB58.DEL]V*.TXT;*", succ: true}),
	listCase("DF all versions left", "[LIB58.DEL]V*.TXT;*"),
	del("DF missing", dfArgs{spec: "[LIB58.DEL]NONE.TXT", succ: true, err: true, user: true}),
	del("DF missing no error routine", dfArgs{spec: "[LIB58.DEL]NONE.TXT", succ: true}),
	del("DF missing wildcard", dfArgs{spec: "[LIB58.DEL]NONE*.TXT;*", succ: true, err: true}),
	del("DF missing directory", dfArgs{spec: "[LIB58.NODIR]X.TXT", succ: true, err: true}),
	withPre(del("DF missing error fails", dfArgs{spec: "[LIB58.DEL]NONE.TXT", succ: true, err: true}), "CLRL\tERRBITS"),
	del("DF bad syntax", dfArgs{spec: "[LIB58.DEL]X.TXT;Q", succ: true, err: true}),
	withPre(del("DF confirm opens", dfArgs{spec: "[LIB58.DEL]O1.TXT;*", succ: true, err: true, conf: true}), "MOVL\t#1,CONFACT"),
	withPre(del("DF confirm leaves open", dfArgs{spec: "[LIB58.DEL]Q*.TXT;*", succ: true, err: true, conf: true}), "MOVL\t#4,CONFACT"),
	listCase("DF leaves open left", "[LIB58.DEL]Q*.TXT;*"),
	withPre(del("DF leaves open, error fails", dfArgs{spec: "[LIB58.DEL]Q*.TXT;*", succ: true, err: true, conf: true}),
		"MOVL\t#4,CONFACT", "CLRL\tERRBITS"),
	listCase("DF error fails left", "[LIB58.DEL]Q*.TXT;*"),
	withPre(del("DF leaves open, no error routine", dfArgs{spec: "[LIB58.DEL]Q*.TXT;*", succ: true, conf: true}), "MOVL\t#4,CONFACT"),
	listCase("DF no error routine left", "[LIB58.DEL]Q*.TXT;*"),
	del("DF dynamic resultant", dfArgs{spec: "[LIB58.DEL]Y1.TXT;*", succ: true, err: true, conf: true, res: dyn("%N")}),
	del("DF fixed resultant 10", dfArgs{spec: "[LIB58.DEL]Y2.TXT;*", succ: true, err: true, conf: true, res: fixed("%N", 10)}),
	del("DF missing dynamic resultant", dfArgs{spec: "[LIB58.DEL]NONE.TXT", err: true, res: dyn("%N")}),
	withPre(del("DF nested", dfArgs{spec: "[LIB58.DEL]N1.TXT;*", succ: true, err: true, conf: true}), "MOVL\t#3,CONFACT"),
	listCase("DF nested left", "[LIB58.DEL]N*.TXT;*"),
	del("DF flags 0", dfArgs{spec: "[LIB58.DEL]F0.TXT;*", succ: true, flags: "ZERO"}),
	del("DF flags 1", dfArgs{spec: "[LIB58.DEL]F1.TXT;*", succ: true, flags: "ONE"}),
	del("DF flags 4", dfArgs{spec: "[LIB58.DEL]F4.TXT;*", succ: true, flags: "FOUR"}),
	del("DF flags 8", dfArgs{spec: "[LIB58.DEL]F8.TXT;*", succ: true, flags: "EIGHT"}),
	del("DF default name", dfArgs{spec: "E1", def: "[LIB58.DEL].TXT;*", succ: true, err: true}),
	del("DF related name", dfArgs{spec: "E2;*", rel: "[LIB58.DEL]X.TXT", succ: true, err: true}),
	del("DF context 1", dfArgs{spec: "[LIB58.DEL]C1.TXT;*", succ: true, err: true, ctx: "CTXR"}),
	del("DF context 2", dfArgs{spec: "C2.TXT;*", succ: true, err: true, ctx: "CTXR"}),
	{name: "FSE DF context", call: "LIB$FILE_SCAN_END", args: []string{"0", "CTXR"},
		data: []string{"%Z:\t.LONG\t0"}, post: nonzero("CTXR"), show: []show{mem("%Z", 1)}},
	withPre(del("DF signal in confirm", dfArgs{spec: "[LIB58.DEL]S*.TXT;*", succ: true, err: true, conf: true}), "MOVL\t#2,CONFACT"),
	listCase("DF signal left", "[LIB58.DEL]S*.TXT;*"),
	del("DF after signal", dfArgs{spec: "[LIB58.DEL]S*.TXT;*", succ: true, err: true, conf: true}),
	listCase("DF after signal left", "[LIB58.DEL]S*.TXT;*"),
	withPre(del("DF signal in success", dfArgs{spec: "[LIB58.DEL]U*.TXT;*", succ: true, err: true}), "MOVL\t#2,SUCCACT"),
	listCase("DF success signal left", "[LIB58.DEL]U*.TXT;*"),
	del("DF user argument only", dfArgs{spec: "[LIB58.DEL]P1.TXT;*", user: true}),
	del("DF directory with a file", dfArgs{spec: "[LIB58.DEL]SUB.DIR;1", succ: true, err: true}),
	del("DF 256 characters", dfArgs{spec: "[LIB58.DEL]" + strings.Repeat("X", 245), succ: true, err: true}),
	del("DF default 256 characters", dfArgs{spec: "X", def: "[LIB58.DEL]" + strings.Repeat("X", 245), succ: true, err: true}),
	{name: "DF no arguments", call: "LIB$DELETE_FILE"},
	{name: "DF 11 arguments", data: []string{str("%S", "[LIB58.DEL]NONE.TXT")},
		call: "LIB$DELETE_FILE", args: []string{"%S", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"}},
	listCase("DEL list after", "[LIB58.DEL...]*.*;*"),

	// RENAME_FILE (subtask 6).
	mkdirs("make REN directories", "[LIB58.REN]", "[LIB58.REN.NEW]"),
	mkfiles("make REN files", "[LIB58.REN]R1.TXT", "[LIB58.REN]R2.TXT", "[LIB58.REN]R3.TXT",
		"[LIB58.REN]W1.TXT", "[LIB58.REN]W2.TXT", "[LIB58.REN]W3.TXT", "[LIB58.REN]W4.TXT",
		"[LIB58.REN]V1.TXT", "[LIB58.REN]V1.TXT", "[LIB58.REN]K1.TXT", "[LIB58.REN]K2.TXT", "[LIB58.REN]K4.TXT",
		"[LIB58.REN]G2.TXT", "[LIB58.REN]G4.TXT", "[LIB58.REN]G8.TXT", "[LIB58.REN]G16.TXT",
		"[LIB58.REN]E1.TXT", "[LIB58.REN]E2.TXT", "[LIB58.REN]E3A.TXT", "[LIB58.REN]E3B.TXT",
		"[LIB58.REN]Y1.TXT", "[LIB58.REN]X1.TXT", "[LIB58.REN]X2.TXT", "[LIB58.REN]Z1.TXT", "[LIB58.REN]Z2.TXT",
		"[LIB58.REN]O1.TXT", "[LIB58.REN]T1.TXT", "[LIB58.REN]T1.TXT", "[LIB58.REN]M1.TXT"),
	rename("RF one", rfArgs{old: "[LIB58.REN]R1.TXT", new: "[LIB58.REN]R1.NEW", succ: true, err: true, user: true}),
	rename("RF * type", rfArgs{old: "[LIB58.REN]R2.TXT", new: "*.DAT", succ: true, err: true}),
	rename("RF new directory", rfArgs{old: "[LIB58.REN]R3.TXT", new: "[LIB58.REN.NEW]", succ: true, err: true}),
	withPre(rename("RF wildcard confirm", rfArgs{old: "[LIB58.REN]W*.TXT", new: "*.OUT", succ: true, err: true, conf: true, user: true}),
		"MOVL\t#^X55555555,CONFBITS"),
	listCase("RF wildcard left", "[LIB58.REN]W*.*;*"),
	rename("RF V1;1 to V2", rfArgs{old: "[LIB58.REN]V1.TXT;1", new: "V2.TXT", def: "[LIB58.REN]", succ: true, err: true}),
	rename("RF V1;2 to V2", rfArgs{old: "[LIB58.REN]V1.TXT;2", new: "V2.TXT", def: "[LIB58.REN]", succ: true, err: true}),
	listCase("RF versions left", "[LIB58.REN]V*.*;*"),
	rename("RF T1 to T2 ;*", rfArgs{old: "[LIB58.REN]T1.TXT;*", new: "[LIB58.REN]T2.TXT;*", succ: true, err: true}),
	listCase("RF ;* left", "[LIB58.REN]T*.*;*"),
	rename("RF CUR_VER exists", rfArgs{old: "[LIB58.REN]K1.TXT;1", new: "[LIB58.REN]K2.TXT", flags: "ONE", succ: true, err: true}),
	rename("RF CUR_VER new name", rfArgs{old: "[LIB58.REN]K1.TXT;1", new: "[LIB58.REN]K3.TXT", flags: "ONE", succ: true, err: true}),
	rename("RF no CUR_VER exists", rfArgs{old: "[LIB58.REN]K4.TXT;1", new: "[LIB58.REN]K2.TXT", succ: true, err: true}),
	listCase("RF CUR_VER left", "[LIB58.REN]K*.*;*"),
	rename("RF flags 2", rfArgs{old: "[LIB58.REN]G2.TXT", new: "[LIB58.REN]H2.TXT", flags: "TWO", succ: true, err: true}),
	rename("RF flags 4", rfArgs{old: "[LIB58.REN]G4.TXT", new: "[LIB58.REN]H4.TXT", flags: "FOUR", succ: true, err: true}),
	rename("RF flags 8", rfArgs{old: "[LIB58.REN]G8.TXT", new: "[LIB58.REN]H8.TXT", flags: "EIGHT", succ: true, err: true}),
	rename("RF flags 16", rfArgs{old: "[LIB58.REN]G16.TXT", new: "[LIB58.REN]H16.TXT", flags: "%G", succ: true, err: true,
		data: []string{"%G:\t.LONG\t16"}}),
	rename("RF missing", rfArgs{old: "[LIB58.REN]NONE.TXT", new: "[LIB58.REN]NEW.TXT", succ: true, err: true, user: true}),
	rename("RF missing no error routine", rfArgs{old: "[LIB58.REN]NONE.TXT", new: "[LIB58.REN]NEW.TXT", succ: true}),
	rename("RF missing wildcard", rfArgs{old: "[LIB58.REN]NONE*.TXT", new: "*.NEW", succ: true, err: true}),
	rename("RF bad wildcard new", rfArgs{old: "[LIB58.REN]E1.TXT", new: "[LIB58.REN]A*B.TXT", succ: true, err: true, conf: true}),
	rename("RF bad syntax new", rfArgs{old: "[LIB58.REN]E1.TXT", new: "[LIB58.REN]E1.TXT;Q", succ: true, err: true, conf: true}),
	rename("RF new missing directory", rfArgs{old: "[LIB58.REN]E2.TXT", new: "[LIB58.NODIR]E2.TXT", succ: true, err: true, conf: true}),
	rename("RF other device", rfArgs{old: "[LIB58.REN]E2.TXT", new: "NLA0:E2.TXT", succ: true, err: true}),
	withPre(rename("RF error fails", rfArgs{old: "[LIB58.REN]E3*.TXT", new: "[LIB58.NODIR]", succ: true, err: true}), "CLRL\tERRBITS"),
	rename("RF error, no routine", rfArgs{old: "[LIB58.REN]E3*.TXT", new: "[LIB58.NODIR]", succ: true}),
	rename("RF dynamic resultants", rfArgs{old: "[LIB58.REN]Y1.TXT", new: "[LIB58.REN]Y9.TXT", succ: true, err: true, conf: true,
		oldRes: dyn("%P"), newRes: dyn("%Q")}),
	rename("RF missing, resultants", rfArgs{old: "[LIB58.REN]NONE.TXT", new: "[LIB58.REN]Y9.TXT", err: true,
		oldRes: dyn("%P"), newRes: dyn("%Q")}),
	rename("RF fixed resultants 12", rfArgs{old: "[LIB58.REN]Y9.TXT", new: "[LIB58.REN]Y8.TXT", succ: true, err: true,
		oldRes: fixed("%P", 12), newRes: fixed("%Q", 12)}),
	rename("RF context 1", rfArgs{old: "[LIB58.REN]X1.TXT", new: "[LIB58.REN]X1.OUT", succ: true, err: true, ctx: "CTXT"}),
	rename("RF context 2", rfArgs{old: "X2.TXT", new: "X2.OUT", succ: true, err: true, ctx: "CTXT"}),
	{name: "FSE RF context", call: "LIB$FILE_SCAN_END", args: []string{"0", "CTXT"}},
	withPre(rename("RF signal in confirm", rfArgs{old: "[LIB58.REN]Z*.TXT", new: "*.OUT", succ: true, err: true, conf: true}),
		"MOVL\t#2,CONFACT"),
	rename("RF after signal", rfArgs{old: "[LIB58.REN]Z*.TXT", new: "*.OUT", succ: true, err: true, conf: true}),
	listCase("RF signal left", "[LIB58.REN]Z*.*;*"),
	withPre(rename("RF confirm opens", rfArgs{old: "[LIB58.REN]O1.TXT", new: "*.OUT", succ: true, err: true, conf: true}),
		"MOVL\t#1,CONFACT"),
	rename("RF to its own name", rfArgs{old: "[LIB58.REN]M1.TXT", new: "[LIB58.REN]M1.TXT", succ: true, err: true}),
	rename("RF 256-character new", rfArgs{old: "[LIB58.REN]M1.TXT", new: "[LIB58.REN]" + strings.Repeat("X", 245), succ: true, err: true}),
	{name: "RF no arguments", call: "LIB$RENAME_FILE"},
	rename("RF 13 arguments", rfArgs{old: "[LIB58.REN]NONE.TXT", new: "X.TXT", count: 13}),
	listCase("REN list after", "[LIB58.REN...]*.*;*"),

	// FID_TO_NAME (subtask 7).
	getFID("ID of DEEP.TXT", "[LIB58.A.SUB]DEEP.TXT", "FIDDEEP", "DIDDEEP"),
	{name: "the device's DVI", pre: []string{"MOVL\t#1,R0"}, show: []show{txt("DVIDSC")}},
	fid("FTN DEEP by DVI", "DVIDSC", "FIDDEEP", dyn("%D"), 6),
	fid("FTN DEEP by DVI without _", "DVI2DSC", "FIDDEEP", dyn("%D"), 6),
	fid("FTN DEEP by LIB58_DISK:", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "LIB58_DISK:")),
	fid("FTN DEEP by LIB58_DISK", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "LIB58_DISK")),
	fid("FTN DEEP by SYS$DISK", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "SYS$DISK")),
	fid("FTN DEEP device and file", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "LIB58_DISK:[LIB58]X.Y")),
	fid("FTN DEEP into S 10", "DVIDSC", "FIDDEEP", fixed("%D", 10), 6),
	fid("FTN DEEP into S 50", "DVIDSC", "FIDDEEP", fixed("%D", 50), 6),
	fid("FTN DEEP into VS 50", "DVIDSC", "FIDDEEP", varying("%D", 50), 6),
	fid("FTN DEEP 3 arguments", "DVIDSC", "FIDDEEP", dyn("%D"), 3),
	fid("FTN DEEP 4 arguments", "DVIDSC", "FIDDEEP", dyn("%D"), 4),
	withArg(fid("FTN DEEP directory-id", "DVIDSC", "FIDDEEP", dyn("%D"), 6), 4, "FIDONE"),
	getFID("ID of A ONE.TXT", "[LIB58.A]ONE.TXT", "FIDONE", ""),
	fid("FTN ONE.TXT", "DVIDSC", "FIDONE", dyn("%D"), 6),
	getFID("ID of LIB58.DIR", "[000000]LIB58.DIR", "FIDDIR", ""),
	fid("FTN LIB58.DIR", "DVIDSC", "FIDDIR", dyn("%D"), 6),
	fid("FTN MFD", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t4,4,0"),
	fid("FTN INDEXF.SYS", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t1,1,0"),
	fid("FTN BITMAP.SYS", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t2,2,0"),
	fid("FTN unused header", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t200,1,0"),
	fid("FTN past the index file", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t9999,1,0"),
	fid("FTN file number 0", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t0,0,0"),
	withPre(fid("FTN wrong sequence", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t0,0,0"),
		"MOVL\tFIDDEEP,%F", "MOVW\tFIDDEEP+4,%F+4", "INCW\t%F+2"),
	withPre(fid("FTN RVN 1", "DVIDSC", "%F", dyn("%D"), 6, "%F:\t.WORD\t0,0,0"),
		"MOVL\tFIDDEEP,%F", "MOVW\tFIDDEEP+4,%F+4", "MOVB\t#1,%F+4"),
	fid("FTN NLA0:", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "NLA0:")),
	fid("FTN no such device", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "XYZ0:")),
	fid("FTN 64-character device", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", strings.Repeat("D", 63)+":")),
	fid("FTN 65-character device", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", strings.Repeat("D", 64)+":")),
	fid("FTN empty device", "%V", "FIDDEEP", dyn("%D"), 6, str("%V", "")),
	fid("FTN 2 arguments", "DVIDSC", "FIDDEEP", dyn("%D"), 2),
	{name: "FTN no arguments", call: "LIB$FID_TO_NAME"},
	withArg(fid("FTN file-id 0", "DVIDSC", "FIDDEEP", dyn("%D"), 6), 1, "0"),
	// A file with no directory entry, and one whose directory is gone.
	mkdirs("make LOST directories", "[LIB58.LOST]", "[LIB58.GONE]"),
	{name: "make ORPHAN.TXT", data: []string{str("%S", "[LIB58.LOST]ORPHAN.TXT")},
		pre: []string{"PUSHAB\t%S", "CALLS\t#1,MKFILE", "PUSHL\tR0",
			"MOVL\tMKNAM+NAM$W_FID,LOSTFID", "MOVW\tMKNAM+NAM$W_FID+4,LOSTFID+4",
			"MOVL\tMKNAM+NAM$W_DID,LOSTDID", "MOVW\tMKNAM+NAM$W_DID+4,LOSTDID+4", "MOVL\t(SP)+,R0"}},
	fid("FTN ORPHAN.TXT entered", "DVIDSC", "LOSTFID", dyn("%D"), 6),
	{name: "remove ORPHAN.TXT's entry", data: []string{"%A:\t.LONG\t0", "%Q:\t.LONG\t0", "%I:\t.LONG\t0"},
		pre: []string{"$ASSIGN_S\tDEVNAM=DVIDSC,CHAN=ACPCHAN", "MOVL\tR0,%A",
			"MOVC5\t#0,(SP),#0,#64,ACPFIB",
			"MOVL\tLOSTDID,ACPFIB+FIB$W_DID", "MOVW\tLOSTDID+4,ACPFIB+FIB$W_DID+4",
			"$QIOW_S\tCHAN=ACPCHAN,FUNC=#IO$_DELETE,IOSB=ACPIOSB,P1=ACPFIBD,P2=#ORPHNAM",
			"MOVL\tR0,%Q", "MOVZWL\tACPIOSB,%I", "$DASSGN_S\tCHAN=ACPCHAN", "MOVL\t%Q,R0"},
		show: []show{mem("%A", 1), mem("%Q", 1), mem("%I", 1)}},
	listCase("LOST list", "[LIB58.LOST]*.*;*"),
	fid("FTN ORPHAN.TXT removed", "DVIDSC", "LOSTFID", dyn("%D"), 6),
	{name: "make G.TXT", data: []string{str("%S", "[LIB58.GONE]G.TXT")},
		pre: []string{"PUSHAB\t%S", "CALLS\t#1,MKFILE", "PUSHL\tR0",
			"MOVL\tMKNAM+NAM$W_FID,GONEFID", "MOVW\tMKNAM+NAM$W_FID+4,GONEFID+4", "MOVL\t(SP)+,R0"}},
	{name: "erase GONE.DIR", data: []string{"\t.ALIGN\tLONG", "%F:\t$FAB\tFNM=<[LIB58]GONE.DIR;1>"},
		pre: []string{"PUSHL\t#0", "PUSHL\t#0", "PUSHAB\t%F", "CALLS\t#3,G^SYS$ERASE"}, show: []show{fab("%F")}},
	fid("FTN G.TXT", "DVIDSC", "GONEFID", dyn("%D"), 6),

	// TRIM_FILESPEC (subtask 7): the manual's example first.
	trimCase("TF manual's example", "DISK$NAME:[DIRECTORY1.DIRECTORY2]FILENAME.EXTENSTION;1", varying("%D", 32), 32, true),
	trimCase("TF manual's, width omitted", "DISK$NAME:[DIRECTORY1.DIRECTORY2]FILENAME.EXTENSTION;1", fixed("%D", 32), -1, false),
	trimWidths("NODE::DUA1:[DIR.SUB]FILENAME.TYP;12", 40, 35, 34, 29, 28, 24, 23, 15, 14, 12, 11, 8, 7, 1, 0),
	trimCase("TF access control", `NODE"USER PASS"::DUA1:[DIR]F.T;1`, fixed("%D", 40), 30, false),
	trimCase("TF access control 20", `NODE"USER PASS"::DUA1:[DIR]F.T;1`, fixed("%D", 40), 20, false),
	trimCase("TF angle brackets", "DUA1:<DIR.SUB>F.T;1", fixed("%D", 40), 10, false),
	trimCase("TF no type or version", "DUA1:[DIR]FILENAME", fixed("%D", 40), 10, false),
	trimCase("TF type, no version", "DUA1:[DIR]FILENAME.TYP", fixed("%D", 40), 12, false),
	trimCase("TF version dot", "DUA1:[DIR]FILENAME.TYP.12", fixed("%D", 40), 15, false),
	trimCase("TF directory only", "DUA1:[DIR.SUB]", fixed("%D", 40), 6, false),
	trimCase("TF logical name device", "SYS$SYSDEVICE:[SYSMGR]LOGIN.COM;2", fixed("%D", 40), 20, false),
	trimCase("TF already fits", "A.B;1", fixed("%D", 40), 40, false),
	trimCase("TF empty", "", fixed("%D", 40), 10, false),
	trimCase("TF into S 20, omitted", "NODE::DUA1:[DIR.SUB]FILENAME.TYP;12", fixed("%D", 20), -1, false),
	trimCase("TF into D, omitted", "NODE::DUA1:[DIR.SUB]FILENAME.TYP;12", dyn("%D"), -1, false),
	trimCase("TF into D, 20", "NODE::DUA1:[DIR.SUB]FILENAME.TYP;12", dyn("%D"), 20, false),
	trimCase("TF into S 10, 20", "NODE::DUA1:[DIR.SUB]FILENAME.TYP;12", fixed("%D", 10), 20, false),
	trimCase("TF 255 characters", "DUA1:[DIR]"+strings.Repeat("N", 245), fixed("%D", 40), 30, false),
	trimCase("TF 256 characters", "DUA1:[DIR]"+strings.Repeat("N", 246), fixed("%D", 40), 30, false),
	trimCase("TF 257 characters", "DUA1:[DIR]"+strings.Repeat("N", 247), fixed("%D", 40), 30, false),
	{name: "TF 1 argument", data: []string{str("%S", "A.B;1")}, call: "LIB$TRIM_FILESPEC", args: []string{"%S"}},
	{name: "TF 5 arguments", data: []string{str("%S", "A.B;1"), fixed("%D", 10)},
		call: "LIB$TRIM_FILESPEC", args: []string{"%S", "%D", "0", "0", "0"}, show: []show{txt("%D")}},

	// The texts of the shared messages, and the routines' own.
	msg("message LIB$_NOWILD", "^X15112A"),
	msg("message SHR$_NOWILD", "^X112A"),
	msg("message SHR$_PARSEFAIL LIB", "^X15124A"),
	msg("message SHR$_PARSEFAIL CLI", "^X3124A"),
	msg("message LIB$_ERRROUCAL", "^X158029"),
	msg("message LIB$_INVFILSPE", "^X15839C"),
	msg("message LIB$_NORMAL", "^X158001"),
	msg("message SS$_DIRNOTEMPTY", "^X2174"),

	// Last, the cases that give a routine a context or FAB that isn't
	// one, or too few arguments: what VMS does with them might leave its
	// heap or RMS's state damaged for the cases after.
	{name: "FFE of junk", call: "LIB$FIND_FILE_END", args: []string{"JUNKCELL"}, post: nonzero("JUNKCELL"),
		data: []string{"%Z:\t.LONG\t0"}, show: []show{mem("%Z", 1)}},
	{name: "FF junk context", data: []string{str("%S", "[LIB58.A]ONE.TXT"), dyn("%D")},
		call: "LIB$FIND_FILE", args: []string{"%S", "%D", "JUNKCELL"}, show: []show{txt("%D")}},
	{name: "FS not a FAB", call: "LIB$FILE_SCAN", args: []string{"JUNK", "FSOK", "FSERR"}},
	{name: "FS 2 arguments", call: "LIB$FILE_SCAN", args: []string{"SFF", "FSOK"}, show: []show{fab("SFF")}},
	{name: "FS context junk", call: "LIB$FILE_SCAN", args: []string{"SFC", "FSOK", "FSERR", "JUNKCELL"}},
	{name: "FSE not a FAB", call: "LIB$FILE_SCAN_END", args: []string{"JUNK"}},
	{name: "FSE junk context", call: "LIB$FILE_SCAN_END", args: []string{"0", "JUNKCELL"}},
	rename("RF 1 argument", rfArgs{old: "[LIB58.REN]M1.TXT", count: 1}),
}

// mkdirs makes each directory with LIB$CREATE_DIR, every access allowed,
// stopping at the first failure: R0 is the last status, %N how many it
// made.
func mkdirs(name string, dirs ...string) probeCase {
	c := probeCase{name: name, data: []string{"%N:\t.LONG\t0"}, show: []show{mem("%N", 1)}}

	for i, d := range dirs {
		c.data = append(c.data, str(fmt.Sprintf("%%F%d", i), d))
		c.pre = append(c.pre, "PUSHAW\tPROTVAL", "PUSHAW\tPROTEN", "PUSHL\t#0", fmt.Sprintf("PUSHAB\t%%F%d", i),
			"CALLS\t#4,G^LIB$CREATE_DIR", fmt.Sprintf("BLBS\tR0,%%K%d", i), "BRW\t%X", fmt.Sprintf("%%K%d:", i), "INCL\t%N")
	}

	c.pre = append(c.pre, "%X:")

	return c
}

// mkfiles makes each file with MKFILE (a second time makes a new
// version), stopping at the first failure.
func mkfiles(name string, files ...string) probeCase {
	c := probeCase{name: name, data: []string{"%N:\t.LONG\t0"}, show: []show{mem("%N", 1)}}

	for i, f := range files {
		c.data = append(c.data, str(fmt.Sprintf("%%F%d", i), f))
		c.pre = append(c.pre, fmt.Sprintf("PUSHAB\t%%F%d", i), "CALLS\t#1,MKFILE",
			fmt.Sprintf("BLBS\tR0,%%K%d", i), "BRW\t%X", fmt.Sprintf("%%K%d:", i), "INCL\t%N")
	}

	c.pre = append(c.pre, "%X:")

	return c
}

// listCase lists the files spec names (LISTF): R0 is the status that
// ended the list.
func listCase(name, spec string) probeCase {
	return probeCase{name: name, data: []string{str("%S", spec)}, pre: []string{"PUSHAB\t%S", "CALLS\t#1,LISTF"}}
}

// withPre adds instructions before a case's own.
func withPre(c probeCase, lines ...string) probeCase {
	c.pre = append(lines, c.pre...)

	return c
}

// withArg replaces a case's argument i (from 0).
func withArg(c probeCase, i int, a string) probeCase {
	c.args = append([]string(nil), c.args...)
	c.args[i] = a

	return c
}

// ffArgs is a LIB$FIND_FILE case's arguments.
type ffArgs struct {
	spec, def, rel string
	flags          string // a longword's label, "" for none
	res            string // the resultant's data at %D, "" for a dynamic string
	vs             bool   // the resultant is a varying string
}

// find is a LIB$FIND_FILE case on the context cell, showing the
// resultant, status-value (preset to ^X55555555), and whether the cell
// holds a context.
func find(name, cell string, a ffArgs) probeCase {
	res := a.res
	if res == "" {
		res = dyn("%D")
	}

	c := probeCase{name: name, data: []string{str("%S", a.spec), res, "%Z:\t.LONG\t0"},
		pre: []string{"MOVL\t#^X55555555,STV"}, call: "LIB$FIND_FILE",
		args: []string{"%S", "%D", cell, "0", "0", "STV", "0"}, post: nonzero(cell)}

	if a.def != "" {
		c.data = append(c.data, str("%E", a.def))
		c.args[3] = "%E"
	}

	if a.rel != "" {
		c.data = append(c.data, str("%R", a.rel))
		c.args[4] = "%R"
	}

	if a.flags != "" {
		c.args[6] = a.flags
	} else {
		c.args = c.args[:6]
	}

	resShow := txt("%D")
	if a.vs {
		resShow = vs("%D")
	}

	c.show = []show{resShow, mem("STV", 1), mem("%Z", 1)}

	return c
}

// findEnd is a LIB$FIND_FILE_END case on the context cell, showing
// whether the cell still holds a context.
func findEnd(name, cell string) probeCase {
	return probeCase{name: name, data: []string{"%Z:\t.LONG\t0"}, call: "LIB$FIND_FILE_END", args: []string{cell},
		post: nonzero(cell), show: []show{mem("%Z", 1)}}
}

// scan is a LIB$FILE_SCAN case on a FAB of its own (spec, default name
// dnm), with the success and error routines (a label or "0") and the
// context cell ("" for none), showing the FAB after.
func scan(name, spec, dnm, succ, err, cell string) probeCase {
	c := probeCase{name: name, data: []string{fabNam("%F", spec, dnm, "^X0000FAB0")},
		call: "LIB$FILE_SCAN", args: []string{"%F", succ, err}, show: []show{fab("%F")}}

	if succ == "0" {
		c.args[1] = "#0"
	}

	if err == "0" {
		c.args[2] = "#0"
	}

	if cell != "" {
		c.args = append(c.args, cell)
	}

	return c
}

// dfArgs is a LIB$DELETE_FILE case's arguments: which of the probe's
// routines it gets, and the rest.
type dfArgs struct {
	spec, def, rel  string
	succ, err, conf bool
	user            bool
	res             string // the resultant-name's data at %N, "" for none
	ctx, flags      string // a cell's and a longword's labels, "" for none
	count           int    // the argument count, 0 for as far as the last given
}

// del is a LIB$DELETE_FILE case.
func del(name string, a dfArgs) probeCase {
	c := probeCase{name: name, data: []string{str("%S", a.spec)}, call: "LIB$DELETE_FILE"}
	args := []string{"%S", "0", "0", "0", "0", "0", "0", "0", "0", "0"}

	if a.def != "" {
		c.data = append(c.data, str("%E", a.def))
		args[1] = "%E"
	}

	if a.rel != "" {
		c.data = append(c.data, str("%R", a.rel))
		args[2] = "%R"
	}

	if a.succ {
		args[3] = "DFOK"
	}

	if a.err {
		args[4] = "DFERR"
	}

	if a.conf {
		args[5] = "DFCONF"
	}

	if a.user {
		args[6] = userArg
	}

	if a.res != "" {
		c.data = append(c.data, a.res)
		args[7] = "%N"
		c.show = append(c.show, txt("%N"))
	}

	if a.ctx != "" {
		args[8] = a.ctx
	}

	if a.flags != "" {
		args[9] = a.flags
	}

	c.args = trim(args)
	if a.count > 0 {
		c.args = args[:a.count]
	}

	return c
}

// rfArgs is a LIB$RENAME_FILE case's arguments.
type rfArgs struct {
	old, new, def, rel string
	flags              string
	succ, err, conf    bool
	user               bool
	oldRes, newRes     string // the resultant names' data at %P and %Q
	ctx                string
	count              int
	data               []string
}

// rename is a LIB$RENAME_FILE case.
func rename(name string, a rfArgs) probeCase {
	c := probeCase{name: name, data: append([]string{str("%S", a.old), str("%T", a.new)}, a.data...), call: "LIB$RENAME_FILE"}
	args := []string{"%S", "%T", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"}

	if a.def != "" {
		c.data = append(c.data, str("%E", a.def))
		args[2] = "%E"
	}

	if a.rel != "" {
		c.data = append(c.data, str("%R", a.rel))
		args[3] = "%R"
	}

	if a.flags != "" {
		args[4] = a.flags
	}

	if a.succ {
		args[5] = "RFOK"
	}

	if a.err {
		args[6] = "RFERR"
	}

	if a.conf {
		args[7] = "RFCONF"
	}

	if a.user {
		args[8] = userArg
	}

	if a.oldRes != "" {
		c.data = append(c.data, a.oldRes)
		args[9] = "%P"
		c.show = append(c.show, txt("%P"))
	}

	if a.newRes != "" {
		c.data = append(c.data, a.newRes)
		args[10] = "%Q"
		c.show = append(c.show, txt("%Q"))
	}

	if a.ctx != "" {
		args[11] = a.ctx
	}

	c.args = trim(args[:12])
	if a.count > 0 {
		c.args = args[:a.count]
	}

	return c
}

// getFID opens spec and keeps its file ID (and directory ID) in the
// cells, and the device's name from its NAM in DVIDSC (DVI2DSC without
// the leading "_").
func getFID(name, spec, fidCell, didCell string) probeCase {
	did := "#0"
	if didCell != "" {
		did = "#" + didCell
	}

	return probeCase{name: name, data: []string{str("%S", spec)},
		pre:  []string{"PUSHL\t" + did, "PUSHAB\t" + fidCell, "PUSHAB\t%S", "CALLS\t#3,GETFID"},
		show: []show{vmem(fidCell, 2)}}
}

// fid is a LIB$FID_TO_NAME case of the device (a descriptor's label) and
// file ID (three words' label) into the filespec at %D, with nargs of
// its six arguments: filespec-length and acp-status preset to ^X55555555.
func fid(name, dev, id, out string, nargs int, data ...string) probeCase {
	c := probeCase{name: name, data: append([]string{out, "%L:\t.LONG\t^X55555555", "%A:\t.LONG\t^X55555555"}, data...),
		call: "LIB$FID_TO_NAME", args: []string{dev, id, "%D", "%L", "0", "%A"}[:nargs]}

	if strings.Contains(out, ".BYTE\t14,11") {
		c.show = []show{vs("%D")}
	} else {
		c.show = []show{txt("%D")}
	}

	c.show = append(c.show, mem("%L", 1), mem("%A", 1))

	return c
}

// trimCase is a LIB$TRIM_FILESPEC case of old into the output at %D,
// width -1 for none; resultant-length is a word preset to ^X5555.
func trimCase(name, old, out string, width int, isVS bool) probeCase {
	c := probeCase{name: name, data: []string{str("%S", old), out, "%L:\t.WORD\t^X5555,^X5555"},
		call: "LIB$TRIM_FILESPEC", args: []string{"%S", "%D", "0", "%L"}}

	if width >= 0 {
		c.data = append(c.data, fmt.Sprintf("%%W:\t.WORD\t%d", width))
		c.args[2] = "%W"
	}

	if isVS {
		c.show = []show{vs("%D")}
	} else {
		c.show = []show{txt("%D")}
	}

	c.show = append(c.show, mem("%L", 1))

	return c
}

// trimWidths is a group of LIB$TRIM_FILESPEC cases of old at each width,
// into a fixed string of 40. Its cases are spliced into the table by
// expand.
func trimWidths(old string, widths ...int) probeCase {
	c := probeCase{name: "\x00widths"}

	for _, w := range widths {
		c.data = append(c.data, fmt.Sprintf("%d", w))
	}

	c.pre = []string{old}

	return c
}

// msg is a $GETMSG case of code, all its parts shown.
func msg(name, code string) probeCase {
	return probeCase{name: name, data: []string{fixed("%D", 132), "%L:\t.LONG\t0"},
		pre:  []string{fmt.Sprintf("$GETMSG_S\tMSGID=#%s,MSGLEN=%%L,BUFADR=%%D,FLAGS=#15", code)},
		post: []string{"MOVW\t%L,%D"}, show: []show{txt("%D")}}
}

// expand returns the cases with each trimWidths group spliced in.
func expand(in []probeCase) []probeCase {
	var out []probeCase

	for _, c := range in {
		if c.name != "\x00widths" {
			out = append(out, c)

			continue
		}

		for _, w := range c.data {
			var n int
			fmt.Sscan(w, &n)
			out = append(out, trimCase("TF width "+w, c.pre[0], fixed("%D", 40), n, false))
		}
	}

	return out
}

// The program's fixed parts: its data (the output line and the log, the
// FAO control strings, the routines' cells), and its code (the handler,
// the routines the probe gives LIBRTL, and the routines that print).
const programHead = `	.TITLE	LIB58	Phase 58 LIBRTL file routines probe
	.IDENT	/V1.0/

; Written by testdata/lib58/gen.go: do not edit. Calls each of Phase
; 58's LIBRTL routines, case by case, each in a routine of its own under
; HANDLER, and prints with LIB$PUT_OUTPUT what it returned in R0, any
; condition it signaled, what it stored, and what it passed the routines
; the program gave it. See gen.go.

	$SSDEF
	$FABDEF
	$NAMDEF
	$FIBDEF
	$IODEF
	$LNMDEF

LOGSIZE = 30000				; the log's size in bytes
FAILST = ` + failSt + `			; what a routine returns for failure

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
INCALL:	.LONG	0		; a case is running: lines go to the log
LOGEND:	.LONG	0		; the log's length
LOGLOST:	.LONG	0		; lines the log had no room for
CONFBITS:	.LONG	-1		; the confirm routines' answers, a bit a call
ERRBITS:	.LONG	-1		; the error routines'
SUCCBITS:	.LONG	-1		; the success routines'
CONFACT:	.LONG	0		; what the next confirm routine does besides
SUCCACT:	.LONG	0		; what the next success routine does besides
SIGVAL:	.LONG	^X0888888A	; the condition a routine signals
T1:	.LONG	0
T2:	.LONG	0
T3:	.LONG	0
LISTCTX:	.LONG	0		; LISTF's context
LISTD:	.WORD	0		; LISTF's name
	.BYTE	14,2
	.LONG	0
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
LOGDSC:	.WORD	0
	.BYTE	14,1
	.LONG	0
LINE:	.BLKB	132
DVIDSC:	.WORD	0		; the device's name, from a NAM's DVI
	.BYTE	14,1
	.ADDRESS	DVIBUF
DVI2DSC:	.WORD	0		; the same without its "_"
	.BYTE	14,1
	.ADDRESS	DVIBUF+1
DVIBUF:	.BLKB	16
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
VMEMCTL:	.ASCID	"  ~!AC:"
LWCTL:	.ASCID	" !XL"
TXTCTL:	.ASCID	/  !AC: !UL "!AF"/
FABHCTL:	.ASCID	"  !AC:"
LOSTCTL:	.ASCID	"  log: !UL lines lost"
CBHCTL:	.ASCID	"  > !AC !UL"
CBDCTL:	.ASCID	/  >  !UL: dsc !UL,!UL !UL "!AF"/
CBRCTL:	.ASCID	"  >  !UL: ref !XL"
CBVCTL:	.ASCID	"  >  !UL: val !XL"
CBFCTL:	.ASCID	"  >  !UL: fab"
CB0CTL:	.ASCID	"  >  !UL: 0"
FAB1CTL:	.ASCID	"!ACfab bid=!XB sts=!XL stv=!XL ctx=!XL ifi=!UL"
FNACTL:	.ASCID	/!ACfna !UL "!AF"/
DNACTL:	.ASCID	/!ACdna !UL "!AF"/
NAMRCTL:	.ASCID	/!ACnam rsl=!UL "!AF" wcc=!UL/
NAMECTL:	.ASCID	/!ACnam esl=!UL "!AF"/
NAMXCTL:	.ASCID	"!ACnam bid=!XB"
RLFCTL:	.ASCID	/!ACrlf rsl=!UL "!AF"/
OPENCTL:	.ASCID	"  >   open !XL close !XL"
OPEN1CTL:	.ASCID	"  >   open !XL"
NESTCTL:	.ASCID	"  >   nested !XL"
LISTCTL:	.ASCID	"  = !AS"
LISTECTL:	.ASCID	"  = end !XL"
CBPFX:	.ASCIC	"  >     "
SHPFX:	.ASCIC	"    "
; Each routine's name, and its arguments' shape: D a descriptor, F a
; FAB, R a longword by reference, V a value.
NM_FSOK:	.ASCIC	"FS success"
NM_FSERR:	.ASCIC	"FS error"
NM_DFOK:	.ASCIC	"DF success"
NM_DFERR:	.ASCIC	"DF error"
NM_DFCONF:	.ASCIC	"DF confirm"
NM_RFOK:	.ASCIC	"RF success"
NM_RFERR:	.ASCIC	"RF error"
NM_RFCONF:	.ASCIC	"RF confirm"
SH_F:	.ASCIC	"F"
SH_DV:	.ASCIC	"DV"
SH_DRRRV:	.ASCIC	"DRRRV"
SH_DFV:	.ASCIC	"DFV"
SH_DDV:	.ASCIC	"DDV"
SH_DDRRRV:	.ASCIC	"DDRRRV"
SH_DDFV:	.ASCIC	"DDFV"
; MKFILE's FAB, RAB, and NAM, and GETFID's.
	.ALIGN	LONG
MKFAB:	$FAB	FAC=PUT,NAM=MKNAM,RAT=CR,RFM=VAR
MKRAB:	$RAB	FAB=MKFAB,RBF=MKREC,RSZ=6
MKNAM:	$NAM
MKREC:	.ASCII	"LIB58!"
	.ALIGN	LONG
FIDFAB:	$FAB	FAC=GET,NAM=FIDNAM
FIDNAM:	$NAM
	.ALIGN	LONG
LOGBUF:	.BLKB	LOGSIZE
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

; RESETCB puts the routines' answers and actions back as they start:
; success, and nothing more.
	.ENTRY	RESETCB,^M<>
	MNEGL	#1,CONFBITS
	MNEGL	#1,ERRBITS
	MNEGL	#1,SUCCBITS
	CLRL	CONFACT
	CLRL	SUCCACT
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

; CBPRINT name, shape, ap: a routine's line ("  > name argc"), then a
; line for each argument, as shape says to show it.
	.ENTRY	CBPRINT,^M<R2,R3,R4,R5,R6,R7>
	MOVL	12(AP),R6		; the routine's argument list
	MOVZBL	(R6),R5
	$FAO_S	CTRSTR=CBHCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R5
	CALLS	#0,EMIT
	MOVL	8(AP),R4		; the shape
	CLRL	R3
1$:	CMPL	R3,R5
	BLSS	2$
	RET
2$:	MOVL	4(R6)[R3],R2		; the argument
	ADDL3	#1,R3,R7		; its number
	MOVB	#^A"V",R0
	MOVZBL	(R4),R1
	CMPL	R3,R1
	BGEQ	3$
	MOVB	1(R4)[R3],R0
3$:	CMPB	R0,#^A"V"
	BNEQ	4$
	BRW	20$
4$:	TSTL	R2
	BNEQ	5$
	$FAO_S	CTRSTR=CB0CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R7
	BRW	90$
5$:	CMPB	R0,#^A"D"
	BNEQ	6$
	BRW	10$
6$:	CMPB	R0,#^A"R"
	BNEQ	7$
	BRW	30$
7$:	$FAO_S	CTRSTR=CBFCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R7
	CALLS	#0,EMIT
	PUSHL	R2
	PUSHAB	CBPFX
	CALLS	#2,FMTFAB
	BRW	91$
10$:	MOVZBL	2(R2),T1		; a descriptor: type, class, length, text
	MOVZBL	3(R2),T2
	MOVZWL	(R2),T3
	MOVL	4(R2),R1
	BNEQ	11$
	CLRL	T3
11$:	$FAO_S	CTRSTR=CBDCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R7,P2=T1,P3=T2,P4=T3,P5=T3,P6=R1
	BRW	90$
20$:	$FAO_S	CTRSTR=CBVCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R7,P2=R2
	BRW	90$
30$:	$FAO_S	CTRSTR=CBRCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R7,P2=(R2)
90$:	CALLS	#0,EMIT
91$:	INCL	R3
	BRW	1$

; FMTFAB prefix, fab: a FAB's lines, each after the prefix (an ASCIC
; string): its block ID, status, status value, context, and whether it's
; open; its file and default names; then its NAM's.
	.ENTRY	FMTFAB,^M<R2,R3,R4>
	MOVL	8(AP),R2
	MOVZBL	FAB$B_BID(R2),R3
	CLRL	R4
	TSTW	FAB$W_IFI(R2)
	BEQL	1$
	MOVL	#1,R4
1$:	$FAO_S	CTRSTR=FAB1CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,-
		P3=FAB$L_STS(R2),P4=FAB$L_STV(R2),P5=FAB$L_CTX(R2),P6=R4
	CALLS	#0,EMIT
	CMPB	R3,#FAB$C_BID
	BNEQ	9$
	MOVZBL	FAB$B_FNS(R2),R3
	MOVL	FAB$L_FNA(R2),R4
	BNEQ	2$
	CLRL	R3
2$:	$FAO_S	CTRSTR=FNACTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,P3=R3,P4=R4
	CALLS	#0,EMIT
	MOVZBL	FAB$B_DNS(R2),R3
	MOVL	FAB$L_DNA(R2),R4
	BNEQ	3$
	CLRL	R3
3$:	$FAO_S	CTRSTR=DNACTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,P3=R3,P4=R4
	CALLS	#0,EMIT
	MOVL	FAB$L_NAM(R2),R4
	BEQL	9$
	PUSHL	R4
	PUSHL	4(AP)
	CALLS	#2,FMTNAM
9$:	RET

; FMTNAM prefix, nam: a NAM's lines: its resultant string and whether it
; holds a wildcard context, its expanded string, and its related NAM's
; resultant string.
	.ENTRY	FMTNAM,^M<R2,R3,R4,R5>
	MOVL	8(AP),R2
	MOVZBL	NAM$B_BID(R2),R3
	CMPB	R3,#NAM$C_BID
	BEQL	1$
	$FAO_S	CTRSTR=NAMXCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3
	CALLS	#0,EMIT
	RET
1$:	CLRL	R5
	TSTL	NAM$L_WCC(R2)
	BEQL	2$
	MOVL	#1,R5
2$:	MOVZBL	NAM$B_RSL(R2),R3
	MOVL	NAM$L_RSA(R2),R4
	BNEQ	3$
	CLRL	R3
3$:	$FAO_S	CTRSTR=NAMRCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,P3=R3,P4=R4,P5=R5
	CALLS	#0,EMIT
	MOVZBL	NAM$B_ESL(R2),R3
	MOVL	NAM$L_ESA(R2),R4
	BNEQ	4$
	CLRL	R3
4$:	$FAO_S	CTRSTR=NAMECTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,P3=R3,P4=R4
	CALLS	#0,EMIT
	MOVL	NAM$L_RLF(R2),R2
	BEQL	9$
	MOVZBL	NAM$B_RSL(R2),R3
	MOVL	NAM$L_RSA(R2),R4
	BNEQ	5$
	CLRL	R3
5$:	$FAO_S	CTRSTR=RLFCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R3,P3=R3,P4=R4
	CALLS	#0,EMIT
9$:	RET

; CONFDO and SUCCDO do what CONFACT or SUCCACT asks, once, R2 the FAB
; (CONFDO only): 1 $OPEN and $CLOSE it, 2 signal SIGVAL, 3 delete
; NESTSPEC with LIB$DELETE_FILE, 4 $OPEN it and leave it open.
CONFDO:	MOVL	CONFACT,R0
	CLRL	CONFACT
	BRB	DOACT
SUCCDO:	MOVL	SUCCACT,R0
	CLRL	SUCCACT
DOACT:	CASEL	R0,#1,#3
1$:	.WORD	10$-1$,20$-1$,30$-1$,40$-1$
	RSB
10$:	PUSHL	#0
	PUSHL	#0
	PUSHL	R2
	CALLS	#3,G^SYS$OPEN
	MOVL	R0,T1
	PUSHL	#0
	PUSHL	#0
	PUSHL	R2
	CALLS	#3,G^SYS$CLOSE
	$FAO_S	CTRSTR=OPENCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=T1,P2=R0
	CALLS	#0,EMIT
	RSB
20$:	PUSHL	SIGVAL
	CALLS	#1,G^LIB$SIGNAL
	RSB
30$:	PUSHAB	DFOK
	PUSHL	#0
	PUSHL	#0
	PUSHAB	NESTSPEC
	CALLS	#4,G^LIB$DELETE_FILE
	$FAO_S	CTRSTR=NESTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R0
	CALLS	#0,EMIT
	RSB
40$:	PUSHL	#0
	PUSHL	#0
	PUSHL	R2
	CALLS	#3,G^SYS$OPEN
	$FAO_S	CTRSTR=OPEN1CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R0
	CALLS	#0,EMIT
	RSB

; The routines the probe gives LIBRTL: each logs its call and arguments,
; may do CONFACT's or SUCCACT's action, and returns its cell's next bit.
	.ENTRY	FSOK,^M<R2>
	PUSHL	AP
	PUSHAB	SH_F
	PUSHAB	NM_FSOK
	CALLS	#3,CBPRINT
	MOVL	4(AP),R2
	BSBW	SUCCDO
	PUSHAB	SUCCBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	FSERR,^M<R2>
	PUSHL	AP
	PUSHAB	SH_F
	PUSHAB	NM_FSERR
	CALLS	#3,CBPRINT
	PUSHAB	ERRBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	DFOK,^M<R2>
	PUSHL	AP
	PUSHAB	SH_DV
	PUSHAB	NM_DFOK
	CALLS	#3,CBPRINT
	CLRL	R2
	BSBW	SUCCDO
	PUSHAB	SUCCBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	DFERR,^M<R2>
	PUSHL	AP
	PUSHAB	SH_DRRRV
	PUSHAB	NM_DFERR
	CALLS	#3,CBPRINT
	PUSHAB	ERRBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	DFCONF,^M<R2>
	PUSHL	AP
	PUSHAB	SH_DFV
	PUSHAB	NM_DFCONF
	CALLS	#3,CBPRINT
	MOVL	8(AP),R2
	BSBW	CONFDO
	PUSHAB	CONFBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	RFOK,^M<R2>
	PUSHL	AP
	PUSHAB	SH_DDV
	PUSHAB	NM_RFOK
	CALLS	#3,CBPRINT
	CLRL	R2
	BSBW	SUCCDO
	PUSHAB	SUCCBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	RFERR,^M<R2>
	PUSHL	AP
	PUSHAB	SH_DDRRRV
	PUSHAB	NM_RFERR
	CALLS	#3,CBPRINT
	PUSHAB	ERRBITS
	CALLS	#1,CBSTAT
	RET

	.ENTRY	RFCONF,^M<R2>
	PUSHL	AP
	PUSHAB	SH_DDFV
	PUSHAB	NM_RFCONF
	CALLS	#3,CBPRINT
	MOVL	12(AP),R2
	BSBW	CONFDO
	PUSHAB	CONFBITS
	CALLS	#1,CBSTAT
	RET

; LISTF spec: a line ("  = name") for each file LIB$FIND_FILE finds for
; spec, at most 50, then its status ("  = end"), which it returns.
	.ENTRY	LISTF,^M<R2,R3>
	CLRL	LISTCTX
	MOVL	#50,R3
1$:	PUSHAL	LISTCTX
	PUSHAQ	LISTD
	PUSHL	4(AP)
	CALLS	#3,G^LIB$FIND_FILE
	BLBC	R0,2$
	$FAO_S	CTRSTR=LISTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=#LISTD
	CALLS	#0,EMIT
	SOBGTR	R3,1$
2$:	MOVL	R0,R2
	$FAO_S	CTRSTR=LISTECTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=R2
	CALLS	#0,EMIT
	PUSHAL	LISTCTX
	CALLS	#1,G^LIB$FIND_FILE_END
	MOVL	R2,R0
	RET

; MKFILE spec: makes the file (one record), its NAM MKNAM.
	.ENTRY	MKFILE,^M<R2>
	MOVL	4(AP),R2
	MOVB	(R2),MKFAB+FAB$B_FNS
	MOVL	4(R2),MKFAB+FAB$L_FNA
	PUSHL	#0
	PUSHL	#0
	PUSHAB	MKFAB
	CALLS	#3,G^SYS$CREATE
	BLBC	R0,9$
	PUSHL	#0
	PUSHL	#0
	PUSHAB	MKRAB
	CALLS	#3,G^SYS$CONNECT
	BLBC	R0,8$
	PUSHL	#0
	PUSHL	#0
	PUSHAB	MKRAB
	CALLS	#3,G^SYS$PUT
	BLBC	R0,8$
	PUSHL	#0
	PUSHL	#0
	PUSHAB	MKFAB
	CALLS	#3,G^SYS$CLOSE
	RET
8$:	MOVL	R0,R2
	PUSHL	#0
	PUSHL	#0
	PUSHAB	MKFAB
	CALLS	#3,G^SYS$CLOSE
	MOVL	R2,R0
9$:	RET

; GETFID spec, fid, did: opens the file, keeps its file ID (and, unless
; did is 0, its directory ID), three words each, and its NAM's device
; name in DVIDSC and DVI2DSC.
	.ENTRY	GETFID,^M<R2,R3,R4,R5>
	MOVL	4(AP),R2
	MOVB	(R2),FIDFAB+FAB$B_FNS
	MOVL	4(R2),FIDFAB+FAB$L_FNA
	PUSHL	#0
	PUSHL	#0
	PUSHAB	FIDFAB
	CALLS	#3,G^SYS$OPEN
	BLBC	R0,9$
	PUSHL	#0
	PUSHL	#0
	PUSHAB	FIDFAB
	CALLS	#3,G^SYS$CLOSE
	MOVL	8(AP),R1
	MOVL	FIDNAM+NAM$W_FID,(R1)
	MOVW	FIDNAM+NAM$W_FID+4,4(R1)
	MOVL	12(AP),R1
	BEQL	1$
	MOVL	FIDNAM+NAM$W_DID,(R1)
	MOVW	FIDNAM+NAM$W_DID+4,4(R1)
1$:	MOVZBW	FIDNAM+NAM$T_DVI,DVIDSC
	SUBW3	#1,DVIDSC,DVI2DSC
	MOVC3	#15,FIDNAM+NAM$T_DVI+1,DVIBUF
	MOVL	#1,R0
9$:	RET

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
; the name as control (MEMCTL or VMEMCTL) shows it.
	.ENTRY	SHOWMEM,^M<R2,R3,R4>
	MOVL	8(AP),R2
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

; SHOWTXT name, descriptor: a class S or D string's length and text.
	.ENTRY	SHOWTXT,^M<R2,R3>
	MOVL	8(AP),R2
	MOVZWL	(R2),R0
	MOVL	4(R2),R3
	BNEQ	1$
	CLRL	R0
1$:	$FAO_S	CTRSTR=TXTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=R3
	CALLS	#0,EMIT
	RET

; SHOWVS name, descriptor: a varying string's CURLEN and text.
	.ENTRY	SHOWVS,^M<R2>
	MOVL	8(AP),R2
	MOVL	4(R2),R2		; CURLEN, then the text
	MOVZWL	(R2)+,R0
	$FAO_S	CTRSTR=TXTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=R2
	CALLS	#0,EMIT
	RET

; SHOWFAB name, fab: the name's line, then the FAB's.
	.ENTRY	SHOWFAB,^M<>
	$FAO_S	CTRSTR=FABHCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
	CALLS	#0,EMIT
	PUSHL	8(AP)
	PUSHAB	SHPFX
	CALLS	#2,FMTFAB
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

func program(cases []probeCase) string {
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

		fmt.Fprintf(&b, "C%d_NAME:\t.ASCIC\t%s\n", n, delimit(c.name))

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

		b.WriteString("\tMOVL\t(SP)+,R0\n\tRET\n")
	}

	// The main routine: each case, then its lines.
	b.WriteString("\n\t.ENTRY\tSTART,^M<>\n")

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n\tCLRL\tSIGCOND\t\t\t; %d: %s\n\tCALLS\t#0,RESETCB\n\tMOVL\t#1,INCALL\n\tCALLS\t#0,C%d\n\tCLRL\tINCALL\n"+
			"\tPUSHL\tR0\n\tPUSHAB\tC%d_NAME\n\tCALLS\t#2,SHOWR0\n\tCALLS\t#0,FLUSH\n", n, c.name, n, n)

		for k, sh := range c.show {
			memCtl := "MEMCTL"
			if sh.vary {
				memCtl = "VMEMCTL"
			}

			switch sh.kind {
			case "mem":
				fmt.Fprintf(&b, "\tPUSHAQ\t%s\n\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#4,SHOWMEM\n",
					memCtl, sh.count, prefix(sh.label, n), n, k+1)
			case "txt":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWTXT\n", prefix(sh.label, n), n, k+1)
			case "vs":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWVS\n", prefix(sh.label, n), n, k+1)
			case "fab":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWFAB\n", prefix(sh.label, n), n, k+1)
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

// command is lib58.com: it builds and runs the probe, the program's
// output in its own file, on the exchange volume's [000000].
const command = `$ ! LIB58.COM - the Phase 58 probe of LIBRTL's file routines.
$ ! Written by testdata/lib58/gen.go. Run it on a fresh exchange
$ ! volume, with its [000000] as the default directory:
$ !
$ !     @LIB58/OUTPUT=LIB58.LOG
$ !
$ ! LIB58.OUT gets the program's output; LIB58.LOG this procedure's.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE LIB58_DISK 'DEV'
$ SHOW LOGICAL SYS$DISK
$ SHOW LOGICAL LIB58_LIST
$ WRITE SYS$OUTPUT "DEVNAM=", F$GETDVI(DEV,"DEVNAM"), " FULLDEVNAM=", F$GETDVI(DEV,"FULLDEVNAM")
$ WRITE SYS$OUTPUT "NODENAME=", F$GETSYI("NODENAME")
$ DEFINE/USER_MODE SYS$OUTPUT LIB58.MLG
$ MACRO/NOLIST LIB58
$ LINK/NOMAP LIB58
$ DEFINE/USER_MODE SYS$OUTPUT LIB58.OUT
$ RUN LIB58
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE [LIB58...]
$ DEASSIGN LIB58_DISK
`

// exchange is exchange.cmd: the govax console script that makes the
// exchange volume.
const exchange = `! EXCHANGE.CMD - builds the Phase 58 probe's exchange volume with
! govax. Written by testdata/lib58/gen.go. Run from the repository root:
!
!     govax console < testdata/lib58/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib58-exchange.dsk" /DEVICE=RD51 LIB58
MOUNT/WRITE DUA1 "testdata/disks/lib58-exchange.dsk"
COPY "testdata/lib58/lib58.mar"/HOST DUA1:[000000]LIB58.MAR
COPY "testdata/lib58/lib58.com"/HOST DUA1:[000000]LIB58.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`

func main() {
	dir := filepath.Join("testdata", "lib58")
	all := expand(cases)

	for name, text := range map[string]string{
		"lib58.mar":    program(all),
		"lib58.com":    command,
		"exchange.cmd": exchange,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("%d cases\n", len(all))
}
