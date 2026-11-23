! FORALL: the statement, the construct, masks, nesting, WHERE inside,
! a pointer assignment, and the as-if-all-at-once rule.
program forall
  implicit none
  integer, parameter :: n = 4
  real :: a(n,n), b(n,n), h(n,n)
  integer :: v(8), i, j
  type pt
    integer, pointer :: p
  end type
  type(pt) :: pat(8)
  integer, target :: obj(8)
  a = 0.0
  forall (i = 1:n) a(i,i) = real(i)
  forall (i = 1:n, j = 1:n, a(i,j) /= 0.0) b(i,j) = 1.0 / a(i,j)
  forall (i = 1:n, j = 1:n) h(i,j) = 1.0 / real(i + j - 1)
  print '(4F8.4)', h
  v = (/ (i, i = 1, 8) /)
  forall (i = 2:8) v(i) = v(i-1)          ! all right sides first
  print '(8I3)', v
  forall (i = 1:n)
    forall (j = 1:n, j > i) a(i,j) = 10.0*i + j
    where (a(i,:) == 0.0) a(i,:) = -1.0
  end forall
  print '(4F6.1)', a
  obj = (/ (10*i, i = 1, 8) /)
  forall (j = 1:8) pat(j)%p => obj(1 + ieor(j-1, 2))
  print '(8I4)', (pat(j)%p, j = 1, 8)
  named: forall (i = 1:8:2)
    v(i) = 0
  end forall named
  print '(8I3)', v
end program forall
