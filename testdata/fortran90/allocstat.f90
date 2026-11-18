! Allocation status (Phase 104, subtask 3): STAT= on ALLOCATE and DEALLOCATE,
! succeeding and failing, into an INTEGER(2) and an array element; a statement
! that stops at its first failure; a zero-sized array; a subprogram's
! allocatable arrays deallocated at every return (an early RETURN, END, a
! function's value, an alternate return, an ENTRY) and never at entry still
! allocated; a SAVE'd one kept; a host's array allocated by its internal
! procedure and freed at the host's return; an internal procedure's own; a
! recursive function's, one at each level.
program allocstat
  implicit none
  real, allocatable :: a(:), b(:), c(:)
  integer :: ist, i, k, total
  integer(2) :: short
  integer :: stats(3)
  integer :: early, nested

  allocate (a(3), stat=ist)
  print '(1x, a, i2, l2)', 'allocate:          ', ist, allocated(a)
  allocate (a(4), stat=ist)
  print '(1x, a, l2, i3)', 'again fails:       ', ist /= 0, size(a)
  deallocate (a, stat=short)
  print '(1x, a, i2, l2)', 'deallocate:        ', short, allocated(a)
  deallocate (a, stat=stats(2))
  print '(1x, a, l2)', 'again fails:       ', stats(2) /= 0

  allocate (b(2))
  allocate (a(5), b(6), c(7), stat=ist)
  print '(1x, a, l2, 3l2, i3)', 'stops at b:        ', ist /= 0, allocated(a), &
    allocated(b), allocated(c), size(b)
  deallocate (a, b, stat=ist)
  print '(1x, a, i2, 3l2)', 'both freed:        ', ist, allocated(a), allocated(b), allocated(c)

  allocate (c(0), stat=ist)
  print '(1x, a, i2, l2, i3)', 'zero-sized:        ', ist, allocated(c), size(c)
  deallocate (c)

  do k = 1, 2
    call work(3 + k)
  end do
  do k = 1, 3
    call keep(k)
  end do

  print '(1x, a, 3i5)', 'early:             ', early(0), early(4), early(-1)

  do k = 1, 2
    call outer(k)
  end do
  call inner_own

  print '(1x, a, 2i5)', 'nested:            ', nested(4), nested(5)

  do k = 1, 2
    call alt(k, *10)
    print '(1x, a, i2)', 'normal return:     ', k
    cycle
10  print '(1x, a, i2)', 'alternate return:  ', k
  end do

  call both(3)
  call second(4)
  call both(2)

end program allocstat

! A host's array allocated by its internal procedure is the host's, freed
! by the host's return, so the next call finds it unallocated.
subroutine outer(n)
  integer, intent(in) :: n
  integer, allocatable :: h(:)
  print '(1x, a, i2, l2)', 'outer at entry:    ', n, allocated(h)
  call fill
  print '(1x, a, i2, 5i3)', 'outer filled:      ', n, h
contains
  subroutine fill
    integer :: i
    allocate (h(n + 2))
    h = (/ (i * n, i = 1, n + 2) /)
  end subroutine fill
end subroutine outer

! An internal procedure's own array is its own, freed at its return.
subroutine inner_own
  integer :: j
  do j = 1, 2
    call mine(j)
  end do
contains
  subroutine mine(m)
    integer, intent(in) :: m
    real, allocatable :: t(:)
    print '(1x, a, i2, l2)', 'mine at entry:     ', m, allocated(t)
    allocate (t(m))
    t = 0.5
    print '(1x, a, i2, f5.1)', 'mine sum:          ', m, sum(t)
  end subroutine mine
end subroutine inner_own

! Allocated, never deallocated: every return does it.
subroutine work(n)
  integer, intent(in) :: n
  integer, allocatable :: w(:)
  integer :: i
  print '(1x, a, i2, l2)', 'work at entry:     ', n, allocated(w)
  allocate (w(n))
  w = (/ (i, i = 1, n) /)
  print '(1x, a, i2, i4)', 'work sum:          ', n, sum(w)
end subroutine work

! SAVE'd: allocated by the first call, kept for the next ones.
subroutine keep(n)
  integer, intent(in) :: n
  integer, allocatable, save :: s(:)
  if (.not. allocated(s)) then
    print '(1x, a, i2)', 'keep allocates at: ', n
    allocate (s(2))
    s = 0
  end if
  s(1) = s(1) + n
  s(2) = s(2) + 1
  print '(1x, a, i2, 2i3)', 'keep:              ', n, s
end subroutine keep

! A function's array, freed at its early RETURN and at its END.
integer function early(n)
  integer, intent(in) :: n
  integer, allocatable :: v(:)
  integer :: i
  if (n < 0) then
    early = -1
    return
  end if
  allocate (v(0:n))
  v = (/ (i * i, i = 0, n) /)
  early = sum(v)
  if (n == 0) return
  early = early + size(v)
end function early

! Each level of the recursion has its own array, freed as the level returns.
recursive integer function nested(n) result(r)
  integer, intent(in) :: n
  integer, allocatable :: q(:)
  allocate (q(n))
  q = n
  if (n > 1) then
    r = sum(q) + nested(n - 1)
  else
    r = sum(q)
  end if
end function nested

! An alternate return's number computed from the array the return frees.
subroutine alt(k, *)
  integer, intent(in) :: k
  integer, allocatable :: z(:)
  allocate (z(2))
  z = k - 1
  if (z(1) == 1) return z(2)
end subroutine alt

! Two entry points, each finding the array unallocated.
subroutine both(n)
  integer, intent(in) :: n
  real, allocatable :: e(:)
  print '(1x, a, i2, l2)', 'both at entry:     ', n, allocated(e)
  go to 1
  entry second(n)
  print '(1x, a, i2, l2)', 'second at entry:   ', n, allocated(e)
1 allocate (e(n))
  e = 2.0
  print '(1x, a, i2, f5.1)', 'sum:               ', n, sum(e)
end subroutine both
