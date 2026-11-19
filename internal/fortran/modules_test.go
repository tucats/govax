package fortran_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Modules (docs/planning/PHASE-106 - Fortran 90 modules.md). These tests compile
// modules and their users by the compiler's driver alone; the programs that run
// are testdata/fortran90's.

// moduleFiles is a ModuleFinder over texts in memory, by module name, as module
// files compiled before would be found.
type moduleFiles map[string][]string

func (m moduleFiles) Find(name string) ([]string, string, bool, []string) {
	lines, ok := m[name]
	if !ok {
		return nil, "", false, []string{"[WORK]" + name + ".F90$MOD", "[SRC]" + name + ".F90$MOD"}
	}

	return lines, "[WORK]" + name + ".F90$MOD", true, nil
}

// compileFree compiles free-form text as one file named name, with /F90 and the
// modules found in found (nil for none).
func compileFree(name, text string, found moduleFiles, edit ...func(*fortran.Options)) fortran.Result {
	opts := noF90Options(name)
	opts.F90, opts.F90Given = true, true

	if found != nil {
		opts.Modules = found
	}

	for _, e := range edit {
		e(&opts)
	}

	return fortran.Compile(opts, fortran.File{Spec: name + ".F90", Name: name, Free: true, Lines: strings.Split(text, "\n")})
}

// messageList is the result's messages, each "line:column IDENT text".
func messageList(r fortran.Result) []string {
	var out []string
	for _, m := range r.Messages {
		out = append(out, fmt.Sprintf("%d:%d %s %s", m.Pos.Line, m.Pos.Column, m.Ident, m.Text))
	}

	return out
}

// moduleText is the text of the module name the result compiled.
func moduleText(t *testing.T, r fortran.Result, name string) []string {
	t.Helper()

	for _, m := range r.Modules {
		if m.Name == name {
			return m.Lines
		}
	}

	t.Fatalf("no module %s: %v", name, messageList(r))

	return nil
}

// constsModule is a module with data of every kind subtask 2 compiles.
const constsModule = `module consts
  real, parameter :: pi = 3.14159, third = 1.0 / 3.0
  double precision, parameter :: e = 2.718281828459045d0
  complex, parameter :: iota = (0.0, 1.0)
  logical(1), parameter :: yes = .true.
  integer(2), parameter :: sizes(2, 2) = reshape((/1, 2, 3, 4/), (/2, 2/))
  character(len=6), parameter :: greeting = 'it''s ' // char(9)
  integer :: calls = 0
  real(8) :: table(10)
  type point
    real x, y
  end type point
  type(point), parameter :: unit_x = point(1.0, 0.0)
  type(point) :: origin
  common /shared/ total, count
  integer count
  namelist /state/ calls, table
end module consts`

// TestModuleText holds a module's text to what Part A's finding 7 says it is:
// declarations with every type's kind written out, constants' values folded,
// and !GOVAX$ lines for the format, the floating format, the section, each
// variable's place, and each type's layout.
func TestModuleText(t *testing.T) {
	r := compileFree("CONSTS", constsModule, nil, func(o *fortran.Options) { o.Version = "1.0-140" })
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	want := []string{
		"!GOVAX$ MODULE FILE 1",
		"!GOVAX$ COMPILER GOVAX FORTRAN V1.0-140",
		"!GOVAX$ SOURCE CONSTS.F90 17-NOV-2026 12:00:00.00",
		"!GOVAX$ FLOATING D",
		"MODULE CONSTS",
		"!GOVAX$ PSECT $MOD$CONSTS SIZE 92 ALIGN 0",
		"!GOVAX$ TYPE POINT SIZE 8 BOUNDARY 4 NATURAL",
		"  TYPE POINT",
		"    REAL(KIND=4) :: X",
		"    REAL(KIND=4) :: Y",
		"  END TYPE POINT",
		"  REAL(KIND=4), PARAMETER :: PI = 3.14159E+00_4",
		"!GOVAX$ FOLDED THIRD",
		"  REAL(KIND=4), PARAMETER :: THIRD = 3.3333334E-01_4",
		"  REAL(KIND=8), PARAMETER :: E = 2.718281828459045E+00_8",
		"  COMPLEX(KIND=4), PARAMETER :: IOTA = (0E+00_4, 1E+00_4)",
		"  LOGICAL(KIND=1), PARAMETER :: YES = .TRUE._1",
		"  INTEGER(KIND=2), PARAMETER :: SIZES(1:2, 1:2) = RESHAPE((/ 1_2, 2_2, 3_2, 4_2 /), (/ 2, 2 /))",
		"  CHARACTER(LEN=6), PARAMETER :: GREETING = 'it''s '//CHAR(9)",
		"  TYPE(POINT), PARAMETER :: UNIT_X = POINT(1E+00_4, 0E+00_4)",
		"!GOVAX$ AT CALLS 0 4",
		"  INTEGER(KIND=4) :: CALLS",
		"!GOVAX$ AT TABLE 4 80",
		"  REAL(KIND=8) :: TABLE(1:10)",
		"!GOVAX$ AT ORIGIN 84 8",
		"  TYPE(POINT) :: ORIGIN",
		"!GOVAX$ COMMON SHARED SIZE 8 ALIGN 0",
		"!GOVAX$ AT TOTAL 0 4",
		"  REAL(KIND=4) :: TOTAL",
		"!GOVAX$ AT COUNT 4 4",
		"  INTEGER(KIND=4) :: COUNT",
		"  COMMON /SHARED/ TOTAL, COUNT",
		"  NAMELIST /STATE/ CALLS, TABLE",
		"END MODULE CONSTS",
	}

	got := moduleText(t, r, "CONSTS")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// The digest leaves the SOURCE line out, so a compilation at another time
	// makes the same file.
	later := compileFree("CONSTS", constsModule, nil, func(o *fortran.Options) {
		o.Version = "1.0-140"
		o.Compiled = o.Compiled.AddDate(0, 0, 1)
	})
	if r.Modules[0].Digest != later.Modules[0].Digest {
		t.Errorf("digests differ: %s and %s", r.Modules[0].Digest, later.Modules[0].Digest)
	}
}

