! ALLOCATE and DEALLOCATE of pointers (Phase 107, subtask 4): a scalar
! pointer of INTEGER, REAL(8), CHARACTER, and a derived type given a new
! target, the old one kept by another pointer and given back through it;
! ALLOCATE of a pointer already associated, which only moves it; array
! pointers of one and two dimensions with bounds of their own, a section of
! one read through another pointer, an array grown by allocating a bigger
! one and moving the pointer; STAT= on both statements, DEALLOCATE of a
! disassociated pointer failing; a module's pointer allocated by a
! subroutine; a host's pointer allocated by its internal procedure; a
! recursive function's frame pointer, a target allocated at each level.
module store
  implicit none
  integer, pointer :: shelf(:)
  real, pointer :: last
end module store

program ptralloc
  use store
  implicit none
  type pair
    integer first, second
  end type pair
  integer, pointer :: p, q
  real(8), pointer :: x
  character(len=5), pointer :: word
  type(pair), pointer :: pp
  integer, pointer :: a(:), b(:), grid(:, :), grown(:)
  integer :: i, j, n, ist
  integer :: deepest

  ! A new target for P, then another; Q keeps the first.
  allocate (p)
  p = 11
  q => p
  allocate (p)
  p = 22
  print '(1x, a, 2i4, 2l2)', 'two targets:    ', q, p, associated (q, p), associated (p)
  deallocate (q)
  print '(1x, a, i4, 2l2)', 'one given back: ', p, associated (q), associated (p)
  deallocate (p)
  print '(1x, a, l2)', 'none left:      ', associated (p)

  ! Other types.
  allocate (x, word, pp)
  x = 1.0d0 / 3.0d0
  word = 'HELLO'
  pp%first = 3
  pp%second = pp%first * 4
  print '(1x, a, f10.7, 1x, a, 1x, a, 2i4)', 'other types:    ', x, word, word(2:4), pp
  deallocate (x, word, pp)
  print '(1x, a, 3l2)', 'given back:     ', associated (x), associated (word), associated (pp)

  ! An array pointer with bounds of its own, and a section of it.
  allocate (a(0:4))
  a = (/ (i * i, i = 0, 4) /)
  b => a(1:3)
  print '(1x, a, 3i4, 5i4)', 'array:          ', lbound (a), ubound (a), size (a), a
  print '(1x, a, 2i4, 3i4)', 'section:        ', lbound (b), ubound (b), b
  b = -b
  print '(1x, a, 5i4)', 'through it:     ', a
  deallocate (a)
  print '(1x, a, l2)', 'given back:     ', associated (a)

  ! Two dimensions.
  allocate (grid(2, -1:1))
  do j = -1, 1
    do i = 1, 2
      grid(i, j) = 10 * i + j
    end do
  end do
  print '(1x, a, 4i4, 6i4)', 'grid:           ', lbound (grid), ubound (grid), grid
  print '(1x, a, i4, 3i4)', 'sums:           ', sum (grid), sum (grid, dim=1)
  deallocate (grid)

  ! Grown as it fills: each time a target twice as long, the values moved.
  n = 2
  allocate (grown(n))
  do i = 1, 9
    if (i > n) then
      allocate (b(2 * n))
      b(1:n) = grown
      deallocate (grown)
      grown => b
      n = 2 * n
    end if
    grown(i) = i * 3
  end do
  print '(1x, a, i4, 9i4)', 'grown:          ', size (grown), grown(1:9)
  deallocate (grown)

  ! STAT=: success is 0; a disassociated pointer can't be deallocated.
  allocate (p, a(3), stat=ist)
  print '(1x, a, i2, 2l2)', 'with STAT=:     ', ist, associated (p), associated (a)
  deallocate (p, a, stat=ist)
  print '(1x, a, i2, 2l2)', 'deallocated:    ', ist, associated (p), associated (a)
  deallocate (p, stat=ist)
  print '(1x, a, l2)', 'disassociated:  ', ist /= 0

  ! A module's pointers, given targets by a subroutine.
  call stock (4)
  print '(1x, a, i4, 4i4, f6.1)', 'module:         ', size (shelf), shelf, last
  deallocate (shelf, last)

  ! A host's pointer, given a target by its internal procedure.
  nullify (q)
  call give
  print '(1x, a, l2, i4)', 'host:           ', associated (q), q
  deallocate (q)

  ! A recursive function's own pointer, a target at each level.
  print '(1x, a, i6)', 'recursion:      ', deepest (5)

contains

  subroutine give
    allocate (q)
    q = 99
  end subroutine give

end program ptralloc

subroutine stock (n)
  use store
  implicit none
  integer, intent(in) :: n
  integer :: i
  allocate (shelf(n), last)
  shelf = (/ (100 + i, i = 1, n) /)
  last = 2.5
end subroutine stock

! Each level allocates its own target, which deeper levels mustn't disturb.
recursive integer function deepest (n) result (r)
  implicit none
  integer, intent(in) :: n
  integer, pointer :: mine
  allocate (mine)
  mine = n * 10
  if (n > 1) then
    r = deepest (n - 1) + mine
  else
    r = mine
  end if
  deallocate (mine)
end function deepest
