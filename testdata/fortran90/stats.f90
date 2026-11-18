! A statistics program (Phase 102, subtask 12): twenty scores in an array
! PARAMETER; their mean, variance, and standard deviation by SUM of array
! expressions; MINVAL, MAXVAL, MAXLOC, and MINLOC; COUNT and SUM with masks;
! a histogram by COUNT, its bars written by implied DOs; standard scores; a sort
! by MINLOC of a shrinking section, and the median; a moving average of
! overlapping sections; and the scores RESHAPEd into a table of five students
! and four tests, summed, averaged, and searched by DIM=, with ALL, ANY, and
! COUNT by DIM=, and a correlation by DOT_PRODUCT.
program stats
  implicit none
  integer, parameter :: n = 20
  integer, parameter :: score(n) = (/ 72, 85, 64, 91, 58, 77, 69, 88, 95, 47, &
                                      81, 73, 66, 79, 84, 55, 90, 62, 76, 83 /)
  integer, parameter :: low(5) = (/ 40, 50, 60, 70, 80 /)
  real :: x(n), z(n), sorted(n), smooth(n - 2), mean, var, sd, median, t
  real :: d1(5), d2(5), r
  integer :: i, j, k(1), hist(5), tests(5, 4), best(4)
  logical :: passed(5)

  x = score
  mean = sum(x) / n
  var = sum((x - mean) ** 2) / (n - 1)
  sd = sqrt(var)
  print '(1x, a, i3, a, i3, a, i3)', 'Count:', size(score), '   lowest:', &
        minval(score), '   highest:', maxval(score)
  print '(1x, a, f7.2, a, f8.2, a, f6.2)', 'Mean:', mean, '   variance:', var, &
        '   deviation:', sd
  k = maxloc(score)
  print '(1x, a, i3)', 'The highest is number', k(1)
  k = minloc(score)
  print '(1x, a, i3)', 'The lowest is number', k(1)
  print '(1x, a, i3)', 'Above the mean:', count(x > mean)
  print '(1x, a, f7.2)', 'Mean of those passing:', &
        sum(x, mask=score >= 60) / count(score >= 60)
  print '(1x, a, f7.2)', 'Mean of those failing:', &
        sum(x, mask=score < 60) / count(score < 60)

  do j = 1, 5
    hist(j) = count(score >= low(j) .and. score < low(j) + 10)
  end do
  hist(5) = hist(5) + count(score >= 90)
  print '(1x, a)', 'Histogram:'
  do j = 1, 5
    print '(1x, i3, a, i3, 2x, 20a1)', low(j), ' -', low(j) + 9 + merge(10, 0, j == 5), &
          ('*', i = 1, hist(j))
  end do
  print '(1x, a, l2)', 'Every score counted:', sum(hist) == n

  z = (x - mean) / sd
  print '(1x, a)', 'Standard scores:'
  print '(1x, 10f6.2)', z
  print '(1x, a, 2f6.2)', 'Their extremes:', minval(z), maxval(z)

  sorted = x
  do i = 1, n - 1
    k = minloc(sorted(i:n))
    j = i - 1 + k(1)
    t = sorted(i)
    sorted(i) = sorted(j)
    sorted(j) = t
  end do
  print '(1x, a)', 'Sorted:'
  print '(1x, 10i4)', nint(sorted)
  median = (sorted(n / 2) + sorted(n / 2 + 1)) / 2.0
  print '(1x, a, f6.1)', 'Median:', median
  print '(1x, a, l2)', 'In order:', all(sorted(1:n-1) <= sorted(2:n))

  smooth = (x(1:n-2) + x(2:n-1) + x(3:n)) / 3.0
  print '(1x, a)', 'Moving averages of three:'
  print '(1x, 9f6.1)', smooth
  x(2:n-1) = (x(1:n-2) + x(2:n-1) + x(3:n)) / 3.0
  print '(1x, a, l2)', 'The same in place:', all(abs(x(2:n-1) - smooth) < 1.0e-4)

  tests = reshape(score, (/ 5, 4 /))
  print '(1x, a)', 'Five students, four tests:'
  do i = 1, 5
    print '(1x, i3, 2x, 4i4, a, i5)', i, tests(i, :), '  total', sum(tests(i, :))
  end do
  print '(1x, a, 4f6.1)', 'Test means:   ', sum(tests, dim=1) / 5.0
  print '(1x, a, 4i6)', 'Test highs:   ', maxval(tests, dim=1)
  do j = 1, 4
    k = maxloc(tests(:, j))
    best(j) = k(1)
  end do
  print '(1x, a, 4i6)', 'By student:   ', best
  print '(1x, a, 5i5)', 'Student lows: ', minval(tests, dim=2)
  print '(1x, a, 5i5)', 'Eighty up:    ', count(tests >= 80, dim=2)
  passed = all(tests >= 60, dim=2)
  print '(1x, a, 5l3)', 'Passed all:   ', passed
  print '(1x, a, 4l3)', 'Someone failed:', any(tests < 60, dim=1)
  print '(1x, a, i3)', 'Top total, student', sum(maxloc(sum(tests, dim=2)))

  d1 = tests(:, 1) - sum(tests(:, 1)) / 5.0
  d2 = tests(:, 2) - sum(tests(:, 2)) / 5.0
  r = dot_product(d1, d2) / sqrt(dot_product(d1, d1) * dot_product(d2, d2))
  print '(1x, a, f7.3)', 'Tests 1 and 2 correlate:', r
end program stats