// constsUser uses every entity of constsModule.
const constsUser = `program area
  use consts
  type(point) :: p
  double precision :: d
  calls = calls + 1
  table(2) = pi
  p = unit_x
  origin = p
  d = third
  count = sizes(2, 1)
  print *, pi, e, iota, yes, greeting, d
  write (*, nml=state)
end program area`

// TestModuleUserSeparately checks that a user compiled with the module in its
// own file and with the module's text from a module file makes the same code
// (finding 6: every USE reads the module's text), and the user's view of the
// module's section and the COMMON block.
func TestModuleUserSeparately(t *testing.T) {
	both := compileFree("AREA", constsModule+"\n"+constsUser, nil)
	if both.Errors != 0 {
		t.Fatalf("messages: %v", messageList(both))
	}

	alone := compileFree("CONSTS", constsModule, nil)
	user := compileFree("AREA", constsUser, moduleFiles{"CONSTS": moduleText(t, alone, "CONSTS")})

	if user.Errors != 0 {
		t.Fatalf("messages: %v", messageList(user))
	}

	// The routine's code, without the source lines, whose numbers differ.
	routine := func(lines []string) string {
		var code []string

		for _, line := range lines {
			if !sourceComment.MatchString(line) {
				code = append(code, line)
			}
		}

		text := strings.Join(code, "\n")
		at := strings.Index(text, "; Storage map for AREA")

		return text[at:]
	}

	if routine(both.MACRO) != routine(user.MACRO) {
		t.Errorf("the user's code differs:\n%s\n----\n%s", routine(both.MACRO), routine(user.MACRO))
	}

	text := strings.Join(user.MACRO, "\n")
	for _, want := range []string{
		"\t.PSECT\t$MOD$CONSTS,PIC,OVR,REL,GBL,SHR,NOEXE,RD,WRT,LONG\n$MOD$CONSTS.$BASE:\n\t.BLKB\t92",
		"\t.PSECT\tSHARED,PIC,OVR,REL,GBL,SHR,NOEXE,RD,WRT,LONG\nSHARED.$BASE:\n\t.BLKB\t8",
		"AREA.CALLS = $MOD$CONSTS.$BASE\t",
		"AREA.ORIGIN = $MOD$CONSTS.$BASE+84\t",
		"AREA.COUNT = SHARED.$BASE+4\t",
		"\tINCL\tAREA.CALLS",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}

	// The module's own object holds the initial value; a user's doesn't.
	if !strings.Contains(strings.Join(alone.MACRO, "\n"), "\t.LONG\t0\t\t\t; CALLS = 0") {
		t.Errorf("no initial value in the module's object:\n%s", strings.Join(alone.MACRO, "\n"))
	}

	// THIRD was folded: a REAL*8 gets its REAL value exactly, not its digits'.
	if !strings.Contains(text, "; 0.333333343267440796D0") {
		t.Errorf("THIRD isn't its REAL value:\n%s", text)
	}
}

// sourceComment is a MACRO file's comment showing a source line.
var sourceComment = regexp.MustCompile(`^;\s+\d+  `)

