! Fortran 90's OPEN and INQUIRE (docs/planning/PHASE-109 - Fortran 90 IO.md,
! subtask 4): ACTION=, POSITION=, DELIM=, PAD=, and STATUS='REPLACE' in OPEN,
! by constants and by run-time values; their answers in INQUIRE, with READ=,
! WRITE=, and READWRITE=; a re-OPEN changing DELIM= and PAD=; a READ of a unit
! opened ACTION='WRITE' and a WRITE of one opened ACTION='READ'; and
! INQUIRE (IOLENGTH=) of scalars, arrays, sections, an implied DO, an
! allocatable array, a passed-length dummy, and a derived type's object, the
! length then used as a direct access file's RECL=.
program openings
  implicit none
  type pair
    sequence
    integer :: key
    real :: value
  end type pair
  character(len=10) :: act, pos, dlm, pd, rd, wr, rw
  character(len=20) :: line
  character(len=8) :: c(20), how
  integer :: ios, iol, k, j
  logical :: there
  real :: x
  double precision :: d(10)
  integer, allocatable :: m(:)
  type(pair) :: p(3)

  ! A file made by REPLACE, whether or not one is there, written only.
  open (10, file='OPENINGS.DAT', status='REPLACE', action='WRITE')
  write (10, '(A)') 'old line'
  close (10)
  open (10, file='OPENINGS.DAT', status='REPLACE', action='WRITE', delim='QUOTE', pad='NO')
  write (10, '(A)') 'first'
  call show(10)
  read (10, '(A)', iostat=ios) line
  print '(A, L2)', ' A READ of a WRITE unit fails:', ios > 0
  close (10)

  ! Appended to; POSITION= and ACTION= given by variables.
  how = 'append'
  open (10, file='OPENINGS.DAT', status='OLD', position=how, action='readwrite ')
  call show(10)
  write (10, '(A)') 'second'
  close (10)

  ! Read only, rewound, and deleted when closed (ACTION='READ' may be).
  open (10, file='OPENINGS.DAT', status='OLD', action='READ')
  call show(10)
  do k = 1, 3
    read (10, '(A)', iostat=ios) line
    if (ios /= 0) exit
    print '(2A)', ' Read: ', trim(line)
  end do
  write (10, '(A)', iostat=ios) 'third'
  print '(A, L2)', ' A WRITE of a READ unit fails:', ios > 0
  rewind (10)
  inquire (10, position=pos)
  print '(2A)', ' After REWIND: ', trim(pos)
  read (10, '(A)') line
  print '(2A)', ' Read again: ', trim(line)
  close (10, status='DELETE')
  inquire (file='OPENINGS.DAT', exist=there)
  print '(A, L2)', ' Still there:', there

  ! A re-OPEN of the same file changes DELIM= and PAD=.
  open (11, file='OPENINGS.TMP', status='REPLACE', delim='APOSTROPHE')
  call show(11)
  open (11, file='OPENINGS.TMP', delim='NONE', pad='NO')
  call show(11)
  close (11, status='DELETE')

  ! Not connected: nothing to tell.
  call show(12)

  ! IOLENGTH=, in longwords: 4 + 80 + 160 bytes, a section of 5 elements,
  ! an implied DO of 3, an allocatable array of 7, a derived type's 3 objects.
  inquire (iolength=iol) x, d, c
  print '(A, I5)', ' Scalar and arrays:', iol
  k = 5
  inquire (iolength=iol) c(2:k+1)
  print '(A, I5)', ' Section:', iol
  inquire (iolength=iol) (d(j), j = 1, 3), x
  print '(A, I5)', ' Implied DO:', iol
  allocate (m(7))
  inquire (iolength=iol) m, 'abc'
  print '(A, I5)', ' Allocatable:', iol
  inquire (iolength=iol) p
  print '(A, I5)', ' Derived type:', iol
  call text('abcdefghi')

  ! The length as a direct access file's RECL=.
  inquire (iolength=iol) k, x, line
  open (13, file='OPENINGS.DIR', status='REPLACE', access='DIRECT', form='UNFORMATTED', recl=iol)
  line = 'record'
  do k = 1, 3
    write (13, rec=k) k, 1.5 * k, line
  end do
  read (13, rec=2) j, x, line
  print '(A, I3, F6.2, 1X, A)', ' Record 2:', j, x, trim(line)
  close (13, status='DELETE')

contains

  ! What INQUIRE tells of a unit's Fortran 90 attributes.
  subroutine show(u)
    integer, intent(in) :: u
    inquire (u, action=act, position=pos, delim=dlm, pad=pd, read=rd, write=wr, readwrite=rw)
    print '(A, I3, 7(1X, A))', ' Unit', u, trim(act), trim(pos), trim(dlm), trim(pd), &
      trim(rd), trim(wr), trim(rw)
  end subroutine show

  ! A passed-length dummy's IOLENGTH=: 9 characters, three longwords.
  subroutine text(s)
    character(len=*), intent(in) :: s
    integer :: n
    inquire (iolength=n) s
    print '(A, I5)', ' Passed length:', n
  end subroutine text

end program openings
