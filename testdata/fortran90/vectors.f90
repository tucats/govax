! Vector subscripts (Phase 102, subtask 6): an array's elements gathered by
! a vector of subscripts and scattered back, vectors computed from vectors,
! a vector beside a triplet, a vector of a vector, a permutation of an
! array's own elements (through a temporary), and an adjustable dummy vector.
program vectors
  implicit none
  real :: a(10), b(3), c(3, 4), d(3, 2)
  integer :: iv(3), jv(5), i, j

  iv(1) = 7
  iv(2) = 2
  iv(3) = 9

  call fill(a)
  b = a(iv)
  print '(1x, a, 3f5.0)', 'gather:  ', b

  a(iv) = -b
  print '(1x, a, 10f5.0)', 'scatter: ', a

  call fill(a)
  b = a(iv + 1) - a(11 - iv(3:1:-1))
  print '(1x, a, 3f5.0)', 'computed:', b

  do j = 1, 4
    do i = 1, 3
      c(i, j) = 10 * i + j
    end do
  end do
  jv(1) = 4
  jv(2) = 1
  d = c(:, jv(1:2))
  do i = 1, 3
    print '(1x, a, i1, a, 2f5.0)', 'd(', i, ',:):  ', (d(i, j), j = 1, 2)
  end do

  jv(1) = 3
  jv(2) = 1
  jv(3) = 2
  b = a(iv(jv(1:3)))
  print '(1x, a, 3f5.0)', 'twice:   ', b

  a(1:3) = a(iv)
  print '(1x, a, 10f5.0)', 'permuted:', a

  call fill(a)
  call zero(a, iv, 3)
  print '(1x, a, 10f5.0)', 'zeroed:  ', a
end program vectors

! Fills x with 10, 20, ..., 100.
subroutine fill(x)
  implicit none
  real :: x(10)
  integer :: i

  do i = 1, 10
    x(i) = 10 * i
  end do
end subroutine fill

! Zeroes the elements of x that kv names.
subroutine zero(x, kv, m)
  implicit none
  integer :: m, kv(m)
  real :: x(10)

  x(kv) = 0.0
end subroutine zero
