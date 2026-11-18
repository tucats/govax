! Derived types: definitions, objects, and their components (docs/planning/
! PHASE-105 - Fortran 90 derived types.md, subtask 2). POINT and SEGMENT have
! no SEQUENCE, so they are naturally aligned; TAGGED and PAIR are SEQUENCE
! types, packed as a STRUCTURE is. Components are reached by % and by DEC's
! period, and an object of a type goes to a host's internal procedure and to an
! external subroutine with its own SEQUENCE definition, given by IMPLICIT.
program points
  implicit none
  type point
    real :: x, y
  end type point
  type tagged
    sequence
    character(len=1) :: tag
    integer :: count
  end type tagged
  type segment
    type(point) :: ends(2)
    character(len=8) :: label
    integer, dimension(3) :: marks
  end type segment
  type pair
    sequence
    real :: u, v
  end type pair
  type(point) :: a, b(3)
  type(segment) :: path(2)
  type(tagged) :: t
  type(pair) :: pr
  integer :: i

  a%x = 1.5
  a%y = -2.0
  do i = 1, 3
    b(i)%x = i
    b(i)%y = a%x * i
  end do

  path(2)%ends(1) = a
  path(2)%ends(2)%x = b(3)%y
  path(2)%ends(2)%y = path(2)%ends(1)%y + 10
  path(2)%label = 'diagonal'
  path(2)%label(1:1) = 'D'
  do i = 1, 3
    path(2)%marks(i) = i * i
  end do
  t%tag = 'Q'
  t%count = 7

  print *, a%x, a%y
  print *, (b(i)%x, b(i)%y, i = 1, 3)
  print *, path(2)%ends(1)%x, path(2)%ends(2)%x, path(2)%ends(2)%y
  print *, path(2)%label, ' ', path(2)%label(2:4)
  print *, path(2)%marks(3), ' ', t%tag, t%count
  print *, a.x + path(2).ends(2).y
  call show(a)

  pr%u = 3.0
  pr%v = 4.0
  call scale(pr, 2.0)
  print *, pr%u, pr%v
contains
  subroutine show(p)
    type(point) p
    print *, 'show', p%x + p%y
  end subroutine show
end program points

subroutine scale(q, f)
  implicit type(pair) (q)
  type pair
    sequence
    real :: u, v
  end type pair
  real f
  q%u = q%u * f
end subroutine scale
