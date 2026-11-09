// gen writes Phase 91's probe of concealed and rooted logical names: the
// command procedure lnm91.com, with its table of cases, and the govax
// console scripts exchange.cmd and copyout.cmd. Run it from the repository
// root:
//
//	go run testdata/lnm91/gen.go
//
// lnm91.mar, the program the procedure runs, is written by hand. See
// README.md.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/lnm91"

// proc collects the procedure's lines.
type proc struct{ strings.Builder }

// line adds one line to the procedure.
func (p *proc) line(format string, args ...any) {
	fmt.Fprintf(p, format+"\n", args...)
}

// section starts a part of the log.
func (p *proc) section(title string) {
	p.line("$ !")
	p.line("$ WRITE SYS$OUTPUT \"######## %s\"", title)
}

// run adds a RUN of the program and its cases, as data lines. Data lines
// aren't substituted by DCL, so a case names the device through a logical
// name, never by a symbol.
func (p *proc) run(cases ...string) {
	p.line("$ RUN 'PROG'")

	for _, c := range cases {
		p.line("%s", c)
	}

	p.line("$ SHOW SYMBOL $STATUS")
}

// each makes a case for every spec under every operation and flag in ops
// ("P-", "PN", "PS", "S-").
func each(ops []string, specs ...string) []string {
	var out []string

	for _, s := range specs {
		for _, o := range ops {
			out = append(out, o+" "+s)
		}
	}

	return out
}

// parseOps are the operations a name is looked at with: $PARSE plain,
// without concealing, and for syntax only, and $SEARCH.
var parseOps = []string{"P-", "PN", "PS", "S-"}

// parseOnly are parseOps without $SEARCH, for names no file is expected of.
var parseOnly = []string{"P-", "PN", "PS"}

// names are the process logical names the procedure defines, deassigned at
// its end.
var names = []string{
	"LDEV", "CDISK", "BASE", "BASE2", "CONC", "CONC2", "NOCON", "TERM", "PHYS",
	"A1", "A2", "A3", "A4", "A5", "LIST", "LIST2", "LIST3",
	"DEEP7", "DEEP8", "DEEP9", "LNM91_F",
}

