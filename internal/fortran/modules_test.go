package fortran_test

import (
	"fmt"
	"slices"
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
		"!GOVAX$ OBJECT",
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

	routine := func(lines []string) string { return routineCode(lines, "AREA") }

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

// sourceComment is a MACRO file's comment showing a source line, and lineNote
// a comment's mention of one.
var (
	sourceComment = regexp.MustCompile(`^;\s+\d+  `)
	lineNote      = regexp.MustCompile(`\(line \d+\)`)
)

// routineCode is the MACRO file's code from the routine name's storage map on,
// without the source lines or mentions of their numbers, which differ between a
// file of the module and its user and a file of the user alone.
func routineCode(lines []string, name string) string {
	var code []string

	for _, line := range lines {
		// A user compiled apart from its module refers to the module's stamp,
		// which a MACRO file that defines it doesn't.
		if strings.Contains(line, "the stamps of the modules used") {
			continue
		}

		if !sourceComment.MatchString(line) {
			code = append(code, lineNote.ReplaceAllString(line, "(line)"))
		}
	}

	text := strings.Join(code, "\n")

	return text[strings.Index(text, "; Storage map for "+name+","):]
}

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

// TestModuleFormat checks MODFORMAT (finding 10): a user compiled with the other
// REAL*8 format than a module with a REAL*8 or COMPLEX*16 entity, of any kind
// (a variable, a component, a dummy argument), and none for a module whose
// entities don't care.
func TestModuleFormat(t *testing.T) {
	g := func(o *fortran.Options) { o.GFloating = true }
	user := "program p\n  use m\nend program"

	for _, module := range []string{
		"module m\n  real(8) :: x\nend module",
		"module m\n  type t\n    complex(8) :: z\n  end type\nend module",
		"module m\ncontains\n  subroutine s(d)\n    double precision :: d\n  end subroutine\nend module",
	} {
		alone := compileFree("M", module, nil, g)
		if alone.Errors != 0 {
			t.Fatalf("messages: %v", messageList(alone))
		}

		r := compileFree("P", user, moduleFiles{"M": moduleText(t, alone, "M")})
		want := "2:7 MODFORMAT the module M was compiled /G_FLOATING, this unit /NOG_FLOATING: its REAL*8 and COMPLEX*16 entities would be read in the wrong format"

		if got := messageList(r); len(got) != 1 || got[0] != want {
			t.Errorf("%s:\n%v", module, got)
		}

		if same := compileFree("P", user, moduleFiles{"M": moduleText(t, alone, "M")}, g); same.Errors != 0 {
			t.Errorf("the same format: %v", messageList(same))
		}
	}

	alone := compileFree("M", "module m\n  real(4) :: x\n  complex(4) :: z\nend module", nil, g)
	if r := compileFree("P", user, moduleFiles{"M": moduleText(t, alone, "M")}); r.Errors != 0 {
		t.Errorf("no REAL*8: %v", messageList(r))
	}
}

// TestModuleStamps checks the stamps (finding 9): a module with variables or
// procedures says OBJECT and defines its stamp, a global symbol carrying its
// text's digest; a user compiled apart refers to it with .EXTRN, one in the
// same file doesn't; and a module of only types and constants asks for none.
func TestModuleStamps(t *testing.T) {
	alone := compileFree("CONSTS", constsModule, nil)
	text := moduleText(t, alone, "CONSTS")
	stamp := sema.StampName("CONSTS", alone.Modules[0].Digest)

	if !strings.HasPrefix(stamp, "$MOD$CONSTS.") || len(stamp) != len("$MOD$CONSTS.")+8 {
		t.Fatalf("stamp %q", stamp)
	}

	if macro := strings.Join(alone.MACRO, "\n"); !strings.Contains(macro, stamp+" == 0") {
		t.Errorf("no definition of %s:\n%s", stamp, macro)
	}

	user := compileFree("AREA", constsUser, moduleFiles{"CONSTS": text})
	if macro := strings.Join(user.MACRO, "\n"); !strings.Contains(macro, "\t.EXTRN\t"+stamp+"\t") {
		t.Errorf("no reference to %s:\n%s", stamp, macro)
	}

	both := compileFree("AREA", constsModule+"\n"+constsUser, nil)
	if macro := strings.Join(both.MACRO, "\n"); strings.Contains(macro, ".EXTRN\t$MOD$") {
		t.Errorf("a stamp referred to in its own file:\n%s", macro)
	}

	types := compileFree("KINDS", "module kinds\n  integer, parameter :: dp = 8\n  type pair\n    integer a, b\n  end type\nend module", nil)
	if slices.Contains(moduleText(t, types, "KINDS"), "!GOVAX$ OBJECT") {
		t.Errorf("an OBJECT line for a module of types")
	}

	kuser := compileFree("Q", "program q\n  use kinds\n  type(pair) :: p\n  p%a = dp\nend program", moduleFiles{"KINDS": moduleText(t, types, "KINDS")})
	if kuser.Errors != 0 {
		t.Fatalf("messages: %v", messageList(kuser))
	}

	if macro := strings.Join(kuser.MACRO, "\n"); strings.Contains(macro, "$MOD$KINDS.") {
		t.Errorf("a stamp asked of a module with no object:\n%s", macro)
	}

	// A long module name's stamp is the hashed form, the same both sides.
	long := sema.StampName("A_VERY_LONG_MODULE_NAME_INDEED", alone.Modules[0].Digest)
	if len(long) != 31 || long != sema.LongName("$MOD$A_VERY_LONG_MODULE_NAME_INDEED."+alone.Modules[0].Digest[:8]) {
		t.Errorf("long stamp %q", long)
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

// procsModule has module procedures of every kind of interface: a generic name
// and an operator of module procedures, an external procedure's interface
// body, RESULT, an assumed-shape array, a passed length, OPTIONAL, a dummy
// procedure with its interface, an alternate return, and a module procedure
// with an internal procedure of its own.
const procsModule = `module ops
  type vec
    real :: x, y
  end type vec
  integer :: calls = 0
  interface swap
    module procedure swapr, swapi
  end interface
  interface operator(+)
    module procedure addvec
  end interface
  interface
    subroutine ext(n)
      integer n
    end subroutine ext
  end interface
contains
  subroutine swapr(a, b)
    real, intent(inout) :: a, b
    real t
    t = a; a = b; b = t
    call note
  contains
    subroutine note
      calls = calls + 1
    end subroutine note
  end subroutine swapr
  subroutine swapi(i, j)
    integer, intent(inout) :: i, j
    integer t
    t = i; i = j; j = t
  end subroutine swapi
  function addvec(u, v) result(w)
    type(vec), intent(in) :: u, v
    type(vec) w
    w = vec(u%x + v%x, u%y + v%y)
  end function addvec
  recursive integer function total(a, f, s, k) result(n)
    real a(:)
    character(len=*) s
    integer, optional :: k
    interface
      real function f(x)
        real x
      end function f
    end interface
    n = size(a) + len(s)
  end function total
  subroutine pick(i, *, *)
    integer i
    if (i > 0) return 1
    return 2
  end subroutine pick
end module ops`

// TestModuleProcedureText checks the text's procedures (subtask 3): the
// generic and operator blocks with MODULE PROCEDURE, the external procedure's
// interface block, and after CONTAINS each module procedure's interface with
// its LINK line, its internal procedure left out.
func TestModuleProcedureText(t *testing.T) {
	r := compileFree("OPS", procsModule, nil)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	text := strings.Join(moduleText(t, r, "OPS"), "\n")

	for _, want := range []string{
		"  INTERFACE\n    SUBROUTINE EXT(N)\n      INTEGER(KIND=4) :: N\n    END SUBROUTINE EXT\n  END INTERFACE\n",
		"  INTERFACE SWAP\n    MODULE PROCEDURE SWAPR, SWAPI\n  END INTERFACE\n",
		"  INTERFACE OPERATOR(+)\n    MODULE PROCEDURE ADDVEC\n  END INTERFACE\nCONTAINS\n",
		"!GOVAX$ LINK SWAPR OPS.SWAPR\n  SUBROUTINE SWAPR(A, B)\n    REAL(KIND=4), INTENT(INOUT) :: A\n" +
			"    REAL(KIND=4), INTENT(INOUT) :: B\n  END SUBROUTINE SWAPR\n",
		"  FUNCTION ADDVEC(U, V) RESULT(W)\n    TYPE(VEC), INTENT(IN) :: U\n    TYPE(VEC), INTENT(IN) :: V\n    TYPE(VEC) :: W\n",
		"  RECURSIVE FUNCTION TOTAL(A, F, S, K) RESULT(N)\n    REAL(KIND=4) :: A(1:)\n    INTERFACE\n      FUNCTION F(X)\n" +
			"        REAL(KIND=4) :: X\n        REAL(KIND=4) :: F\n      END FUNCTION F\n    END INTERFACE\n" +
			"    CHARACTER(LEN=*) :: S\n    INTEGER(KIND=4), OPTIONAL :: K\n    INTEGER(KIND=4) :: N\n  END FUNCTION TOTAL\n",
		"  SUBROUTINE PICK(I, *, *)\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no\n%s\nin\n%s", want, text)
		}
	}

	if strings.Contains(text, "NOTE") {
		t.Errorf("an internal procedure is in the text:\n%s", text)
	}

	// The optimizer's checks hold for a module procedure's internal procedure
	// reaching the module's variable, its host's host's.
	if o := compileFree("OPS", procsModule, nil, func(o *fortran.Options) { o.Optimize = true }); o.Errors != 0 {
		t.Errorf("/OPTIMIZE: %v", messageList(o))
	}

	// The module's code: each module procedure at its global .ENTRY, its
	// internal procedure at a local label, called with no hidden argument.
	code := strings.Join(r.MACRO, "\n")
	for _, want := range []string{
		"\t.ENTRY\tOPS.SWAPR,", "\t.ENTRY\tOPS.TOTAL,", "\tCALLS\t#0,OPS.SWAPR.NOTE",
		"OPS.SWAPR.NOTE:\n\t.WORD\t^M<>\t\t\t; internal procedure NOTE of OPS.SWAPR",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("no %q in\n%s", want, code)
		}
	}
}

