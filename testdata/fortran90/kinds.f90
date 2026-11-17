! kinds.f90: Fortran 90's kinds (docs/planning/PHASE-100 - Fortran 90
! declarations and kinds.md, subtask 4): kind selectors on every type, with
! KIND= and without, kinds named by PARAMETERs, CHARACTER's LEN= and KIND= in
! both orders, kinds on constants of every type, a REAL(8) given a default
! REAL's value and a REAL(8) constant's, and a function with a kind selector.
program kinds
  implicit none
  integer, parameter :: dp = 8, short = 2, qp = 16
  integer(short) :: small = 30000_short
  integer(kind=4) :: big
  real :: r4 = 1.1
  real(kind=dp) :: x, y = 1.1_dp
  real(qp) :: q = 2.5_qp
  complex(dp) :: z = (1.5_dp, -2.0_dp)
  complex(kind=4) :: c
  logical(2) :: flag = .true._2
  logical(kind=1) :: tiny
  character(len=5) :: word = 'kinds'
  character(3, kind=1) :: tag
  character(kind=1, len=*), parameter :: title = 1_'kinded'
  real(kind=dp), external :: area

  big = small
  big = big * 100
  x = r4
  write (*, '(1x, a, i6, i8)') title, small, big
  write (*, '(1x, 2f19.15)') x, y
  q = q * y
  write (*, '(1x, f8.4)') q
  c = z * 2
  write (*, '(1x, 2f6.2)') c
  tiny = .not. flag
  write (*, '(1x, 2l2)') flag, tiny
  tag = word(2:4)
  write (*, '(1x, a, 1x, a)') tag, word
  write (*, '(1x, f10.6)') area(2.0_dp)
end program kinds

! area is a circle's area, in REAL(8).
real(kind=8) function area(r)
  real(8), intent(in) :: r
  area = 3.14159265358979_8 * r * r
end function area
