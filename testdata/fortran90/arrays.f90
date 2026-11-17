! Fortran 90's array assignment (Phase 102, subtask 4): whole arrays and
! sections assigned, each statement a nest of loops in array element order.
! An elemental expression of whole arrays, a comparison into a LOGICAL array,
! a section moved to another array's section, a negative stride, a column and
! a row of a matrix, a scalar broadcast, extents known only when the program
! runs (one of them zero), a stride of 2, a CHARACTER array's substrings and a
! comparison of it, DOUBLE PRECISION and COMPLEX arrays, and an adjustable
! dummy array. Each result is printed whole, or by an implied DO.
program arrays
  implicit none
  real :: a(10), b(10), c(4, 6)
  integer :: i, j, n, k(5)
  logical :: l(10)
  character(len=4) :: names(3)
  double precision :: d(3)
  complex :: z(2)

  do i = 1, 10
    b(i) = i
  end do

  a = b + 2.0 * b
  print '(1x, a, 10f4.0)', 'a:     ', a

  l = a > 15.0
  print '(1x, a, 10l2)', 'l:     ', l

  a(2:10) = b(1:9)
  print '(1x, a, 10f4.0)', 'shift: ', a

  b(10:1:-1) = a
  print '(1x, a, 10f4.0)', 'rev:   ', b

  c = 0.0
  c(:, 3) = a(1:4)
  c(2, :) = 1.0
  c = c + 10.0
  do i = 1, 4
    print '(1x, a, i1, a, 6f4.0)', 'c(', i, ',:): ', (c(i, j), j = 1, 6)
  end do

  n = 4
  a(1:n) = b(n+1:n+n) * 2
  print '(1x, a, 10f4.0)', 'a(1:n):', a

  n = 0
  a(1:n) = -1.0
  a(5:4) = -1.0
  print '(1x, a, 10f4.0)', 'empty: ', a

  k = 7
  k(2:5:2) = -1
  k = -k
  print '(1x, a, 5i4)', 'k:     ', k

  names = 'abcd'
  names(:)(2:3) = 'XY'
  names(2)(1:1) = 'z'
  l(1:3) = names == 'aXYd'
  print '(1x, a, 3(1x, a), 3l2)', 'names: ', names, (l(i), i = 1, 3)

  d = 2.5
  d = d * d
  print '(1x, a, 3f6.2)', 'd:     ', d

  z = (1.0, 2.0)
  z = z * z
  print '(1x, a, 4f5.1)', 'z:     ', z

  call scale(a, 10)
  print '(1x, a, 10f4.0)', 'scaled:', a
end program arrays

! An adjustable dummy array, whole and as a section whose upper bound is the
! array's own.
subroutine scale(x, m)
  implicit none
  integer :: m
  real :: x(m)

  x = x * 2.0
  x(2:) = x(2:) + 1.0
end subroutine scale
