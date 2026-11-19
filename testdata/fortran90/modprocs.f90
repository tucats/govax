! Module procedures (docs/planning/PHASE-106 - Fortran 90 modules.md, subtask
! 3): a generic name and two defined operators of module procedures, a module
! procedure with an internal procedure of its own counting in the module's
! variable, a recursive one, one taking an assumed-shape array, one returning
! an array of its argument's size, one taking a dummy procedure, and a module
! procedure passed as an actual argument.
module vectors
  implicit none
  type vec
    real :: x, y
  end type vec
  integer :: swaps = 0
  interface swap
    module procedure swapr, swapi
  end interface
  interface operator(+)
    module procedure addvec
  end interface
  interface operator(.dot.)
    module procedure dot
  end interface
contains
  subroutine swapr(a, b)
    real, intent(inout) :: a, b
    real :: t
    t = a
    a = b
    b = t
    call count_it
  contains
    subroutine count_it
      swaps = swaps + 1
    end subroutine count_it
  end subroutine swapr
  subroutine swapi(i, j)
    integer, intent(inout) :: i, j
    integer :: t
    t = i
    i = j
    j = t
    swaps = swaps + 1
  end subroutine swapi
  function addvec(u, v) result(w)
    type(vec), intent(in) :: u, v
    type(vec) :: w
    w = vec(u%x + v%x, u%y + v%y)
  end function addvec
  real function dot(u, v)
    type(vec), intent(in) :: u, v
    dot = u%x * v%x + u%y * v%y
  end function dot
  recursive function fact(n) result(f)
    integer, intent(in) :: n
    integer :: f
    if (n <= 1) then
      f = 1
    else
      f = n * fact(n - 1)
    end if
  end function fact
  real function total(a)
    real, intent(in) :: a(:)
    total = sum(a)
  end function total
  function scaled(a, s)
    real, intent(in) :: a(:), s
    real :: scaled(size(a))
    scaled = a * s
  end function scaled
  real function twice(f, x)
    interface
      real function f(x)
        real, intent(in) :: x
      end function f
    end interface
    real, intent(in) :: x
    twice = f(f(x))
  end function twice
  real function half(x)
    real, intent(in) :: x
    half = x / 2
  end function half
end module vectors

program modprocs
  use vectors
  implicit none
  real :: x = 1.0, y = 2.0, z(4) = (/ 1.0, 2.0, 3.0, 4.0 /)
  integer :: i = 3, j = 4
  type(vec) :: w
  call swap(x, y)
  call swap(i, j)
  print '(1X, 2F5.1, 3I3)', x, y, i, j, swaps
  w = vec(1.0, 2.0) + vec(3.0, 4.0)
  print '(1X, 2F5.1, F6.1)', w%x, w%y, w .dot. vec(1.0, 1.0)
  print '(1X, I8)', fact(10)
  print '(1X, F6.1, 4F5.1)', total(z), scaled(z, 2.0)
  print '(1X, F5.1)', twice(half, 8.0)
end program modprocs
