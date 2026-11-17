! Fortran 90's array functions (Phase 102, subtask 9): SUM, PRODUCT, MAXVAL,
! and MINVAL whole, with MASK=, and with DIM=; COUNT, ANY, and ALL, one with
! DIM=; MAXLOC and MINLOC, with a MASK= that finds something and one that
! finds nothing (zeros); a sum of no elements; DOT_PRODUCT of REAL and COMPLEX
! vectors; MATMUL of a matrix and a vector and of two matrices; TRANSPOSE; a
! vector divided by its own sum; and SIZE, SHAPE, LBOUND, UBOUND, and a sum by
! DIM= of an adjustable array.
program reduce
  implicit none
  real :: a(3, 4), b(4, 3), m(2, 3), p(2, 4), v(3), w(3), r(3), c(4)
  integer :: i, j, k(2), cs(4)
  logical :: l(4)
  complex :: z(2), zz

  do j = 1, 4
    do i = 1, 3
      a(i, j) = i + 3 * (j - 1)
    end do
  end do

  print '(1x, a, 2f6.0)', 'sum, masked: ', sum(a), sum(a, mask=a > 6.0)
  print '(1x, a, 3f6.0)', 'prod, max, min:', product(a(1, :)), maxval(a), minval(a)

  c = sum(a, dim=1)
  print '(1x, a, 4f6.0)', 'column sums: ', c
  r = sum(a, dim=2)
  print '(1x, a, 3f6.0)', 'row sums:    ', r
  r = maxval(a, dim=2)
  print '(1x, a, 3f6.0)', 'row maxima:  ', r

  print '(1x, a, i6, 3l3)', 'count, any, all:', count(a > 5.0), any(a > 11.0), all(a > 0.0), all(a > 1.0)
  l = any(a > 11.0, dim=1)
  print '(1x, a, 4l3)', 'any by column:', l

  k = maxloc(a)
  print '(1x, a, 2i3)', 'maxloc:     ', k
  k = minloc(a)
  print '(1x, a, 2i3)', 'minloc:     ', k
  k = maxloc(a, mask=a < 7.0)
  print '(1x, a, 2i3)', 'masked:     ', k
  k = maxloc(a, mask=a > 99.0)
  print '(1x, a, 2i3)', 'none:       ', k

  print '(1x, a, 2f6.0)', 'empty sum, product:', sum(a(1:0, :)), product(a(1:0, :))

  v = (/ 1.0, 2.0, 3.0 /)
  w = (/ 4.0, 5.0, 6.0 /)
  print '(1x, a, f6.0)', 'dot product:', dot_product(v, w)
  z = (/ (1.0, 2.0), (3.0, -1.0) /)
  zz = dot_product(z, z)
  print '(1x, a, 2f6.0)', 'complex dot:', zz

  c = 1.0
  r = matmul(a, c)
  print '(1x, a, 3f6.0)', 'matmul a, c:', r

  m = 0.0
  m(1, 1) = 1.0
  m(2, 2) = 1.0
  m(2, 3) = 1.0
  p = matmul(m, a)
  do i = 1, 2
    print '(1x, a, i1, a, 4f6.0)', 'p(', i, ',:):', (p(i, j), j = 1, 4)
  end do

  b = transpose(a)
  do i = 1, 4
    print '(1x, a, i1, a, 3f6.0)', 'b(', i, ',:):', (b(i, j), j = 1, 3)
  end do

  v = v / sum(v)
  print '(1x, a, 3f7.3)', 'shares:', v

  call stats(a, 3, 4)
end program reduce

! An adjustable array's size, shape, and bounds, and a section's sums.
subroutine stats(x, m, n)
  implicit none
  integer :: m, n, s(2), lo(2), hi(2)
  real :: x(0:m-1, n), c(4)

  s = shape(x)
  lo = lbound(x)
  hi = ubound(x)
  print '(1x, a, i4, 6i3)', 'size, shape, bounds:', size(x), s, lo, hi
  c = sum(x(1:, :), dim=1)
  print '(1x, a, 4f6.0)', 'lower rows:', c
end subroutine stats
