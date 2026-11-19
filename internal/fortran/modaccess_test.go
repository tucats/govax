package fortran_test

import (
	"os"
	"strings"
	"testing"
)

// Access and association (docs/planning/PHASE-106 - Fortran 90 modules.md,
// subtask 4): PUBLIC and PRIVATE, USE's ONLY lists and renames, entities a
// module passes on, and the names two modules give.

// TestModuleAccessText holds the texts of a module with a PRIVATE default and
// of a module that uses it by an ONLY list and passes some of it on: private
// entities named in a PRIVATE statement, a type's private components, and
// each entity from the other module under the name the module knows it by,
// with its FROM line (finding 8).
func TestModuleAccessText(t *testing.T) {
	source, err := os.ReadFile("../../testdata/fortran90/modaccess.f90")
	if err != nil {
		t.Fatal(err)
	}

	r := compileFree("MODACCESS", string(source), nil)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	stacks := strings.Join(moduleText(t, r, "STACKS"), "\n")
	for _, want := range []string{
		"  TYPE CELL\n    PRIVATE\n    REAL(KIND=4) :: VALUE\n  END TYPE CELL",
		"  INTERFACE SWAP\n    MODULE PROCEDURE SWAPR, SWAPI\n  END INTERFACE\n  PRIVATE :: ITEMS, SWAPR, SWAPI\nCONTAINS",
	} {
		if !strings.Contains(stacks, want) {
			t.Errorf("no\n%s\nin\n%s", want, stacks)
		}
	}

	want := []string{
		"MODULE CALC",
		"!GOVAX$ TYPE CELL SIZE 4 BOUNDARY 4 NATURAL",
		"!GOVAX$ FROM CELL STACKS CELL",
		"  TYPE CELL",
		"    PRIVATE",
		"    REAL(KIND=4) :: VALUE",
		"  END TYPE CELL",
		"  INTEGER(KIND=4), PARAMETER :: LIMIT = 5_4",
		"!GOVAX$ PSECT $MOD$STACKS SIZE 404 ALIGN 0",
		"!GOVAX$ FROM DEPTH STACKS DEPTH",
		"!GOVAX$ AT DEPTH 0 4",
		"  INTEGER(KIND=4) :: DEPTH",
		"  PRIVATE :: DEPTH",
		"CONTAINS",
		"!GOVAX$ LINK SUM_TOP CALC.SUM_TOP",
		"  FUNCTION SUM_TOP(N)",
		"    INTEGER(KIND=4), INTENT(IN) :: N",
		"    REAL(KIND=4) :: SUM_TOP",
		"  END FUNCTION SUM_TOP",
		"!GOVAX$ LINK PUSH STACKS.PUSH",
		"!GOVAX$ FROM PUSH STACKS PUSH",
		"  SUBROUTINE PUSH(X)",
		"    REAL(KIND=4), INTENT(IN) :: X",
		"  END SUBROUTINE PUSH",
		"!GOVAX$ LINK TOP STACKS.POP",
		"!GOVAX$ FROM TOP STACKS POP",
		"  FUNCTION TOP()",
		"    REAL(KIND=4) :: TOP",
		"  END FUNCTION TOP",
		"!GOVAX$ LINK MAKE_CELL STACKS.MAKE_CELL",
		"!GOVAX$ FROM MAKE_CELL STACKS MAKE_CELL",
		"  FUNCTION MAKE_CELL(X)",
		"    REAL(KIND=4), INTENT(IN) :: X",
		"    TYPE(CELL) :: MAKE_CELL",
		"  END FUNCTION MAKE_CELL",
		"!GOVAX$ LINK VALUE_OF STACKS.VALUE_OF",
		"!GOVAX$ FROM VALUE_OF STACKS VALUE_OF",
		"  FUNCTION VALUE_OF(C)",
		"    TYPE(CELL), INTENT(IN) :: C",
		"    REAL(KIND=4) :: VALUE_OF",
		"  END FUNCTION VALUE_OF",
		"END MODULE CALC",
	}

	calc := moduleText(t, r, "CALC")
	got := calc[4:]

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// A renamed module procedure is called by its own global name, a generic
	// name renamed reaches its module's private specifics.
	code := strings.Join(r.MACRO, "\n")
	for _, want := range []string{"\tCALLS\t#0,G^STACKS.POP", "\tCALLS\t#2,G^STACKS.SWAPR", "\tCALLS\t#2,G^STACKS.SWAPI"} {
		if !strings.Contains(code, want) {
			t.Errorf("no %q in the program's code", want)
		}
	}
}

