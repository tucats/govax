! transfer.f90: TRANSFER (docs/planning/PHASE-108 - Fortran 90 intrinsic
! procedures.md, subtask 6): SOURCE's bytes as MOLD's type, a scalar, or an
! array of SIZE elements or as many as hold SOURCE. Folded of constants (in
! PARAMETERs too), and computed when running of variables, sections (one with
! a stride, copied first), array expressions, records, passed-length
! CHARACTER dummies, an array past 65,535 bytes (OTS$MOVE5's), and a SIZE known
! only then. What this program prints doesn't depend on a floating format's bits,
! only on bytes being in little-endian order, so gfortran's is the same; the
! VAX's own bits are transfervms.f90's.
program transfers
  implicit none
  type pair
    sequence
    integer :: a, b
  end type pair

  integer, parameter :: abcd = transfer('ABCD', 0)
  character(len=4), parameter :: word = transfer(1684234849, 'abcd')
  integer, parameter :: halves(2) = transfer(196612, (/ 0_2 /))

  integer :: i, n, k(4), m(2)
  integer(2) :: h(2), q(4)
  character(len=8) :: s
  character(len=1) :: c(6)
  real :: x, y(3)
  real(8) :: d
  logical :: t
  type(pair) :: p
  integer :: big(20000)

  ! Folded when compiling.
  write (*, '(1x, a, i12, 1x, a, 2i4)') 'Folded:', abcd, word, halves

  ! A CHARACTER variable as INTEGERs, and back.
  s = 'GOVAXF90'
  k = 0
  k(1:2) = transfer(s, k(1:2))
  write (*, '(1x, a, 2i12)') 'Words:', k(1:2)
  write (*, '(1x, a, a)') 'Back:  ', transfer(k(1:2), s)

  ! Its first six characters as an array of CHARACTER*1.
  c = transfer(s(1:6), c)
  write (*, '(1x, a, 6(1x, a))') 'Chars:', c

  ! An INTEGER's halves, and SIZE known only when running.
  i = 196612
  h = transfer(i, h)
  n = 2
  m = transfer(s, 0, n)
  write (*, '(1x, a, 2i6, 2i12)') 'Halves:', h, m
  write (*, '(1x, a, 2i4)') 'Sizes:', size(transfer(s, (/ 0_2 /))), size(transfer(s, 'x', n))

  ! A section with a stride, and an array expression.
  k = (/ 65, 66, 67, 68 /)
  q = transfer(k(1:4:2), q)
  i = transfer((/ 7, 8 /) + n, 0)
  write (*, '(1x, a, 4i4, i6)') 'Section:', q, i

  ! REAL values through INTEGERs and back are themselves.
  x = 2.5
  y = (/ 1.5, -0.25, 1000.0 /)
  d = transfer(transfer(-7.0d0, (/ 0, 0 /)), d)
  write (*, '(1x, a, l2, 3f9.2, f6.1)') 'Reals:', transfer(transfer(x, i), x) == x, &
    transfer(transfer(y, k(1:3)), y), d

  ! A record, and a LOGICAL.
  p = transfer((/ 11, 22 /), p)
  k(1:2) = transfer(p, k(1:2))
  t = transfer(1, .false.)
  write (*, '(1x, a, 4i4, l2)') 'Record:', p%a, p%b, k(1:2), t

  ! 80,000 bytes as 40,000 INTEGER(2)s.
  big = 0
  big(1) = 5
  big(20000) = 9
  write (*, '(1x, a, 2i7)') 'Big:', size(transfer(big, (/ 0_2 /))), sum(transfer(big, (/ 0_2 /)))

  call lengths('WXYZ', (/ 'AB', 'CD' /))

contains

  ! Lengths known only when the program runs.
  subroutine lengths(word, pairs)
    character(len=*), intent(in) :: word, pairs(:)
    integer(2) :: two(2)

    write (*, '(1x, a, i12, 2i7)') 'Passed:', transfer(word, 0), transfer(pairs, two)
  end subroutine lengths
end program transfers
