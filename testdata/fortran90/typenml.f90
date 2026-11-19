! Objects of derived types in a NAMELIST group (docs/planning/PHASE-105 -
! Fortran 90 derived types.md, subtask 7). An object is an entity for each
! ultimate component, named by its designator: output writes P%X, T%AT%Y, and
! T%MARKS each on a line of its own, and input assigns a component by its
! designator (an array component by element, too) or a whole object, its
! values going to its components in order across them, nulls leaving one as it
! was. One value more than the object holds is TOOMANVAL, IOSTAT 18.
program typenml
  implicit none
  type point
    real :: x, y
  end type point
  type tag
    character(len=4) :: name
    type(point) :: at
    integer :: marks(3)
  end type tag
  type(point) :: p
  type(tag) :: t
  integer :: n, ios
  namelist /grp/ n, p, t

  n = 1
  p = point(1.5, -2.0)
  t = tag('ABCD', point(3.0, 4.0), (/ 1, 2, 3 /))
  write (*, grp)

  ! By component: P%X, T%AT%Y, an element of T%MARKS, and T%NAME.
  read (*, grp)
  write (*, '(1x, i2, 2f6.2, 1x, a, 2f6.2, 3i3)') n, p, t
  ! Whole objects: P's two values, T's six across its components.
  read (*, grp)
  write (*, '(1x, i2, 2f6.2, 1x, a, 2f6.2, 3i3)') n, p, t
  ! A component of a derived type whole, nulls, and a repeat count.
  read (*, grp)
  write (*, '(1x, i2, 2f6.2, 1x, a, 2f6.2, 3i3)') n, p, t
  ! One value too many.
  read (*, grp, iostat=ios)
  write (*, '(1x, i2, 2f6.2)') ios, p
  write (*, grp)
end program typenml
