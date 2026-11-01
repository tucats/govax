//go:build ignore

// Command gen writes the Phase 82 probe of what LINK does with one program
// section given different attributes by different modules (docs/PHASE-82 -
// FORTRAN DEC extensions.md, subtask 7's gap). Run it from the repository root:
//
//	go run testdata/link82/gen.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const dir = "testdata/link82"

// The two sections the variants differ from: BLK has the attributes FORTRAN
// gives a common block (Table 10-1), and LOC those it gives $LOCAL.
var (
	blk = []string{"PIC", "OVR", "REL", "GBL", "SHR", "NOEXE", "RD", "WRT", "LONG"}
	loc = []string{"PIC", "CON", "REL", "LCL", "NOSHR", "NOEXE", "RD", "WRT", "LONG"}
)

// A variant is a module contributing to a section, with some of its attributes
// changed: each key replaces a word of the section's list.
type variant struct {
	name    string
	section string
	changes map[string]string
}

var variants = []variant{
	{"VLCL", "BLK", map[string]string{"GBL": "LCL"}},
	{"VNOSHR", "BLK", map[string]string{"SHR": "NOSHR"}},
	{"VNOWRT", "BLK", map[string]string{"WRT": "NOWRT"}},
	{"VBYTE", "BLK", map[string]string{"LONG": "BYTE"}},
	{"VWORD", "BLK", map[string]string{"LONG": "WORD"}},
	{"VQUAD", "BLK", map[string]string{"LONG": "QUAD"}},
	{"VA4", "BLK", map[string]string{"LONG": "4"}},
	{"VPAGE", "BLK", map[string]string{"LONG": "PAGE"}},
	{"VNOPIC", "BLK", map[string]string{"PIC": "NOPIC"}},
	{"VEXE", "BLK", map[string]string{"NOEXE": "EXE"}},
	{"VNORD", "BLK", map[string]string{"RD": "NORD"}},
	{"VCON", "BLK", map[string]string{"OVR": "CON"}},
	{"VABS", "BLK", map[string]string{"REL": "ABS"}},
	{"VVEC", "BLK", map[string]string{"NOEXE": "NOEXE,VEC"}},
	{"VLIB", "BLK", map[string]string{"NOEXE": "NOEXE,LIB"}},
	{"VMULTI", "BLK", map[string]string{"WRT": "NOWRT", "SHR": "NOSHR", "LONG": "QUAD"}},
	{"LOVR", "LOC", map[string]string{"CON": "OVR"}},
	{"LGBL", "LOC", map[string]string{"LCL": "GBL"}},
	{"LNOWRT", "LOC", map[string]string{"WRT": "NOWRT"}},
	{"LSHR", "LOC", map[string]string{"NOSHR": "SHR"}},
	{"LQUAD", "LOC", map[string]string{"LONG": "QUAD"}},
}

// A linkCase is one LINK: its name is its image's, map's, and listing line's.
type linkCase struct {
	name, what, input string
}

