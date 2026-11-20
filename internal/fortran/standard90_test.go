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
// a mistake in its tables: DEC's extensions a program uses on purpose, each program's
// findings as "file:line what". The obsolescent features are subtask 3's.
var witnessExtensions = map[string][]string{}

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
				case m.Ident == "EXTENSION":
					found = append(found, filepath.Base(f)+":"+strconv.Itoa(m.Pos.Line)+" "+strings.TrimPrefix(m.Text, "extension to Fortran 90: "))
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