func build() string {
	var p proc

	p.line("$ ! LNM91.COM - Phase 91's probe of concealed and rooted logical names.")
	p.line("$ ! Written by testdata/lnm91/gen.go. Run it on the exchange volume,")
	p.line("$ ! with that volume's [000000] as the default directory:")
	p.line("$ !")
	p.line("$ !     @LNM91/OUTPUT=LNM91.LOG")
	p.line("$ !")
	p.line("$ SET NOON")
	p.line("$ SET VERIFY")

	environment(&p)
	tree(&p)
	definitions(&p)
	guideTable(&p)
	forms(&p)
	chains(&p)
	syntax(&p)
	defaults(&p)
	rooted(&p)
	depth(&p)
	files(&p)
	devices(&p)
	dcl(&p)
	cleanup(&p)

	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// environment shows the machine and how VMS's own system names are defined.
func environment(p *proc) {
	p.section("ENVIRONMENT")
	p.line("$ WRITE SYS$OUTPUT F$GETSYI(\"VERSION\"), \" \", F$GETSYI(\"HW_NAME\"), \" \", F$GETSYI(\"NODENAME\")")
	p.line("$ DEV = F$PARSE(\"[000000]\",,,\"DEVICE\")")
	p.line("$ PDEV = F$GETDVI(DEV,\"DEVNAM\")")
	p.line("$ BARE = F$EXTRACT(1,99,PDEV)")
	p.line("$ SHOW SYMBOL DEV")
	p.line("$ SHOW SYMBOL PDEV")
	p.line("$ SHOW SYMBOL BARE")
	p.line("$ SET DEFAULT 'DEV'[000000]")
	p.line("$ SHOW DEFAULT")

	for _, n := range []string{
		"SYS$SYSROOT", "SYS$SYSDEVICE", "SYS$COMMON", "SYS$SPECIFIC", "SYS$SYSTEM",
		"SYS$MANAGER", "SYS$HELP", "SYS$LIBRARY", "SYS$SHARE", "SYS$DISK",
	} {
		p.line("$ SHOW LOGICAL/FULL %s", n)
	}

	p.line("$ SHOW LOGICAL/FULL DISK$*")
}

// tree makes the directories and files, among them decoys at the top level
// of the volume, so a name that loses its root finds something different.
func tree(p *proc) {
	p.section("TREE")

	for _, d := range []string{
		"LNM91.ROOT1.SUB.DEEP", "LNM91.ROOT1.JONES.K", "LNM91.ROOT1.OTHER", "LNM91.ROOT1.TOP",
		"LNM91.ROOT2.SUB", "SUB", "JONES", "OTHER",
		"D1.D2.D3.D4.D5.D6.D7.D8",
	} {
		p.line("$ CREATE/DIRECTORY 'DEV'[%s]", d)
	}

	p.line("$ CREATE T.DAT")
	p.line("template record")
	p.line("$ !")

	for _, f := range []string{
		"LNM91.ROOT1", "LNM91.ROOT1.SUB", "LNM91.ROOT1.SUB.DEEP", "LNM91.ROOT1.JONES", "LNM91.ROOT1.JONES.K",
		"LNM91.ROOT1.OTHER", "LNM91.ROOT2.SUB", "SUB", "JONES", "OTHER",
	} {
		p.line("$ COPY T.DAT 'DEV'[%s]A.DAT", f)
	}

	p.line("$ COPY T.DAT 'DEV'[LNM91.ROOT2.SUB]B.DAT")
	p.line("$ DEFINE/USER_MODE SYS$OUTPUT LNM91.MLG")
	p.line("$ MACRO/NOLIST LNM91")
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ DEFINE/USER_MODE SYS$OUTPUT LNM91.LLG")
	p.line("$ LINK/NOMAP LNM91")
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ IF F$SEARCH(\"LNM91.EXE\") .EQS. \"\" THEN EXIT")
	p.line("$ PROG = DEV + \"[000000]LNM91\"")
	p.line("$ COPY LNM91.EXE 'DEV'[LNM91.ROOT1.SUB]LNM91.EXE")
}

// definitions defines the names the cases use.
func definitions(p *proc) {
	p.section("DEFINITIONS")

	const both, conceal, term = "/TRANSLATION_ATTRIBUTES=(CONCEALED,TERMINAL)", "/TRANSLATION_ATTRIBUTES=CONCEALED", "/TRANSLATION_ATTRIBUTES=TERMINAL"

	for _, d := range [][3]string{
		{"", "LDEV", "'PDEV'"},
		{conceal, "CDISK", "'PDEV'"},
		{both, "BASE", "'PDEV'[LNM91.ROOT1.]"},
		{both, "BASE2", "'PDEV'[LNM91.ROOT2.]"},
		{conceal, "CONC", "'PDEV'[LNM91.ROOT1.]"},
		{conceal, "CONC2", "LDEV:[LNM91.ROOT1.]"},
		{"", "NOCON", "'PDEV'[LNM91.ROOT1.]"},
		{term, "TERM", "'PDEV'[LNM91.ROOT1.]"},
		{both, "PHYS", "'BARE'[LNM91.ROOT1.]"},
		{"", "A1", "BASE:[SUB]"},
		{"", "A2", "BASE:[SUB.]"},
		{conceal, "A3", "BASE:"},
		{conceal, "A4", "BASE:[SUB]"},
		{"", "A5", "A2:[DEEP]"},
		{"", "LIST", "BASE:,BASE2:"},
		{both, "LIST2", "'PDEV'[LNM91.ROOT1.],'PDEV'[LNM91.ROOT2.]"},
		{"", "LIST3", "NOCON:,CDISK:[LNM91]"},
		{both, "DEEP7", "'PDEV'[D1.D2.D3.D4.D5.D6.D7.]"},
		{both, "DEEP8", "'PDEV'[D1.D2.D3.D4.D5.D6.D7.D8.]"},
		{both, "DEEP9", "'PDEV'[D1.D2.D3.D4.D5.D6.D7.D8.D9.]"},
	} {
		p.line("$ DEFINE%s %s %s", d[0], d[1], d[2])
		p.line("$ SHOW LOGICAL/FULL %s", d[1])
	}
}

// guideTable is the table in the File Applications guide's 6.3.4, with
// default [JONES] on the plain device.
func guideTable(p *proc) {
	p.section("GUIDE TABLE, DEFAULT [JONES] ON THE PLAIN DEVICE")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.line("$ SHOW DEFAULT")
	p.run(each(parseOps,
		"BASE:A.DAT", "BASE:[000000]A.DAT", "BASE:[]A.DAT", "BASE:[-]A.DAT", "BASE:[--]A.DAT",
		"BASE:[SUB]A.DAT", "BASE:[.K]A.DAT", "BASE:[SUB.*...]*.DAT", "BASE:[*]*.DAT",
		"BASE:[*...]*.DAT", "BASE:[...]*.DAT", "BASE:[*.*]*.DAT", "BASE:[000000...]*.DAT",
		"BASE:[000000.SUB]A.DAT", "BASE:[.K.-]A.DAT", "BASE:[SUB.-]A.DAT", "BASE:", "BASE",
	)...)
}

// forms is the same name defined with different attributes.
func forms(p *proc) {
	p.section("FORMS OF THE NAME")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOps,
		"CONC:[SUB]A.DAT", "CONC2:[SUB]A.DAT", "NOCON:[SUB]A.DAT", "TERM:[SUB]A.DAT", "PHYS:[SUB]A.DAT",
		"CDISK:[LNM91.ROOT1.SUB]A.DAT",
	)...)
}

// chains is a name through a name, and search lists of roots.
func chains(p *proc) {
	p.section("CHAINS AND LISTS")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOps,
		"A1:A.DAT", "A1:[.DEEP]A.DAT", "A1:[OTHER]A.DAT", "A2:A.DAT", "A2:[DEEP]A.DAT", "A2:[000000]A.DAT",
		"A3:[SUB]A.DAT", "A4:A.DAT", "A4:[OTHER]A.DAT", "A5:A.DAT",
		"LIST:[SUB]A.DAT", "LIST:[SUB]B.DAT", "LIST:[SUB]*.DAT", "LIST2:[SUB]*.DAT", "LIST3:[SUB]A.DAT",
	)...)
}

