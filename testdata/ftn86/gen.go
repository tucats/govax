//go:build ignore

// Command gen writes the VMS round of the FORTRAN program (docs/PHASE-86 - FORTRAN
// milestone.md, subtask 4): can a program govax's compiler made run on a real VMS, with
// FTNRTL built there from its sources? Run it from the repository root:
//
//	go run testdata/ftn86/gen.go
//
// It writes exchange.cmd (the volume, each program compiled by govax three ways),
// ftn86.com (VMS's build of FTNRTL, and of each program, and its run), and copyout.cmd;
// exchanged.cmd, ftn86d.com, and copyoutd.cmd are the same for DUNGEON, which stays local.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	dir    = "testdata/ftn86"
	volume = "testdata/disks/ftn86-exchange.dsk"
	device = "DUA5:" // DUA2 is the author's XFER volume, mounted at startup

	// round is where copyout.cmd puts the next VMS run, under vax/ (and DUNGEON's log
	// as testdata/disks/ftn86d-ROUND.log), to be checked before it replaces vax/'s.
	round = "round5"
)

// rtlModules are FTNRTL's sources, in ftnrtl.opt's order.
var rtlModules = []string{
	"FTN_VECTOR", "FTN_CONTROL", "FTN_UNITS", "FTN_STATEMENT", "FTN_FORMAT", "FTN_EDIT", "FTN_FLOAT",
	"FTN_LISTOUT", "FTN_LISTIN", "FTN_NAMELIST", "FTN_INTERNAL", "FTN_OPEN", "FTN_INQUIRE", "FTN_POSITION",
	"FTN_UNFORMATTED", "FTN_DIRECT", "FTN_RECORD", "FTN_KEYED", "FTN_INTRINSICS", "FTN_SYSTEM",
}

