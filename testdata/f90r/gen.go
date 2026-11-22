//go:build ignore

// Command gen writes the VMS round of the Fortran 90 program (docs/planning/PHASE-111 -
// Fortran 90 milestone.md, subtask 11): does a Fortran 90 program govax's compiler made run
// on a real VMS, with FTNRTL built there from its sources? It is testdata/ftn86's pattern,
// for fcalc and the whole Fortran 90 corpus. Run it from the repository root:
//
//	go run testdata/f90r/gen.go
//
// It writes exchange.cmd (the volume, each program compiled by govax in each of its
// variants), f90r.com (VMS's build of FTNRTL and of each program, and its run), and
// copyout.cmd.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	dir    = "testdata/f90r"
	volume = "testdata/disks/f90r-exchange.dsk"
	device = "DUA5:" // DUA2 is the author's XFER volume, mounted at startup

	// results is where copyout.cmd puts VMS's run: vax/roundN, beside the run the
	// tests read in vax/ (round 1's), until it's checked.
	results = dir + "/vax/round2"

	// headers and directories are INITIALIZE's /HEADERS and /DIRECTORIES: an
	// index file with room for every file govax writes (about 1,700) and VMS's
	// run adds (an object and an image per program, its files, the log), so it
	// never needs an extension header, which VMS 7.1's file system failed on
	// (the phase doc's log); and an MFD with room for the round's 382 names and
	// more, so VMS needn't extend it to add F90R.LOG.
	headers     = 6000
	directories = 800
)

// variants are the ways every program of the corpus is compiled, each in a directory
// of its own, as testdata/ftn86's are: optimized, not, and /RECURSIVE.
var variants = []struct{ suffix, qualifier string }{
	{"", ""},
	{"N", "/NOOPTIMIZE"},
	{"R", "/RECURSIVE"},
}

// fcalcVariants are fcalc's: those, and every check, which runs FTNRTL's checking
// routines on the VAX too. Every one is /G_FLOATING, as fcalc's README says.
var fcalcVariants = append(slices.Clone(variants), struct{ suffix, qualifier string }{"C", "/CHECK=ALL"})

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

func read(path string) string {
	text, err := os.ReadFile(path)
	check(err)

	return string(text)
}

