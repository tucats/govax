! Functions of derived types and derived types as arguments (docs/planning/
! PHASE-105 - Fortran 90 derived types.md, subtask 4). A value of up to 8
! bytes comes back in R0, or R0 and R1, as any other value of that size: TAG's
! 3 bytes and PAIR's 4 in R0, POINT's 8 and REC6's 6 in R0 and R1. A longer
! value, TRIPLE's and BOX's, goes through a hidden first argument, as a
! COMPLEX*16's does. The types are SEQUENCE types, defined again in each unit
! and each interface body, since a unit with no host sees no other's
! definition, and two SEQUENCE definitions agreeing in name and components are
! one type.
program results
  implicit none
  type tag
    sequence
    character(len=3) :: code
  end type tag
  type pair
    sequence
    integer(2) :: a, b
  end type pair
  type rec6
    sequence
    integer(2) :: a
    integer :: b
  end type rec6
  type point
    sequence
    real :: x, y
  end type point
  type triple
    sequence
    real :: x, y, z
  end type triple
  type box
    sequence
    type(point) :: lo, hi
  end type box
  interface
    type(point) function corner(b)
      type point
        sequence
        real :: x, y
      end type point
      type box
        sequence
        type(point) :: lo, hi
      end type box
      type(box), intent(in) :: b
    end function corner
    recursive function chain(n) result(t)
      type rec6
        sequence
        integer(2) :: a
        integer :: b
      end type rec6
      integer, intent(in) :: n
      type(rec6) :: t
    end function chain
  end interface
  interface operator(+)
    function plus(p, q)
      type point
        sequence
        real :: x, y
      end type point
      type(point), intent(in) :: p, q
      type(point) :: plus
    end function plus
  end interface
  type(tag) :: g, tagged
  type(pair) :: s, swap
  type(rec6) :: r, made, other
  type(point) :: p, q, mid, doubled
  type(triple) :: t, lift
  type(box) :: bx
  external tagged, swap, made, other, mid, doubled, lift

  ! Values in registers, stored straight into the target or through a
  ! temporary, and a value through a hidden argument.
  p = point(1.0, 2.0)
  q = point(3.0, 6.0)
  p = mid(p, q)
  print '(2F8.2)', p%x, p%y
  s = swap(pair(1, 2))
  print *, s%a, s%b
  g = tagged('xyz')
  print *, g%code
  t = lift(p, 5.0)
  print '(3F8.2)', t%x, t%y, t%z

  ! A function with an entry point of its type.
  r = made(7)
  print *, r%a, r%b
  r = other(9)
  print *, r%a, r%b

  ! Values as a constructor's components and as arguments.
  bx = box(point(1.0, 2.0), corner(box(point(0.0, 0.0), point(3.0, 4.0))))
  print '(4F8.2)', bx%lo%x, bx%lo%y, bx%hi%x, bx%hi%y
  call show(corner(bx))
  q = half(corner(bx))
  print '(2F8.2)', q%x, q%y

  ! A defined operator's function of a derived type, its value an operand.
  q = p + q
  print '(2F8.2)', q%x, q%y
  q = (p + q) + point(10.0, 20.0)
  print '(2F8.2)', q%x, q%y

  ! Recursion, and a function passed as an argument.
  r = chain(5)
  print *, r%a, r%b
  call apply(doubled, point(8.0, 6.0))
contains
  type(point) function half(p)
    type(point), intent(in) :: p
    half = point(p%x / 2, p%y / 2)
  end function half
end program results

function mid(p, q)
  type point
    sequence
    real :: x, y
  end type point
  type(point) :: mid, p, q
  mid%x = (p%x + q%x) / 2
  mid%y = (p%y + q%y) / 2
end function mid

type(pair) function swap(v)
  type pair
    sequence
    integer(2) :: a, b
  end type pair
  type(pair) :: v
  swap = pair(v%b, v%a)
end function swap

function tagged(s)
  type tag
    sequence
    character(len=3) :: code
  end type tag
  type(tag) :: tagged
  character(len=*) :: s
  tagged%code = s(3:3) // s(2:2) // s(1:1)
end function tagged

function lift(p, z) result(r)
  type point
    sequence
    real :: x, y
  end type point
  type triple
    sequence
    real :: x, y, z
  end type triple
  type(point) :: p
  type(triple) :: r
  real :: z
  r = triple(p%x, p%y, z)
end function lift

function made(n)
  type rec6
    sequence
    integer(2) :: a
    integer :: b
  end type rec6
  type(rec6) :: made, other
  integer :: n
  made = rec6(n, n * 100)
  return
  entry other(n)
  other = rec6(-n, n * 1000)
end function made

type(point) function corner(b)
  type point
    sequence
    real :: x, y
  end type point
  type box
    sequence
    type(point) :: lo, hi
  end type box
  type(box), intent(in) :: b
  corner = b%hi
end function corner

function plus(p, q)
  type point
    sequence
    real :: x, y
  end type point
  type(point), intent(in) :: p, q
  type(point) :: plus
  plus = point(p%x + q%x, p%y + q%y)
end function plus

recursive function chain(n) result(t)
  type rec6
    sequence
    integer(2) :: a
    integer :: b
  end type rec6
  integer, intent(in) :: n
  type(rec6) :: t
  if (n <= 1) then
    t = rec6(1, 1)
  else
    t = chain(n - 1)
    t%a = t%a + 1
    t%b = t%b * n
  end if
end function chain

function doubled(p)
  type point
    sequence
    real :: x, y
  end type point
  type(point) :: doubled, p
  doubled = point(2 * p%x, 2 * p%y)
end function doubled

subroutine show(p)
  type point
    sequence
    real :: x, y
  end type point
  type(point) :: p
  print '(1X, A, 2F8.2)', 'show', p%x, p%y
end subroutine show

subroutine apply(f, p)
  type point
    sequence
    real :: x, y
  end type point
  type(point) :: f, p, q
  external f
  q = f(p)
  print '(1X, A, 2F8.2)', 'apply', q%x, q%y
end subroutine apply
