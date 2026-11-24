package dclcmd

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/sysdef"
)

// starletFinder finds only the definition modules, as fortranModules does when
// no directory has a module file.
type starletFinder struct{ gFloating bool }

func (f starletFinder) Find(name string) ([]string, string, bool, []string) {
	lines, spec, ok := starletModule(name, f.gFloating)
	if !ok {
		return nil, "", false, []string{name}
	}

	return lines, spec, true, nil
}

// TestStarletModules holds every definition module to compiling as a module to
// USE, in both REAL*8 formats, with no object a user would need.
func TestStarletModules(t *testing.T) {
	for _, g := range []bool{false, true} {
		for _, n := range sysdef.Names() {
			name := starletPrefix + strings.TrimPrefix(n, "$")

			lines, _, ok := starletModule(name, g)
			if !ok {
				t.Errorf("%s (G %v) doesn't compile", name, g)

				continue
			}

			for _, l := range lines {
				if strings.HasPrefix(l, "!GOVAX$ OBJECT") {
					t.Errorf("%s has an object", name)
				}
			}
		}
	}

	if _, _, ok := starletModule("STARLET_NOSUCH", false); ok {
		t.Error("STARLET_NOSUCH found")
	}

	if _, _, ok := starletModule("SSDEF", false); ok {
		t.Error("SSDEF found without its prefix")
	}
}

// TestUseStarlet compiles a Fortran 90 program that USEs two definition
// modules: a constant of $SSDEF's and a system service of $SYSSRVNAM's, its
// type INTEGER*4 from the module's text.
func TestUseStarlet(t *testing.T) {
	text := `program p
  use starlet_ssdef
  use starlet_syssrvnam, only: sys$setast
  implicit none
  integer :: status
  status = sys$setast(%val(1))
  if (status == ss$_wasset) print *, 'was set'
  if (status /= ss$_normal) print *, status
end program p`

	r := fortran.Compile(fortran.Options{F90: true, F90Given: true, Modules: starletFinder{}},
		fortran.File{Spec: "P.F90", Name: "P", Free: true, Lines: strings.Split(text, "\n")})
	if r.Errors != 0 {
		for _, m := range r.Messages {
			t.Errorf("%d: %s %s", m.Pos.Line, m.Ident, m.Text)
		}

		t.FailNow()
	}

	macro := strings.Join(r.MACRO, "\n")

	for _, want := range []string{"G^SYS$SETAST", "#9"} {
		if !strings.Contains(macro, want) {
			t.Errorf("no %q in\n%s", want, macro)
		}
	}
}
