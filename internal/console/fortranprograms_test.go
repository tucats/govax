package console

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// The FORTRAN end-to-end harness (docs/planning/PHASE-75 - FORTRAN first
// light.md, subtask 8): each testdata/fortran/name.for is compiled by
// FORTRAN, assembled by MACRO, linked by LINK against FTNRTL, and run,
// and what it prints is compared with name.out, the expected terminal
// output followed by SHOW SYMBOL's line for $STATUS. FORTRAN, MACRO, and
// LINK must say nothing. To add a program, add the two files; the
// expected output is written by hand from the language's rules, never
// copied from a run. A program that reads the terminal has a third file,
// name.in, the lines typed (Phase 80, subtask 14); the terminal doesn't
// echo them, so a reply doesn't show after its prompt.

// fortranPrograms is the harness's directory.
var fortranPrograms = filepath.Join("..", "..", "testdata", "fortran")

// fortranPagingPrograms are the programs also run with paging on
// (the plan's Decision 20: both paging modes for a subset).
var fortranPagingPrograms = []string{"adjust", "aggregates", "align", "altreturns", "args", "astrace", "asts", "automatic", "callback", "chars", "checks", "complex", "complexg", "data", "dialog", "direct", "directives", "entries", "exits", "floatedit", "flow", "formats", "funcs", "funcsg", "handlers", "inquire", "intedit", "internal", "intrinsics", "inventory", "iostmt", "itemlist", "jumps", "ledger", "lengths", "loops", "mathfns", "mathfnsg", "mixed", "mixedg", "numeric", "overlay", "parts", "positions", "powers", "relative", "procs", "reals", "realsg", "recdyn", "records", "recurast", "recurcalc", "recurio", "recurrtl", "recursion", "recurtree", "recurvfe", "rtlcalls", "rtlchars", "runformat", "optalias", "optasts", "optbranch", "optcse", "optchar", "optinvar", "optloop", "optrecur", "optfaults", "optflow", "services", "shared", "stmtfuncs", "stopdo", "stops", "strings", "strrtl", "sysdef", "sysown", "sysroutines", "tables", "text", "typeless", "unformatted", "useropen", "vfe", "vmstour"}

// fortranKernels are the optimizer's kernels (docs/planning/PHASE-84 - FORTRAN
// optimizer.md), compute programs that spend their time in their own loops, as
// the corpus's programs don't, so the counts below mean something. A kernel
// without an expected output (its value is a floating chain, not to be worked out
// by hand) must print the same both ways.
var fortranKernels = filepath.Join("..", "..", "testdata", "opt84", "kernels")

// optimizerModes are the two ways every program is compiled, each on a console of
// its own, since a program may leave files behind it.
var optimizerModes = []struct{ name, qualifier string }{
	{"noopt", "/NOOPTIMIZE"},
	{"opt", "/OPTIMIZE"},
}

// recursiveModes are optimizerModes with every subprogram compiled /RECURSIVE
// (docs/planning/PHASE-89 - reentrant FORTRAN.md, subtask 7), its locals and the compiler's
// cells in each activation's frame. A conforming FORTRAN-77 program doesn't depend on
// an unsaved local keeping its value from one call to the next (section 4.12's SAVE),
// so each must print what it prints compiled the usual way, but for
// recursiveDifferences.
var recursiveModes = []struct{ name, qualifier string }{
	{"recursive-noopt", "/RECURSIVE/NOOPTIMIZE"},
	{"recursive-opt", "/RECURSIVE/OPTIMIZE"},
}

// recursiveDifferences are the programs that print something else compiled
// /RECURSIVE, each with the reason. A program that reads what a local held at the
// last call reads whatever is on the stack, which the two optimizer modes needn't
// leave the same, so their outputs aren't compared with each other either, and one may
// print what it should by chance: what the last call left can still be there.
var recursiveDifferences = map[string]string{
	"align": "prints MOD(%LOC(RC), 16) of SECOND's record, which in the frame is on a " +
		"longword, not the 16 bytes /ALIGN gives it in $LOCAL (subtask 4's frame alignment)",
	"directives": "prints a common block's address, and its ACCVIO's, which move up as " +
		"the subprograms' locals leave $LOCAL for the frame",
	"optasts": "REENT's AST is another activation, with its own STAT, so the first one " +
		"prints its own 10 rather than the 20 the AST stored: what the frame is for",
	"optflow": "WALK's ENTRY AGAIN reads P, which WALK's last call set: an unsaved local " +
		"keeping its value, undefined in FORTRAN-77 (section 4.12)",
	"optforward": "KEEP returns what its last call stored in its value, undefined in " +
		"FORTRAN-77, which deadcode.go keeps working only for static storage",
	"optreg": "E1's ENTRY E2 reads L, which E1's last call set: an unsaved local keeping " +
		"its value, undefined in FORTRAN-77 (section 4.12)",
}

