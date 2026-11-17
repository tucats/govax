! report.f90: the free-form subroutine of mixed (mixed.f90 says why).
subroutine report(a, n)
  implicit none
  include 'limits.inc'
  integer n, a(n), k, total

  total = 0
  do k = 1, n
     total = total + a(k)
  end do
  write (*, 10) total, n, nmax
10 format (1x, "total ", i4, " of ", i2, " squares; at most ", i2)
end subroutine report
