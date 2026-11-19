! Structure constructors, assignment, and operators on derived types (docs/
! planning/PHASE-105 - Fortran 90 derived types.md, subtask 3). A constructor
! of constants is a constant: a PARAMETER's value, an initial value, and its
! components constants too. Any other is built a component at a time, in its
! target when no value reads the target, else in a temporary. A derived type
! has no operators of its own: .DIST. and == here are interface blocks', whose
! bodies define the SEQUENCE type again, as a body has no host, and the
! assignment of a PAIR is a subroutine's, which swaps its components: every
! assignment of a PAIR goes through it, B = PAIR(1, 2) too. REALs are printed
! by formats, objects whole (subtask 7), so gfortran's witness prints the same.
program structs
  implicit none
  type point
    sequence
    real :: x, y
  end type point
  type label
    character(len=6) :: text
    integer :: marks(3)
    type(point) :: at
  end type label
  type pair
    sequence
    integer :: first, second
  end type pair
  interface operator(.dist.)
    real function dist(p, q)
      type point
        sequence
        real :: x, y
      end type point
      type(point), intent(in) :: p, q
    end function dist
  end interface
  interface operator(==)
    logical function same(p, q)
      type point
        sequence
        real :: x, y
      end type point
      type(point), intent(in) :: p, q
    end function same
  end interface
  interface assignment(=)
    subroutine swapped(r, s)
      type pair
        sequence
        integer :: first, second
      end type pair
      type(pair), intent(out) :: r
      type(pair), intent(in) :: s
    end subroutine swapped
  end interface
  type(point), parameter :: origin = point(0.0, 0.0), unit = point(1, 0)
  real, parameter :: ux = unit%x
  type(point) :: p, q = point(3.0, 4.0), path(3)
  type(label) :: l, m = label('start', 7, point(-1.0, 2.5))
  type(pair) :: a, b
  integer :: i
  real :: r

  print '(3f8.3)', ux, origin%y, unit%x + unit%y
  print '(2f8.3)', q
  print '(1x, a, 3i4, 2f8.3)', m

  p = point(1.5, 2.5)
  print '(2f8.3)', p
  r = 6.0
  p = point(r, r / 4)
  print '(2f8.3)', p
  p = point(p%y, p%x)
  print '(2f8.3)', p
  p = origin
  print '(2f8.3)', p

  do i = 1, 3
    path(i) = point(i, i * i)
  end do
  print '(6f8.3)', (path(i), i = 1, 3)

  l = label('mark' // 'ed', (/ 1, 2, 3 /), point(r, -r))
  print '(1x, a, 3i4, 2f8.3)', l
  l = label('x', 5, q)
  print '(1x, 2a, 3i4, 2f8.3)', l%text, '|', l%marks, l%at

  print '(2f8.3)', origin .dist. q, q .dist. point(r, 0.0)
  print *, p == origin, q == point(3, 4), path(2) == q

  b = pair(1, 2)
  a = b
  print *, a%first, a%second
  a = pair(10 * b%first, 20)
  print *, a%first, a%second
end program structs

real function dist(p, q)
  type point
    sequence
    real :: x, y
  end type point
  type(point), intent(in) :: p, q
  dist = sqrt((p%x - q%x)**2 + (p%y - q%y)**2)
end function dist

logical function same(p, q)
  type point
    sequence
    real :: x, y
  end type point
  type(point), intent(in) :: p, q
  same = p%x == q%x .and. p%y == q%y
end function same

subroutine swapped(r, s)
  type pair
    sequence
    integer :: first, second
  end type pair
  type(pair), intent(out) :: r
  type(pair), intent(in) :: s
  r%first = s%second
  r%second = s%first
end subroutine swapped
