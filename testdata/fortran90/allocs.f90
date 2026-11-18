! Allocatable arrays (Phase 104, subtask 2): ALLOCATE with bounds known only
! when the program runs, ALLOCATED before and after, elements, whole arrays,
! and sections of one and two dimensions, SIZE, LBOUND, UBOUND, and SHAPE,
! DEALLOCATE and ALLOCATE again with other bounds, a zero-sized array, a whole
! allocatable array passed to an explicit-shape dummy, an internal procedure
! filling its host's, a recursive function with one of its own at each level,
! and the arrays in output lists whole and as sections.
program allocs
  implicit none
  real, allocatable :: v(:), m(:, :)
  integer, allocatable :: k(:)
  integer :: n, i, j, levels

  n = 6
  print '(1x, a, l2)', 'allocated before:', allocated(v)

  allocate (v(n), m(0:2, -1:n-4))
  print '(1x, a, l2)', 'allocated after: ', allocated(v)
  print '(1x, a, 3i3)', 'size, lbound, ubound of v:', size(v), lbound(v), ubound(v)
  print '(1x, a, 2i3, 1x, 2i3)', 'shape and lbound of m:    ', shape(m), lbound(m)

  do i = 1, n
    v(i) = i * i
  end do
  print '(1x, a, 6f5.0)', 'v:   ', v

  v = v + 1.0
  v(2:n:2) = 0.0
  print '(1x, a, 6f5.0)', 'v:   ', v
  print '(1x, a, f6.0)', 'sum: ', sum(v)

  do j = -1, n - 4
    do i = 0, 2
      m(i, j) = 10 * i + j
    end do
  end do
  do i = 0, 2
    print '(1x, a, i1, a, 4f5.0)', 'm(', i, ',:): ', m(i, :)
  end do
  print '(1x, a, 4f5.0)', 'col sums:', sum(m, 1)

  call total(v, n)

  deallocate (v)
  print '(1x, a, l2)', 'allocated v:', allocated(v)
  allocate (v(-2:2))
  v = (/ 5.0, 4.0, 3.0, 2.0, 1.0 /)
  print '(1x, a, 2i3, f5.0)', 'again:', lbound(v), ubound(v), v(-2)

  call squares(4)
  print '(1x, a, 4i4)', 'k:   ', k

  deallocate (k)
  allocate (k(0))
  deallocate (k)
  allocate (k(3:2))
  print '(1x, a, l2, 2i3, i4)', 'empty:', allocated(k), size(k), ubound(k), sum(k)

  print '(1x, a, i6)', 'levels:', levels(4)

  deallocate (v, m, k)
  print '(1x, a, 3l2)', 'all gone:', allocated(v), allocated(m), allocated(k)

contains

  subroutine squares(count)
    integer, intent(in) :: count
    integer :: i
    allocate (k(count))
    do i = 1, count
      k(i) = i * i
    end do
  end subroutine squares

end program allocs

subroutine total(a, n)
  integer, intent(in) :: n
  real, intent(in) :: a(n)
  print '(1x, a, f6.0)', 'total:', sum(a)
end subroutine total

recursive integer function levels(n) result(r)
  integer, intent(in) :: n
  integer, allocatable :: w(:)
  allocate (w(n))
  w = n
  if (n > 1) then
    r = sum(w) + levels(n - 1)
  else
    r = sum(w)
  end if
  w(1) = r
  r = w(1)
  deallocate (w)
end function levels