// stem is a path's file name with no type, in VMS's case.
func stem(path string) string {
	return strings.ToUpper(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

// rtlModules are FTNRTL's modules, in ftnrtl.opt's order: its lines that name an object.
func rtlModules() []string {
	var modules []string

	for line := range strings.SplitSeq(read("internal/ftnrtl/ftnrtl.opt"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "!") || !strings.HasSuffix(line, ".OBJ") {
			continue
		}

		modules = append(modules, stem(line[strings.LastIndex(line, ",")+1:]))
	}

	return modules
}

// copyFile is exchange.cmd's line copying a host file to the volume.
func copyFile(ex *strings.Builder, host, target string) {
	fmt.Fprintf(ex, "COPY/QUIET \"%s\"/HOST %s\n", host, target)
}

func main() {
	var ex, com, out strings.Builder

	modules := rtlModules()

	fmt.Fprintf(&ex, `! EXCHANGE.CMD - builds the Fortran 90 round's exchange volume with govax. Written by
! testdata/f90r/gen.go. Run from the repository root:
!
!     govax console < testdata/f90r/exchange.cmd
!
INITIALIZE/CONTAINER "%[1]s" /DEVICE=RD54 /HEADERS=%[4]d /DIRECTORIES=%[5]d F90R
MOUNT/WRITE %[2]s "%[1]s"
SET DEFAULT %[2]s[000000]
COPY/QUIET "%[3]s/f90r.com"/HOST F90R.COM
CREATE/DIRECTORY %[2]s[FTNRTL]
COPY/QUIET "internal/ftnrtl/ftnrtl.opt"/HOST %[2]s[FTNRTL]FTNRTL.OPT
COPY/QUIET "%[3]s/f90r.opt"/HOST %[2]s[FTNRTL]F90R.OPT
`, volume, device, dir, headers, directories)

	for _, m := range modules {
		copyFile(&ex, "internal/ftnrtl/mar/"+strings.ToLower(m)+".mar", device+"[FTNRTL]"+m+".MAR")
	}

	fmt.Fprint(&com, `$ ! F90R.COM - does a Fortran 90 program govax's compiler made run on VMS, with FTNRTL
$ ! built here from its sources? (docs/planning/PHASE-111 - Fortran 90 milestone.md, subtask
$ ! 11). Written by testdata/f90r/gen.go. There's no Fortran 90 compiler here, and none is
$ ! needed: each program's .MAR files came from govax's compiler, and VMS's MACRO and LINK
$ ! do the rest.
$ !
$ ! Run it with the exchange volume mounted, its [000000] the default directory:
$ !
$ !     @F90R/OUTPUT=F90R.LOG
$ !
$ SET NOON
$ ROOT = F$PARSE(F$ENVIRONMENT("DEFAULT"),,,"DEVICE")
$ !
$ ! FTNRTL, from its sources.
$ !
$ SET DEFAULT 'ROOT'[FTNRTL]
$ WRITE SYS$OUTPUT "==== FTNRTL"
`)

	for _, m := range modules {
		fmt.Fprintf(&com, "$ MACRO/NOLIST %s\n$ SHOW SYMBOL $STATUS\n", m)
	}

	// A name left from an earlier run in this session would be taken for the options
	// file: LINK translates FTNRTL, and doesn't find the image this LINK is to make.
	fmt.Fprint(&com, `$ IF F$TRNLNM("FTNRTL") .NES. "" THEN DEASSIGN FTNRTL
$ LINK/SHAREABLE=FTNRTL/MAP=FTNRTL/FULL FTNRTL/OPTIONS
$ SHOW SYMBOL $STATUS
$ DEFINE FTNRTL 'ROOT'[FTNRTL]FTNRTL.EXE
$ !
$ ! fcalc, the milestone program, in each of its variants: every script run in its
$ ! name's order, as TestFcalcMilestone runs them, since STATE's files are read later;
$ ! then the files the scripts wrote, and ONESHOT.COM, fcalc as a foreign command.
$ !
`)

	writeFcalc(&ex, &com)

	fmt.Fprint(&com, `$ !
$ ! The Fortran 90 corpus, each program compiled by govax three ways, each in a
$ ! directory of its own, since a program may leave files behind it.
$ !
`)

	// The programs of one file.
	for _, source := range glob("testdata/fortran90/*.f90") {
		name := stem(source)

		for _, v := range variants {
			d := name + v.suffix
			fmt.Fprintf(&ex, "CREATE/DIRECTORY %[1]s[%[2]s]\nSET DEFAULT %[1]s[%[2]s]\n", device, d)
			copyFile(&ex, source, name+".F90")

			if input := strings.TrimSuffix(source, ".f90") + ".in"; exists(input) {
				copyFile(&ex, input, name+".IN")
			}

			fmt.Fprintf(&ex, "FORTRAN%s %s\n", v.qualifier, name)
			fmt.Fprintf(&com, "$ CALL ONE %s %s %s\n", d, name, name)
		}
	}

	// The programs of several files: compiled in compiles.txt's first order (a FORTRAN
	// command for each word), or all by one command without one, and linked in
	// links.txt's first order. Each order is in its own directory with the module files
	// it writes, so no module of one program is taken for another's.
	for _, program := range glob("testdata/fortran90/modules/*") {
		name := stem(program)
		link := strings.Fields(read(filepath.Join(program, "links.txt")))[0]

		var sources []string

		for _, file := range glob(filepath.Join(program, "*")) {
			if ext := filepath.Ext(file); ext == ".for" || ext == ".f90" {
				sources = append(sources, stem(file))
			}
		}

		orders := []string{strings.Join(sources, ",")}
		if compiles := filepath.Join(program, "compiles.txt"); exists(compiles) {
			orders = strings.Fields(strings.SplitN(read(compiles), "\n", 2)[0])
		}

		for _, v := range variants {
			d := "M_" + name + v.suffix
			fmt.Fprintf(&ex, "CREATE/DIRECTORY %[1]s[%[2]s]\nSET DEFAULT %[1]s[%[2]s]\n", device, d)

			for _, file := range glob(filepath.Join(program, "*")) {
				base := filepath.Base(file)
				if base == "links.txt" || base == "compiles.txt" || strings.HasSuffix(base, ".out") {
					continue
				}

				copyFile(&ex, file, strings.ToUpper(base))
			}

			for _, files := range orders {
				fmt.Fprintf(&ex, "FORTRAN%s %s\n", v.qualifier, files)
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
$   LINK/NOMAP/EXECUTABLE='P2' 'P3','ROOT'[FTNRTL]F90R/OPTIONS
$   IF F$SEARCH("''P2'.IN") .NES. "" THEN DEFINE/USER_MODE SYS$INPUT 'P2'.IN
$   RUN 'P2'
$   SHOW SYMBOL $STATUS
$ ENDSUBROUTINE
$ !
$ ! RECORDS file: writes each of a file's records as it is.
$ !
$ RECORDS: SUBROUTINE
$   OPEN/READ RECORDS 'P1'
$ NEXT:
$   READ/END_OF_FILE=DONE RECORDS LINE
$   WRITE SYS$OUTPUT LINE
$   GOTO NEXT
$ DONE:
$   CLOSE RECORDS
$ ENDSUBROUTINE
`)

	fmt.Fprintf(&ex, "SET DEFAULT %s[000000]\nDISMOUNT %s\nEXIT\n", device, device)

	fmt.Fprintf(&out, `! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/f90r/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     mkdir -p %[3]s/mar %[3]s/fcalc
!     govax console < testdata/f90r/copyout.cmd
!
MOUNT/NOWRITE %[1]s "%[2]s"
COPY/QUIET %[1]s[000000]F90R.LOG "%[3]s/f90r.log"/HOST
COPY/QUIET %[1]s[FTNRTL]FTNRTL.MAP "%[3]s/ftnrtl.map"/HOST
COPY/QUIET/BINARY %[1]s[FTNRTL]FTNRTL.EXE "%[3]s/ftnrtl.exe"/HOST
COPY/QUIET %[1]s[FTNRTL]FTNRTL.OPT "%[3]s/mar/ftnrtl.opt"/HOST
`, device, volume, results)

	for _, m := range modules {
		lower := strings.ToLower(m)
		fmt.Fprintf(&out, "COPY/QUIET %s[FTNRTL]%s.OBJ \"%s/%s.obj\"/HOST\n", device, m, results, lower)
		fmt.Fprintf(&out, "COPY/QUIET %s[FTNRTL]%s.MAR \"%s/mar/%s.mar\"/HOST\n", device, m, results, lower)
	}

	// fcalc's objects and image, from its plain build, so govax's MACRO and LINK can be
	// held to VMS's on the compiler's own output.
	for _, m := range fcalcModules() {
		lower := strings.ToLower(m)
		fmt.Fprintf(&out, "COPY/QUIET %s[FCALC]%s.OBJ \"%s/fcalc/%s.obj\"/HOST\n", device, m, results, lower)
		fmt.Fprintf(&out, "COPY/QUIET %s[FCALC]%s.MAR \"%s/fcalc/%s.mar\"/HOST\n", device, m, results, lower)
	}

	fmt.Fprintf(&out, "COPY/QUIET/BINARY %s[FCALC]FCALC.EXE \"%s/fcalc/fcalc.exe\"/HOST\n", device, results)
	fmt.Fprintf(&out, "DISMOUNT %s\nEXIT\n", device)

	write := func(name, text string) {
		check(os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
	}

	write("exchange.cmd", ex.String())
	write("f90r.com", com.String())
	write("copyout.cmd", out.String())
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// fcalcModules are fcalc's modules in compiles.txt's order, the main program last.
func fcalcModules() []string {
	return strings.Fields(read("testdata/fcalc/compiles.txt"))
}

// writeFcalc writes fcalc's part: each variant in a directory of its own, with the
// files of fcalc's commands the scripts run beside it.
func writeFcalc(ex, com *strings.Builder) {
	const fc = "testdata/fcalc"

	modules := fcalcModules()
	scripts := glob(fc + "/*.in")
	files := glob(fc + "/files/*")

	var expected []string
	for _, file := range glob(fc + "/expected/*") {
		expected = append(expected, strings.ToUpper(filepath.Base(file)))
	}

	// The main program first in LINK's list, so the image is named for it.
	link := modules[len(modules)-1] + "," + strings.Join(modules[:len(modules)-1], ",")

	for _, v := range fcalcVariants {
		d := "FCALC" + v.suffix
		fmt.Fprintf(ex, "CREATE/DIRECTORY %[1]s[%[2]s]\nSET DEFAULT %[1]s[%[2]s]\n", device, d)

		for _, m := range modules {
			copyFile(ex, fc+"/"+strings.ToLower(m)+".f90", m+".F90")
		}

		for _, file := range append(append(scripts, files...), fc+"/oneshot.com") {
			copyFile(ex, file, strings.ToUpper(filepath.Base(file)))
		}

		for _, m := range modules {
			fmt.Fprintf(ex, "FORTRAN/G_FLOATING%s %s\n", v.qualifier, m)
		}

		fmt.Fprintf(com, `$ WRITE SYS$OUTPUT "==== %[1]s"
$ SET DEFAULT 'ROOT'[%[1]s]
$ MACRO/NOLIST %[2]s
$ LINK/NOMAP/EXECUTABLE=FCALC %[3]s,'ROOT'[FTNRTL]F90R/OPTIONS
$ SHOW SYMBOL $STATUS
`, d, strings.Join(modules, ","), link)

		for _, script := range scripts {
			name := stem(script)
			fmt.Fprintf(com, `$ WRITE SYS$OUTPUT "==== %[1]s.%[2]s"
$ DEFINE/USER_MODE SYS$INPUT %[2]s.IN
$ RUN FCALC
$ SHOW SYMBOL $STATUS
`, d, name)
		}

		// Not TYPE: fcalc's files have FORTRAN carriage control, FTNRTL's default, and
		// TYPE would take each record's first character for it.
		for _, file := range expected {
			fmt.Fprintf(com, "$ WRITE SYS$OUTPUT \"==== %s.%s\"\n$ CALL RECORDS %s\n", d, stem(file), file)
		}

		fmt.Fprintf(com, `$ WRITE SYS$OUTPUT "==== %[1]s.ONESHOT"
$ @ONESHOT
$ SHOW SYMBOL $STATUS
`, d)
	}
}
