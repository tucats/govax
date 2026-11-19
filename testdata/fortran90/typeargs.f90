! Arrays of derived types and components with rank as actual arguments
! (docs/planning/PHASE-105 - Fortran 90 derived types.md, subtask 5b).
! PATH%X is every X of PATH, elements a POINT's size apart: an assumed-shape
! dummy takes it by a descriptor whose stride is that size, with no copy; an
! old (explicit-shape or assumed-size) dummy, which needs its elements one
! after another, takes a copy, copied back after the call unless the dummy is
! INTENT(IN). Arrays of a type go the same ways: whole, as a section, or as an
! expression's value.
program typeargs
  implicit none
  type point
    real :: x, y
  end type point
  type tagged
    character(len=3) :: name
    integer :: marks(3)
  end type tagged
  type(point) :: path(6), grid(3, 2)
  type(tagged) :: tags(4)
  type(point), parameter :: corner(2) = (/ point(1.0, 2.0), point(3.0, 4.0) /)
  integer :: i, j, k
  real :: t

  path%x = (/ (real(i), i = 1, 6) /)
  path%y = 0.0
  do j = 1, 2
    do i = 1, 3
      grid(i, j) = point(real(i), real(10 * j))
    end do
  end do
  do i = 1, 4
    tags(i)%name = 'T' // char(ichar('0') + i)
    tags(i)%marks = (/ i, 10 * i, 100 * i /)
  end do

  ! Components with rank to assumed-shape dummies: by stride.
  call scale(path%x, 2.0)
  call scale(path(2:6:2)%y, 5.0)
  print 10, (path(i)%x, path(i)%y, i = 1, 6)
  call bump(tags%marks(2))
  call bump(tags(3)%marks)
  call bump(tags(4)%marks(1:3:2))
  print 20, (tags(i)%marks, i = 1, 4)
  call shout(tags%name)
  print 30, tags%name
  call scale(grid(2, :)%y, 0.5)
  call scale2(grid%x, -1.0)
  print 10, ((grid(i, j)%x, grid(i, j)%y, i = 1, 3), j = 1, 2)

  ! To old dummies: a copy, and the copy back.
  call total(path%x, 6, t)
  print 10, t
  call twice(path(1:3)%y, 3)
  print 10, path%y
  call itwice(tags%marks(3), 4)
  print 20, tags%marks(3)
  call lower(tags(2:3)%name, 2)
  print 30, tags%name
  call peek(path%x, 6)
  call total((path%x), 6, t)
  print 10, t

  ! A vector subscript's component, a constant's, and a component's
  ! substrings: each a value, through a temporary never copied back.
  call total(path((/ 1, 3, 5 /))%x, 3, t)
  print 10, t
  call show(path((/ 6, 2 /))%y)
  call show(corner%y)
  call total(corner%x, 2, t)
  print 10, t
  call names(tags%name(2:3))
  k = 1
  call names(tags(2:4)%name(k:k + 1))
  call caps(tags%name(1:1), 4)
  print 30, tags%name

  ! Arrays of a type to assumed-shape dummies: whole, sections.
  call move(path, 1.0)
  call move(path(1:6:5), 100.0)
  print 10, (path(i)%x, path(i)%y, i = 1, 6)
  call move(grid(:, 2), 0.25)
  print 10, ((grid(i, j)%x, grid(i, j)%y, i = 1, 3), j = 1, 2)
  call relay(path(2:4))
  print 10, (path(i)%x, path(i)%y, i = 1, 6)

  ! And to old dummies: in place when contiguous, else a copy and back.
  call drag(path, 6)
  call drag(path(2:3), 2)
  call drag(path(1:6:2), 3)
  print 10, (path(i)%x, path(i)%y, i = 1, 6)
  call drag(grid(2, :), 2)
  print 10, ((grid(i, j)%x, grid(i, j)%y, i = 1, 3), j = 1, 2)
  call tally(merge(path, path(6:1:-1), path%x > 50.0), 6)

10 format (1x, 6f8.2)
20 format (1x, 12i5)
30 format (1x, 4(a, 1x))

contains

  subroutine scale(v, f)
    real :: v(:), f
    v = v * f
  end subroutine scale

  subroutine scale2(v, f)
    real :: v(:, :), f
    v = v * f
  end subroutine scale2

  subroutine bump(m)
    integer :: m(:)
    m = m + 1
  end subroutine bump

  subroutine shout(s)
    character(len=*) :: s(:)
    integer :: k
    do k = 1, size(s)
      s(k)(3:3) = '!'
    end do
  end subroutine shout

  subroutine show(v)
    real, intent(in) :: v(:)
    print 10, v
10  format (1x, 'show', 6f8.2)
  end subroutine show

  subroutine names(s)
    character(len=*), intent(in) :: s(:)
    print 10, len(s), s
10  format (1x, 'names', i2, 4(1x, a))
  end subroutine names

  subroutine caps(s, n)
    integer :: n
    character(len=1) :: s(n)
    s = 'C'
  end subroutine caps

  subroutine total(v, n, t)
    integer :: n
    real, intent(in) :: v(n)
    real :: t
    t = sum(v)
  end subroutine total

  subroutine twice(v, n)
    integer :: n
    real :: v(n)
    v = 2.0 * v + 1.0
  end subroutine twice

  subroutine itwice(v, n)
    integer :: n
    integer :: v(n)
    v = 2 * v + 1
  end subroutine itwice

  subroutine lower(s, n)
    integer :: n
    character(len=3) :: s(n)
    s(1:n)(1:1) = 'u'
  end subroutine lower

  subroutine peek(v, n)
    integer :: n
    real :: v(*)
    print 10, v(1:n)
10  format (1x, 'peek', 6f8.2)
  end subroutine peek

  subroutine move(a, d)
    type(point) :: a(:)
    real :: d
    integer :: k
    a%x = a%x + d
    do k = 1, size(a)
      a(k)%y = a(k)%y + real(k)
    end do
  end subroutine move

  subroutine relay(a)
    type(point) :: a(:)
    call scale(a%y, 10.0)
    call move(a(2:), 1000.0)
    call drag(a, size(a))
  end subroutine relay

  subroutine drag(a, n)
    integer :: n
    type(point) :: a(n)
    a(1:n)%y = -a(1:n)%y
  end subroutine drag

  subroutine tally(a, n)
    integer :: n
    type(point), intent(in) :: a(n)
    print 10, a%x
10  format (1x, 'tally', 6f8.2)
  end subroutine tally

end program typeargs
