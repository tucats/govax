! WHERE inside WHERE, ELSEWHERE with a mask, a named WHERE construct.
program where95
  implicit none
  integer :: a(8), b(8), i
  a = (/ (i, i = -3, 4) /)
  b = 0
  outer: where (a > 0)
    where (mod(a, 2) == 0)
      b = 2
    elsewhere
      b = 1
    end where
  elsewhere (a < -1) outer
    b = -2
  elsewhere outer
    b = -1
  end where outer
  print '(8I3)', b
end program where95
