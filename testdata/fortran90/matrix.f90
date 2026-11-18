! A matrix program (Phase 102, subtask 12): a linear system solved by Gaussian
! elimination with partial pivoting, its row operations on sections of an
! adjustable dummy array, the pivot found by MAXLOC of a section, rows swapped
! as sections, back substitution by DOT_PRODUCT; the determinant as a PRODUCT of
! the diagonal, a constructor's; the inverse a column at a time, checked by
! MATMUL; and an INTEGER matrix's Gram matrix, MATMUL of its TRANSPOSE,
! symmetric, its trace, row sums and column sums by DIM=, and a power.
program matrix
  implicit none
  integer, parameter :: n = 4
  real, parameter :: a0(n, n) = reshape((/ 2.0, 4.0, -2.0, 1.0,   &
                                           1.0, 1.0, 5.0, -1.0,   &
                                           -1.0, 0.0, 2.0, 3.0,   &
                                           3.0, 2.0, -1.0, 2.0 /), (/ n, n /))
  real, parameter :: b0(n) = (/ 14.0, 12.0, 3.0, 2.0 /)
  real :: a(n, n), b(n), x(n), r(n), inv(n, n), e(n), det
  integer :: ia(n, n), g(n, n), p(n, n), i, j, swaps

  print '(1x, a)', 'The matrix and its right side:'
  do i = 1, n
    print '(1x, 4f6.1, 3x, f6.1)', a0(i, :), b0(i)
  end do

  a = a0
  b = b0
  call solve(n, a, b, x, swaps)
  print '(1x, a, 4f8.3)', 'Solution:   ', x
  print '(1x, a, i2)', 'Row swaps:  ', swaps
  print '(1x, a)', 'Reduced:'
  do i = 1, n
    print '(1x, 4f8.3)', (a(i, j), j = i, n)
  end do
  det = product((/ (a(i, i), i = 1, n) /)) * (-1) ** swaps
  print '(1x, a, f9.3)', 'Determinant:', det
  r = b0 - matmul(a0, x)
  print '(1x, a, l2)', 'Residual small:', maxval(abs(r)) < 1.0e-4

  do j = 1, n
    e = 0.0
    e(j) = 1.0
    a = a0
    call solve(n, a, e, x, swaps)
    inv(:, j) = x
  end do
  print '(1x, a)', 'The inverse times 1000, rounded:'
  do i = 1, n
    print '(1x, 4i7)', nint(1000.0 * inv(i, :))
  end do
  print '(1x, a)', 'The matrix times its inverse, rounded:'
  do i = 1, n
    print '(1x, 4i3)', nint(matmul(a0(i, :), inv))
  end do

  ia = nint(a0)
  g = matmul(transpose(ia), ia)
  print '(1x, a)', 'Its Gram matrix:'
  do i = 1, n
    print '(1x, 4i5)', g(i, :)
  end do
  print '(1x, a, l2)', 'Symmetric:', all(g == transpose(g))
  print '(1x, a, i5)', 'Trace:', sum((/ (g(i, i), i = 1, n) /))
  print '(1x, a, 4i5)', 'Row sums:   ', sum(ia, dim=2)
  print '(1x, a, 4i5)', 'Column sums:', sum(ia, dim=1)
  print '(1x, a, 4i5)', 'Largest:    ', maxval(ia, dim=1)
  p = ia
  do i = 2, 3
    p = matmul(p, ia)
  end do
  print '(1x, a)', 'Its cube:'
  do i = 1, n
    print '(1x, 4i6)', p(i, :)
  end do
end program matrix

! solve reduces a to an upper triangle, with b, and solves for x; swaps counts
! the rows exchanged.
subroutine solve(n, a, b, x, swaps)
  implicit none
  integer, intent(in) :: n
  real, intent(inout) :: a(n, n), b(n)
  real, intent(out) :: x(n)
  integer, intent(out) :: swaps
  real :: row(10), f, t
  integer :: i, k, p, loc(1)

  swaps = 0
  do k = 1, n - 1
    loc = maxloc(abs(a(k:n, k)))
    p = k - 1 + loc(1)
    if (p /= k) then
      row(1:n) = a(k, :)
      a(k, :) = a(p, :)
      a(p, :) = row(1:n)
      t = b(k)
      b(k) = b(p)
      b(p) = t
      swaps = swaps + 1
    end if
    do i = k + 1, n
      f = a(i, k) / a(k, k)
      a(i, k:n) = a(i, k:n) - f * a(k, k:n)
      b(i) = b(i) - f * b(k)
    end do
  end do
  do i = n, 1, -1
    x(i) = (b(i) - dot_product(a(i, i+1:n), x(i+1:n))) / a(i, i)
  end do
end subroutine solve