// procsUser calls OPS's procedures by every path.
const procsUser = `program p
  use ops
  real x, y, z(3)
  integer i, j
  type(vec) w
  external g
  real g
  x = 1; y = 2; i = 3; j = 4
  call swap(x, y)
  call swap(i, j)
  w = vec(1.0, 2.0) + vec(3.0, 4.0)
  z = 1.5
  call ext(i)
  i = total(z, g, 'abc')
  call pick(i, *10, *20)
10 continue
20 call apply(swapr)
end program p`

// TestModuleProceduresSeparately checks a user of module procedures compiled
// with the module and apart (finding 6), and the calls it makes: by global
// name, the generic and operator resolved to their specifics, a module
// procedure passed as an argument.
func TestModuleProceduresSeparately(t *testing.T) {
	both := compileFree("P", procsModule+"\n"+procsUser, nil)
	if both.Errors != 0 {
		t.Fatalf("messages: %v", messageList(both))
	}

	alone := compileFree("OPS", procsModule, nil)
	user := compileFree("P", procsUser, moduleFiles{"OPS": moduleText(t, alone, "OPS")})

	if user.Errors != 0 {
		t.Fatalf("messages: %v", messageList(user))
	}

	routine := func(lines []string) string { return routineCode(lines, "P") }

	if routine(both.MACRO) != routine(user.MACRO) {
		t.Errorf("the user's code differs:\n%s\n----\n%s", routine(both.MACRO), routine(user.MACRO))
	}

	code := routine(user.MACRO)
	for _, want := range []string{
		"\tCALLS\t#2,G^OPS.SWAPR", "\tCALLS\t#2,G^OPS.SWAPI", "\tCALLS\t#2,G^OPS.ADDVEC", "\tCALLS\t#1,G^EXT",
		"\tCALLS\t#4,G^OPS.TOTAL", "\tCALLS\t#1,G^OPS.PICK", "\tPUSHAB\tG^OPS.SWAPR\n\tCALLS\t#1,G^APPLY",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("no %q in\n%s", want, code)
		}
	}
}

