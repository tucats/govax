! Array constants and constructors (Phase 102, subtask 7): array PARAMETERs,
! one made by RESHAPE, SIZE and UBOUND of them as constants, an initialized
! array, elements read with subscripts not constant, sections and an
! operation on constants folded when compiling, a constructor of values with
! an implied DO whose variable is its own (the program's J is unchanged),
! one stored straight into a section whose extent is known only when the
! program runs, one that reads its own target (through a temporary), a
! constant vector subscript, and a CHARACTER PARAMETER's section.
program constants
  implicit none
  integer, parameter :: primes(6) = (/ 2, 3, 5, 7, 11, 13 /)
  real, parameter :: m(2, 3) = reshape((/ 1.0, 2.0, 3.0, 4.0, 5.0, 6.0 /), (/ 2, 3 /))
  integer, parameter :: n = size(primes), rows = size(m, 1), cols = ubound(m, 2)
  character(len=3), parameter :: days(3) = (/ 'mon', 'tue', 'wed' /)
  integer :: i, j, k(6), sh(2), first
  integer :: squares(5) = (/ (i * i, i = 1, 5) /)
  real :: a(6), b(2, 3), x
  character(len=3) :: names(3)

  print '(1x, a, 6i4)', 'primes: ', (primes(i), i = 1, n)
  print '(1x, a, 3i4)', 'sizes:  ', n, rows, cols
  print '(1x, a, 5i4)', 'squares:', (squares(i), i = 1, 5)

  k = primes(6:1:-1) * 2
  print '(1x, a, 6i4)', 'twice:  ', k

  k = (/ 1, 2, 3, 4, 5, 6 /) + primes
  print '(1x, a, 6i4)', 'sums:   ', k

  b = m * 10.0
  do i = 1, rows
    print '(1x, a, i1, a, 3f5.0)', 'b(', i, ',:): ', (b(i, j), j = 1, cols)
  end do

  sh = shape(m)
  print '(1x, a, 2i4)', 'shape:  ', sh

  x = 0.5
  a = (/ x, (j * x, j = 1, 4), x + 10.0 /)
  print '(1x, a, 6f5.1)', 'a:      ', a
  print '(1x, a, i4)', 'j:      ', j

  first = 2
  a(first:first + 2) = (/ (real(primes(i)), i = first, first + 2) /)
  print '(1x, a, 6f5.1)', 'filled: ', a

  a = (/ a(6), a(1:5) /)
  print '(1x, a, 6f5.1)', 'rotated:', a

  k(1:3) = primes((/ 6, 4, 2 /))
  print '(1x, a, 6i4)', 'picked: ', k

  names = days(3:1:-1)
  print '(1x, a, 3(1x, a))', 'days:   ', names
end program constants
