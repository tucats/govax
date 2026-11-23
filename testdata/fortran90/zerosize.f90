! Zero-sized explicit-shape arrays (Phase 112, subtask 3): a dimension whose upper
! bound is below its lower bound has no elements (the Fortran 90 standard's
! 5.1.2.4.1), and LBOUND and UBOUND of a whole array's empty dimension are 1 and 0.
module empties
  implicit none
  integer :: k
  integer, parameter :: none(0) = (/ (k, k = 1, 0) /)
  type box
    integer :: first
    real :: nothing(5:4)
    integer :: last
  end type box
contains
  ! An assumed-shape dummy given an empty array or section.
  subroutine shaped(label, x)
    character(len=*), intent(in) :: label
    real, intent(in) :: x(:)
    write (*, '(1X, A, 3I3, F6.1)') label, size(x), lbound(x), ubound(x), sum(x)
  end subroutine shaped

  ! An explicit-shape dummy of a size the caller gives: 0.
  subroutine sized(n, x)
    integer, intent(in) :: n
    real, intent(inout) :: x(n)
    x = -1.0
    write (*, '(1X, A, 2I3)') 'sized:', n, size(x)
  end subroutine sized
end module empties

program zerosize
  use empties
  implicit none
  real :: a(0), b(5:1) = 2.0, c(3)
  integer :: d(2, 0), e(4:3, 2), i
  integer :: x, z0(0), y
  common /pair/ x, z0, y
  type(box) :: bx
  character(len=4) :: names(0)
  namelist /nl/ x, z0

  c = (/ 1.0, 2.0, 3.0 /)
  a = 7.0
  b = a
  d = 3
  write (*, '(1X, A, 6I3)') 'sizes:', size(a), size(b), size(c(3:2)), size(d), size(e), size(none)
  write (*, '(1X, A, 4I3)') 'shapes:', shape(d), shape(e)
  write (*, '(1X, A, 4I3)') 'e bounds:', lbound(e), ubound(e)
  write (*, '(1X, A, 4I3)') 'd bounds:', lbound(d), ubound(d)
  write (*, '(1X, A, 2I3)') 'b bounds:', lbound(b), ubound(b)

  ! The reductions of no elements.
  write (*, '(1X, A, F6.1, 2L2, 2I3)') 'reduced:', sum(a), any(a > 0.0), all(a > 0.0), &
      count(a > 0.0), maxloc(a)
  write (*, '(1X, A, F6.1, I3)') 'product, dot:', product(a), int(dot_product(a, b))

  ! Elements of nothing in output lists and implied DOs: only the text is written.
  write (*, '(1X, A, 10F5.1)') 'a and d:', a, d
  write (*, '(1X, A, 10I3)') 'implied:', (d(1, i), i = 1, 0), (none(i), i = 1, size(none))

  call shaped('whole:', a)
  call shaped('section:', c(2:1))
  call sized(0, a)

  ! Constructors and PACK may make nothing too.
  write (*, '(1X, A, 2I3)') 'made:', size(pack(c, c > 5.0)), size((/ a, c /))
  where (a > 0.0) a = 1.0

  ! An empty array takes no storage in COMMON or a record.
  x = 1
  y = 2
  bx%first = 4
  bx%last = 5
  write (*, '(1X, A, 5I3)') 'stored:', x, y, size(z0), bx%first, bx%last
  write (*, '(1X, A, 2I3)') 'names:', size(names), len(names)
  write (*, nl)

  ! Unformatted: an empty item is no bytes.
  open (1, file='zerosize.tmp', form='unformatted', status='replace')
  write (1) a, x, bx, d
  rewind (1)
  read (1) d, y
  close (1, status='delete')
  write (*, '(1X, A, I3)') 'read back:', y
end program zerosize
