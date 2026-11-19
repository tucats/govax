! Derived types (Phase 105, subtask 2): definitions, declarations of their
! objects, IMPLICIT TYPE(t), a typed FUNCTION, and % and . components.
program shapes
  implicit type(point) (p)
  type point
    sequence
    real :: x, y
  end type point
  type, private :: hidden
    integer i
  end type
  type :: segment
    type(point) :: ends(2)
    character(len=8) :: label
  end type segment
  type(point) :: a, b(3)
  type(segment), dimension(4) :: path
  real r
  a%x = 1.0
  b(2)%y = a%x + 2.0
  path(1)%ends(2)%x = b(2)%y
  path(1)%label(2:4) = 'abc'
  r = a.x + path(1).ends(1).y
  p%x = 3.0
end program shapes

block data inits
  type point
    sequence
    real x, y
  end type
  type(point) o, q(2)
  common /pts/ o, q
  data o / point(1.0 / 2, -3) /, q(1)%x, q(2) / 2*point(0, 0) /
end block data inits

type(point) function mid(a, b)
  type point
    real x, y
  end type
  type(point) a, b
  mid%x = (a%x + b%x) / 2
end function mid

recursive type(point) function twice(a)
  type point
    real x, y
  end type
  type(point) a
end
