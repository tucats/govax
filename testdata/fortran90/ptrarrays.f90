! Array pointers (Phase 107, subtask 3): => to a whole array with its own
! bounds, to sections with bounds from 1, to a section of a pointer's target,
! to another pointer, and to itself reversed; NULLIFY and ASSOCIATED with and
! without a target, a zero-sized one among them; the inquiries; a pointer's
! elements in expressions, assignments, WHERE, reductions, and I/O; a pointer
! passed to an assumed-shape dummy and to an old explicit-shape one; array
! assignments whose sides may meet through a pointer; components with a rank;
! CHARACTER elements; array pointers in a module and in COMMON; and a store
! through a pointer the optimizer must see change its target.
module shelf
  implicit none
  real, pointer :: row(:)
end module shelf

program ptrarrays
  use shelf
  implicit none
  type point
    integer x, y
  end type point
  type grid
    integer coord(0:2)
  end type grid
  integer, target :: a(0:9), m(4, 5), b(5)
  integer, pointer :: p(:), q(:, :), r(:)
  integer, pointer :: e
  type(point), target :: pts(-1:1)
  type(grid), target :: g
  character(len=3), target :: names(4)
  character(len=3), pointer :: cp(:)
  real, target :: data(6)
  integer, pointer :: shared(:)
  common /link/ shared
  integer i, j, total
  integer oldsum
  external oldsum

  ! Disassociated.
  nullify (p, q)
  print *, associated (p), associated (q)

  ! The whole array, with its own bounds.
  a = (/ (10 * i, i = 0, 9) /)
  p => a
  print *, lbound (p), ubound (p), size (p)
  print *, p(0), p(9), associated (p), associated (p, a)

  ! A section: bounds from 1, every third element of A.
  p => a(2:9:3)
  print *, lbound (p), ubound (p), size (p)
  print *, p
  p = p + 1
  print *, a(:4)
  print *, a(5:)
  print *, associated (p, a), associated (p, a(2:9:3)), associated (p, a(2:8:3))

  ! A section of a pointer's target, and another pointer's association.
  r => p(2:3)
  print *, r, associated (r, a(5:8:3))
  r => p
  print *, associated (r, p), size (r)

  ! A pointer reversed onto itself: its own descriptor is read before it
  ! changes.
  p => a(1:)
  p => p(9:1:-1)
  print *, p(1), p(9), lbound (p)

  ! A scalar pointer to an element of an array pointer's target.
  e => p(2)
  e = -1
  print *, a(8)

  ! A zero-sized target is never associated.
  p => a(5:4)
  print *, size (p), associated (p), associated (p, a(5:4))
  nullify (p)
  print *, associated (p), associated (p, a)

  ! Two dimensions: every other column of the middle rows.
  m = reshape ((/ (i, i = 1, 20) /), (/ 4, 5 /))
  q => m(2:3, ::2)
  print *, shape (q), lbound (q, 2), ubound (q, 2)
  print *, sum (q), maxval (q), q(2, 3)
  where (q > 10) q = 0
  print *, m(2, :)
  print *, m(3, :)

  ! Passed to an assumed-shape dummy, which counts from 1, and to an old
  ! explicit-shape dummy, which takes the elements one after another.
  p => a(1:9:2)
  print *, sumof (p), oldsum (p, size (p))
  p => a(3:6)
  print *, sumof (p), oldsum (p, size (p))

  ! An array assignment that may read what it writes through a pointer.
  b = (/ 1, 2, 3, 4, 5 /)
  p => b
  b(2:5) = p(1:4)
  print *, b
  p => b(5:1:-1)
  b = p
  print *, b

  ! A pointer in COMMON, given its target here and used in another unit.
  shared => b(2:4)
  call negate
  print *, b

  ! Components with a rank: a whole array component keeps its bounds, and an
  ! array of records' component is a section.
  pts = (/ point(1, 2), point(3, 4), point(5, 6) /)
  p => pts%y
  print *, lbound (p), ubound (p), p
  p = p * 10
  print *, pts(1)%y
  g%coord = (/ 7, 8, 9 /)
  p => g%coord
  print *, lbound (p), ubound (p), p(2)
  p => g%coord(1:)
  print *, lbound (p), ubound (p), p(2)

  ! CHARACTER elements, and a substring of each.
  names = (/ 'ONE', 'TWO', 'SIX', 'TEN' /)
  cp => names(2:4:2)
  print *, size (cp)
  print *, cp
  cp(2) = 'NIL'
  print *, names
  write (*, '(1X, 4A4)') (cp(j), j = 1, size (cp)), names(1), names(3)

  ! An array pointer in a module, given its target here and used in a
  ! subroutine.
  data = (/ 1.5, 2.5, 3.5, 4.5, 5.5, 6.5 /)
  row => data(2::2)
  call halve
  write (*, '(1X, 6F6.2)') data

  ! Total through a pointer in a loop.
  total = 0
  p => a(::3)
  do i = 1, size (p)
    total = total + p(i)
  end do
  print *, total

  ! A store through a pointer between a store to its target and a read of
  ! it, which the optimizer must not carry past it.
  p => a(2:2)
  a(2) = 7
  p(1) = 1
  total = a(2)
  print *, total

contains

  integer function sumof (v)
    integer v(:)
    sumof = lbound (v, 1) * 1000 + sum (v)
  end function sumof
end program ptrarrays

integer function oldsum (v, n)
  integer n, v(n), i
  oldsum = 0
  do i = 1, n
    oldsum = oldsum + i * v(i)
  end do
end function oldsum

subroutine negate
  integer, pointer :: shared(:)
  common /link/ shared
  shared = -shared
  print *, size (shared), lbound (shared), shared(3)
end subroutine negate

subroutine halve
  use shelf
  row = row / 2
end subroutine halve
