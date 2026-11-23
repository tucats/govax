! Zero-sized explicit-shape arrays.
program zerosize
  implicit none
  real :: a(0), b(5:1)
  integer :: c(3, 0)
  print *, size(a), size(b), size(c), shape(c)
  print *, sum(a), lbound(b), ubound(b)
end program zerosize