// runCounts is what one program's RUN cost: its instructions, and the memory's
// reads and writes. Those are mostly data references, but the reads have a part
// that goes with where code falls and not what it does: instruction fetches that
// refill the fetch window at a new page, and the page table entries that takes.
type runCounts struct {
	instructions, reads, writes int64
}

// countedRun is shareRun, and what the run cost.
func countedRun(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer, image string) (string, runCounts) {
	t.Helper()

	_, r0, w0, mr0, mw0 := c.Engine.Memory().Stats()

	out := shareRun(t, d, buf, image)

	_, r1, w1, mr1, mw1 := c.Engine.Memory().Stats()

	// Each run starts a new run count (Engine.BeginRun), so this is the image's
	// alone.
	return out, runCounts{int64(c.Engine.RunInstructionCount()), r1 - r0 + mr1 - mr0, w1 - w0 + mw1 - mw0}
}

// logGain logs what /OPTIMIZE did for one program (go test -v shows it), the
// measure Phase 84's passes are held to.
func logGain(t *testing.T, name string, before, after runCounts) {
	t.Helper()

	t.Logf("%-12s instructions %10d to %10d (%+6.1f%%), reads %10d to %10d, writes %10d to %10d",
		name, before.instructions, after.instructions,
		100*(float64(after.instructions)/float64(max(before.instructions, 1))-1),
		before.reads, after.reads, before.writes, after.writes)
}

// codeAddresses are what a program's output says of where its code is: the PC in a
// condition's message, and the last two columns of a line of a traceback.
var codeAddresses = regexp.MustCompile(`PC=[0-9A-F]{8}|[0-9A-F]{8}  [0-9A-F]{8}$`)

// maskCodeAddresses is text with those taken out, line by line.
func maskCodeAddresses(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = codeAddresses.ReplaceAllString(line, "<code>")
	}

	return strings.Join(lines, "\n")
}

func TestFortranPrograms(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join(fortranPrograms, "*.for"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no programs in %s (%v)", fortranPrograms, err)
	}

	kernels, err := filepath.Glob(filepath.Join(fortranKernels, "*.for"))
	if err != nil || len(kernels) == 0 {
		t.Fatalf("no kernels in %s (%v)", fortranKernels, err)
	}

	runFortranCorpus(t, append(sources, kernels...), fortranPagingPrograms)
}

// fortran90Programs is the Fortran 90 corpus (docs/planning/PHASE-99 - Fortran
// 90.md, Decision 16): free-form name.f90 files with their name.out, and name.in
// for one that reads the terminal, as testdata/fortran's.
var fortran90Programs = filepath.Join("..", "..", "testdata", "fortran90")

// fortran90PagingPrograms are the Fortran 90 programs also run with paging on.
var fortran90PagingPrograms = []string{"allocs", "allocstat", "allocvms", "arrayio", "autolen", "autos", "arrays", "autohost", "average", "bytes", "cases", "charfns", "clockrand", "clockvms", "constants", "cplx16", "declare", "elements", "examples", "forms", "generics", "hostopt", "hosts", "inquire", "interfaces", "internal", "keywords", "kinds", "layout", "loops", "madetext", "matrix", "moddata", "modaccess", "modprocs", "numbers", "numfns", "model", "modelvms", "operators", "optional", "overlap", "pointers", "precise", "ptralloc", "ptrallocvms", "ptrarrays", "ptrclear", "ptrcarr", "ptrcomps", "ptrdum", "ptrres", "ptrstd", "rampvms", "ramps", "recurhost", "reduce", "rtemps", "selects", "seqargs", "seqshare", "seqtypes", "seqvms", "shapeargs", "shaped", "shapes", "sieve", "smooth", "solver", "sorter", "sparse", "spellings", "squares", "stats", "strings", "tokens", "transform", "typeargs", "typedyn", "typeio", "typenml", "geometry", "stock", "accounts", "values", "vectors", "where"}

