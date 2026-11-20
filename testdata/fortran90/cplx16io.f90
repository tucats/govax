! cplx16io.f90: I/O of Fortran 90's COMPLEX(16) (docs/planning/PHASE-109 -
! Fortran 90 IO.md, subtask 3), two H_floating parts, in every kind of
! statement: formatted output and input, where a complex item is two reals,
! each taking an edit descriptor of its own (F, E, ES, EN, and G); list-
! directed output and input; NAMELIST output and input; and unformatted
! records, its 32 bytes. An array of the type goes element by element.
program cplx16io
  implicit none
  integer, parameter :: q = 16
  complex(q) :: z, w, a(3), b(3)
  complex(q) :: tiny
  character(len=100) :: line
  integer :: i, ios
  namelist /pair/ z, w

  z = (1.5_q, -2.0_q)
  w = (0.25_q, 1000.0_q)
  a = (/ (cmplx(i, -i, kind=q), i = 1, 3) /)

  ! Formatted output: a part per edit descriptor.
  write (*, '(1x, 2f10.4)') z
  write (*, '(1x, f8.2, es14.4)') w
  write (*, '(1x, 2e16.6e3)') z
  write (*, '(1x, 2en14.3)') w
  write (*, '(1x, 2g12.4)') w
  write (*, '(1x, 6f6.1)') a
  write (*, '(1x, 2f37.33)') (1.0_q, -2.0_q) / (3.0_q, 0.0_q)

  ! Formatted input from an internal file, and the values written back.
  line = '  3.25  -0.125  1.0E10  2.5'
  read (line, '(2f8.0, 2e8.0)') z, w
  write (*, '(1x, 4f18.4)') z, w

  ! List-directed output: one value to a record, each part's G field
  ! without its blanks; a small one's E form.
  z = (1.5_q, -2.0_q)
  tiny = (1.0e-100_q, -1.0e100_q)
  print *, z
  print *, tiny

  ! List-directed input: complex constants, with blanks and without.
  line = '(2.5, -0.5), ( 7 , 0 ), (1.0E-3,4)'
  read (line, *) z, w, tiny
  write (*, '(1x, 6f10.4)') z, w, tiny

  ! List-directed output to an internal file, the value whole in one record.
  line = ' '
  write (line, *) (1.0e-100_q, -1.0e100_q)
  write (*, '(1x, a)') trim(line)

  ! NAMELIST output and input, through a file, in Fortran 90's form (&PAIR
  ! ... /, since Phase 109's subtask 6); each value too long for the record
  ! its name is on goes on the next.
  z = (1.5_q, -2.0_q)
  w = (0.25_q, 1000.0_q)
  open (1, file='CPLX16IO.DAT', status='new')
  write (1, pair)
  rewind (1)
  do
    read (1, '(a)', iostat=ios) line
    if (ios /= 0) exit
    write (*, '(1x, a)') trim(line)
  end do
  z = 0
  w = 0
  rewind (1)
  read (1, pair)
  write (*, '(1x, 4f10.4)') z, w
  close (1, status='delete')

  ! Unformatted records: the bytes, and the values read back.
  open (2, file='CPLX16IO.BIN', form='unformatted', status='new')
  write (2) a, (0.0_q, 1.0_q)
  rewind (2)
  read (2) b, z
  close (2, status='delete')
  write (*, '(1x, 6f6.1)') b
  write (*, '(1x, 2f6.1)') z
  write (*, '(1x, l2)') all(a == b)
end program cplx16io
