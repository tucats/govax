! Arrays in I/O lists (Phase 102, subtask 11): sections, with strides, going
! down, and of two dimensions, written an element at a time in array element
! order; array expressions, an array constant and a section of one, and
! constructors with implied DOs; elemental and transformational functions'
! values; sections in an implied-DO list; CHARACTER sections with substrings
! and concatenation; reading into sections, one whose extent was read by the
! same statement, one with a vector subscript, and a section's substrings;
! sections in an internal file's list and in unformatted records.
program arrayio
  implicit none
  integer, parameter :: p(5) = (/ 2, 3, 5, 7, 11 /)
  real :: a(8), b(2, 3), c(4)
  integer :: i, n, iv(3), k(6)
  character(len=4) :: w(3)
  character(len=20) :: line

  a = (/ (real(i), i = 1, 8) /)
  print '(1x, a, 4f5.1)', 'odd:     ', a(1:8:2)
  print '(1x, a, 8f5.1)', 'down:    ', a(8:1:-1)
  print '(1x, a, 8f5.1)', 'sums:    ', a(1:4) + a(5:8), 2.0 * a(7:8)

  b = reshape((/ 1.0, 2.0, 3.0, 4.0, 5.0, 6.0 /), (/ 2, 3 /))
  print '(1x, a, 6f5.1)', 'whole:   ', b
  print '(1x, a, 3f5.1)', 'row 2:   ', b(2, :)
  print '(1x, a, 2f5.1)', 'col 3:   ', b(:, 3)
  print '(1x, a, 4f5.1)', 'corner:  ', b(:, 2:3)
  print '(1x, a, 6f5.1)', 'rows:    ', (b(i, :), i = 1, 2)
  print '(1x, a, 6f5.1)', 'turned:  ', transpose(b)
  print '(1x, a, 3f5.1)', 'colsums: ', sum(b, dim = 1)
  print '(1x, a, 2f5.1)', 'roots:   ', sqrt(b(1, 2:3) + 5.0)

  print '(1x, a, 5i4)', 'primes:  ', p
  print '(1x, a, 3i4)', 'middle:  ', p(2:4)
  print '(1x, a, 5i4)', 'squares: ', p * p
  print '(1x, a, 6i4)', 'built:   ', (/ 0, (i * i, i = 1, 4), 100 /)
  print '(1x, a, 2i4)', 'where:   ', maxloc(b)
  print '(1x, a, 2i4)', 'shape:   ', shape(b)

  w = (/ 'abcd', 'efgh', 'ijkl' /)
  print '(1x, a, 3a3)', 'cut:     ', w(:)(2:3)
  print '(1x, a, 3a5)', 'joined:  ', w(3:1:-1) // '!'

  read *, n, a(1:n)
  print '(1x, a, i2, 8f5.1)', 'read:    ', n, a
  iv = (/ 4, 1, 3 /)
  c = 0.0
  read *, c(iv)
  print '(1x, a, 4f5.1)', 'scatter: ', c
  read *, b(:, 2)
  print '(1x, a, 6f5.1)', 'column:  ', b
  read '(3a2)', w(:)(3:4)
  print '(1x, a, 3a5)', 'ends:    ', w

  write (line, '(5i4)') p(5:1:-1)
  k = 0
  print '(1x, a, a)', 'internal:', line
  read (line, '(3i4)') k(2:6:2)
  print '(1x, a, 6i4)', 'evens:   ', k(2:6:2)

  open (unit = 1, status = 'scratch', form = 'unformatted')
  write (1) a(8:1:-2), p(1:3)
  rewind (1)
  k = 0
  read (1) c, k(1:5:2)
  close (1)
  print '(1x, a, 4f5.1, 6i3)', 'records: ', c, k
end program arrayio