// TestModuleMessages checks what modules and USE report: a module used before
// it is compiled, one that uses itself, what a module's specification part can't
// hold, a use-associated name declared again or put in COMMON or DATA, a local
// name a USE would give, a module with no file, and a damaged one.
func TestModuleMessages(t *testing.T) {
	cases := []struct {
		name, text string
		found      moduleFiles
		want       []string
	}{
		{"later", "program p\n  use m\nend program\nmodule m\n  integer :: i\nend module", nil, []string{
			"2:7 MODLATER the module M is at line 4, after this USE: a module must be compiled before it is used",
		}},
		{"self", "module m\n  use m\nend module", nil, []string{
			"2:7 MODCYCLE the module M uses itself, directly or through other modules",
		}},
		{"assignment", "module m\n  integer :: k\n  k = 1\nend module", nil, []string{
			"3:3 MODSTMT a module's specification part can't hold an assignment or a statement function",
		}},
		{"format", "module m\n10 format (i5)\n  stop\nend module", nil, []string{
			"2:4 MODSTMT a module's specification part can't hold a FORMAT statement",
			"3:3 MODSTMT a module's specification part can't hold an executable statement",
		}},
		{"automatic", "module m\n  integer, automatic :: k\nend module", nil, []string{
			"2:25 AUTOCOMMON K is in module M's variables, so it can't be AUTOMATIC",
		}},
		{"notfound", "program p\n  use gone\nend program", moduleFiles{}, []string{
			"2:7 MODNOTFOUND no module file for GONE; looked in [WORK]GONE.F90$MOD, [SRC]GONE.F90$MOD",
		}},
		{"respecified", "module m\n  integer :: i, j\n  real :: x\n  common /c/ y\nend module\n" +
			"program p\n  use m\n  real i\n  dimension j(3)\n  common /d/ x\n  data y /1.0/\nend program", nil, []string{
			"8:8 USEATTR I comes from the module M, and can't be declared again",
			"9:13 USEATTR J comes from the module M, and can't be declared again",
			"10:14 USECOMMON X comes from the module M, and can't be in COMMON or EQUIVALENCE",
			"11:8 USEATTR Y comes from the module M, and can't be declared again",
		}},
		{"local", "module m\n  integer :: n\n  type t\n    integer k\n  end type\nend module\n" +
			"subroutine s(n)\n  use m\nend subroutine\nprogram p\n  use m\n  type t\n    real r\n  end type\nend program", nil, []string{
			"8:7 USELOCAL N, from the module M, is also a local name of this unit, at line 7",
			"12:8 USEATTR T comes from the module M, and can't be declared again",
		}},
		{"damaged", "program p\n  use m\nend program", moduleFiles{"M": {
			"!GOVAX$ MODULE FILE 1", "MODULE M", "!GOVAX$ PSECT $MOD$M SIZE 4 ALIGN 0", "!GOVAX$ AT I 0 2", "  INTEGER(KIND=4) :: I", "END MODULE M",
		}}, []string{
			"2:7 MODFILEBAD the module file [WORK]M.F90$MOD can't be read, at line 5: I has 4 bytes, not the AT line's 2",
		}},
		{"version", "program p\n  use m\nend program", moduleFiles{"M": {"!GOVAX$ MODULE FILE 9", "MODULE M", "END MODULE M"}}, []string{
			"2:7 MODFILEBAD the module file [WORK]M.F90$MOD can't be read, at line 1: it isn't a module file of this version of govax",
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := messageList(compileFree("A", c.text, c.found))
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// TestModulePsectName checks the hashed form of a module's section past 31
// characters (finding 5).
func TestModulePsectName(t *testing.T) {
	long := "A_MODULE_WITH_A_VERY_LONG_NAME"
	if p := sema.ModulePsect(long); len(p) != 31 || !strings.HasPrefix(p, "$MOD$A_MODULE_WITH_A_V$") || p != sema.ModulePsect(long) {
		t.Errorf("ModulePsect(%s) = %s", long, p)
	}

	if p := sema.ModulePsect("SHORT"); p != "$MOD$SHORT" {
		t.Errorf("ModulePsect(SHORT) = %s", p)
	}
}

// TestModuleGFloating checks a module compiled /G_FLOATING: its text says so,
// and a REAL(8) constant read back has its value's bits in G.
func TestModuleGFloating(t *testing.T) {
	g := func(o *fortran.Options) { o.GFloating = true }
	module := "module gm\n  real(8), parameter :: big = 1.0d300\n  real(8) :: x\nend module"
	user := "program p\n  use gm\n  x = big\nend program"

	r := compileFree("P", module+"\n"+user, nil, g)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	if text := moduleText(t, r, "GM"); text[3] != "!GOVAX$ FLOATING G" {
		t.Errorf("floating line %q", text[3])
	}

	if !strings.Contains(strings.Join(r.MACRO, "\n"), "\tMOVG\tP.$") {
		t.Errorf("no MOVG:\n%s", strings.Join(r.MACRO, "\n"))
	}
}

// TestModuleRecursive checks that a module's variables are static under
// /RECURSIVE, in its section, and a recursive user's too.
func TestModuleRecursive(t *testing.T) {
	r := compileFree("P", "module rm\n  integer :: depth\nend module\nsubroutine s\n  use rm\n  depth = depth + 1\nend subroutine", nil,
		func(o *fortran.Options) { o.Recursive = true })
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	if text := strings.Join(r.MACRO, "\n"); !strings.Contains(text, "S.DEPTH = $MOD$RM.$BASE\t") || !strings.Contains(text, "\tINCL\tS.DEPTH") {
		t.Errorf("depth isn't in the module's section:\n%s", text)
	}
}
