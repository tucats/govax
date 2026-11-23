! ELEMENTAL procedures (Phase 112, subtask 7): module, internal, and external
! ones with an interface block; functions referenced with arrays, scalars,
! sections, and expressions, inside array expressions and reductions, and in
! an I/O list; a subroutine called on arrays in array element order; a
! generic name whose elemental specific is chosen for arrays only when no
! specific fits them exactly; an elemental defined operator; and an elemental
! defined assignment in an array assignment (its value whole before the target
! changes, even where the two overlap) and in a WHERE.
module elemmod
  implicit none

  type money
    integer :: cents
  end type money

  interface operator(.plus.)
    module procedure addm
  end interface

  interface assignment(=)
    module procedure fromint, frommoney
  end interface

  interface twice
    module procedure twicev, twicee
  end interface

contains

  elemental real function clip(x, lo, hi)
    real, intent(in) :: x, lo, hi
    clip = max(lo, min(hi, x))
  end function clip

  elemental integer function cube(k)
    integer, intent(in) :: k
    cube = k * k * k
  end function cube

  elemental character(len=3) function tag(k)
    integer, intent(in) :: k
    write (tag, '(A1,I2.2)') 'T', k
  end function tag

  elemental subroutine order(a, b)
    integer, intent(inout) :: a, b
    integer :: t
    if (a > b) then
      t = a; a = b; b = t
    end if
  end subroutine order

  elemental subroutine accumulate(total, x)
    integer, intent(inout) :: total
    integer, intent(in) :: x
    total = total + x
  end subroutine accumulate

  ! TWICE: a specific for whole vectors, chosen for a rank-1 array, and an
  ! elemental one for scalars and arrays of any other rank.
  function twicev(v) result(r)
    integer, intent(in) :: v(:)
    integer :: r(size(v))
    r = 1000 + 2 * v
  end function twicev

  elemental integer function twicee(k)
    integer, intent(in) :: k
    twicee = 2 * k
  end function twicee

  elemental type(money) function addm(x, y)
    type(money), intent(in) :: x, y
    addm%cents = x%cents + y%cents
  end function addm

  elemental subroutine fromint(m, k)
    type(money), intent(out) :: m
    integer, intent(in) :: k
    m%cents = 100 * k
  end subroutine fromint

  ! A copy that adds one cent: it shows when an element was copied from one
  ! already changed by the same assignment.
  elemental subroutine frommoney(m, n)
    type(money), intent(out) :: m
    type(money), intent(in) :: n
    m%cents = n%cents + 1
  end subroutine frommoney

end module elemmod

program elementals
  use elemmod
  implicit none

  interface
    elemental real function half(x)
      real, intent(in) :: x
    end function half
  end interface

  real :: r(5), s(5)
  integer :: p(4), q(4), m(2, 3), total(3)
  integer :: i
  type(money) :: w(4), v(4)
  character(len=3) :: names(3)

  r = (/ -2.0, -0.5, 0.5, 1.5, 3.0 /)
  print '(5F6.2)', clip(r, -1.0, 1.0)
  print '(5F6.2)', clip(r, 0.0, (/ 1.0, 1.0, 1.0, 2.0, 2.0 /))
  print '(F6.2)', clip(7.0, 0.0, 5.0)

  ! In an array expression, a reduction, and a section of the target itself,
  ! which the assignment reads before it writes.
  s = 2.0 * clip(r, -1.0, 1.0) + 1.0
  print '(5F6.2)', s
  print '(F6.2)', sum(clip(r, -1.0, 1.0))
  r(2:5) = clip(r(1:4), -1.0, 2.0)
  print '(5F6.2)', r
  print '(5F6.2)', half(r)

  p = (/ 1, 2, 3, 4 /)
  print '(4I5)', cube(p)
  print '(4I5)', cube(p(4:1:-1) - 1)
  print '(4I5)', inc(p)
  names = tag((/ 7, 42, 3 /))
  print '(1X,3(A,1X))', names

  ! Calls in array element order, an array with a scalar.
  p = (/ 9, 2, 7, 4 /)
  q = (/ 5, 6, 1, 8 /)
  call order(p, q)
  print '(8I3)', p, q
  m = reshape((/ 1, 2, 3, 4, 5, 6 /), (/ 2, 3 /))
  total = 0
  call accumulate(total, m(1, :))
  call accumulate(total, m(2, :))
  print '(3I4)', total

  ! The generic's specifics.
  print '(4I6)', twice(p)
  print '(I6)', twice(5)
  print '(6I6)', twice(m)

  ! A defined operator and defined assignments.
  w = (/ 1, 2, 3, 4 /)
  v = w .plus. w
  print '(4I6)', w%cents
  print '(4I6)', v%cents
  w(2:4) = w(1:3)
  print '(4I6)', w%cents
  where (p > 4) v = p
  print '(4I6)', v%cents
  where (p > 4)
    w = 0
  elsewhere
    w = w .plus. v
  end where
  print '(4I6)', w%cents

contains

  elemental integer function inc(k)
    integer, intent(in) :: k
    inc = k + 1
  end function inc

end program elementals

elemental real function half(x)
  real, intent(in) :: x
  half = x / 2.0
end function half
