package fortran

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// corpus90 is testdata/fortran90, from this package's directory.
const corpus90 = "../../testdata/fortran90"

// programs90 are compare.py's PROGRAMS90: each program's files, relative to the
// corpus, its main program's first. compare.py is read rather than a list kept here,
// so that a program added to the witness is held to /STANDARD=F90 too.
func programs90(t *testing.T) map[string][]string {
	t.Helper()

	text, err := os.ReadFile("../../testdata/fortran/gfortran/compare.py")
	if err != nil {
		t.Fatal(err)
	}

	block := regexp.MustCompile(`(?s)PROGRAMS90 = \{(.*?)\n\}`).FindSubmatch(text)
	if block == nil {
		t.Fatal("compare.py has no PROGRAMS90")
	}

	programs := map[string][]string{}
	entry := regexp.MustCompile(`(?s)"(\w+)": \[(.*?)\]`)
	file := regexp.MustCompile(`"([^"]+)"`)

	for _, m := range entry.FindAllSubmatch(block[1], -1) {
		for _, f := range file.FindAllSubmatch(m[2], -1) {
			programs[string(m[1])] = append(programs[string(m[1])], string(f[1]))
		}
	}

	if len(programs) < 50 {
		t.Fatalf("read only %d programs from PROGRAMS90", len(programs))
	}

	return programs
}

// directoryFiles opens INCLUDE files beside the file that includes them.
type directoryFiles string

func (d directoryFiles) OpenInclude(name string, _ *source.File) (*source.File, error) {
	text, err := os.ReadFile(filepath.Join(string(d), strings.ToLower(name)))
	if err != nil {
		return nil, err
	}

	return &source.File{Spec: name, Lines: strings.Split(strings.TrimRight(string(text), "\n"), "\n")}, nil
}

func (directoryFiles) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no module " + module)
}

// memoryModules is a ModuleFinder over the module texts compiled so far.
type memoryModules map[string][]string

func (m memoryModules) Find(name string) ([]string, string, bool, []string) {
	lines, ok := m[name]

	return lines, name + ".F90$MOD", ok, nil
}

// witnessExtensions are what /STANDARD=F90 flags in the corpus's programs that isn't
// a mistake in its tables, each program's findings as "file:line message": the
// obsolescent features programs use on purpose (the standard's own 8.1 examples share
// terminal statements; alternate returns are tested; CHARACTER*n, a statement function,
// and fixed source form, Fortran 95's newly obsolescent features, are Fortran 90's
// everyday forms), which gfortran -std=f95 accepts as it must. A file another
// includes is named by the file compiled.
var witnessExtensions = map[string][]string{
	"errata": {
		"errata.f90:78 obsolescent in Fortran 95: a statement function",
		"errata.f90:79 obsolescent in Fortran 95: a statement function",
	},
	"zerolen": {
		"zerolen.f90:50 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"zerolen.f90:51 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"autolen": {
		"autolen.f90:75 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"average": {
		"average.f90:8 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"declare": {
		"declare.f90:13 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"generics": {
		"generics.f90:49 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"generics.f90:110 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"hosts": {
		"hosts.f90:38 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"hosts.f90:57 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"interfaces": {
		"interfaces.f90:31 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"interfaces.f90:104 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"internal": {
		"internal.f90:68 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"layout": {
		"layout.f90:9 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"mixed": {
		"fill.for:3 obsolescent in Fortran 95: fixed source form",
		"fill.for:2 obsolescent in Fortran 95: fixed source form",
		"report.f90:2 obsolescent in Fortran 95: fixed source form",
		"mixed.f90:2 obsolescent in Fortran 95: fixed source form",
	},
	"numbers": {
		"numbers.f90:111 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"numbers.f90:124 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"operators": {
		"operators.f90:77 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"operators.f90:37 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"operators.f90:58 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"operators.f90:103 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"optional": {
		"optional.f90:77 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"polys": {
		"table.for:6 obsolescent in Fortran 95: fixed source form",
	},
	"ptrcarr": {
		"ptrcarr.f90:62 obsolescent in Fortran 95: a statement function",
	},
	"spellings": {
		"spellings.f90:9 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"strings": {
		"strings.f90:7 obsolescent in Fortran 95: the CHARACTER*n form of a length",
	},
	"allocstat": {
		"allocstat.f90:54 obsolescent in Fortran 95: an alternate return",
		"allocstat.f90:154 obsolescent in Fortran 95: an alternate return",
		"allocstat.f90:159 obsolescent in Fortran 95: an alternate return",
	},
	"examples": {
		"examples.f90:167 obsolescent in Fortran 95: a DO loop ending at a CALL statement",
		"examples.f90:208 obsolescent in Fortran 95: a terminal statement shared by DO loops",
		"examples.f90:241 obsolescent in Fortran 95: a terminal statement shared by DO loops",
		"examples.f90:241 obsolescent in Fortran 95: a DO loop ending at an assignment",
		"examples.f90:250 obsolescent in Fortran 95: a terminal statement shared by DO loops",
		"examples.f90:250 obsolescent in Fortran 95: a DO loop ending at an assignment",
	},
	"seqargs": {
		"seqargs.f90:82 obsolescent in Fortran 95: an alternate return",
		"seqargs.f90:145 obsolescent in Fortran 95: an alternate return",
		"seqargs.f90:148 obsolescent in Fortran 95: an alternate return",
	},
}

// TestStandardF90Witness compiles each program gfortran -std=f95 builds (compare.py's
// PROGRAMS90) with /STANDARD=F90: it may flag only what witnessExtensions lists.
// A program of several files has the rest compiled first, as gfortran is given them,
// each module's text kept for the files after it.
func TestStandardF90Witness(t *testing.T) {
	standard, err := sema.Conformance{}.With([]string{"F90"})
	if err != nil {
		t.Fatal(err)
	}

	for name, files := range programs90(t) {
		modules := memoryModules{}

		var found []string

		for _, f := range append(slices.Clone(files[1:]), files[0]) {
			path := filepath.Join(corpus90, f)

			text, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			free := strings.HasSuffix(f, ".f90")
			base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			opts := Options{Standard: standard, Includes: directoryFiles(filepath.Dir(path)), Modules: modules}

			if free {
				opts.F90, opts.F90Given = true, true
			}

			r := Compile(opts, File{Spec: filepath.Base(f), Name: strings.ToUpper(base), Free: free,
				Lines: strings.Split(strings.TrimRight(string(text), "\n"), "\n")})

			for _, m := range r.Messages {
				switch {
				case m.Ident == "EXTENSION", m.Ident == "OBSOLESCENT":
					found = append(found, filepath.Base(f)+":"+strconv.Itoa(m.Pos.Line)+" "+m.Text)
				case m.Severity >= diag.Error:
					t.Errorf("%s: %s: %s %s", name, f, m.Ident, m.Text)
				}
			}

			for _, m := range r.Modules {
				modules[m.Name] = m.Lines
			}
		}

		if want := witnessExtensions[name]; !slices.Equal(found, want) {
			t.Errorf("%s flags %q, want %q", name, found, want)
		}
	}
}
