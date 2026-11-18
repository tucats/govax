! Sections to old dummies, VMS's side (Phase 104, subtask 6): an assumed-shape
! array as ENCODE's and DECODE's buffer, through a temporary when its elements
! aren't one after another and copied back after ENCODE; and LIB$STAT_VM's
! count of bytes, unchanged by twenty calls each copying a section of
! 30,000 elements through a temporary on the heap.
program seqvms
  implicit none
  interface
    subroutine code(b)
      integer :: b(:)
    end subroutine
  end interface
  integer :: buf(6), before, after, j, n
  character(len=4) :: parts(6)
  real :: big(2, 30000)
  real, external :: total

  encode (24, '(a24)', buf) '........................'
  call code(buf(1:6:2))
  decode (24, '(6a4)', buf) parts
  print '(1x, a, 6(1x, a))', 'parts:', parts
  call code(buf(1:2))
  decode (24, '(6a4)', buf) parts
  print '(1x, a, 6(1x, a))', 'parts:', parts

  big = 0.5
  n = 30000
  before = bytes()
  do j = 1, 20
    if (total(big(1, 1:n), n) /= 15000.0) print *, 'wrong'
  end do
  after = bytes()
  print '(1x, a, l2)', 'heap given back:', before == after
contains
  integer function bytes()
    integer :: lib$stat_vm, st
    st = lib$stat_vm(3, bytes)
  end function
end program seqvms

subroutine code(b)
  integer :: b(:)
  character(len=4) :: s
  decode (4, '(a4)', b) s
  print '(1x, a, 1x, a)', 'decoded:', s
  encode (8, '(a8)', b) 'abcdefgh'
end subroutine code

real function total(v, n)
  integer :: n
  real :: v(n)
  total = sum(v)
end function total
