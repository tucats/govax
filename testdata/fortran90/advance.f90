program advance
  ! Fortran 90's nonadvancing I/O (docs/planning/PHASE-109 - Fortran 90 IO.md,
  ! subtask 5): ADVANCE='NO' on output and input, the left tab limit, ADVANCE=
  ! given by a value, SIZE=, EOR=, IOSTAT='s -2 at an end of record, PAD=, the
  ! records a nonadvancing statement leaves open ended by other statements, and
  ! a prompt at the terminal.
  implicit none
  character(len=8) :: line
  character(len=5) :: s5
  character(len=2) :: s2
  character(len=10) :: adv, name
  integer :: i, j, k, n, ios

  ! Output: one record built by three statements.
  write (*, '(A)', advance='NO') ' abc'
  write (*, '(A)', advance='no') 'def'
  write (*, '(A)') 'ghi'

  ! The left tab limit: T and TL count from where the statement began.
  write (*, '(1X, A)', advance='NO') 'abc'
  write (*, '(T2, A)') 'X'
  write (*, '(1X, A)', advance='NO') 'abc'
  write (*, '(A, TL5, A)') 'de', 'Z'

  ! ADVANCE= by a value, in either case, trailing blanks not counting.
  adv = 'no'
  write (*, '(A)', advance=adv) ' x'
  adv = 'YES'
  write (*, '(A)', advance=adv) 'y'
  adv = 'maybe'
  write (*, '(A)', advance=adv, iostat=ios) ' z'
  print '(A, I4)', ' A bad ADVANCE=:', ios

  ! A list-directed statement ends the record first.
  write (*, '(A)', advance='NO') ' part'
  print *, 'list'

  ! Input from a file, a record taken up by three statements.
  open (10, file='ADVANCE.DAT', status='REPLACE')
  write (10, '(A)') '12345678'
  write (10, '(A)') 'ab'
  write (10, '(A)') 'xyz'
  write (10, '(A)') '123'
  close (10)
  open (10, file='ADVANCE.DAT', status='OLD')
  read (10, '(I3)', advance='NO') i
  read (10, '(I2)', advance='NO') j
  read (10, '(I5)') k
  print '(3I6)', i, j, k

  ! An end of record: the item padded, SIZE= the characters there, IOSTAT= -2.
  s5 = '?????'
  read (10, '(A5)', advance='NO', size=n, iostat=ios) s5
  print '(1X, 3A, 2I4)', '"', s5, '"', n, ios

  ! EOR=, after a statement that took part of the record.
  read (10, '(A2)', advance='NO', eor=50, size=n) s2
  print '(1X, A, I3)', s2, n
  read (10, '(A2)', advance='NO', eor=50, size=n) s2
  print *, 'not here'
50 print '(1X, 2A, I3)', 'EOR= taken: ', s2, n

  ! Fortran 90's padding to the field's width, which BZ makes zeros.
  read (10, '(BZ, I5)') k
  print '(I8)', k

  ! PAD='NO': a short record is an error, and a nonadvancing READ's end of
  ! record leaves the item as the record had it.
  close (10)
  open (10, file='ADVANCE.DAT', status='OLD', pad='NO')
  read (10, '(I10)', iostat=ios) k
  print '(A, I4)', ' A short record, PAD=''NO'':', ios
  read (10, '(A5)', advance='NO', size=n, iostat=ios) s5
  print '(2I4)', n, ios

  ! END= at the end of the file, after records read whole.
  read (10, '(A3)') line
  read (10, '(A3)') line
  read (10, '(A1)', advance='NO', end=60) s2
  print *, 'not here either'
60 print '(A)', ' END= taken'
  close (10, status='DELETE')

  ! REWIND ends a record left open, writing it.
  open (11, file='ADVANCE2.DAT', status='REPLACE')
  write (11, '(A)', advance='NO') 'one'
  write (11, '(A)', advance='NO') 'two'
  rewind (11)
  read (11, '(A)') line
  print '(1X, A)', trim(line)
  close (11, status='DELETE')

  ! A prompt: what was written shows before the terminal is read.
  write (*, '(A)', advance='NO') ' Your name? '
  read (*, '(A)') name
  print '(1X, 2A)', 'Hello, ', trim(name)

  ! The image's end writes a record left open.
  write (*, '(A)', advance='NO') ' The end.'
end program advance
