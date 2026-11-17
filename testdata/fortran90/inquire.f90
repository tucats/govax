! inquire.f90: Fortran 90's kind and inquiry functions (docs/planning/PHASE-100 -
! Fortran 90 declarations and kinds.md, subtask 6): kinds chosen by
! SELECTED_INT_KIND and SELECTED_REAL_KIND and named by KIND, in PARAMETERs and
! in executable statements; HUGE, TINY, EPSILON, DIGITS, PRECISION, RANGE,
! RADIX, MINEXPONENT, MAXEXPONENT, and BIT_SIZE of every integer kind and of
! REAL; KIND= on INT, NINT, REAL, CMPLX, and LOGICAL; keyword arguments; and a
! dummy argument asked about. The real model's answers are the VAX's, and
! gfortran's IEEE ones differ (compare.py's KNOWN).
program inquire
  implicit none
  integer, parameter :: sp = kind(1.0), dp = kind(1.0d0)
  integer, parameter :: i1 = selected_int_kind(2), i2 = selected_int_kind(4)
  integer, parameter :: i4 = selected_int_kind(9), wide = selected_real_kind(p=10)
  integer, parameter :: biggest = huge(0)
  integer(i1) :: b
  integer(i2) :: h
  integer :: n
  real(sp) :: x
  real(dp) :: d
  real(wide) :: w
  complex(dp) :: z
  logical(1) :: l1
  logical :: l4

  write (*, '(1x, a, 3i3)') 'Integer kinds:', i1, i2, i4
  write (*, '(1x, a, 3i3)') 'Real kinds:', sp, dp, wide
  write (*, '(1x, a, i3)') 'No such kind:', selected_int_kind(40)
  write (*, '(1x, a, 4i3)') 'KIND:', kind(b), kind(h), kind(z), kind(l1)
  write (*, '(1x, a, 3i11)') 'HUGE:', huge(b), huge(h), biggest
  write (*, '(1x, a, 3i4)') 'DIGITS:', digits(b), digits(h), digits(n)
  write (*, '(1x, a, 3i4)') 'RANGE:', range(b), range(h), range(n)
  write (*, '(1x, a, 3i4)') 'BIT_SIZE:', bit_size(b), bit_size(h), bit_size(n)

  write (*, '(1x, a, i4, i3)') 'Real digits:', digits(x), precision(x)
  write (*, '(1x, a, i4)') 'Real range:', range(x)
  write (*, '(1x, a, 2i5)') 'Exponents:', minexponent(x), maxexponent(x)
  write (*, '(1x, a, i2)') 'RADIX:', radix(x)
  write (*, '(1x, a, e15.7)') 'EPSILON:', epsilon(x)
  write (*, '(1x, a, e15.7)') 'HUGE:', huge(x)
  write (*, '(1x, a, e15.7)') 'TINY:', tiny(x)
  call double(d)

  x = 2.75
  n = 7
  b = int(x, kind=i1)
  h = nint(-x, i2)
  write (*, '(1x, a, 2i3)') 'INT and NINT:', b, h
  write (*, '(1x, a, 3i3)') 'Their kinds:', kind(int(x, kind=i1)), kind(nint(x, i2)), kind(int(a=x))
  d = real(n, kind=dp) / 4
  write (*, '(1x, a, f8.4, i3)') 'REAL:', d, kind(real(n, dp))
  z = cmplx(1.5, -2.0, kind=dp)
  write (*, '(1x, a, 2f6.2, i3)') 'CMPLX:', z, kind(z)
  l1 = logical(.true., kind=1)
  l4 = logical(l1)
  write (*, '(1x, a, 2i3, l2)') 'LOGICAL:', kind(l1), kind(logical(l1)), l4

  w = 1.0_wide / 3.0_wide
  write (*, '(1x, a, l2)') 'Wide enough:', precision(w) >= 10
  write (*, '(1x, a, 2i3)') 'Keywords:', selected_real_kind(r=30), selected_real_kind(r=30, p=6)
  write (*, '(1x, a, i3)') 'For 15 digits and 307:', selected_real_kind(15, 307)
end program inquire

! double asks about its dummy argument, whose value it never uses.
subroutine double(v)
  real(kind=8), intent(in) :: v
  write (*, '(1x, a, i4, i3, i4)') 'Double:', digits(v), precision(v), range(v)
  write (*, '(1x, a, e15.7)') 'Its EPSILON:', epsilon(v)
end subroutine double
