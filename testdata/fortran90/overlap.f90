! Overlap in array assignments (Phase 102, subtask 5): an assignment acts as if
! its whole value were computed before any element of its target is changed.
! A section read ahead of where it is written runs up, one read behind runs
! down (in either dimension, a stride of 2 too), one read both ways and a
! reversal go through a temporary, as does an array an EQUIVALENCE makes part
! of the target; an element of the target is read once, before the loops; and
! a CHARACTER array's own element and substrings.
program overlap
  implicit none
  real :: a(10), b(9), m(4, 4)
  equivalence (a(2), b)
  character(len=4) :: c(3)
  integer :: i, j

  call fill(a)
  a(1:9) = a(2:10)
  print '(1x, a, 10f4.0)', 'ahead:  ', a

  call fill(a)
  a(2:10) = a(1:9)
  print '(1x, a, 10f4.0)', 'behind: ', a

  call fill(a)
  a(2:9) = a(1:8) + a(3:10)
  print '(1x, a, 10f4.0)', 'both:   ', a

  call fill(a)
  a = a / a(10)
  print '(1x, a, 10f4.1)', 'scaled: ', a

  call fill(a)
  a(10:1:-1) = a
  print '(1x, a, 10f4.0)', 'turned: ', a

  call fill(a)
  a(3:9:2) = a(1:7:2)
  print '(1x, a, 10f4.0)', 'by 2:   ', a

  call fill(a)
  b = a(1:9)
  print '(1x, a, 10f4.0)', 'equiv:  ', a

  do j = 1, 4
    do i = 1, 4
      m(i, j) = 10 * i + j
    end do
  end do
  m(1:3, 2:4) = m(2:4, 1:3)
  do i = 1, 4
    print '(1x, a, i1, a, 4f4.0)', 'm(', i, ',:): ', (m(i, j), j = 1, 4)
  end do

  c(1) = 'abcd'
  c(2) = 'efgh'
  c(3) = 'ijkl'
  c = c(2)
  c(:)(2:4) = c(:)(1:3)
  print '(1x, a, 3(1x, a))', 'c:      ', c
end program overlap

! Fills x with 1 to 10.
subroutine fill(x)
  implicit none
  real :: x(10)
  integer :: i

  do i = 1, 10
    x(i) = i
  end do
end subroutine fill
