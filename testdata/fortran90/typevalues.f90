! Values of arrays of derived types (docs/planning/PHASE-105 - Fortran 90
! derived types.md, subtask 5a): array constructors of a type, an array
! PARAMETER of a type and its components, arrays given initial values, a
! structure constructor whose array component's value is an array expression,
! and the intrinsic functions that take any type (MERGE, SPREAD, CSHIFT,
! EOSHIFT with a BOUNDARY, PACK, UNPACK, RESHAPE, TRANSPOSE), each element of
! the type moved whole.
program typevalues
  implicit none
  type point
    real :: x, y
  end type point
  type poly
    integer :: n
    real :: coord(3)
  end type poly
  type(point), parameter :: corners(3) = (/ point(0.0, 0.0), point(1.0, 0.0), point(1.0, 1.0) /)
  type(point), parameter :: origin = point(0.0, 0.0)
  type(point) :: path(4), q(4), grid(2, 3), flip(3, 2)
  type(point) :: start(2) = (/ point(7.0, 8.0), point(9.0, 10.0) /)
  type(point) :: zeros(2) = point(-1.0, -1.0)
  type(poly) :: t, polys(2)
  real :: v(3) = (/ 1.0, 2.0, 3.0 /)
  logical :: keep(4) = (/ .true., .false., .true., .false. /)
  integer :: i, j

  print 10, corners%x, corners(3)%y
  i = 2
  print 10, corners(i)%x, start%y, zeros%x

  path = (/ corners, point(5.0, 6.0) /)
  print 10, path%x
  path = (/ (point(real(i), real(10 * i)), i = 1, 4) /)
  print 10, path%y

  ! The type-generic functions.
  q = merge(path, origin, path%x > 2.0)
  print 10, q%y
  q = cshift(path, 1)
  print 10, q%x
  q = eoshift(path, -2, origin)
  print 10, q%x
  q = pack(path, keep, q)
  print 10, q%x
  q = unpack(start, keep, path)
  print 10, q%x
  grid = reshape(path, (/ 2, 3 /), (/ origin /))
  print 10, grid%x
  flip = transpose(grid)
  print 10, flip(:, 1)%x
  grid = spread(start, 2, 3)
  print 10, grid(2, :)%y

  ! A structure constructor's array component from arrays.
  t = poly(3, v * 2.0)
  print 20, t%n, t%coord
  t = poly(1, 4.5)
  print 20, t%n, t%coord
  polys = poly(2, t%coord(3:1:-1) + v)
  print 20, polys(2)%n, polys(2)%coord
  polys(1) = poly(9, polys(2)%coord(3:1:-1))
  print 30, (polys(j)%n, polys(j)%coord(1), j = 1, 2)

  ! Unformatted I/O of an array of a type, its bytes.
  open (1, status='scratch', form='unformatted')
  write (1) path
  rewind (1)
  q = origin
  read (1) q
  close (1)
  print 10, q%y
10 format (6f6.1)
20 format (i3, 3f6.1)
30 format (2(i3, f6.1))
end program typevalues