// syntax is roots typed in a specification, right and wrong.
func syntax(p *proc) {
	p.section("SYNTAX")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOnly,
		"LDEV:[LNM91.ROOT1.][SUB]A.DAT", "LDEV:[LNM91.ROOT1.]A.DAT", "LDEV:[LNM91.ROOT1.][000000]A.DAT",
		"LDEV:[LNM91.ROOT1.][.K]A.DAT", "LDEV:[LNM91.ROOT1.][-]A.DAT", "LDEV:[LNM91.ROOT1.][]A.DAT",
		"LDEV:[LNM91.ROOT1.][*]A.DAT", "LDEV:[LNM91.ROOT1.][...]A.DAT",
		"LDEV:<LNM91.ROOT1.>[SUB]A.DAT", "LDEV:[LNM91.ROOT1.]<SUB>A.DAT", "LDEV:<LNM91.ROOT1.><SUB>A.DAT",
		"LDEV:[LNM91.ROOT1.][SUB.]A.DAT", "LDEV:[A.][B.]A.DAT", "LDEV:[A...][B]A.DAT", "LDEV:[R*.]A.DAT",
		"LDEV:[R*.][SUB]A.DAT", "LDEV:[A..B.]A.DAT", "LDEV:[.A.][B]A.DAT", "LDEV:[-.A.][B]A.DAT",
		"LDEV:[000000.][SUB]A.DAT", "LDEV:[.][SUB]A.DAT", "LDEV:[.]A.DAT",
		"[LNM91.ROOT1.][SUB]A.DAT", "[LNM91.ROOT1.]A.DAT",
		"BASE:[A.]X.DAT", "BASE:[A.][B]X.DAT",
	)...)
}

// defaults is a root reaching a name through its defaults.
func defaults(p *proc) {
	p.section("ROOTED DEFAULT NAME AND RELATED FILE")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOnly,
		"A.DAT|BASE:[SUB]", "[OTHER]A.DAT|BASE:", "[.K]A.DAT|BASE:[SUB]", "A.DAT|BASE:", "A.DAT|A1:",
		"LDEV:A.DAT|BASE:[SUB]", "LDEV:[OTHER]A.DAT|BASE:[SUB]", "BASE:|[OTHER]A.DAT", "BASE:[SUB]|.DAT",
		"A.DAT||BASE:[SUB]X.DAT", ".DAT||BASE:[SUB]X.DAT", "A.DAT||LDEV:[LNM91.ROOT1.][SUB]X.DAT",
		"BASE:.DAT||LDEV:[OTHER]X.DAT",
	)...)
}

// rooted is the process default on a rooted device.
func rooted(p *proc) {
	p.section("SYS$DISK ROOTED")
	p.line("$ SET DEFAULT BASE:[SUB]")
	p.line("$ SHOW DEFAULT")
	p.line("$ SHOW LOGICAL/FULL SYS$DISK")
	p.run(each(parseOps,
		"A.DAT", "[-]A.DAT", "[--]A.DAT", "[.DEEP]A.DAT", "[OTHER]A.DAT", "[000000]A.DAT", "[]A.DAT",
		"[*]*.DAT", "[...]*.DAT", "LDEV:[LNM91]X.DAT", "LDEV:X.DAT",
	)...)
	p.line("$ SET DEFAULT BASE:[000000]")
	p.line("$ SHOW DEFAULT")
	p.run(each(parseOps, "A.DAT", "[-]A.DAT", "[.SUB]A.DAT", "[SUB]A.DAT")...)
	p.line("$ SET DEFAULT CDISK:[LNM91.ROOT1]")
	p.line("$ SHOW DEFAULT")
	p.run(each(parseOps, "A.DAT", "[.SUB]A.DAT", "BASE:A.DAT")...)
	p.line("$ SET DEFAULT LIST:[SUB]")
	p.line("$ SHOW DEFAULT")
	p.run(each(parseOps, "A.DAT", "*.DAT")...)
	p.line("$ SET DEFAULT 'DEV'[JONES]")
}

