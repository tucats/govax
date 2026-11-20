! numfns.f90: Fortran 90's numeric functions and COMPLEX(16)'s (docs/planning/
! PHASE-108 - Fortran 90 intrinsic procedures.md, subtask 4): CEILING and
! FLOOR of each REAL kind, MODULO of INTEGERs and REALs, beside MOD, folded
! (in PARAMETERs too, of INTEGERs: the standard's 7.1.6.1), in line, on
! arrays, and by keyword; then ABS, SQRT, EXP, LOG, SIN, and COS of
! COMPLEX(16) values (FTNRTL's), their powers in line and called, and MERGE
! of them. COMPLEX(16) has no I/O yet, so its values are printed by their
! parts.
program numfns
  implicit none
  integer, parameter :: q = 16
  integer, parameter :: m = modulo(-8, 5), m2 = modulo(8, -5)
  real :: x(6) = (/ 2.5, -2.5, 3.0, -3.0, 0.1, -0.1 /)
  real(8) :: d = -7.25d0
  real(q) :: h = 123456.75_q, big
  integer :: ia(4) = (/ 8, -8, 8, -8 /), ip(4) = (/ 5, 5, -5, -5 /)
  integer(2) :: i2 = -7
  integer(1) :: i1 = -7
  real :: r(4) = (/ 8.5, -8.5, 8.5, -8.5 /), p(4) = (/ 3.0, 3.0, -3.0, -3.0 /)
  complex(q) :: z, w, zs(3)
  integer :: n

  ! CEILING and FLOOR: of constants, of each kind, of an array.
  write (*, '(1x, a, 4i4)') 'Folded:', ceiling(4.2), floor(-4.2), m, m2
  write (*, '(1x, a, f6.2)') 'MODULO(-8.5, 3.0):', modulo(-8.5, 3.0)
  write (*, '(1x, a, 6i4)') 'CEILING:', ceiling(x)
  write (*, '(1x, a, 6i4)') 'FLOOR:  ', floor(x)
  write (*, '(1x, a, 2i4, 2i8)') 'Kinds:', ceiling(d), floor(d), ceiling(h), floor(h)
  write (*, '(1x, a, 2i10)') 'Whole:', ceiling(16777216.0), floor(a=-0.5)

  ! MODULO beside MOD: P's sign, where MOD has A's.
  write (*, '(1x, a, 4i4)') 'MOD:     ', mod(ia, ip)
  write (*, '(1x, a, 4i4)') 'MODULO:  ', modulo(ia, ip)
  write (*, '(1x, a, 4i4)') 'By 5, -5:', modulo(ia(1:2), 5), modulo(ia(3:4), -5)
  write (*, '(1x, a, 3i4)') 'Short:', modulo(i2, 3_2), modulo(i1, 3_1), modulo(p=5, a=-10)
  write (*, '(1x, a, 4f6.2)') 'REAL:', modulo(r, p)
  write (*, '(1x, a, 2f6.2)') 'Kinds:', modulo(-1.25d0, 1.0d0), real(modulo(-10.0_q, 4.0_q))

  ! COMPLEX(16)'s functions.
  z = (3.0_q, 4.0_q)
  write (*, '(1x, a, t20, f12.6)') 'ABS((3, 4))', abs(z)
  big = 2.0_q ** 13000
  write (*, '(1x, a, t20, f12.6)') 'ABS, scaled', abs(cmplx(3 * big, 4 * big, kind=q)) / big
  call show('SQRT((3, 4))', sqrt(z))
  call show('SQRT((3, -4))', sqrt(conjg(z)))
  call show('SQRT((-3, 4))', sqrt(-conjg(z)))
  call show('SQRT((-3, -4))', sqrt(-z))
  call show('SQRT((-4, 0))', sqrt((-4.0_q, 0.0_q)))
  call show('SQRT((0, 0))', sqrt((0.0_q, 0.0_q)))
  call show('EXP((1, 0))', exp((1.0_q, 0.0_q)))
  call show('EXP((0, 0.5))', exp((0.0_q, 0.5_q)))
  call show('LOG((3, 4))', log(z))
  call show('LOG((-1, 0))', log((-1.0_q, 0.0_q)))
  call show('LOG((0, 1))', log((0.0_q, 1.0_q)))
  call show('SIN((1, 1))', sin((1.0_q, 1.0_q)))
  call show('COS((1, 1))', cos((1.0_q, 1.0_q)))

  ! Powers: in line for a small constant exponent, else called.
  z = (1.0_q, 1.0_q)
  call show('Z**2', z**2)
  call show('Z**3', z**3)
  call show('Z**8', z**8)
  n = 10
  call show('Z**10', z**n)
  call show('Z**(-1)', z**(-1))
  n = -2
  call show('Z**(-2)', z**n)
  n = 0
  call show('Z**0', z**n)
  call show('(2, 0)**(3, 0)', (2.0_q, 0.0_q)**(3.0_q, 0.0_q))
  call show('(0, 1)**(0, 1)', (0.0_q, 1.0_q)**(0.0_q, 1.0_q))
  call show('2.0**(0, 1)', 2.0_q**(0.0_q, 1.0_q))
  w = (1.1_q, 0.7_q)
  n = 5
  write (*, '(1x, a, l2)') 'In line = called:', w**5 == w**n

  ! On arrays, and MERGE.
  zs = (/ (3.0_q, 4.0_q), (0.0_q, -2.0_q), (5.0_q, 12.0_q) /)
  write (*, '(1x, a, t20, 3f8.3)') 'ABS(ZS)', abs(zs)
  zs = sqrt(zs)
  do n = 1, 3
    call show('SQRT(ZS(N))', zs(n))
  end do
  call show('MERGE', merge(z, w, n > 2))
  call show('MERGE', merge(z, w, n < 2))
end program numfns

! Each part of a value, with 6 decimals.
subroutine show(what, x)
  implicit none
  character(len=*), intent(in) :: what
  complex(16), intent(in) :: x
  write (*, '(1x, a, t20, 2f12.6)') what, real(x), aimag(x)
end subroutine show
