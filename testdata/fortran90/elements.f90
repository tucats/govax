! Elemental intrinsic functions on arrays (Phase 102, subtask 8): ABS, MAX
! with a scalar, NINT, SQRT (MTHRTL's, called for each element), MOD, BTEST,
! MERGE of numbers and of CHARACTER values, MIN on a matrix, ICHAR of a
! section of substrings, and DBLE of a section.
program elements
  implicit none
  real :: a(6), b(6), c(2, 3)
  integer :: k(6), i, j
  logical :: l(6)
  character(len=3) :: s(4), t(4)
  double precision :: d(3)

  do i = 1, 6
    a(i) = i - 3.5
  end do

  b = abs(a)
  print '(1x, a, 6f6.1)', 'abs:   ', b

  b = max(a, 0.0) * 2.0
  print '(1x, a, 6f6.1)', 'max:   ', b

  k = nint(a * 2.0)
  print '(1x, a, 6i6)', 'nint:  ', k

  b = sqrt(abs(a) * 4.0)
  print '(1x, a, 6f6.3)', 'sqrt:  ', b

  k = mod(k, 3)
  print '(1x, a, 6i6)', 'mod:   ', k

  l = btest(k, 0)
  print '(1x, a, 6l6)', 'btest: ', l

  b = merge(a, -a, l)
  print '(1x, a, 6f6.1)', 'merge: ', b

  do j = 1, 3
    do i = 1, 2
      c(i, j) = i * 10 + j
    end do
  end do
  c = min(c, 21.5)
  do i = 1, 2
    print '(1x, a, i1, a, 3f6.1)', 'c(', i, ',:):', (c(i, j), j = 1, 3)
  end do

  s = (/ 'ab ', 'cd ', 'ef ', 'gh ' /)
  k(1:4) = ichar(s(:)(1:1)) - ichar('a')
  print '(1x, a, 4i6)', 'ichar: ', (k(i), i = 1, 4)

  t = merge(s, 'zz ', k(1:4) > 2)
  print '(1x, a, 4(1x, a2))', 'names: ', (t(i)(1:2), i = 1, 4)

  d = dble(a(4:6)) * 2d0
  print '(1x, a, 3f6.1)', 'dble:  ', d
end program elements
