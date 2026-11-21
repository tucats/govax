! The parts of a COMPLEX(8) (Phase 111, subtask 2a): REAL and AIMAG of one
! are REAL(8), the standard's 13.13.84 and 13.13.6, where VAX FORTRAN's REAL
! of a COMPLEX*16 is a REAL*4 and its AIMAG takes only a COMPLEX*8; and
! CMPLX(X, Y, 8) of constants keeps every digit, as CMPLX of variables does.
! A REAL*4 0.1 shows as 0.1000000014901161 here.
program cparts
  implicit none
  complex(8) :: z, w
  real(8) :: x, y

  x = 0.1_8
  y = 0.2_8
  z = cmplx(x, y, 8)
  w = cmplx(0.1_8, 0.2_8, 8)
  print '(1x, a, 2f20.16)', 'variables:', z
  print '(1x, a, 2f20.16)', 'constants:', w
  print '(1x, a, 2f20.16)', 'parts:    ', real(z), aimag(z)
  print '(1x, a, 2i3)', 'kinds:    ', kind(real(z)), kind(aimag(z))
  x = real(w) * 3
  print '(1x, a, f20.16)', 'product:  ', x
  print '(1x, a, f20.16)', 'part of a constant:', aimag(cmplx(0.1_8, 0.7_8, 8))
end program cparts