// reexportJ and reexportM are a module with a type and a variable, and a module
// that uses it and passes it on.
const (
	reexportJ = "module j\n  type point\n    real x, y\n  end type\n  integer :: count = 0\nend module"
	reexportM = "module m\n  use j\n  type(point) :: here\nend module"
)

// reexportUser uses both modules: POINT and COUNT reach it by two paths, and
// are one type and one variable.
const reexportUser = "program p\n  use m\n  use j, pt => point\n  type(pt) :: q\n  type(point) :: r\n  q = here\n  r = q\n" +
	"  count = count + 1\n  print *, q%x, r%y, count\nend program"

// TestModuleReexport checks entities a module passes on (finding 8): a user of
// both modules, compiled with them and apart from them, gets one type and one
// variable by both paths, and the same code; and a user of the second module
// alone needs only its module file.
func TestModuleReexport(t *testing.T) {
	both := compileFree("P", reexportJ+"\n"+reexportM+"\n"+reexportUser, nil)
	if both.Errors != 0 {
		t.Fatalf("messages: %v", messageList(both))
	}

	j := compileFree("J", reexportJ, nil)
	m := compileFree("M", reexportM, moduleFiles{"J": moduleText(t, j, "J")})

	found := moduleFiles{"J": moduleText(t, j, "J"), "M": moduleText(t, m, "M")}

	user := compileFree("P", reexportUser, found)
	if user.Errors != 0 {
		t.Fatalf("messages: %v", messageList(user))
	}

	routine := func(lines []string) string { return routineCode(lines, "P") }
	if routine(both.MACRO) != routine(user.MACRO) {
		t.Errorf("the user's code differs:\n%s\n----\n%s", routine(both.MACRO), routine(user.MACRO))
	}

	// M's file alone: J's file isn't needed, nor looked for.
	alone := compileFree("Q", "program q\n  use m\n  here%x = 1.0\n  count = 2\nend program", moduleFiles{"M": found["M"]})
	if alone.Errors != 0 {
		t.Fatalf("messages: %v", messageList(alone))
	}

	text := strings.Join(alone.MACRO, "\n")
	for _, want := range []string{"Q.COUNT = $MOD$J.$BASE\t", "Q.HERE = $MOD$M.$BASE\t"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
}

// TestModuleStale checks two module files that give one type differently, one
// of them made before the type's module changed (MODSTALE), whichever is read
// first.
func TestModuleStale(t *testing.T) {
	j := compileFree("J", reexportJ, nil)
	m := compileFree("M", reexportM, moduleFiles{"J": moduleText(t, j, "J")})
	changed := compileFree("J", strings.Replace(reexportJ, "real x, y", "real x, y, z", 1), nil)
	found := moduleFiles{"J": moduleText(t, changed, "J"), "M": moduleText(t, m, "M")}

	for _, c := range []struct{ text, want string }{
		{"program p\n  use m\n  use j\nend program",
			"3:7 MODSTALE the type POINT of the module J isn't the same in [WORK]M.F90$MOD and [WORK]J.F90$MOD: one of them is out of date"},
		{"program p\n  use j\n  use m\nend program",
			"3:7 MODSTALE the type POINT of the module J isn't the same in [WORK]J.F90$MOD and [WORK]M.F90$MOD: one of them is out of date"},
	} {
		if got := strings.Join(messageList(compileFree("P", c.text, found)), "\n"); got != c.want {
			t.Errorf("got\n%s\nwant\n%s", got, c.want)
		}
	}
}

// TestModuleAccessMessages checks what access and association report.
func TestModuleAccessMessages(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string
	}{
		{"statements", "module m\n  private\n  public\n  integer :: i\n  public :: i\n  private :: i\n  real, private :: x\n  public x\n" +
			"  public :: m, operator(.cross.)\nend module", []string{
			"3:3 ACCESSDEFAULT a module may have only one PUBLIC or PRIVATE statement without a list; the first is at line 2",
			"6:14 ACCESSTWICE I is given PUBLIC or PRIVATE again; it was given at line 5",
			"8:10 ACCESSTWICE X is given PUBLIC or PRIVATE again; it was given at line 7",
			"9:13 ACCESSNAME M is the name of this module, so it can't be PUBLIC or PRIVATE",
			"9:16 ACCESSNAME OPERATOR(.CROSS.) has no interface block in this module, so it can't be PUBLIC or PRIVATE",
		}},
		{"outside", "program p\n  private\nend program", []string{
			"2:3 NOTINMODULE the PRIVATE statement is allowed only in a module",
		}},
		{"attribute", "program p\n  real, public :: x\nend program", []string{
			"2:9 NOTINMODULE the PUBLIC attribute is allowed only in a module",
		}},
		{"types", "module m\n  type, private :: t\n    real r\n  end type\n  type(t) :: v\n  type(t), parameter :: c = t(1.0)\n" +
			"  type pub\n    type(t) :: inner\n  end type\n  namelist /nl/ w\n  real, private :: w\n  interface g\n    module procedure f\n" +
			"  end interface\n  private :: s\ncontains\n  function f(a)\n    type(t) a\n    real f\n    f = a%r\n  end function\n" +
			"  subroutine s(b)\n    type(t) b\n  end subroutine\nend module", []string{
			"5:14 ACCESSTYPE V is public, but its type T is private",
			"6:25 ACCESSTYPE C is public, but its type T is private",
			"8:16 ACCESSCOMPONENT the public type PUB has the component INNER of the private type T, so its components must be PRIVATE",
			"10:13 ACCESSNML the namelist group NL is public, but its item W is private",
			"17:3 ACCESSPROC the module procedure F has the argument A of the private type T, so it must be private, and no public generic interface's",
		}},
		{"components", "module m\n  type pt\n    private\n    real :: x, y\n  end type\n  type(pt), parameter :: origin = pt(0.0, 0.0)\n" +
			"contains\n  real function getx(p)\n    type(pt) p\n    getx = p%x\n  end function\nend module\n" +
			"program p\n  use m\n  type(pt) :: q\n  q = origin\n  print *, q%x, getx(q)\n  q = pt(1.0, 2.0)\nend program", []string{
			"17:14 COMPPRIVATE the components of the type PT are private to the module M",
			"18:7 COMPPRIVATE the components of the type PT are private to the module M",
		}},
		{"only", "module m\n  integer, private :: hid\n  integer :: shown\n  interface operator(.plus.)\n    module procedure add\n" +
			"  end interface\n  private :: operator(.plus.)\ncontains\n  integer function add(a, b)\n    integer, intent(in) :: a, b\n" +
			"    add = a + b\n  end function\nend module\nprogram p\n  use m, only: hid, gone, operator(.plus.), shown\n" +
			"  use m, only: add\n  shown = add(1, 2)\nend program", []string{
			"15:16 ONLYPUBLIC HID isn't a public entity of the module M",
			"15:21 ONLYPUBLIC GONE isn't a public entity of the module M",
			"15:27 ONLYPUBLIC OPERATOR(.PLUS.) isn't a public entity of the module M",
		}},
		{"ambiguous", "module a\n  integer :: x\n  type t\n    real r\n  end type\n  namelist /nl/ x\nend module\n" +
			"module b\n  real :: x\n  type t\n    integer i\n  end type\n  real :: y\n  namelist /nl/ y\nend module\n" +
			"program p\n  use a\n  use b\n  type(t) :: v\n  x = 1\n  write (6, nml=nl)\nend program\n" +
			"subroutine q\n  use a\n  use b\n  y = 2\nend subroutine", []string{
			"19:8 USEAMBIG T can't be referenced here: the modules A and B each give an entity of that name",
			"20:3 USEAMBIG X can't be referenced here: the modules A and B each give an entity of that name",
			"21:17 USEAMBIG NL can't be referenced here: the modules A and B each give an entity of that name",
		}},
		{"scopes", "module m\n  integer, parameter :: n = 3\nend module\nprogram p\n  integer :: n = 7\n  call inner\ncontains\n" +
			"  subroutine inner\n    use m\n    print *, n\n  end subroutine\nend program\nsubroutine s\n  interface\n" +
			"    subroutine t(k)\n      use m\n      integer :: k(n)\n    end subroutine\n  end interface\nend subroutine", nil},
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
