! A character sequence type, a numeric sequence type, and a CHARACTER variable
! sharing storage (docs/planning/PHASE-105 - Fortran 90 derived types.md,
! subtask 6). The standard keeps numbers and characters apart in EQUIVALENCE;
! VAX FORTRAN lets them share, and govax does for sequence types too (the
! author's answer 6), so this program isn't gfortran's to witness. The VAX is
! little-endian: 'ABCD' is the longword 44434241 in hexadecimal.
program seqshare
  implicit none
  type name
    sequence
    character(len=4) :: first, last
  end type name
  type pair
    sequence
    integer :: low, high
  end type pair
  type(name) :: who
  type(pair) :: code
  character(len=8) :: text
  equivalence (who, code, text)
  data who%first(1:2), who%first(3:4), who%last / 'AB', 'CD', 'EFGH' /

  print 10, text, code%low, code%high
  code%low = 1684234849
  print 20, text, who%first, who%last

10 format (1X, A8, 2Z9)
20 format (1X, A8, 1X, A4, 1X, A4)
end program seqshare