func main() {
	if err := os.MkdirAll(filepath.Join(dir, "vax"), 0o755); err != nil {
		panic(err)
	}

	var modules []string

	module := func(name, section string, attrs []string, global string, data string) {
		modules = append(modules, name)

		var b strings.Builder

		fmt.Fprintf(&b, "\t.TITLE\t%s\n\t.PSECT\t%s,%s\n", name, section, strings.Join(attrs, ","))

		if global != "" {
			fmt.Fprintf(&b, "%s::\n", global)
		}

		// An absolute section can hold no data, only room.
		if slices.Contains(attrs, "ABS") {
			fmt.Fprintf(&b, "\t.BLKB\t32\n\t.END\n")
		} else {
			fmt.Fprintf(&b, "\t.LONG\t%s\n\t.END\n", data)
		}
		write(strings.ToLower(name)+".mar", b.String())
	}

	// Sections of 16 bytes from a base module and 32 from a variant, so the
	// map's contributions show an overlay from a concatenation.
	const short, long = "1,2,3,4", "5,6,7,8,9,10,11,12"

	module("BASE", "BLK", blk, "BLKSTART", short)
	module("BASE2", "BLK", blk, "", long)
	module("LBASE", "LOC", loc, "LOCSTART", short)

	for _, v := range variants {
		attrs := blk
		if v.section == "LOC" {
			attrs = loc
		}

		module(v.name, v.section, changed(attrs, v.changes), "", long)
	}

	// A twin of VLCL, for an LCL section in two clusters with no BASE.
	module("VLCL2", "BLK", changed(blk, map[string]string{"GBL": "LCL"}), "", long)

	mains := map[string]string{"MAINP": "BLKSTART", "MAINL": "LOCSTART", "MAINN": ""}
	for _, name := range []string{"MAINL", "MAINN", "MAINP"} {
		modules = append(modules, name)

		store := ""
		if mains[name] != "" {
			store = "\tMOVL\t#1,G^" + mains[name] + "\n"
		}

		write(strings.ToLower(name)+".mar", "\t.TITLE\t"+name+"\n"+
			"\t.PSECT\t$CODE,PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG\n"+
			"\t.ENTRY\tSTART,^M<>\n"+store+"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")
	}

	var cases []linkCase

	add := func(what, input string) {
		cases = append(cases, linkCase{fmt.Sprintf("C%02d", len(cases)+1), what, input})
	}

	options := func(name, text string) string {
		write(strings.ToLower(name)+".opt", text)

		return name + "/OPTIONS"
	}

	add("the same attributes, a control", "MAINP,BASE,BASE2")

	for _, v := range variants {
		main, base := "MAINP", "BASE"
		if v.section == "LOC" {
			main, base = "MAINL", "LBASE"
		}

		add(base+" then "+v.name, main+","+base+","+v.name)
		add(v.name+" then "+base, main+","+v.name+","+base)
	}

	add("a conflict, then one module like the first", "MAINP,BASE,VNOWRT,BASE2")
	add("a conflict, then one module like the second", "MAINP,BASE,VNOWRT,VNOWRT")
	add("two conflicts with the first", "MAINP,BASE,VNOWRT,VQUAD")
	add("conflicts in two attributes, by two modules", "MAINP,VNOWRT,BASE,VNOSHR")

	add("PSECT_ATTR= makes both WRT", "MAINP,BASE,VNOWRT,"+options("ATTRWRT", "PSECT_ATTR=BLK,WRT\n"))
	add("PSECT_ATTR= makes both NOWRT", "MAINP,BASE,VNOWRT,"+options("ATTRRO", "PSECT_ATTR=BLK,NOWRT\n"))
	add("PSECT_ATTR= makes both QUAD", "MAINP,BASE,VQUAD,"+options("ATTRQ", "PSECT_ATTR=BLK,QUAD\n"))
	add("PSECT_ATTR= makes both GBL", "MAINP,BASE,VLCL,"+options("ATTRG", "PSECT_ATTR=BLK,GBL\n"))
	add("COLLECT= makes both GBL", "MAINP,BASE,VLCL,"+options("COLL", "COLLECT=CC,BLK\n"))

	clusters := func(file, first, second string) string {
		return "MAINP," + options(file, "CLUSTER=ONE,,,"+first+".OBJ\nCLUSTER=TWO,,,"+second+".OBJ\n")
	}

	add("GBL in one cluster, LCL in the next", clusters("CL1", "BASE", "VLCL"))
	add("LCL in one cluster, GBL in the next", clusters("CL2", "VLCL", "BASE"))
	add("LCL in two clusters, a control", "MAINN,"+options("CL3", "CLUSTER=ONE,,,VLCL.OBJ\nCLUSTER=TWO,,,VLCL2.OBJ\n"))
	add("GBL in two clusters, WRT then NOWRT", clusters("CL4", "BASE", "VNOWRT"))
	add("GBL in two clusters, NOWRT then WRT", clusters("CL5", "VNOWRT", "BASE"))

	writeProcedures(modules, cases)
	writeExchange(cases)
}

