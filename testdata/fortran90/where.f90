! WHERE (Phase 102, subtask 10): the statement and the construct, with and
! without ELSEWHERE; a construct whose first assignment changes what its mask
! reads (the mask then kept in a temporary); a mask that reads what the
! assignment writes, one element behind (the loop runs down); two dimensions;
! INTEGER targets and a constructor's values; a value through a temporary
! under a mask; and a WHERE statement as a logical IF's.
program where
  implicit none
  real :: a(6), b(6), m(2, 3), q(2, 3)
  integer :: i, j, k(6)
  logical :: l(6)

  a = (/ 4.0, -1.0, 9.0, 0.0, -4.0, 16.0 /)
  b = 0.0
  where (a > 0.0) b = sqrt(a)
  print '(1x, a, 6f6.1)', 'roots:   ', b

  where (a > 0.0)
     b = a / 2.0
  elsewhere
     b = -1.0
  end where
  print '(1x, a, 6f6.1)', 'halves:  ', b

  where (a > 0.0)
     a = -a
     b = 1.0
  end where
  print '(1x, a, 6f6.1)', 'negated: ', a
  print '(1x, a, 6f6.1)', 'flags:   ', b

  a = (/ 1.0, 0.0, 0.0, 2.0, 0.0, 0.0 /)
  where (a(1:5) /= 0.0) a(2:6) = a(1:5)
  print '(1x, a, 6f6.1)', 'smeared: ', a

  do j = 1, 3
    do i = 1, 2
      m(i, j) = i - j
    end do
  end do

  where (m /= 0.0)
     q = 1.0 / m
  elsewhere
     q = 0.0
  end where
  print '(1x, a, 6f6.2)', 'inverses:', q

  k = 0
  l = (/ .true., .false., .true., .false., .true., .false. /)
  where (l) k = (/ (i * 10, i = 1, 6) /)
  where (.not. l) k = k + 1
  print '(1x, a, 6i6)', 'picked:  ', k

  a = (/ 1.0, 2.0, 3.0, 4.0, 5.0, 6.0 /)
  where (l) a = a(6:1:-1)
  print '(1x, a, 6f6.1)', 'swapped: ', a

  if (count(l) == 3) where (a > 3.0) a = 0.0
  print '(1x, a, 6f6.1)', 'cleared: ', a
end program where