// variants are the ways every program is compiled, each in a directory of its own:
// optimized, not, and with every unit /RECURSIVE (docs/PHASE-89 - reentrant FORTRAN.md,
// subtask 12), its locals and the compiler's cells in each activation's frame.
var variants = []struct{ suffix, qualifier string }{
	{"", ""},
	{"N", "/NOOPTIMIZE"},
	{"R", "/RECURSIVE"},
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func glob(pattern string) []string {
	names, err := filepath.Glob(pattern)
	check(err)
	slices.Sort(names)

	return names
}

// stem is a path's file name with no type, in VMS's case.
func stem(path string) string {
	return strings.ToUpper(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func main() {
	var ex, com, out strings.Builder

	fmt.Fprintf(&ex, `! EXCHANGE.CMD - builds the FORTRAN round's exchange volume with govax. Written by
! testdata/ftn86/gen.go. Run from the repository root:
!
!     govax console < testdata/ftn86/exchange.cmd
!
INITIALIZE/CONTAINER "%s" /DEVICE=RD54 FTN86
MOUNT/WRITE %s "%s"
SET DEFAULT %s[000000]
COPY/QUIET "%s/ftn86.com"/HOST FTN86.COM
CREATE/DIRECTORY %s[FTN86]
COPY/QUIET "internal/ftnrtl/ftnrtl.opt"/HOST %s[FTN86]FTNRTL.OPT
COPY/QUIET "%s/ftn86.opt"/HOST %s[FTN86]FTN86.OPT
`, volume, device, volume, device, dir, device, device, dir, device)

	for _, m := range rtlModules {
		fmt.Fprintf(&ex, "COPY/QUIET \"internal/ftnrtl/mar/%s.mar\"/HOST %s[FTN86]%s.MAR\n", strings.ToLower(m), device, m)
	}

	fmt.Fprint(&com, `$ ! FTN86.COM - does a program govax's FORTRAN compiler made run on VMS, with FTNRTL
$ ! built here from its sources? (docs/PHASE-86 - FORTRAN milestone.md, subtask 4). Written
$ ! by testdata/ftn86/gen.go. There's no VAX FORTRAN here, and none is needed: each
$ ! program's .MAR file came from govax's compiler, and VMS's MACRO and LINK do the rest.
$ !
$ ! Run it with the exchange volume mounted as DUA1, its [000000] the default directory:
$ !
$ !     @FTN86/OUTPUT=FTN86.LOG
$ !
$ SET NOON
$ ROOT = F$PARSE(F$ENVIRONMENT("DEFAULT"),,,"DEVICE")
$ !
$ ! FTNRTL, from its sources.
$ !
$ SET DEFAULT 'ROOT'[FTN86]
$ WRITE SYS$OUTPUT "==== FTNRTL"
`)

	for _, m := range rtlModules {
		fmt.Fprintf(&com, "$ MACRO/NOLIST %s\n$ SHOW SYMBOL $STATUS\n", m)
	}

	// A name left from an earlier run in this session would be taken for the options
	// file: LINK translates FTNRTL, and doesn't find the image this LINK is to make.
	fmt.Fprint(&com, `$ IF F$TRNLNM("FTNRTL") .NES. "" THEN DEASSIGN FTNRTL
$ LINK/SHAREABLE=FTNRTL/MAP=FTNRTL/FULL FTNRTL/OPTIONS
$ SHOW SYMBOL $STATUS
$ DEFINE FTNRTL 'ROOT'[FTN86]FTNRTL.EXE
$ !
$ ! Each program, compiled by govax three ways, each in a directory of its own, since
$ ! a program may leave files behind it.
$ !
`)

	// The programs of one file.
	var programs []string
	programs = append(programs, glob("testdata/fortran/*.for")...)
	programs = append(programs, glob("testdata/opt84/kernels/*.for")...)
	programs = append(programs, glob(dir+"/*.for")...)

	for _, source := range programs {
		name := stem(source)

		for _, v := range variants {
			d := name + v.suffix
			fmt.Fprintf(&ex, "CREATE/DIRECTORY %s[%s]\n", device, d)
			fmt.Fprintf(&ex, "COPY/QUIET \"%s\"/HOST %s[%s]%s.FOR\n", source, device, d, name)
			fmt.Fprintf(&ex, "FORTRAN%s %s[%s]%s\n", v.qualifier, device, d, name)

			input := strings.TrimSuffix(source, ".for") + ".in"
			if _, err := os.Stat(input); err == nil {
				fmt.Fprintf(&ex, "COPY/QUIET \"%s\"/HOST %s[%s]%s.IN\n", input, device, d, name)
			}

			fmt.Fprintf(&com, "$ CALL ONE %s %s %s\n", d, name, name)
		}
	}

	// The programs of several files, linked in the first order links.txt gives.
	for _, program := range glob("testdata/fortran/modules/*") {
		links, err := os.ReadFile(filepath.Join(program, "links.txt"))
		check(err)

		link, _, _ := strings.Cut(string(links), "\n")
		link = strings.TrimSpace(link)
		name := stem(program)

		for _, v := range variants {
			d := "M_" + name + v.suffix
			fmt.Fprintf(&ex, "CREATE/DIRECTORY %s[%s]\n", device, d)

			for _, file := range glob(filepath.Join(program, "*")) {
				base := filepath.Base(file)
				if base == "links.txt" || strings.HasSuffix(base, ".out") {
					continue
				}

				fmt.Fprintf(&ex, "COPY/QUIET \"%s\"/HOST %s[%s]%s\n", file, device, d, strings.ToUpper(base))
			}

			for _, file := range glob(filepath.Join(program, "*.for")) {
				fmt.Fprintf(&ex, "FORTRAN%s %s[%s]%s\n", v.qualifier, device, d, stem(file))
			}

			fmt.Fprintf(&com, "$ CALL ONE %s %s %s\n", d, name, link)
		}
	}

	fmt.Fprint(&com, `$ SET DEFAULT 'ROOT'[000000]
$ WRITE SYS$OUTPUT "==== END"
$ EXIT
$ !
$ ! ONE directory image modules: assembles and links a program, and runs it with what
$ ! it reads, if anything. MACRO and LINK should say nothing.
$ !
$ ONE: SUBROUTINE
$   SET NOON
$   WRITE SYS$OUTPUT "==== ''P1'"
$   SET DEFAULT 'ROOT'['P1']
$   MACRO/NOLIST 'P3'
$   LINK/NOMAP/EXECUTABLE='P2' 'P3','ROOT'[FTN86]FTN86/OPTIONS
$   IF F$SEARCH("''P2'.IN") .NES. "" THEN DEFINE/USER_MODE SYS$INPUT 'P2'.IN
$   RUN 'P2'
$   SHOW SYMBOL $STATUS
$ ENDSUBROUTINE
`)

	fmt.Fprintf(&ex, "SET DEFAULT %s[000000]\nDISMOUNT %s\nQUIT\n", device, device)

	fmt.Fprintf(&out, `! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/ftn86/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     mkdir -p testdata/ftn86/vax/%[1]s
!     govax console < testdata/ftn86/copyout.cmd
!
! A run goes to vax/%[1]s, beside the one the tests read in vax/, until it's checked.
MOUNT/NOWRITE %[2]s "%[3]s"
COPY/QUIET %[2]s[000000]FTN86.LOG "%[4]s/ftn86.log"/HOST
COPY/QUIET %[2]s[FTN86]FTNRTL.MAP "%[4]s/ftnrtl.map"/HOST
COPY/QUIET/BINARY %[2]s[FTN86]FTNRTL.EXE "%[4]s/ftnrtl.exe"/HOST
`, round, device, volume, dir+"/vax/"+round)

	for _, m := range rtlModules {
		fmt.Fprintf(&out, "COPY/QUIET %s[FTN86]%s.OBJ \"%s/vax/%s/%s.obj\"/HOST\n", device, m, dir, round, strings.ToLower(m))
	}

	fmt.Fprintf(&out, "DISMOUNT %s\nQUIT\n", device)

	write := func(name, text string) {
		check(os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
	}

	writeDungeon(write)

	write("exchange.cmd", ex.String())
	write("ftn86.com", com.String())
	write("copyout.cmd", out.String())
}

// writeDungeon writes DUNGEON's part. The kit is reached by a symbolic link in the
// gitignored testdata/disks (README.md), and its log goes back there too, since it and
// the volume's compiled .MAR files hold the game's text, which is never committed.
func writeDungeon(write func(name, text string)) {
	const kit = "testdata/disks/dungeon-3.2A"

	var ex, com strings.Builder

	fmt.Fprintf(&ex, `! EXCHANGED.CMD - adds DUNGEON to the FORTRAN round's exchange volume, after EXCHANGE.CMD.
! Written by testdata/ftn86/gen.go. Run from the repository root:
!
!     govax console < testdata/ftn86/exchanged.cmd
!
MOUNT/WRITE %[1]s "%[2]s"
SET DEFAULT %[1]s[000000]
COPY/QUIET "%[3]s/ftn86d.com"/HOST FTN86D.COM
`, device, volume, dir)

	modules := []string{"DUNGEON", "GAME", "GDT", "PARSER", "SUBR", "ROOMS", "VERBS", "OBJECTS", "TIMEFNC"}

	var sources []string

	for _, m := range modules {
		if m != "GAME" {
			sources = append(sources, m+".F")
		}
	}

	fmt.Fprint(&com, `$ ! FTN86D.COM - DUNGEON on VMS, built by VMS's MACRO and LINK from the .MAR files govax's
$ ! compiler made, against the FTNRTL FTN86.COM built. Written by testdata/ftn86/gen.go.
$ ! [DUNGEON] is compiled the usual way, [DUNGEONR] /RECURSIVE, its sections' names
$ ! ending in R.
$ !
$ ! Run it after FTN86.COM, with the exchange volume's [000000] the default directory:
$ !
$ !     @FTN86D/OUTPUT=FTN86D.LOG
$ !
$ SET NOON
$ ROOT = F$PARSE(F$ENVIRONMENT("DEFAULT"),,,"DEVICE")
$ DEFINE FTNRTL 'ROOT'[FTN86]FTNRTL.EXE
`)

	// Each build, in a directory of its own, since a game saves its file there.
	for _, build := range []struct{ suffix, qualifier string }{{"", ""}, {"R", "/RECURSIVE"}} {
		d := "DUNGEON" + build.suffix
		fmt.Fprintf(&ex, `CREATE/DIRECTORY %[1]s[%[2]s]
SET DEFAULT %[1]s[%[2]s]
COPY/QUIET "%[3]s/dindx"/HOST DINDX.DAT
COPY/QUIET "%[3]s/dtext"/HOST DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80
COPY/QUIET "%[3]s/dparam.for"/HOST DPARAM.FOR
COPY/QUIET "testdata/dungeon86/fixtime.for"/HOST FIXTIME.FOR
COPY/QUIET "testdata/dungeon86/game.in"/HOST GAME.IN
COPY/QUIET "testdata/dungeon86/save.in"/HOST SAVE.IN
COPY/QUIET "testdata/dungeon86/restore.in"/HOST RESTORE.IN
`, device, d, kit)

		for _, m := range modules {
			fmt.Fprintf(&ex, "COPY/QUIET \"%s/%s.f\"/HOST %s.F\n", kit, strings.ToLower(m), m)
		}

		fmt.Fprintf(&ex, "FORT/NOCHECK/CONT=40%s %s,GAME.F+FIXTIME.FOR\n", build.qualifier, strings.Join(sources, ","))

		fmt.Fprintf(&com, `$ SET DEFAULT 'ROOT'[%[1]s]
$ WRITE SYS$OUTPUT "==== BUILD%[2]s"
$ MACRO/NOLIST %[3]s
$ LINK/NOMAP/EXECUTABLE=DUNGEON %[3]s,'ROOT'[FTN86]FTN86/OPTIONS
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "==== GAME%[2]s"
$ DEFINE/USER_MODE SYS$INPUT GAME.IN
$ RUN DUNGEON
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "==== SAVE%[2]s"
$ DEFINE/USER_MODE SYS$INPUT SAVE.IN
$ RUN DUNGEON
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "==== RESTORE%[2]s"
$ DEFINE/USER_MODE SYS$INPUT RESTORE.IN
$ RUN DUNGEON
$ SHOW SYMBOL $STATUS
`, d, build.suffix, strings.Join(modules, ","))
	}

	fmt.Fprintf(&ex, "SET DEFAULT %s[000000]\nDISMOUNT %s\nQUIT\n", device, device)
	fmt.Fprint(&com, `$ SET DEFAULT 'ROOT'[000000]
$ WRITE SYS$OUTPUT "==== END"
`)

	write("exchanged.cmd", ex.String())
	write("ftn86d.com", com.String())
	write("copyoutd.cmd", fmt.Sprintf(`! COPYOUTD.CMD - copies DUNGEON's log off the exchange volume, to the gitignored
! testdata/disks: it holds the game's text. Written by testdata/ftn86/gen.go. Run from the
! repository root, after the VAX has dismounted the volume:
!
!     govax console < testdata/ftn86/copyoutd.cmd
!
MOUNT/NOWRITE %[1]s "%[2]s"
COPY/QUIET %[1]s[000000]FTN86D.LOG "testdata/disks/ftn86d-%[3]s.log"/HOST
DISMOUNT %[1]s
QUIT
`, device, volume, round))
}