// changed is attrs with the changes made.
func changed(attrs []string, changes map[string]string) []string {
	out := make([]string, len(attrs))

	for i, a := range attrs {
		out[i] = a
		if to, ok := changes[a]; ok {
			out[i] = to
		}
	}

	return out
}

func writeProcedures(modules []string, cases []linkCase) {
	var asm strings.Builder

	asm.WriteString("$ ! LINK82A.COM - assembles the probe's modules. Written by gen.go.\n$ SET NOON\n")

	for _, m := range modules {
		fmt.Fprintf(&asm, "$ MACRO %s\n$ SHOW SYMBOL $STATUS\n", m)
	}

	write("link82a.com", asm.String())

	write("link82c.com", `$ ! LINK82C.COM - one case: P1 its name, P2 LINK's input, P3 what it asks.
$ ! Written by gen.go.
$ SET NOON
$ WRITE SYS$OUTPUT "=== ''P1': ''P3'"
$ LINK/MAP='P1'/FULL/EXECUTABLE='P1' 'P2'
$ SHOW SYMBOL $STATUS
$ IF F$SEARCH("''P1'.EXE") .EQS. "" THEN GOTO NOIMAGE
$ RUN 'P1'
$ SHOW SYMBOL $STATUS
$ EXIT 1
$NOIMAGE:
$ WRITE SYS$OUTPUT "NO IMAGE"
$ EXIT 1
`)

	var b strings.Builder

	b.WriteString(`$ ! LINK82.COM - the Phase 82 probe of LINK and conflicting program section
$ ! attributes. Written by testdata/link82/gen.go. Run it on the exchange volume,
$ ! with its [000000] as the default directory: @LINK82/OUTPUT=LINK82.LOG
$ SET NOON
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ @LINK82A/OUTPUT=LINK82.MLG
$ SET VERIFY
`)

	for _, c := range cases {
		fmt.Fprintf(&b, "$ @LINK82C %s %q %q\n", c.name, c.input, c.what)
	}

	b.WriteString("$ SET NOVERIFY\n$ WRITE SYS$OUTPUT \"=== END\"\n")
	write("link82.com", b.String())
}

func writeExchange(cases []linkCase) {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds the Phase 82 LINK probe's exchange volume with govax.
! Written by testdata/link82/gen.go. Run from the repository root:
!
!     govax console < testdata/link82/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/link82-exchange.dsk" /DEVICE=RD54 LINK82
MOUNT/WRITE DUA2 "testdata/disks/link82-exchange.dsk"
`)

	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		panic(err)
	}

	for _, f := range files {
		switch filepath.Ext(f) {
		case ".mar", ".com", ".opt":
			fmt.Fprintf(&b, "COPY %q/HOST DUA2:[000000]%s\n", f, strings.ToUpper(filepath.Base(f)))
		}
	}

	b.WriteString("DIRECTORY DUA2:[000000]\nDISMOUNT DUA2\n")
	write("exchange.cmd", b.String())

	b.Reset()
	b.WriteString(`! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/link82/gen.go. Run from the repository root, after the VAX has
! dismounted the volume:
!
!     govax console < testdata/link82/copyout.cmd
!
MOUNT/NOWRITE DUA2 "testdata/disks/link82-exchange.dsk"
COPY DUA2:[000000]LINK82.LOG "testdata/link82/vax/link82.log"/HOST
COPY DUA2:[000000]LINK82.MLG "testdata/link82/vax/link82.mlg"/HOST
`)

	for _, c := range cases {
		fmt.Fprintf(&b, "COPY DUA2:[000000]%s.MAP \"testdata/link82/vax/%s.map\"/HOST\n", c.name, strings.ToLower(c.name))
	}

	b.WriteString("DISMOUNT DUA2\n")
	write("copyout.cmd", b.String())
}

func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		panic(err)
	}
}