// TestFortran90Programs runs the Fortran 90 corpus as TestFortranPrograms runs
// the old one, in every mode: FORTRAN finds each NAME.F90 on the volume and reads
// it in free form, with /F90, by its type. It needs the console, as the old
// corpus's harness does, because each program is assembled, linked against
// FTNRTL, and run on the machine.
func TestFortran90Programs(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join(fortran90Programs, "*.f90"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no programs in %s (%v)", fortran90Programs, err)
	}

	runFortranCorpus(t, sources, fortran90PagingPrograms)
}

// runFortranCorpus compiles, assembles, links, and runs each source (a .for or
// .f90 file), with paging off and on (for pagingPrograms), in optimizerModes and
// recursiveModes, and holds what each prints to its .out file and to what it
// printed the other ways.
func runFortranCorpus(t *testing.T, sources, pagingPrograms []string) {
	t.Helper()

	eachLoader(t, func(t *testing.T, paging bool) {
		printed := map[string]string{}
		counts := map[string]runCounts{}

		recursivePrinted := map[string]string{}

		for _, mode := range append(optimizerModes, recursiveModes...) {
			recursive := strings.HasPrefix(mode.name, "recursive")

			t.Run(mode.name, func(t *testing.T) {
				d, c, buf := shareFindConsole(t, paging)

				for _, source := range sources {
					// The file's type goes to the volume with it: FORTRAN
					// reads a .F90 file in free form.
					ext := filepath.Ext(source)
					base := strings.TrimSuffix(source, ext)

					name := filepath.Base(base)
					if paging && !slices.Contains(pagingPrograms, name) {
						continue
					}

					t.Run(name, func(t *testing.T) {
						text, err := os.ReadFile(source)
						if err != nil {
							t.Fatal(err)
						}

						want, err := os.ReadFile(base + ".out")
						if err != nil && (filepath.Dir(source) != fortranKernels || !os.IsNotExist(err)) {
							t.Fatal(err)
						}

						hasWant := err == nil

						upper := strings.ToUpper(name)
						deleteFilesAfter(t, d, upper)

						if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + upper + strings.ToUpper(ext)},
							rms.TextRecords, splitSource(string(text))); err != nil {
							t.Fatal(err)
						}

						buf.Reset()

						for _, command := range []string{"FORTRAN" + mode.qualifier + " ", "MACRO ", "LINK "} {
							shareFindRun(t, d, command+upper)

							if out := buf.String(); out != "" {
								t.Fatalf("%s%s said:\n%s", command, upper, out)
							}
						}

						input, err := os.ReadFile(base + ".in")
						if err != nil && !os.IsNotExist(err) {
							t.Fatal(err)
						}

						c.In = strings.NewReader(string(input))

						got, cost := countedRun(t, d, c, buf, upper)

						// /OPTIMIZE moves the code, so what a program prints that
						// says where its code is can't be the same.
						matches := got == string(want)
						if mode.qualifier != "/NOOPTIMIZE" {
							matches = maskCodeAddresses(got) == maskCodeAddresses(string(want))
						}

						_, differs := recursiveDifferences[name]

						if hasWant && !matches && !(recursive && differs) {
							t.Errorf("RUN %s printed:\n%s\nwant:\n%s", upper, got, want)
						}

						if recursive && differs {
							return
						}

						if recursive {
							if before, ok := recursivePrinted[name]; !ok {
								recursivePrinted[name] = got
							} else if maskCodeAddresses(got) != maskCodeAddresses(before) {
								t.Errorf("RUN %s printed, compiled %s:\n%s\nand without /OPTIMIZE:\n%s", upper, mode.qualifier, got, before)
							}

							if !hasWant && !differs && maskCodeAddresses(got) != maskCodeAddresses(printed[name]) {
								t.Errorf("RUN %s printed, compiled %s:\n%s\nand compiled the usual way:\n%s", upper, mode.qualifier, got, printed[name])
							}

							return
						}

						if before, ok := printed[name]; !ok {
							printed[name], counts[name] = got, cost
						} else {
							if maskCodeAddresses(got) != maskCodeAddresses(before) {
								t.Errorf("RUN %s printed, with /OPTIMIZE:\n%s\nand without it:\n%s", upper, got, before)
							}

							logGain(t, name, counts[name], cost)
						}
					})
				}
			})
		}
	})
}

// deleteFilesAfter deletes every file named name, of any type, when the test
// ends: the programs share one small volume, and a program's source, MACRO
// file, object, and image would soon fill it.
func deleteFilesAfter(t *testing.T, d *Dispatcher, name string) {
	t.Helper()
	t.Cleanup(func() { shareFindRun(t, d, "DELETE "+name+".*;*") })
}

