package fortran_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/asm"
)

// What testdata/fortran90/modules' programs of several files found (docs/planning/
// PHASE-106 - Fortran 90 modules.md, subtask 7), held here by the compiler alone.

// workModule is the standard's 11.3.3.2 and 11.3.3.4 in one module: a SAVE
// statement, and allocatable arrays with the SAVE attribute.
const workModule = `module work
  save
  integer n
  real, allocatable, save :: a(:), b(:, :)
  real :: total
  save total
end module work`

// TestModuleSaveAndAllocatable checks that SAVE may name a module's variables,
// which are in its section, though not a COMMON block's member, and that each
// allocatable array's descriptor is in the module's section in its unallocated
// form, written by the module's object alone.
func TestModuleSaveAndAllocatable(t *testing.T) {
	r := compileFree("WORK", workModule, nil)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	text := strings.Join(r.MACRO, "\n")
	for _, want := range []string{
		"$MOD$WORK.$BASE:\n\t.BLKB\t4\n\t.WORD\t4\t\t\t; length\n\t.BYTE\tDSC$K_DTYPE_F,DSC$K_CLASS_NCA\n\t.LONG\t0\t\t\t; A: not allocated\n" +
			"\t.BYTE\t0,0,0,1\t\t\t; scale, digits, flags, dimensions",
		"\t.LONG\t0\t\t\t; B: not allocated\n\t.BYTE\t0,0,0,2\t\t\t; scale, digits, flags, dimensions",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}

	user := compileFree("SETUP", `subroutine setup
  use work
  n = 3
  allocate (a(n), b(n, n))
end subroutine setup`, moduleFiles{"WORK": moduleText(t, r, "WORK")})
	if user.Errors != 0 {
		t.Fatalf("the user's messages: %v", messageList(user))
	}

	if text := strings.Join(user.MACRO, "\n"); !strings.Contains(text, "$MOD$WORK.$BASE:\n\t.BLKB\t") || strings.Contains(text, "not allocated") {
		t.Errorf("the user's object gives the module's section values:\n%s", text)
	}

	saved := compileFree("BLOCKS", `module blocks
  common /c/ m
  save m
end module blocks`, nil)
	if got := messageList(saved); len(got) != 1 || !strings.Contains(got[0], "SAVECOMMON") {
		t.Errorf("SAVE of a COMMON block's member in a module: %v", got)
	}
}

// stringsModules are the standard's 11.3.3.6: a module of types, and one whose
// interface bodies for external functions reach them by USE.
const stringsModules = `module varying_strings
  type varying
    integer length
    character (len = 40) text
  end type
end module varying_strings

module operators
  use varying_strings
  interface operator (//)
    function join (x, y)
      use varying_strings
      type (varying) join
      type (varying), intent (in) :: x, y
    end function join
  end interface
end module operators`

// TestModuleInterfaceBodyTypes checks that a module's text, whose interface
// bodies name the module's types with no USE of their own, is read: a body
// there reaches the types of the module it is in.
func TestModuleInterfaceBodyTypes(t *testing.T) {
	r := compileFree("STRINGS", stringsModules, nil)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	found := moduleFiles{"VARYING_STRINGS": moduleText(t, r, "VARYING_STRINGS"), "OPERATORS": moduleText(t, r, "OPERATORS")}

	user := compileFree("JOINER", `subroutine joiner
  use operators
  type (varying) a, b
  a = varying (3, 'one')
  b = a // a
  print *, b % text (1:b % length)
end subroutine joiner`, found)
	if user.Errors != 0 {
		t.Errorf("the user's messages: %v", messageList(user))
	}
}

// TestBoundFromAssumedShape checks an explicit-shape dummy whose bound is an
// inquiry of an assumed-shape dummy, the standard's 11.3.3.5: its code is the
// entry point's, before the first statement.
func TestBoundFromAssumedShape(t *testing.T) {
	r := compileFree("LLS", `subroutine lls (x, a, f, flag)
  real x (:, :)
  real, dimension (size (x, 2)) :: a, f
  integer flag
  a = sum (x, dim = 1)
  f = maxval (abs (x), dim = 1)
  flag = count (a < 0.0)
end subroutine lls`, nil)
	if r.Errors != 0 || r.MACRO == nil {
		t.Fatalf("messages: %v", messageList(r))
	}

	assembles(t, "LLS", r.MACRO)
}

// TestFormatBesideConstant checks that a FORMAT statement's block, $Fn, and a
// floating constant the code generator adds, $KFn, have names apart.
func TestFormatBesideConstant(t *testing.T) {
	r := compileFree("RENAME", `subroutine rename
  real x
  x = 1.0
  x = x + 10.0
  print 1, x
1 format (' X =', F5.1)
end subroutine rename`, nil)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	text := strings.Join(r.MACRO, "\n")
	if !strings.Contains(text, "RENAME.$F1:") || !strings.Contains(text, "RENAME.$KF1:") {
		t.Errorf("no FORMAT block and constant apart in\n%s", text)
	}

	assembles(t, "RENAME", r.MACRO)
}

// assembles checks that MACRO assembles the file.
func assembles(t *testing.T, name string, lines []string) {
	t.Helper()

	lib, err := asm.SystemMacros()
	if err != nil {
		t.Fatal(err)
	}

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)
	a.SetMacroLibraries(lib)

	if _, err := a.Assemble(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Errorf("%s doesn't assemble: %v", name, err)
	}
}
