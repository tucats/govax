! Dynamic arrays of derived types (docs/planning/PHASE-105 - Fortran 90
! derived types.md, subtask 5b): allocatable arrays, automatic arrays, and
! array-valued functions whose elements are of a type, each reached through
! its descriptor as an array of numbers is, an element being the type's size;
! and their components with rank, PTS%X, passed and assigned.
program typedyn
  implicit none
  type point
    real :: x, y
  end type point
  type named
    character(len=4) :: tag
    type(point) :: at
    integer :: hits(2)
  end type named
  type(point), allocatable :: pts(:), grid(:, :)
  type(named), allocatable :: marks(:)
  type(point) :: fixed(3)
  integer :: i, j, n, ierr

  n = 5
  allocate (pts(n), grid(2, 3), stat = ierr)
  print 20, ierr, allocated(pts), size(pts), shape(grid)
  pts%x = (/ (real(i), i = 1, n) /)
  pts%y = pts%x ** 2
  print 10, (pts(i)%x, pts(i)%y, i = 1, n)
  do j = 1, 3
    do i = 1, 2
      grid(i, j) = point(real(i), real(j))
    end do
  end do
  grid(2, :)%y = grid(2, :)%y * 10.0
  print 10, ((grid(i, j)%x, grid(i, j)%y, i = 1, 2), j = 1, 3)

  ! Components of nested types; whole elements moved.
  allocate (marks(0:2))
  do i = 0, 2
    marks(i) = named('M' // char(ichar('0') + i), point(real(i), real(i) / 2.0), (/ i, 2 * i /))
  end do
  marks(2)%at = pts(5)
  marks%hits(2) = marks%hits(1) + 100
  print 30, (marks(i)%tag, marks(i)%at%x, marks(i)%at%y, marks(i)%hits, i = 0, 2)

  ! Passed on: to an assumed-shape dummy, an old one, and by component.
  call nudge(pts, 0.5)
  call drag(pts, n)
  call scale(pts%y, 0.1)
  call scale(marks%at%x, 3.0)
  print 10, (pts(i)%x, pts(i)%y, i = 1, n)
  print 10, marks%at%x

  ! Automatic arrays of a type, and functions whose values are arrays of one.
  call reverse(pts)
  print 10, (pts(i)%x, pts(i)%y, i = 1, n)
  fixed = point(-1.0, -1.0)
  fixed(1:2) = corners(2.0)
  print 10, (fixed(i)%x, fixed(i)%y, i = 1, 3)
  fixed = mids(pts(1:4))
  print 10, (fixed(i)%x, fixed(i)%y, i = 1, 3)
  call show(mids(pts))
  call total(mids(corners(4.0)), 1)

  deallocate (pts, grid, marks)
  print 20, 0, allocated(pts), 0, 0, 0
  allocate (pts(2))
  pts = corners(1.0)
  print 10, (pts(i)%x, pts(i)%y, i = 1, 2)

10 format (1x, 6f8.2)
20 format (1x, i3, l3, 3i3)
30 format (3(1x, a4, 2f6.1, 2i4))

contains

  subroutine nudge(a, d)
    type(point) :: a(:)
    real :: d
    a%x = a%x + d
  end subroutine nudge

  subroutine drag(a, n)
    integer :: n
    type(point) :: a(n)
    a(2:n)%y = a(1:n-1)%y
  end subroutine drag

  subroutine scale(v, f)
    real :: v(:), f
    v = v * f
  end subroutine scale

  subroutine show(a)
    type(point), intent(in) :: a(:)
    print 10, a%x
10  format (1x, 'show', 6f8.2)
  end subroutine show

  subroutine total(a, n)
    integer :: n
    type(point), intent(in) :: a(n)
    print 10, sum(a%x), sum(a%y)
10  format (1x, 'total', 2f8.2)
  end subroutine total

  subroutine reverse(a)
    type(point) :: a(:)
    type(point) :: work(size(a))
    integer :: k
    work = a
    do k = 1, size(a)
      a(k) = work(size(a) + 1 - k)
    end do
  end subroutine reverse

  function corners(s)
    real :: s
    type(point) :: corners(2)
    corners(1) = point(0.0, 0.0)
    corners(2) = point(s, s)
  end function corners

  function mids(a) result(m)
    type(point), intent(in) :: a(:)
    type(point) :: m(size(a) - 1)
    m%x = (a(1:size(a)-1)%x + a(2:)%x) / 2.0
    m%y = (a(1:size(a)-1)%y + a(2:)%y) / 2.0
  end function mids

end program typedyn