// depth is how many levels a root and a directory may have together.
func depth(p *proc) {
	p.section("DEPTH")
	p.line("$ SET DEFAULT 'DEV'[000000]")
	p.run(each(parseOnly,
		"DEEP7:[E1]X.DAT", "DEEP7:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT", "DEEP7:[E1.E2.E3.E4.E5.E6.E7.E8.E9]X.DAT",
		"DEEP8:[E1]X.DAT", "DEEP8:[E1.E2.E3.E4.E5.E6.E7]X.DAT", "DEEP8:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"DEEP8:[E1.E2.E3.E4.E5.E6.E7.E8.E9]X.DAT", "DEEP9:[E1]X.DAT",
		"LDEV:[D1.D2.D3.D4.D5.D6.D7.D8.D9]X.DAT",
	)...)
	p.line("$ CREATE/DIRECTORY/LOG DEEP8:[E1.E2.E3.E4.E5.E6.E7.E8]")
	p.line("$ CREATE/DIRECTORY/LOG DEEP8:[E1.E2.E3.E4.E5.E6.E7.E8.E9]")
	p.line("$ CREATE/DIRECTORY/LOG 'DEV'[D1.D2.D3.D4.D5.D6.D7.D8.E1]")
	p.line("$ CREATE/DIRECTORY/LOG DEEP9:[E1]")
	p.run("C- DEEP8:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT", "O- DEEP8:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"S- DEEP8:[E1...]*.DAT", "S- DEEP8:[*...]*.DAT", "S- LDEV:[D1...]*.DAT")
}

// files is the record file services through a root.
func files(p *proc) {
	p.section("FILES")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(
		"C- BASE:[OTHER]NEW.DAT",
		"O- BASE:[OTHER]NEW.DAT",
		"ON BASE:[OTHER]NEW.DAT",
		"O- CONC:[OTHER]NEW.DAT",
		"O- NOCON:[OTHER]NEW.DAT",
		"O- LDEV:[LNM91.ROOT1.OTHER]NEW.DAT",
		"O- BASE:[NOSUCH]NEW.DAT",
		"O- BASE:[OTHER]NOSUCH.DAT",
		"C- BASE:NEWD.DAT",
		"S- LDEV:[LNM91.ROOT1.JONES]NEWD.DAT",
		"S- LDEV:[JONES]NEWD.DAT",
		"R- BASE:[OTHER]NEW.DAT|BASE2:[SUB]NEW2.DAT",
		"S- BASE2:[SUB]NEW2.DAT",
		"R- BASE2:[SUB]NEW2.DAT|LDEV:[LNM91.ROOT1.OTHER]NEW3.DAT",
		"S- LDEV:[LNM91.ROOT1.OTHER]NEW3.DAT",
		"S- BASE:[OTHER]NEW3.DAT",
		"R- BASE:[OTHER]NEW3.DAT|NEW4",
		"S- BASE:[OTHER]NEW4.DAT",
		"S- LDEV:[LNM91.ROOT1.JONES]NEW4.DAT",
		"E- BASE:[OTHER]NEW4.DAT",
		"E- BASE:[JONES]NEW4.DAT",
		"E- BASE:NEWD.DAT",
		"S- BASE:NEWD.DAT",
		"F- BASE:[SUB]A.DAT",
		"F- LDEV:[LNM91.ROOT1.SUB]A.DAT",
		"FN BASE:[SUB]A.DAT",
		"J-",
	)
	p.line("$ DEFINE LNM91_F BASE:[SUB]LNM91.EXE")
	p.line("$ RUN LNM91_F")
	p.line("J-")
	p.line("$ RUN BASE:[SUB]LNM91")
	p.line("J-")
}

// devices is a rooted name as a device.
func devices(p *proc) {
	p.section("DEVICES")
	p.run(
		"D- BASE:", "D- CONC:", "D- NOCON:", "D- TERM:", "D- PHYS:", "D- CDISK:", "D- LDEV:",
		"D- A1:", "D- A2:", "D- A3:", "D- A4:", "D- LIST:", "D- SYS$SYSROOT:", "D- SYS$COMMON:",
		"D- SYS$SYSDEVICE:", "D- SYS$SYSTEM:",
		"P- SYS$SYSROOT:[SYSEXE]LOGINOUT.EXE", "PN SYS$SYSROOT:[SYSEXE]LOGINOUT.EXE",
		"S- SYS$SYSROOT:[SYSEXE]LOGINOUT.EXE", "SN SYS$SYSROOT:[SYSEXE]LOGINOUT.EXE",
		"S- SYS$SYSTEM:LOGINOUT.EXE", "SN SYS$SYSTEM:LOGINOUT.EXE",
		"S- SYS$COMMON:[SYSEXE]LOGINOUT.EXE", "SN SYS$COMMON:[SYSEXE]LOGINOUT.EXE",
		"S- SYS$SPECIFIC:[SYSEXE]*.EXE", "F- SYS$SYSTEM:LOGINOUT.EXE",
	)
}

// dcl is what DCL and the utilities do with the names.
func dcl(p *proc) {
	p.section("DCL: F$PARSE")
	p.line("$ SET DEFAULT 'DEV'[JONES]")

	for _, s := range []string{
		"BASE:[SUB]A.DAT", "BASE:A.DAT", "BASE:", "BASE:[000000]A.DAT", "BASE:[-]A.DAT", "NOCON:[SUB]A.DAT",
		"CDISK:[LNM91]A.DAT", "A1:A.DAT", "A2:A.DAT", "LIST:[SUB]A.DAT", "LDEV:[LNM91.ROOT1.][SUB]A.DAT",
		"BASE:[NOSUCH]A.DAT", "BASE:[--]A.DAT", "SYS$SYSTEM:LOGINOUT.EXE", "SYS$SYSROOT:[SYSEXE]",
	} {
		p.line("$ CALL FP \"%s\"", s)
	}

	p.section("DCL: F$SEARCH, F$TRNLNM, F$GETDVI, F$FILE_ATTRIBUTES")

	for _, s := range []string{"BASE:[SUB]A.DAT", "BASE:[*...]A.DAT", "A1:A.DAT", "CDISK:[LNM91.ROOT1]A.DAT"} {
		p.line("$ WRITE SYS$OUTPUT F$SEARCH(\"%s\")", s)
		p.line("$ WRITE SYS$OUTPUT F$SEARCH(\"X.NOSUCH\")")
	}

	for _, n := range []string{"BASE", "CONC", "NOCON", "A1", "SYS$SYSROOT", "SYS$SYSDEVICE"} {
		p.line("$ WRITE SYS$OUTPUT \"%s \", F$TRNLNM(\"%s\"), \" \", F$TRNLNM(\"%s\",,,,,\"CONCEALED\"), \" \", F$TRNLNM(\"%s\",,,,,\"TERMINAL\")", n, n, n, n)
	}

	for _, n := range []string{"BASE:", "CONC:", "NOCON:", "CDISK:", "A1:", "SYS$SYSROOT:", "SYS$SYSTEM:"} {
		p.line("$ WRITE SYS$OUTPUT \"%s \", F$GETDVI(\"%s\",\"DEVNAM\"), \" \", F$GETDVI(\"%s\",\"FULLDEVNAM\"), \" \", F$GETDVI(\"%s\",\"CONCEALED\")", n, n, n, n)
	}

	p.line("$ WRITE SYS$OUTPUT F$FILE_ATTRIBUTES(\"BASE:[SUB]A.DAT\",\"FID\")")
	p.line("$ WRITE SYS$OUTPUT F$FILE_ATTRIBUTES(\"BASE:[SUB]A.DAT\",\"DVI\")")

	p.section("DCL: SET DEFAULT")

	for _, s := range []string{
		"BASE:[SUB]", "[.DEEP]", "[-]", "[-]", "[-]", "BASE:", "BASE:[000000]", "[-]", "BASE:[NOSUCH]",
		"A1:", "A2:", "A3:", "A4:", "NOCON:[SUB]", "CONC:[SUB]", "TERM:[SUB]", "LDEV:[LNM91.ROOT1.][SUB]",
		"LIST:[SUB]", "SYS$SYSTEM:", "SYS$SYSROOT:[SYSEXE]", "SYS$COMMON:[SYSEXE]",
	} {
		p.line("$ SET DEFAULT %s", s)
		p.line("$ SHOW DEFAULT")
		p.line("$ SHOW LOGICAL SYS$DISK")
	}

	p.section("DCL: DIRECTORY")
	p.line("$ SET DEFAULT 'DEV'[JONES]")

	for _, s := range []string{
		"BASE:[SUB]", "BASE:[000000]", "BASE:", "BASE:[*]", "BASE:[*...]*.DAT", "BASE:[...]", "BASE:[-]",
		"BASE:[--]", "NOCON:[SUB]", "CONC:[SUB]", "CDISK:[LNM91.ROOT1]", "A1:", "A2:", "LIST:[SUB]",
		"LDEV:[LNM91.ROOT1.][SUB]", "SYS$SYSTEM:LOGINOUT.EXE", "SYS$SYSROOT:[SYSEXE]LOGINOUT.EXE",
		"SYS$COMMON:[SYSEXE]LOGINOUT.EXE", "SYS$SPECIFIC:[SYSEXE]*.EXE",
	} {
		p.line("$ DIRECTORY %s", s)
	}

	p.line("$ DIRECTORY/TOTAL BASE:[SUB]")
	p.line("$ DIRECTORY/SIZE BASE:[SUB]A.DAT")
	p.line("$ DIRECTORY/FULL BASE:[SUB]A.DAT")
	p.line("$ SET DEFAULT BASE:[SUB]")
	p.line("$ DIRECTORY *.DIR,[-]*.DIR")
	p.line("$ DIRECTORY")
	p.line("$ SET DEFAULT BASE:[000000]")
	p.line("$ DIRECTORY [-]*.DIR")
	p.line("$ SET DEFAULT CDISK:[LNM91.ROOT1]")
	p.line("$ DIRECTORY")

	p.section("DCL: FILE COMMANDS")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.line("$ CREATE/DIRECTORY/LOG BASE:[NEWD]")
	p.line("$ CREATE/DIRECTORY/LOG BASE:[NEWD.DEEPER]")
	p.line("$ DIRECTORY 'DEV'[000000]NEWD.DIR")
	p.line("$ DIRECTORY 'DEV'[LNM91.ROOT1]NEWD.DIR")
	p.line("$ CREATE BASE:[NEWD]C.DAT")
	p.line("created record")
	p.line("$ !")
	p.line("$ TYPE BASE:[NEWD]C.DAT")
	p.line("$ COPY/LOG BASE:[NEWD]C.DAT BASE:[NEWD]D.DAT")
	p.line("$ COPY/LOG BASE:[NEWD]C.DAT LDEV:[LNM91]E.DAT")
	p.line("$ COPY/LOG LDEV:[LNM91]E.DAT BASE2:[SUB]E.DAT")
	p.line("$ COPY/LOG BASE:[NEWD]C.DAT BASE:[NEWD]")
	p.line("$ RENAME/LOG BASE:[NEWD]D.DAT BASE:[NEWD]F.DAT")
	p.line("$ RENAME/LOG BASE:[NEWD]F.DAT BASE2:[SUB]G.DAT")
	p.line("$ RENAME/LOG BASE:[NEWD]C.DAT H.DAT")
	p.line("$ DIRECTORY BASE:[NEWD],BASE2:[SUB]")
	p.line("$ DUMP/RECORD BASE:[SUB]A.DAT")
	p.line("$ OPEN/WRITE LNM91_O BASE:[NEWD]W.DAT")
	p.line("$ WRITE LNM91_O \"written\"")
	p.line("$ SHOW LOGICAL/FULL LNM91_O")
	p.line("$ CLOSE LNM91_O")
	p.line("$ OPEN/READ LNM91_O BASE:[NEWD]W.DAT")
	p.line("$ READ LNM91_O R")
	p.line("$ SHOW SYMBOL R")
	p.line("$ CLOSE LNM91_O")
	p.line("$ ANALYZE/RMS_FILE/OUTPUT=LNM91.ANL BASE:[SUB]A.DAT")
	p.line("$ TYPE LNM91.ANL")
	p.line("$ CREATE BASE:[NEWD]P.DAT")
	p.line("one")
	p.line("$ !")
	p.line("$ CREATE BASE:[NEWD]P.DAT")
	p.line("two")
	p.line("$ !")
	p.line("$ PURGE/LOG BASE:[NEWD]P.DAT")
	p.line("$ DELETE/LOG BASE:[NEWD]*.*;*")
	p.line("$ SET PROTECTION=(O:RWED) BASE:[000000]NEWD.DIR")
	p.line("$ DELETE/LOG BASE:[000000]NEWD.DIR;")
	p.line("$ DELETE/LOG BASE2:[SUB]E.DAT;*,G.DAT;*")
	p.line("$ DELETE/LOG LDEV:[LNM91]E.DAT;*")

	p.section("DCL: ERRORS AND IMAGES")
	p.line("$ TYPE BASE:[NOSUCH]X.DAT")
	p.line("$ TYPE BASE:[SUB]NOSUCH.DAT")
	p.line("$ TYPE BASE:[--]X.DAT")
	p.line("$ DIRECTORY BASE:[NOSUCH]")
	p.line("$ RUN BASE:[SUB]NOSUCH")
	p.line("$ RUN SYS$SYSROOT:[SYSEXE]NOSUCH")
	p.line("$ RUN SYS$SYSTEM:NOSUCH")

	p.section("DCL: BACKUP")
	p.line("$ BACKUP/LOG BASE:[SUB...]*.*;* LNM91.BCK/SAVE_SET")
	p.line("$ BACKUP/LIST LNM91.BCK/SAVE_SET")
	p.line("$ BACKUP/LOG LNM91.BCK/SAVE_SET/SELECT=[LNM91.ROOT1.SUB]A.DAT BASE:[OTHER]RESTORED.DAT")
	p.line("$ DIRECTORY BASE:[OTHER]")
	p.line("$ DELETE/LOG BASE:[OTHER]RESTORED.DAT;*")
	p.line("$ !")
	p.line("$ GOTO DONE")

	// F$PARSE's fields of one name, with and without concealing.
	p.line("$ FP: SUBROUTINE")
	p.line("$ V = F$VERIFY(0)")
	p.line("$ WRITE SYS$OUTPUT \"== F$PARSE \", P1")
	p.line("$ WRITE SYS$OUTPUT \"  full  <\", F$PARSE(P1), \">\"")
	p.line("$ WRITE SYS$OUTPUT \"  nocon <\", F$PARSE(P1,,,,\"NO_CONCEAL\"), \">\"")
	p.line("$ WRITE SYS$OUTPUT \"  synt  <\", F$PARSE(P1,,,,\"SYNTAX_ONLY\"), \">\"")
	p.line("$ WRITE SYS$OUTPUT \"  dev   <\", F$PARSE(P1,,,\"DEVICE\"), \"> dir <\", F$PARSE(P1,,,\"DIRECTORY\"), \">\"")
	p.line("$ WRITE SYS$OUTPUT \"  ndev  <\", F$PARSE(P1,,,\"DEVICE\",\"NO_CONCEAL\"), \"> ndir <\", F$PARSE(P1,,,\"DIRECTORY\",\"NO_CONCEAL\"), \">\"")
	p.line("$ WRITE SYS$OUTPUT \"  name  <\", F$PARSE(P1,,,\"NAME\"), \"> type <\", F$PARSE(P1,,,\"TYPE\"), \"> ver <\", F$PARSE(P1,,,\"VERSION\"), \">\"")
	p.line("$ V = F$VERIFY(V)")
	p.line("$ ENDSUBROUTINE")
	p.line("$ DONE:")
}

// cleanup puts the process back as it was.
func cleanup(p *proc) {
	p.section("CLEANUP")
	p.line("$ SET DEFAULT 'DEV'[000000]")

	for _, n := range names {
		p.line("$ DEASSIGN %s", n)
	}

	p.line("$ SHOW DEFAULT")
	p.line("$ WRITE SYS$OUTPUT \"LNM91 FINISHED\"")
}

const exchange = `! EXCHANGE.CMD - builds Phase 91's probe exchange volume with govax.
! Written by testdata/lnm91/gen.go. Run from the repository root:
!
!     govax console < testdata/lnm91/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lnm91-exchange.dsk" /DEVICE=RD51 LNM91
MOUNT/WRITE DUA5 "testdata/disks/lnm91-exchange.dsk"
COPY "testdata/lnm91/lnm91.mar"/HOST DUA5:[000000]LNM91.MAR
COPY "testdata/lnm91/lnm91.com"/HOST DUA5:[000000]LNM91.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
`

const copyout = `! COPYOUT.CMD - copies Phase 91's probe results off lnm91-exchange.dsk
! into testdata/lnm91/vax (README.md). Written by testdata/lnm91/gen.go.
! Run from the repository root after @LNM91/OUTPUT=LNM91.LOG on the VAX:
!
!     govax console < testdata/lnm91/copyout.cmd
!
! The image comes back byte for byte (/BINARY), so govax's oracle can run
! VMS's own image.
MOUNT DUA5 "testdata/disks/lnm91-exchange.dsk"
COPY DUA5:[000000]LNM91.LOG "testdata/lnm91/vax/lnm91.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]LNM91.EXE "testdata/lnm91/vax/lnm91.exe"/HOST/QUIET
DISMOUNT DUA5
`

// buildB is round 2: plain and concealed names with plain directories, a
// tree 15 levels deep through a root, and angle brackets.
func buildB() string {
	var p proc

	p.line("$ ! LNM91B.COM - Phase 91's probe, round 2. Written by testdata/lnm91/gen.go.")
	p.line("$ ! Run it on its exchange volume, with that volume's [000000] as the")
	p.line("$ ! default directory:")
	p.line("$ !")
	p.line("$ !     @LNM91B/OUTPUT=LNM91B.LOG")
	p.line("$ !")
	p.line("$ SET NOON")
	p.line("$ SET VERIFY")
	p.section("ENVIRONMENT")
	p.line("$ WRITE SYS$OUTPUT F$GETSYI(\"VERSION\"), \" \", F$GETSYI(\"HW_NAME\"), \" \", F$GETSYI(\"NODENAME\")")
	p.line("$ DEV = F$PARSE(\"[000000]\",,,\"DEVICE\")")
	p.line("$ PDEV = F$GETDVI(DEV,\"DEVNAM\")")
	p.line("$ SHOW SYMBOL DEV")
	p.line("$ SHOW SYMBOL PDEV")
	p.line("$ SET DEFAULT 'DEV'[000000]")

	p.section("TREE")

	for _, d := range []string{"LNM91.SUB", "LNM91.ROOT1", "SUB", "JONES", "OTHER", "R1.R2.R3.R4.R5.R6.R7"} {
		p.line("$ CREATE/DIRECTORY 'DEV'[%s]", d)
	}

	p.line("$ CREATE T.DAT")
	p.line("template record")
	p.line("$ !")

	for _, f := range []string{"LNM91", "LNM91.SUB", "LNM91.ROOT1", "SUB", "JONES", "OTHER"} {
		p.line("$ COPY T.DAT 'DEV'[%s]A.DAT", f)
	}

	p.line("$ DEFINE/USER_MODE SYS$OUTPUT LNM91.MLG")
	p.line("$ MACRO/NOLIST LNM91")
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ DEFINE/USER_MODE SYS$OUTPUT LNM91.LLG")
	p.line("$ LINK/NOMAP LNM91")
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ IF F$SEARCH(\"LNM91.EXE\") .EQS. \"\" THEN EXIT")
	p.line("$ PROG = DEV + \"[000000]LNM91\"")

	p.section("DEFINITIONS")

	const both, conceal = "/TRANSLATION_ATTRIBUTES=(CONCEALED,TERMINAL)", "/TRANSLATION_ATTRIBUTES=CONCEALED"

	for _, d := range [][3]string{
		{"", "LDEV", "'PDEV'"},
		{"", "P", "'PDEV'[LNM91]"},
		{"", "P2", "P:"},
		{"", "P3", "LDEV:[LNM91]"},
		{"", "N", "'PDEV'[LNM91]A.DAT"},
		{"", "L", "'PDEV'[LNM91],'PDEV'[LNM91.SUB]"},
		{conceal, "C", "'PDEV'[LNM91]"},
		{both, "C2", "'PDEV'[LNM91]"},
		{conceal, "C3", "LDEV:[LNM91]"},
		{both, "D7", "'PDEV'[R1.R2.R3.R4.R5.R6.R7.]"},
	} {
		p.line("$ DEFINE%s %s %s", d[0], d[1], d[2])
		p.line("$ SHOW LOGICAL/FULL %s", d[1])
	}

	p.section("PLAIN NAMES WITH A DIRECTORY")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOps,
		"P:A.DAT", "P:", "P", "P:[SUB]A.DAT", "P:[.ROOT1]A.DAT", "P:[-]A.DAT", "P:[]A.DAT", "P:[000000]A.DAT",
		"P:[*]A.DAT", "P:[...]A.DAT", "P2:A.DAT", "P2:[SUB]A.DAT", "P3:A.DAT", "P3:[SUB]A.DAT",
		"N:", "N", "N:B.DAT", "N:.TXT", "N:;5", "N:[SUB]", "N:[SUB]B.DAT",
		"L:A.DAT", "L:*.DAT", "L:[SUB]A.DAT",
		"A.DAT|P:", "[SUB]A.DAT|P:", "[.SUB]A.DAT|P:", "B.DAT|N:", "P:B.DAT|[SUB]", "P:[SUB]B.DAT|[OTHER]",
		"A.DAT||P:X.DAT",
	)...)

	p.section("CONCEALED NAMES WITH A DIRECTORY")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOps,
		"C:A.DAT", "C:", "C:[SUB]A.DAT", "C:[.ROOT1]A.DAT", "C:[-]A.DAT", "C:[]A.DAT", "C:[000000]A.DAT",
		"C2:A.DAT", "C2:", "C2:[SUB]A.DAT", "C3:A.DAT", "C3:[SUB]A.DAT",
		"A.DAT|C:", "[SUB]A.DAT|C:", "A.DAT||C:X.DAT",
	)...)

	p.section("SYS$DISK WITH A DIRECTORY")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.line("$ DEFINE SYS$DISK 'PDEV'[LNM91]")
	p.line("$ SHOW DEFAULT")
	p.run(each(parseOps, "A.DAT", "[SUB]A.DAT", "[.SUB]A.DAT", "[]A.DAT", "[-]A.DAT")...)
	p.line("$ DEFINE/TRANSLATION_ATTRIBUTES=CONCEALED SYS$DISK 'PDEV'[LNM91]")
	p.line("$ SHOW DEFAULT")
	p.run(each(parseOps, "A.DAT", "[SUB]A.DAT")...)
	p.line("$ SET DEFAULT 'DEV'[JONES]")

	p.section("DCL ON THOSE NAMES")

	for _, s := range []string{"P:", "P:A.DAT", "P:[SUB]", "C:", "C:A.DAT", "C:[SUB]", "C2:", "N:", "L:"} {
		p.line("$ DIRECTORY %s", s)
		p.line("$ WRITE SYS$OUTPUT \"<\", F$PARSE(\"%s\"), \"> <\", F$PARSE(\"%s\",,,,\"NO_CONCEAL\"), \">\"", s, s)
	}

	for _, s := range []string{"P:", "P:[SUB]", "C:", "C:[SUB]", "C2:", "N:"} {
		p.line("$ SET DEFAULT %s", s)
		p.line("$ SHOW DEFAULT")
		p.line("$ SHOW LOGICAL SYS$DISK")
		p.line("$ SET DEFAULT 'DEV'[JONES]")
	}

	p.line("$ TYPE P:A.DAT")
	p.line("$ TYPE C:A.DAT")
	p.line("$ TYPE N:")
	p.line("$ CREATE/DIRECTORY/LOG P:[NEW]")
	p.line("$ CREATE/DIRECTORY/LOG C:[NEW]")

	p.section("FIFTEEN LEVELS")
	p.line("$ SET DEFAULT 'DEV'[000000]")
	p.line("$ CREATE/DIRECTORY/LOG D7:[E1.E2.E3.E4.E5.E6.E7.E8]")
	p.line("$ COPY T.DAT D7:[E1.E2.E3.E4.E5.E6.E7.E8]Y.DAT")
	p.line("$ DIRECTORY D7:[E1.E2.E3.E4.E5.E6.E7.E8]")
	p.line("$ DIRECTORY LDEV:[R1.R2.R3.R4.R5.R6.R7.E1]")
	p.line("$ DIRECTORY LDEV:[R1.R2.R3.R4.R5.R6.R7.E1.E2]")
	p.line("$ TYPE D7:[E1.E2.E3.E4.E5.E6.E7.E8]Y.DAT")
	p.run(
		"C- D7:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"O- D7:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"ON D7:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"F- D7:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"S- D7:[E1.E2.E3.E4.E5.E6.E7.E8]*.DAT",
		"S- D7:[E1...]*.DAT",
		"S- D7:[*...]*.DAT",
		"SN D7:[E1...]*.DAT",
		"S- LDEV:[R1...]*.DAT",
		"S- LDEV:[R1.R2.R3.R4.R5.R6.R7...]*.DAT",
		"P- LDEV:[R1.R2.R3.R4.R5.R6.R7.E1]X.DAT",
		"P- LDEV:[R1.R2.R3.R4.R5.R6.R7.E1.E2]X.DAT",
		"P- LDEV:[R1.R2.R3.R4.R5.R6.R7.][E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
		"S- LDEV:[R1.R2.R3.R4.R5.R6.R7.][E1.E2.E3.E4.E5.E6.E7.E8]*.DAT",
		"E- D7:[E1.E2.E3.E4.E5.E6.E7.E8]X.DAT",
	)
	p.line("$ DELETE/LOG D7:[E1.E2.E3.E4.E5.E6.E7.E8]Y.DAT;*")
	p.line("$ SET DEFAULT D7:[E1.E2.E3.E4.E5.E6.E7.E8]")
	p.line("$ SHOW DEFAULT")
	p.line("$ SET DEFAULT [-]")
	p.line("$ SHOW DEFAULT")

	p.section("ANGLE BRACKETS")
	p.line("$ SET DEFAULT 'DEV'[JONES]")
	p.run(each(parseOps,
		"LDEV:<SUB>A.DAT", "LDEV:<LNM91.SUB>A.DAT", "LDEV:<LNM91.SUB...>A.DAT", "LDEV:<.K>A.DAT", "LDEV:<>A.DAT",
		"LDEV:<->A.DAT", "LDEV:<000000>A.DAT", "<SUB>A.DAT", "A.DAT|<SUB>", "A.DAT|LDEV:<SUB>", "<SUB>A.DAT|[OTHER]",
		"A.DAT||LDEV:<SUB>X.DAT",
	)...)
	p.line("$ DIRECTORY LDEV:<SUB>")
	p.line("$ WRITE SYS$OUTPUT F$PARSE(\"LDEV:<SUB>A.DAT\")")
	p.line("$ SET DEFAULT LDEV:<SUB>")
	p.line("$ SHOW DEFAULT")

	p.section("CLEANUP")
	p.line("$ SET DEFAULT 'DEV'[000000]")

	for _, n := range []string{"LDEV", "P", "P2", "P3", "N", "L", "C", "C2", "C3", "D7"} {
		p.line("$ DEASSIGN %s", n)
	}

	p.line("$ SHOW DEFAULT")
	p.line("$ WRITE SYS$OUTPUT \"LNM91B FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

const exchangeB = `! EXCHANGEB.CMD - builds round 2's exchange volume with govax. Written by
! testdata/lnm91/gen.go. Run from the repository root:
!
!     govax console < testdata/lnm91/exchangeb.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lnm91b-exchange.dsk" /DEVICE=RD51 LNM91B
MOUNT/WRITE DUA5 "testdata/disks/lnm91b-exchange.dsk"
COPY "testdata/lnm91/lnm91.mar"/HOST DUA5:[000000]LNM91.MAR
COPY "testdata/lnm91/lnm91b.com"/HOST DUA5:[000000]LNM91B.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
`

const copyoutB = `! COPYOUTB.CMD - copies round 2's log off lnm91b-exchange.dsk into
! testdata/lnm91/vax (README.md). Written by testdata/lnm91/gen.go. Run from
! the repository root after @LNM91B/OUTPUT=LNM91B.LOG on the VAX:
!
!     govax console < testdata/lnm91/copyoutb.cmd
!
MOUNT DUA5 "testdata/disks/lnm91b-exchange.dsk"
COPY DUA5:[000000]LNM91B.LOG "testdata/lnm91/vax/lnm91b.log"/HOST/QUIET
DISMOUNT DUA5
`

func main() {
	for name, text := range map[string]string{
		"lnm91.com":     build(),
		"exchange.cmd":  exchange,
		"copyout.cmd":   copyout,
		"lnm91b.com":    buildB(),
		"exchangeb.cmd": exchangeB,
		"copyoutb.cmd":  copyoutB,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
