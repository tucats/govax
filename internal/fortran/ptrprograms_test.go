package fortran_test

import "testing"

// What testdata/fortran90's programs of pointers found (docs/planning/PHASE-107 -
// Fortran 90 pointers.md, subtask 8: sparse.f90), held here by the compiler alone.

// sparseModule is the part of sparse.f90's module the three tests need: a matrix
// whose rows are an array pointer component, a function whose value's extent is
// a dummy's component, operators extended to the type, and a pointer function.
const sparseModule = `module sparse_matrices
  type entry
    integer :: col
    real :: val
    type(entry), pointer :: next
  end type entry
  type row
    type(entry), pointer :: first
  end type row
  type matrix
    integer :: n, m
    type(row), pointer :: rows(:)
  end type matrix
  interface operator (*)
    module procedure times_vector
  end interface
  interface assignment (=)
    module procedure fill
  end interface
contains
  function times_vector(a, x) result(y)
    type(matrix), intent(in) :: a
    real, intent(in) :: x(:)
    real :: y(a%n)
    integer :: i
    do i = 1, a%n
      y(i) = x(i) * real(a%m)
    end do
  end function times_vector

  subroutine fill(a, x)
    type(matrix), intent(out) :: a
    real, intent(in) :: x(:)
    a%n = size(x)
    a%m = 1
    nullify(a%rows)
  end subroutine fill

  function find(a, i) result(p)
    type(matrix), intent(in) :: a
    integer, intent(in) :: i
    type(entry), pointer :: p
    p => a%rows(i)%first
  end function find
end module sparse_matrices`

// TestValueFromComponent checks a function whose value's extent is a dummy's
// component, Y(A%N), which semantic analysis refused (BADBOUND), and a caller
// of it.
func TestValueFromComponent(t *testing.T) {
	r := compileFree("SPARSE", sparseModule+`
program main
  use sparse_matrices
  type(matrix) :: a
  real :: y(3), x(3)
  a%n = 3
  a%m = 2
  x = 1.0
  y = times_vector(a, x)
  print *, y, size(times_vector(a, x))
end program main`, nil)
	if r.Errors != 0 || r.MACRO == nil {
		t.Fatalf("messages: %v", messageList(r))
	}

	assembles(t, "SPARSE", r.MACRO)
}

// TestOperandArrays checks an array constant and an array expression as the
// operands of a defined operator and the value of a defined assignment, whose
// dummies are assumed-shape: they go through a temporary and a descriptor, as a
// reference's arguments do. Lowering met the constant itself, and panicked.
func TestOperandArrays(t *testing.T) {
	r := compileFree("SPARSE", sparseModule+`
program main
  use sparse_matrices
  type(matrix) :: a
  real :: y(3), x(3)
  a%n = 3
  a%m = 2
  x = 2.0
  y = a * (/ 1.0, 2.0, 3.0 /)
  print *, a * (/ 1.0, 2.0, 3.0 /), a * (x + 1.0), a * x(3:1:-1), a * x
  a = (/ 4.0, 5.0 /)
  a = x * 2.0
end program main`, nil)
	if r.Errors != 0 || r.MACRO == nil {
		t.Fatalf("messages: %v", messageList(r))
	}

	assembles(t, "SPARSE", r.MACRO)
}

// TestAssociatedOfFunction checks ASSOCIATED whose pointer is a pointer
// function's value, alone and with a target, which semantic analysis refused
// (NOTPOINTER).
func TestAssociatedOfFunction(t *testing.T) {
	r := compileFree("SPARSE", sparseModule+`
program main
  use sparse_matrices
  type(matrix) :: a
  type(entry), pointer :: e
  allocate (a%rows(2))
  nullify (a%rows(1)%first, a%rows(2)%first)
  allocate (a%rows(2)%first)
  e => a%rows(2)%first
  print *, associated(find(a, 1)), associated(find(a, 2)), associated(find(a, 2), e)
end program main`, nil)
	if r.Errors != 0 || r.MACRO == nil {
		t.Fatalf("messages: %v", messageList(r))
	}

	assembles(t, "SPARSE", r.MACRO)
}
