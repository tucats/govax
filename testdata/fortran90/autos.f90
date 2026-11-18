! Automatic arrays (Phase 104, subtask 4a): a subprogram's own arrays whose
! bounds are computed when it is entered, from its dummy arguments, a common
! block, and the inquiry functions SIZE, LBOUND, UBOUND, and LEN with MAX;
! whole and by element, in output lists, passed to an explicit-shape dummy,
! reached by an internal procedure; one of zero size; one at each level of a
! recursion; an ENTRY with the dummies in another order; and storage given
! back at each return, so that ten thousand calls don't use up the stack.
program autos
  implicit none
  integer :: m
  common /sizes/ m
  integer :: i, j, depth
  real :: a(3, 4)
  real :: total

  do j = 1, 4
    do i = 1, 3
      a(i, j) = 10 * i + j
    end do
  end do

  call fill(a, 3)
  call codes('VAX')
  call codes('')

  m = 4
  call sized
  m = 1
  call sized
  m = 0
  call sized

  print '(1x, a, i4)', 'depth:', depth(4)

  call pair(2, 3)
  call pair2(5, 3)

  m = 1000
  total = 0.0
  do i = 1, 10000
    call churn(total)
  end do
  print '(1x, a, f10.0)', 'churned:', total
end program

subroutine fill(a, n)
  implicit none
  integer :: n
  real :: a(n, 4)
  real :: w(size(a, 1)), t(0:n+1)
  integer :: i

  do i = 1, n
    w(i) = sum(a(i, :))
  end do
  t = 0.0
  t(1:n) = w
  print '(1x, a, 3f6.0)', 'row sums:', w
  print '(1x, a, 4i3)', 'w and t:', size(w), size(t), lbound(t), ubound(t)
  call bump
  call show(t, n + 2)
contains
  subroutine bump
    w = w + 1.0
    t(n+1) = w(1) + w(n)
  end subroutine
end subroutine

subroutine show(v, n)
  implicit none
  integer :: n
  real :: v(n)
  print '(1x, a, 5f6.0)', 'shown:   ', v
end subroutine

subroutine codes(word)
  implicit none
  character(len=*) :: word
  integer :: c(len(word)), i

  do i = 1, len(word)
    c(i) = ichar(word(i:i))
  end do
  print '(1x, a, i2, 3i4)', 'codes:', size(c), c
end subroutine

subroutine sized
  implicit none
  integer :: m
  common /sizes/ m
  integer :: k(m), j(max(m - 2, 0)), ends(-1:m)

  k = 7
  j = 8
  ends = 9
  print '(1x, a, 3i3, 2i4)', 'sizes:', size(k), size(j), size(ends), sum(k), sum(ends)
end subroutine

recursive integer function depth(n) result(r)
  implicit none
  integer :: n
  integer :: w(n)

  w = n
  if (n > 1) then
    r = depth(n - 1)
  else
    r = 0
  end if
  r = r + sum(w)
end function

subroutine pair(n, k)
  implicit none
  integer :: n, k
  real :: w(n), v(k)

  w = n
  v = k
  print '(1x, a, 2i3, 2f5.0)', 'pair: ', size(w), size(v), sum(w), sum(v)
  return

  entry pair2(k, n)
  w = 2 * n
  v = 2 * k
  print '(1x, a, 2i3, 2f5.0)', 'pair2:', size(w), size(v), sum(w), sum(v)
end subroutine

subroutine churn(total)
  implicit none
  real :: total
  integer :: m
  common /sizes/ m
  real :: big(m)

  big = 1.0
  total = total + big(m)
end subroutine
