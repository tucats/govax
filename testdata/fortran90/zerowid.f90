! Fortran 95's zero field widths and namelist comments (Phase 112,
! subtask 12): I0, B0, O0, Z0, and F0.d write a value in the fewest
! columns that hold it, and a ! in namelist input starts a comment that
! runs to the end of its record.
program zerowid
  implicit none
  integer :: n, i
  integer(kind=2) :: short
  real :: x
  double precision :: d
  character(len=4) :: name
  logical :: flag
  namelist /opts/ n, x, name, flag

  ! I0: the digits and the sign, no blanks. Iw.m's least digits still hold.
  print '(1X,A,I0,A)', '[', 42, ']'
  print '(1X,A,I0,A)', '[', -7, ']'
  print '(1X,A,I0,A)', '[', 0, ']'
  print '(1X,A,I0,A)', '[', -2147483647 - 1, ']'
  print '(1X,A,I0.4,A)', '[', 5, ']'
  print '(1X,A,I0.3,A)', '[', -5, ']'
  print '(1X,A,SP,I0,A)', '[', 5, ']'
  ! I0.0 of zero has no digits to show: one blank.
  print '(1X,A,I0.0,A)', '[', 0, ']'
  ! A list of fields, each its own width, side by side.
  print '(1X,A,3I0,A)', '[', 1, 22, 333, ']'
  print '(1X,A,3(I0,:,","),A)', '[', (10**i, i = 1, 3), ']'

  ! B0, O0, Z0: the item's bits as an unsigned number.
  print '(1X,A,B0,A)', '[', 10, ']'
  print '(1X,A,O0,A)', '[', 64, ']'
  print '(1X,A,Z0,A)', '[', 255, ']'
  print '(1X,A,Z0,A)', '[', -1, ']'
  short = -1
  print '(1X,A,Z0,A)', '[', short, ']'
  print '(1X,A,B0.8,A)', '[', 5, ']'
  print '(1X,A,O0,A)', '[', 0, ']'

  ! F0.d: the sign, the digits before the point, the point, d digits; a 0
  ! before the point only when it is the only digit.
  print '(1X,A,F0.2,A)', '[', 3.14159, ']'
  print '(1X,A,F0.3,A)', '[', -0.5, ']'
  print '(1X,A,F0.3,A)', '[', 0.0, ']'
  print '(1X,A,F0.0,A)', '[', 0.3, ']'
  print '(1X,A,F0.0,A)', '[', 2.7, ']'
  print '(1X,A,F0.1,A)', '[', 123456.0, ']'
  print '(1X,A,SP,F0.2,A)', '[', 1.5, ']'
  ! The scale factor moves the point, as for any F.
  print '(1X,A,1P,F0.2,A)', '[', 1.5, ']'
  print '(1X,A,-1P,F0.2,A)', '[', 1.5, ']'
  d = 1.0d0 / 3.0d0
  print '(1X,A,F0.10,A)', '[', d, ']'
  print '(1X,A,F0.2,A)', '[', -1234.5d0, ']'

  ! Zero widths in a sentence.
  n = 3
  x = 2.25
  print '(1X,"There are ",I0," parts at ",F0.2," each, ",F0.2," in all.")', &
        n, x, n * x

  ! Namelist input with comments: a line of its own before the group, after
  ! the group's name, after a value with or without a blank, and alone after
  ! the last; a ! inside a character value is the value's.
  n = 0
  x = 0.0
  name = ' '
  flag = .false.
  read (*, nml=opts)
  print '(1X,I0,1X,F0.2,1X,A,1X,L1)', n, x, name, flag
  read (*, nml=opts)
  print '(1X,I0,1X,F0.2,1X,A,1X,L1)', n, x, name, flag
end program zerowid