// TestFortranModules is the harness for programs of several files, each
// directory of testdata/fortran/modules one program: its .FOR (and .F90) files are
// compiled and assembled separately, and any other file (an INCLUDE file) is
// only copied to the volume. links.txt has a LINK command's modules on each
// line, and the program, named for its directory, must print name.out however
// they are ordered: which module's contribution to a common block's overlaid
// section comes first, or longest, or holds the values, mustn't matter; nor whether
// it is compiled /RECURSIVE (recursiveModes). A program that reads the terminal
// has name.in, the lines typed.
//
// A program of Fortran 90 modules has compiles.txt too (docs/planning/PHASE-106 -
// Fortran 90 modules.md, subtask 7): each line an order its files may be compiled
// in, a FORTRAN command for each word ("A B,C D" is three commands, the second
// compiling B and C), so each USE finds its module in this compilation or in a
// module file an earlier command wrote. Every order is followed by every link;
// no module file is left from the order before. Without compiles.txt, one
// FORTRAN command compiles every file.
func TestFortranModules(t *testing.T) {
	programs, err := filepath.Glob(filepath.Join(fortranPrograms, "modules", "*"))
	if err != nil || len(programs) == 0 {
		t.Fatalf("no programs in %s (%v)", fortranPrograms, err)
	}

	runFortranModules(t, programs)
}

// TestFortran90Modules is TestFortranModules for the Fortran 90 corpus's
// programs of several files, whose sources may be of either form (.f90 or .for).
func TestFortran90Modules(t *testing.T) {
	programs, err := filepath.Glob(filepath.Join(fortran90Programs, "modules", "*"))
	if err != nil || len(programs) == 0 {
		t.Fatalf("no programs in %s (%v)", fortran90Programs, err)
	}

	runFortranModules(t, programs)
}

// runFortranModules runs programs, each a directory, in both paging modes and
// every way FORTRAN compiles them.
func runFortranModules(t *testing.T, programs []string) {
	t.Helper()

	eachLoader(t, func(t *testing.T, paging bool) {
		for _, mode := range append(optimizerModes, recursiveModes...) {
			t.Run(mode.name, func(t *testing.T) {
				fortranModules(t, programs, paging, mode.qualifier)
			})
		}
	})
}

// fortranModules runs TestFortranModules's programs one way, with FORTRAN's
// qualifier: on a console of its own, as TestFortranPrograms does.
func fortranModules(t *testing.T, programs []string, paging bool, qualifier string) {
	t.Helper()

	d, c, buf := shareFindConsole(t, paging)

	for _, program := range programs {
		name := filepath.Base(program)

		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(program, name+".out"))
			if err != nil {
				t.Fatal(err)
			}

			links, err := os.ReadFile(filepath.Join(program, "links.txt"))
			if err != nil {
				t.Fatal(err)
			}

			compiles, err := os.ReadFile(filepath.Join(program, "compiles.txt"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}

			input, err := os.ReadFile(filepath.Join(program, name+".in"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}

			files, err := os.ReadDir(program)
			if err != nil {
				t.Fatal(err)
			}

			var sources []string

			for _, file := range files {
				switch file.Name() {
				case name + ".out", name + ".in", "links.txt", "compiles.txt":
					continue
				}

				text, err := os.ReadFile(filepath.Join(program, file.Name()))
				if err != nil {
					t.Fatal(err)
				}

				upper := strings.ToUpper(file.Name())
				deleteFilesAfter(t, d, strings.TrimSuffix(upper, filepath.Ext(upper)))

				if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + upper},
					rms.TextRecords, splitSource(string(text))); err != nil {
					t.Fatal(err)
				}

				// A source of either form; FORTRAN finds NAME.FOR or
				// NAME.F90, and reads the second in free form.
				if ext := filepath.Ext(upper); ext == ".FOR" || ext == ".F90" {
					sources = append(sources, strings.TrimSuffix(upper, ext))
				}
			}

			orders := []string{strings.Join(sources, ",")}
			if compiles != nil {
				orders = strings.Split(strings.TrimSpace(string(compiles)), "\n")
			}

			// Module files go before each order, and after the last: there
			// may be none to delete.
			deleteModules := func() { _ = d.Dispatch("DELETE *.F90$MOD;*") }
			t.Cleanup(deleteModules)

			image := strings.ToUpper(name)

			for _, order := range orders {
				// No module file from the order before may stand in for one
				// this order hasn't written yet.
				deleteModules()
				buf.Reset()

				var commands []string
				for _, files := range strings.Fields(order) {
					commands = append(commands, "FORTRAN"+qualifier+" "+files)
				}

				for _, source := range sources {
					commands = append(commands, "MACRO "+source)
				}

				for _, command := range commands {
					shareFindRun(t, d, command)

					if out := buf.String(); out != "" {
						t.Fatalf("compiled in the order %s, %s said:\n%s", order, command, out)
					}
				}

				for _, modules := range strings.Fields(string(links)) {
					shareFindRun(t, d, "LINK/EXECUTABLE="+image+" "+modules)

					if out := buf.String(); out != "" {
						t.Fatalf("compiled in the order %s, LINK %s said:\n%s", order, modules, out)
					}

					c.In = strings.NewReader(string(input))

					if got := shareRun(t, d, buf, image); got != string(want) {
						t.Errorf("compiled in the order %s, after LINK %s, RUN %s printed:\n%s\nwant:\n%s",
							order, modules, image, got, want)
					}
				}
			}
		})
	}
}

