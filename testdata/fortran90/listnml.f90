program listnml
  ! Fortran 90's list-directed and namelist forms (docs/planning/PHASE-109 -
  ! Fortran 90 IO.md, subtask 6a): list-directed input's character values
  ! between quotation marks and undelimited; DELIM= on list-directed output,
  ! a delimited constant longer than a record read back whole; namelist output
  ! as &GROUP ... /, its text undelimited unless DELIM= says otherwise; and
  ! namelist input in the standard's forms, beside DEC's.
  implicit none
  character(len=12) :: a, b, c, d, e
  character(len=4) :: nm
  character(len=90) :: long, back
  character(len=100) :: line
  integer :: i, n, ios
  real :: x
  namelist /grp/ i, x, nm

  ! List-directed input from the terminal: a constant between apostrophes,
  ! one between quotation marks, undelimited text with apostrophes in it, a
  ! repeated value, and a slash that leaves I as it was.
  i = 0
  read (*, *) a, b, c, d, e, i
  print '(1X, 5(A, "|"), I3)', trim(a), trim(b), trim(c), trim(d), trim(e), i
  ! Undelimited text ended by a blank, and a quoted constant across records;
  ! digits are text to a CHARACTER item.
  read (*, *) a, b
  read (*, *) c, x
  print '(1X, 3(A, "|"), F5.2)', trim(a), trim(b), trim(c), x

  ! DELIM='APOSTROPHE': delimited text, read back by READ *; a constant longer
  ! than a record goes on in the next.
  open (10, file='LISTNML.DAT', status='REPLACE', delim='APOSTROPHE')
  write (10, *) 'it''s', 7, 'ab'
  long = repeat('ab''', 30)
  write (10, *) long
  close (10)
  open (10, file='LISTNML.DAT', status='OLD')
  read (10, '(A)') line
  print '(1X, A)', trim(line)
  n = 0
  do
    read (10, '(A)', iostat=ios) line
    if (ios /= 0) exit
    n = n + 1
  end do
  print '(1X, A, I2)', 'Records of the long constant:', n
  rewind (10)
  read (10, *) a, i, b
  read (10, *) back
  print '(1X, A, "|", I2, "|", A, L2)', trim(a), i, trim(b), back == long
  close (10, status='DELETE')

  ! DELIM='QUOTE', and DELIM= not given, NONE.
  open (11, file='LISTNML.DAT', status='REPLACE', delim='QUOTE')
  write (11, *) 'say "hi"', 'x'
  rewind (11)
  read (11, '(A)') line
  print '(1X, A)', trim(line)
  rewind (11)
  read (11, *) a, b
  print '(1X, 2(A, "|"))', trim(a), trim(b)
  close (11, status='DELETE')
  open (12, file='LISTNML.DAT', status='REPLACE')
  write (12, *) 'it''s', 'ab'
  rewind (12)
  read (12, '(A)') line
  print '(1X, A)', trim(line)
  close (12, status='DELETE')

  ! Namelist output: &GROUP ... /, the text undelimited.
  i = 42
  x = 1.5
  nm = 'it''s'
  write (*, grp)

  ! To a file with DELIM='APOSTROPHE', and read back.
  open (13, file='LISTNML.DAT', status='REPLACE', delim='APOSTROPHE')
  write (13, grp)
  rewind (13)
  do
    read (13, '(A)', iostat=ios) line
    if (ios /= 0) exit
    if (index(line, 'NM') > 0) print '(1X, A)', trim(line)
  end do
  rewind (13)
  i = 0
  x = 0
  nm = ' '
  read (13, grp)
  print '(1X, I3, F6.2, 1X, A)', i, x, trim(nm)
  close (13, status='DELETE')

  ! Namelist input from the terminal: another group passed over, the group
  ! in column 1, a quoted value with a slash in it, the slash that ends it;
  ! DEC's $GROUP ... $END; an apostrophe's constant and a number ended by a
  ! slash.
  read (*, grp)
  print '(1X, I3, F6.2, 1X, A)', i, x, trim(nm)
  read (*, grp)
  print '(1X, I3, F6.2, 1X, A)', i, x, trim(nm)
  read (*, grp)
  print '(1X, I3, F6.2, 1X, A)', i, x, trim(nm)
end program listnml
