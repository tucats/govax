! Phase 113, subtask 2: EXPONENT and FRACTION of a variable move its bits as
! an integer, so that a reserved operand (the sign set, the exponent 0: a
! VAX's own form, which TRANSFER makes here) doesn't fault in the move, as
! MOVF would. EXPONENT of one is 0, as of 0; FRACTION of a value as usual.
! VMS's own, so not in compare.py.
program modelraw
  implicit none
  real :: x
  real(8) :: d
  real(16) :: h

  x = transfer(32768, x)
  d = transfer((/ 32768, 0 /), d)
  h = transfer((/ 32768, 0, 0, 0 /), h)
  write (*, '(1x, a, 3i4)') 'reserved:', exponent(x), exponent(d), exponent(h)
  x = 6.0
  d = -0.375d0
  h = 1024.0_16
  write (*, '(1x, a, 3i4)') 'exponent:', exponent(x), exponent(d), exponent(h)
  write (*, '(1x, a, 3f8.4)') 'fraction:', fraction(x), fraction(d), fraction(h)
end program modelraw
