! model.f90: Fortran 90's numeric model functions (docs/planning/PHASE-108 -
! Fortran 90 intrinsic procedures.md, subtask 5): EXPONENT and FRACTION,
! written in line, and SCALE, SET_EXPONENT, SPACING, RRSPACING, and NEAREST,
! FTNRTL's routines by format; of constants (folded), of variables and
! expressions, elementally on arrays, by keyword, and of REAL, REAL(8)
! (D_floating here, whose 56 digits gfortran's IEEE double hasn't, its
! lines compare.py's KNOWN), and
! REAL(16) (H_floating, IEEE's quadruple precision's 113 digits). Each VAX
! format is the model exactly, so every answer is exact.
program model
  implicit none
  integer, parameter :: q = 16
  real :: x(6) = (/ 3.0, -10.0, 0.1, 1.0, 0.0, -0.75 /)
  real :: r, s
  real(8) :: d = 3.0d0
  real(q) :: h = 3.0_q
  integer :: k, n(6)

  ! The standard's examples (13.13), folded when compiling.
  write (*, '(1x, a, i4, f8.4, 2f6.1)') 'Folded:', exponent(10.0), fraction(3.0), &
    scale(3.0, 2), set_exponent(3.0, 1)
  write (*, '(1x, a, 1p, e14.6, 0p, f12.1)') 'Spacings:', spacing(3.0), rrspacing(-3.0)
  write (*, '(1x, a, 2f12.8)') 'NEAREST:', nearest(3.0, 2.0), nearest(1.0, -1.0)

  ! The same of variables, so computed when running.
  r = 3.0
  s = -1.0
  write (*, '(1x, a, i4, f8.4, 2f6.1)') 'Run:', exponent(r + 7.0), fraction(r), &
    scale(r, 2), set_exponent(r, 1)
  write (*, '(1x, a, 1p, e14.6, 0p, f12.1)') 'Spacings:', spacing(r), rrspacing(-r)
  write (*, '(1x, a, 2f12.8)') 'NEAREST:', nearest(r, 2.0), nearest(r / 3.0, s)

  ! Elemental on arrays: EXPONENT and FRACTION of 0 are 0.
  n = exponent(x)
  write (*, '(1x, a, 6i5)') 'EXPONENT:', n
  write (*, '(1x, a, 6f8.4)') 'FRACTION:', fraction(x)
  write (*, '(1x, a, 6f8.2)') 'SCALE:   ', scale(x, (/ 1, 2, 3, -1, 5, -2 /))
  write (*, '(1x, a, 6f8.4)') 'SET_EXP: ', set_exponent(x, 0)
  write (*, '(1x, a, 6f10.0)') 'RRSPACE: ', rrspacing(x)
  write (*, '(1x, a, 1p, 5e12.4)') 'SPACING: ', spacing(x(1:4)), spacing(x(6))
  write (*, '(1x, a, 3f12.8)') 'NEAREST: ', nearest(x(4:6:2), 1.0), nearest(x(6), -1.0)

  ! By keyword, and S's sign alone counts, of whatever kind.
  write (*, '(1x, a, f6.1, f12.8, 2f6.2)') 'Keywords:', scale(i=1, x=r), &
    nearest(s=-1.0_q, x=1.0), set_exponent(i=-1, x=-r), fraction(x=0.5)

  ! A power of 2 and its neighbors: EXPONENT moves at the power.
  do k = 1, 3
    r = nearest(1.0, real(k - 2))
    if (k == 2) r = 1.0
    write (*, '(1x, a, i3, f12.8, f8.4, 1p, e14.6)') 'Around 1:', exponent(r), r, fraction(r), spacing(r)
  end do

  ! REAL(8), D_floating's 56 digits: a unit in the last place is 2**-54 at 3.
  write (*, '(1x, a, i4, f8.4, 2f6.1)') 'D:', exponent(d), fraction(d), scale(d, 2), set_exponent(d, 1)
  write (*, '(1x, a, 1p, e24.16, 0p, f22.1)') 'D spacings:', spacing(d), rrspacing(d)
  write (*, '(1x, a, 1p, e24.16)') 'D NEAREST:', nearest(d, 1.0) - d

  ! REAL(16), H_floating's 113 digits, as IEEE's quadruple precision has.
  write (*, '(1x, a, i4, f8.4, 2f6.1)') 'H:', exponent(h), fraction(h), scale(h, 2), set_exponent(h, 1)
  write (*, '(1x, a, 1p, e24.16)') 'H SPACING:', spacing(h)
  write (*, '(1x, a, 1p, e24.16)') 'H RRSPACING:', rrspacing(h)
  write (*, '(1x, a, 1p, e24.16)') 'H NEAREST:', nearest(h, -1.0) - h
  write (*, '(1x, a, i6, f8.4)') 'H big:', exponent(scale(h, 1000)), fraction(scale(-h, -1000))
end program model
