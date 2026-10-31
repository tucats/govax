package console

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// The FORTRAN end-to-end harness (docs/PHASE-75 - FORTRAN first
// light.md, subtask 8): each testdata/fortran/name.for is compiled by
// FORTRAN, assembled by MACRO, linked by LINK against FTNRTL, and run,
// and what it prints is compared with name.out, the expected terminal
// output followed by SHOW SYMBOL's line for $STATUS. FORTRAN, MACRO, and
// LINK must say nothing. To add a program, add the two files; the
// expected output is written by hand from the language's rules, never
// copied from a run.

// fortranPrograms is the harness's directory.
var fortranPrograms = filepath.Join("..", "..", "testdata", "fortran")

// fortranPagingPrograms are the programs also run with paging on
// (the plan's Decision 20: both paging modes for a subset).
var fortranPagingPrograms = []string{"args", "data", "lengths", "overlay", "tables"}

func TestFortranPrograms(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join(fortranPrograms, "*.for"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no programs in %s (%v)", fortranPrograms, err)
	}

	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		for _, source := range sources {
			name := strings.TrimSuffix(filepath.Base(source), ".for")
			if paging && !slices.Contains(fortranPagingPrograms, name) {
				continue
			}

			t.Run(name, func(t *testing.T) {
				text, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}

				want, err := os.ReadFile(strings.TrimSuffix(source, ".for") + ".out")
				if err != nil {
					t.Fatal(err)
				}

				upper := strings.ToUpper(name)
				deleteFilesAfter(t, d, upper)

				if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + upper + ".FOR"},
					rms.TextRecords, splitSource(string(text))); err != nil {
					t.Fatal(err)
				}

				buf.Reset()

				for _, command := range []string{"FORTRAN ", "MACRO ", "LINK "} {
					shareFindRun(t, d, command+upper)

					if out := buf.String(); out != "" {
						t.Fatalf("%s%s said:\n%s", command, upper, out)
					}
				}

				if got := shareRun(t, d, buf, upper); got != string(want) {
					t.Errorf("RUN %s printed:\n%s\nwant:\n%s", upper, got, want)
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
// directory of testdata/fortran/modules one program: its .FOR files are
// compiled and assembled separately, and any other file (an INCLUDE file) is
// only copied to the volume. links.txt has a LINK command's modules on each
// line, and the program, named for its directory, must print name.out however
// they are ordered: which module's contribution to a common block's overlaid
// section comes first, or longest, or holds the values, mustn't matter.
func TestFortranModules(t *testing.T) {
	programs, err := filepath.Glob(filepath.Join(fortranPrograms, "modules", "*"))
	if err != nil || len(programs) == 0 {
		t.Fatalf("no programs in %s (%v)", fortranPrograms, err)
	}

	eachLoader(t, func(t *testing.T, paging bool) {
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

				files, err := os.ReadDir(program)
				if err != nil {
					t.Fatal(err)
				}

				var sources []string

				for _, file := range files {
					if file.Name() == name+".out" || file.Name() == "links.txt" {
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

					if strings.HasSuffix(upper, ".FOR") {
						sources = append(sources, strings.TrimSuffix(upper, ".FOR"))
					}
				}

				buf.Reset()

				commands := []string{"FORTRAN " + strings.Join(sources, ",")}
				for _, source := range sources {
					commands = append(commands, "MACRO "+source)
				}

				for _, command := range commands {
					shareFindRun(t, d, command)

					if out := buf.String(); out != "" {
						t.Fatalf("%s said:\n%s", command, out)
					}
				}

				image := strings.ToUpper(name)

				for _, modules := range strings.Fields(string(links)) {
					shareFindRun(t, d, "LINK/EXECUTABLE="+image+" "+modules)

					if out := buf.String(); out != "" {
						t.Fatalf("LINK %s said:\n%s", modules, out)
					}

					if got := shareRun(t, d, buf, image); got != string(want) {
						t.Errorf("after LINK %s, RUN %s printed:\n%s\nwant:\n%s", modules, image, got, want)
					}
				}
			})
		}
	})
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
		want := "           7   2.500000               0\n"

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
	if want := "           2\n"; !strings.HasPrefix(got, want) {
		t.Errorf("RUN TWOMAIN printed:\n%s\nwant it to start:\n%s", got, want)
	}
}
