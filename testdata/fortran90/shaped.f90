! Assumed-shape dummy arguments (Phase 104, subtask 5a): internal procedures,
! and external ones through interface blocks, given whole arrays (of constant
! bounds, adjustable, automatic, and allocatable) and sections, one of no
! elements, with any strides, backward too, of one and two dimensions, a row
! of a matrix among them; their own lower bounds (0:, -1:); SIZE, LBOUND,
! UBOUND, and SUM of them, whole-array assignment and elements written back to
! the caller's array; a CHARACTER(LEN=*) one; one passed on to another, whole
! and as a section, by a recursive function to itself; an OPTIONAL one left
! out.
program shaped
  implicit none
  interface
    subroutine twice(v)
      real :: v(:)
    end subroutine
    real function total(m)
      real :: m(:, :)
    end function
    recursive integer function countdn(v) result(n)
      integer :: v(:)
    end function
    subroutine maybe(x, y)
      real :: x(:)
      real, optional :: y(:)
    end subroutine
  end interface
  real :: a(10), b(3, 4)
  real, allocatable :: h(:)
  character(len=2) :: names(5)
  integer :: i, j, k(6)

  a = (/ (real(i), i = 1, 10) /)
  call show(a)
  call show(a(2:9:3))
  call show(a(10:1:-4))
  call show(a(5:4))
  call twice(a(1:10:2))
  print '(1x, a, 10f5.0)', 'twice:', a

  do j = 1, 4
    do i = 1, 3
      b(i, j) = 10 * i + j
    end do
  end do
  print '(1x, a, f6.0)', 'total:', total(b)
  print '(1x, a, f6.0)', 'total:', total(b(2:3, 1:4:2))
  call corner(b(:, 2:))
  call row(b(2, :))

  allocate (h(4))
  h = (/ 1.0, 2.0, 3.0, 4.0 /)
  call twice(h)
  call show(h(4:1:-1))

  names = (/ 'ab', 'cd', 'ef', 'gh', 'ij' /)
  call words(names(1:5:2))
  call words(names)

  k = (/ 3, 1, 4, 1, 5, 9 /)
  print '(1x, a, i4)', 'countdn:', countdn(k)
  print '(1x, a, i4)', 'countdn:', countdn(k(2:6:2))

  call maybe(a(1:3))
  call maybe(a(1:3), a(4:5))
  call adjust(b, 3, 4)

contains

  subroutine show(v)
    real :: v(:)
    print '(1x, a, 3i3, 10f5.0)', 'show:', size(v), lbound(v), ubound(v), v
  end subroutine show

  subroutine corner(m)
    real :: m(0:, -1:)
    print '(1x, a, 4i3, 2f5.0)', 'corner:', lbound(m), ubound(m), m(0, -1), m(2, 1)
    m(1, 0) = -m(1, 0)
  end subroutine corner

  subroutine row(r)
    real :: r(:)
    integer :: i
    do i = 1, size(r)
      r(i) = r(i) + 100.0
    end do
    call show(r)
  end subroutine row

  subroutine words(w)
    character(len=*) :: w(:)
    print '(1x, a, i2, i3, 1x, 5a)', 'words:', size(w), len(w), w
  end subroutine words

  subroutine adjust(m, n1, n2)
    integer :: n1, n2
    real :: m(n1, n2)
    real :: w(n2)
    w = m(2, :)
    call show(m(3, :))
    call show(w)
  end subroutine adjust
end program shaped

subroutine twice(v)
  real :: v(:)
  v = 2.0 * v
end subroutine twice

real function total(m)
  real :: m(:, :)
  integer :: i, j
  total = 0.0
  do j = 1, size(m, 2)
    do i = 1, size(m, 1)
      total = total + m(i, j)
    end do
  end do
end function total

recursive integer function countdn(v) result(n)
  integer :: v(:)
  if (size(v) == 0) then
    n = 0
  else
    n = v(1) + countdn(v(2:))
  end if
end function countdn

subroutine maybe(x, y)
  real :: x(:)
  real, optional :: y(:)
  if (present(y)) then
    print '(1x, a, 2f6.1)', 'maybe:', sum(x), sum(y)
  else
    print '(1x, a, f6.1)', 'maybe:', sum(x)
  end if
end subroutine maybe
