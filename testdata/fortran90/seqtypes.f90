! SEQUENCE types in COMMON, EQUIVALENCE, and DATA (docs/planning/PHASE-105 -
! Fortran 90 derived types.md, subtask 6). A common block holds objects of
! SEQUENCE types, each unit declaring the types again: two SEQUENCE
! definitions of one name and the same components are one type (4.4.2). A
! block data subprogram gives the block its values by structure constructors
! and by components; the main program's own objects get theirs by DATA, a
! constructor for a whole object, a value for a component; and an array of a
! numeric sequence type shares storage with an INTEGER array.
program seqtypes
  implicit none
  type pair
    sequence
    integer :: first, second
  end type pair
  type stamp
    sequence
    character(len=4) :: tag
    integer :: count
    type(pair) :: at
  end type stamp
  type(pair) :: p, q(3), origin, corners(2)
  type(stamp) :: s, label
  integer :: k(6), i
  common /shared/ p, s
  equivalence (q(2), k(3))
  data origin / pair(0, 0) /, corners / pair(1, 2), pair(3, 4) /
  data label%tag, label%count / 'LBL1', 7 /, label%at / pair(8, 9) /
  data (q(i)%first, q(i)%second, i = 1, 3) / 1, 10, 2, 20, 3, 30 /

  ! Whole objects in I/O lists are the next subtask's: components here.
  print 10, 'P', p%first, p%second
  print 20, 'S', s%tag, s%count, s%at%first, s%at%second
  call bump(3)
  print 10, 'P', p%first, p%second
  print 20, 'S', s%tag, s%count, s%at%first, s%at%second

  print 10, 'ORIGIN', origin%first, origin%second, (corners(i)%first, corners(i)%second, i = 1, 2)
  print 20, 'LABEL', label%tag, label%count, label%at%first, label%at%second

  ! K overlays Q whole: K(1) is Q(1)%FIRST, K(3) Q(2)%FIRST.
  print 10, 'K', k
  k(4) = 99
  q(3) = pair(-1, -2)
  print 10, 'Q', (q(i)%first, q(i)%second, i = 1, 3)
  print 10, 'K', k

10 format (1X, A6, 6I5)
20 format (1X, A6, 1X, A4, 3I5)
end program seqtypes

! The block's objects, by another unit's definitions of the same types.
subroutine bump(n)
  implicit none
  integer, intent(in) :: n
  type pair
    sequence
    integer :: first, second
  end type pair
  type stamp
    sequence
    character(len=4) :: tag
    integer :: count
    type(pair) :: at
  end type stamp
  type(pair) :: p
  type(stamp) :: s
  common /shared/ p, s

  p%first = p%first + n
  s%count = s%count * 10
  s%at = p
  s%tag(4:4) = '!'
end subroutine bump

block data init
  implicit none
  type pair
    sequence
    integer :: first, second
  end type pair
  type stamp
    sequence
    character(len=4) :: tag
    integer :: count
    type(pair) :: at
  end type stamp
  type(pair) :: p
  type(stamp) :: s
  common /shared/ p, s
  data p / pair(5, 6) /
  data s%tag, s%at / 'INIT', pair(7, 8) /, s%count / 2 /
end block data init
