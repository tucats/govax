! Fortran 95's intrinsic changes: MAXLOC and MINLOC with DIM=, CEILING
! and FLOOR with KIND=, NULL with MOLD=, CPU_TIME.
program intrin95
  implicit none
  integer :: m(2,3)
  integer, pointer :: p
  real :: t0, t1
  m = reshape((/ 3, 9, 4, 1, 7, 8 /), (/ 2, 3 /))
  print '(3I3)', maxloc(m, dim=1)
  print '(2I3)', minloc(m, dim=2)
  print '(3I3)', maxloc(m, dim=1, mask=m < 8)
  print '(2I4)', ceiling(2.5, kind=2), floor(-2.5, kind=1)
  p => null(p)
  print *, associated(p)
  call cpu_time(t0)
  call cpu_time(t1)
  print *, t1 >= t0, t0 >= 0.0
end program intrin95