// TestModuleProcedureMessages checks MODULE PROCEDURE's messages, a generic
// name two modules make ambiguous reported at the USE (14.1.2.3), a reference
// checked against a module's interface, and what waits.
func TestModuleProcedureMessages(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string
	}{
		{"modproc", "module m\n  interface g\n    module procedure nothere, x\n  end interface\n  interface\n    module procedure s\n" +
			"  end interface\n  integer x\ncontains\n  subroutine s\n  end subroutine s\nend module", []string{
			"3:22 MODPROCNAME NOTHERE isn't known here, not a module procedure",
			"3:31 MODPROCNAME X is a variable, not a module procedure",
			"6:5 MODPROCGENERIC a MODULE PROCEDURE statement is allowed only in an interface block with a generic name, OPERATOR, or ASSIGNMENT",
		}},
		{"ambiguous", "module a\n  interface swap\n    module procedure sa\n  end interface\ncontains\n  subroutine sa(x)\n    real x\n" +
			"  end subroutine sa\nend module\nmodule b\n  interface swap\n    module procedure sb\n  end interface\ncontains\n" +
			"  subroutine sb(y)\n    real y\n  end subroutine sb\nend module\nprogram p\n  use a\n  use b\nend program", []string{
			"21:7 GENERICAMBIG the specific procedures SA (line 20) and SB of the generic name SWAP can't be told apart by their arguments",
		}},
		{"argument", "module m\ncontains\n  subroutine s(n)\n    integer n\n  end subroutine s\nend module\nprogram p\n  integer k\n  use m\n  call s(1.0)\nend program", []string{
			"9:3 ORDER this USE statement must come before the INTEGER statement at line 8",
			"10:10 ARGTYPE argument 1 of S is a REAL, but its dummy argument N is an INTEGER (line 9)",
		}},
		{"entry", "module m\ncontains\n  subroutine s(x)\n    real x\n    entry e(x)\n  end subroutine s\nend module", []string{
			"3:3 NOTIMPL ENTRY in a module procedure is not implemented yet",
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := messageList(compileFree("A", c.text, nil))
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// TestModuleLongNames checks a module procedure's name past 31 characters: the
// hashed form, the same in the module and a user compiled apart (finding 5).
func TestModuleLongNames(t *testing.T) {
	module := "module very_long_module_name_here\ncontains\n  subroutine a_long_procedure_name_too\n  end subroutine a_long_procedure_name_too\nend module"
	user := "program p\n  use very_long_module_name_here\n  call a_long_procedure_name_too\nend program"

	name := sema.LongName("VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO")

	alone := compileFree("M", module, nil)
	if !strings.Contains(strings.Join(alone.MACRO, "\n"), "\t.ENTRY\t"+name+",") {
		t.Errorf("no .ENTRY %s in\n%s", name, strings.Join(alone.MACRO, "\n"))
	}

	r := compileFree("P", user, moduleFiles{"VERY_LONG_MODULE_NAME_HERE": moduleText(t, alone, "VERY_LONG_MODULE_NAME_HERE")})
	if !strings.Contains(strings.Join(r.MACRO, "\n"), "\tCALLS\t#0,G^"+name) {
		t.Errorf("no call of %s in\n%s", name, strings.Join(r.MACRO, "\n"))
	}
}

// pointersModule has pointers among its variables (docs/planning/PHASE-107 -
// Fortran 90 pointers.md, subtask 2): each is its storage, an address or a
// class S descriptor, and the text says POINTER as it says TARGET.
const pointersModule = `module cursors
  real, target :: slots(4)
  real, pointer :: here
  character(len=3), pointer :: tag
  character(len=8), target :: name
end module cursors`

// pointersUser points the module's pointers and uses them.
const pointersUser = `program walk
  use cursors
  here => slots(2)
  here = 1.0
  tag => name(1:3)
  tag = 'ABC'
  if (associated(here, slots(2))) print *, here, tag
end program walk`

// TestModulePointers holds a module's pointers' text, and a user compiled
// with the module's text from a module file to the code it gets in one file.
func TestModulePointers(t *testing.T) {
	alone := compileFree("CURSORS", pointersModule, nil)
	if alone.Errors != 0 {
		t.Fatalf("messages: %v", messageList(alone))
	}

	text := strings.Join(moduleText(t, alone, "CURSORS"), "\n")
	for _, want := range []string{
		"!GOVAX$ AT HERE 16 4\n  REAL(KIND=4), POINTER :: HERE",
		"!GOVAX$ AT TAG 20 8\n  CHARACTER(LEN=3), POINTER :: TAG",
		"  REAL(KIND=4), TARGET :: SLOTS(1:4)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}

	both := compileFree("WALK", pointersModule+"\n"+pointersUser, nil)
	user := compileFree("WALK", pointersUser, moduleFiles{"CURSORS": moduleText(t, alone, "CURSORS")})

	if both.Errors != 0 || user.Errors != 0 {
		t.Fatalf("messages: %v %v", messageList(both), messageList(user))
	}

	routine := func(lines []string) string { return routineCode(lines, "WALK") }

	if routine(both.MACRO) != routine(user.MACRO) {
		t.Errorf("the user's code differs:\n%s\n----\n%s", routine(both.MACRO), routine(user.MACRO))
	}

	code := strings.Join(user.MACRO, "\n")
	for _, want := range []string{
		"\tMOVAL\tWALK.SLOTS+4,WALK.HERE",
		"\tMOVL\t#^X00004080,@WALK.HERE",
		"\tMOVAB\tWALK.NAME,WALK.TAG+4",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("no %q in\n%s", want, code)
		}
	}
}
