! Image smoothing with WHERE (Phase 102, subtask 12): a small picture, a
! rectangle with a hole and stray pixels, drawn a row at a time by MERGE of
! CHARACTER values; each pixel's neighbors counted from eight sections; WHERE
! statements removing the strays and filling the hole; a weighted smoothing of
! the interior from five sections, once into another array and once in place
! (through a temporary); the result drawn in three levels by a WHERE construct
! with ELSEWHERE into a CHARACTER array; its edges found by a gradient of
! sections and drawn by a WHERE; and the levels scaled to digits by MAXVAL
! under a mask, with COUNT, SUM, and MAXLOC.
program smooth
  implicit none
  integer, parameter :: m = 8, n = 12
  real :: img(m, n), s(m, n), t(m, n), g(m, n)
  integer :: on(m, n), nb(m, n), level(m, n), i, k(2)
  character :: pic(m, n)

  img = 0.0
  img(3:6, 4:9) = 8.0
  img(4, 6) = 0.0
  img(2, 11) = 8.0
  img(7, 2) = 8.0
  print '(1x, a)', 'The picture:'
  do i = 1, m
    print '(3x, 12a1)', merge('#', '.', img(i, :) > 0.0)
  end do

  on = merge(1, 0, img > 0.0)
  nb = 0
  nb(2:m-1, 2:n-1) = on(1:m-2, 1:n-2) + on(2:m-1, 1:n-2) + on(3:m, 1:n-2) &
                   + on(1:m-2, 2:n-1)                    + on(3:m, 2:n-1) &
                   + on(1:m-2, 3:n)   + on(2:m-1, 3:n)   + on(3:m, 3:n)
  print '(1x, a)', 'Neighbors:'
  do i = 1, m
    print '(3x, 12i1)', nb(i, :)
  end do
  where (on == 1 .and. nb <= 1) img = 0.0
  where (on == 0 .and. nb >= 7) img = 8.0
  print '(1x, a)', 'Cleaned:'
  do i = 1, m
    print '(3x, 12a1)', merge('#', '.', img(i, :) > 0.0)
  end do

  s = img
  s(2:m-1, 2:n-1) = (4.0 * img(2:m-1, 2:n-1) + img(1:m-2, 2:n-1) + img(3:m, 2:n-1) &
                   + img(2:m-1, 1:n-2) + img(2:m-1, 3:n)) / 8.0
  t = img
  t(2:m-1, 2:n-1) = (4.0 * t(2:m-1, 2:n-1) + t(1:m-2, 2:n-1) + t(3:m, 2:n-1) &
                   + t(2:m-1, 1:n-2) + t(2:m-1, 3:n)) / 8.0
  print '(1x, a, l2)', 'Smoothed in place the same:', all(s == t)
  print '(1x, a)', 'Smoothed:'
  do i = 1, m
    print '(3x, 12i2)', nint(s(i, :))
  end do

  where (s >= 6.0)
    pic = '#'
    level = 2
  elsewhere
    pic = '.'
    level = 0
  end where
  where (s > 0.0 .and. s < 6.0)
    pic = '+'
    level = 1
  end where
  print '(1x, a)', 'In three levels:'
  do i = 1, m
    print '(3x, 12a1, 3x, 12i1)', pic(i, :), level(i, :)
  end do
  print '(1x, a, 3i4)', 'Pixels by level:', count(level == 0), count(level == 1), &
        count(level == 2)

  g = 0.0
  g(2:m-1, 2:n-1) = abs(s(2:m-1, 3:n) - s(2:m-1, 1:n-2)) &
                  + abs(s(3:m, 2:n-1) - s(1:m-2, 2:n-1))
  pic = ' '
  where (g >= 6.0) pic = '*'
  print '(1x, a)', 'Edges:'
  do i = 1, m
    print '(3x, a1, 12a1, a1)', '|', pic(i, :), '|'
  end do
  k = maxloc(g)
  print '(1x, a, f5.1, a, 2i3)', 'Steepest:', maxval(g), ' at', k

  where (s > 0.0) s = s * 9.0 / maxval(s)
  print '(1x, a)', 'Scaled to digits:'
  do i = 1, m
    print '(3x, 12i1)', nint(s(i, :))
  end do
  print '(1x, a, f6.2)', 'Mean where lit:', sum(s, mask=s > 0.0) / count(s > 0.0)
end program smooth
