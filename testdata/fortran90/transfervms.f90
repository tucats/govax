! transfervms.f90: TRANSFER on the VAX's own formats (docs/planning/PHASE-108
! - Fortran 90 intrinsic procedures.md, subtask 6): the bits of 1.0 in F, D,
! G (a unit compiled OPTIONS /G_FLOATING), and H, folded and when running;
! COMPLEX values made from INTEGERs; the tail past SOURCE, which the standard
! leaves undefined, zero here (the author's answer 5); and a reserved
! operand's bits, which a REAL can't hold as a number, passed through
! TRANSFER and back without a fault. gfortran's formats are IEEE's, so it
! isn't a witness.
program transfervms
  implicit none
  integer, parameter :: one = transfer(1.0, 0)
  integer, external :: gone
  integer :: i, k(8), w(4)
  real :: x, y(2)
  real(8) :: d
  real(16) :: h
  complex :: z
  complex(16) :: zh
  character(len=3) :: s3
  character(len=2) :: s2

  ! 1.0's bits in each format.
  x = 1.0
  d = 1.0d0
  h = 1.0_16
  y = (/ 1.0, 2.0 /)
  write (*, '(1x, a, 2i7)') 'F 1.0:', one, transfer(x, 0)
  write (*, '(1x, a, 2i7)') 'D 1.0:', transfer(d, (/ 0, 0 /))
  write (*, '(1x, a, 4i7)') 'H 1.0:', transfer(h, (/ 0, 0, 0, 0 /))
  write (*, '(1x, a, i7)') 'G 1.0:', gone()
  write (*, '(1x, a, 2i7)') 'Array:', transfer(y, (/ 0 /))

  ! What SOURCE doesn't fill is zero.
  i = 7
  w = transfer(i, (/ 0 /), 4)
  s2 = 'AB'
  s3 = transfer(s2, s3)
  write (*, '(1x, a, 4i3, 2i3, i4)') 'Tails:', w, transfer(transfer(i, d), (/ 0, 0 /)), ichar(s3(3:3))

  ! COMPLEX values from their parts' bits.
  k(1:2) = (/ 16512, 16640 /)
  z = transfer(k(1:2), z)
  k = (/ 16385, 0, 0, 0, 16386, 0, 0, 0 /)
  zh = transfer(k, zh)
  write (*, '(1x, a, 4f5.1)') 'Complex:', z, real(zh), aimag(zh)

  ! ^X8000 in F's first word is a reserved operand: moved, never computed.
  i = -32768
  x = transfer(i, x)
  write (*, '(1x, a, i8)') 'Reserved:', transfer(x, 0)
end program transfervms

options /g_floating
integer function gone()
  real(8) :: g

  g = 1.0d0
  gone = transfer(g, 0)
end function gone
