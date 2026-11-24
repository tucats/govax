! Phase 113, subtask 2: Fortran 90 forms that were NOTIMPL, or refused,
! before: an array BACK= to SCAN, VERIFY, and INDEX; SUM, PRODUCT,
! DOT_PRODUCT, and MATMUL of COMPLEX(16) values; a statement function of a
! derived type; an array bound from an element's component, D(1)%N; and
! TRANSFER to a derived type, folded as a PARAMETER and computed.
module shapes
  implicit none
  type point
    real :: x, y
  end type
  type t
    integer :: n
  end type
  type pair
    integer(2) :: lo, hi
    character(len=2) :: tag
    integer(1) :: b(2)
  end type
end module shapes

program errata
  use shapes
  implicit none
  type(t) :: dd(3)

  call backs
  call complex16
  call stmtfuncs
  dd%n = (/ 2, 5, 3 /)
  call bounds(dd, 2)
  call transfers

contains

  ! BACK= an array, its elements taken with STRING's or SET's.
  subroutine backs
    character(len=6) :: w(3)
    logical :: b(3)
    integer :: k(3)

    w = (/ 'abcabc', 'xxaxxa', 'banana' /)
    b = (/ .true., .false., .true. /)
    k = scan(w, 'a', b)
    write (*, '(1x, a, 3i3)') 'scan:  ', k
    write (*, '(1x, a, 3i3)') 'scan1: ', scan('abcabc', 'bc', b)
    write (*, '(1x, a, 3i3)') 'verify:', verify(w, 'ab', back=b)
    write (*, '(1x, a, 3i3)') 'index: ', index(w, 'a', b)
    write (*, '(1x, a, 3i3)') 'index1:', index('banana', 'an', (/ .true., .false., .true. /))
  end subroutine backs

  ! The reductions of COMPLEX(16) values, with MASK= and DIM=.
  subroutine complex16
    complex(16) :: q(3), m(2, 2), r(2)
    complex(8) :: z(3)
    integer :: i

    q = (/ (1.0_16, 2.0_16), (3.0_16, -1.0_16), (0.5_16, 0.5_16) /)
    z = (/ (2.0_8, 0.0_8), (0.0_8, 1.0_8), (1.0_8, 1.0_8) /)
    m = reshape(q(1:2) * 2, (/ 2, 2 /), pad=(/ (1.0_16, 0.0_16) /))
    write (*, '(1x, a, 2f10.4)') 'sum:    ', sum(q)
    write (*, '(1x, a, 2f10.4)') 'product:', product(q)
    write (*, '(1x, a, 2f10.4)') 'masked: ', sum(q, mask=real(q) > 0.7_16)
    write (*, '(1x, a, 2f10.4)') 'dot:    ', dot_product(q, z)
    r = sum(m, dim=1)
    write (*, '(1x, a, 4f10.4)') 'by dim: ', r
    r = matmul(m, q(1:2))
    write (*, '(1x, a, 4f10.4)') 'matmul: ', r
    write (*, '(1x, a, 4f10.4)') 'mm2:    ', matmul(q(2:3), m)
    i = 2
    write (*, '(1x, a, 4f10.4)') 'run dim:', sum(m, dim=i)
  end subroutine complex16

  ! Statement functions whose value, and dummies, are of a derived type.
  subroutine stmtfuncs
    type(point) :: p, q, mid, shift
    real :: a, d

    mid(p, q) = point((p%x + q%x) / 2, (p%y + q%y) / 2)
    shift(p, d) = point(p%x + d, p%y)
    p = point(1.0, 2.0)
    q = point(3.0, 6.0)
    p = mid(p, q)
    write (*, '(1x, a, 2f6.2)') 'mid:  ', p
    write (*, '(1x, a, 2f6.2)') 'shift:', mid(shift(p, 1.0), q)
    a = 4.0
    q = mid(point(a, a), q)
    write (*, '(1x, a, f6.2)') 'x:    ', q%x
  end subroutine stmtfuncs

  ! Bounds from an element's component.
  subroutine bounds(d, k)
    type(t) :: d(:)
    integer :: k
    real :: y(d(1)%n), z(d(k)%n, d(k + 1)%n)

    write (*, '(1x, a, 3i3)') 'bounds:', size(y), shape(z)
  end subroutine bounds

  ! TRANSFER to a derived type: of constants, a PARAMETER, folded; of
  ! variables, computed.
  subroutine transfers
    type(pair), parameter :: p = transfer((/ 65538, 1145258561 /), pair(0, 0, '', (/ 0, 0 /)))
    type(pair), parameter :: q(2) = transfer((/ 1, 2, 3, 4 /), p, 2)
    type(pair) :: v
    integer :: k

    write (*, '(1x, a, 2i6, 1x, a, 2i4)') 'p:', p
    write (*, '(1x, a, 4i6)') 'q:', q%lo, q%hi
    v = transfer(1145258561, v)
    write (*, '(1x, a, 2i6)') 'v:', v%lo, v%hi
    k = 7
    v = transfer((/ k, k + 1 /), v)
    write (*, '(1x, a, 2i6, 2i4)') 'w:', v%lo, v%hi, ichar(v%tag(1:1)), ichar(v%tag(2:2))
  end subroutine transfers
end program errata
