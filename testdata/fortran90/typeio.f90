! Objects of derived types in I/O lists (docs/planning/PHASE-105 - Fortran 90
! derived types.md, subtask 7). In a formatted, list-directed, or internal file
! statement an object is its ultimate components in order, a component of a
! derived type taken apart in turn and an array component every element of
! it; an array of a type is each element's components, element by element. In
! an unformatted statement an object is its bytes, written and read back whole,
! however long.
program typeio
  implicit none
  type point
    real :: x, y
  end type point
  type tag
    character(len=4) :: name
    type(point) :: at
    integer :: marks(3)
  end type tag
  type big
    integer :: first
    real :: values(20000)
    integer :: last
  end type big
  type(point) :: p, q, pts(3), back(3)
  type(tag) :: t, u, tags(2)
  type(big) :: b, c
  character(len=40) :: line
  integer :: i

  p = point(1.5, -2.0)
  write (*, '(2f6.2)') p
  pts = (/ (point(real(i), real(2*i)), i = 1, 3) /)
  write (*, '(6f5.1)') pts
  write (*, '(4f5.1)') pts(2:3)
  write (*, '(2f5.1)') (pts(i), i = 3, 1, -2)

  t = tag('ABCD', point(3.0, 4.0), (/ 1, 2, 3 /))
  write (*, '(1x, a, 2f5.1, 3i3)') t
  tags = (/ t, tag('WXYZ', p, (/ 7, 8, 9 /)) /)
  write (*, '(1x, a, 2f5.1, 3i3)') tags
  write (*, '(1x, a, 2f5.1, 3i3)') tags(2)%name, tags(2)%at, tags(2)%marks

  ! Values: a constructor's components, and a function's.
  write (*, '(2f5.1)') point(7.0, 8.0)
  write (*, '(2f6.2)') mid(p, pts(3))
  write (*, '(1x, a, 2f5.1, 3i3)') tag('MADE', mid(pts(1), pts(2)), 0)

  ! An internal file both ways.
  write (line, '(2f8.2)') p
  write (*, '(1x, a)') line(1:16)
  read (line, '(2f8.2)') q
  write (*, '(2f6.2)') q
  line = 'PQRS  9.50 -1.25  4  5  6'
  read (line, '(a4, 2f6.2, 3i3)') u
  write (*, '(1x, a, 2f6.2, 3i3)') u

  ! Unformatted: each object's bytes.
  b%first = 11
  b%last = 99
  b%values = (/ (real(i), i = 1, 20000) /)
  open (10, status='SCRATCH', form='UNFORMATTED')
  write (10) p, pts, t
  write (10) b
  rewind (10)
  read (10) q, back, u
  read (10) c
  close (10)
  write (*, '(2f6.2)') q
  write (*, '(6f5.1)') back
  write (*, '(1x, a, 2f5.1, 3i3)') u
  write (*, '(2i4, 2f9.1)') c%first, c%last, c%values(1), c%values(20000)

contains

  function mid(a, b)
    type(point), intent(in) :: a, b
    type(point) :: mid

    mid = point((a%x + b%x) / 2, (a%y + b%y) / 2)
  end function mid
end program typeio
