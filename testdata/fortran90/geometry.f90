! Geometry with a derived type and defined operators (docs/planning/PHASE-105 -
! Fortran 90 derived types.md, subtask 8). VEC is a SEQUENCE type, so that the
! interface bodies, which have no host, define the same type again; the
! operators +, -, and * (a REAL times a VEC) extend the intrinsic ones to VEC,
! where Fortran gives them no meaning, and .DOT., .CROSS., and the unary .ROT.
! are new. A triangle's area and centroid, a polygon's perimeter and area by
! the shoelace formula, an array of VECs by component, and a function's VEC
! value, which comes back in R0 and R1.
program geometry
  implicit none
  type vec
    sequence
    real :: x, y
  end type vec
  interface operator(+)
    function vadd(a, b)
      type vec
        sequence
        real :: x, y
      end type vec
      type(vec), intent(in) :: a, b
      type(vec) :: vadd
    end function vadd
  end interface
  interface operator(-)
    function vsub(a, b)
      type vec
        sequence
        real :: x, y
      end type vec
      type(vec), intent(in) :: a, b
      type(vec) :: vsub
    end function vsub
  end interface
  interface operator(*)
    function vscale(s, a)
      type vec
        sequence
        real :: x, y
      end type vec
      real, intent(in) :: s
      type(vec), intent(in) :: a
      type(vec) :: vscale
    end function vscale
  end interface
  interface operator(.dot.)
    real function vdot(a, b)
      type vec
        sequence
        real :: x, y
      end type vec
      type(vec), intent(in) :: a, b
    end function vdot
  end interface
  interface operator(.cross.)
    real function vcross(a, b)
      type vec
        sequence
        real :: x, y
      end type vec
      type(vec), intent(in) :: a, b
    end function vcross
  end interface
  interface operator(.rot.)
    function vrot(a)
      type vec
        sequence
        real :: x, y
      end type vec
      type(vec), intent(in) :: a
      type(vec) :: vrot
    end function vrot
  end interface
  type(vec), parameter :: origin = vec(0.0, 0.0)
  type(vec) :: tri(3), poly(5), c, d
  real :: area, perim
  integer :: i

  ! A 3-4-5 right triangle.
  tri = (/ origin, vec(4.0, 0.0), vec(0.0, 3.0) /)
  area = 0.5 * ((tri(2) - tri(1)) .cross. (tri(3) - tri(1)))
  c = (1.0 / 3.0) * (tri(1) + tri(2) + tri(3))
  perim = 0
  do i = 1, 3
    d = tri(mod(i, 3) + 1) - tri(i)
    perim = perim + sqrt(d .dot. d)
  end do
  write (*, '(1x, a, f8.3)') 'area     ', area
  write (*, '(1x, a, 2f8.3)') 'centroid ', c
  write (*, '(1x, a, f8.3)') 'perimeter', perim

  ! A square with a notch cut from its top: area 4 less 1.
  poly = (/ origin, vec(2.0, 0.0), vec(2.0, 2.0), vec(1.0, 1.0), vec(0.0, 2.0) /)
  area = 0
  do i = 1, 5
    area = area + (poly(i) .cross. poly(mod(i, 5) + 1))
  end do
  write (*, '(1x, a, f8.3)') 'polygon  ', area / 2
  write (*, '(1x, a, 4f6.1)') 'bounds   ', minval(poly%x), minval(poly%y), maxval(poly%x), maxval(poly%y)

  ! Moved by components, then each turned a quarter turn about the origin.
  poly%x = poly%x + 1.0
  poly%y = 2.0 * poly%y - 1.0
  write (*, '(1x, 10f5.1)') poly
  do i = 1, 5
    poly(i) = .rot. poly(i)
  end do
  write (*, '(1x, 10f5.1)') poly
  write (*, '(1x, 2f6.1, f8.3)') .rot. (.rot. vec(1.0, 2.0)), origin .dot. c
end program geometry

function vadd(a, b)
  type vec
    sequence
    real :: x, y
  end type vec
  type(vec), intent(in) :: a, b
  type(vec) :: vadd

  vadd = vec(a%x + b%x, a%y + b%y)
end function vadd

function vsub(a, b)
  type vec
    sequence
    real :: x, y
  end type vec
  type(vec), intent(in) :: a, b
  type(vec) :: vsub

  vsub = vec(a%x - b%x, a%y - b%y)
end function vsub

function vscale(s, a)
  type vec
    sequence
    real :: x, y
  end type vec
  real, intent(in) :: s
  type(vec), intent(in) :: a
  type(vec) :: vscale

  vscale = vec(s * a%x, s * a%y)
end function vscale

real function vdot(a, b)
  type vec
    sequence
    real :: x, y
  end type vec
  type(vec), intent(in) :: a, b

  vdot = a%x * b%x + a%y * b%y
end function vdot

real function vcross(a, b)
  type vec
    sequence
    real :: x, y
  end type vec
  type(vec), intent(in) :: a, b

  vcross = a%x * b%y - a%y * b%x
end function vcross

function vrot(a)
  type vec
    sequence
    real :: x, y
  end type vec
  type(vec), intent(in) :: a
  type(vec) :: vrot

  vrot = vec(-a%y, a%x)
end function vrot