// TestFortranBlockDataLibrary runs a program whose common block's initial
// values come from a BLOCK DATA module in an object library: the EXTERNAL
// statement naming it is the only reference to it (section 4.1), and the
// program's own contribution to the overlaid section, which comes first,
// mustn't hide the values.
func TestFortranBlockDataLibrary(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		files := map[string][]string{
			"BDMAIN": {
				"      PROGRAM BDMAIN",
				"      EXTERNAL BDINIT",
				"      COMMON /TABLE/ I, X, J",
				"      PRINT *, I, X, J",
				"      END",
			},
			"BDINIT": {
				"      BLOCK DATA BDINIT",
				"      COMMON /TABLE/ I, X",
				"      DATA I /7/, X /2.5/",
				"      END",
			},
		}

		for name, lines := range files {
			if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
				rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
				t.Fatal(err)
			}
		}

		buf.Reset()

		for _, command := range []string{
			"FORTRAN BDMAIN", "FORTRAN BDINIT", "MACRO BDMAIN", "MACRO BDINIT",
			"LIBRARY/CREATE/OBJECT BDLIB BDINIT", "LINK BDMAIN,BDLIB/LIBRARY",
		} {
			shareFindRun(t, d, command)

			if out := buf.String(); out != "" {
				t.Fatalf("%s said:\n%s", command, out)
			}
		}

		got := shareRun(t, d, buf, "BDMAIN")
		want := "          7   2.500000               0\n"

		if !strings.HasPrefix(got, want) {
			t.Errorf("RUN BDMAIN printed:\n%s\nwant it to start:\n%s", got, want)
		}
	})
}

// TestFortranMissingBlockData leaves out the module an EXTERNAL statement
// names, which LINK must report, though no code calls it (section 4.1): a
// program would otherwise run with its common block's initial values missing.
func TestFortranMissingBlockData(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	lines := []string{"      PROGRAM NOBD", "      EXTERNAL GONE", "      COMMON /TABLE/ I", "      PRINT *, I", "      END"}
	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]NOBD.FOR"},
		rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "FORTRAN NOBD")
	shareFindRun(t, d, "MACRO NOBD")
	buf.Reset()

	err := d.Dispatch("LINK NOBD")
	if out := buf.String(); !strings.Contains(out, "GONE") {
		t.Errorf("LINK NOBD returned %v and said:\n%s\nwant it to name GONE", err, out)
	}
}

// TestFortranTwoModulesInitialize links two modules that initialize one byte
// of a common block, which one compilation would refuse: the last module
// linked has it.
func TestFortranTwoModulesInitialize(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	files := map[string][]string{
		"TWOMAIN": {"      PROGRAM TWOMAIN", "      COMMON /BOTH/ I", "      PRINT *, I", "      END"},
		"TWOA":    {"      BLOCK DATA TWOA", "      COMMON /BOTH/ I", "      DATA I /1/", "      END"},
		"TWOB":    {"      BLOCK DATA TWOB", "      COMMON /BOTH/ I", "      DATA I /2/", "      END"},
	}

	for name, lines := range files {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, "FORTRAN "+name)
		shareFindRun(t, d, "MACRO "+name)
	}

	buf.Reset()
	shareFindRun(t, d, "LINK TWOMAIN,TWOA,TWOB")

	if out := buf.String(); out != "" {
		t.Fatalf("LINK said:\n%s", out)
	}

	got := shareRun(t, d, buf, "TWOMAIN")
	if want := "          2\n"; !strings.HasPrefix(got, want) {
		t.Errorf("RUN TWOMAIN printed:\n%s\nwant it to start:\n%s", got, want)
	}
}
