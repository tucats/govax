program journal
  ! A household journal kept in an array of a derived type, through Fortran 90's
  ! I/O (docs/planning/PHASE-109 - Fortran 90 IO.md, subtask 8): the array
  ! written to a file by one statement whose format reverts for each element,
  ! with B, ES, L, and G of a CHARACTER item; each record read back whole by a
  ! nonadvancing READ, its length by SIZE= at its end (EOR=), so the records are
  ! shown byte for byte; the file read into a second array by the same format
  ! and compared; the objects in a direct access file whose RECL= is INQUIRE's
  ! IOLENGTH=, read in another order; NAMELIST output with DELIM= read back, then
  ! a group of changes named by sections and components of the array; a report
  ! whose lines are built by nonadvancing statements; and components of every
  ! element, and of every other one, as items of output lists.
  implicit none
  type entry
    character(len=10) :: date
    integer :: code
    real(8) :: amount
    logical :: cleared
    character(len=12) :: memo
  end type entry
  character(len=*), parameter :: form = '(A10, 1X, B8.8, 1X, ES11.3E2, 1X, L1, 1X, G12.1)'
  type(entry) :: book(5), back(5)
  character(len=60) :: rec
  integer :: i, n, reclen
  real(8) :: total
  namelist /ledger/ book, total

  book(1) = entry('1990-01-05', 5, 1250.0d0, .true., 'Rent')
  book(2) = entry('1990-01-09', 12, -37.5d0, .true., 'Groceries')
  book(3) = entry('1990-01-15', 9, 0.00125d0, .false., 'Interest')
  book(4) = entry('1990-01-20', 3, -1000000.0d0, .false., 'House')
  book(5) = entry('1990-01-31', 255, 12.0d0, .true., "Pat's share")

  ! The whole array by one statement: the format reverts for each element, and
  ! each element's components are its items.
  open (10, file='JOURNAL.TXT', status='REPLACE', action='WRITE')
  write (10, form) book
  close (10)

  ! Each record whole, its length from SIZE= at its end.
  open (10, file='JOURNAL.TXT', status='OLD', action='READ')
  do i = 1, 5
    rec = ' '
    read (10, '(A)', advance='NO', size=n, eor=10) rec
10  print '(1X, I3, " |", A, "|")', n, rec(1:n)
  end do

  ! The file read into another array by the same format.
  rewind (10)
  read (10, form) back
  close (10, status='DELETE')
  print '(1X, A, 4L2)', 'Same:', all(back%date == book%date), all(back%code == book%code), &
      all(back%cleared .eqv. book%cleared), all(back%memo == book%memo)
  print '(1X, A, F14.5)', 'Total read:', sum(back%amount)

  ! Direct access: RECL= from IOLENGTH=, the objects written last to first.
  inquire (iolength=reclen) book(1)
  open (11, file='JOURNAL.DAT', status='REPLACE', access='DIRECT', form='UNFORMATTED', &
      recl=reclen)
  do i = 1, 5
    write (11, rec=6-i) book(i)
  end do
  read (11, rec=2) back(1)
  read (11, rec=5) back(2)
  close (11, status='DELETE')
  print '(1X, A10, I4, F13.2, L2, 1X, A12, "|")', back(1:2)

  ! NAMELIST output with DELIM=, read back; then a group of changes named by
  ! sections and components of the array.
  open (12, file='JOURNAL.NML', status='REPLACE', delim='APOSTROPHE')
  total = sum(book%amount)
  write (12, nml=ledger)
  write (12, '(A)') ' &LEDGER BOOK(2:3)%CLEARED = T, T, BOOK(4)%AMOUNT = -950000.0,'
  write (12, '(A)') '  BOOK(5)%MEMO = ''Pat''''s part'', TOTAL = 0.0 /'
  rewind (12)
  back = book
  book = entry(' ', 0, 0.0d0, .false., ' ')
  total = -1.0d0
  read (12, nml=ledger)
  print '(1X, A, L2, F14.5)', 'First group:', all(book%memo == back%memo), total
  read (12, nml=ledger)
  close (12, status='DELETE')

  ! A report, each line built by nonadvancing statements.
  total = 0
  do i = 1, 5
    write (*, '(1X, A10)', advance='NO') book(i)%date
    if (book(i)%cleared) then
      write (*, '(A)', advance='NO') ' *'
    else
      write (*, '(A)', advance='NO') '  '
    end if
    write (*, '(EN15.3, 1X, A)') book(i)%amount, trim(book(i)%memo)
    total = total + book(i)%amount
  end do
  print '(1X, A, ES15.6)', 'Total:     ', total

  ! Components of every element, and of every other one, as list items.
  print '(1X, B8.8, 4(1X, B8.8))', book%code
  print '(1X, 3F12.2)', book(1:5:2)%amount
  print '(1X, L1, 4L2)', book%cleared
end program journal
