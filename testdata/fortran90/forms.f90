! forms.f90: declarations of every form Phase 100 compiles (docs/planning/
! PHASE-100 - Fortran 90 declarations and kinds.md, subtask 8), Fortran 90's
! beside VAX FORTRAN's in one program: type statements with and without ::,
! the attributes as attributes and as statements, DEC's *n lengths beside kind
! selectors and overriding them after a name, IMPLICIT with kinds, DEC's
! /clist/ initial values and DATA beside =, PARAMETER both ways, COMMON and
! EQUIVALENCE of names declared with ::, INTEGER*1, BYTE, DOUBLE COMPLEX, and
! COMPLEX(16). DEC's forms (VOLATILE, AUTOMATIC, STATIC, *n, /clist/) keep it
! from gfortran's -std=f95, so compare.py doesn't build it.
program forms
  implicit integer(kind=2) (i-k), real(8) (d)
  integer, parameter :: wide = selected_int_kind(9), many = 3
  integer(wide), dimension(many) :: tally = 0
  integer*2 :: short = 7
  integer(1) :: b1 = -5_1
  integer*1 :: b2 = 100
  byte letter /65/
  real(8) :: x*4 = 1.5, y = 2.25_8
  real*8 z /0.125d0/
  double complex :: dz = (1.0d0, -1.0d0)
  complex(16) :: q = (0.5_16, 2.0_16)
  complex*8 c
  logical(kind=2) :: ok = .true.
  logical*4 done
  character(len=4) :: code = 'ABCD', pair*2
  character*6 :: label
  character(len=*), parameter :: banner = 'FORMS'
  real :: plate(2, 2)
  real :: watched = 3.0
  volatile :: watched
  integer :: shared, alias
  common /blk/ shared, plate
  equivalence (alias, tally(2))
  intrinsic :: sqrt
  integer(2), external :: twice
  parameter (limit = 4)
  data label /'forms!'/
  dimension weights(limit)
  ! BUMP's OPTIONAL dummy needs its interface explicit where it is called
  ! (Phase 103, subtask 5: EXPLICITREQ).
  interface
     subroutine bump(counts, n, by)
       integer, intent(in) :: n
       integer(selected_int_kind(9)), intent(inout) :: counts(n)
       integer(2), intent(in), optional :: by
     end subroutine bump
  end interface

  write (*, '(1x, a, 2i3, i2)') banner, many, limit, kind(short)
  write (*, '(1x, a, 3i5, a2)') 'Bytes:', short, b1, b2, char(letter)
  write (*, '(1x, a, 3i3)') 'Kinds:', kind(x), kind(y), kind(z)
  write (*, '(1x, a, f6.3, f8.4, f7.3)') 'Reals:', x, y, z
  dval = sqrt(y) + z
  write (*, '(1x, a, f8.4, i3)') 'Implicit REAL(8):', dval, kind(dval)
  write (*, '(1x, a, 2f6.2, i3)') 'DOUBLE COMPLEX:', dz, kind(dz)
  q = q * q
  write (*, '(1x, a, 2f7.3, i3)') 'COMPLEX(16) squared:', real(q), aimag(q), kind(q)
  c = cmplx(watched, -watched)
  write (*, '(1x, a, 2f6.2)') 'COMPLEX*8:', c
  done = ok .and. .not. .false.
  write (*, '(1x, a, 2l2, i2)') 'Logicals:', ok, done, kind(ok)
  pair = code(3:)
  write (*, '(1x, a, 1x, a, 1x, a, 1x, a)') 'Text:', code, pair, label

  do i = 1, limit
     weights(i) = i * 0.5
  end do
  shared = 10
  do j = 1, 2
     do k = 1, 2
        plate(k, j) = j
     end do
  end do
  call fill
  write (*, '(1x, a, i4, 4f5.1)') 'COMMON:', shared, plate

  alias = 40
  do k = 1, 5
     call bump(tally, many, k)
  end do
  write (*, '(1x, a, 3i5)') 'Tally:', tally
  write (*, '(1x, a, i5, f6.1)') 'Twice:', twice(int(short, 2)), sum4(weights)
end program forms

! fill changes the common block declared with :: in its own words.
subroutine fill
  integer :: count
  real, dimension(2, 2) :: grid
  common /blk/ count, grid
  count = count + 1
  grid(2, 1) = 5.5
end subroutine fill

! bump adds by to one element of counts, chosen by how often it has been
! called; its counter is STATIC and its scratch AUTOMATIC.
subroutine bump(counts, n, by)
  integer, intent(in) :: n
  integer(selected_int_kind(9)), intent(inout) :: counts(n)
  integer(2), intent(in), optional :: by
  integer, static :: calls = 0
  integer, automatic :: slot
  real, target :: spare
  calls = calls + 1
  slot = mod(calls, n) + 1
  spare = by
  counts(slot) = counts(slot) + int(spare)
end subroutine bump

! twice doubles an INTEGER(2), its dummy's attributes given by statements.
integer(kind=2) function twice(m)
  integer*2 m
  intent(in) :: m
  twice = m + m
end function twice

! sum4 is the sum of four REAL values, by IMPLICIT's default type.
function sum4(v)
  dimension v(4)
  intent(in) v
  automatic total
  total = 0
  do i = 1, 4
     total = total + v(i)
  end do
  sum4 = total
end function sum4
