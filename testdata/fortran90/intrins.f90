! Fortran 95's changes to the intrinsic procedures (Phase 112, subtask 11):
! MAXLOC and MINLOC with DIM=, CEILING and FLOOR with KIND=, and the
! subroutine CPU_TIME.
program intrins
  implicit none
  integer :: m(2,3), line(3), d, i, big
  real :: a(5), x, t0, t1
  double precision :: dt
  logical :: never(2,3)

  ! M is   3  4  7
  !        9  1  8
  m = reshape((/ 3, 9, 4, 1, 7, 8 /), (/ 2, 3 /))
  a = (/ 2.0, 5.0, 1.0, 5.0, 0.0 /)

  ! With DIM=, each line along DIM gives the place of its first extreme.
  print '(3I3)', maxloc(m, dim=1)
  print '(2I3)', minloc(m, dim=2)
  print '(3I3)', maxloc(m, dim=1, mask=m < 8)
  never = .false.
  print '(3I3)', minloc(m, 1, never)
  print '(2I3)', maxloc(m(:, 2:3), dim=1)

  ! Of an array of rank one, the place is a scalar; the first of equals wins.
  print '(2I3)', maxloc(a, dim=1), minloc(a, 1)

  ! Without DIM=, a second argument that is LOGICAL is the mask, as in
  ! Fortran 90: the place of 7, the greatest below 8.
  print '(2I3)', maxloc(m, m < 8)

  ! DIM= known only when the program runs: the value's shape follows it.
  do d = 1, 2
    print '(3I3)', maxloc(m, dim=d)
  end do

  ! In an expression, with MINLOC's along the same dimension.
  line = maxloc(m, dim=1) + 10 * minloc(m, dim=1)
  print '(3I3)', line
  print '(3I3)', (minloc(m(:, i), 1), i = 1, 3)

  ! KIND= makes the result that kind of INTEGER.
  print '(2I4)', ceiling(2.5, kind=2), floor(-2.5, kind=1)
  print '(3I2)', kind(ceiling(2.5, kind=1)), kind(floor(2.5, kind=2)), kind(ceiling(2.5))
  x = -7.25
  big = ceiling(70000.5, kind=4)
  print '(2I4,I7)', ceiling(x, kind=2), floor(x, kind=4), big

  ! CPU_TIME: the process's CPU time in seconds; only differences mean
  ! anything, and the time never goes back.
  call cpu_time(t0)
  do i = 1, 1000
    x = x + sqrt(real(i))
  end do
  call cpu_time(t1)
  call cpu_time(dt)
  print '(3L2)', t1 >= t0, t0 >= 0.0, dt >= dble(t1) - 0.01d0
end program intrins
