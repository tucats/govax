! modelvms.f90: the numeric model's functions at the VAX formats' edges
! (docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md, subtask 5),
! which IEEE's formats, with their denormals and infinities, answer
! otherwise: TINY and HUGE in F, D, G, and H, 0 below TINY, and an overflow
! past HUGE, which FTNRTL makes a real one, SS$_FLTOVF_F (%X000004B4) from a
! MULx2, here made each function's value by LIB$SIG_TO_RET. G_floating is a
! unit compiled OPTIONS /G_FLOATING.
program modelvms
  implicit none
  integer, parameter :: q = 16
  real :: t, b, z, one
  real(8) :: dt, db
  real(q) :: ht, hb
  integer :: k
  integer, external :: fscale, fsetexp, fnear, dnear, hscale, gedges

  z = 0.0
  one = 1.0
  t = tiny(t)
  b = huge(b)
  write (*, '(1x, a, 2i5, 2i5)') 'EXPONENT:', exponent(t), exponent(b), exponent(-t), exponent(z)
  write (*, '(1x, a, 2f12.8)') 'FRACTION:', fraction(t), fraction(-b)
  write (*, '(1x, a, 1p, 2e16.8)') 'NEAREST(0):', nearest(z, one), nearest(z, -one)
  write (*, '(1x, a, 1p, 2e16.8)') 'Toward 0:', nearest(t, -one), nearest(-t, one)
  write (*, '(1x, a, 1p, 2e16.8)') 'SPACING:', spacing(z), spacing(t)
  write (*, '(1x, a, 1p, 2e16.8)') 'SPACING:', spacing(scale(one, -100)), spacing(b)
  write (*, '(1x, a, 1p, 2e16.8)') 'Below:', scale(t, -1), set_exponent(b, -200)
  write (*, '(1x, a, 2f14.1)') 'RRSPACING:', rrspacing(b), rrspacing(t)
  write (*, '(1x, a, l2, 1p, e16.8)') 'Below HUGE:', nearest(b, -one) < b, b - nearest(b, -one)
  write (*, '(1x, a, l2)') 'And back:', nearest(nearest(b, -one), one) == b

  ! Past HUGE: an overflow, signaled.
  write (*, '(1x, a, z9.8)') 'SCALE(HUGE, 1):     %X', fscale(b, 1)
  write (*, '(1x, a, z9.8)') 'SCALE(-1, 128):     %X', fscale(-one, 128)
  write (*, '(1x, a, z9.8)') 'SCALE(1, 2**30):    %X', fscale(one, 2**30)
  write (*, '(1x, a, z9.8)') 'SET_EXPONENT(1, 128): %X', fsetexp(one, 128)
  write (*, '(1x, a, z9.8)') 'NEAREST(HUGE, 1):   %X', fnear(b, one)
  write (*, '(1x, a, z9.8)') 'NEAREST(-HUGE, -1): %X', fnear(-b, -one)
  write (*, '(1x, a, z9.8)') 'Within:             %X', fscale(one, 126)

  ! D_floating: F's range, 56 digits.
  dt = tiny(dt)
  db = huge(db)
  write (*, '(1x, a, 2i5, 1p, e24.16)') 'D:', exponent(dt), exponent(db), spacing(dt)
  write (*, '(1x, a, f22.1)') 'D RRSPACING:', rrspacing(db)
  write (*, '(1x, a, 1p, e24.16)') 'D NEAREST(0):', nearest(0.0d0, -one)
  write (*, '(1x, a, z9.8)') 'D NEAREST(HUGE, 1): %X', dnear(db)

  ! H_floating: the widest range, 113 digits.
  ht = tiny(ht)
  hb = huge(hb)
  write (*, '(1x, a, 2i7, 1p, e14.4e4)') 'H:', exponent(ht), exponent(hb), spacing(ht)
  write (*, '(1x, a, 1p, 2e14.4e4)') 'H NEAREST(0):', nearest(0.0_q, one), nearest(-ht, one)
  write (*, '(1x, a, f8.4, 1p, e14.4e4)') 'H:', fraction(-ht), scale(ht, -1)
  write (*, '(1x, a, z9.8)') 'H SCALE(HUGE, 1):   %X', hscale(hb)

  ! G_floating.
  k = gedges()
  write (*, '(1x, a, z9.8)') 'G SCALE(HUGE, 1):   %X', k
end program modelvms

integer function fscale(x, i)
  real :: x
  integer :: i
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  fscale = int(scale(x, i) / huge(x))
end function fscale

integer function fsetexp(x, i)
  real :: x
  integer :: i
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  fsetexp = int(set_exponent(x, i) / huge(x))
end function fsetexp

integer function fnear(x, s)
  real :: x, s
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  fnear = int(nearest(x, s) / huge(x))
end function fnear

integer function dnear(x)
  real(8) :: x
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  dnear = int(nearest(x, 1.0) / huge(x))
end function dnear

integer function hscale(x)
  real(16) :: x
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  hscale = int(scale(x, 1) / huge(x))
end function hscale

options /g_floating
integer function gedges()
  real(8) :: g, gt, gb
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  g = 3.0d0
  gt = tiny(gt)
  gb = huge(gb)
  write (*, '(1x, a, 2i6, f8.4, 1p, e24.16)') 'G:', exponent(gt), exponent(gb), fraction(g), spacing(g)
  write (*, '(1x, a, f20.1, 1p, e24.16)') 'G:', rrspacing(g), nearest(g, -1.0) - g
  write (*, '(1x, a, 1p, 2e24.16)') 'G TINY:', spacing(gt), nearest(0.0d0, 1.0)
  write (*, '(1x, a, f6.1)') 'G SCALE:', scale(g, -1)
  gedges = int(scale(gb, 1) / gb)
end function gedges
