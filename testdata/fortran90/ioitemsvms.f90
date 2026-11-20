! Sections as ENCODE's and DECODE's buffers (Phase 109, subtask 7), VAX
! FORTRAN's statements: a section whose elements aren't one after another is
! written through a copy, put back after ENCODE, and read through one by DECODE;
! and an assumed-shape array's section as an internal file.
program ioitemsvms
  implicit none
  character(len=8) :: rec(4)
  character(len=4) :: cells(6)
  integer :: i

  rec = '........'
  encode (12, 100, rec(1:4:3)) 1234, 'abcdefgh'
100 format (i4, a8)
  print '(1x, a, 4a)', 'encoded: ', rec
  decode (4, 101, rec(1:4:3)) i
101 format (i4)
  print '(1x, a, i5)', 'decoded: ', i

  cells = '----'
  call put(cells(6:1:-1))
  print '(1x, a, 6a)', 'cells:   ', cells

contains

  subroutine put(c)
    character(len=*), intent(inout) :: c(:)
    write (c(1:5:2), '(i4)') 1, 2, 3
  end subroutine put

end program ioitemsvms
