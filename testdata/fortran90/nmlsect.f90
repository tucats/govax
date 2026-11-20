! Sections and components in NAMELIST input's names, and arrays of derived
! types in a group (docs/planning/PHASE-109 - Fortran 90 IO.md, subtask 6b).
! A section, with or without a stride, takes its elements in array element
! order; an array of a derived type is an entity per component, every element's
! (PTS%X), which output writes on a line of its own and input takes by
! element (PTS(2)%X), by section, whole, or the object whole, element by
! element and each element's components in order. An element named with every
! subscript fixed takes its own and the rest of its array's, VMS's rule.
! What can't be is INVREFVAR (19), and too many values TOOMANVAL (18).
program nmlsect
  implicit none
  type point
    real :: x, y
  end type point
  type line
    character(len=3) :: tag
    type(point) :: ends(2)
  end type line
  type vec
    integer :: v(3)
  end type vec
  integer :: k(6), m(2,3), i, ios
  real :: x(5)
  character(len=4) :: c(3)
  type(point) :: pts(3)
  type(line) :: l, ls(2)
  type(vec) :: vs(2)
  namelist /arr/ k, m, x, c
  namelist /obj/ pts, l
  namelist /deep/ ls, vs

  k = 0
  m = 0
  x = 0.0
  c = 'abcd'
  pts = point(0.0, 0.0)
  l = line('ABC', (/ point(1.0, 2.0), point(3.0, 4.0) /))
  ls = l
  vs = vec((/ 1, 2, 3 /))

  ! Sections: a stride, a negative one, every element of a dimension, and
  ! a section of an array of character values with a substring.
  read (*, arr)
  write (*, '(1x, 6i3, 1x, 6i3)') k, m
  write (*, '(1x, 5f5.1, 3(1x, a))') x, c

  ! An array of a derived type: by element, by component, by section,
  ! an element whole, and then the array whole.
  write (*, obj)
  read (*, obj)
  write (*, '(1x, 6f5.1)') pts
  read (*, obj)
  write (*, '(1x, 6f5.1)') pts
  ! An object with an array of a derived type in it, whole and by part.
  read (*, obj)
  write (*, '(1x, a, 4f5.1)') l%tag, l%ends

  ! Two levels of arrays: one name, and the objects whole.
  read (*, deep)
  write (*, '(2(1x, a, 4f5.1))') ls
  write (*, '(1x, 6i3)') vs
  write (*, deep)

  ! What can't be: out of bounds, a stride of 0, too few subscripts, a
  ! component of what has none; an empty section, which takes no value; and
  ! a section given one value too many, the others staying stored.
  do i = 1, 6
    read (*, arr, iostat=ios)
    write (*, '(1x, i3)') ios
  end do
  write (*, '(1x, 6i3)') k

  call framed
end program nmlsect

! A recursive routine's array of a derived type is in its frame: the group's
! block, and the class NCA descriptors in it, are built each time it is used.
recursive subroutine framed
  implicit none
  type point
    real :: x, y
  end type point
  type(point) :: q(2)
  namelist /fr/ q

  q = point(1.0, 2.0)
  read (*, fr)
  write (*, fr)
end subroutine framed
