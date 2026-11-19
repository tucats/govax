! Access and association (docs/planning/PHASE-106 - Fortran 90 modules.md,
! subtask 4): a module whose default is PRIVATE, with private data, a public
! type whose components are private, and a public generic name whose specifics
! are private; a second module that uses the first by an ONLY list with a
! rename and passes some of it on, keeping one name private; a program that
! reaches one entity by two paths, joins two ONLY lists of one module, renames
! a generic name, and has a name two modules give that it never references;
! and an internal procedure whose USE hides its host's name.
module stacks
  implicit none
  private
  public :: push, pop, depth, cell, make_cell, value_of, swap, limit
  integer, parameter :: limit = 100
  integer :: depth = 0
  real :: items(limit)
  type cell
    private
    real :: value
  end type cell
  interface swap
    module procedure swapr, swapi
  end interface
contains
  subroutine push(x)
    real, intent(in) :: x
    depth = depth + 1
    items(depth) = x
  end subroutine push
  real function pop()
    pop = items(depth)
    depth = depth - 1
  end function pop
  function make_cell(x)
    real, intent(in) :: x
    type(cell) :: make_cell
    make_cell = cell(x)
  end function make_cell
  real function value_of(c)
    type(cell), intent(in) :: c
    value_of = c%value
  end function value_of
  subroutine swapr(a, b)
    real, intent(inout) :: a, b
    real :: t
    t = a
    a = b
    b = t
  end subroutine swapr
  subroutine swapi(i, j)
    integer, intent(inout) :: i, j
    integer :: t
    t = i
    i = j
    j = t
  end subroutine swapi
end module stacks

module calc
  use stacks, only: push, top => pop, depth, cell, make_cell, value_of
  implicit none
  private :: depth
  integer, parameter :: limit = 5
contains
  real function sum_top(n)
    integer, intent(in) :: n
    integer :: k
    sum_top = 0.0
    do k = 1, min(n, depth)
      sum_top = sum_top + top()
    end do
  end function sum_top
end module calc

program modaccess
  use calc
  use stacks, only: push, limit
  use stacks, only: depth, exchange => swap
  implicit none
  type(cell) :: c
  real :: x = 1.5, y = 2.5, v
  integer :: i = 1, j = 2, n = 7
  call push(1.0)
  call push(2.0)
  call push(4.0)
  print '(1X, I3)', depth
  v = top()
  print '(1X, F6.1, I3)', v, depth
  v = sum_top(5)
  print '(1X, F6.1, I3)', v, depth
  c = make_cell(3.25)
  print '(1X, F6.2)', value_of(c)
  call exchange(x, y)
  call exchange(i, j)
  print '(1X, 2F5.1, 2I3)', x, y, i, j
  call inner
  print '(1X, I4)', n
contains
  subroutine inner
    use stacks, only: n => limit
    print '(1X, I4)', n
  end subroutine inner
end program modaccess
