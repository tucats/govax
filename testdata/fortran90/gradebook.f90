program gradebook
  ! Students' scores in an array of a derived type with an array component,
  ! through Fortran 90's I/O (docs/planning/PHASE-109 - Fortran 90 IO.md,
  ! subtask 8): a file made by STATUS='REPLACE' and ACTION='WRITE', each name
  ! put flush left by A and T; opened again with POSITION='APPEND' and asked by
  ! INQUIRE; read with ACTION='READ', a short record padded (PAD= by default
  ! YES, its blanks nothing); whole objects read and written, each a name and
  ! four scores; a
  ! test's column as one ranked component across the students, and every other
  ! student's, in formatted and unformatted lists; and names written to an
  ! internal file that is a section with a stride.
  implicit none
  type student
    character(len=10) :: name
    integer :: scores(4)
  end type student
  type(student) :: class(6)
  integer :: n, i, j, ios, column(6), totals(6), at(1)
  character(len=8) :: action, position
  character(len=6) :: cells(6)

  open (20, file='GRADES.DAT', status='REPLACE', action='WRITE')
  write (20, '(A, T11, 4I4)') 'Ada', 91, 85, 77, 98
  write (20, '(A, T11, 4I4)') 'Brook', 68, 74, 80, 71
  write (20, '(A, T11, 3I4)') 'Chen', 88, 92, 79
  close (20)

  ! Two more students at the file's end.
  open (20, file='GRADES.DAT', status='OLD', action='WRITE', position='APPEND')
  inquire (20, action=action, position=position)
  print '(1X, 3A)', trim(action), ' ', trim(position)
  write (20, '(A, T11, 4I4)') 'Dana', 100, 95, 90, 85
  write (20, '(A, T11, 4I4)') 'Eli', 55, 61, 70, 66
  close (20)

  ! Whole objects read until the file's end; Chen's record is short, and its
  ! missing score is padded with blanks.
  open (20, file='GRADES.DAT', status='OLD', action='READ')
  n = 0
  do
    read (20, '(A10, 4I4)', iostat=ios) class(n+1)
    if (ios /= 0) exit
    n = n + 1
  end do
  close (20, status='DELETE')
  print '(1X, A10, 4I4)', class(1:n)

  ! A test's column: one component of every student.
  do j = 1, 4
    print '(1X, A, I1, A, 6I4)', 'Test ', j, ':', class(1:n)%scores(j)
  end do
  print '(1X, A, 4F7.2)', 'Means: ', (sum(class(1:n)%scores(j)) / real(n), j = 1, 4)
  do i = 1, n
    totals(i) = sum(class(i)%scores)
  end do
  at = maxloc(totals(1:n))
  print '(1X, A, 6I5)', 'Totals:', totals(1:n)
  print '(1X, 2A)', 'Best: ', trim(class(at(1))%name)
  print '(1X, A, 3I4)', 'Last test, every other student:', class(1:n:2)%scores(4)

  ! A column through an unformatted record and back.
  open (21, status='SCRATCH', form='UNFORMATTED')
  write (21) class(1:n)%scores(3)
  rewind (21)
  column = -1
  read (21) column(1:n)
  close (21)
  print '(1X, A, 6I4)', 'Read back:', column

  ! Every other student's name, written to every other record.
  cells = '------'
  write (cells(1:n:2), '(A6)') class(1:n:2)%name
  print '(1X, 6("|", A), "|")', cells
end program gradebook
