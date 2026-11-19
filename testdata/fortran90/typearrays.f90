! Arrays of derived types and components with rank (docs/planning/PHASE-105 -
! Fortran 90 derived types.md, subtask 5a). PATH%X is the X of every element
! of PATH, an array of PATH's shape whose elements are a POINT's size apart;
! T%COORD(2:3) is a section of an array component; B%C%X takes its rank from
! the array component C. Each is scalarized as any array is, its element a
! scalar component reference. Arrays of a type are assigned, sectioned, and
! filled from a scalar of the type, an element at a time.
program typearrays
  implicit none
  type point
    real :: x, y
  end type point
  type poly
    integer :: n
    real :: coord(3)
  end type poly
  type box
    character(len=4) :: name
    type(point) :: c(2)
  end type box
  type(point) :: path(6), q(6), p
  type(poly) :: t
  type(box) :: b, bs(3)
  integer :: i
  integer :: iv(2) = (/ 5, 2 /)

  path%x = (/ (real(i), i = 1, 6) /)
  path%y = 2.0 * path%x
  print 10, (path(i)%x, path(i)%y, i = 1, 6)
  path(2:6:2)%y = 0.0
  print 10, path%y
  print 10, sum(path%x), maxval(path%y)

  ! The whole array, a section, a scalar of the type for every element.
  q = path
  q(1:3) = path(4:6)
  print 10, (q(i)%x, q(i)%y, i = 1, 6)
  p = point(-1.0, -2.0)
  q(5:6) = p
  q(iv) = point(9.0, 9.0)
  print 10, (q(i)%x, q(i)%y, i = 1, 6)

  ! Overlapping components: read before written, or apart.
  path(2:6)%x = path(1:5)%x
  print 10, path%x
  path%x = path%y
  print 10, path%x
  where (path%y > 5.0) path%y = -path%y
  print 10, path%y

  ! An array component's sections, and a vector subscript of records.
  t%n = 3
  t%coord = (/ 1.0, 2.0, 3.0 /)
  t%coord(2:3) = t%coord(1:2) + 10.0
  print 10, t%coord
  print 10, path((/ 1, 3 /))%y

  ! Rank from an array component of a component; CHARACTER components.
  b%name = 'BOXA'
  b%c%x = (/ 1.5, 2.5 /)
  b%c%y = b%c%x * 2.0
  print 10, (b%c(i)%x, b%c(i)%y, i = 1, 2)
  bs%name = (/ 'ONE ', 'TWO ', 'SIX ' /)
  bs(2:3)%name(2:3) = 'XX'
  print 20, bs%name, bs(1)%name(1:2)
  bs = b
  bs(3)%c(2)%y = 0.0
  print 20, bs%name
  print 10, bs%c(2)%y, bs(2)%c%x
10 format (6f6.1)
20 format (4(1x, a))
end program typearrays
