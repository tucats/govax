! Assumed-shape dummies, further (Phase 104, subtask 5a): a lower bound that
! is a dummy argument (v(lo:)); an automatic array whose size is SIZE of one;
! an ENTRY with the dummies in another order; an internal procedure reading
! its host's; INTENT(IN) and INTENT(INOUT) ones of rank two given sections
! with steps both ways in both dimensions; DOT_PRODUCT of two.
program shapes
  implicit none
  interface
    subroutine stats(v, lo)
      integer :: lo
      real :: v(lo:)
    end subroutine
    subroutine other(lo, v)
      integer :: lo
      real :: v(lo:)
    end subroutine
    subroutine add(x, y)
      real, intent(inout) :: x(:, :)
      real, intent(in) :: y(:, :)
    end subroutine
    real function dot(a, b)
      real :: a(:), b(:)
    end function
  end interface
  real :: p(6), q(2, 3), r(3, 2)
  integer :: i, j

  p = (/ (real(i * i), i = 1, 6) /)
  call stats(p, 0)
  call stats(p(2:6:2), 5)
  call other(-2, p(6:1:-1))

  do j = 1, 3
    do i = 1, 2
      q(i, j) = i + 10 * j
    end do
  end do
  r = 100.0
  call add(q(:, 1:2), r(1:2, :))
  print '(1x, a, 6f6.0)', 'q:', q
  call add(q(2:1:-1, 3:1:-2), r(2:3, :))
  print '(1x, a, 6f6.0)', 'q:', q

  print '(1x, a, f6.0)', 'dot:', dot(p(1:3), p(4:6))
  print '(1x, a, f6.0)', 'dot:', dot(q(1, :), q(2, :))
end program shapes

subroutine stats(v, lo)
  integer :: lo
  real :: v(lo:)
  real :: w(size(v))
  print '(1x, a, 3i4)', 'stats:', size(v), lbound(v), ubound(v)
  go to 10
  entry other(lo, v)
  print '(1x, a, 3i4)', 'other:', size(v), lbound(v), ubound(v)
10 w = v - sum(v) / size(v)
  print '(1x, a, 6f7.1)', 'w:', w
  print '(1x, a, 2f6.1)', 'ends:', v(lo), v(ubound(v, 1))
  call inner
contains
  subroutine inner
    print '(1x, a, f6.1)', 'inner:', v(lbound(v, 1) + 1)
  end subroutine inner
end subroutine stats

subroutine add(x, y)
  real, intent(inout) :: x(:, :)
  real, intent(in) :: y(:, :)
  x = x + y
end subroutine add

real function dot(a, b)
  real :: a(:), b(:)
  dot = dot_product(a, b)
end function dot
